package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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

func disableFreeModel(model string) {
	if model == "" {
		return
	}
	freeCatalogMu.Lock()
	defer freeCatalogMu.Unlock()
	for i := range freeModelCatalog {
		if freeModelCatalog[i].Model == model && !freeModelCatalog[i].Disabled {
			freeModelCatalog[i].Disabled = true
			fmt.Printf("🚫 [路由自愈] 标记不可用(HTTP确定性错误): %s (%s)\n", freeModelCatalog[i].ID, model)
		}
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
				if f.Disabled {
					f.Disabled = false
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
