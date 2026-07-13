// internal/handler/model_config_handler.go
//
// 用户自定义 API 接入配置的存储。目前还没有接 QQ 登录，所有配置先按固定的
// "default" 用户标识存一份；等 openid 落地后把 userKey 换成真实 openid 就行，
// 存储路径本身已经按最终形态（~/.Aurora/user_configs/{openid}.json）写好了。
//
// ⚠️ 安全现状：这里存的是明文 JSON，AES-256 加密跟 QQ 登录一起放到下一阶段。
// GET 接口不会把 API Key 原样吐回浏览器（只返回 api_key_set 布尔值），但这
// 不代表磁盘上的存储本身是安全的——加密之前不要把真实密钥丢进这个文件测试。
package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// ModelConfigEntry 用户自己配置的一套 API 接入信息
type ModelConfigEntry struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Endpoint     string `json:"endpoint"`
	APIKey       string `json:"api_key,omitempty"` // 只在请求体里写入时使用，响应里永远清空
	APIKeySet    bool   `json:"api_key_set"`       // 响应里用这个告诉前端"已经存了一把 key"
	DefaultModel string `json:"default_model"`
	IsDefault    bool   `json:"is_default"`
}

// 前端在没有修改 Key 的情况下会把这个占位符原样传回来，后端据此判断"不用覆盖旧 key"
const maskedKeyPlaceholder = "••••••••"

var modelConfigMu sync.Mutex

func modelConfigFilePath(userKey string) (string, error) {
	if strings.TrimSpace(userKey) == "" {
		userKey = "default"
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(homeDir, ".Aurora", "user_configs")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return filepath.Join(dir, userKey+".json"), nil
}

func loadModelConfigs(userKey string) ([]ModelConfigEntry, error) {
	path, err := modelConfigFilePath(userKey)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []ModelConfigEntry{}, nil
		}
		return nil, err
	}
	var entries []ModelConfigEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func saveModelConfigs(userKey string, entries []ModelConfigEntry) error {
	path, err := modelConfigFilePath(userKey)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// freeModelView 是免费模型池给前端的展示形态
type freeModelView struct {
	FreeModelDef
	APIKeySet bool `json:"api_key_set"` // 用户存过 Key 或服务端环境变量里有
	IsDefault bool `json:"is_default"`
}

// HandleGetModelConfig GET /api/models/config?openid=...
// 返回用户自定义配置 + 内置免费模型池（设置面板默认展示后者）。
func HandleGetModelConfig(c *gin.Context) {
	userKey := c.Query("openid")
	entries, err := loadModelConfigs(userKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取配置失败: " + err.Error()})
		return
	}
	entryByID := make(map[string]ModelConfigEntry, len(entries))
	safe := make([]ModelConfigEntry, 0, len(entries))
	for _, e := range entries {
		entryByID[e.ID] = e
		if isFreeCatalogID(e.ID) {
			continue // 免费池条目走下面的 free_models 视图，不在自定义列表里重复出现
		}
		e.APIKeySet = e.APIKey != ""
		e.APIKey = ""
		safe = append(safe, e)
	}

	freeModels := make([]freeModelView, 0, len(freeModelCatalog))
	for _, f := range freeModelCatalog {
		v := freeModelView{FreeModelDef: f}
		if e, ok := entryByID[f.ID]; ok {
			v.APIKeySet = e.APIKey != ""
			v.IsDefault = e.IsDefault
		}
		if !v.APIKeySet && os.Getenv(f.KeyEnv) != "" {
			v.APIKeySet = true
		}
		freeModels = append(freeModels, v)
	}

	c.JSON(http.StatusOK, gin.H{"configs": safe, "free_models": freeModels})
}

// HandlePutModelConfig PUT /api/models/config?openid=...
// 请求体：{"configs": [ModelConfigEntry, ...]}，整份列表覆盖式保存。
func HandlePutModelConfig(c *gin.Context) {
	userKey := c.Query("openid")
	var req struct {
		Configs []ModelConfigEntry `json:"configs"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	modelConfigMu.Lock()
	defer modelConfigMu.Unlock()

	existing, err := loadModelConfigs(userKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取旧配置失败: " + err.Error()})
		return
	}
	existingByID := make(map[string]ModelConfigEntry, len(existing))
	for _, e := range existing {
		existingByID[e.ID] = e
	}

	// 只校验格式（非空、长度合理），不校验 Key 是否真的有效——那是用户自己的事
	for i, e := range req.Configs {
		if strings.TrimSpace(e.Endpoint) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "「" + e.Name + "」的 Endpoint 不能为空"})
			return
		}
		if e.APIKey == "" || e.APIKey == maskedKeyPlaceholder {
			// 前端没改 key（还是打码占位符，或者没填）——保留旧值，不能拿空值/占位符覆盖真实 key
			if old, ok := existingByID[e.ID]; ok {
				req.Configs[i].APIKey = old.APIKey
			}
		} else if len(e.APIKey) < 8 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "「" + e.Name + "」的 API Key 长度不合理"})
			return
		}
	}

	// GET 不把免费池条目放进 configs 列表（它们在 free_models 视图里），
	// 所以前端整表覆盖时不会带上它们——这里把磁盘上已有、且本次请求没提到的
	// 免费池条目合并回来，避免用户存过的免费模型 Key 被覆盖丢失
	incomingIDs := make(map[string]bool, len(req.Configs))
	for _, e := range req.Configs {
		incomingIDs[e.ID] = true
	}
	for _, old := range existing {
		if isFreeCatalogID(old.ID) && !incomingIDs[old.ID] {
			req.Configs = append(req.Configs, old)
		}
	}

	if err := saveModelConfigs(userKey, req.Configs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存配置失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
