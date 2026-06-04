package handler

import (
	"crypto/md5"
	"fmt"
	"net/http"
	"strings"
	"time"

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

// 生成缓存 key（与 redis.go 无关，属于分页逻辑）
func generateCacheKey(req PaginateRequest) string {
	hash := md5.Sum([]byte(req.Text))
	return fmt.Sprintf("pages:%x:%.0f:%.0f:%.0f", hash, req.FontSize, req.PageWidth, req.PageHeight)
}

// 估算文本宽度（像素）
func estimateTextWidth(text string, fontSize float64) float64 {
	var width float64
	for _, r := range text {
		if r > 0x7F {
			width += fontSize
		} else {
			width += fontSize * 0.6
		}
	}
	return width
}

var htmlEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
)

// 精确分页（二分法）
func doPaginate(req PaginateRequest) []string {
	text := req.Text
	fontSize := req.FontSize
	pageWidth := req.PageWidth
	pageHeight := req.PageHeight

	lineHeight := fontSize * 1.8
	pad := 24.0
	contentWidth := pageWidth - pad*2
	contentHeight := pageHeight - pad*2
	maxLinesPerPage := int(contentHeight / lineHeight)

	indentWidth := fontSize * 2
	indentStr := "　　"

	type Line struct {
		text        string
		width       float64
		height      float64
		paragraphID int
		indent      bool
		isFirst     bool
		isLast      bool
	}

	lines := make([]Line, 0)
	paragraphs := strings.Split(text, "\n")

	for p := 0; p < len(paragraphs); p++ {
		para := paragraphs[p]

		if para == "" {
			lines = append(lines, Line{text: "", width: 0, height: lineHeight, paragraphID: p, indent: false, isFirst: true, isLast: true})
			continue
		}

		para = strings.TrimLeft(para, " \t\r")
		if para == "" {
			lines = append(lines, Line{text: "", width: 0, height: lineHeight, paragraphID: p, indent: false, isFirst: true, isLast: true})
			continue
		}

		remaining := para
		isFirstLine := true

		for len(remaining) > 0 {
			maxWidth := contentWidth
			if isFirstLine {
				maxWidth = contentWidth - indentWidth
			}

			low, high := 0, len(remaining)
			best := 0
			for low <= high {
				mid := (low + high) / 2
				if estimateTextWidth(remaining[:mid], fontSize) <= maxWidth {
					best = mid
					low = mid + 1
				} else {
					high = mid - 1
				}
			}

			lineText := remaining[:best]
			cutPos := best

			if best < len(remaining) && remaining[best] != ' ' {
				lastSpace := strings.LastIndex(lineText, " ")
				if lastSpace > 0 {
					lineText = lineText[:lastSpace]
					cutPos = lastSpace + 1
				}
			}

			lineText = strings.TrimRight(lineText, " \t")
			nextStart := cutPos
			for nextStart < len(remaining) && remaining[nextStart] == ' ' {
				nextStart++
			}

			isLast := nextStart >= len(remaining)
			lines = append(lines, Line{
				text:        lineText,
				width:       estimateTextWidth(lineText, fontSize),
				height:      lineHeight,
				paragraphID: p,
				indent:      isFirstLine,
				isFirst:     isFirstLine,
				isLast:      isLast,
			})
			isFirstLine = false
			remaining = remaining[nextStart:]
		}
	}

	// 移除开头连续空行
	for len(lines) > 0 && lines[0].text == "" {
		lines = lines[1:]
	}

	pages := make([]string, 0)
	startIdx := 0

	for startIdx < len(lines) {
		maxLines := maxLinesPerPage
		if maxLines > len(lines)-startIdx {
			maxLines = len(lines) - startIdx
		}
		endIdx := startIdx + maxLines - 1

		// 孤行控制
		if lines[endIdx].isFirst && !lines[endIdx].isLast {
			if endIdx+1 < len(lines) && (endIdx+1-startIdx+1) <= maxLinesPerPage {
				endIdx++
			} else {
				endIdx--
			}
		}
		if lines[startIdx].isLast && !lines[startIdx].isFirst {
			if startIdx > 0 && (endIdx-startIdx+2) <= maxLinesPerPage {
				startIdx--
				maxLines = maxLinesPerPage
				if maxLines > len(lines)-startIdx {
					maxLines = len(lines) - startIdx
				}
				endIdx = startIdx + maxLines - 1
				if lines[endIdx].isFirst && !lines[endIdx].isLast {
					if endIdx+1 < len(lines) && (endIdx+1-startIdx+1) <= maxLinesPerPage {
						endIdx++
					} else {
						endIdx--
					}
				}
			}
		}

		if endIdx < startIdx {
			endIdx = startIdx
		}

		pageLines := lines[startIdx : endIdx+1]
		var pageTextParts []string
		for _, l := range pageLines {
			if l.indent {
				pageTextParts = append(pageTextParts, indentStr+l.text)
			} else {
				pageTextParts = append(pageTextParts, l.text)
			}
		}
		pageText := strings.Join(pageTextParts, "\n")
		escaped := htmlEscaper.Replace(pageText)
		html := fmt.Sprintf(`<div style="
      width: 100%%; height: 100%%;
      padding: %.0fpx;
      box-sizing: border-box;
      font-family: 'Inter', system-ui, sans-serif;
      font-size: %.0fpx;
      line-height: 1.8;
      white-space: pre-wrap;
      word-wrap: break-word;
      text-align: justify;
      overflow: hidden;
    ">%s</div>`, pad, fontSize, escaped)

		pages = append(pages, html)
		startIdx = endIdx + 1
	}
	return pages
}

// 快速分页（估算字符数）
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

	// 移除开头空行
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
      font-family: 'Inter', system-ui, sans-serif;
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

// HTTP 接口
func Paginate(c *gin.Context) {
	var req PaginateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	// 1. 尝试 Redis 缓存（使用 redis.go 中的变量和函数）
	if redisEnabled {
		key := generateCacheKey(req)
		if cachedPages, err := getPagesFromCache(key); err == nil {
			c.JSON(http.StatusOK, PaginateResponse{Pages: cachedPages})
			return
		}
	}

	// 2. 分页计算
	var pages []string
	textSize := len(req.Text)
	if textSize > 500_000 {
		fmt.Printf("[INFO] 文本较大 (%d bytes)，启用极速分页模式\n", textSize)
		pages = fastPaginate(req)
	} else {
		done := make(chan []string, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("[WARN] 精确分页崩溃: %v，回退到极速模式\n", r)
					done <- nil
				}
			}()
			done <- doPaginate(req)
		}()
		select {
		case p := <-done:
			if p == nil {
				pages = fastPaginate(req)
			} else {
				pages = p
			}
		case <-time.After(5 * time.Second):
			fmt.Println("[WARN] 精确分页超时，回退到极速模式")
			pages = fastPaginate(req)
		}
	}

	// 3. 异步写入 Redis
	if redisEnabled {
		key := generateCacheKey(req)
		go func() {
			if err := setPagesToCache(key, pages); err != nil {
				fmt.Printf("[WARN] 写入 Redis 缓存失败: %v\n", err)
			}
		}()
	}

	c.JSON(http.StatusOK, PaginateResponse{Pages: pages})
}
