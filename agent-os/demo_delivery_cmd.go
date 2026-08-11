package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// runDemoDelivery builds a real, previewable project with the same hard gate
// used by autonomous production. It is intentionally a visible demo project,
// not a database flag or a synthetic audit response.
func runDemoDelivery() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("❌ 无法定位公司目录：%v\n", err)
		return
	}
	project := "900-全链路演示舱-" + time.Now().Format("0102-150405")
	company := filepath.Join(home, "rescene_data", "company")
	primary := filepath.Join(company, "coder-03", "projects", project)
	if err := os.MkdirAll(primary, 0o755); err != nil {
		fmt.Printf("❌ 无法创建演示项目：%v\n", err)
		return
	}
	brief := "面向公开演示的多 Agent 生产中控台。研究部提交可复算 Excel，设计部提交响应式原型，程序部提交可运行程序，宣传部同时提交 PowerPoint 与 MP4，发布部留下可以反查入口哈希的真实本地预览回执。"
	if _, err := writeProjectFile(primary, "00-需求计划.md", "# 全链路演示需求\n\n- 展示真实多部门分工\n- 所有非文本产物可在前端预览\n- 缺少任一强制阶段不得进入项目审批\n- 发布回执必须绑定可运行入口的 SHA-256\n"); err != nil {
		fmt.Printf("❌ 需求落盘失败：%v\n", err)
		return
	}
	manifest, err := enforceProjectDelivery(&Daughter{Name: "coder-03", Role: "coder"}, primary, project, brief)
	if err != nil {
		fmt.Printf("❌ 硬门槛未通过：%v\n", err)
		return
	}

	// A project approval is aggregated from multiple participating agents. The
	// second checkout represents the design participant's synchronized copy.
	secondary := filepath.Join(company, "designer-04", "projects", project)
	if err := copyDemoProject(primary, secondary); err != nil {
		fmt.Printf("❌ 多 Agent 项目同步失败：%v\n", err)
		return
	}
	fmt.Printf("✅ DEMO_PROJECT=%s\n", project)
	fmt.Printf("✅ STATUS=%s EVIDENCE=%d/11\n", manifest.Status, len(manifest.Evidence))
	fmt.Printf("✅ PRIMARY=%s\n", primary)
	fmt.Printf("✅ PARTICIPANT=%s\n", secondary)
}

func copyDemoProject(source, target string) error {
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		from, err := os.Open(filepath.Join(source, entry.Name()))
		if err != nil {
			return err
		}
		to, err := os.Create(filepath.Join(target, entry.Name()))
		if err != nil {
			from.Close()
			return err
		}
		_, copyErr := io.Copy(to, from)
		closeErr := to.Close()
		from.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}
