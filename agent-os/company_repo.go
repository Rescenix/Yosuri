package main

// company_repo.go — 公司代码仓库：AI 公司产出自动 git 管理 + GitHub PR
//
// 用户：「安排GitHub仓库，pr」——公司 coder 的代码产出自动进 Git 仓库，
// 分 feature 分支提交，用 gh CLI 开 PR 合入 main，像真实公司协作。
//
// 用法：
//   rescene company-repo init    初始化公司仓库（本地 git + GitHub 远程）
//   rescene company-repo push    推送全部产出到 GitHub
//   rescene company-repo pr      开 PR（feature 分支 → main）
// 自动：coder 的 project 产出完成后自动 commit（分 agent 分支）

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// companyRepoPath 公司代码仓库
func companyRepoPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "rescene_data", "company-repo")
}

// runGit 在公司仓库执行 git 命令
func runGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = companyRepoPath()
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// runGh 执行 gh 命令
func runGh(args ...string) (string, error) {
	cmd := exec.Command("gh", args...)
	cmd.Dir = companyRepoPath()
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// ensureCompanyRepo 初始化公司仓库（git init + 基本配置）
func ensureCompanyRepo() error {
	dir := companyRepoPath()
	os.MkdirAll(dir, 0o755)
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		if out, err := runGit("init", "-b", "main"); err != nil {
			return fmt.Errorf("git init: %s", out)
		}
		runGit("config", "user.name", "Rescene AI 公司")
		runGit("config", "user.email", "rescene-ai@rescene.dev")
	}
	// 仓库说明文件
	readme := filepath.Join(dir, "README.md")
	if _, err := os.Stat(readme); err != nil {
		os.WriteFile(readme, []byte("# Rescene AI 公司代码仓库\n\nAI 公司里程序部的产出，自动提交，PR 合入。\n"), 0o644)
	}
	return nil
}

// companyCommit 提交一个 agent 的产出到公司仓库（分 agent 分支）
// agentName: coder-03；branch: feature/coder-03/项目名
func companyCommit(agentName, projectName string, files []string) error {
	if err := ensureCompanyRepo(); err != nil {
		return err
	}
	// 建 feature 分支
	branch := "feature/" + agentName
	if projectName != "" {
		branch += "/" + sanitizeFilename(projectName)
	}
	runGit("checkout", "main")
	runGit("checkout", "-b", branch) // 已存在则忽略错误

	// 复制产出文件到仓库（agents/<name>/<project>/）
	destDir := filepath.Join(companyRepoPath(), "agents", agentName, sanitizeFilename(projectName))
	os.MkdirAll(destDir, 0o755)
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err == nil {
			os.WriteFile(filepath.Join(destDir, filepath.Base(f)), data, 0o644)
		}
	}

	// commit
	runGit("add", ".")
	msg := fmt.Sprintf("[%s] %s: %s 产出 %d 个文件", time.Now().Format("01-02 15:04"), agentName, projectName, len(files))
	if out, err := runGit("commit", "-m", msg); err != nil && !strings.Contains(out, "nothing to commit") {
		return fmt.Errorf("commit: %s", out)
	}
	// 推送远程（如果配置了）
	runGit("push", "-u", "origin", branch)
	return nil
}

// runCompanyRepo 公司仓库子命令
func runCompanyRepo(args []string) {
	if len(args) == 0 {
		fmt.Println("用法: rescene company-repo [init|push|pr]")
		return
	}
	switch args[0] {
	case "init":
		if err := ensureCompanyRepo(); err != nil {
			fmt.Printf("❌ %v\n", err)
			return
		}
		fmt.Printf("✅ 公司仓库已初始化: %s\n", companyRepoPath())
		// 先提交 README（gh repo create --push 需要至少一个 commit）
		runGit("add", ".")
		runGit("commit", "-m", "chore: 公司仓库初始化")
		// 创建 GitHub 远程并推送
		if out, err := runGh("repo", "create", "Rescene-Company", "--private", "--source=.", "--push"); err != nil {
			fmt.Printf("ℹ️ GitHub 远程（可稍后手动 gh repo create Rescene-Company --source=. --push）: %s\n", out)
		} else {
			fmt.Printf("✅ GitHub 仓库已创建并推送: %s\n", out)
		}

	case "push":
		runGit("add", ".")
		runGit("commit", "-m", "公司产出自动提交")
		if out, err := runGit("push", "origin", "main"); err != nil {
			fmt.Printf("⚠️ 推送: %s\n", out)
		} else {
			fmt.Println("✅ 已推送到 GitHub main")
		}

	case "pr":
		// 列出所有 feature 分支并开 PR
		branches, _ := runGit("branch", "--list", "feature/*")
		for _, b := range strings.Split(branches, "\n") {
			b = strings.TrimSpace(strings.TrimPrefix(b, "* "))
			if b == "" {
				continue
			}
			if out, err := runGh("pr", "create", "--base", "main", "--head", b,
				"--title", "AI 公司产出合入: "+b, "--body", "由 Rescene AI 公司程序部自动产出，审核后合入。"); err != nil {
				fmt.Printf("  ⚠️ %s PR: %s\n", b, out)
			} else {
				fmt.Printf("  ✅ %s PR 已开: %s\n", b, out)
			}
		}
		fmt.Println("完成")

	default:
		fmt.Println("用法: rescene company-repo [init|push|pr]")
	}
}
