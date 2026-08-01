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
	"strconv"
	"strings"
	"time"

	"backend/internal/memorydir"

	"github.com/gin-gonic/gin"
)

// memorySyncEnabled 云端记忆同步总开关（RESCENE_MEMORY_SYNC=off 关闭，默认开）。
func memorySyncEnabled() bool {
	return strings.ToLower(os.Getenv("RESCENE_MEMORY_SYNC")) != "off"
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
func applyMemorySyncPayload(payload string) {
	var m map[string]string
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		return
	}
	for name, content := range m {
		file := strings.TrimSuffix(name, ".md")
		if file == name || file == "" {
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
	res, err := client.Do(req)
	if err != nil {
		return false
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return false // 404 = 账号还没有云端记忆，正常
	}
	var parsed struct {
		Payload string `json:"payload"`
	}
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		return false
	}
	applyMemorySyncPayload(parsed.Payload)
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
	res, err := client.Post(cloudAuthBase()+"/api/memory/sync", "application/json", bytes.NewReader(body))
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
