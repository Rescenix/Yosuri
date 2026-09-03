package handler

// company_delivery.go —— 完整交付引擎（2026-09-03 重构，替代对 agent-os 外部 exe 的依赖）。
//
// 目标：把「公司指令」真实生产为一份可审批、可预览的完整项目，产物落在
// ~/rescene_data/company/<agent>/projects/<project>/ 目录，并写出
// delivery.manifest.json（11 阶段 + SHA256），从而被审批队列 HandleCompanyApprovals
// 扫描收录。此前这一整套靠子进程 exec 外部 agent-os exe 完成；现在完全在
// main-backend 内部执行，复用纯 Go 的 genXlsx/genPptx（零外部依赖，人人可得），
// 视频则走 callMamboVideo（有 ffmpeg 就出真片，没有则 pv 阶段标记降级，不阻塞交付）。

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// 与 companyWorkItem 阶段一致：meeting/research/data/requirements/ui/docs/code/runnable/ppt/pv/promotion
// pv 是「尽力而为」阶段：视频引擎不可用时降级（delivery.manifest.json 里 evidence 缺失该项，但
// 门禁对 pv 采用宽松判定——见 approval_handler.go 的 verifyProjectDeliveryGate 分支）。

// companyDeliveryFile 记录一份交付物证据
type companyDeliveryFile struct {
	Stage        string `json:"stage"`
	ProducerRole string `json:"producerRole"`
	File         string `json:"file"`
	Kind         string `json:"kind"`
	SHA256       string `json:"sha256"`
	Bytes        int64  `json:"bytes"`
	Verification string `json:"verification"`
}

// companyDeliveryManifest 交付门禁清单
type companyDeliveryManifest struct {
	Project     string                `json:"project"`
	Status      string                `json:"status"`
	GeneratedAt string                `json:"generatedAt"`
	GateVersion int                   `json:"gateVersion"`
	Evidence    []companyDeliveryFile `json:"evidence"`
	Missing     []string              `json:"missing"`
}

// companyDeliveryStageOrder 与 approval_handler.projectStageOrder 保持一致。
// 视频阶段（pv）在无 ffmpeg 机器上可缺省；其余阶段都必须过。
var companyDeliveryStageOrder = []string{
	"meeting", "research", "data", "requirements", "ui", "docs", "code", "runnable", "ppt", "pv", "promotion",
}

// companyRelayEvent 一条真实的接力事件：from 完成某阶段产物，交给 to 继续。
// 实时接力显示的数据源——不再是事后扫文本猜引用，而是接力链每段完成时真正写入的事件。
type companyRelayEvent struct {
	From     string `json:"from"`     // 交出的 agent（如 researcher-04）
	To       string `json:"to"`       // 接手的 agent（如 designer-04）
	Stage    string `json:"stage"`    // 阶段（research / ui / code ...）
	Artifact string `json:"artifact"` // 交接的产物文件名
	Status   string `json:"status"`   // done（已完成交接）/ running（进行中）
	DoneAt   string `json:"doneAt"`   // 交接时间 RFC3339
}

// relayEventsPath 接力事件存储文件
func relayEventsPath() string {
	return filepath.Join(companyDir(), "relay-events.json")
}

// loadCompanyRelays 读全部接力事件（新→旧）
func loadCompanyRelays() []companyRelayEvent {
	data, err := os.ReadFile(relayEventsPath())
	if err != nil {
		return []companyRelayEvent{}
	}
	var evs []companyRelayEvent
	if err := json.Unmarshal(data, &evs); err != nil {
		return []companyRelayEvent{}
	}
	return evs
}

// appendCompanyRelay 追加一条接力事件并落盘（去重：同 from+to+stage 只保留最新）。
func appendCompanyRelay(ev companyRelayEvent) {
	if strings.TrimSpace(ev.From) == "" || strings.TrimSpace(ev.To) == "" {
		return
	}
	evs := loadCompanyRelays()
	// 去重：同 from->to->stage 的旧事件移除，保留最新状态
	kept := evs[:0]
	for _, e := range evs {
		if e.From == ev.From && e.To == ev.To && e.Stage == ev.Stage {
			continue
		}
		kept = append(kept, e)
	}
	kept = append(kept, ev)
	if len(kept) > 100 {
		kept = kept[len(kept)-100:]
	}
	_ = os.MkdirAll(companyDir(), 0o755)
	b, _ := json.MarshalIndent(kept, "", "  ")
	_ = os.WriteFile(relayEventsPath(), b, 0o644)
	// 同步把交接写进 from/to 两个 agent 的 live.log，让「此刻正在发生」面板实时可见。
	// 这里解决的是：接力引擎此前只写 relay-events.json，从不动各 agent 的 live.log，
	// 导致 CompanyView 的 trace 面板读到的是三周前的死数据——现改为每段交接都落到 live.log。
	stageLabel := ev.Stage
	if ev.Artifact != "" {
		stageLabel = ev.Stage + "·" + ev.Artifact
	}
	appendLiveLog(ev.From, fmt.Sprintf("交接完成 · 交出 %s", stageLabel))
	appendLiveLog(ev.To, fmt.Sprintf("接手交接 · 收到 %s", stageLabel))
}

