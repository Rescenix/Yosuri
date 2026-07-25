//go:build windows

package handler

import (
	"os/exec"
)

// stopLlamaProcess Windows 平台：直接 Kill 子进程。
// Windows 无 POSIX 进程组概念；主进程退出信号（RegisterLlamaCleanupOnExit）
// 会显式调用本函数回收。llama-server.exe 在父进程退出时通常也会被系统回收，
// 这里再显式 kill 一次，确保不残留占用内存/GPU 的进程。
func stopLlamaProcess(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}

// ensureLlamaProcessGroup Windows 平台：无进程组概念，留空即可。
func ensureLlamaProcessGroup(cmd *exec.Cmd) {}
