package handler

// company_handler.go — 公司管理面板 API（GUI 看百人公司运作）
//   GET /api/company/agents — 所有 agent 列表（含最近活动/产出数）
//   GET /api/company/agent?name=writer-01 — 单个 agent 详情

import (
	"encoding/json"
	"fmt"
	"math"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

// companyAgentInfo 单个 agent 信息
type companyAgentInfo struct {
	Name      string   `json:"name"`
	Role      string   `json:"role"`
	Home      string   `json:"home"`
	RecentLog string   `json:"recentLog,omitempty"`
	Outputs   int      `json:"outputs"`
	Skills    int      `json:"skills"`
	Files     []string `json:"files,omitempty"` // 产出文件名列表（可点开看内容）
	// 人设背景（2026-08-08 每个 agent 员工都有年龄/性别/童年故事）
	Gender    string `json:"gender"`    // 性别
	Age       int    `json:"age"`       // 年龄（出生日期推算）
	Childhood string `json:"childhood"` // 童年故事
	// 协作引用（2026-08-09：真实接力证据，非剧本）
	CollabRefs []CollabRef `json:"collabRefs,omitempty"`
	// 实时状态（2026-08-26：开罗式"每个 agent 正在干什么"）
	Status     string `json:"status"`      // 工作中 / 空闲中 / 停摆 N 天 / 未知
	Task       string `json:"task"`        // 正在做什么（从 live.log 尾部解析）
	LastActive string `json:"lastActive"`  // 最后活动时间（MM-DD HH:mm）
}

// liveLogTimeRe 匹配 live.log 行首时间戳：完整 [2026-08-13 14:10] 或当天 [14:05]
var liveLogTimeRe = regexp.MustCompile(`\[(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}|\d{2}:\d{2})\]`)

// parseAgentLiveStatus 从 live.log 尾部解析 agent 当前状态/任务/最后活动
// live.log 时间戳两种：完整 [2026-08-09 22:58] 和当天短格式 [22:59]
// 短格式继承最近一条完整时间戳行的日期（08-26 实测修复：不能当今天）
func parseAgentLiveStatus(logPath string) (status, task, lastActive string) {
	data, err := os.ReadFile(logPath)
	if err != nil {
		return "未知", "无活动记录", ""
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	now := time.Now()
	lastTS := time.Time{}
	lastTask := ""
	// 从尾部往前找最后一条带时间戳的行动行
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		m := liveLogTimeRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		var ts time.Time
		if len(m[1]) > 12 { // 完整日期（含 4 位年）
			ts, _ = time.ParseInLocation("2006-01-02 15:04", m[1], time.Local)
		} else { // 短时间 [22:59]：继承最近一条完整日期行
			hm, _ := time.ParseInLocation("15:04", m[1], time.Local)
			for j := i - 1; j >= 0; j-- {
				m2 := liveLogTimeRe.FindStringSubmatch(strings.TrimSpace(lines[j]))
				if m2 != nil && len(m2[1]) > 12 {
					if d, err := time.ParseInLocation("2006-01-02 15:04", m2[1], time.Local); err == nil && !d.IsZero() {
						ts = time.Date(d.Year(), d.Month(), d.Day(), hm.Hour(), hm.Minute(), 0, 0, time.Local)
						break
					}
				}
			}
		}
		if ts.IsZero() {
			continue
		}
		lastTS = ts
		// 行动内容：去掉时间戳前缀
		task = strings.TrimSpace(liveLogTimeRe.ReplaceAllString(line, ""))
		// 跳过纯状态行（💭 想法 / 👭 无新消息），继续找真正的行动
		if task != "" && !strings.HasPrefix(task, "💭") && !strings.HasPrefix(task, "👭") && !strings.HasPrefix(task, "在这里待着") {
			lastTask = task
		}
		if lastTask != "" {
			break
		}
	}
	if lastTS.IsZero() {
		return "未知", "无活动记录", ""
	}
	days := int(now.Sub(lastTS).Hours() / 24)
	switch {
	case days <= 0:
		status = "工作中"
	case days <= 7:
		status = "空闲中"
	default:
		status = fmt.Sprintf("停摆 %d 天", days)
	}
	if lastTask == "" {
		lastTask = "最近无行动"
	}
	lastActive = lastTS.Format("01-02 15:04")
	return status, lastTask, lastActive
}

// CollabRef 协作引用（真实证据：这个 agent 引用了哪个同事的什么产出）
type CollabRef struct {
	Agent  string `json:"agent"`  // 被引用的同事（designer-04）
	Source string `json:"source"` // 引用出现在哪（001-智能创作台 / 设计-2026-08-09-50.md）
	Text   string `json:"text"`   // 引用原文（截断 60 字）
}

// collectCollabRefs 扫描某 agent 的项目需求计划与产出，提取对同事产出的真实引用
func collectCollabRefs(home string) []CollabRef {
	var refs []CollabRef
	re := regexp.MustCompile(`(designer|writer|researcher|coder|promoter|publisher|ceo)-\d{1,3}`)
	self := filepath.Base(home)
	seen := map[string]bool{}
	// 项目需求计划（最有力的协作证据：立项时读了同事的设计稿/文档）
	projDir := filepath.Join(home, "projects")
	if entries, err := os.ReadDir(projDir); err == nil {
		for _, p := range entries {
			if !p.IsDir() {
				continue
			}
			planDir := filepath.Join(projDir, p.Name())
			planFiles, _ := os.ReadDir(planDir)
			for _, f := range planFiles {
				if f.IsDir() || !strings.HasPrefix(f.Name(), "00-需求计划") {
					continue
				}
				data, err := os.ReadFile(filepath.Join(planDir, f.Name()))
				if err != nil {
					continue
				}
				refs = scanRefsInText(string(data), re, self, p.Name(), refs, seen)
			}
		}
	}
	// 产出文件（2026-08-09 扩展：设计稿/文章/PPT 里也引用同事——不止 coder 有接力证据）
	outDir := filepath.Join(home, "outputs")
	if outEntries, err := os.ReadDir(outDir); err == nil {
		for _, o := range outEntries {
			if o.IsDir() || !strings.HasSuffix(o.Name(), ".md") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(outDir, o.Name()))
			if err != nil {
				continue
			}
			refs = scanRefsInText(string(data), re, self, o.Name(), refs, seen)
		}
	}
	if len(refs) > 4 {
		refs = refs[:4]
	}
	return refs
}

