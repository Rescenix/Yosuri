//go:build windows

package handler

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

// configurePreviewProcess 在 Windows 上隐藏 Chromium 可能闪现的控制台窗口。
func configurePreviewProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}

// stopPreviewProcess 使用 taskkill /T 回收 Chromium 的 renderer/GPU 等整个子进程树；
// taskkill 不可用或进程树已变化时，退回直接结束主进程。
func stopPreviewProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	killer := exec.Command("taskkill", "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F")
	killer.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := killer.Run(); err == nil {
		return nil
	}
	err := cmd.Process.Kill()
	if errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	return err
}
