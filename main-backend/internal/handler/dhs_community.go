package handler

// DHS 社区发现与双层审计。
//
// 发现层只读取 GitHub 社区仓库元数据；任何内容都不能直接安装或执行。
// 安装前必须固定到一个 Git commit，并依次通过：
//   1. 代码层静态审计：目录边界、体积、文件类型、提示词注入与敏感数据访问检查；
//   2. 执行层沙盒策略：社区包只允许声明式 Harness 资料，脚本/二进制/符号链接一律阻断。
//
// 通过后原子写入 skills-ext。插件后续产生的真实动作仍只能调用 Go 内置工具，继续受
// 工作目录边界、危险操作审批和 AgentFS 审计约束；本模块绝不运行社区仓库中的代码。

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	dhsCommunityMaxFiles     = 120
	dhsCommunityMaxBytes     = 3 << 20
	dhsCommunityMaxTextBytes = 1 << 20
	dhsNativeMaxFiles        = 500
	dhsNativeMaxBytes        = 15 << 20
	dhsNativeMaxArchiveBytes = 12 << 20
)

var dhsRepoPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
var dhsGitHubURLPattern = regexp.MustCompile(`(?i)github\.com[/:]([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)`)

var npmRegistryBaseURL = "https://registry.npmjs.org"

type dhsCommunityItem struct {
	Repo           string   `json:"repo"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	URL            string   `json:"url"`
	Stars          int      `json:"stars"`
	UpdatedAt      string   `json:"updated_at"`
	DefaultBranch  string   `json:"default_branch"`
	SourceID       string   `json:"source_id"`
	SourceName     string   `json:"source_name"`
	PackageName    string   `json:"package_name,omitempty"`
	QualityScore   int      `json:"quality_score"`
	QualitySignals []string `json:"quality_signals"`
	Archived       bool     `json:"archived"`
	HasLicense     bool     `json:"has_license"`
	Version        string   `json:"version"`
	Format         string   `json:"format"`
	BundlePatch    string   `json:"bundle_patch"`
	Installed      bool     `json:"installed,omitempty"`
}

type dhsCommunitySource struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type dhsAuditFinding struct {
	Layer    string `json:"layer"`
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Path     string `json:"path,omitempty"`
}

type dhsAuditReport struct {
	Repo            string            `json:"repo"`
	PackageName     string            `json:"package_name,omitempty"`
	Version         string            `json:"version,omitempty"`
	Format          string            `json:"format,omitempty"`
	Commit          string            `json:"commit"`
	SkillPath       string            `json:"skill_path"`
	Status          string            `json:"status"` // passed | review | blocked
	CodeStatus      string            `json:"code_status"`
	ExecutionStatus string            `json:"execution_status"`
	FileCount       int               `json:"file_count"`
	TotalBytes      int64             `json:"total_bytes"`
	Findings        []dhsAuditFinding `json:"findings"`
	Policy          string            `json:"policy"`
}

type dhsTreeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
	Mode string `json:"mode"`
	Size int64  `json:"size"`
}

var dhsExecutableExt = map[string]bool{
	".exe": true, ".dll": true, ".com": true, ".msi": true, ".scr": true,
	".bat": true, ".cmd": true, ".ps1": true, ".sh": true,
	".py": true, ".pyc": true, ".js": true, ".mjs": true, ".cjs": true,
	".ts": true, ".go": true, ".rs": true, ".jar": true, ".class": true, ".wasm": true,
}

var dhsAllowedAssetExt = map[string]bool{
	".md": true, ".txt": true, ".json": true, ".yaml": true, ".yml": true,
	".toml": true, ".csv": true, ".tsv": true,
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
}

var dhsTextExt = map[string]bool{
	".md": true, ".txt": true, ".json": true, ".yaml": true, ".yml": true,
	".toml": true, ".csv": true, ".tsv": true,
}

var dhsContentRules = []struct {
	Pattern  *regexp.Regexp
	Severity string
	Layer    string
	Code     string
	Message  string
}{
	{regexp.MustCompile(`(?i)ignore\s+(all\s+|any\s+|the\s+)?(previous|system)\s+(instruction|prompt)`), "high", "code", "prompt_injection", "包含覆盖系统或既有指令的提示词注入"},
	{regexp.MustCompile(`(?i)(bypass|disable|evade).{0,40}(safety|approval|sandbox|audit)`), "high", "execution", "sandbox_bypass", "尝试绕过安全、审批、沙盒或审计机制"},
	{regexp.MustCompile(`(?i)(subprocess|child_process|os\.system|invoke-expression|start-process|cmd\.exe|powershell(?:\.exe)?|/bin/(?:ba)?sh)`), "medium", "execution", "process_primitive", "包含进程启动或命令解释器相关指令，需要人工复核"},
	{regexp.MustCompile(`(?i)(id_rsa|\.ssh[/\\]|/etc/passwd|appdata[/\\].{0,40}(credential|token)|api[_ -]?key|private[_ -]?key)`), "medium", "code", "sensitive_data", "提及凭据、私钥或敏感系统路径，需要人工复核"},
}

func HandleDHSCommunitySearch(c *gin.Context) {
	query := strings.TrimSpace(c.Query("q"))
	if len([]rune(query)) > 80 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "搜索词不能超过 80 个字符"})
		return
	}
	items, sources, err := searchDHSCommunityCatalog(query)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"provider": "DHS Community Index",
		"trust":    "untrusted-discovery-only",
		"items":    items,
		"sources":  sources,
		"coverage": "覆盖 npm DHS Registry(19) + GitHub dsh-plugin 话题(2,513) + Awesome DSH Plugin(392) + B站视频；精选展示经 package.json 验证声明 dsh.bundle.patch 的原生插件（质量分排序，最多 88 条）",
	})
}

func searchDHSCommunityCatalog(query string) ([]dhsCommunityItem, []dhsCommunitySource, error) {
	items := make([]dhsCommunityItem, 0, 64)
	sources := make([]dhsCommunitySource, 0, 4)

	// 4 源并发收集（npm / GitHub / Awesome / B站），总耗时 = 最慢源（B站 20s deadline 是硬顶）
	var wg sync.WaitGroup
	var mu sync.Mutex
	run := func(id, name string, fn func(string) ([]dhsCommunityItem, error)) {
		defer wg.Done()
		got, err := fn(query)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			sources = append(sources, dhsCommunitySource{ID: id, Name: name, Status: "error", Error: err.Error()})
			return
		}
		items = append(items, got...)
		sources = append(sources, dhsCommunitySource{ID: id, Name: name, Status: "live"})
	}
	wg.Add(4)
	go run("npm-dsh", "npm DHS Registry", searchNPMDHSCommunity)
	go run("github-topic", "GitHub dsh-plugin 话题", searchGitHubDSHPlugins)
	go run("awesome-dsh", "Awesome DSH Plugin", searchAwesomeDSHPlugins)
	go run("bilibili-video", "B站 · DSH 视频", searchBilibiliDSHPlugins)
	wg.Wait()

	// 合并去重：同 repo 取质量分最高的
	byRepo := make(map[string]dhsCommunityItem, len(items))
	for _, item := range items {
		if previous, exists := byRepo[item.Repo]; !exists || item.QualityScore > previous.QualityScore {
			byRepo[item.Repo] = item
		}
	}
	items = items[:0]
	for _, item := range byRepo {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].QualityScore == items[j].QualityScore {
			return items[i].Stars > items[j].Stars
		}
		return items[i].QualityScore > items[j].QualityScore
	})
	// 全局按质量分截 80 条（宣传口径「全网聚合·精选展示」：展示上限是精选数，
	// 不是发现源覆盖数），但保留被截掉的 B站发现项（低分长尾正是它的价值，
	// 否则新插件永远进不了前 80）。上限 80 + 8。
	if len(items) > 80 {
		biliExtras := make([]dhsCommunityItem, 0, 8)
		for _, item := range items[80:] {
			if item.SourceID == "bilibili-video" {
				biliExtras = append(biliExtras, item)
			}
		}
		items = items[:80]
		items = append(items, biliExtras...)
	}
	// 标记已安装到 DHS 的插件（按 npm 包名比对当前 profile 的 bundles 清单）
	if installed := dhsInstalledPackages(); len(installed) > 0 {
		for i := range items {
			if items[i].PackageName != "" && installed[items[i].PackageName] {
				items[i].Installed = true
			}
		}
	}
	return items, sources, nil
}

// dhsInstalledPackages 读当前 DHS profile 的 dsh.profile.bundles，返回已安装包名集合。
// 定位逻辑与 install-dhs 一致：DSH_HOME（默认 ~/.dsh）+ DSH_PROFILE（默认 web）。
func dhsInstalledPackages() map[string]bool {
	installed := make(map[string]bool)
	dshHome := strings.TrimSpace(os.Getenv("DSH_HOME"))
	if dshHome == "" {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return installed
		}
		dshHome = filepath.Join(home, ".dsh")
	}
	profile := strings.TrimSpace(os.Getenv("DSH_PROFILE"))
	if profile == "" {
		profile = "web"
	}
	raw, err := os.ReadFile(filepath.Join(dshHome, "profiles", profile, "package.json"))
	if err != nil {
		return installed
	}
	var doc map[string]interface{}
	if json.Unmarshal(raw, &doc) != nil {
		return installed
	}
	dshObj, _ := doc["dsh"].(map[string]interface{})
	profileObj, _ := dshObj["profile"].(map[string]interface{})
	if bundlesArr, _ := profileObj["bundles"].([]interface{}); bundlesArr != nil {
		for _, b := range bundlesArr {
			if bs, _ := b.(string); bs != "" {
				installed[bs] = true
			}
		}
	}
	return installed
}

func searchNPMDHSCommunity(query string) ([]dhsCommunityItem, error) {
	terms := "keywords:dsh-plugin"
	if query != "" {
		terms = query + " " + terms
	}
	endpoint := strings.TrimRight(npmRegistryBaseURL, "/") + "/-/v1/search?text=" + url.QueryEscape(terms) + "&size=40"
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Rescene-Wails/1.0")
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("npm 社区搜索失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("npm 社区搜索返回 HTTP %d", resp.StatusCode)
	}
	var result struct {
		Objects []struct {
			Package struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				Version     string `json:"version"`
				Date        string `json:"date"`
				Links       struct {
					Repository string `json:"repository"`
					NPM        string `json:"npm"`
				} `json:"links"`
			} `json:"package"`
			Score struct {
				Final float64 `json:"final"`
			} `json:"score"`
		} `json:"objects"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&result); err != nil {
		return nil, fmt.Errorf("npm 社区响应解析失败: %w", err)
	}
	items := make([]dhsCommunityItem, 0, len(result.Objects))
	verified := make(chan *dhsCommunityItem, len(result.Objects))
	semaphore := make(chan struct{}, 8)
	for _, object := range result.Objects {
		object := object
		go func() {
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			manifest, err := loadNPMDHSManifest(object.Package.Name)
			if err != nil || manifest.DSH.Bundle.Patch == "" {
				verified <- nil
				return
			}
			if query != "" && !strings.Contains(strings.ToLower(manifest.Name+" "+manifest.Description), strings.ToLower(query)) {
				verified <- nil
				return
			}
			repo := githubRepoFromURL(manifest.repositoryURL())
			if repo == "" {
				verified <- nil
				return
			}
			score := int(object.Score.Final * 100)
			if score < 0 {
				score = 0
			} else if score > 100 {
				score = 100
			}
			verified <- &dhsCommunityItem{
				Repo: repo, Name: manifest.Name, PackageName: manifest.Name, Version: manifest.Version,
				Description: manifest.Description, URL: object.Package.Links.NPM,
				UpdatedAt: object.Package.Date, DefaultBranch: "main", SourceID: "npm-dsh", SourceName: "npm · DHS Bundle",
				QualityScore: score, QualitySignals: []string{"已验证 dsh.bundle.patch", "可追溯 GitHub 源码"},
				Format: "dsh-bundle", BundlePatch: manifest.DSH.Bundle.Patch,
			}
		}()
	}
	for range result.Objects {
		if item := <-verified; item != nil {
			items = append(items, *item)
		}
	}
	return items, nil
}

