// internal/api/handler.go
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"prismd/internal/domain"
	"prismd/internal/memory"

	"golang.org/x/text/width"
)

type PrimQLHandler struct {
	manager      *domain.Manager
	compileQueue chan []string
}

func NewPrimQLHandler(m *domain.Manager) *PrimQLHandler {
	h := &PrimQLHandler{
		manager:      m,
		compileQueue: make(chan []string, 100),
	}
	go h.compileWorker()
	go h.consolidateWorker()
	return h
}

// compileWorker 动态获取当前图进行压缩
// compileWorker 动态获取当前图进行压缩
func (h *PrimQLHandler) compileWorker() {
	for turns := range h.compileQueue {
		graph := h.manager.CurrentGraph()
		if graph == nil {
			continue
		}
		//node, err := graph.CompileMemoryForce(turns)
		node, err := graph.CompileMemory(turns)
		if err != nil {
			if strings.Contains(err.Error(), "not worth saving") {
				log.Printf("🧹 杉汐判定无长期价值，跳过压缩")
			} else {
				log.Printf("⚠️ 后台压缩失败: %v", err)
			}
			continue
		}
		log.Printf("✅ 后台压缩完成，节点ID=%d，重要性=%.2f", node.ID, node.Intensity)
	}
}

// consolidateWorker 动态获取当前图进行整理
func (h *PrimQLHandler) consolidateWorker() {
	for {
		time.Sleep(6 * time.Hour)
		graph := h.manager.CurrentGraph()
		if graph == nil {
			continue
		}
		log.Println("🌙 夜间记忆整理开始...")
		if err := graph.ConsolidateMemory(); err != nil {
			log.Printf("⚠️ 夜间记忆整理失败: %v", err)
		} else {
			log.Println("✅ 夜间记忆整理完成")
		}
	}
}

func (h *PrimQLHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		w.Write([]byte("PrismD v7.0 (Digital Hippocampus)\n"))
		return
	}

	body, _ := io.ReadAll(r.Body)
	defer r.Body.Close()

	response := h.handle(string(body))
	// 响应始终为 UTF-8；Windows cmd.exe 默认代码页是 GBK，查看中文前需先执行 chcp 65001
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write([]byte(response))
}

