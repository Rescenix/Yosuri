<template>
  <div class="pb-root">
    <!-- 工具栏：后退/前进/刷新 + URL 栏 + 视口切换 + 外部打开 -->
    <div class="pb-toolbar">
      <button class="pb-icon-btn" :disabled="historyIndex <= 0" @click="goBack" title="后退">
        <Icon icon="mdi:arrow-left" width="15" />
      </button>
      <button class="pb-icon-btn" :disabled="historyIndex >= history.length - 1" @click="goForward" title="前进">
        <Icon icon="mdi:arrow-right" width="15" />
      </button>
      <button class="pb-icon-btn" :disabled="!currentUrl" @click="reload" title="刷新">
        <Icon icon="mdi:refresh" width="15" :class="{ 'pb-spin': loading }" />
      </button>
      <div class="pb-url-wrap">
        <Icon icon="mdi:web" width="13" color="#a3a3a3" />
        <input
          v-model="urlInput"
          class="pb-url-input"
          type="text"
          placeholder="输入 URL 后回车"
          spellcheck="false"
          @keydown.enter="navigateTo(urlInput)"
        />
      </div>
      <button class="pb-icon-btn" :class="{ active: viewport === 'mobile' }" @click="viewport = viewport === 'mobile' ? 'desktop' : 'mobile'" title="移动视口 (375px)">
        <Icon icon="mdi:cellphone" width="14" />
      </button>
      <button class="pb-icon-btn" :disabled="!currentUrl" @click="openExternal" title="在新窗口打开">
        <Icon icon="mdi:open-in-new" width="14" />
      </button>
    </div>

    <!-- 空标签页：检测到的本地服务器卡片（仿 Claude Code） -->
    <div v-if="!currentUrl" class="pb-empty">
      <div class="pb-empty-title">本地服务</div>
      <div v-if="serversLoading" class="pb-empty-hint">探测本地端口中…</div>
      <template v-else>
        <div v-for="s in servers" :key="s.port" class="pb-server-card" @click="navigateTo(s.url)">
          <Icon icon="mdi:server-outline" width="15" color="#6b6b6b" />
          <span class="pb-server-name">{{ s.name }}</span>
          <span class="pb-server-port">:{{ s.port }}</span>
          <span class="pb-server-play"><Icon icon="mdi:play" width="15" /></span>
        </div>
        <div v-if="!servers.length" class="pb-empty-hint">没有探测到运行中的本地服务</div>
        <button class="pb-rescan" @click="fetchServers">
          <Icon icon="mdi:refresh" width="13" /> 重新探测
        </button>
      </template>
    </div>

    <!-- 浏览视图：iframe 原生可交互；移动视口时收窄居中 -->
    <div v-else class="pb-viewport" :class="{ mobile: viewport === 'mobile' }">
      <iframe
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
import { ref, onMounted } from 'vue'
import { Icon } from '@iconify/vue'

const servers = ref([])
const serversLoading = ref(true)

// 自维护导航栈：iframe 跨源后退/前进不可控（SecurityError），
// 这里只记录通过 URL 栏 / 服务器卡片发起的导航，iframe 内部的点击跳转仍是原生行为
const history = ref([])
const historyIndex = ref(-1)
const currentUrl = ref('')
const urlInput = ref('')
const frameSrc = ref('')
const frameRef = ref(null)
const loading = ref(false)
const viewport = ref('desktop')
// 同一 URL 重复加载时 iframe 不触发刷新，挂个递增参数占位
let reloadSeq = 0

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
onMounted(fetchServers)

