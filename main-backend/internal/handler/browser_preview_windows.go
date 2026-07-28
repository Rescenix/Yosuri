//go:build windows

package handler

import (
	"os/exec"
	"syscall"
)

// setHideWindow 在 Windows 上隐藏被拉起的 Chrome 进程窗口，避免无头模式下闪现控制台。
func setHideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
