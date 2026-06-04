<template>
  <div>
    <div class="chat-toggle-button" v-if="!isOpen" @click="toggleChat">
      <Icon icon="mdi:chat" width="28" color="white" />
    </div>

    <!-- 聊天窗口 -->
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

      <!-- 消息区域 -->
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
              {{ item.content }}
              <button v-if="isLoggedIn && item.sender === 'bot'" class="ds-btn ds-btn-msg" @click="playVoice(item.content)" title="播放语音">
                <Icon icon="mdi:microphone" width="14" color="#666" />
              </button>
            </div>
          </div>
        </template>
      </div>

      <!-- 输入区域 -->
      <div class="chat-input-area">
        <input type="file" accept="image/*" ref="imageInput" style="display:none" v-if="isLoggedIn"
          @change="handleImageUpload" />
        <button v-if="isLoggedIn" class="ds-btn" @click="imageInput.click()" title="上传图片">
          <Icon icon="heroicons:photo-20-solid" width="18" color="#666" />
        </button>
        <input type="text" class="chat-input" v-model="userInput" placeholder="输入你的问题..."
          @keypress.enter="sendMessage" />
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
          <label>MaxTokens: <input type="number" v-model.number="debugMaxTokens" min="100" max="4096" step="100"
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

let msgId = 0

// 基础状态
const isOpen = ref(false)
const isExpanded = ref(false)
const toggleExpand = () => { isExpanded.value = !isExpanded.value }
const toggleChat = () => {
  isOpen.value = !isOpen.value
  if (isOpen.value) {
    // 打开窗口后等待 DOM 更新再滚动
    nextTick(() => scrollToBottom())
    // 额外延迟确保动画完成
    setTimeout(() => scrollToBottom(), 200)
  }
}
const userInput = ref('')
const messages = ref([])

// 固定会话ID（仅用于发送新消息）
const sessionId = ref(`global_chat_session`)

// 登录状态
const isLoggedIn = ref(!!localStorage.getItem('token'))

// 调试参数
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

// 外部模块
const { updateEmotion } = useEmotion()
const { saveMemory } = useMemory()
const { welcomeMessage, welcomeLoading } = useWelcome()
const { currentStatus } = useStatusPolling()

// 聊天核心逻辑
const { sendMessage } = useChatLogic({
  messages,
  userInput,
  sessionId,
  updateEmotion,
  saveMemory,
  lastTokenUsage,
  lastLatency
})

// 图片上传
const { imageInput, handleImageUpload } = useImageUpload({
  messages,
  sessionId,
  saveMemory
})

// 语音播放
const { playVoice } = useVoicePlay()

const statusDotColor = computed(() => {
  const status = currentStatus.value
  if (!status) return '#98a2b3'
  if (status.includes('活跃') || status.includes('在线') || status.includes('帮忙') || status.includes('聊聊天')) return '#12b76a'
  if (status.includes('发呆') || status.includes('思绪') || status.includes('休眠')) return '#f59e0b'
  if (status.includes('忙碌') || status.includes('整理') || status.includes('写文章')) return '#ef4444'
  return '#98a2b3'
})

const messagesContainer = ref(null)

// 清洗消息内容
function cleanContent(content) {
  if (!content) return ''
  return content.replace(/\[(action|emotion):[^\]]*\]/g, '')
}

// 强制滚动到底部（三重保险）
function scrollToBottom() {
  if (!messagesContainer.value) return
  messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
  requestAnimationFrame(() => {
    if (messagesContainer.value) {
      messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
    }
  })
  setTimeout(() => {
    if (messagesContainer.value) {
      messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight
    }
  }, 100)
}

// 加载所有历史消息
async function loadAllHistory() {
  try {
    const res = await fetch('/api/all-messages')
    if (res.ok) {
      const history = await res.json()
      messages.value = history.map((item, idx) => ({
        id: idx,
        content: cleanContent(item.content),
        sender: item.role,
        timestamp: item.timestamp
      }))
      await nextTick()
      scrollToBottom()
    }
  } catch (e) {
    console.error('加载历史失败', e)
  }
}

// 时间分组逻辑
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

// 监听消息变化（包括新增消息）并滚动到底部
watch(messages, () => {
  scrollToBottom()
}, { deep: true })

// 监听新消息，清洗附加指令（可选，因为发送时已清洗，但保险）
watch(messages, (newMsgs, oldMsgs) => {
  if (newMsgs.length > (oldMsgs?.length || 0)) {
    const last = newMsgs[newMsgs.length - 1]
    if (last && last.content) {
      const cleaned = cleanContent(last.content)
      if (cleaned !== last.content) {
        last.content = cleaned
      }
    }
  }
}, { deep: false })

onMounted(async () => {
  fetchBalance()
  await loadAllHistory()
})
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
</style>