package main

// keyinput.go — 交互式逐键输入：实时补全 + ↑↓选择 + Tab 补全 + 历史 + 中文
// raw mode 由 rawmode_windows.go / rawmode_unix.go 提供

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// 按键类型
type keyKind int

const (
	keyRune keyKind = iota
	keyEnter
	keyTab
	keyBackspace
	keyUp
	keyDown
	keyLeft
	keyRight
	keyEsc
	keyCtrlC
	keyCtrlD
	keyUnknown
)

// slash 命令补全表
var slashCommands = []string{
	"exit", "quit", "clear", "help", "models", "model", "status",
	"shell", "refresh", "history", "env", "report", "rep",
	"marathon", "exec", "learn", "update", "version",
}

// commandDesc 命令简短描述
var commandDesc = map[string]string{
	"exit":     "退出",
	"quit":     "退出",
	"clear":    "清屏",
	"help":     "显示帮助",
	"models":   "列出模型",
	"model":    "切换模型",
	"status":   "系统信息",
	"shell":    "Shell 模式",
	"refresh":  "刷新模型",
	"history":  "命令历史",
	"env":      "环境变量",
	"report":   "查看战报",
	"rep":      "查看战报",
	"marathon": "24H 马拉松",
	"exec":     "执行命令",
	"learn":    "电子女儿学习",
	"update":   "更新",
	"version":  "版本",
}

// 候选列表 ANSI 样式
const (
	bgCand   = "\x1b[40m"               // 纯黑背景遮罩（和终端同色，挡住后面文字）
	fgCmd    = "\x1b[38;2;86;156;214m"  // 亮蓝命令名
	fgDesc   = "\x1b[38;2;140;140;155m" // 灰色描述
	fgSel    = "\x1b[38;2;200;200;215m" // 选中项白色
	markSel  = "▸ "
	markNorm = "  "
)

// 候选列表固定行数（不含 ... 行）
const maxCandidates = 5

// terminalCellWidth 返回 rune 在终端里实际占用的列数。候选描述含中文，不能用
// RuneCount 计算；否则补空格后会越过右边界并产生一次额外的自动换行。
func terminalCellWidth(r rune) int {
	if r == 0 || r == '\n' || r == '\r' || unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) {
		return 0
	}
	if r < 0x20 || (r >= 0x7f && r < 0xa0) {
		return 0
	}
	if r >= 0x1100 && (r <= 0x115f || r == 0x2329 || r == 0x232a ||
		(r >= 0x2e80 && r <= 0xa4cf && r != 0x303f) ||
		(r >= 0xac00 && r <= 0xd7a3) || (r >= 0xf900 && r <= 0xfaff) ||
		(r >= 0xfe10 && r <= 0xfe19) || (r >= 0xfe30 && r <= 0xfe6f) ||
		(r >= 0xff00 && r <= 0xff60) || (r >= 0xffe0 && r <= 0xffe6) ||
		(r >= 0x1f300 && r <= 0x1faff) || (r >= 0x20000 && r <= 0x3fffd)) {
		return 2
	}
	return 1
}

func terminalTextWidth(text string) int {
	width := 0
	for i := 0; i < len(text); {
		// ANSI CSI（如 \x1b[38;2;100;180;255m）不占终端显示列。
		if text[i] == '\x1b' && i+1 < len(text) && text[i+1] == '[' {
			i += 2
			for i < len(text) {
				b := text[i]
				i++
				if b >= 0x40 && b <= 0x7e {
					break
				}
			}
			continue
		}
		r, size := utf8.DecodeRuneInString(text[i:])
		width += terminalCellWidth(r)
		i += size
	}
	return width
}

// fitCandidateRow 把候选行限制在 width-1 列。保留最右一列可避免 Windows
// Console/ConPTY 在恰好写满一行时触发延迟自动换行。
func fitCandidateRow(text string, width int) string {
	usable := width - 1
	if usable < 1 {
		usable = 1
	}
	var b strings.Builder
	used := 0
	for _, r := range text {
		w := terminalCellWidth(r)
		if used+w > usable {
			break
		}
		b.WriteRune(r)
		used += w
	}
	if used < usable {
		b.WriteString(strings.Repeat(" ", usable-used))
	}
	return b.String()
}

func candidateWindowStart(selected, count int) int {
	if selected < maxCandidates {
		return 0
	}
	start := selected - maxCandidates + 1
	if maxStart := count - maxCandidates; start > maxStart {
		return maxStart
	}
	return start
}

