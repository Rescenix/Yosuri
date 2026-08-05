package main

// workdir.go — 她的工作目录：隔离工作区 → 修改项目 → 报告 → 审批 → 落实
// 她在 ~/rescene_data/workdir/ 里工作，完成后你审批才生效

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func workDir() string {
	home := daughterHome()
	return filepath.Join(filepath.Dir(home), "workdir")
}

func ensureWorkDir() error {
	return os.MkdirAll(workDir(), 0755)
}

// InitProject 在工作目录初始化一个项目（git clone 或新建目录）
func InitProject(name, repoURL string) error {
	if err := ensureWorkDir(); err != nil {
		return err
	}
	projDir := filepath.Join(workDir(), name)
	if _, err := os.Stat(projDir); err == nil {
		return fmt.Errorf("项目 %s 已存在", name)
	}
	if repoURL != "" {
		return runCmd("git", "clone", repoURL, projDir)
	}
	return os.MkdirAll(projDir, 0755)
}

// GenerateReport 生成项目变更报告（diff + 摘要）
func GenerateReport(name string) (string, error) {
	projDir := filepath.Join(workDir(), name)
	reportDir := filepath.Join(projDir, ".rescene_reports")
	os.MkdirAll(reportDir, 0755)
	ts := time.Now().Format("20060102-150405")
	reportPath := filepath.Join(reportDir, "report-"+ts+".md")

	// git diff 如果存在
	diff := ""
	if _, err := os.Stat(filepath.Join(projDir, ".git")); err == nil {
		out, _ := runCmdOutput("git", "-C", projDir, "diff", "--stat")
		diff = out
	}
	report := fmt.Sprintf("# 审批报告 · %s\n\n时间：%s\n\n## 变更摘要\n\n%s\n\n---\n\n## 变更详情\n\n%s\n\n---\n\n## 审批\n\n- [ ] 通过（落实更改）\n- [ ] 拒绝（保留工作区）\n", name, ts, diff, "（请运行 git diff 查看完整变更）")
	os.WriteFile(reportPath, []byte(report), 0644)
	return reportPath, nil
}

// ApplyChanges 落实更改：从工作目录应用到真实路径
func ApplyChanges(name, targetDir string) error {
	projDir := filepath.Join(workDir(), name)
	if _, err := os.Stat(projDir); err != nil {
		return fmt.Errorf("项目 %s 不存在", name)
	}
	// 简单实现：复制整个目录（可用 rsync 或 git push）
	return runCmd("cp", "-r", projDir+"/.", targetDir+"/")
}

func runCmd(name string, args ...string) error {
	return runCmdContext(name, args...)
}

func runCmdContext(name string, args ...string) error {
	_, err := runCmdOutput(name, args...)
	return err
}

func runCmdOutput(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}