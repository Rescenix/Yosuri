package handler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewContentGoalBuildsBlockedTaskDAG(t *testing.T) {
	g := newContentGoal("解释多 Agent 公司为何失败", "文章可以直接公开发布")
	if g.Status != goalStatusActive || g.Workflow != "content_studio_v1" {
		t.Fatalf("unexpected goal: status=%s workflow=%s", g.Status, g.Workflow)
	}
	if len(g.Tasks) != 4 {
		t.Fatalf("want 4 tasks, got %d", len(g.Tasks))
	}
	if g.Tasks[0].Status != workStatusReady {
		t.Fatalf("research should be ready, got %s", g.Tasks[0].Status)
	}
	for _, task := range g.Tasks[1:] {
		if task.Status != workStatusBlocked {
			t.Fatalf("task %s should start blocked, got %s", task.ID, task.Status)
		}
	}
}

func TestReconcileCompanyTasksUnlocksOnlyApprovedDependencies(t *testing.T) {
	g := newContentGoal("写一篇多 Agent 文章", "通过终审")
	reconcileCompanyTasks(g)
	if got := findCompanyTask(g, "draft").Status; got != workStatusBlocked {
		t.Fatalf("draft unlocked before research approval: %s", got)
	}

	findCompanyTask(g, "research").Status = workStatusApproved
	reconcileCompanyTasks(g)
	if got := findCompanyTask(g, "draft").Status; got != workStatusReady {
		t.Fatalf("draft not unlocked: %s", got)
	}
	if got := findCompanyTask(g, "promotion").Status; got != workStatusBlocked {
		t.Fatalf("promotion unlocked too early: %s", got)
	}

	findCompanyTask(g, "draft").Status = workStatusApproved
	findCompanyTask(g, "promotion").Status = workStatusApproved
	reconcileCompanyTasks(g)
	if got := findCompanyTask(g, "human-approval").Status; got != workStatusWaitingHuman {
		t.Fatalf("human gate not opened: %s", got)
	}
	if g.Status != goalStatusAwaitingApproval {
		t.Fatalf("goal should await approval, got %s", g.Status)
	}
}

func TestParseReviewerDecision(t *testing.T) {
	decision, err := parseReviewerDecision("```json\n{\"verdict\":\"approve\",\"score\":87,\"issues\":[],\"summary\":\"通过\"}\n```")
	if err != nil {
		t.Fatal(err)
	}
	if decision.Verdict != "approve" || decision.Score != 87 {
		t.Fatalf("unexpected decision: %+v", decision)
	}
	if _, err := parseReviewerDecision(`{"verdict":"maybe","score":80}`); err == nil {
		t.Fatal("invalid verdict should fail")
	}
}

func TestCompanyGoalPersistenceAndArtifactInputs(t *testing.T) {
	root := t.TempDir()
	t.Setenv("RESCENE_COMPANY_WORKFLOW_DIR", root)
	g := newContentGoal("写一篇解释 Agent 协作的文章", "形成可发布正文")
	if err := saveCompanyGoal(g); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadCompanyGoal(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Objective != g.Objective || len(loaded.Tasks) != 4 {
		t.Fatalf("persistence mismatch: %+v", loaded)
	}

	research := findCompanyTask(loaded, "research")
	research.Attempt = 1
	artifact, err := writeCompanyArtifact(loaded, research, strings.Repeat("研究内容", 100))
	if err != nil {
		t.Fatal(err)
	}
	artifact.Accepted = true
	loaded.Artifacts = append(loaded.Artifacts, artifact)
	research.ArtifactID = artifact.ID
	research.Status = workStatusApproved
	draft := findCompanyTask(loaded, "draft")
	input := taskInputs(loaded, draft)
	if !strings.Contains(input, "研究内容") || !strings.Contains(input, "已验收交付物") {
		t.Fatalf("upstream artifact missing from task input: %s", input)
	}
	if _, err := os.Stat(filepath.Join(root, loaded.ID, filepath.FromSlash(artifact.Path))); err != nil {
		t.Fatalf("artifact file missing: %v", err)
	}
}

func TestProductionPromptIncludesReworkReason(t *testing.T) {
	g := newContentGoal("写一篇解释 Agent 协作的文章", "形成可发布正文")
	task := findCompanyTask(g, "research")
	task.LastIssue = "缺少事实边界；没有待核实项"
	prompt := productionPrompt(g, task)
	for _, want := range []string{"本轮必须逐项修复", "缺少事实边界", "没有待核实项"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}
}

func TestContentWorkflowRunsThroughReviewsAndStopsAtHumanGate(t *testing.T) {
	t.Setenv("RESCENE_COMPANY_WORKFLOW_DIR", t.TempDir())
	originalCall := companyModelCall
	t.Cleanup(func() { companyModelCall = originalCall })
	companyModelCall = func(prompt string) (string, error) {
		if strings.Contains(prompt, "只输出 JSON") {
			return `{"verdict":"approve","score":91,"issues":[],"summary":"符合工单验收标准"}`, nil
		}
		return strings.Repeat("这是符合工单要求、区分事实与判断且不会自动发布的正式交付内容。", 30), nil
	}

	g := newContentGoal("解释多 Agent 公司应该如何协作", "形成可以公开发布的完整内容包")
	if err := saveCompanyGoal(g); err != nil {
		t.Fatal(err)
	}
	runCompanyContentWorkflow(g.ID)

	got, err := loadCompanyGoal(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != goalStatusAwaitingApproval {
		t.Fatalf("workflow must stop at human gate, got %s (%s)", got.Status, got.FailureReason)
	}
	for _, id := range []string{"research", "draft", "promotion"} {
		if task := findCompanyTask(got, id); task == nil || task.Status != workStatusApproved {
			t.Fatalf("task %s was not approved: %+v", id, task)
		}
	}
	if approval := findCompanyTask(got, "human-approval"); approval.Status != workStatusWaitingHuman {
		t.Fatalf("human approval should be waiting, got %s", approval.Status)
	}
	if len(got.Artifacts) != 3 || len(got.Reviews) != 3 {
		t.Fatalf("want 3 artifacts and reviews, got %d and %d", len(got.Artifacts), len(got.Reviews))
	}
}
