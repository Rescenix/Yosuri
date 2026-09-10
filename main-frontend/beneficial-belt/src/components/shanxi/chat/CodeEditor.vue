<template>
  <aside class="code-editor-panel">
    <div class="editor-tabs">
      <div
        v-for="tab in tabs"
        :key="tab.path"
        class="editor-tab"
        :class="{ active: tab.path === activeFilePath, conflict: externalChanges.includes(tab.path) }"
        @click="$emit('switch-file', tab.path)"
        @contextmenu.prevent="onTabRightClick($event, tab)"
      >
        <Icon v-if="isPinned(tab)" icon="mdi:pin" width="11" class="tab-pin-icon" />
                <Icon v-if="externalChanges.includes(tab.path)" icon="mdi:alert-circle-outline" width="12" class="tab-conflict-icon" title="磁盘上的文件已被外部修改；请先保存或重新打开后再处理" />
                <span v-if="tab.content !== tab.savedContent" class="tab-dirty-dot" title="有未保存的修改"></span>
                <span class="tab-name">{{ tab.name }}</span>
        <span
          class="tab-close"
          @click.stop="$emit('close-file', tab.path)"
        >&times;</span>
      </div>
      <div class="editor-tab-spacer"></div>
      <slot name="tab-actions"></slot>
    </div>

    <Teleport to="body">
      <div
        v-if="menu.show"
        class="tab-context-menu"
        :style="{ top: menu.y + 'px', left: menu.x + 'px' }"
        @click.stop
      >
        <button v-if="!isPinned(menu.tab)" @click="handlePin">固定到侧边栏</button>
        <button v-else @click="handleUnpin">取消固定</button>
        <button @click="handleClose">关闭标签页</button>
      </div>
    </Teleport>

    <div class="editor-container">
 <VueMonacoEditor
  v-if="activeFilePath"
  v-model:value="code"
  :language="language"
  :options="editorOptions"
  :theme="monacoTheme"
  @mount="onEditorMount"
/>
      <div v-else class="editor-placeholder">
        <div class="editor-empty-mark" aria-hidden="true">
          <Icon icon="mdi:code-braces" width="30" />
          <span class="editor-empty-spark">✦</span>
        </div>
        <strong>打开文件开始编辑</strong>
                <p>从文件树选择代码，或右键文件夹新建示例</p>
                <div class="editor-empty-languages" aria-label="支持的语言">
          <span>TS</span><span>GO</span><span>RS</span><span>PY</span>
        </div>
        <div class="editor-empty-shortcut"><kbd>Ctrl</kbd><b>+</b><kbd>G</kbd><span>打开文件工具</span></div>
      </div>
    </div>
  </aside>
</template>

<script setup>
/* eslint-disable vue/no-v-model-argument */
import { ref, computed, watch } from 'vue'
import { Icon } from '@iconify/vue'
import VueMonacoEditor from '@guolao/vue-monaco-editor'
import * as monaco from 'monaco-editor'
import { resolvedTheme } from '../composables/useTheme.js'

const props = defineProps({
  tabs: { type: Array, default: () => [] },
  activeFilePath: { type: String, default: '' },
  fileContent: { type: String, default: '' },
  language: { type: String, default: 'text' },
  pinnedPaths: { type: Array, default: () => [] },
  externalChanges: { type: Array, default: () => [] }
})

const emit = defineEmits(['update:content', 'switch-file', 'close-file', 'editor-mounted', 'pin-file', 'unpin-file'])

const monacoTheme = computed(() => resolvedTheme() === 'dark' ? 'vs-dark' : 'vs')

function isPinned(tab) {
  return props.pinnedPaths.includes(tab.path)
}

const menu = ref({ show: false, x: 0, y: 0, tab: null })

function onTabRightClick(event, tab) {
  menu.value = { show: true, x: event.clientX, y: event.clientY, tab }
  setTimeout(() => document.addEventListener('click', closeMenu, { once: true }), 0)
}
function closeMenu(event) {
  if (event.target.closest('.tab-context-menu')) return
  menu.value.show = false
}
function handlePin() {
  emit('pin-file', menu.value.tab)
  menu.value.show = false
}
function handleUnpin() {
  emit('unpin-file', menu.value.tab)
  menu.value.show = false
}
function handleClose() {
  emit('close-file', menu.value.tab.path)
  menu.value.show = false
}

