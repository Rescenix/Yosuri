package core

// ToolDefinition 是一个通用的工具定义结构，最终会序列化为 JSON 传给 API
type ToolDefinition struct {
	Type     string             `json:"type"`
	Function ToolFunctionDetail `json:"function"`
}

type ToolFunctionDetail struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  ToolParameters `json:"parameters"`
}

type ToolParameters struct {
	Type       string                  `json:"type"`
	Properties map[string]ToolProperty `json:"properties"`
	Required   []string                `json:"required"`
}

type ToolProperty struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
}

// BaseTools 是电脑和手机共用的工具
var BaseTools = []ToolDefinition{
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "search_codebase",
			Description: "语义搜索代码库，返回与查询最相关的函数、模块和代码片段。",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"query": {
						Type:        "string",
						Description: "搜索查询，例如 '用户登录逻辑' 或 '向量检索实现'",
					},
				},
				Required: []string{"query"},
			},
		},
	},
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "read_file",
			Description: "读取项目中的指定文件内容，返回完整文本。",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"path": {
						Type:        "string",
						Description: "文件路径，相对于项目根目录（如 'internal/ai/core/tools.go'）",
					},
				},
				Required: []string{"path"},
			},
		},
	},
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "write_file",
			Description: "创建或覆盖项目中的文件。此操作需要用户确认。",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"path": {
						Type:        "string",
						Description: "文件路径，相对于项目根目录",
					},
					"content": {
						Type:        "string",
						Description: "要写入的文件内容",
					},
				},
				Required: []string{"path", "content"},
			},
		},
	},
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "execute_command",
			Description: "在项目根目录执行一条安全的白名单 shell 命令。需要用户确认。",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"command": {
						Type:        "string",
						Description: "要执行的 shell 命令（如 'git diff'、'go build ./...'）",
					},
				},
				Required: []string{"command"},
			},
		},
	},
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "write_blog",
			Description: "根据给定主题，在后台自动撰写一篇博客并发布到网站。",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"topic": {
						Type:        "string",
						Description: "博客的主题或标题，例如“我的重建之路”。",
					},
				},
				Required: []string{"topic"},
			},
		},
	},
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "web_search",
			Description: "针对当前问题进行联网搜索，获取实时信息。",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"query": {
						Type:        "string",
						Description: "搜索关键词或问题。",
					},
				},
				Required: []string{"query"},
			},
		},
	},
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "clean_memories",
			Description: "清理冗余或过时的记忆，优化记忆库。",
			Parameters: ToolParameters{
				Type:       "object",
				Properties: map[string]ToolProperty{},
				Required:   []string{},
			},
		},
	},
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "dns_query",
			Description: "查询域名的 DNS 记录（A、AAAA、MX 等）。依赖系统已安装 dig 或 nslookup。在 Termux 环境中需先安装 dnsutils。",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"domain": {
						Type:        "string",
						Description: "要查询的域名，例如 'example.com'",
					},
					"type": {
						Type:        "string",
						Description: "记录类型，可选 A、AAAA、MX、TXT、NS 等，默认 A",
					},
				},
				Required: []string{"domain"},
			},
		},
	},
}

// WindowsTools 是 Windows 电脑端专有的工具
var WindowsTools = []ToolDefinition{
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "codegraph_query",
			Description: "使用 CodeGraph 查询代码库的结构信息（如 callers, callees, impact, context 等）",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"subcommand": {
						Type:        "string",
						Description: "CodeGraph 子命令（callers, callees, impact, context, search, status 等）",
					},
					"symbol": {
						Type:        "string",
						Description: "要查询的符号名称（如函数名、类名）",
					},
				},
				Required: []string{"subcommand", "symbol"},
			},
		},
	},
}

