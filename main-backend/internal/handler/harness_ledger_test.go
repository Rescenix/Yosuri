package handler

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// 账本存在的意义是把"系统知道、模型看不见"的事实讲出来。
// 所以每一项的空态也必须显式说"无"——只在有内容时才提的话，
// 模型会把"没提到"读成"没发生"，等于白记。
func TestLedgerReportStatesEmptyExplicitly(t *testing.T) {
	l := newContextLedger()
	l.noteHistory(3, 3, 24)
	got := l.report(1, map[string]int{"system": 400}, nil)

	for _, want := range []string{"未发生", "无，本次所有工具输出都是完整的", "全部带入"} {
		if !strings.Contains(got, want) {
			t.Errorf("空态没有被显式说明，缺少 %q：\n%s", want, got)
		}
	}
}

// 历史被窗口截掉时必须报出来——这是模型判断"我是不是漏了什么"的唯一依据。
func TestLedgerReportsTruncatedHistory(t *testing.T) {
	l := newContextLedger()
	l.noteHistory(24, 100, 24)
	got := l.report(2, nil, nil)

	if !strings.Contains(got, "带入 24 条") || !strings.Contains(got, "共 100 条") {
		t.Errorf("历史窗口信息不全:\n%s", got)
	}
	if !strings.Contains(got, "76") {
		t.Errorf("没算出被截掉的条数(100-24=76):\n%s", got)
	}
}

// 被截断的工具输出必须连"去哪捞"一起报，否则账本只是在通知坏消息。
func TestLedgerReportsArchivePaths(t *testing.T) {
	l := newContextLedger()
	l.noteArchive(&archivedOutput{
		CallID: "call_7", Tool: "mcp__shell__run",
		OmittedChars: 12000, TotalChars: 18000,
		RelPath: ".aurora/tool_outputs/wf_1_call_7_mcp__shell__run.txt",
	})
	l.noteArchive(nil) // nil 必须被安静忽略，不能 panic
	got := l.report(3, nil, nil)

	for _, want := range []string{"mcp__shell__run", "call_7", "12000", ".aurora/tool_outputs/", "read_file"} {
		if !strings.Contains(got, want) {
			t.Errorf("归档信息缺少 %q：\n%s", want, got)
		}
	}
}

// 压缩会把整段原始细节换成摘要，模型必须被告知，否则它以为自己还记得。
func TestLedgerReportsCompaction(t *testing.T) {
	l := newContextLedger()
	l.noteCompaction(compactionEvent{Round: 5, FoldedMsgs: 12, BeforeChars: 90000, AfterChars: 3000})
	l.noteCompaction(compactionEvent{Round: 9, FoldedMsgs: 8, BeforeChars: 80000, AfterChars: 2500})
	got := l.report(10, nil, nil)

	if !strings.Contains(got, "发生 2 次") || !strings.Contains(got, "20 条") {
		t.Errorf("压缩累计信息不对（应为 2 次、共 20 条）:\n%s", got)
	}
	if !strings.Contains(got, "原始细节已不在上下文里") {
		t.Errorf("没讲清楚后果:\n%s", got)
	}
}

// harness_status 已从常驻改为按需（2026-08-29 收敛）：模型直接调即自动带 schema。
// 它必须在按需池里且可被 load_tools 激活，否则自省能力不可达。
func TestHarnessStatusIsAlwaysAvailable(t *testing.T) {
	if !isOnDemandTool(harnessStatusToolName) {
		t.Fatalf("%s 不在按需池里", harnessStatusToolName)
	}
	activated := map[string]bool{}
	if _, changed := handleLoadTools(`{"names":["`+harnessStatusToolName+`"]}`, activated); !changed {
		t.Fatalf("%s 不能被 load_tools 激活", harnessStatusToolName)
	}
	found := false
	for _, tl := range buildCodeWorkflowTools(activated) {
		if tl["function"].(map[string]any)["name"] == harnessStatusToolName {
			found = true
		}
	}
	if !found {
		t.Fatalf("%s 激活后没进 tools 数组", harnessStatusToolName)
	}
}

// 账本为 nil 时不能 panic（防御性：任何调用路径漏传都不该炸掉整轮）
func TestHandleHarnessStatusNilLedger(t *testing.T) {
	if out := handleHarnessStatus("{}", nil, 1, nil, nil); !strings.Contains(out, "不可用") {
		t.Errorf("nil 账本应回可读提示，实得 %q", out)
	}
}

func TestActivatedToolNames(t *testing.T) {
	got := activatedToolNames(map[string]bool{"a": true, "b": false, "c": true})
	if len(got) != 2 {
		t.Fatalf("只该收 true 的项，实得 %v", got)
	}
	sortStrings(got)
	if got[0] != "a" || got[1] != "c" {
		t.Errorf("内容不对: %v", got)
	}
}

