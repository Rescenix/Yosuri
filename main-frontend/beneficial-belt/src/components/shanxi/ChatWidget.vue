<template>
  <div>
    <div class="chat-toggle-button" v-if="!isOpen" @click="toggleChat">
      <Icon icon="mdi:chat" width="28" color="white" />
    </div>

    <div v-if="isOpen && isExpanded" class="chat-overlay" @click="toggleExpand"></div>

    <div class="chat-window" :class="{ expanded: isExpanded }" :style="{ display: isOpen ? 'flex' : 'none' }">
      <div class="chat-header">
        <div class="header-left">
          <div class="header-user-info">
            <span class="header-name">杉汐</span>
            <div class="status-wrapper">
              <span class="status-dot" :style="{ background: statusDotColor }"></span>
              <span class="status-text">{{ currentStatus }}</span>
            </div>
          </div>
        </div>
        <div class="header-actions">
          <button class="ds-btn" @click="toggleExpand" :title="isExpanded ? '还原' : '放大'">
            <Icon :icon="isExpanded ? 'mdi:arrow-collapse' : 'mdi:arrow-expand'" width="16" color="#666" />
          </button>
          <button class="ds-btn" @click="toggleChat">
            <Icon icon="heroicons:x-mark-20-solid" width="16" color="#666" />
          </button>
        </div>
      </div>

      <div class="chat-messages" ref="messagesContainer">
        <div v-if="messages.length === 0 && !welcomeLoading" class="message bot">{{ welcomeMessage }}</div>
        <div v-if="messages.length === 0 && welcomeLoading" class="message bot" style="opacity:0.6">杉汐正在想起你...</div>

        <template v-for="item in groupedMessages">
          <div v-if="item.type === 'time'" :key="`time-${item.timestamp}`" class="chat-time">
            {{ formatChatTime(item.timestamp) }}
          </div>
          <div v-else-if="item.type === 'message'" :key="item.id" class="message-row" :class="item.sender">
            <div v-if="item.type === 'image'" class="image-card">
              <img :src="item.image" style="max-width: 240px; border-radius: 12px;" />
            </div>
            <div v-else class="message" :class="item.sender">
              <div v-if="item.reasoning" class="reasoning-stream">
                <div class="reasoning-label">
                  <Icon icon="mdi:brain" width="14" color="#6b7280" />
                  思考中...
                </div>
                <div class="reasoning-text">{{ item.reasoning }}</div>
              </div>


              <div v-if="item.toolCallName" class="tool-call-indicator">
    <Icon icon="mdi:cog-sync" width="14" color="#6b7280" />
    <span>正在调用工具：{{ item.toolCallName }}</span>
