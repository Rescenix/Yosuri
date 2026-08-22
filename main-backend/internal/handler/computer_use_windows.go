//go:build windows

package handler

// Windows 原生实现：使用 GDI + SendInput API，无需 CGO / robotgo。
// 截图通过 CreateCompatibleDC / BitBlt 完成，鼠标键盘通过 SendInput。

import (
	"fmt"
	"image"
	"image/color"
	"os/exec"
	"syscall"
	"unsafe"
)

var (
	user32 = syscall.NewLazyDLL("user32.dll")
	gdi32  = syscall.NewLazyDLL("gdi32.dll")

	// GDI
	getDC           = gdi32.NewProc("GetDC")
	releaseDC       = gdi32.NewProc("ReleaseDC")
	createDC        = gdi32.NewProc("CreateDCW")
	deleteDC        = gdi32.NewProc("DeleteDC")
	createCompatibleDC   = gdi32.NewProc("CreateCompatibleDC")
	deleteDC2       = gdi32.NewProc("DeleteDC")
	deleteObject    = gdi32.NewProc("DeleteObject")
	createCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	selectObject    = gdi32.NewProc("SelectObject")
	bitBlt          = gdi32.NewProc("BitBlt")
	getDIBits       = gdi32.NewProc("GetDIBits")
	getDeviceCaps   = gdi32.NewProc("GetDeviceCaps")
	createDIBSection = gdi32.NewProc("CreateDIBSection")

	// User32
	getDC2          = user32.NewProc("GetDC")
	releaseDC2       = user32.NewProc("ReleaseDC")
	getSystemMetrics = user32.NewProc("GetSystemMetrics")
	getDesktopWindow = user32.NewProc("GetDesktopWindow")
	getWindowDC     = user32.NewProc("GetWindowDC")
	getForegroundWindow = user32.NewProc("GetForegroundWindow")
	getWindowRect   = user32.NewProc("GetWindowRect")
	getWindowTextW  = user32.NewProc("GetWindowTextW")
	sendInput       = user32.NewProc("SendInput")
)

const (
	SRCCOPY          = 0x00CC0020
	DIB_RGB_COLORS   = 0
	BI_RGB           = 0
	SM_CXSCREEN      = 0
	SM_CYSCREEN      = 1
	SM_CMONITORS     = 80

	INPUT_MOUSE      = 0
	INPUT_KEYBOARD   = 1

	MOUSEEVENTF_MOVE        = 0x0001
	MOUSEEVENTF_ABSOLUTE    = 0x8000
	MOUSEEVENTF_LEFTDOWN    = 0x0002
	MOUSEEVENTF_LEFTUP      = 0x0004
	MOUSEEVENTF_RIGHTDOWN   = 0x0008
	MOUSEEVENTF_RIGHTUP     = 0x0010
	MOUSEEVENTF_MIDDLEDOWN  = 0x0020
	MOUSEEVENTF_MIDDLEUP    = 0x0040
	MOUSEEVENTF_WHEEL       = 0x0800

	KEYEVENTF_KEYDOWN       = 0x0000
	KEYEVENTF_KEYUP         = 0x0002
	KEYEVENTF_SCANCODE      = 0x0008
)

type BITMAPINFOHEADER struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type BITMAPINFO struct {
	Header BITMAPINFOHEADER
	Colors [1]uint32
}

type MOUSEINPUT struct {
	Dx          int32
	Dy          int32
	MouseData   uint32
	Flags       uint32
	Time        uint32
	ExtraInfo   uintptr
}

type KEYBDINPUT struct {
	WVk         uint16
	WScan       uint16
	Flags       uint32
	Time        uint32
	ExtraInfo   uintptr
}

type HARDWAREINPUT struct {
	UMsg    uint32
	WParamL  uint16
	WParamH  uint16
}

type INPUT struct {
	Type uint32
	_    [4]byte // padding
	Mi   MOUSEINPUT
}

type INPUT_KB struct {
	Type uint32
	_    [4]byte
	Ki   KEYBDINPUT
}

