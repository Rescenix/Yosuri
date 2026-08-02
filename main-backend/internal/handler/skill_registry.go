package handler

// 外部 Skill 托管接入：通过 GitHub 公共 REST API 浏览知名技能仓库，
// 安装时把 SKILL.md 与同目录附属文件下载到用户数据目录。全部由 Go 标准库完成，
// Wails 成品不调用 git、Node、Python 或第三方 CLI。

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	githubAPIBaseURL = "https://api.github.com"
	githubRawBaseURL = "https://raw.githubusercontent.com"
)

var hostedSkillSources = []struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}{
	{ID: "anthropics/skills", Label: "Anthropic Skills"},
	{ID: "openai/skills", Label: "OpenAI Skills"},
	{ID: "vercel-labs/skills", Label: "Vercel Labs Skills"},
	{ID: "rescene-cloud", Label: "ResceneCloud 插件库"},
}

type githubTreeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
	Size int64  `json:"size"`
}

type cachedGitHubSkillTree struct {
	Entries   []githubTreeEntry
	Branch    string
	ExpiresAt time.Time
}

var (
	githubSkillTreeMu    sync.Mutex
	githubSkillTreeCache = map[string]cachedGitHubSkillTree{}
)

type hostedSkillItem struct {
	Name       string `json:"name"`
	Source     string `json:"source"`
	Path       string `json:"path"`
	URL        string `json:"url"`
	Installed  bool   `json:"installed"`
	ExternalID string `json:"external_id,omitempty"`
}

type installedSkillMetadata struct {
	Source string `json:"source"`
	Path   string `json:"path"`
	URL    string `json:"url"`
}

func githubRequest(path string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(githubAPIBaseURL, "/")+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "Rescene-Wails/1.0")
	if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return (&http.Client{Timeout: 25 * time.Second}).Do(req)
}

func githubJSON(path string, out any) error {
	resp, err := githubRequest(path)
	if err != nil {
		return fmt.Errorf("GitHub 连接失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("GitHub 返回 HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 12<<20)).Decode(out); err != nil {
		return fmt.Errorf("GitHub 响应无效: %w", err)
	}
	return nil
}

