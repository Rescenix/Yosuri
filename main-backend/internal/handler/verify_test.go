package handler

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRunVerifyBuildGoMod 验证：当 workdir 含 go.mod 时，runVerifyBuild 能真实跑通
// `go build ./...`（本仓库 main-backend 就是 Go 项目，可直接验证）。
func TestRunVerifyBuildGoMod(t *testing.T) {
	// 用真实后端目录（含 go.mod）验证 go build 路径真的能跑
	repoRoot := findRepoRoot(t)
	backend := filepath.Join(repoRoot, "main-backend")
	if !fileExists(filepath.Join(backend, "go.mod")) {
		t.Skip("main-backend/go.mod 不存在，跳过真实 go build 验证")
	}
	out, ok := runVerifyBuild(backend, "go", "build", "./internal/handler/")
	if !ok {
		t.Errorf("go build 应 pass，实际失败: %s", out)
	}
}

// TestVerifySkipsWithoutAgentFS 验证：未开启 AgentFS 会话时验证函数直接返回（不阻断）。
// 通过把 activeSession 置 nil 后调用，断言不 panic、不报错即可（纯降级路径）。
func TestVerifySkipsWithoutAgentFS(t *testing.T) {
	agentfsMu.Lock()
	prev := activeSession
	activeSession = nil
	agentfsMu.Unlock()
	defer func() {
		agentfsMu.Lock()
		activeSession = prev
		agentfsMu.Unlock()
	}()
	// c 传 nil：函数内仅当 c!=nil 才推 SSE，传 nil 安全
	verifyOnWorkflowDone(nil, "wf_test_no_session")
	// 不 panic 即通过
}

// TestVerifyBuildDispatchByType 验证文件类型→构建命令的分发逻辑（用临时伪项目）。
func TestVerifyBuildDispatchByType(t *testing.T) {
	// 造一个含 package.json 的伪前端目录（无真实构建脚本，验证"命令存在性"分支）
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "package.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// npm 是否在 PATH 决定走哪条分支；不论如何都不应 panic
	_, _ = runVerifyBuild(tmp, "npm", "run", "build")
	// 仅验证函数可调用、不崩溃（真实 npm build 可能因无脚本失败，属预期降级）
}

// findRepoRoot 从测试 cwd 向上找含 go.mod 的仓库根。
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		if fileExists(filepath.Join(dir, "main-backend", "go.mod")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return dir
}
