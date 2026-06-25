<template>
  <div>
    <div class="chat-toggle-button" v-if="!isOpen" @click="toggleChat">
      <Icon icon="mdi:chat" width="28" color="white" />
    </div>

    <div v-if="isOpen && isExpanded" class="chat-overlay" @click="toggleExpand"></div>

    <div class="chat-window" :class="{ expanded: isExpanded, mobile: isMobile }" :style="{ display: isOpen ? 'flex' : 'none' }">
      <div class="chat-header">
        <div class="header-left">
          <button class="header-menu-btn" @click="toggleSidebar" aria-label="展开导航">
            <Icon icon="mdi:menu" width="18" color="#696259" />
          </button>
          <span class="header-name">杉汐</span>
          <span class="status-dot" :style="{ background: statusDotColor }"></span>
          <span class="status-text" :style="{ color: statusTextColor }">{{ currentStatus }}</span>
        </div>
      </div>

      <div v-if="sidebarOpen" class="chat-sidebar-backdrop" @click="toggleSidebar"></div>
      <aside class="chat-sidebar" :class="{ open: sidebarOpen }">
        <h4>星辰核心</h4>
        <a href="/shanxi-hut">项目库</a>
        <a href="/blog">研习录</a>
        <a href="/timeline">生命线</a>
      </aside>

      <div class="chat-content">
        <button
          v-show="showScrollButton"
          class="scroll-to-bottom-btn"
          @click="forceScrollToBottom"
          title="回到底部"
        >
          <Icon icon="mdi:chevron-down" width="20" color="#555" />
        </button>

        <div class="chat-messages" ref="messagesContainer">
          <div v-if="messages.length === 0 && !welcomeLoading" class="message-row bot">
            <div class="assistant-message">{{ welcomeMessage }}</div>
          </div>
          <div v-if="messages.length === 0 && welcomeLoading" class="message-row bot">
            <div class="assistant-message" style="opacity:0.6">杉汐正在想起你...</div>
          </div>

          <template v-for="item in groupedMessages">
            <div v-if="item.type === 'time'" :key="`time-${item.timestamp}`" class="chat-time">
              {{ formatChatTime(item.timestamp) }}
            </div>
            <div v-else-if="item.type === 'message'" :key="item.id" class="message-row" :class="item.sender">
              <div v-if="item.type === 'image'" class="image-card">
                <img :src="item.image" style="max-width: 240px; border-radius: 12px;" />
              </div>
              <div v-else-if="item.sender === 'user'" class="message-bubble user">
                {{ item.content }}
              </div>
              <div v-else class="assistant-message">
                <div v-if="item.recalling" class="recalling-hint">
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
                <div class="markdown-body" v-html="renderMarkdown(item.content, true)"></div>
                <div class="assistant-tools">
                  <button class="tool-btn" @click="playVoice(item.content)" title="朗读">
                    <Icon icon="mdi:volume-high" width="16" />
                  </button>
                  <button class="tool-btn" @click="copyText(item.content)" title="复制">
                    <Icon icon="mdi:content-copy" width="16" />
                  </button>
                </div>
              </div>
            </div>
          </template>
        </div>

        <div v-if="copiedVisible" class="copy-toast">✓ 已复制</div>

        <div class="chat-input-area">
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
            <button v-if="isLoggedIn" class="input-inner-btn input-left-btn" @click="$refs.imageInput.click()" title="上传图片">
              <Icon icon="heroicons:photo-20-solid" width="18" color="#888" />
            </button>
            <button v-if="isLoggedIn" class="input-inner-btn input-param-btn" @click="showParams = !showParams" title="参数">
              <Icon icon="mdi:tune" width="18" color="#888" />
            </button>
            <input type="file" accept="image/*" ref="imageInput" style="display:none" v-if="isLoggedIn" @change="handleImageUpload" />

            <textarea
              ref="chatInputRef"
              class="chat-input"
              v-model="userInput"
              placeholder="输入你的问题..."
              @keypress.enter="sendMessage"
              @input="adjustInputHeight"
              rows="1"
            ></textarea>

            <div class="inline-status-bar" v-if="isLoggedIn">
              <span class="status-item">Token: {{ lastTokenUsage || '--' }}</span>
              <span class="status-item">延迟: {{ lastLatency || '--' }}ms</span>
              <span class="status-item">余额: {{ balance || '--' }}</span>
            </div>

            <button v-if="userInput.trim()" class="input-inner-btn input-right-btn input-send-btn" @click="sendMessage">
              <Icon icon="heroicons:paper-airplane-20-solid" width="18" color="#fff" />
            </button>
          </div>
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

