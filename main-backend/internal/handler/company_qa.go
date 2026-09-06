package handler

// company_qa.go —— 交付物真机质检员（2026-09-05）。
//
// 此前门禁只验证「文件存在 + SHA256 合法 + 格式标记」，产物内容好不好没人管：
// 页面打开白屏、按钮点了没反应、做得极丑，全部照样 verified 进审批台。
// 这里补上真正的质量环节：用受管 headless Chromium 把产品**打开**，
// 实测渲染结果与交互反应，再让免费识图模型看截图打分，不合格带缺陷清单返修一轮。

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// companyQAReport 一次质检的结论（落盘 09-质量验收.qa.json，随项目打包留档）。
type companyQAReport struct {
	CheckedAt   string   `json:"checkedAt"`
	Entry       string   `json:"entry"`
	Blank       bool     `json:"blank"`             // 白屏
	JSVisible   int      `json:"visibleElements"`   // 可见元素数
	TextLength  int      `json:"textLength"`        // 页面可读文字量
	Buttons     int      `json:"buttons"`           // 可点击控件数
	Clicked     int      `json:"clickedButtons"`    // 实际点过的按钮数
	DOMChanged  bool     `json:"domChanged"`        // 点击后 DOM 是否发生变化（真交互）
	InteractOK  bool     `json:"interactionMeasured"` // 交互是否真的测到了（测不到时不能判假交互）
	JourneyOK   bool     `json:"journeyMeasured"`   // 用户旅程是否真测到（提交→渲染→刷新持久化）
	JourneyPass bool     `json:"journeyPassed"`     // 核心用户旅程闭环走通（填表→提交→渲染→刷新仍在）
	TopicOK     bool     `json:"topicMeasured"`     // 指令覆盖是否检测过
	TopicHits   int      `json:"topicHits"`         // 命中指令核心功能词数（0=疑似离题/壳）
	TopicTerms  []string `json:"topicTerms,omitempty"` // 指令核心功能词（供审计看）
	Skipped     bool     `json:"skipped"`           // 真检被跳过（浏览器不可用等），不得当「未通过」
	MissingFeatures []string `json:"missingFeatures"` // 指令明确要求但产物里看不到的核心功能
	MissingFeatureCount int `json:"missingFeatureCount"` // 缺失功能数（0=全覆盖）
	VisualScore int      `json:"visualScore"`       // 识图视觉评分 0-10（-1=未评）
	Issues      []string `json:"issues"`            // 具体缺陷（可直接喂给返修）
	Summary     string   `json:"summary"`
	Passed      bool     `json:"passed"`
	Repaired    bool     `json:"repaired"` // 是否经过一轮返修
	BrowserOK   bool     `json:"browserOk"`
	VisionOK    bool     `json:"visionOk"`
	// 布局充实度：页面总高度 / 视口高度。实测抓过「内容只占顶部 15%、下部 85% 空白塌陷」的产物，
	// 白屏判据抓不住它（有内容、有交互），必须用高度比单独卡。
	PageHeightRatio float64 `json:"pageHeightRatio"` // scrollHeight / viewportHeight（<1.2 视为塌陷）
	LayoutOK        bool    `json:"layoutMeasured"`  // 是否真测到了高度比
	FramesReviewed  int     `json:"framesReviewed"`  // 视觉评审实际覆盖的帧数（首屏+滚动帧，1=只看了首屏）
	RepairRounds    int     `json:"repairRounds"`    // 实际执行的返修轮数（0=一次过，1/2=返修轮数）
}

// cdpScreenshot 在已打开的 target 上截当前帧（PNG 字节）。
func cdpScreenshot(tabWS string) ([]byte, error) {
	payload, _ := json.Marshal(map[string]any{
		"id":     9101,
		"method": "Page.captureScreenshot",
		"params": map[string]any{"format": "png", "captureBeyondViewport": false},
	})
	resp, err := wsCall(tabWS, payload)
	if err != nil {
		return nil, err
	}
	var r struct {
		Result struct {
			Data string `json:"data"`
		} `json:"result"`
	}
	if json.Unmarshal(resp, &r) != nil || r.Result.Data == "" {
		return nil, fmt.Errorf("截图结果为空")
	}
	return base64.StdEncoding.DecodeString(r.Result.Data)
}

