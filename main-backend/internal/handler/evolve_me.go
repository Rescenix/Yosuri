package handler

// evolve_me.go — 聊天界面「AI 成长/进化」六轴雷达（真实数据源，禁游戏属相）。
// 六轴：产出 / 技能 / 协作 / 记忆 / 成功率 / 亲密度（0-100）。
// 数据全部来自磁盘/云端真实文件，不是前端硬编码。

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type evolveMe struct {
	XP    int    `json:"xp"`
	Level int    `json:"level"`
	Stage string `json:"stage"`

	Output       int `json:"output"`       // 产出力
	Skill         int `json:"skill"`        // 技能广度
	Collaboration int `json:"collab"`       // 协作度
	Memory        int `json:"memory"`       // 记忆沉淀
	Success       int `json:"success"`      // 成功率
	Intimacy      int `json:"intimacy"`     // 亲密度（ResceneCloud 权威）

	Refines  int `json:"refines"`
	Skills  int `json:"skills"`
	Memories int `json:"memories"`
	Outputs int `json:"outputs"`
	AxisMax int `json:"axis_max"`
}

func dataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", "rescene_data")
	}
	return filepath.Join(home, "rescene_data")
}

var evolveRefRe = regexp.MustCompile(`\b(designer|writer|researcher|coder|promoter|publisher|ceo)-[0-9]{1,3}\b`)

// HandleEvolveEvents GET /api/evolve/events — 最近技能习得事件（聊天界面气泡用）
func HandleEvolveEvents(c *gin.Context) {
	root := dataDir()
	type ev struct {
		Name string `json:"name"`
		At   string `json:"at"`
	}
	var events []ev
	seenName := map[string]bool{}
	collect := func(dir string) {
		ents, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range ents {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".json")
			var meta struct {
				Name string `json:"name"`
			}
			if data, err := os.ReadFile(filepath.Join(dir, e.Name())); err == nil {
				json.Unmarshal(data, &meta)
				if meta.Name != "" {
					name = meta.Name
				}
			}
			if seenName[name] {
				continue
			}
			seenName[name] = true
			info, _ := e.Info()
			var at string
			if info != nil {
				at = info.ModTime().Format(time.RFC3339)
			}
			events = append(events, ev{Name: name, At: at})
		}
	}
	// 全局技能库 + 各 agent 家目录 skills/（refine 物化技能落在这）
	collect(filepath.Join(root, "skills"))
	if ents, err := os.ReadDir(filepath.Join(root, "company")); err == nil {
		for _, e := range ents {
			if e.IsDir() {
				collect(filepath.Join(root, "company", e.Name(), "skills"))
			}
		}
	}
	sort.Slice(events, func(i, j int) bool { return events[i].At > events[j].At })
	if len(events) > 10 {
		events = events[:10]
	}
	c.JSON(http.StatusOK, gin.H{"events": events})
}
func HandleEvolveMe(c *gin.Context) {
	root := dataDir()
	st := evolveMe{AxisMax: 100}
	// 技能：共享技能库
	if ents, err := os.ReadDir(filepath.Join(root, "skills")); err == nil {
		for _, e := range ents {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
				st.Skills++
			}
		}
	}
	// 记忆：memorydir 的 [[链接]] 条目 + 块
	if data, err := os.ReadFile(filepath.Join(root, "memory", "index.md")); err == nil {
		st.Memories += strings.Count(string(data), "[[")
	}
	for _, f := range []string{"preferences", "project", "decisions", "memories", "pinned"} {
		if data, err := os.ReadFile(filepath.Join(root, "memory", f+".md")); err == nil {
			st.Memories += strings.Count(string(data), "## [")
		}
	}
	// 产出：outputs 目录文件（含 projects 子树）
	outputDir := filepath.Join(root, "outputs")
	if ents, err := os.ReadDir(outputDir); err == nil {
		for _, e := range ents {
			if !e.IsDir() {
				st.Outputs++
			}
		}
	}
	// 项目子目录里的可执行产物
	if pj, err := os.ReadDir(filepath.Join(outputDir, "projects")); err == nil {
		st.Outputs += len(pj)
	}
	// 成功率：会话里「有内容的 AI 回复」占比（真实会话记录，正经解析嵌套 JSON）
	var okMsg, badMsg int
	if data, err := os.ReadFile(filepath.Join(root, "sessions_chat_sessions.json")); err == nil {
		var sess map[string]struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
				Error   string `json:"error"`
			} `json:"messages"`
		}
		sess = map[string]struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
				Error   string `json:"error"`
			} `json:"messages"`
		}{}
		if json.Unmarshal(data, &sess) == nil {
			for _, s := range sess {
				for _, m := range s.Messages {
					if m.Role != "assistant" {
						continue
					}
					if m.Error == "" && strings.TrimSpace(m.Content) != "" {
						okMsg++
					} else {
						badMsg++
					}
				}
			}
		}
	}
	success := 0
	if okMsg+badMsg > 0 {
		success = int(float64(okMsg) / float64(okMsg+badMsg) * 100)
	}
	// 协作：会话/记忆里对同事 agent 的交叉引用（真实接力）
	collab := 0
	walkRefs := func(p string) {
		data, err := os.ReadFile(p)
		if err != nil {
			return
		}
		collab += len(evolveRefRe.FindAll(data, -1))
	}
	// agility：outputs 顶层 md/txt + 记忆文件
	if ents, err := os.ReadDir(outputDir); err == nil {
		for _, e := range ents {
			if !e.IsDir() && (strings.HasSuffix(e.Name(), ".md") || strings.HasSuffix(e.Name(), ".txt")) {
				walkRefs(filepath.Join(outputDir, e.Name()))
			}
		}
	}
	// 亲密度：ResceneCloud 权威（memorydir 落地）
	_, intimacyLv := readIntimacyGlobal()

	// 六轴归一化 0-100
	st.Output        = normScore(st.Outputs, 8)
	st.Skill         = normScore(st.Skills, 2) // 40 技能 → 80
	st.Collaboration = normScore(collab, 15)
	st.Memory        = normScore(st.Memories, 10)
	st.Success       = clampPct(success)
	st.Intimacy      = normScore(intimacyLv, 15)

	// XP / 等级 / 阶段
	st.XP = st.Skills*20 + st.Memories*10 + st.Outputs*5
	if st.XP > 0 {
		st.Level = int((1 + mathSqrt(1+8*float64(st.XP)/100)) / 2)
	}
	// 阶段：赛博成长链（静默电流→数据尘埃→神经漫游者→矩阵幽灵→冬寂），不含 Lv. 前缀（前端自己展示等级数字）
	st.Stage = "静默电流"
	if st.Skills >= 2 {
		st.Stage = "数据尘埃"
	}
	if st.Skills >= 5 && st.Memories >= 4 {
		st.Stage = "神经漫游者"
	}
	if st.Skills >= 10 && st.Memories >= 8 {
		st.Stage = "矩阵幽灵"
	}
	if st.Skills >= 20 && st.Memories >= 15 {
		st.Stage = "冬寂"
	}
	c.JSON(http.StatusOK, st)
}