package handler

// company_multi_tpl.go —— 多文件交付的兜底模板与工具函数。
// 模型不可用时保证工程结构完整、门禁不挂死；可用时这些只是回退，不是默认路径。

import "strings"

// deliveryStripJSFence 剥离 JS/CSS 输出的 markdown 围栏（```js / ```css / ```）。
func deliveryStripJSFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if idx := strings.Index(s, "\n"); idx >= 0 {
			s = s[idx+1:]
		} else {
			s = ""
		}
	}
	if strings.HasSuffix(s, "```") {
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = strings.TrimSpace(s[:idx])
		}
	}
	return strings.TrimSpace(s)
}

// companyDataJSTemplate 数据层兜底：localStorage 通用增删改查。
func companyDataJSTemplate(project string) string {
	key := "yosuri_" + deliverySanitize(project)
	return `// 数据层（localStorage 伪后端）
(function(){
  var KEY='` + key + `';
  function read(){ try{return JSON.parse(localStorage.getItem(KEY))||[]}catch(e){return []} }
  function write(list){ localStorage.setItem(KEY, JSON.stringify(list)) }
  function uid(){ return Date.now().toString(36)+Math.random().toString(36).slice(2,7) }
  window.AppData={
    list:function(){ return read() },
    add:function(item){ var l=read(); item.id=item.id||uid(); item.createdAt=Date.now(); l.unshift(item); write(l); return item },
    update:function(id,patch){ var l=read(); for(var i=0;i<l.length;i++){ if(l[i].id===id){ for(var k in patch){ l[i][k]=patch[k] } break } } write(l); return l },
    remove:function(id){ write(read().filter(function(x){return x.id!==id})); return read() }
  };
})();`
}

// companyAppJSTemplate 逻辑层兜底：通用待办式增删改渲染。
func companyAppJSTemplate(project string) string {
	return `// 应用逻辑
(function(){
  var app=document.getElementById('app');
  function esc(s){ return String(s==null?'':s).replace(/[&<>"]/g,function(c){return{'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[c]}) }
  function render(){
    var items=window.AppData.list();
    var rows=items.length?items.map(function(it){
      return '<li class="row"><span class="txt">'+esc(it.text||it.name||'')+'</span>'+
        '<button class="del" data-id="'+it.id+'">删除</button></li>';
    }).join(''):'<li class="empty">还没有内容，先添加一条吧</li>';
    app.innerHTML='<form id="f"><input id="inp" placeholder="记一条…" required><button type="submit">添加</button></form><ul id="ul">'+rows+'</ul>';
    document.getElementById('f').addEventListener('submit',function(e){
      e.preventDefault();
      var v=document.getElementById('inp').value.trim();
      if(v){ window.AppData.add({text:v}); render() }
    });
    app.querySelectorAll('.del').forEach(function(b){
      b.addEventListener('click',function(){ window.AppData.remove(b.getAttribute('data-id')); render() })
    });
  }
  render();
})();`
}

// companyCSSFallback 样式兜底：亮色响应式。
func companyCSSFallback() string {
	return `:root{--bg:#f5f8ff;--card:#fff;--ink:#1a2b3c;--accent:#1950be;--line:#e2e8f5}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--ink);font:16px/1.6 "Microsoft YaHei","PingFang SC",system-ui,sans-serif}
.shell{max-width:720px;margin:0 auto;padding:32px 20px}
.topbar h1{margin:0 0 6px;font-size:26px}
.topbar .sub{margin:0 0 24px;color:#5b6b7d;font-size:14px}
#f{display:flex;gap:8px;margin-bottom:18px}
#inp{flex:1;padding:12px 14px;border:1px solid var(--line);border-radius:10px;font:inherit;background:var(--card)}
#f button{padding:0 20px;border:0;border-radius:10px;background:var(--accent);color:#fff;font:inherit;font-weight:700;cursor:pointer}
ul{list-style:none;padding:0;margin:0;display:flex;flex-direction:column;gap:10px}
.row{display:flex;align-items:center;justify-content:space-between;gap:12px;background:var(--card);border:1px solid var(--line);border-radius:12px;padding:14px 16px}
.row .txt{flex:1;word-break:break-word}
.row .del{border:1px solid var(--line);background:#fff;color:#b04a4a;border-radius:8px;padding:6px 12px;cursor:pointer}
.empty{color:#8a97a8;text-align:center;padding:24px;background:var(--card);border:1px dashed var(--line);border-radius:12px}
@media(max-width:520px){.shell{padding:20px 14px}.topbar h1{font-size:22px}}`
}
