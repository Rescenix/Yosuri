package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ReviewFinding 审查员的一条发现。
type ReviewFinding struct {
	Severity string `json:"severity"` // high | medium | low
	Issue    string `json:"issue"`
	Hint     string `json:"hint,omitempty"`
}

// auditReviewReq 改动文件卡片「审查」按钮提交：审的是这一个文件本次的 diff。
type auditReviewReq struct {
	Path string `json:"path"`
	Diff string `json:"diff"`
}

// HandleWorkflowAudit 审查按钮接口：用户主动点才跑，不自动旁路。
// 审查员是**独立的一次 LLM 调用**，换一个「事后接手挑刺的工程师」视角看这一处 diff，
// 专门挖潜在问题——写代码的 agent 自己验收自己就是自指回音室，所以必须独立生成。
// 走免费模型池，不烧用户额度。
func HandleWorkflowAudit(c *gin.Context) {
	var req auditReviewReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	findings := auditReview(req.Path, req.Diff)
	// nil = 调用/解析失败（模型确实没问题时返回的是空数组，不是 nil）。
	// 绝不能把失败伪装成「未发现潜在问题」——那是最坏的假安心。
	if findings == nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "审查未完成，请重试"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"findings": findings})
}

// auditReview 对单个文件的 diff 做一次独立审查调用。
// 返回 nil 表示失败（调用/解析出错）；返回空切片表示审查员确实没挑出问题。
func auditReview(path, diff string) []ReviewFinding {
	if strings.TrimSpace(diff) == "" {
		return []ReviewFinding{}
	}

	prompt := fmt.Sprintf(`你是一名独立代码审查员，不是这段改动的作者。你的职责是挑刺：只看下面这一处 diff 里潜在的问题、遗漏和风险，而不是夸它写得对。作者自己的判断不可信，请独立看 diff 判断。

文件：%s

差异（unified diff）：
%s

只输出发现的问题，按严重程度降序，最多 5 条。重点看（有才写，没有别硬凑）：
1. 未处理的错误路径和边界条件（空值、越界、并发、超时、权限）
2. 逻辑漏洞：改了 A 却漏改依赖 A 的 B，或新旧行为不一致
3. 安全隐患（凭据硬编码、注入、越权、日志泄密）
4. 与函数签名/调用方约定冲突（返回值、类型、可空性变了但调用方没跟上）
5. 被删掉却仍被引用的东西，或明显缺失的配套（测试、注释、错误信息）

每条格式：{"severity":"high|medium|low","issue":"问题一句话，不超过40字","hint":"复查方向，不超过30字，可省略"}。
确实没问题就输出 []。只输出 JSON 数组，不要解释、不要代码块包裹。`,
		path, tailChars(diff, 6000))

	// 免费池逐个试，每个源单独给预算（见 auditChat）。
	content, err := auditChat(freeOnlyBackends(), prompt)
	if err != nil {
		log.Printf("⚠️ 审查调用失败: %v", err)
		return nil
	}

	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var findings []ReviewFinding
	if err := json.Unmarshal([]byte(content), &findings); err != nil {
		log.Printf("⚠️ 审查 JSON 解析失败: %v", err)
		return nil
	}

	seen := map[string]bool{}
	out := make([]ReviewFinding, 0, len(findings))
	for _, f := range findings {
		f.Issue = strings.TrimSpace(f.Issue)
		if f.Issue == "" || seen[f.Issue] || len(out) >= 5 {
			continue
		}
		f.Severity = normalizeSeverity(f.Severity)
		f.Hint = strings.TrimSpace(f.Hint)
		seen[f.Issue] = true
		out = append(out, f)
	}
	return out
}

// auditChat 免费池逐个试的轻量审查调用。
//
// 为什么不复用 routeChatOnce：它的 failover 只看调用方给的总 ctx，而免费源自带
// 45s per-backend 超时——总预算只要不大于单源超时，第一个慢源就能把整条链的预算
// 全部吃光，后面的健康源永远轮不到（09-07 实测：45s 总超时正好撞死在第一个源上）。
// 这里给每个源单独开子 ctx，慢源到点就放弃切下一个，同时留一个总上限兜底。
func auditChat(backends []RouterBackend, prompt string) (string, error) {
	const (
		// 免费池实测单源出结果 ~11-15s，10s 会把健康源也误杀（09-07 实测 4 源全 timeout）。
		perBackend = 25 * time.Second
		overall    = 75 * time.Second
		maxTries   = 4
	)
	deadline, cancelDeadline := context.WithTimeout(context.Background(), overall)
	defer cancelDeadline()

	msgs := []map[string]any{{"role": "user", "content": prompt}}
	tried := 0
	for _, b := range backends {
		if tried >= maxTries || deadline.Err() != nil {
			break
		}
		tried++
		ctx, cancel := context.WithTimeout(deadline, perBackend)
		content, _, err := openAIChatOnce(ctx, b, msgs, nil)
		cancel()
		if err != nil {
			log.Printf("🔍 [审查] %s 不可用，切下一个: %v", b.Name, truncateChars(err.Error(), 100))
			continue
		}
		return content, nil
	}
	return "", fmt.Errorf("免费池无可用源（试了 %d 个）", tried)
}

// freeOnlyBackends 免费模型池路由链：审查这种旁路绝不消耗用户自己的付费额度。
func freeOnlyBackends() []RouterBackend {
	all := resolveBackends("", "auto")
	out := make([]RouterBackend, 0, len(all))
	for _, b := range all {
		if b.Source == "free" {
			out = append(out, b)
		}
	}
	return out
}

// normalizeSeverity 只认 high/medium/low，其它值归 medium。
func normalizeSeverity(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "high", "h", "高", "高危", "严重":
		return "high"
	case "low", "l", "低", "提醒", "轻微":
		return "low"
	default:
		return "medium"
	}
}
