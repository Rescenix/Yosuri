<template>
  <!-- MarqueeText：超宽文本轮播。放得下=静止；放不下=来回缓滚（时长随距离自适应）。
       替代旧的「截断48字+省略号」：长命令行不再被掐尾，完整可读。 -->
  <span ref="box" class="marquee-box" :style="{ '--mq-dist': dist + 'px', '--mq-dur': dur + 's' }">
    <span ref="inner" class="marquee-inner" :class="{ 'is-scroll': overflow }">{{ text }}</span>
  </span>
</template>

<script setup>
import { nextTick, onMounted, onUnmounted, ref, watch } from 'vue'

const props = defineProps({
  text: { type: String, default: '' },
})

const box = ref(null)
const inner = ref(null)
const overflow = ref(false)
const dist = ref(0)
const dur = ref(6)
const SPEED = 30 // px/s 恒定滚动速度（时长随行程线性放大，不封顶→不会越来越快）

function measure() {
  const b = box.value, i = inner.value
  if (!b || !i) return
  // 单向右→左循环：起点第一个字立刻可见（不空等进场），匀速滚到露出尾部，
  // 两端各停一下让人读，再回到起点。行程=溢出量，时长随行程线性放大→恒速。
  const boxW = b.clientWidth
  const textW = i.scrollWidth
  const d = textW - boxW
  if (d > 2) {
    overflow.value = true
    dist.value = d + 6
    // 滚动段占 84%（两端各停 8%）→ 总时长 = 滚动时长 / 0.84
    dur.value = (dist.value / SPEED) / 0.84
  } else {
    overflow.value = false
    dist.value = 0
  }
}

let ro = null
onMounted(() => {
  measure()
  if (window.ResizeObserver && box.value) {
    ro = new ResizeObserver(measure)
    ro.observe(box.value)
  }
})
onUnmounted(() => { if (ro) ro.disconnect() })
watch(() => props.text, () => nextTick(measure))
</script>

<style scoped>
.marquee-box { display: block; overflow: hidden; min-width: 0; }
.marquee-inner { display: inline-block; white-space: nowrap; }
/* 单向右→左：起点停 8% → 匀速滚到尾部（linear 恒速）→ 尾部停 8% → 回起点 */
.marquee-inner.is-scroll { animation: mq-roll var(--mq-dur) linear infinite; }
@keyframes mq-roll {
  0%, 8% { transform: translateX(0); }
  92%, 100% { transform: translateX(calc(-1 * var(--mq-dist))); }
}
</style>
