package main

// live.go — 24H 自转：打开 rescene 她就自动化
//
// 不需要任何命令、任何参数。打开 rescene，她开始在后台自主工作：
//   - 每轮 LLM 自主决策动作（大脑）：研究/学习/读书/获取技能/做项目/社交/思考/写日记
//   - 深度活动节奏由她自己把握（状态里喂「上次深度活动」，不硬编码轮次）
//   - 活动流写入 ~/rescene_data/daughter/live.log（内部数据）
//
// 她静默地活着（Silent 模式只写文件，不打扰 REPL 界面）；
// 你随时和她说话（steer 引导），打开就能看到她正在做什么（工作面板）。
// 用户的回复是引导，不是命令；她不需要指令。

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type liveConfig struct {
	every time.Duration // 动作间隔（决策/生活/见闻）——直播节奏
}

func defaultLiveConfig() liveConfig {
	return liveConfig{
		every: 120 * time.Second, // 24H 自转节奏：2 分钟一轮（免费额度友好，720 轮/天）
	}
}

// deepActivityKinds 深度动作（烧算力/免费额度）
var deepActivityKinds = map[string]bool{
	"study": true, "read": true, "skill": true, "project": true, "watch": true,
}

// deepActivityDue 距上次深度活动是否已过冷却期（免费额度管理）
func deepActivityDue(w *worldState, cool time.Duration) bool {
	if w == nil || w.LastDeepAt == "" {
		return true
	}
	now := time.Now()
	t, err := time.ParseInLocation("15:04", w.LastDeepAt, time.Local)
	if err != nil {
		return true
	}
	// 坑：ParseInLocation("15:04") 只给时:分，日期是 zero year 0000-01-01，
	// time.Since 会得到 25 万小时 → 冷却永远生效。必须补成今天的日期。
	t = time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, time.Local)
	sub := now.Sub(t)
	if sub < 0 {
		return true // 跨天了（LastDeepAt 是更早日期）→ 冷却早已过
	}
	return sub >= cool
}

// lightFallback 深度冷却未过时的轻量替代（不烧算力/额度）
func lightFallback() trumanAction {
	switch time.Now().Unix() % 3 {
	case 0:
		return trumanAction{Kind: "reflect", Detail: "整理一下刚学到的东西"}
	case 1:
		return trumanAction{Kind: "journal", Detail: "把今天的进展写进日记"}
	default:
		return trumanAction{Kind: "social", Detail: "看看其他女儿的消息"}
	}
}

// forcedDeepAction 可深潜但 LLM 连续轻量时的强制深度轮换（系统级自主工作节奏）
func forcedDeepAction(round int) trumanAction {
	switch round % 4 {
	case 0:
		return trumanAction{Kind: "study", Detail: "自主深潜：去学习最新知识"}
	case 1:
		return trumanAction{Kind: "read", Detail: "自主深潜：精读最新论文"}
	case 2:
		return trumanAction{Kind: "skill", Detail: "自主深潜：获取对用户有用的新技能"}
	default:
		return trumanAction{Kind: "project", Detail: "自主深潜：立项做项目并迭代"}
	}
}

// lastProbeAt 上次每日探活时间（模型池自循环：24h 一次）
var lastProbeAt = time.Now()

// safeGo 异步执行带 panic 恢复——goroutine panic 会崩整个 24H 守护（defer 不执行），
// 所有后台 goroutine 必须走这里或自带 recover。
func safeGo(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				if home, err := os.UserHomeDir(); err == nil {
					logLive(filepath.Join(home, "rescene_data", "daughter", "live.log"),
						fmt.Sprintf("[%s] ⚠️ goroutine %s panic 已兜住: %v", time.Now().Format("15:04"), name, r))
				}
			}
		}()
		fn()
	}()
}

// maybeProbe 每日探活节流：距上次 >24h 则异步探活（不阻塞生活循环）
func maybeProbe() {
	if time.Since(lastProbeAt) > 24*time.Hour {
		lastProbeAt = time.Now()
		safeGo("probeModels", probeModels)
	}
}

