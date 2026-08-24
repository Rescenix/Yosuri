//go:build windows

package handler

// DHS 自动重启（安装/卸载成功后直接重启 dsh，插件立即生效）。
//
// 仅 Windows 实现：依赖 taskkill / PowerShell / syscall.SysProcAttr{CreationFlags}
// 等 Windows 专用 API，故单独用 //go:build windows 隔离，保证 dhs_community.go
// 能在非 Windows 平台编译。

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// restartDsh 重启指定 profile 的 dsh 进程：找到运行中的进程 → 杀 → detached 拉起 → 端口健康检查。
// 返回 (成功提示, 警告)。任何失败都不阻塞（安装/卸载本身已完成），只降级为提示手动重启。
// 环境变量 DHS_AUTO_RESTART=0 时跳过（测试隔离实例用，避免误杀真实 dsh）。
func restartDsh(profile string) (note string, warning string) {
	if os.Getenv("DHS_AUTO_RESTART") == "0" {
		return "", "未自动重启 dsh（DHS_AUTO_RESTART=0 测试模式），重启 dsh --profile " + profile + " 后生效"
	}
	pid := findDshPid(profile)
	if pid == 0 {
		return "", "未发现运行中的 dsh 进程，重启 dsh --profile " + profile + " 后生效"
	}
	nodeBin, binJS, ok := dshLaunchCommand()
	if !ok {
		return "", "已写入 profile 但未能定位 dsh 启动命令（node/bin.js），请手动重启 dsh --profile " + profile
	}
	if kerr := exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid)).Run(); kerr != nil {
		return "", "已写入 profile 但停止旧 dsh 进程失败（" + kerr.Error() + "），请手动重启 dsh"
	}
	waitPortFree(dshWebPort(), 5*time.Second)
	if serr := spawnDsh(nodeBin, binJS, profile); serr != nil {
		return "", "旧 dsh 已停止但拉起失败（" + serr.Error() + "），请手动重启 dsh --profile " + profile
	}
	if waitPortUp(dshWebPort(), 40*time.Second) {
		return "已自动重启 dsh，插件立即生效", ""
	}
	return "", "已拉起 dsh 但面板端口尚未就绪（可能在启动中），稍后刷新面板即可"
}

// findDshPid 通过 PowerShell 找到运行中的 `node .../dsh/lib/bin.js <profile>` 进程 PID（0=未找到）。
func findDshPid(profile string) int {
	re := fmt.Sprintf("bin\\.js[\\s/\\\\]+%s($|\\s)", regexp.QuoteMeta(profile))
	script := fmt.Sprintf(
		`Get-CimInstance Win32_Process -Filter "Name='node.exe'" | Where-Object { $_.CommandLine -match '%s' } | Select-Object -First 1 -ExpandProperty ProcessId`,
		re)
	out, err := exec.Command("powershell.exe", "-NoProfile", "-Command", script).Output()
	if err != nil {
		return 0
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return pid
}

// dshLaunchCommand 定位 node 可执行文件与 dsh 的 bin.js 绝对路径。
func dshLaunchCommand() (nodeBin, binJS string, ok bool) {
	nodeBin, _ = exec.LookPath("node")
	if nodeBin == "" {
		for _, p := range []string{
			`C:\Program Files\nodejs\node.exe`,
			filepath.Join(os.Getenv("LOCALAPPDATA"), "hermes", "node", "node.exe"),
		} {
			if fi, e := os.Stat(p); e == nil && !fi.IsDir() {
				nodeBin = p
				break
			}
		}
	}
	if nodeBin == "" {
		return "", "", false
	}
	appdata := os.Getenv("APPDATA")
	candidates := []string{
		filepath.Join(appdata, "npm", "node_modules", "@deepseek-ai", "dsh", "lib", "bin.js"),
	}
	for _, p := range candidates {
		if fi, e := os.Stat(p); e == nil && !fi.IsDir() {
			binJS = p
			break
		}
	}
	if binJS == "" {
		return "", "", false
	}
	return nodeBin, binJS, true
}

// spawnDsh 以脱离父进程的方式拉起 `node bin.js <profile>`，输出写日志。
func spawnDsh(nodeBin, binJS, profile string) error {
	logPath := filepath.Join(dshHomeDir(), "dsh-web-restart.log")
	f, ferr := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if ferr != nil {
		f, _ = os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	}
	defer f.Close()
	cmd := exec.Command(nodeBin, binJS, profile)
	cmd.Stdout = f
	cmd.Stderr = f
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x00000200 | 0x00000008, // CREATE_NEW_PROCESS_GROUP | DETACHED_PROCESS
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// dshHomeDir 与端点同一套 DSH_HOME 解析。
func dshHomeDir() string {
	if h := strings.TrimSpace(os.Getenv("DSH_HOME")); h != "" {
		return h
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".dsh")
	}
	return ".dsh"
}

// dshWebPort dsh 面板端口，默认 3080，env DSH_WEB_PORT 可覆盖。
func dshWebPort() string {
	if p := strings.TrimSpace(os.Getenv("DSH_WEB_PORT")); p != "" {
		return p
	}
	return "3080"
}

func waitPortFree(port string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:"+port, 500*time.Millisecond)
		if err != nil {
			return // 端口已释放
		}
		conn.Close()
		time.Sleep(300 * time.Millisecond)
	}
}

func waitPortUp(port string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:"+port, 800*time.Millisecond)
		if err == nil {
			conn.Close()
			return true
		}
		time.Sleep(700 * time.Millisecond)
	}
	return false
}
