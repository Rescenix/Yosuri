package main

// reply.go — 自动回复引擎（她替你回复评论/私信）
//
// 平台消息 → 意图分类 → AI 按你的口吻生成回复 → 安全闸 → 频率控制 → 发出。
// 目标是：你不用为了增加互动而亲自回复——她以你的口吻维持评论区热度。
//
// 人设（persona）：~/rescene_data/reply_persona.md（可编辑，默认亲切自然有温度）。

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// replyIntent 评论意图分类
type replyIntent int

const (
	intentQuestion  replyIntent = iota // 提问：认真回答
	intentPraise                      // 夸赞：感谢 + 互动
	intentCriticism                   // 吐槽/批评：回应 + 化解
	intentTroll                       // 引战/广告/垃圾：跳过
	intentGeneral                     // 一般：自然互动
)

func (i replyIntent) String() string {
	switch i {
	case intentQuestion:
		return "提问"
	case intentPraise:
		return "夸赞"
	case intentCriticism:
		return "批评"
	case intentTroll:
		return "垃圾"
	default:
		return "一般"
	}
}

// replyEvent 一条待回复的平台消息
type replyEvent struct {
	Platform  string // 平台（bilibili / csdn / …）
	ID        string // 平台消息 ID
	User      string // 评论者昵称
	Text      string // 评论/私信内容
	Target    string // 被评论的内容（视频/文章标题）
	Time      string // 时间
	ArticleID int    `json:"-"` // 平台内容 ID（CSDN 文章 ID / B站 oid）
}

// replyEngine 回复引擎状态
type replyEngine struct {
	Persona    string                // 人设（口吻描述）
	LastReply  map[string]time.Time  // 每平台上次回复时间（频率）
	DailyCount map[string]int        // 每平台当日已回复数
	Today      string                // 当天日期（跨日重置计数）
}

// maxReplyPerDay 每平台每日回复上限（防风控）
const maxReplyPerDay = 30

// minReplyInterval 每平台回复最小间隔
const minReplyInterval = 5 * time.Minute

// personaPath 人设文件
func personaPath() string {
	home := daughterHome()
	return filepath.Join(filepath.Dir(home), "reply_persona.md")
}

// defaultPersona 默认口吻（用户可编辑 persona 文件覆盖）
const defaultPersona = `你是这篇文章/视频的作者，回复网友的评论。你的口吻：
- 亲切自然，像朋友聊天，不说"感谢您的支持"这种套话
- 偶尔有梗、有温度，评论者会觉得你在认真看他的留言
- 被夸会开心但不油腻；被批评先认错再解释，不嘴硬
- 回答技术问题专业但通俗，不堆术语
- 每条 20-80 字，像真人打字，不像客服`

// loadPersona 加载人设（文件不存在用默认）
func loadPersona() string {
	data, err := os.ReadFile(personaPath())
	if err != nil || strings.TrimSpace(string(data)) == "" {
		return defaultPersona
	}
	return strings.TrimSpace(string(data))
}

// newReplyEngine 初始化引擎
func newReplyEngine() *replyEngine {
	return &replyEngine{
		Persona:    loadPersona(),
		LastReply:  map[string]time.Time{},
		DailyCount: map[string]int{},
		Today:      time.Now().Format("2006-01-02"),
	}
}

// classifyIntent 意图分类（规则优先，廉价确定；复杂交给生成时模型理解）
func (e *replyEngine) classifyIntent(text string) replyIntent {
	t := strings.TrimSpace(text)
	// 广告/引战/垃圾
	trollWords := []string{"加微信", "加v", "代做", "刷粉", "推广", "广告", "私聊我", "https://", "http://", "vx:", "q群", "tg:", "电报"}
	for _, w := range trollWords {
		if strings.Contains(t, w) {
			return intentTroll
		}
	}
	// 提问
	questionWords := []string{"怎么", "如何", "为什么", "吗？", "吗?", "请教", "求教", "问一下", "请问", "？", "?", "能不能", "可以吗", "求个", "教程"}
	for _, w := range questionWords {
		if strings.Contains(t, w) {
			return intentQuestion
		}
	}
	// 批评
	criticismWords := []string{"垃圾", "骗", "坑", "失望", "差劲", "水", "抄袭", "标题党", "不行", "无语", "什么玩意"}
	for _, w := range criticismWords {
		if strings.Contains(t, w) {
			return intentCriticism
		}
	}
	// 夸赞
	praiseWords := []string{"厉害", "好棒", "牛", "喜欢", "谢谢", "感谢", "学到了", "有用", "收藏", "支持", "优秀", "赞", "棒", "爱了", "良心"}
	for _, w := range praiseWords {
		if strings.Contains(t, w) {
			return intentPraise
		}
	}
	return intentGeneral
}

