//go:build !windows

package handler

import "os/exec"

// setHideWindow 非 Windows 平台无需隐藏窗口，保持 SysProcAttr 为零值。
func setHideWindow(cmd *exec.Cmd) {}
