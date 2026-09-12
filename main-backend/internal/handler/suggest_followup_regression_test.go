package handler

import (
	"reflect"
	"testing"
)

// 回归套件：把 followup 链路历史上出过的 bug 全部钉死。
// 解析层纯函数测试，无网络，秒级。
func TestSuggestParseAllKnownBugs(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{
			name: "干净JSON数组",
			raw:  `["跑一遍测试确认","加个参数校验","同步到文档"]`,
			want: []string{"跑一遍测试确认", "加个参数校验", "同步到文档"},
		},
		{
			name: "bug:数组前夹解释文字",
			raw:  `好的，以下是给用户的建议：["跑一遍测试确认","加个参数校验"] 希望有帮助`,
			want: []string{"跑一遍测试确认", "加个参数校验"},
		},
		{
			name: "bug:数组后夹尾巴",
			raw:  `["把改动同步到官网文档","给函数补单测"] 这样用户就可以继续了`,
			want: []string{"把改动同步到官网文档", "给函数补单测"},
		},
		{
			name: "bug:markdown代码块包裹",
			raw:  "```json\n[\"给 parseFollowUps 补单测\",\"把改动同步到官网文档\"]\n```",
			want: []string{"给 parseFollowUps 补单测", "把改动同步到官网文档"},
		},
		{
			name: "bug:解释前后都有文字",
			raw:  `根据你的工作，我建议：["检查节流逻辑","补充测试","更新文档"] 选一个吧`,
			want: []string{"检查节流逻辑", "补充测试", "更新文档"},
		},
		{
			name: "空数组保持空",
			raw:  `[]`,
			want: []string{},
		},
		{
			name: "空输出不panic",
			raw:  ``,
			want: nil,
		},
		{
			name: "纯文字无数组返回nil",
			raw:  `任务已经完成了，没有更多建议`,
			want: nil,
		},
		{
			name: "单引号数组兜底为空(不panic)",
			raw:  `['单引号','会被JSON拒绝']`,
			want: nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := extractSuggestionArray(c.raw)
			parsed := parseSuggestionJSON(got)
			if !reflect.DeepEqual(parsed, c.want) {
				t.Errorf("extract(%q)\n  got  %#v\n  want %#v", c.raw, parsed, c.want)
			}
		})
	}
}

// 测试清洗：AI口吻过滤 + 去重 + 上限3
func TestSuggestCleanFilters(t *testing.T) {
	got := cleanSuggestions([]string{
		"我可以帮你分析",        // AI 口吻 → 丢
		"跑一遍测试确认",        // ok
		"跑一遍测试确认",        // 重复 → 丢
		"请问需要我做什么",       // AI 口吻 → 丢
		"顺手加个错误提示",       // ok
		"把改动同步到文档",       // ok
		"",                // 空 → 丢
		"第四条超出上限",        // >3 → 丢
	})
	want := []string{"跑一遍测试确认", "顺手加个错误提示", "把改动同步到文档"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("cleanSuggestions got %#v want %#v", got, want)
	}
}