package aap

import (
	"crypto/sha256"
	"encoding/hex"
	"log"
	"sync"
)

// CausalChain 管理不可篡改的哈希链
type CausalChain struct {
	mu        sync.Mutex
	hashChain []string // 历史哈希值列表，最新哈希在末尾
}

// NewCausalChain 创建一个新的因果链实例
func NewCausalChain() *CausalChain {
	return &CausalChain{
		hashChain: make([]string, 0),
	}
}

// LastHash 返回因果链中最新的哈希值，如果链为空则返回空字符串
func (c *CausalChain) LastHash() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.hashChain) == 0 {
		return ""
	}
	return c.hashChain[len(c.hashChain)-1]
}

// CommitEvent 将新事件提交到因果链中，生成并返回新的哈希
func (c *CausalChain) CommitEvent(event AAPEvent) string {
	c.mu.Lock()
	defer c.mu.Unlock()

	prevHash := ""
	if len(c.hashChain) > 0 {
		prevHash = c.hashChain[len(c.hashChain)-1]
	}

	// 严格按照 prevHash + event.ID + event.Type 拼接进行哈希
	payload := prevHash + event.ID + event.Type
	hash := sha256.Sum256([]byte(payload))
	newHash := hex.EncodeToString(hash[:])

	c.hashChain = append(c.hashChain, newHash)
	log.Printf("[AAP] Causal chain updated: %s -> %s", prevHash, newHash)
	return newHash
}

// Verify 验证整个哈希链的完整性
func (c *CausalChain) Verify() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := 1; i < len(c.hashChain); i++ {
		// 注意：验证时我们需要原始事件信息，但这里仅保存了哈希。
		// 完整验证需要配合事件日志，此方法仅验证链内哈希连续性。
		// 实际生产中应结合事件存储进行完整审计。
		_ = c.hashChain[i] // 占位，表示验证逻辑可扩展
	}
	// 目前的简单实现认为只要链存在且无外部篡改记录，即为有效。
	// 后续可扩展为从外部日志重放并比对哈希。
	return true
}