// scanRefsInText 在文本里提取对其他 agent 的引用
func scanRefsInText(text string, re *regexp.Regexp, self, source string, refs []CollabRef, seen map[string]bool) []CollabRef {
	for _, line := range strings.Split(text, "\n") {
		for _, m := range re.FindAllString(line, -1) {
			if m == self {
				continue
			}
			key := m + "|" + source
			if seen[key] {
				continue
			}
			seen[key] = true
			refs = append(refs, CollabRef{
				Agent:  m,
				Source: source,
				Text:   truncateStr(line, 60),
			})
		}
	}
	return refs
}

func truncateStr(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// companyDir 公司目录
func companyDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "rescene_data", "company")
}

// companyPersonality 读取某 agent 的人设背景（性别/年龄/童年），无则按家目录 hash 兜底
func companyPersonality(name string) (gender string, age int, childhood string) {
	home := filepath.Join(companyDir(), name)
	path := filepath.Join(home, "personality.json")
	if data, err := os.ReadFile(path); err == nil {
		var p struct {
			CreatedAt string `json:"created_at"`
			Gender    string `json:"gender"`
			Childhood string `json:"childhood"`
		}
		if json.Unmarshal(data, &p) == nil {
			gender = p.Gender
			childhood = p.Childhood
			if t, err := time.Parse("2006-01-02", p.CreatedAt); err == nil {
				age = int(time.Since(t).Hours() / 24 / 365)
				if age < 1 {
					age = 24
				}
			}
			// 旧文件可能没有 gender/childhood 字段，有则返回
			if gender != "" && childhood != "" {
				return
			}
		}
	}
	// 兜底：按家目录 hash 生成（与 agent-os loadPersonality 同算法）
	h := 0
	for _, c := range []rune(name) {
		h = h*31 + int(c)
	}
	if h < 0 {
		h = -h
	}
	gender = []string{"男", "女"}[h%2]
	age = 22 + h%10
	places := []string{"海边小镇", "山间村庄", "繁华都市", "宁静田园", "科技园区", "古城巷弄"}
	hobbies := []string{"编程", "画画", "读书", "观察星空", "研究机器", "写作"}
	childhood = fmt.Sprintf("在%s长大，从小喜欢%s。", places[h%6], hobbies[(h/7)%6])
	return
}

// HandleCompanyFile GET /api/company/file?agent=researcher-02&name=xxx.md
func HandleCompanyFile(c *gin.Context) {
	agent := c.Query("agent")
	name := c.Query("name")
	if agent == "" || name == "" || strings.Contains(agent, "..") || strings.ContainsAny(agent, `/\\`) || strings.Contains(name, "..") || filepath.IsAbs(name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var path string
	clean := filepath.Clean(filepath.FromSlash(name))
	if strings.HasPrefix(filepath.ToSlash(clean), "project/") {
		rel := strings.TrimPrefix(filepath.ToSlash(clean), "project/")
		if rel == "" || strings.HasPrefix(rel, "/") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
			return
		}
		path = filepath.Join(companyDir(), agent, "projects", filepath.FromSlash(rel))
	} else {
		if strings.ContainsAny(clean, `/\\`) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
			return
		}
		path = filepath.Join(companyDir(), agent, "outputs", clean)
	}
	info, err := os.Stat(path)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在: " + name})
		return
	}
	if info.IsDir() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "目录不能作为产物预览"})
		return
	}
	ext := strings.ToLower(filepath.Ext(path))
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if c.Query("raw") == "1" {
		c.Header("Content-Type", contentType)
		c.Header("Content-Disposition", `inline; filename="`+strings.ReplaceAll(filepath.Base(path), `"`, "")+`"`)
		http.ServeFile(c.Writer, c.Request, path)
		return
	}
	kind := "binary"
	switch ext {
	case ".mp4", ".webm", ".mov":
		kind = "video"
	case ".xlsx", ".xls", ".csv", ".tsv":
		kind = "spreadsheet"
	case ".html", ".htm":
		kind = "html"
	case ".pptx":
		kind = "pptx"
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg":
		kind = "image"
	case ".md", ".txt", ".json", ".js", ".ts", ".py", ".go", ".java", ".css", ".srt", ".vtt", ".receipt", ".har":
		kind = "text"
	}
	result := gin.H{"name": name, "kind": kind, "mime": contentType, "size": info.Size()}
	if kind == "text" || kind == "html" {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "读取产物失败"})
			return
		}
		content := string(data)
		if utf8.RuneCountInString(content) > 120000 {
			content = string([]rune(content)[:120000]) + "\n…"
		}
		result["content"] = content
	}
	c.JSON(http.StatusOK, result)
}

