//go:build !windows

package main

// rawmode_unix.go — POSIX raw mode（termios，Linux/macOS）

import (
	"os"
	"syscall"
	"unsafe"
)

// isTerminal 判断 stdin 是否为 TTY
func isTerminal() bool {
	var t syscall.Termios
	_, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, os.Stdin.Fd(),
		uintptr(syscall.TCGETS), uintptr(unsafe.Pointer(&t)), 0, 0, 0)
	return errno == 0
}

// enableRawMode 关闭 ICANON/ECHO/ISIG，逐键读取；返回恢复函数
func enableRawMode() func() {
	fd := os.Stdin.Fd()
	var old syscall.Termios
	syscall.Syscall6(syscall.SYS_IOCTL, fd, uintptr(syscall.TCGETS),
		uintptr(unsafe.Pointer(&old)), 0, 0, 0)

	raw := old
	raw.Lflag &^= syscall.ICANON | syscall.ECHO | syscall.ISIG
	raw.Iflag &^= syscall.ICRNL | syscall.IXON
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0
	syscall.Syscall6(syscall.SYS_IOCTL, fd, uintptr(syscall.TCSETS),
		uintptr(unsafe.Pointer(&raw)), 0, 0, 0)

	return func() {
		syscall.Syscall6(syscall.SYS_IOCTL, fd, uintptr(syscall.TCSETS),
			uintptr(unsafe.Pointer(&old)), 0, 0, 0)
	}
}

// inputAvailable 非阻塞探测 stdin 是否有待读字节（select timeout=0）
func inputAvailable() bool {
	var rfds syscall.FdSet
	fd := int(os.Stdin.Fd())
	rfds.Bits[fd/64] |= 1 << uint(fd%64)

	tv := syscall.Timeval{}
	n, err := syscall.Select(fd+1, &rfds, nil, nil, &tv)
	if err != nil {
		return true // 无法探测时保守返回 true
	}
	return n > 0
}
