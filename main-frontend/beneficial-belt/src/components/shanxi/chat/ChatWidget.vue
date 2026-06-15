<template>
  <div>
    <div class="chat-toggle-button" v-if="!isOpen" @click="toggleChat">
      <Icon icon="mdi:chat" width="28" color="white" />
    </div>

    <div v-if="isOpen && isExpanded" class="chat-overlay" @click="toggleExpand"></div>

<div class="chat-window" :class="{ expanded: isExpanded, mobile: isMobile }" :style="{ display: isOpen ? 'flex' : 'none' }">
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
          <button class="ds-btn" v-if="!isMobile"  @click="toggleExpand" :title="isExpanded ? '还原' : '放大'">
            <Icon :icon="isExpanded ? 'mdi:arrow-collapse' : 'mdi:arrow-expand'" width="16" color="#666" />
          </button>
          <button class="ds-btn" @click="toggleChat">
            <Icon icon="heroicons:x-mark-20-solid" width="16" color="#666" />
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
  <!-- 回忆状态提示 -->
  <div v-if="item.sender === 'bot' && item.recalling" class="recalling-hint">
    <Icon icon="mdi:memory" width="14" color="#6b7280" />
    <span>杉汐正在回忆与你的过去...</span>
  </div>


              <div v-if="item.reasoning" class="reasoning-stream">
                <div class="reasoning-label">
                  <Icon icon="la:atom" width="14" color="#6b7280" />
                  思考中...
                </div>
                <div class="reasoning-text">{{ item.reasoning }}</div>
              </div>


   <div v-if="item.toolCallName" class="tool-call-indicator">
    <Icon icon="mdi:cog-sync" width="14" color="#6b7280" />
    <span>正在调用工具：{{ item.toolCallName }}</span>
    <span v-if="item.toolCallDetail" class="tool-call-detail">{{ item.toolCallDetail }}</span>
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

           <!-- 输入区域（置底设计） -->
      <!-- 输入区域（置底设计） -->
<div class="chat-input-area">
  <!-- 图片上传按钮（始终在左侧） -->
  <button v-if="isLoggedIn" class="ds-btn ds-btn-icon" @click="imageInput.click()" title="上传图片">
    <Icon icon="heroicons:photo-20-solid" width="18" color="#666" />
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
    @focus="onInputFocus"
    @blur="onInputBlur"
    rows="1"
  ></textarea>


<!-- 语音按钮 -->
<button v-if="!userInput.trim()" class="ds-btn ds-btn-voice" :class="{ recording: isRecording }" @click="startVoiceInput" title="语音输入">
  <Icon icon="mdi:microphone" width="20" :color="isRecording ? '#fff' : '#666'" />
</button>

  <!-- 发送按钮（输入框有内容时显示） -->
  <button v-else class="ds-btn ds-btn-send" @click="sendMessage">
    <Icon icon="heroicons:paper-airplane-20-solid" width="18" color="#fff" />
  </button>
</div>

      <!-- 调试参数（纯文字排版） -->
      <details class="debug-panel" v-if="isLoggedIn">
        <summary>
          <Icon icon="mdi:tune" width="14" color="#888" />
          
          <span class="debug-badge">{{ lastTokenUsage || '0' }}T / {{ lastLatency || '0' }}ms</span>
        </summary>
        <div class="debug-content">
          <div class="debug-row">
            <span class="debug-label">T</span>
            <input type="range" min="0" max="2" step="0.1" v-model.number="debugTemp" @change="updateParams" />
            <span class="debug-value">{{ debugTemp }}</span>
          </div>
          <div class="debug-row">
            <span class="debug-label">TopP</span>
            <input type="range" min="0" max="1" step="0.05" v-model.number="debugTopP" @change="updateParams" />
            <span class="debug-value">{{ debugTopP }}</span>
          </div>
          <div class="debug-row">
            <span class="debug-label">Tokens</span>
            <input type="number" v-model.number="debugMaxTokens" min="100" max="8192" step="100" @change="updateParams" />
          </div>
          <div class="debug-row">
            <span class="debug-label">思考</span>
            <select v-model="debugReasoning" @change="updateParams">
              <option value="">关闭</option>
              <option value="high">开启（高）</option>
              <option value="max">开启（最强）</option>
            </select>
          </div>
          <div class="debug-row debug-stats">
            <span>余额 {{ balance || '--' }}</span>
            <button class="debug-refresh-btn" @click="fetchBalance">刷新</button>
          </div>
        </div>
      </details>
    </div>
  </div>