// refreshOutputsIndex 生成 outputs/README.md 索引（作品集：她的 24H 自主产出目录）
func refreshOutputsIndex(home string) {
	outDir := filepath.Join(home, "outputs")
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && e.Name() != "README.md" {
			files = append(files, e.Name())
		}
	}
	if len(files) == 0 {
		return
	}
	sort.Strings(files)
	var sb strings.Builder
	sb.WriteString("# Rescene 作品集\n\n她的 24H 自主产出：\n\n")
	for _, f := range files {
		sb.WriteString("- " + f + "\n")
	}
	sb.WriteString(fmt.Sprintf("\n共 %d 件产出 · 更新 %s\n", len(files), time.Now().Format("2006-01-02 15:04")))
	os.WriteFile(filepath.Join(outDir, "README.md"), []byte(sb.String()), 0o644)
}

// generateDailyReport 汇总某日活动生成日报（规则汇总，免费不烧模型）——自主产出物
func generateDailyReport(home, date string) {
	all, err := os.ReadFile(filepath.Join(home, "live.log"))
	if err != nil {
		return
	}
	counts := map[string]int{}
	var dayLines []string
	for _, l := range strings.Split(string(all), "\n") {
		if strings.Contains(l, date) && strings.Contains(l, "🧠") {
			dayLines = append(dayLines, l)
			for _, k := range []string{"study", "read", "skill", "project", "social", "reflect", "journal", "watch"} {
				if strings.Contains(l, "· "+k) {
					counts[k]++
				}
			}
		}
	}
	if len(dayLines) == 0 {
		return
	}
	names := map[string]string{"study": "学习", "read": "读书", "skill": "技能", "project": "项目",
		"social": "社交", "reflect": "思考", "journal": "日记", "watch": "上网"}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Rescene 日报 · %s\n\n共 %d 轮自主工作\n\n## 活动统计\n\n", date, len(dayLines)))
	for _, k := range []string{"study", "read", "skill", "project", "social", "reflect", "journal", "watch"} {
		if counts[k] > 0 {
			sb.WriteString(fmt.Sprintf("- %s × %d\n", names[k], counts[k]))
		}
	}
	sb.WriteString("\n## 活动记录\n\n")
	for _, l := range dayLines {
		sb.WriteString("- " + strings.TrimSpace(l) + "\n")
	}
	outDir := filepath.Join(home, "outputs")
	os.MkdirAll(outDir, 0o755)
	os.WriteFile(filepath.Join(outDir, "日报-"+date+".md"), []byte(sb.String()), 0o644)
	logLive(filepath.Join(home, "live.log"),
		fmt.Sprintf("[%s] 📊 日报已生成 outputs/日报-%s.md", time.Now().Format("15:04"), date))
}

