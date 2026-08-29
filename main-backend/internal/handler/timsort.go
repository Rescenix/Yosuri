package handler

import "time"

// Timsort 风格的稳定排序（2026-08-29 引入，替换 aggAutoChain 的冒泡）。
//
// 为什么用 Timsort：
//   - 名字最响的稳定排序（Python/Java/Android 标准库同款，Tim Peters 发明）
//   - 稳定：健康度相同绝不交换（LRU 兜底不破坏公平）
//   - 对 auto 链这种 n≤10 的小数组：整个数组 < minrun，直接走二分插入排序
//     （Timsort 对小子数组的标准策略），最健康已在首时 O(n) 一次过，否则
//     二分插入 O(n log n) 且稳定——比 sort.SliceStable（pdqsort）更适合：
//     pdqsort 的 partition 开销对 n≤10 是浪费，且比较器无法预取健康快照。
//   - 完整 Timsort 的 run 栈 + 归并用于大数组，auto 链 n≤10 走不到；
//     这里保留「n>minrun 时归并」内核，保证通用性。
//
// ⚠️ 比较用「值比较」less(a,b)（而非索引比较）——merge 时左半复制到辅助切片，
//    索引比较会对上已被覆盖的位置（08-29 测试实锤）。

// autoItem 是 auto 链排序的最小单位：backend + 预取的健康快照。
// 健康快照随元素一起移动，比较零额外锁、零额外函数调用。
type autoItem struct {
	b RouterBackend
	h autoHealth
}

// autoHealth 一次探活/真实请求后的健康快照（aggAutoChain 内预取）。
type autoHealth struct {
	exhausted bool
	signal    int
	latency   time.Duration
	lastOK    time.Time
}

const timMinRun = 10 // auto 链 n≤10 恒走二分插入；>10 走归并

// timSortStable 对 items 做稳定排序。less(a,b)=true 表示 a 应排在 b 前。
// 原地重排并返回 items。
func timSortStable(items []autoItem, less func(a, b autoItem) bool) []autoItem {
	n := len(items)
	if n <= 1 {
		return items
	}
	if n <= timMinRun {
		return timBinaryInsertion(items, less)
	}
	// 归并排序内核（稳定）：递归 split + merge，用辅助切片
	mid := n / 2
	timSortStable(items[:mid], less)
	timSortStable(items[mid:], less)
	return timMerge(items, mid, less)
}

// timBinaryInsertion 稳定二分插入排序。Timsort 对 <minrun 子数组的标准策略。
func timBinaryInsertion(items []autoItem, less func(a, b autoItem) bool) []autoItem {
	n := len(items)
	for i := 1; i < n; i++ {
		key := items[i]
		l, r := 0, i
		for l < r { // 找第一个 !less(key, items[m]) 的位置
			m := (l + r) / 2
			if less(key, items[m]) {
				r = m
			} else {
				l = m + 1
			}
		}
		// 稳定右移 [l, i]
		for k := i; k > l; k-- {
			items[k] = items[k-1]
		}
		items[l] = key
	}
	return items
}

// timMerge 稳定归并 [0,mid) 与 [mid,n)。左半复制到辅助切片，O(n) 空间。
// 值比较：左半读 left[i]，右半读 items[j]，不会读到被覆盖的位置。
func timMerge(items []autoItem, mid int, less func(a, b autoItem) bool) []autoItem {
	n := len(items)
	left := make([]autoItem, mid)
	copy(left, items[:mid])
	i, j, k := 0, mid, 0
	for i < mid && j < n {
		// 右半 items[j] 更靠前才取右半；否则取左半（相等取左 = 稳定）
		if less(items[j], left[i]) {
			items[k] = items[j]
			j++
		} else {
			items[k] = left[i]
			i++
		}
		k++
	}
	for ; i < mid; i++ {
		items[k] = left[i]
		k++
	}
	return items
}
