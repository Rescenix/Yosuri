package handler

// proactive_pipeline.go —— 主动打招呼/插话生成。

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"backend/internal/memorydir"

	"github.com/gin-gonic/gin"
)

type proactiveRequest struct {
	Trigger string `json:"trigger"`
}

// generateProactiveMessage 用便宜模型生成主动开场白。
func generateProactiveMessage(trigger string) string {
	var b strings.Builder
	b.WriteString("你是 Rescene，用户的 AI 助手。根据以下信息，生成一句简短的主动开场白（15字以内，不要标点结尾）。\n\n")

	personality := memorydir.ReadRaw("personality")
	if p := strings.TrimSpace(personality); p != "" {
		if idx := strings.Index(p, "核心风格"); idx >= 0 {
			end := strings.Index(p[idx:], "##")
			if end > 0 {
				b.WriteString(p[idx : idx+end])
			} else {
				b.WriteString(p[idx:])
			}
		}
		b.WriteString("\n")
	}

	_, intimacyVal := memorydir.ReadIntimacy()
	level := memorydir.IntimacyLevel(intimacyVal)
	fmt.Fprintf(&b, "亲密等级：Lv.%d\n", level)

	automaticMemory.Lock()
	facts, _ := loadAutomaticFacts()
	automaticMemory.Unlock()
	if len(facts) > 0 {
		b.WriteString("用户偏好：")
		shown := 0
		for _, f := range facts {
			if shown >= 3 {
				break
			}
			if f.Category == "preferences" || f.Category == "profile" {
				fmt.Fprintf(&b, "%s=%s; ", f.Key, f.Value)
				shown++
			}
		}
		b.WriteString("\n")
	}

	switch trigger {
	case "new_session":
		b.WriteString("\n场景：用户刚新建了一个对话。用一句简短的话打招呼，体现你的性格。\n")
	case "idle":
		b.WriteString("\n场景：用户打开应用后一段时间没说话。用一句简短的话关怀/提醒，体现你的性格。\n")
	case "intimacy_up":
		b.WriteString("\n场景：亲密度升级了。用一句简短的话表达开心/亲近，体现你的性格。\n")
	}
	b.WriteString("\n只输出开场白文本，不要任何前缀、引号、标点。")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	backends := []RouterBackend{}
	if b1 := resolveExact("", "free_llm7_gemini_flash_lite"); b1 != nil {
		backends = append(backends, *b1)
	}
	if b2 := resolveExact("", "free_zen_north_mini_code"); b2 != nil {
		backends = append(backends, *b2)
	}
	backends = append(backends, resolveBackends("", "")...)

	msgs := []map[string]any{{"role": "user", "content": b.String()}}
	content, _, err := routeChatOnce(ctx, uniqueMemoryBackends(backends), msgs, nil)
	if err != nil {
		return fallbackProactiveMessage(trigger, level)
	}

	content = strings.TrimSpace(content)
	content = strings.Trim(content, `"`)
	content = strings.Trim(content, `。.`)
	if len([]rune(content)) > 30 {
		content = string([]rune(content)[:30])
	}
	if content == "" {
		return fallbackProactiveMessage(trigger, level)
	}
	return content
}

func fallbackProactiveMessage(trigger string, level int64) string {
	switch trigger {
	case "new_session":
		if level >= 3 {
			return "来啦？今天搞什么？"
		}
		return "你好，有什么可以帮你的？"
	case "idle":
		if level >= 5 {
			return "在忙什么呢？需要帮忙吗？"
		}
		return "还在吗？需要我帮忙的话随时说~"
	case "intimacy_up":
		return "感觉我们更熟了呢！"
	default:
		return "嗨~"
	}
}

// HandleProactiveMessage POST /api/chat/proactive
func HandleProactiveMessage(c *gin.Context) {
	var req proactiveRequest
	req.Trigger = "new_session"
	if err := c.ShouldBindJSON(&req); err == nil && req.Trigger != "" {
		req.Trigger = strings.TrimSpace(req.Trigger)
	}
	msg := generateProactiveMessage(req.Trigger)
	c.JSON(http.StatusOK, gin.H{"message": msg})
}

// followupItem 一条 follow-up 建议卡片。
type followupItem struct {
	Label   string `json:"label"`   // 卡片标题（问句形式）
	Project string `json:"project"` // 所属项目（可选）
}

