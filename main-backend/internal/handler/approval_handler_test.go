package handler

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestInspectProjectDeliveryFindsRunnablePreviewAndStages(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"00-需求计划.md":    "# 需求\n完成一个可运行页面",
		"设计说明.md":       "# UI 设计\n采用响应式布局",
		"output-1.html": `<!doctype html><html><body><main>真实程序</main></body></html>`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	artifacts, preview, previewFile, previewKind, stages := inspectProjectDelivery("designer-04", dir, entries)
	if len(artifacts) != 3 {
		t.Fatalf("want 3 artifacts, got %d", len(artifacts))
	}
	if previewFile != "output-1.html" || previewKind != "html" || preview == "" {
		t.Fatalf("unexpected preview: file=%s kind=%s content=%q", previewFile, previewKind, preview)
	}
	for _, stage := range []string{"requirements", "ui", "code", "runnable"} {
		if !slices.Contains(stages, stage) {
			t.Fatalf("missing stage %s in %v", stage, stages)
		}
	}
}

func TestProjectDeliveryDoesNotCountScriptsAsRenderedMedia(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"PPT-旧产物.md": "# 只是大纲",
		"PV-旧产物.md":  "# 只是分镜",
		"deck.pptx":  "binary-placeholder",
		"launch.mp4": "binary-placeholder",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, _, previewFile, previewKind, stages := inspectProjectDelivery("promoter-01", dir, entries)
	if previewFile != "launch.mp4" || previewKind != "video" {
		t.Fatalf("want real video selected, got file=%s kind=%s", previewFile, previewKind)
	}
	if !slices.Contains(stages, "ppt") || !slices.Contains(stages, "pv") {
		t.Fatalf("real media stages missing: %v", stages)
	}
	if projectArtifactStage("promoter-01", "PV-旧产物.md") == "pv" || projectArtifactStage("promoter-01", "PPT-旧产物.md") == "ppt" {
		t.Fatal("markdown outline/script must not be classified as rendered media")
	}
}

func TestSpreadsheetIsASeparateProjectStage(t *testing.T) {
	if got := projectArtifactStage("researcher-01", "调研数据.xlsx"); got != "data" {
		t.Fatalf("want data stage, got %q", got)
	}
	if got := projectPreviewKind("调研数据.xlsx"); got != "spreadsheet" {
		t.Fatalf("want spreadsheet preview, got %q", got)
	}
}

func TestProjectDecisionCoversAllAgentCopies(t *testing.T) {
	records := []approvalRecord{{Agent: "writer-13", File: "project/001-智能创作工作台", Decision: "reject"}}
	files, projects := approvalDecisionIndex(records)
	if !files["writer-13|project/001-智能创作工作台"] {
		t.Fatal("exact historical record must remain decided")
	}
	if !projects["智能创作工作台"] {
		t.Fatal("one historical project decision must cover the normalized team project")
	}
	for _, file := range []string{"project/002-智能创作工作台", "project/006-智能创作工作台"} {
		if !projects[canonicalApprovalProject(file)] {
			t.Fatalf("same project copy reappeared: %s", file)
		}
	}
}

func TestExplicitProjectIdentityIsIndexed(t *testing.T) {
	_, projects := approvalDecisionIndex([]approvalRecord{{Agent: "coder-03", File: "project/001-other", Project: "智能创作工作台", Decision: "approve"}})
	if !projects["智能创作工作台"] {
		t.Fatal("explicit project identity must be persisted")
	}
}

func TestProjectDeliveryGateRequiresEveryHashedStage(t *testing.T) {
	dir := t.TempDir()
	gate := projectDeliveryGate{Status: "verified"}
	officeZip := func(part string) []byte {
		var buffer bytes.Buffer
		writer := zip.NewWriter(&buffer)
		file, _ := writer.Create(part)
		_, _ = file.Write([]byte("real"))
		_ = writer.Close()
		return buffer.Bytes()
	}
	app := []byte("<!doctype html><style>@media(max-width:800px){}</style><script>document.body.onclick=()=>1</script>")
	appSum := sha256.Sum256(app)
	fixtures := map[string]struct {
		name    string
		content []byte
	}{
		"meeting":      {"meeting.json", []byte(`{"participants":["researcher"]}`)},
		"research":     {"research.md", []byte("real research")},
		"data":         {"data.xlsx", officeZip("xl/workbook.xml")},
		"requirements": {"requirements.md", []byte("requirements")},
		"ui":           {"ui.html", []byte("<!doctype html><style>@media(max-width:800px){}</style>")},
		"docs":         {"docs.md", []byte("docs")},
		"code":         {"output-app.html", app},
		"runnable":     {"output-app.html", app},
		"ppt":          {"deck.pptx", officeZip("ppt/presentation.xml")},
		"pv":           {"launch.mp4", append([]byte{0, 0, 0, 24}, append([]byte("ftypisom"), make([]byte, 12)...)...)},
		"promotion":    {"publish.receipt", []byte(fmt.Sprintf("status=published\nchannel=local-preview\nentry=output-app.html\nentry_sha256=%x\n", appSum[:]))},
	}
	written := map[string]bool{}
	for _, stage := range projectStageOrder {
		fixture := fixtures[stage]
		name, content := fixture.name, fixture.content
		if !written[name] {
			if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
				t.Fatal(err)
			}
			written[name] = true
		}
		sum := sha256.Sum256(content)
		gate.Evidence = append(gate.Evidence, projectDeliveryEvidence{Stage: stage, File: name, SHA256: fmt.Sprintf("%x", sum[:])})
	}
	encoded, _ := json.Marshal(gate)
	if err := os.WriteFile(filepath.Join(dir, "delivery.manifest.json"), encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := verifyProjectDeliveryGate(dir); err != nil {
		t.Fatalf("valid gate rejected: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "launch.mp4"), []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := verifyProjectDeliveryGate(dir); err == nil {
		t.Fatal("tampered evidence must block approval")
	}
}
