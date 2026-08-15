<template>
  <div class="agent-flow" :class="{ streaming: flow.status === 'running' }">
    <!-- 首条回复前的「正在思考」扫描线：不可折叠、无 chevron；首字一到（blockGroups 非空）自动消失 -->
    <div v-if="flow.status === 'running' && blockGroups.length === 0" class="flow-pending-scanline">
      <Icon icon="mdi:sparkles" class="flow-row-icon icon-think" width="13" />
      <span class="flow-pending-label">正在思考</span>
    </div>
    <!--
      ★ 按顺序渲染，但「回复(intent)」始终平铺可见；
      连续出现的「思考 + 工具调用」才收纳进同一个概要栏 + 可折叠时间线。
    -->
    <template v-for="(group, gIdx) in blockGroups" :key="gIdx">
      <!-- 可直接见的回复文本 -->
      <div
        v-if="group.type === 'visible'"
        class="flow-intent markdown-body"
        v-html="renderMarkdown(group.text, true)"
      ></div>

      <!-- 单步思考：不收束，直接平铺 -->
      <div v-else-if="group.type === 'single-thinking'" class="flow-thinking flow-thinking-single">
        <div class="flow-row-head" @click="toggleThink(`single-${gIdx}`)">
          <Icon icon="mdi:sparkles" class="flow-row-icon icon-think" width="13" />
          <span class="flow-thinking-text-label">{{ flow.status === 'running' ? '正在思考' : (flow.status === 'waiting' ? '等待后台任务' : '思考') }}</span>
          <span v-if="!(thinkOpen[`single-${gIdx}`] ?? true) && group.block.text" class="flow-row-preview">{{ onelinePreview(group.block.text) }}</span>
          <span v-else class="flow-spacer"></span>
          <span class="flow-chevron" :class="{ open: thinkOpen[`single-${gIdx}`] ?? true }">›</span>
        </div>
        <div v-if="thinkOpen[`single-${gIdx}`] ?? true" class="flow-detail">
          <div class="flow-thinking-text">{{ group.block.text }}</div>
        </div>
      </div>

      <!-- 联网搜索：状态卡——head 显示「联网搜索 + 扫描线」，body 只显示搜索词；
           引用来源暂存到 searchRefs，回复结束后在末尾紫色区块流式渐变展示 -->
      <div v-else-if="group.type === 'search-tool'" class="flow-search-card">
        <div class="flow-search-head">
          <Icon icon="mdi:magnify" class="flow-search-icon" width="14" />
          <span class="flow-search-summary">{{ searchSummary(group.block) }}</span>
          <span class="flow-spacer"></span>
          <span v-if="group.block.status === 'running' || group.block.status === 'generating'" class="flow-search-scan"></span>
        </div>
        <div class="flow-search-body">
          <!-- 搜索词（queries）：后端聚合的 args.query，非 LLM 生成 -->
          <div v-if="searchQuery(group.block)" class="flow-search-query">
            <Icon icon="mdi:magnify" class="flow-search-query-icon" width="12" />
            <span class="flow-search-query-text">{{ searchQuery(group.block) }}</span>
          </div>
          <!-- 搜索进行中：三行扫描线骨架 -->
          <div v-if="group.block.status === 'running' || group.block.status === 'generating'" class="flow-search-loading">
            <span class="flow-search-loading-line"></span>
            <span class="flow-search-loading-line"></span>
            <span class="flow-search-loading-line"></span>
          </div>
        </div>
      </div>

      <!-- ask_user 提问：平铺显示「问了什么 / 答了什么」 -->
      <div v-else-if="group.type === 'question'" class="flow-question">
        <div class="flow-question-head">
          <Icon icon="mdi:help-circle-outline" class="flow-row-icon icon-question" width="14" />
          <span class="flow-question-q">{{ group.block.question }}</span>
        </div>
        <div v-if="group.block.options && group.block.options.length" class="flow-question-opts">
          <span
            v-for="(o, i) in group.block.options"
            :key="i"
            class="flow-question-opt"
            :class="{ chosen: isChosenAnswer(group.block, o.value || o.label) }"
          >{{ o.label }}</span>
        </div>
        <div class="flow-question-a">
          <span class="flow-question-a-label">回答</span>
          <span class="flow-question-a-text">{{ group.block.answer || '（等待中…）' }}</span>
        </div>
      </div>

      <!-- 截图是工作流中的一条内容：默认紧凑预览，点开才展示完整尺寸。 -->
      <button
        v-else-if="group.type === 'image'"
        type="button"
        class="flow-screenshot"
        :class="{ expanded: group.block.expanded }"
        @click="group.block.expanded = !group.block.expanded"
      >
        <span class="flow-screenshot-head">
          <span><Icon icon="mdi:image-outline" width="14" /> 页面截图</span>
          <span>{{ group.block.expanded ? '收起' : '展开' }}</span>
        </span>
        <img :src="group.block.image" :alt="group.block.content || 'Agent 截图'" />
      </button>

      <!-- 记忆写入：单行彩虹反馈（不占卡片，直接铺在聊天流里） -->
      <div v-else-if="group.type === 'memory-saved'" class="flow-memory-saved">
        <span class="fms-scanline"></span>
        <span class="fms-label">已保存到记忆</span>
        <span class="fms-text">{{ group.block.text }}</span>
      </div>

      <!-- 中途插话：轻量提示条，用户插话后模型会按此转向 -->
      <div v-else-if="group.type === 'steer'" class="flow-steer">
        <span class="flow-steer-text">{{ group.block.text }}</span>
      </div>

      <!-- 自动预览提示：弱化条，不抢正文注意力 -->
      <div v-else-if="group.type === 'preview'" class="flow-preview">
        <Icon icon="mdi:eye-outline" class="flow-preview-icon" width="14" />
        <span class="flow-preview-text">{{ group.block.text }}</span>
      </div>

      <!-- 收纳起来的工具/思考时间线 -->
      <template v-else>
        <div
          class="flow-summary"
          @click="toggleSummary(group, gIdx)"
        >
          <div class="flow-summary-main">
            <Icon icon="mdi:console" width="13" class="flow-summary-icon" />
            <span class="flow-summary-text">{{ groupSummaryTitle(group) }}</span>
          </div>
          <span class="flow-chevron" :class="{ open: isSummaryExpanded(gIdx) }">›</span>
        </div>

        <Transition name="flow-body">
          <div v-show="isSummaryExpanded(gIdx)" class="flow-body">
            <template v-for="(b, i) in group.blocks" :key="`${gIdx}-${i}`">
              <!-- 思考 -->
              <div v-if="b.type === 'thinking'" class="flow-thinking flow-thinking-timeline">
                <div class="flow-row-head" @click.stop="toggleThink(`${gIdx}-${i}`)">
                  <Icon icon="mdi:sparkles" class="flow-row-icon icon-think" width="13" />
                  <span class="flow-thinking-text-label">{{ flow.status === 'running' ? '正在思考' : (flow.status === 'waiting' ? '等待后台任务' : '思考') }}</span>
                  <span v-if="!(thinkOpen[`${gIdx}-${i}`] ?? true) && b.text" class="flow-row-preview">{{ onelinePreview(b.text) }}</span>
                  <span v-else class="flow-spacer"></span>
                  <span class="flow-chevron" :class="{ open: thinkOpen[`${gIdx}-${i}`] ?? true }">›</span>
                </div>
                <div v-if="thinkOpen[`${gIdx}-${i}`] ?? true" class="flow-detail">
                  <div class="flow-thinking-text">{{ b.text }}</div>
                </div>
              </div>

              <!-- 操作 -->
              <div v-else-if="b.type === 'tool'" class="flow-tool flow-tool-timeline">
                <div class="flow-row-head" @click.stop="b.expanded = !b.expanded">
                  <Icon icon="mynaui:tool" class="flow-row-icon icon-tool" width="13" />
                  <span class="flow-tool-label">{{ actionText(b) }}</span>
                  <span v-if="diffCounts(b)" class="flow-tool-counts">
                    <span class="flow-add">+{{ diffCounts(b).added }}</span>
                    <span v-if="diffCounts(b).removed" class="flow-del">−{{ diffCounts(b).removed }}</span>
                  </span>
                  <span class="flow-spacer"></span>
                  <span class="flow-tool-badge" :class="'st-' + b.status">
                    <span v-if="b.status === 'running'" class="flow-badge-dot"></span>{{ toolBadge(b) }}
                  </span>
                  <span class="flow-chevron" :class="{ open: b.expanded }">›</span>
                </div>
                <div v-if="b.expanded" class="flow-detail flow-tool-detail">
                  <div class="flow-tool-body">
                    <div v-if="b.status === 'generating' && (isEdit(b.name) || isWrite(b.name))" class="flow-live-diff">
                      <div
                        v-for="row in livePreviewRows(b)"
                        :key="`${row.type}-${row.no}`"
                        class="flow-live-line"
                        :class="'is-' + row.type"
                      >
                        <span class="flow-live-no">{{ row.no }}</span>
                        <span class="flow-live-sign">{{ row.type === 'add' ? '+' : '−' }}</span>
                        <code>{{ row.text || ' ' }}</code>
                      </div>
                    </div>
                    <DiffViewer
                      v-else-if="isEdit(b.name)"
                      :old-content="editOld(b) || ''"
                      :new-content="editNew(b) || ''"
                      :path="filePath(b) || ''"
                      :start-line="editStartLine(b)"
                    />
                    <DiffViewer
                      v-else-if="isWrite(b.name)"
                      old-content=""
                      :new-content="fileContent(b) || ''"
                      :path="filePath(b) || ''"
                    />
                    <div v-else-if="isRead(b.name)" class="flow-read">
                      <div v-for="row in readRows(b)" :key="row.no" class="flow-read-line">
                        <span class="flow-read-no">{{ row.no }}</span>
                        <code class="flow-read-code">{{ row.text || ' ' }}</code>
                      </div>
                    </div>
                    <!-- arXiv 论文检索：可视化卡片预览 -->
                    <ArxivPaperCard v-else-if="b.name === 'arxiv_search'" :output="b.output || ''" />
                    <pre v-else class="flow-output">{{ toolBodyText(b) }}</pre>
                  </div>
                </div>
              </div>
            </template>
          </div>
        </Transition>
      </template>
    </template>

    <!-- 引用来源：回复结束后在末尾展示。思考卡片样式（灰底+左虚线），
         每条【N】编号 + 白底首字徽标 + 标题 + 虚线分隔的网址。
         生产者（搜索时暂存 searchRefs）→ 消费者（回复结束 visibleRefs
         逐条递增，TransitionGroup 流式渐变渲染）。 -->
    <div v-if="searchRefs.length && flow.status !== 'running'" class="flow-refs">
      <div class="flow-refs-title">引用来源</div>
      <TransitionGroup name="flow-ref" tag="div" class="flow-refs-list">
        <a
          v-for="(u, idx) in visibleRefs"
          :key="u"
          class="flow-ref-item"
          :href="u"
          target="_blank"
          rel="noopener"
        >
          <span class="flow-ref-no">{{ idx + 1 }}</span>
          <span class="flow-ref-content">
            <span class="flow-ref-head">
              <span class="flow-search-badge">{{ searchInitial(u) }}</span>
              <span class="flow-search-title">{{ searchTitle(u, null) }}</span>
            </span>
            <span class="flow-search-url">{{ searchHost(u) }}</span>
          </span>
        </a>
      </TransitionGroup>
    </div>
  </div>
