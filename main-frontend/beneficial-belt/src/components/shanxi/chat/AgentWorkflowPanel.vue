<template>
  <div class="agent-flow" :class="{ streaming: flow.status === 'running' }">
    <!--
      ★ 按顺序渲染，但「回复(intent)」始终平铺可见；
      连续出现的「思考 + 工具调用」才收纳进同一个概要栏 + 可折叠时间线。
    -->
    <template v-for="(group, gIdx) in blockGroups" :key="gIdx">
      <!-- 可直接见的回复文本 -->
      <div
        v-if="group.type === 'visible'"
        class="flow-intent markdown-body"
        v-html="renderMarkdown(group.text, true)"
      ></div>

      <!-- 单步思考：不收束，直接平铺 -->
      <div v-else-if="group.type === 'single-thinking'" class="flow-thinking flow-thinking-single">
        <div class="flow-row-head" @click="toggleThink(`single-${gIdx}`)">
          <Icon icon="mdi:sparkles" class="flow-row-icon icon-think" width="13" />
          <span class="flow-thinking-text-label">{{ flow.status === 'running' ? '正在思考' : '思考' }}</span>
          <span v-if="!(thinkOpen[`single-${gIdx}`] ?? true) && group.block.text" class="flow-row-preview">{{ onelinePreview(group.block.text) }}</span>
          <span v-else class="flow-spacer"></span>
          <span class="flow-chevron" :class="{ open: thinkOpen[`single-${gIdx}`] ?? true }">›</span>
        </div>
        <div v-if="thinkOpen[`single-${gIdx}`] ?? true" class="flow-detail">
          <div class="flow-thinking-text">{{ group.block.text }}</div>
        </div>
      </div>

      <!-- ask_user 提问：平铺显示「问了什么 / 答了什么」 -->
      <div v-else-if="group.type === 'question'" class="flow-question">
        <div class="flow-question-head">
          <Icon icon="mdi:help-circle-outline" class="flow-row-icon icon-question" width="14" />
          <span class="flow-question-q">{{ group.block.question }}</span>
        </div>
        <div v-if="group.block.options && group.block.options.length" class="flow-question-opts">
          <span
            v-for="(o, i) in group.block.options"
            :key="i"
            class="flow-question-opt"
            :class="{ chosen: isChosenAnswer(group.block, o.value || o.label) }"
          >{{ o.label }}</span>
        </div>
        <div class="flow-question-a">
          <span class="flow-question-a-label">回答</span>
          <span class="flow-question-a-text">{{ group.block.answer || '（等待中…）' }}</span>
        </div>
      </div>

      <!-- 收纳起来的工具/思考时间线 -->
      <template v-else>
        <div class="flow-summary" @click="summaryExpanded[gIdx] = !summaryExpanded[gIdx]">
          <div class="flow-summary-main">
            <Icon icon="mdi:star-four-points" width="16" class="flow-summary-icon" />
            <span class="flow-summary-text">{{ groupSummaryTitle(group) }}</span>
          </div>
          <div class="flow-summary-badges">
            <span v-if="groupRunningCount(group)" class="flow-summary-badge running">
              <span class="flow-summary-dot"></span>{{ groupRunningCount(group) }} 进行中
            </span>
            <span v-if="groupThinkingCount(group)" class="flow-summary-badge think">
              <Icon icon="mdi:star-four-points" width="11" /> {{ groupThinkingCount(group) }} 思考
            </span>
            <span v-if="groupToolCount(group)" class="flow-summary-badge tool">
              <Icon icon="mynaui:tool" width="11" /> {{ groupToolCount(group) }} 操作
            </span>
            <span v-if="groupReadCount(group)" class="flow-summary-badge read">{{ groupReadCount(group) }} 读</span>
            <span v-if="groupWriteCount(group)" class="flow-summary-badge write">{{ groupWriteCount(group) }} 写</span>
            <span v-if="groupEditCount(group)" class="flow-summary-badge edit">{{ groupEditCount(group) }} 改</span>
          </div>
          <span class="flow-chevron" :class="{ open: summaryExpanded[gIdx] }">›</span>
        </div>

        <Transition name="flow-body">
          <div v-show="summaryExpanded[gIdx]" class="flow-body">
            <template v-for="(b, i) in group.blocks" :key="`${gIdx}-${i}`">
              <!-- 思考 -->
              <div v-if="b.type === 'thinking'" class="flow-thinking flow-thinking-timeline">
                <div class="flow-row-head" @click.stop="toggleThink(`${gIdx}-${i}`)">
                  <Icon icon="mdi:sparkles" class="flow-row-icon icon-think" width="13" />
                  <span class="flow-thinking-text-label">{{ flow.status === 'running' ? '正在思考' : '思考' }}</span>
                  <span v-if="!(thinkOpen[`${gIdx}-${i}`] ?? true) && b.text" class="flow-row-preview">{{ onelinePreview(b.text) }}</span>
                  <span v-else class="flow-spacer"></span>
                  <span class="flow-chevron" :class="{ open: thinkOpen[`${gIdx}-${i}`] ?? true }">›</span>
                </div>
                <div v-if="thinkOpen[`${gIdx}-${i}`] ?? true" class="flow-detail">
                  <div class="flow-thinking-text">{{ b.text }}</div>
                </div>
              </div>

              <!-- 操作 -->
              <div v-else-if="b.type === 'tool'" class="flow-tool flow-tool-timeline">
                <div class="flow-row-head" @click.stop="b.expanded = !b.expanded">
                  <Icon icon="mynaui:tool" class="flow-row-icon icon-tool" width="13" />
                  <span class="flow-tool-label">{{ actionText(b) }}</span>
                  <span v-if="diffCounts(b)" class="flow-tool-counts">
                    <span class="flow-add">+{{ diffCounts(b).added }}</span>
                    <span v-if="diffCounts(b).removed" class="flow-del">−{{ diffCounts(b).removed }}</span>
                  </span>
                  <span class="flow-spacer"></span>
                  <span class="flow-tool-badge" :class="'st-' + b.status">
                    <span v-if="b.status === 'running'" class="flow-badge-dot"></span>{{ toolBadge(b) }}
                  </span>
                  <span class="flow-chevron" :class="{ open: b.expanded }">›</span>
                </div>
                <div v-if="b.expanded" class="flow-detail flow-tool-detail">
                  <div class="flow-tool-body">
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
                    <div v-else-if="isRead(b.name)" class="flow-read">
                      <div v-for="row in readRows(b)" :key="row.no" class="flow-read-line">
                        <span class="flow-read-no">{{ row.no }}</span>
                        <code class="flow-read-code">{{ row.text || ' ' }}</code>
                      </div>
                    </div>
                    <pre v-else class="flow-output">{{ toolBodyText(b) }}</pre>
                  </div>
                </div>
              </div>
            </template>
          </div>
        </Transition>
      </template>
    </template>
  </div>
