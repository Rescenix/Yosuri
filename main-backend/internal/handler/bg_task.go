package handler

// 后台任务（Hermes 式）：run_task 启动一条命令后立即返回 task_id，不阻塞工作流；
// 进程退出时通过通知通道唤醒所属工作流（agent 主循环收到后注入完成消息继续跑）。
//
// 与 terminal_handler.go 的交互式终端区分：那是 UI 面向的常驻 shell（SSE + stdin），
// 这是 agent 面向的一次性命令托管（poll/log/wait/kill + 完成通知）。
//
// 设计对齐 tools/process_registry.py（Hermes）：
//   - spawn 立即返回 session id；reader goroutine 增量读（read1 语义）追加滚动缓冲
//   - 完成时推 completion 事件 → 主循环 drain 后注入新一轮
//   - Windows 树杀 taskkill /T /F（同 browser_preview_windows.go 模式）

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"backend/internal/ai/core"
)

// 通知通道注册表：workflow_id → 完成通知通道。工作流循环在启动时注册、
// 收尾时注销（同 steer/ask 通道模式）。run_task 按属主 workflow 找到通道，
// 进程退出时推一条 bgTaskResult，主循环 select 到后注入新一轮。
var (
	bgNotifyMu    sync.Mutex
	bgNotifyChans = map[string]chan<- bgTaskResult{}
)

func registerBgNotify(workflow string) chan bgTaskResult {
	ch := make(chan bgTaskResult, 16) // 缓冲：任务完成时主循环可能正阻塞在模型请求上
	bgNotifyMu.Lock()
	bgNotifyChans[workflow] = ch
	bgNotifyMu.Unlock()
	return ch
}

func unregisterBgNotify(workflow string) {
	bgNotifyMu.Lock()
	delete(bgNotifyChans, workflow)
	bgNotifyMu.Unlock()
}

// bgNotifyChanFor 返回属主工作流的通知通道（可能为 nil = 非工作流上下文/已注销）。
func bgNotifyChanFor(workflow string) chan<- bgTaskResult {
	if workflow == "" {
		return nil
	}
	bgNotifyMu.Lock()
	defer bgNotifyMu.Unlock()
	return bgNotifyChans[workflow]
}

// hasPendingBgTasks 该工作流是否还有未完成的后台任务（收尾点据此决定暂停还是真正结束）。
func hasPendingBgTasks(workflow string) bool {
	return pendingBgTaskCount(workflow) > 0
}

// bgSweepFinishedLocked 清掉已结束且超过 TTL 的任务（调用方需持 bgTasksMu）。
// 防止 map 撑到上限后新任务被拒；Hermes 同款 FINISHED_TTL 语义。
func bgSweepFinishedLocked() {
	now := time.Now()
	for id, t := range bgTasks {
		t.mu.Lock()
		exited, startedAt := t.exited, t.startedAt
		t.mu.Unlock()
		if exited && now.Sub(startedAt) > bgTaskTTL {
			delete(bgTasks, id)
		}
	}
}

// pendingBgTaskCount 该工作流未完成的后台任务数。
func pendingBgTaskCount(workflow string) int {
	if workflow == "" {
		return 0
	}
	bgTasksMu.Lock()
	defer bgTasksMu.Unlock()
	n := 0
	for _, t := range bgTasks {
		if t.workflow == workflow {
			t.mu.Lock()
			exited := t.exited
			t.mu.Unlock()
			if !exited {
				n++
			}
		}
	}
	return n
}

func unmarshalToolArgs(argsJSON string, v any) error {
	return json.Unmarshal([]byte(defaultJSONObject(argsJSON)), v)
}

func jsonOf(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("序列化失败: %v", err)
	}
	return string(b)
}

// bgWorkflowCtxKey 把当前 workflow_id 塞进 request context，
// 让工具执行链（executeCodeCalls → callNativeTool → callBgTaskTool）能拿到属主工作流。
type bgWorkflowCtxKey struct{}

func withWorkflowID(ctx context.Context, workflow string) context.Context {
	return context.WithValue(ctx, bgWorkflowCtxKey{}, workflow)
}

func workflowIDFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(bgWorkflowCtxKey{}).(string); ok {
		return v
	}
	return ""
}

// bgTaskResult 一次后台任务的完成通知。
type bgTaskResult struct {
	TaskID   string `json:"task_id"`
	Command  string `json:"command"`
	ExitCode int    `json:"exit_code"`
	Output   string `json:"output"` // 输出尾部（完成通知用，完整日志走 task_log）
}

// bgTask 一个被托管的后台命令。
type bgTask struct {
	id        string
	command   string
	workflow  string // 属主 workflow_id，完成通知按它路由
	cmd       *exec.Cmd
	pid       int
	startedAt time.Time

	mu         sync.Mutex
	output     []byte
	truncated  bool
	exited     bool
	exitCode   int
	killReason string

	done chan struct{} // 进程退出后关闭（task_wait 阻塞点）
	ch   chan<- bgTaskResult
}

