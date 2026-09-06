<template>
  <!-- 可拖动的折角导航：默认 ┐ 形，避开输入区并压缩右下角占位。 -->
  <nav
    class="app-tool-rail"
    :class="[`is-${railLayout}`, { dragging: railDragging, compact: railItems.length <= 4 }]"
    :style="railPositionStyle"
  >
    <button
      v-for="item in railItems"
      :key="item.id"
      class="app-tool-btn"
      :class="[{ active: item.to === route.path, 'agg-api-shortcut': item.id === 'agg' }, { 'is-dragging-item': railItemDragging === item.id }]"
      type="button"
      :title="item.label"
      :aria-label="item.label"
      draggable="true"
      @click="activateRailItem(item)"
      @dragstart="onRailItemDragStart($event, item.id)"
      @dragover.prevent
      @drop.prevent="onRailItemDrop(item.id)"
      @dragend="onRailItemDragEnd"
    >
      <Icon :icon="item.icon" width="16" />
    </button>
    <button class="app-tool-rail-edit" type="button" title="编辑导航" aria-label="编辑导航" @click="railEditorOpen = !railEditorOpen">
      <Icon icon="mdi:pencil-outline" width="15" />
    </button>
    <button
      class="app-tool-rail-grip"
      type="button"
      title="拖动导航"
      aria-label="拖动导航"
      @pointerdown.stop.prevent="onRailGripPointerDown"
    >
      <Icon icon="mdi:drag-vertical" width="15" />
    </button>
  </nav>
  <div v-if="railEditorOpen" class="rail-editor" @click.stop>
    <div class="rail-editor-head"><strong>编辑导航</strong><button type="button" title="关闭" @click="railEditorOpen = false"><Icon icon="mdi:close" width="15" /></button></div>
    <p>直接拖动图标可调整顺序。</p>
    <div class="rail-editor-list">
      <div v-for="item in railItems" :key="item.id" class="rail-editor-item">
        <span><Icon :icon="item.icon" width="15" />{{ item.label }}</span>
        <button type="button" title="移除" @click="removeRailItem(item.id)"><Icon icon="mdi:close" width="15" /></button>
      </div>
    </div>
    <div v-if="availableRailItems.length" class="rail-editor-add">
      <span>添加入口</span>
      <button v-for="item in availableRailItems" :key="item.id" type="button" :title="`添加${item.label}`" @click="addRailItem(item.id)"><Icon :icon="item.icon" width="15" />{{ item.label }}</button>
    </div>
  </div>
  <SettingsModal v-if="showAggApi" default-tab="aggapi" @close="showAggApi = false" />
          <router-view />
          <UpdateModal v-if="showUpdate" :update="updateInfo" @close="showUpdate = false" />
    <!-- 顶部轻量更新提示：15s 自动消失，点击才弹全窗（2026-08-17 用户定稿：堵塞弹窗破坏体验） -->
    <button v-if="showUpdateBanner" class="update-banner" type="button" @click="openUpdateModal">
      <span class="update-banner-dot" />
      <span>发现新版本 <b>{{ updateInfo && updateInfo.latest_version }}</b>，点击查看</span>
      <span class="update-banner-arrow">›</span>
    </button>
    <!-- 升级完成提示：alpha 补丁启动时静默自动应用后，下一次启动显示（2026-08-18 用户定稿） -->
    <div v-if="showUpdatedBanner" class="updated-banner">
      <span class="updated-banner-check">✓</span>
      <span>已更新到 <b>{{ updatedVersion }}</b></span>
      <button class="updated-banner-close" type="button" @click="closeUpdatedBanner" aria-label="关闭">×</button>
    </div>
    <!-- 登录/注册成功提示：欢迎回来横幅，和升级完成横幅同款样式/15s 自动消失（2026-08-20 用户定稿） -->
    <div v-if="showWelcomeBanner" class="updated-banner">
      <span class="updated-banner-check">✓</span>
      <span>欢迎回来，<b>{{ welcomeName }}</b>！</span>
      <button class="updated-banner-close" type="button" @click="closeWelcomeBanner" aria-label="关闭">×</button>
    </div>
    <!-- 全局悬浮剪贴板卡片：输入框右键 剪切/复制/粘贴/全选 + 选中文本悬浮复制按钮（2026-08-27） -->
    <DesktopFloatingMenu />
  </template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Icon } from '@iconify/vue'