</template>

<script setup>
import { reactive, computed, ref, watch } from 'vue'
import { Icon } from '@iconify/vue'
import { diffLines } from 'diff'
import DiffViewer from './DiffViewer.vue'
import ArxivPaperCard from './ArxivPaperCard.vue'
import { renderMarkdown } from './markdownRenderer.js'

const props = defineProps({
  flow: { type: Object, required: true }
})

// ★ 工具调用收进概要组；回复(intent)单独平铺，不收纳
// thinking（思考）与 web_search（联网搜索）也不收束——思考是推理轨迹要
// 全程可见，搜索卡片自带引用来源，收进概要卡片会被折叠看不见。
const blockGroups = computed(() => {
  const groups = []
  let current = null
  for (const b of props.flow?.blocks || []) {
    if (b.type === 'tool' && b.name !== 'web_search') {
      if (!current || current.type === 'visible' || current.type === 'single-thinking' || current.type === 'search-tool') {
        if (current) groups.push(current)
        current = { type: 'summary', blocks: [b] }
      } else {
        current.blocks.push(b)
      }
    } else {
      if (current) {
        groups.push(current)
        current = null
      }
      if (b.type === 'intent') {
        groups.push({ type: 'visible', text: b.text })
      } else if (b.type === 'thinking') {
        // 思考：单步平铺，不收束（推理轨迹全程可见）
        groups.push({ type: 'single-thinking', block: b })
      } else if (b.type === 'question') {
        // ask_user 提问：单独平铺，让用户直接看到「问了什么 / 答了什么」
        groups.push({ type: 'question', block: b })
      } else if (b.type === 'image') {
        groups.push({ type: 'image', block: b })
      } else if (b.type === 'tool' && b.name === 'web_search') {
              // 联网搜索：单独平铺成卡片（自带引用来源，不进概要折叠）
              groups.push({ type: 'search-tool', block: b })
            } else if (b.type === 'steer') {
              // 中途插话：轻量提示条，用户插话后模型会按此转向，给个可见反馈
              groups.push({ type: 'steer', block: b })
            } else if (b.type === 'memory-saved') {
              // 记忆写入：单行彩虹反馈，直接平铺
              groups.push({ type: 'memory-saved', block: b })
      } else if (b.type === 'preview') {
        // 自动预览提示：弱化条，不抢正文注意力
        groups.push({ type: 'preview', block: b })
      }
      // 其他类型（compressed）暂不收纳也不平铺，避免污染回复
    }
  }
  if (current) groups.push(current)
  return groups
})

