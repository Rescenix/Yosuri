package main

// tools_native.go — 本机内置工具（从 re0 main-backend/internal/handler 移植）
// 文件读取/搜索/目录/命令执行，纯标准库实现，不依赖外部进程。

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// nativeToolDefs 返回本机工具定义列表
func nativeToolDefs() []ToolDefinition {
	return append([]ToolDefinition{
		nativeTool("read_file", "按行读取文本文件，返回带行号的内容；一次最多 400 行。offset 从 1 开始，limit 是行数。", map[string]ToolProperty{
			"path":   {Type: "string", Description: "文件路径；相对路径按当前工作目录解析"},
			"offset": {Type: "integer", Description: "起始行号，1-indexed，默认 1"},
			"limit":  {Type: "integer", Description: "读取行数，默认 200，最大 400"},
		}, []string{"path"}),
		nativeTool("grep", "在文件内容中搜索正则表达式，返回 文件:行号:匹配行；默认搜索当前目录，最多 200 条。", map[string]ToolProperty{
			"pattern": {Type: "string", Description: "正则表达式"},
			"path":    {Type: "string", Description: "搜索起点，默认当前工作目录"},
			"type":    {Type: "string", Description: "可选文件类型：go/vue/js/ts/py/json/md/css/html"},
		}, []string{"pattern"}),
		nativeTool("glob", "按文件名或相对路径模式查找文件，例如 **/*.go、src/**/*.vue。", map[string]ToolProperty{
			"pattern": {Type: "string", Description: "glob 模式，支持 *、? 和 **"},
			"path":    {Type: "string", Description: "搜索起点，默认当前工作目录"},
		}, []string{"pattern"}),
		nativeTool("list_directory", "列出目录中的直接子项，区分文件和目录。", map[string]ToolProperty{
			"path": {Type: "string", Description: "目录路径，默认当前工作目录"},
		}, nil),
		nativeTool("directory_tree", "递归列出目录树；默认跳过 .git、node_modules 等大目录并限制返回规模。", map[string]ToolProperty{
			"path":  {Type: "string", Description: "目录路径，默认当前工作目录"},
			"depth": {Type: "integer", Description: "最大深度，默认 4，最大 8"},
		}, nil),
		nativeTool("get_file_info", "读取文件或目录的大小、修改时间、类型和权限。", map[string]ToolProperty{
			"path": {Type: "string", Description: "文件或目录路径"},
		}, []string{"path"}),
		nativeTool("write_file", "创建或完整覆盖一个文本文件；自动创建父目录。", map[string]ToolProperty{
			"path":    {Type: "string", Description: "目标文件路径"},
			"content": {Type: "string", Description: "完整文件内容"},
		}, []string{"path", "content"}),
		nativeTool("edit_file", "在文本文件中做一次定点替换。优先精确匹配；精确失败时允许逐行忽略首尾空白匹配，但拒绝多处歧义。", map[string]ToolProperty{
			"path":       {Type: "string", Description: "目标文件路径"},
			"old_string": {Type: "string", Description: "要替换的原始文本，应从 read_file 结果原样复制"},
			"new_string": {Type: "string", Description: "替换后的文本"},
		}, []string{"path", "old_string", "new_string"}),
		nativeTool("create_directory", "递归创建目录；目录已存在时视为成功。", map[string]ToolProperty{
			"path": {Type: "string", Description: "目录路径"},
		}, []string{"path"}),
		nativeTool("move_file", "移动或重命名文件/目录；目标已存在时拒绝覆盖。", map[string]ToolProperty{
			"source":      {Type: "string", Description: "源路径"},
			"destination": {Type: "string", Description: "目标路径"},
		}, []string{"source", "destination"}),
		nativeTool("delete_file", "删除单个文件。该操作不可逆。", map[string]ToolProperty{
			"path": {Type: "string", Description: "文件路径"},
		}, []string{"path"}),
		nativeTool("run_command", "在当前目录执行一条系统命令并返回退出码、stdout 和 stderr。", map[string]ToolProperty{
			"command": {Type: "string", Description: "要执行的命令"},
			"timeout": {Type: "integer", Description: "超时秒数，默认 120，最大 600"},
		}, []string{"command"}),
		nativeTool("web_search", "联网搜索（Firecrawl），返回带标题/链接/摘要的结果。", map[string]ToolProperty{
			"query": {Type: "string", Description: "搜索关键词"},
			"limit": {Type: "integer", Description: "返回条数，默认 5，最大 10"},
		}, []string{"query"}),
	}, computerUseToolDefs()...)
}

