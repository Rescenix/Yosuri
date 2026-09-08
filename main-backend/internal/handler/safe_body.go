package handler

import (
	"io"
	"net/http"
)

// maxUpstreamBodyBytes 上游响应体读取上限（64MB）：base64 识图/生图响应可达几十 MB，
// 留足余量；异常上游（错误页/回环/攻击）不再能把整进程内存吃光。
const maxUpstreamBodyBytes = 64 << 20

// readUpstreamBody 带上限地读取上游响应体。超限不报错，静默截断到上限
// （调用方拿到的是截断内容，错误体/JSON 解析自然失败，行为与旧版一致）。
func readUpstreamBody(resp *http.Response) ([]byte, error) {
	return io.ReadAll(io.LimitReader(resp.Body, maxUpstreamBodyBytes))
}