import { useAuth } from './composables/useAuth.js'
import { getSkippedVersion, isUpdateNotifyDisabled, shouldShowUpdateBanner, markUpdateBannerShown } from './composables/updatePrefs.js'
import UpdateModal from './components/shanxi/chat/UpdateModal.vue'
import DesktopFloatingMenu from './components/shanxi/chat/DesktopFloatingMenu.vue'
import SettingsModal from './components/shanxi/chat/SettingsModal.vue'

const auth = useAuth()
const route = useRoute()
const router = useRouter()
const showUpdate = ref(false)
const updateInfo = ref(null)
// 聚合 API 快捷入口：点开直接跳到设置弹窗的「聚合 API」tab。
const showAggApi = ref(false)
const RAIL_POSITION_KEY = 'app_tool_rail_position_v1'
const RAIL_ITEMS_KEY = 'app_tool_rail_items_v1'
const railItemDefinitions = [
  { id: 'chat', label: '编码', icon: 'mdi:code-tags', to: '/chat' },
  { id: 'bio', label: '生命建模', icon: 'mdi:dna', to: '/bio' },
  { id: 'company', label: 'Agent 公司', icon: 'mdi:domain', to: '/company' },
  { id: 'sites', label: '站点', icon: 'mdi:web', to: '/sites' },
  { id: 'publish', label: '网文创作', icon: 'mdi:book-open-page-variant-outline', to: '/publish' },
  { id: 'comic', label: '漫画创作', icon: 'mdi:brush', to: '/comic' },
  { id: 'studio', label: '视频剪辑', icon: 'mdi:movie-edit-outline', to: '/studio' },
  { id: 'agg', label: '聚合 API', icon: 'mdi:api' }
]
const railItems = ref([...railItemDefinitions])
const railEditorOpen = ref(false)
const railItemDragging = ref('')
const availableRailItems = computed(() => railItemDefinitions.filter(option => !railItems.value.some(item => item.id === option.id)))
const railLayout = ref('elbow')
const railPosition = ref(null)
const railDragging = ref(false)
let railDrag = null
const railPositionStyle = computed(() => railPosition.value
  ? { left: `${railPosition.value.x}px`, top: `${railPosition.value.y}px`, right: 'auto', bottom: 'auto' }
  : {})
function saveRailItems() {
  localStorage.setItem(RAIL_ITEMS_KEY, JSON.stringify(railItems.value.map(item => item.id)))
}
function activateRailItem(item) {
  if (item.to) router.push(item.to)
  else if (item.id === 'agg') showAggApi.value = true
}
function onRailItemDragStart(event, id) {
  railItemDragging.value = id
  event.dataTransfer.effectAllowed = 'move'
  event.dataTransfer.setData('text/plain', id)
}
function onRailItemDrop(targetId) {
  const sourceId = railItemDragging.value
  if (!sourceId || sourceId === targetId) return
  const sourceIndex = railItems.value.findIndex(item => item.id === sourceId)
  const targetIndex = railItems.value.findIndex(item => item.id === targetId)
  if (sourceIndex < 0 || targetIndex < 0) return
  const next = [...railItems.value]
  const [moved] = next.splice(sourceIndex, 1)
  next.splice(targetIndex, 0, moved)
  railItems.value = next
  saveRailItems()
}
function onRailItemDragEnd() { railItemDragging.value = '' }
function removeRailItem(id) {
  railItems.value = railItems.value.filter(item => item.id !== id)
  saveRailItems()
}
function addRailItem(id) {
  const item = railItemDefinitions.find(option => option.id === id)
  if (!item || railItems.value.some(current => current.id === id)) return
  railItems.value = [...railItems.value, item]
  saveRailItems()
}
function onRailGripPointerDown(event) {
  const rail = event.currentTarget.closest('.app-tool-rail')
  if (!rail) return
  const rect = rail.getBoundingClientRect()
  railDrag = { pointerId: event.pointerId, startX: event.clientX, startY: event.clientY, x: rect.left, y: rect.top, moved: false }
  railDragging.value = true
  event.currentTarget.setPointerCapture?.(event.pointerId)
  window.addEventListener('pointermove', onRailPointerMove)
  window.addEventListener('pointerup', onRailPointerUp, { once: true })
}
function onRailPointerMove(event) {
  if (!railDrag || event.pointerId !== railDrag.pointerId) return
  const dx = event.clientX - railDrag.startX
  const dy = event.clientY - railDrag.startY
  if (Math.abs(dx) + Math.abs(dy) > 4) railDrag.moved = true
  const rail = document.querySelector('.app-tool-rail')
  const width = rail?.offsetWidth || 48
  const height = rail?.offsetHeight || 48
  railPosition.value = {
    x: Math.max(8, Math.min(window.innerWidth - width - 8, railDrag.x + dx)),
    y: Math.max(8, Math.min(window.innerHeight - height - 8, railDrag.y + dy))
  }
}
function onRailPointerUp(event) {
  if (!railDrag || event.pointerId !== railDrag.pointerId) return
  window.removeEventListener('pointermove', onRailPointerMove)
  railDragging.value = false
  const moved = railDrag.moved
  railDrag = null
  if (moved && railPosition.value) localStorage.setItem(RAIL_POSITION_KEY, JSON.stringify(railPosition.value))
}
// 顶部轻量更新横幅：检测到新安装包已就绪时显示 15s，点击才弹全窗（2026-08-17 用户定稿）
const showUpdateBanner = ref(false)
let updateBannerTimer = null
// 待显示的更新通知：应用在后台时检测到新版本，等用户切回来再弹（2026-08-28）
const pendingUpdateBanner = ref(null) // 存 latest_version 字符串，用于 visibilitychange 检测
// 升级完成横幅：alpha 补丁静默自动应用后显示「已更新到 vX」15s（2026-08-18 用户定稿）
const showUpdatedBanner = ref(false)
const updatedVersion = ref('')
let updatedBannerTimer = null
let updateCheckTimer = null
// 登录/注册欢迎横幅：SessionMenuContent 登录/注册成功后广播 auth-welcome，
// 这里监听并显示 15s（2026-08-20 用户定稿：注册成功即直接登录，需要一个提示告诉用户成功了）
const showWelcomeBanner = ref(false)
const welcomeName = ref('')
let welcomeBannerTimer = null

