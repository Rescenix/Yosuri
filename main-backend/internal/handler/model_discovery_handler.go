package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const maxModelCatalogBytes = 4 << 20

func providerModelsURL(endpoint string) string {
	base := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	base = strings.TrimSuffix(base, "/chat/completions")
	if strings.HasSuffix(base, "/models") {
		return base
	}
	return base + "/models"
}

func fetchProviderModels(ctx context.Context, endpoint, apiKey string) ([]ModelConfigModel, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, providerModelsURL(endpoint), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("连接模型目录失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxModelCatalogBytes+1))
	if err != nil {
		return nil, fmt.Errorf("读取模型目录失败: %w", err)
	}
	if len(body) > maxModelCatalogBytes {
		return nil, fmt.Errorf("模型目录响应超过 4 MB")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("模型目录返回 HTTP %d: %s", resp.StatusCode, truncateChars(string(body), 240))
	}

	var payload struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("模型目录不是兼容的 JSON: %w", err)
	}

	seen := make(map[string]bool, len(payload.Data))
	models := make([]ModelConfigModel, 0, len(payload.Data))
	for _, item := range payload.Data {
		id := strings.TrimSpace(item.ID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = id
		}
		models = append(models, ModelConfigModel{ID: id, Name: name})
		if len(models) >= 1000 {
			break
		}
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("提供方没有返回任何模型")
	}
	return models, nil
}

// HandleDiscoverProviderModels POST /api/models/discover?openid=...
// 使用用户填写的 Endpoint + Key 请求 OpenAI 兼容 /models。编辑已有提供方时，
// 前端无需取回明文 Key，只要传 config_id，后端会复用磁盘中保存的 Key。
func HandleDiscoverProviderModels(c *gin.Context) {
	var req struct {
		ConfigID string `json:"config_id"`
		Endpoint string `json:"endpoint"`
		APIKey   string `json:"api_key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	if strings.TrimSpace(req.Endpoint) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Endpoint 不能为空"})
		return
	}

	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey == maskedKeyPlaceholder {
		apiKey = ""
	}
	if apiKey == "" && strings.TrimSpace(req.ConfigID) != "" {
		entries, err := loadModelConfigs(c.Query("openid"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "读取旧配置失败: " + err.Error()})
			return
		}
		for _, entry := range entries {
			if entry.ID == req.ConfigID {
				apiKey = entry.APIKey
				break
			}
		}
	}

	models, err := fetchProviderModels(c.Request.Context(), req.Endpoint, apiKey)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"models": models})
}
