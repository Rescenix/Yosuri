//go:build pipeline

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// 独立流水线工具：产出会议纪要 + 文档 + PV + 可运行项目
// go run -tags pipeline cmd_pipeline.go

func main() {
	home, _ := os.UserHomeDir()
	ceoHome := filepath.Join(home, "rescene_data", "company", "ceo-01")
	os.MkdirAll(filepath.Join(ceoHome, "outputs"), 0o755)

	d := &Daughter{
		Home: ceoHome, MemoryMD: filepath.Join(ceoHome, "memory.md"),
		Journal:     filepath.Join(ceoHome, "journal.md"),
		Stats:       filepath.Join(ceoHome, "stats.json"),
		Personality: loadPersonality(ceoHome),
		World:       loadWorld(ceoHome),
		Role:        "ceo", Name: "ceo-01",
		RolePrompt: "你是公司的CEO，召集例会，写文档，做PV。",
	}

	// 1. 会议纪要
	fmt.Println("🤝 产出证据驱动会议包...")
	if bundle, err := runMeetingBundle(d, ceoHome, "公司周会 - 各部门进度同步与规划"); err != nil {
		fmt.Printf("  ❌ 会议未生成：%v\n", err)
	} else {
		fmt.Printf("  ✅ %d 位发言 · 纪要 %s · PPT %s · 回放 %s\n", len(bundle.Speeches), bundle.MinutesFile, bundle.PPTFile, bundle.ReplayFile)
	}

	// 2. 软件文档
	fmt.Println("📘 产出软件文档...")
	doc := d.modelDoc("Rescene Agent OS — 100 人公司系统")
	if doc != "" {
		fname := fmt.Sprintf("文档-%s-%02d.md", time.Now().Format("2006-01-02"), time.Now().Unix()%100)
		os.WriteFile(filepath.Join(ceoHome, "outputs", fname),
			[]byte(outputMeta("文档")+"# 软件文档\n\n"+doc+"\n"), 0o644)
		fmt.Printf("  ✅ 文档 %s\n", fname)
	}

	// 3. 宣传PV
	fmt.Println("🎬 产出PV脚本...")
	pv := d.modelPV("Rescene 100 人 AI 公司")
	if pv != "" {
		fname := fmt.Sprintf("PV-%s-%02d.md", time.Now().Format("2006-01-02"), time.Now().Unix()%100)
		os.WriteFile(filepath.Join(ceoHome, "outputs", fname),
			[]byte(outputMeta("PV")+"# 宣传PV脚本\n\n"+pv+"\n"), 0o644)
		fmt.Printf("  ✅ PV %s\n", fname)
	}

	// 4. 列出产出
	fmt.Println("\n📂 CEO 产出目录：")
	files, _ := os.ReadDir(filepath.Join(ceoHome, "outputs"))
	for _, f := range files {
		if !f.IsDir() {
			fmt.Println("  📄", f.Name())
		}
	}

	// 5. 检查 coder 可运行项目
	fmt.Println("\n💻 coder 可运行项目：")
	for _, coder := range []string{"coder-27", "coder-03", "coder-21"} {
		projDir := filepath.Join(home, "rescene_data", "company", coder, "projects")
		if projs, err := os.ReadDir(projDir); err == nil {
			for _, p := range projs {
				if p.IsDir() {
					files, _ := os.ReadDir(filepath.Join(projDir, p.Name()))
					for _, f := range files {
						if !f.IsDir() && (strings.HasSuffix(f.Name(), ".py") || strings.HasSuffix(f.Name(), ".js") || strings.HasSuffix(f.Name(), ".go")) {
							fmt.Printf("  ✅ %s/%s/%s\n", coder, p.Name(), f.Name())
						}
					}
				}
			}
		}
	}
	fmt.Println("\n🎉 流水线完成！")
}

func outputMeta(kind string) string {
	return fmt.Sprintf("> 由 CEO 自动产出 · %s · %s\n\n", kind, time.Now().Format("2006-01-02 15:04"))
}
