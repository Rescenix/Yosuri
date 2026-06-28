
<template>
  <div>
    <div class="chat-toggle-button" v-if="!isOpen" @click="toggleChat">
      <Icon icon="mdi:chat" width="28" color="white" />
    </div>

    <div v-if="isOpen && isExpanded" class="chat-overlay" @click="toggleExpand"></div>

    <div class="chat-window" :class="{ expanded: isExpanded, mobile: isMobile }" :style="{ display: isOpen ? 'flex' : 'none' }">
      
      <!-- ★ 左侧折叠菜单（侧边抽屉） -->
      <div v-if="menuOpen" class="drawer-backdrop" @click="menuOpen = false"></div>
      <aside class="drawer-panel" :class="{ open: menuOpen }">
        
   <h4 class="site-icon">
  <Icon icon="majesticons:shooting-star-line" width="20" />
</h4>

        <a href="/shanxi-hut" @click="menuOpen = false">
          <Icon icon="mdi:archive-outline" width="16" style="margin-right:8px" />
          项目库
        </a>
        <a href="/blog" @click="menuOpen = false">
          <Icon icon="mdi:book-open-outline" width="16" style="margin-right:8px" />
          研习录
        </a>
        <a href="/timeline" @click="menuOpen = false">
          <Icon icon="mdi:timeline-clock-outline" width="16" style="margin-right:8px" />
          生命线
        </a>
        <a href="https://github.com/3478556810" target="_blank" rel="noopener" @click="menuOpen = false">
          <Icon icon="mdi:github" width="16" style="margin-right:8px" />
          GitHub
        </a>

        <!-- ★ 分隔线 -->
  <!-- ★ 分隔线 -->
<div class="drawer-divider"></div>

<!-- ★ 新建对话按钮 -->
<div class="new-session-btn" @click="createNewSession">
  <Icon icon="mdi:plus" width="16" color="#696259" />
  <span>新建对话</span>
</div>

<!-- ★ 会话列表 -->
<!-- ★ 会话列表 -->
<div class="session-list">
  <div
    v-for="sess in sessionList"
    :key="sess.id"
    class="session-item"
    :class="{ active: sess.id === sessionId }"
    @click="switchSession(sess.id)"
  >
    <div class="session-main">
      <span class="session-title">{{ sess.title || '新对话' }}</span>
      <span class="session-time">{{ formatSessionTime(sess.updated_at) }}</span>
    </div>

    <!-- 三点按钮 -->
  <div class="session-menu-wrapper" @click.stop>
  <button class="session-menu-btn" @click.stop="toggleSessionMenu($event, sess.id)">
    <Icon icon="mdi:dots-horizontal" width="14" color="#8b847a" />
  </button>
</div>
  </div>
</div>

<Teleport to="body">
  <!-- 三点菜单（保持原样） -->
  <div v-if="activeMenuId !== null" class="session-menu-dropdown" :style="{ top: menuPosition.top + 'px', left: menuPosition.left + 'px' }">
    <button @click.stop="openRenameDialog(currentMenuSession)">重命名</button>
    <button @click.stop="requestDelete(currentMenuSession.id, currentMenuSession.title)">删除</button>
  </div>

  <!-- 删除确认弹窗 -->
  <div v-if="confirmDeleteId !== null" class="confirm-overlay" @click="cancelDelete">
    <div class="confirm-dialog" @click.stop>
      <p>确定删除「{{ confirmDeleteTitle }}」吗？</p>
      <div class="confirm-actions">
        <button class="btn-cancel" @click="cancelDelete">取消</button>
        <button class="btn-delete" @click="confirmDelete">删除</button>
      </div>
    </div>
  </div>

  <!-- 重命名弹窗 -->
  <div v-if="renameTarget !== null" class="confirm-overlay" @click="cancelRename">
    <div class="confirm-dialog" @click.stop>
      <p>重命名「{{ renameTarget.title || '新会话' }}」</p>
      <input
        v-model="renameInputValue"
        class="rename-input"
        placeholder="输入新名称"
        @keyup.enter="confirmRename"
        autofocus
      />
      <div class="confirm-actions">
        <button class="btn-cancel" @click="cancelRename">取消</button>
        <button class="btn-delete" @click="confirmRename">确定</button>
      </div>
    </div>
  </div>
