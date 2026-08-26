<template>
  <div class="studio-shell">
    <!-- 顶部栏 -->
    <header class="studio-header">
      <div class="studio-title">
        <span class="studio-logo">🎬</span>
        <h1>创作工作台 · 漫剧</h1>
        <span class="studio-sub">分镜脚本 → 多平台生成(即梦/Kling/海螺) → 自动剪辑 → 成片</span>
      </div>
      <router-link to="/" class="studio-back">← 返回对话</router-link>
    </header>

    <main class="studio-main">
      <!-- 左栏：输入 -->
      <section class="studio-panel input-panel">
        <div class="panel-head">
          <span>① 分镜剧本</span>
          <span class="panel-hint">每句一个分镜，换行或 | 分隔</span>
        </div>

        <div class="field-row">
          <input v-model="topic" class="studio-input topic-input" placeholder="主题（必填，如：AI 少女的日常）" />
          <select v-model="voice" class="studio-input voice-select" title="配音音色">
            <option value="zh-TW-HsiaoChenNeural">曉臻 · 台湾普通话女声（默认）</option>
            <option value="openai:nova">Nova · OpenAI女声</option>
            <option value="zh-CN-XiaoxiaoNeural">晓晓 · edge女声（免费）</option>
            <option value="zh-CN-XiaoyiNeural">晓伊 · edge女声（免费）</option>
            <option value="zh-CN-YunxiNeural">云希 · 青年男声</option>
          </select>
        </div>

        <div class="field-row">
          <select v-model="orientation" class="studio-input orient-select" title="画面方向">
            <option value="landscape">横屏 16:9</option>
            <option value="portrait">竖屏 9:16</option>
          </select>
          <select v-model="genPlatform" class="studio-input orient-select" title="生成平台">
            <option value="auto">自动分配（即梦/Kling/海螺）</option>
            <option value="jimeng">即梦 Fast VIP（每日免费1条）</option>
            <option value="kling">Kling 国际版（66分/日）</option>
            <option value="hailuo">海螺 MiniMax（新户300分）</option>
          </select>
        </div>

        <div class="credit-row">
          <span class="credit-badge">🔹 即梦 55分</span>
          <span class="credit-badge">🔹 Kling 免登</span>
          <span class="credit-badge">🔹 海螺 300分</span>
        </div>

        <div class="keys-panel">
          <div class="panel-head" style="cursor:pointer" @click="keysOpen = !keysOpen">
            <span>🔑 生成平台 API Key</span>
            <span class="panel-hint">{{ keysOpen ? '收起' : (platformKeyCount + ' 已填') }}</span>
          </div>
          <div v-if="keysOpen" class="keys-body">
            <div class="field-row" v-for="k in platformKeys" :key="k.id">
              <span class="key-label">{{ k.label }}</span>
              <input
                :type="k.show ? 'text' : 'password'"
                v-model="platformKeysMap[k.id]"
                class="studio-input key-input"
                :placeholder="k.placeholder"
              />
              <button class="key-eye" @click="k.show = !k.show" :title="k.show ? '隐藏' : '显示'">{{ k.show ? '🙈' : '👁️' }}</button>
            </div>
            <div class="keys-hint">Key 只存本地浏览器，不会上传。留空则对应平台不参与生成。</div>
          </div>
        </div>

        <textarea
          v-model="text"
          class="studio-input script-input"
          placeholder="粘贴分镜剧本，每行一个镜头：&#10;少女在樱花树下回眸，柔光&#10;她伸手接住飘落的花瓣&#10;背景切换为城市夜景，霓虹灯闪烁&#10;&#10;留空则按主题自动生成分镜"
        ></textarea>

        <div class="gen-row">
          <label class="compose-toggle">
            <input type="checkbox" v-model="compose" :disabled="busy" />
            <span>合成完整视频（配音+拼接）</span>
          </label>
          <button class="studio-btn gen-btn" :disabled="busy" @click="generate">
            <span v-if="!busy">⚡ 生成分镜计划</span>
            <span v-else class="gen-busy">🧠 分镜生成中…</span>
          </button>
        </div>

        <div v-if="logLines.length" class="gen-log">
          <div v-for="(l, i) in logLines" :key="i" class="log-line" :class="{ err: l.startsWith('✗') }">{{ l }}</div>
        </div>

        <!-- 翻译面板 -->
        <div class="trans-panel">
          <div class="panel-head" style="cursor:pointer" @click="transOpen = !transOpen">
            <span>③ 翻译文章（英/日/韩）</span>
            <span class="panel-hint">{{ transOpen ? '收起' : '展开' }}</span>
          </div>
          <div v-if="transOpen" class="trans-body">
            <div class="field-row">
              <select v-model="transLang" class="studio-input voice-select">
                <option value="en">English</option>
                <option value="ja">日本語</option>
                <option value="ko">한국어</option>
              </select>
              <button class="studio-btn gen-btn" :disabled="transBusy" @click="doTranslate" style="flex:0 0 auto;padding:10px 20px">
                {{ transBusy ? '翻译中…' : '翻译' }}
              </button>
            </div>
            <textarea v-if="transResult" class="studio-input script-input" :value="transResult" readonly style="min-height:120px;margin-top:8px"></textarea>
            <div v-if="transError" class="log-line err">{{ transError }}</div>
          </div>
        </div>
      </section>

      <!-- 右栏：分镜计划 / 产物 -->
      <section class="studio-panel result-panel">
        <div class="panel-head">
          <span>② 分镜计划 · 人设 · 生成</span>
          <span v-if="plan" class="panel-hint">{{ plan.characters?.length }} 角色 · {{ plan.shots?.length }} 镜头 · 约 {{ plan.total_dur }}s</span>
        </div>

        <div v-if="!plan" class="empty-state">
          <div class="empty-art">🎬</div>
          <p>左侧写好剧情/主题，点「生成分镜计划」<br />LLM 自动拆镜头 + 人设卡，再逐镜生成</p>
        </div>

        <template v-else>
          <!-- 人设卡 -->
          <div class="manga-sec">
            <div class="timeline-head">👤 人设卡（角色一致性锚点）</div>
            <div class="char-grid">
              <div v-for="ch in plan.characters" :key="ch.id" class="char-card">
                <div class="char-head">
                  <span class="char-name">{{ ch.name }}</span>
                  <span class="char-role">{{ ch.role }}</span>
                </div>
                <div class="char-line">👀 {{ ch.appearance }}</div>
                <div class="char-line char-personality">💬 {{ ch.personality }}</div>
                <div class="char-prompt" :title="ch.ref_prompt">{{ ch.ref_prompt }}</div>
              </div>
            </div>
          </div>

          <!-- 分镜列表 -->
          <div class="manga-sec">
            <div class="timeline-head">🎞️ 分镜（点击生成 → 多平台调度）</div>
            <div class="shot-list">
              <div v-for="(shot, i) in plan.shots" :key="shot.shot_no" class="shot-card" :class="{ generating: shot.status === 'busy', done: shot.status === 'done' }">
                <div class="shot-top">
                  <span class="shot-no">{{ shot.shot_no }}</span>
                  <span class="shot-dur">{{ shot.duration }}s</span>
                  <span class="shot-platform">{{ shot.platform }}</span>
                  <span v-if="shot.status === 'done'" class="shot-done">✅</span>
                  <span v-else-if="shot.status === 'busy'" class="shot-done">⏳</span>
                </div>
                <div class="shot-scene">🏞️ {{ shot.scene }}</div>
                <div class="shot-action">🎥 {{ shot.action }}</div>
                <div class="shot-dialogue" v-if="shot.dialogue">💬 {{ shot.dialogue }}</div>
                <button class="shot-gen-btn" :disabled="shot.status === 'busy'" @click="genShot(i)">
                  {{ shot.status === 'done' ? '重新生成' : (shot.status === 'busy' ? '生成中…' : '⚡ 生成镜头') }}
                </button>
              </div>
            </div>
          </div>

          <div class="export-row" v-if="result">
            <a class="export-path" :href="result.video" download target="_blank">
              📁 {{ result.videoPath }}
            </a>
          </div>
        </template>
      </section>
    </main>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { API_BASE_URL } from '../config.js'

