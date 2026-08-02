package handler

// free_probe.go —— 免费模型池定期探活 + 信号权重（2026-08-02）。
//
// 目标：Auto 智能路由的排序不再只靠用户手排的 free_model_order.json，
// 而是「探活健康度（信号格 0-4） + LRU 使用新鲜度」实时决定权重：
//   - 定期探活：对免费池每个已配 key / 免 key 条目发最小 chat/completions
//     请求，记录延迟与成败，映射成 0-4 信号格（4 = 又快又稳）。
//   - LRU 决定权重：circuitSuccess（真实请求 200 OK）时记录 lastUsedAt，
//     最近被成功用过的模型权重更高（Auto 自动收敛到「最近用得动」的那批）。
//   - 排序：signal 降序 → lastUsedAt 近者优先 → 目录/free_order 顺序兜底。
//   - 前端「免费模型」tab 每模型一张小卡片，卡上画信号格直观显示权重。
//
// 探活只降权重不永久禁用：401/403 等确定性错误仍由真实请求路径的
// disableFreeModel 永久标记；探活信号 0 只是让 Auto 排序把它沉底。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

// probeState 单个免费模型条目的探活状态。
type probeState struct {
	signal    int           // 0-4 信号格；-1 = 尚未探测
	latency   time.Duration // 最近一次探测延迟
	lastProbe time.Time
	lastOK    bool
}

var (
	probeMu     sync.Mutex
	probeStates = map[string]*probeState{}
	// lastUsedAt 记录每个免费条目最近一次真实请求成功的时刻（LRU 新鲜度）。
	lastUsedMu     sync.Mutex
	lastUsedAt     = map[string]time.Time{}
)

const (
	// probeInterval 探活周期：30 分钟一轮（免费档 429 常见，太频繁等于自打限流）。
	probeInterval = 30 * time.Minute
	// probeTimeout 单次探活超时：比正常对话超时(45s)短，探活要快。
	probeTimeout = 12 * time.Second
	// probeLatencyFast / probeLatencyMid：信号分档阈值（成功时）。
	probeLatencyFast = 3 * time.Second
	probeLatencyMid  = 8 * time.Second
	// probeFailToZero 连续失败多少次信号打到 0（期间依次 1 → 0）。
	probeFailToZero = 2
)

// probeKey 与熔断器同键：BaseURL|Model。
func probeKey(b RouterBackend) string { return circuitKey(b) }

// probeSignal 返回某 backend 当前信号格（0-4），未探测过返回 -1。
// resolveBackends 排序与 HandleGetModelConfig 视图共用（并发安全，探活低频）。
func probeSignal(b RouterBackend) int {
	probeMu.Lock()
	defer probeMu.Unlock()
	st, ok := probeStates[probeKey(b)]
	if !ok || st.signal < 0 {
		return -1
	}
	return st.signal
}

// probeSignalByDef 是 probeSignal 的 FreeModelDef 版本（排序/视图用）。
func probeSignalByDef(f FreeModelDef) int {
	return probeSignal(RouterBackend{BaseURL: f.Endpoint, Model: f.Model})
}

// freeLastUsed 返回该条目最近一次真实请求成功的时刻（零值 = 从未成功过）。
func freeLastUsed(b RouterBackend) time.Time {
	lastUsedMu.Lock()
	defer lastUsedMu.Unlock()
	return lastUsedAt[probeKey(b)]
}

// freeLastUsedByDef 是 freeLastUsed 的 FreeModelDef 版本（排序/视图用）。
func freeLastUsedByDef(f FreeModelDef) time.Time {
	return freeLastUsed(RouterBackend{BaseURL: f.Endpoint, Model: f.Model})
}

// markFreeUsed 真实请求成功时记录 LRU 新鲜度。由 circuitSuccess 统一调用
// （该函数只对 Source=="free" 生效，正好覆盖免费池成功路径）。
func markFreeUsed(b RouterBackend) {
	lastUsedMu.Lock()
	lastUsedAt[probeKey(b)] = time.Now()
	lastUsedMu.Unlock()
}

