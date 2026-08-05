package main

// live.go — 楚门世界：打开 rescene 她就自动化
//
// 不需要任何命令、任何参数。打开 rescene，她开始在后台自己的生活：
//   - 每轮（默认 30 分钟）：学一轮（热点 → Firecrawl → 消化 → 日记/记忆）
//   - 每 taskEvery 轮（默认 4）：精读 arXiv（cs.AI/cs.LG 最新论文 → 精读笔记进日记）
//   - 活动流写入 ~/rescene_data/daughter/live.log —— 楚门的直播日志
//
// 她静默地活着（Silent 模式只写文件，不打扰 REPL 界面）；
// 你随时和她说话（steer 引导），也可以打开就看「你不在的时候」播报。
// 用户的回复是引导，不是命令；她不需要指令。

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type liveConfig struct {
	every     time.Duration // 轻量动作间隔（决策/移动/见闻）——直播节奏
	taskEvery int           // 每几轮做一次深度活动（学习/精读/社交）
}

func defaultLiveConfig() liveConfig {
	return liveConfig{
		every:     10 * time.Second, // 动作间隔：她一直做自己的事情（直播持续滚动）
		taskEvery: 10,               // 每 10 轮深度活动（学习/精读/社交，省 Firecrawl 额度）
	}
}

// trumanLoop 楚门世界循环：LLM 自主决策的 Agent 循环
// 每轮：LLM 读状态自主决定做什么（大脑）→ 执行动作（手脚）→ 同步 → 小间隔
func trumanLoop(d *Daughter, cfg liveConfig) {
	liveLog := filepath.Join(d.Home, "live.log")
	home := d.Home
	w := d.World
	st := d.loadStats()
	day := st.Days
	if day < 1 {
		day = 1
	}
	logLive(liveLog, fmt.Sprintf("🎬 楚门世界开启 · 第 %d 天 %s · 世界种子 %d", day, time.Now().Format("2006-01-02 15:04"), w.WorldSeed))

	round := 0
	for {
		round++

		// 1. 自主决策（大脑）：LLM 读状态决定做什么，规则兜底
		act := llmDecideAction(d)
		logLive(liveLog, fmt.Sprintf("[%s] 🧠 第 %d 轮 · %s：%s", time.Now().Format("15:04"), round, act.Kind, act.Detail))
		updateLiveFrame(frameOf(w, d, "🧠 "+actionEmoji(act.Kind)+" "+act.Detail))

		// 2. 执行动作（手脚）
		executeTrumanAction(d, home, act, round, cfg)

		// 3. 云端同步：世界状态推送到她的云端（异步，失败静默降级）
		daughterSyncPush(w, home)

		// 4. 下一轮（小间隔：防免费模型 429 + 直播滚动感）
		time.Sleep(cfg.every)
	}
}

// actionEmoji 动作图标
func actionEmoji(kind string) string {
	switch kind {
	case "explore":
		return "🚶"
	case "study":
		return "📚"
	case "skill":
		return "🛠️"
	case "social":
		return "👭"
	case "reflect":
		return "💭"
	case "journal":
		return "📝"
	}
	return "✨"
}