const topic = ref('')
const voice = ref('zh-TW-HsiaoChenNeural')  // 曉臻默认
const text = ref('')
const busy = ref(false)
const logLines = ref([])
const result = ref(null)
const plan = ref(null)
const segs = ref([])
const pexelsKey = ref(localStorage.getItem('pexels_key') || '')
const keySaved = ref(false)
const orientation = ref('landscape')
const genPlatform = ref('auto')
const compose = ref(false)
const transOpen = ref(false)
const transLang = ref('en')
const transBusy = ref(false)
const transResult = ref('')
const transError = ref('')

// ---- 生成平台 API Key（localStorage 持久化）----
const KEY_STORE = 'studio_platform_keys'
const platformKeys = ref([
  { id: 'jimeng', label: '即梦', placeholder: '即梦 API Key / Cookie', show: false },
  { id: 'hailuo', label: '海螺 MiniMax', placeholder: '海螺 API Key', show: false },
  { id: 'agnes', label: 'Agnes', placeholder: 'Agnes API Key', show: false },
  { id: 'kling', label: 'Kling', placeholder: 'Kling API Key（留空则每日白嫖）', show: false },
])
const keysOpen = ref(false)
const platformKeysMap = ref(loadPlatformKeys())
const platformKeyCount = computed(() => Object.values(platformKeysMap.value).filter(v => v && v.trim()).length)

