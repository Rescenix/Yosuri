<template>
  <!-- 用户消息导航轴：一个节点 = 一条用户消息，点它跳过去。
       固定在聊天正文左侧居中，用短横线表达整段对话的位置。 -->
  <div v-if="items.length" class="umr" ref="rootRef" @mouseenter="onRootEnter" @mouseleave="onRootLeave">
    <div ref="trackRef" class="umr-track" @wheel.prevent="onWheel">
      <div
        v-for="(m, i) in items"
        :key="m.id"
        class="umr-node-wrap"
        :class="[{ active: activeIdx === i }, waveClass(i)]"
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

    <!-- 悬浮时展开完整用户消息列表（多条），点击任意一条跳转；
         列表内滚轮滚动时选中跟随（hoverIdx 同步），不只依赖刻度 -->
    <div
      v-if="hovered"
      ref="listRef"
      class="umr-list"
      @scroll="onListScroll"
      @mouseenter="cancelHide()"
      @mouseleave="scheduleHide()"
    >
      <div
        v-for="(m, i) in items"
        :key="m.id"
        class="umr-list-row"
        :class="{ active: jumpIdx === i, hovered: hoverIdx === i }"
        @mouseenter="hoverIdx = i"
        @click="$emit('jump', m.id)"
      >
        <span class="umr-list-text">{{ preview(m) }}</span>
      </div>
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
const listRef = ref(null)
const rootRef = ref(null)
const hoverIdx = ref(-1)
const tipTop = ref(0)
let hideTimer = null

// 只要用户真正说过的话：附件占位、空内容的气泡不该占一个点位
const items = computed(() =>
  (props.messages || []).filter(m => m.sender === 'user' && (m.content || '').trim())
)

// 刻度轴当前位置：activeId 空或找不到时回落到最后一条（当前位置指示）
const activeIdx = computed(() => {
  if (props.activeId == null || props.activeId === '') return items.value.length - 1
  const target = String(props.activeId)
  const idx = items.value.findIndex(m => String(m.id) === target)
  return idx >= 0 ? idx : items.value.length - 1
})

// 列表的跳转目标索引：必须精确匹配，找不到就 -1（不回落最新）
// 避免「activeId 设了但 id 对不上时又把最新消息标成选中」的 bug
const jumpIdx = computed(() => {
  if (props.activeId == null || props.activeId === '') return -1
  const target = String(props.activeId)
  return items.value.findIndex(m => String(m.id) === target)
})

const hovered = computed(() => (hoverIdx.value >= 0 ? items.value[hoverIdx.value] : null))

// 静止时全是短线；只有鼠标悬浮时，当前刻度与上下三根组成临时波峰。
function waveClass(i) {
  if (hoverIdx.value < 0) return ''
  const distance = Math.abs(i - hoverIdx.value)
  if (distance === 0) return 'wave-peak'
  return distance <= 3 ? `wave-${distance}` : ''
}

function preview(m) {
  const t = (m.content || '').replace(/\s+/g, ' ').trim()
  return t.length > 90 ? t.slice(0, 90) + '…' : t
}

// 节点较多时导航轴独立纵向滚动，不带动聊天正文。
function onWheel(e) {
  const el = trackRef.value
  if (!el) return
  el.scrollTop += (e.deltaY || e.deltaX)
}

// 列表滚动时选中跟随：把 hoverIdx 同步到当前视口顶部那一条，
// 刻度轴波浪也跟着走——滚动列表就能选中，不必回去点刻度。
// 设置一个短暂标记，让 watch(hoverIdx) 知道滚动来自列表自身，不强制居中（避免死循环锁死滚动）。
let listScrollGuard = false
let listScrollGuardTimer = null
function onListScroll() {
  const el = listRef.value
  if (!el) return
  listScrollGuard = true
  clearTimeout(listScrollGuardTimer)
  listScrollGuardTimer = setTimeout(() => { listScrollGuard = false }, 200)
  const rows = el.querySelectorAll('.umr-list-row')
  for (let i = 0; i < rows.length; i++) {
    if (rows[i].offsetTop + rows[i].offsetHeight / 2 >= el.scrollTop) {
      hoverIdx.value = i
      break
    }
  }
}

