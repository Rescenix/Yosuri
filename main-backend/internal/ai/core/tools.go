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
			Name:        "codebase_query",
			Description: "【最高优先级】这是你感知项目代码结构的主要方式。通过精确的代码符号名，从AST级代码知识图谱中查询其定义位置、调用关系等。当你需要查找函数、类、结构体、接口等任何代码实体的定义时，必须优先使用此工具。",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"query": {
						Type:        "string",
						Description: "需要查询的精确代码符号名，例如 'prism_insert' 或 'ChaoticState'。这是你从用户问题中提取出的核心代码实体名称。",
					},
				},
				Required: []string{"query"},
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
			Name:        "clean_memories",
			Description: "清理冗余或过时的记忆，优化记忆库。",
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

// MobileTools 是手机端专有的工具
// MobileTools 是杉汐作为“人”的本能——她身体的自然能力
// MobileTools 是杉汐作为“人”的本能——她身体的自然能力

// ChatTools 默认是 Windows 全量工具
var ChatTools []ToolDefinition

func init() {
	ChatTools = append(ChatTools, BaseTools...)
	ChatTools = append(ChatTools, WindowsTools...)

}
