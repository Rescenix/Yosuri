package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// runDemoDelivery builds a real, previewable project with the same hard gate
// used by autonomous production. It is intentionally a visible demo project,
// not a database flag or a synthetic audit response.
func runDemoDelivery() {
	runDirectiveDelivery("全链路演示舱", "面向公开演示的多 Agent 生产中控台。研究部提交可复算 Excel，设计部提交响应式原型，程序部提交可运行程序，宣传部同时提交 PowerPoint 与 MP4，发布部留下可以反查入口哈希的真实本地预览回执。", "")
}

func parseDirectiveDeliveryArgs(args []string) (model, directive string) {
	var words []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			words = append(words, args[i+1:]...)
			break
		}
		if args[i] == "--model" && i+1 < len(args) {
			model = strings.TrimSpace(args[i+1])
			i++
			continue
		}
		words = append(words, args[i])
	}
	return model, strings.TrimSpace(strings.Join(words, " "))
}

func normalizeDirectiveModel(model string) string {
	lower := strings.ToLower(strings.TrimSpace(model))
	if strings.Contains(lower, "deepseek-v4-pro") || lower == "deepseek_v4_pro" {
		return "deepseek_v4_pro"
	}
	return strings.TrimSpace(model)
}

func enrichDirectiveBrief(model, title, brief string) (string, error) {
	model = normalizeDirectiveModel(model)
	if model == "" {
		return brief, nil
	}
	InitRouter()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	prompt := "你是公司立项策划 Agent。请围绕用户指令给出一份简洁、可执行的项目策划，必须包含目标用户、核心功能、视觉方向、验收标准和传播卖点。不要寒暄，不要虚构已经完成的功能。\n\n用户指令：" + title
	plan, err := CompleteWithModel(ctx, model, ChatRequest{
		Messages:  []ChatMessage{{Role: "user", Content: prompt}},
		MaxTokens: 900, Temperature: 0.2,
	}, nil)
	if err != nil {
		return "", fmt.Errorf("指定模型 %s 策划失败（禁止自动切换）: %w", model, err)
	}
	return brief + "\n\n## 指定模型策划\n\n" + strings.TrimSpace(plan), nil
}

// runDirectiveDelivery turns a user directive into a concrete, auditable
// project immediately. It is the deterministic bridge used by the Company UI;
// the always-on agents may continue iterating later, but the first delivery no
// longer depends on waiting for a random autonomous cycle.
func runDirectiveDelivery(title, brief, model string) {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("❌ 无法定位公司目录：%v\n", err)
		return
	}
	title = strings.TrimSpace(title)
	brief = strings.TrimSpace(brief)
	if title == "" {
		title = "用户指令项目"
	}
	if brief == "" {
		brief = title
	}
	brief, err = enrichDirectiveBrief(model, title, brief)
	if err != nil {
		fmt.Printf("❌ %v\n", err)
		return
	}
	if model != "" {
		fmt.Printf("✅ MODEL=%s\n", normalizeDirectiveModel(model))
	}
	project := "001-" + sanitizeFilename(title) + "-" + time.Now().Format("0102-150405")
	company := filepath.Join(home, "rescene_data", "company")
	primary := filepath.Join(company, "coder-03", "projects", project)
	if err := os.MkdirAll(primary, 0o755); err != nil {
		fmt.Printf("❌ 无法创建演示项目：%v\n", err)
		return
	}
	if _, err := writeProjectFile(primary, "00-需求计划.md", "# "+title+"\n\n## 用户指令\n\n"+brief+"\n\n## 验收标准\n\n- 真实多部门分工\n- 所有非文本产物可在前端预览\n- 可运行程序必须具有真实交互\n- PPT 页面必须用于宣传视频画面\n- 缺少任一强制阶段不得进入项目审批\n- 发布回执必须绑定可运行入口的 SHA-256\n"); err != nil {
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