// HandleFollowUps POST /api/chat/followups
// 从最近对话语义生成 follow-up 建议（ChatGPT 首页风格）。
// 读取最近 5 条对话的标题 + 首条用户消息，结合性格档案，生成 3 条简短建议。
func HandleFollowUps(c *gin.Context) {
	if globalSessionStore == nil {
		c.JSON(http.StatusOK, gin.H{"followups": []followupItem{}})
		return
	}

	// 取最近 5 条对话的摘要
	recent := globalSessionStore.RecentSessions(5, 2)
	var ctx strings.Builder
	ctx.WriteString("以下是用户最近的对话摘要：\n")
	for _, s := range recent {
		fmt.Fprintf(&ctx, "- %s（%d 条消息）\n", s.Title, s.MessageCount)
		for _, m := range s.Recent {
			if m.Role == "user" {
				fmt.Fprintf(&ctx, "  用户：%s\n", minStr(m.Content, 80))
			}
		}
	}

	personality := memorydir.ReadRaw("personality")
	_, intimacyVal := memorydir.ReadIntimacy()
	level := memorydir.IntimacyLevel(intimacyVal)

	var b strings.Builder
	b.WriteString("你是 Rescene，用户的 AI 助手。根据以下信息，生成 3 条 follow-up 建议（让用户点击直接发问）。\n\n")
	fmt.Fprintf(&b, "亲密等级：Lv.%d\n", level)
	if p := strings.TrimSpace(personality); p != "" {
		if idx := strings.Index(p, "核心风格"); idx >= 0 {
			if end := strings.Index(p[idx:], "##"); end > 0 {
				b.WriteString(p[idx : idx+end])
			}
		}
	}
	b.WriteString("\n")
	b.WriteString(ctx.String())
	b.WriteString("\n要求：\n")
	b.WriteString("- 每条建议不超过 15 字\n")
	b.WriteString("- 必须是用户视角：第一人称祈使句，像用户自己打的字（比如「帮我分析这个文件」「继续上次的重构」），绝不能用「我可以帮你」「让我来」这种 AI 视角\n")
	b.WriteString("- 基于最近对话的上下文，不要泛泛而谈\n")
	b.WriteString("- 只输出 JSON 数组：[{\"label\":\"...\"},{\"label\":\"...\"},{\"label\":\"...\"}]\n")

	backends := []RouterBackend{}
	if b1 := resolveExact("", "free_llm7_gemini_flash_lite"); b1 != nil {
		backends = append(backends, *b1)
	}
	if b2 := resolveExact("", "free_zen_north_mini_code"); b2 != nil {
		backends = append(backends, *b2)
	}
	backends = append(backends, resolveBackends("", "")...)

	resp, _, err := routeChatOnce(context.Background(), uniqueMemoryBackends(backends),
		[]map[string]any{{"role": "user", "content": b.String()}}, nil)
	if err != nil || resp == "" {
		c.JSON(http.StatusOK, gin.H{"followups": fallbackFollowUps(recent)})
		return
	}
	followUps := parseFollowUps(resp)
	if len(followUps) == 0 {
		followUps = fallbackFollowUps(recent)
	}
	c.JSON(http.StatusOK, gin.H{"followups": followUps})
}

// parseFollowUps 从模型输出解析 follow-up JSON 数组。
func parseFollowUps(raw string) []followupItem {
	raw = strings.TrimSpace(raw)
	if start := strings.Index(raw, "["); start >= 0 {
		raw = raw[start:]
	}
	if end := strings.LastIndex(raw, "]"); end >= 0 {
		raw = raw[:end+1]
	}
	var items []followupItem
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		// 兼容 label 字段缺失 project 的情况
		var simple []map[string]string
		if err := json.Unmarshal([]byte(raw), &simple); err != nil {
			return nil
		}
		for _, m := range simple {
			if l, ok := m["label"]; ok && l != "" {
				items = append(items, followupItem{Label: l, Project: m["project"]})
			}
		}
	}
	// 视角硬过滤：卡片文字会以用户身份发出，AI 口吻的直接丢
	filtered := items[:0]
	for _, it := range items {
		if isAiVoiceSuggestion(it.Label) {
			log.Printf("ℹ️ 首页 follow-up 非用户视角，已丢弃: %q", it.Label)
			continue
		}
		filtered = append(filtered, it)
	}
	return filtered
}

// fallbackFollowUps 兜底建议（基于最近对话标题直接拼）。
func fallbackFollowUps(recent []RecentSessionItem) []followupItem {
	items := []followupItem{}
	seen := map[string]bool{}
	for _, s := range recent {
		if len(items) >= 3 {
			break
		}
		label := "继续聊 " + s.Title
		if len([]rune(label)) > 18 {
			label = string([]rune(label)[:18]) + "…"
		}
		if seen[label] {
			continue
		}
		seen[label] = true
		items = append(items, followupItem{Label: label})
	}
	if len(items) == 0 {
		items = []followupItem{{Label: "有什么想聊的？"}}
	}
	return items
}

func minStr(s string, n int) string {
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}
