<template>
  <div class="diff-panel">
    <!-- 顶部：搜索框占据左上角（取代原 branch → working tree 标签），右侧保留增删统计 + 刷新 -->
    <div class="diff-toolbar">
      <div class="diff-search-wrap">
        <Icon icon="mdi:magnify" width="13" color="#a3a3a3" />
        <input
          v-model="searchQuery"
          class="diff-search-input"
          type="text"
          placeholder="搜索文件名定位…"
          spellcheck="false"
        />
        <button v-if="searchQuery" class="diff-search-clear" @click="searchQuery = ''">
          <Icon icon="mdi:close" width="12" />
        </button>
      </div>
      <span class="diff-totals">
        <span class="diff-adds">+{{ totals.add }}</span>
        <span class="diff-dels">−{{ totals.del }}</span>
      </span>
      <button class="diff-refresh-btn" @click="fetchList" title="刷新">
        <Icon icon="mdi:refresh" width="14" :class="{ 'diff-spin': listLoading }" />
      </button>
    </div>

    <div v-if="!listLoading && filteredFiles.length === 0" class="diff-empty">
      <Icon icon="mdi:file-compare" width="24" color="#c4c4c4" />
      <span>{{ files.length === 0 ? '工作树没有未提交改动' : '没有匹配的文件' }}</span>
    </div>

    <div v-else class="diff-body">
      <div class="diff-file-card" v-for="df in filteredFiles" :key="df.path">
        <div
          class="diff-file-head"
          :class="{ 'is-open': !!expanded[df.path] }"
          @click="toggleFile(df)"
          @contextmenu.prevent="openCtxMenu($event, df)"
        >
          <span class="diff-chev" :class="{ open: !!expanded[df.path] }">›</span>
          <span class="diff-status-badge" :class="'st-' + df.status">{{ df.status }}</span>
          <!-- 只显示文件名（basename），完整路径走右键菜单复制 -->
          <span class="diff-file-name" :title="df.path">{{ fileBaseName(df.path) }}</span>
          <span class="diff-adds">+{{ df.additions }}</span>
          <span class="diff-dels">−{{ df.deletions }}</span>
        </div>
        <div v-if="expanded[df.path]" class="diff-rows">
          <div v-if="contentLoading[df.path]" class="diff-file-hint">加载 diff…</div>
          <div v-else-if="contents[df.path]?.binary" class="diff-file-hint">二进制文件，不展示 diff</div>
          <div v-else-if="contents[df.path]?.too_large" class="diff-file-hint">文件过大（&gt;300KB），不展示 diff</div>
          <DiffViewer
            v-else-if="contents[df.path]"
            :old-content="contents[df.path].old_content"
            :new-content="contents[df.path].new_content"
            :path="df.path"
          />
        </div>
      </div>
    </div>

    <!-- 右键悬浮菜单：复制完整路径 -->
    <Teleport to="body">
      <div v-if="ctxMenu.show" class="diff-ctx-backdrop" @click="closeCtxMenu" @contextmenu.prevent="closeCtxMenu"></div>
      <div
        v-if="ctxMenu.show"
        class="diff-ctx-menu"
        :style="{ left: ctxMenu.x + 'px', top: ctxMenu.y + 'px' }"
      >
        <div class="diff-ctx-item" @click="copyFullPath">
          <Icon icon="mdi:content-copy" width="14" />
          <span>{{ ctxCopied ? '已复制' : '复制完整路径' }}</span>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup>
import { ref, computed, reactive, onMounted, onUnmounted } from 'vue'
import { Icon } from '@iconify/vue'
import { fileBaseName } from './toolArgs.js'
import DiffViewer from './DiffViewer.vue'

// git 工作树全量 diff：列表秒出（只有元数据），文件内容点击展开时按需拉取
const branch = ref('')
const files = ref([])
const listLoading = ref(true)
const searchQuery = ref('')
const expanded = reactive({})
const contents = reactive({})
const contentLoading = reactive({})

async function fetchList() {
  listLoading.value = true
  try {
    const res = await fetch('/api/git/working-diff')
    const data = await res.json()
    branch.value = data.branch || ''
    files.value = data.files || []
  } catch {
    files.value = []
  } finally {
    listLoading.value = false
  }
}
onMounted(fetchList)

const filteredFiles = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return files.value
  // 按文件名（basename）搜索，不匹配完整路径
  return files.value.filter(f => fileBaseName(f.path).toLowerCase().includes(q))
})

const totals = computed(() => {
  let add = 0, del = 0
  for (const f of files.value) { add += f.additions; del += f.deletions }
  return { add, del }
})

// ---- 右键复制完整路径 ----
const ctxMenu = reactive({ show: false, x: 0, y: 0, path: '' })
const ctxCopied = ref(false)
function openCtxMenu(e, df) {
  ctxCopied.value = false
  // 贴边处理：菜单宽约 180px、高约 40px，靠右/靠下时向内翻折，避免溢出视口
  const mw = 180, mh = 40
  ctxMenu.x = Math.min(e.clientX, window.innerWidth - mw - 8)
  ctxMenu.y = Math.min(e.clientY, window.innerHeight - mh - 8)
  ctxMenu.path = df.path
  ctxMenu.show = true
}
function closeCtxMenu() { ctxMenu.show = false }
async function copyFullPath() {
  const text = ctxMenu.path
  try {
    await navigator.clipboard.writeText(text)
  } catch {
    const ta = document.createElement('textarea')
    ta.value = text; ta.style.position = 'fixed'; ta.style.opacity = '0'
    document.body.appendChild(ta); ta.select(); document.execCommand('copy'); document.body.removeChild(ta)
  }
  ctxCopied.value = true
  setTimeout(closeCtxMenu, 700)
}
function onEscClose(e) { if (e.key === 'Escape') closeCtxMenu() }
onMounted(() => document.addEventListener('keydown', onEscClose))
onUnmounted(() => document.removeEventListener('keydown', onEscClose))

