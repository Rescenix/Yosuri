import { ref } from 'vue'

// 代码编辑器（Monaco）懒加载偏好：跨组件共享的模块级状态。
// 默认开启：打开项目时不加载 Monaco（5MB+ 整包 + 4 个 worker），
// 只在真正打开文件面板时才拉，兼容低配机器启动卡顿。
// localStorage 持久化，key 与 FileToolPanel / SettingsModal 共用。
const EDITOR_LAZY_KEY = 'rescene-editor-lazy'

const editorLazy = ref(localStorage.getItem(EDITOR_LAZY_KEY) !== '0') // 默认开启

function setEditorLazy(v) {
  editorLazy.value = !!v
  try {
    localStorage.setItem(EDITOR_LAZY_KEY, v ? '1' : '0')
  } catch { /* localStorage 不可用时忽略 */ }
}

export function useEditorPrefs() {
  return { editorLazy, setEditorLazy }
}
