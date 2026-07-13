package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"

	"backend/internal/prismd"
	"backend/internal/swiftnet"
)

var prismdClient = prismd.NewClient("localhost:5666")

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function ToolCallFunc `json:"function"`
}

type ToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Role       string `json:"role"`
	Content    string `json:"content"`
	Failed     bool   `json:"failed"`
}

// 可注册的函数签名
type BlogFunc func(topic string) string
type SearchFunc func(query string) (string, error)
type CleanFunc func()

var (
	registeredBlogFunc   BlogFunc
	registeredSearchFunc SearchFunc
	registeredCleanFunc  CleanFunc
)

func RegisterBlogFunc(fn BlogFunc)     { registeredBlogFunc = fn }
func RegisterSearchFunc(fn SearchFunc) { registeredSearchFunc = fn }
func RegisterCleanFunc(fn CleanFunc)   { registeredCleanFunc = fn }

// ----- 项目根路径：可运行时切换 + 落盘持久化，不再是启动时算一次就锁死 -----
// 优先级：上次持久化的选择 > SHANXI_PROJECT_ROOT 环境变量 > 平台默认值。
// 用 atomic.Value 而不是裸 var + mutex：读多写极少（几乎只在切工作目录时写一次），
// 工具调用（read_file/execute_command 等）高频读，atomic.Load 比加锁更轻。
var projectRootAtomic atomic.Value

func init() {
	projectRootAtomic.Store(loadInitialProjectRoot())
}

// workdirStateFile 支持 SHANXI_WORKDIR_STATE_FILE 覆盖路径——主要是给测试用，
// 避免 SetProjectRoot 的落盘操作意外写到真实用户的 ~/shanxi_data/workdir.txt
func workdirStateFile() string {
	if override := os.Getenv("SHANXI_WORKDIR_STATE_FILE"); override != "" {
		return override
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, "shanxi_data", "workdir.txt")
}

func loadInitialProjectRoot() string {
	if data, err := os.ReadFile(workdirStateFile()); err == nil {
		if saved := strings.TrimSpace(string(data)); saved != "" {
			if info, statErr := os.Stat(saved); statErr == nil && info.IsDir() {
				return saved
			}
		}
	}
	if root := os.Getenv("SHANXI_PROJECT_ROOT"); root != "" {
		return root
	}
	if runtime.GOOS == "linux" && runtime.GOARCH == "arm64" {
		return "/data/data/com.termux/files/home"
	}
	return "C:\\Pro2026\\re0"
}

// GetProjectRoot 返回当前生效的工作目录——所有工具调用（read_file/write_file/
// edit_file/execute_command）都应该用这个，不要再直接引用旧的 projectRoot 变量。
func GetProjectRoot() string {
	return projectRootAtomic.Load().(string)
}

// SetProjectRoot 切换工作目录并落盘持久化，供 /api/workdir 调用。
// 校验路径必须真实存在且是目录，避免切到一个不存在的路径导致后续所有工具调用报错。
func SetProjectRoot(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("目录不存在: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("不是目录: %s", path)
	}
	projectRootAtomic.Store(path)
	// 代码搜索索引存的是相对当前工作目录的路径；目录切换后必须丢弃旧索引。
	// 新索引在下一次 search_codebase / 文件变更时按需创建，避免阻塞切换接口。
	ResetCodebaseIndex()
	stateFile := workdirStateFile()
	if err := os.MkdirAll(filepath.Dir(stateFile), 0755); err != nil {
		return fmt.Errorf("持久化工作目录失败: %w", err)
	}
	return os.WriteFile(stateFile, []byte(path), 0644)
}

// ----- 项目白名单（不受 projectRoot 限制的路径） -----
var allowedProjectPaths = []string{
	`C:\Users\undercurrent\AndroidStudioProjects`,
}

// ----- 安全命令白名单 -----
var allowedCommands = []string{
	"git", "go", "npm", "node", "ls", "cat", "grep", "find",
	"mkdir", "touch", "cp", "mv", "rm", "echo", "date", "head", "tail",
	"diff", "patch", "dir", "gitp",
	"cd", "pwsh", "powershell", "cmd", "explorer", "start", "code",
	"python", "pip", "curl", "wget", "tar", "zip", "unzip",
	"chmod", "chown", "ping", "tracert", "netstat", "ipconfig",
	"tasklist", "taskkill", "shutdown", "restart",
}

