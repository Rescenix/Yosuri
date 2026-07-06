<template>
  <div class="bgstep-action">
    <div class="bgstep-action-row" @click="expanded = !expanded">
      <span class="bgstep-badge" :style="{ background: actionBadge(tc).color }">{{ actionBadge(tc).letter }}</span>
      <span class="bgstep-action-label">{{ actionLabel(tc) }}</span>
      <Icon icon="mdi:chevron-right" width="13" class="bgstep-chev" :class="{ open: expanded }" />
    </div>
    <!-- 第三层：真实审计数据，紧邻动作行下方展开，顺着文档流推开后续动作 -->
    <div v-if="expanded" class="bgstep-action-detail">
      <!-- write_file：文件路径 + 行级 diff（新增行标绿） -->
      <template v-if="tc.name === 'write_file' && writeDiff(tc)">
        <div class="bgdiff-card">
          <div class="bgdiff-head">
            <Icon icon="mdi:file-outline" width="13" color="#a3a3a3" />
            <span class="bgdiff-path">{{ writeDiff(tc).path }}</span>
            <span class="bgdiff-add-count">+{{ writeDiff(tc).lineCount }}</span>
          </div>
          <div class="bgdiff-lines">
            <div v-for="(line, li) in writeDiff(tc).preview" :key="li" class="bgdiff-line add">
              <span class="bgdiff-gutter">+</span><span class="bgdiff-text">{{ line || ' ' }}</span>
            </div>
            <div v-if="writeDiff(tc).truncated" class="bgdiff-more">
              ⋯ 还有 {{ writeDiff(tc).lineCount - writeDiff(tc).preview.length }} 行 ⋯
            </div>
          </div>
        </div>
      </template>

      <!-- read_file：文件路径 + 读取内容片段 -->
      <template v-else-if="tc.name === 'read_file'">
        <div class="bgdiff-card">
          <div class="bgdiff-head">
            <Icon icon="mdi:file-outline" width="13" color="#a3a3a3" />
            <span class="bgdiff-path">{{ readArgs(tc).path || '(未知路径)' }}</span>
          </div>
          <div v-if="tc.result" class="bgstep-raw-block">{{ truncateText(tc.result, 600) }}</div>
        </div>
      </template>

      <!-- execute_command：命令原文 + 真实输出 -->
      <template v-else-if="tc.name === 'execute_command'">
        <code class="bgstep-code-line">$ {{ readArgs(tc).command || tc.args }}</code>
        <div v-if="tc.result" class="bgstep-raw-block">{{ truncateText(tc.result, 800) }}</div>
        <div v-if="tc.error" class="bgstep-raw-block error">{{ truncateText(tc.error, 800) }}</div>
      </template>

      <!-- 其它工具：原始参数兜底 -->
      <template v-else>
        <code class="bgstep-code-line">{{ tc.name }}<template v-if="tc.args"> {{ tc.args }}</template></code>
        <div v-if="tc.result" class="bgstep-raw-block">{{ truncateText(tc.result, 600) }}</div>
      </template>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { Icon } from '@iconify/vue'

defineProps({
  tc: { type: Object, required: true }
})

const expanded = ref(false)

function parseToolArgs(argsStr) {
  const args = {}
  const re = /(\w+)="([\s\S]*?)"/g
  let m
  while ((m = re.exec(argsStr || '')) !== null) args[m[1]] = m[2]
  return args
}
function readArgs(tc) {
  return parseToolArgs(tc.args)
}

const WRITE_PREVIEW_LIMIT = 8
function writeDiff(tc) {
  if (tc.name !== 'write_file' || !tc.args) return null
  const args = parseToolArgs(tc.args)
  if (!args.path || args.content === undefined) return null
  const lines = args.content.split('\n')
  return {
    path: args.path,
    lineCount: lines.length,
    preview: lines.slice(0, WRITE_PREVIEW_LIMIT),
    truncated: lines.length > WRITE_PREVIEW_LIMIT
  }
}

function fileBaseName(path) {
  if (!path) return '文件'
  return path.split(/[\\/]/).pop()
}

function truncateText(text, limit) {
  if (!text) return ''
  return text.length > limit ? text.slice(0, limit) + '\n⋯（已截断）' : text
}

