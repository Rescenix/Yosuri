// internal/handler/model_config_handler.go
//
// 用户自定义 API 接入配置的存储。目前还没有接 QQ 登录，所有配置先按固定的
// "default" 用户标识存一份；等 openid 落地后把 userKey 换成真实 openid 就行，
// 存储路径本身已经按最终形态（~/rescene_data/user_configs/{openid}.json）写好了。
//
// ⚠️ 安全现状：这里存的是明文 JSON，AES-256 加密跟 QQ 登录一起放到下一阶段。
// GET 接口不会把 API Key 原样吐回浏览器（只返回 api_key_set 布尔值），但这
// 不代表磁盘上的存储本身是安全的——加密之前不要把真实密钥丢进这个文件测试。
package handler

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// ModelConfigModel 是自定义提供方通过 /models 暴露的一个模型。
// 能力字段通常不会出现在 OpenAI 兼容的模型目录里，未知时保持零值。
type ModelConfigModel struct {
	ID            string `json:"id"`
	Name          string `json:"name,omitempty"`
	Vision        bool   `json:"vision,omitempty"`
	ContextWindow int    `json:"context_window,omitempty"`
	Reasoning     bool   `json:"reasoning,omitempty"`
}

// ModelConfigEntry 用户自己配置的一个 API 提供方。
// Models 保存该提供方通过 /models 返回的完整目录；DefaultModel 仅用于兼容旧配置
// 以及“自动”调用链需要从一个提供方挑选单个默认模型的场景。
type ModelConfigEntry struct {
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	Endpoint      string             `json:"endpoint"`
	APIKey        string             `json:"api_key,omitempty"` // 只在请求体里写入时使用，响应里永远清空
	APIKeySet     bool               `json:"api_key_set"`       // 响应里用这个告诉前端"已经存了一把 key"
	Keyless       bool               `json:"keyless,omitempty"` // /models 与聊天接口均不要求 Bearer Token
	DefaultModel  string             `json:"default_model"`
	IsDefault     bool               `json:"is_default"`
	Vision        bool               `json:"vision,omitempty"`
	ContextWindow int                `json:"context_window,omitempty"`
	Reasoning     bool               `json:"reasoning,omitempty"`
	Models        []ModelConfigModel `json:"models,omitempty"`
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
	dir := filepath.Join(homeDir, "rescene_data", "user_configs")
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

// customModelView 与 freeModelView 保持前端需要的公共字段，让聊天下拉框可以把
// 内置目录和用户自定义提供方目录放进同一套按 vendor 分组的模型列表。
type customModelView struct {
	ID            string `json:"id"`
	Vendor        string `json:"vendor"`
	Name          string `json:"name"`
	APIKeySet     bool   `json:"api_key_set"`
	Keyless       bool   `json:"keyless"`
	Vision        bool   `json:"vision,omitempty"`
	ContextWindow int    `json:"context_window,omitempty"`
	Reasoning     bool   `json:"reasoning,omitempty"`
	Responses     bool   `json:"responses,omitempty"` // 走 Responses 协议（服务端联网搜索）
}

const customModelIDPrefix = "custom::"

func customModelSelectionID(providerID, modelID string) string {
	return customModelIDPrefix + url.QueryEscape(providerID) + "::" + url.QueryEscape(modelID)
}

func parseCustomModelSelectionID(selectionID string) (providerID, modelID string, ok bool) {
	if !strings.HasPrefix(selectionID, customModelIDPrefix) {
		return "", "", false
	}
	parts := strings.SplitN(strings.TrimPrefix(selectionID, customModelIDPrefix), "::", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	providerID, errProvider := url.QueryUnescape(parts[0])
	modelID, errModel := url.QueryUnescape(parts[1])
	if errProvider != nil || errModel != nil || providerID == "" || modelID == "" {
		return "", "", false
	}
	return providerID, modelID, true
}

func configuredProviderModels(e ModelConfigEntry) []ModelConfigModel {
	if len(e.Models) > 0 {
		return e.Models
	}
	if strings.TrimSpace(e.DefaultModel) != "" {
		return []ModelConfigModel{{ID: e.DefaultModel, Name: e.DefaultModel}}
	}
	return nil
}

// ==================== 免费模型池排序（Auto 智能路由顺序） ====================
// 排序存 ~/rescene_data/free_model_order.json（与 sessions/cron_tasks 同目录，
// 全局共享、不分用户），格式为按优先顺序排列的免费模型 ID 数组。
// 前端「编辑模型」弹窗可上移/下移调整；顺序即 Auto 模式路由链中免费池的尝试顺序。

func freeModelOrderPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, "rescene_data", "free_model_order.json"), nil
}

