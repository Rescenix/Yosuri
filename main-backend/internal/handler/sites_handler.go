package handler

// Netlify 站点发布：把 Agent 已经构建好的静态目录打包并上传到用户自己的 Netlify
// 账号。这里刻意不保存 Personal Access Token；令牌只用于当前一次请求，避免把
// 可发布任意站点的凭据以明文留在 Rescene 的本地数据目录里。

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"backend/internal/ai/core"
	"github.com/gin-gonic/gin"
)

const maxSiteArchiveBytes int64 = 100 << 20

type siteRecord struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Source    string `json:"source"`
	SiteID    string `json:"site_id"`
	URL       string `json:"url"`
	UpdatedAt string `json:"updated_at"`
}

func sitesStatePath() string {
	if root := strings.TrimSpace(os.Getenv("RESCENE_SITES_DIR")); root != "" {
		return filepath.Join(root, "sites.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "sites.json"
	}
	return filepath.Join(home, "rescene_data", "sites.json")
}

func loadSites() ([]siteRecord, error) {
	b, err := os.ReadFile(sitesStatePath())
	if os.IsNotExist(err) {
		return []siteRecord{}, nil
	}
	if err != nil {
		return nil, err
	}
	var sites []siteRecord
	if err := json.Unmarshal(b, &sites); err != nil {
		return nil, err
	}
	return sites, nil
}

func saveSites(sites []siteRecord) error {
	p := sitesStatePath()
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(sites, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0600)
}

func safeSiteName(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	raw = regexp.MustCompile(`[^a-z0-9-]+`).ReplaceAllString(raw, "-")
	raw = strings.Trim(raw, "-")
	if len(raw) > 56 {
		raw = strings.Trim(raw[:56], "-")
	}
	return raw
}

func suggestedSiteName(name string) string {
	name = strings.Trim(name, "-")
	if len(name) > 42 {
		name = strings.Trim(name[:42], "-")
	}
	if name == "" {
		name = "agent-site"
	}
	return name + "-" + time.Now().Format("20060102-150405")
}

func isNetlifyNameConflict(message string) bool {
	message = strings.ToLower(message)
	return strings.Contains(message, "subdomain") && strings.Contains(message, "unique")
}

func siteCandidates(root string) []gin.H {
	choices := []struct{ label, rel string }{
		{"dist（推荐：生产构建）", "dist"}, {"build", "build"}, {"out", "out"}, {"根目录", "."},
	}
	result := make([]gin.H, 0, len(choices))
	for _, choice := range choices {
		path := filepath.Join(root, choice.rel)
		if info, err := os.Stat(filepath.Join(path, "index.html")); err == nil && !info.IsDir() {
			result = append(result, gin.H{"label": choice.label, "path": choice.rel})
		}
	}
	return result
}

// HandleSites GET /api/sites：返回本项目可发布的构建产物和历史发布，不返回令牌。
func HandleSites(c *gin.Context) {
	sites, err := loadSites()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取站点记录失败"})
		return
	}
	sort.Slice(sites, func(i, j int) bool { return sites[i].UpdatedAt > sites[j].UpdatedAt })
	root := core.GetProjectRoot()
	c.JSON(http.StatusOK, gin.H{"sites": sites, "workdir": root, "candidates": siteCandidates(root)})
}

func resolvedSiteSource(root, rel string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("请先在聊天里选择项目文件夹")
	}
	rel = filepath.Clean(strings.TrimSpace(rel))
	if rel == "" {
		rel = "dist"
	}
	if filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("发布目录必须位于当前项目内")
	}
	source := filepath.Join(root, rel)
	index, err := os.Stat(filepath.Join(source, "index.html"))
	if err != nil || index.IsDir() {
		return "", fmt.Errorf("在 %s 中找不到 index.html，请先让 Agent 运行构建", rel)
	}
	return source, nil
}

func zipStaticSite(root string) (*bytes.Buffer, error) {
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	var total int64
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		if total > maxSiteArchiveBytes {
			return fmt.Errorf("站点文件超过 100MB，请移除大文件后重试")
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(w, f)
		return err
	})
	if err != nil {
		zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf, nil
}

func netlifyRequest(method, url, token, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return (&http.Client{Timeout: 90 * time.Second}).Do(req)
}

func netlifyError(resp *http.Response) string {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var payload struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(b, &payload) == nil && payload.Message != "" {
		return payload.Message
	}
	return strings.TrimSpace(string(b))
}

// DeploySite POST /api/sites/deploy {name, source, token, site_id?, confirm_public}。
// site_id 存在时更新原地址；缺失时在用户账号中创建一个新 Netlify 站点。
func DeploySite(c *gin.Context) {
	var body struct {
		Name          string `json:"name"`
		Source        string `json:"source"`
		Token         string `json:"token"`
		SiteID        string `json:"site_id"`
		ConfirmPublic bool   `json:"confirm_public"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "发布参数无效"})
		return
	}
	// Agent 可以准备站点，但公开上线必须由用户确认控件显式提交。
	if !body.ConfirmPublic {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请由用户确认后再公开发布"})
		return
	}
	if strings.TrimSpace(body.Token) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请粘贴 Netlify Personal Access Token"})
		return
	}
	name := safeSiteName(body.Name)
	if name == "" {
		name = safeSiteName(filepath.Base(core.GetProjectRoot()))
	}
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入站点名称"})
		return
	}
	source, err := resolvedSiteSource(core.GetProjectRoot(), body.Source)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	siteID := strings.TrimSpace(body.SiteID)
	if siteID == "" {
		payload, _ := json.Marshal(gin.H{"name": name})
		resp, err := netlifyRequest(http.MethodPost, "https://api.netlify.com/api/v1/sites", body.Token, "application/json", bytes.NewReader(payload))
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "无法连接 Netlify：" + err.Error()})
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			message := netlifyError(resp)
			if isNetlifyNameConflict(message) {
				c.JSON(http.StatusConflict, gin.H{
					"error":          "这个公开地址名称已被占用，请确认一个新的名称后再发布",
					"suggested_name": suggestedSiteName(name),
				})
				return
			}
			c.JSON(http.StatusBadGateway, gin.H{"error": "Netlify 创建站点失败：" + message})
			return
		}
		var created struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&created); err != nil || created.ID == "" {
			c.JSON(http.StatusBadGateway, gin.H{"error": "Netlify 未返回站点 ID"})
			return
		}
		siteID = created.ID
	}

	archive, err := zipStaticSite(source)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := netlifyRequest(http.MethodPost, "https://api.netlify.com/api/v1/sites/"+siteID+"/deploys", body.Token, "application/zip", archive)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "无法上传到 Netlify：" + err.Error()})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Netlify 发布失败：" + netlifyError(resp)})
		return
	}
	var deployed struct {
		SSLURL       string `json:"ssl_url"`
		DeploySSLURL string `json:"deploy_ssl_url"`
		URL          string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&deployed); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "无法读取 Netlify 发布结果"})
		return
	}
	url := deployed.SSLURL
	if url == "" {
		url = deployed.DeploySSLURL
	}
	if url == "" {
		url = deployed.URL
	}
	record := siteRecord{ID: siteID, Name: name, Source: body.Source, SiteID: siteID, URL: url, UpdatedAt: time.Now().Format(time.RFC3339)}
	sites, err := loadSites()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存发布记录前读取失败"})
		return
	}
	found := false
	for i := range sites {
		if sites[i].SiteID == siteID {
			sites[i] = record
			found = true
			break
		}
	}
	if !found {
		sites = append(sites, record)
	}
	if err := saveSites(sites); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "站点已发布，但本地记录保存失败：" + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"site": record})
}
