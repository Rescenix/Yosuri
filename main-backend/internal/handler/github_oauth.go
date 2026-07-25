package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// ===== GitHub OAuth 配置 =====
// 在 .env 配置以下三项后生效：
//   GITHUB_CLIENT_ID      —— GitHub OAuth App 的 Client ID
//   GITHUB_CLIENT_SECRET  —— GitHub OAuth App 的 Client Secret
//   GITHUB_CALLBACK_URL   —— 回调地址，必须填 http(s)://<host>/api/auth/github/callback
// 未配置时接口返回友好提示，不影响其它功能（与现有 DEV_MODE 后门共存）。

type githubConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

var githubCfg *githubConfig

// InitGitHubOAuth 在 main 里 godotenv.Load() 之后调用；缺任意一项则跳过（githubCfg 为 nil）。
func InitGitHubOAuth() {
	id := os.Getenv("GITHUB_CLIENT_ID")
	sec := os.Getenv("GITHUB_CLIENT_SECRET")
	cb := os.Getenv("GITHUB_CALLBACK_URL")
	if id == "" || sec == "" || cb == "" {
		log.Println("ℹ️ GitHub OAuth 未配置（GITHUB_CLIENT_ID / GITHUB_CLIENT_SECRET / GITHUB_CALLBACK_URL 缺失），GitHub 登录接口将返回配置提示")
		return
	}
	githubCfg = &githubConfig{ClientID: id, ClientSecret: sec, RedirectURL: cb}
	log.Println("✅ GitHub OAuth 已启用，回调地址:", cb)
}

func getGitHubConfig() *githubConfig { return githubCfg }

// ===== state 防 CSRF（单实例内存存储，带 TTL）=====
type ghStateStore struct {
	mu sync.Mutex
	m  map[string]time.Time
}

func (s *ghStateStore) put(state string, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[state] = time.Now().Add(ttl)
}

func (s *ghStateStore) take(state string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.m[state]
	if !ok {
		return false
	}
	delete(s.m, state)
	return time.Now().Before(exp)
}

var githubStateStore = &ghStateStore{m: make(map[string]time.Time)}

func randState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ===== GitHub 数据结构 =====
type githubTokenResp struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	Error       string `json:"error"`
}

type githubUser struct {
	Login     string `json:"login"`
	ID        int64  `json:"id"`
	AvatarURL string `json:"avatar_url"`
	Name      string `json:"name"`
	Email     string `json:"email"`
}

// ===== 路由处理 =====

// GitHubLogin 发起授权：302 跳到 GitHub 授权页（带 state 防 CSRF）。
func GitHubLogin(c *gin.Context) {
	cfg := getGitHubConfig()
	if cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "GitHub OAuth 未配置，请在 .env 设置 GITHUB_CLIENT_ID / GITHUB_CLIENT_SECRET / GITHUB_CALLBACK_URL",
		})
		return
	}
	state := randState()
	githubStateStore.put(state, 5*time.Minute)

	q := url.Values{}
	q.Set("client_id", cfg.ClientID)
	q.Set("redirect_uri", cfg.RedirectURL)
	q.Set("scope", "read:user user:email")
	q.Set("state", state)
	q.Set("allow_signup", "true")

	authURL := "https://github.com/login/oauth/authorize?" + q.Encode()
	c.Redirect(http.StatusTemporaryRedirect, authURL)
}

// GitHubCallback GitHub 授权后回到的地址：用 code 换 token → 取用户信息 → 签 JWT → 302 回前端。
func GitHubCallback(c *gin.Context) {
	if errParam := c.Query("error"); errParam != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "GitHub 授权被用户拒绝或失败: " + errParam})
		return
	}
	state := c.Query("state")
	code := c.Query("code")
	if state == "" || code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 state 或 code 参数"})
		return
	}
	if !githubStateStore.take(state) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "state 无效或已过期（CSRF 防护拦截）"})
		return
	}

	cfg := getGitHubConfig()
	accessToken, err := exchangeGitHubToken(cfg.ClientID, cfg.ClientSecret, cfg.RedirectURL, code)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "GitHub 换取 access_token 失败: " + err.Error()})
		return
	}

	user, err := fetchGitHubUser(accessToken)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "获取 GitHub 用户信息失败: " + err.Error()})
		return
	}

	token, err := signUserJWT(user.Login, fmt.Sprintf("%d", user.ID), user.Name, user.AvatarURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "签发登录凭证失败: " + err.Error()})
		return
	}

	frontend := os.Getenv("FRONTEND_URL")
	if frontend == "" {
		frontend = "http://localhost:4322"
	}
	// 把 JWT 通过前端 URL 带回，前端从 ?token= 解析后存 localStorage
	c.Redirect(http.StatusTemporaryRedirect, frontend+"/?token="+url.QueryEscape(token))
}

// ===== 内部工具 =====

func exchangeGitHubToken(clientID, clientSecret, redirectURL, code string) (string, error) {
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", redirectURL)

	req, err := http.NewRequest(http.MethodPost, "https://github.com/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tr githubTokenResp
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", fmt.Errorf("解析 GitHub token 响应失败: %s", string(body))
	}
	if tr.Error != "" {
		return "", fmt.Errorf("GitHub 返回错误: %s", tr.Error)
	}
	if tr.AccessToken == "" {
		return "", fmt.Errorf("GitHub 未返回 access_token: %s", string(body))
	}
	return tr.AccessToken, nil
}

func fetchGitHubUser(token string) (*githubUser, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API %d: %s", resp.StatusCode, string(body))
	}
	var u githubUser
	if err := json.Unmarshal(body, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// signUserJWT 签发普通用户 JWT，openid 落地为 GitHub 登录名（与 model_config_handler 的 userKey 规划一致）。
func signUserJWT(openid, sub, name, avatar string) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "", fmt.Errorf("JWT_SECRET 未配置")
	}
	claims := jwt.MapClaims{
		"openid": openid,
		"sub":    sub,
		"name":   name,
		"avatar": avatar,
		"role":   "user",
		"exp":    time.Now().Add(30 * 24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
