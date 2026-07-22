package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"maps"
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
	forkMeta            map[string]forkInfo        // 分支会话 → 父会话与分岐点（根会话不入表）
	domain              string
	// path 在构造时就定死，之后不再重读 SHANXI_DATA_DIR。
	// Append 的落盘是 fire-and-forget 的 goroutine，可能在环境变量被改掉之后才真正执行；
	// 每次现算路径的话，那些迟到的 goroutine 会把内存状态写到另一个位置去
	// （测试里 t.Setenv 恢复环境变量后，就正是这样把真实用户数据覆盖掉的）。
	path string

	fileMu sync.Mutex // 串行化本地文件写入，避免并发重写互相踩踏
}

// forkInfo 记录一条分支会话的血缘。ForkIndex 是从父会话拷贝过来的前缀长度，
// 所以 msgs[ForkIndex:] 恰好是"只属于这条分支"的消息（List 用它算标题）。
type forkInfo struct {
	ParentID  string
	ForkIndex int
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
	// ParentID 非空表示这是一条分支会话（见 Fork）。两个字段都 omitempty，
	// 所以老记录读进来是根会话、重写后也不会平白多出这两个键。
	ParentID  string `json:"parent_id,omitempty"`
	ForkIndex int    `json:"fork_index,omitempty"`
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
		forkMeta:            make(map[string]forkInfo),
		domain:              domain,
		path:                sessionsFilePath(domain),
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
	data, err := os.ReadFile(s.path)
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
		// ForkIndex>0 而 ParentID 为空 = 父会话已被删、它被提升成了根，
		// 但仍要记住"自己的内容从第几条开始"，否则重启后标题会跳回拷贝来的前缀那句
		if rec.ParentID != "" || rec.ForkIndex > 0 {
			s.forkMeta[sid] = forkInfo{ParentID: rec.ParentID, ForkIndex: rec.ForkIndex}
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
		fm := s.forkMeta[sid]
		// 审批规则必须在锁内深拷贝。只拷 map 引用的话，下面的 json.Marshal 是在
		// 释放读锁之后才读这个 map 的，而 SetApprovalRule 会在写锁里改同一个 map——
		// 两边碰的是同一块内存，-race 能稳定复现（fatal: concurrent map iteration
		// and map write 在生产里也真的会崩）。nil/空保持不建 map，omitempty 行为不变。
		var rules map[string]bool
		if src := s.approvalRules[sid]; len(src) > 0 {
			rules = make(map[string]bool, len(src))
			for k, v := range src {
				rules[k] = v
			}
		}
		records[sid] = sessionRecord{
			Messages:      toPersistedMessages(msgs),
			CompressIndex: s.lastCompressIndexes[sid],
			ApprovalRules: rules,
			ParentID:      fm.ParentID,
			ForkIndex:     fm.ForkIndex,
		}
	}
	s.mu.RUnlock()

	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}

	path := s.path // 构造时定死，迟到的落盘 goroutine 也只会写这一个位置
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

// Fork 从 parentID 的前 keep 条消息拷出一条新分支会话，返回新会话 ID。
// 用于"编辑并重发某条历史消息"：以前那是 Truncate（把后面的对话永久砍掉），
// 现在改成开新分支，原来那条线索完整保留——这是分支功能的全部意义。
//
// keep 沿用 Truncate 时代的语义（从头保留几条），前端算好的
// "被编辑消息之前已完成的往返对数 × 2" 一个字都不用改。
// 父会话为空（或不存在，二者在惰性建表下不可区分）返回 ok=false。
func (s *SessionStore) Fork(parentID string, keep int) (string, bool) {
	s.mu.Lock()
	parentMsgs := s.sessions[parentID]
	if len(parentMsgs) == 0 {
		s.mu.Unlock()
		return "", false
	}
	if keep < 0 {
		keep = 0
	}
	if keep > len(parentMsgs) {
		keep = len(parentMsgs)
	}

	// 时钟粒度粗时（Windows 尤其）紧循环里 UnixNano 真会撞，兜一下
	newID := ""
	for {
		newID = fmt.Sprintf("sess_%d", time.Now().UnixNano())
		if _, exists := s.sessions[newID]; !exists {
			break
		}
	}

	// 必须复制而非切片别名：否则子会话首次 Append 在容量够时会写进父会话的底层数组。
	// 注意这是浅拷贝，父子共享每条消息内部的 ToolCalls/Blocks 切片——当前安全，
	// 因为 Append 是唯一的消息写入路径，没有任何地方原地修改历史消息。
	copied := make([]DSMessage, keep)
	copy(copied, parentMsgs[:keep])
	s.sessions[newID] = copied
	s.forkMeta[newID] = forkInfo{ParentID: parentID, ForkIndex: keep}

	// 审批规则跟着分支走：分支是同一条工作线的延续，一编辑重发就要把危险工具
	// 重新批一遍是明显的体验倒退。但必须克隆成新 map——共享引用的话子会话之后
	// SetApprovalRule 会静默改写父会话的权限。
	if parentRules := s.approvalRules[parentID]; len(parentRules) > 0 {
		s.approvalRules[newID] = maps.Clone(parentRules)
	}
	s.mu.Unlock()

	// 同步落盘（不像 Append 起 goroutine）：分叉正是"原分支得以保全"这个承诺
	// 变持久的时刻，异步窗口里崩了就丢分支。
	if err := s.persistAll(); err != nil {
		log.Printf("⚠️ 分叉会话后保存本地文件失败: %v", err)
	}
	return newID, true
}