// ----- init -----
func init() {
	robotgoCaptureScreen = winCaptureScreen
	robotgoCaptureWindow = winCaptureWindow
	robotgoGetActivePID = winGetActivePID
	robotgoToImage = winBitmapToImage
	robotgoFreeBitmap = winFreeBitmap
	robotgoMoveMouse = winMoveMouse
	robotgoClick = winClick
	robotgoDoubleClick = winDoubleClick
	robotgoDrag = winDrag
	robotgoTypeStr = winTypeStr
	robotgoKeyDown = winKeyDown
	robotgoKeyUp = winKeyUp
	robotgoKeyTap = winKeyTap
	robotgoGetScreenSize = winGetScreenSize
	robotgoGetDisplayCount = winGetDisplayCount
	robotgoScroll = winScroll
}

// ----- 截图 -----

func winCaptureScreen() *robotgoBitmap {
	w := int(getSysMetrics(SM_CXSCREEN))
	h := int(getSysMetrics(SM_CYSCREEN))
	return captureRect(0, 0, w, h)
}

func winCaptureWindow(pid int) *robotgoBitmap {
	// 用前台窗口做近似
	hwnd, _, _ := getForegroundWindow.Call()
	if hwnd == 0 {
		return winCaptureScreen()
	}
	rect := getWindowRect_(hwnd)
	if rect.right <= rect.left || rect.bottom <= rect.top {
		return winCaptureScreen()
	}
	return captureRect(int(rect.left), int(rect.top),
		int(rect.right-rect.left), int(rect.bottom-rect.top))
}

func winGetActivePID() int {
	hwnd, _, _ := getForegroundWindow.Call()
	return int(hwnd)
}

func captureRect(x, y, w, h int) *robotgoBitmap {
	if w <= 0 || h <= 0 {
		return nil
	}

	hwnd, _, _ := getDesktopWindow.Call()
	hdc, _, _ := getDC2.Call(hwnd)
	if hdc == 0 {
		return nil
	}
	defer releaseDC2.Call(hwnd, hdc)

	memDC, _, _ := createCompatibleDC.Call(hdc)
	if memDC == 0 {
		return nil
	}
	defer deleteDC2.Call(memDC)

	hbmp, _, _ := createCompatibleBitmap.Call(hdc, uintptr(w), uintptr(h))
	if hbmp == 0 {
		return nil
	}
	defer deleteObject.Call(hbmp)

	selectObject.Call(memDC, hbmp)

	bitBlt.Call(memDC, 0, 0, uintptr(w), uintptr(h), hdc, uintptr(x), uintptr(y), SRCCOPY)

	// 构建 BITMAPINFO
	bmi := BITMAPINFO{
		Header: BITMAPINFOHEADER{
			Size:    uint32(unsafe.Sizeof(BITMAPINFOHEADER{})),
			Width:   int32(w),
			Height:  int32(-h), // 负值 = 从上到下，不翻转
			Planes:  1,
			BitCount: 32,
		},
	}

	pixels := make([]byte, w*h*4)
	getDIBits.Call(hdc, hbmp, 0, uintptr(h), uintptr(unsafe.Pointer(&pixels[0])),
		uintptr(unsafe.Pointer(&bmi)), DIB_RGB_COLORS)

	return &robotgoBitmap{
		Width:  w,
		Height: h,
		Bytes:  pixels,
	}
}

func winBitmapToImage(b *robotgoBitmap) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, b.Width, b.Height))
	// BGRA → RGBA 转换
	for y := 0; y < b.Height; y++ {
		for x := 0; x < b.Width; x++ {
			i := (y*b.Width + x) * 4
			// Windows GDI 返回 BGRA
			img.Set(x, y, color.RGBA{
				R: b.Bytes[i+2],
				G: b.Bytes[i+1],
				B: b.Bytes[i+0],
				A: 255,
			})
		}
	}
	return img
}

func winFreeBitmap(b *robotgoBitmap) {
	// Bytes 由 Go GC 管理，无需手动释放
}

// ----- 鼠标 -----

