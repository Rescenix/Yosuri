// internal/memory/graph.go
package memory

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"go.etcd.io/bbolt"
)

// ErrClusterExists 在尝试注册一个已存在的簇时返回
var ErrClusterExists = errors.New("cluster already exists")

type NodeID uint64
type SynapseID uint64

type EdgeKind uint8

const (
	EdgeAssoc    EdgeKind = iota // 自由联想
	EdgeTemporal                 // 时间关联
	EdgeSemantic                 // 语义关联
	EdgeEpisodic                 // 情景关联
)

// ==================== 节点 ====================
type MemoryNode struct {
	ID           NodeID
	Text         string
	Role         string
	CreatedAt    time.Time
	LastAccessAt time.Time
	BaseEnergy   float64
	DecayRate    float64
	AccessCount  uint64
	Emotion      string  // 主情绪
	Intensity    float64 // 强度
	EventType    string  // 事件类型
	OutEdges     []SynapseID
	SourceTurns  []string `json:"source_turns,omitempty"`
	Importance   float64  `json:"importance,omitempty"`
	Hash         string   `json:"hash,omitempty"`
	Cluster      string   `json:"cluster,omitempty"` // 逻辑隔离：UserBase / CodeWork / ToolLog / Session
}

func (n *MemoryNode) EffectiveEnergy(now time.Time) float64 {
	elapsed := now.Sub(n.LastAccessAt).Hours()
	if elapsed <= 0 {
		return n.BaseEnergy
	}
	return n.BaseEnergy * math.Exp(-n.DecayRate*elapsed)
}

// ==================== 突触 ====================
type Synapse struct {
	ID        SynapseID
	From      NodeID
	To        NodeID
	Kind      EdgeKind
	Weight    float64
	LastUsed  time.Time
	DecayRate float64
}

func (s *Synapse) EffectiveWeight(now time.Time) float64 {
	elapsed := now.Sub(s.LastUsed).Hours()
	if elapsed <= 0 {
		return s.Weight
	}
	return s.Weight * math.Exp(-s.DecayRate*elapsed)
}

// ==================== 激活态 ====================
type ActivationState struct {
	Seeds     []NodeID
	Energy    map[NodeID]float64
	Frontier  []NodeID
	MaxDepth  uint8
	HopDecay  float64
	Threshold float64
	Now       time.Time
}

// ==================== 图结构 ====================
type Graph struct {
	mu       sync.RWMutex
	nodes    map[NodeID]*MemoryNode
	synapses map[SynapseID]*Synapse
	clusters map[string]string // 动态簇注册表：name -> description
	nextNID  NodeID
	nextSID  SynapseID
	db       *bbolt.DB
	inverted *InvertedIndex
}

// ★ 簇间权重矩阵：控制不同记忆簇之间的激活扩散强度
var clusterBridgeWeight = map[string]map[string]float64{
	"UserBase": {
		"UserBase": 1.0,
		"CodeWork": 0.8,
		"ToolLog":  0.05, // 阻断工具日志污染用户画像
		"Session":  1.0,
	},
	"CodeWork": {
		"UserBase": 0.8,
		"CodeWork": 1.0,
		"ToolLog":  0.3,
		"Session":  0.8,
	},
	"ToolLog": {
		"UserBase": 0.05,
		"CodeWork": 0.3,
		"ToolLog":  1.0,
		"Session":  0.05,
	},
	"Session": {
		"UserBase": 1.0,
		"CodeWork": 0.8,
		"ToolLog":  0.05,
		"Session":  1.0,
	},
}

func NewGraph(dbPath string) (*Graph, error) {
	db, err := bbolt.Open(dbPath, 0666, nil)
	if err != nil {
		return nil, err
	}

	// 确保 buckets 存在
	if err := db.Update(func(tx *bbolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte("nodes")); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte("synapses")); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte("clusters")); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	g := &Graph{
		nodes:    make(map[NodeID]*MemoryNode),
		synapses: make(map[SynapseID]*Synapse),
		clusters: make(map[string]string),
		nextNID:  1,
		nextSID:  1,
		db:       db,
	}

	// 恢复数据
	if err := g.loadFromDB(); err != nil {
		return nil, err
	}
	if err := g.loadClustersFromDB(); err != nil {
		return nil, err
	}

	g.inverted = NewInvertedIndex()
	for _, n := range g.nodes {
		g.inverted.Add(n.ID, n.Text)
	}
	return g, nil
}

