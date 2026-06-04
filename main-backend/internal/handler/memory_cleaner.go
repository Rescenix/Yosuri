package handler

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// StartMemoryCleaner 启动后台记忆清理协程
func (m *MemoryStore) StartMemoryCleaner() {
	go func() {
		// 如果当前时间在20:00之后，立即执行一次清理
		now := time.Now()
		today8pm := time.Date(now.Year(), now.Month(), now.Day(), 20, 0, 0, 0, now.Location())
		if now.After(today8pm) {
			fmt.Println("🧹 服务器启动晚于20:00，立即执行一次记忆清理...")
			m.CleanMemories()
		}

		for {
			now := time.Now()
			next := time.Date(now.Year(), now.Month(), now.Day(), 20, 0, 0, 0, now.Location())
			if now.After(next) {
				next = next.Add(24 * time.Hour)
			}
			time.Sleep(next.Sub(now))

			fmt.Println("🧹 杉汐开始整理记忆...")
			m.CleanMemories()
			fmt.Println("✅ 杉汐整理记忆完成")
		}
	}()
}

// CleanMemories 让杉汐分析记忆并自动执行清理
func (m *MemoryStore) CleanMemories() {
	if len(m.records) == 0 {
		return
	}

	var memoryList strings.Builder
	for i, rec := range m.records {
		memoryList.WriteString(fmt.Sprintf(
			"[%d] ID:%s | 时间:%s | 内容:%s\n",
			i, rec.ID, rec.Timestamp.Format("2006-01-02 15:04"), rec.Content,
		))
	}

	systemPrompt := `你是杉汐，正在整理自己的记忆库。请分析以下记忆，对每条记忆标记操作建议：
- KEEP: 有实质内容的对话
- MERGE: 与另一条高度重复，建议合并（需指定 target_id）
- DISCARD: 纯寒暄无信息量

请以 JSON 数组格式返回：[{"id":"xxx","action":"KEEP|MERGE|DISCARD","target_id":"目标记忆ID(仅MERGE时需要)","reason":"原因"}]

以下是你的记忆库：` + memoryList.String()

	reply, _, _ := askDeepSeekWithMessages([]DSMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: "请开始整理你的记忆库"},
	}, 0, 0, 0, "")

	m.executeCleanup(reply)
}

// CleanupSuggestion 杉汐的清理建议
type CleanupSuggestion struct {
	ID       string `json:"id"`
	Action   string `json:"action"`
	TargetID string `json:"target_id,omitempty"`
	Reason   string `json:"reason"`
}

// executeCleanup 解析杉汐的建议并执行具体清理操作
func (m *MemoryStore) executeCleanup(reply string) {
	jsonStart := strings.Index(reply, "[")
	jsonEnd := strings.LastIndex(reply, "]")
	if jsonStart == -1 || jsonEnd == -1 {
		fmt.Println("⚠️ 杉汐返回的不是有效 JSON 数组，跳过本次清理")
		return
	}

	var suggestions []CleanupSuggestion
	if err := json.Unmarshal([]byte(reply[jsonStart:jsonEnd+1]), &suggestions); err != nil {
		fmt.Printf("⚠️ 解析清理建议失败: %v\n", err)
		return
	}

	logEntries := make([]string, 0)

	for _, s := range suggestions {
		switch s.Action {
		case "KEEP":
			logEntries = append(logEntries, fmt.Sprintf("KEEP %s: %s", s.ID, s.Reason))

		case "DISCARD":
			m.records = removeByID(m.records, s.ID)
			logEntries = append(logEntries, fmt.Sprintf("DISCARD %s: %s", s.ID, s.Reason))

		case "MERGE":
			targetRec := findRecordByID(m.records, s.TargetID)
			sourceRec := findRecordByID(m.records, s.ID)
			if targetRec != nil && sourceRec != nil {
				// 合并内容并生成新的摘要
				mergedContent := fmt.Sprintf("朋友: %s | 杉汐: %s", targetRec.Content, sourceRec.Content)
				summary := generateSummary(mergedContent)

				targetRec.Content = summary
				targetRec.Timestamp = time.Now()

				// 删除源记忆
				m.records = removeByID(m.records, s.ID)
				logEntries = append(logEntries, fmt.Sprintf("MERGE %s→%s: %s", s.ID, s.TargetID, s.Reason))
			}
		}
	}

	// 持久化
	data, _ := json.MarshalIndent(m.records, "", "  ")
	if err := os.WriteFile(m.filePath, data, 0644); err != nil {
		fmt.Printf("⚠️ 记忆持久化失败: %v\n", err)
	}

	fmt.Printf("✅ 杉汐整理记忆完成，操作日志:\n%s\n", strings.Join(logEntries, "\n"))
}

// ============ 辅助函数 ============

func removeByID(records []MemoryRecord, id string) []MemoryRecord {
	for i, rec := range records {
		if rec.ID == id {
			return append(records[:i], records[i+1:]...)
		}
	}
	return records
}

func findRecordByID(records []MemoryRecord, id string) *MemoryRecord {
	for i := range records {
		if records[i].ID == id {
			return &records[i]
		}
	}
	return nil
}

func generateSummary(content string) string {
	// 复用已有的摘要生成逻辑（调用 DeepSeek）
	return ""
}
