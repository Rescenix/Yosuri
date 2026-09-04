package handler

// company_live.go —— 生产过程的实时直播总线。
//
// 交付引擎每完成一步、每产出一点模型文本，都往这里发一条事件；
// 前端「生产大屏」用 EventSource 订阅，实时看到报告文字往外长、原型页面一版版被换掉。
// 事件同时留在环形缓冲里，晚开页面的观众能立刻补上历史。

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// companyLiveEvent 一条直播事件。
type companyLiveEvent struct {
	Seq     int64  `json:"seq"`
	Time    string `json:"time"`
	Kind    string `json:"kind"` // stage 阶段开始 / delta 模型文本 / artifact 产物落盘 / iteration 原型换版 / done 交付完成 / error 失败
	Stage   string `json:"stage,omitempty"`
	Role    string `json:"role,omitempty"`
	Agent   string `json:"agent,omitempty"`
	Text    string `json:"text,omitempty"`
	Replaced bool  `json:"replaced,omitempty"` // delta 帧为 true 时：整块替换当前阶段文字（流式碎片→完整正文）
	File    string `json:"file,omitempty"`
	Project string `json:"project,omitempty"`
	Version string `json:"version,omitempty"` // 原型版本号：v1 最小原型 / v2 设计迭代 / final 终版
}

const companyLiveBufferMax = 900

type companyLiveHub struct {
	mu      sync.RWMutex
	seq     int64
	buf     []companyLiveEvent
	subs    map[chan companyLiveEvent]struct{}
	project string
}

var companyLive = &companyLiveHub{subs: map[chan companyLiveEvent]struct{}{}}

// companyLivePublish 发布一条事件：入环形缓冲 + 广播给所有在线观众。
func companyLivePublish(ev companyLiveEvent) {
	companyLive.mu.Lock()
	companyLive.seq++
	ev.Seq = companyLive.seq
	if ev.Time == "" {
		ev.Time = time.Now().Format("2006-01-02 15:04:05")
	}
	if ev.Project == "" {
		ev.Project = companyLive.project
	}
	companyLive.buf = append(companyLive.buf, ev)
	if len(companyLive.buf) > companyLiveBufferMax {
		companyLive.buf = companyLive.buf[len(companyLive.buf)-companyLiveBufferMax:]
	}
	subs := make([]chan companyLiveEvent, 0, len(companyLive.subs))
	for ch := range companyLive.subs {
		subs = append(subs, ch)
	}
	companyLive.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- ev:
		default: // 观众跟不上就丢帧，直播不阻塞生产
		}
	}
	companyLiveAppendLog(ev)
}

// companyLiveStage 阶段开始事件。
func companyLiveStage(project, stage, role, agent, title string) {
	companyLivePublish(companyLiveEvent{Kind: "stage", Stage: stage, Role: role, Agent: agent, Text: title, Project: project})
}

// companyLiveArtifact 产物落盘事件；原型类产物带 version 让大屏换页。
func companyLiveArtifact(project, stage, role, file, version string) {
	kind := "artifact"
	if version != "" {
		kind = "iteration"
	}
	companyLivePublish(companyLiveEvent{Kind: kind, Stage: stage, Role: role, File: file, Version: version, Project: project})
}

// companyLiveProject 当前正在生产的项目名（事件默认归属）。
func companyLiveProject() string {
	companyLive.mu.RLock()
	defer companyLive.mu.RUnlock()
	return companyLive.project
}

// companyLiveBegin 一条指令开始生产：清历史缓冲、记住项目名。
func companyLiveBegin(project string) {
	companyLive.mu.Lock()
	companyLive.buf = nil
	companyLive.seq = 0
	companyLive.project = project
	companyLive.mu.Unlock()
	companyLivePublish(companyLiveEvent{Kind: "stage", Stage: "kickoff", Text: "立项：会议 + 最小可运行原型", Project: project})
}

// companyLiveResetForProject 交付引擎入口调用：等价于 companyLiveBegin。
func companyLiveResetForProject(project string) { companyLiveBegin(project) }

// companyLiveSnapshot 当前缓冲事件（晚开页面补历史用）。
func companyLiveSnapshot() []companyLiveEvent {
	companyLive.mu.RLock()
	defer companyLive.mu.RUnlock()
	out := make([]companyLiveEvent, len(companyLive.buf))
	copy(out, companyLive.buf)
	return out
}

// HandleCompanyLive GET /api/company/live —— SSE 实时直播生产过程。
func HandleCompanyLive(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	ch := make(chan companyLiveEvent, 64)
	companyLive.mu.Lock()
	companyLive.subs[ch] = struct{}{}
	history := make([]companyLiveEvent, len(companyLive.buf))
	copy(history, companyLive.buf)
	companyLive.mu.Unlock()
	defer func() {
		companyLive.mu.Lock()
		delete(companyLive.subs, ch)
		companyLive.mu.Unlock()
	}()

	flusher, _ := c.Writer.(http.Flusher)
	send := func(ev companyLiveEvent) bool {
		data, err := json.Marshal(ev)
		if err != nil {
			return true
		}
		if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", data); err != nil {
			return false
		}
		if flusher != nil {
			flusher.Flush()
		}
		return true
	}
	for _, ev := range history {
		if !send(ev) {
			return
		}
	}
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	ctx := c.Request.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := fmt.Fprint(c.Writer, ": ping\n\n"); err != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		case ev := <-ch:
			if !send(ev) {
				return
			}
		}
	}
}

// companyLiveLogPath 直播事件落盘位置：项目真身目录内，随完整交付一起打包留档。
func companyLiveLogPath(project string) string {
	if project == "" {
		return filepath.Join(companyDir(), "live", "latest.ndjson")
	}
	return filepath.Join(companyProjectDir(project), "08-生产直播.ndjson")
}

// companyLiveAppendLog 每条事件同时写一行 ndjson，供事后回看与打包。
func companyLiveAppendLog(ev companyLiveEvent) {
	data, err := json.Marshal(ev)
	if err != nil {
		return
	}
	path := companyLiveLogPath(ev.Project)
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	if f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
		_, _ = f.Write(append(data, '\n'))
		_ = f.Close()
	}
}
