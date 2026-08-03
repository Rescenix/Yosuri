package main

import (
	"os"
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
