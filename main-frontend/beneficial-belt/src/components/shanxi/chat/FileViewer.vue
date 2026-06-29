<template>
  <div v-if="file" class="file-viewer">
    <div class="file-viewer-header">
      <span>{{ file.name }}</span>
      <button @click="$emit('close')" class="close-btn">
        <Icon icon="mdi:close" width="16" />
      </button>
    </div>
    <div class="file-viewer-body">
      <pre><code ref="codeEl" class="hljs"></code></pre>
    </div>
  </div>
</template>

<script setup>
import { ref, watch, nextTick, defineProps, defineEmits } from 'vue'
import hljs from 'highlight.js'
import { Icon } from '@iconify/vue'

const props = defineProps({
  file: { type: Object, default: null },
  content: { type: String, default: '' }
})

defineEmits(['close'])

const codeEl = ref(null)

watch([() => props.content, () => props.file], async () => {
  if (props.file && props.content) {
    await nextTick()
    if (codeEl.value) {
      codeEl.value.textContent = props.content
      hljs.highlightElement(codeEl.value)
    }
  }
})
</script>

<style scoped>
.file-viewer {
  border: 1px solid #e4dfd4;
  background: #faf9f6;
  display: flex;
  flex-direction: column;
  max-height: 40%;
  overflow: hidden;
  flex-shrink: 0;
}
.file-viewer-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 4px 12px;
  font-size: 12px;
  background: #f0ede5;
  border-bottom: 1px solid #e4dfd4;
}
.file-viewer-body {
  overflow: auto;
  padding: 8px;
}
pre {
  margin: 0;
  white-space: pre-wrap;
  font-family: monospace;
  font-size: 12px;
}
</style>