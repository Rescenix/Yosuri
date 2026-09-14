package handler

// rp_tools.go —— 角色扮演模式下唯一开放的工具面：编排 Agent「Yosuri」的笔。
//
// RP 模式默认零工具（模型只演不碰系统），但纯演撑不起「参数会变」的世界：
// 受伤了 HP 谁扣？捡到剑谁记？好感涨了谁加？答案是——由导演来。
// 系统提示词里 Yosuri 是编排者，它通过 rp_set_stat 把人物参数写回角色卡，
// 下一轮注入的人设/状态段就是新的数值，模型演的时候自然续上。
//
// 允许的命令：
//   set      设置字段（数值/文本）：set(name, agent, field, value)
//   add      数值字段增减（扣血用负值）：add(name, agent, field, delta)
//   custom   设置自定义参数（灵力/san值/声望…）：custom(name, agent, key, value)
//   world    补一条世界书设定（Yosuri 现场编设定）：
//            world(name, title, content, keys)
//
// 参数全在 args 里带名字，方便模型记忆（旧版曾把名字藏进 JSON 键，模型拼错键就丢）。

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"backend/internal/ai/core"
	"backend/internal/memorydir"
)

// rpSetStatToolDef RP 编排工具的 schema。参数刻意扁平 + 语义化名字，
// 免费模型也能一次填对：模型只需记住「改参数用 rp_set_stat」这一个入口。
var rpSetStatToolDef = core.ToolDefinition{
	Type: "function",
	Function: core.ToolFunctionDetail{
		Name: "rp_set_stat",
		Description: "修改某张角色卡的人物参数（你是 Yosuri，本场戏的编排者）。" +
			"剧情里发生的事要落到数值上：受伤→hp 减、升级→level 加、买到装备→" +
			"equipment 追加、好感变化→affection 改。命令 set 设字段、add 增减数值、" +
			"custom 设自定义参数（灵力/san值/声望…）、world 补世界书设定。",
		Parameters: core.ToolParameters{
			Type: "object",
			Properties: map[string]core.ToolProperty{
				"command": {Type: "string", Description: "set | add | custom | world"},
				"agent":   {Type: "string", Description: "角色卡 id 或名字（如 yosuri / 猫娘）"},
				"field":   {Type: "string", Description: "字段名：level/hp/maxHp/atk/def/spd/mag/gold/affection/title/equipment"},
				"value":   {Type: "string", Description: "字段值：数值写数字，title 写称号文本，equipment 写装备名（追加）"},
				"key":     {Type: "string", Description: "custom 用：自定义参数名（灵力/san值/声望…）"},
				"title":   {Type: "string", Description: "world 用：世界书条目标题"},
				"content": {Type: "string", Description: "world 用：设定正文"},
				"keys":    {Type: "string", Description: "world 用：触发关键词，逗号分隔"},
			},
			Required: []string{"command", "agent"},
		},
	},
}

// rpBattleStartToolDef 触发战斗的编排工具：Yosuri 在剧情里遇到遭遇/Boss 时调用，
// 战场快照通过 SSE 弹到前端，本场戏从聊天变成「能开打的酒馆」。
var rpBattleStartToolDef = core.ToolDefinition{
	Type: "function",
	Function: core.ToolFunctionDetail{
		Name: "rp_battle_start",
		Description: "触发一场战斗（你是 Yosuri，本场戏的编排者）。剧情推进到遭遇战、伏击、" +
			"Boss 战、决斗时调用它：当前场上所有角色会带着各自的人物参数（HP/攻/防/速/元素/护盾）" +
			"进入战场，用户逐格操作，战斗结果自动写回角色卡。" +
			"敌人可选：slime(史莱姆)/wolf(灰狼)/goblin(哥布林)/skeleton(骷髅兵)/orc(兽人)/" +
			"bandit(强盗头目)/dragon(幼龙)。",
		Parameters: core.ToolParameters{
			Type: "object",
			Properties: map[string]core.ToolProperty{
				"enemy": {Type: "string", Description: "敌人 key（必填）：slime/wolf/goblin/skeleton/orc/bandit/dragon"},
			},
			Required: []string{"enemy"},
		},
	},
}

// rpToolDefs RP 模式开放的工具集：Yosuri 的笔（改参数）+ 指挥棒（开战）。
func rpToolDefs() []core.ToolDefinition {
	return []core.ToolDefinition{rpSetStatToolDef, rpBattleStartToolDef}
}

