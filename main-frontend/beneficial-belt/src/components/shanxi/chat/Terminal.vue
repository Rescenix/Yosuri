<template>
  <div
    class="terminal-panel"
    :class="{ collapsed: !open, embedded }"
    :style="embedded ? {} : { height: open ? height + 'px' : '28px' }"
  >
    <div v-if="!embedded" class="terminal-titlebar" @click="$emit('update:open', !open)">
      <span class="terminal-title">TERMINAL · powershell</span>
      <div class="terminal-titlebar-actions" @click.stop>
        <Icon
          icon="mdi:stop-circle-outline"
          width="14"
          class="term-action-icon"
          title="Ctrl+C 中断当前命令"
          @click="sendInterrupt"
        />
        <Icon icon="mdi:chevron-down" width="16" class="collapse-chevron" :class="{ rotated: !open }" @click="$emit('update:open', !open)" />
      </div>
    </div>

    <div class="terminal-body" ref="bodyRef" v-show="open || embedded">
      <pre class="term-output" v-html="outputHtml"></pre>
      <div class="term-input-row">
        <span class="term-prompt">▷</span>
        <input
          ref="inputRef"
          v-model="pendingInput"
          class="term-input"
          type="text"
          spellcheck="false"
          autocomplete="off"
          placeholder="输入命令，回车执行…"
          @keydown.enter="sendCommand"
          @keydown.ctrl.c.exact="sendInterrupt"
        />
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { Icon } from '@iconify/vue'
import Convert from 'ansi-to-html'

const props = defineProps({
  open: { type: Boolean, default: true },
  height: { type: Number, default: 180 },
  embedded: { type: Boolean, default: false },
  snippetVisible: { type: Boolean, default: false },
  snippetInsertCmd: { type: String, default: '' },
  terminalId: { type: String, default: '' }
})
defineEmits(['update:open', 'toggle-snippet'])

const terminalId = props.terminalId || ('term_' + Date.now().toString(36) + Math.random().toString(36).slice(2, 8))

const convert = new Convert({ colors: { 0: '#000', 1: '#e8e8e8', 2: '#5cb85c', 3: '#f0ad4e', 4: '#5bc0de', 5: '#d9534f', 6: '#5bc0de', 7: '#e8e8e8', 8: '#555', 9: '#ff6b6b', 10: '#51cf66', 11: '#ffd43b', 12: '#339af0', 13: '#cc5de8', 14: '#20c997', 15: '#fff' } })

const output = ref('')
const URL_RE = /(https?:\/\/[^\s<>"')\]]+)/g
const outputHtml = computed(() => {
  // 先把 URL 替换成占位符，避免 ANSI 转换把 URL 拆进不同 span
  const urls = []
  let raw = output.value.replace(URL_RE, (m) => {
    urls.push(m)
    return '\x00URL_' + (urls.length - 1) + '\x00'
  })
  let html = convert.toHtml(raw)
  // 把占位符还原为可点击链接
  html = html.replace(/\x00URL_(\d+)\x00/g, (_, i) => {
    const url = urls[parseInt(i)]
    return '<a href="' + url + '" target="_blank" rel="noopener" style="color:#5bc0de;text-decoration:none;cursor:pointer">' + url + '</a>'
  })
  return html
})
const pendingInput = ref('')
const bodyRef = ref(null)
const inputRef = ref(null)
let eventSource = null

function scrollToBottom() {
  nextTick(() => {
    if (bodyRef.value) bodyRef.value.scrollTop = bodyRef.value.scrollHeight
  })
}

function appendChunk(text) {
  output.value += text
  // 超过一定长度截掉前面的，避免纯前端 DOM 无限增长卡死
  if (output.value.length > 200000) {
    output.value = output.value.slice(-150000)
  }
  scrollToBottom()
}

function connect() {
  eventSource = new EventSource(`/api/terminal/stream?id=${encodeURIComponent(terminalId)}`)
  eventSource.addEventListener('chunk', (e) => {
    try {
      appendChunk(JSON.parse(e.data))
    } catch (err) {}
  })
  eventSource.addEventListener('exit', () => {
    appendChunk('\n[进程已退出]\n')
  })
  eventSource.onerror = () => {
    // EventSource 自带重连，这里不用手动处理；后端会话本身是常驻的，重连后接着收
  }
}

async function sendToTerminal(data) {
  try {
    await fetch('/api/terminal/input', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id: terminalId, data })
    })
  } catch (e) {}
}

