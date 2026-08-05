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
	every     time.Duration // 轮间隔
	taskEvery int           // 每几轮一次 arXiv 精读
}

func defaultLiveConfig() liveConfig {
	return liveConfig{
		every:     30 * time.Minute,
		taskEvery: 4,
	}
}

// trumanLoop 楚门世界循环：静默运行（只写 live.log），由 REPL 打开时后台启动
// 每轮 = 自主探索（决定往哪走）→ 移动一步（新区域生成）→ 活动（学习/精读/社交）
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

		// 1. 自主探索：她决定往哪走（能力短板/心情/探索欲驱动）
		dir, nx, ny, reason := w.PlanNextStep()
		logLive(liveLog, fmt.Sprintf("[%s] 🧭 第 %d 轮 · 决定向%s走（%s）", time.Now().Format("15:04"), round, dir, reason))

		// 2. 移动一步 → 新区域生成（无限世界展开）
		trav := w.StepTo(home, dir, nx, ny)
		logLive(liveLog, fmt.Sprintf("[%s] 🚶 %s", time.Now().Format("15:04"), trav))

		// 3. 在到达区域活动
		// 3a. 学一轮（HN 热点 → Firecrawl → 消化 → 日记/记忆）
		cur := w.CurrentRegion()
		logLive(liveLog, fmt.Sprintf("[%s] 📚 第 %d 轮学习（在%s·%s）", time.Now().Format("15:04"), round, cur.Icon, cur.Name))
		if err := d.LearnOnce(); err != nil {
			logLive(liveLog, fmt.Sprintf("[%s] ⚠️ 学习失败: %v", time.Now().Format("15:04"), err))
		} else {
			logLive(liveLog, fmt.Sprintf("[%s] ✅ 学习完成", time.Now().Format("15:04")))
		}

		// 3b. 每 taskEvery 轮：精读 arXiv + 社交（社交类区域触发）
		if round%cfg.taskEvery == 0 {
			logLive(liveLog, fmt.Sprintf("[%s] 📄 精读 arXiv", time.Now().Format("15:04")))
			if err := d.arxivDigest(); err != nil {
				logLive(liveLog, fmt.Sprintf("[%s] ⚠️ arXiv 精读失败: %v", time.Now().Format("15:04"), err))
			} else {
				logLive(liveLog, fmt.Sprintf("[%s] ✅ arXiv 精读完成", time.Now().Format("15:04")))
			}

			// 社交：在社交类区域遇到其他女儿（云端真实明信片）
			if meet := w.MeetFriend(home); meet != "" {
				logLive(liveLog, fmt.Sprintf("[%s] 👭 在%s遇到 %s", time.Now().Format("15:04"), cur.Name, meet))
			}
		}

		// 4. 云端同步：世界状态推送到她的云端（异步，失败静默降级）
		daughterSyncPush(w, home)

		// 5. 下一轮
		time.Sleep(cfg.every)
	}
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
