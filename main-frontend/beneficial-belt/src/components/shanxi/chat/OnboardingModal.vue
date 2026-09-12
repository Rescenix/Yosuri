<template>
  <Teleport to="body">
    <div class="ob-mask">
      <div class="ob-card" role="dialog" :aria-label="tr('首次启动引导')">
        <div class="ob-steps">
          <span v-for="(s, i) in STEPS" :key="s" class="ob-dot" :class="{ on: i === step, done: i < step }"></span>
        </div>

        <!-- 1 语言 + 领养 Agent -->
        <section v-if="step === 0" class="ob-body">
          <h2>{{ tr('选语言，领养你的 Agent') }}</h2>
          <p class="ob-sub">{{ tr('语言随时可在「设置 → 通用」改；选一张角色卡就是领养一个 Agent，人设和头像都写在这张卡上，之后也可在「设置 → 角色卡」里改。') }}</p>

          <div class="ob-lang-grid">
            <button
              v-for="l in LANGS"
              :key="l.id"
              class="ob-lang"
              :class="{ on: pickedLang === l.id }"
              type="button"
              @click="pickLang(l.id)"
            >
              <span class="ob-lang-name">{{ l.name }}</span>
              <span class="ob-lang-tag">{{ l.tag }}</span>
            </button>
          </div>

          <div class="ob-persona-grid">
            <button
              v-for="p in presets"
              :key="p.id"
              class="ob-persona"
              :class="{ on: pickedPersona === p.id }"
              type="button"
              @click="pickPersona(p)"
            >
              <span class="ob-persona-icon"><Icon :icon="p.icon" width="20" /></span>
              <span class="ob-persona-body">
                <span class="ob-persona-name">{{ tr(p.name) }}</span>
                                <span class="ob-persona-desc">{{ tr(p.desc) }}</span>
              </span>
            </button>
          </div>

          <label class="ob-autostart">
            <input v-model="autoStart" type="checkbox" />
            <span class="ob-autostart-body">
              <span class="ob-autostart-title">{{ tr('开机自动启动') }}</span>
              <span class="ob-autostart-desc">{{ tr('随 Windows 启动后台常驻，随时响应；可在「设置 → 版本」随时关闭。') }}</span>
            </span>
          </label>
        </section>

        <!-- 2 功能导航 + 用户协议 -->
        <section v-else class="ob-body">
          <h2>{{ tr('这些入口，随时能去') }}</h2>
          <p class="ob-sub">{{ tr('右下角折角导航可以拖动、增删、排序。') }}</p>
          <div class="ob-grid">
            <button
              v-for="item in navigableItems"
              :key="item.id"
              class="ob-item"
              type="button"
              :disabled="!agreed"
              @click="go(item)"
            >
              <span class="ob-item-icon"><Icon :icon="item.icon" width="20" /></span>
              <span class="ob-item-body">
                <span class="ob-item-label">{{ tr(item.label) }}</span>
                <span class="ob-item-desc">{{ tr(desc[item.id] || '') }}</span>
              </span>
              <span class="ob-item-arrow">›</span>
            </button>
          </div>
        </section>

        <footer class="ob-foot">
          <div class="ob-agree-row">
            <label class="ob-check">
              <input v-model="agreed" type="checkbox" />
              <span>{{ tr('我已阅读并同意') }}</span>
            </label>
            <button class="ob-link" type="button" @click="showAgreement = true">{{ tr('《Yosuri 用户协议》') }}</button>
            <label v-if="step === 1" class="ob-check ob-check-right">
              <input v-model="dontShow" type="checkbox" />
              <span>{{ tr('不再显示') }}</span>
            </label>
          </div>
          <div class="ob-foot-actions">
            <button v-if="step > 0" class="ob-btn" type="button" @click="step--">{{ tr('上一步') }}</button>
            <button v-if="step < STEPS.length - 1" class="ob-btn ob-btn-primary" type="button" :disabled="!agreed" @click="step++">{{ tr('继续') }}</button>
            <button v-else class="ob-btn ob-btn-primary" type="button" :disabled="!agreed" @click="finish">{{ tr('开始使用') }}</button>
          </div>
        </footer>

        <!-- 完整协议：卡片内覆盖层，蓝链点开 -->
        <div v-if="showAgreement" class="ob-doc">
          <header class="ob-doc-head">
            <strong>{{ AGREEMENT_TITLE }}</strong>
            <button class="ob-doc-close" type="button" :title="tr('返回')" :aria-label="tr('返回')" @click="showAgreement = false">
              <Icon icon="mdi:close" width="18" />
            </button>
          </header>
          <div class="ob-doc-body">
            <section v-for="s in AGREEMENT_SECTIONS" :key="s.h" class="ob-doc-sec">
              <h3>{{ s.h }}</h3>
              <p>{{ s.p }}</p>
            </section>
          </div>
          <footer class="ob-doc-foot">
            <button class="ob-btn ob-btn-primary" type="button" @click="readAndAgree">{{ tr('已阅读，同意') }}</button>
          </footer>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Icon } from '@iconify/vue'
