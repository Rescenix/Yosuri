package handler

import (
	"net/http"
	"os"
	"strings"

	"backend/internal/memorydir"

	"github.com/gin-gonic/gin"
)

// HandleMemoryInject GET /api/memory/inject
// 返回当前记忆状态，供前端如实展示。前两段是每次对话无条件注入的原文；
// 自动事实是索引命中后的按需检索内容，不能伪称为每轮都注入。
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
	autoFacts := memorydir.ReadRaw("facts")
	out := []injectSeg{
		{Key: "system", Title: "自定义指令（昵称 / 身份 / 指令）", Raw: userInstructionsPrompt(), Enabled: strings.TrimSpace(userInstructionsPrompt()) != ""},
		{Key: "memory", Title: "长期记忆（常驻 pinned + 记忆索引）", Raw: memoryRaw, Enabled: memoryRaw != ""},
		{Key: "memory", Title: "自动提取事实（按当前任务检索）", Raw: autoFacts, Enabled: autoFacts != ""},
	}
	c.JSON(http.StatusOK, gin.H{"segments": out})
}

// memoryFactItem 一条记忆事实（前端卡片展示用）。
type memoryFactItem struct {
	Category string `json:"category"`
	Key      string `json:"key"`
	Label    string `json:"label"`
	Value    string `json:"value"`
	Updated  string `json:"updated,omitempty"`
}

// HandleMemoryList GET /api/memory/list
// 返回结构化记忆列表（分组 + 单条信息），供前端卡片化展示。
// 只暴露自动提取事实（facts.json），不暴露手动维护的 preferences/projects 等文件。
// 返回 label（中文翻译）+ key（原始键）+ value + updated。
func HandleMemoryList(c *gin.Context) {
	automaticMemory.Lock()
	defer automaticMemory.Unlock()

	facts, err := loadAutomaticFacts()
	if err != nil || len(facts) == 0 {
		c.JSON(http.StatusOK, gin.H{"groups": []memoryFactItem{}})
		return
	}
	out := make([]memoryFactItem, 0, len(facts))
	for _, f := range facts {
		out = append(out, memoryFactItem{
			Category: f.Category,
			Key:      f.Key,
			Label:    memoryKeyLabel(f.Key),
			Value:    f.Value,
			Updated:  f.Updated.Format("01-02 15:04"),
		})
	}
	c.JSON(http.StatusOK, gin.H{"groups": out})
}

// HandleMemorySummary GET /api/memory/summary
// 返回派生记忆摘要（memory/summary.md）——LLM 整理的高密度人类可读视图。
// 只读不写；summary.md 不存在（首次运行尚未蒸馏）时返回 404，前端回退旧视图。
func HandleMemorySummary(c *gin.Context) {
	p := summaryPath()
	data, err := os.ReadFile(p)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "摘要尚未生成"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"summary": string(data)})
}

// memoryKeyLabel 英文 key → 中文可读标签。
func memoryKeyLabel(key string) string {
	switch strings.ToLower(key) {
	case "emoji_usage", "use_of_emoji":
		return "表情符号频率"
	case "language", "preferred_language":
		return "语言"
	case "message_length", "response_length", "preferred_message_length":
		return "回复长度"
	case "tone", "formality":
		return "语气风格"
	case "output_format", "preferred_output_format":
		return "输出格式"
	case "preferred_programming_language":
		return "编程语言偏好"
	case "duration_preference", "mock_backend_task_length":
		return "时长偏好"
	default:
		return key
	}
}
