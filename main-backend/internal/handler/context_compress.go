package handler

// 上下文感知压缩 —— 长任务跑到上下文快满时，把早期执行记录折叠成一段摘要，
// 腾出预算继续跑，而不是撞上模型上下文窗口直接崩掉。
//
// 三条设计都来自实证（用户研究报告 #4/#5/#6），不是拍脑袋：
//
//  #5 自适应窗口：只有真实 prompt_tokens 超过窗口 80% 才触发。前几轮保留完整原文，
//     给初期信息收集留足灵活性——过早压缩反而丢信息。
//  #4 上下文感知：压缩提示里显式带上「原始任务」，让模型生成的是"与这个任务相关"
//     的定向摘要，而不是无差别浓缩。相比无脑摘要，能少丢关键信息。
//  #6 分层保留：最近几轮原样保留（正在用的细节不能动）；摘要提示明确要求保住
//     文件路径/行号/标识符/报错/未完成 TODO——这些改错一个字工具调用就失效。
//
// 与已有机制的配合：工具输出早已首尾截断+落盘（tool_output.go），所以进到这里的
// 每条 tool 消息本就 ≤6000 字符；压缩要解决的是"轮数堆积"，不是"单条太长"。

import (
	"context"
	"fmt"
	"strings"
)

const (
	// compressTriggerRatio 真实 prompt_tokens 占窗口多大比例时触发压缩。
	compressTriggerRatio = 0.8
	// compressKeepRecentRounds 最近几轮原样保留（正在推进的细节不压）。
	compressKeepRecentRounds = 3
	// compressMarker 摘要消息的标记，既给人看也便于下一次压缩把旧摘要一起折叠。
	compressMarker = "[早期执行记录已压缩]"
)

// compressResult 压缩产出，供调用方发 SSE / 更新统计。
type compressResult struct {
	Compressed  bool
	FoldedMsgs  int // 被折叠掉的消息条数
	BeforeChars int
	AfterChars  int
}

// compressContextIfNeeded 在真实上下文超过阈值时，把早期轮次折叠成一段任务相关摘要。
// window 为 0（模型没报上下文窗口）时用 estimatedContextWindow 兜底。
// 返回可能被替换过的 msgs 和一份压缩报告；未触发时原样返回。
func (r *WorkflowRunner) compressContextIfNeeded(
	ctx context.Context, backends []RouterBackend,
	msgs []map[string]any, task string, promptTokens, window int,
) ([]map[string]any, compressResult) {

	if window <= 0 {
		window = estimatedContextWindow
	}
	// #5 自适应窗口：没到 80% 不动
	if promptTokens < int(float64(window)*compressTriggerRatio) {
		return msgs, compressResult{}
	}

	// preamble 边界：system + 会话历史 + 本次任务，绝不压缩——
	// 任务本身一旦被折叠，模型就忘了自己在干什么。
	// 用「内容等于 task 的那条 user 消息」定位，它不随摘要插入而移动，
	// 因此再次压缩时能把上一版摘要一并纳入 middle 重新折叠（不会越堆越多）。
	taskIdx := lastIndexOfTask(msgs, task)
	if taskIdx < 0 {
		taskIdx = firstAssistantIndex(msgs) - 1
	}
	middleStart := taskIdx + 1
	if middleStart < 1 {
		return msgs, compressResult{}
	}

	// #6 分层：保留最近 N 轮。轮边界 = 带 tool_calls 的 assistant 消息。
	tailStart := roundStartFromEnd(msgs, compressKeepRecentRounds)
	if tailStart <= middleStart {
		// 可压缩的中段不足一轮——再压也省不出什么，避免为极少内容白花一次 LLM 调用
		return msgs, compressResult{}
	}

	middle := msgs[middleStart:tailStart]
	if len(middle) == 0 {
		return msgs, compressResult{}
	}
	beforeChars := msgsChars(middle)

	// #4 上下文感知：把任务塞进压缩提示，生成定向摘要
	summary, err := r.summarizeRounds(ctx, backends, task, middle)
	if err != nil || strings.TrimSpace(summary) == "" {
		// 压缩失败绝不能中断主任务——原样返回，下一轮还会再试
		return msgs, compressResult{}
	}

	summaryMsg := map[string]any{
		"role": "user",
		"content": fmt.Sprintf("%s\n（以下是本任务早期若干轮执行的摘要，原始细节已省略，如需精确内容可重新读取相关文件）：\n%s",
			compressMarker, summary),
	}

	// 折叠：preamble + 摘要 + 最近 N 轮。middle 是整轮整轮切的，
	// 不会留下"有结果没调用"或"有调用没结果"的孤儿，tool 配对天然完整。
	out := make([]map[string]any, 0, middleStart+1+len(msgs)-tailStart)
	out = append(out, msgs[:middleStart]...)
	out = append(out, summaryMsg)
	out = append(out, msgs[tailStart:]...)

	return out, compressResult{
		Compressed:  true,
		FoldedMsgs:  len(middle),
		BeforeChars: beforeChars,
		AfterChars:  msgsChars([]map[string]any{summaryMsg}),
	}
}

