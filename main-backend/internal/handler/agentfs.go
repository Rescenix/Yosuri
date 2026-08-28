package handler

// AgentFS —— 给 AI 文件写操作加上「本地历史时间线」。
//
// 设计定位（VS Code Timeline 风格）：
//   - AI 直接修改真实项目文件，无需显式"应用"。
//   - 每次写操作前捕获 before 内容，按 sha256 寻址 + gzip 压缩保存到本地。
//   - 审计时间线 audit.jsonl 逐笔记录路径、hash、工具来源，不存完整内容。
//   - 完全不走 git：回滚 = 从本地 blob 还原；diff = blob 与当前文件对比。
//   - GC 按版本数 / 总大小 / 年龄回收，避免大项目频繁改动导致存储膨胀。
//
// 关键约束：历史数据位于 ~/rescene_data/agentfs/history/<project>/，与用户项目 git
// 完全隔离，绝不污染主仓库。AgentFS 是旁路——任何错误都降级静默跳过，绝不
// 阻断正常的 Go 内置写盘主流程；外部 mcp__fs__* 仍保留兼容埋点。
//
// 埋点：Go 内置写工具与兼容 MCP 写工具在真实落盘前后分别调用
// OnBeforeWrite / OnAfterWrite（仅 write_file/edit_file 两类）。

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"backend/internal/ai/core"
)

// agentfsMu 保护 activeSession 的读写（callMCPTool 与工作流并发执行）。
var agentfsMu sync.Mutex

// activeSession 当前工作目录对应的 AgentFS 会话（由 SetWorkdir → OpenAgentFSSession 设置）。
var activeSession *agentfsSession

// agentfsSession 一个项目会话。
type agentfsSession struct {
	SessionID string    `json:"session_id"`
	Project   string    `json:"project"` // 项目名（= filepath.Base(workdir)）
	Workdir   string    `json:"workdir"` // 绝对路径
	OpenedAt  time.Time `json:"opened_at"`
	Seq       int       `json:"seq"` // 审计序号计数器
	// LastReportedSeq 上次 workflow_done 已下发过的审计序号水位线。
	// 卡片只报「本次工作流新产生的改动」：collectChangedFiles 只聚合
	// seq > LastReportedSeq 的审计条目，避免每次发「你好」都重复列出
	// 上一个工作流改过的文件（2026-08-28 用户反馈）。
	LastReportedSeq int `json:"last_reported_seq,omitempty"`
}

// agentfsRoot 返回 AgentFS 数据根目录（可被 RESCENE_DATA_DIR 覆盖，与 session/checkpoint 同域）。
func agentfsRoot() string {
	if d := os.Getenv("RESCENE_DATA_DIR"); d != "" {
		return filepath.Join(d, "agentfs")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, "rescene_data", "agentfs")
}

func agentfsSessionPath(project string) string {
	return filepath.Join(agentfsRoot(), "sessions", project+".json")
}

// sha256Of 计算内容哈希（用于审计 before/after 对比与 blob 寻址）。
func sha256Of(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])[:16]
}

// resolveAbsPath 把工具参数里的 path 解析成绝对路径（相对路径按当前工作目录）。
func resolveAbsPath(p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Clean(filepath.Join(core.GetProjectRoot(), p))
}

