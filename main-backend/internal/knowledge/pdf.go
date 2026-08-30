package knowledge

// pdf.go —— PDF 文本层提取。
//
// 用纯 Go 库 ledongthuc/pdf（无 cgo、跨平台）。它只能抽 PDF 内嵌的文本层：
// 扫描件/图片型 PDF 没有文本层，抽出来是空的 —— 这种情况静默跳过并返回空串，
// 不报错也不假装抽到了内容（诚报能力，不做假 OCR）。

import (
	"bytes"
	"strings"

	"github.com/ledongthuc/pdf"
)

// extractPdf 抽取 PDF 文本层纯文本，页之间用换行分隔。
func extractPdf(path string) (string, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var buf bytes.Buffer
	total := r.NumPage()
	for i := 1; i <= total; i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		text, err := p.GetPlainText(nil)
		if err != nil {
			continue // 单页损坏不影响其它页
		}
		buf.WriteString(text)
		buf.WriteString("\n")
	}
	return strings.TrimSpace(buf.String()), nil
}