package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// nim_refresh.go —— 提供方模型列表每日重探（2026-08-02 从 NIM 专属泛化到全部提供方）。
//
// 与 free_probe.go 的分工：
//   - free_probe.go（30 分钟）：对目录里**现有条目**发最小 chat/completions 探测，
//     产出「可用性信号格 0-4」——衡量的是暂时性状态（快/慢/429/过载）。
//   - 本文件（24 小时）：拉各提供方 /v1/models 列表，对照目录做「存在性检查」——
//     模型在列表里 = 仍存在（恢复 Disabled）；不在 = 已下架/改名（标记 Disabled）。
//     避免把「暂时过载/限流」误判成「模型没了」，也避免选到 410 死模型。
//
// 为什么不「无脑新增」列表里出现的新模型：/v1/models 只说明「存在」，
// 不区分免费/付费，也没有免费额度标识。盲目把拉到的模型全加进免费池会违反
// 「免费池只收真免费档」的铁律。所以策略是：以手写目录为基线，运行时
// 动态调整各条目的 Disabled 开关；新增模型仍走「实测可用才收录」流程。

const providerListRefreshInterval = 24 * time.Hour

type providerModelList struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// disableFreeModel 运行时把某个免费模型标记为不可用（遇 400/401/403 等确定性错误时调用）。
// 用互斥锁保护全局 freeModelCatalog 切片，避免并发写冲突。
var freeCatalogMu sync.Mutex

// ==================== Disabled 状态持久化（2026-08-30 新增） ====================
// Disabled 是内存态，重启即丢——重启后死源会短暂出现在下拉里直到被重新探活/请求命中。
// 无 key 免 key 网关（Kilo/LLM7/Zen）探活零成本，完全可以在启动时立即探活并持久化结果：
//   - 启动：loadPersistedDisabledModels() 恢复上次标死的模型 → freeModelCatalog 直接置位
//   - 运行时：disableFreeModel/forceDisableFreeModel 标死时同步 persistDisabledModels()
//   - 恢复：providerListRefreshOnce 发现模型回到 /v1/models 列表 → 从持久化移除并写盘
// 文件：~/rescene_data/free_model_disabled.json（与 free_model_order.json 同目录），
// 存被标死模型的 Model 名数组（上游模型名，不是 catalog ID）。

func freeModelDisabledPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, "rescene_data", "free_model_disabled.json")
}

// loadPersistedDisabledModels 读回上次持久化的死源 Model 名集合（空文件/不存在 = 空集）。
func loadPersistedDisabledModels() map[string]bool {
	path := freeModelDisabledPath()
	if path == "" {
		return map[string]bool{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]bool{}
	}
	var names []string
	if json.Unmarshal(data, &names) != nil {
		return map[string]bool{}
	}
	set := make(map[string]bool, len(names))
	for _, n := range names {
		if n != "" {
			set[n] = true
		}
	}
	return set
}

// persistDisabledModels 把当前 freeModelCatalog 里所有 Disabled=true 的 Model 名写盘。
func persistDisabledModels() {
	path := freeModelDisabledPath()
	if path == "" {
		return
	}
	freeCatalogMu.Lock()
	names := make([]string, 0)
	for _, f := range freeModelCatalog {
		if f.Disabled && f.Model != "" {
			names = append(names, f.Model)
		}
	}
	freeCatalogMu.Unlock()
	data, _ := json.MarshalIndent(names, "", "  ")
	_ = os.WriteFile(path, data, 0600)
}

// removePersistedDisabledModel 模型恢复可用时，从持久化名单移除该 Model 名并写盘。
func removePersistedDisabledModel(model string) {
	if model == "" {
		return
	}
	set := loadPersistedDisabledModels()
	if !set[model] {
		return // 不在名单里，无需写盘
	}
	delete(set, model)
	names := make([]string, 0, len(set))
	for n := range set {
		names = append(names, n)
	}
	path := freeModelDisabledPath()
	if path == "" {
		return
	}
	data, _ := json.MarshalIndent(names, "", "  ")
	_ = os.WriteFile(path, data, 0600)
}

// applyPersistedDisabledModels 启动时把持久化死源名单应用到 freeModelCatalog
// （Disable=true 由 disableFreeModel/forceDisableFreeModel 写盘，这里是恢复）。
func applyPersistedDisabledModels() {
	set := loadPersistedDisabledModels()
	if len(set) == 0 {
		return
	}
	freeCatalogMu.Lock()
	defer freeCatalogMu.Unlock()
	applied := 0
	for i := range freeModelCatalog {
		if set[freeModelCatalog[i].Model] && !freeModelCatalog[i].Disabled {
			freeModelCatalog[i].Disabled = true
			applied++
		}
	}
	if applied > 0 {
		fmt.Printf("🗂️ [持久化] 恢复 %d 个上次标记的死源（free_model_disabled.json）\n", applied)
	}
}

// forceDisableFreeModel 强制把某个免费模型标记为不可用。
// 用于「模型不存在」类确定性错误（model_unavailable / currently unavailable）：
// 这是永久下架，不是临时故障，必须淘汰——模型若上游已不存在，
// 保留在链里只会让 auto 每次首跳白撞（2026-08-29 实锤：LLM7 模型已下架）。
func forceDisableFreeModel(model string) {
	if model == "" {
		return
	}
	freeCatalogMu.Lock()
	marked := false
	for i := range freeModelCatalog {
		if freeModelCatalog[i].Model == model && !freeModelCatalog[i].Disabled {
			freeModelCatalog[i].Disabled = true
			marked = true
			fmt.Printf("🚫 [路由自愈] 强制标记下架(模型不存在): %s (%s)\n", freeModelCatalog[i].ID, model)
		}
	}
	freeCatalogMu.Unlock()
	if marked {
		persistDisabledModels()
	}
}

