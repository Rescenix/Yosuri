package handler

// worldbook_handler.go —— 世界书的 HTTP 接口。
//
//	GET    /api/worldbook?agent_id=   读（agent_id 空=全局书；非空=该卡私有书）
//	POST   /api/worldbook?agent_id=   整本覆盖写（body: WorldBook）
//	POST   /api/worldbook/entry       增改单条（body: {agent_id?, entry, uid?}）
//	DELETE /api/worldbook/entry       删单条（query: agent_id, uid）
//	POST   /api/worldbook/test        试触发（body: {agent_id?, text} -> 命中列表）
//
// 前端「世界书」面板走这套；模型侧的注入在 context_provider / RP 链路里自动做，
// 不需要前端每次传。

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// HandleWorldbookGet GET /api/worldbook
func HandleWorldbookGet(c *gin.Context) {
	path := agentLorePathOrGlobal(c.Query("agent_id"))
	lb := readLoreFile(path)
	if lb == nil {
		lb = &WorldBook{}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "scope": c.Query("agent_id"), "worldbook": lb})
}

// HandleWorldbookSave POST /api/worldbook —— 整本覆盖。
// 面板一次性提交用。条目数上限防手滑塞进几千条把上下文吃满。
func HandleWorldbookSave(c *gin.Context) {
	var body WorldBook
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体不是合法 JSON"})
		return
	}
	if len(body.Entries) > 2000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "条目过多（上限 2000）"})
		return
	}
	path := agentLorePathOrGlobal(c.Query("agent_id"))
	if err := writeLoreFile(path, &body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// HandleWorldbookUpsertEntry POST /api/worldbook/entry
// entry.uid 为 0 时自动分配（现有最大 uid+1），非 0 时原位更新。
func HandleWorldbookUpsertEntry(c *gin.Context) {
	var req struct {
		AgentID string    `json:"agent_id"`
		Entry   LoreEntry `json:"entry"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体不是合法 JSON"})
		return
	}
	if strings.TrimSpace(req.Entry.Content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "设定正文不能为空"})
		return
	}
	if len([]rune(req.Entry.Content)) > 4000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "单条设定过长（上限 4000 字）"})
		return
	}
	path := agentLorePathOrGlobal(req.AgentID)
	lb := readLoreFile(path)
	if lb == nil {
		lb = &WorldBook{}
	}
	maxUID := 0
	replaced := false
	for i := range lb.Entries {
		if lb.Entries[i].UID > maxUID {
			maxUID = lb.Entries[i].UID
		}
		if req.Entry.UID != 0 && lb.Entries[i].UID == req.Entry.UID {
			lb.Entries[i] = req.Entry
			replaced = true
		}
	}
	if !replaced {
		if req.Entry.UID == 0 {
			req.Entry.UID = maxUID + 1
		}
		if req.Entry.Order == 0 {
			req.Entry.Order = len(lb.Entries) + 1
		}
		lb.Entries = append(lb.Entries, req.Entry)
	}
	if err := writeLoreFile(path, lb); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "entry": req.Entry})
}

// HandleWorldbookDeleteEntry DELETE /api/worldbook/entry?agent_id=&uid=
func HandleWorldbookDeleteEntry(c *gin.Context) {
	uid, err := strconv.Atoi(c.Query("uid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uid 非法"})
		return
	}
	path := agentLorePathOrGlobal(c.Query("agent_id"))
	lb := readLoreFile(path)
	if lb == nil {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	out := lb.Entries[:0]
	for _, e := range lb.Entries {
		if e.UID == uid {
			continue
		}
		out = append(out, e)
	}
	lb.Entries = out
	if err := writeLoreFile(path, lb); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// HandleWorldbookTest POST /api/worldbook/test —— 试触发，面板「测一测」用。
// 让用户在写关键词时立刻看到「这句话会激活哪几条」，不用开一局去猜。
func HandleWorldbookTest(c *gin.Context) {
	var req struct {
		AgentID string `json:"agent_id"`
		Text    string `json:"text"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体不是合法 JSON"})
		return
	}
	lb := loadWorldBook(req.AgentID)
	hits := matchWorldBook(lb, req.Text)
	type hitView struct {
		UID     int      `json:"uid"`
		Name    string   `json:"name"`
		Matched []string `json:"matched"`
		Depth   int      `json:"depth"`
		Order   int      `json:"order"`
	}
	views := make([]hitView, 0, len(hits))
	for _, h := range hits {
		views = append(views, hitView{UID: h.Entry.UID, Name: h.Entry.Name, Matched: h.Matched, Depth: h.Entry.Depth, Order: h.Entry.Order})
	}
	sort.SliceStable(views, func(i, j int) bool { return views[i].Order < views[j].Order })
	c.JSON(http.StatusOK, gin.H{"ok": true, "hits": views, "total": len(views)})
}
