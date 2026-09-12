package handler

// component_download.go — 可选组件（ffmpeg 等）的按需下载。
//
// 组件不随安装包捆绑：用户用到视频/音频类功能时，由设置侧的「组件下载」
// 入口或 agent 的 component_download 工具触发，从国内可达的镜像拉取，
// 解压到 ~/rescene_data/components/<id>/，供 ffmpeg locator 直接命中。
//
// 下载源策略：优先 gh-proxy 镜像（实测国内可达、支持 Range），
// 失败回退官方直链。zip 内只挑 bin 目录下的 ffmpeg.exe / ffprobe.exe。

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"backend/internal/ai/core"

	"github.com/gin-gonic/gin"
)

// componentDef 一个可下载组件的元数据。
type componentDef struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// URLs 按优先级排列的 zip 直链，逐个尝试直到成功。
	URLs []string
	// Binaries 解压后需要落地的可执行文件名（从包内任意路径里挑）。
	Binaries []string
	// VerifyArgs 装完后的自检参数（如 -version），空则不校验。
	VerifyArgs []string
}

// gyanWinZip 是 gyan.dev 的 win64 essentials 包（GitHub 仓库 GyanD/codexffmpeg
// 同步发布同名资产），gh-proxy 前缀实测国内可达。
var componentCatalog = []componentDef{
	{
		ID:          "ffmpeg",
		Name:        "FFmpeg",
		Description: "视频/音频处理引擎：长视频拼接、抽帧、配音合成、去水印",
		URLs: []string{
			"https://gh-proxy.com/https://github.com/GyanD/codexffmpeg/releases/download/9.0.1/ffmpeg-9.0.1-essentials_build.zip",
			"https://github.com/GyanD/codexffmpeg/releases/download/9.0.1/ffmpeg-9.0.1-essentials_build.zip",
			"https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip",
		},
		Binaries:   []string{"ffmpeg.exe", "ffprobe.exe"},
		VerifyArgs: []string{"-version"},
	},
}

func componentDefByID(id string) (componentDef, bool) {
	for _, c := range componentCatalog {
		if c.ID == id {
			return c, true
		}
	}
	return componentDef{}, false
}

// componentsRootDir 组件根目录 ~/rescene_data/components/。
func componentsRootDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "rescene_data", "components")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return dir, nil
}

// componentBinDir 某组件的可执行文件目录（locator 命中路径）。
func componentBinDir(id string) (string, error) {
	root, err := componentsRootDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, id, "bin"), nil
}

// componentInstalled 组件是否已就位：bin 目录里每个必需 exe 都在。
func componentInstalled(def componentDef) bool {
	dir, err := componentBinDir(def.ID)
	if err != nil {
		return false
	}
	for _, bin := range def.Binaries {
		if _, err := os.Stat(filepath.Join(dir, bin)); err != nil {
			return false
		}
	}
	return true
}

// componentStatus 单个组件的状态：系统 PATH 有、组件目录有、还是缺。
type componentStatus struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	OnPath      bool   `json:"on_path"`
	Installed   bool   `json:"installed"`
	InstallPath string `json:"install_path,omitempty"`
	Available   bool   `json:"available"` // 当前平台可下载
}

func buildComponentStatus(def componentDef) componentStatus {
	st := componentStatus{
		ID:          def.ID,
		Name:        def.Name,
		Description: def.Description,
		Available:   runtime.GOOS == "windows",
	}
	if _, err := exec.LookPath(def.ID); err == nil {
		st.OnPath = true
	}
	if componentInstalled(def) {
		st.Installed = true
		if dir, err := componentBinDir(def.ID); err == nil {
			st.InstallPath = dir
		}
	}
	return st
}

// ---------- 下载任务 ----------

type componentJob struct {
	ID        string `json:"id"`
	State     string `json:"state"` // downloading / extracting / done / error
	Percent   int    `json:"percent"`
	Error     string `json:"error,omitempty"`
	Source    string `json:"source,omitempty"` // 实际命中的下载源
	StartedAt int64  `json:"started_at"`
}

var (
	componentJobsMu sync.Mutex
	componentJobs   = map[string]*componentJob{} // key: component id
	componentJobRun sync.Map                     // key: component id -> struct{}（互斥：同一组件只允许一个在途任务）
)

func componentJobSnapshot(id string) *componentJob {
	componentJobsMu.Lock()
	defer componentJobsMu.Unlock()
	if j, ok := componentJobs[id]; ok {
		cp := *j
		return &cp
	}
	return nil
}

func (j *componentJob) update(fn func(*componentJob)) {
	componentJobsMu.Lock()
	defer componentJobsMu.Unlock()
	fn(j)
}

