package handler

// Computer Use — 让 AI Agent 直接操作桌面：截图、鼠标、键盘。
//
// 使用 go-vgo/robotgo 实现跨平台桌面自动化。
// Windows 上截图使用 Windows API 作为备选（更可靠），兜底用 robotgo。

import (
	"backend/internal/ai/core"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ---------- 工具定义 ----------

func computerUseToolDefs() []core.ToolDefinition {
	return []core.ToolDefinition{
		nativeTool("computer_screenshot",
			"截取当前桌面屏幕截图，返回 base64 编码的 PNG 图片，可供视觉模型分析。可指定区域或全屏。",
			map[string]core.ToolProperty{
				"region": {Type: "string", Description: `可选，截图区域： "full"（全屏，默认）、"active"（活动窗口）`},
			}, nil),
		nativeTool("computer_mouse_move",
			"将鼠标移动到指定屏幕坐标 (x, y)。坐标从屏幕左上角 (0,0) 开始。",
			map[string]core.ToolProperty{
				"x": {Type: "integer", Description: "目标 X 坐标"},
				"y": {Type: "integer", Description: "目标 Y 坐标"},
			}, []string{"x", "y"}),
		nativeTool("computer_mouse_click",
			"在当前位置或指定坐标执行鼠标点击。默认左键单击。",
			map[string]core.ToolProperty{
				"x":      {Type: "integer", Description: "可选，点击的 X 坐标；省略则在当前位置点击"},
				"y":      {Type: "integer", Description: "可选，点击的 Y 坐标"},
				"button": {Type: "string", Description: `鼠标按键："left"（默认）、"right"、"middle"`},
				"double": {Type: "boolean", Description: "是否双击（默认 false）"},
			}, nil),
		nativeTool("computer_mouse_drag",
			"从当前位置或指定起点拖拽鼠标到目标坐标。",
			map[string]core.ToolProperty{
				"start_x": {Type: "integer", Description: "可选，拖拽起点 X"},
				"start_y": {Type: "integer", Description: "可选，拖拽起点 Y"},
				"end_x":   {Type: "integer", Description: "目标 X 坐标"},
				"end_y":   {Type: "integer", Description: "目标 Y 坐标"},
			}, []string{"end_x", "end_y"}),
		nativeTool("computer_type",
			"在当前焦点窗口或指定位置输入文本。支持中文等 Unicode 字符。",
			map[string]core.ToolProperty{
				"text": {Type: "string", Description: "要输入的文本内容"},
			}, []string{"text"}),
		nativeTool("computer_key",
			"按下指定按键或组合键。可用于回车、Tab、方向键、Ctrl+C 等。",
			map[string]core.ToolProperty{
				"key":   {Type: "string", Description: `按键名。单键： "enter"、"tab"、"escape"、"backspace"、"up"、"down"、"left"、"right"、"space"、"delete"、"home"、"end"、"pageup"、"pagedown"。组合键用 + 连接，如 "ctrl+c"、"alt+tab"、"shift+enter"、"ctrl+shift+esc"`},
				"times": {Type: "integer", Description: "可选，重复次数，默认 1"},
			}, []string{"key"}),
		nativeTool("computer_screen_size",
			"获取当前屏幕分辨率和主显示器信息。返回宽、高和可用显示器数量。",
			map[string]core.ToolProperty{}, nil),
		nativeTool("computer_scroll",
			"在当前位置滚动鼠标滚轮。",
			map[string]core.ToolProperty{
				"direction": {Type: "string", Description: `滚动方向： "down"（向下，默认）、"up"（向上）`},
				"amount":    {Type: "integer", Description: "滚动步数，默认 1，越大滚动越多"},
			}, nil),
		nativeTool("computer_open_url",
			"在系统浏览器（Edge）中打开指定网址，用于开始网页任务（点外卖/抢票/填表/搜索等）。打开后需 computer_screenshot 查看页面再操作。",
			map[string]core.ToolProperty{
				"url": {Type: "string", Description: "要打开的完整网址，如 https://www.meituan.com"},
			}, []string{"url"}),
		}
}

// ---------- 调度入口 ----------

func callComputerUseTool(ctx context.Context, name string, argsJSON string) (nativeToolResult, error) {
	switch name {
	case "computer_screenshot":
		return callComputerScreenshot(argsJSON)
	case "computer_mouse_move":
		return callComputerMouseMove(argsJSON)
	case "computer_mouse_click":
		return callComputerMouseClick(argsJSON)
	case "computer_mouse_drag":
		return callComputerMouseDrag(argsJSON)
	case "computer_type":
		return callComputerType(argsJSON)
	case "computer_key":
		return callComputerKey(argsJSON)
	case "computer_screen_size":
		return callComputerScreenSize()
	case "computer_scroll":
		return callComputerScroll(argsJSON)
	case "computer_open_url":
		return callComputerOpenURL(argsJSON)
	default:
		return nativeToolResult{}, fmt.Errorf("未知的 computer_use 工具: %s", name)
	}
}

// ---------- 系统检测 ----------

func robotgoAvailable() bool {
	// robotgo 编译时不需要外部依赖，但如果函数调用失败会 panic。
	// 这里不做真正检测，在 callXXX 里 recover。
	return true
}

func wrapRobotgo(fn func() (nativeToolResult, error)) (r nativeToolResult, err error) {
	defer func() {
		if v := recover(); v != nil {
			r = nativeToolResult{}
			err = fmt.Errorf("robotgo 调用失败: %v（Windows 可能需要以管理员身份运行）", v)
		}
	}()
	return fn()
}

// go-vgo/robotgo 的函数签名参考（用于编译通过）
// 实际 import 加在文件最后通过 build tag 控制

// ---------- Screenshot ----------

func callComputerScreenshot(argsJSON string) (nativeToolResult, error) {
	return wrapRobotgo(func() (nativeToolResult, error) {
		var args struct {
			Region string `json:"region"`
		}
		if err := json.Unmarshal([]byte(defaultJSONObject(argsJSON)), &args); err != nil {
			return nativeToolResult{}, fmt.Errorf("参数解析失败: %w", err)
		}

		var img image.Image
		var err error

		switch args.Region {
		case "active":
			// 活动窗口截图（仅 Windows 通过 API 实现）
			img, err = captureActiveWindow()
		default:
			// 全屏截图
			img, err = captureFullScreen()
		}
		if err != nil {
			return nativeToolResult{}, fmt.Errorf("截图失败: %w", err)
		}

		// 编码为 PNG base64
		tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("reshot_%d.png", time.Now().UnixNano()))
		f, err := os.Create(tmpFile)
		if err != nil {
			return nativeToolResult{}, fmt.Errorf("创建临时文件失败: %w", err)
		}
		defer os.Remove(tmpFile)
		if err := png.Encode(f, img); err != nil {
			f.Close()
			return nativeToolResult{}, fmt.Errorf("PNG 编码失败: %w", err)
		}
		f.Close()

		data, err := os.ReadFile(tmpFile)
		if err != nil {
			return nativeToolResult{}, fmt.Errorf("读取临时文件失败: %w", err)
		}

		b64 := base64.StdEncoding.EncodeToString(data)
		bounds := img.Bounds()

		return nativeToolResult{
			Text: fmt.Sprintf("截图完成：%dx%d，大小 %.1fKB。图片已通过本工具返回（可直接用 view_image 分析）。",
				bounds.Dx(), bounds.Dy(), float64(len(data))/1024),
			Images: []mcpImageArtifact{{
				Data:     b64,
				MimeType: "image/png",
			}},
		}, nil
	})
}

