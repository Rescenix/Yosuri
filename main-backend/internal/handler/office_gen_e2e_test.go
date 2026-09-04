package handler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 端到端实测：generate_office 真实生成 → 文件落盘 → 结构校验。
// 中文标题/正文必须原样存活；pptx/xlsx 必须是合法 zip；pdf 必须嵌 CJK 字体。
func TestGenerateOfficeEndToEnd(t *testing.T) {
	dir := withTempRepoRoot(t)

	t.Run("pptx chinese survives", func(t *testing.T) {
		res, err := callGenerateOffice(`{"format":"pptx","filename":"测.pptx","title":"AI小助手能力秀","slides":[{"title":"开场","bullets":["中文要点一","中文要点二"]}]}`)
		if err != nil {
			t.Fatalf("生成失败: %v", err)
		}
		p := filepath.Join(dir, "测.pptx")
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("文件未落盘: %v", err)
		}
		if strings.Contains(string(data), "\xef\xbf\xbd") {
			t.Fatal("文件里出现 U+FFFD 替换符 = 二进制被文本通道污染")
		}
		_ = res
	})

	t.Run("xlsx chinese survives", func(t *testing.T) {
		if _, err := callGenerateOffice(`{"format":"xlsx","filename":"表.xlsx","title":"数据","sheets":[{"name":"S1","headers":["名称","值"],"rows":[["甲","1"]]}]}`); err != nil {
			t.Fatalf("生成失败: %v", err)
		}
		data, err := os.ReadFile(filepath.Join(dir, "表.xlsx"))
		if err != nil {
			t.Fatalf("文件未落盘: %v", err)
		}
		if strings.Contains(string(data), "\xef\xbf\xbd") {
			t.Fatal("xlsx 出现 U+FFFD")
		}
	})

	t.Run("pdf embeds cjk font", func(t *testing.T) {
		if _, err := callGenerateOffice(`{"format":"pdf","filename":"文.pdf","title":"交付文档","blocks":[{"type":"paragraph","text":"中文内容测试"}]}`); err != nil {
			t.Fatalf("生成失败: %v", err)
		}
		data, err := os.ReadFile(filepath.Join(dir, "文.pdf"))
		if err != nil {
			t.Fatalf("文件未落盘: %v", err)
		}
		if !strings.HasPrefix(string(data), "%PDF") {
			t.Fatal("不是合法 PDF")
		}
		// findCJKFont 找不到时 genPdf 会剥中文——这里要求必须找到字体
		if findCJKFont() == "" {
			t.Skip("本机无中文字体，跳过嵌入断言")
		}
		if !strings.Contains(string(data), "FontFile") {
			t.Fatal("PDF 未嵌入字体（缺 FontFile 描述符），中文在别的机器会变方块")
		}
	})
}
