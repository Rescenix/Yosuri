<template>
  <div class="agent-flow" :class="{ streaming: flow.status === 'running' }">
    <!-- 首条回复前的「正在思考」扫描线：不可折叠、无 chevron；首字一到（blockGroups 非空）自动消失 -->
    <div v-if="flow.status === 'running' && blockGroups.length === 0" class="flow-pending-scanline">
          <RoseParticleLoader :size="20" class="flow-pending-loader" />
          <span class="flow-pending-label">{{ tr('正在思考') }}</span>
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
        :class="{ 'flow-retry-note': group.retryNote }"
        v-html="group.html"
      ></div>

      <!-- 单步思考：不收束，直接平铺（思考中展开，结束后自动收起+显示耗时） -->
            <div v-else-if="group.type === 'single-thinking'" class="flow-thinking flow-thinking-single">
                          <div class="flow-row-head" @click="toggleThink(`single-${gIdx}`, group.block)">
                                          <RoseParticleLoader v-if="!thinkDone(group.block)" :size="18" class="flow-row-icon icon-think" />
                                                                        <Icon v-else icon="mdi:thought-bubble-outline" class="flow-row-icon icon-think" width="16" />
                            <span class="flow-thinking-text-label">{{ thinkLabelText(group.block) }}</span>
                <span v-if="thinkCollapsed(`single-${gIdx}`, group.block) && group.block.text" class="flow-row-preview">{{ onelinePreview(group.block.text) }}</span>
                <span v-else class="flow-spacer"></span>
                <span class="flow-chevron" :class="{ open: thinkIsOpen(`single-${gIdx}`, group.block) }">›</span>
              </div>
              <div v-if="thinkIsOpen(`single-${gIdx}`, group.block)" class="flow-detail flow-thinking-detail">
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
          <span class="flow-question-a-label">{{ tr('回答') }}</span>
          <!-- answered=false = 超时/断线走 fallback 兜底，不是用户的选择，不伪装成用户回答 -->
          <span class="flow-question-a-text">{{ group.block.answered ? (group.block.answer || tr('（等待中…）')) : tr('（未回答，已按兜底继续）') }}</span>
        </div>
      </div>

      <!-- 截图：不再折叠，默认直接展示完整图片。 -->
            <div
              v-else-if="group.type === 'image'"
              class="flow-screenshot"
            >
              <span class="flow-screenshot-head">
                <span><Icon icon="mdi:image-outline" width="14" /> {{ tr('页面截图') }}</span>
              </span>
              <img :src="group.block.image" :alt="group.block.content || tr('Agent 截图')" />
            </div>

      <!-- 生成视频：内嵌可拖动进度条播放块（同图片内嵌块模式，默认展开可播） -->
      <div v-else-if="group.type === 'video'" class="flow-video">
        <div class="flow-video-head">
          <Icon icon="mdi:video-outline" width="14" />
          <span>{{ tr('生成视频') }}</span>
          <span v-if="group.block.size || group.block.seconds" class="flow-video-meta">
            {{ group.block.size || '' }}<template v-if="group.block.size && group.block.seconds"> · </template>{{ group.block.seconds ? group.block.seconds + 's' : '' }}
          </span>
        </div>
        <video
          :src="group.block.url"
          controls
          playsinline
          preload="metadata"
          class="flow-video-player"
        ></video>
        <div v-if="group.block.caption" class="flow-video-caption">{{ group.block.caption }}</div>
                </div>

                <!-- 生成音乐：内嵌音频播放条（music_generate 工具产物，同视频块模式） -->
                <div v-else-if="group.type === 'audio'" class="flow-audio">
                  <div class="flow-audio-head">
                    <Icon icon="mdi:music-note-outline" width="14" />
                    <span>{{ tr('生成音乐') }}</span>
                    <span v-if="group.block.size" class="flow-video-meta">{{ group.block.size }}</span>
                  </div>
                  <audio :src="group.block.url" controls preload="metadata" class="flow-audio-player"></audio>
                  <div v-if="group.block.caption" class="flow-video-caption">{{ group.block.caption }}</div>
                </div>

        <!-- Agent 交付文件（md/pdf/pptx/docx/xlsx/html/txt 等）：
             交付卡片，点「预览」把内容送进右侧预览窗口；带内嵌「下载」按钮。 -->
        <div v-else-if="group.type === 'file'" class="flow-file-deliver">
          <div class="flow-file-head">
            <span
              class="flow-file-icon"
              :class="'ext-' + (group.block.ext ? group.block.ext.replace('.', '') : 'file')"
            >
              <Icon
                :icon="fileIcon(group.block.ext)"
                width="16"
              />
            </span>
            <div class="flow-file-meta">
              <span class="flow-file-name" :title="group.block.name">{{ group.block.name }}</span>
              <span class="flow-file-sub">
                <span>{{ tr('Agent 交付文件') }}</span>
                <template v-if="group.block.size"><span>·</span><span>{{ formatFileSize(group.block.size) }}</span></template>
              </span>
            </div>
            <div class="flow-file-actions">
              <button type="button" class="flow-file-btn" @click="previewDeliveredFile(group.block)">
                <Icon icon="mdi:eye-outline" width="13" /> {{ tr('预览') }}
              </button>
              <a
                class="flow-file-btn download"
                :href="backendURL(group.block.url || ('/api/agent/file?path=' + encodeURIComponent(group.block.path) + '&raw=1'))"
                :download="group.block.name"
              >
                <Icon icon="mdi:download-outline" width="13" /> {{ tr('下载') }}
              </a>
            </div>
          </div>
          <div v-if="group.block.previewError" class="flow-file-err">{{ group.block.previewError }}</div>
          </div>

          <!-- Agent 调 chart 工具产出的 ECharts 图表：前端 ChartRenderer 直出 -->
          <div v-else-if="group.type === 'chart'" class="flow-chart">
            <ChartRenderer
              :title="group.block.title"
              :type="group.block.chartType"
              :data="group.block.chartData"
              :options="group.block.chartOptions"
            />
          </div>

          <!-- 记忆写入：单行彩虹反馈（不占卡片，直接铺在聊天流里） -->
      <div v-else-if="group.type === 'memory-saved'" class="flow-memory-saved">
        <span class="fms-scanline"></span>
        <span class="fms-label">{{ tr('已保存到记忆') }}</span>
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
            <span class="flow-summary-text">
              <!-- 概要栏外显「当前正在执行的指令」，指令切换时淡入淡出；
                   全部命令/思考完成才收束成摘要（groupSummaryTitle 兜底态） -->
              <Transition name="cmd-fade" mode="out-in">
                <span :key="groupSummaryKey(group)">{{ groupSummaryTitle(group) }}</span>
              </Transition>
            </span>
          </div>
          <span class="flow-chevron" :class="{ open: isSummaryExpanded(gIdx) }">›</span>
        </div>

        <Transition name="flow-body">
          <div v-show="isSummaryExpanded(gIdx)" class="flow-body">
            <template v-for="(b, i) in group.blocks" :key="`${gIdx}-${i}`">
              <!-- 思考 -->
                            <div v-if="b.type === 'thinking'" class="flow-thinking flow-thinking-timeline">
                              <div class="flow-row-head" @click.stop="toggleThink(`${gIdx}-${i}`, b)">
                                                              <RoseParticleLoader v-if="!thinkDone(b)" :size="18" class="flow-row-icon icon-think" />
                              <Icon v-else icon="mdi:thought-bubble-outline" class="flow-row-icon icon-think" width="16" />
                                <span class="flow-thinking-text-label">{{ thinkLabelText(b) }}</span>
                                <span v-if="thinkCollapsed(`${gIdx}-${i}`, b) && b.text" class="flow-row-preview">{{ onelinePreview(b.text) }}</span>
                                <span v-else class="flow-spacer"></span>
                                <span class="flow-chevron" :class="{ open: thinkIsOpen(`${gIdx}-${i}`, b) }">›</span>
                              </div>
                              <div v-if="thinkIsOpen(`${gIdx}-${i}`, b)" class="flow-detail flow-thinking-detail">
                                <div class="flow-thinking-text">{{ b.text }}</div>
                              </div>
                            </div>

              <!-- 操作 -->
              <div v-else-if="b.type === 'tool'" class="flow-tool flow-tool-timeline">
                <div class="flow-row-head" @click.stop="b.expanded = !b.expanded">
                  <!-- 执行中：静态扳手换成玫瑰粒子动画（与思考行同源，小号+灰色），
                       完成/失败后落回静态图标，不再空转消耗 rAF -->
                  <RoseParticleLoader v-if="b.status === 'running'" :size="14" :particle-count="28" class="flow-row-icon icon-tool icon-tool-live" />
                  <Icon v-else icon="mynaui:tool" class="flow-row-icon icon-tool" width="13" />
                  <span class="flow-tool-label" :class="{ 'is-running': b.status === 'running' }">{{ actionText(b) }}</span>
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

              <!-- 中间意图（「我先看看这个文件」）：收进过程的短说明，一行预览 -->
              <div v-else-if="b.type === 'intent'" class="flow-intent-inline">
                <Icon icon="mdi:message-processing-outline" width="12" class="flow-intent-inline-icon" />
                <span class="flow-intent-inline-text">{{ onelinePreview(b.text) }}</span>
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
      <div class="flow-refs-title">{{ tr('引用来源') }}</div>
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

    <!-- 建议按钮已移至 ChatWidget（工具条下方，2026-09-12：与 workflow_done 解耦，
          done 后独立拉取，晚到不顶聊天内容）。面板内不再渲染。 -->

    <!-- 改动文件：工作流收尾固定内嵌在最底部（像引用来源一样平铺）。
             列出本次会话改过的文件，逐个预览 diff / 一键回退到工作流前版本。
             数据源 flow.changedFiles（后端 workflow_done 下发，各收尾分支都有）。 -->
    <div v-if="flow.changedFiles && flow.changedFiles.length" class="flow-changed-files">
      <div class="flow-refs-title">{{ tr('本次改动') }}{{ flow.changedFiles.length }}{{ tr('个文件') }}</div>
      <div class="flow-changed-list">
        <div v-for="f in flow.changedFiles" :key="f.rel_path" class="flow-changed-item">
          <div class="flow-changed-row">
            <Icon icon="mdi:file-code-outline" class="flow-changed-file-icon" width="13" />
            <span class="flow-changed-path" :title="f.rel_path">{{ f.rel_path }}</span>
            <span class="flow-changed-stats">
              <b class="flow-changed-add">+{{ f.added || 0 }}</b>
              <b class="flow-changed-del">−{{ f.removed || 0 }}</b>
            </span>
            <button type="button" class="flow-changed-btn audit" :disabled="f.audit && f.audit.loading" @click="auditChangedFile(f)">
              <Icon icon="mdi:shield-search" width="12" /> {{ (f.audit && f.audit.loading) ? tr('审查中…') : tr('审查') }}
            </button>
            <button type="button" class="flow-changed-btn" :disabled="changedRestoring" @click="previewChangedFile(f)">
              <Icon icon="mdi:eye-outline" width="12" /> {{ tr('预览') }}
            </button>
            <button type="button" class="flow-changed-btn danger" :disabled="changedRestoring" @click="restoreChangedFile(f)">
              <Icon icon="mdi:undo" width="12" /> {{ tr('回退') }}
            </button>
          </div>
          <!-- 该文件的审查结果：独立免费模型调用挑这一处 diff 的潜在问题 -->
          <div v-if="f.audit && f.audit.error" class="flow-audit-error">{{ f.audit.error }}</div>
          <div v-if="f.audit && f.audit.findings" class="flow-audit-card">
            <div class="flow-audit-title">
              <Icon icon="mdi:shield-search" width="13" />
              <span>{{ tr('独立审查') }}</span>
              <span v-if="f.audit.findings.length" class="flow-audit-count">{{ f.audit.findings.length }}{{ tr('个潜在问题') }}</span>
            </div>
            <div v-if="!f.audit.findings.length" class="flow-audit-empty">{{ tr('未发现潜在问题') }}</div>
            <div v-else class="flow-audit-list">
              <div v-for="(a, ai) in f.audit.findings" :key="ai" class="flow-audit-row">
                <span class="flow-audit-sev" :class="'sev-' + a.severity">{{ sevLabel(a.severity) }}</span>
                <div class="flow-audit-body">
                  <div class="flow-audit-issue">{{ a.issue }}</div>
                  <div v-if="a.hint" class="flow-audit-hint">{{ a.hint }}</div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
      <div v-if="changedMsg" class="flow-changed-msg" :class="{ error: changedMsgErr }">{{ changedMsg }}</div>
      <!-- 预览 diff（点「预览」后内嵌展开，像工具 diff 一样） -->
      <div v-if="changedDiffOpen" class="flow-changed-diff">
        <div v-if="changedDiffLoading" class="flow-changed-state">{{ tr('正在读取快照…') }}</div>
        <div v-else-if="changedDiffError" class="flow-changed-state error">{{ changedDiffError }}</div>
        <div v-else-if="changedDiffLines.length" class="flow-changed-code">
          <div v-for="(l, li) in changedDiffLines" :key="li" class="flow-changed-code-line" :class="l.kind">
                      <code>{{ l.text || ' ' }}</code>
          </div>
        </div>
        <div v-else class="flow-changed-state">{{ tr('该文件没有可显示的文本差异') }}</div>
      </div>
    </div>
  </div>
  <!-- 回退确认弹窗（轻量模态，替代原生 window.confirm） -->
  <Teleport to="body">
    <div v-if="confirmRestoreTarget" class="confirm-overlay" @click.self="confirmRestoreTarget = null">
      <div class="confirm-dialog">
        <p>{{ confirmRestoreMsg }}</p>
        <div class="confirm-actions">
          <button type="button" class="confirm-btn cancel" @click="confirmRestoreTarget = null">{{ tr('取消') }}</button>
          <button type="button" class="confirm-btn ok" @click="doRestore()">{{ tr('确认回退') }}</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { reactive, computed, ref, watch, nextTick, onUnmounted } from 'vue'