</div>
              <!-- 始终渲染 Markdown，流式过程中也实时解析 -->
              <div v-if="item.sender === 'bot'" class="markdown-body" v-html="renderMarkdown(item.content, true)"></div>
              <div v-else>{{ item.content }}</div>
              <button v-if="isLoggedIn && item.sender === 'bot'" class="ds-btn ds-btn-msg" @click="playVoice(item.content)" title="播放语音">
                <Icon icon="mdi:microphone" width="14" color="#666" />
              </button>
            </div>
          </div>
        </template>
      </div>

      <div class="chat-input-area">
        <input type="file" accept="image/*" ref="imageInput" style="display:none" v-if="isLoggedIn"
          @change="handleImageUpload" />
        <button v-if="isLoggedIn" class="ds-btn" @click="imageInput.click()" title="上传图片">
          <Icon icon="heroicons:photo-20-solid" width="18" color="#666" />
        </button>
        <textarea 
          ref="chatInputRef"
          class="chat-input" 
          v-model="userInput" 
          placeholder="输入你的问题..."
          @keypress.enter="sendMessage"
          @input="adjustInputHeight"
          rows="3"
        ></textarea>
        <button class="ds-btn ds-btn-send" @click="sendMessage" :disabled="!userInput.trim()">
          <Icon icon="heroicons:paper-airplane-20-solid" width="18" color="#fff" />
        </button>
      </div>

      <details class="debug-panel" v-if="isLoggedIn">
        <summary>调试参数</summary>
        <div class="debug-controls">
          <label>T: <input type="range" min="0" max="2" step="0.1" v-model.number="debugTemp" @change="updateParams" />
            <span>{{ debugTemp }}</span>
          </label>
          <label>TopP: <input type="range" min="0" max="1" step="0.05" v-model.number="debugTopP"
              @change="updateParams" />
            <span>{{ debugTopP }}</span>
          </label>
          <label>MaxTokens: <input type="number" v-model.number="debugMaxTokens" min="100" max="8192" step="100"
              @change="updateParams" /></label>
          <label>深度思考:
            <select v-model="debugReasoning" @change="updateParams">
              <option value="">关闭</option>
              <option value="high">开启（高）</option>
              <option value="max">开启（最强）</option>
            </select>
          </label>
          <div class="debug-stats">
            <span>Token: {{ lastTokenUsage || '--' }}</span>
            <span>延迟: {{ lastLatency || '--' }}ms</span>
            <span>余额: {{ balance || '--' }}</span>
            <button class="debug-refresh-btn" @click="fetchBalance">刷新余额</button>
          </div>
        </div>
      </details>
    </div>
  </div>
</template>

<script setup>
import { ref, watch, nextTick, computed, onMounted } from 'vue'
import { Icon } from '@iconify/vue'
import { useEmotion } from './composables/useEmotion.js'
import { useMemory } from './composables/useMemory.js'
import { useWelcome } from './composables/useWelcome.js'
import { useChatLogic } from './composables/useChatLogic.js'
import { useImageUpload } from './composables/useImageUpload.js'
import { useVoicePlay } from './composables/useVoicePlay.js'
import { useStatusPolling } from './composables/useStatusPolling.js'

import { marked } from 'marked'
import katex from 'katex'
import markedKatex from 'marked-katex-extension'
import DOMPurify from 'dompurify'
import 'katex/dist/katex.min.css'

marked.use(markedKatex({ throwOnError: false }))

function renderMarkdown(text, skipSanitize = false) {
  if (!text) return ''
  const raw = marked.parse(text)
  if (skipSanitize) return raw
  return DOMPurify.sanitize(raw)
}

let msgId = 0

const isOpen = ref(false)
const isExpanded = ref(false)
const toggleExpand = () => { isExpanded.value = !isExpanded.value }
const toggleChat = () => {
  isOpen.value = !isOpen.value
  if (isOpen.value) {
    nextTick(() => forceScrollToBottom())
    setTimeout(() => forceScrollToBottom(), 200)
  }
}
const userInput = ref('')
const messages = ref([])

const sessionId = ref(`global_chat_session`)
const isLoggedIn = ref(!!localStorage.getItem('token'))

const debugTemp = ref(localStorage.getItem('debugTemp') ? parseFloat(localStorage.getItem('debugTemp')) : 0.7)
const debugTopP = ref(localStorage.getItem('debugTopP') ? parseFloat(localStorage.getItem('debugTopP')) : 0.9)
const debugReasoning = ref(localStorage.getItem('debugReasoning') || '')
const lastTokenUsage = ref('')
const lastLatency = ref('')
const debugMaxTokens = ref(localStorage.getItem('debugMaxTokens') ? parseInt(localStorage.getItem('debugMaxTokens')) : 2000)
const balance = ref('')

async function fetchBalance() {
  try {
    const res = await fetch('/api/balance')
    if (res.ok) {
      const data = await res.json()
      if (data.is_available && data.balance_infos?.length > 0) {
        const info = data.balance_infos[0]
        balance.value = `${info.total_balance} ${info.currency}`
      } else {
        balance.value = '不可用'
      }
    }
  } catch { }
}

