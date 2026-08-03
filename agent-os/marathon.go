package main

// marathon.go — Rescene Agent OS 24H 马拉松模式
//
// 核心循环：需求 → 计划 → 自检（Rescene 方法论）
//   - 热点驱动：自动抓取 Hacker News / GitHub 前沿话题 → 模型选题立项
//   - 用户自编排：--task "..." 直接指定项目方向，跳过热点选题
//   - 迭代做项目：每个项目经历 执行 → 自检 多轮循环，越做越完善
//   - 模型轮换：全网免费模型（Zen 免key + 商汤/魔搭/阶跃等）自动轮询 + 熔断退避
//   - 全程日志 + 战报 report.md

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// —— 配置 ——

type marathonConfig struct {
	hours       int           // 运行时长（小时）
	interval    time.Duration // 每轮间隔（含失败退避基准）
	outDir      string        // 输出目录
	customTask  string        // 用户自编排任务（非空则跳过热点选题）
	customModel string        // 固定模型（空=自动轮换）
	iterPerProj int           // 每个项目迭代轮数（执行+自检 对数 ×2）
	hotSource   string        // 热点源: hn / github / off
	quick       bool          // 快速自测模式
	fixedRounds int           // 固定轮数（>0 时优先于 hours）
}

func defaultMarathonConfig() marathonConfig {
	return marathonConfig{
		hours:       24,
		interval:    60 * time.Second,
		outDir:      "marathon",
		iterPerProj: 6, // 3 对 执行+自检
		hotSource:   "hn",
	}
}

// —— 任务阶段 ——

type stage int

const (
	stageKickoff stage = iota // 需求+计划（立项）
	stageExec                 // 执行（写代码/文档）
	stageCheck                // 自检（对照需求计划）
)

func (s stage) String() string {
	switch s {
	case stageKickoff:
		return "kickoff"
	case stageExec:
		return "exec"
	case stageCheck:
		return "check"
	}
	return "?"
}

// —— 统计 ——

type marathonStats struct {
	mu       sync.Mutex
	start    time.Time
	end      time.Time
	rounds   int
	success  int
	fail     int
	modelOK  map[string]int
	modelBad map[string]int
	latency  map[string]time.Duration
	projects int
	files    int
}

func newMarathonStats() *marathonStats {
	return &marathonStats{
		start:    time.Now(),
		modelOK:  make(map[string]int),
		modelBad: make(map[string]int),
		latency:  make(map[string]time.Duration),
	}
}

// —— 主入口 ——

