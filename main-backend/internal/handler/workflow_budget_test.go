package handler

import (
	"strings"
	"testing"
)

// 轮次/token 预算任一触顶都该终止，且带上对应的原因文案。
func TestCodeWorkflowExhausted(t *testing.T) {
	if done, reason := codeWorkflowExhausted(5, 100, 100, 60, 500000); done {
		t.Fatalf("轮次和 token 都远没到上限，不该判耗尽，reason=%q", reason)
	}
	if done, reason := codeWorkflowExhausted(60, 100, 100, 60, 500000); !done {
		t.Fatal("round>=maxRounds 应判耗尽")
	} else if !strings.Contains(reason, "最大迭代轮数") {
		t.Errorf("轮次触顶的 reason 应提到最大迭代轮数，got %q", reason)
	}
	if done, reason := codeWorkflowExhausted(5, 400000, 200000, 60, 500000); !done {
		t.Fatal("input+output token 达到预算应判耗尽")
	} else if !strings.Contains(reason, "token 预算") {
		t.Errorf("token 触顶的 reason 应提到 token 预算，got %q", reason)
	}
}

// CODE_WORKFLOW_MAX_TOKENS 环境变量应覆盖默认预算；未设置或非法值时退回默认。
func TestCodeWorkflowTokenBudgetEnvOverride(t *testing.T) {
	t.Setenv("CODE_WORKFLOW_MAX_TOKENS", "100")
	if got := codeWorkflowTokenBudget(); got != 100 {
		t.Fatalf("env=100 时预算应为 100，got %d", got)
	}

	t.Setenv("CODE_WORKFLOW_MAX_TOKENS", "")
	if got := codeWorkflowTokenBudget(); got != codeWorkflowMaxTokensDefault {
		t.Fatalf("未设置 env 时应退回默认值 %d，got %d", codeWorkflowMaxTokensDefault, got)
	}

	t.Setenv("CODE_WORKFLOW_MAX_TOKENS", "not-a-number")
	if got := codeWorkflowTokenBudget(); got != codeWorkflowMaxTokensDefault {
		t.Fatalf("非法 env 值应退回默认值 %d，got %d", codeWorkflowMaxTokensDefault, got)
	}
}