func (g *Graph) Close() error {
	return g.db.Close()
}

// 从数据库加载所有节点和边
func (g *Graph) loadFromDB() error {
	return g.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("nodes"))
		if err := b.ForEach(func(k, v []byte) error {
			var n MemoryNode
			if err := json.Unmarshal(v, &n); err != nil {
				return err
			}
			g.nodes[n.ID] = &n
			if n.ID >= g.nextNID {
				g.nextNID = n.ID + 1
			}
			return nil
		}); err != nil {
			return err
		}

		sb := tx.Bucket([]byte("synapses"))
		return sb.ForEach(func(k, v []byte) error {
			var s Synapse
			if err := json.Unmarshal(v, &s); err != nil {
				return err
			}
			g.synapses[s.ID] = &s
			if s.ID >= g.nextSID {
				g.nextSID = s.ID + 1
			}
			if from, ok := g.nodes[s.From]; ok {
				from.OutEdges = append(from.OutEdges, s.ID)
			}
			return nil
		})
	})
}

// ==================== 基本操作（含持久化） ====================
func (g *Graph) AddNode(role, text string) *MemoryNode {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()
	n := &MemoryNode{
		ID:           g.nextNID,
		Text:         text,
		Role:         role,
		CreatedAt:    now,
		LastAccessAt: now,
		BaseEnergy:   0.5,
		DecayRate:    0.001,
		OutEdges:     make([]SynapseID, 0),
	}
	g.nextNID++
	g.nodes[n.ID] = n

	g.saveNodeToDB(n)
	g.inverted.Add(n.ID, text)
	return n
}

// AddNodeWithCluster 创建带指定簇归属的节点并持久化（供 ENGRAM 动态绑定簇使用）
func (g *Graph) AddNodeWithCluster(role, cluster, text string) *MemoryNode {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()
	n := &MemoryNode{
		ID:           g.nextNID,
		Text:         text,
		Role:         role,
		Cluster:      cluster,
		CreatedAt:    now,
		LastAccessAt: now,
		BaseEnergy:   0.5,
		DecayRate:    0.001,
		OutEdges:     make([]SynapseID, 0),
	}
	g.nextNID++
	g.nodes[n.ID] = n

	g.saveNodeToDB(n)
	g.inverted.Add(n.ID, text)
	return n
}

// ==================== 动态簇注册表 ====================

// loadClustersFromDB 从 bolt 的 clusters bucket 恢复簇注册表
func (g *Graph) loadClustersFromDB() error {
	return g.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("clusters"))
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			g.clusters[string(k)] = string(v)
			return nil
		})
	})
}

func (g *Graph) saveClusterToDB(name, desc string) {
	g.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte("clusters")).Put([]byte(name), []byte(desc))
	})
}

func (g *Graph) deleteClusterFromDB(name string) {
	g.db.Update(func(tx *bbolt.Tx) error {
		return tx.Bucket([]byte("clusters")).Delete([]byte(name))
	})
}

// Clusters 返回当前所有活跃簇的副本（name -> description）
func (g *Graph) Clusters() map[string]string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	cpy := make(map[string]string, len(g.clusters))
	for k, v := range g.clusters {
		cpy[k] = v
	}
	return cpy
}

// ClusterExists 判断簇是否已注册
func (g *Graph) ClusterExists(name string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	_, ok := g.clusters[name]
	return ok
}

// AddCluster 注册一个新簇；若同名簇已存在，返回 ErrClusterExists
func (g *Graph) AddCluster(name, desc string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.clusters[name]; ok {
		return ErrClusterExists
	}
	g.clusters[name] = desc
	g.saveClusterToDB(name, desc)
	return nil
}

