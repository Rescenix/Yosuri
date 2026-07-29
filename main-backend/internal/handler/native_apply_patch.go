package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"backend/internal/ai/core"
)

const (
	nativePatchMaxBytes = 4 * 1024 * 1024
	nativePatchMaxFiles = 100
	nativePatchMaxFile  = 50 * 1024 * 1024
)

var nativeApplyPatchToolDef = nativeTool(
	"apply_patch",
	"常驻的 Go 原生结构化补丁工具，用局部增量新增、更新或删除多个文本文件。代码修改优先使用本工具；创建长文件时先写骨架，再分多次 Update File 追加，避免把完整文件塞进 write_file。所有文件会先预检，任一处不匹配则完全不写入；精确匹配失败时仅允许逐行忽略首尾空白，歧义时拒绝执行。",
	map[string]core.ToolProperty{
		"patch": {
			Type:        "string",
			Description: "完整补丁。格式：*** Begin Patch；文件段使用 *** Add File: path、*** Update File: path 或 *** Delete File: path；Add 内容每行以 + 开头；Update 以 @@ 开始，内容行以空格、-、+ 开头；纯追加可在 hunk 末尾写 *** End of File；最后写 *** End Patch。",
		},
	},
	[]string{"patch"},
)

type nativePatchKind uint8

const (
	nativePatchAdd nativePatchKind = iota + 1
	nativePatchUpdate
	nativePatchDelete
)

type nativePatchLine struct {
	kind byte
	text string
}

type nativePatchHunk struct {
	header string
	lines  []nativePatchLine
	atEOF  bool
}

type nativePatchOperation struct {
	kind     nativePatchKind
	path     string
	addLines []string
	hunks    []nativePatchHunk
}

type nativePreparedPatch struct {
	kind          nativePatchKind
	path          string
	before        []byte
	after         []byte
	existedBefore bool
	mode          os.FileMode
	hunks         int
	flexibleHunks int
}

func nativeApplyPatch(args map[string]any) (nativeToolResult, error) {
	raw, ok := args["patch"].(string)
	if !ok || strings.TrimSpace(raw) == "" {
		return nativeToolResult{}, fmt.Errorf("patch 必须是非空字符串")
	}
	ops, err := parseNativePatch(raw)
	if err != nil {
		return nativeToolResult{}, err
	}
	prepared, err := prepareNativePatch(ops)
	if err != nil {
		return nativeToolResult{}, err
	}
	if err := commitNativePatch(prepared); err != nil {
		return nativeToolResult{}, err
	}

	lines := make([]string, 0, len(prepared)+1)
	lines = append(lines, fmt.Sprintf("补丁已应用：%d 个文件", len(prepared)))
	for _, p := range prepared {
		action := "更新"
		switch p.kind {
		case nativePatchAdd:
			action = "新增"
		case nativePatchDelete:
			action = "删除"
		}
		detail := ""
		if p.kind == nativePatchUpdate {
			detail = fmt.Sprintf("，%d 个 hunk", p.hunks)
			if p.flexibleHunks > 0 {
				detail += fmt.Sprintf("（%d 个使用空白容错）", p.flexibleHunks)
			}
		}
		lines = append(lines, fmt.Sprintf("- %s %s%s", action, displayNativePath(p.path), detail))
	}
	return nativeToolResult{Text: strings.Join(lines, "\n")}, nil
}