// HandleCompanyAgents GET /api/company/agents
func HandleCompanyAgents(c *gin.Context) {
	dir := companyDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"agents": []companyAgentInfo{}})
		return
	}
	var agents []companyAgentInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info := companyAgentInfo{
			Name: e.Name(),
			Home: filepath.Join(dir, e.Name()),
		}
		// 人设背景（每个 agent 都有年龄/性别/童年故事）
		info.Gender, info.Age, info.Childhood = companyPersonality(e.Name())
		// 角色：从名子解析（writer-01 → writer）
		parts := strings.SplitN(e.Name(), "-", 2)
		if len(parts) > 0 {
			info.Role = parts[0]
		}
		// live.log 尾部
		logPath := filepath.Join(dir, e.Name(), "live.log")
		if data, err := os.ReadFile(logPath); err == nil {
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			if len(lines) > 3 {
				info.RecentLog = strings.Join(lines[len(lines)-3:], "\n")
			} else {
				info.RecentLog = string(data)
			}
		}
		// 开罗式实时状态：正在干什么 + 最后活动时间
		info.Status, info.Task, info.LastActive = parseAgentLiveStatus(logPath)
		// 产出数 + 文件名列表
		outputDir := filepath.Join(dir, e.Name(), "outputs")
		if outEntries, err := os.ReadDir(outputDir); err == nil {
			for _, o := range outEntries {
				if !o.IsDir() && !strings.HasPrefix(o.Name(), "README") {
					info.Outputs++
					info.Files = append(info.Files, o.Name())
				}
			}
			if len(info.Files) > 8 {
				info.Files = info.Files[len(info.Files)-8:]
			}
		}
		// 技能数
		skillDir := filepath.Join(dir, e.Name(), "skills")
		if skillEntries, err := os.ReadDir(skillDir); err == nil {
			for _, s := range skillEntries {
				if !s.IsDir() && strings.HasSuffix(s.Name(), ".json") {
					info.Skills++
				}
			}
		}
		// 协作引用（真实接力证据）
		info.CollabRefs = collectCollabRefs(filepath.Join(dir, e.Name()))
		agents = append(agents, info)
	}
	c.JSON(http.StatusOK, gin.H{"agents": agents})
}

// HandleCompanyAgent GET /api/company/agent?name=writer-01
func HandleCompanyAgent(c *gin.Context) {
	name := c.Query("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name 参数必填"})
		return
	}
	home := filepath.Join(companyDir(), name)
	if _, err := os.Stat(home); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent 不存在"})
		return
	}
	info := companyAgentInfo{Name: name, Home: home}
	parts := strings.SplitN(name, "-", 2)
	if len(parts) > 0 {
		info.Role = parts[0]
	}
	// 活动日志
	logPath := filepath.Join(home, "live.log")
	if data, err := os.ReadFile(logPath); err == nil {
		info.RecentLog = string(data)
	}
	// 产出
	outputDir := filepath.Join(home, "outputs")
	if outEntries, err := os.ReadDir(outputDir); err == nil {
		for _, o := range outEntries {
			if !o.IsDir() {
				info.Outputs++
			}
		}
	}
	// 技能
	skillDir := filepath.Join(home, "skills")
	if skillEntries, err := os.ReadDir(skillDir); err == nil {
		for _, s := range skillEntries {
			if !s.IsDir() && strings.HasSuffix(s.Name(), ".json") {
				info.Skills++
			}
		}
	}
	_ = time.Now()
	_ = json.Valid
	c.JSON(http.StatusOK, info)
}