// countNodesInClusterLocked 统计归属某簇的节点数（调用方须持有锁）
func (g *Graph) countNodesInClusterLocked(name string) int {
	count := 0
	for _, n := range g.nodes {
		if n.Cluster == name {
			count++
		}
	}
	return count
}

// RemoveClusterIfEmpty 仅当簇下没有关联记忆节点时才删除该簇。
// 返回：exists=簇是否存在；nodeCount=非空时的关联节点数；removed=是否已删除。
func (g *Graph) RemoveClusterIfEmpty(name string) (exists bool, nodeCount int, removed bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.clusters[name]; !ok {
		return false, 0, false
	}
	if c := g.countNodesInClusterLocked(name); c > 0 {
		return true, c, false
	}
	delete(g.clusters, name)
	g.deleteClusterFromDB(name)
	return true, 0, true
}

// SeedDefaultClusters 仅当注册表为空（首次启动）时，播种默认簇，保证旧系统平滑过渡
func (g *Graph) SeedDefaultClusters(defaults map[string]string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.clusters) > 0 {
		return
	}
	for name, desc := range defaults {
		g.clusters[name] = desc
		g.saveClusterToDB(name, desc)
	}
}

func (g *Graph) AddSynapse(from, to NodeID, kind EdgeKind, weight float64) *Synapse {
	g.mu.Lock()
	defer g.mu.Unlock()

	s := &Synapse{
		ID:        g.nextSID,
		From:      from,
		To:        to,
		Kind:      kind,
		Weight:    weight,
		LastUsed:  time.Now(),
		DecayRate: 0.002,
	}
	g.nextSID++
	g.synapses[s.ID] = s

	if fromNode, ok := g.nodes[from]; ok {
		fromNode.OutEdges = append(fromNode.OutEdges, s.ID)
	}

	g.saveSynapseToDB(s)
	return s
}

func (g *Graph) Node(id NodeID) *MemoryNode {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.nodes[id]
}

func (g *Graph) Synapse(id SynapseID) *Synapse {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.synapses[id]
}

// Nodes 返回所有节点的副本，避免外部并发读写崩溃
func (g *Graph) Nodes() map[NodeID]*MemoryNode {
	g.mu.RLock()
	defer g.mu.RUnlock()
	cpy := make(map[NodeID]*MemoryNode, len(g.nodes))
	for id, n := range g.nodes {
		cpy[id] = n
	}
	return cpy
}

// NodesByTime 返回按创建时间倒序排列的最近 n 个节点
func (g *Graph) NodesByTime(n int) []*MemoryNode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	all := make([]*MemoryNode, 0, len(g.nodes))
	for _, node := range g.nodes {
		all = append(all, node)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})
	if n > len(all) {
		n = len(all)
	}
	return all[:n]
}

// Synapses 返回所有突触的副本
func (g *Graph) Synapses() map[SynapseID]*Synapse {
	g.mu.RLock()
	defer g.mu.RUnlock()
	cpy := make(map[SynapseID]*Synapse, len(g.synapses))
	for id, s := range g.synapses {
		cpy[id] = s
	}
	return cpy
}

// ==================== 持久化辅助方法 ====================
func (g *Graph) saveNodeToDB(n *MemoryNode) {
	data, _ := json.Marshal(n)
	g.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("nodes"))
		idBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(idBytes, uint64(n.ID))
		return b.Put(idBytes, data)
	})
}

func (g *Graph) saveSynapseToDB(s *Synapse) {
	data, _ := json.Marshal(s)
	g.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("synapses"))
		idBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(idBytes, uint64(s.ID))
		return b.Put(idBytes, data)
	})
}

