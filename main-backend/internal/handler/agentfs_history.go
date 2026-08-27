package handler

// AgentFS 本地历史存储引擎（VS Code Timeline 风格）。
//
// 设计要点：
//   - AI 直接改真实项目文件，AgentFS 在每次写操作前把 before 内容保存到本地。
//   - 内容按 sha256 寻址并 gzip 压缩，重复内容只存一份。
//   - 审计时间线只记录元数据（路径、hash、blob 引用），不存完整内容。
//   - 完全不走 git，回滚 = 从本地 blob 还原 before 内容。
//   - GC 按版本数 / 总大小 / 年龄回收，避免大项目膨胀。

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pmezard/go-difflib/difflib"
)

// historyMu 保护 audit.jsonl 写入与 GC，避免并发导致行错乱。
var historyMu sync.Mutex

// agentfsHistoryDir 项目历史数据根目录。
func agentfsHistoryDir(project string) string {
	return filepath.Join(agentfsRoot(), "history", project)
}

// agentfsAuditPath 审计日志路径。
func agentfsAuditPath(project string) string {
	return filepath.Join(agentfsHistoryDir(project), "audit.jsonl")
}

// agentfsBlobDir 内容寻址 blob 根目录。
func agentfsBlobDir(project string) string {
	return filepath.Join(agentfsHistoryDir(project), "blobs")
}

// agentfsBlobPath 返回某个 hash 对应的本地文件路径。
func agentfsBlobPath(project, hash string) string {
	if len(hash) < 2 {
		return filepath.Join(agentfsBlobDir(project), hash)
	}
	return filepath.Join(agentfsBlobDir(project), hash[:2], hash)
}

// agentfsAudit 审计时间线的一行。
// Commit/Branch/ParentCommit 为兼容前端而保留的"伪 git"标识，不代表真实 git 提交。
type agentfsAudit struct {
	Seq          int       `json:"seq"`
	TS           time.Time `json:"ts"`
	Op           string    `json:"op"`       // write / edit
	RelPath      string    `json:"rel_path"` // 相对工作目录的路径
	BeforeHash   string    `json:"before_hash"`
	AfterHash    string    `json:"after_hash"`
	BeforeBlob   string    `json:"before_blob,omitempty"` // 本次写之前的文件状态 hash（空表示文件此前不存在）
	AfterBlob    string    `json:"after_blob,omitempty"`  // 本次写之后的文件状态 hash
	Commit       string    `json:"commit"`                // 伪提交标识，供前端时间线显示用
	ParentCommit string    `json:"parent_commit,omitempty"`
	Branch       string    `json:"branch,omitempty"`
	Tool         string    `json:"tool"` // write_file / edit_file（兼容外部 mcp__fs__*）
	SessionID    string    `json:"session_id"`
	ExistsBefore bool      `json:"exists_before"`
}

// historyStore 单个项目的历史存储。
type historyStore struct {
	project string
}

func newHistoryStore(project string) *historyStore {
	return &historyStore{project: project}
}

