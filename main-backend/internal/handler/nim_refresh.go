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

// nim_refresh.go —— 每日拉取 NVIDIA NIM 最新可用模型列表，
// 将免费池里已退役/下架的 NIM 模型标记为 Disabled，避免选到 410 死模型。
//
// 为什么只做"剔除"不做"无脑新增"：NIM /v1/models 不区分免费/付费，
// 我们没有可靠的免费标识字段，盲目把拉到的模型全加进免费池会违反
// "免费池只收真免费档"的铁律。所以策略是：以手写目录为基线，
// 运行时探测哪些 NIM 模型已不可用 → 动态 Disabled。

const nimModelsURL = "https://integrate.api.nvidia.com/v1/models"

type nimModel struct {
	ID      string `json:"id"`
	OwnedBy string `json:"owned_by"`
}

type nimListResponse struct {
	Data []nimModel `json:"data"`
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

// nimRefreshOnce 拉一次 NIM 模型列表，标记免费池中不可用的 NIM 条目。
func nimRefreshOnce() {
	key := os.Getenv("NVIDIA_NIM_API_KEY")
	if key == "" {
		// 没配 Key 就不探测（探测了也没权限），保留目录原样
		return
	}
	req, err := http.NewRequest(http.MethodGet, nimModelsURL, nil)
	if err != nil {
		fmt.Printf("⚠️ [NIM刷新] 构造请求失败: %v\n", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("⚠️ [NIM刷新] 请求失败（保留目录）: %v\n", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		fmt.Printf("⚠️ [NIM刷新] HTTP %d（保留目录）: %s\n", resp.StatusCode, truncateChars(string(raw), 200))
		return
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("⚠️ [NIM刷新] 读响应失败: %v\n", err)
		return
	}
	var list nimListResponse
	if err := json.Unmarshal(body, &list); err != nil {
		fmt.Printf("⚠️ [NIM刷新] 解析失败: %v\n", err)
		return
	}
	alive := make(map[string]bool, len(list.Data))
	for _, m := range list.Data {
		alive[strings.TrimSpace(m.ID)] = true
	}

	disabled, enabled := 0, 0
	for i := range freeModelCatalog {
		f := &freeModelCatalog[i]
		if f.Vendor != "NVIDIA NIM" {
			continue
		}
		// 目录里的 Model 形如 "qwen/qwen3.5-397b-a17b"，NIM 列表 id 同构
		if alive[strings.TrimSpace(f.Model)] {
			if f.Disabled {
				f.Disabled = false
				enabled++
				fmt.Printf("✅ [NIM刷新] 恢复可用: %s\n", f.ID)
			}
		} else {
			if !f.Disabled {
				f.Disabled = true
				disabled++
				fmt.Printf("🚫 [NIM刷新] 标记退役(下架): %s (%s)\n", f.ID, f.Model)
			}
		}
	}
	fmt.Printf("🔄 [NIM刷新] 完成：本批禁用 %d / 恢复 %d（目录共 %d 个 NIM 条目）\n", disabled, enabled, countNIM())
}

func countNIM() int {
	n := 0
	for _, f := range freeModelCatalog {
		if f.Vendor == "NVIDIA NIM" {
			n++
		}
	}
	return n
}

// startNIMDailyRefresh 每日拉取一次；生产环境常驻、无"启动"概念，用 ticker 周期触发。
// 首次立即跑一次（启动即探），之后每 24h 一次。
func startNIMDailyRefresh() {
	nimRefreshOnce()
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			nimRefreshOnce()
		}
	}()
}