// probeCatalogEntry 对单个条目探活一次，更新信号。
func probeCatalogEntry(f *FreeModelDef) {
	key := ""
	if e, ok := freeEntrySavedKey(f.ID); ok {
		key = e
	}
	if key == "" && !f.Local && !f.Keyless {
		key = os.Getenv(f.KeyEnv)
	}
	if key == "" && !f.Local && !f.Keyless {
		return // 没配 key 不探活，保持 -1（前端不显示信号或显示灰格）
	}
	if f.Disabled {
		recordProbeResult(f, 0, 0, false)
		return
	}
	b := RouterBackend{
		BaseURL: f.Endpoint, Model: f.Model, APIKey: key,
		IsLocal: f.Local, Keyless: f.Keyless,
	}
	start := time.Now()
	ok, status := probeChatOnce(b)
	lat := time.Since(start)
	recordProbeResult(f, lat, status, ok)
}

// freeEntrySavedKey 读用户保存的同 ID 条目 key（避免重复 loadModelConfigs 的开销，
// 直接走内存缓存版；探活低频，直接读文件也完全没问题）。
func freeEntrySavedKey(id string) (string, bool) {
	if entries, err := loadModelConfigs(""); err == nil {
		for _, e := range entries {
			if e.ID == id {
				return e.APIKey, true
			}
		}
	}
	return "", false
}

// probeChatOnce 发一次最小探活请求。返回 (是否成功, HTTP 状态码)。
// 429 视为「可用但受限」：成功路径但信号低（见 recordProbeResult）。
func probeChatOnce(b RouterBackend) (bool, int) {
	reqBody := map[string]any{
		"model":      b.Model,
		"messages":   []map[string]any{{"role": "user", "content": "hi"}},
		"max_tokens": 1,
		"stream":     false,
	}
	body, _ := json.Marshal(reqBody)
	req, err := http.NewRequest(http.MethodPost, chatCompletionsURL(b.BaseURL), bytes.NewBuffer(body))
	if err != nil {
		return false, 0
	}
	req.Header.Set("Content-Type", "application/json")
	if b.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+b.APIKey)
	}
	client := &http.Client{Timeout: probeTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return false, 0
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK, resp.StatusCode
}

// recordProbeResult 按一次探活结果更新信号格。
//   - 200：按延迟分档 4/3/2，连续失败清零
//   - 429：2 格（能用但受限，Auto 排在健康的后面）
//   - 其他失败：连续失败计数，1 次 → 1 格，≥2 次 → 0 格
func recordProbeResult(f *FreeModelDef, lat time.Duration, status int, ok bool) {
	probeMu.Lock()
	defer probeMu.Unlock()
	k := f.Endpoint + "|" + f.Model
	st := probeStates[k]
	if st == nil {
		st = &probeState{signal: -1}
		probeStates[k] = st
	}
	st.lastProbe = time.Now()
	st.latency = lat
	if ok {
		st.lastOK = true
		switch {
		case lat <= probeLatencyFast:
			st.signal = 4
		case lat <= probeLatencyMid:
			st.signal = 3
		default:
			st.signal = 2
		}
		return
	}
	st.lastOK = false
	switch {
	case status == http.StatusTooManyRequests: // 429：可用但受限
		st.signal = 2
	case st.signal <= 0:
		st.signal = 0
	default:
		st.signal--
		if st.signal < 1 {
			st.signal = 0
		}
	}
}

// probeOnce 探一轮：并行对免费池所有可探条目发最小请求。
// 启动时立即跑一次，之后由 ticker 周期触发。
func probeOnce() {
	freeCatalogMu.Lock()
	snapshot := make([]FreeModelDef, len(freeModelCatalog))
	copy(snapshot, freeModelCatalog)
	freeCatalogMu.Unlock()

	var wg sync.WaitGroup
	for i := range snapshot {
		f := &snapshot[i]
		if f.Local {
			continue // 本地模型不探（目录已无 Local 条目，防御性跳过）
		}
		wg.Add(1)
		go func(ff *FreeModelDef) {
			defer wg.Done()
			probeCatalogEntry(ff)
		}(f)
	}
	wg.Wait()
	fmt.Printf("🛰️ [免费池探活] 完成一轮：%d 个条目（并发探测）\n", len(snapshot))
}

// startFreeProbeLoop 定期探活；首次立即跑（启动即校准信号），之后每 probeInterval 一轮。
// 在 chat.go 的 init() 里与 startProviderDailyRefresh（每日列表重探）一起挂载。
func startFreeProbeLoop() {
	go func() {
		// 启动延迟 3s：等服务器起来，避免与其他 init 网络任务扎堆
		time.Sleep(3 * time.Second)
		probeOnce()
		ticker := time.NewTicker(probeInterval)
		defer ticker.Stop()
		for range ticker.C {
			probeOnce()
		}
	}()
}
