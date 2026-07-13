<template>
  <div v-if="attachments.length" class="attach-chip-row">
    <div v-for="att in attachments" :key="att.id" class="attach-chip" :class="[att.kind, att.status]">
      <img v-if="att.kind === 'image'" :src="att.previewUrl" class="attach-chip-thumb" />
      <div v-else class="attach-chip-icon">
        <Icon v-if="att.kind === 'folder'" icon="mdi:folder-outline" width="20" color="#8a8378" />
        <span v-else>{{ att.ext }}</span>
      </div>
      <div class="attach-chip-meta">
        <span class="attach-chip-name" :title="att.name">{{ att.name }}</span>
        <span v-if="att.status === 'analyzing'" class="attach-chip-status">分析中…</span>
        <span v-else-if="att.status === 'error'" class="attach-chip-status error">{{ att.errorMsg }}</span>
        <span v-else-if="att.kind === 'folder'" class="attach-chip-status">{{ att.fileCount }} 个文件</span>
      </div>
      <button v-if="removable" class="attach-chip-remove" type="button" @click="$emit('remove', att.id)" title="移除">
        <Icon icon="mdi:close" width="11" />
      </button>
    </div>
  </div>
</template>

<script setup>
// 附件预览 chip 行——输入框上方（可移除）和用户消息气泡里（只读回放）共用同一份，
// 之前工作流气泡是把附件内容整段拼进纯文本显示，太丑；现在气泡也用这份 chip
// 展示"贴过什么"，正文只留用户自己敲的话
import { Icon } from '@iconify/vue'

defineProps({
  attachments: { type: Array, default: () => [] },
  removable: { type: Boolean, default: false }
})
defineEmits(['remove'])
</script>

<style scoped>
.attach-chip-row {
  display: flex;
  gap: 8px;
  overflow-x: auto;
}
.attach-chip {
  position: relative;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  width: 168px;
  padding: 6px 22px 6px 6px;
  border: 1px solid #e5e5e5;
  border-radius: 10px;
  background: #fafafa;
  box-sizing: border-box;
}
.attach-chip.error { border-color: #f3c9c2; background: #fff5f3; }
.attach-chip.analyzing { opacity: 0.7; }
.attach-chip-thumb { width: 36px; height: 36px; border-radius: 7px; object-fit: cover; flex-shrink: 0; }
.attach-chip-icon {
  width: 36px; height: 36px; border-radius: 7px; flex-shrink: 0;
  display: flex; align-items: center; justify-content: center;
  background: #ececec; color: #6b6b6b; font-size: 10px; font-weight: 700;
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
}
.attach-chip-meta { min-width: 0; display: flex; flex-direction: column; gap: 1px; }
.attach-chip-name { font-size: 12px; color: #262626; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.attach-chip-status { font-size: 10.5px; color: #a3a3a3; }
.attach-chip-status.error { color: #d94834; }
.attach-chip-remove {
  position: absolute; top: -5px; right: -5px;
  width: 16px; height: 16px; border-radius: 50%;
  display: flex; align-items: center; justify-content: center;
  background: #404040; color: #fff; border: 1.5px solid #fff;
  cursor: pointer; padding: 0;
}
.attach-chip-remove:hover { background: #1a1a1a; }
</style>
