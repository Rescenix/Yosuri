package handler

// 工具输出分层截断 —— 超长输出只保留首尾，中间落盘，模型要细节时自己去读。
//
// 为什么不是原来那样一刀切：原来 truncateChars(out, 10000) 直接砍掉尾巴，
// 而尾巴恰恰是最值钱的部分——编译/测试输出的结论（"3 failed"）、命令的退出信息、
// 报错的最后一行，全在末尾。砍完模型看到一堆无关的开头，还得再跑一次才知道结果。
//
// 改成首尾都留之后，截断变成无损的（全文落盘可回查），于是预算可以放心压到
// 6000 字符：20 轮最坏从 5 万 tok 降到 3 万，且信息量反而比原来大。
//
// 落盘位置必须在项目根下——MCP filesystem server 的 allowed root 就是项目根，
// 放到系统临时目录的话模型根本读不回来，"可回查"就是句空话。

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"backend/internal/ai/core"
)

const (
	// codeResultBudgetChars 单条工具结果进上下文的字符预算。
	// 超过就走首尾保留 + 落盘；比原来的 codeResultMaxChars 低，因为现在截断是可回查的。
	codeResultBudgetChars = 6000
	// 首尾配比：头部给上下文（命令是什么、文件开头长什么样），
	// 尾部给结论（报错、汇总、退出码）。尾部略少但足够放下结论段。
	codeResultHeadChars = 3600
	codeResultTailChars = 2400
)

// toolOutputSpillDir 溢出文件目录：项目根下的 .aurora/tool_outputs，
// 在 MCP filesystem 的 allowed root 之内，模型可以直接读回。
func toolOutputSpillDir() string {
	return filepath.Join(core.GetProjectRoot(), ".aurora", "tool_outputs")
}

// toolOutputArchiveTTL 归档保留时长。跟检查点同一个量级：够覆盖"隔天回来接着问"，
// 又不至于把项目目录堆成垃圾场。
const toolOutputArchiveTTL = 24 * time.Hour

// spillToolOutput 把完整输出写盘，返回可供模型读取的路径；写失败返回空串
// （调用方据此退化成纯截断，绝不因为落盘失败中断任务）。
//
// 文件名带上工具名，是为了让归档目录本身可读：模型（和人）扫一眼目录就知道
// 哪个文件是哪次调用的产物，不用逐个打开猜。
func spillToolOutput(workflowID, callID, toolName, content string) string {
	dir := toolOutputSpillDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}
	name := fmt.Sprintf("%s_%s_%s.txt",
		sanitizeFileToken(workflowID), sanitizeFileToken(callID), sanitizeFileToken(toolName))
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return ""
	}
	return path
}

// sanitizeFileToken 把 id 里的路径分隔符等换掉，避免拼出越界路径。
func sanitizeFileToken(s string) string {
	if s == "" {
		return "x"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

// compactToolOutput 把一条工具结果压到预算内：
// 预算内原样返回；超了则保留首尾、中间换成一行说明，并把全文落盘告诉模型路径。
// 返回进上下文的文本。
// 第二个返回值是归档记录：没触发截断时为 nil，供账本（harness_status）登记
// "这次丢了什么、去哪捞"。
func compactToolOutput(workflowID, callID, toolName, output string) (string, *archivedOutput) {
	if len(output) <= codeResultBudgetChars {
		return output, nil
	}

	head := cutAtRuneBoundary(output, codeResultHeadChars)
	tail := cutTailAtRuneBoundary(output, codeResultTailChars)
	omitted := len(output) - len(head) - len(tail)
	if omitted < 0 {
		omitted = 0
	}

	var record *archivedOutput
	hint := fmt.Sprintf("……[中间省略约 %d 字符]……", omitted)
	if path := spillToolOutput(workflowID, callID, toolName, output); path != "" {
		// 给相对路径：MCP filesystem 的 allowed root 就是项目根，相对路径更好用；
		// 顺带点名 head/tail 参数，免得模型为了看中间一段把整个文件读回来。
		rel, err := filepath.Rel(core.GetProjectRoot(), path)
		if err != nil {
			rel = path
		}
		rel = filepath.ToSlash(rel)
		hint = fmt.Sprintf("……[中间省略约 %d 字符。完整输出已存到 %s，需要中间部分就用 read_file 按行读它（别整篇读回来）]……",
			omitted, rel)
		record = &archivedOutput{
			CallID: callID, Tool: toolName, OmittedChars: omitted,
			TotalChars: len(output), RelPath: rel,
		}
	}

	return head + "\n" + hint + "\n" + tail, record
}

// cutAtRuneBoundary 取前 n 字节，但不切碎多字节 UTF-8。
func cutAtRuneBoundary(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !isRuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// cutTailAtRuneBoundary 取后 n 字节，同样对齐到字符边界。
func cutTailAtRuneBoundary(s string, n int) string {
	if len(s) <= n {
		return s
	}
	start := len(s) - n
	for start < len(s) && !isRuneStart(s[start]) {
		start++
	}
	return s[start:]
}

// isRuneStart 判断该字节是否是一个 UTF-8 字符的首字节（续接字节形如 10xxxxxx）。
func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }

// sweepToolOutputArchive 按 TTL 清理归档目录，在工作流启动时调一次。
//
// 这里原本是 cleanupToolOutputSpills(workflowID)：任务一成功就把本次落盘的
// 全文删干净。那样有个说不通的后果——被截断的工具结果在上下文里留着一句
// 「完整输出已存到 xxx」，而任务收尾的那一刻文件就没了，指针当场变悬空；
// 下一轮对话里模型照着去读只会拿到"文件不存在"。
//
// 现在改成归档：成功也留着，靠时间淘汰。谁产生的、什么工具产生的都写在文件名里，
// 模型可以通过 harness_status 拿到目录和清单再按需读回。
func sweepToolOutputArchive() {
	dir := toolOutputSpillDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	deadline := time.Now().Add(-toolOutputArchiveTTL)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil || info.ModTime().After(deadline) {
			continue
		}
		os.Remove(filepath.Join(dir, e.Name()))
	}
}
