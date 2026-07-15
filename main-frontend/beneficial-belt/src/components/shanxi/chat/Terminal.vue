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
      <pre class="term-output">{{ output }}</pre>
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
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { Icon } from '@iconify/vue'

const props = defineProps({
  open: { type: Boolean, default: true },
  height: { type: Number, default: 180 },
  embedded: { type: Boolean, default: false }  // true = 右侧工具面板全高模式，无折叠标题栏
})
defineEmits(['update:open'])

// 真实终端：后端常驻一个 powershell 进程，SSE 推输出，POST 写 stdin。
// terminalId 存 localStorage——刷新页面/重开面板都接回同一个后端会话，
// cd 到哪、设了什么环境变量都还在，这才是"终端"而不是一次性命令框。
const TERMINAL_ID_KEY = 'aether_terminal_id'
function getOrCreateTerminalId() {
  let id = localStorage.getItem(TERMINAL_ID_KEY)
  if (!id) {
    id = 'term_' + Date.now().toString(36) + Math.random().toString(36).slice(2, 8)
    localStorage.setItem(TERMINAL_ID_KEY, id)
  }
  return id
}
const terminalId = getOrCreateTerminalId()

const output = ref('')
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
  min-height: 0;
  height: auto;
  border-top: none;
  transition: none;
}
.terminal-panel {
  flex-shrink: 0;
  width: 100%;
  background: #ffffff;
  border-top: 1px solid #e5e5e5;
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
  color: #9a958a;
  font-size: 11px;
  font-family: "JetBrains Mono", ui-monospace, monospace;
  letter-spacing: 0.3px;
  user-select: none;
}
.terminal-titlebar:hover { background: rgba(255,255,255,0.03); }
.terminal-titlebar-actions { display: flex; align-items: center; gap: 8px; }
.term-action-icon { cursor: pointer; opacity: 0.75; }
.term-action-icon:hover { opacity: 1; color: #c96442; }

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
  color: #334155;
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
  color: #334155;
  font: inherit;
  caret-color: #c96442;
}
.term-input::placeholder { color: #94a3b8; }
</style>
