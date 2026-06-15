<template>
  <div>
    <div class="chat-toggle-button" v-if="!isOpen" @click="toggleChat">
      <Icon icon="mdi:chat" width="28" color="white" />
    </div>

    <div v-if="isOpen && isExpanded" class="chat-overlay" @click="toggleExpand"></div>

    <div class="chat-window" :class="{ expanded: isExpanded, mobile: isMobile }" :style="{ display: isOpen ? 'flex' : 'none' }">
      <div class="chat-header">
        <div class="header-left">
          <span class="header-name">杉汐</span>
          <span class="status-dot" :style="{ background: statusDotColor }"></span>
         <span class="status-text" :style="{ color: statusTextColor }">{{ currentStatus }}</span>
        </div>
        <div class="header-actions">
          <button class="header-btn" @click="toggleChat">
            <Icon icon="heroicons:x-mark-20-solid" width="16" color="#888" />
          </button>
        </div>
      </div>

      <div class="chat-messages" ref="messagesContainer">
        <div v-if="messages.length === 0 && !welcomeLoading" class="message-row bot">
          <div class="message bot">{{ welcomeMessage }}</div>
        </div>
        <div v-if="messages.length === 0 && welcomeLoading" class="message-row bot">
          <div class="message bot" style="opacity:0.6">杉汐正在想起你...</div>
        </div>
        <template v-for="item in groupedMessages">
          <div v-if="item.type === 'time'" :key="`time-${item.timestamp}`" class="chat-time">
            {{ formatChatTime(item.timestamp) }}
          </div>
          <div v-else-if="item.type === 'message'" :key="item.id" class="message-row" :class="item.sender">
            <div v-if="item.type === 'image'" class="image-card">
              <img :src="item.image" style="max-width: 240px; border-radius: 12px;" />
            </div>
            <div v-else class="message" :class="item.sender">
              <div v-if="item.sender === 'bot' && item.recalling" class="recalling-hint">
                <Icon icon="mdi:memory" width="14" color="#6b7280" />
                <span>杉汐正在回忆与你的过去...</span>
              </div>

              <div v-if="item.reasoning" class="reasoning-stream">
                <div class="reasoning-label">
                  <Icon icon="la:atom" width="14" color="#6b7280" />
                  思考中...
                </div>
             <div class="reasoning-text" v-html="renderMarkdown(item.reasoning, true)"></div>
              </div>

              <div v-if="item.toolCallName" class="tool-call-indicator">
                <Icon icon="mdi:cog-sync" width="14" color="#6b7280" />
                <span>正在调用工具：{{ item.toolCallName }}</span>
                <span v-if="item.toolCallDetail" class="tool-call-detail">{{ item.toolCallDetail }}</span>
              </div>
              <div v-if="item.sender === 'bot'" class="markdown-body" v-html="renderMarkdown(item.content, true)"></div>
              <div v-else>{{ item.content }}</div>
              <button v-if="isLoggedIn && item.sender === 'bot'" class="ds-btn ds-btn-msg" @click="playVoice(item.content)" title="播放语音">
                <Icon icon="mdi:microphone" width="14" color="#666" />
              </button>
            </div>
          </div>
        </template>
      </div>

      <!-- 输入区域（极简融合） -->
      <div class="chat-input-area">
        <!-- 参数面板（输入框上方弹出） -->
        <div v-if="showParams" class="params-panel">
          <div class="param-row">
            <span class="param-label">T</span>
            <input type="range" min="0" max="2" step="0.1" v-model.number="debugTemp" @change="updateParams" />
            <span class="param-value">{{ debugTemp }}</span>
          </div>
          <div class="param-row">
            <span class="param-label">TopP</span>
            <input type="range" min="0" max="1" step="0.05" v-model.number="debugTopP" @change="updateParams" />
            <span class="param-value">{{ debugTopP }}</span>
          </div>
          <div class="param-row">
            <span class="param-label">Tokens</span>
            <input type="number" v-model.number="debugMaxTokens" min="100" max="8192" step="100" @change="updateParams" />
          </div>
          <div class="param-row">
            <span class="param-label">思考</span>
            <select v-model="debugReasoning" @change="updateParams">
              <option value="">关闭</option>
              <option value="high">开启（高）</option>
              <option value="max">开启（最强）</option>
            </select>
          </div>
        </div>

        <div class="input-wrapper">
          <!-- 图片按钮：嵌入输入框左下角 -->
          <button v-if="isLoggedIn" class="input-inner-btn input-left-btn" @click="imageInput.click()" title="上传图片">
            <Icon icon="heroicons:photo-20-solid" width="18" color="#888" />
          </button>
          <!-- 参数按钮：紧挨图片按钮 -->
          <button v-if="isLoggedIn" class="input-inner-btn input-param-btn" @click="showParams = !showParams" title="参数">
            <Icon icon="mdi:tune" width="18" color="#888" />
          </button>
          <input type="file" accept="image/*" ref="imageInput" style="display:none" v-if="isLoggedIn" @change="handleImageUpload" />

          <textarea 
            ref="chatInputRef"
            class="chat-input" 
            :class="{ recording: isRecording }"
            v-model="userInput" 
            placeholder="输入你的问题..."
            @keypress.enter="sendMessage"
            @input="adjustInputHeight"
            rows="1"
          ></textarea>

          <!-- 内嵌状态栏（Token/延迟/余额） -->
          <div class="inline-status-bar" v-if="isLoggedIn">
            <span class="status-item">Token: {{ lastTokenUsage || '--' }}</span>
            <span class="status-item">延迟: {{ lastLatency || '--' }}ms</span>
            <span class="status-item">余额: {{ balance || '--' }}</span>
          </div>

          <!-- 语音按钮：嵌入输入框右下角 -->
          <button 
            v-if="!userInput.trim()" 
            class="input-inner-btn input-right-btn input-voice-btn" 
            :class="{ recording: isRecording }"
            @mousedown.prevent="startVoiceInput"
            @mouseup.prevent="stopVoiceAndSend"
            @mouseleave.prevent="stopVoiceAndSend"
            @touchstart.prevent="startVoiceInput"
            @touchend.prevent="stopVoiceAndSend"
            title="按住说话"
          >
            <Icon icon="mdi:microphone" width="20" :color="isRecording ? '#fff' : '#888'" />
          </button>

          <!-- 发送按钮：嵌入输入框右下角 -->
          <button v-else class="input-inner-btn input-right-btn input-send-btn" @click="sendMessage">
            <Icon icon="heroicons:paper-airplane-20-solid" width="18" color="#fff" />
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, watch, nextTick, computed, onMounted } from 'vue'
import { Icon } from '@iconify/vue'
import { marked } from 'marked'
import hljs from 'highlight.js'
import 'highlight.js/styles/atom-one-dark.min.css'
import katex from 'katex'
import markedKatex from 'marked-katex-extension'
import DOMPurify from 'dompurify'
import 'katex/dist/katex.min.css'
import { useChatWidget } from './useChatWidget.js'