const code = ref('')

watch(
  () => props.fileContent,
  (val) => {
    code.value = val || ''
  },
  { immediate: true }
)

watch(code, (val) => {
  emit('update:content', val)
})

const editorOptions = computed(() => ({
  minimap: { enabled: false },
  lineNumbers: 'on',
  scrollBeyondLastLine: false,
  automaticLayout: true,
  fontSize: 13,
    lineHeight: 20,
  fontFamily: "'Cascadia Code', 'JetBrains Mono', Consolas, monospace",
  fontLigatures: true,
  tabSize: 2,
  wordWrap: 'on',
  cursorStyle: 'line',
  cursorWidth: 2,
  cursorBlinking: 'blink',
  padding: { top: 10, bottom: 12 },
    renderLineHighlight: 'line',
  matchBrackets: 'always',
  occurrencesHighlight: 'singleFile',
  selectionHighlight: true,
  smoothScrolling: true,
  cursorSmoothCaretAnimation: 'on'
}))

function onEditorMount(editor) {
  // VS Code 的 Format Document 快捷键。没有注册相应格式化器的语言会保持 Monaco
  // 默认行为（不改写内容），已注册的 HTML/CSS/JSON/JS 等直接格式化当前文档。
  editor.addAction({
    id: 'file-tool.format-document',
    label: '格式化文档',
    keybindings: [monaco.KeyMod.Shift | monaco.KeyMod.Alt | monaco.KeyCode.KeyF],
    run: (instance) => instance.getAction('editor.action.formatDocument')?.run()
  })
  emit('editor-mounted', editor)
}
</script>
<style scoped>
.code-editor-panel {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  background: var(--app-surface);
  border-left: 1px solid var(--app-border);
}

.editor-tabs {
  display: flex;
  align-items: center;
  background: var(--app-surface);
  border-bottom: 1px solid var(--app-border);
  overflow-x: auto;
  flex-shrink: 0;
}

.editor-tab {
  position: relative;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  width: fit-content;
  max-width: 160px;
  height: 34px;
  padding: 0 10px;
  font-size: 12px;
  color: var(--app-text-faint);
  cursor: pointer;
  border: none;
  border-right: 1px solid var(--app-border);
  background: transparent;
  transition: color 0.15s ease;
}

.editor-tab:hover {
  color: var(--app-text-soft);
}

.editor-tab.active {
  color: var(--app-text);
  background: var(--app-surface);
}

.tab-close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  border-radius: 4px;
  flex-shrink: 0;
  font-size: 14px;
  line-height: 1;
  color: var(--app-text-faint);
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
}
.editor-tab:hover .tab-close {
  color: var(--app-text-soft);
}
.tab-close:hover {
  background: var(--app-surface-3);
  color: var(--app-text);
}

.editor-tab-spacer {
  flex: 1;
}

.tab-pin-icon {
  color: #c96442;
  flex-shrink: 0;
}
.tab-conflict-icon {
  color: #d58a2d;
  flex-shrink: 0;
}
.tab-dirty-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--app-accent);
  flex-shrink: 0;
}

.tab-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}

.tab-context-menu {
  position: fixed;
  z-index: 9999;
  background: var(--app-surface);
  border: 1px solid var(--app-border);
  border-radius: 6px;
  box-shadow: 0 2px 8px rgba(0,0,0,0.15);
  display: flex;
  flex-direction: column;
  padding: 4px 0;
  min-width: 140px;
}
.tab-context-menu button {
  padding: 6px 16px;
  text-align: left;
  border: none;
  background: none;
  font-size: 12px;
  cursor: pointer;
  color: var(--app-text);
}
.tab-context-menu button:hover {
  background: var(--app-surface-3);
}

/* 鼠标进编辑区消失的问题：之前只覆盖了 3 个选择器，漏了 lines-content/
   monaco-editor-background/inputarea 这几层——鼠标移到这些层上时用的是 Monaco
   自己内部动态切的 mouse-xxx 类，没盖到就掉回它自己的（有时是空/none）默认值。
   这里只扩到"实际渲染文字内容"的几层，不用通配符打全部子元素——滚动条、行号
   沟槽这些非文本区域的光标形状（default/pointer）是对的，不该被一起强改成 text。 */
