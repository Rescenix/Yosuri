package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
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

// versionRe 匹配完整 SemVer（含预发布与构建元数据）。tag 可能带代号前缀，
// 如 ginnungagap_v0.0.4，因此不能只接受整串版本号。
var versionRe = regexp.MustCompile(`(?i:v?)\d+\.\d+\.\d+(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?`)

// extractVersion 从 tag/name 字符串里提取第一个完整 SemVer；提取不到返回原串。
func extractVersion(s string) string {
	m := versionRe.FindString(s)
	if m == "" {
		return s
	}
	return strings.TrimPrefix(strings.TrimPrefix(m, "v"), "V")
}

type semVersion struct {
	core       [3]string
	prerelease []string
}

// compareVersions 返回 latest 是否严格大于 cur，遵循 SemVer 2.0.0 的优先级规则。
// 构建元数据不参与比较；正式版高于同版本的预发布版。
func compareVersions(cur, latest string) bool {
	c, cOK := parseSemVersion(cur)
	l, lOK := parseSemVersion(latest)
	if !cOK || !lOK {
		return false
	}
	return compareSemVersions(l, c) > 0
}

func parseSemVersion(v string) (semVersion, bool) {
	v = strings.TrimPrefix(strings.TrimPrefix(v, "v"), "V")
	if v == "" {
		return semVersion{}, false
	}

	precedence := v
	if i := strings.IndexByte(v, '+'); i >= 0 {
		if !validIdentifiers(v[i+1:], true) {
			return semVersion{}, false
		}
		precedence = v[:i]
	}

	var prerelease []string
	if i := strings.IndexByte(precedence, '-'); i >= 0 {
		pre := precedence[i+1:]
		if !validIdentifiers(pre, false) {
			return semVersion{}, false
		}
		prerelease = strings.Split(pre, ".")
		precedence = precedence[:i]
	}

	parts := strings.Split(precedence, ".")
	if len(parts) != 3 {
		return semVersion{}, false
	}
	var parsed semVersion
	for i, part := range parts {
		if !validNumericIdentifier(part) {
			return semVersion{}, false
		}
		parsed.core[i] = part
	}
	parsed.prerelease = prerelease
	return parsed, true
}

func validIdentifiers(s string, build bool) bool {
	if s == "" {
		return false
	}
	for _, identifier := range strings.Split(s, ".") {
		if identifier == "" {
			return false
		}
		numeric := true
		for _, r := range identifier {
			if !((r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '-') {
				return false
			}
			if r < '0' || r > '9' {
				numeric = false
			}
		}
		if !build && numeric && len(identifier) > 1 && identifier[0] == '0' {
			return false
		}
	}
	return true
}

func validNumericIdentifier(s string) bool {
	if s == "" || (len(s) > 1 && s[0] == '0') {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func compareSemVersions(a, b semVersion) int {
	for i := 0; i < 3; i++ {
		if cmp := compareNumericIdentifier(a.core[i], b.core[i]); cmp != 0 {
			return cmp
		}
	}
	if len(a.prerelease) == 0 && len(b.prerelease) == 0 {
		return 0
	}
	if len(a.prerelease) == 0 {
		return 1
	}
	if len(b.prerelease) == 0 {
		return -1
	}

	limit := len(a.prerelease)
	if len(b.prerelease) < limit {
		limit = len(b.prerelease)
	}
	for i := 0; i < limit; i++ {
		aID, bID := a.prerelease[i], b.prerelease[i]
		aNumeric, bNumeric := isNumeric(aID), isNumeric(bID)
		if aNumeric && bNumeric {
			if cmp := compareNumericIdentifier(aID, bID); cmp != 0 {
				return cmp
			}
			continue
		}
		if aNumeric {
			return -1
		}
		if bNumeric {
			return 1
		}
		if cmp := strings.Compare(aID, bID); cmp != 0 {
			return cmp
		}
	}
	return len(a.prerelease) - len(b.prerelease)
}

func compareNumericIdentifier(a, b string) int {
	if len(a) != len(b) {
		return len(a) - len(b)
	}
	return strings.Compare(a, b)
}

func isNumeric(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}
