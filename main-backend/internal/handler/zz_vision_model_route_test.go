package handler

// 识图模型路由测试（chat_engines_gemini_vision.go 的 analyzeImageWithModelID +
// HandleAetherVisionPreprocess 的回退链）。不打网络：resolveExact 对未知 ID 直接
// 返回 nil，默认视觉模型路径在没有可用 Vision 模型时也会在发请求前就短路返回错误。

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

// 设置面板「模型」页把识图模型接进来的落点是 model_router 的 resolveExact——
// 确认免费池里 Vision 标记的条目能解析出 backend（本地 llama 已移除，2026-08-01）。
func TestVisionModelResolvesInCatalog(t *testing.T) {
	b := resolveExact("", "free_step_1o_turbo_vision")
	if b == nil {
		// 没配 STEP_API_KEY 时 resolveExact 返回 nil 属正常，跳过断言
		t.Skip("无 STEP_API_KEY 环境变量，跳过精确命中断言")
	}
	if !b.Vision {
		t.Errorf("free_step_1o_turbo_vision 的 Vision 标记应为 true，got false")
	}
	if b.BaseURL == "" {
		t.Errorf("识图模型缺少 BaseURL")
	}
}

func TestHandleAetherVisionPreprocess_AllPathsFail(t *testing.T) {
	// 不设置任何视觉模型相关环境变量，且指定模型是未知 ID——
	// 两条路径（指定模型 + 默认视觉模型）都该短路失败，返回 502，而不是 panic 或挂起。
	// 注意：若测试环境恰好配了 STEP_API_KEY 等，visionBackends 会真的发起网络调用，
	// 本测试通过 unknown model 分支独立验证回退链的错误处理（不打网络）。
	t.Setenv("VISION_MODEL_ID", "totally-bogus-vision-id") // 强制默认视觉模型解析失败

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/aether/vision-preprocess", HandleAetherVisionPreprocess)

	body := `{"image_base64":"aGVsbG8=","mime_type":"image/png","model":"totally-bogus-model-id"}`
	req := httptest.NewRequest(http.MethodPost, "/api/aether/vision-preprocess", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// 指定模型失败 + 默认视觉模型失败（VISION_MODEL_ID 无效）→ 502
	if w.Code != http.StatusBadGateway {
		t.Fatalf("预期 502（两条路径都失败），got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "视觉预处理失败") {
		t.Errorf("响应体应包含失败提示，got: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "GEMINI_API_KEY") {
		t.Errorf("错误提示不应再指向 GEMINI_API_KEY（Gemini 已移除），got: %s", w.Body.String())
	}
}
