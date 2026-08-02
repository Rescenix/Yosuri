package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"backend/internal/ai/core"
)

const (
	nativeReadMaxLines = 400
	nativeGrepMaxHits  = 200
	nativeWalkMaxFiles = 100000
)

var nativeIgnoredDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "__pycache__": true,
	".pytest_cache": true, ".gocache": true, ".mimocode": true,
}

func callNativeFileTool(name, argsJSON string) (nativeToolResult, error) {
	var args map[string]any
	if err := json.Unmarshal([]byte(defaultJSONObject(argsJSON)), &args); err != nil {
		return nativeToolResult{}, fmt.Errorf("参数解析失败: %w", err)
	}
	switch name {
	case "read_file":
		return nativeReadFile(args)
	case "grep":
		return nativeGrep(args)
	case "glob":
		return nativeGlob(args)
	case "list_directory":
		return nativeListDirectory(args)
	case "directory_tree":
		return nativeDirectoryTree(args)
	case "get_file_info":
		return nativeGetFileInfo(args)
	case "write_file":
		return nativeWriteFile(args)
	case "edit_file":
		return nativeEditFile(args)
	case "apply_patch":
		return nativeApplyPatch(args)
	case "create_directory":
		return nativeCreateDirectory(args)
	case "move_file":
		return nativeMoveFile(args)
	case "delete_file":
		return nativeDeleteFile(args)
	case "delete_directory":
		return nativeDeleteDirectory(args)
	default:
		return nativeToolResult{}, fmt.Errorf("未知文件工具: %s", name)
	}
}

func defaultJSONObject(s string) string {
	if strings.TrimSpace(s) == "" {
		return "{}"
	}
	return s
}

func stringArg(args map[string]any, name string) string {
	v, _ := args[name].(string)
	return v
}

func intArg(args map[string]any, name string, fallback int) int {
	switch v := args[name].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		n, _ := strconv.Atoi(v.String())
		return n
	default:
		return fallback
	}
}

func nativeAbsPath(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", fmt.Errorf("path 不能为空")
	}
	abs := absAgainstRoot(raw)
	if abs == "" {
		return "", fmt.Errorf("无法解析路径 %q", raw)
	}
	return filepath.Clean(abs), nil
}

func nativeReadFile(args map[string]any) (nativeToolResult, error) {
	path, err := nativeAbsPath(stringArg(args, "path"))
	if err != nil {
		return nativeToolResult{}, err
	}
	offset := intArg(args, "offset", 1)
	limit := intArg(args, "limit", 200)
	if offset < 1 {
		offset = 1
	}
	if limit < 1 {
		limit = 1
	}
	if limit > nativeReadMaxLines {
		limit = nativeReadMaxLines
	}

	f, err := os.Open(path)
	if err != nil {
		return nativeToolResult{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nativeToolResult{}, err
	}
	if !info.Mode().IsRegular() {
		return nativeToolResult{}, fmt.Errorf("不是普通文件: %s", path)
	}
	if info.Size() > 50*1024*1024 {
		return nativeToolResult{}, fmt.Errorf("文件超过 50MB，请缩小目标")
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var lines []string
	total := 0
	end := offset + limit - 1
	for scanner.Scan() {
		total++
		if total >= offset && total <= end {
			lines = append(lines, fmt.Sprintf("%d:%s", total, scanner.Text()))
		}
	}
	if err := scanner.Err(); err != nil {
		return nativeToolResult{}, err
	}
	if offset > total && total > 0 {
		return nativeToolResult{}, fmt.Errorf("起始行 %d 超出文件范围（共 %d 行）", offset, total)
	}
	shownEnd := offset + len(lines) - 1
	header := fmt.Sprintf("文件 %s（共 %d 行，显示 %d-%d", displayNativePath(path), total, offset, maxInt(offset-1, shownEnd))
	if shownEnd < total {
		header += fmt.Sprintf("，还有 %d 行，续读 offset=%d", total-shownEnd, shownEnd+1)
	}
	header += "）"
	return nativeToolResult{Text: header + "\n" + strings.Join(lines, "\n")}, nil
}

func nativeGrep(args map[string]any) (nativeToolResult, error) {
	pattern := stringArg(args, "pattern")
	if pattern == "" {
		return nativeToolResult{}, fmt.Errorf("pattern 不能为空")
	}
	rootRaw := stringArg(args, "path")
	if rootRaw == "" {
		rootRaw = "."
	}
	root, err := nativeAbsPath(rootRaw)
	if err != nil {
		return nativeToolResult{}, err
	}
	exts := nativeTypeExtensions(stringArg(args, "type"))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "rg", "-n", "--no-heading", "--smart-case", pattern)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "RG_CONFIG_PATH=/dev/null")

	var argFiles []string
	for ext := range exts {
		argFiles = append(argFiles, "*"+ext)
	}
	if len(argFiles) > 0 {
		cmd.Args = append(cmd.Args, argFiles...)
	}

	out, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		return nativeToolResult{}, fmt.Errorf("rg 搜索超时（30s）")
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		// rg exit code 1 = no matches
		return nativeToolResult{Text: fmt.Sprintf("在 %s 下未找到匹配 %q 的内容。", displayNativePath(root), pattern)}, nil
	}
	if err != nil {
		return nativeToolResult{}, fmt.Errorf("rg 调用失败: %w", err)
	}

	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nativeToolResult{Text: fmt.Sprintf("在 %s 下未找到匹配 %q 的内容。", displayNativePath(root), pattern)}, nil
	}

	lines := strings.Split(raw, "\n")
	hits := make([]string, 0, len(lines))
	for _, line := range lines {
		hits = append(hits, displayNativePath(root)+"/"+line)
	}
	if len(hits) >= nativeGrepMaxHits {
		hits = hits[:nativeGrepMaxHits]
		hits = append(hits, fmt.Sprintf("（结果已截断到 %d 条，请缩小 path 或增加 type）", nativeGrepMaxHits))
	}
	return nativeToolResult{Text: strings.Join(hits, "\n")}, nil
}