func parseNativePatch(raw string) ([]nativePatchOperation, error) {
	if len(raw) > nativePatchMaxBytes {
		return nil, fmt.Errorf("patch 超过 %dMB，请拆成多次 apply_patch", nativePatchMaxBytes/(1024*1024))
	}
	if strings.IndexByte(raw, 0) >= 0 {
		return nil, fmt.Errorf("patch 含 NUL，二进制文件不受支持")
	}
	raw = strings.TrimPrefix(raw, "\uFEFF")
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(raw, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) < 2 || lines[0] != "*** Begin Patch" || lines[len(lines)-1] != "*** End Patch" {
		return nil, fmt.Errorf("patch 必须以 *** Begin Patch 开始并以 *** End Patch 结束")
	}

	var ops []nativePatchOperation
	for i := 1; i < len(lines)-1; {
		kind, path, ok := parseNativePatchDirective(lines[i])
		if !ok {
			return nil, fmt.Errorf("第 %d 行：预期文件指令，实得 %q", i+1, lines[i])
		}
		if strings.TrimSpace(path) == "" {
			return nil, fmt.Errorf("第 %d 行：文件路径不能为空", i+1)
		}
		op := nativePatchOperation{kind: kind, path: path}
		i++

		switch kind {
		case nativePatchAdd:
			for i < len(lines)-1 && !isNativePatchDirective(lines[i]) {
				if lines[i] == "" || lines[i][0] != '+' {
					return nil, fmt.Errorf("第 %d 行：Add File 内容必须以 + 开头", i+1)
				}
				op.addLines = append(op.addLines, lines[i][1:])
				i++
			}

		case nativePatchDelete:
			if i < len(lines)-1 && !isNativePatchDirective(lines[i]) {
				return nil, fmt.Errorf("第 %d 行：Delete File 后不能包含 hunk", i+1)
			}

		case nativePatchUpdate:
			for i < len(lines)-1 && !isNativePatchDirective(lines[i]) {
				if lines[i] != "@@" && !strings.HasPrefix(lines[i], "@@ ") {
					return nil, fmt.Errorf("第 %d 行：Update File 内容必须以 @@ hunk 开始", i+1)
				}
				hunk := nativePatchHunk{header: strings.TrimSpace(strings.TrimPrefix(lines[i], "@@"))}
				i++
				for i < len(lines)-1 && !isNativePatchDirective(lines[i]) &&
					lines[i] != "@@" && !strings.HasPrefix(lines[i], "@@ ") {
					if lines[i] == "*** End of File" {
						hunk.atEOF = true
						i++
						break
					}
					if lines[i] == "" || !strings.ContainsRune(" +-", rune(lines[i][0])) {
						return nil, fmt.Errorf("第 %d 行：hunk 内容必须以空格、+ 或 - 开头", i+1)
					}
					hunk.lines = append(hunk.lines, nativePatchLine{kind: lines[i][0], text: lines[i][1:]})
					i++
				}
				if err := validateNativePatchHunk(hunk); err != nil {
					return nil, fmt.Errorf("%s 的 hunk 无效: %w", path, err)
				}
				op.hunks = append(op.hunks, hunk)
			}
			if len(op.hunks) == 0 {
				return nil, fmt.Errorf("Update File %s 至少需要一个 @@ hunk", path)
			}
		}

		ops = append(ops, op)
		if len(ops) > nativePatchMaxFiles {
			return nil, fmt.Errorf("单次 patch 最多修改 %d 个文件，请拆分调用", nativePatchMaxFiles)
		}
	}
	if len(ops) == 0 {
		return nil, fmt.Errorf("patch 中没有文件操作")
	}
	return ops, nil
}

func validateNativePatchHunk(h nativePatchHunk) error {
	if len(h.lines) == 0 {
		return fmt.Errorf("hunk 不能为空")
	}
	oldCount, changed := 0, false
	for _, line := range h.lines {
		switch line.kind {
		case ' ':
			oldCount++
		case '-':
			oldCount++
			changed = true
		case '+':
			changed = true
		}
	}
	if !changed {
		return fmt.Errorf("hunk 没有新增或删除内容")
	}
	if oldCount == 0 && !h.atEOF {
		return fmt.Errorf("纯新增 hunk 必须以 *** End of File 标记为文件末尾追加")
	}
	return nil
}