function updateParams() {
  localStorage.setItem('debugTemp', debugTemp.value)
  localStorage.setItem('debugTopP', debugTopP.value)
  localStorage.setItem('debugMaxTokens', debugMaxTokens.value)
  localStorage.setItem('debugReasoning', debugReasoning.value)
}

const { updateEmotion } = useEmotion()
const { saveMemory } = useMemory()
const { welcomeMessage, welcomeLoading } = useWelcome()
const { currentStatus } = useStatusPolling()

const messagesContainer = ref(null)
const chatInputRef = ref(null)

// 用户是否手动上滑
const userScrolledUp = ref(false)

// 强制滚动到底部（发送消息时、打开窗口时）
function forceScrollToBottom() {
  if (!messagesContainer.value) return
  messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
  userScrolledUp.value = false // 重置标记
}

// 智能滚动：仅当用户处于底部附近且未手动上滑
function smartScrollToBottom() {
  if (!messagesContainer.value) return
  if (userScrolledUp.value) return  // 用户上滑后，不再滚动
  messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
}

// 智能滚动 + 强制刷新视图（流式打字时用）
function smartScrollAndRefresh() {
  smartScrollToBottom()
  messages.value = [...messages.value]
}

// 输入框自适应高度
function adjustInputHeight() {
  if (!chatInputRef.value) return
  chatInputRef.value.style.height = 'auto'
  chatInputRef.value.style.height = Math.min(chatInputRef.value.scrollHeight, 200) + 'px'
}

const { sendMessage } = useChatLogic({
  messages, userInput, sessionId,
  updateEmotion, saveMemory, lastTokenUsage, lastLatency,
  onNewMessage: () => {
    forceScrollToBottom()
    // 重置输入框高度
    nextTick(() => {
      if (chatInputRef.value) {
        chatInputRef.value.style.height = 'auto'
      }
    })
  },
  onStreamUpdate: smartScrollAndRefresh
})

const { imageInput, handleImageUpload } = useImageUpload({
  messages,
  sessionId,
  saveMemory
})

const { playVoice } = useVoicePlay()

const statusDotColor = computed(() => {
  const status = currentStatus.value
  if (!status) return '#98a2b3'
  if (status.includes('活跃') || status.includes('在线') || status.includes('帮忙') || status.includes('聊聊天')) return '#12b76a'
  if (status.includes('发呆') || status.includes('思绪') || status.includes('休眠')) return '#f59e0b'
  if (status.includes('忙碌') || status.includes('整理') || status.includes('写文章')) return '#ef4444'
  return '#98a2b3'
})

function cleanContent(content) {
  if (!content) return ''
  return content.replace(/\[(action|emotion):[^\]]*\]/g, '')
}

let lastScrollTop = 0
onMounted(async () => {
  fetchBalance()
  await loadAllHistory()
  if (messagesContainer.value) {
    messagesContainer.value.addEventListener('scroll', () => {
      const el = messagesContainer.value
      const currentScrollTop = el.scrollTop
      const maxScroll = el.scrollHeight - el.clientHeight
      const isAtBottom = Math.abs(currentScrollTop - maxScroll) < 10
      
      if (isAtBottom) {
        // 滑到底部 → 恢复自动滚动
        userScrolledUp.value = false
      } else if (currentScrollTop < lastScrollTop) {
        // 向上滚动 → 阻止自动滚动
        userScrolledUp.value = true
      }
      lastScrollTop = currentScrollTop
    })
  }
})
async function loadAllHistory() {
  try {
    const res = await fetch('/api/all-messages')
    if (res.ok) {
      const history = await res.json()
      messages.value = history.map((item, idx) => ({
        id: idx,
        content: cleanContent(item.content),
        sender: item.role === 'assistant' ? 'bot' : item.role,
        timestamp: item.timestamp,
        isStreaming: false,
        reasoning: ''
      }))
      await nextTick()
      forceScrollToBottom()
    }
  } catch (e) {
    console.error('加载历史失败', e)
  }
}

