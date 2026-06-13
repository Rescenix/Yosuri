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
