package main

// ansiart.go — 终端 ANSI 真彩图渲染
// 用半块字符(▀)在终端显示彩色图片，不需要外部依赖

import (
	"fmt"
	"image"
	_ "image/png"
	"os"
	"strings"
)

// RenderANSIArt 将图片渲染为 ANSI 真彩字符串（半块字符 ▀）
// 每个字符 = 2 像素垂直（上半 = 前景色，下半 = 背景色）
// 暗色像素用真实颜色渲染，不砍掉，形成连续画面
func RenderANSIArt(path string, maxWidth int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return "", err
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	if maxWidth <= 0 {
		maxWidth = 55
	}

	// 计算缩放比例
	charW := maxWidth
	charH := int(float64(h) / float64(w) * float64(charW) * 0.5)
	if charH < 5 {
		charH = 5
	}
	pixelW := charW
	pixelH := charH * 2

	// 缩放图片
	scaled := scaleImage(img, pixelW, pixelH)

	var sb strings.Builder
	// 紫色主题色边框
	sb.WriteString("\033[38;2;150;120;220m")

	for y := 0; y < pixelH; y += 2 {
		sb.WriteString("  ")
		for x := 0; x < pixelW; x++ {
			r1, g1, b1, _ := scaled.At(x, y).RGBA()
			r1 = r1 >> 8
			g1 = g1 >> 8
			b1 = b1 >> 8

			if y+1 < pixelH {
				r2, g2, b2, _ := scaled.At(x, y+1).RGBA()
				r2 = r2 >> 8
				g2 = g2 >> 8
				b2 = b2 >> 8

				// 用半块字符，上下各一个颜色
				sb.WriteString(fmt.Sprintf("\033[38;2;%d;%d;%d;48;2;%d;%d;%dm▀",
					r1, g1, b1, r2, g2, b2))
			} else {
				sb.WriteString(fmt.Sprintf("\033[38;2;%d;%d;%dm▀", r1, g1, b1))
			}
		}
		sb.WriteString("\033[0m\n")
	}
	sb.WriteString("\033[0m")

	return sb.String(), nil
}

// scaleImage 双线性插值缩放
func scaleImage(img image.Image, newW, newH int) image.Image {
	bounds := img.Bounds()
	oldW := bounds.Dx()
	oldH := bounds.Dy()

	out := image.NewRGBA(image.Rect(0, 0, newW, newH))

	for y := 0; y < newH; y++ {
		for x := 0; x < newW; x++ {
			srcX := float64(x) * float64(oldW) / float64(newW)
			srcY := float64(y) * float64(oldH) / float64(newH)

			ix := int(srcX)
			iy := int(srcY)

			if ix >= oldW-1 {
				ix = oldW - 2
			}
			if iy >= oldH-1 {
				iy = oldH - 2
			}

			xf := srcX - float64(ix)
			yf := srcY - float64(iy)

			// 4个邻近像素
			r00, g00, b00, a00 := img.At(ix, iy).RGBA()
			r10, g10, b10, a10 := img.At(ix+1, iy).RGBA()
			r01, g01, b01, a01 := img.At(ix, iy+1).RGBA()
			r11, g11, b11, a11 := img.At(ix+1, iy+1).RGBA()

			// 双线性插值
			r := bilinear(r00, r10, r01, r11, xf, yf) >> 8
			g := bilinear(g00, g10, g01, g11, xf, yf) >> 8
			b := bilinear(b00, b10, b01, b11, xf, yf) >> 8
			a := bilinear(a00, a10, a01, a11, xf, yf) >> 8

			out.Set(x, y, &colorNRGBA{uint8(r), uint8(g), uint8(b), uint8(a)})
		}
	}

	return out
}

func bilinear(c00, c10, c01, c11 uint32, xf, yf float64) uint32 {
	c0 := float64(c00)*(1-xf) + float64(c10)*xf
	c1 := float64(c01)*(1-xf) + float64(c11)*xf
	return uint32(c0*(1-yf) + c1*yf)
}

// colorNRGBA 简单的 NRGBA 实现
type colorNRGBA struct {
	R, G, B, A uint8
}

func (c *colorNRGBA) RGBA() (uint32, uint32, uint32, uint32) {
	r := uint32(c.R) * 0x101
	g := uint32(c.G) * 0x101
	b := uint32(c.B) * 0x101
	a := uint32(c.A) * 0x101
	return r, g, b, a
}