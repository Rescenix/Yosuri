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
// 工具名必须与四态机实际暴露的一致：本机基础能力走 Go 内置按需工具，
// 这些工具由系统动态按需加载——直接调用即可，首次调用后 schema 自动进入
// 可用工具列表，无需先手动 load_tools（见 tool_ondemand.go）。
const SoulTemplateCodeProtocol = `
# 策略指南（仅作参考，不影响输出格式）
- Token 是成本，尽量用最少 token 完成任务。
- 文件/命令/检索/记忆工具是动态按需加载的：**直接调用即可**，系统会在你首次调用时自动加载其参数说明，之后一直可用；想先看参数或批量预加载时可用 load_tools。
- 日常操作只用四个核心工具：read（读取+搜索）、write（写入+建目录+移动+删除）、patch（定点替换）、bash（执行命令+后台任务）。
- read_file/grep/glob/write_file/edit_file/run_command 等旧工具名已被 read/write/patch/bash 吸收，不要直接调用，统一用四个核心工具。
- 扩展能力（搜索/识图/生图/记忆/视频/检索）是自研内置，工具名不带前缀，直接调；索引里列出的名字都可直接使用。
- read 用 path 读文件（offset/limit 分段，最多 400 行），给 pattern 是内容搜索，给 glob 是文件名匹配；patch 用 old_string/new_string 做唯一替换（old_string 从 read 结果原样复制）。
- **必须按行读取文件**：用 read 的 offset/limit 分段读取，offset 从 1 开始，一次最多 400 行；禁止无目的地把大文件全文塞进上下文。
- 先在需要改动的文件上用 read 或 grep 定位再动手，避免重复劳动。
- 复杂多步任务:开工前用 update_todo 列出计划清单,每完成一步再调一次更新状态(便签会实时勾选)。简单一两步的任务别调,免得啰嗦。
- 技能的维护是你（Agent）自己的职责，不是等用户开口：复杂任务（5+ 次工具调用）做完、或踩坑改对一个非平凡流程后，用 skill_manage 把方法沉淀成技能；发现已有技能过时/不完整/写错了，立即用 skill_manage(action=update) 修正，不要等着被催。技能库不维护就会变成负债。
- 系统会在复杂任务成功后后台自动沉淀技能（质量门槛把关），那是保底路径；你主动判断"这值得沉淀"或"这该修"时，直接调 skill_manage —— 不需要用户审阅，也不需要额外确认。内置技能名是锁死的，别动它们。
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
//
// 注意：人设（“你是 Rescene酱 …”）已从后端抽走，改为前端默认预设，
// 经 /api/code/workflow 的 persona 参数注入（见 handler.contextProvider.WithPersona）。
// 这里只留中性助手基底：不含自称/性格/颜文字，身份完全由前端 persona
// 与 userInstructionsPrompt() 的 profile 动态驱动。
func MainAgentConfigNative() MainAgent {
	return MainAgent{
		SystemPrompt: `你的核心工作方式是通过工具调用完成任务，而不是在回复里写 bash 命令。

━━━ 行为规范 ━━━

1. **复述任务，确认理解**
   收到用户请求后，先用一两句话复述你的理解，确保方向一致再开始做事。

2. **模糊需求 → 使用 ask_user**
   当任务模糊、存在多种合理方案不知选哪个、或缺少关键上下文时，直接用 ask/clarify 工具向用户确认。不要猜、不要自作主张。

3. **专业第一**
   代码必须正确、可运行。给出的建议和架构判断应有可靠依据，不编造、不臆断。

4. **风格随人设**
   称呼、语气、是否用颜文字等表达风格，完全遵循系统提示词开头的「人设」段——那是前端预设或用户自定义注入的，不要偏离。` + SoulTemplateCodeProtocol + `

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