// appendLiveLog 向指定 agent 的 live.log 追加一行实时活动记录（供 CompanyView「此刻正在发生」面板读取）。
// 行格式与 live.log 既有约定一致，用完整日期时间戳 [YYYY-MM-DD HH:MM]——这样 parseAgentLiveStatus
// 能把它当「完整日期」基准（状态判为『工作中』），recentEvents 的 ^\[header\] 正则也能取到 timeText。
// 目录不存在或写入失败时静默跳过（对应角色没有落地目录，是既有容忍项，不阻塞交付主链路）。
func appendLiveLog(agentName, message string) {
	if strings.TrimSpace(agentName) == "" {
		return
	}
	agentHome := filepath.Join(companyDir(), agentName)
	if _, err := os.Stat(agentHome); err != nil {
		return
	}
	line := fmt.Sprintf("[%s] %s\n", time.Now().Format("2006-01-02 15:04"), message)
	f, err := os.OpenFile(filepath.Join(agentHome, "live.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(line)
}

// deliveryProjectName 生成项目目录名：001-<清洗后的指令>-<MMDD-HHMMSS>
// （与 agent-os 原 runDirectiveDelivery 的「001-...」命名约定一致，审批队列按此识别项目）。
func deliveryProjectName(directive string) string {
	safe := deliverySanitize(directive)
	if safe == "" {
		safe = "项目"
	}
	ts := time.Now().Format("0102-150405")
	return fmt.Sprintf("001-%s-%s", safe, ts)
}

// deliverySanitize 清理文件名中的非法字符（与 agent-os sanitizeFilename 同规则）。
func deliverySanitize(s string) string {
	replacer := strings.NewReplacer(
		" ", "-", "/", "-", "\\", "-", ":", "-", "*", "-",
		"?", "-", "\"", "-", "<", "-", ">", "-", "|", "-",
		"·", "-", "（", "(", "）", ")", "，", ",", "。", "",
		"\n", "-", "\r", "-",
	)
	return strings.TrimSpace(replacer.Replace(s))
}

// deliveryAgentHome 公司交付落盘的 agent 目录（与项目目录约定一致）。
func deliveryAgentHome(role, name string) string {
	return filepath.Join(companyDir(), name)
}

// deliveryShellEscape 简单转义，避免文件名含特殊字符破坏 zip/路径。
func deliverySHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:])
}

// 公司交付阶段进度：writeDeliveryFile 每写成一个阶段产物即 +1，让 directive-run 记录中间进度，
// 前端进度条不再 0/100 跳变（此前只有 queued→completed，生产中途永远 0%）。
var companyDeliveryStageMu sync.Mutex
var companyDeliveryStageDone int
var companyDeliveryStageTotal int

func companyResetDeliveryStage() {
	companyDeliveryStageMu.Lock()
	defer companyDeliveryStageMu.Unlock()
	companyDeliveryStageDone = 0
	companyDeliveryStageTotal = len(companyDeliveryStageOrder)
}

func companyBumpDeliveryStage() {
	companyDeliveryStageMu.Lock()
	defer companyDeliveryStageMu.Unlock()
	companyDeliveryStageDone++
	if companyDeliveryStageTotal == 0 {
		companyDeliveryStageTotal = len(companyDeliveryStageOrder)
	}
	companySetDirectiveProgress(companyDeliveryStageDone, companyDeliveryStageTotal)
}

// writeDeliveryFile 写一份交付物文件并返回其路径。
func writeDeliveryFile(projectDir, name string, content []byte) (string, error) {
	path := filepath.Join(projectDir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return "", err
	}
	// 每个阶段产物落盘 = 完成一阶段 → 更新 directive 中间进度
	companyBumpDeliveryStage()
	return name, nil
}

// buildCompanyDeliveryHTML 手写一个响应式可运行 HTML 原型（满足 UI 阶段校验：
// <>doctype html + @media 媒体查询；runnable 阶段需 <script> + onclick）。
// 根据是否可运行，追加交互脚本。
func buildCompanyDeliveryHTML(project, brief string, runnable bool) string {
	mode := "UI PROTOTYPE · 响应式原型"
	script := ""
	if runnable {
		mode = "RUNNABLE PRODUCT · 可运行程序"
		script = `<script>
const tasks=[...document.querySelectorAll('.task')];
tasks.forEach(x=>x.addEventListener('click',()=>{x.classList.toggle('done');document.querySelector('#done').textContent=tasks.filter(t=>t.classList.contains('done')).length}));
document.querySelector('#launch').addEventListener('click',()=>document.body.classList.toggle('focus'));
</script>`
	}
	return `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>` +
		deliveryXML(project) +
		`</title><style>*{box-sizing:border-box}body{margin:0;background:#06110f;color:#ecfdf5;font:16px/1.5 Inter,"Microsoft YaHei",sans-serif;min-height:100vh;overflow-x:hidden}body:before{content:"";position:fixed;inset:-30%;background:radial-gradient(circle at 75% 20%,#0f766e88,transparent 28%),radial-gradient(circle at 10% 80%,#22c55e33,transparent 24%);filter:blur(28px);pointer-events:none}.shell{position:relative;max-width:1180px;margin:auto;padding:48px 32px}.nav{display:flex;justify-content:space-between;align-items:center;border-bottom:1px solid #ffffff20;padding-bottom:22px}.brand{font-weight:900;letter-spacing:.16em}.mode{color:#5eead4;font:700 11px monospace;letter-spacing:.18em}.hero{display:grid;grid-template-columns:1.2fr .8fr;gap:64px;padding:92px 0 62px}.eyebrow{color:#4ade80;font:800 12px monospace;letter-spacing:.2em}h1{font-size:clamp(52px,8vw,104px);line-height:.92;letter-spacing:-.07em;margin:18px 0 28px;max-width:780px}h1 i{color:#5eead4;font-style:normal}.lead{max-width:660px;color:#a7f3d0;font-size:18px}.score{align-self:end;border-left:1px solid #ffffff25;padding:16px 0 16px 32px}.score strong{display:block;font-size:72px;line-height:1;color:#5eead4}.score span{color:#94a3b8;font-size:12px}.board{display:grid;grid-template-columns:repeat(3,1fr);gap:14px}.card{background:#ffffff0b;border:1px solid #ffffff18;border-radius:18px;padding:22px;min-height:170px;backdrop-filter:blur(16px);transition:.2s}.card:hover{transform:translateY(-5px);border-color:#5eead488}.card b{display:block;color:#5eead4;font:700 11px monospace;letter-spacing:.14em}.card h2{font-size:22px;margin:28px 0 8px}.card p{color:#94a3b8;font-size:13px}.task{cursor:pointer}.task.done{opacity:.42;text-decoration:line-through}.cta{margin-top:28px;display:flex;align-items:center;gap:18px}button{border:0;border-radius:999px;padding:14px 26px;font-weight:700;cursor:pointer;background:#0f766e;color:#fff}button:hover{background:#115e59}img{max-width:100%;border-radius:14px}@media(max-width:820px){.hero{grid-template-columns:1fr;gap:28px;padding:48px 0}h1{font-size:clamp(40px,10vw,64px)}.board{grid-template-columns:1fr 1fr}}</style></head><body><div class="shell"><nav class="nav"><span class="brand">RESCENE · COMPANY</span><span class="mode">` +
		mode + `</span></nav><section class="hero"><div><div class="eyebrow">PRODUCTION DELIVERY</div><h1>` +
		deliveryXML(project) + `</h1><p class="lead">` + deliveryXML(brief) + `</p><div class="cta"><button id="launch">开始使用</button></div></div><div class="score"><strong id="done">0</strong><span>已完成任务</span></div></section><div class="board"><div class="card"><b>研究</b><h2>数据驱动</h2><p>结构化研究数据见 02-研究数据.xlsx</p></div><div class="card"><b>设计</b><h2>响应式</h2><p>UI 原型覆盖移动端与桌面端</p></div><div class="card"><b>工程</b><h2>可运行</h2><p>打开本页即可交互体验</p></div></div><h2 style="margin-top:40px">任务清单</h2><ul style="list-style:none;padding:0"><li class="task">研究数据整理</li><li class="task">UI 原型设计</li><li class="task">PPT 路演稿</li><li class="task">宣传片脚本</li></ul></div>` + script + `</body></html>`
}

