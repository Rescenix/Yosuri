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
// workflow_done 太久（verify 同款语义——加分项，不是阻断项）。
const suggestFollowUpTimeout = 12 * time.Second

// suggestFollowUp 在工作流 completed 收尾时，用轻量 LLM 调用为最终回答生成 2-3 条
// follow-up 建议，随 workflow_done 下发，前端渲染成按钮行让用户免打字继续。
//
// 旁路约束：任何错误（调用失败/超时/解析失败）都只返回 nil，绝不阻断收尾。
// 数据来源：任务原文 + 最终回答 + 动作记录节选，不喂整段 transcript，省 token 省时间。
// 要不要出建议由 LLM 自己判断——prompt 明确让它"没有值得继续的方向就输出空数组"，
// 前端收到空数组自然不渲染按钮行，0 打扰。

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

基于以上**刚刚完成的具体工作**，生成 2-3 条用户接下来最可能想说的话。

硬性要求：
1. 用户视角第一人称：这些文字会被直接当成用户发的消息，必须像用户自己敲出来的。
   正确：「把刚才那个空指针补上」「跑一遍测试确认」「顺手加个错误提示」
   错误：「我可以帮你…」「让我来…」「建议你…」「是否需要…」（AI 口吻，一律不许）
2. 必须落到本次工作的具体对象上——句子里要出现本次真正改动的文件名、函数名、功能点。
   不许写「继续完善功能」「还有什么需要帮忙的吗」这种换到任何任务都成立的空话。
3. 只提**上面记录里真实存在**的东西，没做过的、看不到的绝不臆造。
4. 每条不超过 20 字，中文，直给，不客套。
5. 任务已经彻底收尾、确实没有下一步可做时，输出空数组 []。
6. 只输出一个 JSON 数组，例如 ["给 parseFollowUps 补单测", "把改动同步到官网文档"]，
   不要任何解释、不要代码块包裹。`,
		truncateChars(task, 500), truncateChars(finalOutput, 3000), transcriptSnippet)

	msgs := []map[string]any{{"role": "user", "content": prompt}}
	ctx, cancel := context.WithTimeout(context.Background(), suggestFollowUpTimeout)
	defer cancel()

	content, _, err := routeChatOnce(ctx, resolveBackends("", "auto"), msgs, nil)
	if err != nil {
		log.Printf("⚠️ follow-up 建议生成失败: %v", err)
		return nil
	}

	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var suggestions []string
	if err := json.Unmarshal([]byte(content), &suggestions); err != nil {
		log.Printf("⚠️ follow-up 建议 JSON 解析失败: %v", err)
		return nil
	}

	// 清洗：去空、去重、最多 3 条。不截断——按钮上要完整显示，截了用户看不到全句。
	// 视角硬过滤：这些文字会以用户身份发出去，AI 口吻的句子直接丢，不指望 prompt 每次都听话。
	seen := map[string]bool{}
	out := make([]string, 0, len(suggestions))
	for _, s := range suggestions {
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
