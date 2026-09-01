package handler

import (
	"archive/zip"
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
	updateRepoName  = "Yosuri"
	// 官网 update.json 优先（国内可达的 Cloudflare CDN），GitHub API 兜底
	siteUpdateURL  = "https://yosuri.com/update.json"
	// 2026-08-28 用户定稿：官网一更新要尽快触发后台自动下载 → 缓存 TTL 30min→60s。
	// update.json 是静态小 JSON、Cloudflare Pages 项目级缓存已 Disabled，压力可忽略；
	// GitHub 兜底未认证 API 限 60 次/小时/IP——官网可达时走官网不触发，官网挂了兜底超限也只是静默。
	updateCacheTTL = 60 * time.Second
	// 官网目前只发布 portable zip。旧安装器地址已经下线，不能再把缺失的
	// download_url_exe 静默回退到 setup.exe，否则真实用户会稳定得到 HTTP 404。
	updateDownloadURL = "https://download.shanca.me/Rescene-windows-amd64-setup.exe"
	updateHotPatchURL = "https://download.shanca.me/Rescene-windows-amd64-portable.zip"
)

// githubRelease 是 GitHub /releases/latest 响应里用到的字段子集。
// 也兼容官网 update.json 的相同字段，所以客户端可以优先从国内可达的 update.json 获取。
type githubRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
	DownloadURL string `json:"download_url"`     // 官网 JSON 提供，GitHub 无此字段
	DownloadExe string `json:"download_url_exe"` // 热补丁通道：新版 rescene.exe 直链（官网 JSON 提供）
	DownloadZip string `json:"download_url_zip"` // 热补丁通道的新字段名；兼容旧 download_url_exe
	Changelog   []changelogEntry `json:"changelog"` // 官网 update.json 提供：按版本分列的更新日志
}

// changelogEntry 是官网 update.json 的 changelog 数组元素。
// 用于根治「版本号与公告脱节」：发版时顶层 body 易漏改，
// 改用 changelog 中与 latest 同版本的条目生成公告，保证一致（2026-08-19）。
type changelogEntry struct {
	Version string   `json:"version"`
	Date    string   `json:"date"`
	Title   string   `json:"title"`
	Items   []string `json:"items"`
}

// changelogFor 在 changelog 中找与给定版本完全匹配的条目。
func (r *githubRelease) changelogFor(version string) *changelogEntry {
	for i := range r.Changelog {
		if r.Changelog[i].Version == version {
			return &r.Changelog[i]
		}
	}
	return nil
}

// toMarkdown 把 changelog 条目渲染成与历史 body 风格一致的 Markdown 公告。
func (e *changelogEntry) toMarkdown() string {
	if e == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("## " + e.Version + "\n\n")
	if e.Title != "" {
		b.WriteString(e.Title + "\n\n")
	}
	for _, it := range e.Items {
		b.WriteString("- " + it + "\n")
	}
	return b.String()
}

