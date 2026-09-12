package handler

// memory_crypto.go —— 桌面端记忆加密（2026-09-12，方案A）。
//
// 与云端 ResceneCloud auth_v2.go / 手机端 MemoryStore / 官网 chat-v2.js 共用同一套
// 派生规范（salt 字符串、迭代次数、输出长度必须三端完全一致，改一处全端失效）：
//
//   login_hash = PBKDF2-HMAC-SHA256(password, "rescene-login:"+username, 600000, 32B) → 登录用（云端存这个）
//   KEK        = PBKDF2-HMAC-SHA256(password, "rescene-mem:"+username,  210000, 32B) → 包装 AK
//   AK         = 32B 随机账号密钥（与密码无关，改密码不变）
//   wrapped_ak = AES-256-GCM(KEK, AK)                       → 存云端（密码锁住的 AK）
//   payload    = AES-256-GCM(AK, {文件名: 内容} JSON)        → 存云端（{"v":2,"iv","ct"}）
//
// 信任边界：密码明文只在用户设备上（本机回环进本地后端），转发云端时只发 login_hash；
// 云端无密码、无 KEK、无 AK，只能看到 wrapped_ak + 密文 payload，零解密能力。

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/pbkdf2"
)

const (
	loginPBKDF2Iter = 600000 // 与云端 auth_v2.go 完全一致
	keksPBKDF2Iter  = 210000
	cryptoKeyLen    = 32
)

// memLoginSalt 登录哈希盐：与云端一致。
func memLoginSalt(username string) string { return "rescene-login:" + strings.ToLower(strings.TrimSpace(username)) }

// memKEKSalt 密钥包装盐：与云端一致。
func memKEKSalt(username string) string { return "rescene-mem:" + strings.ToLower(strings.TrimSpace(username)) }

// MemLoginHash 客户端派生登录哈希（只发这个上云，密码明文不出设备）。
func MemLoginHash(password, username string) string {
	dk := pbkdf2.Key([]byte(password), []byte(memLoginSalt(username)), loginPBKDF2Iter, cryptoKeyLen, sha256.New)
	return hex.EncodeToString(dk)
}

// memDeriveKEK 密码派生密钥包装密钥。
func memDeriveKEK(password, username string) []byte {
	return pbkdf2.Key([]byte(password), []byte(memKEKSalt(username)), keksPBKDF2Iter, cryptoKeyLen, sha256.New)
}

// memSeal 加密，输出 base64(iv|ct)。
func memSeal(key, plaintext []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	iv := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(iv); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(gcm.Seal(iv, iv, plaintext, nil)), nil
}

// memOpen 解密 memSeal 的输出。
func memOpen(key []byte, sealedB64 string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(sealedB64)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize() {
		return nil, fmt.Errorf("密文过短")
	}
	return gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
}

// akDiskPath AK 落盘位置：rescene_data/memory_ak_<uid>。
// AK 是「解锁云端密文的钥匙」，存在用户本机（威胁模型=云端被拖/不可信，本地=用户设备可信）。
func akDiskPath(uid int64) string {
	return filepath.Join(dataRootDir(), fmt.Sprintf("memory_ak_%d", uid))
}

// recoveryCodePath 恢复码落盘位置：rescene_data/memory_recovery_<uid>。
// 恢复码 = 本地自动生成的第二把钥匙（SSH 私钥同思路）：忘密码时客户端本地读它解
// 云端副本，拿回 AK。用户无感、云端无钥匙、真 E2E 保持（2026-09-12 用户定稿简化版）。
func recoveryCodePath(uid int64) string {
	return filepath.Join(dataRootDir(), fmt.Sprintf("memory_recovery_%d", uid))
}

// EnsureRecoveryCode 确保本机持有恢复码：无则生成（12 字符大写字母+数字，如 ABCD-EFGH-JKLM）
// 并落盘。返回恢复码；失败返回空串（不阻塞主流程）。
func EnsureRecoveryCode(uid int64) string {
	if b, err := os.ReadFile(recoveryCodePath(uid)); err == nil {
		code := strings.TrimSpace(string(b))
		if len(code) >= 8 {
			return code
		}
	}
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // 去掉易混淆 I/O/0/1
	code := make([]byte, 14)
	if _, err := rand.Read(code); err != nil {
		return ""
	}
	for i := range code {
		code[i] = charset[int(code[i])%len(charset)]
	}
	s := string(code[:4]) + "-" + string(code[4:8]) + "-" + string(code[8:12]) + "-" + string(code[12:14])
	if err := os.WriteFile(recoveryCodePath(uid), []byte(s), 0o600); err != nil {
		return ""
	}
	return s
}

