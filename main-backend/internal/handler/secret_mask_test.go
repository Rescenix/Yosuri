package handler

import (
	"strings"
	"testing"
)

func TestMaskSecretText(t *testing.T) {
	cases := []struct{ name, in, wantContains, wantNotContains string }{
		{"dotenv", `API_KEY=sk-abc123def456ghi789`, `API_KEY=***`, "sk-abc123"},
		{"json", `{"api_key": "sk-proj-verysecretvalue", "model": "x"}`, `"api_key": "***"`, "verysecret"},
		{"yaml", `  access_token: ghp_abcdefghijklmnop`, `access_token: ***`, "ghp_abc"},
		{"colon-space", `Authorization: Bearer abcdefghijklmnop`, `***`, "abcdefghijk"},
	}
	for _, c := range cases {
		got := maskSecretText(c.in)
		if !strings.Contains(got, c.wantContains) || strings.Contains(got, c.wantNotContains) {
			t.Errorf("%s: got %q", c.name, got)
		}
	}
}

func TestMaskSecretText_NoFalsePositive(t *testing.T) {
	keeps := []string{
		`APIKey: key, ParamsB: f.ParamsB`,            // 源码标识符引用（短值）
		`"api_key": true`,                            // 布尔
		`apiKey = os.Getenv("X")`,                    // 代码不是值
		`path: C:/Pro2026/re0/main-backend`,          // 普通 kv
		`// token 数量估算见 sessionTokenStats.js`,     // 注释无 kv
		`timeout: 180`,                               // 数字
	}
	for _, s := range keeps {
		if got := maskSecretText(s); got != s {
			t.Errorf("不应掩码 %q，得到 %q", s, got)
		}
	}
}

func TestMaskSecretText_BareToken(t *testing.T) {
	got := maskSecretText("echo done, key was sk-proj-abcdefghijklmnop1234 in env")
	if strings.Contains(got, "abcdefghijklmnop") {
		t.Errorf("裸 sk- token 应被掩码: %q", got)
	}
	// task_id 这类 task_ 前缀不能被误伤（\b 挡在 'a' 后无边界）
	safe := "后台任务 task_1757422333 完成"
	if m := maskSecretText(safe); m != safe {
		t.Errorf("task_id 不应误伤: %q", m)
	}
}

func TestMaskSecretText_Idempotent(t *testing.T) {
	in := `API_KEY=sk-abc123def456ghi789`
	once := maskSecretText(in)
	if twice := maskSecretText(once); twice != once {
		t.Errorf("重复掩码不稳定: %q vs %q", once, twice)
	}
}