// startComponentDownload 后台起下载；已有在途任务时直接复用。
func startComponentDownload(def componentDef) (*componentJob, bool) {
	if _, loaded := componentJobRun.LoadOrStore(def.ID, struct{}{}); loaded {
		return componentJobSnapshot(def.ID), false
	}
	j := &componentJob{ID: def.ID, State: "downloading", StartedAt: time.Now().Unix()}
	componentJobsMu.Lock()
	componentJobs[def.ID] = j
	componentJobsMu.Unlock()
	go runComponentDownload(def, j)
	return j, true
}

func runComponentDownload(def componentDef, j *componentJob) {
	defer componentJobRun.Delete(def.ID)
	var lastErr error
	for _, u := range def.URLs {
		err := downloadAndInstall(def, u, j)
		if err == nil {
			j.update(func(jj *componentJob) { jj.State = "done"; jj.Percent = 100; jj.Source = u; jj.Error = "" })
			return
		}
		lastErr = err
	}
	j.update(func(jj *componentJob) {
		jj.State = "error"
		jj.Error = fmt.Sprintf("所有下载源均失败: %v", lastErr)
	})
}

// downloadAndInstall 从 url 下载 zip 到临时文件，再解压所需二进制。
func downloadAndInstall(def componentDef, url string, j *componentJob) error {
	tmp, err := os.CreateTemp("", "yosuri-comp-*.zip")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	defer tmp.Close()

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: HTTP %d", url, resp.StatusCode)
	}

	var written int64
	total := resp.ContentLength // -1 表示未知
	buf := make([]byte, 256*1024)
	for {
		n, rerr := io.ReadFull(resp.Body, buf)
		if n > 0 {
			if _, werr := tmp.Write(buf[:n]); werr != nil {
				return werr
			}
			written += int64(n)
			if total > 0 {
				pct := int(float64(written) / float64(total) * 90) // 下载占 0-90%，解压收尾
				j.update(func(jj *componentJob) {
					if jj.State == "downloading" && pct > jj.Percent {
						jj.Percent = pct
					}
				})
			}
		}
		if rerr == io.EOF || rerr == io.ErrUnexpectedEOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	tmp.Close()
	return extractComponentBinaries(def, tmpName, j)
}

// extractComponentBinaries 从 zip 里挑出 def.Binaries 指定的文件，
// 落到 components/<id>/bin/。先写临时名再原子改名，避免半截 exe 被 locator 命中。
func extractComponentBinaries(def componentDef, zipPath string, j *componentJob) error {
	j.update(func(jj *componentJob) { jj.State = "extracting" })
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("压缩包损坏或不可读: %w", err)
	}
	defer r.Close()

	dir, err := componentBinDir(def.ID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	want := map[string]bool{}
	for _, b := range def.Binaries {
		want[strings.ToLower(b)] = false
	}
	for _, f := range r.File {
		base := strings.ToLower(filepath.Base(f.Name))
		if _, ok := want[base]; !ok || f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		tmpPath := filepath.Join(dir, base+".download")
		out, err := os.Create(tmpPath)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			os.Remove(tmpPath)
			return err
		}
		final := filepath.Join(dir, base)
		if err := os.Rename(tmpPath, final); err != nil {
			os.Remove(tmpPath)
			return err
		}
		if runtime.GOOS != "windows" {
			_ = os.Chmod(final, 0755)
		}
		want[base] = true
	}
	for b, found := range want {
		if !found {
			return fmt.Errorf("压缩包里找不到 %s", b)
		}
	}
	// 自检：跑一次 -version 确认能执行
	if len(def.VerifyArgs) > 0 && len(def.Binaries) > 0 {
		bin := filepath.Join(dir, def.Binaries[0])
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if out, err := hiddenCommandContext(ctx, bin, def.VerifyArgs...).CombinedOutput(); err != nil {
			return fmt.Errorf("组件自检失败: %v %s", err, truncateTail(string(out), 200))
		}
	}
	return nil
}

// ---------- locator ----------