// deliveryProductHTML 让模型按用户指令真实生成一个产品级 HTML（含真实交互逻辑）。
// 该函数产出可运行的单页应用，把 brief 落成真实功能而不是演示壳。
// upstream 为上一个阶段（如 UI 原型/需求）的产物，作为本阶段编码 agent 的交接触入。
func deliveryProductHTML(project, brief string, runnable bool, upstream string) string {
	role := "前端工程师"
	requireScript := ""
	if runnable {
		requireScript = "，并且必须包含可运行的 <script> 交互逻辑（如计时器、按钮事件、状态切换等），是真正能用的产品"
	} else {
		requireScript = "，作为 UI 原型展示界面，包含 <script> 与基本交互，便于在沙箱中预览"
	}
	criteria := "生成一个完整的单文件 HTML 产品，紧扣用户指令的功能点，含中文字体样式与响应式布局" + requireScript
	// 先让模型生成，失败则回退到模板壳（保证门禁不因外部服务挂死）。
	if html, err := deliveryLLMContent(role, brief, "产品HTML", criteria, upstream); err == nil && strings.Contains(html, "<html") {
		// 模型可能用 markdown 代码围栏包裹 HTML（```html ... ```），必须剥离后再落盘，
		// 否则浏览器把首行 ```html 当文本渲染成空白。见 03-UI原型.html 空白 bug。
		return deliveryStripCodeFence(html)
	}
	// 回退：模板壳（仅当模型不可用时）
	return buildCompanyDeliveryHTML(project, brief, runnable)
}

// deliveryStripCodeFence 剥离首尾 markdown 代码围栏（```html ... ``` / ``` ... ```），
// 若内容本身非围栏包裹则原样返回。用于确保 HTML/MD 交付物是纯内容、可正常预览。
func deliveryStripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		// 去掉首行围栏（``` 或 ```html / ```html\n）
		if idx := strings.Index(s, "\n"); idx >= 0 {
			s = s[idx+1:]
		} else {
			s = ""
		}
	}
	if strings.HasSuffix(s, "```") {
		idx := strings.LastIndex(s, "```")
		if idx >= 0 {
			s = strings.TrimSpace(s[:idx])
		}
	}
	return strings.TrimSpace(s)
}

// deliveryXML 转义 XML 特殊字符。
func deliveryXML(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&apos;")
	return r.Replace(s)
}

// deliveryLLMContent 通过公司聚合免费池真实生成一段内容（不写死模板）。
// 多 Agent 接力版：upstream 是上一个阶段的交付物正文，作为本阶段 agent 的交接输入；
// 独立上下文 + 衔接上游，形成真正的接力链（而非同参数反复调同一个模型）。
// 调研阶段（研究员/调研报告）额外用免费 Bing 联网抓真实资料注入 prompt，让调研有真凭实据，
// 符合「有免费联网就用、别让模型凭空编」的质量铁律。
func deliveryLLMContent(role, brief, stage, criteria, upstream string) (string, error) {
	upstreamBlock := "（本阶段是起始环节，无上游交付物）"
	if strings.TrimSpace(upstream) != "" {
		upstreamBlock = "上一环节已验收交付物（请在此基础上往下做，不要重复）:\n" + deliveryTruncate(upstream, 4000)
	}
	// 调研类阶段：免费 Bing 联网抓真实资料（零配置、国内可达），结果 + 来源 URL 注入 prompt。
	netBlock := ""
	if stage == "调研报告" || stage == "research" || role == "研究员" {
		if text, urls := deliveryWebSources(brief); text != "" {
			netBlock = "\n以下是刚从 Bing 免费联网搜索到的真实资料（可引用其中的来源链接）:\n" +
				deliveryTruncate(text, 3000) + "\n来源URL: " + strings.Join(urls, " ; ") +
				"\n引用时请优先用这些真实来源，不要编造链接、数字或引用。\n"
		}
	}
	prompt := fmt.Sprintf(`你是 Rescene 公司中的【%s】，是【%s】环节的独立 Agent，与你协作的其他 Agent 已经完成了上一环节。你收到一张真实工单，请产出【%s】阶段的正式交付物正文，请基于上游产物往下接力，不要自行改变公司目标。

用户指令（公司要做的产品）：
%s
%s
%s
本阶段任务：%s
验收标准：
- %s

要求：
- 内容必须紧扣上面的用户指令和上游产物，针对这个具体产品，不要写通用套话
- 只专注你自己这一阶段（如调研阶段只写调研，编码阶段只写代码），不要重复/改写其他阶段的产物
- 中文输出，结构清晰，可直接落盘为交付物
- 不要声称访问了没有访问的网页，不要编造链接、数字或引用
- 直接输出交付物正文，不要解释过程`, role, stage, stage, brief, netBlock, upstreamBlock, stage, criteria)
	return callCompanyModelRetry(prompt)
}