func runMarathon(args []string) {
	cfg := defaultMarathonConfig()
	parseMarathonArgs(args, &cfg)
	if cfg.fixedRounds > 0 {
		cfg.hours = 1 // fixedRounds 优先
	}

	InitRouter()
	models := GetWorkingModels()
	if len(models) == 0 {
		fmt.Println(ColorRed + "❌ 没有可用模型。配置环境变量或使用免 key 的 Zen 模型。" + ColorReset)
		os.Exit(1)
	}
	if cfg.customModel != "" {
		models = filterModels(models, cfg.customModel)
		if len(models) == 0 {
			fmt.Printf(ColorRed+"❌ 模型不存在或未配置 key: %s\n"+ColorReset, cfg.customModel)
			os.Exit(1)
		}
	}

	roundsDir := filepath.Join(cfg.outDir, "projects")
	if err := os.MkdirAll(roundsDir, 0o755); err != nil {
		fmt.Println("❌ 无法创建输出目录:", err)
		os.Exit(1)
	}

	stats := newMarathonStats()
	totalRounds := cfg.hours * 3600 / int(cfg.interval.Seconds())
	if cfg.fixedRounds > 0 {
		totalRounds = cfg.fixedRounds
	}

	printMarathonBanner(cfg, models, totalRounds)

	logFile, err := os.OpenFile(filepath.Join(cfg.outDir, "marathon.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fmt.Println("❌ 无法创建日志文件:", err)
		os.Exit(1)
	}
	defer logFile.Close()

	logLine := func(format string, a ...any) {
		line := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), fmt.Sprintf(format, a...))
		fmt.Println(line)
		logFile.WriteString(line + "\n")
	}

	// 优雅退出：Ctrl+C 也生成战报
	gracefulExit = func() {
		stats.mu.Lock()
		stats.end = time.Now()
		stats.mu.Unlock()
		writeMarathonReport(cfg.outDir, stats)
		fmt.Println(ColorYellow + "\n⏹️  提前结束，战报已生成。再见～" + ColorReset)
		os.Exit(0)
	}

	logLine("🏁 马拉松开始：%d 个模型，目标 %d 轮，热点源=%s", len(models), totalRounds, cfg.hotSource)

	// 项目状态机
	projIdx := 0
	var projName, projDir, projBrief string

	for round := 1; round <= totalRounds; round++ {
		// 计算当前阶段：每项目 iterPerProj 轮（1 kickoff + (iterPerProj-1) 迭代轮）
		phaseInProj := (round - 1) % cfg.iterPerProj
		if phaseInProj == 0 {
			// 新项目 kickoff
			projIdx++
			projName, projBrief = kickoffProject(cfg, logLine)
			if projName == "" {
				logLine("⚠️  立项失败，等待 %s 后重试...", cfg.interval)
				time.Sleep(cfg.interval)
				round-- // 重试本轮
				continue
			}
			projDir = filepath.Join(cfg.outDir, "projects",
				fmt.Sprintf("%03d-%s", projIdx, sanitizeFilename(projName)))
			os.MkdirAll(projDir, 0o755)
			stats.mu.Lock()
			stats.projects++
			stats.files++
			stats.mu.Unlock()
			// 立项产出立即落盘
			fpath := filepath.Join(projDir, "00-需求计划.md")
			os.WriteFile(fpath, []byte(projBrief), 0o644)
			logLine("📦 新项目 [%03d] %s → %s", projIdx, projName, projDir)
			logLine("   ✓ 立项产出已落盘 → %s", fpath)
			// kickoff 已完成，直接进入下一轮（开始执行）
			continue
		}

		// 计算当前阶段（kickoff 后交替 执行/自检）
		st := stageKickoff
		if phaseInProj > 0 {
			if phaseInProj%2 == 1 {
				st = stageExec
			} else {
				st = stageCheck
			}
		}

		// 选模型（跳过熔断）
		model := pickModel(models, round)
		if model == nil {
			logLine("⚠️  所有模型熔断中，等待 %s...", cfg.interval)
			time.Sleep(cfg.interval)
			continue
		}

		prompt := buildStagePrompt(st, projName, projBrief, projDir, round, phaseInProj, cfg)
		logLine("▶ 轮 %d/%d | %s | 模型 %s | 项目 %s", round, totalRounds, st, model.ID, projName)

		start := time.Now()
		content, err := callWithRetry(model, prompt, 3, cfg.interval)
		dur := time.Since(start)

		stats.mu.Lock()
		stats.rounds++
		if err != nil {
			stats.fail++
			stats.modelBad[model.ID]++
			stats.mu.Unlock()
			logLine("   ✗ 失败 (%s): %v", dur.Round(time.Millisecond), err)
			// 失败也落盘，便于排障
			writeStageFile(projDir, st, round, phaseInProj, content, true)
			stats.mu.Lock()
			stats.files++
			stats.mu.Unlock()
			continue
		}

		stats.success++
		stats.modelOK[model.ID]++
		stats.latency[model.ID] += dur
		stats.mu.Unlock()

		fpath := writeStageFile(projDir, st, round, phaseInProj, content, false)
		stats.mu.Lock()
		stats.files++
		stats.mu.Unlock()
		logLine("   ✓ 成功 (%s) → %s", dur.Round(time.Millisecond), fpath)

		// 迭代轮：把产出喂回项目上下文（下一轮自检/执行用）
		if st == stageExec {
			projBrief = extractProjectBrief(projBrief, content)
		}

		time.Sleep(cfg.interval)
	}

	stats.mu.Lock()
	stats.end = time.Now()
	stats.mu.Unlock()
	writeMarathonReport(cfg.outDir, stats)
	logLine("🏁 马拉松完成！战报: %s/report.md", cfg.outDir)
}

// —— 立项：热点选题 or 用户自编排 ——

