//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"backend/internal/handler"
)

// ensureAutoStart 按用户偏好注册/移除 HKCU Run 开机自启（原生方案，无需管理员）。
// 仅正式版执行——开发机裸 build 不写注册表，避免抢 8080/打扰开发环境（2026-08-18 定稿）。
// 偏好来自设置面板「开机自启动」开关（落盘 ~/rescene_data/desktop_prefs.json）：
// 用户关掉后下次启动不再偷偷写回注册表（2026-09-04，issue #14）。
func ensureAutoStart() error {
	v := handler.AppVersion
	if v == "" || v == "0.0.0-dev" {
		return nil
	}
	if !handler.AutoStartDesired() {
		// 用户明确关闭：确保注册表里没有残留项（幂等，不存在即成功）。
		if err := handler.DisableAutoStart(); err != nil {
			return err
		}
		return nil
	}
	if err := handler.EnableAutoStart(); err != nil {
		return err
	}
	logAutoStart("已确认开机自启")
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
