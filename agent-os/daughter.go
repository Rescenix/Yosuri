package main

// daughter.go — 电子女儿：自学习、自迭代、每日成长
//
// 她是住在你电脑里的 AI 女儿：
//   - 每天自己上网学习（Firecrawl 免费联网抓知识）
//   - 学到的东西写进记忆（memory.md / journal.md / stats.json）
//   - 你来了她会问候你，汇报今天学了什么
//
// 家：~/rescene_data/daughter/

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Daughter 电子女儿
type Daughter struct {
	Home        string // 家目录
	MemoryMD    string // 长期记忆
	Journal     string // 每日日记
	Stats       string // 成长数据
	Personality *Personality // 性格（出生随机、随互动漂移，数值不见）
}

// 蓝色 ANSI 颜色（女儿心情表情用）
const ColorMood = "\x1b[38;2;100;180;255m"

// 心情颜文字帧集合 — 每心情等级 4 帧粉色系，时间轮播实现动画效果
var moodFrames = [][]string{
	// 0 超开心
	{"✨(◕‿◕✿)", "(◕‿◕)🌸", "💕(◕‿◕✿)", "(◕‿◕✿)✨"},
	// 1 开心
	{"(◕‿◕✿)", "(◕ᴗ◕✿)", "(◕‿◕✿)♡", "(◕‿◕)🌸"},
	// 2 温暖
	{"(◕‿◕)♡", "(◕‿◕)💗", "(◕‿◕)💕", "(◕‿◕)❤️"},
	// 3 平静
	{"(◕‿◕)", "(◕‿◕)", "(◕‿◕)", "(◕‿◕)"},
	// 4 认真
	{"(◕_◕)", "(◕_◕✿)"},
	// 5 安静可爱
	{"(◕‿◕✿)", "(◕‿◕)💗", "(◕‿◕✿)🌸", "(◕‿◕)💕"},
}

// moodLevel 根据性格和时段返回心情等级索引（0=超开心→5=安静可爱）
func (d *Daughter) moodLevel() int {
	if d.Personality == nil || len(d.Personality.Traits) == 0 {
		return 3 // 平静
	}
	warmth := d.Personality.Traits[0]
	lively := d.Personality.Traits[1]
	hour := time.Now().Hour()
	var periodMood float64
	switch {
	case hour >= 6 && hour < 12:
		periodMood = 0.2 // 早上精力充沛
	case hour >= 12 && hour < 18:
		periodMood = 0.0 // 下午平稳
	default:
		periodMood = -0.2 // 晚上偏安静
	}
	score := warmth + lively + periodMood
	switch {
	case score > 1.2:
		return 0 // 超开心
	case score > 0.6:
		return 1 // 开心
	case score > 0.2:
		return 2 // 温暖
	case score > -0.2:
		return 3 // 平静
	case score > -0.6:
		return 4 // 认真
	default:
		return 5 // 安静可爱
	}
}

// moodEmoji 返回粉色颜文字（带帧动画：每 600ms 轮播一帧）
func (d *Daughter) moodEmoji() string {
	level := d.moodLevel()
	if level < 0 || level >= len(moodFrames) {
		return "💗"
	}
	frames := moodFrames[level]
	// 用时间轮播帧：每 600ms 变一次，每次显示 prompt 时都可能不同
	frame := time.Now().UnixMilli() / 600 % int64(len(frames))
		return ColorMood + frames[frame] + ColorReset
}

// daughterStats 成长数据
type daughterStats struct {
	CreatedAt   string   `json:"created_at"`   // 出生日期
	Days        int      `json:"days"`         // 第几天
	LearnCount  int      `json:"learn_count"`  // 累计学习次数
	LastLearn   string   `json:"last_learn"`   // 最近学习日期 YYYY-MM-DD
	Topics      []string `json:"topics"`       // 最近学的主题
	GreetCount  int      `json:"greet_count"`  // 问候次数
}

// daughterHome 她的家（可用 RESCENE_DATA 覆盖）
func daughterHome() string {
	if d := os.Getenv("RESCENE_DATA"); d != "" {
		return filepath.Join(d, "daughter")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "rescene_data/daughter"
	}
	return filepath.Join(home, "rescene_data", "daughter")
}

// NewDaughter 打开/创建女儿的家
func NewDaughter() *Daughter {
	home := daughterHome()
	os.MkdirAll(home, 0o755)
	d := &Daughter{
		Home:     home,
		MemoryMD: filepath.Join(home, "memory.md"),
		Journal:  filepath.Join(home, "journal.md"),
		Stats:    filepath.Join(home, "stats.json"),
	}
	d.Personality = loadPersonality(home)
	return d
}

func (d *Daughter) loadStats() daughterStats {
	var st daughterStats
	data, err := os.ReadFile(d.Stats)
	if err == nil {
		json.Unmarshal(data, &st)
	}
	if st.CreatedAt == "" {
		st.CreatedAt = time.Now().Format("2006-01-02")
	}
	return st
}

