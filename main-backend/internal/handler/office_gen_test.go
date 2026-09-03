package handler

// office_gen_test.go — 测试 office 文件生成器。
// 每种格式生成一个示例文件，验证 zip 结构/PDF 头部/文本内容。

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"backend/internal/ai/core"
)

func TestGenDocx(t *testing.T) {
	blocks := []officeBlock{
		{Type: "heading", Text: "第一章", Level: 1},
		{Type: "paragraph", Text: "这是正文内容，包含中文。这是正文内容，包含中文。"},
		{Type: "bullets", Items: []string{"第一项", "第二项", "第三项"}},
		{Type: "table", Headers: []string{"姓名", "年龄", "城市"}, Rows: [][]string{{"张三", "28", "北京"}, {"李四", "32", "上海"}}},
	}
	data, err := genDocx("测试文档", blocks)
	if err != nil {
		t.Fatalf("genDocx 失败: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("genDocx 返回空数据")
	}

	// 验证是合法 zip
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("不是合法 zip: %v", err)
	}
	found := false
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			found = true
			rc, _ := f.Open()
			buf := new(bytes.Buffer)
			buf.ReadFrom(rc)
			rc.Close()
			content := buf.String()
			if !strings.Contains(content, "测试文档") {
				t.Error("word/document.xml 缺少标题")
			}
			if !strings.Contains(content, "正文内容") {
				t.Error("word/document.xml 缺少正文")
			}
			if !strings.Contains(content, "张三") {
				t.Error("word/document.xml 缺少表格数据")
			}
		}
	}
	if !found {
		t.Error("docx 中没有 word/document.xml")
	}
}

func TestGenPptx(t *testing.T) {
	slides := []officeSlide{
		{Title: "需求分析", Bullets: []string{"用户调研", "竞品分析", "需求优先级"}},
		{Title: "技术方案", Bullets: []string{"架构设计", "数据库选型", "部署方案"}},
		{Title: "实施计划", Bullets: []string{"第一周", "第二周", "第三周"}},
	}
	data, err := genPptx("项目汇报", slides)
	if err != nil {
		t.Fatalf("genPptx 失败: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("genPptx 返回空数据")
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("不是合法 zip: %v", err)
	}
	slideCount := 0
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml") {
			slideCount++
			rc, _ := f.Open()
			buf := new(bytes.Buffer)
			buf.ReadFrom(rc)
			rc.Close()
			content := buf.String()
			if !strings.Contains(content, "需求分析") && !strings.Contains(content, "技术方案") && !strings.Contains(content, "实施计划") {
				t.Errorf("slide %s 缺少预期标题", f.Name)
			}
		}
	}
	if slideCount != 3 {
		t.Errorf("期望 3 页幻灯片，实际 %d", slideCount)
	}
}

func TestGenXlsx(t *testing.T) {
	sheets := []officeSheet{
		{
			Name:    "用户表",
			Headers: []string{"ID", "姓名", "邮箱"},
			Rows: [][]string{
				{"1", "张三", "zhang@example.com"},
				{"2", "李四", "li@example.com"},
			},
		},
		{
			Name:    "订单表",
			Headers: []string{"订单号", "金额", "状态"},
			Rows: [][]string{
				{"ORD001", "100", "已完成"},
				{"ORD002", "200", "处理中"},
			},
		},
	}
	data, err := genXlsx("测试工作簿", sheets)
	if err != nil {
		t.Fatalf("genXlsx 失败: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("genXlsx 返回空数据")
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("不是合法 zip: %v", err)
	}

	found := map[string]bool{}
	for _, f := range zr.File {
		found[f.Name] = true
	}
	if !found["xl/workbook.xml"] {
		t.Error("缺少 xl/workbook.xml")
	}
	if !found["xl/worksheets/sheet1.xml"] {
		t.Error("缺少 xl/worksheets/sheet1.xml")
	}
	if !found["xl/worksheets/sheet2.xml"] {
		t.Error("缺少 xl/worksheets/sheet2.xml")
	}
	if !found["xl/sharedStrings.xml"] {
		t.Error("缺少 xl/sharedStrings.xml")
	}

	// 验证 sharedStrings 包含中文
	zr2, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("重新打开 zip 失败: %v", err)
	}
	rc, _ := zr2.Open("xl/sharedStrings.xml")
	buf := new(bytes.Buffer)
	buf.ReadFrom(rc)
	rc.Close()
	ss := buf.String()
	for _, want := range []string{"张三", "李四", "ORD001"} {
		if !strings.Contains(ss, want) {
			t.Errorf("sharedStrings 缺少 %q", want)
		}
	}
}

