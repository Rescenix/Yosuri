package handler

// memory_sync.go —— 云端记忆同步（可选）。
//
// 把本地记忆目录打包成 JSON 记忆包同步到 ResceneCloud（按 UID 账号存储），
// 换设备登录后拉回 —— 记忆跟账号走，与亲密等级/统计同一套"珍惜账号"语义。
//
//   - 可选开关：环境变量 RESCENE_MEMORY_SYNC=off 关闭（默认开启）。关闭后不推不拉。
//   - 推送＝云端备份，仅登录用户（2026-09-04 定稿）：游客不再向云端写记忆包。
//     ① StartMemorySyncLoop 定时全量推送（启动 2s 后立即推一次，此后每 60s 一次）；
//     ② 记忆写工具（remember / memory_pin / memory_append / memory_handoff）
//       写完后异步补推一次（fire-and-forget，不阻塞）。
//     兼容旧版：游客期已上传的记忆包不删除，UID 并入正式账号后照样能拉回。
//   - 拉取：re0 启动时（有 uid 缓存）自动拉一次；前端也可在登录/启动后显式调
//     POST /api/memory/sync/pull 触发。
//   - 合并策略：拉取时云端文件写回本地（同名覆盖），本地独有文件保留 —— 换设备
//     后本地记忆为空或稀疏，云端历史能直接恢复；推送是本地全量覆盖云端（后写赢）。
//   - 隐私：只同步记忆 md 文本（偏好/决策/索引），不同步会话内容与 handoff 工作态。

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"log"
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

// uidFromToken 从 JWT payload 解出 uid（登录 token 或游客 token 均可）。
// 不依赖 intimacy.md（那个只在亲密度接口成功后落盘——登录用户若亲密度接口用的
// 登录 token 而推送用游客 token，uid 对不上 → 403 → 零上传，2026-08-30 实锤）。
// JWT 格式 header.payload.signature，payload 是 base64(JSON)，本地只读缓存 token
// 解 payload 即可（不解密不验签，不涉及密钥）。
func uidFromToken(tok string) int64 {
	if tok == "" {
		return 0
	}
	parts := strings.Split(tok, ".")
	if len(parts) < 2 {
		return 0
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// 兼容带 padding 的编码
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return 0
		}
	}
	var claims struct {
		UID json.Number `json:"uid"`
	}
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.UseNumber()
	if err := dec.Decode(&claims); err != nil {
		return 0
	}
	if n, err := claims.UID.Int64(); err == nil {
		return n
	}
	return 0
}

// uidFromGuestToken 从本地缓存的 guest JWT payload 解出 uid（兼容旧调用）。
func uidFromGuestToken() int64 {
	return uidFromToken(readGuestToken())
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

// ── 登录 token 缓存（2026-08-30：登录用户用登录 token 推送记忆，guest token 的 uid
// 是游客号，与登录账号 uid 不匹配 → 云端 requireUIDMatch 403 → 零上传） ──

func loginTokenPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "rescene_data", "cloud_login_token")
}

