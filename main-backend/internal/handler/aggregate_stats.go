package handler

// aggregate_stats.go —— 聚合端口调用统计（2026-08-27）。
// 聚合端口（/v1/chat/completions、/v1/responses）是外部工具（Claude Code/Codex 等）
// 直调的高频入口，调用量远超应用内聊天——但之前完全没统计，排行榜只有应用内小头。
// 这里做内存原子计数 + 定时批量上报，不阻塞主链路：
//   - 每次成功调用 aggStatsInc(model) 只做 map 计数（mutex + int 累加，纳秒级）
//   - 后台 goroutine 每 30s 把累计值批量 POST 到 ResceneCloud /api/agg-stats/inc
//   - 上报共享密钥 RESCENE_AGG_STATS_KEY（re0 与云端配对），未配置则静默跳过
//   - token 量按请求输入估算（与应用内 estimateContentTokens 不同——聚合请求体是 JSON）

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type aggStat struct {
	calls  int64
	tokens int64
}

var (
	aggStatsMu   sync.Mutex
	aggStatsBuf  = map[string]map[string]*aggStat{} // 外层 model → 内层 uid → 计数
	aggStatsStop = make(chan struct{})
)

// aggStatsInc 记录一次聚合端口调用（成功分支调用）。内存计数，异步上报，零阻塞。
// tokens 用请求 JSON 长度估算（不读响应，避免多一次解析）。
// model 传 catalog ID（b.ID），与应用内 user_stats 同口径——云端才能按模型合并进排行榜。
// uid 从请求上下文取，游客传空字符串。
func aggStatsInc(b RouterBackend, reqTokens int64, uid string) {
	model := b.ID
	if model == "" {
		model = b.Model // 兜底：上游模型名
	}
	aggStatsMu.Lock()
	byUID, ok := aggStatsBuf[model]
	if !ok {
		byUID = map[string]*aggStat{}
		aggStatsBuf[model] = byUID
	}
	st := byUID[uid]
	if st == nil {
		st = &aggStat{}
		byUID[uid] = st
	}
	st.calls++
	st.tokens += reqTokens
	aggStatsMu.Unlock()
}

// estimateJSONTokens 粗略估算 JSON 请求体的 token 数（中文按字计，英文按词，取 max）。
func estimateJSONTokens(raw []byte) int64 {
	if len(raw) == 0 {
		return 0
	}
	// 字节数 / 4 是英文 token 近似；/ 2 是中文近似；取较大值再乘 0.8 保守
	en := int64(len(raw)) / 4
	cn := int64(len(raw)) / 2
	if cn > en {
		en = cn
	}
	if en > 0 {
		en = en * 8 / 10
	}
	return en
}

// StartAggStatsFlusher 启动定时批量上报 goroutine（每 30s 一次）。StartBackend 调用。
func StartAggStatsFlusher() {
	// 上一次上报时间戳记录在汇总里，重启用
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-aggStatsStop:
			flushAggStatsNow() // 退出前刷一次
			return
		case <-ticker.C:
			flushAggStatsNow()
		}
	}
}

// flushAggStatsNow 把累计计数批量上报云端（一把梭，失败静默丢弃——下个周期重新累计）。
func flushAggStatsNow() {
	aggStatsMu.Lock()
	if len(aggStatsBuf) == 0 {
		aggStatsMu.Unlock()
		return
	}
	type row struct {
		Model  string `json:"model"`
		UID    string `json:"uid"`
		Calls  int64  `json:"calls"`
		Tokens int64  `json:"tokens"`
		Date   string `json:"date"`
	}
	rows := make([]row, 0, len(aggStatsBuf))
	// 北京时间当天（与云端排行 range 过滤口径一致，自然日）
	date := time.Now().In(time.FixedZone("CST", 8*3600)).Format("2006-01-02")
	for m, byUID := range aggStatsBuf {
		for uid, st := range byUID {
			rows = append(rows, row{Model: m, UID: uid, Calls: st.calls, Tokens: st.tokens, Date: date})
		}
	}
	aggStatsBuf = map[string]map[string]*aggStat{}
	aggStatsMu.Unlock()

	body, _ := json.Marshal(map[string]any{"rows": rows})
	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequest(http.MethodPost, cloudAuthBase()+"/api/agg-stats/inc", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}