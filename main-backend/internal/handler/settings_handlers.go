package handler

// 设置面板只读/配置端点：技能库、MCP 生态、用户档案（含自定义指令）。
//   GET  /api/skills   —— 列出内置、已学与外部技能
//   GET  /api/mcp      —— 列出已配置的 MCP server 与运行时注册的工具
//   GET  /api/profile  —— 读取用户档案 + 自定义指令
//   POST /api/profile  —— 保存用户档案 + 自定义指令

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// ---------------- 技能库 ----------------

// HandleListSkills GET /api/skills —— 返回完整技能库存：候选/启用/归档的自研技能
// 与外部导入两类，设置页据此提供类似 Hermes 的审阅和恢复动作。
// 每条带 source 字段（learned / external）供前端区分展示。
func HandleListSkills(c *gin.Context) {
	skills := loadBuiltinSkills()
	skills = append(skills, loadLearnedSkillsForSettings()...)
	skills = append(skills, loadExternalSkills()...)
	if skills == nil {
		skills = make([]Skill, 0)
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	c.JSON(http.StatusOK, gin.H{
		"skills":  skills,
		"dir":     skillsDir(),
		"ext_dir": externalSkillsDir(),
	})
}

func HandleUpdateSkillStatus(c *gin.Context) {
	var body struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体不是合法 JSON"})
		return
	}
	s, err := setLearnedSkillStatus(c.Param("name"), body.Status)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"skill": s})
}

func HandleDeleteSkill(c *gin.Context) {
	if err := deleteLearnedSkill(c.Param("name")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ---------------- MCP 生态 ----------------

type mcpServerView struct {
	Name         string   `json:"name"`
	Command      string   `json:"command,omitempty"`
	Args         []string `json:"args,omitempty"`
	URL          string   `json:"url,omitempty"`
	Transport    string   `json:"transport"`
	Source       string   `json:"source,omitempty"`
	RegistryName string   `json:"registry_name,omitempty"`
	Status       string   `json:"status"`
	Tools        []string `json:"tools"` // 该 server 运行时注册的完整工具名
}

// HandleMCPStatus GET /api/mcp —— 读 mcp.json 配置 + 运行时已注册工具。
func HandleMCPStatus(c *gin.Context) {
	// 触发懒初始化，确保 mcpRoutes 已填充（无配置时静默为空）
	loadMCPToolDefs()

	servers := make([]mcpServerView, 0)
	path := mcpConfigPath()
	if data, err := os.ReadFile(path); err == nil {
		var cfg mcpConfig
		if json.Unmarshal(data, &cfg) == nil {
			for name, sc := range cfg.Servers {
				transport := "stdio"
				if sc.URL != "" {
					transport = "streamable-http"
				}
				view := mcpServerView{
					Name: name, Command: sc.Command, Args: sc.Args, URL: sc.URL,
					Transport: transport, Source: sc.Source, RegistryName: sc.RegistryName,
					Status: "unavailable", Tools: []string{},
				}
				prefix := "mcp__" + name + "__"
				for full := range mcpRoutes {
					if strings.HasPrefix(full, prefix) {
						view.Tools = append(view.Tools, full)
					}
				}
				sort.Strings(view.Tools)
				if mcpConns[name] != nil {
					view.Status = "connected"
				}
				servers = append(servers, view)
			}
		}
	}
	sort.Slice(servers, func(i, j int) bool { return servers[i].Name < servers[j].Name })
	c.JSON(http.StatusOK, gin.H{"servers": servers, "config_path": path})
}

// ---------------- 用户档案 + 自定义指令 ----------------

type UserProfile struct {
	FullName     string `json:"full_name"`
	Work         string `json:"work"`
	Instructions string `json:"instructions"`
}

var profileMu sync.Mutex

func profileFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(homeDir, "rescene_data")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "profile.json"), nil
}

// loadUserProfile 读用户档案；文件不存在时返回零值。供 HTTP 与提示词注入共用。
func loadUserProfile() UserProfile {
	profileMu.Lock()
	defer profileMu.Unlock()
	var p UserProfile
	path, err := profileFilePath()
	if err != nil {
		return p
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return p
	}
	_ = json.Unmarshal(data, &p)
	return p
}

// HandleGetProfile GET /api/profile
func HandleGetProfile(c *gin.Context) {
	c.JSON(http.StatusOK, loadUserProfile())
}

// HandleSaveProfile POST /api/profile
func HandleSaveProfile(c *gin.Context) {
	var p UserProfile
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体不是合法 JSON"})
		return
	}
	profileMu.Lock()
	defer profileMu.Unlock()
	path, err := profileFilePath()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	data, _ := json.MarshalIndent(p, "", "  ")
	if err := os.WriteFile(path, data, 0600); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// userInstructionsPrompt 把用户档案整理成系统提示词片段；空字段不产出文本。
// 昵称(full_name)即用户对 AI 的自我身份表达（AI 用它称呼用户），身份(work)
// 与自定义指令(instructions)一并注入。所有字段空则整体返回空串。
// 该片段在四态机主链路 HandleCodeWorkflow 与 chat_stream（已废弃）都注入；
// 后端 SoulTemplateBase 只给中性助手基底，身份完全由此处 profile 驱动。
func userInstructionsPrompt() string {
	p := loadUserProfile()
	var b strings.Builder
	if s := strings.TrimSpace(p.FullName); s != "" {
		b.WriteString("\n用户的昵称是：" + s + "，请用这个称呼他/她。")
	}
	if s := strings.TrimSpace(p.Work); s != "" {
		b.WriteString("\n用户的职业/身份：" + s + "。")
	}
	if s := strings.TrimSpace(p.Instructions); s != "" {
		b.WriteString("\n\n用户的自定义指令（请始终遵循）：\n" + s)
	}
	return b.String()
}
