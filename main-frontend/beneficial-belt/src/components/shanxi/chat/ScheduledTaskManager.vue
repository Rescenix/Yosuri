<template>
  <Teleport to="body">
    <div class="stm-backdrop" @click.self="onBackdrop">
      <div class="stm-card">
        <!-- 标题栏 -->
        <div class="stm-header">
          <h2 class="stm-title">定时任务</h2>
          <button class="stm-close" @click="$emit('close')" title="关闭">
            <Icon icon="mdi:close" width="18" />
          </button>
        </div>

        <p class="stm-desc">管理已排程的定时任务，到点会在右下角弹出系统通知提醒你。</p>

        <div class="stm-body">
          <!-- 加载中 -->
          <div v-if="loading" class="stm-empty">
            <span class="stm-loading-spinner"></span>
            <span>加载中...</span>
          </div>

          <!-- 空状态 -->
          <div v-else-if="!tasks.length" class="stm-empty">
            <Icon icon="mdi:clock-outline" width="36" color="#c5c5c5" />
            <span class="stm-empty-title">还没有定时任务</span>
            <span class="stm-empty-sub">排程一个提示词，到点自动提醒你</span>
          </div>

          <!-- 任务列表 -->
          <div v-else class="stm-list">
            <div v-for="t in tasks" :key="t.id" class="stm-item">
              <div class="stm-item-main">
                <div class="stm-item-top">
                  <span class="stm-item-name">{{ t.name || '定时任务' }}</span>
                  <span class="stm-item-badge" :class="t.enabled ? 'on' : 'off'">
                    {{ t.enabled ? '开启' : '关闭' }}
                  </span>
                </div>
                <div class="stm-item-prompt">{{ t.prompt }}</div>
                <div class="stm-item-meta">
                  <span class="stm-item-schedule">🕐 {{ formatSchedule(t) }}</span>
                  <span v-if="t.lastFired" class="stm-item-last">上次触发 {{ formatLast(t.lastFired) }}</span>
                </div>
              </div>
              <div class="stm-item-actions">
                <button
                  class="stm-item-del"
                  :class="{ confirm: confirmId === t.id }"
                  :disabled="deletingId === t.id"
                  @click="onDelete(t)"
                  :title="confirmId === t.id ? '再次点击确认删除' : '删除任务'"
                >
                  <Icon v-if="deletingId !== t.id" icon="mdi:trash-can-outline" width="15" />
                  <span v-else class="stm-spinner"></span>
                  <span>{{ confirmId === t.id ? '确认删除?' : '' }}</span>
                </button>
              </div>
            </div>
          </div>
        </div>

        <div class="stm-footer">
          <button class="stm-btn stm-btn-cancel" @click="$emit('close')">关闭</button>
          <button class="stm-btn stm-btn-primary" @click="$emit('create')">
            <Icon icon="mdi:plus" width="15" />
            新建定时任务
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { Icon } from '@iconify/vue'

const emit = defineEmits(['close', 'create'])

const tasks = ref([])
const loading = ref(true)
const deletingId = ref(null)
const confirmId = ref(null)

// 频率 → 中文描述（与 ScheduledTaskModal 的 FREQ_MAP 保持一致）
const FREQ_TEXT = {
  every_1h: '每小时',
  every_2h: '每 2 小时',
  every_6h: '每 6 小时',
  every_12h: '每 12 小时',
  daily: '每天',
  weekdays: '工作日',
  weekly: '每周',
  monthly: '每月'
}

// 从 cron 表达式里取出 HH:MM（5 段：分 时 日 月 周）
function cronTime(cron) {
  if (!cron) return null
  const f = cron.trim().split(/\s+/)
  if (f.length !== 5) return null
  const h = f[1].replace(/^\*\/?/, '')
  const m = f[0].replace(/^\*\/?/, '')
  if (!/^\d+$/.test(h) || !/^\d+$/.test(m)) return null
  return String(h).padStart(2, '0') + ':' + String(m).padStart(2, '0')
}

function formatSchedule(t) {
  const text = FREQ_TEXT[t.frequency]
  if (text && ['daily', 'weekdays', 'weekly', 'monthly'].includes(t.frequency)) {
    const tm = cronTime(t.cron)
    return text + (tm ? ' ' + tm : '')
  }
  if (text) return text
  // 未知频率直接展示 cron 原文
  return t.cron || '—'
}