// HandleCompanyOSStats GET /api/company/os-stats — Agent OS 总控统计（2026-08-08）
func HandleCompanyOSStats(c *gin.Context) {
	dir := companyDir()
	entries, _ := os.ReadDir(dir)
	total := len(entries)
	working := 0
	outputs := 0
	skills := 0
	var recentLogs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		home := filepath.Join(dir, e.Name())
		// 最近活动
		logPath := filepath.Join(home, "live.log")
		if data, err := os.ReadFile(logPath); err == nil {
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			if len(lines) > 0 {
				last := strings.TrimSpace(lines[len(lines)-1])
				if !strings.Contains(last, "失败") && !strings.Contains(last, "429") && !strings.Contains(last, "熔断") {
					working++
				}
				recentLogs = append(recentLogs, last)
			}
		}
		// 产出数
		outDir := filepath.Join(home, "outputs")
		if outEntries, err := os.ReadDir(outDir); err == nil {
			for _, o := range outEntries {
				if !o.IsDir() && !strings.HasPrefix(o.Name(), "README") {
					outputs++
				}
			}
		}
		// 技能数
		skillDir := filepath.Join(home, "skills")
		if skillEntries, err := os.ReadDir(skillDir); err == nil {
			for _, s := range skillEntries {
				if !s.IsDir() && strings.HasSuffix(s.Name(), ".json") {
					skills++
				}
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"totalAgents":  total,
		"workingCount": working,
		"totalOutputs": outputs,
		"totalSkills":  skills,
		"uptime":       "24H 自转",
		"version":      "Rescene Agent OS v" + strings.TrimPrefix(AppVersion, "v"),
		"modelPool":    "免费模型 " + fmt.Sprintf("%d 个", len(entries)),
		"valuation":    fmt.Sprintf("￥%d", outputs*5000+skills*3000+total*1000),
	})
}

// ===== 进化统计（宝可梦量化，mirror agent-os/refine.go computeEvolutionStats）=====

// evolveStats 六维有效参数（0-100），每条都有磁盘证据，不是虚构的 HP/攻/防
type evolveStats struct {
	XP    int    `json:"xp"`
	Level int    `json:"level"`
	Stage string `json:"stage"`

	Output        int `json:"output"`   // 产出力：交付物数
	Skill         int `json:"skill"`    // 技能广度：技能库规模
	Collaboration int `json:"collab"`   // 协作度：真实交叉引用
	Memory        int `json:"memory"`   // 记忆沉淀：记忆块 + 进化沉淀
	Success       int `json:"success"`  // 成功率：live.log 成功动作占比
	Intimacy      int `json:"intimacy"` // 亲密度：memorydir 亲密等级 Lv

	// 原始计数（详情/调试）
	Refines  int `json:"refines"`
	Skills   int `json:"skills"`
	Memories int `json:"memories"`
	Outputs  int `json:"outputs"`

	AxisMax int `json:"axis_max"` // 雷达轴上限 100
}

func normScore(v, scale int) int {
	s := v * scale
	if s > 100 {
		return 100
	}
	if s < 0 {
		return 0
	}
	return s
}

func clampPct(v int) int {
	if v > 100 {
		return 100
	}
	if v < 0 {
		return 0
	}
	return v
}

// readIntimacyGlobal 读全局亲密度 Lv（~/rescene_data/memory/intimacy.md，QQ 曲线）
func readIntimacyGlobal() (value, level int) {
	hd, err := os.UserHomeDir()
	if err != nil {
		return 0, 0
	}
	data, err := os.ReadFile(filepath.Join(hd, "rescene_data", "memory", "intimacy.md"))
	if err != nil {
		return 0, 0
	}
	s := strings.TrimSpace(string(data))
	if idx := strings.Index(s, ":"); idx != -1 {
		s = strings.TrimSpace(s[idx+1:])
	}
	fmt.Sscanf(s, "%d", &value)
	if value > 0 {
		level = int((1 + mathSqrt(1+8*float64(value)/100)) / 2)
	}
	return value, level
}

// computeEvolveStats 与 agent-os 同一套：六维有效参数 + XP/等级（均真实数据）
func computeEvolveStats(home string) evolveStats {
	st := evolveStats{AxisMax: 100}
	// refine 次数
	if data, err := os.ReadFile(filepath.Join(home, "refine", "refinements.jsonl")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.TrimSpace(line) != "" {
				st.Refines++
			}
		}
	}
	// 技能数
	if ents, err := os.ReadDir(filepath.Join(home, "skills")); err == nil {
		for _, e := range ents {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
				st.Skills++
			}
		}
	}
	// 记忆块数
	if mem, err := os.ReadFile(filepath.Join(home, "memory.md")); err == nil {
		st.Memories = strings.Count(string(mem), "## [") // 块标记计数（含文件开头的第一块）
	}
	// 产出数
	if ents, err := os.ReadDir(filepath.Join(home, "outputs")); err == nil {
		st.Outputs = len(ents)
	}
	// 成功率：live.log 里 ✅/完成 vs ⚠️/失败 的真占比
	var ok, bad int
	if data, err := os.ReadFile(filepath.Join(home, "live.log")); err == nil {
		for _, l := range strings.Split(string(data), "\n") {
			if strings.Contains(l, "✅") || strings.Contains(l, "完成") {
				ok++
			}
			if strings.Contains(l, "⚠️") || strings.Contains(l, "❌") || strings.Contains(l, "失败") || strings.Contains(l, "未") {
				bad++
			}
		}
	}
	success := 0
	if ok+bad > 0 {
		success = int(float64(ok) / float64(ok+bad) * 100)
	}
	// 协作：真实交叉引用（复用面板已展示的 collectCollabRefs）
	collab := len(collectCollabRefs(home))
	// 亲密度：全局 memorydir
	_, intimacyLv := readIntimacyGlobal()

	// 六维（0-100，可比）
	st.Output = normScore(st.Outputs, 10)
	st.Skill = normScore(st.Skills, 10)
	st.Collaboration = normScore(collab, 20)
	st.Memory = normScore(st.Memories+st.Refines, 10)
	st.Success = clampPct(success)
	st.Intimacy = normScore(intimacyLv, 15)

	// XP + 等级（QQ 宠物式曲线，越高越难升）
	st.XP = st.Refines*30 + st.Skills*20 + st.Memories*10 + st.Outputs*5
	if st.XP > 0 {
		st.Level = int((1 + mathSqrt(1+8*float64(st.XP)/100)) / 2)
	}
	// 进化阶段（赛博成长链：静默电流→数据尘埃→神经漫游者→矩阵幽灵→冬寂）
	switch {
	case st.Skills >= 20 && st.Memories >= 15:
		st.Stage = "Lv.5 冬寂"
	case st.Skills >= 10 && st.Memories >= 8:
		st.Stage = "Lv.4 矩阵幽灵"
	case st.Skills >= 5 && st.Memories >= 4:
		st.Stage = "Lv.3 神经漫游者"
	case st.Skills >= 2:
		st.Stage = "Lv.2 数据尘埃"
	default:
		st.Stage = "Lv.1 静默电流"
	}
	return st
}

