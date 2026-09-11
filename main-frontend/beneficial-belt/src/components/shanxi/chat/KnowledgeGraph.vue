<template>
  <div class="kg-container" ref="container">
    <div v-if="loading" class="kg-loading">
      <div class="kg-spin"></div>
      <span>{{ tr('正在用免费模型分析文档…') }}</span>
    </div>
    <div v-else-if="error" class="kg-error">
      <span>{{ error }}</span>
      <button class="kg-retry" @click="generate">{{ tr('重试') }}</button>
    </div>
    <div v-else-if="!hasData" class="kg-empty">{{ tr('暂无可图谱化的内容') }}</div>
    <svg v-else ref="svg" :width="width" :height="height"></svg>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch, nextTick } from 'vue'
import * as d3 from 'd3'
import { useI18n, tr } from '../../../composables/useI18n.js'



const props = defineProps({
  graph: { type: Object, default: () => ({ nodes: [], links: [] }) },
  loading: { type: Boolean, default: false },
  error: { type: String, default: '' },
})

const container = ref(null)
const svg = ref(null)
const width = ref(600)
const height = ref(400)

const hasData = ref(false)
let simulation = null
let zoomBehavior = null

// 实体类型 -> 颜色
const typeColor = {
  concept: '#3b82f6',
  person: '#f59e0b',
  tool: '#8b5cf6',
  technology: '#10b981',
  process: '#ec4899',
  principle: '#f97316',
  module: '#06b6d4',
  feature: '#6366f1',
}
const defaultColor = '#94a3b8'

watch(() => props.graph, (g) => {
  if (g && g.nodes && g.nodes.length) {
    hasData.value = true
    nextTick(() => render())
  } else {
    hasData.value = false
  }
}, { deep: true })

watch(() => props.loading, (v) => {
  if (!v && props.graph?.nodes?.length) {
    hasData.value = true
    nextTick(() => render())
  }
})

function render() {
  const el = svg.value
  if (!el) return
  const g = props.graph
  if (!g || !g.nodes?.length) return

  const w = width.value, h = height.value
  const svgEl = d3.select(el)
    .attr('width', w).attr('height', h)
    .html('')

  // 缩放容器
  const root = svgEl.append('g').attr('class', 'kg-root')

  // 缩放行为
  zoomBehavior = d3.zoom()
    .scaleExtent([0.3, 4])
    .on('zoom', (event) => {
      root.attr('transform', event.transform)
    })
  svgEl.call(zoomBehavior)

  // 力导向仿真
  const nodes = g.nodes.map(n => ({ ...n }))
  const links = g.links.map(l => ({ ...l }))

  simulation = d3.forceSimulation(nodes)
    .force('link', d3.forceLink(links).id(d => d.id).distance(100))
    .force('charge', d3.forceManyBody().strength(-220))
    .force('center', d3.forceCenter(w / 2, h / 2))
    .force('collision', d3.forceCollide(28))

  // 连线
  const linkG = root.append('g').attr('class', 'kg-links')
    .selectAll('line').data(links).join('line')
    .attr('stroke', '#cbd5e1')
    .attr('stroke-width', 1.5)
    .attr('opacity', 0.7)

  // 连线标签
  const linkLabelG = root.append('g').attr('class', 'kg-link-labels')
    .selectAll('text').data(links).join('text')
    .attr('font-size', '9px')
    .attr('fill', '#64748b')
    .attr('text-anchor', 'middle')
    .text(d => d.label || '')

  // 节点组
  const nodeG = root.append('g').attr('class', 'kg-nodes')
    .selectAll('g').data(nodes).join('g')
    .attr('cursor', 'pointer')
    .call(d3.drag()
      .on('start', (e, d) => {
        if (!e.active) simulation.alphaTarget(0.3).restart()
        d.fx = d.x; d.fy = d.y
      })
      .on('drag', (e, d) => { d.fx = e.x; d.fy = e.y })
      .on('end', (e, d) => {
        if (!e.active) simulation.alphaTarget(0)
        d.fx = null; d.fy = null
      })
    )

  // 节点圆形
  nodeG.append('circle')
    .attr('r', d => 10 + Math.min(d.count || 1, 8) * 2)
    .attr('fill', d => typeColor[d.type] || defaultColor)
    .attr('stroke', '#fff')
    .attr('stroke-width', 2)
    .style('filter', 'drop-shadow(0 2px 4px rgba(0,0,0,0.1))')

  // 节点名称
  nodeG.append('text')
    .attr('text-anchor', 'middle')
    .attr('dy', d => 12 + Math.min(d.count || 1, 8) * 2 + 12)
    .attr('font-size', '11px')
    .attr('font-weight', '600')
    .attr('fill', '#1e293b')
    .text(d => d.name)

  // tooltip
  const tooltip = d3.select(container.value).append('div')
    .attr('class', 'kg-tooltip')
    .style('opacity', 0)

  nodeG.on('mouseenter', function (e, d) {
    d3.select(this).select('circle')
      .transition().duration(150)
      .attr('r', (d.count ? 10 + Math.min(d.count, 8) * 2 : 14) + 4)
    tooltip.html(('<strong>' + d.name + tr('</strong><br>类型：') + d.type || tr('未知') + tr('<br>出现：') + d.count || 1 + tr(' 次')))
      .style('left', (e.offsetX + 12) + 'px')
      .style('top', (e.offsetY - 8) + 'px')
      .style('opacity', 1)
  })
  nodeG.on('mouseleave', function () {
    d3.select(this).select('circle')
      .transition().duration(150)
      .attr('r', (d.count ? 10 + Math.min(d.count, 8) * 2 : 14))
    tooltip.style('opacity', 0)
  })

  simulation.on('tick', () => {
    linkG
      .attr('x1', d => d.source.x).attr('y1', d => d.source.y)
      .attr('x2', d => d.target.x).attr('y2', d => d.target.y)
    linkLabelG
      .attr('x', d => (d.source.x + d.target.x) / 2)
      .attr('y', d => (d.source.y + d.target.y) / 2 - 4)
    nodeG.attr('transform', d => `translate(${d.x},${d.y})`)
  })
}