// summarizeRounds 用一次非流式调用把若干轮执行记录压成任务相关摘要。
func (r *WorkflowRunner) summarizeRounds(
	ctx context.Context, backends []RouterBackend, task string, middle []map[string]any,
) (string, error) {
	sys := fmt.Sprintf(`你在压缩一个正在进行的编程 agent 任务的早期执行记录，好腾出上下文继续干活。

原始任务：%s

把下面这些轮次的执行过程压成要点式摘要，规则：
- 只保留与「原始任务」相关的信息：已经查明的结论、改过的文件、下一步待办。
- 必须原样保住：文件路径、行号、函数名、UUID/hash/URL 等标识符、以及关键报错——这些改错一个字符后续工具调用就会失效。
- 丢弃：冗余的中间过程、已经读完且无后续价值的文件正文、重复的探索。
- 未完成的 TODO / 待验证项要显式列出，别让它们在压缩里消失。
只输出摘要正文，不要客套。`, task)

	var b strings.Builder
	for _, m := range middle {
		role, _ := m["role"].(string)
		content, _ := m["content"].(string)
		if names := toolCallNames(m); len(names) > 0 {
			fmt.Fprintf(&b, "[%s 调用工具: %s] %s\n", role, strings.Join(names, ", "), content)
			continue
		}
		fmt.Fprintf(&b, "[%s] %s\n", role, content)
	}

	msgs := []map[string]any{
		{"role": "system", "content": sys},
		{"role": "user", "content": b.String()},
	}
	content, _, err := routeChatOnce(ctx, backends, msgs, nil)
	return content, err
}

// ---- 定位辅助 ----

// lastIndexOfTask 找内容恰好等于 task 的最后一条 user 消息（本次任务那条）。
func lastIndexOfTask(msgs []map[string]any, task string) int {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i]["role"] == "user" {
			if c, _ := msgs[i]["content"].(string); c == task {
				return i
			}
		}
	}
	return -1
}

// firstAssistantIndex 第一条 assistant 消息的位置（第一轮的开始）。
func firstAssistantIndex(msgs []map[string]any) int {
	for i, m := range msgs {
		if m["role"] == "assistant" {
			return i
		}
	}
	return len(msgs)
}

// roundStartFromEnd 返回"倒数第 keep 轮"的起始下标（带 tool_calls 的 assistant）。
// 不足 keep 轮时返回 len(msgs)（即没有可保留的完整尾巴之外的中段）。
func roundStartFromEnd(msgs []map[string]any, keep int) int {
	seen := 0
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i]["role"] != "assistant" {
			continue
		}
		if len(toolCallNames(msgs[i])) > 0 {
			seen++
			if seen == keep {
				return i
			}
		}
	}
	return len(msgs)
}

// toolCallNames 取一条 assistant 消息里的工具名。兼容两种类型：
// 本轮内存里是 []map[string]any；从检查点 JSON 反序列化回来则是 []interface{}——
// 不兼容后者的话，续跑后的轮次边界检测会失效。
func toolCallNames(m map[string]any) []string {
	raw, ok := m["tool_calls"]
	if !ok {
		return nil
	}
	var items []any
	switch v := raw.(type) {
	case []map[string]any:
		for _, it := range v {
			items = append(items, it)
		}
	case []any:
		items = v
	default:
		return nil
	}
	var names []string
	for _, it := range items {
		cm, ok := it.(map[string]any)
		if !ok {
			continue
		}
		if fn, ok := cm["function"].(map[string]any); ok {
			if n, ok := fn["name"].(string); ok {
				names = append(names, n)
			}
		}
	}
	return names
}

// msgsChars 统计一批消息 content 的总字符数。
func msgsChars(msgs []map[string]any) int {
	n := 0
	for _, m := range msgs {
		if c, ok := m["content"].(string); ok {
			n += len(c)
		}
	}
	return n
}
