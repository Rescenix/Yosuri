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
