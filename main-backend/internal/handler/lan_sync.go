package handler

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"backend/internal/memorydir"
)

// LanSyncService 局域网同步服务（零云端，数据不出局域网）。
// 独立 HTTP 服务监听 0.0.0.0:port，token 鉴权 + AES-GCM payload 加密：
// 记忆/会话内容全部用 token 派生的 AES 密钥加密传输，内网即使被嗅探也读不到明文。
// 只暴露 /lan/ 端点，不碰 re0 主服务的其他 API。
type LanSyncService struct {
	store   *SessionStore
	token   string
	port    string
	server  *http.Server
	mu      sync.Mutex
	running bool
}

// NewLanSyncService 创建并启动局域网同步服务。
func NewLanSyncService(store *SessionStore, port string) *LanSyncService {
	b := make([]byte, 16)
	rand.Read(b)
	token := hex.EncodeToString(b)
	if port == "" {
		port = "18080"
	}
	return &LanSyncService{store: store, token: token, port: port}
}

// Start 启动 HTTP 服务（goroutine，非阻塞）。幂等：已运行时直接返回。
// 默认不自动调用（避免每次启动触发 Windows 防火墙弹窗），由前端 /api/lan/enable 按需开启。
func (s *LanSyncService) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/lan/info", s.handleInfo)
	mux.HandleFunc("/lan/memory/pull", s.handleMemoryPull)
	mux.HandleFunc("/lan/memory/push", s.handleMemoryPush)
	mux.HandleFunc("/lan/sessions/pull", s.handleSessionsPull)
	mux.HandleFunc("/lan/sessions/push", s.handleSessionsPush)

	addr := "0.0.0.0:" + s.port
	s.server = &http.Server{Addr: addr, Handler: mux}
	s.running = true
	log.Printf("📡 局域网同步服务 %s（token=%s，AES-GCM 加密传输）", addr, s.token)
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("⚠️ 局域网同步服务异常: %v", err)
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
		}
	}()
}

// Stop 停止局域网同步服务（幂等）。
func (s *LanSyncService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if s.server != nil {
		_ = s.server.Shutdown(ctx)
	}
	s.running = false
	log.Printf("📴 局域网同步服务已停止")
}

// Running 返回服务是否正在监听。
func (s *LanSyncService) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// Token 返回当前 token（供前端显示）。
func (s *LanSyncService) Token() string { return s.token }

// Port 返回监听端口。
func (s *LanSyncService) Port() string { return s.port }

// 本机局域网 IPv4
func getLanIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}
	return "127.0.0.1"
}

func (s *LanSyncService) checkToken(r *http.Request) bool {
	return r.URL.Query().Get("token") == s.token
}

// requireToken 鉴权：token 可在 URL query（?token=）或 JSON body（{token}）里。
// body 读取后恢复，供后续 handler 继续解析加密 payload。
func (s *LanSyncService) requireToken(w http.ResponseWriter, r *http.Request) bool {
	tok := r.URL.Query().Get("token")
	if tok == "" && r.Body != nil {
		bodyBytes, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		var b struct {
			Token string `json:"token"`
		}
		_ = json.Unmarshal(bodyBytes, &b)
		tok = b.Token
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes)) // 恢复 body
	}
	if tok != s.token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

// GET /lan/info?token=xxx 返回连接信息（明文：token 本身就是凭证）
func (s *LanSyncService) handleInfo(w http.ResponseWriter, r *http.Request) {
	if !s.requireToken(w, r) {
		return
	}
	json.NewEncoder(w).Encode(map[string]any{
		"ip":     getLanIP(),
		"port":   s.port,
		"token":  s.token,
		"device": "re0",
	})
}

// POST /lan/memory/pull body:{token} → 响应 {ct, iv} 加密的 {files, mod_times}
func (s *LanSyncService) handleMemoryPull(w http.ResponseWriter, r *http.Request) {
	if !s.requireToken(w, r) {
		return
	}
	files := make(map[string]string)
	modTimes := make(map[string]string)
	for f := range memorydir.SyncableFiles {
		if c := memorydir.ReadRaw(f); c != "" {
			files[f] = c
			modTimes[f] = memorydir.FileModTime(f).Format(time.RFC3339)
		}
	}
	respBytes, _ := json.Marshal(map[string]any{"files": files, "mod_times": modTimes})
	writeEncrypted(w, s.token, respBytes)
}

