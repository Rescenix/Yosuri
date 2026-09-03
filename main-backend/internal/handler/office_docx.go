package handler

// office_docx.go — 纯 Go 生成 .docx（Word）。
// OOXML 本质是 zip 包 + XML 部件，用标准库即可，无需外部依赖。

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
)

// genDocx 生成 .docx 字节流。title 为文档标题，blocks 为内容块。
func genDocx(title string, blocks []officeBlock) ([]byte, error) {
	var body strings.Builder

	if title != "" {
		body.WriteString("<w:p><w:pPr><w:pStyle w:val=\"Title\"/></w:pPr><w:r><w:t>")
		body.WriteString(xmlEscape(title))
		body.WriteString("</w:t></w:r></w:p>")
	}

	for _, b := range blocks {
		switch b.Type {
		case "heading":
			lvl := b.Level
			if lvl < 1 || lvl > 3 {
				lvl = 1
			}
			style := []string{"Heading1", "Heading2", "Heading3"}[lvl-1]
			body.WriteString("<w:p><w:pPr><w:pStyle w:val=\"" + style + "\"/></w:pPr><w:r><w:t>")
			body.WriteString(xmlEscape(b.Text))
			body.WriteString("</w:t></w:r></w:p>")
		case "paragraph":
			body.WriteString("<w:p><w:r><w:t>")
			body.WriteString(xmlEscape(b.Text))
			body.WriteString("</w:t></w:r></w:p>")
		case "bullets":
			for _, item := range b.Items {
				body.WriteString("<w:p><w:pPr><w:numPr><w:ilvl w:val=\"0\"/><w:numId w:val=\"1\"/></w:numPr></w:pPr><w:r><w:t>")
				body.WriteString(xmlEscape(item))
				body.WriteString("</w:t></w:r></w:p>")
			}
		case "table":
			body.WriteString(docxTableXML(b))
		}
	}

	document := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:body>` + body.String() + `<w:sectPr><w:pgSz w:w="11906" w:h="16838"/></w:sectPr></w:body></w:document>`

	contentTypes := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
</Types>`

	rels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="word/styles.xml"/>
</Relationships>`

	styles := docxStylesXML()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	files := map[string]string{
		"[Content_Types].xml": contentTypes,
		"_rels/.rels":         rels,
		"word/document.xml":   document,
		"word/styles.xml":     styles,
	}
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			return nil, fmt.Errorf("zip 创建 %s 失败: %w", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			return nil, fmt.Errorf("zip 写入 %s 失败: %w", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("zip 关闭失败: %w", err)
	}
	return buf.Bytes(), nil
}

// docxStylesXML 最小 styles.xml：标题/正文样式 + 编号列表定义。
func docxStylesXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:ascii="Microsoft YaHei" w:eastAsia="Microsoft YaHei"/><w:sz w:val="21"/></w:rPr></w:rPrDefault></w:docDefaults>
<w:style w:type="paragraph" w:styleId="Title"><w:name w:val="Title"/><w:basedOn w:val="Normal"/><w:pPr><w:jc w:val="center"/><w:spacing w:after="240"/></w:pPr><w:rPr><w:sz w:val="44"/><w:b/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/><w:basedOn w:val="Normal"/><w:pPr><w:spacing w:before="240" w:after="120"/></w:pPr><w:rPr><w:sz w:val="32"/><w:b/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="Heading2"><w:name w:val="heading 2"/><w:basedOn w:val="Normal"/><w:pPr><w:spacing w:before="200" w:after="100"/></w:pPr><w:rPr><w:sz w:val="28"/><w:b/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="Heading3"><w:name w:val="heading 3"/><w:basedOn w:val="Normal"/><w:pPr><w:spacing w:before="160" w:after="80"/></w:pPr><w:rPr><w:sz w:val="24"/><w:b/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="Normal"><w:name w:val="Normal"/></w:style>
<w:numbering><w:abstractNum w:abstractNumId="0"><w:lvl w:ilvl="0"><w:numFmt w:val="bullet"/><w:lvlText w:val="•"/></w:lvl></w:abstractNum><w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num></w:numbering>
</w:styles>`
}

// docxTableXML 生成 w:tbl 表格 XML。
func docxTableXML(b officeBlock) string {
	var sb strings.Builder
	sb.WriteString(`<w:tbl><w:tblPr><w:tblW w:w="0" w:type="auto"/><w:tblBorders><w:top w:val="single" w:sz="4"/><w:left w:val="single" w:sz="4"/><w:bottom w:val="single" w:sz="4"/><w:right w:val="single" w:sz="4"/><w:insideH w:val="single" w:sz="4"/><w:insideV w:val="single" w:sz="4"/></w:tblBorders></w:tblPr>`)

	if len(b.Headers) > 0 {
		sb.WriteString("<w:tr>")
		for _, h := range b.Headers {
			sb.WriteString(`<w:tc><w:tcPr><w:tcW w:w="0" w:type="auto"/><w:shd w:val="clear" w:fill="E8E8F0"/></w:tcPr><w:p><w:r><w:rPr><w:b/></w:rPr><w:t>`)
			sb.WriteString(xmlEscape(h))
			sb.WriteString("</w:t></w:r></w:p></w:tc>")
		}
		sb.WriteString("</w:tr>")
	}
	for _, row := range b.Rows {
		sb.WriteString("<w:tr>")
		for _, cell := range row {
			sb.WriteString(`<w:tc><w:tcPr><w:tcW w:w="0" w:type="auto"/></w:tcPr><w:p><w:r><w:t>`)
			sb.WriteString(xmlEscape(cell))
			sb.WriteString("</w:t></w:r></w:p></w:tc>")
		}
		sb.WriteString("</w:tr>")
	}
	sb.WriteString("</w:tbl>")
	return sb.String()
}

// xmlEscape 转义 XML 特殊字符。
func xmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return r.Replace(s)
}
