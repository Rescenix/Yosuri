package handler

import (
	"strings"
	"testing"
	"time"
)

func TestTerminalSessionRunsRealCommand(t *testing.T) {
	id := "test-" + t.Name()
	t.Cleanup(func() {
		termRegistryMu.Lock()
		s, ok := termRegistry[id]
		delete(termRegistry, id)
		termRegistryMu.Unlock()
		if ok && s.cmd.Process != nil {
			s.cmd.Process.Kill()
		}
	})

	s, err := getOrCreateTermSession(id)
	if err != nil {
		t.Fatalf("getOrCreateTermSession failed: %v", err)
	}

	ch := s.subscribe()
	defer s.unsubscribe(ch)

	marker := "TERMTEST_MARKER_12345"
	if _, err := s.stdin.Write([]byte("Write-Host " + marker + "\r\n")); err != nil {
		t.Fatalf("write to stdin failed: %v", err)
	}

	var collected strings.Builder
	deadline := time.After(10 * time.Second)
	for {
		select {
		case p, ok := <-ch:
			if !ok {
				t.Fatalf("session closed before marker seen, got: %s", collected.String())
			}
			collected.Write(p)
			if strings.Contains(collected.String(), marker) {
				return // 命令真的执行了，输出真的传回来了
			}
		case <-deadline:
			t.Fatalf("timeout waiting for marker, got: %s", collected.String())
		}
	}
}
