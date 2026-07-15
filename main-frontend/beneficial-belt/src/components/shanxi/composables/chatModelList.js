// 聊天下拉与设置面板共享的「用户在设置里选为可用的模型列表」。
// 设置面板 toggleVendorModels 写这里，ChatWidget 下拉直接读这里——
// 保证「设置里没选 → 下拉空」「设置里选了谁 → 下拉才有谁」，同源响应式同步。
import { ref } from 'vue'

const CHAT_LIST_KEY = 'chatModelList'

function load() {
  try { return JSON.parse(localStorage.getItem(CHAT_LIST_KEY) || '[]') } catch (e) { return [] }
}

// 模块级单例 ref，整个 app 共享同一份状态（聊天窗 + 设置面板都 import 它）
export const chatModelList = ref(load())

export function syncChatModelList() {
  chatModelList.value = load()
}

export function setChatModelList(list) {
  localStorage.setItem(CHAT_LIST_KEY, JSON.stringify(list))
  chatModelList.value = list
}
