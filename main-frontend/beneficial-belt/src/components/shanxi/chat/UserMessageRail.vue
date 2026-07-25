<template>
  <!-- 用户消息导航轴：一个节点 = 一条用户消息，点它跳过去。
       长对话里靠滚滚动条找"我当时问的那句话"很痛苦，这条轴把提问节奏压成一行。 -->
  <div v-if="items.length" class="umr" @mouseleave="hoverIdx = -1">
    <div ref="trackRef" class="umr-track" @wheel.prevent="onWheel">
      <div
        v-for="(m, i) in items"
        :key="m.id"
        class="umr-node-wrap"
        :class="{ active: activeIdx === i }"
      >
        <button
          class="umr-node"
          :class="{ hovered: hoverIdx === i, active: activeIdx === i }"
          :aria-label="`跳到第 ${i + 1} 条提问`"
          @mouseenter="hoverIdx = i"
          @click="$emit('jump', m.id)"
        >
          <span class="umr-node-num">{{ i + 1 }}</span>
        </button>
      </div>
    </div>

    <!-- 悬浮预览：跟着节点走，超出轴宽时贴边，不让它飘到工具栏外面 -->
    <div v-if="hovered" class="umr-tip" :style="{ left: tipLeft + 'px' }">
      <span class="umr-tip-idx">#{{ hoverIdx + 1 }}</span>
      <span class="umr-tip-text">{{ preview(hovered) }}</span>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, nextTick, watch } from 'vue'

const props = defineProps({
  // 完整消息列表，组件自己筛出用户消息
  messages: { type: Array, default: () => [] },
  // 当前高亮的用户消息 id（最后一条或用户指定）
  activeId: { type: String, default: '' }
})
defineEmits(['jump'])

const trackRef = ref(null)
const hoverIdx = ref(-1)
const tipLeft = ref(0)

// 只要用户真正说过的话：附件占位、空内容的气泡不该占一个点位
const items = computed(() =>
  (props.messages || []).filter(m => m.sender === 'user' && (m.content || '').trim())
)

const activeIdx = computed(() => {
  if (props.activeId == null || props.activeId === '') return items.value.length - 1
  const target = String(props.activeId)
  const idx = items.value.findIndex(m => String(m.id) === target)
  return idx >= 0 ? idx : items.value.length - 1
})

const hovered = computed(() => (hoverIdx.value >= 0 ? items.value[hoverIdx.value] : null))

function preview(m) {
  const t = (m.content || '').replace(/\s+/g, ' ').trim()
  return t.length > 90 ? t.slice(0, 90) + '…' : t
}

// 轴很窄，鼠标滚轮走横向更顺手（deltaY 直接喂 scrollLeft，不用按住 shift）
function onWheel(e) {
  const el = trackRef.value
  if (!el) return
  el.scrollLeft += (e.deltaY || e.deltaX)
}

// 气泡跟着当前悬浮的节点定位；节点可能被滚出可视区，所以要减掉 scrollLeft
watch(hoverIdx, async (i) => {
  if (i < 0) return
  await nextTick()
  const el = trackRef.value
  const wrap = el?.children?.[i]
  const dot = wrap?.querySelector('.umr-node')
  if (!el || !dot) return
  const raw = dot.offsetLeft - el.scrollLeft + dot.offsetWidth / 2
  tipLeft.value = Math.max(0, Math.min(raw, el.clientWidth))
})

// 新消息进来时自动滚到最右，保持"最近的提问"可见
watch(() => items.value.length, async () => {
  await nextTick()
  if (trackRef.value) trackRef.value.scrollLeft = trackRef.value.scrollWidth
})
</script>

<style scoped>
.umr {
  position: relative;
  flex: 1 1 auto;
  min-width: 0;
  display: flex;
  align-items: center;
  padding: 0 10px;
}
.umr-track {
  display: flex;
  align-items: center;
  overflow-x: auto;
  scrollbar-width: none;      /* 轴本身就很细，再挂一条滚动条太吵 */
  padding: 8px 4px;
  width: 100%;
}
.umr-track::-webkit-scrollbar { display: none; }

.umr-node-wrap {
  position: relative;
  flex: 0 0 auto;
  display: flex;
  align-items: center;
}