func kickoffProject(cfg marathonConfig, logLine func(string, ...any)) (string, string) {
	if cfg.customTask != "" {
		return "用户任务-" + time.Now().Format("0102-1504"), cfg.customTask
	}

	// 抓热点
	topics, err := fetchHotTopics(cfg.hotSource)
	if err != nil {
		logLine("   ⚠️ 热点抓取失败: %v（改用内置话题）", err)
		topics = fallbackTopics
	}
	if len(topics) == 0 {
		logLine("   ⚠️ 热点抓取为空（改用内置话题）")
		topics = fallbackTopics
	}

	// 模型选题 + 立项（需求+计划）— 尝试轮换模型直到有可用的
	models := GetWorkingModels()
	if len(models) == 0 {
		logLine("   ❌ 无可用模型")
		return "", ""
	}

	var lastErr error
	for attempt := 0; attempt < len(models); attempt++ {
		model := pickModel(models, int(time.Now().UnixNano())+attempt)
		if model == nil {
			lastErr = fmt.Errorf("所有模型熔断中")
			break
		}

		prompt := fmt.Sprintf(`你是 Rescene Agent OS 的项目立项官。基于以下今日前沿话题，选择一个最有价值的做项目。

今日话题:
%s

要求（遵循 需求→计划 方法论）:
1. 【选题】一句话说明选哪个、为什么（用户价值 + 可行性）
2. 【需求】目标用户、核心功能、验收标准（3条）
3. 【计划】实现步骤（5步以内，可在一台普通电脑上完成，纯代码/脚本/文档类）

输出格式（严格）:
项目名称: <10字以内>
---需求---
...
---计划---
...`, strings.Join(topics, "\n"))

		msg := ChatRequest{
			Model:       model.Model,
			Messages:    []ChatMessage{
				{Role: "system", Content: "你是 Rescene Agent OS 的立项官。输出简洁、可执行，用中文。"},
				{Role: "user", Content: prompt},
			},
			Stream:      true,
			MaxTokens:   2048,
			Temperature: 0.7,
		}

		content, err := CompleteWithModel(context.Background(), model.ID, msg, nil)
		if err != nil {
			lastErr = fmt.Errorf("[%s] %v", model.ID, err)
			logLine("   ⚠️ 立项模型 %s 失败: %v", model.ID, err)
			continue
		}

		name := parseProjectName(content)
		if name == "" {
			name = "项目-" + time.Now().Format("0102-1504")
		}
		return name, content
	}
	logLine("   ❌ 立项失败: %v", lastErr)
	return "", ""
}

// —— 阶段提示词 ——

func buildStagePrompt(st stage, projName, projBrief, projDir string, round, phaseInProj int, cfg marathonConfig) string {
	switch st {
	case stageExec:
		return fmt.Sprintf("你是 Rescene Agent OS 的开发核心。项目「%s」当前上下文：\n\n%s\n\n请执行本轮开发：写出真实可用的代码/脚本/文档（纯文本，直接输出，代码用三个反引号围栏包裹）。\n优先实现最小可用版本，下一轮会自检并改进。", projName, briefOr(projBrief, "（暂无上下文）"))
	case stageCheck:
		return fmt.Sprintf("你是 Rescene Agent OS 的质量官。对项目「%s」最近一轮产出做严格自检：\n\n%s\n\n自检清单（输出格式）:\n---问题---\n1. ...\n---改进---\n下一轮执行时优先修复的问题（最多3条，具体可执行）", projName, briefOr(projBrief, "（无产出）"))
	default: // kickoff
		return projBrief
	}
}

// —— 模型选择与重试 ——

func pickModel(models []FreeModel, seed int) *FreeModel {
	if len(models) == 0 {
		return nil
	}
	for k := 0; k < len(models); k++ {
		cand := models[(seed+k)%len(models)]
		if !circuitIsOpen(cand) {
			m := cand
			return &m
		}
	}
	return nil
}

