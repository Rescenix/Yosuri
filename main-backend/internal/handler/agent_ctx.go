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

type rpCastCtxKey struct{}

// withRPCast 把 RP 会话的 cast（多角色同框的角色卡 id 列表）塞进 ctx，
// 供 rp_battle_start 工具开战时读（战斗要带全场角色上场，不是只带主卡）。
func withRPCast(ctx context.Context, cast []string) context.Context {
	var clean []string
	for _, id := range cast {
		if s := memorydir.SanitizeAgentID(id); s != "" {
			clean = append(clean, s)
		}
	}
	if len(clean) == 0 {
		return ctx
	}
	return context.WithValue(ctx, rpCastCtxKey{}, clean)
}

// rpCastFromCtx 取当前 RP 会话的 cast；无则空。
func rpCastFromCtx(ctx context.Context) []string {
	if v, ok := ctx.Value(rpCastCtxKey{}).([]string); ok {
		return v
	}
	return nil
}
