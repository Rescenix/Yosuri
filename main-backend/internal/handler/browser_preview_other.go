//go:build !windows

package handler

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// configurePreviewProcess 建立独立进程组，退出时可以连同 Chromium 子进程一起回收。
func configurePreviewProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func stopPreviewProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err == nil {
		if err = syscall.Kill(-pgid, syscall.SIGKILL); err == nil {
			return nil
		}
	}
	err = cmd.Process.Kill()
	if errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	return err
}