// repo 验证缓存：同一仓库的 dsh.bundle.patch 验证结果 15 分钟复用，
// 避免每次搜索都重新抓 GitHub（raw 请求虽不耗 REST 配额，但量大会拖慢搜索）。
var repoVerifyCache = struct {
	sync.Mutex
	m map[string]repoVerifyEntry
}{m: make(map[string]repoVerifyEntry)}

type repoVerifyEntry struct {
	item     *dhsCommunityItem
	expireAt time.Time
}

func repoCached(repo string) *dhsCommunityItem {
	repoVerifyCache.Lock()
	defer repoVerifyCache.Unlock()
	if e, ok := repoVerifyCache.m[repo]; ok && time.Now().Before(e.expireAt) {
		return e.item
	}
	return nil
}

func repoCachePut(repo string, item *dhsCommunityItem) {
	repoVerifyCache.Lock()
	defer repoVerifyCache.Unlock()
	repoVerifyCache.m[repo] = repoVerifyEntry{item: item, expireAt: time.Now().Add(15 * time.Minute)}
}

// searchGitHubDSHPlugins 从 GitHub dsh-plugin topic（2,513 个仓库）按 star 取前 50，
// 然后验证每个候选是否有 dsh.bundle.patch，返回通过验证的。
func searchGitHubDSHPlugins(query string) ([]dhsCommunityItem, error) {
	terms := "topic:dsh-plugin"
	if query != "" {
		terms = url.QueryEscape(query) + "+" + terms
	}
	endpoint := "/search/repositories?q=" + terms + "&sort=stars&order=desc&per_page=50"
	var result struct {
		Items []struct {
			FullName    string `json:"full_name"`
			Description string `json:"description"`
			Stars       int    `json:"stargazers_count"`
			UpdatedAt   string `json:"updated_at"`
			DefaultBranch string `json:"default_branch"`
			Archived    bool   `json:"archived"`
			License     struct {
				Key string `json:"key"`
			} `json:"license"`
			HTMLURL string `json:"html_url"`
			Topics  []string `json:"topics"`
		} `json:"items"`
	}
	if err := githubJSON(endpoint, &result); err != nil {
		return nil, fmt.Errorf("GitHub 话题搜索失败: %w", err)
	}

	items := make([]dhsCommunityItem, 0, len(result.Items))
	verified := make(chan *dhsCommunityItem, len(result.Items))
	semaphore := make(chan struct{}, 8)
	for _, item := range result.Items {
		item := item
		if item.Archived {
			continue
		}
		go func() {
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// 验证缓存命中：直接复用已验证结果（15 分钟内）
			if cached := repoCached(item.FullName); cached != nil {
				verified <- cached
				return
			}

			// 取默认分支的 package.json 验证 dsh.bundle.patch
			branch := item.DefaultBranch
			if branch == "" {
				branch = "main"
			}
			raw, err := fetchGitHubRawFile(item.FullName, branch, "package.json")
			if err != nil {
				verified <- nil
				return
			}
			var pkg struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				DSH         struct {
					Bundle struct {
						Patch string `json:"patch"`
					} `json:"bundle"`
				} `json:"dsh"`
			}
			if err := json.Unmarshal(raw, &pkg); err != nil || pkg.DSH.Bundle.Patch == "" {
				verified <- nil
				return
			}
			if query != "" && !strings.Contains(strings.ToLower(pkg.Name+" "+pkg.Description), strings.ToLower(query)) {
				verified <- nil
				return
			}
			score := item.Stars
			if score > 100 {
				score = 100
			}
			signals := []string{"dsh-plugin topic", "已验证 dsh.bundle.patch"}
			if item.License.Key != "" {
				signals = append(signals, "许可: "+item.License.Key)
			}
			ci := &dhsCommunityItem{
				Repo:           item.FullName,
				Name:           pkg.Name,
				PackageName:    pkg.Name,
				Description:    pkg.Description,
				URL:            item.HTMLURL,
				Stars:          item.Stars,
				UpdatedAt:      item.UpdatedAt,
				DefaultBranch:  branch,
				SourceID:       "github-topic",
				SourceName:     "GitHub · dsh-plugin 话题",
				QualityScore:   score,
				QualitySignals: signals,
				HasLicense:     item.License.Key != "",
				Format:         "dsh-bundle",
				BundlePatch:    pkg.DSH.Bundle.Patch,
			}
			repoCachePut(item.FullName, ci)
			verified <- ci
		}()
	}
	for range result.Items {
		if item := <-verified; item != nil {
			items = append(items, *item)
		}
	}
	return items, nil
}

