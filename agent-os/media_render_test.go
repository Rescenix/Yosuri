package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsePPTMarkdown(t *testing.T) {
	source := parsePPTMarkdown("真实多 Agent", "## 为什么\n- 文本不等于交付\n- 要能运行\n## 怎么做\n- PPTX\n- MP4")
	if source.Topic != "真实多 Agent" || len(source.Slides) != 2 || len(source.Slides[0].Points) != 2 {
		t.Fatalf("unexpected source: %#v", source)
	}
}

func TestRenderPPTXWritesOfficeFile(t *testing.T) {
	if _, err := os.Stat(findRepoFile("main-frontend", "beneficial-belt", "node_modules", "pptxgenjs")); err != nil {
		t.Skip("pptxgenjs not installed")
	}
	dir := t.TempDir()
	name, err := renderPPTX(dir, "真实交付", "## 第一页\n- 成品必须可预览\n## 第二页\n- 成品必须可下载")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 4 || string(data[:2]) != "PK" {
		t.Fatalf("not an OOXML zip: %d bytes", len(data))
	}
}
