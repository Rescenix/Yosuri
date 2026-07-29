package handler

import (
	"strings"
	"testing"
)

const frontendDesignBodyMarker = "一屏一个视觉重心"

func TestFrontendDesignSkillIsPreloadedForFrontendTask(t *testing.T) {
	provider := newWorkflowContextProvider("请实现一个响应式 Vue 前端设置页面")
	prompt := provider.SystemPrompt()
	if !strings.Contains(prompt, frontendDesignBodyMarker) {
		t.Fatalf("前端任务未自动注入 frontend-design 正文")
	}
	if !strings.Contains(prompt, "宿主确定性匹配") {
		t.Fatalf("自动加载段缺少强制遵循标记")
	}
}

func TestFrontendDesignSkillIsNotInjectedIntoUnrelatedTask(t *testing.T) {
	provider := newWorkflowContextProvider("为 Go 后端的分页函数补充单元测试")
	if strings.Contains(provider.SystemPrompt(), frontendDesignBodyMarker) {
		t.Fatalf("非前端任务不应注入 frontend-design")
	}
}

func TestExplicitSkillNameForcesPreload(t *testing.T) {
	provider := newWorkflowContextProvider("请使用 frontend-design 完成这项工作")
	if !strings.Contains(provider.SystemPrompt(), frontendDesignBodyMarker) {
		t.Fatalf("任务显式点名技能时必须自动加载正文")
	}
}

func TestAutoLoadedSkillIsIncludedInContextBreakdown(t *testing.T) {
	matched := newWorkflowContextProvider("设计一个网站前端")
	unmatched := newWorkflowContextProvider("检查 Go 服务日志")
	if matched.Breakdown()["skill"] <= unmatched.Breakdown()["skill"] {
		t.Fatalf("自动加载的技能正文未计入 context breakdown")
	}
}