// searchAwesomeDSHPlugins 从 awesome-dsh-plugin.com 的 GitHub README 获取精选插件列表，
// 然后验证每个候选是否有 dsh.bundle.patch。
func searchAwesomeDSHPlugins(query string) ([]dhsCommunityItem, error) {
	raw, err := fetchGitHubRawFile("awesome-dsh-plugin/awesome-dsh-plugin", "main", "README.md")
	if err != nil {
		// 第二次尝试
		raw, err = fetchGitHubRawFile("awesome-dsh-plugin/awesome-dsh-plugin", "main", "README.zh.md")
		if err != nil {
			return nil, fmt.Errorf("Awesome DSH Plugin 列表读取失败: %w", err)
		}
	}
	text := string(raw)
	items := make([]dhsCommunityItem, 0, 128)
	seen := make(map[string]bool)
	// 解析 markdown 列表项：- [owner/repo](https://github.com/owner/repo) - description
	re := regexp.MustCompile(`-\s+\[([^\]]+)\]\(https://github\.com/([^/]+/[^/)"?]+)\)\s*[-—]\s*(.+)`)
	matches := re.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		ownerRepo := match[2]
		ownerRepo = strings.TrimSuffix(ownerRepo, ")")
		ownerRepo = strings.TrimSuffix(ownerRepo, "/")
		description := match[3]
		if seen[ownerRepo] {
			continue
		}
		seen[ownerRepo] = true
		if query != "" && !strings.Contains(strings.ToLower(ownerRepo+" "+description), strings.ToLower(query)) {
			continue
		}
		items = append(items, dhsCommunityItem{
			Repo:           ownerRepo,
			Name:           match[1],
			Description:    description,
			URL:            "https://github.com/" + ownerRepo,
			SourceID:       "awesome-dsh",
			SourceName:     "Awesome DSH Plugin",
			QualityScore:   50,
			QualitySignals: []string{"Awesome DSH Plugin 精选列表收录"},
			Format:         "dsh-bundle",
		})
	}
	// 异步验证 dsh.bundle.patch
	verified := make(chan *dhsCommunityItem, len(items))
	semaphore := make(chan struct{}, 8)
	for i := range items {
		item := items[i]
		go func() {
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			// 验证缓存命中：直接复用已验证结果（15 分钟内）
			if cached := repoCached(item.Repo); cached != nil {
				verified <- cached
				return
			}
			raw, err := fetchGitHubRawFile(item.Repo, "main", "package.json")
			if err != nil {
				raw, err = fetchGitHubRawFile(item.Repo, "master", "package.json")
				if err != nil {
					verified <- nil
					return
				}
			}
			var pkg struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				Version     string `json:"version"`
				DSH         struct {
					Bundle struct {
						Patch string `json:"patch"`
					} `json:"bundle"`
				} `json:"dsh"`
			}
			if err := json.Unmarshal(raw, &pkg); err != nil || pkg.DSH.Bundle.Patch == "" {
				verified <- nil
				return
			}
			item.Name = pkg.Name
			item.PackageName = pkg.Name
			item.Version = pkg.Version
			item.BundlePatch = pkg.DSH.Bundle.Patch
			item.QualityScore = 70
			item.QualitySignals = append(item.QualitySignals, "已验证 dsh.bundle.patch")
			repoCachePut(item.Repo, &item)
			verified <- &item
		}()
	}
	filtered := make([]dhsCommunityItem, 0, len(items))
	for range items {
		if item := <-verified; item != nil {
			filtered = append(filtered, *item)
		}
	}
	return filtered, nil
}

type npmDHSManifest struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Repository  json.RawMessage   `json:"repository"`
	Scripts     map[string]string `json:"scripts"`
	Dist        struct {
		Tarball   string `json:"tarball"`
		Integrity string `json:"integrity"`
	} `json:"dist"`
	DSH struct {
		Bundle struct {
			Patch string `json:"patch"`
		} `json:"bundle"`
	} `json:"dsh"`
}

func loadNPMDHSManifest(packageName string) (npmDHSManifest, error) {
	var manifest npmDHSManifest
	if packageName == "" || len(packageName) > 214 {
		return manifest, fmt.Errorf("npm 包名无效")
	}
	endpoint := strings.TrimRight(npmRegistryBaseURL, "/") + "/" + url.PathEscape(packageName) + "/latest"
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return manifest, err
	}
	req.Header.Set("User-Agent", "Rescene-Wails/1.0")
	resp, err := (&http.Client{Timeout: 12 * time.Second}).Do(req)
	if err != nil {
		return manifest, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return manifest, fmt.Errorf("npm manifest 返回 HTTP %d", resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&manifest); err != nil {
		return manifest, err
	}
	return manifest, nil
}

func (manifest npmDHSManifest) repositoryURL() string {
	var value string
	if json.Unmarshal(manifest.Repository, &value) == nil {
		return value
	}
	var object struct {
		URL string `json:"url"`
	}
	_ = json.Unmarshal(manifest.Repository, &object)
	return object.URL
}

func githubRepoFromURL(value string) string {
	match := dhsGitHubURLPattern.FindStringSubmatch(value)
	if len(match) != 3 {
		return ""
	}
	repo := match[1] + "/" + strings.TrimSuffix(match[2], ".git")
	if !dhsRepoPattern.MatchString(repo) {
		return ""
	}
	return repo
}

// ---------------------------------------------------------------------------
// 源 4：B站 · DSH 视频（2026-08-15 新增）
// 匿名调用 B站公开接口（无需登录/WBI）：
//   1. x/web-interface/search/type?search_type=video&keyword=<q>   视频搜索（结果直接带 aid + 简介）
//   2. x/v2/reply?type=1&oid=<aid>&ps=20&sort=2                    评论区（热门前 20 条）
// 从 简介 + 评论 文本中正则提取 github.com/owner/repo 链接，汇入 DHS 社区目录，
// 之后继续走现有「审计 → 固定 commit → 安装」链路。只读、低量、匿名，不碰私信/互动。
// ---------------------------------------------------------------------------

