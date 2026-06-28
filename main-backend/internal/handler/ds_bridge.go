// ds_bridge.go
package handler

import (
	"bytes"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
)

// GetDSReply 调用 Node.js 脚本获取 DS 回复
func GetDSReply(message string) (string, error) {
	cmd := exec.Command("node", "ds_dom_observer.js", message)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("Playwright failed: %w, output: %s", err, out.String())
	}
	reply := strings.TrimSpace(out.String())
	return reply, nil
}

// DSHandler 供后端路由调用的处理函数
func DSHandler(w http.ResponseWriter, r *http.Request) {
	msg := r.URL.Query().Get("msg")
	reply, err := GetDSReply(msg)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	// 写入 PrismD
	_, err = http.Post("http://localhost:5666", "text/plain",
		strings.NewReader("ENGRAM ds_chat "+reply))
	if err != nil {
		http.Error(w, "PrismD write failed: "+err.Error(), 500)
		return
	}
	w.Write([]byte(reply))
}

func main() {
	http.HandleFunc("/api/ds", DSHandler)
	http.ListenAndServe(":8080", nil)
}
