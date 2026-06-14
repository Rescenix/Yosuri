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
                  <Icon icon="la:atom" width="14" color="#6b7280" />
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

           <!-- 输入区域（置底设计） -->
      <div class="chat-input-area">
        <button v-if="isLoggedIn" class="ds-btn ds-btn-icon" @click="imageInput.click()" title="上传图片">
          <Icon icon="heroicons:photo-20-solid" width="18" color="#666" />
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

        <button class="ds-btn ds-btn-send" @click="sendMessage" :disabled="!userInput.trim()">
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

import { Icon } from '@iconify/vue'


import { marked } from 'marked'
import katex from 'katex'
import markedKatex from 'marked-katex-extension'
import DOMPurify from 'dompurify'
import 'katex/dist/katex.min.css'

// 引入解耦后的逻辑
import { useChatWidget } from './useChatWidget.js'

marked.use(markedKatex({ katex, throwOnError: false }))

const props = defineProps({
  autoOpen: { type: Boolean, default: false },
  sessionId: { type: String, default: 'global_chat_session' }
})

// 保留 renderMarkdown，因为模板中直接用
function renderMarkdown(text, skipSanitize = false) {
  if (!text) return ''
  const raw = marked.parse(text)
  if (skipSanitize) return raw
  return DOMPurify.sanitize(raw)
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
</style>