const (
	bgTaskOutputCap = 256 * 1024 // 滚动输出上限（256KB），超出丢最旧
	bgTaskMaxTasks  = 32         // 同时跟踪的上限，超出拒绝新任务
	bgTaskTTL       = 30 * time.Minute // 已结束任务的保留时长（对齐 Hermes FINISHED_TTL）
)

var (
	bgTasksMu sync.Mutex
	bgTasks   = map[string]*bgTask{}
)

// startBgTask 启动一个后台命令并立即返回 task_id（不等待进程）。
// workflow 为属主工作流；ch 非 nil 时进程退出会推一条 bgTaskResult（Hermes notify_on_complete）。
func startBgTask(workflow, command string, ch chan<- bgTaskResult) (string, error) {
	if strings.TrimSpace(command) == "" {
		return "", fmt.Errorf("command 不能为空")
	}
	bgTasksMu.Lock()
	bgSweepFinishedLocked() // 先清掉过期已结束任务，给新任务腾位子
	if len(bgTasks) >= bgTaskMaxTasks {
		bgTasksMu.Unlock()
		return "", fmt.Errorf("后台任务数已达上限 %d，请先 task_kill 清理", bgTaskMaxTasks)
	}
	bgTasksMu.Unlock()

	task := &bgTask{
		id:        fmt.Sprintf("task_%d", time.Now().UnixNano()),
		command:   command,
		workflow:  workflow,
		startedAt: time.Now(),
		done:      make(chan struct{}),
		ch:        ch,
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", command)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	} else {
		cmd = exec.Command("/bin/sh", "-lc", command)
	}
	cmd.Dir = core.GetProjectRoot()
	cmd.Stdout = task
	cmd.Stderr = task
	task.cmd = cmd

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("启动后台任务失败: %w", err)
	}
	task.pid = cmd.Process.Pid

	bgTasksMu.Lock()
	bgTasks[task.id] = task
	bgTasksMu.Unlock()

	// 等待 + 收尾：进程退出 → 记退出码 → 关 done → 推完成通知
	go func() {
		err := cmd.Wait()
		exitCode := 0
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				exitCode = ee.ExitCode()
			} else {
				exitCode = -1
			}
		}
		task.mu.Lock()
		task.exited = true
		task.exitCode = exitCode
		task.mu.Unlock()
		close(task.done)
		if task.ch != nil {
			// 非阻塞投递：工作流已结束/收尾点没在等时，通知宁可丢弃也不让 goroutine 卡死
			// （缓冲 16 条通常够，极端情况丢一条完成通知可接受——agent 还能 task_status 自查）。
			select {
			case task.ch <- bgTaskResult{
				TaskID:   task.id,
				Command:  task.command,
				ExitCode: exitCode,
				Output:   task.outputTail(2000),
			}:
			default:
			}
		}
	}()

	return task.id, nil
}

// Write 实现 io.Writer：reader 增量写滚动缓冲（直接当 cmd.Stdout/Stderr，无需额外 goroutine）。
func (t *bgTask) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.output = append(t.output, p...)
	if len(t.output) > bgTaskOutputCap {
		t.output = t.output[len(t.output)-bgTaskOutputCap:]
		t.truncated = true
	}
	return len(p), nil
}

func (t *bgTask) outputTail(n int) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := string(t.output)
	if len(out) > n {
		return "…（输出过长已截断）\n" + out[len(out)-n:]
	}
	return out
}

func (t *bgTask) snapshot() (string, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.output), t.truncated
}

func bgTaskStatus(taskID string) map[string]any {
	bgTasksMu.Lock()
	t := bgTasks[taskID]
	bgTasksMu.Unlock()
	if t == nil {
		return map[string]any{"status": "not_found", "error": "任务不存在或已过期: " + taskID}
	}
	t.mu.Lock()
	exited, exitCode := t.exited, t.exitCode
	t.mu.Unlock()
	out, truncated := t.snapshot()
	r := map[string]any{
		"task_id":    t.id,
		"command":    t.command,
		"status":     "running",
		"pid":        t.pid,
		"uptime_seconds": int(time.Since(t.startedAt).Seconds()),
		"output_preview": out,
		"output_truncated": truncated,
	}
	if exited {
		r["status"] = "exited"
		r["exit_code"] = exitCode
	}
	return r
}

func bgTaskLog(taskID string, offset, limit int) map[string]any {
	bgTasksMu.Lock()
	t := bgTasks[taskID]
	bgTasksMu.Unlock()
	if t == nil {
		return map[string]any{"status": "not_found", "error": "任务不存在或已过期: " + taskID}
	}
	out, _ := t.snapshot()
	lines := strings.Split(out, "\n")
	total := len(lines)
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 200
	}
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	t.mu.Lock()
	exited, exitCode := t.exited, t.exitCode
	t.mu.Unlock()
	r := map[string]any{
		"task_id":     t.id,
		"command":     t.command,
		"status":      "running",
		"output":      strings.Join(lines[offset:end], "\n"),
		"total_lines": total,
		"showing":     fmt.Sprintf("%d-%d/%d 行", offset+1, end, total),
	}
	if exited {
		r["status"] = "exited"
		r["exit_code"] = exitCode
	}
	return r
}