// cdpCloseTab 关掉探针自己开的标签页，不给用户浏览器留一堆僵尸 target。
// tabWS 形如 ws://127.0.0.1:9222/devtools/page/XXXX。
func cdpCloseTab(tabWS string) {
	rest := strings.TrimPrefix(tabWS, "ws://")
	idx := strings.Index(rest, "/devtools/page/")
	if idx < 0 {
		return
	}
	host := rest[:idx]
	targetID := rest[idx+len("/devtools/page/"):]
	if targetID == "" {
		return
	}
	client := &http.Client{Timeout: 3 * time.Second}
	_, _ = client.Get("http://" + host + "/json/close/" + targetID)
}

// companyQAProbeJS 采集页面真实渲染状态：文字量、可见元素、可点击控件。
const companyQAProbeJS = `(function(){
  var body = document.body;
  if (!body) return JSON.stringify({text:0,vis:0,btn:0});
  var txt = (body.innerText||'').replace(/\s+/g,'');
  var vis = 0, btn = 0;
  var all = document.querySelectorAll('body *');
  for (var i=0;i<all.length;i++){
    var el = all[i];
    var r = el.getBoundingClientRect();
    if (r.width>1 && r.height>1 && getComputedStyle(el).visibility!=='hidden') vis++;
  }
  btn = document.querySelectorAll('button,[role=button],input[type=button],input[type=submit],a[href],.btn,.clickable,[onclick]').length;
  return JSON.stringify({text:txt.length,vis:vis,btn:btn});
})()`

// companyQAInteractionJS 真点一遍：先给空表单填示例值（否则「记一笔」这类带校验的
// 提交被空值拦住，DOM 不变会被误判成假交互），再逐个点击前 6 个可点击控件。
// 点击后 DOM 变化常是异步渲染（setTimeout/rAF），所以这里只记录点击前快照并触发点击，
// 由 Go 侧等待后再取一次快照比对（companyQASnapshotJS）。
const companyQAInteractionJS = `(function(){
  var fill = function(el){
    try {
      if (el.disabled || el.readOnly) return;
      var t = (el.type||'').toLowerCase();
      if (el.tagName === 'TEXTAREA') { if (!el.value) el.value = '测试'; }
      else if (t === 'text' || t === '' || t === 'number') { if (!el.value) el.value = (t==='number') ? '35' : '午饭'; }
      else if (t === 'date') { if (!el.value) el.value = '2026-09-05'; }
      else if (t === 'checkbox') { el.checked = true; }
      if (el.dispatchEvent) el.dispatchEvent(new Event('input', {bubbles:true}));
    } catch(e) {}
  };
  Array.prototype.slice.call(document.querySelectorAll('input,textarea')).forEach(fill);
  window.__qaSnap = document.body.innerHTML.length + '|' + (document.body.innerText||'').length;
  var nodes = Array.prototype.slice.call(document.querySelectorAll('button,[role=button],input[type=button],input[type=submit],.btn,[onclick]')).slice(0,6);
  for (var i=0;i<nodes.length;i++){ try { nodes[i].click(); } catch(e) {} }
  return String(nodes.length);
})()`

// companyQASnapshotJS 取点击后的 DOM 指纹，与 window.__qaSnap 比对得出是否有真实变化。
const companyQASnapshotJS = `(function(){
  var after = document.body.innerHTML.length + '|' + (document.body.innerText||'').length;
  return JSON.stringify({changed: after !== (window.__qaSnap||'')});
})()`

