package handler

// company_handler.go — 公司管理面板 API（GUI 看百人公司运作）
//   GET /api/company/agents — 所有 agent 列表（含最近活动/产出数）
//   GET /api/company/agent?name=writer-01 — 单个 agent 详情

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
		"version":      "Rescene Agent OS v0.1.0",
		"modelPool":    "免费模型 " + fmt.Sprintf("%d 个", len(entries)),
		"valuation":    fmt.Sprintf("￥%d", outputs*5000+skills*3000+total*1000),
	})
}
