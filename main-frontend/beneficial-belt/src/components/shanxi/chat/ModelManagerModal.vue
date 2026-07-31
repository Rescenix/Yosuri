<template>
  <Teleport to="body">
    <div class="mm-backdrop" @click="$emit('close')">
      <div class="mm-card" @click.stop>
        <div class="mm-header">
          <span class="mm-title">模型</span>
          <button class="mm-close" @click="$emit('close')" title="关闭">
            <Icon icon="mdi:close" width="18" />
          </button>
        </div>
        <div class="mm-search">
          <Icon icon="mdi:magnify" width="14" class="mm-search-icon" />
          <input v-model="q" type="text" placeholder="搜索模型" class="mm-search-input" />
        </div>
        <div class="mm-body">
          <div v-if="loading" class="mm-loading">加载中...</div>
          <template v-else>
            <div v-for="grp in filteredGroups" :key="grp.vendor" class="mm-group">
              <div class="mm-group-head">
                <span class="mm-group-name">{{ grp.vendor }}</span>
                <label class="mm-switch" @click.prevent="toggleVendor(grp)">
                  <input type="checkbox" :checked="vendorOn(grp)" />
                  <span class="mm-switch-track"></span>
                </label>
              </div>
              <div v-for="m in grp.items" :key="m.id" class="mm-model-row">
                <span class="mm-model-name">{{ m.name }}</span>
                <span v-if="m.keyless" class="mm-tag mm-tag-free">免 Key</span>
                <span v-else-if="!m.api_key_set" class="mm-tag mm-tag-nkey">未配 Key</span>
                <label class="mm-switch" @click.prevent="toggleHidden(m.id)">
                  <input type="checkbox" :checked="!isHidden(m.id)" />
                  <span class="mm-switch-track"></span>
                </label>
              </div>
            </div>
          </template>
        </div>
        <div class="mm-thanks">💙 感谢所有免费模型提供方的赞助支持</div>
        <div class="mm-footer" @click="$emit('add-provider')">
          <Icon icon="mdi:plus" width="14" /> 添加提供方...
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { ref, computed } from 'vue'
import { Icon } from '@iconify/vue'
import { hiddenModelIds, isHidden, toggleHidden, batchSet } from '../composables/modelVisibility.js'

const props = defineProps({
  freeModels: { type: Array, default: () => [] },
  loading: { type: Boolean, default: false }
})
defineEmits(['close', 'add-provider'])

const q = ref('')

// 按提供方分组（本地模型不在此管理，走自动识图候选）
const groups = computed(() => {
  const map = new Map()
  for (const fm of props.freeModels) {
    if (fm.local) continue
    const v = fm.vendor || '其他'
    if (!map.has(v)) map.set(v, { vendor: v, items: [] })
    map.get(v).items.push(fm)
  }
  return Array.from(map.values())
})

const filteredGroups = computed(() => {
  const term = q.value.trim().toLowerCase()
  return groups.value
    .map(g => ({
      vendor: g.vendor,
      items: term ? g.items.filter(m => (m.name || '').toLowerCase().includes(term)) : g.items
    }))
    .filter(g => g.items.length > 0)
})

// 提供方级开关：整组所有模型都可见 = 开
function vendorOn(grp) {
  return grp.items.every(m => !isHidden(m.id))
}
function toggleVendor(grp) {
  batchSet(grp.items.map(m => m.id), !vendorOn(grp))
}
</script>

<style scoped>
.mm-backdrop {
  position: fixed; inset: 0; background: rgba(15, 23, 42, 0.45);
  display: flex; align-items: center; justify-content: center; z-index: 99999;
}
.mm-card {
  width: 420px; max-height: 80vh; background: var(--app-surface);
  border: 1px solid var(--app-border); border-radius: 14px;
  display: flex; flex-direction: column; box-shadow: 0 20px 60px rgba(0,0,0,0.25);
  overflow: hidden;
}
.mm-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 14px 16px; border-bottom: 1px solid var(--app-border);
}
.mm-title { font-size: 15px; font-weight: 700; color: var(--app-text); }
.mm-close {
  background: none; border: none; color: var(--app-text-soft); cursor: pointer;
  display: flex; padding: 2px;
}
.mm-close:hover { color: var(--app-text); }
.mm-search {
  display: flex; align-items: center; gap: 6px; margin: 12px 16px 4px;
  background: var(--app-surface-3); border: 1px solid var(--app-border);
  border-radius: 8px; padding: 7px 10px;
}
.mm-search-icon { color: var(--app-text-faint); flex-shrink: 0; }
.mm-search-input {
  border: none; background: none; outline: none; flex: 1;
  font-size: 13px; color: var(--app-text);
}
.mm-body { overflow-y: auto; padding: 8px 8px 4px; flex: 1; }
.mm-group { margin-bottom: 6px; }
.mm-group-head {
  display: flex; align-items: center; justify-content: space-between;
  padding: 6px 8px;
}
.mm-group-name {
  font-size: 11px; font-weight: 700; letter-spacing: 0.04em;
  color: var(--app-text-faint); text-transform: uppercase;
}
.mm-model-row {
  display: flex; align-items: center; gap: 8px;
  padding: 7px 8px; border-radius: 8px;
}
.mm-model-row:hover { background: var(--app-surface-3); }
.mm-model-name { flex: 1; font-size: 13px; color: var(--app-text); }
.mm-tag {
  font-size: 10px; font-weight: 600; padding: 1px 7px; border-radius: 999px; flex-shrink: 0;
}
.mm-tag-free { color: var(--app-accent); background: var(--app-accent-soft); }
.mm-tag-nkey { color: var(--app-text-faint); background: var(--app-surface-3); }
.mm-switch { position: relative; display: inline-flex; cursor: pointer; flex-shrink: 0; }
.mm-switch input { position: absolute; opacity: 0; width: 0; height: 0; }
.mm-switch-track {
  width: 36px; height: 20px; border-radius: 999px; background: var(--app-surface-3);
  border: 1px solid var(--app-border); transition: background 0.15s;
  position: relative;
}
.mm-switch-track::after {
  content: ''; position: absolute; top: 2px; left: 2px; width: 14px; height: 14px;
  border-radius: 50%; background: #fff; transition: transform 0.15s;
}
.mm-switch input:checked + .mm-switch-track { background: var(--app-accent); border-color: var(--app-accent); }
.mm-switch input:checked + .mm-switch-track::after { transform: translateX(16px); }
.mm-thanks {
  padding: 8px 16px;
  font-size: 11px;
  color: var(--app-text-faint);
  text-align: center;
  border-top: 1px solid var(--app-border-soft);
  user-select: none;
}
.mm-footer {
  display: flex; align-items: center; gap: 6px; padding: 12px 16px;
  border-top: 1px solid var(--app-border); font-size: 13px;
  color: var(--app-text-soft); cursor: pointer;
}
.mm-footer:hover { color: var(--app-accent); }
.mm-loading { padding: 20px; text-align: center; color: var(--app-text-faint); font-size: 13px; }
</style>
