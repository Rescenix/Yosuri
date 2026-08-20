package handler

// aggregate_sync.go —— 聚合 API「一键同步 / 还原」（2026-08-19）。
//
// 解决痛点：用户在前端点一下，就能把聚合端口（localhost:8080/v1 + Bearer key）
// 同步到本机外部工具（codex / dsh），无需手动抄 Base URL 和 Key。
//
// 设计约束：
//   - 所有接口只在 loopback 监听的后端上跑（聚合端口本就只监听本机），不外泄。
//   - 真实 key 来自 aggregateAPIKey()（= env RESCENE_AGG_API_KEY 或源码默认 key），
//     前端原来显示的是假脱敏值 «redacted:sk-…»，这里吐真值给本机工具用。
//   - sync 默认只「生成配置片段」返回给前端；真正写回用户目录需前端显式传 apply=true，
//     写回前自动备份原文件到带时间戳的 .bak，绝不静默覆盖。
//   - claude code 不在此列：它只认 Anthropic /v1/messages 协议，聚合端口目前只有
//     OpenAI 协议（二期预留未实现），直连不了。
//
// 写回落点：
//   - codex:  ~/.codex/config.toml   （[model_providers.openai-http] base_url/env_key）
//   - dsh:    ~/.dsh/settings.yaml    （llm-pi-ai.providers.rescene.baseURL + credentials.yaml）

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	aggSyncCodex = "codex"
	aggSyncDsh   = "dsh"
)

// aggExport 导出结构：前端直接展示 + 复制。
type aggExport struct {
	BaseURL string            `json:"base_url"`
	APIKey  string            `json:"api_key"` // 真实运行态 key（仅本机用）
	Snippets map[string]string `json:"snippets"` // tool -> 配置片段文本
	Status  map[string]string `json:"status"`  // tool -> "synced" | "unsynced" | "missing"
}

// HandleAggregateExport GET /api/aggregate/export
// 返回聚合端口地址、真实 key，各工具配置片段，以及当前是否已同步的状态。
func HandleAggregateExport(c *gin.Context) {
	base := aggLocalBaseURL()
	key := aggregateAPIKey()
	snips := map[string]string{
		aggSyncCodex: codexAggSnippet(base, key),
		aggSyncDsh:   dshAggSnippet(base, key),
	}
	status := map[string]string{
		aggSyncCodex: toolSyncStatus(codexConfigPath(), base),
		aggSyncDsh:   toolSyncStatus(dshSettingsPath(), base),
	}
	c.JSON(http.StatusOK, aggExport{
		BaseURL:  base,
		APIKey:   key,
		Snippets: snips,
		Status:   status,
	})
}

// toolSyncStatus 探测某工具配置是否已指向聚合端口。
// 读文件内容，含聚合端口 base_url 视作已同步；文件不存在标注 missing。
func toolSyncStatus(path, base string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "missing"
	}
	if strings.Contains(string(data), base) {
		return "synced"
	}
	return "unsynced"
}

// aggSyncReq 一键同步请求。
type aggSyncReq struct {
	Tools []string `json:"tools"` // 如 ["codex","dsh"]
	Apply bool     `json:"apply"` // true=写回用户目录（带自动备份）；false=只返回片段
}

// aggSyncResult 单工具写回结果。
type aggSyncResult struct {
	Tool         string `json:"tool"`
	OK           bool   `json:"ok"`
	Path         string `json:"path,omitempty"`         // 写入/应写入的文件
	BackedUp     string `json:"backed_up,omitempty"`     // 本次写回的时间戳备份
	OrigBackedUp string `json:"orig_backed_up,omitempty"` // 用户原始配置备份（首次写回时存，还原用这份）
	Error        string `json:"error,omitempty"`
}

// HandleAggregateSync POST /api/aggregate/sync
// 生成片段并在 apply=true 时写回用户目录（先备份）。
func HandleAggregateSync(c *gin.Context) {
	var req aggSyncReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	if len(req.Tools) == 0 {
		req.Tools = []string{aggSyncCodex, aggSyncDsh}
	}
	base := aggLocalBaseURL()
	key := aggregateAPIKey()
	results := make([]aggSyncResult, 0, len(req.Tools))
	for _, t := range req.Tools {
		r := aggSyncResult{Tool: t}
		switch t {
		case aggSyncCodex:
						r.Path = codexConfigPath()
						if req.Apply {
							// 首次写回前，先存用户原始配置（只存一次），还原永远回这份
							if ob, err := ensureOrigBackup(r.Path); err == nil && ob != "" {
								r.OrigBackedUp = ob
							}
							bak, err := backupFile(r.Path)
							if err == nil && bak != "" {
								r.BackedUp = bak
							}
							// 只替换 [model_providers.openai-http] 段，不覆盖其他配置
							if err := applyCodexProvider(r.Path, base, key); err != nil {
								r.Error = err.Error()
							} else {
								r.OK = true
							}
			} else {
				r.OK = true // 仅预览
			}
		case aggSyncDsh:
			if err := applyDshConfig(base, key, req.Apply, &r); err != nil {
				r.Error = err.Error()
			}
		default:
			r.Error = "不支持的工具: " + t
		}
		results = append(results, r)
	}
	c.JSON(http.StatusOK, gin.H{"results": results})
}

