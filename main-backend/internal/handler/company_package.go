package handler

// company_package.go —— 项目交付的唯一真身层。
//
// 交付引擎此前把同一份产物镜像到多个角色目录，审批队列再按文件名反推阶段，
// 导致一个项目同时存在于六份目录里，既无法整体打包，也无法判断哪一份才是真相。
// 这里把「项目」收敛成一个目录：company/projects/<项目名>/，
// 审批、预览、打包、迭代全部以它为准。

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

// companyProjectsRoot 项目真身根目录。
func companyProjectsRoot() string {
	return filepath.Join(companyDir(), "projects")
}

// companyProjectDir 单个项目的真身目录。
func companyProjectDir(projectName string) string {
	return filepath.Join(companyProjectsRoot(), projectName)
}

// companyProjectIndex 项目索引：记录项目身份与参与角色，供审批台直接读取，
// 不再靠扫描各 agent 目录的 outputs/ 反推。
type companyProjectIndex struct {
	Project     string   `json:"project"`
	Title       string   `json:"title"`
	Directive   string   `json:"directive"`
	Roles       []string `json:"roles"`
	Agents      []string `json:"agents"`
	CreatedAt   string   `json:"createdAt"`
	GeneratedAt string   `json:"generatedAt"`
	Status      string   `json:"status"`
}

func companyProjectIndexPath(projectDir string) string {
	return filepath.Join(projectDir, "project.json")
}