// killBgTask 树杀：Windows 用 taskkill /T /F（同 browser_preview_windows.go），
// 避免只杀 powershell 父进程、子进程泄漏成孤儿。
func killBgTask(taskID string) map[string]any {
	bgTasksMu.Lock()
	t := bgTasks[taskID]
	bgTasksMu.Unlock()
	if t == nil {
		return map[string]any{"status": "not_found", "error": "任务不存在或已过期: " + taskID}
	}
	t.mu.Lock()
	exited := t.exited
	t.mu.Unlock()
	if exited {
		return map[string]any{"status": "already_exited", "task_id": taskID}
	}
	if t.cmd != nil && t.cmd.Process != nil {
		if runtime.GOOS == "windows" {
			exec.Command("taskkill", "/PID", fmt.Sprintf("%d", t.cmd.Process.Pid), "/T", "/F").Run()
		} else {
			_ = t.cmd.Process.Kill()
		}
	}
	t.mu.Lock()
	t.killReason = "task_kill"
	t.mu.Unlock()
	return map[string]any{"status": "killed", "task_id": taskID}
}

// waitBgTask 阻塞等任务完成，超时返回。timeout<=0 时用默认 180s（对齐 Hermes 钳制语义）。
func waitBgTask(taskID string, timeout int) map[string]any {
	bgTasksMu.Lock()
	t := bgTasks[taskID]
	bgTasksMu.Unlock()
	if t == nil {
		return map[string]any{"status": "not_found", "error": "任务不存在或已过期: " + taskID}
	}
	if timeout <= 0 {
		timeout = 180
	}
	if timeout > 600 {
		timeout = 600
	}
	select {
	case <-t.done:
		t.mu.Lock()
		code := t.exitCode
		t.mu.Unlock()
		return map[string]any{"status": "exited", "task_id": taskID, "exit_code": code, "output": t.outputTail(2000)}
	case <-time.After(time.Duration(timeout) * time.Second):
		return map[string]any{"status": "timeout", "task_id": taskID, "timeout": timeout}
	}
}

// bgTaskDoneMessage 构造注入模型历史的完成通知消息（Hermes completion 注入同款语义）。
func bgTaskDoneMessage(res bgTaskResult) map[string]any {
	return map[string]any{
		"role":    "system",
		"content": fmt.Sprintf("[后台任务 %s 完成] 命令：%s\n退出码：%d\n输出尾部：\n%s", res.TaskID, res.Command, res.ExitCode, res.Output),
	}
}

// bgTaskDonePayload 构造推给前端的完成事件负载。
func bgTaskDonePayload(res bgTaskResult) map[string]any {
	return map[string]any{
		"task_id":   res.TaskID,
		"command":   res.Command,
		"exit_code": res.ExitCode,
		"output":    truncateChars(res.Output, 2000),
	}
}

// callBgTaskTool 分发 run_task / task_status / task_log / task_wait / task_kill。
// workflow 为空（非工作流上下文）时 run_task 仍可用，只是没有完成通知。
func callBgTaskTool(ctx context.Context, name, argsJSON string, workflow string) (nativeToolResult, error) {
	var args struct {
		Command string `json:"command"`
		TaskID  string `json:"task_id"`
		Timeout int    `json:"timeout"`
		Offset  int    `json:"offset"`
		Limit   int    `json:"limit"`
	}
	if err := unmarshalToolArgs(argsJSON, &args); err != nil {
		return nativeToolResult{}, err
	}
	switch name {
	case "run_task":
		var ch chan<- bgTaskResult
		if workflow != "" {
			ch = bgNotifyChanFor(workflow)
		}
		id, err := startBgTask(workflow, args.Command, ch)
		if err != nil {
			return nativeToolResult{}, err
		}
		return nativeToolResult{Text: fmt.Sprintf("后台任务已启动，task_id=%s（不阻塞当前工作流）。用 task_status 查状态、task_log 看输出、task_wait 阻塞等结果、task_kill 终止。", id)}, nil
	case "task_status":
		return nativeToolResult{Text: jsonOf(bgTaskStatus(args.TaskID))}, nil
	case "task_log":
		return nativeToolResult{Text: jsonOf(bgTaskLog(args.TaskID, args.Offset, args.Limit))}, nil
	case "task_wait":
		return nativeToolResult{Text: jsonOf(waitBgTask(args.TaskID, args.Timeout))}, nil
	case "task_kill":
		return nativeToolResult{Text: jsonOf(killBgTask(args.TaskID))}, nil
	}
	return nativeToolResult{}, fmt.Errorf("未知的后台任务工具: %s", name)
}
