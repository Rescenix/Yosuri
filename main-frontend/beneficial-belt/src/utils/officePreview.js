/**
 * officePreview.js — 纯前端 Office 文件渲染预览（不进右侧预览窗，不弹浏览器新标签）。
 * 全部用 data:text/html 自包含 HTML，无需后端参与。
 */
import { parsePptxPreview, pptxElementStyle } from './pptxPreview.js'
import { parseXlsxPreview } from './xlsxPreview.js'

// ─── PPTX → HTML 预览（翻页幻灯片） ───
export async function pptxToHtml(buffer) {
  const deck = await parsePptxPreview(buffer)
  if (!deck.slides || deck.slides.length === 0) return '<p>（空幻灯片）</p>'

  const slides = deck.slides
  const w = deck.size?.w || 9144000
  const h = deck.size?.h || 6858000

  let slidesHtml = ''
  for (let i = 0; i < slides.length; i++) {
    const els = slides[i].elements || []
    let inner = ''
    for (const el of els) {
      const style = pptxElementStyle(el, { w, h })
      const s = `left:${style.left};top:${style.top};width:${style.width};height:${style.height};color:${style.color};font-size:${style.fontSize}`
      if (el.type === 'text') {
        const lines = (el.text || '').split('\n').filter(Boolean).map(l => `<div>${esc(l)}</div>`).join('')
        inner += `<div class="el" style="${s}">${lines}</div>`
      } else if (el.type === 'image' && el.src) {
        inner += `<img class="el" style="${s}" src="${el.src}" alt="" />`
      }
    }
    slidesHtml += `<div class="slide" data-i="${i}">${inner}</div>`
  }

  return wrapHtml(`
<style>
*{margin:0;padding:0;box-sizing:border-box}
html,body{width:100%;height:100%;overflow:hidden;background:#f0f0f2;font-family:system-ui,-apple-system,sans-serif}
.stage{position:relative;width:100vw;height:calc(100vh - 48px);margin:0 auto}
.slide{position:absolute;inset:0;display:none;background:#fff;box-shadow:0 2px 12px rgba(0,0,0,.08);margin:12px;border-radius:6px;overflow:hidden}
.slide[data-active]{display:block}
.el{position:absolute;overflow:hidden;word-break:break-word;padding:2px}
.nav{position:fixed;bottom:0;left:0;right:0;display:flex;align-items:center;justify-content:center;gap:16px;height:48px;background:#fff;border-top:1px solid #e2e8f0;font-size:14px;z-index:10}
.nav button{width:36px;height:36px;border-radius:50%;border:1px solid #d1d5db;background:#fff;cursor:pointer;font-size:16px}
.nav button:hover{background:#f3f4f6}
.nav button:disabled{opacity:.3;cursor:default}
</style>
<div class="stage" id="S">${slidesHtml}</div>
<div class="nav">
  <button id="P" disabled>‹</button>
  <span id="C">1 / ${slides.length}</span>
  <button id="N">›</button>
</div>
<script>
(function(){let s=0,t=document.getElementById('S'),p=document.getElementById('P'),n=document.getElementById('N'),c=document.getElementById('C');
function show(i){s=((i%${slides.length})+${slides.length})%${slides.length};
t.querySelectorAll('.slide').forEach((el,idx)=>el.toggleAttribute('data-active',idx===s));
c.textContent=(s+1)+' / ${slides.length}';p.disabled=s===0;n.disabled=s===${slides.length}-1}
p.onclick=()=>show(s-1);n.onclick=()=>show(s+1);document.addEventListener('keydown',e=>{if(e.key==='ArrowLeft')show(s-1);if(e.key==='ArrowRight')show(s+1)});show(0)})();<`+`/script>`)
}

// ─── XLSX → HTML 预览（表格） ───
export async function xlsxToHtml(buffer) {
  const { rows, truncated } = await parseXlsxPreview(buffer)
  if (!rows || rows.length === 0) return '<p>（空表格）</p>'

  let body = ''
  for (let ri = 0; ri < rows.length; ri++) {
    const cells = rows[ri] || []
    const tag = ri === 0 ? 'th' : 'td'
    body += '<tr>' + cells.map(c => `<${tag}>${esc(c)}</${tag}>`).join('') + '</tr>'
  }
  const note = truncated ? '<p style="color:#94a3b8;font-size:13px;text-align:center;margin-top:8px">（仅显示前 40 行，文件完整）</p>' : ''

  return wrapHtml(`
<style>
body{font-family:system-ui,-apple-system,sans-serif;padding:24px;margin:0;background:#f8f9fa}
table{width:100%;border-collapse:collapse;background:#fff;box-shadow:0 1px 4px rgba(0,0,0,.06);border-radius:8px;overflow:hidden}
th,td{padding:8px 12px;text-align:left;border-bottom:1px solid #e2e8f0;font-size:14px;line-height:1.5}
th{background:#f1f5f9;font-weight:600;color:#1e293b}
tr:last-child td{border-bottom:none}
</style>
<table>${body}</table>${note}`)
}

