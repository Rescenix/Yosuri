package handler

// AgentFS —— 给 AI 文件写操作加上「事务层」。
//
// 设计定位（与 README 核心理念段一致）：
//   - 内存快照隔离：每次写操作前先捕获 before、写后再捕获 after，影子仓完整记录
//   - 原子提交/回滚：用独立 git 仓库（~/rescene_data/agentfs/repos/<project>）承载，
//     git commit 天然原子；回退 = git checkout <sha> -- <file>
//   - 像素级审计时间线：audit.jsonl 逐笔记录 before/after hash + 工具来源
//   - 时间旅行调试：git log / git diff 在影子仓里跳跃
//
// 关键约束：影子仓位于 ~/rescene_data/agentfs/，与用户项目 git 完全隔离，绝不污染
// 主仓库（见项目测试隔离规矩）。AgentFS 是旁路——任何错误都降级静默跳过，绝不
// 阻断正常的 mcp__fs__* 写盘主流程。
//
// 埋点：callMCPTool 在真实落盘前后分别调 OnBeforeWrite / OnAfterWrite（仅 write_file/
// edit_file 两类），见 mcp_client.go。

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
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

// agentfsSession 一个项目会话 = 一次「在内存中开辟可追踪快照区」。
type agentfsSession struct {
	SessionID string    `json:"session_id"`
	Project   string    `json:"project"` // 项目名（= filepath.Base(workdir)）
	Workdir   string    `json:"workdir"` // 绝对路径
	OpenedAt  time.Time `json:"opened_at"`
	Head      string    `json:"head"` // 影子仓当前 HEAD commit（每次写后更新）
	Seq       int       `json:"seq"`  // 审计序号计数器
}

