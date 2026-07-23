package handler

import (
	"strings"
	"testing"
)

// 稳定段必须全部排在易变段之前。这是 provider 存在的首要理由：
// 前缀缓存只认"从头逐字节相同"，把每完成一个任务就会变的技能库排在前面，
// 等于每跑完一个任务就把它后面所有内容的缓存作废。
func TestStableSectionsComeFirst(t *testing.T) {
	p := newWorkflowContextProvider()
	seenVolatile := false
	for _, s := range p.sections {
		if !s.stable {
			seenVolatile = true
			continue
		}
		if seenVolatile {
			t.Errorf("稳定段 %q 排在了易变段后面，前缀缓存会被无谓打断", s.key)
		}
	}
}

// 分类占用必须与系统提示词同源：加了一段却忘了登记，前端面板就会少算
// （加 MCP 工具索引时就真漏过一次）。这里断言每个非空段都被计入了 breakdown。
func TestBreakdownCoversEverySection(t *testing.T) {
	p := newWorkflowContextProvider()
	bd := p.Breakdown()
	for _, s := range p.sections {
		if strings.TrimSpace(s.content) == "" {
			continue
		}
		if bd[s.key] == 0 {
			t.Errorf("段 %q 有内容却没进 breakdown，前端分类会少算", s.key)
		}
	}
}

// breakdown 的 key 必须落在前端 contextBreakdown.js 认识的桶里，
// 多出来的 key 会被前端静默丢弃，导致「分类之和 ≈ prompt_tokens」对不上。
func TestBreakdownKeysMatchFrontendContract(t *testing.T) {
	allowed := map[string]bool{
		"system": true, "subagent": true, "skill": true, "memory": true, "tools": true,
	}
	for k := range newWorkflowContextProvider().Breakdown() {
		if !allowed[k] {
			t.Errorf("breakdown 出现前端不认识的 key %q，该分类会被丢掉", k)
		}
	}
}

// SystemPrompt 必须真的包含各段内容——拼接顺序可以变，内容不能丢。
func TestSystemPromptContainsSections(t *testing.T) {
	p := newWorkflowContextProvider()
	sp := p.SystemPrompt()
	for _, s := range p.sections {
		if c := strings.TrimSpace(s.content); c != "" && !strings.Contains(sp, c) {
			t.Errorf("段 %q 的内容没出现在系统提示词里", s.key)
		}
	}
}

func TestInvokingBuildsSystemHistoryTask(t *testing.T) {
	p := newWorkflowContextProvider()
	history := []DSMessage{
		{Role: "user", Content: "上一条问题"},
		{Role: "assistant", Content: "上一条回答"},
	}
	msgs := p.Invoking(history, "本次任务")

	if len(msgs) != 4 {
		t.Fatalf("应为 system + 2 条历史 + 1 条任务 = 4，实得 %d", len(msgs))
	}
	if msgs[0]["role"] != "system" {
		t.Errorf("首条必须是 system，实得 %v", msgs[0]["role"])
	}
	// 历史里的用户消息现在会被打上「已完成」前缀（见 buildChatMessages）：
	// 不打标的话模型会把旧任务读成待办，收尾时回头重做上一个任务。
	// 所以这里断言"原文还在"而不是"完全相等"；assistant 回复不打标，仍是全等。
	if !strings.Contains(msgs[1]["content"].(string), "上一条问题") || msgs[2]["content"] != "上一条回答" {
		t.Error("会话历史没被带进去——模型会不知道上一条说了什么")
	}
	if !strings.HasPrefix(msgs[1]["content"].(string), completedTaskPrefix) {
		t.Errorf("历史里的用户任务应被标记为已完成，实得 %q", msgs[1]["content"])
	}
	last := msgs[len(msgs)-1]
	if last["role"] != "user" || last["content"] != "本次任务" {
		t.Errorf("末条应是本次任务，实得 %v", last)
	}
}

// Invoked 必须把当轮状态原样交给落状态回调（检查点靠它续跑）。
func TestInvokedPassesRoundStateThrough(t *testing.T) {
	p := newWorkflowContextProvider()
	var gotRound int
	var gotState roundState
	called := 0
	p.OnInvoked(func(round int, st roundState) {
		called++
		gotRound, gotState = round, st
	})

	want := roundState{
		msgs:         []map[string]any{{"role": "user", "content": "x"}},
		transcript:   []string{"read_file(...) => ok"},
		callSigCount: map[string]int{"read_file|{}": 2},
		callSeq:      3, inputTokens: 1200, outputTokens: 340,
	}
	p.Invoked(7, want)

	if called != 1 || gotRound != 7 {
		t.Fatalf("回调应被调用一次且 round=7，实得 called=%d round=%d", called, gotRound)
	}
	if gotState.callSeq != 3 || gotState.inputTokens != 1200 || len(gotState.msgs) != 1 {
		t.Errorf("roundState 传丢了: %+v", gotState)
	}
	if gotState.callSigCount["read_file|{}"] != 2 {
		t.Error("熔断计数没传过去，续跑后重复调用护栏会失效")
	}
}

// 没注册回调时 Invoked 不能炸（子代理等场景不需要落检查点）。
func TestInvokedWithoutHookIsNoop(t *testing.T) {
	newWorkflowContextProvider().Invoked(1, roundState{})
}

func TestRestoreActivatedToolsIgnoresEmpty(t *testing.T) {
	p := newWorkflowContextProvider()
	base := len(p.Tools())
	p.RestoreActivatedTools(nil)
	p.RestoreActivatedTools(map[string]bool{})
	if len(p.Tools()) != base {
		t.Error("空的激活集不该改变工具数组")
	}
}
