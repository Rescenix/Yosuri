package handler

// cloud_auth.go —— re0 侧的鉴权薄中间件。
//
// 鉴权逻辑已全部收口到独立私有服务 ResceneCloud（C:\Pro2026\ResceneCloud，对应私有仓
// github.com/Rescenix/ResceneCloud）。本文件只做三件事，不持有任何密钥 / OAuth 逻辑：
//   1. 把 /api/login、/api/auth/me 等流量代理到 RESCENE_CLOUD_URL（含 Authorization 透传）
//   2. /api/auth/me 直接由云端验签回传 is_vip 等（2026-08-19 起不再走本地
//      middleware.AuthRequired()，避免打包版无 .env 时本地默认密钥与云端不符登不上）
//   3. 暴露 RESCENE_CLOUD_URL 给前端（/api/auth/cloud-config），供其直接发起 GitHub 登录
//
// 这样开源的 re0 不含任何付费/鉴权密钥，商业闭环留在私有 ResceneCloud。

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"backend/internal/memorydir"

	"github.com/gin-gonic/gin"
)

// cloudHTTPClient 转发到 ResceneCloud 的专用客户端，带超时兜底。
//
// 修复：之前用 http.DefaultClient（零超时）转发登录/验签请求——ResceneCloud 部署在
// Render 免费实例上，闲置后会休眠，冷启动常需 10~50s。零超时下前端 fetch 会一直
// 卡在「处理中…」，用户体感就是「登录不了」（2026-08-20 用户反馈复现）。
// 25s 足够覆盖绝大多数冷启动，超时后前端能拿到明确错误提示重试，而不是无限转圈。
var cloudHTTPClient = &http.Client{Timeout: 25 * time.Second}

// cloudAuthBase 返回 ResceneCloud 基址，未配置则回退默认云端，保证开箱即连。
func cloudAuthBase() string {
	u := os.Getenv("RESCENE_CLOUD_URL")
	if u == "" {
		u = "https://rescenecloud.onrender.com"
	}
	return strings.TrimRight(u, "/")
}

// WarmCloudAuth 启动时后台预热 ResceneCloud（打一发 /healthz），不阻塞启动。
// 目的：把 Render 免费实例的冷启动开销提前到「应用启动」而不是「用户点登录」，
// 减少用户第一次登录/注册时撞上冷启动超时的概率。失败静默——不影响正常代理逻辑，
// 真正的登录请求仍走 cloudHTTPClient 的 25s 超时兜底。
func WarmCloudAuth() {
	go func() {
		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Get(cloudAuthBase() + "/healthz")
		if err != nil {
			log.Printf("⚠️ ResceneCloud 预热失败（不影响启动，登录时会重试）: %v", err)
			return
		}
		resp.Body.Close()
	}()
}

// proxyToCloud 把当前请求（方法/查询/body/特定头）转发到 ResceneCloud 的 targetPath，
// 并把响应原样写回。用于 /api/login。
func proxyToCloud(c *gin.Context, targetPath string) {
	proxyToCloudOpt(c, targetPath, false)
}

// proxyToCloudAuth 同 proxyToCloud，但额外透传 Authorization 头
// （供需要携带用户 JWT 的端点使用，如 /api/auth/uid/bind）。
func proxyToCloudAuth(c *gin.Context, targetPath string) {
	proxyToCloudOpt(c, targetPath, true)
}

func proxyToCloudOpt(c *gin.Context, targetPath string, forwardAuth bool) {
	target := cloudAuthBase() + targetPath

	var body io.Reader
	if c.Request.Body != nil {
		if b, err := io.ReadAll(c.Request.Body); err == nil {
			body = strings.NewReader(string(b))
		}
	}

	req, err := http.NewRequest(c.Request.Method, target, body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "构造鉴权请求失败: " + err.Error()})
		return
	}
	// 透传 Content-Type；Authorization 仅在 forwardAuth 时转发（登录/OAuth 不需要）
	if ct := c.GetHeader("Content-Type"); ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	if forwardAuth {
		if auth := c.GetHeader("Authorization"); auth != "" {
			req.Header.Set("Authorization", auth)
		}
	}

	resp, err := cloudHTTPClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": cloudErrorMessage(err)})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
}

