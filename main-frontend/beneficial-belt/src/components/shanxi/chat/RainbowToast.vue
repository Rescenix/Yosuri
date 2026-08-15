<template>
  <Teleport to="body">
    <div class="rainbow-toasts">
      <TransitionGroup name="rb-pop">
        <div v-for="t in toasts" :key="t.id" class="rb-toast" :class="{ 'rb-dark': isDark }">
          <!-- 星星线稿图标（lucide sparkles 标准路径，紫罗兰描边）+ 动态扫描线 -->
          <span class="rb-icon-wrap">
            <svg class="rb-icon" viewBox="0 0 24 24" fill="none" stroke="#a78bfa" stroke-width="1.9"
                 stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <path d="M9.937 15.5A2 2 0 0 0 8.5 14.063l-6.135-1.582a.5.5 0 0 1 0-.962L8.5 9.936A2 2 0 0 0 9.937 8.5l1.582-6.135a.5.5 0 0 1 .963 0L14.063 8.5A2 2 0 0 0 15.5 9.937l6.135 1.581a.5.5 0 0 1 0 .964L15.5 14.063a2 2 0 0 0-1.437 1.437l-1.582 6.135a.5.5 0 0 1-.963 0z" />
              <path d="M20 3v4" />
              <path d="M22 5h-4" />
              <path d="M4 17v2" />
              <path d="M5 18H3" />
            </svg>
            <span class="rb-scanline"></span>
          </span>

          <div class="rb-body">
            <!-- 彩虹渐变主文案：已保存(蜜桃橙) → 到(粉) → 记忆(淡紫) -->
            <span class="rb-title">
              <span v-for="(ch, i) in titleChars" :key="i" :class="'rb-ch-' + (i % 3)">{{ ch }}</span>
            </span>
            <span v-if="t.sub" class="rb-sub">{{ t.sub }}</span>
          </div>

          <div class="rb-meta">
            <span v-if="t.count != null" class="rb-count">{{ t.count }}</span>
            <span v-if="t.ms != null" class="rb-ms">{{ t.ms }}</span>
          </div>
        </div>
      </TransitionGroup>
    </div>
  </Teleport>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'

const toasts = ref([])
let idSeq = 0
const seen = new Set()
let timer = 0

// 亮/暗自适应：跟随 html[data-theme]（useTheme.js 运行时注入）
const isDark = ref(document.documentElement.getAttribute('data-theme') === 'dark')
const themeObserver = new MutationObserver(() => {
  isDark.value = document.documentElement.getAttribute('data-theme') === 'dark'
})

function push(t) {
  const id = ++idSeq
  toasts.value.push({
    id,
    title: t.title || '已保存到记忆',
    sub: t.sub || '',
    count: t.count != null ? t.count : null,
    ms: t.ms != null ? t.ms : null,
  })
  setTimeout(() => {
    toasts.value = toasts.value.filter(x => x.id !== id)
  }, t.duration || 4200)
}

// 全局触发 API：window.showRainbowToast({ title, sub, count, ms, duration })
function onGlobal(e) {
  push(e.detail || {})
}
window.__rainbowToast = push

onMounted(() => {
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] })
  window.addEventListener('rainbow-toast', onGlobal)
  // 技能习得轮询（沿用 SkillToasts 的 /api/evolve/events 数据源，换成彩虹样式）
  poll()
  timer = setInterval(poll, 15000)
})
onUnmounted(() => {
  themeObserver.disconnect()
  window.removeEventListener('rainbow-toast', onGlobal)
  delete window.__rainbowToast
  clearInterval(timer)
})

async function poll() {
  try {
    const r = await fetch('/api/evolve/events')
    if (!r.ok) return
    const { events } = await r.json()
    if (!Array.isArray(events)) return
    if (seen.size === 0) {
      for (const e of events) seen.add(e.name)
      return
    }
    for (const e of events) {
      if (!seen.has(e.name)) {
        seen.add(e.name)
        push({ title: '新技能已习得', sub: e.name, count: '+XP', ms: '已入库' })
      }
    }
  } catch (e) { /* 后端没起静默 */ }
}

