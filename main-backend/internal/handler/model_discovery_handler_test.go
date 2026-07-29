package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProviderModelsURL(t *testing.T) {
	tests := map[string]string{
		"https://api.example.com/v1":                  "https://api.example.com/v1/models",
		"https://api.example.com/v1/":                 "https://api.example.com/v1/models",
		"https://api.example.com/v1/models":           "https://api.example.com/v1/models",
		"https://api.example.com/v1/chat/completions": "https://api.example.com/v1/models",
	}
	for input, want := range tests {
		if got := providerModelsURL(input); got != want {
			t.Errorf("providerModelsURL(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestFetchProviderModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("模型目录路径 = %q, want /v1/models", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-api-key" {
			t.Errorf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[
			{"id":"model-a","name":"Model A"},
			{"id":"model-b"},
			{"id":"model-a","name":"duplicate"}
		]}`))
	}))
	defer server.Close()

	models, err := fetchProviderModels(context.Background(), server.URL+"/v1", "test-api-key")
	if err != nil {
		t.Fatalf("fetchProviderModels() error = %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("模型数量 = %d, want 2", len(models))
	}
	if models[0].ID != "model-a" || models[0].Name != "Model A" {
		t.Fatalf("第一个模型 = %+v", models[0])
	}
	if models[1].ID != "model-b" || models[1].Name != "model-b" {
		t.Fatalf("缺少 name 时应回退到 id，实得 %+v", models[1])
	}
}

func TestCustomModelSelectionIDRoundTrip(t *testing.T) {
	selectionID := customModelSelectionID("cfg:用户", "org/model+vision")
	providerID, modelID, ok := parseCustomModelSelectionID(selectionID)
	if !ok {
		t.Fatalf("无法解析 selection id: %q", selectionID)
	}
	if providerID != "cfg:用户" || modelID != "org/model+vision" {
		t.Fatalf("round trip = (%q, %q)", providerID, modelID)
	}
}