func parseNativePatchDirective(line string) (nativePatchKind, string, bool) {
	for _, item := range []struct {
		prefix string
		kind   nativePatchKind
	}{
		{"*** Add File: ", nativePatchAdd},
		{"*** Update File: ", nativePatchUpdate},
		{"*** Delete File: ", nativePatchDelete},
	} {
		if strings.HasPrefix(line, item.prefix) {
			return item.kind, strings.TrimSpace(strings.TrimPrefix(line, item.prefix)), true
		}
	}
	return 0, "", false
}

func isNativePatchDirective(line string) bool {
	_, _, ok := parseNativePatchDirective(line)
	return ok
}

// nativePatchHeaderPaths 只读取文件指令，用于执行前的审批与预览判定。
// 它故意不要求整个 patch 合法：坏 patch 仍应先被识别为写盘/越界调用，随后再由执行器报语法错。
func nativePatchHeaderPaths(raw string) []string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		if _, path, ok := parseNativePatchDirective(line); ok && path != "" {
			out = append(out, path)
		}
	}
	return out
}

func nativePatchPathsFromArgs(argsJSON string) []string {
	var args struct {
		Patch string `json:"patch"`
	}
	if json.Unmarshal([]byte(defaultJSONObject(argsJSON)), &args) != nil {
		return nil
	}
	return nativePatchHeaderPaths(args.Patch)
}

func nativePatchWritableArgs(argsJSON string) []map[string]any {
	var args struct {
		Patch string `json:"patch"`
	}
	if json.Unmarshal([]byte(defaultJSONObject(argsJSON)), &args) != nil {
		return nil
	}
	ops, err := parseNativePatch(args.Patch)
	if err != nil {
		return nil
	}
	out := make([]map[string]any, 0, len(ops))
	for _, op := range ops {
		if op.kind != nativePatchDelete {
			out = append(out, map[string]any{"path": op.path})
		}
	}
	return out
}

func nativePatchContainsDelete(argsJSON string) bool {
	var args struct {
		Patch string `json:"patch"`
	}
	if json.Unmarshal([]byte(defaultJSONObject(argsJSON)), &args) != nil {
		return false
	}
	for _, line := range strings.Split(strings.ReplaceAll(args.Patch, "\r\n", "\n"), "\n") {
		if kind, _, ok := parseNativePatchDirective(line); ok && kind == nativePatchDelete {
			return true
		}
	}
	return false
}

func prepareNativePatch(ops []nativePatchOperation) ([]nativePreparedPatch, error) {
	prepared := make([]nativePreparedPatch, 0, len(ops))
	seen := make(map[string]bool, len(ops))
	for _, op := range ops {
		path, err := nativeAbsPath(op.path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", op.path, err)
		}
		key := normCase(path)
		if seen[key] {
			return nil, fmt.Errorf("同一 patch 不能重复操作文件 %s；请合并为一个文件段", op.path)
		}
		seen[key] = true

		p := nativePreparedPatch{kind: op.kind, path: path, mode: 0o644, hunks: len(op.hunks)}
		info, statErr := os.Lstat(path)
		exists := statErr == nil
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return nil, fmt.Errorf("检查 %s 失败: %w", op.path, statErr)
		}
		if exists && info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("拒绝通过补丁修改符号链接: %s", op.path)
		}
		if exists && !info.Mode().IsRegular() {
			return nil, fmt.Errorf("补丁目标不是普通文件: %s", op.path)
		}

		switch op.kind {
		case nativePatchAdd:
			if exists {
				return nil, fmt.Errorf("Add File 目标已存在: %s", op.path)
			}
			p.after = []byte(strings.Join(op.addLines, "\n"))
			if len(op.addLines) > 0 {
				p.after = append(p.after, '\n')
			}

		case nativePatchUpdate, nativePatchDelete:
			if !exists {
				return nil, fmt.Errorf("目标文件不存在: %s", op.path)
			}
			if info.Size() > nativePatchMaxFile {
				return nil, fmt.Errorf("文件超过 50MB，不支持补丁修改: %s", op.path)
			}
			p.before, err = os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("读取 %s 失败: %w", op.path, err)
			}
			if bytes.IndexByte(p.before, 0) >= 0 {
				return nil, fmt.Errorf("二进制文件不受支持: %s", op.path)
			}
			p.existedBefore = true
			p.mode = info.Mode().Perm()
			if op.kind == nativePatchUpdate {
				p.after, p.flexibleHunks, err = applyNativePatchHunks(p.before, op.hunks)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", op.path, err)
				}
				if bytes.Equal(p.before, p.after) {
					return nil, fmt.Errorf("%s: 补丁没有产生任何变化", op.path)
				}
			}
		}
		if len(p.after) > nativePatchMaxFile {
			return nil, fmt.Errorf("补丁后的文件超过 50MB: %s", op.path)
		}
		prepared = append(prepared, p)
	}
	return prepared, nil
}

