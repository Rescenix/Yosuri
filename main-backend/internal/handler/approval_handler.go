package handler

// approval_handler.go — 审批工作台（2026-08-08）
// 用户只审批，不聊天：agent 全自动跑项目，产出停在审批门前，用户点「批准发布」或「退回迭代」。
//
//   GET  /api/company/approvals          — 待审批队列（各 agent 最近产出物）
//   POST /api/company/approve            — 审批动作 {agent, file, decision: approve|reject, feedback}
//   GET  /api/company/approval-history   — 审批历史

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

var projectStageOrder = []string{"meeting", "research", "data", "requirements", "ui", "docs", "code", "runnable", "ppt", "pv", "promotion"}

type projectDeliveryEvidence struct {
	Stage        string `json:"stage"`
	ProducerRole string `json:"producerRole"`
	File         string `json:"file"`
	Kind         string `json:"kind"`
	SHA256       string `json:"sha256"`
	Verification string `json:"verification"`
}

type projectDeliveryGate struct {
	Project     string                    `json:"project,omitempty"`
	Status      string                    `json:"status"`
	GeneratedAt string                    `json:"generatedAt,omitempty"`
	Evidence    []projectDeliveryEvidence `json:"evidence"`
	Missing     []string                  `json:"missing,omitempty"` // 尽力而为阶段缺省（如 pv 视频）时记录，门禁据此放宽
}

func zipContains(content []byte, required string) bool {
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return false
	}
	for _, file := range reader.File {
		if file.Name == required {
			return true
		}
	}
	return false
}

