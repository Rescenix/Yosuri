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

// deliverySHA256 计算交付物哈希，供门禁复算。
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
	criteria := "生成一个完整的单文件 HTML 产品，紧扣用户指令的功能点，含中文字体样式与响应式布局" + requireScript + `。视觉规格（硬要求，缺一条算不合格）：
- 设计感优先于模板感：主字体用 system-ui 栈（-apple-system/PingFang SC/Microsoft YaHei），中文排版行高≥1.6
- 自定一套和谐配色（主色+辅色+中性灰阶，CSS 变量 :root 定义），禁止默认浏览器蓝紫，禁止纯黑纯白大面积对撞
- 层次感：页面至少有 hero 区 + 核心功能区 + 数据/统计区三个层级，卡片圆角 12-16px + 细腻阴影（多层 box-shadow，不用生硬单层）
- 交互反馈：按钮 hover/active 有过渡（transition ≥ .18s），列表项有进入感，数字变化有过渡
- 响应式 @media 至少断点 640px/960px，移动端不破版
- 细节：中文标点正常渲染、数字用 tabular-nums、图标用 emoji 或纯 CSS 绘制（不引外部图标库）
- 禁止大面积空白：内容不足一屏时，用统计卡片区、使用说明区、图例/分类汇总区把页面填满到至少 2.5 屏高度；列表为空时空状态区要占位有分量（图标+文案+引导按钮），绝不允许页面下半部分整体塌陷成空白
- 收支/数值类产品：正负值必须颜色区分（收入绿/支出红），合计区做成带背景色的统计卡片，净额为负时标红警示`
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
	return companyLiveModelCall(role, stage, prompt)
}

// companyLiveModelCall 走流式竞速，把模型正在写的字实时推给生产大屏；
// 流式失败时退回非流式重试，保证生产不因流式协议差异而中断。
func companyLiveModelCall(role, stage, prompt string) (string, error) {
	project := companyLiveProject()
	emit := companyLiveDeltaEmitter(project, role, stage)
	content, err := companyModelRaceStream(prompt, emit)
	if err == nil && strings.TrimSpace(content) != "" {
		emit("") // 收尾：把缓冲区剩余内容推出去
		// 竞速下「先出字接管大屏的源」未必是「先写完被采纳的源」，流式碎片可能只覆盖正文一角。
		// 补一帧 replaced：前端用它整块替换本阶段文字，保证大屏最终显示的是真正交付的正文。
		companyLivePublish(companyLiveEvent{Kind: "delta", Stage: stage, Role: role, Replaced: true, Text: deliveryTruncate(content, 6000), Project: project})
		return content, nil
	}
	// 回退：非流式重试（部分免费源不支持 stream，或流中途断开）
	out, retryErr := callCompanyModelRetry(prompt)
	if retryErr == nil {
		companyLivePublish(companyLiveEvent{Kind: "delta", Stage: stage, Role: role, Replaced: true, Text: deliveryTruncate(out, 6000), Project: project})
		return out, nil
	}
	return "", err
}

