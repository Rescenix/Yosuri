package prismdriver

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"net"
	"strings"
)

func init() {
	sql.Register("prism", &PrismDriver{})
}

type PrismDriver struct{}

func (d *PrismDriver) Open(dsn string) (driver.Conn, error) {
	// dsn 格式: root@localhost:5666/prism
	// 简化处理，直接取地址
	addr := "localhost:5666"
	if strings.Contains(dsn, "@") {
		parts := strings.SplitN(dsn, "@", 2)
		if len(parts) == 2 {
			addr = strings.Split(parts[1], "/")[0]
		}
	}
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	return &prismConn{conn: conn}, nil
}

type prismConn struct {
	conn net.Conn
}

func (c *prismConn) Prepare(query string) (driver.Stmt, error) {
	return &prismStmt{conn: c.conn, query: query}, nil
}

func (c *prismConn) Close() error {
	return c.conn.Close()
}

func (c *prismConn) Begin() (driver.Tx, error) {
	return nil, fmt.Errorf("transactions not supported")
}

type prismStmt struct {
	conn  net.Conn
	query string
}

func (s *prismStmt) Close() error { return nil }

func (s *prismStmt) NumInput() int { return -1 }

func (s *prismStmt) Exec(args []driver.Value) (driver.Result, error) {
	// 将 args 替换占位符
	cmd := replacePlaceholders(s.query, args)
	_, err := s.conn.Write([]byte(cmd + "\n"))
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 4096)
	n, err := s.conn.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := string(buf[:n])
	if strings.HasPrefix(resp, "ERROR") {
		return nil, fmt.Errorf("prism error: %s", resp)
	}
	return &prismResult{resp: resp}, nil
}

func (s *prismStmt) Query(args []driver.Value) (driver.Rows, error) {
	cmd := replacePlaceholders(s.query, args)
	_, err := s.conn.Write([]byte(cmd + "\n"))
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 4096)
	n, err := s.conn.Read(buf)
	if err != nil {
		return nil, err
	}
	resp := string(buf[:n])
	// 解析 LOOM 响应为 rows
	return &prismRows{data: resp}, nil
}

type prismResult struct {
	resp string
}

func (r *prismResult) LastInsertId() (int64, error) { return 0, nil }
func (r *prismResult) RowsAffected() (int64, error) { return 1, nil }

type prismRows struct {
	data  string
	index int
	cols  []string
	rows  [][]string
}

func (r *prismRows) Columns() []string {
	// 暂时固定
	return []string{"id", "score"}
}

func (r *prismRows) Close() error { return nil }

func (r *prismRows) Next(dest []driver.Value) error {
	// 简单解析响应行，演示用
	lines := strings.Split(r.data, "\n")
	if r.index >= len(lines) {
		return fmt.Errorf("EOF")
	}
	// 第一行是 OK，跳过
	if r.index == 0 {
		r.index++
	}
	if r.index >= len(lines) {
		return fmt.Errorf("EOF")
	}
	line := lines[r.index]
	parts := strings.Split(line, " ")
	if len(parts) >= 2 {
		dest[0] = parts[0]
		dest[1] = parts[1]
	}
	r.index++
	return nil
}

func replacePlaceholders(query string, args []driver.Value) string {
	if len(args) == 0 {
		return query
	}
	// 简单替换：将第一个占位符 ? 替换为字符串值
	return strings.Replace(query, "?", fmt.Sprint(args[0]), 1)
}
