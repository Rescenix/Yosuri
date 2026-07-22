// handler/chat_utils.go - 对话历史工具函数

package handler

// 截断历史消息
func truncateHistory(history []DSMessage, maxHistory int) []DSMessage {
	if len(history) > maxHistory {
		return history[len(history)-maxHistory:]
	}
	return history
}

// 构建消息列表
func buildChatMessages(systemPrompt string, history []DSMessage, userMessage string) []map[string]string {
	msgs := []map[string]string{
		{"role": "system", "content": systemPrompt},
	}
	for _, msg := range history {
		msgs = append(msgs, map[string]string{"role": msg.Role, "content": msg.Content})
	}
	msgs = append(msgs, map[string]string{"role": "user", "content": userMessage})
	return msgs
}
