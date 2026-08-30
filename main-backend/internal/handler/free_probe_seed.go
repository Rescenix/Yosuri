package handler

import (
	"fmt"
	"time"
)

// free_probe_seed.go —— 官方探测基准 seed（2026-08-26 多轮平均定稿）。
//
// 背景：客户端本地后端做日级探活，但「首次启动/本地还没探过」时列表必须以官方
// 探测结果为基准，否则第一屏可能把暂时不可用的模型摆出来被小白用户问「怎么用不了」。
// 这份 seed = 官方（开发者）2026-08-26 实测三个免 key 网关全部目录条目**多轮平均**结果，
// 客户端启动时灌进 probeStates 作为「首次基准」；之后本地日级探活会逐日覆盖它。
//
// ⚠️ 只覆盖三个免 key 提供方（Kilo/LLM7/Zen，Keyless:true）——只有它们的探测是
// 零成本的（不烧用户填的 key 额度）。需 key 厂商不探、不进 seed，靠真实请求驱动。
//
// 判定口径（08-26 改动：区分「真死」拉黑与「暂时/限流」降权，不再一刀切判死）：
//   - 确定性错误 HTTP 401/400/403/404 → 真死（模型下架/无权限/名字变了，全局一致的）
//     → signal 0，列表隐藏。
//   - HTTP 429 / 503 / 传输超时 → 暂时不可用或本机 IP 被限流：**不判死**
//     （429 是 IP 级限流，官方这台被限不代表用户那台被限；503 是上游抖动随时恢复）
//     → signal 1，保留显示但排后，auto 链会故障转移跳过。
//   - HTTP 200 且 usage>0 → 可用 → signal 2/3/4（按延迟分档）。
//   - HTTP 200 但 usage=0（空回复）→ 质量可疑 → signal 1（降权）。
//
// ⚠️ 08-26 多轮实测教训：单次探测会把 429/503 这类「暂时性」误判成死源，从而
// 错误隐藏「用户那边明明可用」的模型。LLM7 免费档按 IP 限流（官方这台 429 一片，
// 用户那台未必）；Zen 的 503 也是抖动。所以 seed 只对「确定性错误」判死，
// 429/503 一律降权不隐藏。
//
// 更新方式：改动后跑 scripts/probe_keyless.py（或按目录条目 curl 最小请求多轮），
// 把实测 rep/ok_rate/lat 按上面的口径折算成 Status/OK/LatMS 填回，遵循「下次启动生效」。

type seedProbe struct {
	ID     string // 目录 ID（仅注释用，便于定位）
	Status int    // 代表性 HTTP 状态码；0 = 传输错误/超时
	LatMS  int64  // 平均延迟 ms（取成功轮次均值；全失败取 0）
	OK     bool   // 是否探活成功（200 且 usage>0）
}

