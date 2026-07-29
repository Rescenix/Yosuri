package handler

import (
	"encoding/json"
	"strings"
	"testing"
)

// 常驻工具集必须包含 load_tools 本身——否则模型没有任何办法把按需工具拿回来，
// 整个按需加载机制会变成"永远加载不了"。
func TestNativeToolsAlwaysIncludeLoadTools(t *testing.T) {
	tools := buildCodeWorkflowTools(nil)
	found := false
	for _, tl := range tools {
		fn := tl["function"].(map[string]any)
		if fn["name"] == loadToolsToolName {
			found = true
		}
	}
	if !found {
		t.Fatalf("常驻工具集里没有 %s，按需加载将无法启动", loadToolsToolName)
	}
}

func TestHandleLoadToolsUnknownName(t *testing.T) {
	activated := map[string]bool{}
	out, changed := handleLoadTools(`{"names":["mcp__不存在__foo"]}`, activated)
	if changed {
		t.Error("不存在的工具不该被标记为已激活")
	}
	if !strings.Contains(out, "不存在") {
		t.Errorf("应提示名字不存在让模型自己纠正，实得: %s", out)
	}
	if len(activated) != 0 {
		t.Errorf("激活集被污染: %v", activated)
	}
}

func TestHandleLoadToolsBadArgs(t *testing.T) {
	activated := map[string]bool{}
	if out, _ := handleLoadTools(`{`, activated); !strings.Contains(out, "解析失败") {
		t.Errorf("坏 JSON 应回可读提示而不是空串: %q", out)
	}
	if out, _ := handleLoadTools(`{"names":[]}`, activated); !strings.Contains(out, "为空") {
		t.Errorf("空 names 应回提示: %q", out)
	}
}

// 激活后该工具必须真的进 tools 数组——只回 schema 文本而不进数组的话，
// 模型看得见说明书却调不动，会卡在反复 load_tools。
func TestActivatedToolEntersToolsArray(t *testing.T) {
	name := "read_file"

	base := len(buildCodeWorkflowTools(nil))
	activated := map[string]bool{}
	out, changed := handleLoadTools(`{"names":["`+name+`"]}`, activated)
	if !changed || !activated[name] {
		t.Fatalf("%s 没有被激活: %s", name, out)
	}
	// 返回给模型的必须是可用的完整 schema，不是一句"已加载"
	if !strings.Contains(out, "parameters") {
		t.Error("load_tools 的结果里没有完整参数 schema")
	}

	after := buildCodeWorkflowTools(activated)
	if len(after) != base+1 {
		t.Fatalf("激活后 tools 数量 %d，应为 %d", len(after), base+1)
	}
	last := after[len(after)-1]["function"].(map[string]any)
	if last["name"] != name {
		t.Errorf("进数组的不是刚激活的工具: %v", last["name"])
	}
}

// 索引行必须精简：一个工具占一行、一句话。膨胀了就退化成"换个地方塞全量 schema"。
func TestToolIndexIsCompact(t *testing.T) {
	defs := allOnDemandToolDefs()
	index := mcpToolIndexPrompt()
	fullJSON, _ := json.Marshal(buildCodeWorkflowTools(func() map[string]bool {
		all := map[string]bool{}
		for _, d := range defs {
			all[d.Function.Name] = true
		}
		return all
	}()))

	if estimateTokenCount(index) > estimateTokenCount(string(fullJSON))/2 {
		t.Errorf("索引 %d tok 相对全量 %d tok 不够精简，省不出多少",
			estimateTokenCount(index), estimateTokenCount(string(fullJSON)))
	}
	for _, d := range defs {
		if !strings.Contains(index, d.Function.Name) {
			t.Errorf("索引里漏了工具 %s——模型将永远不知道它存在", d.Function.Name)
		}
	}
}

func TestFirstSentence(t *testing.T) {
	cases := map[string]string{
		"读取文件内容。支持 head/tail 参数。":     "读取文件内容",
		"Read a file. Supports head.": "Read a file",
		"没有句号的短描述":                    "没有句号的短描述",
	}
	for in, want := range cases {
		if got := firstSentence(in); got != want {
			t.Errorf("firstSentence(%q) = %q，期望 %q", in, got, want)
		}
	}
}
