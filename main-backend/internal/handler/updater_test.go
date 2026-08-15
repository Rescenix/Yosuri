package handler

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractVersionPreservesSemVerSuffixes(t *testing.T) {
	tests := map[string]string{
		"v0.1.2-alpha.4":                    "0.1.2-alpha.4",
		"Release V1.2.3-rc.1+windows.amd64": "1.2.3-rc.1+windows.amd64",
		"ginnungagap_v0.0.4":                "0.0.4",
	}
	for input, want := range tests {
		if got := extractVersion(input); got != want {
			t.Errorf("extractVersion(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestCompareVersionsSemVerPrecedence(t *testing.T) {
	ascending := []string{
		"1.0.0-alpha",
		"1.0.0-alpha.1",
		"1.0.0-alpha.beta",
		"1.0.0-beta",
		"1.0.0-beta.2",
		"1.0.0-beta.11",
		"1.0.0-rc.1",
		"1.0.0",
	}
	for i := 1; i < len(ascending); i++ {
		cur, latest := ascending[i-1], ascending[i]
		if !compareVersions(cur, latest) {
			t.Errorf("expected %q to be newer than %q", latest, cur)
		}
		if compareVersions(latest, cur) {
			t.Errorf("did not expect %q to be newer than %q", cur, latest)
		}
	}
}

func TestCompareVersionsApplicationCases(t *testing.T) {
	tests := []struct {
		name        string
		cur, latest string
		want        bool
	}{
		{name: "next alpha", cur: "0.1.2-alpha.3", latest: "0.1.2-alpha.4", want: true},
		{name: "alpha to stable", cur: "0.1.2-alpha.4", latest: "0.1.2", want: true},
		{name: "next core prerelease", cur: "0.1.2", latest: "0.1.3-alpha.1", want: true},
		{name: "build metadata ignored", cur: "1.2.3+build.1", latest: "1.2.3+build.2", want: false},
		{name: "v prefix", cur: "v2.0.0-rc.1", latest: "V2.0.0", want: true},
		{name: "large numeric identifier", cur: "1.0.0-alpha.99999999999999999999", latest: "1.0.0-alpha.100000000000000000000", want: true},
		{name: "invalid current", cur: "1.0", latest: "1.0.1", want: false},
		{name: "invalid leading zero", cur: "1.0.0-alpha.01", latest: "1.0.0-alpha.2", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compareVersions(tt.cur, tt.latest); got != tt.want {
				t.Fatalf("compareVersions(%q, %q) = %v, want %v", tt.cur, tt.latest, got, tt.want)
			}
		})
	}
}

func TestFetchReleaseFromSiteJSON(t *testing.T) {
	// 模拟官网 update.json 响应
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"tag_name": "ginnungagap_v0.0.9",
			"name": "v0.1.3-alpha.1",
			"body": "聚合 API 首发！更新检查改为国内可达接口",
			"html_url": "https://github.com/Rescenix/ResceneAgent/releases/tag/ginnungagap_v0.0.9",
			"published_at": "2026-08-12T17:30:27Z",
			"download_url": "https://download.shanca.me/Rescene-windows-amd64-setup.exe"
		}`))
	}))
	defer srv.Close()

	rel, err := fetchRelease(srv.URL)
	if err != nil {
		t.Fatalf("fetchRelease from site: %v", err)
	}
	if rel.Name != "v0.1.3-alpha.1" {
		t.Errorf("Name = %q, want %q", rel.Name, "v0.1.3-alpha.1")
	}
	if rel.DownloadURL != "https://download.shanca.me/Rescene-windows-amd64-setup.exe" {
		t.Errorf("DownloadURL = %q, want download.shanca.me", rel.DownloadURL)
	}
	if rel.TagName != "ginnungagap_v0.0.9" {
		t.Errorf("TagName = %q, want ginnungagap_v0.0.9", rel.TagName)
	}
	if !strings.Contains(rel.Body, "聚合 API") {
		t.Errorf("Body should contain 聚合 API, got: %q", rel.Body)
	}
}

func TestFetchReleaseFromGitHub(t *testing.T) {
	// 模拟 GitHub API 响应（无 download_url 字段）
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"tag_name": "v0.1.2",
			"name": "v0.1.2 稳定版",
			"body": "稳定版发布",
			"html_url": "https://github.com/Rescenix/ResceneAgent/releases/tag/v0.1.2",
			"published_at": "2026-08-11T04:32:10Z"
		}`))
	}))
	defer srv.Close()

	rel, err := fetchRelease(srv.URL)
	if err != nil {
		t.Fatalf("fetchRelease from GitHub: %v", err)
	}
	if rel.Name != "v0.1.2 稳定版" {
		t.Errorf("Name = %q, want %q", rel.Name, "v0.1.2 稳定版")
	}
	if rel.DownloadURL != "" {
		// GitHub API 不应返回 download_url
		t.Errorf("expected empty DownloadURL from GitHub, got: %q", rel.DownloadURL)
	}
}

func TestFetchRelease404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := fetchRelease(srv.URL)
	if err != errNoRelease {
		t.Fatalf("expected errNoRelease for 404, got: %v", err)
	}
}

