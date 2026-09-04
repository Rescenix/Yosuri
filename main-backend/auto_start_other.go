//go:build !windows

package main

// ensureAutoStart 非 Windows 平台无注册表自启，空实现（与 desktop_tray_other.go 同模式）。
func ensureAutoStart() error { return nil }