func validateProjectEvidenceFormat(projectPath string, evidence projectDeliveryEvidence, content []byte) error {
	lower := strings.ToLower(evidence.File)
	switch evidence.Stage {
	case "meeting":
		if !json.Valid(content) {
			return fmt.Errorf("会议证据不是有效 JSON")
		}
	case "data":
		if !strings.HasSuffix(lower, ".xlsx") || !zipContains(content, "xl/workbook.xml") {
			return fmt.Errorf("研究数据不是有效 XLSX")
		}
	case "ui":
		text := strings.ToLower(string(content))
		if !strings.HasSuffix(lower, ".html") || !strings.Contains(text, "<!doctype html") || !strings.Contains(text, "@media") {
			return fmt.Errorf("UI 证据不是响应式 HTML 原型")
		}
	case "code":
		if !strings.Contains(strings.ToLower(string(content)), "<!doctype html") {
			return fmt.Errorf("编码证据不是可打开程序")
		}
	case "runnable":
		text := strings.ToLower(string(content))
		if !strings.Contains(text, "<script>") || (!strings.Contains(text, ".onclick") && !strings.Contains(text, "addeventlistener")) {
			return fmt.Errorf("程序没有可执行交互")
		}
	case "ppt":
		if !strings.HasSuffix(lower, ".pptx") || !zipContains(content, "ppt/presentation.xml") {
			return fmt.Errorf("演示稿不是有效 PPTX")
		}
	case "pv":
		if strings.HasSuffix(lower, ".json") {
			// 无视频引擎时的生图兜底：必须是含 pv-fallback-images 标记的 JSON（06-宣传PV.manifest.json）
			if !json.Valid(content) || !strings.Contains(strings.ToLower(string(content)), "pv-fallback-images") {
				return fmt.Errorf("宣传素材兜底不是有效清单")
			}
		} else if len(content) < 12 || !bytes.Equal(content[4:8], []byte("ftyp")) {
			return fmt.Errorf("宣传片不是有效 MP4 容器")
		}
	case "promotion":
		fields := map[string]string{}
		for _, line := range strings.Split(string(content), "\n") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				fields[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
		entry := filepath.Base(fields["entry"])
		published, err := os.ReadFile(filepath.Join(projectPath, entry))
		if fields["status"] != "published" || fields["channel"] == "" || entry == "." || err != nil {
			return fmt.Errorf("发布回执缺少真实渠道或入口")
		}
		sum := sha256.Sum256(published)
		if !strings.EqualFold(fields["entry_sha256"], fmt.Sprintf("%x", sum[:])) {
			return fmt.Errorf("发布回执无法验证入口文件")
		}
	}
	return nil
}

func verifyProjectDeliveryGate(projectPath string) (projectDeliveryGate, error) {
	data, err := os.ReadFile(filepath.Join(projectPath, "delivery.manifest.json"))
	if err != nil {
		return projectDeliveryGate{}, fmt.Errorf("缺少 delivery.manifest.json")
	}
	var gate projectDeliveryGate
	if err := json.Unmarshal(data, &gate); err != nil || gate.Status != "verified" {
		return gate, fmt.Errorf("交付清单未通过")
	}
	stages := map[string]bool{}
	for _, evidence := range gate.Evidence {
		if evidence.Stage == "" || evidence.File == "" || filepath.Base(evidence.File) != evidence.File {
			return gate, fmt.Errorf("交付清单包含非法证据路径")
		}
		content, readErr := os.ReadFile(filepath.Join(projectPath, evidence.File))
		if readErr != nil || len(content) == 0 {
			return gate, fmt.Errorf("证据缺失: %s", evidence.File)
		}
		sum := sha256.Sum256(content)
		if evidence.SHA256 == "" || !strings.EqualFold(evidence.SHA256, fmt.Sprintf("%x", sum[:])) {
			return gate, fmt.Errorf("证据哈希不匹配: %s", evidence.File)
		}
		if formatErr := validateProjectEvidenceFormat(projectPath, evidence, content); formatErr != nil {
			return gate, fmt.Errorf("%s: %w", evidence.File, formatErr)
		}
		stages[evidence.Stage] = true
	}
	for _, stage := range projectStageOrder {
		if !stages[stage] {
			// 视频（pv）是尽力而为阶段：仅在 manifest 明确标记 Missing 含 "pv" 时放行，
			// 其余阶段一律必须齐全。这让没装 ffmpeg 的用户也能完成其余 10 阶段交付，
			// 同时不放松第三方项目（其 Missing 为空）的严格门禁。
			if stage == "pv" && containsString(gate.Missing, "pv") {
				continue
			}
			return gate, fmt.Errorf("缺少阶段: %s", stage)
		}
	}
	return gate, nil
}

// containsString 判断切片是否包含目标字符串。
func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func projectArtifactStage(agent, name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(name, "会议") || strings.Contains(lower, "meeting"):
		return "meeting"
	case strings.HasSuffix(lower, ".xlsx") || strings.HasSuffix(lower, ".xls") || strings.HasSuffix(lower, ".csv") || strings.HasSuffix(lower, ".tsv"):
		return "data"
	case strings.Contains(name, "调研") || strings.Contains(lower, "research"):
		return "research"
	case strings.Contains(name, "需求") || strings.Contains(name, "计划") || strings.Contains(lower, "requirement") || strings.Contains(lower, "spec"):
		return "requirements"
	case strings.Contains(name, "设计") || strings.Contains(lower, "design") || strings.Contains(lower, "ui"):
		return "ui"
	case strings.Contains(name, "文档") || strings.Contains(lower, "readme") || strings.Contains(lower, "document"):
		return "docs"
	case strings.HasSuffix(lower, ".pptx"):
		return "ppt"
	case strings.HasSuffix(lower, ".mp4") || strings.HasSuffix(lower, ".webm") || strings.HasSuffix(lower, ".mov"):
		return "pv"
	case strings.Contains(name, "PPT") || strings.Contains(lower, "slides") || strings.Contains(name, "PV") || strings.Contains(lower, "storyboard"):
		// 大纲与分镜属于文档，不冒充已经渲染完成的 PPT / PV。
		return "docs"
	case strings.Contains(name, "宣传") || strings.Contains(name, "发布") || strings.Contains(lower, "promotion") || strings.Contains(lower, "campaign"):
		return "promotion"
	case strings.HasSuffix(lower, ".receipt") || strings.HasSuffix(lower, ".har"):
		return "promotion"
	case strings.Contains(name, "质量验收") || strings.Contains(lower, "qa.json") || strings.Contains(name, "质检"):
		// 质检报告是独立证据，不算任何交付阶段（stage 为空则不进 stageSet），
		// 但 kind=text 让它仍可预览——审批者能直接打开看质检结论。
		return ""
	case strings.HasPrefix(lower, "output-") || strings.HasSuffix(lower, ".py") || strings.HasSuffix(lower, ".js") || strings.HasSuffix(lower, ".ts") || strings.HasSuffix(lower, ".go") || strings.HasSuffix(lower, ".java") || strings.HasSuffix(lower, ".html"):
		return "code"
	case strings.HasPrefix(agent, "designer-"):
		return "ui"
	case strings.HasPrefix(agent, "researcher-"):
		return "research"
	case strings.HasSuffix(lower, ".md") && strings.HasPrefix(agent, "writer-"):
		return "docs"
	case strings.HasPrefix(agent, "promoter-") || strings.HasPrefix(agent, "publisher-"):
		return "promotion"
	}
	return ""
}

