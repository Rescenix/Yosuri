//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"

	"backend/internal/handler"
)

// pointUserDataAtTemp 把桌面偏好文件指到临时目录，让 AutoStartDesired() 读得到可控的偏好。
// 必须顺手清缓存：偏好是包级缓存的，不清就会读到上一个用例写的那份。
func pointUserDataAtTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("RESCENE_DATA_DIR", dir)
	handler.ResetDesktopPrefsCacheForTest()
	t.Cleanup(handler.ResetDesktopPrefsCacheForTest)
	return dir
}

func writePrefs(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "desktop_prefs.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func touch(t *testing.T, dir string, names ...string) {
	t.Helper()
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func exists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

// 回归（2026-09-23 用户报障「设置里已经关掉开机自启，开机还是自己起来」）：
// 关掉开关后启动文件夹里不能留任何本应用的快捷方式 —— 留着的那条通道会照样把应用拉起来。
func TestRemoveStartupShortcutsClearsOnlyOurs(t *testing.T) {
	dir := t.TempDir()
	ours := []string{
		"Yosuri.lnk",
		"Yosuri.lnk.bak-0923",
		"Rescene.lnk",
		"ResceneAgent.lnk",
		"Rescene.lnk.bak-0901",
	}
	others := []string{
		"FocuSee.lnk",
		"desktop.ini",
		"rescene-bili-frontdesk.vbs", // 名字像但不是快捷方式，属于 B 站前台守护，不能连坐
		"Yosuri-notes.txt",
	}
	touch(t, dir, append(append([]string{}, ours...), others...)...)

	removeStartupShortcuts(dir)

	for _, n := range ours {
		if exists(dir, n) {
			t.Errorf("关闭自启后应被清理，但仍然存在：%s", n)
		}
	}
	for _, n := range others {
		if !exists(dir, n) {
			t.Errorf("不属于本应用的启动项，不该被删：%s", n)
		}
	}
}

// 关闭自启时清理要幂等：重复调用、目录本来就空，都不该报错或误伤。
func TestRemoveStartupShortcutsIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	removeStartupShortcuts(dir) // 空目录
	touch(t, dir, "Yosuri.lnk")
	removeStartupShortcuts(dir)
	removeStartupShortcuts(dir) // 已清干净，再来一次
	if exists(dir, "Yosuri.lnk") {
		t.Fatal("Yosuri.lnk 应已被清理")
	}
}

// 偏好=关 → 启动文件夹里的 Yosuri.lnk 必须被删掉（这条就是用户报障的根因）。
func TestSyncStartupShortcutRemovesWhenDisabled(t *testing.T) {
	prefsDir := pointUserDataAtTemp(t)
	writePrefs(t, prefsDir, `{"auto_start_enabled": false}`)

	startup := t.TempDir()
	touch(t, startup, "Yosuri.lnk")

	syncStartupShortcut(startup, `C:\fake\rescene.exe`, `C:\fake`)

	if exists(startup, "Yosuri.lnk") {
		t.Fatal("用户已关闭开机自启，启动文件夹里的 Yosuri.lnk 必须被删除")
	}
}

// 偏好=开 → 已有的 Yosuri.lnk 保持不动（ensureShortcut 对已存在的 lnk 直接短路，
// 所以这个用例不会真的去调 PowerShell 建快捷方式）。
func TestSyncStartupShortcutKeepsWhenEnabled(t *testing.T) {
	prefsDir := pointUserDataAtTemp(t)
	writePrefs(t, prefsDir, `{"auto_start_enabled": true}`)

	startup := t.TempDir()
	touch(t, startup, "Yosuri.lnk")

	syncStartupShortcut(startup, `C:\fake\rescene.exe`, `C:\fake`)

	if !exists(startup, "Yosuri.lnk") {
		t.Fatal("开机自启开着时不该删掉启动快捷方式")
	}
}

// 偏好文件缺失 = 从未设置过 → 按出厂默认「开」处理，不能把老用户的启动项误删。
func TestSyncStartupShortcutDefaultsToEnabled(t *testing.T) {
	pointUserDataAtTemp(t) // 不写 desktop_prefs.json

	startup := t.TempDir()
	touch(t, startup, "Yosuri.lnk")

	syncStartupShortcut(startup, `C:\fake\rescene.exe`, `C:\fake`)

	if !exists(startup, "Yosuri.lnk") {
		t.Fatal("从未设置过偏好时应按默认「开」处理，不该删除启动快捷方式")
	}
}

// 偏好=开 时，启动文件夹里的旧品牌 lnk 要被改名清理，不能和新入口并存。
func TestSyncStartupShortcutClearsOldBrandWhenEnabled(t *testing.T) {
	prefsDir := pointUserDataAtTemp(t)
	writePrefs(t, prefsDir, `{"auto_start_enabled": true}`)

	startup := t.TempDir()
	touch(t, startup, "ResceneAgent.lnk", "Yosuri.lnk")

	syncStartupShortcut(startup, `C:\fake\rescene.exe`, `C:\fake`)

	if exists(startup, "ResceneAgent.lnk") {
		t.Fatal("旧品牌 ResceneAgent.lnk 应被改名清理")
	}
	if !exists(startup, "Yosuri.lnk") {
		t.Fatal("Yosuri.lnk 应保留")
	}
}
