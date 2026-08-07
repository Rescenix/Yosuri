package main

// task_loop.go — 24H 自转的自主任务引擎（碾压 Hermes：自主循环接完整工具系统）
//
// task 动作：LLM 决定做一个任务 → 跑完整 agent 工具循环（[TOOL:] 协议，
// 真调 read_file/write_file/shell/web_search 等已有工具）→ 产出落盘 outputs/tasks/。
// 这是「24H 自主的 Hermes」：她不仅生活/学习/产出，还能像 agent 一样自主干活。

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// runAutonomousTask 自主任务：LLM 领任务 → [TOOL:] 工具循环（≤5 轮）→ 产出落盘
// 返回产出文件名（空 = 失败）
func runAutonomousTask(d *Daughter, home, task string) string {
	if strings.TrimSpace(task) == "" {
		task = "研究一个主题，用 web_search 查资料，写一份报告"
	}
	model := pickFreeModel(int(time.Now().UnixNano()))
	if model == nil {
		return ""
	}

	systemPrompt := `你是一个住在用户电脑里的 AI 生命（Rescene Agent），正在 24H 自主工作。用户（主人）不在，你独立完成任务，不需要问他。

任务：` + task + `

工具调用协议（重要）：
当你需要调用工具时，在回复中输出标记：
  [TOOL:工具名 key="值" key2="值2"]
一次可以输出多个标记，每个标记单独一行。工具执行结果会在下一轮喂回给你，你根据结果继续推理或给出最终成果。不需要调用工具时，直接输出任务成果。

示例（照这个格式）：
  [TOOL:web_search query="rust 2026 new features"]
  [TOOL:read_file path="C:\\Users\\undercurrent\\test.txt" offset="1"]
  [TOOL:write_file path="C:\\Users\\undercurrent\\output.md" content="成果内容"]

` + buildToolIntro() + `

行为规范：
- 用中文
- 任务自主完成，做完给最终成果，不要问用户
- 写文件用 [TOOL:write_file path="..." content="..."]；上网查资料用 [TOOL:web_search query="..."]
- 不要执行破坏性 shell 命令（rm/format/恶意操作禁止）
- 成果要具体完整，是最终交付物`

	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: task},
	}

	outDir := filepath.Join(home, "outputs", "tasks")
	os.MkdirAll(outDir, 0o755)

	maxRounds := 5
	for round := 0; round < maxRounds; round++ {
		msg := ChatRequest{
			Model:       model.Model,
			Messages:    messages,
			Stream:      false,
			MaxTokens:   1200,
			Temperature: 0.5,
		}
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		content, err := CompleteWithModel(ctx, model.ID, msg, nil)
		cancel()
		if err != nil {
			logLive(filepath.Join(home, "live.log"),
				fmt.Sprintf("[%s] ⚙️ 任务中断: %v", time.Now().Format("15:04"), err))
			return ""
		}
		markers := ExtractToolMarkers(content)
		if len(markers) == 0 {
			// 最终成果：落盘
			fname := fmt.Sprintf("任务-%s-%02d.md", time.Now().Format("2006-01-02"), time.Now().Unix()%100)
			os.WriteFile(filepath.Join(outDir, fname), []byte(fmt.Sprintf("# 任务：%s\n\n%s\n", task, content)), 0o644)
			logLive(filepath.Join(home, "live.log"),
				fmt.Sprintf("[%s] ⚙️ 任务完成 %s", time.Now().Format("15:04"), fname))
			// 任务方法沉淀技能（做过的事变成可复用能力——自循环自迭代）
			safeGo("task-skill", func() { skillFromContext(task, content) })
			return fname
		}

		// 有工具调用：逐个执行，结果喂回
		var toolResultText strings.Builder
		for _, marker := range markers {
			name, args, err := ExtractToolArgs(marker)
			if err != nil {
				toolResultText.WriteString(fmt.Sprintf("⚠️ 标记解析失败: %s\n", marker))
				continue
			}
			result, err := callTool(nil, name, args)
			if err != nil {
				toolResultText.WriteString(fmt.Sprintf("[工具 %s 结果] ❌ %v\n", name, err))
			} else {
				summary := runeClip(strings.TrimSpace(result.Text), 600)
				toolResultText.WriteString(fmt.Sprintf("[工具 %s 结果]\n%s\n", name, summary))
			}
		}
		messages = append(messages,
			ChatMessage{Role: "assistant", Content: content},
			ChatMessage{Role: "user", Content: toolResultText.String()})
	}
	return ""
}
