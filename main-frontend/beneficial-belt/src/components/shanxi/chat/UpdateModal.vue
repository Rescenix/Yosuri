<template>
  <div class="update-modal-backdrop" @keydown.esc="$emit('close')">
    <div class="update-modal-card" role="dialog" aria-label="发现新版本">
      <div class="update-modal-header">
        <div class="update-modal-title">
          <span class="update-modal-badge">NEW</span>
          发现新版本 {{ update.latest_version }}
        </div>
        <button class="update-modal-close" type="button" title="稍后再说" aria-label="关闭" @click="$emit('close')">×</button>
      </div>

      <div class="update-modal-meta">
        <span>当前版本：v{{ update.current_version }}</span>
        <span v-if="update.published_at">发布于 {{ formatDate(update.published_at) }}</span>
      </div>

      <div class="update-modal-body">
        <div v-if="update.release_notes" class="update-modal-notes" v-html="renderedNotes" />
        <div v-else class="update-modal-notes-empty">本次更新没有附带更新说明。</div>
      </div>

      <div class="update-modal-footer">
        <button class="update-modal-btn ghost" type="button" @click="$emit('close')">稍后再说</button>
        <button class="update-modal-btn ghost" type="button" @click="onSkip" :disabled="downloading">跳过此版本</button>
        <button
          v-if="dlState === 'done'"
          class="update-modal-btn primary"
          type="button"
          :disabled="opening"
          @click="onInstall"
        >{{ opening ? '正在启动…' : '一键安装' }}</button>
        <button
          v-else-if="dlState === 'downloading'"
          class="update-modal-btn primary"
          type="button"
          disabled
        >正在准备安装包…</button>
        <button
          v-else
          class="update-modal-btn primary"
          type="button"
          @click="onInstall"
        >{{ dlState === 'error' ? '重试下载' : '开始下载' }}</button>
      </div>
      <div v-if="dlState === 'error'" class="update-modal-dlerr">{{ dlError }}</div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { renderMarkdown } from './markdownRenderer.js'
import { setSkippedVersion } from '../../../composables/updatePrefs.js'

const props = defineProps({
  update: { type: Object, required: true }
})
const emit = defineEmits(['close'])

const opening = ref(false)
const dlState = ref('downloading') // downloading | done | error
const dlError = ref('')
let dlTimer = null

const renderedNotes = computed(() => renderMarkdown(props.update.release_notes || ''))

