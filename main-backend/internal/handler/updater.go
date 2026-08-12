package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
	// 官网 update.json 优先（国内可达的 Cloudflare CDN），GitHub API 兜底
	siteUpdateURL   = "https://rescene.shanca.me/update.json"
	updateCacheTTL  = 30 * time.Minute // 官网接口无认证限流，可缩短缓存；GitHub 未认证 API 限 60 次/小时/IP
	// 下载走官网安装器直链（GitHub 仅作版本/更新内容基准，用户流量不引到 GitHub）
	updateDownloadURL = "https://download.shanca.me/Rescene-windows-amd64-setup.exe"
)

// githubRelease 是 GitHub /releases/latest 响应里用到的字段子集。
// 也兼容官网 update.json 的相同字段，所以客户端可以优先从国内可达的 update.json 获取。
type githubRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
	DownloadURL string `json:"download_url"` // 官网 JSON 提供，GitHub 无此字段
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

// HandleCheckUpdate 检查最新版本（官网 update.json 优先 → GitHub 兜底）。
// 失败（离线/接口不可达/无 release）时返回 ok=false，前端静默不打扰。
func HandleCheckUpdate(c *gin.Context) {
	info, err := checkUpdate()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "update": info})
}

// checkUpdate 检查最新版本是否比当前版本新。
// 数据源顺序：官网 update.json（rescene.shanca.me，Cloudflare CDN，国内可达）
// → GitHub API 兜底（Release 基准）。两者都失败返回错误，前端静默不打扰。
func checkUpdate() (*updateInfo, error) {
	updateMu.Lock()
	defer updateMu.Unlock()
	if updateCache != nil && time.Since(updateCachedAt) < updateCacheTTL {
		return updateCache, nil
	}

	// 1) 官网 update.json（优先，国内可达）
	rel, err := fetchRelease(siteUpdateURL)
	if err != nil {
		// 2) GitHub API 兜底
		rel, err = fetchRelease(fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest",
			updateRepoOwner, updateRepoName))
		if err != nil {
			// 两处都 404（还没有 release / 文件未部署）→ 视为无更新，而不是报错
			if err == errNoRelease {
				info := &updateInfo{HasUpdate: false, CurrentVersion: AppVersion}
				updateCache, updateCachedAt = info, time.Now()
				return info, nil
			}
			return nil, err
		}
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

	var downloadURL string
	if rel.DownloadURL != "" {
		downloadURL = rel.DownloadURL
	} else {
		downloadURL = updateDownloadURL
	}
	info := &updateInfo{
		HasUpdate:      compareVersions(AppVersion, latestNum),
		CurrentVersion: AppVersion,
		LatestVersion:  latest,
		ReleaseName:    rel.Name,
		ReleaseNotes:   rel.Body,
		ReleaseURL:     rel.HTMLURL,
		DownloadURL:    downloadURL, // 官网安装器直链，不经 GitHub
		PublishedAt:    rel.PublishedAt,
	}

	updateCache, updateCachedAt = info, time.Now()
	return info, nil
}

// fetchRelease 拉取并解析版本信息 JSON（兼容 GitHub API 与官网 update.json 两种来源）。
// 404（仓库无 release / 官网无该文件）视为无更新。
func fetchRelease(url string) (*githubRelease, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
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
		return nil, errNoRelease
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("更新接口 %s 返回 %d", url, resp.StatusCode)
	}

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

// errNoRelease 表示接口正常但还没有正式 release，调用方应视为无更新而不是报错。
var errNoRelease = fmt.Errorf("no release yet")

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

// ============ 自动下载安装包（后台） ============

// updateDownloadState 记录后台下载进度，供前端轮询。
type updateDownloadState struct {
	mu        sync.Mutex
	State     string  `json:"state"` // idle | downloading | done | error
	DoneBytes int64   `json:"done_bytes"`
	TotalBytes int64  `json:"total_bytes"`
	Percent   float64 `json:"percent"`
	Path      string  `json:"path"`
	ErrMsg    string  `json:"error"`
}

var updateDL = &updateDownloadState{State: "idle"}

// updateSetupFileName 安装包文件名（与官网下载页一致）。
const updateSetupFileName = "Rescene-windows-amd64-setup.exe"

