// 流式瀑布渐变参数 store（与 chatModelList.js 等现有 composable 同一模式）
// ChatWidget.vue / SettingsModal.vue 均 import { streamFadeConfig }，直接 v-model
// 绑定各字段即可（改动即时生效、自动持久化到 localStorage['streamFadeConfig']）。
import { reactive, watch } from 'vue'

export const STREAM_FADE_DEFAULTS = {
  enabled: true,   // 总开关
  fadeMs: 500,     // 单字符淡入时长（ms）
  staggerMs: 14,   // 相邻字符的级联延迟（ms/字符）
  maxSweepMs: 350, // 单批 chunk 的最大扫过时长（ms）
  blurPx: 2,       // 淡入起始模糊强度（px）
}

function loadPersisted() {
  try { return JSON.parse(localStorage.getItem('streamFadeConfig') || '{}') } catch (e) { return {} }
}

export const streamFadeConfig = reactive({ ...STREAM_FADE_DEFAULTS, ...loadPersisted() })

watch(streamFadeConfig, () => {
  try { localStorage.setItem('streamFadeConfig', JSON.stringify(streamFadeConfig)) } catch (e) {}
}, { deep: true })

export function resetStreamFadeConfig() {
  Object.assign(streamFadeConfig, STREAM_FADE_DEFAULTS)
}