.editor-container {
  flex: 1;
  overflow: hidden;
  /* 亮色模式：深色 I-beam；暗色模式：浅色 I-beam */
  cursor: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='8' height='16'%3E%3Cpath d='M3 0h2v16h-2zM1 0h6v1H1zM1 15h6v1H1z' fill='%23333'/%3E%3C/svg%3E") 3 8, text;
  caret-color: var(--app-text);
  }
  [data-theme="dark"] .editor-container {
  cursor: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='8' height='16'%3E%3Cpath d='M3 0h2v16h-2zM1 0h6v1H1zM1 15h6v1H1z' fill='%23ddd'/%3E%3C/svg%3E") 3 8, text;
}
.editor-container :deep(*) {
  cursor: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='8' height='16'%3E%3Cpath d='M3 0h2v16h-2zM1 0h6v1H1zM1 15h6v1H1z' fill='%23333'/%3E%3C/svg%3E") 3 8, text !important;
}
[data-theme="dark"] .editor-container :deep(*) {
  cursor: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='8' height='16'%3E%3Cpath d='M3 0h2v16h-2zM1 0h6v1H1zM1 15h6v1H1z' fill='%23ddd'/%3E%3C/svg%3E") 3 8, text !important;
}
.editor-container :deep(.monaco-editor .cursor) {
  background: var(--app-text) !important;
  border-left-color: var(--app-text) !important;
}
.editor-container :deep(.inputarea) {
  caret-color: var(--app-text) !important;
}

.editor-placeholder {
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: var(--app-text-soft);
  font-size: 13px;
  background:
    linear-gradient(color-mix(in srgb, var(--app-border), transparent 82%) 1px, transparent 1px),
    linear-gradient(90deg, color-mix(in srgb, var(--app-border), transparent 82%) 1px, transparent 1px),
    radial-gradient(circle at 50% 42%, color-mix(in srgb, var(--app-accent), transparent 90%), transparent 34%),
    var(--app-surface);
  background-size: 28px 28px, 28px 28px, auto, auto;
}
.editor-empty-mark {
  position: relative;
  width: 58px;
  height: 58px;
  display: grid;
  place-items: center;
  margin-bottom: 4px;
  border: 1px solid color-mix(in srgb, var(--app-accent), var(--app-border) 54%);
  border-radius: 4px;
  background: color-mix(in srgb, var(--app-accent), var(--app-surface) 92%);
  color: var(--app-accent);
  box-shadow: 8px 8px 0 color-mix(in srgb, #f3a9ce, transparent 84%);
}
.editor-empty-spark {
  position: absolute;
  top: -9px;
  right: -8px;
  color: #e78aba;
  font-size: 15px;
}
.editor-placeholder strong {
  color: var(--app-text);
  font-size: 15px;
  letter-spacing: .02em;
}
.editor-placeholder p {
  margin: 0;
  color: var(--app-text-faint);
  font-size: 12px;
}
.editor-empty-languages {
  display: flex;
  gap: 5px;
  margin-top: 5px;
}
.editor-empty-languages span {
  min-width: 30px;
  padding: 3px 6px;
  border: 1px solid var(--app-border);
  border-radius: 2px;
  background: var(--app-surface-2);
  color: var(--app-text-soft);
  font: 700 10px/1.2 "JetBrains Mono", ui-monospace, Consolas, monospace;
  text-align: center;
}
.editor-empty-shortcut {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 8px;
  color: var(--app-text-faint);
  font-size: 11px;
}
.editor-empty-shortcut kbd {
  min-width: 22px;
  padding: 2px 5px;
  border: 1px solid var(--app-border);
  border-bottom-width: 2px;
  border-radius: 2px;
  background: var(--app-surface);
  color: var(--app-text-soft);
  font: 600 10px/1.2 "JetBrains Mono", ui-monospace, Consolas, monospace;
  text-align: center;
}
.editor-empty-shortcut b {
  font-weight: 500;
  color: var(--app-text-faint);
}
.editor-empty-shortcut span {
  margin-left: 3px;
}
</style>
