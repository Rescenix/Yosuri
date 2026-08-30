package handler

// native_knowledge_tools.go —— 外挂知识库工具的 Go 执行层。
//
// knowledge_search / knowledge_list 两个按需工具，配合 knowledge 包实现外挂 RAG：
//   - knowledge_search：按关键词检索知识库，返回相关片段
//   - knowledge_list：列出知识库现有文件清单（含大小/修改时间/片段数）
//
// 与 memory_search（agent 自身沉淀的记忆）区分：knowledge 是用户丢进去的外部文档。

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"backend/internal/ai/core"
	"backend/internal/knowledge"
)

var knowledgeSearchToolDef = core.ToolDefinition{
	Type: "function",
	Function: core.ToolFunctionDetail{
		Name: "knowledge_search",
		Description: "检索外挂知识库（~/rescene_data/knowledge/ 目录下用户存放的 md/txt/docx/pptx/pdf 文档），" +
			"返回与查询最相关的文本片段。当需要参考用户提供的文档、内部资料、规范文档时调用。" +
			"与 memory_search 的区别：本工具查的是用户外部文档，memory_search 查的是 agent 自己沉淀的记忆。",
		Parameters: core.ToolParameters{
			Type: "object",
			Properties: map[string]core.ToolProperty{
				"query": {Type: "string", Description: "检索关键词或问题"},
				"top_k": {Type: "integer", Description: "返回片段数，默认 3，最大 5"},
			},
			Required: []string{"query"},
		},
	},
}

var knowledgeListToolDef = core.ToolDefinition{
	Type: "function",
	Function: core.ToolFunctionDetail{
		Name:        "knowledge_list",
		Description: "列出外挂知识库（~/rescene_data/knowledge/）当前有哪些文档，含文件名、大小、修改时间、片段数。用于先了解库里有什么，再决定搜什么。",
		Parameters: core.ToolParameters{
			Type:       "object",
			Properties: map[string]core.ToolProperty{},
		},
	},
}

func callNativeKnowledgeTool(name, argsJSON string) (nativeToolResult, error) {
	switch name {
	case "knowledge_search":
		var args struct {
			Query string `json:"query"`
			TopK  int    `json:"top_k"`
		}
		if err := json.Unmarshal([]byte(defaultJSONObject(argsJSON)), &args); err != nil {
			return nativeToolResult{}, fmt.Errorf("参数解析失败: %w", err)
		}
		if args.Query == "" {
			return nativeToolResult{}, fmt.Errorf("query 不能为空")
		}
		if args.TopK <= 0 {
			args.TopK = 3
		}
		if args.TopK > 5 {
			args.TopK = 5
		}
		hit := knowledge.Search(args.Query, args.TopK)
		if hit == "" {
			return nativeToolResult{Text: fmt.Sprintf("知识库中没有与 %q 相关的内容（或知识库为空，把文档放进 %s 即可）。", args.Query, knowledge.Dir())}, nil
		}
		return nativeToolResult{Text: hit}, nil

	case "knowledge_list":
		files := knowledge.ListFiles()
		if len(files) == 0 {
			return nativeToolResult{Text: fmt.Sprintf("知识库为空。把 md/txt/docx/pptx/pdf 文档放进 %s 即可。", knowledge.Dir())}, nil
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("知识库共 %d 个文件（%s）：\n\n", len(files), knowledge.Dir()))
		for i, f := range files {
			b.WriteString(fmt.Sprintf("%d. %s（%s，%s）\n",
				i+1, f.Name, humanSize(f.Size), tsCompact(f.ModTime)))
		}
		return nativeToolResult{Text: b.String()}, nil

	default:
		return nativeToolResult{}, fmt.Errorf("未知知识库工具: %s", name)
	}
}

// humanSize 把字节数格式化成可读大小。
func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// tsCompact 把 unix 秒格式化成 "08-30 15:04" 紧凑时间。
func tsCompact(unix int64) string {
	return time.Unix(unix, 0).Format("01-02 15:04")
}
