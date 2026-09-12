package handler

// chat_schema.go —— 聊天记录云端同步的统一 schema（2026-09-12）。
//
// 三端（桌面/手机/官网）会话结构各异，云端只认一种统一格式：
//
//	{"sessions":[{"id","title","updatedAt"(ms),"messages":[{"role","content","ts"(ms)}]}]}
//
// 桌面本地是 map[sid]sessionRecord（含富消息/工具调用/时间戳），推云前转成
// 统一格式（只保留 role+content+时间，跨端可读）；拉云后合并回本地 map。

import (
	"encoding/json"
	"sort"
	"time"
)

// cloudChatMessage 统一格式的单条消息。
type cloudChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Ts      int64  `json:"ts"`
}

// cloudChatSession 统一格式的单个会话。
type cloudChatSession struct {
	ID        string             `json:"id"`
	Title     string             `json:"title"`
	UpdatedAt int64              `json:"updatedAt"`
	Messages  []cloudChatMessage `json:"messages"`
}

// cloudChatPayload 统一格式的云端会话包。
type cloudChatPayload struct {
	Sessions []cloudChatSession `json:"sessions"`
}

// chatTitleFrom 会话标题：显式标题优先，否则首条用户消息前 18 字。
func chatTitleFrom(rec sessionRecord) string {
	if rec.SessionTitle != "" {
		return rec.SessionTitle
	}
	for _, m := range rec.Messages {
		if m.Role == "user" && m.Content != "" {
			r := []rune(m.Content)
			if len(r) > 18 {
				return string(r[:18]) + "…"
			}
			return m.Content
		}
	}
	return "新会话"
}

// encodeLocalSessionsToCloud 本地会话文件 JSON → 统一云端格式 JSON。
// 无消息/无文本内容的会话跳过（空会话不占云空间）。
func encodeLocalSessionsToCloud(localRaw []byte) ([]byte, error) {
	var local map[string]sessionRecord
	if err := json.Unmarshal(localRaw, &local); err != nil {
		return nil, err
	}
	out := cloudChatPayload{Sessions: make([]cloudChatSession, 0, len(local))}
	for sid, rec := range local {
		msgs := make([]cloudChatMessage, 0, len(rec.Messages))
		var lastTs int64
		for _, m := range rec.Messages {
			if m.Content == "" {
				continue // 工具调用等无文本消息跨端无意义，跳过
			}
			ts := m.Timestamp.UnixMilli()
			if ts > lastTs {
				lastTs = ts
			}
			msgs = append(msgs, cloudChatMessage{Role: m.Role, Content: m.Content, Ts: ts})
		}
		if len(msgs) == 0 {
			continue
		}
		if lastTs == 0 {
			lastTs = time.Now().UnixMilli()
		}
		out.Sessions = append(out.Sessions, cloudChatSession{
			ID: sid, Title: chatTitleFrom(rec), UpdatedAt: lastTs, Messages: msgs,
		})
	}
	sort.Slice(out.Sessions, func(i, j int) bool { return out.Sessions[i].UpdatedAt > out.Sessions[j].UpdatedAt })
	return json.Marshal(out)
}

// mergeCloudIntoLocal 统一云端格式合并进本机会话文件 JSON：
// 只补「本地没有的会话」（同名保留本地=当前设备是活跃源，防数据回退）。
// 返回 (合并后的本地 JSON, 新增会话数)。
func mergeCloudIntoLocal(localRaw, cloudRaw []byte) ([]byte, int) {
	var local map[string]sessionRecord
	if err := json.Unmarshal(localRaw, &local); err != nil || local == nil {
		local = map[string]sessionRecord{}
	}
	var cloud cloudChatPayload
	if err := json.Unmarshal(cloudRaw, &cloud); err != nil {
		return localRaw, 0
	}
	added := 0
	for _, cs := range cloud.Sessions {
		if cs.ID == "" || len(cs.Messages) == 0 {
			continue
		}
		if _, ok := local[cs.ID]; ok {
			continue // 同名保留本地
		}
		rec := sessionRecord{SessionTitle: cs.Title}
		for _, m := range cs.Messages {
			if m.Role == "" || m.Content == "" {
				continue
			}
			ts := m.Ts
			if ts == 0 {
				ts = time.Now().UnixMilli()
			}
			rec.Messages = append(rec.Messages, persistedMessage{
				Role: m.Role, Content: m.Content, Timestamp: time.UnixMilli(ts),
			})
		}
		if len(rec.Messages) == 0 {
			continue
		}
		local[cs.ID] = rec
		added++
	}
	if added == 0 {
		return localRaw, 0
	}
	merged, err := json.Marshal(local)
	if err != nil {
		return localRaw, 0
	}
	return merged, added
}
