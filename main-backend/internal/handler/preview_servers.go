package handler

// 预览浏览器的本地开发服务器探测（仿 Claude Code 浏览器面板的服务器卡片）。
//
// GET /api/preview/servers 并发 TCP 探测一组候选本地端口（~400ms 超时），
// 返回存活的服务器卡片给前端预览面板的空标签页展示。
// 只做端口连通性检测，不发 HTTP 请求——快，且对任何协议的服务都有效。

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type previewServerCandidate struct {
	Name string `json:"name"`
	Port int    `json:"port"`
	URL  string `json:"url"`
}

// 候选清单按本项目的真实拓扑写死；将来要动态配置再抽到文件里（简单优先）
var previewCandidates = []previewServerCandidate{
	{Name: "beneficial-belt (Astro)", Port: 4322},
	{Name: "Astro 默认", Port: 4321},
	{Name: "Vite 默认", Port: 5173},
	{Name: "main-backend API", Port: 8080},
	{Name: "DS 浏览器代理", Port: 3000},
	{Name: "PrismD", Port: 5666},
	{Name: "Ollama", Port: 11434},
}

// HandlePreviewServers GET /api/preview/servers — 返回当前存活的本地服务
func HandlePreviewServers(c *gin.Context) {
	alive := make([]previewServerCandidate, 0, len(previewCandidates))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, cand := range previewCandidates {
		wg.Add(1)
		go func(cand previewServerCandidate) {
			defer wg.Done()
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", cand.Port), 400*time.Millisecond)
			if err != nil {
				return
			}
			conn.Close()
			cand.URL = fmt.Sprintf("http://localhost:%d", cand.Port)
			mu.Lock()
			alive = append(alive, cand)
			mu.Unlock()
		}(cand)
	}
	wg.Wait()

	// 稳定排序：按候选清单原始顺序（并发探测完成顺序是随机的）
	ordered := make([]previewServerCandidate, 0, len(alive))
	for _, cand := range previewCandidates {
		for _, a := range alive {
			if a.Port == cand.Port {
				ordered = append(ordered, a)
				break
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"servers": ordered})
}
