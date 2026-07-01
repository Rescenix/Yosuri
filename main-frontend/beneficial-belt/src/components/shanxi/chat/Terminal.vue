<template>
  <div class="terminal-panel" :class="{ collapsed: !open }" :style="{ height: open ? height + 'px' : '28px' }">
    <div class="terminal-titlebar" @click="$emit('update:open', !open)">
      <span class="terminal-title">TERMINAL · node</span>
      <Icon icon="mdi:chevron-down" width="16" class="collapse-chevron" :class="{ rotated: !open }" />
    </div>
    <div class="terminal-body" v-show="open">
      <div class="term-line" v-for="(line, i) in lines" :key="i" v-html="line"></div>
      <div class="term-line term-cursor-line">
        <span class="term-prompt">PS C:\PrismD&gt;</span>
        <span class="term-cursor"></span>
      </div>
    </div>
  </div>
</template>

<script setup>
import { Icon } from '@iconify/vue'

defineProps({
  open: { type: Boolean, default: true },
  height: { type: Number, default: 180 }
})
defineEmits(['update:open'])

/* 目前为静态占位内容，尚无后端进程输出流（WS/SSE）接入，仅供视觉参考 */
const lines = [
  '<span class="term-prompt">PS C:\\PrismD&gt;</span> node server.js',
  '<span class="term-tag ok">[server]</span> listening on :3000',
  '<span class="term-tag warn">[ollama]</span> 正在加载本地模型 qwen2.5-coder:32b ...',
  '<span class="term-tag ok">[ollama]</span> 模型已就绪 · 显存占用 18.2GB',
  '<span class="term-tag info">[watch]</span> 监听文件变更中 ...'
]
</script>

<style scoped>
.terminal-panel {
  flex-shrink: 0;
  width: 100%;
  background: #1a1815;
  border-top: 1px solid #38332c;
  overflow: hidden;
  transition: height 180ms ease;
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
}

.terminal-titlebar {
  height: 28px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 12px;
  cursor: pointer;
  color: #9a958a;
  font-size: 11px;
  font-family: "JetBrains Mono", ui-monospace, monospace;
  letter-spacing: 0.3px;
  user-select: none;
}
.terminal-titlebar:hover { background: rgba(255,255,255,0.03); }

.collapse-chevron { transition: transform 180ms ease; }
.collapse-chevron.rotated { transform: rotate(180deg); }

.terminal-body {
  flex: 1;
  overflow-y: auto;
  padding: 8px 12px 10px;
  font-family: "JetBrains Mono", ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  line-height: 1.7;
  color: #d8d3c8;
}

.term-line { white-space: pre-wrap; word-break: break-all; }
.term-prompt { color: #6b8f71; margin-right: 6px; }
.term-tag { font-weight: 700; margin-right: 4px; }
.term-tag.ok { color: #6b9f7a; }
.term-tag.warn { color: #d0a45b; }
.term-tag.info { color: #6f9bc9; }

.term-cursor-line { display: flex; align-items: center; }
.term-cursor {
  display: inline-block;
  width: 7px;
  height: 14px;
  margin-left: 4px;
  background: #c96442;
  animation: term-blink 1s step-end infinite;
}
@keyframes term-blink {
  0%, 50% { opacity: 1; }
  50.01%, 100% { opacity: 0; }
}
</style>
