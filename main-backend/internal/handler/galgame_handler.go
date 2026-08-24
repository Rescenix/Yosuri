package handler

// galgame_handler.go —— Galgame 模式：AI 生角色立绘 + 剧本式对话 + 选项分支。
//
// 三件事拼在一起：
//   1. 剧本由聚合免费模型池现写（每推进一幕才写下一幕，所以分支是真的分支，
//      不是预生成的假树——选项不同，后面整条线就不同）。
//   2. 立绘和背景走 image_generate.go 的免费无 key 生图；角色 seed 固定，
//      同一角色换表情仍是同一张脸。
//   3. 好感度跟着选项走，最后一幕按好感度收 good / normal / bad 结局。
//
// 存档落在 ~/rescene_data/galgame/<sessionId>/，session.json + 图片同目录。
// 回溯（rollback）把 scenes 截断到某一幕，就能回到那个选择点重走另一条分支。

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	galMaxScenes      = 10 // 一周目最多几幕，到了就收结局
	galPortraitWidth  = 768
	galPortraitHeight = 1152
	galBackgroundW    = 1344
	galBackgroundH    = 768
)

// galMu 串行化同一进程内对存档的读改写。剧本推进是分钟级的慢操作，
// 粒度粗一点没关系，比起并发写坏 session.json 划算。
var galMu sync.Mutex

type galCharacter struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Role         string            `json:"role"`         // 女主角 / 男主角 / 配角
	Persona      string            `json:"persona"`      // 性格与说话风格
	AppearanceEN string            `json:"appearanceEn"` // 立绘英文提示词
	Color        string            `json:"color"`        // 名字牌配色
	Affection    int               `json:"affection"`    // 好感度
	Seed         int64             `json:"seed"`         // 固定 seed，保证换表情还是同一张脸
	Portraits    map[string]string `json:"portraits"`    // emotion -> 图片 URL
}

type galLine struct {
	Speaker string `json:"speaker"` // 空 = 旁白
	Emotion string `json:"emotion"` // normal/smile/blush/sad/angry/surprised/serious
	Text    string `json:"text"`
}

type galChoice struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	Target    string `json:"target"`    // 这个选项影响谁的好感
	Affection int    `json:"affection"` // 好感增减
	Hint      string `json:"hint"`      // 悬浮提示，可空
}

type galScene struct {
	ID            string      `json:"id"`
	Index         int         `json:"index"`
	Title         string      `json:"title"`
	Place         string      `json:"place"`
	BackgroundEN  string      `json:"backgroundEn"`
	BackgroundURL string      `json:"backgroundUrl,omitempty"`
	Lines         []galLine   `json:"lines"`
	Choices       []galChoice `json:"choices"`
	ChosenID      string      `json:"chosenId,omitempty"` // 玩家在这一幕选了哪个
	Ending        string      `json:"ending,omitempty"`   // good / normal / bad
	EndingText    string      `json:"endingText,omitempty"`
}

type galSession struct {
	ID         string         `json:"id"`
	Title      string         `json:"title"`
	Synopsis   string         `json:"synopsis"`
	Premise    string         `json:"premise"`
	Style      string         `json:"style"`
	ArtStyleEN string         `json:"artStyleEn"`
	Characters []galCharacter `json:"characters"`
	Scenes     []galScene     `json:"scenes"`
	CreatedAt  string         `json:"createdAt"`
	UpdatedAt  string         `json:"updatedAt"`
}

func galgameOutputDir() string {
	if root := strings.TrimSpace(os.Getenv("RESCENE_GALGAME_DIR")); root != "" {
		return root
	}
	return filepath.Join(resceneUserDataDir(), "galgame")
}

func galSessionDir(id string) (string, error) {
	clean := sanitizeImageName(id)
	if clean == "" {
		return "", fmt.Errorf("存档 ID 无效")
	}
	return filepath.Join(galgameOutputDir(), clean), nil
}

func loadGalSession(id string) (*galSession, error) {
	dir, err := galSessionDir(id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "session.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("存档不存在: %s", id)
		}
		return nil, err
	}
	var s galSession
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("存档已损坏: %w", err)
	}
	return &s, nil
}