func (h *PrimQLHandler) handle(raw string) string {
	graph := h.manager.CurrentGraph()
	if graph == nil {
		return "ERROR no active domain\n"
	}

	parts := strings.SplitN(strings.TrimSpace(raw), " ", 2)
	cmd := strings.ToUpper(parts[0])
	rest := ""
	if len(parts) > 1 {
		rest = strings.TrimSpace(parts[1])
	}

	switch cmd {
	case "ENGRAM":
		subParts := strings.SplitN(rest, " ", 2)
		if len(subParts) < 2 {
			return "ERROR role and content required\n"
		}
		role := subParts[0]
		text := strings.TrimSpace(subParts[1])
		node := graph.AddNode(role, text)
		log.Printf("ENGRAM: id=%d, role=%s, text=%.40s", node.ID, role, text)
		return fmt.Sprintf("OK %d\n", node.ID)

	case "LOOM":
		loomArg := strings.TrimSpace(rest)

		// LOOM -N / --N：返回最近新增的 N 条记忆（按创建时间倒序）
		if strings.HasPrefix(loomArg, "-") {
			cleaned := strings.TrimLeft(loomArg, "-")
			if strings.HasPrefix(strings.ToLower(cleaned), "n") {
				nStr := strings.TrimSpace(cleaned[1:]) // 去掉 'n' 或 'N'，再取数字部分
				if n, err := strconv.Atoi(nStr); err == nil && n > 0 {
					nodes := graph.NodesByTime(n)

					var sb strings.Builder
					sb.WriteString(fmt.Sprintf("Recent %d memories:\n\n", len(nodes)))
					sb.WriteString(
						PadRight("ID", 5) + " " +
							PadRight("Role", 12) + " " +
							PadRight("Content", 35) + " " +
							PadRight("Energy", 8) + " " +
							PadRight("Emotion", 8) + "\n",
					)
					sb.WriteString(strings.Repeat("-", 72) + "\n")

					for _, node := range nodes {
						text := truncateByWidth(node.Text, 33)
						role := truncateByWidth(node.Role, 12)
						emotion := node.Emotion
						if emotion == "" {
							emotion = "-"
						}
						sb.WriteString(
							PadRight(fmt.Sprintf("%d", node.ID), 5) + " " +
								PadRight(role, 12) + " " +
								PadRight(text, 35) + " " +
								PadRight(fmt.Sprintf("%.2f", node.BaseEnergy), 8) + " " +
								PadRight(emotion, 8) + "\n",
						)
					}
					sb.WriteString(fmt.Sprintf("\n%d results\n", len(nodes)))
					return "OK\n" + sb.String()
				}
			}
		}

		// LOOM <id>：按 ID 精确查询单条记忆
		if id, err := strconv.ParseUint(loomArg, 10, 64); err == nil {
			n := graph.Node(memory.NodeID(id))
			if n == nil {
				return "ERROR node not found\n"
			}
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("── ID: %d ──\n", n.ID))
			sb.WriteString(fmt.Sprintf("Role: %s\n", n.Role))
			sb.WriteString(fmt.Sprintf("Content: %s\n", n.Text))
			sb.WriteString(fmt.Sprintf("Energy: %.2f | Emotion: %s | Intensity: %.2f | EventType: %s | Cluster: %s\n",
				n.BaseEnergy, n.Emotion, n.Intensity, n.EventType, n.Cluster))
			return "OK\n" + sb.String()
		}

		// 原有文本检索逻辑保持不变...

		var relevantNodes []*memory.MemoryNode
		intent, err := memory.AnalyzeUserIntent(rest)
		if err != nil || intent == nil {
			relevantNodes, _ = graph.RetrieveRelevant(rest)
		} else {
			relevantNodes, _ = graph.RetrieveRelevantByIntent(intent)
			// 新增：意图命中为空时回退
			if len(relevantNodes) == 0 {
				relevantNodes, _ = graph.RetrieveRelevant(rest)
			}
		}

		if relevantNodes == nil {
			return "OK 0 results\n"
		}

		now := time.Now()
		seeds := make([]memory.NodeID, 0, len(relevantNodes))
		for _, n := range relevantNodes {
			seeds = append(seeds, n.ID)
		}

		st := &memory.ActivationState{
			Seeds:     seeds,
			Energy:    make(map[memory.NodeID]float64),
			Frontier:  make([]memory.NodeID, 0),
			MaxDepth:  3,
			HopDecay:  0.7,
			Threshold: 0.01,
			Now:       now,
		}
		results := graph.SpreadActivation(st)

		for _, sn := range results {
			n := graph.Node(sn.Node.ID)
			if n == nil {
				continue
			}
			n.AccessCount++
			n.LastAccessAt = now
			n.BaseEnergy = math.Min(0.99, n.BaseEnergy+0.01)
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Query: %s\n\n", rest))
		sb.WriteString(fmt.Sprintf("%-5s %-12s %-35s %-8s %-6s %-8s %-8s\n",
			"ID", "Role", "Content", "Energy", "Score", "Emotion", "Inten"))
		sb.WriteString(strings.Repeat("-", 95) + "\n")

		for _, sn := range results {
			n := graph.Node(sn.Node.ID)
			if n == nil {
				continue
			}
			text := n.Text
			runes := []rune(text)
			if len(runes) > 33 {
				text = string(runes[:33]) + "..."
			}
			emotion := n.Emotion
			if emotion == "" {
				emotion = "-"
			}

			sb.WriteString(fmt.Sprintf("%-5d %-12s %-35s %-8.2f %-6.2f %-8s %-8.2f\n",
				n.ID, n.Role, text, n.BaseEnergy, sn.Score, emotion, n.Intensity))
		}
		sb.WriteString(fmt.Sprintf("\n%d results\n", len(results)))
		return "OK\n" + sb.String()

	case "REFRACT":
		var params struct {
			ID      uint64  `json:"id"`
			Content string  `json:"content,omitempty"`
			Role    string  `json:"role,omitempty"`
			Energy  float64 `json:"energy,omitempty"`
		}
		if err := json.Unmarshal([]byte(rest), &params); err != nil {
			return fmt.Sprintf("ERROR invalid json: %v\n", err)
		}
		n := graph.Node(memory.NodeID(params.ID))
		if n == nil {
			return "ERROR node not found\n"
		}
		if params.Content != "" {
			n.Text = params.Content
		}
		if params.Role != "" {
			n.Role = params.Role
		}
		if params.Energy > 0 {
			n.BaseEnergy = math.Min(0.99, params.Energy)
		} else {
			n.BaseEnergy = math.Min(0.99, n.BaseEnergy+0.25)
		}
		n.LastAccessAt = time.Now()
		log.Printf("REFRACT: id=%d, text=%.40s, energy=%.2f", n.ID, n.Text, n.BaseEnergy)
		return fmt.Sprintf("OK refracted %d\n", n.ID)

	case "PRUNE":
		if strings.TrimSpace(rest) == "" {
			graph.PurgeAll()
			log.Println("⚠️ PRUNE: 整个记忆场已被清空")
			return "OK all memories pruned\n"
		}
		idStr := strings.TrimSpace(rest)
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			return "ERROR invalid id\n"
		}
		n := graph.Node(memory.NodeID(id))
		if n == nil {
			return "ERROR node not found\n"
		}
		n.BaseEnergy = 0.0
		log.Printf("PRUNE: id=%d 已遗忘", id)
		return fmt.Sprintf("OK pruned %d\n", id)

	case "DRIFT":
		now := time.Now()
		count := 0
		for _, n := range graph.Nodes() {
			if n.BaseEnergy > 0.01 {
				n.BaseEnergy *= 0.95
				n.LastAccessAt = now
				count++
			}
		}
		log.Printf("DRIFT: %d 个节点被演化", count)
		return fmt.Sprintf("OK drifted %d nodes\n", count)

	case "STATS":
		if strings.ToUpper(rest) == "FULL" {
			var sb strings.Builder
			for _, n := range graph.Nodes() {
				sb.WriteString(fmt.Sprintf("── ID: %d ──\n", n.ID))
				sb.WriteString(fmt.Sprintf("Role: %s\n", n.Role))
				sb.WriteString(fmt.Sprintf("Content: %s\n", n.Text))
				sb.WriteString(fmt.Sprintf("Energy: %.2f | Emotion: %s | Intensity: %.2f | EventType: %s | Cluster: %s\n",
					n.BaseEnergy, n.Emotion, n.Intensity, n.EventType, n.Cluster))
				sb.WriteString(strings.Repeat("-", 60) + "\n")
			}
			sb.WriteString(fmt.Sprintf("\nTotal: %d neurons\n", len(graph.Nodes())))
			return "OK\n" + sb.String()
		}

		var sb strings.Builder
		sb.WriteString(
			PadRight("ID", 5) + " " +
				PadRight("Role", 15) + " " +
				PadRight("Content", 20) + " " +
				PadRight("Energy", 8) + " " +
				PadRight("Emotion", 8) + " " +
				PadRight("Inten", 8) + " " +
				PadRight("EventType", 10) + "\n",
		)
		sb.WriteString(strings.Repeat("-", 90) + "\n")

		for _, n := range graph.Nodes() {
			text := n.Text
			if DisplayWidth(text) > 18 {
				runes := []rune(text)
				text = string(runes[:6]) + ".."
			}
			role := n.Role
			if DisplayWidth(role) > 14 {
				runes := []rune(role)
				role = string(runes[:6]) + ".."
			}
			emotion := n.Emotion
			if emotion == "" {
				emotion = "-"
			}
			eventType := n.EventType
			if eventType == "" {
				eventType = "-"
			}

			sb.WriteString(
				PadRight(fmt.Sprintf("%d", n.ID), 5) + " " +
					PadRight(role, 15) + " " +
					PadRight(text, 20) + " " +
					PadRight(fmt.Sprintf("%.2f", n.BaseEnergy), 8) + " " +
					PadRight(emotion, 8) + " " +
					PadRight(fmt.Sprintf("%.2f", n.Intensity), 8) + " " +
					PadRight(eventType, 10) + "\n",
			)
		}
		sb.WriteString(fmt.Sprintf("\nTotal: %d neurons\n", len(graph.Nodes())))
		return "OK\n" + sb.String()

	case "CONSOLIDATE":
		if err := graph.ConsolidateMemory(); err != nil {
			return fmt.Sprintf("ERROR %v\n", err)
		}
		return "OK consolidated\n"

	case "GRAPH":
		nodes := make([]map[string]interface{}, 0)
		for _, n := range graph.Nodes() {
			nodes = append(nodes, map[string]interface{}{
				"id":     n.ID,
				"role":   n.Role,
				"text":   n.Text[:min(len(n.Text), 50)],
				"energy": n.BaseEnergy,
			})
		}
		edges := make([]map[string]interface{}, 0)
		for _, s := range graph.Synapses() {
			edges = append(edges, map[string]interface{}{
				"from":   s.From,
				"to":     s.To,
				"kind":   s.Kind,
				"weight": s.Weight,
			})
		}
		resp, _ := json.Marshal(map[string]interface{}{
			"nodes": nodes,
			"edges": edges,
		})
		return "OK " + string(resp) + "\n"

	case "COMPILE":
		turns := strings.Split(rest, "\n---\n")
		select {
		case h.compileQueue <- turns:
			log.Printf("📨 压缩任务已入队 (turns=%d)", len(turns))
			return "ACK\n"
		default:
			return "ERROR compile queue full\n"
		}
	case "COMPILE_SYNC":
		turns := strings.Split(rest, "\n---\n")
		if len(turns) == 0 {
			return "ERROR no turns provided\n"
		}
		graph := h.manager.CurrentGraph()
		if graph == nil {
			return "ERROR no active domain\n"
		}
		node, err := graph.CompileMemory(turns)
		if err != nil {
			return fmt.Sprintf("ERROR %v\n", err)
		}
		log.Printf("✅ 同步压缩完成，节点ID=%d，重要性=%.2f", node.ID, node.Intensity)
		return fmt.Sprintf("OK compiled %d (importance=%.2f)\n", node.ID, node.Intensity)
	case "DOMAIN":
		parts := strings.SplitN(rest, " ", 2)
		subCmd := strings.ToUpper(parts[0])
		subRest := ""
		if len(parts) > 1 {
			subRest = parts[1]
		}
		switch subCmd {
		case "USE":
			if err := h.manager.Use(subRest); err != nil {
				return fmt.Sprintf("ERROR %v\n", err)
			}
			return fmt.Sprintf("OK switched to domain '%s'\n", subRest)
		case "CREATE":
			if err := h.manager.Create(subRest); err != nil {
				return fmt.Sprintf("ERROR %v\n", err)
			}
			return fmt.Sprintf("OK created domain '%s'\n", subRest)
		case "LIST":
			list := h.manager.List()
			var sb strings.Builder
			for name, count := range list {
				sb.WriteString(fmt.Sprintf("%s: %d nodes\n", name, count))
			}
			return "OK\n" + sb.String()
		case "DROP":
			if err := h.manager.Drop(subRest); err != nil {
				return fmt.Sprintf("ERROR %v\n", err)
			}
			return fmt.Sprintf("OK dropped domain '%s'\n", subRest)
		default:
			return "ERROR unknown DOMAIN subcommand\n"
		}

	default:
		return "UNKNOWN\n"
	}
}