// saveBlob 把数据 gzip 压缩后按 sha256 内容寻址落盘；已存在则跳过。
func (s *historyStore) saveBlob(data []byte) (string, error) {
	hash := sha256Of(data)
	path := agentfsBlobPath(s.project, hash)
	if _, err := os.Stat(path); err == nil {
		return hash, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(data); err != nil {
		gw.Close()
		return "", err
	}
	if err := gw.Close(); err != nil {
		return "", err
	}
	compressed := buf.Bytes()

	// 优先原子写入（tmp → rename）；Windows 偶发访问拒绝时兜底直接写。
	tmp := path + "." + fmt.Sprintf("%d", time.Now().UnixNano()) + ".tmp"
	if err := os.WriteFile(tmp, compressed, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		if err := os.WriteFile(path, compressed, 0o644); err != nil {
			return "", err
		}
	}
	return hash, nil
}

// loadBlob 读取并解压指定 hash 的 blob。
func (s *historyStore) loadBlob(hash string) ([]byte, error) {
	path := agentfsBlobPath(s.project, hash)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gr.Close()
	return io.ReadAll(gr)
}

// RecordWrite 记录一次写操作：保存 before/after blob、追加审计日志。
func (s *historyStore) RecordWrite(sess *agentfsSession, op, relPath, tool string, before, after []byte, existsBefore bool) (*agentfsAudit, error) {
	beforeHash := ""
	afterHash := sha256Of(after)
	beforeBlob := ""
	afterBlob := ""

	if existsBefore {
		beforeHash = sha256Of(before)
		h, err := s.saveBlob(before)
		if err != nil {
			return nil, fmt.Errorf("保存 before blob 失败: %w", err)
		}
		beforeBlob = h
	}
	{
		h, err := s.saveBlob(after)
		if err != nil {
			return nil, fmt.Errorf("保存 after blob 失败: %w", err)
		}
		afterBlob = h
	}

	audit := &agentfsAudit{
		Seq:          sess.Seq,
		TS:           time.Now(),
		Op:           op,
		RelPath:      relPath,
		BeforeHash:   beforeHash,
		AfterHash:    afterHash,
		BeforeBlob:   beforeBlob,
		AfterBlob:    afterBlob,
		Commit:       fmt.Sprintf("afs-%d", sess.Seq),
		Branch:       "main",
		Tool:         tool,
		SessionID:    sess.SessionID,
		ExistsBefore: existsBefore,
	}

	historyMu.Lock()
	defer historyMu.Unlock()

	ap := agentfsAuditPath(s.project)
	if err := os.MkdirAll(filepath.Dir(ap), 0o755); err != nil {
		return nil, err
	}
	buf, err := json.Marshal(audit)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(ap, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	_, _ = f.Write(append(buf, '\n'))
	_ = f.Close()

	// 异步触发 GC，避免阻塞写操作主流程。
	go s.gcIfNeeded()

	return audit, nil
}

// List 读取审计时间线，可选按 sessionID 过滤。
func (s *historyStore) List(sessionID string) ([]agentfsAudit, error) {
	historyMu.Lock()
	defer historyMu.Unlock()

	ap := agentfsAuditPath(s.project)
	data, err := os.ReadFile(ap)
	if err != nil {
		if os.IsNotExist(err) {
			return []agentfsAudit{}, nil
		}
		return nil, err
	}
	var out []agentfsAudit
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var a agentfsAudit
		if json.Unmarshal([]byte(line), &a) != nil {
			continue
		}
		if sessionID != "" && a.SessionID != sessionID {
			continue
		}
		out = append(out, a)
	}
	return out, nil
}

// Find 按 seq 查找审计记录。
func (s *historyStore) Find(seq int) (*agentfsAudit, error) {
	all, err := s.List("")
	if err != nil {
		return nil, err
	}
	for i := range all {
		if all[i].Seq == seq {
			return &all[i], nil
		}
	}
	return nil, fmt.Errorf("未找到 seq=%d 的审计记录", seq)
}

// Restore 返回某条审计记录对应的 after 内容（即该次写操作完成后的文件状态）。
func (s *historyStore) Restore(seq int) ([]byte, error) {
	a, err := s.Find(seq)
	if err != nil {
		return nil, err
	}
	if a.AfterBlob == "" {
		return nil, fmt.Errorf("该记录没有保存 after 状态")
	}
	return s.loadBlob(a.AfterBlob)
}

// RestoreBefore 返回某条审计记录写操作之前的文件内容（回退到工作流前版本）。
// 返回 nil,nil 表示该文件在本次写之前不存在（即文件是本次写新建的，回退=删除）。
func (s *historyStore) RestoreBefore(seq int) ([]byte, error) {
	a, err := s.Find(seq)
	if err != nil {
		return nil, err
	}
	if !a.ExistsBefore {
		return nil, nil
	}
	if a.BeforeBlob == "" {
		return nil, fmt.Errorf("该记录没有保存 before 状态")
	}
	return s.loadBlob(a.BeforeBlob)
}

// Diff 生成某条审计记录本身对应的 unified diff（before → after）。
func (s *historyStore) Diff(workdir, relPath string, seq int) (string, error) {
	a, err := s.Find(seq)
	if err != nil {
		return "", err
	}
	var before []byte
	if a.ExistsBefore {
		before, err = s.loadBlob(a.BeforeBlob)
		if err != nil {
			return "", fmt.Errorf("读取历史版本失败: %w", err)
		}
	}
	after, err := s.loadBlob(a.AfterBlob)
	if err != nil {
		// 兜底：after blob 不存在时读当前真实盘
		after, err = os.ReadFile(filepath.Join(workdir, relPath))
		if err != nil {
			return "", fmt.Errorf("读取当前文件失败: %w", err)
		}
	}
	return unifiedDiff(string(before), string(after), relPath+" (before)", relPath+" (after)")
}

// unifiedDiff 生成统一格式 diff。
func unifiedDiff(a, b, from, to string) (string, error) {
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(a),
		B:        difflib.SplitLines(b),
		FromFile: from,
		ToFile:   to,
		Context:  3,
	}
	var buf bytes.Buffer
	if err := difflib.WriteUnifiedDiff(&buf, diff); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// gcIfNeeded 在写入后视情况触发 GC。
func (s *historyStore) gcIfNeeded() {
	// 简单策略：每次写入都尝试 GC；GC 内部会快速判断是否需要实际清理。
	if err := s.gc(defaultGCConfig()); err != nil {
		log.Printf("⚠️ AgentFS GC 失败 %s: %v", s.project, err)
	}
}

// gcConfig GC 策略配置。
type gcConfig struct {
	MaxEntriesPerFile int
	MaxTotalBytes     int64
	MaxAge            time.Duration
}

// defaultGCConfig 默认 GC 策略：
//   - 每个文件最多保留 50 个版本
//   - 单个项目历史总大小不超过 500MB
//   - 超过 30 天的版本清理
func defaultGCConfig() gcConfig {
	return gcConfig{
		MaxEntriesPerFile: 50,
		MaxTotalBytes:     500 * 1024 * 1024,
		MaxAge:            30 * 24 * time.Hour,
	}
}

// gc 执行垃圾回收：删除超限审计记录及其未引用的 blob。
func (s *historyStore) gc(cfg gcConfig) error {
	historyMu.Lock()
	defer historyMu.Unlock()

	ap := agentfsAuditPath(s.project)
	data, err := os.ReadFile(ap)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var entries []agentfsAudit
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var a agentfsAudit
		if json.Unmarshal([]byte(line), &a) == nil {
			entries = append(entries, a)
		}
	}

	cutoff := time.Now().Add(-cfg.MaxAge)
	byPath := make(map[string][]agentfsAudit)
	for i := len(entries) - 1; i >= 0; i-- {
		a := entries[i]
		if a.TS.Before(cutoff) {
			continue // 超龄丢弃
		}
		byPath[a.RelPath] = append(byPath[a.RelPath], a)
	}

	var kept []agentfsAudit
	for _, list := range byPath {
		// 按时间从新到旧排序，保留前 MaxEntriesPerFile 个
		sort.Slice(list, func(i, j int) bool {
			return list[i].TS.After(list[j].TS)
		})
		if len(list) > cfg.MaxEntriesPerFile {
			list = list[:cfg.MaxEntriesPerFile]
		}
		kept = append(kept, list...)
	}
	sort.Slice(kept, func(i, j int) bool {
		return kept[i].Seq < kept[j].Seq
	})

	// 如果总大小仍超限，从最早的开始删
	for totalBlobSize(s.project) > cfg.MaxTotalBytes && len(kept) > 0 {
		kept = kept[1:]
	}

	// 重写 audit.jsonl
	if err := os.MkdirAll(filepath.Dir(ap), 0o755); err != nil {
		return err
	}
	tmp := ap + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	for _, a := range kept {
		buf, _ := json.Marshal(a)
		_, _ = f.Write(append(buf, '\n'))
	}
	_ = f.Close()
	if err := os.Rename(tmp, ap); err != nil {
		return err
	}

	// 清理未被引用的 blob
	referenced := make(map[string]bool)
	for _, a := range kept {
		if a.BeforeBlob != "" {
			referenced[a.BeforeBlob] = true
		}
		if a.AfterBlob != "" {
			referenced[a.AfterBlob] = true
		}
	}
	return pruneBlobs(s.project, referenced)
}

// totalBlobSize 计算项目 blob 目录总大小。
func totalBlobSize(project string) int64 {
	var total int64
	_ = filepath.Walk(agentfsBlobDir(project), func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}

// pruneBlobs 删除未被引用的 blob 文件。
func pruneBlobs(project string, referenced map[string]bool) error {
	blobDir := agentfsBlobDir(project)
	if _, err := os.Stat(blobDir); os.IsNotExist(err) {
		return nil
	}
	return filepath.Walk(blobDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		hash := filepath.Base(path)
		if !referenced[hash] {
			_ = os.Remove(path)
		}
		return nil
	})
}