func saveGalSession(s *galSession) error {
	dir, err := galSessionDir(s.ID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	s.UpdatedAt = time.Now().Format(time.RFC3339)
	data, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(filepath.Join(dir, "session.json"), data, 0o600)
}

// callGalgameModel 走本地聚合免费模型池写剧本。和 callLocalAggregate 同一个端点，
// 但换成编剧人设、放宽 max_tokens——一幕剧本比一段文案长得多。
func callGalgameModel(ctx context.Context, system, user string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"model": "auto",
		"messages": []map[string]any{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"max_tokens":  4096,
		"temperature": 0.9,
		"stream":      false,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://127.0.0.1:8080/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+aggregateAPIKey())
	client := &http.Client{Timeout: 3 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("聚合 API HTTP %d", resp.StatusCode)
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("模型返回空响应")
	}
	return out.Choices[0].Message.Content, nil
}

// galExtractJSON 从模型输出里抠出 JSON 对象。免费模型爱在 JSON 前后加寒暄，
// cleanJSON 只处理代码块围栏，这里再按第一个 { 到最后一个 } 兜一层。
func galExtractJSON(raw string) string {
	s := cleanJSON(raw)
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}

const galSystemPrompt = `你是资深 galgame 编剧，写日式视觉小说剧本。
硬性要求：
- 只输出 JSON，不要任何解释文字、不要 Markdown 代码块。
- 对白口语化、有性格，一句一行，单句不超过 60 字。
- 旁白 speaker 留空字符串。
- emotion 只能是 normal / smile / blush / sad / angry / surprised / serious 之一。
- 英文提示词字段（appearanceEn / backgroundEn）必须是纯英文，描述画面，不要出现文字或字幕。`

// galCharacterSeed 由角色名派生固定 seed：同一个角色不同表情仍是同一张脸。
func galCharacterSeed(name string) int64 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return int64(h.Sum32()%(1<<30)) + 1
}

func galSlug(prefix string, i int) string {
	return fmt.Sprintf("%s-%d", prefix, i+1)
}

// HandleGalgameNew POST /api/galgame/new —— 开新档：立人设 + 写第一幕。
func HandleGalgameNew(c *gin.Context) {
	var req struct {
		Premise   string `json:"premise"`
		Style     string `json:"style"`
		Tone      string `json:"tone"`
		Heroines  int    `json:"heroines"`
		PlayerYou string `json:"playerName"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	premise := strings.TrimSpace(req.Premise)
	if premise == "" {
		premise = "转学第一天，我在天台遇到了一个正在逃课的女生。"
	}
	style := strings.TrimSpace(req.Style)
	if style == "" {
		style = "日常校园恋爱，轻松略带青春感伤"
	}
	if req.Heroines < 1 || req.Heroines > 4 {
		req.Heroines = 2
	}
	player := strings.TrimSpace(req.PlayerYou)
	if player == "" {
		player = "主角"
	}

	prompt := fmt.Sprintf(`为一部 galgame 立项，并写出第一幕。

故事前提：%s
题材风格：%s
情绪基调：%s
可攻略角色数量：%d
玩家扮演：%s（第二人称视角，玩家不出现在立绘里）

输出 JSON：
{
  "title": "作品标题",
  "synopsis": "一句话简介",
  "artStyleEn": "整部作品统一的英文画风提示词，如 anime visual novel art, soft pastel lighting, detailed",
  "characters": [
    {
      "name": "角色名",
      "role": "女主角/男主角/配角",
      "persona": "性格、说话方式、与主角的关系（40字内）",
      "appearanceEn": "纯英文外貌提示词：发型发色、瞳色、服装、身形、气质",
      "color": "#RRGGBB 名字牌配色"
    }
  ],
  "scene": {
    "title": "这一幕的小标题",
    "place": "场景与时间，如 学校天台·黄昏",
    "backgroundEn": "纯英文背景提示词，画面里不要有人",
    "lines": [
      {"speaker": "", "emotion": "normal", "text": "旁白"},
      {"speaker": "角色名", "emotion": "smile", "text": "台词"}
    ],
    "choices": [
      {"text": "玩家可选的行动或台词", "target": "受影响的角色名", "affection": 2, "hint": "可空"}
    ]
  }
}

要求：第一幕 8-14 行对白，至少两个角色露面；choices 给 3 个，各自倾向不同角色或不同态度，affection 取 -2 到 3。`,
		premise, style, strings.TrimSpace(req.Tone), req.Heroines, player)

	raw, err := callGalgameModel(c.Request.Context(), galSystemPrompt, prompt)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "剧本生成失败: " + err.Error()})
		return
	}
	var parsed struct {
		Title      string `json:"title"`
		Synopsis   string `json:"synopsis"`
		ArtStyleEN string `json:"artStyleEn"`
		Characters []struct {
			Name         string `json:"name"`
			Role         string `json:"role"`
			Persona      string `json:"persona"`
			AppearanceEN string `json:"appearanceEn"`
			Color        string `json:"color"`
		} `json:"characters"`
		Scene struct {
			Title        string      `json:"title"`
			Place        string      `json:"place"`
			BackgroundEN string      `json:"backgroundEn"`
			Lines        []galLine   `json:"lines"`
			Choices      []galChoice `json:"choices"`
		} `json:"scene"`
	}
	if err := json.Unmarshal([]byte(galExtractJSON(raw)), &parsed); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "剧本解析失败: " + err.Error(), "raw": truncateChars(raw, 800)})
		return
	}
	if len(parsed.Characters) == 0 || len(parsed.Scene.Lines) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "模型没写出角色或对白，请重试", "raw": truncateChars(raw, 800)})
		return
	}

	session := &galSession{
		ID:         fmt.Sprintf("gal-%d", time.Now().UnixMilli()),
		Title:      strings.TrimSpace(parsed.Title),
		Synopsis:   strings.TrimSpace(parsed.Synopsis),
		Premise:    premise,
		Style:      style,
		ArtStyleEN: strings.TrimSpace(parsed.ArtStyleEN),
		CreatedAt:  time.Now().Format(time.RFC3339),
	}
	if session.Title == "" {
		session.Title = "无题"
	}
	if session.ArtStyleEN == "" {
		session.ArtStyleEN = "anime visual novel art, soft lighting, clean lineart, detailed"
	}
	for i, ch := range parsed.Characters {
		name := strings.TrimSpace(ch.Name)
		if name == "" {
			continue
		}
		session.Characters = append(session.Characters, galCharacter{
			ID:           galSlug("char", i),
			Name:         name,
			Role:         strings.TrimSpace(ch.Role),
			Persona:      strings.TrimSpace(ch.Persona),
			AppearanceEN: strings.TrimSpace(ch.AppearanceEN),
			Color:        strings.TrimSpace(ch.Color),
			Seed:         galCharacterSeed(name),
			Portraits:    map[string]string{},
		})
	}
	scene := galScene{
		ID:           galSlug("scene", 0),
		Index:        0,
		Title:        strings.TrimSpace(parsed.Scene.Title),
		Place:        strings.TrimSpace(parsed.Scene.Place),
		BackgroundEN: strings.TrimSpace(parsed.Scene.BackgroundEN),
		Lines:        parsed.Scene.Lines,
		Choices:      normalizeGalChoices(parsed.Scene.Choices),
	}
	session.Scenes = []galScene{scene}

	galMu.Lock()
	err = saveGalSession(session)
	galMu.Unlock()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "存档写入失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, session)
}

func normalizeGalChoices(choices []galChoice) []galChoice {
	out := make([]galChoice, 0, len(choices))
	for i, ch := range choices {
		ch.Text = strings.TrimSpace(ch.Text)
		if ch.Text == "" {
			continue
		}
		ch.ID = galSlug("choice", i)
		if ch.Affection < -5 {
			ch.Affection = -5
		}
		if ch.Affection > 5 {
			ch.Affection = 5
		}
		out = append(out, ch)
	}
	return out
}

// HandleGalgameAdvance POST /api/galgame/advance —— 选一个选项，写出下一幕。
func HandleGalgameAdvance(c *gin.Context) {
	var req struct {
		SessionID string `json:"sessionId" binding:"required"`
		ChoiceID  string `json:"choiceId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	galMu.Lock()
	session, err := loadGalSession(req.SessionID)
	galMu.Unlock()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if len(session.Scenes) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "存档里没有任何剧情"})
		return
	}
	current := &session.Scenes[len(session.Scenes)-1]
	if current.Ending != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "本周目已经完结，请回溯到某一幕重选或开新档"})
		return
	}
	var chosen *galChoice
	for i := range current.Choices {
		if current.Choices[i].ID == req.ChoiceID {
			chosen = &current.Choices[i]
			break
		}
	}
	if chosen == nil && len(current.Choices) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "选项不存在: " + req.ChoiceID})
		return
	}

	// 好感度按选项结算。target 写的是角色名，对不上就忽略——
	// 免费模型偶尔会写个不存在的名字，不该因此中断游戏。
	if chosen != nil {
		current.ChosenID = chosen.ID
		for i := range session.Characters {
			if session.Characters[i].Name == strings.TrimSpace(chosen.Target) {
				session.Characters[i].Affection += chosen.Affection
				break
			}
		}
	}

	nextIndex := len(session.Scenes)
	mustEnd := nextIndex >= galMaxScenes-1
	raw, err := callGalgameModel(c.Request.Context(), galSystemPrompt, galAdvancePrompt(session, chosen, nextIndex, mustEnd))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "剧本生成失败: " + err.Error()})
		return
	}
	var parsed struct {
		Title        string      `json:"title"`
		Place        string      `json:"place"`
		BackgroundEN string      `json:"backgroundEn"`
		Lines        []galLine   `json:"lines"`
		Choices      []galChoice `json:"choices"`
		Ending       string      `json:"ending"`
		EndingText   string      `json:"endingText"`
	}
	if err := json.Unmarshal([]byte(galExtractJSON(raw)), &parsed); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "剧本解析失败: " + err.Error(), "raw": truncateChars(raw, 800)})
		return
	}
	if len(parsed.Lines) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "模型没写出对白，请重试", "raw": truncateChars(raw, 800)})
		return
	}
	ending := strings.ToLower(strings.TrimSpace(parsed.Ending))
	if ending != "good" && ending != "normal" && ending != "bad" {
		ending = ""
	}
	if mustEnd && ending == "" {
		ending = galEndingByAffection(session)
	}
	next := galScene{
		ID:           galSlug("scene", nextIndex),
		Index:        nextIndex,
		Title:        strings.TrimSpace(parsed.Title),
		Place:        strings.TrimSpace(parsed.Place),
		BackgroundEN: strings.TrimSpace(parsed.BackgroundEN),
		Lines:        parsed.Lines,
		Ending:       ending,
		EndingText:   strings.TrimSpace(parsed.EndingText),
	}
	if ending == "" {
		next.Choices = normalizeGalChoices(parsed.Choices)
	}
	if next.BackgroundEN == "" {
		next.BackgroundEN = current.BackgroundEN
	}
	session.Scenes = append(session.Scenes, next)

	galMu.Lock()
	err = saveGalSession(session)
	galMu.Unlock()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "存档写入失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, session)
}

