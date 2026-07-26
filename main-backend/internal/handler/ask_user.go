package handler

// ask_user —— 让 agent 在工作流跑一半时向用户提问并暂停等待回答（human-in-the-loop）。
// 对标审批（approval.go）的 RequestPort 模式：agent 调 ask_user 工具 → 后端推 question
// 事件、在 per-workflow 的 channel 上阻塞等待 → 前端弹提问弹窗，用户回答后 POST
// /api/code/workflow/answer 唤醒循环，答案作为 user/tool 结果注入上下文续跑。
//
// 与 steer（中途插话）的区别：steer 是用户主动塞话、下一轮才被看见；ask_user 是 agent
// 主动发问、循环当场停住等回答，是真正的「同步阻塞式人机交互」。同一工作流同一时刻
// 只允许一个未决提问（循环阻塞着，天然串行），所以 registry 按 workflowID 索引即可。

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"backend/internal/ai/core"
	"github.com/gin-gonic/gin"
)

const askUserToolName = "ask_user"

// askUserTimeout 后端兜底超时：比前端默认超时（按用户习惯，不会太长）略长，避免前端
// 整个挂掉时 goroutine 永久阻塞、SSE 连接一直挂着。到点用问题自带的 fallback 继续，不卡死任务。
const askUserTimeout = 5 * time.Minute

// askUserOption 是 ask_user 的一个候选选项。
type askUserOption struct {
	Label string `json:"label"`           // 展示文字
	Value string `json:"value,omitempty"` // 回传值，省略则与 label 相同
}

// askUserArgs 是 ask_user 工具的参数。
type askUserArgs struct {
	Question   string          `json:"question"`              // 必须：问用户的话
	Options    []askUserOption `json:"options,omitempty"`     // 可选：候选（单选/多选）
	Multi      bool            `json:"multi,omitempty"`       // 是否多选，默认单选
	AllowOther bool            `json:"allow_other,omitempty"` // 是否额外提供「其他」自由输入
	Fallback   string          `json:"fallback,omitempty"`    // 用户不答/超时时的兜底答案
}

var askUserToolDef = core.ToolDefinition{
	Type: "function",
	Function: core.ToolFunctionDetail{
		Name: askUserToolName,
		Description: "当你需要用户拍板、选方向或补充信息才能继续时调用本工具。" +
			"调用后工作流会暂停，把问题（及可选项）推到输入框上方，等用户回答后再继续。" +
			"适合确认方案（A/B/C）、收集偏好、让用户在几个候选里挑一个。\n" +
			"【必须调用本工具的触发词】只要用户让你「问他/向用户提问/用选项让他选/让我选 A/B/C" +
			"/问我想要哪种/让我挑一个方向」等明确要把问题抛给用户本人的情况，" +
			"【必须立即调用】不要先解释、复述要求、展示参数、思考选项格式，也不要先在回复文本里写选项；" +
			"直接用一句简短问题和 2-5 个简短选项调用。只有本工具会显示可交互选项。\n" +
			"【不要调用的情况】能用工具自己查到的信息（读文件、跑命令、查 API）就别问；" +
			"不要每步都问。仅在确有分叉、需要人决断时才用。",
		Parameters: core.ToolParameters{
			Type: "object",
			Properties: map[string]core.ToolProperty{
				"question": {
					Type:        "string",
					Description: "要问用户的问题，清晰具体",
				},
				"options": {
					Type: "array",
					Description: "必须提供 2-5 个选项。每项是 {label: 展示文字, value: A/B/C/D/E}；" +
						"不得把选项写进 question 正文。",
					MinItems: 2,
					Items: &core.ToolProperty{
						Type:        "object",
						Description: "一个选项：{label: 必填展示文字, value: 可选回传值}",
						Properties: map[string]core.ToolProperty{
							"label": {
								Type:        "string",
								Description: "选项文案，不要包含 A/B/C/D 前缀",
							},
							"value": {
								Type:        "string",
								Description: "简短回传值，建议使用 A/B/C/D",
							},
						},
						Required: []string{"label"},
					},
				},
				"multi": {
					Type:        "boolean",
					Description: "是否允许多选，默认 false（单选）",
				},
				"allow_other": {
					Type:        "boolean",
					Description: "是否额外提供「其他」自由输入框，默认 false",
				},
				"fallback": {
					Type:        "string",
					Description: "用户不答或超时时的兜底答案；不填则超时按空答案继续",
				},
			},
			Required: []string{"question", "options"},
		},
	},
}

// askUserReply 是 answer 端点写回的回答（已拼成最终文字）。
type askUserReply struct {
	answer string
}

