package handler

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestDataTemplateHasByDay 多文件数据层必须带 byDay 日期聚合接口：
// 打卡/习惯/统计类产品的 app.js 直接用它做每日趋势，不再自己拼日期逻辑（接口漂移来源）。
func TestDataTemplateHasByDay(t *testing.T) {
	js := companyDataJSTemplate("测试项目")
	if !strings.Contains(js, "byDay:function") {
		t.Fatal("data.js 模板缺 byDay 接口")
	}
	if !strings.Contains(js, "createdAt") {
		t.Fatal("byDay 缺省字段应为 createdAt")
	}
}

// TestDataByDayRealExec 用 Node 真执行 data.js 验证 byDay 语义（有 node 才跑，无则跳过）。
func TestDataByDayRealExec(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("本机无 node，跳过真执行")
	}
	dir := t.TempDir()
	jsPath := filepath.Join(dir, "data.js")
	if err := os.WriteFile(jsPath, []byte(companyDataJSTemplate("测试项目")), 0o644); err != nil {
		t.Fatal(err)
	}
	script := `
const fs = require('fs');
global.localStorage = { _m:{}, getItem(k){return this._m[k]||null}, setItem(k,v){this._m[k]=v} };
global.window = global;
eval(fs.readFileSync(process.argv[2],'utf8'));
AppData.add({text:'a'}); AppData.add({text:'b'});
const today = AppData.byDay();
const custom = AppData.byDay('day');
const empty = AppData.byDay('不存在字段');
console.log('RESULT:' + JSON.stringify({today, custom, empty}));
`
	if err := os.WriteFile(filepath.Join(dir, "run.js"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("node", filepath.Join(dir, "run.js"), jsPath).CombinedOutput()
	if err != nil {
		t.Fatalf("node 执行失败: %v\n%s", err, out)
	}
	idx := strings.Index(string(out), "RESULT:")
	if idx < 0 {
		t.Fatalf("输出缺 RESULT 标记: %s", out)
	}
	var r struct {
		Today  []struct{ Day string; Count int } `json:"today"`
		Empty  []struct{}                        `json:"empty"`
	}
	if err := json.Unmarshal([]byte(string(out)[idx+7:]), &r); err != nil {
		t.Fatalf("结果解析失败: %v\n%s", err, out)
	}
	if len(r.Today) != 1 || r.Today[0].Count != 2 {
		t.Fatalf("byDay 缺省应按 createdAt 聚合出 1 天 2 条: %+v", r.Today)
	}
	if len(r.Empty) != 0 {
		t.Fatalf("不存在的字段应返回空数组: %+v", r.Empty)
	}
}

// TestDeliveryKeywords 关键词提取：多词、去重、数量上限、英文兜底。
func TestDeliveryKeywords(t *testing.T) {
	kws := deliveryKeywords("做一个学生专注冲刺台，能番茄钟计时和待办统计", 3)
	if len(kws) == 0 || len(kws) > 3 {
		t.Fatalf("应取 1-3 个关键词: %v", kws)
	}
	for _, k := range kws {
		r := []rune(k)
		if len(r) < 2 || len(r) > 4 {
			t.Fatalf("关键词应为 2-4 字: %v", kws)
		}
	}
	seen := map[string]bool{}
	for _, k := range kws {
		if seen[k] {
			t.Fatalf("关键词重复: %v", kws)
		}
		seen[k] = true
	}
	if got := deliveryKeywords("", 3); got != nil {
		t.Fatalf("空输入应返回 nil: %v", got)
	}
	en := deliveryKeywords("Todo List", 3)
	if len(en) == 0 || en[0] != strings.ToLower(en[0]) {
		t.Fatalf("英文应小写兜底: %v", en)
	}
}
