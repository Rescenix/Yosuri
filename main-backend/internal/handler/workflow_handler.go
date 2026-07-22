package handler

// WorkflowRunner 工作流编排执行器
type WorkflowRunner struct {
	chatHandler *ChatHandler
}

const estimatedContextWindow = 128000

// maxHistoryMessages 是历史窗口的保守兜底值，仅在拿不到模型 context_window 时使用
// （见 agent_workflow_handler.go 的 historyLimitFor）。
const maxHistoryMessages = 10

func estimateTokenCount(s string) int {
	return len(s) / 4
}

func NewWorkflowRunner(chatHandler *ChatHandler) *WorkflowRunner {
	return &WorkflowRunner{chatHandler: chatHandler}
}
