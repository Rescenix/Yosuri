package handler

// 工具审批（RequestPort 模式，对标 agent-framework-go 的 ExternalRequest）：
// Ask 模式下，四态机在执行「危险工具」（写盘 / 执行命令 / MCP 文件写删）前
// 通过 SSE 推 approval_request 事件，goroutine 在一个 per-request 的 channel 上阻塞等待；
// 前端弹批准条，POST /api/code/workflow/approve {id, allow, remember} 写回 channel 恢复执行。
// Yolo 模式下全程不拦截，工具照跑。
//
// don't-ask-again 规则（remember=true）按「工具签名」（tool + 可选归一化参数）存进
// SessionStore，该会话内同款危险工具不再弹批准条——抄自 agent-framework-go 的
// toolapproval 中间件常设规则思路。

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"backend/internal/ai/core"
)

// 危险工具分级：这些工具在 Ask 模式必须等人批准；其余（read_file /
// search_memory / dispatch_agent / 只读 MCP）任何模式都直过，不烦人。
var dangerousToolSet = map[string]bool{
	// MCP filesystem 写删类：mcp__fs__write / edit / delete_file / move_file / create_directory
	"mcp__fs__write_file":       true,
	"mcp__fs__edit_file":        true,
	"mcp__fs__delete_file":      true,
	"mcp__fs__delete_directory": true,
	"mcp__fs__move_file":        true,
	"mcp__fs__create_directory": true,
	"mcp__fs__create_file":      true,
	// MCP shell：执行任意命令，副作用最大，必拦
	"mcp__shell__run": true,
	// 注：浏览器渲染/真机验证/截图已由 harness 内置预览面板（browser_preview_tool.go）
	// 负责，不再给 LLM 暴露 chrome_devtools MCP——避免 agent 自己开独立 Chrome 窗口、
	// 架空后端预览面板。故 chrome_devtools 工具不再登记进危险集合。
}

// isDangerousTool 判定一个工具名是否需要审批拦截。
// 规则：内置写/执行工具、MCP 文件系统写删类（mcp__fs__ 且非 read/list 前缀）算危险。
func isDangerousTool(name string) bool {
	if dangerousToolSet[name] {
		return true
	}
	// MCP 文件系统写删类用前缀识别：mcp__fs__X，X 不是 read/list/get 开头即危险
	if strings.HasPrefix(name, "mcp__fs__") {
		rest := strings.TrimPrefix(name, "mcp__fs__")
		switch {
		case strings.HasPrefix(rest, "read"),
			strings.HasPrefix(rest, "list"),
			strings.HasPrefix(rest, "get"),
			strings.HasPrefix(rest, "search"),
			strings.HasPrefix(rest, "directory_tree"):
			return false
		default:
			return true // write/edit/delete/move/create/rename 等
		}
	}
	return false
}

// isReadOnlyToolCall 是 Harness 的通用审批判定。它不依赖 Agent 名称或提示词：
// shell 仅放行 Git 的查询子命令（允许先 cd 到工作目录），其余命令仍按危险操作审批。
func isReadOnlyToolCall(name, argsJSON string) bool {
	if strings.HasPrefix(name, "mcp__fs__") {
		return !isDangerousTool(name)
	}
	if name != "mcp__shell__run" {
		return !isDangerousTool(name) && name != "dispatch_agent"
	}
	var args struct {
		Command string `json:"command"`
	}
	if json.Unmarshal([]byte(argsJSON), &args) != nil {
		return false
	}
	for _, part := range strings.Split(args.Command, ";") {
		part = strings.TrimSpace(strings.ToLower(part))
		if part == "" || strings.HasPrefix(part, "cd ") || strings.HasPrefix(part, "set-location ") {
			continue
		}
		if !strings.HasPrefix(part, "git ") {
			return false
		}
		fields := strings.Fields(part)
		if len(fields) < 2 {
			return false
		}
		switch fields[1] {
		case "status", "diff", "show", "log", "branch", "rev-parse", "ls-files", "remote":
		default:
			return false
		}
	}
	return true
}

