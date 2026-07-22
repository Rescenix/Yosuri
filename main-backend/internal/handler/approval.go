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
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// 危险工具分级：这些工具在 Ask 模式必须等人批准；其余（read_file /
// search_memory / dispatch_agent / 只读 MCP）任何模式都直过，不烦人。
var dangerousToolSet = map[string]bool{
	// 内置（虽然工作流链里已被 MCP 取代，仍兜底拦截，防止模型直接调内置）
	"write_file":      true,
	"edit_file":       true,
	"execute_command": true,
	// MCP filesystem 写删类：mcp__fs__write / edit / delete_file / move_file / create_directory
	"mcp__fs__write_file":       true,
	"mcp__fs__edit_file":        true,
	"mcp__fs__delete_file":      true,
	"mcp__fs__delete_directory": true,
	"mcp__fs__move_file":        true,
	"mcp__fs__create_directory": true,
	"mcp__fs__create_file":      true,
	// 浏览器等外部副作用类（若接了 mcp 浏览器，按下不表，先留口）
	"mcp__playwright__*": false, // 占位，下面用前缀匹配处理
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

// approvalWaiter 是单次工作流运行期的审批等待器。
// 每个 workflow 请求 newApprovalWaiter() 一个，随请求生命周期存在。
type approvalWaiter struct {
	mu    sync.Mutex
	chans map[string]chan approvalDecision
}

type approvalDecision struct {
	allow bool
}

func newApprovalWaiter() *approvalWaiter {
	return &approvalWaiter{
		chans: make(map[string]chan approvalDecision),
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

// expect 登记一个待审批 id，返回该 id 的 decision channel。
func (w *approvalWaiter) expect(id string) chan approvalDecision {
	w.mu.Lock()
	defer w.mu.Unlock()
	ch := make(chan approvalDecision, 1)
	w.chans[id] = ch
	return ch
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

// shouldAutoApprove 检查该会话是否已对 tool 设了 don't-ask-again。
func (r *WorkflowRunner) shouldAutoApprove(sessionID, tool string) bool {
	if sessionID == "" || r.chatHandler == nil || r.chatHandler.sessionStore == nil {
		return false
	}
	return r.chatHandler.sessionStore.GetApprovalRule(sessionID, rememberKey(tool))
}

// setAutoApprove 把 tool 的 don't-ask-again 规则写入会话状态。
func (r *WorkflowRunner) setAutoApprove(sessionID, tool string) {
	if sessionID == "" || r.chatHandler == nil || r.chatHandler.sessionStore == nil {
		return
	}
	r.chatHandler.sessionStore.SetApprovalRule(sessionID, rememberKey(tool), true)
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

	// remember 规则：仅当允许 + 勾选时生效。工具名从原 approval_request 已发出，
	// 这里从 registry 找不到 tool 名——改为由前端在 remember 时把 tool 也带来。
	// 为解耦，前端 approve 时若 remember=true 需额外带 tool 字段（见 approvalResponse 扩展）。
	if req.Allow && req.Remember && req.Tool != "" && r.chatHandler != nil {
		sessionID := c.Query("session_id")
		if sessionID == "" {
			sessionID = c.GetHeader("X-Session-Id")
		}
		r.setAutoApprove(sessionID, req.Tool)
	}

	// 用完整 id（含 requestID::）去 resolve，waiter 内部按 id 找 channel
	if !waiter.resolve(req.ID, req.Allow) {
		c.JSON(http.StatusNotFound, gin.H{"error": "该审批 id 不存在或已处理", "id": req.ID})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "id": req.ID, "allow": req.Allow})
}
