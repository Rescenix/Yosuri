// handler/chat_utils.go - 对话历史工具函数

package handler

// 任务生命周期状态（DSMessage.Status，只对 role=="user" 有意义）。
const (
	taskStatusCompleted = "completed"
)

// 历史任务的呈现前缀。
//
// 为什么需要它：历史里的 user 消息是纯指令原文，跟"当前这条任务"长得一模一样。
// 模型看到的是一串看起来都还没做的祈使句，收尾时注意力一发散，就会回头去执行
// 上一个甚至上上个任务（实测发生过）。而对应的 assistant 回复只有最终那段文本，
// 工具轨迹（Blocks）根本不会发给模型，所以它也无从判断"这条当时到底干没干活"。
//
// 加一个显式标记，把"已结题"这件事从模型的推断变成系统的断言。
const completedTaskPrefix = "[历史任务·已完成，仅供参考，不要重新执行] "

// historyContractPrompt 把上面那个前缀的含义作为系统契约讲清楚。
// 只有标记没有契约的话，模型仍可能把前缀当噪音略过去。
const historyContractPrompt = `
━━━ 对话历史的读法 ━━━
带「` + completedTaskPrefix + `」前缀的用户消息是**已经做完并结题**的历史任务，
它们留在上下文里只为提供背景（改过哪些文件、做过什么决定）。
不要重新执行它们，也不要把它们里面的待办当成你现在要做的事。
本次唯一需要执行的任务，是整段对话**最后一条**没有该前缀的用户消息。
如果历史任务确实有遗留问题需要处理，等用户明确提出，不要自作主张回头补做。
`

// taskDone 判断一条历史消息是不是"已经结题的任务"。
//
// 空 Status 一律按已完成处理：历史落盘只发生在工作流成功收尾时
// （agent_workflow_handler.go 里全仓库唯一的 Append 调用点），所以存量老数据
// 按定义也全是已完成的，没有"空=未知"这种中间态。
func taskDone(m DSMessage) bool {
	return m.Role == "user" && (m.Status == "" || m.Status == taskStatusCompleted)
}

// 截断历史消息
func truncateHistory(history []DSMessage, maxHistory int) []DSMessage {
	if len(history) > maxHistory {
		return history[len(history)-maxHistory:]
	}
	return history
}

// 构建消息列表。history 里的已完成任务会被打上前缀，与末尾这条真正待办的
// userMessage 区分开——后者是唯一需要执行的任务。
func buildChatMessages(systemPrompt string, history []DSMessage, userMessage string) []map[string]string {
	msgs := []map[string]string{
		{"role": "system", "content": systemPrompt},
	}
	for _, msg := range history {
		content := msg.Content
		if taskDone(msg) && content != "" {
			content = completedTaskPrefix + content
		}
		msgs = append(msgs, map[string]string{"role": msg.Role, "content": content})
	}
	msgs = append(msgs, map[string]string{"role": "user", "content": userMessage})
	return msgs
}
