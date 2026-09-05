// handler/chat_utils.go - 对话历史工具函数

package handler

import "strings"

// 任务生命周期状态（DSMessage.Status，只对 role=="user" 有意义）。
const (
	taskStatusCompleted   = "completed"
	taskStatusFailed      = "failed"
	taskStatusInterrupted = "interrupted"
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
const failedTaskPrefix = "[历史任务·执行失败，仅供上下文，不要自行重试] "
const interruptedTaskPrefix = "[历史任务·已中断，仅供上下文，不要自行续跑] "
const unknownTaskPrefix = "[历史任务·状态未知，仅供上下文，不要自行执行] "

// historyContractPrompt 把上面那个前缀的含义作为系统契约讲清楚。
// 只有标记没有契约的话，模型仍可能把前缀当噪音略过去。
const historyContractPrompt = `
━━━ 对话历史的读法 ━━━
历史用户消息会带明确状态：
- 「` + completedTaskPrefix + `」表示已经完成；
- 「` + failedTaskPrefix + `」表示当时执行失败；
- 「` + interruptedTaskPrefix + `」表示当时被停止或连接中断。
这些消息只提供背景。无论状态如何，都不要自行重试、续跑或补做。
本次唯一需要执行的任务，是整段对话**最后一条没有历史状态前缀**的用户消息。
如果用户明确要求继续旧任务，再结合对应的失败/中断摘要处理。
`

// taskDone 判断一条历史消息是不是"已经结题的任务"。
//
// 空 Status 一律按已完成处理：历史落盘只发生在工作流成功收尾时
// （agent_workflow_handler.go 里全仓库唯一的 Append 调用点），所以存量老数据
// 按定义也全是已完成的，没有"空=未知"这种中间态。
func taskDone(m DSMessage) bool {
	return m.Role == "user" && (m.Status == "" || m.Status == taskStatusCompleted)
}

func historyTaskPrefix(m DSMessage) string {
	if m.Role != "user" {
		return ""
	}
	switch m.Status {
	case "", taskStatusCompleted:
		return completedTaskPrefix
	case taskStatusFailed:
		return failedTaskPrefix
	case taskStatusInterrupted:
		return interruptedTaskPrefix
	default:
		return unknownTaskPrefix
	}
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
//
// 多 Agent 群聊：带 Agent 标记的历史消息前面加「【某某 说】」，让当前发言的
// Agent 分得清哪些话是同事说的、哪些是用户说的（role 只有 user/assistant 两种，
// 光靠 role 区分不了多个助手）。
func buildChatMessages(systemPrompt string, history []DSMessage, userMessage string) []map[string]string {
	msgs := []map[string]string{
		{"role": "system", "content": systemPrompt},
	}
	for _, msg := range history {
		content := msg.Content
		if prefix := historyTaskPrefix(msg); prefix != "" && content != "" {
			content = prefix + content
		}
		if tag := agentSpeechTag(msg); content != "" && tag != "" {
			content = tag + content
		}
		msgs = append(msgs, map[string]string{"role": msg.Role, "content": content})
	}
	msgs = append(msgs, map[string]string{"role": "user", "content": userMessage})
	return msgs
}

// agentSpeechTag 给群聊历史消息生成说话人标记。单 Agent（无标记）返回空串，
// 行为与改造前逐字节一致——不破坏前缀缓存。
func agentSpeechTag(m DSMessage) string {
	if strings.TrimSpace(m.Agent) == "" {
		return ""
	}
	name := m.Agent
	if card := GetAgentCard(m.Agent); card != nil && card.Name != "" {
		name = card.Name
	}
	return "【" + name + " 说】"
}
