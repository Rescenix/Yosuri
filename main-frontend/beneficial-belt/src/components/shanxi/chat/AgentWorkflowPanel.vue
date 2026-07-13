<template>
  <div class="agent-flow">
    <template v-for="(b, i) in flow.blocks" :key="i">
      <!-- 思考：折叠的暗色文本块 -->
      <div v-if="b.type === 'thinking'" class="flow-thinking">
        <div class="flow-thinking-label" @click="toggleThink(i)">
          <span class="flow-chevron" :class="{ open: thinkOpen[i] }">›</span>
          思考
        </div>
        <div v-if="thinkOpen[i]" class="flow-thinking-text">{{ b.text }}</div>
      </div>

      <!-- 意图/最终回答：直接平铺的 markdown -->
      <div v-else-if="b.type === 'intent'" class="flow-intent-wrap">
        <div class="flow-intent markdown-body" v-html="renderMarkdown(b.text, true)"></div>
        <div class="flow-intent-tools">
          <button class="tool-btn" @click="copyText(b.text)" title="复制">
            <Icon icon="mdi:content-copy" width="15" />
          </button>
        </div>
      </div>

      <!-- 操作 + 结果：一行卡片，点击展开结果（Diff / 命令输出） -->
      <div v-else-if="b.type === 'tool'" class="flow-tool">
        <div class="flow-tool-head" @click="b.expanded = !b.expanded">
          <span class="flow-badge" :style="{ background: badge(b.name).bg }">{{ badge(b.name).ch }}</span>
          <span class="flow-tool-label">{{ toolLabel(b.name) }}</span>
          <span class="flow-tool-param">{{ keyParam(b) }}</span>
          <span class="flow-tool-state">
            <Icon v-if="b.status === 'running'" icon="mdi:loading" class="flow-spin" width="14" color="#8a8378" />
            <Icon v-else-if="b.status === 'ok'" icon="mdi:check" width="14" color="#12b76a" />
            <Icon v-else icon="mdi:close" width="14" color="#d94834" />
          </span>
          <span class="flow-chevron" :class="{ open: b.expanded }">›</span>
        </div>
        <div v-if="b.expanded" class="flow-tool-body">
          <DiffViewer
            v-if="b.name === 'edit_file'"
            :old-content="b.args.old_string || ''"
            :new-content="b.args.new_string || ''"
            :path="b.args.path || ''"
            :start-line="editStartLine(b)"
          />
          <DiffViewer
            v-else-if="b.name === 'write_file'"
            old-content=""
            :new-content="b.args.content || ''"
            :path="b.args.path || ''"
          />
          <pre v-else class="flow-output">{{ toolBodyText(b) }}</pre>
          <button v-if="b.name !== 'edit_file' && b.name !== 'write_file'" class="tool-btn flow-output-copy" @click="copyText(toolBodyText(b))" title="复制结果">
            <Icon icon="mdi:content-copy" width="14" />
          </button>
        </div>
      </div>
    </template>

    <!-- 状态行 -->
    <div class="flow-status" :class="flow.status">
      <template v-if="flow.status === 'running'">
        <Icon icon="mdi:loading" class="flow-spin" width="13" />
        执行中 · {{ elapsedText }}
      </template>
      <template v-else>
        {{ statusText }} · {{ elapsedText }} · {{ totalTokens }} tokens
      </template>
    </div>
  </div>
</template>

<script setup>
import { reactive, ref, computed, onMounted, onUnmounted } from 'vue'
import { Icon } from '@iconify/vue'
import DiffViewer from './DiffViewer.vue'
import { renderMarkdown } from './markdownRenderer.js'

const props = defineProps({
  flow: { type: Object, required: true }
})

// ==================== 状态行计时 ====================
const nowTick = ref(Date.now())
let timer = null
onMounted(() => { timer = setInterval(() => { nowTick.value = Date.now() }, 1000) })
onUnmounted(() => clearInterval(timer))

const elapsedText = computed(() => {
  const end = props.flow.endTime || nowTick.value
  const sec = Math.max(0, Math.floor((end - props.flow.startTime) / 1000))
  return `${Math.floor(sec / 60)}m ${sec % 60}s`
})
const totalTokens = computed(() => (props.flow.inputTokens || 0) + (props.flow.outputTokens || 0))
const statusText = computed(() => ({
  completed: '✓ 完成', failed: '✗ 失败', stopped: '■ 已停止'
}[props.flow.status] || props.flow.status))

// ==================== 思考块折叠 ====================
const thinkOpen = reactive({})
function toggleThink(i) { thinkOpen[i] = !thinkOpen[i] }

