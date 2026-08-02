<template>
  <Teleport to="body">
    <div class="fo-backdrop" @click="$emit('close')">
      <div class="fo-card" @click.stop>
        <div class="fo-header">
          <span class="fo-title">Auto 自定义排序</span>
          <button class="fo-close" @click="$emit('close')" title="关闭">
            <Icon icon="mdi:close" width="18" />
          </button>
        </div>
        <div class="fo-hint">
          <Icon icon="mdi:auto-fix" width="13" class="fo-hint-icon" />
          <span>此顺序决定 Auto 智能路由逐个尝试免费模型的顺序 —— 拖动调整，松手自动保存</span>
        </div>
        <div class="fo-body">
          <div v-if="loading" class="fo-loading">加载中...</div>
          <template v-else-if="list.length">
            <div
              v-for="m in list"
              :key="m.id"
              class="fo-row"
              :class="{ dragging: dragId === m.id, over: dragOverId === m.id }"
              draggable="true"
              @dragstart="onDragStart($event, m.id)"
              @dragend="onDragEnd"
              @dragover.prevent="onDragOver(m.id)"
              @dragleave="onDragLeave(m.id)"
              @drop.prevent="onDrop(m.id)"
            >
              <Icon icon="mdi:drag-vertical" width="16" class="fo-drag" />
              <span class="fo-name">{{ m.name }}</span>
              <span class="fo-vendor">{{ m.vendor }}</span>
              <span class="fo-tag" :class="{ free: m.keyless, nkey: !m.keyless && !m.api_key_set }">{{ m.keyless ? '免 Key' : (m.api_key_set ? '已配 Key' : '未配 Key') }}</span>
            </div>
          </template>
          <div v-else class="fo-loading">没有免费模型</div>
        </div>
        <div class="fo-footer">
          <button class="fo-reset" type="button" :disabled="saving" @click="resetOrder">恢复默认顺序</button>
          <button class="fo-pin-ds" type="button" :disabled="saving" @click="pinDeepSeekTop" title="将 DeepSeek（小鲸鱼）模型置顶">
            <Icon icon="mdi:pin" width="13" /> 小鲸鱼置顶
          </button>
          <span class="fo-save-state">{{ saving ? '保存中...' : saveError }}</span>
          <button class="fo-done" type="button" @click="$emit('close')">完成</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { Icon } from '@iconify/vue'

const props = defineProps({
  openid: { type: String, default: '' }
})
defineEmits(['close'])

const loading = ref(true)
const saving = ref(false)
const saveError = ref('')
const list = ref([]) // 已按 free_model_order.json 排序的免费模型
const dragId = ref('')
const dragOverId = ref('')

function configUrl() {
  return `/api/models/config${props.openid ? '?openid=' + encodeURIComponent(props.openid) : ''}`
}

const FREE_ORDER_KEY = 'freeModelOrder'

function loadOrderFromStorage() {
  try { return JSON.parse(localStorage.getItem(FREE_ORDER_KEY) || '[]') } catch { return [] }
}
function saveOrderToStorage(order) {
  localStorage.setItem(FREE_ORDER_KEY, JSON.stringify(order))
}

async function load() {
  loading.value = true
  saveError.value = ''
  try {
    const res = await fetch(configUrl())
    if (!res.ok) throw new Error('HTTP ' + res.status)
    const data = await res.json()
    let models = data.free_models || []
    // 优先用 localStorage 里的排序（用户保存的顺序），后端按信号/最近使用重排了
    const savedOrder = loadOrderFromStorage()
    if (savedOrder.length) {
      const byId = new Map(models.map(m => [m.id, m]))
      const ordered = savedOrder.map(id => byId.get(id)).filter(Boolean)
      const rest = models.filter(m => !savedOrder.includes(m.id))
      models = [...ordered, ...rest]
    }
    list.value = models
  } catch (e) {
    saveError.value = '加载失败：' + (e.message || '网络错误')
  } finally {
    loading.value = false
  }
}
onMounted(load)

function onDragStart(e, id) {
  dragId.value = id
  if (e.dataTransfer) e.dataTransfer.effectAllowed = 'move'
}
function onDragEnd() {
  dragId.value = ''
  dragOverId.value = ''
}
function onDragOver(id) {
  if (dragId.value && dragId.value !== id) dragOverId.value = id
}
function onDragLeave(id) {
  if (dragOverId.value === id) dragOverId.value = ''
}
async function onDrop(targetId) {
  const from = dragId.value
  dragId.value = ''
  dragOverId.value = ''
  if (!from || from === targetId) return
  const arr = list.value.map(m => m.id)
  const fi = arr.indexOf(from)
  const ti = arr.indexOf(targetId)
  if (fi === -1 || ti === -1) return
  arr.splice(fi, 1)
  arr.splice(ti, 0, from)
  // 本地立即重排
  const byId = new Map(list.value.map(m => [m.id, m]))
  list.value = arr.map(id => byId.get(id)).filter(Boolean)
  await saveOrder(arr)
}

async function saveOrder(arr) {
  saving.value = true
  saveError.value = ''
  saveOrderToStorage(arr) // 先存本地，关掉重开弹窗时恢复
  try {
    const res = await fetch('/api/models/free-order', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ order: arr })
    })
    if (!res.ok) throw new Error('HTTP ' + res.status)
    // 通知 ChatWidget / 设置面板重新拉取（聊天下拉顺序同步更新）
    window.dispatchEvent(new CustomEvent('model-config-changed'))
  } catch (e) {
    saveError.value = '保存失败：' + (e.message || '网络错误')
    await load() // 回滚到后端顺序
  } finally {
    saving.value = false
  }
}