import { Icon } from '@iconify/vue'
import { diffLines } from 'diff'
import DiffViewer from './DiffViewer.vue'
import ArxivPaperCard from './ArxivPaperCard.vue'
import RoseParticleLoader from './RoseParticleLoader.vue'
import { renderMarkdown } from './markdownRenderer.js'
import { requestPreview } from '../composables/previewBus.js'
// 桌面版页面源是 https://wails.localhost，<a href> 原生加载不走 fetch 桥，
// 下载/预览链接必须绝对化到后端 http 源（详见 desktopTransport.js）。
import { backendURL } from '../../../desktopTransport.js'
import { pptxToHtml, xlsxToHtml, docxToHtml } from '../../../utils/officePreview.js'
import ChartRenderer from './ChartRenderer.vue'
import { useI18n, tr } from '../../../composables/useI18n.js'



const props = defineProps({
  flow: { type: Object, required: true }
})

// ==================== Agent 交付文件卡片 ====================
// md/html/txt 拉取内容在前端转 HTML 送右侧预览窗口；pdf/pptx/docx/xlsx 等二进制
// 走 /api/agent/file?path=..&raw=1 新开。预览只做一次，失败给可见错误，不循环弹。
const TEXT_PREVIEW_EXTS = new Set(['md', 'htm', 'html', 'txt'])
function fileIcon(ext) {
  switch ((ext || '').replace('.', '').toLowerCase()) {
    case 'pdf': return 'mdi:file-pdf-box'
    case 'pptx': return 'mdi:file-powerpoint-box'
    case 'docx': case 'doc': return 'mdi:file-word-box'
    case 'xlsx': case 'xls': case 'csv': return 'mdi:file-excel-box'
    case 'html': case 'htm': return 'mdi:language-html5'
    case 'txt': return 'mdi:file-document-outline'
    case 'md': return 'mdi:markdown'
    default: return 'mdi:file-outline'
  }
}
function formatFileSize(bytes) {
  const n = Number(bytes) || 0
  if (n < 1024) return n + ' B'
  if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KB'
  return (n / (1024 * 1024)).toFixed(1) + ' MB'
}
async function previewDeliveredFile(block) {
  if (block._previewing) return
  block._previewing = true
  block.previewError = ''
  const ext = (block.ext || '').replace('.', '').toLowerCase()
  try {
    if (block.url) {
      // 带静态直链的交付物（如 fetch_media 落到媒体目录的文档）：浏览器原生
      // 渲染 pdf/图片/文本，直接进右侧预览窗，绕开 /api/agent/file 路径沙箱。
      requestPreview(block.url)
    } else if (TEXT_PREVIEW_EXTS.has(ext)) {
      // 文本类（md/html/txt）：拉 content，前端渲染成独立 HTML blob 进右侧窗口。
      const res = await fetch('/api/agent/file?path=' + encodeURIComponent(block.path))
      if (!res.ok) throw new Error(tr('读取失败 (') + res.status + ')')
      const data = await res.json()
      const body = ext === 'txt' ? escapeHtml(data.content || '')
        : renderMarkdown(data.content || '')
      const html = '<!doctype html><html><head><meta charset="utf-8">' +
        '<style>body{font-family:system-ui,Segoe UI,Microsoft YaHei,sans-serif;padding:24px;line-height:1.7;color:#1e293b;max-width:880px;margin:0 auto;}' +
        'pre{background:#f8f7fc;padding:14px;border-radius:8px;overflow:auto}code{background:#f1f5f9;padding:1px 5px;border-radius:4px;font-size:.92em}' +
        'pre code{background:none;padding:0}table{border-collapse:collapse}th,td{border:1px solid #e2e8f0;padding:6px 10px}img{max-width:100%}' +
        '.katex-display{overflow-x:auto}svg{max-width:100%}</style></head><body>' + body + '</body></html>'
      requestPreview('data:text/html;charset=utf-8,' + encodeURIComponent(html))
    } else if (ext === 'pdf') {
      // PDF：预览窗内嵌 raw（浏览器原生渲染 PDF）
      requestPreview('/api/agent/file?path=' + encodeURIComponent(block.path) + '&raw=1')
    } else {
      // pptx/xlsx/docx：前端解析渲染成 HTML 进右侧预览窗（不再是源码/下载）
      const binary = await fetch('/api/agent/file?path=' + encodeURIComponent(block.path) + '&raw=1')
      if (!binary.ok) throw new Error(tr('读取失败 (') + binary.status + ')')
      const buf = await binary.arrayBuffer()
      if (ext === 'pptx') requestPreview(await pptxToHtml(buf))
      else if (ext === 'xlsx') requestPreview(await xlsxToHtml(buf))
      else if (ext === 'docx' || ext === 'doc') requestPreview(await docxToHtml(buf))
      else window.open('/api/agent/file?path=' + encodeURIComponent(block.path) + '&raw=1', '_blank')
    }
  } catch (err) {
    block.previewError = err.message || tr('预览失败')
  } finally {
    block._previewing = false
  }
}
function escapeHtml(s) {
  return String(s).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]))
}

