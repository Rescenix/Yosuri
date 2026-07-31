package handler

import (
	"net/http"
	"strings"

	"backend/internal/memorydir"

	"github.com/gin-gonic/gin"
)

// HandleMemoryInject GET /api/memory/inject
// 返回「当前真正无条件注入每次对话系统提示词」的两段原文，供前端记忆面板
// 如实展示（展示 = 实际注入，零漂移）。两段与 context_provider.go 的装配严格一致：
//   1) 用户自定义指令段（userInstructionsPrompt，归 system 桶）
//   2) 长期记忆段（memorydir：常驻 pinned.md + 记忆索引 index.md，归 memory 桶）
// 任一段为空则不下发，前端只渲染收到的段。
func HandleMemoryInject(c *gin.Context) {
	type injectSeg struct {
		Key     string `json:"key"`     // 与 context provider 的分类桶一致：system / memory
		Title   string `json:"title"`   // 前端展示用的人话标题
		Raw     string `json:"raw"`     // 真正拼接进系统提示词的原文
		Enabled bool   `json:"enabled"` // 该段是否有内容
	}
	memoryRaw := ""
	if pinned := memorydir.ReadPinned(); pinned != "" {
		memoryRaw += "\n# 常驻记忆\n" + pinned + "\n"
	}
	if idx := memorydir.ReadIndex(); idx != "" {
		memoryRaw += "\n# 长期记忆索引\n" + idx
	}
	memoryRaw = strings.TrimSpace(memoryRaw)
	out := []injectSeg{
		{Key: "system", Title: "自定义指令（昵称 / 身份 / 指令）", Raw: userInstructionsPrompt(), Enabled: strings.TrimSpace(userInstructionsPrompt()) != ""},
		{Key: "memory", Title: "长期记忆（常驻 pinned + 记忆索引）", Raw: memoryRaw, Enabled: memoryRaw != ""},
	}
	c.JSON(http.StatusOK, gin.H{"segments": out})
}