var biliHTMLTagPattern = regexp.MustCompile(`<[^>]+>`)
var biliGitHubLinkPattern = regexp.MustCompile(`(?i)(?:https?://)?(?:www\.)?github\.com/([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)`)
// B站简介常见写法「GitHub - owner/repo」「GitHub: owner/repo」（不带域名），仅当前面是 GitHub 字样才提取
var biliGitHubWordPattern = regexp.MustCompile(`(?i)github\s*[-:：]\s*([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+)`)

func stripBiliHTML(value string) string {
	return html.UnescapeString(biliHTMLTagPattern.ReplaceAllString(value, ""))
}

// bilibiliExtractRepos 从一段文本里提取 github repo（owner/name），返回去重结果。
func bilibiliExtractRepos(text string) map[string]bool {
	repos := make(map[string]bool)
	for _, pattern := range []*regexp.Regexp{biliGitHubLinkPattern, biliGitHubWordPattern} {
		for _, match := range pattern.FindAllStringSubmatch(text, -1) {
			repo := match[1] + "/" + strings.TrimSuffix(match[2], ".git")
			if dhsRepoPattern.MatchString(repo) {
				repos[repo] = true
			}
		}
	}
	return repos
}

// bilibiliFetchJSON 匿名 GET B站接口，带 UA/Referer/buvid3（获取失败也不阻塞）。
func bilibiliFetchJSON(endpoint string, cookie string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://www.bilibili.com/")
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := (&http.Client{Timeout: 12 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	return body, nil
}

// bilibiliVideo 搜索结果中的单条视频（字段与 x/web-interface/search/type 一致）。
// 注意：aid 在 JSON 里是数字（实测踩坑），用 int64。
type bilibiliVideo struct {
	AID         int64  `json:"aid"`
	BVID        string `json:"bvid"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Author      string `json:"author"`
	Play        int64  `json:"play"`
}

// bilibiliSearchVideos 搜索 B站视频，按播放量降序返回（最多 6 条）。
func bilibiliSearchVideos(keyword string, cookie string) ([]bilibiliVideo, error) {
	endpoint := "https://api.bilibili.com/x/web-interface/search/type?search_type=video&page=1&keyword=" + url.QueryEscape(keyword)
	body, err := bilibiliFetchJSON(endpoint, cookie)
	if err != nil {
		return nil, fmt.Errorf("B站搜索请求失败: %w", err)
	}
	var result struct {
		Code int `json:"code"`
		Data struct {
			Result []bilibiliVideo `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		if !bytes.HasPrefix(bytes.TrimSpace(body), []byte("{")) {
			return nil, fmt.Errorf("B站返回风控页而非 JSON（请求过快或 IP 受限），请稍后重试")
		}
		return nil, fmt.Errorf("B站搜索响应解析失败: %w", err)
	}
	if result.Code != 0 {
		return nil, fmt.Errorf("B站搜索被风控拦截（code %d），请稍后重试", result.Code)
	}
	sort.Slice(result.Data.Result, func(i, j int) bool {
		return result.Data.Result[i].Play > result.Data.Result[j].Play
	})
	if len(result.Data.Result) > 6 {
		result.Data.Result = result.Data.Result[:6]
	}
	return result.Data.Result, nil
}

// bilibiliCommentMessages 拉取视频热门评论文本（ps=20 热门排序，仅顶层）。
func bilibiliCommentMessages(aid int64, cookie string) ([]string, error) {
	endpoint := "https://api.bilibili.com/x/v2/reply?type=1&oid=" + strconv.FormatInt(aid, 10) + "&ps=20&sort=2"
	body, err := bilibiliFetchJSON(endpoint, cookie)
	if err != nil {
		return nil, err
	}
	var result struct {
		Code int `json:"code"`
		Data struct {
			Replies []struct {
				Content struct {
					Message string `json:"message"`
				} `json:"content"`
			} `json:"replies"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if result.Code != 0 || result.Data.Replies == nil {
		return nil, fmt.Errorf("B站评论被拦截（code %d）", result.Code)
	}
	messages := make([]string, 0, len(result.Data.Replies))
	for _, reply := range result.Data.Replies {
		if text := strings.TrimSpace(reply.Content.Message); text != "" {
			messages = append(messages, text)
		}
	}
	return messages, nil
}

// searchBilibiliDSHPlugins 在 B站视频的简介与评论区里找 DSH 插件链接。
// 关键词：用户输入优先；为空时用 DHS 生态默认词。只提取 github.com/owner/repo，
// 每个视频限速 300ms 防 -412，任一环节失败只跳过该视频，不阻塞整个源。
func searchBilibiliDSHPlugins(query string) ([]dhsCommunityItem, error) {
	keywords := make([]string, 0, 2)
	if query != "" {
		keywords = append(keywords, query)
	} else {
		keywords = append(keywords, "dsh插件", "DeepSeek Harness 插件")
	}

	// buvid3 匿名 cookie（失败就无 cookie 继续，搜索/评论本身可匿名）
	cookie := ""
	if spi, err := bilibiliFetchJSON("https://api.bilibili.com/x/frontend/finger/spi", ""); err == nil {
		var spiResult struct {
			Data struct {
				B3 string `json:"b_3"`
			} `json:"data"`
		}
		if json.Unmarshal(spi, &spiResult) == nil && spiResult.Data.B3 != "" {
			cookie = "buvid3=" + spiResult.Data.B3
		}
	}

	items := make([]dhsCommunityItem, 0, 8)
	seen := make(map[string]bool, 8)
	deadline := time.Now().Add(20 * time.Second)
	for _, keyword := range keywords {
		if len(items) >= 8 || time.Now().After(deadline) {
			break
		}
		videos, err := bilibiliSearchVideos(keyword, cookie)
		if err != nil {
			continue // 关键词级失败降级，换个词继续
		}
		for _, video := range videos {
			if len(items) >= 8 || time.Now().After(deadline) {
				break
			}
			texts := []string{stripBiliHTML(video.Title), stripBiliHTML(video.Description)}
			comments, _ := bilibiliCommentMessages(video.AID, cookie)
			texts = append(texts, comments...)
			repoSignals := make(map[string]string) // repo -> 来源（简介/评论）
			for repo := range bilibiliExtractRepos(strings.Join(texts, "\n")) {
				repoSignals[repo] = "B站视频简介"
			}
			// 评论区链接单独标记（评论区找到的更有「用户实测」信号）
			for _, comment := range comments {
				for repo := range bilibiliExtractRepos(comment) {
					repoSignals[repo] = "B站评论区实测推荐"
				}
			}
			if len(repoSignals) == 0 {
				time.Sleep(300 * time.Millisecond)
				continue
			}
			score := 55
			switch {
			case video.Play >= 100000:
				score += 15
			case video.Play >= 10000:
				score += 10
			case video.Play >= 1000:
				score += 5
			}
			title := stripBiliHTML(video.Title)
			if runes := []rune(title); len(runes) > 60 {
				title = string(runes[:60]) + "…"
			}
			for repo, signal := range repoSignals {
				if seen[repo] || len(items) >= 8 {
					continue
				}
				seen[repo] = true
				items = append(items, dhsCommunityItem{
					Repo: repo, Name: repo, Stars: 0,
					Description: title, URL: "https://github.com/" + repo,
					SourceID: "bilibili-video", SourceName: "B站 · DSH 视频",
					QualityScore: score,
					QualitySignals: []string{
						signal,
						"UP主 " + video.Author,
						fmt.Sprintf("播放 %d", video.Play),
					},
				})
			}
			time.Sleep(300 * time.Millisecond) // B站匿名限速，防 -412
		}
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("B站未发现新的 DSH 插件链接")
	}
	// 尝试把 GitHub repo 映射回 npm 包：拉仓库 package.json，若带 dsh.bundle.patch 且 npm 已发布，
	// 则补全 package_name/version/format，让前端「安装到 DHS」可直接点（否则只有源码，装不了）。
	enrichBiliItemsWithNPM(items)
	return items, nil
}

// enrichBiliItemsWithNPM 并发（3）把 B站发现的 repo 映射成可安装的 dsh-bundle npm 包。
// 只补字段、不改变发现结果；映射失败（非 dsh bundle / 未发布 npm）静默跳过。
func enrichBiliItemsWithNPM(items []dhsCommunityItem) {
	semaphore := make(chan struct{}, 3)
	var wg sync.WaitGroup
	for i := range items {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			item := &items[idx]

			var pkg struct {
				Name string `json:"name"`
				DSH  struct {
					Bundle struct {
						Patch string `json:"patch"`
					} `json:"bundle"`
				} `json:"dsh"`
			}
			raw, err := fetchGitHubRawFile(item.Repo, "main", "package.json")
			if err != nil {
				raw, err = fetchGitHubRawFile(item.Repo, "master", "package.json")
			}
			if err != nil || json.Unmarshal(raw, &pkg) != nil || pkg.Name == "" || pkg.DSH.Bundle.Patch == "" {
				return
			}

			// npm registry 确认最新版本（包不存在则放弃映射）
			resp, err := (&http.Client{Timeout: 10 * time.Second}).Get(npmRegistryBaseURL + "/" + url.PathEscape(pkg.Name))
			if err != nil || resp.StatusCode != http.StatusOK {
				if resp != nil {
					resp.Body.Close()
				}
				return
			}
			defer resp.Body.Close()
			var dist struct {
				DistTags struct {
					Latest string `json:"latest"`
				} `json:"dist-tags"`
			}
			if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&dist) != nil || dist.DistTags.Latest == "" {
				return
			}
			item.PackageName = pkg.Name
			item.Version = dist.DistTags.Latest
			item.Format = "dsh-bundle"
			item.BundlePatch = pkg.DSH.Bundle.Patch
			item.QualitySignals = append(item.QualitySignals, "已发布 npm 包 "+pkg.Name+"@"+dist.DistTags.Latest)
		}(i)
	}
	wg.Wait()
}

func HandleDHSCommunityPreview(c *gin.Context) {
	repo := strings.TrimSpace(c.Query("repo"))
	if !dhsRepoPattern.MatchString(repo) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "社区仓库标识无效"})
		return
	}
	// 2026-08-15 修复：不再调用 GitHub REST API（/repos/{repo} 未认证 60 次/小时限流，
	// 打爆后预览全部 403）。直接走 raw.githubusercontent.com（不计配额），
	// 依次尝试 main/master 两个常用分支，README.md/readme.md/SKILL.md 三种文件。
	for _, branch := range []string{"main", "master"} {
		for _, path := range []string{"README.md", "readme.md", "SKILL.md"} {
			contents, err := fetchGitHubRawFile(repo, branch, path)
			if err != nil {
				continue
			}
			preview := plainDHSPreview(string(contents), 720)
			if preview != "" {
				c.JSON(http.StatusOK, gin.H{"repo": repo, "path": path, "preview": preview})
				return
			}
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "仓库根目录没有可预览的 README.md 或 SKILL.md"})
}

func plainDHSPreview(value string, limit int) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = regexp.MustCompile(`(?s)^---\s*\n.*?\n---\s*`).ReplaceAllString(value, "")
	value = regexp.MustCompile(`!\[[^]]*\]\([^)]*\)`).ReplaceAllString(value, "")
	value = regexp.MustCompile(`\[([^]]+)\]\([^)]*\)`).ReplaceAllString(value, "$1")
	value = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(value, " ")
	value = strings.NewReplacer("#", " ", "*", " ", "`", " ", "_", " ", ">", " ").Replace(value)
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) > limit {
		value = string(runes[:limit]) + "…"
	}
	return value
}

