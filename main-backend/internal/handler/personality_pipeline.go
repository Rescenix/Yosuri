package handler

// personality_pipeline.go —— 记忆淬炼性格，千人千面。
//
// 性格不是用户填的表单，是从交互记忆里蒸馏出来的：
//   - 事实记忆（facts.json）：tone / message_length / language / emoji_usage ...
//   - 亲密度（intimacy.md）：语气亲疏阈值
//   - 常驻记忆（pinned.md）：用户显式设定的身份/称呼/指令
//
// 蒸馏产物 memory/personality.md 由 context_provider 每轮注入系统提示词，
// 模型据此调整语气/长度/自称/忌讳，实现千人千面。
//
// 触发：启动时一次 + 记忆变更 debounce 30s + 亲密度升级。

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"backend/internal/memorydir"
)

var personalityPipeline = struct {
	sync.Mutex
	once   sync.Once
	timer  *time.Timer
	mu     sync.Mutex
}{
	timer: nil,
}

// personalityProfile 蒸馏过程中用到的中间结构。
type personalityProfile struct {
	Tone         string
	Length       string
	Language     string
	Emoji        string
	SelfName     string
	Taboos       []string
	IntimacyLevel int64
	IntimacyValue int64
	Extra        map[string]string // 未归一化的额外偏好
}

// personalityPath 性格档案在 memorydir 的路径。
func personalityPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "rescene_data", "memory", "personality.md")
}