// 检测到新版本：页面在前台直接弹，后台则挂起等用户切回来（2026-08-28 用户定稿：
// 后台弹横幅=用户切回来早消失了，等于没提示）
function showBanner() {
  if (typeof document !== 'undefined' && document.visibilityState !== 'visible') {
    pendingUpdateBanner.value = updateInfo.value ? updateInfo.value.latest_version : true
    return
  }
  showUpdateBanner.value = true
  updateBannerTimer = setTimeout(() => {
    showUpdateBanner.value = false
    updateBannerTimer = null
  }, 15000)
}

// 应用回到前台时，把挂起的更新通知弹出来（一次性）
function onWindowVisible() {
  if (pendingUpdateBanner.value) {
    pendingUpdateBanner.value = null
    showBanner()
  }
}

function closeUpdatedBanner() {
  clearTimeout(updatedBannerTimer)
  updatedBannerTimer = null
  showUpdatedBanner.value = false
}

function closeWelcomeBanner() {
  clearTimeout(welcomeBannerTimer)
  welcomeBannerTimer = null
  showWelcomeBanner.value = false
}

function onAuthWelcome(e) {
  const name = e.detail && e.detail.name
  if (!name) return
  welcomeName.value = name
  showWelcomeBanner.value = true
  clearTimeout(welcomeBannerTimer)
  welcomeBannerTimer = setTimeout(closeWelcomeBanner, 15000)
}

function openUpdateModal() {
  clearTimeout(updateBannerTimer)
  updateBannerTimer = null
  showUpdateBanner.value = false
  showUpdate.value = true
}

