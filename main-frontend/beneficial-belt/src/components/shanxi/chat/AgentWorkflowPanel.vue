<template>
  <div class="agent-flow" :class="{ streaming: flow.status === 'running' }">
    <template v-for="(b, i) in flow.blocks" :key="i">
      <!-- 思考：可折叠的弱化文本，底色同聊天背景，只靠左侧竖线与正文区分 -->
      <div v-if="b.type === 'thinking'" class="flow-thinking">
        <div class="flow-thinking-label" @click="toggleThink(i)">
          <span class="flow-thinking-text-label">{{ flow.status === 'running' ? '正在思考' : '思考完成' }}</span>
          <span class="flow-chevron" :class="{ open: thinkOpen[i] ?? true }">›</span>
        </div>
        <div v-if="thinkOpen[i] ?? true" class="flow-thinking-text">{{ b.text }}</div>
      </div>

      <!-- 意图/最终回答：直接平铺的 markdown，跟 chat 模式的气泡内容一个样式，
           不带复制按钮这类额外装饰——保持跟 chat 模式一致的简洁 -->
      <div v-else-if="b.type === 'intent'" class="flow-intent markdown-body" v-html="renderMarkdown(b.text, true)"></div>

      <!-- 操作：默认就是一行与正文同样式的白话（"编辑了 xx.go +11 −6"），没有
           徽章/状态图标/边框；点一下才展开下面那张白卡片看 Diff 或命令输出 -->
      <div v-else-if="b.type === 'tool'" class="flow-tool">
        <div class="flow-tool-head" @click="b.expanded = !b.expanded">
          <span class="flow-tool-label">{{ actionText(b) }}</span>
          <span v-if="diffCounts(b)" class="flow-tool-counts">
            <span class="flow-add">+{{ diffCounts(b).added }}</span>
            <span v-if="diffCounts(b).removed" class="flow-del">−{{ diffCounts(b).removed }}</span>
          </span>
          <span v-if="b.status === 'error'" class="flow-tool-failed">失败</span>
          <span class="flow-chevron" :class="{ open: b.expanded }">›</span>
        </div>
        <div v-if="b.expanded" class="flow-tool-body">
          <!-- 内置 edit_file / MCP 的 mcp__fs__edit_file 都走 diff 视图 -->
          <DiffViewer
            v-if="isEdit(b.name)"
            :old-content="editOld(b) || ''"
            :new-content="editNew(b) || ''"
            :path="filePath(b) || ''"
            :start-line="editStartLine(b)"
          />
          <DiffViewer
            v-else-if="isWrite(b.name)"
            old-content=""
            :new-content="fileContent(b) || ''"
            :path="filePath(b) || ''"
          />
          <!-- 读文件：带真实行号的等宽列表，跟 Diff 视图的行号列同一观感，
               而不是一堆无编号的裸文本——引用某一行时用户没法对上号 -->
          <div v-else-if="isRead(b.name)" class="flow-read">
            <div v-for="row in readRows(b)" :key="row.no" class="flow-read-line">
              <span class="flow-read-no">{{ row.no }}</span>
              <code class="flow-read-code">{{ row.text || ' ' }}</code>
            </div>
          </div>
          <pre v-else class="flow-output">{{ toolBodyText(b) }}</pre>
        </div>
      </div>

      <!-- 上下文压缩：一行弱化提示，跟思考块一个视觉重量。让用户知道早期历史被
           折叠成了摘要（省了多少字符），而不是无声改写。 -->
      <div v-else-if="b.type === 'compressed'" class="flow-compressed">
        <span class="flow-compressed-icon">🗜️</span>
        <span>已压缩早期 {{ b.foldedMessages }} 条执行记录以节省上下文（{{ compactChars(b.beforeChars) }} → {{ compactChars(b.afterChars) }}）</span>
      </div>
    </template>
  </div>
</template>

<script setup>
import { reactive } from 'vue'
import { diffLines } from 'diff'
import DiffViewer from './DiffViewer.vue'
import { renderMarkdown } from './markdownRenderer.js'

const props = defineProps({
  flow: { type: Object, required: true }
})

// ==================== 思考块折叠 ====================
// 思考块默认展开（模板用 thinkOpen[i] ?? true），toggle 基于"当前是否可见"取反：
// 默认未点过视为展开，点一下收起，再点展开。
const thinkOpen = reactive({})
function toggleThink(i) { thinkOpen[i] = !(thinkOpen[i] ?? true) }