// irreversibleToolSet 不可逆文件操作：一旦执行（尤其 YOLO 全自动模式下）无法无损
// 撤回，即使有 AgentFS 影子仓能还原，也比普通写盘风险高一个量级，所以 YOLO 模式
// 下也必须走审批拦截，不让 agent「畅通无阻」地删/移。
var irreversibleToolSet = map[string]bool{
	"mcp__fs__delete_file":      true,
	"mcp__fs__delete_directory": true,
	"mcp__fs__move_file":        true,
}

// isIrreversibleTool 判定一个工具名是否代表不可逆的文件操作（删除/移动/重命名）。
// YOLO 模式下其余写操作（write/edit/create）仍畅通，仅这几类强制进审批。
func isIrreversibleTool(name string) bool {
	if irreversibleToolSet[name] {
		return true
	}
	if strings.HasPrefix(name, "mcp__fs__") {
		rest := strings.TrimPrefix(name, "mcp__fs__")
		switch {
		case strings.HasPrefix(rest, "delete"),
			strings.HasPrefix(rest, "move"),
			strings.HasPrefix(rest, "rename"):
			return true
		}
	}
	return false
}

// ---- 工作目录越界判定 ----
//
// MCP 各 server 底层已不再锁死目录（见 mcp_client.go fsAllowedDirs / grep_server.py），
// 「能不能碰工作目录以外的文件」改由这里判断：Ask 模式弹确认，Yolo 模式直接放行。
// 以前底层硬拦，agent 只能把文件都往工作目录里塞。

// toolPathArgs 从工具参数 JSON 里挑出「看起来是文件路径」的字段值。
// 覆盖 MCP filesystem 全家（path / source / destination / paths[]）与自研 grep server。
// mcp__shell__run 的 command 不在此列——它本来就是危险工具，任何路径都要批。
func toolPathArgs(argsJSON string) []string {
	if argsJSON == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &m); err != nil {
		return nil
	}
	var out []string
	add := func(v any) {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	// filePath 是 chrome_devtools upload_file 的参数名（驼峰，跟其余 MCP server 的
	// snake_case 不一样）——它读一个本地文件路径喂给浏览器上传，是跟 fs__read_file
	// 同量级的越界读风险（把 main-backend/.env 之类的敏感文件传到任意网页），
	// 不认出这个参数名的话，越界检测和目录级 remember 都对它失效。
	for _, k := range []string{"path", "source", "destination", "file_path", "filePath"} {
		add(m[k])
	}
	if arr, ok := m["paths"].([]any); ok {
		for _, v := range arr {
			add(v)
		}
	}
	return out
}

// absAgainstRoot 把路径参数解析成绝对路径；相对路径按工作目录解析
// （那就是各 MCP server 进程的实际 cwd 语义）。
func absAgainstRoot(p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Clean(filepath.Join(core.GetProjectRoot(), p))
}

// normCase 在 Windows 上抹掉大小写差异，否则 c:\x 与 C:\X 会被判成两个目录。
func normCase(p string) string {
	if runtime.GOOS == "windows" {
		return strings.ToLower(p)
	}
	return p
}

// pathOutsideRoot 判定单个路径是否落在 agent 工作目录之外。
func pathOutsideRoot(p string) bool {
	root := normCase(filepath.Clean(core.GetProjectRoot()))
	abs := normCase(absAgainstRoot(p))
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return true // 跨盘符（Windows 上 C: → D:）Rel 直接报错，按越界处理
	}
	return rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// toolOutsideRoot 返回该次工具调用是否触碰了工作目录之外的路径，以及第一个越界路径。
func toolOutsideRoot(argsJSON string) (bool, string) {
	for _, p := range toolPathArgs(argsJSON) {
		if pathOutsideRoot(p) {
			return true, p
		}
	}
	return false, ""
}