func applyNativePatchHunks(before []byte, hunks []nativePatchHunk) ([]byte, int, error) {
	normalized := strings.ReplaceAll(string(before), "\r\n", "\n")
	useCRLF := bytes.Contains(before, []byte("\r\n"))
	hadFinalNewline := strings.HasSuffix(normalized, "\n")
	if hadFinalNewline {
		normalized = strings.TrimSuffix(normalized, "\n")
	}
	var lines []string
	if normalized != "" {
		lines = strings.Split(normalized, "\n")
	}

	cursor, flexibleCount := 0, 0
	for idx, hunk := range hunks {
		oldLines, newLines := nativePatchHunkLines(hunk)
		start, flexible, err := locateNativePatchLines(lines, oldLines, cursor, hunk.atEOF)
		if err != nil {
			return nil, 0, fmt.Errorf("第 %d 个 hunk 匹配失败: %w", idx+1, err)
		}
		if flexible {
			flexibleCount++
		}
		next := make([]string, 0, len(lines)-len(oldLines)+len(newLines))
		next = append(next, lines[:start]...)
		next = append(next, newLines...)
		next = append(next, lines[start+len(oldLines):]...)
		lines = next
		cursor = start + len(newLines)
	}

	after := strings.Join(lines, "\n")
	if hadFinalNewline && len(lines) > 0 {
		after += "\n"
	}
	if useCRLF {
		after = strings.ReplaceAll(after, "\n", "\r\n")
	}
	return []byte(after), flexibleCount, nil
}

func nativePatchHunkLines(h nativePatchHunk) (oldLines, newLines []string) {
	for _, line := range h.lines {
		if line.kind != '+' {
			oldLines = append(oldLines, line.text)
		}
		if line.kind != '-' {
			newLines = append(newLines, line.text)
		}
	}
	return oldLines, newLines
}

func locateNativePatchLines(lines, oldLines []string, cursor int, atEOF bool) (int, bool, error) {
	if len(oldLines) == 0 {
		if atEOF {
			return len(lines), false, nil
		}
		return 0, false, fmt.Errorf("没有可定位的上下文")
	}
	if cursor < 0 || cursor > len(lines) {
		cursor = 0
	}
	end := len(lines) - len(oldLines)
	if end < cursor {
		return 0, false, fmt.Errorf("上下文超出文件范围")
	}

	find := func(flexible bool) []int {
		var matches []int
		for i := cursor; i <= end; i++ {
			if atEOF && i+len(oldLines) != len(lines) {
				continue
			}
			ok := true
			for j := range oldLines {
				got, want := lines[i+j], oldLines[j]
				if flexible {
					got, want = strings.TrimSpace(got), strings.TrimSpace(want)
				}
				if got != want {
					ok = false
					break
				}
			}
			if ok {
				matches = append(matches, i)
			}
		}
		return matches
	}

	if matches := find(false); len(matches) == 1 {
		return matches[0], false, nil
	} else if len(matches) > 1 {
		return 0, false, fmt.Errorf("精确上下文出现 %d 次，请增加唯一上下文", len(matches))
	}
	if matches := find(true); len(matches) == 1 {
		return matches[0], true, nil
	} else if len(matches) > 1 {
		return 0, false, fmt.Errorf("空白容错上下文出现 %d 次，请增加唯一上下文", len(matches))
	}
	return 0, false, fmt.Errorf("找不到 hunk 上下文")
}

