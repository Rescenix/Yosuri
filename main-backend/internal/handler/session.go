package handler

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"backend/internal/ai/core"

	"github.com/gin-gonic/gin"
)

// 会话按用途分域存储（各自一个本地 JSON 文件），物理隔离，互不干扰，
// 也方便以后分别做压缩/清理策略。
const (
	ChatSessionsDomain = "chat_sessions"
	CodeSessionsDomain = "code_sessions"
)

// SessionStore 维护所有会话的对话历史与压缩游标。
// 内存里的 map 是权威状态；每次写操作后异步整份重写对应域的本地 JSON 文件——
// 个人使用场景数据量小，简单粗暴地整份重写比增量更新更不容易出 bug。
type SessionStore struct {
	mu                  sync.RWMutex
	sessions            map[string][]DSMessage
	lastCompressIndexes map[string]int             // 每个 session 上次压缩的消息数量
	approvalRules       map[string]map[string]bool // sessionID → (ruleKey → true)
	domain              string

	fileMu sync.Mutex // 串行化本地文件写入，避免并发重写互相踩踏
}

// persistedMessage 是 DSMessage 面向持久化的镜像。
// DSMessage.Timestamp 打了 json:"-"，是为了不把时间戳带进发给 LLM 的请求体；
// 这里单独定义一个带 timestamp 字段的结构体用于持久化，两头互不影响。
type persistedMessage struct {
	Role             string          `json:"role"`
	Content          string          `json:"content,omitempty"`
	ReasoningContent string          `json:"reasoning_content,omitempty"`
	Timestamp        time.Time       `json:"timestamp"`
	ToolCalls        []core.ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string          `json:"tool_call_id,omitempty"`
	Model            string          `json:"model,omitempty"`
	Blocks           []FlowBlock     `json:"blocks,omitempty"`
}

// sessionRecord 是单个会话在本地文件里的完整存储形态：
// 消息列表和压缩游标绑在一起。
type sessionRecord struct {
	Messages      []persistedMessage `json:"messages"`
	CompressIndex int                `json:"compress_index"`
	// ApprovalRules 是「don't ask again」常设规则：key=approve:<tool>，value=true 表示
	// 该会话对这款危险工具免审批（抄 agent-framework-go toolapproval 常设规则思路）
	ApprovalRules map[string]bool `json:"approval_rules,omitempty"`
}

func toPersistedMessages(msgs []DSMessage) []persistedMessage {
	out := make([]persistedMessage, len(msgs))
	for i, m := range msgs {
		out[i] = persistedMessage{
			Role:             m.Role,
			Content:          m.Content,
			ReasoningContent: m.ReasoningContent,
			Timestamp:        m.Timestamp,
			ToolCalls:        m.ToolCalls,
			ToolCallID:       m.ToolCallID,
			Model:            m.Model,
			Blocks:           m.Blocks,
		}
	}
	return out
}

func fromPersistedMessages(msgs []persistedMessage) []DSMessage {
	out := make([]DSMessage, len(msgs))
	for i, m := range msgs {
		out[i] = DSMessage{
			Role:             m.Role,
			Content:          m.Content,
			ReasoningContent: m.ReasoningContent,
			Timestamp:        m.Timestamp,
			ToolCalls:        m.ToolCalls,
			ToolCallID:       m.ToolCallID,
			Model:            m.Model,
			Blocks:           m.Blocks,
		}
	}
	return out
}

// 旧版本地 JSON 落盘格式（PrismD 之前、多域拆分之前），仅用于一次性迁移旧数据
type legacySessionFileData struct {
	Sessions            map[string][]persistedMessage `json:"sessions"`
	LastCompressIndexes map[string]int                `json:"last_compress_indexes"`
}

// NewSessionStore 创建一个绑定到指定域（如 ChatSessionsDomain）的会话存储。
// 启动时从该域对应的本地文件加载已有会话；如果该文件不存在且发现更早期的
// 单文件旧版格式（sessions.json，PrismD 迁移前遗留），会做一次性迁移。
func NewSessionStore(domain string) *SessionStore {
	store := &SessionStore{
		sessions:            make(map[string][]DSMessage),
		lastCompressIndexes: make(map[string]int),
		approvalRules:       make(map[string]map[string]bool),
		domain:              domain,
	}
	if err := store.loadFromFile(); err != nil {
		log.Printf("⚠️ 加载本地会话文件失败（域=%s）：%v，本次以空会话启动", domain, err)
	}
	if len(store.sessions) == 0 {
		store.migrateLegacyJSONFile()
	}
	return store
}

