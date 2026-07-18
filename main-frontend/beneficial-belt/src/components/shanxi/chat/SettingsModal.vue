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
          <!-- 左侧边栏：两个 tab -->
          <div class="settings-sidebar">
            <button class="settings-tab" :class="{ on: activeTab === 'providers' }" @click="activeTab = 'providers'">提供方</button>
            <button class="settings-tab" :class="{ on: activeTab === 'appearance' }" @click="activeTab = 'appearance'">外观</button>
          </div>

          <!-- 右侧内容区 -->
          <div class="settings-content">
            <!-- ========== 提供方 ========== -->
            <div v-show="activeTab === 'providers'" class="settings-panel">
              <div class="settings-section-title">免费模型提供商</div>
              <div class="settings-section-desc">点一个提供商，它的全部模型会直接进聊天下拉框（再点一次移除）。填了 Key 的提供商才会真正进调用链。</div>

              <div v-if="loading" class="settings-loading">加载中...</div>
              <template v-else>
                <div v-for="grp in vendorGroups" :key="grp.vendor" class="vendor-group">
                  <div class="vendor-head">
                    <span class="vendor-name">{{ grp.vendor }}</span>
                    <span class="vendor-count">{{ grp.items.length }} 个模型</span>
                    <span class="vendor-keystate" :class="{ on: grp.hasKey }">{{ grp.hasKey ? '已配 Key' : '未配 Key' }}</span>
                    <button
                      class="model-pick-btn"
                      :class="{ on: isVendorSelected(grp.vendor) }"
                      type="button"
                      @click.stop="toggleVendorModels(grp)"
                    >{{ isVendorSelected(grp.vendor) ? '已选' : '选为可用' }}</button>
                    <button v-if="editingVendor !== grp.vendor" class="vendor-key-btn" @click.stop="startEditVendor(grp)">{{ grp.hasKey ? '改 Key' : '填 Key' }}</button>
                    <button v-else class="vendor-key-btn" @click.stop="cancelVendorEdit">收起</button>
                  </div>
                  <div v-if="editingVendor === grp.vendor" class="vendor-key-inline">
                    <input
                      v-model="vendorKeyDraft"
                      type="password"
                      class="vendor-key-input"
                      :placeholder="grp.hasKey ? '••••••••（留空则不修改）' : '输入 ' + grp.vendor + ' 的 API Key'"
                      @keyup.enter="saveVendorKey(grp)"
                    />
                    <button class="vendor-key-save" type="button" @click="saveVendorKey(grp)">保存</button>
                    <button class="vendor-key-cancel" type="button" @click="cancelVendorEdit">取消</button>
                  </div>
                  <div class="vendor-model-hint">点击「选为可用」将加入：{{ grp.items.map(m => m.name).join('、') }}</div>
                </div>
              </template>

              <div class="settings-section-title" style="margin-top: 18px;">自定义 API 配置</div>
              <div class="settings-section-desc">配置自己的模型接入方式；设为默认的配置会排在调用链最前面。</div>

              <template v-if="!loading">
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
            </div>

            <!-- ========== 外观 ========== -->
            <div v-show="activeTab === 'appearance'" class="settings-panel">
              <div class="settings-section-title">流式渐变</div>
              <div class="settings-section-desc">AI 回复逐字级联淡入的"瀑布"效果（仿 ChatGPT/Gemini）。改动即时生效并自动保存。</div>

              <div class="param-row">
                <span class="param-label">流式渐变</span>
                <label class="param-switch">
                  <input type="checkbox" v-model="streamFadeConfig.enabled" />
                  <span class="param-switch-track"></span>
                </label>
              </div>

              <template v-if="streamFadeConfig.enabled">
                <div class="param-row">
                  <span class="param-label">淡入时长</span>
                  <input class="param-range" type="range" min="150" max="1500" step="50" v-model.number="streamFadeConfig.fadeMs" />
                  <span class="param-value">{{ streamFadeConfig.fadeMs }} ms</span>
                </div>
                <div class="param-row">
                  <span class="param-label">级联间隔</span>
                  <input class="param-range" type="range" min="0" max="40" step="2" v-model.number="streamFadeConfig.staggerMs" />
                  <span class="param-value">{{ streamFadeConfig.staggerMs }} ms/字</span>
                </div>
                <div class="param-row">
                  <span class="param-label">模糊强度</span>
                  <input class="param-range" type="range" min="0" max="6" step="0.5" v-model.number="streamFadeConfig.blurPx" />
                  <span class="param-value">{{ streamFadeConfig.blurPx }} px</span>
                </div>
                <div class="param-row">
                  <span class="param-label">大块扫过上限</span>
                  <input class="param-range" type="range" min="100" max="1000" step="50" v-model.number="streamFadeConfig.maxSweepMs" />
                  <span class="param-value">{{ streamFadeConfig.maxSweepMs }} ms</span>
                </div>
              </template>
              <div class="param-reset-row">
                <button class="param-reset-btn" type="button" @click="resetStreamFadeConfig">恢复默认</button>
              </div>

              <!-- 无限循环全格式预览（纯前端，不花 token） -->
              <div class="settings-section-title" style="margin-top: 18px;">实时预览</div>
              <div class="settings-section-desc">下方按当前配置无限循环重播淡入，覆盖标题/列表/代码/表格/公式等全部格式。调参即时生效。</div>
              <div class="preview-stage" ref="previewStage">
                <div class="preview-bubble markdown-body" ref="previewBubble" v-html="previewHtml"></div>
              </div>
            </div>
          </div>

          <div v-if="errorMsg" class="settings-error">{{ errorMsg }}</div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { ref, computed, nextTick, onMounted, onUnmounted } from 'vue'
