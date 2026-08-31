// 聊天气泡和 code 模式的 step 卡片叙述文字要用同一套 markdown/LaTeX 渲染管线——
// 之前 MessageStepGroup 里的叙述文字是纯 {{ para }} 插值，没有 markdown-it 也没有
// katex，导致 code 模式看不到任何格式化和数学公式。抽成共享模块，两边 import 同一份。
import MarkdownIt from 'markdown-it'
// 用 @neilsustc 现代分支：老的 markdown-it-katex@2.0.3 内嵌 katex 0.6，
// 不支持 \boxed / aligned 等命令（渲染成红字或整块退化成纯文本）
import markdownItKatex from '@neilsustc/markdown-it-katex'
import DOMPurify from 'dompurify'

// $$...$$ 块级公式跟前面的说明文字挤在同一行时（比如"独立公式：$$\n...\n$$"），
// markdown-it-katex 的 math_block 规则要求 $$ 必须是这一行的第一个字符，挤在一起
// 就直接判定不是块公式，退回普通段落，inline 规则又把相邻的 $$ 当"空内容"吐成裸
// 文本——公式和后面所有内容全部级联炸成纯文本。强制在每对 $$...$$ 前后补空行，
// 保证它永远被识别成独立的块。放在 math_bracket 之后跑，这样 [..] 转出来的 $$ 也
// 能一起被规范化。
function normalizeDisplayMath(src) {
  return src.replace(/\$\$([\s\S]*?)\$\$/g, (match, inner, offset, full) => {
    const before = full.slice(0, offset)
    const after = full.slice(offset + match.length)
    const needLeading = before.length > 0 && !/\n\s*$/.test(before)
    const needTrailing = after.length > 0 && !/^\s*\n/.test(after)
    return (needLeading ? '\n\n' : '') + '$$' + inner + '$$' + (needTrailing ? '\n\n' : '')
  })
}

