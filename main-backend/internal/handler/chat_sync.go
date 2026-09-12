package handler

// chat_sync.go —— 桌面端聊天记录云端同步（2026-09-12）。
//
// 职责：前端把本地会话 JSON 交到后端 → 后端用本机 AK 加密 → 上云；
//       拉取反向解密后交回前端合并。前端全程不接触 AK（密钥只在后端内存/磁盘）。
// 会话包格式：{sessionId: {title, messages:[...], updatedAt, persona}}（前端定义）。
// 冲突合并：前端按 updatedAt 新胜旧（后端不做合并，云端正/反向只是密文通道）。

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// syncChatFromCloud 登录后拉云端会话并本地恢复（404=无云端会话，静默）。返回是否成功。
func syncChatFromCloud() bool {
	_, uid, isLogin := syncIdentity()
	if !isLogin || uid <= 0 {
		return false
	}
	ak := loadAKFromDisk(uid)
	if len(ak) != 32 {
		return false
	}
	if chatSyncStore == nil {
		return false
	}
	tok, _, _ := syncIdentity()
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/chat/sync?uid=%d", cloudAuthBase(), uid), nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := cloudHTTPClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return false // 无云端会话：首次
	}
	if resp.StatusCode != 200 {
		return false
	}
	respBody, _ := readUpstreamBody(resp)
	var upstream struct {
		Payload string `json:"payload"`
	}
	if err := json.Unmarshal(respBody, &upstream); err != nil {
		return false
	}
	plain := DecryptMemoryPayload(upstream.Payload, ak)
	if plain == "" {
		return false
	}
	// 统一 schema 合并（防数据回退，2026-09-12）：云端只补「本地没有的会话」，
	// 同名保留本地（当前设备是活跃源）。云端格式 {sessions:[...]} 由 chat_schema 转换。
	localRaw, _ := os.ReadFile(sessionsFilePath(ChatSessionsDomain))
	merged, added := mergeCloudIntoLocal(localRaw, []byte(plain))
	if added > 0 {
		if err := os.WriteFile(sessionsFilePath(ChatSessionsDomain), merged, 0o644); err == nil && chatSyncStore != nil {
			_ = chatSyncStore.loadFromFile()
		}
	}
	log.Printf("💬 云端会话合并完成：补入 %d 个新会话（同名保留本地）", added)
	return true
}

// syncChatToCloud 登录后把本机会话推云（加密）。返回是否成功。
func syncChatToCloud() bool {
	_, uid, isLogin := syncIdentity()
	if !isLogin || uid <= 0 {
		return false
	}
	ak := loadAKFromDisk(uid)
	if len(ak) != 32 || chatSyncStore == nil {
		return false
	}
	payload, err := os.ReadFile(sessionsFilePath(ChatSessionsDomain))
	if err != nil || len(payload) == 0 || strings.TrimSpace(string(payload)) == "" {
		return false
	}
	// 本地 map 格式 → 统一云端 schema（跨端可读）
	cloudJSON, err := encodeLocalSessionsToCloud(payload)
	if err != nil || string(cloudJSON) == `{"sessions":[]}` {
		return false
	}
	enc, err := EncryptMemoryPayload(string(cloudJSON), ak)
	if err != nil {
		return false
	}
	tok, _, _ := syncIdentity()
	body, _ := json.Marshal(map[string]any{"uid": uid, "payload": enc})
	upReq, err := http.NewRequest("POST", cloudAuthBase()+"/api/chat/sync", strings.NewReader(string(body)))
	if err != nil {
		return false
	}
	upReq.Header.Set("Content-Type", "application/json")
	upReq.Header.Set("Authorization", "Bearer "+tok)
	resp, err := cloudHTTPClient.Do(upReq)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		log.Printf("💬 会话已加密上云（%d 字节明文）", len(payload))
		return true
	}
	return false
}