func projectPreviewKind(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".xlsx") || strings.HasSuffix(lower, ".xls") || strings.HasSuffix(lower, ".csv") || strings.HasSuffix(lower, ".tsv"):
		return "spreadsheet"
	case strings.HasSuffix(lower, ".mp4") || strings.HasSuffix(lower, ".webm") || strings.HasSuffix(lower, ".mov"):
		return "video"
	case strings.HasSuffix(lower, ".pptx"):
		return "pptx"
	case strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") || strings.HasSuffix(lower, ".gif") || strings.HasSuffix(lower, ".webp") || strings.HasSuffix(lower, ".svg"):
		return "image"
	case strings.HasSuffix(lower, ".html"):
		return "html"
	case strings.HasSuffix(lower, ".py") || strings.HasSuffix(lower, ".js") || strings.HasSuffix(lower, ".ts") || strings.HasSuffix(lower, ".go") || strings.HasSuffix(lower, ".java"):
		return "code"
	case strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".txt") || strings.HasSuffix(lower, ".json") || strings.HasSuffix(lower, ".receipt") || strings.HasSuffix(lower, ".har"):
		return "text"
	default:
		return ""
	}
}

func inspectProjectDelivery(agent, projectPath string, entries []os.DirEntry) (files []gin.H, preview, previewFile, previewKind string, stages []string) {
	stageSet := map[string]bool{}
	bestPreviewScore := -1
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		name := entry.Name()
		stage := projectArtifactStage(agent, name)
		kind := projectPreviewKind(name)
		if stage != "" {
			stageSet[stage] = true
		}
		// output-* 是 Agent OS 真实提取并验证过的可运行源文件。
		if stage == "code" && strings.HasPrefix(strings.ToLower(name), "output-") {
			stageSet["runnable"] = true
		}
		files = append(files, gin.H{"name": name, "stage": stage, "kind": kind, "size": info.Size(), "previewable": kind != ""})
		if kind == "" {
			continue
		}
		score := map[string]int{"video": 120, "pptx": 110, "spreadsheet": 105, "html": 100, "image": 90, "code": 80, "text": 50}[kind]
		if strings.HasPrefix(strings.ToLower(name), "output-") {
			score += 15
		}
		if score <= bestPreviewScore {
			continue
		}
		if kind == "video" || kind == "pptx" || kind == "spreadsheet" || kind == "image" {
			bestPreviewScore, previewFile, previewKind = score, name, kind
			continue
		}
		data, err := os.ReadFile(filepath.Join(projectPath, name))
		if err != nil {
			continue
		}
		content := string(data)
		if utf8.RuneCountInString(content) > 6000 {
			content = string([]rune(content)[:6000]) + "\n…"
		}
		bestPreviewScore, preview, previewFile, previewKind = score, content, name, kind
	}
	for _, stage := range projectStageOrder {
		if stageSet[stage] {
			stages = append(stages, stage)
		}
	}
	return
}

// approvalRecord 一条审批记录（持久化到公司目录 .approvals.json）
type approvalRecord struct {
	Agent    string `json:"agent"`
	File     string `json:"file"`
	Project  string `json:"project,omitempty"` // 规范化项目身份；项目审批不再绑定某个 Agent 的单份目录
	Decision string `json:"decision"`          // approve | reject
	Feedback string `json:"feedback,omitempty"`
	Time     string `json:"time"`
}

func canonicalApprovalProject(file string) string {
	clean := filepath.ToSlash(filepath.Clean(file))
	if !strings.HasPrefix(clean, "project/") {
		return ""
	}
	name := strings.TrimPrefix(clean, "project/")
	name = filepath.Base(filepath.FromSlash(name))
	// 与审批台展示的项目标题同一套规范化：去序号前缀与交付时间戳后缀，
	// 保证「前端看到的标题」和「后端记录的已决定项目」是同一个身份。
	name = deliveryProjectTitle(name)
	name = strings.TrimLeft(name, "0123456789-_")
	return strings.ToLower(strings.TrimSpace(name))
}