// ----- 敏感路径黑名单（禁止读写） -----
var forbiddenPaths = []string{
	`C:\Windows`,
	`C:\Program Files`,
	`C:\Program Files (x86)`,
	`C:\System Volume Information`,
	`C:\$Recycle.Bin`,
	`/etc`, `/root`, `/proc`, `/sys`, `~/.ssh`, `~/.gnupg`,
}

// isPathSafe 检查给定路径是否安全
func isPathSafe(path string) bool {
	for _, fp := range forbiddenPaths {
		if matched, _ := filepath.Match(fp, path); matched {
			return false
		}
		if strings.HasPrefix(path, fp) {
			return false
		}
	}
	if strings.Contains(path, "\\.git\\") || strings.Contains(path, "/.git/") {
		return false
	}
	return true
}

// isPathAllowed 检查路径是否在允许的范围内（项目根目录或白名单）
// isPathAllowed 现在放行所有路径（敏感路径由 isPathSafe 拦截）
func isPathAllowed(path string) bool {
	return true
}

// ----- 临时代码搜索 -----
func memorySearch(query string, topK int) []string {
	queryLower := strings.ToLower(query)
	results := []string{
		"文件: internal/ai/core/prompt.go —— 系统提示词定义",
		"文件: internal/ai/core/tools.go —— 工具定义列表",
		"文件: internal/handler/chat.go —— 核心聊天处理逻辑",
	}
	var filtered []string
	for _, r := range results {
		if strings.Contains(strings.ToLower(r), queryLower) {
			filtered = append(filtered, r)
		}
	}
	if len(filtered) > topK {
		return filtered[:topK]
	}
	return filtered
}

func formatCodeSearchResults(results []string) string {
	if len(results) == 0 {
		return "未找到相关代码"
	}
	var sb strings.Builder
	sb.WriteString("找到以下相关代码片段：\n")
	for i, r := range results {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, r))
	}
	return sb.String()
}

func mapButtonToKeycode(button string) string {
	switch button {
	case "home":
		return "KEYCODE_HOME"
	case "back":
		return "KEYCODE_BACK"
	case "recents":
		return "KEYCODE_APP_SWITCH"
	default:
		return "KEYCODE_HOME"
	}
}
func executeCodebaseQuery(query string) string {
	// 直接调用 Python 内置 sqlite3，零依赖，零故障
	pythonScript := fmt.Sprintf(
		"import sqlite3; conn=sqlite3.connect(r'C:\\Users\\undercurrent\\.cache\\codebase-memory-mcp\\C-Pro2026-re0.db'); cursor=conn.execute(\"SELECT name, file_path FROM nodes WHERE name = ? LIMIT 10\", [%q]); results=[f'{row[0]} -> {row[1]}' for row in cursor]; conn.close(); print('\\n'.join(results) if results else '在代码知识图谱中未找到匹配的代码实体。')",
		query,
	)

	cmd := exec.Command("python", "-c", pythonScript)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("[错误] 代码知识图谱查询失败: %v\n%s", err, string(output))
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return fmt.Sprintf("在代码知识图谱中未找到与 '%s' 匹配的代码实体。", query)
	}
	return result
}

