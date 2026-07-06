<template>
  <div class="msg-group">
    <!-- 第一部分：头部自然语言（开始等描述），排除最后一段总结 -->
    <div v-for="(para, idx) in startNarratives" :key="'s' + idx" class="group-narrative">{{ para }}</div>
    
    <!-- 加载中占位符（只在没有任何文字且没有工具时显示） -->
    <div v-if="showLoadingPlaceholder && startNarratives.length === 0" class="group-narrative group-narrative-loading">正在处理...</div>

    <!-- 第二部分：唯一的聚合折叠摘要与白卡片 -->
    <template v-if="allToolCalls.length">
      <div class="agent-group-summary" @click="expanded = !expanded">
        <Icon :icon="statusIcon" :color="statusColor" :spin="group.status === 'running'" width="13" />
        <span>{{ summaryText }}</span>
        <span class="agent-group-chev" :class="{ open: expanded }">›</span>
      </div>
      <div v-if="expanded" class="agent-group-card">
        <ToolActionRow v-for="(tc, i) in allToolCalls" :key="i" :tc="tc" />
      </div>
    </template>

    <!-- 第三部分：尾部自然语言总结（如“已经成功将...”）放在白卡片下方 -->
    <div v-if="endNarrative" class="group-narrative">{{ endNarrative }}</div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { Icon } from '@iconify/vue'
import ToolActionRow from './ToolActionRow.vue'

const props = defineProps({
  group: { type: Object, required: true }
})

const expanded = ref(false)
defineExpose({
  expand: () => { expanded.value = true }
})

// 清洗可能的残留 JSON 标记
function stripStrayToolJSON(text) {
  return (text || '').replace(/\{[\s\S]*"tool"[\s\S]*\}\s*$/, '').trim()
}

// 提取所有自然语言段落的原始数组
const narrativeParts = computed(() => {
  const parts = []
  for (const step of props.group.steps) {
    const cleaned = stripStrayToolJSON(step.content)
    if (cleaned) parts.push(cleaned)
  }
  return parts
})

// 头部自然语言：除最后一段外的所有段落
const startNarratives = computed(() => {
  if (narrativeParts.value.length <= 1) return narrativeParts.value
  return narrativeParts.value.slice(0, -1)
})

// 尾部自然语言：取出最后一段作为总结
const endNarrative = computed(() => {
  if (narrativeParts.value.length <= 1) return ''
  return narrativeParts.value[narrativeParts.value.length - 1]
})

// 加载占位符状态
const showLoadingPlaceholder = computed(() => {
  return props.group.status === 'running' && narrativeParts.value.length === 0 && allToolCalls.value.length === 0
})

// 跨步骤聚合所有工具调用
const allToolCalls = computed(() => {
  const calls = []
  for (const step of props.group.steps) {
    if (step.toolCalls) calls.push(...step.toolCalls)
  }
  return calls
})

// 折叠摘要文本统计
const summaryText = computed(() => {
  let reads = 0, edits = 0, commands = 0, others = 0
  for (const tc of allToolCalls.value) {
    if (tc.name === 'read_file') reads++
    else if (tc.name === 'write_file') edits++
    else if (tc.name === 'execute_command') commands++
    else others++
  }
  const parts = []
  if (reads > 0) parts.push(`读取了 ${reads} 个文件`)
  if (edits > 0) parts.push(`编辑了 ${edits} 个文件`)
  if (commands > 0) parts.push(`运行了 ${commands} 条命令`)
  if (others > 0) parts.push(`调用了 ${others} 次工具`)
  return parts.join('，')
})

// 状态图标与颜色
const statusIcon = computed(() => {
  if (props.group.status === 'running') return 'mdi:loading'
  if (props.group.status === 'failed') return 'mdi:alert-circle'
  if (props.group.status === 'stopped') return 'mdi:stop-circle'
  return 'mdi:check-circle'
})
const statusColor = computed(() => {
  if (props.group.status === 'running') return '#c96442'
  if (props.group.status === 'failed') return '#d94834'
  if (props.group.status === 'stopped') return '#a3a3a3'
  return '#12b76a'
})
</script>

<style scoped>
.msg-group { margin: 4px 0; width: 100%; }

.group-narrative {
  font-weight: 500;
  font-size: 14px;
  color: #1a1a1a;
  margin: 2px 0 6px 0;
  white-space: pre-wrap;
  word-break: break-word;
}
.group-narrative-loading { color: #8a8a8a; font-weight: 400; font-size: 13px; }

.agent-group-summary {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  cursor: pointer;
  font-size: 13px;
  color: #6b6b6b;
  padding: 2px 0;
  transition: color 0.15s ease;
  user-select: none;
}
.agent-group-summary:hover { color: #1a1a1a; }
.agent-group-chev {
  display: inline-block;
  font-size: 12px;
  color: #a3a3a3;
  transition: transform 0.15s ease;
}
.agent-group-chev.open { transform: rotate(90deg); }

/* 外层白卡片——唯一一张 */
.agent-group-card {
  margin-top: 8px;
  border: 1px solid #e4dfd4;
  border-radius: 12px;
  background: #ffffff;
  padding: 10px 12px;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.03);
  overflow: hidden;
}
</style>