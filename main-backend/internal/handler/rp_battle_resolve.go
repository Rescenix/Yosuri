package handler

// rp_battle_resolve.go —— 回合结算 + HTTP 接口。
//
// 结算规则（自研轻量回合制，主打「看得懂的爽」）：
//   - 行动顺序按速度降序，同速我方先手
//   - 普通攻击：伤害 = 攻 - 防*0.5 + 随机±20%；保底 1 点
//   - 暴击：15% 概率，1.6 倍
//   - 防御：本回合 DEF 翻倍（下一回合行动时自动重置）
//   - 护盾先吃伤害（残盾值计入白条，盾破才掉血）
//   - 元素克制：火>草>水>雷>火、光>暗>光 三组克制，克制 1.3 倍
//   - 敌方 AI：每回合 70% 攻击 / 30% 防御，血量 <30% 时 50% 用技能（1.4 倍，耗魔）
//
// 战斗结束自动写回角色卡：我方存活角色扣到当前 HP、掉落金币平分给全员、
// 战败时全员 HP 保底 1（剧情不允许团灭重开）。

import (
	"fmt"
	"math/rand"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// elementBonus 元素克制关系：a 打 b 是否克制。返回倍数。
func elementBonus(a, b string) float64 {
	if a == "" || b == "" || a == b {
		return 1.0
	}
	pairs := map[string]bool{
		"fire_grass": true, "grass_water": true, "water_thunder": true, "thunder_fire": true,
		"holy_dark": true,
	}
	if pairs[a+"_"+b] {
		return 1.3
	}
	return 1.0
}

// resolveUnit 结算单个单位的一次行动（前端按按钮逐格驱动）。
// 调用方持锁。敌方传 action 会被忽略，AI 自己选。
func (b *RPBattle) resolveUnit(unitID, action string) ([]BattleEvent, error) {
	u := b.unitByID(unitID)
	if u == nil {
		return nil, fmt.Errorf("单位不存在")
	}
	if !u.Alive {
		return nil, fmt.Errorf("%s 已经倒下", u.Name)
	}
	// 防御状态行动前自动解除（上一回合摆防御的）
	if u.Defending {
		u.Defending = false
	}
	var evs []BattleEvent
	if u.Side == "enemy" {
		// 敌方 AI：70% 攻 / 30% 防，残血 50% 技能
		action = "attack"
		if rand.Intn(10) < 3 {
			action = "defend"
		} else if u.HP*3 < u.MaxHP && rand.Intn(10) < 5 {
			action = "skill"
		}
	} else if action == "" {
		action = "attack"
	}
	switch action {
	case "defend":
		u.Defending = true
		evs = append(evs, BattleEvent{Turn: b.Turn, Actor: u.Name, Action: "defend", Message: u.Name + " 摆出防御姿态"})
	case "skill":
		b.applySkill(u, &evs)
	default:
		b.applyAttack(u, &evs)
	}
	return evs, nil
}

// resolveTurn 结算一回合：按速度排序轮流行动，返回事件列表。
// 调用方持锁。我方动作来自 pending 队列（玩家在回合开始前提交）；敌方自动。
// 防御：行动前解除上一回合姿态，行动时选防御则设标记，本回合后续攻击减伤。
func (b *RPBattle) resolveTurn(actions map[string]string) []BattleEvent {
	var evs []BattleEvent
	order := make([]int, 0, len(b.Units))
	for i := range b.Units {
		if b.Units[i].Alive {
			order = append(order, i)
		}
	}
	b.sortSpeed(order)

	for _, idx := range order {
		u := &b.Units[idx]
		if !u.Alive {
			continue
		}
		// 防御状态行动前自动解除（上一回合摆防御的）
		if u.Defending {
			u.Defending = false
		}
		action := ""
		if u.Side == "ally" {
			action = actions[u.ID]
			if action == "" {
				action = "attack"
			}
		} else {
			// 敌方 AI：70% 攻 / 30% 防，残血 50% 技能
			action = "attack"
			if rand.Intn(10) < 3 {
				action = "defend"
			} else if u.HP*3 < u.MaxHP && rand.Intn(10) < 5 {
				action = "skill"
			}
		}
		switch action {
		case "defend":
			u.Defending = true
			evs = append(evs, BattleEvent{Turn: b.Turn, Actor: u.Name, Action: "defend", Message: u.Name + " 摆出防御姿态"})
		case "skill":
			b.applySkill(u, &evs)
		default:
			b.applyAttack(u, &evs)
		}
		if b.overLocked() {
			break
		}
	}
	b.Turn++
	return evs
}

// pickEnemyTarget 选敌方目标（当前只有单敌，未来可扩展多敌）。
func (b *RPBattle) pickEnemyTarget() *BattleUnit {
	for i := range b.Units {
		if b.Units[i].Alive && b.Units[i].Side == "enemy" {
			return &b.Units[i]
		}
	}
	return nil
}

// pickAllyTarget 敌方打我方的随机目标。
func (b *RPBattle) pickAllyTarget() *BattleUnit {
	var idx []int
	for i := range b.Units {
		if b.Units[i].Alive && b.Units[i].Side == "ally" {
			idx = append(idx, i)
		}
	}
	if len(idx) == 0 {
		return nil
	}
	return &b.Units[idx[rand.Intn(len(idx))]]
}

// applyAttack 一次普通攻击结算。
func (b *RPBattle) applyAttack(u *BattleUnit, evs *[]BattleEvent) {
	var target *BattleUnit
	if u.Side == "ally" {
		target = b.pickEnemyTarget()
	} else {
		target = b.pickAllyTarget()
	}
	if target == nil {
		return
	}
	atk := float64(u.ATK)
	crit := rand.Intn(100) < 15
	mult := elementBonus(u.Element, target.Element)
	dmg := atk - float64(target.DEF)*0.5
	// 防御：本回合摆出防御姿态的单位受伤减半（行动前标记已由上一回合解除）
	if target.Defending {
		dmg *= 0.5
	}
	dmg *= (0.8 + rand.Float64()*0.4) // ±20%
	if crit {
		dmg *= 1.6
	}
	dmg *= mult
	// 保底 1 点：所有倍率算完后再兜底——之前写在防御减半前面，
	// 防御 ×0.5 再乘随机下限 → 0.4 → int() 归 0，防御单位永远打不死（战斗结束不了的根因）。
	if dmg < 1 {
		dmg = 1
	}
	deal := target.absorb(int(dmg + 0.5))
	msg := fmt.Sprintf("%s 攻击 %s，造成 %d 点伤害", u.Name, target.Name, deal)
	if crit {
		msg += "（暴击！）"
	}
	if mult > 1.01 {
		msg += "（属性克制）"
	}
	if u.Shield > 0 {
		msg += "（护盾吸收）"
	}
	*evs = append(*evs, BattleEvent{Turn: b.Turn, Actor: u.Name, Action: "attack", Target: target.Name, Amount: deal, Message: msg})
}

// applySkill 技能：1.5 倍伤害，耗 8 魔，不够魔就普攻。
func (b *RPBattle) applySkill(u *BattleUnit, evs *[]BattleEvent) {
	if u.MP < 8 && u.MaxMP > 0 {
		b.applyAttack(u, evs)
		return
	}
	if u.MaxMP > 0 {
		u.MP -= 8
	}
	var target *BattleUnit
	if u.Side == "ally" {
		target = b.pickEnemyTarget()
	} else {
		target = b.pickAllyTarget()
	}
	if target == nil {
		return
	}
	mult := elementBonus(u.Element, target.Element)
	dmg := (float64(u.ATK)*0.5 + float64(u.MAG)*1.0) * 1.5 * mult
	if dmg < 2 {
		dmg = 2
	}
	deal := target.absorb(int(dmg + 0.5))
	msg := fmt.Sprintf("%s 释放技能，对 %s 造成 %d 点伤害", u.Name, target.Name, deal)
	if mult > 1.01 {
		msg += "（属性克制）"
	}
	*evs = append(*evs, BattleEvent{Turn: b.Turn, Actor: u.Name, Action: "skill", Target: target.Name, Amount: deal, Message: msg})
}

// absorb 承受伤害：先扣护盾再扣血。返回实际扣除量。
func (u *BattleUnit) absorb(dmg int) int {
	deal := dmg
	if u.Shield > 0 {
		if u.Shield >= dmg {
			u.Shield -= dmg
			return dmg
		}
		dmg -= u.Shield
		u.Shield = 0
	}
	u.HP -= dmg
	if u.HP <= 0 {
		u.HP = 0
		u.Alive = false
	}
	return deal
}

// overLocked 是否有一方全灭。
func (b *RPBattle) overLocked() bool {
	allies, enemies := b.aliveSides()
	return allies == 0 || enemies == 0
}

// finish 战斗结束：写回角色卡（调用方持锁）。
// 死亡规则：人物会死——HP 归 0 的角色保持死亡（Alive=false，HP=0）。
// 主角（cast[0]，即 BattleUnit 第一个我方）死亡 = 游戏结束，置 GameOver。
// 战利品：胜利时敌人 Loot 进全员背包（同名物品叠加数量）。
func (b *RPBattle) finish() {
	allies, enemies := b.aliveSides()
	b.Over = true
	b.Victory = allies > 0 && enemies == 0
	ep := enemyPresets[strings.TrimPrefix(b.enemyID(), "enemy:")]
	if b.Victory {
		if ep.Key != "" {
			b.GoldGain = ep.Gold
		}
		// 战利品快照：前端终局/结果面板展示用（每人一份）。
		if ep.Loot.Name != "" {
			loot := ep.Loot
			loot.Count = b.allyCount()
			b.Loot = &loot
		}
	}
	// 写回每个我方角色：按战况扣 HP；胜利发金币 + 战利品
	for i := range b.Units {
		u := &b.Units[i]
		if u.Side != "ally" {
			continue
		}
		if c := GetAgentCard(u.ID); c != nil {
			st := c.Stats
			st.HP = u.HP
			if b.Victory {
				if b.GoldGain > 0 {
					st.Gold += b.GoldGain / b.allyCount()
				}
				// 经验：胜利每人拿敌人经验（仅存活者；阵亡不拿）
				if u.Alive && ep.Exp > 0 {
					st.Exp += ep.Exp
					st = levelUpIfNeeded(st)
				}
				// 战利品进背包：同名叠加数量
				if ep.Loot.Name != "" {
					found := false
					for j := range st.Inventory {
						if st.Inventory[j].Name == ep.Loot.Name {
							st.Inventory[j].Count++
							found = true
							break
						}
					}
					if !found {
						item := ep.Loot
						item.Count = 1
						st.Inventory = append(st.Inventory, item)
					}
				}
			}
			c.Stats = st // ★ 值拷贝必须写回，否则 UPSERT 存的是没改的副本
			_, _ = UpsertAgent(*c)
		}
	}
	// 主角（第一个我方单位）死了 → 游戏结束
	if !b.Victory && len(b.Units) > 0 && b.Units[0].Side == "ally" && !b.Units[0].Alive {
		b.GameOver = true
	}
}

// summaryText 战斗一句话总结（模型拿到后接戏叙述战果/终局）。
func (b *RPBattle) summaryText() string {
	var alive []string
	var dead []string
	for i := range b.Units {
		u := &b.Units[i]
		if u.Side == "enemy" {
			continue
		}
		if u.Alive {
			alive = append(alive, u.Name)
		} else {
			dead = append(dead, u.Name)
		}
	}
	ep := enemyPresets[strings.TrimPrefix(b.enemyID(), "enemy:")]
	if b.GameOver {
		return fmt.Sprintf("主角（%s）倒下了——世界线走向终局。阵亡：%s；存活：%s。",
			b.Units[0].Name, joinNames(dead), joinNames(alive))
	}
	if b.Victory {
		loot := ""
		if ep.Loot.Name != "" {
			loot = "，获得战利品「" + ep.Loot.Name + "」" + fmt.Sprintf("×%d", b.allyCount())
		}
		return fmt.Sprintf("胜利！击败了%s%s。存活：%s；阵亡：%s。",
			ep.Name, loot, joinNames(alive), joinNames(dead))
	}
	return fmt.Sprintf("战败。阵亡：%s；存活：%s。", joinNames(dead), joinNames(alive))
}

func joinNames(names []string) string {
	if len(names) == 0 {
		return "无"
	}
	return strings.Join(names, "、")
}

func (b *RPBattle) allyCount() int {
	n := 0
	for i := range b.Units {
		if b.Units[i].Side == "ally" {
			n++
		}
	}
	return n
}

func (b *RPBattle) enemyID() string {
	for i := range b.Units {
		if b.Units[i].Side == "enemy" {
			return b.Units[i].ID
		}
	}
	return ""
}

// ── HTTP 接口 ──

// HandleRPBattleStart POST /api/rp/battle/start
// body: {session_id, agent_ids: [..], enemy: "slime"}
func HandleRPBattleStart(c *gin.Context) {
	var req struct {
		SessionID string   `json:"session_id"`
		AgentIDs  []string `json:"agent_ids"`
		Enemy     string   `json:"enemy"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体不是合法 JSON"})
		return
	}
	if req.Enemy == "" {
		req.Enemy = "slime"
	}
	b, err := newRPBattle(req.SessionID, req.AgentIDs, req.Enemy)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// 不自动结算：摆好战场就返回，每一步都由前端按按钮驱动（resolveUnit 逐个结算）。
	c.JSON(http.StatusOK, gin.H{"ok": true, "battle": b, "events": []BattleEvent{}})
}

// HandleRPBattleAction POST /api/rp/battle/action
// body: {battle_id, actor_id, action: "attack"|"defend"|"skill"}
func HandleRPBattleAction(c *gin.Context) {
	var req struct {
		BattleID string `json:"battle_id"`
		ActorID  string `json:"actor_id"`
		Action   string `json:"action"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体不是合法 JSON"})
		return
	}
	b := getRPBattle(req.BattleID)
	if b == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "战斗不存在或已结束"})
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.Over {
		c.JSON(http.StatusOK, gin.H{"ok": true, "battle": b, "events": []BattleEvent{}})
		return
	}
	evs, err := b.resolveUnit(req.ActorID, req.Action)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if b.overLocked() {
		b.finish()
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "battle": b, "events": evs})
}

