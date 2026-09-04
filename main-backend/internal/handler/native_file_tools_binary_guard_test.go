package handler

import (
	"strings"
	"testing"
)

// 验证文本通道写二进制文档被拦截并给出 generate_office 引导。
func TestRejectBinaryOfficeWrite(t *testing.T) {
	for _, ext := range []string{".pdf", ".docx", ".pptx", ".xlsx", ".doc", ".xls", ".ppt"} {
		if err := rejectBinaryOfficeWrite("交付" + ext); err == nil {
			t.Errorf("%s 应被拦截", ext)
		} else if !strings.Contains(err.Error(), "generate_office") {
			t.Errorf("%s 错误信息应引导到 generate_office，得到: %v", ext, err)
		}
	}
	for _, name := range []string{"notes.md", "main.go", "data.json", "readme.txt", "style.css"} {
		if err := rejectBinaryOfficeWrite(name); err != nil {
			t.Errorf("%s 是文本文件不应拦截: %v", name, err)
		}
	}
}

// 验证 write_file 端到端拦截（走真实 nativeWriteFile 入口）。
func TestNativeWriteFileRejectsPptx(t *testing.T) {
	_, err := nativeWriteFile(map[string]any{
		"path":    "测试.pptx",
		"content": "PK\x03\x04 fake zip bytes",
	})
	if err == nil {
		t.Fatal("write_file 写 .pptx 应被拒绝")
	}
	if !strings.Contains(err.Error(), "generate_office") {
		t.Fatalf("错误应引导用 generate_office，得到: %v", err)
	}
}
