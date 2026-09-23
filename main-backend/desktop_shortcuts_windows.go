//go:build windows

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"backend/internal/handler"
)

const shortcutAppName = "Yosuri"

// oldBrandShortcutNames 旧品牌快捷方式名，桌面/开始菜单/启动文件夹都要清。
var oldBrandShortcutNames = []string{"Rescene", "ResceneAgent"}

// ensureDesktopShortcuts 确保桌面/开始菜单的 Yosuri 快捷方式存在，并让启动文件夹里的
// 快捷方式与用户「开机自启」偏好保持一致。
// 每次启动自愈（2026-09-01 实锤）：hotpatch 只替换 exe、不跑 NSIS 安装器，
// 导致升级用户开始菜单从未有快捷方式（用户原话「开始菜单从没有建立过快捷方式」）。
// 同时清理旧品牌（Rescene/ResceneAgent）残留快捷方式。
//
// ⚠️ 2026-09-23 修复（用户报「设置里已经关掉开机自启，开机还是自己起来」）：
// 开机自启其实有两条通道 —— ①注册表 HKCU\...\Run 的 ResceneAgent 值，②启动文件夹的
// Yosuri.lnk。设置面板只关了 ①，本函数却仍在 ② 里无条件建 lnk，等于关了一个还剩一个。
// 现在启动文件夹单独跟随偏好：关 = 清理干净，开 = 缺了补齐。桌面/开始菜单与自启无关，照旧自愈。
func ensureDesktopShortcuts() {
	exePath, err := os.Executable()
	if err != nil {
		log.Printf("⚠️ 修复快捷方式失败：定位 exe：%v", err)
		return
	}
	exeDir := filepath.Dir(exePath)

	created := 0
	// 桌面 + 开始菜单：与自启偏好无关，始终自愈
	for _, dir := range []string{startMenuProgramsDir(), desktopDir()} {
		if dir == "" {
			continue
		}
		renameOldBrandShortcuts(dir)
		if ensureShortcut(dir, exePath, exeDir) {
			created++
		}
	}

	// 启动文件夹：跟随「开机自启」偏好（用户关掉后不能再被它拉起来）
	if dir := startupDir(); dir != "" {
		if syncStartupShortcut(dir, exePath, exeDir) {
			created++
		}
	}

	if created > 0 {
		// 通知 Explorer 刷新图标缓存（隐藏窗口）
		refreshCmd := exec.Command("cmd", "/c", "ie4uinit.exe", "-show")
		refreshCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		_ = refreshCmd.Run()
	}
}

// syncStartupShortcut 让启动文件夹里的快捷方式与用户「开机自启」偏好保持一致：
// 开启 → 缺了补齐（顺手清旧品牌）；关闭 → 全清干净。返回是否真的新建了快捷方式。
// 单独抽出来是为了能在临时目录里跑回归测试（见 desktop_shortcuts_windows_test.go）。
func syncStartupShortcut(dir, exePath, exeDir string) bool {
	if handler.AutoStartDesired() {
		renameOldBrandShortcuts(dir)
		return ensureShortcut(dir, exePath, exeDir)
	}
	removeStartupShortcuts(dir)
	return false
}

// renameOldBrandShortcuts 把旧品牌快捷方式改名备份，避免新旧两个入口并存。
func renameOldBrandShortcuts(dir string) {
	for _, old := range oldBrandShortcutNames {
		oldLnk := filepath.Join(dir, old+".lnk")
		if _, err := os.Stat(oldLnk); err == nil {
			_ = os.Rename(oldLnk, oldLnk+".bak-"+time.Now().Format("0102"))
			log.Printf("🔧 已清理旧快捷方式 %s", oldLnk)
		}
	}
}

// ensureShortcut 缺了就建，已存在不动；返回是否真的新建了一个。
func ensureShortcut(dir, exePath, exeDir string) bool {
	lnkPath := filepath.Join(dir, shortcutAppName+".lnk")
	if _, err := os.Stat(lnkPath); err == nil {
		return false // 已存在，不动
	}
	if err := createWindowsShortcut(lnkPath, exePath, exeDir); err != nil {
		log.Printf("⚠️ 创建快捷方式失败 %s：%v", lnkPath, err)
		return false
	}
	log.Printf("🔧 已创建快捷方式 %s", lnkPath)
	return true
}

// removeStartupShortcuts 关闭开机自启时清空启动文件夹里的本应用快捷方式：
// Yosuri.lnk 本体、旧品牌 lnk、以及历史改名留下的 .bak-* 残留
// （.bak 不会被 Windows 执行，但留在启动文件夹里属于脏数据）。
func removeStartupShortcuts(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("⚠️ 清理启动文件夹失败：%v", err)
		return
	}
	prefixes := make([]string, 0, len(oldBrandShortcutNames)+1)
	prefixes = append(prefixes, shortcutAppName+".lnk")
	for _, old := range oldBrandShortcutNames {
		prefixes = append(prefixes, old+".lnk")
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		matched := false
		for _, prefix := range prefixes {
			if name == prefix || strings.HasPrefix(name, prefix+".bak-") {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		path := filepath.Join(dir, name)
		if err := os.Remove(path); err != nil {
			log.Printf("⚠️ 清理启动文件夹快捷方式失败 %s：%v", path, err)
			continue
		}
		log.Printf("🔧 已按「关闭开机自启」偏好清理 %s", path)
	}
}

func startMenuProgramsDir() string {
	if v, err := os.UserConfigDir(); err == nil {
		return filepath.Join(v, "Microsoft", "Windows", "Start Menu", "Programs")
	}
	return ""
}

func desktopDir() string {
	// 优先取注册表/Shell API 的真实桌面路径（可能被 OneDrive 重定向）
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		"[Environment]::GetFolderPath('Desktop')")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err == nil {
		if p := strings.TrimSpace(string(out)); p != "" {
			return p
		}
	}
	return filepath.Join(os.Getenv("USERPROFILE"), "Desktop")
}

func startupDir() string {
	if v, err := os.UserConfigDir(); err == nil {
		return filepath.Join(v, "Microsoft", "Windows", "Start Menu", "Programs", "Startup")
	}
	return ""
}

// createWindowsShortcut 用 WScript.Shell COM 创建 .lnk（带隐藏窗口，不闪黑框）。
func createWindowsShortcut(lnkPath, target, workDir string) error {
	// 注意：PowerShell 参数里的路径用转义引号，防空格路径截断
	ps := fmt.Sprintf(
		`$ws = New-Object -ComObject WScript.Shell; $s = $ws.CreateShortcut('%s'); $s.TargetPath = '%s'; $s.WorkingDirectory = '%s'; $s.IconLocation = '%s,0'; $s.Save()`,
		strings.ReplaceAll(lnkPath, "'", "''"),
		strings.ReplaceAll(target, "'", "''"),
		strings.ReplaceAll(workDir, "'", "''"),
		strings.ReplaceAll(target, "'", "''"),
	)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} // 隐藏控制台窗口（CREATE_NO_WINDOW 与 HideWindow 互斥，只留 HideWindow）
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
