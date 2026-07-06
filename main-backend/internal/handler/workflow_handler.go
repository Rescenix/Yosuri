package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"backend/internal/agent"

	"github.com/gin-gonic/gin"
)

// WorkflowRunner 工作流编排执行器
type WorkflowRunner struct {
	chatHandler *ChatHandler
}

const estimatedContextWindow = 128000

func estimateTokenCount(s string) int {
	return len(s) / 4
}

func NewWorkflowRunner(chatHandler *ChatHandler) *WorkflowRunner {
	return &WorkflowRunner{chatHandler: chatHandler}
}

// checkIntent 快速判断用户输入是闲聊还是任务
func (r *WorkflowRunner) checkIntent(c *gin.Context, userMessage string) (isTask bool, reply string) {
	systemPrompt := `你是一个意图分类器。只分析用户的话，不做任何额外操作。

判断规则：
- 如果用户的话是问候、闲聊、提问、情感表达等，属于"chat"
- 如果用户的话要求写代码、修改文件、创建项目、调试、重构等，属于"task"

必须只输出JSON，不要任何其他文字：
{"type":"chat"或"task","reply":"如果是chat，给出简短友好的中文回复；如果是task，回复空字符串"}`

	sessionID := fmt.Sprintf("intent_%d", time.Now().UnixNano())
	content, _, _, err := r.chatHandler.resolveCloudConversation(
		c, systemPrompt, userMessage, sessionID,
		0.1, 0.9, 256, "",
	)
	if err != nil {
		return true, ""
	}

	var result struct {
		Type  string `json:"type"`
		Reply string `json:"reply"`
	}

	if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &result); err != nil {
		return true, ""
	}

	if result.Type == "chat" {
		return false, result.Reply
	}
	return true, ""
}

// callLLMOnce 调用一次 LLM，流式返回内容，不做工具循环
// messages 是不含 system 的对话历史（如 [{"role":"user","content":"..."}]）
func (r *WorkflowRunner) callLLMOnce(c *gin.Context, systemPrompt string, messages []map[string]interface{}, temperature, topP float64, maxTokens int) (string, error) {
	apiKey := os.Getenv("CLOUD_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("缺少 Cloud API Key (CLOUD_API_KEY)")
	}
	model := os.Getenv("CLOUD_MODEL")
	if model == "" {
		model = "qwen3-coder:480b-cloud"
	}

	// 构建完整消息：system + 对话历史
	var apiMessages []map[string]interface{}
	apiMessages = append(apiMessages, map[string]interface{}{"role": "system", "content": systemPrompt})
	apiMessages = append(apiMessages, messages...)

	reqBody := map[string]interface{}{
		"model":    model,
		"messages": apiMessages,
		"stream":   true,
		"options": map[string]interface{}{
			"temperature": temperature,
			"top_p":       topP,
		},
	}
	if maxTokens > 0 {
		reqBody["max_tokens"] = maxTokens
	}

	content, _, _, err := r.chatHandler.sendCloudStream(c, reqBody, apiKey)
	return content, err
}