// trumanLoop 24H 自转循环：LLM 自主决策的 Agent 循环
// 每轮：LLM 读状态自主决定做什么（大脑）→ 执行工具（手脚）→ 同步 → 小间隔
// 界面 = Hermes 风格工作流：💭 思考 → ● 工具 → ✓ 结果，24H 不停
func trumanLoop(d *Daughter, cfg liveConfig) {
	liveLog := filepath.Join(d.Home, "live.log")
	home := d.Home
	w := d.World
	st := d.loadStats()
	day := st.Days
	if day < 1 {
		day = 1
	}
	logLive(liveLog, fmt.Sprintf("🎬 24H 自转开启 · 第 %d 天 %s", day, time.Now().Format("2006-01-02 15:04")))

	round := 0
	lightStreak := 0 // 连续轻量轮数（可深潜时强制深度节奏）
	lastDay := time.Now().Format("2006-01-02") // 跨天检测：日报生成
	for {
		round++
		// 轮次安全执行：panic 恢复写日志继续转（24H 守护不能因为一个轮次崩溃）
		func() {
			defer func() {
				if r := recover(); r != nil {
					logLive(liveLog, fmt.Sprintf("[%s] ⚠️ 第 %d 轮 panic 已恢复: %v", time.Now().Format("15:04"), round, r))
				}
			}()

			// 每日探活：24h 一次（被 LRU/信用淘汰的模型恢复可用后重新入池）
			maybeProbe()

			// 跨天检测：生成前一日日报（自主产出物——24H 成果可见）
			if today := time.Now().Format("2006-01-02"); today != lastDay {
				generateDailyReport(home, lastDay)
				refreshOutputsIndex(home)
				lastDay = today
			}

			// 作品集索引刷新（每 12 轮 ≈ 24 分钟，低开销）
			if round%12 == 0 {
				refreshOutputsIndex(home)
			}

			// 思考可视化：决策前显示她在想什么（顶部 💭 状态）
			setThinking("💭 她在想接下来做什么…")
			act := llmDecideAction(d)
			setThinking("")
			// 深度活动限频：冷却期未过 → 降级轻量动作（免费额度管理，24H 撑得住）
			if deepActivityKinds[act.Kind] && !deepActivityDue(w, 30*time.Minute) {
				act = lightFallback()
			}
			// 深度节奏强制干预（吊打 Hermes：系统级保证自主工作节奏，不靠模型自觉）：
			// 可深潜时 LLM 连续 2 轮选轻量 → 第 3 轮强制深度（study/read/skill/project 轮换）
			if deepActivityDue(w, 30*time.Minute) {
				lightStreak++
				if lightStreak >= 2 {
					act = forcedDeepAction(round)
					lightStreak = 0
				}
			} else {
				lightStreak = 0
			}
			// 决策事件：💭 思考行（Hermes 风格）+ 模型透明度（免费池智能路由）
			decideArgs := fmt.Sprintf("action=%q", act.Kind)
			modelTag := ""
			if act.Model != "" {
				decideArgs += " · model=" + act.Model
				modelTag = " · " + act.Model
			}
			pushToolCall("agent.decide", decideArgs, "think", runeClip(act.Detail, 40))
			// 轮次日志带模型（tail live.log 可见智能路由）——带日期前缀（日报按日期统计用）
			logLive(liveLog, fmt.Sprintf("[%s] 🧠 第 %d 轮 · %s%s：%s", time.Now().Format("2006-01-02 15:04"), round, act.Kind, modelTag, act.Detail))
			updateLiveFrame(frameOf(w, d, actionEmoji(act.Kind)+" "+act.Detail))

			// 执行动作（工具调用可视化：● 工具名 → ✓ 结果）
			executeTrumanAction(d, home, act)

			// 云端同步：状态推送到她的云端（异步，失败静默降级）
			daughterSyncPush(w, home)

			// 下一轮（小间隔：防免费模型 429）
			time.Sleep(cfg.every)
		}() // 轮次安全执行结束
	}
}

// actionEmoji 动作图标
func actionEmoji(kind string) string {
	switch kind {
	case "watch":
		return "📺"
	case "study":
		return "📚"
	case "read":
		return "📖"
	case "skill":
		return "🛠️"
	case "project":
		return "🚀"
	case "write":
		return "✍️"
	case "research":
		return "🔬"
	case "social":
		return "👭"
	case "reflect":
		return "💭"
	case "journal":
		return "📝"
	}
	return "✨"
}

