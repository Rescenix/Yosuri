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
func suggestFollowUp(task, finalOutput string, transcript []string) []string {
	if strings.TrimSpace(finalOutput) == "" {
		return nil
	}

	var transcriptSnippet string
	if len(transcript) > 0 {
		transcriptSnippet = truncateChars(strings.Join(transcript, "\n"), 2000)
	}

	prompt := fmt.Sprintf(`任务：%s

最终回答：
%s

动作记录（节选）：
%s

基于以上已完成的工作，给用户生成 2-3 条最自然的继续推进建议（比如：改进/扩展功能、修复隐患、验证某个点、或者把结果整理成文档）。
要求：
1. 每条一句话，中文，简短具体，像用户自己想说的下一句话，不要客套。
2. 如果任务真的已经收尾、没有可继续的方向，输出空数组。
3. 只输出一个 JSON 数组，例如 ["继续完善 xxx", "帮我把结果写成文档"]，不要任何解释和代码块包裹。`,
		truncateChars(task, 500), truncateChars(finalOutput, 2000), transcriptSnippet)

	msgs := []map[string]any{{"role": "user", "content": prompt}}
	ctx, cancel := context.WithTimeout(context.Background(), suggestFollowUpTimeout)
	defer cancel()

	content, _, err := routeChatOnce(ctx, resolveBackends("default", ""), msgs, nil)
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
	seen := map[string]bool{}
	out := make([]string, 0, len(suggestions))
	for _, s := range suggestions {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] || len(out) >= 3 {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