// ★ 思考区域「瀑布滚动」：流式追加思考 text 时，把当前正在书写的思考窗口滚到底，
// 否则新增思考一直被钉在顶部折叠线后，看起来"置顶不动"。
watch(
  () => props.flow?.blocks,
  () => {
    nextTick(() => {
      const list = document.querySelectorAll('.agent-flow.streaming .flow-thinking-detail')
      if (list.length) list[list.length - 1].scrollTop = list[list.length - 1].scrollHeight
    })
  },
  { deep: true }
)


// ==================== 改动文件卡片（内嵌工作流底部） ====================
const changedDiffOpen = ref(false)
const changedDiffLoading = ref(false)
const changedDiffError = ref('')
const changedDiffRaw = ref('')
const changedMsg = ref('')
const changedMsgErr = ref(false)
const changedRestoring = ref(false)
// 回退确认弹窗（轻量模态，替代原生 window.confirm）
const confirmRestoreTarget = ref(null)
const confirmRestoreMsg = ref('')

// 当前项目名：从 ChatWidget 的工作目录 localStorage 取（与 /api/agentfs/* 的 project 口径一致）
function currentProjectName() {
  try {
    const raw = localStorage.getItem('aether_workdir_state_v1')
    if (raw) {
      const d = JSON.parse(raw)
      if (d?.current?.name) return d.current.name
    }
  } catch {}
  return ''
}

