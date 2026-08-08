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
	"os/exec"
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

// HandlePublish POST /api/publish —— 一键发布到多个平台
func HandlePublish(c *gin.Context) {
	var req publishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	runes := len([]rune(req.Content))
	// md 转纯文本（发布到网文平台是纯文本格式）
	plain := stripMarkdown(req.Content)
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

// guiPublishOne 发布到单平台：优先无头 Chrome 自动发布（登录态在发布专用 profile）
func guiPublishOne(p PubPlatform, title, content string) error {
	cfg := loadPubAccount(p.ID)
	if cfg.PublishURL == "" {
		return headlessChromePublish(p, title, content)
	}
	// 端点模式：cookie + HTTP POST
	cookie, err := edgeCookieDomain(p.Domain)
	if err != nil {
		return err
	}
	if cookie == "" {
		return fmt.Errorf("未找到 %s 登录态", p.Name)
	}
	return guiPostArticle(cfg, cookie, title, content)
}

// HandlePublishLoginChrome POST /api/publish/login-chrome —— 打开发布专用 Chrome 登录
func HandlePublishLoginChrome(c *gin.Context) {
	exe := chromeExePath()
	if _, err := os.Stat(exe); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "未找到 Chrome，请先安装 https://www.google.com/chrome/"})
		return
	}
	os.MkdirAll(chromeProfileDir(), 0o755)
	cmd := exec.Command(exe, "--user-data-dir="+chromeProfileDir())
	cmd.Start()
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "发布专用 Chrome 已打开，请登录要发布的平台后关闭窗口"})
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
