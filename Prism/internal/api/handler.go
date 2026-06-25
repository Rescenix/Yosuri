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

	"prismd/internal/memory"

	"golang.org/x/text/width"
)

type PrimQLHandler struct {
	graph        *memory.Graph
	compileQueue chan []string // 异步压缩任务队列
}

func NewPrimQLHandler(g *memory.Graph) *PrimQLHandler {
	h := &PrimQLHandler{
		graph:        g,
		compileQueue: make(chan []string, 100), // 缓冲 100 个压缩任务
	}
	// 启动后台压缩工人
	go h.compileWorker()
	return h
}

// compileWorker 不断从队列取任务，调用 CompileMemory
func (h *PrimQLHandler) compileWorker() {
	for turns := range h.compileQueue {
		node, err := h.graph.CompileMemory(turns)
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

func (h *PrimQLHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		w.Write([]byte("PrismD v7.0 (Digital Hippocampus)\n"))
		return
	}

	body, _ := io.ReadAll(r.Body)
	defer r.Body.Close()

	response := h.handle(string(body))
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write([]byte(response))
}

func (h *PrimQLHandler) handle(raw string) string {
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
		node := h.graph.AddNode(role, text)
		log.Printf("ENGRAM: id=%d, role=%s, text=%.40s", node.ID, role, text)
		return fmt.Sprintf("OK %d\n", node.ID)

	case "LOOM":
		var relevantNodes []*memory.MemoryNode
		intent, err := memory.AnalyzeUserIntent(rest)
		if err != nil || intent == nil {
			relevantNodes, _ = h.graph.RetrieveRelevant(rest)
		} else {
			relevantNodes, _ = h.graph.RetrieveRelevantByIntent(intent)
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
		results := h.graph.SpreadActivation(st)

		for _, sn := range results {
			n := h.graph.Node(sn.Node.ID)
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
			n := h.graph.Node(sn.Node.ID)
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
		n := h.graph.Node(memory.NodeID(params.ID))
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
			h.graph.PurgeAll()
			log.Println("⚠️ PRUNE: 整个记忆场已被清空")
			return "OK all memories pruned\n"
		}
		idStr := strings.TrimSpace(rest)
		id, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			return "ERROR invalid id\n"
		}
		n := h.graph.Node(memory.NodeID(id))
		if n == nil {
			return "ERROR node not found\n"
		}
		n.BaseEnergy = 0.0
		log.Printf("PRUNE: id=%d 已遗忘", id)
		return fmt.Sprintf("OK pruned %d\n", id)

	case "DRIFT":
		now := time.Now()
		count := 0
		for _, n := range h.graph.Nodes() {
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
			for _, n := range h.graph.Nodes() {
				sb.WriteString(fmt.Sprintf("── ID: %d ──\n", n.ID))
				sb.WriteString(fmt.Sprintf("Role: %s\n", n.Role))
				sb.WriteString(fmt.Sprintf("Content: %s\n", n.Text))
				sb.WriteString(fmt.Sprintf("Energy: %.2f | Emotion: %s | Intensity: %.2f | EventType: %s\n",
					n.BaseEnergy, n.Emotion, n.Intensity, n.EventType))
				sb.WriteString(strings.Repeat("-", 60) + "\n")
			}
			sb.WriteString(fmt.Sprintf("\nTotal: %d neurons\n", len(h.graph.Nodes())))
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

		for _, n := range h.graph.Nodes() {
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
		sb.WriteString(fmt.Sprintf("\nTotal: %d neurons\n", len(h.graph.Nodes())))
		return "OK\n" + sb.String()
	// case "CONSOLIDATE":
	// 	if err := h.graph.ConsolidateMemory(); err != nil {
	// 		return fmt.Sprintf("ERROR %v\n", err)
	// 	}
	// 	return "OK consolidated\n"
	case "GRAPH":
		nodes := make([]map[string]interface{}, 0)
		for _, n := range h.graph.Nodes() {
			nodes = append(nodes, map[string]interface{}{
				"id":     n.ID,
				"role":   n.Role,
				"text":   n.Text[:min(len(n.Text), 50)],
				"energy": n.BaseEnergy,
			})
		}
		edges := make([]map[string]interface{}, 0)
		for _, s := range h.graph.Synapses() {
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
		// 异步投递，不阻塞
		select {
		case h.compileQueue <- turns:
			log.Printf("📨 压缩任务已入队 (turns=%d)", len(turns))
			return "ACK\n"
		default:
			return "ERROR compile queue full\n"
		}

	default:
		return "UNKNOWN\n"
	}
}

func (h *PrimQLHandler) findSeeds(query string) []memory.NodeID {
	var seeds []memory.NodeID
	for _, n := range h.graph.Nodes() {
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