// 鼠标是否在 .umr 根内（刻度+列表）。用真实 mouseenter/mouseleave 事件跟踪，
// 不用 CSS :hover 伪类——列表是 absolute 定位在 .umr 盒子外的子元素，
// 滚动/动态渲染时 :hover 判定会滞后，导致「鼠标已离开但 matches(':hover') 仍为
// true → 永远不排定隐藏」（2026-08-31 实锤：列表一直显示不消失）。
let mouseInside = false
function onRootEnter() { mouseInside = true }
function onRootLeave() {
  mouseInside = false
  scheduleHide()
}

// 鼠标离开刻度/列表时延迟 200ms 再隐藏——留出从刻度移到列表的时间，
// 否则列表一弹鼠标一移开就消失，根本点不到列表里的条目。
// ⚠️ 守卫：只有鼠标真的离开整个 .umr 根（刻度+列表都算里面）才排定隐藏。
// 滚轮滚动列表时，行的 class 变化会让鼠标"离开"某一行触发 mouseleave 冒泡，
// 但鼠标其实还在列表里——直接隐藏会把列表销毁（消息多滚动时列表闪没，实测实锤 2026-08-31）。
function scheduleHide() {
  if (mouseInside) {
    // 鼠标仍在 .umr 根内（刻度或列表），不排定隐藏；取消待执行的隐藏
    cancelHide()
    return
  }
  if (hideTimer) clearTimeout(hideTimer)
  hideTimer = setTimeout(() => { hoverIdx.value = -1 }, 200)
}
function cancelHide() {
  if (hideTimer) { clearTimeout(hideTimer); hideTimer = null }
}

// 气泡跟着当前悬浮的节点定位；节点可能被滚出可视区，所以要减掉 scrollTop。
watch(hoverIdx, async (i) => {
  if (i < 0) return
  await nextTick()
  const el = trackRef.value
  const wrap = el?.children?.[i]
  const dot = wrap?.querySelector('.umr-node')
  if (!el || !dot) return
  const raw = dot.offsetTop - el.scrollTop + dot.offsetHeight / 2
  tipTop.value = Math.max(0, Math.min(raw, el.clientHeight))
  // 刻度选中 → 列表滚动跟随到该行（双向联动）。
  // ⚠️ 若滚动来自列表自身（用户正在滚列表），跳过强制居中——否则 scrollTop 被
  // 拉回顶部，与用户滚轮打架，形成死循环锁死滚动（消息多时 30 次滚轮 scrollTop 纹丝不动，实测实锤 2026-08-31）。
  const lst = listRef.value
  if (lst && !listScrollGuard) {
    const row = lst.querySelectorAll('.umr-list-row')[i]
    if (row) {
      const mid = row.offsetTop + row.offsetHeight / 2 - lst.clientHeight / 2
      if (Math.abs(mid - lst.scrollTop) > 4) lst.scrollTop = Math.max(0, mid)
    }
  }
})

// 新消息进来时自动滚到底，保持最近的提问可见。
watch(() => items.value.length, async () => {
  await nextTick()
  if (trackRef.value) trackRef.value.scrollTop = trackRef.value.scrollHeight
})
</script>

<style scoped>
.umr {
  position: absolute;
  z-index: 45;
  top: 24px;
  /* chat-content 紧邻会话列表；留 12px gutter 后开始画刻度。 */
  left: 12px;
  display: flex;
  align-items: center;
}
.umr-track {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  width: 48px;
  box-sizing: border-box;
  max-height: min(42vh, 360px);
  overflow-y: auto;
  scrollbar-width: none;      /* 轴本身就很细，再挂一条滚动条太吵 */
  padding: 6px 4px;
}
.umr-track::-webkit-scrollbar { display: none; }

.umr-node-wrap {
  position: relative;
  flex: 0 0 auto;
  display: flex;
  align-items: flex-start;
  flex-direction: column;
}

/* Codex 式刻度：默认短线，当前消息拉长并加深。 */
.umr-node-wrap:not(:last-child)::after {
  content: '';
  width: 0;
  height: 5px;
}

