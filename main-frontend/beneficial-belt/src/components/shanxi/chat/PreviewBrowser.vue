<template>
  <div class="pb-root">
    <div class="pb-toolbar">
      <div class="pb-nav-group">
        <button class="pb-icon-btn" :disabled="historyIndex <= 0" @click="goBack" title="后退">
          <Icon icon="mdi:arrow-left" width="16" />
        </button>
        <button class="pb-icon-btn" :disabled="historyIndex >= history.length - 1" @click="goForward" title="前进">
          <Icon icon="mdi:arrow-right" width="16" />
        </button>
        <button class="pb-icon-btn" :disabled="!currentUrl" @click="reload" title="刷新">
          <Icon icon="mdi:refresh" width="16" :class="{ 'pb-spin': loading }" />
        </button>
      </div>

      <div class="pb-url-wrap">
        <Icon icon="mdi:web" width="14" class="pb-url-icon" />
        <input
          :value="displayedUrl"
          class="pb-url-input"
          :class="{ 'is-hidden': addressHidden }"
          type="text"
          :placeholder="addressHidden ? '地址已隐藏' : '输入 URL'"
          :readonly="addressHidden"
          spellcheck="false"
          @input="onUrlInput"
          @keydown.enter="navigateTo(urlInput)"
        />
        <button
          class="pb-url-privacy-btn"
          type="button"
          :class="{ active: addressHidden }"
          :title="addressHidden ? '显示地址' : '隐藏地址'"
          :aria-label="addressHidden ? '显示地址' : '隐藏地址'"
          :aria-pressed="addressHidden"
          @click="toggleAddressVisibility"
        >
          <Icon :icon="addressHidden ? 'mdi:eye-off-outline' : 'mdi:eye-outline'" width="15" />
        </button>
      </div>

      <div class="pb-actions">
        <button class="pb-icon-btn" :class="{ active: viewport === 'mobile' }" @click="viewport = viewport === 'mobile' ? 'desktop' : 'mobile'" title="移动视口">
          <Icon icon="mdi:cellphone" width="15" />
        </button>
        <button class="pb-icon-btn" :disabled="!currentUrl" @click="openExternal" title="外部打开">
          <Icon icon="mdi:open-in-new" width="15" />
        </button>
      </div>
    </div>

    <!-- 空态不再摆放大图标+标题的装饰插画（仿 Cursor 的 Browser 面板：空的时候
         就是空的，只有工具栏，不占地方讲一遍"输入 URL 打开页面"）。
         有本地服务时才有内容可看，没有就是纯空白。 -->
    <div v-if="!currentUrl" class="pb-empty-shell">
      <div v-if="filteredServers.length" class="pb-local-section">
        <div class="pb-local-title">本地服务</div>
        <div class="pb-local-list">
          <button v-for="s in filteredServers" :key="s.port" class="pb-local-card" @click="navigateTo(s.url)">
            <span class="pb-local-left">
              <Icon icon="mdi:server-outline" width="15" />
              <span class="pb-local-name">{{ s.name }}</span>
            </span>
            <span class="pb-local-port">:{{ s.port }}</span>
          </button>
        </div>
      </div>
    </div>

    <div v-else class="pb-viewport" :class="{ mobile: viewport === 'mobile' }">
      <!-- CDP 模式：连真实 Chromium 的 target ws，用 startScreencast 把真实渲染画面刷进来。
           取代 iframe —— agent 写的 HTML 由真实浏览器引擎渲染。
           双向：canvas 渲染帧 + 把用户的鼠标/键盘经 cdpSocket 转成 input 打回 Chromium，
           于是用户在面板里就能直接操作 agent 渲染的页面。 -->
      <canvas
        v-if="cdpFrame"
        ref="cdpCanvas"
        class="pb-frame pb-canvas"
        @mousemove="onCanvasMouse('mouseMoved', $event)"
        @mousedown="onCanvasMouse('mousePressed', $event)"
        @mouseup="onCanvasMouse('mouseReleased', $event)"
        @contextmenu.prevent
      ></canvas>
      <div v-else-if="cdpError" class="pb-error" role="alert">
        <Icon icon="mdi:alert-circle-outline" width="28" />
        <span>{{ cdpError }}</span>
      </div>
      <iframe
        v-else-if="!isCDPPreview"
        ref="frameRef"
        :src="frameSrc"
        class="pb-frame"
        sandbox="allow-scripts allow-same-origin allow-forms allow-popups allow-modals allow-downloads"
        @load="loading = false"
      ></iframe>
      <div v-if="loading" class="pb-loading-bar"></div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch, onUnmounted } from 'vue'
