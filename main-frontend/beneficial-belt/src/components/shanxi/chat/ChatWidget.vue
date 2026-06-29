<template>
  <div>
    <div class="chat-toggle-button" v-if="!isOpen" @click="toggleChat">
      <Icon icon="mdi:chat" width="28" color="white" />
    </div>

    <div v-if="isOpen && isExpanded" class="chat-overlay" @click="toggleExpand"></div>

    <div class="chat-window" :class="{ expanded: isExpanded, mobile: isMobile }" :style="{ display: isOpen ? 'flex' : 'none' }">
      
      <!-- ★ 左侧折叠菜单（侧边抽屉） —— 改为项目任务列表 -->
      <div v-if="menuOpen" class="drawer-backdrop" @click="menuOpen = false"></div>
      <aside class="drawer-panel" :class="{ open: menuOpen }">
        <h4 class="site-icon">
          <Icon icon="majesticons:shooting-star-line" width="20" />
        </h4>

        <!-- 静态导航链接保留（项目库、研习录等） -->
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

        <div class="drawer-divider"></div>

        <!-- ★★★ 项目任务列表 (替代原来的会话列表) ★★★ -->
        <div class="project-task-tree">
          <div class="tree-header">
            <Icon icon="mdi:file-tree" width="16" color="#696259" />
            <span>项目任务</span>
          </div>
          <div class="tree-content">
            <div
              v-for="proj in projects"
              :key="proj.id"
              class="project-node"
            >
              <div
                class="project-item"
                :class="{ active: currentProject?.id === proj.id }"
                @click="selectProject(proj)"
              >
                <Icon
                  :icon="proj.expanded ? 'mdi:chevron-down' : 'mdi:chevron-right'"
                  width="14"
                  color="#8b847a"
                  class="expand-icon"
                  @click.stop="proj.expanded = !proj.expanded"
                />
                <Icon icon="mdi:folder-outline" width="16" color="#f59e0b" style="margin-right:6px" />
                <span class="project-name">{{ proj.name }}</span>
              </div>
              <!-- 子任务列表 -->
              <div v-if="proj.expanded" class="subtask-list">
                <div
                  v-for="task in proj.tasks"
                  :key="task.id"
                  class="subtask-item"
                  :class="{ active: currentTask?.id === task.id && currentProject?.id === proj.id }"
                  @click="selectTask(proj, task)"
                >
                  <Icon icon="mdi:checkbox-blank-circle-outline" width="10" color="#a0a0a0" style="margin-right:8px" />
                  <span>{{ task.name }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 移除的：新建对话按钮、会话列表、三点菜单、重命名/删除弹窗 -->
        <!-- 相关 JS 逻辑也一并移除（见 script） -->
      </aside>

      <!-- ★ 主内容区（随菜单推开） -->
      <div class="chat-main" :class="{ shifted: menuOpen }">
        <!-- ★★★ 顶部栏改造：项目名 → 子任务名 ★★★ -->
        <div class="chat-header">
          <div class="header-left">
            <button class="header-menu-btn" @click="menuOpen = !menuOpen" aria-label="展开导航">
              <Icon icon="mdi:menu" width="18" color="#696259" />
            </button>
            
            <!-- 新：项目/子任务面包屑 + 折叠选择器 -->
            <div class="project-breadcrumb" @click="showTaskDropdown = !showTaskDropdown">
              <span class="project-current">{{ currentProject?.name || '选择项目' }}</span>
              <span v-if="currentTask" class="breadcrumb-separator">→</span>
              <span v-if="currentTask" class="task-current">{{ currentTask.name }}</span>
              <Icon icon="mdi:chevron-down" width="16" color="#696259" style="margin-left:6px" />
            </div>
            
            <!-- 折叠列表（下拉） -->
            <div v-if="showTaskDropdown" class="task-dropdown" @click.stop>
              <div v-for="proj in projects" :key="proj.id" class="dropdown-project">
                <div class="dropdown-project-name" @click="selectProject(proj); showTaskDropdown = false">
                  <Icon icon="mdi:folder-outline" width="14" color="#f59e0b" style="margin-right:4px" />
                  {{ proj.name }}
                </div>
                <div
                  v-for="task in proj.tasks"
                  :key="task.id"
                  class="dropdown-task"
                  :class="{ active: currentTask?.id === task.id && currentProject?.id === proj.id }"
                  @click="selectTask(proj, task); showTaskDropdown = false"
                >
                  {{ task.name }}
                </div>
              </div>
            </div>
          </div>
          <!-- 右侧可以保留原状态指示的位置，但不再显示杉汐状态 -->
          <div class="header-right">
            <!-- 可放置其他操作，当前留空 -->
          </div>
        </div>
    <div class="chat-body">
    <FileTreePanel
      :project-name="currentProject?.name || ''"
      :files="fileTree"
      :selected="selectedFile"
      @select="onFileSelect"
      @toggle="onToggleFolder"
    />
    <div class="chat-content">
      
          <!-- 其余内容保持不变：滚动按钮、消息列表、欢迎语、推理链等 -->
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
  <button v-if="isLoggedIn" class="input-inner-btn input-model-btn" @click.stop="showModelMenu = !showModelMenu" title="切换模型">
    <Icon :icon="currentModelIcon" width="18" color="#888" />
  </button>
  
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


<GitPanel
  v-if="showGitPanel && !selectedFile"
  @ai-commit="onAiCommit"
/>
<CodePreviewPanel
  v-else-if="selectedFile"
  :selected-file="selectedFile"
  :file-content="fileContent"
  :loading="fileLoading"
  @close="selectedFile = null"
/>


      </div>
      
    </div>
  </div></div>
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
import FileTreePanel from './FileTreePanel.vue'
import CodePreviewPanel from './CodePreviewPanel.vue'
import GitPanel from './GitPanel.vue'
const props = defineProps({
  autoOpen: { type: Boolean, default: false },
  sessionId: { type: String, default: 'global_chat_session' }
})


const viewingFile = ref(null)    // 当前查看的文件对象
const showGitPanel = ref(false)
const fileContent = ref('')
const fileLoading = ref(false)

async function onFileSelect(file) {
  selectedFile.value = file
  fileLoading.value = true
  fileContent.value = ''
  // 模拟请求，实际应调用后端接口
  setTimeout(() => {
    fileContent.value = `// 这是 ${file.name} 的模拟内容\n// 实际应从后端获取`
    fileLoading.value = false
  }, 300)
}
// ==================== 新增：项目任务数据结构 ====================
const projects = ref([
  {
    id: 'prismd',
    name: 'PrismD',
    expanded: true,
    tasks: [
      { id: 't1', name: 'P级任务收尾' },
      { id: 't2', name: '上下文注入层' },
      { id: 't3', name: '前端可视化重构' },
      { id: 't4', name: '系统稳定性' }
    ]
  },
  {
    id: 'shanxi',
    name: '杉汐 Cloud',
    expanded: false,
    tasks: [
      { id: 't5', name: 'API 封装' },
      { id: 't6', name: '计费系统' },
      { id: 't7', name: '用户面板' }
    ]
  },
  {
    id: 'playwright',
    name: 'DS 粮仓攻陷',
    expanded: false,
    tasks: [
      { id: 't8', name: 'PoW 自动求解' },
      { id: 't9', name: '浏览器自动化' },
      { id: 't10', name: '消息轮询代理' }
    ]
  }
])
const fileTree = ref([
  { name: 'src', type: 'folder', expanded: true, children: [
      { name: 'main.go', type: 'file' },
      { name: 'handler.go', type: 'file' }
  ]},
  { name: 'go.mod', type: 'file' }
])
const selectedFile = ref(null)


function onToggleFolder(folder) {
  folder.expanded = !folder.expanded
}
const currentProject = ref(null)
const currentTask = ref(null)
const showTaskDropdown = ref(false)

function selectProject(proj) {
  currentProject.value = proj
  currentTask.value = null
  // 可以在这里触发加载项目上下文等操作
}

function selectTask(proj, task) {
  currentProject.value = proj
  currentTask.value = task
  // 这里后续可以切换对话上下文，加载对应任务的记忆/文件
}

// 默认选中第一个项目和它的第一个任务
onMounted(() => {
  if (projects.value.length > 0) {
    const firstProj = projects.value[0]
    selectProject(firstProj)
    if (firstProj.tasks.length > 0) {
      selectTask(firstProj, firstProj.tasks[0])
    }
  }
  
  document.addEventListener('click', () => {
    showTaskDropdown.value = false
  })
})

// ==================== 工具函数保持不变 ====================
function cleanContent(content) {
  return content ? content.replace(/\[(action|emotion):[^\]]*\]/g, '') : ''
}

