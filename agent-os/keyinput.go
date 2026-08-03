package main

// keyinput.go — 交互式逐键输入：实时补全 + ↑↓选择 + Tab 补全 + 历史 + 中文
// raw mode 由 rawmode_windows.go / rawmode_unix.go 提供

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
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
	"shell", "agent", "refresh", "history", "env", "report", "rep",
	"marathon", "exec", "update", "version",
}

// 候选列表最多显示行数
const maxCandidates = 9

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

	// 根据当前输入刷新候选列表（仅 / 命令前缀）
	refreshCandidates := func() {
		matches = s.matchCommands(string(buf))
		if len(matches) > 0 {
			selIdx = 0
		} else {
			selIdx = -1
		}
	}

	// 全量重绘：输入行 + 候选列表（光标回到输入行行首）
	redraw := func() {
		fmt.Print("\r\x1b[K")
		fmt.Print(prompt + string(buf))
		if len(matches) > 0 {
			n := len(matches)
			rows := n
			if n > maxCandidates {
				n = maxCandidates
				rows = maxCandidates + 1
			}
			for i := 0; i < n; i++ {
				fmt.Print("\n")
				if i == selIdx {
					fmt.Print(ColorCyan + "▸ " + matches[i] + ColorReset)
				} else {
					fmt.Print("  " + matches[i])
				}
			}
			if len(matches) > maxCandidates {
				fmt.Print("\n  …")
			}
			fmt.Printf("\x1b[%dA\r", rows) // 光标上移回输入行
			fmt.Print("\x1b[J")            // 清掉输入行以下的残留
		}
	}

	redraw()
	for {
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

// readEscapeSeq 解析 ESC [...] 方向键序列
func readEscapeSeq() (keyKind, rune, error) {
	// 立即读第二个字节判断是否为 '[' 序列
	b2, err := readByte()
	if err != nil {
		return keyEsc, 0, nil
	}
	if b2 != '[' {
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