// companyLiveDeltaEmitter 返回一个增量回调：按 ~80 字或 ~150ms 聚合成一条事件，
// 避免每个 token 都发一帧把大屏和磁盘日志刷爆。传空串表示收尾冲刷。
func companyLiveDeltaEmitter(project, role, stage string) func(string) {
	var mu sync.Mutex
	pending := strings.Builder{}
	last := time.Now()
	return func(delta string) {
		mu.Lock()
		defer mu.Unlock()
		if delta == "" {
			if pending.Len() > 0 {
				companyLivePublish(companyLiveEvent{Kind: "delta", Stage: stage, Role: role, Text: pending.String(), Project: project})
				pending.Reset()
			}
			return
		}
		pending.WriteString(delta)
		if pending.Len() >= 80 || time.Since(last) > 150*time.Millisecond {
			companyLivePublish(companyLiveEvent{Kind: "delta", Stage: stage, Role: role, Text: pending.String(), Project: project})
			pending.Reset()
			last = time.Now()
		}
	}
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

// deliveryWebSources 用清洗后的检索词做 Bing 免 key 联网，返回（正文, URL 列表）。
// 检索词必须先过 deliverySearchQuery：整句指令直接丢给 Bing 会拆成单字
// （实测「做一个学生专注冲刺台：番茄钟功能」检索出「做」字的生僻字词典），
// 抽出产品名词与功能词再搜，来源相关性才成立。
func deliveryWebSources(brief string) (string, []string) {
	text, urls, err := bingSearch(context.Background(), deliverySearchQuery(brief), 5)
	if err != nil || len(urls) == 0 {
		return "", nil
	}
	return text, urls
}

// deliverySearchQuery 从整句指令里抽出适合搜索引擎的检索词：
// 剥掉语气/格式词后保留产品名与功能关键词，控制在 40 字内。
func deliverySearchQuery(brief string) string {
	cleaned := brief
	for _, s := range []string{
		"做一个", "做一个", "请做一个", "帮我做一个", "帮我做", "给我做一个", "给我做",
		"做一个能", "做一个可以", "开发一个", "写一个", "实现一个", "设计一个",
		"要能运行", "要能", "可以运行", "能够运行", "能运行", "可运行",
		"包含", "包括", "支持", "并且", "然后", "还有", "以及", "要求", "需要",
		"功能", "的软件", "的工具", "的应用", "的小工具", "软件", "小工具",
	} {
		cleaned = strings.ReplaceAll(cleaned, s, " ")
	}
	// 标点全部转空格，压掉多余空白
	cleaned = regexp.MustCompile(`[^\p{Han}a-zA-Z0-9]+`).ReplaceAllString(cleaned, " ")
	fields := strings.Fields(cleaned)
	// 每段取前 12 字（防超长短语），拼成紧凑检索词
	var parts []string
	n := 0
	for _, f := range fields {
		r := []rune(f)
		if len(r) > 12 {
			f = string(r[:12])
		}
		if f == "" {
			continue
		}
		parts = append(parts, f)
		n += len([]rune(f))
		if n >= 40 {
			break
		}
	}
	q := strings.Join(parts, " ")
	if strings.TrimSpace(q) == "" {
		return deliveryTruncate(brief, 40) // 兜底：清洗完为空就用原句前 40 字
	}
	return q
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

	// 从模型产出抓要点行（cleaned），依次分配到各内容页。
	// 大纲 prompt 按固定页序输出（封面页由 slides 初始化兜底，模型只产出 4 个内容页），
	// 每页要点数 3-5，超长截 36 字 —— 与 pptxSlideXML 的标题降字号/自动换行规则匹配。
	var lines []string
	if outline, err := deliveryLLMContent("路演策划", brief, "路演PPT大纲",
		"给出 4 页路演 PPT 大纲，按固定顺序：第1页「核心问题」=用户痛点 3 条（具体场景，不是空话）；第2页「方案亮点」=产品差异化 3-4 条；第3页「功能实现」=核心功能 4-5 条（功能名+一句话价值）；第4页「宣传行动」=目标人群、推广渠道、call-to-action 各 1-2 条。每条单独一行，不要 markdown 符号，不要编造数据，紧扣这个产品", upstream); err == nil && len(outline) > 20 {
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
	projectDir := companyProjectDir(projectName)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return "", err
	}

	manifest := companyDeliveryManifest{
		Project: projectName, Status: "blocked", GeneratedAt: time.Now().Format(time.RFC3339), GateVersion: 1,
	}
	var missing []string
	// 重置本指令的阶段进度计数器（writeDeliveryFile 每阶段 +1）
	companyResetDeliveryStage()
	companyLiveResetForProject(projectName)
	// 工程选型：需要状态与存储的产品走多文件（index/app.js/styles.css/data.js），
	// 无状态小工具走单文件快车道。选型理由写进项目身份供复盘。
	plan := companyDecidePlan(brief)

	// 1. meeting —— 结构化 kickoff 会议证据（JSON 合法，零模型调用，秒级完成）
	meetingContent := fmt.Sprintf(`{"topic":%q,"kind":"evidence_kickoff","participants":["researcher","writer","designer","coder","promoter","publisher"],"decision":"所有硬门槛通过后才能进入审批"}`, projectName)
	if _, err := writeDeliveryFile(projectDir, "00-项目会议.meeting.json", []byte(meetingContent)); err != nil {
		return "", err
	}
	companyLiveStage(projectName, "meeting", "ceo", "ceo-01", "立项会议完成，直接进入最小原型")

	// 2. MVP —— 一开始就产出可运行的最小原型（v1）。
	// 此前链路要跑完会议/需求/调研/数据四个文本阶段才见到能操作的东西，
	// 用户全程只看进度条。现在第一条指令几十秒内就有真页面，后续阶段全部围着它迭代。
	companyLiveStage(projectName, "mvp", "coder", "coder-03", "最小可运行原型 v1 生成中（"+plan.Reason+"）")
	var mvpHTML string
	if plan.MultiFile {
		mvpFiles := deliveryMultiProject(projectName, brief, "")
		for name, content := range mvpFiles {
			if _, err := writeDeliveryFile(projectDir, name, []byte(content)); err != nil {
				return "", err
			}
		}
		mvpHTML = companyInlineMulti(mvpFiles)
	} else {
		mvpHTML = deliveryProductHTML(projectName, brief, true, "")
	}
	if _, err := writeDeliveryFile(projectDir, "output-app.html", []byte(mvpHTML)); err != nil {
		return "", err
	}
	companyLiveArtifact(projectName, "mvp", "coder", "output-app.html", "v1")
	prevContent := mvpHTML // 接力链：最小原型就是后续所有环节的底座

	// 3. requirements —— 需求计划（需求分析师 agent，基于最小原型补全需求与验收标准）
	reqContent := ""
	if c, err := deliveryLLMContent("需求分析师", brief, "需求计划", "产品已有一版可运行最小原型，把用户指令拆成具体、可验收的需求与验收标准，含功能点。结构要求：①用户故事（作为<角色>，我要<功能>，以便<价值>）至少 3 条；②功能清单表格（功能点|优先级 P0/P1/P2|验收标准）；③非功能需求（性能/兼容/数据安全）至少 2 条；④明确「不在本期范围」的边界项防止范围蔓延。不要写通用套话，每条都要针对这个具体产品", prevContent); err == nil && strings.TrimSpace(c) != "" {
		reqContent = c
	} else {
		// 模型不可用时回退到基础模板，保证门禁不因外部服务挂死
		reqContent = "# " + projectName + "\n\n## 用户指令\n\n" + brief + "\n\n## 验收标准\n\n- 真实多阶段分工\n- 所有非文本产物可预览\n- 可运行程序具有真实交互\n- 缺少任一强制阶段不得进入审批\n"
	}
	if _, err := writeDeliveryFile(projectDir, "00-需求计划.md", []byte(reqContent)); err != nil {
		return "", err
	}
	prevContent = reqContent // 接力链：需求产物交给下一环节
	appendCompanyRelay(companyRelayEvent{From: "writer-15", To: "researcher-04", Stage: "requirements", Artifact: "00-需求计划.md", Status: "done", DoneAt: time.Now().Format(time.RFC3339)})

	// 4. research —— 调研报告（研究员 agent，基于最小原型与需求往下接力）
	companyLiveStage(projectName, "research", "researcher", "researcher-04", "调研报告撰写中（Bing 联网取真实来源）")
	researchContent := ""
	if c, err := deliveryLLMContent("研究员", brief, "调研报告", "给出该产品要解决的核心问题、目标用户画像（具体到人群特征而非「所有人」）、同类产品对比表（至少 2 个竞品：名称|定位|差异）、关键实现要点与技术选型建议。结论必须有依据，引用联网资料时标注来源", prevContent); err == nil && strings.TrimSpace(c) != "" {
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
	// 行数下限：模型只给 2-3 条也算没干完活——从调研/需求产物里抽功能行补足到 6 条。
	dataRows := [][]string{
		{"功能点", "说明", "状态"},
	}
	if dataPoints, err := deliveryLLMContent("数据分析师", brief, "研究数据",
		"给出该产品 6-10 条核心功能点，每条单独一行，严格用「功能点|说明|状态(必备/可选)」三列格式，不要用 #、*、- 等 markdown 符号，不要写标题。功能点必须覆盖用户指令里的每一个明确要求，说明列写清楚这个功能具体做什么、用户怎么用", prevContent); err == nil && strings.TrimSpace(dataPoints) != "" {
		for _, ln := range deliverySplitDataLines(dataPoints) {
			row := deliveryParseDataRow(ln, 3)
			if row == nil {
				continue
			}
			dataRows = append(dataRows, row)
		}
	}
	// 行数下限兜底：不足 6 条时从上游产物（需求/调研）抽含功能语义的行补足，
	// 抽不出来就用通用证据行。xlsx 只有两三行会显得敷衍，也过不了「可复算证据」的定位。
	if n := len(dataRows) - 1; n < 6 {
		for _, ln := range deliverySplitDataLines(prevContent) {
			if n >= 6 {
				break
			}
			row := deliveryParseDataRow(ln, 3)
			if row == nil {
				continue
			}
			// 去重：功能点名已在表里就跳过
			dup := false
			for _, have := range dataRows[1:] {
				if have[0] == row[0] {
					dup = true
					break
				}
			}
			if !dup {
				dataRows = append(dataRows, row)
				n++
			}
		}
		for n < 6 {
			dataRows = append(dataRows, []string{fmt.Sprintf("补充功能点 %d", n+1), "基于用户指令的功能延伸，详见需求计划与可运行程序", "可选"})
			n++
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

	// 5. ui —— 设计迭代（设计师 agent 在最小原型 v1 上改出 v2，大屏实时换页）
	companyLiveStage(projectName, "ui", "designer", "designer-04", "UI 设计迭代：在最小原型上重做视觉与布局")
	uiHTML := deliveryProductHTML(projectName, brief, false, "已上线的最小可运行原型（在此基础上做设计升级，保留全部已有功能）:\n"+deliveryTruncate(mvpHTML, 2500)+"\n\n调研结论:\n"+deliveryTruncate(researchContent, 1500))
	if _, err := writeDeliveryFile(projectDir, "03-UI原型.html", []byte(uiHTML)); err != nil {
		return "", err
	}
	companyLiveArtifact(projectName, "ui", "designer", "03-UI原型.html", "v2")
	prevContent = uiHTML // 接力链：设计稿交给编码环节
	appendCompanyRelay(companyRelayEvent{From: "designer-04", To: "coder-03", Stage: "ui", Artifact: "03-UI原型.html", Status: "done", DoneAt: time.Now().Format(time.RFC3339)})

	// 6. docs —— 软件文档（文档工程师 agent，基于 UI 原型产物往下接力）
	docsContent := ""
	if c, err := deliveryLLMContent("文档工程师", brief, "软件文档", "写该产品的完整软件文档，结构要求：①产品简介（一句话定位+核心价值）；②快速上手（3 步内跑起来，配具体操作说明）；③功能详解（每个功能一节：入口→操作→预期结果）；④常见问题 FAQ 至少 3 条；⑤技术实现说明（数据存储方式、浏览器兼容性）。面向最终用户写，别写成开发日志", prevContent); err == nil && strings.TrimSpace(c) != "" {
		docsContent = c
	} else {
		docsContent = "# " + projectName + " 软件文档\n\n## 运行\n\n直接打开 `output-app.html`。\n\n## 验收\n\n以 `delivery.manifest.json` 中 SHA-256 与阶段状态为准。\n"
	}
	if _, err := writeDeliveryFile(projectDir, "04-软件文档.md", []byte(docsContent)); err != nil {
		return "", err
	}

	// 7+8. code + runnable —— 终版迭代（把设计稿 v2 落成最终可运行程序）
	companyLiveStage(projectName, "code", "coder", "coder-03", "终版迭代：设计稿落地为最终可运行程序")
	upstream := "最小原型 v1（功能基线，必须全部保留）:\n" + deliveryTruncate(mvpHTML, 2000) + "\n\n设计稿 v2（视觉与布局以此为准）:\n" + deliveryTruncate(uiHTML, 2500)
	var runnableHTML string
	if plan.MultiFile {
		// 多文件项目：终版重新产出 4 个源文件（覆盖 v1），再内联成 output-app.html 供预览与门禁。
		finalFiles := deliveryMultiProject(projectName, brief, upstream)
		for name, content := range finalFiles {
			if _, err := writeDeliveryFile(projectDir, name, []byte(content)); err != nil {
				return "", err
			}
		}
		runnableHTML = companyInlineMulti(finalFiles)
	} else {
		runnableHTML = deliveryProductHTML(projectName, brief, true, upstream)
	}
	if _, err := writeDeliveryFile(projectDir, "output-app.html", []byte(runnableHTML)); err != nil {
		return "", err
	}
	companyLiveArtifact(projectName, "runnable", "coder", "output-app.html", "final")
	// 真机质检：用受管浏览器把产品真正打开，实测渲染/交互 + 免费识图评审视觉；
	// 不合格则带缺陷清单返修一轮。返修会改写 output-app.html，故 prevContent 与
	// 后续发布回执 SHA256 都以质检后的最终版为准。质检能力缺失时降级放行不卡死。
	companyLiveStage(projectName, "qa", "qa", "qa-01", "质检员正在真机打开产品实测（渲染 / 交互 / 视觉）")
	finalEntry, qaReport := companyQAAudit(projectDir, projectName, brief, "output-app.html", plan.MultiFile, upstream)
	if strings.TrimSpace(finalEntry) != "" {
		runnableHTML = finalEntry
	}
	if qaFile := saveCompanyQAReport(projectDir, qaReport); qaFile != "" {
		companyLiveArtifact(projectName, "qa", "qa", qaFile, "")
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

	// 项目身份落盘：参与角色来自上面的证据分工，审批台直接读这份索引，
	// 不再把产物复制到各角色目录去凑「多 Agent」。
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
	_ = writeCompanyProjectIndex(projectDir, projectName, brief, manifest)
	companyLivePublish(companyLiveEvent{Kind: "done", Text: "完整交付已落盘 · 进入人类审批", Project: projectName})

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
	// 图片文件名前缀用关键词，后缀序数。生图 prompt 用清洗后的检索词（deliverySearchQuery），
	// 整句指令直接塞给生图模型会带出「做一个/要能运行」这类语气词，画面主题被稀释。
	searchQ := deliverySearchQuery(brief)
	shotPrompts := []string{
		searchQ + "，产品宣传主视觉，清新现代风格，高清，明亮，无文字",
		searchQ + "，产品界面示意，科技感，干净，扁平化，无文字",
		searchQ + "，使用场景插画，柔和光影，专业质感，无文字",
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
	// 生图分镜 prompt：清洗后的检索词 + 分镜叙事（封面→功能→激励），「无文字」避免 AI 乱码字。
	searchQ := deliverySearchQuery(brief)
	shots := []string{
		"封面主视觉：" + searchQ + " · 产品主界面 · 明亮现代 · 海报构图 · 无文字",
		"功能介绍：" + searchQ + " · 核心功能示意 · 清新扁平 · 和谐配色 · 无文字",
		"使用场景：" + searchQ + " · 用户使用插画 · 柔和光影 · 专业质感 · 无文字",
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
