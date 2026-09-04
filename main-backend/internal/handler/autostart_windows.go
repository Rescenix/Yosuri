//go:build windows

package handler

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// 开机自启：HKCU\...\Run 注册表项（原生方案，无需管理员权限）。
// 值名与安装器写入的一致，改这里要同步看 main-backend/auto_start_windows.go。
const autoStartRunKeyName = "ResceneAgent"

func autoStartSupported() bool { return true }

// autoStartRunKey 打开（必要时创建）HKCU Run 键。
func autoStartRunKey(write bool) (registry.Key, error) {
	access := uint32(registry.QUERY_VALUE)
	if write {
		access = registry.SET_VALUE
	}
	k, _, err := registry.CreateKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Run`, access)
	if err != nil {
		return 0, fmt.Errorf("打开自启注册表失败: %w", err)
	}
	return k, nil
}

// autoStartCommand 是自启时写入的命令行：带 --background 静默常驻。
func autoStartCommand() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("获取自身路径失败: %w", err)
	}
	return fmt.Sprintf(`"%s" --background`, exe), nil
}

// autoStartRegistered 注册表里当前是否已有自启项。
func autoStartRegistered() bool {
	k, err := autoStartRunKey(false)
	if err != nil {
		return false
	}
	defer k.Close()
	v, _, err := k.GetStringValue(autoStartRunKeyName)
	return err == nil && strings.TrimSpace(v) != ""
}

// enableAutoStart 写入自启项（幂等：值已一致则跳过）。
func enableAutoStart() error {
	cmd, err := autoStartCommand()
	if err != nil {
		return err
	}
	k, err := autoStartRunKey(true)
	if err != nil {
		return err
	}
	defer k.Close()
	if cur, _, _ := k.GetStringValue(autoStartRunKeyName); strings.EqualFold(cur, cmd) {
		return nil
	}
	if err := k.SetStringValue(autoStartRunKeyName, cmd); err != nil {
		return fmt.Errorf("写入开机自启失败: %w", err)
	}
	return nil
}

// disableAutoStart 删除自启项（不存在视为成功）。
func disableAutoStart() error {
	k, err := autoStartRunKey(true)
	if err != nil {
		return err
	}
	defer k.Close()
	if err := k.DeleteValue(autoStartRunKeyName); err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("删除开机自启失败: %w", err)
	}
	return nil
}