// 常驻工具被误当成需要加载时，系统应直接给出答案（"它已经在你手上了"），
// 而不是回一句"名字不存在，自己去核对索引"——后者会把模型支到错误的方向。
func TestLoadToolsTellsAboutResidentTools(t *testing.T) {
	out, changed := handleLoadTools(`{"names":["harness_status"]}`, map[string]bool{})
	if changed {
		t.Error("常驻工具不该产生激活变更")
	}
	if !strings.Contains(out, "常驻工具") || !strings.Contains(out, "直接调用") {
		t.Errorf("没告诉模型它已经可以直接调用了:\n%s", out)
	}
	if strings.Contains(out, "不存在") {
		t.Errorf("不该再说名字不存在——它确实存在，只是不需要加载:\n%s", out)
	}
}

// 无参工具序列化后绝不能出现 "required": null —— DeepSeek 的 schema 校验会因此
// 拒掉整个请求（400 null is not of type "array"），导致整轮静默降级到备用模型。
// 这条守的是所有无必填参数的工具，不只 harness_status。
func TestNoArgToolSchemaHasNoNullRequired(t *testing.T) {
	for _, tl := range buildCodeWorkflowTools(nil) {
		b, err := json.Marshal(tl)
		if err != nil {
			t.Fatalf("工具定义序列化失败: %v", err)
		}
		if strings.Contains(string(b), `"required":null`) {
			fn := tl["function"].(map[string]any)
			t.Errorf("%v 的 schema 含 \"required\":null，DeepSeek 会拒掉整个请求:\n%s", fn["name"], b)
		}
	}
}

// 全新会话（零历史）时【会话历史】那一段不能整个消失——
// 消失的话模型分不清"没有历史"和"账本没记这项"。
func TestLedgerReportsEmptyHistoryExplicitly(t *testing.T) {
	l := newContextLedger()
	l.noteHistory(0, 0, 24)
	got := l.report(0, nil, nil)
	if !strings.Contains(got, "【会话历史】") {
		t.Errorf("零历史时该段整个消失了:\n%s", got)
	}
	if !strings.Contains(got, "全新会话") {
		t.Errorf("没说清楚是新会话而不是漏记:\n%s", got)
	}
}

// 账本落盘是后续所有优化决策的数据来源，必须真的写下去、且字段能算对。
func TestLedgerPersistWritesAnalyzableRecord(t *testing.T) {
	withTempDataDir(t)

	l := newContextLedger()
	l.noteHistory(24, 100, 24) // 丢了 76 条
	l.noteCompaction(compactionEvent{Round: 5, FoldedMsgs: 12})
	l.noteCompaction(compactionEvent{Round: 9, FoldedMsgs: 8})
	l.noteArchive(&archivedOutput{CallID: "c1", Tool: "mcp__fs__read_text_file", OmittedChars: 9000})
	l.noteArchive(&archivedOutput{CallID: "c2", Tool: "mcp__shell__run", OmittedChars: 1500})

	l.persist(ledgerRecord{
		WorkflowID: "wf_x", SessionID: "s1", Task: "写一个技能卡片墙",
		Outcome: "completed", Rounds: 14, InTokens: 52000, OutTokens: 3000,
		ActivatedTools: 4,
	})

	data, err := os.ReadFile(ledgerLogPath())
	if err != nil {
		t.Fatalf("账本没落盘: %v", err)
	}
	var rec ledgerRecord
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &rec); err != nil {
		t.Fatalf("落盘的不是合法 JSONL: %v", err)
	}

	if rec.HistoryDropped != 76 {
		t.Errorf("丢弃历史条数应为 76，实得 %d", rec.HistoryDropped)
	}
	if rec.Compactions != 2 || rec.FoldedMsgs != 20 {
		t.Errorf("压缩统计不对: %d 次 / %d 条", rec.Compactions, rec.FoldedMsgs)
	}
	if rec.Truncations != 2 || rec.TruncatedChars != 10500 {
		t.Errorf("截断统计不对: %d 次 / %d 字符", rec.Truncations, rec.TruncatedChars)
	}
	if rec.Outcome != "completed" || rec.Rounds != 14 {
		t.Errorf("结局/轮次丢失: %+v", rec)
	}
	if rec.At.IsZero() {
		t.Error("没有时间戳，无法做趋势分析")
	}
}

// 多次落盘必须是追加，不能互相覆盖——否则积累不出数据。
func TestLedgerPersistAppends(t *testing.T) {
	withTempDataDir(t)
	for i := 0; i < 3; i++ {
		newContextLedger().persist(ledgerRecord{WorkflowID: "wf", Outcome: "completed"})
	}
	data, _ := os.ReadFile(ledgerLogPath())
	if n := len(strings.Split(strings.TrimSpace(string(data)), "\n")); n != 3 {
		t.Errorf("应有 3 行记录，实得 %d", n)
	}
}