</Teleport>
      </aside>

      <!-- ★ 主内容区（随菜单推开） -->
      <div class="chat-main" :class="{ shifted: menuOpen }">
        <div class="chat-header">
          <div class="header-left">
            <button class="header-menu-btn" @click="menuOpen = !menuOpen" aria-label="展开导航">
              <Icon icon="mdi:menu" width="18" color="#696259" />
            </button>
            <span class="header-name">杉汐</span>
            <span class="status-dot" :style="{ background: statusDotColor }"></span>
            <span class="status-text" :style="{ color: statusTextColor }">{{ currentStatus }}</span>
          </div>
        </div>

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
    <Icon icon="majesticons:shooting-star-line" width="14" color="#6b7280" />
    <span class="recalling-text">杉汐正在回忆与你的过去</span>
    <span class="recalling-dots">
      <span class="dot">.</span>
      <span class="dot">.</span>
      <span class="dot">.</span>
    </span>
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
  <!-- ★ 模型选择按钮 -->
  <button v-if="isLoggedIn" class="input-inner-btn input-model-btn" @click.stop="showModelMenu = !showModelMenu" title="切换模型">
    <Icon :icon="currentModelIcon" width="18" color="#888" />
  </button>
  
  <!-- ★ 模型弹出菜单 -->
  <div v-if="showModelMenu" class="model-menu">
    <div 
      v-for="m in modelOptions" 
      :key="m.value" 
      class="model-option" 
      :class="{ active: selectedModel === m.value }"
      @click="selectModel(m.value)"
    >
      <Icon :icon="m.icon" width="16" style="margin-right:8px" />
      <span>{{ m.label }}</span>
    </div>
  </div>

  <!-- 原有的上传图片按钮 -->
  <button v-if="isLoggedIn" class="input-inner-btn input-left-btn" @click="$refs.imageInput.click()" title="上传图片">
    <Icon icon="heroicons:photo-20-solid" width="18" color="#888" />
  </button>

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
  </div>
</template>

<script setup>
import { ref, watch, nextTick, computed, onMounted } from 'vue'
import { Icon } from '@iconify/vue'
import hljs from 'highlight.js'
import 'highlight.js/styles/atom-one-dark.min.css'
import DOMPurify from 'dompurify'
import 'katex/dist/katex.min.css'
import MarkdownIt from 'markdown-it'
import markdownItKatex from 'markdown-it-katex'
import { useChatWidget } from './useChatWidget.js'

// ==================== Props ====================
const props = defineProps({
  autoOpen: { type: Boolean, default: false },
  sessionId: { type: String, default: 'global_chat_session' }
})

// ==================== 工具函数 ====================
function cleanContent(content) {
  return content ? content.replace(/\[(action|emotion):[^\]]*\]/g, '') : ''
}

function formatSessionTime(ts) {
  if (!ts) return ''
  const d = new Date(ts)
  return `${d.getMonth() + 1}/${d.getDate()} ${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}`
}

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


// 模型选项
const modelOptions = [
  { label: '本地 7B', value: 'local', icon: 'mdi:memory' },
  { label: 'Cloud 480B', value: 'cloud', icon: 'mdi:cloud' },
  { label: 'DS 官方', value: 'ds', icon: 'mdi:api' },
  { label: 'DS 浏览器', value: 'ds_browser', icon: 'mdi:web' },
]

const selectedModel = ref(localStorage.getItem('selectedModel') || 'ds_browser')
const showModelMenu = ref(false)

const currentModelIcon = computed(() => {
  const model = modelOptions.find(m => m.value === selectedModel.value)
  return model ? model.icon : 'mdi:help-circle'
})

function selectModel(value) {
  selectedModel.value = value
  localStorage.setItem('selectedModel', value)
  showModelMenu.value = false
}

// 点击页面其他地方关闭菜单
onMounted(() => {
  document.addEventListener('click', () => {
    showModelMenu.value = false
  })
})
// ==================== Markdown 渲染 ====================
const md = new MarkdownIt({ breaks: true, linkify: true, html: true })
md.use(markdownItKatex, { throwOnError: false, errorColor: '#ef4444', strict: false })
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

// ==================== useChatWidget ====================
const {
  isOpen, isExpanded, isMobile, userInput, messages, sessionId,
  isLoggedIn, debugTemp, debugTopP, debugReasoning, lastTokenUsage, lastLatency, debugMaxTokens, balance,
  welcomeMessage, welcomeLoading, currentStatus, statusDotColor,
  messagesContainer, chatInputRef, userScrolledUp,
  forceScrollToBottom, adjustInputHeight,
  sendMessage, handleImageUpload, playVoice,
  toggleExpand, toggleChat, updateParams,
  groupedMessages, formatChatTime
} = useChatWidget(props, { renderMarkdown })

// ==================== UI 状态 ====================
const menuOpen = ref(false)
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
  nextTick(() => { highlightAllCodeBlocks() })
}, { deep: true })

// ==================== 会话列表 ====================
const sessionList = ref([])