// HandleWorkflowRun POST /api/workflow/run — SSE 流式 + Tool Loop 迭代执行
func (r *WorkflowRunner) HandleWorkflowRun(c *gin.Context) {
	var req agent.WorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	// 设置 SSE 头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	workflowID := agent.NewWorkflowID()

	// ★ 第一层：轻量级意图判断（闲聊直接拦截）
	isTask, chatReply := r.checkIntent(c, req.Task)
	if !isTask {
		rawWriteSSE(c, "workflow_start", "workflow_start", map[string]string{
			"workflow_id": workflowID,
			"task":        req.Task,
			"total_steps": "0",
		})
		c.Writer.Flush()

		rawWriteSSE(c, "workflow_done", "workflow_done", map[string]string{
			"workflow_id":  workflowID,
			"status":       string(agent.WorkflowCompleted),
			"final_output": chatReply,
			"total_time":   "0ms",
		})
		c.Writer.Flush()
		return
	}

	// ★ 第二层：Tool Loop 迭代执行
	agt := agent.MainAgentConfig()
	systemPrompt := agt.SystemPrompt

	// 对话历史（不含 system，callLLMOnce 里面会加上）
	messages := []map[string]interface{}{
		{"role": "user", "content": req.Task},
	}

	cumulativeInputTokens := estimateTokenCount(systemPrompt + req.Task)
	cumulativeOutputTokens := 0
	iterationCount := 0

	// 发送 workflow_start
	rawWriteSSE(c, "workflow_start", "workflow_start", map[string]string{
		"workflow_id": workflowID,
		"task":        req.Task,
		"total_steps": "1",
	})
	c.Writer.Flush()

	for {
		if c.Request.Context().Err() != nil {
			break // 客户端断开
		}

		iterationCount++
		stepID := agent.NewStepID()

		// ── step_start ──
		rawWriteSSE(c, "step_start", "step_start", map[string]string{
			"step_id":    stepID,
			"agent":      "main",
			"agent_role": "主控Agent",
			"step_index": fmt.Sprintf("%d", iterationCount-1),
			"prompt":     req.Task,
		})
		c.Writer.Flush()

		// ── 调用 LLM（流式 content 事件已在此过程中发出） ──
		content, err := r.callLLMOnce(c, systemPrompt, messages, agt.Temp, agt.TopP, 4096)
		outputTokens := estimateTokenCount(content)
		cumulativeOutputTokens += outputTokens

		if err != nil {
			// 本轮 LLM 调用失败：发 step_done + workflow_done
			ctxPct := float64(cumulativeInputTokens+cumulativeOutputTokens) / float64(estimatedContextWindow) * 100
			rawWriteSSE(c, "step_done", "step_done", map[string]string{
				"step_id":                  stepID,
				"agent":                    "main",
				"status":                   "failed",
				"content":                  err.Error(),
				"output_tokens":            fmt.Sprintf("%d", outputTokens),
				"cumulative_input_tokens":  fmt.Sprintf("%d", cumulativeInputTokens),
				"cumulative_output_tokens": fmt.Sprintf("%d", cumulativeOutputTokens),
				"context_window":           fmt.Sprintf("%d", estimatedContextWindow),
				"context_window_pct":       fmt.Sprintf("%.1f", ctxPct),
			})
			c.Writer.Flush()

			rawWriteSSE(c, "workflow_done", "workflow_done", map[string]string{
				"workflow_id":              workflowID,
				"status":                   string(agent.WorkflowFailed),
				"final_output":             fmt.Sprintf("任务执行失败: %s", err.Error()),
				"cumulative_input_tokens":  fmt.Sprintf("%d", cumulativeInputTokens),
				"cumulative_output_tokens": fmt.Sprintf("%d", cumulativeOutputTokens),
				"context_window":           fmt.Sprintf("%d", estimatedContextWindow),
				"context_window_pct":       fmt.Sprintf("%.1f", ctxPct),
			})
			c.Writer.Flush()
			return
		}

		// ── 检测是否包含工具调用 ──
		if tc, jsonStr, ok := parseToolCallFromText(content); ok {
			// 剥离 JSON 得到干净的叙述文本（用于 step 的 content 展示）
			cleanContent := stripToolJSON(content, jsonStr)

			// 发送 tool_call_start
			rawWriteSSE(c, "tool_call", "tool_call_start", map[string]string{
				"name": tc.Tool,
				"args": formatToolArgs(tc.Args),
			})
			c.Writer.Flush()

			// 执行工具（静默执行，不额外发 SSE）
			sessionID := "wf_" + workflowID
			resultContent, execErr := executeToolSilently(sessionID, *tc)
			if execErr != nil {
				rawWriteSSE(c, "tool_call", "tool_call_error", map[string]string{
					"name":  tc.Tool,
					"error": fmt.Sprintf("%v", execErr),
				})
				c.Writer.Flush()
				resultContent = fmt.Sprintf("工具执行失败: %v", execErr)
			} else {
				rawWriteSSE(c, "tool_call", "tool_call_result", map[string]string{
					"name":   tc.Tool,
					"result": resultContent,
				})
				c.Writer.Flush()
			}

			// 将本轮 assistant + tool 结果追加到对话历史
			messages = append(messages, map[string]interface{}{"role": "assistant", "content": content})
			messages = append(messages, map[string]interface{}{"role": "tool", "content": resultContent})
			cumulativeInputTokens += estimateTokenCount(content + resultContent)

			// 本轮 step_done：content 用剥离 JSON 后的干净文本
			ctxPct := float64(cumulativeInputTokens+cumulativeOutputTokens) / float64(estimatedContextWindow) * 100
			rawWriteSSE(c, "step_done", "step_done", map[string]string{
				"step_id":                  stepID,
				"agent":                    "main",
				"status":                   "completed",
				"content":                  cleanContent,
				"output_tokens":            fmt.Sprintf("%d", outputTokens),
				"cumulative_input_tokens":  fmt.Sprintf("%d", cumulativeInputTokens),
				"cumulative_output_tokens": fmt.Sprintf("%d", cumulativeOutputTokens),
				"context_window":           fmt.Sprintf("%d", estimatedContextWindow),
				"context_window_pct":       fmt.Sprintf("%.1f", ctxPct),
			})
			c.Writer.Flush()

			// ← 循环继续，让 LLM 看到工具执行结果后决定下一步
			continue
		}

		// ── 没有工具调用 → 这是最终回答 ──
		// Agent 最后一轮不带工具调用的输出本身就是给用户看的总结，不需要再额外
		// 发起一次 LLM 请求去"重新总结"一遍——那样只是白费一次 API 往返和 token。
		finalContent := strings.TrimSpace(content)
		finalOutput := finalContent

		// 最终 step_done
		ctxPct := float64(cumulativeInputTokens+cumulativeOutputTokens) / float64(estimatedContextWindow) * 100
		rawWriteSSE(c, "step_done", "step_done", map[string]string{
			"step_id":                  stepID,
			"agent":                    "main",
			"status":                   "completed",
			"content":                  finalContent,
			"output_tokens":            fmt.Sprintf("%d", outputTokens),
			"cumulative_input_tokens":  fmt.Sprintf("%d", cumulativeInputTokens),
			"cumulative_output_tokens": fmt.Sprintf("%d", cumulativeOutputTokens),
			"context_window":           fmt.Sprintf("%d", estimatedContextWindow),
			"context_window_pct":       fmt.Sprintf("%.1f", ctxPct),
		})
		c.Writer.Flush()

		// 最终 workflow_done
		rawWriteSSE(c, "workflow_done", "workflow_done", map[string]string{
			"workflow_id":              workflowID,
			"status":                   string(agent.WorkflowCompleted),
			"final_output":             finalOutput,
			"cumulative_input_tokens":  fmt.Sprintf("%d", cumulativeInputTokens),
			"cumulative_output_tokens": fmt.Sprintf("%d", cumulativeOutputTokens),
			"context_window":           fmt.Sprintf("%d", estimatedContextWindow),
			"context_window_pct":       fmt.Sprintf("%.1f", ctxPct),
		})
		c.Writer.Flush()
		return
	}
}

// HandleListWorkflows GET /api/workflows — 列出可用工作流
func (r *WorkflowRunner) HandleListWorkflows(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"workflows": agent.ListWorkflows(),
	})
}