function actionLabel(tc) {
  if (tc.name === 'write_file') {
    const args = parseToolArgs(tc.args)
    const diff = writeDiff(tc)
    return `编辑了 ${fileBaseName(args.path)}${diff ? ' +' + diff.lineCount : ''}`
  }
  if (tc.name === 'execute_command') {
    const args = parseToolArgs(tc.args)
    const cmd = args.command || tc.args || ''
    return `运行了 ${cmd.length > 42 ? cmd.slice(0, 42) + '…' : cmd}`
  }
  if (tc.name === 'read_file') {
    const args = parseToolArgs(tc.args)
    return `读取了 ${fileBaseName(args.path)}`
  }
  return tc.name
}
// 设计稿要求的 16x16 圆角色块字母徽章：R 读取(蓝) / W 编辑(强调色) / > 执行命令(灰) / · 说明性文字(弱色)
function actionBadge(tc) {
  if (tc.name === 'read_file') return { letter: 'R', color: '#5b8def' }
  if (tc.name === 'write_file') return { letter: 'W', color: '#c96442' }
  if (tc.name === 'execute_command') return { letter: '>', color: '#8a8378' }
  return { letter: '·', color: '#a3a3a3' }
}
</script>

<style scoped>
.bgstep-action-row {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 7px 0;
  cursor: pointer;
  font-size: 12.5px;
  color: #262626;
  border-bottom: 1px solid #ececec;
}
.bgstep-action:last-child .bgstep-action-detail { border-bottom: none; }
.bgstep-action-row:hover { background: rgba(0, 0, 0, 0.03); }
.bgstep-action-label { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.bgstep-badge {
  flex-shrink: 0;
  width: 16px;
  height: 16px;
  border-radius: 5px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 10px;
  font-weight: 700;
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
  color: #fff;
  line-height: 1;
}

.bgstep-action-detail {
  padding: 8px 0 10px;
  background: transparent;
  border-bottom: 1px solid #ececec;
}
.bgstep-code-line {
  display: block;
  padding: 7px 10px;
  background: #f5f5f5;
  border: 1px solid #e5e5e5;
  border-radius: 8px;
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
  font-size: 11.5px;
  color: #404040;
  white-space: pre-wrap;
  word-break: break-all;
}
.bgstep-raw-block {
  margin-top: 6px;
  padding: 7px 10px;
  background: #fafafa;
  border: 1px solid #e5e5e5;
  border-radius: 8px;
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
  font-size: 11px;
  color: #6b6b6b;
  white-space: pre-wrap;
  word-break: break-word;
  max-height: 220px;
  overflow-y: auto;
}
.bgstep-raw-block.error { color: #d94834; border-color: #f3c9c2; background: #fff5f3; }

.bgdiff-card { border: 1px solid #e5e5e5; border-radius: 8px; overflow: hidden; background: #fff; }
.bgdiff-head { display: flex; align-items: center; gap: 6px; padding: 6px 10px; background: #f5f5f5; border-bottom: 1px solid #e5e5e5; }
.bgdiff-path { flex: 1; min-width: 0; font-family: "JetBrains Mono", ui-monospace, Menlo, monospace; font-size: 11.5px; font-weight: 600; color: #262626; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.bgdiff-add-count { font-family: "JetBrains Mono", ui-monospace, Menlo, monospace; font-size: 11px; font-weight: 700; color: #12b76a; flex-shrink: 0; }
.bgdiff-lines { padding: 2px 0; }
.bgdiff-line { display: flex; font-family: "JetBrains Mono", ui-monospace, Menlo, monospace; font-size: 11.5px; line-height: 1.6; padding: 0 10px; }
.bgdiff-line.add { background: rgba(18, 183, 106, 0.08); }
.bgdiff-gutter { width: 16px; flex-shrink: 0; color: #12b76a; font-weight: 700; user-select: none; }
.bgdiff-text { flex: 1; min-width: 0; color: #262626; white-space: pre; overflow-x: auto; }
.bgdiff-more { padding: 4px 10px; font-size: 10.5px; color: #a3a3a3; text-align: center; }
</style>
