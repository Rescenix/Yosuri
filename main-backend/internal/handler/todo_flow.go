package handler

// 任务 TODO —— agent 在工作流里维护一份结构化待办清单，实时下发给前端左下角便签。
//
// 机制与 load_tools 同类:update_todo 是个纯 UI 副作用的常驻工具,在 executeCodeCalls
// 前的预处理层直接办掉(不进真正的工具执行链、不需审批),把清单通过 SSE 的 todo 事件
// 推给前端,便签随执行进度实时勾选。复杂多步任务才用,简单任务不调。

import (
	"encoding/json"
	"strings"

	"backend/internal/ai/core"
)

const updateTodoToolName = "update_todo"

var updateTodoToolDef = core.ToolDefinition{
	Type: "function",
	Function: core.ToolFunctionDetail{
		Name: updateTodoToolName,
		Description: "维护当前任务的 TODO 清单(显示在左下角便签)。复杂多步任务开始时先列出计划," +
			"每完成一步就再调一次把对应项 status 改成 done、下一项改成 doing。简单一两步的任务不用调。" +
			"每次传完整清单(全量覆盖),不是增量。",
		Parameters: core.ToolParameters{
			Type: "object",
			Properties: map[string]core.ToolProperty{
				"items": {
					Type: "array",
					Description: "完整 TODO 数组,顺序即展示顺序。每项是对象 {\"text\":\"待办文本\",\"status\":\"pending|doing|done\"}:" +
						"pending=待办,doing=进行中(同一时刻通常只有一个),done=已完成。",
					Items: &core.ToolProperty{Type: "object"},
				},
			},
			Required: []string{"items"},
		},
	},
}

// todoItem 下发给前端的规范化 TODO 项。
type todoItem struct {
	Text   string `json:"text"`
	Status string `json:"status"` // pending | doing | done
}

// handleUpdateTodo 解析 update_todo 参数,返回规范化清单 + 给模型的简短回执。
// 容错:status 缺失/非法一律归为 pending;兼容模型偶尔用 content/done 之类字段名。
func handleUpdateTodo(argsJSON string) ([]todoItem, string) {
	var args struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return nil, "update_todo 参数解析失败,items 应为对象数组 [{\"text\":...,\"status\":...}]"
	}
	out := make([]todoItem, 0, len(args.Items))
	for _, m := range args.Items {
		text := firstString(m, "text", "content", "title", "todo")
		if strings.TrimSpace(text) == "" {
			continue
		}
		status := normalizeTodoStatus(m)
		out = append(out, todoItem{Text: strings.TrimSpace(text), Status: status})
	}
	if len(out) == 0 {
		return nil, "update_todo 的 items 为空或无有效项"
	}
	done := 0
	for _, it := range out {
		if it.Status == "done" {
			done++
		}
	}
	return out, "已更新 TODO 便签(" + itoa(done) + "/" + itoa(len(out)) + " 完成)"
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// normalizeTodoStatus 把各种写法归一到 pending/doing/done。
func normalizeTodoStatus(m map[string]any) string {
	s := strings.ToLower(strings.TrimSpace(firstString(m, "status", "state")))
	switch s {
	case "done", "completed", "complete", "finished", "ok":
		return "done"
	case "doing", "in_progress", "in-progress", "inprogress", "active", "running", "current":
		return "doing"
	case "pending", "todo", "waiting", "", "planned":
		// 兼容 done:true 布尔写法
		if b, ok := m["done"].(bool); ok && b {
			return "done"
		}
		return "pending"
	default:
		return "pending"
	}
}

// itoa 小工具:避免为了两个数字引入 strconv(本文件其余都用 encoding/json/strings)。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