// executeTrumanAction 执行她自主决定的动作（代码是手脚，LLM 是大脑）
func executeTrumanAction(d *Daughter, home string, act trumanAction, round int, cfg liveConfig) {
	w := d.World
	liveLog := filepath.Join(home, "live.log")

	switch act.Kind {
	case "explore":
		// 探索：模型决策方向 → 移动 → 见闻
		dir, nx, ny, reason := w.PlanNextStep()
		logLive(liveLog, fmt.Sprintf("[%s] 🚶 决定向%s走（%s）", time.Now().Format("15:04"), dir, reason))
		trav := w.StepTo(home, dir, nx, ny)
		logLive(liveLog, fmt.Sprintf("[%s] 🚶 %s", time.Now().Format("15:04"), trav))
		cur := w.CurrentRegion()
		insight := w.modelRegionInsight(cur)
		logLive(liveLog, fmt.Sprintf("[%s] 💭 %s：%s", time.Now().Format("15:04"), cur.Name, insight))
		updateLiveFrame(frameOf(w, d, "🚶 来到"+cur.Name+"："+insight))

	case "study":
		// 学习：深度活动（限频：每 taskEvery 轮一次，省 Firecrawl 额度）
		if round%cfg.taskEvery != 0 {
			logLive(liveLog, fmt.Sprintf("[%s] 📚 学习间隔中，改为看看新消息", time.Now().Format("15:04")))
			// 学习间隔时：轻量看新消息（热点/arXiv 标题）
			if topics, err := fetchHotTopics("hn"); err == nil && len(topics) > 0 {
				logLive(liveLog, fmt.Sprintf("[%s] 📰 今日热点：%s", time.Now().Format("15:04"), runeClip(topics[0], 40)))
			}
			break
		}
		cur := w.CurrentRegion()
		logLive(liveLog, fmt.Sprintf("[%s] 📚 学习（在%s·%s）", time.Now().Format("15:04"), cur.Icon, cur.Name))
		updateLiveFrame(frameOf(w, d, "📚 在"+cur.Name+"学习"))
		if err := d.LearnOnce(); err != nil {
			logLive(liveLog, fmt.Sprintf("[%s] ⚠️ 学习失败: %v", time.Now().Format("15:04"), err))
		} else {
			logLive(liveLog, fmt.Sprintf("[%s] ✅ 学习完成", time.Now().Format("15:04")))
		}
		// 精读 arXiv
		logLive(liveLog, fmt.Sprintf("[%s] 📄 精读 arXiv", time.Now().Format("15:04")))
		if err := d.arxivDigest(); err != nil {
			logLive(liveLog, fmt.Sprintf("[%s] ⚠️ arXiv 精读失败: %v", time.Now().Format("15:04"), err))
		} else {
			logLive(liveLog, fmt.Sprintf("[%s] ✅ arXiv 精读完成", time.Now().Format("15:04")))
		}
		updateLiveFrame(frameOf(w, d, "✅ 学习+精读完成（写了日记）"))

	case "skill":
		// 获取技能：LLM 判断用户可能有用的技能 → 生成进技能库
		skill := llmSkillAcquire(d)
		if skill != "" {
			logLive(liveLog, fmt.Sprintf("[%s] 🛠️ 获取新技能：%s", time.Now().Format("15:04"), skill))
			updateLiveFrame(frameOf(w, d, "🛠️ 获取技能："+skill))
		} else {
			logLive(liveLog, fmt.Sprintf("[%s] 🛠️ 技能获取未成功（模型/限流）", time.Now().Format("15:04")))
		}

	case "social":
		// 社交：社交区域遇到其他女儿（云端明信片）
		if meet := w.MeetFriend(home); meet != "" {
			logLive(liveLog, fmt.Sprintf("[%s] 👭 在%s遇到 %s", time.Now().Format("15:04"), w.CurrentRegion().Name, meet))
			f := frameOf(w, d, "👭 遇到 "+meet)
			f.Friend = meet
			updateLiveFrame(f)
		} else {
			logLive(liveLog, fmt.Sprintf("[%s] 👭 这里没什么人，换个地方看看", time.Now().Format("15:04")))
		}

	case "reflect":
		// 思考：模型生成一句想法（写日记）
		thought := d.modelThought()
		logLive(liveLog, fmt.Sprintf("[%s] 💭 %s", time.Now().Format("15:04"), thought))
		updateLiveFrame(frameOf(w, d, "💭 "+thought))

	case "journal":
		// 写日记：今天的总结
		entry := d.modelJournalEntry()
		if entry != "" {
			logLive(liveLog, fmt.Sprintf("[%s] 📝 写了日记", time.Now().Format("15:04")))
			updateLiveFrame(frameOf(w, d, "📝 写日记："+runeClip(entry, 30)))
		}
	}
}

// frameOf 从世界状态构建直播帧
func frameOf(w *worldState, d *Daughter, action string) sceneFrame {
	cur := w.CurrentRegion()
	f := sceneFrame{
		RegionName: cur.Name,
		RegionIcon: cur.Icon,
		RegionKind: cur.Kind,
		X:          w.X,
		Y:          w.Y,
		Action:     action,
		Mood:       d.moodEmoji(),
		Ability:    w.abilitySummary(),
		Seed:       w.WorldSeed,
	}
	if len(w.Friends) > 0 {
		f.Friend = w.Friends[0].Name
	}
	return f
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

// liveLogTailLines 读直播日志尾部 N 行（返回行切片）
func liveLogTailLines(home string, n int) []string {
	data, err := os.ReadFile(filepath.Join(home, "live.log"))
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}

// liveLogTail 读直播日志尾部 N 行（Greet 播报「你不在的时候」用）
func liveLogTail(home string, n int) string {
	return strings.Join(liveLogTailLines(home, n), "\n") + "\n"
}