function loadPlatformKeys() {
  try {
    const raw = localStorage.getItem(KEY_STORE)
    if (raw) return JSON.parse(raw)
  } catch (e) {}
  return {}
}
// 监听变化持久化
watch(platformKeysMap, (v) => {
  localStorage.setItem(KEY_STORE, JSON.stringify(v))
}, { deep: true })

function saveKey() {
  localStorage.setItem('pexels_key', pexelsKey.value.trim())
  keySaved.value = true
  setTimeout(() => keySaved.value = false, 2000)
}

const SEG_COLORS = ['#ff6b6b', '#4ecdc4', '#ffd93d', '#6c5ce7', '#00b894', '#fd79a8', '#74b9ff', '#e17055']

function segW(seg) {
  const total = result.value?.duration || 1
  return Math.max(8, (seg.duration / total) * 100) + '%'
}
function srcShort(src) {
  if (!src) return ''
  const s = String(src)
  if (s.includes('Bing')) return '🌐 联网素材'
  if (s.includes('素材池')) return '📁 本地素材'
  return '🎨 动态背景'
}
function fmtDur(d) {
  const m = Math.floor(d / 60), s = Math.round(d % 60)
  return `${m}:${String(s).padStart(2, '0')}`
}
function pushLog(l) { logLines.value.push(l) }

async function doTranslate() {
  const src = text.value.trim()
  if (!src) { transError.value = '✗ 先在上方贴入要翻译的文章'; return }
  transBusy.value = true
  transError.value = ''
  transResult.value = ''
  try {
    const resp = await fetch(`${API_BASE_URL}/api/translate`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ text: src, target_lang: transLang.value })
    })
    const data = await resp.json()
    if (!resp.ok || !data.ok) {
      transError.value = '✗ ' + (data.error || resp.status)
      return
    }
    transResult.value = data.translated
  } catch (e) {
    transError.value = '✗ 请求失败：' + e.message
  } finally {
    transBusy.value = false
  }
}

async function generate() {
  const t = topic.value.trim()
  if (!t) { pushLog('✗ 先填主题'); return }
  busy.value = true
  plan.value = null
  result.value = null
  logLines.value = []
  pushLog('🧠 LLM 生成分镜计划 + 人设卡…')
  try {
    const resp = await fetch(`${API_BASE_URL}/api/studio/manga/plan`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ topic: t, script: text.value.trim(), genre: '' })
    })
    const data = await resp.json()
    if (!resp.ok || !data.ok) {
      pushLog('✗ ' + (data.error || resp.status))
      return
    }
    plan.value = data.plan
    // 给每个镜头加前端状态
    plan.value.shots.forEach(s => s.status = '')
    pushLog(`✅ 分镜计划完成：${plan.value.characters.length} 角色 · ${plan.value.shots.length} 镜头 · 约 ${plan.value.total_dur}s`)
  } catch (e) {
    pushLog('✗ 请求失败：' + e.message)
  } finally {
    busy.value = false
  }
}

async function genShot(i) {
  const shot = plan.value.shots[i]
  if (!shot || shot.status === 'busy') return
  shot.status = 'busy'
  pushLog(`🎥 生成镜头 ${shot.shot_no}（${shot.platform}）…`)
  try {
    const resp = await fetch(`${API_BASE_URL}/api/studio/manga/shot`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        shot_no: shot.shot_no,
        prompt: shot.prompt,
        platform: genPlatform.value === 'auto' ? shot.platform : genPlatform.value,
        ref_image: ''
      })
    })
    const data = await resp.json()
    if (resp.ok && data.ok) {
      shot.status = 'done'
      pushLog(`✅ 镜头 ${shot.shot_no} 已提交生成`)
    } else {
      shot.status = ''
      pushLog('✗ ' + (data.error || '生成失败'))
    }
  } catch (e) {
    shot.status = ''
    pushLog('✗ 请求失败：' + e.message)
  }
}