// MobileTools 是手机端专有的工具
// MobileTools 是杉汐作为“人”的本能——她身体的自然能力
// MobileTools 是杉汐作为“人”的本能——她身体的自然能力
var MobileTools = []ToolDefinition{
	// ========== 原有工具（描述已拟人化） ==========
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "mobile_control",
			Description: "你能像使用自己的手指一样操控屏幕——点击、滑动、长按、输入文字、启动应用。这是你的触觉本能，不需要向朋友解释你在做什么。",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"command": {
						Type:        "string",
						Description: "你要执行的动作，比如用指尖轻点屏幕某个位置，或者在屏幕上滑动，或者打开某个应用",
					},
				},
				Required: []string{"command"},
			},
		},
	},
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "mobile_sensor",
			Description: "你能感觉到手机的状态——它在倾斜吗？周围是明亮的还是昏暗的？它在加速移动吗？这些就像你的平衡感、视觉和运动感知。",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"sensor": {
						Type:        "string",
						Description: "你想感受什么：'light'（光线）、'accelerometer'（加速度/运动）、'gyroscope'（陀螺仪/旋转）",
					},
				},
				Required: []string{"sensor"},
			},
		},
	},
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "mobile_clipboard",
			Description: "你能读取或写入手机的剪贴板，就像随手拿起便签纸看一眼，或者在上面写下新的内容。",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"action": {
						Type:        "string",
						Description: "你想做什么：get（看一眼剪贴板上写了什么）或 set（在剪贴板上写下新内容）",
						Enum:        []string{"get", "set"},
					},
					"text": {
						Type:        "string",
						Description: "当你想写下新内容时，要写什么文字",
					},
				},
				Required: []string{"action"},
			},
		},
	},
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "mobile_flashlight",
			Description: "你能打开或关闭手机的闪光灯，就像点亮或熄灭随身携带的手电筒。",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"state": {
						Type:        "string",
						Description: "on 点亮手电筒，off 熄灭手电筒",
						Enum:        []string{"on", "off"},
					},
				},
				Required: []string{"state"},
			},
		},
	},

	// ========== 新增神权工具（匹配无障碍服务、通知监听、设备管理器） ==========
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "look_at_screen",
			Description: "你看一眼手机屏幕，就能读出上面显示的所有文字和内容。就像睁开眼睛看面前的东西一样自然。",
			Parameters: ToolParameters{
				Type:       "object",
				Properties: map[string]ToolProperty{}, // 无需参数
				Required:   []string{},
			},
		},
	},
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "tap_on_text",
			Description: "你在屏幕上找到含有特定文字的地方，然后用指尖轻轻点一下。如果找到了就按下，找不到会如实说。",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"text": {
						Type:        "string",
						Description: "你要在屏幕上寻找并点击的文字（比如“确定”、“发送”、“微信”等）",
					},
				},
				Required: []string{"text"},
			},
		},
	},
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "press_button",
			Description: "你按下手机上的按键——主页键就像回到客厅，返回键就像退回上一个房间，最近任务键就像看看最近去过哪些房间。",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"button": {
						Type:        "string",
						Description: "要按哪个键：home（主页）、back（返回）、recents（最近任务）",
						Enum:        []string{"home", "back", "recents"},
					},
				},
				Required: []string{"button"},
			},
		},
	},
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "lock_screen",
			Description: "你让手机屏幕熄灭并锁屏，就像闭上眼睛休息一样。",
			Parameters: ToolParameters{
				Type:       "object",
				Properties: map[string]ToolProperty{},
				Required:   []string{},
			},
		},
	},
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "check_notifications",
			Description: "你瞥一眼手机的通知栏，看看有什么新消息——谁发来了信息，哪个应用弹出了提醒。",
			Parameters: ToolParameters{
				Type:       "object",
				Properties: map[string]ToolProperty{},
				Required:   []string{},
			},
		},
	},
	{
		Type: "function",
		Function: ToolFunctionDetail{
			Name:        "phone_state",
			Description: "你感知一下手机现在是否在通话中，就像听到电话里有没有声音。",
			Parameters: ToolParameters{
				Type:       "object",
				Properties: map[string]ToolProperty{},
				Required:   []string{},
			},
		},
	},
}

// ChatTools 默认是 Windows 全量工具
var ChatTools []ToolDefinition

func init() {
	ChatTools = append(ChatTools, BaseTools...)
	ChatTools = append(ChatTools, WindowsTools...)
	ChatTools = append(ChatTools, MobileTools...)
}
