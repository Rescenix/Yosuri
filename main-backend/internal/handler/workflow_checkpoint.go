package handler

// 工作流断点续跑 —— 四态机每轮落盘一次进度，后端重启/SSE 断线后可从断点接着跑。
//
// 为什么需要：Yolo 全自动模式下一个任务可能跑十几轮、几分钟，期间后端重启或
// 前端断线，整个进行中的工作流就凭空消失，只能从头重发（工具全部重跑一遍，
// 已改的文件还可能被再改一次）。
//
// 落盘时机：每轮工具结果都写进 msgs 之后。这个位置是天然的安全边界——
// 此刻没有任何"已发起未完成"的工具调用，msgs 自洽，恢复后直接进下一轮问模型即可。
//
// 生命周期：任务正常收尾（完成/熔断）即删；只有异常中断留下的检查点才有意义。
// 上游报错、轮次/token 预算耗尽都留档（可续跑重试，见 codeWorkflowExhausted），
// 过期的由 24h TTL 兜底清掉。

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// checkpointTTL 超过这个时长没更新的检查点视为废弃，list/save 时顺手清理。
const checkpointTTL = 24 * time.Hour

// workflowCheckpoint 一个进行中工作流的全部可恢复状态。
// msgs 是完整对话（含系统提示词），恢复时整体replay，不再重建 prompt——
// 保证续跑用的上下文和中断前逐字一致。
type workflowCheckpoint struct {
	WorkflowID string `json:"workflow_id"`
	SessionID  string `json:"session_id"`
	OpenID     string `json:"openid"`
	Task       string `json:"task"`
	Mode       string `json:"mode"`
	Model      string `json:"model"`
	Effort     string `json:"effort"`
	// Round 已完成的轮数，续跑从这里接着算。终止条件是轮次上限（codeWorkflowMaxRounds，
	// 兜底值）和 token 预算（codeWorkflowTokenBudget）任一先触顶，见 codeWorkflowExhausted；
	// 两者都可以在单次请求上用 ?max_rounds=/?max_tokens= 覆盖。
	Round        int              `json:"round"`
	Msgs         []map[string]any `json:"msgs"`
	Transcript   []string         `json:"transcript"`
	CallSigCount map[string]int   `json:"call_sig_count"`
	// ActivatedTools 已被 load_tools 激活的 MCP 工具（见 tool_ondemand.go）。
	// 不存的话续跑后 tools 数组缩回常驻集，模型上一轮刚加载的工具突然消失，
	// 只能再 load 一遍——白白浪费一轮。
	ActivatedTools map[string]bool `json:"activated_tools"`
	CallSeq        int             `json:"call_seq"`
	InputTokens    int             `json:"input_tokens"`
	OutputTokens   int             `json:"output_tokens"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// checkpointDir 与会话文件同域：SHANXI_DATA_DIR 覆盖，默认 ~/shanxi_data/workflow_checkpoints。
func checkpointDir() string {
	dataDir := os.Getenv("SHANXI_DATA_DIR")
	if dataDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			homeDir = "."
		}
		dataDir = filepath.Join(homeDir, "shanxi_data")
	}
	return filepath.Join(dataDir, "workflow_checkpoints")
}

func checkpointPath(workflowID string) string {
	return filepath.Join(checkpointDir(), workflowID+".json")
}

// saveWorkflowCheckpoint 原子落盘（先写临时文件再 rename）——直接覆写的话，
// 恰好在写一半时进程被杀会留下半个 JSON，续跑时反而读到损坏状态。
func saveWorkflowCheckpoint(cp *workflowCheckpoint) {
	if cp.WorkflowID == "" {
		return
	}
	cp.UpdatedAt = time.Now()
	if err := os.MkdirAll(checkpointDir(), 0o755); err != nil {
		log.Printf("⚠️ 创建检查点目录失败: %v", err)
		return
	}
	data, err := json.Marshal(cp)
	if err != nil {
		log.Printf("⚠️ 序列化检查点失败: %v", err)
		return
	}
	path := checkpointPath(cp.WorkflowID)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		log.Printf("⚠️ 写检查点失败: %v", err)
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		log.Printf("⚠️ 提交检查点失败: %v", err)
		os.Remove(tmp)
	}
}

// loadWorkflowCheckpoint 读一个检查点；不存在/损坏/已过期都返回 nil（调用方按新任务处理）。
func loadWorkflowCheckpoint(workflowID string) *workflowCheckpoint {
	if workflowID == "" {
		return nil
	}
	data, err := os.ReadFile(checkpointPath(workflowID))
	if err != nil {
		return nil
	}
	var cp workflowCheckpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		log.Printf("⚠️ 检查点 %s 解析失败，按新任务处理: %v", workflowID, err)
		return nil
	}
	if time.Since(cp.UpdatedAt) > checkpointTTL {
		deleteWorkflowCheckpoint(workflowID)
		return nil
	}
	return &cp
}

func deleteWorkflowCheckpoint(workflowID string) {
	if workflowID == "" {
		return
	}
	os.Remove(checkpointPath(workflowID))
}

// listWorkflowCheckpoints 列出可续跑的检查点（新→旧），顺手清掉过期的。
// 前端用它在会话里显示「上次有个任务没跑完，续跑？」。
func listWorkflowCheckpoints(sessionID string) []map[string]any {
	entries, err := os.ReadDir(checkpointDir())
	if err != nil {
		return []map[string]any{}
	}
	out := []map[string]any{}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		id := e.Name()[:len(e.Name())-len(".json")]
		cp := loadWorkflowCheckpoint(id) // 过期的在这里被清掉并返回 nil
		if cp == nil {
			continue
		}
		if sessionID != "" && cp.SessionID != sessionID {
			continue
		}
		out = append(out, map[string]any{
			"workflow_id": cp.WorkflowID,
			"session_id":  cp.SessionID,
			"task":        cp.Task,
			"mode":        cp.Mode,
			"model":       cp.Model,
			"round":       cp.Round,
			"updated_at":  cp.UpdatedAt,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i]["updated_at"].(time.Time).After(out[j]["updated_at"].(time.Time))
	})
	return out
}