</template>

<script setup>
import { ref, watch, nextTick, computed, onMounted } from 'vue'
import { Icon } from '@iconify/vue'
import { marked } from 'marked'          // marked 只导出 marked
import katex from 'katex'
import markedKatex from 'marked-katex-extension'
import DOMPurify from 'dompurify'
import 'katex/dist/katex.min.css'

// 引入解耦后的逻辑
import { useChatWidget } from './useChatWidget.js'




// 语音识别
const isRecording = ref(false)
let recognition = null

function startVoiceInput() {
  const SpeechRecognition = window.SpeechRecognition || window.webkitSpeechRecognition
  if (!SpeechRecognition) {
    alert('你的浏览器不支持语音识别，请使用 Chrome 或 Edge')
    return
  }

  // 如果之前有识别正在进行，先中止
  if (recognition) {
    try {
      recognition.abort()
    } catch (e) {}
  }

  // 强制设置为录音状态，确保动画触发
  isRecording.value = true

  // 每次创建新的识别实例，避免 already started 错误
  recognition = new SpeechRecognition()
  recognition.lang = 'zh-CN'
  recognition.interimResults = false
  recognition.maxAlternatives = 1
  recognition.continuous = false

  recognition.onresult = (event) => {
    const transcript = event.results[0][0].transcript
    userInput.value = transcript
    isRecording.value = false
    nextTick(() => {
      sendMessage()
    })
  }

  recognition.onerror = (event) => {
    console.error('语音识别错误:', event.error)
    isRecording.value = false
    if (event.error === 'not-allowed') {
      alert('请允许麦克风权限后重试')
    }
    // no-speech, audio-capture 等其他错误静默处理，不需要弹窗
  }

  recognition.onend = () => {
    isRecording.value = false
  }

  try {
    recognition.start()
  } catch (e) {
    console.error('语音识别启动失败:', e)
    isRecording.value = false
  }
}
marked.use(markedKatex({ katex, throwOnError: false }))

const props = defineProps({
  autoOpen: { type: Boolean, default: false },
  sessionId: { type: String, default: 'global_chat_session' }
})

// 保留 renderMarkdown，因为模板中直接用
function renderMarkdown(text, skipSanitize = false) {
  if (!text) return ''
  
  // 清洗可能导致 marked 解析异常的 Unicode 字符
  text = text
    .replace(/\u200B/g, '')
    .replace(/\u00A0/g, ' ')
    .replace(/\u200E/g, '')
    .replace(/\u200F/g, '')

  // 保护中文书名号/括号等标点，防止 marked 误判为标记符
  text = text
    .replace(/\uff08/g, '___OP_BRACKET___')  // 中文左书名号
    .replace(/\uff09/g, '___CL_BRACKET___')  // 中文右书名号

  const raw = marked.parse(text)
  
  // 还原占位符
  const restored = raw
    .replace(/___OP_BRACKET___/g, '（')
    .replace(/___CL_BRACKET___/g, '）')

  if (skipSanitize) return restored
  return DOMPurify.sanitize(restored)
}

// 从 useChatWidget 获取所有响应式状态和方法
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
</script>

<style scoped>
/* 保留必要的 scoped 样式，也可以外移到 chat-scoped.css，但保留内联最安全 */
@import '../../../styles/shanxi/chat-window.css';
@import './chat-scoped.css';   /* 可选：把 scoped 样式也提出去，这里先保持内联 */



</style>

<style>
@import './chat-global.css';
@import './chat-mobile.css';
/* 语音录音时输入框蓝色脉冲 */
.chat-input.recording {
  border-color: #2563eb !important;
  box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.2) !important;
  animation: shanxi-pulse-border 1.5s ease-in-out infinite !important;
}

@keyframes shanxi-pulse-border {
  0%, 100% { box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.2); }
  50% { box-shadow: 0 0 0 8px rgba(37, 99, 235, 0.4); }
}

/* 语音按钮录音时变红并脉冲 */
.ds-btn-voice.recording {
  background: #ef4444 !important;
  animation: shanxi-pulse-bg 1s ease-in-out infinite !important;
}

@keyframes shanxi-pulse-bg {
  0%, 100% { box-shadow: 0 0 0 0 rgba(239, 68, 68, 0.6); }
  50% { box-shadow: 0 0 0 12px rgba(239, 68, 68, 0); }
}

.ds-btn-voice.recording .iconify {
  color: #fff !important;
}
</style>