package main

// tools_daughter.go — 女儿专属工具（24H 自转工具系统扩充）
// 把已有能力包装成正式工具：真浏览器抓网页/技能库/记忆/作品集。
// task 动作的工具从 13 → 17，更接近 Hermes 的工具广度。

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// daughterToolDefs 女儿专属工具定义（追加到 nativeToolDefs）
func daughterToolDefs() []ToolDefinition {
	return []ToolDefinition{
		nativeTool("browser_fetch", "用真浏览器（Edge headless）抓取网页正文，返回纯文本（最多 3000 字）。", map[string]ToolProperty{
			"url": {Type: "string", Description: "要抓取的网页 URL"},
		}, []string{"url"}),
		nativeTool("skills_list", "列出她的技能库（技能名 + 一句话描述）。", nil, nil),
		nativeTool("read_memory", "读她的长期记忆或日记（memory.md / journal.md 尾部 1500 字）。", map[string]ToolProperty{
			"kind": {Type: "string", Description: "memory（长期记忆）或 journal（日记），默认 memory"},
		}, nil),
		nativeTool("outputs_list", "列出她的作品集（outputs/ 全部产出文件名）。", nil, nil),
	}
}

// callDaughterTool 执行女儿专属工具（由 callNativeTool 分发）
func callDaughterTool(ctx context.Context, name string, args map[string]string) (ToolResult, error) {
	switch name {
	case "browser_fetch":
		url := args["url"]
		if url == "" {
			return ToolResult{}, fmt.Errorf("browser_fetch 需要 url 参数")
		}
		text := edgeFetchText(url)
		if text == "" {
			return ToolResult{}, fmt.Errorf("抓取失败（网络/反爬/Edge 不可用）")
		}
		return ToolResult{Text: runeClip(text, 3000)}, nil

	case "skills_list":
		var lines []string
		for _, s := range loadSkills() {
			lines = append(lines, fmt.Sprintf("%s（%s）", s.Name, s.Description))
		}
		if len(lines) == 0 {
			return ToolResult{Text: "（技能库为空）"}, nil
		}
		return ToolResult{Text: strings.Join(lines, "\n")}, nil

	case "read_memory":
		kind := args["kind"]
		if kind != "journal" {
			kind = "memory"
		}
		data, err := os.ReadFile(filepath.Join(daughterHome(), kind+".md"))
		if err != nil {
			return ToolResult{}, fmt.Errorf("读取 %s.md 失败: %v", kind, err)
		}
		return ToolResult{Text: truncTail(string(data), 1500)}, nil

	case "outputs_list":
		entries, err := os.ReadDir(filepath.Join(daughterHome(), "outputs"))
		if err != nil {
			return ToolResult{}, fmt.Errorf("读取作品集失败: %v", err)
		}
		var names []string
		for _, e := range entries {
			if !e.IsDir() {
				names = append(names, e.Name())
			}
		}
		if len(names) == 0 {
			return ToolResult{Text: "（暂无产出）"}, nil
		}
		return ToolResult{Text: strings.Join(names, "\n")}, nil
	}
	return ToolResult{}, fmt.Errorf("未知女儿工具: %s", name)
}

// unmarshalToolArgsJSON 工具参数解析（callNativeTool 的 argsJSON → map）
func unmarshalToolArgsJSON(argsJSON string) map[string]string {
	args := map[string]string{}
	if argsJSON != "" {
		json.Unmarshal([]byte(argsJSON), &args)
	}
	return args
}
