<template>
  <div class="chart-renderer">
    <div class="chart-header">
      <Icon icon="mdi:chart-box-outline" width="14" color="#1950BE" />
      <span class="chart-title">{{ title || tr('数据图表') }}</span>
      <span class="chart-type-badge">{{ typeLabel }}</span>
    </div>
    <div ref="chartEl" class="chart-canvas"></div>
    <div class="chart-actions">
      <button class="chart-action-btn" @click="exportPNG" :title="tr('导出 PNG')">
        <Icon icon="mdi:download" width="13" />
        <span>PNG</span>
      </button>
      <button class="chart-action-btn" @click="exportSVG" :title="tr('导出 SVG')">
        <Icon icon="mdi:vector-square" width="13" />
        <span>SVG</span>
      </button>
      <button class="chart-action-btn" @click="toggleFullscreen" :title="tr('全屏')">
        <Icon :icon="fullscreen ? 'mdi:fullscreen-exit' : 'mdi:fullscreen'" width="13" />
      </button>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onBeforeUnmount, computed, watch, nextTick } from 'vue'
import * as echarts from 'echarts'
import { Icon } from '@iconify/vue'
import { useI18n, tr } from '../../../composables/useI18n.js'



const props = defineProps({
  title: { type: String, default: '' },
  type: { type: String, default: 'line' },
  data: { type: Object, required: true },
  options: { type: Object, default: () => ({}) },
  height: { type: Number, default: 320 }
})

const chartEl = ref(null)
const fullscreen = ref(false)
let chart = null

const typeLabel = computed(() => {
  const map = { line: tr('折线图'), bar: tr('柱状图'), pie: tr('饼图'), scatter: tr('散点图'), radar: tr('雷达图') }
  return map[props.type] || props.type
})

const brandColor = computed(() => props.options.color || '#1950BE')
const palette = computed(() => props.options.palette || [brandColor.value, '#E06E1E', '#12B76A', '#F59E0B', '#8B5CF6', '#06B6D4'])

function buildOption() {
  const opts = props.options
  const data = props.data
  const color = brandColor.value

  const base = {
    title: props.title ? { text: props.title, left: 'center', textStyle: { fontSize: 14, fontWeight: 600, color: '#1f2937' } } : undefined,
    tooltip: { trigger: 'axis', backgroundColor: 'rgba(255,255,255,0.96)', borderColor: '#e5e7eb', textStyle: { color: '#374151', fontSize: 12 } },
    legend: data.series || data.items ? { bottom: 0, textStyle: { fontSize: 11 } } : undefined,
    grid: ['pie', 'radar'].includes(props.type) ? undefined : { left: 48, right: 24, top: props.title ? 40 : 20, bottom: 40, containLabel: true },
    color: palette.value,
  }

  if (props.type === 'line') {
    if (data.series) {
      return { ...base, xAxis: { type: 'category', data: data.x, axisLabel: { fontSize: 11 } }, yAxis: { type: 'value', name: opts.y_label }, series: data.series.map(s => ({ name: s.name, type: 'line', data: s.data, smooth: opts.smooth, areaStyle: opts.fill ? { opacity: 0.15 } : undefined })) }
    }
    return { ...base, xAxis: { type: 'category', data: data.x, axisLabel: { fontSize: 11 } }, yAxis: { type: 'value', name: opts.y_label }, series: [{ type: 'line', data: data.y, smooth: opts.smooth, areaStyle: opts.fill ? { color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1, colorStops: [{ offset: 0, color: color + '40' }, { offset: 1, color: color + '00' }] } } : undefined, lineStyle: { width: 2.5 }, itemStyle: { color } }] }
  }

  if (props.type === 'bar') {
    if (data.series) {
      return { ...base, xAxis: { type: 'category', data: data.x, axisLabel: { fontSize: 11 } }, yAxis: { type: 'value', name: opts.y_label }, series: data.series.map(s => ({ name: s.name, type: 'bar', data: s.data, stack: opts.stacked ? 'total' : undefined })) }
    }
    const barOpts = { ...base, xAxis: { type: 'category', data: data.x, axisLabel: { fontSize: 11 } }, yAxis: { type: 'value', name: opts.y_label }, series: [{ type: 'bar', data: data.y, itemStyle: { color, borderRadius: [4, 4, 0, 0] }, barMaxWidth: 48 }] }
    if (opts.horizontal) {
      barOpts.xAxis = { type: 'value', name: opts.y_label }
      barOpts.yAxis = { type: 'category', data: data.x, axisLabel: { fontSize: 11 } }
      barOpts.series[0].itemStyle.borderRadius = [0, 4, 4, 0]
    }
    return barOpts
  }

  if (props.type === 'pie') {
    return { ...base, tooltip: { trigger: 'item', formatter: '{b}: {c} ({d}%)' }, series: [{ type: 'pie', radius: opts.radius || ['40%', '70%'], center: ['50%', '52%'], data: data.items, emphasis: { itemStyle: { shadowBlur: 10, shadowOffsetX: 0, shadowColor: 'rgba(0,0,0,0.2)' } }, label: { formatter: '{b}\n{d}%', fontSize: 11 } }] }
  }

  if (props.type === 'scatter') {
    return { ...base, xAxis: { type: 'value', name: opts.x_label, splitLine: { lineStyle: { type: 'dashed' } } }, yAxis: { type: 'value', name: opts.y_label, splitLine: { lineStyle: { type: 'dashed' } } }, series: [{ type: 'scatter', data: data.x.map((xi, i) => [xi, data.y[i]]), symbolSize: 10, itemStyle: { color, opacity: 0.8 } }] }
  }

  if (props.type === 'radar') {
    return { ...base, tooltip: {}, radar: { indicator: data.labels.map(l => ({ name: l, max: data.max, min: data.min })), center: ['50%', '55%'], radius: '65%' }, series: [{ type: 'radar', data: data.series ? data.series.map(s => ({ name: s.name, value: s.data })) : [{ value: data.y, name: props.title }], areaStyle: opts.fill ? { opacity: 0.2 } : undefined }] }
  }

  return base
}

