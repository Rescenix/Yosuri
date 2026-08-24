//go:build !windows

package handler

// DHS 自动重启（非 Windows 兜底实现）。
//
// 自动重启依赖 taskkill / PowerShell / Windows 专属 SysProcAttr 字段，
// 非 Windows 平台无法自动拉起 dsh，统一降级为提示用户手动重启（与
// auto_start_other.go / desktop_tray_other.go 同模式）。安装/卸载本身已完成。

// restartDsh 非 Windows 平台不做自动重启，返回手动重启提示。
func restartDsh(profile string) (note string, warning string) {
	return "", "非 Windows 平台不支持自动重启 dsh，请手动重启 dsh --profile " + profile + " 后生效"
}
