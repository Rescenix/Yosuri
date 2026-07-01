<template>
  <div>
    <div class="chat-toggle-button" v-if="!isOpen" @click="toggleChat">
      <Icon icon="mdi:chat" width="28" color="white" />
    </div>

    <div v-if="isOpen && isExpanded" class="chat-overlay" @click="toggleExpand"></div>

    <div class="chat-window" :class="{ expanded: isExpanded }" :style="{ display: isOpen ? 'flex' : 'none' }">
      
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
            
            <!-- Professional：项目/子任务面包屑 + 折叠选择器 -->
            <div v-if="uiMode === 'pro'" class="project-breadcrumb" @click="showTaskDropdown = !showTaskDropdown">
              <span class="project-current">{{ currentProject?.name || '选择项目' }}</span>
              <span v-if="currentTask" class="breadcrumb-separator">→</span>
              <span v-if="currentTask" class="task-current">{{ currentTask.name }}</span>
              <Icon icon="mdi:chevron-down" width="16" color="#696259" style="margin-left:6px" />
            </div>

            <!-- AIStudio：当前会话名（纯文本，无下拉） -->
            <div v-else class="studio-breadcrumb">
              <span class="breadcrumb-separator">›</span>
              <span class="studio-session-name">{{ activeSessionObj?.name || '未命名会话' }}</span>
            </div>

            <!-- 折叠列表（下拉） -->
            <div v-if="uiMode === 'pro' && showTaskDropdown" class="task-dropdown" @click.stop>
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
          <!-- 右侧：模式切换器 + 展开按钮 -->
          <div class="header-right">
            <div class="mode-switch">
              <div class="mode-seg" :class="{ active: uiMode === 'pro' }" @click="setUiMode('pro')">Professional</div>
              <div class="mode-seg" :class="{ active: uiMode === 'studio' }" @click="setUiMode('studio')">AIStudio</div>
            </div>
            <button class="header-expand-btn" @click="toggleExpand" :title="isExpanded ? '退出工作区' : '展开工作区'">
              <Icon :icon="isExpanded ? 'mdi:fullscreen-exit' : 'mdi:fullscreen'" width="18" color="#696259" />
            </button>
          </div>
        </div>
    <div class="chat-body" :class="uiMode">
    <!-- Professional 左：文件树手风琴 -->
    <FileTreePanel
      v-if="isExpanded && uiMode === 'pro'"
      :project-name="currentProject?.name || ''"
      :files="fileTree"
      :selected="selectedFile"
      :pinned-files="pinnedFiles"
      @select="onFileSelect"
      @toggle="onToggleFolder"
      @unpin-file="handleUnpinFile"
      @refresh-tree="fetchFileTree"
    />
    <!-- Professional 中：编辑器 + 终端 -->
    <div class="editor-column" v-if="isExpanded && uiMode === 'pro'">
      <CodeEditor
        :tabs="editorTabs"
        :activeFilePath="activeEditorFile"
        :fileContent="fileContent"
        :language="editorLanguage"
        :pinned-paths="pinnedFiles.map(f => f.path)"
        @update:content="val => fileContent = val"
        @switch-file="activeEditorFile = $event"
        @close-file="closeEditorTab"
        @pin-file="handlePinFile"
        @unpin-file="handleUnpinFile"
      />
      <Terminal v-model:open="terminalOpen" :height="terminalHeight" />
    </div>
    <!-- AIStudio 左：会话列表 -->
    <SessionList
      v-if="isExpanded && uiMode === 'studio'"
      :sessions="sessionList"
      :active-session="activeSession"
      @select="selectSession"
      @new-session="newSession"
    />
    <!-- 共享聊天列（Professional 右栏 / AIStudio 中栏时间线） -->
    <div class="chat-content" :class="{ studio: isExpanded && uiMode === 'studio' }">
      <!-- AIStudio 中栏头部：会话名 + 分支 + 状态 + 模型 -->
      <div v-if="isExpanded && uiMode === 'studio'" class="studio-chat-header">
        <span class="sch-name">{{ activeSessionObj?.name || '未命名会话' }}</span>
        <span class="sch-branch">{{ activeSessionObj?.branch || 'main' }}</span>
        <span class="sch-status">
          <span class="sch-dot" :class="'status-' + (activeSessionObj?.status || 'idle')"></span>
          {{ { running: '运行中', done: '已完成', idle: '空闲' }[activeSessionObj?.status] || '空闲' }}
        </span>
        <div class="sch-spacer"></div>
        <div class="sch-model" @click.stop="showModelMenu = !showModelMenu">
          <span>{{ modelOptions.find(m => m.value === selectedModel)?.label || '模型' }}</span>
          <span class="sch-caret">▾</span>
          <div v-if="showModelMenu" class="model-menu-dropdown sch-model-menu" @click.stop>
            <div
              v-for="m in modelOptions"
              :key="m.value"
              class="model-menu-item"
              :class="{ active: selectedModel === m.value }"
              @click="selectModel(m.value)"
            >
              <Icon :icon="m.icon" width="16" style="margin-right:8px" />
              {{ m.label }}
            </div>
          </div>
        </div>
      </div>
      
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
                <!-- Agent 步骤组（AIStudio 展开：可折叠步骤卡；Professional：降级为工具药丸） -->
                <div v-else-if="item.kind === 'group' && isExpanded && uiMode === 'studio'" class="agent-group">
                  <div class="agent-group-summary" @click="toggleGroup(item.id)">
                    <span>{{ item.text }}</span>
                    <span class="agent-group-chev" :class="{ open: expandedGroups[item.id] }">›</span>
                  </div>
                  <div v-if="expandedGroups[item.id]" class="agent-group-card">
                    <div
                      v-for="(it, i) in item.items"
                      :key="i"
                      class="agent-group-item"
                      :class="{ jump: item.gtype === 'edits' }"
                      @click="item.gtype === 'edits' && openDiffFile(it.name)"
                    >
                      <span class="agv-verb">{{ it.verb }}</span>
                      <span class="agv-name">{{ it.name }}</span>
                      <template v-if="it.adds !== undefined">
                        <span class="agv-add">{{ it.adds }}</span>
                        <span class="agv-del">{{ it.dels }}</span>
                      </template>
                      <span v-else-if="it.meta" class="agv-meta">{{ it.meta }}</span>
                    </div>
                  </div>
                </div>
                <div v-else-if="item.kind === 'group'" class="tool-call-indicator degraded">
                  <Icon icon="mdi:cog-sync" width="14" color="#6b7280" />
                  <span>{{ item.text }}</span>
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
  <!-- 模型切换按钮 -->
  <button 
    v-if="isLoggedIn" 
    class="input-inner-btn input-model-btn" 
    @click.stop="showModelMenu = !showModelMenu" 
    title="切换模型"
  >
    <Icon :icon="currentModelIcon" width="18" color="#888" />
  </button>

  <!-- ★★★ 新添加的下拉菜单 ★★★ -->
  <div v-if="showModelMenu" class="model-menu-dropdown" @click.stop>
    <div 
      v-for="m in modelOptions" 
      :key="m.value"
      class="model-menu-item"
      :class="{ active: selectedModel === m.value }"
      @click="selectModel(m.value)"
    >
      <Icon :icon="m.icon" width="16" style="margin-right:8px" />
      {{ m.label }}
    </div>
  </div>

  <!-- 核心输入框 -->
  <textarea
    ref="chatInputRef"
    class="chat-input"
    v-model="userInput"
    placeholder="输入你的问题..."
    @keypress.enter="sendMessage"
    @input="adjustInputHeight"
    rows="1"
  ></textarea>

  <!-- 发送按钮 -->
  <button v-if="userInput.trim()" class="input-inner-btn input-right-btn input-send-btn" @click="sendMessage">
    <Icon icon="heroicons:paper-airplane-20-solid" width="18" color="#fff" />
  </button>
