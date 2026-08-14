<template>
  <Teleport to="body">
    <div class="skill-toasts">
      <TransitionGroup name="sk-pop">
        <div v-for="t in toasts" :key="t.id" class="sk-toast">
          <div class="sk-icon">💡</div>
          <div class="sk-body">
            <span class="sk-tag">技能习得</span>
            <strong>{{ t.name }}</strong>
          </div>
          <span class="sk-xp">+30 XP</span>
        </div>
      </TransitionGroup>
    </div>
  </Teleport>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'

const toasts = ref([])
const seen = new Set()
let timer = 0
let idSeq = 0

async function poll() {
  try {
    const r = await fetch('/api/evolve/events')
    if (!r.ok) return
    const { events } = await r.json()
    if (!Array.isArray(events)) return
    // 首次只记录基线，不弹历史技能
    if (seen.size === 0) {
      for (const e of events) seen.add(e.name)
      return
    }
    for (const e of events) {
      if (!seen.has(e.name)) {
        seen.add(e.name)
        const id = ++idSeq
        toasts.value.push({ id, name: e.name })
        setTimeout(() => {
          toasts.value = toasts.value.filter(t => t.id !== id)
        }, 4500)
      }
    }
  } catch (e) { /* 后端没起静默 */ }
}

onMounted(() => {
  poll()
  timer = setInterval(poll, 15000)
})
onUnmounted(() => clearInterval(timer))
</script>

<style scoped>
.skill-toasts { position: fixed; left: 24px; bottom: 96px; z-index: 999999; display: flex; flex-direction: column-reverse; gap: 10px; pointer-events: none; }
.sk-toast { display: flex; align-items: center; gap: 10px; min-width: 240px; max-width: 320px; padding: 11px 14px; background: #fff; border: 1px solid #e0e7ff; border-radius: 14px; box-shadow: 0 12px 34px rgba(79, 70, 229, 0.18); }
.sk-icon { display: grid; place-items: center; width: 32px; height: 32px; flex: 0 0 auto; border-radius: 10px; background: linear-gradient(145deg, #6366f1, #8b5cf6); color: #fff; }
.sk-body { min-width: 0; }
.sk-tag { display: block; color: #6366f1; font-size: 9px; font-weight: 800; letter-spacing: 0.14em; }
.sk-body strong { display: block; overflow: hidden; margin-top: 1px; color: #111827; font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }
.sk-xp { margin-left: auto; color: #f59e0b; font-size: 12px; font-weight: 900; white-space: nowrap; }
.sk-pop-enter-active, .sk-pop-leave-active { transition: all 0.35s cubic-bezier(0.34, 1.56, 0.64, 1); }
.sk-pop-enter-from { opacity: 0; transform: translateY(18px) scale(0.85); }
.sk-pop-leave-to { opacity: 0; transform: translateX(-16px) scale(0.9); }
</style>
