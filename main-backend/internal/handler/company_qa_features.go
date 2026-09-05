package handler

import (
	"os"
	"path/filepath"
	"strings"
)

// companyQAMissingFeatures 对比指令与产物，找出「指令明确要求但产物里看不到」的核心功能。
// 扫描项目**全部**源文件（index.html + app.js + styles.css + data.js + output-app.html），
// 只看 HTML 入口会漏掉多文件项目的真实逻辑（功能全在 app.js/data.js 里）。
func companyQAMissingFeatures(brief, content, research string, projectDir string) []string {
	files := []string{content}
	// 多文件项目：扫全部源文件（index.html + app.js + styles.css + data.js + output-app.html）
	if projectDir != "" {
		for _, name := range []string{"app.js", "data.js", "styles.css", "index.html", "output-app.html"} {
			if data, err := os.ReadFile(filepath.Join(projectDir, name)); err == nil {
				files = append(files, string(data))
			}
		}
	}
	allContent := strings.Join(files, "\n")

	checks := inferFeaturesFromBrief(brief)
	lower := strings.ToLower(allContent)

	var missing []string
	for _, c := range checks {
		hit := false
		for _, t := range c.terms {
			if strings.Contains(lower, strings.ToLower(t)) {
				hit = true
				break
			}
		}
		if !hit {
			missing = append(missing, c.name)
		}
	}
	return missing
}

// inferFeaturesFromBrief 从指令文本里提取核心功能点（按常见产品模式匹配）。
// 用户指令通常是「做一个X，要能A、能B、能C」，按关键词映射到具体功能。
func inferFeaturesFromBrief(brief string) []struct {
	name  string
	terms []string
} {
	low := strings.ToLower(brief)
	var out []struct {
		name  string
		terms []string
	}
	// 通用模式：收支/记账 → 必须有收入/支出分类+添加+列表
	if strings.Contains(low, "收支") || strings.Contains(low, "记账") || strings.Contains(low, "记一笔") {
		out = append(out, struct{ name string; terms []string }{
			name:  "收支类型（收入/支出分类）",
			terms: []string{"收入", "支出"},
		})
		out = append(out, struct{ name string; terms []string }{
			name:  "添加记账记录",
			terms: []string{"添加", "新增", "记一笔"},
		})
	}
	// 按月/按月份/统计 → 必须有月度汇总区
	if strings.Contains(low, "按月") || strings.Contains(low, "月份") || strings.Contains(low, "统计") {
		out = append(out, struct{ name string; terms []string }{
			name:  "月度统计/合计展示",
			terms: []string{"月度", "月份", "合计", "统计"},
		})
	}
	// 分类/类别 → 必须有分类筛选
	if strings.Contains(low, "分类") || strings.Contains(low, "类别") {
		out = append(out, struct{ name string; terms []string }{
			name:  "分类筛选",
			terms: []string{"分类", "类别"},
		})
	}
	// 持久化/保存/刷新不丢 → 必须有本地存储实现
	if strings.Contains(low, "保存") || strings.Contains(low, "持久") || strings.Contains(low, "刷新不丢") || strings.Contains(low, "数据") {
		out = append(out, struct{ name string; terms []string }{
			name:  "本地存储持久化",
			terms: []string{"localStorage"},
		})
	}
	// 编辑/删除 → 必须有数据操作
	if strings.Contains(low, "编辑") || strings.Contains(low, "删除") || strings.Contains(low, "管理") {
		out = append(out, struct{ name string; terms []string }{
			name:  "数据编辑/删除操作",
			terms: []string{"编辑", "删除"},
		})
	}
	// 图表/可视化 → 必须有图形展示
	if strings.Contains(low, "图表") || strings.Contains(low, "可视化") || strings.Contains(low, "走势") {
		out = append(out, struct{ name string; terms []string }{
			name:  "图表可视化",
			terms: []string{"chart", "图表", "canvas", "svg"},
		})
	}
	// 登录/注册/账号 → 必须有登录态
	if strings.Contains(low, "登录") || strings.Contains(low, "注册") || strings.Contains(low, "账号") || strings.Contains(low, "用户") {
		out = append(out, struct{ name string; terms []string }{
			name:  "登录/注册",
			terms: []string{"登录", "注册", "密码"},
		})
	}
	// 待办/任务 → 必须有增删改查
	if strings.Contains(low, "待办") || strings.Contains(low, "任务") || strings.Contains(low, "清单") {
		out = append(out, struct{ name string; terms []string }{
			name:  "待办增删改查",
			terms: []string{"待办", "任务", "完成", "勾选"},
		})
	}
	// 番茄钟/计时 → 必须有计时器
	if strings.Contains(low, "番茄") || strings.Contains(low, "计时") || strings.Contains(low, "倒计时") {
		out = append(out, struct{ name string; terms []string }{
			name:  "计时器运行",
			terms: []string{"计时", "倒计时", "开始", "暂停"},
		})
	}
	return out
}

// companyQAMissingFeaturesPrompt 把缺失功能列表缝成返修工单的一段。
func companyQAMissingFeaturesPrompt(missing []string) string {
	if len(missing) == 0 {
		return ""
	}
	lines := []string{
		"",
		"【指令功能缺失清单】对比用户指令，以下核心功能在产物中完全看不到，必须逐条补全：",
	}
	for _, m := range missing {
		lines = append(lines, "- 缺失功能："+m)
	}
	lines = append(lines, "这些是用户明确要求的功能，不是锦上添花。缺任何一条都是不合格交付。")
	return strings.Join(lines, "\n")
}
