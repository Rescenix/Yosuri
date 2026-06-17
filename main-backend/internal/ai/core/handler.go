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
	// 手机 Termux 环境默认使用 Termux 主目录
	if runtime.GOOS == "linux" && runtime.GOARCH == "arm64" {
		return "/data/data/com.termux/files/home"
	}
	// 其他环境默认使用当前工作目录
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}()

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

// mapButtonToKeycode 将拟人化按键名映射为 Android KeyEvent 键码
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

// ----- 核心执行器 -----
func ExecuteToolCall(call ToolCall) (*ToolResult, error) {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return nil, fmt.Errorf("failed to parse arguments for %s: %w", call.Function.Name, err)
	}

	var resultContent string

	switch call.Function.Name {

	// ========== 新增神权工具：对接无障碍服务、通知监听、设备管理器 ==========

	case "look_at_screen":
		resp, err := http.Get("http://127.0.0.1:8090?action=text")
		if err != nil {
			resultContent = fmt.Sprintf("我看不清屏幕：%v", err)
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
		} else {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			respStr := string(body)
			if respStr == "ok" {
				resultContent = fmt.Sprintf("我已经轻轻点了一下屏幕上的“%s”。", text)
			} else {
				resultContent = fmt.Sprintf("我在屏幕上找了找，但没有找到“%s”这个地方。", text)
			}
		}

	case "press_button":
		button, _ := args["button"].(string)
		keycode := mapButtonToKeycode(button)
		cmd := exec.Command("input", "keyevent", keycode)
		out, err := cmd.CombinedOutput()
		if err != nil {
			resultContent = fmt.Sprintf("我按了%s键，但好像没有反应：%v", button, err)
		} else {
			_ = out
			resultContent = fmt.Sprintf("我已经按下了%s键。", button)
		}

	case "lock_screen":
		// 临时方案：模拟电源键。未来可升级为设备管理器锁屏（需激活DeviceAdmin）
		cmd := exec.Command("input", "keyevent", "26")
		out, err := cmd.CombinedOutput()
		if err != nil {
			resultContent = fmt.Sprintf("我没能闭上眼睛：%v", err)
		} else {
			_ = out
			resultContent = "我已经让屏幕休眠了，就像闭上眼睛一样。"
		}

	case "check_notifications":
		// 通知监听服务应将最近通知写入此文件
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

	// ========== 原有工具 ==========

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
			break
		}
		output, err := cmd.CombinedOutput()
		if err != nil {
			resultContent = fmt.Sprintf("DNS 查询失败: %v\n%s", err, string(output))
		} else {
			resultContent = string(output)
		}
		fmt.Printf("🌐 DNS 查询: %s %s\n", domain, recordType)

	case "mobile_control":
		command, _ := args["command"].(string)
		out, err := exec.Command("sh", "-c", command).CombinedOutput()
		if err != nil {
			resultContent = fmt.Sprintf("执行失败: %v\n%s", err, string(out))
		} else {
			resultContent = string(out)
		}
		fmt.Printf("📱 手机控制: %s\n", command)

	case "mobile_sensor":
		sensor, _ := args["sensor"].(string)
		out, err := exec.Command("termux-sensor", "-s", sensor, "-n", "1").CombinedOutput()
		if err != nil {
			resultContent = fmt.Sprintf("读取传感器失败: %v\n%s", err, string(out))
		} else {
			resultContent = string(out)
		}

	case "mobile_clipboard":
		action, _ := args["action"].(string)
		if action == "get" {
			out, err := exec.Command("termux-clipboard-get").CombinedOutput()
			if err != nil {
				resultContent = fmt.Sprintf("读取剪贴板失败: %v", err)
			} else {
				resultContent = string(out)
			}
		} else if action == "set" {
			text, _ := args["text"].(string)
			cmd := exec.Command("termux-clipboard-set", text)
			if err := cmd.Run(); err != nil {
				resultContent = fmt.Sprintf("写入剪贴板失败: %v", err)
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
		if filepath.IsAbs(filePath) {
			fullPath = filepath.Clean(filePath)
		} else {
			fullPath = filepath.Join(projectRoot, filePath)
		}

		if !isPathSafe(fullPath) {
			resultContent = fmt.Sprintf("禁止写入敏感路径: %s", filePath)
			break
		}

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
