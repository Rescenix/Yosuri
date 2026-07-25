package handler

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
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

// EnsureLocalLlamaServer 按需启动本地 llama-server（异步、非阻塞）。
//
// 设计目标（见 issue：后端每次启动都拉起 llama 吃光 2G 内存、且杀死主进程后
// llama 子进程变孤儿继续跑）：
//  1. 主服务启动阶段【不再】无条件拉起 llama —— 只有真正选了某个 IsLocal 的
//     识图模型（如 local_llama_qwen2_5_vl_7b）发起识图请求时，才在这里按需拉起。
//  2. 启动失败只打日志、不阻塞调用方（调用方会回退到云端 Gemini 识图）。
//  3. 通过进程组（Setpgid）+ 信号处理，保证主进程退出时整个 llama 进程组一起退出，
//     不再产生孤儿进程。
//
// 调用方（analyzeImageWithModelID）在命中本地模型时先调本函数，再走推理。
func EnsureLocalLlamaServer() {
	cfg := loadLocalLlamaConfig()

	if _, err := os.Stat(cfg.ModelPath); err != nil {
		log.Printf("⚠️ 本地 llama 模型未找到 (%s)，无法启动本地识图服务。可通过 LLAMA_MODEL_PATH 指定路径，或改用云端识图模型。", cfg.ModelPath)
		return
	}
	if _, err := exec.LookPath(cfg.BinPath); err != nil {
		log.Printf("⚠️ llama-server 可执行文件未找到 (%s)，无法启动本地识图服务。请安装 llama.cpp 并加入 PATH，或通过 LLAMA_SERVER_BIN 指定。", cfg.BinPath)
		return
	}

	llamaServerMu.Lock()
	// 已在运行 / 正在启动，直接返回（runOnce 防止并发重复拉起）
	if llamaServerCmd != nil {
		llamaServerMu.Unlock()
		return
	}
	llamaServerMu.Unlock()

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

	log.Printf("🦙 按需启动本地 llama-server: %s %s", cfg.BinPath, strings.Join(args, " "))
	cmd := exec.Command(cfg.BinPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// 非 Windows 下建独立进程组：主进程退出时统一 kill 整个组，避免 llama 变孤儿继续占内存。
	// Windows 无进程组概念，改由 main 的退出信号处理里显式调用 StopLocalLlamaServer() 回收。
	ensureLlamaProcessGroup(cmd)

	llamaServerMu.Lock()
	llamaServerCmd = cmd
	llamaServerURL = fmt.Sprintf("http://%s:%d/v1", cfg.Host, cfg.Port)
	llamaServerMu.Unlock()

	if err := cmd.Start(); err != nil {
		log.Printf("⚠️ 启动 llama-server 失败: %v", err)
		llamaServerMu.Lock()
		llamaServerCmd = nil
		llamaServerURL = ""
		llamaServerMu.Unlock()
		return
	}

	// 异步等待就绪，不阻塞调用方（调用方自己带超时去打 llama 的 /v1/chat/completions）。
	go func() {
		if err := waitForLlamaServer(120 * time.Second); err != nil {
			log.Printf("⚠️ 本地 llama-server 在超时内未就绪: %v（识图将回退云端）", err)
			llamaServerMu.Lock()
			if llamaServerCmd == cmd {
				StopLocalLlamaServer()
			}
			llamaServerMu.Unlock()
			return
		}
		log.Printf("🦙 本地 llama-server 已就绪: %s (n_gpu_layers=%d)", llamaServerURL, cfg.NGPULayers)
		_ = cmd.Wait()
		llamaServerMu.Lock()
		if llamaServerCmd == cmd {
			llamaServerCmd = nil
			llamaServerURL = ""
		}
		llamaServerMu.Unlock()
		log.Println("🦙 本地 llama-server 已退出")
	}()
}

// IsLocalLlamaModel 判断某个模型 ID 是否走本地 llama-server（即需要本地识图服务）。
// 仅当该模型在免费池/用户配置中标记为 Local=true 时返回 true。
func IsLocalLlamaModel(modelID string) bool {
	if modelID == "" {
		return false
	}
	b := resolveExact("", modelID)
	if b == nil {
		return false
	}
	return b.IsLocal
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

// StopLocalLlamaServer 停止本地 llama-server 进程，并清空状态。
// 幂等：未运行时直接返回 nil。
// 进程组级 kill（连孙子进程一起）走非 Windows 的 llama_local_posix.go，
// 这里 Windows 下直接 Kill 子进程，并由 main 的退出信号统一触发。
func StopLocalLlamaServer() error {
	llamaServerMu.Lock()
	defer llamaServerMu.Unlock()
	if llamaServerCmd == nil || llamaServerCmd.Process == nil {
		return nil
	}
	cmd := llamaServerCmd
	llamaServerCmd = nil
	llamaServerURL = ""
	return stopLlamaProcess(cmd)
}

// RegisterLlamaCleanupOnExit 注册退出信号监听：主进程收到 SIGINT/SIGTERM 时，
// 显式停掉本地 llama-server，避免子进程变孤儿继续占内存（Windows 下尤其依赖此路径）。
func RegisterLlamaCleanupOnExit() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("🦙 收到退出信号，正在停止本地 llama-server（如有）...")
		_ = StopLocalLlamaServer()
		os.Exit(0)
	}()
}

// LocalLlamaServerURL 返回本地 llama-server 的 OpenAI 兼容端点；未启动时返回空字符串。
func LocalLlamaServerURL() string {
	return llamaServerURL
}
