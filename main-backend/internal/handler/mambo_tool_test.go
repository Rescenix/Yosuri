package handler

// mambo_tool_test.go —— 曼波视频工具的真实端到端验证：
// 真实调用 callMamboVideo（edge-tts 配音 + SRT 字幕 + 素材匹配 + ffmpeg 合成），
// 并校验产物：mp4 可解码（h264+aac 1080x1920）、srt 非空、manifest 结构完整。

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCallMamboVideo_EndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("端到端测试跳过（短模式）")
	}
	ctx := context.Background()
	args, _ := json.Marshal(map[string]string{
		"topic": "曼波测试",
		"text":  "曼波曼波，跳舞啦！|音乐响起，节奏飞起！|跟我一起摇起来！",
		"out":   filepath.Join(os.TempDir(), "mambo_e2e_test.mp4"),
	})
	res, err := callMamboVideo(ctx, string(args))
	if err != nil {
		t.Fatalf("callMamboVideo 失败: %v", err)
	}
	if res.Text == "" {
		t.Fatal("返回为空")
	}

	// 解析引擎返回的 JSON
	var out struct {
		Ok       bool   `json:"ok"`
		Video    string `json:"video"`
		Srt      string `json:"srt"`
		Manifest string `json:"manifest"`
		Duration float64 `json:"duration"`
		Segments int    `json:"segments"`
	}
	if err := json.Unmarshal([]byte(res.Text), &out); err != nil {
		t.Fatalf("引擎返回不是 JSON: %v\n%s", err, res.Text)
	}
	if !out.Ok || out.Video == "" {
		t.Fatalf("引擎 ok=false: %s", res.Text)
	}
	if out.Segments < 3 {
		t.Fatalf("分段数异常: %d", out.Segments)
	}

	// 1) mp4 存在且可解码
	if _, err := os.Stat(out.Video); err != nil {
		t.Fatalf("视频不存在: %v", err)
	}
	ffprobe := exec.Command("ffprobe", "-v", "error",
		"-show_entries", "stream=codec_name,codec_type,width,height",
		"-of", "csv=p=0", out.Video)
	probeOut, err := ffprobe.CombinedOutput()
	if err != nil {
		t.Fatalf("ffprobe 失败: %v\n%s", err, probeOut)
	}
	probe := string(probeOut)
	for _, want := range []string{"h264", "aac", "1080", "1920"} {
		if !strings.Contains(probe, want) {
			t.Errorf("视频流缺 %s，实际: %s", want, probe)
		}
	}

	// 2) srt 存在且非空（至少 3 条字幕）
	srtBytes, err := os.ReadFile(out.Srt)
	if err != nil {
		t.Fatalf("srt 读取失败: %v", err)
	}
	if len(srtBytes) < 100 {
		t.Fatalf("srt 内容过短: %d bytes", len(srtBytes))
	}
	subs := strings.Count(string(srtBytes), "-->")
	if subs < 3 {
		t.Fatalf("字幕条目过少: %d", subs)
	}

	// 3) manifest 结构完整
	manBytes, err := os.ReadFile(out.Manifest)
	if err != nil {
		t.Fatalf("manifest 读取失败: %v", err)
	}
	var man struct {
		Topic    string `json:"topic"`
		Segments []struct {
			Sentence string `json:"sentence"`
			Source   string `json:"source"`
		} `json:"segments"`
	}
	if err := json.Unmarshal(manBytes, &man); err != nil {
		t.Fatalf("manifest 解析失败: %v", err)
	}
	if len(man.Segments) != out.Segments {
		t.Errorf("manifest 分段数不一致: %d != %d", len(man.Segments), out.Segments)
	}
	for _, s := range man.Segments {
		if s.Sentence == "" || s.Source == "" {
			t.Errorf("manifest 段信息缺失: %+v", s)
		}
	}

	t.Logf("PASS: video=%s (%.1fs, %d 段), srt=%s", out.Video, out.Duration, out.Segments, out.Srt)
}
