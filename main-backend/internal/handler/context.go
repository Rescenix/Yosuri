package handler

import "fmt"

var UseSemanticMemory = true

const shortTermRounds = 6

// 全局 LRU 记忆实例（手机端使用）
var lruMemory *LRUMemory

func InitLRUMemory(capacity int) {
    lruMemory = NewLRUMemory(capacity)
}

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

func buildContextWindow(
    systemPrompt string,
    history []DSMessage,
    userMsg DSMessage,
    memoryStore *MemoryStore,
) []DSMessage {
    msgs := []DSMessage{{Role: "system", Content: systemPrompt}}

    // 长期记忆注入
    if UseSemanticMemory && memoryStore != nil {
        related := memoryStore.SearchSimilar(userMsg.Content, 3)
        for _, rec := range related {
            msgs = append(msgs, DSMessage{
                Role:    "system",
                Content: fmt.Sprintf("[相关记忆] %s", rec.Content),
            })
        }
    } else if !UseSemanticMemory && lruMemory != nil {
        // 手机端：从 LRU 缓存中获取最近记忆
        recent := lruMemory.GetRecent(5)
        for _, mem := range recent {
            msgs = append(msgs, DSMessage{
                Role:    "system",
                Content: fmt.Sprintf("[记忆] %s", mem),
            })
        }
    }

    // 滑动窗口：保留最近 shortTermRounds 轮完整对话
    start := len(history) - shortTermRounds*2
    if start < 0 {
        start = 0
    }
    recent := history[start:]

    // 工具调用链完整性保护
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

    msgs = append(msgs, userMsg)
    return msgs
}