async function resetOrder() {
  saving.value = true
  saveError.value = ''
  saveOrderToStorage([]) // 清空本地排序
  try {
    const res = await fetch('/api/models/free-order', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ order: [] }) // 空 = 恢复目录默认顺序
    })
    if (!res.ok) throw new Error('HTTP ' + res.status)
    window.dispatchEvent(new CustomEvent('model-config-changed'))
    await load() // 重新按目录顺序展示
  } catch (e) {
    saveError.value = '恢复失败：' + (e.message || '网络错误')
  } finally {
    saving.value = false
  }
}

// 小鲸鱼一键置顶：将名称含「DeepSeek」的模型全部提到列表最前面
async function pinDeepSeekTop() {
  saving.value = true
  saveError.value = ''
  try {
    const arr = list.value.map(m => m.id)
    const dsIds = list.value.filter(m => m.name.includes('DeepSeek')).map(m => m.id)
    // 将 DeepSeek 模型移到最前面，非 DeepSeek 保持相对顺序
    const rest = arr.filter(id => !dsIds.includes(id))
    const newOrder = [...dsIds, ...rest]
    // 本地立即重排
    const byId = new Map(list.value.map(m => [m.id, m]))
    list.value = newOrder.map(id => byId.get(id)).filter(Boolean)
    await saveOrder(newOrder)
  } catch (e) {
    saveError.value = '置顶失败：' + (e.message || '网络错误')
    await load()
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.fo-backdrop {
  position: fixed; inset: 0; background: rgba(15, 23, 42, 0.45);
  display: flex; align-items: center; justify-content: center; z-index: 99999;
}
.fo-card {
  width: 460px; max-height: 80vh; background: var(--app-surface);
  border: 1px solid var(--app-border); border-radius: 14px;
  display: flex; flex-direction: column; box-shadow: 0 20px 60px rgba(0,0,0,0.25);
  overflow: hidden;
}
.fo-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 14px 16px; border-bottom: 1px solid var(--app-border);
}
.fo-title { font-size: 15px; font-weight: 700; color: var(--app-text); }
.fo-close {
  background: none; border: none; color: var(--app-text-soft); cursor: pointer;
  display: flex; padding: 2px;
}
.fo-close:hover { color: var(--app-text); }
.fo-hint {
  display: flex; align-items: flex-start; gap: 6px; margin: 10px 16px 0;
  padding: 8px 10px; border-radius: 8px;
  background: var(--app-accent-soft); color: var(--app-text-soft);
  font-size: 11px; line-height: 1.5;
}
.fo-hint-icon { color: var(--app-accent); flex-shrink: 0; margin-top: 1px; }
.fo-body { overflow-y: auto; padding: 10px 8px 4px; flex: 1; }
.fo-row {
  display: flex; align-items: center; gap: 8px; padding: 8px 10px;
  font-size: 13px; color: var(--app-text); cursor: grab; user-select: none;
  border-radius: 8px; border-bottom: 1px solid var(--app-border-soft);
  transition: background 0.12s, opacity 0.12s;
}
.fo-row:last-child { border-bottom: none; }
.fo-row:hover { background: var(--app-surface-3); }
.fo-row.dragging { opacity: 0.45; }
.fo-row.over { background: var(--app-accent-soft); }
.fo-row:active { cursor: grabbing; }
.fo-drag { color: var(--app-text-faint); flex-shrink: 0; }
.fo-name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.fo-vendor { font-size: 11px; color: var(--app-text-faint); flex-shrink: 0; max-width: 120px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.fo-tag { font-size: 10px; font-weight: 600; padding: 1px 7px; border-radius: 999px; flex-shrink: 0; }
.fo-tag.free { color: var(--app-accent); background: var(--app-accent-soft); }
.fo-tag.nkey { color: var(--app-text-faint); background: var(--app-surface-3); }
.fo-footer {
  display: flex; align-items: center; gap: 10px; padding: 12px 16px;
  border-top: 1px solid var(--app-border);
}
.fo-reset {
  font-size: 12px; color: var(--app-text-soft); background: none;
  border: 1px solid var(--app-border); border-radius: 999px; padding: 4px 12px;
  cursor: pointer;
}
.fo-reset:hover:not(:disabled) { color: var(--app-accent); border-color: var(--app-accent); }
.fo-reset:disabled { opacity: 0.5; cursor: default; }
.fo-pin-ds {
  font-size: 12px; color: var(--app-accent); background: var(--app-accent-soft);
  border: 1px solid var(--app-accent); border-radius: 999px; padding: 4px 12px;
  cursor: pointer; display: inline-flex; align-items: center; gap: 4px; white-space: nowrap;
}
.fo-pin-ds:hover:not(:disabled) { opacity: 0.85; }
.fo-pin-ds:disabled { opacity: 0.5; cursor: default; }
.fo-save-state { flex: 1; font-size: 11px; color: var(--app-text-faint); }
.fo-done {
  font-size: 12px; font-weight: 600; color: #fff;
  background: var(--app-accent); border: none; border-radius: 999px; padding: 5px 16px;
  cursor: pointer;
}
.fo-done:hover { opacity: 0.9; }
.fo-loading { padding: 24px; text-align: center; color: var(--app-text-faint); font-size: 13px; }
</style>