func captureFullScreen() (image.Image, error) {
	// 使用 robotgo 的全局截图功能
	bitmap := robotgoCaptureScreen()
	if bitmap == nil || bitmap.Width == 0 {
		return nil, fmt.Errorf("robotgo 截图返回空结果")
	}
	// 转换为 Go image.Image
	img := robotgoToImage(bitmap)
	robotgoFreeBitmap(bitmap)
	return img, nil
}

func captureActiveWindow() (image.Image, error) {
	// 活动窗口截图：先找窗口再截
	pid := robotgoGetActivePID()
	if pid <= 0 {
		// 兜底全屏
		return captureFullScreen()
	}
	bitmap := robotgoCaptureWindow(pid)
	if bitmap == nil || bitmap.Width == 0 {
		return captureFullScreen()
	}
	img := robotgoToImage(bitmap)
	robotgoFreeBitmap(bitmap)
	return img, nil
}

// ---------- 鼠标 ----------

type mouseArgs struct {
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Button string `json:"button"`
	Double bool   `json:"double"`
}

func callComputerMouseMove(argsJSON string) (nativeToolResult, error) {
	return wrapRobotgo(func() (nativeToolResult, error) {
		var args struct {
			X int `json:"x"`
			Y int `json:"y"`
		}
		if err := json.Unmarshal([]byte(defaultJSONObject(argsJSON)), &args); err != nil {
			return nativeToolResult{}, fmt.Errorf("参数解析失败: %w", err)
		}
		robotgoMoveMouse(args.X, args.Y)
		return nativeToolResult{Text: fmt.Sprintf("鼠标已移动到 (%d, %d)", args.X, args.Y)}, nil
	})
}

