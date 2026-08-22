package handler

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// 悬浮球演示面板的数据源：只读地"旁听"四态机 SSE（writeCodeSSE 已经在发的
// thinking/intent/action/action_delta/result/workflow_start/workflow_done 等事件），
// 不改变主聊天流程任何行为——发布方（writeCodeSSE）没有订阅者时直接跳过，不产生
// 任何额外开销；有订阅者时也只是把已经序列化好的同一份字节多发一路，不重复 marshal。
type watchHub struct {
	mu   sync.Mutex
	subs map[chan []byte]struct{}
}

var globalWatchHub = &watchHub{subs: make(map[chan []byte]struct{})}

func (h *watchHub) subscribe() chan []byte {
	ch := make(chan []byte, 64)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *watchHub) unsubscribe(ch chan []byte) {
	h.mu.Lock()
	delete(h.subs, ch)
	h.mu.Unlock()
	close(ch)
}

func (h *watchHub) hasSubscribers() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs) > 0
}

// publish 把一帧已经格式化好的 SSE 数据（"event: x\ndata: y\n\n"）广播给所有订阅者。
// 订阅者处理不过来就丢这一条，绝不阻塞发布方——主聊天工作流的实时性不能被悬浮窗拖慢。
func (h *watchHub) publish(frame []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- frame:
		default:
		}
	}
}

// HandleAgentWatch GET /api/agent/watch —— 悬浮窗只读订阅四态机事件流（不影响主流程）。
func HandleAgentWatch(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	ch := globalWatchHub.subscribe()
	defer globalWatchHub.unsubscribe(ch)

	c.Writer.Write([]byte(": connected\n\n"))
	c.Writer.Flush()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case frame, ok := <-ch:
			if !ok {
				return
			}
			c.Writer.Write(frame)
			c.Writer.Flush()
		case <-ticker.C:
			if c.Request.Context().Err() != nil {
				return
			}
			c.Writer.Write([]byte(": heartbeat\n\n"))
			c.Writer.Flush()
		}
	}
}