// ==================== 引用来源（生产者-消费者） ====================
// 生产者：搜索期间（web_search 块到达）持续收集 open_page URL 到 searchRefs，
// 不渲染。消费者：回复结束（flow.status !== 'running'）后 visibleRefs 逐条递增，
// TransitionGroup 流式渐变渲染。解决「图标/标题与 LLM 回答抢渲染」的错位感。
// 【诊断】window.__refsDebug 记录实际数据，排查「结尾不渲染」时看这里。
const searchRefs = computed(() => {
  const seen = new Set()
  const out = []
  for (const b of props.flow?.blocks || []) {
    if (b.type !== 'tool' || b.name !== 'web_search') continue
    for (const u of searchSources(b)) {
      if (!seen.has(u)) {
        seen.add(u)
        out.push(u)
      }
    }
  }
  if (out.length || !window.__refsDebug) {
    window.__refsDebug = window.__refsDebug || { log: [] }
    window.__refsDebug.log.push({
      t: Date.now(), ev: 'searchRefs', count: out.length, urls: [...out],
      flowStatus: props.flow?.status, blocks: (props.flow?.blocks || []).length
    })
    if (window.__refsDebug.log.length > 50) window.__refsDebug.log.shift()
    console.log('[refs] searchRefs computed →', out.length, 'urls, flow.status =', props.flow?.status)
  }
  return out
})
// 消费者：当前已渲染条数，回复结束后由 watch 逐条递增
const refsVisible = ref(0)
let refsTimer = null
const visibleRefs = computed(() => searchRefs.value.slice(0, refsVisible.value))
// 回复结束（status 非 running）触发流式渲染：每 260ms 放出一条。
// immediate + 不依赖 prev：组件可能在任何时序挂载（历史消息/续跑），
// 只要当前已非 running 就立即开始逐条渲染，绝不漏触发。
function startRefsStream() {
  refsVisible.value = 0
  if (refsTimer) clearInterval(refsTimer)
  const total = searchRefs.value.length
  // 【诊断】
  console.log('[refs] startRefsStream → total =', total, ', status =', props.flow?.status)
  if (window.__refsDebug) window.__refsDebug.log.push({ t: Date.now(), ev: 'startRefsStream', total, status: props.flow?.status })
  if (total > 0) {
    refsTimer = setInterval(() => {
      refsVisible.value++
      if (refsVisible.value >= total) clearInterval(refsTimer)
    }, 260)
  }
}
watch(
  () => props.flow?.status,
  (st) => {
    // 【诊断】
    console.log('[refs] watch status →', st, ', prev urls =', searchRefs.value.length)
    if (st && st !== 'running') startRefsStream()
  },
  { immediate: true }
)

// 每个 summary 组的展开状态
const summaryExpanded = reactive({})

// 概要时间线默认收起，不随运行/收尾状态自动展开收起——用户点击才展开，
// 选择会记住；避免运行时卡片突然弹开又合上的突兀感。
function isSummaryExpanded(index) {
  return summaryExpanded[index] ?? false
}
function toggleSummary(group, index) {
  summaryExpanded[index] = !isSummaryExpanded(index)
}

// 各类型 block 统计（按组）
// 概要栏徽章已移除，计数函数一并删除；展开/收起只由用户点击控制。

function groupSummaryTitle(group) {
  const running = group.blocks.some(b => b.status === 'running')
  if (running) {
    const last = group.blocks[group.blocks.length - 1]
    if (last?.type === 'tool') return actionText(last)
    if (last?.type === 'thinking') return '正在思考…'
    return 'Agent 正在处理…'
  }
  return '运行了多个命令'
}

// 收起态思考行的一行预览：取首个非空行、压掉空白、截断
function onelinePreview(text) {
  const s = (text || '').replace(/\s+/g, ' ').trim()
  return s.length > 46 ? s.slice(0, 46) + '…' : s
}
// 工具耗时格式：<1s 显示 ms，否则显示 s
function fmtMs(ms) {
  if (!ms || ms < 0) return ''
  return ms < 1000 ? ms + 'ms' : (ms / 1000).toFixed(1) + 's'
}
// 状态徽章文案：完成带耗时、进行中、失败
function toolBadge(b) {
  if (b.status === 'generating') {
    const chars = Number(b.generatedChars || 0)
    return chars >= 1000 ? `生成参数 ${(chars / 1000).toFixed(1)}k` : '生成参数'
  }
  if (b.status === 'running') return '进行中'
  if (b.status === 'error') return '失败'
  const t = fmtMs(b.elapsedMs)
  return t ? '完成 ' + t : '完成'
}

// ==================== 思考块折叠 ====================
// 思考块默认展开（模板用 thinkOpen[i] ?? true），toggle 基于"当前是否可见"取反：
// 默认未点过视为展开，点一下收起，再点展开。
const thinkOpen = reactive({})
function toggleThink(i) { thinkOpen[i] = !(thinkOpen[i] ?? true) }

// ==================== 动作行文案 ====================
// 一行白话，动词 + 对象，读起来跟正文一样（"编辑了 tools.go"），不靠图标传达语义。
// 运行中把"了"换成"正在…"，这样连状态图标也省了。
const VERBS = {
  read_file: '读取',
  grep: '搜索',
  glob: '查找文件',
  list_directory: '列目录',
  directory_tree: '目录树',
  get_file_info: '文件信息',
  mcp__fs__read_file: '读取',
  mcp__fs__read_text_file: '读取',
  mcp__grep__read_range: '读取',
  write_file: '写入',
  mcp__fs__write_file: '写入',
  mcp__fs__create_file: '新建',
  edit_file: '编辑',
  create_directory: '新建目录',
  move_file: '移动',
  delete_file: '删除',
  delete_directory: '删除目录',
  run_command: '执行命令',
  web_fetch: '抓取网页',
  view_image: '查看图片',
  memory_search: '搜索记忆',
  memory_append: '写入记忆',
  mcp__fs__edit_file: '编辑',
  inject_preview_js: '注入',
  execute_command: '运行',
  search_codebase: '搜索代码库',
  codegraph_query: '分析调用链',
  search_memory: '检索记忆',
  dispatch_agent: '派发子代理',
  web_search: '联网搜索',
  mcp__web_search__web_search: '联网搜索',
  mcp__web_fetch__web_fetch: '抓取网页'
}

function baseName(p) {
  const s = String(p || '')
  const i = Math.max(s.lastIndexOf('/'), s.lastIndexOf('\\'))
  return i >= 0 ? s.slice(i + 1) : s
}

// 动作对象：文件类取文件名（全路径太长且没信息量），命令类取命令原文，其余取首个参数
function target(b) {
  const a = b.args || {}
  const path = filePath(b)
  if (path) return baseName(path)
  const v = a.command || a.task || a.query || Object.values(a)[0] || ''
  const s = String(v)
  return s.length > 48 ? s.slice(0, 48) + '…' : s
}

function actionText(b) {
  // load_tools 只是按需取 MCP 工具 schema 的内部动作，把一串 mcp__fs__read_file,
  // mcp__fs__edit_file 摊开念出来对用户没有信息量，只有噪音——统一成一句轻量提示
  if (b.name === 'load_tools') return b.status === 'running' ? '加载 MCP 工具中…' : '加载了 MCP 工具'
  // 联网搜索（Firecrawl 工具）：显示「搜索到 N 个来源」而不是把一堆
  // 搜索词原样摊开——图2 那种「搜索到 35 个网页」摘要形态。
  if (isWebSearch(b.name)) {
    const n = searchSources(b).length
    const label = n > 0 ? `搜索到 ${n} 个来源` : (b.status === 'running' ? '联网搜索中…' : '联网搜索')
    const q = (b.args && b.args.query) || ''
    return q ? `${label} · ${String(q).slice(0, 30)}${String(q).length > 30 ? '…' : ''}` : label
  }
  const verb = VERBS[b.name] || (b.name.startsWith('mcp__') ? b.name.split('__').slice(1).join(' · ') : b.name)
  const obj = target(b)
  const running = b.status === 'running' || b.status === 'generating'
  // 读文件时把 head/tail/行范围（偏移和限制）显式带出来，否则用户以为每次都读全文
  const range = isRead(b.name) ? readRangeLabel(b) : ''
  const suffix = range ? `（${range}）` : ''
  if (!obj) return running ? `正在${verb}` : `${verb}了`
  return running ? `正在${verb} ${obj}${suffix}` : `${verb}了 ${obj}${suffix}`
}

