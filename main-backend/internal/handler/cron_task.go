package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"git.sr.ht/~jackmordaunt/go-toast/v2"
)

// ==================== 定时任务（cron） ====================
// 前端 ScheduledTaskModal 创建任务 → POST /api/cron/create 落盘；
// 后台调度器每 30s 检查一次 cron 匹配，到点调 Windows 原生 toast（右下角）。

type CronTask struct {
	ID        string    `json:"id"`
	Name      string    `json:"name,omitempty"`
	Prompt    string    `json:"prompt"`
	Cron      string    `json:"cron"`
	Frequency string    `json:"frequency"`
	DeliverTo string    `json:"deliverTo,omitempty"`
	Model     string    `json:"model,omitempty"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
	LastFired time.Time `json:"lastFired,omitempty"`
}

const (
	cronAppID     = "ResceneAgent"    // go-toast AUMID（写注册表，Win10+ 生效）
	cronTasksFile = "cron_tasks.json" // 落盘文件名，放 ~/rescene_data/
)

var (
	cronMu       sync.RWMutex
	cronTasks    = map[string]*CronTask{}
	cronToasts   = make(map[string]time.Time) // taskID → 上次触发时间（防同分钟重复弹）
	cronSaveMu   sync.Mutex                   // 串行化落盘，防并发写同一个 .tmp
	cronInitOnce sync.Once
)

func cronTasksPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, "rescene_data", cronTasksFile)
}

func init() {
	startCronScheduler()
}

// ==================== 存储 ====================

func loadCronTasks() {
	data, err := os.ReadFile(cronTasksPath())
	if err != nil {
		return // 首次运行无文件
	}
	var list []CronTask
	if err := json.Unmarshal(data, &list); err != nil {
		log.Printf("⚠️ [定时任务] 解析 %s 失败: %v", cronTasksFile, err)
		return
	}
	cronMu.Lock()
	defer cronMu.Unlock()
	for i := range list {
		t := list[i]
		cronTasks[t.ID] = &t
	}
}

func saveCronTasks() {
	cronMu.RLock()
	list := make([]CronTask, 0, len(cronTasks))
	for _, t := range cronTasks {
		list = append(list, *t)
	}
	cronMu.RUnlock()
	sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt.Before(list[j].CreatedAt) })
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		log.Printf("⚠️ [定时任务] 序列化失败: %v", err)
		return
	}
	cronSaveMu.Lock()
	defer cronSaveMu.Unlock()
	path := cronTasksPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Printf("⚠️ [定时任务] 创建目录失败: %v", err)
		return
	}
	// 原子落盘：先写临时文件再 rename，避免进程在写入中途被杀留下半截 JSON。
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		log.Printf("⚠️ [定时任务] 落盘失败: %v", err)
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		// Windows 上 rename 到已存在文件需要 MoveFileEx(MOVEFILE_REPLACE_EXISTING)，
		// Go 的 os.Rename 底层就是它，一般可直接覆盖；失败时退一步删再改名。
		_ = os.Remove(path)
		if err2 := os.Rename(tmp, path); err2 != nil {
			log.Printf("⚠️ [定时任务] 落盘失败: %v", err2)
		}
	}
}

// ==================== cron 匹配（5 段：分 时 日 月 周） ====================

// cronFieldMatches 匹配单个字段：支持 *、*/n、n、a-b、a-b/n、a,b
func cronFieldMatches(field string, v int) bool {
	field = strings.TrimSpace(field)
	if field == "*" {
		return true
	}
	if strings.Contains(field, ",") {
		for _, part := range strings.Split(field, ",") {
			if cronFieldMatches(part, v) {
				return true
			}
		}
		return false
	}
	// 步进：*/n 或 a-b/n
	if idx := strings.Index(field, "/"); idx >= 0 {
		base := field[:idx]
		step, err := strconv.Atoi(field[idx+1:])
		if err != nil || step <= 0 {
			return false
		}
		if base == "*" {
			return v%step == 0
		}
		lo, err := strconv.Atoi(base)
		return err == nil && v >= lo && (v-lo)%step == 0
	}
	if strings.Contains(field, "-") {
		parts := strings.SplitN(field, "-", 2)
		lo, err1 := strconv.Atoi(parts[0])
		hi, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			return false
		}
		return v >= lo && v <= hi
	}
	n, err := strconv.Atoi(field)
	return err == nil && v == n
}

// cronFieldValid 校验单个字段语法：*、n、a-b、*/n、a-b/n、逗号列表（元素递归校验）
func cronFieldValid(field string) bool {
	field = strings.TrimSpace(field)
	if field == "" {
		return false
	}
	if strings.Contains(field, ",") {
		for _, part := range strings.Split(field, ",") {
			if !cronFieldValid(part) {
				return false
			}
		}
		return true
	}
	if field == "*" {
		return true
	}
	if idx := strings.Index(field, "/"); idx >= 0 {
		base := field[:idx]
		step, err := strconv.Atoi(field[idx+1:])
		if err != nil || step <= 0 {
			return false
		}
		if base == "*" {
			return true
		}
		_, err = strconv.Atoi(base)
		return err == nil
	}
	if strings.Contains(field, "-") {
		parts := strings.SplitN(field, "-", 2)
		_, err1 := strconv.Atoi(parts[0])
		_, err2 := strconv.Atoi(parts[1])
		return err1 == nil && err2 == nil
	}
	_, err := strconv.Atoi(field)
	return err == nil
}

// cronExprValid 校验 5 段 cron 表达式（分 时 日 月 周）语法
func cronExprValid(expr string) bool {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return false
	}
	for _, f := range fields {
		if !cronFieldValid(f) {
			return false
		}
	}
	return true
}

func cronMatches(expr string, t time.Time) bool {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return false
	}
	if !cronFieldMatches(fields[0], t.Minute()) {
		return false
	}
	if !cronFieldMatches(fields[1], t.Hour()) {
		return false
	}
	if !cronFieldMatches(fields[2], t.Day()) {
		return false
	}
	if !cronFieldMatches(fields[3], int(t.Month())) {
		return false
	}
	// 周：cron 里 0 和 7 都表示周日，Go Weekday() 周日=0
	dow := int(t.Weekday())
	if dow == 0 {
		dow = 7 // 让表达式里的 7 能匹配；0 由下面单独兜
	}
	if !cronFieldMatches(fields[4], dow) && !(dow == 7 && cronFieldMatches(fields[4], 0)) {
		return false
	}
	return true
}

// ==================== 调度器 ====================

func startCronScheduler() {
	cronInitOnce.Do(func() {
		loadCronTasks()
		// 注册 AUMID：go-toast 需要应用名在注册表 AppUserModelId 下才能显示 toast
		// （写 HKCU\SOFTWARE\Classes\AppUserModelId\ResceneAgent，幂等）
		_ = toast.SetAppData(toast.AppData{
			AppID:               cronAppID,
			IconBackgroundColor: "3b82f6",
		})
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				fireDueCronTasks()
			}
		}()
		log.Printf("🕒 [定时任务] 调度器已启动（已加载 %d 个任务）", len(cronTasks))
	})
}

func fireDueCronTasks() {
	now := time.Now()
	cronMu.RLock()
	var due []*CronTask
	for id, t := range cronTasks {
		if !t.Enabled {
			continue
		}
		if !cronMatches(t.Cron, now) {
			continue
		}
		if last, ok := cronToasts[id]; ok && last.Format("2006-01-02 15:04") == now.Format("2006-01-02 15:04") {
			continue // 同分钟已弹过
		}
		due = append(due, t)
	}
	cronMu.RUnlock()
	for _, t := range due {
		notifyCronTask(t, now)
	}
}

func notifyCronTask(t *CronTask, now time.Time) {
	title := t.Name
	if title == "" {
		title = "定时任务"
	}
	msg := t.Prompt
	if runes := []rune(msg); len(runes) > 60 {
		msg = string(runes[:60]) + "…"
	}
	// 先登记防重弹，再推送：即使推送失败也不会在同一分钟内反复重试刷屏
	cronMu.Lock()
	t.LastFired = now
	cronToasts[t.ID] = now
	cronMu.Unlock()
	if err := pushWindowsToast(title, msg); err != nil {
		log.Printf("⚠️ [定时任务] %s 弹窗失败: %v", t.ID, err)
		return
	}
	saveCronTasks()
	log.Printf("🔔 [定时任务] 已触发: %s (%s)", title, t.Cron)
}

// pushWindowsToast 弹 Windows 右下角原生通知；非 Windows 平台为 no-op。
func pushWindowsToast(title, message string) error {
	n := toast.Notification{
		AppID: cronAppID,
		Title: title,
		Body:  message,
	}
	return n.Push()
}

// ==================== HTTP API ====================

func HandleCronCreate(c *gin.Context) {
	var req struct {
		Name      string `json:"name"`
		Prompt    string `json:"prompt" binding:"required"`
		Cron      string `json:"cron" binding:"required"`
		Frequency string `json:"frequency"`
		DeliverTo string `json:"deliverTo"`
		Model     string `json:"model"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "prompt 和 cron 必填"})
		return
	}
	if !cronExprValid(req.Cron) {
		c.JSON(400, gin.H{"error": "cron 表达式格式不正确（需 5 段：分 时 日 月 周）"})
		return
	}
	task := &CronTask{
		ID:        fmt.Sprintf("cron_%d", time.Now().UnixNano()),
		Name:      req.Name,
		Prompt:    req.Prompt,
		Cron:      req.Cron,
		Frequency: req.Frequency,
		DeliverTo: req.DeliverTo,
		Model:     req.Model,
		Enabled:   true,
		CreatedAt: time.Now(),
	}
	cronMu.Lock()
	cronTasks[task.ID] = task
	cronMu.Unlock()
	saveCronTasks()
	log.Printf("📌 [定时任务] 已创建: %s cron=%s", task.ID, task.Cron)
	c.JSON(200, gin.H{"id": task.ID, "ok": true})
}

