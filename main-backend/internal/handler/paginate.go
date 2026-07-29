package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

type PaginateRequest struct {
	Text       string  `json:"text" binding:"required"`
	FontSize   float64 `json:"fontSize" binding:"required"`
	PageWidth  float64 `json:"pageWidth" binding:"required"`
	PageHeight float64 `json:"pageHeight" binding:"required"`
}

type PaginateResponse struct {
	Pages []string `json:"pages"`
}

var htmlEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
)

// ---------- 精确字符宽度表 (单位: 相对于 1px 字体大小) ----------
// 实际宽度 = fontSize * widthFactor
// 以下数据基于 Inter 字体 + Noto Sans SC 在 16px 下的近似值
var (
	// 汉字 / 全角符号
	fullWidthFactor = 1.0
	// 英文小写字母平均宽度
	lowerWidthFactor = 0.52
	// 英文大写字母平均宽度
	upperWidthFactor = 0.68
	// 数字
	digitWidthFactor = 0.55
	// 空格
	spaceWidthFactor = 0.28
	// 常见标点 (半角)
	punctuationWidth = 0.35
)

func getCharWidth(r rune, fontSize float64) float64 {
	switch {
	case r >= 0x4E00 && r <= 0x9FFF: // 常用汉字
		return fontSize * fullWidthFactor
	case r == ' ':
		return fontSize * spaceWidthFactor
	case r >= '0' && r <= '9':
		return fontSize * digitWidthFactor
	case r >= 'a' && r <= 'z':
		return fontSize * lowerWidthFactor
	case r >= 'A' && r <= 'Z':
		return fontSize * upperWidthFactor
	case strings.ContainsRune("，。！？；：“”‘’、", r):
		// 中文标点全角
		return fontSize * fullWidthFactor
	case strings.ContainsRune(",.!?;:()[]{}", r):
		return fontSize * punctuationWidth
	default:
		// 其他字符：按全角处理
		return fontSize * fullWidthFactor
	}
}

// 计算字符串精确宽度
func measureWidth(s string, fontSize float64) float64 {
	var width float64
	for _, r := range s {
		width += getCharWidth(r, fontSize)
	}
	return width
}

// 禁止出现在行首的标点 (需要挪到上一行行尾)
var lineStartForbidden = map[rune]bool{
	'。': true, '，': true, '！': true, '？': true, '；': true, '：': true,
	'”': true, '’': true, '》': true, '】': true, '』': true, '、': true,
	'.': true, ',': true, '!': true, '?': true, ';': true, ':': true,
	')': true, ']': true, '}': true,
}

// 检查一个字符串是否以禁止行首的字符开头
func startsWithForbidden(s string) bool {
	if s == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(s)
	return lineStartForbidden[r]
}

// ---------- 高质量流式分页 (对标 Pretext) ----------
func highQualityPaginate(req PaginateRequest) []string {
	text := req.Text
	fontSize := req.FontSize
	pageWidth := req.PageWidth
	pageHeight := req.PageHeight

	lineHeight := fontSize * 1.8
	pad := 24.0
	contentWidth := pageWidth - pad*2
	contentHeight := pageHeight - pad*2
	maxLinesPerPage := int(contentHeight / lineHeight)
	if maxLinesPerPage < 1 {
		maxLinesPerPage = 1
	}

	indentWidth := fontSize * 2 // 首行缩进两个汉字的宽度
	indentStr := "　　"

	type lineInfo struct {
		text   string
		indent bool // 是否需要首行缩进
	}

	var allLines []lineInfo
	paragraphs := strings.Split(text, "\n")

	for _, para := range paragraphs {
		if para == "" {
			allLines = append(allLines, lineInfo{text: "", indent: false})
			continue
		}
		trimmed := strings.TrimLeft(para, " \t\r")
		if trimmed == "" {
			allLines = append(allLines, lineInfo{text: "", indent: false})
			continue
		}

		remaining := trimmed
		isFirstLine := true

		for len(remaining) > 0 {
			maxWidth := contentWidth
			if isFirstLine {
				maxWidth = contentWidth - indentWidth
			}
			// 二分查找最长可容纳的字符数
			low, high := 0, len(remaining)
			best := 0
			for low <= high {
				mid := (low + high) / 2
				if mid == 0 {
					best = 0
					break
				}
				w := measureWidth(remaining[:mid], fontSize)
				if w <= maxWidth {
					best = mid
					low = mid + 1
				} else {
					high = mid - 1
				}
			}
			if best == 0 {
				best = 1
			}
			// 获取当前行候选文本
			candidate := remaining[:best]
			cutPos := best

			// 英文单词保护：如果切在了单词中间，回退到前一个空格
			if best < len(remaining) && remaining[best] != ' ' && !unicode.IsSpace(rune(remaining[best])) {
				lastSpace := strings.LastIndex(candidate, " ")
				if lastSpace > 0 {
					candidate = candidate[:lastSpace]
					cutPos = lastSpace + 1
				}
			}

			// 标点避头尾：检查下一行的首字符是否不允许出现在行首
			nextStart := cutPos
			for nextStart < len(remaining) && remaining[nextStart] == ' ' {
				nextStart++
			}
			if nextStart < len(remaining) && startsWithForbidden(remaining[nextStart:]) {
				// 把禁止标点挪到本行末尾
				// 找到第一个不是禁止标点的位置
				origCut := cutPos
				for cutPos < len(remaining) && startsWithForbidden(remaining[cutPos:]) {
					cutPos += utf8.RuneLen(rune(remaining[cutPos]))
				}
				if cutPos > origCut {
					candidate = remaining[:cutPos]
				}
				nextStart = cutPos
				for nextStart < len(remaining) && remaining[nextStart] == ' ' {
					nextStart++
				}
			}

			lineText := strings.TrimRight(candidate, " \t")
			allLines = append(allLines, lineInfo{
				text:   lineText,
				indent: isFirstLine,
			})
			isFirstLine = false
			remaining = remaining[nextStart:]
		}
	}

	// 移除开头的连续空行
	for len(allLines) > 0 && allLines[0].text == "" {
		allLines = allLines[1:]
	}

	// 分页 + 孤行控制
	var pages []string
	idx := 0
	total := len(allLines)

	for idx < total {
		end := idx + maxLinesPerPage
		if end > total {
			end = total
		}
		// 孤行控制：如果当前页最后一行是段落首行且不是唯一行，则回退一行
		if end < total && end > idx && allLines[end].indent {
			// 将段首行移到下一页
			end--
		}
		if end == idx { // 安全保护
			end = idx + 1
		}

		pageLines := allLines[idx:end]
		var textParts []string
		for _, line := range pageLines {
			if line.indent {
				textParts = append(textParts, indentStr+line.text)
			} else {
				textParts = append(textParts, line.text)
			}
		}
		pageText := strings.Join(textParts, "\n")
		escaped := htmlEscaper.Replace(pageText)
		html := fmt.Sprintf(`<div style="
			width: 100%%; height: 100%%;
			padding: %.0fpx;
			box-sizing: border-box;
			font-family: 'Inter', system-ui, 'Noto Sans SC', sans-serif;
			font-size: %.0fpx;
			line-height: 1.8;
			white-space: pre-wrap;
			word-wrap: break-word;
			text-align: justify;
			overflow: hidden;
		">%s</div>`, pad, fontSize, escaped)
		pages = append(pages, html)
		idx = end
	}
	return pages
}

