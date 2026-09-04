package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	// path 在构造时就定死，之后不再重读 RESCENE_DATA_DIR。
	// 每次现算路径会让运行期切换环境/配置后把会话写进另一个位置。
	path string
	// 用户显式设置的会话标题；未设置时前端用首条用户消息当标题。
	sessionTitles map[string]string
	// ── 最近对话快速索引（Redis ZSET 语义，纯内存）──
	// recentIndex: 会话 ID 按最近更新时间降序（类似 ZREVRANGE 0 -1）
	// updatedAt:   sessionID → 最后活动时间（ZSET 的 score）
	// 每次 Append/Upsert/Fork/Delete 增量维护，List() 与 SearchSessions()
	// 直接按序取，不再全量 sort；崩溃后由 loadFromFile 从消息重建。
	recentIndex []string
	updatedAt   map[string]time.Time

	// ── recall 倒排索引（纯内存，BM25）──
	// 写路径增量维护（Append/UpsertWorkflowPair/Fork/Delete），loadFromFile 全量重建。
	// 只存 token→会话统计不存原文，崩溃重启由消息重建，与 recentIndex 同一生存策略。
	searchIx *SessionSearchIndex

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
	// Status 任务生命周期状态（见 DSMessage.Status）。omitempty：旧存档没有这个字段，
	// 解出来是空串，由 taskDone 统一按"已完成"解读——旧数据本来就只在成功时才落盘。
	Status string `json:"status,omitempty"`
	// WorkflowID 用于失败/停止后的续跑幂等更新；老存档没有时为空，继续按追加语义处理。
	WorkflowID string `json:"workflow_id,omitempty"`
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
	// SessionTitle 是用户显式设置的会话标题；空表示未设置，回退到首条用户消息。
	SessionTitle string `json:"session_title,omitempty"`
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
			Status:           m.Status,
			WorkflowID:       m.WorkflowID,
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
			Status:           m.Status,
			WorkflowID:       m.WorkflowID,
		}
	}
	return out
}

// 旧版本地 JSON 落盘格式（PrismD 之前、多域拆分之前），仅用于一次性迁移旧数据
type legacySessionFileData struct {
	Sessions            map[string][]persistedMessage `json:"sessions"`
	LastCompressIndexes map[string]int                `json:"last_compress_indexes"`
}

// 全局会话存储引用，供 session_search 等工具访问
var globalSessionStore *SessionStore

// NewSessionStore 创建一个绑定到指定域（如 ChatSessionsDomain）的会话存储。
// 启动时从该域对应的本地文件加载已有会话；如果该文件不存在且发现更早期的
// 单文件旧版格式（sessions.json，PrismD 迁移前遗留），会做一次性迁移。
func NewSessionStore(domain string) *SessionStore {
	store := &SessionStore{
		sessions:            make(map[string][]DSMessage),
		lastCompressIndexes: make(map[string]int),
		approvalRules:       make(map[string]map[string]bool),
		forkMeta:            make(map[string]forkInfo),
		sessionTitles:       make(map[string]string),
		updatedAt:           make(map[string]time.Time),
		searchIx:            newSessionSearchIndex(),
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
// RESCENE_DATA_DIR 环境变量覆盖（测试/多实例场景）。
func sessionsFilePath(domain string) string {
	dataDir := os.Getenv("RESCENE_DATA_DIR")
	if dataDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			homeDir = "."
		}
		dataDir = filepath.Join(homeDir, "rescene_data")
	}
	return filepath.Join(dataDir, "sessions_"+domain+".json")
}

// ── 最近对话快速索引（Redis ZSET 语义）──────────────────────────────
// 调用约定：所有方法都要求调用方已持有写锁（s.mu.Lock），内部不再加锁。