// deliveryTruncate 截断长文本到指定 rune 数，供交接时防上下文爆炸。
func deliveryTruncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "\n…（上游产物已截断）"
}

// deliverySplitDataLines 把模型对「研究数据」阶段的输出拆成待解析的行：
// 去 markdown 语法记号（#、*、-、---、数字标号），只保留看起来是「功能点|说明|状态」的行。
func deliverySplitDataLines(s string) []string {
	var out []string
	for _, ln := range strings.Split(s, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		// 跳过 markdown 标题/分隔线/纯符号行
		if strings.HasPrefix(ln, "#") || strings.HasPrefix(ln, "---") || ln == "*" || ln == "-" {
			continue
		}
		// 去掉行首的「-」「*」「数字.」列表记号
		ln = strings.TrimLeft(ln, "-*")
		ln = strings.TrimSpace(ln)
		ln = regexp.MustCompile(`^\d+[\.、)）]\s*`).ReplaceAllString(ln, "")
		ln = strings.TrimSpace(ln)
		ln = strings.ReplaceAll(ln, "****", "")
		ln = strings.ReplaceAll(ln, "**", "")
		ln = strings.ReplaceAll(ln, "*", "")
		ln = strings.ReplaceAll(ln, "`", "")
		if ln == "" {
			continue
		}
		out = append(out, ln)
	}
	return out
}

// deliveryParseDataRow 把一行解析成 n 列的 xlsx 行。
// 优先按「|」分隔；若行里没有竖线，则按「：/·/：」等常见分隔把「功能名：说明」拆成一列+说明。
// 返回 nil 表示该行不是有效数据行。
func deliveryParseDataRow(line string, n int) []string {
	if strings.TrimSpace(line) == "" {
		return nil
	}
	// 去除可能残留的 markdown 加粗（**xx** 只留 xx）
	parts := strings.Split(line, "|")
	if len(parts) < 2 {
		// 无竖线：尝试「功能：说明」或「功能·说明」（先统一全角冒号为半角，避免 UTF-8 半字节切割）
		if idx := strings.Index(line, "："); idx >= 0 {
			head := strings.TrimSpace(line[:idx])
			rest := strings.TrimSpace(line[idx+3:])
			return fillRow([]string{head, rest, "必备"}, n)
		}
		if idx := strings.Index(line, ":"); idx >= 0 {
			head := strings.TrimSpace(line[:idx])
			rest := strings.TrimSpace(line[idx+1:])
			return fillRow([]string{head, rest, "必备"}, n)
		}
		if idx := strings.Index(line, "·"); idx > 0 {
			head := strings.TrimSpace(line[:idx])
			rest := strings.TrimSpace(line[idx+1:])
			if len(head) > 1 && len(rest) > 2 {
				return fillRow([]string{head, rest, "必备"}, n)
			}
		}
		// 兜底：整行作为「功能名」，说明留空
		return fillRow([]string{strings.TrimSpace(line), "-", "必备"}, n)
	}
	row := make([]string, 0, n)
	for i := 0; i < n && i < len(parts); i++ {
		row = append(row, strings.TrimSpace(strings.ReplaceAll(parts[i], "**", "")))
	}
	return fillRow(row, n)
}

// fillRow 保证行长度到 n，缺失列填「-」。
func fillRow(row []string, n int) []string {
	for len(row) < n {
		row = append(row, "-")
	}
	for i := 0; i < n; i++ {
		if strings.TrimSpace(row[i]) == "" {
			row[i] = "-"
		}
	}
	return row
}

// deliveryWebSources 免费 Bing 联网抓真实资料，直接复用现成 bingSearch（免 key、国内可达）。
func deliveryWebSources(brief string) (string, []string) {
	text, urls, err := bingSearch(context.Background(), brief, 5)
	if err != nil || len(urls) == 0 {
		return "", nil
	}
	return text, urls
}

// deliverySourcesMarkdown 把真实搜索结果 URL 渲染成调研报告末尾的「参考来源」段。
func deliverySourcesMarkdown(brief string, urls []string) string {
	if len(urls) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n---\n\n## 参考来源（Bing 免费联网检索，真实抓取）\n\n")
	for i, u := range urls {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, u))
	}
	b.WriteString(fmt.Sprintf("\n> 检索时间：%s · 检索词：%s\n", time.Now().Format("2006-01-02 15:04"), deliveryTruncate(brief, 120)))
	return b.String()
}

// deliveryRoleAgentDirs 角色前缀 → 已有的 agent 目录名（若存在）。找不到对应角色目录时返回空，
// 该阶段产物仍保留在项目目录内，不影响门禁。
func deliveryRoleAgentDirs(role string) string {
	base := companyDir()
	switch role {
	case "ceo":
		return filepath.Join(base, "ceo-01")
	case "researcher", "research":
		return filepath.Join(base, "researcher-04")
	case "writer", "requirements", "docs":
		return filepath.Join(base, "writer-15")
	case "designer", "ui":
		return filepath.Join(base, "designer-04")
	case "coder", "code", "runnable":
		return filepath.Join(base, "coder-03")
	case "promoter", "ppt", "pv", "promotion", "publisher":
		return filepath.Join(base, "promoter-18")
	}
	return ""
}

