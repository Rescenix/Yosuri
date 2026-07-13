<template>
  <div class="diffviewer">
    <div v-if="collapsedRows.length === 0" class="dv-empty">无内容变化</div>
    <template v-for="(row, i) in collapsedRows" :key="i">
      <div v-if="row.type === 'gap'" class="dv-gap">⋯ {{ row.count }} 行未变化 ⋯</div>
      <div v-else class="dv-line" :class="'dv-' + row.type">
        <span class="dv-lineno">{{ (row.type === 'del' ? row.oldNo : row.newNo) ?? '' }}</span>
        <span class="dv-sign">{{ row.type === 'add' ? '+' : row.type === 'del' ? '−' : '' }}</span>
        <code class="dv-code" v-html="highlightLine(row.text) || '&nbsp;'"></code>
      </div>
    </template>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { diffLines } from 'diff'
import hljs from 'highlight.js'

const props = defineProps({
  oldContent: { type: String, default: '' },
  newContent: { type: String, default: '' },
  // 可选：用来猜语言做语法高亮，猜不出来就原样展示不高亮
  path: { type: String, default: '' },
  // edit_file 传入的 old_string/new_string 只是文件片段，行号永远从 1 开始；
  // 后端会算出这段片段在真实文件里的起始行号，传进来做偏移，不然显示的行号和文件对不上
  startLine: { type: Number, default: 1 }
})

const EXT_LANG_MAP = {
  js: 'javascript', jsx: 'javascript', mjs: 'javascript', cjs: 'javascript',
  ts: 'typescript', tsx: 'typescript',
  vue: 'xml', html: 'xml', htm: 'xml', xml: 'xml',
  py: 'python', go: 'go', java: 'java', kt: 'kotlin',
  c: 'c', h: 'c', cpp: 'cpp', hpp: 'cpp', cc: 'cpp',
  cs: 'csharp', rb: 'ruby', php: 'php', rs: 'rust', swift: 'swift',
  css: 'css', scss: 'scss', less: 'less',
  json: 'json', yml: 'yaml', yaml: 'yaml', toml: 'ini', ini: 'ini',
  md: 'markdown', sh: 'bash', bash: 'bash', sql: 'sql'
}

const language = computed(() => {
  if (!props.path) return null
  const ext = props.path.split('.').pop()?.toLowerCase()
  return EXT_LANG_MAP[ext] || null
})

