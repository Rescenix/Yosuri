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
)

// ensureDesktopShortcuts 确保桌面/开始菜单/启动文件夹的 Yosuri 快捷方式存在。
// 每次启动自愈（2026-09-01 实锤）：hotpatch 只替换 exe、不跑 NSIS 安装器，
// 导致升级用户开始菜单从未有快捷方式（用户原话「开始菜单从没有建立过快捷方式」）。
// 同时清理旧品牌（Rescene/ResceneAgent）残留快捷方式。
func ensureDesktopShortcuts() {
	exePath, err := os.Executable()
	if err != nil {
		log.Printf("⚠️ 修复快捷方式失败：定位 exe：%v", err)
		return
	}
	exeDir := filepath.Dir(exePath)
	shortcutName := "Yosuri"
	oldNames := []string{"Rescene", "ResceneAgent"}

	// 目标目录：桌面 + 开始菜单 Programs + 启动文件夹
	dirs := []string{
		startMenuProgramsDir(),
		desktopDir(),
		startupDir(),
	}
	created := 0
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		// 删除旧品牌快捷方式（含 .bak 备份）
		for _, old := range oldNames {
			oldLnk := filepath.Join(dir, old+".lnk")
			if _, err := os.Stat(oldLnk); err == nil {
				_ = os.Rename(oldLnk, oldLnk+".bak-"+time.Now().Format("0102"))
				log.Printf("🔧 已清理旧快捷方式 %s", oldLnk)
			}
		}
		lnkPath := filepath.Join(dir, shortcutName+".lnk")
		if _, err := os.Stat(lnkPath); err == nil {
			continue // 已存在，不动
		}
		if err := createWindowsShortcut(lnkPath, exePath, exeDir); err != nil {
			log.Printf("⚠️ 创建快捷方式失败 %s：%v", lnkPath, err)
			continue
		}
		created++
		log.Printf("🔧 已创建快捷方式 %s", lnkPath)
	}
	if created > 0 {
		// 通知 Explorer 刷新图标缓存（隐藏窗口）
		refreshCmd := exec.Command("cmd", "/c", "ie4uinit.exe", "-show")
		refreshCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		_ = refreshCmd.Run()
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