function sendCommand() {
  const cmd = pendingInput.value
  pendingInput.value = ''
  sendToTerminal(cmd + '\r\n')
}

// Ctrl+C：给前台正在跑的命令（比如 npm run dev）发中断信号，0x03 是 ETX 控制字符
function sendInterrupt() {
  sendToTerminal('\x03')
}

// 片段面板插入命令：按分号拆分逐条发送，避免 PowerShell 把分号当新行时状态错乱
watch(() => props.snippetInsertCmd, (cmd) => {
  if (!cmd) return
  const parts = cmd.split(';').map(s => s.trim()).filter(Boolean)
  parts.forEach((part, i) => {
    setTimeout(() => sendToTerminal(part + '\r\n'), i * 300)
  })
})

onMounted(() => {
  connect()
  if (props.embedded) nextTick(() => inputRef.value?.focus())
})
onUnmounted(() => {
  // 只断前端连接，后端 shell 进程继续留着——下次打开面板还能接上
  if (eventSource) eventSource.close()
})
</script>

<style scoped>
.terminal-panel.embedded {
  flex: 1;
  min-width: 0;
  min-height: 0;
  height: auto;
  border-top: none;
  transition: none;
}
.terminal-panel {
  flex-shrink: 0;
  width: 100%;
  background: var(--app-surface);
  border-top: 1px solid var(--app-border);
  overflow: hidden;
  transition: height 180ms ease;
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
}

.terminal-titlebar {
  height: 28px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 12px;
  cursor: pointer;
  color: var(--app-text-faint);
  font-size: 11px;
  font-family: var(--app-font);
  letter-spacing: 0.3px;
  user-select: none;
}
.terminal-titlebar:hover { background: rgba(255,255,255,0.03); }
.terminal-titlebar-actions { display: flex; align-items: center; gap: 8px; }
.term-action-icon { cursor: pointer; opacity: 0.75; }
.term-action-icon:hover { opacity: 1; color: #c96442; }

/* 嵌入模式顶部单标签页头：白底融入卡片，底部一条细边框，标签左对齐 */
.terminal-tabbar {
  display: flex;
  align-items: center;
  gap: 6px;
  height: 34px;
  padding: 0 10px;
  background: var(--app-surface);
  border-bottom: 1px solid var(--app-border-soft);
  border-radius: 12px 12px 0 0;
  flex-shrink: 0;
}
.terminal-tab {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--app-text);
  font-family: var(--app-font);
}
.terminal-tab.active { font-weight: 500; }
.terminal-tab-icon { flex-shrink: 0; color: var(--app-text-soft); }
.terminal-tabbar-actions {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-left: auto;
}
.term-tab-interrupt { }
.term-tab-snippet { opacity: 0.6; }
.term-tab-snippet:hover { opacity: 1; color: var(--app-accent); }
.term-tab-snippet.active { opacity: 1; color: var(--app-accent); }

.collapse-chevron { transition: transform 180ms ease; cursor: pointer; }
.collapse-chevron.rotated { transform: rotate(180deg); }

.terminal-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 8px 12px 10px;
  font-family: "JetBrains Mono", ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  line-height: 1.6;
  color: var(--app-text);
  display: flex;
  flex-direction: column;
}

.term-output {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
  font: inherit;
  color: inherit;
}

.term-input-row {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 2px;
  flex-shrink: 0;
}
.term-prompt { color: #c96442; flex-shrink: 0; }
.term-input {
  flex: 1;
  min-width: 0;
  background: transparent;
  border: none;
  outline: none;
  color: var(--app-text);
  font: inherit;
  caret-color: #c96442;
}
.term-input::placeholder { color: var(--app-text-faint); }
</style>

<style>
/* v-html 内容不受 scoped 样式管辖，hover 必须写在全局 */
.term-output a:hover { text-decoration: underline !important; }
</style>