// SetChatSyncSessionStore 把全局 sessionStore 实例交给聊天云同步 handler。
// 同步直接读/写 sessions_<domain>.json 文件 + 重载，前端零改动。
func SetChatSyncSessionStore(store *SessionStore) { chatSyncStore = store }

// chatSyncStore 会话存储实例（router_factory 注入）。nil = 测试环境未注入，同步自动跳过。
var chatSyncStore *SessionStore

// HandleChatSyncPush 聊天记录全量推云：POST /api/chat/sync {}（无需 body——直接读本机会话文件）。
// 后端加密 → 上游。未登录/无 AK → 409。
func HandleChatSyncPush(c *gin.Context) {
	_, uid, isLogin := syncIdentity()
	if !isLogin || uid <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}
	ak := loadAKFromDisk(uid)
	if len(ak) != 32 {
		c.JSON(http.StatusConflict, gin.H{"error": "记忆密钥未解锁，请先开启记忆同步或重新登录"})
		return
	}
	if chatSyncStore == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "会话存储未就绪"})
		return
	}
	payload, err := os.ReadFile(sessionsFilePath(ChatSessionsDomain))
	if err != nil || len(payload) == 0 || strings.TrimSpace(string(payload)) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "暂无会话可同步"})
		return
	}
	cloudJSON, err := encodeLocalSessionsToCloud(payload)
	if err != nil || string(cloudJSON) == `{"sessions":[]}` {
		c.JSON(http.StatusBadRequest, gin.H{"error": "暂无会话可同步"})
		return
	}
	enc, err := EncryptMemoryPayload(string(cloudJSON), ak)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "加密失败"})
		return
	}
	tok, _, _ := syncIdentity()
	body, _ := json.Marshal(map[string]any{"uid": uid, "payload": enc})
	upReq, err := http.NewRequest("POST", cloudAuthBase()+"/api/chat/sync", strings.NewReader(string(body)))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "请求构造失败"})
		return
	}
	upReq.Header.Set("Content-Type", "application/json")
	upReq.Header.Set("Authorization", "Bearer "+tok)
	resp, err := cloudHTTPClient.Do(upReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": cloudErrorMessage(err)})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		c.JSON(resp.StatusCode, gin.H{"error": "云端保存失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// HandleChatSyncPull 从云端拉会话（解密后返回明文给前端合并）：GET /api/chat/sync。
// 无云端数据 → 404（前端首次处理）。
func HandleChatSyncPull(c *gin.Context) {
	_, uid, isLogin := syncIdentity()
	if !isLogin || uid <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "请先登录"})
		return
	}
	ak := loadAKFromDisk(uid)
	tok, _, _ := syncIdentity()
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/chat/sync?uid=%d", cloudAuthBase(), uid), nil)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "请求构造失败"})
		return
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := cloudHTTPClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": cloudErrorMessage(err)})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		c.JSON(http.StatusNotFound, gin.H{"error": "暂无云端会话"})
		return
	}
	if resp.StatusCode != 200 {
		c.JSON(resp.StatusCode, gin.H{"error": "云端拉取失败"})
		return
	}
	respBody, _ := readUpstreamBody(resp)
	var upstream struct {
		Payload string `json:"payload"`
	}
	if err := json.Unmarshal(respBody, &upstream); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "云端响应异常"})
		return
	}
	plain := DecryptMemoryPayload(upstream.Payload, ak)
	if plain == "" || strings.TrimSpace(plain) == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "云端会话为空"})
		return
	}
	// 统一 schema 合并（防数据回退）：云端只补本地没有的会话，同名保留本地。
	localRaw, _ := os.ReadFile(sessionsFilePath(ChatSessionsDomain))
	merged, added := mergeCloudIntoLocal(localRaw, []byte(plain))
	if added > 0 {
		if err := os.WriteFile(sessionsFilePath(ChatSessionsDomain), merged, 0o644); err == nil && chatSyncStore != nil {
			_ = chatSyncStore.loadFromFile()
		}
	}
	c.JSON(http.StatusOK, gin.H{"payload": plain, "added": added})
}