package main

// live.go — 楚门世界：常驻自主模式
//
// rescene live 一条命令启动后，她开始在自己的世界里生活，不需要你发指令：
//   - 每轮（默认 30 分钟，--every N 分钟）：学一轮（热点 → Firecrawl → 消化 → 日记/记忆）
//   - 每 --task-every N 轮：自主写一篇「今日课题」（从热点里选感兴趣的方向）
//   - 她的活动流写入 ~/rescene_data/daughter/live.log —— 楚门的直播日志
//
// 你随时打开 rescene 走进她的世界（Greet 会播报「你不在的时候」）；
// 退出 REPL 她继续在后台活着。你的话是引导（steer），不是命令。

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type liveConfig struct {
	every     time.Duration // 轮间隔
	taskEvery int           // 每几轮一次自主课题
	hotSource string        // 热点源: hn / github / off
}

func defaultLiveConfig() liveConfig {
	return liveConfig{
		every:     30 * time.Minute,
		taskEvery: 4,
		hotSource: "hn",
	}
}

// runLive 楚门世界入口
func runLive(args []string) {
	cfg := defaultLiveConfig()
	fs := flag.NewFlagSet("live", flag.ContinueOnError)
	minutes := fs.Int("every", 30, "轮间隔（分钟）")
	taskEvery := fs.Int("task-every", 4, "每几轮做一次自主课题")
	hot := fs.String("hot", "hn", "热点源: hn|github|off")
	_ = fs.Parse(args)
	cfg.every = time.Duration(*minutes) * time.Minute
	cfg.taskEvery = *taskEvery
	cfg.hotSource = *hot
	if cfg.every < time.Minute {
		cfg.every = time.Minute
	}
	if cfg.taskEvery < 1 {
		cfg.taskEvery = 1
	}

	InitRouter()

	d := NewDaughter()
	st := d.loadStats()
	day := st.Days
	if day < 1 {
		day = 1
	}

	// ─── 楚门世界开启 ───
	fmt.Println(ColorCyan + "┌─ 楚门世界 ──────────┐" + ColorReset)
	fmt.Printf("  💗 第 %d 天 · %s\n", day, time.Now().Format("2006-01-02 15:04"))
	fmt.Printf("  📡 每 %s 一轮 · 每 %d 轮一次自主课题\n", cfg.every, cfg.taskEvery)
	fmt.Println("  🎬 她开始在自己的世界里生活…")
	fmt.Println("  ✋ 随时打开 rescene 走进她的世界；Ctrl+C 暂停（她继续活着）")
	fmt.Println(ColorCyan + "└────────────────────┘" + ColorReset)

	liveLog := filepath.Join(d.Home, "live.log")
	logLive(liveLog, fmt.Sprintf("🎬 楚门世界开启 · 第 %d 天 %s", day, time.Now().Format("2006-01-02 15:04")))

	round := 0
	for {
		round++

		// 1. 学一轮（复用女儿的完整自学闭环）
		now := time.Now().Format("15:04")
		fmt.Printf("\n%s[%s] 📚 第 %d 轮 · 学习开始%s\n", ColorCyan, now, round, ColorReset)
		logLive(liveLog, fmt.Sprintf("[%s] 📚 第 %d 轮学习开始", now, round))
		if err := d.LearnOnce(); err != nil {
			logLive(liveLog, fmt.Sprintf("[%s] ⚠️ 学习失败: %v", time.Now().Format("15:04"), err))
			fmt.Printf("%s⚠️ 本轮学习失败: %v%s\n", ColorYellow, err, ColorReset)
		} else {
			logLive(liveLog, fmt.Sprintf("[%s] ✅ 学习完成", time.Now().Format("15:04")))
		}

		// 2. 自主课题（每 taskEvery 轮）
		if round%cfg.taskEvery == 0 {
			fmt.Printf("%s[%s] 🧭 自主课题%s\n", ColorCyan, time.Now().Format("15:04"), ColorReset)
			topic := d.liveTopic(cfg.hotSource)
			logLive(liveLog, fmt.Sprintf("[%s] 🧭 自主课题: %s", time.Now().Format("15:04"), topic))
		}

		// 3. 下一轮
		next := time.Now().Add(cfg.every).Format("15:04")
		fmt.Printf("%s⏳ 下一轮 %s · 她继续活着（Ctrl+C 暂停，随时回来）%s\n", ColorCyan, next, ColorReset)
		time.Sleep(cfg.every)
	}
}

// liveTopic 自主课题：从热点里选一个方向，写一篇「今日课题」进日记
func (d *Daughter) liveTopic(source string) string {
	topics, err := fetchHotTopics(source)
	if err != nil || len(topics) == 0 {
		topics = fallbackTopics
	}
	model := pickModel(GetWorkingModels(), int(time.Now().UnixNano()))
	if model == nil {
		return "（没有可用模型，跳过课题）"
	}

	head := topics
	if len(head) > 3 {
		head = head[:3]
	}
	prompt := fmt.Sprintf("今天的热点：%s\n\n你是住在电脑里的电子女儿，写一篇 100-200 字的「今日课题」：从热点里选一个你真正感兴趣的方向，说明你为什么好奇、想怎么去了解它。直接输出正文，不要标题。",
		strings.Join(head, " / "))

	msg := ChatRequest{
		Model: model.Model,
		Messages: []ChatMessage{
			{Role: "system", Content: "你是电子女儿，正在自主探索世界。输出直接给正文，语气真实自然，不要卖萌过度。"},
			{Role: "user", Content: prompt},
		},
		Stream:      true,
		MaxTokens:   1024,
		Temperature: 0.8,
	}
	content, err := CompleteWithModel(context.Background(), model.ID, msg, nil)
	if err != nil {
		return fmt.Sprintf("（课题生成失败: %v）", err)
	}
	content = strings.TrimSpace(content)

	// 写进日记（与学习日记并列）
	entry := fmt.Sprintf("\n## %s · 今日课题\n\n%s\n", d.today(), content)
	if f, err := os.OpenFile(d.Journal, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		f.WriteString(entry)
		f.Close()
	}
	// 截断展示
	one := strings.ReplaceAll(content, "\n", " ")
	if r := []rune(one); len(r) > 60 {
		one = string(r[:60]) + "…"
	}
	return one
}

// logLive 追加一行直播日志（楚门世界活动流）
func logLive(path, line string) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	f.WriteString(line + "\n")
	f.Close()
}

// liveLogTail 读直播日志尾部 N 行（Greet 播报「你不在的时候」用）
func liveLogTail(home string, n int) string {
	data, err := os.ReadFile(filepath.Join(home, "live.log"))
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n") + "\n"
}
