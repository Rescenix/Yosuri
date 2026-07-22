package core

// 内置工具执行器（ExecuteToolCall）及全部文件/命令/记忆工具已整体退役——
// 工具统一走 MCP（主 Agent 用 load_tools 按需加载，子代理用 MCP 只读子集）。
// 本文件现在只剩：工具调用的数据结构 + 项目根路径管理 + 博客生成回调注册。

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
)

// ToolCall / ToolCallFunc 是工具调用的数据结构，四态机主链与子代理都用它承载
// 模型发起的调用（name + arguments），与具体执行方式（MCP）无关，故留在 core。
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function ToolCallFunc `json:"function"`
}

type ToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// BlogFunc 博客生成回调（与工具执行无关，博客 HTTP 链路在用）。
type BlogFunc func(topic string) string

var registeredBlogFunc BlogFunc

func RegisterBlogFunc(fn BlogFunc) { registeredBlogFunc = fn }

// ----- 项目根路径：可运行时切换 + 落盘持久化，不再是启动时算一次就锁死 -----
// 优先级：上次持久化的选择 > SHANXI_PROJECT_ROOT 环境变量 > 平台默认值。
// 用 atomic.Value 而不是裸 var + mutex：读多写极少（几乎只在切工作目录时写一次），
// MCP 工具执行时高频读工作目录，atomic.Load 比加锁更轻。
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

// GetProjectRoot 返回当前生效的工作目录——所有工具调用（内置 read_file/list_dir
// 与 MCP 的 fs/shell/grep）都应该用这个，不要再直接引用旧的 projectRoot 变量。
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
	stateFile := workdirStateFile()
	if err := os.MkdirAll(filepath.Dir(stateFile), 0755); err != nil {
		return fmt.Errorf("持久化工作目录失败: %w", err)
	}
	return os.WriteFile(stateFile, []byte(path), 0644)
}
