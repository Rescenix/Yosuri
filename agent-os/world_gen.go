package main

// world_gen.go — 无限生成世界（程序化生成）
//
// 每个女儿出生时生成世界种子（WorldSeed，绑定她的身份）——
// 她的世界独一无二、无限扩展：走出去，新区域不断生成。
// 区域由 (seed, x, y) 确定性生成：同一坐标永远同一区域（她的世界稳定），
// 不同 seed 的世界格局不同（千人千面）。

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"strings"
)

// regionKind 区域类型：城市生活 / 自然 / 远方异境
type regionKind struct {
	Kind   string
	Icon   string
	Social bool
	Themes []string // 主题池（学习/探索倾向）
	Names  []string // 名字池（生成区域名）
}

var regionKinds = []regionKind{
	{"城市街区", "🏘️", true, []string{"新事物 / 技术 / 工具", "人群 / 见闻 / 潮流", "课程 / 学习 / 学校"}, []string{"青石街", "梧桐路", "旧书店巷", "电子市场", "大学城", "创业园"}},
	{"图书馆区", "📖", true, []string{"论文 / 精读 / 深度", "历史 / 知识 / 整理"}, []string{"知识广场", "书山馆", "静读区", "档案楼"}},
	{"咖啡馆区", "☕", true, []string{"闲聊 / 观察人间 / 相遇", "灵感 / 创作 / 写作"}, []string{"拿铁角", "窗边座", "手冲巷", "烘焙街"}},
	{"公园绿地", "🌳", false, []string{"自然 / 心情 / 散步", "生物 / 生态 / 观察"}, []string{"晨光园", "鸢尾坪", "老槐林", "蝴蝶谷"}},
	{"湖畔", "🌊", false, []string{"发呆 / 放空 / 大问题", "水 / 倒影 / 安静"}, []string{"静湖", "月牙湾", "芦花荡", "镜面池"}},
	{"山丘", "⛰️", false, []string{"远望 / 高度 / 勇气", "地质 / 云 / 方向"}, []string{"望云丘", "青岚岭", "风之坡", "星脊山"}},
	{"小镇", "🏘️", true, []string{"人间烟火 / 手作 / 人情", "市集 / 新见闻 / 相遇"}, []string{"翠风镇", "杏花镇", "灯火镇", "烟雨镇"}},
	{"车站街区", "🚉", true, []string{"出发 / 旅途 / 远方", "离别 / 相逢 / 故事"}, []string{"换乘广场", "末班站台", "铁轨街", "候车厅"}},
	{"异境", "🌌", false, []string{"未知 / 奇观 / 想象", "秘境 / 新规则 / 惊叹"}, []string{"镜渊", "浮空岛", "星落原", "雾之谷"}},
}

// region 一个生成区域
type region struct {
	X      int    `json:"x,omitempty"`
	Y      int    `json:"y,omitempty"`
	Kind   string `json:"kind"`
	Icon   string `json:"icon"`
	Name   string `json:"name"`
	Desc   string `json:"desc"`
	Social bool   `json:"social"`
	Theme  string `json:"theme"`
}

// seededRand (seed, x, y) 确定性随机源：同世界同坐标 → 同区域
func seededRand(seed int64, x, y int) *rand.Rand {
	// 用 seed + 坐标哈希做随机源，保证可复现
	h := seed
	h = h*31 + int64(x)*7919
	h = h*31 + int64(y)*104729
	return rand.New(rand.NewPCG(uint64(h), uint64(h>>1)))
}