import { Icon } from '@iconify/vue'
import { previewRequest } from '../composables/previewBus.js'

const servers = ref([])
const serversLoading = ref(true)
const history = ref([])
const historyIndex = ref(-1)
const currentUrl = ref('')
const urlInput = ref('')
const addressHidden = ref(false)
const frameSrc = ref('')
const frameRef = ref(null)
const loading = ref(false)
const viewport = ref('desktop')
const cdpFrame = ref('')          // CDP screencast 帧(base64 data URL)
const cdpError = ref('')
const isCDPPreview = ref(false)
let cdpSocket = null              // 当前同源 CDP 中转 ws
let currentCDPTarget = ''
let currentCDPUrl = ''
let reloadSeq = 0
const cdpCanvas = ref(null)       // 双向渲染画布（替代 <img>）
const ADDRESS_VISIBILITY_KEY = 'preview-browser-address-hidden'

const displayedUrl = computed(() => {
  if (!addressHidden.value) return urlInput.value
  return urlInput.value ? '地址已隐藏' : ''
})

function isFrontend(s) {
  if (s.category) return s.category === 'frontend'
  return [4322, 4321, 5173, 3001].includes(s.port)
}

const filteredServers = computed(() => servers.value.filter(isFrontend))

async function fetchServers() {
  serversLoading.value = true
  try {
    const res = await fetch('/api/preview/servers')
    const data = await res.json()
    servers.value = data.servers || []
  } catch {
    servers.value = []
  } finally {
    serversLoading.value = false
  }
}

function cdpRelayUrl(targetWS, targetURL) {
  const scheme = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const query = targetWS
    ? `ws=${encodeURIComponent(targetWS)}`
    : `url=${encodeURIComponent(targetURL)}`
  return `${scheme}//${window.location.host}/api/preview/cdp?${query}`
}

// 连接同源后端中转；CDP 命令、帧 ACK 和 9222 连接全部由后端负责。
function connectCDP(wsUrl, targetURL = '') {
  disconnectCDP()
  isCDPPreview.value = true
  currentCDPTarget = wsUrl
  currentCDPUrl = targetURL
  cdpFrame.value = ''
  cdpError.value = ''
  loading.value = true
  try {
    const sock = new WebSocket(cdpRelayUrl(wsUrl, targetURL))
    cdpSocket = sock
    sock.onmessage = (ev) => {
      let m
      try { m = JSON.parse(ev.data) } catch { return }
      if (m.type === 'frame' && m.data) {
        // 双向渲染：把 screencast 帧画到 canvas（不再用 <img>），canvas 才能收交互。
        const img = new Image()
        img.onload = () => {
          const cv = cdpCanvas.value
          if (!cv) return
          // 按帧真实像素设画布尺寸，避免拉伸；CSS 再自适应容器。
          if (cv.width !== img.naturalWidth) cv.width = img.naturalWidth
          if (cv.height !== img.naturalHeight) cv.height = img.naturalHeight
          const ctx = cv.getContext('2d')
          ctx.drawImage(img, 0, 0)
        }
        img.src = 'data:image/png;base64,' + m.data
        cdpFrame.value = m.data
        cdpError.value = ''
        loading.value = false
      } else if (m.type === 'error') {
        cdpError.value = m.message || '预览不可用：Chrome CDP 未运行'
        loading.value = false
      }
    }
    sock.onerror = () => {
      if (cdpSocket !== sock) return
      cdpError.value = '预览不可用：Chrome CDP 未运行'
      loading.value = false
    }
    sock.onclose = () => {
      if (cdpSocket !== sock) return
      cdpSocket = null
      if (!cdpFrame.value && !cdpError.value) {
        cdpError.value = '预览不可用：Chrome CDP 连接已断开'
        loading.value = false
      }
    }
  } catch {
    cdpSocket = null
    cdpError.value = '预览不可用：Chrome CDP 未运行'
    loading.value = false
  }
}