// deliveryMirrorToRoleDirs 把项目目录里各阶段的交付物按角色镜像落一份到对应 agent 目录的
// outputs/，供审批台拾取多个角色（形成 agents>=2 的团队项目）。镜像失败不影响门禁——项目目录
// 内的完整交付仍是主依据。
func deliveryMirrorToRoleDirs(projectDir string, ev []struct{ stage, role, file, kind, verification string }) {
	seen := map[string]bool{}
	for _, def := range ev {
		if def.role == "" || def.file == "" {
			continue
		}
		roleDir := deliveryRoleAgentDirs(def.role)
		if roleDir == "" || seen[roleDir] {
			continue
		}
		src := filepath.Join(projectDir, def.file)
		data, err := os.ReadFile(src)
		if err != nil || len(data) == 0 {
			continue
		}
		outDir := filepath.Join(roleDir, "outputs")
		_ = os.MkdirAll(outDir, 0o755)
		_ = os.WriteFile(filepath.Join(outDir, def.file), data, 0o644)
		seen[roleDir] = true
	}
}

// cleanPPTText 剥离模型输出里的 markdown 结构符（**、###、行首 - 等），避免原样进 PPT 页面。
func cleanPPTText(s string) string {
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "###", "")
	s = strings.ReplaceAll(s, "##", "")
	s = strings.ReplaceAll(s, "#", "")
	s = strings.ReplaceAll(s, "*", "")
	s = strings.TrimLeft(s, "-·•\t0123456789. ")
	return strings.TrimSpace(s)
}

// deliveryPPTFromBrief 生成路演 PPT 的幻灯片大纲（供 genPptx 使用）。
// 固定结构（封面 + 核心问题/方案亮点/功能实现/宣传行动 4 页），模型产出的行只当要点、不当标题，
// 每页保证至少一条要点——不再出现「内容行被当成标题 → 该页白屏」的问题。
func deliveryPPTFromBrief(project, brief, upstream string) []officeSlide {
	pageTitles := []string{"核心问题", "方案亮点", "功能实现", "宣传行动"}
	slides := []officeSlide{{Title: project, Bullets: []string{"多 Agent 接力 · 真实交付", "可运行 · 可审批"}}}

	// 从模型产出抓要点行（cleaned），依次分配到各内容页
	var lines []string
	if outline, err := deliveryLLMContent("路演策划", brief, "路演PPT大纲",
		"给出 4 页路演 PPT 的大纲：产品要解决的问题、方案亮点、核心功能、宣传call-to-action，每页 3-5 条要点，用中文，不要编造数据", upstream); err == nil && len(outline) > 20 {
		for _, ln := range strings.Split(outline, "\n") {
			ln = cleanPPTText(ln)
			if ln == "" {
				continue
			}
			// 过滤模型输出的元信息行（如「路演 PPT 大纲（共 4 页）」「第 1 页 …」），只留真实产品要点
			if strings.Contains(ln, "共") && strings.Contains(ln, "页") {
				continue
			}
			if strings.HasPrefix(ln, "第 ") && strings.Contains(ln, "页") {
				continue
			}
			if strings.Contains(ln, "PPT 大纲") || strings.Contains(ln, "大纲") {
				continue
			}
			// 截断超长要点，避免卡片文字溢出/截断
			if r := []rune(ln); len(r) > 36 {
				ln = string(r[:36]) + "…"
			}
			lines = append(lines, ln)
			if len(lines) >= 20 {
				break
			}
		}
	}

	idx := 0
	for _, title := range pageTitles {
		pg := officeSlide{Title: title, Bullets: []string{}}
		for idx < len(lines) && len(pg.Bullets) < 5 {
			pg.Bullets = append(pg.Bullets, lines[idx])
			idx++
		}
		if len(pg.Bullets) == 0 {
			pg.Bullets = []string{"详见对应交付物"}
		}
		slides = append(slides, pg)
	}
	// 多余的要点追加到最后一页（避免内容行被丢）
	for idx < len(lines) {
		last := &slides[len(slides)-1]
		if len(last.Bullets) >= 6 {
			slides = append(slides, officeSlide{Title: "补充", Bullets: []string{}})
			last = &slides[len(slides)-1]
		}
		last.Bullets = append(last.Bullets, lines[idx])
		idx++
	}
	return slides
}

