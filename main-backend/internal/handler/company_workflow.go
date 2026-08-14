package handler

// company_workflow.go — 目标驱动的多 Agent 内容工作流。
//
// 关键路径不依赖“Agent 定时醒来碰巧看到别人的文件”，而是：
// Goal -> Task DAG -> Artifact -> Review -> Rework/Approve。
// 每个生产任务使用独立上下文执行，Reviewer 再用独立上下文验收；最终发布前
// 必须由用户明确批准。

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

const (
	goalStatusActive           = "active"
	goalStatusAwaitingApproval = "awaiting_approval"
	goalStatusCompleted        = "completed"
	goalStatusFailed           = "failed"

	workStatusBlocked      = "blocked"
	workStatusReady        = "ready"
	workStatusRunning      = "running"
	workStatusRework       = "rework"
	workStatusApproved     = "approved"
	workStatusWaitingHuman = "waiting_human"
	workStatusFailed       = "failed"
)

type companyGoal struct {
	ID            string            `json:"id"`
	Objective     string            `json:"objective"`
	SuccessMetric string            `json:"successMetric"`
	Status        string            `json:"status"`
	Workflow      string            `json:"workflow"`
	Tasks         []companyWorkItem `json:"tasks"`
	Artifacts     []companyArtifact `json:"artifacts"`
	Reviews       []companyReview   `json:"reviews"`
	Events        []companyEvent    `json:"events"`
	UserFeedback  string            `json:"userFeedback,omitempty"`
	FailureReason string            `json:"failureReason,omitempty"`
	Tags          []string          `json:"tags,omitempty"`
	CreatedAt     time.Time         `json:"createdAt"`
	UpdatedAt     time.Time         `json:"updatedAt"`
}

type companyWorkItem struct {
	ID                 string    `json:"id"`
	Stage              string    `json:"stage"`
	Title              string    `json:"title"`
	OwnerRole          string    `json:"ownerRole"`
	ReviewerRole       string    `json:"reviewerRole,omitempty"`
	Objective          string    `json:"objective"`
	Dependencies       []string  `json:"dependencies"`
	AcceptanceCriteria []string  `json:"acceptanceCriteria"`
	Status             string    `json:"status"`
	Attempt            int       `json:"attempt"`
	MaxAttempts        int       `json:"maxAttempts"`
	ArtifactID         string    `json:"artifactId,omitempty"`
	LastIssue          string    `json:"lastIssue,omitempty"`
	StartedAt          time.Time `json:"startedAt,omitempty"`
	FinishedAt         time.Time `json:"finishedAt,omitempty"`
}

type companyArtifact struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"taskId"`
	Stage     string    `json:"stage"`
	Producer  string    `json:"producer"`
	Version   int       `json:"version"`
	Path      string    `json:"path"`
	Preview   string    `json:"preview"`
	Accepted  bool      `json:"accepted"`
	CreatedAt time.Time `json:"createdAt"`
}

type companyReview struct {
	ID         string    `json:"id"`
	TaskID     string    `json:"taskId"`
	ArtifactID string    `json:"artifactId"`
	Reviewer   string    `json:"reviewer"`
	Verdict    string    `json:"verdict"`
	Score      int       `json:"score"`
	Issues     []string  `json:"issues"`
	Summary    string    `json:"summary"`
	CreatedAt  time.Time `json:"createdAt"`
}

type companyEvent struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	TaskID    string    `json:"taskId,omitempty"`
	Actor     string    `json:"actor"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

type reviewerDecision struct {
	Verdict string   `json:"verdict"`
	Score   int      `json:"score"`
	Issues  []string `json:"issues"`
	Summary string   `json:"summary"`
}

var (
	companyWorkflowMu sync.Mutex
	companyRuns       sync.Map
	companyModelCall  = callLocalAggregate
)

func companyWorkflowDir() string {
	if override := strings.TrimSpace(os.Getenv("RESCENE_COMPANY_WORKFLOW_DIR")); override != "" {
		return override
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "rescene_data", "company_workflows")
}

func companyGoalDir(id string) string {
	return filepath.Join(companyWorkflowDir(), id)
}

func companyGoalStatePath(id string) string {
	return filepath.Join(companyGoalDir(id), "state.json")
}

func newWorkflowID(prefix string) string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return fmt.Sprintf("%s-%s-%s", prefix, time.Now().Format("20060102-150405"), hex.EncodeToString(b))
}