// readLine 读取一行输入（实时补全/选择/历史/中文）；非终端时回退 bufio 整行读
func (s *Shell) readLine() (string, error) {
	if !isTerminal() {
		if !s.scanner.Scan() {
			return "", io.EOF
		}
		return s.scanner.Text(), nil
	}

	restore := enableRawMode()
	defer restore()

	var buf []rune
	histIdx := len(s.history)
	prompt := s.promptStr()
	matches := []string(nil)
	selIdx := -1
	hadCandidates := false // 上次是否有候选（决定是否先清旧候选区）
	oldCandRows := 0       // 上次候选占的行数（用于清旧候选区）

	// 根据当前输入刷新候选列表（仅 / 命令前缀）
	refreshCandidates := func() {
		matches = s.matchCommands(string(buf))
		if len(matches) > 0 {
			selIdx = 0
		} else {
			selIdx = -1
		}
	}

	// 重绘：候选列表显示在输入行上方（向下打印，永不卡光标）
	// 关键修复（2026-08-04 残留 bug）：
	//   1. 所有上移前先 \r 归位列 0 —— 原实现上移时光标停在 prompt 末尾列（~36），
	//      \x1b[J 清屏只清「该列之后 + 下方」，候选区顶部行行首残留旧内容
	//   2. 候选行用显式 \r\n 结束 —— 不依赖终端对 \n 的解释（CRLF / LF-only）
	//   3. 输入行是固定锚点：清屏后显式下移回输入行位置再重画，取消候选后输入行不漂移
	redraw := func() {
		// 1. 清输入行 + 旧候选区（光标锚定在输入行位置 I）
		if hadCandidates && oldCandRows > 0 {
			fmt.Print("\r")                     // 归位列 0
			fmt.Printf("\x1b[%dA", oldCandRows) // I → 候选区顶部 T
			fmt.Print("\x1b[J")                 // 清候选区 + 输入行 + 下方
			fmt.Printf("\x1b[%dB", oldCandRows) // T → 回输入行锚点 I
			hadCandidates = false
		} else {
			fmt.Print("\r\x1b[K") // 只清输入行
		}

		// 2. 画新候选（输入行上方，向下打印）
		if len(matches) > 0 {
			tw := terminalWidth()
			windowStart := candidateWindowStart(selIdx, len(matches))
			rows := maxCandidates
			if len(matches) > maxCandidates {
				rows = maxCandidates + 1
			}
			fmt.Printf("\x1b[%dA", rows) // I → 候选区顶部 T
			// 画候选行（固定 maxCandidates 行，少于则空行补齐）
			for i := 0; i < maxCandidates; i++ {
				text := ""
				clr := fgCmd
				matchIdx := windowStart + i
				if matchIdx < len(matches) {
					cmd := matches[matchIdx]
					desc := commandDesc[cmd]
					mark := markNorm
					if matchIdx == selIdx {
						mark = markSel
						clr = fgSel
					}
					text = fmt.Sprintf("%s%-12s%s", mark, "/"+cmd, desc)
				}
				// 先用当前黑色背景擦整行，再写至 width-1 列，既形成遮罩也不触发换行。
				text = fitCandidateRow(text, tw)
				fmt.Print("\r" + bgCand + "\x1b[2K" + clr + text + ColorReset + "\r\n")
			}
			if len(matches) > maxCandidates {
				text := fitCandidateRow("  …", tw)
				fmt.Print("\r" + bgCand + "\x1b[2K" + text + ColorReset + "\r\n")
			}
			// 画完 rows 行后光标自然落在 T+rows = I（输入行锚点）
			oldCandRows = rows
			hadCandidates = true
		} else {
			oldCandRows = 0
		}

		// 3. 画输入行（光标在输入行锚点，列 0）
		fmt.Print(prompt + string(buf))
		fmt.Print("\x1b[K") // 清输入行尾部（输入变短时）
	}

	redraw()
	// 楚门世界直播：双栏直播屏（场景 3/4 + 日志 1/4，输入行上方固定 liveSceneRows 行）
	drawSceneBlock(prompt, buf, s.daughter.Home)
	lastSceneVer := liveFrameVersion()
	lastAnimTick := time.Now()

	for {
		// 实时直播轮询：无按键且（直播帧更新 或 每 400ms 动画节拍）→ 覆写直播屏
		// 400ms 节拍 = 颜表情帧轮播（moodEmoji 600ms）+ 日志滚动 → 任何时候都有动画
		if !inputAvailable() {
			now := time.Now()
			if v := liveFrameVersion(); v != lastSceneVer || now.Sub(lastAnimTick) > 400*time.Millisecond {
				lastSceneVer = v
				lastAnimTick = now
				if !hadCandidates {
					overwriteScene(prompt, buf, s.daughter.Home)
				}
			}
			time.Sleep(80 * time.Millisecond)
			continue
		}

		kind, r, err := readKey()
		if err != nil {
			return "", err
		}

		switch kind {
		case keyEnter:
			// 有选中候选且输入是未完成的 / 命令 → 用选中项确认执行
			if selIdx >= 0 && strings.HasPrefix(string(buf), "/") {
				buf = []rune("/" + matches[selIdx])
			}
			// 清掉候选区 + 输入行（候选在输入行上方；先 \r 归位再上移，避免列残留）
			if hadCandidates && oldCandRows > 0 {
				fmt.Print("\r")
				fmt.Printf("\x1b[%dA", oldCandRows)
				fmt.Print("\x1b[J")
			} else {
				fmt.Print("\r\x1b[K")
			}
			hadCandidates = false
			fmt.Println()
			return string(buf), nil

		case keyTab:
			// Tab：补全当前选中（或唯一）候选
			if selIdx >= 0 {
				buf = []rune("/" + matches[selIdx])
				refreshCandidates() // 完整命令不再有候选 → 关闭列表
				redraw()
			}

		case keyBackspace:
			if len(buf) > 0 {
				buf = buf[:len(buf)-1]
				refreshCandidates()
				redraw()
			}

		case keyUp:
			if selIdx >= 0 {
				if selIdx > 0 {
					selIdx--
					redraw()
				}
			} else if histIdx > 0 {
				histIdx--
				buf = []rune(s.history[histIdx])
				refreshCandidates()
				redraw()
			}

		case keyDown:
			if selIdx >= 0 {
				if selIdx < len(matches)-1 {
					selIdx++
					redraw()
				}
			} else if histIdx < len(s.history) {
				histIdx++
				if histIdx == len(s.history) {
					buf = nil
				} else {
					buf = []rune(s.history[histIdx])
				}
				refreshCandidates()
				redraw()
			}

		case keyLeft, keyRight:
			// 暂不处理光标移动

		case keyEsc:
			// 关闭候选列表
			if selIdx >= 0 {
				matches = nil
				selIdx = -1
				redraw()
			}
		case keyCtrlC:
			restore() // 先恢复终端再退出，避免卡死
			fmt.Println("^C")
			gracefulExit()

		case keyCtrlD:
			fmt.Println()
			return "", io.EOF

		case keyRune:
			buf = append(buf, r)
			refreshCandidates()
			redraw()
		}
	}
}

