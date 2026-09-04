package handler

// summary_pipeline.go —— 派生记忆摘要视图。
//
// 铁律：事实永远不删。本文件只「读」真相源（facts.json + 手动 md + 亲密度），
// 「写」一个派生文件 memory/summary.md——给前端看的高可读摘要。
// 真相源、index.md、云端同步白名单一概不动；agent 召回链路零变化。
//
// summary.md 不进 SyncableFiles（派生视图不参与跨设备同步，避免覆盖真相源）。

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"backend/internal/memorydir"
)

var summaryPipeline = struct {
	sync.Mutex
	timer *time.Timer
}{}

// summaryPath 派生摘要文件路径。
func summaryPath() string {
	dir := automaticMemoryDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "summary.md")
}

// collectSummarySources 收集蒸馏原料（全部只读）：facts + 手动记忆 + 亲密度。
func collectSummarySources() string {
	var b strings.Builder

	automaticMemory.Lock()
	facts, _ := loadAutomaticFacts()
	automaticMemory.Unlock()
	if len(facts) > 0 {
		b.WriteString("<facts>\n")
		for _, f := range facts {
			fmt.Fprintf(&b, "%s | %s | %s\n", f.Category, f.Key, f.Value)
		}
		b.WriteString("</facts>\n")
	}

	// 手动记忆文件（用户/agent 显式写的，权重高于自动提取）
	for _, name := range []string{"preferences", "project", "decisions", "memories", "pinned"} {
		content := memorydir.ReadRaw(name)
		content = strings.TrimSpace(content)
		if content == "" {
			continue
		}
		if len([]rune(content)) > 600 {
			content = string([]rune(content)[:600]) + "…"
		}
		fmt.Fprintf(&b, "<manual file=%q>\n%s\n</manual>\n", name, content)
	}

	_, intimacyVal := memorydir.ReadIntimacy()
	level := memorydir.IntimacyLevel(intimacyVal)
	fmt.Fprintf(&b, "<intimacy level=%d/>\n", level)

	return b.String()
}

// distillSummary 蒸馏派生摘要：LLM 把真相源整理成高密度人类可读的 summary.md。
// 只读真相源、只写 summary.md——事实永远不删。LLM 失败回退本地拼接，绝不空窗。
func distillSummary() {
	summaryPipeline.Lock()
	defer summaryPipeline.Unlock()

	sources := collectSummarySources()
	if strings.TrimSpace(sources) == "" {
		return // 还没有任何记忆，不生成空摘要
	}

	prompt := `你是记忆整理器。把下面的原始记忆素材整理成一份「给用户看」的高密度摘要。
要求：
- 全部用中文自然语言写成人话，禁止 key=value 机器串
- 分节：## 关于你（偏好/风格/画像）、## 项目与决定、## 相处状态（亲密度一句话）
- 每节 1-5 条，一条一句，信息密度优先；重复/矛盾条目合并，矛盾保留最新
- 演示/测试/mock/示例类内容一律丢弃
- 没有内容的节整个省略
- 只输出 Markdown 正文，不要解释、不要代码块围栏

素材：
` + sources

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	backends := []RouterBackend{}
	if b1 := resolveExact("", "free_llm7_gemini_flash_lite"); b1 != nil {
		backends = append(backends, *b1)
	}
	if b2 := resolveExact("", "free_zen_north_mini_code"); b2 != nil {
		backends = append(backends, *b2)
	}
	backends = append(backends, resolveBackends("", "")...)

	content, _, err := routeChatOnce(ctx, uniqueMemoryBackends(backends),
		[]map[string]any{{"role": "user", "content": prompt}}, nil)
	if err != nil {
		content = ""
	}
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```markdown")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	if content == "" {
		content = fallbackSummary(sources) // LLM 挂了也要有可读视图
	}
	if len([]rune(content)) > 2000 {
		content = string([]rune(content)[:2000])
	}

	p := summaryPath()
	if p == "" {
		return
	}
	os.MkdirAll(filepath.Dir(p), 0o755)
	// 只存正文（## 分节），不加 "# 记忆概览" 标题——前端渲染时统一加 `## 标题`，
	// 双标题会出现「记忆概览 记忆概览」重复（2026-09-04 用户实测）。
	os.WriteFile(p, []byte(content+"\n"), 0o644)
}

// fallbackSummary LLM 失败时的本地兜底：至少把 facts 渲染成中文分节列表。
func fallbackSummary(sources string) string {
	var b strings.Builder
	automaticMemory.Lock()
	facts, _ := loadAutomaticFacts()
	automaticMemory.Unlock()
	sections := map[string]string{"preferences": "关于你", "projects": "项目与决定", "decisions": "项目与决定", "profile": "关于你"}
	bySection := map[string][]string{}
	for _, f := range facts {
		if mockNoiseKey(f.Key, f.Value) {
			continue
		}
		sec := sections[f.Category]
		if sec == "" {
			sec = "其他"
		}
		bySection[sec] = append(bySection[sec], fmt.Sprintf("- %s：%s", memoryKeyLabel(f.Key), f.Value))
	}
	for _, sec := range []string{"关于你", "项目与决定", "其他"} {
		items := bySection[sec]
		if len(items) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## %s\n%s\n\n", sec, strings.Join(items, "\n"))
	}
	return strings.TrimSpace(b.String())
}

// scheduleSummaryDistill 防抖调度（30s 内多次变更只蒸馏一次）。
func scheduleSummaryDistill() {
	summaryPipeline.Lock()
	defer summaryPipeline.Unlock()
	if summaryPipeline.timer != nil {
		summaryPipeline.timer.Stop()
	}
	summaryPipeline.timer = time.AfterFunc(30*time.Second, func() {
		distillSummary()
	})
}
