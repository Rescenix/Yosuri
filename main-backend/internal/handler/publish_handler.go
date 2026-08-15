package handler

// publish_handler.go — 多平台一键发布 API（GUI 发布面板后端）
//   GET  /api/publish/platforms — 平台列表
//   POST /api/publish           — {title, content, platforms:["fanqie",...]} 一键发布
// cookie 自动获取：Edge 调试端口（浏览器不关）→ 兜底复制 cookie 库 + headless 读取。
// 发布端点未配置时：打开平台创作页（浏览器已登录），生成发布稿提示粘贴。

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// HandlePublishPlatforms GET /api/publish/platforms
func HandlePublishPlatforms(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"platforms": PubPlatforms})
}

// publishRequest 发布请求
type publishRequest struct {
	Title     string   `json:"title" binding:"required"`
	Content   string   `json:"content" binding:"required"`
	Platforms []string `json:"platforms" binding:"required"`
	Format    string   `json:"format"` // xhs=小红书格式排版（默认空=纯文本）
}

// publishComposeRequest 发布前的 Agent 创作流水线请求。
type publishComposeRequest struct {
	Stage    string `json:"stage" binding:"required"`
	Brief    string `json:"brief"`
	Audience string `json:"audience"`
	Style    string `json:"style"`
	Title    string `json:"title"`
	Content  string `json:"content" binding:"required"`
}

// HandlePublishCompose POST /api/publish/compose —— 构思、提纲、成稿、润色。
func HandlePublishCompose(c *gin.Context) {
	var req publishComposeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	if len([]rune(req.Content)) > 60000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "单次创作内容不能超过 60000 字"})
		return
	}

	context := fmt.Sprintf("故事脑洞/任务：%s\n作品定位：%s\n作者自定义文风与要求：%s\n小说名或章节名：%s\n\n当前故事材料：\n%s",
		strings.TrimSpace(req.Brief), strings.TrimSpace(req.Audience), strings.TrimSpace(req.Style), strings.TrimSpace(req.Title), strings.TrimSpace(req.Content))
	var instruction string
	switch req.Stage {
	case "ideation":
		instruction = "你是网文小说灵感策划师。基于作者材料补齐可持续连载的故事方案：题材与一句话卖点、世界观规则、主角身份与欲望、金手指或核心设定、主要矛盾、关键配角、开篇冲突、长线悬念、3 个候选书名。不得把作者没有指定的文风当成固定要求，不要直接写章节正文。"
	case "outline":
		instruction = "你是网文小说故事结构师。把材料整理成可直接续写的卷纲或章节细纲：主线目标、阶段冲突、人物关系变化、伏笔与回收位置、情绪高低点、每章核心事件和章末钩子。严格遵守已有世界观与人设，不要替作者擅自指定文风。"
	case "draft":
		instruction = "你是网文小说文字创作师。严格根据作者设定、提纲和自定义文风要求，续写一章可发布的中文小说正文。第一行使用 # 章节标题。以场景、人物动作、对话和心理推动情节，保持人设与世界观一致，避免总结腔、说明书腔和套路化 AI 表达；不得擅自改动核心设定。"
	case "polish":
		instruction = "你是网文小说风格编辑师。按作者填写的文风与创作要求校对完整章节：保持人设、视角、称谓、时间线和世界观一致，改善场景感、对话自然度、节奏、转场与章末钩子，删除重复和 AI 套话。不得统一成你自己的文风。返回完整章节，第一行使用 # 章节标题，不要解释修改过程。"
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "未知创作阶段"})
		return
	}

	result, err := callLocalAggregate(instruction + "\n\n" + context)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Agent 创作失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"stage": req.Stage, "content": strings.TrimSpace(result)})
}

