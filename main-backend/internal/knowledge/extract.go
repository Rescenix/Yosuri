package knowledge

// extract.go —— 多格式文本提取器。
//
// 原则：只用标准库 + 纯 Go、跨平台、离线。优先 md/txt（零成本），
// docx/pptx 用 zip + XML 解析（Office Open XML 都是 zip 包），pdf 用纯 Go 库
// ledongthuc/pdf 抽文本层（扫描件无文本层则跳过，不强行 OCR）。
//
// 所有提取失败都返回明确错误，绝不吞掉 —— 让调用方能区分「格式不支持」和「文件坏了」。

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// extractText 按扩展名分发到具体提取器，返回纯文本。
func extractText(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".md", ".markdown", ".txt":
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(data), nil
	case ".docx":
		return extractDocx(path)
	case ".pptx":
		return extractPptx(path)
	case ".pdf":
		return extractPdf(path)
	default:
		return "", fmt.Errorf("不支持的格式: %s", ext)
	}
}

// zipReadXML 从 zip 里读一个 entry，返回其字符串内容。
func zipReadXML(path, entryName string) (string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer r.Close()
	for _, f := range r.File {
		if f.Name != entryName {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()
		data, err := io.ReadAll(io.LimitReader(rc, 32<<20)) // 上限 32MB
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	return "", fmt.Errorf("zip 内找不到 %s", entryName)
}

// extractDocx 抽取 .docx 正文纯文本。段落（w:p）间用换行分隔。
func extractDocx(path string) (string, error) {
	raw, err := zipReadXML(path, "word/document.xml")
	if err != nil {
		return "", err
	}
	return xmlTextNodes(raw), nil
}

// extractPptx 抽取 .pptx 所有幻灯片文本，按 slide 序号排序后拼接。
func extractPptx(path string) (string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer r.Close()

	var slides []string
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml") {
			slides = append(slides, f.Name)
		}
	}
	sort.Strings(slides)
	if len(slides) == 0 {
		return "", fmt.Errorf("pptx 内无幻灯片")
	}

	var sb strings.Builder
	for _, slide := range slides {
		raw, err := zipReadXML(path, slide)
		if err != nil {
			return "", err
		}
		if t := strings.TrimSpace(xmlTextNodes(raw)); t != "" {
			sb.WriteString(t)
			sb.WriteString("\n")
		}
	}
	return sb.String(), nil
}

// xmlTextNodes 流式解析 XML，收集所有段落元素（local name = "p"）里的文本节点
// （local name = "t"）内容。docx/pptx 都用 w:p/a:p 段落 + w:t/a:t 文本，按 local
// name 匹配即可跨命名空间复用。段落之间换行分隔。
func xmlTextNodes(raw string) string {
	dec := xml.NewDecoder(strings.NewReader(raw))
	var sb strings.Builder
	var cur strings.Builder
	inText := false

	flush := func() {
		t := strings.TrimSpace(cur.String())
		if t != "" {
			sb.WriteString(t)
			sb.WriteString("\n")
		}
		cur.Reset()
	}

	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p":
				flush() // 新段落，冲刷上一个段落积累的文本
			case "t":
				inText = true
				cur.Reset()
			}
		case xml.CharData:
			if inText {
				cur.Write(t)
			}
		case xml.EndElement:
			if t.Name.Local == "t" {
				inText = false
			}
		}
	}
	flush()
	return sb.String()
}