// HandleAutoDownload 触发后台下载最新安装包。
// 下载目录：%LOCALAPPDATA%\Rescene\updates\（用户可写，不必管理员权限）。
// 重复调用不重复下载：已 done 直接返回；正在下返回进行中。
func HandleAutoDownload(c *gin.Context) {
	updateDL.mu.Lock()
	if updateDL.State == "done" {
		updateDL.mu.Unlock()
		c.JSON(http.StatusOK, gin.H{"ok": true, "state": "done", "path": updateDL.Path})
		return
	}
	if updateDL.State == "downloading" {
		updateDL.mu.Unlock()
		c.JSON(http.StatusOK, gin.H{"ok": true, "state": "downloading"})
		return
	}
	updateDL.mu.Unlock()

	// 先同步检查本地是否已有安装包（上次启动已下载完 → 本次直接弹一键安装）。
	// updateDL 是内存状态，重启后丢失，必须落到磁盘判断。
	localDir := filepath.Join(os.Getenv("LOCALAPPDATA"), "Rescene", "updates")
	dest := filepath.Join(localDir, updateSetupFileName)
	if fi, err := os.Stat(dest); err == nil && fi.Size() > 1024*1024 {
		updateDL.mu.Lock()
		updateDL.State = "done"
		updateDL.Path = dest
		updateDL.mu.Unlock()
		c.JSON(http.StatusOK, gin.H{"ok": true, "state": "done", "path": dest})
		return
	}

	updateDL.mu.Lock()
	updateDL.State = "downloading"
	updateDL.DoneBytes = 0
	updateDL.TotalBytes = 0
	updateDL.Percent = 0
	updateDL.ErrMsg = ""
	updateDL.mu.Unlock()

	go func() {
		err := downloadInstaller()
		updateDL.mu.Lock()
		defer updateDL.mu.Unlock()
		if err != nil {
			updateDL.State = "error"
			updateDL.ErrMsg = err.Error()
			return
		}
		updateDL.State = "done"
	}()
	c.JSON(http.StatusOK, gin.H{"ok": true, "state": "downloading"})
}

// HandleUpdateDownloadStatus 返回下载进度。
func HandleUpdateDownloadStatus(c *gin.Context) {
	updateDL.mu.Lock()
	defer updateDL.mu.Unlock()
	c.JSON(http.StatusOK, gin.H{
		"ok":          updateDL.State != "error",
		"state":       updateDL.State,
		"done_bytes":  updateDL.DoneBytes,
		"total_bytes": updateDL.TotalBytes,
		"percent":     math.Round(updateDL.Percent*10) / 10,
		"path":        updateDL.Path,
		"error":       updateDL.ErrMsg,
	})
}

// downloadInstaller 流式下载安装包到本地并更新进度。
func downloadInstaller() error {
	localDir := filepath.Join(os.Getenv("LOCALAPPDATA"), "Rescene", "updates")
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		return err
	}
	dest := filepath.Join(localDir, updateSetupFileName)

	// 若已存在同版本安装包且非 0 字节，直接复用（跳过重复下载）
	if fi, err := os.Stat(dest); err == nil && fi.Size() > 1024*1024 {
		updateDL.mu.Lock()
		updateDL.Path = dest
		updateDL.mu.Unlock()
		return nil
	}

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(updateDownloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载安装包失败：HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(dest + ".part")
	if err != nil {
		return err
	}
	defer out.Close()

	total := resp.ContentLength
	buf := make([]byte, 64*1024)
	var done int64
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				return werr
			}
			done += int64(n)
			updateDL.mu.Lock()
			updateDL.DoneBytes = done
			updateDL.TotalBytes = total
			if total > 0 {
				updateDL.Percent = float64(done) / float64(total) * 100
			}
			updateDL.mu.Unlock()
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	if err := out.Close(); err != nil {
		return err
	}
	if err := os.Rename(dest+".part", dest); err != nil {
		return err
	}

	updateDL.mu.Lock()
	updateDL.Path = dest
	updateDL.mu.Unlock()
	return nil
}

// HandleInstallUpdate 启动已下载的安装程序（用户确认后调用）。
// 关键：安装程序要覆盖正在运行的 rescene.exe，所以必须：
//  1) cmd /c start 分离启动安装程序（独立进程，不随本进程退出）
//  2) 返回响应后延时退出本进程，释放 exe 文件锁，安装程序才能覆盖
func HandleInstallUpdate(c *gin.Context) {
	updateDL.mu.Lock()
	state, path := updateDL.State, updateDL.Path
	updateDL.mu.Unlock()
	if state != "done" || path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "安装包尚未就绪"})
		return
	}
	if _, err := os.Stat(path); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "安装包文件不存在"})
		return
	}
	// cmd /c start "" "path"：分离启动，安装程序不继承本进程句柄
	cmd := exec.Command("cmd", "/c", "start", "", path)
	if err := cmd.Start(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "启动安装程序失败：" + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "path": path})
	// 延时退出本进程：给 HTTP 响应刷完 + 安装程序完全拉起的时间，
	// 然后让出 rescene.exe 文件锁，NSIS 才能覆盖安装。
	go func() {
		time.Sleep(3 * time.Second)
		os.Exit(0)
	}()
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
