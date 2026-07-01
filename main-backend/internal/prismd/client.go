// internal/prismd/client.go
package prismd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client 是 PrismD 服务的 Go 语言驱动，通过 HTTP 发送 PrimQL 原语。
type Client struct {
	baseURL string
	cli     *http.Client
}

// NewClient 创建 PrismD 客户端，addr 格式如 "localhost:5666"。
const DefaultClientTimeout = 60 * time.Second

func NewClient(addr string) *Client {
	return &Client{
		baseURL: "http://" + addr,
		cli: &http.Client{
			Timeout: DefaultClientTimeout,
		},
	}
}

// Engrave 刻入一条新记忆。
func (c *Client) Engrave(role, content string) (string, error) {
	body := fmt.Sprintf("ENGRAM %s %s", role, content)
	return c.do(body)
}

// Loom 检索涌现与 query 相关的记忆。
func (c *Client) Loom(query string) (string, error) {
	body := fmt.Sprintf("LOOM %s", query)
	return c.do(body)
}

// Refract 更新并强化指定记忆的电导及内容。
func (c *Client) Refract(id uint64, content, role string, energy float64) (string, error) {
	params := map[string]interface{}{
		"id": id,
	}
	if content != "" {
		params["content"] = content
	}
	if role != "" {
		params["role"] = role
	}
	if energy > 0 {
		params["energy"] = energy
	}
	jsonData, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("refract marshal: %w", err)
	}
	body := fmt.Sprintf("REFRACT %s", string(jsonData))
	return c.do(body)
}

// Prune 主动遗忘指定记忆。
func (c *Client) Prune(id uint64) (string, error) {
	body := fmt.Sprintf("PRUNE %d", id)
	return c.do(body)
}

// Drift 触发一次全局演化，衰减长期未激活的记忆。
func (c *Client) Drift() (string, error) {
	return c.do("DRIFT")
}

// Stats 获取记忆场当前状态。
func (c *Client) Stats() (string, error) {
	return c.do("STATS")
}

// do 发送 HTTP POST 请求，返回响应体文本。
func (c *Client) do(body string) (string, error) {
	req, err := http.NewRequest(http.MethodPost, c.baseURL, bytes.NewBufferString(body))
	if err != nil {
		return "", fmt.Errorf("prismd request: %w", err)
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := c.cli.Do(req)
	if err != nil {
		return "", fmt.Errorf("prismd call: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("prismd read: %w", err)
	}
	respStr := string(respBytes)

	if resp.StatusCode != http.StatusOK {
		return respStr, fmt.Errorf("prismd returned status %d: %s", resp.StatusCode, respStr)
	}

	return respStr, nil
}
