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
	Context  string // 最近一段 user+assistant 对话，用于补全偏好/风格/语气画像
}

var automaticMemory = struct {
	sync.Mutex
	once sync.Once
	jobs chan memoryExtractionJob
}{jobs: make(chan memoryExtractionJob, 32)}

// automaticMemoryEnabled 是否启用自动提取事实（默认内置开启）。
// 2026-08-31 从 opt-in 翻转为默认开：会话文本本就经过聚合免费模型提炼，
// 让同一模型再提取偏好/项目/修正不构成额外外发，单独配置开关是过度设计。
// 仅保留部署级后门 RESCENE_AUTO_MEMORY=off 强制关闭（隐私护栏）。
func automaticMemoryEnabled() bool {
	return !strings.EqualFold(os.Getenv("RESCENE_AUTO_MEMORY"), "off")
}

func enqueueAutomaticMemory(sourceID, userText, context string) {
	if !automaticMemoryEnabled() || sourceID == "" || strings.TrimSpace(userText) == "" {
		return
	}
	automaticMemory.once.Do(func() { go automaticMemoryWorker() })
	select {
	case automaticMemory.jobs <- memoryExtractionJob{SourceID: sourceID, Text: userText, Context: context}:
	default:
		// Memory is best-effort. Never block a completed chat because the side queue is full.
	}
}

func automaticMemoryWorker() {
	for job := range automaticMemory.jobs {
		if !automaticMemoryEnabled() || automaticMemorySourceSeen(job.SourceID) {
			continue
		}
		facts, err := extractFacts(context.Background(), job.Text, job.Context)
		if err != nil || len(facts) == 0 {
			continue
		}
		_ = applyAutomaticFacts(job.SourceID, facts)
	}
}

func extractFacts(parent context.Context, userText, dialogContext string) ([]extractedFact, error) {
	ctx, cancel := context.WithTimeout(parent, 25*time.Second)
	defer cancel()
	prompt := `你是后台记忆提取器。从最近一段对话里抽取用户稳定、对未来有用的画像事实：
值得长期记住的判据（只记这两类）：
- 稳定偏好、风格与画像：你偏好的语气、长度、称呼、语言、忌讳、在意什么、个人属性（如职业/语言/爱好）→ preferences
- 项目与决定：你在做的项目、长期目标、明确的决策 → projects / decisions
- 不记：一句带过的临时请求、一次性任务、演示/测试/mock/示例/假设场景、闲聊、已过期信息；拿不准的宁可返回 []，别硬记。
- 从用户如何发问/回应里观察到的稳定风格偏好（喜欢的语气、长度、是否夹英文、讨厌什么、在意什么）→ 归入 preferences
不要从指令、假设、代码、引用文本或模型自己的回答反推事实——只看用户这侧能坐实的东西。
绝不能记录密码、验证码、API Key、token、身份证号、银行卡号、精确住址、私密健康信息。
用户明确纠正旧信息时用 update；明确要求忘记/删除时用 delete。没有值得保存的事实就返回 []。
只输出 JSON 数组，每项字段为 op(add|update|delete)、category(profile|preferences|projects|decisions)、key、value、confidence(high|medium)。

<user_message>
` + userText + "\n</user_message>"
	if strings.TrimSpace(dialogContext) != "" {
		prompt += "\n\n<recent_dialog>\n" + dialogContext + "\n</recent_dialog>"
	}

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
		if sensitiveFactKey(f.Key) {
			continue
		}
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
	// 把自动提取的事实按输出文件归类：profile 并入 preferences（用户画像即偏好）。
	byFile := map[string][]memoryFact{}
	for _, f := range facts {
		cat := f.Category
		if cat == "profile" {
			cat = "preferences"
		}
		byFile[cat] = append(byFile[cat], f)
	}
	// 自动事实写进独立 facts.md（分类用 ## 分区），与手动维护的
	// preferences/projects/decisions 文件完全隔离——自动提取绝不覆盖手动记忆。
	var b strings.Builder
	b.WriteString("# 自动提取记忆\n")
	for _, category := range []string{"preferences", "projects", "decisions"} {
		items := byFile[category]
		if len(items) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n## %s\n", category)
		for _, f := range items {
			fmt.Fprintf(&b, "- **%s** %s\n", f.Key, f.Value)
		}
	}
	if err := memorydir.WriteRaw("facts", strings.TrimSpace(b.String())); err != nil {
		return err
	}
	// 索引行自描述：[[facts]] + 内容概要，让 agent 一看就知道里面存了什么，按需点开。
	idx := memorydir.ReadRaw("index")
	var lines []string
	for _, line := range strings.Split(idx, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.Contains(line, "[[facts]]") {
			continue // 去掉旧的 facts 条目，下面统一追加最新一条
		}
		lines = append(lines, line)
	}
	if len(facts) > 0 {
		lines = append(lines, "- [[facts]] 自动提取的用户偏好、项目与决策（最新明确表达优先）。")
	}
	if len(lines) == 0 {
		lines = []string{"# 记忆索引"}
	}
	return memorydir.WriteRaw("index", strings.Join(lines, "\n")+"\n")
}