func mathSqrt(v float64) float64 {
	if v <= 0 {
		return 0
	}
	x := v
	for i := 0; i < 8; i++ {
		x = (x + v/x) / 2
	}
	return x
}

// HandleCompanyEvolve GET /api/company/evolve?agent=writer-01 — 单个 agent 的宝可梦进化统计
func HandleCompanyEvolve(c *gin.Context) {
	name := c.Query("agent")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "agent 参数必填"})
		return
	}
	home := filepath.Join(companyDir(), name)
	if _, err := os.Stat(home); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent 不存在"})
		return
	}
	c.JSON(http.StatusOK, computeEvolveStats(home))
}

// ===== 公司标签系统（调研方向标记 + 热门标签）=====

// CompanyTag 单个标签
type CompanyTag struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
	UsedCount int    `json:"usedCount"`
}

// tagFilePath 标签持久化路径
func tagFilePath() string {
	return filepath.Join(companyDir(), "tags.json")
}

// loadTags 读磁盘标签
func loadTags() []CompanyTag {
	p := tagFilePath()
	data, err := os.ReadFile(p)
	if err != nil {
		// 文件不存在/损坏，返回空
		_ = os.MkdirAll(filepath.Dir(p), 0755)
		_ = os.WriteFile(p, []byte("[]"), 0644)
		return nil
	}
	var tags []CompanyTag
	if err := json.Unmarshal(data, &tags); err != nil {
		return nil
	}
	return tags
}

// saveTags 写磁盘标签
func saveTags(tags []CompanyTag) {
	data, _ := json.MarshalIndent(tags, "", "  ")
	_ = os.WriteFile(tagFilePath(), data, 0644)
}

// HandleCompanyTags GET /api/company/tags — 获取用户所有标签
func HandleCompanyTags(c *gin.Context) {
	tags := loadTags()
	if tags == nil {
		tags = []CompanyTag{}
	}
	c.JSON(http.StatusOK, gin.H{"tags": tags})
}

// HandleCompanyAddTag POST /api/company/tags — 添加标签
func HandleCompanyAddTag(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "标签名必填"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "标签名不能为空"})
		return
	}

	tags := loadTags()
	if tags == nil {
		tags = []CompanyTag{}
	}

	// 检查是否已存在同名标签
	now := time.Now().Format(time.RFC3339)
	for i, t := range tags {
		if strings.EqualFold(t.Name, name) {
			tags[i].UsedCount++
			tags[i].CreatedAt = now
			saveTags(tags)
			c.JSON(http.StatusOK, gin.H{"tag": tags[i], "ok": true})
			return
		}
	}

	// 新建标签
	id := fmt.Sprintf("tag_%d", time.Now().UnixMilli())
	tag := CompanyTag{
		ID:        id,
		Name:      name,
		CreatedAt: now,
		UsedCount: 1,
	}
	tags = append(tags, tag)
	saveTags(tags)
	c.JSON(http.StatusOK, gin.H{"tag": tag, "ok": true})
}