func nativeTypeExtensions(name string) map[string]bool {
	sets := map[string][]string{
		"go": {".go"}, "vue": {".vue"}, "js": {".js", ".jsx", ".mjs", ".cjs"},
		"ts": {".ts", ".tsx"}, "py": {".py"}, "json": {".json"},
		"md": {".md"}, "css": {".css", ".scss", ".less"}, "html": {".html", ".htm"},
	}
	out := map[string]bool{}
	for _, ext := range sets[strings.ToLower(name)] {
		out[ext] = true
	}
	return out
}

func nativeGlob(args map[string]any) (nativeToolResult, error) {
	pattern := filepath.ToSlash(stringArg(args, "pattern"))
	if pattern == "" {
		return nativeToolResult{}, fmt.Errorf("pattern 不能为空")
	}
	re, err := regexp.Compile(globPatternRegexp(pattern))
	if err != nil {
		return nativeToolResult{}, fmt.Errorf("glob 无效: %w", err)
	}
	rootRaw := stringArg(args, "path")
	if rootRaw == "" {
		rootRaw = "."
	}
	root, err := nativeAbsPath(rootRaw)
	if err != nil {
		return nativeToolResult{}, err
	}
	var matches []string
	visited := 0
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			if path != root && nativeIgnoredDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		visited++
		if visited > nativeWalkMaxFiles || len(matches) >= 1000 {
			return io.EOF
		}
		rel, err := filepath.Rel(root, path)
		if err == nil && re.MatchString(filepath.ToSlash(rel)) {
			matches = append(matches, displayNativePath(path))
		}
		return nil
	})
	if err != nil && err != io.EOF {
		return nativeToolResult{}, err
	}
	sort.Strings(matches)
	if len(matches) == 0 {
		return nativeToolResult{Text: fmt.Sprintf("在 %s 下未匹配到 %q。", displayNativePath(root), pattern)}, nil
	}
	return nativeToolResult{Text: strings.Join(matches, "\n")}, nil
}

func globPatternRegexp(pattern string) string {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				i++
				if i+1 < len(pattern) && pattern[i+1] == '/' {
					i++
					b.WriteString("(?:.*/)?")
				} else {
					b.WriteString(".*")
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		case '.', '+', '(', ')', '[', ']', '{', '}', '^', '$', '|', '\\':
			b.WriteByte('\\')
			b.WriteByte(pattern[i])
		default:
			b.WriteByte(pattern[i])
		}
	}
	b.WriteString("$")
	return b.String()
}

func nativeListDirectory(args map[string]any) (nativeToolResult, error) {
	raw := stringArg(args, "path")
	if raw == "" {
		raw = "."
	}
	path, err := nativeAbsPath(raw)
	if err != nil {
		return nativeToolResult{}, err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nativeToolResult{}, err
	}
	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		kind := "[FILE]"
		if e.IsDir() {
			kind = "[DIR]"
		}
		lines = append(lines, kind+" "+e.Name())
	}
	return nativeToolResult{Text: strings.Join(lines, "\n")}, nil
}

