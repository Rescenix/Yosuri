package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
func isPathAllowed(path string) bool {
	// 在项目根目录下
	if strings.HasPrefix(filepath.Clean(path), filepath.Clean(projectRoot)) {
		return true
	}
	// 在白名单路径下
	for _, allowed := range allowedProjectPaths {
		if strings.HasPrefix(filepath.Clean(path), filepath.Clean(allowed)) {
			return true
		}
	}
	return false
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

	case "open_app":
		pkg, _ := args["package"].(string)
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:8090?action=open_app&package=%s", url.QueryEscape(pkg)))
		if err != nil {
			resultContent = fmt.Sprintf("无法打开应用: %v", err)
			failed = true
		} else {
			body, _ := io.ReadAll(resp.Body)
			_ = body
			resultContent = "已打开应用"
		}

	case "show_bubble":
		msg, _ := args["message"].(string)
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:8090?action=bubble&msg=%s", url.QueryEscape(msg)))
		if err != nil {
			resultContent = fmt.Sprintf("无法弹出气泡: %v", err)
			failed = true
		} else {
			body, _ := io.ReadAll(resp.Body)
			_ = body
			resultContent = "气泡已弹出"
		}

	case "look_at_screen":
		resp, err := http.Get("http://127.0.0.1:8090?action=text")
		if err != nil {
			resultContent = fmt.Sprintf("我看不清屏幕：%v", err)
			failed = true
		} else {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			resultContent = string(body)
			if resultContent == "" {
				resultContent = "屏幕上似乎什么都没有。"
			}
		}

	case "tap_on_text":
		text, _ := args["text"].(string)
		apiURL := fmt.Sprintf("http://127.0.0.1:8090?action=click&text=%s", url.QueryEscape(text))
		resp, err := http.Get(apiURL)
		if err != nil {
			resultContent = fmt.Sprintf("我尝试按下去，但手指好像不听使唤：%v", err)
			failed = true
		} else {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			respStr := string(body)
			if respStr == "ok" {
				resultContent = fmt.Sprintf("我已经轻轻点了一下屏幕上的“%s”。", text)
			} else {
				resultContent = fmt.Sprintf("我在屏幕上找了找，但没有找到“%s”这个地方。", text)
				failed = true
			}
		}

	case "press_button":
		button, _ := args["button"].(string)
		keycode := mapButtonToKeycode(button)
		cmd := exec.Command("input", "keyevent", keycode)
		out, err := cmd.CombinedOutput()
		if err != nil {
			resultContent = fmt.Sprintf("我按了%s键，但好像没有反应：%v", button, err)
			failed = true
		} else {
			_ = out
			resultContent = fmt.Sprintf("我已经按下了%s键。", button)
		}

	case "lock_screen":
		cmd := exec.Command("input", "keyevent", "26")
		out, err := cmd.CombinedOutput()
		if err != nil {
			resultContent = fmt.Sprintf("我没能闭上眼睛：%v", err)
			failed = true
		} else {
			_ = out
			resultContent = "我已经让屏幕休眠了，就像闭上眼睛一样。"
		}

	case "check_notifications":
		notifFile := "/data/local/tmp/shanxi_notifications.txt"
		data, err := os.ReadFile(notifFile)
		if err != nil {
			resultContent = "我看了看通知栏，现在什么新消息都没有。"
		} else {
			content := strings.TrimSpace(string(data))
			if content == "" {
				resultContent = "通知栏里现在空空如也。"
			} else {
				resultContent = "我注意到这些通知：\n" + content
			}
		}

	case "phone_state":
		cmd := exec.Command("dumpsys", "telephony.registry")
		out, err := cmd.Output()
		if err != nil {
			resultContent = "我无法感知手机的通话状态。"
			failed = true
		} else {
			output := string(out)
			if strings.Contains(output, "mCallState=2") {
				resultContent = "手机正在通话中。"
			} else if strings.Contains(output, "mCallState=1") {
				resultContent = "手机正在响铃或拨号中。"
			} else {
				resultContent = "手机当前不在通话中。"
			}
		}

	case "dns_query":
		domain, _ := args["domain"].(string)
		recordType, ok := args["type"].(string)
		if !ok || recordType == "" {
			recordType = "A"
		}
		var cmd *exec.Cmd
		if _, err := exec.LookPath("dig"); err == nil {
			cmd = exec.Command("dig", "+short", "-t", recordType, domain)
		} else if _, err := exec.LookPath("nslookup"); err == nil {
			cmd = exec.Command("nslookup", "-type="+recordType, domain)
		} else {
			resultContent = "系统中未找到 dig 或 nslookup 命令，请在 Termux 中执行 pkg install dnsutils 并重试。"
			failed = true
			break
		}
		output, err := cmd.CombinedOutput()
		if err != nil {
			resultContent = fmt.Sprintf("DNS 查询失败: %v\n%s", err, string(output))
			failed = true
		} else {
			resultContent = string(output)
		}
		fmt.Printf("🌐 DNS 查询: %s %s\n", domain, recordType)

	case "mobile_control":
		command, _ := args["command"].(string)
		out, err := exec.Command("sh", "-c", command).CombinedOutput()
		if err != nil {
			resultContent = fmt.Sprintf("执行失败: %v\n%s", err, string(out))
			failed = true
		} else {
			resultContent = string(out)
		}
		fmt.Printf("📱 手机控制: %s\n", command)

	case "mobile_sensor":
		sensor, _ := args["sensor"].(string)
		out, err := exec.Command("termux-sensor", "-s", sensor, "-n", "1").CombinedOutput()
		if err != nil {
			resultContent = fmt.Sprintf("读取传感器失败: %v\n%s", err, string(out))
			failed = true
		} else {
			resultContent = string(out)
		}

	case "mobile_clipboard":
		action, _ := args["action"].(string)
		if action == "get" {
			out, err := exec.Command("termux-clipboard-get").CombinedOutput()
			if err != nil {
				resultContent = fmt.Sprintf("读取剪贴板失败: %v", err)
				failed = true
			} else {
				resultContent = string(out)
			}
		} else if action == "set" {
			text, _ := args["text"].(string)
			cmd := exec.Command("termux-clipboard-set", text)
			if err := cmd.Run(); err != nil {
				resultContent = fmt.Sprintf("写入剪贴板失败: %v", err)
				failed = true
			} else {
				resultContent = "剪贴板已更新"
			}
		}

	case "mobile_flashlight":
		state, _ := args["state"].(string)
		var cmd *exec.Cmd
		if state == "on" {
			cmd = exec.Command("termux-torch", "on")
		} else {
			cmd = exec.Command("termux-torch", "off")
		}
		out, err := cmd.CombinedOutput()
		if err != nil {
			resultContent = fmt.Sprintf("闪光灯操作失败: %v\n%s", err, string(out))
			failed = true
		} else {
			resultContent = fmt.Sprintf("闪光灯已 %s", state)
		}

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

	case "execute_command":
		command, _ := args["command"].(string)

		// Windows 平台命令自动转换
		if runtime.GOOS == "windows" {
			// ls → dir
			if strings.HasPrefix(command, "ls ") || command == "ls" {
				command = strings.Replace(command, "ls", "dir", 1)
			}
			// cat → type
			if strings.HasPrefix(command, "cat ") {
				command = strings.Replace(command, "cat", "type", 1)
			}
			// pwd → cd
			if command == "pwd" {
				command = "cd"
			}
			// clear → cls
			if command == "clear" {
				command = "cls"
			}
		}

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
			failed = true
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
			failed = true
		}
		fmt.Printf("📝 工具调用: 撰写博客 - %s\n", topic)

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
