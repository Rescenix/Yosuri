package handler

// memory_pipeline_dedup_test.go —— 自动提取去重验证：
//   1. canonicalFactKey 把同义别名收敛到同一规范 key（裂变治本）；
//   2. applyAutomaticFacts 同簇 add 不并列、update 覆盖、等价改写不重刷；
//   3. consolidateFacts 存量合并 45→ 少、幂等。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCanonicalFactKey(t *testing.T) {
	// 只测确定性形态归一：小写、分隔符折叠、装饰性前后缀剥离。
	// 语义同义（coding_language vs code_language）不靠硬编码表，由提取器
	// prompt 里的 existing_facts 清单让模型复用已有 key 来挡。
	cases := map[string]string{
		"code_language":            "code_language",
		"Code-Language":            "code_language",
		"code language":            "code_language",
		"preferred_code_language":  "code_language",
		"code_language_preference": "code_language",
		"coding_language":          "coding_language", // 形态不同义，不强行合并
		"reply_length_preference":  "reply_length",
		"use_of_emoji":             "emoji",
		"project_root_path":        "project_root_path", // project_ 不剥（承载语义的前缀）
		"current_project_name":     "current_project_name",
		"my_tone":                  "tone",
		"fibonacci_py_development": "fibonacci_py_development", // 无装饰成分，原样
	}
	for in, want := range cases {
		if got := canonicalFactKey(in); got != want {
			t.Errorf("canonicalFactKey(%q) = %q，期望 %q", in, got, want)
		}
	}
}

func TestApplyAutomaticFactsDedup(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir) // Windows 下 os.UserHomeDir 读这个
	t.Setenv("RESCENE_AUTO_MEMORY", "")

	base := time.Now().UTC().Add(-time.Hour)
	// 预置：同簇两条形态别名（preferred_ 前缀裂变）+ 一条独立维度 + 一条不同语义维度
	preset := []memoryFact{
		{Category: "preferences", Key: "code_language", Value: "C", Updated: base},
		{Category: "preferences", Key: "preferred_code_language", Value: "C language", Updated: base.Add(time.Minute)},
		{Category: "preferences", Key: "coding_language", Value: "C", Updated: base.Add(2 * time.Minute)},
		{Category: "preferences", Key: "tone", Value: "soft, cute", Updated: base},
	}
	data, _ := json.Marshal(preset)
	if err := os.MkdirAll(filepath.Join(dir, "rescene_data", "memory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(automaticMemoryFactsPath(), data, 0o644); err != nil {
		t.Fatal(err)
	}

	// 新一轮提取：又一条 preferred_ 前缀别名 + tone 的等价改写 + 一个新维度
	changes := []extractedFact{
		{Op: "add", Category: "preferences", Key: "code_language_preference", Value: "C", Confidence: "high"},
		{Op: "update", Category: "preferences", Key: "tone_preference", Value: "soft cute", Confidence: "medium"},
		{Op: "add", Category: "preferences", Key: "greeting_style", Value: "中文+颜文字", Confidence: "high"},
	}
	if err := applyAutomaticFacts("dedup_test_1", changes); err != nil {
		t.Fatal(err)
	}

	facts, err := loadAutomaticFacts()
	if err != nil {
		t.Fatal(err)
	}
	byKey := map[string]memoryFact{}
	for _, f := range facts {
		byKey[f.Key] = f
	}
	// code_language 簇（含 preferred_/…_preference 裂变）归 1，coding_language 形态不同不并，
	// tone 归 1，greeting_style 新增 → 共 4
	if len(facts) != 4 {
		t.Fatalf("应归并为 4 条，得到 %d: %+v", len(facts), facts)
	}
	if _, ok := byKey["code_language"]; !ok {
		t.Error("归一后应存在规范 key code_language")
	}
	if _, ok := byKey["coding_language"]; !ok {
		t.Error("coding_language 形态独立，不应被强行并入")
	}
	if _, ok := byKey["tone"]; !ok {
		t.Error("tone 应保留（等价改写不新增并列）")
	}
	if byKey["tone"].Value != "soft, cute" {
		t.Errorf("soft cute 与 soft, cute 归一化等价，应保留旧表达，得到 %q", byKey["tone"].Value)
	}
	if byKey["greeting_style"].Value != "中文+颜文字" {
		t.Error("新维度应正常写入")
	}
	// 所有落盘 key 必须已是规范形式（重跑归一不变）
	for _, f := range facts {
		if canonicalFactKey(f.Key) != f.Key {
			t.Errorf("落盘 key %q 未规范化", f.Key)
		}
	}
}

func TestConsolidateFactsIdempotent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("RESCENE_AUTO_MEMORY", "")
	if err := os.MkdirAll(filepath.Join(dir, "rescene_data", "memory"), 0o755); err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC()
	// 只用形态可归一的裂变样本（preferred_ 前缀 / _preference 后缀 / profile↔preferences 同桶），
	// 语义同义合并不在确定性层做，不放进本测试断言。
	preset := []memoryFact{
		{Category: "preferences", Key: "message_length", Value: "short and direct", Updated: base},
		{Category: "preferences", Key: "preferred_message_length", Value: "简短直接", Updated: base.Add(time.Minute)},
		{Category: "preferences", Key: "message_length_preference", Value: "short and direct", Updated: base.Add(2 * time.Minute)},
		{Category: "profile", Key: "language", Value: "Chinese", Updated: base},
		{Category: "preferences", Key: "preferred_language", Value: "中文", Updated: base.Add(time.Minute)},
		{Category: "projects", Key: "mock_demo_thing", Value: "演示用", Updated: base},
	}
	data, _ := json.Marshal(preset)
	if err := os.WriteFile(automaticMemoryFactsPath(), data, 0o644); err != nil {
		t.Fatal(err)
	}

	consolidateFacts()
	facts, err := loadAutomaticFacts()
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]memoryFact{}
	for _, f := range facts {
		keys[f.Category+"\x00"+f.Key] = f
	}
	// message_length 簇 3→1（保留最新表达 简短直接），language 簇（profile+preferences 同桶）2→1，mock 噪音删除
	if len(facts) != 2 {
		t.Fatalf("清洗后应剩 2 条，得到 %d: %+v", len(facts), facts)
	}
	if _, ok := keys["preferences\x00message_length"]; !ok {
		t.Error("message_length 簇应归并且用规范 key")
	}
	if keys["preferences\x00message_length"].Value != "short and direct" {
		t.Errorf("同簇应保留最新表达，得到 %q", keys["preferences\x00message_length"].Value)
	}
	if _, ok := keys["preferences\x00language"]; !ok {
		t.Error("language 簇应归并进 preferences 桶")
	}

	// 幂等：再跑一次不落新盘、条数不变
	before, _ := os.Stat(automaticMemoryFactsPath())
	consolidateFacts()
	after, _ := os.Stat(automaticMemoryFactsPath())
	if !before.ModTime().Equal(after.ModTime()) {
		t.Error("第二次清洗不应重写 facts.json（幂等）")
	}
}
