package core

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// RunCodeInSandbox 在 Docker 沙箱中执行代码，返回 stdout 和 stderr
func RunCodeInSandbox(language string, code string) (string, error) {
	var image string
	var cmd []string

	switch language {
	case "python", "py":
		image = "python:3.11-alpine"
		cmd = []string{"python", "-c", code}
	case "go":
		image = "golang:1.22-alpine"
		cmd = []string{"sh", "-c", fmt.Sprintf("echo '%s' > /tmp/main.go && go run /tmp/main.go", code)}
	case "javascript", "js":
		image = "node:20-alpine"
		cmd = []string{"node", "-e", code}
	case "c":
		image = "gcc:latest"
		cmd = []string{"sh", "-c", fmt.Sprintf("cat > /tmp/main.c << 'EOF'\n%s\nEOF\ngcc -o /tmp/a.out /tmp/main.c && /tmp/a.out", code)}
	default:
		return "", fmt.Errorf("不支持的语言: %s", language)
	}

	// 构建 docker run 命令
	args := []string{
		"run", "--rm",
		"--net=none",                       // 禁止网络
		"--memory=256m",                    // 内存限制
		"--cpus=0.5",                       // CPU 限制
		"--tmpfs", "/tmp:rw,exec,size=64m", // 可写可执行的临时目录
		"--stop-timeout", "5", // 超时 5 秒
		image,
	}
	args = append(args, cmd...)

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	execCmd := exec.CommandContext(ctx, "docker", args...)
	output, err := execCmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("代码执行超时（5秒限制）")
	}

	if err != nil {
		return string(output), fmt.Errorf("执行错误: %v", err)
	}

	return string(output), nil
}
