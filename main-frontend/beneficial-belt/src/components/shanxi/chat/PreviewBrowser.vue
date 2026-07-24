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
          v-model="urlInput"
          class="pb-url-input"
          type="text"
          placeholder="输入 URL"
          spellcheck="false"
          @keydown.enter="navigateTo(urlInput)"
        />
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

    <div v-if="!currentUrl" class="pb-empty-shell">
      <div class="pb-empty-hero">
        <div class="pb-empty-icon">
          <Icon icon="mdi:web" width="32" />
        </div>
        <div class="pb-empty-title">开始浏览</div>
        <div class="pb-empty-subtitle">输入 URL 以打开页面</div>
      </div>

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
import { ref, computed, onMounted, watch } from 'vue'
import { Icon } from '@iconify/vue'
import { previewRequest } from '../composables/previewBus.js'

const servers = ref([])
const serversLoading = ref(true)
const history = ref([])
const historyIndex = ref(-1)
const currentUrl = ref('')
const urlInput = ref('')
const frameSrc = ref('')
const frameRef = ref(null)
const loading = ref(false)
const viewport = ref('desktop')
let reloadSeq = 0

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

onMounted(() => {
  fetchServers()
  if (previewRequest.url) navigateTo(previewRequest.url)
})

watch(() => previewRequest.seq, () => {
  if (previewRequest.url) navigateTo(previewRequest.url)
})

function hasScheme(raw) {
  return /^[a-z][a-z\d+\-.]*:\/\//i.test(raw)
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

function loadUrl(url) {
  currentUrl.value = url
  urlInput.value = url
  loading.value = true
  frameSrc.value = url
}

function navigateTo(raw) {
  const url = normalizeUrl(raw)
  if (!url) return
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
  try {
    const url = new URL(currentUrl.value)
    url.searchParams.set('__pb_reload', String(reloadSeq))
    frameSrc.value = url.toString()
  } catch {
    frameSrc.value = currentUrl.value
  }
}

function openExternal() {
  if (currentUrl.value) window.open(currentUrl.value, '_blank', 'noopener')
}
</script>

<style scoped>
.pb-root {
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
  background: #ffffff;
}

.pb-toolbar {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  border-bottom: 1px solid #ece7df;
  background: #ffffff;
  flex-shrink: 0;
}

.pb-nav-group,
.pb-actions {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.pb-icon-btn {
  width: 30px;
  height: 30px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: #8b857d;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: background 0.18s ease, color 0.18s ease;
}

.pb-icon-btn:hover:not(:disabled) {
  background: #f4f1ec;
  color: #26211d;
}

.pb-icon-btn:disabled {
  color: #d8d3cc;
  cursor: default;
}

.pb-icon-btn.active {
  background: #f4f1ec;
  color: #26211d;
}

.pb-url-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  height: 38px;
  padding: 0 12px;
  border: 1px solid #e8e2d9;
  border-radius: 999px;
  background: #fbfaf8;
}

.pb-url-icon {
  color: #9a9388;
  flex-shrink: 0;
}

.pb-url-input {
  flex: 1;
  min-width: 0;
  border: none;
  outline: none;
  background: transparent;
  font-size: 13px;
  color: #26211d;
  font-family: "SF Pro Text", "Segoe UI", sans-serif;
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

.pb-empty-hero {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  min-height: 360px;
  color: #7c756c;
  text-align: center;
}

.pb-empty-icon {
  width: 64px;
  height: 64px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #7e786f;
  background: #f7f4ef;
  border: 1px solid #ece5dc;
}

.pb-empty-title {
  font-size: 18px;
  font-weight: 600;
  color: #23201c;
}

.pb-empty-subtitle {
  font-size: 13px;
  color: #8e867b;
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
  color: #8a8378;
}

.pb-local-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.pb-local-card {
  width: 100%;
  border: 1px solid #ebe4db;
  border-radius: 14px;
  background: #ffffff;
  padding: 12px 14px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: #24211d;
  cursor: pointer;
  transition: border-color 0.18s ease, box-shadow 0.18s ease, transform 0.18s ease;
}

.pb-local-card:hover {
  border-color: #ddd3c7;
  box-shadow: 0 10px 22px rgba(33, 24, 18, 0.05);
  transform: translateY(-1px);
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
  color: #978f84;
  font-family: "JetBrains Mono", ui-monospace, monospace;
}

.pb-viewport {
  flex: 1;
  min-height: 0;
  position: relative;
  display: flex;
  justify-content: center;
  background: #ffffff;
}

.pb-viewport.mobile {
  padding: 16px;
  background: #f7f4ef;
}

.pb-frame {
  width: 100%;
  height: 100%;
  border: none;
  background: #ffffff;
}

.pb-viewport.mobile .pb-frame {
  width: 390px;
  max-width: 100%;
  border: 1px solid #e6dfd6;
  border-radius: 18px;
  box-shadow: 0 18px 36px rgba(33, 24, 18, 0.08);
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