// 审查按钮（每个改动文件一个）：拉该文件本次 diff，交给后端独立免费模型审查，
// 挑这一处改动的潜在问题。结果挂在 f.audit 上内联渲染。
async function auditChangedFile(f) {
  if (!f || !f.first_seq) return
  if (f.audit && f.audit.loading) return
  if (!f.audit) f.audit = { loading: false, findings: null, error: '' }
  f.audit.loading = true
  f.audit.error = ''
  try {
    const dres = await fetch('/api/agentfs/diff', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ project: '', seq: f.first_seq })
    })
    const ddata = await dres.json()
    if (!dres.ok) throw new Error(ddata.error || tr('Diff 读取失败'))
    const ares = await fetch('/api/code/workflow/audit', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: f.rel_path, diff: ddata.diff || '' })
    })
    const adata = await ares.json().catch(() => ({}))
    if (!ares.ok) throw new Error(adata.error || (tr('审查失败 (') + ares.status + ')'))
    f.audit.findings = Array.isArray(adata.findings) ? adata.findings : []
  } catch (err) {
    f.audit.error = err.message || tr('审查请求失败，请重试')
  } finally {
    f.audit.loading = false
  }
}

function sevLabel(s) {
  return s === 'high' ? tr('高') : s === 'low' ? tr('低') : tr('中')
}

// diff 原始文本 → 行数组（保留行首 +/− 符号作为增删标记，不做红绿双重显示）
const changedDiffLines = computed(() => {
  const rows = []
  for (const line of changedDiffRaw.value.split('\n')) {
    if (!line) continue
    // 跳过 diff 头行（@@ 位置、--- +++ 文件名），它们不是内容
    if (line.startsWith('@@') || line.startsWith('--- ') || line.startsWith('+++ ')) continue
    if (line.startsWith('-') && !line.startsWith('--')) rows.push({ kind: 'del', text: line })
    else if (line.startsWith('+') && !line.startsWith('++')) rows.push({ kind: 'add', text: line })
    else rows.push({ kind: '', text: line })
  }
  return rows
})

async function previewChangedFile(f) {
  if (!f || !f.first_seq) return
  // 已展开 → 再次点击收回（2026-08-28 用户反馈：预览展开后点不收回）
  if (changedDiffOpen.value) {
    changedDiffOpen.value = false
    changedDiffRaw.value = ''
    return
  }
  changedDiffOpen.value = true
  changedDiffLoading.value = true
  changedDiffError.value = ''
  changedDiffRaw.value = ''
  try {
    const res = await fetch('/api/agentfs/diff', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      // project 留空：后端自动用当前 AgentFS 会话的项目，不依赖前端猜工作目录名
      body: JSON.stringify({ project: '', seq: f.first_seq })
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || (tr('Diff 读取失败 (') + res.status + ')'))
    changedDiffRaw.value = data.diff || ''
  } catch (err) {
    changedDiffError.value = err.message || tr('无法读取该快照')
  } finally {
    changedDiffLoading.value = false
  }
}

async function restoreChangedFile(f) {
  if (!f || !f.first_seq) return
  // 轻量确认弹窗，替代原生 window.confirm（2026-08-28 用户反馈）
  confirmRestoreTarget.value = f
  confirmRestoreMsg.value = f.exists_before === false
    ? (tr('删除 ') + f.rel_path + tr('？（本次工作流新建的文件，回退=删除）'))
    : (tr('回退 ') + f.rel_path + tr(' 到本次工作流开始前的版本？'))
}

async function doRestore() {
  const f = confirmRestoreTarget.value
  confirmRestoreTarget.value = null
  if (!f || !f.first_seq) return
  changedRestoring.value = true
  changedMsg.value = ''
  changedMsgErr.value = false
  try {
    const res = await fetch('/api/agentfs/restore', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      // project 留空：后端自动用当前 AgentFS 会话的项目
      body: JSON.stringify({ project: '', seq: f.first_seq, before: true })
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || (tr('回退失败 (') + res.status + ')'))
    changedMsg.value = data.deleted ? (tr('已删除 ') + data.restored) : (tr('已回退 ') + data.restored)
    // 回退后从列表移除；全部回退完收起到卡片标题状态
    props.flow.changedFiles = (props.flow.changedFiles || []).filter(x => x.rel_path !== f.rel_path)
    changedDiffOpen.value = false
  } catch (err) {
    changedMsg.value = err.message || tr('回退失败')
    changedMsgErr.value = true
  } finally {
    changedRestoring.value = false
  }
}

// ★ 工具调用收进概要组；回复(intent)单独平铺，不收纳
// thinking（思考）与 web_search（联网搜索）也不收束——思考是推理轨迹要
// 全程可见，搜索卡片自带引用来源，收进概要卡片会被折叠看不见。

// intent 的 markdown 节流只做「省」，必须配一次「补」：被节流吃掉的那一帧
// 之后如果模型正好吐完，就再没有 onStreamUpdate 触发重算，屏幕会停在半句话上，
// 直到 workflow_done 把 status 翻成非 running 才「恢复」——这就是收尾必现的
// 「最后几个词生成一半卡住、恢复才显示」。下面的尾随定时器补上那一帧。
const intentFlushTick = ref(0)
const intentFlushTimers = new Map() // block 对象 -> timer；同一 block 只排一个
function scheduleIntentFlush(b, delay) {
  if (intentFlushTimers.has(b)) return
  const t = setTimeout(() => {
    intentFlushTimers.delete(b)
    b._cachedAt = 0         // 清掉节流时间戳，下一次计算必定重算 html
    intentFlushTick.value++ // 触发 blockGroups 重算 → 渲染最新全文
  }, Math.max(delay, 16))
  intentFlushTimers.set(b, t)
}