// Delete 删除指定会话（内存 + 本地文件），供 DELETE /api/sessions/:id 使用。
// 它的分支会被提升为根会话而不是级联删除——分支即拷贝，每条分支都自带完整前缀，
// 是自洽的会话，静默销毁用户分叉出来的工作正是本功能要防止的事。
func (s *SessionStore) Delete(sessionID string) {
	s.mu.Lock()
	delete(s.sessions, sessionID)
	delete(s.lastCompressIndexes, sessionID)
	delete(s.approvalRules, sessionID)
	delete(s.forkMeta, sessionID)
	for childID, fm := range s.forkMeta {
		if fm.ParentID == sessionID {
			// 只清掉父指针（ParentID 空即为根），ForkIndex 要留着：
			// 它还兼着"这条会话自己的内容从第几条开始"的职责，标题就是从那里往后找的。
			// 一并清掉的话，分支会在父会话被删的瞬间把标题改成拷贝来的前缀的第一句
			// ——用户眼里就是"我的分支莫名其妙改名了"。
			s.forkMeta[childID] = forkInfo{ForkIndex: fm.ForkIndex}
		}
	}
	s.mu.Unlock()

	if err := s.persistAll(); err != nil {
		log.Printf("⚠️ 删除会话后保存本地文件失败: %v", err)
	}
}

// ForkIndex 返回该会话从父会话拷贝过来的前缀长度（根会话为 0）。
// 统计类聚合用它跳过拷贝来的前缀，免得同一批消息被每条分支各数一遍。
func (s *SessionStore) ForkIndex(sessionID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.forkMeta[sessionID].ForkIndex
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
		// 分支标题从分岐点之后开始找：分支共享父会话的前缀，从头扫的话
		// 所有分支的标题会跟父会话一模一样，侧边栏根本分不出谁是谁。
		fm := s.forkMeta[id]
		// 钳制是防手改文件；msgs[len(msgs):] 本身是合法空切片，不会 panic
		start := min(fm.ForkIndex, len(msgs))
		title := "新对话"
		for _, m := range msgs[start:] {
			if m.Role == "user" {
				title = m.Content
				break
			}
		}
		infos = append(infos, SessionInfo{
			ID:        id,
			Title:     title,
			UpdatedAt: msgs[len(msgs)-1].Timestamp,
			ParentID:  fm.ParentID,
			ForkIndex: fm.ForkIndex,
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
	// 分支血缘：根会话两个字段都是零值，omitempty 保证它们的 wire 格式不变
	ParentID  string `json:"parent_id,omitempty"`
	ForkIndex int    `json:"fork_index,omitempty"`
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
		for sid, msgs := range store.sessions {
			// 跳过分支从父会话拷来的前缀，否则同一批消息会被每条分支各数一遍。
			// 这里直接读 forkMeta 而不调 ForkIndex()——外层已经持有 RLock，
			// 再取一次读锁遇上等待中的写者会死锁。
			if fi := store.forkMeta[sid].ForkIndex; fi > 0 && fi <= len(msgs) {
				msgs = msgs[fi:]
			}
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