// ==================== 动作行文案 ====================
// 一行白话，动词 + 对象，读起来跟正文一样（"编辑了 tools.go"），不靠图标传达语义。
// 运行中把"了"换成"正在…"，这样连状态图标也省了。
const VERBS = {
  read_file: '读取',
  mcp__fs__read_file: '读取',
  mcp__fs__read_text_file: '读取',
  mcp__grep__read_range: '读取',
  write_file: '写入',
  mcp__fs__write_file: '写入',
  mcp__fs__create_file: '新建',
  edit_file: '编辑',
  mcp__fs__edit_file: '编辑',
  execute_command: '运行',
  search_codebase: '搜索代码库',
  codegraph_query: '分析调用链',
  search_memory: '检索记忆',
  dispatch_agent: '派发子代理',
  web_search: '联网搜索'
}

function baseName(p) {
  const s = String(p || '')
  const i = Math.max(s.lastIndexOf('/'), s.lastIndexOf('\\'))
  return i >= 0 ? s.slice(i + 1) : s
}

// 动作对象：文件类取文件名（全路径太长且没信息量），命令类取命令原文，其余取首个参数
function target(b) {
  const a = b.args || {}
  if (a.path) return baseName(a.path)
  const v = a.command || a.task || a.query || Object.values(a)[0] || ''
  const s = String(v)
  return s.length > 48 ? s.slice(0, 48) + '…' : s
}

function actionText(b) {
  // load_tools 只是按需取 MCP 工具 schema 的内部动作，把一串 mcp__fs__read_file,
  // mcp__fs__edit_file 摊开念出来对用户没有信息量，只有噪音——统一成一句轻量提示
  if (b.name === 'load_tools') return b.status === 'running' ? '加载 MCP 工具中…' : '加载了 MCP 工具'
  const verb = VERBS[b.name] || (b.name.startsWith('mcp__') ? b.name.split('__').slice(1).join(' · ') : b.name)
  const obj = target(b)
  const running = b.status === 'running'
  // 读文件时把 head/tail/行范围（偏移和限制）显式带出来，否则用户以为每次都读全文
  const range = isRead(b.name) ? readRangeLabel(b) : ''
  const suffix = range ? `（${range}）` : ''
  if (!obj) return running ? `正在${verb}` : `${verb}了`
  return running ? `正在${verb} ${obj}${suffix}` : `${verb}了 ${obj}${suffix}`
}

// 只有写/改文件才有增删行数（对齐设计稿的 "+11 −6"）；其它工具返回 null 不显示。
// 模板里一行要问三次（有没有、加了几、删了几），而流式期间每来一个 token 就重渲染一遍，
// 不缓存就是对着整份文件反复跑 diff。工具块的 args 落定后不再变，按块缓存是安全的。
const countsCache = new WeakMap()
function diffCounts(b) {
  if (countsCache.has(b)) return countsCache.get(b)
  const v = computeDiffCounts(b)
  countsCache.set(b, v)
  return v
}
function computeDiffCounts(b) {
  if (!isEdit(b.name) && !isWrite(b.name)) return null
  const oldStr = isEdit(b.name) ? editOld(b) : ''
  const newStr = isEdit(b.name) ? editNew(b) : fileContent(b)
  let added = 0, removed = 0
  for (const p of diffLines(oldStr || '', newStr || '')) {
    if (!p.added && !p.removed) continue
    const lines = p.value.split('\n')
    if (lines[lines.length - 1] === '') lines.pop()
    if (p.added) added += lines.length
    else removed += lines.length
  }
  return (added || removed) ? { added, removed } : null
}

function isEdit(name) { return name === 'edit_file' || name === 'mcp__fs__edit_file' }
function isWrite(name) { return name === 'write_file' || name === 'mcp__fs__write_file' || name === 'mcp__fs__create_file' }
function isRead(name) {
  return name === 'read_file' || name === 'mcp__fs__read_file' ||
    name === 'mcp__fs__read_text_file' || name === 'mcp__grep__read_range'
}