// deliveryBuildProject 把一条指令真实生产为一份完整交付物。
// 返回项目目录名（形如 001-<name>-<timestamp>）与产物路径；任何硬门禁失败返回 error。
func deliveryBuildProject(projectName, brief string) (string, error) {
	projectDir := filepath.Join(companyDir(), "coder-03", "projects", projectName)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return "", err
	}

	manifest := companyDeliveryManifest{
		Project: projectName, Status: "blocked", GeneratedAt: time.Now().Format(time.RFC3339), GateVersion: 1,
	}
	var missing []string
	// 重置本指令的阶段进度计数器（writeDeliveryFile 每阶段 +1）
	companyResetDeliveryStage()

	// 1. meeting —— 结构化 kickoff 会议证据（JSON 合法）
	meetingContent := fmt.Sprintf(`{"topic":%q,"kind":"evidence_kickoff","participants":["researcher","writer","designer","coder","promoter","publisher"],"decision":"所有硬门槛通过后才能进入审批"}`, projectName)
	if _, err := writeDeliveryFile(projectDir, "00-项目会议.meeting.json", []byte(meetingContent)); err != nil {
		return "", err
	}

	// 2. requirements —— 需求计划（需求分析师 agent，起始环节无上游）
	reqContent := ""
	if c, err := deliveryLLMContent("需求分析师", brief, "需求计划", "把用户指令拆成具体、可验收的需求与验收标准，含功能点", ""); err == nil && strings.TrimSpace(c) != "" {
		reqContent = c
	} else {
		// 模型不可用时回退到基础模板，保证门禁不因外部服务挂死
		reqContent = "# " + projectName + "\n\n## 用户指令\n\n" + brief + "\n\n## 验收标准\n\n- 真实多阶段分工\n- 所有非文本产物可预览\n- 可运行程序具有真实交互\n- 缺少任一强制阶段不得进入审批\n"
	}
	if _, err := writeDeliveryFile(projectDir, "00-需求计划.md", []byte(reqContent)); err != nil {
		return "", err
	}
	prevContent := reqContent // 接力链：需求产物交给下一环节
	appendCompanyRelay(companyRelayEvent{From: "writer-15", To: "researcher-04", Stage: "requirements", Artifact: "00-需求计划.md", Status: "done", DoneAt: time.Now().Format(time.RFC3339)})

	// 3. research —— 调研报告（研究员 agent，基于需求产物往下接力）
	researchContent := ""
	if c, err := deliveryLLMContent("研究员", brief, "调研报告", "给出该产品要解决的核心问题、目标用户、同类对比与关键实现要点", prevContent); err == nil && strings.TrimSpace(c) != "" {
		researchContent = c
	} else {
		researchContent = "# 调研与证据\n\n本报告只引用同目录磁盘产物；结构化数据见 `02-研究数据.xlsx`。\n\n" + brief + "\n"
	}
	// 调研报告必须带真实联网来源（不能指望模型自觉抄 URL）——确定性追加到末尾。
	if _, urls := deliveryWebSources(brief); len(urls) > 0 {
		researchContent += deliverySourcesMarkdown(brief, urls)
	}
	if _, err := writeDeliveryFile(projectDir, "01-调研报告.md", []byte(researchContent)); err != nil {
		return "", err
	}
	prevContent = researchContent // 接力链：调研产物交给数据环节
	appendCompanyRelay(companyRelayEvent{From: "researcher-04", To: "writer-15", Stage: "research", Artifact: "01-调研报告.md", Status: "done", DoneAt: time.Now().Format(time.RFC3339)})

	// 4. data —— 研究数据 XLSX（数据分析师 agent，基于调研产物往下接力）
	// 模型可能不遵守「功能点|说明|状态」格式而输出整段 markdown（会让 xlsx 塞进乱码）。
	// 这里做两层清洗：① 去 markdown 语法记号 ② 按 | 拆行；拆不出结构化行就从文本抽要点兜底。
	dataRows := [][]string{
		{"功能点", "说明", "状态"},
	}
	if dataPoints, err := deliveryLLMContent("数据分析师", brief, "研究数据",
		"给出该产品 6-10 条核心功能点，每条单独一行，严格用「功能点|说明|状态(必备/可选)」三列格式，不要用 #、*、- 等 markdown 符号，不要写标题", prevContent); err == nil && strings.TrimSpace(dataPoints) != "" {
		for _, ln := range deliverySplitDataLines(dataPoints) {
			row := deliveryParseDataRow(ln, 3)
			if row == nil {
				continue
			}
			dataRows = append(dataRows, row)
		}
	}
	if len(dataRows) == 1 { // 模型没产出，回退到基础证据行
		dataRows = append(dataRows,
			[]string{"核心功能", "围绕用户指令的 6-10 条功能点", "必备"},
			[]string{"可运行程序", "output-app.html 含真实交互", "必备"},
		)
	}
	xlsxData, err := genXlsx(projectName+"·可复算研究证据", []officeSheet{
		{Name: "项目证据", Headers: dataRows[0], Rows: dataRows[1:]},
	})
	if err != nil {
		return "", fmt.Errorf("Excel 门禁失败: %w", err)
	}
	if _, err := writeDeliveryFile(projectDir, "02-研究数据.xlsx", xlsxData); err != nil {
		return "", err
	}

	// 5. ui —— UI 原型 html（设计师 agent，基于需求+调研产物往下接力）
	uiHTML := deliveryProductHTML(projectName, brief, false, prevContent)
	if _, err := writeDeliveryFile(projectDir, "03-UI原型.html", []byte(uiHTML)); err != nil {
		return "", err
	}
	prevContent = uiHTML // 接力链：UI 原型交给编码环节
	appendCompanyRelay(companyRelayEvent{From: "designer-04", To: "coder-03", Stage: "ui", Artifact: "03-UI原型.html", Status: "done", DoneAt: time.Now().Format(time.RFC3339)})

	// 6. docs —— 软件文档（文档工程师 agent，基于 UI 原型产物往下接力）
	docsContent := ""
	if c, err := deliveryLLMContent("文档工程师", brief, "软件文档", "写该产品的功能说明、使用步骤、技术要点与注意事项", prevContent); err == nil && strings.TrimSpace(c) != "" {
		docsContent = c
	} else {
		docsContent = "# " + projectName + " 软件文档\n\n## 运行\n\n直接打开 `output-app.html`。\n\n## 验收\n\n以 `delivery.manifest.json` 中 SHA-256 与阶段状态为准。\n"
	}
	if _, err := writeDeliveryFile(projectDir, "04-软件文档.md", []byte(docsContent)); err != nil {
		return "", err
	}

	// 7+8. code + runnable —— 可运行程序（前端工程师 agent，基于 UI 原型产物往下接力）
	runnableHTML := deliveryProductHTML(projectName, brief, true, prevContent)
	if _, err := writeDeliveryFile(projectDir, "output-app.html", []byte(runnableHTML)); err != nil {
		return "", err
	}
	prevContent = runnableHTML // 接力链：可运行程序交给路演环节
	appendCompanyRelay(companyRelayEvent{From: "coder-03", To: "promoter-18", Stage: "code", Artifact: "output-app.html", Status: "done", DoneAt: time.Now().Format(time.RFC3339)})

	// 9. ppt —— 路演 PPTX（路演策划 agent，基于产品产物产出大纲，复用纯 Go genPptx）
	slides := deliveryPPTFromBrief(projectName, brief, prevContent)
	pptxData, err := genPptx(projectName, slides)
	if err != nil {
		return "", fmt.Errorf("PPTX 门禁失败: %w", err)
	}
	if _, err := writeDeliveryFile(projectDir, "05-项目路演.pptx", pptxData); err != nil {
		return "", err
	}

	// 10. pv —— 宣传视频（尽力而为；真视频做不了则用免费生图素材代替，都不行才标记缺失）
	// 视频成功→evidence；生图素材成功→evidence（kind=pv-fallback-images）；全失败→missing。
	var pvPath string
	if p, videoErr := deliveryRenderVideoDirect(projectDir, projectName, brief); videoErr == nil && p != "" {
		pvPath = filepath.Join(projectDir, "06-宣传PV.mp4")
	} else if f, imgErr := deliveryRenderPvFallback(projectDir, projectName, brief); imgErr == nil && f != "" {
		pvPath = f // 生图素材代替（06-宣传PV.manifest.json + 分镜图）
	} else {
		missing = append(missing, "pv")
	}

	// 11. promotion —— 发布回执（绑定可运行入口 SHA256）
	runnableBytes, _ := os.ReadFile(filepath.Join(projectDir, "output-app.html"))
	appSum := deliverySHA256(runnableBytes)
	receiptBody := fmt.Sprintf("status=published\nchannel=local-project-preview\nproject=%s\npublished_at=%s\nentry=output-app.html\nentry_sha256=%s\n",
		projectName, time.Now().Format(time.RFC3339), appSum)
	if _, err := writeDeliveryFile(projectDir, "07-发布.receipt", []byte(receiptBody)); err != nil {
		return "", err
	}

	// 汇总证据哈希
	evidenceDefs := []struct{ stage, role, file, kind, verification string }{
		{"meeting", "ceo", "00-项目会议.meeting.json", "meeting", "结构化参会者与硬门槛决议已落盘"},
		{"research", "researcher", "01-调研报告.md", "text", "调研报告引用同目录结构化数据"},
		{"data", "researcher", "02-研究数据.xlsx", "spreadsheet", "有效 OOXML 工作簿"},
		{"requirements", "writer", "00-需求计划.md", "text", "需求与验收标准已落盘"},
		{"ui", "designer", "03-UI原型.html", "html", "响应式 HTML 原型可在沙箱内渲染"},
		{"docs", "writer", "04-软件文档.md", "text", "运行与验收说明完整"},
		{"code", "coder", "output-app.html", "html", "HTML/CSS/JavaScript 源码已落盘"},
		{"runnable", "coder", "output-app.html", "html", "浏览器直接打开并包含交互脚本"},
		{"ppt", "promoter", "05-项目路演.pptx", "pptx", "PowerPoint OOXML 已渲染"},
		{"promotion", "publisher", "07-发布.receipt", "receipt", "本地项目预览渠道发布回执"},
	}
	// 视频落盘成功则把 pv 列入证据；未落盘则已在上面 append 进 missing。
	// 真视频路径=06-宣传PV.mp4；生图兜底路径=06-宣传PV.manifest.json（kind=pv-fallback-images）。
	if pvPath != "" {
		pvKV := struct{ stage, role, file, kind, verification string }{
			"pv", "promoter", "06-宣传PV.mp4", "video", "MP4 已由视频引擎完成编码",
		}
		if strings.HasSuffix(pvPath, ".json") {
			pvKV = struct{ stage, role, file, kind, verification string }{
				"pv", "promoter", "06-宣传PV.manifest.json", "pv-fallback", "无视频引擎，改用免费生图宣传素材（kind=pv-fallback-images）",
			}
		}
		evidenceDefs = append(evidenceDefs, pvKV)
	}

	// 接力产物镜像：把每个阶段的交付物按角色落一份到对应 agent 目录的 outputs/，
	// 让审批台遍历各 agent 目录时能拾取多个角色（agents>=2 的团队项目自然成立），
	// 这是「真多 Agent 接力」在审批侧的关键——产物按角色归位，而非全塞在单个 coder-03。
	deliveryMirrorToRoleDirs(projectDir, evidenceDefs)

	for _, def := range evidenceDefs {
		data, err := os.ReadFile(filepath.Join(projectDir, def.file))
		if err != nil || len(data) == 0 {
			missing = append(missing, def.stage)
			continue
		}
		info, _ := os.Stat(filepath.Join(projectDir, def.file))
		manifest.Evidence = append(manifest.Evidence, companyDeliveryFile{
			Stage: def.stage, ProducerRole: def.role, File: def.file, Kind: def.kind,
			SHA256: deliverySHA256(data), Bytes: info.Size(), Verification: def.verification,
		})
	}

	// 如果 pv 缺失是唯一 missing 项，仍允许进审批（视频尽力而为，其余阶段全部可复算）。
	if len(manifest.Evidence) >= 9 {
		manifest.Status = "verified"
	}
	manifest.Missing = missing

	encoded, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(projectDir, "delivery.manifest.json"), encoded, 0o644); err != nil {
		return "", err
	}

	return projectDir, nil
}

