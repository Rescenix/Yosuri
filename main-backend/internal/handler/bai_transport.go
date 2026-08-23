package handler

// B.AI 在部分大陆网络下会遭遇普通 DNS 污染：api.b.ai 被解析到与 Cloudflare
// 无关且 443 不可达的地址，最终表现为 dial tcp ... i/o timeout。这里只对 B.AI
// 域名启用 DoH 解析，其他提供方继续使用系统 DNS，避免扩大网络行为变化范围。

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	baiAPIHost       = "api.b.ai"
	baiDoHURL        = "https://doh.pub/dns-query?name=api.b.ai&type=A"
	baiDNSCacheFloor = time.Minute
	baiDNSCacheCeil  = 10 * time.Minute
)

var baiLocalProxyPorts = []string{"7890", "7897", "10809"}

var baiDNSCache struct {
	sync.Mutex
	ips     []net.IP
	expires time.Time
}

var baiDoHClient = &http.Client{
	Timeout: 8 * time.Second,
	Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout: 5 * time.Second,
	},
}

func isBAIEndpoint(endpoint string) bool {
	u, err := url.Parse(strings.TrimSpace(endpoint))
	return err == nil && strings.EqualFold(u.Hostname(), baiAPIHost)
}

type baiDoHResponse struct {
	Answer []struct {
		Type int    `json:"type"`
		TTL  int    `json:"TTL"`
		Data string `json:"data"`
	} `json:"Answer"`
}

func parseBAIDoHResponse(raw []byte) ([]net.IP, time.Duration, error) {
	var payload baiDoHResponse
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, 0, fmt.Errorf("解析 B.AI DoH 响应失败: %w", err)
	}
	var ips []net.IP
	minTTL := baiDNSCacheCeil
	for _, answer := range payload.Answer {
		if answer.Type != 1 { // 只取 A 记录；当前 Cloudflare IPv4 已确认本机可达
			continue
		}
		ip := net.ParseIP(strings.TrimSpace(answer.Data))
		if ip == nil || ip.To4() == nil {
			continue
		}
		ips = append(ips, ip.To4())
		if answer.TTL > 0 {
			ttl := time.Duration(answer.TTL) * time.Second
			if ttl < minTTL {
				minTTL = ttl
			}
		}
	}
	if len(ips) == 0 {
		return nil, 0, fmt.Errorf("B.AI DoH 响应没有有效 A 记录")
	}
	if minTTL < baiDNSCacheFloor {
		minTTL = baiDNSCacheFloor
	}
	if minTTL > baiDNSCacheCeil {
		minTTL = baiDNSCacheCeil
	}
	return ips, minTTL, nil
}

func lookupBAIIPs(ctx context.Context) ([]net.IP, error) {
	now := time.Now()
	baiDNSCache.Lock()
	if len(baiDNSCache.ips) > 0 && now.Before(baiDNSCache.expires) {
		ips := append([]net.IP(nil), baiDNSCache.ips...)
		baiDNSCache.Unlock()
		return ips, nil
	}
	baiDNSCache.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baiDoHURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/dns-json")
	resp, err := baiDoHClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("B.AI DoH 查询失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("B.AI DoH 查询返回 HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return nil, fmt.Errorf("读取 B.AI DoH 响应失败: %w", err)
	}
	ips, ttl, err := parseBAIDoHResponse(raw)
	if err != nil {
		return nil, err
	}
	baiDNSCache.Lock()
	baiDNSCache.ips = append([]net.IP(nil), ips...)
	baiDNSCache.expires = time.Now().Add(ttl)
	baiDNSCache.Unlock()
	return ips, nil
}

type baiIPLookup func(context.Context) ([]net.IP, error)
type baiNetDial func(context.Context, string, string) (net.Conn, error)

func dialBAIResolved(ctx context.Context, network, address string, lookup baiIPLookup, dial baiNetDial) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil || !strings.EqualFold(host, baiAPIHost) {
		return dial(ctx, network, address)
	}
	ips, err := lookup(ctx)
	if err != nil {
		return nil, err
	}
	var failures []string
	for _, ip := range ips {
		conn, dialErr := dial(ctx, network, net.JoinHostPort(ip.String(), port))
		if dialErr == nil {
			return conn, nil
		}
		failures = append(failures, ip.String()+": "+dialErr.Error())
	}
	return nil, fmt.Errorf("B.AI DoH 地址均连接失败（%s）", strings.Join(failures, "; "))
}

func baiAwareDialContext(timeout time.Duration) func(context.Context, string, string) (net.Conn, error) {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	dialer := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		return dialBAIResolved(ctx, network, address, lookupBAIIPs, dialer.DialContext)
	}
}

// baiProxy 优先级：B.AI 专用环境变量 > 标准 HTTP(S)_PROXY > 本机 Clash 常用混合端口。
// Rescene 不修改系统代理；这里只在 B.AI 请求上复用已经运行的本地代理进程。
func baiProxy(req *http.Request) (*url.URL, error) {
	if raw := strings.TrimSpace(os.Getenv("BAI_PROXY_URL")); raw != "" {
		proxyURL, err := url.Parse(raw)
		if err != nil || proxyURL.Host == "" {
			return nil, fmt.Errorf("BAI_PROXY_URL 无效: %q", raw)
		}
		return proxyURL, nil
	}
	if proxyURL, err := http.ProxyFromEnvironment(req); err != nil || proxyURL != nil {
		return proxyURL, err
	}
	for _, port := range baiLocalProxyPorts {
		address := net.JoinHostPort("127.0.0.1", port)
		conn, err := net.DialTimeout("tcp", address, 150*time.Millisecond)
		if err != nil {
			continue
		}
		conn.Close()
		return url.Parse("http://" + address)
	}
	return nil, nil
}

func backendHTTPClient(b RouterBackend, timeout time.Duration, streaming bool) *http.Client {
	if !isBAIEndpoint(b.BaseURL) {
		if streaming {
			return streamHTTPClient()
		}
		return &http.Client{Timeout: timeout}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = baiProxy
	transport.DialContext = baiAwareDialContext(15 * time.Second)
	transport.TLSHandshakeTimeout = 15 * time.Second
	if streaming {
		transport.ResponseHeaderTimeout = 30 * time.Second
		transport.IdleConnTimeout = 90 * time.Second
		return &http.Client{Transport: transport}
	}
	return &http.Client{Timeout: timeout, Transport: transport}
}
