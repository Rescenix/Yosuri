package handler

// ── 超长任务暂存（2026-09-10，431 根治）──
// GET /api/code/workflow 的 task 参数拼在 URL query 里：中文 2000 字
// encodeURIComponent 后约 20KB，超过 dev server/网关的请求行上限 → 431，
// 工作流压根启动不了（「气泡都没出现 / 模型不回复」的根因之一）。
// 超长任务改 POST 暂存到本机内存，拿一个短 prepare_id，SSE 的 URL 只带 id，
// HandleCodeWorkflow 启动时取回并一次性删除。仅本机后端持有，15 分钟过期。

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	stashMaxTaskLen = 200000 // 单任务上限 20 万字
	stashTTL        = 15 * time.Minute
)

type stashedTask struct {
	task string
	exp  time.Time
}

var (
	taskStashMu sync.Mutex
	taskStash   = map[string]stashedTask{}
)

// HandleWorkflowStashTask POST /api/code/workflow/prepare
// body: {"task": "..."} → {"prepare_id": "..."}
func HandleWorkflowStashTask(c *gin.Context) {
	var req struct {
		Task string `json:"task"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Task) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "任务内容不能为空"})
		return
	}
	if len(req.Task) > stashMaxTaskLen {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "任务内容过长（上限 20 万字）"})
		return
	}
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "暂存失败，请重试"})
		return
	}
	pid := hex.EncodeToString(raw)
	taskStashMu.Lock()
	taskStash[pid] = stashedTask{task: req.Task, exp: time.Now().Add(stashTTL)}
	taskStashMu.Unlock()
	c.JSON(http.StatusOK, gin.H{"prepare_id": pid})
}

// takeStashedTask 取回暂存任务，一次性（取走即删）；不存在/过期返回 false。
func takeStashedTask(pid string) (string, bool) {
	taskStashMu.Lock()
	defer taskStashMu.Unlock()
	s, ok := taskStash[pid]
	if !ok || time.Now().After(s.exp) {
		delete(taskStash, pid)
		return "", false
	}
	delete(taskStash, pid)
	return s.task, true
}