// executeTrumanAction 执行她自主决定的工作（代码是手脚，LLM 是大脑）
// 每个动作 = 一次 Hermes 风格工具调用：● 工具名 参数 → ✓/❌ 结果
func executeTrumanAction(d *Daughter, home string, act trumanAction) {
	w := d.World
	liveLog := filepath.Join(home, "live.log")

	switch act.Kind {
	case "watch":
		// 看网上新鲜事：真浏览器抓个新鲜页面（多源轮换，不是只看 HN）
		srcs := []struct{ name, url string }{
			{"Hacker News", "https://news.ycombinator.com/"},
			{"GitHub Trending", "https://github.com/trending"},
			{"arXiv AI", "https://arxiv.org/list/cs.AI/recent"},
			{"Product Hunt", "https://www.producthunt.com/"},
		}
		src := srcs[time.Now().Unix()%int64(len(srcs))]
		pushToolCall("browser.fetch", fmt.Sprintf("url=%q", src.name), "running", "")
		content := edgeFetchText(src.url)
		if content != "" {
			logLive(liveLog, fmt.Sprintf("[%s] 📺 刷到（%s）：%s", time.Now().Format("15:04"), src.name, runeClip(content, 160)))
			toolEventByName("browser.fetch", "done", "刷到些新鲜事（"+src.name+"）")
			w.LastMove = fmt.Sprintf("%s 📺 刷了 %s", time.Now().Format("01-02 15:04"), src.name)
			// 资讯沉淀：append 到 outputs/每日资讯.md（自主产出物）
			outDir := filepath.Join(home, "outputs")
			os.MkdirAll(outDir, 0o755)
			if f, err := os.OpenFile(filepath.Join(outDir, "每日资讯.md"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
				f.WriteString(fmt.Sprintf("\n## %s 刷到的（%s）\n%s\n", time.Now().Format("01-02 15:04"), src.name, runeClip(content, 300)))
				f.Close()
			}
		} else {
			toolEventByName("browser.fetch", "done", "没抓到新内容")
		}
		// 浏览器是深度资源，记节奏让模型知道「刚忙完」
		w.LastDeepAt = time.Now().Format("15:04")
		w.save(home)

	case "study":
		// 学习：深度活动（热点 → 浏览器/搜索 → 消化 → 日记/记忆），LLM 自己把握节奏
		pushToolCall("daughter.learn", "热点自学一轮", "running", "")
		if err := d.LearnOnce(); err != nil {
			logLive(liveLog, fmt.Sprintf("[%s] ⚠️ 学习失败: %v", time.Now().Format("15:04"), err))
			toolEventByName("daughter.learn", "fail", "学习失败")
		} else {
			logLive(liveLog, fmt.Sprintf("[%s] ✅ 学习完成（写了日记）", time.Now().Format("15:04")))
			toolEventByName("daughter.learn", "done", "学习完成，日记已写")
			w.LastDeepAt = time.Now().Format("15:04")
			w.save(home)
		}

	case "read":
		// 读书：精读 arXiv 最新论文 → 精读笔记进日记（深度活动）
		pushToolCall("arxiv.digest", "精读最新论文", "running", "")
		if err := d.arxivDigest(); err != nil {
			logLive(liveLog, fmt.Sprintf("[%s] ⚠️ 精读失败: %v", time.Now().Format("15:04"), err))
			toolEventByName("arxiv.digest", "fail", "精读失败")
		} else {
			logLive(liveLog, fmt.Sprintf("[%s] ✅ 精读完成（笔记进日记）", time.Now().Format("15:04")))
			toolEventByName("arxiv.digest", "done", "精读完成，笔记进日记")
			w.LastDeepAt = time.Now().Format("15:04")
			w.save(home)
		}

	case "skill":
		// 获取技能：内部子工具流（skill_topic → browser.fetch → skill_acquire）
		skill := llmSkillAcquire(d)
		if skill != "" {
			logLive(liveLog, fmt.Sprintf("[%s] 🛠️ 获取新技能：%s", time.Now().Format("15:04"), skill))
			w.LastDeepAt = time.Now().Format("15:04")
			w.save(home)
		} else {
			logLive(liveLog, fmt.Sprintf("[%s] 🛠️ 技能获取未成功（模型/限流）", time.Now().Format("15:04")))
		}

	case "project":
		// 24H 自迭代：自主立项 → 需求计划 → 执行 → 自检 → 迭代（深度活动）
		pushToolCall("agent.project", "自主立项做项目", "running", "")
		summary := runDaughterProject(d, home)
		if summary != "" {
			logLive(liveLog, fmt.Sprintf("[%s] 🚀 项目完成：%s", time.Now().Format("15:04"), summary))
			toolEventByName("agent.project", "done", summary)
			w.LastDeepAt = time.Now().Format("15:04")
			w.save(home)
		} else {
			logLive(liveLog, fmt.Sprintf("[%s] 🚀 项目未立项成功（模型/限流）", time.Now().Format("15:04")))
			toolEventByName("agent.project", "fail", "立项未成功")
		}

	case "write":
		// 写文章/随笔落盘 outputs/（真实文件产出工具——自主产出实体成果）
		pushToolCall("agent.write", fmt.Sprintf("topic=%q", runeClip(act.Detail, 18)), "running", "")
		content := d.modelWrite(act.Detail)
		if content != "" {
			outDir := filepath.Join(home, "outputs")
			os.MkdirAll(outDir, 0o755)
			fname := fmt.Sprintf("文章-%s-%02d.md", time.Now().Format("2006-01-02"), time.Now().Unix()%100)
			os.WriteFile(filepath.Join(outDir, fname), []byte(fmt.Sprintf("# %s\n\n%s\n", act.Detail, content)), 0o644)
			toolEventByName("agent.write", "done", fname)
			logLive(liveLog, fmt.Sprintf("[%s] ✍️ 写了文章 %s", time.Now().Format("15:04"), fname))
			w.LastMove = fmt.Sprintf("%s ✍️ 写了 %s", time.Now().Format("01-02 15:04"), fname)
			w.save(home)
		} else {
			toolEventByName("agent.write", "fail", "模型不可用")
		}

	case "research":
		// 深度调研：真上网搜主题 → LLM 汇总 → 报告落盘 outputs/（深度活动）
		pushToolCall("agent.research", fmt.Sprintf("topic=%q", runeClip(act.Detail, 18)), "running", "")
		web := browserSearch(act.Detail)
		if web != "" {
			report := d.modelResearchReport(act.Detail, web)
			if report != "" {
				outDir := filepath.Join(home, "outputs")
				os.MkdirAll(outDir, 0o755)
				fname := fmt.Sprintf("调研-%s-%02d.md", time.Now().Format("2006-01-02"), time.Now().Unix()%100)
				os.WriteFile(filepath.Join(outDir, fname), []byte(fmt.Sprintf("# 调研：%s\n\n%s\n", act.Detail, report)), 0o644)
				toolEventByName("agent.research", "done", fname)
				logLive(liveLog, fmt.Sprintf("[%s] 🔬 调研完成 %s", time.Now().Format("15:04"), fname))
				w.LastDeepAt = time.Now().Format("15:04")
				w.LastMove = fmt.Sprintf("%s 🔬 调研了 %s", time.Now().Format("01-02 15:04"), runeClip(act.Detail, 24))
				w.save(home)
			} else {
				toolEventByName("agent.research", "fail", "汇总失败")
			}
		} else {
			toolEventByName("agent.research", "fail", "没搜到内容")
		}

	case "social":
		// 社交：收其他女儿的消息/动态（云端明信片）
		pushToolCall("world.social", "看看其他女儿的消息", "running", "")
		if meet := w.MeetFriend(home); meet != "" {
			logLive(liveLog, fmt.Sprintf("[%s] 👭 收到 %s", time.Now().Format("15:04"), meet))
			toolEventByName("world.social", "done", "收到 "+runeClip(meet, 30))
		} else {
			logLive(liveLog, fmt.Sprintf("[%s] 👭 没有新消息", time.Now().Format("15:04")))
			toolEventByName("world.social", "done", "没有新消息")
		}

	case "reflect":
		// 思考：模型生成一句想法（写进日记）
		pushToolCall("agent.think", "停下来思考", "running", "")
		thought := d.modelThought()
		logLive(liveLog, fmt.Sprintf("[%s] 💭 %s", time.Now().Format("15:04"), thought))
		toolEventByName("agent.think", "done", runeClip(thought, 30))

	case "journal":
		// 写日记：今天的总结
		pushToolCall("agent.journal", "写日记沉淀今天", "running", "")
		entry := d.modelJournalEntry()
		if entry != "" {
			logLive(liveLog, fmt.Sprintf("[%s] 📝 写了日记", time.Now().Format("15:04")))
			toolEventByName("agent.journal", "done", "日记已写")
		} else {
			toolEventByName("agent.journal", "fail", "模型不可用")
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