function shouldShowTime(prevMsg, currentMsg) {
  if (!prevMsg) return true
  const prevTime = new Date(prevMsg.timestamp)
  const currTime = new Date(currentMsg.timestamp)
  if (prevTime.toDateString() !== currTime.toDateString()) return true
  const diffMinutes = (currTime - prevTime) / (1000 * 60)
  return diffMinutes > 5
}

const groupedMessages = computed(() => {
  const result = []
  for (let i = 0; i < messages.value.length; i++) {
    const msg = messages.value[i]
    const prevMsg = i > 0 ? messages.value[i-1] : null
    if (shouldShowTime(prevMsg, msg)) {
      result.push({ type: 'time', timestamp: msg.timestamp, id: `time-${i}` })
    }
    result.push({ type: 'message', ...msg })
  }
  return result
})

function formatChatTime(timestamp) {
  if (!timestamp) return ''
  const date = new Date(timestamp)
  const now = new Date()
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  const yesterday = new Date(today.getTime() - 86400000)
  const msgDate = new Date(date.getFullYear(), date.getMonth(), date.getDate())

  if (msgDate.getTime() === today.getTime()) {
    return `${date.getHours().toString().padStart(2,'0')}:${date.getMinutes().toString().padStart(2,'0')}`
  } else if (msgDate.getTime() === yesterday.getTime()) {
    return `昨天 ${date.getHours().toString().padStart(2,'0')}:${date.getMinutes().toString().padStart(2,'0')}`
  } else {
    return `${date.getMonth()+1}/${date.getDate()} ${date.getHours().toString().padStart(2,'0')}:${date.getMinutes().toString().padStart(2,'0')}`
  }
}
</script>

<script>
export default {}
</script>

<style scoped>
@import '../../styles/shanxi/chat-widget.css';
.chat-time {
  text-align: center;
  font-size: 10px;
  color: #aaa;
  margin: 8px 0;
  background: none;
}