// 更新检查 + 触发后台下载。
// 2026-08-25 秒弹定稿：检测到新版本 → 立刻弹轻量横幅（同一版本 4 小时节流，
// 一天最多 6 次），同时后台静默下载；安装包就绪后下次启动再确认弹窗。不再等下载完才提示。
async function checkAndDownload(silent) {
  if (isUpdateNotifyDisabled()) return
  let res
  try {
    res = await fetch('/api/update/check')
  } catch { return }
  let data
  try { data = await res.json() } catch { return }
  if (!(data.ok && data.update && data.update.has_update)) return
  if (getSkippedVersion() === data.update.latest_version) return
  updateInfo.value = data.update
  // 秒弹：新版本一被检测到就提示（8 小时节流，同版本一天最多弹 3 次），不等待下载完成
  if (shouldShowUpdateBanner(data.update.latest_version)) {
    markUpdateBannerShown(data.update.latest_version)
    showBanner()
  }
  try {
    const dl = await fetch('/api/update/download', { method: 'POST' })
    const dlData = await dl.json()
    if (dlData.state === 'done') {
      // 安装包已就绪（本次启动前已下好）：横幅已弹过，无需额外动作
      return
    }
    // 下载中：轮询等待完成，完成后静默（下次启动磁盘判断 done → 版本 tab 可装）
    const timer = setInterval(async () => {
      try {
        const r = await fetch('/api/update/download/status')
        const d = await r.json()
        if (d.state === 'done' || d.state === 'error') {
          clearInterval(timer)
        }
      } catch { /* 忽略轮询错误 */ }
    }, 2000)
  } catch {
    // 下载接口不可达：本次不下载，下次再试
  }
}

onBeforeUnmount(() => {
  if (updateBannerTimer) clearTimeout(updateBannerTimer)
  if (updatedBannerTimer) clearTimeout(updatedBannerTimer)
  if (welcomeBannerTimer) clearTimeout(welcomeBannerTimer)
  if (updateCheckTimer) clearInterval(updateCheckTimer)
  window.removeEventListener('pointermove', onRailPointerMove)
  window.removeEventListener('auth-welcome', onAuthWelcome)
  document.removeEventListener('visibilitychange', onWindowVisible)
})

onMounted(() => {
  try {
    // 旧版本会把一次轻点拖拽柄误存为竖排；统一迁回稳定的默认折角布局。
    railLayout.value = 'elbow'
    const savedPosition = JSON.parse(localStorage.getItem(RAIL_POSITION_KEY) || 'null')
    if (savedPosition && Number.isFinite(savedPosition.x) && Number.isFinite(savedPosition.y)) railPosition.value = savedPosition
    const savedItemIDs = JSON.parse(localStorage.getItem(RAIL_ITEMS_KEY) || 'null')
    if (Array.isArray(savedItemIDs)) {
      const seen = new Set()
      railItems.value = savedItemIDs
        .filter(id => typeof id === 'string' && !seen.has(id) && seen.add(id))
        .map(id => railItemDefinitions.find(item => item.id === id))
        .filter(Boolean)
    }
  } catch { /* 本地偏好不可用时使用默认折角布局 */ }
  const params = new URLSearchParams(window.location.search)
  const token = params.get('token')
  if (token) {
    const url = new URL(window.location.href)
    url.searchParams.delete('token')
    window.history.replaceState({}, document.title, url.pathname + url.search)
    localStorage.setItem('token', token)
    window.dispatchEvent(new Event('auth-change'))
  }
  window.addEventListener('auth-welcome', onAuthWelcome)
  // 应用回到前台 → 弹挂起的更新通知（2026-08-28）
  document.addEventListener('visibilitychange', onWindowVisible)
})

onMounted(async () => {
  // 1) 升级完成提示：alpha 补丁静默自动应用后，后端留了一次性标记（读完即删）
  try {
    const r = await fetch('/api/update/last-applied')
    const d = await r.json()
    if (d && d.ok && d.version) {
      updatedVersion.value = d.version
      // 前台直接弹升级完成横幅，后台挂起等用户切回来（2026-08-28）
      if (document.visibilityState === 'visible') {
        showUpdatedBanner.value = true
        updatedBannerTimer = setTimeout(closeUpdatedBanner, 15000)
      } else {
        pendingUpdateBanner.value = 'upgraded'
      }
    }
  } catch { /* 标记接口不可达：静默 */ }
  // 2) 更新检查 + 后台下载：启动时一次 + 每 60 秒周期检查
  //    （2026-08-28 用户定稿：官网一更新尽快自动下载；30min→60s，配合后端缓存 TTL 60s，
  //    发布后最多 1 分钟触发下载。update.json 静态小 JSON + CDN 已去缓存，压力可忽略）
  await checkAndDownload(false)
  updateCheckTimer = setInterval(() => checkAndDownload(true), 60 * 1000)
})
</script>

