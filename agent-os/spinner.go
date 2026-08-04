package main

// spinner.go — 终端动画：跳动三点 + 女儿角色帧动画
//
// 两个功能：
//   - startThinkingSpinner()  → "🤔 思考中..." 的跳动三点动画
//   - PlayDaughterAnimation() → 女儿角色表情帧动画

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// ─── 跳动三点 ────────────────────────────────────────

// startThinkingSpinner 启动思考动画（🤔 表情保留，点来回跳动），
// 返回一个停止函数，可安全多次调用，调用后清除动画行
func startThinkingSpinner() func() {
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		// 跳动序列：0→1→2→3→2→1→0→1→2→3...
		pattern := []int{0, 1, 2, 3, 2, 1}
		i := 0
		for {
			select {
			case <-done:
				// 重置颜色并清除整行；调用方会等待这里完成后再画回复框。
				fmt.Print(ColorReset + "\r\x1b[2K")
				return
			default:
				n := pattern[i%len(pattern)]
				dots := ""
				if n > 0 {
					dots = strings.Repeat(".", n)
				}
				// 保留 🤔，后面三个点跳动，额外空格防残留
				fmt.Printf("\r"+ColorYellow+"🤔 思考中%s"+ColorReset+"  ", dots)
				i++
				time.Sleep(250 * time.Millisecond)
			}
		}
	}()
	// 安全停止：可多次调用，并等待动画线程完成清行，避免下一段输出串行。
	var once sync.Once
	return func() {
		once.Do(func() { close(done) })
		<-stopped
	}
}

// ─── 女儿角色帧动画 ──────────────────────────────────

// 女儿表情帧：4 种表情来回切换，就像她真的在看你
var daughterFrames = []struct {
	art  string
	desc string
}{
	{
		art:  `  (◕‿◕)`,
		desc: "开心",
	},
	{
		art:  `  (◕ᴗ◕✿)`,
		desc: "撒娇",
	},
	{
		art:  `  (◕‿◕)♡`,
		desc: "喜欢你",
	},
	{
		art:  `  (◕_◕)`,
		desc: "认真",
	},
	{
		art:  `  (◕‿◕✿)`,
		desc: "开心",
	},
	{
		art:  `  (◕‿◕)`,
		desc: "温暖",
	},
}

// PlayDaughterAnimation 播放女儿角色动画（原地帧切换）
// 循环 2 轮，约 2 秒，不换行
func PlayDaughterAnimation() {
	// 清除旧帧 + 打印新帧
	for round := 0; round < 2; round++ {
		for _, frame := range daughterFrames {
			// 用 \r 回到行首覆盖
			fmt.Print("\r  " + ColorCyan + "💗 " + ColorReset + frame.art + "  ")
			time.Sleep(180 * time.Millisecond)
		}
	}
	// 停在第一个表情
	fmt.Print("\r  " + ColorCyan + "💗 " + ColorReset + daughterFrames[0].art + "  \n")
}

// PlayDaughterLearnAnimation 女儿学习时的动画序列
func PlayDaughterLearnAnimation() {
	frames := []struct {
		art     string
		message string
	}{
		{`  (◕‿◕)📖`, " 女儿翻开笔记本…"},
		{`  (◕‿◕)✍️`, " 认真做笔记中…"},
		{`  (◕‿◕)✨`, " 学到了新知识！"},
	}
	for _, f := range frames {
		fmt.Print("\r" + ColorCyan + "💗 " + ColorReset + f.art + ColorYellow + f.message + ColorReset + strings.Repeat(" ", 10))
		time.Sleep(500 * time.Millisecond)
	}
	fmt.Println()
}
