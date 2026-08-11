package handler

// studio_semantic.go —— 文案成片 LLM 语义分析层（2026-08-06）
//
// 用户方向：「LLM 先进行语义分析，搜素材，然后文案 chunk 语义主题匹配，自动剪辑」
// 即：不再用滑动窗口关键词瞎猜素材，而是让 LLM 理解每句文案的语义，
// 给出「主题标签 + 英文搜索词」，素材库（Pixabay/Pexels 都是英文库）按语义词精准搜索。
//
// 输出结构传给 Python 引擎 mambo_video.py --semantic <json>：
//
//	[{"sentence":"原句","topic":"中文主题(2-6字)","search_terms":["dancing","party"]}]
//
// LLM 失败/无 key 时返回 nil，引擎自动降级到关键词+映射表兜底（保持可用性）。

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// studioSegmentPlan 每段的语义分析结果（LLM 输出 → 引擎输入 → manifest 记录）
type studioSegmentPlan struct {
	Sentence    string   `json:"sentence"`
	Topic       string   `json:"topic"`        // 中文主题标签：给用户看「这段在讲什么」
	SearchTerms []string `json:"search_terms"` // 英文搜索词：给素材库搜素材用
}

// studioSemanticResult 语义分析完整结果（引擎 --semantic 参数）
type studioSemanticResult struct {
	Topic    string              `json:"topic"`
	Segments []studioSegmentPlan `json:"segments"`
}

// analyzeStudioSegments 用 LLM 对分段文案做语义分析。
// 长文案按批分块调用 LLM（每批 15 句），合并结果——万字科普 250+ 句也能逐句配素材。
// 失败批跳过（部分分析总比没有好）；全部失败返回 nil（调用方静默降级）。
// 复用主对话的路由：用户配置的模型（含刚换的付费模型）> 免费池。
func analyzeStudioSegments(ctx context.Context, topic string, segments []string) *studioSemanticResult {
	if len(segments) == 0 {
		return nil
	}
	// 语义分析专用模型（2026-08-06 用户定「用 step credit」）：
	// ① Step Plan 订阅端点 plan_step_gateway（step-3.7-flash，走月池 Credit 不扣余额）
	// ② 回落免费 step-3.7-flash ③ 默认路由（用户配置 > 免费池）
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
		b = &bb
	}

	const batchSize = 15
	var all []studioSegmentPlan
	for start := 0; start < len(segments); start += batchSize {
		end := start + batchSize
		if end > len(segments) {
			end = len(segments)
		}
		batch := segments[start:end]
		plans := analyzeSemanticBatch(ctx, *b, topic, batch)
		if plans == nil {
			continue // 该批失败，跳过（后续批继续）
		}
		all = append(all, plans...)
	}
	if len(all) == 0 {
		return nil
	}
	return &studioSemanticResult{Topic: topic, Segments: all}
}

// analyzeSemanticBatch 单批语义分析：prompt → LLM → 容错解析 → 校验清洗。
func analyzeSemanticBatch(ctx context.Context, b RouterBackend, topic string, segments []string) []studioSegmentPlan {
	segJSON, _ := json.Marshal(segments)

	prompt := fmt.Sprintf(`你是短视频素材导演。用户要做一个主题为「%s」的口播短视频，文案已按句分成若干段。
请对每一句做语义分析，输出 JSON 数组，不要任何解释、不要 markdown 代码块。

每个元素格式：
{"sentence": "原句原样", "topic": "中文主题标签(2-6字，概括这句的画面内容)", "search_terms": ["2-4个英文搜索词"]}

要求：
- search_terms 用英文，适合在 Pixabay/Pexels 免费素材库搜索视频，贴近素材库常见标签
  （如：跳舞→dancing, 音乐→music, 城市夜景→city night, 咖啡→coffee, 跑步→running）
- 抽象/情绪句给画面联想词（如「奋斗」→motivation, hard work）
- 梗词/拟声词（如「曼波」「哇哦」）给相近的画面词，实在没有就用通用氛围词
- 严格按输入段落顺序输出，每条 sentence 必须与输入逐字一致

输入段落（JSON 数组）：
%s`, topic, string(segJSON))

	msgs := []map[string]any{{"role": "user", "content": prompt}}
	// 语义分析是前置步骤：单批 30s 超时，超时就跳过该批，不拖住整个成片
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	content, _, err := openAIChatOnce(ctx, b, msgs, nil)
	if err != nil {
		return nil
	}

	// 容错解析：先试完整数组，再试截取 [] 之间的片段
	plans := []studioSegmentPlan{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &plans); err != nil {
		start := strings.Index(content, "[")
		end := strings.LastIndex(content, "]")
		if start < 0 || end <= start || json.Unmarshal([]byte(content[start:end+1]), &plans) != nil {
			return nil
		}
	}
	// 校验：sentence 与输入对齐、search_terms 非空；不合格的段剔除
	valid := plans[:0]
	seen := map[string]bool{}
	for _, p := range plans {
		p.Sentence = strings.TrimSpace(p.Sentence)
		if p.Sentence == "" || seen[p.Sentence] || len(p.SearchTerms) == 0 {
			continue
		}
		seen[p.Sentence] = true
		valid = append(valid, p)
	}
	return valid
}

// splitStudioSentences 与引擎 scripts/mambo_video.py split_sentences 一致：
// 有 | 按 | 分段；否则按中文句读切分。（语义分析的结果要按句喂给 LLM，
// sentence 字段必须和引擎切分结果逐字一致，才能命中 semantic_map。）
func splitStudioSentences(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	var out []string
	if strings.Contains(text, "|") {
		for _, s := range strings.Split(text, "|") {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	for _, s := range strings.FieldsFunc(text, func(r rune) bool {
		return r == '。' || r == '！' || r == '？' || r == '!' || r == '?' || r == '；' || r == ';' || r == '\n'
	}) {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// semanticForSegment 引擎侧兜底：--semantic 里没覆盖到的段，退回原句
func (r *studioSemanticResult) searchTermsFor(sentence string) []string {
	if r == nil {
		return nil
	}
	for _, p := range r.Segments {
		if strings.TrimSpace(p.Sentence) == strings.TrimSpace(sentence) {
			return p.SearchTerms
		}
	}
	return nil
}