</template>

<script setup>
import { reactive, computed, ref } from 'vue'
import { Icon } from '@iconify/vue'
import { diffLines } from 'diff'
import DiffViewer from './DiffViewer.vue'
import { renderMarkdown } from './markdownRenderer.js'

const props = defineProps({
  flow: { type: Object, required: true }
})

// ★ 把连续的工具调用和思考收进一组；回复(intent)单独平铺，不收纳
// 例外：只有 1 步且是思考时，不收束，直接平铺
const blockGroups = computed(() => {
  const groups = []
  let current = null
  for (const b of props.flow?.blocks || []) {
    if (b.type === 'thinking' || b.type === 'tool') {
      if (!current || current.type === 'visible' || current.type === 'single-thinking') {
        if (current) groups.push(current)
        current = { type: 'summary', blocks: [b] }
      } else {
        current.blocks.push(b)
      }
    } else {
      if (current) {
        groups.push(current)
        current = null
      }
      if (b.type === 'intent') {
        groups.push({ type: 'visible', text: b.text })
      } else if (b.type === 'question') {
        // ask_user 提问：单独平铺，让用户直接看到「问了什么 / 答了什么」
        groups.push({ type: 'question', block: b })
      }
      // 其他类型（compressed/steer/preview）暂不收纳也不平铺，避免污染回复
    }
  }
  if (current) groups.push(current)

  // 后处理：单步思考不收束
  return groups.map(g => {
    if (g.type === 'summary' && g.blocks.length === 1 && g.blocks[0].type === 'thinking') {
      return { type: 'single-thinking', block: g.blocks[0] }
    }
    return g
  })
})