// HandleRPBattleTurn POST /api/rp/battle/turn
// body: {battle_id, actions: {"<unitID>": "attack"|"defend"|"skill", ...}}
// 结算一整回合：我方按提交的动作行动（没提交的默认攻击），敌方 AI 自动，
// 按速度排序逐单位结算；一方全灭即收尾写回角色卡。返回本回合全部事件 + 最新战场。
func HandleRPBattleTurn(c *gin.Context) {
	var req struct {
		BattleID string            `json:"battle_id"`
		Actions  map[string]string `json:"actions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体不是合法 JSON"})
		return
	}
	b := getRPBattle(req.BattleID)
	if b == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "战斗不存在或已结束"})
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.Over {
		c.JSON(http.StatusOK, gin.H{"ok": true, "battle": b, "events": []BattleEvent{}})
		return
	}
	evs := b.resolveTurn(req.Actions)
	if b.overLocked() {
		b.finish()
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "battle": b, "events": evs})
}

// HandleRPBattleGet GET /api/rp/battle/:id —— 断线/刷新后取回状态。
func HandleRPBattleGet(c *gin.Context) {
	b := getRPBattle(c.Param("id"))
	if b == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "战斗不存在"})
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	c.JSON(http.StatusOK, gin.H{"ok": true, "battle": b})
}