// ---------- 极速分页（超大文本回退，保持原有逻辑）---------
func fastPaginate(req PaginateRequest) []string {
	text := req.Text
	fontSize := req.FontSize
	pageWidth := req.PageWidth
	pageHeight := req.PageHeight

	lineHeight := fontSize * 1.8
	pad := 24.0
	contentWidth := pageWidth - pad*2
	contentHeight := pageHeight - pad*2
	maxLinesPerPage := int(contentHeight / lineHeight)

	charsPerLine := int(contentWidth / (fontSize * 0.55))
	indentStr := "　　"

	type line struct {
		text   string
		indent bool
	}

	var lines []line
	paragraphs := strings.Split(text, "\n")

	for _, para := range paragraphs {
		if para == "" {
			lines = append(lines, line{"", false})
			continue
		}
		trimmed := strings.TrimLeft(para, " \t\r")
		if trimmed == "" {
			lines = append(lines, line{"", false})
			continue
		}
		var paraLines []string
		for len(trimmed) > 0 {
			end := charsPerLine
			if end > len(trimmed) {
				end = len(trimmed)
			}
			paraLines = append(paraLines, trimmed[:end])
			trimmed = trimmed[end:]
		}
		if len(paraLines) > 0 {
			paraLines[0] = indentStr + paraLines[0]
			lines = append(lines, line{paraLines[0], true})
			for i := 1; i < len(paraLines); i++ {
				lines = append(lines, line{paraLines[i], false})
			}
		}
	}

	for len(lines) > 0 && lines[0].text == "" {
		lines = lines[1:]
	}

	var pages []string
	for len(lines) > 0 {
		end := maxLinesPerPage
		if end > len(lines) {
			end = len(lines)
		}
		pageLines := lines[:end]
		lines = lines[end:]

		var parts []string
		for _, l := range pageLines {
			parts = append(parts, l.text)
		}
		pageText := strings.Join(parts, "\n")
		escaped := htmlEscaper.Replace(pageText)
		html := fmt.Sprintf(`<div style="
			width: 100%%; height: 100%%;
			padding: %.0fpx;
			box-sizing: border-box;
			font-family: 'Inter', system-ui, 'Noto Sans SC', sans-serif;
			font-size: %.0fpx;
			line-height: 1.8;
			white-space: pre-wrap;
			word-wrap: break-word;
			text-align: justify;
			overflow: hidden;
		">%s</div>`, pad, fontSize, escaped)
		pages = append(pages, html)
	}
	return pages
}

// ---------- HTTP 接口 ----------
func Paginate(c *gin.Context) {
	var req PaginateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	// 选择分页算法
	var pages []string
	textSize := len(req.Text)
	if textSize > 500_000 {
		fmt.Printf("[INFO] 超大文本 (%d bytes)，启用极速分页\n", textSize)
		pages = fastPaginate(req)
	} else {
		done := make(chan []string, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("[WARN] 高质量分页崩溃: %v\n", r)
					done <- nil
				}
			}()
			done <- highQualityPaginate(req)
		}()
		select {
		case p := <-done:
			if p == nil {
				pages = fastPaginate(req)
			} else {
				pages = p
			}
		case <-time.After(4 * time.Second):
			fmt.Println("[WARN] 高质量分页超时，回退到极速模式")
			pages = fastPaginate(req)
		}
	}

	c.JSON(http.StatusOK, PaginateResponse{Pages: pages})
}
