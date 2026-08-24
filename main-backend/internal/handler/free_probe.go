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
	"strings"
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
	lastUsedMu sync.Mutex
	lastUsedAt = map[string]time.Time{}
	// lastLatency 记录每个条目最近一次真实请求成功的延迟（用于 auto 排序，零额外探活）。
	lastLatencyMu sync.Mutex
	lastLatency   = map[string]time.Duration{}
	// autoDisabled 记录自动发现模型（auto_ 前缀）的确定性不可用标记：
	// key = endpoint|model（与 probeStates 同键）。401/403/404 等确定性错误
	// 时标记；探活成功（200）自动移除（拉起）。聚合 API /v1/models 输出时
	// 跳过被标记的模型，避免外部工具选到付费墙/已下架的模型。
	autoDisabledMu sync.Mutex
	autoDisabled   = map[string]bool{}
	// aggAutoTier 聚合端口 auto 候选梯队（2026-08-13 用户「auto 应该有预备役机制」）：
	// key = backend ID，value = 梯队编号（1=最快/60s, 2=备用/120s, 3=兜底/180s）。
	// 梯队越小探测越频繁，auto 链按梯队+延迟排序，最快的先上。
	// ⚠️ 2026-08-13 废除探活梯队：商汤 5h/500 次探活烧额度。
	// 改为完全靠真实请求延迟排序，零额外探测。
	aggAutoTierMu sync.Mutex
	aggAutoTier   = map[string]int{}
)

// isProtectedModel 判断模型是否受保护（DeepSeek 系永不淘汰）。
// 用户明确要求：DeepSeek 是核心卖点模型，即使上游临时 401/403/404 或探活
// 失败，也不从聚合池移除（真实请求失败由熔断/failover 兜底，冷却后自愈）。
func isProtectedModel(model string) bool {
	return strings.Contains(strings.ToLower(model), "deepseek")
}

// disableAutoModel 标记一个自动发现模型确定性不可用（401/403/404）。
// 注意：自动发现（auto_）模型按实际可用性淘汰——Kilo/Ollama/Zen 等
// 付费墙 DeepSeek（401/403）照样淘汰，不能因为名字带 deepseek 就保活成
// "付费墙占位"；目录里精选的免费 DeepSeek 由 disableFreeModel 保活。
func disableAutoModel(endpoint, model string) {
	if endpoint == "" || model == "" {
		return
	}
	autoDisabledMu.Lock()
	autoDisabled[endpoint+"|"+model] = true
	autoDisabledMu.Unlock()
}

// enableAutoModel 探活恢复时移除不可用标记（拉起）。
func enableAutoModel(endpoint, model string) {
	autoDisabledMu.Lock()
	delete(autoDisabled, endpoint+"|"+model)
	autoDisabledMu.Unlock()
}

