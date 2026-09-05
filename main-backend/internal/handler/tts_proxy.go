// tts_proxy.go —— 自定义语音代理：把前端朗读请求转发到云端/外部 TTS。
// 支持 OpenAI 兼容格式（POST {model, voice, input} → audio/*），
// MiniMax 的 t2a_v2（{model, text, voice_setting}）也走同一兼容层。
package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// TTSConfig 自定义语音的云端配置（设置面板可改，存内存 + 磁盘）。
// Provider 取值：
//   - "openai"    OpenAI 兼容 /v1/audio/speech（MiniMax 开放平台新端点也认）
//   - "minimax"   MiniMax t2a_v2 专属格式（group_id + voice_setting）
//   - ""          未启用（回落到前端 Edge 直连）
type TTSConfig struct {
	Enabled   bool   `json:"enabled"`
	Provider  string `json:"provider"`
	BaseURL   string `json:"base_url"`   // 如 https://api.minimax.chat/v1 或 OpenAI 兼容根
	APIKey    string `json:"api_key"`    // 云端鉴权（只存本地）
	Model     string `json:"model"`      // 如 minimax-tts 或 gpt-4o-mini-tts
	Voice     string `json:"voice"`      // 如 male-qn-qingse 或 nova
	GroupID   string `json:"group_id"`   // MiniMax 专属
	Speed     float64 `json:"speed"`     // 语速倍率（OpenAI 兼容 tts 用）
	TestOK    bool   `json:"test_ok"`    // 上次测试结果（前端显示连通性）
	VoiceName string `json:"voice_name"` // 前端展示名（如「自定义·云端少女」）
}

var ttsConfig = TTSConfig{}

func defaultTTSConfig() TTSConfig {
	return TTSConfig{
		Provider: "openai",
		BaseURL:  "https://api.minimax.chat/v1",
		Model:    "minimax-tts",
		Voice:    "male-qn-qingse",
		Speed:    1.0,
	}
}

// GET /api/tts/config 读当前自定义语音配置（APIKey 打码返回）
func HandleGetTTSConfig(c *gin.Context) {
	cfg := ttsConfig
	if cfg.APIKey != "" {
		k := cfg.APIKey
		if len(k) > 6 {
			cfg.APIKey = k[:2] + "****" + k[len(k)-2:]
		} else {
			cfg.APIKey = "****"
		}
	}
	c.JSON(http.StatusOK, cfg)
}

