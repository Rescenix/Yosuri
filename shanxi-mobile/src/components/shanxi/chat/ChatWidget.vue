<template>
  <div>
    <div class="chat-toggle-button" v-if="!isOpen" @click="toggleChat">
      <Icon icon="mdi:chat" width="28" color="white" />
    </div>

    <div v-if="isOpen && isExpanded" class="chat-overlay" @click="toggleExpand"></div>

    <div class="chat-window" :class="{ expanded: isExpanded, mobile: isMobile }" :style="{ display: isOpen ? 'flex' : 'none' }">
      <!-- 头部：状态行移到名字下方 -->
      <div class="chat-header">
        <div class="header-left">
          <span class="header-name">杉汐</span>
          <div class="header-status">
            <span class="status-dot" :style="{ background: statusDotColor }"></span>
            <span class="status-text" :style="{ color: statusTextColor }">{{ currentStatus }}</span>
          </div>
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

      <!-- 输入区域 -->
      <div class="chat-input-area">
        <!-- 参数面板 -->
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
          <!-- 图片按钮 -->
          <button v-if="isLoggedIn" class="input-inner-btn input-left-btn" @click="$refs.imageInput.click()" title="上传图片">
            <Icon icon="heroicons:photo-20-solid" width="18" color="#888" />
          </button>
          <!-- 参数按钮 -->
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

          <!-- 内嵌状态栏 -->
          <div class="inline-status-bar" v-if="isLoggedIn">
            <span class="status-item">Token: {{ lastTokenUsage || '--' }}</span>
            <span class="status-item">延迟: {{ lastLatency || '--' }}ms</span>
            <span class="status-item">余额: {{ balance || '--' }}</span>
          </div>

          <!-- 语音按钮 -->
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

          <!-- 发送按钮 -->
          <button v-else class="input-inner-btn input-right-btn input-send-btn" @click="sendMessage">
            <Icon icon="heroicons:paper-airplane-20-solid" width="18" color="#fff" />
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, watch, nextTick, computed } from 'vue'
import { Icon } from '@iconify/vue'
import hljs from 'highlight.js'
import 'highlight.js/styles/atom-one-dark.min.css'
import DOMPurify from 'dompurify'
import 'katex/dist/katex.min.css'

import MarkdownIt from 'markdown-it'
import markdownItKatex from 'markdown-it-katex'

import { useChatWidget } from './useChatWidget.js'

// ===== 全局语音回调（供原生接口调用） =====
window.onVoiceResult = (text) => {
  if (text && !text.startsWith('语音识别出错')) {
    userInput.value = text
    nextTick(() => {
      sendMessage()
    })
  } else {
    alert(text || '语音识别出错')
  }
  isRecording.value = false
}

window.onVoiceError = (msg) => {
  alert('语音识别失败: ' + msg)
  isRecording.value = false
}

// 创建 markdown-it 实例
const md = new MarkdownIt({
  breaks: true,
  linkify: true,
  html: true
})

// 修正后的插件：智能转换 [ ... ] 为行内或块级公式
md.use(function(md) {
  md.core.ruler.before('normalize', 'math_bracket', function(state) {
    state.src = state.src.replace(/\[([\s\S]*?)\]/g, (match, inner) => {
      if (!/\\[a-zA-Z]+/.test(inner)) return match;
      if (/^\s*\${1,2}[\s\S]*\${1,2}\s*$/.test(inner)) return match;
      
      const trimmed = inner.trim();
      if (trimmed.includes('\n') || trimmed.length > 60 || /\\begin\{/.test(trimmed)) {
        return `$$\n${trimmed}\n$$`;
      }
      return `$${trimmed}$`;
    });
    return true;
  });
});

md.use(markdownItKatex, {
  throwOnError: false,
  errorColor: '#ef4444',
  strict: false
})

hljs.registerLanguage('math', function() {
  return { name: 'math' }
})

// ===== 语音状态 =====
const isRecording = ref(false)

const startVoiceInput = () => {
  if (window.NativeVoice) {
    window.NativeVoice.startListening()
    isRecording.value = true
  } else {
    alert('原生语音接口不可用')
  }
}

const stopVoiceAndSend = () => {
  if (window.NativeVoice) {
    window.NativeVoice.stopListening()
  }
  isRecording.value = false
}

const props = defineProps({
  autoOpen: { type: Boolean, default: false },
  sessionId: { type: String, default: 'global_chat_session' }
})

// ===== 代码块高亮函数 =====
function highlightAllCodeBlocks() {
  requestAnimationFrame(() => {
    document.querySelectorAll('.chat-messages .markdown-body pre').forEach(pre => {
      const code = pre.querySelector('code')
      if (!code) return

      const classList = [...code.classList]
      const langClass = classList.find(c => c.startsWith('language-'))
      const lang = langClass ? langClass.replace('language-', '') : 'text'
      pre.setAttribute('data-lang', lang)

      hljs.highlightElement(code)

      if (!pre.querySelector('.code-btn-group')) {
        const btnGroup = document.createElement('div')
        btnGroup.className = 'code-btn-group'

        const runBtn = document.createElement('button')
        runBtn.className = 'run-code-btn'
        runBtn.textContent = '运行'
        runBtn.onclick = async function() {
          runBtn.textContent = '运行中...'
          runBtn.disabled = true
          try {
            const res = await fetch('/api/run', {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ language: lang, code: code.textContent })
            })
            const data = await res.json()
            const oldOutput = pre.parentElement.querySelector('.code-output')
            if (oldOutput) oldOutput.remove()
            const outputDiv = document.createElement('div')
            outputDiv.className = 'code-output'
            const errorHtml = data.error ? `<span style="color:#ef4444;font-weight:500;">${data.error}</span>` : ''
            const outputText = (data.output || '(无输出)').trim()
            outputDiv.innerHTML = errorHtml
            const textDiv = document.createElement('div')
            textDiv.textContent = outputText
            textDiv.style.cssText = `
              margin:0;
              padding:8px 12px;
              background:transparent;
              color:#333;
              border:none;
              box-shadow:none;
              line-height:1.5;
              white-space:pre-wrap;
              word-break:break-word;
              font-family: monospace;
            `
            outputDiv.appendChild(textDiv)
            outputDiv.style.cssText = `
              margin-top: 0 !important;
              padding: 0 !important;
              background: #f5f5f5 !important;
              border-radius: 0 0 8px 8px !important;
              font-size: 13px !important;
              color: #333333 !important;
              white-space: pre-wrap !important;
              width: 100% !important;
              box-sizing: border-box !important;
              border: none !important;
              box-shadow: none !important;
            `
            pre.parentElement.insertBefore(outputDiv, pre.nextSibling)
          } catch (e) {
            alert('运行失败: ' + e.message)
          } finally {
            runBtn.textContent = '运行'
            runBtn.disabled = false
          }
        }

        const copyBtn = document.createElement('button')
        copyBtn.className = 'copy-code-btn'
        copyBtn.textContent = '复制'
        copyBtn.onclick = function() {
          navigator.clipboard.writeText(code.textContent || '').then(() => {
            copyBtn.textContent = '已复制'
            setTimeout(() => { copyBtn.textContent = '复制' }, 2000)
          })
        }

        btnGroup.appendChild(runBtn)
        btnGroup.appendChild(copyBtn)
        pre.appendChild(btnGroup)
      }
    })
  })
}