// POST /lan/memory/push body:{token, ct, iv} → 加密的 {files}，响应加密 {ok, written}
func (s *LanSyncService) handleMemoryPush(w http.ResponseWriter, r *http.Request) {
	if !s.requireToken(w, r) {
		return
	}
	var req struct {
		Token string `json:"token"`
		CT    string `json:"ct"`
		IV    string `json:"iv"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	plain, err := decryptPayload(s.token, req.CT, req.IV)
	if err != nil {
		http.Error(w, "解密失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	var inner struct {
		Files map[string]string `json:"files"`
	}
	if err := json.Unmarshal(plain, &inner); err != nil {
		http.Error(w, "payload 解析失败", http.StatusBadRequest)
		return
	}
	written := 0
	for name, content := range inner.Files {
		// WriteRaw 只写 SyncableFiles 白名单文件，防路径穿越
		if err := memorydir.WriteRaw(name, content); err == nil {
			written++
		}
	}
	respBytes, _ := json.Marshal(map[string]any{"ok": true, "written": written})
	writeEncrypted(w, s.token, respBytes)
}

// POST /lan/sessions/pull body:{token} → 响应 {ct, iv} 加密的 {sessions}
func (s *LanSyncService) handleSessionsPull(w http.ResponseWriter, r *http.Request) {
	if !s.requireToken(w, r) {
		return
	}
	type sessionDetail struct {
		ID       string   `json:"id"`
		Title    string   `json:"title"`
		Messages []msgDTO `json:"messages"`
	}
	var out []sessionDetail
	for _, si := range s.store.List() {
		msgs := s.store.Get(si.ID)
		var msgsOut []msgDTO
		for _, m := range msgs {
			msgsOut = append(msgsOut, msgDTO{Role: m.Role, Content: m.Content, Model: m.Model, Status: m.Status})
		}
		out = append(out, sessionDetail{ID: si.ID, Title: si.Title, Messages: msgsOut})
	}
	respBytes, _ := json.Marshal(map[string]any{"sessions": out})
	writeEncrypted(w, s.token, respBytes)
}

// POST /lan/sessions/push body:{token, ct, iv} → 加密的 {sessions}，响应加密 {ok, count}
func (s *LanSyncService) handleSessionsPush(w http.ResponseWriter, r *http.Request) {
	if !s.requireToken(w, r) {
		return
	}
	var req struct {
		Token string `json:"token"`
		CT    string `json:"ct"`
		IV    string `json:"iv"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	plain, err := decryptPayload(s.token, req.CT, req.IV)
	if err != nil {
		http.Error(w, "解密失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	var inner struct {
		Sessions []struct {
			ID       string   `json:"id"`
			Title    string   `json:"title"`
			Messages []msgDTO `json:"messages"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal(plain, &inner); err != nil {
		http.Error(w, "payload 解析失败", http.StatusBadRequest)
		return
	}
	count := 0
	for _, ss := range inner.Sessions {
		if ss.ID == "" {
			continue
		}
		s.store.Delete(ss.ID)
		if ss.Title != "" {
			s.store.SetSessionTitle(ss.ID, ss.Title)
		}
		for _, m := range ss.Messages {
			s.store.Append(ss.ID, DSMessage{Role: m.Role, Content: m.Content, Model: m.Model, Status: m.Status, Timestamp: time.Now()})
		}
		count++
	}
	respBytes, _ := json.Marshal(map[string]any{"ok": true, "count": count})
	writeEncrypted(w, s.token, respBytes)
}

// msgDTO 轻量消息传输（不含内部字段 Timestamp(-) / WorkflowID(-) / Blocks(-)）
type msgDTO struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Model   string `json:"model,omitempty"`
	Status  string `json:"status,omitempty"`
}

// ── AES-GCM payload 加密（token 派生密钥，每包随机 IV）──

func aesGCMKey(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:16] // AES-128
}

// encryptPayload AES-GCM 加密，返回 {ct, iv}（base64）
func encryptPayload(token string, data []byte) (map[string]string, error) {
	block, err := aes.NewCipher(aesGCMKey(token))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	iv := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(iv); err != nil {
		return nil, err
	}
	ct := gcm.Seal(nil, iv, data, nil)
	return map[string]string{
		"ct": base64.StdEncoding.EncodeToString(ct),
		"iv": base64.StdEncoding.EncodeToString(iv),
	}, nil
}

// decryptPayload 解密 {ct, iv}
func decryptPayload(token, ctB64, ivB64 string) ([]byte, error) {
	ct, err := base64.StdEncoding.DecodeString(ctB64)
	if err != nil {
		return nil, err
	}
	iv, err := base64.StdEncoding.DecodeString(ivB64)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(aesGCMKey(token))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, iv, ct, nil)
}

// writeEncrypted 把 data JSON 加密后写响应
func writeEncrypted(w http.ResponseWriter, token string, data []byte) {
	enc, err := encryptPayload(token, data)
	if err != nil {
		http.Error(w, "加密失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(enc)
}
