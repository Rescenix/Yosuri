package handler

// studio_manga.go —— 创作工作台「漫剧」核心 API（2026-08-26）
//
//   POST /api/studio/manga/plan    剧本 → LLM 分镜 + 人设卡
//     请求: {topic, script, genre?}
//     响应: {ok, topic, genre, characters: [...], shots: [...]}
//   POST /api/studio/manga/shot    单镜头 → 指定平台生成素材
//     请求: {shot, platform, ref_image?}
//
// 漫剧流水线（分镜驱动，区别于 mambo 的文案成片）：
//   剧本 → LLM 拆分镜（画面+台词+时长） → 人设卡（角色一致性）
//   → 多平台调度（即梦/Kling/海螺）生成镜头素材 → 剪辑成片
//
// 复用 free pool 路由：resolveExact / resolveBackends 同 studio_semantic.go。

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// mangaCharacter 人设卡（角色一致性锚点）
type mangaCharacter struct {
	ID         string `json:"id"`         // 角色编号 C1/C2...
	Name       string `json:"name"`       // 角色名
	Role       string `json:"role"`       // 主角/反派/配角
	Appearance string `json:"appearance"` // 外貌特征（中文，供参考图提示词）
	Personality string `json:"personality"` // 性格
	RefPrompt string `json:"ref_prompt"` // 英文参考图提示词（含统一画风前缀）
}

// mangaShot 分镜（镜头）
type mangaShot struct {
	Index     int      `json:"index"`
	ShotNo    string   `json:"shot_no"`    // 镜头编号 S1 S2...
	Scene     string   `json:"scene"`      // 场景/背景描述
	Action    string   `json:"action"`     // 镜头动作描述（画面内容）
	Character string   `json:"character"`  // 出场角色（对应人设卡 id/name，可为空）
	Dialogue  string   `json:"dialogue"`   // 台词（可为空）
	Duration  int      `json:"duration"`   // 建议时长（秒）
	Platform  string   `json:"platform"`   // 推荐平台 auto/jimeng/kling/hailuo
	Prompt    string   `json:"prompt"`     // 英文生成提示词（图生视频/文生视频）
}

// mangaPlan 完整分镜计划
type mangaPlan struct {
	Topic      string          `json:"topic"`
	Genre      string          `json:"genre"`
	Characters []mangaCharacter `json:"characters"`
	Shots      []mangaShot     `json:"shots"`
	TotalDur   int             `json:"total_dur"`
}

// resolveMangaLLM 选择漫剧用 LLM（复用语义分析的路由逻辑）
func resolveMangaLLM() *RouterBackend {
	b := resolveExact("", "plan_step_gateway")
	if b == nil {
		b = resolveExact("", "free_step_3_7_flash")
	}
	if b == nil {
		backends := resolveBackends("", "")
		if len(backends) == 0 {
			return nil
		}
		bb := backends[0]
		return &bb
	}
	return b
}