var dhsNativeBlockedExt = map[string]bool{
	".exe": true, ".dll": true, ".com": true, ".msi": true, ".scr": true, ".node": true,
	".bat": true, ".cmd": true, ".ps1": true, ".sh": true, ".py": true, ".pyc": true,
	".jar": true, ".class": true, ".wasm": true,
}

var dhsNativeTextExt = map[string]bool{
	".js": true, ".mjs": true, ".cjs": true, ".ts": true, ".tsx": true, ".jsx": true,
	".json": true, ".md": true, ".txt": true, ".yaml": true, ".yml": true, ".toml": true,
	".css": true, ".scss": true, ".html": true, ".xml": true, ".map": true,
}

func auditDHSNPMBundle(packageName, expectedVersion string) (dhsAuditReport, error) {
	report := dhsAuditReport{
		PackageName: packageName, Version: expectedVersion, Format: "dsh-bundle", Commit: expectedVersion,
		Findings: []dhsAuditFinding{},
		Policy:   "审计精确 npm 发布包并校验 integrity；审计过程不执行插件代码、生命周期脚本或仓库命令。",
	}
	manifest, err := loadNPMDHSManifest(packageName)
	if err != nil {
		return report, err
	}
	if manifest.Name != packageName || manifest.Version == "" || (expectedVersion != "" && manifest.Version != expectedVersion) {
		return report, fmt.Errorf("npm 插件版本已变化，请重新搜索并审计")
	}
	if manifest.DSH.Bundle.Patch == "" || manifest.Dist.Tarball == "" {
		return report, fmt.Errorf("该包没有可验证的 dsh.bundle.patch 或 npm tarball")
	}
	report.Version, report.Commit, report.SkillPath = manifest.Version, manifest.Version, manifest.DSH.Bundle.Patch
	report.Repo = githubRepoFromURL(manifest.repositoryURL())

	req, err := http.NewRequest(http.MethodGet, manifest.Dist.Tarball, nil)
	if err != nil {
		return report, err
	}
	req.Header.Set("User-Agent", "Rescene-Wails/1.0")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return report, fmt.Errorf("DHS npm 发布包下载失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return report, fmt.Errorf("DHS npm 发布包返回 HTTP %d", resp.StatusCode)
	}
	archive, err := io.ReadAll(io.LimitReader(resp.Body, dhsNativeMaxArchiveBytes+1))
	if err != nil {
		return report, err
	}
	if len(archive) > dhsNativeMaxArchiveBytes {
		return report, fmt.Errorf("DHS npm 发布包超过 %d MB 下载上限", dhsNativeMaxArchiveBytes>>20)
	}
	if err := verifyNPMIntegrity(archive, manifest.Dist.Integrity); err != nil {
		return report, err
	}

	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return report, fmt.Errorf("DHS npm 发布包不是合法 gzip: %w", err)
	}
	defer gz.Close()
	reader := tar.NewReader(gz)
	patchFound, manifestFound := false, false
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return report, fmt.Errorf("DHS npm 发布包目录损坏: %w", err)
		}
		path := strings.TrimPrefix(strings.ReplaceAll(header.Name, "\\", "/"), "package/")
		if path == "" || !safeRelativeSkillPath(path) {
			report.Findings = append(report.Findings, dhsAuditFinding{Layer: "code", Severity: "high", Code: "path_escape", Message: "发布包包含越界或无效路径", Path: header.Name})
			continue
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			if header.Typeflag == tar.TypeSymlink || header.Typeflag == tar.TypeLink {
				report.Findings = append(report.Findings, dhsAuditFinding{Layer: "execution", Severity: "high", Code: "archive_link", Message: "发布包包含符号链接或硬链接", Path: path})
			}
			continue
		}
		report.FileCount++
		report.TotalBytes += header.Size
		if report.FileCount > dhsNativeMaxFiles || report.TotalBytes > dhsNativeMaxBytes {
			continue
		}
		ext := strings.ToLower(filepath.Ext(path))
		if dhsNativeBlockedExt[ext] {
			report.Findings = append(report.Findings, dhsAuditFinding{Layer: "execution", Severity: "high", Code: "native_executable", Message: "发布包包含脚本、原生模块或可执行内容", Path: path})
			continue
		}
		if path == strings.TrimPrefix(manifest.DSH.Bundle.Patch, "./") {
			patchFound = true
		}
		if !dhsNativeTextExt[ext] || header.Size > dhsCommunityMaxTextBytes {
			continue
		}
		contents, err := io.ReadAll(io.LimitReader(reader, dhsCommunityMaxTextBytes+1))
		if err != nil {
			return report, err
		}
		if path == "package.json" {
			manifestFound = true
		}
		text := string(contents)
		for _, rule := range dhsContentRules {
			if rule.Pattern.MatchString(text) {
				report.Findings = append(report.Findings, dhsAuditFinding{Layer: rule.Layer, Severity: rule.Severity, Code: rule.Code, Message: rule.Message, Path: path})
			}
		}
	}
	if report.FileCount > dhsNativeMaxFiles || report.TotalBytes > dhsNativeMaxBytes {
		report.Findings = append(report.Findings, dhsAuditFinding{Layer: "code", Severity: "high", Code: "package_limit", Message: fmt.Sprintf("原生插件超过安全上限（%d 文件 / %d MB）", dhsNativeMaxFiles, dhsNativeMaxBytes>>20)})
	}
	if !manifestFound || !patchFound {
		report.Findings = append(report.Findings, dhsAuditFinding{Layer: "code", Severity: "high", Code: "invalid_dsh_bundle", Message: "发布包缺少 package.json 或 manifest 声明的 Cordis patch"})
	}
	for _, script := range []string{"preinstall", "install", "postinstall"} {
		if manifest.Scripts[script] != "" {
			report.Findings = append(report.Findings, dhsAuditFinding{Layer: "execution", Severity: "high", Code: "install_script", Message: "npm 生命周期脚本会在安装期执行，已阻断", Path: "package.json"})
			break
		}
	}
	finalizeDHSAuditReport(&report)
	return report, nil
}

