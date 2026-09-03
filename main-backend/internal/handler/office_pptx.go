package handler

// office_pptx.go — 纯 Go 生成 .pptx（PowerPoint）。
// OOXML 幻灯片本质也是 zip + XML：每页一个 slideN.xml，标题+要点用文本框。

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
)

// genPptx 生成 .pptx 字节流。title 为封面标题，slides 为内容页。
func genPptx(title string, slides []officeSlide) ([]byte, error) {
	if len(slides) == 0 {
		// 没有显式 slides 时用空页兜底，保证文件结构合法
		slides = []officeSlide{{Title: title, Bullets: []string{}}}
	}

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

	if err := write("[Content_Types].xml", pptxContentTypes(len(slides))); err != nil {
		return nil, err
	}
	if err := write("_rels/.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
</Relationships>`); err != nil {
		return nil, err
	}
	if err := write("ppt/presentation.xml", pptxPresentationXML(title, len(slides))); err != nil {
		return nil, err
	}
	if err := write("ppt/_rels/presentation.xml.rels", pptxPresentationRels(len(slides))); err != nil {
		return nil, err
	}

	// 每页 slide + 对应 rels
	for i := range slides {
		idx := i + 1
		s := slides[i]
		if err := write(fmt.Sprintf("ppt/slides/slide%d.xml", idx), pptxSlideXML(idx, s.Title, s.Bullets)); err != nil {
			return nil, err
		}
		if err := write(fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", idx), pptxSlideRels(idx)); err != nil {
			return nil, err
		}
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// pptxContentTypes 声明所有 part 的 content type。
func pptxContentTypes(slideCount int) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
<Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/>
<Override PartName="/ppt/slideLayouts/slideLayout1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>`)
	for i := 1; i <= slideCount; i++ {
		sb.WriteString(fmt.Sprintf(`
<Override PartName="/ppt/slides/slide%d.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>`, i))
	}
	sb.WriteString("\n</Types>")
	return sb.String()
}

// pptxPresentationXML 演示文稿根：每页一个 sldId 引用。
func pptxPresentationXML(title string, slideCount int) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:presentation xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
<p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst>
<p:sldIdLst>`)
	for i := 1; i <= slideCount; i++ {
		sb.WriteString(fmt.Sprintf(`<p:sldId id="%d" r:id="rId%d"/>`, 256+i, i+1))
	}
	sb.WriteString(`</p:sldIdLst>
<p:sldSz cx="12192000" cy="6858000"/>
<p:notesSz cx="6858000" cy="9144000"/>
</p:presentation>`)
	return sb.String()
}

// pptxPresentationRels presentation.xml 的 rels：master + 各 slide。
func pptxPresentationRels(slideCount int) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="slideMasters/slideMaster1.xml"/>`)
	for i := 1; i <= slideCount; i++ {
		sb.WriteString(fmt.Sprintf(`
<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide%d.xml"/>`, i+1, i))
	}
	sb.WriteString("\n</Relationships>")
	return sb.String()
}

// pptxSlideXML 单页：亮蓝白设计版（背景浅蓝 + 顶部主色标题条 + 左侧装饰条 + 要点卡片 + 页脚页码）。
func pptxSlideXML(idx int, slideTitle string, bullets []string) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
<p:cSld><p:spTree>`)

	// 背景：浅蓝 #F5F8FF
	sb.WriteString(`<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>`)
	sb.WriteString(`<p:sp><p:nvSpPr><p:cNvPr id="2" name="bg"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="12192000" cy="6858000"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:solidFill><a:srgbClr val="FFFFFF"/></a:solidFill></p:spPr><p:txBody><a:bodyPr/><a:lstStyle/><a:p/></p:txBody></p:sp>`)

	// 顶部标题条：#1950BE + 白字标题。长标题（带时间戳的项目名）自动降字号 + 条内换行垂直居中，避免溢出截断。
	titleSz := 3400
	if n := len([]rune(slideTitle)); n > 24 {
		titleSz = 2400
	}
	if n := len([]rune(slideTitle)); n > 44 {
		titleSz = 2000
	}
	sb.WriteString(fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="3" name="titlebar"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="12192000" cy="1120000"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:solidFill><a:srgbClr val="1950BE"/></a:solidFill></p:spPr><p:txBody><a:bodyPr wrap="square" anchor="ctr" lIns="360000" rIns="360000"/><a:lstStyle/><a:p><a:pPr algn="l"/><a:r><a:rPr lang="zh-CN" sz="%d" b="1"><a:solidFill><a:srgbClr val="FFFFFF"/></a:solidFill></a:rPr><a:t>`, titleSz))
	sb.WriteString(xmlEscape(slideTitle))
	sb.WriteString(`</a:t></a:r></a:p></p:txBody></p:sp>`)

	// 要点列表（bullet •，自动换行不截断）—— markdown 文本渲染
	shapeID := 5 // 供页脚 shape 使用
	sb.WriteString(`<p:sp><p:nvSpPr><p:cNvPr id="4" name="要点"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="457200" y="1500000"/><a:ext cx="11277600" cy="4900000"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr><p:txBody><a:bodyPr wrap="square"><a:lstStyle/>`)
	for _, b := range bullets {
		sb.WriteString(`<a:p><a:buChar char="•"/><a:r><a:rPr lang="zh-CN" sz="1800"><a:solidFill><a:srgbClr val="333333"/></a:solidFill></a:rPr><a:t>`)
		sb.WriteString(xmlEscape(b))
		sb.WriteString(`</a:t></a:r></a:p>`)
	}
	sb.WriteString(`</p:txBody></p:sp>`)

	// 页脚：左下项目名 + 右下页码
	sb.WriteString(fmt.Sprintf(`<p:sp><p:nvSpPr><p:cNvPr id="%d" name="footer"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="720000" y="6340000"/><a:ext cx="11400000" cy="400000"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr><p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr lang="zh-CN" sz="1200"><a:solidFill><a:srgbClr val="9AA9C0"/></a:solidFill></a:rPr><a:t>Rescene AI 公司 · 第 %d 页</a:t></a:r></a:p></p:txBody></p:sp>`, shapeID, idx))

	sb.WriteString(`</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sld>`)
	return sb.String()
}

// pptxSlideRels 每页 slide 的 rels（引用版式）。
func pptxSlideRels(idx int) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
</Relationships>`
}