// HandleMangaPlan POST /api/studio/manga/plan
func HandleMangaPlan(c *gin.Context) {
	var req struct {
		Topic  string `json:"topic"`
		Script string `json:"script"`
		Genre  string `json:"genre"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体解析失败: " + err.Error()})
		return
	}
	if strings.TrimSpace(req.Topic) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "topic 必填"})
		return
	}
	genre := req.Genre
	if genre == "" {
		genre = "都市奇幻"
	}

	b := resolveMangaLLM()
	if b == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "免费池不可用，无可用 LLM"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	plan, err := buildMangaPlan(ctx, *b, req.Topic, genre, req.Script)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "分镜生成失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "plan": plan})
}

// buildMangaPlan 调用 LLM 生成 人设卡 + 分镜。
// script 为空时 LLM 自行编剧情；非空则按给定剧情拆。
// 分两步调用：先人设卡（短输出），再分镜（按人设生成），避免单次输出被 max_tokens 截断。
func buildMangaPlan(ctx context.Context, b RouterBackend, topic, genre, script string) (*mangaPlan, error) {
	if strings.TrimSpace(script) == "" {
		script = "（由你根据主题创作完整剧情，包含起承转合，3-5 幕）"
	}

	plan := &mangaPlan{Topic: topic, Genre: genre}

	// ---- 第一步：人设卡 ----
	charPrompt := fmt.Sprintf(`你是动漫角色设计师。为 %s 题材 AI 漫剧《%s》设计 3-5 个角色。

剧情梗概：
%s

输出严格 JSON 数组，不要 markdown 代码块、不要解释：
[
  {"id":"C1","name":"角色名","role":"主角/反派/配角","appearance":"外貌特征(中文,发型/瞳色/服装/标志特征)","personality":"性格(一句话)","ref_prompt":"英文定妆参考图提示词"}
]

硬性要求：
- 必有主角；每个 ref_prompt 必须以同一画风前缀开头，如 "anime style, cinematic lighting, "
- appearance 要具体到可画：发型颜色长度、瞳色、服装款式颜色、独特标志
- 中文回复，除 ref_prompt 是英文`, genre, topic, script)
	ctx1, cancel1 := context.WithTimeout(ctx, 60*time.Second)
	defer cancel1()
	charOut, _, err := openAIChatOnce(ctx1, b, []map[string]any{{"role": "user", "content": charPrompt}}, nil)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(cleanJSONBlock(charOut), &plan.Characters); err != nil {
		return nil, fmt.Errorf("人设卡解析失败: %v\n输出: %.300s", err, charOut)
	}
	// 校验
	var chars []mangaCharacter
	for _, ch := range plan.Characters {
		if strings.TrimSpace(ch.Name) == "" {
			continue
		}
		if ch.ID == "" {
			ch.ID = fmt.Sprintf("C%d", len(chars)+1)
		}
		if ch.Role == "" {
			ch.Role = "配角"
		}
		chars = append(chars, ch)
	}
	plan.Characters = chars
	if len(chars) == 0 {
		return nil, fmt.Errorf("人设卡为空")
	}

	// ---- 第二步：分镜 ----
	charBrief := make([]string, 0, len(chars))
	for _, ch := range chars {
		charBrief = append(charBrief, fmt.Sprintf("%s %s(%s): %s", ch.ID, ch.Name, ch.Role, ch.Appearance))
	}
	shotPrompt := fmt.Sprintf(`你是动漫导演兼分镜师。为 %s 题材 AI 漫剧《%s》拆分镜。

剧情梗概：
%s

角色表（id 名称(定位): 外貌）：
%s

输出严格 JSON 数组，不要 markdown 代码块、不要解释：
[
  {"shot_no":"S1","scene":"场景/背景","action":"镜头画面动作","character":"角色名(无则空)","dialogue":"台词(无则空)","duration":5,"platform":"auto","prompt":"英文视频生成提示词"}
]

硬性要求：
- 8-15 个镜头覆盖完整故事，每镜头 duration 4-6 秒
- action 写清画面内容与镜头运动(推/拉/摇/固定)，给视频生成模型用
- prompt 是英文，含画风前缀、画面内容、镜头运动、光影氛围
- 台词短句为主，符合角色性格；旁白镜头 character 留空
- 中文回复，除 prompt 是英文`, genre, topic, script, strings.Join(charBrief, "\n"))
	ctx2, cancel2 := context.WithTimeout(ctx, 90*time.Second)
	defer cancel2()
	shotOut, _, err := openAIChatOnce(ctx2, b, []map[string]any{{"role": "user", "content": shotPrompt}}, nil)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(cleanJSONBlock(shotOut), &plan.Shots); err != nil {
		return nil, fmt.Errorf("分镜解析失败: %v\n输出: %.300s", err, shotOut)
	}
	// 校验
	var shots []mangaShot
	total := 0
	for i, s := range plan.Shots {
		if strings.TrimSpace(s.Scene) == "" && strings.TrimSpace(s.Action) == "" {
			continue
		}
		s.Index = i + 1
		if s.Duration <= 0 {
			s.Duration = 5
		}
		if s.Duration > 8 {
			s.Duration = 8
		}
		if s.Platform == "" {
			s.Platform = "auto"
		}
		total += s.Duration
		shots = append(shots, s)
	}
	plan.Shots = shots
	plan.TotalDur = total
	if len(shots) == 0 {
		return nil, fmt.Errorf("分镜为空")
	}
	return plan, nil
}

// cleanJSONBlock 去掉可能的 markdown 围栏，返回 JSON 片段
func cleanJSONBlock(raw string) []byte {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start >= 0 && end > start {
		return []byte(raw[start : end+1])
	}
	return []byte(raw)
}

// HandleMangaShot POST /api/studio/manga/shot
// 占位：单镜头生成（多平台调度后续接入真实平台 API）
func HandleMangaShot(c *gin.Context) {
	var req struct {
		ShotNo   string `json:"shot_no"`
		Prompt   string `json:"prompt"`
		Platform string `json:"platform"`
		RefImage string `json:"ref_image"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Platform == "" {
		req.Platform = "auto"
	}
	// 目前先回执：多平台调度在下一阶段接入（即梦/Kling/海螺 API）
	c.JSON(http.StatusOK, gin.H{
		"ok":       true,
		"shot_no":  req.ShotNo,
		"platform": req.Platform,
		"status":   "queued",
		"note":     "多平台生成调度尚未接入真实平台 API，此接口为流水线占位",
	})
}