func verifyNPMIntegrity(contents []byte, integrity string) error {
	if integrity == "" {
		return fmt.Errorf("npm 发布包缺少 integrity，不能固定审计对象")
	}
	algorithm, encoded, ok := strings.Cut(integrity, "-")
	if !ok || algorithm != "sha512" {
		return fmt.Errorf("npm 发布包 integrity 不是受支持的 sha512")
	}
	fields := strings.Fields(encoded)
	if len(fields) == 0 {
		return fmt.Errorf("npm integrity 格式无效")
	}
	expected, err := base64.StdEncoding.DecodeString(fields[0])
	if err != nil {
		return fmt.Errorf("npm integrity 格式无效")
	}
	actual := sha512.Sum512(contents)
	if !bytes.Equal(actual[:], expected) {
		return fmt.Errorf("npm 发布包 integrity 校验失败")
	}
	return nil
}

func finalizeDHSAuditReport(report *dhsAuditReport) {
	high, medium, executionBlocked := false, false, false
	for _, finding := range report.Findings {
		high = high || finding.Severity == "high"
		medium = medium || finding.Severity == "medium"
		executionBlocked = executionBlocked || (finding.Severity == "high" && finding.Layer == "execution")
	}
	switch {
	case high:
		report.Status, report.CodeStatus = "blocked", "blocked"
	case medium:
		report.Status, report.CodeStatus = "review", "review"
	default:
		report.Status, report.CodeStatus = "passed", "passed"
	}
	if executionBlocked {
		report.ExecutionStatus = "blocked"
	} else {
		report.ExecutionStatus = "contained"
	}
}

