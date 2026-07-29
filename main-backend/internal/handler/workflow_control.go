package handler

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

type workflowControl struct {
	cancel  context.CancelFunc
	stopped atomic.Bool
	done    chan struct{}
	once    sync.Once
}

var (
	workflowControlMu sync.Mutex
	workflowControls  = map[string]*workflowControl{}
)

func registerWorkflowControl(parent context.Context, workflowID string) (context.Context, *workflowControl) {
	ctx, cancel := context.WithCancel(parent)
	control := &workflowControl{cancel: cancel, done: make(chan struct{})}
	workflowControlMu.Lock()
	workflowControls[workflowID] = control
	workflowControlMu.Unlock()
	return ctx, control
}

func unregisterWorkflowControl(workflowID string, control *workflowControl) {
	workflowControlMu.Lock()
	if workflowControls[workflowID] == control {
		delete(workflowControls, workflowID)
	}
	workflowControlMu.Unlock()
	control.cancel()
	control.once.Do(func() { close(control.done) })
}

func stopWorkflow(workflowID string) (*workflowControl, bool) {
	workflowControlMu.Lock()
	control := workflowControls[workflowID]
	workflowControlMu.Unlock()
	if control == nil {
		return nil, false
	}
	control.stopped.Store(true)
	control.cancel()
	return control, true
}

// HandleCodeWorkflowStop 让“停止”成为后端可辨认的生命周期事件。
// 旧前端只关闭 EventSource，后端只能把它当网络断线，也来不及可靠落盘部分上下文。
func (r *WorkflowRunner) HandleCodeWorkflowStop(c *gin.Context) {
	var body struct {
		WorkflowID string `json:"workflow_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.WorkflowID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workflow_id 必填"})
		return
	}
	control, ok := stopWorkflow(body.WorkflowID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "工作流不存在或已结束"})
		return
	}
	// 等主处理器的 defer 完成会话历史 upsert。短暂超时只是不阻塞停止按钮，
	// control 的取消信号仍已送达，主处理器随后会自行收尾。
	select {
	case <-control.done:
	case <-time.After(2 * time.Second):
	}
	c.JSON(http.StatusOK, gin.H{"status": "stopped", "workflow_id": body.WorkflowID})
}
