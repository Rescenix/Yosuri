package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

type departmentAudit struct {
	Role           string   `json:"role"`
	Name           string   `json:"name"`
	Responsibility string   `json:"responsibility"`
	Expected       []string `json:"expected"`
	Evidence       []string `json:"evidence"`
	Passed         bool     `json:"passed"`
	Issue          string   `json:"issue"`
}

func hasAnyExtension(files []string, extensions ...string) bool {
	allowed := map[string]bool{}
	for _, ext := range extensions {
		allowed[ext] = true
	}
	for _, file := range files {
		if allowed[strings.ToLower(filepath.Ext(file))] {
			return true
		}
	}
	return false
}

func auditDepartment(role, name, responsibility string, expected []string, files []string, passed bool, issue string) departmentAudit {
	if passed {
		issue = ""
	}
	evidence := make([]string, 0, 6)
	for _, file := range files {
		ext := strings.ToLower(filepath.Ext(file))
		if ext == ".md" || ext == ".txt" || ext == ".log" {
			continue
		}
		evidence = append(evidence, filepath.Base(file))
		if len(evidence) == 6 {
			break
		}
	}
	return departmentAudit{Role: role, Name: name, Responsibility: responsibility, Expected: expected, Evidence: evidence, Passed: passed, Issue: issue}
}

// HandleCompanyProductionAudit 用磁盘上的真实文件审计部门，不根据角色名、日志话术或文件名前缀判定完成。
func HandleCompanyProductionAudit(c *gin.Context) {
	root := companyDir()
	roleFiles := map[string][]string{}
	// 通过硬门禁的项目按 manifest 中的生产责任归属证据；文件所在的 Agent
	// 目录只是存储位置，不能继续把整包产物错误算给同一个人。
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry == nil || entry.IsDir() || entry.Name() != "delivery.manifest.json" {
			return nil
		}
		projectDir := filepath.Dir(path)
		gate, gateErr := verifyProjectDeliveryGate(projectDir)
		if gateErr != nil {
			return nil
		}
		for _, evidence := range gate.Evidence {
			if evidence.ProducerRole != "" {
				roleFiles[evidence.ProducerRole] = append(roleFiles[evidence.ProducerRole], filepath.Join(projectDir, evidence.File))
			}
		}
		return nil
	})
	counts := map[string]int{"text": 0, "spreadsheet": 0, "presentation": 0, "video": 0, "image": 0, "code": 0, "other": 0}
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry == nil || entry.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) < 2 {
			return nil
		}
		role := strings.SplitN(parts[0], "-", 2)[0]
		roleFiles[role] = append(roleFiles[role], path)
		switch strings.ToLower(filepath.Ext(path)) {
		case ".md", ".txt", ".log":
			counts["text"]++
		case ".xlsx", ".xls", ".csv", ".tsv":
			counts["spreadsheet"]++
		case ".pptx":
			counts["presentation"]++
		case ".mp4", ".webm", ".mov":
			counts["video"]++
		case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg":
			counts["image"]++
		case ".py", ".js", ".ts", ".go", ".java", ".sh", ".html":
			counts["code"]++
		default:
			counts["other"]++
		}
		return nil
	})

	researchFiles := roleFiles["researcher"]
	designFiles := roleFiles["designer"]
	coderFiles := roleFiles["coder"]
	promoterFiles := roleFiles["promoter"]
	publisherFiles := roleFiles["publisher"]
	writerFiles := roleFiles["writer"]
	audits := []departmentAudit{
		auditDepartment("researcher", "研究部", "采集证据、结构化数据、形成可复算结论", []string{"来源清单", ".xlsx/.csv 数据集", "结论与数据核对"}, researchFiles, hasAnyExtension(researchFiles, ".xlsx", ".csv", ".tsv"), "没有可复算的 Excel/CSV 数据交付"),
		auditDepartment("writer", "作者部", "把已验收事实转成可发布正文", []string{"正文", "事实引用", "版本差异"}, writerFiles, hasAnyExtension(writerFiles, ".md", ".docx"), "没有正文交付"),
		auditDepartment("designer", "设计部", "交付可视设计而不是设计描述", []string{"UI 原型", ".png/.svg 视觉稿", "设计令牌"}, designFiles, hasAnyExtension(designFiles, ".html", ".png", ".svg", ".fig"), "只有设计说明，缺少可视原型或设计稿"),
		auditDepartment("coder", "程序部", "编码、运行、测试并留下验证收据", []string{"源代码", "可运行入口", "测试报告"}, coderFiles, hasAnyExtension(coderFiles, ".html", ".exe", ".wasm"), "存在源码，但没有可直接运行入口或端到端测试收据"),
		auditDepartment("promoter", "宣传部", "制作演示型传播物料", []string{".pptx", ".mp4 PV", "封面/海报"}, promoterFiles, hasAnyExtension(promoterFiles, ".pptx") && hasAnyExtension(promoterFiles, ".mp4", ".webm", ".mov"), "缺少 PPTX 或真实视频成品"),
		auditDepartment("publisher", "发布部", "把成品发布到真实渠道并记录结果", []string{"发布 URL", "平台回执", "状态/时间"}, publisherFiles, hasAnyExtension(publisherFiles, ".receipt", ".har"), "没有机器可验证的发布回执"),
	}
	passed := 0
	for _, audit := range audits {
		if audit.Passed {
			passed++
		}
	}
	c.JSON(http.StatusOK, gin.H{"counts": counts, "departments": audits, "passed": passed, "total": len(audits)})
}
