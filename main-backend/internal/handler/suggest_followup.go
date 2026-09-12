package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// suggestFollowUpTimeout 生成 follow-up 建议的硬超时：这是收尾旁路，绝不能拖慢
// workflow_done 太久（verify 同款语义——加分项，不是阻断项）。12s：主模型（推理模型
// 光思考可能 10-30s）需要比 6s 更宽的窗口才能吐完 3 条；拿不到就 nil、按钮不出现，0 打扰。
const suggestFollowUpTimeout = 12 * time.Second

// suggestFollowUp 在工作流 completed 收尾时，用轻量 LLM 调用为最终回答生成 2-3 条
// follow-up 建议，随 workflow_done 下发，前端渲染成按钮行让用户免打字继续。
//
// 旁路约束：任何错误（调用失败/超时/解析失败）都只返回 nil，绝不阻断收尾。
// 数据来源：任务原文 + 最终回答 + 动作记录节选，不喂整段 transcript，省 token 省时间。
// 必出 2-3 条——prompt 已禁止输出空数组（哪怕任务收尾也基于最终回答给
// 复盘/完善/延伸三个方向），前端收到数组就渲染按钮行。

// tailChars 保留字符串**尾部**（按 rune 计），用于收尾上下文——
// 工作流的结论、最后几步动作都在末尾，按头部截断会把关键信息全砍掉。
func tailChars(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return "...[前文省略]\n" + string(runes[len(runes)-max:])
}

func suggestFollowUp(task, finalOutput string, transcript []string) []string {
	if strings.TrimSpace(finalOutput) == "" {
		return nil
	}

	var transcriptSnippet string
	if len(transcript) > 0 {
		// 取尾部若干条：继续推进的方向取决于「刚刚做到哪」，不是任务开头读了什么文件。
		// transcript 现在混合了「意图:」与工具调用日志，12 条约覆盖最后三四轮。
		const tailSteps = 12
		tail := transcript
		if len(tail) > tailSteps {
			tail = tail[len(tail)-tailSteps:]
		}
		transcriptSnippet = tailChars(strings.Join(tail, "\n"), 1600)
	}

	prompt := fmt.Sprintf(`任务：%s

最终回答：
%s

动作记录（节选）：
%s
注意：动作记录可能为空——纯对话/角色扮演（酒馆）场景没有工具调用，这是正常情况。

基于以上**刚刚完成的具体工作**（动作记录为空时，就完全基于最终回答），生成 3 条用户接下来最可能想说的话。

硬性要求：
1. 用户视角第一人称：这些文字会被直接当成用户发的消息，必须像用户自己敲出来的。
   正确：「把刚才那个空指针补上」「跑一遍测试确认」「顺手加个错误提示」
   错误：「我可以帮你…」「让我来…」「建议你…」「是否需要…」（AI 口吻，一律不许）
2. 落到本次对话的具体对象上——说的时候提到刚才真正聊到的内容（RP 场景就顺着当前剧情/口吻，给 3 条角色会说或你想回的话）。
   不许写「继续完善功能」「还有什么需要帮忙的吗」这种换到任何对话都成立的空话。
3. 只提**上面记录里真实存在**的东西，没做过的、看不到的绝不臆造。
4. 每条不超过 20 字，中文，直给，不客套。
5. 只输出一个 JSON 数组，例如 ["给 parseFollowUps 补单测", "把改动同步到官网文档", "补上参数校验"]，
   不要任何解释、不要代码块包裹。**必须恰好 3 条，禁止 2 条、禁止空数组**——哪怕任务已收尾，
   也基于最终回答给用户「复盘确认 / 顺手完善 / 相关延伸」三个方向。
6. 禁止以「没有动作记录」「没有上下文」为由拒答——记录可能就是空的，永远基于任务和最终回答生成。`,
		truncateChars(task, 500), truncateChars(finalOutput, 3000), transcriptSnippet)

	msgs := []map[string]any{{"role": "user", "content": prompt}}
	ctx, cancel := context.WithTimeout(context.Background(), suggestFollowUpTimeout)
	defer cancel()

	// 轻量快模型独立生成（方案 B）：建议只是几百 token 的小任务，用 auto 免费池的
	// 轻量模型（实测 deepseek-flash 级 ~1s）秒出，不绑定主模型——主模型是推理模型，
	// 光思考 10-30s，只会把建议拖慢。拿到就渲染，拿不到 nil 按钮不出现，0 打扰。
	backends := resolveBackends("", "auto")
	content, _, err := routeChatOnce(ctx, backends, msgs, nil)
	if err != nil {
		log.Printf("⚠️ follow-up 建议生成失败: %v", err)
		return nil
	}

	content = strings.TrimSpace(content)
	var suggestions []string
	if err := json.Unmarshal([]byte(content), &suggestions); err != nil {
		log.Printf("⚠️ follow-up 建议 JSON 解析失败: %v（原文: %.200s）", err, content)
		return nil
	}

	// 清洗：去空、去重、最多 3 条。不截断——按钮上要完整显示，截了用户看不到全句。
	// 视角硬过滤：这些文字会以用户身份发出去，AI 口吻的句子直接丢，不指望 prompt 每次都听话。
	return cleanSuggestions(suggestions)
}

// extractSuggestionArray 从模型原始输出中提取纯 JSON 数组段：
// 轻量模型偶尔在数组前后夹解释文字/标题/markdown 包裹，或整段拒答没有方括号。
// 返回只含 [ ... ] 的内容；找不到方括号返回空串（上层按解析失败处理）。
func extractSuggestionArray(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	start := strings.Index(s, "[")
	end := strings.LastIndex(s, "]")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return ""
}

// parseSuggestionJSON 从原始输出提取并解析建议数组；解析失败返回 nil。
func parseSuggestionJSON(raw string) []string {
	seg := extractSuggestionArray(raw)
	if seg == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(seg), &out); err != nil {
		return nil
	}
	return cleanSuggestions(out)
}

// cleanSuggestions 去空、去重、AI 口吻过滤、上限 3 条。
func cleanSuggestions(list []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(list))
	for _, s := range list {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] || len(out) >= 3 {
			continue
		}
		if isAiVoiceSuggestion(s) {
			log.Printf("ℹ️ follow-up 建议非用户视角，已丢弃: %q", s)
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// aiVoiceMarkers —— 出现即判定为 AI 口吻（这些词只可能出自助手之口）。
var aiVoiceMarkers = []string{
	"我可以", "我能", "我来", "让我", "建议你", "您可以", "您需要",
	"是否需要", "还有什么", "需要我", "帮你", "为您", "是否要",
}

func isAiVoiceSuggestion(s string) bool {
	for _, m := range aiVoiceMarkers {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}