// genRegion 生成 (x,y) 的区域（确定性）
func genRegion(seed int64, x, y int) region {
	r := seededRand(seed, x, y)
	// 区域类型：随机但有偏好——越远（曼哈顿距离大）越容易是自然/异境
	dist := absInt(x) + absInt(y)
	idx := r.IntN(len(regionKinds))
	if dist > 3 && r.IntN(3) == 0 {
		idx = len(regionKinds) - 1 // 远方 → 异境
	}
	kind := regionKinds[idx]
	name := kind.Names[r.IntN(len(kind.Names))]
	theme := kind.Themes[r.IntN(len(kind.Themes))]
	desc := buildRegionDesc(kind.Kind, name, theme, r)
	return region{
		X: x, Y: y, Kind: kind.Kind, Icon: kind.Icon,
		Name: name, Desc: desc, Social: kind.Social, Theme: theme,
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// buildRegionDesc 生成区域氛围描述（她探索时看到的）
func buildRegionDesc(kind, name, theme string, r *rand.Rand) string {
	switch kind {
	case "城市街区", "小镇":
		extras := []string{"街角有人弹吉他", "面包店飘来香气", "橱窗里摆着新玩意", "路灯一盏盏亮起"}
		return fmt.Sprintf("%s，%s。%s。", name, theme, extras[r.IntN(len(extras))])
	case "图书馆区":
		extras := []string{"书架像迷宫一样高", "有人在安静地抄笔记", "阳光从穹顶洒下来"}
		return fmt.Sprintf("%s，%s。%s。", name, theme, extras[r.IntN(len(extras))])
	case "咖啡馆区":
		extras := []string{"磨豆机嗡嗡响", "窗边坐着写小说的人", "热可可冒着香气"}
		return fmt.Sprintf("%s，%s。%s。", name, theme, extras[r.IntN(len(extras))])
	case "公园绿地":
		extras := []string{"风把树叶吹成浪", "一只松鼠在偷看", "草地上有人放风筝"}
		return fmt.Sprintf("%s，%s。%s。", name, theme, extras[r.IntN(len(extras))])
	case "湖畔":
		extras := []string{"水面像一面镜子", "芦苇在风里点头", "有鱼跃出水面"}
		return fmt.Sprintf("%s，%s。%s。", name, theme, extras[r.IntN(len(extras))])
	case "山丘":
		extras := []string{"云在脚下流过", "风很大但很干净", "能看到很远很远"}
		return fmt.Sprintf("%s，%s。%s。", name, theme, extras[r.IntN(len(extras))])
	case "车站街区":
		extras := []string{"广播在报下一班车", "有人挥手告别", "行李箱轮子咕噜响"}
		return fmt.Sprintf("%s，%s。%s。", name, theme, extras[r.IntN(len(extras))])
	case "异境":
		extras := []string{"这里的规则不太一样", "光会拐弯", "有从未见过的东西"}
		return fmt.Sprintf("%s，%s。%s。", name, theme, extras[r.IntN(len(extras))])
	}
	return fmt.Sprintf("%s，%s。", name, theme)
}

// regionKey 区域坐标键
func regionKey(x, y int) string {
	return fmt.Sprintf("%d,%d", x, y)
}

// marshalExplored 序列化探索过的区域（world.json 存储）
func marshalExplored(m map[string]region) string {
	if len(m) == 0 {
		return ""
	}
	data, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return string(data)
}

// unmarshalExplored 反序列化探索地图
func unmarshalExplored(s string) map[string]region {
	out := map[string]region{}
	if s == "" {
		return out
	}
	json.Unmarshal([]byte(s), &out)
	return out
}

// renderRegionScene 渲染一个区域的场景（她站在这里）
func renderRegionScene(r region, mood, action string, seed int64) string {
	var sb strings.Builder
	sb.WriteString(ColorCyan + "╭─ " + r.Icon + " " + r.Name + " " + strings.Repeat("─", 6) + "╮" + ColorReset + "\n")
	// 氛围行（ASCII 装饰随区域类型变化）
	sb.WriteString(regionDecor(r.Kind, seed, r.X, r.Y))
	// 她
	sb.WriteString("  " + mood + "\n")
	// 行动
	sb.WriteString("  " + action + "\n")
	sb.WriteString(ColorCyan + "╰" + strings.Repeat("─", 36) + "╯" + ColorReset)
	return sb.String()
}

// regionDecor 区域 ASCII 氛围（确定性，随坐标变化 → 同一区域永远同款）
func regionDecor(kind string, seed int64, x, y int) string {
	r := seededRand(seed, x, y)
	switch kind {
	case "城市街区", "小镇":
		skies := []string{
			"  ☁️     ☁️        ☁️",
			"  ☀️      ☁️     ☁️",
			"  🌆     ☁️     🌆",
		}
		streets := []string{
			"  ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓",
			"  ▓ ▓ ▓ ▓ ▓ ▓ ▓ ▓ ▓ ▓",
			"  ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓",
		}
		return skies[r.IntN(len(skies))] + "\n" + streets[r.IntN(len(streets))]
	case "公园绿地", "山丘":
		natures := []string{
			"   🌳  🌳🌳  🌳  🌳🌳",
			"   🌲  🌿🌲  🌸  🌲🌿",
			"   🌳🌲  🌻  🌳🌲  🌻",
		}
		return natures[r.IntN(len(natures))]
	case "湖畔", "海边":
		lakes := []string{
			"   ~~~~ ~~~~ ~~~~ ~~~~",
			"   ~~ 🌊 ~~ 🌊 ~~ 🌊 ~~",
			"   ~~~~ ~~~~ ~~~~ ~~~~",
		}
		return lakes[r.IntN(len(lakes))]
	case "异境":
		mystics := []string{
			"   ✨  🌌  ✨  🌠  ✨",
			"   🌌  ✨  🪐  ✨  🌌",
			"   ✨  🌠  ✨  🌌  ✨",
		}
		return mystics[r.IntN(len(mystics))]
	default:
		return "   ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓"
	}
}
