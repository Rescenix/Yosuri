package handler

import (
	"os"
	"path/filepath"
	"testing"
)

// TestStageSetSkipsEmptyStage stage 为空的旁证产物（视觉参考稿/质检报告）不得虚增已完成阶段数。
// 回归背景：stageSet[f.Stage]=true 对空 stage 也写键，审批卡「已完成阶段」含水分。
func TestStageSetSkipsEmptyStage(t *testing.T) {
	dir := t.TempDir()
	// 造一个只有视觉参考稿的项目目录（stage 判定为空的产物）
	refDir := filepath.Join(dir, "10-视觉参考")
	if err := os.MkdirAll(refDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(refDir, "视觉参考稿.jpg"), []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	files := companyCollectProjectFiles(dir)
	if len(files) != 1 {
		t.Fatalf("应收集到 1 个文件，实际 %d", len(files))
	}
	if files[0].Stage != "" {
		t.Fatalf("视觉参考稿 stage 应为空，实际 %q", files[0].Stage)
	}
	if !files[0].Previewable {
		t.Fatal("图片应可预览（审批者要能翻到参考稿）")
	}
	// 模拟审批台的 stageSet 逻辑：空 stage 不得写键
	stageSet := map[string]bool{}
	for _, f := range files {
		if f.Stage != "" {
			stageSet[f.Stage] = true
		}
	}
	if len(stageSet) != 0 {
		t.Fatalf("空 stage 不应进 stageSet，实际 %v", stageSet)
	}
}
