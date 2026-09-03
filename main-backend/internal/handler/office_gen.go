package handler

// office_gen.go — 纯 Go 原生 office 文件生成器。
// 零 Python 依赖，零外部库（OOXML 用标准库 archive/zip + 手写 XML），
// PDF 用 go-pdf/fpdf + 系统字体（simhei.ttf）支持中文。
// 编译进 Wails exe，用户开箱即用，无需下载任何东西。

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"backend/internal/ai/core"
)

// officeBlock 统一块模型，用于 docx/pdf 的段落/列表/标题/表格。
type officeBlock struct {
	Type    string     `json:"type"`             // heading|paragraph|bullets|table
	Text    string     `json:"text,omitempty"`    // heading/paragraph 的文本
	Level   int        `json:"level,omitempty"`   // heading 级别 1-3
	Items   []string   `json:"items,omitempty"`   // bullets 列表项
	Headers []string   `json:"headers,omitempty"` // table 表头
	Rows    [][]string `json:"rows,omitempty"`    // table 行数据
}

// officeSlide pptx 幻灯片
type officeSlide struct {
	Title   string   `json:"title"`
	Bullets []string `json:"bullets"`
}

// officeSheet xlsx 工作表
type officeSheet struct {
	Name    string   `json:"name,omitempty"`
	Headers []string `json:"headers,omitempty"`
	Rows    [][]string `json:"rows,omitempty"`
}

// officeContent 统一输入结构
type officeContent struct {
	Format   string        `json:"format"`   // docx|pptx|xlsx|pdf
	Filename string        `json:"filename,omitempty"`
	Title    string        `json:"title,omitempty"`
	Blocks   []officeBlock `json:"blocks,omitempty"`   // docx/pdf
	Slides   []officeSlide `json:"slides,omitempty"`   // pptx
	Sheets   []officeSheet `json:"sheets,omitempty"`   // xlsx
}

// generateOfficeToolDef generate_office 工具定义
var generateOfficeToolDef = core.ToolDefinition{
	Type: "function",
	Function: core.ToolFunctionDetail{
		Name:        "generate_office",
		Description: "生成办公文档(PDF/DOCX/PPTX/XLSX)，纯内置生成，无需任何外部库。输入结构化 JSON，输出文件直接出现在聊天交付卡片。format 必填(docx/pptx/xlsx/pdf)；filename 可选(默认自动生成)；title 文档标题；blocks 是通用块列表(支持 heading/paragraph/bullets/table)；slides 是 pptx 专用(每页标题+要点)；sheets 是 xlsx 专用(表名+表头+行数据)。",
		Parameters: core.ToolParameters{
			Type: "object",
			Properties: map[string]core.ToolProperty{
				"format":   {Type: "string", Description: "文件格式：docx/pptx/xlsx/pdf"},
				"filename": {Type: "string", Description: "可选文件名(含扩展名)，默认 交付-时间戳.{format}"},
				"title":    {Type: "string", Description: "文档/演示/工作簿标题"},
				"blocks":   {Type: "array", Description: "docx/pdf 的内容块：[{type:heading|paragraph|bullets|table, text, level, items, headers, rows}]"},
				"slides":   {Type: "array", Description: "pptx 幻灯片：[{title, bullets}]"},
				"sheets":   {Type: "array", Description: "xlsx 工作表：[{name, headers, rows}]"},
			},
			Required: []string{"format"},
		},
	},
}

// callGenerateOffice 执行 generate_office 工具
func callGenerateOffice(argsJSON string) (nativeToolResult, error) {
	var args struct {
		Format   string        `json:"format"`
		Filename string        `json:"filename"`
		Title    string        `json:"title"`
		Blocks   []officeBlock `json:"blocks"`
		Slides   []officeSlide `json:"slides"`
		Sheets   []officeSheet `json:"sheets"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return nativeToolResult{}, fmt.Errorf("参数解析失败: %w", err)
	}

	format := strings.ToLower(strings.TrimSpace(args.Format))
	if format == "" {
		return nativeToolResult{}, fmt.Errorf("format 必填(docx/pptx/xlsx/pdf)")
	}

	valid := map[string]bool{"docx": true, "pptx": true, "xlsx": true, "pdf": true}
	if !valid[format] {
		return nativeToolResult{}, fmt.Errorf("不支持的格式: %s（支持 docx/pptx/xlsx/pdf）", format)
	}

	filename := strings.TrimSpace(args.Filename)
	if filename == "" {
		filename = fmt.Sprintf("交付-%d.%s", time.Now().UnixMilli(), format)
	}
	if !strings.HasSuffix(strings.ToLower(filename), "."+format) {
		return nativeToolResult{}, fmt.Errorf("文件名扩展名必须与 format 一致（.%s）", format)
	}
	if strings.Contains(filename, "/") || strings.Contains(filename, "\\") || strings.Contains(filename, "..") {
		return nativeToolResult{}, fmt.Errorf("文件名不能包含路径分隔符或 ..")
	}

	absPath := filepath.Join(core.GetProjectRoot(), filename)
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return nativeToolResult{}, fmt.Errorf("创建目录失败: %w", err)
	}

	var data []byte
	var err error

	switch format {
	case "docx":
		data, err = genDocx(args.Title, args.Blocks)
	case "pptx":
		data, err = genPptx(args.Title, args.Slides)
	case "xlsx":
		data, err = genXlsx(args.Title, args.Sheets)
	case "pdf":
		data, err = genPdf(args.Title, args.Blocks)
	}
	if err != nil {
		return nativeToolResult{}, fmt.Errorf("生成失败: %w", err)
	}

	if err := os.WriteFile(absPath, data, 0o644); err != nil {
		return nativeToolResult{}, fmt.Errorf("写入文件失败: %w", err)
	}

	return nativeToolResult{
		Text:  fmt.Sprintf("已生成 %s（%d 字节）", displayNativePath(absPath), len(data)),
		Files: deliverableFromPath(absPath),
	}, nil
}