function formatDate(iso) {
  if (!iso) return ''
  const d = new Date(iso)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

// 记住跳过该版本，以后启动不再提示（新版本发布后重新提示）
function onSkip() {
  setSkippedVersion(props.update.latest_version)
  emit('close')
}

// 挂载时若安装包未就绪则补齐下载（App.vue 启动时已静默预下载，此处兜底）
onMounted(() => {
  startDownload()
})

async function startDownload() {
  dlError.value = ''
  try {
    const res = await fetch('/api/update/download', { method: 'POST' })
    if (!res.ok) throw new Error('触发失败')
    const d = await res.json()
    if (d.state === 'done') {
      dlState.value = 'done'
      return
    }
  } catch {
    dlState.value = 'error'
    dlError.value = '下载失败，请检查网络'
    return
  }
  pollStatus()
}

function pollStatus() {
  if (dlTimer) clearInterval(dlTimer)
  dlTimer = setInterval(async () => {
    try {
      const res = await fetch('/api/update/download/status')
      if (!res.ok) return
      const d = await res.json()
      if (d.state === 'downloading') {
        dlState.value = 'downloading'
      } else if (d.state === 'done') {
        dlState.value = 'done'
        clearInterval(dlTimer)
        dlTimer = null
      } else if (d.state === 'error') {
        dlState.value = 'error'
        dlError.value = d.error || '下载失败'
        clearInterval(dlTimer)
        dlTimer = null
      }
    } catch { /* 轮询失败忽略 */ }
  }, 1500)
}

// 一键安装：直接拉起本地已下载的安装程序
async function onInstall() {
  opening.value = true
  try {
    const res = await fetch('/api/update/install', { method: 'POST' })
    if (res.ok) {
      emit('close')
    } else {
      const d = await res.json().catch(() => ({}))
      dlError.value = d.error || '启动安装失败'
      dlState.value = 'error'
    }
  } catch {
    dlError.value = '启动安装失败'
    dlState.value = 'error'
  } finally {
    opening.value = false
  }
}

onUnmounted(() => {
  if (dlTimer) clearInterval(dlTimer)
})
</script>

<style scoped>
.update-modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(20, 18, 15, 0.35);
  backdrop-filter: blur(6px);
  -webkit-backdrop-filter: blur(6px);
  z-index: 20010;
  display: flex;
  align-items: center;
  justify-content: center;
}
.update-modal-card {
  width: 560px;
  max-width: calc(100vw - 48px);
  max-height: min(70vh, 620px);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: var(--app-surface);
  border-radius: 16px;
  box-shadow: var(--app-shadow);
}
.update-modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px 10px;
  flex-shrink: 0;
}
.update-modal-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 15px;
  font-weight: 700;
  color: var(--app-text);
}
.update-modal-badge {
  padding: 2px 7px;
  border-radius: 999px;
  font-size: 10px;
  font-weight: 750;
  letter-spacing: 0.4px;
  color: #fff;
  background: var(--app-accent);
}
.update-modal-close {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 6px;
  border: none;
  background: transparent;
  cursor: pointer;
  color: var(--app-text-soft);
  font-size: 18px;
  line-height: 1;
}
.update-modal-close:hover { background: var(--app-surface-3); color: var(--app-text); }
.update-modal-meta {
  display: flex;
  gap: 14px;
  padding: 0 20px 10px;
  color: var(--app-text-faint);
  font-size: 11.5px;
}
.update-modal-body {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 2px 20px 14px;
  border-top: 1px solid var(--app-border-soft);
}
.update-modal-notes {
  padding-top: 12px;
  color: var(--app-text-soft);
  font-size: 12.5px;
  line-height: 1.65;
  word-break: break-word;
}
.update-modal-notes :deep(h1),
.update-modal-notes :deep(h2),
.update-modal-notes :deep(h3) {
  margin: 12px 0 6px;
  font-size: 13.5px;
  color: var(--app-text);
}
.update-modal-notes :deep(h1:first-child),
.update-modal-notes :deep(h2:first-child),
.update-modal-notes :deep(h3:first-child) { margin-top: 0; }
.update-modal-notes :deep(p) { margin: 6px 0; }
.update-modal-notes :deep(ul),
.update-modal-notes :deep(ol) { margin: 6px 0; padding-left: 20px; }
.update-modal-notes :deep(code) {
  padding: 1px 5px;
  border-radius: 5px;
  background: var(--app-code-bg);
  font-family: var(--app-font);
  font-size: 11.5px;
}
.update-modal-notes :deep(pre) {
  padding: 10px 12px;
  border-radius: 8px;
  background: var(--app-code-bg);
  overflow: auto;
}
.update-modal-notes :deep(pre code) { padding: 0; background: none; }
.update-modal-notes :deep(a) { color: var(--app-accent); }
.update-modal-notes-empty {
  padding-top: 14px;
  color: var(--app-text-faint);
  font-size: 12.5px;
}
.update-modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 12px 20px 16px;
  border-top: 1px solid var(--app-border-soft);
  flex-shrink: 0;
}
.update-modal-btn {
  min-height: 32px;
  padding: 5px 16px;
  border-radius: 8px;
  border: 1px solid transparent;
  font: inherit;
  font-size: 12.5px;
  font-weight: 600;
  cursor: pointer;
}
.update-modal-btn.ghost {
  color: var(--app-text-soft);
  background: transparent;
  border-color: var(--app-border);
}
.update-modal-btn.ghost:hover { background: var(--app-surface-3); color: var(--app-text); }
.update-modal-btn.primary {
  color: #fff;
  background: var(--app-accent);
  border-color: var(--app-accent);
}
.update-modal-btn.primary:hover { background: var(--app-accent-hover); }
.update-modal-btn.primary:disabled { cursor: default; opacity: 0.6; }
.update-modal-dlerr {
  margin: 0 20px 14px;
  color: #e5484d;
  font-size: 12px;
  text-align: right;
}
</style>