func loadFreeModelOrder() []string {
	path, err := freeModelOrderPath()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil // 不存在/读失败 = 未排序，走目录默认顺序
	}
	var order []string
	if json.Unmarshal(data, &order) != nil {
		return nil
	}
	// 过滤掉目录里已不存在的 ID（如条目被移除），避免脏数据
	valid := make([]string, 0, len(order))
	for _, id := range order {
		if isFreeCatalogID(id) {
			valid = append(valid, id)
		}
	}
	return valid
}

func saveFreeModelOrder(order []string) error {
	path, err := freeModelOrderPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(order, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// freeOrderRank 返回 freeModelCatalog ID → 排序位次（0 开始）的映射。
// 路由链与 free_models 视图共用它做排序，保证「设置面板看到的顺序 = Auto 路由顺序」。
func freeOrderRank() map[string]int {
	order := loadFreeModelOrder()
	rank := make(map[string]int, len(order))
	for i, id := range order {
		rank[id] = i
	}
	return rank
}

// HandlePutFreeModelOrder PUT /api/models/free-order
// 请求体：{"order": ["free_xxx", ...]}——免费模型池的完整排序（含所有目录条目）。
// 校验：必须全是 freeModelCatalog 里的 ID；允许子集（缺失的按目录顺序排在末尾）。
func HandlePutFreeModelOrder(c *gin.Context) {
	var req struct {
		Order []string `json:"order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	seen := map[string]bool{}
	clean := make([]string, 0, len(req.Order))
	for _, id := range req.Order {
		id = strings.TrimSpace(id)
		if id == "" || !isFreeCatalogID(id) || seen[id] {
			continue
		}
		seen[id] = true
		clean = append(clean, id)
	}
	if err := saveFreeModelOrder(clean); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存排序失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
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
	customModels := make([]customModelView, 0)
	for _, e := range entries {
		entryByID[e.ID] = e
		if isFreeCatalogID(e.ID) {
			continue // 免费池条目走下面的 free_models 视图，不在自定义列表里重复出现
		}
		e.APIKeySet = e.APIKey != ""
		e.APIKey = ""
		safe = append(safe, e)
		vendor := strings.TrimSpace(e.Name)
		if vendor == "" {
			vendor = "自定义 API"
		}
		for _, model := range configuredProviderModels(e) {
			if strings.TrimSpace(model.ID) == "" {
				continue
			}
			name := strings.TrimSpace(model.Name)
			if name == "" {
				name = model.ID
			}
			customModels = append(customModels, customModelView{
				ID:            customModelSelectionID(e.ID, model.ID),
				Vendor:        vendor,
				Name:          name,
				APIKeySet:     e.APIKeySet,
				Keyless:       e.Keyless,
				Vision:        model.Vision || e.Vision,
				ContextWindow: model.ContextWindow,
				Reasoning:     model.Reasoning || e.Reasoning,
			})
		}
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
	// 免费池显示顺序 = Auto 智能路由顺序（用户可在「编辑模型」弹窗调整）；
	// 没保存过排序时保持目录声明顺序。
	orderRank := freeOrderRank()
	sort.SliceStable(freeModels, func(i, j int) bool {
		ri, iok := orderRank[freeModels[i].ID]
		rj, jok := orderRank[freeModels[j].ID]
		if iok && jok {
			return ri < rj
		}
		return iok // 有排序的在前，没排序的（目录顺序）在后
	})
	freeModelOrder := make([]string, 0, len(freeModels))
	for _, fm := range freeModels {
		freeModelOrder = append(freeModelOrder, fm.ID)
	}

	// 内置联网搜索模型（「模型」tab 独立配置，不走免费模型池）。
	// key 状态 = user_configs 里同名 ID 条目有 key，或环境变量兜底。
	searchModels := make([]map[string]any, 0, len(builtinSearchModels))
	for _, s := range builtinSearchModels {
		keySet := false
		if e, ok := entryByID[s.ID]; ok && e.APIKey != "" {
			keySet = true
		}
		if !keySet && os.Getenv(s.KeyEnv) != "" {
			keySet = true
		}
		searchModels = append(searchModels, map[string]any{
			"id": s.ID, "vendor": s.Vendor, "name": s.Name,
			"model": s.Model, "api_key_set": keySet,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"configs":         safe,
		"free_models":     freeModels,
		"custom_models":   customModels,
		"search_models":   searchModels,
		"free_model_order": freeModelOrder,
	})
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
			} else {
				req.Configs[i].APIKey = ""
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
