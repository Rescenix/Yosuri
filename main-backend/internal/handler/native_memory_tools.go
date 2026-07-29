package handler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"backend/internal/ai/core"
	"backend/internal/swiftnet"
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
		hits := swiftnet.Default().Select(query, 1200, 0)
		if len(hits) == 0 {
			return nativeToolResult{Text: fmt.Sprintf("未找到与 %q 相关的记忆。", query)}, nil
		}
		lines := make([]string, 0, len(hits))
		for _, hit := range hits {
			lines = append(lines, fmt.Sprintf("%s|%s|%s|%s", hit.ID, hit.Cluster, hit.Keywords, hit.Text))
		}
		return nativeToolResult{Text: strings.Join(lines, "\n")}, nil
	case "memory_append":
		res := swiftnet.Default().MemAppend(stringArg(args, "text"), stringArg(args, "cluster"), stringArg(args, "keywords"))
		if res.Err != "" {
			return nativeToolResult{}, fmt.Errorf("%s", res.Err)
		}
		if res.MergedID != "" {
			return nativeToolResult{Text: "已合并到已有记忆 " + res.MergedID}, nil
		}
		return nativeToolResult{Text: "已写入记忆 " + res.ID}, nil
	case "memory_pin":
		pid, text := stringArg(args, "pid"), stringArg(args, "text")
		if pid == "" || text == "" {
			return nativeToolResult{}, fmt.Errorf("pid 和 text 不能为空")
		}
		swiftnet.Default().Pin(pid, stringArg(args, "cluster"), text)
		return nativeToolResult{Text: "已写入常驻记忆 " + pid}, nil
	case "memory_handoff":
		block := stringArg(args, "block")
		if block == "" {
			return nativeToolResult{}, fmt.Errorf("block 不能为空")
		}
		swiftnet.Default().HandoffWrite(block)
		return nativeToolResult{Text: "已更新会话交接工作态"}, nil
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