// galAdvancePrompt 把已走过的剧情、好感度和刚做的选择压成上下文。
// 只带每一幕的梗概和最近一幕的完整对白——全量塞进去会撑爆免费模型的上下文。
func galAdvancePrompt(s *galSession, chosen *galChoice, nextIndex int, mustEnd bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "作品：%s\n简介：%s\n题材：%s\n\n登场角色：\n", s.Title, s.Synopsis, s.Style)
	for _, ch := range s.Characters {
		fmt.Fprintf(&b, "- %s（%s）好感度 %d：%s\n", ch.Name, ch.Role, ch.Affection, ch.Persona)
	}
	b.WriteString("\n已发生的剧情：\n")
	for i, scene := range s.Scenes {
		fmt.Fprintf(&b, "第%d幕《%s》@%s", i+1, scene.Title, scene.Place)
		if i == len(s.Scenes)-1 {
			b.WriteString("\n")
			for _, line := range scene.Lines {
				speaker := line.Speaker
				if speaker == "" {
					speaker = "旁白"
				}
				fmt.Fprintf(&b, "  %s：%s\n", speaker, line.Text)
			}
		} else {
			if len(scene.Lines) > 0 {
				fmt.Fprintf(&b, "：%s……\n", truncateChars(scene.Lines[0].Text, 40))
			} else {
				b.WriteString("\n")
			}
		}
	}
	if chosen != nil {
		fmt.Fprintf(&b, "\n玩家刚刚选择了：「%s」\n请让这个选择明确改变后续走向，而不是走回同一条路。\n", chosen.Text)
	}
	fmt.Fprintf(&b, "\n现在写第%d幕。输出 JSON：\n", nextIndex+1)
	b.WriteString(`{
  "title": "这一幕的小标题",
  "place": "场景与时间",
  "backgroundEn": "纯英文背景提示词，画面里不要有人",
  "lines": [{"speaker": "角色名或空字符串", "emotion": "smile", "text": "台词"}],
  "choices": [{"text": "玩家的行动或台词", "target": "受影响的角色名", "affection": 2, "hint": "可空"}],
  "ending": "",
  "endingText": ""
}`)
	b.WriteString("\n\n要求：8-14 行对白，承接上一幕并体现玩家的选择带来的后果。")
	if mustEnd {
		fmt.Fprintf(&b, "\n这是最后一幕：写完结，choices 给空数组，ending 按好感度填 good / normal / bad（当前最高好感是 %s），endingText 写 2-3 句收束。", galTopCharacter(s))
	} else {
		b.WriteString("\nchoices 给 2-3 个，彼此后果明显不同，affection 取 -2 到 3。ending 和 endingText 都留空字符串。")
	}
	return b.String()
}

