<template>
  <Teleport to="body">
    <div class="stm-backdrop" @click.self="$emit('close')">
      <div class="stm-card">
        <!-- 标题栏 -->
        <div class="stm-header">
          <h2 class="stm-title">新建定时任务</h2>
          <button class="stm-close" @click="$emit('close')" title="关闭">
            <Icon icon="mdi:close" width="18" />
          </button>
        </div>

        <p class="stm-desc">
          排程一个提示词，到点就会在右下角弹出系统通知提醒你。
        </p>

        <div class="stm-body">
          <!-- 名称 -->
          <div class="stm-field">
            <label class="stm-label">名称 <span class="stm-optional">可选</span></label>
            <input
              v-model="form.name"
              class="stm-input"
              type="text"
              placeholder="晨间简报"
            />
          </div>

          <!-- 提示词 -->
          <div class="stm-field">
            <label class="stm-label">提示词</label>
            <textarea
              v-model="form.prompt"
              class="stm-textarea"
              placeholder="总结我未读的 Slack 话题，并把前 5 条邮件发给我..."
              rows="3"
            ></textarea>
          </div>

          <!-- 频率 + 投递至（并排） -->
          <div class="stm-row">
            <div class="stm-field stm-half">
              <label class="stm-label">频率</label>
              <select v-model="form.frequency" class="stm-select">
                <option value="every_1h">每小时</option>
                <option value="every_2h">每 2 小时</option>
                <option value="every_6h">每 6 小时</option>
                <option value="every_12h">每 12 小时</option>
                <option value="daily">每天</option>
                <option value="weekdays">工作日</option>
                <option value="weekly">每周</option>
                <option value="monthly">每月</option>
              </select>
            </div>
            <div class="stm-field stm-half">
              <label class="stm-label">投递至</label>
              <select v-model="form.deliverTo" class="stm-select">
                <option value="desktop">此桌面</option>
                <option value="chat">此会话</option>
                <option value="all">所有设备</option>
              </select>
            </div>
          </div>

          <!-- 时间：左小时 + 右分钟（仅每天/工作日/每周/每月需要，每小时类自动整点触发） -->
          <div v-if="needsTime" class="stm-row">
            <div class="stm-field stm-half">
              <label class="stm-label">小时</label>
              <select v-model="form.hour" class="stm-select">
                <option v-for="h in HOUR_OPTIONS" :key="h" :value="h">{{ h }}</option>
              </select>
            </div>
            <div class="stm-field stm-half">
              <label class="stm-label">分钟</label>
              <select v-model="form.minute" class="stm-select">
                <option v-for="m in MINUTE_OPTIONS" :key="m" :value="m">{{ m }}</option>
              </select>
            </div>
          </div>

          <!-- 模型 -->
          <div class="stm-field">
            <label class="stm-label">模型 <span class="stm-optional">可选</span></label>
            <select v-model="form.model" class="stm-select">
              <option value="">默认 (全局模型)</option>
              <option value="deepseek-v4-flash-free">DeepSeek V4 Flash</option>
              <option value="mimo-v2.5-free">Mimo 2.5</option>
              <option value="north-mini-code-free">North Mini Code</option>
            </select>
          </div>

          <!-- 时间反馈：大白话描述，如"每天 9:00" -->
          <div class="stm-schedule-preview">
            <span class="stm-schedule-text">🕐 {{ scheduleText }}</span>
          </div>
        </div>

        <div class="stm-footer">
          <button class="stm-btn stm-btn-cancel" @click="$emit('close')">取消</button>
          <button class="stm-btn stm-btn-primary" :disabled="!form.prompt.trim()" @click="onCreate">创建定时任务</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { ref, reactive, computed } from 'vue'
import { Icon } from '@iconify/vue'

const emit = defineEmits(['close', 'create'])

const form = reactive({
  name: '',
  prompt: '',
  frequency: 'daily',
  deliverTo: 'desktop',
  model: '',
  hour: '09',
  minute: '00'
})

// 小时/分钟下拉选项（每 15 分钟一档，小学生也容易选）
const HOUR_OPTIONS = []
for (let h = 0; h < 24; h++) HOUR_OPTIONS.push(String(h).padStart(2, '0'))
const MINUTE_OPTIONS = []
for (let m = 0; m < 60; m++) MINUTE_OPTIONS.push(String(m).padStart(2, '0'))

