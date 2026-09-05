package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"backend/internal/ai/core"
	"backend/internal/memorydir"
)

func callNativeMemoryTool(ctx context.Context, name, argsJSON string) (nativeToolResult, error) {
	var args map[string]any
	if err := json.Unmarshal([]byte(defaultJSONObject(argsJSON)), &args); err != nil {
		return nativeToolResult{}, fmt.Errorf("参数解析失败: %w", err)
	}
	// 当前 Agent 身份：决定 scope=private 写到哪、memory_search 查哪几层。
	// 空串（单 Agent 老链路 / 子代理 / 后台任务）时 private 自动退回通用记忆，
	// 行为与改造前一致。
	agentID := agentIDFromCtx(ctx)
	switch name {
	case "memory_search":
		query := stringArg(args, "query")
		if query == "" {
			return nativeToolResult{}, fmt.Errorf("query 不能为空")
		}
		scope := strings.ToLower(strings.TrimSpace(stringArg(args, "scope")))
		var parts []string
		if scope != "private" {
			if hits := memorydir.Search(query); hits != "" {
				parts = append(parts, "【通用记忆（所有 Agent 共享）】\n"+hits)
			}
		}
		if scope != "shared" && agentID != "" {
			if hits := memorydir.AgentSearch(agentID, query); hits != "" {
				parts = append(parts, "【我的私有记忆】\n"+hits)
			}
		}
		if len(parts) == 0 {
			return nativeToolResult{Text: fmt.Sprintf("未找到与 %q 相关的记忆。", query)}, nil
		}
		return nativeToolResult{Text: strings.Join(parts, "\n\n")}, nil
	case "memory_append":
		text := strings.TrimSpace(stringArg(args, "text"))
		if text == "" {
			return nativeToolResult{}, fmt.Errorf("text 不能为空")
		}
		file := memoryFileForCluster(stringArg(args, "cluster"))
		// 摘要：取前 40 字，与 remember 工具一致
		summary := text
		if runes := []rune(text); len(runes) > 40 {
			summary = string(runes[:40]) + "…"
		}
		scope := strings.ToLower(strings.TrimSpace(stringArg(args, "scope")))
		if scope == "private" && agentID != "" {
			if err := memorydir.AgentRemember(agentID, file, summary, text); err != nil {
				return nativeToolResult{}, fmt.Errorf("写入私有记忆失败: %w", err)
			}
			// 私有记忆不进云端同步白名单，也不参与通用性格蒸馏：只属于这个 Agent。
			return nativeToolResult{Text: fmt.Sprintf("已写入我的私有记忆 %s（agents/%s/memory/%s.md）", file, agentID, file)}, nil
		}
		if err := memorydir.Remember(file, summary, text); err != nil {
			return nativeToolResult{}, fmt.Errorf("写入失败: %w", err)
		}
		// 云端记忆同步（可选）：记忆变了，异步推送
		pushMemorySync()
		// 性格蒸馏（防抖 30s）：记忆变更可能影响语气/偏好，异步触发重蒸馏
		schedulePersonalityDistill()
		// 记忆摘要蒸馏（防抖 30s）：事实变了，前端视图要跟着更新
		scheduleSummaryDistill()
		return nativeToolResult{Text: fmt.Sprintf("已写入记忆 %s（memory/%s.md）", file, file)}, nil
	case "memory_pin":
		pid, text := stringArg(args, "pid"), stringArg(args, "text")
		if pid == "" || text == "" {
			return nativeToolResult{}, fmt.Errorf("pid 和 text 不能为空")
		}
		if err := memorydir.Pin(pid, text); err != nil {
			return nativeToolResult{}, fmt.Errorf("写入失败: %w", err)
		}
		// 云端记忆同步（可选）：常驻记忆变了，异步推送
		pushMemorySync()
		return nativeToolResult{Text: fmt.Sprintf("已写入常驻记忆 %s（每轮无条件注入 pinned.md）", pid)}, nil
	case "memory_handoff":
		block := stringArg(args, "block")
		if block == "" {
			return nativeToolResult{}, fmt.Errorf("block 不能为空")
		}
		if err := memorydir.HandoffWrite(block); err != nil {
			return nativeToolResult{}, fmt.Errorf("写入失败: %w", err)
		}
		// 云端记忆同步（可选）：交接工作态变了，异步推送（handoff 不进白名单，但推送无害）
		pushMemorySync()
		return nativeToolResult{Text: "已更新会话交接工作态（handoff.md）"}, nil
	case "memory_delete":
		file := strings.TrimSpace(stringArg(args, "file"))
		if file == "" {
			return nativeToolResult{}, fmt.Errorf("file 不能为空（要删除的记忆文件名，如 preferences）")
		}
		if err := memorydir.DeleteMemory(file); err != nil {
			return nativeToolResult{}, fmt.Errorf("删除失败: %w", err)
		}
		// 云端记忆同步（可选）：记忆删了，异步推一次保持云端一致
		pushMemorySync()
		return nativeToolResult{Text: fmt.Sprintf("已删除记忆 %s（memory/%s.md 已移除，索引已同步）", file, file)}, nil
	case "workdir_read":
		path, err := nativeWorkdirNotePath()
		if err != nil {
			return nativeToolResult{}, err
		}
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			return nativeToolResult{Text: "当前项目尚无 workdir.md"}, nil
		}
		if err != nil {
			return nativeToolResult{}, err
		}
		return nativeToolResult{Text: string(data)}, nil
	case "workdir_write":
		return nativeWriteWorkdir(args, false)
	case "workdir_append":
		return nativeWriteWorkdir(args, true)
	default:
		return nativeToolResult{}, fmt.Errorf("未知记忆工具: %s", name)
	}
}

