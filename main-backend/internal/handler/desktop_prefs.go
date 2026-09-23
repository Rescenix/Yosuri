package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/gin-gonic/gin"
)

// desktopPrefs 是桌面端行为偏好，落在用户数据目录，跨重启保留。
// 与前端 localStorage 分开存：这些项需要后端在启动路径上直接读到
// （例如开机自启要在进程启动时决定是否写注册表，不能等前端回来同步）。
type desktopPrefs struct {
	// AutoStartEnabled 开机自启开关。nil = 用户从未设置过，按出厂默认「开」处理。
	AutoStartEnabled *bool `json:"auto_start_enabled,omitempty"`
	// CloseToTray 点关闭按钮的行为：true = 缩到右下角托盘继续后台，false = 直接退出进程。
	// nil = 用户从未设置过，按出厂默认「缩到托盘」处理（与历史行为一致）。
	CloseToTray *bool `json:"close_to_tray,omitempty"`
}

const desktopPrefsFileName = "desktop_prefs.json"

var (
	desktopPrefsMu  sync.Mutex
	desktopPrefsVal *desktopPrefs
)

func desktopPrefsPath() string {
	return filepath.Join(resceneUserDataDir(), desktopPrefsFileName)
}

// loadDesktopPrefs 读偏好文件（只读一次并缓存；文件不存在/损坏返回 nil，按默认处理）。
func loadDesktopPrefs() *desktopPrefs {
	desktopPrefsMu.Lock()
	defer desktopPrefsMu.Unlock()
	if desktopPrefsVal != nil {
		return desktopPrefsVal
	}
	data, err := os.ReadFile(desktopPrefsPath())
	if err != nil {
		return nil
	}
	var p desktopPrefs
	if json.Unmarshal(data, &p) != nil {
		return nil
	}
	desktopPrefsVal = &p
	return &p
}

// ResetDesktopPrefsCacheForTest 丢掉偏好内存缓存。测试切换 RESCENE_DATA_DIR 后必须调用，
// 否则会读到上一个用例缓存下来的偏好（loadDesktopPrefs 命中缓存就直接返回，不再读盘）。
func ResetDesktopPrefsCacheForTest() {
	desktopPrefsMu.Lock()
	desktopPrefsVal = nil
	desktopPrefsMu.Unlock()
}

// saveDesktopPrefs 增量写偏好（读-改-写，避免覆盖掉其他字段）。
func saveDesktopPrefs(mutate func(*desktopPrefs)) error {
	desktopPrefsMu.Lock()
	defer desktopPrefsMu.Unlock()

	p := desktopPrefs{}
	if data, err := os.ReadFile(desktopPrefsPath()); err == nil {
		_ = json.Unmarshal(data, &p)
	}
	mutate(&p)
	data, err := json.MarshalIndent(&p, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(resceneUserDataDir(), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(desktopPrefsPath(), data, 0o600); err != nil {
		return err
	}
	copied := p
	desktopPrefsVal = &copied
	return nil
}

// autoStartDesired 返回用户是否希望开机自启（未设置过 = true，保持出厂行为不变）。
func autoStartDesired() bool {
	p := loadDesktopPrefs()
	if p == nil || p.AutoStartEnabled == nil {
		return true
	}
	return *p.AutoStartEnabled
}

// AutoStartDesired 供桌面壳（package main）在启动路径上读取用户意愿。
func AutoStartDesired() bool { return autoStartDesired() }

// closeToTrayDesired 返回点关闭按钮时是否缩到右下角托盘（未设置过 = true，保持历史行为）。
// 由用户在设置面板「常规 → 启动」里决定，而不是由程序写死（2026-09-23 用户要求）。
func closeToTrayDesired() bool {
	p := loadDesktopPrefs()
	if p == nil || p.CloseToTray == nil {
		return true
	}
	return *p.CloseToTray
}

// CloseToTrayDesired 供桌面壳在窗口关闭回调（OnBeforeClose）里实时读取。
func CloseToTrayDesired() bool { return closeToTrayDesired() }

// EnableAutoStart / DisableAutoStart 供桌面壳复用同一套注册表读写实现。
func EnableAutoStart() error  { return enableAutoStart() }
func DisableAutoStart() error { return disableAutoStart() }

// HandleGetAutoStart GET /api/desktop/autostart —— 当前开机自启状态。
// enabled 是用户意愿，registered 是注册表实际状态（两者可能因手动改注册表而不一致）。
func HandleGetAutoStart(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"ok":         true,
		"enabled":    autoStartDesired(),
		"registered": autoStartRegistered(),
		"supported":  autoStartSupported(),
	})
}

// HandleSetAutoStart POST /api/desktop/autostart  body: {enabled: bool}
// 开启写注册表，关闭删注册表项，并把意愿落盘供下次启动判断。
func HandleSetAutoStart(c *gin.Context) {
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 enabled"})
		return
	}
	if !autoStartSupported() {
		c.JSON(http.StatusOK, gin.H{"ok": true, "enabled": *req.Enabled, "registered": false})
		return
	}
	var applyErr error
	if *req.Enabled {
		applyErr = enableAutoStart()
	} else {
		applyErr = disableAutoStart()
	}
	if applyErr != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "error": applyErr.Error(),
			"enabled": autoStartDesired(), "registered": autoStartRegistered()})
		return
	}
	enabled := *req.Enabled
	if err := saveDesktopPrefs(func(p *desktopPrefs) { p.AutoStartEnabled = &enabled }); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": true, "enabled": enabled,
			"registered": autoStartRegistered(), "warning": "偏好保存失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "enabled": enabled, "registered": autoStartRegistered()})
}

// HandleGetCloseBehavior GET /api/desktop/close-behavior —— 点关闭按钮时的行为。
func HandleGetCloseBehavior(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true, "close_to_tray": closeToTrayDesired()})
}

// HandleSetCloseBehavior POST /api/desktop/close-behavior  body: {close_to_tray: bool}
// 只落盘偏好，无需重启：OnBeforeClose 每次点关闭按钮都实时读这个值。
func HandleSetCloseBehavior(c *gin.Context) {
	var req struct {
		CloseToTray *bool `json:"close_to_tray"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.CloseToTray == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 close_to_tray"})
		return
	}
	want := *req.CloseToTray
	if err := saveDesktopPrefs(func(p *desktopPrefs) { p.CloseToTray = &want }); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error(),
			"close_to_tray": closeToTrayDesired()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "close_to_tray": want})
}
