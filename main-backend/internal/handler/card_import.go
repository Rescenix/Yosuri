package handler

// card_import.go —— 酒馆（Tavern）角色卡导入。
//
// 支持社区事实标准 character-card-spec 的 V1 / V2 / V3，三种载体：
//
//	.png   —— JSON 经 base64 后塞进 PNG 的 tEXt/iTXt chunk（V2 用 "chara"，V3 用 "ccv3"）
//	.json  —— 直接是卡片 JSON
//	.charx —— V3 的 zip 包（内含 card.json + assets）
//
// 导入后落成一张 AgentCard；卡片自带的世界书写进该角色的私有世界书。
//
//	POST /api/cards/import  body: {"filename": "...", "data": "data:...;base64,..." | "<base64>"}

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

var pngSignature = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

// maxCardFileBytes 单张卡上限：正常卡（含立绘）远小于这个数。
const maxCardFileBytes = 24 << 20

// importedCard 把 V1/V2/V3 归一化后的结果。
type importedCard struct {
	Name         string
	Persona      string // description + personality + scenario + mes_example，分段拼好
	Greeting     string
	AltGreetings []string
	MesExample   string
	Tags         []string
	Creator      string
	Spec         string // "v1" / "v2" / "v3"
	WorldBook    []importedLore
}

type importedLore struct {
	Name      string
	Keys      []string
	Keys2     []string
	Content   string
	Enabled   bool
	Constant  bool
	Selective bool
	CaseMatch bool
	Order     int
	Depth     int
	Position  string
}

// ── 原始 JSON 结构（V2/V3 用 data 包一层，V1 扁平）────────────────

type rawCard struct {
	Spec             string          `json:"spec"`
	Name             string          `json:"name"`
	Description      string          `json:"description"`
	Personality      string          `json:"personality"`
	Scenario         string          `json:"scenario"`
	FirstMes         string          `json:"first_mes"`
	MesExample       string          `json:"mes_example"`
	AltGreetings     []string        `json:"alternate_greetings"`
	SystemPrompt     string          `json:"system_prompt"`
	PostHistory      string          `json:"post_history_instructions"`
	Creator          string          `json:"creator"`
	CharacterVersion string          `json:"character_version"`
	Tags             []string        `json:"tags"`
	CharacterBook    json.RawMessage `json:"character_book"`
	// V3 独有
	Nickname           string   `json:"nickname"`
	GroupOnlyGreetings []string `json:"group_only_greetings"`
	// V2/V3 把真正的字段包在 data 里
	Data *rawCard `json:"data"`
}

type rawLoreEntry struct {
	Keys           []string       `json:"keys"`
	Content        string         `json:"content"`
	Name           string         `json:"name"`
	Comment        string         `json:"comment"`
	Enabled        *bool          `json:"enabled"`
	Constant       bool           `json:"constant"`
	Selective      bool           `json:"selective"`
	SecondaryKeys  []string       `json:"secondary_keys"`
	CaseSensitive  bool           `json:"case_sensitive"`
	InsertionOrder int            `json:"insertion_order"`
	Position       string         `json:"position"`
	Extensions     map[string]any `json:"extensions"`
}

// ── PNG 文本块 ────────────────────────────────────────────────

// pngTextChunks 抓 PNG 里的 tEXt / iTXt 块，只留角色卡关心的两个 keyword。
func pngTextChunks(data []byte) map[string]string {
	out := map[string]string{}
	pos := len(pngSignature)
	for pos+8 <= len(data) {
		length := int(binary.BigEndian.Uint32(data[pos : pos+4]))
		typ := string(data[pos+4 : pos+8])
		if length < 0 || pos+12+length > len(data) {
			break
		}
		body := data[pos+8 : pos+8+length]
		pos += 12 + length
		switch typ {
		case "tEXt":
			// keyword\0text
			if i := bytes.IndexByte(body, 0); i > 0 {
				kw := string(body[:i])
				if kw == "chara" || kw == "ccv3" {
					out[kw] = string(body[i+1:])
				}
			}
		case "iTXt":
			// keyword\0 compFlag compMethod langTag\0 translatedKeyword\0 text
			if i := bytes.IndexByte(body, 0); i > 0 {
				kw := string(body[:i])
				if kw != "chara" && kw != "ccv3" {
					continue
				}
				rest := body[i+1:]
				if len(rest) < 2 || rest[0] == 1 { // 1 = zlib 压缩，社区导出基本不用
					continue
				}
				rest = rest[2:]
				if j := bytes.IndexByte(rest, 0); j >= 0 {
					rest = rest[j+1:]
				}
				if j := bytes.IndexByte(rest, 0); j >= 0 {
					rest = rest[j+1:]
				}
				out[kw] = string(rest)
			}
		}
	}
	return out
}