func (h *PrimQLHandler) findSeeds(query string) []memory.NodeID {
	graph := h.manager.CurrentGraph()
	if graph == nil {
		return nil
	}
	var seeds []memory.NodeID
	for _, n := range graph.Nodes() {
		if strings.Contains(strings.ToLower(n.Text), strings.ToLower(query)) {
			seeds = append(seeds, n.ID)
		}
	}
	return seeds
}

func DisplayWidth(s string) int {
	w := 0
	for _, r := range s {
		kind := width.LookupRune(r).Kind()
		if kind == width.EastAsianWide || kind == width.EastAsianFullwidth {
			w += 2
		} else {
			w += 1
		}
	}
	return w
}

func PadRight(s string, total int) string {
	pad := total - DisplayWidth(s)
	if pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}

// truncateByWidth 按显示宽度截断字符串，超出 maxWidth 时截断并追加 "..."
func truncateByWidth(s string, maxWidth int) string {
	if DisplayWidth(s) <= maxWidth {
		return s
	}
	var sb strings.Builder
	w := 0
	for _, r := range s {
		rw := 1
		kind := width.LookupRune(r).Kind()
		if kind == width.EastAsianWide || kind == width.EastAsianFullwidth {
			rw = 2
		}
		if w+rw > maxWidth-3 {
			break
		}
		sb.WriteRune(r)
		w += rw
	}
	return sb.String() + "..."
}