// 复制提示
const copiedVisible = ref(false)
async function copyText(text) {
  try {
    await navigator.clipboard.writeText(text)
    copiedVisible.value = true
    setTimeout(() => { copiedVisible.value = false }, 2000)
    return true
  } catch (err) {
    const textarea = document.createElement('textarea')
    textarea.value = text
    textarea.style.position = 'fixed'
    textarea.style.opacity = '0'
    document.body.appendChild(textarea)
    textarea.select()
    try {
      document.execCommand('copy')
      copiedVisible.value = true
      setTimeout(() => { copiedVisible.value = false }, 2000)
      return true
    } catch (e) {
      console.error('复制失败:', e)
      return false
    } finally {
      document.body.removeChild(textarea)
    }
  }
}

// Markdown 渲染
const md = new MarkdownIt({
  breaks: true,
  linkify: true,
  html: true
})
md.use(markdownItKatex, {
  throwOnError: false,
  errorColor: '#ef4444',
  strict: false
})
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
})

function renderMarkdown(text, skipSanitize = false) {
  if (!text) return ''
  text = text.replace(/[\u200B\u00A0\u200E\u200F]/g, '')
  text = text.replace(/\\dots/g, '\\ldots')
  text = text.replace(/(?<!\$)\\implies(?!\$)/g, ' $\\implies$ ')
  text = text.replace(/(?<!\$)(\\bbox\[[^\]]*\])(?!\$)/g, (match) => `$${match}$`)
  if (/\\bbox/.test(text)) text = '\\require{bbox}\n' + text
  const raw = md.render(text)
  return skipSanitize ? raw : DOMPurify.sanitize(raw)
}

// 代码块高亮 & 复制
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
        const copyBtn = document.createElement('button')
        copyBtn.className = 'copy-code-btn'
        copyBtn.textContent = '复制'
        copyBtn.onclick = async () => {
          const success = await copyText(code.textContent || '')
          if (success) {
            copyBtn.textContent = '已复制'
            setTimeout(() => { copyBtn.textContent = '复制' }, 2000)
          }
        }
        btnGroup.appendChild(copyBtn)
        pre.appendChild(btnGroup)
      }
    })
  })
}

// Props 必须在 useChatWidget 之前定义
const props = defineProps({
  autoOpen: { type: Boolean, default: false },
  sessionId: { type: String, default: 'global_chat_session' }
})

import { useChatWidget } from './useChatWidget.js'

const {
  isOpen, isExpanded, isMobile, userInput, messages,
  isLoggedIn, debugTemp, debugTopP, debugReasoning, lastTokenUsage, lastLatency, debugMaxTokens, balance,
  welcomeMessage, welcomeLoading, currentStatus, statusDotColor,
  messagesContainer, chatInputRef, userScrolledUp,
  forceScrollToBottom, adjustInputHeight,
  sendMessage, handleImageUpload, playVoice,
  toggleExpand, toggleChat, updateParams,
  groupedMessages, formatChatTime
} = useChatWidget(props, { renderMarkdown })

const sidebarOpen = ref(false)
const toggleSidebar = () => { sidebarOpen.value = !sidebarOpen.value }

const showParams = ref(false)

const statusTextColor = computed(() => {
  const status = currentStatus.value
  if (!status) return '#98a2b3'
  if (status.includes('活跃') || status.includes('在线') || status.includes('帮忙') || status.includes('聊聊天')) return '#12b76a'
  if (status.includes('发呆') || status.includes('思绪') || status.includes('休眠')) return '#f59e0b'
  if (status.includes('忙碌') || status.includes('整理') || status.includes('写文章')) return '#ef4444'
  return '#98a2b3'
})

const showScrollButton = computed(() => {
  return isOpen.value && userScrolledUp.value
})

watch(messages, () => {
  nextTick(() => {
    highlightAllCodeBlocks()
  })
}, { deep: true })
</script>

<style scoped>
@import '../../../styles/shanxi/chat-window.css';

.copy-toast {
  position: fixed;
  bottom: 120px;
  left: 50%;
  transform: translateX(-50%);
  background: rgba(52, 51, 51, 0.75);
  backdrop-filter: blur(4px);
  color: #fff;
  padding: 6px 18px;
  border-radius: 20px;
  font-size: 13px;
  z-index: 1000;
  pointer-events: none;
  animation: copyFadeIn 0.2s ease;
}

@keyframes copyFadeIn {
  from { opacity: 0; transform: translateX(-50%) translateY(4px); }
  to   { opacity: 1; transform: translateX(-50%) translateY(0); }
}

.message-bubble.user {
  font-family: 'Georgia', 'Noto Serif SC', 'Source Han Serif SC', 'Songti SC', '宋体', serif;
}
</style>

<style>
@import './chat-global.css';
</style>