// session_search 工具：三种调用形态（对应 Hermes session_search 的
// SEARCH / BROWSE / READ 模式）：
//   - query 非空            → 全文搜索所有历史对话，返回匹配片段
//   - query 空（不传）      → 列出最近对话（标题 + 最近几条消息），先看聊了什么
//   - session_id 非空       → 读取指定会话的最近消息内容
var sessionSearchToolDef = core.ToolDefinition{
	Type: "function",
	Function: core.ToolFunctionDetail{
		Name: "session_search",
		Description: "查看历史对话，三种用法：①query 传关键词→搜索所有历史对话找到匹配消息；" +
			"②不传参数（或 query 为空）→直接列出最近对话的标题和最近几条消息，先看最近聊了什么；" +
			"③传 session_id→读取该会话的最近消息内容。当你想不起过去说过什么、做过什么，" +
			"或需要参考之前的讨论结果时调用。",
		Parameters: core.ToolParameters{
			Type: "object",
			Properties: map[string]core.ToolProperty{
				"query":      {Type: "string", Description: "搜索关键词，大小写不敏感；不传则浏览最近对话"},
				"session_id": {Type: "string", Description: "要读取的会话 ID；传了就读该会话的最近消息"},
				"limit":      {Type: "integer", Description: "最大返回条数，默认 10，最大 50"},
			},
		},
	},
}