import { useI18n, tr } from '../../../composables/useI18n.js'
import { BUILTIN_AGENT_CARDS } from '../composables/agentPresets.js'
import { useAgentsStore } from '../composables/useAgents.js'
import { AGREEMENT_TITLE, AGREEMENT_SECTIONS } from '../composables/userAgreement.js'
const { setLocale } = useI18n()

const props = defineProps({
  items: { type: Array, default: () => [] },
})
const emit = defineEmits(['close'])

const router = useRouter()

const STEPS = ['setup', 'nav']
const step = ref(0)
const agreed = ref(!!localStorage.getItem('studio_api_agreed'))
// 「不再显示」默认勾上：走完一次引导即静默，想再看回来取消勾选。
const dontShow = ref(true)
const showAgreement = ref(false)
// 开机自启：引导里默认勾选，用户确认过才落盘，避免后台静默写注册表（2026-09-12，issue #17 杀软误报）。
const autoStart = ref(true)

const LANGS = [
  { id: 'zh', name: tr('简体中文'), tag: tr('默认') },
  { id: 'en', name: 'English', tag: 'Beta' },
]
const pickedLang = ref(localStorage.getItem('ameko_locale_v1') === 'en' ? 'en' : 'zh')
function pickLang(id) {
  pickedLang.value = id
  setLocale(id)
}

const agentStore = useAgentsStore()
const presets = ref(BUILTIN_AGENT_CARDS)
// 选中的内置角色卡 id（只做回显）。内置卡在 useAgents 里已播种成真实角色卡，
// 这里选中即把对应那张设为当前 Agent；后端没起时静默跳过，之后在设置里补。
const pickedPersona = ref('')
async function pickPersona(p) {
  pickedPersona.value = p.id
  try {
    if (!agentStore.agentsLoaded.value) await agentStore.loadAgents()
    const hit = agentStore.agents.value.find((a) => a.id === p.id || a.name === p.name)
    if (hit) agentStore.selectAgent(hit.id)
  } catch { /* 后端没起：留着在设置 → 角色卡 里建 */ }
}

function readAndAgree() {
  agreed.value = true
  localStorage.setItem('studio_api_agreed', '1')
  showAgreement.value = false
}

const navigableItems = computed(() => props.items.filter((item) => item.to))
const desc = {
  chat: tr('对话、写代码、跑终端'),
  company: tr('Agent 团队实时状态与交付'),
  sites: tr('部署与查看自己的站点'),
  publish: tr('网文写作与一键发布'),
  comic: tr('漫画分镜与生成'),
  game: tr('星迹：联机副本与交易行'),
  studio: tr('AI 短剧与视频剪辑'),
}

