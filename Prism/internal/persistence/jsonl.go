package persistence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// NodeRecord 是用于持久化的节点记录（与 memory.MemoryNode 解耦）
type NodeRecord struct {
	ID           uint64    `json:"id"`
	Text         string    `json:"text"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	LastAccessAt time.Time `json:"last_access_at"`
	BaseEnergy   float64   `json:"base_energy"`
	DecayRate    float64   `json:"decay_rate"`
	AccessCount  uint64    `json:"access_count"`
}

// SynapseRecord 是用于持久化的突触记录
type SynapseRecord struct {
	ID        uint64    `json:"id"`
	From      uint64    `json:"from"`
	To        uint64    `json:"to"`
	Weight    float64   `json:"weight"`
	LastUsed  time.Time `json:"last_used"`
	DecayRate float64   `json:"decay_rate"`
}

type JSONLStore struct {
	mu       sync.Mutex
	dataDir  string
	nodeFile *os.File
	synFile  *os.File
}

func NewJSONLStore(dataDir string) *JSONLStore {
	os.MkdirAll(dataDir, 0755)
	nodeFile, _ := os.OpenFile(filepath.Join(dataDir, "nodes.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	synFile, _ := os.OpenFile(filepath.Join(dataDir, "synapses.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	return &JSONLStore{dataDir: dataDir, nodeFile: nodeFile, synFile: synFile}
}

func (s *JSONLStore) AppendNode(rec NodeRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, _ := json.Marshal(rec)
	s.nodeFile.Write(append(data, '\n'))
}

func (s *JSONLStore) AppendSynapse(rec SynapseRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, _ := json.Marshal(rec)
	s.synFile.Write(append(data, '\n'))
}

func (s *JSONLStore) LoadAll() ([]NodeRecord, []SynapseRecord, error) {
	// TODO: 从文件读取并解析
	return nil, nil, nil
}
