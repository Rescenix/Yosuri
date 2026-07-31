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
		Description: "把「稳定的长期事实」写进长期记忆文件，下次对话还能读到。\n" +
			"【必须主动调用的场景】对话中出现了以下任意一类稳定事实：\n" +
			"  - 用户身份/职业/背景：如「我是全栈工程师」「我做二次元风格的 Agent 产品」\n" +
			"  - 用户偏好与习惯：如「我喜欢简短回复」「别用毛玻璃」「回复用中文」\n" +
			"  - 环境事实：开发平台、操作系统、工具链、目录结构等长期不变的信息\n" +
			"  - 项目约定与决策：命名规范、架构取舍、用户纠正过你的偏好（纠正=最该记）\n" +
			"  - 用户明确说「记住」「记下来」「别忘了」「你要记住」时\n" +
			"【不要调用的场景】\n" +
			"  - 任务进度、当前会话的具体工作内容——那是 workdir/handoff 的职责，不是长期记忆\n" +
			"  - 一次性问答、临时信息、很快就会过期的数据\n" +
			"  - 工作流完成摘要——「每次工作流都写？一次对话多少工作流多少md？」（用户原话，严禁）\n" +
			"判断标准就一条：这条信息三个月后还成立吗？成立就记，不成立就不记。\n" +
			"file 参数指定写入哪个文件（不含 .md），summary 参数指定 index.md 中这行的摘要描述。\n" +
			"常用的 file：preferences（用户偏好）、project（项目）、decisions（决策）。",
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
