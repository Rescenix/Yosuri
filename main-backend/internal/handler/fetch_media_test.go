package handler

// fetch_media_test.go —— fetch_media 工件产出回归（09-10 修复：音频/视频重复弹
// 交付卡 + 卡片绝对路径过不了 /api/agent/file 审批注册表必 400；09-14 改版：
// 只收本地路径，不再收 http(s) URL）。
// 枚举 callNativeFetchMedia 实际会产出的各媒体形态，钉死：
// ① 音频/视频只产播放工件，绝不产 Files；② 文档类交付卡必须带 /api/media 直链；
// ③ http(s) URL 输入必须报错（引导先下载到本地，杜绝起 HTTP 服务绕圈）。

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCallNativeFetchMedia_AudioNoFileCard(t *testing.T) {
	mediaDir := t.TempDir()
	t.Setenv("RESCENE_MEDIA_DIR", mediaDir)
	src := filepath.Join(t.TempDir(), "voice.wav")
	os.WriteFile(src, []byte("RIFF....WAVEfmt data"), 0o644)

	args, _ := json.Marshal(map[string]string{"path": src})
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
	if !strings.HasPrefix(res.Audios[0].URL, "/api/media/") {
		t.Errorf("音频工件必须带 /api/media 静态直链，得到 %q", res.Audios[0].URL)
	}
}

func TestCallNativeFetchMedia_VideoNoFileCard(t *testing.T) {
	t.Setenv("RESCENE_MEDIA_DIR", t.TempDir())
	src := filepath.Join(t.TempDir(), "clip.mp4")
	os.WriteFile(src, []byte{0, 0, 0, 0x18, 'f', 't', 'y', 'p'}, 0o644)

	args, _ := json.Marshal(map[string]string{"path": src})
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
	src := filepath.Join(t.TempDir(), "paper.pdf")
	os.WriteFile(src, []byte("%PDF-1.4 test"), 0o644)

	args, _ := json.Marshal(map[string]string{"path": src})
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

func TestCallNativeFetchMedia_RejectsRemoteURL(t *testing.T) {
	t.Setenv("RESCENE_MEDIA_DIR", t.TempDir())

	args, _ := json.Marshal(map[string]string{"path": "http://localhost:8080/canon.ogg"})
	_, err := callNativeFetchMedia(context.Background(), string(args))
	if err == nil {
		t.Fatal("http(s) URL 输入必须报错（fetch_media 只收本地路径，杜绝起 HTTP 服务绕圈）")
	}
	if !strings.Contains(err.Error(), "只收本地") {
		t.Errorf("报错应引导下载到本地，实际: %v", err)
	}
}

func TestCallNativeFetchMedia_MissingFile(t *testing.T) {
	t.Setenv("RESCENE_MEDIA_DIR", t.TempDir())

	args, _ := json.Marshal(map[string]string{"path": filepath.Join(t.TempDir(), "nope.mp3")})
	_, err := callNativeFetchMedia(context.Background(), string(args))
	if err == nil {
		t.Fatal("不存在的文件必须报错")
	}
}