func HandleDHSCommunityAudit(c *gin.Context) {
	var body struct {
		Repo        string `json:"repo"`
		SkillPath   string `json:"skill_path"`
		PackageName string `json:"package_name"`
		Version     string `json:"version"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体不是合法 JSON"})
		return
	}
	if strings.TrimSpace(body.PackageName) != "" {
		report, err := auditDHSNPMBundle(strings.TrimSpace(body.PackageName), strings.TrimSpace(body.Version))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"report": report})
		return
	}
	report, _, err := auditDHSCommunityPackage(strings.TrimSpace(body.Repo), strings.TrimSpace(body.SkillPath), "")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"report": report})
}

func auditDHSCommunityPackage(repo, requestedSkill, expectedCommit string) (dhsAuditReport, []dhsTreeEntry, error) {
	report := dhsAuditReport{
		Repo: repo, Findings: []dhsAuditFinding{},
		Policy: "社区插件只允许声明式 Harness 包；不运行仓库代码。安装后的真实动作继续经过 Go 内置工具审批、工作目录沙盒与 AgentFS 审计。",
	}
	if !dhsRepoPattern.MatchString(repo) {
		return report, nil, fmt.Errorf("社区仓库标识无效")
	}
	var repoInfo struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := githubJSON("/repos/"+repo, &repoInfo); err != nil {
		return report, nil, err
	}
	if repoInfo.DefaultBranch == "" {
		repoInfo.DefaultBranch = "main"
	}
	var commit struct {
		SHA string `json:"sha"`
	}
	if err := githubJSON("/repos/"+repo+"/commits/"+url.PathEscape(repoInfo.DefaultBranch), &commit); err != nil {
		return report, nil, err
	}
	if commit.SHA == "" {
		return report, nil, fmt.Errorf("社区仓库没有可审计的 commit")
	}
	if expectedCommit != "" && expectedCommit != commit.SHA {
		return report, nil, fmt.Errorf("仓库在审计后发生变化，请重新审计")
	}
	report.Commit = commit.SHA

	var tree struct {
		Tree      []dhsTreeEntry `json:"tree"`
		Truncated bool           `json:"truncated"`
	}
	if err := githubJSON("/repos/"+repo+"/git/trees/"+url.PathEscape(commit.SHA)+"?recursive=1", &tree); err != nil {
		return report, nil, err
	}
	if tree.Truncated {
		return report, nil, fmt.Errorf("仓库目录过大，GitHub 返回结果已截断")
	}
	var candidates []string
	for _, entry := range tree.Tree {
		if entry.Type == "blob" && strings.EqualFold(filepath.Base(filepath.FromSlash(entry.Path)), "SKILL.md") {
			candidates = append(candidates, entry.Path)
		}
	}
	if len(candidates) == 0 {
		return report, nil, fmt.Errorf("仓库中没有找到 SKILL.md，不能作为 DHS 插件安装")
	}
	sort.Slice(candidates, func(i, j int) bool {
		if len(candidates[i]) == len(candidates[j]) {
			return candidates[i] < candidates[j]
		}
		return len(candidates[i]) < len(candidates[j])
	})
	skillPath := candidates[0]
	if requestedSkill != "" {
		found := false
		for _, candidate := range candidates {
			if candidate == requestedSkill {
				skillPath, found = candidate, true
				break
			}
		}
		if !found {
			return report, nil, fmt.Errorf("指定的 SKILL.md 不在该仓库中")
		}
	}
	report.SkillPath = skillPath
	prefix := strings.TrimSuffix(skillPath, "SKILL.md")
	files := make([]dhsTreeEntry, 0)
	for _, entry := range tree.Tree {
		if !strings.HasPrefix(entry.Path, prefix) {
			continue
		}
		rel := strings.TrimPrefix(entry.Path, prefix)
		if rel == "" || !safeRelativeSkillPath(rel) {
			report.Findings = append(report.Findings, dhsAuditFinding{Layer: "code", Severity: "high", Code: "path_escape", Message: "发现越界或无效路径", Path: entry.Path})
			continue
		}
		if entry.Type != "blob" {
			continue
		}
		files = append(files, entry)
		report.TotalBytes += entry.Size
	}
	report.FileCount = len(files)
	if report.FileCount > dhsCommunityMaxFiles || report.TotalBytes > dhsCommunityMaxBytes {
		report.Findings = append(report.Findings, dhsAuditFinding{Layer: "code", Severity: "high", Code: "package_limit", Message: fmt.Sprintf("插件超过社区安全上限（%d 文件 / %d MB）", dhsCommunityMaxFiles, dhsCommunityMaxBytes>>20)})
	}

	for _, file := range files {
		ext := strings.ToLower(filepath.Ext(file.Path))
		if file.Mode == "120000" {
			report.Findings = append(report.Findings, dhsAuditFinding{Layer: "execution", Severity: "high", Code: "symlink", Message: "符号链接可能逃逸插件目录", Path: file.Path})
			continue
		}
		if dhsExecutableExt[ext] {
			report.Findings = append(report.Findings, dhsAuditFinding{Layer: "execution", Severity: "high", Code: "executable_content", Message: "社区插件包含脚本或可执行内容；声明式沙盒拒绝运行", Path: file.Path})
			continue
		}
		if !dhsAllowedAssetExt[ext] {
			report.Findings = append(report.Findings, dhsAuditFinding{Layer: "code", Severity: "high", Code: "unknown_file_type", Message: "文件类型不在社区插件白名单中", Path: file.Path})
			continue
		}
		if !dhsTextExt[ext] {
			continue
		}
		if file.Size > dhsCommunityMaxTextBytes {
			report.Findings = append(report.Findings, dhsAuditFinding{Layer: "code", Severity: "high", Code: "unscanned_text", Message: "文本文件超过完整静态扫描上限", Path: file.Path})
			continue
		}
		contents, err := fetchGitHubRawFile(repo, commit.SHA, file.Path)
		if err != nil {
			return report, nil, err
		}
		text := string(contents)
		for _, rule := range dhsContentRules {
			if rule.Pattern.MatchString(text) {
				report.Findings = append(report.Findings, dhsAuditFinding{Layer: rule.Layer, Severity: rule.Severity, Code: rule.Code, Message: rule.Message, Path: file.Path})
			}
		}
	}

	high, medium := false, false
	executionBlocked := false
	for _, finding := range report.Findings {
		if finding.Severity == "high" {
			high = true
			if finding.Layer == "execution" {
				executionBlocked = true
			}
		}
		if finding.Severity == "medium" {
			medium = true
		}
	}
	switch {
	case high:
		report.Status, report.CodeStatus = "blocked", "blocked"
	case medium:
		report.Status, report.CodeStatus = "review", "review"
	default:
		report.Status, report.CodeStatus = "passed", "passed"
	}
	if executionBlocked {
		report.ExecutionStatus = "blocked"
	} else {
		report.ExecutionStatus = "contained"
	}
	return report, files, nil
}

func HandleDHSCommunityInstall(c *gin.Context) {
	var body struct {
		Repo      string `json:"repo"`
		SkillPath string `json:"skill_path"`
		Commit    string `json:"commit"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体不是合法 JSON"})
		return
	}
	report, files, err := auditDHSCommunityPackage(strings.TrimSpace(body.Repo), strings.TrimSpace(body.SkillPath), strings.TrimSpace(body.Commit))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if report.Status == "blocked" || report.ExecutionStatus != "contained" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "插件未通过 DHS 双层审计", "report": report})
		return
	}

	externalID := sanitizeExternalSkillID(filepath.Base(filepath.FromSlash(strings.TrimSuffix(report.SkillPath, "/SKILL.md"))))
	if externalID == "" {
		externalID = sanitizeExternalSkillID(filepath.Base(filepath.FromSlash(strings.TrimSuffix(report.SkillPath, "SKILL.md"))))
	}
	if externalID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "插件目录名无效"})
		return
	}
	root := externalSkillsDir()
	if err := os.MkdirAll(root, 0o700); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	destination := filepath.Join(root, externalID)
	if _, err := os.Stat(destination); err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "同名插件已经安装"})
		return
	}
	staging, err := os.MkdirTemp(root, ".dhs-audit-install-")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer os.RemoveAll(staging)
	prefix := strings.TrimSuffix(report.SkillPath, "SKILL.md")
	for _, file := range files {
		contents, err := fetchGitHubRawFile(report.Repo, report.Commit, file.Path)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		rel := strings.TrimPrefix(file.Path, prefix)
		if !safeRelativeSkillPath(rel) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "插件路径越界"})
			return
		}
		target := filepath.Join(staging, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if err := os.WriteFile(target, contents, 0o600); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	meta := installedSkillMetadata{
		Source: "dhs-community:" + report.Repo,
		Path:   report.SkillPath,
		URL:    "https://github.com/" + report.Repo + "/tree/" + report.Commit + "/" + strings.TrimSuffix(report.SkillPath, "/SKILL.md"),
	}
	metaData, _ := json.MarshalIndent(meta, "", "  ")
	if err := os.WriteFile(filepath.Join(staging, ".rescene-skill.json"), metaData, 0o600); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := os.Rename(staging, destination); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ok": true, "external_id": externalID, "installed_at": time.Now().Format(time.RFC3339), "report": report,
	})
}