function go(item) {
  if (!agreed.value) return
  finish()
  router.push(item.to)
}
function finish() {
  if (agreed.value) localStorage.setItem('studio_api_agreed', '1')
  // 引导里确认的自启意愿落盘（后端写 HKCU Run 注册表；失败静默，用户可在设置里再改）。
  try {
    fetch('/api/desktop/autostart', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enabled: autoStart.value }),
    })
  } catch { /* 后端没起：保持出厂默认，设置页仍可调 */ }
  emit('close', { dontShow: dontShow.value })
}
</script>

<style scoped>
.ob-mask {
  position: fixed; inset: 0; z-index: 10002; display: flex; align-items: center; justify-content: center;
  padding: 20px; background: rgba(15, 23, 42, 0.32);
}
.ob-card {
  position: relative; display: flex; flex-direction: column; width: min(700px, 100%);
  max-height: calc(100vh - 40px); padding: 16px 18px 14px;
  border: 1px solid var(--app-border); border-radius: 18px; background: var(--app-surface);
  color: var(--app-text); box-shadow: 0 24px 60px rgba(15, 23, 42, 0.24);
}
.ob-steps { display: flex; gap: 6px; margin-bottom: 12px; }
.ob-dot { width: 26px; height: 4px; border-radius: 999px; background: var(--app-surface-3); }
.ob-dot.done { background: color-mix(in srgb, var(--app-accent) 45%, transparent); }
.ob-dot.on { background: var(--app-accent); }
.ob-body { display: flex; flex-direction: column; overflow: auto; }
.ob-body h2 { margin: 0; font-size: 17px; font-weight: 600; }
.ob-sub { margin: 6px 0 12px; color: var(--app-text-faint); font-size: 12px; line-height: 1.6; }
.ob-lang-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; margin-bottom: 12px; }
.ob-lang {
  display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 11px 14px;
  border: 1px solid var(--app-border); border-radius: 13px; color: var(--app-text);
  background: var(--app-surface-2); font: inherit; cursor: pointer;
}
.ob-lang:hover, .ob-lang.on { border-color: var(--app-accent); }
.ob-lang.on { background: var(--app-surface-3); }
.ob-lang-name { font-size: 13.5px; font-weight: 600; }
.ob-lang-tag { color: var(--app-text-faint); font-size: 11px; }
.ob-persona-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; }
.ob-persona {
  display: flex; align-items: center; gap: 10px; min-height: 52px; padding: 10px 12px;
  border: 1px solid var(--app-border); border-radius: 13px; color: var(--app-text);
  background: var(--app-surface-2); font: inherit; text-align: left; cursor: pointer;
}
.ob-persona:hover, .ob-persona.on { border-color: var(--app-accent); }
.ob-persona.on { background: var(--app-surface-3); }
.ob-persona-icon {
  display: grid; width: 32px; height: 32px; flex: none; place-items: center; border-radius: 10px;
  color: var(--app-accent); background: color-mix(in srgb, var(--app-accent) 12%, transparent);
}
.ob-persona-body { display: flex; flex-direction: column; gap: 3px; min-width: 0; }
.ob-persona-name { font-size: 13px; font-weight: 600; }
.ob-persona-desc { color: var(--app-text-faint); font-size: 11.5px; line-height: 1.4; }
.ob-autostart {
  display: flex; align-items: flex-start; gap: 8px; margin-top: 12px; padding: 10px 12px;
  border: 1px solid var(--app-border); border-radius: 13px; background: var(--app-surface-2);
  cursor: pointer; color: var(--app-text);
}
.ob-autostart:hover { border-color: var(--app-accent); }
.ob-autostart input { margin-top: 2px; accent-color: var(--app-accent); }
.ob-autostart-body { display: flex; flex-direction: column; gap: 2px; }
.ob-autostart-title { font-size: 13px; font-weight: 600; }
.ob-autostart-desc { color: var(--app-text-faint); font-size: 11.5px; line-height: 1.4; }
.ob-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; }
.ob-item {
  display: flex; align-items: center; gap: 10px; min-height: 52px; padding: 10px 12px;
  border: 1px solid var(--app-border); border-radius: 13px; color: var(--app-text);
  background: var(--app-surface-2); font: inherit; text-align: left; cursor: pointer;
  transition: border-color .15s ease, background .15s ease;
}
.ob-item:hover:not(:disabled) { border-color: var(--app-accent); background: var(--app-surface-3); }
.ob-item:disabled { opacity: .5; cursor: not-allowed; }
.ob-item-icon {
  display: grid; width: 32px; height: 32px; flex: none; place-items: center; border-radius: 10px;
  color: var(--app-accent); background: color-mix(in srgb, var(--app-accent) 12%, transparent);
}
.ob-item-body { display: flex; flex-direction: column; gap: 3px; min-width: 0; }
.ob-item-label { font-size: 13px; font-weight: 600; }
.ob-item-desc { color: var(--app-text-faint); font-size: 11.5px; line-height: 1.4; }
.ob-item-arrow { margin-left: auto; color: var(--app-text-faint); font-size: 18px; }
.ob-foot {
  display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap;
  margin-top: 14px; padding-top: 12px; border-top: 1px solid var(--app-border);
}
.ob-agree-row { display: flex; align-items: center; gap: 4px; flex-wrap: wrap; }
.ob-check { display: inline-flex; align-items: center; gap: 6px; color: var(--app-text-soft); font-size: 12px; cursor: pointer; }
.ob-check input { accent-color: var(--app-accent); }
.ob-check-right { margin-left: 12px; }
.ob-link {
  padding: 0; border: none; background: none; color: var(--app-accent); font: inherit; font-size: 12px;
  cursor: pointer; text-decoration: underline; text-underline-offset: 2px;
}
.ob-link:hover { filter: brightness(1.1); }
.ob-foot-actions { display: flex; gap: 8px; }
.ob-btn {
  padding: 7px 14px; border: 1px solid var(--app-border); border-radius: 9px; color: var(--app-text-soft);
  background: var(--app-surface); font: inherit; font-size: 12.5px; cursor: pointer;
}
.ob-btn:hover { color: var(--app-accent); border-color: var(--app-accent); }
.ob-btn-primary { color: #fff; border-color: var(--app-accent); background: var(--app-accent); font-weight: 600; }
.ob-btn-primary:hover { color: #fff; filter: brightness(1.06); }
.ob-btn-primary:disabled { opacity: .45; cursor: not-allowed; filter: none; }
.ob-doc {
  position: absolute; inset: 0; display: flex; flex-direction: column; padding: 16px 18px 14px;
  border-radius: 18px; background: var(--app-surface);
}
.ob-doc-head { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.ob-doc-head strong { font-size: 15px; }
.ob-doc-close {
  display: grid; width: 30px; height: 30px; flex: none; place-items: center; padding: 0; border: 0;
  border-radius: 9px; color: var(--app-text-faint); background: transparent; cursor: pointer;
}
.ob-doc-close:hover { color: var(--app-text); background: var(--app-surface-3); }
.ob-doc-body { flex: 1; overflow: auto; margin-top: 10px; }
.ob-doc-sec { margin-bottom: 14px; }
.ob-doc-sec h3 { margin: 0 0 4px; font-size: 13px; font-weight: 600; }
.ob-doc-sec p { margin: 0; color: var(--app-text-soft); font-size: 12.5px; line-height: 1.8; }
.ob-doc-foot { display: flex; justify-content: flex-end; margin-top: 12px; padding-top: 12px; border-top: 1px solid var(--app-border); }
@media (max-width: 620px) {
  .ob-grid, .ob-lang-grid, .ob-persona-grid { grid-template-columns: 1fr; }
  .ob-card { padding: 14px; }
}
</style>
