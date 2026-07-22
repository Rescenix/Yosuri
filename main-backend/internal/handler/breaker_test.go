package handler

import "testing"

// 熔断护栏：同一 工具名+参数 真实执行 limit 次后，第 limit+1 次起被拦。
// 实况里健康模型几乎不触发（看得见工具结果就不会重复调），所以只能单测保证它的确会拦。
func TestShouldBlockRepeat(t *testing.T) {
	counts := map[string]int{}
	limit := codeRepeatCallLimit // 生产用的真实阈值

	// 前 limit 次都应放行
	for i := 1; i <= limit; i++ {
		if shouldBlockRepeat(counts, "read_file", `{"path":"x"}`, limit) {
			t.Fatalf("第 %d 次（≤%d）不该被拦", i, limit)
		}
	}
	// 第 limit+1 次起被拦
	if !shouldBlockRepeat(counts, "read_file", `{"path":"x"}`, limit) {
		t.Fatalf("第 %d 次应被熔断", limit+1)
	}
	if !shouldBlockRepeat(counts, "read_file", `{"path":"x"}`, limit) {
		t.Error("超限后应持续被拦")
	}
}

// 参数不同 = 不同签名，各自独立计数，不该互相牵连。
func TestShouldBlockRepeatDistinctArgs(t *testing.T) {
	counts := map[string]int{}
	// 对 a.go 打满到被拦
	for i := 0; i <= codeRepeatCallLimit; i++ {
		shouldBlockRepeat(counts, "read_file", `{"path":"a.go"}`, codeRepeatCallLimit)
	}
	if !shouldBlockRepeat(counts, "read_file", `{"path":"a.go"}`, codeRepeatCallLimit) {
		t.Fatal("a.go 应已被熔断")
	}
	// 换个文件，第一次必须放行——不能被 a.go 的计数牵连
	if shouldBlockRepeat(counts, "read_file", `{"path":"b.go"}`, codeRepeatCallLimit) {
		t.Error("不同参数是不同签名，第一次不该被拦")
	}
	// 工具名不同也是不同签名
	if shouldBlockRepeat(counts, "write_file", `{"path":"a.go"}`, codeRepeatCallLimit) {
		t.Error("不同工具名是不同签名，第一次不该被拦")
	}
}
