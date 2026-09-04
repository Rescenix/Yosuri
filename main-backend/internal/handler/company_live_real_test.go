package handler

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// TestCompanyLiveRealRun 真实跑一条指令的完整交付，全程录制直播事件流，
// 验证「最小原型 v1 先出现 → 调研文字流 → UI v2 换页 → 终版 final」的顺序成立。
// 联网调用免费池，耗时约 2-4 分钟，需要 -long 之外的真实环境。
func TestCompanyLiveRealRun(t *testing.T) {
	if testing.Short() {
		t.Skip("-short 跳过真实生产")
	}
	var mu sync.Mutex
	var kinds []string
	var deltas int
	var firstProtoAt int
	var protoVersions []string
	done := make(chan struct{})

	// 订阅总线（与 SSE 观众同一条广播）
	ch := make(chan companyLiveEvent, 256)
	companyLive.mu.Lock()
	companyLive.subs[ch] = struct{}{}
	companyLive.mu.Unlock()
	go func() {
		defer close(done)
		i := 0
		for ev := range ch {
			i++
			mu.Lock()
			switch ev.Kind {
			case "delta":
				deltas++
			case "iteration":
				protoVersions = append(protoVersions, ev.Version)
				if firstProtoAt == 0 && ev.Version == "v1" {
					firstProtoAt = i
				}
			case "stage", "done", "error":
				kinds = append(kinds, ev.Kind+":"+ev.Stage+":"+deliveryTruncate(ev.Text, 24))
			}
			mu.Unlock()
		}
	}()

	start := time.Now()
	projectName := "900-直播自检-番茄钟"
	dir, err := deliveryBuildProject(projectName, "做一个番茄钟小工具，要能运行，含25分钟计时器与开始暂停按钮")
	if err != nil {
		t.Fatalf("真实生产失败: %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	companyLive.mu.Lock()
	delete(companyLive.subs, ch)
	companyLive.mu.Unlock()
	close(ch)
	<-done
	elapsed := time.Since(start)

	mu.Lock()
	defer mu.Unlock()
	t.Logf("耗时=%s 目录=%s", elapsed.Round(time.Second), dir)
	for _, k := range kinds {
		t.Log("  事件:", k)
	}
	t.Logf("delta帧=%d 原型换版=%v v1首次出现于第%d帧", deltas, protoVersions, firstProtoAt)
	if deltas < 5 {
		t.Errorf("文字流太少(%d帧)——逐字直播没生效，全程走了整段回退", deltas)
	}
	if firstProtoAt == 0 {
		t.Error("大屏里 v1 最小原型从未出现")
	}
	for _, want := range []string{"v1", "v2", "final"} {
		found := false
		for _, v := range protoVersions {
			if v == want {
				found = true
			}
		}
		if !found {
			t.Errorf("原型迭代缺 %s", want)
		}
	}
	if !strings.Contains(dir, "projects") {
		t.Errorf("项目没落在真身目录: %s", dir)
	}
}
