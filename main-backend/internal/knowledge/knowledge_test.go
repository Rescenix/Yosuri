package knowledge

// knowledge_test.go —— 外挂知识库核心逻辑的回归测试。
//
// 覆盖：
//   - 分块（splitChunks）：中文文本切成合理片段，不切坏 UTF-8
//   - 检索（Search）：中文关键词能命中知识库文档片段
//   - 兜底（bigram 空集）：空文本/空查询不 panic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitChunksKeepsParagraphs(t *testing.T) {
	text := "第一段内容，关于人工智能的发展和未来趋势。\n\n第二段关于机器学习。\n\n第三段是关于深度学习神经网络。"
	chunks := splitChunks("/tmp/x.md", text)
	if len(chunks) == 0 {
		t.Fatal("分块结果为空")
	}
	joined := strings.Join(chunkContents(chunks), "\n")
	// 全部内容必须保留，不能丢字
	for _, kw := range []string{"人工智能", "机器学习", "深度学习"} {
		if !strings.Contains(joined, kw) {
			t.Errorf("分块丢失内容 %q", kw)
		}
	}
	// 每块不超上限
	for i, c := range chunks {
		if len([]rune(c.Content)) > chunkMax {
			t.Errorf("chunk[%d] 超长 %d > %d", i, len([]rune(c.Content)), chunkMax)
		}
	}
}

func TestSplitChunksHandlesLongUnbrokenParagraph(t *testing.T) {
	// 模拟 PDF 抽出来的一整段无换行长文本
	text := strings.Repeat("这是一段没有换行的超长内容，用来验证硬切逻辑不会死循环或超限。", 200)
	chunks := splitChunks("/tmp/long.md", text)
	if len(chunks) == 0 {
		t.Fatal("长文本分块为空")
	}
	for i, c := range chunks {
		if len([]rune(c.Content)) > chunkMax {
			t.Errorf("chunk[%d] 超长 %d > %d", i, len([]rune(c.Content)), chunkMax)
		}
	}
}

func TestSearchEmptyAndEmptyQuery(t *testing.T) {
	// 空查询返回空
	if got := Search("   ", 3); got != "" {
		t.Errorf("空查询应返回空串，实际 %q", got)
	}
	// 空知识库不 panic 返回空
	if got := Search("随便查查", 3); got != "" {
		t.Errorf("空库应返回空串，实际 %q", got)
	}
}

func TestSearchChineseHit(t *testing.T) {
	dir := Dir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "kb-test-中文.md")
	content := "Rescene 是一个开源 AI Agent 框架，支持多模型路由和记忆引擎。\n\n它使用 Go 编写后端。"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(p)
	invalidateAll()

	hit := Search("模型路由", 2)
	if hit == "" {
		t.Fatal("中文关键词未召回知识库文档")
	}
	if !strings.Contains(hit, "多模型路由") {
		t.Errorf("召回内容不含关键词，实际:\n%s", hit)
	}
}

// chunkContents 提取所有块内容，便于拼接断言。
func chunkContents(chunks []Chunk) []string {
	out := make([]string, len(chunks))
	for i, c := range chunks {
		out[i] = c.Content
	}
	return out
}