// ── 解析入口 ──────────────────────────────────────────────────

func decodeCardBase64(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, s)
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	// 部分导出省略了 padding
	return base64.RawStdEncoding.DecodeString(s)
}

// parseCharacterCard 按文件魔数分流。
func parseCharacterCard(data []byte) (*importedCard, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("文件是空的")
	}
	switch {
	case bytes.HasPrefix(data, pngSignature):
		texts := pngTextChunks(data)
		raw := texts["ccv3"]
		if raw == "" {
			raw = texts["chara"]
		}
		if raw == "" {
			return nil, fmt.Errorf("这张图片里没有角色卡数据。多半是微信/QQ 转发、截图或二次保存把隐藏数据弄丢了——请直接下载原始的 PNG 卡文件再导入")
		}
		js, err := decodeCardBase64(raw)
		if err != nil {
			return nil, fmt.Errorf("角色卡数据损坏，无法解码")
		}
		return parseCardJSON(js)
	case bytes.HasPrefix(data, []byte("PK\x03\x04")):
		return parseCharx(data)
	default:
		return parseCardJSON(data)
	}
}

func parseCharx(data []byte) (*importedCard, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("CHARX 包损坏，无法解压")
	}
	var fallback []byte
	for _, f := range zr.File {
		name := strings.ToLower(f.Name)
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(rc, maxCardFileBytes))
		rc.Close()
		if strings.HasSuffix(name, "card.json") {
			return parseCardJSON(body)
		}
		if fallback == nil {
			fallback = body
		}
	}
	if fallback != nil {
		return parseCardJSON(fallback)
	}
	return nil, fmt.Errorf("CHARX 包里找不到角色卡数据")
}

func parseCardJSON(data []byte) (*importedCard, error) {
	var raw rawCard
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("这不是角色卡：文件既不是图片，也不是可识别的 JSON")
	}
	// V2 / V3：真正字段在 data 里
	if raw.Data != nil && strings.TrimSpace(raw.Data.Name) != "" {
		out := cardFromRaw(raw.Data)
		if strings.Contains(raw.Spec, "chara_card_v3") {
			out.Spec = "v3"
		} else {
			out.Spec = "v2"
		}
		return out, nil
	}
	// V1：扁平
	if strings.TrimSpace(raw.Name) == "" {
		return nil, fmt.Errorf("这不是角色卡：里面找不到角色名字")
	}
	out := cardFromRaw(&raw)
	out.Spec = "v1"
	return out, nil
}

func cardFromRaw(r *rawCard) *importedCard {
	name := strings.TrimSpace(r.Name)
	if name == "" {
		name = strings.TrimSpace(r.Nickname)
	}
	out := &importedCard{
		Name:         name,
		Greeting:     strings.TrimSpace(r.FirstMes),
		MesExample:   strings.TrimSpace(r.MesExample),
		AltGreetings: r.AltGreetings,
		Tags:         r.Tags,
		Creator:      strings.TrimSpace(r.Creator),
	}

	var parts []string
	add := func(label, v string) {
		if s := strings.TrimSpace(v); s != "" {
			parts = append(parts, label+"\n"+s)
		}
	}
	add("【人设】", r.Description)
	add("【性格】", r.Personality)
	add("【场景】", r.Scenario)
	add("【示例对话】", r.MesExample)
	out.Persona = strings.Join(parts, "\n\n")

	// 世界书：数组 / 对象 map 两种导出都见过
	if len(r.CharacterBook) > 0 {
		out.WorldBook = parseLorebook(r.CharacterBook)
	}
	return out
}