// stripMarkdown 去掉常见 markdown 格式标记，保留纯文本
// 图片注释转占位标记【图: 文件名】（发布时对应位置插图），其他标记去除
func stripMarkdown(s string) string {
	// 1. 图片注释 <!-- IMAGE N｜文件：xxx.png｜ALT：... --> → 【图: xxx.png】占位
	//    其他 HTML 注释（非图片）删除
	for strings.Contains(s, "<!--") {
		i := strings.Index(s, "<!--")
		j := strings.Index(s[i:], "-->")
		if j < 0 {
			s = s[:i]
			break
		}
		comment := s[i+4 : i+j]
		if strings.Contains(comment, "IMAGE") || strings.Contains(comment, "图片") || strings.Contains(comment, "图") {
			// 提取「文件：xxx.png」
			fn := ""
			for _, sep := range []string{"文件：", "文件:"} {
				if k := strings.Index(comment, sep); k >= 0 {
					fn = strings.TrimSpace(comment[k+len(sep):]) // 字节偏移（Index 返回字节位置）
					if p := strings.IndexAny(fn, "｜| "); p > 0 {
						fn = fn[:p]
					}
					break
				}
			}
			marker := "【图片】"
			if fn != "" {
				marker = "【图: " + fn + "】"
			}
			s = s[:i] + "\n" + marker + "\n" + s[i+j+3:]
		} else {
			s = s[:i] + s[i+j+3:]
		}
	}
	// 2. 删代码块 ```...```
	for strings.Contains(s, "```") {
		i := strings.Index(s, "```")
		j := strings.Index(s[i+3:], "```")
		if j < 0 {
			s = s[:i] + strings.TrimSpace(s[i+3:])
			break
		}
		s = s[:i] + strings.TrimSpace(s[i+3:i+3+j]) + s[i+3+j+3:]
	}
	// 3. 按行：删分隔线 / 标题符号 / 引用符号
	lines := strings.Split(s, "\n")
	var out []string
	for _, l := range lines {
		t := strings.TrimSpace(l)
		// 分隔线 --- / *** / ___
		if t == "---" || t == "***" || t == "___" || (strings.Trim(t, "-") == "" && len(t) >= 3) {
			continue
		}
		t = strings.TrimLeft(t, "# ")
		t = strings.TrimLeft(t, "> ")
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		out = append(out, t)
	}
	s = strings.Join(out, "\n")
	// 4. 行内标记：[text](url) → text、**bold** → bold、`code` → code、*italic* → italic
	s = fixInlineMarkdown(s)
	return strings.TrimSpace(s)
}

// fixInlineMarkdown 行内 markdown 处理（保留文字，去格式标记）
func fixInlineMarkdown(s string) string {
	// [text](url) → text
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '[' {
			if j := strings.Index(s[i:], "]"); j > 0 {
				// 紧接着是 (url)
				if i+j+1 < len(s) && s[i+j+1] == '(' {
					if end := strings.Index(s[i+j+1:], ")"); end >= 0 {
						b.WriteString(s[i+1 : i+j])
						i = i + j + 1 + end + 1
						continue
					}
				}
			}
		}
		b.WriteByte(s[i])
		i++
	}
	s = b.String()
	// **bold** / ~~strike~~ / `code` 成对标记
	s = strings.NewReplacer("**", "", "`", "", "~~", "").Replace(s)
	// 残余单星号 *italic*（成对消除）
	var b2 strings.Builder
	star := false
	for i := 0; i < len(s); i++ {
		if s[i] == '*' {
			star = !star
			continue
		}
		b2.WriteByte(s[i])
	}
	return b2.String()
}

// HandlePublish POST /api/publish —— 一键发布到多个平台
type publishResult struct {
	Platform string `json:"platform"`
	Name     string `json:"name"`
	OK       bool   `json:"ok"`
	Message  string `json:"message"`
}

// mdToRichText md → 富文本（保留标题/列表/引用/加粗/图片占位，平台富文本编辑器可识别）
// 只清洗脏标记（非图片 HTML 注释、代码块围栏、链接 URL），格式结构全保留
func mdToRichText(s string) string {
	// 1. 图片注释 → 【图: 文件名】占位；其他 HTML 注释删除
	for strings.Contains(s, "<!--") {
		i := strings.Index(s, "<!--")
		j := strings.Index(s[i:], "-->")
		if j < 0 {
			s = s[:i]
			break
		}
		comment := s[i+4 : i+j]
		if strings.Contains(comment, "IMAGE") || strings.Contains(comment, "图片") {
			fn := ""
			for _, sep := range []string{"文件：", "文件:"} {
				if k := strings.Index(comment, sep); k >= 0 {
					fn = strings.TrimSpace(comment[k+len(sep):])
					if p := strings.IndexAny(fn, "｜| "); p > 0 {
						fn = fn[:p]
					}
					break
				}
			}
			marker := "【图片】"
			if fn != "" {
				marker = "【图: " + fn + "】"
			}
			s = s[:i] + "\n" + marker + "\n" + s[i+j+3:]
		} else {
			s = s[:i] + s[i+j+3:]
		}
	}
	// 2. 代码块围栏 ``` 去掉，内容保留
	for strings.Contains(s, "```") {
		i := strings.Index(s, "```")
		j := strings.Index(s[i+3:], "```")
		if j < 0 {
			s = s[:i] + strings.TrimSpace(s[i+3:])
			break
		}
		s = s[:i] + strings.TrimSpace(s[i+3:i+3+j]) + s[i+3+j+3:]
	}
	// 3. 链接 [text](url) → text（富文本超链接文字）
	s = fixLinkText(s)
	// 4. 行内残留：~~删除线~~（**加粗保留，富文本支持）
	s = strings.ReplaceAll(s, "~~", "")
	// 5. 清理多余空行（保留段落结构）
	lines := strings.Split(s, "\n")
	var out []string
	blank := 0
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			blank++
			if blank <= 1 {
				out = append(out, "")
			}
			continue
		}
		blank = 0
		out = append(out, l)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// fixLinkText [text](url) → text（保留链接文字，去掉 URL）