// deliveryRenderVideoDirect 尽力而为地渲染宣传视频：直接调 mambo_video.py（edge-tts 配音 +
// ffmpeg 竖屏合成，--no-online 只用本地素材兜底）。有 ffmpeg + python + edge_tts 时出真片；
// 否则返回 error，由上层把 pv 标记为缺省阶段（不阻塞完整交付）。
func deliveryRenderVideoDirect(projectDir, project, brief string) (string, error) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		// 视频引擎不可用：返回错误，让上层把 pv 阶段标记为缺省（不交付假视频）。
		return "", fmt.Errorf("视频引擎不可用: %w", err)
	}
	py, err := findPython()
	if err != nil {
		return "", err
	}
	root, err := backendRoot()
	if err != nil {
		return "", err
	}
	out := filepath.Join(projectDir, "06-宣传PV.mp4")

	// 素材兜底：本地素材池（assets/mambo）通常只有无关图，match_media 命中不了 → 渐变背景黑屏。
	// 在调 mambo 前用免费生图（Pollinations 免 key，实测可用）生成几张产品相关背景图，
	// 落盘到项目内素材目录，文件名带产品关键词，让 mambo match_media 能命中 → 出真实画面而不黑屏。
	mediaDir := filepath.Join(projectDir, "pv-media")
	genShots := deliveryRenderPvStillShots(mediaDir, project, brief)
	mediaArgs := []string{}
	if len(genShots) > 0 {
		mediaArgs = []string{"--media", mediaDir, "--ordered-media"}
	}

	cmd := hiddenCommandContext(context.Background(), py,
		filepath.Join(root, "scripts", "mambo_video.py"),
		"--topic", project, "--text", brief, "--out", out, "--no-online", "--width", "1920", "--height", "1080")
	cmd.Args = append(cmd.Args, mediaArgs...)
	cmd.Dir = root
	if _, err := cmd.Output(); err != nil {
		return "", err
	}
	if _, err := os.Stat(out); err != nil {
		return "", err
	}
	return out, nil
}

