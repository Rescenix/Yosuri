//go:build !windows

package handler

// 非 Windows 平台暂无系统代理注册表读取；留空走「常见代理端口自动探测」兜底。

// sysProxyURL 非 Windows 平台返回空串（无系统代理注册表），
// B.AI 请求依赖环境变量 / 配置端口 / 常见代理端口自动探测。
func sysProxyURL() string {
	return ""
}
