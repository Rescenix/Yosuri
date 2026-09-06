package handler

import "testing"

// TestProjectArtifactStageQAReport 质检报告不算交付阶段（不污染 stageSet），但可预览。
func TestProjectArtifactStageQAReport(t *testing.T) {
	cases := []struct {
		name  string
		stage string // 期望 stage（"" = 不算任何阶段）
		kind  string
	}{
		{"09-质量验收.qa.json", "", "text"},
		{"qa-report.json", "", "text"},
		{"质检报告.md", "", "text"},
		// 相邻产物不受影响
		{"07-发布.receipt", "promotion", "text"},
		{"05-项目路演.pptx", "ppt", "pptx"},
		{"00-需求计划.md", "requirements", "text"},
	}
	for _, c := range cases {
		if got := projectArtifactStage("", c.name); got != c.stage {
			t.Errorf("stage(%q) = %q, want %q", c.name, got, c.stage)
		}
		if got := projectPreviewKind(c.name); got != c.kind {
			t.Errorf("kind(%q) = %q, want %q", c.name, got, c.kind)
		}
	}
}
