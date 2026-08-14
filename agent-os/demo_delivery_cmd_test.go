package main

import "testing"

func TestParseDirectiveDeliveryArgsKeepsSelectedModel(t *testing.T) {
	model, directive := parseDirectiveDeliveryArgs([]string{"--model", "custom::cfg_123::deepseek-v4-pro", "--", "做一个", "番茄钟"})
	if model != "custom::cfg_123::deepseek-v4-pro" {
		t.Fatalf("model = %q", model)
	}
	if directive != "做一个 番茄钟" {
		t.Fatalf("directive = %q", directive)
	}
	if got := normalizeDirectiveModel(model); got != "deepseek_v4_pro" {
		t.Fatalf("normalized model = %q", got)
	}
}
