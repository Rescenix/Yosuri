package handler

import (
	"net/http"
)

var (
	DeepSeekTransport http.RoundTripper = http.DefaultTransport
)
