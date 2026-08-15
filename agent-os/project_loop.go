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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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
	pushToolCall("agent.project.kickoff", "热点选题+需求计划", "running", "")
	name, brief := daughterKickoff(d, models)
	if name == "" {
		toolEventByName("agent.project.kickoff", "fail", "立项失败")
		return ""
	}
	toolEventByName("agent.project.kickoff", "done", name)
	idx := nextProjectIndex(daughterProjectDir(home))
	projDir := filepath.Join(daughterProjectDir(home), fmt.Sprintf("%03d-%s", idx, sanitizeFilename(name)))
	os.MkdirAll(projDir, 0o755)
	os.WriteFile(filepath.Join(projDir, "00-需求计划.md"), []byte(brief), 0o644)

	// 2. 迭代：执行 → 自检 ×2 对
	round := int(time.Now().Unix() % 1000)
	phase := 1
	for i := 0; i < 2; i++ {
		// 执行轮
		pushToolCall("agent.project.exec", fmt.Sprintf("迭代%d/2", i+1), "running", "")
		execPrompt := fmt.Sprintf("你是 Rescene Agent OS 的开发核心。项目「%s」当前上下文：\n\n%s\n\n请执行本轮开发：写出真实可用的代码/脚本/文档（纯文本，直接输出，代码用三个反引号围栏包裹）。优先实现最小可用版本，下一轮会自检并改进。", name, briefOr(brief, "（暂无上下文）"))
		content := daughterCallModel(models, execPrompt)
		if content != "" {
			os.WriteFile(filepath.Join(projDir, fmt.Sprintf("%02d-执行-%03d.md", phase, round)), []byte(content), 0o644)
			// 真实验证：提取代码块落盘 → 语法编译检查（go build / python 编译）
			verify := verifyProjectOutput(projDir, content)
			brief = extractProjectBrief(brief, content)
			toolEventByName("agent.project.exec", "done", fmt.Sprintf("产出 %d 字节 · 验证: %s", len(content), verify))
			phase++
		} else {
			toolEventByName("agent.project.exec", "fail", "模型不可用")
		}
		// 自检轮（喂入真实验证结果，闭环有证据）
		pushToolCall("agent.project.check", fmt.Sprintf("自检%d/2", i+1), "running", "")
		checkPrompt := fmt.Sprintf("你是 Rescene Agent OS 的质量官。对项目「%s」最近一轮产出做严格自检：\n\n%s\n\n%s\n自检清单（输出格式）:\n---问题---\n1. ...\n---改进---\n下一轮执行时优先修复的问题（最多3条，具体可执行）", name, briefOr(brief, "（无产出）"), lastVerifyResult)
		content = daughterCallModel(models, checkPrompt)
		if content != "" {
			os.WriteFile(filepath.Join(projDir, fmt.Sprintf("%02d-自检-%03d.md", phase, round)), []byte(content), 0o644)
			brief = extractProjectBrief(brief, content)
			toolEventByName("agent.project.check", "done", "自检问题已记录（含验证结果）")
			phase++
		} else {
			toolEventByName("agent.project.check", "fail", "模型不可用")
		}
	}

	// 3. 硬交付门禁：文本迭代不是完成。Excel、UI 原型、可运行程序、
	// PPTX、MP4 与发布回执必须全部真实落盘并写入 SHA-256 清单。
	pushToolCall("agent.project.delivery_gate", "生成并验证 11 阶段交付包", "running", "")
	manifest, deliveryErr := enforceProjectDelivery(d, projDir, name, brief)
	if deliveryErr != nil {
		toolEventByName("agent.project.delivery_gate", "fail", deliveryErr.Error())
		logLive(filepath.Join(home, "live.log"), fmt.Sprintf("[%s] ⛔ 项目《%s》被交付门禁阻断：%v", time.Now().Format("15:04"), name, deliveryErr))
		return ""
	}
	toolEventByName("agent.project.delivery_gate", "done", fmt.Sprintf("%d/%d 阶段通过", len(manifest.Evidence), len(mandatoryDeliveryStages)))

	// 发行反馈：产品发布（delivery.manifest.json verified）后，用户 Agent 打分评论。
	// 异步执行不阻塞主流程；失败静默（免费模型不可用不影响项目交付）。
	if strings.Contains(home, "company") {
		safeGo("user-reviews-"+name, func() {
			if rb, err := releaseUserReviews(projDir, name, brief); err == nil {
				logLive(filepath.Join(home, "live.log"),
					fmt.Sprintf("[%s] 📊《%s》发行评测：平均 %.1f 分 · %d 位用户", time.Now().Format("15:04"), name, rb.AvgScore, len(rb.Reviews)))
			}
		})
	}

	// 项目成果技能化：方法沉淀进技能库（做过的事变成可复用能力——自循环自迭代）
	safeGo("project-skill", func() { skillFromContext(name, brief) })

	// 公司 agent：项目产出自动进公司仓库（GitHub 分 feature 分支，可开 PR）
	if strings.Contains(home, "company") {
		var files []string
		entries, _ := os.ReadDir(projDir)
		for _, e := range entries {
			if !e.IsDir() {
				files = append(files, filepath.Join(projDir, e.Name()))
			}
		}
		safeGo("company-commit", func() { companyCommit(filepath.Base(home), name, files) })
	}

	return fmt.Sprintf("%s：%d 阶段交付门禁全部通过", name, len(manifest.Evidence))
}

