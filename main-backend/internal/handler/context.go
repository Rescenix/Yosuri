package handler

const shortTermRounds = 5

// 全局 LRU 记忆实例（手机端使用，当前不在上下文窗口中注入，保留以备未来扩展）
var lruMemory *LRUMemory

func InitLRUMemory(capacity int) {
	lruMemory = NewLRUMemory(capacity)
}

// extractToolChain 提取与某个工具调用相关的完整消息链
func extractToolChain(history []DSMessage, toolIdx int) []DSMessage {
	var chain []DSMessage
	callID := history[toolIdx].ToolCallID

	i := toolIdx - 1
	for i >= 0 {
		msg := history[i]
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				if tc.ID == callID {
					chain = append(chain, msg)
					i = -1
					break
				}
			}
		}
		i--
	}
	chain = append(chain, history[toolIdx])

	j := toolIdx + 1
	for j < len(history) {
		msg := history[j]
		if msg.Role == "assistant" && len(msg.ToolCalls) == 0 {
			chain = append(chain, msg)
			break
		}
		j++
	}
	return chain
}

// buildContextWindow 构建发送给大模型的消息列表
// 长期记忆由系统提示词注入，此处不保留短期历史
func buildContextWindow(
	systemPrompt string,
	history []DSMessage,
	userMsg DSMessage,
) []DSMessage {
	msgs := []DSMessage{{Role: "system", Content: systemPrompt}}
	msgs = append(msgs, userMsg)
	return msgs
}
