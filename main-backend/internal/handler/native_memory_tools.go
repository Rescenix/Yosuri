package handler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"backend/internal/ai/core"
	"backend/internal/memorydir"
)

func callNativeMemoryTool(name, argsJSON string) (nativeToolResult, error) {
	var args map[string]any
	if err := json.Unmarshal([]byte(defaultJSONObject(argsJSON)), &args); err != nil {
		return nativeToolResult{}, fmt.Errorf("参数解析失败: %w", err)
	}
	switch name {
	case "memory_search":
		query := stringArg(args, "query")
		if query == "" {
			return nativeToolResult{}, fmt.Errorf("query 不能为空")
		}
		hits := memorydir.Search(query)
		if hits == "" {
			return nativeToolResult{Text: fmt.Sprintf("未找到与 %q 相关的记忆。", query)}, nil
		}
		return nativeToolResult{Text: hits}, nil
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
		if err := memorydir.Remember(file, summary, text); err != nil {
			return nativeToolResult{}, fmt.Errorf("写入失败: %w", err)
		}
		return nativeToolResult{Text: fmt.Sprintf("已写入记忆 %s（memory/%s.md）", file, file)}, nil
	case "memory_pin":
		pid, text := stringArg(args, "pid"), stringArg(args, "text")
		if pid == "" || text == "" {
			return nativeToolResult{}, fmt.Errorf("pid 和 text 不能为空")
		}
		if err := memorydir.Pin(pid, text); err != nil {
			return nativeToolResult{}, fmt.Errorf("写入失败: %w", err)
		}
		return nativeToolResult{Text: fmt.Sprintf("已写入常驻记忆 %s（每轮无条件注入 pinned.md）", pid)}, nil
	case "memory_handoff":
		block := stringArg(args, "block")
		if block == "" {
			return nativeToolResult{}, fmt.Errorf("block 不能为空")
		}
		if err := memorydir.HandoffWrite(block); err != nil {
			return nativeToolResult{}, fmt.Errorf("写入失败: %w", err)
		}
		return nativeToolResult{Text: "已更新会话交接工作态（handoff.md）"}, nil
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