func fixLinkText(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '[' {
			if j := strings.Index(s[i:], "]"); j > 0 && i+j+1 < len(s) && s[i+j+1] == '(' {
				if end := strings.Index(s[i+j+1:], ")"); end >= 0 {
					b.WriteString(s[i+1 : i+j])
					i = i + j + 1 + end + 1
					continue
				}
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// xhsFormat 小红书格式：富文本 + 话题标签
func xhsFormat(title, content string) string {
	title = strings.TrimSpace(title)
	if r := []rune(title); len(r) > 20 {
		title = string(r[:20])
	}
	body := mdToRichText(content)
	return title + "\n\n" + body + "\n\n#AI写作 #AI小说 #人工智能创作 #网文 #ResceneAI"
}

// publishRequest 发布请求
type publishRequest2 struct {
	Title     string   `json:"title" binding:"required"`
	Content   string   `json:"content" binding:"required"`
	Platforms []string `json:"platforms" binding:"required"`
	Format    string   `json:"format"`
}

// HandlePublish POST /api/publish —— 一键发布到多个平台
func HandlePublish(c *gin.Context) {
	var req publishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	runes := len([]rune(req.Content))
	// md 转纯文本（网文平台）；xhs=小红书富文本格式（保留标题/加粗/图片占位）
	plain := stripMarkdown(req.Content)
	if req.Format == "xhs" {
		plain = xhsFormat(req.Title, req.Content) // 用原始 md（保留格式结构）
	}
	results := make([]publishResult, 0, len(req.Platforms))
	for _, key := range req.Platforms {
		p := FindPubPlatform(key)
		if p == nil {
			results = append(results, publishResult{Platform: key, Name: key, OK: false, Message: "未知平台"})
			continue
		}
		msg := ""
		if runes < p.MinLen {
			msg = fmt.Sprintf("需 ≥%d 字（当前 %d）", p.MinLen, runes)
		}
		err := guiPublishOne(*p, req.Title, plain)
		if err != nil {
			results = append(results, publishResult{Platform: p.ID, Name: p.Name, OK: false, Message: err.Error()})
			continue
		}
		results = append(results, publishResult{Platform: p.ID, Name: p.Name, OK: true, Message: msg})
	}
	c.JSON(http.StatusOK, gin.H{"results": results})
}

// guiPublishOne 发布到单平台：通过运行中 Edge CDP 自动发布（复用登录态）
func guiPublishOne(p PubPlatform, title, content string) error {
	// 优先通过 CDP 连运行中 Edge（不启动新浏览器）
	return cdpPublishOne(p, title, content)
}

// HandlePublishLoginEdge POST /api/publish/login-edge —— 检查 Edge 调试端口状态
func HandlePublishLoginEdge(c *gin.Context) {
	exe := edgeExePath()
	if _, err := os.Stat(exe); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "未找到 Edge，请先安装 Microsoft Edge"})
		return
	}
	if edgeBrowserWS() != "" {
		c.JSON(http.StatusOK, gin.H{"ok": true, "message": "Edge 调试端口已连接，可直接发布"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":      false,
		"message": "Edge 调试端口未开。请关闭所有 Edge 窗口，然后运行桌面上的「Edge调试启动.bat」重启 Edge，再登录晋江/番茄",
	})
}

// pubAccountCfg 平台账号配置（publish_config.json）
type pubAccountCfg struct {
	PublishURL string `json:"publish_url"`
	Referer    string `json:"referer"`
}

// loadPubAccount 读配置（~/.rescene_data/publish_config.json）
func loadPubAccount(id string) pubAccountCfg {
	var cfg struct {
		Platforms map[string]pubAccountCfg `json:"platforms"`
	}
	home, _ := os.UserHomeDir()
	if data, err := os.ReadFile(filepath.Join(home, "rescene_data", "publish_config.json")); err == nil {
		jsonUnmarshal(data, &cfg)
	}
	if cfg.Platforms == nil {
		return pubAccountCfg{}
	}
	return cfg.Platforms[id]
}

// writePubDraft 生成发布稿（outputs/publish/）
func writePubDraft(p PubPlatform, title, content string) string {
	home, _ := os.UserHomeDir()
	outDir := filepath.Join(home, "rescene_data", "daughter", "outputs", "publish")
	os.MkdirAll(outDir, 0o755)
	path := filepath.Join(outDir, fmt.Sprintf("发布稿-%s-%s.md", p.ID, time.Now().Format("2006-01-02-1504")))
	os.WriteFile(path, []byte(fmt.Sprintf("# 发布稿 · %s\n\n标题：%s\n\n%s\n", p.Name, title, content)), 0o644)
	return path
}