// OpenAgentFSSession 开辟（或恢复）一个项目的 AgentFS 会话。
// 由 SetWorkdir 成功后调用。失败静默返回 nil（旁路，不阻断主流程）。
func OpenAgentFSSession(project, workdir string, boundSessionID ...string) *agentfsSession {
	agentfsMu.Lock()
	defer agentfsMu.Unlock()

	if project == "" {
		project = filepath.Base(workdir)
	}

	// 确保历史目录存在
	histDir := agentfsHistoryDir(project)
	if err := os.MkdirAll(histDir, 0o755); err != nil {
		log.Printf("⚠️ AgentFS: 创建历史目录失败 %s: %v", histDir, err)
		return nil
	}

	sessionID := fmt.Sprintf("afs_%d", time.Now().UnixNano())
	if len(boundSessionID) > 0 && strings.TrimSpace(boundSessionID[0]) != "" {
		sessionID = strings.TrimSpace(boundSessionID[0])
	}
	sess := &agentfsSession{
		SessionID: sessionID,
		Project:   project,
		Workdir:   workdir,
		OpenedAt:  time.Now(),
		Seq:       0,
	}
	// 恢复已有会话的 seq
	if data, err := os.ReadFile(agentfsAuditPath(project)); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			var a agentfsAudit
			if json.Unmarshal([]byte(line), &a) == nil && a.Seq >= sess.Seq {
				sess.Seq = a.Seq + 1
			}
		}
	}
	// 落盘会话文件
	if buf, err := json.MarshalIndent(sess, "", "  "); err == nil {
		_ = os.MkdirAll(filepath.Dir(agentfsSessionPath(project)), 0o755)
		_ = os.WriteFile(agentfsSessionPath(project), buf, 0o644)
	}
	activeSession = sess
	return sess
}

// restoreActiveSession 后端重启后 activeSession 内存态丢失（nil）时，从磁盘
// sessions/*.json 恢复最近打开的会话。避免「后端重启 + 前端未刷新页面」时
// agent 写操作因 sess==nil 被 OnBeforeWrite/OnAfterWrite 静默跳过、审计零记录，
// 最终 workflow_done 收不到 changed_files（卡片不弹）的断链场景。
func restoreActiveSession() *agentfsSession {
	agentfsMu.Lock()
	defer agentfsMu.Unlock()
	if activeSession != nil {
		return activeSession
	}
	root := agentfsRoot()
	entries, err := os.ReadDir(filepath.Join(root, "sessions"))
	if err != nil {
		return nil
	}
	var latestName string
	var latestMod time.Time
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		if info, err := e.Info(); err == nil && info.ModTime().After(latestMod) {
			latestName = e.Name()
			latestMod = info.ModTime()
		}
	}
	if latestName == "" {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(root, "sessions", latestName))
	if err != nil {
		return nil
	}
	var sess agentfsSession
	if json.Unmarshal(data, &sess) != nil || sess.Workdir == "" {
		return nil
	}
	// 恢复 seq 计数（与 OpenAgentFSSession 同口径），保证审计序号连续
	if ad, err := os.ReadFile(agentfsAuditPath(sess.Project)); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(ad)), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			var a agentfsAudit
			if json.Unmarshal([]byte(line), &a) == nil && a.Seq >= sess.Seq {
				sess.Seq = a.Seq + 1
			}
		}
	}
	activeSession = &sess
	return activeSession
}

// OnBeforeWrite 在文件工具真实落盘前调用：捕获 before 内容，存进 pending。
// apply_patch 的每个新增/更新文件会分别调用一次；删除仍走不可逆审批。
func OnBeforeWrite(fullName string, args map[string]any) {
	if fullName != "write_file" && fullName != "edit_file" &&
		fullName != "apply_patch" &&
		fullName != "mcp__fs__write_file" && fullName != "mcp__fs__edit_file" {
		return
	}
	agentfsMu.Lock()
	sess := activeSession
	agentfsMu.Unlock()
	if sess == nil {
		sess = restoreActiveSession() // 后端重启后会话丢失：尝试从磁盘恢复
	}
	if sess == nil {
		return
	}
	p, _ := args["path"].(string)
	if p == "" {
		return
	}
	abs := resolveAbsPath(p)
	rel, err := filepath.Rel(sess.Workdir, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		rel = filepath.Base(abs)
	}
	rel = filepath.ToSlash(rel)
	before, err := os.ReadFile(abs)
	hash := ""
	exists := err == nil
	if exists {
		hash = sha256Of(before)
	}
	// 把 before 暂存到会话的 pending（用全局 map 按 relPath 关联，本进程内有效）
	agentfsPending.Store(sess.SessionID+"\x00"+rel, beforeHashEntry{hash: hash, exists: exists, data: before})
}