// 把各种"读一段"的参数翻成人话贴在动作行尾（偏移和限制）。之前前端只认老 native tool
// 的 start_line/end_line，MCP 的 head/tail、自研 read_range 的 start/end 都没显示，
// 所以读文件看起来永远是"读全文"。覆盖三套命名：
//   mcp__fs__read_text_file → head / tail（头/尾 N 行）
//   mcp__grep__read_range   → start / end（第 X–Y 行，能读中间任意段）
//   老 native read_file      → start_line / end_line / mode=outline
function readRangeLabel(b) {
  const a = b.args || {}
  const head = parseInt(a.head, 10)
  const tail = parseInt(a.tail, 10)
  if (Number.isFinite(head)) return `前 ${head} 行`
  if (Number.isFinite(tail)) return `后 ${tail} 行`
  const s = parseInt(a.start ?? a.start_line, 10)
  const e = parseInt(a.end ?? a.end_line, 10)
  if (Number.isFinite(s) && Number.isFinite(e)) return `第 ${s}–${e} 行`
  if (Number.isFinite(s)) return `第 ${s} 行起`
  if (a.mode === 'outline') return '骨架'
  return ''
}

function compactChars(n) {
  if (!n) return '0'
  if (n >= 1000) return (n / 1000).toFixed(1).replace(/\.0$/, '') + 'k'
  return String(n)
}