// generateReply AI 生成回复（按人设口吻）
func (e *replyEngine) generateReply(ev replyEvent, intent replyIntent) (string, error) {
	intentHint := ""
	switch intent {
	case intentQuestion:
		intentHint = "评论者在提问，认真回答他的问题；不确定就说你的真实看法。"
	case intentPraise:
		intentHint = "评论者在夸你，真诚感谢并回赠一点互动（比如反问他的兴趣/补充一句小细节）。"
	case intentCriticism:
		intentHint = "评论者在批评，先接住情绪（不反驳），简要解释或认错，态度真诚。"
	case intentTroll:
		return "", fmt.Errorf("垃圾评论，跳过")
	default:
		intentHint = "自然互动，像朋友闲聊。"
	}

	prompt := fmt.Sprintf(`%s

现在回复一条评论：
评论者：%s
评论：%s
（这条评论是关于你的作品《%s》）

意图：%s
%s

直接输出你的回复，不要任何解释。`, e.Persona, ev.User, ev.Text, ev.Target, intent.String(), intentHint)

	msg := ChatRequest{
		Model:       currentModel,
		Messages:    []ChatMessage{{Role: "user", Content: prompt}},
		Stream:      false,
		MaxTokens:   512,
		Temperature: 0.8,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	content, err := CompleteWithModel(ctx, msg.Model, msg, nil)
	if err != nil {
		return "", err
	}
	reply := strings.TrimSpace(content)
	// 去掉可能的引号包裹
	reply = strings.Trim(reply, "\"'“”「」")
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return "", fmt.Errorf("生成空回复")
	}
	return reply, nil
}

// safetyCheck 安全闸：回复不含敏感内容才放行
func (e *replyEngine) safetyCheck(reply string) bool {
	// 敏感词：政治/人身攻击/承诺/联系方式/链接
	bad := []string{"fuck", "傻逼", "你妈", "妈的", "去你", "他妈", "cnm", "草泥马", "http://", "https://", "加微信", "加我微信", "加我v", "vx", "电话", "qq号", "qq群", "保证", "绝对", "最牛逼", "天下第一"}
	for _, w := range bad {
		if strings.Contains(strings.ToLower(reply), strings.ToLower(w)) {
			return false
		}
	}
	// 长度
	r := []rune(reply)
	if len(r) < 2 || len(r) > 200 {
		return false
	}
	return true
}

// rateLimit 频率控制：每平台间隔 + 每日上限
func (e *replyEngine) rateLimit(platform string) bool {
	// 跨日重置
	if e.Today != time.Now().Format("2006-01-02") {
		e.Today = time.Now().Format("2006-01-02")
		e.DailyCount = map[string]int{}
	}
	if e.DailyCount[platform] >= maxReplyPerDay {
		return false
	}
	if last, ok := e.LastReply[platform]; ok && time.Since(last) < minReplyInterval {
		return false
	}
	return true
}

// markReplied 记录一次回复（频率统计）
func (e *replyEngine) markReplied(platform string) {
	e.LastReply[platform] = time.Now()
	e.DailyCount[platform]++
}

// runReplyCycle 跑一轮回复：拉取 → 分类 → 生成 → 审核 → 发出
// 返回处理统计。回复失败/跳过都记录，不中断。
func (e *replyEngine) runReplyCycle(adapters []platformAdapter) map[string]int {
	stats := map[string]int{}
	for _, ad := range adapters {
		platform := ad.Name()
		// 频率控制
		if !e.rateLimit(platform) {
			stats[platform+"_skipped_rate"] = 1
			continue
		}
		events, err := ad.fetchNewComments()
		if err != nil {
			stats[platform+"_fetch_error"] = 1
			continue
		}
		replied := 0
		for _, ev := range events {
			intent := e.classifyIntent(ev.Text)
			if intent == intentTroll {
				stats[platform+"_skipped_troll"]++
				continue
			}
			reply, err := e.generateReply(ev, intent)
			if err != nil {
				stats[platform+"_gen_error"]++
				continue
			}
			if !e.safetyCheck(reply) {
				stats[platform+"_blocked_safety"]++
				continue
			}
			if err := ad.postReply(ev, reply); err != nil {
				stats[platform+"_post_error"]++
				continue
			}
			ad.markHandled(ev)
			e.markReplied(platform)
			replied++
			// 间隔控制：一条接一条发太快 → 每平台同轮最多 5 条
			if replied >= 5 {
				break
			}
		}
		stats[platform+"_replied"] = replied
	}
	return stats
}

// platformAdapter 平台适配器接口（核心 + 适配器架构）
type platformAdapter interface {
	Name() string
	fetchNewComments() ([]replyEvent, error)   // 拉新评论
	postReply(ev replyEvent, reply string) error // 发回复
	markHandled(ev replyEvent)                   // 标记已处理（本地去重）
}

// runReply 跑一轮自动回复（她替你回复评论）
func runReply(args []string) {
	InitRouter()
	// 选默认模型（与 REPL 相同策略）
	defaultModel := "free_zen_deepseek_v4_flash"
	if available := GetWorkingModels(); len(available) > 0 {
		for _, m := range available {
			if m.ID == "free_zen_deepseek_v4_flash" {
				defaultModel = m.ID
				break
			}
			if m.ID == "free_zen_north_mini_code" {
				defaultModel = m.ID
				break
			}
		}
	}
	currentModel = defaultModel

	eng := newReplyEngine()
	adapters := []platformAdapter{newCSDNAdapter()}
	fmt.Println("🤖 Rescene 回复引擎启动（口吻：" + strings.TrimSpace(strings.SplitN(eng.Persona, "\n", 2)[0]) + "）")
	stats := eng.runReplyCycle(adapters)
	fmt.Println(formatReplyStats(stats))
}

// formatReplyStats 格式化一轮统计（终端/日志输出）
func formatReplyStats(stats map[string]int) string {
	var sb strings.Builder
	sb.WriteString("🤖 本轮回复战报：\n")
	for k, v := range stats {
		if v > 0 {
			sb.WriteString(fmt.Sprintf("  %s: %d\n", k, v))
		}
	}
	return sb.String()
}
