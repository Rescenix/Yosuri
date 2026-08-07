package main

// cdp_call.go — CDP 协议通用调用（Page.navigate / Runtime.evaluate）
// 复用 cdp_ws.go 的最小 WebSocket 客户端。

import (
	"encoding/json"
	"fmt"
)

// cdpConn 一次 CDP WebSocket 会话
type cdpConn struct {
	wsURL string
	id    int
}

// cdpDial 建立 CDP 会话（复用 cdpGetAllCookies 的握手逻辑）
func cdpDial(wsURL string) (*cdpConn, error) {
	return &cdpConn{wsURL: wsURL}, nil
}

// call 发送 CDP 命令并等待响应
func (c *cdpConn) call(method string, params map[string]any) ([]byte, error) {
	c.id++
	req := map[string]any{"id": c.id, "method": method}
	if params != nil {
		req["params"] = params
	}
	payload, _ := json.Marshal(req)

	// 每次调用新建连接（简单可靠；发布流程低频调用可接受）
	resp, err := wsCall(c.wsURL, payload)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// cdpNavigate 打开页面
func cdpNavigate(wsURL, url string) error {
	c := &cdpConn{wsURL: wsURL}
	_, err := c.call("Page.navigate", map[string]any{"url": url})
	return err
}

// cdpEval 执行 JS，返回结果值（字符串）
func cdpEval(wsURL, js string) (string, error) {
	c := &cdpConn{wsURL: wsURL}
	resp, err := c.call("Runtime.evaluate", map[string]any{"expression": js, "returnByValue": true})
	if err != nil {
		return "", err
	}
	var r struct {
		Result struct {
			Result struct {
				Type  string          `json:"type"`
				Value json.RawMessage `json:"value"`
			} `json:"result"`
		} `json:"result"`
	}
	if json.Unmarshal(resp, &r) != nil {
		return "", fmt.Errorf("CDP eval 解析失败")
	}
	if len(r.Result.Result.Value) > 0 {
		var v string
		if json.Unmarshal(r.Result.Result.Value, &v) == nil {
			return v, nil
		}
		// bool/number 等非字符串值：返回原始 JSON 文本
		return string(r.Result.Result.Value), nil
	}
	return "", nil
}

// cdpFillForm 智能填表：找到标题/正文输入框填入内容，点提交按钮
// 返回执行后的页面提示（成功/失败特征）
func cdpFillForm(wsURL, title, content string) (string, error) {
	js := fmt.Sprintf(`(function() {
  var titleSet = false, contentSet = false;

  // 1. 标题框：input[type=text] 或 input 中第一个非隐藏
  var inputs = Array.from(document.querySelectorAll('input'));
  var titleEl = inputs.find(function(i) {
    var t = (i.type || 'text').toLowerCase();
    var ph = (i.placeholder || '').toLowerCase();
    return (t === 'text' || t === '') && (ph.indexOf('标题') >= 0 || ph.indexOf('书名') >= 0 || ph.indexOf('章节') >= 0 || ph.indexOf('title') >= 0);
  }) || inputs.find(function(i) { return (i.type || 'text') === 'text' && i.offsetParent !== null; });
  if (titleEl) {
    var setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
    setter.call(titleEl, %q);
    titleEl.dispatchEvent(new Event('input', {bubbles: true}));
    titleEl.dispatchEvent(new Event('change', {bubbles: true}));
    titleSet = true;
  }

  // 2. 正文框：textarea 或 contenteditable，取最大的
  var areas = Array.from(document.querySelectorAll('textarea, [contenteditable="true"]'));
  var contentEl = areas.sort(function(a, b) { return b.value ? b.value.length : b.innerText.length; })[0];
  if (contentEl) {
    if (contentEl.tagName === 'TEXTAREA') {
      var s2 = Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype, 'value').set;
      s2.call(contentEl, %q);
      contentEl.dispatchEvent(new Event('input', {bubbles: true}));
    } else {
      contentEl.innerText = %q;
      contentEl.dispatchEvent(new InputEvent('input', {bubbles: true, inputType: 'insertText', data: %q}));
    }
    contentSet = true;
  }

  return JSON.stringify({title: titleSet, content: contentSet, page: document.title});
})()`, title, content, content, content)
	return cdpEval(wsURL, js)
}

// cdpClickPublish 找发布/提交按钮点击（返回按钮文字或空）
func cdpClickPublish(wsURL string) (string, error) {
	js := `(function() {
  var btns = Array.from(document.querySelectorAll('button, input[type=submit], a.btn'));
  var kw = ['发布', '提交', '保存', '发表', '上传', '确定', 'publish', 'submit'];
  for (var i = 0; i < btns.length; i++) {
    var t = (btns[i].innerText || btns[i].value || '').trim();
    if (t.length < 12 && kw.some(function(k) { return t.indexOf(k) >= 0; })) {
      btns[i].click();
      return t;
    }
  }
  return '';
})()`
	return cdpEval(wsURL, js)
}