function highlightLine(text) {
  if (text === '') return ''
  try {
    if (language.value && hljs.getLanguage(language.value)) {
      return hljs.highlight(text, { language: language.value }).value
    }
    return hljs.highlightAuto(text).value
  } catch (e) {
    // 高亮失败就退回纯文本，diff 本身不能因为高亮出错而挂掉
    return escapeHtml(text)
  }
}
function escapeHtml(s) {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

// jsdiff 输出的是"连续同类型片段"（一段纯新增/纯删除/纯不变），这里拆成逐行、
// 并分别累加旧/新两侧的行号——展示的时候左右各一列行号，跟真实 diff 工具一致
const flatRows = computed(() => {
  const parts = diffLines(props.oldContent || '', props.newContent || '')
  const rows = []
  let oldNo = props.startLine || 1
  let newNo = props.startLine || 1
  for (const part of parts) {
    const lines = part.value.split('\n')
    if (lines.length > 0 && lines[lines.length - 1] === '') lines.pop()
    for (const text of lines) {
      if (part.added) {
        rows.push({ type: 'add', oldNo: null, newNo: newNo++, text })
      } else if (part.removed) {
        rows.push({ type: 'del', oldNo: oldNo++, newNo: null, text })
      } else {
        rows.push({ type: 'ctx', oldNo: oldNo++, newNo: newNo++, text })
      }
    }
  }
  return rows
})

// 长段未变化的上下文折叠成一行提示，只在增删行附近保留几行上下文，
// 避免一个小改动却要把整个文件都摊平展示
const CONTEXT_RADIUS = 3
const COLLAPSE_THRESHOLD = CONTEXT_RADIUS * 2 + 2
const collapsedRows = computed(() => {
  const rows = flatRows.value
  const out = []
  let i = 0
  while (i < rows.length) {
    if (rows[i].type !== 'ctx') { out.push(rows[i]); i++; continue }
    let j = i
    while (j < rows.length && rows[j].type === 'ctx') j++
    const run = rows.slice(i, j)
    if (run.length > COLLAPSE_THRESHOLD) {
      out.push(...run.slice(0, CONTEXT_RADIUS))
      out.push({ type: 'gap', count: run.length - CONTEXT_RADIUS * 2 })
      out.push(...run.slice(-CONTEXT_RADIUS))
    } else {
      out.push(...run)
    }
    i = j
  }
  return out
})
</script>

<style scoped>
.diffviewer {
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
  font-size: 11.5px;
  border: 1px solid #e5e5e5;
  border-radius: 8px;
  overflow: hidden;
  background: #ffffff;
}
.dv-empty {
  padding: 10px 12px;
  color: #a3a3a3;
  font-size: 11.5px;
  text-align: center;
}
.dv-gap {
  padding: 4px 10px;
  font-size: 10.5px;
  color: #a3a3a3;
  text-align: center;
  background: #f5f5f5;
  border-top: 1px solid #ececec;
  border-bottom: 1px solid #ececec;
}
.dv-line {
  display: flex;
  align-items: flex-start;
  line-height: 1.6;
}
.dv-line.dv-add { background: rgba(18, 183, 106, 0.10); }
.dv-line.dv-del { background: rgba(217, 72, 52, 0.08); }
.dv-lineno {
  flex-shrink: 0;
  width: 30px;
  text-align: right;
  padding-right: 8px;
  color: #a3a3a3;
  user-select: none;
}
.dv-sign {
  flex-shrink: 0;
  width: 14px;
  text-align: center;
  font-weight: 700;
}
.dv-line.dv-add .dv-sign { color: #12b76a; }
.dv-line.dv-del .dv-sign { color: #d94834; }
.dv-code {
  flex: 1;
  min-width: 0;
  font-family: "JetBrains Mono", ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  white-space: pre-wrap;       /* 长行智能换行，不再横滚 */
  overflow-wrap: anywhere;     /* 超长无空格 token（如长字符串/路径）也能断行 */
  overflow-x: hidden;          /* 彻底消除横向滚轴 */
  color: #262626;
  background: transparent;
  padding: 0 10px 0 0;
}

/* 全局引入的 highlight.js 主题是给 markdown 深色代码块用的（atom-one-dark），
   直接套用到这里白底上文字会糊成一片——这里自己定义一套浅色 token 配色，
   跟全局深色主题互不干扰（选择器只作用于本组件内的 .dv-code） */
.dv-code :deep(.hljs-comment),
.dv-code :deep(.hljs-quote) { color: #9c9284; font-style: italic; }
.dv-code :deep(.hljs-keyword),
.dv-code :deep(.hljs-selector-tag),
.dv-code :deep(.hljs-literal) { color: #a626a4; }
.dv-code :deep(.hljs-string),
.dv-code :deep(.hljs-attr),
.dv-code :deep(.hljs-regexp) { color: #50a14f; }
.dv-code :deep(.hljs-number) { color: #986801; }
.dv-code :deep(.hljs-title),
.dv-code :deep(.hljs-title.function_),
.dv-code :deep(.hljs-name) { color: #4078f2; }
.dv-code :deep(.hljs-built_in),
.dv-code :deep(.hljs-type) { color: #c18401; }
.dv-code :deep(.hljs-variable),
.dv-code :deep(.hljs-params) { color: #262626; }
.dv-code :deep(.hljs-tag) { color: #e45649; }
</style>