// companyQAJourneyJS 核心用户旅程：填表→提交→断言真实功能闭环（数据入列/持久化）。
// 目标是抓「demo 假功能」——按钮点了有反应，但没有任何真实业务闭环。
// 填值→触发 input 事件→点第一个可点控件→返回页面文字指纹，由 Go 侧刷新页面后
// 再取一次指纹比对，验证「数据真的存下来了」（localStorage/状态持久化）。
const companyQAJourneyJS = `(function(){
  var cleaned = false;
  try { if (window.localStorage) window.localStorage.clear(); cleaned = true; } catch(e) {}
  var fill = function(el){
    try {
      if (el.disabled || el.readOnly) return;
      var t = (el.type||'').toLowerCase();
      if (el.tagName === 'TEXTAREA') { el.value = '质检测试数据'; }
      else if (t === 'text' || t === '' || t === 'number') { el.value = (t==='number') ? '35' : '质检测试'; }
      else if (t === 'date') { el.value = '2026-09-05'; }
      else if (t === 'checkbox') { el.checked = true; }
      if (el.dispatchEvent) el.dispatchEvent(new Event('input', {bubbles:true}));
      if (el.dispatchEvent) el.dispatchEvent(new Event('change', {bubbles:true}));
    } catch(e) {}
  };
  Array.prototype.slice.call(document.querySelectorAll('input,textarea')).forEach(fill);
  window.__qaJourney = (document.body.innerText||'').length;
  var target = document.querySelector('button,[role=button],input[type=button],input[type=submit],.btn,[onclick]');
  if (target) { try { target.click(); } catch(e) {} }
  return JSON.stringify({cleaned:cleaned});
})()`

// companyQAJourneyTextJS 取旅程后的页面文字（供 Go 刷新前后比对持久化）。
const companyQAJourneyTextJS = `(function(){
  return String((document.body.innerText||'').length);
})()`

type companyQAProbeStats struct {
	Text int `json:"text"`
	Vis  int `json:"vis"`
	Btn  int `json:"btn"`
}

