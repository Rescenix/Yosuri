package domain

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"prismd/internal/memory"
)

type Manager struct {
	mu      sync.RWMutex
	domains map[string]*memory.Graph
	current string
	dataDir string
}

func NewManager(dataDir string) *Manager {
	m := &Manager{
		domains: make(map[string]*memory.Graph),
		dataDir: dataDir,
	}
	// ★ 关键：启动时扫描数据目录，预加载所有域
	m.loadExistingDomains()
	return m
}

// loadExistingDomains 扫描数据目录，自动发现并加载所有 prismd_*.bolt 文件
func (m *Manager) loadExistingDomains() {
	pattern := filepath.Join(m.dataDir, "prismd_*.bolt")
	files, err := filepath.Glob(pattern)
	if err != nil {
		log.Printf("[DomainManager] 扫描域文件失败: %v", err)
		return
	}

	for _, file := range files {
		// 从文件名提取域名：prismd_Atri.bolt → Atri
		base := filepath.Base(file)
		domainName := strings.TrimPrefix(base, "prismd_")
		domainName = strings.TrimSuffix(domainName, ".bolt")

		if domainName == "" {
			continue
		}

		graph, err := memory.NewGraph(file)
		if err != nil {
			log.Printf("[DomainManager] 加载域 '%s' 失败: %v", domainName, err)
			continue
		}
		m.domains[domainName] = graph
		log.Printf("[DomainManager] 已发现并加载域: %s (%d 节点)", domainName, len(graph.Nodes()))
	}
}

func (m *Manager) CurrentGraph() *memory.Graph {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.domains[m.current]
}

func (m *Manager) CurrentDomain() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current
}

func (m *Manager) Use(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.domains[name]; !ok {
		// 尝试打开已存在的数据库文件
		dbPath := filepath.Join(m.dataDir, fmt.Sprintf("prismd_%s.bolt", name))
		graph, err := memory.NewGraph(dbPath)
		if err != nil {
			return fmt.Errorf("域 '%s' 不存在且无法创建", name)
		}
		m.domains[name] = graph
	}
	m.current = name
	return nil
}

func (m *Manager) Create(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.domains[name]; ok {
		return fmt.Errorf("域 '%s' 已存在", name)
	}
	dbPath := filepath.Join(m.dataDir, fmt.Sprintf("prismd_%s.bolt", name))
	graph, err := memory.NewGraph(dbPath)
	if err != nil {
		return err
	}
	m.domains[name] = graph
	m.current = name
	return nil
}

func (m *Manager) List() map[string]int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]int)
	for name, g := range m.domains {
		result[name] = len(g.Nodes())
	}
	return result
}

func (m *Manager) Drop(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if g, ok := m.domains[name]; ok {
		g.Close()
		delete(m.domains, name)
		dbPath := filepath.Join(m.dataDir, fmt.Sprintf("prismd_%s.bolt", name))
		os.Remove(dbPath)
	}
	if m.current == name {
		m.current = ""
	}
	return nil
}

func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, g := range m.domains {
		g.Close()
	}
}