// callWithRetry 带退避重试（429/5xx 等临时错误）
func callWithRetry(m *FreeModel, prompt string, attempts int, interval time.Duration) (string, error) {
	msg := ChatRequest{
		Model: m.Model,
		Messages: []ChatMessage{
			{Role: "system", Content: "你是 Rescene Agent OS 的自主工作核心。输出直接给正文，中文。"},
			{Role: "user", Content: prompt},
		},
		Stream:      true,
		MaxTokens:   2048,
		Temperature: 0.7,
	}

	var lastErr error
	for i := 0; i < attempts; i++ {
		content, err := CompleteWithModel(context.Background(), m.ID, msg, nil)
		if err == nil {
			return content, nil
		}
		lastErr = err
		// 429 限流：熔断该模型，直接失败——外层会轮换到下一个模型
		if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "Rate limit") {
			circuitFail(*m)
			return "", err
		}
		// 其它错误（超时/解析）短暂重试
		if i < attempts-1 {
			time.Sleep(2 * time.Second)
		}
	}
	return "", lastErr
}

// —— 文件落盘 ——

func writeStageFile(projDir string, st stage, round, phaseInProj int, content string, failed bool) string {
	var fname string
	switch st {
	case stageKickoff:
		fname = "00-需求计划.md"
	case stageExec:
		fname = fmt.Sprintf("%02d-执行-%03d.md", phaseInProj/2+1, round)
	case stageCheck:
		fname = fmt.Sprintf("%02d-自检-%03d.md", phaseInProj/2+1, round)
	}
	if failed {
		fname = "FAILED-" + fname
	}
	fpath := filepath.Join(projDir, fname)
	os.WriteFile(fpath, []byte(content), 0o644)
	return fpath
}

// extractProjectBrief 增量拼接项目上下文（控制长度）
func extractProjectBrief(brief, newContent string) string {
	if len(newContent) > 6000 {
		newContent = newContent[:6000]
	}
	if brief == "" {
		return newContent
	}
	if len(brief)+len(newContent) > 12000 {
		// 保留最近产出
		return brief[len(brief)-6000:] + "\n\n---本轮新产出---\n" + newContent
	}
	return brief + "\n\n---本轮新产出---\n" + newContent
}

func briefOr(b, fallback string) string {
	if strings.TrimSpace(b) == "" {
		return fallback
	}
	return b
}

// —— 热点抓取 ——

var hotHTTPClient = &http.Client{Timeout: 15 * time.Second}

var fallbackTopics = []string{
	"开源 AI 推理框架的最新进展",
	"终端 CLI 工具的新范式",
	"免费模型 API 的聚合与路由",
	"本地优先的 AI 应用架构",
	"开发者工具链的自动化",
}

func fetchHotTopics(source string) ([]string, error) {
	switch source {
	case "hn":
		return fetchHNTopics()
	case "github":
		return fetchGitHubTopics()
	default:
		return fetchHNTopics()
	}
}

// fetchHNTopics Hacker News 官方 API（免费无 key）— 并发抓取
func fetchHNTopics() ([]string, error) {
	resp, err := hotHTTPClient.Get("https://hacker-news.firebaseio.com/v0/topstories.json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	var ids []int
	if err := json.Unmarshal(body, &ids); err != nil {
		return nil, err
	}
	if len(ids) > 10 {
		ids = ids[:10]
	}

	// 并发抓取标题（每个 item 独立请求，互相不拖累）
	type result struct {
		title string
	}
	ch := make(chan result, len(ids))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 5) // 最多 5 并发

	for _, id := range ids {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			client := &http.Client{Timeout: 8 * time.Second}
			ir, err := client.Get(fmt.Sprintf("https://hacker-news.firebaseio.com/v0/item/%d.json", id))
			if err != nil {
				return
			}
			ib, _ := io.ReadAll(io.LimitReader(ir.Body, 1<<20))
			ir.Body.Close()
			var item struct {
				Title string `json:"title"`
				Type  string `json:"type"`
			}
			if json.Unmarshal(ib, &item) == nil && item.Title != "" && item.Type == "story" {
				ch <- result{title: item.Title}
			}
		}(id)
	}
	wg.Wait()
	close(ch)

	var topics []string
	for r := range ch {
		topics = append(topics, r.title)
	}
	return topics, nil
}

