<template>
  <!-- 用户消息导航轴：一个圆点 = 一条用户消息，点它跳过去。
       长对话里靠滚滚动条找"我当时问的那句话"很痛苦，这条轴把提问节奏压成一行。 -->
  <div v-if="items.length" class="umr" @mouseleave="hoverIdx = -1">
    <div ref="trackRef" class="umr-track" @wheel.prevent="onWheel">
      <button
        v-for="(m, i) in items"
        :key="m.id"
        class="umr-dot"
        :class="{ hovered: hoverIdx === i }"
        :aria-label="`跳到第 ${i + 1} 条提问`"
        @mouseenter="hoverIdx = i"
        @click="$emit('jump', m.id)"
      ></button>
    </div>

    <!-- 悬浮预览：跟着圆点走，超出轴宽时贴边，不让它飘到工具栏外面 -->
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
})
defineEmits(['jump'])

const trackRef = ref(null)
const hoverIdx = ref(-1)
const tipLeft = ref(0)

// 只要用户真正说过的话：附件占位、空内容的气泡不该占一个点位
const items = computed(() =>
  (props.messages || []).filter(m => m.sender === 'user' && (m.content || '').trim())
)

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

// 气泡跟着当前悬浮的圆点定位；圆点可能被滚出可视区，所以要减掉 scrollLeft
watch(hoverIdx, async (i) => {
  if (i < 0) return
  await nextTick()
  const el = trackRef.value
  const dot = el?.children?.[i]
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
  gap: 7px;
  overflow-x: auto;
  scrollbar-width: none;      /* 轴本身就很细，再挂一条滚动条太吵 */
  padding: 6px 2px;
  width: 100%;
}
.umr-track::-webkit-scrollbar { display: none; }
.umr-dot {
  flex: 0 0 auto;
  width: 7px;
  height: 7px;
  padding: 0;
  border: none;
  border-radius: 50%;
  background: var(--app-border, #d4d4d8);
  cursor: pointer;
  transition: transform 0.12s ease, background 0.12s ease;
}
.umr-dot:hover,
.umr-dot.hovered {
  background: var(--app-accent, #6366f1);
  transform: scale(1.6);
}
.umr-tip {
  position: absolute;
  bottom: calc(100% + 4px);
  transform: translateX(-50%);
  max-width: 320px;
  display: flex;
  gap: 6px;
  align-items: baseline;
  padding: 5px 9px;
  border-radius: 8px;
  background: var(--app-surface, #fff);
  border: 1px solid var(--app-border, #e5e5e5);
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.1);
  font-size: 12px;
  line-height: 1.4;
  color: var(--app-text, #1a1a1a);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  pointer-events: none;   /* 别挡住下面的圆点，否则鼠标一移上来就闪 */
  z-index: 60;
}
.umr-tip-idx { color: var(--app-text-faint, #94a3b8); flex: 0 0 auto; }
.umr-tip-text { overflow: hidden; text-overflow: ellipsis; }
</style>