</div>
          </div>
   </div>

    <!-- AIStudio 右：工具面板 Diff / Terminal -->
    <aside class="tool-panel" v-if="isExpanded && uiMode === 'studio'">
      <div class="tool-panel-tabs">
        <div class="tool-tab" :class="{ active: toolTab === 'diff' }" @click="setToolTab('diff')">Diff</div>
        <div class="tool-tab" :class="{ active: toolTab === 'terminal' }" @click="setToolTab('terminal')">Terminal</div>
        <div class="tool-tabs-spacer"></div>
        <span class="tool-panel-meta">{{ toolTab === 'diff' ? '3 files' : 'node' }}</span>
      </div>
      <DiffPanel
        v-if="toolTab === 'diff'"
        :files="diffFiles"
        :expanded-diffs="expandedDiffs"
        :totals="diffTotals"
        @toggle-file="toggleDiffFile"
      />
      <Terminal v-else class="tool-panel-terminal" :open="true" :embedded="true" />
    </aside>
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
import CodeEditor from './CodeEditor.vue'
import Terminal from './Terminal.vue'
import SessionList from './SessionList.vue'
import DiffPanel from './DiffPanel.vue'
const props = defineProps({
  autoOpen: { type: Boolean, default: false },
  sessionId: { type: String, default: 'global_chat_session' }
})

