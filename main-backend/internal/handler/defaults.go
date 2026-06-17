package handler

import (
	"backend/internal/ai/core"
	"net/http"
)

var (
	SystemPrompt                        = core.SystemPrompt
	ChatTools                           = core.ChatTools
	DeepSeekTransport http.RoundTripper = http.DefaultTransport
)
