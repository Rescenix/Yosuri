package handler

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestResolvedSiteSourceStaysInsideProject(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "dist"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "dist", "index.html"), []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := resolvedSiteSource(root, "dist")
	if err != nil || got != filepath.Join(root, "dist") {
		t.Fatalf("resolvedSiteSource() = %q, %v", got, err)
	}
	if _, err := resolvedSiteSource(root, "../secret"); err == nil {
		t.Fatal("path traversal was accepted")
	}
	if _, err := resolvedSiteSource(root, "missing"); err == nil {
		t.Fatal("directory without index.html was accepted")
	}
}

func TestZipStaticSitePreservesRelativeFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "assets"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("<h1>site</h1>"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "assets", "app.js"), []byte("console.log(1)"), 0644); err != nil {
		t.Fatal(err)
	}
	archive, err := zipStaticSite(root)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(archive.Bytes()), int64(archive.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(zr.File) != 2 || zr.File[0].Name != "assets/app.js" || zr.File[1].Name != "index.html" {
		t.Fatalf("zip files = %#v", zr.File)
	}
	f, err := zr.File[1].Open()
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil || string(b) != "<h1>site</h1>" {
		t.Fatalf("index content = %q, %v", b, err)
	}
}

func TestDeploySiteRequiresExplicitPublicConfirmation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/sites/deploy", strings.NewReader(`{"token":"not-used"}`))
	request.Header.Set("Content-Type", "application/json")
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = request
	DeploySite(ctx)
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "用户确认") {
		t.Fatalf("DeploySite without confirmation = %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestNetlifyNameConflictGetsUserReviewableSuggestion(t *testing.T) {
	if !isNetlifyNameConflict(`{"errors":{"subdomain":["must be unique"]}}`) {
		t.Fatal("Netlify unique-subdomain error was not recognized")
	}
	if isNetlifyNameConflict("invalid access token") {
		t.Fatal("unrelated Netlify error was marked as a name conflict")
	}
	suggestion := suggestedSiteName("test1")
	if !strings.HasPrefix(suggestion, "test1-") || len(suggestion) > 63 {
		t.Fatalf("suggestion = %q", suggestion)
	}
}
