package handler

// rp_battle_test.go —— RP 战斗全链路验证（测试内自建角色卡，真实跑结算）。
// 用户要拍视频宣传「能开打的酒馆」，先在这里证明闭环真实可跑：
// 喂基本设定 → 自动结算 → 胜利掉落进背包 → 主角死亡=终局。
// 测试不产二进制，end-to-end 走内存。

import (
	"testing"
	"time"
)

// TestRPBattleFullLoop 全链路：主角+队友打史莱姆，胜利→金币+战利品进背包。
func TestRPBattleFullLoop(t *testing.T) {
	// 喂基本设定：两个角色（主角=骑士，队友=法师）
	hero := AgentCard{ID: "rp-hero", Name: "骑士小雅", Persona: "勇敢的骑士"}
	hero.Stats = AgentStats{Level: 1, HP: 120, MaxHP: 120, ATK: 12, DEF: 14, SPD: 6, MAG: 4, Gold: 0}
	if _, err := UpsertAgent(hero); err != nil {
		t.Fatalf("建主角卡失败: %v", err)
	}
	companion := AgentCard{ID: "rp-mage", Name: "法师小焰", Persona: "火系法师"}
	companion.Stats = AgentStats{Level: 1, HP: 70, MaxHP: 70, ATK: 6, DEF: 5, SPD: 7, MAG: 18, Element: "fire", Gold: 0}
	if _, err := UpsertAgent(companion); err != nil {
		t.Fatalf("建队友卡失败: %v", err)
	}
	t.Cleanup(func() { _ = DeleteAgent("rp-hero"); _ = DeleteAgent("rp-mage") })

	// 开战 + 自动结算完整场（slime 史莱姆）
	b, err := newRPBattle("", []string{"rp-hero", "rp-mage"}, "slime")
	if err != nil {
		t.Fatalf("开战失败: %v", err)
	}
	b.mu.Lock()
	events := 0
	for !b.overLocked() && b.Turn <= 30 {
		evs := b.resolveTurn(map[string]string{})
		events += len(evs)
		if b.overLocked() {
			break
		}
	}
	b.finish()
	b.mu.Unlock()

	if events == 0 {
		t.Fatal("战斗没有任何事件发生")
	}
	if !b.Over {
		t.Fatal("战斗没有正常结束")
	}
	t.Logf("回合数=%d 事件数=%d 胜利=%v 金币=%d 终局=%v",
		b.Turn, events, b.Victory, b.GoldGain, b.GameOver)

	// 验证状态写回：不管胜负，背包字段应可用；若胜利应有战利品
	heroAfter := GetAgentCard("rp-hero")
	mageAfter := GetAgentCard("rp-mage")
	if heroAfter == nil || mageAfter == nil {
		t.Fatal("角色卡写回后读不到了")
	}
	t.Logf("主角: HP=%d/%d 金币=%d 背包=%v", heroAfter.Stats.HP, heroAfter.Stats.MaxHP, heroAfter.Stats.Gold, heroAfter.Stats.Inventory)
	t.Logf("队友: HP=%d/%d 金币=%d 背包=%v", mageAfter.Stats.HP, mageAfter.Stats.MaxHP, mageAfter.Stats.Gold, mageAfter.Stats.Inventory)

	if b.Victory {
		if heroAfter.Stats.Gold == 0 && mageAfter.Stats.Gold == 0 {
			t.Errorf("胜利但没有发金币")
		}
		if len(heroAfter.Stats.Inventory) == 0 && len(mageAfter.Stats.Inventory) == 0 {
			t.Errorf("胜利但战利品没进背包")
		}
	}
}

// TestRPBattleGameOver 主角死亡 → 游戏结束（终局结算的前提）。
func TestRPBattleGameOver(t *testing.T) {
	solo := AgentCard{ID: "rp-solo", Name: "独行侠"}
	solo.Stats = AgentStats{Level: 1, HP: 90, MaxHP: 90, ATK: 10, DEF: 5, SPD: 8, MAG: 4, Gold: 0}
	if _, err := UpsertAgent(solo); err != nil {
		t.Fatalf("建卡失败: %v", err)
	}
	t.Cleanup(func() { _ = DeleteAgent("rp-solo") })

	// 打幼龙：数据上必败，验证终局标记
	b, err := newRPBattle("", []string{"rp-solo"}, "dragon")
	if err != nil {
		t.Fatalf("开战失败: %v", err)
	}
	b.mu.Lock()
	for !b.overLocked() && b.Turn <= 30 {
		if b.overLocked() {
			break
		}
		b.resolveTurn(map[string]string{})
	}
	b.finish()
	gameOver := b.GameOver
	victory := b.Victory
	b.mu.Unlock()

	if victory {
		t.Fatal("独行侠打幼龙不该赢")
	}
	after := GetAgentCard("rp-solo")
	if after == nil {
		t.Fatal("角色卡读不到")
	}
	t.Logf("终局=%v 主角HP=%d（0=已死亡）", gameOver, after.Stats.HP)
	if !gameOver {
		t.Fatal("主角死亡但没有标记游戏结束")
	}
	if after.Stats.HP != 0 {
		t.Fatalf("主角 HP 应为 0（死亡），实际 %d", after.Stats.HP)
	}
}

// TestRPBattleSummary 结算文本（Yosuri 接戏叙述用）可读。
func TestRPBattleSummary(t *testing.T) {
	hero := AgentCard{ID: "rp-sum-hero", Name: "测试主角"}
	hero.Stats = AgentStats{Level: 1, HP: 100, MaxHP: 100, ATK: 20, DEF: 15, SPD: 12, MAG: 5}
	if _, err := UpsertAgent(hero); err != nil {
		t.Fatalf("建卡失败: %v", err)
	}
	t.Cleanup(func() { _ = DeleteAgent("rp-sum-hero") })

	b, err := newRPBattle("", []string{"rp-sum-hero"}, "goblin")
	if err != nil {
		t.Fatalf("开战失败: %v", err)
	}
	b.mu.Lock()
	for !b.overLocked() && b.Turn <= 30 {
		if b.overLocked() {
			break
		}
		b.resolveTurn(map[string]string{})
	}
	b.finish()
	summary := b.summaryText()
	b.mu.Unlock()

	if summary == "" {
		t.Fatal("结算文本为空")
	}
	t.Logf("结算文本: %s", summary)
	now := time.Now()
	_ = now
}