// outsideRememberKey 让越界访问的「不再询问」按目录记，而不是按工具记。
// 否则批准一次越界写盘后，之后任意目录的写都会静默放行——那等于把闸门拆了。
func outsideRememberKey(p string) string {
	return "approve:outside:" + normCase(filepath.Dir(absAgainstRoot(p)))
}

// approvalWaiter 是单次工作流运行期的审批等待器。
// 每个 workflow 请求 newApprovalWaiter() 一个，随请求生命周期存在。
type approvalWaiter struct {
	mu    sync.Mutex
	chans map[string]chan approvalDecision
	// keys: approval id → don't-ask-again 规则键。越界访问按目录记、普通危险工具按
	// 工具名记，两种粒度不同，所以在发起审批时就定好，approve 端点照此落库。
	keys map[string]string
}

type approvalDecision struct {
	allow bool
}

func newApprovalWaiter() *approvalWaiter {
	return &approvalWaiter{
		chans: make(map[string]chan approvalDecision),
		keys:  make(map[string]string),
	}
}

// approvalBackendTimeout 是后端侧的兜底超时：比前端的 60s 倒计时长 5s，
// 正常情况下前端会先发 approve 请求；只有前端整个挂掉（标签页关了、JS 崩了、
// 断网）时才由它兜底放行，避免工作流 goroutine 永久阻塞、SSE 连接一直挂着。
const approvalBackendTimeout = 65 * time.Second

// wait 阻塞直到该 id 收到批准决定 / 超时 / ctx 取消。返回是否允许执行。
// 调用前务必先 register 好 id（用 expect），否则无法被 approve 唤醒。
func (w *approvalWaiter) wait(id string, done <-chan struct{}) bool {
	w.mu.Lock()
	ch, ok := w.chans[id]
	w.mu.Unlock()
	if !ok {
		// 没登记就当允许（不应发生，防御性）
		return true
	}
	timer := time.NewTimer(approvalBackendTimeout)
	defer timer.Stop()
	select {
	case dec := <-ch:
		return dec.allow
	case <-timer.C:
		// 超时默认放行，与前端 60s 自动同意语义保持一致（不是"拒绝"，
		// 否则用户走开一会儿回来会发现任务被判死，比放行更难受）
		return true
	case <-done:
		return false // 客户端断开，中止执行
	}
}

// expect 登记一个待审批 id（连同它的 don't-ask-again 规则键），返回 decision channel。
func (w *approvalWaiter) expect(id, rememberKey string) chan approvalDecision {
	w.mu.Lock()
	defer w.mu.Unlock()
	ch := make(chan approvalDecision, 1)
	w.chans[id] = ch
	w.keys[id] = rememberKey
	return ch
}

// rememberKeyFor 取回该审批 id 对应的规则键（approve 端点写 don't-ask-again 时用）。
func (w *approvalWaiter) rememberKeyFor(id string) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.keys[id]
}

// resolve 由 approve 端点调用，把决定写回对应 channel。返回是否成功（id 存在）。
func (w *approvalWaiter) resolve(id string, allow bool) bool {
	w.mu.Lock()
	ch, ok := w.chans[id]
	w.mu.Unlock()
	if !ok {
		return false
	}
	select {
	case ch <- approvalDecision{allow: allow}:
	default:
	}
	return true
}

// rememberKey 由工具签名生成「don't ask again」规则键。
// 只取工具名 + 危险参数类别（不存完整参数，避免路径不同就失效），粒度=工具级。
func rememberKey(tool string) string {
	return "approve:" + tool
}

// shouldAutoApproveKey 检查该会话是否已对某条规则键设了 don't-ask-again。
func (r *WorkflowRunner) shouldAutoApproveKey(sessionID, key string) bool {
	if sessionID == "" || key == "" || r.chatHandler == nil || r.chatHandler.sessionStore == nil {
		return false
	}
	return r.chatHandler.sessionStore.GetApprovalRule(sessionID, key)
}

