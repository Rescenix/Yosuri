//go:build !windows

package handler

// 非 Windows 平台没有注册表自启机制，接口保持可用但恒为「不支持」，
// 前端据此隐藏开关（与 auto_start_other.go / desktop_tray_other.go 同模式）。

func autoStartSupported() bool { return false }

func autoStartRegistered() bool { return false }

func enableAutoStart() error { return nil }

func disableAutoStart() error { return nil }