// fetchGitHubTopics GitHub 搜索 API（免费限流 60次/h）
func fetchGitHubTopics() ([]string, error) {
	resp, err := hotHTTPClient.Get(
		"https://api.github.com/search/repositories?q=stars:%3E500+created:%3E2026-01-01&sort=stars&order=desc&per_page=10")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	var out struct {
		Items []struct {
			FullName    string `json:"full_name"`
			Description string `json:"description"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	var topics []string
	for _, it := range out.Items {
		t := it.FullName
		if it.Description != "" {
			t += " — " + it.Description
		}
		topics = append(topics, t)
	}
	return topics, nil
}

// —— 解析 ——

var nameRe = regexp.MustCompile(`项目名称[:：]\s*([^\n]{1,20})`)

func parseProjectName(content string) string {
	if m := nameRe.FindStringSubmatch(content); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func filterModels(models []FreeModel, id string) []FreeModel {
	var out []FreeModel
	for _, m := range models {
		if m.ID == id {
			out = append(out, m)
		}
	}
	return out
}

func parseMarathonArgs(args []string, cfg *marathonConfig) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		next := func() string {
			if i+1 < len(args) {
				i++
				return args[i]
			}
			return ""
		}
		switch a {
		case "--hours":
			if v := next(); v != "" {
				fmt.Sscanf(v, "%d", &cfg.hours)
			}
		case "--interval":
			if v := next(); v != "" {
				var sec int
				if n, _ := fmt.Sscanf(v, "%d", &sec); n == 1 && sec > 0 {
					cfg.interval = time.Duration(sec) * time.Second
				}
			}
		case "--dir":
			if v := next(); v != "" {
				cfg.outDir = v
			}
		case "--task":
			if v := next(); v != "" {
				cfg.customTask = v
			}
		case "--model":
			if v := next(); v != "" {
				cfg.customModel = v
			}
		case "--hot":
			if v := next(); v != "" {
				cfg.hotSource = v
			}
		case "--iters":
			if v := next(); v != "" {
				var n int
				if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
					cfg.iterPerProj = n
				}
			}
		case "--rounds":
			if v := next(); v != "" {
				var n int
				if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
					cfg.fixedRounds = n
				}
			}
		case "--quick":
			cfg.quick = true
			cfg.hours = 1
			cfg.interval = 8 * time.Second
			cfg.iterPerProj = 3
		}
	}
	if cfg.hours < 1 {
		cfg.hours = 1
	}
	if cfg.fixedRounds > 0 {
		cfg.hours = 1
	}
}

func printMarathonBanner(cfg marathonConfig, models []FreeModel, totalRounds int) {
	fmt.Printf(ColorCyan+`
╔══════════════════════════════════════════════╗
║   RESCENE AGENT OS · 24H 马拉松模式          ║
║   需求→计划→自检 · 热点驱动 · 自主立项      ║
╚══════════════════════════════════════════════╝
`+ColorReset)
	fmt.Printf("⏱️  时长:      %d 小时 (~%d 轮)\n", cfg.hours, totalRounds)
	fmt.Printf("🔄 间隔:      %s\n", cfg.interval)
	fmt.Printf("📡 模型:      %d 个可用\n", len(models))
	for _, m := range models {
		mark := "🔓免key"
		if !m.Keyless {
			mark = "🔑需key"
		}
		fmt.Printf("               %s %s\n", mark, m.ID)
	}
	fmt.Printf("📂 输出目录:  %s\n", cfg.outDir)
	if cfg.customTask != "" {
		fmt.Printf("🎯 自编排:    %s\n", truncateStr(cfg.customTask, 60))
	} else {
		fmt.Printf("🔥 热点源:    %s（自动选题立项）\n", cfg.hotSource)
	}
	fmt.Printf("♻️  每项目:    %d 轮（1立项 + %d 次执行/自检）\n", cfg.iterPerProj, cfg.iterPerProj-1)
	fmt.Println("按 Ctrl+C 提前结束并生成战报。")
	fmt.Println()
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// —— 战报 ——

func writeMarathonReport(outDir string, stats *marathonStats) {
	stats.mu.Lock()
	defer stats.mu.Unlock()

	var sb strings.Builder
	sb.WriteString("# 🌐 Rescene Agent OS · 24H 马拉松战报\n\n")
	sb.WriteString(fmt.Sprintf("- 开始时间: %s\n", stats.start.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("- 结束时间: %s\n", stats.end.Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("- 运行时长: %s\n", stats.end.Sub(stats.start).Round(time.Second)))
	sb.WriteString(fmt.Sprintf("- 总轮数: **%d**\n", stats.rounds))
	rate := 0.0
	if stats.rounds > 0 {
		rate = float64(stats.success) / float64(stats.rounds) * 100
	}
	sb.WriteString(fmt.Sprintf("- 成功率: **%.1f%%** (%d 成功 / %d 失败)\n", rate, stats.success, stats.fail))
	sb.WriteString(fmt.Sprintf("- 立项项目: **%d** 个\n", stats.projects))
	sb.WriteString(fmt.Sprintf("- 产出文件: **%d** 个\n\n", stats.files))

	sb.WriteString("## 📊 模型表现\n\n")
	sb.WriteString("| 模型 | 成功 | 失败 | 平均耗时 |\n")
	sb.WriteString("|------|------|------|----------|\n")
	ids := make([]string, 0)
	seen := map[string]bool{}
	for k := range stats.modelOK {
		if !seen[k] {
			seen[k] = true
			ids = append(ids, k)
		}
	}
	for k := range stats.modelBad {
		if !seen[k] {
			seen[k] = true
			ids = append(ids, k)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		ok := stats.modelOK[id]
		bad := stats.modelBad[id]
		avg := time.Duration(0)
		if ok > 0 {
			avg = stats.latency[id] / time.Duration(ok)
		}
		sb.WriteString(fmt.Sprintf("| %s | %d | %d | %s |\n", id, ok, bad, avg.Round(time.Millisecond)))
	}

	sb.WriteString("\n## 📁 项目产出\n\n")
	projRoot := filepath.Join(outDir, "projects")
	entries, _ := os.ReadDir(projRoot)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	if len(entries) == 0 {
		sb.WriteString("（无立项产出）\n")
	} else {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			sb.WriteString(fmt.Sprintf("- **%s**\n", e.Name()))
			files, _ := os.ReadDir(filepath.Join(projRoot, e.Name()))
			for _, f := range files {
				if !f.IsDir() {
					sb.WriteString(fmt.Sprintf("  - [%s](projects/%s/%s)\n", f.Name(), e.Name(), f.Name()))
				}
			}
		}
	}

	sb.WriteString("\n---\n*Rescene Agent OS · 需求→计划→自检 · 让 AI 成为公共资源 (｡•ᴗ•｡)♡*\n")

	os.WriteFile(filepath.Join(outDir, "report.md"), []byte(sb.String()), 0o644)
}

// sanitizeFilename 清理文件名中的非法字符
func sanitizeFilename(s string) string {
	replacer := strings.NewReplacer(
		" ", "-", "/", "-", "\\", "-", ":", "-", "*", "-",
		"?", "-", "\"", "-", "<", "-", ">", "-", "|", "-",
		"·", "-", "（", "(", "）", ")", "，", ",", "。", "",
		"\n", "-", "\r", "-",
	)
	return replacer.Replace(s)
}

// gracefulExit 由 main.go 的信号处理调用；marathon 会覆盖它以优雅生成战报
var gracefulExit = func() {
	os.Exit(0)
}

// showReport 查看马拉松战报：rescene report [--dir marathon]
func showReport(args []string) {
	outDir := "marathon"
	for i := 0; i < len(args); i++ {
		if args[i] == "--dir" && i+1 < len(args) {
			outDir = args[i+1]
			i++
		}
	}

	if !printReport(outDir) {
		fmt.Printf("❌ 找不到战报: %s\n", filepath.Join(outDir, "report.md"))
		fmt.Println("先运行 `rescene marathon` 产生战报，或用 --dir 指定输出目录。")
		os.Exit(1)
	}
}

// printReport 打印战报内容；返回是否找到。供 CLI 和交互式 /report 共用，不退出进程。
func printReport(outDir string) bool {
	reportPath := filepath.Join(outDir, "report.md")
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return false
	}
	fmt.Print(string(data))
	fmt.Println()
	return true
}