// 需要手动选时间的频率（每小时类自动整点触发，不显示时间下拉）
const needsTime = computed(() => ['daily', 'weekdays', 'weekly', 'monthly'].includes(form.frequency))

// 频率 → 时间占位格式（{t} 会被替换为 HH:MM）
const FREQ_MAP = {
  every_1h:  { text: '每小时', cron: () => '0 * * * *',       timeText: () => '每小时的整点' },
  every_2h:  { text: '每 2 小时', cron: () => '0 */2 * * *',   timeText: () => '每 2 小时的整点' },
  every_6h:  { text: '每 6 小时', cron: () => '0 */6 * * *',   timeText: () => '每 6 小时的整点' },
  every_12h: { text: '每 12 小时', cron: () => '0 */12 * * *', timeText: () => '每 12 小时的整点' },
  daily:     { text: '每天',   cron: t => `${t.m} ${t.h} * * *`,   timeText: t => `每天 ${t.t}` },
  weekdays:  { text: '工作日', cron: t => `${t.m} ${t.h} * * 1-5`, timeText: t => `工作日 ${t.t}` },
  weekly:    { text: '每周',   cron: t => `${t.m} ${t.h} * * 1`,   timeText: t => `每周一 ${t.t}` },
  monthly:   { text: '每月',   cron: t => `${t.m} ${t.h} 1 * *`,   timeText: t => `每月 1 日 ${t.t}` }
}

const freqMeta = computed(() => FREQ_MAP[form.frequency] || FREQ_MAP.daily)
// 把 form.hour / form.minute 合成对象，cron 用去前导零的小时/分钟，描述用完整 HH:MM
function timeParts() {
  const h = (form.hour || '09').replace(/^0/, '')
  const m = (form.minute || '00').replace(/^0/, '')
  const t = (form.hour || '00') + ':' + (form.minute || '00')
  return { h, m, t }
}
const cronExpr = computed(() => {
  const p = timeParts()
  const fn = freqMeta.value.cron
  return typeof fn === 'function' ? fn(p) : fn
})
const scheduleText = computed(() => {
  const p = timeParts()
  const fn = freqMeta.value.timeText
  return typeof fn === 'function' ? fn(p) : fn
})

function onCreate() {
  if (!form.prompt.trim()) return
  emit('create', {
    name: form.name.trim() || undefined,
    prompt: form.prompt.trim(),
    frequency: form.frequency,
    deliverTo: form.deliverTo,
    model: form.model || undefined,
    cron: cronExpr.value
  })
}
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
}

.stm-field { display: flex; flex-direction: column; gap: 5px; }
.stm-row { display: flex; flex-direction: row; gap: 12px; }
.stm-half { flex: 1; }

.stm-label {
  font-size: 12px;
  font-weight: 600;
  color: #333;
}
.stm-optional { font-weight: 400; color: #aaa; }

.stm-input,
.stm-select {
  width: 100%;
  padding: 8px 10px;
  font-size: 13px;
  font-family: inherit;
  color: #1a1a1a;
  border: 1px solid #d0d0d0;
  border-radius: 8px;
  background: #fff;
  outline: none;
  box-sizing: border-box;
  transition: border-color 0.15s ease;
}
.stm-input:focus,
.stm-select:focus {
  border-color: #3b82f6;
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.15);
}

.stm-textarea {
  width: 100%;
  padding: 8px 10px;
  font-size: 13px;
  font-family: inherit;
  color: #1a1a1a;
  border: 1px solid #d0d0d0;
  border-radius: 8px;
  background: #fff;
  outline: none;
  box-sizing: border-box;
  resize: vertical;
  min-height: 60px;
  line-height: 1.5;
  transition: border-color 0.15s ease;
}
.stm-textarea:focus {
  border-color: #3b82f6;
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.15);
}
.stm-textarea::placeholder,
.stm-input::placeholder { color: #bbb; }

.stm-schedule-preview {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  background: #f5f5f5;
  border-radius: 8px;
}
.stm-schedule-text {
  font-size: 13px;
  font-weight: 600;
  color: #1a1a1a;
}

.stm-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 12px 20px 18px;
}

.stm-btn {
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
.stm-btn-primary:disabled { background: #93c5fd; cursor: not-allowed; }
</style>
