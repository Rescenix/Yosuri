package handler

// cloud_auth.go —— re0 侧的鉴权薄中间件。
//
// 鉴权逻辑已全部收口到独立私有服务 ResceneCloud（C:\Pro2026\ResceneCloud，对应私有仓
// github.com/Rescenix/ResceneCloud）。本文件只做三件事，不持有任何密钥 / OAuth 逻辑：
//   1. 把 /api/login 的流量反向代理到 RESCENE_CLOUD_URL
//   2. /api/auth/me 用本地 middleware.AuthRequired() 验 JWT（与 ResceneCloud 共用
//      JWT_SECRET），回传 is_vip 等——无需信任网络即可判定会员
//   3. 暴露 RESCENE_CLOUD_URL 给前端（/api/auth/cloud-config），供其直接发起 GitHub 登录
//
// 这样开源的 re0 不含任何付费/鉴权密钥，商业闭环留在私有 ResceneCloud。

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"

	"backend/internal/memorydir"

	"github.com/gin-gonic/gin"
)

// cloudAuthBase 返回 ResceneCloud 基址，未配置则回退默认云端，保证开箱即连。
func cloudAuthBase() string {
	u := os.Getenv("RESCENE_CLOUD_URL")
	if u == "" {
		u = "https://rescenecloud.onrender.com"
	}
	return strings.TrimRight(u, "/")
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

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "连接 ResceneCloud 失败: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
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

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "连接 ResceneCloud 失败: " + err.Error()})
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
		}
	}
}

// CloudStatsIncProxy 主页统计增量上报：转发到 ResceneCloud 的 /api/stats/inc。
// uid 由前端/后端随 body 携带，re0 只透传（不需要知道 uid）。
func CloudStatsIncProxy(c *gin.Context) {
	proxyToCloud(c, "/api/stats/inc")
}

// CloudStatsGetProxy 主页统计查询：转发到 ResceneCloud 的 /api/stats（带 uid 查询参数）。
func CloudStatsGetProxy(c *gin.Context) {
	q := c.Request.URL.Query()
	proxyToCloud(c, "/api/stats?"+q.Encode())
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