const blockGroups = computed(() => {
  // 依赖：尾随补刷定时器到点后强制重算本 computed
  void intentFlushTick.value
  const groups = []
  const blocks = props.flow?.blocks || []
  const streaming = props.flow?.status === 'running' || props.flow?.status === 'waiting'

  // 流式中：全平铺，思考实时可见（用户爱看思考，禁提前收束）。
  if (streaming) {
    let current = null
    for (const b of blocks) {
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
            const now = Date.now()
            const throttled = b._cachedAt && (now - b._cachedAt < 100)
            if (throttled) {
              scheduleIntentFlush(b, 100 - (now - b._cachedAt))
            } else if (b.text !== b._cachedText) {
              b._cachedHtml = renderMarkdown(b.text, true)
              b._cachedText = b.text
              b._cachedAt = now
            }
            groups.push({ type: 'visible', text: b.text, html: b._cachedHtml, retryNote: !!b.retryNote })
        } else if (b.type === 'thinking') {
          groups.push({ type: 'single-thinking', block: b })
        } else if (b.type === 'question') {
          groups.push({ type: 'question', block: b })
        } else if (b.type === 'image') {
          groups.push({ type: 'image', block: b })
        } else if (b.type === 'video') {
          groups.push({ type: 'video', block: b })
        } else if (b.type === 'file') {
          groups.push({ type: 'file', block: b })
        } else if (b.type === 'chart') {
          groups.push({ type: 'chart', block: b })
        } else if (b.type === 'tool' && b.name === 'web_search') {
          groups.push({ type: 'search-tool', block: b })
        } else if (b.type === 'steer') {
          groups.push({ type: 'steer', block: b })
        } else if (b.type === 'memory-saved') {
          groups.push({ type: 'memory-saved', block: b })
        } else if (b.type === 'preview') {
          groups.push({ type: 'preview', block: b })
        }
      }
    }
    if (current) groups.push(current)
    return groups
  }

  // 收尾后：「过程 → 总结」收束。LLM 习惯 大量思考→工具调用→最后总结；
  // 最后一个 intent（总结）之前的所有 thinking/tool/短意图都是过程，
  // 收进一个 process 概要组（一行，可展开看时间线）；最终总结平铺。
  let lastIntentIdx = -1
  for (let i = 0; i < blocks.length; i++) {
    if (blocks[i].type === 'intent') lastIntentIdx = i
  }
  let processBlocks = [] // 收束中的过程块（thinking/tool/中间 intent）
    const flushProcess = () => {
      if (processBlocks.length) {
        // 单个思考不收束：只有一条 thinking、没有工具没有中间意图时，直接平铺
        // （单步思考本身就是完整推理轨迹，收进概要组反而把推理藏起来了）
        if (processBlocks.length === 1 && processBlocks[0].type === 'thinking') {
          groups.push({ type: 'single-thinking', block: processBlocks[0] })
        } else {
          groups.push({ type: 'process', blocks: processBlocks })
        }
        processBlocks = []
      }
    }
  for (let i = 0; i < blocks.length; i++) {
    const b = blocks[i]
    if (b.type === 'tool' && b.name !== 'web_search') {
      processBlocks.push(b)
    } else {
      if (b.type === 'intent') {
        if (i === lastIntentIdx) {
          flushProcess()
          const now = Date.now()
          const throttled = b._cachedAt && (now - b._cachedAt < 100)
          if (throttled) {
            scheduleIntentFlush(b, 100 - (now - b._cachedAt))
          } else if (b.text !== b._cachedText) {
            b._cachedHtml = renderMarkdown(b.text, true)
            b._cachedText = b.text
            b._cachedAt = now
          }
          groups.push({ type: 'visible', text: b.text, html: b._cachedHtml, retryNote: !!b.retryNote })
        } else {
          processBlocks.push(b)
        }
      } else if (b.type === 'thinking') {
        processBlocks.push(b)
      } else if (b.type === 'question') {
        flushProcess()
        groups.push({ type: 'question', block: b })
      } else if (b.type === 'image') {
        flushProcess()
        groups.push({ type: 'image', block: b })
      } else if (b.type === 'video') {
        flushProcess()
        groups.push({ type: 'video', block: b })
      } else if (b.type === 'file') {
        flushProcess()
        groups.push({ type: 'file', block: b })
      } else if (b.type === 'chart') {
        flushProcess()
        groups.push({ type: 'chart', block: b })
      } else if (b.type === 'tool' && b.name === 'web_search') {
        flushProcess()
        groups.push({ type: 'search-tool', block: b })
      } else if (b.type === 'steer') {
        flushProcess()
        groups.push({ type: 'steer', block: b })
      } else if (b.type === 'memory-saved') {
        flushProcess()
        groups.push({ type: 'memory-saved', block: b })
      } else if (b.type === 'preview') {
        flushProcess()
        groups.push({ type: 'preview', block: b })
      }
    }
  }
  flushProcess()
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

// 概要栏外显「当前指令」：取组内最后一个 running/generating 的块（不是简单取 last——
// 命令流式生成时是 generating 状态，旧实现只认 running 会提前收束成「运行了多个命令」）。
function currentActiveBlock(group) {
  const blocks = group.blocks || []
  for (let i = blocks.length - 1; i >= 0; i--) {
    const s = blocks[i].status
    if (s === 'running' || s === 'generating') return blocks[i]
  }
  return null
}

function groupSummaryTitle(group) {
  const cur = currentActiveBlock(group)
  if (cur) {
    if (cur.type === 'tool') return actionText(cur)
    if (cur.type === 'thinking') return tr('正在思考…')
    return tr('Agent 正在处理…')
  }
  // 全部命令/思考都完成了，才收束成摘要
    return tr('运行了多个命令')
  }

