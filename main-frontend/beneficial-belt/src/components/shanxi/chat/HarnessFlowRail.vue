<template>
  <aside class="harness-flow-rail" :class="{ compact }" aria-label="Harness 实时架构">
    <div ref="canvasRef" class="harness-flow-canvas" title="滚轮滚动画布；Ctrl/⌘ + 滚轮缩放画布" @wheel="handleCanvasWheel">
      <div class="harness-graph-viewport" :style="graphViewportStyle">
      <div class="harness-graph-stage" :style="graphStageStyle">
        <svg class="harness-edges" viewBox="0 0 360 620" preserveAspectRatio="xMidYMin meet" aria-hidden="true">
          <defs>
            <marker id="harness-arrow" viewBox="0 0 7 7" markerWidth="7" markerHeight="7" refX="6" refY="3.5" markerUnits="userSpaceOnUse" orient="auto">
              <path d="M0,0 L7,3.5 L0,7 Z" fill="var(--app-accent)" />
            </marker>
          </defs>
          <path
            v-for="edge in edges"
            :key="edge.key"
            :d="edge.d"
            class="harness-edge"
            :class="{ active: edge.active, complete: edge.complete }"
            marker-end="url(#harness-arrow)"
          />
        </svg>

        <section
          v-for="node in nodes"
          :key="node.key"
          class="harness-graph-node"
          :class="[node.key, node.state]"
        >
          <strong>{{ node.label }}</strong>
          <span>{{ node.detail }}</span>
        </section>

        <span class="harness-loop-label">AGENT LOOP</span>
        <span class="harness-ops-label">TRACE / VERIFY</span>
      </div>
      </div>
    </div>
  </aside>
</template>

<script setup>
import { computed, nextTick, ref } from 'vue'

const props = defineProps({
  flow: { type: Object, default: null },
  compact: { type: Boolean, default: false }
})
const blocks = computed(() => props.flow?.blocks || [])
const tools = computed(() => blocks.value.filter(block => block.type === 'tool'))
const runningTool = computed(() => tools.value.find(block => block.status === 'running'))
const lastTool = computed(() => tools.value[tools.value.length - 1])
const hasThinking = computed(() => blocks.value.some(block => block.type === 'thinking'))
const hasIntent = computed(() => blocks.value.some(block => block.type === 'intent'))
const canvasRef = ref(null)
const graphZoom = ref(1)
const graphViewportStyle = computed(() => ({
  width: `${360 * graphZoom.value}px`,
  height: `${620 * graphZoom.value}px`
}))
const graphStageStyle = computed(() => ({ transform: `scale(${graphZoom.value})` }))

function handleCanvasWheel(event) {
  // 普通滚轮仍由可滚动容器处理；只接管浏览器会拿去做页面缩放的 Ctrl/⌘ + 滚轮。
  if (!event.ctrlKey && !event.metaKey) return
  event.preventDefault()
  const canvas = canvasRef.value
  if (!canvas) return

  const rect = canvas.getBoundingClientRect()
  const pointerX = event.clientX - rect.left
  const pointerY = event.clientY - rect.top
  const previousZoom = graphZoom.value
  const nextZoom = Math.min(2.4, Math.max(0.55, previousZoom * Math.exp(-event.deltaY * 0.0015)))
  if (nextZoom === previousZoom) return

  // 缩放后把鼠标所指的画布坐标留在原位，而不是跳回画布左上角。
  const graphX = (canvas.scrollLeft + pointerX) / previousZoom
  const graphY = (canvas.scrollTop + pointerY) / previousZoom
  graphZoom.value = nextZoom
  nextTick(() => {
    canvas.scrollLeft = graphX * nextZoom - pointerX
    canvas.scrollTop = graphY * nextZoom - pointerY
  })
}