<style>
/* 顶部轻量更新横幅：fixed 顶部居中，不堵塞界面；15s 自动消失，点击弹全窗（2026-08-17） */
.update-banner {
  position: fixed;
  top: 14px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 10001;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  max-width: calc(100vw - 32px);
  padding: 7px 14px;
  border: 1px solid var(--app-border);
  border-radius: 999px;
  background: var(--app-surface-2);
  color: var(--app-text-soft);
  font-size: 12.5px;
  line-height: 1;
  cursor: pointer;
  box-shadow: 0 4px 18px rgba(15, 23, 42, 0.12);
  transition: background 0.15s ease, color 0.15s ease, border-color 0.15s ease;
}
.update-banner:hover { background: var(--app-surface-3); color: var(--app-text); border-color: var(--app-accent); }
.update-banner b { font-weight: 600; color: var(--app-accent); }
.update-banner-dot {
  width: 7px; height: 7px;
  border-radius: 50%;
  background: var(--app-accent);
  flex: none;
  animation: update-banner-pulse 1.2s ease-in-out infinite;
}
.update-banner-arrow { color: var(--app-text-faint); font-size: 14px; }
@keyframes update-banner-pulse {
  0%, 100% { opacity: 1; box-shadow: 0 0 0 0 color-mix(in srgb, var(--app-accent) 35%, transparent); }
  50% { opacity: 0.75; box-shadow: 0 0 0 5px color-mix(in srgb, var(--app-accent) 0%, transparent); }
}
/* 升级完成横幅：alpha 静默自动应用后显示「已更新到 vX」，成功绿色（2026-08-18） */
.updated-banner {
  position: fixed;
  top: 14px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 10001;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  max-width: calc(100vw - 32px);
  padding: 7px 14px;
  border: 1px solid color-mix(in srgb, #22c55e 45%, var(--app-border));
  border-radius: 999px;
  background: var(--app-surface-2);
  color: var(--app-text-soft);
  font-size: 12.5px;
  line-height: 1;
  box-shadow: 0 4px 18px rgba(15, 23, 42, 0.12);
}
.updated-banner b { font-weight: 600; color: #22c55e; }
.updated-banner-check {
  width: 16px; height: 16px;
  border-radius: 50%;
  background: #22c55e;
  color: #fff;
  font-size: 11px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: none;
}
.updated-banner-close {
  border: none;
  background: none;
  color: var(--app-text-faint);
  font-size: 14px;
  line-height: 1;
  cursor: pointer;
  padding: 0 0 0 4px;
}
.updated-banner-close:hover { color: var(--app-text); }
/* 默认 ┐ 折角导航：底边横排 + 右侧上折，既避开聊天输入区，也保留足够命中面积。 */
.app-tool-rail {
  position: fixed;
  right: 30px;
  bottom: 18px;
  z-index: 9999;
  display: grid;
  grid-template-columns: repeat(7, 30px);
  grid-template-rows: repeat(2, 30px);
  align-items: center;
  gap: 2px;
  padding: 4px;
  background: var(--app-surface-2);
  border: 1px solid var(--app-border);
  border-radius: 16px 16px 5px 16px;
  box-shadow: 0 4px 16px rgba(15,23,42,.08);
  touch-action: none;
  transition: border-radius .2s ease, box-shadow .2s ease;
}
.app-tool-rail.dragging { cursor: grabbing; box-shadow: 0 12px 28px rgba(15,23,42,.18); }
.app-tool-rail.is-elbow > .app-tool-btn:nth-child(-n+7) { grid-row: 2; }
.app-tool-rail.is-elbow > .app-tool-btn:nth-child(8) { grid-column: 7; grid-row: 1; }
.app-tool-rail.is-elbow > .app-tool-rail-edit { grid-column: 6; grid-row: 1; }
.app-tool-rail.is-elbow > .app-tool-rail-grip { grid-column: 5; grid-row: 1; }
.app-tool-rail.is-elbow.compact {
  display: flex;
  width: max-content;
  grid-template-columns: none;
  grid-template-rows: none;
  border-radius: 999px;
}
.app-tool-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  width: 30px;
  height: 30px;
  padding: 0;
  border-radius: 9px;
  border: none;
  background: transparent;
  color: var(--app-text-faint);
  font-size: 11.5px;
  text-decoration: none;
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
}
.app-tool-btn:hover { background: var(--app-surface-3); color: var(--app-text-soft); }
.app-tool-btn.active { background: var(--app-surface); color: var(--app-text); font-weight: 600; }
.app-tool-btn.is-dragging-item { opacity: .42; transform: scale(.92); }
.agg-api-shortcut {
  color: var(--app-accent);
  font-weight: 600;
}
.agg-api-shortcut:hover { background: var(--app-surface-3); color: var(--app-accent); }
.app-tool-rail-grip {
  display: grid;
  width: 30px;
  height: 30px;
  place-items: center;
  padding: 0;
  border: 0;
  border-radius: 9px;
  color: var(--app-text-faint);
  background: transparent;
  cursor: grab;
}
.app-tool-rail-grip:hover { color: var(--app-accent); background: var(--app-surface-3); }
.app-tool-rail.dragging .app-tool-rail-grip { cursor: grabbing; }
.app-tool-rail-edit {
  display: grid; width: 30px; height: 30px; place-items: center; padding: 0;
  border: 0; border-radius: 9px; color: var(--app-text-faint); background: transparent; cursor: pointer;
}
.app-tool-rail-edit:hover { color: var(--app-accent); background: var(--app-surface-3); }
.rail-editor {
  position: fixed; right: 30px; bottom: 96px; z-index: 10000; width: 238px; padding: 12px;
  border: 1px solid var(--app-border); border-radius: 14px; color: var(--app-text); background: var(--app-surface);
  box-shadow: 0 18px 42px rgba(15,23,42,.18);
}
.rail-editor-head { display: flex; align-items: center; justify-content: space-between; }
.rail-editor-head strong { font-size: 13px; }
.rail-editor-head button,.rail-editor-item button { display: grid; width: 26px; height: 26px; place-items: center; padding: 0; border: 0; border-radius: 7px; color: var(--app-text-faint); background: transparent; cursor: pointer; }
.rail-editor-head button:hover,.rail-editor-item button:hover { color: #dc2626; background: rgba(220,38,38,.08); }
.rail-editor > p { margin: 6px 0 10px; color: var(--app-text-faint); font-size: 11px; }
.rail-editor-list { display: grid; gap: 3px; max-height: 190px; overflow: auto; }
.rail-editor-item { display: flex; align-items: center; justify-content: space-between; min-height: 32px; padding-left: 8px; border-radius: 8px; background: var(--app-surface-2); font-size: 12px; }
.rail-editor-item span { display: inline-flex; align-items: center; gap: 7px; min-width: 0; }
.rail-editor-add { display: flex; flex-wrap: wrap; gap: 5px; margin-top: 10px; padding-top: 10px; border-top: 1px solid var(--app-border); }
.rail-editor-add > span { width: 100%; color: var(--app-text-faint); font-size: 11px; }
.rail-editor-add button { display: inline-flex; align-items: center; gap: 4px; min-height: 27px; padding: 0 7px; border: 1px solid var(--app-border); border-radius: 7px; color: var(--app-text-soft); background: var(--app-surface); font: inherit; font-size: 11px; cursor: pointer; }
.rail-editor-add button:hover { color: var(--app-accent); border-color: var(--app-accent); }
html:has(.company-view),html:has(.publish-view) { scrollbar-width: thin; scrollbar-color: #aa8fa0 #f4f1f3; }
html:has(.company-view)::-webkit-scrollbar,html:has(.publish-view)::-webkit-scrollbar { width: 9px; }
html:has(.company-view)::-webkit-scrollbar-track,html:has(.publish-view)::-webkit-scrollbar-track { background: #f4f1f3; }
html:has(.company-view)::-webkit-scrollbar-thumb { border: 2px solid #f4f1f3; border-radius: 999px; background: linear-gradient(#73b895,#39775d); }
html:has(.publish-view)::-webkit-scrollbar-thumb { border: 2px solid #f4f1f3; border-radius: 999px; background: linear-gradient(#dba8bc,#aaa4d4); }
html:has(.company-view)::-webkit-scrollbar-thumb:hover { background: linear-gradient(#55a77d,#245f47); }
html:has(.publish-view)::-webkit-scrollbar-thumb:hover { background: linear-gradient(#c77f9d,#8883bf); }
@media (max-width: 620px) {
  .app-tool-rail { right: 22px; bottom: 10px; padding: 3px; gap: 1px; }
  .app-tool-rail.is-elbow { grid-template-columns: repeat(7, 26px); grid-template-rows: repeat(2, 26px); }
  .app-tool-btn,.app-tool-rail-grip,.app-tool-rail-edit { width: 26px; height: 26px; flex-basis: 26px; }
  .rail-editor { right: 10px; bottom: 76px; width: min(238px, calc(100vw - 20px)); }
}
</style>
