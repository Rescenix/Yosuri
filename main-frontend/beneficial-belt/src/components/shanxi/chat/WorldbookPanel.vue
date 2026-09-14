<template>
  <div class="wb-panel">
    <!-- 顶栏：模式切换（书本 / 条目）+ 关闭 -->
    <div class="wb-panel-header">
      <div class="wb-panel-tabs">
        <button type="button" class="wb-tab" :class="{ on: mode === 'book' }" @click="mode = 'book'">
          <Icon icon="ph:book-open" width="15" /> {{ tr('书本') }}
        </button>
        <button type="button" class="wb-tab" :class="{ on: mode === 'list' }" @click="mode = 'list'">
          <Icon icon="ph:list-bullets" width="15" /> {{ tr('条目') }}
        </button>
      </div>
      <div class="wb-panel-scope">
        <button type="button" class="wb-scope-btn" :class="{ on: !agentId }" @click="setScope('')">{{ tr('全局') }}</button>
        <button type="button" class="wb-scope-btn" :class="{ on: !!agentId }" @click="setScope(rpPrimaryId)">{{ tr('角色') }}</button>
      </div>
      <button type="button" class="wb-panel-close" @click="$emit('close')" title="关闭"><Icon icon="ph:x" width="16" /></button>
    </div>

    <!-- 书本模式：复用 re1 阅读器分页心智 -->
    <WorldbookBookView v-if="mode === 'book'" :entries="entries" class="wb-book-host" />

    <!-- 条目模式：增删改 + 试触发 -->
    <div v-else class="wb-list">
      <div class="wb-list-actions">
        <input v-model="testText" class="wb-test-input" :placeholder="tr('输入一句话，测哪些设定会触发')" @keyup.enter="testTrigger" />
        <button type="button" class="wb-test-btn" @click="testTrigger">{{ tr('试触发') }}</button>
        <button type="button" class="wb-add-btn" @click="startNewEntry"><Icon icon="ph:plus" width="14" /> {{ tr('新条目') }}</button>
      </div>
      <div v-if="testHits.length" class="wb-test-result">
        <span class="wb-test-result-label">{{ tr('命中') }} {{ testHits.length }}：</span>
        {{ testHits.map(h => h.name || ('#' + h.uid)).join('、') }}
      </div>

      <div v-if="!entries.length" class="wb-list-empty">{{ tr('还没有设定，点「新条目」写第一条吧') }}</div>

      <div v-for="e in sortedEntries" :key="e.uid" class="wb-entry" :class="{ off: e.enabled === false }">
        <div class="wb-entry-head">
          <span class="wb-entry-name">{{ e.name || ('#' + e.uid) }}</span>
          <span v-if="e.constant" class="wb-entry-badge const">{{ tr('常驻') }}</span>
          <span v-if="e.enabled === false" class="wb-entry-badge off">{{ tr('停用') }}</span>
          <span v-if="e.keys && e.keys.length" class="wb-entry-keys">{{ e.keys.join('、') }}</span>
          <div class="wb-entry-ops">
            <button type="button" class="wb-op-btn" :title="tr('编辑')" @click="editEntry(e)"><Icon icon="ph:pencil" width="13" /></button>
            <button type="button" class="wb-op-btn" :title="tr('删除')" @click="deleteEntry(e)"><Icon icon="ph:trash" width="13" /></button>
          </div>
        </div>
        <div class="wb-entry-content">{{ (e.content || '').slice(0, 120) }}{{ (e.content || '').length > 120 ? '…' : '' }}</div>
      </div>
    </div>

    <!-- 编辑弹层 -->
    <div v-if="editing" class="wb-edit-overlay" @click.self="editing = null">
      <div class="wb-edit-card">
        <div class="wb-edit-title">{{ editing.uid ? tr('编辑条目') : tr('新条目') }}</div>
        <label class="wb-edit-field">
          <span>{{ tr('标题') }}</span>
          <input v-model="editDraft.name" :placeholder="tr('比如：王都的规矩')" />
        </label>
        <label class="wb-edit-field">
          <span>{{ tr('触发词（逗号分隔，空=常驻）') }}</span>
          <input v-model="editKeys" :placeholder="tr('王都, 国王, 税')" />
        </label>
        <label class="wb-edit-field wb-edit-content">
          <span>{{ tr('设定正文') }}</span>
          <textarea v-model="editDraft.content" rows="5" :placeholder="tr('这条设定的事实内容…')"></textarea>
        </label>
        <div class="wb-edit-options">
          <label class="wb-check"><input type="checkbox" v-model="editDraft.constant" /> {{ tr('常驻（无条件注入）') }}</label>
          <label class="wb-check"><input type="checkbox" v-model="editDraft.enabled" /> {{ tr('启用') }}</label>
        </div>
        <div class="wb-edit-actions">
          <button type="button" class="wb-edit-cancel" @click="editing = null">{{ tr('取消') }}</button>
          <button type="button" class="wb-edit-save" @click="saveEntry">{{ tr('保存') }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
// WorldbookPanel.vue —— 世界书面板。
// 两种视图：书本（翻页读设定集，复用 re1 reader-standalone 的分页心智）+
// 条目（增删改、触发词、常驻、试触发）。挂在标题栏右侧「世界书」按钮。
// 数据走 /api/worldbook：agent_id 空=全局书，非空=该角色卡私有书。

import { ref, reactive, computed, onMounted } from 'vue'
import { tr } from '../../../composables/useI18n.js'
import WorldbookBookView from './WorldbookBookView.vue'

const props = defineProps({
  agentId: { type: String, default: '' }, // 当前会话的扮演角色 id（'' = 全局）
})
const emit = defineEmits(['close'])

const rpPrimaryId = computed(() => props.agentId)
const agentId = ref(props.agentId || '')
const mode = ref('book')
const entries = ref([])
const testText = ref('')
const testHits = ref([])

function setScope(v) { agentId.value = v; load() }

function wbUrl() {
  const q = agentId.value ? `?agent_id=${encodeURIComponent(agentId.value)}` : ''
  return '/api/worldbook' + q
}

async function load() {
  try {
    const res = await fetch(wbUrl())
    if (!res.ok) return
    const d = await res.json()
    entries.value = d.worldbook?.entries || []
    testHits.value = []
  } catch { /* 面板静默失败，下次打开重试 */ }
}

async function testTrigger() {
  const text = (testText.value || '').trim()
  if (!text) return
  try {
    const res = await fetch('/api/worldbook/test', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ agent_id: agentId.value, text }),
    })
    if (!res.ok) return
    const d = await res.json()
    testHits.value = d.hits || []
  } catch { /* 静默 */ }
}

