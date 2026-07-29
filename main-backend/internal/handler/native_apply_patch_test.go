package handler

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func nativePatchArgsJSON(t *testing.T, patch string) string {
	t.Helper()
	data, err := json.Marshal(map[string]any{"patch": patch})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestApplyPatchIsResidentNativeTool(t *testing.T) {
	found := false
	for _, tool := range buildCodeWorkflowTools(nil) {
		fn := tool["function"].(map[string]any)
		if fn["name"] == "apply_patch" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("apply_patch 必须常驻，不能要求模型先 load_tools")
	}
	if isNativeOnDemandTool("apply_patch") {
		t.Fatal("apply_patch 不应同时出现在按需工具集")
	}
	if !isNativeExecutableTool("apply_patch") {
		t.Fatal("常驻 apply_patch 没有接入原生执行链")
	}
}

func TestNativeApplyPatchBuildsLongFileInChunks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "long.txt")
	add := "*** Begin Patch\n*** Add File: " + path + "\n+section 1\n+line 1\n*** End Patch\n"
	if _, err := callNativeTool(context.Background(), "apply_patch", nativePatchArgsJSON(t, add)); err != nil {
		t.Fatalf("创建骨架失败: %v", err)
	}

	appendChunk := "*** Begin Patch\n*** Update File: " + path + "\n@@\n+section 2\n+line 2\n*** End of File\n*** End Patch\n"
	result, err := callNativeTool(context.Background(), "apply_patch", nativePatchArgsJSON(t, appendChunk))
	if err != nil {
		t.Fatalf("分段追加失败: %v", err)
	}
	if !strings.Contains(result.Text, "1 个文件") {
		t.Fatalf("结果摘要不完整: %s", result.Text)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "section 1\nline 1\nsection 2\nline 2\n"
	if string(got) != want {
		t.Fatalf("分段写入结果不符\nwant: %q\n got: %q", want, got)
	}
}

func TestNativeApplyPatchWhitespaceFallbackAndAmbiguity(t *testing.T) {
	root := t.TempDir()
	flexiblePath := filepath.Join(root, "flexible.go")
	before := "func hello() {\n\treturn\n}\n"
	if err := os.WriteFile(flexiblePath, []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}
	patch := "*** Begin Patch\n*** Update File: " + flexiblePath + "\n@@\n- func hello() {\n-   return\n- }\n+func hello() {\n+\treturn \"ok\"\n+}\n*** End Patch"
	result, err := callNativeFileTool("apply_patch", nativePatchArgsJSON(t, patch))
	if err != nil {
		t.Fatalf("空白容错补丁失败: %v", err)
	}
	if !strings.Contains(result.Text, "空白容错") {
		t.Fatalf("结果没有报告容错匹配: %s", result.Text)
	}

	ambiguousPath := filepath.Join(root, "ambiguous.txt")
	if err := os.WriteFile(ambiguousPath, []byte("same\nsame\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ambiguous := "*** Begin Patch\n*** Update File: " + ambiguousPath + "\n@@\n-same\n+changed\n*** End Patch"
	if _, err := callNativeFileTool("apply_patch", nativePatchArgsJSON(t, ambiguous)); err == nil ||
		!strings.Contains(err.Error(), "出现 2 次") {
		t.Fatalf("重复上下文应拒绝执行，实得: %v", err)
	}
	got, _ := os.ReadFile(ambiguousPath)
	if string(got) != "same\nsame\n" {
		t.Fatalf("歧义补丁不应改文件，实得: %q", got)
	}
}

func TestNativeApplyPatchPreflightsAllFiles(t *testing.T) {
	root := t.TempDir()
	newPath := filepath.Join(root, "should-not-exist.txt")
	missingPath := filepath.Join(root, "missing.txt")
	patch := "*** Begin Patch\n" +
		"*** Add File: " + newPath + "\n+created too early\n" +
		"*** Update File: " + missingPath + "\n@@\n-old\n+new\n" +
		"*** End Patch"

	if _, err := callNativeFileTool("apply_patch", nativePatchArgsJSON(t, patch)); err == nil {
		t.Fatal("第二个文件预检失败时整次 patch 应失败")
	}
	if _, err := os.Stat(newPath); !os.IsNotExist(err) {
		t.Fatalf("全量预检失败前不应写入第一个文件，stat err=%v", err)
	}
}

func TestNativeApplyPatchMultiFileAndDelete(t *testing.T) {
	root := t.TempDir()
	updatePath := filepath.Join(root, "update.txt")
	deletePath := filepath.Join(root, "delete.txt")
	if err := os.WriteFile(updatePath, []byte("before\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(deletePath, []byte("remove me\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	addPath := filepath.Join(root, "nested", "add.txt")
	patch := "*** Begin Patch\n" +
		"*** Add File: " + addPath + "\n+added\n" +
		"*** Update File: " + updatePath + "\n@@\n-before\n+after\n" +
		"*** Delete File: " + deletePath + "\n" +
		"*** End Patch"
	if _, err := callNativeFileTool("apply_patch", nativePatchArgsJSON(t, patch)); err != nil {
		t.Fatalf("多文件补丁失败: %v", err)
	}
	if data, _ := os.ReadFile(addPath); string(data) != "added\n" {
		t.Fatalf("新增文件内容不符: %q", data)
	}
	if data, _ := os.ReadFile(updatePath); string(data) != "after\r\n" {
		t.Fatalf("更新文件未保留 CRLF: %q", data)
	}
	if _, err := os.Stat(deletePath); !os.IsNotExist(err) {
		t.Fatalf("Delete File 未删除目标: %v", err)
	}
}

func TestNativeApplyPatchCanEmptyAFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty-me.txt")
	if err := os.WriteFile(path, []byte("only line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	patch := "*** Begin Patch\n*** Update File: " + path + "\n@@\n-only line\n*** End Patch"
	if _, err := callNativeFileTool("apply_patch", nativePatchArgsJSON(t, patch)); err != nil {
		t.Fatalf("清空文件失败: %v", err)
	}
	if data, _ := os.ReadFile(path); len(data) != 0 {
		t.Fatalf("删除最后一行后应得到空文件，实得 %q", data)
	}
}

func TestApplyPatchApprovalAndPreviewMetadata(t *testing.T) {
	args := nativePatchArgsJSON(t, "*** Begin Patch\n"+
		"*** Update File: ../outside/App.vue\n@@\n-old\n+new\n"+
		"*** Delete File: obsolete.txt\n"+
		"*** End Patch")

	if !isDangerousTool("apply_patch") {
		t.Fatal("apply_patch 是写盘工具，Ask 模式必须审批")
	}
	if !isIrreversibleToolCall("apply_patch", args) {
		t.Fatal("含 Delete File 的 apply_patch 在 YOLO 模式也必须审批")
	}
	if outside, path := toolOutsideRoot(args); !outside || path != "../outside/App.vue" {
		t.Fatalf("补丁路径未进入越界审批，outside=%v path=%q", outside, path)
	}
	if !isFrontendEdit("apply_patch", args) {
		t.Fatal("修改 .vue 的补丁应触发前端预览")
	}
	if path, err := parseFrontendEditPath("apply_patch", args); err != nil || path != "../outside/App.vue" {
		t.Fatalf("未取到补丁中的前端路径: path=%q err=%v", path, err)
	}
}