// ----- 核心执行器 -----
func ExecuteToolCall(call ToolCall) (*ToolResult, error) {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return nil, fmt.Errorf("failed to parse arguments for %s: %w", call.Function.Name, err)
	}

	var resultContent string
	failed := false

	switch call.Function.Name {

	case "search_memory":
		mode, _ := args["mode"].(string)
		if mode == "" {
			mode = "summary"
		}

		if mode == "summary" {
			query, _ := args["query"].(string)
			if query == "" {
				resultContent = "搜索记忆需要提供查询内容"
				failed = true
				break
			}

			hits := swiftnet.Default().Select(query, 600, 0.12)
			if len(hits) == 0 {
				resultContent = fmt.Sprintf("记忆库中没有与 %q 相关的内容", query)
			} else {
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("找到 %d 条相关记忆（需要全文时用 mode=\"detail\" + id 展开）：\n", len(hits)))
				for _, h := range hits {
					text := h.Text
					if r := []rune(text); len(r) > 60 {
						text = string(r[:60]) + "…"
					}
					sb.WriteString(fmt.Sprintf("- %s [%s] %s\n", h.ID, h.Cluster, text))
				}
				resultContent = sb.String()
			}
			fmt.Printf("🧠 工具调用: 搜索记忆(summary) - %s，命中 %d\n", query, len(hits))

		} else if mode == "detail" {
			// SwiftNet 的 ID 是 0x 开头的十六进制字符串
			id, _ := args["id"].(string)
			if id == "" {
				if idFloat, ok := args["id"].(float64); ok {
					id = fmt.Sprintf("0x%x", uint64(idFloat))
				}
			}
			if id == "" {
				resultContent = "detail 模式需要提供有效的 id 参数（summary 结果里 0x 开头的 ID）"
				failed = true
				break
			}

			node, ok := swiftnet.Default().Expand(id)
			if !ok {
				resultContent = fmt.Sprintf("记忆 %s 不存在", id)
				failed = true
				break
			}
			resultContent = fmt.Sprintf("── %s ──\n簇: %s\n关键词: %s\n内容: %s", node.ID, node.Cluster, node.Keywords, node.Text)
			fmt.Printf("🧠 工具调用: 搜索记忆(detail) - id=%s\n", id)

		} else {
			resultContent = "mode 参数无效，仅支持 summary 或 detail"
			failed = true
		}

	case "list_dir":
		dirPath, _ := args["path"].(string)
		recursive, _ := args["recursive"].(bool)
		if dirPath == "" {
			resultContent = "list_dir 需要 path 参数"
			failed = true
			break
		}
		var fullPath string
		if filepath.IsAbs(dirPath) {
			fullPath = filepath.Clean(dirPath)
		} else {
			fullPath = filepath.Join(GetProjectRoot(), dirPath)
		}
		if !isPathSafe(fullPath) {
			resultContent = fmt.Sprintf("禁止访问敏感路径: %s", dirPath)
			failed = true
			break
		}
		listing, err := listDirTool(fullPath, recursive)
		if err != nil {
			resultContent = fmt.Sprintf("列目录失败: %v", err)
			failed = true
			break
		}
		resultContent = listing
		fmt.Printf("📁 工具调用: 列目录 - %s (recursive=%v)\n", dirPath, recursive)

	case "codegraph_query":
		subcommand, _ := args["subcommand"].(string)
		symbol, _ := args["symbol"].(string)
		cmd := exec.Command("codegraph", subcommand, symbol)
		cmd.Dir = GetProjectRoot()
		output, err := cmd.CombinedOutput()
		if err != nil {
			resultContent = fmt.Sprintf("CodeGraph 查询失败: %v\n%s", err, string(output))
			failed = true
		} else {
			resultContent = string(output)
		}
		fmt.Printf("📊 工具调用: CodeGraph %s - %s\n", subcommand, symbol)

	case "search_codebase":
		query, _ := args["query"].(string)
		result, err := SearchLocalCodebase(query, 5)
		if err != nil {
			resultContent = fmt.Sprintf("搜索代码库失败: %v", err)
			failed = true
		} else {
			resultContent = result
		}
		fmt.Printf("🔎 工具调用: 搜索代码库 - %s\n", query)
	case "codebase_query":
		query, _ := args["query"].(string)
		resultContent = executeCodebaseQuery(query)
		// 关键：如果查询函数返回了错误信息，必须将工具标记为失败！
		if strings.HasPrefix(resultContent, "[错误]") || strings.HasPrefix(resultContent, "[警告]") {
			failed = true
		}
		fmt.Printf("🧬 工具调用: 代码知识图谱 - %s (失败: %v)\n", query, failed)
	case "read_file":
		filePath, _ := args["path"].(string)
		mode, _ := args["mode"].(string)
		if mode == "" {
			mode = "full"
		}
		startLine, hasStart := argToInt(args["start_line"])
		endLine, hasEnd := argToInt(args["end_line"])
		hasRange := hasStart || hasEnd

		if mode != "full" && mode != "outline" {
			resultContent = "mode 参数无效，仅支持 full 或 outline"
			failed = true
			break
		}
		if mode == "outline" && hasRange {
			resultContent = "参数冲突：outline 模式不能与 start_line/end_line 同时使用"
			failed = true
			break
		}

		var fullPath string
		if filepath.IsAbs(filePath) {
			fullPath = filepath.Clean(filePath)
		} else {
			fullPath = filepath.Join(GetProjectRoot(), filePath)
		}

		// 实时同步索引
		if err := UpdateCodeIndex(fullPath); err != nil {
			fmt.Printf("⚠️ 更新索引失败: %v\n", err)
		}

		if !isPathAllowed(fullPath) {
			resultContent = fmt.Sprintf("路径越界: %s", filePath)
			failed = true
		} else if !isPathSafe(fullPath) {
			resultContent = fmt.Sprintf("禁止访问敏感路径: %s", filePath)
			failed = true
		} else {
			data, err := os.ReadFile(fullPath)
			if err != nil {
				resultContent = fmt.Sprintf("读取文件失败: %v", err)
				failed = true
			} else if mode == "outline" {
				resultContent = buildFileOutline(fullPath, data)
			} else if hasRange {
				resultContent = extractLineRange(data, startLine, hasStart, endLine, hasEnd)
			} else {
				resultContent = string(data)
			}
		}
		fmt.Printf("📂 工具调用: 读取文件 - %s (mode=%s, range=%v)\n", filePath, mode, hasRange)

	case "write_file":
		filePath, _ := args["path"].(string)
		content, _ := args["content"].(string)

		var fullPath string
		if filepath.IsAbs(filePath) {
			fullPath = filepath.Clean(filePath)
		} else {
			fullPath = filepath.Join(GetProjectRoot(), filePath)
		}

		if !isPathAllowed(fullPath) {
			resultContent = fmt.Sprintf("路径越界: %s", filePath)
			failed = true
			break
		}

		if !isPathSafe(fullPath) {
			resultContent = fmt.Sprintf("禁止写入敏感路径: %s", filePath)
			failed = true
			break
		}

		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			resultContent = fmt.Sprintf("无法创建父目录 (路径: %s): %v", fullPath, err)
			failed = true
			break
		}

		const chunkSize = 4096
		f, err := os.Create(fullPath)
		if err != nil {
			resultContent = fmt.Sprintf("文件创建失败 (路径: %s): %v", fullPath, err)
			failed = true
			break
		}

		for start := 0; start < len(content); start += chunkSize {
			end := start + chunkSize
			if end > len(content) {
				end = len(content)
			}
			if _, err := f.WriteString(content[start:end]); err != nil {
				f.Close()
				resultContent = fmt.Sprintf("文件写入失败 (路径: %s): %v", fullPath, err)
				failed = true
				break
			}
		}
		f.Close()

		if !failed {
			if updateErr := UpdateCodeIndex(fullPath); updateErr != nil {
				fmt.Printf("⚠️ 更新索引失败: %v\n", updateErr)
			}
			resultContent = fmt.Sprintf("SUCCESS: 已在指定路径 %s 成功创建文件。", fullPath)
			fmt.Printf("📝 文件写入完成，实际写入 %d 字节\n", len(content))
		}
		fmt.Printf("📝 工具调用: 写入文件 - %s\n", filePath)
	case "edit_file":
		filePath, _ := args["path"].(string)
		oldStr, _ := args["old_string"].(string)
		newStr, _ := args["new_string"].(string)

		var fullPath string
		if filepath.IsAbs(filePath) {
			fullPath = filepath.Clean(filePath)
		} else {
			fullPath = filepath.Join(GetProjectRoot(), filePath)
		}

		if !isPathAllowed(fullPath) {
			resultContent = fmt.Sprintf("路径越界: %s", filePath)
			failed = true
			break
		}
		if !isPathSafe(fullPath) {
			resultContent = fmt.Sprintf("禁止访问敏感路径: %s", filePath)
			failed = true
			break
		}

		data, err := os.ReadFile(fullPath)
		if err != nil {
			resultContent = fmt.Sprintf("读取文件失败: %v", err)
			failed = true
			break
		}

		content := string(data)
		if !strings.Contains(content, oldStr) {
			resultContent = fmt.Sprintf("在文件中未找到匹配的内容: %s", oldStr)
			failed = true
			break
		}

		if strings.Count(content, oldStr) > 1 {
			resultContent = fmt.Sprintf("找到 %d 处匹配，请提供更精确的 old_string", strings.Count(content, oldStr))
			failed = true
			break
		}

		// old_string 在文件里的起始行号——diff 面板展示这次编辑时用它做行号偏移，
		// 不然前端只能拿 old_string 自身的相对行号（永远从 1 开始），跟文件里的真实位置对不上
		matchIdx := strings.Index(content, oldStr)
		startLine := strings.Count(content[:matchIdx], "\n") + 1

		newContent := strings.Replace(content, oldStr, newStr, 1)
		if err := os.WriteFile(fullPath, []byte(newContent), 0644); err != nil {
			resultContent = fmt.Sprintf("写入文件失败: %v", err)
			failed = true
			break
		}

		// 更新代码索引
		if updateErr := UpdateCodeIndex(fullPath); updateErr != nil {
			fmt.Printf("⚠️ 更新索引失败: %v\n", updateErr)
		}
		resultContent = fmt.Sprintf("SUCCESS: 已在 %s 第 %d 行精确替换 1 处内容", filePath, startLine)
		fmt.Printf("✏️ 工具调用: 编辑文件 - %s (old: %q -> new: %q)\n", filePath, oldStr, newStr)
	case "execute_command":

		command, ok := args["command"].(string)
		if !ok || command == "" {
			resultContent = "错误：未提供有效的命令"
			failed = true
			break
		}

		// Windows 平台命令自动转换
		if runtime.GOOS == "windows" {
			if strings.HasPrefix(command, "ls ") || command == "ls" {
				command = strings.Replace(command, "ls", "dir", 1)
			}
			if strings.HasPrefix(command, "cat ") {
				command = strings.Replace(command, "cat", "type", 1)
			}
			if command == "pwd" {
				command = "cd"
			}
			if command == "clear" {
				command = "cls"
			}
		}

		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			// 使用 PowerShell 执行，避免 cmd 对引号解析的兼容性问题
			cmd = exec.Command("powershell", "-Command", command)
		} else {
			cmd = exec.Command("bash", "-c", command)
		}
		cmd.Dir = GetProjectRoot()

		output, err := cmd.CombinedOutput()
		if err != nil {
			resultContent = fmt.Sprintf("命令执行失败: %v\n%s", err, string(output))
			failed = true
		} else {
			resultContent = string(output)
		}
		log.Printf("[EXEC] 最终命令: [%s]", command)
		fmt.Printf("⚙️ 工具调用: 执行命令 [%d字节] %s\n", len(command), command)
	case "web_search":
		query, _ := args["query"].(string)
		if registeredSearchFunc != nil {
			result, err := registeredSearchFunc(query)
			if err != nil {
				resultContent = fmt.Sprintf("web_search failed: %v", err)
				failed = true
			} else {
				resultContent = result
			}
		} else {
			resultContent = "搜索功能未注册"
			failed = true
		}
		fmt.Printf("🔍 工具调用: 联网搜索 - %s\n", query)

	case "clean_memories":
		if registeredCleanFunc != nil {
			registeredCleanFunc()
			resultContent = "冗余记忆已清理完成。"
		} else {
			resultContent = "记忆清理功能未注册"
			failed = true
		}
		fmt.Println("🧹 工具调用: 清理记忆")

	default:
		return nil, fmt.Errorf("unknown tool: %s", call.Function.Name)
	}

	return &ToolResult{
		ToolCallID: call.ID,
		Role:       "tool",
		Content:    resultContent,
		Failed:     failed,
	}, nil
}