const titleChars = computed(() => {
  const t = toasts.value[0]
  return t ? t.title.split('') : []
})
</script>

<style scoped>
.rainbow-toasts {
  position: fixed;
  left: 24px;
  bottom: 96px;
  z-index: 999999;
  display: flex;
  flex-direction: column-reverse;
  gap: 10px;
  pointer-events: none;
}

.rb-toast {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 250px;
  max-width: 340px;
  padding: 12px 16px;
  background: rgba(255, 255, 255, 0.92);
  backdrop-filter: blur(14px);
  -webkit-backdrop-filter: blur(14px);
  border: 1px solid rgba(226, 232, 240, 0.9);
  border-radius: 16px;
  box-shadow: 0 14px 40px rgba(120, 90, 160, 0.14), 0 2px 8px rgba(120, 90, 160, 0.06);
}
.rb-toast.rb-dark {
  background: rgba(38, 38, 42, 0.92);
  border-color: rgba(70, 70, 80, 0.9);
  box-shadow: 0 14px 40px rgba(0, 0, 0, 0.5), 0 2px 8px rgba(0, 0, 0, 0.3);
}

.rb-icon-wrap {
  position: relative;
  width: 36px;
  height: 36px;
  flex: 0 0 auto;
  overflow: hidden;
  border-radius: 10px;
  display: grid;
  place-items: center;
}
.rb-icon {
  width: 30px;
  height: 30px;
  opacity: 0.9;
  display: block;
}
.rb-dark .rb-icon { stroke: #c4b5fd; }

/* 动态扫描线：暖橙→淡紫 渐变光带，自上而下循环扫描（正是截图里那个动态效果） */
.rb-scanline {
  position: absolute;
  left: 4px;
  right: 4px;
  height: 3px;
  border-radius: 2px;
  background: linear-gradient(90deg, transparent, rgba(255, 154, 92, 0.85), rgba(179, 157, 219, 0.85), transparent);
  filter: blur(0.3px);
  box-shadow: 0 0 6px rgba(255, 154, 92, 0.5), 0 0 12px rgba(179, 157, 219, 0.35);
  pointer-events: none;
  animation: rb-scan 1.8s ease-in-out infinite;
}
.rb-dark .rb-scanline {
  background: linear-gradient(90deg, transparent, rgba(255, 170, 110, 0.9), rgba(190, 165, 235, 0.9), transparent);
  box-shadow: 0 0 6px rgba(255, 170, 110, 0.55), 0 0 12px rgba(190, 165, 235, 0.4);
}
@keyframes rb-scan {
  0%   { top: 6%;  opacity: 0; }
  12%  { opacity: 1; }
  88%  { opacity: 1; }
  100% { top: 88%; opacity: 0; }
}

.rb-body { min-width: 0; display: flex; flex-direction: column; gap: 2px; }

/* 彩虹渐变逐字：已保存(蜜桃橙) → 到(粉) → 记忆(淡紫) */
.rb-title {
  font-size: 14px;
  font-weight: 700;
  letter-spacing: 0.02em;
  line-height: 1.2;
  white-space: nowrap;
}
.rb-ch-0 { color: #ff9a5c; }   /* 蜜桃橙 */
.rb-ch-1 { color: #ff7eb3; }   /* 粉 */
.rb-ch-2 { color: #b39ddb; }   /* 淡紫 */

.rb-sub {
  font-size: 11px;
  color: var(--app-text-soft, #6b6b6b);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 220px;
}
.rb-dark .rb-sub { color: #a8a8b0; }

.rb-meta {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 10px;
  color: #9a9aa5;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}
.rb-count { color: #a78bfa; font-weight: 700; }
.rb-ms { color: #b9b9c4; }

/* 入场：弹性放大 + 上浮；退场：左滑淡出 */
.rb-pop-enter-active { transition: all 0.4s cubic-bezier(0.34, 1.56, 0.64, 1); }
.rb-pop-leave-active { transition: all 0.28s ease; }
.rb-pop-enter-from { opacity: 0; transform: translateY(18px) scale(0.85); }
.rb-pop-leave-to { opacity: 0; transform: translateX(-16px) scale(0.9); }
</style>
