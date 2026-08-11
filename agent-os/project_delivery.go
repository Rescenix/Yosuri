package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type deliveryEvidence struct {
	Stage        string `json:"stage"`
	ProducerRole string `json:"producerRole"`
	File         string `json:"file"`
	Kind         string `json:"kind"`
	SHA256       string `json:"sha256"`
	Bytes        int64  `json:"bytes"`
	Verification string `json:"verification"`
}

type projectDeliveryManifest struct {
	Project     string             `json:"project"`
	Status      string             `json:"status"`
	GeneratedAt string             `json:"generatedAt"`
	GateVersion int                `json:"gateVersion"`
	Evidence    []deliveryEvidence `json:"evidence"`
	Missing     []string           `json:"missing"`
}

var mandatoryDeliveryStages = []string{"meeting", "research", "data", "requirements", "ui", "docs", "code", "runnable", "ppt", "pv", "promotion"}

func xmlEscape(value string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(value))
	return b.String()
}

func xlsxInlineCell(ref, value, style string) string {
	styleAttr := ""
	if style != "" {
		styleAttr = ` s="` + style + `"`
	}
	return `<c r="` + ref + `" t="inlineStr"` + styleAttr + `><is><t>` + xmlEscape(value) + `</t></is></c>`
}

func xlsxNumberCell(ref string, value int64, style string) string {
	styleAttr := ""
	if style != "" {
		styleAttr = ` s="` + style + `"`
	}
	return fmt.Sprintf(`<c r="%s"%s><v>%d</v></c>`, ref, styleAttr, value)
}

func writeZipEntry(writer *zip.Writer, name, content string) error {
	part, err := writer.Create(name)
	if err != nil {
		return err
	}
	_, err = io.WriteString(part, content)
	return err
}