function renderChart() {
  if (!chartEl.value) return
  if (chart) chart.dispose()
  chart = echarts.init(chartEl.value)
  chart.setOption(buildOption())
}

function exportPNG() {
  if (!chart) return
  const url = chart.getDataURL({ type: 'png', pixelRatio: 2, backgroundColor: '#fff' })
  const a = document.createElement('a')
  a.href = url
  a.download = `${props.title || 'chart'}.png`
  a.click()
}

function exportSVG() {
  if (!chart) return
  const svg = chart.renderToSVGString()
  const blob = new Blob([svg], { type: 'image/svg+xml' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `${props.title || 'chart'}.svg`
  a.click()
  URL.revokeObjectURL(url)
}

function toggleFullscreen() {
  fullscreen.value = !fullscreen.value
  document.body.classList.toggle('chart-fullscreen-active', fullscreen.value)
  nextTick(() => {
    chartEl.value.style.height = fullscreen.value ? '80vh' : props.height + 'px'
    chart?.resize()
  })
}

function handleResize() {
  chart?.resize()
}

onMounted(() => {
  chartEl.value.style.height = props.height + 'px'
  renderChart()
  window.addEventListener('resize', handleResize)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', handleResize)
  chart?.dispose()
})

watch(() => [props.data, props.type, props.options], renderChart, { deep: true })
</script>

<style scoped>
.chart-renderer {
  background: #fff;
  border: 1px solid var(--app-border, #e5e7eb);
  border-radius: 10px;
  padding: 12px;
  margin: 4px 0;
}
.chart-header {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 8px;
}
.chart-title {
  font-size: 13px;
  font-weight: 600;
  color: #1f2937;
  flex: 1;
}
.chart-type-badge {
  font-size: 10px;
  font-weight: 600;
  color: #1950BE;
  background: rgba(25, 80, 190, 0.08);
  padding: 2px 8px;
  border-radius: 10px;
}
.chart-canvas {
  width: 100%;
}
.chart-actions {
  display: flex;
  gap: 4px;
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px solid #f3f4f6;
}
.chart-action-btn {
  display: flex;
  align-items: center;
  gap: 3px;
  font-size: 11px;
  color: #6b7280;
  background: #f9fafb;
  border: 1px solid #e5e7eb;
  border-radius: 6px;
  padding: 3px 8px;
  cursor: pointer;
  transition: all 0.15s;
}
.chart-action-btn:hover {
  background: #1950BE;
  color: #fff;
  border-color: #1950BE;
}
</style>