func galTopCharacter(s *galSession) string {
	best := ""
	high := -1 << 30
	for _, ch := range s.Characters {
		if ch.Affection > high {
			high, best = ch.Affection, fmt.Sprintf("%s（%d）", ch.Name, ch.Affection)
		}
	}
	if best == "" {
		return "无"
	}
	return best
}

func galEndingByAffection(s *galSession) string {
	high := -1 << 30
	for _, ch := range s.Characters {
		if ch.Affection > high {
			high = ch.Affection
		}
	}
	switch {
	case high >= 8:
		return "good"
	case high >= 3:
		return "normal"
	default:
		return "bad"
	}
}

// HandleGalgameRollback POST /api/galgame/rollback —— 回到某一幕重选，走另一条分支。
func HandleGalgameRollback(c *gin.Context) {
	var req struct {
		SessionID  string `json:"sessionId" binding:"required"`
		SceneIndex int    `json:"sceneIndex"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	galMu.Lock()
	defer galMu.Unlock()
	session, err := loadGalSession(req.SessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if req.SceneIndex < 0 || req.SceneIndex >= len(session.Scenes) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "幕号超出范围"})
		return
	}
	// 好感度是选项累加出来的，回溯必须把被截掉那些幕的加成退回去，
	// 否则反复回溯重选会把好感度刷上天。
	for i := req.SceneIndex; i < len(session.Scenes); i++ {
		scene := session.Scenes[i]
		if scene.ChosenID == "" {
			continue
		}
		for _, choice := range scene.Choices {
			if choice.ID != scene.ChosenID {
				continue
			}
			for j := range session.Characters {
				if session.Characters[j].Name == strings.TrimSpace(choice.Target) {
					session.Characters[j].Affection -= choice.Affection
					break
				}
			}
		}
	}
	session.Scenes = session.Scenes[:req.SceneIndex+1]
	session.Scenes[req.SceneIndex].ChosenID = ""
	if err := saveGalSession(session); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "存档写入失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, session)
}

// HandleGalgamePortrait POST /api/galgame/portrait —— 生成/复用角色某个表情的立绘。
func HandleGalgamePortrait(c *gin.Context) {
	var req struct {
		SessionID   string `json:"sessionId" binding:"required"`
		CharacterID string `json:"characterId"`
		Name        string `json:"name"`
		Emotion     string `json:"emotion"`
		Force       bool   `json:"force"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	galMu.Lock()
	session, err := loadGalSession(req.SessionID)
	galMu.Unlock()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	emotion := normalizeGalEmotion(req.Emotion)
	idx := -1
	for i, ch := range session.Characters {
		if ch.ID == req.CharacterID || (req.Name != "" && ch.Name == strings.TrimSpace(req.Name)) {
			idx = i
			break
		}
	}
	if idx < 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "角色不存在"})
		return
	}
	character := session.Characters[idx]
	if !req.Force && character.Portraits != nil {
		if url, ok := character.Portraits[emotion]; ok && url != "" {
			if _, statErr := os.Stat(galAssetPath(session.ID, url)); statErr == nil {
				c.JSON(http.StatusOK, gin.H{"imageUrl": url, "cached": true, "emotion": emotion, "characterId": character.ID})
				return
			}
		}
	}

	dir, err := galSessionDir(session.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	prompt := strings.Join(filterEmpty([]string{
		"anime visual novel character sprite of " + character.AppearanceEN,
		galEmotionPrompt(emotion),
		"upper body, facing viewer, standing pose, centered composition",
		"plain flat pastel background, soft studio lighting",
		session.ArtStyleEN,
		"masterpiece, best quality, highly detailed",
	}), ", ")
	result, err := generateImage(c.Request.Context(), imageGenSpec{
		Prompt:   prompt,
		Negative: "text, watermark, signature, multiple people, extra limbs, bad hands, nsfw, blurry, cropped head",
		Width:    galPortraitWidth,
		Height:   galPortraitHeight,
		Seed:     character.Seed,
		OutDir:   dir,
		Name:     fmt.Sprintf("%s-%s", character.ID, emotion),
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "立绘生成失败: " + err.Error()})
		return
	}
	url := galAssetURL(session.ID, result.File)

	galMu.Lock()
	defer galMu.Unlock()
	// 出图这几十秒里存档可能已经推进过，重新读一遍再写回，别把新剧情覆盖掉。
	if fresh, freshErr := loadGalSession(session.ID); freshErr == nil {
		session = fresh
	}
	for i := range session.Characters {
		if session.Characters[i].ID != character.ID {
			continue
		}
		if session.Characters[i].Portraits == nil {
			session.Characters[i].Portraits = map[string]string{}
		}
		session.Characters[i].Portraits[emotion] = url
	}
	if err := saveGalSession(session); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "存档写入失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"imageUrl": url, "cached": false, "emotion": emotion, "characterId": character.ID, "provider": result.Provider})
}

