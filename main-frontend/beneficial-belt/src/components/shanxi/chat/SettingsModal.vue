<template>
  <Teleport to="body">
    <div class="settings-modal-backdrop" @click="$emit('close')" @keydown.esc="$emit('close')">
      <div class="settings-modal-card" @click.stop>
        <div class="settings-modal-header">
          <span class="settings-modal-title">设置</span>
          <button class="settings-modal-close" @click="$emit('close')" title="关闭">
            <Icon icon="mdi:close" width="18" />
          </button>
        </div>
        <div class="settings-modal-body">
          <div class="settings-section-title">API 配置</div>
          <div class="settings-section-desc">配置自己的模型接入方式，保存后可以在底部模型切换器里选择"自定义"使用。</div>

          <div v-if="loading" class="settings-loading">加载中...</div>
          <template v-else>
            <div v-for="cfg in configs" :key="cfg.id" class="api-config-card">
              <div class="api-config-row">
                <span class="api-config-name">{{ cfg.name || '未命名配置' }}</span>
                <span v-if="cfg.is_default" class="api-config-default-badge">默认</span>
                <div class="api-config-actions">
                  <button v-if="!cfg.is_default" class="api-config-action-btn" @click="setDefault(cfg.id)">设为默认</button>
                  <button class="api-config-action-btn" @click="startEdit(cfg)">编辑</button>
                  <button class="api-config-action-btn danger" @click="removeConfig(cfg.id)">删除</button>
                </div>
              </div>
              <div class="api-config-meta">
                {{ cfg.endpoint }} · {{ cfg.default_model || '未指定模型' }} · {{ cfg.api_key_set ? '已设置 Key' : '未设置 Key' }}
              </div>
            </div>

            <div v-if="!editingConfig" class="api-config-add-btn" @click="startAdd">
              <Icon icon="mdi:plus" width="15" /> 新增 API 配置
            </div>

            <div v-else class="api-config-form">
              <div class="api-preset-row">
                <span class="api-preset-label">预设模板：</span>
                <button v-for="p in PRESETS" :key="p.name" class="api-preset-btn" type="button" @click="applyPreset(p)">{{ p.name }}</button>
              </div>
              <label class="api-form-field">
                <span>API 名称</span>
                <input v-model="editingConfig.name" type="text" placeholder="比如 DeepSeek" autocomplete="off" />
              </label>
              <label class="api-form-field">
                <span>Endpoint</span>
                <input v-model="editingConfig.endpoint" type="text" placeholder="https://api.example.com" autocomplete="off" />
              </label>
              <label class="api-form-field">
                <span>API Key</span>
                <input
                  v-model="editingConfig.api_key"
                  type="password"
                  autocomplete="new-password"
                  :placeholder="editingConfig.api_key_set ? '••••••••（留空则不修改）' : '输入 API Key'"
                />
              </label>
              <label class="api-form-field">
                <span>默认模型名</span>
                <input v-model="editingConfig.default_model" type="text" placeholder="比如 deepseek-chat" autocomplete="off" />
              </label>
              <div class="api-form-actions">
                <button class="api-form-btn cancel" type="button" @click="cancelEdit">取消</button>
                <button class="api-form-btn save" type="button" @click="saveConfig">保存</button>
              </div>
            </div>
          </template>
          <div v-if="errorMsg" class="settings-error">{{ errorMsg }}</div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { Icon } from '@iconify/vue'

const props = defineProps({
  // QQ 登录接入前先留空，后端会落到固定的 "default" 用户文件
  openid: { type: String, default: '' }
})
const emit = defineEmits(['close'])

const PRESETS = [
  { name: 'DeepSeek', endpoint: 'https://api.deepseek.com' },
  { name: 'Agnes', endpoint: 'https://apihub.agnes-ai.com/v1' },
  { name: '本地 Ollama', endpoint: 'http://localhost:11434' }
]
const MASKED = '••••••••'

const configs = ref([])
const loading = ref(true)
const errorMsg = ref('')
const editingConfig = ref(null)
const isNew = ref(false)

