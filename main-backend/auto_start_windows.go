//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"

	"backend/internal/handler"
)

// autoStartRunKeyName 是 HKCU 开机自启项的注册表值名。
const autoStartRunKeyName = "ResceneAgent"

// ensureAutoStart 把应用自己注册进 HKCU Run（原生 Windows 自启，无需管理员）。
// 仅正式版（AppVersion 已注入且非 dev 占位）执行——开发机裸 build 不写注册表，
// 避免抢 8080/打扰开发环境（2026-08-18 定稿）。
func ensureAutoStart() error {
	v := handler.AppVersion
	if v == "" || v == "0.0.0-dev" {
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取自身路径失败: %w", err)
	}
	// 路径带空格（Programs\Rescene Agent\）必须引号包裹；--background 静默常驻，
	// 用户手动双击图标时 SingleInstanceLock 会把窗口转发给已有实例（showWindow）。
	cmd := fmt.Sprintf(`"%s" --background`, exe)

	// 幂等：值已一致就不写（保留用户可能的手动修改），OpenKey 时写权限请求，
	// 不存在则创建。
	k, _, err := registry.CreateKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("打开自启注册表失败: %w", err)
	}
	defer k.Close()
	cur, _, _ := k.GetStringValue(autoStartRunKeyName)
	if strings.EqualFold(cur, cmd) {
		return nil
	}
	if err := k.SetStringValue(autoStartRunKeyName, cmd); err != nil {
		return fmt.Errorf("写入自启注册表失败: %w", err)
	}
	logAutoStart("已写入开机自启: " + cmd)
	return nil
}

// removeAutoStart 删除自启项（供设置面板开关用，暂未接 UI）。
func removeAutoStart() error {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return nil
		}
		return fmt.Errorf("打开自启注册表失败: %w", err)
	}
	defer k.Close()
	if err := k.DeleteValue(autoStartRunKeyName); err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("删除自启注册表失败: %w", err)
	}
	logAutoStart("已移除开机自启")
	return nil
}

func logAutoStart(msg string) {
	// 与主程序日志同风格；路径里有 exe 名，仅记状态不记完整值以少刷屏。
	fmt.Fprintf(os.Stderr, "[autostart] %s (exe=%s)\n", msg, filepath.Base(autoStartExeName()))
}

func autoStartExeName() string {
	if exe, err := os.Executable(); err == nil {
		return exe
	}
	return "rescene.exe"
}