// setAutoApproveKey 把某条规则键的 don't-ask-again 写入会话状态。
func (r *WorkflowRunner) setAutoApproveKey(sessionID, key string) {
	if sessionID == "" || key == "" || r.chatHandler == nil || r.chatHandler.sessionStore == nil {
		return
	}
	r.chatHandler.sessionStore.SetApprovalRule(sessionID, key, true)
}

// 审批请求载荷（前端 POST 用）
type approvalResponse struct {
	ID       string `json:"id"`
	Allow    bool   `json:"allow"`
	Remember bool   `json:"remember"`
	Tool     string `json:"tool"` // remember=true 时带上工具名，写 don't-ask-again 规则
}

// ---- 全局审批 registry：让独立的 POST /api/code/workflow/approve 能定位到
// 正在进行中的工作流里那个阻塞的 waiter。以 approval id 为 key（id 全局唯一，
// 由四态机用 call id 或纳秒时间戳生成，不会跨工作流撞车）。 ----

var (
	approvalRegistryMu sync.Mutex
	approvalRegistry   = make(map[string]*approvalWaiter)
)

// registerApprovalWaiter 把一个 waiter 挂进全局 registry（按它持有的所有 approval id）。
// 简化做法：注册时并不知道 id（id 是执行时才生成），所以改为「按 request 维度」——
// 这里用 requestID（= workflow 级唯一串）作为 bucket，approve 端点带 requestID 查。
func registerApprovalWaiter(requestID string, w *approvalWaiter) {
	approvalRegistryMu.Lock()
	approvalRegistry[requestID] = w
	approvalRegistryMu.Unlock()
}

func unregisterApprovalWaiter(requestID string) {
	approvalRegistryMu.Lock()
	delete(approvalRegistry, requestID)
	approvalRegistryMu.Unlock()
}

// HandleCodeWorkflowApprove POST /api/code/workflow/approve
// 前端批准条「允许/拒绝」回调：把决定写回阻塞中的 waiter channel，恢复四态机执行。
// 同时处理 remember（don't-ask-again）：允许且勾选时，把该工具签名写进会话规则。
func (r *WorkflowRunner) HandleCodeWorkflowApprove(c *gin.Context) {
	var req approvalResponse
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	if req.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id 必填"})
		return
	}

	// 从 approval id 反解出 requestID：我们在推 approval_request 时把 requestID 编码进
	// id（格式 requestID::callID），这里拆出来定位 waiter。
	requestID := req.ID
	if idx := strings.Index(req.ID, "::"); idx >= 0 {
		requestID = req.ID[:idx]
	}

	approvalRegistryMu.Lock()
	waiter := approvalRegistry[requestID]
	approvalRegistryMu.Unlock()
	if waiter == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "审批已超时或工作流已结束", "id": req.ID})
		return
	}

	// remember 规则：仅当允许 + 勾选时生效。规则键在发起审批时就由 waiter 记下了
	// （越界访问按目录记、普通危险工具按工具名记），这里照取即可；取不到再退回前端
	// 带来的 tool 名，兼容老前端。
	if req.Allow && req.Remember && r.chatHandler != nil {
		sessionID := c.Query("session_id")
		if sessionID == "" {
			sessionID = c.GetHeader("X-Session-Id")
		}
		key := waiter.rememberKeyFor(req.ID)
		if key == "" && req.Tool != "" {
			key = rememberKey(req.Tool)
		}
		r.setAutoApproveKey(sessionID, key)
	}

	// 用完整 id（含 requestID::）去 resolve，waiter 内部按 id 找 channel
	if !waiter.resolve(req.ID, req.Allow) {
		c.JSON(http.StatusNotFound, gin.H{"error": "该审批 id 不存在或已处理", "id": req.ID})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "id": req.ID, "allow": req.Allow})
}