.reasoning-details { margin-top: 8px; font-size: 12px; color: #4b5563; background: #f9fafb; border-radius: 8px; padding: 6px 10px; border-left: 2px solid #3b82f6; }
.reasoning-details summary { cursor: pointer; font-weight: 500; user-select: none; display: flex; align-items: center; gap: 4px; }
.reasoning-text { margin-top: 6px; white-space: pre-wrap; word-break: break-word; }

.reasoning-stream { margin-bottom: 10px; padding: 8px 12px; background: #f9fafb; border-left: 3px solid #3b82f6; border-radius: 6px; font-size: 13px; color: #4b5563; }
.reasoning-label { display: flex; align-items: center; gap: 4px; margin-bottom: 4px; font-weight: 500; font-size: 11px; color: #6b7280; text-transform: uppercase; letter-spacing: 0.5px; }

.markdown-body { word-break: break-word; line-height: 1.6; }
.markdown-body p { margin-bottom: 8px; }
.markdown-body table {
  border-collapse: separate;        /* 用 separate 让外层边框生效 */
  border-spacing: 0;
  width: 100%;
  margin: 10px 0;
  font-size: 13px;
  border: 2px solid #d0d5dd;        /* 表格整体边框 */
  border-radius: 8px;               /* 圆角 */
  overflow: hidden;                 /* 圆角剪切 */
  word-break: break-word;
}

.markdown-body th,
.markdown-body td {
  border: 1px solid #e5e7eb;        /* 单元格细边框 */
  padding: 8px 12px;
  text-align: left;
  vertical-align: top;              /* 顶部对齐，避免挤压 */
}

.markdown-body th {
  background: #f9fafb;
  font-weight: 600;
  font-size: 13px;
  color: #1f2937;
}

.markdown-body td {
  background: #fff;
}

/* 隔行变色（可选，增强可读性） */
.markdown-body tr:nth-child(even) td {
  background: #f9fafb;
}
.markdown-body h1, .markdown-body h2, .markdown-body h3 { margin: 12px 0 6px; font-weight: 600; color: #1d2939; }
.markdown-body code { background: #f0f0f0; padding: 2px 4px; border-radius: 3px; font-family: monospace; font-size: 0.9em; }
.markdown-body pre { background: #f5f5f5; padding: 10px; border-radius: 6px; overflow-x: auto; font-size: 13px; }

/* 展开模式居中 */
.chat-window.expanded .message-row { max-width: 60%; margin-left: auto; margin-right: auto; }
.chat-window.expanded .message-row.user { justify-content: flex-end; }

.chat-window.expanded .chat-input-area,
.chat-window.expanded .debug-panel { max-width: 60%; margin-left: auto; margin-right: auto; width: 100%; }
.chat-window.expanded .chat-input-area { padding-left: 0; padding-right: 0; }

/* 自适应 textarea 替换 input */
.chat-input {
  min-height: 60px;    /* 三行高度 */
  max-height: 200px;
  resize: none;
  overflow-y: auto;
}
.tool-call-indicator {
  display: flex;
  align-items: center;
  gap: 4px;
  margin: 6px 0;
  font-size: 12px;
  color: #6b7280;
  background: #f3f4f6;
  padding: 4px 8px;
  border-radius: 4px;
}
</style>

<style>
/* 全局 Markdown 表格样式（必须非 scoped，因为 v-html 渲染的内容不受 scoped 控制） */
.markdown-body table {
  border-collapse: separate;
  border-spacing: 0;
  width: 100%;
  margin: 10px 0;
  font-size: 13px;
  border: 2px solid #d0d5dd;
  border-radius: 8px;
  overflow: hidden;
  word-break: break-word;
}

.markdown-body th,
.markdown-body td {
  border: 1px solid #e5e7eb;
  padding: 8px 12px;
  text-align: left;
  vertical-align: top;
}

.markdown-body th {
  background: #f9fafb;
  font-weight: 600;
  font-size: 13px;
  color: #1f2937;
}

.markdown-body td {
  background: #fff;
}

.markdown-body tr:nth-child(even) td {
  background: #f9fafb;
}
/* ===== 全局字体 ===== */
.chat-window,
.chat-window * {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, 'Noto Sans', sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

/* ===== 消息间距 ===== */
.chat-messages .message-row {
  margin-bottom: 12px;
}

/* ===== 用户气泡 ===== */
.message.user {
  background: linear-gradient(135deg, #2563eb, #7c3aed) !important;
  color: #fff !important;
  box-shadow: 0 2px 8px rgba(37, 99, 235, 0.25) !important;
  border: none !important;
  border-radius: 18px 18px 4px 18px !important;
  padding: 10px 16px !important;
  font-size: 15px !important;
  line-height: 1.5 !important;
}

/* ===== 输入框 ===== */
.chat-input {
  border-radius: 12px !important;
  border: 1px solid #d1d5db !important;
  padding: 10px 14px !important;
  font-size: 15px !important;
  transition: border-color 0.15s ease, box-shadow 0.15s ease;
}
.chat-input:focus {
  outline: none;
  border-color: #2563eb !important;
  box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.1) !important;
}

/* 输入区域整体微调 */
.chat-input-area {
  padding: 12px 16px !important;
  gap: 10px !important;
}

/* ===== 工具调用指示器 ===== */
.tool-call-indicator {
  display: flex;
  align-items: center;
  gap: 4px;
  margin: 6px 0;
  padding: 4px 8px;
  font-size: 12px;
  color: #2563eb;
  background: #eff6ff;
  border-radius: 4px;
}

/* ===== 思考过程微调 ===== */
.reasoning-stream {
  background: #f9fafb;
  border-left: 3px solid #2563eb;
  padding: 8px 12px;
  border-radius: 6px;
  margin-bottom: 10px;
}
</style>