function renderMarkdown(text, skipSanitize = false) {
  if (!text) return ''
  text = text.replace(/[\u200B\u00A0\u200E\u200F]/g, '')
  text = text.replace(/\\big\$/g, '')
  text = text.replace(/\\big\\\$/g, '')
  text = text.replace(/\\dots/g, '\\ldots')
  text = text.replace(/(?<!\$)\\implies(?!\$)/g, ' $\\implies$ ')
  text = text.replace(/(?<!\$)(\\bbox\[[^\]]*\])(?!\$)/g, (match) => `$${match}$`)
  if (/\\bbox/.test(text)) {
    text = '\\require{bbox}\n' + text
  }
  text = text.replace(/\\boxed\{([^}]*)\}/g, (_, content) => {
    return `\\bbox[border:1px solid black]{${content}}`
  })

  const raw = md.render(text)
  return skipSanitize ? raw : DOMPurify.sanitize(raw)
}

const {
  isOpen, isExpanded, isMobile, userInput, messages,
  isLoggedIn, debugTemp, debugTopP, debugReasoning, lastTokenUsage, lastLatency, debugMaxTokens, balance,
  welcomeMessage, welcomeLoading, currentStatus, statusDotColor,
  messagesContainer, chatInputRef, userScrolledUp,
  forceScrollToBottom, smartScrollToBottom, smartScrollAndRefresh, adjustInputHeight,
  sendMessage, handleImageUpload, playVoice,
  toggleExpand, toggleChat, fetchBalance, updateParams,
  groupedMessages, formatChatTime,
} = useChatWidget(props, { renderMarkdown })

isLoggedIn.value = true

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

const showParams = ref(false)
</script>
<style scoped>
@import '../../../styles/shanxi/chat-window.css';
@import './chat-scoped.css';

/* ===== 头部修正：适当字体大小 ===== */
.chat-header .header-left {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
}

.chat-header .header-status {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 0.75rem;
}

.chat-header .status-text {
  font-size: 0.75rem;
  opacity: 0.7;
}

.chat-header .header-name {
  font-size: 1rem;
}

/* ===== 输入框高度 ===== */
.chat-input {
  min-height: 80px;
  padding: 12px 16px;
  font-size: 0.95rem;
  line-height: 1.5;
}

/* ===== 手机端全屏不滑动 + 对齐修正 ===== */
@media (max-width: 768px) {
  html, body {
    width: 100vw;
    height: 100vh;
    overflow: hidden;
    position: fixed;
    margin: 0;
    padding: 0;
  }

  .chat-window {
    width: 100vw !important;
    height: 100vh !important;
    max-width: 100vw !important;
    max-height: 100vh !important;
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }

  .chat-messages {
    flex: 1;
    overflow-y: auto;
    overflow-x: hidden;
  }

  .message-row {
    max-width: 90vw !important;
    margin-left: auto !important;
    margin-right: auto !important;
  }

  .message-row.user {
    justify-content: flex-end;
    margin-right: 10% !important;
  }

  .message.user {
    max-width: 80% !important;
    margin-right: 6% !important;
  }

  pre, code, .katex-display, table {
    max-width: 100% !important;
    overflow-x: auto !important;
    word-wrap: break-word;
  }
}
</style>

<style>
@import './chat-global.css';
@import './chat-mobile.css';
</style>