function disconnectCDP() {
  if (cdpSocket) { try { cdpSocket.close() } catch {} cdpSocket = null }
  cdpFrame.value = ''
  cdpError.value = ''
  isCDPPreview.value = false
  currentCDPTarget = ''
  currentCDPUrl = ''
}

onMounted(() => {
  try {
    addressHidden.value = window.localStorage.getItem(ADDRESS_VISIBILITY_KEY) === '1'
  } catch {}
  fetchServers()
  if (previewRequest.url) navigateTo(previewRequest.url, previewRequest.cdp_ws, previewRequest.cdp_error)
  window.addEventListener('keydown', onKeyDown)
  window.addEventListener('keyup', onKeyUp)
})

watch(() => previewRequest.seq, () => {
  if (previewRequest.url) navigateTo(previewRequest.url, previewRequest.cdp_ws, previewRequest.cdp_error)
})

onUnmounted(() => {
  disconnectCDP()
  window.removeEventListener('keydown', onKeyDown)
  window.removeEventListener('keyup', onKeyUp)
})

function hasScheme(raw) {
  return /^[a-z][a-z\d+.-]*:\/\//i.test(raw)
}

function looksLikeLocalAddress(raw) {
  return /^(localhost|127\.0\.0\.1|0\.0\.0\.0|\[::1\]|10\.|192\.168\.|172\.(1[6-9]|2\d|3[0-1])\.)/i.test(raw)
}

function normalizeUrl(raw) {
  raw = (raw || '').trim()
  if (!raw) return ''
  if (hasScheme(raw)) return raw
  if (raw.startsWith('//')) return 'https:' + raw
  if (looksLikeLocalAddress(raw)) return 'http://' + raw
  return 'https://' + raw
}

function onUrlInput(event) {
  if (!addressHidden.value) urlInput.value = event.target.value
}

function toggleAddressVisibility() {
  addressHidden.value = !addressHidden.value
  try {
    window.localStorage.setItem(ADDRESS_VISIBILITY_KEY, addressHidden.value ? '1' : '0')
  } catch {}
}

function navigateTo(raw, cdpWs, cdpStartupError) {
  const url = normalizeUrl(raw)
  if (!url) return
  if (cdpStartupError) {
    currentUrl.value = url
    urlInput.value = url
    // 创建 target 当时失败可能只是 Chrome 尚未就绪；让同源后端按 URL
    // 重新创建一次，失败仍会收到明确 error 消息，成功则直接显示首帧。
    connectCDP('', url)
    return
  }
  // 有 CDP ws → 走真实 Chromium 渲染（内嵌窗口）
  if (cdpWs) {
    currentUrl.value = url
    urlInput.value = url
    loading.value = true
    connectCDP(cdpWs)
    return
  }
  // 否则走原 iframe
  history.value = history.value.slice(0, historyIndex.value + 1)
  history.value.push(url)
  historyIndex.value = history.value.length - 1
  loadUrl(url)
}

function goBack() {
  if (historyIndex.value <= 0) return
  historyIndex.value--
  loadUrl(history.value[historyIndex.value])
}

function goForward() {
  if (historyIndex.value >= history.value.length - 1) return
  historyIndex.value++
  loadUrl(history.value[historyIndex.value])
}