// writeCompanyProjectIndex 落盘项目身份。参与角色来自交付清单的 producerRole，
// 是真实分工记录，不是镜像目录名。
func writeCompanyProjectIndex(projectDir, projectName, directive string, manifest companyDeliveryManifest) error {
	roleSet := map[string]bool{}
	for _, ev := range manifest.Evidence {
		if ev.ProducerRole != "" {
			roleSet[ev.ProducerRole] = true
		}
	}
	roles := make([]string, 0, len(roleSet))
	for role := range roleSet {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	index := companyProjectIndex{
		Project:     projectName,
		Title:       deliveryProjectTitle(projectName),
		Directive:   directive,
		Roles:       roles,
		Agents:      companyRoleAgentNames(roles),
		CreatedAt:   time.Now().Format(time.RFC3339),
		GeneratedAt: manifest.GeneratedAt,
		Status:      manifest.Status,
	}
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(companyProjectIndexPath(projectDir), data, 0o644)
}

// deliveryProjectTitle 从落盘目录名还原可读标题：去掉序号前缀与时间戳后缀。
func deliveryProjectTitle(projectName string) string {
	name := strings.TrimSpace(projectName)
	name = strings.TrimPrefix(name, "./")
	if idx := strings.LastIndex(name, "-"); idx > 0 {
		tail := name[idx+1:]
		if len(tail) == 11 && tail[2] == '-' && tail[5] == '-' && tail[8] == '-' {
			name = name[:idx]
		}
	}
	if dash := strings.Index(name, "-"); dash > 0 && dash <= 4 {
		head := name[:dash]
		if strings.TrimLeft(head, "0123456789") == "" {
			name = name[dash+1:]
		}
	}
	return strings.TrimSpace(name)
}

// companyRoleAgentNames 把交付清单里的角色映射回真实 agent 目录名，
// 供审批台展示「哪些部门参与」，不再依赖镜像目录是否存在。
func companyRoleAgentNames(roles []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, role := range roles {
		dir := deliveryRoleAgentDirs(role)
		if dir == "" {
			continue
		}
		name := filepath.Base(dir)
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// companyLoadProjectManifest 读取交付清单。
func companyLoadProjectManifest(projectDir string) (companyDeliveryManifest, error) {
	var manifest companyDeliveryManifest
	data, err := os.ReadFile(filepath.Join(projectDir, "delivery.manifest.json"))
	if err != nil {
		return manifest, err
	}
	err = json.Unmarshal(data, &manifest)
	return manifest, err
}

// companyZipProject 把项目真身目录整体打成 zip。
// 排除包本身，避免重复打包时体积滚雪球。
func companyZipProject(projectDir string) ([]byte, error) {
	files := companyCollectProjectFiles(projectDir)
	if len(files) == 0 {
		return nil, fmt.Errorf("项目内没有可打包的产物")
	}
	buf := &bytes.Buffer{}
	writer := zip.NewWriter(buf)
	for _, file := range files {
		src := filepath.Join(projectDir, filepath.FromSlash(file.Path))
		info, statErr := os.Stat(src)
		if statErr != nil || info.Size() > 64<<20 {
			continue
		}
		entry, newErr := writer.Create(file.Path)
		if newErr != nil {
			continue
		}
		data, readErr := os.ReadFile(src)
		if readErr != nil {
			continue
		}
		if _, writeErr := entry.Write(data); writeErr != nil {
			continue
		}
	}
	// 交付清单与项目身份一并入包，接收方拿到包就能复算验收。
	if data, err := os.ReadFile(filepath.Join(projectDir, "delivery.manifest.json")); err == nil {
		if entry, err := writer.Create("delivery.manifest.json"); err == nil {
			_, _ = entry.Write(data)
		}
	}
	if data, err := os.ReadFile(companyProjectIndexPath(projectDir)); err == nil {
		if entry, err := writer.Create("project.json"); err == nil {
			_, _ = entry.Write(data)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// companyProjectZipName 包名：项目标题 + 交付时间，避免同名覆盖。
func companyProjectZipName(projectName string) string {
	title := deliveryProjectTitle(projectName)
	if title == "" {
		title = "delivery"
	}
	return fmt.Sprintf("%s-完整交付.zip", deliverySanitize(title))
}

// companySafeProjectDir 校验项目名并返回真身目录，拒绝路径穿越。
func companySafeProjectDir(projectName string) (string, error) {
	name := strings.TrimSpace(filepath.Base(projectName))
	if name == "" || name == "." || name == ".." || strings.Contains(name, "..") {
		return "", fmt.Errorf("非法项目名")
	}
	if dir, ok := companyFindProjectDir(name); ok {
		return dir, nil
	}
	return "", fmt.Errorf("项目不存在")
}

// companyFindProjectDir 按项目名定位真身目录，兼容历史遗留的 agent 内 projects/ 位置。
func companyFindProjectDir(name string) (string, bool) {
	for _, root := range companyProjectRoots() {
		dir := filepath.Join(root, name)
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir, true
		}
	}
	return "", false
}

// HandleCompanyProjectPackage GET /api/company/package?project=xxx
// 返回整个项目的 zip 包。
func HandleCompanyProjectPackage(c *gin.Context) {
	dir, err := companySafeProjectDir(c.Query("project"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	data, err := companyZipProject(dir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Disposition", `attachment; filename="`+companyProjectZipName(filepath.Base(dir))+`"`)
	c.Data(http.StatusOK, "application/zip", data)
}

// qaViewForDir 项目目录 → 前端审批卡用的质检摘要（无质检返回 checked:false）。
// 与 collectCompanyProjectPending 里的 qa 字段同一套形状，审批卡与项目列表共用。
func qaViewForDir(projectDir string) gin.H {
	qa, ok := loadCompanyQAReport(projectDir)
	if !ok {
		return gin.H{"checked": false}
	}
	return gin.H{
		"checked": true, "passed": qa.Passed, "repaired": qa.Repaired, "blank": qa.Blank,
		"visualScore": qa.VisualScore, "buttons": qa.Buttons, "clicked": qa.Clicked,
		"domChanged": qa.DOMChanged, "interactMeasured": qa.InteractOK, "visibleElements": qa.JSVisible, "textLength": qa.TextLength,
		"journeyMeasured": qa.JourneyOK, "journeyPassed": qa.JourneyPass, "topicHits": qa.TopicHits,
		"browserOk": qa.BrowserOK, "visionOk": qa.VisionOK, "issues": qa.Issues,
		"summary": qa.Summary, "checkedAt": qa.CheckedAt,
		// 新增质检维度：多帧评审帧数、布局充实度、返修轮数（前端审批卡展示用）
		"framesReviewed": qa.FramesReviewed, "layoutMeasured": qa.LayoutOK,
		"pageHeightRatio": qa.PageHeightRatio, "repairRounds": qa.RepairRounds,
	}
}

// HandleCompanyProjects GET /api/company/projects
// 项目真身列表：审批台、迭代区、打包入口共用同一份数据源。
func HandleCompanyProjects(c *gin.Context) {
	out := []gin.H{}
	for _, root := range companyProjectRoots() {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			dir := filepath.Join(root, entry.Name())
			manifest, mErr := companyLoadProjectManifest(dir)
			if mErr != nil {
				continue
			}
			index := loadCompanyProjectIndex(dir, entry.Name())
			files := companyBindManifestEvidence(companyCollectProjectFiles(dir), manifest)
			stages := []string{}
			stageSet := map[string]bool{}
			for _, ev := range manifest.Evidence {
				stageSet[ev.Stage] = true
			}
			for _, stage := range projectStageOrder {
				if stageSet[stage] {
					stages = append(stages, stage)
				}
			}
			out = append(out, gin.H{
				"project": index.Project, "title": index.Title, "directive": index.Directive,
				"roles": index.Roles, "agents": index.Agents, "status": manifest.Status,
				"missing": manifest.Missing, "stages": stages, "artifacts": files,
				"generatedAt": manifest.GeneratedAt, "gateVersion": manifest.GateVersion,
				"qa": qaViewForDir(dir),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		gi, _ := out[i]["generatedAt"].(string)
		gj, _ := out[j]["generatedAt"].(string)
		return gi > gj
	})
	c.JSON(http.StatusOK, gin.H{"projects": out})
}

// collectCompanyProjectPending 扫描项目真身目录，产出待审批项目条目。
// 一个项目只出现一次（不再因镜像到多个 agent 目录而重复），产物来自交付清单本身。
func collectCompanyProjectPending(decidedKey, decidedProjects map[string]bool) []gin.H {
	out := []gin.H{}
	for _, root := range companyProjectRoots() {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		agent := companyProjectAgentOf(root)
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			projectName := entry.Name()
			relFile := "project/" + projectName
			if decidedProjects[canonicalApprovalProject(relFile)] {
				continue
			}
			projectDir := filepath.Join(root, projectName)
			gate, gateErr := verifyProjectDeliveryGate(projectDir)
			if gateErr != nil {
				// 硬门禁未通过的项目留在生产区，绝不进入人类审批队列。
				continue
			}
			// 质量硬门禁：真机质检未通过的项目也留在生产区，等质检返修后再进审批台。
			// （无质检文件的老项目放行——那是历史资产，不是本轮的交付。）
			if qa, ok := loadCompanyQAReport(projectDir); ok && !qa.Skipped && !qa.Passed {
				continue
			}
			if decidedKey[agent+"|"+relFile] {
				continue
			}
			manifest, _ := companyLoadProjectManifest(projectDir)
			index := loadCompanyProjectIndex(projectDir, projectName)
			files := companyBindManifestEvidence(companyCollectProjectFiles(projectDir), manifest)
			artifacts := []gin.H{}
			stageSet := map[string]bool{}
			srcCode := ""
			reqPlan := ""
			for _, f := range files {
				stageSet[f.Stage] = true
				if f.Name == "output-app.html" || strings.HasPrefix(f.Name, "output-") {
					srcCode = f.Name
					// output-* 同时是可运行程序证据：runnable 阶段与 code 共用这份产物，
					// 缺了它审批台的「程序」节点会永远灰着。
					stageSet["runnable"] = true
				}
				if strings.HasPrefix(f.Name, "00-需求计划") {
					reqPlan = f.Name
				}
				artifacts = append(artifacts, gin.H{
					"name": f.Name, "path": f.Path, "stage": f.Stage, "kind": f.Kind,
					"size": f.Size, "previewable": f.Previewable,
					"producerRole": f.Role, "sha256": f.SHA256, "verification": f.Verify,
				})
			}
			stages := []string{}
			for _, stage := range projectStageOrder {
				if stageSet[stage] {
					stages = append(stages, stage)
				}
			}
			agents := index.Agents
			if len(agents) == 0 {
				agents = []string{agent}
			}
			qa, qaOK := loadCompanyQAReport(projectDir)
			qaView := gin.H{"checked": false}
			if qaOK {
				qaView = gin.H{
					"checked": true, "passed": qa.Passed, "repaired": qa.Repaired, "blank": qa.Blank,
					"visualScore": qa.VisualScore, "buttons": qa.Buttons, "clicked": qa.Clicked,
					"domChanged": qa.DOMChanged, "interactMeasured": qa.InteractOK, "visibleElements": qa.JSVisible, "textLength": qa.TextLength,
					"journeyMeasured": qa.JourneyOK, "journeyPassed": qa.JourneyPass, "topicHits": qa.TopicHits,
					"browserOk": qa.BrowserOK, "visionOk": qa.VisionOK, "issues": qa.Issues,
					"summary": qa.Summary, "checkedAt": qa.CheckedAt,
				}
			}
			out = append(out, gin.H{
				"agent": agent, "file": relFile, "score": 92, "kind": "project",
				"project": projectName, "title": index.Title, "requirement": reqPlan,
				"source": srcCode, "artifacts": artifacts, "stages": stages,
				"roles": index.Roles, "agents": agents, "missing": manifest.Missing,
				"gateStatus": gate.Status, "qa": qaView,
				"packageUrl": "/api/company/package?project=" + projectName,
			})
		}
	}
	return out
}

// HandleCompanyProjectFile GET /api/company/project-file?project=xxx&path=01-调研报告.md
// 从项目真身目录读取产物，路径限制在项目内。
func HandleCompanyProjectFile(c *gin.Context) {
	dir, err := companySafeProjectDir(c.Query("project"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rel := filepath.ToSlash(strings.TrimSpace(c.Query("path")))
	if rel == "" || strings.HasPrefix(rel, "/") || strings.Contains(rel, "..") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	path := filepath.Join(dir, filepath.FromSlash(rel))
	info, statErr := os.Stat(path)
	if statErr != nil || info.IsDir() {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
		return
	}
	ext := strings.ToLower(filepath.Ext(path))
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if c.Query("raw") == "1" {
		c.Header("Content-Disposition", `inline; filename="`+filepath.Base(path)+`"`)
		http.ServeFile(c.Writer, c.Request, path)
		return
	}
	result := gin.H{"name": filepath.Base(rel), "path": rel, "mime": contentType, "size": info.Size()}
	if kind := projectPreviewKind(filepath.Base(rel)); kind == "text" || kind == "html" || kind == "code" {
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
		result["truncated"] = utf8.RuneCountInString(string(data)) > 120000
	}
	c.JSON(http.StatusOK, result)
}

// mustReadFile 读取文件内容，失败返回空。
func mustReadFile(path string) []byte {
	data, _ := os.ReadFile(path)
	return data
}

// loadCompanyProjectIndex 读取项目身份，缺失时按目录名兜底（历史项目没有 project.json）。
func loadCompanyProjectIndex(projectDir, projectName string) companyProjectIndex {
	index := companyProjectIndex{Project: projectName, Title: deliveryProjectTitle(projectName)}
	if data, err := os.ReadFile(companyProjectIndexPath(projectDir)); err == nil {
		_ = json.Unmarshal(data, &index)
		if index.Project == "" {
			index.Project = projectName
		}
		if index.Title == "" {
			index.Title = deliveryProjectTitle(projectName)
		}
	}
	return index
}

// companyProjectFile 项目内一份产物（含子目录，路径相对项目根）。
type companyProjectFile struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Stage    string `json:"stage"`
	Kind     string `json:"kind"`
	Size     int64  `json:"size"`
	SHA256   string `json:"sha256,omitempty"`
	Role     string `json:"producerRole,omitempty"`
	Verify   string `json:"verification,omitempty"`
	Previewable bool `json:"previewable"`
}

// companyCollectProjectFiles 递归收集项目内全部产物。
// 此前审批扫描用 entry.IsDir() 直接跳过目录，宣传分镜、生图素材这类子目录产物
// 从未进入交付视图，也就无从打包。
func companyCollectProjectFiles(projectDir string) []companyProjectFile {
	out := []companyProjectFile{}
	_ = filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(projectDir, path)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "..") {
			return nil
		}
		base := filepath.Base(rel)
		if base == "delivery.manifest.json" || base == "project.json" {
			return nil
		}
		stage := projectArtifactStage("", base)
		kind := projectPreviewKind(base)
		out = append(out, companyProjectFile{
			Name: base, Path: rel, Stage: stage, Kind: kind,
			Size: info.Size(), Previewable: kind != "",
		})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// companyBindManifestEvidence 用交付清单给产物补上角色、哈希与验收说明，
// 让审批台展示的是清单里真正验证过的文件，而不是按文件名猜出来的。
func companyBindManifestEvidence(files []companyProjectFile, manifest companyDeliveryManifest) []companyProjectFile {
	byPath := map[string]companyDeliveryFile{}
	for _, ev := range manifest.Evidence {
		byPath[filepath.ToSlash(ev.File)] = ev
	}
	for i := range files {
		if ev, ok := byPath[files[i].Path]; ok {
			files[i].SHA256 = ev.SHA256
			files[i].Role = ev.ProducerRole
			files[i].Verify = ev.Verification
			if files[i].Stage == "" {
				files[i].Stage = ev.Stage
			}
		}
	}
	return files
}
