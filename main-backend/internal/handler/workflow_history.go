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
	transcript []string, blocks []FlowBlock,
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
		},
	)
	// Extraction is deliberately asynchronous and only sees the user's completed
	// task text; assistant prose is not treated as a source of user facts.
	if status == taskStatusCompleted {
		enqueueAutomaticMemory(workflowID, task)
	}
}

func workflowFailureText(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "任务执行失败。"
	}
	return fmt.Sprintf("任务执行失败：%s", reason)
}
