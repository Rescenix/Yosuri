package handler

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
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

	client := backendHTTPClient(RouterBackend{BaseURL: endpoint}, 30*time.Second, false)
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

// ==================== 免费池自动发现（提供方粒度） ====================
// 一个提供方一把 key，自动拉它的 /v1/models，全部模型直接进池（2026-08-04）。
// 自定义 API 能动态发现是因为前端手动点「发现模型」→ /api/models/discover；
// 免费池之前是写死目录（freeModelCatalog），现在把同一个机制自动接到免费池：
// 用户给某个提供方（如 StepFun）配过 key，它的全部模型自动出现，不用逐个人工录入。

const freePoolDiscTTL = 6 * time.Hour

type discoveredProvider struct {
	Endpoint string
	KeyEnv   string
	Keyless  bool
	Vendor   string
	Models   []ModelConfigModel
}

var (
	discMu      sync.Mutex
	discCache   []discoveredProvider // 免费池各提供方的模型快照
	discFetched time.Time            // 上次拉取时间
	discUserKey string               // 拉取时用的 userKey（key 集合变了要重拉）
)

// WarmFreePoolDiscovery 启动时后台预热（main 调用），不阻塞启动。
func WarmFreePoolDiscovery() {
	go ensureFreePoolDiscovery("")
}