// 只保留 markedKatex 配置，不再自定义 marked 渲染器
marked.use(markedKatex({ katex, throwOnError: false }))

// ===== 语音识别 =====
const isRecording = ref(false)
let recognition = null

function startVoiceInput() {
  const SpeechRecognition = window.SpeechRecognition || window.webkitSpeechRecognition
  if (!SpeechRecognition) {
    alert('你的浏览器不支持语音识别，请使用 Chrome 或 Edge')
    return
  }

  if (recognition) {
    try { recognition.abort() } catch (e) {}
  }

  isRecording.value = true

  navigator.mediaDevices.getUserMedia({
    audio: {
      echoCancellation: true,
      noiseSuppression: true,
      autoGainControl: true
    }
  }).then(stream => {
    recognition = new SpeechRecognition()
    recognition.lang = 'zh-CN'
    recognition.interimResults = false
    recognition.maxAlternatives = 3
    recognition.continuous = false

    recognition.onresult = (event) => {
      let best = event.results[0][0].transcript
      for (let i = 0; i < event.results[0].length; i++) {
        const alt = event.results[0][i]
        if (alt.confidence > 0.8) {
          best = alt.transcript
          break
        }
      }
      userInput.value = best
    }

    recognition.onerror = (event) => {
      console.error('语音识别错误:', event.error)
      isRecording.value = false
      if (event.error === 'not-allowed') {
        alert('请允许麦克风权限后重试')
      }
    }

    recognition.onend = () => {
      isRecording.value = false
      stream.getTracks().forEach(track => track.stop())
    }

    try {
      recognition.start()
    } catch (e) {
      console.error('语音识别启动失败:', e)
      isRecording.value = false
    }
  }).catch(err => {
    console.error('麦克风权限被拒绝:', err)
    isRecording.value = false
    alert('请允许麦克风权限后重试')
  })
}

function stopVoiceAndSend() {
  if (!recognition) return
  try { recognition.stop() } catch (e) {}
  isRecording.value = false
  setTimeout(() => {
    if (userInput.value.trim()) sendMessage()
  }, 300)
}

const props = defineProps({
  autoOpen: { type: Boolean, default: false },
  sessionId: { type: String, default: 'global_chat_session' }
})