// ==================== AIStudio 模式状态 ====================
// 两种模式共享 messages / selectedModel / 终端 / 列宽，仅呈现不同
const uiMode = ref(localStorage.getItem('prism_ui_mode') || 'studio')  // 'pro' | 'studio'
function setUiMode(mode) {
  uiMode.value = mode
  localStorage.setItem('prism_ui_mode', mode)
}

// 会话列表（演示数据种子，可后续落后端 /api/sessions）
const STATUS_MAP = {
  running: { label: '运行中', color: 'var(--chat-accent, #c96442)', pulse: true },
  done: { label: '已完成', color: '#12b76a', pulse: false },
  idle: { label: '空闲', color: '#a39c8f', pulse: false }
}
const sessionList = ref([
  { id: 'aether', name: 'Aether', desc: '上下文注入层缓存优化', branch: 're0', status: 'running', time: '刚刚' },
  { id: 'prism', name: 'Prism', desc: '文件树懒加载与固定标签', branch: 'main', status: 'done', time: '2 小时前' },
  { id: 'nebula', name: 'Nebula', desc: '终端输出流式渲染', branch: 'dev', status: 'idle', time: '昨天' }
])
const activeSession = ref('aether')
const activeSessionObj = computed(
  () => sessionList.value.find(s => s.id === activeSession.value) || sessionList.value[0] || null
)
function selectSession(id) {
  activeSession.value = id
}
function newSession() {
  const id = 'sess_' + Date.now().toString(36)
  sessionList.value = [
    { id, name: '未命名会话', desc: '等待第一条指令…', branch: 'main', status: 'idle', time: '刚刚' },
    ...sessionList.value
  ]
  activeSession.value = id
}

// 右侧工具面板：Diff / Terminal
const toolTab = ref('diff')
const expandedDiffs = ref({ 'server.js': true })
function setToolTab(t) { toolTab.value = t }
function toggleDiffFile(name) {
  expandedDiffs.value = { ...expandedDiffs.value, [name]: !expandedDiffs.value[name] }
}
function openDiffFile(name) {
  toolTab.value = 'diff'
  expandedDiffs.value = { ...expandedDiffs.value, [name]: true }
}
// Diff 演示数据（占位条，实施接真实 /api/git-diff 时保留此结构）
const diffFiles = ref([
  { name: 'server.js', adds: '+18', dels: '-4', rows: [
    { gap: '12 行未修改' },
    { n: 41, t: 'ctx', w: 62 }, { n: 42, t: 'ctx', w: 48 },
    { n: 43, t: 'del', w: 74 }, { n: 44, t: 'del', w: 58 },
    { n: 43, t: 'add', w: 80 }, { n: 44, t: 'add', w: 66 }, { n: 45, t: 'add', w: 52 },
    { n: 46, t: 'ctx', w: 44 },
    { gap: '36 行未修改' },
    { n: 83, t: 'ctx', w: 58 }, { n: 84, t: 'add', w: 70 }, { n: 85, t: 'add', w: 46 }, { n: 86, t: 'ctx', w: 62 }
  ] },
  { name: 'query.json', adds: '+2', dels: '-2', rows: [
    { n: 3, t: 'ctx', w: 52 }, { n: 4, t: 'del', w: 64 }, { n: 4, t: 'add', w: 70 }, { n: 5, t: 'ctx', w: 40 },
    { gap: '3 行未修改' },
    { n: 9, t: 'del', w: 56 }, { n: 9, t: 'add', w: 62 }, { n: 10, t: 'ctx', w: 46 }
  ] },
  { name: 'verify_go_query.py', adds: '+9', dels: '-1', rows: [
    { gap: '18 行未修改' },
    { n: 22, t: 'ctx', w: 54 }, { n: 23, t: 'del', w: 48 }, { n: 23, t: 'add', w: 76 }, { n: 24, t: 'add', w: 60 }, { n: 25, t: 'add', w: 68 }, { n: 26, t: 'ctx', w: 42 },
    { gap: '40 行未修改' }
  ] }
])
const diffTotals = '+29 −7'

