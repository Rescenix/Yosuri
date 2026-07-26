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
	cmd := exec.CommandContext(ctx, name, args...)
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
