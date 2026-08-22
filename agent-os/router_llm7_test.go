package main

import "testing"

func TestLLM7ModelsAreKeylessFreeModels(t *testing.T) {
	want := map[string]string{
		"free_llm7_deepseek_v4_flash": "DeepSeek-V4-Flash-0731",
		"free_llm7_codestral":         "codestral-latest",
		"free_llm7_gemini_flash_lite": "gemini-3.1-flash-lite",
		"free_llm7_gpt_oss_20b":       "gpt-oss:20b",
		"free_llm7_llama_3_1_8b":      "meta-Llama-3.1-8B-Instruct-Turbo",
		"free_llm7_minimax_m2_7":      "minimax-m2.7",
	}

	found := map[string]bool{}
	for _, model := range freeModels {
		upstream, ok := want[model.ID]
		if !ok {
			continue
		}
		found[model.ID] = true
		if model.Model != upstream || model.Endpoint != "https://api.llm7.io/v1" {
			t.Errorf("%s 路由错误: %+v", model.ID, model)
		}
		if !model.Keyless || !isFreeModel(model) {
			t.Errorf("%s 必须作为免 key 免费模型入池: %+v", model.ID, model)
		}
	}
	for id := range want {
		if !found[id] {
			t.Errorf("LLM7 模型未同步到 Agent OS: %s", id)
		}
	}
}

func TestKeyedFreeProvidersAreSyncedToAgentOS(t *testing.T) {
	want := map[string]string{
		"free_google_gemini_2_5_flash": "gemini-2.5-flash",
		"free_groq_llama_3_3_70b":      "llama-3.3-70b-versatile",
		"free_groq_qwen3_32b":          "qwen/qwen3-32b",
		"free_openrouter_router":       "openrouter/free",
	}
	found := map[string]bool{}
	for _, model := range freeModels {
		upstream, ok := want[model.ID]
		if !ok {
			continue
		}
		found[model.ID] = true
		if model.Model != upstream || model.Keyless || model.KeyEnv == "" || model.KeyURL == "" {
			t.Errorf("%s 的 Agent OS 提供方配置错误: %+v", model.ID, model)
		}
	}
	for id := range want {
		if !found[id] {
			t.Errorf("Agent OS 缺少提供方条目: %s", id)
		}
	}
}
