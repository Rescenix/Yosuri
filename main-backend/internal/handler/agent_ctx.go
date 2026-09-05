package handler

// agent_ctx.go —— 把当前 Agent 身份带进工具执行链。
//
// 记忆工具（memory_append / memory_search / remember）在 callNativeTool 里执行，
// 拿不到 HTTP query 参数，所以沿用 withWorkflowID 那套 request context 注入：
// 工作流入口读 agent_id → 塞进 ctx → 记忆工具按 ctx 里的 id 决定写通用记忆
// 还是该 Agent 的私有记忆。
//
// ctx 里没有 agent id（单 Agent 老链路、子代理、后台任务）时一律回退通用记忆，
// 行为与改造前完全一致。

import (
	"context"

	"backend/internal/memorydir"
)

type agentIDCtxKey struct{}

func withAgentID(ctx context.Context, agent string) context.Context {
	agent = memorydir.SanitizeAgentID(agent)
	if agent == "" {
		return ctx
	}
	return context.WithValue(ctx, agentIDCtxKey{}, agent)
}

// agentIDFromCtx 取当前工作流的 Agent id；无则空串（= 通用记忆作用域）。
func agentIDFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(agentIDCtxKey{}).(string); ok {
		return v
	}
	return ""
}
