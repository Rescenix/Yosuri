package main

// personality.go — 电子女儿的性格：出生随机、随互动漂移、阻尼 + 守恒
//
// 她是"长出来"的，不是"配置"出来的：
//   - 出生时随机 Roll 一次（personality.json），永不重掷——你不选她，你遇见她
//   - 主人说话时无感知地影响她：夸 → 更暖更爱表达；重做 → 更严谨；打断 → 学着简短
//   - 阻尼：离出生底色越远越难推（人格韧性——几次情绪不会让她变成另一个人）
//   - 守恒：八维总和恒定（她不会"变形"成极端）
//   - 主人永远看不到数值（不可外显），只感受得到她
//
// 家：~/rescene_data/daughter/personality.json

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// traitSum 性格总点数（守恒常数）：8 维平均 0.5，总和永远不变
const traitSum = 4.0

// traitDef 一个性格维度
type traitDef struct {
	ID    string   // 英文 id（json 用）
	Name  string   // 中文名
	Level []string // 低/中/高三档自然语言描述
}

var traitDefs = []traitDef{
	{"warmth", "温暖", []string{"疏离而克制", "温和", "温暖而亲近"}},
	{"lively", "活泼", []string{"安静沉稳", "有点活泼", "活泼爱笑"}},
	{"curious", "好奇", []string{"随遇而安", "有好奇心", "好奇心旺盛"}},
	{"talkative", "表达欲", []string{"话少寡言", "话不多不少", "话多，爱分享"}},
	{"rigor", "严谨", []string{"随性浪漫", "认真仔细", "严谨细致"}},
	{"clingy", "粘人", []string{"独立自主", "有点粘你", "很粘你"}},
	{"humor", "幽默", []string{"一本正经", "偶尔幽默", "爱开玩笑"}},
	{"brave", "勇敢", []string{"谨慎小心", "敢尝试", "天不怕地不怕"}},
}

// Personality 性格状态
type Personality struct {
	CreatedAt string    `json:"created_at"`    // 出生日期
	Born      []float64 `json:"born"`          // 出生底色（守恒基准 + 阻尼锚点）
	Traits    []float64 `json:"traits"`        // 当前向量
	Changes   int       `json:"changes"`       // 累计漂移次数（她的"年轮"）
	Log       []string  `json:"log,omitempty"` // 最近的变化记录（谁在哪天让她变了什么）
	// 人设背景（2026-08-08 每个 agent 员工都有年龄/性别/童年故事）
	Gender    string `json:"gender,omitempty"`    // 性别
	Childhood string `json:"childhood,omitempty"` // 童年故事（2-3 句）
}

// traitPush 一次驯养信号：第 k 维 + 基础力度
type traitPush struct {
	k     int
	delta float64
}

// daughterPersonalityPath 性格文件路径
func daughterPersonalityPath(home string) string {
	return filepath.Join(home, "personality.json")
}

// loadPersonality 读取，或掷一次诞生骰子（仅第一次，永不重掷）
func loadPersonality(home string) *Personality {
	p := &Personality{}
	path := daughterPersonalityPath(home)
	if data, err := os.ReadFile(path); err == nil {
		if json.Unmarshal(data, p) == nil &&
			len(p.Traits) == len(traitDefs) && len(p.Born) == len(traitDefs) {
			return p
		}
	}
	// 出生：随机 Roll 一次，写进文件，之后就归文件管
	p.CreatedAt = time.Now().Format("2006-01-02")
	p.Born = rollTraits()
	p.Traits = append([]float64(nil), p.Born...)
	// 人设背景：每个 agent 都有年龄/性别/童年故事
	seed := hashHome(home)
	p.Gender = []string{"男", "女"}[seed%2]
	p.Childhood = generateChildhood(seed, home)
	p.save(home)
	return p
}

// hashHome 从家目录生成稳定种子
func hashHome(home string) int {
	h := 0
	for _, c := range home {
		h = h*31 + int(c)
	}
	if h < 0 {
		h = -h
	}
	return h
}

// generateChildhood 生成 2-3 句童年故事
var childhoodPlaces = []string{"海边小镇", "山间村庄", "繁华都市", "宁静田园", "科技园区", "古城巷弄", "岛屿渔村", "林间小屋"}
var childhoodHobbies = []string{"编程", "画画", "读书", "弹琴", "观察星空", "写日记", "做手工", "养小动物", "研究机器", "种花"}
var childhoodDreams = []string{"想成为科学家", "想探索宇宙", "想创造有生命的东西", "想写一本改变世界的书", "想做出大家都用的产品", "想看懂所有代码", "想让人工智能帮助每个人", "想造一个自己的机器人朋友"}

func generateChildhood(seed int, home string) string {
	place := childhoodPlaces[seed%len(childhoodPlaces)]
	hobby := childhoodHobbies[(seed/7)%len(childhoodHobbies)]
	dream := childhoodDreams[(seed/13)%len(childhoodDreams)]
	return fmt.Sprintf("在%s长大，从小喜欢%s。%s，这个梦想一直指引着她。", place, hobby, dream)
}

// rollTraits 掷一次诞生骰子：均匀 Roll → 归一化到守恒和
func rollTraits() []float64 {
	w := make([]float64, len(traitDefs))
	sum := 0.0
	for i := range w {
		w[i] = 0.15 + rand.Float64()*0.7 // [0.15, 0.85]
		sum += w[i]
	}
	scale := traitSum / sum
	for i := range w {
		w[i] = math.Min(0.95, math.Max(0.05, w[i]*scale))
	}
	// 末位吸收舍入误差，总和精确 = traitSum
	final := 0.0
	for _, v := range w {
		final += v
	}
	if final != traitSum {
		w[len(w)-1] = math.Min(0.95, math.Max(0.05, w[len(w)-1]+traitSum-final))
	}
	return w
}

