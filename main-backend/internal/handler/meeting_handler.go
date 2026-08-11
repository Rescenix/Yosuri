package handler

// meeting_handler.go — 公司会议产物包 API
// GET /api/company/meetings — 真实证据发言 + PPT/VTT/AI 重建回放；旧文件明确标为仅纪要。

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
)

type meetingSpeechResponse struct {
	Order      int    `json:"order"`
	Agent      string `json:"agent"`
	Department string `json:"department"`
	Role       string `json:"role"`
	Text       string `json:"text"`
	Source     string `json:"source"`
	Start      string `json:"start"`
	End        string `json:"end"`
}

type meetingResponse struct {
	ID             string                  `json:"id,omitempty"`
	Agent          string                  `json:"agent"`
	Topic          string                  `json:"topic"`
	Host           string                  `json:"host,omitempty"`
	StartedAt      string                  `json:"startedAt,omitempty"`
	Time           string                  `json:"time"`
	Kind           string                  `json:"kind"`
	Disclaimer     string                  `json:"disclaimer,omitempty"`
	Speeches       []meetingSpeechResponse `json:"speeches"`
	File           string                  `json:"file"`
	Content        string                  `json:"content,omitempty"`
	PPTFile        string                  `json:"pptFile,omitempty"`
	TranscriptFile string                  `json:"transcriptFile,omitempty"`
	ReplayFile     string                  `json:"replayFile,omitempty"`
	PPTError       string                  `json:"pptError,omitempty"`
	ReplayError    string                  `json:"replayError,omitempty"`
	sortTime       int64
}

type meetingManifest struct {
	ID             string                  `json:"id"`
	Topic          string                  `json:"topic"`
	Host           string                  `json:"host"`
	StartedAt      string                  `json:"startedAt"`
	Kind           string                  `json:"kind"`
	Disclaimer     string                  `json:"disclaimer"`
	Speeches       []meetingSpeechResponse `json:"speeches"`
	MinutesFile    string                  `json:"minutesFile"`
	PPTFile        string                  `json:"pptFile"`
	TranscriptFile string                  `json:"transcriptFile"`
	ReplayFile     string                  `json:"replayFile"`
	PPTError       string                  `json:"pptError"`
	ReplayError    string                  `json:"replayError"`
}

func optionalBase(name string) string {
	if strings.TrimSpace(name) == "" {
		return ""
	}
	return filepath.Base(name)
}

func readMeetingManifest(outDir, agent, name string) (meetingResponse, bool) {
	path := filepath.Join(outDir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		return meetingResponse{}, false
	}
	var manifest meetingManifest
	if json.Unmarshal(data, &manifest) != nil || manifest.MinutesFile == "" {
		return meetingResponse{}, false
	}
	content, _ := os.ReadFile(filepath.Join(outDir, filepath.Base(manifest.MinutesFile)))
	info, _ := os.Stat(path)
	response := meetingResponse{
		ID: manifest.ID, Agent: agent, Topic: manifest.Topic, Host: manifest.Host,
		StartedAt: manifest.StartedAt, Kind: manifest.Kind, Disclaimer: manifest.Disclaimer,
		Speeches: manifest.Speeches, File: filepath.Base(manifest.MinutesFile), Content: string(content),
		PPTFile: optionalBase(manifest.PPTFile), TranscriptFile: optionalBase(manifest.TranscriptFile),
		ReplayFile: optionalBase(manifest.ReplayFile), PPTError: manifest.PPTError, ReplayError: manifest.ReplayError,
	}
	if info != nil {
		response.Time = info.ModTime().Format("01-02 15:04")
		response.sortTime = info.ModTime().UnixNano()
	}
	return response, true
}

var meetingDepartmentNames = map[string]string{
	"researcher": "研究部", "writer": "作者部", "designer": "设计部",
	"coder": "程序部", "promoter": "宣传部", "publisher": "发布部", "ceo": "CEO",
}

