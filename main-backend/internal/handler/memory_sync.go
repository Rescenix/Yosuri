package handler

// memory_sync.go —— 云端记忆同步（可选）。
//
// 把本地记忆目录打包成 JSON 记忆包同步到 ResceneCloud（按 UID 账号存储），
// 换设备登录后拉回 —— 记忆跟账号走，与亲密等级/统计同一套"珍惜账号"语义。
//
//   - 可选开关：环境变量 RESCENE_MEMORY_SYNC=off 关闭（默认开启）。关闭后不推不拉。
//   - 推送：记忆写工具（remember / memory_pin / memory_append / memory_handoff）
//     写完后异步全量推送（fire-and-forget，不阻塞）。
//   - 拉取：re0 启动时（有 uid 缓存）自动拉一次；前端也可在登录/启动后显式调
//     POST /api/memory/sync/pull 触发。
//   - 合并策略：拉取时云端文件写回本地（同名覆盖），本地独有文件保留 —— 换设备
//     后本地记忆为空或稀疏，云端历史能直接恢复；推送是本地全量覆盖云端（后写赢）。
//   - 隐私：只同步记忆 md 文本（偏好/决策/索引），不同步会话内容与 handoff 工作态。

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"backend/internal/memorydir"

	"github.com/gin-gonic/gin"
)

// memorySyncEnabled 云端记忆同步开关：
//   - 环境变量 RESCENE_MEMORY_SYNC=off → 强制关闭（部署级，前端开关同时禁用）
//   - 本地设置文件 ~/rescene_data/cloud_sync_enabled.md 内容为 off → 关闭（前端"记忆"tab 可切换）
//   - 默认开启
func memorySyncEnabled() bool {
	if strings.ToLower(os.Getenv("RESCENE_MEMORY_SYNC")) == "off" {
		return false
	}
	if p := memorySyncSettingPath(); p != "" {
		if data, err := os.ReadFile(p); err == nil && strings.TrimSpace(strings.ToLower(string(data))) == "off" {
			return false
		}
	}
	return true
}

// memorySyncSettingPath 前端记忆 tab 开关的本地落盘位置。
func memorySyncSettingPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "rescene_data", "cloud_sync_enabled.md")
}

// guestTokenPath 游客/登录 JWT 的本地落盘位置（前端 fetchUid 拿到 token 后
// POST /api/auth/guest-token 缓存到这里；后端 push/pull 云端记忆时带上鉴权头）。
func guestTokenPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "rescene_data", "cloud_guest_token")
}