func HandleCronList(c *gin.Context) {
	cronMu.RLock()
	list := make([]CronTask, 0, len(cronTasks))
	for _, t := range cronTasks {
		list = append(list, *t)
	}
	cronMu.RUnlock()
	sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt.Before(list[j].CreatedAt) })
	c.JSON(200, list)
}

// HandleCronToggle 启用/停用定时任务（前端管理面板的开关用）。
// body: {"enabled": true|false}；不传 enabled 时按当前值取反。
func HandleCronToggle(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	_ = c.ShouldBindJSON(&req)
	cronMu.Lock()
	t, ok := cronTasks[id]
	enabled := false
	if ok {
		if req.Enabled != nil {
			t.Enabled = *req.Enabled
		} else {
			t.Enabled = !t.Enabled
		}
		enabled = t.Enabled
	}
	cronMu.Unlock()
	if !ok {
		c.JSON(404, gin.H{"error": "任务不存在"})
		return
	}
	saveCronTasks()
	c.JSON(200, gin.H{"ok": true, "enabled": enabled})
}

func HandleCronDelete(c *gin.Context) {
	id := c.Param("id")
	cronMu.Lock()
	_, ok := cronTasks[id]
	if ok {
		delete(cronTasks, id)
		delete(cronToasts, id)
	}
	cronMu.Unlock()
	if !ok {
		c.JSON(404, gin.H{"error": "任务不存在"})
		return
	}
	saveCronTasks()
	c.JSON(200, gin.H{"ok": true})
}

// HandleCronTest 手动触发一次弹窗，用于验证系统通知是否可用。
func HandleCronTest(c *gin.Context) {
	var req struct {
		Title   string `json:"title"`
		Message string `json:"message"`
	}
	_ = c.ShouldBindJSON(&req)
	title := req.Title
	if title == "" {
		title = "Rescene 定时任务"
	}
	msg := req.Message
	if msg == "" {
		msg = "这是一条测试通知"
	}
	if err := pushWindowsToast(title, msg); err != nil {
		c.JSON(500, gin.H{"error": "弹窗失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