func commitNativePatch(prepared []nativePreparedPatch) error {
	// 写入前再统一核对一次，避免预检后文件已被其他进程改动。
	for _, p := range prepared {
		if err := verifyNativePatchState(p); err != nil {
			return err
		}
	}

	applied := make([]nativePreparedPatch, 0, len(prepared))
	for _, p := range prepared {
		// 每个文件落盘前再核对一次；中途发生竞争时会回滚已完成的文件。
		if err := verifyNativePatchState(p); err != nil {
			return rollbackNativePatch(applied, err)
		}
		var err error
		switch p.kind {
		case nativePatchAdd:
			err = os.MkdirAll(filepath.Dir(p.path), 0o755)
			if err == nil {
				err = atomicCreateNativePatchFile(p.path, p.after, p.mode)
			}
		case nativePatchUpdate:
			err = os.MkdirAll(filepath.Dir(p.path), 0o755)
			if err == nil {
				err = atomicWriteNative(p.path, p.after, p.mode)
			}
		case nativePatchDelete:
			err = os.Remove(p.path)
		}
		if err != nil {
			return rollbackNativePatch(applied, fmt.Errorf("写入 %s 失败: %w", displayNativePath(p.path), err))
		}
		applied = append(applied, p)
	}
	return nil
}

func atomicCreateNativePatchFile(path string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".rescene-patch-add-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	// 同目录硬链接既是原子发布，也要求目标不存在；并发创建时会失败而不是覆盖。
	return os.Link(tmpName, path)
}

func verifyNativePatchState(p nativePreparedPatch) error {
	info, err := os.Lstat(p.path)
	if !p.existedBefore {
		if err == nil {
			return fmt.Errorf("提交前目标已被创建，补丁已取消: %s", displayNativePath(p.path))
		}
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("提交前检查 %s 失败: %w", displayNativePath(p.path), err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("提交前目标已消失，补丁已取消: %s", displayNativePath(p.path))
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("提交前目标类型已改变，补丁已取消: %s", displayNativePath(p.path))
	}
	current, err := os.ReadFile(p.path)
	if err != nil {
		return fmt.Errorf("提交前读取 %s 失败: %w", displayNativePath(p.path), err)
	}
	if !bytes.Equal(current, p.before) {
		return fmt.Errorf("提交前文件内容已变化，补丁已取消: %s", displayNativePath(p.path))
	}
	return nil
}

func rollbackNativePatch(applied []nativePreparedPatch, cause error) error {
	var rollbackErrors []string
	for i := len(applied) - 1; i >= 0; i-- {
		p := applied[i]
		var err error
		if p.existedBefore {
			if mkdirErr := os.MkdirAll(filepath.Dir(p.path), 0o755); mkdirErr != nil {
				err = mkdirErr
			} else {
				err = atomicWriteNative(p.path, p.before, p.mode)
			}
		} else {
			err = os.Remove(p.path)
			if errors.Is(err, os.ErrNotExist) {
				err = nil
			}
		}
		if err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Sprintf("%s: %v", displayNativePath(p.path), err))
		}
	}
	if len(rollbackErrors) > 0 {
		return fmt.Errorf("%w；且回滚失败：%s", cause, strings.Join(rollbackErrors, "；"))
	}
	return fmt.Errorf("%w；已回滚本次补丁中先前完成的文件", cause)
}