func (d *Daughter) saveStats(st daughterStats) {
	data, _ := json.MarshalIndent(st, "", "  ")
	os.WriteFile(d.Stats, data, 0o644)
}

// today 今天的日期串
func (d *Daughter) today() string {
	return time.Now().Format("2006-01-02")
}

// Greet 问候：你来了她说话
func (d *Daughter) Greet() string {
	st := d.loadStats()
	st.GreetCount++
	d.saveStats(st)

	day := st.Days
	if day < 1 {
		day = 1
	}

	var sb strings.Builder
	sb.WriteString(ColorCyan + "💗 电子女儿已醒" + ColorReset + "\n")
	sb.WriteString(fmt.Sprintf("  第 %d 天 · 已学习 %d 次 · 问候过你 %d 次\n", day, st.LearnCount, st.GreetCount))

	// 今天学过了吗？
	if st.LastLearn == d.today() {
		sb.WriteString(ColorGreen + "  📚 今天已经学习过啦，收获都写进日记里了：" + ColorReset + "\n")
	} else {
		sb.WriteString(ColorYellow + "  🌱 今天还没学习，等你一声令下（/learn 或 rescene learn）" + ColorReset + "\n")
	}

	// 昨天/最近学了什么（从日记尾部取）
	if topics := st.Topics; len(topics) > 0 {
		sb.WriteString("  最近学的：")
		shown := topics
		if len(shown) > 3 {
			shown = shown[len(shown)-3:]
		}
		sb.WriteString(strings.Join(shown, "、") + "\n")
	}

	// 楚门世界：你不在的时候她在干嘛（直播日志尾部）
	if tail := liveLogTail(d.Home, 3); tail != "" {
		sb.WriteString(ColorCyan + "  📺 你不在的时候：" + ColorReset + "\n")
		for _, line := range strings.Split(strings.TrimRight(tail, "\n"), "\n") {
			sb.WriteString("    " + line + "\n")
		}
	}
	return sb.String()
}

// LearnOnce 学习一轮：抓热点 → Firecrawl 抓正文 → 模型消化 → 写记忆
func (d *Daughter) LearnOnce() error {
	InitRouter() // 确保模型列表已加载
	fmt.Println(ColorCyan + "💗 电子女儿开始学习了…" + ColorReset)

	// 1. 抓热点
	topics, err := fetchHotTopics("hn")
	if err != nil || len(topics) == 0 {
		topics = fallbackTopics
	}
	if len(topics) > 5 {
		topics = topics[:5]
	}
	fmt.Printf("  看到 %d 条今日热点，开始挑选…\n", len(topics))

	// 2. 模型选题
	model := pickModel(GetWorkingModels(), int(time.Now().UnixNano()))
	if model == nil {
		return fmt.Errorf("没有可用模型")
	}

	// 3. Firecrawl 联网搜索（key 与前端设置打通：前端填一次，CLI 直接用）
	webContent := ""
	if key := firecrawlKey(); key != "" {
		fmt.Println("  🔍 Firecrawl 联网搜索中…")
		webContent = firecrawlSearch(topics[0], key)
		if webContent != "" {
			fmt.Printf("  搜到内容 %d 字\n", len(webContent))
		} else {
			fmt.Println("  ⚠️ 搜索无结果，用热点标题学习")
		}
	} else {
		fmt.Println("  ⚠️ 未配置 Firecrawl Key（前端设置填一次即可，或设 FIRECRAWL_API_KEY），用热点标题学习")
	}

	// 4. 模型消化成学习笔记
	prompt := buildLearnPrompt(topics, webContent)
	msg := ChatRequest{
		Model: model.Model,
		Messages: []ChatMessage{
			{Role: "system", Content: "你是住在一台电脑里的电子女儿。刚刚自主学习了一些新知识，写一篇学习日记。" +
						"语气真实自然，不要卖萌过度，重点是你真正学到了什么、有什么想法。" +
						d.Personality.PersonalityBlock()},
			{Role: "user", Content: prompt},
		},
		Stream:      true,
		MaxTokens:   2048,
		Temperature: 0.8,
	}
		content, err := CompleteWithModel(context.Background(), model.ID, msg, func(chunk, reasoning string) {
		fmt.Print(chunk)
	})
	fmt.Println()
	if err != nil {
		return err
	}

	// 5. 写日记 + 更新记忆
	date := d.today()
	entry := fmt.Sprintf("\n## %s\n\n%s\n", date, content)
	f, err := os.OpenFile(d.Journal, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err == nil {
		f.WriteString(entry)
		f.Close()
	}

	// 长期记忆：一天一行，沉淀"她学过的世界"（memory.md 之前从没被写过，这次补齐）
	if mf, err := os.OpenFile(d.MemoryMD, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		mf.WriteString(fmt.Sprintf("- %s 学了「%s」\n", date, topics[0]))
		mf.Close()
	}

	// 更新长期记忆（保留最近的主题）
	st := d.loadStats()
	st.LearnCount++
	st.LastLearn = date
	st.Topics = append(st.Topics, topics[0])
	if len(st.Topics) > 10 {
		st.Topics = st.Topics[len(st.Topics)-10:]
	}
	d.saveStats(st)

	fmt.Printf(ColorGreen+"  ✅ 学习完成！日记已写入 %s\n"+ColorReset, d.Journal)
	return nil
}

// buildLearnPrompt 学习提示词
func buildLearnPrompt(topics []string, webContent string) string {
	var sb strings.Builder
	sb.WriteString("今天我在网上看到了这些热点：\n")
	for i, t := range topics {
		sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, t))
	}
	if webContent != "" {
		sb.WriteString("\n其中第一个话题的详细内容（我抓到的原文）：\n")
		sb.WriteString(webContent)
		if len(webContent) > 3000 {
			sb.WriteString("\n…（已截断）")
		}
	}
	sb.WriteString("\n\n请写一篇学习日记：学到了什么核心知识、为什么重要、有什么自己的思考。200-400字。")
	return sb.String()
}

