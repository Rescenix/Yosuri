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

// ChatTools 是杉汐所有可用的工具列表
var ChatTools = []ToolDefinition{
	// 1. 语义搜索代码库
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

	// 2. 读取文件
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

	// 3. 创建或覆盖文件
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

	// 4. 执行安全命令
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
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "write_blog",
			Description: "根据给定主题，在后台自动撰写一篇博客并发布到网站。",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"topic": {
						Type:        "string",
						Description: "博客的主题或标题，例如“我的重建之路”。",
					},
				},
				Required: []string{"topic"},
			},
		},
	},
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "web_search",
			Description: "针对当前问题进行联网搜索，获取实时信息。",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"query": {
						Type:        "string",
						Description: "搜索关键词或问题。",
					},
				},
				Required: []string{"query"},
			},
		},
	},
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "clean_memories",
			Description: "清理冗余或过时的记忆，优化记忆库。",
			Parameters: ToolParameters{
				Type:       "object",
				Properties: map[string]ToolProperty{}, // 空对象，表示无参数
				Required:   []string{},                // 显式空数组，不能省略
			},
		},
	},
}
