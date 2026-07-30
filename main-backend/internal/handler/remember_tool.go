package handler

import (
	"encoding/json"

	"backend/internal/ai/core"
	"backend/internal/swiftnet"
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
			"keywords 参数填写该记忆相关的 [[反向链接关键词]]，多个用 / 分隔，方便联想召回。",
		Parameters: core.ToolParameters{
			Type: "object",
			Properties: map[string]core.ToolProperty{
				"text": {
					Type:        "string",
					Description: "要记住的内容。用自然语言，一句话说清楚（如「用户偏好简短回复，不喜欢啰嗦」）。",
				},
				"cluster": {
					Type:        "string",
					Description: "分类，可选值：preference（用户偏好）/ project（项目知识）/ interaction（重要互动）。默认为 preference。",
				},
				"keywords": {
					Type:        "string",
					Description: "相关的 [[反向链接关键词]]，用 / 分隔。例如「简短/精炼/风格偏好」。写进去后这些词之间会自动建立关联，下次提到任意一个都能联想召回本条。",
				},
			},
			Required: []string{"text"},
		},
	},
}

// handleRemember 处理 remember 工具调用，写入 SwiftNet 记忆系统。
func handleRemember(argsJSON string) string {
	var args struct {
		Text     string `json:"text"`
		Cluster  string `json:"cluster"`
		Keywords string `json:"keywords"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "参数解析失败: " + err.Error()
	}
	args.Text = trimSpaces(args.Text)
	if args.Text == "" {
		return "text 不能为空，请带上要记住的内容。"
	}
	if args.Cluster == "" {
		args.Cluster = "preference"
	}

	// 如果 keywords 为空但 text 里有 [[...]]，自动提取
	kw := args.Keywords
	if kw == "" {
		kw = args.Text
	}

	result := swiftnet.Default().MemAppend(args.Text, args.Cluster, kw)
	if !result.OK {
		if result.MergedID != "" {
			return "已记住。该内容与已有记忆相似，已合并更新。"
		}
		return "写入失败: " + result.Err
	}
	return "已记住 ✅ 下次对话时我会自动想起这条。"
}

func trimSpaces(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	if start >= end {
		return ""
	}
	return s[start:end]
}