// cloudErrorMessage 把连接 ResceneCloud 失败的原因转成用户可读的提示。
// 单独识别超时：Render 免费实例冷启动慢，超时比普通网络错误更常见，
// 给出「稍后重试」而不是让用户误以为账号密码错了或网络彻底不通。
func cloudErrorMessage(err error) string {
	if ne, ok := err.(interface{ Timeout() bool }); ok && ne.Timeout() {
		return "连接 ResceneCloud 超时（云端可能正在冷启动），请稍后重试"
	}
	return "连接 ResceneCloud 失败: " + err.Error()
}

// CloudLoginProxy 转发到 ResceneCloud 的账号登录（用户名+密码 → JWT）。
// 双模式：{username,password}=账号登录，{password}=管理员密码登录。
func CloudLoginProxy(c *gin.Context) {
	proxyToCloud(c, "/api/login")
}

// CloudRegisterProxy 转发到 ResceneCloud 的开放注册（用户名+密码 → 建号 + JWT）。
func CloudRegisterProxy(c *gin.Context) {
	proxyToCloud(c, "/api/auth/register")
}

// CloudUidProxy 游客 UID 分发：转发到 ResceneCloud 统一验证并签发（前端不可伪造）。
// 同一 device_id 恒定返回同一 UID；换设备/清缓存 = 新游客号，登录 bind 后永久保留。
func CloudUidProxy(c *gin.Context) {
	proxyToCloud(c, "/api/auth/uid")
}

// CloudMeProxy 把 token 校验直接代理到 ResceneCloud 云端（透传 Authorization 头）。
// 设计要点：re0 开源侧不持有任何鉴权密钥，验签完全由云端用它的 JWT_SECRET 完成
// （与签发同源），避免「本地硬编码密钥 → 开源泄露 → 可被伪造任意 token」的漏洞。
// 云端 /api/auth/me 回传的字段（authenticated/role/uid/is_vip…）原样转发给前端，
// 行为与旧版本地验签一致，但密钥零落本地。
func CloudMeProxy(c *gin.Context) {
	proxyToCloudAuth(c, "/api/auth/me")
}

// CloudUidBindProxy 登录后 UID 绑定：把游客 UID 升级为正式账号（需透传用户 JWT）。
func CloudUidBindProxy(c *gin.Context) {
	proxyToCloudAuth(c, "/api/auth/uid/bind")
}

// CloudBindEmailSendCodeProxy 账号邮箱补绑/改绑 - 发验证码（需登录 token，透传 Authorization）。
func CloudBindEmailSendCodeProxy(c *gin.Context) {
	proxyToCloudAuth(c, "/api/auth/bind-send-code")
}

// CloudBindEmailProxy 账号邮箱补绑/改绑 - 校验验证码后绑定（需登录 token，透传 Authorization）。
func CloudBindEmailProxy(c *gin.Context) {
	proxyToCloudAuth(c, "/api/auth/bind-email")
}

// AuthMe 本地验 JWT（复用 middleware.AuthRequired 透传的 claims），回传 is_vip。
// 这是薄中间件：不信任网络，只信本地用 JWT_SECRET 验过的 token。
func AuthMe(c *gin.Context) {
	role, _ := c.Get("role")
	openid, _ := c.Get("openid")
	login, _ := c.Get("login")
	name, _ := c.Get("name")
	avatar, _ := c.Get("avatar")
	uid, _ := c.Get("uid")
	isVip, _ := c.Get("is_vip")
	c.JSON(http.StatusOK, gin.H{
		"authenticated": true,
		"role":          role,
		"openid":        openid,
		"login":         login,
		"name":          name,
		"avatar":        avatar,
		"uid":           uid,
		"is_vip":        isVip,
	})
}

// ── 亲密度（无上限互动值）薄代理 ──
//
// 亲密度随 UID 账号存 ResceneCloud（云端权威、跨设备保留）。re0 只做透传，
// 并在成功响应时把最新值同步到本地缓存 memory/intimacy.md —— context_provider
// 每轮从缓存注入系统提示词，离线也能用最近一次的值。