// loadUrl：把地址切进 iframe 模式（CDP 分支不调用它）。
// 原实现被删后内联回来——frameSrc 驱动 <iframe :src>，loading 触发进度条。
function loadUrl(url) {
  disconnectCDP()
  currentUrl.value = url
  urlInput.value = url
  loading.value = true
  frameSrc.value = url
}

function reload() {
  if (!currentUrl.value) return
  if (isCDPPreview.value && (currentCDPTarget || currentCDPUrl)) {
    connectCDP(currentCDPTarget, currentCDPUrl)
    return
  }
  loading.value = true
  reloadSeq++
  try {
    const url = new URL(currentUrl.value)
    url.searchParams.set('__pb_reload', String(reloadSeq))
    frameSrc.value = url.toString()
  } catch {
    frameSrc.value = currentUrl.value
  }
}

// 双向交互：把 canvas 上的鼠标事件经 cdpSocket 转成后端约定的 input 消息打回 Chromium。
// 坐标相对 canvas 显示区，后端再映射回页面坐标（layoutW/H = canvas 显示尺寸，
// viewW/H = Chromium 页面尺寸，这里先按画布实际像素 1:1，后续可由 preview_open 带 pageSize 精化）。
function onCanvasMouse(action, e) {
  if (!cdpSocket || cdpSocket.readyState !== WebSocket.OPEN) return
  const cv = cdpCanvas.value
  if (!cv) return
  const rect = cv.getBoundingClientRect()
  const btn = e.button === 2 ? 'right' : e.button === 1 ? 'middle' : 'left'
  cdpSocket.send(JSON.stringify({
    type: 'input', kind: 'mouse', action,
    x: e.clientX - rect.left, y: e.clientY - rect.top,
    button: btn,
    layoutW: rect.width, layoutH: rect.height,
    viewW: cv.width, viewH: cv.height,
    // 诊断字段：让后端日志里的 dbg[...] 显示真实值，
    // 用于定位「坐标恒 (0,0)」是 rect 异常还是 clientX 异常（见 browser_preview_tool.go:247）。
    dbgRectLeft: Math.round(rect.left), dbgRectTop: Math.round(rect.top),
    dbgClientX: Math.round(e.clientX), dbgClientY: Math.round(e.clientY),
  }))
}

// 键盘：面板聚焦时全局转发（节流靠浏览器自身重复事件即可）。
function onKeyDown(e) {
  if (!cdpSocket || cdpSocket.readyState !== WebSocket.OPEN) return
  cdpSocket.send(JSON.stringify({
    type: 'input', kind: 'key', key: e.key, code: e.code, keyAction: 'keyDown',
  }))
}
function onKeyUp(e) {
  if (!cdpSocket || cdpSocket.readyState !== WebSocket.OPEN) return
  cdpSocket.send(JSON.stringify({
    type: 'input', kind: 'key', key: e.key, code: e.code, keyAction: 'keyUp',
  }))
}

function openExternal() {
  if (currentUrl.value) window.open(currentUrl.value, '_blank', 'noopener')
}
</script>

<style scoped>
/* 全部换成 var(--app-*)：原先又是一份独立的硬编码暖米色，跟主题系统脱节。
   顶部工具栏整体收紧、仿 Cursor 的 Browser 面板——更矮、更素，图标条 + 一条
   贯穿的地址栏，没有多余的圆角卡片感。 */
.pb-root {
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
  background: var(--app-surface);
}

.pb-toolbar {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  border-bottom: 1px solid var(--app-border);
  background: var(--app-surface);
  flex-shrink: 0;
}

.pb-nav-group,
.pb-actions {
  display: inline-flex;
  align-items: center;
  gap: 2px;
}

.pb-icon-btn {
  width: 26px;
  height: 26px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--app-text-faint);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
}