func winMoveMouse(x, y int) {
	// 计算绝对坐标（0-65535 范围）
	screenW := getSysMetrics(SM_CXSCREEN)
	screenH := getSysMetrics(SM_CYSCREEN)
	absX := uint32(int32(x) * 65535 / int32(maxInt(screenW-1, 1)))
	absY := uint32(int32(y) * 65535 / int32(maxInt(screenH-1, 1)))

	inp := INPUT{
		Type: INPUT_MOUSE,
		Mi: MOUSEINPUT{
			Dx:        int32(absX),
			Dy:        int32(absY),
			Flags:     MOUSEEVENTF_MOVE | MOUSEEVENTF_ABSOLUTE,
		},
	}
	sendInput.Call(1, uintptr(unsafe.Pointer(&inp)), unsafe.Sizeof(inp))
}

func winClick(button string) {
	var downFlag uint32 = MOUSEEVENTF_LEFTDOWN
	var upFlag uint32 = MOUSEEVENTF_LEFTUP
	switch button {
	case "right":
		downFlag = MOUSEEVENTF_RIGHTDOWN
		upFlag = MOUSEEVENTF_RIGHTUP
	case "center":
		downFlag = MOUSEEVENTF_MIDDLEDOWN
		upFlag = MOUSEEVENTF_MIDDLEUP
	}
	winSendMouseButton(downFlag)
	winSendMouseButton(upFlag)
}

func winDoubleClick(button string) {
	winClick(button)
	winClick(button)
}

func winSendMouseButton(flag uint32) {
	inp := INPUT{
		Type: INPUT_MOUSE,
		Mi: MOUSEINPUT{
			Flags: flag,
		},
	}
	sendInput.Call(1, uintptr(unsafe.Pointer(&inp)), unsafe.Sizeof(inp))
}

func winDrag(x, y int) {
	// 先按下左键
	winSendMouseButton(MOUSEEVENTF_LEFTDOWN)
	// 移动到目标位置
	winMoveMouse(x, y)
	// 释放左键
	winSendMouseButton(MOUSEEVENTF_LEFTUP)
}

// ----- 键盘 -----

// 虚拟键码映射（常用键）
var keyCodeMap = map[string]uint16{
	"enter":     0x0D,
	"return":    0x0D,
	"tab":       0x09,
	"escape":    0x1B,
	"esc":       0x1B,
	"backspace": 0x08,
	"delete":    0x2E,
	"del":       0x2E,
	"home":      0x24,
	"end":       0x23,
	"pageup":    0x21,
	"pgup":      0x21,
	"pagedown":  0x22,
	"pgdn":      0x22,
	"up":        0x26,
	"down":      0x28,
	"left":      0x25,
	"right":     0x27,
	"space":     0x20,
	"ctrl":      0x11,
	"control":   0x11,
	"alt":       0x12,
	"shift":     0x10,
	"capslock":  0x14,
	"f1":        0x70,
	"f2":        0x71,
	"f3":        0x72,
	"f4":        0x73,
	"f5":        0x74,
	"f6":        0x75,
	"f7":        0x76,
	"f8":        0x77,
	"f9":        0x78,
	"f10":       0x79,
	"f11":       0x7A,
	"f12":       0x7B,
	"0":         0x30,
	"1":         0x31,
	"2":         0x32,
	"3":         0x33,
	"4":         0x34,
	"5":         0x35,
	"6":         0x36,
	"7":         0x37,
	"8":         0x38,
	"9":         0x39,
	"a":         0x41,
	"b":         0x42,
	"c":         0x43,
	"d":         0x44,
	"e":         0x45,
	"f":         0x46,
	"g":         0x47,
	"h":         0x48,
	"i":         0x49,
	"j":         0x4A,
	"k":         0x4B,
	"l":         0x4C,
	"m":         0x4D,
	"n":         0x4E,
	"o":         0x4F,
	"p":         0x50,
	"q":         0x51,
	"r":         0x52,
	"s":         0x53,
	"t":         0x54,
	"u":         0x55,
	"v":         0x56,
	"w":         0x57,
	"x":         0x58,
	"y":         0x59,
	"z":         0x5A,
	";":         0xBA,
	"'":         0xDE,
	",":         0xBC,
	".":         0xBE,
	"/":         0xBF,
	"`":         0xC0,
	"-":         0xBD,
	"=":         0xBB,
	"[":         0xDB,
	"]":         0xDD,
	"\\":        0xDC,
}