function move(i, dir) {
  const j = i + dir
  if (j < 0 || j >= segs.value.length) return
  const arr = segs.value
  ;[arr[i], arr[j]] = [arr[j], arr[i]]
}
function remove(i) {
  segs.value.splice(i, 1)
}

onMounted(() => { document.title = '创作工作台 · 漫剧' })
</script>

<style scoped>
.studio-shell {
  min-height: 100vh;
  background: var(--app-bg);
  color: var(--app-text);
  display: flex;
  flex-direction: column;
}
.studio-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 18px 28px;
  border-bottom: 1px solid var(--app-border-soft);
  background: var(--app-surface);
}
.studio-title { display: flex; align-items: baseline; gap: 10px; }
.studio-logo { font-size: 22px; }
.studio-title h1 { font-size: 19px; font-weight: 700; margin: 0; }
.studio-sub { font-size: 12.5px; color: var(--app-text-faint); }
.studio-back {
  color: var(--app-accent); text-decoration: none; font-size: 13.5px; font-weight: 600;
}
.studio-main {
  flex: 1; display: grid; grid-template-columns: 420px 1fr; gap: 18px;
  padding: 20px 28px; max-width: 1500px; width: 100%; margin: 0 auto; box-sizing: border-box;
}
.studio-panel {
  background: var(--app-surface);
  border: 1px solid var(--app-border-soft);
  border-radius: 14px;
  padding: 16px;
  display: flex; flex-direction: column; gap: 12px;
  min-height: 0;
}
.panel-head { display: flex; justify-content: space-between; align-items: baseline; }
.panel-head > span:first-child { font-weight: 700; font-size: 14.5px; }
.panel-hint { font-size: 12px; color: var(--app-text-faint); }

.studio-input {
  width: 100%; box-sizing: border-box;
  background: var(--app-bg);
  color: var(--app-text);
  border: 1px solid var(--app-border);
  border-radius: 9px;
  padding: 10px 12px;
  font-size: 13.5px;
  font-family: var(--app-font);
  outline: none;
}
.studio-input:focus { border-color: var(--app-accent); }
.field-row { display: flex; gap: 10px; }
.topic-input { flex: 1; }
.voice-select { flex: 0 0 190px; }
.script-input {
  flex: 1; min-height: 240px; resize: vertical; line-height: 1.7;
}

