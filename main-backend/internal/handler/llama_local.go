package handler

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// LocalLlamaConfig 本地 llama-server 启动配置。
// 全部通过环境变量读取，方便不同机器独立配置而不污染代码仓库。
type LocalLlamaConfig struct {
	BinPath     string // llama-server 可执行文件路径
	ModelPath   string // gguf 模型路径
	MmprojPath  string // vision projector 路径（多模态模型需要，可选）
	Port        int    // 服务端口
	NGPULayers  int    // offload 到 GPU 的层数
	ContextSize int    // 上下文长度
	Host        string // 监听地址
}

var (
	llamaServerCmd *exec.Cmd
	llamaServerMu  sync.Mutex
	llamaServerURL string
)

func loadLocalLlamaConfig() LocalLlamaConfig {
	bin := os.Getenv("LLAMA_SERVER_BIN")
	if bin == "" {
		if runtime.GOOS == "windows" {
			bin = "llama-server.exe"
		} else {
			bin = "llama-server"
		}
	}

	model := os.Getenv("LLAMA_MODEL_PATH")
	if model == "" {
		// 默认指向项目内 main-backend/models 目录下的 Qwen2.5-VL
		model = "models/Qwen2.5-VL-7B-Instruct-Q4_K_M.gguf"
	}
	// 统一用反斜杠：Windows 下 llama-server 对路径分隔符不敏感，但日志更清爽。
	model = filepath.Clean(model)
	if !filepath.IsAbs(model) {
		cwd, err := os.Getwd()
		if err == nil {
			model = filepath.Join(cwd, model)
		}
	}

	port := 8081
	if v := os.Getenv("LLAMA_SERVER_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			port = p
		}
	}

	// 999 表示"能放多少放多少"，是 llama.cpp 社区常见的"全部 GPU 层"写法。
	ngl := 999
	if v := os.Getenv("LLAMA_N_GPU_LAYERS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			ngl = n
		}
	}

	ctx := 32768
	if v := os.Getenv("LLAMA_CONTEXT_SIZE"); v != "" {
		if c, err := strconv.Atoi(v); err == nil && c > 0 {
			ctx = c
		}
	}

	host := os.Getenv("LLAMA_SERVER_HOST")
	if host == "" {
		host = "127.0.0.1"
	}

	return LocalLlamaConfig{
		BinPath:     bin,
		ModelPath:   model,
		MmprojPath:  os.Getenv("LLAMA_MMPROJ_PATH"),
		Port:        port,
		NGPULayers:  ngl,
		ContextSize: ctx,
		Host:        host,
	}
}

// StartLocalLlamaServer 尝试启动本地 llama-server。
// 如果模型文件或可执行文件缺失，则只打印警告并返回 nil，不影响主服务启动。
func StartLocalLlamaServer() error {
	cfg := loadLocalLlamaConfig()

	if _, err := os.Stat(cfg.ModelPath); err != nil {
		log.Printf("⚠️ 本地 llama 模型未找到 (%s)，跳过启动本地服务。可通过 LLAMA_MODEL_PATH 指定路径。", cfg.ModelPath)
		return nil
	}

	if _, err := exec.LookPath(cfg.BinPath); err != nil {
		log.Printf("⚠️ llama-server 可执行文件未找到 (%s)，跳过启动本地服务。请安装 llama.cpp 并加入 PATH，或通过 LLAMA_SERVER_BIN 指定。", cfg.BinPath)
		return nil
	}

	llamaServerMu.Lock()
	defer llamaServerMu.Unlock()

	if llamaServerCmd != nil {
		return nil
	}

	args := []string{
		"--model", cfg.ModelPath,
		"--port", strconv.Itoa(cfg.Port),
		"--host", cfg.Host,
		"--n-gpu-layers", strconv.Itoa(cfg.NGPULayers),
		"--ctx-size", strconv.Itoa(cfg.ContextSize),
	}
	if cfg.MmprojPath != "" {
		args = append(args, "--mmproj", cfg.MmprojPath)
	}

	log.Printf("🦙 启动本地 llama-server: %s %s", cfg.BinPath, strings.Join(args, " "))
	cmd := exec.Command(cfg.BinPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 llama-server 失败: %w", err)
	}

	llamaServerCmd = cmd
	llamaServerURL = fmt.Sprintf("http://%s:%d/v1", cfg.Host, cfg.Port)

	if err := waitForLlamaServer(60 * time.Second); err != nil {
		_ = cmd.Process.Kill()
		llamaServerCmd = nil
		llamaServerURL = ""
		return err
	}

	log.Printf("🦙 本地 llama-server 已就绪: %s (n_gpu_layers=%d)", llamaServerURL, cfg.NGPULayers)

	// 守护 goroutine：进程意外退出时打印日志并清理状态。
	go func() {
		_ = cmd.Wait()
		llamaServerMu.Lock()
		llamaServerCmd = nil
		llamaServerMu.Unlock()
		log.Println("🦙 本地 llama-server 已退出")
	}()

	return nil
}

func waitForLlamaServer(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 1 * time.Second}
	healthURL := strings.TrimSuffix(llamaServerURL, "/v1") + "/health"
	for time.Now().Before(deadline) {
		resp, err := client.Get(healthURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("llama-server 在 %v 内未就绪", timeout)
}

// StopLocalLlamaServer 停止本地 llama-server 进程。
func StopLocalLlamaServer() error {
	llamaServerMu.Lock()
	defer llamaServerMu.Unlock()
	if llamaServerCmd == nil || llamaServerCmd.Process == nil {
		return nil
	}
	return llamaServerCmd.Process.Kill()
}

// LocalLlamaServerURL 返回本地 llama-server 的 OpenAI 兼容端点；未启动时返回空字符串。
func LocalLlamaServerURL() string {
	return llamaServerURL
}
