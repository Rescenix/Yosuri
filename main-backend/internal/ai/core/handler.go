package core

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

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

// ----- 项目根路径（不硬编码，优先用环境变量，否则自动适配） -----
var projectRoot = func() string {
	if root := os.Getenv("SHANXI_PROJECT_ROOT"); root != "" {
		return root
	}
	if runtime.GOOS == "linux" && runtime.GOARCH == "arm64" {
		return "/data/data/com.termux/files/home"
	}
	return "C:\\Pro2026\\re0"
}()

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
		query, _ := args["query"].(string)
		if query == "" {
			resultContent = "搜索记忆需要提供查询内容"
			failed = true
			break
		}

		// 直接通过 HTTP 调用 PrismD 的 LOOM 原语
		resp, err := http.Post("http://localhost:5666", "text/plain", strings.NewReader("LOOM "+query))
		if err != nil {
			resultContent = fmt.Sprintf("记忆检索失败: %v", err)
			failed = true
			break
		}
		defer resp.Body.Close()

		bodyBytes, _ := io.ReadAll(resp.Body)
		responseText := string(bodyBytes)

		if strings.HasPrefix(responseText, "OK") {
			// 解析 PrismD 返回的 LOOM 结果，提取记忆内容
			lines := strings.Split(responseText, "\n")
			var memories []string
			dataStart := false
			for _, line := range lines {
				if strings.HasPrefix(line, "OK") {
					dataStart = true
					continue
				}
				if !dataStart || strings.HasPrefix(line, "ID") {
					continue
				}
				cols := strings.Split(line, "\t")
				if len(cols) >= 3 {
					memories = append(memories, fmt.Sprintf("[%s] %s", cols[1], cols[2]))
				}
			}
			if len(memories) == 0 {
				resultContent = "没有找到相关记忆"
			} else {
				resultContent = strings.Join(memories, "\n")
			}
		} else {
			resultContent = fmt.Sprintf("记忆检索失败: %s", responseText)
			failed = true
		}
		fmt.Printf("🧠 工具调用: 搜索记忆 - %s\n", query)

	case "codegraph_query":
		subcommand, _ := args["subcommand"].(string)
		symbol, _ := args["symbol"].(string)
		cmd := exec.Command("codegraph", subcommand, symbol)
		cmd.Dir = projectRoot
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
		var fullPath string
		if filepath.IsAbs(filePath) {
			fullPath = filepath.Clean(filePath)
		} else {
			fullPath = filepath.Join(projectRoot, filePath)
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
			} else {
				resultContent = string(data)
			}
		}
		fmt.Printf("📂 工具调用: 读取文件 - %s\n", filePath)

	case "write_file":
		filePath, _ := args["path"].(string)
		content, _ := args["content"].(string)

		var fullPath string
		if filepath.IsAbs(filePath) {
			fullPath = filepath.Clean(filePath)
		} else {
			fullPath = filepath.Join(projectRoot, filePath)
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
			fullPath = filepath.Join(projectRoot, filePath)
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
		resultContent = fmt.Sprintf("SUCCESS: 已在 %s 中精确替换 1 处内容", filePath)
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
		cmd.Dir = projectRoot

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