const stage = computed(() => {
  if (!props.flow) return -1
  if (props.flow.status === 'failed') return 7
  if (props.flow.status === 'completed') return 7
  if (runningTool.value) return 3
  if (hasIntent.value) return 4
  if (hasThinking.value) return 2
  if (props.flow.modelInfo) return 1
  return 0
})

const nodeState = index => {
  if (props.flow?.status === 'failed' && index === 7) return 'error'
  if (stage.value === index && props.flow?.status === 'running') return 'active'
  return stage.value > index || props.flow?.status === 'completed' ? 'complete' : 'idle'
}

const nodes = computed(() => [
  { key: 'gateway', label: 'Gateway', detail: props.flow ? 'task received' : 'waiting', state: nodeState(0) },
  { key: 'memory', label: 'Working memory', detail: props.flow?.modelInfo ? 'context assembled' : 'per turn', state: nodeState(1) },
  { key: 'agent', label: 'LLM agent', detail: hasThinking.value ? 'reasoning…' : 'reason', state: nodeState(2) },
  { key: 'tools', label: 'Tools', detail: runningTool.value?.name || lastTool.value?.name || 'create event…', state: nodeState(3) },
  { key: 'reply', label: 'Reply', detail: props.flow?.status === 'completed' ? 'back to you' : 'compose', state: nodeState(4) },
  { key: 'trace', label: 'Trace', detail: `${tools.value.length} event${tools.value.length === 1 ? '' : 's'} · always on`, state: nodeState(5) },
  { key: 'eval', label: 'Eval', detail: 'deterministic + judge', state: nodeState(6) },
  { key: 'release', label: 'Release', detail: props.flow?.status === 'failed' ? 'blocked' : 'result gate', state: nodeState(7) }
])

const edgeDefs = [
  { key: 'gateway-memory', d: 'M 102 118 C 114 118, 120 118, 132 118', at: 1 },
  { key: 'memory-agent', d: 'M 188 154 C 188 170, 188 178, 188 194', at: 2 },
  { key: 'agent-tools', d: 'M 188 266 C 188 278, 188 286, 188 300', at: 3 },
  { key: 'tools-agent', d: 'M 208 300 C 226 282, 226 276, 208 266', at: 2 },
  { key: 'agent-reply', d: 'M 244 230 C 252 230, 258 230, 266 230', at: 4 },
  { key: 'reply-trace', d: 'M 310 266 C 310 294, 310 312, 310 340', at: 5 },
  { key: 'trace-eval', d: 'M 310 406 L 310 438', at: 6 },
  { key: 'eval-release', d: 'M 310 500 L 310 532', at: 7 },
  { key: 'reply-loop', d: 'M 342 230 C 350 104, 250 52, 64 70 C 42 74, 40 88, 40 96', at: 0 }
]
const edges = computed(() => edgeDefs.map(edge => ({
  ...edge,
  active: props.flow?.status === 'running' && stage.value === edge.at,
  complete: stage.value > edge.at || props.flow?.status === 'completed'
})))
</script>