func newContentGoal(objective, successMetric string) *companyGoal {
	now := time.Now()
	if strings.TrimSpace(successMetric) == "" {
		successMetric = "研究、正文和传播物料全部通过独立验收，并由用户最终批准"
	}
	g := &companyGoal{
		ID:            newWorkflowID("goal"),
		Objective:     strings.TrimSpace(objective),
		SuccessMetric: strings.TrimSpace(successMetric),
		Status:        goalStatusActive,
		Workflow:      "content_studio_v1",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	g.Tasks = []companyWorkItem{
		{
			ID: "research", Stage: "research", Title: "研究包", OwnerRole: "researcher", ReviewerRole: "editor",
			Objective:          "建立文章的事实边界、核心问题、论点地图和待核实事项",
			AcceptanceCriteria: []string{"包含读者为什么关心", "区分事实、推断与待核实项", "给出作者可直接使用的论点结构", "不得编造来源链接"},
			Status:             workStatusReady, MaxAttempts: 2,
		},
		{
			ID: "draft", Stage: "draft", Title: "文章初稿", OwnerRole: "writer", ReviewerRole: "editor",
			Objective:          "根据已验收研究包完成可发表的中文文章初稿",
			Dependencies:       []string{"research"},
			AcceptanceCriteria: []string{"回应目标读者的真实问题", "关键判断与研究包一致", "结构清晰且不是空泛 AI 腔", "不把待核实内容写成确定事实"},
			Status:             workStatusBlocked, MaxAttempts: 2,
		},
		{
			ID: "promotion", Stage: "promotion", Title: "传播包", OwnerRole: "promoter", ReviewerRole: "editor",
			Objective:          "把已验收正文转化为标题、摘要和平台传播物料",
			Dependencies:       []string{"draft"},
			AcceptanceCriteria: []string{"包含至少 3 个标题方案", "摘要忠于正文", "传播文案不夸大正文结论", "明确推荐的主标题及理由"},
			Status:             workStatusBlocked, MaxAttempts: 2,
		},
		{
			ID: "human-approval", Stage: "approval", Title: "用户终审", OwnerRole: "user",
			Objective: "由用户决定批准交付或退回修改", Dependencies: []string{"promotion"},
			AcceptanceCriteria: []string{"用户明确批准"}, Status: workStatusBlocked, MaxAttempts: 1,
		},
	}
	g.addEvent("goal.created", "manager", "公司目标已建立，研究任务解除阻塞", "")
	return g
}

func (g *companyGoal) addEvent(kind, actor, message, taskID string) {
	g.Events = append(g.Events, companyEvent{
		ID: newWorkflowID("event"), Type: kind, Actor: actor, Message: message, TaskID: taskID, CreatedAt: time.Now(),
	})
	if len(g.Events) > 200 {
		g.Events = g.Events[len(g.Events)-200:]
	}
	g.UpdatedAt = time.Now()
}

func saveCompanyGoal(g *companyGoal) error {
	if g == nil || strings.TrimSpace(g.ID) == "" {
		return errors.New("目标为空")
	}
	dir := companyGoalDir(g.ID)
	if err := os.MkdirAll(filepath.Join(dir, "artifacts"), 0o755); err != nil {
		return err
	}
	g.UpdatedAt = time.Now()
	b, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(companyGoalStatePath(g.ID), b, 0o644)
}

func loadCompanyGoal(id string) (*companyGoal, error) {
	if !validWorkflowID(id) {
		return nil, errors.New("目标 ID 非法")
	}
	b, err := os.ReadFile(companyGoalStatePath(id))
	if err != nil {
		return nil, err
	}
	var g companyGoal
	if err := json.Unmarshal(b, &g); err != nil {
		return nil, err
	}
	return &g, nil
}

func validWorkflowID(id string) bool {
	return id != "" && !strings.Contains(id, "..") && !strings.ContainsAny(id, `/\\`)
}

func listCompanyGoals() ([]companyGoal, error) {
	entries, err := os.ReadDir(companyWorkflowDir())
	if os.IsNotExist(err) {
		return []companyGoal{}, nil
	}
	if err != nil {
		return nil, err
	}
	goals := make([]companyGoal, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || !validWorkflowID(entry.Name()) {
			continue
		}
		g, err := loadCompanyGoal(entry.Name())
		if err == nil {
			goals = append(goals, *g)
		}
	}
	sort.Slice(goals, func(i, j int) bool { return goals[i].CreatedAt.After(goals[j].CreatedAt) })
	return goals, nil
}

func findCompanyTask(g *companyGoal, id string) *companyWorkItem {
	for i := range g.Tasks {
		if g.Tasks[i].ID == id {
			return &g.Tasks[i]
		}
	}
	return nil
}

func dependenciesApproved(g *companyGoal, task *companyWorkItem) bool {
	for _, dep := range task.Dependencies {
		t := findCompanyTask(g, dep)
		if t == nil || t.Status != workStatusApproved {
			return false
		}
	}
	return true
}

func reconcileCompanyTasks(g *companyGoal) {
	for i := range g.Tasks {
		t := &g.Tasks[i]
		if t.Status == workStatusBlocked && dependenciesApproved(g, t) {
			if t.Stage == "approval" {
				t.Status = workStatusWaitingHuman
				g.Status = goalStatusAwaitingApproval
				g.addEvent("goal.awaiting_approval", "manager", "全部生产任务已通过，等待用户终审", t.ID)
			} else {
				t.Status = workStatusReady
				g.addEvent("task.ready", "manager", t.Title+"已解除阻塞", t.ID)
			}
		}
	}
}

func artifactContent(g *companyGoal, artifactID string) string {
	for _, artifact := range g.Artifacts {
		if artifact.ID == artifactID {
			b, err := os.ReadFile(filepath.Join(companyGoalDir(g.ID), filepath.FromSlash(artifact.Path)))
			if err == nil {
				return string(b)
			}
		}
	}
	return ""
}

func taskInputs(g *companyGoal, task *companyWorkItem) string {
	var parts []string
	for _, dep := range task.Dependencies {
		if upstream := findCompanyTask(g, dep); upstream != nil && upstream.ArtifactID != "" {
			content := artifactContent(g, upstream.ArtifactID)
			if utf8.RuneCountInString(content) > 8000 {
				content = string([]rune(content)[:8000])
			}
			parts = append(parts, fmt.Sprintf("【%s的已验收交付物】\n%s", upstream.Title, content))
		}
	}
	if strings.TrimSpace(g.UserFeedback) != "" && (task.Stage == "draft" || task.Stage == "promotion") {
		parts = append(parts, "【用户退回意见】\n"+g.UserFeedback)
	}
	if len(parts) == 0 {
		return "（无上游输入）"
	}
	return strings.Join(parts, "\n\n")
}

func productionPrompt(g *companyGoal, task *companyWorkItem) string {
	reworkBlock := ""
	if strings.TrimSpace(task.LastIssue) != "" {
		reworkBlock = "\n\n上次验收退回原因（本轮必须逐项修复）：\n- " + strings.ReplaceAll(task.LastIssue, "；", "\n- ")
	}
	return fmt.Sprintf(`你是 Rescene 公司中的【%s】。你收到的是一张正式工单，不要自行改变公司目标。

公司目标：%s
成功标准：%s
本任务：%s
任务目标：%s

上游已验收输入：
%s

验收标准：
- %s
%s

直接输出最终交付物正文。不要解释工作过程，不要声称访问了没有访问的网页，不要编造链接或引用。`,
		task.OwnerRole, g.Objective, g.SuccessMetric, task.Title, task.Objective, taskInputs(g, task),
		strings.Join(task.AcceptanceCriteria, "\n- "), reworkBlock)
}

func deterministicArtifactCheck(task *companyWorkItem, content string) error {
	n := utf8.RuneCountInString(strings.TrimSpace(content))
	minimum := 300
	if task.Stage == "promotion" {
		minimum = 120
	}
	if n < minimum {
		return fmt.Errorf("交付物过短：%d 字，最低要求 %d 字", n, minimum)
	}
	if strings.Contains(strings.ToLower(content), "作为一个ai") {
		return errors.New("交付物包含明显的模型免责声明")
	}
	return nil
}

func reviewPrompt(g *companyGoal, task *companyWorkItem, content string) string {
	return fmt.Sprintf(`你是与生产者上下文隔离的【%s】，负责验收公司交付物。不要因为文风流畅就放行。

公司目标：%s
工单：%s
验收标准：
- %s

待验收交付物：
---
%s
---

只输出 JSON，不要代码围栏：
{"verdict":"approve或reject","score":0到100,"issues":["具体问题"],"summary":"一句话验收结论"}

只有全部关键标准满足时才能 approve；reject 时 issues 必须具体、可执行，最多 3 条。`,
		task.ReviewerRole, g.Objective, task.Title, strings.Join(task.AcceptanceCriteria, "\n- "), content)
}

func parseReviewerDecision(content string) (reviewerDecision, error) {
	clean := strings.TrimSpace(content)
	clean = strings.TrimPrefix(clean, "```json")
	clean = strings.TrimPrefix(clean, "```")
	clean = strings.TrimSuffix(clean, "```")
	var decision reviewerDecision
	if err := json.Unmarshal([]byte(strings.TrimSpace(clean)), &decision); err != nil {
		return decision, err
	}
	decision.Verdict = strings.ToLower(strings.TrimSpace(decision.Verdict))
	if decision.Verdict != "approve" && decision.Verdict != "reject" {
		return decision, errors.New("Reviewer verdict 非法")
	}
	if decision.Score < 0 {
		decision.Score = 0
	}
	if decision.Score > 100 {
		decision.Score = 100
	}
	if len(decision.Issues) > 3 {
		decision.Issues = decision.Issues[:3]
	}
	return decision, nil
}

func writeCompanyArtifact(g *companyGoal, task *companyWorkItem, content string) (companyArtifact, error) {
	id := newWorkflowID("artifact")
	version := task.Attempt
	rel := filepath.ToSlash(filepath.Join("artifacts", fmt.Sprintf("%s-v%d.md", task.Stage, version)))
	abs := filepath.Join(companyGoalDir(g.ID), filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return companyArtifact{}, err
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		return companyArtifact{}, err
	}
	preview := strings.TrimSpace(content)
	if utf8.RuneCountInString(preview) > 180 {
		preview = string([]rune(preview)[:180]) + "…"
	}
	return companyArtifact{ID: id, TaskID: task.ID, Stage: task.Stage, Producer: task.OwnerRole, Version: version, Path: rel, Preview: preview, CreatedAt: time.Now()}, nil
}

func markWorkflowFailure(goalID, taskID string, err error) {
	companyWorkflowMu.Lock()
	defer companyWorkflowMu.Unlock()
	g, loadErr := loadCompanyGoal(goalID)
	if loadErr != nil {
		return
	}
	if task := findCompanyTask(g, taskID); task != nil {
		task.Status = workStatusFailed
		task.LastIssue = err.Error()
		task.FinishedAt = time.Now()
	}
	g.Status = goalStatusFailed
	g.FailureReason = err.Error()
	g.addEvent("goal.failed", "manager", err.Error(), taskID)
	_ = saveCompanyGoal(g)
}

func runCompanyContentWorkflow(goalID string) {
	if _, loaded := companyRuns.LoadOrStore(goalID, true); loaded {
		return
	}
	defer companyRuns.Delete(goalID)

	for _, taskID := range []string{"research", "draft", "promotion"} {
		for {
			companyWorkflowMu.Lock()
			g, err := loadCompanyGoal(goalID)
			if err != nil {
				companyWorkflowMu.Unlock()
				return
			}
			task := findCompanyTask(g, taskID)
			if task == nil || task.Status == workStatusApproved {
				companyWorkflowMu.Unlock()
				break
			}
			if !dependenciesApproved(g, task) && len(task.Dependencies) > 0 {
				companyWorkflowMu.Unlock()
				return
			}
			if task.Attempt >= task.MaxAttempts {
				companyWorkflowMu.Unlock()
				markWorkflowFailure(goalID, taskID, errors.New(task.Title+"已耗尽返工预算"))
				return
			}
			task.Status = workStatusRunning
			task.Attempt++
			task.StartedAt = time.Now()
			task.LastIssue = ""
			g.addEvent("task.started", task.OwnerRole, fmt.Sprintf("%s开始第 %d/%d 次执行", task.Title, task.Attempt, task.MaxAttempts), task.ID)
			prompt := productionPrompt(g, task)
			_ = saveCompanyGoal(g)
			companyWorkflowMu.Unlock()

			content, err := companyModelCall(prompt)
			if err != nil {
				markWorkflowFailure(goalID, taskID, fmt.Errorf("%s调用模型失败: %w", task.Title, err))
				return
			}

			companyWorkflowMu.Lock()
			g, err = loadCompanyGoal(goalID)
			if err != nil {
				companyWorkflowMu.Unlock()
				return
			}
			task = findCompanyTask(g, taskID)
			artifact, err := writeCompanyArtifact(g, task, content)
			if err != nil {
				companyWorkflowMu.Unlock()
				markWorkflowFailure(goalID, taskID, err)
				return
			}
			g.Artifacts = append(g.Artifacts, artifact)
			task.ArtifactID = artifact.ID
			g.addEvent("artifact.submitted", task.OwnerRole, task.Title+"已提交验收", task.ID)
			_ = saveCompanyGoal(g)
			companyWorkflowMu.Unlock()

			decision := reviewerDecision{}
			if err := deterministicArtifactCheck(task, content); err != nil {
				decision = reviewerDecision{Verdict: "reject", Score: 0, Issues: []string{err.Error()}, Summary: "确定性质量门未通过"}
			} else {
				reviewText, reviewErr := companyModelCall(reviewPrompt(g, task, content))
				if reviewErr != nil {
					markWorkflowFailure(goalID, taskID, fmt.Errorf("%s验收模型失败: %w", task.Title, reviewErr))
					return
				}
				decision, err = parseReviewerDecision(reviewText)
				if err != nil {
					markWorkflowFailure(goalID, taskID, fmt.Errorf("%s验收结果无法解析: %w", task.Title, err))
					return
				}
			}

			companyWorkflowMu.Lock()
			g, err = loadCompanyGoal(goalID)
			if err != nil {
				companyWorkflowMu.Unlock()
				return
			}
			task = findCompanyTask(g, taskID)
			review := companyReview{
				ID: newWorkflowID("review"), TaskID: task.ID, ArtifactID: task.ArtifactID, Reviewer: task.ReviewerRole,
				Verdict: decision.Verdict, Score: decision.Score, Issues: decision.Issues, Summary: decision.Summary, CreatedAt: time.Now(),
			}
			g.Reviews = append(g.Reviews, review)
			for i := range g.Artifacts {
				if g.Artifacts[i].ID == task.ArtifactID {
					g.Artifacts[i].Accepted = decision.Verdict == "approve"
				}
			}
			if decision.Verdict == "approve" {
				task.Status = workStatusApproved
				task.FinishedAt = time.Now()
				g.addEvent("review.approved", task.ReviewerRole, fmt.Sprintf("%s验收通过（%d 分）", task.Title, decision.Score), task.ID)
				reconcileCompanyTasks(g)
				_ = saveCompanyGoal(g)
				companyWorkflowMu.Unlock()
				break
			}

			task.Status = workStatusRework
			task.LastIssue = strings.Join(decision.Issues, "；")
			g.addEvent("review.rejected", task.ReviewerRole, task.Title+"被退回："+task.LastIssue, task.ID)
			_ = saveCompanyGoal(g)
			exhausted := task.Attempt >= task.MaxAttempts
			companyWorkflowMu.Unlock()
			if exhausted {
				markWorkflowFailure(goalID, taskID, errors.New(task.Title+"连续未通过验收"))
				return
			}
		}
	}

	companyWorkflowMu.Lock()
	defer companyWorkflowMu.Unlock()
	g, err := loadCompanyGoal(goalID)
	if err == nil {
		reconcileCompanyTasks(g)
		_ = saveCompanyGoal(g)
	}
}

// HandleCompanyGoals GET /api/company/goals
func HandleCompanyGoals(c *gin.Context) {
	companyWorkflowMu.Lock()
	defer companyWorkflowMu.Unlock()
	goals, err := listCompanyGoals()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"goals": goals})
}