// ==================== 工具卡片 ====================
const TOOL_LABELS = {
  read_file: '读取文件',
  write_file: '写入文件',
  edit_file: '编辑文件',
  execute_command: '执行命令',
  search_codebase: '搜索代码库',
  codegraph_query: '调用链分析',
  search_memory: '检索记忆',
  dispatch_agent: '雨燕子代理',
  web_search: '联网搜索'
}
function toolLabel(name) {
  if (TOOL_LABELS[name]) return TOOL_LABELS[name]
  if (name.startsWith('mcp__')) return name.split('__').slice(1).join(' · ')
  return name
}

function badge(name) {
  if (name === 'read_file') return { ch: 'R', bg: '#5b8def' }
  if (name === 'write_file' || name === 'edit_file') return { ch: 'W', bg: '#c96442' }
  if (name === 'execute_command') return { ch: '>', bg: '#8a8378' }
  if (name === 'dispatch_agent') return { ch: '◆', bg: '#8b5cf6' }
  if (name.startsWith('mcp__')) return { ch: 'M', bg: '#0d9488' }
  return { ch: '·', bg: '#a3a3a3' }
}

function keyParam(b) {
  const a = b.args || {}
  const v = a.path || a.command || a.task || a.query || Object.values(a)[0] || ''
  const s = String(v)
  return s.length > 60 ? s.slice(0, 60) + '…' : s
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

// 复制文本：优先 clipboard API，失败兜底 execCommand（同 ChatWidget.copyText 行为）
async function copyText(text) {
  try {
    await navigator.clipboard.writeText(text || '')
  } catch (err) {
    const ta = document.createElement('textarea')
    ta.value = text || ''
    ta.style.position = 'fixed'
    ta.style.opacity = '0'
    document.body.appendChild(ta)
    ta.select()
    try { document.execCommand('copy') } catch (_) {}
    document.body.removeChild(ta)
  }
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
.flow-thinking-label {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: #9c958a;
  cursor: pointer;
  user-select: none;
}
.flow-thinking-text {
  margin-top: 4px;
  padding: 8px 10px;
  font-size: 12.5px;
  line-height: 1.65;
  color: #8a8378;
  background: #faf9f5;
  border-left: 2px solid #e4dfd4;
  border-radius: 0 6px 6px 0;
  white-space: pre-wrap;
  word-break: break-word;
}

/* ---------- 意图 ---------- */
.flow-intent-wrap { margin: 6px 0; }
.flow-intent {
  font-size: 14px;
  line-height: 1.7;
  color: #2d2a26;
  word-break: break-word;
}
.flow-intent-tools {
  display: flex;
  justify-content: flex-end;
  margin-top: 2px;
}

/* ---------- 操作 + 结果卡片 ---------- */
.flow-tool { position: relative; }
.flow-output-copy {
  position: absolute;
  top: 6px;
  right: 8px;
  background: rgba(255, 255, 255, 0.85);
  border: 1px solid #e5e5e5;
  border-radius: 6px;
  padding: 3px 5px;
  cursor: pointer;
}
.flow-output-copy:hover { background: #f0f0f0; }

/* ---------- 操作 + 结果卡片 ---------- */
.flow-tool {
  margin: 6px 0;
  border: 1px solid #e5e5e5;
  border-radius: 10px;
  background: #fff;
  overflow: hidden;
}
.flow-tool-head {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 12px;
  cursor: pointer;
  user-select: none;
}
.flow-tool-head:hover {
  background: #fafafa;
}
.flow-badge {
  flex-shrink: 0;
  width: 18px;
  height: 18px;
  border-radius: 5px;
  color: #fff;
  font-size: 11px;
  font-weight: 700;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
}
.flow-tool-label {
  flex-shrink: 0;
  font-size: 13px;
  color: #2d2a26;
  font-weight: 500;
}
.flow-tool-param {
  flex: 1;
  min-width: 0;
  font-size: 12px;
  color: #8a8378;
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.flow-tool-state {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
}
.flow-chevron {
  flex-shrink: 0;
  color: #a3a3a3;
  font-size: 14px;
  transition: transform 0.15s;
  display: inline-block;
}
.flow-chevron.open {
  transform: rotate(90deg);
}
.flow-tool-body {
  border-top: 1px solid #f0efe9;
  padding: 8px 12px;
  background: #fcfcfa;
}
.flow-output {
  margin: 0;
  max-height: 320px;
  overflow: auto;
  font-size: 12px;
  line-height: 1.6;
  color: #4b4741;
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
  white-space: pre-wrap;
  word-break: break-all;
}

/* ---------- 状态行 ---------- */
.flow-status {
  display: flex;
  align-items: center;
  gap: 5px;
  margin-top: 8px;
  font-size: 12px;
  color: #8a8378;
}
.flow-status.completed { color: #12b76a; }
.flow-status.failed { color: #d94834; }

.flow-spin {
  animation: flow-rotate 0.9s linear infinite;
}
@keyframes flow-rotate {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
</style>