// askRegistry 按 workflowID 索引正在等待回答的提问，让独立的
// POST /api/code/workflow/answer 能唤醒对应工作流里阻塞的那次 ask_user。
var (
	askRegistryMu sync.Mutex
	askRegistry   = make(map[string]chan askUserReply)
)

func registerAskUser(workflowID string) chan askUserReply {
	ch := make(chan askUserReply, 1)
	askRegistryMu.Lock()
	askRegistry[workflowID] = ch
	askRegistryMu.Unlock()
	return ch
}

func unregisterAskUser(workflowID string) {
	askRegistryMu.Lock()
	delete(askRegistry, workflowID)
	askRegistryMu.Unlock()
}

// waitAskUser 阻塞直到该 workflow 的提问被回答 / 超时 / ctx 取消。
// 返回最终注入上下文的答案文字（空串 = 超时或断线，调用方应再套 fallback）。
func waitAskUser(workflowID string, ch chan askUserReply, done <-chan struct{}) string {
	timer := time.NewTimer(askUserTimeout)
	defer timer.Stop()
	select {
	case r := <-ch:
		return r.answer
	case <-timer.C:
		return ""
	case <-done:
		return ""
	}
}

// HandleCodeWorkflowAnswer POST /api/code/workflow/answer
// 前端提问弹窗「确认」回调：把用户答案写回阻塞中的 ask_user channel，恢复四态机执行。
func (r *WorkflowRunner) HandleCodeWorkflowAnswer(c *gin.Context) {
	var req struct {
		ID         string   `json:"id"`
		Answer     string   `json:"answer"`
		Selected   []string `json:"selected,omitempty"`
		WorkflowID string   `json:"workflow_id,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}
	if req.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id 必填"})
		return
	}
	askRegistryMu.Lock()
	ch, ok := askRegistry[req.WorkflowID]
	askRegistryMu.Unlock()
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "提问不存在或已结束", "id": req.ID})
		return
	}
	answer := req.Answer
	if answer == "" && len(req.Selected) > 0 {
		answer = joinSelected(req.Selected)
	}
	select {
	case ch <- askUserReply{answer: answer}:
	default:
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "id": req.ID})
}

func joinSelected(vals []string) string {
	out := ""
	for i, v := range vals {
		if i > 0 {
			out += "、"
		}
		out += v
	}
	return out
}

// handleAskUser 处理一次 ask_user 调用：把问题推到前端弹窗、阻塞等用户回答，
// 返回「要作为工具结果注入上下文的答案」和一个用于可视化轨迹的 FlowBlock。
// flowBlocks 由调用方负责 append（这里只构造），保持对主循环局部变量的访问简单。
func handleAskUser(c *gin.Context, workflowID string, askCh chan askUserReply, argsJSON string) (string, FlowBlock) {
	var a askUserArgs
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil || strings.TrimSpace(a.Question) == "" {
		// 参数错误不是“用户没回答”。明确回给模型让它立刻修正重调，避免前端什么都
		// 没显示、模型却误以为已经等待用户直到超时。
		msg := "ask_user 参数无效：question 必填，options 必须是 [{\"label\":\"...\",\"value\":\"A\"}]；请立即修正参数并重新调用"
		return msg, FlowBlock{Type: "question", Question: "提问参数无效", Answer: msg}
	}
	if len(a.Options) < 2 {
		msg := "ask_user 参数无效：options 必须由你填写至少 2 项，格式为 [{\"label\":\"选项文案\",\"value\":\"A\"}, ...]；不要把选项写进 question，请立即重新调用"
		return msg, FlowBlock{Type: "question", Question: "提问参数无效", Answer: msg}
	}
	id := "ask_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	// 选项透传给前端（label+value），value 缺省回退到 label
	opts := make([]map[string]string, 0, len(a.Options))
	blockOpts := make([]askUserOption, 0, len(a.Options))
	for _, o := range a.Options {
		v := o.Value
		if v == "" {
			v = o.Label
		}
		opts = append(opts, map[string]string{"label": o.Label, "value": v})
		blockOpts = append(blockOpts, askUserOption{Label: o.Label, Value: v})
	}
	writeCodeSSE(c, "question", map[string]any{
		"id":          id,
		"workflow_id": workflowID,
		"question":    a.Question,
		"options":     opts,
		"multi":       a.Multi,
		"allow_other": a.AllowOther,
	})
	ans := waitAskUser(workflowID, askCh, c.Request.Context().Done())
	if ans == "" {
		ans = a.Fallback // 超时/断线兜底
	}
	writeCodeSSE(c, "question_answered", map[string]any{"id": id, "answer": ans})
	return ans, FlowBlock{
		Type:     "question",
		Question: a.Question,
		Options:  blockOpts,
		Answer:   ans,
		Multi:    a.Multi,
	}
}