<style scoped>
.harness-flow-rail {
  position: absolute;
  z-index: 2;
  inset: -8px -8px -8px auto;
  width: 368px;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  padding: 0 8px 8px;
  overflow: hidden;
  background:
    radial-gradient(circle at 0 0, color-mix(in srgb, var(--app-accent) 8%, transparent), transparent 38%),
    linear-gradient(color-mix(in srgb, var(--app-text) 3.5%, transparent) 1px, transparent 1px),
    linear-gradient(90deg, color-mix(in srgb, var(--app-text) 3.5%, transparent) 1px, transparent 1px);
  background-size: auto, 24px 24px, 24px 24px;
  pointer-events: auto;
}
.harness-flow-canvas {
  position: relative;
  flex: 1;
  min-height: 0;
  overflow: auto;
  scrollbar-width: thin;
  scrollbar-color: color-mix(in srgb, var(--app-accent) 38%, transparent) transparent;
}
.harness-flow-canvas::-webkit-scrollbar { width: 8px; height: 8px; }
.harness-flow-canvas::-webkit-scrollbar-thumb { border: 2px solid transparent; border-radius: 999px; background: color-mix(in srgb, var(--app-accent) 42%, transparent); background-clip: padding-box; }
.harness-graph-viewport { position: relative; min-width: 360px; min-height: 620px; }
.harness-graph-stage { position: relative; width: 360px; height: 620px; transform-origin: top left; }
.harness-edges { position: absolute; inset: 0; width: 360px; height: 620px; overflow: visible; }
.harness-edge {
  fill: none;
  stroke: color-mix(in srgb, var(--app-text-faint) 45%, transparent);
  stroke-width: 1;
  stroke-dasharray: 4 5;
  transition: stroke .2s ease, opacity .2s ease;
}
.harness-edge.complete { stroke: color-mix(in srgb, var(--app-accent) 38%, var(--app-border)); }
.harness-edge.active {
  stroke: var(--app-accent);
  stroke-width: 1.7;
  filter: drop-shadow(0 0 4px color-mix(in srgb, var(--app-accent) 38%, transparent));
  animation: harnessFlow 1s linear infinite;
}
.harness-graph-node {
  position: absolute;
  z-index: 1;
  box-sizing: border-box;
  min-height: 58px;
  padding: 10px 11px;
  border: 1px solid color-mix(in srgb, var(--app-border) 78%, transparent);
  border-radius: 9px;
  background: color-mix(in srgb, var(--app-surface) 90%, transparent);
  transition: border-color .2s ease, background .2s ease, box-shadow .2s ease;
}
.harness-graph-node strong { display: block; color: var(--app-text-soft); font-size: 11px; line-height: 1.2; }
.harness-graph-node span { display: block; margin-top: 5px; overflow: hidden; color: var(--app-text-faint); font: 8.5px/1.25 "JetBrains Mono", ui-monospace, monospace; text-overflow: ellipsis; white-space: nowrap; }
.harness-graph-node.complete { border-color: color-mix(in srgb, var(--app-accent) 26%, var(--app-border)); }
.harness-graph-node.active { border-color: var(--app-accent); background: color-mix(in srgb, var(--app-accent) 9%, var(--app-surface)); box-shadow: 0 0 0 3px color-mix(in srgb, var(--app-accent) 10%, transparent), 0 8px 20px color-mix(in srgb, var(--app-accent) 12%, transparent); }
.harness-graph-node.active strong { color: var(--app-accent); }
.harness-graph-node.error { border-color: #df4d43; }
.harness-graph-node.gateway { left: 8px; top: 88px; width: 96px; }
.harness-graph-node.memory { left: 132px; top: 88px; width: 112px; }
.harness-graph-node.agent { left: 132px; top: 194px; width: 112px; }
.harness-graph-node.tools { left: 132px; top: 300px; width: 112px; }
.harness-graph-node.reply { left: 266px; top: 194px; width: 86px; }
.harness-graph-node.trace { left: 266px; top: 340px; width: 86px; }
.harness-graph-node.eval { left: 266px; top: 438px; width: 86px; }
.harness-graph-node.release { left: 266px; top: 532px; width: 86px; }
.harness-loop-label, .harness-ops-label { position: absolute; color: var(--app-text-faint); font: 8px/1 "JetBrains Mono", ui-monospace, monospace; letter-spacing: .12em; }
.harness-loop-label { left: 134px; top: 176px; }
.harness-ops-label { left: 266px; top: 320px; color: var(--app-accent); }
.harness-flow-rail.compact {
  position: relative;
  inset: auto;
  width: 100%;
  height: auto;
  min-height: 260px;
  padding: 0;
}
@keyframes harnessFlow { to { stroke-dashoffset: -18; } }
@media (max-width: 1199px) {
  .harness-flow-rail:not(.compact) { display: none; }
}
</style>