func parseLorebook(raw json.RawMessage) []importedLore {
	var lb struct {
		Entries json.RawMessage `json:"entries"`
	}
	if err := json.Unmarshal(raw, &lb); err != nil || len(lb.Entries) == 0 {
		return nil
	}
	var list []rawLoreEntry
	// 先按数组解
	if err := json.Unmarshal(lb.Entries, &list); err != nil {
		// 再按 {"0": {...}, "1": {...}} 解
		var m map[string]rawLoreEntry
		if err2 := json.Unmarshal(lb.Entries, &m); err2 != nil {
			return nil
		}
		for _, v := range m {
			list = append(list, v)
		}
	}
	out := make([]importedLore, 0, len(list))
	for _, e := range list {
		content := strings.TrimSpace(e.Content)
		if content == "" {
			continue
		}
		enabled := true
		if e.Enabled != nil {
			enabled = *e.Enabled
		}
		l := importedLore{
			Name:      strings.TrimSpace(e.Name),
			Keys:      trimKeys(e.Keys),
			Keys2:     trimKeys(e.SecondaryKeys),
			Content:   content,
			Enabled:   enabled,
			Constant:  e.Constant,
			Selective: e.Selective,
			CaseMatch: e.CaseSensitive,
			Order:     e.InsertionOrder,
			Position:  "before",
		}
		if l.Name == "" {
			l.Name = strings.TrimSpace(e.Comment)
		}
		if l.Name == "" && len(l.Keys) > 0 {
			l.Name = l.Keys[0]
		}
		switch strings.ToLower(e.Position) {
		case "after", "after_char":
			l.Position = "after"
		}
		if e.Extensions != nil {
			if v, ok := e.Extensions["depth"].(float64); ok {
				l.Depth = int(v)
			}
		}
		out = append(out, l)
	}
	return out
}

func trimKeys(in []string) []string {
	out := make([]string, 0, len(in))
	for _, k := range in {
		if s := strings.TrimSpace(k); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// ── HTTP ──────────────────────────────────────────────────────

// HandleImportCharacterCard POST /api/cards/import
func HandleImportCharacterCard(c *gin.Context) {
	var req struct {
		Filename string `json:"filename"`
		Data     string `json:"data"` // dataURL 或裸 base64
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体不是合法 JSON"})
		return
	}
	b64 := strings.TrimSpace(req.Data)
	if strings.HasPrefix(b64, "data:") {
		if i := strings.Index(b64, ","); i > 0 {
			b64 = b64[i+1:]
		}
	}
	raw, err := decodeCardBase64(b64)
	if err != nil || len(raw) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件内容读不出来，换一个文件试试"})
		return
	}
	if len(raw) > maxCardFileBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "角色卡文件太大（上限 24MB）"})
		return
	}

	card, err := parseCharacterCard(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	warnings := []string{}
	persona := card.Persona
	if len([]rune(persona)) > 8000 {
		persona = string([]rune(persona)[:8000])
		warnings = append(warnings, "人设超过 8000 字，已截断（酒馆卡里常见超长设定）")
	}

	saved, err := UpsertAgent(AgentCard{
		Name:         card.Name,
		Persona:      persona,
		Greeting:     card.Greeting,
		AltGreetings: card.AltGreetings,
		MesExample:   card.MesExample,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 头像：PNG 卡本身就是立绘，直接用原图
	if bytes.HasPrefix(raw, pngSignature) {
		if len(raw) <= 3*1024*1024 {
			dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(raw)
			if err := SaveAgentAvatar(saved.ID, dataURL); err != nil {
				warnings = append(warnings, "头像没存上："+err.Error())
			}
		} else {
			warnings = append(warnings, "立绘超过 3MB，头像已跳过（人设照常导入）")
		}
	}

	// 世界书 → 该角色的私有世界书
	loreCount := 0
	if len(card.WorldBook) > 0 {
		lb := &WorldBook{Name: saved.Name + " 的世界书"}
		for i, e := range card.WorldBook {
			lb.Entries = append(lb.Entries, LoreEntry{
				UID:       i + 1,
				Name:      e.Name,
				Keys:      e.Keys,
				Keys2:     e.Keys2,
				Content:   e.Content,
				Enabled:   e.Enabled,
				Constant:  e.Constant,
				Selective: e.Selective,
				CaseMatch: e.CaseMatch,
				Order:     e.Order,
				Depth:     e.Depth,
				Position:  e.Position,
			})
		}
		if err := writeLoreFile(agentLoreFilePath(saved.ID), lb); err != nil {
			warnings = append(warnings, "世界书没导入："+err.Error())
		} else {
			loreCount = len(lb.Entries)
		}
	}

	fillAgentAvatar(&saved)
	c.JSON(http.StatusOK, gin.H{
		"ok":                true,
		"agent":             saved,
		"spec":              card.Spec,
		"worldbook_entries": loreCount,
		"warnings":          warnings,
	})
}
