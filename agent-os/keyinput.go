package main

// keyinput.go — 交互式逐键输入：Tab 补全 + 方向键历史 + 中文输入
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

// readLine 读取一行输入（支持补全/历史/中文）；非终端时回退 bufio 整行读
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

	redraw := func() {
		fmt.Print("\r\x1b[K") // 回车 + 清行
		fmt.Print(prompt)
		fmt.Print(string(buf))
	}

	redraw()
	for {
		kind, r, err := readKey()
		if err != nil {
			return "", err
		}

		switch kind {
		case keyEnter:
			fmt.Println()
			return string(buf), nil

		case keyTab:
			line := string(buf)
			completed, matches := s.complete(line)
			if completed != line {
				buf = []rune(completed)
				redraw()
			} else if len(matches) > 0 {
				fmt.Println()
				fmt.Println(ColorYellow + "  " + strings.Join(matches, "   ") + ColorReset)
				redraw()
			}

		case keyBackspace:
			if len(buf) > 0 {
				buf = buf[:len(buf)-1]
				redraw()
			}

		case keyUp:
			if histIdx > 0 {
				histIdx--
				buf = []rune(s.history[histIdx])
				redraw()
			}

		case keyDown:
			if histIdx < len(s.history) {
				histIdx++
				if histIdx == len(s.history) {
					buf = nil
				} else {
					buf = []rune(s.history[histIdx])
				}
				redraw()
			}

		case keyLeft, keyRight, keyEsc:
			// 暂不处理光标移动，忽略

		case keyCtrlC:
			fmt.Println("^C")
			gracefulExit()

		case keyCtrlD:
			fmt.Println()
			return "", io.EOF

		case keyRune:
			buf = append(buf, r)
			redraw()
		}
	}
}

// complete 返回补全结果：唯一匹配直接补全；多匹配返回候选列表
func (s *Shell) complete(line string) (string, []string) {
	if !strings.HasPrefix(line, "/") {
		return line, nil
	}
	prefix := strings.TrimPrefix(line, "/")
	var matches []string
	for _, c := range slashCommands {
		if strings.HasPrefix(c, prefix) && c != prefix {
			matches = append(matches, c)
		}
	}
	if len(matches) == 1 {
		return "/" + matches[0], nil
	}
	if len(matches) > 1 {
		sort.Strings(matches)
		return line, matches
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
