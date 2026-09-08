package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// 测试局域网同步加密握手：POST /lan/memory/pull 带 AES-GCM 加密请求
// 能解密响应返回 {ct,iv} = 加密版；返回明文/400 = 未加密版
func main() {
	// 1. 拿 token
	c := &http.Client{Timeout: 5 * time.Second}
	r, err := c.Get("http://127.0.0.1:8080/api/lan/sync-info")
	if err != nil {
		fmt.Println("拿 sync-info 失败:", err)
		return
	}
	var info map[string]any
	json.NewDecoder(r.Body).Decode(&info)
	r.Body.Close()
	token, _ := info["token"].(string)
	fmt.Println("token:", token[:8]+"...")

	// 2. AES-GCM 加密请求（空对象 {}）
	key := sha256.Sum256([]byte(token))
	block, _ := aes.NewCipher(key[:16])
	gcm, _ := cipher.NewGCM(block)
	iv := make([]byte, gcm.NonceSize())
	rand.Read(iv)
	ct := gcm.Seal(nil, iv, []byte("{}"), nil)
	body, _ := json.Marshal(map[string]string{
		"token": token,
		"ct":    base64.StdEncoding.EncodeToString(ct),
		"iv":    base64.StdEncoding.EncodeToString(iv),
	})

	req, _ := http.NewRequest("POST", "http://127.0.0.1:18080/lan/memory/pull", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		fmt.Println("请求失败:", err)
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	fmt.Printf("HTTP %d\n", resp.StatusCode)
	s := string(respBody)
	if len(s) > 200 {
		s = s[:200]
	}
	fmt.Println("响应:", s)

	// 3. 判断：加密版响应含 ct/iv 字段
	if bytes.Contains(respBody, []byte(`"ct"`)) && resp.StatusCode == 200 {
		fmt.Println("✅ 加密版已实装：响应是 AES-GCM 密文 {ct,iv}")
	} else {
		fmt.Println("❌ 还是未加密版：响应不是 {ct,iv}")
	}
}