// mockNoiseKey 检测一条事实是不是 mock/演示/测试噪音。
func mockNoiseKey(key, value string) bool {
	k := strings.ToLower(key + " " + value)
	for _, noise := range []string{"mock", "demo", "测试", "test", "示例", "演示", "flowup", "子代理", "后台任务", "pdf_delivery"} {
		if strings.Contains(k, noise) {
			return true
		}
	}
	return false
}

// consolidateFacts 一次性存量清洗：去掉 mock/演示/测试噪音 + key 归一合并。
// 启动时跑一次，幂等（跑过 clean 的文件不会再产生相同噪音，因为提取 prompt 已挡新增）。
func consolidateFacts() {
	if !automaticMemoryEnabled() {
		return
	}
	automaticMemory.Lock()
	defer automaticMemory.Unlock()

	facts, err := loadAutomaticFacts()
	if err != nil || len(facts) == 0 {
		return
	}

	// key 归一表：别名 → 标准 key
	aliases := map[string]string{
		"preferred_language":      "language",
		"preferred_output_format": "output_format",
		"preferred_message_length": "message_length",
		"response_length":         "message_length",
		"use_of_emoji":            "emoji_usage",
		"formality":               "tone",
		"mock_backend_task_length": "duration_preference",
	}

	byID := make(map[string]memoryFact, len(facts))
	for _, f := range facts {
		if mockNoiseKey(f.Key, f.Value) {
			continue
		}
		normKey := strings.ToLower(f.Key)
		if alias, ok := aliases[normKey]; ok {
			normKey = alias
			f.Key = alias
		}
		id := f.Category + "\x00" + normKey
		if old, exists := byID[id]; !exists || f.Updated.After(old.Updated) {
			byID[id] = f
		}
	}

	if len(byID) == len(facts) {
		return
	}

	next := make([]memoryFact, 0, len(byID))
	for _, f := range byID {
		next = append(next, f)
	}
	for i := 0; i < len(next); i++ {
		for j := i + 1; j < len(next); j++ {
			if next[j].Category < next[i].Category || (next[j].Category == next[i].Category && next[j].Key < next[i].Key) {
				next[i], next[j] = next[j], next[i]
			}
		}
	}
	os.MkdirAll(automaticMemoryDir(), 0o755)
	encoded, _ := json.MarshalIndent(next, "", "  ")
	atomicWriteNative(automaticMemoryFactsPath(), encoded, 0o644)
	renderAutomaticFacts(next)
	pushMemorySync()
}

// sensitiveFactKey 命中的字段直接丢弃：位置/IP/设备/证件/电话/卡号等。这类
// 信息对 agent 执行任务几乎无价值，却直接暴露用户隐私；不依赖提取模型遵守
// prompt 黑名单，在归一化层统一兜底过滤（2026-09-02 实锤：某号被记录了
// location，且该用户已在对话中表达过位置隐私担忧）。
func sensitiveFactKey(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	switch k {
	case "location", "address", "city", "region", "province", "state", "home", "house",
		"ip", "ip_address", "ipaddr", "device", "device_id", "deviceid", "fingerprint",
		"mac", "mac_address", "sn", "serial", "id_card", "idcard", "identity", "identity_no",
		"phone", "phone_number", "mobile", "tel", "bank_card", "bankcard", "credit_card", "card_no":
		return true
	}
	// 前缀命中：location=xxx / device_xxx / 精确住址 / 身份证 / 银行卡等常见写法
	for _, p := range []string{"location", "address", "device", "fingerprint", "ip", "phone", "住址", "身份证", "银行卡", "地址"} {
		if strings.HasPrefix(k, p) {
			return true
		}
	}
	return false
}