func loadGitHubSkillTree(source string) ([]githubTreeEntry, string, error) {
	githubSkillTreeMu.Lock()
	if cached, ok := githubSkillTreeCache[source]; ok && time.Now().Before(cached.ExpiresAt) {
		entries := append([]githubTreeEntry(nil), cached.Entries...)
		githubSkillTreeMu.Unlock()
		return entries, cached.Branch, nil
	}
	githubSkillTreeMu.Unlock()

	var repo struct {
		DefaultBranch string `json:"default_branch"`
	}
	escapedSource := strings.Join(strings.Split(source, "/"), "/")
	if err := githubJSON("/repos/"+escapedSource, &repo); err != nil {
		return nil, "", err
	}
	if repo.DefaultBranch == "" {
		repo.DefaultBranch = "main"
	}
	var tree struct {
		Tree      []githubTreeEntry `json:"tree"`
		Truncated bool              `json:"truncated"`
	}
	treePath := "/repos/" + escapedSource + "/git/trees/" + url.PathEscape(repo.DefaultBranch) + "?recursive=1"
	if err := githubJSON(treePath, &tree); err != nil {
		return nil, "", err
	}
	if tree.Truncated {
		return nil, "", fmt.Errorf("GitHub 仓库目录过大，返回结果已截断")
	}
	githubSkillTreeMu.Lock()
	githubSkillTreeCache[source] = cachedGitHubSkillTree{
		Entries: tree.Tree, Branch: repo.DefaultBranch, ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	githubSkillTreeMu.Unlock()
	return tree.Tree, repo.DefaultBranch, nil
}

func isHostedSkillSource(source string) bool {
	for _, item := range hostedSkillSources {
		if item.ID == source {
			return true
		}
	}
	return false
}

func listHostedSkills(source, query string) ([]hostedSkillItem, error) {
	if !isHostedSkillSource(source) {
		return nil, fmt.Errorf("不支持的技能托管源")
	}
	// ResceneCloud 插件库由前端直接通过 /api/plugins 访问，
	// 这里只处理 GitHub 源（anthropics/skills、openai/skills、vercel-labs/skills）。
	if source == "rescene-cloud" {
		return nil, fmt.Errorf("ResceneCloud 插件库请通过前端插件市场访问")
	}
	tree, _, err := loadGitHubSkillTree(source)
	if err != nil {
		return nil, err
	}
	query = strings.ToLower(strings.TrimSpace(query))
	installed := installedHostedSkills()
	items := make([]hostedSkillItem, 0)
	for _, entry := range tree {
		if entry.Type != "blob" || !strings.EqualFold(filepath.Base(entry.Path), "SKILL.md") {
			continue
		}
		dir := strings.TrimSuffix(entry.Path, "/SKILL.md")
		name := filepath.Base(filepath.FromSlash(dir))
		if query != "" && !strings.Contains(strings.ToLower(name+" "+entry.Path), query) {
			continue
		}
		key := source + ":" + entry.Path
		items = append(items, hostedSkillItem{
			Name: name, Source: source, Path: entry.Path,
			URL:       "https://github.com/" + source + "/tree/HEAD/" + dir,
			Installed: installed[key] != "", ExternalID: installed[key],
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

func installedHostedSkills() map[string]string {
	out := map[string]string{}
	entries, err := os.ReadDir(externalSkillsDir())
	if err != nil {
		return out
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(externalSkillsDir(), entry.Name(), ".rescene-skill.json"))
		if err != nil {
			continue
		}
		var meta installedSkillMetadata
		if json.Unmarshal(data, &meta) == nil && meta.Source != "" && meta.Path != "" {
			out[meta.Source+":"+meta.Path] = entry.Name()
		}
	}
	return out
}

func HandleSkillRegistry(c *gin.Context) {
	source := c.DefaultQuery("source", hostedSkillSources[0].ID)
	items, err := listHostedSkills(source, c.Query("q"))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"provider": "GitHub", "sources": hostedSkillSources, "items": items})
}

func HandleInstallHostedSkill(c *gin.Context) {
	var body struct {
		Source string `json:"source"`
		Path   string `json:"path"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体不是合法 JSON"})
		return
	}
	body.Source, body.Path = strings.TrimSpace(body.Source), strings.TrimSpace(body.Path)
	if !isHostedSkillSource(body.Source) || !strings.HasSuffix(body.Path, "/SKILL.md") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "技能来源或路径无效"})
		return
	}
	tree, branch, err := loadGitHubSkillTree(body.Source)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	prefix := strings.TrimSuffix(body.Path, "SKILL.md")
	var files []githubTreeEntry
	var total int64
	foundSkill := false
	for _, entry := range tree {
		if entry.Type != "blob" || !strings.HasPrefix(entry.Path, prefix) {
			continue
		}
		rel := strings.TrimPrefix(entry.Path, prefix)
		if !safeRelativeSkillPath(rel) {
			continue
		}
		files = append(files, entry)
		total += entry.Size
		if entry.Path == body.Path {
			foundSkill = true
		}
	}
	if !foundSkill {
		c.JSON(http.StatusNotFound, gin.H{"error": "GitHub 仓库中未找到该技能"})
		return
	}
	if len(files) > 200 || total > 5<<20 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "技能目录超过安全安装上限（200 个文件 / 5 MB）"})
		return
	}
	externalID := sanitizeExternalSkillID(filepath.Base(filepath.FromSlash(strings.TrimSuffix(body.Path, "/SKILL.md"))))
	if externalID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "技能目录名无效"})
		return
	}
	root := externalSkillsDir()
	if err := os.MkdirAll(root, 0o700); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	destination := filepath.Join(root, externalID)
	if _, err := os.Stat(destination); err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "同名技能已经安装"})
		return
	}
	staging, err := os.MkdirTemp(root, ".install-")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer os.RemoveAll(staging)
	for _, file := range files {
		contents, err := fetchGitHubRawFile(body.Source, branch, file.Path)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		rel := filepath.FromSlash(strings.TrimPrefix(file.Path, prefix))
		target := filepath.Join(staging, rel)
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
		Source: body.Source, Path: body.Path,
		URL: "https://github.com/" + body.Source + "/tree/HEAD/" + strings.TrimSuffix(body.Path, "/SKILL.md"),
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
	c.JSON(http.StatusOK, gin.H{"ok": true, "external_id": externalID})
}

func fetchGitHubRawFile(source, branch, path string) ([]byte, error) {
	endpoint := strings.TrimRight(githubRawBaseURL, "/") + "/" + source + "/" + url.PathEscape(branch) + "/" + path
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Rescene-Wails/1.0")
	resp, err := (&http.Client{Timeout: 25 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitHub 文件下载失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub 文件下载返回 HTTP %d", resp.StatusCode)
	}
	contents, err := io.ReadAll(io.LimitReader(resp.Body, (2<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(contents) > 2<<20 {
		return nil, fmt.Errorf("单个技能文件超过 2 MB 安全上限")
	}
	return contents, nil
}

func safeRelativeSkillPath(path string) bool {
	if path == "" || strings.Contains(path, "\\") || strings.HasPrefix(path, "/") {
		return false
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	return clean != "." && !filepath.IsAbs(clean) && clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}

func sanitizeExternalSkillID(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else if b.Len() > 0 {
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-_")
}

func HandleDeleteExternalSkill(c *gin.Context) {
	externalID := c.Param("id")
	if externalID == "" || sanitizeExternalSkillID(externalID) != externalID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "技能标识无效"})
		return
	}
	root := filepath.Clean(externalSkillsDir())
	target := filepath.Clean(filepath.Join(root, externalID))
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "技能路径越界"})
		return
	}
	info, err := os.Stat(target)
	if err != nil || !info.IsDir() {
		c.JSON(http.StatusNotFound, gin.H{"error": "外部技能不存在"})
		return
	}
	if err := os.RemoveAll(target); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
