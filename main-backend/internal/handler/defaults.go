package handler

import (
    "backend/internal/ai/core"
    "net/http"
)

var (
    SystemPrompt                        = core.SystemPrompt
    ChatTools                           = core.ChatTools
    DeepSeekTransport http.RoundTripper = http.DefaultTransport
    AliyunTransport   http.RoundTripper = http.DefaultTransport // 仅电脑端使用
)