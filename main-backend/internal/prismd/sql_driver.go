// internal/prismd/sql_driver.go
package prismd

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// 注册驱动
func init() {
	sql.Register("prismd", &PrismDDriver{})
}

// PrismDDriver 实现了 database/sql/driver.Driver 接口
type PrismDDriver struct{}

func (d *PrismDDriver) Open(dsn string) (driver.Conn, error) {
	// dsn 格式： "host:port" 如 "localhost:5666"
	addr := dsn
	if !strings.Contains(addr, "://") {
		addr = "http://" + addr
	}
	return &prismdConn{
		baseURL: addr,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

// prismdConn 实现了 database/sql/driver.Conn 接口
type prismdConn struct {
	baseURL string
	client  *http.Client
}

func (c *prismdConn) Prepare(query string) (driver.Stmt, error) {
	return nil, fmt.Errorf("prismd: Prepare not supported, use Exec/Query directly")
}

func (c *prismdConn) Close() error {
	return nil
}

func (c *prismdConn) Begin() (driver.Tx, error) {
	return nil, fmt.Errorf("prismd: transactions not supported")
}

// 实现 driver.Execer 接口，用于执行非查询语句（ENGRAM, PRUNE, DRIFT等）
func (c *prismdConn) Exec(query string, args []driver.Value) (driver.Result, error) {
	// 将 driver.Value 切片转换为字符串替换占位符? (如果有)
	// 但PrimQL没有占位符，所以直接发送query即可
	return c.exec(query)
}

// 实现 driver.Queryer 接口，用于执行查询语句（LOOM, STATS等）
func (c *prismdConn) Query(query string, args []driver.Value) (driver.Rows, error) {
	return c.query(query)
}

// ---------- 内部实现 ----------

func (c *prismdConn) exec(query string) (driver.Result, error) {
	resp, err := c.client.Post(c.baseURL, "text/plain", strings.NewReader(query))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prismd exec: %s", string(body))
	}
	return &prismdResult{resp: string(body)}, nil
}

func (c *prismdConn) query(query string) (driver.Rows, error) {
	resp, err := c.client.Post(c.baseURL, "text/plain", strings.NewReader(query))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prismd query: %s", string(body))
	}
	// 将响应文本转为 Rows 格式，这里简单地按行分割
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	cols := []string{"result"}
	rows := make([][]string, len(lines))
	for i, line := range lines {
		rows[i] = []string{line}
	}
	return &prismdRows{cols: cols, rows: rows, pos: 0}, nil
}

// ---------- driver.Rows 实现 ----------
type prismdRows struct {
	cols []string
	rows [][]string
	pos  int
}

func (r *prismdRows) Columns() []string {
	return r.cols
}

func (r *prismdRows) Close() error {
	return nil
}

func (r *prismdRows) Next(dest []driver.Value) error {
	if r.pos >= len(r.rows) {
		return io.EOF
	}
	for i, v := range r.rows[r.pos] {
		dest[i] = v
	}
	r.pos++
	return nil
}

// ---------- driver.Result 实现 ----------
type prismdResult struct {
	resp string
}

func (r *prismdResult) LastInsertId() (int64, error) {
	// 从响应中解析ID，如 "OK 123"
	// 为了简化，这里返回0
	return 0, nil
}

func (r *prismdResult) RowsAffected() (int64, error) {
	return 1, nil
}
