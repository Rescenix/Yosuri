//go:build windows

package handler

import (
	"os"
	"os/exec"
	"syscall"
)

// launchUpdateScript 直接拉起隐藏的 cmd 子进程，不再通过 start 打开可见终端窗口。
// CREATE_NO_WINDOW 同时避免 Windows Terminal 为批处理创建新的黑窗。
func launchUpdateScript(scriptPath string) error {
	comspec := os.Getenv("ComSpec")
	if comspec == "" {
		comspec = "cmd.exe"
	}
	cmd := exec.Command(comspec, "/d", "/c", "call", scriptPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
	return cmd.Start()
}
