package handler

import (
	"bufio"
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

func TestIsHTTPProxyPort(t *testing.T) {
	// 本地 mock HTTP 代理：收到 CONNECT 就回 HTTP/1.1 200 Connection Established
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := fmt.Sprint(ln.Addr().(*net.TCPAddr).Port)
	go func() {
		for {
			conn, aerr := ln.Accept()
			if aerr != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				br := bufio.NewReader(c)
				if _, rerr := br.ReadString('\n'); rerr != nil {
					return
				}
				c.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
			}(conn)
		}
	}()
	if !isHTTPProxyPort(port) {
		t.Fatalf("mock HTTP 代理端口 %s 应被识别为代理", port)
	}
	// 普通服务（非代理）不应被识别：监听但不回 HTTP 响应
	ln2, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln2.Close()
	port2 := fmt.Sprint(ln2.Addr().(*net.TCPAddr).Port)
	go func() {
		conn, aerr := ln2.Accept()
		if aerr != nil {
			return
		}
		defer conn.Close()
		time.Sleep(3 * time.Second) // 收到 CONNECT 不回任何内容
	}()
	if isHTTPProxyPort(port2) {
		t.Fatalf("非代理端口 %s 不应被识别为代理", port2)
	}
}

func TestBAIProxyConfigURLPriority(t *testing.T) {
	// 配置 local_proxy_port 应优先于 BAI_PROXY_URL 环境变量
	t.Setenv("BAI_PROXY_URL", "http://127.0.0.1:18080")
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("ALL_PROXY", "")
	old := baiConfigProxyURL
	defer func() { baiConfigProxyURL = old }()
	baiConfigProxyURL = func() string { return "http://127.0.0.1:9910" }
	req, err := http.NewRequest(http.MethodGet, "https://api.b.ai/v1/models", nil)
	if err != nil {
		t.Fatal(err)
	}
	proxyURL, err := baiProxy(req)
	if err != nil {
		t.Fatal(err)
	}
	if proxyURL == nil || proxyURL.String() != "http://127.0.0.1:9910" {
		t.Fatalf("local_proxy_port 应优先于环境变量: %v", proxyURL)
	}
}

func TestBaiConfigProxyURLPortCompose(t *testing.T) {
	// 端口拼 URL 逻辑：有效端口拼 http://127.0.0.1:<port>，0/非法返回空
	old := baiConfigProxyPort
	defer func() { baiConfigProxyPort = old }()
	baiConfigProxyPort = func() int { return 9910 }
	if got := baiConfigProxyURL(); got != "http://127.0.0.1:9910" {
		t.Fatalf("端口 9910 应拼成 http://127.0.0.1:9910, got %q", got)
	}
	baiConfigProxyPort = func() int { return 0 }
	if got := baiConfigProxyURL(); got != "" {
		t.Fatalf("端口 0 应返回空(走自动探测), got %q", got)
	}
	baiConfigProxyPort = func() int { return 99999 }
	if got := baiConfigProxyURL(); got != "" {
		t.Fatalf("非法端口 99999 应返回空, got %q", got)
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