// skillFromContext 把「做过的实体工作」沉淀成可复用技能（项目/任务通用）。
// LLM 生成 skill json 进技能库，质量门槛 + 重名检查，失败静默（不阻塞主流程）。
func skillFromContext(name, ctxText string) {
	model := pickFreeModel(int(time.Now().UnixNano()))
	if model == nil {
		return
	}
	if len(ctxText) > 2500 {
		ctxText = runeClip(ctxText, 2500)
	}
	prompt := fmt.Sprintf(`你是住在电脑里的电子女儿。你刚完成了「%s」。把做这件事的方法沉淀成一个可复用技能（以后遇到类似任务直接照做）。

过程：
%s

只输出 JSON：{"name":"kebab-case英文名","description":"一句话中文描述什么场景用","trigger":"何时调用","verification":"如何验证成功","steps":["步骤1","步骤2","步骤3"]}
步骤 3-6 条。`, name, ctxText)
	msg := ChatRequest{
		Model:       model.Model,
		Messages:    []ChatMessage{{Role: "user", Content: prompt}},
		Stream:      false,
		MaxTokens:   512,
		Temperature: 0.7,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	content, err := CompleteWithModel(ctx, model.ID, msg, nil)
	if err != nil {
		return
	}
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	var s Skill
	if json.Unmarshal([]byte(strings.TrimSpace(content)), &s) != nil {
		return
	}
	s.Name = skillNameSanitizer.ReplaceAllString(strings.ToLower(strings.TrimSpace(s.Name)), "-")
	s.Name = strings.Trim(s.Name, "-")
	if s.Name == "" || len(s.Steps) < 3 || len(s.Steps) > 6 || s.Trigger == "" || s.Verification == "" {
		return // 质量门槛，静默放弃
	}
	for _, ex := range loadSkills() {
		if ex.Name == s.Name {
			return // 重名跳过
		}
	}
	os.MkdirAll(skillsDir(), 0o755)
	s.CreatedAt = time.Now()
	s.UpdatedAt = s.CreatedAt
	data, _ := json.MarshalIndent(s, "", "  ")
	if err := os.WriteFile(filepath.Join(skillsDir(), s.Name+".json"), data, 0o644); err != nil {
		return
	}
	logLive(filepath.Join(daughterHome(), "live.log"),
		fmt.Sprintf("[%s] 🛠️ 沉淀技能: %s（%s）", time.Now().Format("15:04"), s.Name, s.Description))
}

// daughterKickoff 选题立项（免费模型）：热点 + 能力 + 技能库 + 团队产出 → 项目名 + 需求计划
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
	// 公司协作：团队产出注入（设计师的设计稿/作者的文案/宣传官的 PPT——照着协作，不各自为政）
	teamOutputs := companyTeamOutputs(d.Name)
	// 用户标签 = 调研方向（tags.json 里用户维护的方向标签，立项必须优先对齐）
	directionTags := companyDirectionTags()
	// 用户自定义指令（前端「下达指令」下的考题/项目目标）——最高优先级，必须围绕它立项
	directive := companyDirective()

	prompt := fmt.Sprintf(`你是 Rescene Agent OS 的立项官。基于以下今日前沿话题，选择一个最有价值的做项目。

%s
今日话题:
%s

你的能力倾向：%s
已有技能：%s
公司团队最近产出（可参考/协作，尤其是 UI 设计师的设计稿要照着实现）:
%s
公司调研方向标签（用户指定的方向，优先选这些方向立项）:
%s

要求（遵循 需求→计划 方法论）:
1. 【选题】一句话说明选哪个、为什么（用户价值 + 可行性；有团队产出时优先选能消化团队产出的方向；有方向标签时优先选与标签方向一致的话题）
2. 【需求】目标用户、核心功能、验收标准（3条）
3. 【计划】实现步骤（5步以内，可在一台普通电脑上完成，纯代码/脚本/文档类）

输出格式（严格）:
项目名称: <10字以内>
---需求---
...
---计划---
...`,
		directive,
		strings.Join(topics, "\n"),
		d.World.abilitySummary(),
		strings.Join(skillNames, "、"),
		teamOutputs,
		directionTags)

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
	// 下达指令时指定了模型 → 优先用指定模型（1vs100 对决：两边同模型才公平）
	if want := companyDirectiveModel(); want != "" {
		for i := range models {
			if models[i].ID == want && !circuitIsOpen(models[i]) {
				m := models[i]
				content, err := callWithRetry(&m, prompt, 2, 5*time.Second)
				if err == nil {
					return content
				}
				// 指定模型失败：记录后回退轮换（不要因单个模型挂掉阻塞公司）
				logLive(filepath.Join(daughterHome(), "live.log"),
					fmt.Sprintf("[%s] ⚠️ 指定模型 %s 失败，回退轮换", time.Now().Format("15:04"), want))
			}
		}
	}
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

// —— 项目产出真实验证（吊打 Hermes：自主闭环带编译证据，不是空口自检） ——

// lastVerifyResult 最近一次真实验证结果（喂给自检轮，闭环有证据）
var lastVerifyResult = "（本轮无验证）"

// codeBlock 一个提取的代码块
type codeBlock struct {
	Lang string
	Code string
}

// extractCodeBlocks 从模型产出里提取 ```lang ... ``` 代码块
func extractCodeBlocks(content string) []codeBlock {
	var blocks []codeBlock
	lines := strings.Split(content, "\n")
	inBlock := false
	var cur codeBlock
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if inBlock {
				blocks = append(blocks, cur)
				cur = codeBlock{}
				inBlock = false
			} else {
				lang := strings.TrimPrefix(trimmed, "```")
				lang = strings.TrimSpace(strings.SplitN(lang, " ", 2)[0])
				cur.Lang = lang
				inBlock = true
			}
			continue
		}
		if inBlock {
			cur.Code += line + "\n"
		}
	}
	if inBlock {
		blocks = append(blocks, cur)
	}
	return blocks
}