function normalizeUrl(raw) {
  raw = (raw || '').trim()
  if (!raw) return ''
  if (!/^https?:\/\//i.test(raw)) raw = 'http://' + raw
  return raw
}

function loadUrl(url) {
  currentUrl.value = url
  urlInput.value = url
  loading.value = true
  frameSrc.value = url
}

function navigateTo(raw) {
  const url = normalizeUrl(raw)
  if (!url) return
  // 截断前进分支，追加新条目
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

function reload() {
  if (!currentUrl.value) return
  loading.value = true
  reloadSeq++
  const url = new URL(currentUrl.value)
  url.searchParams.set('__pb_reload', String(reloadSeq))
  frameSrc.value = url.toString()
}

function openExternal() {
  if (currentUrl.value) window.open(currentUrl.value, '_blank', 'noopener')
}
</script>

<style scoped>
.pb-root {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  background: #fff;
}

/* ---------- 工具栏 ---------- */
.pb-toolbar {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 6px 8px;
  border-bottom: 1px solid #f0efe9;
  flex-shrink: 0;
}
.pb-icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border: none;
  background: transparent;
  border-radius: 6px;
  color: #6b6b6b;
  cursor: pointer;
  flex-shrink: 0;
}
.pb-icon-btn:hover:not(:disabled) { background: #f0f0f0; }
.pb-icon-btn:disabled { color: #d4d0c8; cursor: default; }
.pb-icon-btn.active { background: #ece9e2; color: #2d2a26; }
.pb-url-wrap {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 5px;
  background: #f7f6f2;
  border: 1px solid #e5e5e5;
  border-radius: 999px;
  padding: 3px 10px;
}
.pb-url-input {
  flex: 1;
  min-width: 0;
  border: none;
  outline: none;
  background: transparent;
  font-size: 12px;
  color: #2d2a26;
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
}

/* ---------- 空标签页 ---------- */
.pb-empty {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 18px 14px;
}
.pb-empty-title {
  font-size: 11.5px;
  font-weight: 600;
  color: #a3a3a3;
  letter-spacing: 0.05em;
  margin-bottom: 8px;
}
.pb-empty-hint {
  font-size: 12px;
  color: #a3a3a3;
  padding: 8px 0;
}
.pb-server-card {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 9px 12px;
  margin-bottom: 6px;
  border: 1px solid #e5e5e5;
  border-radius: 10px;
  background: #fff;
  cursor: pointer;
  transition: border-color 0.12s, box-shadow 0.12s;
}
.pb-server-card:hover {
  border-color: #c4bcae;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.04);
}
.pb-server-name {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  color: #2d2a26;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.pb-server-port {
  font-size: 12px;
  color: #8a8378;
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
}
.pb-server-play {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border-radius: 999px;
  background: #1a1a1a;
  color: #fff;
  flex-shrink: 0;
}
.pb-rescan {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin-top: 6px;
  font-size: 11.5px;
  color: #8a8378;
  background: transparent;
  border: 1px solid #e5e5e5;
  border-radius: 999px;
  padding: 4px 10px;
  cursor: pointer;
}
.pb-rescan:hover { background: #f7f6f2; }

/* ---------- 浏览视图 ---------- */
.pb-viewport {
  flex: 1;
  min-height: 0;
  position: relative;
  display: flex;
  justify-content: center;
  background: #fff;
}
.pb-viewport.mobile {
  background: #f0efe9;
  padding: 10px 0;
}
.pb-frame {
  border: none;
  width: 100%;
  height: 100%;
}
.pb-viewport.mobile .pb-frame {
  width: 375px;
  border: 1px solid #d4d0c8;
  border-radius: 12px;
  background: #fff;
}
.pb-loading-bar {
  position: absolute;
  top: 0;
  left: 0;
  height: 2px;
  width: 100%;
  background: linear-gradient(90deg, transparent, #c96442, transparent);
  background-size: 50% 100%;
  background-repeat: no-repeat;
  animation: pb-slide 1s linear infinite;
}
@keyframes pb-slide {
  from { background-position: -100% 0; }
  to { background-position: 200% 0; }
}
.pb-spin { animation: pb-rotate 0.9s linear infinite; }
@keyframes pb-rotate {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
</style>