// callNativeTool 分发执行本机工具
func callNativeTool(ctx context.Context, name, argsJSON string) (ToolResult, error) {
	switch name {
	case "read_file", "grep", "glob", "list_directory", "directory_tree", "get_file_info",
		"write_file", "edit_file", "create_directory", "move_file", "delete_file":
		return callNativeFileTool(name, argsJSON)
	case "run_command":
		return callNativeCommand(ctx, argsJSON)
	case "web_search":
		return callFirecrawlSearch(ctx, argsJSON)
	case "computer_screenshot", "computer_mouse_move", "computer_mouse_click",
		"computer_mouse_drag", "computer_type", "computer_key",
		"computer_screen_size", "computer_scroll":
		return callComputerUseTool(ctx, name, argsJSON)
	default:
		return ToolResult{}, fmt.Errorf("未知工具: %s", name)
	}
}

// argsMap 解析 argsJSON 为 map
func argsMap(argsJSON string) (map[string]string, error) {
	var m map[string]string
	if err := json.Unmarshal([]byte(argsJSON), &m); err != nil {
		return nil, fmt.Errorf("参数解析失败: %v", err)
	}
	return m, nil
}

func argStr(m map[string]string, k, def string) string {
	if v, ok := m[k]; ok && strings.TrimSpace(v) != "" {
		return v
	}
	return def
}