// sessionsFilePath 返回该域对应的本地会话文件路径，支持
// SHANXI_DATA_DIR 环境变量覆盖（测试/多实例场景）。
func sessionsFilePath(domain string) string {
	dataDir := os.Getenv("SHANXI_DATA_DIR")
	if dataDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			homeDir = "."
		}
		dataDir = filepath.Join(homeDir, "shanxi_data")
	}
	return filepath.Join(dataDir, "sessions_"+domain+".json")
}

// loadFromFile 从本地文件加载该域的全部会话到内存
func (s *SessionStore) loadFromFile() error {
	path := sessionsFilePath(s.domain)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 首次运行，正常情况
		}
		return err
	}

	var records map[string]sessionRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for sid, rec := range records {
		s.sessions[sid] = fromPersistedMessages(rec.Messages)
		s.lastCompressIndexes[sid] = rec.CompressIndex
		if len(rec.ApprovalRules) > 0 {
			s.approvalRules[sid] = rec.ApprovalRules
		}
	}
	return nil
}

// migrateLegacyJSONFile 是一次性的历史数据搬家：更早期版本把所有会话攒在
// 本地一个 sessions.json 文件里（PrismD 迁移之前），现在改成按域各自一个文件。
// 仅在当前域的文件首次为空时触发。
func (s *SessionStore) migrateLegacyJSONFile() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}
	legacyPath := filepath.Join(homeDir, "shanxi_data", "sessions.json")
	data, err := os.ReadFile(legacyPath)
	if err != nil {
		return // 没有旧文件，全新安装的正常情况
	}

	var legacy legacySessionFileData
	if err := json.Unmarshal(data, &legacy); err != nil {
		log.Printf("⚠️ 解析旧版会话文件失败，跳过迁移: %v", err)
		return
	}
	if len(legacy.Sessions) == 0 {
		return
	}

	log.Printf("🔄 检测到旧版本地会话文件（%d 个会话），迁移到域 '%s'...", len(legacy.Sessions), s.domain)

	s.mu.Lock()
	for sid, msgs := range legacy.Sessions {
		s.sessions[sid] = fromPersistedMessages(msgs)
		s.lastCompressIndexes[sid] = legacy.LastCompressIndexes[sid]
	}
	s.mu.Unlock()

	if err := s.persistAll(); err != nil {
		log.Printf("⚠️ 迁移会话到本地文件失败: %v", err)
		return
	}
	log.Printf("✅ 会话迁移完成")
}

// persistAll 把内存中该域的全部会话整份写入本地文件（原子替换，避免半写损坏）
func (s *SessionStore) persistAll() error {
	s.mu.RLock()
	records := make(map[string]sessionRecord, len(s.sessions))
	for sid, msgs := range s.sessions {
		records[sid] = sessionRecord{
			Messages:      toPersistedMessages(msgs),
			CompressIndex: s.lastCompressIndexes[sid],
			ApprovalRules: s.approvalRules[sid],
		}
	}
	s.mu.RUnlock()

	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}

	path := sessionsFilePath(s.domain)
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Append 追加消息，自动补时间戳，异步持久化到本地文件
func (s *SessionStore) Append(sessionID string, msg DSMessage) {
	s.mu.Lock()
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	s.sessions[sessionID] = append(s.sessions[sessionID], msg)
	s.mu.Unlock()

	go func() {
		if err := s.persistAll(); err != nil {
			log.Printf("⚠️ 保存会话到本地文件失败: %v", err)
		}
	}()
}

// Truncate 只保留会话的前 keep 条消息，丢弃其余——用于"编辑并重发某条历史消息"：
// 把被编辑的那条及其之后的全部对话截掉，再由新工作流从这个点接着写。
// keep 由前端按"被编辑消息之前已完成的往返对数 × 2"算好传来（每对 = user+assistant）。
func (s *SessionStore) Truncate(sessionID string, keep int) {
	if keep < 0 {
		keep = 0
	}
	s.mu.Lock()
	msgs := s.sessions[sessionID]
	if keep < len(msgs) {
		// 复制而非切片别名，避免底层数组残留被后续 append 覆写读到脏数据
		kept := make([]DSMessage, keep)
		copy(kept, msgs[:keep])
		s.sessions[sessionID] = kept
		// 压缩位点不能指向已被截掉的位置，跟着回退
		if s.lastCompressIndexes[sessionID] > keep {
			s.lastCompressIndexes[sessionID] = keep
		}
	}
	s.mu.Unlock()

	go func() {
		if err := s.persistAll(); err != nil {
			log.Printf("⚠️ 截断会话后保存本地文件失败: %v", err)
		}
	}()
}