// freeProbeSeed 官方探测基准（2026-08-26 curl 串行实测定稿，23 条免 key 目录条目全量）。
// ⚠️ 必须串行探测！并发（8线程）会触发免费网关按 IP 限流 429，把活模型误判成死源
// （实测 LLM7 全系并发 429、单次 curl 全 200；Kilo auto 并发 503、单次 200）。
var freeProbeSeed = []seedProbe{
	// —— Kilo Gateway（2026-08-15 接入，免 key）——
	{ID: "kilo_auto_free", Status: 200, LatMS: 2237, OK: true},
	{ID: "kilo_step_3_7_flash_free", Status: 200, LatMS: 2154, OK: true},
	{ID: "kilo_tencent_hy3_free", Status: 200, LatMS: 2629, OK: true},
	{ID: "kilo_laguna_s_2_1_free", Status: 200, LatMS: 1142, OK: true},
	{ID: "kilo_laguna_xs_2_1_free", Status: 200, LatMS: 902, OK: true},
	{ID: "kilo_north_mini_code_free", Status: 200, LatMS: 1628, OK: true},
	{ID: "kilo_nemotron_lightning_free", Status: 200, LatMS: 859, OK: true},
	{ID: "kilo_nemotron_nano_free", Status: 200, LatMS: 1202, OK: true},
	{ID: "kilo_nemotron_super_free", Status: 200, LatMS: 907, OK: true},
	{ID: "kilo_nemotron_ultra_free", Status: 200, LatMS: 1074, OK: true},
	{ID: "kilo_liquid_lfm_free", Status: 503, LatMS: 0, OK: false}, // 真不可用：上游模型 unavailable for real-time inference
	{ID: "kilo_nemotron_safety_free", Status: 200, LatMS: 933, OK: true},
	{ID: "kilo_openrouter_free", Status: 200, LatMS: 1257, OK: true},
	// —— LLM7.io（2026-08-22 接入，免 key）——
	// ⚠️ 2026-08-29 实锤：free_llm7_deepseek_v4_flash 上游模型 DeepSeek-V4-Flash-0731
	// 已下架（真实请求 400 Model 'DeepSeek-V4-Flash-0731' is currently unavailable）。
	// 之前 seed 标 200 健康 → 每次重启复活 → auto 链首跳白撞 → 整链拖垮。
	{ID: "free_llm7_deepseek_v4_flash", Status: 400, LatMS: 0, OK: false}, // 真死：model currently unavailable
	{ID: "free_llm7_codestral", Status: 200, LatMS: 1860, OK: true},
	{ID: "free_llm7_gemini_flash_lite", Status: 200, LatMS: 2036, OK: true},
	{ID: "free_llm7_gpt_oss_20b", Status: 400, LatMS: 0, OK: false}, // 真死：model currently unavailable
	{ID: "free_llm7_llama_3_1_8b", Status: 200, LatMS: 1114, OK: true},
	{ID: "free_llm7_minimax_m2_7", Status: 200, LatMS: 2760, OK: true},
	// —— OpenCode Zen（免 key）——
	{ID: "free_zen_mimo_v2_5", Status: 200, LatMS: 4962, OK: true},
	{ID: "free_zen_north_mini_code", Status: 401, LatMS: 0, OK: false}, // 真死：Model not supported
	{ID: "free_zen_longcat_2_0", Status: 401, LatMS: 0, OK: false},     // 真死：Model not supported
	{ID: "free_zen_laguna_s_2_1", Status: 200, LatMS: 1951, OK: true},
}

// applyFreeProbeSeed 把官方探测基准灌进 probeStates（幂等：只在本地尚无探测结果时生效）。
// 首次启动 probeStates 为空 → 灌 seed；本地日级探活（probeOnce）开始后逐个覆盖 seed 值。
// 注意 probeStates 是进程内内存态，每次重启为空，故每次启动都以 seed 作为「首次基准」。
//
// 分档判定（与 seedProbe 注释口径一致）：确定性错误 → 隐藏；429/503/超时 → 降权保留；
// 200 且 OK → 按延迟分档；200 空回复 → 降权。这样「本机IP被限流」不会误杀用户那边可用的模型。
func applyFreeProbeSeed() {
	probeMu.Lock()
	alreadyProbed := len(probeStates) > 0
	probeMu.Unlock()
	if alreadyProbed {
		return
	}
	loaded := 0
	for _, sp := range freeProbeSeed {
		f := catalogFreeByID(sp.ID)
		if f == nil {
			continue
		}
		k := f.Endpoint + "|" + f.Model
		lat := time.Duration(sp.LatMS) * time.Millisecond
		st := &probeState{lastProbe: time.Now(), latency: lat}
		switch {
		case sp.Status == 401 || sp.Status == 403 || sp.Status == 404 || sp.Status == 400:
			// 确定性错误 → 真死，隐藏（列表不再出现）
			st.signal, st.lastOK = 0, false
		case sp.Status == 200 && sp.OK:
			// 可用 → 按延迟分档
			st.lastOK = true
			switch {
			case lat <= probeLatencyFast:
				st.signal = 4
			case lat <= probeLatencyMid:
				st.signal = 3
			default:
				st.signal = 2
			}
		default:
			// 429 / 503 / 超时 / 200 空回复 → 暂时或本机限流：降权保留，不判死
			st.signal, st.lastOK = 1, false
		}
		probeMu.Lock()
		probeStates[k] = st
		probeMu.Unlock()
		loaded++
	}
	fmt.Printf("🌱 [官方探测基准] 已灌入 %d 个免 key 模型（死/受限/可用三档，日级探活将覆盖）\n", loaded)
}

// catalogFreeByID 在 freeModelCatalog 里按目录 ID 找条目。
func catalogFreeByID(id string) *FreeModelDef {
	freeCatalogMu.Lock()
	defer freeCatalogMu.Unlock()
	for i := range freeModelCatalog {
		if freeModelCatalog[i].ID == id {
			return &freeModelCatalog[i]
		}
	}
	return nil
}
