package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const meetingDisclaimer = "本回放由各部门真实落盘产物重建，不是现场录屏；发言与证据文件一一对应。"

type meetingEvidence struct {
	Agent      string `json:"agent"`
	Department string `json:"department"`
	Role       string `json:"role"`
	Source     string `json:"source"`
	Size       int64  `json:"size"`
	Snippet    string `json:"-"`
	Modified   int64  `json:"-"`
}

type meetingSpeech struct {
	Order      int    `json:"order"`
	Agent      string `json:"agent"`
	Department string `json:"department"`
	Role       string `json:"role"`
	Text       string `json:"text"`
	Source     string `json:"source"`
	Start      string `json:"start"`
	End        string `json:"end"`
}

type meetingBundle struct {
	ID             string          `json:"id"`
	Topic          string          `json:"topic"`
	Host           string          `json:"host"`
	StartedAt      string          `json:"startedAt"`
	Kind           string          `json:"kind"`
	Disclaimer     string          `json:"disclaimer"`
	Speeches       []meetingSpeech `json:"speeches"`
	MinutesFile    string          `json:"minutesFile"`
	PPTFile        string          `json:"pptFile,omitempty"`
	TranscriptFile string          `json:"transcriptFile"`
	ReplayFile     string          `json:"replayFile,omitempty"`
	PPTError       string          `json:"pptError,omitempty"`
	ReplayError    string          `json:"replayError,omitempty"`
}

var meetingRoleOrder = []string{"researcher", "writer", "designer", "coder", "publisher", "promoter"}

func roleDepartment(role string) string {
	for _, item := range CompanyRoles {
		if item.Key == role {
			return item.Name
		}
	}
	return role
}

func agentRoleFromName(name string) string {
	for _, role := range meetingRoleOrder {
		if strings.HasPrefix(strings.ToLower(name), role+"-") || strings.EqualFold(name, role) {
			return role
		}
	}
	return ""
}

func evidenceText(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".md" && ext != ".txt" && ext != ".json" && ext != ".html" && ext != ".js" && ext != ".ts" && ext != ".go" && ext != ".py" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	text := strings.TrimSpace(string(data))
	return runeClip(text, 1200)
}

func newestAgentEvidence(agentDir, agent, role string) (meetingEvidence, bool) {
	best := meetingEvidence{}
	for _, rootName := range []string{"outputs", "projects"} {
		root := filepath.Join(agentDir, rootName)
		_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if entry.IsDir() {
				if entry.Name() == "node_modules" || entry.Name() == ".git" || entry.Name() == "archive" {
					return filepath.SkipDir
				}
				return nil
			}
			name := strings.ToLower(entry.Name())
			if strings.HasPrefix(name, "会议-") || strings.HasSuffix(name, ".meeting.json") || name == "readme.md" || strings.HasPrefix(name, ".") {
				return nil
			}
			info, statErr := entry.Info()
			if statErr != nil || info.Size() == 0 || info.ModTime().UnixNano() <= best.Modified {
				return nil
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return nil
			}
			source := filepath.ToSlash(rel)
			if rootName == "projects" {
				source = "project/" + source
			}
			best = meetingEvidence{
				Agent: agent, Department: roleDepartment(role), Role: role,
				Source: source, Size: info.Size(), Modified: info.ModTime().UnixNano(),
				Snippet: evidenceText(path),
			}
			return nil
		})
	}
	return best, best.Source != ""
}

// collectMeetingEvidence 每个部门只取一份最新的真实磁盘产物，发言不得脱离该证据。
func collectMeetingEvidence(ceoHome string) []meetingEvidence {
	companyRoot := filepath.Dir(ceoHome)
	entries, err := os.ReadDir(companyRoot)
	if err != nil {
		return nil
	}
	byRole := map[string]meetingEvidence{}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == filepath.Base(ceoHome) {
			continue
		}
		role := agentRoleFromName(entry.Name())
		if role == "" {
			continue
		}
		if candidate, ok := newestAgentEvidence(filepath.Join(companyRoot, entry.Name()), entry.Name(), role); ok {
			if old, exists := byRole[role]; !exists || candidate.Modified > old.Modified {
				byRole[role] = candidate
			}
		}
	}
	result := make([]meetingEvidence, 0, len(byRole))
	for _, role := range meetingRoleOrder {
		if item, ok := byRole[role]; ok {
			result = append(result, item)
		}
	}
	return result
}

