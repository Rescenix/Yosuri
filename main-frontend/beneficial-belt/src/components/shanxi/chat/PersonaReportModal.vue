<template>
  <Teleport to="body">
    <div class="prm-backdrop" @click.self="close">
      <div class="prm-card">
        <div class="prm-scroll">
          <!-- 封面 -->
          <section class="prm-section prm-cover">
            <div class="prm-cover-icon"><Icon icon="mdi:heart-pulse" width="56" /></div>
            <div class="prm-cover-title">你的{{ rangeLabel }}人设周报</div>
            <div class="prm-cover-sub">
              这一周，你换了 <b>{{ stats.switchCount }}</b> 次人设<br />
              和 AI 一起过了 <b>{{ stats.activeDays }}</b> 天
            </div>
          </section>

          <!-- 换人设次数 -->
          <section class="prm-section prm-sec-1">
            <div class="prm-bignum">{{ stats.switchCount }}</div>
            <div class="prm-title">这周你换了 {{ stats.switchCount }} 次人设</div>
            <div class="prm-text" v-if="stats.switchCount >= 7">比我换衣服还勤，怎么，是隔壁的御姐不够香了吗</div>
            <div class="prm-text" v-else-if="stats.switchCount >= 3">雨露均沾型选手，每个预设都是你的心头好</div>
            <div class="prm-text" v-else>从一而终，认定一个就不撒手</div>
          </section>

          <!-- 每日随机 -->
          <section class="prm-section prm-sec-2">
            <div class="prm-bignum">{{ stats.randomDays }}</div>
            <div class="prm-title">每日随机开了 {{ stats.randomDays }} 天</div>
            <div class="prm-text" v-if="stats.randomTop">抽奖池里最常翻牌的是 <b>{{ stats.randomTop }}</b></div>
            <div class="prm-text" v-else>把今天交给命运，明天的事明天再说</div>
          </section>

          <!-- 最宠人设 -->
          <section class="prm-section prm-sec-3">
            <div class="prm-cover-icon"><Icon :icon="topIcon" width="48" /></div>
            <div class="prm-title">本周最宠人设</div>
            <div class="prm-bignum prm-bignum-name">{{ stats.topName }}</div>
            <div class="prm-text">翻牌了 <b>{{ stats.topCount }}</b> 次，是真正的正宫</div>
          </section>

          <!-- 自定义 -->
          <section class="prm-section prm-sec-4">
            <div class="prm-bignum">{{ stats.customCount }}</div>
            <div class="prm-title">你亲手捏了 {{ stats.customCount }} 个人设</div>
            <div class="prm-text" v-if="stats.customCount > 0">从捏人设到训 AI，你已经是个合格的造物主了</div>
            <div class="prm-text" v-else>本周还没动手捏，预设够用就好</div>
          </section>

          <!-- 结尾 -->
          <section class="prm-section prm-sec-5">
            <div class="prm-cover-icon"><Icon icon="mdi:hand-wave" width="56" /></div>
            <div class="prm-title">下周，也要好好聊天哦</div>
            <div class="prm-text">每一天的随机，都是新的相遇</div>
            <button class="prm-close-btn" type="button" @click="close">收下这份周报</button>
          </section>
        </div>
        <button class="prm-x" type="button" title="关闭" @click="close"><Icon icon="mdi:close" width="18" /></button>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { computed } from 'vue'
import { Icon } from '@iconify/vue'

const emit = defineEmits(['close'])

// ── 数据：读 localStorage.personaHistory，统计最近 7 天 ──
const rangeDays = 7
const history = computed(() => {
  try {
    const arr = JSON.parse(localStorage.getItem('personaHistory') || '[]')
    return Array.isArray(arr) ? arr : []
  } catch { return [] }
})
const cutoff = computed(() => {
  const d = new Date()
  d.setDate(d.getDate() - rangeDays)
  d.setHours(0, 0, 0, 0)
  return d.getTime()
})
const weekHistory = computed(() => history.value.filter(x => x.ts >= cutoff.value))
const rangeLabel = computed(() => {
  const end = new Date()
  const start = new Date()
  start.setDate(start.getDate() - rangeDays + 1)
  const f = d => `${d.getMonth() + 1}.${d.getDate()}`
  return `${f(start)}-${f(end)}`
})
const stats = computed(() => {
  const h = weekHistory.value
  const switchCount = h.length
  const activeDays = new Set(h.map(x => x.key)).size
  const randomDays = new Set(h.filter(x => x.mode === 'random').map(x => x.key)).size
  const counts = {}
  for (const x of h) counts[x.name] = (counts[x.name] || 0) + 1
  let topName = '', topCount = 0
  for (const [n, c] of Object.entries(counts)) {
    if (c > topCount) { topName = n; topCount = c }
  }
  const customCount = (() => {
    try {
      const arr = JSON.parse(localStorage.getItem('myPersonas') || '[]')
      return Array.isArray(arr) ? arr.length : 0
    } catch { return 0 }
  })()
  const rCounts = {}
  for (const x of h.filter(x => x.mode === 'random')) rCounts[x.name] = (rCounts[x.name] || 0) + 1
  let randomTop = ''
  let rMax = 0
  for (const [n, c] of Object.entries(rCounts)) {
    if (c > rMax) { randomTop = n; rMax = c }
  }
  return { switchCount, activeDays, randomDays, topName, topCount, customCount, randomTop }
})