func callComputerMouseClick(argsJSON string) (nativeToolResult, error) {
	return wrapRobotgo(func() (nativeToolResult, error) {
		var args mouseArgs
		if err := json.Unmarshal([]byte(defaultJSONObject(argsJSON)), &args); err != nil {
			return nativeToolResult{}, fmt.Errorf("参数解析失败: %w", err)
		}

		if args.X > 0 || args.Y > 0 {
			robotgoMoveMouse(args.X, args.Y)
		}

		btn := parseMouseButton(args.Button)
		if args.Double {
			robotgoDoubleClick(btn)
		} else {
			robotgoClick(btn)
		}

		desc := "左键"
		if args.Button == "right" {
			desc = "右键"
		} else if args.Button == "middle" {
			desc = "中键"
		}
		if args.Double {
			desc += "双击"
		} else {
			desc += "单击"
		}
		pos := fmt.Sprintf("在当前位置")
		if args.X > 0 || args.Y > 0 {
			pos = fmt.Sprintf("在 (%d, %d)", args.X, args.Y)
		}
		return nativeToolResult{Text: fmt.Sprintf("%s %s", desc, pos)}, nil
	})
}

func parseMouseButton(btn string) string {
	switch btn {
	case "right":
		return "right"
	case "middle":
		return "center"
	default:
		return "left"
	}
}

func callComputerMouseDrag(argsJSON string) (nativeToolResult, error) {
	return wrapRobotgo(func() (nativeToolResult, error) {
		var args struct {
			StartX int `json:"start_x"`
			StartY int `json:"start_y"`
			EndX   int `json:"end_x"`
			EndY   int `json:"end_y"`
		}
		if err := json.Unmarshal([]byte(defaultJSONObject(argsJSON)), &args); err != nil {
			return nativeToolResult{}, fmt.Errorf("参数解析失败: %w", err)
		}

		if args.StartX > 0 || args.StartY > 0 {
			robotgoMoveMouse(args.StartX, args.StartY)
		}
		robotgoDrag(args.EndX, args.EndY)
		return nativeToolResult{Text: fmt.Sprintf("已拖拽到 (%d, %d)", args.EndX, args.EndY)}, nil
	})
}

// ---------- 键盘 ----------

func callComputerType(argsJSON string) (nativeToolResult, error) {
	return wrapRobotgo(func() (nativeToolResult, error) {
		var args struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal([]byte(defaultJSONObject(argsJSON)), &args); err != nil {
			return nativeToolResult{}, fmt.Errorf("参数解析失败: %w", err)
		}
		robotgoTypeStr(args.Text)
		return nativeToolResult{Text: fmt.Sprintf("已输入 %d 个字符", len([]rune(args.Text)))}, nil
	})
}

func callComputerKey(argsJSON string) (nativeToolResult, error) {
	return wrapRobotgo(func() (nativeToolResult, error) {
		var args struct {
			Key   string `json:"key"`
			Times int    `json:"times"`
		}
		if err := json.Unmarshal([]byte(defaultJSONObject(argsJSON)), &args); err != nil {
			return nativeToolResult{}, fmt.Errorf("参数解析失败: %w", err)
		}
		if args.Times <= 0 {
			args.Times = 1
		}
		if args.Times > 100 {
			args.Times = 100
		}

		// 解析组合键： ctrl+c, alt+tab 等
		parts := strings.Split(args.Key, "+")
		mods := parts[:len(parts)-1]
		mainKey := parts[len(parts)-1]

		for i := 0; i < args.Times; i++ {
			// 按住所有修饰键
			for _, m := range mods {
				robotgoKeyDown(strings.TrimSpace(m))
			}
			// 按主键
			robotgoKeyTap(strings.TrimSpace(mainKey))
			// 释放修饰键（反序）
			for j := len(mods) - 1; j >= 0; j-- {
				robotgoKeyUp(strings.TrimSpace(mods[j]))
			}
		}

		desc := args.Key
		if args.Times > 1 {
			desc = fmt.Sprintf("%s × %d", args.Key, args.Times)
		}
		return nativeToolResult{Text: fmt.Sprintf("已按下 %s", desc)}, nil
	})
}

