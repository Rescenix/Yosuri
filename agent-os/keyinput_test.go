package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestComplete(t *testing.T) {
	s := &Shell{}

	cases := []struct {
		in       string
		wantLine string
		wantList []string
	}{
		// 唯一匹配 → 直接补全
		{"/rep", "/report", nil},
		{"/hi", "/history", nil},
		{"/mar", "/marathon", nil},
		// 多匹配 → 返回候选列表（字母序）
		{"/ex", "/ex", []string{"exec", "exit"}},
		{"/m", "/m", []string{"marathon", "model", "models"}},
		{"/e", "/e", []string{"env", "exec", "exit"}},
		// 无匹配 → 原样返回
		{"/zzz", "/zzz", nil},
		// 非 / 前缀 → 不补全
		{"hello", "hello", nil},
		{"rep", "rep", nil},
		// 完全匹配已存在 → 不重复补全
		{"/report", "/report", nil},
	}

	for _, c := range cases {
		gotLine, gotList := s.complete(c.in)
		if gotLine != c.wantLine {
			t.Errorf("complete(%q) line = %q, want %q", c.in, gotLine, c.wantLine)
		}
		if !reflect.DeepEqual(gotList, c.wantList) {
			t.Errorf("complete(%q) list = %v, want %v", c.in, gotList, c.wantList)
		}
	}
}

// 实时补全：输入 / 应显示全部命令候选
func TestMatchAllOnSlash(t *testing.T) {
	s := &Shell{}
	ms := s.matchCommands("/")
	if len(ms) != len(slashCommands) {
		t.Errorf("输入 / 应返回全部 %d 命令, got %d", len(slashCommands), len(ms))
	}
	// 应已排序
	for i := 1; i < len(ms); i++ {
		if ms[i-1] > ms[i] {
			t.Errorf("候选未排序: %v", ms)
		}
	}
}

// 实时补全：输入 /de 应显示 de 开头的候选
func TestMatchPrefix(t *testing.T) {
	s := &Shell{}
	ms := s.matchCommands("/re")
	if len(ms) == 0 {
		t.Fatal("/re 应有候选")
	}
	for _, m := range ms {
		if !strings.HasPrefix(m, "re") {
			t.Errorf("候选 %q 不以 re 开头", m)
		}
	}
	// 完整命令不再作为候选
	ms = s.matchCommands("/report")
	for _, m := range ms {
		if m == "report" {
			t.Errorf("完整命令 /report 不应再列为候选")
		}
	}
}

// 模拟 readLine 中 Tab 按键的处理路径（不依赖 raw mode）
func TestTabHandlingPath(t *testing.T) {
	s := &Shell{}

	// 模拟：输入 /rep 然后 Tab → 应补全为 /report
	completed, matches := s.complete("/rep")
	if completed != "/report" {
		t.Errorf("Tab 补全 /rep → %q, want /report", completed)
	}
	if matches != nil {
		t.Errorf("/rep 应唯一匹配, got list %v", matches)
	}
}

// 验证历史导航（方向键 ↑↓ 的数据路径）
func TestHistoryNavigation(t *testing.T) {
	s := &Shell{}
	s.history = []string{"ls", "/models", "git status"}

	// ↑ 从末尾回看
	histIdx := len(s.history)
	if histIdx > 0 {
		histIdx--
		if got := s.history[histIdx]; got != "git status" {
			t.Errorf("↑ 应为 git status, got %q", got)
		}
	}
	// 再 ↑
	if histIdx > 0 {
		histIdx--
		if got := s.history[histIdx]; got != "/models" {
			t.Errorf("↑↑ 应为 /models, got %q", got)
		}
	}
}

// 验证 promptStr 格式正确
func TestPromptStr(t *testing.T) {
	s := &Shell{}
	p := s.promptStr()
	if !strings.Contains(p, "$") {
		t.Errorf("prompt 应含 $, got %q", p)
	}
}
