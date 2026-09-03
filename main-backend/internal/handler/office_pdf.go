package handler

// office_pdf.go — 纯 Go 生成 .pdf。
// 用 go-pdf/fpdf 渲染中文：从系统字体目录找中文字体（simhei.ttf 黑体等），
// 嵌入 PDF 保证跨机器打开不乱码。找不到字体时回退纯 ASCII（不崩溃）。

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/go-pdf/fpdf"
)

// pdfFontPaths 候选中文字体（按优先级）。
var pdfFontPaths = []string{
	"/c/Windows/Fonts/simhei.ttf",   // 黑体
	"C:/Windows/Fonts/simhei.ttf",   // 黑体（Windows 原生路径）
	"/Windows/Fonts/simhei.ttf",     // MSYS 风格
	"/usr/share/fonts/truetype/droid/DroidSansFallbackFull.ttf",
	"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
	"/System/Library/Fonts/PingFang.ttc",
}

// findCJKFont 探测系统里第一个存在的中文字体文件。
func findCJKFont() string {
	// 直接探测候选
	for _, p := range pdfFontPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// Windows: 遍历 Fonts 目录找常见中文字体
	if runtime.GOOS == "windows" {
		fontsDir := filepath.Join(os.Getenv("WINDIR"), "Fonts")
		if entries, err := os.ReadDir(fontsDir); err == nil {
			for _, e := range entries {
				n := strings.ToLower(e.Name())
				if strings.Contains(n, "simhei") || strings.Contains(n, "msyh") ||
					strings.Contains(n, "simsun") || strings.Contains(n, "notosanscjk") ||
					strings.Contains(n, "yahei") {
					return filepath.Join(fontsDir, e.Name())
				}
			}
		}
	}
	return ""
}

// genPdf 生成 PDF 字节流。
func genPdf(title string, blocks []officeBlock) ([]byte, error) {
	fontPath := findCJKFont()
	hasCJK := fontPath != ""

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(20, 18, 20)
	pdf.AddPage()

	fontFamily := "helvetica"
	if hasCJK {
		fontFamily = "cjk"
		// AddUTF8FontFromBytes 需要字体文件字节；ttc（集合字体）fpdf 支持有限，
		// 直接尝试注册，失败则回退 ASCII。
		data, err := os.ReadFile(fontPath)
		if err == nil {
			// 同一字体注册常规 + 粗体两个样式（fpdf 按 family+style 组合索引）
			pdf.AddUTF8FontFromBytes(fontFamily, "", data)
			pdf.AddUTF8FontFromBytes(fontFamily, "B", data)
			// fpdf 内部粗体走合成：注册 B 后所有 "B" 引用可解析
		} else {
			hasCJK = false
			fontFamily = "helvetica"
		}
	}

	if title != "" {
		pdf.SetFont(fontFamily, "B", 18)
		pdf.SetTextColor(43, 58, 103)
		pdf.CellFormat(0, 12, title, "", 1, "C", false, 0, "")
		pdf.Ln(4)
	}

	bodyText := func(s string) string {
		if hasCJK {
			return s
		}
		// 无中文字体：剥掉非 ASCII，避免乱码
		var sb strings.Builder
		for _, r := range s {
			if r < 128 {
				sb.WriteRune(r)
			}
		}
		return sb.String()
	}

	for _, b := range blocks {
		switch b.Type {
		case "heading":
			sizes := map[int]float64{1: 15, 2: 13, 3: 11.5}
			sz := sizes[b.Level]
			if sz == 0 {
				sz = 13
			}
			pdf.SetFont(fontFamily, "B", sz)
			pdf.SetTextColor(43, 58, 103)
			pdf.MultiCell(0, sz/3+4, bodyText(b.Text), "", "L", false)
			pdf.Ln(2)
		case "paragraph":
			pdf.SetFont(fontFamily, "", 11)
			pdf.SetTextColor(40, 40, 40)
			pdf.MultiCell(0, 6, bodyText(b.Text), "", "L", false)
			pdf.Ln(1)
		case "bullets":
			pdf.SetFont(fontFamily, "", 11)
			pdf.SetTextColor(40, 40, 40)
			for _, item := range b.Items {
				pdf.SetX(24)
				pdf.CellFormat(6, 6, "•", "", 0, "R", false, 0, "")
				pdf.MultiCell(0, 6, bodyText(item), "", "L", false)
			}
			pdf.Ln(1)
		case "table":
			pdfTable(pdf, fontFamily, b, bodyText)
		}
	}

	// 输出到 bytes.Buffer 再取字节
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("PDF 输出失败: %w", err)
	}
	return buf.Bytes(), nil
}

// pdfTable 用 fpdf 画简单表格（表头深色 + 行分隔线）。
func pdfTable(pdf *fpdf.Fpdf, fontFamily string, b officeBlock, bodyText func(string) string) {
	headers := b.Headers
	rows := b.Rows
	if len(headers) == 0 && len(rows) == 0 {
		return
	}
	cols := len(headers)
	for _, r := range rows {
		if len(r) > cols {
			cols = len(r)
		}
	}
	if cols == 0 {
		return
	}
	width := 210.0 - 40.0 // A4 宽 210，左右 margin 各 20
	colW := width / float64(cols)
	lineH := 7.0

	drawRow := func(cells []string, bold bool, fill bool) {
		y := pdf.GetY()
		if y+lineH > 280 { // A4 高 297 - 底边距
			pdf.AddPage()
			y = pdf.GetY()
		}
		x := 20.0
		for c := 0; c < cols; c++ {
			text := ""
			if c < len(cells) {
				text = cells[c]
			}
			if fill {
				pdf.SetFillColor(232, 232, 240)
			}
			pdf.Rect(x, y, colW, lineH, "F")
			pdf.SetXY(x+1.5, y+1.2)
			if bold {
				pdf.SetFont(fontFamily, "B", 9)
			} else {
				pdf.SetFont(fontFamily, "", 9)
			}
			pdf.MultiCell(colW-3, lineH-2.4, bodyText(text), "", "L", false)
			x += colW
		}
		// 底线
		pdf.SetDrawColor(180, 180, 190)
		pdf.Line(20, y+lineH, 20+width, y+lineH)
		pdf.SetY(y + lineH)
	}

	if len(headers) > 0 {
		drawRow(headers, true, true)
	}
	for _, r := range rows {
		drawRow(r, false, false)
	}
	pdf.Ln(4)
}

var _ = fmt.Sprintf