func nativeDirectoryTree(args map[string]any) (nativeToolResult, error) {
	raw := stringArg(args, "path")
	if raw == "" {
		raw = "."
	}
	root, err := nativeAbsPath(raw)
	if err != nil {
		return nativeToolResult{}, err
	}
	depth := intArg(args, "depth", 4)
	if depth < 1 {
		depth = 1
	}
	if depth > 8 {
		depth = 8
	}
	lines := []string{displayNativePath(root)}
	count := 0
	var walk func(string, string, int) error
	walk = func(dir, prefix string, level int) error {
		if level >= depth || count >= 2000 {
			return nil
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil
		}
		filtered := entries[:0]
		for _, e := range entries {
			if !e.IsDir() || !nativeIgnoredDirs[e.Name()] {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
		for i, e := range entries {
			count++
			last := i == len(entries)-1
			branch := "├─ "
			nextPrefix := prefix + "│  "
			if last {
				branch = "└─ "
				nextPrefix = prefix + "   "
			}
			suffix := ""
			if e.IsDir() {
				suffix = "/"
			}
			lines = append(lines, prefix+branch+e.Name()+suffix)
			if e.IsDir() {
				_ = walk(filepath.Join(dir, e.Name()), nextPrefix, level+1)
			}
			if count >= 2000 {
				break
			}
		}
		return nil
	}
	if err := walk(root, "", 0); err != nil {
		return nativeToolResult{}, err
	}
	if count >= 2000 {
		lines = append(lines, "…（目录树已截断到 2000 项）")
	}
	return nativeToolResult{Text: strings.Join(lines, "\n")}, nil
}

func nativeGetFileInfo(args map[string]any) (nativeToolResult, error) {
	path, err := nativeAbsPath(stringArg(args, "path"))
	if err != nil {
		return nativeToolResult{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nativeToolResult{}, err
	}
	kind := "file"
	if info.IsDir() {
		kind = "directory"
	}
	return nativeToolResult{Text: fmt.Sprintf(
		"path: %s\ntype: %s\nsize: %d\nmodified: %s\npermissions: %s",
		displayNativePath(path), kind, info.Size(), info.ModTime().Format(time.RFC3339), info.Mode().Perm(),
	)}, nil
}

func nativeWriteFile(args map[string]any) (nativeToolResult, error) {
	path, err := nativeAbsPath(stringArg(args, "path"))
	if err != nil {
		return nativeToolResult{}, err
	}
	content, ok := args["content"].(string)
	if !ok {
		return nativeToolResult{}, fmt.Errorf("content 必须是字符串")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nativeToolResult{}, err
	}
	if err := atomicWriteNative(path, []byte(content), 0o644); err != nil {
		return nativeToolResult{}, err
	}
	return nativeToolResult{Text: "已写入 " + displayNativePath(path)}, nil
}

func nativeEditFile(args map[string]any) (nativeToolResult, error) {
	path, err := nativeAbsPath(stringArg(args, "path"))
	if err != nil {
		return nativeToolResult{}, err
	}
	oldText, oldOK := args["old_string"].(string)
	newText, newOK := args["new_string"].(string)
	if !oldOK || !newOK || oldText == "" {
		return nativeToolResult{}, fmt.Errorf("old_string/new_string 必须是字符串，且 old_string 不能为空")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nativeToolResult{}, err
	}
	content := string(data)
	start, end, flexible, err := locateNativeEdit(content, oldText)
	if err != nil {
		return nativeToolResult{}, err
	}
	if strings.Contains(content[start:end], "\r\n") {
		newText = strings.ReplaceAll(strings.ReplaceAll(newText, "\r\n", "\n"), "\n", "\r\n")
	}
	modified := content[:start] + newText + content[end:]
	if err := atomicWriteNative(path, []byte(modified), 0o644); err != nil {
		return nativeToolResult{}, err
	}
	line := strings.Count(content[:start], "\n") + 1
	mode := "精确匹配"
	if flexible {
		mode = "忽略行首尾空白匹配"
	}
	return nativeToolResult{Text: fmt.Sprintf("已编辑 %s（第 %d 行，%s）", displayNativePath(path), line, mode)}, nil
}

func locateNativeEdit(content, oldText string) (start, end int, flexible bool, err error) {
	if count := strings.Count(content, oldText); count == 1 {
		start = strings.Index(content, oldText)
		return start, start + len(oldText), false, nil
	} else if count > 1 {
		return 0, 0, false, fmt.Errorf("old_string 在文件中出现 %d 次，请提供更长的唯一上下文", count)
	}

	contentLines, contentOffsets := splitLineOffsets(content)
	oldLines := strings.Split(strings.ReplaceAll(oldText, "\r\n", "\n"), "\n")
	if len(oldLines) == 0 || len(oldLines) > len(contentLines) {
		return 0, 0, false, fmt.Errorf("找不到 old_string")
	}
	var matches [][2]int
	for i := 0; i+len(oldLines) <= len(contentLines); i++ {
		ok := true
		for j, oldLine := range oldLines {
			if strings.TrimSpace(strings.TrimSuffix(contentLines[i+j], "\r")) != strings.TrimSpace(oldLine) {
				ok = false
				break
			}
		}
		if ok {
			s := contentOffsets[i]
			eLine := i + len(oldLines)
			e := len(content)
			if eLine < len(contentOffsets) {
				e = contentOffsets[eLine]
				if strings.HasSuffix(oldText, "\n") {
					// 保留被匹配块自身末尾换行的语义。
				} else if e > s && content[e-1] == '\n' {
					e--
					if e > s && content[e-1] == '\r' {
						e--
					}
				}
			}
			matches = append(matches, [2]int{s, e})
		}
	}
	if len(matches) == 0 {
		return 0, 0, false, fmt.Errorf("找不到 old_string（精确与逐行空白容错均未匹配）")
	}
	if len(matches) > 1 {
		return 0, 0, false, fmt.Errorf("空白容错匹配得到 %d 处结果，请提供更长的唯一上下文", len(matches))
	}
	return matches[0][0], matches[0][1], true, nil
}

func splitLineOffsets(s string) ([]string, []int) {
	lines := strings.Split(s, "\n")
	offsets := make([]int, len(lines))
	pos := 0
	for i, line := range lines {
		offsets[i] = pos
		pos += len(line)
		if i < len(lines)-1 {
			pos++
		}
	}
	return lines, offsets
}

func nativeCreateDirectory(args map[string]any) (nativeToolResult, error) {
	path, err := nativeAbsPath(stringArg(args, "path"))
	if err != nil {
		return nativeToolResult{}, err
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return nativeToolResult{}, err
	}
	return nativeToolResult{Text: "已创建目录 " + displayNativePath(path)}, nil
}

func nativeMoveFile(args map[string]any) (nativeToolResult, error) {
	source, err := nativeAbsPath(stringArg(args, "source"))
	if err != nil {
		return nativeToolResult{}, err
	}
	destination, err := nativeAbsPath(stringArg(args, "destination"))
	if err != nil {
		return nativeToolResult{}, err
	}
	if _, err := os.Stat(destination); err == nil {
		return nativeToolResult{}, fmt.Errorf("目标已存在，拒绝覆盖: %s", destination)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return nativeToolResult{}, err
	}
	if err := os.Rename(source, destination); err != nil {
		return nativeToolResult{}, err
	}
	return nativeToolResult{Text: fmt.Sprintf("已移动 %s → %s", displayNativePath(source), displayNativePath(destination))}, nil
}

func nativeDeleteFile(args map[string]any) (nativeToolResult, error) {
	path, err := nativeAbsPath(stringArg(args, "path"))
	if err != nil {
		return nativeToolResult{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nativeToolResult{}, err
	}
	if info.IsDir() {
		return nativeToolResult{}, fmt.Errorf("目标是目录，请使用 delete_directory")
	}
	if err := os.Remove(path); err != nil {
		return nativeToolResult{}, err
	}
	return nativeToolResult{Text: "已删除文件 " + displayNativePath(path)}, nil
}

func nativeDeleteDirectory(args map[string]any) (nativeToolResult, error) {
	path, err := nativeAbsPath(stringArg(args, "path"))
	if err != nil {
		return nativeToolResult{}, err
	}
	root := filepath.Clean(core.GetProjectRoot())
	if sameNativePath(path, root) || filepath.Dir(path) == path {
		return nativeToolResult{}, fmt.Errorf("拒绝删除工作目录根或磁盘根")
	}
	info, err := os.Stat(path)
	if err != nil {
		return nativeToolResult{}, err
	}
	if !info.IsDir() {
		return nativeToolResult{}, fmt.Errorf("目标不是目录，请使用 delete_file")
	}
	if err := os.RemoveAll(path); err != nil {
		return nativeToolResult{}, err
	}
	return nativeToolResult{Text: "已删除目录 " + displayNativePath(path)}, nil
}

func atomicWriteNative(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	tmp, err := os.CreateTemp(dir, ".rescene-write-*")
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
	if runtime.GOOS == "windows" {
		// Windows 的 os.Rename 不能可靠覆盖已有目标。不能先删目标再 rename：
		// rename 一旦失败会把用户原文件也丢掉。这里退化为直接覆盖写；AgentFS 已在
		// 调用前保存 before 版本，至少保证失败时旧文件不会因我们主动删除而消失。
		return os.WriteFile(path, data, mode)
	}
	return os.Rename(tmpName, path)
}

func displayNativePath(path string) string {
	rel, err := filepath.Rel(core.GetProjectRoot(), path)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(rel)
	}
	return path
}

func sameNativePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
