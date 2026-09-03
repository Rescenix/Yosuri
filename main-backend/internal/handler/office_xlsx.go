package handler

// office_xlsx.go — 纯 Go 生成 .xlsx（Excel）。
// 每张 sheet 是一个 worksheet.xml，共享字符串表 (sharedStrings.xml) 记文本。

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// genXlsx 生成 .xlsx 字节流。title 为工作簿标题，sheets 为工作表列表。
func genXlsx(title string, sheets []officeSheet) ([]byte, error) {
	if len(sheets) == 0 {
		sheets = []officeSheet{{Name: title, Headers: nil, Rows: nil}}
	}
	if len(sheets) == 0 {
		sheets = []officeSheet{{Name: "Sheet1", Rows: [][]string{{"（空）"}}}}
	}

	// 收集所有独特字符串，构建 sharedStrings
	ssMap := map[string]int{}
	ssList := []string{}
	addSS := func(s string) int {
		if idx, ok := ssMap[s]; ok {
			return idx
		}
		idx := len(ssList)
		ssList = append(ssList, s)
		ssMap[s] = idx
		return idx
	}

	// 预扫描：头 + 行
	sheetCellRefs := make([][][]int, len(sheets)) // [sheet][row][col]
	for si, sh := range sheets {
		nRows := len(sh.Rows)
		if len(sh.Headers) > 0 {
			nRows++
		}
		refs := make([][]int, nRows)
		ri := 0
		if len(sh.Headers) > 0 {
			row := make([]int, len(sh.Headers))
			for ci, h := range sh.Headers {
				row[ci] = addSS(h)
			}
			refs[ri] = row
			ri++
		}
		for _, r := range sh.Rows {
			row := make([]int, len(r))
			for ci, v := range r {
				row[ci] = addSS(v)
			}
			refs[ri] = row
			ri++
		}
		sheetCellRefs[si] = refs
	}

	// 构建 sharedStrings.xml
	var ssXML strings.Builder
	ssXML.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="` + strconv.Itoa(len(ssList)) + `" uniqueCount="` + strconv.Itoa(len(ssList)) + `">`)
	for _, s := range ssList {
		ssXML.WriteString("<si><t>" + xmlEscape(s) + "</t></si>")
	}
	ssXML.WriteString("</sst>")

	// 构建每个 sheet 的 worksheet.xml
	sheetXMLs := make([]string, len(sheets))
	for si, sh := range sheets {
		refs := sheetCellRefs[si]
		var wb strings.Builder
		cols := 0
		for _, row := range refs {
			if len(row) > cols {
				cols = len(row)
			}
		}
		wb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<sheetViews><sheetView tabSelected="` + boolStr(si == 0) + `" workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews>`)
		if cols > 0 {
			wb.WriteString("<cols>")
			for c := 0; c < cols; c++ {
				wb.WriteString(`<col min="` + strconv.Itoa(c+1) + `" max="` + strconv.Itoa(c+1) + `" width="25" customWidth="1"/>`)
			}
			wb.WriteString("</cols>")
		}
		wb.WriteString("<sheetData>")
		for ri, row := range refs {
			wb.WriteString(`<row r="` + strconv.Itoa(ri+1) + `">`)
			for ci, ssi := range row {
				colLetter := colLetters(ci)
				cellRef := colLetter + strconv.Itoa(ri+1)
				style := "0"
				if ri == 0 && len(sh.Headers) > 0 {
					style = "1" // 加粗表头样式
				}
				wb.WriteString(`<c r="` + cellRef + `" t="s" s="` + style + `"><v>` + strconv.Itoa(ssi) + `</v></c>`)
			}
			wb.WriteString("</row>")
		}
		wb.WriteString("</sheetData></worksheet>")
		sheetXMLs[si] = wb.String()
	}

	// 构建 workbook.xml
	var wbXML strings.Builder
	wbXML.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<sheets>`)
	for si, sh := range sheets {
		name := sh.Name
		if name == "" {
			name = fmt.Sprintf("Sheet%d", si+1)
		}
		wbXML.WriteString(`<sheet name="` + xmlEscape(name) + `" sheetId="` + strconv.Itoa(si+1) + `" r:id="rId` + strconv.Itoa(si+1) + `"/>`)
	}
	wbXML.WriteString("</sheets></workbook>")

	// styles.xml（带加粗表头样式）
	stylesXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<fonts count="2"><font><sz val="11"/><name val="Microsoft YaHei"/></font><font><b/><sz val="11"/><name val="Microsoft YaHei"/></font></fonts>
<fills count="2"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill></fills>
<borders count="1"><border><left/><right/><top/><bottom/><diagonal/></border></borders>
<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>
<cellXfs count="2"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/><xf numFmtId="0" fontId="1" fillId="0" borderId="0" applyFont="1"/></cellXfs>
</styleSheet>`

	// 构建 zip
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	write := func(name, content string) error {
		w, err := zw.Create(name)
		if err != nil {
			return fmt.Errorf("zip 创建 %s 失败: %w", name, err)
		}
		_, err = w.Write([]byte(content))
		return err
	}

	// [Content_Types].xml
	var ct strings.Builder
	ct.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
<Override PartName="/xl/sharedStrings.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml"/>
<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>
</Types>`)
	if err := write("[Content_Types].xml", ct.String()); err != nil {
		return nil, err
	}
	if err := write("_rels/.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`); err != nil {
		return nil, err
	}
	if err := write("xl/_rels/workbook.xml.rels", xlsxWorkbookRels(len(sheets))); err != nil {
		return nil, err
	}
	if err := write("xl/workbook.xml", wbXML.String()); err != nil {
		return nil, err
	}
	for si := range sheets {
		if err := write(fmt.Sprintf("xl/worksheets/sheet%d.xml", si+1), sheetXMLs[si]); err != nil {
			return nil, err
		}
	}
	if err := write("xl/sharedStrings.xml", ssXML.String()); err != nil {
		return nil, err
	}
	if err := write("xl/styles.xml", stylesXML); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// xlsxWorkbookRels workbook.xml 的 rels：各 sheet + sharedStrings + styles。
func xlsxWorkbookRels(sheetCount int) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	for i := 1; i <= sheetCount; i++ {
		sb.WriteString(fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet%d.xml"/>`, i, i))
	}
	sb.WriteString(`<Relationship Id="rId` + strconv.Itoa(sheetCount+1) + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings" Target="sharedStrings.xml"/>
<Relationship Id="rId` + strconv.Itoa(sheetCount+2) + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`)
	return sb.String()
}

// colLetters 把列索引转成 A-Z, AA-ZZ 等。
func colLetters(i int) string {
	s := ""
	for i >= 0 {
		s = string(rune('A'+(i%26))) + s
		i = i/26 - 1
	}
	return s
}

// boolStr true → "1"，false → "0"。
func boolStr(v bool) string {
	if v {
		return "1"
	}
	return "0"
}