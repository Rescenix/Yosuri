package main

// cdp_ws2.go — CDP WebSocket 通用调用（握手 + 发送 + 接收单响应）

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"
)

// wsCall 连接 WS → 发一条命令 → 读一条响应 → 关闭
// wsURL 形如 ws://127.0.0.1:9222/devtools/page/XXXX —— host 和 path 必须拆分
func wsCall(wsURL string, payload []byte) ([]byte, error) {
	u := strings.TrimPrefix(wsURL, "ws://")
	host := u
	path := "/"
	if i := strings.Index(u, "/"); i >= 0 {
		host = u[:i]
		path = u[i:]
	}
	conn, err := net.DialTimeout("tcp", host, 5*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// 握手（GET 用 WS 路径，Host 用 host）
	key := make([]byte, 16)
	rand.Read(key)
	secKey := base64.StdEncoding.EncodeToString(key)
	req := "GET " + path + " HTTP/1.1\r\n" +
		"Host: " + host + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + secKey + "\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		return nil, err
	}

	br := bufio.NewReader(conn)
	status, err := br.ReadString('\n')
	if err != nil || !strings.Contains(status, "101") {
		return nil, fmt.Errorf("WS 握手失败: %s", strings.TrimSpace(status))
	}
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return nil, err
		}
		if line == "\r\n" {
			break
		}
	}

	// 发送命令（mask）
	if err := wsSendFrame(conn, payload); err != nil {
		return nil, err
	}

	// 读响应（可能有多条事件，找含 "id" 的）
	for i := 0; i < 20; i++ {
		frame, err := wsReadFrame(br)
		if err != nil {
			return nil, err
		}
		var probe struct {
			ID int `json:"id"`
		}
		if json.Unmarshal(frame, &probe) == nil && probe.ID > 0 {
			return frame, nil
		}
	}
	return nil, fmt.Errorf("CDP 响应超时（事件流无命令响应）")
}