async function fetchSessionList() {
  try {
    const res = await fetch('/api/sessions')
    if (res.ok) {
      const data = await res.json()
      sessionList.value = Array.isArray(data) ? data : []
    } else {
      sessionList.value = []
    }
  } catch (e) {
    console.error('加载会话列表失败', e)
    sessionList.value = []
  }
}

function createNewSession() {
  const newId = 'sess_' + Date.now().toString(36)
  localStorage.setItem('prism_session_id', newId)
  sessionId.value = newId
  messages.value = []
  menuOpen.value = false
  fetchSessionList()
}

async function loadSessionHistory(sid) {
  try {
    const apiBase = import.meta.env.VITE_API_BASE || ''
    const res = await fetch(`${apiBase}/api/sessions/${sid}`)
    if (res.ok) {
      const history = await res.json()
      messages.value = history.map((item, idx) => ({
        id: idx,
        content: cleanContent(item.content),
        sender: item.role === 'assistant' ? 'bot' : item.role,
        timestamp: item.timestamp || new Date(),
        isStreaming: false,
        reasoning: ''
      }))
      await nextTick()
      forceScrollToBottom()
    }
  } catch (e) {
    console.error('加载会话历史失败', e)
  }
}

async function switchSession(id) {
  if (id === sessionId.value) return
  sessionId.value = id
  localStorage.setItem('prism_session_id', id)
  messages.value = []
  await loadSessionHistory(id)
  menuOpen.value = false
}

// ==================== 三点菜单 ====================
const activeMenuId = ref(null)
const menuPosition = ref({ top: 0, left: 0 })
const currentMenuSession = ref(null)

function toggleSessionMenu(event, id) {
  if (activeMenuId.value === id) {
    activeMenuId.value = null
    return
  }
  const btn = event.currentTarget
  const rect = btn.getBoundingClientRect()
  menuPosition.value = {
    top: rect.bottom - 20,
    left: rect.left + 50
  }
  activeMenuId.value = id
  currentMenuSession.value = sessionList.value.find(s => s.id === id)
}

// ==================== 重命名（自定义弹窗） ====================
const renameTarget = ref(null)
const renameInputValue = ref('')

function openRenameDialog(sess) {
  renameTarget.value = sess
  renameInputValue.value = sess.title || ''
  activeMenuId.value = null // 关闭三点菜单
}

function confirmRename() {
  if (!renameTarget.value) return
  const newTitle = renameInputValue.value.trim()
  if (newTitle) {
    renameTarget.value.title = newTitle
    // 可以在这里调用后端更新接口，目前先更新本地数据并刷新列表
    fetchSessionList()
  }
  renameTarget.value = null
}

function cancelRename() {
  renameTarget.value = null
}

// ==================== 删除确认（自定义弹窗） ====================
const confirmDeleteId = ref(null)
const confirmDeleteTitle = ref('')

function requestDelete(id, title) {
  confirmDeleteId.value = id
  confirmDeleteTitle.value = title || '该会话'
  activeMenuId.value = null
}

function confirmDelete() {
  if (!confirmDeleteId.value) return
  const deletedId = confirmDeleteId.value
  sessionList.value = sessionList.value.filter(s => s.id !== deletedId)

  // 如果删除的是当前活动会话，切换到第一个会话（如果存在），否则创建新会话
  if (deletedId === sessionId.value) {
    if (sessionList.value.length > 0) {
      switchSession(sessionList.value[0].id)
    } else {
      createNewSession()
    }
  }
  confirmDeleteId.value = null
}

function cancelDelete() {
  confirmDeleteId.value = null
}

// ==================== 全局点击关闭菜单 ====================
function handleGlobalClick() {
  activeMenuId.value = null
}

// ==================== 初始化 ====================
onMounted(() => {
  fetchSessionList()
  document.addEventListener('click', handleGlobalClick)
})
</script>
<style scoped>
@import '../../../styles/shanxi/chat-window.css';

.input-model-btn {
  left: 44px; /* 放在上传按钮右边 */
}

.model-menu {
  position: absolute;
  bottom: 50px;
  left: 44px;
  background: #fff;
  border: 1px solid #e4dfd4;
  border-radius: 12px;
  box-shadow: 0 4px 16px rgba(0,0,0,0.08);
  z-index: 100;
  min-width: 140px;
  overflow: hidden;
}

.model-option {
  display: flex;
  align-items: center;
  padding: 10px 16px;
  cursor: pointer;
  font-size: 13px;
  color: #1b1a18;
  transition: background 0.15s;
}

.model-option:hover {
  background: #f2ede3;
}

.model-option.active {
  background: #e8e3d8;
  font-weight: 600;
}
</style>
<style>
@import './chat-global.css';
</style>