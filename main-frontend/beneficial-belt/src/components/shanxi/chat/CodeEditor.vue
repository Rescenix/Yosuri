<template>
  <aside class="code-editor-panel">
    <div class="editor-tabs">
      <div
        v-for="tab in tabs"
        :key="tab.path"
        class="editor-tab"
        :class="{ active: tab.path === activeFilePath }"
        @click="$emit('switch-file', tab.path)"
      >
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
  language: { type: String, default: 'text' }
})

const emit = defineEmits(['update:content', 'switch-file', 'close-file', 'editor-mounted'])

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