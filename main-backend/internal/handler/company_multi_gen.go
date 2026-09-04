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

	// 2) 样式 styles.css —— 响应式 + 明暗适配
	css, err := deliveryLLMContent("设计师", brief, "样式",
		"写该产品的 styles.css：亮色为主、响应式（含 @media）、卡片式布局、按钮与表单样式。只输出 CSS 代码本体，不要 markdown 围栏",
		upstream)
	if err != nil || !strings.Contains(css, "@media") {
		css = companyCSSFallback()
	}
	files["styles.css"] = deliveryStripJSFence(css)

	// 3) 逻辑 app.js —— 页面结构 + 交互全由它渲染（DOM 所有权归 app.js，
	//    index.html 只给 #app 挂载点；模型各文件独立生成，不约定清楚必崩）。
	appJS, err := deliveryLLMContent("前端工程师", brief, "应用逻辑",
		"数据接口契约（必须严格照此调用，data.js 已固定实现）：window.AppData.list() 返回对象数组；window.AppData.add(obj) 新增并返回带 id 的对象，obj 字段自定但必须一次传全；window.AppData.update(id,patch) 局部更新；window.AppData.remove(id) 删除。页面上所有 DOM（表单、列表容器、合计区、空状态）由 app.js 自己创建并挂载到 document.getElementById('app') 下，严禁假设 index.html 里已有其它元素。只输出 JS 代码本体，不要 markdown 围栏",
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