function configUrl() {
  return `/api/models/config${props.openid ? '?openid=' + encodeURIComponent(props.openid) : ''}`
}

async function loadConfigs() {
  loading.value = true
  errorMsg.value = ''
  try {
    const res = await fetch(configUrl())
    if (!res.ok) throw new Error('加载失败')
    const data = await res.json()
    configs.value = data.configs || []
  } catch (e) {
    errorMsg.value = '加载配置失败，请稍后再试'
  } finally {
    loading.value = false
  }
}

async function persist(nextConfigs) {
  const res = await fetch(configUrl(), {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ configs: nextConfigs })
  })
  if (!res.ok) {
    const data = await res.json().catch(() => ({}))
    throw new Error(data.error || '保存失败')
  }
}

function startAdd() {
  isNew.value = true
  errorMsg.value = ''
  editingConfig.value = {
    id: 'cfg_' + Date.now().toString(36),
    name: '', endpoint: '', api_key: '', api_key_set: false,
    default_model: '', is_default: configs.value.length === 0
  }
}
function startEdit(cfg) {
  isNew.value = false
  errorMsg.value = ''
  // api_key 故意留空——用户不填就代表"不改密钥"，绝不会把已经打码的值当真密钥送回后端
  editingConfig.value = { ...cfg, api_key: '' }
}
function cancelEdit() {
  editingConfig.value = null
}
function applyPreset(p) {
  if (!editingConfig.value) return
  editingConfig.value.endpoint = p.endpoint
  if (!editingConfig.value.name) editingConfig.value.name = p.name
}

async function saveConfig() {
  if (!editingConfig.value.endpoint.trim()) {
    errorMsg.value = 'Endpoint 不能为空'
    return
  }
  errorMsg.value = ''
  const entry = {
    id: editingConfig.value.id,
    name: editingConfig.value.name,
    endpoint: editingConfig.value.endpoint,
    api_key: editingConfig.value.api_key || MASKED,
    default_model: editingConfig.value.default_model,
    is_default: editingConfig.value.is_default
  }
  let next = isNew.value
    ? [...configs.value, entry]
    : configs.value.map(c => (c.id === entry.id ? entry : c))
  if (entry.is_default) {
    next = next.map(c => ({ ...c, api_key: c.id === entry.id ? entry.api_key : MASKED, is_default: c.id === entry.id }))
  }
  try {
    await persist(next)
    await loadConfigs()
    editingConfig.value = null
  } catch (e) {
    errorMsg.value = e.message
  }
}

async function removeConfig(id) {
  const next = configs.value.filter(c => c.id !== id).map(c => ({ ...c, api_key: MASKED }))
  try {
    await persist(next)
    await loadConfigs()
  } catch (e) {
    errorMsg.value = e.message
  }
}

async function setDefault(id) {
  const next = configs.value.map(c => ({ ...c, api_key: MASKED, is_default: c.id === id }))
  try {
    await persist(next)
    await loadConfigs()
  } catch (e) {
    errorMsg.value = e.message
  }
}

function handleEsc(e) {
  if (e.key === 'Escape') emit('close')
}

onMounted(() => {
  loadConfigs()
  document.addEventListener('keydown', handleEsc)
})
onUnmounted(() => {
  document.removeEventListener('keydown', handleEsc)
})
</script>