// HandleCompanyGoal GET /api/company/goals/:id
func HandleCompanyGoal(c *gin.Context) {
	companyWorkflowMu.Lock()
	defer companyWorkflowMu.Unlock()
	g, err := loadCompanyGoal(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "目标不存在"})
		return
	}
	c.JSON(http.StatusOK, g)
}

// HandleCreateCompanyGoal POST /api/company/goals
func HandleCreateCompanyGoal(c *gin.Context) {
	var req struct {
		Objective     string `json:"objective"`
		SuccessMetric string `json:"successMetric"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || utf8.RuneCountInString(strings.TrimSpace(req.Objective)) < 4 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入至少 4 个字的公司目标"})
		return
	}
	// 目标创建时把当前标签方向记入目标（标签驱动调研方向）
	tags := loadTags()
	var tagNames []string
	for _, t := range tags {
		tagNames = append(tagNames, t.Name)
	}
	if len(tagNames) > 0 && !strings.Contains(req.Objective, "标签") {
		req.Objective = req.Objective + "（方向标签：" + strings.Join(tagNames, "、") + "）"
	}
	g := newContentGoal(req.Objective, req.SuccessMetric)
	g.Tags = tagNames
	companyWorkflowMu.Lock()
	err := saveCompanyGoal(g)
	companyWorkflowMu.Unlock()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	go runCompanyContentWorkflow(g.ID)
	c.JSON(http.StatusAccepted, g)
}

// HandleRunCompanyGoal POST /api/company/goals/:id/run
func HandleRunCompanyGoal(c *gin.Context) {
	companyWorkflowMu.Lock()
	g, err := loadCompanyGoal(c.Param("id"))
	if err == nil && g.Status == goalStatusFailed {
		g.Status = goalStatusActive
		g.FailureReason = ""
		for i := range g.Tasks {
			if g.Tasks[i].Status == workStatusFailed {
				g.Tasks[i].Status = workStatusRework
				g.Tasks[i].Attempt = 0
			}
		}
		g.addEvent("goal.resumed", "user", "用户要求重新运行失败任务", "")
		err = saveCompanyGoal(g)
	}
	companyWorkflowMu.Unlock()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "目标不存在"})
		return
	}
	if g.Status == goalStatusCompleted || g.Status == goalStatusAwaitingApproval {
		c.JSON(http.StatusConflict, gin.H{"error": "当前目标不需要运行"})
		return
	}
	go runCompanyContentWorkflow(g.ID)
	c.JSON(http.StatusAccepted, gin.H{"status": "running", "id": g.ID})
}

// HandleCompanyGoalDecision POST /api/company/goals/:id/decision
func HandleCompanyGoalDecision(c *gin.Context) {
	var req struct {
		Decision string `json:"decision"`
		Feedback string `json:"feedback"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误"})
		return
	}
	req.Decision = strings.ToLower(strings.TrimSpace(req.Decision))
	if req.Decision != "approve" && req.Decision != "reject" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "decision 必须是 approve 或 reject"})
		return
	}

	companyWorkflowMu.Lock()
	g, err := loadCompanyGoal(c.Param("id"))
	if err != nil {
		companyWorkflowMu.Unlock()
		c.JSON(http.StatusNotFound, gin.H{"error": "目标不存在"})
		return
	}
	if g.Status != goalStatusAwaitingApproval {
		companyWorkflowMu.Unlock()
		c.JSON(http.StatusConflict, gin.H{"error": "目标尚未进入用户终审"})
		return
	}
	approval := findCompanyTask(g, "human-approval")
	if req.Decision == "approve" {
		approval.Status = workStatusApproved
		approval.FinishedAt = time.Now()
		g.Status = goalStatusCompleted
		g.addEvent("goal.completed", "user", "用户终审批准，目标完成", approval.ID)
		err = saveCompanyGoal(g)
		companyWorkflowMu.Unlock()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, g)
		return
	}

	if strings.TrimSpace(req.Feedback) == "" {
		companyWorkflowMu.Unlock()
		c.JSON(http.StatusBadRequest, gin.H{"error": "退回时必须填写修改意见"})
		return
	}
	g.UserFeedback = strings.TrimSpace(req.Feedback)
	g.Status = goalStatusActive
	for _, id := range []string{"draft", "promotion", "human-approval"} {
		t := findCompanyTask(g, id)
		t.ArtifactID = ""
		t.FinishedAt = time.Time{}
		t.LastIssue = ""
		switch id {
		case "draft":
			t.Status = workStatusRework
			t.Attempt = 0
		default:
			t.Status = workStatusBlocked
			t.Attempt = 0
		}
	}
	g.addEvent("goal.rejected", "user", "用户退回修改："+g.UserFeedback, "draft")
	err = saveCompanyGoal(g)
	companyWorkflowMu.Unlock()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	go runCompanyContentWorkflow(g.ID)
	c.JSON(http.StatusAccepted, g)
}

// HandleCompanyArtifact GET /api/company/goals/:id/artifacts/:artifactId
func HandleCompanyArtifact(c *gin.Context) {
	companyWorkflowMu.Lock()
	defer companyWorkflowMu.Unlock()
	g, err := loadCompanyGoal(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "目标不存在"})
		return
	}
	content := artifactContent(g, c.Param("artifactId"))
	if content == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "交付物不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"content": content})
}
