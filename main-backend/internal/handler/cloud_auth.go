package handler

// cloud_auth.go —— re0 侧的鉴权薄中间件。
//
// 鉴权逻辑已全部收口到独立私有服务 ResceneCloud（C:\Pro2026\ResceneCloud，对应私有仓
// github.com/Rescenix/ResceneCloud）。本文件只做三件事，不持有任何密钥 / OAuth 逻辑：
//   1. 把 /api/login 与 /api/auth/github/* 的流量反向代理到 RESCENE_CLOUD_URL
//   2. /api/auth/me 用本地 middleware.AuthRequired() 验 JWT（与 ResceneCloud 共用
//      JWT_SECRET），回传 is_vip 等——无需信任网络即可判定会员
//   3. 暴露 RESCENE_CLOUD_URL 给前端（/api/auth/cloud-config），供其直接发起 GitHub 登录
//
// 这样开源的 re0 不含任何付费/鉴权密钥，商业闭环留在私有 ResceneCloud。

import (
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// cloudAuthBase 返回 ResceneCloud 基址，未配置则回退本地 8088。
func cloudAuthBase() string {
	u := os.Getenv("RESCENE_CLOUD_URL")
	if u == "" {
		u = "http://localhost:8088"
	}
	return strings.TrimRight(u, "/")
}

// proxyToCloud 把当前请求（方法/查询/body/特定头）转发到 ResceneCloud 的 targetPath，
// 并把响应原样写回。用于 /api/login 与 /api/auth/github/callback。
func proxyToCloud(c *gin.Context, targetPath string) {
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
	// 透传 Content-Type；Authorization 不转发（登录/OAuth 不需要）
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
}

// CloudLoginProxy 把密码登录转发到 ResceneCloud（sha256 比对在那里完成）。
func CloudLoginProxy(c *gin.Context) {
	proxyToCloud(c, "/api/login")
}

// CloudGitHubLogin 转发到 ResceneCloud 的 GitHub 授权发起。
func CloudGitHubLogin(c *gin.Context) {
	// 302 重定向，直接让浏览器跳到 ResceneCloud 返回的 GitHub 授权页
	target := cloudAuthBase() + "/api/auth/github"
	c.Redirect(http.StatusTemporaryRedirect, target)
}

// CloudGitHubCallback 转发 GitHub 回调到 ResceneCloud；ResceneCloud 会把 JWT 通过
// ?token= 带回前端（与前端 App.vue 的回收逻辑一致），这里直接透传重定向。
func CloudGitHubCallback(c *gin.Context) {
	q := c.Request.URL.Query()
	target := cloudAuthBase() + "/api/auth/github/callback?" + q.Encode()
	c.Redirect(http.StatusTemporaryRedirect, target)
}

// AuthMe 本地验 JWT（复用 middleware.AuthRequired 透传的 claims），回传 is_vip。
// 这是薄中间件：不信任网络，只信本地用 JWT_SECRET 验过的 token。
func AuthMe(c *gin.Context) {
	role, _ := c.Get("role")
	openid, _ := c.Get("openid")
	login, _ := c.Get("login")
	name, _ := c.Get("name")
	avatar, _ := c.Get("avatar")
	isVip, _ := c.Get("is_vip")
	c.JSON(http.StatusOK, gin.H{
		"authenticated": true,
		"role":          role,
		"openid":        openid,
		"login":         login,
		"name":          name,
		"avatar":        avatar,
		"is_vip":        isVip,
	})
}

// CloudAuthConfig 把 ResceneCloud 基址暴露给前端，供其直接发起 GitHub 登录跳转。
func CloudAuthConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"cloud_url":        cloudAuthBase(),
		"github_login_url": cloudAuthBase() + "/api/auth/github",
	})
}
