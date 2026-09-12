<template>
  <div class="comp-modal-backdrop" @keydown.esc="$emit('close')">
    <div class="comp-modal-card" role="dialog" :aria-label="tr('组件下载')">
      <div class="comp-modal-header">
        <div class="comp-modal-title">
          <Icon icon="mdi:download-circle-outline" width="20" />
          {{ tr('组件下载') }}
        </div>
        <button class="comp-modal-close" type="button" :title="tr('关闭')" :aria-label="tr('关闭')" @click="$emit('close')">×</button>
      </div>

      <div class="comp-modal-hint">{{ tr('这些功能引擎不随安装包捆绑；用到视频、音频处理时在这里一键补齐。') }}</div>

      <div v-if="loading" class="comp-modal-loading">{{ tr('正在检查组件状态…') }}</div>
      <div v-else-if="loadError" class="comp-modal-loading error">{{ loadError }}</div>

      <div v-for="comp in components" :key="comp.id" class="comp-row">
        <div class="comp-row-main">
          <div class="comp-row-name">{{ comp.name }}</div>
          <div class="comp-row-desc">{{ comp.description }}</div>
        </div>
        <div class="comp-row-right">
          <span v-if="comp.on_path" class="comp-badge ok">{{ tr('已就绪（系统）') }}</span>
          <span v-else-if="comp.installed" class="comp-badge ok">{{ tr('已就绪') }}</span>
          <template v-else-if="jobOf(comp.id) && (jobOf(comp.id).state === 'downloading' || jobOf(comp.id).state === 'extracting')">
            <div class="comp-progress">
              <div class="comp-progress-fill" :style="{ width: (jobOf(comp.id).percent || 0) + '%' }"></div>
            </div>
            <span class="comp-progress-text">{{ jobOf(comp.id).state === 'extracting' ? tr('解压中…') : tr('下载中') + ' ' + (jobOf(comp.id).percent || 0) + '%' }}</span>
          </template>
          <button
            v-else
            class="comp-install-btn"
            type="button"
            :disabled="!comp.available"
            :title="comp.available ? tr('下载安装') : tr('当前平台暂不支持自动下载')"
            @click="install(comp.id)"
          >{{ comp.available ? tr('下载') : tr('不支持') }}</button>
        </div>
        <div v-if="jobOf(comp.id) && jobOf(comp.id).state === 'error'" class="comp-row-error">{{ jobOf(comp.id).error }}</div>
      </div>

      <div class="comp-modal-footer">
        <span class="comp-modal-alt">{{ tr('也可以自行安装 ffmpeg 并加入 PATH，应用会自动识别。') }}</span>
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import { Icon } from '@iconify/vue'
import { useI18n, tr } from '../../../composables/useI18n.js'

useI18n()
const emit = defineEmits(['close'])

const components = ref([])
const jobs = ref({})
const loading = ref(true)
const loadError = ref('')
let pollTimer = null

function jobOf(id) {
  return jobs.value[id]
}

async function refresh() {
  try {
    const res = await fetch('/api/components')
    if (!res.ok) throw new Error('HTTP ' + res.status)
    const data = await res.json()
    components.value = data.components || []
    jobs.value = data.jobs || {}
    loading.value = false
    loadError.value = ''
    // 有在途任务时保持轮询
    const active = Object.values(jobs.value).some(j => j.state === 'downloading' || j.state === 'extracting')
    if (active && !pollTimer) {
      pollTimer = setInterval(refresh, 1500)
    } else if (!active && pollTimer) {
      clearInterval(pollTimer)
      pollTimer = null
    }
  } catch (e) {
    loadError.value = tr('组件状态获取失败') + ': ' + e.message
    loading.value = false
  }
}