func (d *Daughter) meetingSpeechFromEvidence(topic string, evidence meetingEvidence) string {
	prompt := fmt.Sprintf(`你是%s的 %s，正在参加「%s」。你只能根据下面这份已经落盘的产物做口头汇报，禁止虚构指标、进度或未完成的工作。

证据文件：%s（%d bytes）
文件摘录：
%s

用第一人称说 2-3 句自然、具体的中文发言：我交付了什么、它解决什么、需要哪个部门下一步接力。不要标题和列表。`, evidence.Department, evidence.Agent, topic, evidence.Source, evidence.Size, evidence.Snippet)
	if spoken := strings.TrimSpace(modelCallRetry(prompt)); spoken != "" {
		return runeClip(spoken, 360)
	}
	return fmt.Sprintf("我是%s的 %s。我确认已交付 %s（%d bytes），该文件已在磁盘落盘并可复核；由于模型暂不可用，本次不扩写文件之外的结论，等待下游部门按证据接力。", evidence.Department, evidence.Agent, evidence.Source, evidence.Size)
}

func meetingClock(seconds int) string {
	return fmt.Sprintf("00:%02d:%02d.000", seconds/60, seconds%60)
}

func buildMeetingSpeeches(d *Daughter, topic string, evidence []meetingEvidence) []meetingSpeech {
	speeches := make([]meetingSpeech, 0, len(evidence))
	for i, item := range evidence {
		start := 2 + i*8
		speeches = append(speeches, meetingSpeech{
			Order: i + 1, Agent: item.Agent, Department: item.Department, Role: item.Role,
			Text: d.meetingSpeechFromEvidence(topic, item), Source: item.Source,
			Start: meetingClock(start), End: meetingClock(start + 7),
		})
	}
	return speeches
}

func buildMeetingVTT(topic string, speeches []meetingSpeech) string {
	var b strings.Builder
	b.WriteString("WEBVTT\n\n")
	b.WriteString("00:00:00.000 --> 00:00:02.000\n主持人：" + topic + "。" + meetingDisclaimer + "\n\n")
	for _, speech := range speeches {
		fmt.Fprintf(&b, "%s --> %s\n%s · %s：%s\n证据：%s\n\n", speech.Start, speech.End, speech.Department, speech.Agent, speech.Text, speech.Source)
	}
	return b.String()
}

func buildMeetingMinutes(topic, startedAt string, speeches []meetingSpeech) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# 会议纪要：%s\n\n> %s\n\n- 时间：%s\n- 性质：基于真实产物的 AI 重建会议\n- 主持：CEO\n\n## 部门逐席发言\n\n", topic, meetingDisclaimer, startedAt)
	for _, speech := range speeches {
		fmt.Fprintf(&b, "### %s · %s\n\n%s\n\n- 证据：`%s`\n- 时间轴：%s → %s\n\n", speech.Department, speech.Agent, speech.Text, speech.Source, speech.Start, speech.End)
	}
	b.WriteString("## CEO 决策\n\n- 未找到证据的部门不计为参会或完成。\n- 所有下一步行动必须引用源文件，不能用口头状态替代交付。\n- 人类审批仍以完整项目为单位，会议材料不单独进入审批台。\n")
	return b.String()
}

func buildMeetingPPT(topic string, speeches []meetingSpeech) string {
	var b strings.Builder
	b.WriteString("## 本次会议\n")
	fmt.Fprintf(&b, "- %s\n- %d 个部门凭磁盘产物发言\n- AI 重建回放，不是现场录屏\n", topic, len(speeches))
	for _, speech := range speeches {
		fmt.Fprintf(&b, "## %s · %s\n- %s\n- 证据：%s\n", speech.Department, speech.Agent, runeClip(speech.Text, 150), speech.Source)
	}
	b.WriteString("## 决策与验收\n- 没有证据的部门不显示为参会\n- 产物可从会议页直接预览\n- 项目级审批保持为唯一人类闸门\n")
	return b.String()
}