// ==================== 多跳传播（含簇间权重隔离） ====================
func (g *Graph) SpreadActivation(st *ActivationState) []ScoredNode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	for _, seed := range st.Seeds {
		st.Energy[seed] = 1.0
		st.Frontier = append(st.Frontier, seed)
	}

	for depth := uint8(0); depth < st.MaxDepth && len(st.Frontier) > 0; depth++ {
		nextFrontier := make([]NodeID, 0)

		for _, id := range st.Frontier {
			e := st.Energy[id]
			if e < st.Threshold {
				continue
			}
			srcNode := g.nodes[id]
			if srcNode == nil {
				continue
			}
			for _, synID := range srcNode.OutEdges {
				s := g.synapses[synID]
				if s == nil {
					continue
				}
				dstNode := g.nodes[s.To]
				if dstNode == nil {
					continue
				}

				// ★ 簇间权重衰减：阻断跨簇污染
				clusterFactor := 1.0
				if matrix, ok := clusterBridgeWeight[srcNode.Cluster]; ok {
					if factor, ok2 := matrix[dstNode.Cluster]; ok2 {
						clusterFactor = factor
					}
				}

				nextEnergy := e * s.EffectiveWeight(st.Now) * st.HopDecay * clusterFactor
				if nextEnergy < st.Threshold {
					continue
				}
				if nextEnergy > st.Energy[s.To] {
					st.Energy[s.To] = nextEnergy
					nextFrontier = append(nextFrontier, s.To)
				}
			}
		}
		st.Frontier = dedup(nextFrontier)
	}

	return g.rankByEnergy(st.Energy)
}

func dedup(ids []NodeID) []NodeID {
	seen := make(map[NodeID]bool)
	result := make([]NodeID, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result
}

type ScoredNode struct {
	Node  *MemoryNode
	Score float64
}

// rankByEnergy 现在作为 *Graph 的方法，返回完整的节点信息
func (g *Graph) rankByEnergy(energy map[NodeID]float64) []ScoredNode {
	result := make([]ScoredNode, 0, len(energy))
	for id, e := range energy {
		if e > 0 {
			node := g.nodes[id]
			if node == nil {
				continue
			}
			result = append(result, ScoredNode{Node: node, Score: e})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Score > result[j].Score
	})
	return result
}

func (g *Graph) HasMemoryHash(hash string) bool {
	// Nodes() 已返回副本，内部加锁安全
	for _, n := range g.Nodes() {
		if n.Hash == hash {
			return true
		}
	}
	return false
}

// PurgeAll 清空图中所有节点和突触，立即持久化到 bolt
func (g *Graph) PurgeAll() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.nodes = make(map[NodeID]*MemoryNode)
	g.synapses = make(map[SynapseID]*Synapse)
	g.nextNID = 1
	g.nextSID = 1

	g.db.Update(func(tx *bbolt.Tx) error {
		tx.DeleteBucket([]byte("nodes"))
		tx.CreateBucket([]byte("nodes"))
		tx.DeleteBucket([]byte("synapses"))
		tx.CreateBucket([]byte("synapses"))
		return nil
	})
	g.inverted = NewInvertedIndex()
}

// ExpandSeeds 从种子节点出发，沿突触收集 n 跳内的邻居（带簇间权重）
// 调用方必须持有 g.mu.RLock / Lock
func (g *Graph) ExpandSeeds(seeds map[NodeID]bool, maxHops int) map[NodeID]float64 {
	visited := make(map[NodeID]float64)
	frontier := make(map[NodeID]bool)
	for id := range seeds {
		frontier[id] = true
		visited[id] = 1.0
	}

	hopDecay := 0.6
	for hop := 1; hop <= maxHops; hop++ {
		nextFrontier := make(map[NodeID]bool)
		for id := range frontier {
			srcNode := g.nodes[id]
			if srcNode == nil {
				continue
			}
			for _, synID := range srcNode.OutEdges {
				s := g.synapses[synID]
				if s == nil {
					continue
				}
				dstNode := g.nodes[s.To]
				if dstNode == nil {
					continue
				}
				if _, seen := visited[s.To]; !seen {
					// ★ 簇间权重衰减
					clusterFactor := 1.0
					if matrix, ok := clusterBridgeWeight[srcNode.Cluster]; ok {
						if factor, ok2 := matrix[dstNode.Cluster]; ok2 {
							clusterFactor = factor
						}
					}
					score := math.Pow(hopDecay, float64(hop)) * s.Weight * clusterFactor
					visited[s.To] = score
					nextFrontier[s.To] = true
				}
			}
		}
		frontier = nextFrontier
	}
	return visited
}