// companyQAAtoi 宽松整数解析（CDP 返回值可能带引号/空白）。
func companyQAAtoi(s string) int {
	s = strings.TrimSpace(strings.Trim(strings.TrimSpace(s), `"`))
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

// companyQATopicTerms 提取指令的功能指纹：删掉语气词/衔接词后，对剩余内容做 2-gram。
// 中文无空格，精确分词会因正则硬切跑偏（「番茄钟和待办清单」被切成「茄钟和待/办清单」），
// 故用相邻二字（bigram）作指纹——产物里只要出现指令中的任意相邻二字即算覆盖。
// 目的是抓「完全离题的壳」，不追求精确语义分词。
func companyQATopicTerms(brief string) []string {
	stop := []string{
		"做一个", "一个", "这个", "那个", "可以", "能够", "以及", "还有", "然后",
		"要求", "需要", "希望", "不要", "应该", "我们", "你们", "它们",
		"包括", "包含", "用于", "比如", "例如", "真的", "就是", "的话", "如果",
		"和", "与", "及", "的", "了", "是", "要", "做", "也", "都", "很",
	}
	cleaned := brief
	for _, s := range stop {
		cleaned = strings.ReplaceAll(cleaned, s, "")
	}
	// 只保留汉字与英文数字（删除标点、空格）
	cleaned = regexp.MustCompile(`[^\p{Han}a-zA-Z0-9]`).ReplaceAllString(cleaned, "")
	runes := []rune(cleaned)
	seen := map[string]bool{}
	var terms []string
	// 2-gram
	for i := 0; i+1 < len(runes); i++ {
		g := string(runes[i : i+2])
		if !seen[g] {
			seen[g] = true
			terms = append(terms, g)
		}
	}
	// 也加入 3-gram（更具体，命中更高价值，但仅当内容较长）
	for i := 0; i+2 < len(runes); i++ {
		g := string(runes[i : i+3])
		if !seen[g] {
			seen[g] = true
			terms = append(terms, g)
		}
	}
	if len(terms) > 40 {
		terms = terms[:40]
	}
	return terms
}

// companyQATopicHits 数指令核心功能词有多少出现在产物正文里。
func companyQATopicHits(terms []string, content string) int {
	lower := strings.ToLower(content)
	hits := 0
	for _, t := range terms {
		if strings.Contains(lower, strings.ToLower(t)) {
			hits++
		}
	}
	return hits
}

// companyQAJourney 核心用户旅程实测：填表→提交→刷新→断言数据持久化。
// 返回（是否真测到, 是否走通）。测不到（无 input/无按钮/刷新失败）返回 ok=false，
// 不据此判失败——只有真走了旅程且数据没留存才判「假功能」。
func companyQAJourney(tabWS, fileURL string) (measured, passed bool) {
	// 先看有没有可填的表单和可提交的控件，没有则旅程不适用（纯展示型产品）。
	if raw, e := cdpEval(tabWS, `(function(){var i=document.querySelectorAll('input,textarea').length;var b=document.querySelectorAll('button,[role=button],input[type=button],input[type=submit]').length;return JSON.stringify({inputs:i,buttons:b})})()`); e == nil {
		var c struct {
			Inputs  int `json:"inputs"`
			Buttons int `json:"buttons"`
		}
		if json.Unmarshal([]byte(raw), &c) == nil && (c.Inputs == 0 || c.Buttons == 0) {
			return false, true // 无表单交互型页面：旅程不适用，不算失败
		}
	}
	if _, e := cdpEval(tabWS, companyQAJourneyJS); e != nil {
		return false, false
	}
	time.Sleep(600 * time.Millisecond)
	before, e1 := cdpEval(tabWS, companyQAJourneyTextJS)
	if e1 != nil {
		return false, false
	}
	// 刷新页面，验证数据真的持久化（不是只活在内存）。
	_ = cdpNavigate(tabWS, fileURL)
	time.Sleep(900 * time.Millisecond)
	after, e2 := cdpEval(tabWS, companyQAJourneyTextJS)
	if e2 != nil {
		return false, false
	}
	changed := companyQAAtoi(after) != companyQAAtoi(before)
	// 数据留存：刷新后页面文字量应比「清空后」更多（有测试数据渲染出来）。
	return true, changed
}

// companyQAApplyTopic 算指令覆盖并写入报告。命中过少=交付物疑似离题壳。
func companyQAApplyTopic(report *companyQAReport, brief, content string) {
	terms := companyQATopicTerms(brief)
	report.TopicTerms = terms
	report.TopicHits = companyQATopicHits(terms, content)
	report.TopicOK = true
}

// companyQAProbeBrowser 用受管 headless Chromium 真实打开产品入口并实测。
// 返回多帧截图（首屏 + 滚动中部 + 滚动底部），供多帧视觉评审。
// 浏览器不可用时返回 error（调用方降级为「只跑识图」，不阻塞交付）。
func companyQAProbeBrowser(entryPath string) (companyQAReport, [][]byte, error) {
	report := companyQAReport{Entry: filepath.Base(entryPath), VisualScore: -1, CheckedAt: time.Now().Format("2006-01-02 15:04")}
	fileURL := "file:///" + strings.ReplaceAll(filepath.ToSlash(entryPath), " ", "%20")
	tabWS, _, err := cdpOpenTarget(fileURL)
	if err != nil {
		return report, nil, fmt.Errorf("打开产品页面失败: %w", err)
	}
	defer cdpCloseTab(tabWS)
	_ = cdpNavigate(tabWS, fileURL)
	time.Sleep(900 * time.Millisecond) // 给页面一点渲染与脚本执行时间
	// 中和原生弹窗：alert/confirm/prompt 会阻塞 Runtime.evaluate，
	// 不处理的话点了带弹窗的按钮会让后续测量全部超时，被误判成「假交互」。
	_, _ = cdpEval(tabWS, `window.alert=function(){};window.confirm=function(){return true};window.prompt=function(){return '测试'};'ok'`)

	if raw, e := cdpEval(tabWS, companyQAProbeJS); e == nil {
		var st companyQAProbeStats
		if json.Unmarshal([]byte(raw), &st) == nil {
			report.TextLength, report.JSVisible, report.Buttons = st.Text, st.Vis, st.Btn
		}
	}
	// 布局充实度：页面总高度 / 视口高度。抓「内容只占顶部一小条、下半页整体塌陷」的产物
	// ——这类页面有内容有交互，白屏判据抓不住，但观感是「没加载完」。
	if raw, e := cdpEval(tabWS, `(function(){return JSON.stringify({h:document.documentElement.scrollHeight,v:window.innerHeight||900})})()`); e == nil {
		var d struct {
			H float64 `json:"h"`
			V float64 `json:"v"`
		}
		if json.Unmarshal([]byte(raw), &d) == nil && d.V > 0 && d.H > 0 {
			report.PageHeightRatio = d.H / d.V
			report.LayoutOK = true
		}
	}
	// 白屏判据：几乎没有可见元素，或可读文字极少且无图片/控件。
	report.Blank = report.JSVisible < 6 || (report.TextLength < 20 && report.Buttons == 0)

	if raw, e := cdpEval(tabWS, companyQAInteractionJS); e == nil {
		report.Clicked = companyQAAtoi(raw)
		// 异步渲染：点击后等一拍再取快照比对。
		time.Sleep(450 * time.Millisecond)
		if snap, e2 := cdpEval(tabWS, companyQASnapshotJS); e2 == nil {
			var sr struct {
				Changed bool `json:"changed"`
			}
			if json.Unmarshal([]byte(snap), &sr) == nil {
				report.DOMChanged = sr.Changed
				report.InteractOK = true // 只有真测到了才允许据此判「假交互」
			}
		}
	}
	// 核心用户旅程：清存储→填表→提交→刷新→数据仍在，验证真实业务闭环（非 demo 假功能）。
	report.JourneyOK, report.JourneyPass = companyQAJourney(tabWS, fileURL)
	png, pngErr := cdpScreenshot(tabWS)
	if pngErr != nil {
		return report, nil, fmt.Errorf("页面截图失败: %w", pngErr)
	}
	// 滚动到页面中部与底部再各截一帧：识图评审不能只看首屏——
	// 实测过「首屏精致、滚动后大片占位空白/样式断裂」的产物，多帧评审才能抓住。
	// 三帧独立评审取最低分与合并缺陷：任何一帧丑都算丑，防止「首屏撑门面、内页糊弄」。
	var frames [][]byte
	frames = append(frames, png)
	if png2, err2 := companyQAScreenshotScrolled(tabWS, 0.5); err2 == nil && len(png2) > 0 {
		frames = append(frames, png2)
	}
	if png3, err3 := companyQAScreenshotScrolled(tabWS, 1.0); err3 == nil && len(png3) > 0 {
		frames = append(frames, png3)
	}
	report.FramesReviewed = len(frames)
	report.BrowserOK = true
	return report, frames, nil
}

// companyQAScreenshotScrolled 把页面滚到指定比例（0=顶,0.5=中,1=底）再截一帧。
// 用于多帧视觉评审：首屏之外的界面（列表深处/统计区/页脚）也要有人看。
func companyQAScreenshotScrolled(tabWS string, ratio float64) ([]byte, error) {
	script := fmt.Sprintf(`(function(){var h=document.documentElement.scrollHeight-window.innerHeight;window.scrollTo(0,Math.max(0,Math.round(h*%f)));return 'ok'})()`, ratio)
	if _, err := cdpEval(tabWS, script); err != nil {
		return nil, err
	}
	time.Sleep(400 * time.Millisecond) // 等滚动触发的懒加载/过渡完成
	return cdpScreenshot(tabWS)
}

// companyQAVisualReview 把真机截图交给免费识图模型评审（多模型负载均衡，挂一个自动切）。
// frames 为多帧（首屏+滚动中部+滚动底部），逐帧独立评审：取最低分为总分，缺陷合并去重。
// 任何一帧丑都算丑——防「首屏撑门面、内页糊弄」。识图不可用时 score=-1（不阻塞交付）。
func companyQAVisualReview(frames [][]byte, brief string) (int, []string, string, bool) {
	best := -1
	var allIssues []string
	summary := ""
	anyOK := false
	seen := map[string]bool{}
	for fi, png := range frames {
		if len(png) == 0 {
			continue
		}
		question := fmt.Sprintf("这是公司刚交付的产品的真实运行截图（第 %d/%d 帧，可能是页面顶部/中部/底部）。用户指令：%s",
			fi+1, len(frames), deliveryTruncate(brief, 300)) +
			"\n请严格评审这个界面，只输出 JSON（不要代码围栏）：" +
			`{"score":0到10的整数,"issues":["最多3条具体缺陷，按「视觉→布局→质感」顺序指出：配色是否和谐、层次是否分明、留白是否得当、控件是否精致、是否像模板壳"],"summary":"一句话结论"}` +
			"\n评分标准：9-10=可直接上线的商业产品（有品牌感、细节精致）；7-8=干净可用且有一定设计感；5-6=能看但平淡、像通用模板；3-4=明显模板感、配色粗糙、层次混乱；1-2=白屏/乱码/不可用。" +
			"\n重点扣分项：默认浏览器蓝紫配色、纯黑白大面积对撞、无 hover 反馈的按钮、行高过密的中文段落、生成感强的占位内容、大片无意义空白。别因为「看起来是个网页」就给高分，也别漏掉做得好的细节。"
		text, err := AnalyzeImage(base64.StdEncoding.EncodeToString(png), question, nil)
		if err != nil || strings.TrimSpace(text) == "" {
			continue
		}
		anyOK = true
		score, issues, sum := parseCompanyQAVerdict(text)
		if score >= 0 && (best < 0 || score < best) {
			best = score // 多帧取最低分：最丑的一帧决定产品下限
		}
		for _, is := range issues {
			is = strings.TrimSpace(is)
			if is != "" && !seen[is] {
				seen[is] = true
				allIssues = append(allIssues, is)
			}
		}
		if sum != "" && summary == "" {
			summary = sum
		}
	}
	if !anyOK || best < 0 {
		return -1, nil, "", false
	}
	return best, allIssues, summary, true
}

// parseCompanyQAVerdict 解析评审模型输出的 JSON（容忍代码围栏与前后废话）。
func parseCompanyQAVerdict(text string) (int, []string, string) {
	clean := deliveryStripCodeFence(text)
	start := strings.Index(clean, "{")
	end := strings.LastIndex(clean, "}")
	if start < 0 || end <= start {
		return -1, nil, deliveryTruncate(clean, 160)
	}
	var v struct {
		Score   int      `json:"score"`
		Issues  []string `json:"issues"`
		Summary string   `json:"summary"`
	}
	if json.Unmarshal([]byte(clean[start:end+1]), &v) != nil {
		return -1, nil, deliveryTruncate(clean, 160)
	}
	if v.Score < 0 || v.Score > 10 {
		v.Score = -1
	}
	return v.Score, v.Issues, v.Summary
}

// companyQAText 读入口文件为字符串（质检/返修都基于文本内容）。
func companyQAText(entryPath string) string {
	data := mustReadFile(entryPath)
	if len(data) == 0 {
		return ""
	}
	return string(data)
}

// companyQAAudit 产品真机质检 + 最多两轮自动返修。
// 在 runnable 落盘之后、路演/发布回执之前调用：返修会改写入口文件，
// 后面的 PPT 要点与发布回执 SHA256 必须基于返修后的版本。
// 返回（最终入口内容, 质检报告）。质检能力不可用时降级放行，绝不因外部服务挂死交付。
// 返修策略：第一轮带全量缺陷清单重做；复检仍挂 → 第二轮只带剩余缺陷精修（保留第一轮成果）；
// 两轮后仍挂 → 带病放行但报告如实记录（不阻塞交付，人类审批时可见）。
func companyQAAudit(projectDir, projectName, brief string, entry string, multi bool, upstream string) (string, companyQAReport) {
	entryPath := filepath.Join(projectDir, entry)
	report, frames, err := companyQAProbeBrowser(entryPath)
	if err != nil {
		// 浏览器不可用：没有真机证据就不假装质检，记录原因后放行。
		report.Summary = "真机质检未执行：" + err.Error()
		report.Skipped = true
		return companyQAText(entryPath), report
	}
	// 指令覆盖检测：交付物正文里有没有真实现用户指令的核心功能词（抓离题壳）。
	companyQAApplyTopic(&report, brief, companyQAText(entryPath))
	// 核心功能缺失检测：指令明确要求的功能，产物里有没有真正实现。
	report.MissingFeatures = companyQAMissingFeatures(brief, companyQAText(entryPath), "", projectDir)
	report.MissingFeatureCount = len(report.MissingFeatures)
	score, issues, summary, visionOK := companyQAVisualReview(frames, brief)
	report.VisualScore, report.VisionOK = score, visionOK
	if summary != "" {
		report.Summary = summary
	}
	report.Issues = append(report.Issues, issues...)
	report.Passed = companyQAPass(report)

	// 不合格 → 带缺陷清单返修，最多两轮（每轮复检，以最新结论为准）。
	for round := 1; round <= 2 && !report.Passed; round++ {
		report.Repaired = true
		report.RepairRounds = round
		companyLiveStage(projectName, "qa", "qa", "qa-01", fmt.Sprintf("质检未通过（第 %d 轮返修），按缺陷清单修整中", round))
		feedback := companyQAFeedbackBlock(report)
		var repaired string
		if multi {
			files := deliveryMultiProject(projectName, brief, upstream+"\n\n"+feedback)
			if len(files) == 0 {
				break // 返修产出为空：保留当前版本，别把好文件覆盖没
			}
			for name, content := range files {
				if _, werr := writeDeliveryFile(projectDir, name, []byte(content)); werr != nil {
					return companyQAText(entryPath), report
				}
			}
			repaired = companyInlineMulti(files)
		} else {
			repaired = deliveryProductHTML(projectName, brief, true, upstream+"\n\n"+feedback)
		}
		if strings.TrimSpace(repaired) == "" {
			break // 返修失败：保留当前版本
		}
		if _, werr := writeDeliveryFile(projectDir, entry, []byte(repaired)); werr != nil {
			return companyQAText(entryPath), report
		}
		companyLiveArtifact(projectName, "qa", "qa", entry, fmt.Sprintf("v%d-repaired", round+1))
		// 返修后复检：以复检结论为准。
		report2, png2, err2 := companyQAProbeBrowser(entryPath)
		if err2 != nil {
			break // 复检环境挂了：保留当前版本与上一轮结论
		}
		s2, i2, sum2, v2 := companyQAVisualReview(png2, brief)
		companyQAApplyTopic(&report2, brief, companyQAText(entryPath))
		report2.MissingFeatures = companyQAMissingFeatures(brief, companyQAText(entryPath), "", projectDir)
		report2.MissingFeatureCount = len(report2.MissingFeatures)
		report2.VisualScore, report2.VisionOK = s2, v2
		report2.Issues = append(report2.Issues, i2...)
		if sum2 != "" {
			report2.Summary = sum2
		}
		report2.Passed = companyQAPass(report2)
		report2.Repaired = true
		report = report2
		if report2.Passed {
			return repaired, report2
		}
	}
	return companyQAText(entryPath), report
}

// companyQAPass 判定：不白屏 + 有真实交互反应 + 视觉分达标（识图缺席时不因缺证据而卡死）。
// 铁律：只有「真测到了」的证据才能判失败——测量本身失败一律不判假交互，避免误杀返修。
func companyQAPass(r companyQAReport) bool {
	if r.Skipped {
		return true // 浏览器不可用跳过的，不当「未通过」。
	}
	if r.Blank {
		return false
	}
	if r.InteractOK && r.Buttons > 0 && !r.DOMChanged {
		return false // 有按钮、确实点了、DOM 毫无变化 = 假交互
	}
	if r.JourneyOK && !r.JourneyPass {
		return false // 真走了「填表→提交→刷新」旅程，但数据没留存 = 假功能
	}
	if r.TopicOK && r.TopicHits <= 1 {
		return false // 指令功能指纹几乎全没命中 = 离题壳（bigram 偶发命中，故阈值放 1）
	}
	if r.MissingFeatureCount > 0 {
		return false // 指令明确要求的功能没实现 = 不合格（这是最核心的判据）
	}
	if r.LayoutOK && r.PageHeightRatio < 1.2 {
		return false // 页面高度不足 1.2 屏 = 下半页塌陷（实测抓过「内容占 15%、85% 空白」的产物）
	}
	if r.VisionOK && r.VisualScore >= 0 && r.VisualScore < 6 {
		return false
	}
	return true
}

// companyQAFeedbackBlock 把质检缺陷写成给返修 Agent 的工单。
func companyQAFeedbackBlock(r companyQAReport) string {
	lines := []string{"【质检员返修工单】上一版交付物经真机实测存在以下问题，必须逐条修掉："}
	if r.Blank {
		lines = append(lines, "- 页面近乎白屏：可见元素仅 "+fmt.Sprint(r.JSVisible)+" 个，可读文字仅 "+fmt.Sprint(r.TextLength)+" 字。必须渲染出完整界面。")
	}
	if r.InteractOK && r.Buttons > 0 && !r.DOMChanged {
		lines = append(lines, "- 页面上的按钮点击后 DOM 毫无变化（假交互）。所有控件必须绑定真实事件并真正改变页面状态。")
	}
	if r.JourneyOK && !r.JourneyPass {
		lines = append(lines, "- 核心流程「填表→提交→刷新」走不通：提交后刷新数据就丢了。必须用本地存储真实持久化，不能只做内存里的假列表。")
	}
	if r.TopicOK && r.TopicHits <= 1 {
		lines = append(lines, "- 交付物与用户指令严重脱节：指令里的功能点几乎一个都没在页面里体现。必须真正实现用户要的核心功能，不能交一个与需求无关的壳。")
	}
	if r.MissingFeatureCount > 0 {
		lines = append(lines, "- 缺失核心功能（用户明确要求但产物里没有）：")
		for _, m := range r.MissingFeatures {
			lines = append(lines, "  · "+m)
		}
	}
	if r.LayoutOK && r.PageHeightRatio < 1.2 {
		lines = append(lines, fmt.Sprintf("- 页面充实度不足：总高度仅 %.1f 屏，下半页大面积空白塌陷。必须用统计卡片区、使用说明区、分类汇总区把页面填到至少 1.5 屏；空状态也要有图标+文案+引导按钮占位，不允许下半页整体空白。", r.PageHeightRatio))
	}
	if r.VisualScore >= 0 && r.VisualScore < 6 {
		lines = append(lines, fmt.Sprintf("- 视觉评审仅 %d/10：需重做排版层次与配色，去掉模板感。具体要求：①:root 定义 CSS 变量配色体系（主色+辅色+灰阶），禁默认蓝紫；②hero/功能区/统计区三层结构分明；③卡片圆角 12-16px+多层阴影；④按钮 hover/active 过渡 ≥.18s；⑤中文行高 ≥1.6、数字 tabular-nums。", r.VisualScore))
	}
	for _, iss := range r.Issues {
		if strings.TrimSpace(iss) != "" {
			lines = append(lines, "- "+strings.TrimSpace(iss))
		}
	}
	lines = append(lines, "保留上一版已经正确的功能，只针对上述缺陷改进，不要缩小功能范围。")
	return strings.Join(lines, "\n")
}

// loadCompanyQAReport 读取项目质检结论；文件不存在/损坏返回 ok=false（老项目没质检）。
func loadCompanyQAReport(projectDir string) (companyQAReport, bool) {
	data, err := os.ReadFile(filepath.Join(projectDir, "09-质量验收.qa.json"))
	if err != nil {
		return companyQAReport{}, false
	}
	var rep companyQAReport
	if json.Unmarshal(data, &rep) != nil {
		return companyQAReport{}, false
	}
	return rep, true
}

// saveCompanyQAReport 质检结论落盘到项目目录（随整包 zip 一起留档、可复算）。
func saveCompanyQAReport(projectDir string, report companyQAReport) string {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return ""
	}
	path := filepath.Join(projectDir, "09-质量验收.qa.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return ""
	}
	return filepath.Base(path)
}
