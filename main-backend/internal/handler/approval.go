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
	"os"
	"path/filepath"
	"regexp"
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
	"write_file": true,
	"edit_file":  true,
	"apply_patch": true,
	// 核心四件套：write/patch/bash 是文件写删与命令执行的入口，ask 模式必须拦截。
	// read 是纯读，任何模式直过（与旧 read_file 同级，不烦人）。
	"write":             true,
	"patch":             true,
	"bash":              true,
	"create_directory":  true,
	"move_file":         true,
	"delete_file":       true,
	"delete_directory":  true,
	"run_command":       true,
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
	if name != "mcp__shell__run" && name != "run_command" {
		return !isDangerousTool(name) && name != "dispatch_agent"
	}
	var args struct {
		Command string `json:"command"`
	}
	if json.Unmarshal([]byte(argsJSON), &args) != nil {
		return false
	}
	// 不允许把“看起来以 git status 开头”的复合 shell 命令伪装成只读操作。
	// 重定向、管道、后台执行、命令替换都可能在同一条命令里产生任意副作用。
	lowerCommand := strings.ToLower(args.Command)
	for _, token := range []string{"|", "&", ">", "<", "\n", "\r", "`", "$(", "@("} {
		if strings.Contains(lowerCommand, token) {
			return false
		}
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
	"delete_file":               true,
	"delete_directory":          true,
	"move_file":                 true,
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

func isIrreversibleToolCall(name, argsJSON string) bool {
	if isIrreversibleTool(name) {
		return true
	}
	return name == "apply_patch" && nativePatchContainsDelete(argsJSON)
}

// ---- 敏感文件覆写保护（YOLO 模式也强制审批）----
//
// 教训（2026-08-16 实锤）：agent 曾用「项目初始化」模板整体覆盖 README.md，
// 116 行品牌文档被 14 行模板替换。这类文件被 write_file 整体覆写 = 内容直接
// 蒸发，YOLO 全自动模式下没有人工确认就等于允许 agent 随手毁掉仓库门面/
// 依赖清单/密钥文件。所以对「已存在的敏感文件」做整文件覆写时，与不可逆
// 删除同级，YOLO 模式也必须进审批。
//
// edit_file / mcp__fs__edit_file（定向行级编辑）刻意不在集合里：改动小、
// AgentFS 影子仓可还原，日常维护不必每次都弹窗。

// sensitiveWriteToolSet 整文件覆写/新建类写工具。
var sensitiveWriteToolSet = map[string]bool{
	"write_file":              true,
	"apply_patch":             true,
	"create_file":             true,
	"mcp__fs__write_file":     true,
	"mcp__fs__create_file":    true,
}

// isSensitiveFile 判定路径是否命中「敏感文件」名单：仓库门面文档（README*/
// LICENSE）、依赖清单与锁文件、密钥凭据（.env*/证书/私钥）、协作规范
// （AGENTS.md/CLAUDE.md/.cursorrules/.gitignore）。这些被整体覆写 = 信息丢失
// 或安全风险。
func isSensitiveFile(p string) bool {
	base := strings.ToLower(filepath.Base(absAgainstRoot(p)))
	// 仓库门面文档（覆盖四语 README.*）
	if base == "readme" || base == "readme.md" || strings.HasPrefix(base, "readme.") {
		return true
	}
	if base == "license" || base == "license.md" || base == "license.txt" || base == "copying" {
		return true
	}
	// 密钥/凭据（基名以 .env 开头：.env / .env.local / .env.production 等）
	if strings.HasPrefix(base, ".env") {
		return true
	}
	for _, suf := range []string{".pem", ".key", ".p12", ".pfx", ".crt", ".cer"} {
		if strings.HasSuffix(base, suf) {
			return true
		}
	}
	// 依赖清单 / 锁文件 / 构建脚本 / 协作规范
	switch base {
	case "go.mod", "go.sum", "package.json", "package-lock.json", "pnpm-lock.yaml",
		"yarn.lock", "cargo.toml", "cargo.lock", "pyproject.toml", "requirements.txt",
		"composer.json", "composer.lock", "gemfile", "gemfile.lock", "pom.xml",
		"build.gradle", "dockerfile", "makefile", "cmakelists.txt",
		"agents.md", "claude.md", ".cursorrules", ".gitignore", ".gitattributes",
		"security.md", "contributing.md", "code_of_conduct.md":
		return true
	}
	return false
}

// fileExists 判断文件是否已存在（覆写才危险，新建不拦）。
// 复用 verify.go 的同包实现，避免重复声明。
func pathExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// isSensitiveOverwrite 判定：写类工具 + 目标为「已存在的敏感文件」→ 即使
// YOLO 模式也必须审批。apply_patch 的路径藏在 patch 头里，其余走通用参数。
func isSensitiveOverwrite(name, argsJSON string) bool {
	if !sensitiveWriteToolSet[name] {
		return false
	}
	var paths []string
	if name == "apply_patch" {
		paths = nativePatchHeaderPaths(argsJSON)
	} else {
		paths = toolPathArgs(argsJSON)
	}
	for _, p := range paths {
		if isSensitiveFile(p) && pathExists(absAgainstRoot(p)) {
			return true
		}
	}
	return false
}

// ---- 破坏性 shell 命令保护（YOLO 模式也强制审批）----
//
// 教训（仓库铁律）：git checkout -- / git restore 会把工作区整体覆盖回 HEAD，
// agent 一句命令就能抹掉用户全部未提交改动。这类命令 Ask 模式本来就因
// run_command 危险被拦，但 YOLO 模式畅通——必须单独点名拦截，与不可逆
// 删除同级。

// destructiveCommandPatterns 破坏性命令特征（命令转小写后匹配）。
// 覆盖工作区：git checkout --/.、git restore、git reset --hard、git clean
// 文件删除：git rm、rm -rf/-r、Remove-Item 递归、rd/rmdir /s、del /f|/s
// 远程强推：git push --force / -f
var destructiveCommandPatterns = []struct {
	label string
	re    *regexp.Regexp
}{
	{"git checkout 恢复工作区", regexp.MustCompile(`git\s+checkout\s+--`)},
	{"git checkout 点(整个目录)", regexp.MustCompile(`git\s+checkout\s+\.`)},
	{"git restore 覆盖工作区", regexp.MustCompile(`git\s+restore\b`)},
	{"git reset --hard", regexp.MustCompile(`git\s+reset\s+--hard`)},
	{"git clean 清除文件", regexp.MustCompile(`git\s+clean\b`)},
	{"git rm 删除", regexp.MustCompile(`git\s+rm\b`)},
	{"rm 强制/递归删除", regexp.MustCompile(`\brm\s+(?:-{1,2}[a-z]*[rf][a-z]*|--recursive|--force)\b`)},
	{"Remove-Item 递归删除", regexp.MustCompile(`remove-item[^\n]*-(?:r|recurse|force)` )},
	{"rd/rmdir 递归删除", regexp.MustCompile(`\b(?:rd|rmdir)\s+/s\b`)},
	{"del 强制/递归删除", regexp.MustCompile(`\bdel\s+/(?:f|s|q)` )},
	{"git push 强推", regexp.MustCompile(`git\s+push\s+[^\n]*?(?:--force|-f\b)`)},
}

// isDestructiveCommand 判定一条 shell 命令是否含破坏性操作（大小写不敏感）。
func isDestructiveCommand(command string) bool {
	lower := strings.ToLower(command)
	for _, p := range destructiveCommandPatterns {
		if p.re.MatchString(lower) {
			return true
		}
	}
	return false
}

// isDestructiveToolCall 判定一次工具调用是否为破坏性 shell 命令
// （run_command / mcp__shell__run 专用，YOLO 模式也强制审批）。
func isDestructiveToolCall(name, argsJSON string) bool {
	if name != "run_command" && name != "mcp__shell__run" {
		return false
	}
	var args struct {
		Command string `json:"command"`
	}
	if json.Unmarshal([]byte(argsJSON), &args) != nil {
		return false
	}
	return isDestructiveCommand(args.Command)
}

// ---- 工作目录越界判定 ----
//
// 用户额外接入的 filesystem MCP 底层会放开可达根目录（见 mcp_client.go
// fsAllowedDirs），「能不能碰工作目录以外的文件」由这里统一判断：
// Ask 模式弹确认，Yolo 模式直接放行。

// toolPathArgs 从工具参数 JSON 里挑出「看起来是文件路径」的字段值。
// 覆盖 Go 内置文件工具和外部 MCP 常见的 path / source / destination / paths[]。
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
	if patch, ok := m["patch"].(string); ok {
		for _, path := range nativePatchHeaderPaths(patch) {
			add(path)
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