/* 节点之间的水平连线 */
.umr-node-wrap:not(:last-child)::after {
  content: '';
  width: 18px;
  height: 2px;
  margin: 0 4px;
  border-radius: 1px;
  background: var(--app-border, #e4e4e7);
  transition: background 0.2s ease;
}
/* 已走过路径：从起点到当前激活节点之间的连线和节点都高亮 */
.umr-node-wrap:has(~ .active):not(:last-child)::after,
.umr-node-wrap.active:not(:last-child)::after {
  background: var(--app-accent, #6366f1);
}
.umr-node-wrap:has(~ .active) .umr-node {
  border-color: color-mix(in srgb, var(--app-accent, #6366f1) 55%, transparent);
}
.umr-node-wrap:has(~ .active) .umr-node-num {
  color: color-mix(in srgb, var(--app-accent, #6366f1) 75%, transparent);
}

.umr-node {
  position: relative;
  flex: 0 0 auto;
  width: 20px;
  height: 20px;
  padding: 0;
  border: 2px solid var(--app-border, #e4e4e7);
  border-radius: 50%;
  background: var(--app-surface, #fff);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.18s cubic-bezier(0.34, 1.56, 0.64, 1);
}
.umr-node-num {
  font-size: 10px;
  font-weight: 600;
  color: var(--app-text-soft, #71717a);
  line-height: 1;
  user-select: none;
}

.umr-node:hover,
.umr-node.hovered {
  border-color: var(--app-accent, #6366f1);
  transform: scale(1.15);
  box-shadow: 0 0 0 3px rgba(99, 102, 241, 0.12);
}
.umr-node:hover .umr-node-num,
.umr-node.hovered .umr-node-num {
  color: var(--app-accent, #6366f1);
}

.umr-node.active {
  border-color: var(--app-accent, #6366f1);
  background: var(--app-accent, #6366f1);
  transform: scale(1.15);
}
.umr-node.active .umr-node-num {
  color: #fff;
}

.umr-tip {
  position: absolute;
  bottom: calc(100% + 6px);
  transform: translateX(-50%);
  max-width: 320px;
  display: flex;
  gap: 6px;
  align-items: baseline;
  padding: 6px 10px;
  border-radius: 10px;
  background: var(--app-surface);
  border: 1px solid var(--app-border);
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.1);
  font-size: 12px;
  line-height: 1.4;
  color: var(--app-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  pointer-events: none;   /* 别挡住下面的节点，否则鼠标一移上来就闪 */
  z-index: 60;
}
.umr-tip-idx { color: var(--app-text-faint); flex: 0 0 auto; font-weight: 600; }
.umr-tip-text { overflow: hidden; text-overflow: ellipsis; }
</style>

<style>
/* ==================== 二阶堂希罗 · 红黑洛丽塔导航轴 ==================== */

/* 轨道：粉色波点 + 底部蕾丝 */
[data-skin="witchtrial_hiiro"] .umr {
  background: rgba(233, 30, 99, 0.04);
  border-top: 1px solid rgba(233, 30, 99, 0.12);
  border-bottom: 1px solid rgba(233, 30, 99, 0.12);
}

/* 节点：洛丽塔丝绒圆点 */
[data-skin="witchtrial_hiiro"] .umr-node {
  width: 26px;
  height: 26px;
  border-width: 1.5px;
  border-color: rgba(240, 98, 146, 0.5);
  border-radius: 50%;
  background: radial-gradient(circle at 30% 30%, rgba(100, 32, 62, 0.9), rgba(38, 14, 24, 0.98));
  box-shadow: inset 0 0 9px rgba(240, 98, 146, 0.18), 0 0 0 1px rgba(0, 0, 0, 0.4);
}
[data-skin="witchtrial_hiiro"] .umr-node-num {
  color: #ffc2d6;
  font-family: var(--app-font, 'ZCOOL QingKe HuangYou', 'PingFang SC', cursive);
  font-size: 12px;
}

/* 已走过节点：粉丝绒 */
[data-skin="witchtrial_hiiro"] .umr-node-wrap:has(~ .active) .umr-node {
  border-color: rgba(240, 98, 146, 0.75);
  background: radial-gradient(circle at 30% 30%, rgba(120, 40, 72, 0.9), rgba(44, 16, 28, 0.98));
}
[data-skin="witchtrial_hiiro"] .umr-node-wrap:has(~ .active) .umr-node-num {
  color: #ffd6e4;
}

/* 当前激活节点：实心粉填充 + 白字（提高优先级，确保点击后变色不被默认 hover 覆盖） */
[data-skin="witchtrial_hiiro"] .umr-node.active,
[data-skin="witchtrial_hiiro"] .umr-node-wrap.active .umr-node {
  border-color: rgba(240, 98, 146, 0.95) !important;
  background: linear-gradient(145deg, #f06292, #c2185b) !important;
  box-shadow:
    0 0 0 1px rgba(0, 0, 0, 0.5),
    inset 0 1px 0 rgba(255, 220, 235, 0.35);
  transform: scale(1.25);
}
[data-skin="witchtrial_hiiro"] .umr-node.active .umr-node-num,
[data-skin="witchtrial_hiiro"] .umr-node-wrap.active .umr-node .umr-node-num {
  color: #fff !important;
  text-shadow: none;
}

/* 覆盖默认 scoped 的 hover 蓝紫光，改用柔和的粉色内发光 */
[data-skin="witchtrial_hiiro"] .umr-node:hover,
[data-skin="witchtrial_hiiro"] .umr-node.hovered {
  border-color: rgba(240, 98, 146, 0.85);
  transform: scale(1.15);
  box-shadow: inset 0 0 12px rgba(240, 98, 146, 0.35), 0 0 0 1px rgba(0, 0, 0, 0.4);
}
[data-skin="witchtrial_hiiro"] .umr-node:hover .umr-node-num,
[data-skin="witchtrial_hiiro"] .umr-node.hovered .umr-node-num {
  color: #ffd6e4;
}

/* 点击按下态：变深 */
[data-skin="witchtrial_hiiro"] .umr-node:active {
  background: linear-gradient(145deg, #c2185b, #880e4f) !important;
  border-color: rgba(240, 98, 146, 0.8) !important;
  transform: scale(0.95);
}
[data-skin="witchtrial_hiiro"] .umr-node:active .umr-node-num {
  color: #ffd6e4;
}

/* 连线：粉色藤蔓 */
[data-skin="witchtrial_hiiro"] .umr-node-wrap:has(~ .active):not(:last-child)::after,
[data-skin="witchtrial_hiiro"] .umr-node-wrap.active:not(:last-child)::after {
  height: 2.5px;
  border-radius: 2px;
  background: linear-gradient(90deg, rgba(240,98,146,0.25), rgba(240,98,146,0.95), rgba(240,98,146,0.25));
}

/* 悬浮提示：粉丝绒卡片 */
[data-skin="witchtrial_hiiro"] .umr-tip {
  background: rgba(32, 16, 22, 0.96);
  border-color: rgba(233, 30, 99, 0.32);
  color: #ffeef4;
  border-radius: 14px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.45), inset 0 0 22px rgba(233, 30, 99, 0.05);
}
[data-skin="witchtrial_hiiro"] .umr-tip-idx {
  color: #ff9ec4;
}

/* ==================== 魔女审判 · 用户消息导航轴 ==================== */

/* 轨道：暗红烙印底 */
[data-skin="witchtrial"] .umr {
  background: rgba(199, 62, 62, 0.05);
  border-top: 1px solid rgba(199, 62, 62, 0.14);
  border-bottom: 1px solid rgba(199, 62, 62, 0.14);
}

/* 节点：审判火印 */
[data-skin="witchtrial"] .umr-node {
  width: 22px;
  height: 22px;
  border-width: 1.5px;
  border-color: rgba(199, 62, 62, 0.5);
  background: radial-gradient(circle at 30% 30%, rgba(65, 22, 22, 0.9), rgba(22, 10, 12, 0.98));
  box-shadow: inset 0 0 6px rgba(199, 62, 62, 0.18), 0 0 0 1px rgba(0, 0, 0, 0.45);
}
[data-skin="witchtrial"] .umr-node-num {
  color: #e08a78;
  font-family: var(--app-font, 'Cinzel', 'Noto Serif SC', serif);
  font-size: 10px;
}

/* 已走过节点：暗红烙印 */
[data-skin="witchtrial"] .umr-node-wrap:has(~ .active) .umr-node {
  border-color: rgba(199, 62, 62, 0.7);
  background: radial-gradient(circle at 30% 30%, rgba(85, 28, 28, 0.9), rgba(30, 14, 16, 0.98));
}
[data-skin="witchtrial"] .umr-node-wrap:has(~ .active) .umr-node-num {
  color: #f0a898;
}

/* 当前激活节点：燃烧 */
[data-skin="witchtrial"] .umr-node.active {
  border-color: rgba(199, 62, 62, 0.95);
  background: radial-gradient(circle at 30% 30%, #c73e3e, #681a1a);
  box-shadow:
    0 0 0 1px rgba(0, 0, 0, 0.5),
    0 0 14px rgba(199, 62, 62, 0.6),
    inset 0 0 10px rgba(255, 160, 120, 0.3);
  animation: umr-flame-pulse 1.6s ease-in-out infinite;
}
[data-skin="witchtrial"] .umr-node.active .umr-node-num {
  color: #fff;
  text-shadow: 0 0 8px rgba(255, 120, 80, 0.9);
}

@keyframes umr-flame-pulse {
  0%, 100% { box-shadow: 0 0 0 1px rgba(0,0,0,0.5), 0 0 14px rgba(199,62,62,0.6), inset 0 0 10px rgba(255,160,120,0.3); }
  50% { box-shadow: 0 0 0 1px rgba(0,0,0,0.5), 0 0 24px rgba(199,62,62,0.9), inset 0 0 14px rgba(255,160,120,0.45); }
}

/* 连线：炽热轨迹 */
[data-skin="witchtrial"] .umr-node-wrap:has(~ .active):not(:last-child)::after,
[data-skin="witchtrial"] .umr-node-wrap.active:not(:last-child)::after {
  height: 2px;
  background: linear-gradient(90deg, rgba(199,62,62,0.3), rgba(199,62,62,0.95), rgba(199,62,62,0.3));
  box-shadow: 0 0 8px rgba(199, 62, 62, 0.5);
}

/* 悬浮提示：羊皮纸 */
[data-skin="witchtrial"] .umr-tip {
  background: rgba(28, 20, 16, 0.96);
  border-color: rgba(139, 110, 90, 0.38);
  color: #e8ddd0;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.5), inset 0 0 22px rgba(139, 110, 90, 0.06);
}
[data-skin="witchtrial"] .umr-tip-idx {
  color: #e08a78;
}
</style>
