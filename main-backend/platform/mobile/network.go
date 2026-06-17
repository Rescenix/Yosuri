// backend/platform/mobile/network.go
package mobile

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"os"
	"time"
)

func NewDeepSeekTransport() http.RoundTripper {
	// 读取神权CA证书
	caCert, err := os.ReadFile("/data/data/com.termux/files/home/shanxi-ca.crt")
	if err != nil {
		return http.DefaultTransport
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			d := net.Dialer{Timeout: 30 * time.Second}
			return d.DialContext(ctx, "tcp", "127.0.0.1:8443")
		},
		TLSClientConfig: &tls.Config{
			RootCAs:    caCertPool,  // 信任我们的神权CA
			ServerName: "localhost", // 关键！检查证书的 CN 是否匹配
		},
	}
}