// 每个 summary 组的展开状态
const summaryExpanded = reactive({})

// 各类型 block 统计（按组）
const groupToolBlocks = (g) => g.blocks.filter(b => b.type === 'tool')
const groupThinkingBlocks = (g) => g.blocks.filter(b => b.type === 'thinking')
const groupRunningCount = (g) => groupToolBlocks(g).filter(b => b.status === 'running').length
const groupThinkingCount = (g) => groupThinkingBlocks(g).length
const groupToolCount = (g) => groupToolBlocks(g).length
const groupReadCount = (g) => groupToolBlocks(g).filter(b => isRead(b.name)).length
const groupWriteCount = (g) => groupToolBlocks(g).filter(b => isWrite(b.name)).length
const groupEditCount = (g) => groupToolBlocks(g).filter(b => isEdit(b.name)).length

function groupSummaryTitle(group) {
  const running = group.blocks.some(b => b.status === 'running')
  if (running) {
    const last = group.blocks[group.blocks.length - 1]
    if (last?.type === 'tool') return actionText(last)
    if (last?.type === 'thinking') return '正在思考…'
    return 'Agent 正在处理…'
  }
  const total = group.blocks.length
  return total ? `已完成 · ${total} 步` : '已完成'
}

// 收起态思考行的一行预览：取首个非空行、压掉空白、截断
function onelinePreview(text) {
  const s = (text || '').replace(/\s+/g, ' ').trim()
  return s.length > 46 ? s.slice(0, 46) + '…' : s
}
// 工具耗时格式：<1s 显示 ms，否则显示 s
function fmtMs(ms) {
  if (!ms || ms < 0) return ''
  return ms < 1000 ? ms + 'ms' : (ms / 1000).toFixed(1) + 's'
}
// 状态徽章文案：完成带耗时、进行中、失败
function toolBadge(b) {
  if (b.status === 'running') return '进行中'
  if (b.status === 'error') return '失败'
  const t = fmtMs(b.elapsedMs)
  return t ? '完成 ' + t : '完成'
}

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
  web_search: '联网搜索',
  mcp__web_search__web_search: '联网搜索',
  mcp__web_fetch__web_fetch: '抓取网页'
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

// 判断某个选项是否被用户的回答命中（answer 可能是「A、B」这类拼接，或自由文本）
function isChosenAnswer(block, value) {
  const ans = (block.answer || '').trim()
  if (!ans) return false
  if (ans === value) return true
  // 多选题答案形如「A、B」，按顿号/、切分后看是否含该 value
  return ans.split(/[、,]/).map(s => s.trim()).includes(value)
}

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
/* .message-row 是 flex 容器，子元素默认按内容宽度收缩（shrink-to-fit）。
   用户气泡 .message-bubble.user 和纯文本回答 .assistant-message 都显式撑了
   width:100%，这里漏了同一条，工具卡片那一列就只有文字本身那么宽，跟上下
   气泡对不齐——width:100% 让它跟其余消息块占满同一条列宽。 */
.agent-flow {
  width: 100%;
  max-width: 100%;
  padding: 2px 0;
}

