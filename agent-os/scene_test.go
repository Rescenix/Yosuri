package main

import (
	"strings"
	"testing"
)

func TestSceneColumnsUseThreeQuarterSplitAndReserveLastCell(t *testing.T) {
	left, right := sceneColWidthsFor(120)
	if left != 87 || right != 29 {
		t.Fatalf("120 列内容区应拆为 87+29, got %d+%d", left, right)
	}
	if left+right+3 != 119 {
		t.Fatalf("共享边框画布宽度 = %d, want 119", left+right+3)
	}
}

func TestLiveSceneHasFixedHeightAndWidth(t *testing.T) {
	f := sceneFrame{
		RegionName: "翠风镇", RegionIcon: "🏘️", RegionKind: "小镇",
		Mood: "(◕‿◕✿)✨", Action: "📚 在翠风镇学习",
		Ability:  "编程生疏·写作生涩·研究有方法·设计有审美·社交受欢迎",
		LogLines: []string{"[13:02] 🧭 第 1 轮 · 决定向南走（今天想去看看）", "☁️ 云端状态已恢复（跨设备）"},
	}
	lines := renderLiveLinesAtWidth(f, 120)
	if len(lines) != liveSceneRows {
		t.Fatalf("直播高度 = %d, want %d", len(lines), liveSceneRows)
	}
	for i, line := range lines {
		if strings.Contains(line, "\n") || strings.Contains(line, "\r") {
			t.Fatalf("第 %d 行包含换行符", i)
		}
		if got := terminalTextWidth(line); got != 119 {
			t.Fatalf("第 %d 行宽度 = %d, want 119: %q", i, got, line)
		}
	}
}

func TestLiveSceneUsesOneContinuousSharedBorder(t *testing.T) {
	f := sceneFrame{RegionName: "镜渊", RegionIcon: "🌌", RegionKind: "秘境"}
	lines := renderLiveLinesAtWidth(f, 100)
	if strings.Contains(lines[0], "╮"+ColorReset+ColorCyan+"╭") {
		t.Fatal("左右栏不应使用割裂的两个外框")
	}
	if !strings.Contains(lines[0], "┬") || !strings.Contains(lines[len(lines)-1], "┴") {
		t.Fatal("共享边框应使用 ┬ 和 ┴ 连接左右栏")
	}
	if count := strings.Count(lines[0], "镜渊"); count != 1 {
		t.Fatalf("顶部区域名镜渊应只显示一次, got %d", count)
	}
}

func TestTerminalTruncationHandlesANSIChineseAndEmoji(t *testing.T) {
	text := ColorCyan + "🏘️ 翠风镇 · 今天继续学习和探索" + ColorReset
	got := fitTerminalText(text, 12, true)
	if width := terminalTextWidth(got); width != 12 {
		t.Fatalf("裁剪后宽度 = %d, want 12: %q", width, got)
	}
}

func TestEmojiClustersUseWindowsTerminalWidth(t *testing.T) {
	cases := []string{"🏔️", "☁️", "✅", "❤️", "✨", "👩‍💻", "🇨🇳", "1️⃣"}
	for _, text := range cases {
		if got := terminalTextWidth(text); got != 2 {
			t.Errorf("%q 宽度 = %d, want 2", text, got)
		}
	}
}
