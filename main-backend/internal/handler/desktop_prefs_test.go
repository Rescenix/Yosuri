package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

// newDesktopPrefsEnv 把偏好文件指到临时目录并清掉包级缓存（偏好是缓存的，不清会串味）。
func newDesktopPrefsEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("RESCENE_DATA_DIR", dir)
	ResetDesktopPrefsCacheForTest()
	t.Cleanup(ResetDesktopPrefsCacheForTest)
	return dir
}

func readPrefsFile(t *testing.T, dir string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, desktopPrefsFileName))
	if err != nil {
		t.Fatalf("偏好文件应已落盘: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("偏好文件应是合法 JSON，实际 %q: %v", string(raw), err)
	}
	return m
}

func postCloseBehavior(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/desktop/close-behavior",
		bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	HandleSetCloseBehavior(c)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("响应应是合法 JSON，实际 %q: %v", rec.Body.String(), err)
	}
	return m
}

// 从未设置过偏好时默认「缩到托盘」，保持既有行为不突变。
func TestCloseToTrayDefaultsToTrue(t *testing.T) {
	newDesktopPrefsEnv(t)
	if !closeToTrayDesired() {
		t.Fatal("未设置过偏好时应默认「关闭窗口缩到托盘」")
	}
	if !CloseToTrayDesired() {
		t.Fatal("导出版本应与内部判断一致")
	}
}

// 用户选「关闭即退出」→ 落盘 + 立即生效（OnBeforeClose 每次实时读，不需要重启）。
func TestSetCloseBehaviorPersistsFalse(t *testing.T) {
	dir := newDesktopPrefsEnv(t)
	rec := postCloseBehavior(t, `{"close_to_tray": false}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("应返回 200，实际 %d：%s", rec.Code, rec.Body.String())
	}
	if got := decodeBody(t, rec)["close_to_tray"]; got != false {
		t.Fatalf("响应应回显 false，实际 %v", got)
	}
	if closeToTrayDesired() {
		t.Fatal("写入后应立刻读到 false")
	}
	if got := readPrefsFile(t, dir)["close_to_tray"]; got != false {
		t.Fatalf("偏好文件里应是 false，实际 %v", got)
	}
}

// 写 close_to_tray 不能把已有的 auto_start_enabled 抹掉（读-改-写）。
func TestSetCloseBehaviorKeepsAutoStartField(t *testing.T) {
	dir := newDesktopPrefsEnv(t)
	if err := os.WriteFile(filepath.Join(dir, desktopPrefsFileName),
		[]byte(`{"auto_start_enabled": false}`), 0o600); err != nil {
		t.Fatal(err)
	}
	ResetDesktopPrefsCacheForTest()

	postCloseBehavior(t, `{"close_to_tray": false}`)

	prefs := readPrefsFile(t, dir)
	if got, ok := prefs["auto_start_enabled"]; !ok || got != false {
		t.Fatalf("auto_start_enabled 应原样保留 false，实际 %v（存在=%v）", got, ok)
	}
	if got := prefs["close_to_tray"]; got != false {
		t.Fatalf("close_to_tray 应写入 false，实际 %v", got)
	}
}

// 反过来也一样：写 auto_start_enabled 不能把 close_to_tray 抹掉。
// 这里直接走 saveDesktopPrefs 而不过 HandleSetAutoStart —— 后者会真的去写 HKCU 注册表，
// 测试不该动用户的系统启动项。
func TestSaveDesktopPrefsKeepsSiblingFields(t *testing.T) {
	dir := newDesktopPrefsEnv(t)
	off := false
	if err := saveDesktopPrefs(func(p *desktopPrefs) { p.AutoStartEnabled = &off }); err != nil {
		t.Fatal(err)
	}
	postCloseBehavior(t, `{"close_to_tray": false}`)

	prefs := readPrefsFile(t, dir)
	if got, ok := prefs["auto_start_enabled"]; !ok || got != false {
		t.Fatalf("auto_start_enabled 应保留 false，实际 %v（存在=%v）", got, ok)
	}
	if got := prefs["close_to_tray"]; got != false {
		t.Fatalf("close_to_tray 应保留 false，实际 %v", got)
	}
}

// 缺字段要挡住，不能默默当成 false（那会替用户把「缩到托盘」改掉）。
func TestSetCloseBehaviorRejectsMissingField(t *testing.T) {
	newDesktopPrefsEnv(t)
	rec := postCloseBehavior(t, `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("缺 close_to_tray 应返回 400，实际 %d：%s", rec.Code, rec.Body.String())
	}
	if !closeToTrayDesired() {
		t.Fatal("非法请求不该改动偏好")
	}
}

// GET 要如实反映已落盘的偏好。
func TestGetCloseBehaviorReflectsSavedValue(t *testing.T) {
	newDesktopPrefsEnv(t)
	postCloseBehavior(t, `{"close_to_tray": false}`)

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/desktop/close-behavior", nil)
	HandleGetCloseBehavior(c)

	d := decodeBody(t, rec)
	if d["ok"] != true {
		t.Fatalf("应返回 ok=true，实际 %v", d["ok"])
	}
	if d["close_to_tray"] != false {
		t.Fatalf("应返回 close_to_tray=false，实际 %v", d["close_to_tray"])
	}
}
