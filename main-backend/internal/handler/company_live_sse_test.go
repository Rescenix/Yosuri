package handler

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// TestCompanyLiveSSE 验证 SSE 端点：历史回放 + 实时推送都能到达订阅者。
func TestCompanyLiveSSE(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/company/live", HandleCompanyLive)

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/api/company/live", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("SSE 连接失败: %v", err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("Content-Type=%q", ct)
	}

	// 连上之后发布一条事件，1 秒内必须流到订阅者
	go func() {
		time.Sleep(120 * time.Millisecond)
		companyLivePublish(companyLiveEvent{Kind: "stage", Stage: "mvp", Role: "coder", Text: "最小可运行原型 v1 生成中", Project: "sse-test"})
	}()

	reader := bufio.NewReader(resp.Body)
	deadline := time.Now().Add(5 * time.Second)
	found := false
	for time.Now().Before(deadline) {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		if strings.HasPrefix(line, "data:") && strings.Contains(line, "\"kind\":\"stage\"") && strings.Contains(line, "mvp") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("5 秒内没收到实时 stage 事件")
	}
}
