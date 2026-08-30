package handler

import (
	"sort"
	"testing"
	"time"
)

func TestTimSortStable(t *testing.T) {
	// 1. 空/单元素
	got := timSortStable(nil, func(a, b autoItem) bool { return true })
	if len(got) != 0 {
		t.Fatal("nil 应返回空")
	}
	got = timSortStable([]autoItem{{}}, func(a, b autoItem) bool { return true })
	if len(got) != 1 {
		t.Fatal("单元素应返回")
	}

	// 2. 降序 signal + 升序 latency + 稳定
	raw := []autoItem{
		{h: autoHealth{signal: 3, latency: 100, lastOK: t0(1)}},
		{h: autoHealth{signal: 1, latency: 50, lastOK: t0(2)}},
		{h: autoHealth{signal: 4, latency: 200, lastOK: t0(3)}},
		{h: autoHealth{signal: 2, latency: 80, lastOK: t0(4)}},
		{h: autoHealth{signal: 3, latency: 50, lastOK: t0(5)}},  // 同 signal 3 但延迟更低，应排 #1 前面
		{h: autoHealth{signal: 0, latency: 0, lastOK: t0(6)}},   // 死源沉底
	}
	less := func(a, b autoItem) bool {
		ha, hb := a.h, b.h
		if ha.signal == 0 && hb.signal != 0 {
			return false
		}
		if ha.signal != hb.signal {
			return ha.signal > hb.signal
		}
		if ha.latency != hb.latency {
			return ha.latency < hb.latency
		}
		return ha.lastOK.After(hb.lastOK)
	}
	got = timSortStable(raw, less)
	check := func(i, idx int, label string) {
		t.Helper()
		if got[i].h.lastOK != t0(idx) {
			t.Fatalf("位置 %d: 期望索引 %d (%s), 实际 %v", i, idx, label, got[i].h.lastOK)
		}
	}
	check(0, 3, "signal 4")
	check(1, 5, "signal 3, latency 50")
	check(2, 1, "signal 3, latency 100")
	check(3, 4, "signal 2, latency 80")
	check(4, 2, "signal 1, latency 50")
	check(5, 6, "signal 0 死源沉底")

	// 3. 耗尽沉底
	raw2 := []autoItem{
		{h: autoHealth{exhausted: true, signal: 4, latency: 10}},
		{h: autoHealth{exhausted: false, signal: 3, latency: 50}},
		{h: autoHealth{exhausted: false, signal: 4, latency: 20}},
		{h: autoHealth{exhausted: true, signal: 4, latency: 5}},
	}
	less2 := func(a, b autoItem) bool {
		ha, hb := a.h, b.h
		if ha.exhausted != hb.exhausted {
			return !ha.exhausted
		}
		if ha.signal != hb.signal {
			return ha.signal > hb.signal
		}
		return ha.latency < hb.latency
	}
	got2 := timSortStable(raw2, less2)
	if got2[0].h.exhausted || got2[1].h.exhausted {
		t.Fatal("未耗尽应先排")
	}
	if !got2[2].h.exhausted || !got2[3].h.exhausted {
		t.Fatal("耗尽应沉底")
	}

	// 3b. 熔断沉底：signal 高但熔断中的源应排到未熔断健康源后面（08-29 实锤：
	// LLM7 探活 signal=2 但真实 400 熔断，之前排 auto 链第一拖垮整链）
	rawCB := []autoItem{
		{h: autoHealth{signal: 2, latency: 273, lastOK: t0(1), circuitOpen: true}},  // 熔断中但探活健康
		{h: autoHealth{signal: -1, latency: 0, lastOK: t0(2), circuitOpen: false}},  // 未探测但可用
		{h: autoHealth{signal: 1, latency: 500, lastOK: t0(3), circuitOpen: false}}, // 健康可用
	}
	lessCB := func(a, b autoItem) bool {
		ha, hb := a.h, b.h
		if ha.exhausted != hb.exhausted {
			return !ha.exhausted
		}
		if ha.circuitOpen != hb.circuitOpen {
			return !ha.circuitOpen
		}
		if ha.signal != hb.signal {
			return ha.signal > hb.signal
		}
		if ha.latency != hb.latency {
			return ha.latency < hb.latency
		}
		return ha.lastOK.After(hb.lastOK)
	}
	gotCB := timSortStable(rawCB, lessCB)
	if gotCB[0].h.circuitOpen {
		t.Fatalf("熔断中的源应沉底, 首位实际 circuitOpen=%v", gotCB[0].h.circuitOpen)
	}
	if !gotCB[2].h.circuitOpen {
		t.Fatalf("熔断中的源应排最后, 末位实际 circuitOpen=%v", gotCB[2].h.circuitOpen)
	}

	// 4. 稳定性：完全相同健康度的元素顺序不变
	raw3 := []autoItem{
		{h: autoHealth{signal: 1, latency: 10, lastOK: t0(1)}},
		{h: autoHealth{signal: 1, latency: 10, lastOK: t0(2)}},
		{h: autoHealth{signal: 1, latency: 10, lastOK: t0(3)}},
	}
	less3 := func(a, b autoItem) bool { return false } // 永远不交换
	got3 := timSortStable(raw3, less3)
	for i := range got3 {
		if got3[i].h.lastOK != t0(i+1) {
			t.Fatalf("稳定排序: 位置 %d 期望 %d, 实际 %v", i, i+1, got3[i].h.lastOK)
		}
	}

	// 5. 大量随机数据验证正确性（与 sort.SliceStable 结果一致）
	n := 100
	rng := make([]autoItem, n)
	for i := 0; i < n; i++ {
		rng[i] = autoItem{
			h: autoHealth{
				exhausted: i%10 == 0,
				signal:    i % 5,
				latency:   time.Duration((i*7)%1000) * time.Millisecond,
				lastOK:    t0(i % 20),
			},
		}
	}
	// 复制一份，用标准库排序做基准
	ref := make([]autoItem, n)
	copy(ref, rng)
	lessRef := func(a, b autoItem) bool {
		ha, hb := a.h, b.h
		if ha.exhausted != hb.exhausted {
			return !ha.exhausted
		}
		if ha.signal == 0 && hb.signal != 0 {
			return false
		}
		if ha.signal != hb.signal {
			return ha.signal > hb.signal
		}
		if ha.latency != hb.latency {
			return ha.latency < hb.latency
		}
		return ha.lastOK.After(hb.lastOK)
	}
	sort.SliceStable(ref, func(i, j int) bool { return lessRef(ref[i], ref[j]) })
	gotRng := timSortStable(rng, lessRef)
	for i := range ref {
		if gotRng[i].h.lastOK != ref[i].h.lastOK {
			t.Fatalf("n=%d 位置 %d: 期望 idx=%v 实际 idx=%v", n, i, ref[i].h.lastOK, gotRng[i].h.lastOK)
		}
	}
}

func t0(day int) time.Time {
	return time.Date(2026, 8, day, 0, 0, 0, 0, time.UTC)
}