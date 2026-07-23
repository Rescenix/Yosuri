package handler

// 上下文账本（harness 的自省层）
//
// 问题：模型对自己的记忆状况是完全瞎的。上下文里哪些内容被压缩折叠了、哪条工具
// 结果被截断了、被截掉的部分去哪了、会话历史带进来多少条——这些系统全知道，
// 模型全不知道。它只能从残缺的上下文里猜，猜错就变成"重复读文件""重跑命令"
// 甚至"回头重做旧任务"。
//
// 账本把这些系统事实记下来，通过 harness_status 工具讲给模型听：
// 你现在知道什么、丢了什么、丢的东西去哪捞。

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"backend/internal/ai/core"
)

// archivedOutput 一条被截断并归档的工具结果（compactToolOutput 产出）。
type archivedOutput struct {
	CallID       string
	Tool         string
	OmittedChars int
	TotalChars   int
	RelPath      string
}

// compactionEvent 一次上下文压缩。
type compactionEvent struct {
	Round       int
	FoldedMsgs  int
	BeforeChars int
	AfterChars  int
}

// contextLedger 单个工作流的上下文账本。
// 工作流是单 goroutine 驱动的，但 dispatch_agent 的子代理会并发跑，
// 保守起见加锁——账本被读写的频率极低，锁的代价可以忽略。
type contextLedger struct {
	mu sync.Mutex

	HistoryIncluded int // 实际带进上下文的历史消息条数
	HistorySessionN int // 该会话累计的历史消息总条数
	HistoryLimit    int // 本次的历史窗口上限

	archives   []archivedOutput
	compaction []compactionEvent
}

func newContextLedger() *contextLedger { return &contextLedger{} }

func (l *contextLedger) noteHistory(included, total, limit int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.HistoryIncluded, l.HistorySessionN, l.HistoryLimit = included, total, limit
}

func (l *contextLedger) noteArchive(a *archivedOutput) {
	if a == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.archives = append(l.archives, *a)
}

func (l *contextLedger) noteCompaction(e compactionEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.compaction = append(l.compaction, e)
}

// report 渲染给模型看的账本。刻意用纯文本而不是 JSON：
// 这段是给模型读的，不是给程序解析的，纯文本更省 token 也更好懂。
func (l *contextLedger) report(round int, breakdown map[string]int, activated []string) string {
	l.mu.Lock()
	defer l.mu.Unlock()

	var b strings.Builder
	fmt.Fprintf(&b, "━━━ 上下文账本（第 %d 轮）━━━\n", round)

	// 1) 静态段占用
	if len(breakdown) > 0 {
		keys := make([]string, 0, len(breakdown))
		total := 0
		for k, v := range breakdown {
			keys = append(keys, fmt.Sprintf("%s=%d", k, v))
			total += v
		}
		sortStrings(keys)
		fmt.Fprintf(&b, "【系统提示词占用】共 %d tok（%s）\n", total, strings.Join(keys, " "))
	}

	// 2) 历史窗口：说清楚"有没有被截掉"，这是模型判断"我是不是漏了什么"的依据。
	// 无条件输出——新会话时如果整段消失，模型分不清"没有历史"和"账本没记这项"。
	switch {
	case l.HistorySessionN == 0:
		b.WriteString("【会话历史】无（全新会话，之前没有对话）\n")
	case l.HistorySessionN > l.HistoryIncluded:
		fmt.Fprintf(&b, "【会话历史】带入 %d 条，该会话共 %d 条，最早的 %d 条超出窗口(上限 %d)未带入\n",
			l.HistoryIncluded, l.HistorySessionN, l.HistorySessionN-l.HistoryIncluded, l.HistoryLimit)
	default:
		fmt.Fprintf(&b, "【会话历史】带入 %d 条（全部带入，没有遗漏）\n", l.HistoryIncluded)
	}

	// 3) 压缩：折叠过的轮次已经变成摘要，原始细节不在上下文里了
	if len(l.compaction) == 0 {
		b.WriteString("【上下文压缩】未发生，本次所有轮次的原始内容都还在\n")
	} else {
		folded := 0
		for _, c := range l.compaction {
			folded += c.FoldedMsgs
		}
		fmt.Fprintf(&b, "【上下文压缩】发生 %d 次，累计把 %d 条消息折叠成摘要——"+
			"这些轮次的原始细节已不在上下文里，只剩摘要；需要精确内容请重新读文件或查下面的归档\n",
			len(l.compaction), folded)
	}

	// 4) 被截断的工具结果 + 去哪捞（这是账本最实用的一段）
	if len(l.archives) == 0 {
		b.WriteString("【工具结果截断】无，本次所有工具输出都是完整的\n")
	} else {
		fmt.Fprintf(&b, "【工具结果截断】%d 条超长输出只保留了首尾，全文已归档：\n", len(l.archives))
		for _, a := range l.archives {
			fmt.Fprintf(&b, "  - %s (call %s)：共 %d 字符，省略 %d，全文 → %s\n",
				a.Tool, a.CallID, a.TotalChars, a.OmittedChars, a.RelPath)
		}
		b.WriteString("  用 mcp__fs__read_text_file 读上面的路径可取回全文（支持 head/tail 只取一段）\n")
	}

	// 5) 已激活的按需工具
	if len(activated) > 0 {
		sortStrings(activated)
		fmt.Fprintf(&b, "【已加载的 MCP 工具】%s\n", strings.Join(activated, ", "))
	} else {
		b.WriteString("【已加载的 MCP 工具】无（要用文件/命令类工具得先 load_tools）\n")
	}

	// 6) 归档目录：让模型知道往哪翻更早的东西
	if rel, err := filepath.Rel(core.GetProjectRoot(), toolOutputSpillDir()); err == nil {
		fmt.Fprintf(&b, "【历史归档目录】%s（保留 %s，含更早工作流的完整工具输出，文件名格式 工作流ID_调用ID_工具名.txt）\n",
			filepath.ToSlash(rel), toolOutputArchiveTTL)
	}

	return b.String()
}

