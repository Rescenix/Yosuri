package handler

// stats_sync.go —— 主页统计的云端同步（热力图 + 模型数据）。
//
// 每条消息落库后异步上报一条聚合增量到 ResceneCloud（fire-and-forget，不阻塞对话）：
//   - uid 来自本地缓存 memory/intimacy.md（前端 fetchUid → fetchIntimacy 后自动写入），
//     uid 尚未就绪时静默跳过（下次消息再补）。
//   - 只传聚合计数 + token 估计值（estimateContentTokens，与本地统计同口径），
//     不传消息内容，隐私友好。
//   - 云端按 (uid, date, model, hour) 主键累加；同一 uid 跨设备自然累计。
//
// 展示侧（stats_handler.go）云端优先：uid 有且云端可达 → 用云端聚合重算窗口指标；
// 云端不可达 / 无 uid → 回退本地会话历史实时聚合（现状）。

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"backend/internal/memorydir"
)

// reportStatsAsync 异步上报一条消息的统计增量。调用方在消息落库后执行。
func reportStatsAsync(msg DSMessage) {
	uid, _ := memorydir.ReadIntimacy()
	if uid <= 0 {
		// 账号 uid 尚未同步到本地缓存（首次启动 fetchIntimacy 还没完成），跳过。
		return
	}
	ts := msg.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	payload := map[string]any{
		"uid":      uid,
		"date":     ts.Format("2006-01-02"),
		"model":    msg.Model,
		"hour":     ts.Hour(),
		"messages": 1,
		"tokens":   estimateContentTokens(msg.Content),
	}
	body, _ := json.Marshal(payload)
	go func() {
		req, err := http.NewRequest(http.MethodPost, cloudAuthBase()+"/api/stats/inc", bytes.NewReader(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return // 云端不可达：静默丢弃，统计近似即可
		}
		resp.Body.Close()
	}()
}

// cloudDailyStat / cloudModelStat / cloudHourStat 云端 GET /api/stats 返回的聚合行。
type cloudDailyStat struct {
	Date     string `json:"date"`
	Messages int64  `json:"messages"`
	Tokens   int64  `json:"tokens"`
}

type cloudModelStat struct {
	Model    string `json:"model"`
	Messages int64  `json:"messages"`
	Tokens   int64  `json:"tokens"`
}

type cloudHourStat struct {
	Hour     int   `json:"hour"`
	Messages int64 `json:"messages"`
}

// cloudStatsResp 云端 GET /api/stats 响应体。
type cloudStatsResp struct {
	Daily  []cloudDailyStat  `json:"daily"`
	Models []cloudModelStat  `json:"models"`
	Hours  []cloudHourStat   `json:"hours"`
}

// fetchCloudStats 从云端拉取某 uid 的统计聚合。失败返回 ok=false（调用方回退本地）。
func fetchCloudStats(uid int64) (resp cloudStatsResp, ok bool) {
	if uid <= 0 {
		return resp, false
	}
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest(http.MethodGet, cloudAuthBase()+"/api/stats?uid="+strconv.FormatInt(uid, 10), nil)
	if err != nil {
		return resp, false
	}
	res, err := client.Do(req)
	if err != nil || res.StatusCode != http.StatusOK {
		if res != nil {
			res.Body.Close()
		}
		return resp, false
	}
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return resp, false
	}
	return resp, true
}
