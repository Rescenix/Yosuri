package handler

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// 复刻 CompanyView.vue recentEvents 的行解析正则：^\[([^\]]+)\]\s*(.*)$
var livlogFrontRe = regexp.MustCompile(`^\[([^\]]+)\]\s*(.*)$`)

// TestAppendLiveLog 验证把一行接力记录写进 agent live.log 后，能被前端「此刻正在发生」面板
// 和公司实时状态解析（完整日期时间戳）正确读取。用临时 agent 目录，跑完清理，不污染真实数据。
func TestAppendLiveLog(t *testing.T) {
	name := "__livlog_test__"
	agentHome := filepath.Join(companyDir(), name)
	_ = os.MkdirAll(agentHome, 0o755)
	defer os.RemoveAll(agentHome)

	const msg = "交接完成 · 交出 ui·03-UI原型.html"
	appendLiveLog(name, msg)

	data, err := os.ReadFile(filepath.Join(agentHome, "live.log"))
	if err != nil {
		t.Fatalf("读取 live.log 失败: %v", err)
	}
	line := strings.TrimSpace(string(data))
	if line == "" {
		t.Fatal("live.log 未写入任何内容")
	}

	// 1) 前端 recentEvents 的 ^\[header\]\s*(.*)$ 必须能取到 timeText 和 message
	m := livlogFrontRe.FindStringSubmatch(line)
	if m == nil {
		t.Fatalf("行 %q 不匹配前端解析正则 ^\\[([^^]+)\\]\\s*(.*)$", line)
	}
	timeText, rest := m[1], m[2]
	if strings.TrimSpace(rest) != msg {
		t.Fatalf("message 解析错误: got %q want %q", rest, msg)
	}

	// 2) 公司实时状态 liveLogTimeRe 必须把它当【完整日期】基准（否则会判成短时间戳/继承旧日期）
	ts := liveLogTimeRe.FindStringSubmatch(line)
	if len(ts) < 2 || len(ts[1]) <= 12 {
		t.Fatalf("行 %q 的时间戳不是完整日期格式: %+v", line, ts)
	}

	// 3) 时间戳格式一致性：展开成 [YYYY-MM-DD HH:MM]
	if !strings.HasPrefix(line, "["+ts[1]+"]") {
		t.Fatalf("行首时间戳与解析不一致: line=%q ts=%q", line, ts[1])
	}
	if !strings.HasPrefix(timeText+" ]", "["+ts[1]+" ]") && !strings.HasPrefix(line, "["+ts[1]+"]") {
		t.Fatalf("timeText=%q 与 liveLogTimeRe=%q 不一致", timeText, ts[1])
	}
}