// HandleGalgameBackground POST /api/galgame/background —— 生成/复用某一幕的背景。
func HandleGalgameBackground(c *gin.Context) {
	var req struct {
		SessionID string `json:"sessionId" binding:"required"`
		SceneID   string `json:"sceneId"`
		Force     bool   `json:"force"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	galMu.Lock()
	session, err := loadGalSession(req.SessionID)
	galMu.Unlock()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	idx := -1
	for i, scene := range session.Scenes {
		if scene.ID == req.SceneID {
			idx = i
			break
		}
	}
	if idx < 0 {
		if len(session.Scenes) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "这一幕不存在"})
			return
		}
		idx = len(session.Scenes) - 1
	}
	scene := session.Scenes[idx]
	if !req.Force && scene.BackgroundURL != "" {
		if _, statErr := os.Stat(galAssetPath(session.ID, scene.BackgroundURL)); statErr == nil {
			c.JSON(http.StatusOK, gin.H{"imageUrl": scene.BackgroundURL, "cached": true, "sceneId": scene.ID})
			return
		}
	}
	dir, err := galSessionDir(session.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	backdrop := scene.BackgroundEN
	if strings.TrimSpace(backdrop) == "" {
		backdrop = scene.Place
	}
	prompt := strings.Join(filterEmpty([]string{
		"anime visual novel background art of " + backdrop,
		"no people, no characters, no text",
		"wide establishing shot, detailed scenery, atmospheric lighting",
		session.ArtStyleEN,
		"masterpiece, best quality",
	}), ", ")
	result, err := generateImage(c.Request.Context(), imageGenSpec{
		Prompt:   prompt,
		Negative: "people, person, character, face, text, watermark, signature, blurry",
		Width:    galBackgroundW,
		Height:   galBackgroundH,
		OutDir:   dir,
		Name:     "bg-" + scene.ID,
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "背景生成失败: " + err.Error()})
		return
	}
	url := galAssetURL(session.ID, result.File)

	galMu.Lock()
	defer galMu.Unlock()
	if fresh, freshErr := loadGalSession(session.ID); freshErr == nil {
		session = fresh
	}
	for i := range session.Scenes {
		if session.Scenes[i].ID == scene.ID {
			session.Scenes[i].BackgroundURL = url
		}
	}
	if err := saveGalSession(session); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "存档写入失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"imageUrl": url, "cached": false, "sceneId": scene.ID, "provider": result.Provider})
}

// HandleGalgameSessions GET /api/galgame/sessions —— 存档列表，最近的在前。
func HandleGalgameSessions(c *gin.Context) {
	root := galgameOutputDir()
	entries, err := os.ReadDir(root)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"sessions": []any{}})
		return
	}
	type brief struct {
		ID        string `json:"id"`
		Title     string `json:"title"`
		Synopsis  string `json:"synopsis"`
		Scenes    int    `json:"scenes"`
		Ending    string `json:"ending"`
		UpdatedAt string `json:"updatedAt"`
	}
	sessions := make([]brief, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		s, err := loadGalSession(entry.Name())
		if err != nil {
			continue
		}
		ending := ""
		if len(s.Scenes) > 0 {
			ending = s.Scenes[len(s.Scenes)-1].Ending
		}
		sessions = append(sessions, brief{ID: s.ID, Title: s.Title, Synopsis: s.Synopsis, Scenes: len(s.Scenes), Ending: ending, UpdatedAt: s.UpdatedAt})
	}
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].UpdatedAt > sessions[j].UpdatedAt })
	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

// HandleGalgameSession GET /api/galgame/session/:id —— 读档。
func HandleGalgameSession(c *gin.Context) {
	session, err := loadGalSession(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, session)
}

// HandleGalgameDeleteSession DELETE /api/galgame/session/:id —— 删档（连图一起）。
func HandleGalgameDeleteSession(c *gin.Context) {
	dir, err := galSessionDir(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	galMu.Lock()
	err = os.RemoveAll(dir)
	galMu.Unlock()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func normalizeGalEmotion(e string) string {
	e = strings.ToLower(strings.TrimSpace(e))
	switch e {
	case "smile", "blush", "sad", "angry", "surprised", "serious", "normal":
		return e
	default:
		return "normal"
	}
}

func galEmotionPrompt(emotion string) string {
	switch emotion {
	case "smile":
		return "warm smile, cheerful expression"
	case "blush":
		return "blushing, shy embarrassed expression, looking away"
	case "sad":
		return "sad expression, downcast eyes, teary"
	case "angry":
		return "angry expression, furrowed brows, pouting"
	case "surprised":
		return "surprised expression, wide eyes, open mouth"
	case "serious":
		return "serious calm expression, steady gaze"
	default:
		return "neutral gentle expression"
	}
}

func galAssetURL(sessionID, file string) string {
	return fmt.Sprintf("/api/galgame/asset/%s/%s", sanitizeImageName(sessionID), filepath.Base(file))
}

// galAssetPath 把资源 URL 还原成本地路径，用于判断缓存的图是否还在。
func galAssetPath(sessionID, url string) string {
	dir, err := galSessionDir(sessionID)
	if err != nil {
		return ""
	}
	return filepath.Join(dir, filepath.Base(url))
}

func filterEmpty(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s := strings.TrimSpace(item); s != "" {
			out = append(out, s)
		}
	}
	return out
}
