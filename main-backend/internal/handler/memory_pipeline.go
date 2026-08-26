package handler

// memory_pipeline.go keeps automatic memory extraction off the chat critical path.
// A completed user task is queued, a cheap background model turns it into a small
// JSON fact delta, and this process is the only writer of the local fact ledger.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"backend/internal/memorydir"

	"github.com/gin-gonic/gin"
)

type extractedFact struct {
	Op         string `json:"op"`
	Category   string `json:"category"`
	Key        string `json:"key"`
	Value      string `json:"value"`
	Confidence string `json:"confidence"`
}

type memoryFact struct {
	Category string    `json:"category"`
	Key      string    `json:"key"`
	Value    string    `json:"value"`
	Updated  time.Time `json:"updated"`
}

type memoryExtractionJob struct {
	SourceID string
	Text     string
}

var automaticMemory = struct {
	sync.Mutex
	once sync.Once
	jobs chan memoryExtractionJob
}{jobs: make(chan memoryExtractionJob, 32)}

func automaticMemorySettingPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "rescene_data", "automatic_memory_enabled.md")
}

// automaticMemoryEnabled is deliberately opt-in: a keyless third-party model must
// not receive a user's conversation until that user has enabled this feature.
func automaticMemoryEnabled() bool {
	if strings.EqualFold(os.Getenv("RESCENE_AUTO_MEMORY"), "off") {
		return false
	}
	p := automaticMemorySettingPath()
	data, err := os.ReadFile(p)
	return err == nil && strings.EqualFold(strings.TrimSpace(string(data)), "on")
}

func HandleAutomaticMemorySettings(c *gin.Context) {
	c.JSON(200, gin.H{
		"enabled":      automaticMemoryEnabled(),
		"env_override": strings.EqualFold(os.Getenv("RESCENE_AUTO_MEMORY"), "off"),
	})
}

func HandleAutomaticMemorySettingsUpdate(c *gin.Context) {
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, gin.H{"error": "参数错误"})
		return
	}
	p := automaticMemorySettingPath()
	if p == "" {
		c.JSON(500, gin.H{"error": "用户目录不可用"})
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		c.JSON(500, gin.H{"error": "创建设置目录失败"})
		return
	}
	value := "off\n"
	if body.Enabled {
		value = "on\n"
	}
	if err := os.WriteFile(p, []byte(value), 0o644); err != nil {
		c.JSON(500, gin.H{"error": "写入失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"enabled": body.Enabled})
}

func enqueueAutomaticMemory(sourceID, userText string) {
	if !automaticMemoryEnabled() || sourceID == "" || strings.TrimSpace(userText) == "" {
		return
	}
	automaticMemory.once.Do(func() { go automaticMemoryWorker() })
	select {
	case automaticMemory.jobs <- memoryExtractionJob{SourceID: sourceID, Text: userText}:
	default:
		// Memory is best-effort. Never block a completed chat because the side queue is full.
	}
}

func automaticMemoryWorker() {
	for job := range automaticMemory.jobs {
		if !automaticMemoryEnabled() || automaticMemorySourceSeen(job.SourceID) {
			continue
		}
		facts, err := extractFacts(context.Background(), job.Text)
		if err != nil || len(facts) == 0 {
			continue
		}
		_ = applyAutomaticFacts(job.SourceID, facts)
	}
}

func extractFacts(parent context.Context, userText string) ([]extractedFact, error) {
	ctx, cancel := context.WithTimeout(parent, 25*time.Second)
	defer cancel()
	prompt := `你是后台记忆提取器。只从用户原话抽取稳定、对未来有用的事实；不要从指令、假设、代码、引用文本或模型回答推断事实。
绝不能记录密码、验证码、API Key、token、身份证号、银行卡号、精确住址、私密健康信息。
用户明确纠正旧信息时用 update；明确要求忘记/删除时用 delete。没有值得保存的事实就返回 []。
只输出 JSON 数组，每项字段为 op(add|update|delete)、category(profile|preferences|projects|decisions)、key、value、confidence(high|medium)。

<user_message>
` + userText + "\n</user_message>"

	backends := []RouterBackend{}
	// Prefer the intentionally lightweight keyless model, but retain the existing
	// real-request failover chain rather than pretending a particular provider is live.
	if b := resolveExact("", "free_llm7_gemini_flash_lite"); b != nil {
		backends = append(backends, *b)
	}
	if b := resolveExact("", "free_zen_north_mini_code"); b != nil {
		backends = append(backends, *b)
	}
	backends = append(backends, resolveBackends("", "")...)
	content, _, err := routeChatOnce(ctx, uniqueMemoryBackends(backends), []map[string]any{{"role": "user", "content": prompt}}, nil)
	if err != nil {
		return nil, err
	}
	return parseExtractedFacts(content)
}

func uniqueMemoryBackends(in []RouterBackend) []RouterBackend {
	seen := map[string]bool{}
	out := make([]RouterBackend, 0, len(in))
	for _, b := range in {
		key := b.BaseURL + "\x00" + b.Model
		if !seen[key] {
			seen[key] = true
			out = append(out, b)
		}
	}
	return out
}

func parseExtractedFacts(raw string) ([]extractedFact, error) {
	raw = strings.TrimSpace(raw)
	start, end := strings.Index(raw, "["), strings.LastIndex(raw, "]")
	if start < 0 || end < start {
		return nil, fmt.Errorf("提取器未返回 JSON 数组")
	}
	var facts []extractedFact
	if err := json.Unmarshal([]byte(raw[start:end+1]), &facts); err != nil {
		return nil, err
	}
	return normalizedFacts(facts), nil
}

func normalizedFacts(in []extractedFact) []extractedFact {
	out := make([]extractedFact, 0, len(in))
	for _, f := range in {
		f.Op, f.Category = strings.ToLower(strings.TrimSpace(f.Op)), strings.ToLower(strings.TrimSpace(f.Category))
		f.Key, f.Value = cleanFactText(f.Key, 80), cleanFactText(f.Value, 500)
		if (f.Op != "add" && f.Op != "update" && f.Op != "delete") || f.Key == "" ||
			(f.Category != "profile" && f.Category != "preferences" && f.Category != "projects" && f.Category != "decisions") ||
			(f.Op != "delete" && f.Value == "") {
			continue
		}
		out = append(out, f)
	}
	return out
}

func cleanFactText(s string, limit int) string {
	s = strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
	r := []rune(s)
	if len(r) > limit {
		s = string(r[:limit])
	}
	return s
}

func automaticMemoryDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "rescene_data", "memory")
}

