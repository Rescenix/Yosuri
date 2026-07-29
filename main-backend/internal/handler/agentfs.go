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

// AgentFSDiff POST /api/agentfs/diff {project, seq} 返回该审计记录的 before 与当前文件的 diff。
func AgentFSDiff(c *gin.Context) {
	var body struct {
		Project string `json:"project"`
		Seq     int    `json:"seq"`
	}
	_ = c.BindJSON(&body)
	if body.Project == "" || body.Seq <= 0 {
		c.JSON(400, gin.H{"error": "project 与 seq 必填"})
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

// AgentFSRestore POST /api/agentfs/restore {project, seq} 把某次写操作前的版本还原到真实盘。
func AgentFSRestore(c *gin.Context) {
	var body struct {
		Project string `json:"project"`
		Seq     int    `json:"seq"`
	}
	_ = c.BindJSON(&body)
	if body.Project == "" || body.Seq <= 0 {
		c.JSON(400, gin.H{"error": "project 与 seq 必填"})
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
	data, err := store.Restore(body.Seq)
	if err != nil {
		c.JSON(500, gin.H{"error": "还原失败: " + err.Error()})
		return
	}
	dst := filepath.Join(workdir, a.RelPath)
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