// HandleAggregateRestore POST /api/aggregate/restore
// 还原：把指定工具恢复回用户原始配置（*.orig-bak，首次写回前存的那份）。
// 没有 orig-bak 说明从未同步过，无法还原。
func HandleAggregateRestore(c *gin.Context) {
	var req struct {
		Tools []string `json:"tools"`
	}
	_ = c.ShouldBindJSON(&req)
	if len(req.Tools) == 0 {
		req.Tools = []string{aggSyncCodex, aggSyncDsh}
	}
	results := make([]aggSyncResult, 0, len(req.Tools))
	for _, t := range req.Tools {
		r := aggSyncResult{Tool: t}
		var path string
		switch t {
		case aggSyncCodex:
			path = codexConfigPath()
		case aggSyncDsh:
			path = dshSettingsPath()
		default:
			r.Error = "不支持的工具: " + t
			results = append(results, r)
			continue
		}
		orig := path + ".orig-bak"
		if _, err := os.Stat(orig); err != nil {
			r.Error = "没有可还原的原始配置（从未同步过或原备份已删）"
			results = append(results, r)
			continue
		}
		if err := copyFile(orig, path); err != nil {
			r.Error = err.Error()
		} else {
			r.OK = true
			r.BackedUp = orig
			r.Path = path
		}
		results = append(results, r)
	}
	c.JSON(http.StatusOK, gin.H{"results": results})
}

// ===== 落点路径 =====

func codexConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex", "config.toml")
}

func dshSettingsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".dsh", "settings.yaml")
}

func dshCredentialsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".dsh", ".credentials.yaml")
}

// ===== 片段生成 =====

// aggLocalBaseURL 聚合端口地址（与前端硬编码 http://localhost:8080/v1 保持一致）。
func aggLocalBaseURL() string {
	if p := os.Getenv("PORT"); p != "" {
		return "http://localhost:" + p + "/v1"
	}
	return "http://localhost:8080/v1"
}

// codexAggSnippet 生成 codex openai-http provider 片段（TOML）。
// 关键字段从 codex 二进制确认：base_url / env_key / wire_api / requires_openai_auth。
func codexAggSnippet(base, key string) string {
	return fmt.Sprintf(`# Rescene 聚合端口（一键同步生成）
# 把下面 [model_providers.openai-http] 段合并进 ~/.codex/config.toml
[model_providers.openai-http]
name = "Rescene 聚合端口"
base_url = "%s"
wire_api = "chat"
env_key = "RESCENE_AGG_API_KEY"
requires_openai_auth = false
supports_websockets = false

[model_providers.openai-http.env]
RESCENE_AGG_API_KEY = "%s"
`, base, key)
}

// dshAggSnippet 生成 dsh settings.yaml 中 llm-pi-ai.providers.rescene 片段（YAML）。
func dshAggSnippet(base, key string) string {
	return fmt.Sprintf(`# Rescene 聚合端口（一键同步生成）
# 合并进 ~/.dsh/settings.yaml 的 llm-pi-ai.providers.rescene
llm-pi-ai:
  providers:
    rescene:
      apiKeyEnv: RESCENE_API_KEY
      api: openai-completions
      baseURL: %s
      models:
        - id: auto
agent-default-model:
  provider: rescene
  model: auto
`, base)
}

// ===== dsh 写回（YAML 局部替换，避免覆盖用户其他配置）=====

// applyDshConfig 把 rescene provider 的 baseURL 写进 dsh settings.yaml。
// 因 dsh 真实 key 来自 .credentials.yaml 的 RESCENE_API_KEY（与聚合端口 key 是两回事，
// 但 dsh 已用 RESCENE_API_KEY 调通，这里不改 key，只确保 baseURL 指向聚合端口）。
func applyDshConfig(base, key string, apply bool, r *aggSyncResult) error {
	r.Path = dshSettingsPath()
	if !apply {
		r.OK = true
		return nil
	}
	// 首次写回前先存用户原始配置
	if ob, err := ensureOrigBackup(r.Path); err == nil && ob != "" {
		r.OrigBackedUp = ob
	}
	bak, err := backupFile(r.Path)
	if err == nil && bak != "" {
		r.BackedUp = bak
	}
	// 读原配置，定位 llm-pi-ai.providers.rescene 段，改写 baseURL 行（保留其他配置）
	data, err := os.ReadFile(r.Path)
	if err != nil {
		r.Error = err.Error()
		return err
	}
	out := rewriteDshBaseURL(string(data), base)
	if err := os.WriteFile(r.Path, []byte(out), 0o644); err != nil {
		r.Error = err.Error()
		return err
	}
	// 同步写回 credentials，确保有 RESCENE_API_KEY
	if err := ensureDshCredentials(key); err != nil {
		r.Error = "key写入失败: " + err.Error()
		return err
	}
	r.OK = true
	return nil
}

