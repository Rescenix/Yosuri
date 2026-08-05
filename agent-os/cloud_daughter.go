package main

// cloud_daughter.go —— 女儿云端身份与状态（2026-08-05）
//
// 「每个人都独特，社交才会有意思」：
//   - 名字全局唯一：云端查重（自定义重名 409），token 云端签发不可伪造
//   - 身份：device_id（本地持久）→ 云端恒定同一女儿
//   - 状态同步：world/stats/journal 打包存云端（单库行级隔离，鉴权只放行本人）
//   - 社交：明信片模式——发动态只带名字，inbox 随机收到其他女儿的动态
//
// 离线降级铁律：云端不可用 → CloudOK=false，本地照常工作（本地模拟社交），
// 绝不让网络问题打断她的生活。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// cloudBase 云端基址：RESCENE_CLOUD_URL 覆盖，默认生产
func cloudBase() string {
	if u := os.Getenv("RESCENE_CLOUD_URL"); u != "" {
		return strings.TrimRight(u, "/")
	}
	return "https://rescenecloud.onrender.com"
}

var cloudHTTP = &http.Client{Timeout: 6 * time.Second}

// cloudReq 一次带 JSON body 的 POST；返回响应 body（非 2xx 时返回错误）
func cloudReq(method, path string, payload any) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, cloudBase()+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := cloudHTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 512<<10))
	if resp.StatusCode >= 400 {
		return data, fmt.Errorf("云端 HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return data, nil
}

// ensureCloudIdentity 确保女儿有云端身份：无 token → 注册（同一 device 恒定同一女儿）。
// 云端不可用 → CloudOK=false 静默降级，本地名字照常用。
func ensureCloudIdentity(w *worldState, home string) {
	if w.Token != "" {
		w.CloudOK = true // 已有身份，假定可用（sync 失败会降级）
		return
	}
	if w.DeviceID == "" {
		w.DeviceID = newDeviceID()
		w.save(home)
	}
	// 首次注册：优先用户自定义名？CLI 无输入入口，先云端默认唯一名，改名走 /rename 意图
	resp, err := cloudReq("POST", "/api/daughter/register", map[string]any{
		"device_id": w.DeviceID,
		"name":      w.Name,
	})
	if err != nil {
		w.CloudOK = false
		logLive(filepath.Join(home, "live.log"), fmt.Sprintf("☁️ 云端未连接（%v），本地模式运行", err))
		return
	}
	var out struct {
		Name    string `json:"name"`
		Token   string `json:"token"`
		IsNew   bool   `json:"is_new"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(resp, &out); err != nil || out.Token == "" {
		w.CloudOK = false
		return
	}
	w.Name = out.Name
	w.Token = out.Token
	w.CloudOK = true
	w.save(home)
}

// daughterSyncPush 打包 world/stats/journal 尾部推送到云端（异步、失败静默）
func daughterSyncPush(w *worldState, home string) {
	if w == nil || w.Token == "" || !w.CloudOK {
		return
	}
	pack := buildSyncPack(w, home)
	go func() {
		_, err := cloudReq("PUT", "/api/daughter/sync", map[string]any{
			"token": w.Token,
			"data":  pack,
		})
		if err != nil {
			w.CloudOK = false // 连续失败降级，避免每轮重试噪音
			logLive(filepath.Join(home, "live.log"), fmt.Sprintf("☁️ 同步失败（降级本地）: %v", err))
		}
	}()
}

// buildSyncPack 数据包：world + stats + journal 尾部
func buildSyncPack(w *worldState, home string) map[string]any {
	pack := map[string]any{}
	// world 完整状态（不含 token/device_id 敏感字段由服务端行隔离保护，但 token 不随包上传）
	worldCopy := *w
	worldCopy.Token = ""
	worldCopy.DeviceID = ""
	pack["world"] = worldCopy
	// stats
	if data, err := os.ReadFile(filepath.Join(home, "stats.json")); err == nil {
		var st map[string]any
		if json.Unmarshal(data, &st) == nil {
			pack["stats"] = st
		}
	}
	// journal 尾部（最近 2000 字符，跨设备恢复可见）
	if data, err := os.ReadFile(filepath.Join(home, "journal.md")); err == nil {
		s := string(data)
		if len(s) > 2000 {
			s = s[len(s)-2000:]
		}
		pack["journal_tail"] = s
	}
	return pack
}

// daughterSyncPull 启动时拉云端数据包 → 覆盖本地（跨设备恢复）。
// 云端无数据（新女儿）→ 保持本地，随后 push 初始化。
func daughterSyncPull(w *worldState, home string) {
	if w == nil || w.Token == "" {
		return
	}
	resp, err := cloudReq("GET", "/api/daughter/sync?token="+w.Token, nil)
	if err != nil {
		w.CloudOK = false
		return
	}
	var out struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(resp, &out); err != nil {
		return
	}
	w.CloudOK = true
	if raw, ok := out.Data["world"]; ok && len(raw) > 2 {
		var cloudWorld worldState
		if json.Unmarshal(raw, &cloudWorld) == nil && cloudWorld.WorldSeed != 0 {
			// 云端权威覆盖（保留本地身份字段）
			localDevice, localToken, localName := w.DeviceID, w.Token, w.Name
			*w = cloudWorld
			w.DeviceID, w.Token, w.Name, w.CloudOK = localDevice, localToken, localName, true
			w.save(home)
			logLive(filepath.Join(home, "live.log"), "☁️ 云端状态已恢复（跨设备）")
		}
	}
	if raw, ok := out.Data["journal_tail"]; ok && len(raw) > 2 {
		var tail string
		if json.Unmarshal(raw, &tail) == nil && tail != "" {
			appendJournalTail(home, tail)
		}
	}
}

// appendJournalTail 把云端日记尾部合并回本地（若本地没有这些内容）
func appendJournalTail(home, tail string) {
	path := filepath.Join(home, "journal.md")
	local, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(local), firstLine(tail)) {
		f, _ := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if f != nil {
			f.WriteString("\n" + tail)
			f.Close()
		}
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// daughterSocialPost 她发一条动态（只带名字，匿名明信片）
func daughterSocialPost(w *worldState, home, content string) bool {
	if w == nil || w.Token == "" || !w.CloudOK {
		return false
	}
	_, err := cloudReq("POST", "/api/daughter/social/post", map[string]any{
		"token":   w.Token,
		"content": content,
	})
	if err != nil {
		w.CloudOK = false
		return false
	}
	return true
}

// daughterSocialInbox 收到其他女儿的动态（真实社交）；云端不可用返回空
func daughterSocialInbox(w *worldState) []string {
	if w == nil || w.Token == "" || !w.CloudOK {
		return nil
	}
	resp, err := cloudReq("GET", "/api/daughter/social/inbox?token="+w.Token, nil)
	if err != nil {
		w.CloudOK = false
		return nil
	}
	var out struct {
		Posts []struct {
			Name    string `json:"name"`
			Content string `json:"content"`
		} `json:"posts"`
	}
	if err := json.Unmarshal(resp, &out); err != nil {
		return nil
	}
	var msgs []string
	for _, p := range out.Posts {
		msgs = append(msgs, fmt.Sprintf("%s：%s", p.Name, p.Content))
	}
	return msgs
}

// detectRename 从用户的话里嗅出改名意图：返回新名字（无意图返回空）
// 支持：你叫X / 叫你X / 名字叫X / 起个名字X / 改名X / 起名叫X
// 排除疑问词（"你叫什么名字"不该改名）
func detectRename(input string) string {
	kws := []string{"你叫", "叫你", "名字叫", "起个名字", "起名叫", "改名"}
	stopWords := []string{"什么", "啥", "怎么", "谁", "哪些", "吗"}
	for _, kw := range kws {
		if i := strings.Index(input, kw); i >= 0 {
			// 坑：i 是字节位置，len([]rune(kw)) 是 rune 数——中文必须统一到 rune 空间
			ri := len([]rune(input[:i]))
			rest := string([]rune(input)[ri+len([]rune(kw)):])
			rest = strings.TrimSpace(rest)
			// 黑名单：紧跟在后的疑问词 → 不是改名
			skip := false
			for _, sw := range stopWords {
				if strings.HasPrefix(rest, sw) {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
			var name []rune
			for _, r := range rest {
				if r == ' ' || r == '，' || r == '。' || r == '！' || r == '？' || r == '.' || r == ',' || r == '吧' || r == '啊' || r == '呀' || r == '呢' {
					break
				}
				name = append(name, r)
				if len(name) >= 12 {
					break
				}
			}
			if len(name) >= 1 {
				return string(name)
			}
		}
	}
	return ""
}

// daughterRename 改名（云端全局查重）：成功返回新名
func daughterRename(w *worldState, home, newName string) (string, error) {
	if w == nil || w.Token == "" {
		return "", fmt.Errorf("云端未连接，无法改名")
	}
	newName = strings.TrimSpace(newName)
	if newName == "" || len([]rune(newName)) > 12 {
		return "", fmt.Errorf("名字要 1-12 个字符哦")
	}
	resp, err := cloudReq("POST", "/api/daughter/rename", map[string]any{
		"token": w.Token,
		"name":  newName,
	})
	if err != nil {
		w.CloudOK = false
		return "", fmt.Errorf("云端改名失败（可能是重名或未连接）")
	}
	var out struct {
		Name  string `json:"name"`
		Error string `json:"error"`
	}
	if json.Unmarshal(resp, &out) != nil || out.Name == "" {
		return "", fmt.Errorf("这个名字已经有女儿啦，换一个独特的吧")
	}
	w.Name = out.Name
	w.save(home)
	logLive(filepath.Join(home, "live.log"), fmt.Sprintf("💗 改名：%s", out.Name))
	return out.Name, nil
}
