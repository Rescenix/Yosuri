package handler

import (
	"encoding/json"
	"strings"

	"backend/internal/ai/core"
	"backend/internal/memorydir"
)

const rememberToolName = "remember"

var rememberToolDef = core.ToolDefinition{
	Type: "function",
	Function: core.ToolFunctionDetail{
		Name:        rememberToolName,
		Description: "当用户说「记住」「记下来」「别忘了」「你要记住」等明确要你永久记住某件事时调用。\n" +
			"把用户想让你记住的内容写进你的长期记忆文件，下次对话你还能读到。\n" +
			"【必须调用的场景】用户明确说「记住我喜欢XXX」「记一下这个约定」「别忘了XXX」「记下来」。\n" +
			"【不要调用的场景】普通对话、用户没说「记住」、工作流步骤摘要——那些不用记。\n" +
			"file 参数指定写入哪个文件（不含 .md），summary 参数指定 index.md 中这行的摘要描述。",
		Parameters: core.ToolParameters{
			Type: "object",
			Properties: map[string]core.ToolProperty{
				"text": {
					Type:        "string",
					Description: "要记住的内容。用自然语言写清楚。",
				},
				"file": {
					Type:        "string",
					Description: "文件名（不含 .md），如「preferences」「project-re0」「session-jul30」。相同 file 的内容会合并到同一个文件。",
				},
				"summary": {
					Type:        "string",
					Description: "index.md 中该条目的摘要，一句话说清本条关联什么。例如「用户偏好：简短回复，常用 deepseek」。不提供则自动从 text 截取前 40 字。",
				},
			},
			Required: []string{"text", "file"},
		},
	},
}

// handleRemember 处理 remember 工具调用，写入 memory/ 目录。
func handleRemember(argsJSON string) string {
	var args struct {
		Text    string `json:"text"`
		File    string `json:"file"`
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "参数解析失败: " + err.Error()
	}
	args.Text = strings.TrimSpace(args.Text)
	if args.Text == "" {
		return "text 不能为空，请带上要记住的内容。"
	}
	if args.File == "" {
		return "file 不能为空，请指定文件名（不含 .md）。"
	}
	// 防路径穿越
	args.File = strings.TrimSpace(args.File)
	args.File = strings.ReplaceAll(args.File, "/", "")
	args.File = strings.ReplaceAll(args.File, "\\", "")
	args.File = strings.ReplaceAll(args.File, "..", "")
	if args.File == "" {
		return "文件名无效。"
	}

	if args.Summary == "" {
		// 自动截取前 40 个字
		runes := []rune(args.Text)
		if len(runes) > 40 {
			args.Summary = string(runes[:40]) + "…"
		} else {
			args.Summary = args.Text
		}
	}

	if err := memorydir.Remember(args.File, args.Summary, args.Text); err != nil {
		return "写入失败: " + err.Error()
	}
	return "已记住 ✅ 已写入 memory/" + args.File + ".md，下次对话时我会自动想起。"
}
