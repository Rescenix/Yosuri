<template>
  <div class="snippet-panel">
    <!-- 顶部工具栏 -->
    <div class="snippet-toolbar">
      <button class="snippet-new-btn" type="button" @click="startCreate">
        <span class="snippet-new-icon">{ }</span>
        <span>New Snippet</span>
      </button>
      <div class="snippet-toolbar-actions">
        <button class="snippet-icon-btn" type="button" @click="searchOpen = !searchOpen" :title="searchOpen ? '关闭搜索' : '搜索'">
          <Icon :icon="searchOpen ? 'mdi:close' : 'mdi:magnify'" width="15" />
        </button>
      </div>
    </div>

    <!-- 搜索栏 -->
    <div v-if="searchOpen" class="snippet-search">
      <Icon icon="mdi:magnify" width="14" />
      <input ref="searchInputRef" v-model="searchQuery" type="text" placeholder="搜索片段…" @keydown.esc="closeSearch" />
    </div>

    <!-- 新建/编辑表单 -->
    <div v-if="editing" class="snippet-form">
      <div class="snippet-form-header">
        <button class="snippet-icon-btn" type="button" @click="cancelEdit">
          <Icon icon="mdi:arrow-left" width="16" />
        </button>
        <span class="snippet-form-title">{{ editing.id ? '编辑片段' : '新片段' }}</span>
        <button class="snippet-save-btn" type="button" @click="saveSnippet" :disabled="!editing.name.trim()">保存</button>
      </div>
      <label class="snippet-field">
        <span>名称</span>
        <input v-model="editing.name" type="text" placeholder="例如：Deploy" autocomplete="off" />
      </label>
      <label class="snippet-field">
        <span>命令</span>
        <textarea v-model="editing.command" rows="4" placeholder="例如：bash /root/deploy.sh" spellcheck="false"></textarea>
      </label>
    </div>

    <!-- 片段列表 -->
    <div v-else class="snippet-list">
      <div v-if="filteredSnippets.length === 0" class="snippet-empty">
        {{ searchQuery ? '没有匹配的片段' : '还没有片段，点击上方按钮创建' }}
      </div>
      <div
        v-for="s in filteredSnippets"
        :key="s.id"
        class="snippet-item"
        @click="$emit('insert', s.command)"
      >
        <div class="snippet-item-head">
          <span class="snippet-item-name">{{ s.name }}</span>
          <div class="snippet-item-actions">
            <button class="snippet-item-action" title="编辑" @click.stop="startEdit(s)">
              <Icon icon="mdi:pencil-outline" width="13" />
            </button>
            <button class="snippet-item-action danger" title="删除" @click.stop="deleteSnippet(s.id)">
              <Icon icon="mdi:delete-outline" width="13" />
            </button>
          </div>
        </div>
        <div class="snippet-item-cmd">{{ s.command }}</div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, nextTick, watch } from 'vue'
import { Icon } from '@iconify/vue'

const emit = defineEmits(['insert'])

const STORAGE_KEY = 'aurora_snippets'

function loadSnippets() {
  try {
    return JSON.parse(localStorage.getItem(STORAGE_KEY) || '[]')
  } catch { return [] }
}
function saveSnippets(list) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(list))
}

const snippets = ref(loadSnippets())
const searchOpen = ref(false)
const searchQuery = ref('')
const searchInputRef = ref(null)
const editing = ref(null) // { id?, name, command }

const filteredSnippets = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return snippets.value
  return snippets.value.filter(s =>
    s.name.toLowerCase().includes(q) || s.command.toLowerCase().includes(q)
  )
})

function startCreate() {
  editing.value = { name: '', command: '' }
}
function startEdit(s) {
  editing.value = { ...s }
}
function cancelEdit() {
  editing.value = null
}
function saveSnippet() {
  if (!editing.value || !editing.value.name.trim()) return
  if (editing.value.id) {
    snippets.value = snippets.value.map(s => s.id === editing.value.id ? { ...editing.value } : s)
  } else {
    snippets.value = [...snippets.value, { ...editing.value, id: 'sn_' + Date.now().toString(36), createdAt: Date.now() }]
  }
  saveSnippets(snippets.value)
  editing.value = null
}
function deleteSnippet(id) {
  snippets.value = snippets.value.filter(s => s.id !== id)
  saveSnippets(snippets.value)
}

