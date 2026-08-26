package handler

import (
	"strings"
	"testing"
)

func TestRewriteDshBaseURL_EmptyProviders(t *testing.T) {
	src := "llm-pi-ai:\n  providers: {}\nagent-default-model:\n  provider: rescene\n  model: auto\n"
	out := rewriteDshBaseURL(src, "http://localhost:8080/v1")
	if !strings.Contains(out, "baseURL: http://localhost:8080/v1") {
		t.Fatalf("baseURL not rewritten:\n%s", out)
	}
	if strings.Contains(out, "{}") {
		t.Fatalf("old providers: {} still present:\n%s", out)
	}
	if !strings.Contains(out, "rescene:") {
		t.Fatalf("rescene section missing:\n%s", out)
	}
}

func TestRewriteDshBaseURL_NoProviders(t *testing.T) {
	src := "llm-pi-ai:\nagent-default-model:\n  provider: rescene\n  model: auto\n"
	out := rewriteDshBaseURL(src, "http://localhost:8080/v1")
	if !strings.Contains(out, "baseURL: http://localhost:8080/v1") {
		t.Fatalf("baseURL not rewritten:\n%s", out)
	}
}
