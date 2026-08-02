package handler

// shared_pool.go —— Rescene 共享池（2026-08-02）。
//
// 把用户的共享 Key 放在 ResceneCloud 云端，用户注册后即可免费试用，
// 无需自己填 Key。限流在云端执行，开源 re0 无法绕过。
//
// 两个端点：
//   GET  /api/models/shared-pool       → 返回云端共享池可用模型列表
//   POST /api/chat/shared-pool          → 代理聊天请求到云端（流式 SSE）
//
// 共享池模式切换：
//   前端发起聊天时，如果用户选了「免费模式」，走 /api/chat/shared-pool；
//   如果用户填了自己的 Key，走原有的 /api/code/workflow 本地路由。
//
// ResceneCloud 端需要实现：
//   POST /v1/chat/completions（限流 + 共享 Key 路由）
//   GET  /api/models/config（返回共享池可用模型，含 free_model_order）

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// SharedPoolEnabled 返回是否启用了共享池（服务端设了任意共享 Key）。
func SharedPoolEnabled() bool {
	envVars := []string{"SENSENOVA_API_KEY", "MODELSCOPE_API_KEY", "NVIDIA_NIM_API_KEY", "DEEPSEEK_API_KEY", "STEP_API_KEY", "OLLAMA_API_KEY"}
	for _, key := range envVars {
		if os.Getenv(key) != "" {
			return true
		}
	}
	return false
}

// HandleGetSharedPoolModels GET /api/models/shared-pool
// 返回共享池中可用的模型列表（由 ResceneCloud 提供）。
// 如果本地 re0 直接配了 Key，也返回本地可用的 free_models。
func HandleGetSharedPoolModels(c *gin.Context) {
	// 优先从 ResceneCloud 获取共享池模型列表
	cloudURL := os.Getenv("RESCENE_CLOUD_URL")
	if cloudURL == "" {
		cloudURL = "https://rescenecloud.onrender.com"
	}
	cloudURL = strings.TrimRight(cloudURL, "/")

	// 透传 Authorization（用户 JWT）
	auth := c.GetHeader("Authorization")

	req, _ := http.NewRequest("GET", cloudURL+"/api/models/config", nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// 云端不可达时，回退本地 free_models（如果本地有 Key）
		if SharedPoolEnabled() {
			HandleGetModelConfig(c)
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "共享池不可用，请检查网络连接",
			"enabled": false,
		})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "解析共享池响应失败"})
		return
	}
	// 标记这是共享池的模型
	data["shared_pool"] = true
	c.JSON(resp.StatusCode, data)
}

// HandleSharedPoolChat POST /api/chat/shared-pool
// 将聊天请求代理到 ResceneCloud 共享池，支持流式 SSE。
// 云端负责：鉴权（JWT）→ 限流（每日配额）→ 用共享 Key 路由到上游。
func HandleSharedPoolChat(c *gin.Context) {
	cloudURL := os.Getenv("RESCENE_CLOUD_URL")
	if cloudURL == "" {
		cloudURL = "https://rescenecloud.onrender.com"
	}
	cloudURL = strings.TrimRight(cloudURL, "/")

	// 读取请求体
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "读取请求体失败"})
		return
	}

	// 构造转发请求到云端
	target := cloudURL + "/v1/chat/completions"
	req, err := http.NewRequest("POST", target, bytes.NewReader(bodyBytes))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "构造请求失败: " + err.Error()})
		return
	}
	req.Header.Set("Content-Type", "application/json")

	// 透传用户 JWT（云端鉴权 + 限流用）
	if auth := c.GetHeader("Authorization"); auth != "" {
		req.Header.Set("Authorization", auth)
	}

	// 发起请求
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "连接共享池失败: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	// 检查是否限流（429）
	if resp.StatusCode == http.StatusTooManyRequests {
		body, _ := io.ReadAll(resp.Body)
		c.Data(http.StatusTooManyRequests, "application/json", body)
		return
	}

	// 判断是否是流式响应（SSE）
	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/event-stream") {
		// 流式：逐行转发到前端
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Status(http.StatusOK)

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
		for scanner.Scan() {
			line := scanner.Text()
			c.Writer.Write([]byte(line + "\n"))
			c.Writer.Flush()
		}
	} else {
		// 非流式：直接透传
		body, _ := io.ReadAll(resp.Body)
		for k, v := range resp.Header {
			for _, vv := range v {
				c.Header(k, vv)
			}
		}
		c.Data(resp.StatusCode, ct, body)
	}
}