// callRPBattleStart 执行开战：读 ctx 里的 cast 建战斗，**只摆战场不结算**。
// 回合制由用户逐格操作：前端每个存活我方单位一行攻击/技能/防御按钮，
// 点完「结束回合」→ POST /api/rp/battle/turn 统一结算（resolveTurn）。
// 战斗结束状态写回角色卡，世界线继续自由发展。
func callRPBattleStart(ctx context.Context, argsJSON string) (nativeToolResult, error) {
	var a struct {
		Enemy string `json:"enemy"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return nativeToolResult{}, fmt.Errorf("参数解析失败：%v", err)
	}
	a.Enemy = strings.TrimSpace(a.Enemy)
	if a.Enemy == "" {
		return nativeToolResult{}, fmt.Errorf("需要 enemy 参数（slime/wolf/goblin/skeleton/orc/bandit/dragon）")
	}
	cast := rpCastFromCtx(ctx)
	if len(cast) == 0 {
		if id := agentIDFromCtx(ctx); id != "" {
			cast = []string{id}
		}
	}
	if len(cast) == 0 {
		return nativeToolResult{}, fmt.Errorf("没有可上场的角色")
	}
	b, err := newRPBattle("", cast, a.Enemy)
	if err != nil {
		return nativeToolResult{}, fmt.Errorf("开战失败：%v", err)
	}
	// 不结算：只发现身事件，战场已建好（含 battle.id），前端拿到 id 后
	// 逐回合调 /api/rp/battle/turn 驱动。
	ep := enemyPresets[a.Enemy]
	b.mu.Lock()
	all := []BattleEvent{{Turn: 0, Actor: ep.Name, Action: "appear", Message: "⚔️ " + ep.Name + " 现身了！"}}
	b.mu.Unlock()

	text := fmt.Sprintf("遭遇战开始：%s 拦住了去路，进入战斗！由用户逐格操作回合，结束后系统自动写回角色卡。", ep.Name)
	return nativeToolResult{Text: text, Battle: b, BattleEvents: all}, nil
}

// callRPSetStat 执行一次 RP 编排写操作。返回给模型的文本 + 是否改了卡。
func callRPSetStat(argsJSON string) (string, error) {
	var a struct {
		Command string `json:"command"`
		Agent   string `json:"agent"`
		Field   string `json:"field"`
		Value   string `json:"value"`
		Key     string `json:"key"`
		Title   string `json:"title"`
		Content string `json:"content"`
		Keys    string `json:"keys"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return "", fmt.Errorf("参数解析失败：%v", err)
	}
	a.Command = strings.ToLower(strings.TrimSpace(a.Command))
	a.Agent = strings.TrimSpace(a.Agent)
	if a.Command == "" || a.Agent == "" {
		return "", fmt.Errorf("需要 command 和 agent 参数")
	}

	switch a.Command {
	case "world":
		return rpWorldAppend(a.Title, a.Content, a.Keys)
	case "set", "add", "custom":
		card := resolveRPAgent(a.Agent)
		if card == nil {
			return "", fmt.Errorf("找不到角色卡「%s」（id 或名字都行，可先 /api/agents 看有哪些）", a.Agent)
		}
		return rpApplyStat(card, a.Command, a.Field, a.Value, a.Key)
	default:
		return "", fmt.Errorf("未知命令 %q（set/add/custom/world）", a.Command)
	}
}

// resolveRPAgent 按 id 或名字找角色卡。
func resolveRPAgent(name string) *AgentCard {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	if c := GetAgentCard(name); c != nil {
		return c
	}
	for _, c := range readAgentCards() {
		if strings.EqualFold(strings.TrimSpace(c.Name), name) {
			cc := c
			return &cc
		}
	}
	return nil
}

// rpApplyStat set/add/custom 的实际写卡逻辑。
func rpApplyStat(card *AgentCard, command, field, value, key string) (string, error) {
	field = strings.ToLower(strings.TrimSpace(field))
	value = strings.TrimSpace(value)
	st := card.Stats

	switch command {
	case "custom":
		key = strings.TrimSpace(key)
		if key == "" {
			return "", fmt.Errorf("custom 需要 key（参数名）")
		}
		st.Custom = upsertCustomStat(st.Custom, key, value)
	case "set", "add":
		if field == "" {
			return "", fmt.Errorf("set/add 需要 field（level/hp/atk/...）")
		}
		delta := command == "add"
		iv, isNum := parseRPNumeric(value)
		if isNum {
			if err := applyNumericStat(&st, field, iv, delta); err != nil {
				return "", err
			}
		} else if delta {
			return "", fmt.Errorf("add 只支持数值字段，%q 不是数字", value)
		} else {
			// 文本字段：title / equipment（追加）/ element / marks（印记）
			switch field {
			case "title":
				st.Title = value
			case "equipment":
				st.Equipment = appendUnique(st.Equipment, value)
			case "element":
				st.Element = strings.ToLower(value)
			case "marks":
				st.Marks = upsertCustomStat(st.Marks, key, value)
			default:
				return "", fmt.Errorf("字段 %q 需要数值，收到文本 %q", field, value)
			}
		}
	default:
		return "", fmt.Errorf("未知命令")
	}

	card.Stats = st
	saved, err := UpsertAgent(*card)
	if err != nil {
		return "", fmt.Errorf("写卡失败：%v", err)
	}
	return "已更新 " + saved.Name + "：" + formatAgentStats(saved.Name, saved.Stats), nil
}