// OnAfterWrite 在文件工具真实落盘后调用：把 before 内容写入本地历史并记录审计。
func OnAfterWrite(fullName string, args map[string]any) {
	if fullName != "write_file" && fullName != "edit_file" &&
		fullName != "apply_patch" &&
		fullName != "mcp__fs__write_file" && fullName != "mcp__fs__edit_file" {
		return
	}
	agentfsMu.Lock()
	sess := activeSession
	agentfsMu.Unlock()
	if sess == nil {
		return
	}
	p, _ := args["path"].(string)
	if p == "" {
		return
	}
	abs := resolveAbsPath(p)
	rel, err := filepath.Rel(sess.Workdir, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		rel = filepath.Base(abs)
	}
	rel = filepath.ToSlash(rel)
	after, err := os.ReadFile(abs)
	if err != nil {
		return // 文件读不到（可能删除类，不在本钩子范围），跳过
	}

	// 取 before 信息
	key := sess.SessionID + "\x00" + rel
	var before []byte
	existsBefore := false
	if e, ok := agentfsPending.LoadAndDelete(key); ok {
		entry := e.(beforeHashEntry)
		before = entry.data
		existsBefore = entry.exists
	}

	sess.Seq++
	store := newHistoryStore(sess.Project)
	_, recordErr := store.RecordWrite(sess, opName(fullName), rel, fullName, before, after, existsBefore)
	if recordErr != nil {
		log.Printf("⚠️ AgentFS: 记录历史失败 %s: %v", rel, recordErr)
	}
}

// opName 把工具名映射成写操作类型。
func opName(fullName string) string {
	if fullName == "edit_file" || fullName == "apply_patch" || fullName == "mcp__fs__edit_file" {
		return "edit"
	}
	return "write"
}

// beforeHashEntry pending 暂存的 before 信息。
type beforeHashEntry struct {
	hash   string
	exists bool
	data   []byte
}

// agentfsPending 进程内 before 暂存，key = sessionID\x00relPath。
var agentfsPending sync.Map

// collectChangedFiles 聚合本次工作流新改过的文件列表，随 workflow_done 下发。
// 每个文件：first_seq=本会话第一次写（回退基准=它之前的状态，即工作流前版本）、
// last_seq=最后一次写、ops=本会话对该文件的写次数。sess 为空或没有新改动返回 nil。
// 只聚合 seq > sess.LastReportedSeq 的审计条目——上次 workflow_done 已报过的
// 文件不重复上报（2026-08-28 用户反馈「发句你好也弹卡片」：旧会话审计一直在，
// 不做水位线过滤时每次收尾都把历史改动全列一遍）。
func collectChangedFiles(sess *agentfsSession) []map[string]any {
	if sess == nil {
		return nil
	}
	ap := agentfsAuditPath(sess.Project)
	data, err := os.ReadFile(ap)
	if err != nil {
		return nil
	}
	type fileAgg struct {
		firstSeq     int
		lastSeq      int
		ops          int
		lastOp       string
		existsBefore bool
	}
	agg := map[string]*fileAgg{}
	var order []string
	floor := sess.LastReportedSeq
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var a agentfsAudit
		if json.Unmarshal([]byte(line), &a) != nil || a.SessionID != sess.SessionID {
			continue
		}
		if a.Seq <= floor {
			// 上次已上报过的历史条目，跳过
			continue
		}
		f, ok := agg[a.RelPath]
		if !ok {
			order = append(order, a.RelPath)
			f = &fileAgg{}
			agg[a.RelPath] = f
		}
		if f.firstSeq == 0 || a.Seq < f.firstSeq {
			f.firstSeq = a.Seq
			f.existsBefore = a.ExistsBefore // 首次写之前是否存在=回退是恢复还是删除
		}
		if a.Seq > f.lastSeq {
			f.lastSeq = a.Seq
			f.lastOp = a.Op
		}
		f.ops++
	}
	if len(order) == 0 {
		return nil
	}
	store := newHistoryStore(sess.Project)
	files := make([]map[string]any, 0, len(order))
	for _, rel := range order {
		f := agg[rel]
		added, removed := 0, 0
		// 该文件 first_seq 的 before vs 当前真实盘 → 增删行数（卡片红绿 +− 数字）
		if diffStr, err := store.Diff(sess.Workdir, rel, f.firstSeq); err == nil {
			for _, ln := range strings.Split(diffStr, "\n") {
				if strings.HasPrefix(ln, "+") && !strings.HasPrefix(ln, "+++") {
					added++
				} else if strings.HasPrefix(ln, "-") && !strings.HasPrefix(ln, "---") {
					removed++
				}
			}
		}
		files = append(files, map[string]any{
			"rel_path":     rel,
			"first_seq":    f.firstSeq,
			"last_seq":     f.lastSeq,
			"ops":          f.ops,
			"op":           f.lastOp,
			"exists_before": f.existsBefore,
			"added":        added,
			"removed":      removed,
		})
	}
	return files
}