// sanitizeReleaseNotes 去掉发布说明里面向「去官网手动下载」的引导语：
// 应用内是自动下载安装包，这类提示会误导用户以为要自己去下载（2026-08-19）。
// 只在回退到顶层 body（GitHub release 风格）时可能含此类行；changelog 生成的公告本就干净。
func sanitizeReleaseNotes(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		trim := strings.TrimSpace(ln)
		if strings.Contains(trim, "安装包统一放官网下载") || strings.Contains(trim, "yosuri.com/download") {
			continue
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}

// updateInfo 是 /api/update/check 的响应体。
type updateInfo struct {
	HasUpdate      bool   `json:"has_update"`
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	ReleaseName    string `json:"release_name"`
	ReleaseNotes   string `json:"release_notes"`
	ReleaseURL     string `json:"release_url"`
	DownloadURL    string `json:"download_url"`               // 官网安装器直链
	DownloadExe    string `json:"download_url_exe,omitempty"` // 热补丁通道：新 exe 直链
	HotPatch       bool   `json:"hot_patch"`                  // true = 本次走热补丁（直接换 exe），false = 全量安装器
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
// 数据源顺序：官网 update.json（yosuri.com，Cloudflare CDN，国内可达）
// → GitHub API 兜底（Release 基准）。两者都失败返回错误，前端静默不打扰。
func checkUpdate() (*updateInfo, error) {
	updateMu.Lock()
	defer updateMu.Unlock()
	if updateCache != nil && time.Since(updateCachedAt) < updateCacheTTL {
		return updateCache, nil
	}

	// 1) 官网 update.json（优先，国内可达）
	// 支持 RESCENE_UPDATE_URL env 覆盖（本地/内网测试更新源，2026-08-16）
	updateURL := siteUpdateURL
	if v := os.Getenv("RESCENE_UPDATE_URL"); v != "" {
		updateURL = v
	}
	rel, err := fetchRelease(updateURL)
	fromSite := err == nil
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

	// 根治「版本号与公告脱节」（2026-08-19）：发版时顶层 body/published_at 易漏改，
	// 公告优先用 changelog 中与 latest 同版本的条目生成；没有再回退 body。
	// 这样弹窗的公告永远和版本号、发布日期一致。
	releaseNotes := rel.Body
	published := rel.PublishedAt
	if entry := rel.changelogFor(latest); entry != nil {
		if md := entry.toMarkdown(); md != "" {
			releaseNotes = md
		}
		// changelog 命中版本时，其 date 是精确的发布日期（YYYY-MM-DD），
		// 始终优先于顶层 published_at（顶层是老版本残留，发版易漏改）。
		// 无 date 字段才回退顶层 published_at。
		if entry.Date != "" {
			published = entry.Date + "T00:00:00Z"
		}
	}
	// 剥离「去官网手动下载」误导语：应用内自动下载，此类提示多余且误导（2026-08-19）。
	releaseNotes = sanitizeReleaseNotes(releaseNotes)

	var downloadURL string
	if rel.DownloadURL != "" {
		downloadURL = rel.DownloadURL
	} else {
		downloadURL = updateDownloadURL
	}
	// 防空版本弹窗（2026-08-13 重大 bug）：裸构建未注入版本时 AppVersion=0.0.0-dev，
	// 会永远低于线上 release 导致每次启动都弹更新。无法判断当前版本时就不打扰用户。
	cur := AppVersion
	hasUpdate := true
	if cur == "" || cur == "0.0.0-dev" {
		cur = "未知（开发版）"
		hasUpdate = false
	} else {
		hasUpdate = compareVersions(AppVersion, latestNum)
	}
	hotPatchURL := resolveHotPatchURL(rel, fromSite)
	// hot_patch = 是否存在可用的 zip 热补丁通道（2026-08-28 用户定稿：不再区分
	// 预发布/正式版，正式版同样走 zip 热补丁自动应用，旧的「正式版→一键安装」语义删除）。
	hotPatch := hotPatchURL != ""
	info := &updateInfo{
		HasUpdate:      hasUpdate,
		CurrentVersion: cur,
		LatestVersion:  latest,
		ReleaseName:    rel.Name,
		ReleaseNotes:   releaseNotes,
		ReleaseURL:     rel.HTMLURL,
		DownloadURL:    downloadURL, // 官网安装器直链，不经 GitHub
		DownloadExe:    hotPatchURL,
		HotPatch:       hotPatch,
		PublishedAt:    published,
	}

	updateCache, updateCachedAt = info, time.Now()
	return info, nil
}

func resolveHotPatchURL(rel *githubRelease, fromSite bool) string {
	if rel.DownloadZip != "" {
		return rel.DownloadZip
	}
	if rel.DownloadExe != "" {
		return rel.DownloadExe
	}
	// 兼容已经部署但缺少下载字段的 update.json。官网 portable.zip 是稳定的
	// “当前版本”地址；GitHub 兜底不能这样猜，否则 CDN 尚未同步时可能装错版本。
	if fromSite {
		return updateHotPatchURL
	}
	return ""
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

// lastAppliedVersionFileName 是「已更新到 vX」一次性标记文件名（updates 目录内）。
// alpha 预发布补丁启动时静默自动应用后写入；前端启动读一次显示升级完成提示即删。
const lastAppliedVersionFileName = "last-applied.txt"

// writeLastAppliedVersion 从补丁 exe 二进制里提取新版本号，写入一次性标记。
// 版本串取自 ldflags 注入的 UTF-8 版本（versionRe 不匹配 UTF-16 版本资源）。
// 提取失败静默跳过（提示非关键路径）。
func writeLastAppliedVersion(newExe, localDir string) error {
	data, err := os.ReadFile(newExe)
	if err != nil {
		return err
	}
	ms := versionRe.FindAll(data, -1)
	if len(ms) == 0 {
		return nil
	}
	best := string(ms[0])
	for _, m := range ms[1:] {
		if len(m) > len(best) {
			best = string(m)
		}
	}
	return os.WriteFile(filepath.Join(localDir, lastAppliedVersionFileName), []byte(best), 0o600)
}

// HandleLastAppliedUpdate 返回上次静默自动应用的新版本号（一次性：读完即删）。
// 前端启动时调用，有版本则显示「已更新到 vX」提示；无标记返回空串，前端静默。
func HandleLastAppliedUpdate(c *gin.Context) {
	localDir := filepath.Join(os.Getenv("LOCALAPPDATA"), "Rescene", "updates")
	mark := filepath.Join(localDir, lastAppliedVersionFileName)
	data, err := os.ReadFile(mark)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": true, "version": ""})
		return
	}
	_ = os.Remove(mark) // 一次性：读完即删，避免每次启动重复提示
	c.JSON(http.StatusOK, gin.H{"ok": true, "version": strings.TrimSpace(string(data))})
}

// HandleClearPendingHotPatch 删除待应用的热补丁 exe（用户「跳过此版本」时调用，
// 防止下次启动被自动应用；「稍后再说」不删——那是下次启动更新的入口）。
func HandleClearPendingHotPatch(c *gin.Context) {
	localDir := filepath.Join(os.Getenv("LOCALAPPDATA"), "Rescene", "updates")
	newExe := filepath.Join(localDir, updateHotPatchFileName)
	if err := os.Remove(newExe); err != nil && !os.IsNotExist(err) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
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

// ============ 自动下载安装包（后台） ============

// updateDownloadState 记录后台下载进度，供前端轮询。
type updateDownloadState struct {
	mu         sync.Mutex
	State      string  `json:"state"` // idle | downloading | done | error
	DoneBytes  int64   `json:"done_bytes"`
	TotalBytes int64   `json:"total_bytes"`
	Percent    float64 `json:"percent"`
	Path       string  `json:"path"`
	ErrMsg     string  `json:"error"`
	Applying   bool    `json:"-"`
}

var updateDL = &updateDownloadState{State: "idle"}

// updateSetupFileName 安装包文件名（与官网下载页一致）。
const updateSetupFileName = "Rescene-windows-amd64-setup.exe"

// updateHotPatchFileName 热补丁通道下载的新版 exe 文件名（直接替换运行中 exe）。
const updateHotPatchFileName = "rescene-new.exe"

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

	// 更新通道已经统一为 portable zip 热更新。清单不可用或没有 ZIP 时明确报错，
	// 绝不能再静默走已经下线的 setup.exe 地址。
	info, err := checkUpdate()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"ok": false, "error": "无法读取更新清单：" + err.Error()})
		return
	}
	exeURL := info.DownloadExe
	if exeURL == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"ok": false, "error": "更新清单缺少 ZIP 下载地址"})
		return
	}

	// 先同步检查本地是否已有安装包（上次启动已下载完 → 本次直接弹一键安装）。
	// updateDL 是内存状态，重启后丢失，必须落到磁盘判断。
	localDir := filepath.Join(os.Getenv("LOCALAPPDATA"), "Rescene", "updates")
	dest := filepath.Join(localDir, updateHotPatchFileName)
	ready := isLikelyWindowsExecutable(dest)
	if ready {
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
	updateDL.Path = ""
	updateDL.mu.Unlock()

	go func() {
		// 热补丁：下载官网 zip → 解压提取 exe 存为待应用补丁。
		// version 用于 zip 缓存隔离（rescene-update-<version>.zip，2026-08-16）
		targetVersion := ""
		if info != nil {
			targetVersion = info.LatestVersion
		}
		err := downloadHotPatchZip(exeURL, dest, targetVersion)
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

// downloadHotPatchZip 下载官网 zip（内含 rescene.exe + setup.exe）→ 解压提取 rescene.exe
// → 存为待应用热补丁（rescene-new.exe）→ 删除 zip。更新包只传 zip 一个文件（2026-08-13）。
//
// ⚠️ zip 缓存按目标版本隔离（2026-08-16 修复）：
// 旧实现用固定文件名 rescene-update.zip，本地残留（上次中断/解压前崩溃）会永远复用旧 zip
// → 线上发了新版本但解压出的永远是旧 exe → 「更新了还是旧版本」死循环（mock 实测复现：
// 08-14 的 alpha.3 zip 被 08-16 的 alpha.5 更新流程复用）。现在 zip 名带版本
// （rescene-update-<version>.zip），版本变了必然重新下载；同版本重试才复用本地缓存。
func downloadHotPatchZip(zipURL, dest, version string) error {
	localDir := filepath.Dir(dest)
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		return err
	}
	// 已下载完成 → 复用
	if isLikelyWindowsExecutable(dest) {
		updateDL.mu.Lock()
		updateDL.Path = dest
		updateDL.mu.Unlock()
		return nil
	}
	zipName := "rescene-update.zip"
	if version != "" {
		zipName = fmt.Sprintf("rescene-update-%s.zip", sanitizeVersionForFilename(version))
	}
	zipPath := filepath.Join(localDir, zipName)
	fi, err := os.Stat(zipPath)
	if err != nil || fi.Size() < 1024*1024 {
		// 下载 zip（带进度）
		client := &http.Client{Timeout: 10 * time.Minute}
		resp, err := client.Get(zipURL)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("下载更新包失败：HTTP %d", resp.StatusCode)
		}
		out, err := os.Create(zipPath + ".part")
		if err != nil {
			return err
		}
		total := resp.ContentLength
		buf := make([]byte, 64*1024)
		var done int64
		for {
			n, rerr := resp.Body.Read(buf)
			if n > 0 {
				if _, werr := out.Write(buf[:n]); werr != nil {
					out.Close()
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
				out.Close()
				return rerr
			}
		}
		if err := out.Close(); err != nil {
			return err
		}
		// Windows 的 Rename 不会覆盖已有文件；先清掉可能残留的小包。
		if err := os.Remove(zipPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := os.Rename(zipPath+".part", zipPath); err != nil {
			return err
		}
	}
	// 解压提取 rescene.exe
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		_ = os.Remove(zipPath) // 损坏包不能在下一次重试时继续复用
		return err
	}
	defer zr.Close()
	found := false
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || !strings.EqualFold(filepath.Base(f.Name), "rescene.exe") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		extractPath := dest + ".part"
		out, err := os.Create(extractPath)
		if err != nil {
			rc.Close()
			return err
		}
		_, cerr := io.Copy(out, rc)
		rc.Close()
		closeErr := out.Close()
		if cerr != nil {
			_ = os.Remove(extractPath)
			return cerr
		}
		if closeErr != nil {
			_ = os.Remove(extractPath)
			return closeErr
		}
		if !isLikelyWindowsExecutable(extractPath) {
			_ = os.Remove(extractPath)
			return fmt.Errorf("更新包中的 rescene.exe 无效")
		}
		if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
			_ = os.Remove(extractPath)
			return err
		}
		if err := os.Rename(extractPath, dest); err != nil {
			_ = os.Remove(extractPath)
			return err
		}
		found = true
		break
	}
	os.Remove(zipPath)
	if !found {
		return fmt.Errorf("更新包 zip 里没有 rescene.exe")
	}
	updateDL.mu.Lock()
	updateDL.Path = dest
	updateDL.mu.Unlock()
	return nil
}