// CloudIntimacyIncProxy 亲密度 +1 上报：转发到 ResceneCloud 的 /api/auth/intimacy/inc。
func CloudIntimacyIncProxy(c *gin.Context) {
	proxyIntimacyToCloud(c, "/api/auth/intimacy/inc")
}

// CloudIntimacyGetProxy 亲密度查询：转发到 ResceneCloud 的 /api/auth/intimacy（带 uid 查询参数）。
func CloudIntimacyGetProxy(c *gin.Context) {
	q := c.Request.URL.Query()
	proxyIntimacyToCloud(c, "/api/auth/intimacy?"+q.Encode())
}

// proxyIntimacyToCloud 转发亲密度请求到云端，成功后解析 {uid, intimacy} 写本地缓存。
func proxyIntimacyToCloud(c *gin.Context, targetPath string) {
	target := cloudAuthBase() + targetPath

	var body io.Reader
	if c.Request.Body != nil {
		if b, err := io.ReadAll(c.Request.Body); err == nil {
			body = strings.NewReader(string(b))
		}
	}

	req, err := http.NewRequest(c.Request.Method, target, body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "构造亲密度请求失败: " + err.Error()})
		return
	}
	if ct := c.GetHeader("Content-Type"); ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	// 透传 Authorization：游客/登录用户 JWT 云端验签，读取端点要求「查自己」
	if auth := c.GetHeader("Authorization"); auth != "" {
		req.Header.Set("Authorization", auth)
	}

	resp, err := cloudHTTPClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": cloudErrorMessage(err)})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)

	// 云端权威值 → 本地缓存（供每轮注入；失败静默不影响响应）
	if resp.StatusCode == http.StatusOK {
		var parsed struct {
			UID      int64 `json:"uid"`
			Intimacy int64 `json:"intimacy"`
		}
		if json.Unmarshal(respBody, &parsed) == nil && parsed.UID > 0 {
			memorydir.WriteIntimacy(parsed.UID, parsed.Intimacy)
			TriggerPersonalityOnIntimacyLevelUp()
		}
	}
}

// CloudStatsIncProxy 主页统计增量上报：转发到 ResceneCloud 的 /api/stats/inc。
// uid 由前端/后端随 body 携带，re0 只透传（不需要知道 uid）。带 Authorization（云端验签）。
func CloudStatsIncProxy(c *gin.Context) {
	proxyToCloudAuth(c, "/api/stats/inc")
}

// CloudStatsGetProxy 主页统计查询：转发到 ResceneCloud 的 /api/stats（带 uid 查询参数）。
// 带 Authorization：云端要求 JWT uid == 请求 uid（只能查自己）。
func CloudStatsGetProxy(c *gin.Context) {
	q := c.Request.URL.Query()
	proxyToCloudAuth(c, "/api/stats?"+q.Encode())
}

// CloudNotificationProxy 透传 /api/notifications/* 到 ResceneCloud（带 Authorization）。
func CloudNotificationProxy(c *gin.Context) {
	proxyToCloudAuth(c, c.Request.URL.Path)
}

// CloudDHSAuditsProxy 透传 /api/dhs/audits 到 ResceneCloud（带 Authorization）。
// GET 公开读（社区共享可信标签），POST 上报审计结果（云端验 JWT，游客 401 前端静默）。
func CloudDHSAuditsProxy(c *gin.Context) {
	proxyToCloudAuth(c, "/api/dhs/audits")
}

// CloudDHSFavoritesProxy 透传 /api/dhs/favorites* 到 ResceneCloud（带 Authorization）。
// 爱心收藏按 uid 存云端：GET 拉我的收藏，POST /toggle 收藏/取消（云端验 JWT）。
func CloudDHSFavoritesProxy(c *gin.Context) {
	proxyToCloudAuth(c, c.Request.URL.Path)
}

// CloudAuthConfig 把 ResceneCloud 基址暴露给前端，供其发起账号登录（/api/login）。
func CloudAuthConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"cloud_url": cloudAuthBase(),
	})
}
