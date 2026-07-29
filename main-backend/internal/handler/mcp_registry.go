package handler

// MCP 官方 Registry 接入。这里只消费公开的发现 API，并把可直接连接的
// Streamable HTTP server 写进用户 mcp.json；运行时连接由 mcp_client.go 的
// 纯 Go HTTP transport 完成，因此 Wails 成品不依赖 npx、Node 或 Python。

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const officialMCPRegistrySource = "official-registry"

var (
	officialMCPRegistryBaseURL = "https://registry.modelcontextprotocol.io/v0.1"
	mcpConfigWriteMu           sync.Mutex
)

type registryMCPItem struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
	URL         string `json:"url"`
	Transport   string `json:"transport"`
	Installed   bool   `json:"installed"`
	ConfigName  string `json:"config_name,omitempty"`
}

type officialRegistryResponse struct {
	Servers []struct {
		Server struct {
			Name        string `json:"name"`
			Title       string `json:"title"`
			Description string `json:"description"`
			Version     string `json:"version"`
			Remotes     []struct {
				Type    string `json:"type"`
				URL     string `json:"url"`
				Headers []struct {
					Name       string `json:"name"`
					IsRequired bool   `json:"isRequired"`
				} `json:"headers"`
			} `json:"remotes"`
		} `json:"server"`
	} `json:"servers"`
	Metadata struct {
		NextCursor string `json:"nextCursor"`
		Count      int    `json:"count"`
	} `json:"metadata"`
}

func registryHTTPClient() *http.Client {
	return &http.Client{Timeout: 20 * time.Second}
}

func fetchOfficialMCPRegistry(query string, limit int) ([]registryMCPItem, error) {
	if limit < 1 || limit > 100 {
		limit = 30
	}
	endpoint, err := url.Parse(officialMCPRegistryBaseURL + "/servers")
	if err != nil {
		return nil, err
	}
	values := endpoint.Query()
	values.Set("limit", strconv.Itoa(limit))
	values.Set("version", "latest")
	if query = strings.TrimSpace(query); query != "" {
		values.Set("search", query)
	}
	endpoint.RawQuery = values.Encode()
	req, _ := http.NewRequest(http.MethodGet, endpoint.String(), nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Rescene-Wails/1.0")
	resp, err := registryHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("MCP Registry 连接失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("MCP Registry 返回 HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var decoded officialRegistryResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("MCP Registry 响应无效: %w", err)
	}
	installed := installedRegistryMCPs()
	items := make([]registryMCPItem, 0, len(decoded.Servers))
	for _, entry := range decoded.Servers {
		var remoteURL, transport string
		for _, remote := range entry.Server.Remotes {
			if remote.Type == "streamable-http" && strings.HasPrefix(remote.URL, "https://") &&
				!strings.Contains(remote.URL, "{") && len(remote.Headers) == 0 {
				remoteURL, transport = remote.URL, remote.Type
				break
			}
		}
		if remoteURL == "" {
			continue // 开箱即用版本不展示需要额外本地运行时的 package
		}
		cfgName := installed[entry.Server.Name]
		items = append(items, registryMCPItem{
			Name: entry.Server.Name, Title: entry.Server.Title, Description: entry.Server.Description,
			Version: entry.Server.Version, URL: remoteURL, Transport: transport,
			Installed: cfgName != "", ConfigName: cfgName,
		})
	}
	return items, nil
}

func installedRegistryMCPs() map[string]string {
	out := map[string]string{}
	data, err := os.ReadFile(mcpConfigPath())
	if err != nil {
		return out
	}
	var cfg mcpConfig
	if json.Unmarshal(data, &cfg) != nil {
		return out
	}
	for name, server := range cfg.Servers {
		if server.Source == officialMCPRegistrySource && server.RegistryName != "" {
			out[server.RegistryName] = name
		}
	}
	return out
}

func HandleMCPRegistry(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "30"))
	items, err := fetchOfficialMCPRegistry(c.Query("q"), limit)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"provider": "MCP Official Registry",
		"items":    items,
	})
}

func HandleInstallRegistryMCP(c *gin.Context) {
	var body struct {
		Name  string `json:"name"`
		URL   string `json:"url"`
		Title string `json:"title"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体不是合法 JSON"})
		return
	}
	body.Name, body.URL = strings.TrimSpace(body.Name), strings.TrimSpace(body.URL)
	verified, err := fetchOfficialMCPRegistry(body.Name, 100)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	valid := false
	for _, item := range verified {
		if item.Name == body.Name && item.URL == body.URL {
			valid = true
			break
		}
	}
	if !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该远程地址不在 MCP 官方 Registry 的最新版本中"})
		return
	}
	configName := ""
	err = updateMCPConfig(func(cfg *mcpConfig) error {
		for _, server := range cfg.Servers {
			if server.Source == officialMCPRegistrySource && server.RegistryName == body.Name {
				return fmt.Errorf("该 MCP 已安装")
			}
		}
		configName = uniqueMCPConfigName(body.Name, cfg.Servers)
		cfg.Servers[configName] = mcpServerConfig{
			URL: body.URL, Source: officialMCPRegistrySource, RegistryName: body.Name,
		}
		return nil
	})
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "已安装") {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	ReinitMCP()
	status := "needs_setup"
	tools := 0
	if conn := mcpConns[configName]; conn != nil {
		status = "connected"
		prefix := "mcp__" + configName + "__"
		for name := range mcpRoutes {
			if strings.HasPrefix(name, prefix) {
				tools++
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "config_name": configName, "status": status, "tools": tools})
}

func HandleUninstallRegistryMCP(c *gin.Context) {
	name := c.Param("name")
	err := updateMCPConfig(func(cfg *mcpConfig) error {
		server, ok := cfg.Servers[name]
		if !ok || server.Source != officialMCPRegistrySource {
			return fmt.Errorf("只能从这里移除通过官方 Registry 安装的 MCP")
		}
		delete(cfg.Servers, name)
		return nil
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ReinitMCP()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func updateMCPConfig(change func(*mcpConfig) error) error {
	mcpConfigWriteMu.Lock()
	defer mcpConfigWriteMu.Unlock()
	path := mcpConfigPath()
	cfg := mcpConfig{Servers: map[string]mcpServerConfig{}}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("现有 MCP 配置无法解析: %w", err)
		}
	}
	if cfg.Servers == nil {
		cfg.Servers = map[string]mcpServerConfig{}
	}
	if err := change(&cfg); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	return os.WriteFile(path, data, 0o600)
}

func uniqueMCPConfigName(registryName string, servers map[string]mcpServerConfig) string {
	base := registryName
	if slash := strings.LastIndex(base, "/"); slash >= 0 {
		base = base[slash+1:]
	}
	var b strings.Builder
	for _, r := range strings.ToLower(base) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else if b.Len() > 0 {
			b.WriteByte('-')
		}
	}
	base = strings.Trim(b.String(), "-_")
	if base == "" {
		base = "remote"
	}
	if _, used := servers[base]; !used {
		return base
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if _, used := servers[candidate]; !used {
			return candidate
		}
	}
}