// rebuildRecentIndex 从 sessions 全量重建索引（启动加载后调用一次）。
// 排序键 = 会话最后一条消息的时间戳；空会话（惰性建表）时间零值沉底。
func (s *SessionStore) rebuildRecentIndex() {
	s.recentIndex = s.recentIndex[:0]
	s.updatedAt = make(map[string]time.Time, len(s.sessions))
	for sid, msgs := range s.sessions {
		if len(msgs) == 0 {
			continue
		}
		last := msgs[len(msgs)-1].Timestamp
		if last.IsZero() {
			// 旧数据迁移常见：时间戳零值。用首条时间兜底，还为零就跳过索引
			for _, m := range msgs {
				if !m.Timestamp.IsZero() {
					last = m.Timestamp
					break
				}
			}
		}
		if last.IsZero() {
			continue
		}
		s.updatedAt[sid] = last
		s.recentIndex = append(s.recentIndex, sid)
	}
	sort.SliceStable(s.recentIndex, func(i, j int) bool {
		return s.updatedAt[s.recentIndex[i]].After(s.updatedAt[s.recentIndex[j]])
	})
}

// touchRecent 将会话的最近活动时间更新为 t 并移到正确位置（最新在前）。
// 新会话直接插入；已存在的先移除旧位再按时间二分插入。
func (s *SessionStore) touchRecent(sid string, t time.Time) {
	if t.IsZero() {
		t = time.Now()
	}
	old, existed := s.updatedAt[sid]
	s.updatedAt[sid] = t
	if !existed {
		// 二分找到第一个时间 ≤ t 的位置（降序），插到它前面
		pos := sort.Search(len(s.recentIndex), func(i int) bool {
			return s.updatedAt[s.recentIndex[i]].Before(t) || s.updatedAt[s.recentIndex[i]].Equal(t)
		})
		s.recentIndex = append(s.recentIndex, "")
		copy(s.recentIndex[pos+1:], s.recentIndex[pos:])
		s.recentIndex[pos] = sid
		return
	}
	if old.Equal(t) {
		return // 时间没变，位置不动
	}
	// 移除旧位置
	pos := -1
	for i, id := range s.recentIndex {
		if id == sid {
			pos = i
			break
		}
	}
	if pos >= 0 {
		s.recentIndex = append(s.recentIndex[:pos], s.recentIndex[pos+1:]...)
	}
	// 插入新位置（时间只会往后走，一般直接到最前）
	npos := sort.Search(len(s.recentIndex), func(i int) bool {
		return s.updatedAt[s.recentIndex[i]].Before(t) || s.updatedAt[s.recentIndex[i]].Equal(t)
	})
	s.recentIndex = append(s.recentIndex, "")
	copy(s.recentIndex[npos+1:], s.recentIndex[npos:])
	s.recentIndex[npos] = sid
}

// removeRecent 从索引中移除一个会话（Delete 时调用）。
func (s *SessionStore) removeRecent(sid string) {
	delete(s.updatedAt, sid)
	for i, id := range s.recentIndex {
		if id == sid {
			s.recentIndex = append(s.recentIndex[:i], s.recentIndex[i+1:]...)
			return
		}
	}
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
		if rec.SessionTitle != "" {
			s.sessionTitles[sid] = rec.SessionTitle
		}
	}
	// 加载完重建最近对话索引（ZSET：sessionID → 最后消息时间，降序）
	s.rebuildRecentIndex()
	// 重建 recall 倒排索引（BM25）：全量灌一遍，之后写路径增量维护
	if s.searchIx == nil {
		s.searchIx = newSessionSearchIndex()
	}
	for sid, msgs := range s.sessions {
		s.searchIx.addDoc(sid, msgs)
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
	legacyPath := filepath.Join(homeDir, "rescene_data", "sessions.json")
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
	s.rebuildRecentIndex()
	s.mu.Unlock()

	if err := s.persistAll(); err != nil {
		log.Printf("⚠️ 迁移会话到本地文件失败: %v", err)
		return
	}
	// 迁移成功后把旧文件改名为 .migrated 收尾，否则会留下一个隐蔽的「删不掉」陷阱：
	// 用户之后删光本域会话（sessions_<domain>.json 变空），下次启动时
	// NewSessionStore 看到 len(store.sessions)==0 又会触发一次迁移，刚删掉的旧对话全部复活。
	// 改名而非删除：原始数据保留一份可手动找回，但 migrate 不会再读到它。
	if err := os.Rename(legacyPath, legacyPath+".migrated"); err != nil {
		log.Printf("⚠️ 迁移后清理旧版会话文件失败（不影响迁移结果）: %v", err)
	}
	log.Printf("✅ 会话迁移完成")
}

