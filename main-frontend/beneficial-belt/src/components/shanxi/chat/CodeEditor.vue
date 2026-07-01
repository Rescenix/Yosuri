<template>
  <aside class="code-editor-panel">
    <div class="editor-tabs">
      <div
        v-for="tab in tabs"
        :key="tab.path"
        class="editor-tab"
        :class="{ active: tab.path === activeFilePath }"
        @click="$emit('switch-file', tab.path)"
        @contextmenu.prevent="onTabRightClick($event, tab)"
      >
        <Icon v-if="isPinned(tab)" icon="mdi:pin" width="11" class="tab-pin-icon" />
        <span class="tab-name">{{ tab.name }}</span>
        <button
          v-if="tabs.length > 1"
          class="tab-close"
          @click.stop="$emit('close-file', tab.path)"
        >
          <Icon icon="mdi:close" width="12" />
        </button>
      </div>
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
  theme="vs"
  @mount="onEditorMount"
/>
      <div v-else class="editor-placeholder">
        选择文件开始编辑
      </div>
    </div>
  </aside>
</template>

<script setup>
/* eslint-disable vue/no-v-model-argument */
import { ref, watch } from 'vue'
import VueMonacoEditor from '@guolao/vue-monaco-editor'

const props = defineProps({
  tabs: { type: Array, default: () => [] },
  activeFilePath: { type: String, default: '' },
  fileContent: { type: String, default: '' },
  language: { type: String, default: 'text' },
  pinnedPaths: { type: Array, default: () => [] }
})

const emit = defineEmits(['update:content', 'switch-file', 'close-file', 'editor-mounted', 'pin-file', 'unpin-file'])

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

const editorOptions = {
  minimap: { enabled: false },
  lineNumbers: 'on',
  scrollBeyondLastLine: false,
  automaticLayout: true,
  fontSize: 13,
  tabSize: 2,
  wordWrap: 'on'
}

function onEditorMount(editor) {
  emit('editor-mounted', editor)
}
</script>
<style scoped>
.code-editor-panel {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  background: #ffffff;
  border-left: 1px solid #e4dfd4;
}

.editor-tabs {
  display: flex;
  background: #f3f3f3;
  border-bottom: 1px solid #e4dfd4;
  overflow-x: auto;
  flex-shrink: 0;
}

.editor-tab {
  display: flex;
  align-items: center;
  padding: 6px 12px;
  font-size: 12px;
  color: #333;
  cursor: pointer;
  border-right: 1px solid #e4dfd4;
  gap: 8px;
}

.editor-tab.active {
  background: #ffffff;
  color: #000;
}

.tab-close {
  background: none;
  border: none;
  color: #666;
  cursor: pointer;
  padding: 0;
}

.tab-pin-icon {
  color: #c96442;
  flex-shrink: 0;
}

.tab-context-menu {
  position: fixed;
  z-index: 9999;
  background: #fff;
  border: 1px solid #d4cfc4;
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
  color: #4a4540;
}
.tab-context-menu button:hover {
  background: #f0ede3;
}

.editor-container {
  flex: 1;
  overflow: hidden;
}

.editor-placeholder {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #888;
  font-size: 14px;
  background: #ffffff;
}
</style>