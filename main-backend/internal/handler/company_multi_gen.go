package handler

// company_multi_gen.go —— 多文件项目生成器：模型分文件产出，落盘成真实工程结构。

import (
	"fmt"
	"strings"
)

// companyMultiFiles 一次多文件交付的全部产物（文件名 → 内容）。
type companyMultiFiles map[string]string

// deliveryMultiProject 生成多文件项目：index.html + app.js + styles.css + data.js。
// 每个文件单独让模型产出（单文件塞不下完整工程，分开写才不被 token 上限截断）。
// upstream 为最小原型/调研等上游产物；任何文件模型失败都有兜底模板，门禁不挂死。
func deliveryMultiProject(project, brief, upstream string) companyMultiFiles {
	files := companyMultiFiles{}

	// 1) 数据层 data.js —— 确定性生成，不调模型。
	// localStorage CRUD 是机械代码，模型产出不比模板好，反而每次接口都漂移
	// （实测：data.js 出 list()、app.js 调 getAll() → 页面直接崩）。
	// 接口恒定，app.js 的 prompt 才能钉死契约。
	files["data.js"] = companyDataJSTemplate(project)

	// 2) 样式 styles.css —— 响应式 + 明暗适配 + 设计规格（亮蓝白以外的自定和谐配色）
	css, err := deliveryLLMContent("设计师", brief, "样式",
		"写该产品的 styles.css，设计规格（硬要求）：①:root 定义 CSS 变量（主色+辅色+中性灰阶 3 档以上），配色和谐有品牌感，禁止默认浏览器蓝紫 #3B82F6/#6366f1，禁止纯黑纯白大面积对撞；②卡片圆角 12-16px、多层细腻阴影、hover 有 translateY 过渡 ≥.18s；③中文排版 font-family 带 PingFang SC/Microsoft YaHei、行高≥1.6、数字 tabular-nums；④响应式 @media 至少 640px/960px 两档断点；⑤表单控件有 focus 态样式；⑥主容器用 min-height:100vh 的 flex 布局，统计卡片区/说明区撑满视口，禁止下半页塌陷成空白；⑦合计/统计区用带背景色的卡片（不是裸文本），正负值有绿/红色区分。只输出 CSS 代码本体，不要 markdown 围栏",
		upstream)
	if err != nil || !strings.Contains(css, "@media") {
		css = companyCSSFallback()
	}
	files["styles.css"] = deliveryStripJSFence(css)

	// 3) 逻辑 app.js —— 页面结构 + 交互全由它渲染（DOM 所有权归 app.js，
	//    index.html 只给 #app 挂载点；模型各文件独立生成，不约定清楚必崩）。
	appJS, err := deliveryLLMContent("前端工程师", brief, "应用逻辑",
		"数据接口契约（必须严格照此调用，data.js 已固定实现）：window.AppData.list() 返回对象数组；window.AppData.add(obj) 新增并返回带 id 的对象，obj 字段自定但必须一次传全；window.AppData.update(id,patch) 局部更新；window.AppData.remove(id) 删除。页面上所有 DOM（表单、列表容器、合计区、空状态）由 app.js 自己创建并挂载到 document.getElementById('app') 下，严禁假设 index.html 里已有其它元素。交互质感（硬要求）：①操作后列表有可见反馈（新条目高亮淡入、删除有过渡）；②空状态要精心设计（图标+引导文案+主操作按钮，不是一行灰字）；③统计数字变化用 CSS 过渡而非瞬间跳变；④列表为空/加载中/出错三种状态都要处理；⑤收支/数值类产品：合计区做成统计卡片（收入/支出/净额三卡），净额为负标红；⑥页面底部要有内容（统计卡/使用说明/分类汇总），禁止下半页空白。只输出 JS 代码本体，不要 markdown 围栏",
		upstream)
	if err != nil || !strings.Contains(appJS, "AppData") || !strings.Contains(appJS, "getElementById('app')") && !strings.Contains(appJS, `getElementById("app")`) {
		appJS = companyAppJSTemplate(project)
	}
	files["app.js"] = deliveryStripJSFence(appJS)

	// 4) 页面 index.html —— 骨架 + 引用三个文件（确定性拼装，不让模型自由发挥引用路径）
	files["index.html"] = fmt.Sprintf(`<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>%s</title>
<link rel="stylesheet" href="styles.css">
</head>
<body>
<main class="shell">
<header class="topbar"><h1>%s</h1><p class="sub">%s</p></header>
<section id="app" aria-live="polite"></section>
</main>
<script src="data.js"></script>
<script src="app.js"></script>
</body>
</html>`, deliveryXML(project), deliveryXML(project), deliveryXML(deliveryTruncate(brief, 80)))

	return files
}
