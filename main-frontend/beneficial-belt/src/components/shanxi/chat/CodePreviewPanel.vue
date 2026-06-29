<template>
  <aside v-if="selectedFile" class="code-preview-panel">
    <div class="panel-header">
      <span class="file-name">
        <Icon icon="mdi:file-code-outline" width="16" />
        {{ selectedFile.name }}
      </span>
      <button class="close-btn" @click="$emit('close')">
        <Icon icon="mdi:close" width="16" />
      </button>
    </div>
    <div class="panel-body">
      <div v-if="loading" class="loading-state">加载中...</div>
      <pre v-else><code ref="codeEl" class="hljs"></code></pre>
    </div>
  </aside>
</template>

<script setup>
import { ref, watch, nextTick, defineProps, defineEmits } from 'vue'
import { Icon } from '@iconify/vue'
import hljs from 'highlight.js'

const props = defineProps({
  selectedFile: { type: Object, default: null },
  fileContent: { type: String, default: '' },
  loading: { type: Boolean, default: false }
})

defineEmits(['close'])

const codeEl = ref(null)

// 当文件内容变化时，高亮代码
watch([() => props.fileContent, () => props.loading], async ([content, isLoading]) => {
  if (!isLoading && content && codeEl.value) {
    await nextTick()
    // 简单根据文件名判断语言，你可以扩展
    const ext = props.selectedFile?.name?.split('.').pop() || ''
    const langMap = { 'js': 'javascript', 'ts': 'typescript', 'vue': 'html', 'go': 'go', 'py': 'python', 'css': 'css', 'html': 'html' }
    const lang = langMap[ext] || 'text'
    codeEl.value.className = `hljs language-${lang}`
    codeEl.value.textContent = content
    hljs.highlightElement(codeEl.value)
  }
})
</script>

<style scoped>
.code-preview-panel {
  width: 400px; /* 固定宽度，可调整 */
  height: 100%;
  border-left: 1px solid #e4dfd4;
  background: #faf9f6;
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
}
.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  font-size: 12px;
  font-weight: 600;
  color: #696259;
  border-bottom: 1px solid #e4dfd4;
}
.file-name {
  display: flex;
  align-items: center;
  gap: 6px;
}
.close-btn {
  background: none;
  border: none;
  cursor: pointer;
  color: #888;
  padding: 2px;
}
.panel-body {
  flex: 1;
  overflow-y: auto;
  padding: 12px;
}
pre {
  margin: 0;
  white-space: pre-wrap;
  font-family: 'Fira Code', monospace;
  font-size: 12px;
  line-height: 1.5;
}
.loading-state {
  color: #888;
  font-size: 12px;
  text-align: center;
  padding: 20px;
}
</style>