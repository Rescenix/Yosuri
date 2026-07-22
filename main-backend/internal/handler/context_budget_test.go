package handler

import (
	"encoding/json"
	"testing"
)

// TestContextBudgetReport 不是断言用例，是一把尺子：打印每轮请求里静态前缀
// 各部分的真实 token 占用，以及其中哪些是「会变的」——变的部分决定了
// 前缀缓存能不能命中。改上下文结构前后各跑一次对比。
//
//	go test ./internal/handler/ -run TestContextBudgetReport -v
func TestContextBudgetReport(t *testing.T) {
	// 走真实装配路径（context_provider.go），免得这把尺子和线上跑的不是一套
	p := newWorkflowContextProvider()
	toolsJSON, _ := json.Marshal(p.Tools())
	toolIndex := mcpToolIndexPrompt()
	mcpDefs := loadMCPToolDefs()
	// 改造前的口径：MCP 全量 schema 无条件进每一轮，用来对照现在省了多少
	allToolsJSON, _ := json.Marshal(buildCodeWorkflowTools(func() map[string]bool {
		all := map[string]bool{}
		for _, t := range mcpDefs {
			all[t.Function.Name] = true
		}
		return all
	}()))

	total, volatileTok := 0, 0
	t.Log("──────── 每轮请求的静态前缀构成（按装配顺序）────────")
	for _, s := range p.sections {
		tok := estimateTokenCount(s.content)
		total += tok
		tag := "稳定"
		if !s.stable {
			tag = "易变"
			volatileTok += tok
		}
		t.Logf("%-14s %7d tok   [%s]", s.key, tok, tag)
	}
	toolSchemaTok := estimateTokenCount(string(toolsJSON))
	total += toolSchemaTok
	t.Logf("%-14s %7d tok   [稳定]（走 tools 参数，不在提示词里）", "常驻工具schema", toolSchemaTok)
	t.Logf("%-22s %7d tok", "合计", total)
	pct := 0.0
	if total > 0 {
		pct = float64(volatileTok) / float64(total) * 100
	}
	t.Logf("其中易变部分         %7d tok（占 %.0f%%）—— 这些一变，整个前缀缓存作废", volatileTok, pct)
	t.Logf("常驻工具 %d 个；MCP %d 个走按需加载", len(buildCodeWorkflowTools(nil)), len(mcpDefs))

	t.Log("──────── 按需加载省了多少 ────────")
	before := estimateTokenCount(string(allToolsJSON))
	after := estimateTokenCount(string(toolsJSON)) + estimateTokenCount(toolIndex)
	t.Logf("改造前（全量 schema 每轮发）  %6d tok", before)
	t.Logf("改造后（常驻 schema + 索引）  %6d tok", after)
	if before > 0 {
		t.Logf("每轮省 %d tok（%.0f%%）；按 %d 轮上限算，单个任务最多省 %d tok",
			before-after, float64(before-after)/float64(before)*100,
			codeWorkflowMaxRounds, (before-after)*codeWorkflowMaxRounds)
	}

	t.Log("──────── 工具输出（动态部分）────────")
	t.Logf("单条结果上限 %d 字符 ≈ %d tok；%d 轮最坏 ≈ %d tok",
		codeResultMaxChars, codeResultMaxChars/4,
		codeWorkflowMaxRounds, codeWorkflowMaxRounds*codeResultMaxChars/4)
}