const copiedVisible = ref(false)
async function copyText(text) {
  try {
    await navigator.clipboard.writeText(text)
    copiedVisible.value = true
    setTimeout(() => { copiedVisible.value = false }, 2000)
  } catch (err) {
    const textarea = document.createElement('textarea')
    textarea.value = text
    textarea.style.position = 'fixed'
    textarea.style.opacity = '0'
    document.body.appendChild(textarea)
    textarea.select()
    document.execCommand('copy')
    document.body.removeChild(textarea)
    copiedVisible.value = true
    setTimeout(() => { copiedVisible.value = false }, 2000)
  }
}
// ==================== 模型选择 ====================
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

// ==================== Markdown 渲染 (保持不变) ====================
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

const showScrollButton = computed(() => {
  return isOpen.value && userScrolledUp.value
})

watch(messages, () => {
  nextTick(() => { highlightAllCodeBlocks() })
}, { deep: true })

// ==================== 移除所有会话相关逻辑 ====================
// 删除了 fetchSessionList, createNewSession, loadSessionHistory, switchSession
// 删除了三点菜单相关：activeMenuId, menuPosition, currentMenuSession, toggleSessionMenu
// 删除了重命名/删除弹窗逻辑
// 所有会话管理的 JS 代码已清除

// ==================== 初始化 ====================
// onMounted 已在上面定义，注意不要重复
</script>