.umr-node {
  position: relative;
  flex: 0 0 auto;
  /* 全局 button 使用 border-box；这里的 width 必须只描述可见细线，
     左右 padding 仅负责扩大点击热区，否则短线会被 padding 完全吃掉。 */
  box-sizing: content-box;
  width: 7px;
  height: 1px;
  padding: 4px 8px;
  border: 0;
  border-radius: 999px;
  background: color-mix(in srgb, var(--app-text-faint, #a1a1aa) 54%, transparent);
  background-clip: content-box;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: width 0.18s ease, height 0.18s ease, background 0.18s ease;
}
.umr-node-num {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
}

.umr-node:hover,
.umr-node.hovered {
  width: 7px;
  height: 1px;
  background-color: var(--app-text-soft, #71717a);
}

.umr-node.active {
  width: 7px;
  height: 1px;
  background-color: var(--app-text, #27272a);
}
.umr-node-wrap.wave-3 .umr-node { width: 11px; }
.umr-node-wrap.wave-2 .umr-node { width: 16px; }
.umr-node-wrap.wave-1 .umr-node { width: 23px; }
.umr-node-wrap.wave-peak .umr-node {
  width: 32px;
  height: 2px;
  background-color: var(--app-text, #27272a);
}

/* 悬浮列表：从轴的右侧紧贴展开（无缝隙，鼠标可顺滑移入），显示全部用户消息 */
.umr-list {
  position: absolute;
  left: calc(100%);
  top: 0;
  width: min(360px, 42vw);
  max-height: 46vh;
  overflow-y: auto;
  background: var(--app-surface);
  border: 1px solid var(--app-border);
  border-radius: 10px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
  padding: 4px;
  z-index: 60;
}
.umr-list-row {
  display: flex;
  gap: 8px;
  align-items: baseline;
  padding: 6px 8px;
  border-radius: 6px;
  cursor: pointer;
  font-size: 12px;
  line-height: 1.4;
  color: var(--app-text);
}
.umr-list-row:hover,
.umr-list-row.hovered {
  background: color-mix(in srgb, var(--app-accent, #6366f1) 10%, transparent);
}
.umr-list-row.active {
  background: color-mix(in srgb, var(--app-accent, #6366f1) 14%, transparent);
  font-weight: 600;
}
.umr-list-text {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 720px) {
  .umr { left: 0; }
  .umr-track { padding-inline: 3px; }
  .umr-node-wrap.wave-peak .umr-node { width: 28px; }
  .umr-list { width: min(300px, 76vw); }
}
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
  width: 20px;
  height: 20px;
  border-width: 1.5px;
  border-color: rgba(240, 98, 146, 0.5);
  border-radius: 0;
  background: radial-gradient(circle at 30% 30%, rgba(100, 32, 62, 0.9), rgba(38, 14, 24, 0.98));
  box-shadow: inset 0 0 9px rgba(240, 98, 146, 0.18), 0 0 0 1px rgba(0, 0, 0, 0.4);
  transform: rotate(45deg);
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
  transform: rotate(45deg) scale(1.25);
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
  transform: rotate(45deg) scale(1.15);
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

/* 悬浮列表：粉丝绒卡片 */
[data-skin="witchtrial_hiiro"] .umr-list {
  background: rgba(32, 16, 22, 0.96);
  border-color: rgba(233, 30, 99, 0.32);
  border-radius: 14px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.45), inset 0 0 22px rgba(233, 30, 99, 0.05);
}
[data-skin="witchtrial_hiiro"] .umr-list-row:hover,
[data-skin="witchtrial_hiiro"] .umr-list-row.hovered {
  background: rgba(233, 30, 99, 0.12);
}
[data-skin="witchtrial_hiiro"] .umr-list-row.active {
  background: rgba(233, 30, 99, 0.2);
}
[data-skin="witchtrial_hiiro"] .umr-list-text {
  color: #ffeef4;
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
  width: 16px;
  height: 16px;
  border-width: 1.5px;
  border-color: rgba(199, 62, 62, 0.5);
  border-radius: 0;
  background: radial-gradient(circle at 30% 30%, rgba(65, 22, 22, 0.9), rgba(22, 10, 12, 0.98));
  box-shadow: inset 0 0 6px rgba(199, 62, 62, 0.18), 0 0 0 1px rgba(0, 0, 0, 0.45);
  transform: rotate(45deg);
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
  transform: rotate(45deg) scale(1.15);
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

/* 悬浮列表：羊皮纸 */
[data-skin="witchtrial"] .umr-list {
  background: rgba(28, 20, 16, 0.96);
  border-color: rgba(139, 110, 90, 0.38);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.5), inset 0 0 22px rgba(139, 110, 90, 0.06);
}
[data-skin="witchtrial"] .umr-list-row:hover,
[data-skin="witchtrial"] .umr-list-row.hovered {
  background: rgba(139, 110, 90, 0.12);
}
[data-skin="witchtrial"] .umr-list-row.active {
  background: rgba(139, 110, 90, 0.2);
}
[data-skin="witchtrial"] .umr-list-text {
  color: #e8ddd0;
}

/* 主题只改变颜色，不改变 Codex 式竖向刻度的尺寸与形态。 */
[data-skin="witchtrial_hiiro"] .umr,
[data-skin="witchtrial"] .umr {
  border: 0;
  background: transparent;
}
[data-skin="witchtrial_hiiro"] .umr-node,
[data-skin="witchtrial"] .umr-node {
  width: 7px;
  height: 1px;
  border: 0;
  border-radius: 999px;
  background: color-mix(in srgb, var(--app-text-faint) 54%, transparent);
  background-clip: content-box;
  box-shadow: none;
  transform: none;
  animation: none;
}
[data-skin="witchtrial_hiiro"] .umr-node:hover,
[data-skin="witchtrial_hiiro"] .umr-node.hovered,
[data-skin="witchtrial"] .umr-node:hover,
[data-skin="witchtrial"] .umr-node.hovered {
  width: 7px;
  height: 1px;
  background: var(--app-text-soft);
  background-clip: content-box;
  box-shadow: none;
  transform: none;
}
[data-skin="witchtrial_hiiro"] .umr-node.active,
[data-skin="witchtrial_hiiro"] .umr-node-wrap.active .umr-node,
[data-skin="witchtrial"] .umr-node.active {
  width: 7px !important;
  height: 1px !important;
  border: 0 !important;
  border-radius: 999px;
  background: var(--app-text) !important;
  background-clip: content-box !important;
  box-shadow: none;
  transform: none;
  animation: none;
}
[data-skin="witchtrial_hiiro"] .umr-node-wrap:not(:last-child)::after,
[data-skin="witchtrial"] .umr-node-wrap:not(:last-child)::after {
  width: 0;
  height: 5px;
  border: 0;
  background: transparent;
  box-shadow: none;
}

[data-skin="witchtrial_hiiro"] .umr-node-wrap.wave-3 .umr-node,
[data-skin="witchtrial"] .umr-node-wrap.wave-3 .umr-node { width: 11px !important; }
[data-skin="witchtrial_hiiro"] .umr-node-wrap.wave-2 .umr-node,
[data-skin="witchtrial"] .umr-node-wrap.wave-2 .umr-node { width: 16px !important; }
[data-skin="witchtrial_hiiro"] .umr-node-wrap.wave-1 .umr-node,
[data-skin="witchtrial"] .umr-node-wrap.wave-1 .umr-node { width: 23px !important; }
[data-skin="witchtrial_hiiro"] .umr-node-wrap.wave-peak .umr-node,
[data-skin="witchtrial"] .umr-node-wrap.wave-peak .umr-node {
  width: 32px !important;
  height: 2px !important;
  background: var(--app-text) !important;
  background-clip: content-box !important;
}

@media (max-width: 720px) {
  [data-skin="witchtrial_hiiro"] .umr-node-wrap.wave-peak .umr-node,
  [data-skin="witchtrial"] .umr-node-wrap.wave-peak .umr-node {
    width: 28px !important;
  }
}
</style>