// Delete 删除指定会话（内存 + 本地文件），供 DELETE /api/sessions/:id 使用
func (s *SessionStore) Delete(sessionID string) {
	s.mu.Lock()
	delete(s.sessions, sessionID)
	delete(s.lastCompressIndexes, sessionID)
	delete(s.approvalRules, sessionID)
	s.mu.Unlock()

	if err := s.persistAll(); err != nil {
		log.Printf("⚠️ 删除会话后保存本地文件失败: %v", err)
	}
}

// GetApprovalRule 读取该会话对某工具签名的「don't ask again」规则。
func (s *SessionStore) GetApprovalRule(sessionID, ruleKey string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rules := s.approvalRules[sessionID]
	return rules != nil && rules[ruleKey]
}

// SetApprovalRule 写入（或清除）该会话对某工具签名的「don't ask again」规则，异步落盘。
func (s *SessionStore) SetApprovalRule(sessionID, ruleKey string, val bool) {
	s.mu.Lock()
	if s.approvalRules[sessionID] == nil {
		s.approvalRules[sessionID] = make(map[string]bool)
	}
	if val {
		s.approvalRules[sessionID][ruleKey] = true
	} else {
		delete(s.approvalRules[sessionID], ruleKey)
	}
	s.mu.Unlock()

	go func() {
		if err := s.persistAll(); err != nil {
			log.Printf("⚠️ 保存审批规则到本地文件失败: %v", err)
		}
	}()
}

// Get 返回指定会话的消息切片（副本）
func (s *SessionStore) Get(sessionID string) []DSMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	msgs := s.sessions[sessionID]
	if msgs == nil {
		return nil
	}
	copied := make([]DSMessage, len(msgs))
	copy(copied, msgs)
	return copied
}

// GetCompressIndex 获取上次压缩位置（已压缩的消息数量）
func (s *SessionStore) GetCompressIndex(sessionID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastCompressIndexes[sessionID]
}

// SetCompressIndex 更新压缩游标（通常在成功压缩后调用），异步持久化到本地文件
func (s *SessionStore) SetCompressIndex(sessionID string, index int) {
	s.mu.Lock()
	s.lastCompressIndexes[sessionID] = index
	s.mu.Unlock()

	go func() {
		if err := s.persistAll(); err != nil {
			log.Printf("⚠️ 保存压缩游标到本地文件失败: %v", err)
		}
	}()
}

// AllSessions 返回所有会话消息的快照副本（按 sessionID 分组），供统计聚合使用
func (s *SessionStore) AllSessions() map[string][]DSMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string][]DSMessage, len(s.sessions))
	for id, msgs := range s.sessions {
		copied := make([]DSMessage, len(msgs))
		copy(copied, msgs)
		out[id] = copied
	}
	return out
}

// List 列出所有会话摘要
func (s *SessionStore) List() []SessionInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var infos []SessionInfo
	for id, msgs := range s.sessions {
		if len(msgs) == 0 {
			continue
		}
		title := "新对话"
		for _, m := range msgs {
			if m.Role == "user" {
				title = m.Content
				break
			}
		}
		infos = append(infos, SessionInfo{
			ID:        id,
			Title:     title,
			UpdatedAt: msgs[len(msgs)-1].Timestamp,
		})
	}
	// map 遍历顺序随机，"最近会话"列表不排序的话每次刷新顺序都在跳——按更新时间降序
	sort.Slice(infos, func(i, j int) bool { return infos[i].UpdatedAt.After(infos[j].UpdatedAt) })
	return infos
}

// SessionInfo 会话摘要
type SessionInfo struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AllMessage 所有会话的扁平消息
type AllMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// GetAllMessagesHandler 获取所有会话的全部消息（按时间排序）
func GetAllMessagesHandler(store *SessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if store == nil {
			c.JSON(500, gin.H{"error": "session store not initialized"})
			return
		}
		store.mu.RLock()
		defer store.mu.RUnlock()

		var all []AllMessage
		for _, msgs := range store.sessions {
			for _, msg := range msgs {
				all = append(all, AllMessage{
					Role:      msg.Role,
					Content:   msg.Content,
					Timestamp: msg.Timestamp,
				})
			}
		}
		sort.Slice(all, func(i, j int) bool {
			return all[i].Timestamp.Before(all[j].Timestamp)
		})
		c.JSON(200, all)
	}
}
