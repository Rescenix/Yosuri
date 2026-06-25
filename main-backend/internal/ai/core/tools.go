package core

// ToolDefinition 是一个通用的工具定义结构，最终会序列化为 JSON 传给 API
type ToolDefinition struct {
	Type     string             `json:"type"`
	Function ToolFunctionDetail `json:"function"`
}

type ToolFunctionDetail struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  ToolParameters `json:"parameters"`
}

type ToolParameters struct {
	Type       string                  `json:"type"`
	Properties map[string]ToolProperty `json:"properties"`
	Required   []string                `json:"required"`
}

type ToolProperty struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
}

// BaseTools 是电脑和手机共用的工具
var BaseTools = []ToolDefinition{
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "search_codebase",
			Description: "语义搜索代码库，返回与查询最相关的函数、模块和代码片段。",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"query": {
						Type:        "string",
						Description: "搜索查询，例如 '用户登录逻辑' 或 '向量检索实现'",
					},
				},
				Required: []string{"query"},
			},
		},
	},
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "read_file",
			Description: "读取项目中的指定文件内容，返回完整文本。",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"path": {
						Type:        "string",
						Description: "文件路径，相对于项目根目录（如 'internal/ai/core/tools.go'）",
					},
				},
				Required: []string{"path"},
			},
		},
	},

	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "write_file",
			Description: "创建或覆盖项目中的文件。此操作需要用户确认。",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"path": {
						Type:        "string",
						Description: "文件路径，相对于项目根目录",
					},
					"content": {
						Type:        "string",
						Description: "要写入的文件内容",
					},
				},
				Required: []string{"path", "content"},
			},
		},
	},
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "edit_file",
			Description: "精确编辑文件内容，通过替换指定旧字符串为新字符串实现修改。保留 diff 信息，要求 old_string 在文件中唯一。",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"path": {
						Type:        "string",
						Description: "要编辑的文件路径，相对于项目根目录",
					},
					"old_string": {
						Type:        "string",
						Description: "要替换的旧字符串，必须在文件中唯一匹配",
					},
					"new_string": {
						Type:        "string",
						Description: "替换后的新字符串",
					},
				},
				Required: []string{"path", "old_string", "new_string"},
			},
		},
	},
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "execute_command",
			Description: "在项目根目录执行一条安全的白名单 shell 命令。需要用户确认。",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"command": {
						Type:        "string",
						Description: "要执行的 shell 命令（如 'git diff'、'go build ./...'）",
					},
				},
				Required: []string{"command"},
			},
		},
	},
	// {
	// 	Type: "function",
	// 	Function: ToolFunctionDetail{
	// 		Name:        "search_memory",
	// 		Description: "搜索你的长期记忆库，找回与当前对话相关的历史记忆。当且仅当当前上下文不知道的时候调用此工具。",
	// 		Parameters: ToolParameters{
	// 			Type: "object",
	// 			Properties: map[string]ToolProperty{
	// 				"query": {
	// 					Type:        "string",
	// 					Description: "从用户消息中提取的搜索查询，用于检索相关记忆",
	// 				},
	// 			},
	// 			Required: []string{"query"},
	// 		},
	// 	},
	// },
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "clean_memories",
			Description: "清理冗余或过时的记忆，优化记忆库,禁止主动调用。",
			Parameters: ToolParameters{
				Type:       "object",
				Properties: map[string]ToolProperty{},
				Required:   []string{},
			},
		},
	},
}

// WindowsTools 是 Windows 电脑端专有的工具
var WindowsTools = []ToolDefinition{
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "codegraph_query",
			Description: "使用 CodeGraph 查询代码库的结构信息（如 callers, callees, impact, context 等）",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"subcommand": {
						Type:        "string",
						Description: "CodeGraph 子命令（callers, callees, impact, context, search, status 等）",
					},
					"symbol": {
						Type:        "string",
						Description: "要查询的符号名称（如函数名、类名）",
					},
				},
				Required: []string{"subcommand", "symbol"},
			},
		},
	},
}

// ChatTools 默认是 Windows 全量工具
var ChatTools []ToolDefinition

func init() {
	ChatTools = append(ChatTools, BaseTools...)
	ChatTools = append(ChatTools, WindowsTools...)
}
