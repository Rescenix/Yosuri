package handler

import (
	"strings"
	"testing"
)

// TestDeliveryHTMLComplete 单文件 HTML 完整性判定：截断产物（无闭合/无事件绑定）必须拦下。
func TestDeliveryHTMLComplete(t *testing.T) {
	complete := `<!doctype html><html><body><button id="b">x</button><script>document.getElementById('b').addEventListener('click',function(){});</script></body></html>`
	if !deliveryHTMLComplete(complete) {
		t.Fatal("完整产物（闭合+事件绑定）应判完整")
	}
	truncatedTail := `<!doctype html><html><body><button id="b">x</button><script>document.getElementById('b').addEventListener('click',functio`
	if deliveryHTMLComplete(truncatedTail) {
		t.Fatal("缺 </html> 的截断产物不应判完整")
	}
	noEvents := `<!doctype html><html><body><p>静态页</p></body></html>`
	if deliveryHTMLComplete(noEvents) {
		t.Fatal("无 <script> 的产物不应判完整（runnable 需要交互）")
	}
	scriptNoBind := `<!doctype html><html><body><script>var x=1;</script></body></html>`
	if deliveryHTMLComplete(scriptNoBind) {
		t.Fatal("script 无事件绑定不应判完整（假交互）")
	}
}

// TestDeliveryRepairHTMLTail 浅截断修复：剥掉残段补闭合；深度截断不修返回空。
func TestDeliveryRepairHTMLTail(t *testing.T) {
	// 浅截断：逻辑写完（有 addEventListener），最后的 script 段被砍半
	shallow := `<!doctype html><html><body><script>document.getElementById('b').addEventListener('click',f);</script><script>function helper(){`
	got := deliveryRepairHTMLTail(shallow)
	if got == "" {
		t.Fatal("浅截断应可修复")
	}
	if !strings.HasSuffix(got, "</script></body></html>") {
		t.Fatalf("修复后必须闭合: %q", got[len(got)-40:])
	}
	if !strings.Contains(got, "addEventListener") {
		t.Fatal("修复不得丢掉已有的事件绑定")
	}
	if strings.Contains(got, "function helper(){") {
		t.Fatal("未闭合的残段（半行 JS）必须被剥掉，否则语法错")
	}

	// 深度截断：事件绑定整个没写出来
	deep := `<!doctype html><html><body><div>内容</div><scri`
	if got := deliveryRepairHTMLTail(deep); got != "" {
		t.Fatalf("深度截断（无事件绑定）不应修，返回空串让调用方回退模板壳: %q", got)
	}

	// 完全没开头
	if got := deliveryRepairHTMLTail("<p>残段</p>"); got != "" {
		t.Fatalf("无 <html> 开头不修: %q", got)
	}
}
