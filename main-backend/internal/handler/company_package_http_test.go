package handler

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

// companyPackageTestRouter 只注册本次新增的三个端点，直接打真实 HTTP 链路。
func companyPackageTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/company/projects", HandleCompanyProjects)
	r.GET("/api/company/project-file", HandleCompanyProjectFile)
	r.GET("/api/company/package", HandleCompanyProjectPackage)
	return r
}

// companyPackageTestProject 找一个磁盘上真实存在、已过门禁的项目。
func companyPackageTestProject(t *testing.T) (string, string) {
	t.Helper()
	for _, root := range companyProjectRoots() {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			dir := filepath.Join(root, e.Name())
			if _, err := os.Stat(filepath.Join(dir, "delivery.manifest.json")); err != nil {
				continue
			}
			if _, err := verifyProjectDeliveryGate(dir); err != nil {
				continue
			}
			return e.Name(), dir
		}
	}
	return "", ""
}

func TestCompanyPackageEndpoints(t *testing.T) {
	name, dir := companyPackageTestProject(t)
	if name == "" {
		t.Skip("磁盘上没有已过门禁的项目，跳过")
	}
	r := companyPackageTestRouter()

	// 1. 项目列表
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/company/projects", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("projects 状态码 %d", rec.Code)
	}
	var list struct {
		Projects []struct {
			Project   string               `json:"project"`
			Title     string               `json:"title"`
			Artifacts []companyProjectFile `json:"artifacts"`
			Stages    []string             `json:"stages"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("projects JSON: %v", err)
	}
	if len(list.Projects) == 0 {
		t.Fatal("projects 返回空")
	}
	t.Logf("projects 返回 %d 个", len(list.Projects))

	// 2. 整包下载
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/company/package?project="+url.QueryEscape(name), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("package 状态码 %d body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/zip" {
		t.Fatalf("package Content-Type=%q", ct)
	}
	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatalf("下载到的不是合法 zip: %v", err)
	}
	found := map[string]bool{}
	for _, f := range zr.File {
		found[f.Name] = true
	}
	if !found["delivery.manifest.json"] {
		t.Fatal("zip 内缺交付清单")
	}
	t.Logf("package ok：%d 个文件，%d 字节", len(zr.File), rec.Body.Len())

	// 3. 单文件读取（含子目录产物）
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/company/project-file?project="+url.QueryEscape(name)+"&path=output-app.html", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("project-file 状态码 %d body=%s", rec.Code, rec.Body.String())
	}
	var file struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &file)
	if file.Content == "" {
		t.Fatal("project-file 没返回内容")
	}

	// 4. 路径穿越必须拒绝
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/company/project-file?project="+url.QueryEscape(name)+"&path=../../finance.json", nil))
	if rec.Code == http.StatusOK {
		t.Fatal("路径穿越未被拦截")
	}
	t.Logf("路径穿越被拒：状态码 %d", rec.Code)

	// 5. 不存在的项目
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/company/package?project=不存在的项目-xyz", nil))
	if rec.Code == http.StatusOK {
		t.Fatal("不存在的项目却返回 200")
	}
	_ = dir
}