// 只有写/改文件才有增删行数（对齐设计稿的 "+11 −6"）；其它工具返回 null 不显示。
// 参数现在会流式到达，因此缓存必须以原始参数为 key；不能再只按工具块缓存。
const countsCache = new WeakMap()
function diffCounts(b) {
  const raw = b._rawArgs || JSON.stringify(b.args || {})
  const cached = countsCache.get(b)
  if (cached?.raw === raw) return cached.value
  const v = computeDiffCounts(b)
  countsCache.set(b, { raw, value: v })
  return v
}
function computeDiffCounts(b) {
  if (!isEdit(b.name) && !isWrite(b.name)) return null
  const oldStr = isEdit(b.name) ? editOld(b) : ''
  const newStr = isEdit(b.name) ? editNew(b) : fileContent(b)
  if (b.status === 'generating') {
    const added = Number(b.totalLiveLines || 0)
    return added ? { added, removed: 0 } : null
  }
  let added = 0, removed = 0
  for (const p of diffLines(oldStr || '', newStr || '')) {
    if (!p.added && !p.removed) continue
    const lines = p.value.split('\n')
    if (lines[lines.length - 1] === '') lines.pop()
    if (p.added) added += lines.length
    else removed += lines.length
  }
  return (added || removed) ? { added, removed } : null
}

function isEdit(name) { return name === 'edit_file' || name === 'mcp__fs__edit_file' }
function isWrite(name) {
  return name === 'write_file' || name === 'mcp__fs__write_file' ||
    name === 'mcp__fs__create_file' || name === 'inject_preview_js'
}
function isRead(name) {
  return name === 'read_file' || name === 'mcp__fs__read_file' ||
    name === 'mcp__fs__read_text_file' || name === 'mcp__grep__read_range'
}
// Firecrawl 联网搜索的 web_search 工具卡片（工具结果 URL 由后端 result 事件透出）
function isWebSearch(name) {
  return name === 'web_search' || name === 'mcp__web_search__web_search'
}
// 引用来源列表：Firecrawl 搜索结果 URL（后端 result 事件透出，回填到 args.urls）
function searchSources(b) {
  const a = b.args || {}
  const urls = Array.isArray(a.urls) ? a.urls : []
  return urls.filter(Boolean)
}
function searchHost(u) {
  try { return new URL(u).hostname.replace(/^www\./, '') } catch { return u }
}
// 常见中文站点域名 → 中文名（图2 那种「图标 + 中文名」的引用来源形态；
// 未知域名回退显示域名本身）
const SEARCH_SITE_NAMES = {
  'thepaper.cn': '澎湃新闻', 'm.jiemian.com': '界面新闻', 'jiemian.com': '界面新闻',
  'yicai.com': '第一财经', '36kr.com': '36氪', 'qq.com': '腾讯新闻',
  '163.com': '网易新闻', 'ifeng.com': '凤凰网', 'news.cn': '新华网',
  'people.com.cn': '人民网', 'cctv.com': '央视网', 'cntv.cn': '央视网',
  'huanqiu.com': '环球网', 'chinanews.com.cn': '中新网', 'xinhuanet.com': '新华网',
  'hgdaily.com.cn': '黄冈日报', 'cngold.org': '金投网', 'sina.com.cn': '新浪新闻',
  'zhihu.com': '知乎', 'bilibili.com': '哔哩哔哩', 'sohu.com': '搜狐新闻',
  'thecover.cn': '封面新闻', 'stdaily.com': '科技日报', 'ce.cn': '中国经济网',
  'gov.cn': '中国政府网', 'cnr.cn': '央广网', '12371.cn': '共产党员网',
  'gmw.cn': '光明网', 'china.com.cn': '中国网', 'cnstock.com': '上海证券报',
}
// 站点中文名/图标：不请求任何外部 favicon 服务（Google 在大陆会挂起超时，
// 每张图卡几秒整个 UI 卡死）。用站点名首字生成本地圆形徽标，零网络依赖。
function searchSiteName(u) {
  const host = searchHost(u)
  // 去掉端口和子域细节，匹配主域名表；支持 m.xx.com → xx.com
  for (const [domain, name] of Object.entries(SEARCH_SITE_NAMES)) {
    if (host === domain || host.endsWith('.' + domain)) return name
  }
  return host
}
// 本地首字图标（替代 favicon）：取中文站点名首字符，未知域名取域名首字母大写
function searchInitial(u) {
  const name = searchSiteName(u)
  const c = name.trim().charAt(0)
  return c ? c.toUpperCase() : '网'
}
// 站点 favicon：favicon.im（国内可直连，实测 1.3s 返回真实图标）。
// 不用 Google s2/favicons——大陆访问 Google 挂起超时（实测 HTTP 000 + 8s），
// 每个来源卡几秒整个 UI 卡死。
function searchFavicon(u) {
  const host = searchHost(u)
  return `https://favicon.im/${encodeURIComponent(host)}`
}
// favicon 加载状态：加载失败回退本地首字徽标（faviconFailed 是 Set）
const faviconFailed = new Set()
// faviconBump 是响应式计数器：img error 时自增，让 faviconOK 重新求值
// （Set 本身不触发 Vue 依赖，纯 ref 变化才能驱动模板重渲染）
const faviconBump = ref(0)
function faviconOK(u) {
  void faviconBump.value // 建立响应式依赖
  return !faviconFailed.has(u)
}
function onFaviconError(e, u) {
  faviconFailed.add(u)
  faviconBump.value++
}
// 来源标题：白底首字徽标旁显示真实新闻标题。DS open_page 只给 URL，
// 标题走后端 /api/fetch-title 代理抓 <title>。抓取失败保持站名不阻塞 UI。
// 预热：URL 进入 searchRefs 后立即抓（不等渲染），渲染时多数已命中缓存。
const searchTitleCache = new Map() // url → title
const searchTitleFetching = new Set()
const searchTitleBump = ref(0)
function searchTitle(u) {
  void searchTitleBump.value // 建立响应式依赖：fetch 完成时 bump 触发重渲染
  const cached = searchTitleCache.get(u)
  if (cached) return cached
  fetchTitle(u)
  return searchSiteName(u)
}
function fetchTitle(u) {
  if (searchTitleCache.has(u) || searchTitleFetching.has(u)) return
  searchTitleFetching.add(u)
  fetch(`/api/fetch-title?url=${encodeURIComponent(u)}`, { signal: AbortSignal.timeout(6000) })
    .then(r => r.json())
    .then(d => { if (d && d.title) searchTitleCache.set(u, d.title) })
    .catch(() => { /* 抓取失败保持站名 */ })
    .finally(() => {
      searchTitleFetching.delete(u)
      searchTitleBump.value++
    })
}
// 预热：引用收集到就抓标题（渲染在回复结束后，标题先到缓存）
watch(searchRefs, (urls) => { for (const u of urls) fetchTitle(u) }, { immediate: true })
// 搜索词（queries）：后端 SSE args.query 已聚合（过滤了 ws_call_id 尾巴）
function searchQuery(b) {
  return (b.args && b.args.query) || ''
}
// 搜索卡概要：head 只显示状态，不重复搜索词（搜索词在 body 完整展示）
function searchSummary(b) {
  return (b.status === 'running' || b.status === 'generating') ? '正在联网搜索…' : '联网搜索完成'
}