/* ---------- 概要栏：把一次 agent 回复之间的思考和工具调用都收纳进来 ---------- */
.flow-summary {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px;
  border-radius: 10px;
  background: var(--app-surface-2);
  border: 1px solid var(--app-border-soft);
  cursor: pointer;
  user-select: none;
  transition: background 0.14s ease, border-color 0.14s ease;
}
.flow-summary:hover {
  background: var(--app-surface-3);
  border-color: var(--app-border);
}
.flow-summary-main {
  display: flex;
  align-items: center;
  gap: 7px;
  min-width: 0;
  flex-shrink: 1;
}
.flow-summary-icon {
  flex-shrink: 0;
  color: var(--app-accent);
}
.flow-summary-text {
  font-size: 13.5px;
  font-weight: 600;
  color: var(--app-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.flow-summary-badges {
  display: flex;
  align-items: center;
  gap: 5px;
  margin-left: auto;
  flex-shrink: 0;
}
.flow-summary-badge {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  font-size: 11px;
  font-weight: 600;
  padding: 2px 7px;
  border-radius: 999px;
}
.flow-summary-badge.running {
  color: var(--app-accent);
  background: var(--app-accent-soft);
}
.flow-summary-badge.think {
  color: #8b5cf6;
  background: rgba(139, 92, 246, 0.1);
}
.flow-summary-badge.tool {
  color: var(--app-text-soft);
  background: rgba(100, 116, 139, 0.1);
}
.flow-summary-badge.read { color: #0ea5e9; background: rgba(14, 165, 233, 0.1); }
.flow-summary-badge.write { color: #12b76a; background: rgba(18, 183, 106, 0.1); }
.flow-summary-badge.edit { color: #f59e0b; background: rgba(245, 158, 11, 0.1); }
.flow-summary-dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: currentColor;
  animation: flowSummaryPulse 1.2s ease-in-out infinite;
}
@keyframes flowSummaryPulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.45; transform: scale(0.75); }
}
.flow-summary-icon { color: #8b5cf6; }

/* 折叠/展开动画 */
.flow-body-enter-active,
.flow-body-leave-active {
  transition: all 0.2s ease;
  overflow: hidden;
}
.flow-body-enter-from,
.flow-body-leave-to {
  opacity: 0;
  max-height: 0;
}
.flow-body-enter-to,
.flow-body-leave-from {
  opacity: 1;
  max-height: 800px;
}

/* 展开组本身不缩进：每一条思考 / 工具调用标题都要独占完整一行。 */
.flow-body {
  margin-top: 4px;
}
/* 时间线里的标题：灰色小字、去卡片底 */
.flow-body .flow-row-head,
.flow-thinking-single .flow-row-head {
  position: relative;
  background: transparent;
  border: none;
  padding: 4px 0;
  font-size: 12.5px;
  color: var(--app-text-soft);
}
.flow-body .flow-row-head:hover,
.flow-thinking-single .flow-row-head:hover {
  background: transparent;
}
/* 图标即节点：背后垫一层与聊天背景同色的圆底，把竖线遮住 */
/* 只让展开详情缩进并显示竖线，标题仍和上层内容左对齐。 */
.flow-detail {
  margin: 4px 0 6px 24px;
  padding: 0 0 2px 16px;
  border-left: 2px solid var(--app-border);
  min-width: 0;
}
.flow-tool-detail { margin-top: 6px; }

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
  color: var(--app-text-faint);
}
.flow-compressed-icon { font-size: 12px; opacity: 0.8; }
/* 中途插话提示：跟压缩提示同样"轻"，但左侧竖线用强调色——这是用户自己
   插的一句话，比系统后台动作稍微值得多看一眼，但依然不该抢正文注意力。 */
.flow-steer {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 6px 0;
  padding: 3px 0 3px 12px;
  border-left: 2px solid var(--app-accent, #6366f1);
  font-size: 12px;
  color: var(--app-text-faint);
}
.flow-steer-icon { font-size: 12px; opacity: 0.8; }
/* 自动预览提示：跟插话提示同款弱化条 */
.flow-preview {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 6px 0;
  padding: 3px 0 3px 12px;
  border-left: 2px solid var(--app-accent, #6366f1);
  font-size: 12px;
  color: var(--app-text-faint);
  word-break: break-all;
}
.flow-preview-icon { font-size: 12px; opacity: 0.8; }
/* ---------- 卡片行（思考/工具共用）：仿图1 的一行小卡片 ---------- */
.flow-row-head {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 6px 11px;
  border-radius: 9px;
  background: var(--app-surface-2, var(--app-code-bg));
  border: 1px solid var(--app-border-soft, #ededf0);
  cursor: pointer;
  user-select: none;
  font-size: 13.5px;
  transition: background 0.14s ease, border-color 0.14s ease;
}
.flow-row-head:hover {
  background: var(--app-surface-3);
  border-color: #e2e2e6;
}
.flow-row-icon { flex-shrink: 0; }
.icon-think { color: #8b5cf6; }
.icon-tool { color: var(--app-text-soft); }
.flow-row-preview {
  flex: 1;
  min-width: 0;
  color: var(--app-text-faint);
  font-size: 12.5px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.flow-spacer { flex: 1; }
/* 状态徽章：完成(绿)/进行中(灰+脉冲点)/失败(红)，胶囊底 */
.flow-tool-badge {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 11.5px;
  font-weight: 600;
  padding: 1.5px 8px;
  border-radius: 999px;
  font-variant-numeric: tabular-nums;
}
.flow-tool-badge.st-ok { color: #12b76a; background: rgba(18, 183, 106, 0.1); }
.flow-tool-badge.st-error { color: #d94834; background: rgba(217, 72, 52, 0.1); }
.flow-tool-badge.st-running { color: var(--app-text-soft); background: rgba(100, 116, 139, 0.1); }
.flow-badge-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
  animation: flowBadgePulse 1.2s ease-in-out infinite;
}
@keyframes flowBadgePulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.4; transform: scale(0.7); }
}

.flow-thinking-label {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 17px;
  cursor: pointer;
  user-select: none;
}
/* 思考标签：默认灰色小字；streaming 时轻微闪烁 */
.flow-thinking-text-label {
  color: var(--app-text-soft);
  font-size: inherit;
}
.agent-flow.streaming .flow-thinking-text-label {
  animation: reasonShimmer 3s linear infinite;
  background: linear-gradient(100deg, var(--app-text-soft) 40%, var(--app-text) 50%, var(--app-text-soft) 60%);
  background-size: 250% 100%;
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}
/* 思考正文：灰色小字，左侧时间线由 .flow-detail 提供 */
.flow-thinking-text {
  padding: 2px 0;
  font-size: 12px;
  line-height: 1.7;
  color: var(--app-text-faint);
  background: transparent;
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
  color: var(--app-text);
  word-break: break-word;
}

/* ---------- 操作行 ---------- */
/* 收起态就是一行正文：无边框、无底色、无徽章，字号字色跟 .flow-intent 一致，
   读起来像在叙述而不是像一张控件卡片。白卡片留给展开后的 Diff / 输出。 */
.flow-tool {
  margin: 6px 0;
}
.flow-tool-label {
  min-width: 0;
  max-width: 60%;
  font-size: inherit;
  line-height: 1.5;
  color: inherit;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
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
  color: var(--app-text-faint);
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
  border: 1px solid var(--app-border);
  border-radius: 10px;
  background: var(--app-surface);
  padding: 8px 12px;
  overflow: hidden;
}
.flow-read {
  max-height: 320px;
  overflow: auto;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
  font-size: 11.5px;
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
  color: var(--app-text-faint);
  user-select: none;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
  font-size: 11.5px;
}
.flow-read-code {
  flex: 1;
  min-width: 0;
  color: var(--app-text);
  background: transparent;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
  font-size: 11.5px;
  white-space: pre-wrap;
  word-break: break-all;
}
.flow-output {
  margin: 0;
  max-height: 320px;
  overflow: auto;
  font-size: 12px;
  line-height: 1.6;
  color: var(--app-text);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
  white-space: pre-wrap;
  word-break: break-all;
}

/* ---------- ask_user 提问块 ---------- */
.flow-question {
  padding: 10px 12px;
  border-radius: 10px;
  background: var(--app-surface-2);
  border: 1px solid var(--app-border-soft);
  margin: 4px 0;
}
.flow-question-head {
  display: flex;
  align-items: flex-start;
  gap: 6px;
}
.icon-question {
  flex-shrink: 0;
  margin-top: 2px;
  color: var(--app-accent);
}
.flow-question-q {
  font-size: 13px;
  color: var(--app-text);
  line-height: 1.5;
  font-weight: 500;
}
.flow-question-opts {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 8px;
}
.flow-question-opt {
  font-size: 12px;
  padding: 3px 10px;
  border-radius: 12px;
  border: 1px solid var(--app-border);
  color: var(--app-text-soft);
  background: var(--app-surface);
}
.flow-question-opt.chosen {
  border-color: var(--app-accent);
  background: var(--app-accent-soft);
  color: var(--app-accent);
}
.flow-question-a {
  display: flex;
  align-items: baseline;
  gap: 6px;
  margin-top: 8px;
}
.flow-question-a-label {
  flex-shrink: 0;
  font-size: 11px;
  color: var(--app-text-faint);
}
.flow-question-a-text {
  font-size: 13px;
  color: var(--app-text);
  line-height: 1.5;
  word-break: break-word;
}
</style>
