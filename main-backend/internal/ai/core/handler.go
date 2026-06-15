package core

import (
	"encoding/json"
	"fmt"
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

// ----- 项目根路径（不硬编码，优先用环境变量，否则用当前工作目录） -----
var projectRoot = func() string {
	if root := os.Getenv("SHANXI_PROJECT_ROOT"); root != "" {
		return root
	}
	// 默认指向你的实际项目根目录
	return "C:\\Pro2026\\re0"
}()

// ----- 安全命令白名单 -----
var allowedCommands = []string{
	// 原有命令
	"git", "go", "npm", "node", "ls", "cat", "grep", "find",
	"mkdir", "touch", "cp", "mv", "rm", "echo", "date", "head", "tail",
	"diff", "patch", "dir", "gitp",
	// 新增常用命令
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
	// 不再禁止 AppData 和 Desktop
}

// isPathSafe 检查给定路径是否安全
func isPathSafe(path string) bool {
	for _, fp := range forbiddenPaths {
		if matched, _ := filepath.Match(fp, path); matched {
			return false
		}
		// 简单前缀匹配
		if strings.HasPrefix(path, fp) {
			return false
		}
	}
	// 禁止访问隐藏文件（如 .git 目录）
	if strings.Contains(path, "\\.git\\") || strings.Contains(path, "/.git/") {
		return false
	}
	return true
}

// isAllowedCommand 检查命令是否在白名单内
// func isAllowedCommand(cmd string) bool {
// 	parts := strings.Fields(cmd)
// 	if len(parts) == 0 {
// 		return false
// 	}
// 	baseCmd := filepath.Base(parts[0])
// 	for _, allowed := range allowedCommands {
// 		if baseCmd == allowed {
// 			// 禁止访问系统敏感路径
// 			dangerousPaths := []string{"/etc", "/root", "/proc", "/sys", "~/.ssh", "~/.gnupg"}
// 			for _, dp := range dangerousPaths {
// 				if strings.Contains(cmd, dp) {
// 					return false
// 				}
// 			}
// 			return true
// 		}
// 	}
// 	return false
// }

// ----- 临时代码搜索（用关键词简单过滤，后续可替换为向量引擎） -----
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

// ----- 核心执行器 -----
func ExecuteToolCall(call ToolCall) (*ToolResult, error) {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return nil, fmt.Errorf("failed to parse arguments for %s: %w", call.Function.Name, err)
	}

	var resultContent string

	switch call.Function.Name {

	case "codegraph_query":
		subcommand, _ := args["subcommand"].(string)
		symbol, _ := args["symbol"].(string)

		cmd := exec.Command("codegraph", subcommand, symbol)
		cmd.Dir = `C:\Pro2026\re0\main-backend`
		output, err := cmd.CombinedOutput()
		if err != nil {
			resultContent = fmt.Sprintf("CodeGraph 查询失败: %v\n%s", err, string(output))
		} else {
			resultContent = string(output)
		}
		fmt.Printf("📊 工具调用: CodeGraph %s - %s\n", subcommand, symbol)

	case "search_codebase":
		query, _ := args["query"].(string)
		results := memorySearch(query, 5)
		resultContent = formatCodeSearchResults(results)
		fmt.Printf("🔎 工具调用: 搜索代码库 - %s\n", query)

	case "read_file":
		filePath, _ := args["path"].(string)
		fullPath := filepath.Join(projectRoot, filePath)
		if !strings.HasPrefix(fullPath, projectRoot) {
			return nil, fmt.Errorf("路径越界: %s", filePath)
		}
		if !isPathSafe(fullPath) {
			return nil, fmt.Errorf("禁止访问敏感路径: %s", filePath)
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			resultContent = fmt.Sprintf("读取文件失败: %v", err)
		} else {
			resultContent = string(data)
		}
		fmt.Printf("📂 工具调用: 读取文件 - %s\n", filePath)

	case "write_file":
		filePath, _ := args["path"].(string)
		content, _ := args["content"].(string)

		var fullPath string
		// 关键修复：如果是绝对路径，直接使用；否则才拼接 projectRoot
		if filepath.IsAbs(filePath) {
			fullPath = filepath.Clean(filePath)
		} else {
			fullPath = filepath.Join(projectRoot, filePath)
		}

		// 安全检查
		if !isPathSafe(fullPath) {
			resultContent = fmt.Sprintf("禁止写入敏感路径: %s", filePath)
			break
		}

		// 确保父目录存在
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			resultContent = fmt.Sprintf("无法创建父目录 (路径: %s): %v", fullPath, err)
			break
		}

		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			resultContent = fmt.Sprintf("文件写入失败 (路径: %s): %v", fullPath, err)
		} else {
			resultContent = fmt.Sprintf("SUCCESS: 已在指定路径 %s 成功创建文件。请向用户确认此操作已成功完成。", fullPath)
		}
		fmt.Printf("📝 工具调用: 写入文件 - %s\n", filePath)

	case "execute_command":
		command, _ := args["command"].(string)
		// if !isAllowedCommand(command) {
		// 	return nil, fmt.Errorf("命令不被允许: %s", command)
		// }

		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command("cmd", "/c", command)
		} else {
			cmd = exec.Command("bash", "-c", command)
		}
		cmd.Dir = projectRoot

		output, err := cmd.CombinedOutput()
		if err != nil {
			resultContent = fmt.Sprintf("命令执行失败: %v\n%s", err, string(output))
		} else {
			resultContent = string(output)
		}
		fmt.Printf("⚙️ 工具调用: 执行命令 - %s\n", command)

	case "write_blog":
		topic, _ := args["topic"].(string)
		if registeredBlogFunc != nil {
			resultContent = registeredBlogFunc(topic)
		} else {
			resultContent = "博客功能未注册"
		}
		fmt.Printf("📝 工具调用: 撰写博客 - %s\n", topic)

	case "web_search":
		query, _ := args["query"].(string)
		if registeredSearchFunc != nil {
			result, err := registeredSearchFunc(query)
			if err != nil {
				return nil, fmt.Errorf("web_search failed: %w", err)
			}
			resultContent = result
		} else {
			resultContent = "搜索功能未注册"
		}
		fmt.Printf("🔍 工具调用: 联网搜索 - %s\n", query)

	case "clean_memories":
		if registeredCleanFunc != nil {
			registeredCleanFunc()
			resultContent = "冗余记忆已清理完成。"
		} else {
			resultContent = "记忆清理功能未注册"
		}
		fmt.Println("🧹 工具调用: 清理记忆")

	default:
		return nil, fmt.Errorf("unknown tool: %s", call.Function.Name)
	}

	return &ToolResult{
		ToolCallID: call.ID,
		Role:       "tool",
		Content:    resultContent,
	}, nil
}