// persistAll 把内存中该域的全部会话整份写入本地文件（原子替换，避免半写损坏）
func (s *SessionStore) persistAll() error {
	// 从取内存快照开始就串行化。若只在最终写文件时加锁，先取到的旧快照
	// 可能晚于新快照落盘，把较新的会话状态反向覆盖。
	s.fileMu.Lock()
	defer s.fileMu.Unlock()

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
			SessionTitle:  s.sessionTitles[sid],
		}
	}
	s.mu.RUnlock()

	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}

	path := s.path // 构造时定死，迟到的落盘 goroutine 也只会写这一个位置
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	// 复用文件工具的跨平台安全写入：Windows 的 os.Rename 无法可靠覆盖已有目标，
	// 原实现会导致第二次会话落盘开始持续 Access is denied。
	return atomicWriteNative(path, data, 0644)
}

// Append 追加消息，自动补时间戳并同步持久化到本地文件。
// 同步是有意的：异步整文件重写会出现旧 goroutine 晚到、测试/刷新立即读到半写 JSON，
// 甚至客户端退出前最后一轮还没落盘。工作流主链已改为一次 Upsert 一组消息，写频率很低。
func (s *SessionStore) Append(sessionID string, msg DSMessage) {
	s.mu.Lock()
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	s.sessions[sessionID] = append(s.sessions[sessionID], msg)
	// 最近对话索引：这条消息是新的最后活动
	s.touchRecent(sessionID, msg.Timestamp)
	// recall 倒排索引：增量灌这条消息（addDoc 全量替换该会话，消息量小可接受；
	// 若以后会话上千条消息再换纯增量 append）
	s.searchIx.addDoc(sessionID, s.sessions[sessionID])
	s.mu.Unlock()

	if err := s.persistAll(); err != nil {
		log.Printf("⚠️ 保存会话到本地文件失败: %v", err)
	}
	// 主页统计云端同步：异步上报增量（不阻塞落盘/对话）
	reportStatsAsync(msg)
}