func callNativeSessionSearch(argsJSON string) (nativeToolResult, error) {
	var args struct {
		Query     string `json:"query"`
		SessionID string `json:"session_id"`
		Limit     int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return nativeToolResult{}, fmt.Errorf("参数解析失败: %w", err)
	}
	if args.Limit <= 0 {
		args.Limit = 10
	}
	if args.Limit > 50 {
		args.Limit = 50
	}

	store := globalSessionStore
	if store == nil {
		return nativeToolResult{Text: "会话存储尚未初始化，无法查看。"}, nil
	}

	// 形态③：读指定会话
	if args.SessionID != "" {
		msgs := store.ReadSession(args.SessionID, args.Limit)
		if len(msgs) == 0 {
			return nativeToolResult{Text: fmt.Sprintf("会话 %s 不存在或没有消息。", args.SessionID)}, nil
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("会话「%s」（%s，共显示 %d 条最近消息）：\n\n",
			msgs[0].Title, args.SessionID, len(msgs)))
		for _, m := range msgs {
			roleLabel := "用户"
			if m.Role == "assistant" {
				roleLabel = "助手"
			}
			ts := m.Timestamp.Format("01-02 15:04")
			b.WriteString(fmt.Sprintf("【%s %s】%s\n\n", roleLabel, ts, m.Content))
		}
		return nativeToolResult{Text: b.String()}, nil
	}

	// 形态②：浏览最近对话
	if strings.TrimSpace(args.Query) == "" {
		items := store.RecentSessions(args.Limit, 3)
		if len(items) == 0 {
			return nativeToolResult{Text: "还没有任何历史对话。"}, nil
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("最近的 %d 个对话（按最后活动排序）：\n\n", len(items)))
		for i, it := range items {
			ts := it.UpdatedAt.Format("01-02 15:04")
			b.WriteString(fmt.Sprintf("【%d】%s（%s，%d 条消息）session_id=%s\n", i+1, it.Title, ts, it.MessageCount, it.SessionID))
			for _, m := range it.Recent {
				roleLabel := "用户"
				if m.Role == "assistant" {
					roleLabel = "助手"
				}
				b.WriteString(fmt.Sprintf("   · %s：%s\n", roleLabel, m.Content))
			}
			b.WriteString("\n")
		}
		b.WriteString("想读某个对话的完整内容，用 session_id 参数再调一次本工具。")
		return nativeToolResult{Text: b.String()}, nil
	}

	// 形态①：关键词搜索
	results := store.SearchSessions(args.Query, args.Limit)
	if len(results) == 0 {
		return nativeToolResult{Text: fmt.Sprintf("未找到与 %q 相关的历史对话。", args.Query)}, nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("在历史对话中找到 %d 条与 %q 相关的消息：\n\n", len(results), args.Query))
	for i, r := range results {
		ts := r.Timestamp.Format("01-02 15:04")
		roleLabel := "用户"
		if r.Role == "assistant" {
			roleLabel = "助手"
		}
		b.WriteString(fmt.Sprintf("【%d】会话：%s（session_id=%s）\n", i+1, r.Title, r.SessionID))
		b.WriteString(fmt.Sprintf("   时间：%s ｜ 角色：%s", ts, roleLabel))
		if r.Model != "" {
			b.WriteString(fmt.Sprintf(" ｜ 模型：%s", r.Model))
		}
		b.WriteString(fmt.Sprintf("\n   内容：%s\n\n", r.Content))
	}
	return nativeToolResult{Text: b.String()}, nil
}

// memoryFileForCluster 把 memory_append 的 cluster 分类映射到 memorydir 文件名。
// 大小写不敏感 + 中文别名；未知名归到 memories.md，避免每个新分类都建一个文件。
func memoryFileForCluster(cluster string) string {
	switch strings.ToLower(strings.TrimSpace(cluster)) {
	case "userbase", "用户", "偏好":
		return "preferences"
	case "codework", "项目", "代码", "工程":
		return "project"
	case "decisions", "决策", "决定":
		return "decisions"
	case "work", "工作", "工作态":
		return "handoff"
	}
	if strings.TrimSpace(cluster) == "" {
		return "memories"
	}
	return "memories"
}

func nativeWorkdirNotePath() (string, error) {
	project := filepath.Base(core.GetProjectRoot())
	if project == "" || project == "." {
		return "", fmt.Errorf("当前工作目录无效")
	}
	return filepath.Join(resceneUserDataDir(), "projects", project, "workdir.md"), nil
}

func nativeWriteWorkdir(args map[string]any, appendMode bool) (nativeToolResult, error) {
	block := strings.TrimSpace(stringArg(args, "block"))
	if block == "" {
		return nativeToolResult{}, fmt.Errorf("block 不能为空")
	}
	path, err := nativeWorkdirNotePath()
	if err != nil {
		return nativeToolResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nativeToolResult{}, err
	}
	if appendMode {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nativeToolResult{}, err
		}
		defer f.Close()
		prefix := ""
		if info, _ := f.Stat(); info != nil && info.Size() > 0 {
			prefix = "\n\n"
		}
		if _, err := f.WriteString(prefix + block + "\n"); err != nil {
			return nativeToolResult{}, err
		}
		return nativeToolResult{Text: "已追加当前项目 workdir.md"}, nil
	}
	if err := atomicWriteNative(path, []byte(block+"\n"), 0o644); err != nil {
		return nativeToolResult{}, err
	}
	return nativeToolResult{Text: "已重写当前项目 workdir.md"}, nil
}
