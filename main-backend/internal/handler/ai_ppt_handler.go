package handler

// ai_ppt_handler.go — AI PPT 生成（看得见的成果，不是文档）
//   POST /api/ai/ppt {topic} → LLM 生成 PPT 大纲 JSON {title, slides:[{title,points[]}]}
// 前端渲染成可翻页幻灯片（浏览器直接看/截图宣传）

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// HandleAIPPT POST /api/ai/ppt
func HandleAIPPT(c *gin.Context) {
	var req struct {
		Topic string `json:"topic" binding:"required"`
		Pages int    `json:"pages"` // 页数（默认 8，最多 12）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入主题"})
		return
	}
	topic := strings.TrimSpace(req.Topic)
	pages := req.Pages
	if pages < 4 || pages > 12 {
		pages = 8
	}

	prompt := `你是专业 PPT 设计师。围绕「` + topic + `」制作一份 PPT 大纲。

输出（严格 JSON，不要任何其他文字）：
{"title":"PPT 标题","slides":[{"title":"第1页标题","points":["要点1","要点2","要点3"]},{"title":"第2页标题","points":[...]},...]}

要求：` + strconv.Itoa(pages) + ` 页，每页 3-4 个要点，逻辑递进（引入-展开-案例-结论），标题抓眼球，像真正的商业/知识分享 PPT。`

	content, err := callLocalAggregate(prompt)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"error": "模型暂不可用，请稍后重试", "topic": topic})
		return
	}

	// 解析 JSON（可能被 ```json 包裹）
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var ppt struct {
		Title  string `json:"title"`
		Slides []struct {
			Title  string   `json:"title"`
			Points []string `json:"points"`
		} `json:"slides"`
	}
	if json.Unmarshal([]byte(content), &ppt) != nil || len(ppt.Slides) == 0 {
		// 解析失败：降级返回原始文本
		c.JSON(http.StatusOK, gin.H{"error": "PPT 生成格式异常，请重试", "raw": content})
		return
	}
	c.JSON(http.StatusOK, gin.H{"title": ppt.Title, "slides": ppt.Slides})
}