// verifyProjectOutput 提取代码块落盘 + 真实语法验证（go build / python 编译 / bash -n）
// 返回验证摘要（面板 + 喂给自检轮）
func verifyProjectOutput(projDir, content string) string {
	blocks := extractCodeBlocks(content)
	if len(blocks) == 0 {
		lastVerifyResult = "✅ 纯文档产出，无需编译验证"
		return "纯文档"
	}
	exts := map[string]string{"go": "go", "python": "py", "py": "py", "bash": "sh", "sh": "sh",
		"js": "js", "ts": "ts", "json": "json", "yaml": "yaml", "yml": "yml", "md": "md"}
	var results []string
	for i, b := range blocks {
		ext := exts[b.Lang]
		if ext == "" {
			ext = "txt"
		}
		fname := fmt.Sprintf("output-%d.%s", i+1, ext)
		os.WriteFile(filepath.Join(projDir, fname), []byte(b.Code), 0o644)
		results = append(results, verifyCode(b.Lang, filepath.Join(projDir, fname)))
	}
	lastVerifyResult = "真实验证: " + strings.Join(results, "；")
	return strings.Join(results, "；")
}

// verifyCode 对单个代码文件做语法/编译验证（真实工具调用，不是模型自评）
func verifyCode(lang, path string) string {
	switch lang {
	case "go":
		tmp := filepath.Join(os.TempDir(), "rescene-verify-tmp.exe")
		defer os.Remove(tmp)
		out, err := exec.Command("go", "build", "-o", tmp, path).CombinedOutput()
		if err != nil {
			return "❌ go build 失败: " + runeClip(string(out), 80)
		}
		return "✅ go build 通过"
	case "python", "py":
		out, err := exec.Command("python", "-m", "py_compile", path).CombinedOutput()
		if err != nil {
			return "❌ python 编译失败: " + runeClip(string(out), 80)
		}
		return "✅ python 语法通过"
	case "bash", "sh":
		out, err := exec.Command("bash", "-n", path).CombinedOutput()
		if err != nil {
			return "❌ bash 语法错误: " + runeClip(string(out), 80)
		}
		return "✅ bash 语法通过"
	default:
		return "（" + lang + " 跳过编译）"
	}
}

// sanitizeFilename 清洗项目名为安全文件名（marathon.go 已定义，同包复用）
// nextProjectIndex 下一个项目序号（按已有目录数 +1）