// fuseAndRank 多信号融合打分（调用方需持有读锁）
func (g *Graph) fuseAndRank(query string, candidates map[NodeID]float64) []ScoredNode {
	results := make([]ScoredNode, 0, len(candidates))
	for id, graphScore := range candidates {
		node := g.nodes[id]
		if node == nil {
			continue
		}
		textScore := KeywordMatch(query, node.Text)
		recencyScore := math.Exp(-0.01 * time.Since(node.LastAccessAt).Hours())
		importanceScore := node.Intensity
		emotionScore := float64(emotionMatch(query, node.Emotion))

		score := 0.25*textScore + 0.20*graphScore + 0.15*recencyScore + 0.30*importanceScore + 0.10*emotionScore

		results = append(results, ScoredNode{Node: node, Score: score})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	return results
}

// fuseAndRankWithIntent 基于意图的多信号融合打分（调用方需持有读锁）
func (g *Graph) fuseAndRankWithIntent(query string, candidates map[NodeID]float64, intent *UserIntent) []ScoredNode {
	results := make([]ScoredNode, 0, len(candidates))
	for id, graphScore := range candidates {
		node := g.nodes[id]
		if node == nil {
			continue
		}
		textScore := KeywordMatch(query, node.Text)
		recencyScore := math.Exp(-0.01 * time.Since(node.LastAccessAt).Hours())
		importanceScore := node.Intensity
		// 意图中的情绪标签，与记忆的情绪标签匹配
		emotionScore := 0.0
		if intent != nil && intent.Emotion != "" && node.Emotion == intent.Emotion {
			emotionScore = 1.0
		}

		score := 0.25*textScore + 0.20*graphScore + 0.15*recencyScore + 0.30*importanceScore + 0.10*emotionScore

		results = append(results, ScoredNode{Node: node, Score: score})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	return results
}

func emotionMatch(query string, emotion string) int {
	if strings.Contains(query, "生气") && emotion == "angry" {
		return 1
	}
	if strings.Contains(query, "开心") && emotion == "happy" {
		return 1
	}
	return 0
}

// RetrieveRelevant 关键词检索 + 图扩散（加读锁保护并发）
func (g *Graph) RetrieveRelevant(query string) ([]*MemoryNode, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	seeds := g.inverted.Query(query)
	if len(seeds) == 0 {
		return nil, nil
	}

	graphScores := g.ExpandSeeds(seeds, 2)
	ranked := g.fuseAndRank(query, graphScores)

	n := len(ranked)
	if n > 3 {
		n = 3
	}
	result := make([]*MemoryNode, n)
	for i := 0; i < n; i++ {
		result[i] = ranked[i].Node
	}
	return result, nil
}

// RetrieveRelevantByIntent 基于意图的检索（加读锁保护并发）
func (g *Graph) RetrieveRelevantByIntent(intent *UserIntent) ([]*MemoryNode, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var queryTerms []string
	if intent.Keywords != nil {
		queryTerms = append(queryTerms, intent.Keywords...)
	}
	if intent.Emotion != "" {
		queryTerms = append(queryTerms, intent.Emotion)
	}

	seeds := g.inverted.Query(strings.Join(queryTerms, " "))
	if len(seeds) == 0 {
		return nil, nil
	}

	graphScores := g.ExpandSeeds(seeds, 2)
	ranked := g.fuseAndRankWithIntent(strings.Join(queryTerms, " "), graphScores, intent)

	n := len(ranked)
	if n > 3 {
		n = 3
	}
	result := make([]*MemoryNode, n)
	for i := 0; i < n; i++ {
		result[i] = ranked[i].Node
	}
	return result, nil
}

func KeywordMatch(query, text string) float64 {
	if query == "" || text == "" {
		return 0.0
	}
	qLower := strings.ToLower(query)
	tLower := strings.ToLower(text)
	words := strings.Fields(qLower)
	hits := 0
	for _, w := range words {
		if strings.Contains(tLower, w) {
			hits++
		}
	}
	return min(1.0, float64(hits)/10.0)
}
