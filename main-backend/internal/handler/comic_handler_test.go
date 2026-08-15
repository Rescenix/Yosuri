package handler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func tinyComicPNG(t *testing.T) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 12))
	for y := 0; y < 12; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: 180, G: uint8(80 + x*10), B: uint8(100 + y*8), A: 255})
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(out.Bytes())
}

func TestComicLocalSDRenderAndAssemble(t *testing.T) {
	gin.SetMode(gin.TestMode)
	encoded := tinyComicPNG(t)
	sd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sdapi/v1/options" {
			_ = json.NewEncoder(w).Encode(map[string]any{"sd_model_checkpoint": "test-model"})
			return
		}
		if r.URL.Path == "/sdapi/v1/txt2img" {
			_ = json.NewEncoder(w).Encode(map[string]any{"images": []string{encoded}})
			return
		}
		http.NotFound(w, r)
	}))
	defer sd.Close()
	t.Setenv("RESCENE_SD_URL", sd.URL)
	root := t.TempDir()
	t.Setenv("RESCENE_COMICS_DIR", root)
	pageID := "comic-test"
	if err := os.MkdirAll(filepath.Join(root, pageID), 0o755); err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	router.GET("/status", HandleComicStatus)
	router.POST("/start", HandleComicStartSD)
	router.POST("/render", HandleComicRenderPanel)
	router.POST("/assemble", HandleComicAssemble)

	status := httptest.NewRecorder()
	router.ServeHTTP(status, httptest.NewRequest(http.MethodGet, "/status", nil))
	if status.Code != http.StatusOK || !bytes.Contains(status.Body.Bytes(), []byte(`"online":true`)) {
		t.Fatalf("status: %d %s", status.Code, status.Body.String())
	}
	start := httptest.NewRecorder()
	router.ServeHTTP(start, httptest.NewRequest(http.MethodPost, "/start", nil))
	if start.Code != http.StatusOK || !bytes.Contains(start.Body.Bytes(), []byte(`"online":true`)) {
		t.Fatalf("start online SD: %d %s", start.Code, start.Body.String())
	}

	for index := 0; index < 2; index++ {
		body, _ := json.Marshal(map[string]any{"pageId": pageID, "panel": map[string]any{"index": index, "scene": "雨夜", "promptEn": "anime rainy night"}})
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/render", bytes.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("render %d: %d %s", index, response.Code, response.Body.String())
		}
	}
	body, _ := json.Marshal(map[string]any{"pageId": pageID, "title": "测试", "dialogue": []string{"一", "二"}})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/assemble", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("assemble: %d %s", response.Code, response.Body.String())
	}
	if info, err := os.Stat(filepath.Join(root, pageID, "comic_page.png")); err != nil || info.Size() == 0 {
		t.Fatalf("assembled page missing: %v", err)
	}
}
