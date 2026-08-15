package handler

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDHSCommunitySearchFindsExternalNPMPackages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/agent-review-harness/latest" {
			_, _ = w.Write([]byte(`{"name":"agent-review-harness","version":"1.0.0","description":"review harness","repository":{"url":"git+https://github.com/acme/review-harness.git"},"dsh":{"bundle":{"patch":"./cordis.patch.yml"}}}`))
			return
		}
		if r.URL.Path != "/-/v1/search" || !strings.Contains(r.URL.Query().Get("text"), "review") {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"objects":[{"package":{"name":"agent-review-harness","description":"review harness","version":"1.0.0","date":"2026-08-15T00:00:00Z","links":{"repository":"git+https://github.com/acme/review-harness.git","npm":"https://npmjs.com/package/agent-review-harness"}},"score":{"final":0.82}}]}`))
	}))
	defer server.Close()
	oldNPM := npmRegistryBaseURL
	npmRegistryBaseURL = server.URL
	t.Cleanup(func() { npmRegistryBaseURL = oldNPM })

	items, err := searchNPMDHSCommunity("review")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Repo != "acme/review-harness" || items[0].SourceID != "npm-dsh" || items[0].BundlePatch != "./cordis.patch.yml" {
		t.Fatalf("npm 外部插件发现结果错误: %+v", items)
	}
}

func TestDHSCommunityPreviewIsPlainAndBounded(t *testing.T) {
	preview := plainDHSPreview("# Review Harness\n\n[Docs](https://example.com) <script>alert(1)</script> "+strings.Repeat("能力说明 ", 200), 80)
	if strings.Contains(preview, "<script>") || !strings.Contains(preview, "Docs") || len([]rune(preview)) > 81 {
		t.Fatalf("预览内容没有安全清洗或长度限制: %q", preview)
	}
}

func TestDHSNativeBundleAuditPinsAndScansNPMPackage(t *testing.T) {
	archive := buildDHSNPMPackage(t, map[string]string{
		"package/package.json":     `{"name":"dsh-safe","version":"1.2.3","dsh":{"bundle":{"patch":"./cordis.patch.yml"}}}`,
		"package/cordis.patch.yml": "plugins:\n  safe: true\n",
		"package/lib/index.js":     "export function apply(ctx) { return ctx }",
	})
	digest := sha512.Sum512(archive)
	integrity := "sha512-" + base64.StdEncoding.EncodeToString(digest[:])
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/dsh-safe/latest":
			_, _ = fmt.Fprintf(w, `{"name":"dsh-safe","version":"1.2.3","description":"safe","repository":{"url":"https://github.com/acme/dsh-safe"},"dsh":{"bundle":{"patch":"./cordis.patch.yml"}},"dist":{"tarball":%q,"integrity":%q}}`, server.URL+"/dsh-safe.tgz", integrity)
		case "/dsh-safe.tgz":
			_, _ = w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	oldNPM := npmRegistryBaseURL
	npmRegistryBaseURL = server.URL
	t.Cleanup(func() { npmRegistryBaseURL = oldNPM })

	report, err := auditDHSNPMBundle("dsh-safe", "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "passed" || report.ExecutionStatus != "contained" || report.FileCount != 3 {
		t.Fatalf("DHS npm 原生包应通过精确发布物审计: %+v", report)
	}
}

func buildDHSNPMPackage(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var output bytes.Buffer
	gz := gzip.NewWriter(&output)
	tw := tar.NewWriter(gz)
	for name, contents := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(contents)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(contents)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func TestDHSCommunityAuditContainsDeclarativePackage(t *testing.T) {
	server := newDHSAuditServer(t, false, false)
	defer server.Close()
	oldAPI, oldRaw := githubAPIBaseURL, githubRawBaseURL
	githubAPIBaseURL, githubRawBaseURL = server.URL, server.URL
	t.Cleanup(func() { githubAPIBaseURL, githubRawBaseURL = oldAPI, oldRaw })

	report, files, err := auditDHSCommunityPackage("acme/safe-plugin", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "passed" || report.ExecutionStatus != "contained" {
		t.Fatalf("声明式插件应通过双层审计: %+v", report)
	}
	if report.Commit != "commit-safe" || report.SkillPath != "safe-plugin/SKILL.md" || len(files) != 2 {
		t.Fatalf("审计未固定正确版本或目录: report=%+v files=%+v", report, files)
	}
}

func TestDHSCommunityAuditBlocksExecutableContent(t *testing.T) {
	server := newDHSAuditServer(t, true, false)
	defer server.Close()
	oldAPI, oldRaw := githubAPIBaseURL, githubRawBaseURL
	githubAPIBaseURL, githubRawBaseURL = server.URL, server.URL
	t.Cleanup(func() { githubAPIBaseURL, githubRawBaseURL = oldAPI, oldRaw })

	report, _, err := auditDHSCommunityPackage("acme/safe-plugin", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "blocked" || report.ExecutionStatus != "blocked" {
		t.Fatalf("含脚本插件必须被执行层阻断: %+v", report)
	}
	found := false
	for _, finding := range report.Findings {
		if finding.Code == "executable_content" {
			found = true
		}
	}
	if !found {
		t.Fatalf("阻断报告缺少 executable_content: %+v", report.Findings)
	}
}

func TestDHSCommunityAuditBlocksTextTooLargeToScan(t *testing.T) {
	server := newDHSAuditServer(t, false, true)
	defer server.Close()
	oldAPI, oldRaw := githubAPIBaseURL, githubRawBaseURL
	githubAPIBaseURL, githubRawBaseURL = server.URL, server.URL
	t.Cleanup(func() { githubAPIBaseURL, githubRawBaseURL = oldAPI, oldRaw })

	report, _, err := auditDHSCommunityPackage("acme/safe-plugin", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "blocked" {
		t.Fatalf("无法完整扫描的大文本必须被阻断: %+v", report)
	}
	for _, finding := range report.Findings {
		if finding.Code == "unscanned_text" {
			return
		}
	}
	t.Fatalf("阻断报告缺少 unscanned_text: %+v", report.Findings)
}

func TestDHSCommunityPolicyRejectsActiveSVG(t *testing.T) {
	if dhsAllowedAssetExt[".svg"] || dhsTextExt[".svg"] {
		t.Fatal("社区插件不能允许可携带脚本或事件处理器的 SVG 主动内容")
	}
}

func newDHSAuditServer(t *testing.T, includeScript, includeOversizedText bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/safe-plugin":
			_, _ = w.Write([]byte(`{"default_branch":"main"}`))
		case "/repos/acme/safe-plugin/commits/main":
			_, _ = w.Write([]byte(`{"sha":"commit-safe"}`))
		case "/repos/acme/safe-plugin/git/trees/commit-safe":
			script := ""
			if includeScript {
				script = `,{"path":"safe-plugin/scripts/run.py","type":"blob","mode":"100644","size":20}`
			}
			oversized := ""
			if includeOversizedText {
				oversized = `,{"path":"safe-plugin/references/oversized.md","type":"blob","mode":"100644","size":1048577}`
			}
			_, _ = w.Write([]byte(`{"tree":[{"path":"safe-plugin/SKILL.md","type":"blob","mode":"100644","size":80},{"path":"safe-plugin/references/guide.md","type":"blob","mode":"100644","size":20}` + script + oversized + `],"truncated":false}`))
		case "/acme/safe-plugin/commit-safe/safe-plugin/SKILL.md":
			_, _ = w.Write([]byte("---\nname: safe-plugin\ndescription: safe\n---\n\nUse approved Go tools."))
		case "/acme/safe-plugin/commit-safe/safe-plugin/references/guide.md":
			_, _ = w.Write([]byte("Declarative guidance."))
		default:
			http.NotFound(w, r)
		}
	}))
}