// changedFilesPayload 聚合当前会话新改过的文件列表，供 workflow_done 各分支统一下发。
// 线程安全地取 activeSession；无会话或没新改动返回 nil（前端不弹卡片）。
// 读取后把水位线推到当前审计最大 seq——这批文件已上报过，下次收尾不再重复。
func changedFilesPayload() []map[string]any {
	agentfsMu.Lock()
	defer agentfsMu.Unlock()
	if activeSession == nil {
		return nil
	}
	files := collectChangedFiles(activeSession)
	if len(files) == 0 {
		return nil
	}
	// 推进水位线到本次审计末尾（含本工作流全部写操作）
	ap := agentfsAuditPath(activeSession.Project)
	if data, err := os.ReadFile(ap); err == nil {
		maxSeq := activeSession.LastReportedSeq
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			var a agentfsAudit
			if json.Unmarshal([]byte(line), &a) == nil && a.SessionID == activeSession.SessionID && a.Seq > maxSeq {
				maxSeq = a.Seq
			}
		}
		if maxSeq > activeSession.LastReportedSeq {
			activeSession.LastReportedSeq = maxSeq
			// 落盘会话（重启恢复时水位线不丢，避免重启后重复上报旧文件）
			if buf, err := json.MarshalIndent(activeSession, "", "  "); err == nil {
				_ = os.MkdirAll(filepath.Dir(agentfsSessionPath(activeSession.Project)), 0o755)
				_ = os.WriteFile(agentfsSessionPath(activeSession.Project), buf, 0o644)
			}
		}
	}
	return files
}

// --- HTTP handlers ---

// AgentFSOpen POST /api/agentfs/open {project?, workdir?} 开辟/恢复会话。
func AgentFSOpen(c *gin.Context) {
	var body struct {
		Project   string `json:"project"`
		Workdir   string `json:"workdir"`
		SessionID string `json:"session_id"`
	}
	_ = c.BindJSON(&body)
	if body.Workdir == "" {
		body.Workdir = core.GetProjectRoot()
	}
	if body.Project == "" {
		body.Project = filepath.Base(body.Workdir)
	}
	sess := OpenAgentFSSession(body.Project, body.Workdir, body.SessionID)
	if sess == nil {
		c.JSON(500, gin.H{"error": "AgentFS 会话开启失败（见后端日志）"})
		return
	}
	c.JSON(200, gin.H{"session_id": sess.SessionID, "project": sess.Project})
}

