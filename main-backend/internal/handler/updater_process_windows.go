//go:build windows

package handler

import (
	"os"
	"os/exec"
	"syscall"
)

// launchUpdateScript 直接拉起隐藏的 cmd 子进程，不再通过 start 打开可见终端窗口。
// CREATE_NO_WINDOW 同时避免 Windows Terminal 为批处理创建新的黑窗。
// ⚠️ scriptPath 不手动加引号（2026-08-15 实测纠错）：Go 的 exec.Command 在 Windows 上
// 用 EscapeArg 拼接命令行，含空格参数会自动加引号；手动加会变双重引号 → syntax error。
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
