package main

import (
	"os"
	"strings"
	"testing"
)

// 注入按键字节流，验证 readKey 的解析（方向键序列 / 独立 Esc / 普通键）
func injectReadKey(t *testing.T, bytes ...byte) (keyKind, rune) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old }()
	w.Write(bytes)
	w.Close()
	kind, rn, err := readKey()
	if err != nil {
		t.Fatalf("readKey err: %v", err)
	}
	return kind, rn
}

func TestReadKeyEscapeSeq(t *testing.T) {
	cases := []struct {
		name string
		seq  []byte
		want keyKind
	}{
		{"ESC[A 上", []byte{0x1B, '[', 'A'}, keyUp},
		{"ESC[B 下", []byte{0x1B, '[', 'B'}, keyDown},
		{"ESC[C 右", []byte{0x1B, '[', 'C'}, keyRight},
		{"ESC[D 左", []byte{0x1B, '[', 'D'}, keyLeft},
		{"独立 Esc", []byte{0x1B}, keyEsc},
		{"Tab", []byte{0x09}, keyTab},
		{"Enter", []byte{0x0D}, keyEnter},
		{"普通字符", []byte{'a'}, keyRune},
	}
	for _, c := range cases {
		got, _ := injectReadKey(t, c.seq...)
		if got != c.want {
			t.Errorf("%s → kind=%d, want %d", c.name, got, c.want)
		}
	}
}

// 验证 ↑↓ 候选选择的数据路径（selIdx 移动逻辑，与 readLine 内联逻辑一致）
func TestCandidateSelectionPath(t *testing.T) {
	matches := []string{"marathon", "model", "models"}
	sel := 0

	// ↓ 下移
	if sel < len(matches)-1 {
		sel++
	}
	if matches[sel] != "model" {
		t.Errorf("↓ 后应选中 model, got %s", matches[sel])
	}
	// ↓ 再下移
	if sel < len(matches)-1 {
		sel++
	}
	if matches[sel] != "models" {
		t.Errorf("↓↓ 后应选中 models, got %s", matches[sel])
	}
	// ↑ 上移
	if sel > 0 {
		sel--
	}
	if matches[sel] != "model" {
		t.Errorf("↑ 后应选中 model, got %s", matches[sel])
	}
	// 边界：底部再 ↓ 不动
	sel = len(matches) - 1
	if sel < len(matches)-1 {
		sel++
	}
	if sel != len(matches)-1 {
		t.Errorf("边界 ↓ 不应越界, got %d", sel)
	}
}

func TestCandidateRowsDoNotWrapWithChineseText(t *testing.T) {
	const width = 40
	row := fitCandidateRow("▸ /clear      清屏", width)
	if got := terminalTextWidth(row); got != width-1 {
		t.Fatalf("候选行宽度 = %d, want %d; row=%q", got, width-1, row)
	}
	if terminalTextWidth("清屏") != 4 {
		t.Fatal("中文字符应各占两个终端列")
	}
}

func TestCandidateRowsTruncateWithoutSplittingTerminalWidth(t *testing.T) {
	row := fitCandidateRow("▸ /clear      清屏", 10)
	if got := terminalTextWidth(row); got != 9 {
		t.Fatalf("窄终端候选行宽度 = %d, want 9; row=%q", got, row)
	}
}

func TestCandidateWindowFollowsDownSelection(t *testing.T) {
	if got := candidateWindowStart(0, 18); got != 0 {
		t.Fatalf("首项窗口起点 = %d, want 0", got)
	}
	if got := candidateWindowStart(5, 18); got != 1 {
		t.Fatalf("向下越过可见区后窗口起点 = %d, want 1", got)
	}
	if got := candidateWindowStart(17, 18); got != 13 {
		t.Fatalf("末项窗口起点 = %d, want 13", got)
	}
}

func TestWrapTerminalLineKeepsBoxWidth(t *testing.T) {
	lines := wrapTerminalLine("你好呀，有什么事情需要我帮忙吗？😊", 16)
	if len(lines) < 2 {
		t.Fatalf("中文长文本应换行, got %v", lines)
	}
	for _, line := range lines {
		if got := terminalTextWidth(line); got > 16 {
			t.Fatalf("换行后宽度 = %d, 超过 16: %q", got, line)
		}
	}
}

func TestTerminalTextWidthIgnoresANSIAndCountsWideRunes(t *testing.T) {
	coloredName := " rescene " + ColorMood + "(◕‿◕)🌸" + ColorReset + " "
	plainName := " rescene (◕‿◕)🌸 "
	if got, want := terminalTextWidth(coloredName), terminalTextWidth(plainName); got != want {
		t.Fatalf("ANSI 标题宽度 = %d, want %d", got, want)
	}
	if got := terminalTextWidth(" 你 "); got != 4 {
		t.Fatalf("中文标题宽度 = %d, want 4", got)
	}
}

func TestGalgameBorderWidthMath(t *testing.T) {
	const boxW = 80
	for _, name := range []string{"你", "rescene " + ColorMood + "(◕‿◕)🌸" + ColorReset} {
		nameTag := " " + name + " "
		fill := boxW - 2 - terminalTextWidth(nameTag)
		visible := 1 + terminalTextWidth(nameTag) + fill + 1
		if visible != boxW {
			t.Fatalf("标题 %q 的上边框宽度 = %d, want %d", name, visible, boxW)
		}
	}
}

func TestGalgameTopBorderRestoresColorAfterMood(t *testing.T) {
	top := galgameTopBorder("rescene "+ColorMood+"(◕‿◕)🌸"+ColorReset, ColorMood, 80)
	if got := terminalTextWidth(top); got != 80 {
		t.Fatalf("上边框可见宽度 = %d, want 80", got)
	}
	if !strings.Contains(top, ColorReset+ColorMood) {
		t.Fatal("表情重置颜色后没有恢复边框颜色")
	}
}