function updateSize() {
  if (!container.value) return
  const r = container.value.getBoundingClientRect()
  if (r.width > 50 && r.height > 50) {
    width.value = r.width
    height.value = r.height
  }
}

let ro = null
onMounted(() => {
  updateSize()
  if (typeof ResizeObserver !== 'undefined' && container.value) {
    ro = new ResizeObserver(() => {
      updateSize()
      if (hasData.value) nextTick(() => render())
    })
    ro.observe(container.value)
  }
  if (props.graph?.nodes?.length) {
    hasData.value = true
    nextTick(() => render())
  }
})

onUnmounted(() => {
  if (ro) ro.disconnect()
  if (simulation) simulation.stop()
})

function generate() {
  // 通知父组件重新生成
  emit('regenerate')
}

const emit = defineEmits(['regenerate'])
</script>

<style scoped>
.kg-container {
  width: 100%;
  height: 100%;
  min-height: 300px;
  position: relative;
  background: linear-gradient(135deg, #fafbfc 0%, #f1f5f9 100%);
  border-radius: 12px;
  overflow: hidden;
}
.kg-loading, .kg-error, .kg-empty {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  color: #64748b;
  font-size: 13px;
}
.kg-spin {
  width: 28px; height: 28px;
  border: 3px solid #e2e8f0;
  border-top-color: #3b82f6;
  border-radius: 50%;
  animation: kg-spin 0.8s linear infinite;
}
@keyframes kg-spin { to { transform: rotate(360deg); } }
.kg-error { color: #ef4444; }
.kg-retry {
  padding: 4px 14px;
  border: 1px solid #ef4444;
  border-radius: 6px;
  background: transparent;
  color: #ef4444;
  font-size: 12px;
  cursor: pointer;
}
.kg-retry:hover { background: #fef2f2; }
.kg-tooltip {
  position: absolute;
  pointer-events: none;
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  padding: 8px 12px;
  font-size: 12px;
  color: #1e293b;
  box-shadow: 0 4px 12px rgba(0,0,0,0.08);
  z-index: 100;
  max-width: 200px;
}
</style>
