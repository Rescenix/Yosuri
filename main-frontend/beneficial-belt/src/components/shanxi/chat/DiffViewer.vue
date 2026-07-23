<template>
  <div class="diffviewer">
    <div v-if="collapsedRows.length === 0" class="dv-empty">无内容变化</div>
    <template v-for="(row, i) in collapsedRows" :key="i">
      <div v-if="row.type === 'fold'" class="dv-fold" @click="expandAll">
        <span class="dv-fold-text">{{ row.count }} unchanged lines</span>
      </div>
      <div v-else class="dv-line" :class="'dv-' + row.type">
        <span class="dv-lineno">{{ (row.type === 'del' ? row.oldNo : row.newNo) ?? '' }}</span>
        <span class="dv-sign">{{ row.type === 'add' ? '+' : row.type === 'del' ? '−' : '' }}</span>
        <code class="dv-code" v-html="highlightLine(row.text) || '&nbsp;'"></code>
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { diffLines } from 'diff'
import hljs from 'highlight.js'

const props = defineProps({
  oldContent: { type: String, default: '' },
  newContent: { type: String, default: '' },
  path: { type: String, default: '' },
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
    return escapeHtml(text)
  }
}

function escapeHtml(s) {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

const flatRows = computed(() => {
  const parts = diffLines(props.oldContent || '', props.newContent || '')
  const rows = []
  let oldNo = props.startLine || 1
  let newNo = props.startLine || 1
  for (const part of parts) {
    const lines = part.value.split('\n')
    if (lines.length > 0 && lines[lines.length - 1] === '') lines.pop()
    for (const text of lines) {
      if (part.added) rows.push({ type: 'add', oldNo: null, newNo: newNo++, text })
      else if (part.removed) rows.push({ type: 'del', oldNo: oldNo++, newNo: null, text })
      else rows.push({ type: 'ctx', oldNo: oldNo++, newNo: newNo++, text })
    }
  }
  return rows
})

const CONTEXT_RADIUS = 3
const expanded = ref(false)

function expandAll() {
  expanded.value = true
}

const collapsedRows = computed(() => {
  const rows = flatRows.value
  if (!rows.length) return []

  if (expanded.value) return rows

  const keep = new Array(rows.length).fill(false)
  const source = new Array(rows.length).fill(false)
  for (let i = 0; i < rows.length; i++) {
    if (rows[i].type !== 'ctx') keep[i] = source[i] = true
  }
  for (let i = 0; i < rows.length; i++) {
    if (!source[i]) continue
    for (let d = 1; d <= CONTEXT_RADIUS; d++) {
      const p = i - d
      const q = i + d
      if (p >= 0 && rows[p].type === 'ctx' && !keep[p]) keep[p] = true
      if (q < rows.length && rows[q].type === 'ctx' && !keep[q]) keep[q] = true
    }
  }

  const out = []
  let foldCount = 0

  for (let i = 0; i < rows.length; i++) {
    if (keep[i]) {
      if (foldCount > 0) {
        out.push({ type: 'fold', count: foldCount })
        foldCount = 0
      }
      out.push(rows[i])
    } else {
      foldCount++
    }
  }
  if (foldCount > 0) {
    out.push({ type: 'fold', count: foldCount })
  }
  return out
})
</script>

<style scoped>
.diffviewer {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
  font-size: 11.5px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  overflow: hidden;
  background: var(--app-surface);
}
.dv-empty {
  padding: 10px 12px;
  color: var(--app-text-faint);
  font-size: 11.5px;
  text-align: center;
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
  color: var(--app-text-faint);
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
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
  white-space: pre-wrap;       /* 长行智能换行，不再横滚 */
  overflow-wrap: anywhere;     /* 超长无空格 token（如长字符串/路径）也能断行 */
  overflow-x: hidden;          /* 彻底消除横向滚轴 */
  color: var(--app-text);
  background: transparent;
  padding: 0 10px 0 0;
}

/* 全局引入的 highlight.js 主题是给 markdown 深色代码块用的（atom-one-dark），
   直接套用到这里白底上文字会糊成一片——这里自己定义一套浅色 token 配色，
   跟全局深色主题互不干扰（选择器只作用于本组件内的 .dv-code） */
.dv-code :deep(.hljs-comment),
.dv-code :deep(.hljs-quote) { color: var(--app-text-faint); font-style: italic; }
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
.dv-code :deep(.hljs-params) { color: var(--app-text); }
.dv-code :deep(.hljs-tag) { color: #e45649; }
.dv-fold {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  background: var(--app-surface-2);
  border-top: 1px solid var(--app-border);
  border-bottom: 1px solid var(--app-border);
  color: var(--app-text-soft);
  font-size: 12px;
  user-select: none;
  cursor: pointer;
}
.dv-fold:hover {
  background: var(--app-surface-3);
}
.dv-fold-text {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}
</style>
