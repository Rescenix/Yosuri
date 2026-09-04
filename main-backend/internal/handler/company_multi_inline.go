package handler

// company_multi_inline.go —— 把多文件工程内联成单个 output-app.html。
// 多文件项目落 4 个真实源文件供打包下载，同时生成一份内联版：
// 预览 iframe、门禁校验、审批台展示都读这份内联版，无需起本地服务器。

import "strings"

// companyInlineMulti 把 index.html 里的 <link>/<script src> 替换成内联 <style>/<script>。
func companyInlineMulti(files companyMultiFiles) string {
	html := files["index.html"]
	if css := files["styles.css"]; css != "" {
		html = strings.Replace(html, `<link rel="stylesheet" href="styles.css">`,
			"<style>\n"+css+"\n</style>", 1)
	}
	if data := files["data.js"]; data != "" {
		html = strings.Replace(html, `<script src="data.js"></script>`,
			"<script>\n"+data+"\n</script>", 1)
	}
	if app := files["app.js"]; app != "" {
		html = strings.Replace(html, `<script src="app.js"></script>`,
			"<script>\n"+app+"\n</script>", 1)
	}
	return html
}
