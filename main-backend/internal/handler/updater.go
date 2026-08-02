package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// AppVersion 是当前运行版本，打包时由构建脚本用 ldflags 注入
// （-X backend/internal/handler.AppVersion=<version>，来源 wails.json info.productVersion）。
// 本地开发/未注入时回退为 0.0.0-dev，会被任何正式 release 判定为可更新。
var AppVersion = "0.0.0-dev"

const (
	updateRepoOwner = "Rescenix"
	updateRepoName  = "ResceneAgent"
	updateCacheTTL  = time.Hour // GitHub 未认证 API 限 60 次/小时/IP，内存缓存避免每次启动都打
	// 下载走官网安装器直链（GitHub 仅作版本/更新内容基准，用户流量不引到 GitHub）
	updateDownloadURL = "https://download.shanca.me/Rescene-windows-amd64-setup.exe"
)

// githubRelease 是 GitHub /releases/latest 响应里用到的字段子集。
type githubRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
}

// updateInfo 是 /api/update/check 的响应体。
type updateInfo struct {
	HasUpdate      bool   `json:"has_update"`
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	ReleaseName    string `json:"release_name"`
	ReleaseNotes   string `json:"release_notes"`
	ReleaseURL     string `json:"release_url"`
	DownloadURL    string `json:"download_url"` // 官网安装器直链
	PublishedAt    string `json:"published_at"`
}

var (
	updateMu       sync.Mutex
	updateCache    *updateInfo
	updateCachedAt time.Time
)

// HandleCheckUpdate 检查 GitHub 最新 release 是否比当前版本新。
// 失败（离线/GitHub 限流/无 release）时返回 ok=false，前端静默不打扰。
func HandleCheckUpdate(c *gin.Context) {
	info, err := checkUpdate()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "update": info})
}

func checkUpdate() (*updateInfo, error) {
	updateMu.Lock()
	defer updateMu.Unlock()
	if updateCache != nil && time.Since(updateCachedAt) < updateCacheTTL {
		return updateCache, nil
	}

	req, err := http.NewRequest(http.MethodGet,
		fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", updateRepoOwner, updateRepoName), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ResceneAgent/"+AppVersion)
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// 仓库还没有任何 release，视为无更新
		info := &updateInfo{HasUpdate: false, CurrentVersion: AppVersion}
		updateCache, updateCachedAt = info, time.Now()
		return info, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API 返回 %d", resp.StatusCode)
	}

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}

	// 以 release 名称为准（用户发布时名称才是真正的版本线，如 v0.1.2-alpha.2；
	// tag 可能是代号格式 ginnungagap_v0.0.4，只作兜底）
	latest := rel.Name
	if latest == "" {
		latest = rel.TagName
	}
	latestNum := extractVersion(latest)
	if latestNum == latest {
		if t := extractVersion(rel.TagName); t != rel.TagName {
			latestNum, latest = t, rel.TagName
		}
	}
	info := &updateInfo{
		HasUpdate:      compareVersions(AppVersion, latestNum),
		CurrentVersion: AppVersion,
		LatestVersion:  latest,
		ReleaseName:    rel.Name,
		ReleaseNotes:   rel.Body,
		ReleaseURL:     rel.HTMLURL,
		DownloadURL:    updateDownloadURL, // 官网安装器直链，不经 GitHub
		PublishedAt:    rel.PublishedAt,
	}

	updateCache, updateCachedAt = info, time.Now()
	return info, nil
}

// HandleOpenUpdateDownload 让系统浏览器打开安装器下载地址（失败时前端回退 release 页面）。
// 走后端 exec 而非前端 window.open：Wails WebView2 里 window.open 不可靠。
func HandleOpenUpdateDownload(c *gin.Context) {
	var req struct {
		URL string `json:"url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.URL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 url"})
		return
	}
	if !strings.HasPrefix(req.URL, "https://") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法 url"})
		return
	}
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", req.URL)
	if err := cmd.Start(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "打开浏览器失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// versionRe 匹配 v 前缀或裸的 x.y.z 版本号（tag 可能带代号前缀，如 ginnungagap_v0.0.4）。
var versionRe = regexp.MustCompile(`v?\d+\.\d+\.\d+`)

// extractVersion 从 tag/name 字符串里提取第一个 x.y.z 版本号；提取不到返回原串。
func extractVersion(s string) string {
	m := versionRe.FindString(s)
	if m == "" {
		return s
	}
	return strings.TrimPrefix(m, "v")
}

// compareVersions 返回 latest 是否严格大于 cur（三段数字 semver，忽略 v 前缀与 -pre/+build 后缀）。
func compareVersions(cur, latest string) bool {
	c := versionParts(cur)
	l := versionParts(latest)
	for i := 0; i < 3; i++ {
		if l[i] > c[i] {
			return true
		}
		if l[i] < c[i] {
			return false
		}
	}
	return false
}

func versionParts(v string) [3]int {
	v = strings.TrimPrefix(strings.TrimPrefix(v, "v"), "V")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	var out [3]int
	for i := 0; i < len(parts) && i < 3; i++ {
		if n, err := strconv.Atoi(parts[i]); err == nil {
			out[i] = n
		}
	}
	return out
}
