// handler/encoding.go
package handler

import (
	"bytes"
	"io"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// convertToUTF8 强制将 GBK/GB18030 数据转为 UTF-8。如果失败，返回原始数据。
func convertToUTF8(data []byte) ([]byte, error) {
	reader := transform.NewReader(bytes.NewReader(data), simplifiedchinese.GBK.NewDecoder())
	utf8Data, err := io.ReadAll(reader)
	if err != nil {
		return data, err
	}
	return utf8Data, nil
}
