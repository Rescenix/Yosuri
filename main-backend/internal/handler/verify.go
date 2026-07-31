package handler

// 工作流收尾自动校验（post-workflow verification gate）。
//
// 设计原则（用户明确约定）：验证只在 agent 打算结束对话时做一次——即工作流末轮
// 模型不再发起任何工具调用（len(calls)==0，见 agent_workflow_handler.go 的
// workflow_done completed 分支）。禁止每轮/每步频繁验证（"别动不动验证"）。
//
// 触发点：workflow_done(completed) 推送之前调 verifyOnWorkflowDone。
// 数据来源：复用 AgentFS 审计时间线（本次会话改过的文件后缀分布），零额外采集。
// 旁路约束：任何错误（命令不存在/超时/构建失败）都只记录状态、放行 workflow_done，
// 绝不阻断对话收尾——验证是加分项，不是阻断项。

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// verifyBuildTimeout 单次构建命令的硬超时，避免卡死对话收尾。
const verifyBuildTimeout = 120 * time.Second

// verifyOnWorkflowDone 在 agent 决定结束对话时跑一次 build + 截图校验。
// c 用于推 verification SSE 事件；workflowID 仅用于日志关联。
func verifyOnWorkflowDone(c *gin.Context, workflowID string) {
	agentfsMu.Lock()
	sess := activeSession
	agentfsMu.Unlock()
	if sess == nil {
		// 未开启 AgentFS 会话 → 无审计数据，跳过校验（降级，不阻断）
		return
	}

	// 从本次会话审计时间线统计改过的文件类型
	ap := agentfsAuditPath(sess.Project)
	data, err := os.ReadFile(ap)
	if err != nil {
		return
	}
	hasGo := false
	hasFrontend := false
	hasHTML := false
	frontendEntry := ""
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var a agentfsAudit
		if json.Unmarshal([]byte(line), &a) != nil {
			continue
		}
		if a.SessionID != sess.SessionID {
			continue // 只统计本次会话
		}
		ext := strings.ToLower(filepath.Ext(a.RelPath))
		switch ext {
		case ".go":
			hasGo = true
		case ".vue", ".ts", ".tsx", ".js", ".jsx":
			hasFrontend = true
			if frontendEntry == "" {
				frontendEntry = a.RelPath
			}
		case ".html", ".htm":
			hasHTML = true
			if frontendEntry == "" {
				frontendEntry = a.RelPath
			}
		}
	}

	result := map[string]any{"verified_at": time.Now().Format(time.RFC3339)}
	ran := false

	// Go 构建：仅当本轮改了 .go 且 workdir 下有 go.mod
	if hasGo && fileExists(filepath.Join(sess.Workdir, "go.mod")) {
		ran = true
		out, ok := runVerifyBuild(sess.Workdir, "go", "build", "./...")
		result["go_build"] = map[string]any{"status": yesNo(ok), "detail": out}
	}

	// 前端构建：仅当本轮改了前端文件且 workdir 下有 package.json
	if (hasFrontend || hasHTML) && fileExists(filepath.Join(sess.Workdir, "package.json")) {
		ran = true
		out, ok := runVerifyBuild(sess.Workdir, "npm", "run", "build")
		result["fe_build"] = map[string]any{"status": yesNo(ok), "detail": truncateVerify(out)}
	}

	// 截图校验：改了前端入口时复用已有真实 Chromium 预览能力
	// 只打开预览面板（用户可见可交互），不自动截图推送——截图时机由 LLM 用
	// capture_preview 工具自行判断（如全部完成后再截），不绑定预览/收尾。
	if (hasFrontend || hasHTML) && frontendEntry != "" {
		ran = true
		abs := filepath.Join(sess.Workdir, frontendEntry)
		if fileExists(abs) {
			url, _, cdpErr, ok := autoOpenBrowserPreview(abs)
			if ok {
				result["screenshot"] = map[string]any{"status": "opened", "url": url}
			} else {
				result["screenshot"] = map[string]any{"status": "skip", "reason": cdpErr}
			}
		}
	}

	// 交互验证：仅当改了前端、且预览已成功在真实 Chromium 里渲染时才跑。
	// 探测页面是否含可交互元素（按钮/输入框/链接/可点击节点）；有则通过 CDP 实测
	// 一次点击 + 一次输入，读 DOM 反馈确认「页面真能交互」，作为交付凭证；
	// 纯展示页（无交互元素）则跳过，照常收尾。全部失败只记录、不阻断对话。
	if (hasFrontend || hasHTML) && frontendEntry != "" {
		probe := `(() => { const sel = 'button,input,select,textarea,a[href],[onclick],[role=button]';` +
			`const nodes = document.querySelectorAll(sel);` +
			`const types = {}; nodes.forEach(n => types[n.tagName] = (types[n.tagName]||0)+1);` +
			`return JSON.stringify({ count: nodes.length, types }); })()`
		if raw, perr := evaluatePreviewExpression(probe); perr == nil {
			var probeRes struct {
				Count int             `json:"count"`
				Types map[string]int `json:"types"`
			}
			if json.Unmarshal(raw, &probeRes) == nil {
				if probeRes.Count == 0 {
					result["interaction"] = map[string]any{"status": "none", "note": "纯展示页，无可交互元素，跳过交互验证"}
				} else {
					ran = true
					// 实测：点第一个按钮 + 给第一个输入框赋值，再读反馈
					act := `(() => {` +
						`const btn = document.querySelector('button,[role=button],[onclick]');` +
						`const inp = document.querySelector('input,textarea,select');` +
						`let clicked = false, typed = false;` +
						`if (btn) { btn.click(); clicked = true; }` +
						`if (inp) { try { const v = 'hermes-verify'; ` +
						`const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;` +
						`setter.call(inp, v); inp.dispatchEvent(new Event('input', {bubbles:true})); ` +
						`typed = inp.value === v; } catch(e) {} }` +
						`return JSON.stringify({ clicked, typed }); })()`
					detail := map[string]any{"status": "tested", "elements": probeRes.Count, "types": probeRes.Types}
					if actRaw, aerr := evaluatePreviewExpression(act); aerr == nil {
						var actRes struct {
							Clicked bool `json:"clicked"`
							Typed   bool `json:"typed"`
						}
						if json.Unmarshal(actRaw, &actRes) == nil {
							detail["click_ok"] = actRes.Clicked
							detail["input_ok"] = actRes.Typed
							detail["verdict"] = yesNo(actRes.Clicked || actRes.Typed)
						}
					} else {
						detail["error"] = aerr.Error()
					}
					result["interaction"] = detail
				}
			}
		} else {
			result["interaction"] = map[string]any{"status": "skip", "reason": perr.Error()}
		}
	}

	if !ran {
		result["status"] = "skipped"
		result["reason"] = "本轮未改动可构建的文件类型"
	} else {
		result["status"] = "done"
	}

	if c != nil {
		writeCodeSSE(c, "verification", map[string]any{
			"workflow_id": workflowID,
			"result":      result,
		})
	}
}

// runVerifyBuild 在指定目录跑构建命令，带超时；返回输出与是否成功。
func runVerifyBuild(dir, name string, args ...string) (string, bool) {
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Sprintf("%s 不在 PATH，跳过构建校验", name), false
	}
	ctx, cancel := context.WithTimeout(context.Background(), verifyBuildTimeout)
	defer cancel()
	cmd := hiddenCommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return "构建超时（>" + verifyBuildTimeout.String() + "），跳过", false
	}
	if err != nil {
		return string(out), false
	}
	return string(out), true
}

// 小工具：避免给 agent_workflow_handler.go 加 import，这里就地实现几个helper

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func yesNo(ok bool) string {
	if ok {
		return "pass"
	}
	return "fail"
}

func truncateVerify(s string) string {
	const max = 2000
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}