// callNativeFileTool 文件工具实现
func callNativeFileTool(name, argsJSON string) (ToolResult, error) {
	m, err := argsMap(argsJSON)
	if err != nil {
		return ToolResult{}, err
	}
	cwd, _ := os.Getwd()

	switch name {
	case "read_file":
		path := argStr(m, "path", cwd)
		if !filepath.IsAbs(path) {
			path = filepath.Join(cwd, path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return ToolResult{}, err
		}
		lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
		offset := atoiDefault(m["offset"], 1)
		limit := atoiDefault(m["limit"], 200)
		if offset < 1 {
			offset = 1
		}
		if limit < 1 || limit > 400 {
			limit = 200
		}
		var sb strings.Builder
		for i := offset - 1; i < len(lines) && i < offset-1+limit; i++ {
			fmt.Fprintf(&sb, "%d|%s\n", i+1, lines[i])
		}
		fmt.Fprintf(&sb, "（共 %d 行，显示 %d-%d）", len(lines), offset, offset-1+limit)
		return ToolResult{Text: sb.String()}, nil

	case "list_directory":
		path := argStr(m, "path", cwd)
		if !filepath.IsAbs(path) {
			path = filepath.Join(cwd, path)
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return ToolResult{}, err
		}
		var sb strings.Builder
		for _, e := range entries {
			marker := "📄"
			if e.IsDir() {
				marker = "📁"
			}
			fmt.Fprintf(&sb, "%s %s\n", marker, e.Name())
		}
		fmt.Fprintf(&sb, "（共 %d 项）", len(entries))
		return ToolResult{Text: sb.String()}, nil

	case "get_file_info":
		path := argStr(m, "path", cwd)
		if !filepath.IsAbs(path) {
			path = filepath.Join(cwd, path)
		}
		info, err := os.Stat(path)
		if err != nil {
			return ToolResult{}, err
		}
		kind := "文件"
		if info.IsDir() {
			kind = "目录"
		}
		return ToolResult{Text: fmt.Sprintf("路径: %s\n类型: %s\n大小: %d 字节\n修改时间: %s",
			path, kind, info.Size(), info.ModTime().Format("2006-01-02 15:04:05"))}, nil

	case "write_file":
		path := argStr(m, "path", "")
		if path == "" {
			return ToolResult{}, fmt.Errorf("缺少 path")
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(cwd, path)
		}
		content := m["content"]
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return ToolResult{}, err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return ToolResult{}, err
		}
		return ToolResult{Text: fmt.Sprintf("✅ 已写入 %s（%d 字节）", path, len(content))}, nil

	case "edit_file":
		path := argStr(m, "path", "")
		if path == "" {
			return ToolResult{}, fmt.Errorf("缺少 path")
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(cwd, path)
		}
		oldStr := m["old_string"]
		newStr := m["new_string"]
		if oldStr == "" {
			return ToolResult{}, fmt.Errorf("缺少 old_string")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return ToolResult{}, err
		}
		content := string(data)
		if !strings.Contains(content, oldStr) {
			return ToolResult{}, fmt.Errorf("old_string 未找到（注意应原样复制 read_file 的内容）")
		}
		content = strings.Replace(content, oldStr, newStr, 1)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return ToolResult{}, err
		}
		return ToolResult{Text: fmt.Sprintf("✅ 已替换 %s", path)}, nil

	case "create_directory":
		path := argStr(m, "path", "")
		if path == "" {
			return ToolResult{}, fmt.Errorf("缺少 path")
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(cwd, path)
		}
		if err := os.MkdirAll(path, 0o755); err != nil {
			return ToolResult{}, err
		}
		return ToolResult{Text: fmt.Sprintf("✅ 已创建目录 %s", path)}, nil

	case "delete_file":
		path := argStr(m, "path", "")
		if path == "" {
			return ToolResult{}, fmt.Errorf("缺少 path")
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(cwd, path)
		}
		if err := os.Remove(path); err != nil {
			return ToolResult{}, err
		}
		return ToolResult{Text: fmt.Sprintf("🗑️ 已删除 %s", path)}, nil

	case "grep", "glob", "directory_tree", "move_file":
		return ToolResult{}, fmt.Errorf("工具 %s 尚未移植，跳过", name)
	}
	return ToolResult{}, fmt.Errorf("未知文件工具: %s", name)
}

// callNativeCommand 执行系统命令
func callNativeCommand(ctx context.Context, argsJSON string) (ToolResult, error) {
	m, err := argsMap(argsJSON)
	if err != nil {
		return ToolResult{}, err
	}
	cmdStr := m["command"]
	if cmdStr == "" {
		return ToolResult{}, fmt.Errorf("缺少 command")
	}
	timeout := atoiDefault(m["timeout"], 120)
	if timeout > 600 {
		timeout = 600
	}
	if timeout < 1 {
		timeout = 1
	}

	cctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	var shell, flag string
	if runtime.GOOS == "windows" {
		shell = "cmd"
		flag = "/c"
	} else {
		shell = "/bin/sh"
		flag = "-c"
	}
	cmd := exec.CommandContext(cctx, shell, flag, cmdStr)
	out, err := cmd.CombinedOutput()
	elapsed := time.Since(time.Now().Add(-time.Since(time.Now())))

	text := fmt.Sprintf("$ %s\n%s", cmdStr, string(out))
	if err != nil {
		if cctx.Err() == context.DeadlineExceeded {
			text += fmt.Sprintf("\n⚠️ 超时（%ds）", timeout)
		} else {
			text += fmt.Sprintf("\n⚠️ 退出码: %v", err)
		}
	} else {
		text += fmt.Sprintf("\n✅ 完成（%s）", elapsed.Round(time.Millisecond))
	}
	return ToolResult{Text: text}, nil
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return def
	}
	return n
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
