package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteProjectResearchXLSXCreatesPreviewableOOXML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "00-需求计划.md"), []byte("真实需求"), 0o644); err != nil {
		t.Fatal(err)
	}
	name, err := writeProjectResearchXLSX(dir, "视觉工作台")
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.OpenReader(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("not a valid xlsx zip: %v", err)
	}
	defer reader.Close()
	parts := map[string]bool{}
	for _, file := range reader.File {
		parts[file.Name] = true
	}
	for _, required := range []string{"[Content_Types].xml", "xl/workbook.xml", "xl/worksheets/sheet1.xml", "xl/styles.xml"} {
		if !parts[required] {
			t.Fatalf("xlsx missing %s", required)
		}
	}
}

func TestRunnablePrototypeContainsRealInteraction(t *testing.T) {
	html := projectPrototypeHTML("视觉工作台", "做一个真正可运行的产品", true)
	for _, expected := range []string{"<!doctype html>", "<script>", ".onclick", "VERIFIED MILESTONES"} {
		if !strings.Contains(html, expected) {
			t.Fatalf("prototype missing %q", expected)
		}
	}
}
