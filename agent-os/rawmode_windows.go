//go:build windows

package main

// rawmode_windows.go — Windows 控制台 raw mode（kernel32 API）

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32                    = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode          = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode          = kernel32.NewProc("SetConsoleMode")
	procGetNumEvents            = kernel32.NewProc("GetNumberOfConsoleInputEvents")
	procGetConsoleScreenBufferInfo = kernel32.NewProc("GetConsoleScreenBufferInfo")
)

type coord struct {
	X, Y int16
}

type smallRect struct {
	Left, Top, Right, Bottom int16
}

type consoleScreenBufferInfo struct {
	dwSize              coord
	dwCursorPosition    coord
	wAttributes         uint16
	srWindow            smallRect
	dwMaximumWindowSize coord
}

// terminalWidth 获取控制台窗口宽度（字符列数）
func terminalWidth() int {
	var csbi consoleScreenBufferInfo
	r, _, _ := procGetConsoleScreenBufferInfo.Call(os.Stdout.Fd(), uintptr(unsafe.Pointer(&csbi)))
	if r == 0 {
		return 80
	}
	return int(csbi.srWindow.Right - csbi.srWindow.Left + 1)
}

const (
	enableLineInput    = 0x0002
	enableEchoInput    = 0x0004
	enableProcessedInp = 0x0001
	enableVT           = 0x0004 // ENABLE_VIRTUAL_TERMINAL_PROCESSING（stdout）
	enableVTInput      = 0x0200 // ENABLE_VIRTUAL_TERMINAL_INPUT（stdin 方向键→ESC序列）
)

// isTerminal 判断 stdin 是否为交互式控制台
func isTerminal() bool {
	var mode uint32
	r, _, _ := procGetConsoleMode.Call(os.Stdin.Fd(), uintptr(unsafe.Pointer(&mode)))
	return r != 0
}

// enableRawMode 关闭行缓冲与回显，启用 VT 转义（stdout 处理 + stdin 方向键）；返回恢复函数
func enableRawMode() func() {
	// stdin: 关闭 行输入/回显/进程内 Ctrl+C 处理（改为读到字节 0x03）
	//         开启 VT 输入：方向键产生 ESC[A-D 序列
	var inMode uint32
	procGetConsoleMode.Call(os.Stdin.Fd(), uintptr(unsafe.Pointer(&inMode)))
	oldIn := inMode
	inMode &^= enableLineInput | enableEchoInput | enableProcessedInp
	inMode |= enableVTInput
	procSetConsoleMode.Call(os.Stdin.Fd(), uintptr(inMode))

	// stdout: 启用 VT 转义处理（\x1b[A / \x1b[J 等光标控制必需）
	var outMode uint32
	procGetConsoleMode.Call(os.Stdout.Fd(), uintptr(unsafe.Pointer(&outMode)))
	oldOut := outMode
	outMode |= enableVT
	procSetConsoleMode.Call(os.Stdout.Fd(), uintptr(outMode))

	return func() {
		procSetConsoleMode.Call(os.Stdin.Fd(), uintptr(oldIn))
		procSetConsoleMode.Call(os.Stdout.Fd(), uintptr(oldOut))
	}
}

// inputAvailable 非阻塞探测 stdin 是否有待处理输入事件
// 用于区分「独立 Esc 键」与「ESC[ 方向键序列」：独立 Esc 后没有后续事件 → 直接作为 Esc
func inputAvailable() bool {
	var n uint32
	r, _, _ := procGetNumEvents.Call(os.Stdin.Fd(), uintptr(unsafe.Pointer(&n)))
	if r == 0 {
		return true // 无法探测时保守返回 true（走正常读取路径）
	}
	return n > 0
}
