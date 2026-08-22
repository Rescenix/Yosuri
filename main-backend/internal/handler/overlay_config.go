package handler

// 悬浮球演示功能开关。这是测试性功能，默认关闭，不能悄悄影响普通用户——
// 只有用户在设置面板里主动打开过，下次启动才会创建悬浮窗（见 overlay_windows.go
// startOverlay() 里的判断）。开关本身在 main-backend 里存取，跟主进程窗口创建
// 是不同的 goroutine，所以这里只管配置读写，不碰任何 UI 代码。
import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

type overlayFeatureConfig struct {
	Enabled bool `json:"enabled"`
}

func overlayFeatureConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, "rescene_data", "overlay_feature_config.json"), nil
}

// loadOverlayFeatureConfig 读不到/解析失败一律当作关闭——宁可保守，不能因为
// 配置文件损坏就意外把测试功能打开给用户。
func loadOverlayFeatureConfig() overlayFeatureConfig {
	cfg := overlayFeatureConfig{Enabled: false}
	path, err := overlayFeatureConfigPath()
	if err != nil {
		return cfg
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(data, &cfg)
	return cfg
}

func saveOverlayFeatureConfig(cfg overlayFeatureConfig) error {
	path, err := overlayFeatureConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// OverlayEnabled 给 main 包（overlay_windows.go 的 startOverlay 和轮询循环）用：
// 每次都从磁盘重读，不缓存——这样设置面板/悬浮球右键关掉之后能立刻生效，
// 不用等下次启动（用户实测反馈："关闭了，关闭应用还是显示悬浮球"，说明只在
// 启动时读一次不够，必须是运行时能感知到的开关）。
func OverlayEnabled() bool {
	return loadOverlayFeatureConfig().Enabled
}

// DisableOverlay 给悬浮球右键"关闭"用：一步把开关关掉并落盘。
func DisableOverlay() error {
	return saveOverlayFeatureConfig(overlayFeatureConfig{Enabled: false})
}

// HandleGetOverlayConfig GET /api/overlay/config
func HandleGetOverlayConfig(c *gin.Context) {
	c.JSON(http.StatusOK, loadOverlayFeatureConfig())
}

// HandlePutOverlayConfig PUT /api/overlay/config —— 修改立即落盘，
// 但悬浮窗本身只在启动时决定建不建，改完要重启应用才生效（设置面板需要提示这一点）。
func HandlePutOverlayConfig(c *gin.Context) {
	var req overlayFeatureConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	if err := saveOverlayFeatureConfig(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, req)
}