// rewriteDshBaseURL 在 dsh settings.yaml 中替换 llm-pi-ai.providers 段为 rescene 聚合端口配置。
// dsh 实际使用行内大括号 YAML 格式，不是标准缩进。
func rewriteDshBaseURL(content, base string) string {
	lines := strings.Split(content, "\n")
	llmIdx := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "llm-pi-ai:" {
			llmIdx = i
			break
		}
	}
	if llmIdx < 0 {
		return content + "\nllm-pi-ai:\n  providers:\n    {\n      rescene:\n        {\n          apiKeyEnv: RESCENE_API_KEY,\n          api: openai-completions,\n          baseURL: " + base + ",\n          models: [ { id: auto } ]\n        }\n    }\n"
	}

	// 找到 providers: 行并替换整段到下一个顶层 key
	providersIdx := -1
	nextTopIdx := len(lines)
	for i := llmIdx + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "providers:" || strings.HasPrefix(trimmed, "providers:") {
			providersIdx = i
			continue
		}
		if providersIdx >= 0 && trimmed != "" && !strings.HasPrefix(lines[i], " ") && !strings.HasPrefix(lines[i], "	") {
			nextTopIdx = i
			break
		}
	}
	if providersIdx < 0 {
		lines = append(lines[:llmIdx+1], append([]string{"  providers:\n    {\n      rescene:\n        {\n          apiKeyEnv: RESCENE_API_KEY,\n          api: openai-completions,\n          baseURL: " + base + ",\n          models: [ { id: auto } ]\n        }\n    }"}, lines[llmIdx+1:]...)...)
		return strings.Join(lines, "\n")
	}

	// 替换 providers 块
	block := fmt.Sprintf(`  providers:
    {
      rescene:
        {
          apiKeyEnv: RESCENE_API_KEY,
          api: openai-completions,
          baseURL: %s,
          models: [ { id: auto } ]
        }
    }`, base)

	before := lines[:providersIdx]
	after := lines[nextTopIdx:]
	out := append(before, block)
	out = append(out, after...)
	return strings.Join(out, "\n")
}

// ===== 备份/还原工具 =====

// ensureDshCredentials 确保 .credentials.yaml 中有 RESCENE_API_KEY 条目，
// 值为聚合端口的真实 key（dsh settings.yaml 里 apiKeyEnv 指向这个变量名）。
func ensureDshCredentials(aggKey string) error {
	path := dshCredentialsPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		// 文件不存在则创建
		return os.WriteFile(path, []byte("RESCENE_API_KEY: "+aggKey+"\n"), 0o644)
	}
	content := string(data)
	if strings.Contains(content, "RESCENE_API_KEY:") {
		return nil // 已有，不变
	}
	// 追加到末尾
	content += "RESCENE_API_KEY: " + aggKey + "\n"
	return os.WriteFile(path, []byte(content), 0o644)
}

// applyCodexProvider 读 codex config.toml，替换 [model_providers.openai-http] 段为聚合端口配置。
// 只改该段，保留其他所有配置。
func applyCodexProvider(path, base, key string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		// 文件不存在则直接创建新文件
		return os.WriteFile(path, []byte(codexAggSnippet(base, key)), 0o644)
	}
	content := string(data)

	// 构建替换段文本
	aggBlock := fmt.Sprintf(`[model_providers.openai-http]
name = "Rescene 聚合端口"
base_url = "%s"
wire_api = "chat"
env_key = "RESCENE_AGG_API_KEY"
requires_openai_auth = false
supports_websockets = false

[model_providers.openai-http.env]
RESCENE_AGG_API_KEY = "%s"`, base, key)

	start := strings.Index(content, "[model_providers.openai-http]")
	if start < 0 {
		// 没有该段则追加
		content = content + "\n" + aggBlock + "\n"
	} else {
		// 从 [model_providers.openai-http] 到下一个 [section 或文件尾
		next := strings.Index(content[start+1:], "\n[")
		before := content[:start]
		var after string
		if next >= 0 {
			after = content[start+1+next:] // 保留下一个 section 的换行
		}
		content = before + aggBlock + "\n" + after
	}

	return os.WriteFile(path, []byte(content), 0o644)
}

func backupFile(path string) (string, error) {
	if _, err := os.Stat(path); err != nil {
		return "", nil // 文件不存在，无需备份
	}
	ts := time.Now().Format("20060102-150405")
	bak := path + ".bak-" + ts
	if err := copyFile(path, bak); err != nil {
		return "", err
	}
	return bak, nil
}

// ensureOrigBackup 首次写回前把用户当前配置存为「原始备份」（*.orig-bak）。
// 只存一次：已存在则跳过，返回已有路径。还原永远回到这份，而不是最近一次同步备份。
func ensureOrigBackup(path string) (string, error) {
	orig := path + ".orig-bak"
	if _, err := os.Stat(orig); err == nil {
		return orig, nil // 已存在，不覆盖
	}
	if _, err := os.Stat(path); err != nil {
		return "", nil // 原文件不存在，无需备份
	}
	if err := copyFile(path, orig); err != nil {
		return "", err
	}
	return orig, nil
}