// read 输出的行号来源不统一：
//   - range 模式（native read_file / mcp__grep__read_range）每行已带真实行号 "12:内容"
//   - outline 模式带 "L12  内容"
//   直接解析出来用；
//   - 全文模式（read_text_file 最常见）不带行号，退化为从 start/start_line（有就用，没有 1）顺序编号。
// read_range 的首行是 "# 路径 第 X-Y 行(...)" 元信息，不是正文，跳过不显示。
function readRows(b) {
  const raw = b.output || ''
  if (!raw) return []
  const lines = raw.split('\n')
  if (lines.length && lines[lines.length - 1] === '') lines.pop()
  const startArg = parseInt(b.args?.start ?? b.args?.start_line, 10)
  let base = Number.isFinite(startArg) ? startArg : 1
  const rows = []
  for (const line of lines) {
    if (/^#\s/.test(line)) continue // read_range 的头部元信息行
    const rangeMatch = /^(\d+):(.*)$/.exec(line)
    if (rangeMatch) { rows.push({ no: rangeMatch[1], text: rangeMatch[2] }); continue }
    const outlineMatch = /^L(\d+)\s+(.*)$/.exec(line)
    if (outlineMatch) { rows.push({ no: outlineMatch[1], text: outlineMatch[2] }); continue }
    rows.push({ no: base + rows.length, text: line })
  }
  return rows
}

// MCP filesystem 的 edit_file 真实 schema：{ path, edits: [{oldText, newText}] }（数组，
// 每项一对 oldText/newText）。内置 edit_file 是 { path, old_string, new_string }（单数）。
// 两者都要兼容。write_file 内容字段内置/MCP 都是 content，path 都是 path。
function editOld(b) {
  const a = b.args || {}
  if (a.old_string) return a.old_string
  if (a.oldText) return a.oldText
  if (Array.isArray(a.edits) && a.edits[0]) return a.edits[0].oldText || ''
  return ''
}
function editNew(b) {
  const a = b.args || {}
  if (a.new_string) return a.new_string
  if (a.newText) return a.newText
  if (Array.isArray(a.edits) && a.edits[0]) return a.edits[0].newText || ''
  return ''
}
function fileContent(b) { const a = b.args || {}; return a.content || '' }
function filePath(b) { const a = b.args || {}; return a.path || '' }

// edit_file 结果里带 "第 N 行"，用来给 Diff 做行号偏移
function editStartLine(b) {
  const m = /第\s*(\d+)\s*行/.exec(b.output || '')
  return m ? parseInt(m[1]) : 1
}

function toolBodyText(b) {
  const out = b.output || (b.status === 'running' ? '执行中…' : '(无输出)')
  if (b.name === 'execute_command') return `$ ${b.args.command || ''}\n\n${out}`
  if (b.name === 'dispatch_agent') return `任务：${b.args.task || ''}\n\n${out}`
  return out
}
</script>

<style scoped>
.agent-flow {
  max-width: 100%;
  padding: 2px 0;
}

/* ---------- 思考 ---------- */
.flow-thinking {
  margin: 6px 0;
}
/* 上下文压缩提示：跟思考块一样"轻"，不抢注意力——它是后台省 token 的动作，
   不是用户要读的内容。左侧一条竖线 + 弱化文字。 */
.flow-compressed {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 6px 0;
  padding: 3px 0 3px 12px;
  border-left: 2px solid var(--app-border, #e2e8f0);
  font-size: 12px;
  color: var(--app-text-faint, #94a3b8);
}
.flow-compressed-icon { font-size: 12px; opacity: 0.8; }
.flow-thinking-label {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 17px;
  cursor: pointer;
  user-select: none;
}
/* 白光字体表面扫描：只挂在文字 span 上，chevron 留在 clip 外避免被裁重叠 */
.flow-thinking-text-label {
  color: #1e293b;
  background: linear-gradient(100deg, #1e293b 40%, #ffffff 50%, #1e293b 60%);
  background-size: 250% 100%;
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  animation: reasonShimmer 5s linear infinite;
}
.agent-flow:not(.streaming) .flow-thinking-text-label {
  animation: none;
}
/* 思考正文不再是一块灰底色块——底色跟聊天背景一样（透明），只留左侧一条竖线
   把它和正文区分开，视觉上"轻"下去，不跟回答抢注意力 */
.flow-thinking-text {
  margin-top: 4px;
  padding: 2px 0 2px 12px;
  font-size: 13px;
  line-height: 1.7;
  color: #94a3b8;
  background: transparent;
  border-left: 2px solid rgba(148, 163, 184, 0.35);
  white-space: pre-wrap;
  word-break: break-word;
}

/* ---------- 意图/最终回答 ---------- */
/* 合并后普通对话的 bot 回答就渲染在这里。chat-window.css 里把用户气泡
   和 .assistant-message 都提到了 17px（Claude 风格），但 agentflow 面板是
   scoped 样式、不吃那条规则，原本硬编码 14px —— 于是合并后 bot 字
   明显比用户小。这里对齐到 17px，落差消失。 */
.flow-intent {
  margin: 6px 0;
  font-size: 17px;
  line-height: 1.75;
  color: #1e293b;
  word-break: break-word;
}

/* ---------- 操作行 ---------- */
/* 收起态就是一行正文：无边框、无底色、无徽章，字号字色跟 .flow-intent 一致，
   读起来像在叙述而不是像一张控件卡片。白卡片留给展开后的 Diff / 输出。 */
.flow-tool {
  margin: 6px 0;
}
.flow-tool-head {
  display: flex;
  align-items: baseline;
  gap: 6px;
  cursor: pointer;
  user-select: none;
  padding: 1px 0;
}
.flow-tool-label {
  min-width: 0;
  font-size: 17px;
  line-height: 1.75;
  color: #1e293b;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.flow-tool-head:hover .flow-tool-label {
  text-decoration: underline;
  text-decoration-color: rgba(148, 163, 184, 0.5);
  text-underline-offset: 3px;
}
.flow-tool-counts {
  flex-shrink: 0;
  display: inline-flex;
  gap: 5px;
  font-size: 13px;
  font-weight: 600;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
}
.flow-add { color: #12b76a; }
.flow-del { color: #d94834; }
.flow-tool-failed {
  flex-shrink: 0;
  font-size: 13px;
  color: #d94834;
}
.flow-chevron {
  flex-shrink: 0;
  color: #a3a3a3;
  font-size: 14px;
  transition: transform 0.15s;
  display: inline-block;
  /* 折叠朝右▸，展开向下▾ */
}
.flow-chevron.open {
  transform: rotate(90deg);
}
/* 展开态才出现的白卡片：真正装 Diff / 命令输出的地方 */
.flow-tool-body {
  margin: 6px 0 2px;
  border: 1px solid #e5e5e5;
  border-radius: 10px;
  background: #fff;
  padding: 8px 12px;
  overflow: hidden;
}
.flow-read {
  max-height: 320px;
  overflow: auto;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
  font-size: 12px;
  line-height: 1.6;
}
.flow-read-line {
  display: flex;
  align-items: flex-start;
}
.flow-read-no {
  flex-shrink: 0;
  width: 34px;
  text-align: right;
  padding-right: 10px;
  color: #a3a3a3;
  user-select: none;
}
.flow-read-code {
  flex: 1;
  min-width: 0;
  color: #262626;
  background: transparent;
  white-space: pre-wrap;
  word-break: break-all;
}
.flow-output {
  margin: 0;
  max-height: 320px;
  overflow: auto;
  font-size: 12px;
  line-height: 1.6;
  color: #4b4741;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