async function install(id) {
  try {
    const res = await fetch(`/api/components/${id}/install`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ start: true }),
    })
    const data = await res.json()
    if (!res.ok && res.status !== 202) throw new Error(data.error || 'HTTP ' + res.status)
    if (data.job) jobs.value = { ...jobs.value, [data.job.id]: data.job }
    if (!pollTimer) pollTimer = setInterval(refresh, 1500)
    await refresh()
  } catch (e) {
    jobs.value = { ...jobs.value, [id]: { id, state: 'error', error: tr('启动下载失败') + ': ' + e.message } }
  }
}

onMounted(refresh)
onUnmounted(() => { if (pollTimer) clearInterval(pollTimer) })
</script>

<style scoped>
.comp-modal-backdrop {
  position: fixed;
  inset: 0;
  z-index: 10050;
  background: rgba(10, 15, 30, 0.45);
  backdrop-filter: blur(3px);
  display: flex;
  align-items: center;
  justify-content: center;
}
.comp-modal-card {
  width: min(560px, calc(100vw - 48px));
  background: var(--sx-panel-bg, #fff);
  border-radius: 14px;
  box-shadow: 0 18px 60px rgba(10, 20, 50, 0.28);
  padding: 18px 22px 16px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.comp-modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.comp-modal-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 17px;
  font-weight: 800;
  color: var(--sx-text, #232837);
}
.comp-modal-close {
  border: none;
  background: transparent;
  font-size: 22px;
  line-height: 1;
  cursor: pointer;
  color: var(--sx-text-muted, #8a8a8a);
  padding: 2px 6px;
  border-radius: 6px;
}
.comp-modal-close:hover { background: rgba(120, 140, 180, 0.12); }
.comp-modal-hint {
  font-size: 13px;
  color: var(--sx-text-muted, #6b7a99);
  line-height: 1.5;
}
.comp-modal-loading { font-size: 13px; color: var(--sx-text-muted, #6b7a99); padding: 12px 0; }
.comp-modal-loading.error { color: #d0342c; }
.comp-row {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 8px 14px;
  align-items: center;
  padding: 12px 14px;
  border: 1px solid var(--sx-border, #dde6f5);
  border-radius: 10px;
  background: var(--sx-subtle-bg, #f6f9ff);
}
.comp-row-name { font-size: 15px; font-weight: 800; color: var(--sx-text, #232837); }
.comp-row-desc { font-size: 12px; color: var(--sx-text-muted, #6b7a99); margin-top: 2px; line-height: 1.45; }
.comp-row-right { display: flex; flex-direction: column; align-items: flex-end; gap: 6px; min-width: 130px; }
.comp-badge {
  font-size: 12px;
  font-weight: 800;
  padding: 4px 10px;
  border-radius: 999px;
}
.comp-badge.ok { background: #e5f7ec; color: #1e8e4e; }
.comp-progress {
  width: 130px;
  height: 8px;
  border-radius: 4px;
  background: #dbe6f7;
  overflow: hidden;
}
.comp-progress-fill {
  height: 100%;
  background: linear-gradient(90deg, #1950be, #4f8dff);
  border-radius: 4px;
  transition: width 0.4s ease;
}
.comp-progress-text { font-size: 12px; color: var(--sx-text-muted, #6b7a99); }
.comp-install-btn {
  border: none;
  border-radius: 8px;
  padding: 7px 18px;
  font-size: 13px;
  font-weight: 800;
  cursor: pointer;
  color: #fff;
  background: #1950be;
  transition: filter 0.15s ease;
}
.comp-install-btn:hover:not(:disabled) { filter: brightness(1.1); }
.comp-install-btn:disabled { background: #b9c6de; cursor: not-allowed; }
.comp-row-error {
  grid-column: 1 / -1;
  font-size: 12px;
  color: #d0342c;
  line-height: 1.45;
  word-break: break-all;
}
.comp-modal-footer { padding-top: 2px; }
.comp-modal-alt { font-size: 12px; color: var(--sx-text-muted, #8a8a8a); line-height: 1.5; }
</style>