// distillPersonality 从记忆里蒸馏性格档案，写进 memory/personality.md。
// 幂等：每次全量重蒸馏，不依赖上次结果。
func distillPersonality() {
	if !automaticMemoryEnabled() {
		return
	}

	profile := personalityProfile{
		Extra: make(map[string]string),
	}

	// ── 1) 读自动提取事实 ──
	automaticMemory.Lock()
	facts, err := loadAutomaticFacts()
	if err == nil {
		for _, f := range facts {
			switch strings.ToLower(f.Key) {
			case "tone", "formality":
				profile.Tone = mergeProfileValue(profile.Tone, f.Value)
			case "message_length", "response_length":
				profile.Length = mergeProfileValue(profile.Length, f.Value)
			case "language", "preferred_language":
				profile.Language = mergeProfileValue(profile.Language, f.Value)
			case "emoji_usage", "use_of_emoji":
				profile.Emoji = mergeProfileValue(profile.Emoji, f.Value)
			default:
				if f.Category == "preferences" || f.Category == "profile" {
					profile.Extra[f.Key] = f.Value
				}
			}
		}
	}
	automaticMemory.Unlock()

	// ── 2) 读亲密度 ──
	_, intimacyVal := memorydir.ReadIntimacy()
	profile.IntimacyValue = intimacyVal
	profile.IntimacyLevel = memorydir.IntimacyLevel(intimacyVal)

	// ── 3) 读常驻记忆（pinned 里的身份/称呼/指令优先级最高）──
	pinned := memorydir.ReadPinned()
	if pinned != "" {
		for _, line := range strings.Split(pinned, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// 解析 "- **P01** 内容" 格式
			if idx := strings.Index(line, "**"); idx >= 0 {
				rest := line[idx+2:]
				if end := strings.Index(rest, "**"); end >= 0 {
					rest = strings.TrimSpace(rest[end+2:])
				}
				if strings.Contains(strings.ToLower(rest), "自称") || strings.Contains(strings.ToLower(rest), "名字") {
					profile.SelfName = rest
				}
				if strings.Contains(strings.ToLower(rest), "别") || strings.Contains(strings.ToLower(rest), "不要") || strings.Contains(strings.ToLower(rest), "禁") {
					profile.Taboos = append(profile.Taboos, rest)
				}
			}
		}
	}

	// ── 4) 渲染 personality.md ──
	var b strings.Builder
	b.WriteString("# 性格档案（自动蒸馏，勿手改）\n\n")

	// 核心风格
	b.WriteString("## 核心风格\n")
	if profile.Tone != "" {
		fmt.Fprintf(&b, "- **语气**：%s\n", profile.Tone)
	}
	if profile.Length != "" {
		fmt.Fprintf(&b, "- **回复长度**：%s\n", profile.Length)
	}
	if profile.Language != "" {
		fmt.Fprintf(&b, "- **语言**：%s\n", profile.Language)
	}
	if profile.Emoji != "" {
		fmt.Fprintf(&b, "- **颜文字/emoji**：%s\n", profile.Emoji)
	}
	if profile.SelfName != "" {
		fmt.Fprintf(&b, "- **自称/身份**：%s\n", profile.SelfName)
	}

	// 亲密度响应规则
	b.WriteString("\n## 亲密度响应\n")
	fmt.Fprintf(&b, "当前亲密等级：Lv.%d（亲密值 %d）\n", profile.IntimacyLevel, profile.IntimacyValue)
	b.WriteString("- Lv1-2：保持礼貌、简洁、专业\n")
	b.WriteString("- Lv3-4：更自然、亲切、体贴\n")
	b.WriteString("- Lv5+：主动分享想法、像熟悉的朋友\n")
	b.WriteString("- 自然地融入语气，不要刻意提及等级数字\n")

	// 忌讳
	if len(profile.Taboos) > 0 {
		b.WriteString("\n## 忌讳（用户明确禁止的）\n")
		for _, t := range profile.Taboos {
			fmt.Fprintf(&b, "- %s\n", t)
		}
	}

	// 额外偏好（未归一化的）
	if len(profile.Extra) > 0 {
		b.WriteString("\n## 其他偏好\n")
		keys := make([]string, 0, len(profile.Extra))
		for k := range profile.Extra {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "- **%s**：%s\n", k, profile.Extra[k])
		}
	}

	content := strings.TrimSpace(b.String()) + "\n"

	// 写进 memorydir（白名单外，直接写文件——personality.md 不进云端同步白名单，
	// 但进 memorydir 目录可被手动备份；如需云端同步再加进 SyncableFiles）
	os.MkdirAll(filepath.Dir(personalityPath()), 0o755)
	os.WriteFile(personalityPath(), []byte(content), 0o644)

	// 更新索引行
	idx := memorydir.ReadRaw("index")
	var lines []string
	hasPersonality := false
	for _, line := range strings.Split(idx, "\n") {
		if strings.Contains(line, "[[personality]]") {
			hasPersonality = true
			break
		}
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	personalityLine := "- [[personality]] 性格档案：从记忆蒸馏的语气/长度/忌讳/亲密度响应（自动更新）。"
	if hasPersonality {
		// 替换旧的
		var newLines []string
		for _, line := range strings.Split(idx, "\n") {
			if strings.Contains(line, "[[personality]]") {
				newLines = append(newLines, personalityLine)
			} else if strings.TrimSpace(line) != "" {
				newLines = append(newLines, line)
			}
		}
		lines = newLines
	} else {
		lines = append(lines, personalityLine)
	}
	if len(lines) == 0 {
		lines = []string{"# 记忆索引"}
	}
	memorydir.WriteRaw("index", strings.Join(lines, "\n")+"\n")
}

// mergeProfileValue 合并同一 key 的多个取值，保留最新的非空值。
func mergeProfileValue(old, new string) string {
	new = strings.TrimSpace(new)
	if new == "" {
		return old
	}
	if old == "" {
		return new
	}
	// 都非空：保留更长的（通常更具体）
	if len(new) > len(old) {
		return new
	}
	return old
}

// schedulePersonalityDistill 防抖调度蒸馏（30s 内多次变更只触发一次）。
func schedulePersonalityDistill() {
	personalityPipeline.mu.Lock()
	defer personalityPipeline.mu.Unlock()
	if personalityPipeline.timer != nil {
		personalityPipeline.timer.Stop()
	}
	personalityPipeline.timer = time.AfterFunc(30*time.Second, func() {
		distillPersonality()
	})
}

// TriggerPersonalityOnIntimacyLevelUp 亲密度升级时立即蒸馏（不等 debounce）。
func TriggerPersonalityOnIntimacyLevelUp() {
	go distillPersonality()
}