// upsertCustomStat 同名覆盖，新名追加。
func upsertCustomStat(list []CustomStat, name, value string) []CustomStat {
	name = strings.TrimSpace(name)
	for i := range list {
		if list[i].Name == name {
			list[i].Value = value
			return list
		}
	}
	return append(list, CustomStat{Name: name, Value: value})
}

// parseRPNumeric 尽量把字符串转成数字；转不动返回 isNum=false。
func parseRPNumeric(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	if v, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return v, true
	}
	if f, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
		return int(f), true
	}
	return 0, false
}

// applyNumericStat 把数值写进对应字段。delta=true 时是增减（可负）。
func applyNumericStat(st *AgentStats, field string, v int, delta bool) error {
	set := func(cur *int) {
		if delta {
			*cur += v
		} else {
			*cur = v
		}
	}
	switch field {
	case "level":
		set(&st.Level)
	case "hp":
		set(&st.HP)
	case "maxhp":
		set(&st.MaxHP)
	case "mp":
		set(&st.MP)
	case "maxmp":
		set(&st.MaxMP)
	case "shield":
		set(&st.Shield)
	case "atk":
		set(&st.ATK)
	case "def":
		set(&st.DEF)
	case "spd":
		set(&st.SPD)
	case "mag":
		set(&st.MAG)
	case "gold":
		set(&st.Gold)
	case "affection":
		set(&st.Affection)
	default:
		return fmt.Errorf("未知数值字段 %q（level/hp/maxHp/mp/maxMp/shield/atk/def/spd/mag/gold/affection）", field)
	}
	// 血不能负数、不能超上限；盾不能负数；好感钳 0-100
	if st.HP < 0 {
		st.HP = 0
	}
	if st.MaxHP > 0 && st.HP > st.MaxHP {
		st.HP = st.MaxHP
	}
	if st.Shield < 0 {
		st.Shield = 0
	}
	if st.MP < 0 {
		st.MP = 0
	}
	if st.MaxMP > 0 && st.MP > st.MaxMP {
		st.MP = st.MaxMP
	}
	if st.Affection < 0 {
		st.Affection = 0
	}
	if st.Affection > 100 {
		st.Affection = 100
	}
	return nil
}

func appendUnique(list []string, s string) []string {
	for _, x := range list {
		if x == s {
			return list
		}
	}
	return append(list, s)
}

// rpWorldAppend 补一条世界书设定（Yosuri 现场编设定），写进主角色卡私有书。
func rpWorldAppend(title, content, keys string) (string, error) {
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	if title == "" || content == "" {
		return "", fmt.Errorf("world 需要 title 和 content")
	}
	// 取主角色（无 agent_id 时写全局书）；RP 工具经 ctx 带身份，但这里直接
	// 落全局也行——私有一致性由前端面板管理。先写全局，保证一定能读到。
	path := loreFilePath()
	lb := readLoreFile(path)
	if lb == nil {
		lb = &WorldBook{}
	}
	maxUID := 0
	for _, e := range lb.Entries {
		if e.UID > maxUID {
			maxUID = e.UID
		}
	}
	var keyList []string
	for _, k := range strings.Split(keys, ",") {
		if k = strings.TrimSpace(k); k != "" {
			keyList = append(keyList, k)
		}
	}
	lb.Entries = append(lb.Entries, LoreEntry{
		UID:     maxUID + 1,
		Name:    title,
		Keys:    keyList,
		Content: content,
		Order:   len(lb.Entries) + 1,
		Enabled: true,
	})
	if err := writeLoreFile(path, lb); err != nil {
		return "", fmt.Errorf("写世界书失败：%v", err)
	}
	return "已把「" + title + "」写进世界书" + phraseForKeys(keyList), nil
}

func phraseForKeys(keys []string) string {
	if len(keys) == 0 {
		return "（常驻，无条件注入）"
	}
	return "（触发词：" + strings.Join(keys, "、") + "）"
}

// rpToolNames 给 RP 硬闸放行用的名单。
func rpToolNames() map[string]bool {
	return map[string]bool{"rp_set_stat": true, "rp_battle_start": true}
}

// ensureAgentIDForRP 让 memorydir 的 Agent 路径函数可用（工具层目前不直接用，
// 保留这个守卫防止未来误把未清洗 id 传进文件路径）。
var _ = memorydir.SanitizeAgentID
