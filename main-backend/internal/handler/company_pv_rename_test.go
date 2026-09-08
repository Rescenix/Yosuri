package handler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPvShotRenameChinese pv 素材文件名必须含中文关键词（match_media 靠中文滑窗词匹配文件名子串）。
// 回归背景：sanitizeImageName 只留 ASCII，中文前缀退化成空串 → 文件名变 img-时间戳 → 命中率归零。
// 修复方式：ASCII 名落盘后 os.Rename 成中文名（绕开 sanitize）。本测试验证重命名语义。
func TestPvShotRenameChinese(t *testing.T) {
	dir := t.TempDir()
	// 模拟 generateImage 的 ASCII 落盘产物
	asciiPath := filepath.Join(dir, "shot-01.jpg")
	if err := os.WriteFile(asciiPath, []byte("fake-jpeg"), 0o644); err != nil {
		t.Fatal(err)
	}
	kws := deliveryKeywords("做一个学生专注冲刺台，能番茄钟计时", 3)
	if len(kws) == 0 {
		t.Fatal("中文指令应能提取关键词")
	}
	prefix := strings.Join(kws, "-")
	if prefix == "" {
		t.Fatal("前缀不应为空")
	}
	// 重命名（与 deliveryRenderPvStillShots 内逻辑一致）
	chineseName := filepath.Join(dir, fmt.Sprintf("%s-%02d%s", prefix, 1, filepath.Ext(asciiPath)))
	if err := os.Rename(asciiPath, chineseName); err != nil {
		t.Fatalf("中文重命名失败: %v", err)
	}
	if _, err := os.Stat(chineseName); err != nil {
		t.Fatalf("中文名文件应存在: %v", err)
	}
	if _, err := os.Stat(asciiPath); !os.IsNotExist(err) {
		t.Fatal("原 ASCII 文件应已被重命名走")
	}
	// 中文名必须真含关键词（match_media 的滑窗词才匹配得上）
	base := filepath.Base(chineseName)
	for _, k := range kws {
		if !strings.Contains(base, k) {
			t.Fatalf("文件名 %q 缺关键词 %q", base, k)
		}
	}
}
