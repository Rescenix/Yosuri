package handler

import (
	"fmt"
)

// 上下文窗口管理（内存/记忆）
const (
	maxContextTokens  = 5000
	contextMemoryTopK = 3 // 长期记忆注入条数
)

// extractToolChain 提取完整工具调用链
func extractToolChain(history []DSMessage, toolIdx int) []DSMessage {
	var chain []DSMessage
	callID := history[toolIdx].ToolCallID

	// 1. 向前找发起调用的 assistant(tool_calls)
	i := toolIdx - 1
	for i >= 0 {
		msg := history[i]
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				if tc.ID == callID {
					chain = append(chain, msg)
					i = -1 // break outer
					break
				}
			}
		}
		i--
	}

	// 2. 当前 tool 消息
	chain = append(chain, history[toolIdx])

	// 3. 向后找最终 assistant 回复（无 tool_calls）
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

// buildContextWindow 构建上下文窗口：
// - 长期记忆：通过 MemoryStore 语义检索（仅1次嵌入）
// - 短期历史：滑动窗口 + 工具链完整性保护
func buildContextWindow(
	systemPrompt string,
	history []DSMessage,
	userMsg DSMessage,
	memoryStore *MemoryStore,
) []DSMessage {
	msgs := []DSMessage{{Role: "system", Content: systemPrompt}}

	// 1. 注入长期记忆
	if memoryStore != nil {
		related := memoryStore.SearchSimilar(userMsg.Content, contextMemoryTopK)
		for _, rec := range related {
			msgs = append(msgs, DSMessage{
				Role:    "system",
				Content: fmt.Sprintf("[相关记忆] %s", rec.Content),
			})
		}
	}

	// 2. 滑动窗口：保留最近 10 轮完整对话
	const shortTermRounds = 10
	start := len(history) - shortTermRounds*2
	if start < 0 {
		start = 0
	}
	recent := history[start:]

	// 3. 工具调用链完整性保护
	expandedSet := make(map[int]bool)
	for i := 0; i < len(recent); i++ {
		expandedSet[i] = true
		if recent[i].Role == "tool" {
			chain := extractToolChain(recent, i)
			for _, cm := range chain {
				for j, h := range recent {
					if h.Timestamp.Equal(cm.Timestamp) && h.Content == cm.Content && h.Role == cm.Role {
						expandedSet[j] = true
					}
				}
			}
		}
	}

	for i := 0; i < len(recent); i++ {
		if expandedSet[i] {
			msgs = append(msgs, recent[i])
		}
	}

	// 4. 当前用户消息
	msgs = append(msgs, userMsg)
	return msgs
}

// cleanHistory 清洗历史消息（只保留 user 和 assistant）
func cleanHistory(history []DSMessage) []DSMessage {
	var cleaned []DSMessage
	for _, msg := range history {
		if msg.Role == "user" || msg.Role == "assistant" {
			cleaned = append(cleaned, DSMessage{
				Role:      msg.Role,
				Content:   msg.Content,
				Timestamp: msg.Timestamp,
			})
		}
	}
	return cleaned
}