// formatMemorySummary 把 PrismD LOOM <query> 的表格响应整理成给模型看的摘要列表。
// 表格里除 Content 外的列本身不含空白字符，所以用 strings.Fields 从两端剥离，
// 中间剩下的 token 拼回去就是 Content —— 避免按字节偏移量切列在中文内容下错位。
func formatMemorySummary(query, responseText string) string {
	if strings.HasPrefix(strings.TrimSpace(responseText), "OK 0 results") {
		return fmt.Sprintf("没有找到与「%s」相关的记忆", query)
	}

	var rows []string
	for _, line := range strings.Split(responseText, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || isLoomTableNoiseLine(trimmed) {
			continue
		}

		fields := strings.Fields(trimmed)
		if len(fields) < 6 { // ID Role [Content...] Energy Score Emotion Inten
			continue
		}
		id := fields[0]
		role := fields[1]
		tail := fields[len(fields)-4:]
		energy, score := tail[0], tail[1]
		content := strings.Join(fields[2:len(fields)-4], " ")

		rows = append(rows, fmt.Sprintf("[ID:%s] role=%s energy=%s score=%s content=%s", id, role, energy, score, content))
	}

	if len(rows) == 0 {
		return fmt.Sprintf("没有找到与「%s」相关的记忆", query)
	}

	rows = append(rows, "（如需查看某条记忆的完整内容，调用 search_memory(mode=\"detail\", id=<对应ID>)）")
	return strings.Join(rows, "\n")
}

