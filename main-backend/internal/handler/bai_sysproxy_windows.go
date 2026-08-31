//go:build windows

package handler

// Windows 系统代理读取：HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings
// 的 ProxyEnable + ProxyServer。Clash / 迷雾通等代理软件开启「系统代理」模式时会写入，
// 这样用户即使不知道自己软件的代理端口，B.AI 请求也能自动复用系统代理。

import (
	"strings"

	"golang.org/x/sys/windows/registry"
)

// sysProxyURL 返回 Windows 系统代理的 HTTP 代理 URL（如 http://127.0.0.1:7890）。
// 系统代理未启用或值无效时返回空串。
func sysProxyURL() string {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()

	enable, _, err := k.GetIntegerValue("ProxyEnable")
	if err != nil || enable == 0 {
		return ""
	}
	server, _, err := k.GetStringValue("ProxyServer")
	if err != nil || strings.TrimSpace(server) == "" {
		return ""
	}
	server = strings.TrimSpace(server)

	// ProxyServer 可能是 "host:port" 或 "http=host:port;https=host:port;..."
	if strings.Contains(server, "=") {
		for _, part := range strings.Split(server, ";") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(strings.ToLower(part), "https=") {
				return "http://" + strings.TrimSpace(part[len("https="):])
			}
		}
		for _, part := range strings.Split(server, ";") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(strings.ToLower(part), "http=") {
				return "http://" + strings.TrimSpace(part[len("http="):])
			}
		}
		return ""
	}
	// 纯 "host:port"
	return "http://" + server
}