func approvalDecisionIndex(recs []approvalRecord) (map[string]bool, map[string]bool) {
	files := map[string]bool{}
	projects := map[string]bool{}
	for _, record := range recs {
		files[record.Agent+"|"+record.File] = true
		project := strings.ToLower(strings.TrimSpace(record.Project))
		if project == "" {
			// 向后兼容：旧版虽然声称按项目审批，却逐文件写记录。任一旧项目记录
			// 都应代表整个规范化项目已被决定，避免同名的其他 Agent 目录再次出现。
			project = canonicalApprovalProject(record.File)
		}
		if project != "" {
			projects[project] = true
		}
	}
	return files, projects
}

// approvalsFilePath 审批记录文件
func approvalsFilePath() string {
	return filepath.Join(companyDir(), ".approvals.json")
}

// loadApprovals 读取审批记录
func loadApprovals() []approvalRecord {
	var recs []approvalRecord
	data, err := os.ReadFile(approvalsFilePath())
	if err != nil {
		return recs
	}
	json.Unmarshal(data, &recs)
	return recs
}

// saveApprovals 保存审批记录
func saveApprovals(recs []approvalRecord) {
	if len(recs) > 200 {
		recs = recs[len(recs)-200:]
	}
	data, _ := json.MarshalIndent(recs, "", "  ")
	os.WriteFile(approvalsFilePath(), data, 0o644)
}

// companyProjectRoots 项目扫描位置：真身根目录优先，历史遗留的 agent 内 projects/ 目录
// 作为只读兼容（老版本把项目写在 coder-03/projects/ 下），不迁移、不删除用户数据。
func companyProjectRoots() []string {
	roots := []string{companyProjectsRoot()}
	if entries, err := os.ReadDir(companyDir()); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			legacy := filepath.Join(companyDir(), e.Name(), "projects")
			if info, statErr := os.Stat(legacy); statErr == nil && info.IsDir() {
				roots = append(roots, legacy)
			}
		}
	}
	return roots
}

// companyProjectAgentOf 从项目路径推断归属 agent 目录名（用于文件读取参数）。
func companyProjectAgentOf(root string) string {
	rel, err := filepath.Rel(companyDir(), root)
	if err != nil {
		return ""
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) >= 2 {
		return parts[0]
	}
	return ""
}

// HandleCompanyApprovals GET /api/company/approvals
// 返回：各 agent 未审批的最近产出物（待审批队列）+ 项目真身交付
func HandleCompanyApprovals(c *gin.Context) {
	decided := loadApprovals()
	decidedKey, decidedProjects := approvalDecisionIndex(decided)

	dir := companyDir()
	entries, _ := os.ReadDir(dir)
	var pending []gin.H
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		agentDir := filepath.Join(dir, e.Name())
		// 1. outputs 下的产出物（会议/需求/计划/文档/设计/PPT/PV/调研/文章）
		outDir := filepath.Join(agentDir, "outputs")
		best := ""
		bestScore := -1
		if files, err := os.ReadDir(outDir); err == nil {
			for _, f := range files {
				if f.IsDir() || strings.HasPrefix(f.Name(), "README") ||
					strings.HasPrefix(f.Name(), "今日目标") || strings.HasPrefix(f.Name(), "每日资讯") {
					continue
				}
				key := e.Name() + "|outputs/" + f.Name()
				if decidedKey[key] {
					continue
				}
				score := produceScore(f.Name())
				if score > bestScore {
					bestScore = score
					best = "outputs/" + f.Name()
				}
			}
		}
		// 2. 项目交付统一从项目根目录扫描，见 collectCompanyProjectPending。
		if best != "" {
			pending = append(pending, gin.H{
				"agent": e.Name(),
				"file":  best,
				"score": bestScore,
				"kind":  fileKind(best),
			})
		}
	}
	pending = append(pending, collectCompanyProjectPending(decidedKey, decidedProjects)...)
	// 按分数降序（会议/项目/需求/设计优先）
	sort.Slice(pending, func(i, j int) bool {
		si, _ := pending[i]["score"].(int)
		sj, _ := pending[j]["score"].(int)
		return si > sj
	})
	c.JSON(http.StatusOK, gin.H{"pending": pending})
}