// 把非代码文本里的各种 LaTeX 定界符统一成 $/$$（markdown-it-katex 只认后者）。
// 顺序敏感：\[...\] / \(...\) 必须先于裸 [...] 启发式，否则 \[ 里的 [ 会被
// 启发式咬掉一半，留下游离的 \ } ] 散落在正文里（数学渲染炸裂的主要成因）。
function convertMathDelimiters(seg) {
  // 1) \[...\] → $$...$$ —— LLM 输出显示公式的标准定界符。
  //    (?<!\\) 防止吃掉公式内部的换行符 \\[4pt]（\\ 后面跟 [4pt] 的情形）
  seg = seg.replace(/(?<!\\)\\\[([\s\S]*?)(?<!\\)\\\]/g, function (_m, inner) {
    return '\n\n$$\n' + inner.trim() + '\n$$\n\n'
  })
  // 2) \(...\) → $...$ —— 行内公式标准定界符
  seg = seg.replace(/(?<!\\)\\\(([\s\S]*?)(?<!\\)\\\)/g, function (_m, inner) {
    return '$' + inner.trim() + '$'
  })
  // 3) 裸 [...]（内含 LaTeX 命令时）——部分模型漏写反斜杠的兜底启发式。
  //    只在已有数学区域（步骤 1/2 转换产物或原生 $..$）之外跑：否则 $$ 块里的
  //    \Bigl[ ... \Bigr] 会被二次咬成 \Bigl$ ... \Bigr$，整块公式报废。
  seg = seg
    .split(/(\$\$[\s\S]*?\$\$|\$[^$\n]+?\$)/g)
    .map(function (part, i) {
      if (i % 2 === 1) return part
      return part.replace(/\[([\s\S]*?)\]/g, function (match, inner) {
        if (!/\\[a-zA-Z]+/.test(inner)) return match
        if (/\$/.test(inner)) return match
        const trimmed = inner.trim()
        if (trimmed.includes('\n') || trimmed.length > 60 || /\\begin\{/.test(trimmed)) {
          return '$$\n' + trimmed + '\n$$'
        }
        return '$' + trimmed + '$'
      })
    })
    .join('')
  return seg
}

// 代码围栏/行内代码里的 \[ \] $ 必须原样保留：按代码片段切分，只变换非代码段。
// 围栏允许未闭合（(?:```|$)）——流式输出中途的半截代码块也要保护住。
const CODE_SPLIT_RE = /(```[\s\S]*?(?:```|$)|~~~[\s\S]*?(?:~~~|$)|`[^`\n]*`)/g
function transformOutsideCode(src, fn) {
  return src
    .split(CODE_SPLIT_RE)
    .map(function (part, i) { return i % 2 === 1 ? part : fn(part) })
    .join('')
}

const md = new MarkdownIt({ breaks: true, linkify: true, html: true })
md.use(markdownItKatex, { throwOnError: false, errorColor: '#ef4444', strict: false })
md.use(function (md) {
  md.core.ruler.before('normalize', 'math_delimiters', function (state) {
    state.src = transformOutsideCode(state.src, convertMathDelimiters)
    return true
  })
  md.core.ruler.after('normalize', 'math_block_spacing', function (state) {
    state.src = transformOutsideCode(state.src, normalizeDisplayMath)
    return true
  })
})

// 零宽空格/不换行空格/从左到右标记等不可见字符——直接在源码里写字面量容易在
// 传输/编辑过程中被悄悄改写，用 fromCharCode 从码位构造，规避这个坑
const INVISIBLE_CHARS_RE = new RegExp('[' + [0x200B, 0x00A0, 0x200E, 0x200F].map(function (c) { return String.fromCharCode(c) }).join('') + ']', 'g')

// 撇号问题（$...$ 内含 r' 时 inline 规则不触发）是旧版 markdown-it-katex 的 bug，
// @neilsustc 分支已根治——枚举测试 p01_apostrophe_inline 验证通过，无需再做占位符保护。

export function renderMarkdown(text, skipSanitize = false) {
  if (!text) return ''
  text = text.replace(INVISIBLE_CHARS_RE, '')
  text = text.replace(/\\dots/g, '\\ldots')
  text = text.replace(/(?<!\$)\\implies(?!\$)/g, ' $\\implies$ ')
  text = text.replace(/(?<!\$)(\\bbox\[[^\]]*\])(?!\$)/g, function (match) { return '$' + match + '$' })
  if (/\\bbox/.test(text)) text = '\\require{bbox}\n' + text
  const raw = md.render(text)
  return skipSanitize ? raw : DOMPurify.sanitize(raw)
}

// ── mermaid 图表（嵌入聊天正文，不算交付文件）──
// 渲染时先把 ```mermaid 围栏转成占位 div（图源码 base64 存 data 属性，躲开
// DOMPurify 对 <svg> 的清洗——mermaid 的 SVG 带 style/foreignObject，sanitize
// 会剥掉导致图表烂掉），之后调用方在 DOM 就绪后调 renderMermaidIn() 批量转换。
// 懒加载：mermaid 是重型依赖（~1MB），动态 import 只在真有图时加载。
md.renderer.rules.fence = function (tokens, idx) {
  const t = tokens[idx]
  if (t.info && t.info.trim().toLowerCase() === 'mermaid') {
    const b64 = btoa(unescape(encodeURIComponent(t.content)))
    return '<div class="mermaid-diagram" data-diagram="' + b64 + '"></div>'
  }
  // 默认 fence 渲染（code block）
  const lang = t.info ? t.info.trim() : ''
  const code = md.utils.escapeHtml(t.content)
  return '<pre><code' + (lang ? ' class="language-' + md.utils.escapeHtml(lang) + '"' : '') + '>' + code + '</code></pre>\n'
}

// renderMermaidIn 把容器内未转换的 .mermaid-diagram 渲染成 SVG。
// 幂等：已经转换过的（有 svg 子节点）跳过。聊天流式渲染时每帧都调它也没事。
let mermaidLoadPromise = null
export async function renderMermaidIn(root) {
  const nodes = (root && typeof root.querySelectorAll === 'function')
    ? root.querySelectorAll('.mermaid-diagram')
    : []
  if (!nodes.length) return
  if (!mermaidLoadPromise) {
    mermaidLoadPromise = import('mermaid').then(m => {
      m.default.initialize({ startOnLoad: false, theme: 'default', securityLevel: 'loose' })
      return m.default
    })
  }
  const mermaid = await mermaidLoadPromise
  for (const el of Array.from(nodes)) {
    if (el.querySelector('svg')) continue
    const b64 = el.getAttribute('data-diagram')
    if (!b64) continue
    try {
      const code = decodeURIComponent(escape(atob(b64)))
      const { svg } = await mermaid.render('mmd-' + Math.random().toString(36).slice(2, 10), code)
      el.innerHTML = svg
      el.classList.add('rendered')
    } catch (e) {
      el.innerHTML = '<div class="mermaid-error">图表渲染失败：' + String(e.message || e) + '</div>'
    }
  }
}