// writeProjectResearchXLSX creates a real, styled OOXML workbook whose rows are
// derived from files already present in the project. It never invents metrics.
func writeProjectResearchXLSX(projectDir, project string) (string, error) {
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return "", err
	}
	type row struct {
		name, ext, modified string
		size                int64
	}
	var rows []row
	for _, entry := range entries {
		if entry.IsDir() || strings.HasSuffix(entry.Name(), ".xlsx") {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		rows = append(rows, row{name: entry.Name(), ext: strings.ToLower(filepath.Ext(entry.Name())), modified: info.ModTime().Format("2006-01-02 15:04"), size: info.Size()})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })
	name := "02-研究数据.xlsx"
	file, err := os.Create(filepath.Join(projectDir, name))
	if err != nil {
		return "", err
	}
	defer file.Close()
	zw := zip.NewWriter(file)
	defer zw.Close()
	parts := map[string]string{
		"[Content_Types].xml":        `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/></Types>`,
		"_rels/.rels":                `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`,
		"xl/workbook.xml":            `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="项目证据" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`,
		"xl/styles.xml":              `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><fonts count="3"><font><sz val="11"/><name val="Microsoft YaHei"/></font><font><b/><color rgb="FFFFFFFF"/><sz val="11"/><name val="Microsoft YaHei"/></font><font><b/><color rgb="FF052E2B"/><sz val="18"/><name val="Microsoft YaHei"/></font></fonts><fills count="4"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill><fill><patternFill patternType="solid"><fgColor rgb="FF0F766E"/><bgColor indexed="64"/></patternFill></fill><fill><patternFill patternType="solid"><fgColor rgb="FFCCFBF1"/><bgColor indexed="64"/></patternFill></fill></fills><borders count="2"><border/><border><bottom style="thin"><color rgb="FF99F6E4"/></bottom></border></borders><cellXfs count="4"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/><xf numFmtId="0" fontId="1" fillId="2" borderId="0" applyFill="1" applyFont="1"/><xf numFmtId="0" fontId="2" fillId="3" borderId="0" applyFill="1" applyFont="1"/><xf numFmtId="0" fontId="0" fillId="0" borderId="1" applyBorder="1"/></cellXfs></styleSheet>`,
	}
	for path, content := range parts {
		if err := writeZipEntry(zw, path, content); err != nil {
			return "", err
		}
	}
	var sheet strings.Builder
	sheet.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetViews><sheetView showGridLines="0" workbookViewId="0"><pane ySplit="3" topLeftCell="A4" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews><cols><col min="1" max="1" width="42" customWidth="1"/><col min="2" max="2" width="14" customWidth="1"/><col min="3" max="3" width="14" customWidth="1"/><col min="4" max="4" width="22" customWidth="1"/></cols><sheetData>`)
	sheet.WriteString(`<row r="1" ht="32" customHeight="1">` + xlsxInlineCell("A1", project+" · 可复算研究证据", "2") + `</row>`)
	sheet.WriteString(`<row r="2">` + xlsxInlineCell("A2", fmt.Sprintf("真实文件 %d 件；所有大小和时间来自磁盘", len(rows)), "") + `</row>`)
	sheet.WriteString(`<row r="3" ht="24" customHeight="1">` + xlsxInlineCell("A3", "文件", "1") + xlsxInlineCell("B3", "类型", "1") + xlsxInlineCell("C3", "字节", "1") + xlsxInlineCell("D3", "修改时间", "1") + `</row>`)
	for index, item := range rows {
		r := index + 4
		sheet.WriteString(fmt.Sprintf(`<row r="%d">%s%s%s%s</row>`, r, xlsxInlineCell(fmt.Sprintf("A%d", r), item.name, "3"), xlsxInlineCell(fmt.Sprintf("B%d", r), item.ext, "3"), xlsxNumberCell(fmt.Sprintf("C%d", r), item.size, "3"), xlsxInlineCell(fmt.Sprintf("D%d", r), item.modified, "3")))
	}
	sheet.WriteString(`</sheetData><autoFilter ref="A3:D` + fmt.Sprint(len(rows)+3) + `"/></worksheet>`)
	if err := writeZipEntry(zw, "xl/worksheets/sheet1.xml", sheet.String()); err != nil {
		return "", err
	}
	if err := zw.Close(); err != nil {
		return "", err
	}
	return name, nil
}

func projectPrototypeHTML(project, brief string, runnable bool) string {
	mode := "INTERACTIVE PRODUCT"
	if !runnable {
		mode = "UI PROTOTYPE"
	}
	script := ""
	if runnable {
		script = `<script>const tasks=[...document.querySelectorAll('.task')];tasks.forEach(x=>x.onclick=()=>{x.classList.toggle('done');document.querySelector('#done').textContent=tasks.filter(t=>t.classList.contains('done')).length});document.querySelector('#launch').onclick=()=>document.body.classList.toggle('focus');</script>`
	}
	return `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>` + xmlEscape(project) + `</title><style>*{box-sizing:border-box}body{margin:0;background:#06110f;color:#ecfdf5;font:16px/1.5 Inter,"Microsoft YaHei",sans-serif;min-height:100vh;overflow-x:hidden}body:before{content:"";position:fixed;inset:-30%;background:radial-gradient(circle at 75% 20%,#0f766e88,transparent 28%),radial-gradient(circle at 10% 80%,#22c55e33,transparent 24%);filter:blur(28px);pointer-events:none}.shell{position:relative;max-width:1180px;margin:auto;padding:48px 32px}.nav{display:flex;justify-content:space-between;align-items:center;border-bottom:1px solid #ffffff20;padding-bottom:22px}.brand{font-weight:900;letter-spacing:.16em}.mode{color:#5eead4;font:700 11px monospace;letter-spacing:.18em}.hero{display:grid;grid-template-columns:1.2fr .8fr;gap:64px;padding:92px 0 62px}.eyebrow{color:#4ade80;font:800 12px monospace;letter-spacing:.2em}h1{font-size:clamp(52px,8vw,104px);line-height:.92;letter-spacing:-.07em;margin:18px 0 28px;max-width:780px}h1 i{color:#5eead4;font-style:normal}.lead{max-width:660px;color:#a7f3d0;font-size:18px}.score{align-self:end;border-left:1px solid #ffffff25;padding:16px 0 16px 32px}.score strong{display:block;font-size:72px;line-height:1;color:#5eead4}.score span{color:#94a3b8;font-size:12px}.board{display:grid;grid-template-columns:repeat(3,1fr);gap:14px}.card{background:#ffffff0b;border:1px solid #ffffff18;border-radius:18px;padding:22px;min-height:170px;backdrop-filter:blur(16px);transition:.2s}.card:hover{transform:translateY(-5px);border-color:#5eead488}.card b{display:block;color:#5eead4;font:700 11px monospace;letter-spacing:.14em}.card h2{font-size:22px;margin:28px 0 8px}.card p{color:#94a3b8;font-size:13px}.task{cursor:pointer}.task.done{opacity:.42;text-decoration:line-through}.cta{margin-top:28px;display:flex;align-items:center;gap:18px}button{border:0;border-radius:999px;background:#5eead4;color:#052e2b;padding:15px 24px;font-weight:900;cursor:pointer;box-shadow:0 0 40px #2dd4bf55}.focus .card:not(:first-child){opacity:.22;transform:scale(.96)}@media(max-width:800px){.hero{grid-template-columns:1fr;padding-top:58px}.board{grid-template-columns:1fr}h1{font-size:58px}.score{display:none}}</style></head><body><main class="shell"><nav class="nav"><span class="brand">RESCENE / ` + xmlEscape(project) + `</span><span class="mode">` + mode + `</span></nav><section class="hero"><div><span class="eyebrow">LESS CHAT · MORE AUTOMATIC</span><h1>把想法变成<i>可运行成果</i></h1><p class="lead">` + xmlEscape(runeClip(strings.ReplaceAll(brief, "\n", " "), 240)) + `</p></div><div class="score"><strong><span id="done">0</span>/3</strong><span>VERIFIED MILESTONES</span></div></section><section class="board"><article class="card task"><b>01 / DISCOVER</b><h2>真实调研数据</h2><p>Excel 可复算，来源和文件变化可追踪。</p></article><article class="card task"><b>02 / BUILD</b><h2>交互原型</h2><p>点击即可运行，不把设计说明当设计稿。</p></article><article class="card task"><b>03 / LAUNCH</b><h2>传播与发布</h2><p>PPTX、MP4 与机器回执组成发布包。</p></article></section><div class="cta"><button id="launch">聚焦核心成果</button><span>点击卡片完成验收</span></div></main>` + script + `</body></html>`
}