const TOP_ICONS = {
  'Yosuri酱': 'mdi:heart',
  '猫娘': 'mdi:cat',
  '御姐': 'mdi:flower-tulip',
  '萝莉': 'mdi:candy',
  '学姐': 'mdi:school',
}
const topIcon = computed(() => TOP_ICONS[stats.value.topName] || 'mdi:account-heart')

const close = () => emit('close')
</script>

<style scoped>
.prm-backdrop {
  position: fixed; inset: 0; z-index: 100000;
  display: flex; align-items: center; justify-content: center;
  background: rgba(0, 0, 0, .55); backdrop-filter: blur(6px);
}
.prm-card {
  position: relative;
  width: min(440px, 90vw); height: min(640px, 86vh);
  background: linear-gradient(160deg, #14101f 0%, #1a142e 55%, #241a3d 100%);
  border-radius: 20px; overflow: hidden;
  box-shadow: 0 24px 60px rgba(0, 0, 0, .5);
  color: #fff;
}
.prm-scroll {
  height: 100%; overflow-y: auto;
  scrollbar-width: none;
}
.prm-scroll::-webkit-scrollbar { display: none; }
.prm-x {
  position: absolute; top: 14px; right: 14px;
  width: 30px; height: 30px; border-radius: 50%;
  border: none; background: rgba(255,255,255,.1); color: rgba(255,255,255,.75);
  cursor: pointer; display: flex; align-items: center; justify-content: center;
}
.prm-x:hover { background: rgba(255,255,255,.2); color: #fff; }
.prm-section {
  min-height: 320px; padding: 40px 30px 36px;
  box-sizing: border-box;
  display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 12px;
  text-align: center;
}
.prm-cover { min-height: 340px; gap: 16px; }
.prm-sec-1 { background: linear-gradient(165deg, #2d1b4e 0%, #1a1033 100%); }
.prm-sec-2 { background: linear-gradient(165deg, #4a1942 0%, #2d1030 100%); }
.prm-sec-3 { background: linear-gradient(165deg, #16324f 0%, #0f2037 100%); }
.prm-sec-4 { background: linear-gradient(165deg, #1f3d2b 0%, #122617 100%); }
.prm-sec-5 { background: linear-gradient(165deg, #3d1e3d 0%, #241124 100%); }
.prm-cover-icon { display: flex; color: rgba(255,255,255,.85); }
.prm-cover-title { font-size: 24px; font-weight: 800; letter-spacing: .02em; }
.prm-cover-sub { font-size: 13.5px; color: rgba(255,255,255,.65); line-height: 1.9; }
.prm-cover-sub b { color: #fff; }
.prm-bignum { font-size: 72px; font-weight: 900; line-height: 1; font-family: "JetBrains Mono", ui-monospace, monospace; text-shadow: 0 8px 30px rgba(0,0,0,.35); }
.prm-bignum-name { font-size: 42px; }
.prm-title { font-size: 18px; font-weight: 700; }
.prm-text { font-size: 13.5px; color: rgba(255,255,255,.7); line-height: 1.8; max-width: 300px; }
.prm-text b { color: #fff; }
.prm-close-btn {
  margin-top: 12px; padding: 9px 26px; border: none; border-radius: 999px;
  background: rgba(255,255,255,.14); color: #fff; font-size: 13px; font-weight: 650;
  cursor: pointer; transition: background .15s;
}
.prm-close-btn:hover { background: rgba(255,255,255,.26); }
</style>
