package handler

import (
	"net/http"
)

var (
	DeepSeekTransport http.RoundTripper = http.DefaultTransport
	AliyunTransport   http.RoundTripper = http.DefaultTransport // 仅电脑端使用
)