// readLoginToken 读本地缓存的登录 JWT（member token，含 uid=登录账号）。
func readLoginToken() string {
	if p := loginTokenPath(); p != "" {
		if data, err := os.ReadFile(p); err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return ""
}

// readAuthToken 优先读登录 token，回退 guest token。
// 登录用户用 member JWT（uid=登录账号），游客用 guest JWT（uid=游客号）——
// 两者 uid 必须匹配云端 requireUIDMatch，否则 403 静默失败。
func readAuthToken() (token string, isLogin bool) {
	if t := readLoginToken(); t != "" {
		return t, true
	}
	return readGuestToken(), false
}

// syncIdentity 云端记忆同步用的当前身份：token、uid、是否登录账号。
// uid 一律从「实际鉴权 token」解，保证与云端 requireUIDMatch 同源。
func syncIdentity() (token string, uid int64, isLogin bool) {
	tok, login := readAuthToken()
	return tok, uidFromToken(tok), login
}

// HandleLoginTokenStore POST /api/auth/login-token {token}
// 前端登录成功后（refresh / login）把 member JWT 缓存到本地，供 push 鉴权。
func HandleLoginTokenStore(c *gin.Context) {
	var req struct {
		Token string `json:"token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 token"})
		return
	}
	p := loginTokenPath()
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

// HandleLoginTokenGet GET /api/auth/login-token
// 前端重启后 WebView2 localStorage 丢失时，从这里恢复登录 token（2026-08-30：
// WebView2 数据目录随 exe 路径变化 → localStorage 清空 → 登录身份丢失，改为
// 从 rescene_data 持久化文件恢复，登录态不再依赖浏览器存储）。
func HandleLoginTokenGet(c *gin.Context) {
	tok := readLoginToken()
	if tok == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "无缓存登录态"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": tok})
}

// HandleLoginTokenDelete DELETE /api/auth/login-token
// 前端登出时清掉缓存 token，避免重启后 restoreLoginToken 又自动恢复登录态。
func HandleLoginTokenDelete(c *gin.Context) {
	p := loginTokenPath()
	if p != "" {
		_ = os.Remove(p)
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// HandleMemorySyncSettings GET /api/memory/sync/settings → {enabled, env_override, logged_in}
// env_override=true 表示部署级 RESCENE_MEMORY_SYNC=off 强制关闭，前端开关应禁用。
// logged_in=false 表示当前是游客身份：云端备份只对登录账号开放，前端开关同样禁用。
func HandleMemorySyncSettings(c *gin.Context) {
	_, _, isLogin := syncIdentity()
	c.JSON(http.StatusOK, gin.H{
		"enabled":      memorySyncEnabled(),
		"env_override": strings.ToLower(os.Getenv("RESCENE_MEMORY_SYNC")) == "off",
		"logged_in":    isLogin,
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

// pushMemorySync 异步把本地记忆全量推送到云端（备份）。
//
// 2026-09-04 定稿：只给登录用户备份。游客（未登录）不再向云端写记忆包——
// 游客号无人认领、换设备即失效，写上去既占云端存储也谈不上"跟账号走"，
// 记忆留在本地即可。登录判定只看 login token（member JWT），游客 token 一律不推。
//
// uid 与 token 必须同源（云端 requireUIDMatch 校验 JWT uid == 请求 uid）：
// 统一从「实际鉴权 token」解 uid，杜绝 body uid 与 JWT uid 错配导致 403。
// 带重试（网络抖动/云端冷启动）与失败日志——之前 fire-and-forget 连日志都没有，
// 401/超时全吞掉，「零上传」时无从排查（2026-08-30 实锤）。
func pushMemorySync() {
	if !memorySyncEnabled() {
		return
	}
	tok, uid, isLogin := syncIdentity()
	if !isLogin || uid <= 0 {
		return // 未登录（游客）：不备份，静默跳过
	}
	payload := memorySyncPayload()
	if payload == "" {
		return
	}
	body, _ := json.Marshal(map[string]any{"uid": uid, "payload": payload})
	client := &http.Client{Timeout: 8 * time.Second}
	target := cloudAuthBase() + "/api/memory/sync"

	go func() {
		// 最多重试 3 次（间隔 1s/2s/4s）：Render 免费实例冷启动慢，首击 401/超时后重试能补上
		for attempt := 1; attempt <= 3; attempt++ {
			req, err := http.NewRequest(http.MethodPost, target, bytes.NewReader(body))
			if err != nil {
				return
			}
			req.Header.Set("Content-Type", "application/json")
			// 鉴权头：云端要求 JWT uid == 请求 uid（08-17 起），不带会 401 静默失败
			if tok != "" {
				req.Header.Set("Authorization", "Bearer "+tok)
			}
			resp, err := client.Do(req)
			if err != nil {
				log.Printf("[memory-sync] 推送失败(uid=%d, 第%d次): %v", uid, attempt, err)
				time.Sleep(time.Duration(attempt) * time.Second)
				continue
			}
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return // 成功
			}
			log.Printf("[memory-sync] 推送被拒(uid=%d, HTTP %d, 第%d次)", uid, resp.StatusCode, attempt)
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}()
}

// pullMemorySync 从云端拉取记忆包并写回本地。返回是否成功拉取并应用。
// 与 push 同源原则：token 优先登录 token（登录用户 uid=登录账号），回退 guest token。
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
	if tok, _ := readAuthToken(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	res, err := client.Do(req)
	if err != nil {
		log.Printf("[memory-sync] 拉取失败(uid=%d): %v", uid, err)
		return false
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		switch res.StatusCode {
		case http.StatusNotFound:
			// 404 = 账号还没有云端记忆，正常
			log.Printf("[memory-sync] 拉取无云端记忆(uid=%d, HTTP 404) —— 正常", uid)
		case http.StatusForbidden:
			log.Printf("[memory-sync] 拉取被拒(uid=%d, HTTP 403) —— JWT uid 与请求 uid 不匹配（token 缓存过期/错源），检查 cloud_login_token 与 cloud_guest_token", uid)
		case http.StatusUnauthorized:
			log.Printf("[memory-sync] 拉取被拒(uid=%d, HTTP 401) —— token 无效/过期", uid)
		default:
			log.Printf("[memory-sync] 拉取被拒(uid=%d, HTTP %d)", uid, res.StatusCode)
		}
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

// HandleMemorySyncPull 显式触发：从云端拉记忆包写回本地（前端登录/启动后调用，跨设备恢复记忆）。
// uid 从「实际鉴权 token」解析（登录 token 优先，回退 guest token），而不是信任前端传的 uid：
// 前端可能残留过期登录 token 的旧 uid（游客号），而后端 readAuthToken 取的是登录 token（账号 uid），
// 二者不匹配 → 云端 requireUIDMatch 403 → 拉取永远失败（2026-09-02 实锤）。
// 兼容旧调用方：body 里的 uid 仅作兜底，token 解不出 uid 时用。
func HandleMemorySyncPull(c *gin.Context) {
	uid := uidFromToken(mustAuthToken())
	if uid <= 0 {
		// 兜底：前端传的 uid（旧调用方/无 token 场景）
		var req struct {
			UID int64 `json:"uid"`
		}
		if err := c.ShouldBindJSON(&req); err == nil && req.UID > 0 {
			uid = req.UID
		}
	}
	if uid <= 0 {
		uid, _ = memorydir.ReadIntimacy()
	}
	if uid <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无法确定 uid"})
		return
	}
	ok := pullMemorySync(uid)
	c.JSON(http.StatusOK, gin.H{"ok": ok, "restored": ok})
}

// HandleMemorySyncPush POST /api/memory/sync/push {uid}
// 显式触发：把本地记忆全量推送到云端（前端手动同步按钮）。
// 与自动备份同规则：仅登录用户可备份，游客返回明确提示。
func HandleMemorySyncPush(c *gin.Context) {
	var req struct {
		UID int64 `json:"uid"`
	}
	tok, tokenUID, isLogin := syncIdentity()
	if !isLogin || tokenUID <= 0 {
		c.JSON(http.StatusOK, gin.H{"ok": false, "reason": "登录账号后才提供云端记忆备份"})
		return
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
	// uid 以鉴权 token 为准，忽略前端传值（防错配 403）
	body, _ := json.Marshal(map[string]any{"uid": tokenUID, "payload": payload})
	client := &http.Client{Timeout: 5 * time.Second}
	r, err := http.NewRequest(http.MethodPost, cloudAuthBase()+"/api/memory/sync", bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "构造请求失败"})
		return
	}
	r.Header.Set("Content-Type", "application/json")
	// 鉴权头：云端要求 JWT uid == 请求 uid，不带会 401
	if tok != "" {
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
		uid := uidFromToken(mustAuthToken())
		if uid <= 0 {
			uid, _ = memorydir.ReadIntimacy()
		}
		if uid > 0 {
			pullMemorySync(uid)
		}
	}()
}

// mustAuthToken 取当前可用鉴权 token（登录优先，回退游客）。
func mustAuthToken() string {
	tok, _ := readAuthToken()
	return tok
}

// memorySyncLoopInterval 定时全量同步周期（2026-08-28 用户定稿：开启即自动同步）。
const memorySyncLoopInterval = 60 * time.Second

// StartMemorySyncLoop 云端记忆同步定时循环（2026-08-28 用户定稿新增）：
// 不再等记忆写工具才推送——开关开启（默认开）时，启动 2s 后立即全量推+拉一次，
// 此后每 60s 双向同步一次（push 全量覆盖云端 / pull 比较后写回本地，防旧覆盖新）。
// 开关关闭或 uid 未就绪时静默跳过；每次 tick 都重新检查开关，随时生效。
// uid 从「实际鉴权 token」解析（2026-08-30 修复：登录用户用 login token、游客用
// guest token，二者 uid 与云端 requireUIDMatch 匹配，杜绝零上传）。
// 2026-09-04：备份（push）只对登录用户生效；拉取仍按当前身份走，旧版游客期已备份
// 的记忆在 UID 并入账号后照样能拉回。
func StartMemorySyncLoop() {
	go func() {
		time.Sleep(2 * time.Second) // 等 uid 缓存 / 前端 guest-token/login-token 落盘 / 网络就绪
		for {
			if memorySyncEnabled() {
				_, uid, isLogin := syncIdentity()
				if uid <= 0 {
					// 兼容旧版：无 token 缓存时回退 intimacy.md 里的 uid（只用于拉取）
					uid, _ = memorydir.ReadIntimacy()
				}
				if isLogin {
					pushMemorySync()
				}
				if uid > 0 {
					pullMemorySync(uid)
				}
			}
			time.Sleep(memorySyncLoopInterval)
		}
	}()
}