function closeSearch() {
  searchOpen.value = false
  searchQuery.value = ''
}

watch(searchOpen, (v) => {
  if (v) nextTick(() => searchInputRef.value?.focus())
})
</script>

<style scoped>
.snippet-panel {
  width: 260px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  background: var(--app-surface-2);
  border-left: 1px solid var(--app-border);
  min-height: 0;
}

.snippet-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 10px;
  border-bottom: 1px solid var(--app-border);
  flex-shrink: 0;
}

.snippet-new-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 10px;
  font-size: 12px;
  font-weight: 600;
  color: var(--app-text);
  background: var(--app-surface-3);
  border: 1px solid var(--app-border);
  border-radius: 7px;
  cursor: pointer;
  transition: background 0.12s;
}
.snippet-new-btn:hover { background: var(--app-border); }

.snippet-new-icon {
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
  font-size: 11px;
  color: var(--app-accent);
  font-weight: 700;
}

.snippet-toolbar-actions {
  display: flex;
  gap: 2px;
}

.snippet-icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  border: none;
  background: transparent;
  color: var(--app-text-faint);
  border-radius: 6px;
  cursor: pointer;
  transition: background 0.12s, color 0.12s;
}
.snippet-icon-btn:hover { background: var(--app-surface-3); color: var(--app-text); }

.snippet-search {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 0 10px;
  height: 32px;
  border-bottom: 1px solid var(--app-border);
  flex-shrink: 0;
  color: var(--app-text-faint);
}
.snippet-search input {
  flex: 1;
  min-width: 0;
  border: none;
  outline: none;
  background: transparent;
  font-size: 12px;
  color: var(--app-text);
}

.snippet-form {
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 0;
  min-height: 0;
  overflow-y: auto;
}

.snippet-form-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border-bottom: 1px solid var(--app-border);
  flex-shrink: 0;
}

.snippet-form-title {
  flex: 1;
  font-size: 13px;
  font-weight: 600;
  color: var(--app-text);
}

.snippet-save-btn {
  padding: 4px 12px;
  font-size: 12px;
  font-weight: 600;
  color: #fff;
  background: var(--app-accent);
  border: none;
  border-radius: 6px;
  cursor: pointer;
  transition: opacity 0.12s;
}
.snippet-save-btn:disabled { opacity: 0.4; cursor: default; }
.snippet-save-btn:hover:not(:disabled) { opacity: 0.85; }

.snippet-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 10px 12px 0;
}
.snippet-field span {
  font-size: 11px;
  font-weight: 600;
  color: var(--app-text-soft);
}
.snippet-field input,
.snippet-field textarea {
  font-size: 12.5px;
  color: var(--app-text);
  background: var(--app-surface);
  border: 1px solid var(--app-border);
  border-radius: 7px;
  padding: 7px 10px;
  outline: none;
  font-family: inherit;
  resize: vertical;
}
.snippet-field input:focus,
.snippet-field textarea:focus {
  border-color: var(--app-accent);
}
.snippet-field textarea {
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
  font-size: 12px;
  line-height: 1.5;
  min-height: 80px;
}

.snippet-list {
  flex: 1;
  overflow-y: auto;
  padding: 4px 0;
}

.snippet-empty {
  padding: 24px 16px;
  text-align: center;
  font-size: 12px;
  color: var(--app-text-faint);
  line-height: 1.6;
}

.snippet-item {
  padding: 10px 12px;
  cursor: pointer;
  transition: background 0.1s;
}
.snippet-item:hover { background: var(--app-surface-3); }

.snippet-item-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  margin-bottom: 4px;
}

.snippet-item-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--app-text);
}

.snippet-item-actions {
  display: flex;
  gap: 2px;
  opacity: 0;
  transition: opacity 0.12s;
}
.snippet-item:hover .snippet-item-actions { opacity: 1; }

.snippet-item-action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border: none;
  background: transparent;
  color: var(--app-text-faint);
  border-radius: 4px;
  cursor: pointer;
}
.snippet-item-action:hover { background: var(--app-surface); color: var(--app-text); }
.snippet-item-action.danger:hover { color: #d94834; }

.snippet-item-cmd {
  font-size: 11.5px;
  color: var(--app-text-faint);
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  line-height: 1.5;
}
</style>