async function toggleFile(df) {
  expanded[df.path] = !expanded[df.path]
  if (!expanded[df.path] || contents[df.path] || df.binary) return
  contentLoading[df.path] = true
  try {
    const res = await fetch(`/api/git/working-diff/file?path=${encodeURIComponent(df.path)}`)
    contents[df.path] = await res.json()
  } catch {
    contents[df.path] = { old_content: '', new_content: '', binary: false, too_large: false }
  } finally {
    contentLoading[df.path] = false
  }
}
</script>

<style scoped>
.diff-panel {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
}

/* ---------- 顶部工具栏：搜索框占左、统计+刷新靠右，单行 ---------- */
.diff-toolbar {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border-bottom: 1px solid var(--app-border-soft);
}
.diff-totals { display: flex; gap: 6px; flex-shrink: 0; }
.diff-refresh-btn {
  display: inline-flex;
  align-items: center;
  border: none;
  background: transparent;
  color: var(--app-text-faint);
  cursor: pointer;
  border-radius: 5px;
  padding: 2px;
}
.diff-refresh-btn:hover { background: #f0f0f0; }
.diff-spin { animation: diff-rotate 0.9s linear infinite; }
@keyframes diff-rotate { from { transform: rotate(0); } to { transform: rotate(360deg); } }

.diff-search-wrap {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 5px;
  background: var(--app-surface);
  border: 1px solid var(--app-border);
  border-radius: 7px;
  padding: 4px 8px;
}
.diff-search-input {
  flex: 1;
  min-width: 0;
  border: none;
  outline: none;
  background: transparent;
  font-size: 12px;
  color: var(--app-text);
}
.diff-search-clear {
  display: inline-flex;
  border: none;
  background: transparent;
  color: var(--app-text-faint);
  cursor: pointer;
  padding: 1px;
}

.diff-empty {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: var(--app-text-faint);
  font-size: 12.5px;
}

.diff-body {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  min-height: 0;
  padding: 0 0 12px;
}

/* 仿 VS Code / Claude Code：紧密矩形行列表，不是带间距的圆角卡片 */
.diff-file-card {
  background: transparent;
  /* 不能用 overflow:hidden——它会成为 sticky 的滚动容器、令下面的吸顶失效 */
}
.diff-file-head {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 5px 12px;
  cursor: pointer;
  background: var(--app-surface);
  /* 展开后滚动时，当前文件的文件名 + 增删栏吸附到滚动区顶部 */
  position: sticky;
  top: 0;
  z-index: 2;
}
.diff-file-head:hover { background: #f6f7f9; }
.diff-file-head.is-open {
  background: #f4f6f8;
  border-bottom: 1px solid var(--app-border-soft);
}
.diff-chev {
  display: inline-block;
  font-size: 12px;
  color: var(--app-text-faint);
  transition: transform 0.15s ease;
}
.diff-chev.open { transform: rotate(90deg); }
.diff-status-badge {
  flex-shrink: 0;
  width: 16px;
  height: 16px;
  border-radius: 4px;
  font-size: 10px;
  font-weight: 700;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-family: var(--app-font);
}
.st-M { background: #d99c2b; }
.st-A, .st-U { background: #12b76a; }
.st-D { background: #d94834; }
.st-R, .st-C { background: #8b5cf6; }
.diff-file-name {
  flex: 1;
  min-width: 0;
  font-family: var(--app-font);
  font-size: 12px;
  font-weight: 600;
  color: var(--app-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.diff-adds, .diff-dels {
  font-family: var(--app-font);
  font-size: 12px;
  font-weight: 600;
  flex-shrink: 0;
}
.diff-adds { color: #12b76a; }
.diff-dels { color: #d94834; }

.diff-rows { border-top: 1px solid var(--app-border-soft); }
.diff-file-hint {
  padding: 12px;
  font-size: 12px;
  color: var(--app-text-faint);
}

/* ---------- 右键复制完整路径悬浮菜单 ---------- */
.diff-ctx-backdrop {
  position: fixed;
  inset: 0;
  z-index: 30000;
}
.diff-ctx-menu {
  position: fixed;
  z-index: 30001;
  min-width: 160px;
  background: var(--app-surface);
  border: 1px solid var(--app-border);
  border-radius: 8px;
  box-shadow: 0 8px 28px rgba(0, 0, 0, 0.16);
  padding: 4px;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
}
.diff-ctx-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 8px;
  font-size: 12.5px;
  color: var(--app-text);
  cursor: pointer;
  border-radius: 6px;
}
.diff-ctx-item:hover { background: #f1f5f9; color: #2563eb; }
</style>