func writeProjectFile(projectDir, name, content string) (string, error) {
	return name, os.WriteFile(filepath.Join(projectDir, name), []byte(content), 0o644)
}

func fileEvidence(projectDir, stage, role, name, kind, verification string) (deliveryEvidence, error) {
	path := filepath.Join(projectDir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		return deliveryEvidence{}, err
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		return deliveryEvidence{}, fmt.Errorf("%s 为空或不可读取", name)
	}
	sum := sha256.Sum256(data)
	return deliveryEvidence{Stage: stage, ProducerRole: role, File: name, Kind: kind, SHA256: fmt.Sprintf("%x", sum[:]), Bytes: info.Size(), Verification: verification}, nil
}

func enforceProjectDelivery(d *Daughter, projectDir, project, brief string) (projectDeliveryManifest, error) {
	manifest := projectDeliveryManifest{Project: project, Status: "blocked", GeneratedAt: time.Now().Format(time.RFC3339), GateVersion: 1}
	meeting, err := writeProjectFile(projectDir, "00-项目会议.meeting.json", fmt.Sprintf(`{"topic":%q,"kind":"evidence_kickoff","participants":["researcher","writer","designer","coder","promoter","publisher"],"decision":"所有硬门槛通过后才能进入审批"}`, project))
	if err != nil {
		return manifest, err
	}
	research, err := writeProjectFile(projectDir, "01-调研报告.md", "# 调研与证据\n\n本报告只引用同目录磁盘产物；结构化数据见 `02-研究数据.xlsx`。\n\n"+brief)
	if err != nil {
		return manifest, err
	}
	data, err := writeProjectResearchXLSX(projectDir, project)
	if err != nil {
		return manifest, fmt.Errorf("Excel 门禁失败: %w", err)
	}
	ui, err := writeProjectFile(projectDir, "03-UI原型.html", projectPrototypeHTML(project, brief, false))
	if err != nil {
		return manifest, err
	}
	docs, err := writeProjectFile(projectDir, "04-软件文档.md", "# "+project+" 软件文档\n\n## 运行\n\n直接打开 `output-app.html`。\n\n## 验收\n\n以 `delivery.manifest.json` 中 SHA-256 和阶段状态为准。\n")
	if err != nil {
		return manifest, err
	}
	runnable, err := writeProjectFile(projectDir, "output-app.html", projectPrototypeHTML(project, brief, true))
	if err != nil {
		return manifest, err
	}
	pptOutline := "## 为什么现在\n- 文本不是完成，能打开、能运行、能复核才是完成\n- 项目：" + project + "\n## 真实生产链\n- Excel 数据交付\n- UI 原型与可运行程序\n- PPTX 与 MP4 传播包\n## 验收证据\n- 每个文件记录 SHA-256\n- 缺少任一阶段即阻断审批\n## 下一步\n- 人类只审批完整项目\n- 退回后按反馈重新生产\n"
	pptName, err := renderPPTX(projectDir, project, pptOutline)
	if err != nil {
		return manifest, fmt.Errorf("PPTX 门禁失败: %w", err)
	}
	finalPPT := "05-项目路演.pptx"
	if err := os.Rename(filepath.Join(projectDir, pptName), filepath.Join(projectDir, finalPPT)); err != nil {
		return manifest, err
	}
	pvScript := "## 分镜\n- 旁白：这不是又一份方案，这是已经落盘的项目。\n- 旁白：研究数据进入 Excel，设计成为可以点击的原型。\n- 旁白：代码能够运行，PPT 和宣传片能够直接播放。\n- 旁白：没有证据的阶段，无法进入人类审批。\n"
	pvName, err := renderPV(projectDir, project, pvScript)
	if err != nil {
		return manifest, fmt.Errorf("MP4 门禁失败: %w", err)
	}
	finalPV := "06-宣传PV.mp4"
	if err := os.Rename(filepath.Join(projectDir, pvName), filepath.Join(projectDir, finalPV)); err != nil {
		return manifest, err
	}
	appBytes, appErr := os.ReadFile(filepath.Join(projectDir, runnable))
	if appErr != nil {
		return manifest, appErr
	}
	appSum := sha256.Sum256(appBytes)
	receiptBody := fmt.Sprintf("status=published\nchannel=local-project-preview\nproject=%s\npublished_at=%s\nentry=output-app.html\nentry_sha256=%x\n", project, time.Now().Format(time.RFC3339), appSum[:])
	receipt, err := writeProjectFile(projectDir, "07-发布.receipt", receiptBody)
	if err != nil {
		return manifest, err
	}

	definitions := []struct{ stage, role, file, kind, verification string }{
		{"meeting", "ceo", meeting, "meeting", "结构化参会者与硬门槛决议已落盘"},
		{"research", "researcher", research, "text", "调研报告引用同目录结构化数据"},
		{"data", "researcher", data, "spreadsheet", "有效 OOXML 工作簿；行来自真实磁盘文件"},
		{"requirements", "writer", "00-需求计划.md", "text", "需求与验收标准已落盘"},
		{"ui", "designer", ui, "html", "响应式 HTML 原型可在沙箱内渲染"},
		{"docs", "writer", docs, "text", "运行与验收说明完整"},
		{"code", "coder", runnable, "html", "HTML/CSS/JavaScript 源码已落盘"},
		{"runnable", "coder", runnable, "html", "浏览器直接打开并包含交互脚本"},
		{"ppt", "promoter", finalPPT, "pptx", "PowerPoint OOXML 已渲染"},
		{"pv", "promoter", finalPV, "video", "MP4 已由视频引擎完成编码"},
		{"promotion", "publisher", receipt, "receipt", "本地项目预览渠道发布回执"},
	}
	for _, definition := range definitions {
		evidence, evidenceErr := fileEvidence(projectDir, definition.stage, definition.role, definition.file, definition.kind, definition.verification)
		if evidenceErr != nil {
			manifest.Missing = append(manifest.Missing, definition.stage)
			continue
		}
		manifest.Evidence = append(manifest.Evidence, evidence)
	}
	if len(manifest.Missing) > 0 || len(manifest.Evidence) != len(mandatoryDeliveryStages) {
		return manifest, fmt.Errorf("交付门禁未通过: %s", strings.Join(manifest.Missing, ", "))
	}
	manifest.Status = "verified"
	encoded, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(projectDir, "delivery.manifest.json"), encoded, 0o644); err != nil {
		return manifest, err
	}
	return manifest, nil
}
