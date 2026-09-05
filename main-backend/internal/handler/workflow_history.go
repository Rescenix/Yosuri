package handler

import (
	"fmt"
	"strings"
)

const workflowHistorySummaryMaxChars = 8000

func workflowHistoryContent(status, final string, transcript []string, blocks []FlowBlock) string {
	final = strings.TrimSpace(final)
	if final == "" {
		for i := len(blocks) - 1; i >= 0; i-- {
			if blocks[i].Type == "intent" && strings.TrimSpace(blocks[i].Text) != "" {
				final = strings.TrimSpace(blocks[i].Text)
				break
			}
		}
	}
	if final == "" {
		switch status {
		case taskStatusCompleted:
			final = "工作流已完成。"
		case taskStatusFailed:
			final = "工作流执行失败。"
		default:
			final = "工作流在完成前中断。"
		}
	}

	// 成功任务保留最终答复即可；失败/中断额外带上最后若干条工具摘要，
	// 下一轮模型才能知道哪些操作已经做过，避免从零猜测或重复修改。
	if status != taskStatusCompleted && len(transcript) > 0 {
		const maxSteps = 16
		start := 0
		if len(transcript) > maxSteps {
			start = len(transcript) - maxSteps
		}
		final += "\n\n中断前已执行步骤摘要：\n- " + strings.Join(transcript[start:], "\n- ")
	}
	return truncateChars(final, workflowHistorySummaryMaxChars)
}

func (r *WorkflowRunner) persistWorkflowHistory(
	sessionID, workflowID, task, status, final, model string,
	transcript []string, blocks []FlowBlock, inTok, outTok int,
	agentID string,
) {
	if sessionID == "" || workflowID == "" || r.chatHandler == nil || r.chatHandler.sessionStore == nil {
		return
	}
	switch status {
	case taskStatusCompleted, taskStatusFailed, taskStatusInterrupted:
	default:
		status = taskStatusInterrupted
	}
	content := workflowHistoryContent(status, final, transcript, blocks)
	r.chatHandler.sessionStore.UpsertWorkflowPair(
		sessionID,
		workflowID,
		DSMessage{
			Role: "user", Content: task, Status: status, WorkflowID: workflowID,
		},
		DSMessage{
			Role: "assistant", Content: content, Model: model, Blocks: blocks, WorkflowID: workflowID,
			TokenUsage: inTok + outTok, Agent: agentID,
		},
	)
	// Extraction is asynchronous and sees both the user's completed task text and
	// the recent dialog around it, so the resulting picture includes style/preference
	// signals that only appear in how the user interacts, not in the bare task string.
	if status == taskStatusCompleted {
		enqueueAutomaticMemory(workflowID, task, recentDialogContext(r.chatHandler.sessionStore, sessionID))
	}
}

// recentDialogContext 拼一段最近对话（用户+助手各侧）供自动画像提取。
// 只取末尾若干条、单条截断，控制喂给提取模型的 token 量；拿不到就返回空。
func recentDialogContext(store *SessionStore, sessionID string) string {
	if store == nil {
		return ""
	}
	msgs := store.Get(sessionID)
	const maxPairs = 8
	if len(msgs) > maxPairs*2 {
		msgs = msgs[len(msgs)-maxPairs*2:]
	}
	var b strings.Builder
	for _, m := range msgs {
		text := strings.TrimSpace(m.Content)
		if text == "" {
			continue
		}
		if len(text) > 500 {
			text = text[:500] + "…"
		}
		fmt.Fprintf(&b, "[%s] %s\n", m.Role, text)
	}
	return b.String()
}

func workflowFailureText(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "任务执行失败。"
	}
	return fmt.Sprintf("任务执行失败：%s", reason)
}