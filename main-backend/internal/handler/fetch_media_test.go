package handler

// fetch_media_test.go —— fetch_media 工件产出回归（09-10 修复：音频/视频重复弹
// 交付卡 + 卡片绝对路径过不了 /api/agent/file 审批注册表必 400）。
// 枚举 callNativeFetchMedia 实际会产出的各媒体形态，钉死：
// ① 音频/视频只产播放工件，绝不产 Files；② 文档类交付卡必须带 /api/media 直链。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCallNativeFetchMedia_AudioNoFileCard(t *testing.T) {
	t.Setenv("RESCENE_MEDIA_DIR", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/wav")
		w.Write([]byte("RIFF....WAVEfmt data"))
	}))
	defer srv.Close()

	args, _ := json.Marshal(map[string]string{"url": srv.URL + "/voice.wav"})
	res, err := callNativeFetchMedia(context.Background(), string(args))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Audios) != 1 {
		t.Fatalf("音频应产 1 个播放工件，得到 %d", len(res.Audios))
	}
	if len(res.Files) != 0 {
		t.Errorf("音频不应再产交付卡片（重复弹卡+绝对路径必400），得到 %d 个", len(res.Files))
	}
}

func TestCallNativeFetchMedia_VideoNoFileCard(t *testing.T) {
	t.Setenv("RESCENE_MEDIA_DIR", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		w.Write([]byte{0, 0, 0, 0x18, 'f', 't', 'y', 'p'})
	}))
	defer srv.Close()

	args, _ := json.Marshal(map[string]string{"url": srv.URL + "/clip.mp4"})
	res, err := callNativeFetchMedia(context.Background(), string(args))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Videos) != 1 {
		t.Fatalf("视频应产 1 个播放工件，得到 %d", len(res.Videos))
	}
	if len(res.Files) != 0 {
		t.Errorf("视频不应再产交付卡片，得到 %d 个", len(res.Files))
	}
}

func TestCallNativeFetchMedia_DocCarriesMediaURL(t *testing.T) {
	t.Setenv("RESCENE_MEDIA_DIR", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.Write([]byte("%PDF-1.4 test"))
	}))
	defer srv.Close()

	args, _ := json.Marshal(map[string]string{"url": srv.URL + "/paper.pdf"})
	res, err := callNativeFetchMedia(context.Background(), string(args))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Files) != 1 {
		t.Fatalf("文档应产 1 个交付卡，得到 %d", len(res.Files))
	}
	if !strings.HasPrefix(res.Files[0].URL, "/api/media/") {
		t.Errorf("交付卡必须带 /api/media 静态直链（前端预览/下载靠它绕开路径沙箱），得到 %q", res.Files[0].URL)
	}
}