// projectReviewMeeting 把通过 11 阶段硬门槛的项目作为一次交付评审展示。
// 视频是项目演示回放，不冒充现场会议录像；部门汇报直接绑定 manifest 证据。
func projectReviewMeeting(agent, projectName, projectPath string, gate projectDeliveryGate) (meetingResponse, bool) {
	topic := strings.TrimSpace(gate.Project)
	if topic == "" {
		topic = projectName
	}
	prefix := "project/" + filepath.ToSlash(projectName) + "/"
	byRole := map[string]projectDeliveryEvidence{}
	priority := map[string]int{"meeting": 1, "research": 3, "data": 2, "requirements": 2, "docs": 3, "ui": 3, "code": 2, "runnable": 3, "ppt": 2, "pv": 3, "promotion": 3}
	var meetingFile, pptFile, replayFile string
	for _, evidence := range gate.Evidence {
		switch evidence.Stage {
		case "meeting":
			meetingFile = prefix + evidence.File
		case "ppt":
			pptFile = prefix + evidence.File
		case "pv":
			replayFile = prefix + evidence.File
		}
		old, exists := byRole[evidence.ProducerRole]
		if !exists || priority[evidence.Stage] > priority[old.Stage] {
			byRole[evidence.ProducerRole] = evidence
		}
	}
	if meetingFile == "" || (pptFile == "" && replayFile == "") {
		return meetingResponse{}, false
	}

	roleOrder := []string{"researcher", "writer", "designer", "coder", "promoter", "publisher"}
	speeches := make([]meetingSpeechResponse, 0, len(roleOrder))
	var minutes strings.Builder
	fmt.Fprintf(&minutes, "# 项目交付评审：%s\n\n> 本评审展示已经通过交付门槛的真实文件，不是现场会议录像。\n\n- 项目状态：11/11 阶段已验证\n- 主持：CEO\n- 交付 Agent：%s\n\n## 部门证据汇报\n\n", topic, agent)
	for _, role := range roleOrder {
		evidence, ok := byRole[role]
		if !ok {
			continue
		}
		department := meetingDepartmentNames[role]
		text := strings.TrimSpace(evidence.Verification)
		if text == "" {
			text = "该阶段文件已经落盘并通过哈希校验"
		}
		order := len(speeches) + 1
		start := 2 + (order-1)*5
		speeches = append(speeches, meetingSpeechResponse{
			Order: order, Agent: agent, Department: department, Role: role,
			Text: text, Source: prefix + evidence.File,
			Start: fmt.Sprintf("00:00:%02d.000", start), End: fmt.Sprintf("00:00:%02d.000", start+4),
		})
		fmt.Fprintf(&minutes, "### %s\n\n%s\n\n- 证据：`%s`\n\n", department, text, evidence.File)
	}
	minutes.WriteString("## CEO 结论\n\n- 会议、调研、数据、需求、UI、文档、代码、程序、PPT、PV、发布回执均已落盘。\n- 项目已进入人类审批门，最终价值判断仍由人类完成。\n")

	info, _ := os.Stat(filepath.Join(projectPath, "delivery.manifest.json"))
	response := meetingResponse{
		ID: "project-review-" + projectName, Agent: agent, Topic: topic + " · 完整交付评审",
		Host: "CEO", StartedAt: gate.GeneratedAt, Kind: "project_review",
		Disclaimer: "本评审回放展示该项目已验证的真实交付物，不是现场会议录像。",
		Speeches:   speeches, File: meetingFile, Content: minutes.String(), PPTFile: pptFile, ReplayFile: replayFile,
	}
	if info != nil {
		response.Time = info.ModTime().Format("01-02 15:04")
		response.sortTime = info.ModTime().UnixNano()
	}
	return response, true
}

// HandleCompanyMeetings 返回历史纪要、完整会议包和已验证项目交付评审。
func HandleCompanyMeetings(c *gin.Context) {
	dir := companyDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"meetings": []meetingResponse{}})
		return
	}
	meetings := make([]meetingResponse, 0)
	projectSeen := map[string]bool{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		outDir := filepath.Join(dir, entry.Name(), "outputs")
		files, _ := os.ReadDir(outDir)
		manifestMinutes := map[string]bool{}
		for _, file := range files {
			if file.IsDir() || !strings.HasPrefix(file.Name(), "会议-") || !strings.HasSuffix(file.Name(), ".meeting.json") {
				continue
			}
			if meeting, ok := readMeetingManifest(outDir, entry.Name(), file.Name()); ok {
				manifestMinutes[meeting.File] = true
				meetings = append(meetings, meeting)
			}
		}
		for _, file := range files {
			if file.IsDir() || !strings.HasPrefix(file.Name(), "会议-") || !strings.HasSuffix(strings.ToLower(file.Name()), ".md") || manifestMinutes[file.Name()] {
				continue
			}
			data, readErr := os.ReadFile(filepath.Join(outDir, file.Name()))
			if readErr != nil {
				continue
			}
			info, _ := file.Info()
			legacy := meetingResponse{Agent: entry.Name(), Topic: "历史会议", Kind: "minutes_only", Speeches: []meetingSpeechResponse{}, File: file.Name(), Content: string(data)}
			if info != nil {
				legacy.Time = info.ModTime().Format("01-02 15:04")
				legacy.sortTime = info.ModTime().UnixNano()
			}
			meetings = append(meetings, legacy)
		}

		projectsDir := filepath.Join(dir, entry.Name(), "projects")
		projects, projectsErr := os.ReadDir(projectsDir)
		if projectsErr != nil {
			continue
		}
		for _, project := range projects {
			if !project.IsDir() || projectSeen[project.Name()] {
				continue
			}
			projectPath := filepath.Join(projectsDir, project.Name())
			if _, statErr := os.Stat(filepath.Join(projectPath, "delivery.manifest.json")); statErr != nil {
				continue
			}
			gate, gateErr := verifyProjectDeliveryGate(projectPath)
			if gateErr != nil {
				continue
			}
			if meeting, ok := projectReviewMeeting(entry.Name(), project.Name(), projectPath, gate); ok {
				projectSeen[project.Name()] = true
				meetings = append(meetings, meeting)
			}
		}
	}
	sort.Slice(meetings, func(i, j int) bool { return meetings[i].sortTime > meetings[j].sortTime })
	if len(meetings) > 10 {
		meetings = meetings[:10]
	}
	c.JSON(http.StatusOK, gin.H{"meetings": meetings})
}
