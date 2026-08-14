package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ===== 迭代计划（从已审批项目选，每天调研前沿技术迭代产品）=====

// iteratePlan 一次迭代计划
type iteratePlan struct {
	Project    string `json:"project"`              // 规范化项目名
	Name       string `json:"name"`                 // 展示名
	StartedAt  string `json:"startedAt"`            // 开始时间
	LastReport string `json:"lastReport,omitempty"` // 最近报告摘要
	ReportFile string `json:"reportFile,omitempty"` // 最近报告文件路径
}

// iteratePlanPath 持久化路径
func iteratePlanPath() string {
	return filepath.Join(companyDir(), "iterate_plan.json")
}

// loadIteratePlans 读磁盘
func loadIteratePlans() []iteratePlan {
	data, err := os.ReadFile(iteratePlanPath())
	if err != nil {
		return nil
	}
	var plans []iteratePlan
	json.Unmarshal(data, &plans)
	return plans
}

// saveIteratePlans 写磁盘
func saveIteratePlans(plans []iteratePlan) {
	data, _ := json.MarshalIndent(plans, "", "  ")
	os.WriteFile(iteratePlanPath(), data, 0644)
}

// iterateReportDir 迭代报告目录
func iterateReportDir(project string) string {
	return filepath.Join(companyDir(), "iterate", project)
}

// iterateCandidate 一个候选项目（已审批、未加入迭代）
type iterateCandidate struct {
	Project string `json:"project"`
	Name    string `json:"name"`
}

// collectCandidates 收集已审批但未加入迭代的项目
func collectCandidates() []iterateCandidate {
	recs := loadApprovals()
	planned := loadIteratePlans()
	plannedSet := map[string]bool{}
	for _, p := range planned {
		plannedSet[p.Project] = true
	}
	seen := map[string]bool{}
	var candidates []iterateCandidate
	for _, r := range recs {
		if r.Decision != "approve" {
			continue
		}
		proj := strings.ToLower(strings.TrimSpace(r.Project))
		if proj == "" || plannedSet[proj] || seen[proj] {
			continue
		}
		seen[proj] = true
		name := proj
		// 尝试从 .approvals.json 之外的路径找展示名
		candidates = append(candidates, iterateCandidate{Project: proj, Name: name})
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Project < candidates[j].Project })
	return candidates
}

// HandleCompanyIterate GET /api/company/iterate — 查看迭代计划 + 候选项目
func HandleCompanyIterate(c *gin.Context) {
	plans := loadIteratePlans()
	// 最近报告摘要随行返回
	for i := range plans {
		if plans[i].ReportFile != "" {
			if data, err := os.ReadFile(plans[i].ReportFile); err == nil {
				lines := strings.SplitN(string(data), "\n", 4)
				summary := strings.TrimSpace(lines[0])
				if len(lines) > 1 {
					summary += " | " + strings.TrimSpace(lines[1])
				}
				if len(summary) > 200 {
					summary = summary[:200] + "…"
				}
				plans[i].LastReport = summary
			}
		}
	}
	candidates := collectCandidates()
	c.JSON(http.StatusOK, gin.H{
		"plans":      plans,
		"candidates": candidates,
	})
}

// HandleCompanyIterateStop POST /api/company/iterate/stop — 停止迭代一个项目
func HandleCompanyIterateStop(c *gin.Context) {
	var req struct {
		Project string `json:"project"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Project == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请指定项目"})
		return
	}
	proj := strings.ToLower(strings.TrimSpace(req.Project))
	plans := loadIteratePlans()
	var kept []iteratePlan
	found := false
	for _, p := range plans {
		if p.Project == proj {
			found = true
			continue
		}
		kept = append(kept, p)
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "该项目不在迭代计划中"})
		return
	}
	saveIteratePlans(kept)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
func HandleCompanyIterateStart(c *gin.Context) {
	var req struct {
		Project string `json:"project"`
		Name    string `json:"name,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Project == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请指定项目"})
		return
	}
	proj := strings.ToLower(strings.TrimSpace(req.Project))
	plans := loadIteratePlans()
	for _, p := range plans {
		if p.Project == proj {
			c.JSON(http.StatusConflict, gin.H{"error": "该项目已在迭代计划中"})
			return
		}
	}
	name := req.Name
	if name == "" {
		name = proj
	}
	plan := iteratePlan{
		Project:   proj,
		Name:      name,
		StartedAt: time.Now().Format(time.RFC3339),
	}
	// 立即生成第一份迭代报告
	report, err := generateIterateReport(proj, name)
	if err != nil {
		// 报告生成失败不影响计划，记录错误
		plan.LastReport = fmt.Sprintf("首次调研失败: %v", err)
	} else {
		dir := iterateReportDir(proj)
		os.MkdirAll(dir, 0755)
		fname := fmt.Sprintf("报告-%s.md", time.Now().Format("2006-01-02"))
		fpath := filepath.Join(dir, fname)
		os.WriteFile(fpath, []byte(report), 0644)
		plan.ReportFile = fpath
		lines := strings.SplitN(report, "\n", 2)
		summary := strings.TrimSpace(lines[0])
		if len(summary) > 120 {
			summary = summary[:120] + "…"
		}
		plan.LastReport = summary
	}
	plans = append(plans, plan)
	saveIteratePlans(plans)
	// 注册每日 cron 调研（TODO：后续接调度器，目前用 POST 手动触发）
	c.JSON(http.StatusAccepted, plan)
}

// generateIterateReport 用 LLM 生成前沿技术调研 + 迭代决策
func generateIterateReport(project, name string) (string, error) {
	// 读取项目已有的产出/目标信息
	homeDir, _ := os.UserHomeDir()
	ctx := ""
	goalDir := filepath.Join(homeDir, "rescene_data", "company_workflows")
	if ents, err := os.ReadDir(goalDir); err == nil {
		for _, e := range ents {
			if strings.Contains(strings.ToLower(e.Name()), project) {
				if data, err := os.ReadFile(filepath.Join(goalDir, e.Name())); err == nil {
					ctx = string(data)
					if len(ctx) > 2000 {
						ctx = ctx[:2000]
					}
				}
				break
			}
		}
	}
	prompt := fmt.Sprintf(`你是一个前沿技术调研 Agent，负责每天为产品迭代收集最新技术动态。

目标项目：%s
项目信息：
%s

请生成一份「前沿技术迭代报告」，包含：
1. 今天日期和项目名称
2. 该领域当前最前沿的技术突破（3-5条，说明技术名称、来源、对项目的影响）
3. 针对该项目的具体迭代建议（2-3条，可操作的技术方案）
4. 推荐关注的开源项目/论文/工具（1-2条）

格式：用 Markdown，中文字号，第一行标题为「【前沿技术迭代】%s - 2026-08-12」`, name, ctx, name)
	content, err := callLocalAggregate(prompt)
	if err != nil {
		return "", fmt.Errorf("LLM 调研失败: %w", err)
	}
	return content, nil
}