// sanitizeVersionForFilename 把版本号转成安全的文件名片段（去空格/斜杠等非法字符）。
func sanitizeVersionForFilename(v string) string {
	re := regexp.MustCompile(`[^A-Za-z0-9._-]+`)
	return re.ReplaceAllString(v, "_")
}

// isPrereleaseVersion 判断版本串是否为预发布（semver 预发布标识：-alpha / -beta / -rc / -dev 等）。
// 开发版 0.0.0-dev（未注入版本）按预发布处理（测试通道）。正式版（v0.1.2 等无后缀）返回 false。
func isPrereleaseVersion(v string) bool {
	if v == "" || v == "0.0.0-dev" {
		return true
	}
	return regexp.MustCompile(`-[A-Za-z]`).MatchString(v)
}

func isLikelyWindowsExecutable(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || fi.Size() < 1024*1024 {
		return false
	}
	var magic [2]byte
	_, err = io.ReadFull(f, magic[:])
	return err == nil && magic == [2]byte{'M', 'Z'}
}

// HandleUpdateDownloadStatus 返回下载进度。
// idle 时检查磁盘：后台已自动下载的补丁（重启后内存态丢失）→ 报 done，
// 让版本 tab / 弹窗能识别「后台已下好」并直接提供一键安装（2026-08-16 用户定稿）。
func HandleUpdateDownloadStatus(c *gin.Context) {
	updateDL.mu.Lock()
	defer updateDL.mu.Unlock()
	if updateDL.State == "idle" {
		localDir := filepath.Join(os.Getenv("LOCALAPPDATA"), "Rescene", "updates")
		dest := filepath.Join(localDir, updateHotPatchFileName)
		if isLikelyWindowsExecutable(dest) {
			updateDL.State = "done"
			updateDL.Path = dest
		}
	}
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

// downloadInstaller 流式下载更新包到本地并更新进度。
// url 为空时用默认安装器直链；dest 决定保存位置（热补丁 exe 或 setup.exe）。
func downloadInstaller(url, dest string) error {
	localDir := filepath.Dir(dest)
	if err := os.MkdirAll(localDir, 0o755); err != nil {
		return err
	}

	// 若已存在同版本安装包且非 0 字节，直接复用（跳过重复下载）
	if fi, err := os.Stat(dest); err == nil && fi.Size() > 1024*1024 {
		updateDL.mu.Lock()
		updateDL.Path = dest
		updateDL.mu.Unlock()
		return nil
	}

	dlURL := url
	if dlURL == "" {
		dlURL = updateDownloadURL
	}

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(dlURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载更新包失败：HTTP %d", resp.StatusCode)
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
//  1. cmd /c start 分离启动安装程序（独立进程，不随本进程退出）
//  2. 返回响应后延时退出本进程，释放 exe 文件锁，安装程序才能覆盖
func HandleInstallUpdate(c *gin.Context) {
	updateDL.mu.Lock()
	if updateDL.Applying {
		updateDL.mu.Unlock()
		c.JSON(http.StatusConflict, gin.H{"error": "更新正在安装，请勿重复点击"})
		return
	}
	state, path := updateDL.State, updateDL.Path
	if state == "done" && path != "" {
		updateDL.Applying = true
	}
	updateDL.mu.Unlock()
	if state != "done" || path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "安装包尚未就绪"})
		return
	}
	resetApplying := func() {
		updateDL.mu.Lock()
		updateDL.Applying = false
		updateDL.mu.Unlock()
	}
	if _, err := os.Stat(path); err != nil {
		resetApplying()
		c.JSON(http.StatusBadRequest, gin.H{"error": "安装包文件不存在"})
		return
	}

	// 热补丁通道：下载的是新版 rescene.exe（rescene-new.exe）→ 写替换脚本直接换 exe，
	// 免 NSIS 安装向导（2026-08-13）。脚本流程：等本进程退出（释放文件锁）→ copy 覆盖
	// 安装目录 exe → 删除临时 exe → 启动新版 → 删除脚本。
	if filepath.Base(path) == updateHotPatchFileName {
		exePath, err := os.Executable() // 运行中的 rescene.exe 完整路径（含安装目录）
		if err != nil {
			resetApplying()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "定位程序路径失败"})
			return
		}
		if err := startHotPatch(path, exePath); err != nil {
			resetApplying()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "启动热补丁失败：" + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "hot_patch": true, "path": path})
		// 延时退出本进程：给 HTTP 响应刷完 + 脚本完全拉起的时间，
		// 然后让出 rescene.exe 文件锁，脚本才能 copy 覆盖。
		go func() {
			time.Sleep(3 * time.Second)
			os.Exit(0)
		}()
		return
	}

	// cmd /c start "" "path"：分离启动，安装程序不继承本进程句柄
	cmd := exec.Command("cmd", "/c", "start", "", path)
	if err := cmd.Start(); err != nil {
		resetApplying()
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

// claimHotPatch 用同目录原子重命名认领补丁。安装接口和下次启动逻辑即使同时触发，
// 也只有一个进程能成功认领，避免两个批处理争抢同一个 exe。
func claimHotPatch(path string) (string, error) {
	claimed := filepath.Join(filepath.Dir(path), fmt.Sprintf("rescene-applying-%d-%d.exe", os.Getpid(), time.Now().UnixNano()))
	if err := os.Rename(path, claimed); err != nil {
		return "", err
	}
	return claimed, nil
}

// startHotPatch 认领补丁并用唯一脚本隐藏执行。启动失败时恢复标准文件名，允许重试。
func startHotPatch(path, exePath string) error {
	claimed, err := claimHotPatch(path)
	if err != nil {
		return fmt.Errorf("更新已被另一个进程接管或文件不可用: %w", err)
	}
	restore := func() { _ = os.Rename(claimed, path) }
	scriptPath := filepath.Join(filepath.Dir(path), fmt.Sprintf("apply-update-%d-%d.cmd", os.Getpid(), time.Now().UnixNano()))
	script := hotPatchBatTemplate(claimed, path, exePath, os.Getpid())
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		restore()
		return fmt.Errorf("写更新脚本: %w", err)
	}
	if err := launchUpdateScript(scriptPath); err != nil {
		_ = os.Remove(scriptPath)
		restore()
		return fmt.Errorf("运行更新脚本: %w", err)
	}
	return nil
}