<style scoped>
@import '../../../styles/shanxi/chat-window.css';

/* 新增：项目任务树样式 */
.project-task-tree {
  margin-top: 8px;
  flex: 1;
  overflow-y: auto;
}

.tree-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  font-size: 12px;
  font-weight: 600;
  color: #696259;
  border-bottom: 1px solid #e4dfd4;
  margin-bottom: 8px;
}

.project-item, .subtask-item {
  display: flex;
  align-items: center;
  padding: 6px 12px;
  cursor: pointer;
  transition: background 0.15s;
  border-radius: 6px;
  margin: 2px 6px;
  font-size: 13px;
  color: #1b1a18;
}

.project-item:hover, .subtask-item:hover {
  background: #f2ede3;
}

.project-item.active, .subtask-item.active {
  background: #e8e3d8;
  font-weight: 600;
}

.expand-icon {
  margin-right: 4px;
}

.subtask-list {
  padding-left: 28px;
}

.subtask-item {
  font-size: 12px;
  color: #4a4540;
}

/* 顶部栏项目面包屑 */
.project-breadcrumb {
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  font-size: 14px;
  font-weight: 500;
  color: #1b1a18;
  padding: 4px 8px;
  border-radius: 6px;
  transition: background 0.15s;
}

.project-breadcrumb:hover {
  background: #f2ede3;
}

.breadcrumb-separator {
  color: #a0a0a0;
  margin: 0 4px;
}

.task-current {
  color: #696259;
  font-weight: 400;
}

/* 任务下拉菜单 */
.task-dropdown {
  position: absolute;
  top: 100%;
  left: 0;
  background: #fff;
  border: 1px solid #e4dfd4;
  border-radius: 10px;
  box-shadow: 0 6px 20px rgba(0,0,0,0.08);
  z-index: 110;
  min-width: 200px;
  padding: 8px 0;
}

.dropdown-project {
  padding: 4px 0;
}

.dropdown-project-name {
  display: flex;
  align-items: center;
  padding: 6px 16px;
  font-size: 13px;
  font-weight: 600;
  color: #1b1a18;
  cursor: pointer;
}

.dropdown-project-name:hover {
  background: #f2ede3;
}

.dropdown-task {
  padding: 6px 16px 6px 44px;
  font-size: 12px;
  color: #4a4540;
  cursor: pointer;
}

.dropdown-task:hover {
  background: #f2ede3;
}

.dropdown-task.active {
  background: #e8e3d8;
  font-weight: 600;
}

/* 确保顶部栏相对定位以容纳下拉 */
.chat-header {
  position: relative;
}

/* 其余样式保持不变 */
.input-model-btn {
  left: 44px;
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
.chat-body { display: flex; flex: 1; overflow: hidden; }

.chat-body {
  display: flex;
  flex: 1;
  overflow: hidden;
}

.chat-content {
  flex: 1;
  width: auto !important;        /* 强制覆盖可能存在的 width:100% */
  max-width: none !important;    /* 移除固定最大宽度 */
  min-width: 0;                  /* 允许收缩 */
  overflow: hidden;
}
</style>

<style>
@import './chat-global.css';
</style>