func buildMeetingReplayScript(topic string, speeches []meetingSpeech) string {
	var b strings.Builder
	b.WriteString("## AI 重建会议分镜\n- 旁白：主持人宣布会议开始。" + meetingDisclaimer + "\n")
	for _, speech := range speeches {
		fmt.Fprintf(&b, "- 旁白：%s %s 发言。%s。证据文件 %s。\n", speech.Department, speech.Agent, speech.Text, speech.Source)
	}
	b.WriteString("- 旁白：CEO 总结，只有已落盘、可预览、可复核的文件才算完成。\n")
	return b.String()
}

func runMeetingBundle(d *Daughter, home, topic string) (meetingBundle, error) {
	if strings.TrimSpace(topic) == "" {
		topic = "公司周会"
	}
	evidence := collectMeetingEvidence(home)
	if len(evidence) == 0 {
		return meetingBundle{}, fmt.Errorf("没有找到任何部门真实产物，拒绝生成空壳会议")
	}
	outDir := filepath.Join(home, "outputs")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return meetingBundle{}, err
	}
	now := time.Now()
	id := fmt.Sprintf("%s-%02d", now.Format("2006-01-02-150405"), now.UnixNano()%100)
	// 社长指令注入——directive.json 存在则作为会议最高议题
	bossDirective := companyDirective()
	if bossDirective != "" {
		topic = strings.TrimSpace(topic)
		if !strings.Contains(topic, bossDirective) {
			topic = "社长指令专题会 · " + bossDirective
		}
	}
	speeches := buildMeetingSpeeches(d, topic, evidence)
	bundle := meetingBundle{
		ID: id, Topic: topic, Host: d.Name, StartedAt: now.Format(time.RFC3339),
		Kind: "ai_reconstruction", Disclaimer: meetingDisclaimer, Speeches: speeches,
		MinutesFile: "会议-" + id + ".md", TranscriptFile: "会议-" + id + ".vtt",
	}
	minutes := buildMeetingMinutes(topic, now.Format("2006-01-02 15:04"), speeches)
	if bossDirective != "" {
		minutes += fmt.Sprintf("\n\n## 社长通报（用户指令）\n\n> 社长指令：**%s**\n\n各部门已围绕该指令汇报进展，下一步由 CEO 统筹分配接力。社长的审批决定项目是否进入生产。\n", bossDirective)
	}
	if err := os.WriteFile(filepath.Join(outDir, bundle.MinutesFile), []byte(outputMeta("会议")+minutes), 0o644); err != nil {
		return bundle, err
	}
	if err := os.WriteFile(filepath.Join(outDir, bundle.TranscriptFile), []byte(buildMeetingVTT(topic, speeches)), 0o644); err != nil {
		return bundle, err
	}
	if name, err := renderPPTX(outDir, "会议 · "+topic, buildMeetingPPT(topic, speeches)); err != nil {
		bundle.PPTError = err.Error()
	} else {
		bundle.PPTFile = name
	}
	if bundle.PPTFile == "" {
		bundle.ReplayError = "会议 PPT 未生成，拒绝用色块兜底生成回放"
	} else {
		slideDir := filepath.Join(outDir, ".meeting-slides-"+id)
		defer os.RemoveAll(slideDir)
		if err := renderPPTXSlides(filepath.Join(outDir, bundle.PPTFile), slideDir); err != nil {
			bundle.ReplayError = err.Error()
		} else if name, err := renderPVWithMedia(outDir, "AI 重建会议 · "+topic, buildMeetingReplayScript(topic, speeches), slideDir); err != nil {
			bundle.ReplayError = err.Error()
		} else {
			bundle.ReplayFile = name
		}
	}
	manifest := "会议-" + id + ".meeting.json"
	data, _ := json.MarshalIndent(bundle, "", "  ")
	if err := os.WriteFile(filepath.Join(outDir, manifest), data, 0o644); err != nil {
		return bundle, err
	}
	return bundle, nil
}