const sortedEntries = computed(() => [...entries.value].sort((a, b) => {
  const ac = a.constant ? 0 : 1, bc = b.constant ? 0 : 1
  if (ac !== bc) return ac - bc
  return (a.order || 0) - (b.order || 0)
}))

// ── 编辑 ──
const editing = ref(null)
const editDraft = reactive({ uid: 0, name: '', content: '', constant: false, enabled: true })
const editKeys = ref('')

function startNewEntry() {
  Object.assign(editDraft, { uid: 0, name: '', content: '', constant: false, enabled: true })
  editKeys.value = ''
  editing.value = {}
}
function editEntry(e) {
  Object.assign(editDraft, {
    uid: e.uid, name: e.name || '', content: e.content || '',
    constant: !!e.constant, enabled: e.enabled !== false,
  })
  editKeys.value = (e.keys || []).join(',')
  editing.value = e
}
async function saveEntry() {
  const entry = {
    uid: editDraft.uid,
    name: editDraft.name.trim(),
    keys: editKeys.value.split(/[,，]/).map(s => s.trim()).filter(Boolean),
    content: editDraft.content.trim(),
    constant: editDraft.constant,
    enabled: editDraft.enabled,
  }
  if (!entry.content) return
  try {
    const res = await fetch('/api/worldbook/entry', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ agent_id: agentId.value, entry }),
    })
    if (!res.ok) return
    editing.value = null
    load()
  } catch { /* 静默 */ }
}
async function deleteEntry(e) {
  try {
    await fetch(`/api/worldbook/entry?agent_id=${encodeURIComponent(agentId.value)}&uid=${e.uid}`, { method: 'DELETE' })
    load()
  } catch { /* 静默 */ }
}

onMounted(load)
</script>
