package handler

import (
	"testing"

	"backend/internal/ai/core"
)

// TestIsSensitiveOverwrite 验证 YOLO 模式下「敏感文件整体覆写强制拦截」的判定边界：
// 已存在的 README/依赖清单/.env 被 write_file/apply_patch 覆盖必须返回 true，
// 新建敏感文件、定向 edit_file、普通文件覆写返回 false。
func TestIsSensitiveOverwrite(t *testing.T) {
	// 固定 project root 为真实仓库：agentfs_test 等会全局 SetProjectRoot 到临时目录
	// 且不还原，全包跑时相对路径会解析到 temp 里导致存在性误判。
	realRoot := "C:/Pro2026/re0"
	oldRoot := core.GetProjectRoot()
	if err := core.SetProjectRoot(realRoot); err != nil {
		t.Fatalf("SetProjectRoot: %v", err)
	}
	t.Cleanup(func() { core.SetProjectRoot(oldRoot) })

	// 必须拦截：已存在敏感文件被整文件覆写（README 惨案场景）
	mustBlock := []struct {
		name string
		args string
	}{
		{"write_file", `{"path": "README.md", "content": "# 项目初始化"}`},
		{"write_file", `{"path": "README.zh-CN.md", "content": "x"}`},
		{"write_file", `{"path": ".gitignore", "content": "x"}`},
		{"write_file", `{"path": "main-backend/go.mod", "content": "module x"}`},
		{"write_file", `{"path": "main-backend/go.sum", "content": "x"}`},
		{"write_file", `{"path": "main-frontend/beneficial-belt/package.json", "content": "{}"}`},
		{"write_file", `{"path": "C:/Pro2026/re0/README.md", "content": "# 模板"}`},
		// 09-06 工具面收敛后的核心写工具名：闸门必须同样认，漏了等于 README 惨案重演
		{"write", `{"path": "README.md", "content": "# 项目初始化"}`},
		{"write", `{"path": "main-backend/go.mod", "content": "module x"}`},
		{"write", `{"path": "main-frontend/beneficial-belt/package.json", "content": "{}"}`},
		{"mcp__fs__write_file", `{"path": "README.md", "content": "x"}`},
		// apply_patch 的路径藏在 patch 头里
		{"apply_patch", "*** Begin Patch\n*** Update File: README.md\n@@\n-# 旧内容\n+# 新内容\n*** End Patch"},
	}
	for _, c := range mustBlock {
		if !isSensitiveOverwrite(c.name, c.args) {
			t.Errorf("%s %s 覆写已存在的敏感文件，YOLO 下应强制拦截（isSensitiveOverwrite 应为 true）", c.name, c.args)
		}
	}

	// 不应拦截：新建敏感文件 / 定向行级编辑 / 普通文件覆写 / 非写工具
	shouldPass := []struct {
		name string
		args string
	}{
		// 不存在的 README 路径（新建不拦）：用带随机后缀的不存在路径
		{"write_file", `{"path": "C:/Pro2026/re0/__no_such_dir__/README.md", "content": "x"}`},
		// 定向编辑不在敏感覆写集合里（改动小、可还原）
		{"edit_file", `{"path": "README.md", "old_string": "a", "new_string": "b"}`},
		{"mcp__fs__edit_file", `{"path": "README.md"}`},
		// 普通文件覆写不拦
		{"write_file", `{"path": "main-backend/internal/handler/chat.go", "content": "x"}`},
		{"write_file", `{"path": "src/App.vue", "content": "x"}`},
		// 非写工具不归这里管
		{"read_file", `{"path": "README.md"}`},
		{"run_command", `{"command": "ls -la"}`},
	}
	for _, c := range shouldPass {
		if isSensitiveOverwrite(c.name, c.args) {
			t.Errorf("%s %s 不该被 isSensitiveOverwrite 判定为敏感覆写，实际返回 true", c.name, c.args)
		}
	}

	// isSensitiveFile 单独判定（不依赖文件存在性）：密钥/凭据类必须命中
	for _, p := range []string{".env", ".env.local", ".env.production", "keys/server.pem", "cert.p12", "id_rsa.key"} {
		if !isSensitiveFile(p) {
			t.Errorf("路径 %s 是密钥/凭据类敏感文件，isSensitiveFile 应为 true", p)
		}
	}
}

