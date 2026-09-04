package handler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readMultiForTest(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("读 %s 失败: %v", name, err)
	}
	return string(data)
}

// TestCompanyMultiRealRun 真实跑一条「记账本」指令，验证多文件工程落地：
// 4 个源文件齐全、内联版可预览、数据层真用 localStorage。
func TestCompanyMultiRealRun(t *testing.T) {
	if testing.Short() {
		t.Skip("-short 跳过真实生产")
	}
	projectName := "901-直播自检-记账本"
	dir, err := deliveryBuildProject(projectName, "做一个记账本，能记录每笔收支、按月份分类、显示月度合计，数据要保存下来刷新不丢")
	if err != nil {
		t.Fatalf("真实生产失败: %v", err)
	}
	t.Logf("目录=%s", dir)
	for _, f := range []string{"index.html", "app.js", "styles.css", "data.js", "output-app.html"} {
		if len(readMultiForTest(t, dir, f)) < 20 {
			t.Errorf("文件 %s 内容过短", f)
		}
	}
	data := readMultiForTest(t, dir, "data.js")
	if !strings.Contains(data, "localStorage") {
		t.Error("data.js 没用 localStorage——数据层是假的")
	}
	app := readMultiForTest(t, dir, "app.js")
	if !strings.Contains(app, "AppData") {
		t.Error("app.js 没接 AppData——逻辑层与数据层脱节")
	}
	inline := readMultiForTest(t, dir, "output-app.html")
	if strings.Contains(inline, `src="data.js"`) || strings.Contains(inline, `href="styles.css"`) {
		t.Error("内联版仍残留外链，预览会白屏")
	}
	t.Logf("index=%dB app.js=%dB data.js=%dB 内联=%dB",
		len(readMultiForTest(t, dir, "index.html")), len(app), len(data), len(inline))
}