func disableFreeModel(model string) {
	if model == "" {
		return
	}
	freeCatalogMu.Lock()
	marked := false
	for i := range freeModelCatalog {
		if freeModelCatalog[i].Model == model && !freeModelCatalog[i].Disabled {
			freeModelCatalog[i].Disabled = true
			marked = true
			fmt.Printf("🚫 [路由自愈] 标记不可用(HTTP确定性错误): %s (%s)\n", freeModelCatalog[i].ID, model)
		}
	}
	freeCatalogMu.Unlock()
	if marked {
		persistDisabledModels()
	}
}

// fetchProviderList 拉一次某提供方的 /v1/models，返回模型 ID 集合。
// 拉失败（网络/401/403/超时）返回 nil——调用方保留目录现状，不误判下架。
// 带浏览器 UA：部分网关（如 OpenCode Zen）对无 UA 请求返回 Cloudflare 403/1010。
func fetchProviderList(endpoint, key string) map[string]bool {
	url := strings.TrimRight(endpoint, "/") + "/models"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("⚠️ [提供方重探] %s 请求失败（保留现状）: %v\n", url, err)
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		fmt.Printf("⚠️ [提供方重探] %s HTTP %d（保留现状）: %s\n", url, resp.StatusCode, truncateChars(string(raw), 200))
		return nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("⚠️ [提供方重探] %s 读响应失败: %v\n", url, err)
		return nil
	}
	var list providerModelList
	if err := json.Unmarshal(body, &list); err != nil {
		fmt.Printf("⚠️ [提供方重探] %s 解析失败: %v\n", url, err)
		return nil
	}
	alive := make(map[string]bool, len(list.Data))
	for _, m := range list.Data {
		alive[strings.TrimSpace(m.ID)] = true
	}
	return alive
}

// providerListRefreshOnce 遍历免费池，按 Endpoint 分组，每组拉一次 /v1/models，
// 对照目录里该 Endpoint 的条目做存在性检查：在列表 → 恢复可用；不在 → 标记下架。
func providerListRefreshOnce() {
	freeCatalogMu.Lock()
	// 按 endpoint 分组（同网关多条目只拉一次列表）
	type grp struct {
		endpoint string
		key      string // 组内第一个可用 key（Keyless 组为空）
		needKey  bool   // 组内存在非 Keyless 条目（需 key 才能拉）
		models   []*FreeModelDef
	}
	groups := map[string]*grp{}
	for i := range freeModelCatalog {
		f := &freeModelCatalog[i]
		if f.Local {
			continue
		}
		g := groups[f.Endpoint]
		if g == nil {
			g = &grp{endpoint: f.Endpoint}
			groups[f.Endpoint] = g
		}
		g.models = append(g.models, f)
		if f.Keyless {
			continue
		}
		g.needKey = true
		if g.key == "" {
			if k, ok := freeEntrySavedKey(f.ID); ok && k != "" {
				g.key = k
			}
		}
		if g.key == "" {
			g.key = os.Getenv(f.KeyEnv)
		}
	}
	freeCatalogMu.Unlock()

	checked := 0
	for _, g := range groups {
		if g.needKey && g.key == "" {
			// 没配 key 的提供方拉不了列表，保留目录现状
			continue
		}
		alive := fetchProviderList(g.endpoint, g.key)
		if alive == nil {
			continue // 拉失败：无法确认，保留现状（避免误判）
		}
		disabled, enabled := 0, 0
		for _, f := range g.models {
			if alive[strings.TrimSpace(f.Model)] {
				if f.Disabled && !manuallyPinnedDeadCatalog[f.ID] {
					f.Disabled = false
					removePersistedDisabledModel(f.Model) // 活回列表 → 清持久化死源标记
					enabled++
					fmt.Printf("✅ [提供方重探] 恢复可用: %s (%s)\n", f.ID, f.Model)
				}
			} else {
				if !f.Disabled {
					f.Disabled = true
					disabled++
					fmt.Printf("🚫 [提供方重探] 标记退役(下架): %s (%s)\n", f.ID, f.Model)
				}
			}
			checked++
		}
		fmt.Printf("🔄 [提供方重探] %s：列表 %d 个模型，目录 %d 条目，本批禁用 %d / 恢复 %d\n",
			g.endpoint, len(alive), len(g.models), disabled, enabled)
	}
	if checked == 0 {
		fmt.Printf("🔄 [提供方重探] 完成：无可检查条目（无 key 或拉取失败）\n")
	}
}

// startProviderDailyRefresh 每日拉一次各提供方模型列表；首次立即跑（启动即校准），
// 之后每 24h 一次。与 startFreeProbeLoop（30min 可用性探活）互补。
func startProviderDailyRefresh() {
	go func() {
		// 启动延迟 8s：等服务器起来 + 错开探活首轮（3s）
		time.Sleep(8 * time.Second)
		providerListRefreshOnce()
		ticker := time.NewTicker(providerListRefreshInterval)
		defer ticker.Stop()
		for range ticker.C {
			providerListRefreshOnce()
		}
	}()
}