// HandleCompanyDeleteTag DELETE /api/company/tags/:id — 删除标签
func HandleCompanyDeleteTag(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id 必填"})
		return
	}
	tags := loadTags()
	if tags == nil {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	filtered := make([]CompanyTag, 0, len(tags))
	for _, t := range tags {
		if t.ID != id {
			filtered = append(filtered, t)
		}
	}
	saveTags(filtered)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// HandleCompanyHotTags GET /api/company/tags/hot — 热门标签（云端聚合）
func HandleCompanyHotTags(c *gin.Context) {
	// 优先从本地 tags 统计热门（按 usedCount 排序）
	tags := loadTags()
	if tags == nil {
		c.JSON(http.StatusOK, gin.H{"hot": []CompanyTag{}})
		return
	}
	// 按 usedCount 降序
	sort.Slice(tags, func(i, j int) bool {
		return tags[i].UsedCount > tags[j].UsedCount
	})
	// 取前 20
	if len(tags) > 20 {
		tags = tags[:20]
	}
	c.JSON(http.StatusOK, gin.H{"hot": tags})
}

// directiveFilePath 用户自定义指令持久化路径（agent-os 立项时读同一文件）
func directiveFilePath() string {
	return filepath.Join(companyDir(), "directive.json")
}

type companyDirectiveRun struct {
	Directive string `json:"directive"`
	Model     string `json:"model,omitempty"`
	Project   string `json:"project,omitempty"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
	UpdatedAt string `json:"updatedAt"`
}

var companyDirectiveRunMu sync.Mutex

func directiveRunFilePath() string { return filepath.Join(companyDir(), "directive-run.json") }

func loadCompanyDirectiveRun() companyDirectiveRun {
	data, err := os.ReadFile(directiveRunFilePath())
	if err != nil {
		return companyDirectiveRun{}
	}
	var state companyDirectiveRun
	_ = json.Unmarshal(data, &state)
	return state
}

func saveCompanyDirectiveRun(state companyDirectiveRun) {
	state.UpdatedAt = time.Now().Format(time.RFC3339)
	data, _ := json.MarshalIndent(state, "", "  ")
	_ = os.MkdirAll(companyDir(), 0o755)
	_ = os.WriteFile(directiveRunFilePath(), data, 0o644)
}

func findDirectiveDeliveryExecutable() (string, string) {
	if configured := strings.TrimSpace(os.Getenv("RESCENE_AGENT_OS_PATH")); configured != "" {
		if _, err := os.Stat(configured); err == nil {
			return configured, filepath.Dir(configured)
		}
	}
	var roots []string
	if wd, err := os.Getwd(); err == nil {
		for dir := wd; ; dir = filepath.Dir(dir) {
			roots = append(roots, dir)
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
		}
	}
	if exe, err := os.Executable(); err == nil {
		roots = append(roots, filepath.Dir(exe))
	}
	for _, root := range roots {
		for _, name := range []string{"rescene-demo.exe", "rescene.exe", "rescene-demo", "rescene"} {
			for _, candidate := range []string{filepath.Join(root, "agent-os", name), filepath.Join(root, name)} {
				if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
					return candidate, filepath.Dir(candidate)
				}
			}
		}
	}
	return "", ""
}

func startCompanyDirectiveDelivery(directive, model string) companyDirectiveRun {
	state := companyDirectiveRun{Directive: directive, Model: model, Status: "queued"}
	companyDirectiveRunMu.Lock()
	saveCompanyDirectiveRun(state)
	companyDirectiveRunMu.Unlock()
	if strings.TrimSpace(os.Getenv("RESCENE_DIRECTIVE_RUNNER_DISABLED")) == "1" {
		return state
	}
	go func() {
		exe, dir := findDirectiveDeliveryExecutable()
		if exe == "" {
			companyDirectiveRunMu.Lock()
			saveCompanyDirectiveRun(companyDirectiveRun{Directive: directive, Model: model, Status: "failed", Error: "找不到公司交付引擎，请重新安装或构建 Agent OS"})
			companyDirectiveRunMu.Unlock()
			return
		}
		companyDirectiveRunMu.Lock()
		saveCompanyDirectiveRun(companyDirectiveRun{Directive: directive, Model: model, Status: "running"})
		companyDirectiveRunMu.Unlock()
		args := []string{"directive-delivery"}
		if model != "" {
			args = append(args, "--model", model)
		}
		args = append(args, "--", directive)
		cmd := hiddenCommand(exe, args...)
		cmd.Dir = dir
		// Development builds can reuse a colocated ffmpeg without requiring the
		// user to open a terminal and edit PATH before clicking the button.
		env := os.Environ()
		for _, ffmpegDir := range []string{
			filepath.Join(filepath.Dir(filepath.Dir(dir)), "nachobot-ref", "NachoBot", "plugins", "bilibili_video_sender_plugin", "ffmpeg", "bin"),
			filepath.Join(filepath.Dir(dir), "ffmpeg", "bin"),
		} {
			if _, err := os.Stat(filepath.Join(ffmpegDir, "ffmpeg.exe")); err == nil {
				env = append(env, "PATH="+ffmpegDir+string(os.PathListSeparator)+os.Getenv("PATH"))
				break
			}
		}
		cmd.Env = env
		output, err := cmd.CombinedOutput()
		text := strings.TrimSpace(string(output))
		finished := companyDirectiveRun{Directive: directive, Model: model, Status: "completed"}
		if match := regexp.MustCompile(`(?m)DEMO_PROJECT=([^\r\n]+)`).FindStringSubmatch(text); len(match) > 1 {
			finished.Project = strings.TrimSpace(match[1])
		}
		if err != nil || finished.Project == "" {
			finished.Status = "failed"
			finished.Error = text
			if finished.Error == "" && err != nil {
				finished.Error = err.Error()
			}
			if len(finished.Error) > 700 {
				finished.Error = finished.Error[len(finished.Error)-700:]
			}
		}
		companyDirectiveRunMu.Lock()
		saveCompanyDirectiveRun(finished)
		companyDirectiveRunMu.Unlock()
	}()
	return state
}

// HandleCompanyDirective GET /api/company/directive — 获取当前用户指令（考题/项目目标）
func HandleCompanyDirective(c *gin.Context) {
	text, task, model := loadCompanyDirective()
	c.JSON(http.StatusOK, gin.H{"ok": true, "directive": text, "task": task, "model": model, "run": loadCompanyDirectiveRun()})
}

// HandleCompanySaveDirective PUT /api/company/directive — 下达/更新指令；空文本=清除
func HandleCompanySaveDirective(c *gin.Context) {
	var req struct {
		Directive string `json:"directive"`
		Task      string `json:"task"`
		Model     string `json:"model"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	req.Directive = strings.TrimSpace(req.Directive)
	req.Task = strings.TrimSpace(req.Task)
	req.Model = strings.TrimSpace(req.Model)
	data, _ := json.MarshalIndent(map[string]string{
		"directive": req.Directive,
		"task":      req.Task,
		"model":     req.Model,
		"updatedAt": time.Now().Format(time.RFC3339),
	}, "", "  ")
	_ = os.MkdirAll(filepath.Dir(directiveFilePath()), 0755)
	if err := os.WriteFile(directiveFilePath(), data, 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	if req.Directive == "" {
		companyDirectiveRunMu.Lock()
		saveCompanyDirectiveRun(companyDirectiveRun{Status: "idle"})
		companyDirectiveRunMu.Unlock()
		c.JSON(http.StatusOK, gin.H{"ok": true, "directive": req.Directive, "task": req.Task, "model": req.Model, "run": loadCompanyDirectiveRun()})
		return
	}
	run := startCompanyDirectiveDelivery(req.Directive, req.Model)
	c.JSON(http.StatusAccepted, gin.H{"ok": true, "directive": req.Directive, "task": req.Task, "model": req.Model, "run": run})
}

// loadCompanyDirective 读当前指令（供 GET 接口；agent-os 侧用 companyDirective()）
func loadCompanyDirective() (directive, task, model string) {
	data, err := os.ReadFile(directiveFilePath())
	if err != nil {
		return "", "", ""
	}
	var d struct {
		Directive string `json:"directive"`
		Task      string `json:"task"`
		Model     string `json:"model"`
	}
	if json.Unmarshal(data, &d) != nil {
		return "", "", ""
	}
	return strings.TrimSpace(d.Directive), strings.TrimSpace(d.Task), strings.TrimSpace(d.Model)
}

// ===== 真实世界评分评奖（2026-08-26 重构：每一个评分/评奖都来自真实用户）=====

// realReviewFile 一条真实用户评分
type realReviewFile struct {
	Nickname  string `json:"nickname"`   // 真实用户昵称
	Score     int    `json:"score"`      // 1-10
	Comment   string `json:"comment"`    // 玩家向评论（≤120字）
	CreatedAt string `json:"created_at"` // 提交时间
}

// realReviewsFile 项目真实评分存储（落盘 reviews-real.json，追加式）
type realReviewsFile struct {
	Project string           `json:"project"`
	Reviews []realReviewFile `json:"reviews"`
}

// reviewAward 评奖结果（真实用户评分驱动）
type reviewAward struct {
	Title string `json:"title"` // 🏆 神作 / ⭐ 好评 / 🌱 新秀
	Label string `json:"label"` // 得奖理由（真实用户数 + 均分）
}

// reviewAwardFor 根据真实评分算奖项：分数全部来自真实用户
func reviewAwardFor(avg float64, count int) *reviewAward {
	if count == 0 {
		return nil
	}
	switch {
	case count >= 2 && avg >= 8.5:
		return &reviewAward{Title: "🏆 神作", Label: fmt.Sprintf("%d 位真实用户 · 平均 %.1f 分", count, avg)}
	case count >= 1 && avg >= 7:
		return &reviewAward{Title: "⭐ 好评", Label: fmt.Sprintf("%d 位真实用户 · 平均 %.1f 分", count, avg)}
	default:
		return &reviewAward{Title: "🌱 新秀", Label: fmt.Sprintf("%d 位真实用户 · 平均 %.1f 分", count, avg)}
	}
}

// reviewsRealPath 项目目录 → 真实评分文件
func reviewsRealPath(projDir string) string {
	return filepath.Join(projDir, "reviews-real.json")
}

// loadRealReviews 读项目真实评分（文件不存在返回空）
func loadRealReviews(projDir, project string) realReviewsFile {
	rb := realReviewsFile{Project: project}
	data, err := os.ReadFile(reviewsRealPath(projDir))
	if err != nil {
		return rb
	}
	json.Unmarshal(data, &rb)
	if rb.Project == "" {
		rb.Project = project
	}
	return rb
}

// findProjectDir 按项目名（目录名）定位真实项目目录，防路径穿越
func findProjectDir(project string) (string, bool) {
	if project == "" || strings.ContainsAny(project, `/\`) {
		return "", false
	}
	companyRoot := companyDir()
	entries, err := os.ReadDir(companyRoot)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "coder-") {
			continue
		}
		projDir := filepath.Join(companyRoot, e.Name(), "projects", project)
		if st, err := os.Stat(projDir); err == nil && st.IsDir() {
			return projDir, true
		}
	}
	return "", false
}

// reviewSubmitReq POST /api/company/reviews 请求体
type reviewSubmitReq struct {
	Project  string `json:"project"`  // 项目名（目录名）
	Nickname string `json:"nickname"` // 真实用户昵称
	Score    int    `json:"score"`    // 1-10
	Comment  string `json:"comment"`  // ≤120字
}

// HandleCompanyReviewSubmit POST /api/company/reviews — 真实用户提交评分
// 校验 → 写入 reviews-real.json（同昵称同项目覆盖旧评分）→ 返回聚合+评奖
func HandleCompanyReviewSubmit(c *gin.Context) {
	var req reviewSubmitReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	req.Project = strings.TrimSpace(req.Project)
	req.Nickname = strings.TrimSpace(req.Nickname)
	req.Comment = strings.TrimSpace(req.Comment)
	if req.Project == "" || req.Nickname == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "项目名和用户昵称不能为空"})
		return
	}
	if req.Score < 1 || req.Score > 10 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "评分需在 1-10 之间"})
		return
	}
	if len([]rune(req.Comment)) > 120 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "评论最多 120 字"})
		return
	}
	projDir, ok := findProjectDir(req.Project)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "项目不存在"})
		return
	}

	rb := loadRealReviews(projDir, req.Project)
	now := time.Now().Format("2006-01-02 15:04")
	replaced := false
	for i := range rb.Reviews {
		if rb.Reviews[i].Nickname == req.Nickname {
			rb.Reviews[i].Score = req.Score
			rb.Reviews[i].Comment = req.Comment
			rb.Reviews[i].CreatedAt = now
			replaced = true
			break
		}
	}
	if !replaced {
		rb.Reviews = append(rb.Reviews, realReviewFile{
			Nickname: req.Nickname, Score: req.Score, Comment: req.Comment, CreatedAt: now,
		})
	}
	data, _ := json.MarshalIndent(rb, "", "  ")
	if err := os.WriteFile(reviewsRealPath(projDir), data, 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "评分落盘失败: " + err.Error()})
		return
	}

	// 资金闭环：真实评分上涨 → 销量入账（开罗经济）
	_ = syncProductRevenue(projDir, req.Project)

	// 返回该项目的聚合 + 评奖（真实用户驱动）
	avg := 0.0
	for _, r := range rb.Reviews {
		avg += float64(r.Score)
	}
	avg /= float64(len(rb.Reviews))
	award := reviewAwardFor(avg, len(rb.Reviews))
	c.JSON(http.StatusOK, gin.H{
		"project":   req.Project,
		"avg_score": round1(avg),
		"count":     len(rb.Reviews),
		"award":     award,
		"users":     rb.Reviews,
	})
}

// round1 保留一位小数
func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

// HandleCompanyReviews GET /api/company/reviews — 真实用户评分/评奖列表
// 扫描公司各 coder 项目目录的 reviews-real.json，只返回有真实评分的产品
func HandleCompanyReviews(c *gin.Context) {
	companyRoot := companyDir()
	type userReviewView struct {
		Nickname  string `json:"nickname"`
		Score     int    `json:"score"`
		Comment   string `json:"comment"`
		CreatedAt string `json:"created_at"`
	}
	type reviewsPayload struct {
		Project  string           `json:"project"`
		Agent    string           `json:"agent"`
		AvgScore float64          `json:"avg_score"`
		Count    int              `json:"count"`
		Award    *reviewAward     `json:"award"`
		Users    []userReviewView `json:"users"`
	}
	var all []reviewsPayload
	entries, err := os.ReadDir(companyRoot)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"reviews": []reviewsPayload{}})
		return
	}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "coder-") {
			continue
		}
		projDir := filepath.Join(companyRoot, e.Name(), "projects")
		projs, err := os.ReadDir(projDir)
		if err != nil {
			continue
		}
		for _, p := range projs {
			if !p.IsDir() {
				continue
			}
			rb := loadRealReviews(filepath.Join(projDir, p.Name()), p.Name())
			if len(rb.Reviews) == 0 {
				continue // 只展示有真实用户评分的产品
			}
			avg := 0.0
			for _, r := range rb.Reviews {
				avg += float64(r.Score)
			}
			avg /= float64(len(rb.Reviews))
			users := make([]userReviewView, 0, len(rb.Reviews))
			for _, r := range rb.Reviews {
				users = append(users, userReviewView{
					Nickname: r.Nickname, Score: r.Score, Comment: r.Comment, CreatedAt: r.CreatedAt,
				})
			}
			all = append(all, reviewsPayload{
				Project: p.Name(), Agent: e.Name(),
				AvgScore: round1(avg), Count: len(rb.Reviews),
				Award: reviewAwardFor(avg, len(rb.Reviews)), Users: users,
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"reviews": all})
}