// ─── DOCX → HTML 预览（段落/标题/表格） ───
export async function docxToHtml(buffer) {
  const { default: JSZip } = await import('jszip')
  const zip = await JSZip.loadAsync(buffer)
  const parser = new DOMParser()
  const docFile = zip.file('word/document.xml')
  if (!docFile) return '<p>无法解析 docx 结构</p>'
  const xml = parser.parseFromString(await docFile.async('text'), 'application/xml')

  const nsResolver = (prefix) => ({
    w: 'http://schemas.openxmlformats.org/wordprocessingml/2006/main',
  }[prefix] || null)

  const body = xml.getElementsByTagNameNS('http://schemas.openxmlformats.org/wordprocessingml/2006/main', 'body')[0]
  if (!body) return '<p>空文档</p>'

  let html = ''
  for (const node of Array.from(body.children || [])) {
    const local = node.localName || ''
    if (local === 'p') {
      // 段落
      const pStyle = node.getElementsByTagNameNS('http://schemas.openxmlformats.org/wordprocessingml/2006/main', 'pStyle')[0]
      const styleId = pStyle?.getAttribute('w:val') || ''
      const texts = Array.from(node.getElementsByTagNameNS('http://schemas.openxmlformats.org/wordprocessingml/2006/main', 't'))
        .map(t => t.textContent || '').join('')
      if (!texts.trim()) { html += '<br/>'; continue }

      if (styleId.startsWith('Heading1') || styleId.startsWith('heading1')) {
        html += `<h1>${esc(texts)}</h1>`
      } else if (styleId.startsWith('Heading2') || styleId.startsWith('heading2')) {
        html += `<h2>${esc(texts)}</h2>`
      } else if (styleId.startsWith('Heading3') || styleId.startsWith('heading3')) {
        html += `<h3>${esc(texts)}</h3>`
      } else if (styleId === 'Title') {
        html += `<h1 style="text-align:center">${esc(texts)}</h1>`
      } else {
        const numPr = node.getElementsByTagNameNS('http://schemas.openxmlformats.org/wordprocessingml/2006/main', 'numPr')[0]
        if (numPr) {
          html += `<li>${esc(texts)}</li>`
        } else {
          html += `<p>${esc(texts)}</p>`
        }
      }
    } else if (local === 'tbl') {
      // 表格
      html += '<table>'
      const rows = node.getElementsByTagNameNS('http://schemas.openxmlformats.org/wordprocessingml/2006/main', 'tr')
      for (const row of Array.from(rows)) {
        html += '<tr>'
        const cells = row.getElementsByTagNameNS('http://schemas.openxmlformats.org/wordprocessingml/2006/main', 'tc')
        for (const cell of Array.from(cells)) {
          const cellTexts = Array.from(cell.getElementsByTagNameNS('http://schemas.openxmlformats.org/wordprocessingml/2006/main', 't'))
            .map(t => t.textContent || '').join('')
          html += `<td>${esc(cellTexts)}</td>`
        }
        html += '</tr>'
      }
      html += '</table>'
    }
  }

  return wrapHtml(`
<style>
body{font-family:system-ui,-apple-system,Microsoft YaHei,sans-serif;padding:24px 32px;max-width:800px;margin:0 auto;color:#1e293b;line-height:1.8;background:#fff}
h1{font-size:24px;margin:24px 0 12px;color:#1e293b;border-bottom:2px solid #e2e8f0;padding-bottom:6px}
h2{font-size:20px;margin:20px 0 10px;color:#334155}
h3{font-size:17px;margin:16px 0 8px;color:#475569}
p{margin:6px 0;font-size:15px;text-align:justify}
li{margin:4px 0 4px 24px;font-size:15px}
table{width:100%;border-collapse:collapse;margin:12px 0;background:#fff;box-shadow:0 1px 4px rgba(0,0,0,.06);border-radius:6px;overflow:hidden}
td{padding:6px 10px;border:1px solid #e2e8f0;font-size:14px}
br{margin:4px}
</style>
${html}`)
}

// ─── 工具函数 ───
function esc(s) {
  return String(s || '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
}

function wrapHtml(body) {
  return 'data:text/html;charset=utf-8,' + encodeURIComponent('<!doctype html><html><head><meta charset="utf-8"></head><body>' + body + '</body></html>')
}