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

// companyCSSFallback 样式兜底：亮色响应式 + 品牌质感（模型挂了也不掉档）。
func companyCSSFallback() string {
	return `:root{--bg:#f4f8f6;--card:#ffffff;--ink:#1a2b3c;--muted:#64748b;--accent:#1f7a55;--accent-deep:#155c40;--accent-soft:#e6f4ec;--line:#dfe7e2;--shadow-sm:0 1px 2px rgba(15,23,42,.05);--shadow-md:0 4px 14px rgba(15,23,42,.08)}
*{box-sizing:border-box}
body{margin:0;background:linear-gradient(180deg,#f7fbf9,#eef4f1);color:var(--ink);font:16px/1.65 -apple-system,"PingFang SC","Microsoft YaHei",system-ui,sans-serif}
.shell{max-width:860px;margin:0 auto;padding:36px 22px 64px}
.topbar{padding:26px 0 20px;border-bottom:2px solid var(--accent);margin-bottom:6px}
.topbar h1{margin:0 0 8px;font-size:28px;font-weight:900;letter-spacing:-.02em}
.topbar .sub{margin:0;color:var(--muted);font-size:14px}
#f{display:flex;gap:10px;margin:22px 0 20px}
#inp{flex:1;padding:13px 15px;border:1.5px solid var(--line);border-radius:12px;font:inherit;background:var(--card);box-shadow:var(--shadow-sm);transition:border-color .18s,box-shadow .18s}
#inp:focus{outline:none;border-color:var(--accent);box-shadow:0 0 0 3px var(--accent-soft)}
#f button{padding:0 24px;border:0;border-radius:12px;background:linear-gradient(135deg,var(--accent),var(--accent-deep));color:#fff;font:inherit;font-weight:700;cursor:pointer;box-shadow:var(--shadow-sm);transition:transform .18s,box-shadow .18s,filter .18s}
#f button:hover{transform:translateY(-1px);box-shadow:var(--shadow-md);filter:brightness(1.05)}
#f button:active{transform:translateY(0)}
ul{list-style:none;padding:0;margin:0;display:flex;flex-direction:column;gap:10px}
.row{display:flex;align-items:center;justify-content:space-between;gap:12px;background:var(--card);border:1px solid var(--line);border-radius:14px;padding:15px 18px;box-shadow:var(--shadow-sm);transition:transform .18s,box-shadow .18s;animation:row-in .28s ease}
.row:hover{transform:translateY(-2px);box-shadow:var(--shadow-md)}
@keyframes row-in{from{opacity:0;transform:translateY(8px)}to{opacity:1;transform:none}}
.row .txt{flex:1;word-break:break-word;font-variant-numeric:tabular-nums}
.row .del{border:1px solid var(--line);background:#fff;color:#b04a4a;border-radius:9px;padding:7px 14px;cursor:pointer;transition:background .18s,border-color .18s}
.row .del:hover{background:#fef2f2;border-color:#fecaca}
.empty{display:flex;flex-direction:column;align-items:center;gap:10px;color:#8a97a8;text-align:center;padding:44px 24px;background:var(--card);border:1.5px dashed var(--line);border-radius:16px}
.empty::before{content:"🗂️";font-size:34px}
@media(max-width:640px){.shell{padding:22px 14px 48px}.topbar h1{font-size:22px}#f{flex-direction:column}#f button{padding:12px}}
@media(min-width:961px){.shell{padding:44px 32px 80px}}`
}