// manuallyDeadAutoModels 人工审计确认「确定下架或付费」的自动发现模型（endpoint|model）。
// 2026-08-21 实测：用鲁棒长句探活全部免费池 + 自动发现（魔搭/商汤/StepFun 三个整账号
// 免费额度制厂商），这批返回的是明确的「不存在/无 provider 支撑/无访问权限」错误，
// 不是偶发的空回复抖动（那类疑罪从无，留给 30 分钟探活信号自然降权，不进这份名单）：
//   - 魔搭 MiniMax/MiniMax-M3、Qwen/Qwen3-4B：400 "has no provider supported"
//   - 商汤 sensenova-u1-fast：404 "model is not found"
//   - StepFun 一批 TTS/ASR/搜索/订阅制端点：404 "does not exist or you do not have
//     access to it"——订阅制体系，免费 key 没权限，且本来就是非聊天模型
//
// autoDisabled 是运行时内存态（进程重启/自动发现快照刷新会丢），这份是持久名单，
// 与 isAutoModelDisabled 合并判定，保证这批不会再冒出来给用户选。
var manuallyDeadAutoModels = map[string]bool{
	"https://api-inference.modelscope.cn/v1|MiniMax/MiniMax-M3": true,
	"https://api-inference.modelscope.cn/v1|Qwen/Qwen3-4B":      true,
	"https://token.sensenova.cn/v1|sensenova-u1-fast":           true,
	"https://api.stepfun.com/v1|dr-search-api":                  true,
	"https://api.stepfun.com/v1|search-image":                   true,
	"https://api.stepfun.com/v1|step-2x-large":                  true,
	"https://api.stepfun.com/v1|step-asr":                       true,
	"https://api.stepfun.com/v1|step-asr-1.1":                   true,
	"https://api.stepfun.com/v1|step-asr-1.1-stream":            true,
	"https://api.stepfun.com/v1|step-audio-2-think":             true,
	"https://api.stepfun.com/v1|step-image-edit-2":              true,
	"https://api.stepfun.com/v1|step-overture-preview":          true,
	"https://api.stepfun.com/v1|step-tts-2":                     true,
	"https://api.stepfun.com/v1|step-tts-mini":                  true,
	"https://api.stepfun.com/v1|step-tts-vivid":                 true,
	"https://api.stepfun.com/v1|stepaudio-2-asr-pro":            true,
	"https://api.stepfun.com/v1|stepaudio-2.5-asr":              true,
	"https://api.stepfun.com/v1|stepaudio-2.5-asr-stream":       true,
	"https://api.stepfun.com/v1|stepaudio-2.5-realtime":         true,
	"https://api.stepfun.com/v1|stepaudio-2.5-tts":              true,
}

// manuallyPinnedDeadCatalog 人工确认「上游仍挂在 /v1/models 列表里、但实际调用会挂」的
// 目录条目（catalog ID）。2026-08-21：free_zen_deepseek_v4_flash 实测 HTTP 400
// "Model is unavailable"——nim_refresh.go 的每日重探只做「存在性检查」（模型还在
// /v1/models 列表里就判定「仍可用」），检测不出「listed 但调用挂」这种情况，会在下次
// 启动/24h 重探时把手动 Disabled 悄悄拨回 false。这份名单让 providerListRefreshOnce
// 的自动恢复逻辑跳过这些条目，人工禁用才真正是"永久"的（除非从这份名单里删掉）。
var manuallyPinnedDeadCatalog = map[string]bool{
	"free_zen_deepseek_v4_flash": true,
}

// isAutoModelDisabled 查询自动发现模型是否被标记不可用（运行时探活淘汰 ∪ 人工审计名单）。
func isAutoModelDisabled(endpoint, model string) bool {
	if manuallyDeadAutoModels[endpoint+"|"+model] {
		return true
	}
	autoDisabledMu.Lock()
	defer autoDisabledMu.Unlock()
	return autoDisabled[endpoint+"|"+model]
}

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

// markFreeUsed 真实请求成功时记录 LRU 新鲜度 + 延迟。由 circuitSuccess 统一调用
// （该函数只对 Source=="free" 生效，正好覆盖免费池成功路径）。
// 延迟用于 auto 排序（零额外探活，纯真实使用数据）。
func markFreeUsed(b RouterBackend) {
	lastUsedMu.Lock()
	lastUsedAt[probeKey(b)] = time.Now()
	lastUsedMu.Unlock()
	// 记录真实请求延迟（从 circuitSuccess 捎带，不额外烧额度）
	lastLatencyMu.Lock()
	lastLatency[probeKey(b)] = b.Timeout
	lastLatencyMu.Unlock()
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
	client := backendHTTPClient(b, probeTimeout, false)
	resp, err := client.Do(req)
	if err != nil {
		return false, 0
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, resp.StatusCode
	}
	// 200 但 usage=0 = 请求未真正处理（魔搭 DS 间歇空回复 bug，实测 usage=0 空 content）
	// → 判失败，防「200 空回复」模型靠状态码混进可用池（2026-08-13）
	var probeResp struct {
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}
	if json.NewDecoder(resp.Body).Decode(&probeResp) == nil && probeResp.Usage.TotalTokens == 0 {
		return false, resp.StatusCode
	}
	return true, resp.StatusCode
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
	// 自动发现模型（auto_ 前缀）同样探活：不可用的信号打 0 且标记确定性
	// 淘汰（401/403/404），恢复的移除标记重新进池。聚合 API /v1/models 输出
	// 时按信号 0 + autoDisabled 过滤，外部工具就看不到不可用模型了。
	probeAutoDiscovered(&wg)
	wg.Wait()
	fmt.Printf("🛰️ [免费池探活] 完成一轮：%d 个目录条目 + 自动发现模型（并发探测）\n", len(snapshot))
}

