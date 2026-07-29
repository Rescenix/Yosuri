package handler

// 识图模型路由测试（chat_engines_gemini_vision.go 的 analyzeImageWithModelID +
// HandleAetherVisionPreprocess 的回退链）。不打网络：resolveExact 对未知 ID 直接
// 返回 nil，Gemini 路径在拿到空 GEMINI_API_KEY 时也在发请求前就短路返回错误。

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestAnalyzeImageWithModelID_UnknownModel(t *testing.T) {
	_, err := analyzeImageWithModelID(context.Background(), "totally-bogus-model-id", "", "image/png", "")
	if err == nil || !strings.Contains(err.Error(), "未找到") {
		t.Fatalf("未知模型 ID 应报「未找到」，got: %v", err)
	}
}

func TestAnalyzeImageWithBackend_DoesNotRejectUnknownVisionMetadata(t *testing.T) {
	var requestBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		requestBody = string(raw)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"识图成功"}}]}`))
	}))
	defer upstream.Close()

	backend := RouterBackend{
		Name: "自定义多模态模型", BaseURL: upstream.URL, Model: "multimodal-model",
		APIKey: "test-key", Timeout: time.Second,
		// 模拟 /models 没有返回能力字段：实际支持识图，但目录元数据为 false。
		Vision: false,
	}
	got, err := analyzeImageWithBackend(
		context.Background(), backend, "aGVsbG8=", "image/png", "描述图片",
	)
	if err != nil {
		t.Fatalf("未知视觉能力元数据不应阻止真实调用: %v", err)
	}
	if got != "识图成功" {
		t.Fatalf("识图响应不匹配: %q", got)
	}
	for _, want := range []string{
		`"model":"multimodal-model"`,
		`"type":"image_url"`,
		`data:image/png;base64,aGVsbG8=`,
	} {
		if !strings.Contains(requestBody, want) {
			t.Errorf("上游请求缺少 %q: %s", want, requestBody)
		}
	}
}

// 设置面板「模型」页把本地 llama.cpp 识图模型接进来的落点就是这个 catalog 条目——
// 确认它在路由层确实解析得到、且带 Vision=true，否则前端选了也白选。
func TestLocalLlamaVisionModelResolvesInCatalog(t *testing.T) {
	b := resolveExact("", "local_llama_qwen2_5_vl_7b")
	if b == nil {
		t.Fatal("local_llama_qwen2_5_vl_7b 应能被 resolveExact 解析（Local=true 不需要 Key）")
	}
	if !b.Vision {
		t.Errorf("本地 llama 识图模型的 Vision 标记应为 true，got false")
	}
	if b.BaseURL == "" {
		t.Errorf("本地 llama 识图模型缺少 BaseURL")
	}
}

func TestHandleAetherVisionPreprocess_FallsBackToGeminiOnBadModel(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "") // 强制 Gemini 回退路径也短路失败，隔离网络依赖

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/aether/vision-preprocess", HandleAetherVisionPreprocess)

	body := `{"image_base64":"aGVsbG8=","mime_type":"image/png","model":"totally-bogus-model-id"}`
	req := httptest.NewRequest(http.MethodPost, "/api/aether/vision-preprocess", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 两条路径（指定模型 + Gemini 回退）都该失败，返回 502，而不是 panic 或挂起
	if w.Code != http.StatusBadGateway {
		t.Fatalf("预期 502（两条路径都失败），got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "视觉预处理失败") {
		t.Errorf("响应体应包含失败提示，got: %s", w.Body.String())
	}
}