// ---- 落盘：积累"长任务到底死于什么"的真实数据 ----

// ledgerRecord 一个工作流跑完后留下的一行事实。
// 刻意做得很窄：只记能回答"退化发生在哪"的字段，不记会话内容（隐私 + 体积）。
type ledgerRecord struct {
	At         time.Time `json:"at"`
	WorkflowID string    `json:"workflow_id"`
	SessionID  string    `json:"session_id"`
	Task       string    `json:"task"` // 截断到 120 字，只为人肉回溯时认得出是哪个任务
	Outcome    string    `json:"outcome"`
	Rounds     int       `json:"rounds"`
	InTokens   int       `json:"in_tokens"`
	OutTokens  int       `json:"out_tokens"`

	// —— 下面这几项就是"优化该往哪使劲"的答案来源 ——
	HistoryIncluded int `json:"history_included"`
	HistoryTotal    int `json:"history_total"`
	HistoryDropped  int `json:"history_dropped"` // >0 说明窗口不够用
	Compactions     int `json:"compactions"`     // >0 说明上下文压力大
	FoldedMsgs      int `json:"folded_msgs"`
	Truncations     int `json:"truncations"`     // 工具输出腰斩次数：前端任务尤其容易高
	TruncatedChars  int `json:"truncated_chars"` // 累计被截掉多少
	ActivatedTools  int `json:"activated_tools"`
}

// ledgerLogPath 与会话/检查点同域，方便一起备份或清理。
func ledgerLogPath() string {
	dataDir := os.Getenv("SHANXI_DATA_DIR")
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		dataDir = filepath.Join(home, "shanxi_data")
	}
	return filepath.Join(dataDir, "harness_ledger.jsonl")
}

// persist 追加一行 JSONL。
//
// 用追加而不是整份重写：这份日志会长期累积，整份重写既慢又有并发覆盖风险
// （session.go 那个数据竞争就是整份重写踩出来的）。追加天然并发安全——
// 单次 write 小于 PIPE_BUF 时是原子的，多个工作流并发收尾也不会互相撕裂。
//
// 落盘失败只打日志：账本是观测设施，不该让它反过来影响任务成败。
func (l *contextLedger) persist(rec ledgerRecord) {
	l.mu.Lock()
	rec.HistoryIncluded = l.HistoryIncluded
	rec.HistoryTotal = l.HistorySessionN
	if d := l.HistorySessionN - l.HistoryIncluded; d > 0 {
		rec.HistoryDropped = d
	}
	rec.Compactions = len(l.compaction)
	for _, c := range l.compaction {
		rec.FoldedMsgs += c.FoldedMsgs
	}
	rec.Truncations = len(l.archives)
	for _, a := range l.archives {
		rec.TruncatedChars += a.OmittedChars
	}
	l.mu.Unlock()

	rec.At = time.Now()
	rec.Task = truncateChars(rec.Task, 120)

	data, err := json.Marshal(rec)
	if err != nil {
		return
	}
	path := ledgerLogPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("⚠️ 账本落盘失败: %v", err)
		return
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		log.Printf("⚠️ 账本写入失败: %v", err)
	}
}

// activatedToolNames 把激活集摊平成名字列表（map 顺序随机，报告里再排序）。
func activatedToolNames(activated map[string]bool) []string {
	out := make([]string, 0, len(activated))
	for name, on := range activated {
		if on {
			out = append(out, name)
		}
	}
	return out
}

// sortStrings 小工具：避免为了排序单独引 sort 影响可读性（就地插入排序，量级很小）
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// ---- harness_status 工具 ----

const harnessStatusToolName = "harness_status"

var harnessStatusToolDef = core.ToolDefinition{
	Type: "function",
	Function: core.ToolFunctionDetail{
		Name: harnessStatusToolName,
		Description: "查看你自己当前的上下文状况：系统提示词各段占用、会话历史带入了多少条/" +
			"有没有被窗口截掉、上下文是否发生过压缩折叠、哪些工具输出被截断以及全文归档在哪、" +
			"已加载了哪些 MCP 工具。当你怀疑「我是不是漏看了什么」「刚才那个长输出的中间部分去哪了」" +
			"「之前是不是已经查过这个」时调它，比盲目重跑一遍工具便宜得多。",
		Parameters: core.ToolParameters{Type: "object", Properties: map[string]core.ToolProperty{}},
	},
}

// handleHarnessStatus 处理一次 harness_status 调用。纯读，无副作用。
func handleHarnessStatus(argsJSON string, l *contextLedger, round int, breakdown map[string]int, activated []string) string {
	// 参数本来就是空对象，这里只是防御性地容忍模型乱传
	if strings.TrimSpace(argsJSON) != "" && strings.TrimSpace(argsJSON) != "{}" {
		var ignored map[string]any
		_ = json.Unmarshal([]byte(argsJSON), &ignored)
	}
	if l == nil {
		return "账本不可用（本次工作流没有初始化账本）"
	}
	return l.report(round, breakdown, activated)
}