// POST /api/tts/config 保存自定义语音配置
func HandleSaveTTSConfig(c *gin.Context) {
	var body TTSConfig
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// 允许留空 api_key（表示不改）？不行——保存即覆盖，但空 api_key = 清除
	ttsConfig = body
	if err := saveTTSConfigFile(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ttsConfigPath 配置落盘位置（rescene_data 下，跟其他用户数据同目录）
func ttsConfigPath() string {
	base := os.Getenv("RESCENE_DATA_DIR")
	if base == "" {
		base = "rescene_data"
	}
	return filepath.Join(base, "tts_config.json")
}

func saveTTSConfigFile() error {
	b, _ := json.MarshalIndent(ttsConfig, "", "  ")
	return os.WriteFile(ttsConfigPath(), b, 0o600)
}

func LoadTTSConfigFile() {
	b, err := os.ReadFile(ttsConfigPath())
	if err != nil {
		return // 没有配置文件 = 用默认（未启用）
	}
	var cfg TTSConfig
	if json.Unmarshal(b, &cfg) == nil {
		ttsConfig = cfg
	}
}

// SpeakRequest 前端朗读请求体
type SpeakRequest struct {
	Text  string  `json:"text"`
	Model string  `json:"model"`
	Voice string  `json:"voice"`
	Speed float64 `json:"speed"`
}

// HandleTTSSpeak POST /api/tts/speak —— 把文本转发到云端 TTS 合成音频。
// 用当前保存的 ttsConfig（provider/base_url/api_key），支持：
//   - openai 兼容：POST {base}/audio/speech，body {model, voice, input, speed}
//   - minimax t2a_v2：POST {base}/t2a_v2，body {model, text, voice_setting}
//
// 成功时原样回传 audio/* 流；失败回 JSON 错误。
func HandleTTSSpeak(c *gin.Context) {
	cfg := ttsConfig
	if !cfg.Enabled || cfg.APIKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "自定义语音未启用或未填密钥，请先到设置里配置"})
		return
	}
	var req SpeakRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Text == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "text 不能为空"})
		return
	}
	model := cfg.Model
	if req.Model != "" {
		model = req.Model
	}
	voice := cfg.Voice
	if req.Voice != "" {
		voice = req.Voice
	}
	speed := cfg.Speed
	if req.Speed > 0 {
		speed = req.Speed
	}

	var reqBody []byte
	var url string
	var contentType string
	switch strings.ToLower(cfg.Provider) {
	case "minimax":
		url = strings.TrimRight(cfg.BaseURL, "/") + "/t2a_v2"
		contentType = "application/json"
		body := map[string]interface{}{
			"model":   model,
			"text":    req.Text,
			"stream":  false,
			"voice_setting": map[string]interface{}{
				"voice_id": voice,
				"speed":    speed,
				"vol":      1,
				"pitch":    0,
			},
		}
		reqBody, _ = json.Marshal(body)
	default: // openai 兼容
		url = strings.TrimRight(cfg.BaseURL, "/") + "/audio/speech"
		contentType = "application/json"
		body := map[string]interface{}{
			"model": model,
			"input": req.Text,
			"voice": voice,
			"speed": speed,
		}
		reqBody, _ = json.Marshal(body)
	}

	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "构造请求失败: " + err.Error()})
		return
	}
	httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	httpReq.Header.Set("Content-Type", contentType)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "云端 TTS 请求失败: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "读取云端响应失败: " + err.Error()})
		return
	}
	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadGateway, gin.H{
			"error":  "云端 TTS 报错（HTTP " + resp.Status + "）",
			"detail": truncate(string(respBody), 300),
		})
		return
	}
	// 能解析为 JSON = 不是音频（是错误 JSON 体）
	var probe interface{}
	if json.Unmarshal(respBody, &probe) == nil {
		if m, ok := probe.(map[string]interface{}); ok {
			if _, has := m["error"]; has || m["status"] != nil {
				c.JSON(http.StatusBadGateway, gin.H{"error": "云端 TTS 返回非音频数据", "detail": truncate(string(respBody), 300)})
				return
			}
		}
	}

	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "audio/mpeg"
	}
	c.Header("Content-Type", ct)
	c.Header("Cache-Control", "no-store")
	c.Status(http.StatusOK)
	c.Writer.Write(respBody)
}

// truncate 安全截断字符串（避免错误体过大）
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// POST /api/tts/test —— 发一句「测试」到云端 TTS，验通+预存结果
func HandleTTSTest(c *gin.Context) {
	cfg := ttsConfig
	if !cfg.Enabled || cfg.APIKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error": "自定义语音未启用或未填密钥"})
		return
	}
	url := strings.TrimRight(cfg.BaseURL, "/") + "/audio/speech"
	body, _ := json.Marshal(map[string]interface{}{
		"model": cfg.Model,
		"input": "你好，语音测试成功。",
		"voice": cfg.Voice,
		"speed": cfg.Speed,
	})
	httpReq, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(httpReq)
	if err != nil {
		ttsConfig.TestOK = false
		saveTTSConfigFile()
		c.JSON(http.StatusOK, gin.H{"ok": false, "error": "请求失败: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		ttsConfig.TestOK = false
		saveTTSConfigFile()
		c.JSON(http.StatusOK, gin.H{"ok": false, "error": "HTTP " + resp.Status, "detail": truncate(string(respBody), 200)})
		return
	}
	ttsConfig.TestOK = true
	saveTTSConfigFile()
	c.JSON(http.StatusOK, gin.H{"ok": true, "bytes": len(respBody)})
}