// AgentFSLog GET /api/agentfs/log?project= 返回审计时间线。
func AgentFSLog(c *gin.Context) {
	project := c.Query("project")
	sessionID := strings.TrimSpace(c.Query("session_id"))
	if project == "" {
		agentfsMu.Lock()
		if activeSession != nil {
			project = activeSession.Project
		}
		agentfsMu.Unlock()
	}
	if project == "" {
		c.JSON(400, gin.H{"error": "project 必填"})
		return
	}
	store := newHistoryStore(project)
	logEntries, err := store.List(sessionID)
	if err != nil {
		c.JSON(500, gin.H{"error": "读取审计日志失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"project": project, "log": logEntries, "current_branch": "main"})
}

// AgentFSDiff POST /api/agentfs/diff {project?, seq} 返回该审计记录的 before 与当前文件的 diff。
// project 可省略：为空时自动用当前 AgentFS 会话的项目（前端不用猜工作目录名）。
func AgentFSDiff(c *gin.Context) {
	var body struct {
		Project string `json:"project"`
		Seq     int    `json:"seq"`
	}
	_ = c.BindJSON(&body)
	if body.Seq <= 0 {
		c.JSON(400, gin.H{"error": "seq 必填"})
		return
	}
	if body.Project == "" {
		agentfsMu.Lock()
		if activeSession != nil {
			body.Project = activeSession.Project
		}
		agentfsMu.Unlock()
	}
	if body.Project == "" {
		c.JSON(400, gin.H{"error": "project 必填（当前无 AgentFS 会话）"})
		return
	}
	workdir := core.GetProjectRoot()
	if workdir == "" {
		agentfsMu.Lock()
		if activeSession != nil && activeSession.Project == body.Project {
			workdir = activeSession.Workdir
		}
		agentfsMu.Unlock()
	}
	store := newHistoryStore(body.Project)
	a, err := store.Find(body.Seq)
	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}
	diff, err := store.Diff(workdir, a.RelPath, body.Seq)
	if err != nil {
		c.JSON(500, gin.H{"error": "diff 失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"project": body.Project, "seq": body.Seq, "rel_path": a.RelPath, "diff": diff})
}

// AgentFSRestore POST /api/agentfs/restore {project?, seq, before?}
// 默认把文件还原到 seq 对应的写操作之后的状态（时间线点节点恢复）；
// before=true 时还原到该次写之前的状态（工作流结束卡片「回退到工作流前」），
// 若文件是本次写新建的（不存在 before），则回退=删除该文件。
// project 可省略：为空时自动用当前 AgentFS 会话的项目。
func AgentFSRestore(c *gin.Context) {
	var body struct {
		Project string `json:"project"`
		Seq     int    `json:"seq"`
		Before  bool   `json:"before"`
	}
	_ = c.BindJSON(&body)
	if body.Seq <= 0 {
		c.JSON(400, gin.H{"error": "seq 必填"})
		return
	}
	if body.Project == "" {
		agentfsMu.Lock()
		if activeSession != nil {
			body.Project = activeSession.Project
		}
		agentfsMu.Unlock()
	}
	if body.Project == "" {
		c.JSON(400, gin.H{"error": "project 必填（当前无 AgentFS 会话）"})
		return
	}
	workdir := core.GetProjectRoot()
	if workdir == "" {
		agentfsMu.Lock()
		if activeSession != nil && activeSession.Project == body.Project {
			workdir = activeSession.Workdir
		}
		agentfsMu.Unlock()
	}
	store := newHistoryStore(body.Project)
	a, err := store.Find(body.Seq)
	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}
	dst := filepath.Join(workdir, a.RelPath)

	var data []byte
	if body.Before {
		data, err = store.RestoreBefore(body.Seq)
		if err != nil {
			c.JSON(500, gin.H{"error": "还原失败: " + err.Error()})
			return
		}
		if data == nil {
			// 文件是本次写新建的：回退到工作流前 = 删除它
			if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
				c.JSON(500, gin.H{"error": "删除文件失败: " + err.Error()})
				return
			}
			c.JSON(200, gin.H{"restored": a.RelPath, "seq": body.Seq, "deleted": true})
			return
		}
	} else {
		data, err = store.Restore(body.Seq)
		if err != nil {
			c.JSON(500, gin.H{"error": "还原失败: " + err.Error()})
			return
		}
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		c.JSON(500, gin.H{"error": "创建目录失败: " + err.Error()})
		return
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		c.JSON(500, gin.H{"error": "写盘失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"restored": a.RelPath, "seq": body.Seq, "to": dst})
}