// findComponentBin 定位组件可执行文件：PATH 优先，其次组件目录。
// 返回绝对路径；两处都没有时返回引导安装的错误。
func findComponentBin(name string) (string, error) {
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	if p, err := exec.LookPath(name + ".exe"); err == nil {
		return p, nil
	}
	// 按二进制名在目录里找：ffmpeg/ffprobe 可能同属一个组件（id=ffmpeg）。
	for _, def := range componentCatalog {
		matched := false
		for _, b := range def.Binaries {
			if strings.EqualFold(b, name) || strings.EqualFold(strings.TrimSuffix(b, ".exe"), name) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		if bin, err := componentBinDir(def.ID); err == nil {
			for _, b := range def.Binaries {
				if strings.EqualFold(b, name) || strings.EqualFold(strings.TrimSuffix(b, ".exe"), name) {
					full := filepath.Join(bin, b)
					if _, err := os.Stat(full); err == nil {
						return full, nil
					}
				}
			}
		}
		return "", fmt.Errorf("未找到 %s。请点侧栏「组件下载」安装 %s 组件，或让 AI 助手执行 component_download 安装", name, def.ID)
	}
	return "", fmt.Errorf("未找到 %s，且没有对应的可下载组件", name)
}

// ---------- HTTP ----------

// HandleGetComponentStatus GET /api/components — 全部组件状态 + 在途任务。
func HandleGetComponentStatus(c *gin.Context) {
	list := make([]componentStatus, 0, len(componentCatalog))
	for _, def := range componentCatalog {
		list = append(list, buildComponentStatus(def))
	}
	jobs := gin.H{}
	for _, def := range componentCatalog {
		if j := componentJobSnapshot(def.ID); j != nil {
			jobs[def.ID] = j
		}
	}
	c.JSON(http.StatusOK, gin.H{"components": list, "jobs": jobs})
}

// HandleComponentInstall POST /api/components/:id/install {start:true}
// start=true 触发后台下载；否则只返回任务进度（前端轮询用）。
func HandleComponentInstall(c *gin.Context) {
	id := c.Param("id")
	def, ok := componentDefByID(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "未知组件: " + id})
		return
	}
	var body struct {
		Start bool `json:"start"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.Start {
		st := buildComponentStatus(def)
		if st.Installed || st.OnPath {
			c.JSON(http.StatusOK, gin.H{"job": gin.H{"id": id, "state": "done", "percent": 100}})
			return
		}
		j, _ := startComponentDownload(def)
		c.JSON(http.StatusAccepted, gin.H{"job": j})
		return
	}
	j := componentJobSnapshot(id)
	if j == nil {
		c.JSON(http.StatusOK, gin.H{"job": gin.H{"id": id, "state": "idle"}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"job": j})
}

// ---------- agent 原生工具 ----------

// componentDownloadToolDef component_download 工具定义（按需加载）：
// agent 遇到缺组件的报错时自助安装，不用用户去点侧栏。
var componentDownloadToolDef = core.ToolDefinition{
	Type: "function",
	Function: core.ToolFunctionDetail{
		Name:        "component_download",
		Description: "按需下载可选组件（当前：ffmpeg——视频拼接、抽帧、配音合成、去水印的处理引擎）。当功能报「未找到 ffmpeg / 请点组件下载」时调用本工具安装。不传 id 时返回全部组件清单与状态。",
		Parameters: core.ToolParameters{
			Type: "object",
			Properties: map[string]core.ToolProperty{
				"id": {
					Type:        "string",
					Description: "组件 id（如 ffmpeg）。缺省时仅查询状态。",
				},
				"install": {
					Type:        "boolean",
					Description: "true 触发下载并等待完成（约 100MB，视网速数分钟）；false/缺省只查询状态。",
				},
			},
			Required: []string{},
		},
	},
}

func callComponentDownload(ctx context.Context, argsJSON string) (nativeToolResult, error) {
	var args struct {
		ID      string `json:"id"`
		Install bool   `json:"install"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return nativeToolResult{}, fmt.Errorf("参数解析失败: %w", err)
	}
	if args.ID == "" {
		list := make([]componentStatus, 0, len(componentCatalog))
		for _, def := range componentCatalog {
			list = append(list, buildComponentStatus(def))
		}
		out, _ := json.Marshal(gin.H{"components": list})
		return nativeToolResult{Text: string(out)}, nil
	}
	def, ok := componentDefByID(args.ID)
	if !ok {
		return nativeToolResult{}, fmt.Errorf("未知组件: %s（可用: ffmpeg）", args.ID)
	}
	st := buildComponentStatus(def)
	if st.OnPath || st.Installed {
		out, _ := json.Marshal(gin.H{"ok": true, "state": "ready", "status": st})
		return nativeToolResult{Text: string(out)}, nil
	}
	if !args.Install {
		out, _ := json.Marshal(gin.H{"ok": false, "state": "missing", "hint": "传 install=true 开始下载", "status": st})
		return nativeToolResult{Text: string(out)}, nil
	}
	startComponentDownload(def)
	// 阻塞等待：轮询到 done/error 或超时（下载 100MB 量级，上限 15 分钟）
	deadline := time.Now().Add(15 * time.Minute)
	for {
		cur := componentJobSnapshot(def.ID)
		if cur != nil && (cur.State == "done" || cur.State == "error") {
			out, _ := json.Marshal(gin.H{"ok": cur.State == "done", "state": cur.State, "error": cur.Error, "status": buildComponentStatus(def)})
			return nativeToolResult{Text: string(out)}, nil
		}
		if time.Now().After(deadline) {
			return nativeToolResult{Text: `{"ok":false,"state":"timeout","hint":"下载仍在后台进行，稍后再查 component_download 状态"}`}, nil
		}
		select {
		case <-ctx.Done():
			return nativeToolResult{Text: `{"ok":false,"state":"canceled","hint":"下载任务仍在后台继续，不会被取消"}`}, nil
		case <-time.After(2 * time.Second):
		}
	}
}
