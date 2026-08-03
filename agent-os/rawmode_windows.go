//go:build windows

package main

// rawmode_windows.go — Windows 控制台 raw mode（kernel32 API）

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode = kernel32.NewProc("SetConsoleMode")
)

const (
	enableLineInput    = 0x0002
	enableEchoInput    = 0x0004
	enableProcessedInp = 0x0001
)

// isTerminal 判断 stdin 是否为交互式控制台
func isTerminal() bool {
	var mode uint32
	r, _, _ := procGetConsoleMode.Call(os.Stdin.Fd(), uintptr(unsafe.Pointer(&mode)))
	return r != 0
}

// enableRawMode 关闭行缓冲与回显；返回恢复函数
func enableRawMode() func() {
	var mode uint32
	procGetConsoleMode.Call(os.Stdin.Fd(), uintptr(unsafe.Pointer(&mode)))
	old := mode

	// 关闭 行输入/回显/进程内 Ctrl+C 处理（改为读到字节 0x03）
	mode &^= enableLineInput | enableEchoInput | enableProcessedInp
	procSetConsoleMode.Call(os.Stdin.Fd(), uintptr(mode))

	return func() {
		procSetConsoleMode.Call(os.Stdin.Fd(), uintptr(old))
	}
}
