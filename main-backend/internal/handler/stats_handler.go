// internal/handler/stats_handler.go
package handler

import (
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// ModelTokenStat 单个模型在某统计窗口内的Token消耗
type ModelTokenStat struct {
	Model  string `json:"model"`
	Tokens int    `json:"tokens"`
}

// StatsWindow 某一时间窗口（总共/30天/7天）内的聚合统计
type StatsWindow struct {
	TotalSessions int              `json:"total_sessions"`
	TotalMessages int              `json:"total_messages"`
	TotalTokens   int              `json:"total_tokens"`
	ActiveDays    int              `json:"active_days"`
	CurrentStreak int              `json:"current_streak"`
	LongestStreak int              `json:"longest_streak"`
	PeakHour      string           `json:"peak_hour"`
	FavoriteModel string           `json:"favorite_model"`
	ModelTokens   []ModelTokenStat `json:"model_tokens"`
}

// StatsOverview GET /api/stats/overview 响应
type StatsOverview struct {
	Total   StatsWindow `json:"total"`
	Last30d StatsWindow `json:"last_30d"`
	Last7d  StatsWindow `json:"last_7d"`
}

// DailyStat 单日代码活动统计
type DailyStat struct {
	Date   string `json:"date"`
	Count  int    `json:"count"`
	Tokens int    `json:"tokens"`
}

// DayDetailMessage 某一天内的单条消息摘要
type DayDetailMessage struct {
	SessionID string `json:"session_id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Model     string `json:"model,omitempty"`
	Tokens    int    `json:"tokens"`
	Timestamp int64  `json:"timestamp"`
}

// statMessage 扁平化后的单条消息，附带所属会话ID，供统计聚合使用
type statMessage struct {
	SessionID string
	Role      string
	Timestamp time.Time
	Model     string
	Content   string
}

// StatsHandler 统计API处理器，数据完全来自 SessionStore，不依赖 PrismD
type StatsHandler struct {
	sessionStore *SessionStore
}

// NewStatsHandler 创建统计处理器
func NewStatsHandler(store *SessionStore) *StatsHandler {
	return &StatsHandler{sessionStore: store}
}

func estimateContentTokens(content string) int {
	return len(content) / 4
}

func (h *StatsHandler) flattenMessages() []statMessage {
	sessions := h.sessionStore.AllSessions()
	out := make([]statMessage, 0)
	for sid, msgs := range sessions {
		for _, m := range msgs {
			out = append(out, statMessage{
				SessionID: sid,
				Role:      m.Role,
				Timestamp: m.Timestamp,
				Model:     m.Model,
				Content:   m.Content,
			})
		}
	}
	return out
}

// chineseHourLabel 将 0-23 的小时转为中文时段描述，如 "凌晨 1 点"、"下午 3 点"
func chineseHourLabel(hour int) string {
	switch {
	case hour == 0:
		return "凌晨 12 点"
	case hour >= 1 && hour <= 5:
		return "凌晨 " + strconv.Itoa(hour) + " 点"
	case hour >= 6 && hour <= 8:
		return "早上 " + strconv.Itoa(hour) + " 点"
	case hour >= 9 && hour <= 11:
		return "上午 " + strconv.Itoa(hour) + " 点"
	case hour == 12:
		return "中午 12 点"
	case hour >= 13 && hour <= 17:
		return "下午 " + strconv.Itoa(hour-12) + " 点"
	default: // 18-23
		return "晚上 " + strconv.Itoa(hour-12) + " 点"
	}
}

// computeCurrentStreak 从 now 所在日期往回数连续活跃天数（dayActive 已按窗口筛选过）
func computeCurrentStreak(dayActive map[string]bool, now time.Time) int {
	streak := 0
	day := now
	for i := 0; i < 3650; i++ {
		key := day.Format("2006-01-02")
		if !dayActive[key] {
			break
		}
		streak++
		day = day.AddDate(0, 0, -1)
	}
	return streak
}

// computeLongestStreak 计算已排序日期列表中最长的连续天数
func computeLongestStreak(sortedDays []string) int {
	if len(sortedDays) == 0 {
		return 0
	}
	longest, current := 1, 1
	for i := 1; i < len(sortedDays); i++ {
		prev, errPrev := time.Parse("2006-01-02", sortedDays[i-1])
		cur, errCur := time.Parse("2006-01-02", sortedDays[i])
		if errPrev != nil || errCur != nil {
			continue
		}
		if cur.Sub(prev) == 24*time.Hour {
			current++
			if current > longest {
				longest = current
			}
		} else {
			current = 1
		}
	}
	return longest
}

// computeWindow 聚合 [since, now] 时间窗口内的统计指标；since 为零值表示不设下限（全部时间）
func computeWindow(msgs []statMessage, since time.Time, now time.Time) StatsWindow {
	hasSince := !since.IsZero()

	sessionSet := make(map[string]bool)
	dayActive := make(map[string]bool)
	hourHist := make(map[int]int)
	modelMsgCount := make(map[string]int)
	modelTokens := make(map[string]int)
	totalMessages := 0
	totalTokens := 0

	for _, m := range msgs {
		if hasSince && m.Timestamp.Before(since) {
			continue
		}
		if m.Timestamp.After(now) {
			continue
		}
		totalMessages++
		tokens := estimateContentTokens(m.Content)
		totalTokens += tokens
		sessionSet[m.SessionID] = true
		dayActive[m.Timestamp.Format("2006-01-02")] = true
		hourHist[m.Timestamp.Hour()]++
		if m.Model != "" {
			modelMsgCount[m.Model]++
			modelTokens[m.Model] += tokens
		}
	}

	activeDaysList := make([]string, 0, len(dayActive))
	for d := range dayActive {
		activeDaysList = append(activeDaysList, d)
	}
	sort.Strings(activeDaysList)

	peakHour := "-"
	if totalMessages > 0 {
		bestHour, bestCount := -1, 0
		for hr, cnt := range hourHist {
			if cnt > bestCount || (cnt == bestCount && (bestHour == -1 || hr < bestHour)) {
				bestHour, bestCount = hr, cnt
			}
		}
		if bestHour >= 0 {
			peakHour = chineseHourLabel(bestHour)
		}
	}

	favoriteModel := "-"
	{
		bestModel, bestCount := "", 0
		for model, cnt := range modelMsgCount {
			if cnt > bestCount {
				bestModel, bestCount = model, cnt
			}
		}
		if bestModel != "" {
			favoriteModel = bestModel
		}
	}

	modelTokensList := make([]ModelTokenStat, 0, len(modelTokens))
	for model, tok := range modelTokens {
		modelTokensList = append(modelTokensList, ModelTokenStat{Model: model, Tokens: tok})
	}
	sort.Slice(modelTokensList, func(i, j int) bool {
		return modelTokensList[i].Tokens > modelTokensList[j].Tokens
	})
	if len(modelTokensList) > 5 {
		modelTokensList = modelTokensList[:5]
	}

	return StatsWindow{
		TotalSessions: len(sessionSet),
		TotalMessages: totalMessages,
		TotalTokens:   totalTokens,
		ActiveDays:    len(dayActive),
		CurrentStreak: computeCurrentStreak(dayActive, now),
		LongestStreak: computeLongestStreak(activeDaysList),
		PeakHour:      peakHour,
		FavoriteModel: favoriteModel,
		ModelTokens:   modelTokensList,
	}
}

// HandleOverview GET /api/stats/overview — 总览卡片所需的全部数据，按 总共/30天/7天 分层
func (h *StatsHandler) HandleOverview(c *gin.Context) {
	now := time.Now()
	msgs := h.flattenMessages()

	overview := StatsOverview{
		Total:   computeWindow(msgs, time.Time{}, now),
		Last30d: computeWindow(msgs, now.AddDate(0, 0, -30), now),
		Last7d:  computeWindow(msgs, now.AddDate(0, 0, -7), now),
	}
	c.JSON(http.StatusOK, overview)
}

// HandleDailyStats GET /api/stats/daily?days=30 — 按天聚合的消息数与Token消耗，供热力图使用
func (h *StatsHandler) HandleDailyStats(c *gin.Context) {
	days := 30
	if q := c.Query("days"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 {
			days = n
		}
	}

	now := time.Now()
	msgs := h.flattenMessages()

	dayMap := make(map[string]*DailyStat)
	for _, m := range msgs {
		key := m.Timestamp.Format("2006-01-02")
		if _, ok := dayMap[key]; !ok {
			dayMap[key] = &DailyStat{Date: key}
		}
		dayMap[key].Count++
		dayMap[key].Tokens += estimateContentTokens(m.Content)
	}

	result := make([]DailyStat, 0, days)
	for i := days - 1; i >= 0; i-- {
		key := now.AddDate(0, 0, -i).Format("2006-01-02")
		if s, ok := dayMap[key]; ok {
			result = append(result, *s)
		} else {
			result = append(result, DailyStat{Date: key, Count: 0, Tokens: 0})
		}
	}

	c.JSON(http.StatusOK, result)
}

// HandleDayDetail GET /api/stats/detail?date=2026-07-01 — 返回某一天的详细对话记录摘要
func (h *StatsHandler) HandleDayDetail(c *gin.Context) {
	dateStr := c.Query("date")
	if dateStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少date参数"})
		return
	}
	target, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "日期格式错误，需要YYYY-MM-DD"})
		return
	}

	msgs := h.flattenMessages()
	details := make([]DayDetailMessage, 0)
	for _, m := range msgs {
		if m.Timestamp.Year() != target.Year() || m.Timestamp.Month() != target.Month() || m.Timestamp.Day() != target.Day() {
			continue
		}
		details = append(details, DayDetailMessage{
			SessionID: m.SessionID,
			Role:      m.Role,
			Content:   m.Content,
			Model:     m.Model,
			Tokens:    estimateContentTokens(m.Content),
			Timestamp: m.Timestamp.Unix(),
		})
	}
	sort.Slice(details, func(i, j int) bool {
		return details[i].Timestamp < details[j].Timestamp
	})

	c.JSON(http.StatusOK, details)
}
