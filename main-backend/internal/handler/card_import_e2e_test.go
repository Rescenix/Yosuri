package handler

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"backend/internal/memorydir"

	"github.com/gin-gonic/gin"
)

// resetAgentCache 清掉 2s 内存缓存，让测试读到刚写的注册表。
func resetAgentCache() {
	agentRegistry.Lock()
	agentRegistry.cached = nil
	agentRegistry.cachedM = time.Time{}
	agentRegistry.Unlock()
}

// TestImportHandlerEndToEnd 走真实 HTTP handler：PNG 卡 → 角色卡落盘 + 头像 + 世界书。
func TestImportHandlerEndToEnd(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("RESCENE_DATA_DIR", dir)
	// memorydir.agentsRoot() 走 os.UserHomeDir()，不跟 RESCENE_DATA_DIR；
	// 不改 USERPROFILE 的话头像/世界书会写进真实 ~/rescene_data（测试污染）。
	t.Setenv("USERPROFILE", dir)
	resetAgentCache()
	defer resetAgentCache()

	cardJSON := `{
	  "spec": "chara_card_v2",
	  "data": {
	    "name": "终端测试卡",
	    "description": "这是一张用于端到端测试的卡。",
	    "personality": "沉稳。",
	    "scenario": "书房。",
	    "first_mes": "你来了。",
	    "character_book": {
	      "entries": [
	        {"keys": ["书房"], "content": "书房在东侧二楼。", "enabled": true},
	        {"keys": ["钥匙"], "content": "钥匙藏在书架第三层。", "enabled": true}
	      ]
	    }
	  }
	}`
	png := makeTextPNG("chara", base64.StdEncoding.EncodeToString([]byte(cardJSON)))
	body := fmt.Sprintf(`{"filename":"test.png","data":"data:image/png;base64,%s"}`,
		base64.StdEncoding.EncodeToString(png))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/cards/import", HandleImportCharacterCard)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/cards/import", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("导入失败 HTTP %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		OK              bool   `json:"ok"`
		Spec            string `json:"spec"`
		WorldbookEntries int   `json:"worldbook_entries"`
		Agent           struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Persona  string `json:"persona"`
			Greeting string `json:"greeting"`
			Avatar   string `json:"avatar"`
		} `json:"agent"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应不是 JSON: %v", err)
	}
	if !resp.OK || resp.Agent.Name != "终端测试卡" || resp.Spec != "v2" {
		t.Fatalf("响应字段不对: %+v", resp)
	}
	if resp.Agent.Greeting != "你来了。" {
		t.Fatalf("开场白没落库: %q", resp.Agent.Greeting)
	}
	if resp.WorldbookEntries != 2 {
		t.Fatalf("世界书应 2 条，实际 %d", resp.WorldbookEntries)
	}
	if !strings.Contains(resp.Agent.Persona, "这是一张用于端到端测试的卡") {
		t.Fatalf("人设没落库: %q", resp.Agent.Persona)
	}

	// 1) 注册表真的写了
	raw, err := os.ReadFile(filepath.Join(dir, "agents.json"))
	if err != nil {
		t.Fatalf("agents.json 没生成: %v", err)
	}
	if !strings.Contains(string(raw), "终端测试卡") {
		t.Fatalf("agents.json 里没有新卡: %s", raw)
	}

	// 2) 世界书写到角色私有文件（路径以 memorydir 为准，别自己拼）
	lorePath := agentLoreFilePath(resp.Agent.ID)
	if !strings.HasPrefix(lorePath, dir) {
		t.Fatalf("世界书路径跑到临时目录外了: %s", lorePath)
	}
	lb, err := os.ReadFile(lorePath)
	if err != nil {
		t.Fatalf("世界书文件没生成 %s: %v", lorePath, err)
	}
	if !strings.Contains(string(lb), "书房在东侧二楼") {
		t.Fatalf("世界书内容不对: %s", lb)
	}

	// 3) 头像（PNG 原图）落盘
	avatarPath := memorydir.AgentAvatarPath(resp.Agent.ID)
	if !strings.HasPrefix(avatarPath, dir) {
		t.Fatalf("头像路径跑到临时目录外了: %s", avatarPath)
	}
	av, err := os.ReadFile(avatarPath)
	if err != nil {
		entries, _ := os.ReadDir(filepath.Dir(avatarPath))
		names := []string{}
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("头像没落盘: %v（目录内容: %v）", err, names)
	}
	if !strings.Contains(string(av), "data:image/png;base64,") {
		t.Fatalf("头像不是 dataURL: %.40s", av)
	}

	// 4) 列表接口能读到（缓存清过）
	cards := readAgentCards()
	found := false
	for _, c := range cards {
		if c.ID == resp.Agent.ID && c.Greeting == "你来了。" {
			found = true
		}
	}
	if !found {
		t.Fatalf("readAgentCards 读不到新卡: %+v", cards)
	}
}

// 坏文件必须返回可读错误，且不落任何卡。
func TestImportHandlerRejectsPlainImage(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("RESCENE_DATA_DIR", dir)
	resetAgentCache()
	defer resetAgentCache()

	png := makeTextPNG("Comment", "just a comment chunk")
	body := fmt.Sprintf(`{"filename":"x.png","data":"%s"}`, base64.StdEncoding.EncodeToString(png))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/cards/import", HandleImportCharacterCard)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/cards/import", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("应 400，实际 %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "微信") {
		t.Fatalf("错误信息应提示转发丢数据: %s", w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "agents.json")); err == nil {
		if cards := readAgentCards(); len(cards) > 0 {
			t.Fatalf("失败却落了卡: %+v", cards)
		}
	}
}