// isLoomTableNoiseLine 判断该行是否为 LOOM 表格里的非数据行（表头/分隔线/统计行等）。
func isLoomTableNoiseLine(trimmed string) bool {
	if strings.HasPrefix(trimmed, "OK") || strings.HasPrefix(trimmed, "Query:") || strings.HasPrefix(trimmed, "ID ") {
		return true
	}
	if strings.HasSuffix(trimmed, "results") {
		return true
	}
	if strings.Count(trimmed, "-") == len(trimmed) {
		return true
	}
	return false
}

// formatMemoryDetail 把 PrismD LOOM <id> 的响应整理成给模型看的完整记忆信息。
func formatMemoryDetail(id uint64, responseText string) (string, bool) {
	trimmed := strings.TrimSpace(responseText)
	if strings.HasPrefix(trimmed, "ERROR") {
		return fmt.Sprintf("未找到 ID 为 %d 的记忆，可能已被清理或 ID 有误", id), true
	}
	trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "OK"))
	if trimmed == "" {
		return fmt.Sprintf("未找到 ID 为 %d 的记忆，可能已被清理或 ID 有误", id), true
	}
	return trimmed, false
}

// argToInt 将工具参数（JSON 数字解出来是 float64，部分模型会传字符串）转换为 int。
func argToInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case string:
		if i, err := strconv.Atoi(strings.TrimSpace(n)); err == nil {
			return i, true
		}
	}
	return 0, false
}