.gen-row { display: flex; }
.credit-row { display: flex; gap: 8px; flex-wrap: wrap; }
.credit-badge {
  font-size: 11.5px; font-weight: 600;
  background: var(--app-accent-soft, #2dd4bf22);
  color: var(--app-accent);
  border: 1px solid var(--app-border);
  border-radius: 20px;
  padding: 4px 12px;
}
.studio-btn {
  flex: 1;
  border: none; border-radius: 10px;
  padding: 12px 16px;
  font-size: 14.5px; font-weight: 700;
  cursor: pointer;
  transition: transform .12s, opacity .2s;
}
.studio-btn:disabled { opacity: .55; cursor: not-allowed; }
.gen-btn { background: var(--app-accent); color: #fff; }
.gen-btn:not(:disabled):hover { transform: translateY(-1px); }
.gen-busy { display: inline-block; animation: pulse 1.2s infinite; }
@keyframes pulse { 50% { opacity: .55; } }

.gen-log {
  background: var(--app-bg);
  border-radius: 9px; padding: 10px 12px;
  max-height: 130px; overflow-y: auto;
  font-size: 12px; font-family: ui-monospace, Consolas, monospace;
  display: flex; flex-direction: column; gap: 3px;
}
.log-line { color: var(--app-text-soft); word-break: break-all; }
.log-line.err { color: #ff6b6b; }

.empty-state {
  flex: 1; display: flex; flex-direction: column;
  align-items: center; justify-content: center; gap: 12px;
  color: var(--app-text-faint); text-align: center; line-height: 1.8;
}
.empty-art { font-size: 52px; }

.video-wrap {
  background: #000; border-radius: 10px; overflow: hidden;
  display: flex; justify-content: center;
  max-height: 46vh;
}
.studio-video { height: 100%; max-height: 46vh; width: auto; }

.timeline-head { font-size: 13px; font-weight: 600; margin-top: 4px; }
.timeline {
  display: flex; gap: 6px; align-items: stretch;
  overflow-x: auto; padding: 8px 2px 12px;
}
.timeline-seg {
  min-width: 150px; flex: 0 0 auto;
  background: var(--app-bg);
  border: 1px solid var(--app-border);
  border-left: 4px solid #888;
  border-radius: 8px;
  padding: 8px 10px;
  display: flex; flex-direction: column; gap: 5px;
}
.seg-top { display: flex; justify-content: space-between; font-size: 11px; color: var(--app-text-faint); }
.seg-idx { font-weight: 700; }
.seg-text {
  font-size: 12.5px; font-weight: 600; line-height: 1.4;
  display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden;
}
.seg-topic {
  font-size: 11px; font-weight: 700; color: var(--app-accent);
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}
.seg-src { font-size: 11px; color: var(--app-text-faint); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.seg-ops { display: flex; gap: 4px; margin-top: 2px; }
.seg-btn {
  border: 1px solid var(--app-border); background: var(--app-surface);
  color: var(--app-text); border-radius: 6px;
  width: 26px; height: 24px; font-size: 12px; cursor: pointer;
}
.seg-btn:disabled { opacity: .35; cursor: not-allowed; }
.seg-btn.del:hover { border-color: #ff6b6b; color: #ff6b6b; }

.export-row { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
.regen-btn { flex: 0 1 auto; background: var(--app-accent-soft, #2dd4bf22); color: var(--app-accent); border: 1px solid var(--app-accent); }
.export-path {
  font-size: 11.5px; color: var(--app-text-faint);
  text-decoration: none; word-break: break-all;
}
.export-path:hover { color: var(--app-accent); }

/* ---- 漫剧：人设卡 + 分镜列表 ---- */
.manga-sec { display: flex; flex-direction: column; gap: 8px; }
.char-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 10px; }
.char-card {
  background: var(--app-bg); border: 1px solid var(--app-border);
  border-left: 4px solid var(--app-accent); border-radius: 9px;
  padding: 10px 12px; display: flex; flex-direction: column; gap: 5px;
}
.char-head { display: flex; align-items: center; justify-content: space-between; }
.char-name { font-weight: 700; font-size: 13.5px; }
.char-role {
  font-size: 11px; font-weight: 700; color: var(--app-accent);
  background: var(--app-accent-soft, #2dd4bf22);
  border-radius: 20px; padding: 2px 9px;
}
.char-line { font-size: 12px; color: var(--app-text-soft); line-height: 1.5; }
.char-personality { color: var(--app-text-faint); }
.char-prompt {
  font-size: 10.5px; color: var(--app-text-faint); font-family: ui-monospace, Consolas, monospace;
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis; line-height: 1.6;
}
.shot-list { display: flex; flex-direction: column; gap: 8px; }
.shot-card {
  background: var(--app-bg); border: 1px solid var(--app-border);
  border-radius: 9px; padding: 10px 12px;
  display: flex; flex-direction: column; gap: 5px;
}
.shot-card.generating { border-color: var(--app-accent); opacity: .8; }
.shot-card.done { border-color: #12b76a55; }
.shot-top { display: flex; align-items: center; gap: 10px; font-size: 11.5px; }
.shot-no { font-weight: 800; color: var(--app-accent); }
.shot-dur { color: var(--app-text-faint); }
.shot-platform {
  color: var(--app-accent); border: 1px solid var(--app-border);
  border-radius: 20px; padding: 1px 8px; font-size: 10.5px;
}
.shot-done { margin-left: auto; }
.shot-scene { font-size: 12px; font-weight: 700; color: var(--app-text); }
.shot-action { font-size: 12px; color: var(--app-text-soft); line-height: 1.5; }
.shot-dialogue { font-size: 12px; color: var(--app-accent); font-style: italic; }
.shot-gen-btn {
  align-self: flex-end; margin-top: 2px;
  border: 1px solid var(--app-accent); background: var(--app-accent-soft, #2dd4bf22);
  color: var(--app-accent); border-radius: 8px; padding: 6px 14px;
  font-size: 12.5px; font-weight: 700; cursor: pointer;
}
.shot-gen-btn:disabled { opacity: .5; cursor: not-allowed; }
</style>
