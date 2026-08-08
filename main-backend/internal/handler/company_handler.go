package handler

// company_handler.go — 公司管理面板 API（GUI 看百人公司运作）
//   GET /api/company/agents — 所有 agent 列表（含最近活动/产出数）
//   GET /api/company/agent?name=writer-01 — 单个 agent 详情

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// companyAgentInfo 单个 agent 信息
type companyAgentInfo struct {
	Name      string   `json:"name"`
	Role      string   `json:"role"`
	Home      string   `json:"home"`
	RecentLog string   `json:"recentLog,omitempty"`
	Outputs   int      `json:"outputs"`
	Skills    int      `json:"skills"`
	Files     []string `json:"files,omitempty"` // 产出文件名列表（可点开看内容）
}

// companyDir 公司目录
func companyDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "rescene_data", "company")
}

// HandleCompanyAgents GET /api/company/agents
func HandleCompanyAgents(c *gin.Context) {
	dir := companyDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"agents": []companyAgentInfo{}})
		return
	}
	var agents []companyAgentInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		info := companyAgentInfo{
			Name: e.Name(),
			Home: filepath.Join(dir, e.Name()),
		}
		// 角色：从名子解析（writer-01 → writer）
		parts := strings.SplitN(e.Name(), "-", 2)
		if len(parts) > 0 {
			info.Role = parts[0]
		}
		// live.log 尾部
		logPath := filepath.Join(dir, e.Name(), "live.log")
		if data, err := os.ReadFile(logPath); err == nil {
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			if len(lines) > 3 {
				info.RecentLog = strings.Join(lines[len(lines)-3:], "\n")
			} else {
				info.RecentLog = string(data)
			}
		}
		// 产出数 + 文件名列表
		outputDir := filepath.Join(dir, e.Name(), "outputs")
		if outEntries, err := os.ReadDir(outputDir); err == nil {
			for _, o := range outEntries {
				if !o.IsDir() && !strings.HasPrefix(o.Name(), "README") {
					info.Outputs++
					info.Files = append(info.Files, o.Name())
				}
			}
			if len(info.Files) > 8 {
				info.Files = info.Files[len(info.Files)-8:]
			}
		}
		// 技能数
		skillDir := filepath.Join(dir, e.Name(), "skills")
		if skillEntries, err := os.ReadDir(skillDir); err == nil {
			for _, s := range skillEntries {
				if !s.IsDir() && strings.HasSuffix(s.Name(), ".json") {
					info.Skills++
				}
			}
		}
		agents = append(agents, info)
	}
	c.JSON(http.StatusOK, gin.H{"agents": agents})
}

// HandleCompanyAgent GET /api/company/agent?name=writer-01
func HandleCompanyAgent(c *gin.Context) {
	name := c.Query("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name 参数必填"})
		return
	}
	home := filepath.Join(companyDir(), name)
	if _, err := os.Stat(home); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent 不存在"})
		return
	}
	info := companyAgentInfo{Name: name, Home: home}
	parts := strings.SplitN(name, "-", 2)
	if len(parts) > 0 {
		info.Role = parts[0]
	}
	// 活动日志
	logPath := filepath.Join(home, "live.log")
	if data, err := os.ReadFile(logPath); err == nil {
		info.RecentLog = string(data)
	}
	// 产出
	outputDir := filepath.Join(home, "outputs")
	if outEntries, err := os.ReadDir(outputDir); err == nil {
		for _, o := range outEntries {
			if !o.IsDir() {
				info.Outputs++
			}
		}
	}
	// 技能
	skillDir := filepath.Join(home, "skills")
	if skillEntries, err := os.ReadDir(skillDir); err == nil {
		for _, s := range skillEntries {
			if !s.IsDir() && strings.HasSuffix(s.Name(), ".json") {
				info.Skills++
			}
		}
	}
	_ = time.Now()
	_ = json.Valid
	c.JSON(http.StatusOK, info)
}