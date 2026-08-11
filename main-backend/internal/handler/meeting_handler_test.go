package handler

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadMeetingManifestKeepsMissingMediaEmpty(t *testing.T) {
	dir := t.TempDir()
	minutes := "会议-demo.md"
	if err := os.WriteFile(filepath.Join(dir, minutes), []byte("# 会议"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := `{"id":"demo","topic":"周会","kind":"ai_reconstruction","minutesFile":"会议-demo.md","transcriptFile":"会议-demo.vtt","speeches":[{"order":1,"agent":"writer-01","department":"作者","text":"已交付","source":"文章.md"}]}`
	if err := os.WriteFile(filepath.Join(dir, "会议-demo.meeting.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	meeting, ok := readMeetingManifest(dir, "ceo-01", "会议-demo.meeting.json")
	if !ok {
		t.Fatal("manifest should load")
	}
	if meeting.PPTFile != "" || meeting.ReplayFile != "" {
		t.Fatalf("missing media must stay empty: %#v", meeting)
	}
	if meeting.TranscriptFile != "会议-demo.vtt" || len(meeting.Speeches) != 1 {
		t.Fatalf("unexpected response: %#v", meeting)
	}
}

func TestProjectReviewMeetingUsesVerifiedProjectMediaAndDepartmentEvidence(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "delivery.manifest.json")
	if err := os.WriteFile(manifest, []byte(`{"status":"verified"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	gate := projectDeliveryGate{
		Project: "完整演示项目", Status: "verified", GeneratedAt: "2026-08-11T12:00:00+08:00",
		Evidence: []projectDeliveryEvidence{
			{Stage: "meeting", ProducerRole: "ceo", File: "00-项目会议.meeting.json"},
			{Stage: "research", ProducerRole: "researcher", File: "01-调研报告.md", Verification: "调研已核验"},
			{Stage: "ui", ProducerRole: "designer", File: "03-UI原型.html", Verification: "UI 已渲染"},
			{Stage: "runnable", ProducerRole: "coder", File: "output-app.html", Verification: "程序可交互"},
			{Stage: "ppt", ProducerRole: "promoter", File: "05-项目路演.pptx"},
			{Stage: "pv", ProducerRole: "promoter", File: "06-宣传PV.mp4", Verification: "横屏产品演示"},
			{Stage: "promotion", ProducerRole: "publisher", File: "07-发布.receipt", Verification: "发布已回验"},
		},
	}
	meeting, ok := projectReviewMeeting("coder-03", "900-demo", dir, gate)
	if !ok {
		t.Fatal("verified project should become a review meeting")
	}
	if meeting.Kind != "project_review" || meeting.PPTFile != "project/900-demo/05-项目路演.pptx" || meeting.ReplayFile != "project/900-demo/06-宣传PV.mp4" {
		t.Fatalf("unexpected project review media: %#v", meeting)
	}
	if len(meeting.Speeches) != 5 || meeting.Speeches[0].Department != "研究部" || meeting.Speeches[0].Source != "project/900-demo/01-调研报告.md" {
		t.Fatalf("department evidence was not mapped: %#v", meeting.Speeches)
	}
}
