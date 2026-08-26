package handler

// 受保护工作区是 Agent 工具链的应用层边界，不是操作系统级沙盒：开启后，
// filesystem MCP 只能拿到当前项目根目录，工作流也会拒绝所有带有越界文件路径的调用。
// 写盘和命令仍在用户的宿主进程中执行，因此 UI 必须如实称为“受保护工作区”。

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

type protectedWorkspaceConfig struct {
	Enabled bool `json:"enabled"`
}

func protectedWorkspaceConfigPath() string {
	return filepath.Join(resceneUserDataDir(), "protected_workspace.json")
}

// ProtectedWorkspaceEnabled 对配置损坏和读取失败采取 fail-open：普通模式不能因一
// 个设置文件损坏而不可用；用户显式开启并且成功落盘后才进入受保护模式。
func ProtectedWorkspaceEnabled() bool {
	data, err := os.ReadFile(protectedWorkspaceConfigPath())
	if err != nil {
		return false
	}
	var cfg protectedWorkspaceConfig
	return json.Unmarshal(data, &cfg) == nil && cfg.Enabled
}

func saveProtectedWorkspaceConfig(cfg protectedWorkspaceConfig) error {
	path := protectedWorkspaceConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func HandleGetProtectedWorkspaceConfig(c *gin.Context) {
	c.JSON(http.StatusOK, protectedWorkspaceConfig{Enabled: ProtectedWorkspaceEnabled()})
}

// HandlePutProtectedWorkspaceConfig 更新开关后重建 MCP；filesystem server 的
// allowed directories 是启动参数，必须重建才会立刻收紧或放开。
func HandlePutProtectedWorkspaceConfig(c *gin.Context) {
	var cfg protectedWorkspaceConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	if err := saveProtectedWorkspaceConfig(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ReinitMCP()
	c.JSON(http.StatusOK, cfg)
}
