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
        <button class="update-modal-btn ghost" type="button" @click="onSkip" :disabled="dlState === 'downloading'">跳过此版本</button>
        <button
          v-if="dlState === 'done'"
          class="update-modal-btn primary"
          type="button"
          disabled
        >已就绪，下次启动生效</button>
        <button
          v-else-if="dlState === 'downloading'"
          class="update-modal-btn primary dl-progress-btn"
          type="button"
          disabled
        >
          <span class="dl-progress-fill" :style="{ width: dlPercent + '%' }"></span>
          <span class="dl-progress-text">{{ dlPercentText }}</span>
        </button>
        <button
          v-else
          class="update-modal-btn primary"
          type="button"
          disabled
        >安装包未就绪</button>
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

const dlState = ref('downloading') // downloading | done | error
const dlError = ref('')
// 下载进度（后端 /api/update/download/status 返回 percent 0~100；下载完自动应用在下次启动）
const dlPercent = ref(0)
const dlPercentText = computed(() => {
  const p = Math.round(dlPercent.value)
  if (p <= 0) return '正在下载…'
  if (p >= 100) return '解压安装包…'
  return `下载中 ${p}%`
})
let dlTimer = null

const renderedNotes = computed(() => renderMarkdown(props.update.release_notes || ''))

function formatDate(iso) {
  if (!iso) return ''
  const d = new Date(iso)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

// 记住跳过该版本，以后启动不再提示（新版本发布后重新提示）。
// 同时删除已下载的待应用热补丁 exe——否则下次启动会被自动应用（2026-08-13）。
function onSkip() {
  setSkippedVersion(props.update.latest_version)
  fetch('/api/update/pending', { method: 'DELETE' }).catch(() => {})
  emit('close')
}

// 挂载时只查询后台下载状态（下载由启动时 App.vue 静默完成，弹窗不再触发下载，2026-08-16 用户定稿）
onMounted(() => {
  refreshStatus()
})

async function refreshStatus() {
  try {
    const r = await fetch('/api/update/download/status')
    const d = await r.json().catch(() => ({}))
    if (d.state === 'done') {
      dlState.value = 'done'
    } else if (d.state === 'downloading') {
      dlState.value = 'downloading'
      if (typeof d.percent === 'number') dlPercent.value = d.percent
      pollStatus()
    } else if (d.state === 'error') {
      dlState.value = 'error'
      dlError.value = d.error || '下载失败'
    } else {
      // idle：后台未下载/重启后磁盘无补丁 → 不提供下载按钮，提示未就绪
      dlState.value = 'error'
      dlError.value = '安装包未就绪，请下次启动时自动下载'
    }
  } catch {
    dlState.value = 'error'
    dlError.value = '安装包未就绪'
  }
}

async function startDownload() {
  dlError.value = ''
  dlState.value = 'downloading'
  try {
    const res = await fetch('/api/update/download', { method: 'POST' })
    const d = await res.json().catch(() => ({}))
    if (!res.ok) throw new Error(d.error || `触发失败 (${res.status})`)
    if (d.state === 'done') {
      dlState.value = 'done'
      return
    }
  } catch (err) {
    dlState.value = 'error'
    dlError.value = err.message || '下载失败，请检查网络'
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
        if (typeof d.percent === 'number') dlPercent.value = d.percent
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
/* 下载进度涂黑按钮：用填充色从左到右表示真实下载进度（2026-08-28 用户定稿） */
.dl-progress-btn {
  position: relative;
  overflow: hidden;
  min-width: 130px;
}
.dl-progress-btn .dl-progress-fill {
  position: absolute;
  inset: 0;
  right: auto;
  background: rgba(0, 0, 0, 0.28); /* 已下载部分涂黑 */
  transition: width 0.3s ease;
  pointer-events: none;
  border-radius: inherit;
}
.dl-progress-btn .dl-progress-text {
  position: relative;
  z-index: 1;
  white-space: nowrap;
}
.update-modal-dlerr {
  margin: 0 20px 14px;
  color: #e5484d;
  font-size: 12px;
  text-align: right;
}
.update-progress-mask {
  position: fixed;
  inset: 0;
  background: rgba(20, 18, 15, 0.45);
  backdrop-filter: blur(4px);
  z-index: 20020;
  display: flex;
  align-items: center;
  justify-content: center;
}
.update-progress-card {
  width: 320px;
  background: var(--app-surface);
  border-radius: 14px;
  padding: 24px 24px 20px;
  box-shadow: var(--app-shadow);
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.update-progress-title {
  font-size: 14.5px;
  font-weight: 700;
  color: var(--app-text);
}
.update-progress-track {
  height: 6px;
  border-radius: 3px;
  background: var(--app-surface-3);
  overflow: hidden;
}
.update-progress-bar {
  width: 40%;
  height: 100%;
  border-radius: 3px;
  background: var(--app-accent);
  animation: update-progress-slide 1.1s ease-in-out infinite;
}
@keyframes update-progress-slide {
  0% { margin-left: -40%; }
  100% { margin-left: 100%; }
}
.update-progress-hint {
  font-size: 11.5px;
  color: var(--app-text-faint);
}
</style>