// probeAutoDiscovered 对自动发现快照里的每个模型做一次最小探活。
// 成功 → 移除不可用标记（拉起）；确定性错误(401/403/404) → 标记淘汰。
// 探活信号由 recordProbeResultDef 统一管理（key=endpoint|model）。
func probeAutoDiscovered(wg *sync.WaitGroup) {
	// 直接读 discCache（discoveredFreeModels 内部会再取锁并可能触发重拉，
	// 探活场景只需当前快照，避免重入锁/重复网络开销）
	discMu.Lock()
	snap := make([]freeModelView, 0)
	for _, p := range discCache {
		for _, m := range p.Models {
			if catalogHasModel(m.ID, p.Endpoint) {
				continue
			}
			if !isFreePoolDiscoveryEligible(p.Endpoint, m.ID) {
				continue
			}
			snap = append(snap, freeModelView{
				FreeModelDef: FreeModelDef{
					ID:       "auto_" + sanitizeID(p.Vendor) + "_" + hexEncode(m.ID),
					Vendor:   p.Vendor,
					Name:     m.Name,
					Endpoint: p.Endpoint,
					Model:    m.ID,
					KeyEnv:   p.KeyEnv,
					Keyless:  p.Keyless,
				},
				APIKeySet: true,
			})
		}
	}
	discMu.Unlock()

	for i := range snap {
		fm := snap[i]
		wg.Add(1)
		go func(fm freeModelView) {
			defer wg.Done()
			key := ""
			if !fm.Keyless {
				key = os.Getenv(fm.KeyEnv)
				if key == "" {
					if entries, err := loadModelConfigs(""); err == nil {
						for _, e := range entries {
							if e.ID == fm.ID {
								key = e.APIKey
								break
							}
						}
					}
				}
				if key == "" {
					// 没 key 的源无法探活：保持现状，不淘汰
					return
				}
			}
			b := RouterBackend{
				BaseURL: fm.Endpoint, Model: fm.Model,
				APIKey: key, Keyless: fm.Keyless,
			}
			start := time.Now()
			ok, status := probeChatOnce(b)
			lat := time.Since(start)
			recordProbeResultDef(fm.Endpoint, fm.Model, lat, status, ok)
			if ok {
				enableAutoModel(fm.Endpoint, fm.Model)
				return
			}
			// 确定性错误：永久淘汰（聚合池不再出现；探活恢复后自动拉起）
			if status == http.StatusUnauthorized || status == http.StatusForbidden || status == http.StatusNotFound {
				disableAutoModel(fm.Endpoint, fm.Model)
			}
		}(fm)
	}
}

// recordProbeResultDef 与 recordProbeResult 同逻辑，但接收 endpoint/model 而不是
// *FreeModelDef（自动发现模型不在 freeModelCatalog 里，没有 FreeModelDef）。
func recordProbeResultDef(endpoint, model string, lat time.Duration, status int, ok bool) {
	probeMu.Lock()
	defer probeMu.Unlock()
	k := endpoint + "|" + model
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
