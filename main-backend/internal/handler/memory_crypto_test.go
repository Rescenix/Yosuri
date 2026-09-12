package handler

// memory_crypto_test.go —— 桌面端记忆加密单测（2026-09-12 方案A）。
// 验证：登录哈希与云端同口径（固定向量）、AK 包装/解锁、payload 加密往返、
// 老明文格式兼容透传。

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/pbkdf2"
)

// pbkdf2_sha256_600k 测试内独立重算（不依赖被测函数，防参数漂移）。
func pbkdf2_sha256_600k(pw, salt []byte) []byte {
	return pbkdf2.Key(pw, salt, 600000, 32, sha256.New)
}

// TestMemLoginHash_MatchesCloudSpec 固定向量验证：PBKDF2 参数与云端 auth_v2.go 完全一致。
// 该哈希值由相同参数独立计算（PBKDF2-HMAC-SHA256, 600000 iter, "rescene-login:"+username）。
func TestMemLoginHash_MatchesCloudSpec(t *testing.T) {
	got := MemLoginHash("testPass123", "mem_test_user")
	// 独立用标准库重算一遍，双重确认参数没跑偏。
	dk := pbkdf2_sha256_600k([]byte("testPass123"), []byte("rescene-login:mem_test_user"))
	want := hex.EncodeToString(dk)
	if got != want {
		t.Fatalf("登录哈希不一致:\n got=%s\nwant=%s", got, want)
	}
	if len(got) != 64 {
		t.Fatalf("哈希长度 %d != 64", len(got))
	}
}

// TestAKEnvelope_Unlock 模拟登录链路：KEK 包装 AK → 存盘 → 重读解锁，往返一致。
func TestAKEnvelope_Unlock(t *testing.T) {
	username, password := "mem_test_user", "testPass123"
	kek := memDeriveKEK(password, username)
	ak := make([]byte, 32)
	for i := range ak {
		ak[i] = byte(i)
	}
	sealed, err := memSeal(kek, ak)
	if err != nil {
		t.Fatalf("包装 AK: %v", err)
	}
	// 模拟云端返回 wrapped_ak → 用密码 KEK 解开
	got, err := memOpen(kek, sealed)
	if err != nil {
		t.Fatalf("解锁 AK: %v", err)
	}
	if !bytes.Equal(got, ak) {
		t.Fatalf("AK 往返不一致")
	}
	// 错密码解不开（凭证校验语义）
	wrongKek := memDeriveKEK("wrongPass456", username)
	if _, err := memOpen(wrongKek, sealed); err == nil {
		t.Fatalf("错误 KEK 竟然解开了信封，严重安全缺陷")
	}
}

// TestMemoryPayload_EncryptRoundtrip payload 加密 → 云端 → 解密还原。
func TestMemoryPayload_EncryptRoundtrip(t *testing.T) {
	ak := make([]byte, 32)
	for i := range ak {
		ak[i] = byte(255 - i)
	}
	plain := `{"preferences.md":"喜欢简洁界面","project.md":"在开发社交App"}`
	enc, err := EncryptMemoryPayload(plain, ak)
	if err != nil {
		t.Fatalf("加密: %v", err)
	}
	if len(enc) < 60 {
		t.Fatalf("密文过短: %q", enc)
	}
	// 云端不可见明文：密文串里不应出现原文任何片段
	if bytes.Contains([]byte(enc), []byte("简洁")) || bytes.Contains([]byte(enc), []byte("preferences")) {
		t.Fatalf("密文泄露明文特征: %q", enc)
	}
	dec := DecryptMemoryPayload(enc, ak)
	if dec != plain {
		t.Fatalf("解密不一致:\n got=%q\nwant=%q", dec, plain)
	}
	// 错 AK 解不出
	wrong := make([]byte, 32)
	dec2 := DecryptMemoryPayload(enc, wrong)
	if dec2 != "" {
		t.Fatalf("错误 AK 竟然解出内容")
	}
}

// TestDecryptMemoryPayload_LegacyPlain 老明文格式（存量兼容）：无 AK / 非 v2 格式透传。
func TestDecryptMemoryPayload_LegacyPlain(t *testing.T) {
	legacy := `{"preferences.md":"老数据明文"}`
	if got := DecryptMemoryPayload(legacy, nil); got != legacy {
		t.Fatalf("老明文应原样透传, got=%q", got)
	}
	// 有 AK 但格式不是 v2 密文 → 也透传（存量还没重推）
	ak := make([]byte, 32)
	if got := DecryptMemoryPayload(legacy, ak); got != legacy {
		t.Fatalf("非 v2 应透传, got=%q", got)
	}
}

// TestEnsureAccountAK_DiskPersistence AK 落盘 → 重读不重新生成（改密码/重启不丢）。
func TestEnsureAccountAK_DiskPersistence(t *testing.T) {
	dir := t.TempDir()
	old := os.Getenv("RESCENE_DATA_DIR")
	_ = os.Setenv("RESCENE_DATA_DIR", dir)
	defer os.Setenv("RESCENE_DATA_DIR", old)
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Join(dir, "memory_ak_42")) })

	username, password := "mem_test_user", "testPass123"
	kek := memDeriveKEK(password, username)
	ak := make([]byte, 32)
	for i := range ak {
		ak[i] = byte(i)
	}
	sealed, _ := memSeal(kek, ak)
	// 首次：云端有信封 → 解开落盘
	got := EnsureAccountAK(42, username, password, sealed, "test-token")
	if got == nil || !bytes.Equal(got, ak) {
		t.Fatalf("首次解锁 AK 失败")
	}
	// 二次：直接读盘，即使云端信封给了错的也返回磁盘 AK（不重复生成）
	got2 := EnsureAccountAK(42, username, password, "corrupted-envelope", "test-token")
	if got2 == nil || !bytes.Equal(got2, ak) {
		t.Fatalf("AK 未持久化，二次调用变了")
	}
}
