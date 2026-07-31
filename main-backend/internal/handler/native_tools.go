package handler

// Wails 内置工具层。
//
// 这些工具由 Go 进程直接执行，不依赖用户机器上的 Python、Node、npm 或 npx。
// MCP 仍保留给真正的外部扩展；本机基础能力不再需要绕一层 stdio 子进程。

import (
	"context"
	"fmt"
	"os"
	"strings"

	"backend/internal/ai/core"
)

type nativeToolResult struct {
	Text   string
	Images []mcpImageArtifact
}

func nativeOnDemandToolDefs() []core.ToolDefinition {
	defs := []core.ToolDefinition{
		nativeTool("read_file", "按行读取文本文件，返回带行号的内容；一次最多 400 行。offset 从 1 开始，limit 是行数。", map[string]core.ToolProperty{
			"path":   {Type: "string", Description: "文件路径；相对路径按当前工作目录解析"},
			"offset": {Type: "integer", Description: "起始行号，1-indexed，默认 1"},
			"limit":  {Type: "integer", Description: "读取行数，默认 200，最大 400"},
		}, []string{"path"}),
		nativeTool("grep", "在文件内容中搜索正则表达式，返回 文件:行号:匹配行；默认搜索当前项目，最多 200 条。", map[string]core.ToolProperty{
			"pattern": {Type: "string", Description: "Go 正则表达式"},
			"path":    {Type: "string", Description: "搜索起点，默认当前工作目录"},
			"type":    {Type: "string", Description: "可选文件类型：go/vue/js/ts/py/json/md/css/html"},
		}, []string{"pattern"}),
		nativeTool("glob", "按文件名或相对路径模式查找文件，例如 **/*.go、src/**/*.vue。", map[string]core.ToolProperty{
			"pattern": {Type: "string", Description: "glob 模式，支持 *、? 和 **"},
			"path":    {Type: "string", Description: "搜索起点，默认当前工作目录"},
		}, []string{"pattern"}),
		nativeTool("list_directory", "列出目录中的直接子项，区分文件和目录。", map[string]core.ToolProperty{
			"path": {Type: "string", Description: "目录路径，默认当前工作目录"},
		}, nil),
		nativeTool("directory_tree", "递归列出目录树；默认跳过 .git、node_modules 等大目录并限制返回规模。", map[string]core.ToolProperty{
			"path":  {Type: "string", Description: "目录路径，默认当前工作目录"},
			"depth": {Type: "integer", Description: "最大深度，默认 4，最大 8"},
		}, nil),
		nativeTool("get_file_info", "读取文件或目录的大小、修改时间、类型和权限。", map[string]core.ToolProperty{
			"path": {Type: "string", Description: "文件或目录路径"},
		}, []string{"path"}),
		nativeTool("write_file", "创建或完整覆盖一个文本文件；自动创建父目录。", map[string]core.ToolProperty{
			"path":    {Type: "string", Description: "目标文件路径"},
			"content": {Type: "string", Description: "完整文件内容"},
		}, []string{"path", "content"}),
		nativeTool("edit_file", "在文本文件中做一次定点替换。优先精确匹配；精确失败时允许逐行忽略首尾空白匹配，但拒绝多处歧义。", map[string]core.ToolProperty{
			"path":       {Type: "string", Description: "目标文件路径"},
			"old_string": {Type: "string", Description: "要替换的原始文本，应从 read_file 结果原样复制"},
			"new_string": {Type: "string", Description: "替换后的文本"},
		}, []string{"path", "old_string", "new_string"}),
		nativeTool("create_directory", "递归创建目录；目录已存在时视为成功。", map[string]core.ToolProperty{
			"path": {Type: "string", Description: "目录路径"},
		}, []string{"path"}),
		nativeTool("move_file", "移动或重命名文件/目录；目标已存在时拒绝覆盖。", map[string]core.ToolProperty{
			"source":      {Type: "string", Description: "源路径"},
			"destination": {Type: "string", Description: "目标路径"},
		}, []string{"source", "destination"}),
		nativeTool("delete_file", "删除单个文件。该操作不可逆，任何模式都需要用户批准。", map[string]core.ToolProperty{
			"path": {Type: "string", Description: "文件路径"},
		}, []string{"path"}),
		nativeTool("delete_directory", "递归删除目录。该操作不可逆，任何模式都需要用户批准。", map[string]core.ToolProperty{
			"path": {Type: "string", Description: "目录路径"},
		}, []string{"path"}),
		nativeTool("run_command", "在当前项目目录执行一条系统命令并返回退出码、stdout 和 stderr。", map[string]core.ToolProperty{
			"command": {Type: "string", Description: "要执行的命令"},
			"timeout": {Type: "integer", Description: "超时秒数，默认 120，最大 600"},
		}, []string{"command"}),
		nativeTool("web_fetch", "通过 Go HTTP 客户端抓取网页并提取可读文本，不依赖 Python。", map[string]core.ToolProperty{
			"url":       {Type: "string", Description: "http(s) URL"},
			"max_chars": {Type: "integer", Description: "最大返回字符数，默认 8000，最大 30000"},
		}, []string{"url"}),
		nativeTool("view_image", "分析图片内容。可传本地路径、图片 URL 或 base64；视觉请求直接复用内置 Go 模型路由。", map[string]core.ToolProperty{
			"path":         {Type: "string", Description: "可选，本地图片路径"},
			"image_url":    {Type: "string", Description: "可选，http(s) 图片 URL"},
			"image_base64": {Type: "string", Description: "可选，图片 base64"},
			"question":     {Type: "string", Description: "希望视觉模型回答的问题"},
		}, nil),
		nativeTool("memory_search", "搜索本地长期记忆中的相关事实。", map[string]core.ToolProperty{
			"query": {Type: "string", Description: "检索问题或关键词"},
		}, []string{"query"}),
		nativeTool("memory_append", "向本地长期记忆写入一条可复用事实；SwiftNet 会自动做相似项防重。", map[string]core.ToolProperty{
			"text":     {Type: "string", Description: "要记住的事实"},
			"cluster":  {Type: "string", Description: "分类，如 UserBase/CodeWork/Decisions"},
			"keywords": {Type: "string", Description: "可选同义关键词，用 / 分隔"},
		}, []string{"text"}),
		nativeTool("memory_pin", "写入或更新一条每轮无条件注入的常驻记忆。", map[string]core.ToolProperty{
			"pid":     {Type: "string", Description: "稳定编号，如 P03；同编号会覆盖"},
			"cluster": {Type: "string", Description: "分类标签"},
			"text":    {Type: "string", Description: "常驻内容"},
		}, []string{"pid", "text"}),
		nativeTool("memory_handoff", "重写会话交接工作态，供下一次对话继续未完成任务。", map[string]core.ToolProperty{
			"block": {Type: "string", Description: "当前进度、关键事实和下一步"},
		}, []string{"block"}),
		nativeTool("workdir_read", "读取当前项目独立的 workdir.md 工作笔记。", map[string]core.ToolProperty{}, nil),
		nativeTool("workdir_write", "完整重写当前项目独立的 workdir.md 工作笔记。", map[string]core.ToolProperty{
			"block": {Type: "string", Description: "完整 Markdown 内容"},
		}, []string{"block"}),
		nativeTool("workdir_append", "向当前项目独立的 workdir.md 工作笔记末尾追加内容。", map[string]core.ToolProperty{
			"block": {Type: "string", Description: "要追加的 Markdown 内容"},
		}, []string{"block"}),
	}
	// 搜索服务需要密钥；没有 Key 时连 schema 都不暴露，模型不会选中后才失败。
	if nativeWebSearchEnabled() {
		defs = append(defs, nativeTool("web_search", "联网搜索并返回标题、URL 与摘要；结果可继续交给 web_fetch 读取正文。", map[string]core.ToolProperty{
			"query": {Type: "string", Description: "搜索关键词"},
			"count": {Type: "integer", Description: "返回结果数量，默认 5，最大 10"},
		}, []string{"query"}))
	}
	// Computer Use：桌面操作工具
	defs = append(defs, computerUseToolDefs()...)
	// capture_preview：截「用户正在看的内嵌预览页」发聊天。按需加载而非常驻——
	// 默认不给模型截图能力，避免每轮都截一堆垃圾图；用户明确要看效果时，
	// 模型用 load_tools 激活后照常调用。
	defs = append(defs, capturePreviewToolDef)
	return defs
}