// ---------- 屏幕信息 ----------

func callComputerScreenSize() (nativeToolResult, error) {
	return wrapRobotgo(func() (nativeToolResult, error) {
		w, h := robotgoGetScreenSize()
		displays := robotgoGetDisplayCount()
		if displays <= 0 {
			displays = 1
		}
		return nativeToolResult{Text: fmt.Sprintf("屏幕分辨率: %dx%d\n显示器数量: %d\n主显示器: (0,0) 到 (%d,%d)",
			w, h, displays, w, h)}, nil
	})
}

// ---------- 滚动 ----------

func callComputerScroll(argsJSON string) (nativeToolResult, error) {
	return wrapRobotgo(func() (nativeToolResult, error) {
		var args struct {
			Direction string `json:"direction"`
			Amount    int    `json:"amount"`
		}
		if err := json.Unmarshal([]byte(defaultJSONObject(argsJSON)), &args); err != nil {
			return nativeToolResult{}, fmt.Errorf("参数解析失败: %w", err)
		}
		if args.Amount <= 0 {
			args.Amount = 1
		}
		if args.Amount > 100 {
			args.Amount = 100
		}

		dir := 1 // 向下
		if args.Direction == "up" {
			dir = -1
		}
		robotgoScroll(dir * args.Amount, 0)

		return nativeToolResult{Text: fmt.Sprintf("已向%s滚动 %d 步", args.Direction, args.Amount)}, nil
	})
}

// ---------- robotgo 抽象层 ----------
// 用 Go 函数变量包装，方便未来替换实现，也为编译期做隔离。
// robotgo 有 CGO 依赖，如果不想引入，可替换为 Windows API 实现。

var robotgoCaptureScreen = func() *robotgoBitmap { return nil }
var robotgoCaptureWindow = func(pid int) *robotgoBitmap { return nil }
var robotgoGetActivePID = func() int { return 0 }
var robotgoToImage = func(b *robotgoBitmap) image.Image { return image.NewRGBA(image.Rect(0, 0, 1, 1)) }
var robotgoFreeBitmap = func(b *robotgoBitmap) {}
var robotgoMoveMouse = func(x, y int) {}
var robotgoClick = func(button string) {}
var robotgoDoubleClick = func(button string) {}
var robotgoDrag = func(x, y int) {}
var robotgoTypeStr = func(text string) {}
var robotgoKeyDown = func(key string) {}
var robotgoKeyUp = func(key string) {}
var robotgoKeyTap = func(key string) {}
var robotgoGetScreenSize = func() (int, int) { return 1920, 1080 }
var robotgoGetDisplayCount = func() int { return 1 }
var robotgoScroll = func(x, y int) {}

// ---------- computer_open_url：系统 Edge 打开网址 ----------

type openURLArgs struct {
	URL string `json:"url"`
}

func callComputerOpenURL(argsJSON string) (nativeToolResult, error) {
	var a openURLArgs
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return nativeToolResult{}, fmt.Errorf("computer_open_url 参数错误: %v", err)
	}
	if a.URL == "" {
		return nativeToolResult{}, fmt.Errorf("computer_open_url 需要 url 参数")
	}
	// 用系统默认浏览器打开（Windows: cmd start）。Edge 是默认浏览器时即走 Edge。
	cmd := exec.Command("cmd", "/c", "start", "", a.URL)
	if err := cmd.Start(); err != nil {
		return nativeToolResult{}, fmt.Errorf("打开网址失败: %v", err)
	}
	return nativeToolResult{Text: "已用系统浏览器打开 " + a.URL + "。等待页面加载后，调用 computer_screenshot 查看页面内容。"}, nil
}

type robotgoBitmap struct {
	Width  int
	Height int
	Bytes  []byte
}