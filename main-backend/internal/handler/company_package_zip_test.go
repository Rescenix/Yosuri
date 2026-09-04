package handler

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestCompanyZipProject 拿磁盘上真实存在的项目目录验证整包能打出、能解压、含清单。
func TestCompanyZipProject(t *testing.T) {
	root := filepath.Join(companyDir(), "coder-03", "projects")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Skip("无历史项目目录，跳过")
	}
	var target string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		if _, err := os.Stat(filepath.Join(dir, "delivery.manifest.json")); err == nil {
			target = dir
			break
		}
	}
	if target == "" {
		t.Skip("没有含交付清单的项目，跳过")
	}
	data, err := companyZipProject(target)
	if err != nil {
		t.Fatalf("打包失败: %v", err)
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip 非法: %v", err)
	}
	names := map[string]bool{}
	for _, f := range reader.File {
		names[f.Name] = true
	}
	if !names["delivery.manifest.json"] {
		t.Fatalf("包内缺交付清单，实际: %v", names)
	}
	if !names["output-app.html"] && !names["03-UI原型.html"] {
		t.Fatalf("包内缺可运行产物，实际: %v", names)
	}
	t.Logf("项目=%s 包大小=%dB 文件数=%d", filepath.Base(target), len(data), len(names))
	for n := range names {
		t.Log("  ", n)
	}
}
