package handler

// 性能模式开关。低配/核显机器上，WebView2 的 GPU 进程在界面重绘时容易吃满 3D 引擎
// （流式输出、常驻动效叠加时 GPU 占用可飙到 80%+），表现为"带不动/卡顿"。打开性能
// 模式后，桌面壳在创建 WebView2 环境时注入 --disable-gpu，界面改走软件渲染：GPU 占用
// 归零，代价是渲染压力转到 CPU。开关持久化在本地，改动需重启应用才生效（WebView2 环境
// 只在启动时创建一次）。默认关闭——不影响正常机器的硬件加速。
import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

type perfFeatureConfig struct {
	GPUDisabled bool `json:"gpu_disabled"`
}

func perfFeatureConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, "rescene_data", "perf_feature_config.json"), nil
}

// loadPerfFeatureConfig 读不到/解析失败一律当作关闭——保守处理，不能因为配置文件损坏
// 就悄悄把软件渲染打开，让正常机器的用户白白损失硬件加速。
func loadPerfFeatureConfig() perfFeatureConfig {
	cfg := perfFeatureConfig{GPUDisabled: false}
	path, err := perfFeatureConfigPath()
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

func savePerfFeatureConfig(cfg perfFeatureConfig) error {
	path, err := perfFeatureConfigPath()
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

// GPUDisabled 给 main 包用：启动时读一次，决定是否给 WebView2 注入 --disable-gpu。
// 这里缓存无所谓，因为开关只在启动时生效，运行中改了也要重启才起作用。
func GPUDisabled() bool {
	return loadPerfFeatureConfig().GPUDisabled
}

// HandleGetPerfConfig GET /api/perf/config
func HandleGetPerfConfig(c *gin.Context) {
	c.JSON(http.StatusOK, loadPerfFeatureConfig())
}

// HandlePutPerfConfig PUT /api/perf/config —— 修改立即落盘，但 WebView2 环境只在启动时
// 创建，改完要重启应用才生效（设置面板需提示这一点）。
func HandlePutPerfConfig(c *gin.Context) {
	var cfg perfFeatureConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体格式错误"})
		return
	}
	if err := savePerfFeatureConfig(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存失败"})
		return
	}
	c.JSON(http.StatusOK, cfg)
}
