package handler

// rp_tools_chain_test.go —— RP 编排工具的完整可执行链验证。
// 「未知工具 rp_battle_start」bug 回归测试：模型调 RP 工具被 executeCodeCalls
// 兜底拒掉（isNativeExecutableTool 不认识 RP 工具名）。本测试断言两个 RP 工具
// 都能通过可执行判定 + 真实派发到 callNativeTool 并正确执行。

import (
	"context"
	"testing"
)

// TestRPToolExecutable 两个 RP 工具必须被判定为「可执行工具」——
// 否则 executeCodeCalls 兜底分支会甩「未知工具」（2026-09-14 实锤）。
func TestRPToolExecutable(t *testing.T) {
	for _, name := range []string{"rp_set_stat", "rp_battle_start"} {
		if !isNativeExecutableTool(name) {
			t.Errorf("%s 不在可执行工具名单里 → 模型一调就报未知工具", name)
		}
		if !rpToolNames()[name] {
			t.Errorf("%s 不在 RP 硬闸放行名单里 → 被 RP 拦下", name)
		}
	}
}

// TestRPToolDispatch 走到 callNativeTool 派发分支：RP 工具必须能真实执行。
func TestRPToolDispatch(t *testing.T) {
	// 准备一个可开战的角色卡
	hero := AgentCard{ID: "rp-chain-hero", Name: "链测主角"}
	hero.Stats = AgentStats{Level: 1, HP: 100, MaxHP: 100, ATK: 15, DEF: 10, SPD: 9, MAG: 4}
	if _, err := UpsertAgent(hero); err != nil {
		t.Fatalf("建卡失败: %v", err)
	}
	t.Cleanup(func() { _ = DeleteAgent("rp-chain-hero") })

	ctx := withRPCast(context.Background(), []string{"rp-chain-hero"})

	// 1) callNativeTool 必须认得 rp_battle_start
	res, err := callNativeTool(ctx, "rp_battle_start", `{"enemy":"slime"}`)
	if err != nil {
		t.Fatalf("rp_battle_start 派发失败: %v", err)
	}
	if res.Battle == nil {
		t.Fatal("rp_battle_start 没有返回战场快照")
	}
	if len(res.BattleEvents) == 0 {
		t.Fatal("自动结算没有产出事件时间轴")
	}
	t.Logf("开战 OK: %s, 事件 %d 条, 战利品 %+v", res.Battle.summaryText(), len(res.BattleEvents), res.Battle.Loot)

	// 2) callNativeTool 必须认得 rp_set_stat（迷你改值）
	sres, err := callNativeTool(ctx, "rp_set_stat", `{"command":"set","agent":"rp-chain-hero","field":"gold","value":"66"}`)
	if err != nil {
		t.Fatalf("rp_set_stat 派发失败: %v", err)
	}
	after := GetAgentCard("rp-chain-hero")
	if after == nil || after.Stats.Gold != 66 {
		t.Fatalf("rp_set_stat 改值没生效: gold=%v (want 66), 返回=%s", after.Stats.Gold, sres.Text)
	}
	t.Logf("rp_set_stat OK: %s", sres.Text)
}

// TestRPToolsInRPDefs RP 模式工具面必须同时暴露两个编排工具。
func TestRPToolsInRPDefs(t *testing.T) {
	got := map[string]bool{}
	for _, d := range rpToolDefs() {
		got[d.Function.Name] = true
	}
	for _, want := range []string{"rp_set_stat", "rp_battle_start"} {
		if !got[want] {
			t.Errorf("RP 工具面缺少 %s（模型根本看不到这个工具）", want)
		}
	}
}