// Agent 时间线步骤组折叠态
const expandedGroups = ref({})
function toggleGroup(id) {
  expandedGroups.value = { ...expandedGroups.value, [id]: !expandedGroups.value[id] }
}


const viewingFile = ref(null)    // 当前查看的文件对象
const fileContent = ref('')
const fileLoading = ref(false)
const editorTabs = ref([])              // 已打开的文件标签
const activeEditorFile = ref(null)      // 当前编辑的文件路径
const editorContent = ref('')
const editorLanguage = ref('text')
let monacoEditor = null                 // Monaco 实例引用

const pinnedFiles = ref([])             // 已固定到侧边栏的文件 { name, path }
const terminalOpen = ref(true)
const terminalHeight = ref(180)

function handlePinFile(tab) {
  if (!pinnedFiles.value.find(f => f.path === tab.path)) {
    pinnedFiles.value = [...pinnedFiles.value, { name: tab.name, path: tab.path }]
  }
}
function handleUnpinFile(f) {
  pinnedFiles.value = pinnedFiles.value.filter(x => x.path !== f.path)
}


function closeEditorTab(path) {
  editorTabs.value = editorTabs.value.filter(t => t.path !== path)
  if (activeEditorFile.value === path) {
    activeEditorFile.value = editorTabs.value[0]?.path || null
  }
}
function onEditorMounted(editor) {
  monacoEditor = editor
}
async function onFileSelect(file) {
  // 如果已经打开了同一个文件，不重复加载
  if (activeEditorFile.value === file.path) return

  selectedFile.value = file
  activeEditorFile.value = file.path
  fileContent.value = ''

  // 自动判断语言
  const ext = file.name.split('.').pop()
  const langMap = { js: 'javascript', ts: 'typescript', vue: 'html', go: 'go', py: 'python', css: 'css', html: 'html', json: 'json', md: 'markdown' }
  editorLanguage.value = langMap[ext] || 'text'

  // 加入标签页
  const existing = editorTabs.value.find(t => t.path === file.path)
  if (!existing) {
    editorTabs.value.push({ name: file.name, path: file.path })
  }

  // 从后端读取真实文件内容
  try {
    const res = await fetch(`/api/file?path=${encodeURIComponent(file.path)}`)
    if (res.ok) {
      fileContent.value = await res.text()
    } else {
      fileContent.value = `// 无法加载文件: ${res.status}`
    }
  } catch (e) {
    fileContent.value = `// 网络错误: ${e.message}`
  }
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
const fileTree = ref([])

async function fetchFileTree() {
  try {
    const res = await fetch('/api/file-tree')
    if (res.ok) {
      fileTree.value = await res.json()
    }
  } catch (e) {
    console.error('文件树加载失败', e)
  }
}

onMounted(() => {
  fetchFileTree()
   document.addEventListener('click', () => {
    showTaskDropdown.value = false;
    showModelMenu.value = false; // 加上这一行
  });
})
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
  isOpen, isExpanded, userInput, messages, sessionId,
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
/* ==================== 输入框容器（彻底改成 Flex 布局） ==================== */
/* ==================== 输入框容器 ==================== */
.input-wrapper {
  display: flex;
  align-items: center;
  gap: 4px;                  /* 保留极小的间隙，让图标和文字有呼吸感 */
  padding: 6px 12px;         /* 整体内边距 */
  border: 1px solid #d4cfc4;
  border-radius: 24px;
  background: #ffffff;
  width: 100%;
  box-sizing: border-box;
  position: relative;
  transition: border-color 0.2s;
}
.input-wrapper:focus-within {
  border-color: #c96442;
}

/* ==================== 按钮图标 ==================== */
.input-inner-btn {
  margin-left: -5%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  border: none;
  cursor: pointer;
  flex-shrink: 0;
  width: 28px;
  height: 28px;
  padding: 0;
  color: #888;
  border-radius: 50%;
  transition: background 0.2s;
  z-index: 10;               /* 确保按钮在输入框之上 */
}
.input-inner-btn:hover {
  background: rgba(0, 0, 0, 0.05);
}

/* 发送按钮 */
.input-send-btn {
  width: 32px;
  height: 32px;
  background: #c96442;
  color: #fff;
}
.input-send-btn:hover {
  background: #b85737;
}

/* ==================== 核心文本输入 ==================== */
.chat-input {
  margin-left: 1%;
  flex: 1;
  border: none;
  outline: none;
  background: transparent;
  padding: 15px 0 !important;  /* ✅ 左侧 padding 归零，占位符紧贴图标 */
  resize: none;
  min-height: 44px;
  max-height: 150px;
  line-height: 24px;
  font-size: 15px;
  color: #1b1a18;
  font-family: inherit;
  box-sizing: border-box;
  overflow-y: hidden;
  -ms-overflow-style: none;
  scrollbar-width: none;
  position: relative;
  z-index: 1;                 /* 输入框在底层，让 z-index:10 的按钮可以点击 */
}
.chat-input::-webkit-scrollbar {
  display: none;
}
.chat-input::placeholder {
  color: #a9a9a9;
  font-style: italic;
}

/* ==================== 模型下拉菜单 ==================== */
.model-menu-dropdown {
  position: absolute;
  bottom: 56px;         /* 定位到输入框上方 */
  left: 6px;            /* 与地球图标左对齐 */
  background: #ffffff;
  border: 1px solid #d4cfc4;
  border-radius: 12px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
  padding: 6px 0;
  min-width: 140px;
  z-index: 9999;        /* 必须浮在最上层 */
}

.model-menu-item {
  display: flex;
  align-items: center;
  padding: 8px 16px;
  font-size: 14px;
  cursor: pointer;
  transition: background 0.15s;
}
.model-menu-item:hover {
  background: #f5f2ec;
}
.model-menu-item.active {
  background: #f0e4d7;
  font-weight: 500;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 8px;
}
.header-expand-btn {
  border: none;
  background: transparent;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 6px;
}
.header-expand-btn:hover { background: rgba(0,0,0,0.05); }

/* 模式切换胶囊 [Professional | AIStudio] */
.mode-switch {
  display: flex;
  align-items: center;
  gap: 2px;
  background: #f4f2ec;
  border: 1px solid #e4dfd4;
  border-radius: 999px;
  padding: 2px;
}
.mode-seg {
  padding: 4px 12px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 500;
  color: #a39c8f;
  cursor: pointer;
  white-space: nowrap;
  transition: background 0.15s ease, color 0.15s ease;
}
.mode-seg.active {
  background: #c96442;
  color: #fff;
  font-weight: 600;
}

/* AIStudio 面包屑（纯文本会话名） */
.studio-breadcrumb {
  display: flex;
  align-items: center;
  gap: 6px;
}
.studio-breadcrumb .breadcrumb-separator { color: #a39c8f; }
.studio-session-name {
  font-size: 13px;
  font-weight: 500;
  color: #696259;
}

/* ==================== IDE 工作区三栏几何（仅全屏展开时生效） ==================== */
/* 中间列：编辑器(flex:1) + 终端(卡在编辑区宽度下方，不横跨文件树/聊天) */
.editor-column {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
</style>

<style>
@import './chat-global.css';

/* ===== Professional：聊天区 = 固定宽度全高右栏，把 flex:1 让给中间编辑器列 ===== */
.chat-window.expanded .chat-body.pro .chat-content {
  flex: 0 0 380px !important;
  width: 380px !important;
  max-width: 380px !important;
  border-left: 1px solid #e4dfd4;
}

/* ===== AIStudio：聊天区 = 中栏 flex:1，会话列表在左、工具面板在右 ===== */
.chat-window.expanded .chat-body.studio .chat-content {
  flex: 1 !important;
  width: auto !important;
  max-width: none !important;
  min-width: 0;
  border-right: 1px solid #e4dfd4;
  background: #ffffff;
}

/* 会话列表左栏（宽度固定，可后续接拖拽） */
.chat-window.expanded .chat-body.studio > .session-panel {
  width: 260px;
  flex-shrink: 0;
  border-right: 1px solid #e4dfd4;
}

/* 右侧工具面板 */
.chat-window.expanded .tool-panel {
  width: 380px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
  background: #faf9f6;
}
.chat-window.expanded .tool-panel-tabs {
  height: 48px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 0 12px;
  border-bottom: 1px solid #e4dfd4;
}
.chat-window.expanded .tool-tab {
  padding: 5px 14px;
  border-radius: 7px;
  font-size: 12px;
  font-weight: 600;
  color: #a39c8f;
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
}
.chat-window.expanded .tool-tab.active { background: #e8e3d8; color: #1b1a18; }
.chat-window.expanded .tool-tabs-spacer { flex: 1; }
.chat-window.expanded .tool-panel-meta {
  font-size: 10.5px;
  color: #a39c8f;
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
}
.chat-window.expanded .tool-panel-terminal { border-radius: 0; }

/* AIStudio 中栏头部 */
.chat-window.expanded .studio-chat-header {
  height: 48px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 18px;
  border-bottom: 1px solid #e4dfd4;
  position: relative;
}
.chat-window.expanded .studio-chat-header .sch-name { font-size: 13.5px; font-weight: 700; color: #1b1a18; }
.chat-window.expanded .studio-chat-header .sch-branch {
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
  font-size: 10.5px;
  padding: 2px 9px;
  border-radius: 999px;
  border: 1px solid #e4dfd4;
  color: #696259;
  background: #f4f2ec;
}
.chat-window.expanded .studio-chat-header .sch-status {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 11.5px;
  color: #696259;
}
.chat-window.expanded .studio-chat-header .sch-dot { width: 7px; height: 7px; border-radius: 50%; }
.chat-window.expanded .studio-chat-header .sch-dot.status-running { background: #c96442; animation: sess-pulse 1.6s ease-in-out infinite; }
.chat-window.expanded .studio-chat-header .sch-dot.status-done { background: #12b76a; }
.chat-window.expanded .studio-chat-header .sch-dot.status-idle { background: #a39c8f; }
.chat-window.expanded .studio-chat-header .sch-spacer { flex: 1; }
.chat-window.expanded .studio-chat-header .sch-model {
  position: relative;
  display: flex;
  align-items: center;
  gap: 6px;
  background: #f4f2ec;
  border-radius: 999px;
  padding: 5px 12px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  color: #1b1a18;
}
.chat-window.expanded .studio-chat-header .sch-caret { font-size: 9px; color: #a39c8f; }
.chat-window.expanded .studio-chat-header .sch-model-menu {
  position: absolute;
  top: 100%;
  right: 0;
  left: auto;
  bottom: auto;
  margin-top: 6px;
}
@keyframes sess-pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.5; transform: scale(1.25); }
}

/* AIStudio 中栏：消息流 + 输入框收在 720px 阅读列内居中 */
.chat-window.expanded .chat-body.studio .chat-content .chat-messages {
  max-width: 720px;
  width: 100%;
  margin: 0 auto;
  padding: 22px 24px;
  box-sizing: border-box;
}
.chat-window.expanded .chat-body.studio .chat-content .input-wrapper {
  max-width: 720px;
  margin: 0 auto;
}

/* Agent 时间线步骤组 */
.chat-window.expanded .agent-group { width: 100%; }
.chat-window.expanded .agent-group-summary {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  cursor: pointer;
  font-size: 13px;
  color: #696259;
  padding: 2px 0;
  transition: color 0.15s ease;
}
.chat-window.expanded .agent-group-summary:hover { color: #1b1a18; }
.chat-window.expanded .agent-group-chev {
  display: inline-block;
  font-size: 12px;
  color: #a39c8f;
  transition: transform 0.15s ease;
}
.chat-window.expanded .agent-group-chev.open { transform: rotate(90deg); }
.chat-window.expanded .agent-group-card {
  margin-top: 8px;
  border: 1px solid #e4dfd4;
  border-radius: 10px;
  background: #f4f2ec;
  padding: 10px 14px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.chat-window.expanded .agent-group-item {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12.5px;
}
.chat-window.expanded .agent-group-item.jump { cursor: pointer; }
.chat-window.expanded .agent-group-item .agv-verb { color: #696259; flex-shrink: 0; }
.chat-window.expanded .agent-group-item .agv-name {
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
  font-size: 12px;
  font-weight: 600;
  color: #1b1a18;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.chat-window.expanded .agent-group-item .agv-add,
.chat-window.expanded .agent-group-item .agv-del {
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
  font-size: 11.5px;
  font-weight: 600;
  flex-shrink: 0;
}
.chat-window.expanded .agent-group-item .agv-add { color: #12b76a; }
.chat-window.expanded .agent-group-item .agv-del { color: #d94834; }
.chat-window.expanded .agent-group-item .agv-meta {
  font-size: 11.5px;
  color: #a39c8f;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>