// ensureFreePoolDiscovery 保证免费池发现缓存新鲜：TTL 内直接用，过期/换用户则
// 同步并行拉所有「有 key 的提供方」的 /v1/models；单源失败静默（保持现状），
// 不阻塞整体。
func ensureFreePoolDiscovery(userKey string) {
	discMu.Lock()
	if userKey == discUserKey && time.Since(discFetched) < freePoolDiscTTL {
		discMu.Unlock()
		return
	}
	discMu.Unlock()

	envKeys := userKeysByEnv(userKey)
	// 按 endpoint 去重：同提供方多个目录条目只拉一次
	type prov struct {
		ep, keyEnv, vendor string
		keyless            bool
	}
	seen := map[string]prov{}
	for _, f := range freeModelCatalog {
		if f.Disabled || f.Local {
			continue
		}
		if f.KeyEnv != "" {
			if envKeys[f.KeyEnv] == "" && os.Getenv(f.KeyEnv) == "" && !f.Keyless {
				continue // 没 key 的提供方不拉
			}
		} else if !f.Keyless {
			continue
		}
		if _, ok := seen[f.Endpoint]; !ok {
			seen[f.Endpoint] = prov{ep: f.Endpoint, keyEnv: f.KeyEnv, vendor: f.Vendor, keyless: f.Keyless}
		}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make([]discoveredProvider, 0, len(seen))
	for _, p := range seen {
		wg.Add(1)
		go func(p prov) {
			defer wg.Done()
			key := ""
			if !p.keyless {
				key = envKeys[p.keyEnv]
				if key == "" {
					key = os.Getenv(p.keyEnv)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()
			models, err := fetchProviderModels(ctx, p.ep, key)
			if err != nil || len(models) == 0 {
				return // 拉不到保持现状，静默
			}
			mu.Lock()
			results = append(results, discoveredProvider{
				Endpoint: p.ep, KeyEnv: p.keyEnv, Keyless: p.keyless, Vendor: p.vendor, Models: models,
			})
			mu.Unlock()
		}(p)
	}
	wg.Wait()

	discMu.Lock()
	discCache = results
	discFetched = time.Now()
	discUserKey = userKey
	discMu.Unlock()
}

// catalogHasModel 目录里是否已有同名上游模型（避免自动发现重复）。
// 按 (endpoint, model) 区分：普通 v1 与 Step Plan 订阅端点是两个体系，
// 同名模型（如 step-3.7-flash）可以在两个分组各出现一次，用户可区分走
// 余额还是走订阅 Credit（2026-08-04）。
func catalogHasModel(upstreamID, endpoint string) bool {
	for _, f := range freeModelCatalog {
		if f.Endpoint == endpoint && (f.Model == upstreamID || f.ID == upstreamID) {
			return true
		}
	}
	return false
}

// discoveredFreeModels 把自动发现结果转成 free_models 视图条目（auto_ 前缀 ID，
// 前端直接分组显示，api_key_set=true 保证进下拉）。
func discoveredFreeModels(userKey string) []freeModelView {
	ensureFreePoolDiscovery(userKey)
	discMu.Lock()
	defer discMu.Unlock()
	var out []freeModelView
	for _, p := range discCache {
		for _, m := range p.Models {
			if catalogHasModel(m.ID, p.Endpoint) {
				continue
			}
			id := "auto_" + sanitizeID(p.Vendor) + "_" + hexEncode(m.ID)
			out = append(out, freeModelView{
				FreeModelDef: FreeModelDef{
					ID: id, Vendor: p.Vendor, Name: m.Name + "（自动发现）",
					Endpoint: p.Endpoint, Model: m.ID, KeyEnv: p.KeyEnv, Keyless: p.Keyless,
				},
				APIKeySet: true,
			})
		}
	}
	return out
}

// sanitizeID 去掉 vendor 名里的非字母数字（只做 ID 组件，不展示）
func sanitizeID(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return strings.ToLower(b.String())
}

// hexEncode 用 hex 编码上游模型 ID（可能含 / : 等特殊字符，不能直接进 URL 参数）
func hexEncode(s string) string {
	return hex.EncodeToString([]byte(s))
}

// resolveAutoDiscovered 把 auto_ 前缀的自动发现模型解析成 RouterBackend
// （endpoint + key 来自发现快照；上游模型 ID 是真实 ID，直接进 Model 字段）。
func resolveAutoDiscovered(userKey, model string, envKeys map[string]string) *RouterBackend {
	for _, fm := range discoveredFreeModels(userKey) {
		if fm.ID != model {
			continue
		}
		key := ""
		if !fm.Keyless {
			key = envKeys[fm.KeyEnv]
			if key == "" {
				key = os.Getenv(fm.KeyEnv)
			}
			if key == "" {
				return nil
			}
		}
		return &RouterBackend{
			ID: fm.ID, Name: fm.Name, BaseURL: fm.Endpoint, Model: fm.Model,
			APIKey:  key,
			Timeout: 45 * time.Second, Source: "free",
			Keyless: fm.Keyless,
		}
	}
	return nil
}

// autoReadableID 把自动发现模型的内部 ID（auto_<vendor>_<hex>）解码成可读形式：
// auto_<vendor>_<真实模型名>。hex 是上游模型 ID 的 hex 编码（可能含 / : 等字符，
// 不能直接进 URL 参数，所以内部用 hex；对外展示时解码回真实名）。
// 解码失败/非 auto_ 前缀时原样返回。
func autoReadableID(id string) string {
	if !strings.HasPrefix(id, "auto_") {
		return id
	}
	idx := strings.LastIndex(id, "_")
	if idx < 0 {
		return id
	}
	hexPart := id[idx+1:]
	raw, err := hex.DecodeString(hexPart)
	if err != nil || len(raw) == 0 {
		return id
	}
	return id[:idx+1] + string(raw)
}

// resolveAutoReadable 把 /v1/models 暴露的可读 auto_ ID（auto_<vendor>_<真实模型名>）
// 反解回发现快照的原始条目并精确路由。这是聚合 API 的入口：外部工具从
// /v1/models 拿到可读 ID 后原样填回 model 字段，这里负责匹配。找不到返回 nil。
// 已淘汰（autoDisabled）的模型不路由——列表里没有的模型不该还能用。
func resolveAutoReadable(userKey, model string) *RouterBackend {
	if !strings.HasPrefix(model, "auto_") {
		return nil
	}
	envKeys := userKeysByEnv(userKey)
	for _, fm := range discoveredFreeModels(userKey) {
		if autoReadableID(fm.ID) == model {
			if isAutoModelDisabled(fm.Endpoint, fm.Model) {
				return nil
			}
			return resolveAutoDiscovered(userKey, fm.ID, envKeys)
		}
	}
	return nil
}