// recoverKEK 恢复码派生密钥包装密钥：与云端 ak_recover.go 规范一致。
func recoverKEK(code, username string) []byte {
	return pbkdf2.Key([]byte(code), []byte("rescene-recover:"+strings.ToLower(strings.TrimSpace(username))), keksPBKDF2Iter, cryptoKeyLen, sha256.New)
}

// SetRecoveryEnvelope 用恢复码锁 AK 并上传副本到云端（登录态）。
// 幂等：云端副本被覆盖为最新（AK 不变则副本不变）。
func SetRecoveryEnvelope(uid int64, username string) error {
	ak := loadAKFromDisk(uid)
	if len(ak) != 32 {
		return fmt.Errorf("本机无 AK")
	}
	code := EnsureRecoveryCode(uid)
	if code == "" {
		return fmt.Errorf("恢复码生成失败")
	}
	kek := recoverKEK(code, username)
	sealed, err := memSeal(kek, ak)
	if err != nil {
		return err
	}
	tok, _, _ := syncIdentity()
	if tok == "" {
		return fmt.Errorf("无登录态")
	}
	body, _ := json.Marshal(map[string]any{"wrapped_ak_recovery": sealed})
	req, err := http.NewRequest("POST", cloudAuthBase()+"/api/memory/ak/recovery-set", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := cloudHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("recovery-set HTTP %d", resp.StatusCode)
	}
	return nil
}

// SetRecoveryEnvelopeWithToken 用恢复码锁 AK 并上传副本到云端（登录态，token 直传不读盘）。
func SetRecoveryEnvelopeWithToken(uid int64, username, token string) error {
	ak := loadAKFromDisk(uid)
	if len(ak) != 32 {
		return fmt.Errorf("本机无 AK")
	}
	code := EnsureRecoveryCode(uid)
	if code == "" {
		return fmt.Errorf("恢复码生成失败")
	}
	kek := recoverKEK(code, username)
	sealed, err := memSeal(kek, ak)
	if err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]any{"wrapped_ak_recovery": sealed})
	req, err := http.NewRequest("POST", cloudAuthBase()+"/api/memory/ak/recovery-set", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := cloudHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("recovery-set HTTP %d", resp.StatusCode)
	}
	return nil
}

// RecoverAKWithLocalCode 忘密码恢复：用本地恢复码解云端副本，返回 AK。
func RecoverAKWithLocalCode(uid int64, username, wrappedRecovery string) ([]byte, error) {
	code := EnsureRecoveryCode(uid)
	if code == "" {
		return nil, fmt.Errorf("本机无恢复码")
	}
	return memOpen(recoverKEK(code, username), wrappedRecovery)
}

// usernameFromToken 从 JWT payload 解出 login（账号名），与 uidFromToken 同思路。
func usernameFromToken(tok string) string {
	if tok == "" {
		return ""
	}
	parts := strings.Split(tok, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return ""
		}
	}
	var claims struct {
		Login string `json:"login"`
	}
	if err := json.NewDecoder(bytes.NewReader(payload)).Decode(&claims); err != nil {
		return ""
	}
	return claims.Login
}

// HandleChangePwd 正常改密码（记得旧密码）：POST /api/memory/ak/change-pwd。
// body: {old_password, new_password}（登录态）。
// 流程：本地 AK 用新密码 KEK 重打包 → 云端 ak/rotate（服务端验证旧哈希）→
// AK 不变、恢复码副本不变（恢复码锁的同一个 AK）→ 记忆原封不动。
func HandleChangePwd(c *gin.Context) {
	tok, uid, isLogin := syncIdentity()
	if !isLogin || uid <= 0 || tok == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体不是合法 JSON"})
		return
	}
	var username string
	if tok != "" {
		username = usernameFromToken(tok)
	}
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "账号信息异常"})
		return
	}
	// 本机 AK：改密只重打包，AK 不变（记忆密钥与密码无关）
	ak := loadAKFromDisk(uid)
	if len(ak) != 32 {
		c.JSON(http.StatusConflict, gin.H{"error": "本机没有记忆密钥，请先用当前密码重新登录一次"})
		return
	}
	newKEK := memDeriveKEK(req.NewPassword, username)
	newWrapped, err := memSeal(newKEK, ak)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "信封生成失败"})
		return
	}
	body, _ := json.Marshal(map[string]any{
		"old_password_hash": MemLoginHash(req.OldPassword, username),
		"new_password_hash": MemLoginHash(req.NewPassword, username),
		"new_wrapped_ak":    newWrapped,
	})
	upReq, err := http.NewRequest("POST", cloudAuthBase()+"/api/memory/ak/rotate", strings.NewReader(string(body)))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "改密请求构造失败"})
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
	respBody, _ := readUpstreamBody(resp)
	if resp.StatusCode != 200 {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(respBody, &e)
		msg := e.Error
		if msg == "" {
			msg = "旧密码验证失败或云端异常"
		}
		c.JSON(resp.StatusCode, gin.H{"error": msg})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// HandleRecoveryCodeGet 备份恢复码：GET /api/memory/ak/recovery-code（登录态）。
// 返回本地恢复码（无则自动生成落盘）——只在用户主动备份时展示，平时零感知。
func HandleRecoveryCodeGet(c *gin.Context) {
	tok, _, _ := syncIdentity()
	if tok == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}
	uid := uidFromToken(tok)
	if uid <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "登录态无效"})
		return
	}
	code := EnsureRecoveryCode(uid)
	if code == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "恢复码生成失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"recovery_code": code})
}

