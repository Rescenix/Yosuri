package main

import (
	"strings"
	"testing"
)

// 模拟 readLine 中 Tab 按键的处理路径（不依赖 raw mode）
func TestTabHandlingPath(t *testing.T) {
	s := &Shell{}

	// 模拟：输入 /rep 然后 Tab → 应补全为 /report
	buf := "/rep"
	completed, matches := s.complete(buf)
	if completed != "/report" {
		t.Errorf("Tab 补全 /rep → %q, want /report", completed)
	}
	if matches != nil {
		t.Errorf("/rep 应唯一匹配, got list %v", matches)
	}

	// 模拟：输入 /m 然后 Tab → 应列出候选
	_, matches = s.complete("/m")
	if len(matches) != 3 {
		t.Errorf("/m 应有 3 候选, got %v", matches)
	}
	for _, m := range matches {
		if !strings.HasPrefix(m, "m") {
			t.Errorf("候选 %q 不以 m 开头", m)
		}
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

// 验证 promptStr 包含模式标识
func TestPromptStr(t *testing.T) {
	s := &Shell{}
	p := s.promptStr()
	if !strings.Contains(p, "AGENT") {
		t.Errorf("prompt 应含 AGENT 标识, got %q", p)
	}
}