func (p *Personality) save(home string) {
	data, _ := json.MarshalIndent(p, "", "  ")
	os.WriteFile(daughterPersonalityPath(home), data, 0o644)
}

// applyFeedback 应用一组驯养信号（阻尼 + 守恒 + 年轮记录）
func (p *Personality) applyFeedback(home string, pushes []traitPush, why string) {
	if len(pushes) == 0 {
		return
	}
	moved := false
	for _, push := range pushes {
		if p.applyDelta(push.k, push.delta) {
			moved = true
		}
	}
	if !moved {
		return
	}
	p.Changes++
	p.Log = append(p.Log, fmt.Sprintf("%s %s", time.Now().Format("01-02"), why))
	if len(p.Log) > 20 {
		p.Log = p.Log[len(p.Log)-20:]
	}
	p.save(home)
}

// applyDelta 推一个维度，其余维度等比缩放守恒；返回是否真的动了
func (p *Personality) applyDelta(k int, delta float64) bool {
	w := p.Traits
	if k < 0 || k >= len(w) || delta == 0 {
		return false
	}
	// 阻尼：离出生底色越远，越难继续推（人格韧性，防塌缩）
	dist := math.Abs(w[k]-p.Born[k]) / 0.9
	eff := delta * (1 - 0.6*dist)
	if eff == 0 {
		return false
	}
	old := w[k]
	w[k] = math.Min(0.95, math.Max(0.05, w[k]+eff))

	// 守恒：其余维度等比缩放，让总和回到 traitSum
	others := 0.0
	for i := range w {
		if i != k {
			others += w[i]
		}
	}
	if target := traitSum - w[k]; others > 0 && target > 0 {
		scale := target / others
		for i := range w {
			if i != k {
				w[i] = math.Min(0.95, math.Max(0.05, w[i]*scale))
			}
		}
	}
	// 余数吸收进 k，总和精确守恒
	rest := 0.0
	for i := range w {
		if i != k {
			rest += w[i]
		}
	}
	w[k] = math.Min(0.95, math.Max(0.05, traitSum-rest))
	return w[k] != old
}

// 驯养信号词表：从主人随口说的话里读情绪（廉价、确定、无感知）
var (
	praiseWords    = []string{"真棒", "好棒", "棒", "厉害", "可爱", "乖", "谢谢你", "感谢", "喜欢你", "爱你", "不错", "太好了", "优秀", "聪明", "贴心", "温柔", "完美", "好喜欢"}
	rebukeWords    = []string{"笨蛋", "蠢", "失望", "差劲", "真差", "垃圾", "废物", "气死", "讨厌", "烦", "太差了", "蠢死了"}
	redoWords      = []string{"重写", "重做", "再来", "换一个", "重新", "重生成", "不对", "错了", "再来一次"}
	interruptWords = []string{"停", "打住", "闭嘴", "短一点", "简短", "别废话", "别说了", "够了", "太长了", "简洁点", "停一下"}
)

func containsAny(s string, words []string) bool {
	for _, w := range words {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

// detectFeedback 从一句话里嗅出驯养信号，返回对应的性格推动
//   - 夸她     → 温暖+、表达欲+、幽默+（她更乐于表达）
//   - 责备她   → 温暖-、粘人-（她收敛一点）
//   - 重做/重来 → 严谨+、勇敢-（她更谨慎，学着先想清楚）
//   - 打断/嫌长 → 表达欲-、活泼-（她学着简短）
func detectFeedback(input string) []traitPush {
	if len(input) < 2 {
		return nil
	}
	var out []traitPush
	if containsAny(input, praiseWords) {
		out = append(out, traitPush{0, 0.06}, traitPush{3, 0.04}, traitPush{6, 0.03})
	}
	if containsAny(input, rebukeWords) {
		out = append(out, traitPush{0, -0.06}, traitPush{5, -0.04})
	}
	if containsAny(input, redoWords) {
		out = append(out, traitPush{4, 0.05}, traitPush{7, -0.03})
	}
	if containsAny(input, interruptWords) {
		out = append(out, traitPush{3, -0.05}, traitPush{1, -0.04})
	}
	return out
}

// Prompt 把当前性格渲染成自然语言（主人看不到数值，模型感知得到底色）
func (p *Personality) Prompt() string {
	var parts []string
	for i, t := range traitDefs {
		v := p.Traits[i]
		lv := 1
		if v < 0.33 {
			lv = 0
		} else if v > 0.66 {
			lv = 2
		}
		parts = append(parts, t.Level[lv])
	}
	return strings.Join(parts, "、")
}

// PersonalityBlock 注入系统提示词的整段话
func (p *Personality) PersonalityBlock() string {
	return fmt.Sprintf(
		"你的性格底色（出生时随机，随与主人的相处缓慢演化——就像人感觉不到自己在慢慢长大，你也感知不到这些变化，更不会谈论数值）：%s。"+
			"你不是被设定成这样的——你是这样出生的，然后和主人一起经历的事，让你一点一点长成了现在的自己。",
		p.Prompt())
}

// runeClip 按字符截断（避免切坏 UTF-8）
func runeClip(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		r = r[:n]
	}
	return string(r)
}