// fileKind 产出物类型
func fileKind(path string) string {
	name := filepath.Base(path)
	switch {
	case strings.HasPrefix(name, "会议"):
		return "meeting"
	case strings.HasPrefix(name, "需求"):
		return "requirement"
	case strings.HasPrefix(name, "计划"):
		return "plan"
	case strings.HasPrefix(name, "文档"):
		return "doc"
	case strings.HasPrefix(name, "设计"):
		return "design"
	case strings.HasSuffix(strings.ToLower(name), ".pptx"):
		return "ppt"
	case strings.HasSuffix(strings.ToLower(name), ".mp4") || strings.HasSuffix(strings.ToLower(name), ".webm") || strings.HasSuffix(strings.ToLower(name), ".mov"):
		return "pv"
	case strings.HasPrefix(name, "文章"):
		return "article"
	case strings.HasPrefix(name, "调研"):
		return "research"
	default:
		return "doc"
	}
}

// produceScore 产出物优先级打分（高分的优先展示，低分的日常文件靠后）
func produceScore(name string) int {
	switch {
	case strings.HasPrefix(name, "会议"):
		return 100
	case strings.HasPrefix(name, "需求"):
		return 95
	case strings.HasPrefix(name, "计划"):
		return 90
	case strings.HasPrefix(name, "文档"):
		return 85
	case strings.HasPrefix(name, "设计"):
		return 80
	case strings.HasPrefix(name, "PPT"):
		return 75
	case strings.HasPrefix(name, "PV"):
		return 70
	case strings.HasPrefix(name, "文章"):
		return 60
	case strings.HasPrefix(name, "调研"):
		return 55
	case strings.HasPrefix(name, "学习"):
		return 40
	case strings.HasPrefix(name, "任务"):
		return 30
	default:
		return 10
	}
}

// HandleCompanyApprove POST /api/company/approve {agent, file, decision, feedback}
func HandleCompanyApprove(c *gin.Context) {
	var body struct {
		Agent    string `json:"agent"`
		File     string `json:"file"`
		Project  string `json:"project"`
		Decision string `json:"decision"` // approve | reject
		Feedback string `json:"feedback"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.File == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	// 项目级审批以项目真身为单位，不绑定某个 agent 目录，此时 agent 允许为空。
	if body.Agent != "" && (strings.Contains(body.Agent, "..") || strings.ContainsAny(body.Agent, `/\`)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法 agent"})
		return
	}
	if body.Decision != "approve" && body.Decision != "reject" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "decision 只能是 approve 或 reject"})
		return
	}
	// 安全校验：仅禁止路径穿越与绝对路径（审批队列的 file 是 project/xxx 相对路径，允许 /）
	if strings.Contains(body.File, "..") || filepath.IsAbs(body.File) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法文件名"})
		return
	}

	recs := loadApprovals()
	recs = append(recs, approvalRecord{
		Agent:    body.Agent,
		File:     body.File,
		Project:  strings.ToLower(strings.TrimSpace(body.Project)),
		Decision: body.Decision,
		Feedback: body.Feedback,
		Time:     time.Now().Format("2006-01-02 15:04"),
	})
	saveApprovals(recs)

	// 审批结果：approved 标发布、rejected 标退回（写入 agent 目录一个标记文件）
	markDir := filepath.Join(companyDir(), body.Agent, "approvals")
	os.MkdirAll(markDir, 0o755)
	base := strings.TrimSuffix(body.File, filepath.Ext(body.File))
	mark := "approved"
	if body.Decision == "reject" {
		mark = "rejected"
	}
	markPath := filepath.Join(markDir, base+"."+mark)
	os.MkdirAll(filepath.Dir(markPath), 0o755)
	var fb string
	if body.Feedback != "" {
		fb = "：「" + body.Feedback + "」"
	}
	os.WriteFile(markPath, []byte(fmt.Sprintf("%s %s %s%s",
		time.Now().Format("2006-01-02 15:04"), body.Decision, body.Agent, fb)), 0o644)

	c.JSON(http.StatusOK, gin.H{"ok": true, "decision": body.Decision})
}

// HandleCompanyApprovalHistory GET /api/company/approval-history
func HandleCompanyApprovalHistory(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"history": loadApprovals()})
}