func nativeWebSearchEnabled() bool {
	return strings.TrimSpace(os.Getenv("BING_SEARCH_API_KEY")) != ""
}

func nativeTool(name, description string, properties map[string]core.ToolProperty, required []string) core.ToolDefinition {
	return core.ToolDefinition{
		Type: "function",
		Function: core.ToolFunctionDetail{
			Name:        name,
			Description: description,
			Parameters: core.ToolParameters{
				Type:       "object",
				Properties: properties,
				Required:   required,
			},
		},
	}
}

func isNativeOnDemandTool(name string) bool {
	for _, def := range nativeOnDemandToolDefs() {
		if def.Function.Name == name {
			return true
		}
	}
	return false
}

func isNativeExecutableTool(name string) bool {
	return name == "apply_patch" || isNativeOnDemandTool(name)
}

func allOnDemandToolDefs() []core.ToolDefinition {
	defs := nativeOnDemandToolDefs()
	return append(defs, loadMCPToolDefs()...)
}

func callNativeTool(ctx context.Context, name, argsJSON string) (nativeToolResult, error) {
	switch name {
	case "read_file", "grep", "glob", "list_directory", "directory_tree", "get_file_info",
		"write_file", "edit_file", "apply_patch", "create_directory", "move_file", "delete_file", "delete_directory":
		return callNativeFileTool(name, argsJSON)
	case "run_command":
		return callNativeCommand(ctx, argsJSON)
	case "web_fetch":
		return callNativeWebFetch(ctx, argsJSON)
	case "web_search":
		return callNativeWebSearch(ctx, argsJSON)
	case "view_image":
		return callNativeViewImage(ctx, argsJSON)
	case "memory_search", "memory_append", "memory_pin", "memory_handoff",
		"workdir_read", "workdir_write", "workdir_append":
		return callNativeMemoryTool(name, argsJSON)
	case "computer_screenshot", "computer_mouse_move", "computer_mouse_click",
		"computer_mouse_drag", "computer_type", "computer_key",
		"computer_screen_size", "computer_scroll":
		return callComputerUseTool(ctx, name, argsJSON)
	default:
		return nativeToolResult{}, fmt.Errorf("未知的内置工具: %s", name)
	}
}
