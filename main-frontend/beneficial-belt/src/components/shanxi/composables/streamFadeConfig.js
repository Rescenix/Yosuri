// 流式瀑布渐变参数 store（与 chatModelList.js 等现有 composable 同一模式）
// ChatWidget.vue / SettingsModal.vue 均 import { streamFadeConfig }，直接 v-model
// 绑定各字段即可（改动即时生效、自动持久化到 localStorage['streamFadeConfig']）。
import { reactive, watch } from 'vue'

export const STREAM_FADE_DEFAULTS = {
  enabled: false,  // 总开关（默认关：09-02 watch 修复后渐变真正触发，每个 chunk 淡入=闪；以前流畅是因为它没跑）
  fadeMs: 100,     // 单字符淡入时长（ms）
  staggerMs: 8,    // 相邻字符的级联延迟（ms/字符）
  maxSweepMs: 250, // 单批 chunk 的最大扫过时长（ms）
  blurPx: 0,       // 淡入起始模糊强度（px）
}

// 渐变开关 09-05 才上线：默认关优先，历史持久化的 enabled 一律不采信（避免调试期坏值残留）
function loadPersisted() {
  try {
    const raw = JSON.parse(localStorage.getItem('streamFadeConfig') || '{}')
    delete raw.enabled
    delete raw.__v
    return raw
  } catch (e) { return {} }
}

export const streamFadeConfig = reactive({ ...STREAM_FADE_DEFAULTS, ...loadPersisted() })

watch(streamFadeConfig, () => {
  try { localStorage.setItem('streamFadeConfig', JSON.stringify(streamFadeConfig)) } catch (e) {}
}, { deep: true })

export function resetStreamFadeConfig() {
  Object.assign(streamFadeConfig, STREAM_FADE_DEFAULTS)
}
