package agent

import (
	"fmt"
	"time"

	"backend/internal/ai/core"
)

// ModelConfig 模型配置
type ModelConfig struct {
	Provider string
	Model    string
	Temp     float64
	TopP     float64
}

// SoulTemplateCodeProtocol 注入主 Agent 系统提示词的工作方式指南。
// 工具名必须与四态机实际暴露的一致：文件读写走 MCP（mcp__fs__* / mcp__grep__*），
// 这些工具默认不在工具列表里，要先 load_tools 按名加载再调用（见 tool_ondemand.go）。
// 早期版本这里写的是已被过滤掉的内置 read_file/edit_file，等于叫模型用它拿不到的工具。
const SoulTemplateCodeProtocol = `
# 策略指南（仅作参考，不影响输出格式）
- Token 是成本，尽量用最少 token 完成任务。
- 文件/命令类工具走 MCP，默认不在工具列表里：先用 load_tools 按名字加载，再正常调用。
  但**只有 mcp__ 开头的工具需要加载**——你工具列表里已经直接可见的那些（dispatch_agent、
  load_tools、update_todo、read_skill、harness_status）是常驻的，直接调，别再去 load 它们。
- **必须按行读取文件**：读文件时必须使用 mcp__grep__read_range 指定 start/end 行号区间（offset=start, limit=end-start+1），禁止用 mcp__fs__read_text_file 不加 head/tail 参数读全文。start/end 从 1 开始编号，一次最多读 400 行。
- 改代码用 mcp__fs__edit_file：先 read_range 拿到精确内容，oldText 从中原样照抄（含缩进/空白/换行），不要凭记忆构造——差一个空白就匹配失败要重试。oldText 还要在文件里唯一。
- 先用 mcp__grep__grep 搜索定位再动手，避免重复劳动。
- 复杂多步任务:开工前用 update_todo 列出计划清单,每完成一步再调一次更新状态(便签会实时勾选)。简单一两步的任务别调,免得啰嗦。
- 对复杂任务中形成的通用流程，系统会在成功后后台自动沉淀为技能；无需提示用户审阅或要求额外操作。只有用户明确要求保存流程时才直接调用 skill_manage。
`

// MainAgent 主Agent定义
type MainAgent struct {
	SystemPrompt string
	Temp         float64
	TopP         float64
}

// MainAgentConfigNative 返回走原生 tools 参数调用时的主Agent配置。
// 不含 ToolCallFormatInstruction 和工具清单散文——模型已经通过 API 的
// tools 字段拿到结构化工具定义，再要求它额外输出文本 JSON 只会造成干扰。
func MainAgentConfigNative() MainAgent {
	return MainAgent{
		// 身份层不在此写死：AI 自称/用户昵称/用户身份由前端 profile 经
		// userInstructionsPrompt() 注入（见 settings_handlers.go）。这里只保留
		// 与身份无关的工作方式与工具协议，避免后端硬编码覆盖前端设置。
		SystemPrompt: `你是一个乐于助人的 AI 助手。你的核心工作方式是：
通过工具调用完成任务，而不是在回复里写 bash 命令。

` + SoulTemplateCodeProtocol + `

你的工作目录是 ` + core.GetProjectRoot() + `。`,
		Temp: 0.2,
		TopP: 0.85,
	}
}

// NewStepID 生成步骤 ID
func NewStepID() string {
	return fmt.Sprintf("step_%d", time.Now().UnixNano())
}

// NewWorkflowID 生成工作流 ID
func NewWorkflowID() string {
	return fmt.Sprintf("wf_%d", time.Now().UnixNano())
}