.pb-icon-btn:hover:not(:disabled) {
  background: var(--app-surface-3);
  color: var(--app-text);
}

.pb-icon-btn:disabled {
  color: var(--app-border);
  cursor: default;
}

.pb-icon-btn.active {
  background: var(--app-surface-3);
  color: var(--app-text);
}

.pb-url-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  height: 28px;
  padding: 0 10px;
  border: 1px solid var(--app-border);
  border-radius: 7px;
  background: var(--app-surface-2);
}

.pb-url-icon {
  color: var(--app-text-faint);
  flex-shrink: 0;
}

.pb-url-input {
  flex: 1;
  min-width: 0;
  border: none;
  outline: none;
  background: transparent;
  font-size: 12.5px;
  color: var(--app-text);
}

.pb-url-input.is-hidden {
  color: var(--app-text-faint);
  cursor: default;
  user-select: none;
}

.pb-url-privacy-btn {
  width: 24px;
  height: 24px;
  margin-right: -5px;
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 0;
  border-radius: 5px;
  background: transparent;
  color: var(--app-text-faint);
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
}

.pb-url-privacy-btn:hover,
.pb-url-privacy-btn.active {
  background: var(--app-surface-3);
  color: var(--app-text);
}

.pb-url-privacy-btn:focus-visible {
  outline: 2px solid var(--app-accent);
  outline-offset: 1px;
}

.pb-empty-shell {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  justify-content: center;
  padding: 24px 28px 32px;
}

.pb-local-section {
  max-width: 720px;
  margin: 0 auto;
  width: 100%;
}

.pb-local-title {
  margin-bottom: 10px;
  font-size: 12px;
  font-weight: 600;
  color: var(--app-text-faint);
}

.pb-local-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.pb-local-card {
  width: 100%;
  border: 1px solid var(--app-border);
  border-radius: 10px;
  background: var(--app-surface);
  padding: 10px 12px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: var(--app-text);
  cursor: pointer;
  transition: border-color 0.15s ease, background 0.15s ease;
}

.pb-local-card:hover {
  border-color: var(--app-accent);
  background: var(--app-surface-2);
}

.pb-local-left {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.pb-local-name {
  font-size: 14px;
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.pb-local-port {
  font-size: 12px;
  color: var(--app-text-faint);
  font-family: "JetBrains Mono", ui-monospace, monospace;
}

.pb-viewport {
  flex: 1;
  min-height: 0;
  position: relative;
  display: flex;
  justify-content: center;
  background: var(--app-surface);
}

.pb-viewport.mobile {
  padding: 16px;
  background: var(--app-surface-2);
}

.pb-frame {
  width: 100%;
  height: 100%;
  border: none;
  background: var(--app-surface);
}

.pb-canvas {
  display: block;
  /* 保持 screencast 原始宽高比，完整显示页面并避免绘制缓冲撑开容器。
     contain 下鼠标坐标按画布实际显示区域映射。 */
  width: 100%;
  height: 100%;
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
  cursor: crosshair;
  touch-action: none;
}

.pb-error {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 24px;
  color: var(--app-text-faint);
  font-size: 13px;
  text-align: center;
}

.pb-viewport.mobile .pb-frame {
  width: 390px;
  max-width: 100%;
  border: 1px solid var(--app-border);
  border-radius: 18px;
  box-shadow: 0 18px 36px rgba(0, 0, 0, 0.12);
}

.pb-loading-bar {
  position: absolute;
  top: 0;
  left: 0;
  height: 2px;
  width: 100%;
  background: linear-gradient(90deg, transparent, var(--app-accent), transparent);
  background-size: 50% 100%;
  background-repeat: no-repeat;
  animation: pb-slide 1s linear infinite;
}

.pb-spin {
  animation: pb-rotate 0.9s linear infinite;
}

@keyframes pb-slide {
  from { background-position: -100% 0; }
  to { background-position: 200% 0; }
}

@keyframes pb-rotate {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
</style>
