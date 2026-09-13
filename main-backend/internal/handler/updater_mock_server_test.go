package handler

// 模拟更新服务器端到端实测（2026-09-13）：
// 自包含 httptest 服务器，走 checkUpdate → downloadHotPatchZip 真实全流程。
// 初版曾依赖本机常驻 mock（127.0.0.1:18234），换机器/CI 跑必挂（技术债），
// 2026-09-13 改为测试内自建服务器，无外部依赖。

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// newMockReleaseServer 自建模拟更新源：update.json + portable.zip。
// 假 exe = MZ 头 + ≥1MB + 版本串，满足 isLikelyWindowsExecutable / exeContainsVersion。
func newMockReleaseServer(t *testing.T, version string) *httptest.Server {
	t.Helper()

	exe := make([]byte, 1024*1024+4096)
	copy(exe, "MZ")
	copy(exe[1024*1024:], []byte("v"+version))

	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	w, err := zw.Create("rescene.exe")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(exe); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/update.json", func(w http.ResponseWriter, r *http.Request) {
		rel := map[string]any{
			"name":             "v" + version,
			"tag_name":         "v" + version,
			"published_at":     "2026-09-13T08:00:00Z",
			"body":             "mock update source",
			"html_url":         r.Host,
			"download_url":     "http://" + r.Host + "/Rescene-windows-amd64-setup.exe",
			"download_url_zip": "http://" + r.Host + "/Rescene-windows-amd64-portable.zip",
		}
		_ = json.NewEncoder(w).Encode(rel)
	})
	mux.HandleFunc("/Rescene-windows-amd64-portable.zip", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(zipBuf.Bytes())
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// setAppVersionForTest 临时覆盖 AppVersion 并在测试结束后恢复原值。
// 同一包内测试共享全局变量，不恢复会污染其他测试（如 TestFetchRelease_buildsUpdateInfo）。
func setAppVersionForTest(t *testing.T, v string) {
	t.Helper()
	orig := AppVersion
	AppVersion = v
	t.Cleanup(func() { AppVersion = orig })
}

func resetUpdateCacheForTest() {
	updateCache = nil
	updateCachedAt = time.Time{}
}

func TestCheckUpdate_FromMockServer(t *testing.T) {
	resetUpdateCacheForTest()
	srv := newMockReleaseServer(t, "0.3.10-mock")
	setAppVersionForTest(t, "0.3.9")
	t.Setenv("RESCENE_UPDATE_URL", srv.URL+"/update.json")

	info, err := checkUpdate()
	if err != nil {
		t.Fatalf("checkUpdate 应成功: %v", err)
	}
	if info == nil {
		t.Fatal("checkUpdate 返回 nil")
	}
	if !info.HasUpdate {
		t.Fatalf("0.3.9 → v0.3.10-mock 应判定有更新: %+v", info)
	}
	if info.LatestVersion != "v0.3.10-mock" {
		t.Fatalf("LatestVersion 应为 v0.3.10-mock，实际 %q", info.LatestVersion)
	}
	if info.DownloadExe == "" {
		t.Fatal("热补丁 zip 地址不应为空")
	}
	t.Logf("✓ checkUpdate 命中 mock 更新源: latest=%s hot_patch=%s", info.LatestVersion, info.DownloadExe)
}

func TestDownloadHotPatch_FromMockServer(t *testing.T) {
	resetUpdateCacheForTest()
	srv := newMockReleaseServer(t, "0.3.10-mock")
	setAppVersionForTest(t, "0.3.9")
	t.Setenv("RESCENE_UPDATE_URL", srv.URL+"/update.json")

	info, err := checkUpdate()
	if err != nil || info == nil {
		t.Fatalf("checkUpdate 失败: %v", err)
	}

	dir := t.TempDir()
	dest := filepath.Join(dir, updateHotPatchFileName)
	if err := downloadHotPatchZip(info.DownloadExe, dest, info.LatestVersion); err != nil {
		t.Fatalf("downloadHotPatchZip 失败: %v", err)
	}

	fi, err := os.Stat(dest)
	if err != nil || fi.Size() < 1024*1024 {
		t.Fatalf("补丁 exe 未落盘或过小: size=%d err=%v", fi.Size(), err)
	}
	// 旁路版本标记应同步落盘（bug2 修复：下载完成即写目标版本）
	sv, err := os.ReadFile(filepath.Join(dir, updateHotPatchVersionFileName))
	if err != nil {
		t.Fatalf("rescene-new.version 应被写入: %v", err)
	}
	if string(bytes.TrimSpace(sv)) != "v0.3.10-mock" {
		t.Fatalf("sidecar 版本串错误: %q", sv)
	}
	t.Logf("✓ 模拟更新全链路成功: 下载 %s → 解压 → 补丁落盘 %s (%d bytes), sidecar=%s",
		info.DownloadExe, dest, fi.Size(), sv)
}

// TestHandleAutoDownload_StaleDoneAndOldResidual 防回归（2026-09-13 实测洞）：
// 磁盘残留旧版 rescene-new.exe（不含最新版本串）+ 内存 State=done 时，
// HandleAutoDownload 必须清残留强制重下，绝不能短路放行旧包（现象：一键安装装旧版）。
func TestHandleAutoDownload_StaleDoneAndOldResidual(t *testing.T) {
	resetUpdateCacheForTest()
	srv := newMockReleaseServer(t, "0.3.10-mock")
	setAppVersionForTest(t, "0.3.9")
	t.Setenv("RESCENE_UPDATE_URL", srv.URL+"/update.json")

	// 旧残留：MZ 头 + ≥1MB + 但版本串是旧的（不含 v0.3.10-mock）
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)
	updDir := filepath.Join(dir, "Rescene", "updates")
	if err := os.MkdirAll(updDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(updDir, updateHotPatchFileName)
	oldExe := make([]byte, 1024*1024+4096)
	copy(oldExe, "MZ")
	copy(oldExe[1024*1024:], []byte("v0.3.9-stale-residual"))
	if err := os.WriteFile(dest, oldExe, 0o755); err != nil {
		t.Fatal(err)
	}

	// 模拟进程内残留 done 态（旧代码的洞：内存 State=done 直接短路放行）
	updateDL.mu.Lock()
	updateDL.State = "done"
	updateDL.Path = dest
	updateDL.mu.Unlock()
	t.Cleanup(func() {
		updateDL.mu.Lock()
		updateDL.State = ""
		updateDL.Path = ""
		updateDL.mu.Unlock()
	})

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/update/download", nil)
	HandleAutoDownload(c)

	if w.Code != http.StatusOK {
		t.Fatalf("HandleAutoDownload status=%d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		State string `json:"state"`
		Path  string `json:"path"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	// 关键断言：内存 done + 旧残留 → 必须返回 downloading（正重新下载），不能是 done 放行旧包
	if resp.State == "done" && resp.Path == dest {
		t.Fatal("BUG 复现：内存 done 态短路放行了旧残留（一键安装会装旧版）")
	}

	// 等后台下载完成（最多 10s），最终产物应含最新版本串
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		updateDL.mu.Lock()
		st := updateDL.State
		updateDL.mu.Unlock()
		if st == "done" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	updateDL.mu.Lock()
	final := updateDL.State
	updateDL.mu.Unlock()
	if final != "done" {
		t.Fatalf("重新下载未完成，state=%s", final)
	}
	if !exeContainsVersion(dest, "v0.3.10-mock") {
		t.Fatalf("下载产物应包含 v0.3.10-mock 版本串（旧残留未被替换）")
	}
	t.Log("✓ 内存 done + 旧残留 → 强制重下，产物为新版本（旧包不再被放行安装）")
}