// ===== 代码块高亮函数 =====
function highlightAllCodeBlocks() {
  requestAnimationFrame(() => {
   document.querySelectorAll('.chat-messages .markdown-body pre, .chat-messages .reasoning-text pre').forEach(pre => {
      const code = pre.querySelector('code')
      if (!code) return
      
      const classList = [...code.classList]
      const langClass = classList.find(c => c.startsWith('language-'))
      const lang = langClass ? langClass.replace('language-', '') : 'text'
      pre.setAttribute('data-lang', lang)
      
      hljs.highlightElement(code)
      
      if (!pre.querySelector('.copy-code-btn')) {
        const copyBtn = document.createElement('button')
        copyBtn.className = 'copy-code-btn'
        copyBtn.textContent = '复制'
        copyBtn.onclick = function() {
          navigator.clipboard.writeText(code.textContent || '').then(() => {
            copyBtn.textContent = '已复制'
            setTimeout(() => { copyBtn.textContent = '复制' }, 2000)
          })
        }
        pre.appendChild(copyBtn)
      }
    })
  })
}

function renderMarkdown(text, skipSanitize = false) {
  if (!text) return ''
  text = text
    .replace(/\u200B/g, '')
    .replace(/\u00A0/g, ' ')
    .replace(/\u200E/g, '')
    .replace(/\u200F/g, '')
  const raw = marked.parse(text)
  if (skipSanitize) return raw
  return DOMPurify.sanitize(raw)
}

const {
  isOpen, isExpanded, isMobile, userInput, messages,
  isLoggedIn, debugTemp, debugTopP, debugReasoning, lastTokenUsage, lastLatency, debugMaxTokens, balance,
  welcomeMessage, welcomeLoading, currentStatus, statusDotColor,
  messagesContainer, chatInputRef, userScrolledUp,
  forceScrollToBottom, smartScrollToBottom, smartScrollAndRefresh, adjustInputHeight,
  sendMessage, handleImageUpload, playVoice,
  toggleExpand, toggleChat, fetchBalance, updateParams,
  groupedMessages, formatChatTime
} = useChatWidget(props, { renderMarkdown })
const statusTextColor = computed(() => {
  const status = currentStatus.value
  if (!status) return '#98a2b3'
  if (status.includes('活跃') || status.includes('在线') || status.includes('帮忙') || status.includes('聊聊天')) return '#12b76a'
  if (status.includes('发呆') || status.includes('思绪') || status.includes('休眠')) return '#f59e0b'
  if (status.includes('忙碌') || status.includes('整理') || status.includes('写文章')) return '#ef4444'
  return '#98a2b3'
})
watch(messages, () => {
  nextTick(() => {
    highlightAllCodeBlocks()
  })
}, { deep: true })

// 参数面板显示状态
const showParams = ref(false)
</script>

<style scoped>
@import '../../../styles/shanxi/chat-window.css';
@import './chat-scoped.css';
</style>

<style>
@import './chat-global.css';
@import './chat-mobile.css';

/* ===== 新增输入框内嵌元素样式 ===== */
.input-wrapper {
  position: relative;
  display: flex;
  align-items: flex-end;
}

/* 参数按钮 */
.input-param-btn {
  left: 46px; /* 图片按钮宽度32px + 间距14px */
  background: transparent;
  width: 32px;
  height: 32px;
  border-radius: 50%;
  position: absolute;
  bottom: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  border: none;
  z-index: 2;
}
.input-param-btn:hover {
  background: rgba(0,0,0,0.05);
}

/* 内嵌状态栏（Token/延迟/余额） */
.inline-status-bar {
  position: absolute;
  bottom: 2px;
  left: 50%;
  transform: translateX(-50%);
  display: flex;
  gap: 12px;
  font-size: 10px;
  color: #aaa;
  font-family: monospace;
  white-space: nowrap;
  pointer-events: none;
  z-index: 3;
}

/* 输入框底部留出状态栏空间 */
.chat-input {
  padding-bottom: 22px !important; /* 额外留空 */
}

/* 参数面板（输入框上方弹出） */
.params-panel {
  margin-bottom: 8px;
  padding: 8px 12px;
  background: #f9fafb;
  border-radius: 12px;
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  font-size: 12px;
}
.param-row {
  display: flex;
  align-items: center;
  gap: 4px;
}
.param-label { width: 36px; color: #666; }
.param-value { width: 24px; text-align: right; }
.param-row input[type="range"] { width: 70px; }
.param-row input[type="number"] { width: 50px; border: 1px solid #d0d5dd; border-radius: 4px; padding: 2px 4px; }
.param-row select { border: 1px solid #d0d5dd; border-radius: 4px; padding: 2px 4px; background: #fff; }
</style>