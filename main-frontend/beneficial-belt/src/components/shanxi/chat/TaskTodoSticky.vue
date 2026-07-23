<template>
  <!-- 便签(仿拍立得/贴纸):顶上一枚图钉,列出当前任务 TODO,随执行实时勾选。
       默认常驻显示(空的时候也在,给个占位),不再只在有任务时才冒出来。 -->
  <div class="task-sticky" :class="{ empty: !items.length }">
    <span class="sticky-pin"></span>
    <div class="sticky-head">
      <span class="sticky-title">当前任务</span>
      <span v-if="items.length" class="sticky-count">{{ doneCount }}/{{ items.length }}</span>
    </div>
    <ul v-if="items.length" class="sticky-list">
      <li v-for="(it, i) in items" :key="i" class="sticky-item" :class="'s-' + it.status">
        <span class="sticky-check">
          <svg v-if="it.status === 'done'" viewBox="0 0 16 16" width="13" height="13">
            <path d="M3 8.5l3 3 7-7" fill="none" stroke="currentColor" stroke-width="2"
              stroke-linecap="round" stroke-linejoin="round" />
          </svg>
          <span v-else-if="it.status === 'doing'" class="sticky-spin"></span>
          <span v-else class="sticky-dot"></span>
        </span>
        <span class="sticky-text">{{ it.text }}</span>
      </li>
    </ul>
    <div v-else class="sticky-empty">暂无任务 · 空闲中～</div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
const props = defineProps({
  items: { type: Array, default: () => [] }
})
const doneCount = computed(() => props.items.filter(i => i.status === 'done').length)
</script>

<style scoped>
.task-sticky {
  position: relative;
  width: 200px;
  max-height: 260px;
  overflow-y: auto;
  padding: 16px 14px 13px;
  /* 便签是刻意的"纸"，不跟随 --app-surface（那会让它变成普通面板，失去质感）。
     但暗色下一张纯白纸会非常刺眼，所以单独给一套暗色纸张变量。 */
  background: var(--sticky-paper, #fffdf5);
  /* 微微的纸张质感 + 轻微旋转,像随手贴的便签 */
  background-image:
    repeating-linear-gradient(180deg, transparent, transparent 27px, var(--sticky-rule, rgba(0, 0, 0, 0.03)) 28px);
  border-radius: 3px;
  box-shadow: 0 6px 18px rgba(0, 0, 0, 0.14), 0 1px 3px rgba(0, 0, 0, 0.1);
  transform: rotate(-2deg);
  font-family: var(--app-font, "PingFang SC", sans-serif);
}
/* 顶部图钉 */
.sticky-pin {
  position: absolute;
  top: -7px;
  left: 50%;
  transform: translateX(-50%);
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: radial-gradient(circle at 35% 30%, #ff8a8a, #e0443f);
  box-shadow: 0 2px 3px rgba(0, 0, 0, 0.28);
}
.sticky-pin::after {
  content: '';
  position: absolute;
  top: 11px;
  left: 50%;
  transform: translateX(-50%);
  width: 2px;
  height: 6px;
  background: rgba(0, 0, 0, 0.18);
}
.sticky-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin-bottom: 8px;
}
.sticky-title {
  font-size: 13.5px;
  font-weight: 700;
  color: var(--sticky-ink, #4a4436);
  letter-spacing: 0.5px;
}
.sticky-count {
  font-size: 11px;
  color: var(--sticky-ink-faint, #a89f88);
  font-variant-numeric: tabular-nums;
}
/* 空态:便签矮一点、给一句占位 */
.task-sticky.empty { padding-bottom: 14px; }
.sticky-empty {
  font-size: 12px;
  color: var(--sticky-ink-faint, #b3a98f);
  padding: 2px 0 0;
}
.sticky-list { list-style: none; margin: 0; padding: 0; }
.sticky-item {
  display: flex;
  align-items: flex-start;
  gap: 7px;
  padding: 3px 0;
  font-size: 12.5px;
  line-height: 1.5;
  color: var(--sticky-ink-soft, #5b544a);
}
.sticky-check {
  flex-shrink: 0;
  width: 15px;
  height: 15px;
  margin-top: 2px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.sticky-dot {
  width: 9px;
  height: 9px;
  border: 1.5px solid #c4baa2;
  border-radius: 50%;
}
.sticky-spin {
  width: 11px;
  height: 11px;
  border: 2px solid rgba(224, 68, 63, 0.25);
  border-top-color: #e0443f;
  border-radius: 50%;
  animation: stickySpin 0.8s linear infinite;
}
@keyframes stickySpin { to { transform: rotate(360deg); } }
.sticky-text { min-width: 0; word-break: break-word; }
/* 完成:打勾变绿、文字划掉弱化 */
.sticky-item.s-done .sticky-check { color: #4caf50; }
.sticky-item.s-done .sticky-text {
  color: var(--sticky-ink-faint, #a89f88);
  text-decoration: line-through;
  text-decoration-color: rgba(168, 159, 136, 0.6);
}
/* 进行中:文字加重 */
.sticky-item.s-doing .sticky-text { color: var(--app-text); font-weight: 600; }

/* 弹入动画 */
.sticky-pop-enter-active { transition: transform 0.28s cubic-bezier(0.22, 1, 0.36, 1), opacity 0.28s ease; }
.sticky-pop-leave-active { transition: transform 0.2s ease, opacity 0.2s ease; }
.sticky-pop-enter-from { opacity: 0; transform: rotate(-2deg) translateY(14px) scale(0.9); }
.sticky-pop-leave-to { opacity: 0; transform: rotate(-2deg) scale(0.92); }
</style>
