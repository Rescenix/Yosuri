//go:build !windows

package handler

// Stub 实现：非 Windows 平台上的空操作（不做任何实际桌面操作）。
// 截图返回 1x1 占位图，鼠标键盘操作静默忽略。

import (
	"image"
)

func init() {
	robotgoCaptureScreen = func() *robotgoBitmap {
		return &robotgoBitmap{Width: 1, Height: 1, Bytes: []byte{0, 0, 0, 255}}
	}
	robotgoCaptureWindow = func(pid int) *robotgoBitmap {
		return &robotgoBitmap{Width: 1, Height: 1, Bytes: []byte{0, 0, 0, 255}}
	}
	robotgoGetActivePID = func() int { return 0 }
	robotgoToImage = func(b *robotgoBitmap) image.Image {
		return image.NewRGBA(image.Rect(0, 0, 1, 1))
	}
	robotgoFreeBitmap = func(b *robotgoBitmap) {}
	robotgoMoveMouse = func(x, y int) {}
	robotgoClick = func(button string) {}
	robotgoDoubleClick = func(button string) {}
	robotgoDrag = func(x, y int) {}
	robotgoTypeStr = func(text string) {}
	robotgoKeyDown = func(key string) {}
	robotgoKeyUp = func(key string) {}
	robotgoKeyTap = func(key string) {}
	robotgoGetScreenSize = func() (int, int) { return 1920, 1080 }
	robotgoGetDisplayCount = func() int { return 1 }
	robotgoScroll = func(x, y int) {}

	// captureFullScreen / captureActiveWindow 是 computer_use_tool.go 里的普通函数
	// （不是可重新赋值的变量），无需在此覆写：它们已经调用上面 stub 掉的
	// robotgo* 变量返回 1x1 占位图，非 Windows 上会自动走到「全屏截图」分支。
}
