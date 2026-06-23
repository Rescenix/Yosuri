package mobile

import "backend/internal/ai/core"

var ChatTools []core.ToolDefinition

func init() {
	ChatTools = append(ChatTools, core.BaseTools...)

}
