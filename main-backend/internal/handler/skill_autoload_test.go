package handler

import (
	"strings"
	"testing"
)

const frontendDesignBodyMarker = "一屏一个视觉重心"

// 宿主不再做技能全文预加载：任何任务（包括命中 trigger 的前端任务、
// 显式点名技能的任务）都不该把技能正文塞进系统提示词，正文只能靠
// 模型自己调 skill_view 取回。
func TestNoSkillBodyIsPreloadedForAnyTask(t *testing.T) {
	for _, task := range []string{
		"请实现一个响应式 Vue 前端设置页面",
		"请使用 frontend-design 完成这项工作",
		"为 Go 后端的分页函数补充单元测试",
	} {
		provider := newWorkflowContextProvider(task)
		if strings.Contains(provider.SystemPrompt(), frontendDesignBodyMarker) {
			t.Fatalf("任务 %q 不应预加载技能正文", task)
		}
	}
}

// 索引段必须始终在场，且带"命中先取全文"的强制规则。
func TestSkillIndexAlwaysInjected(t *testing.T) {
	provider := newWorkflowContextProvider("随便什么任务")
	sp := provider.SystemPrompt()
	if !strings.Contains(sp, "技能库索引") {
		t.Fatalf("系统提示词缺少技能库索引段")
	}
	if !strings.Contains(sp, "skill_view") {
		t.Fatalf("技能库索引缺少 skill_view 取全文指引")
	}
}

// 索引是稳定段：换任务不该改变 skill 桶的占用（全文预加载时代会变）。
func TestSkillBreakdownIsTaskIndependent(t *testing.T) {
	a := newWorkflowContextProvider("设计一个网站前端")
	b := newWorkflowContextProvider("检查 Go 服务日志")
	if a.Breakdown()["skill"] != b.Breakdown()["skill"] {
		t.Fatalf("skill 桶占用随任务变化，说明还有按任务注入的正文: %d vs %d",
			a.Breakdown()["skill"], b.Breakdown()["skill"])
	}
}
