//go:build !windows

package main

// tools_computer_stub.go — 非 Windows 平台空实现（从 computer_use_stub.go 移植）。
// 截图返回 1x1 占位图，鼠标键盘操作静默忽略。

import (
	"fmt"
	"image"
	"image/color"
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

	captureFullScreen = func() (image.Image, error) {
		img := image.NewRGBA(image.Rect(0, 0, 1, 1))
		img.Set(0, 0, color.Black)
		return img, fmt.Errorf("Computer Use 仅在 Windows 上可用")
	}
	captureActiveWindow = func() (image.Image, error) {
		img := image.NewRGBA(image.Rect(0, 0, 1, 1))
		img.Set(0, 0, color.Black)
		return img, fmt.Errorf("Computer Use 仅在 Windows 上可用")
	}
}
