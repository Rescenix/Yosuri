package handler

import (
	"strings"
	"testing"
)

func TestParseCompanyQAVerdict(t *testing.T) {
	cases := []struct {
		name      string
		in        string
		wantScore int
		wantIssue int
	}{
		{"纯JSON", `{"score":8,"issues":["对比度偏低"],"summary":"干净可用"}`, 8, 1},
		{"带围栏", "```json\n{\"score\":3,\"issues\":[\"白屏\",\"无样式\"],\"summary\":\"不可用\"}\n```", 3, 2},
		{"带废话", "评审如下：{\"score\":7,\"issues\":[],\"summary\":\"可上线\"} 以上。", 7, 0},
		{"分数越界", `{"score":99,"issues":[],"summary":"x"}`, -1, 0},
		{"非JSON", "这个界面挺好的", -1, 0},
	}
	for _, c := range cases {
		score, issues, _ := parseCompanyQAVerdict(c.in)
		if score != c.wantScore {
			t.Errorf("%s: score=%d want %d", c.name, score, c.wantScore)
		}
		if len(issues) != c.wantIssue {
			t.Errorf("%s: issues=%d want %d", c.name, len(issues), c.wantIssue)
		}
	}
}

func TestCompanyQAPass(t *testing.T) {
	cases := []struct {
		name string
		rep  companyQAReport
		want bool
	}{
		{"白屏必挂", companyQAReport{Blank: true, BrowserOK: true}, false},
		{"假交互必挂", companyQAReport{InteractOK: true, JSVisible: 40, TextLength: 900, Buttons: 5, DOMChanged: false, VisionOK: true, VisualScore: 9}, false},
		{"交互未测到不判挂", companyQAReport{InteractOK: false, JSVisible: 40, TextLength: 900, Buttons: 5, DOMChanged: false, VisionOK: false, VisualScore: -1}, true},
		{"视觉低分挂", companyQAReport{JSVisible: 40, TextLength: 900, Buttons: 3, DOMChanged: true, VisionOK: true, VisualScore: 4}, false},
		{"旅程未走通挂", companyQAReport{JSVisible: 50, TextLength: 900, InteractOK: true, Buttons: 4, DOMChanged: true, JourneyOK: true, JourneyPass: false}, false},
		{"旅程未测到不挂", companyQAReport{JSVisible: 50, TextLength: 900, Buttons: 4, DOMChanged: true, JourneyOK: false, VisionOK: true, VisualScore: 8}, true},
		{"离题壳挂", companyQAReport{JSVisible: 50, TextLength: 900, Buttons: 4, DOMChanged: true, JourneyOK: true, JourneyPass: true, TopicOK: true, TopicHits: 0}, false},
		{"浏览器跳过不挂", companyQAReport{Skipped: true}, true},
		{"识图缺席不卡", companyQAReport{JSVisible: 40, TextLength: 900, Buttons: 3, DOMChanged: true, VisionOK: false, VisualScore: -1}, true},
		{"无按钮但渲染完整", companyQAReport{JSVisible: 30, TextLength: 1200, Buttons: 0, VisionOK: true, VisualScore: 8}, true},
		{"全过", companyQAReport{JSVisible: 60, TextLength: 2000, Buttons: 6, Clicked: 4, DOMChanged: true, VisionOK: true, VisualScore: 8}, true},
	}
	for _, c := range cases {
		if got := companyQAPass(c.rep); got != c.want {
			t.Errorf("%s: pass=%v want %v", c.name, got, c.want)
		}
	}
}

func TestCompanyQATopicTerms(t *testing.T) {
	terms := companyQATopicTerms("做一个番茄钟和待办清单，记录收支，按月统计")
	joined := strings.Join(terms, " ")
	// 语气词/单字必须被过滤，不能成为独立指纹
	for _, bad := range []string{"做一个", "和", "的", "可以"} {
		for _, tm := range terms {
			if tm == bad {
				t.Errorf("语气词 %q 不该进入功能指纹 terms=%v", bad, terms)
			}
		}
	}
	// 核心功能二字指纹必须在
	for _, want := range []string{"番茄", "待办", "收支", "统计"} {
		if !strings.Contains(joined, want) {
			t.Errorf("功能指纹缺失 %q terms=%v", want, terms)
		}
	}
	// 命中计数：产物覆盖了功能词时 hits>0；完全没覆盖时 0。
	if got := companyQATopicHits(terms, "这是我的番茄钟，有待办清单"); got < 2 {
		t.Errorf("覆盖率统计偏低: got=%d terms=%v", got, terms)
	}
	if got := companyQATopicHits(terms, "欢迎使用,这是一个模板"); got != 0 {
		t.Errorf("离题产物不该命中功能词: got=%d", got)
	}
}

func TestCompanyQAReportRoundTrip(t *testing.T) {
	dir := t.TempDir()
	in := companyQAReport{
		CheckedAt: "2026-09-05 20:00", Entry: "output-app.html",
		JSVisible: 42, TextLength: 1500, Buttons: 6, Clicked: 4, DOMChanged: true,
		VisualScore: 8, Issues: []string{"标题对比度偏低"}, Summary: "干净可用",
		Passed: true, BrowserOK: true, VisionOK: true,
	}
	name := saveCompanyQAReport(dir, in)
	if name != "09-质量验收.qa.json" {
		t.Fatalf("落盘文件名异常: %q", name)
	}
	out, ok := loadCompanyQAReport(dir)
	if !ok {
		t.Fatal("读回失败")
	}
	if out.VisualScore != 8 || !out.Passed || !out.DOMChanged || out.Clicked != 4 {
		t.Errorf("读回字段漂移: %+v", out)
	}
	if len(out.Issues) != 1 || out.Issues[0] != "标题对比度偏低" {
		t.Errorf("缺陷清单丢失: %+v", out.Issues)
	}
	// 未质检的老项目目录必须返回 ok=false，而不是零值冒充通过。
	if _, ok := loadCompanyQAReport(t.TempDir()); ok {
		t.Error("无质检文件却返回 ok，前端会误显示通过徽章")
	}
}

func TestCompanyQAFeedbackBlock(t *testing.T) {
	rep := companyQAReport{
		Blank: false, JSVisible: 40, TextLength: 900, Buttons: 5, DOMChanged: false, InteractOK: true,
		VisualScore: 3, VisionOK: true, Issues: []string{"配色单调"},
	}
	block := companyQAFeedbackBlock(rep)
	for _, want := range []string{"假交互", "视觉评审仅 3/10", "配色单调", "不要缩小功能范围"} {
		if !strings.Contains(block, want) {
			t.Errorf("返修工单缺 %q:\n%s", want, block)
		}
	}
	if strings.Contains(block, "白屏") {
		t.Error("非白屏却写了白屏缺陷，会误导返修 Agent")
	}
}