// hotPatchBatTemplate 生成热补丁替换脚本：先精确等待旧进程退出，再重试覆盖。
// 只有覆盖成功才删除新 exe；失败时恢复标准补丁名并重启旧版，允许下次重试。
func hotPatchBatTemplate(newExe, pendingExe, exePath string, oldPID int) string {
	newExe = strings.ReplaceAll(newExe, "%", "%%")
	pendingExe = strings.ReplaceAll(pendingExe, "%", "%%")
	exePath = strings.ReplaceAll(exePath, "%", "%%")
	return fmt.Sprintf(`@echo off
setlocal DisableDelayedExpansion
set "OLDPID=%d"
for /l %%%%I in (1,1,120) do (
  tasklist /fi "PID eq %%OLDPID%%" /nh 2>nul | find "%%OLDPID%%" >nul
  if errorlevel 1 goto replace
  ping -n 2 127.0.0.1 >nul
)
goto failed
:replace
for /l %%%%I in (1,1,30) do (
  copy /y "%s" "%s" >nul 2>&1
  if not errorlevel 1 goto copied
  ping -n 2 127.0.0.1 >nul
)
goto failed
:failed
if exist "%s" move /y "%s" "%s" >nul 2>&1
start "" /b "%s" -no-hotpatch
del /q "%%~f0" >nul 2>&1
exit /b 1
:copied
del /q "%s"
start "" /b "%s"
del /q "%%~f0" >nul 2>&1
exit /b 0
`, oldPID, newExe, exePath, newExe, newExe, pendingExe, exePath, newExe, exePath)
}

