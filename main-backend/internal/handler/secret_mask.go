package handler

// secret_mask.go —— 工具出口密钥脱敏（Hermes 同款思路，09-10）。
//
// 原则：文件在磁盘上仍是明文，掩码只发生在「结果进模型上下文之前」。
// 模型经 read/grep 看到的密钥值一律 ***；真正用 key 的是后端进程读盘直连
// 上游，那条路不经过模型。配合 isSensitiveFile（整文件拦截）与
// isCredentialFile（公开读口黑名单）构成三层：整文件 → 行内值 → 出口端点。

import (
	"regexp"
	"strings"
)

// secretKVPattern 匹配「键名含密钥语义词 + : 或 = + 值」的行内形态。
// 键名必须命中 key/token/secret/password/passwd/credential/authorization 之一
// （允许前后缀，如 api_key、ACCESS_TOKEN、"apiKey"），否则不动。
// 值类故意排除 `.` 与 `(`：代码引用（os.Getenv(...)、a.b.c）几乎必含其一，
// 而真实密钥串基本不含，一条规则同时挡掉误伤。Bearer/Basic 前缀一并吞掉。
var secretKVPattern = regexp.MustCompile(`(?i)(["']?[a-z0-9_\-]*(?:api[_\-]?key|secret|token|password|passwd|credential|authorization)[a-z0-9_\-]*["']?)(\s*[:=]\s*)(["']?)(?:(?:bearer|basic)\s+)?([a-z0-9\-_/+=]{8,})(["']?)`)

// 值短于此长度不掩码：源码里的标识符引用（APIKey: key）、布尔/数字占位
// （"api_key": true）不是秘密，掩了反而诱导模型把 *** 抄回补丁里损坏源码。
const maskMinValueLen = 8

func maskSecretText(s string) string {
	if !strings.ContainsAny(s, ":=") && !hasTokenPrefix(s) {
		return s
	}
	s = secretKVPattern.ReplaceAllStringFunc(s, func(m string) string {
		groups := secretKVPattern.FindStringSubmatch(m)
		if groups == nil {
			return m
		}
		val := groups[4]
		if len(val) < maskMinValueLen || strings.Contains(val, "***") {
			return m
		}
		return groups[1] + groups[2] + groups[3] + "***" + groups[5]
	})
	// 独立 token 形态：sk-/ghp_/xoxb-/eyJ(JWT) 这类自带前缀的密钥串，
	// 即使不在 kv 结构里（裸出现在日志/命令输出）也掩掉。
	return secretTokenPattern.ReplaceAllString(s, "$1***")
}

// secretTokenPattern 常见密钥前缀 + 足够长的实体串。
var secretTokenPattern = regexp.MustCompile(`(?i)\b(sk|ghp|gho|github_pat|xoxb|xapp|AKIA|ya29)[-_][A-Za-z0-9_\-]{12,}`)

func hasTokenPrefix(s string) bool {
	return secretTokenPattern.MatchString(s)
}
