package handler

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// 未注册的 workflowID 投递应该失败——不能让消息凭空消失还被当成"成功"。
func TestSendSteerMessageUnknownWorkflow(t *testing.T) {
	if sendSteerMessage("wf_不存在", "hi") {
		t.Fatal("未注册的 workflow 不该投递成功")
	}
}

// 注册后应该能收到消息；工作流结束（unregister）后同一个 id 就找不到了。
func TestRegisterAndUnregisterSteerChannel(t *testing.T) {
	ch := registerSteerChannel("wf_test1")
	defer unregisterSteerChannel("wf_test1")

	if !sendSteerMessage("wf_test1", "等一下别改那个文件") {
		t.Fatal("已注册的 workflow 应该能收到 steer 消息")
	}
	select {
	case msg := <-ch:
		if msg != "等一下别改那个文件" {
			t.Errorf("收到的消息不对: %q", msg)
		}
	default:
		t.Fatal("channel 里应该有一条消息")
	}

	unregisterSteerChannel("wf_test1")
	if sendSteerMessage("wf_test1", "hi") {
		t.Fatal("unregister 之后不该还能投递成功")
	}
}

// 队列满了之后应该拒绝而不是阻塞——用户打字比处理速度快是正常场景，不该卡住整个请求。
func TestSendSteerMessageQueueFull(t *testing.T) {
	registerSteerChannel("wf_full")
	defer unregisterSteerChannel("wf_full")

	ok := 0
	for range steerQueueCap + 3 {
		if sendSteerMessage("wf_full", "msg") {
			ok++
		}
	}
	if ok != steerQueueCap {
		t.Fatalf("队列容量 %d，成功投递次数应等于容量，实得 %d", steerQueueCap, ok)
	}
}

func TestHandleCodeWorkflowSteerEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registerSteerChannel("wf_ep")
	defer unregisterSteerChannel("wf_ep")

	newCtx := func(body string) (*gin.Context, *httptest.ResponseRecorder) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/api/code/workflow/steer", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		return c, w
	}

	// 正常投递
	c, w := newCtx(`{"workflow_id":"wf_ep","message":"改用中文写"}`)
	HandleCodeWorkflowSteer(c)
	if w.Code != 200 {
		t.Fatalf("正常投递应 200，实得 %d: %s", w.Code, w.Body.String())
	}

	// 未知 workflow_id
	c, w = newCtx(`{"workflow_id":"wf_不存在","message":"hi"}`)
	HandleCodeWorkflowSteer(c)
	if w.Code != 404 {
		t.Fatalf("未知 workflow_id 应 404，实得 %d", w.Code)
	}

	// 缺字段
	c, w = newCtx(`{"workflow_id":"wf_ep"}`)
	HandleCodeWorkflowSteer(c)
	if w.Code != 400 {
		t.Fatalf("message 为空应 400，实得 %d", w.Code)
	}
}