import { Icon } from '@iconify/vue'
import { chatModelList, setChatModelList, syncChatModelList } from '../composables/chatModelList.js'
import { streamFadeConfig, resetStreamFadeConfig } from '../composables/streamFadeConfig.js'
import { PREVIEW_MARKDOWN } from '../composables/useDemoMode.js'
import { renderMarkdown } from './markdownRenderer.js'

const props = defineProps({
  openid: { type: String, default: '' }
})
const emit = defineEmits(['close'])

// 左侧边栏当前 tab
const activeTab = ref('providers')

// ============ 流式渐变无限循环预览（纯前端，不花 token） ============
const previewBubble = ref(null)
const previewHtml = ref('')
let previewTimer = null
let previewOffset = 0

// 用真实 renderMarkdown 把样本渲染成 HTML（含 katex/表格/代码），按当前配置
// 逐字包 span.stream-fade-seg 触发全局 om-stream-fade 动画；到尾部后从头循环。
function renderPreviewFrame() {
  const el = previewBubble.value
  if (!el) return
  const { fadeMs, staggerMs, blurPx } = streamFadeConfig
  const html = renderMarkdown(PREVIEW_MARKDOWN, true)
  const text = PREVIEW_MARKDOWN
  // 用一段“打字机”窗口：每帧多露几个字 + 给新露出的字加淡入动画
  const spanAll = (fullHtml) => {
    // 简单地整段插 span 会破坏标签，这里对纯文本快照逐字动画不合适；
    // 改为：整段 v-html 渲染，再对当前可见文本节点逐字包 span 做一次性淡入。
    el.innerHTML = fullHtml
    const walker = document.createTreeWalker(el, NodeFilter.SHOW_TEXT, {
      acceptNode(n) {
        const p = n.parentElement
        if (p && p.closest('pre, code, table, .katex, .code-btn-group')) return NodeFilter.FILTER_REJECT
        return NodeFilter.FILTER_ACCEPT
      }
    })
    const nodes = []
    let n
    while ((n = walker.nextNode())) nodes.push(n)
    let i = 0
    for (const node of nodes) {
      const text = node.nodeValue
      const frag = document.createDocumentFragment()
      for (const ch of text) {
        const span = document.createElement('span')
        span.className = 'stream-fade-seg'
        span.textContent = ch
        span.style.animationDuration = fadeMs + 'ms'
        span.style.animationDelay = (i * staggerMs) + 'ms'
        span.style.setProperty('--sf-blur', blurPx + 'px')
        frag.appendChild(span)
        i++
      }
      node.parentNode.replaceChild(frag, node)
    }
  }
  spanAll(html)
}

function startPreviewLoop() {
  stopPreviewLoop()
  renderPreviewFrame()
  // 整段淡入播完（按字符数估算时长）后从头循环
  const total = PREVIEW_MARKDOWN.length
  const { fadeMs, staggerMs } = streamFadeConfig
  const oneRound = total * staggerMs + fadeMs + 800
  previewTimer = setInterval(() => {
    if (activeTab.value === 'appearance') renderPreviewFrame()
  }, Math.max(2500, oneRound))
}
function stopPreviewLoop() {
  if (previewTimer) { clearInterval(previewTimer); previewTimer = null }
}