// HandleRecoverDo 忘密码一站式恢复：POST /api/memory/ak/recover-do。
// 前端只传 {username, email, code(邮箱验证码), new_password, recovery_code?}，后端一把梭：
//   1. 云端 claim（验证码证明邮箱所有权）→ 拿恢复副本密文
//   2. 恢复码解副本 → 拿回 AK：优先用户输入的 recovery_code（跨设备/换机场景，
//      用当初备份的恢复码），否则用本机恢复码（同设备场景，用户无感）
//   3. 新密码派生新 KEK → 重新包装 AK → 云端 finalize（新密码登录 + 记忆原封不动）
func HandleRecoverDo(c *gin.Context) {
	var req struct {
		Username     string `json:"username" binding:"required"`
		Email        string `json:"email" binding:"required"`
		Code         string `json:"code" binding:"required"` // 邮箱验证码
		NewPassword  string `json:"new_password" binding:"required"`
		RecoveryCode string `json:"recovery_code"` // 可选：用户备份的恢复码（跨设备）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体不是合法 JSON"})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Code = strings.TrimSpace(req.Code)

	// 1) 云端 claim：验证码换恢复副本 + 恢复 JWT
	claimBody, _ := json.Marshal(map[string]any{"username": req.Username, "email": req.Email, "code": req.Code})
	claimReq, err := http.NewRequest("POST", cloudAuthBase()+"/api/auth/ak-recover-claim", strings.NewReader(string(claimBody)))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "恢复请求构造失败"})
		return
	}
	claimReq.Header.Set("Content-Type", "application/json")
	claimResp, err := cloudHTTPClient.Do(claimReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": cloudErrorMessage(err)})
		return
	}
	claimBodyResp, _ := readUpstreamBody(claimResp)
	claimResp.Body.Close()
	if claimResp.StatusCode != 200 {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(claimBodyResp, &e)
		msg := e.Error
		if msg == "" {
			msg = "验证码验证失败"
		}
		c.JSON(claimResp.StatusCode, gin.H{"error": msg})
		return
	}
	var claim struct {
		Token             string `json:"token"`
		WrappedRecovery   string `json:"wrapped_ak_recovery"`
	}
	if err := json.Unmarshal(claimBodyResp, &claim); err != nil || claim.Token == "" || claim.WrappedRecovery == "" {
		c.JSON(http.StatusBadGateway, gin.H{"error": "云端响应异常"})
		return
	}
	// 2) 恢复码解副本 → AK（优先用户备份码，其次本机恢复码）
	uid := uidFromToken(claim.Token)
	if uid <= 0 {
		c.JSON(http.StatusBadGateway, gin.H{"error": "恢复凭证异常"})
		return
	}
	var ak []byte
	if strings.TrimSpace(req.RecoveryCode) != "" {
		ak, err = memOpen(recoverKEK(strings.TrimSpace(req.RecoveryCode), req.Username), claim.WrappedRecovery)
	} else {
		ak, err = RecoverAKWithLocalCode(uid, req.Username, claim.WrappedRecovery)
	}
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "恢复失败：恢复码不正确或本机没有这台设备的恢复码。请用原设备，或输入当初备份的恢复码"})
		return
	}
	// 3) 新密码重打包 AK → finalize
	newKEK := memDeriveKEK(req.NewPassword, req.Username)
	newWrapped, err := memSeal(newKEK, ak)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "信封生成失败"})
		return
	}
	recoveryCode := EnsureRecoveryCode(uid)
	recKek := recoverKEK(recoveryCode, req.Username)
	newWrappedRec, err := memSeal(recKek, ak)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "恢复副本生成失败"})
		return
	}
	finalBody, _ := json.Marshal(map[string]any{
		"new_password_hash":       MemLoginHash(req.NewPassword, req.Username),
		"new_wrapped_ak":          newWrapped,
		"new_wrapped_ak_recovery": newWrappedRec,
	})
	finalReq, err := http.NewRequest("POST", cloudAuthBase()+"/api/memory/ak/recover-finalize", strings.NewReader(string(finalBody)))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "恢复提交构造失败"})
		return
	}
	finalReq.Header.Set("Content-Type", "application/json")
	finalReq.Header.Set("Authorization", "Bearer "+claim.Token)
	finalResp, err := cloudHTTPClient.Do(finalReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": cloudErrorMessage(err)})
		return
	}
	defer finalResp.Body.Close()
	if finalResp.StatusCode != 200 {
		c.JSON(finalResp.StatusCode, gin.H{"error": "恢复提交失败，请稍后再试"})
		return
	}
	// 本地 AK 用新 KEK 覆盖不了——AK 文件本身没变（AK 不变），新密码下次登录自动解锁
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// dataRootDir 本地数据根目录（与 memory_sync 同源）。
func dataRootDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "rescene_data")
}

