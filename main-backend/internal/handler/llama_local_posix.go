//go:build !windows

package handler

import (
	"fmt"
	"os/exec"
	"syscall"
)

// stopLlamaProcess 非 Windows 平台：通过进程组 ID（负的 pid）一次性 kill 整个组，
// 连 llama-server 的子孙进程一起带走，避免主进程被 kill 后 llama 变孤儿继续占内存。
func stopLlamaProcess(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err == nil {
		// 负 pid 表示向整个进程组发信号
		if kerr := syscall.Kill(-pgid, syscall.SIGKILL); kerr == nil {
			return nil
		}
	}
	// 退路：直接 kill 子进程
	return cmd.Process.Kill()
}

// ensureLlamaProcessGroup 在启动 llama-server 时建立独立进程组，
// 使主进程退出时可以通过 stopLlamaProcess 的进程组 kill 一并清理。
func ensureLlamaProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

var _ = fmt.Sprintf
