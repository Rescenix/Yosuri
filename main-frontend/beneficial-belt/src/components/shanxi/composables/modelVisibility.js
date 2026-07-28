// 模型可见性 store：用户可在「编辑模型」弹窗里隐藏不想在下拉出现的模型。
// 默认空集合 = 后端 free_models 里「免 key 或已配 Key」的模型全部显示（无手动门控）。
// 这里只存被用户显式隐藏的 id；与「填了 Key 才出现」的判定（isModelVisible）正交。
//
// 注意：hiddenModelIds 是 ref(new Set())，直接 mutate Set 不会触发 Vue 响应式，
// 每次变更后必须重新赋值一个全新 Set（见 persistAndNotify）。
import { ref } from 'vue'

const HIDDEN_KEY = 'hiddenModelIds'

function load() {
  try { return new Set(JSON.parse(localStorage.getItem(HIDDEN_KEY) || '[]')) } catch (e) { return new Set() }
}

export const hiddenModelIds = ref(load())

function persistAndNotify() {
  localStorage.setItem(HIDDEN_KEY, JSON.stringify(Array.from(hiddenModelIds.value)))
  // 强制触发响应式：替换为一个新 Set 引用
  hiddenModelIds.value = new Set(hiddenModelIds.value)
}

export function isHidden(id) {
  return hiddenModelIds.value.has(id)
}

export function setHidden(id, hidden) {
  if (hidden) hiddenModelIds.value.add(id)
  else hiddenModelIds.value.delete(id)
  persistAndNotify()
}

export function toggleHidden(id) {
  if (hiddenModelIds.value.has(id)) hiddenModelIds.value.delete(id)
  else hiddenModelIds.value.add(id)
  persistAndNotify()
}

// 批量：整组提供方开/关（on=true 全部可见）
export function batchSet(ids, visible) {
  for (const id of ids) {
    if (visible) hiddenModelIds.value.delete(id)
    else hiddenModelIds.value.add(id)
  }
  persistAndNotify()
}

export function syncHidden() {
  hiddenModelIds.value = load()
}