// extractLineRange 返回 [start, end] 闭区间（1-indexed）的行，越界自动 clamp，每行带 "行号:" 前缀。
func extractLineRange(data []byte, start int, hasStart bool, end int, hasEnd bool) string {
	lines := strings.Split(string(data), "\n")
	total := len(lines)
	if !hasStart || start < 1 {
		start = 1
	}
	if !hasEnd || end > total {
		end = total
	}
	if start > total {
		return fmt.Sprintf("(文件共 %d 行，start_line=%d 已超出范围)\n", total, start)
	}
	if end < start {
		end = start
	}
	var sb strings.Builder
	for i := start; i <= end; i++ {
		sb.WriteString(fmt.Sprintf("%d:%s\n", i, strings.TrimRight(lines[i-1], "\r")))
	}
	return sb.String()
}

// buildFileOutline 返回文件的签名骨架：Go 文件走 AST，其余（含 AST 解析失败）走正则。
func buildFileOutline(path string, data []byte) string {
	if strings.ToLower(filepath.Ext(path)) == ".go" {
		if out := goOutline(path, data); out != "" {
			return out
		}
	}
	return regexOutline(data)
}

// 关键字后必须跟空白 + 标识符或 '('（Go 方法接收者），否则会误伤 HTML 的 class="..."、type="..." 等属性行。
var outlineLinePattern = regexp.MustCompile(`^[ \t]*(export\s+(default\s+)?)?(async\s+)?(function|func|class|def|interface|type|struct|enum)\s+[A-Za-z_$(]`)

// regexOutline 逐行匹配常见声明关键字（func/class/def/export/interface/type/struct/enum 等），返回带行号的签名列表。
func regexOutline(data []byte) string {
	lines := strings.Split(string(data), "\n")
	var sb strings.Builder
	for i, line := range lines {
		line = strings.TrimRight(line, "\r")
		if outlineLinePattern.MatchString(line) {
			sb.WriteString(fmt.Sprintf("L%d  %s\n", i+1, strings.TrimSpace(line)))
		}
	}
	if sb.Len() == 0 {
		return "(未匹配到函数/类/导出等签名)\n"
	}
	return sb.String()
}

// goOutline 用 go/parser 提取顶层函数与类型声明的签名（含行号）；解析失败返回空串以便回退到正则。
func goOutline(path string, data []byte) string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, data, parser.SkipObjectResolution)
	if err != nil {
		return ""
	}
	var sb strings.Builder
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			line := fset.Position(d.Pos()).Line
			sb.WriteString(fmt.Sprintf("L%d  %s\n", line, goFuncSignature(fset, d)))
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				if ts, ok := spec.(*ast.TypeSpec); ok {
					line := fset.Position(ts.Pos()).Line
					sb.WriteString(fmt.Sprintf("L%d  type %s\n", line, ts.Name.Name))
				}
			}
		}
	}
	if sb.Len() == 0 {
		return "(未发现顶层函数或类型声明)\n"
	}
	return sb.String()
}

// goFuncSignature 打印去掉函数体后的函数声明，折叠为单行签名。
func goFuncSignature(fset *token.FileSet, d *ast.FuncDecl) string {
	body := d.Body
	d.Body = nil
	var buf bytes.Buffer
	err := printer.Fprint(&buf, fset, d)
	d.Body = body
	if err != nil {
		return "func " + d.Name.Name
	}
	return strings.Join(strings.Fields(buf.String()), " ")
}
