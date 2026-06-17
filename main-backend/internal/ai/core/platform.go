package core

// PlatformProvider 定义了平台特化组件必须提供的能力
type PlatformProvider interface {
	// GetSystemPrompt 返回该平台专用的系统提示词
	GetSystemPrompt() string
	// GetTools 返回该平台专用的工具定义列表
	GetTools() []ToolDefinition
	// GetToolExecutor 返回该平台专用的工具执行器
	GetToolExecutor() ToolExecutor
}

// ToolExecutor 定义了工具执行的接口
type ToolExecutor interface {
	ExecuteToolCall(call ToolCall) (*ToolResult, error)
}
