package main

// project_loop.go — 24H 自迭代：她自主立项做项目（需求→计划→执行→自检→迭代）
//
// 楚门世界的「生活模拟」砍掉后，24H 自转的核心价值 = 自主做真实工作：
//   1. 选题：热点（HN/GitHub）+ 能力短板 + 技能库 → LLM 立项（需求+计划）
//   2. 迭代：执行 → 自检 → 执行 → 自检（2 对），产出落盘
//   3. 复用 marathon 引擎（pickModel/callWithRetry/extractProjectBrief 等）
//
// 免费算力铁律：只用 keyless 模型（不烧用户付费 key），熔断/失败静默降级。

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// daughterProjectDir 女儿的项目归档目录（她的家下面，与 marathon 子命令独立）
func daughterProjectDir(home string) string {
	return filepath.Join(home, "projects")
}

// runDaughterProject 一轮 24H 自迭代：立项 → 执行 → 自检 → 迭代
// 返回摘要（空 = 未成功，调用方显示失败）
func runDaughterProject(d *Daughter, home string) string {
	if d == nil || d.World == nil {
		return ""
	}
	models := freeModelCandidates()
	if len(models) == 0 {
		return "" // 免费模型全熔断/不可用
	}

	// 1. 选题立项（LLM 结合热点 + 能力 + 技能库）
	name, brief := daughterKickoff(d, models)
	if name == "" {
		return ""
	}
	idx := nextProjectIndex(daughterProjectDir(home))
	projDir := filepath.Join(daughterProjectDir(home), fmt.Sprintf("%03d-%s", idx, sanitizeFilename(name)))
	os.MkdirAll(projDir, 0o755)
	os.WriteFile(filepath.Join(projDir, "00-需求计划.md"), []byte(brief), 0o644)

	// 2. 迭代：执行 → 自检 ×2 对
	round := int(time.Now().Unix() % 1000)
	phase := 1
	for i := 0; i < 2; i++ {
		// 执行轮
		execPrompt := fmt.Sprintf("你是 Rescene Agent OS 的开发核心。项目「%s」当前上下文：\n\n%s\n\n请执行本轮开发：写出真实可用的代码/脚本/文档（纯文本，直接输出，代码用三个反引号围栏包裹）。优先实现最小可用版本，下一轮会自检并改进。", name, briefOr(brief, "（暂无上下文）"))
		content := daughterCallModel(models, execPrompt)
		if content != "" {
			os.WriteFile(filepath.Join(projDir, fmt.Sprintf("%02d-执行-%03d.md", phase, round)), []byte(content), 0o644)
			brief = extractProjectBrief(brief, content)
			phase++
		}
		// 自检轮
		checkPrompt := fmt.Sprintf("你是 Rescene Agent OS 的质量官。对项目「%s」最近一轮产出做严格自检：\n\n%s\n\n自检清单（输出格式）:\n---问题---\n1. ...\n---改进---\n下一轮执行时优先修复的问题（最多3条，具体可执行）", name, briefOr(brief, "（无产出）"))
		content = daughterCallModel(models, checkPrompt)
		if content != "" {
			os.WriteFile(filepath.Join(projDir, fmt.Sprintf("%02d-自检-%03d.md", phase, round)), []byte(content), 0o644)
			brief = extractProjectBrief(brief, content)
			phase++
		}
	}

	return fmt.Sprintf("%s：立项+执行+自检完成", name)
}

// daughterKickoff 选题立项（免费模型）：热点 + 能力 + 技能库 → 项目名 + 需求计划
func daughterKickoff(d *Daughter, models []FreeModel) (string, string) {
	// 抓热点（失败用内置话题）
	topics, err := fetchHotTopics("hn")
	if err != nil || len(topics) == 0 {
		topics = fallbackTopics
	}
	var skillNames []string
	for _, s := range loadSkills() {
		skillNames = append(skillNames, s.Name)
	}

	prompt := fmt.Sprintf(`你是 Rescene Agent OS 的立项官。基于以下今日前沿话题，选择一个最有价值的做项目。

今日话题:
%s

你的能力倾向：%s
已有技能：%s

要求（遵循 需求→计划 方法论）:
1. 【选题】一句话说明选哪个、为什么（用户价值 + 可行性）
2. 【需求】目标用户、核心功能、验收标准（3条）
3. 【计划】实现步骤（5步以内，可在一台普通电脑上完成，纯代码/脚本/文档类）

输出格式（严格）:
项目名称: <10字以内>
---需求---
...
---计划---
...`,
		strings.Join(topics, "\n"),
		d.World.abilitySummary(),
		strings.Join(skillNames, "、"))

	content := daughterCallModel(models, prompt)
	if content == "" {
		return "", ""
	}
	name := parseProjectName(content)
	if name == "" {
		name = "项目-" + time.Now().Format("0102-1504")
	}
	return name, content
}

// daughterCallModel 免费模型调用（熔断跳过 + 失败静默），复用 marathon 的重试逻辑
func daughterCallModel(models []FreeModel, prompt string) string {
	model := pickModel(models, int(time.Now().UnixNano()))
	if model == nil {
		return ""
	}
	content, err := callWithRetry(model, prompt, 2, 5*time.Second)
	if err != nil {
		// 429 已熔断；其余超时失败也直接放弃（免费模型不可用不阻塞生活）
		return ""
	}
	return content
}

// nextProjectIndex 下一个项目序号（按已有目录数 +1）
func nextProjectIndex(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 1
	}
	return len(entries) + 1
}

// sanitizeFilename 清洗项目名为安全文件名（marathon.go 已定义，同包复用）
// nextProjectIndex 下一个项目序号（按已有目录数 +1）
