package handler

import "testing"

// 2026-09-07 实锤：sk- 明文密钥混进自动提取事实并同步云端。
// 以下用例锁死归一化层的密钥硬过滤。
func TestSensitiveFactValue(t *testing.T) {
	cases := []struct {
		name string
		s    string
		want bool
	}{
		{"openai key", "我的key是 sk-abcd1234efgh5678ijklmnop", true},
		{"sk short", "sk-test", false},
		{"aws key", "AKIAIOSFODNN7EXAMPLE", true},
		{"github pat", "ghp_abcdef1234567890abcdef1234567890", true},
		{"pem private key", "-----BEGIN RSA PRIVATE KEY-----\nMIIEow", true},
		{"token assign", "token=eyJhbGciOiJIUzI1NiJ9.xxxx", true},
		{"plain fact", "他喜欢深夜写代码", false},
		{"normal url", "项目部署在 rescene.shanca.me", false},
	}
	for _, c := range cases {
		if got := sensitiveFactValue(c.s); got != c.want {
			t.Errorf("%s: sensitiveFactValue(%q) = %v, want %v", c.name, c.s, got, c.want)
		}
	}
}

func TestSensitiveFactKeyExtra(t *testing.T) {
	for _, k := range []string{"api_key", "access_token", "secret", "password", "密钥", "密码"} {
		if !sensitiveFactKey(k) {
			t.Errorf("sensitiveFactKey(%q) = false, want true", k)
		}
	}
	if sensitiveFactKey("location") == false {
		t.Errorf("sensitiveFactKey(location) = false, want true")
	}
	if sensitiveFactKey("职业") {
		t.Errorf("sensitiveFactKey(职业) = true, want false")
	}
}