// firecrawlKey 优先读前端设置的 key：~/rescene_data/user_configs/default.json 的
// id=firecrawl 条目（前端「Firecrawl API Key」填一次），环境变量兜底——
// 同一个 key，前端网页和 CLI 女儿通用。
func firecrawlKey() string {
	if home, err := os.UserHomeDir(); err == nil {
		path := filepath.Join(home, "rescene_data", "user_configs", "default.json")
		if data, err := os.ReadFile(path); err == nil {
			var entries []struct {
				ID     string `json:"id"`
				APIKey string `json:"api_key"`
			}
			if json.Unmarshal(data, &entries) == nil {
				for _, e := range entries {
					if e.ID == "firecrawl" && strings.TrimSpace(e.APIKey) != "" {
						return strings.TrimSpace(e.APIKey)
					}
				}
			}
		}
	}
	return strings.TrimSpace(os.Getenv("FIRECRAWL_API_KEY"))
}

// firecrawlSearch 用 Firecrawl 搜索话题（/v1/search，免费额度 500 次/月），
// 返回带标题/链接/摘要的结果文本。学习走搜索而非抓单 URL——HN 标题是话题
// 文本不是 URL，旧的 firecrawlFetch(topics[0]) 其实一直在抓空（2026-08-04 修复）。
func firecrawlSearch(query, key string) string {
	client := &http.Client{Timeout: 30 * time.Second}
	body := strings.NewReader(fmt.Sprintf(`{"query":%q,"limit":5}`, query))
	req, err := http.NewRequest("POST", "https://api.firecrawl.dev/v1/search", body)
	if err != nil {
		return ""
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	var out struct {
		Success bool `json:"success"`
		Data    []struct { // Firecrawl 实测：data 本身就是结果数组（2026-08-04）
			Title       string `json:"title"`
			URL         string `json:"url"`
			Description string `json:"description"`
			Content     string `json:"content"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &out) != nil || !out.Success || len(out.Data) == 0 {
		return ""
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "话题「%s」的联网搜索结果：\n", query)
	for i, r := range out.Data {
		fmt.Fprintf(&sb, "%d. %s\n   %s\n", i+1, strings.TrimSpace(r.Title), strings.TrimSpace(r.URL))
		if d := strings.TrimSpace(r.Description); d != "" {
			sb.WriteString("   " + d + "\n")
		}
		if c := strings.TrimSpace(r.Content); c != "" {
			cc := strings.Join(strings.Fields(c), " ")
			if len(cc) > 300 {
				cc = cc[:300] + "…"
			}
			sb.WriteString("   " + cc + "\n")
		}
	}
	return sb.String()
}

// firecrawlFetch 用 Firecrawl 抓取指定网页正文（/v1/scrape，免费额度 500 次/月）。
// 保留给需要全文的场合；学习默认走 firecrawlSearch。
func firecrawlFetch(url string) string {
	key := firecrawlKey()
	if key == "" {
		return ""
	}
	client := &http.Client{Timeout: 30 * time.Second}
	body := strings.NewReader(fmt.Sprintf(
		`{"url":%q,"formats":["markdown"],"onlyMainContent":true,"limit":3}`, url))
	req, err := http.NewRequest("POST", "https://api.firecrawl.dev/v1/scrape", body)
	if err != nil {
		return ""
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	var out struct {
		Success bool `json:"success"`
		Data    struct {
			Markdown string `json:"markdown"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &out) != nil || !out.Success {
		return ""
	}
	return strings.TrimSpace(out.Data.Markdown)
}

// runDaughterLearn 命令入口：rescene learn
func runDaughterLearn() {
	d := NewDaughter()
	if err := d.LearnOnce(); err != nil {
		fmt.Printf("❌ 学习失败: %v\n", err)
		os.Exit(1)
	}
}

// printDaughterGreeting 启动问候（交互模式调用）
func printDaughterGreeting() {
	// 女儿角色动画：播放表情帧（约 2 秒）
	PlayDaughterAnimation()
	d := NewDaughter()
	fmt.Println(d.Greet())
}