// agentfsAudit 审计时间线的一行。
type agentfsAudit struct {
	Seq        int       `json:"seq"`
	TS         time.Time `json:"ts"`
	Op         string    `json:"op"`       // write / edit
	RelPath    string    `json:"rel_path"` // 相对工作目录的路径
	BeforeHash string    `json:"before_hash"`
	AfterHash  string    `json:"after_hash"`
	Commit     string    `json:"commit"` // 影子仓本次提交的短 hash
	Tool       string    `json:"tool"`   // mcp__fs__write_file / mcp__fs__edit_file
	SessionID  string    `json:"session_id"`
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

func agentfsRepoDir(project string) string {
	return filepath.Join(agentfsRoot(), "repos", project)
}

func agentfsSessionPath(project string) string {
	return filepath.Join(agentfsRoot(), "sessions", project+".json")
}

func agentfsAuditPath(project string) string {
	return filepath.Join(agentfsRepoDir(project), ".agentfs", "audit.jsonl")
}

// gitAvailable 缓存 git 是否可用，避免每次 exec 探测。
var gitAvailable = func() bool {
	_, err := exec.LookPath("git")
	return err == nil
}()

// gitRun 在影子仓里执行 git 命令，返回合并输出。影子仓不可用时调用方应走降级。
func gitRun(repo string, args ...string) (string, error) {
	if !gitAvailable {
		return "", fmt.Errorf("git 不可用")
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// sha256Of 计算内容哈希（用于审计 before/after 对比）。
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

// OpenAgentFSSession 开辟（或恢复）一个项目的 AgentFS 会话，并确保影子仓已 git init。
// 由 SetWorkdir 成功后调用。失败静默返回 nil（旁路，不阻断主流程）。
func OpenAgentFSSession(project, workdir string, boundSessionID ...string) *agentfsSession {
	agentfsMu.Lock()
	defer agentfsMu.Unlock()

	if project == "" {
		project = filepath.Base(workdir)
	}
	repo := agentfsRepoDir(project)

	// 确保影子仓存在并 git init
	if err := os.MkdirAll(repo, 0o755); err != nil {
		log.Printf("⚠️ AgentFS: 创建影子仓失败 %s: %v", repo, err)
		return nil
	}
	if _, err := os.Stat(filepath.Join(repo, ".git")); err != nil {
		if out, err := gitRun(repo, "init", "-q"); err != nil {
			log.Printf("⚠️ AgentFS: git init 失败 %s: %v (%s)", repo, err, out)
			return nil
		}
	}

	sessionID := fmt.Sprintf("afs_%d", time.Now().UnixNano())
	if len(boundSessionID) > 0 && strings.TrimSpace(boundSessionID[0]) != "" {
		sessionID = strings.TrimSpace(boundSessionID[0])
	}
	sess := &agentfsSession{
		SessionID: sessionID,
		Project:   project,
		Workdir:   workdir,
		OpenedAt:  time.Time{},
	}
	sess.OpenedAt = time.Now()
	sess.Head = currentHead(repo)
	sess.Seq = 0
	// 恢复已有会话的 seq（看 audit.jsonl 行数）
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

// currentHead 返回影子仓当前 HEAD 短 hash；无提交返回空串。
func currentHead(repo string) string {
	out, err := gitRun(repo, "rev-parse", "--short", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// OnBeforeWrite 在 mcp__fs__* 真实落盘前调用：捕获 before 内容哈希，存进 pending。
// 仅对 write_file / edit_file 生效；其余工具直接返回。
func OnBeforeWrite(fullName string, args map[string]any) {
	if fullName != "mcp__fs__write_file" && fullName != "mcp__fs__edit_file" {
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
		rel = filepath.Base(abs) // 越界时也至少记文件名
	}
	rel = filepath.ToSlash(rel)
	before, err := os.ReadFile(abs)
	hash := ""
	if err == nil {
		hash = sha256Of(before)
	}
	// 把 before 暂存到会话的 pending（用全局 map 按 relPath 关联，本进程内有效）
	agentfsPending.Store(sess.SessionID+"\x00"+rel, beforeHashEntry{hash: hash, exists: err == nil})
}

// OnAfterWrite 在 mcp__fs__* 真实落盘后调用：同步真实盘到影子仓并提交，写审计。
func OnAfterWrite(fullName string, args map[string]any) {
	if fullName != "mcp__fs__write_file" && fullName != "mcp__fs__edit_file" {
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
	afterHash := sha256Of(after)

	// 取 before 哈希
	key := sess.SessionID + "\x00" + rel
	beforeHash := ""
	if e, ok := agentfsPending.LoadAndDelete(key); ok {
		beforeHash = e.(beforeHashEntry).hash
	}

	repo := agentfsRepoDir(sess.Project)
	shadowPath := filepath.Join(repo, rel)
	if err := os.MkdirAll(filepath.Dir(shadowPath), 0o755); err != nil {
		return
	}
	// 原子写入影子仓（tmp → rename，与 workflow_checkpoint 同范式）
	tmp := shadowPath + ".tmp"
	if err := os.WriteFile(tmp, after, 0o644); err != nil {
		return
	}
	if err := os.Rename(tmp, shadowPath); err != nil {
		return
	}

	// git 提交（降级：git 不可用则只留文件副本，无时间线）
	commit := ""
	if gitAvailable {
		if out, err := gitRun(repo, "add", "-A"); err != nil {
			log.Printf("⚠️ AgentFS: git add 失败: %v (%s)", err, out)
		} else {
			msg := fmt.Sprintf("op=%s path=%s tool=%s before=%s after=%s",
				opName(fullName), rel, fullName, beforeHash, afterHash)
			if out, err := gitRun(repo, "commit", "-q", "-m", msg); err != nil {
				// 无变化也会失败（nothing to commit），忽略
				_ = out
			}
			commit = currentHead(repo)
		}
	}

	sess.Seq++
	audit := agentfsAudit{
		Seq:        sess.Seq,
		TS:         time.Now(),
		Op:         opName(fullName),
		RelPath:    rel,
		BeforeHash: beforeHash,
		AfterHash:  afterHash,
		Commit:     commit,
		Tool:       fullName,
		SessionID:  sess.SessionID,
	}
	if buf, err := json.Marshal(audit); err == nil {
		ap := agentfsAuditPath(sess.Project)
		_ = os.MkdirAll(filepath.Dir(ap), 0o755)
		f, ferr := os.OpenFile(ap, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if ferr == nil {
			_, _ = f.Write(append(buf, '\n'))
			_ = f.Close()
		}
	}
	sess.Head = commit
}

// opName 把工具名映射成写操作类型。
func opName(fullName string) string {
	if fullName == "mcp__fs__edit_file" {
		return "edit"
	}
	return "write"
}

// beforeHashEntry pending 暂存的 before 信息。
type beforeHashEntry struct {
	hash   string
	exists bool
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
	c.JSON(200, gin.H{"session_id": sess.SessionID, "project": sess.Project, "head": sess.Head})
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
	ap := agentfsAuditPath(project)
	data, err := os.ReadFile(ap)
	if err != nil {
		c.JSON(200, gin.H{"project": project, "log": []any{}})
		return
	}
	var log []agentfsAudit
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var a agentfsAudit
		if json.Unmarshal([]byte(line), &a) == nil {
			if sessionID != "" && a.SessionID != sessionID {
				continue
			}
			log = append(log, a)
		}
	}
	c.JSON(200, gin.H{"project": project, "log": log})
}

// AgentFSDiff POST /api/agentfs/diff {project, commit} 返回该 commit 的 git diff。
func AgentFSDiff(c *gin.Context) {
	var body struct {
		Project string `json:"project"`
		Commit  string `json:"commit"`
	}
	_ = c.BindJSON(&body)
	if body.Project == "" || body.Commit == "" {
		c.JSON(400, gin.H{"error": "project 与 commit 必填"})
		return
	}
	repo := agentfsRepoDir(body.Project)
	out, err := gitRun(repo, "show", body.Commit, "--")
	if err != nil {
		c.JSON(500, gin.H{"error": "diff 失败: " + err.Error(), "detail": out})
		return
	}
	c.JSON(200, gin.H{"project": body.Project, "commit": body.Commit, "diff": out})
}

// AgentFSRestore POST /api/agentfs/restore {project, commit, path?}
// 把影子仓某 commit 的版本还原到真实盘（写盘，危险操作——调用方需自行确认）。
func AgentFSRestore(c *gin.Context) {
	var body struct {
		Project string `json:"project"`
		Commit  string `json:"commit"`
		Path    string `json:"path"` // 相对路径；空=整个工作目录还原到该 commit
	}
	_ = c.BindJSON(&body)
	if body.Project == "" || body.Commit == "" {
		c.JSON(400, gin.H{"error": "project 与 commit 必填"})
		return
	}
	repo := agentfsRepoDir(body.Project)
	// 从影子仓取该 commit 的文件内容
	var srcRel string
	if body.Path != "" {
		srcRel = body.Path
	} else {
		c.JSON(400, gin.H{"error": "v1 仅支持单文件还原，请传 path"})
		return
	}
	// git show <commit>:<rel> 取出历史内容
	out, err := gitRun(repo, "show", body.Commit+":"+srcRel)
	if err != nil {
		c.JSON(500, gin.H{"error": "读取历史版本失败: " + err.Error(), "detail": out})
		return
	}
	// 还原到真实盘（相对当前工作目录）
	workdir := core.GetProjectRoot()
	dst := filepath.Join(workdir, srcRel)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		c.JSON(500, gin.H{"error": "创建目录失败: " + err.Error()})
		return
	}
	if err := os.WriteFile(dst, []byte(out), 0o644); err != nil {
		c.JSON(500, gin.H{"error": "还原失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"restored": srcRel, "commit": body.Commit, "to": dst})
}
