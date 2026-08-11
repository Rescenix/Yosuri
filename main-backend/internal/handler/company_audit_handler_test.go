package handler

import "testing"

func TestDepartmentAuditRequiresExpectedBinaryEvidence(t *testing.T) {
	files := []string{"调研报告.md", "数据.csv"}
	if !hasAnyExtension(files, ".xlsx", ".csv", ".tsv") {
		t.Fatal("CSV should count as structured spreadsheet evidence")
	}
	if hasAnyExtension([]string{"PV-脚本.md"}, ".mp4", ".webm") {
		t.Fatal("video script must not count as rendered video")
	}
}