// 指令切换淡入淡出的 key：流式生成（generating）中 key 固定防闪；
// 命令确定/执行中/下一条指令切换时以「块 id + 内容」为 key → out-in 淡出旧指令淡入新指令。
function groupSummaryKey(group) {
  const cur = currentActiveBlock(group)
  if (!cur) return 'done'
  if (cur.type === 'thinking') return `think:${cur.id || cur.startTime || 0}`
  const cmd = String((cur.args && cur.args.command) || '').trim()
  if (cur.status === 'generating') return `tool-gen:${cur.id || cur.startTime || 0}`
  return `tool:${cur.id || cur.startTime || 0}:${cmd}`
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
// 状态徽章文案：完成带耗时、进行中实时跳动、失败。
// 长命令（配音/合成/下载）干等最像卡死：running 时徽章显示实时秒数，
// 超过 20s 切「仍在执行」文案，明确告诉用户没卡、还在跑。
function toolBadge(b) {
  if (b.status === 'generating') {
    const chars = Number(b.generatedChars || 0)
    return chars >= 1000 ? (tr('生成参数 ') + (chars / 1000).toFixed(1) + 'k') : tr('生成参数')
  }
  if (b.status === 'running') {
    const live = fmtMs(nowTick.value ? (nowTick.value - (b.startTime || nowTick.value)) : 0)
    if (!live) return tr('进行中')
    const secs = (nowTick.value - b.startTime) / 1000
    return secs >= 20 ? (tr('仍在执行 ') + live) : live
  }
  if (b.status === 'error') return tr('失败')
  const t = fmtMs(b.elapsedMs)
  return t ? tr('完成 ') + t : tr('完成')
}

// ==================== 思考块折叠 ====================
// 思考块「思考中展开、结束后自动收起」：同一个块只有在流式书写途中才视为进行中
// （flow.status === 'running' 且它就是 blocks 最后一个）；一旦被后面的块顶掉
// 或整条流收尾，自动收起成一行（带预览）。手工点过以手工状态为准，不被自动收起覆盖。
function thinkDone(b) {
  if (!props.flow || !b) return true
  if (b.elapsedMs) return true
  if (props.flow.status !== 'running') return true
  const blocks = props.flow.blocks || []
  return blocks[blocks.length - 1] !== b
}
// 思考头文案：思考中 → 「正在思考 12.3s」（250ms 实时跳动）；已结束 → 「思考 3.2s」
const nowTick = ref(0)
let thinkTimer = null
// 有思考块在流式书写时，用轻量定时器驱动实时计时显示。
// 同时在这里给「被新块顶掉但没记耗时」的思考块补定格 elapsedMs：只有 appendText
// 自己会记（intent/正文增量走它），而 tool 块是各事件处理器直接 flow.blocks.push
// 的，绕过 appendText → 前面的思考块永远没有 elapsedMs，thinkDone 却已判结束，
// 标签就退化成光秃秃的「思考」（时间没了，2026-09-10 用户反馈）。
// 定格取下一个块的 startTime（≈思考结束那一刻），没有就用当前时间。
watch(
  () => props.flow?.blocks?.length,
  () => {
    const blocks = props.flow?.blocks || []
    for (let i = 0; i < blocks.length - 1; i++) {
      const b = blocks[i]
      if (b.type === 'thinking' && b.startTime && !b.elapsedMs) {
        const next = blocks[i + 1]
        b.elapsedMs = Math.max(0, (next.startTime || Date.now()) - b.startTime)
      }
    }
    // 计时器驱动两类实时跳动：流式书写中的思考块 + 执行中的工具块（长命令秒数）
    const hasLive = blocks.some((b) =>
      (b.type === 'thinking' && !thinkDone(b)) ||
      (b.type === 'tool' && b.status === 'running'))
    if (hasLive && !thinkTimer) {
      thinkTimer = setInterval(() => { nowTick.value = Date.now() }, 250)
    } else if (!hasLive && thinkTimer) {
      clearInterval(thinkTimer)
      thinkTimer = null
    }
  },
  { immediate: true }
)
onUnmounted(() => {
  if (thinkTimer) clearInterval(thinkTimer)
  for (const t of intentFlushTimers.values()) clearTimeout(t)
  intentFlushTimers.clear()
})

function thinkLabelText(b) {
  if (!thinkDone(b)) {
    const label = props.flow?.status === 'waiting' ? tr('等待后台任务') : tr('正在思考')
    if (b.startTime) {
      const live = fmtMs((nowTick.value || Date.now()) - b.startTime)
      if (live) return `${label} ${live}`
    }
    return label
  }
  const t = fmtMs(b.elapsedMs)
  return t ? (tr('思考 ') + t) : tr('思考')
}
// 默认展开 = 仍在思考中；手工点过则按手工状态
function thinkIsOpen(key, b) {
  return thinkOpen[key] ?? !thinkDone(b)
}
function thinkCollapsed(key, b) {
  return !thinkIsOpen(key, b)
}
const thinkOpen = reactive({})
function toggleThink(key, b) {
  thinkOpen[key] = !thinkIsOpen(key, b)
}
// 整条流收尾（含 stop/失败）：最后的思考块可能来不及被下一个块顶掉，这里补记耗时
watch(
  () => props.flow?.status,
  (st) => {
    if (st === 'running' || st === 'waiting') return
    const blocks = props.flow?.blocks || []
    const last = blocks[blocks.length - 1]
    if (last && last.type === 'thinking' && last.startTime && !last.elapsedMs) {
      last.elapsedMs = Date.now() - last.startTime
    }
    if (thinkTimer) { clearInterval(thinkTimer); thinkTimer = null }
  }
)

// ==================== 动作行文案 ====================
// 一行白话，动词 + 对象，读起来跟正文一样（"编辑了 tools.go"），不靠图标传达语义。
// 运行中把"了"换成"正在…"，这样连状态图标也省了。
const VERBS = {
  read_file: tr('读取'),
  grep: tr('搜索'),
  glob: tr('查找文件'),
  list_directory: tr('列目录'),
  directory_tree: tr('目录树'),
  get_file_info: tr('文件信息'),
  mcp__fs__read_file: tr('读取'),
  mcp__fs__read_text_file: tr('读取'),
  mcp__grep__read_range: tr('读取'),
  write_file: tr('写入'),
  mcp__fs__write_file: tr('写入'),
  mcp__fs__create_file: tr('新建'),
  edit_file: tr('编辑'),
  create_directory: tr('新建目录'),
  move_file: tr('移动'),
  delete_file: tr('删除'),
  delete_directory: tr('删除目录'),
  run_command: tr('执行命令'),
  web_fetch: tr('抓取网页'),
  view_image: tr('查看图片'),
  memory_search: tr('搜索记忆'),
  memory_append: tr('写入记忆'),
  mcp__fs__edit_file: tr('编辑'),
  inject_preview_js: tr('注入'),
  execute_command: tr('运行'),
  search_codebase: tr('搜索代码库'),
  codegraph_query: tr('分析调用链'),
  search_memory: tr('检索记忆'),
  dispatch_agent: tr('派发子代理'),
  web_search: tr('联网搜索'),
  mcp__web_search__web_search: tr('联网搜索'),
  mcp__web_fetch__web_fetch: tr('抓取网页')
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
  if (b.name === 'load_tools') return b.status === 'running' ? tr('加载 MCP 工具中…') : tr('加载了 MCP 工具')
  // 联网搜索（Firecrawl 工具）：显示「搜索到 N 个来源」而不是把一堆
  // 搜索词原样摊开——图2 那种「搜索到 35 个网页」摘要形态。
  if (isWebSearch(b.name)) {
    const n = searchSources(b).length
    const label = n > 0 ? (tr('搜索到 ') + n + tr(' 个来源')) : (b.status === 'running' ? tr('联网搜索中…') : tr('联网搜索'))
    const q = (b.args && b.args.query) || ''
    return q ? `${label} · ${String(q).slice(0, 30)}${String(q).length > 30 ? '…' : ''}` : label
  }
  const verb = VERBS[b.name] || (b.name.startsWith('mcp__') ? b.name.split('__').slice(1).join(' · ') : b.name)
  const obj = target(b)
  const running = b.status === 'running' || b.status === 'generating'
  // 读文件时把 head/tail/行范围（偏移和限制）显式带出来，否则用户以为每次都读全文
  const range = isRead(b.name) ? readRangeLabel(b) : ''
  const suffix = range ? `（${range}）` : ''
  if (!obj) return running ? (tr('正在') + verb) : (verb + tr('了'))
  return running ? (tr('正在') + verb + ' ' + obj + suffix) : (verb + tr('了 ') + obj + suffix)
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
  'thepaper.cn': tr('澎湃新闻'), 'm.jiemian.com': tr('界面新闻'), 'jiemian.com': tr('界面新闻'),
  'yicai.com': tr('第一财经'), '36kr.com': tr('36氪'), 'qq.com': tr('腾讯新闻'),
  '163.com': tr('网易新闻'), 'ifeng.com': tr('凤凰网'), 'news.cn': tr('新华网'),
  'people.com.cn': tr('人民网'), 'cctv.com': tr('央视网'), 'cntv.cn': tr('央视网'),
  'huanqiu.com': tr('环球网'), 'chinanews.com.cn': tr('中新网'), 'xinhuanet.com': tr('新华网'),
  'hgdaily.com.cn': tr('黄冈日报'), 'cngold.org': tr('金投网'), 'sina.com.cn': tr('新浪新闻'),
  'zhihu.com': tr('知乎'), 'bilibili.com': tr('哔哩哔哩'), 'sohu.com': tr('搜狐新闻'),
  'thecover.cn': tr('封面新闻'), 'stdaily.com': tr('科技日报'), 'ce.cn': tr('中国经济网'),
  'gov.cn': tr('中国政府网'), 'cnr.cn': tr('央广网'), '12371.cn': tr('共产党员网'),
  'gmw.cn': tr('光明网'), 'china.com.cn': tr('中国网'), 'cnstock.com': tr('上海证券报'),
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
  return c ? c.toUpperCase() : tr('网')
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
  return (b.status === 'running' || b.status === 'generating') ? tr('正在联网搜索…') : tr('联网搜索完成')
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
  // answered=false（超时/断线走 fallback）不是用户的选择：不高亮任何选项
  if (!block.answered) return false
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
  const out = b.output || (b.status === 'running' ? tr('执行中…') : tr('(无输出)'))
  if (b.name === 'execute_command') return `$ ${b.args.command || ''}\n\n${out}`
  if (b.name === 'dispatch_agent') return (tr('任务：') + b.args.task || '' + '\n\n' + out)
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
  /* <code> 有 UA 默认 font-family:monospace（Windows=新宋体），父级 .flow-live-diff
     的 font 简写不覆盖子元素 UA 样式，必须在此显式指定；中文走雅黑。 */
  font-family: Consolas, ui-monospace, 'Cascadia Mono', 'PingFang SC', 'Microsoft YaHei', monospace;
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
.flow-thinking-detail {
  /* 思考窗口：高度随内容自适应（少内容不白占留白），超出 max-height 才滚动；滚动条隐藏，仍可滚轮/触控滚动 */
  height: auto;
  max-height: 240px;
  overflow-y: auto;
  padding-right: 10px;
  border-right: 2px solid var(--app-border);
  scrollbar-width: none;
}
.flow-thinking-detail::-webkit-scrollbar { width: 0; height: 0; display: none; }
.flow-thinking-detail::-webkit-scrollbar-track { background: transparent; }
.flow-thinking-detail::-webkit-scrollbar-thumb { background: transparent; }
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
.flow-pending-loader {
  color: var(--app-accent);
  line-height: 0;
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
.icon-think { color: var(--app-accent); }
.icon-search { color: #8b5cf6; }
.icon-tool { color: var(--app-text-soft); }
/* 执行中的玫瑰粒子：跟静态扳手同色，不抢思考行的品牌色 */
.icon-tool-live { color: var(--app-text-soft); }
/* 执行中的动作文字：与思考标签同款的微光扫过，暗示"还在动" */
.flow-tool-label.is-running {
  animation: reasonShimmer 3s linear infinite;
  background: linear-gradient(100deg, var(--app-text-soft) 40%, var(--app-text) 50%, var(--app-text-soft) 60%);
  background-size: 250% 100%;
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}

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
  font-family: var(--app-font, -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif);
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
  /* 多 agent 群聊：正文沿用本角色名牌色（--agent-flow-color 由外层 agentflow-wrap 注入）；
     未注入（单 agent 通用对话）落回默认文字色，不影响既有样式 */
  color: var(--agent-flow-color, var(--app-text));
  word-break: break-word;
}

/* 免费模型重试提示（「重新尝试连接神经网络 (n/3)」）：小字 + 彩虹渐变文字 + 扫描线动画 */
.flow-retry-note {
  font-size: 12px;
  line-height: 1.6;
  margin: 3px 0;
  background: linear-gradient(90deg, #ff6b6b, #ffa94d, #ffe066, #69db7c, #4dabf7, #9775fa, #f783ac);
  background-size: 200% auto;
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
  animation: flow-retry-scan 1.8s linear infinite;
  position: relative;
  display: inline-block;
  opacity: .85;
}
@keyframes flow-retry-scan {
  0% { background-position: 0% center; }
  100% { background-position: 200% center; }
}

/* 生成视频内嵌块：带进度条播放器，同图片内嵌块模式 */
.flow-video {
  width: min(100%, 460px);
  margin: 10px 0;
  padding: 0;
  overflow: hidden;
  border: 1px solid color-mix(in srgb, var(--app-accent) 16%, var(--app-border));
  border-radius: 10px;
  background: var(--app-surface-2);
}
.flow-video-head {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 10px;
  font-size: 11.5px;
  font-weight: 600;
  color: var(--app-text-soft);
}
.flow-video-head svg { color: var(--app-accent); }
.flow-video-meta { margin-left: auto; font-size: 10.5px; font-weight: 400; color: var(--app-text-faint); }
.flow-video-player {
  display: block;
  width: 100%;
  max-height: 420px;
  background: #000;
}
.flow-video-caption { padding: 6px 10px 8px; font-size: 11px; color: var(--app-text-soft); }
/* 生成音乐内嵌播放条 */
.flow-audio { border-top: 1px solid var(--app-border); }
.flow-audio-head {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 10px;
  font-size: 11.5px;
  font-weight: 600;
  color: var(--app-text-soft);
}
.flow-audio-head svg { color: var(--app-accent); }
.flow-audio-player {
  display: block;
  width: 100%;
  padding: 0 10px 10px;
  box-sizing: border-box;
}

/* Agent 交付文件卡片 */
.flow-file-deliver {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 8px 0;
  padding: 10px 12px;
  border: 1px solid color-mix(in srgb, var(--app-accent) 18%, var(--app-border));
  border-radius: 10px;
  background: var(--app-surface-2);
}
.flow-file-head {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}
.flow-file-icon {
  width: 34px;
  height: 34px;
  border-radius: 8px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--app-surface-3);
  color: var(--app-accent);
}
.flow-file-meta {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
}
.flow-file-name {
  font-size: 12.5px;
  font-weight: 600;
  color: var(--app-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.flow-file-sub {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 10.5px;
  color: var(--app-text-faint);
}
.flow-file-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}
.flow-file-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 5px 9px;
  border: 1px solid var(--app-border);
  border-radius: 7px;
  background: var(--app-surface-3);
  color: var(--app-text);
  font-size: 11px;
  cursor: pointer;
  text-decoration: none;
}
.flow-file-btn:hover { border-color: var(--app-accent); color: var(--app-accent); }
.flow-file-btn.download:hover { border-color: var(--app-accent); color: var(--app-accent); }
.flow-file-err { font-size: 11px; color: #d94834; }

/* 截图：默认直接完整展示（不折叠、不点击展开） */
.flow-screenshot {
  display: block;
  width: min(100%, 680px);
  margin: 10px 0;
  padding: 0;
  overflow: hidden;
  border: 1px solid color-mix(in srgb, var(--app-accent) 16%, var(--app-border));
  border-radius: 10px;
  background: var(--app-surface-2);
  color: var(--app-text-soft);
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
  height: auto;
  max-height: 520px;
  object-fit: contain;
  background: var(--app-surface-3);
}

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
/* 概要栏当前指令切换淡入淡出 */
.cmd-fade-enter-active, .cmd-fade-leave-active { transition: opacity 0.18s ease; }
.cmd-fade-enter-from, .cmd-fade-leave-to { opacity: 0; }
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

/* ============ 独立审查（改动文件行内按钮 + 结果卡片） ============ */
/* 收进过程组的中间意图：一行弱化预览，不抢工具/思考时间线 */
.flow-intent-inline {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 3px 0 3px 24px;
  padding: 2px 0 2px 16px;
  border-left: 2px solid var(--app-border);
  font-size: 12.5px;
  color: var(--app-text-soft);
  min-width: 0;
}
.flow-intent-inline-icon {
  flex-shrink: 0;
  color: var(--app-text-faint);
}
.flow-intent-inline-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.flow-changed-item {
  display: flex;
  flex-direction: column;
}
.flow-audit-error {
  margin-top: 6px;
  font-size: 12.5px;
  color: #c0392b;
}
.flow-audit-card {
  margin-top: 6px;
  padding: 10px 12px;
  border-radius: 10px;
  border: 1px solid var(--app-border);
  background: var(--app-surface-2);
}
.flow-audit-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  font-weight: 600;
  color: var(--app-text);
}
.flow-audit-count {
  font-size: 12px;
  font-weight: 400;
  color: var(--app-text-faint);
}
.flow-audit-empty {
  margin-top: 6px;
  font-size: 12.5px;
  color: var(--app-text-faint);
}
.flow-audit-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 8px;
}
.flow-audit-row {
  display: flex;
  align-items: flex-start;
  gap: 8px;
}
.flow-audit-sev {
  flex-shrink: 0;
  margin-top: 1px;
  padding: 1px 7px;
  font-size: 11.5px;
  border-radius: 6px;
  color: #fff;
}
.flow-audit-sev.sev-high { background: #d64545; }
.flow-audit-sev.sev-medium { background: #e08a2e; }
.flow-audit-sev.sev-low { background: #8a94a6; }
.flow-audit-body { flex: 1; min-width: 0; }
.flow-audit-issue {
  font-size: 13px;
  color: var(--app-text);
  line-height: 1.5;
  word-break: break-word;
}
.flow-audit-hint {
  margin-top: 2px;
  font-size: 12px;
  color: var(--app-text-faint);
  line-height: 1.4;
  word-break: break-word;
}

/* ============ 改动文件卡片（内嵌工作流底部） ============ */
.flow-changed-files {
  margin-top: 10px;
  padding-top: 8px;
  border-top: 1px dashed var(--app-border);
}
.flow-changed-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-top: 6px;
}
.flow-changed-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 5px 8px;
  border-radius: 8px;
  background: var(--app-surface-2);
}
.flow-changed-file-icon {
  flex-shrink: 0;
  color: var(--app-text-faint);
}
.flow-changed-path {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: var(--app-font, -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif);
  font-size: 12px;
  color: var(--app-text);
}
.flow-changed-ops {
  flex-shrink: 0;
  font-size: 10.5px;
  color: var(--app-text-faint);
}
.flow-changed-stats {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-family: var(--app-font, -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif);
  font-size: 10.5px;
}
.flow-changed-add {
  color: #2ea043;
  font-weight: 700;
}
.flow-changed-del {
  color: #e24546;
  font-weight: 700;
}
.flow-changed-btn {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: 3px;
  padding: 2px 8px;
  border: 1px solid var(--app-border);
  border-radius: 6px;
  background: transparent;
  color: var(--app-text-soft);
  font-size: 10.5px;
  cursor: pointer;
}
.flow-changed-btn:hover {
  color: var(--app-text);
  border-color: var(--app-accent);
}
.flow-changed-btn.danger:hover {
  color: #e24546;
  border-color: #e24546;
}
.flow-changed-btn.audit:hover {
  color: var(--app-accent);
  border-color: var(--app-accent);
}
.flow-changed-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}
.flow-changed-msg {
  margin-top: 6px;
  font-size: 11.5px;
  color: var(--app-accent);
}
.flow-changed-msg.error {
  color: #e24546;
}
.flow-changed-diff {
  margin-top: 8px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  overflow: hidden;
  max-height: 280px;
  overflow-y: auto;
  background: var(--app-surface-2);
}
.flow-changed-state {
  padding: 10px 12px;
  font-size: 11.5px;
  color: var(--app-text-faint);
}
.flow-changed-state.error {
  color: #e24546;
}
.flow-changed-code-line {
  display: flex;
  gap: 8px;
  padding: 1px 10px;
  /* 这是 <code> 的父级，但 <code> 有 UA 默认 font-family:monospace（Windows=新宋体），
     必须在这里给子级 code 显式覆盖；同时中文走雅黑避免宋体混排。 */
  font-size: 11px;
  white-space: pre-wrap;
  word-break: break-all;
  color: var(--app-text);
}
.flow-changed-code-line code {
  font-family: Consolas, ui-monospace, 'Cascadia Mono', 'PingFang SC', 'Microsoft YaHei', monospace;
}
.flow-changed-code-line.add {
  background: color-mix(in srgb, #2ea043 10%, transparent);
  color: #1f7a3d;
}
.flow-changed-code-line.del {
  background: color-mix(in srgb, #e24546 10%, transparent);
  color: #c02a2b;
}
.flow-changed-line-no {
  flex-shrink: 0;
  width: 14px;
  color: var(--app-text-faint);
  text-align: right;
  user-select: none;
}

/* 回退确认弹窗：Teleport 到 body 后没有 ChatWidget 的 scope id，
   全局 chat-window.css 的 .confirm-overlay 是 scoped 的匹配不上，
   所以这里必须自带 scoped 样式（2026-08-28 实测弹窗裸奔 position:static）。 */
.confirm-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.35);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 2147482000;
}
.confirm-dialog {
  background: var(--app-surface, #fff);
  border: 1px solid var(--app-border, #e4e4e7);
  border-radius: 14px;
  padding: 22px 24px;
  max-width: 320px;
  box-shadow: 0 10px 36px rgba(0, 0, 0, 0.18);
  text-align: center;
}
.confirm-dialog p {
  color: var(--app-text, #27272a);
  margin: 0 0 18px;
  font-size: 13.5px;
  line-height: 1.6;
}
.confirm-actions {
  display: flex;
  gap: 10px;
  justify-content: center;
}
.confirm-btn {
  border: none;
  padding: 8px 20px;
  border-radius: 8px;
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s;
}
.confirm-btn.cancel {
  background: var(--app-surface-3, #f1f1f3);
  color: var(--app-text, #27272a);
}
.confirm-btn.cancel:hover {
  opacity: 0.8;
}
.confirm-btn.ok {
  background: #2ea043;
  color: #fff;
}
.confirm-btn.ok:hover {
  background: #238636;
}
</style>