// readGuestToken 读本地缓存的 JWT（无则空串——旧版/未缓存时推送会 401 静默失败）。
func readGuestToken() string {
	if p := guestTokenPath(); p != "" {
		if data, err := os.ReadFile(p); err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return ""
}

// HandleGuestTokenStore POST /api/auth/guest-token {token}
// 前端把云端签发的 JWT 交给本地后端缓存，供 memory_sync 推送/拉取鉴权。
func HandleGuestTokenStore(c *gin.Context) {
	var req struct {
		Token string `json:"token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 token"})
		return
	}
	p := guestTokenPath()
	if p == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "用户目录不可用"})
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "写入失败"})
		return
	}
	if err := os.WriteFile(p, []byte(req.Token+"\n"), 0o644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "写入失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// HandleMemorySyncSettings GET /api/memory/sync/settings → {enabled, env_override}
// env_override=true 表示部署级 RESCENE_MEMORY_SYNC=off 强制关闭，前端开关应禁用。
func HandleMemorySyncSettings(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"enabled":      memorySyncEnabled(),
		"env_override": strings.ToLower(os.Getenv("RESCENE_MEMORY_SYNC")) == "off",
	})
}

// HandleMemorySyncSettingsUpdate POST /api/memory/sync/settings {enabled}
// 前端记忆 tab 开关：写本地设置文件，即时生效（push 每次调用都检查）。
func HandleMemorySyncSettingsUpdate(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	p := memorySyncSettingPath()
	if p == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "用户目录不可用"})
		return
	}
	os.MkdirAll(filepath.Dir(p), 0o755)
	val := "on"
	if !req.Enabled {
		val = "off"
	}
	if err := os.WriteFile(p, []byte(val+"\n"), 0o644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "写入失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"enabled": req.Enabled})
}

// memorySyncPayload 打包白名单记忆文件为 JSON map（文件名含 .md → 内容）。
func memorySyncPayload() string {
	files := make([]string, 0, len(memorydir.SyncableFiles))
	for f := range memorydir.SyncableFiles {
		files = append(files, f)
	}
	m := map[string]string{}
	for _, f := range files {
		if c := memorydir.ReadRaw(f); c != "" {
			m[f+".md"] = c
		}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return string(b)
}

// applyMemorySyncPayload 把云端记忆包写回本地。文件名去 .md 后必须在白名单内
// （WriteRaw 内二次校验，防路径穿越）；本地独有文件不受影响（合并语义）。
// cloudUpdatedAt 是云端包最近更新时间：早于本地文件 mtime 说明本地更新（鉴权断链
// 期云端会停更，直接拉回会把新记忆覆盖成旧版），跳过不覆盖。
func applyMemorySyncPayload(payload string, cloudUpdatedAt time.Time) {
	var m map[string]string
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		return
	}
	for name, content := range m {
		file := strings.TrimSuffix(name, ".md")
		if file == name || file == "" {
			continue
		}
		// 防旧覆盖新：本地文件比云端包还新 → 保留本地（等下次 push 推上去）
		if !cloudUpdatedAt.IsZero() && memorydir.FileModTime(file).After(cloudUpdatedAt) {
			continue
		}
		if err := memorydir.WriteRaw(file, strings.TrimSpace(content)); err != nil {
			continue // 白名单外文件跳过
		}
	}
}

// pushMemorySync 异步把本地记忆全量推送到云端（fire-and-forget）。
func pushMemorySync() {
	if !memorySyncEnabled() {
		return
	}
	uid, _ := memorydir.ReadIntimacy()
	if uid <= 0 {
		return // 账号 uid 尚未同步到本地缓存
	}
	payload := memorySyncPayload()
	if payload == "" {
		return
	}
	body, _ := json.Marshal(map[string]any{"uid": uid, "payload": payload})
	go func() {
		req, err := http.NewRequest(http.MethodPost, cloudAuthBase()+"/api/memory/sync", bytes.NewReader(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		// 鉴权头：云端要求 JWT uid == 请求 uid（08-17 起），不带会 401 静默失败
		if tok := readGuestToken(); tok != "" {
			req.Header.Set("Authorization", "Bearer "+tok)
		}
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return
		}
		resp.Body.Close()
	}()
}

// pullMemorySync 从云端拉取记忆包并写回本地。返回是否成功拉取并应用。
func pullMemorySync(uid int64) bool {
	if !memorySyncEnabled() || uid <= 0 {
		return false
	}
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest(http.MethodGet, cloudAuthBase()+"/api/memory/sync?uid="+strconv.FormatInt(uid, 10), nil)
	if err != nil {
		return false
	}
	// 鉴权头：云端要求 JWT uid == 请求 uid，不带会 401 静默失败
	if tok := readGuestToken(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	res, err := client.Do(req)
	if err != nil {
		return false
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return false // 404 = 账号还没有云端记忆，正常
	}
	var parsed struct {
		Payload   string `json:"payload"`
		UpdatedAt string `json:"updated_at"`
	}
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		return false
	}
	// 云端 updated_at 是 UTC（SQLite datetime('now')）；与本地 mtime 的 After 比较
	// 按绝对时刻，时区无关，无需手动换算。
	var cloudAt time.Time
	if parsed.UpdatedAt != "" {
		if t, err := time.Parse("2006-01-02 15:04:05", parsed.UpdatedAt); err == nil {
			cloudAt = t
		}
	}
	applyMemorySyncPayload(parsed.Payload, cloudAt)
	return true
}

// HandleMemorySyncPull POST /api/memory/sync/pull {uid}
// 显式触发：从云端拉记忆包写回本地（前端登录/启动后调用，跨设备恢复记忆）。
func HandleMemorySyncPull(c *gin.Context) {
	var req struct {
		UID int64 `json:"uid"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.UID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 uid"})
		return
	}
	ok := pullMemorySync(req.UID)
	c.JSON(http.StatusOK, gin.H{"ok": ok, "restored": ok})
}

// HandleMemorySyncPush POST /api/memory/sync/push {uid}
// 显式触发：把本地记忆全量推送到云端（前端手动同步按钮）。
func HandleMemorySyncPush(c *gin.Context) {
	var req struct {
		UID int64 `json:"uid"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.UID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 uid"})
		return
	}
	payload := memorySyncPayload()
	if payload == "" {
		c.JSON(http.StatusOK, gin.H{"ok": false, "reason": "本地暂无记忆"})
		return
	}
	body, _ := json.Marshal(map[string]any{"uid": req.UID, "payload": payload})
	client := &http.Client{Timeout: 5 * time.Second}
	r, err := http.NewRequest(http.MethodPost, cloudAuthBase()+"/api/memory/sync", bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "构造请求失败"})
		return
	}
	r.Header.Set("Content-Type", "application/json")
	// 鉴权头：云端要求 JWT uid == 请求 uid，不带会 401
	if tok := readGuestToken(); tok != "" {
		r.Header.Set("Authorization", "Bearer "+tok)
	}
	res, err := client.Do(r)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "连接 ResceneCloud 失败: " + err.Error()})
		return
	}
	defer res.Body.Close()
	c.Status(res.StatusCode)
}

// StartupMemorySyncPull re0 启动时自动拉取一次云端记忆（uid 已缓存时）。
// 在 StartBackend 末尾以 goroutine 调用；延迟 2s 等 uid 缓存/网络就绪。
func StartupMemorySyncPull() {
	go func() {
		time.Sleep(2 * time.Second)
		uid, _ := memorydir.ReadIntimacy()
		if uid > 0 {
			pullMemorySync(uid)
		}
	}()
}