// ApplyPendingHotPatch 启动早期调用（main 入口，wails.Run 之前）：
// 上次会话后台下载了热补丁 exe（rescene-new.exe）→ 本次启动直接自动应用：
// 写替换脚本 → 分离启动 → 等 3 秒让脚本拉起 → 退出本进程，
// 脚本 copy 覆盖 exe 后启动新版。返回 true 表示本进程即将退出，调用方应立即 return。
//
// ⚠️ 2026-08-28 用户定稿：不再发测试版，正式版也一律自动应用（免用户去 tab 点「一键安装」）。
// 旧的「正式版不自动应用、留弹窗确认」分支已删除。
func ApplyPendingHotPatch() bool {
	localDir := filepath.Join(os.Getenv("LOCALAPPDATA"), "Rescene", "updates")
	newExe := filepath.Join(localDir, updateHotPatchFileName)
	fi, err := os.Stat(newExe)
	if err != nil || fi.Size() < 1024*1024 {
		return false // 没有待应用的热补丁
	}
	// 写「已更新到 vX」一次性标记：前端启动时读取并显示升级完成提示（2026-08-18 用户定稿）
	_ = writeLastAppliedVersion(newExe, localDir)
	exePath, err := os.Executable()
	if err != nil {
		return false
	}
	if err := startHotPatch(newExe, exePath); err != nil {
		return false
	}
	time.Sleep(3 * time.Second)
	os.Exit(0)
	return true
}