const PRESETS = [
  { name: 'DeepSeek', endpoint: 'https://api.deepseek.com' }
]
const MASKED = '••••••••'

const configs = ref([])
const freeModels = ref([])
const loading = ref(true)
const errorMsg = ref('')
const editingConfig = ref(null)
const editingVendor = ref(null)
const vendorKeyDraft = ref('')
const isNew = ref(false)

const vendorGroups = computed(() => {
  const map = new Map()
  for (const fm of freeModels.value) {
    const v = fm.vendor || '其他'
    if (!map.has(v)) map.set(v, { vendor: v, items: [], hasKey: false })
    const g = map.get(v)
    g.items.push(fm)
    if (fm.api_key_set) g.hasKey = true
  }
  return Array.from(map.values())
})

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
    freeModels.value = data.free_models || []
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
  editingConfig.value = { ...cfg, api_key: '' }
}
function startEditVendor(grp) {
  editingVendor.value = grp.vendor
  vendorKeyDraft.value = ''
}
function cancelVendorEdit() {
  editingVendor.value = null
  vendorKeyDraft.value = ''
}
async function saveVendorKey(grp) {
  const key = vendorKeyDraft.value
  if (!key || !key.trim()) {
    errorMsg.value = '请输入 API Key'
    return
  }
  errorMsg.value = ''
  const ids = grp.items.map(fm => fm.id)
  const idSet = new Set(ids)
  const untouched = configs.value
    .filter(c => !idSet.has(c.id))
    .map(c => ({ ...c, api_key: MASKED }))
  const vendorEntries = ids.map(id => {
    const fm = grp.items.find(x => x.id === id)
    return {
      id,
      name: grp.vendor,
      endpoint: fm.endpoint,
      api_key: key,
      default_model: fm.model,
      is_default: false
    }
  })
  try {
    await persist([...untouched, ...vendorEntries])
    await loadConfigs()
    editingVendor.value = null
    vendorKeyDraft.value = ''
  } catch (e) {
    errorMsg.value = e.message
  }
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

const chatList = chatModelList
function loadChatList() {
  syncChatModelList()
}
function isInChatList(value) {
  return chatList.value.some(m => m.value === value)
}
function isVendorSelected(vendor) {
  const items = vendorGroups.value.find(g => g.vendor === vendor)?.items || []
  if (items.length === 0) return false
  return items.every(it => isInChatList(it.id))
}
function toggleVendorModels(grp) {
  const entries = grp.items.map(fm => ({ label: fm.name, value: fm.id }))
  const allIn = entries.every(e => isInChatList(e.value))
  let next
  if (allIn) {
    const drop = new Set(entries.map(e => e.value))
    next = chatList.value.filter(m => !drop.has(m.value))
  } else {
    const have = new Set(chatList.value.map(m => m.value))
    next = [...chatList.value]
    for (const e of entries) if (!have.has(e.value)) next.push(e)
  }
  setChatModelList(next)
}

function handleEsc(e) {
  if (e.key === 'Escape') emit('close')
}

onMounted(() => {
  loadConfigs()
  loadChatList()
  document.addEventListener('keydown', handleEsc)
  nextTick(startPreviewLoop)
})
onUnmounted(() => {
  document.removeEventListener('keydown', handleEsc)
  stopPreviewLoop()
})
</script>

<style scoped>
.settings-modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(20, 18, 15, 0.35);
  backdrop-filter: blur(6px);
  -webkit-backdrop-filter: blur(6px);
  z-index: 20000;
  display: flex;
  align-items: center;
  justify-content: center;
}
.settings-modal-card {
  width: 820px;
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
  display: flex; align-items: center; justify-content: center;
  width: 28px; height: 28px; border-radius: 6px; border: none;
  background: transparent; cursor: pointer; color: #6b6b6b;
}
.settings-modal-close:hover { background: #f5f5f5; }
.settings-modal-body {
  display: flex;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}
/* 左侧边栏 */
.settings-sidebar {
  width: 160px;
  flex-shrink: 0;
  border-right: 1px solid #ececec;
  padding: 16px 12px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  background: #fafafa;
  overflow-y: auto;
}
.settings-tab {
  text-align: left;
  padding: 9px 14px;
  font-size: 13.5px;
  font-weight: 600;
  color: #6b6b6b;
  background: transparent;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.15s ease;
}
.settings-tab:hover { background: #f0f0f0; }
.settings-tab.on { color: #fff; background: #c96442; }
/* 右侧内容 */
.settings-content {
  flex: 1;
  min-width: 0;
  overflow-y: auto;
  padding: 18px 22px 22px;
}
.settings-panel { display: block; }

.settings-section-title { font-size: 13.5px; font-weight: 700; color: #1a1a1a; margin-bottom: 4px; }
.settings-section-desc { font-size: 12px; color: #a3a3a3; margin-bottom: 14px; line-height: 1.5; }
.settings-error { font-size: 12px; color: #d94834; padding: 8px 0; }
.settings-loading { font-size: 12.5px; color: #a3a3a3; padding: 8px 0; }

.model-pick-btn {
  flex-shrink: 0; margin-left: auto; padding: 3px 12px; font-size: 12px; font-weight: 600;
  color: #6b6b6b; background: #f0f0f0; border: 1px solid #ddd; border-radius: 999px; cursor: pointer;
  transition: all 0.15s ease;
}
.model-pick-btn:hover { background: #e8e8e8; }
.model-pick-btn.on { color: #fff; background: #c96442; border-color: #c96442; }
.api-config-card { border: 1px solid #e5e5e5; border-radius: 10px; padding: 10px 12px; margin-bottom: 8px; background: #fafafa; }
.api-config-row { display: flex; align-items: center; gap: 8px; }
.api-config-name { font-size: 13px; font-weight: 600; color: #1a1a1a; flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.api-config-default-badge { font-size: 10.5px; font-weight: 600; color: #12b76a; background: rgba(18, 183, 106, 0.12); padding: 2px 8px; border-radius: 999px; flex-shrink: 0; }
.api-config-actions { display: flex; gap: 4px; flex-shrink: 0; }
.api-config-action-btn { font-size: 11.5px; color: #6b6b6b; background: transparent; border: 1px solid #e5e5e5; border-radius: 6px; padding: 3px 8px; cursor: pointer; }
.api-config-action-btn:hover { background: #f0f0f0; }
.api-config-action-btn.danger { color: #d94834; border-color: #f3c9c2; }
.api-config-meta { margin-top: 5px; font-size: 11px; color: #a3a3a3; font-family: "JetBrains Mono", ui-monospace, Menlo, monospace; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.api-config-add-btn {
  display: flex; align-items: center; justify-content: center; gap: 6px;
  padding: 9px 0; border: 1px dashed #d4d4d4; border-radius: 10px;
  color: #6b6b6b; font-size: 12.5px; font-weight: 600; cursor: pointer;
}
.api-config-add-btn:hover { background: #fafafa; border-color: #c4c4c4; }

.api-config-form { border: 1px solid #e5e5e5; border-radius: 10px; padding: 14px; background: #fafafa; }
.api-free-badge { font-size: 10px; font-weight: 700; color: #0d9488; background: #f0fdfa; border: 1px solid #99f6e4; border-radius: 999px; padding: 1px 7px; }
.api-config-card.free { background: #fcfffe; }
.api-config-card.free:first-child { margin-top: 0; }

.vendor-group { margin-bottom: 8px; }
.vendor-group:last-child { margin-bottom: 0; }
.vendor-head {
  display: flex; align-items: center; flex-wrap: wrap; gap: 8px;
  padding: 9px 12px; background: #f4f4f5; border: 1px solid #e5e5e5; border-radius: 10px; user-select: none;
}
.vendor-head:hover { background: #ededee; }
.vendor-name { font-size: 13px; font-weight: 700; color: #1a1a1a; flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.vendor-count { font-size: 10.5px; font-weight: 600; color: #8a8a8a; background: #ffffff; border: 1px solid #e5e5e5; border-radius: 999px; padding: 1px 8px; flex-shrink: 0; }
.vendor-keystate { font-size: 10.5px; font-weight: 600; color: #b0b0b0; flex-shrink: 0; }
.vendor-keystate.on { color: #12b76a; }
.vendor-key-btn { font-size: 11px; font-weight: 600; color: #1a1a1a; background: #ffffff; border: 1px solid #e5e5e5; border-radius: 999px; padding: 3px 10px; cursor: pointer; flex-shrink: 0; }
.vendor-key-btn:hover { background: #f0f0f0; }
.vendor-model-hint { margin-top: 6px; font-size: 11px; color: #9b9b9b; line-height: 1.5; padding-left: 2px; }
.vendor-key-inline { display: flex; align-items: center; gap: 8px; background: #ffffff; border: 1px solid #e5e5e5; border-radius: 10px; padding: 8px 10px; margin-bottom: 8px; }
.vendor-key-input { flex: 1; min-width: 0; font-size: 12.5px; color: #1a1a1a; border: 1px solid #e5e5e5; border-radius: 8px; padding: 6px 10px; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif; }
.vendor-key-input:focus { outline: none; border-color: #c0c0c0; }
.vendor-key-save { font-size: 12px; font-weight: 600; color: #fff; background: #1a1a1a; border: none; border-radius: 8px; padding: 6px 14px; cursor: pointer; flex-shrink: 0; }
.vendor-key-save:hover { background: #333; }
.vendor-key-cancel { font-size: 12px; font-weight: 600; color: #6b6b6b; background: #f4f4f5; border: 1px solid #e5e5e5; border-radius: 8px; padding: 6px 12px; cursor: pointer; flex-shrink: 0; }
.vendor-key-cancel:hover { background: #ededee; }
.api-preset-label { font-size: 11.5px; color: #a3a3a3; margin-right: 2px; }
.api-preset-btn { font-size: 11.5px; font-weight: 600; color: #1a1a1a; background: #ffffff; border: 1px solid #e5e5e5; border-radius: 999px; padding: 4px 10px; cursor: pointer; }
.api-preset-btn:hover { background: #f0f0f0; }

.api-form-field { display: flex; flex-direction: column; gap: 4px; margin-bottom: 10px; }
.api-form-field span { font-size: 11.5px; color: #6b6b6b; font-weight: 600; }
.api-form-field input { font-size: 13px; padding: 7px 10px; border: 1px solid #e5e5e5; border-radius: 8px; background: #ffffff; outline: none; font-family: inherit; }
.api-form-field input:focus { border-color: #c96442; }

.api-form-actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 4px; }
.api-form-btn { font-size: 12.5px; font-weight: 600; padding: 6px 14px; border-radius: 8px; cursor: pointer; border: none; }
.api-form-btn.cancel { background: transparent; border: 1px solid #e5e5e5; color: #6b6b6b; }
.api-form-btn.cancel:hover { background: #f0f0f0; }
.api-form-btn.save { background: #1a1a1a; color: #fff; }
.api-form-btn.save:hover { background: #333333; }

/* 流式渐变参数控件 */
.param-row { display: flex; align-items: center; gap: 12px; padding: 7px 0; }
.param-label { flex-shrink: 0; width: 92px; font-size: 12.5px; color: #1a1a1a; }
.param-range { flex: 1; min-width: 0; height: 4px; accent-color: #c96442; cursor: pointer; }
.param-value { flex-shrink: 0; width: 72px; text-align: right; font-size: 12px; color: #6b6b6b; font-family: "JetBrains Mono", ui-monospace, Menlo, monospace; }
.param-switch { position: relative; display: inline-block; margin-left: auto; cursor: pointer; }
.param-switch input { position: absolute; opacity: 0; width: 0; height: 0; }
.param-switch-track { display: block; width: 38px; height: 22px; border-radius: 999px; background: #d5d5d5; transition: background 0.15s ease; position: relative; }
.param-switch-track::after { content: ''; position: absolute; top: 2px; left: 2px; width: 18px; height: 18px; border-radius: 50%; background: #fff; box-shadow: 0 1px 2px rgba(0,0,0,0.2); transition: transform 0.15s ease; }
.param-switch input:checked + .param-switch-track { background: #c96442; }
.param-switch input:checked + .param-switch-track::after { transform: translateX(16px); }
.param-reset-row { display: flex; justify-content: flex-end; margin-top: 6px; }
.param-reset-btn { padding: 4px 14px; font-size: 12px; color: #6b6b6b; background: #f0f0f0; border: 1px solid #ddd; border-radius: 999px; cursor: pointer; transition: background 0.15s ease; }
.param-reset-btn:hover { background: #e8e8e8; }

/* 实时预览 */
.preview-stage {
  background: #fff;
  border: 1px solid #eee;
  border-radius: 10px;
  padding: 14px;
  min-height: 280px;
  max-height: 360px;
  overflow-y: auto;
  font-size: 14px;
  line-height: 1.75;
  color: #1e293b;
}
.preview-bubble { white-space: pre-wrap; word-break: break-word; }
</style>
