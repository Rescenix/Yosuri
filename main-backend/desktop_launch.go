package main

func hasBackgroundFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--background" {
			return true
		}
	}
	return false
}

// hasNoHotPatchFlag 热补丁脚本 :failed 拉起旧版时携带 -no-hotpatch，
// 让旧版跳过 ApplyPendingHotPatch，避免「补丁恢复→旧版启动→再触发→copy失败→循环」死循环
// （2026-08-16 实测：残留实例锁 exe 时脚本 copy 失败 → :failed 恢复补丁 → 新实例又触发）。
func hasNoHotPatchFlag(args []string) bool {
	for _, arg := range args {
		if arg == "-no-hotpatch" {
			return true
		}
	}
	return false
}
