package handler

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestIsBAIEndpoint(t *testing.T) {
	if !isBAIEndpoint("https://api.b.ai/v1") || !isBAIEndpoint("https://API.B.AI/v1/chat/completions") {
		t.Fatal("api.b.ai 应启用 B.AI DoH 适配")
	}
	if isBAIEndpoint("https://docs.b.ai/") || isBAIEndpoint("https://api.example.com/v1") {
		t.Fatal("非 api.b.ai 端点不应改变网络解析")
	}
}

func TestParseBAIDoHResponse(t *testing.T) {
	raw := []byte(`{"Answer":[{"type":1,"TTL":300,"data":"104.21.2.129"},{"type":1,"TTL":120,"data":"172.67.152.206"},{"type":28,"TTL":60,"data":"::1"}]}`)
	ips, ttl, err := parseBAIDoHResponse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(ips) != 2 || ips[0].String() != "104.21.2.129" || ips[1].String() != "172.67.152.206" {
		t.Fatalf("DoH A 记录解析错误: %v", ips)
	}
	if ttl != 2*time.Minute {
		t.Fatalf("应使用最小有效 TTL，得到 %s", ttl)
	}
}

func TestDialBAIResolvedUsesDoHIP(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	accepted := make(chan struct{})
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr == nil {
			conn.Close()
			close(accepted)
		}
	}()

	dialer := &net.Dialer{Timeout: time.Second}
	conn, err := dialBAIResolved(
		context.Background(),
		"tcp",
		net.JoinHostPort(baiAPIHost, fmt.Sprint(port)),
		func(context.Context) ([]net.IP, error) { return []net.IP{net.ParseIP("127.0.0.1")}, nil },
		dialer.DialContext,
	)
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
	select {
	case <-accepted:
	case <-time.After(time.Second):
		t.Fatal("未连接到 DoH 返回的地址")
	}
}

func TestBAIProxyURLOverride(t *testing.T) {
	t.Setenv("BAI_PROXY_URL", "http://127.0.0.1:18080")
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("ALL_PROXY", "")
	req, err := http.NewRequest(http.MethodGet, "https://api.b.ai/v1/models", nil)
	if err != nil {
		t.Fatal(err)
	}
	proxyURL, err := baiProxy(req)
	if err != nil {
		t.Fatal(err)
	}
	if proxyURL == nil || proxyURL.String() != "http://127.0.0.1:18080" {
		t.Fatalf("BAI_PROXY_URL 未生效: %v", proxyURL)
	}
}

func TestBAILiveNetworkAdapter(t *testing.T) {
	key := os.Getenv("BAI_API_KEY")
	if key == "" {
		t.Skip("未配置 BAI_API_KEY，跳过真实 B.AI 网络适配测试")
	}
	b := RouterBackend{
		BaseURL: "https://api.b.ai/v1",
		Model:   "deepseek-v4-flash",
		APIKey:  key,
		Timeout: 45 * time.Second,
	}
	content, calls, err := openAIChatOnce(
		context.Background(),
		b,
		[]map[string]any{{"role": "user", "content": "只回复 BAI_DOH_OK"}},
		nil,
	)
	if err != nil {
		t.Fatalf("B.AI 网络适配真实请求失败: %v", err)
	}
	if content == "" && len(calls) == 0 {
		t.Fatal("B.AI 网络适配返回空响应")
	}
}
