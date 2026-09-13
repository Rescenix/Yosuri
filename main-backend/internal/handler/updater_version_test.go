package handler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 构造含各种版本串的二进制 blob：Go 伪版本（曾经的乱码元凶）+ 真实发布版本 + 库版本。
func versionTestBlob() []byte {
	return []byte(
		"module github.com/Rescenix/Yosuri pseudo v0.0.0-20250511090121-5959a4027728\n" +
			"AppVersion=0.3.9\n" +
			"some lib v0.55.1 / v0.16.47 / 0.17.0\n" +
			"released as v0.3.10-mock for testing\n")
}

func TestGoPseudoVersionRe(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"v0.0.0-20250511090121-5959a4027728", true},
		{"v1.2.3-20240101120000-abcdef123456", true},
		{"0.3.9", false},
		{"v0.3.10-mock", false},
		{"0.55.1", false},
	}
	for _, c := range cases {
		if got := goPseudoVersionRe.MatchString(c.in); got != c.want {
			t.Errorf("goPseudoVersionRe(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestPickAppVersionSkipsPseudo(t *testing.T) {
	got := pickAppVersion(versionTestBlob())
	// 伪版本必须被剔除；剩下的最长者（库版本 0.55.1 长度 6）胜过 "0.3.9"（5），
	// 这正是二进制扫描天生多义的注脚——正常路径由 rescene-new.version 决定，不在本测试断言。
	if strings.Contains(got, "20250511") || strings.Contains(got, "0.0.0-") {
		t.Fatalf("pickAppVersion 未剔除 Go 伪版本: %q", got)
	}
	if got == "" {
		t.Fatal("pickAppVersion 在存在合法候选时返回空")
	}
}

func TestPendingPatchVersion_SidecarWins(t *testing.T) {
	localDir := t.TempDir()
	newExe := filepath.Join(localDir, "rescene-new.exe")
	if err := os.WriteFile(newExe, versionTestBlob(), 0o600); err != nil {
		t.Fatal(err)
	}
	// 伪版本在 exe 里，但旁路标记里的目标版本才是权威
	if err := writePendingPatchVersion(localDir, "v0.3.10-mock"); err != nil {
		t.Fatal(err)
	}
	got := pendingPatchVersion(newExe, localDir)
	if got != "v0.3.10-mock" {
		t.Fatalf("sidecar 优先级失败: got %q, want v0.3.10-mock", got)
	}
	// 旁路缺失时回退扫描（且不吐伪版本）
	if err := os.Remove(filepath.Join(localDir, "rescene-new.version")); err != nil {
		t.Fatal(err)
	}
	got = pendingPatchVersion(newExe, localDir)
	if strings.Contains(got, "20250511") || strings.Contains(got, "0.0.0-") {
		t.Fatalf("兜底扫描仍吐伪版本: %q", got)
	}
}

func TestWriteLastAppliedVersion_UsesSidecar(t *testing.T) {
	localDir := t.TempDir()
	newExe := filepath.Join(localDir, "rescene-new.exe")
	if err := os.WriteFile(newExe, versionTestBlob(), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writePendingPatchVersion(localDir, "v0.3.10-mock"); err != nil {
		t.Fatal(err)
	}
	if err := writeLastAppliedVersion(newExe, localDir); err != nil {
		t.Fatal(err)
	}
	mark := filepath.Join(localDir, lastAppliedVersionFileName)
	data, err := os.ReadFile(mark)
	if err != nil {
		t.Fatalf("last-applied.txt 未写入: %v", err)
	}
	if got := strings.TrimSpace(string(data)); got != "v0.3.10-mock" {
		t.Fatalf("标记版本错误: got %q, want v0.3.10-mock（旧实现这里会写 Go 伪版本）", got)
	}
}

func TestWriteLastAppliedVersion_EmptySidecarSkips(t *testing.T) {
	localDir := t.TempDir()
	newExe := filepath.Join(localDir, "rescene-new.exe")
	if err := os.WriteFile(newExe, versionTestBlob(), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writePendingPatchVersion(localDir, ""); err != nil { // 空版本不落盘
		t.Fatal(err)
	}
	if err := writeLastAppliedVersion(newExe, localDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(localDir, lastAppliedVersionFileName)); !os.IsNotExist(err) {
		t.Fatal("无版本信息时不应写入 last-applied.txt")
	}
}
