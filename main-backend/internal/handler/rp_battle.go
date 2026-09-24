package handler

// rp_battle.go —— RP 酒馆战斗引擎（服务端结算，前端只发动作）。
//
// 卖点：普通酒馆只能纯聊天，这里能真正开打——cast 角色带着人物参数
// （HP/攻/防/速/元素/护盾）上场，回合制结算，打完自动写回角色卡。
// 逻辑是本仓库自研的简单回合制，不搬星迹服务端战斗（那是网游资产）。

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// BattleUnit 一个战斗单位（我方=角色卡，敌方=敌人模板）。
type BattleUnit struct {
	ID        string `json:"id"` // 我方=agent id；敌方=enemy:<key>
	Name      string `json:"name"`
	Avatar    string `json:"avatar,omitempty"` // 我方=角色卡头像 base64；敌方=元素 emoji 图标
	Side      string `json:"side"`             // ally | enemy
	HP        int    `json:"hp"`
	MaxHP     int    `json:"maxHp"`
	Shield    int    `json:"shield"`
	MP        int    `json:"mp"`
	MaxMP     int    `json:"maxMp"`
	ATK       int    `json:"atk"`
	DEF       int    `json:"def"`
	SPD       int    `json:"spd"`
	MAG       int    `json:"mag"`
	Element   string `json:"element"`
	Alive     bool   `json:"alive"`
	Defending bool   `json:"-"`
}

// EnemyPreset 敌人模板（野外遭遇/剧情战通用）。
type EnemyPreset struct {
	Key     string
	Name    string
	Element string
	HP      int
	ATK     int
	DEF     int
	SPD     int
	MAG     int
	Gold    int // 掉落金币
	Exp     int
	Loot    InvItem // 掉落的战利品（进背包）
}

// enemyPresets 内置敌人池：等级从弱到强，酒馆开荒够用。每只都有战利品，
// 打完进背包（真正的世界收集感，不是数字药店）。
var enemyPresets = map[string]EnemyPreset{
	"slime":    {Key: "slime", Name: "史莱姆", Element: "water", HP: 60, ATK: 8, DEF: 4, SPD: 5, MAG: 2, Gold: 20, Exp: 10, Loot: InvItem{Name: "史莱姆凝胶", Icon: "🟢", Desc: "软乎乎黏答答，炼金素材"}},
	"wolf":     {Key: "wolf", Name: "灰狼", Element: "wind", HP: 85, ATK: 12, DEF: 5, SPD: 12, MAG: 3, Gold: 35, Exp: 16, Loot: InvItem{Name: "狼牙", Icon: "🦷", Desc: "锐利的尖牙，可做箭簇"}},
	"goblin":   {Key: "goblin", Name: "哥布林", Element: "dark", HP: 70, ATK: 10, DEF: 6, SPD: 9, MAG: 4, Gold: 40, Exp: 18, Loot: InvItem{Name: "哥布林匕首", Icon: "🔪", Desc: "粗制滥造但能砍人"}},
	"skeleton": {Key: "skeleton", Name: "骷髅兵", Element: "dark", HP: 90, ATK: 13, DEF: 8, SPD: 6, MAG: 5, Gold: 55, Exp: 24, Loot: InvItem{Name: "陈旧骨片", Icon: "💀", Desc: "带着陌生年代的气息"}},
	"orc":      {Key: "orc", Name: "兽人", Element: "fire", HP: 140, ATK: 16, DEF: 10, SPD: 7, MAG: 6, Gold: 80, Exp: 35, Loot: InvItem{Name: "兽人战斧", Icon: "🪓", Desc: "沉重、粗糙、要命的斧头"}},
	"bandit":   {Key: "bandit", Name: "强盗头目", Element: "dark", HP: 120, ATK: 14, DEF: 8, SPD: 10, MAG: 7, Gold: 100, Exp: 40, Loot: InvItem{Name: "强盗的宝箱钥匙", Icon: "🗝️", Desc: "开某处财宝箱的钥匙"}},
	"dragon":   {Key: "dragon", Name: "幼龙", Element: "fire", HP: 200, ATK: 20, DEF: 12, SPD: 8, MAG: 15, Gold: 200, Exp: 80, Loot: InvItem{Name: "龙鳞", Icon: "🐉", Desc: "火烧不尽的一小块龙鳞"}},
}

// elementEmoji 敌方按元素给个占位图标（战斗场景头像位）。
func elementEmoji(el string) string {
	switch el {
	case "fire":
		return "🔥"
	case "water":
		return "💧"
	case "wind":
		return "🌪️"
	case "dark":
		return "🌑"
	case "light", "holy":
		return "✨"
	case "ice":
		return "❄️"
	case "thunder":
		return "⚡"
	default:
		return "👾"
	}
}

