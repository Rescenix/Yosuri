package main

// cdp_ws.go — 最小 WebSocket 客户端（RFC6455，仅 CDP 需要）
// 纯 stdlib：握手 + mask 帧发送 + 帧接收（支持 16/64 位长度）。

import (
	"bufio"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
)

// cdpGetAllCookies 通过 CDP WebSocket 取全部 cookie（复用 wsCall，host/path 已修复）
func cdpGetAllCookies(wsURL string) ([]cookieRec, error) {
	resp, err := wsCall(wsURL, []byte(`{"id":1,"method":"Network.getAllCookies"}`))
	if err != nil {
		return nil, err
	}
	var result struct {
		ID     int `json:"id"`
		Result struct {
			Cookies []cookieRec `json:"cookies"`
		} `json:"result"`
	}
	if json.Unmarshal(resp, &result) != nil {
		return nil, fmt.Errorf("CDP 响应解析失败")
	}
	return result.Result.Cookies, nil
}

// wsSendFrame 发送 client 帧（必须 mask）
func wsSendFrame(conn net.Conn, payload []byte) error {
	mask := make([]byte, 4)
	rand.Read(mask)
	header := []byte{0x81} // FIN + text
	l := len(payload)
	switch {
	case l < 126:
		header = append(header, byte(0x80|l))
	case l < 65536:
		header = append(header, 0x80|126)
		header = binary.BigEndian.AppendUint16(header, uint16(l))
	default:
		header = append(header, 0x80|127)
		header = binary.BigEndian.AppendUint64(header, uint64(l))
	}
	header = append(header, mask...)
	masked := make([]byte, l)
	for i := 0; i < l; i++ {
		masked[i] = payload[i] ^ mask[i%4]
	}
	_, err := conn.Write(append(header, masked...))
	return err
}

// wsReadFrame 读取一帧（server 帧不 mask，支持 126/127 长度）
func wsReadFrame(br *bufio.Reader) ([]byte, error) {
	first, err := br.ReadByte()
	if err != nil {
		return nil, err
	}
	opcode := first & 0x0f
	second, err := br.ReadByte()
	if err != nil {
		return nil, err
	}
	masked := second&0x80 != 0
	l := int(second & 0x7f)
	if l == 126 {
		var b [2]byte
		if _, err := io.ReadFull(br, b[:]); err != nil {
			return nil, err
		}
		l = int(binary.BigEndian.Uint16(b[:]))
	} else if l == 127 {
		var b [8]byte
		if _, err := io.ReadFull(br, b[:]); err != nil {
			return nil, err
		}
		l = int(binary.BigEndian.Uint64(b[:]))
	}
	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(br, mask[:]); err != nil {
			return nil, err
		}
	}
	payload := make([]byte, l)
	if _, err := io.ReadFull(br, payload); err != nil {
		return nil, err
	}
	if masked {
		for i := 0; i < l; i++ {
			payload[i] ^= mask[i%4]
		}
	}
	if opcode == 0x8 { // close
		return nil, fmt.Errorf("WebSocket 连接关闭")
	}
	return payload, nil
}
