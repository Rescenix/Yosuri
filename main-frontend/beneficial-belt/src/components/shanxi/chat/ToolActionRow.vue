<template>
  <div class="bgstep-action">
    <div class="bgstep-action-row" @click="expanded = !expanded">
      <span class="bgstep-badge" :style="{ background: actionBadge(tc).color }">{{ actionBadge(tc).letter }}</span>
      <span class="bgstep-action-label">{{ actionLabel(tc) }}</span>
      <Icon icon="mdi:chevron-right" width="13" class="bgstep-chev" :class="{ open: expanded }" />
    </div>
    <!-- 第三层：真实审计数据，紧邻动作行下方展开，顺着文档流推开后续动作 -->
    <div v-if="expanded" class="bgstep-action-detail">
      <!-- write_file：全新写入，没有 before 快照，DiffViewer 里 oldContent 传空
           字符串，jsdiff 会正确地把所有内容展示成新增行 -->
      <template v-if="tc.name === 'write_file' && readArgs(tc).path">
        <div class="bgdiff-card">
          <div class="bgdiff-head">
            <Icon icon="mdi:file-outline" width="13" color="#a3a3a3" />
            <span class="bgdiff-path">{{ readArgs(tc).path }}</span>
            <span class="bgdiff-add-count">+{{ diffStats(tc).added }}</span>
          </div>
          <DiffViewer :old-content="''" :new-content="readArgs(tc).content || ''" :path="readArgs(tc).path" />
        </div>
      </template>

      <!-- edit_file：old_string → new_string，是真正意义上的前后对比 -->
      <template v-else-if="tc.name === 'edit_file' && readArgs(tc).path">
        <div class="bgdiff-card">
          <div class="bgdiff-head">
            <Icon icon="mdi:file-outline" width="13" color="#a3a3a3" />
            <span class="bgdiff-path">{{ readArgs(tc).path }}</span>
            <span class="bgdiff-add-count">+{{ diffStats(tc).added }}</span>
            <span class="bgdiff-del-count">−{{ diffStats(tc).removed }}</span>
          </div>
          <DiffViewer
            :old-content="readArgs(tc).old_string || ''"
            :new-content="readArgs(tc).new_string || ''"
            :path="readArgs(tc).path"
            :start-line="editStartLine(tc)"
          />
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
import { diffLines } from 'diff'
import { parseToolArgs, fileBaseName } from './toolArgs.js'
import DiffViewer from './DiffViewer.vue'

defineProps({
  tc: { type: Object, required: true }
})

const expanded = ref(false)

function readArgs(tc) {
  return parseToolArgs(tc.args)
}

function truncateText(text, limit) {
  if (!text) return ''
  return text.length > limit ? text.slice(0, limit) + '\n⋯（已截断）' : text
}

// old_string/new_string 只是文件片段，行号天然从 1 开始；后端在 result 文本里
// 附带了这段片段在真实文件中的起始行号（"第 N 行"），这里解析出来传给 DiffViewer
// 做偏移，解析不到就退回 1（老会话记录、非 edit_file 场景等兜底）
function editStartLine(tc) {
  const m = /第\s*(\d+)\s*行/.exec(tc.result || '')
  return m ? parseInt(m[1], 10) : 1
}

// 轻量统计增删行数，只给折叠摘要/标题栏用——真正的逐行渲染在 DiffViewer 里
function diffStats(tc) {
  const args = readArgs(tc)
  const oldStr = tc.name === 'edit_file' ? (args.old_string || '') : ''
  const newStr = tc.name === 'edit_file' ? (args.new_string || '') : (args.content || '')
  const parts = diffLines(oldStr, newStr)
  let added = 0, removed = 0
  for (const p of parts) {
    const lines = p.value.split('\n')
    if (lines.length > 0 && lines[lines.length - 1] === '') lines.pop()
    if (p.added) added += lines.length
    else if (p.removed) removed += lines.length
  }
  return { added, removed }
}

function actionLabel(tc) {
  if (tc.name === 'write_file') {
    const args = readArgs(tc)
    return `编辑了 ${fileBaseName(args.path)} +${diffStats(tc).added}`
  }
  if (tc.name === 'edit_file') {
    const args = readArgs(tc)
    const { added, removed } = diffStats(tc)
    return `编辑了 ${fileBaseName(args.path)} +${added} −${removed}`
  }
  if (tc.name === 'execute_command') {
    const args = readArgs(tc)
    const cmd = args.command || tc.args || ''
    return `运行了 ${cmd.length > 42 ? cmd.slice(0, 42) + '…' : cmd}`
  }
  if (tc.name === 'read_file') {
    const args = readArgs(tc)
    return `读取了 ${fileBaseName(args.path)}`
  }
  return tc.name
}
// 设计稿要求的 16x16 圆角色块字母徽章：R 读取(蓝) / W 编辑(强调色，write_file 和
// edit_file 共用，都是"改文件"这个语义) / > 执行命令(灰) / · 说明性文字(弱色)
function actionBadge(tc) {
  if (tc.name === 'read_file') return { letter: 'R', color: '#5b8def' }
  if (tc.name === 'write_file' || tc.name === 'edit_file') return { letter: 'W', color: '#c96442' }
  if (tc.name === 'execute_command') return { letter: '>', color: '#94a3b8' }
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
.bgdiff-head { display: flex; align-items: baseline; gap: 6px; padding: 6px 10px; background: #f5f5f5; border-bottom: 1px solid #e5e5e5; }
.bgdiff-head :deep(svg) { align-self: center; }
.bgdiff-path { flex: 1; min-width: 0; font-family: "JetBrains Mono", ui-monospace, Menlo, monospace; font-size: 11.5px; font-weight: 600; color: #262626; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.bgdiff-add-count { font-family: "JetBrains Mono", ui-monospace, Menlo, monospace; font-size: 11.5px; font-weight: 700; color: #12b76a; flex-shrink: 0; }
.bgdiff-del-count { font-family: "JetBrains Mono", ui-monospace, Menlo, monospace; font-size: 11.5px; font-weight: 700; color: #d94834; flex-shrink: 0; }
</style>