func TestGenPdf(t *testing.T) {
	blocks := []officeBlock{
		{Type: "heading", Text: "第一章 引言", Level: 1},
		{Type: "paragraph", Text: "这是 PDF 正文。包含中文测试。"},
		{Type: "bullets", Items: []string{"要点一", "要点二", "要点三"}},
		{Type: "table", Headers: []string{"项目", "数值"}, Rows: [][]string{{"A", "100"}, {"B", "200"}}},
	}
	data, err := genPdf("测试 PDF 文档", blocks)
	if err != nil {
		t.Fatalf("genPdf 失败: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("genPdf 返回空数据")
	}
	// 验证 PDF 头部
	header := string(data[:5])
	if header != "%PDF-" {
		t.Errorf("PDF 头部异常: %q", header)
	}
	// 验证包含文本
	raw := string(data)
	if !strings.Contains(raw, "PDF") {
		t.Error("PDF 内容缺少 PDF 标识")
	}
}

func TestCallGenerateOfficeDocx(t *testing.T) {
	// 模拟 agent 工具调用
	args := `{"format":"docx","filename":"test_交付.docx","title":"测试报告","blocks":[{"type":"heading","text":"摘要","level":1},{"type":"paragraph","text":"本报告由 agent 自动生成。"}]}`
	result, err := callGenerateOffice(args)
	if err != nil {
		t.Fatalf("callGenerateOffice 失败: %v", err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("期望 1 个交付文件，实际 %d", len(result.Files))
	}
	f := result.Files[0]
	if f.Ext != ".docx" {
		t.Errorf("扩展名期望 .docx，实际 %s", f.Ext)
	}
	if f.Size <= 0 {
		t.Error("文件大小应为正数")
	}
	// 确认文件已落盘可读
	info, err := os.Stat(filepath.Join(core.GetProjectRoot(), f.Path))
	if err != nil {
		t.Fatalf("落盘文件不可读: %v", err)
	}
	if info.Size() != f.Size {
		t.Errorf("stat 大小 %d 与 deliverable 大小 %d 不一致", info.Size(), f.Size)
	}
	// 清理
	os.Remove(filepath.Join(core.GetProjectRoot(), f.Path))
}

func TestCallGenerateOfficePptx(t *testing.T) {
	args := `{"format":"pptx","filename":"报告.pptx","title":"项目汇报","slides":[{"title":"概述","bullets":["目标","范围"]},{"title":"方案","bullets":["架构","技术选型"]}]}`
	result, err := callGenerateOffice(args)
	if err != nil {
		t.Fatalf("callGenerateOffice pptx 失败: %v", err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("期望 1 个交付文件，实际 %d", len(result.Files))
	}
	if result.Files[0].Ext != ".pptx" {
		t.Errorf("扩展名期望 .pptx，实际 %s", result.Files[0].Ext)
	}
	// 清理
	os.Remove(filepath.Join(core.GetProjectRoot(), result.Files[0].Path))
}

func TestCallGenerateOfficeXlsx(t *testing.T) {
	args := `{"format":"xlsx","filename":"数据.xlsx","title":"销售数据","sheets":[{"name":"Sheet1","headers":["月份","金额"],"rows":[["一月","100"],["二月","200"]]}]}`
	result, err := callGenerateOffice(args)
	if err != nil {
		t.Fatalf("callGenerateOffice xlsx 失败: %v", err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("期望 1 个交付文件，实际 %d", len(result.Files))
	}
	if result.Files[0].Ext != ".xlsx" {
		t.Errorf("扩展名期望 .xlsx，实际 %s", result.Files[0].Ext)
	}
	os.Remove(filepath.Join(core.GetProjectRoot(), result.Files[0].Path))
}

func TestCallGenerateOfficePdf(t *testing.T) {
	args := `{"format":"pdf","filename":"报告.pdf","title":"PDF 报告","blocks":[{"type":"heading","text":"第一章","level":1},{"type":"paragraph","text":"这是 PDF 正文。"}]}`
	result, err := callGenerateOffice(args)
	if err != nil {
		t.Fatalf("callGenerateOffice pdf 失败: %v", err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("期望 1 个交付文件，实际 %d", len(result.Files))
	}
	if result.Files[0].Ext != ".pdf" {
		t.Errorf("扩展名期望 .pdf，实际 %s", result.Files[0].Ext)
	}
	os.Remove(filepath.Join(core.GetProjectRoot(), result.Files[0].Path))
}

func TestCallGenerateOfficeErrors(t *testing.T) {
	// 空格式
	_, err := callGenerateOffice(`{"format":""}`)
	if err == nil {
		t.Error("空格式应返回错误")
	}
	// 不支持格式
	_, err = callGenerateOffice(`{"format":"txt"}`)
	if err == nil {
		t.Error("不支持格式应返回错误")
	}
	// 扩展名不匹配
	_, err = callGenerateOffice(`{"format":"docx","filename":"x.pdf"}`)
	if err == nil {
		t.Error("扩展名不匹配应返回错误")
	}
	// 路径穿越
	_, err = callGenerateOffice(`{"format":"docx","filename":"../../etc/passwd.docx"}`)
	if err == nil {
		t.Error("路径穿越应返回错误")
	}
}

// TestWriteOfficeSamples 受控导出 4 种格式样本到固定目录，
// 供 Python 第三方库（python-docx/pptx/openpyxl/pypdf）真实验证。
// 只在设置 WRITE_OFFICE_SAMPLES=1 时运行（与 dump_prompt_test.go 同模式）。
func TestWriteOfficeSamples(t *testing.T) {
	if os.Getenv("WRITE_OFFICE_SAMPLES") != "1" {
		t.Skip("未设置 WRITE_OFFICE_SAMPLES，跳过样本导出")
	}
	dir := `C:/Pro2026/GIFS/office_verify`
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	docx, err := genDocx("测试 Word 文档", []officeBlock{
		{Type: "heading", Text: "第一章 概述", Level: 1},
		{Type: "paragraph", Text: "这是用纯 Go 生成的 docx 正文，包含中文内容。"},
		{Type: "bullets", Items: []string{"要点一", "要点二", "要点三"}},
		{Type: "table", Headers: []string{"姓名", "城市"}, Rows: [][]string{{"张三", "北京"}, {"李四", "上海"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(dir+"/sample.docx", docx, 0o644)

	pptx, err := genPptx("项目汇报", []officeSlide{
		{Title: "需求分析", Bullets: []string{"用户调研", "竞品分析"}},
		{Title: "技术方案", Bullets: []string{"架构设计", "部署方案"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(dir+"/sample.pptx", pptx, 0o644)

	xlsx, err := genXlsx("销售数据", []officeSheet{
		{Name: "月度", Headers: []string{"月份", "金额"}, Rows: [][]string{{"一月", "100"}, {"二月", "200"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(dir+"/sample.xlsx", xlsx, 0o644)

	pdf, err := genPdf("测试 PDF 文档", []officeBlock{
		{Type: "heading", Text: "第一章 引言", Level: 1},
		{Type: "paragraph", Text: "这是纯 Go 生成的 PDF 正文，包含中文。"},
	})
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(dir+"/sample.pdf", pdf, 0o644)

	t.Log("样本已写入 " + dir)
}