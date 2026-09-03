package handler

// Wails 内置工具层。
//
// 这些工具由 Go 进程直接执行，不依赖用户机器上的 Python、Node、npm 或 npx。
// MCP 仍保留给真正的外部扩展；本机基础能力不再需要绕一层 stdio 子进程。

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"backend/internal/ai/core"
)

type nativeToolResult struct {
	Text   string
	Images []mcpImageArtifact
	Videos []mcpVideoArtifact
	// Files 是 Agent 落盘、可作为产物交付的文件（md/pdf/pptx/docx/xlsx 等）。
	// 由 write/patch/bash 等写文件的工具填充；执行层把它们转成 artifact(kind:file)。
	Files []fileDeliverable
	// URLs 是 web_search（Firecrawl 联网搜索）结果的引用来源，透出给前端来源卡片
	URLs []string
}

func nativeOnDemandToolDefs() []core.ToolDefinition {
	defs := []core.ToolDefinition{
		// ── 核心工具（Pi 风格最小集：read / write / patch / bash）──
		// 文件/命令/检索的日常操作全部收敛到这 4 个；旧的文件系统工具
		// （read_file/grep/glob/run_command/task_* 等）被吸收为 legacy，
		// 执行层仍兼容，但不再出现在模型可见的工具索引里。
		nativeTool("read", "读取与检索：读取文件内容、在文件内容中搜索正则(pattern)、按文件名匹配(glob)、列目录或查看文件/目录信息。给了 pattern 做内容搜索，给了 glob 做文件名搜索，path 指向目录且无 pattern/glob 时列目录，info=true 只看元信息。", map[string]core.ToolProperty{
			"path":    {Type: "string", Description: "文件或目录路径；相对路径按当前工作目录解析"},
			"offset":  {Type: "integer", Description: "读文件起始行号，1-indexed，默认 1"},
			"limit":   {Type: "integer", Description: "读文件行数，默认 200，最大 400"},
			"pattern": {Type: "string", Description: "可选：在文件内容中搜索的正则表达式"},
			"glob":    {Type: "string", Description: "可选：按文件名/路径匹配，如 **/*.go"},
			"depth":   {Type: "integer", Description: "可选：path 为目录时递归列目录的深度，默认 4，最大 8"},
			"info":    {Type: "boolean", Description: "可选：只返回文件/目录的大小、修改时间、类型"},
		}, []string{"path"}),
		nativeTool("write", "写文件系统：默认写入完整文件内容（自动创建父目录）；action=create_dir 建目录；action=move 移动/重命名（需 source）；action=delete 删除文件或目录（不可逆）。", map[string]core.ToolProperty{
			"path":    {Type: "string", Description: "目标路径"},
			"content": {Type: "string", Description: "文件完整内容（action=write 时必填）"},
			"action":  {Type: "string", Description: "操作类型：write(默认)/create_dir/move/delete"},
			"source":  {Type: "string", Description: "action=move 时的源路径"},
		}, []string{"path"}),
		nativeTool("patch", "在文本文件中做一次定点替换；old_string 应从 read 结果原样复制（含缩进/空白/换行）。", map[string]core.ToolProperty{
			"path":       {Type: "string", Description: "目标文件路径"},
			"old_string": {Type: "string", Description: "要替换的原始文本"},
			"new_string": {Type: "string", Description: "替换后的文本"},
		}, []string{"path", "old_string", "new_string"}),
		nativeTool("bash", "执行系统命令并返回退出码、stdout、stderr；默认前台等待完成。background=true 后台执行并立即返回 task_id，之后用 action=status/log/wait/kill + task_id 管理。", map[string]core.ToolProperty{
			"command":    {Type: "string", Description: "要执行的命令"},
			"timeout":    {Type: "integer", Description: "前台超时秒数，默认 120，最大 600"},
			"background": {Type: "boolean", Description: "后台执行（默认 false）"},
			"action":     {Type: "string", Description: "后台任务管理：status/log/wait/kill"},
			"task_id":    {Type: "string", Description: "后台任务 ID"},
		}, []string{"command"}),
		// ── 扩展工具（仍按需暴露，非核心）──
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
		nativeTool("run_task", "在后台启动一条命令并立即返回 task_id，不阻塞当前工作流。进程退出时会自动通知你（工作流被唤醒继续处理）。适合长耗时任务（构建/下载/批量脚本）。配套 task_status / task_log / task_wait / task_kill 管理。", map[string]core.ToolProperty{
			"command": {Type: "string", Description: "要在后台执行的命令"},
		}, []string{"command"}),
		nativeTool("task_status", "查询后台任务的运行状态和输出预览。", map[string]core.ToolProperty{
			"task_id": {Type: "string", Description: "run_task 返回的 task_id"},
		}, []string{"task_id"}),
		nativeTool("task_log", "按行分页读取后台任务的完整输出。", map[string]core.ToolProperty{
			"task_id": {Type: "string", Description: "run_task 返回的 task_id"},
			"offset":  {Type: "integer", Description: "起始行号，0 表示从最早开始"},
			"limit":   {Type: "integer", Description: "返回行数，默认 200"},
		}, []string{"task_id"}),
		nativeTool("task_wait", "阻塞等待后台任务完成并返回退出码和输出尾部；超时返回 timeout。需要立即拿到结果时用。", map[string]core.ToolProperty{
			"task_id": {Type: "string", Description: "run_task 返回的 task_id"},
			"timeout": {Type: "integer", Description: "等待秒数，默认 180，最大 600"},
		}, []string{"task_id"}),
		nativeTool("task_kill", "终止后台任务（树杀，含子进程）。", map[string]core.ToolProperty{
			"task_id": {Type: "string", Description: "run_task 返回的 task_id"},
		}, []string{"task_id"}),
		nativeTool("web_fetch", "通过 Go HTTP 客户端抓取网页并提取可读文本，不依赖 Python。", map[string]core.ToolProperty{
			"url":       {Type: "string", Description: "http(s) URL"},
			"max_chars": {Type: "integer", Description: "最大返回字符数，默认 8000，最大 30000"},
		}, []string{"url"}),
		nativeTool("view_image", "分析图片内容（主动识图）。用户贴图、截图（capture_preview / computer_screenshot）或预览页出现图片后，需要了解图中内容时主动调用本工具。可传本地路径、图片 URL 或 base64；视觉请求直接复用内置 Go 模型路由。", map[string]core.ToolProperty{
			"path":         {Type: "string", Description: "可选，本地图片路径"},
			"image_url":    {Type: "string", Description: "可选，http(s) 图片 URL"},
			"image_base64": {Type: "string", Description: "可选，图片 base64"},
			"question":     {Type: "string", Description: "希望视觉模型回答的问题"},
		}, nil),
		nativeTool("image_generate", "生成一张图片（免费、无需 API key，Go 侧直连，不依赖 MCP）。prompt 用英文描述画面：主体、动作、场景、光线、画风。出图后直接显示在对话里，同时落盘本地。", map[string]core.ToolProperty{
			"prompt":   {Type: "string", Description: "英文画面描述，越具体越好；不要要求画面里出现文字"},
			"negative": {Type: "string", Description: "可选，不希望出现的元素"},
			"width":    {Type: "integer", Description: "宽度，默认 1024，范围 256-1536"},
			"height":   {Type: "integer", Description: "高度，默认 1024，范围 256-1536"},
			"seed":     {Type: "integer", Description: "可选随机种子；同 seed + 同 prompt 出图稳定，用于保持角色一致"},
		}, []string{"prompt"}),
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
		nativeTool("memory_delete", "删除一条记忆（file 为记忆文件名，如 preferences/projects/decisions）：移除 memory/<file>.md 并同步更新记忆索引。用于忘掉错误、过期或已被用户否定的记忆。", map[string]core.ToolProperty{
			"file": {Type: "string", Description: "要删除的记忆文件名（不含 .md，如 preferences）"},
		}, []string{"file"}),
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
	// Computer Use：桌面操作工具
	defs = append(defs, computerUseToolDefs()...)
	// capture_preview：截「用户正在看的内嵌预览页」发聊天。按需加载而非常驻——
	// 默认不给模型截图能力，避免每轮都截一堆垃圾图；用户明确要看效果时，
	// 模型用 load_tools 激活后照常调用。
	defs = append(defs, capturePreviewToolDef)
	// arxiv_search：arXiv 论文检索/预览（alphaXiv 风格），Go 直连 API 免外部依赖
	defs = append(defs, arxivToolDef)
	// knowledge_search / knowledge_list：外挂知识库 RAG 检索
	defs = append(defs, knowledgeSearchToolDef, knowledgeListToolDef)
	// generate_office：纯 Go 原生生成 docx/pptx/xlsx/pdf（零 Python 依赖，开箱即用）
	defs = append(defs, generateOfficeToolDef)
	// mambo_video：曼波视频一键生成（配音+字幕+素材匹配+ffmpeg 合成）
	defs = append(defs, mamboToolDef)
	// video_watermark_remove：AI 视频去水印（ffmpeg delogo + 清元数据）
	defs = append(defs, watermarkToolDef)
	// video_generate：AI 生视频（Agnes 免费 API，$0/秒）
	defs = append(defs, videoGenToolDef)
	// 原常驻工具简化为按需加载（2026-08-29 收敛）：update_todo/skill_view/skill_manage/
	// harness_status/open_preview/inject_preview/remember/web_search/session_search
	// 参数简单，模型一眼就知道怎么调，不需要常驻完整 schema 占 3k+ token。
	// 常驻只保留 dispatch_agent/apply_patch/load_tools/ask_user 四个复杂/交互控制面。
	defs = append(defs, updateTodoToolDef, skillViewToolDef, skillManageToolDef,
		harnessStatusToolDef, openPreviewToolDef, injectPreviewToolDef,
		rememberToolDef, webSearchToolDef, sessionSearchToolDef)
	return defs
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
	return name == "apply_patch" || name == "web_search" || name == "session_search" || isNativeOnDemandTool(name)
}

func allOnDemandToolDefs() []core.ToolDefinition {
	defs := nativeOnDemandToolDefs()
	return append(defs, loadMCPToolDefs()...)
}

func callNativeTool(ctx context.Context, name, argsJSON string) (nativeToolResult, error) {
	switch name {
	case "read":
		return callNativeReadTool(argsJSON)
	case "write":
		return callNativeWriteTool(argsJSON)
	case "patch":
		return callNativePatchTool(argsJSON)
	case "bash":
		return callNativeBashTool(ctx, argsJSON)
	case "read_file", "grep", "glob", "list_directory", "directory_tree", "get_file_info",
		"write_file", "edit_file", "apply_patch", "create_directory", "move_file", "delete_file", "delete_directory":
		return callNativeFileTool(name, argsJSON)
	case "run_command":
		return callNativeCommand(ctx, argsJSON)
	case "run_task", "task_status", "task_log", "task_wait", "task_kill":
		return callBgTaskTool(ctx, name, argsJSON, workflowIDFromCtx(ctx))
	case "web_fetch":
		return callNativeWebFetch(ctx, argsJSON)
	case "arxiv_search":
		text, err := callArxivSearch(ctx, argsJSON)
		if err != nil {
			return nativeToolResult{}, err
		}
		return nativeToolResult{Text: text}, nil
	case "mambo_video":
		return callMamboVideo(ctx, argsJSON)
	case "video_watermark_remove":
		return callWatermarkRemove(ctx, argsJSON)
	case "web_search":
		return callFirecrawlSearch(ctx, argsJSON)
	case "view_image":
		return callNativeViewImage(ctx, argsJSON)
	case "image_generate":
		return callNativeImageGenerate(ctx, argsJSON)
	case "video_generate":
		return callNativeVideoGenerate(ctx, argsJSON)
	case "memory_search", "memory_append", "memory_pin", "memory_handoff",
		"workdir_read", "workdir_write", "workdir_append":
		return callNativeMemoryTool(name, argsJSON)
	case "knowledge_search", "knowledge_list":
		return callNativeKnowledgeTool(name, argsJSON)
	case "generate_office":
		return callGenerateOffice(argsJSON)
	case "session_search":
		return callNativeSessionSearch(argsJSON)
	case "computer_screenshot", "computer_mouse_move", "computer_mouse_click",
		"computer_mouse_drag", "computer_type", "computer_key",
		"computer_screen_size", "computer_scroll":
		return callComputerUseTool(ctx, name, argsJSON)
	default:
		return nativeToolResult{}, fmt.Errorf("未知的内置工具: %s", name)
	}
}

// legacyFileToolSet 被 read/write/patch/bash 四个核心工具吸收的旧文件/命令/检索工具。
// 它们仍可被调用（执行层 / 续跑检查点 / 内部引用兼容），但不再出现在模型可见的
// 工具索引里——模型只认识 read/write/patch/bash 这四个核心。
var legacyFileToolSet = map[string]bool{
	"read_file": true, "grep": true, "glob": true,
	"list_directory": true, "directory_tree": true, "get_file_info": true,
	"write_file": true, "edit_file": true, "apply_patch": true,
	"create_directory": true, "move_file": true, "delete_file": true, "delete_directory": true,
	"run_command": true,
	"run_task":    true, "task_status": true, "task_log": true, "task_wait": true, "task_kill": true,
}

// coreToolIndexDefs 返回模型可见的工具定义：4 个核心工具 + 未并入核心的扩展工具。
// legacy 文件/命令类工具被过滤掉——索引里只有 read/write/patch/bash 四个核心，
// 其余是搜索/视觉/记忆/视频/MCP 等扩展。
func coreToolIndexDefs() []core.ToolDefinition {
	defs := nativeOnDemandToolDefs()
	out := make([]core.ToolDefinition, 0, len(defs))
	for _, d := range defs {
		if !legacyFileToolSet[d.Function.Name] {
			out = append(out, d)
		}
	}
	return out
}

// isCoreTool 判断名字是否是 read/write/patch/bash 之一。
func isCoreTool(name string) bool {
	return name == "read" || name == "write" || name == "patch" || name == "bash"
}

// callNativeReadTool 把 read 的多种模式分发到底层文件/检索实现：
// pattern → grep（内容搜索）；glob → glob（文件名匹配）；info → get_file_info；
// path 指向目录 → list_directory / directory_tree；否则 → read_file。
func callNativeReadTool(argsJSON string) (nativeToolResult, error) {
	var a struct {
		Path    string `json:"path"`
		Offset  int    `json:"offset"`
		Limit   int    `json:"limit"`
		Pattern string `json:"pattern"`
		Glob    string `json:"glob"`
		Depth   int    `json:"depth"`
		Info    bool   `json:"info"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil || strings.TrimSpace(a.Path) == "" {
		return nativeToolResult{}, fmt.Errorf("read 需要 path 参数")
	}
	switch {
	case strings.TrimSpace(a.Pattern) != "":
		return callNativeFileTool("grep", jsonMap("pattern", a.Pattern, "path", a.Path))
	case strings.TrimSpace(a.Glob) != "":
		return callNativeFileTool("glob", jsonMap("pattern", a.Glob, "path", a.Path))
	case a.Info:
		return callNativeFileTool("get_file_info", jsonMap("path", a.Path))
	}
	if info, err := os.Stat(absAgainstRoot(a.Path)); err == nil && info.IsDir() {
		if a.Depth > 0 {
			return callNativeFileTool("directory_tree", jsonMap("path", a.Path, "depth", a.Depth))
		}
		return callNativeFileTool("list_directory", jsonMap("path", a.Path))
	}
	args := map[string]any{"path": a.Path}
	if a.Offset > 0 {
		args["offset"] = a.Offset
	}
	if a.Limit > 0 {
		args["limit"] = a.Limit
	}
	return callNativeFileTool("read_file", jsonMapFrom(args))
}

// callNativeWriteTool 把 write 的 action 分发到底层文件操作：
// create_dir → create_directory；move → move_file；delete → delete_file/delete_directory；
// 默认 write → write_file。
func callNativeWriteTool(argsJSON string) (nativeToolResult, error) {
	var a struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		Action  string `json:"action"`
		Source  string `json:"source"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil || strings.TrimSpace(a.Path) == "" {
		return nativeToolResult{}, fmt.Errorf("write 需要 path 参数")
	}
	switch strings.ToLower(strings.TrimSpace(a.Action)) {
	case "create_dir":
		return callNativeFileTool("create_directory", jsonMap("path", a.Path))
	case "move":
		if strings.TrimSpace(a.Source) == "" {
			return nativeToolResult{}, fmt.Errorf("write action=move 需要 source 参数")
		}
		return callNativeFileTool("move_file", jsonMap("source", a.Source, "destination", a.Path))
	case "delete":
		if info, err := os.Stat(absAgainstRoot(a.Path)); err == nil && info.IsDir() {
			return callNativeFileTool("delete_directory", jsonMap("path", a.Path))
		}
		return callNativeFileTool("delete_file", jsonMap("path", a.Path))
	default:
		return callNativeFileTool("write_file", jsonMap("path", a.Path, "content", a.Content))
	}
}

// callNativePatchTool 把 patch 分发到底层 edit_file（定点替换）。
func callNativePatchTool(argsJSON string) (nativeToolResult, error) {
	var a struct {
		Path      string `json:"path"`
		OldString string `json:"old_string"`
		NewString string `json:"new_string"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil || a.Path == "" || a.OldString == "" {
		return nativeToolResult{}, fmt.Errorf("patch 需要 path 和 old_string 参数")
	}
	return callNativeFileTool("edit_file", jsonMap("path", a.Path, "old_string", a.OldString, "new_string", a.NewString))
}

// callNativeBashTool 把 bash 分发到命令/后台任务实现：
// action=status/log/wait/kill → 后台任务管理；background=true → run_task；否则 → run_command。
func callNativeBashTool(ctx context.Context, argsJSON string) (nativeToolResult, error) {
	var a struct {
		Command    string `json:"command"`
		Timeout    int    `json:"timeout"`
		Background bool   `json:"background"`
		Action     string `json:"action"`
		TaskID     string `json:"task_id"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return nativeToolResult{}, fmt.Errorf("bash 参数解析失败: %v", err)
	}
	if a.Action != "" {
		taskName := ""
		switch strings.ToLower(strings.TrimSpace(a.Action)) {
		case "status":
			taskName = "task_status"
		case "log":
			taskName = "task_log"
		case "wait":
			taskName = "task_wait"
		case "kill":
			taskName = "task_kill"
		default:
			return nativeToolResult{}, fmt.Errorf("bash action 只支持 status/log/wait/kill，收到 %q", a.Action)
		}
		if a.TaskID == "" {
			return nativeToolResult{}, fmt.Errorf("bash action=%s 需要 task_id", a.Action)
		}
		return callBgTaskTool(ctx, taskName, jsonMap("task_id", a.TaskID), workflowIDFromCtx(ctx))
	}
	if a.Background {
		if strings.TrimSpace(a.Command) == "" {
			return nativeToolResult{}, fmt.Errorf("bash 需要 command 参数")
		}
		return callBgTaskTool(ctx, "run_task", jsonMap("command", a.Command), workflowIDFromCtx(ctx))
	}
	if strings.TrimSpace(a.Command) == "" {
		return nativeToolResult{}, fmt.Errorf("bash 需要 command 参数")
	}
	args := map[string]any{"command": a.Command}
	if a.Timeout > 0 {
		args["timeout"] = a.Timeout
	}
	return callNativeCommand(ctx, jsonMapFrom(args))
}

// jsonMap 快速构造扁平 JSON 参数字符串（值为 string 或 int）。
func jsonMap(pairs ...any) string {
	m := map[string]any{}
	for i := 0; i+1 < len(pairs); i += 2 {
		m[pairs[i].(string)] = pairs[i+1]
	}
	return jsonMapFrom(m)
}

func jsonMapFrom(m map[string]any) string {
	b, _ := json.Marshal(m)
	return string(b)
}