// 把各种"读一段"的参数翻成人话贴在动作行尾（offset=起点, limit=实际行数）。
// 之前前端只认老 native tool 的 start_line/end_line，MCP 的 head/tail、自研 read_range
// 的 start/end 都没显示，所以读文件看起来永远是"读全文"。覆盖三套命名：
//   mcp__fs__read_text_file → head / tail（头/尾 N 行）
//   mcp__grep__read_range   → start / end（第 X–Y 行，能读中间任意段）
//   老 native read_file     → start_line / end_line / mode=outline
//
// limit 优先用"实际返回的行数"——从 read_range 输出首行的元信息 "# 路径 第 X-Y 行" 里
// 解析出来：agent 传 start=6 不传 end 时，工具会默认读到 400 行（甚至文件末尾），
// 把这个真实数字展示出来比 "limit=400" 更有信息量；正好也能看出"是不是浪费 token 了"。
function readRangeLabel(b) {
  const a = b.args || {}
  const head = parseInt(a.head, 10)
  const tail = parseInt(a.tail, 10)
  const s = parseInt(a.start ?? a.start_line, 10)
  const e = parseInt(a.end ?? a.end_line, 10)
  // read_range 的元信息行 "# path 第 X-Y 行（共 N 行）"：X=真实起点, Y=真实终点
  // 只有成功时才有元信息行，失败时看 agent 传的参数
  if (b.status !== 'error' && b.output) {
    const metaMatch = /^#\s.+\s第\s(\d+)-(\d+)\s行/.exec(b.output.split('\n')[0] || '')
    if (metaMatch) {
      const ms = parseInt(metaMatch[1], 10), me = parseInt(metaMatch[2], 10)
      if (Number.isFinite(ms) && Number.isFinite(me)) return `offset=${ms}, limit=${me - ms + 1}`
    }
  }
  if (Number.isFinite(head)) return `offset=0, limit=${head}`
  if (Number.isFinite(tail)) return `offset=-${tail}, limit=${tail}`
  if (Number.isFinite(s) && Number.isFinite(e)) return `offset=${s}, limit=${e - s + 1}`
  if (Number.isFinite(s)) return `offset=${s}, limit=400`
  if (a.mode === 'outline') return 'offset=0, limit=outline'
  return 'offset=0, limit=full'
}

function compactChars(n) {
  if (!n) return '0'
  if (n >= 1000) return (n / 1000).toFixed(1).replace(/\.0$/, '') + 'k'
  return String(n)
}

