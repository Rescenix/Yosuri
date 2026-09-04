package handler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// 视频生成任务落盘：任务状态原来只在内存 map 里，应用重启（或热更新换 exe）后
// 前端继续轮询同一个 task_id 会拿到 404「任务不存在」，而前端不处理这个分支，
// 于是界面永远停在「生成中」——用户看到的就是点了没反应、也没有任何错误反馈。
// 现在每次状态变更同步写一个 JSON 到 ~/rescene_data/studio_tasks/，
// 内存查不到时回读磁盘，能明确区分「已完成/已失败」与「任务记录已丢失」。

const studioTasksDir = "studio_tasks"

func studioTaskPath(id string) string {
	safe := strings.NewReplacer("/", "_", "\\", "_", "..", "_").Replace(id)
	return filepath.Join(resceneUserDataDir(), studioTasksDir, safe+".json")
}

// persistAgnesTask 把任务快照写盘（失败静默：落盘是可用性兜底，不该阻断生成主流程）。
func persistAgnesTask(id string, t *agnesTask) {
	if id == "" || t == nil {
		return
	}
	t.Mu.Lock()
	snap := struct {
		Status  string `json:"status"`
		Video   string `json:"video"`
		Name    string `json:"name"`
		Size    string `json:"size"`
		Seconds string `json:"seconds"`
		Err     string `json:"error"`
	}{t.Status, t.Video, t.Name, t.Size, t.Seconds, t.Err}
	t.Mu.Unlock()

	dir := filepath.Join(resceneUserDataDir(), studioTasksDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	data, err := json.Marshal(snap)
	if err != nil {
		return
	}
	_ = os.WriteFile(studioTaskPath(id), data, 0o600)
}

// loadAgnesTaskFromDisk 从磁盘读回任务快照；文件不存在返回 nil。
func loadAgnesTaskFromDisk(id string) *agnesTask {
	data, err := os.ReadFile(studioTaskPath(id))
	if err != nil {
		return nil
	}
	var snap struct {
		Status  string `json:"status"`
		Video   string `json:"video"`
		Name    string `json:"name"`
		Size    string `json:"size"`
		Seconds string `json:"seconds"`
		Err     string `json:"error"`
	}
	if json.Unmarshal(data, &snap) != nil || snap.Status == "" {
		return nil
	}
	return &agnesTask{
		Status: snap.Status, Video: snap.Video, Name: snap.Name,
		Size: snap.Size, Seconds: snap.Seconds, Err: snap.Err,
	}
}