// matchCommands 返回当前行匹配的 / 命令候选（按字母序，不含 / 前缀）
// 输入 "/" 时返回全部命令；已完整输入的命令不再作为候选
func (s *Shell) matchCommands(line string) []string {
	if !strings.HasPrefix(line, "/") {
		return nil
	}
	prefix := strings.TrimPrefix(line, "/")
	var ms []string
	for _, c := range slashCommands {
		if strings.HasPrefix(c, prefix) && c != prefix {
			ms = append(ms, c)
		}
	}
	sort.Strings(ms)
	return ms
}

// complete 返回补全结果：唯一匹配直接补全；多匹配返回候选列表
func (s *Shell) complete(line string) (string, []string) {
	ms := s.matchCommands(line)
	if len(ms) == 1 {
		return "/" + ms[0], nil
	}
	if len(ms) > 1 {
		return line, ms
	}
	return line, nil
}

// readKey 读取一个按键（含 UTF-8 中文、方向键转义序列）
func readKey() (keyKind, rune, error) {
	b, err := readByte()
	if err != nil {
		return keyUnknown, 0, err
	}

	switch b {
	case 0x0D, 0x0A: // CR / LF
		return keyEnter, 0, nil
	case 0x09: // Tab
		return keyTab, 0, nil
	case 0x7F, 0x08: // DEL / BS
		return keyBackspace, 0, nil
	case 0x03: // Ctrl+C
		return keyCtrlC, 0, nil
	case 0x04: // Ctrl+D
		return keyCtrlD, 0, nil
	case 0x1B: // ESC 或方向键序列
		return readEscapeSeq()
	}

	// 多字节 UTF-8：按需读足后续字节
	if b >= 0x80 {
		seq := []byte{b}
		for utf8.FullRune(seq) == false && len(seq) < 4 {
			nb, err := readByte()
			if err != nil {
				break
			}
			seq = append(seq, nb)
		}
		r, _ := utf8.DecodeRune(seq)
		return keyRune, r, nil
	}
	return keyRune, rune(b), nil
}

// readEscapeSeq 解析 ESC 序列：方向键 ESC[A-D；独立 Esc 直接返回
func readEscapeSeq() (keyKind, rune, error) {
	// 关键：先探测是否有后续字节。没有 → 这是独立的 Esc 键
	if !inputAvailable() {
		return keyEsc, 0, nil
	}

	b2, err := readByte()
	if err != nil {
		return keyEsc, 0, nil
	}
	if b2 != '[' {
		return keyEsc, 0, nil
	}
	// 再探测是否有第三个字节（[A 的 A）
	if !inputAvailable() {
		return keyEsc, 0, nil
	}
	b3, err := readByte()
	if err != nil {
		return keyEsc, 0, nil
	}
	switch b3 {
	case 'A':
		return keyUp, 0, nil
	case 'B':
		return keyDown, 0, nil
	case 'C':
		return keyRight, 0, nil
	case 'D':
		return keyLeft, 0, nil
	default:
		return keyUnknown, 0, nil
	}
}

// readByte 读单个字节
func readByte() (byte, error) {
	var buf [1]byte
	n, err := os.Stdin.Read(buf[:])
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, io.EOF
	}
	return buf[0], nil
}