// TestIsDestructiveCommand 验证 YOLO 模式下「破坏性 shell 命令强制拦截」的判定边界：
// git checkout -- / restore / reset --hard / clean / rm -rf / 强推必须返回 true，
// 只读 git 查询与普通命令返回 false。
func TestIsDestructiveCommand(t *testing.T) {
	// 必须拦截的破坏性命令
	mustBlock := []string{
		"git checkout -- README.md",
		"git checkout -- .",
		"git checkout .",
		"git restore README.md",
		"git restore .",
		"git reset --hard HEAD",
		"git clean -fd",
		"git rm -r node_modules",
		"cd /c/Pro2026/re0 && git checkout -- main-backend",
		"GIT CHECKOUT -- README.MD", // 大小写不敏感
		"rm -rf node_modules",
		"rm -r build/",
		"rm -f main.go",
		// 09-06 补：裸 rm / del / Remove-Item 不带任何标志，删单个文件最常见，
		// 旧正则要求带 -r/-f 全部漏网——这是 agent 删文件不弹条的直接原因。
		"rm main.go",
		"rm README.md",
		"rm a.txt b.txt",
		"rm \"C:/Pro2026/re0/x.go\"",
		"rm *.tmp",
		"rm $HOME/x",
		"del main.go",
		"erase build.txt",
		"Remove-Item main.go",
		"Remove-Item .\\build",
		"cd x && rm -rf dist",
		"Remove-Item -Recurse -Force .\\build",
		"rd /s /q build",
		"rmdir /s build",
		"del /f /s /q *.tmp",
		"git push -f origin main",
		"git push --force origin main",
	}
	for _, cmd := range mustBlock {
		if !isDestructiveCommand(cmd) {
			t.Errorf("命令 %q 是破坏性操作，YOLO 下应强制拦截（isDestructiveCommand 应为 true）", cmd)
		}
	}

	// 不应拦截：只读 git / 普通命令 / 无害的 git checkout（切换分支）
	shouldPass := []string{
		"git status",
		"git diff",
		"git log --oneline -5",
		"git branch --show-current",
		"git checkout feature-branch", // 切分支，不是恢复工作区
		"git checkout -b new-feature",
		"git reset --mixed HEAD", // mixed 不动工作区
		"ls -la",
		"go build ./...",
		"cd /c/Pro2026/re0 && go test ./internal/handler/",
		"node --version",
		"npm run build",
	}
	for _, cmd := range shouldPass {
		if isDestructiveCommand(cmd) {
			t.Errorf("命令 %q 是只读/无害操作，不该被 isDestructiveCommand 判定为破坏性，实际返回 true", cmd)
		}
	}
}

// TestIsIrreversibleToolCall 验证删除/移动类调用的判定：remove 工具（09-06 起
// 独立成名）、旧 write(action=delete/move) 兼容路径、legacy delete_file/move_file
// 都必须返回 true；普通 write 返回 false。
func TestIsIrreversibleToolCall(t *testing.T) {
	mustBlock := []struct {
		name string
		args string
	}{
		{"remove", `{"path": "a.txt"}`},
		{"remove", `{"path": "build", "action": "delete"}`},
		{"remove", `{"path": "b.txt", "action": "move", "source": "a.txt"}`},
		{"remove", `{"path": "a.txt", "action": "DELETE"}`},
		// 旧调用形态：删除曾塞在 write 的 action 里，闸门不能因改名漏网
		{"write", `{"path": "a.txt", "action": "delete"}`},
		{"write", `{"path": "b.txt", "action": "move", "source": "a.txt"}`},
		{"delete_file", `{"path": "a.txt"}`},
		{"delete_directory", `{"path": "build"}`},
		{"move_file", `{"source": "a", "destination": "b"}`},
		{"mcp__fs__delete_file", `{"path": "a.txt"}`},
		{"mcp__fs__rename", `{"path": "a.txt"}`},
	}
	for _, c := range mustBlock {
		if !isIrreversibleToolCall(c.name, c.args) {
			t.Errorf("%s %s 是不可逆操作，YOLO 下应强制拦截（isIrreversibleToolCall 应为 true）", c.name, c.args)
		}
	}
	shouldPass := []struct {
		name string
		args string
	}{
		{"write", `{"path": "a.txt", "content": "x"}`},
		{"write", `{"path": "d", "action": "create_dir"}`},
		{"write", `{"path": "a.txt"}`}, // action 缺省 = 普通写入
		{"patch", `{"path": "a.txt", "old_string": "x", "new_string": "y"}`},
		{"read", `{"path": "a.txt"}`},
		{"write", `{invalid json`}, // 解析失败不误判（Ask 模式 write 本身仍危险）
	}
	for _, c := range shouldPass {
		if isIrreversibleToolCall(c.name, c.args) {
			t.Errorf("%s %s 不是不可逆操作，不该被判定为 true", c.name, c.args)
		}
	}
}

// TestApprovalWaiterUnknownIDDenies 锁死方向：审批 id 查不到时必须拒绝。
// 曾经的「防御性 return true」等于让伪造/错位 id 旁路整个审批闸门。
func TestApprovalWaiterUnknownIDDenies(t *testing.T) {
	w := newApprovalWaiter()
	done := make(chan struct{})
	if w.wait("wf1::never-registered", done) {
		t.Error("未登记的审批 id 应判定为拒绝（false），实际放行")
	}
}

// TestIsDestructiveToolCall 验证工具调用层面的判定：仅 run_command /
// mcp__shell__run 生效，其余工具一律返回 false。
func TestIsDestructiveToolCall(t *testing.T) {
	if !isDestructiveToolCall("run_command", `{"command": "git checkout -- ."}`) {
		t.Error("run_command 含 git checkout -- 应判定为破坏性")
	}
	if !isDestructiveToolCall("mcp__shell__run", `{"command": "rm -rf dist"}`) {
		t.Error("mcp__shell__run 含 rm -rf 应判定为破坏性")
	}
	if isDestructiveToolCall("write_file", `{"command": "git checkout -- .", "path": "README.md"}`) {
		t.Error("write_file 不是 shell 工具，不该被判定为破坏性命令")
	}
	if isDestructiveToolCall("run_command", `{"command": "git status"}`) {
		t.Error("git status 是只读操作，不该被判定为破坏性")
	}
	// 参数损坏时防御性放行（宁可编译不过也不误杀？不——参数解析失败返回 false 即不拦，
	// 但 Ask 模式 run_command 本来就危险，仍有兜底）
	if isDestructiveToolCall("run_command", `{invalid json`) {
		t.Error("参数损坏时不应判定为破坏性（解析失败防御性返回 false）")
	}
}