// read 输出的行号来源不统一：
//   - range 模式（native read_file / mcp__grep__read_range）每行已带真实行号 "12:内容"
//   - outline 模式带 "L12  内容"
//   直接解析出来用；
//   - 全文模式（read_text_file 最常见）不带行号，退化为从 start/start_line（有就用，没有 1）顺序编号。
// read_range 的首行是 "# 路径 第 X-Y 行(...)" 元信息，不是正文，跳过不显示。
function readRows(b) {
  const raw = b.output || ''
  if (!raw) return []
  const lines = raw.split('\n')
  if (lines.length && lines[lines.length - 1] === '') lines.pop()
  const startArg = parseInt(b.args?.start ?? b.args?.start_line, 10)
  let base = Number.isFinite(startArg) ? startArg : 1
  const rows = []
  for (const line of lines) {
    if (/^#\s/.test(line)) continue // read_range 的头部元信息行
    const rangeMatch = /^(\d+):(.*)$/.exec(line)
    if (rangeMatch) { rows.push({ no: rangeMatch[1], text: rangeMatch[2] }); continue }
    const outlineMatch = /^L(\d+)\s+(.*)$/.exec(line)
    if (outlineMatch) { rows.push({ no: outlineMatch[1], text: outlineMatch[2] }); continue }
    rows.push({ no: base + rows.length, text: line })
  }
  return rows
}

// MCP filesystem 的 edit_file 真实 schema：{ path, edits: [{oldText, newText}] }（数组，
// 每项一对 oldText/newText）。内置 edit_file 是 { path, old_string, new_string }（单数）。
// 两者都要兼容。write_file 内容字段内置/MCP 都是 content，path 都是 path。
function streamedArg(b, key) {
  const raw = b._rawArgs || ''
  // 截取 JSON 字符串的已到达部分；JSON.parse 负责还原转义。流恰好停在反斜杠时，
  // 保留上一次可解析内容，下一批 token 到来后自然补全。
  const match = new RegExp(`"${key}"\\s*:\\s*"((?:\\\\.|[^"\\\\])*)`).exec(raw)
  if (!match) return undefined
  try { return JSON.parse(`"${match[1]}"`) } catch { return undefined }
}
function argValue(b, ...keys) {
  const a = b.args || {}
  for (const key of keys) {
    const streamed = streamedArg(b, key)
    if (streamed !== undefined) return streamed
    if (a[key] != null) return a[key]
  }
  return ''
}
function editOld(b) {
  const a = b.args || {}
  if (a.old_string) return a.old_string
  return argValue(b, 'old_string', 'oldText')
}
function editNew(b) {
  const a = b.args || {}
  if (a.new_string) return a.new_string
  return argValue(b, 'new_string', 'newText')
}
function fileContent(b) {
  return b.name === 'inject_preview_js' ? argValue(b, 'js') : argValue(b, 'content')
}
function filePath(b) {
  if (b.name === 'inject_preview_js') return 'preview/injected.js'
  return argValue(b, 'path')
}

// 生成阶段使用纯文本、固定行数的轻量预览。禁止挂载 DiffViewer：后者会在每次
// 流式刷新时对每一行执行 highlight.js，长 HTML 足以持续占满浏览器主线程。
const LIVE_PREVIEW_LINES = 36
const livePreviewCache = new WeakMap()
function livePreviewData(b) {
  if (Array.isArray(b.liveLines)) {
    return {
      rows: b.liveLines.map(row => ({ ...row, type: 'add' }))
    }
  }
  const raw = b._rawArgs || ''
  const cached = livePreviewCache.get(b)
  if (cached?.raw === raw) return cached.value
  const rows = []
  const append = (text, type) => {
    const lines = (text || '').split('\n')
    for (let i = 0; i < lines.length && rows.length < LIVE_PREVIEW_LINES; i++) {
      rows.push({ type, no: i + 1, text: lines[i] })
    }
  }
  if (isEdit(b.name)) append(editOld(b), 'del')
  append(isEdit(b.name) ? editNew(b) : fileContent(b), 'add')
  const value = { rows }
  livePreviewCache.set(b, { raw, value })
  return value
}
function livePreviewRows(b) { return livePreviewData(b).rows }

// 判断某个选项是否被用户的回答命中（answer 可能是「A、B」这类拼接，或自由文本）
function isChosenAnswer(block, value) {
  const ans = (block.answer || '').trim()
  if (!ans) return false
  if (ans === value) return true
  // 多选题答案形如「A、B」，按顿号/、切分后看是否含该 value
  return ans.split(/[、,]/).map(s => s.trim()).includes(value)
}

// edit_file 结果里带 "第 N 行"，用来给 Diff 做行号偏移
function editStartLine(b) {
  const m = /第\s*(\d+)\s*行/.exec(b.output || '')
  return m ? parseInt(m[1]) : 1
}

function toolBodyText(b) {
  const out = b.output || (b.status === 'running' ? '执行中…' : '(无输出)')
  if (b.name === 'execute_command') return `$ ${b.args.command || ''}\n\n${out}`
  if (b.name === 'dispatch_agent') return `任务：${b.args.task || ''}\n\n${out}`
  return out
}
</script>

<style scoped>
/* .message-row 是 flex 容器，子元素默认按内容宽度收缩（shrink-to-fit）。
   用户气泡 .message-bubble.user 和纯文本回答 .assistant-message 都显式撑了
   width:100%，这里漏了同一条，工具卡片那一列就只有文字本身那么宽，跟上下
   气泡对不齐——width:100% 让它跟其余消息块占满同一条列宽。 */
.agent-flow {
  width: 100%;
  max-width: 100%;
  padding: 2px 0;
}

.flow-live-diff {
  overflow: hidden;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: var(--app-surface);
  font: 11.5px/1.6 ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  /* 跟主聊天区相同：渐变属于稳定的视口遮罩，不给每一行反复做 opacity 动画。 */
  -webkit-mask-image: linear-gradient(
    to bottom,
    rgba(0, 0, 0, 0.3) 0,
    rgba(0, 0, 0, 0.8) 30px,
    #000 72px,
    #000 100%
  );
  mask-image: linear-gradient(
    to bottom,
    rgba(0, 0, 0, 0.3) 0,
    rgba(0, 0, 0, 0.8) 30px,
    #000 72px,
    #000 100%
  );
}
.flow-live-line {
  display: grid;
  grid-template-columns: 34px 16px minmax(0, 1fr);
  min-height: 18px;
}
.flow-live-line.is-add { background: rgba(18, 183, 106, 0.10); }
.flow-live-line.is-del { background: rgba(217, 72, 52, 0.08); }
.flow-live-no {
  padding-right: 8px;
  color: var(--app-text-faint);
  text-align: right;
  user-select: none;
}
.flow-live-sign {
  color: #12b76a;
  text-align: center;
  font-weight: 700;
}
.flow-live-line.is-del .flow-live-sign { color: #d94834; }
.flow-live-line code {
  min-width: 0;
  padding-right: 10px;
  overflow: hidden;
  color: var(--app-text);
  background: transparent;
  white-space: pre;
  text-overflow: ellipsis;
}

/* ---------- 概要栏：把一次 agent 回复之间的思考和工具调用都收纳进来 ----------
   扁平行、无卡片底：跟思考/操作行同一套视觉语言，运行/已结束不再区分卡片，
   运行中靠文字高光扫描动画（reasonShimmer）表达状态。 */
.flow-summary {
  display: flex;
  align-items: center;
  gap: 7px;
  margin: 1px 0;
  padding: 4px 2px;
  border-radius: 6px;
  cursor: pointer;
  user-select: none;
  transition: background 0.14s ease;
}
.flow-summary:hover {
  background: var(--app-surface-2);
}
/* 徽章已移除，让 chevron 靠右 */
.flow-summary > .flow-chevron {
  margin-left: auto;
}
.flow-summary-main {
  display: flex;
  align-items: center;
  gap: 7px;
  min-width: 0;
  flex-shrink: 1;
}
.flow-summary-icon {
  flex-shrink: 0;
  width: 13px;
  height: 13px;
  color: var(--app-text-faint);
}
.flow-summary-text {
  font-size: 11.5px;
  font-weight: 400;
  color: var(--app-text-soft);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
/* 运行中：跟思考标签一样的高光扫描动画（keyframes 在 chat-global.css） */
.agent-flow.streaming .flow-summary-text {
  animation: reasonShimmer 3s linear infinite;
  background: linear-gradient(100deg, var(--app-text-soft) 40%, var(--app-text) 50%, var(--app-text-soft) 60%);
  background-size: 250% 100%;
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}

/* 折叠/展开动画 */
.flow-body-enter-active,
.flow-body-leave-active {
  transition: all 0.2s ease;
  overflow: hidden;
}
.flow-body-enter-from,
.flow-body-leave-to {
  opacity: 0;
  max-height: 0;
}
.flow-body-enter-to,
.flow-body-leave-from {
  opacity: 1;
  max-height: 800px;
}

/* 展开的思考 / 工具节点属于概要的子级，整体向右缩进；图标样式保持一致，
   但不与概要左侧图标共用同一竖轴。 */
.flow-body {
  margin: 4px 0 0 24px;
}
/* 时间线里的标题：灰色小字、去卡片底 */
.flow-body .flow-row-head,
.flow-thinking-single .flow-row-head {
  position: relative;
  background: transparent;
  border: none;
  padding: 4px 0;
  font-size: 12.5px;
  color: var(--app-text-soft);
}
.flow-body .flow-row-head:hover,
.flow-thinking-single .flow-row-head:hover {
  background: transparent;
}
/* 节点的详情再缩进一级并显示竖线。 */
.flow-detail {
  margin: 4px 0 6px 24px;
  padding: 0 0 2px 16px;
  border-left: 2px solid var(--app-border);
  min-width: 0;
}
.flow-tool-detail { margin-top: 6px; }

/* ---------- 首条回复前的「正在思考」扫描线（不可折叠，首字到即隐藏） ---------- */
.flow-pending-scanline {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 0;
  margin: 4px 0;
  font-size: 12px;
  color: var(--app-text-soft);
}
.flow-pending-scanline .flow-pending-label {
  font-weight: 500;
  font-size: 12.5px;
  color: var(--app-text);
  white-space: nowrap;
  /* 复用 reasoning 的白色高光扫描 */
  background: linear-gradient(100deg, var(--app-text) 40%, var(--app-accent) 50%, var(--app-text) 60%);
  background-size: 250% 100%;
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  animation: reasonShimmer 6s linear infinite;
  }

  /* ---------- 思考 ---------- */
.flow-thinking {
  margin: 6px 0;
}
/* 上下文压缩提示：跟思考块一样"轻"，不抢注意力——它是后台省 token 的动作，
   不是用户要读的内容。左侧一条竖线 + 弱化文字。 */
.flow-compressed {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 6px 0;
  padding: 3px 0 3px 12px;
  border-left: 2px solid var(--app-border, #e2e8f0);
  font-size: 12px;
  color: var(--app-text-faint);
}
.flow-compressed-icon { font-size: 12px; opacity: 0.8; }
/* 中途插话提示：用户插话——左侧强调色竖线 + 正文，不做卡片。 */
.flow-steer {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 6px 0;
  padding: 6px 0 6px 14px;
  border-left: 3px solid var(--app-accent, #6366f1);
  font-size: 16px;
  color: var(--app-text);
  line-height: 1.6;
}
.flow-steer-text { flex: 1; min-width: 0; }
/* 记忆写入反馈：单行彩虹渐变文字 + 左侧发光扫描线，不占卡片 */
.flow-memory-saved {
  position: relative;
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 8px 0 8px 14px;
  padding: 5px 0 5px 14px;
  font-size: 15px;
  font-weight: 600;
  line-height: 1.5;
  border-left: 3px solid rgba(179, 157, 219, 0.6);
}
.fms-scanline {
  position: absolute;
  left: -14px;
  top: 0;
  bottom: 0;
  width: 3px;
  background: linear-gradient(180deg, #ff9a5c, #ff7eb3, #b39ddb);
  animation: fms-pulse 1.6s ease-in-out infinite;
  border-radius: 2px;
}
@keyframes fms-pulse {
  0%, 100% { opacity: 0.55; }
  50% { opacity: 1; }
}
.fms-label {
  background: linear-gradient(90deg, #ff9a5c, #ff7eb3, #b39ddb);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  white-space: nowrap;
}
.fms-text {
  color: var(--app-text-soft, #6b6b6b);
  font-weight: 400;
  font-size: 13px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}
/* 自动预览提示：跟插话提示同款弱化条 */
.flow-preview {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 6px 0;
  padding: 3px 0 3px 12px;
  border-left: 2px solid var(--app-accent, #6366f1);
  font-size: 12px;
  color: var(--app-text-faint);
  word-break: break-all;
}
.flow-preview-icon { font-size: 12px; opacity: 0.8; }
.flow-preview-text { flex: 1; min-width: 0; word-break: break-all; }
/* ---------- 卡片行（思考/工具共用）：仿图1 的一行小卡片 ---------- */
.flow-row-head {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 6px 11px;
  border-radius: 9px;
  background: var(--app-surface-2, var(--app-code-bg));
  border: 1px solid var(--app-border-soft, #ededf0);
  cursor: pointer;
  user-select: none;
  font-size: 13.5px;
  transition: background 0.14s ease, border-color 0.14s ease;
}
.flow-row-head:hover {
  background: var(--app-surface-3);
  border-color: #e2e2e6;
}
.flow-row-icon { flex-shrink: 0; }
.icon-think { color: #8b5cf6; }
.icon-search { color: #8b5cf6; }
.icon-tool { color: var(--app-text-soft); }

/* ---------- 联网搜索独立卡片（透明背景，非工具卡片） ---------- */
.flow-search-card {
  margin: 2px 0;
  background: transparent;
  border: none;
}
/* 引用来源（回复末尾）：思考卡片样式——灰色背景 + 左边虚线，与工作流区分 */
.flow-refs {
  margin: 8px 0 4px;
  padding: 8px 10px;
  border-radius: 10px;
  background: var(--app-surface-2, var(--app-code-bg));
  border: 1px solid var(--app-border-soft, #ededf0);
}
.flow-refs-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--app-text-soft);
  padding: 0 2px 6px;
}
.flow-refs-list {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding-left: 10px;
}
/* 左侧竖直线：整根连续线贯穿整个列表（容器级伪元素，非每条各自画） */
.flow-refs-list::before {
  content: '';
  position: absolute;
  left: 1px;
  top: 2px;
  bottom: 2px;
  width: 1px;
  background: var(--app-border-strong, #c9c9d1);
}
/* 单条引用：编号 + 内容（标题行 / 网址行分隔） */
.flow-ref-item {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 2px 2px;
  text-decoration: none;
  color: var(--app-text);
}
.flow-ref-item:hover {
  background: var(--app-surface-3, rgba(0,0,0,0.04));
  border-radius: 6px;
}
.flow-ref-no {
  flex-shrink: 0;
  font-size: 11px;
  font-weight: 700;
  color: #8b5cf6;
  padding-top: 1px;
}
.flow-ref-content {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.flow-ref-head {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}
/* 白底首字徽标（替代 favicon：零网络依赖、不等待加载） */
.flow-search-badge {
  flex-shrink: 0;
  width: 18px;
  height: 18px;
  border-radius: 5px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 10.5px;
  font-weight: 700;
  color: #8b5cf6;
  background: #fff;
  border: 1px solid var(--app-border-soft, #e5e5ea);
}
.flow-search-title {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 12.5px;
  font-weight: 500;
  color: var(--app-text);
}
/* 网址：虚线分隔，灰色小字 */
.flow-search-url {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--app-text-faint);
  font-size: 11.5px;
  border-top: 1px dashed var(--app-border-soft, #e2e2e8);
  padding-top: 2px;
}
/* 每条引用渐变出现：淡入 + 轻微下移 */
.flow-ref-enter-active {
  transition: opacity 0.35s ease, transform 0.35s ease;
}
.flow-ref-enter-from {
  opacity: 0;
  transform: translateY(4px);
}
.flow-ref-leave-active {
  transition: opacity 0.2s ease;
}
.flow-ref-leave-to {
  opacity: 0;
}
/* 搜索卡 head：联网搜索 + 扫描线 */
.flow-search-head {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 2px;
  cursor: pointer;
  user-select: none;
  border-bottom: 1px solid var(--app-border-soft, rgba(0,0,0,0.06));
}
.flow-search-icon {
  flex-shrink: 0;
  color: #8b5cf6;
}
.flow-search-summary {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 12.5px;
  color: var(--app-text);
}
/* 搜索进行中：右侧呼吸圆点（扫描线动画） */
.flow-search-scan {
  flex-shrink: 0;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #8b5cf6;
  animation: flow-search-pulse 1.1s ease-in-out infinite;
}
@keyframes flow-search-pulse {
  0%, 100% { opacity: 0.25; transform: scale(0.8); }
  50% { opacity: 1; transform: scale(1.15); }
}
.flow-search-body {
  padding: 6px 2px 2px;
}
/* 搜索词（queries）行：放大镜 + 搜索词，多词用「；」分隔 */
.flow-search-query {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 3px 4px 6px;
  font-size: 12px;
  color: var(--app-text-faint);
}
.flow-search-query-icon {
  flex-shrink: 0;
  color: #8b5cf6;
  opacity: 0.7;
}
.flow-search-query-text {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
/* 流式加载占位：三行扫描线骨架 */
.flow-search-loading {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 4px 0;
}
.flow-search-loading-line {
  height: 10px;
  border-radius: 5px;
  background: linear-gradient(90deg, rgba(139,92,246,0.08) 25%, rgba(139,92,246,0.22) 50%, rgba(139,92,246,0.08) 75%);
  background-size: 200% 100%;
  animation: flow-search-shimmer 1.4s linear infinite;
}
.flow-search-loading-line:nth-child(2) { width: 85%; }
.flow-search-loading-line:nth-child(3) { width: 60%; }
@keyframes flow-search-shimmer {
  0% { background-position: 200% 0; }
  100% { background-position: -200% 0; }
}
.flow-search-source {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 4px;
  border-radius: 6px;
  text-decoration: none;
  color: var(--app-text);
  font-size: 12.5px;
}
.flow-search-source:hover {
  background: var(--app-surface-2, rgba(0,0,0,0.04));
}
/* 站点图标徽标：白底（favicon 透出真实图标），加载失败回退本地首字
   （主题紫文字，白底可读；不用主题色背景——favicon 会被染成紫色） */
.flow-search-badge {
  flex-shrink: 0;
  width: 18px;
  height: 18px;
  border-radius: 5px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  position: relative;
  overflow: hidden;
  font-size: 10.5px;
  font-weight: 700;
  color: #8b5cf6;
  background: #fff;
}
.flow-search-favicon {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: contain;
}
.flow-search-badge-fallback {
  line-height: 1;
}
.flow-search-title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--app-text);
  font-weight: 500;
  /* 新闻标题带浅色下划线：可点击链接感（图2 那种来源列表形态） */
  text-decoration: underline;
  text-decoration-color: rgba(100, 116, 139, 0.35);
  text-underline-offset: 3px;
}
.flow-search-url {
  flex-shrink: 0;
  max-width: 38%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--app-text-faint);
  font-size: 11.5px;
}
.flow-search-empty {
  padding: 6px 2px;
  color: var(--app-text-faint);
  font-size: 12px;
}
.flow-row-preview {
  flex: 1;
  min-width: 0;
  color: var(--app-text-faint);
  font-size: 12.5px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.flow-spacer { flex: 1; }
/* 状态徽章：完成(绿)/进行中(灰+脉冲点)/失败(红)，胶囊底 */
.flow-tool-badge {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 11.5px;
  font-weight: 600;
  padding: 1.5px 8px;
  border-radius: 999px;
  font-variant-numeric: tabular-nums;
}
.flow-tool-badge.st-ok { color: #12b76a; background: rgba(18, 183, 106, 0.1); }
.flow-tool-badge.st-error { color: #d94834; background: rgba(217, 72, 52, 0.1); }
.flow-tool-badge.st-running { color: var(--app-text-soft); background: rgba(100, 116, 139, 0.1); }
.flow-badge-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
  animation: flowBadgePulse 1.2s ease-in-out infinite;
}
@keyframes flowBadgePulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.4; transform: scale(0.7); }
}

.flow-thinking-label {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 17px;
  cursor: pointer;
  user-select: none;
}
/* 思考标签：默认灰色小字；streaming 时轻微闪烁 */
.flow-thinking-text-label {
  color: var(--app-text-soft);
  font-size: inherit;
}
.agent-flow.streaming .flow-thinking-text-label {
  animation: reasonShimmer 3s linear infinite;
  background: linear-gradient(100deg, var(--app-text-soft) 40%, var(--app-text) 50%, var(--app-text-soft) 60%);
  background-size: 250% 100%;
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}
/* 思考正文：灰色小字，左侧时间线由 .flow-detail 提供 */
.flow-thinking-text {
  padding: 2px 0;
  font-size: 12px;
  line-height: 1.7;
  color: var(--app-text-faint);
  background: transparent;
  white-space: pre-wrap;
  word-break: break-word;
}

/* ---------- 意图/最终回答 ---------- */
/* 合并后普通对话的 bot 回答就渲染在这里。chat-window.css 里把用户气泡
   和 .assistant-message 都提到了 17px（Claude 风格），但 agentflow 面板是
   scoped 样式、不吃那条规则，原本硬编码 14px —— 于是合并后 bot 字
   明显比用户小。这里对齐到 17px，落差消失。 */
.flow-intent {
  margin: 6px 0;
  font-size: 17px;
  line-height: 1.75;
  color: var(--app-text);
  word-break: break-word;
}

/* 截图默认只占一行半的紧凑预览，不抢走聊天阅读节奏；点击后才展开全图。 */
.flow-screenshot {
  display: block;
  width: min(100%, 360px);
  margin: 10px 0;
  padding: 0;
  overflow: hidden;
  border: 1px solid color-mix(in srgb, var(--app-accent) 16%, var(--app-border));
  border-radius: 10px;
  background: var(--app-surface-2);
  color: var(--app-text-soft);
  cursor: pointer;
  text-align: left;
  transition: border-color .16s ease, box-shadow .16s ease;
}
.flow-screenshot:hover { border-color: color-mix(in srgb, var(--app-accent) 38%, var(--app-border)); box-shadow: 0 4px 14px color-mix(in srgb, var(--app-accent) 10%, transparent); }
.flow-screenshot-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 30px;
  padding: 0 9px;
  color: var(--app-text-faint);
  font-size: 11px;
}
.flow-screenshot-head span:first-child { display: inline-flex; align-items: center; gap: 5px; }
.flow-screenshot-head svg { color: var(--app-accent); }
.flow-screenshot img {
  display: block;
  width: 100%;
  height: 72px;
  object-fit: cover;
  object-position: top;
  background: var(--app-surface-3);
  transition: height .2s ease;
}
.flow-screenshot.expanded { width: min(100%, 680px); }
.flow-screenshot.expanded img { height: auto; max-height: 520px; object-fit: contain; }

/* ---------- 操作行 ---------- */
/* 收起态就是一行正文：无边框、无底色、无徽章，字号字色跟 .flow-intent 一致，
   读起来像在叙述而不是像一张控件卡片。白卡片留给展开后的 Diff / 输出。 */
.flow-tool {
  margin: 6px 0;
}
.flow-tool-label {
  min-width: 0;
  max-width: 60%;
  font-size: inherit;
  line-height: 1.5;
  color: inherit;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.flow-tool-counts {
  flex-shrink: 0;
  display: inline-flex;
  gap: 5px;
  font-size: 13px;
  font-weight: 600;
  font-family: var(--app-mono-font, ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace);
}
.flow-add { color: #12b76a; }
.flow-del { color: #d94834; }
.flow-tool-failed {
  flex-shrink: 0;
  font-size: 13px;
  color: #d94834;
}
.flow-chevron {
  flex-shrink: 0;
  color: var(--app-text-faint);
  font-size: 14px;
  transition: transform 0.15s;
  display: inline-block;
  /* 折叠朝右▸，展开向下▾ */
}
.flow-chevron.open {
  transform: rotate(90deg);
}
/* 展开态才出现的白卡片：真正装 Diff / 命令输出的地方 */
.flow-tool-body {
  margin: 6px 0 2px;
  border: 1px solid var(--app-border);
  border-radius: 10px;
  background: var(--app-surface);
  padding: 8px 12px;
  overflow: hidden;
}
.flow-read {
  max-height: 320px;
  overflow: auto;
  font-family: var(--app-mono-font, ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace);
  font-size: 11.5px;
  line-height: 1.6;
}
.flow-read-line {
  display: flex;
  align-items: flex-start;
}
.flow-read-no {
  flex-shrink: 0;
  width: 34px;
  text-align: right;
  padding-right: 10px;
  color: var(--app-text-faint);
  user-select: none;
  font-family: var(--app-mono-font, ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace);
  font-size: 11.5px;
}
.flow-read-code {
  flex: 1;
  min-width: 0;
  color: var(--app-text);
  background: transparent;
  font-family: var(--app-mono-font, ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace);
  font-size: 11.5px;
  white-space: pre-wrap;
  word-break: break-all;
}
.flow-output {
  margin: 0;
  max-height: 320px;
  overflow: auto;
  font-size: 12px;
  line-height: 1.6;
  color: var(--app-text);
  font-family: var(--app-mono-font, ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace);
  white-space: pre-wrap;
  word-break: break-all;
}

/* ---------- ask_user 提问块 ---------- */
.flow-question {
  padding: 10px 12px;
  border-radius: 10px;
  background: var(--app-surface-2);
  border: 1px solid var(--app-border-soft);
  margin: 4px 0;
}
.flow-question-head {
  display: flex;
  align-items: flex-start;
  gap: 6px;
}
.icon-question {
  flex-shrink: 0;
  margin-top: 2px;
  color: var(--app-accent);
}
.flow-question-q {
  font-size: 13px;
  color: var(--app-text);
  line-height: 1.5;
  font-weight: 500;
}
.flow-question-opts {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 8px;
}
.flow-question-opt {
  font-size: 12px;
  padding: 3px 10px;
  border-radius: 12px;
  border: 1px solid var(--app-border);
  color: var(--app-text-soft);
  background: var(--app-surface);
}
.flow-question-opt.chosen {
  border-color: var(--app-accent);
  background: var(--app-accent-soft);
  color: var(--app-accent);
}
.flow-question-a {
  display: flex;
  align-items: baseline;
  gap: 6px;
  margin-top: 8px;
}
.flow-question-a-label {
  flex-shrink: 0;
  font-size: 11px;
  color: var(--app-text-faint);
}
.flow-question-a-text {
  font-size: 13px;
  color: var(--app-text);
  line-height: 1.5;
  word-break: break-word;
}
</style>