// UpsertWorkflowPair 按 workflowID 原位写入一组 user/assistant 消息。
//
// 首次失败/停止时追加；同一 workflow 从检查点续跑后再次收尾时替换原来的两条，
// 因而状态可以从 interrupted/failed 变成 completed，聊天历史不会出现重复任务。
func (s *SessionStore) UpsertWorkflowPair(sessionID, workflowID string, user, assistant DSMessage) {
	if sessionID == "" || workflowID == "" {
		return
	}
	now := time.Now()
	if user.Timestamp.IsZero() {
		user.Timestamp = now
	}
	if assistant.Timestamp.IsZero() {
		assistant.Timestamp = now
	}
	user.WorkflowID = workflowID
	assistant.WorkflowID = workflowID

	s.mu.Lock()
	msgs := s.sessions[sessionID]
	first := -1
	existingCompleted := false
	filtered := make([]DSMessage, 0, len(msgs)+2)
	for _, msg := range msgs {
		if msg.WorkflowID == workflowID {
			if msg.Role == "user" && msg.Status == taskStatusCompleted {
				existingCompleted = true
			}
			if first < 0 {
				first = len(filtered)
			}
			continue
		}
		filtered = append(filtered, msg)
	}
	// 极窄竞态：旧的中断请求还在 defer，新续跑已经完成。完成态是终局，
	// 迟到的 failed/interrupted 绝不能把它降级回未完成。
	if existingCompleted && user.Status != taskStatusCompleted {
		s.mu.Unlock()
		return
	}
	if first < 0 {
		first = len(filtered)
	}
	filtered = append(filtered, DSMessage{}, DSMessage{})
	copy(filtered[first+2:], filtered[first:len(filtered)-2])
	filtered[first] = user
	filtered[first+1] = assistant
	s.sessions[sessionID] = filtered
	// 最近对话索引：工作流收尾即最后活动
	s.touchRecent(sessionID, now)
	// recall 倒排索引：工作流对子替换后重灌该会话
	s.searchIx.addDoc(sessionID, filtered)
	s.mu.Unlock()

	// 工作流已经进入终态，停止接口会等待这里完成后再返回；同步落盘确保用户随后
	// 关闭客户端或立即发下一条消息时，失败/中断上下文已经可靠可见。
	if err := s.persistAll(); err != nil {
		log.Printf("⚠️ 保存工作流历史失败: %v", err)
	}
	// 主页统计云端同步：user + assistant 两条都上报（异步，不阻塞）
	reportStatsAsync(user)
	reportStatsAsync(assistant)
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
	// 最近对话索引：新分支立即进入索引（时间 = 分叉时刻，最新）
	s.touchRecent(newID, time.Now())
	// recall 倒排索引：分支内容 = 拷贝的前缀，一并灌进索引
	s.searchIx.addDoc(newID, copied)
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
	// 最近对话索引：删除即移除
	s.removeRecent(sessionID)
	// recall 倒排索引：删除即摘除
	s.searchIx.removeDoc(sessionID)
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

// SetApprovalRule 写入（或清除）该会话对某工具签名的「don't ask again」规则并落盘。
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

	if err := s.persistAll(); err != nil {
		log.Printf("⚠️ 保存审批规则到本地文件失败: %v", err)
	}
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

// SetCompressIndex 更新压缩游标（通常在成功压缩后调用）并持久化到本地文件。
func (s *SessionStore) SetCompressIndex(sessionID string, index int) {
	s.mu.Lock()
	s.lastCompressIndexes[sessionID] = index
	s.mu.Unlock()

	if err := s.persistAll(); err != nil {
		log.Printf("⚠️ 保存压缩游标到本地文件失败: %v", err)
	}
}

// SetSessionTitle 设置用户显式标题；空字符串表示清除显式标题，回退到默认。
func (s *SessionStore) SetSessionTitle(sessionID, title string) {
	if sessionID == "" {
		return
	}
	s.mu.Lock()
	if title == "" {
		delete(s.sessionTitles, sessionID)
	} else {
		s.sessionTitles[sessionID] = title
	}
	s.mu.Unlock()

	if err := s.persistAll(); err != nil {
		log.Printf("⚠️ 保存会话标题失败: %v", err)
	}
}

// SessionTitle 返回会话标题：显式设置优先，否则从首条用户消息派生。
func (s *SessionStore) SessionTitle(sessionID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if t, ok := s.sessionTitles[sessionID]; ok && t != "" {
		return t
	}
	msgs := s.sessions[sessionID]
	fm := s.forkMeta[sessionID]
	start := min(fm.ForkIndex, len(msgs))
	for _, m := range msgs[start:] {
		if m.Role == "user" && strings.TrimSpace(m.Content) != "" {
			return strings.TrimSpace(m.Content)
		}
	}
	return "新对话"
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

// SearchResult 一条会话搜索匹配结果
type SearchResult struct {
	SessionID string    `json:"session_id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Role      string    `json:"role"`
	Timestamp time.Time `json:"timestamp"`
	Model     string    `json:"model,omitempty"`
}

// SessionSearchHit 一个命中会话（recall 前端展示用）：
// 按会话聚合的搜索结果，含会话信息 + 命中消息片段列表。
type SessionSearchHit struct {
	SessionID    string         `json:"session_id"`
	Title        string         `json:"title"`
	UpdatedAt    time.Time      `json:"updated_at"`
	MessageCount int            `json:"message_count"`
	Hits         []SearchResult `json:"hits"` // 命中的消息片段，最多 3 条
}

// SearchSessionsBySession 搜索全部会话并按会话聚合返回命中会话。
// 走内存倒排索引（BM25 打分）：中文 bigram 分词，搜「鉴权」命中「鉴权方案」，
// 相关度降序。O(命中) 复杂度，不扫全量消息。每会话最多 3 条命中片段；
// limit 限制返回的会话数。
func (s *SessionStore) SearchSessionsBySession(query string, limit int) []SessionSearchHit {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	q := strings.TrimSpace(query)
	if q == "" {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// BM25 打分出候选会话（相关度降序）
	scores := s.searchIx.search(q)
	if len(scores) == 0 {
		return make([]SessionSearchHit, 0)
	}
	type scored struct {
		sid   string
		score float64
	}
	cands := make([]scored, 0, len(scores))
	for sid, sc := range scores {
		if msgs := s.sessions[sid]; len(msgs) == 0 {
			continue
		}
		cands = append(cands, scored{sid, sc})
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].score > cands[j].score })

	// 排序后回查原文取片段。优先用查询原句做字面定位；
	// bigram 召回（原句不连续出现）时退回最长命中词元定位。
	qLower := strings.ToLower(q)
	out := make([]SessionSearchHit, 0, limit)
	for _, cd := range cands {
		if len(out) >= limit {
			break
		}
		sid := cd.sid
		msgs := s.sessions[sid]
		title := s.sessionTitleLocked(sid)
		if len([]rune(title)) > 40 {
			title = string([]rune(title)[:40]) + "…"
		}
		hit := SessionSearchHit{
			SessionID:    sid,
			Title:        title,
			UpdatedAt:    s.updatedAt[sid],
			MessageCount: len(msgs),
		}
		// 该会话的片段定位词：优先查询原句；原句不连续出现（bigram 召回）
		// 时退回查询分词里最长的、在该会话原文里真实命中的词元。
		matchQ := qLower
		if !containsAllText(msgs, matchQ) {
			for _, t := range tokenize(q) {
				if len([]rune(t)) >= 2 && containsAllText(msgs, t) {
					matchQ = t
					break
				}
			}
		}
		for _, msg := range msgs {
			content := msg.Content
			if content == "" || !strings.Contains(strings.ToLower(content), matchQ) {
				continue
			}
			idx := strings.Index(strings.ToLower(content), matchQ)
			start := idx - 60
			if start < 0 {
				start = 0
			}
			end := idx + len(q) + 60
			if end > len(content) {
				end = len(content)
			}
			snippet := content[start:end]
			if start > 0 {
				snippet = "…" + snippet
			}
			if end < len(content) {
				snippet = snippet + "…"
			}
			hit.Hits = append(hit.Hits, SearchResult{
				SessionID: sid,
				Title:     title,
				Content:   snippet,
				Role:      msg.Role,
				Timestamp: msg.Timestamp,
				Model:     msg.Model,
			})
			if len(hit.Hits) >= 3 {
				break
			}
		}
		if len(hit.Hits) == 0 {
			continue
		}
		out = append(out, hit)
	}
	return out
}

// sessionTitleLocked 返回会话标题（调用方需持有读锁）。
func (s *SessionStore) sessionTitleLocked(sid string) string {
	if t, ok := s.sessionTitles[sid]; ok && t != "" {
		return t
	}
	msgs := s.sessions[sid]
	fm := s.forkMeta[sid]
	start := min(fm.ForkIndex, len(msgs))
	for _, m := range msgs[start:] {
		if m.Role == "user" && strings.TrimSpace(m.Content) != "" {
			return strings.TrimSpace(m.Content)
		}
	}
	return "新对话"
}

// SearchSessions 在全部会话中搜索文本，返回匹配的消息片段。
// 按最近对话索引顺序扫描（最新的先命中），query 大小写不敏感，
// limit 限制返回条数（默认 10，最大 50）。
func (s *SessionStore) SearchSessions(query string, limit int) []SearchResult {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []SearchResult
	for _, sid := range s.recentIndex {
		msgs := s.sessions[sid]
		title := s.sessionTitleLocked(sid)
		if len([]rune(title)) > 40 {
			title = string([]rune(title)[:40]) + "…"
		}
		for _, msg := range msgs {
			content := msg.Content
			if content == "" {
				continue
			}
			if !strings.Contains(strings.ToLower(content), q) {
				continue
			}
			// 截取匹配片段：前后各取 60 字
			idx := strings.Index(strings.ToLower(content), q)
			start := idx - 60
			if start < 0 {
				start = 0
			}
			end := idx + len(q) + 60
			if end > len(content) {
				end = len(content)
			}
			snippet := content[start:end]
			// 如果不是从头开始，加 …
			if start > 0 {
				snippet = "…" + snippet
			}
			if end < len(content) {
				snippet = snippet + "…"
			}

			results = append(results, SearchResult{
				SessionID: sid,
				Title:     title,
				Content:   snippet,
				Role:      msg.Role,
				Timestamp: msg.Timestamp,
				Model:     msg.Model,
			})
			if len(results) >= limit {
				return results
			}
		}
	}
	return results
}

// RecentSessionItem 最近对话浏览条目：会话摘要 + 最近几条消息内容
// （Hermes session_search 的 BROWSE 模式：无关键词直接看最近聊了什么）。
type RecentSessionItem struct {
	SessionID    string    `json:"session_id"`
	Title        string    `json:"title"`
	UpdatedAt    time.Time `json:"updated_at"`
	MessageCount int       `json:"message_count"`
	// Recent 最近几条消息（按时间正序，最多 preview 条）
	Recent []SearchResult `json:"recent"`
}

// RecentSessions 返回最近对话列表（按更新时间降序），每个会话带最近 preview 条消息。
// 直接走 recentIndex 有序表，O(limit) 出结果，不扫描全量会话。
func (s *SessionStore) RecentSessions(limit, preview int) []RecentSessionItem {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	if preview <= 0 {
		preview = 3
	}
	if preview > 10 {
		preview = 10
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]RecentSessionItem, 0, limit)
	for _, sid := range s.recentIndex {
		msgs := s.sessions[sid]
		if len(msgs) == 0 {
			continue
		}
		title := s.sessionTitleLocked(sid)
		if len([]rune(title)) > 40 {
			title = string([]rune(title)[:40]) + "…"
		}
		item := RecentSessionItem{
			SessionID:    sid,
			Title:        title,
			UpdatedAt:    s.updatedAt[sid],
			MessageCount: len(msgs),
		}
		// 取最近 preview 条非空消息
		start := len(msgs) - preview
		if start < 0 {
			start = 0
		}
		for _, m := range msgs[start:] {
			if strings.TrimSpace(m.Content) == "" {
				continue
			}
			content := m.Content
			if len([]rune(content)) > 120 {
				content = string([]rune(content)[:120]) + "…"
			}
			item.Recent = append(item.Recent, SearchResult{
				SessionID: sid,
				Title:     title,
				Content:   content,
				Role:      m.Role,
				Timestamp: m.Timestamp,
				Model:     m.Model,
			})
		}
		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}
	return out
}

// ReadSession 读取单个会话的最近 limit 条消息（Hermes session_search 的 READ 模式）。
// 会话不存在返回 nil。
func (s *SessionStore) ReadSession(sessionID string, limit int) []SearchResult {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	msgs := s.sessions[sessionID]
	if len(msgs) == 0 {
		return nil
	}
	title := s.sessionTitleLocked(sessionID)
	if len([]rune(title)) > 40 {
		title = string([]rune(title)[:40]) + "…"
	}
	start := len(msgs) - limit
	if start < 0 {
		start = 0
	}
	out := make([]SearchResult, 0, len(msgs)-start)
	for _, m := range msgs[start:] {
		if strings.TrimSpace(m.Content) == "" {
			continue
		}
		out = append(out, SearchResult{
			SessionID: sessionID,
			Title:     title,
			Content:   m.Content,
			Role:      m.Role,
			Timestamp: m.Timestamp,
			Model:     m.Model,
		})
	}
	return out
}

// List 列出所有会话摘要（按最近对话索引顺序，无需全量排序）
func (s *SessionStore) List() []SessionInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	infos := make([]SessionInfo, 0, len(s.recentIndex))
	for _, id := range s.recentIndex {
		msgs := s.sessions[id]
		if len(msgs) == 0 {
			continue
		}
		fm := s.forkMeta[id]
		_, titleSet := s.sessionTitles[id]
		infos = append(infos, SessionInfo{
			ID:            id,
			Title:         s.sessionTitleLocked(id),
			UpdatedAt:     msgs[len(msgs)-1].Timestamp,
			ParentID:      fm.ParentID,
			ForkIndex:     fm.ForkIndex,
			TitleExplicit: titleSet && s.sessionTitles[id] != "",
		})
	}
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
	// 该会话是否有「显式设置」的标题（用户手动改名过 / AI 标题已写入）。
	// 前端据此决定是否允许 AI 标题覆盖：没显式设置过才能覆盖，
	// 否则用户手改的标题会被 AI 生成结果顶掉。
	TitleExplicit bool `json:"title_explicit,omitempty"`
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