<style scoped>
.settings-modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(20, 18, 15, 0.35);
  backdrop-filter: blur(6px);
  -webkit-backdrop-filter: blur(6px);
  /* 这个组件用 <Teleport to="body"> 挂到 body 下，跟 .chat-window 是兄弟节点
     而不是子节点——层叠顺序只看 z-index 数值。.chat-window 本身是铺满全屏的
     position:fixed，z-index:9998，之前这里的 9990 比它还低，导致弹窗其实
     渲染出来了、只是被全屏的聊天窗口挡在下面，DOM 里能查到但用户根本看不见。 */
  z-index: 20000;
  display: flex;
  align-items: center;
  justify-content: center;
}
.settings-modal-card {
  width: 560px;
  max-width: calc(100vw - 48px);
  max-height: calc(100vh - 96px);
  background: #ffffff;
  border-radius: 16px;
  box-shadow: 0 24px 64px rgba(0, 0, 0, 0.24);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.settings-modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid #e5e5e5;
  flex-shrink: 0;
}
.settings-modal-title { font-size: 15px; font-weight: 700; color: #1a1a1a; }
.settings-modal-close {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 6px;
  border: none;
  background: transparent;
  cursor: pointer;
  color: #6b6b6b;
}
.settings-modal-close:hover { background: #f5f5f5; }
.settings-modal-body { padding: 18px 20px 22px; overflow-y: auto; }

.settings-section-title { font-size: 13.5px; font-weight: 700; color: #1a1a1a; margin-bottom: 4px; }
.settings-section-desc { font-size: 12px; color: #a3a3a3; margin-bottom: 14px; }
.settings-loading { font-size: 12.5px; color: #a3a3a3; padding: 12px 0; }

.api-config-card { border: 1px solid #e5e5e5; border-radius: 10px; padding: 10px 12px; margin-bottom: 8px; background: #fafafa; }
.api-config-row { display: flex; align-items: center; gap: 8px; }
.api-config-name { font-size: 13px; font-weight: 600; color: #1a1a1a; flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.api-config-default-badge { font-size: 10.5px; font-weight: 600; color: #12b76a; background: rgba(18, 183, 106, 0.12); padding: 2px 8px; border-radius: 999px; flex-shrink: 0; }
.api-config-actions { display: flex; gap: 4px; flex-shrink: 0; }
.api-config-action-btn { font-size: 11.5px; color: #6b6b6b; background: transparent; border: 1px solid #e5e5e5; border-radius: 6px; padding: 3px 8px; cursor: pointer; }
.api-config-action-btn:hover { background: #f0f0f0; }
.api-config-action-btn.danger { color: #d94834; border-color: #f3c9c2; }
.api-config-meta {
  margin-top: 5px;
  font-size: 11px;
  color: #a3a3a3;
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.api-config-add-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 9px 0;
  border: 1px dashed #d4d4d4;
  border-radius: 10px;
  color: #6b6b6b;
  font-size: 12.5px;
  font-weight: 600;
  cursor: pointer;
}
.api-config-add-btn:hover { background: #fafafa; border-color: #c4c4c4; }

.api-config-form { border: 1px solid #e5e5e5; border-radius: 10px; padding: 14px; background: #fafafa; }
.api-preset-row { display: flex; align-items: center; gap: 6px; margin-bottom: 12px; flex-wrap: wrap; }
.api-preset-label { font-size: 11.5px; color: #a3a3a3; margin-right: 2px; }
.api-preset-btn { font-size: 11.5px; font-weight: 600; color: #1a1a1a; background: #ffffff; border: 1px solid #e5e5e5; border-radius: 999px; padding: 4px 10px; cursor: pointer; }
.api-preset-btn:hover { background: #f0f0f0; }

.api-form-field { display: flex; flex-direction: column; gap: 4px; margin-bottom: 10px; }
.api-form-field span { font-size: 11.5px; color: #6b6b6b; font-weight: 600; }
.api-form-field input {
  font-size: 13px;
  padding: 7px 10px;
  border: 1px solid #e5e5e5;
  border-radius: 8px;
  background: #ffffff;
  outline: none;
  font-family: inherit;
}
.api-form-field input:focus { border-color: #c96442; }

.api-form-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 4px; }
.api-form-btn { font-size: 12.5px; font-weight: 600; padding: 6px 14px; border-radius: 8px; cursor: pointer; border: none; }
.api-form-btn.cancel { background: transparent; border: 1px solid #e5e5e5; color: #6b6b6b; }
.api-form-btn.cancel:hover { background: #f0f0f0; }
.api-form-btn.save { background: #1a1a1a; color: #fff; }
.api-form-btn.save:hover { background: #333333; }

.settings-error { margin-top: 10px; font-size: 12px; color: #d94834; }
</style>