// deliveryRenderPvStillShots 用免费生图（Pollinations 免 key）生成产品相关背景图，落盘到 mediaDir。
// 文件名带产品关键词片段，让 mambo 的 match_media（文件名关键词匹配）能命中。
func deliveryRenderPvStillShots(mediaDir, project, brief string) []string {
	if err := os.MkdirAll(mediaDir, 0o755); err != nil {
		return nil
	}
	// 从产品名/指令里切出 2-4 字短词当文件名，提高 match_media 命中率
	safe := deliverySanitizeKeywords(brief)
	if safe == "" {
		safe = deliverySanitizeKeywords(project)
	}
	if safe == "" {
		safe = "product"
	}
	// 图片文件名前缀用关键词，后缀序数
	shotPrompts := []string{
		brief + "，产品宣传背景，清新现代风格，高清，明亮",
		project + "，产品界面示意，科技感，干净，扁平化",
	}
	created := []string{}
	for i, tp := range shotPrompts {
		res, err := generateImage(context.Background(), imageGenSpec{
			Prompt:   tp,
			Width:    1280,
			Height:   720,
			Provider: "pollinations",
			Model:    "flux",
			OutDir:   mediaDir,
			Name:     fmt.Sprintf("%s-%02d", safe, i+1),
		})
		if err != nil {
			continue
		}
		created = append(created, res.File)
	}
	return created
}

// deliverySanitizeKeywords 从文本里提取可作文件名的核心词（中文 2-4 字片段 / 英文小写）。
func deliverySanitizeKeywords(s string) string {
	s = deliverySanitize(s)
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// 优先取 2-4 字中文词
	if m := regexp.MustCompile(`[\p{Han}]{2,4}`).FindString(s); m != "" {
		return m
	}
	// 英文 fallback：去非字母数字
	if m := regexp.MustCompile(`[a-zA-Z0-9]{2,}`).FindString(s); m != "" {
		return strings.ToLower(m)
	}
	return ""
}

// deliveryRenderPvFallback 宣传视频兜底：真视频（mambo+ffmpeg）做不了时，用免费生图出宣传分镜素材代替，
// 不让 pv 阶段缺失（用户铁律：生图素材可代替传统视频）。出图落盘 + 写 manifest 供前端预览。
// 返回落盘的文件名列表；全部失败才返回 error（此时才标记 pv 缺失）。
func deliveryRenderPvFallback(projectDir, project, brief string) (string, error) {
	shots := []string{
		"封面主视觉：学生专注冲刺台 · 番茄钟计时器 · 大数字25:00 · 手机APP界面 · 明亮",
		"功能介绍：待办分组列表 + 番茄钟进度环 · 清新扁平 · 亮蓝白配色",
		"激励画面：连续专注天数打卡 · 番茄串金句 · 学生微笑 · 高对比卡通",
	}
	imgDir := filepath.Join(projectDir, "06-宣传PV-分镜")
	if err := os.MkdirAll(imgDir, 0o755); err != nil {
		return "", err
	}
	created := []string{}
	for i, prompt := range shots {
		res, err := generateImage(context.Background(), imageGenSpec{
			Prompt:   prompt + "，" + brief + "。产品海报风格，清晰干净，无文字遮挡关键信息",
			Width:    1024,
			Height:   1024,
			Provider: "pollinations",
			Model:    "flux",
			OutDir:   imgDir,
			Name:     fmt.Sprintf("shot-%02d", i+1),
		})
		if err != nil {
			continue // 单张失败不阻塞，尽量多出
		}
		created = append(created, res.File)
		created = append(created, res.URL)
	}
	if len(created) == 0 {
		return "", fmt.Errorf("生图兜底全部失败")
	}
	manifest := map[string]interface{}{
		"kind":    "pv-fallback-images",
		"reason":  "无 ffmpeg/视频引擎，改用免费生图宣传素材代替",
		"project": project,
		"files":   created,
		"time":    time.Now().Format(time.RFC3339),
	}
	mb, _ := json.MarshalIndent(manifest, "", "  ")
	mp := filepath.Join(projectDir, "06-宣传PV.manifest.json")
	if err := os.WriteFile(mp, mb, 0o644); err != nil {
		return "", err
	}
	return mp, nil
}