func TestFetchRelease_buildsUpdateInfo(t *testing.T) {
	// 验证从 fetchRelease 获取的数据能正确构建 updateInfo
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"tag_name": "v0.2.0",
			"name": "v0.2.0",
			"body": "大版本更新",
			"html_url": "https://github.com/Rescenix/ResceneAgent/releases/tag/v0.2.0",
			"published_at": "2026-08-13T00:00:00Z",
			"download_url": "https://download.shanca.me/Rescene-windows-amd64-setup.exe"
		}`))
	}))
	defer srv.Close()

	rel, err := fetchRelease(srv.URL)
	if err != nil {
		t.Fatalf("fetchRelease: %v", err)
	}

	// 模拟 checkUpdate() 中的版本比较和 updateInfo 构建逻辑
	latest := rel.Name
	latestNum := extractVersion(latest)
	info := &updateInfo{
		HasUpdate:      compareVersions(AppVersion, latestNum),
		CurrentVersion: AppVersion,
		LatestVersion:  latest,
		ReleaseName:    rel.Name,
		ReleaseNotes:   rel.Body,
		ReleaseURL:     rel.HTMLURL,
		DownloadURL:    rel.DownloadURL,
		PublishedAt:    rel.PublishedAt,
	}

	if info.LatestVersion != "v0.2.0" {
		t.Errorf("LatestVersion = %q, want v0.2.0", info.LatestVersion)
	}
	if info.ReleaseNotes != "大版本更新" {
		t.Errorf("ReleaseNotes = %q, want 大版本更新", info.ReleaseNotes)
	}
	if info.DownloadURL != "https://download.shanca.me/Rescene-windows-amd64-setup.exe" {
		t.Errorf("DownloadURL = %q, want download.shanca.me", info.DownloadURL)
	}
	// 0.0.0-dev（AppVersion）vs 0.2.0 → 应有更新
	if !info.HasUpdate {
		t.Error("expected HasUpdate=true for 0.0.0-dev vs v0.2.0")
	}

	// 验证 JSON 序列化后前端能正确解析
	b, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var back map[string]interface{}
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if back["has_update"] != true {
		t.Errorf("JSON has_update = %v, want true", back["has_update"])
	}
	if back["current_version"] != "0.0.0-dev" {
		t.Errorf("JSON current_version = %v", back["current_version"])
	}
}

func TestResolveHotPatchURL(t *testing.T) {
	tests := []struct {
		name     string
		rel      githubRelease
		fromSite bool
		want     string
	}{
		{name: "new zip field", rel: githubRelease{DownloadZip: "https://example.test/new.zip", DownloadExe: "https://example.test/old.zip"}, fromSite: true, want: "https://example.test/new.zip"},
		{name: "legacy exe field", rel: githubRelease{DownloadExe: "https://example.test/old.zip"}, fromSite: true, want: "https://example.test/old.zip"},
		{name: "deployed site json missing field", rel: githubRelease{}, fromSite: true, want: updateHotPatchURL},
		{name: "github fallback must not guess", rel: githubRelease{}, fromSite: false, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveHotPatchURL(&tt.rel, tt.fromSite); got != tt.want {
				t.Fatalf("resolveHotPatchURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDownloadHotPatchZipExtractsNestedCaseInsensitiveExe(t *testing.T) {
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	w, err := zw.Create("Rescene/RESCENE.EXE")
	if err != nil {
		t.Fatal(err)
	}
	payload := append([]byte{'M', 'Z'}, bytes.Repeat([]byte{0x5a}, 1024*1024)...)
	if _, err := w.Write(payload); err != nil {
		t.Fatal(err)
	}
	setup, err := zw.Create("Rescene-windows-amd64-setup.exe")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := setup.Write([]byte("installer must not be extracted as the hot patch")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive.Bytes())
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), updateHotPatchFileName)
	if err := downloadHotPatchZip(srv.URL, dest); err != nil {
		t.Fatalf("downloadHotPatchZip: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatal("extracted executable differs from archive payload")
	}
}

func TestHotPatchBatKeepsPatchUntilCopySucceeds(t *testing.T) {
	script := hotPatchBatTemplate(
		`C:\Users\A%20\rescene-applying.exe`,
		`C:\Users\A%20\rescene-new.exe`,
		`C:\Apps\rescene.exe`,
		4321,
	)
	if !strings.Contains(script, "for /l %%I in (1,1,120)") {
		t.Fatalf("hot patch script does not wait for the old process: %q", script)
	}
	if !strings.Contains(script, `tasklist /fi "PID eq %OLDPID%"`) {
		t.Fatalf("hot patch script does not check the old process id: %q", script)
	}
	if !strings.Contains(script, "for /l %%I in (1,1,30)") {
		t.Fatalf("hot patch script does not retry copy: %q", script)
	}
	if !strings.Contains(script, `C:\Users\A%%20\rescene-applying.exe`) {
		t.Fatalf("hot patch script did not escape percent signs: %q", script)
	}
	copyAt := strings.Index(script, "copy /y")
	copiedAt := strings.Index(script, ":copied")
	deleteAt := strings.LastIndex(script, "del /q")
	if copyAt < 0 || copiedAt < copyAt || deleteAt < copiedAt {
		t.Fatalf("hot patch script deletes the patch before a successful copy: %q", script)
	}
	if !strings.Contains(script, `move /y "C:\Users\A%%20\rescene-applying.exe" "C:\Users\A%%20\rescene-new.exe"`) {
		t.Fatalf("hot patch script does not restore a failed patch for retry: %q", script)
	}
	if strings.Contains(script, `start "" "`) {
		t.Fatalf("hot patch script may open a visible console: %q", script)
	}
}

func TestClaimHotPatchIsSingleWinner(t *testing.T) {
	pending := filepath.Join(t.TempDir(), updateHotPatchFileName)
	if err := os.WriteFile(pending, []byte("patch"), 0o600); err != nil {
		t.Fatal(err)
	}
	claimed, err := claimHotPatch(pending)
	if err != nil {
		t.Fatalf("first claim failed: %v", err)
	}
	if _, err := os.Stat(claimed); err != nil {
		t.Fatalf("claimed patch missing: %v", err)
	}
	if _, err := claimHotPatch(pending); err == nil {
		t.Fatal("second claim unexpectedly succeeded")
	}
}