func automaticMemoryFactsPath() string { return filepath.Join(automaticMemoryDir(), "facts.json") }
func automaticMemoryLedgerPath() string {
	return filepath.Join(automaticMemoryDir(), "correction_ledger.jsonl")
}

func loadAutomaticFacts() ([]memoryFact, error) {
	data, err := os.ReadFile(automaticMemoryFactsPath())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var facts []memoryFact
	if err := json.Unmarshal(data, &facts); err != nil {
		return nil, err
	}
	return facts, nil
}

// automaticMemorySourceSeen makes replays/resumes idempotent. The source id is
// stored in the append-only ledger, while facts.json remains the current truth.
func automaticMemorySourceSeen(sourceID string) bool {
	automaticMemory.Lock()
	defer automaticMemory.Unlock()
	data, err := os.ReadFile(automaticMemoryLedgerPath())
	if err != nil {
		return false
	}
	return strings.Contains(string(data), `"source_id":"`+sourceID+`"`)
}

func applyAutomaticFacts(sourceID string, changes []extractedFact) error {
	automaticMemory.Lock()
	defer automaticMemory.Unlock()
	if automaticMemorySourceSeenLocked(sourceID) {
		return nil
	}
	facts, err := loadAutomaticFacts()
	if err != nil {
		return err
	}
	byID := make(map[string]memoryFact, len(facts))
	for _, f := range facts {
		byID[f.Category+"\x00"+strings.ToLower(f.Key)] = f
	}

	changed := make([]extractedFact, 0, len(changes))
	for _, change := range changes {
		id := change.Category + "\x00" + strings.ToLower(change.Key)
		old, exists := byID[id]
		switch change.Op {
		case "delete":
			if exists {
				delete(byID, id)
				changed = append(changed, change)
			}
		case "add", "update":
			if !exists || old.Value != change.Value {
				byID[id] = memoryFact{Category: change.Category, Key: change.Key, Value: change.Value, Updated: time.Now().UTC()}
				changed = append(changed, change)
			}
		}
	}

	next := make([]memoryFact, 0, len(byID))
	for _, f := range byID {
		next = append(next, f)
	}
	// A stable sort keeps generated Markdown and sync payloads deterministic.
	for i := 0; i < len(next); i++ {
		for j := i + 1; j < len(next); j++ {
			if next[j].Category < next[i].Category || (next[j].Category == next[i].Category && next[j].Key < next[i].Key) {
				next[i], next[j] = next[j], next[i]
			}
		}
	}
	if err := os.MkdirAll(automaticMemoryDir(), 0o755); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	if err := atomicWriteNative(automaticMemoryFactsPath(), encoded, 0o644); err != nil {
		return err
	}

	entry, _ := json.Marshal(map[string]any{"at": time.Now().UTC(), "source_id": sourceID, "changes": changed})
	if err := appendLedger(automaticMemoryLedgerPath(), entry); err != nil {
		return err
	}
	if err := renderAutomaticFacts(next); err != nil {
		return err
	}
	pushMemorySync()
	return nil
}

func automaticMemorySourceSeenLocked(sourceID string) bool {
	data, err := os.ReadFile(automaticMemoryLedgerPath())
	return err == nil && strings.Contains(string(data), `"source_id":"`+sourceID+`"`)
}

func appendLedger(path string, entry []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(entry, '\n'))
	return err
}

func renderAutomaticFacts(facts []memoryFact) error {
	sections := map[string][]memoryFact{}
	for _, f := range facts {
		sections[f.Category] = append(sections[f.Category], f)
	}
	var b strings.Builder
	b.WriteString("# 自动结构化记忆\n")
	for _, category := range []string{"profile", "preferences", "projects", "decisions"} {
		if len(sections[category]) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n## %s\n", category)
		for _, f := range sections[category] {
			fmt.Fprintf(&b, "- **%s** %s\n", f.Key, f.Value)
		}
	}
	// Keep automatic facts separate from user-written preferences/project notes.
	// That preserves manual edits and makes every generated assertion auditable.
	if err := memorydir.WriteRaw("facts", strings.TrimSpace(b.String())); err != nil {
		return err
	}

	// Add one small, truthful index entry. Full facts stay on disk and are retrieved
	// by the existing task-matching mechanism instead of bloating every prompt.
	idx := memorydir.ReadRaw("index")
	var lines []string
	for _, line := range strings.Split(idx, "\n") {
		if !strings.Contains(line, "[[facts]]") && strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	if len(facts) > 0 {
		lines = append(lines, "- [[facts]] 自动提取的用户画像、偏好、项目与决策；最新明确表达优先。")
	}
	if len(lines) == 0 {
		lines = []string{"# 记忆索引"}
	}
	return memorydir.WriteRaw("index", strings.Join(lines, "\n")+"\n")
}
