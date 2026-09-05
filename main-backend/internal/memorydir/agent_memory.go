// agent_memory.go —— 每个 Agent 一份私有记忆，与通用记忆并存。
//
// 目录结构：
//
//	~/rescene_data/memory/            ← 通用记忆（所有 Agent 共享，原有那套）
//	~/rescene_data/agents/<id>/memory/ ← 该 Agent 的私有记忆（同构：index.md + 若干 .md）
//
// 设计要点：
//   - 私有记忆与通用记忆用同一套读写语义（Remember / ReadIndex / ReadWithLinks），
//     所以 Agent 学新东西的链路不用重写一遍。
//   - 私有记忆不进云端同步白名单：角色卡是本地资产，跨设备同步的是通用记忆。
//     （以后要同步再单独开一条按 agent 分片的通道，别硬塞进现有白名单。）
//   - agent id 做严格白名单化清洗，防目录穿越。
package memorydir

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// agentIDRe 允许的 agent id 字符集：小写字母/数字/连字符/下划线，1-40 位。
// 中文名在前端映射成拼音或哈希后的 id，落盘只认这个字符集。
var agentIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,39}$`)

// SanitizeAgentID 把任意字符串压成合法 agent id；不合法返回空串。
func SanitizeAgentID(id string) string {
	id = strings.TrimSpace(strings.ToLower(id))
	if agentIDRe.MatchString(id) {
		return id
	}
	return ""
}

// agentsRoot 返回 ~/rescene_data/agents
func agentsRoot() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "rescene_data", "agents")
}

// AgentDir 返回某个 agent 的根目录（含 memory/ 与 avatar）。
func AgentDir(id string) string {
	id = SanitizeAgentID(id)
	if id == "" || agentsRoot() == "" {
		return ""
	}
	return filepath.Join(agentsRoot(), id)
}

// AgentMemoryDir 返回某个 agent 的私有记忆目录。
func AgentMemoryDir(id string) string {
	d := AgentDir(id)
	if d == "" {
		return ""
	}
	return filepath.Join(d, "memory")
}

// AgentAvatarPath 返回某个 agent 的头像文件路径（base64 dataURL 文本，
// 与全局 custom_avatar 同一存法，前端直接当 img src 用）。
func AgentAvatarPath(id string) string {
	d := AgentDir(id)
	if d == "" {
		return ""
	}
	return filepath.Join(d, "avatar")
}

// ListAgentIDs 列出已存在的 agent id（按目录名排序）。
func ListAgentIDs() []string {
	root := agentsRoot()
	if root == "" {
		return nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if SanitizeAgentID(e.Name()) != "" {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

// ── 私有记忆读写：与通用记忆同一套语义，只是落在 agent 目录下 ──

// AgentRemember 往该 agent 的私有记忆里写一条（追加 + 更新它自己的 index.md）。
func AgentRemember(id, file, summary, content string) error {
	dir := AgentMemoryDir(id)
	if dir == "" {
		return fmt.Errorf("agent id 非法: %s", id)
	}
	file = strings.TrimSpace(file)
	if file == "" {
		return fmt.Errorf("记忆文件名不能为空")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	filePath := filepath.Join(dir, file+".md")
	existing := ""
	if data, err := os.ReadFile(filePath); err == nil {
		existing = strings.TrimSpace(string(data)) + "\n\n"
	}
	if err := os.WriteFile(filePath, []byte(existing+content+"\n"), 0o644); err != nil {
		return err
	}
	return agentUpdateIndex(dir, file, summary)
}

// agentUpdateIndex 更新 agent 私有 index.md 里对应 [[file]] 的行。
func agentUpdateIndex(dir, file, summary string) error {
	idxPath := filepath.Join(dir, "index.md")
	data, _ := os.ReadFile(idxPath)
	idxContent := strings.TrimSpace(string(data))
	newLine := fmt.Sprintf("- [[%s]] %s", file, summary)
	if idxContent == "" {
		idxContent = "# 记忆索引\n\n" + newLine + "\n"
		return os.WriteFile(idxPath, []byte(idxContent), 0o644)
	}
	replaced := false
	var lines []string
	for _, line := range strings.Split(idxContent, "\n") {
		if m := linkRe.FindStringSubmatch(line); m != nil && strings.TrimSpace(m[1]) == file {
			lines = append(lines, newLine)
			replaced = true
		} else {
			lines = append(lines, line)
		}
	}
	if !replaced {
		lines = append(lines, newLine)
	}
	return os.WriteFile(idxPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

// AgentReadIndex 读取该 agent 私有 index.md 全文。
func AgentReadIndex(id string) string {
	dir := AgentMemoryDir(id)
	if dir == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(dir, "index.md"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// agentParseIndex 解析 agent 私有 index.md 为结构化行（与 ParseIndex 同规则）。
func agentParseIndex(dir string) []IndexLine {
	data, err := os.ReadFile(filepath.Join(dir, "index.md"))
	if err != nil {
		return nil
	}
	var out []IndexLine
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m := linkRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		file := strings.TrimSpace(m[1])
		summary := strings.TrimSpace(linkRe.ReplaceAllString(line, ""))
		summary = strings.TrimLeft(summary, "- 	")
		out = append(out, IndexLine{File: file, Summary: summary, Raw: line})
	}
	return out
}

// AgentReadWithLinks 按 task 从该 agent 的私有记忆里联想召回（同通用记忆的 bigram 打分）。
// 返回「私有索引 + 命中文件正文」；无命中时只返回索引（可能为空串）。
func AgentReadWithLinks(id, task string, maxFiles int) string {
	dir := AgentMemoryDir(id)
	if dir == "" {
		return ""
	}
	idx := AgentReadIndex(id)
	if strings.TrimSpace(task) == "" || maxFiles <= 0 {
		return idx
	}
	lines := agentParseIndex(dir)
	if len(lines) == 0 {
		return idx
	}
	type scored struct {
		ov   float64
		file string
	}
	var hits []scored
	qToks := Norm(task)
	for _, line := range lines {
		hay := line.Summary + " " + line.File
		ov := Overlap(hay, task)
		if ov <= 0 {
			hayLower := strings.ToLower(hay)
			for _, t := range qToks {
				if strings.Contains(hayLower, t) {
					ov = 0.1
					break
				}
			}
		}
		if ov > 0.15 {
			hits = append(hits, scored{ov, line.File})
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].ov > hits[j].ov })
	var parts []string
	if idx != "" {
		parts = append(parts, "📇 我的私有记忆索引", idx)
	}
	for i, h := range hits {
		if i >= maxFiles {
			break
		}
		data, err := os.ReadFile(filepath.Join(dir, h.file+".md"))
		if err != nil {
			continue
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			continue
		}
		parts = append(parts, "━━━ "+h.file+" ━━━", content)
	}
	return strings.Join(parts, "\n")
}

// AgentSearch 在该 agent 私有记忆全库检索（memory_search 的私有侧）。
func AgentSearch(id, query string) string {
	dir := AgentMemoryDir(id)
	if dir == "" || strings.TrimSpace(query) == "" {
		return ""
	}
	lines := agentParseIndex(dir)
	type scored struct {
		score float64
		file  string
	}
	var hits []scored
	for _, line := range lines {
		if ov := Overlap(line.Summary+" "+line.File, query); ov > 0.15 {
			hits = append(hits, scored{ov, line.File})
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	var parts []string
	for i, h := range hits {
		if i >= 3 {
			break
		}
		data, err := os.ReadFile(filepath.Join(dir, h.file+".md"))
		if err != nil || strings.TrimSpace(string(data)) == "" {
			continue
		}
		parts = append(parts, "━━━ "+h.file+" ━━━", strings.TrimSpace(string(data)))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n\n")
}