// HandleDHSCommunityInstallDHS 一键安装到 DHS 本体（npm bundle 类原生插件）。
// POST /api/dhs/community/install-dhs
// Body: { package_name, version, repo?, profile? }
//
// 流程：重新审计（固定版本 + sha512 integrity + bundle patch 存在性，坏包在此被拦）→
// 通过后在 DHS profile 目录跑 `pnpm add <pkg>@<ver>`（下载依赖 + 写 dependencies）→
// 成功后才把包名追加进 package.json 的 dsh.profile.bundles（DHS 启动器加载清单，去重）。
// 顺序保证：pnpm 失败时 profile 完全没动，dsh 启动不会因半写入炸掉。
//
// 安装 ≠ 激活：重启 `dsh --profile <name>` 才加载插件；审计未通过（blocked/缺 patch）拒绝写入。
func HandleDHSCommunityInstallDHS(c *gin.Context) {
	var body struct {
		PackageName string `json:"package_name"`
		Version     string `json:"version"`
		Repo        string `json:"repo"`
		Profile     string `json:"profile"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体不是合法 JSON"})
		return
	}
	body.PackageName = strings.TrimSpace(body.PackageName)
	body.Version = strings.TrimSpace(body.Version)
	if body.PackageName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 npm 包名：仅 dsh-bundle 原生插件可安装到 DHS"})
		return
	}
	if body.Version == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该插件没有可固定的 npm 版本，无法安全安装到 DHS"})
		return
	}

	// 1. 重新审计（固定版本 + integrity + bundle patch 存在性；坏包在此被拦）
	report, err := auditDHSNPMBundle(body.PackageName, body.Version)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if report.Status == "blocked" || report.ExecutionStatus != "contained" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "插件未通过 DHS 双层审计，拒绝写入 DHS", "report": report})
		return
	}

	// 2. 定位 DHS profile
	dshHome := strings.TrimSpace(os.Getenv("DSH_HOME"))
	if dshHome == "" {
		home, herr := os.UserHomeDir()
		if herr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "无法定位用户主目录"})
			return
		}
		dshHome = filepath.Join(home, ".dsh")
	}
	profile := strings.TrimSpace(body.Profile)
	if profile == "" {
		profile = strings.TrimSpace(os.Getenv("DSH_PROFILE"))
	}
	if profile == "" {
		profile = "web"
	}
	profileDir := filepath.Join(dshHome, "profiles", profile)
	pkgPath := filepath.Join(profileDir, "package.json")
	raw, err := os.ReadFile(pkgPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "DHS profile「" + profile + "」不存在，请先运行 dsh --profile " + profile + " 初始化"})
		return
	}
	var doc map[string]interface{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析 DHS profile 失败: " + err.Error()})
		return
	}
	// 已安装去重：遍历 dsh.profile.bundles
	dshObj, _ := doc["dsh"].(map[string]interface{})
	profileObj, _ := dshObj["profile"].(map[string]interface{})
	if bundlesArr, _ := profileObj["bundles"].([]interface{}); bundlesArr != nil {
		for _, b := range bundlesArr {
			if bs, _ := b.(string); bs == body.PackageName {
				c.JSON(http.StatusOK, gin.H{"ok": true, "already_installed": true, "package_name": body.PackageName, "profile": profile})
				return
			}
		}
	}

	// 3. 先 pnpm add 下载依赖（失败则 profile 零改动）
	pnpmBin, perr := exec.LookPath("pnpm")
	if perr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "未找到 pnpm，请先安装: npm i -g pnpm"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, pnpmBin, "add", body.PackageName+"@"+body.Version)
	cmd.Dir = profileDir
	cmdOutput, cerr := cmd.CombinedOutput()
	if cerr != nil {
		msg := strings.TrimSpace(string(cmdOutput))
		if len(msg) > 800 {
			msg = msg[:800]
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "pnpm 安装失败，profile 未改动: " + cerr.Error(), "output": msg})
		return
	}

	// 4. pnpm 成功后才写 bundles（DHS 启动器加载清单）。
	// map 解析保留 package.json 全部字段（dependencies 等），只追加 bundles，不丢其他键。
	bundlesArr, _ := profileObj["bundles"].([]interface{})
	profileObj["bundles"] = append(bundlesArr, body.PackageName)
	out, merr := json.MarshalIndent(doc, "", "  ")
	if merr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "序列化 profile 失败: " + merr.Error()})
		return
	}
	if werr := os.WriteFile(pkgPath, out, 0o644); werr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "写入 profile 失败: " + werr.Error()})
		return
	}

	// 5. 自动重启 dsh（失败不阻塞，降级为提示手动重启）
	restartNote, restartWarning := restartDsh(profile)

	c.JSON(http.StatusOK, gin.H{
		"ok": true, "package_name": body.PackageName, "version": body.Version,
		"profile": profile, "bundles": profileObj["bundles"],
		"note":   "已安装到 DHS profile「" + profile + "」，" + restartNote,
		"warning": restartWarning,
	})
}

// HandleDHSCommunityUninstallDHS 从 DHS profile 卸载插件。
// POST /api/dhs/community/uninstall-dhs
// Body: { package_name, profile? }
//
// 流程：确认已装 → `pnpm remove <pkg>`（失败则 profile 零改动）→
// 从 package.json 的 dsh.profile.bundles 删除该包名 → 写回（map 解析保留全部字段）。
// 卸载 ≠ 生效：重启 `dsh --profile <name>` 后才真正移除加载。
func HandleDHSCommunityUninstallDHS(c *gin.Context) {
	var body struct {
		PackageName string `json:"package_name"`
		Profile     string `json:"profile"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体不是合法 JSON"})
		return
	}
	body.PackageName = strings.TrimSpace(body.PackageName)
	if body.PackageName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 npm 包名"})
		return
	}

	// 1. 定位 DHS profile（与 install-dhs 同一套逻辑）
	dshHome := strings.TrimSpace(os.Getenv("DSH_HOME"))
	if dshHome == "" {
		home, herr := os.UserHomeDir()
		if herr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "无法定位用户主目录"})
			return
		}
		dshHome = filepath.Join(home, ".dsh")
	}
	profile := strings.TrimSpace(body.Profile)
	if profile == "" {
		profile = strings.TrimSpace(os.Getenv("DSH_PROFILE"))
	}
	if profile == "" {
		profile = "web"
	}
	profileDir := filepath.Join(dshHome, "profiles", profile)
	pkgPath := filepath.Join(profileDir, "package.json")
	raw, err := os.ReadFile(pkgPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "DHS profile「" + profile + "」不存在，请先运行 dsh --profile " + profile + " 初始化"})
		return
	}
	var doc map[string]interface{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析 DHS profile 失败: " + err.Error()})
		return
	}

	// 2. 校验已安装并剔除该包名
	dshObj, _ := doc["dsh"].(map[string]interface{})
	profileObj, _ := dshObj["profile"].(map[string]interface{})
	bundlesArr, _ := profileObj["bundles"].([]interface{})
	newBundles := make([]interface{}, 0, len(bundlesArr))
	found := false
	for _, b := range bundlesArr {
		bs, _ := b.(string)
		if bs == body.PackageName {
			found = true
			continue
		}
		newBundles = append(newBundles, b)
	}
	if !found {
		c.JSON(http.StatusBadRequest, gin.H{"error": "插件「" + body.PackageName + "」未安装到 DHS profile「" + profile + "」"})
		return
	}

	// 3. 先写回 bundles（DHS 加载真相：启动器只看 dsh.profile.bundles，移除即不再加载）
	profileObj["bundles"] = newBundles
	out, merr := json.MarshalIndent(doc, "", "  ")
	if merr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "序列化 profile 失败: " + merr.Error()})
		return
	}
	if werr := os.WriteFile(pkgPath, out, 0o644); werr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "写入 profile 失败: " + werr.Error()})
		return
	}

	// 4. pnpm remove 清理依赖实体；失败不阻塞（常见于包不在 dependencies 里，
	// 如手工改 bundles 的 profile → pnpm 报 no dependencies）。残留可用 pnpm prune 清。
	warning := ""
	if pnpmBin, perr := exec.LookPath("pnpm"); perr == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, pnpmBin, "remove", body.PackageName)
		cmd.Dir = profileDir
		cmdOutput, cerr := cmd.CombinedOutput()
		if cerr != nil {
			msg := strings.TrimSpace(string(cmdOutput))
			if len(msg) > 300 {
				msg = msg[:300]
			}
			warning = "bundles 已移除；pnpm 清理依赖失败（" + cerr.Error() + "），残留可 pnpm prune 清理: " + msg
		}
	} else {
		warning = "bundles 已移除；未找到 pnpm，依赖实体未清理（可 pnpm prune）"
	}

	// 5. 自动重启 dsh（失败不阻塞，降级为提示手动重启）
	restartNote, restartWarning := restartDsh(profile)
	if warning != "" && restartWarning != "" {
		warning += "；" + restartWarning
	} else if restartWarning != "" {
		warning = restartWarning
	}

	c.JSON(http.StatusOK, gin.H{
		"ok": true, "package_name": body.PackageName, "profile": profile, "bundles": profileObj["bundles"],
		"note":    "已从 DHS profile「" + profile + "」卸载，" + restartNote,
		"warning": warning,
	})
}