function formatLast(iso) {
  if (!iso) return ''
  const d = new Date(iso)
  if (isNaN(d.getTime())) return ''
  const p = n => String(n).padStart(2, '0')
  return `${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

async function load() {
  loading.value = true
  try {
    const r = await fetch('/api/cron/list')
    if (!r.ok) throw new Error('HTTP ' + r.status)
    tasks.value = await r.json()
  } catch (e) {
    console.log('加载定时任务失败:', e)
    tasks.value = []
  } finally {
    loading.value = false
  }
}

function onDelete(t) {
  if (confirmId.value !== t.id) {
    // 第一次点击：进入确认态，3 秒后自动恢复
    confirmId.value = t.id
    setTimeout(() => {
      if (confirmId.value === t.id) confirmId.value = null
    }, 3000)
    return
  }
  confirmId.value = null
  deletingId.value = t.id
  fetch('/api/cron/delete/' + t.id, { method: 'DELETE' })
    .then(r => r.ok ? r.json() : Promise.reject(new Error('HTTP ' + r.status)))
    .then(() => {
      tasks.value = tasks.value.filter(x => x.id !== t.id)
      emit('toast', '🗑️ 定时任务已删除')
    })
    .catch(e => {
      console.log('删除定时任务失败:', e)
      emit('toast', '❌ 删除失败：' + (e.message || '网络错误'))
    })
    .finally(() => { deletingId.value = null })
}

function onBackdrop() {
  // 有删除确认态时点背景不关闭，避免误触
  if (confirmId.value) { confirmId.value = null; return }
  emit('close')
}

onMounted(load)
</script>

<style scoped>
.stm-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.25);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 99999;
}

.stm-card {
  width: 440px;
  max-width: 90vw;
  background: #fff;
  border-radius: 14px;
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.18), 0 2px 8px rgba(0, 0, 0, 0.06);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  position: relative;
  z-index: 100000;
}

.stm-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 18px 20px 0;
}
.stm-title {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
  color: #1a1a1a;
  line-height: 1.3;
}
.stm-close {
  flex-shrink: 0;
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: none;
  background: transparent;
  border-radius: 6px;
  color: #888;
  cursor: pointer;
}
.stm-close:hover { background: #f0f0f0; color: #333; }

.stm-desc {
  margin: 8px 20px 0;
  font-size: 12px;
  line-height: 1.5;
  color: #888;
}

.stm-body {
  padding: 16px 20px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  max-height: 46vh;
  overflow-y: auto;
}

/* 空状态 / 加载中 */
.stm-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 34px 0;
  color: #999;
  font-size: 13px;
}
.stm-empty-title { font-size: 14px; font-weight: 600; color: #555; margin-top: 4px; }
.stm-empty-sub { font-size: 12px; color: #aaa; }
.stm-loading-spinner {
  width: 18px;
  height: 18px;
  border: 2px solid #e5e5e5;
  border-top-color: #3b82f6;
  border-radius: 50%;
  animation: stm-rotate 0.8s linear infinite;
}
@keyframes stm-rotate { to { transform: rotate(360deg); } }

/* 任务列表 */
.stm-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.stm-item {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px 14px;
  background: #f7f7f8;
  border: 1px solid #ededf0;
  border-radius: 10px;
}
.stm-item-main { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 4px; }
.stm-item-top { display: flex; align-items: center; gap: 8px; }
.stm-item-name {
  font-size: 13px;
  font-weight: 700;
  color: #1a1a1a;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.stm-item-badge {
  flex-shrink: 0;
  font-size: 11px;
  font-weight: 600;
  padding: 1px 7px;
  border-radius: 99px;
}
.stm-item-badge.on { background: #dcfce7; color: #16a34a; }
.stm-item-badge.off { background: #f1f1f1; color: #999; }
.stm-item-prompt {
  font-size: 12px;
  color: #666;
  line-height: 1.5;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.stm-item-meta {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.stm-item-schedule { font-size: 12px; font-weight: 600; color: #3b82f6; }
.stm-item-last { font-size: 11px; color: #aaa; }

.stm-item-actions { flex-shrink: 0; display: flex; align-items: center; }
.stm-item-del {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 6px 8px;
  border: none;
  background: transparent;
  border-radius: 6px;
  color: #bbb;
  font-size: 12px;
  font-family: inherit;
  cursor: pointer;
  transition: all 0.15s ease;
}
.stm-item-del:hover { background: #fee2e2; color: #dc2626; }
.stm-item-del.confirm { background: #dc2626; color: #fff; font-weight: 600; }
.stm-item-del:disabled { opacity: 0.5; cursor: not-allowed; }
.stm-spinner {
  width: 13px;
  height: 13px;
  border: 2px solid rgba(255, 255, 255, 0.4);
  border-top-color: #fff;
  border-radius: 50%;
  animation: stm-rotate 0.8s linear infinite;
}

/* 底部按钮 */
.stm-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 12px 20px 18px;
}
.stm-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 8px 18px;
  font-size: 13px;
  font-weight: 600;
  font-family: inherit;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.15s ease;
  border: none;
}
.stm-btn-cancel {
  background: #fff;
  color: #555;
  border: 1px solid #d0d0d0;
}
.stm-btn-cancel:hover { background: #f5f5f5; }
.stm-btn-primary {
  background: #3b82f6;
  color: #fff;
}
.stm-btn-primary:hover { background: #2563eb; }
</style>