// loadAKFromDisk 读本地 AK 文件；不存在返回空。
func loadAKFromDisk(uid int64) []byte {
	b, err := os.ReadFile(akDiskPath(uid))
	if err != nil || len(b) == 0 {
		return nil
	}
	return b
}

// storeAKToDisk 保存 AK 到本地（0600 权限）。
func storeAKToDisk(uid int64, ak []byte) error {
	return os.WriteFile(akDiskPath(uid), ak, 0o600)
}

// EnsureAccountAK 登录后确保本机持有账号密钥 AK：
//   - 云端已有 wrapped_ak → 用密码 KEK 解开，落盘
//   - 云端没有（老账号迁移）→ 生成新 AK，KEK 包装后 POST /api/memory/ak/set 存云端，落盘
//   - 云端有信封但解不开（改过密码/信封损坏）→ 返回 nil，绝不生成新 AK 覆盖云端旧记忆
// token 参数：登录响应的 member JWT（避免依赖盘上 cloud_login_token——登录 goroutine
// 启动时前端可能还没把 token 存盘，读盘必然失败导致 AK 解锁永远不落盘，2026-09-12 实锤）。
func EnsureAccountAK(uid int64, username, password, wrappedAK, token string) []byte {
	if ak := loadAKFromDisk(uid); len(ak) == 32 {
		return ak
	}
	kek := memDeriveKEK(password, username)
	if wrappedAK != "" {
		if ak, err := memOpen(kek, wrappedAK); err == nil && len(ak) == 32 {
			_ = storeAKToDisk(uid, ak)
			return ak
		}
		// 云端有信封但解不开：不生成新 AK（会覆盖云端旧记忆），保持无密钥态
		log.Printf("⚠️ 账号 %d 云端信封解不开（密码可能已改），旧记忆保持不动，等待走找回/改密流程", uid)
		return nil
	}
	// 云端无信封（老账号/首次）：生成新 AK 并注册信封；注册失败则不落盘（防本地新 AK 加密覆盖云端）。
	ak := make([]byte, 32)
	if _, err := rand.Read(ak); err != nil {
		return nil
	}
	sealed, err := memSeal(kek, ak)
	if err != nil {
		return nil
	}
	if err := postAKSetWithToken(uid, sealed, token); err != nil {
		log.Printf("⚠️ 账号 %d 信封注册失败: %v（保持无密钥态，下次登录重试）", uid, err)
		return nil
	}
	_ = storeAKToDisk(uid, ak)
	return ak
}

// postAKSetWithToken 用指定 token 上报信封（登录响应直传，不读盘）。
func postAKSetWithToken(uid int64, wrappedAK, token string) error {
	if token == "" {
		return fmt.Errorf("无登录态")
	}
	body, _ := json.Marshal(map[string]any{"wrapped_ak": wrappedAK})
	req, err := http.NewRequest("POST", cloudAuthBase()+"/api/memory/ak/set", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := cloudHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("ak/set HTTP %d", resp.StatusCode)
	}
	return nil
}

// EncryptMemoryPayload 加密整个记忆包：输出 {"v":2,"iv":...,"ct":...}（云端存这个）。
func EncryptMemoryPayload(payload string, ak []byte) (string, error) {
	sealed, err := memSeal(ak, []byte(payload))
	if err != nil {
		return "", err
	}
	out, _ := json.Marshal(map[string]any{"v": 2, "ct": sealed})
	return string(out), nil
}

// DecryptMemoryPayload 解密云端记忆包：
//   - v2 密文格式 → 用 AK 解出明文 JSON
//   - 老明文格式（{文件名:内容}）→ 原样返回（存量兼容；含凭证的已在云端清理）
// 返回解出的明文 payload（map JSON），失败返回空串。
func DecryptMemoryPayload(payload string, ak []byte) string {
	var env struct {
		V  int    `json:"v"`
		CT string `json:"ct"`
	}
	if err := json.Unmarshal([]byte(payload), &env); err == nil && env.V == 2 && env.CT != "" && len(ak) == 32 {
		plain, err := memOpen(ak, env.CT)
		if err != nil {
			return ""
		}
		return string(plain)
	}
	// 老明文格式：直接透传（本地无 AK 或云端仍是老数据）。
	return payload
}
