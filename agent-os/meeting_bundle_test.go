package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCollectMeetingEvidenceUsesOneRealArtifactPerDepartment(t *testing.T) {
	company := t.TempDir()
	ceoHome := filepath.Join(company, "ceo-01")
	for _, item := range []struct{ agent, folder, file, content string }{
		{"researcher-02", "outputs", "调研.md", "# 调研\n真实资料"},
		{"designer-04", "outputs", "设计.md", "# UI\n组件规范"},
		{"coder-03", "projects/demo", "main.go", "package main"},
	} {
		dir := filepath.Join(company, item.agent, filepath.FromSlash(item.folder))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, item.file), []byte(item.content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(ceoHome, 0o755); err != nil {
		t.Fatal(err)
	}

	evidence := collectMeetingEvidence(ceoHome)
	if len(evidence) != 3 {
		t.Fatalf("want 3 departments, got %d: %#v", len(evidence), evidence)
	}
	if evidence[0].Role != "researcher" || evidence[1].Role != "designer" || evidence[2].Role != "coder" {
		t.Fatalf("unexpected role order: %#v", evidence)
	}
	if evidence[2].Source != "project/demo/main.go" {
		t.Fatalf("project evidence must be previewable, got %q", evidence[2].Source)
	}
}

func TestMeetingVTTBindsSpeechToEvidence(t *testing.T) {
	speeches := []meetingSpeech{{Order: 1, Agent: "writer-01", Department: "作者", Text: "交付文章。", Source: "文章.md", Start: "00:00:02.000", End: "00:00:09.000"}}
	vtt := buildMeetingVTT("周会", speeches)
	for _, want := range []string{"WEBVTT", "writer-01", "交付文章。", "证据：文章.md", "00:00:02.000 --> 00:00:09.000"} {
		if !strings.Contains(vtt, want) {
			t.Fatalf("VTT missing %q:\n%s", want, vtt)
		}
	}
}
