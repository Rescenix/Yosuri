// internal/api/kv_handler.go
package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// HandleKV 处理 /kv/{domain}/{key} 与 /kv/{domain} 的原始 KV 读写，
// 绕开 ENGRAM/REFRACT 的记忆图语义（簇归属、重要性衰减），
// 只是把一段 JSON 按 key 存进指定域自己的 bolt 文件。
// domain 由调用方在每次请求里显式指定，不依赖 DOMAIN USE 切换的全局状态，
// 因此天然不存在"USE 与后续操作非原子"的竞争问题。
func (h *PrimQLHandler) HandleKV(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/kv/")
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		http.Error(w, "domain required", http.StatusBadRequest)
		return
	}
	parts := strings.SplitN(path, "/", 2)
	domainName := parts[0]

	graph, err := h.manager.Domain(domainName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// /kv/{domain}：列出该域下所有 key（可选 ?prefix=）
	if len(parts) < 2 || parts[1] == "" {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		keys, err := graph.KVKeys(r.URL.Query().Get("prefix"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if keys == nil {
			keys = []string{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(keys)
		return
	}

	key := parts[1]
	switch r.Method {
	case http.MethodGet:
		value, ok, err := graph.KVGet(key)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(value)

	case http.MethodPut:
		body, err := io.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := graph.KVPut(key, body); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))

	case http.MethodDelete:
		if err := graph.KVDelete(key); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
