package handler

// 工作目录越界判定的单元测试（approval.go）。
// 全部从 core.GetProjectRoot() 现取真实根再派生用例，不写死路径、也不调
// SetProjectRoot（那会把 workdir.txt 落盘覆盖用户当前选中的项目）。

import (
	"path/filepath"
	"strings"
	"testing"

	"backend/internal/ai/core"
)

func TestPathOutsideRoot(t *testing.T) {
	root := filepath.Clean(core.GetProjectRoot())
	parent := filepath.Dir(root)

	// 挑一个跟 root 不同的盘符，构造跨盘用例
	otherVolume := "D:\\tmp\\x.txt"
	if strings.EqualFold(filepath.VolumeName(root), "D:") {
		otherVolume = "E:\\tmp\\x.txt"
	}

	cases := []struct {
		name string
		path string
		want bool
	}{
		{"相对路径-目录内", "main-backend/foo.go", false},
		{"绝对路径-目录内", filepath.Join(root, "main-backend", "foo.go"), false},
		{"大小写不同-目录内", strings.ToUpper(root) + `\MAIN-BACKEND\foo.go`, false},
		{"工作目录自身", root, false},
		{"父目录", parent, true},
		{"同级兄弟目录", filepath.Join(parent, "some-other-project"), true},
		{"跨盘符", otherVolume, true},
		{"相对路径打点跑出去", "../outside.txt", true},
	}
	for _, tc := range cases {
		if got := pathOutsideRoot(tc.path); got != tc.want {
			t.Errorf("%s: pathOutsideRoot(%q) = %v, want %v", tc.name, tc.path, got, tc.want)
		}
	}
}

func TestToolOutsideRoot(t *testing.T) {
	root := filepath.Clean(core.GetProjectRoot())
	outside := filepath.Join(filepath.Dir(root), "elsewhere", "x.txt")
	outsideJSON := strings.ReplaceAll(outside, `\`, `\\`)

	cases := []struct {
		name    string
		args    string
		want    bool
		wantHit string
	}{
		{"写到目录内", `{"path":"main-backend/x.go","content":"a"}`, false, ""},
		{"写到目录外", `{"path":"` + outsideJSON + `","content":"a"}`, true, outside},
		{"move 源在外", `{"source":"` + outsideJSON + `","destination":"b.txt"}`, true, outside},
		{"move 目标在外", `{"source":"a.txt","destination":"` + outsideJSON + `"}`, true, outside},
		{"多文件读-有一个在外", `{"paths":["a.txt","` + outsideJSON + `"]}`, true, outside},
		{"多文件读-全在内", `{"paths":["a.txt","main-backend/b.go"]}`, false, ""},
		{"shell 的 command 不当路径看", `{"command":"echo hi"}`, false, ""},
		{"http 的 URL path 不误判成越界", `{"path":"/api/v1/users"}`, false, ""},
		{"空参数", ``, false, ""},
		{"坏 JSON 不 panic", `{not json`, false, ""},
	}
	for _, tc := range cases {
		got, hit := toolOutsideRoot(tc.args)
		if got != tc.want || hit != tc.wantHit {
			t.Errorf("%s: toolOutsideRoot(%s) = (%v,%q), want (%v,%q)",
				tc.name, tc.args, got, hit, tc.want, tc.wantHit)
		}
	}
}

// filesystem server 的 allowed dir 参数必须被换成整机根，否则审批通过了底层也执行不了。
func TestExpandFilesystemArgs(t *testing.T) {
	roots := fsAllowedDirs()
	if len(roots) == 0 {
		t.Fatal("fsAllowedDirs 不该返回空")
	}

	got := expandFilesystemArgs([]string{"-y", "@modelcontextprotocol/server-filesystem", `C:\Pro2026\re0`})
	want := append([]string{"-y", "@modelcontextprotocol/server-filesystem"}, roots...)
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("filesystem args 展开错误:\n got=%v\nwant=%v", got, want)
	}

	// 非 filesystem server 原样不动
	other := []string{`C:\tools\custom_mcp_server.py`}
	if kept := expandFilesystemArgs(other); strings.Join(kept, "|") != strings.Join(other, "|") {
		t.Errorf("非 filesystem server 的 args 被误改: %v", kept)
	}
}

// 越界的 don't-ask-again 必须按目录记：批准一次不能等于放行所有目录。
func TestOutsideRememberKeyIsDirScoped(t *testing.T) {
	a := outsideRememberKey(`C:\SomeDir\a.txt`)
	b := outsideRememberKey(`C:\SomeDir\b.txt`)
	c := outsideRememberKey(`C:\OtherDir\c.txt`)
	if a != b {
		t.Errorf("同目录不同文件应共用规则键: %q vs %q", a, b)
	}
	if a == c {
		t.Errorf("不同目录不该共用规则键，都是 %q", a)
	}
	if !strings.HasPrefix(a, "approve:outside:") {
		t.Errorf("规则键前缀意外: %q", a)
	}
	// 与按工具名记的键不能撞车
	if a == rememberKey("mcp__fs__write_file") {
		t.Errorf("越界键与工具键撞车: %q", a)
	}
}