func winTypeStr(text string) {
	for _, r := range text {
		vk, ok := keyCodeMap[stringsToLower(string(r))]
		if ok {
			winKeyTapCode(vk)
		} else {
			// 用剪贴板粘贴处理 Unicode（比逐字符 SendInput 可靠）
			winPasteText(string(r))
		}
	}
}

func winPasteText(text string) {
	// 保存当前剪贴板
	// 设置文本到剪贴板
	winSetClipboard(text)
	// Ctrl+V
	winKeyDownCode(0x11) // Ctrl
	winKeyTapCode(0x56)  // V
	winKeyUpCode(0x11)
}

func winSetClipboard(text string) {
	// 使用 PowerShell 设置剪贴板
	escaped := stringsReplaceAll(text, "'", "''")
	execCmd := exec.Command("powershell", "-NoLogo", "-NoProfile", "-Command", fmt.Sprintf("Set-Clipboard -Value '%s'", escaped))
	execCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	_ = execCmd.Run()
}

func winKeyTap(key string) {
	if vk, ok := keyCodeMap[stringsToLower(key)]; ok {
		winKeyTapCode(vk)
	}
}

func winKeyDown(key string) {
	if vk, ok := keyCodeMap[stringsToLower(key)]; ok {
		winKeyDownCode(vk)
	}
}

func winKeyUp(key string) {
	if vk, ok := keyCodeMap[stringsToLower(key)]; ok {
		winKeyUpCode(vk)
	}
}

func winKeyTapCode(vk uint16) {
	winSendKeyboard(vk, KEYEVENTF_KEYDOWN)
	winSendKeyboard(vk, KEYEVENTF_KEYUP)
}

func winKeyDownCode(vk uint16) {
	winSendKeyboard(vk, KEYEVENTF_KEYDOWN)
}

func winKeyUpCode(vk uint16) {
	winSendKeyboard(vk, KEYEVENTF_KEYUP)
}

func winSendKeyboard(vk uint16, flags uint32) {
	inp := INPUT_KB{
		Type: INPUT_KEYBOARD,
		Ki: KEYBDINPUT{
			WVk:   vk,
			Flags: flags,
		},
	}
	sendInput.Call(1, uintptr(unsafe.Pointer(&inp)), unsafe.Sizeof(inp))
}

// ----- 屏幕信息 -----

func winGetScreenSize() (int, int) {
	w := int(getSysMetrics(SM_CXSCREEN))
	h := int(getSysMetrics(SM_CYSCREEN))
	return w, h
}

func winGetDisplayCount() int {
	return int(getSysMetrics(SM_CMONITORS))
}

func getSysMetrics(index int) int {
	ret, _, _ := getSystemMetrics.Call(uintptr(index))
	return int(ret)
}

// ----- 滚动 -----

func winScroll(x, y int) {
	inp := INPUT{
		Type: INPUT_MOUSE,
		Mi: MOUSEINPUT{
			MouseData: uint32(y * 120), // WHEEL_DELTA = 120
			Flags:     MOUSEEVENTF_WHEEL,
		},
	}
	sendInput.Call(1, uintptr(unsafe.Pointer(&inp)), unsafe.Sizeof(inp))
}

// ----- 辅助函数 -----

type RECT struct {
	left, top, right, bottom int32
}

func getWindowRect_(hwnd uintptr) RECT {
	var rect RECT
	getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
	return rect
}

// strings 包函数别名的 go:build 安全版本
func stringsToLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		b[i] = c
	}
	return string(b)
}

func stringsReplaceAll(s, old, new string) string {
	// 简单实现
	result := make([]byte, 0, len(s)*2)
	for i := 0; i < len(s); i++ {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			result = append(result, new...)
			i += len(old) - 1
		} else {
			result = append(result, s[i])
		}
	}
	return string(result)
}