// BattleEvent 一回合的一条结算事件（前端逐条渲染：伤害数字/闪避/暴击…）。
type BattleEvent struct {
	Turn    int    `json:"turn"`
	Actor   string `json:"actor"`
	Action  string `json:"action"` // attack | defend | skill | hurt | miss | crit | shield
	Target  string `json:"target"`
	Amount  int    `json:"amount"`
	Message string `json:"message"`
}

// RPBattle 一场战斗的状态。
type RPBattle struct {
	mu        sync.Mutex
	ID        string        `json:"id"`
	SessionID string        `json:"sessionId"`
	Turn      int           `json:"turn"`
	Units     []BattleUnit  `json:"units"`
	Log       []BattleEvent `json:"log"`
	Over      bool          `json:"over"`
	Victory   bool          `json:"victory"`
	GameOver  bool          `json:"gameOver"` // 主角死亡：游戏结束
	GoldGain  int           `json:"goldGain"` // 掉落金币
	Loot      *InvItem      `json:"loot"`     // 战利品快照（胜利时 set，前端展示）
	CreatedAt int64         `json:"createdAt"`
}

// rpBattles 内存战斗表：本地单机，不落盘（打完即弃，结果写回角色卡）。
var rpBattles = struct {
	sync.Mutex
	m map[string]*RPBattle
}{m: map[string]*RPBattle{}}

// newRPBattle 开战：我方 cast 角色读人物参数，敌方按敌人模板生成。
func newRPBattle(sessionID string, agentIDs []string, enemyKey string) (*RPBattle, error) {
	ep, ok := enemyPresets[enemyKey]
	if !ok {
		ep = enemyPresets["slime"]
	}
	b := &RPBattle{
		ID:        fmt.Sprintf("battle_%d", time.Now().UnixNano()),
		SessionID: sessionID,
		Turn:      1,
		CreatedAt: time.Now().UnixMilli(),
	}
	// 我方：cast 角色，读角色卡 stats；没数值的角色给默认弱鸡面板。
	for _, id := range agentIDs {
		if id == "" {
			continue
		}
		u := BattleUnit{ID: id, Side: "ally", Alive: true}
		if c := GetAgentCard(id); c != nil {
			u.Name = c.Name
			u.Avatar = c.Avatar // 角色卡头像（base64 dataURL）带进战场
			s := c.Stats
			u.MaxHP = s.MaxHP
			if u.MaxHP <= 0 {
				u.MaxHP = 100
			}
			u.HP = s.HP
			if u.HP <= 0 {
				u.HP = u.MaxHP
			}
			u.Shield = s.Shield
			u.MP, u.MaxMP = s.MP, s.MaxMP
			u.ATK, u.DEF, u.SPD, u.MAG = s.ATK, s.DEF, s.SPD, s.MAG
			u.Element = s.Element
			if u.ATK <= 0 {
				u.ATK = 10
			}
			if u.DEF <= 0 {
				u.DEF = 5
			}
			if u.SPD <= 0 {
				u.SPD = 8
			}
		} else {
			u.Name = id
			u.MaxHP, u.HP = 100, 100
			u.ATK, u.DEF, u.SPD, u.MAG = 10, 5, 8, 3
		}
		b.Units = append(b.Units, u)
	}
	if len(b.Units) == 0 {
		return nil, fmt.Errorf("没有可上场的角色")
	}
	// 敌方
	b.Units = append(b.Units, BattleUnit{
		ID: "enemy:" + ep.Key, Name: ep.Name, Side: "enemy",
		Avatar: elementEmoji(ep.Element),
		MaxHP:  ep.HP, HP: ep.HP, ATK: ep.ATK, DEF: ep.DEF,
		SPD: ep.SPD, MAG: ep.MAG, Element: ep.Element, Alive: true,
	})
	rpBattles.Lock()
	rpBattles.m[b.ID] = b
	rpBattles.Unlock()
	return b, nil
}

func getRPBattle(id string) *RPBattle {
	rpBattles.Lock()
	defer rpBattles.Unlock()
	return rpBattles.m[id]
}

// unitByID 找战斗单位（调用方持锁）。
func (b *RPBattle) unitByID(id string) *BattleUnit {
	for i := range b.Units {
		if b.Units[i].ID == id {
			return &b.Units[i]
		}
	}
	return nil
}

// aliveAllies / aliveEnemies 存活计数。
func (b *RPBattle) aliveSides() (allies, enemies int) {
	for i := range b.Units {
		if !b.Units[i].Alive {
			continue
		}
		if b.Units[i].Side == "ally" {
			allies++
		} else {
			enemies++
		}
	}
	return
}

// sortSpeed 行动顺序：速度降序；同速我方先手（先手优势给玩家）。
func (b *RPBattle) sortSpeed(order []int) {
	sort.SliceStable(order, func(i, j int) bool {
		a, bb := b.Units[order[i]], b.Units[order[j]]
		if a.SPD != bb.SPD {
			return a.SPD > bb.SPD
		}
		return a.Side == "ally" && bb.Side == "enemy"
	})
}
