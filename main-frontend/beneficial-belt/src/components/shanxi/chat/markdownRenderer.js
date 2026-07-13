// 聊天气泡和 code 模式的 step 卡片叙述文字要用同一套 markdown/LaTeX 渲染管线——
// 之前 MessageStepGroup 里的叙述文字是纯 {{ para }} 插值，没有 markdown-it 也没有
// katex，导致 code 模式看不到任何格式化和数学公式。抽成共享模块，两边 import 同一份。
import MarkdownIt from 'markdown-it'
import markdownItKatex from 'markdown-it-katex'
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

const md = new MarkdownIt({ breaks: true, linkify: true, html: true })
md.use(markdownItKatex, { throwOnError: false, errorColor: '#ef4444', strict: false })
md.use(function (md) {
  md.core.ruler.before('normalize', 'math_bracket', function (state) {
    state.src = state.src.replace(/\[([\s\S]*?)\]/g, (match, inner) => {
      if (!/\\[a-zA-Z]+/.test(inner)) return match
      if (/^\s*\${1,2}[\s\S]*\${1,2}\s*$/.test(inner)) return match
      const trimmed = inner.trim()
      if (trimmed.includes('\n') || trimmed.length > 60 || /\\begin\{/.test(trimmed)) {
        return `$$\n${trimmed}\n$$`
      }
      return `$${trimmed}$`
    })
    return true
  })
  md.core.ruler.after('normalize', 'math_block_spacing', function (state) {
    state.src = normalizeDisplayMath(state.src)
    return true
  })
})

// 零宽空格/不换行空格/从左到右标记等不可见字符——直接在源码里写字面量容易在
// 传输/编辑过程中被悄悄改写，用 fromCharCode 从码位构造，规避这个坑
const INVISIBLE_CHARS_RE = new RegExp('[' + [0x200B, 0x00A0, 0x200E, 0x200F].map(function (c) { return String.fromCharCode(c) }).join('') + ']', 'g')

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
