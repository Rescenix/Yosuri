package handler

// 中途插话（steering）—— 用户在工作流跑着的时候发一条消息，不用等它完全停下
// 才能表达新想法。跟审批 registry（approval.go）是同一个模式：按 workflowID
// 索引一个全局 registry，独立的 POST /api/code/workflow/steer 端点通过它把
// 消息投进正在跑的 goroutine，四态机每轮非阻塞取一条拼进上下文。
//
// 不做跨断线持久化：steer 天然依附于一条活的 SSE 连接，断线后 channel 里没
// 消费掉的消息直接跟着 workflow 生命周期一起被回收——用户重连/续跑后重新
// 打字即可，不值得为这个小概率窗口给 workflowCheckpoint 加字段。

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

// steerQueueCap 允许用户在一轮处理期间多打几句排队；channel 的 FIFO 语义天然
// 实现"按发送顺序逐条投递、一轮取一条"，不需要额外的队列结构。
const steerQueueCap = 8

var (
	steerRegistryMu sync.Mutex
	steerRegistry   = make(map[string]chan string)
)

// registerSteerChannel 给一个工作流开一条 steer 消息通道，工作流启动时调用。
func registerSteerChannel(workflowID string) chan string {
	ch := make(chan string, steerQueueCap)
	steerRegistryMu.Lock()
	steerRegistry[workflowID] = ch
	steerRegistryMu.Unlock()
	return ch
}

// unregisterSteerChannel 工作流结束时清掉（defer 调用，跟 unregisterApprovalWaiter 同款）。
func unregisterSteerChannel(workflowID string) {
	steerRegistryMu.Lock()
	delete(steerRegistry, workflowID)
	steerRegistryMu.Unlock()
}

// sendSteerMessage 把一条消息投进对应工作流的通道。
// 返回 false = 工作流不存在（已结束/断线）或队列已满（用户打字远快于处理速度）。
func sendSteerMessage(workflowID, message string) bool {
	steerRegistryMu.Lock()
	ch, ok := steerRegistry[workflowID]
	steerRegistryMu.Unlock()
	if !ok {
		return false
	}
	select {
	case ch <- message:
		return true
	default:
		return false
	}
}

type steerRequest struct {
	WorkflowID string `json:"workflow_id"`
	Message    string `json:"message"`
}

// HandleCodeWorkflowSteer POST /api/code/workflow/steer
// 往正在跑的工作流里插一条消息，下一轮会被当作用户中途插话拼进上下文。
func HandleCodeWorkflowSteer(c *gin.Context) {
	var req steerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	if req.WorkflowID == "" || req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workflow_id 和 message 均必填"})
		return
	}
	if !sendSteerMessage(req.WorkflowID, req.Message) {
		c.JSON(http.StatusNotFound, gin.H{"error": "工作流不存在或已结束"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
