package handler

import (
	"encoding/json"
	"testing"
)

func TestNativeCommandReadOnlyApprovalBoundary(t *testing.T) {
	for _, command := range []string{
		"git status",
		"cd src; git diff --stat",
		"Set-Location main-backend; git log -1",
	} {
		if !isReadOnlyToolCall("run_command", `{"command":`+quoteJSONForTest(command)+`}`) {
			t.Errorf("只读命令被误判为危险: %s", command)
		}
	}
	for _, command := range []string{
		"git status > status.txt",
		"git status | Remove-Item secret.txt",
		"git status $(Remove-Item secret.txt)",
		"git status; Remove-Item secret.txt",
		"npm test",
	} {
		if isReadOnlyToolCall("run_command", `{"command":`+quoteJSONForTest(command)+`}`) {
			t.Errorf("有副作用的命令被误放行为只读: %s", command)
		}
	}
}

func quoteJSONForTest(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func TestCappedCommandBuffer(t *testing.T) {
	buf := newCappedCommandBuffer(4)
	n, err := buf.Write([]byte("abcdefgh"))
	if err != nil || n != 8 {
		t.Fatalf("Writer 契约不正确: n=%d err=%v", n, err)
	}
	if got := buf.String(); got[:4] != "abcd" || !buf.truncated {
		t.Fatalf("输出未正确限流: %q", got)
	}
}
