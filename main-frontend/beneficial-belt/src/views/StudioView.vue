<template>
  <div class="studio-shell">
    <!-- 顶部栏 -->
    <header class="studio-header">
      <div class="studio-title">
        <span class="studio-logo">🎬</span>
        <h1>创作工作台 · 文案成片</h1>
        <span class="studio-sub">贴文案 → 自动配音 + 字幕 + 联网配素材 → 一键成片</span>
      </div>
      <router-link to="/" class="studio-back">← 返回对话</router-link>
    </header>

    <main class="studio-main">
      <!-- 左栏：输入 -->
      <section class="studio-panel input-panel">
        <div class="panel-head">
          <span>① 写文案</span>
          <span class="panel-hint">每句自动一段，换行或 | 分隔</span>
        </div>

        <div class="field-row">
          <input v-model="topic" class="studio-input topic-input" placeholder="主题（必填）" />
          <select v-model="voice" class="studio-input voice-select" title="配音音色">
            <option value="zh-TW-HsiaoChenNeural">曉臻 · 台湾普通话女声（默认）</option>
            <option value="openai:nova">Nova · OpenAI女声</option>
            <option value="zh-CN-XiaoxiaoNeural">晓晓 · edge女声（免费）</option>
            <option value="zh-CN-XiaoyiNeural">晓伊 · edge女声（免费）</option>
            <option value="zh-CN-YunxiNeural">云希 · 青年男声</option>
          </select>
          <select v-model="orientation" class="studio-input orient-select" title="画面方向">
            <option value="landscape">横屏 16:9</option>
            <option value="portrait">竖屏 9:16</option>
          </select>
        </div>

        <div class="field-row api-row">
          <input v-model="pexelsKey" class="studio-input api-input" placeholder="Pexels API Key（有 key 时自动搜真实视频素材）" />
          <button class="studio-input api-save-btn" @click="saveKey">保存</button>
          <span v-if="keySaved" class="api-saved">✓</span>
        </div>

        <textarea
          v-model="text"
          class="studio-input script-input"
          placeholder="粘贴你的文案，例如：&#10;曼波曼波，跳舞啦！&#10;音乐响起，节奏飞起！&#10;跟我一起摇起来！曼波曼波~&#10;&#10;留空则按主题自动生成曼波模板文案"
        ></textarea>

        <div class="gen-row">
          <label class="compose-toggle">
            <input type="checkbox" v-model="compose" :disabled="busy" />
            <span>合成完整视频（配音+拼接）</span>
          </label>
          <button class="studio-btn gen-btn" :disabled="busy" @click="generate">
            <span v-if="!busy">⚡ {{ compose ? '一键生成完整视频' : '生成素材片段' }}</span>
            <span v-else class="gen-busy">🎬 生成中…（约 10-20 分钟）</span>
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

      <!-- 右栏：产物 -->
      <section class="studio-panel result-panel">
        <div class="panel-head">
          <span>② 预览 · 剪辑 · 导出</span>
          <span v-if="result" class="panel-hint">{{ fmtDur(result.duration) }} · {{ result.segments.length }} 段</span>
        </div>

        <div v-if="!result" class="empty-state">
          <div class="empty-art">🎬</div>
          <p>左侧贴好文案，点「一键生成视频」<br />自动配音 + 字幕 + 联网搜素材拼接</p>
        </div>

        <template v-else>
          <!-- 视频预览 -->
          <div class="video-wrap">
            <video :src="result.video" controls playsinline class="studio-video"></video>
          </div>

          <!-- 段落剪辑时间轴 -->
          <div class="timeline-head">
            <span>段落剪辑（删段 / 换序 → 重新生成生效）</span>
          </div>
          <div class="timeline">
            <div v-for="(seg, i) in segs" :key="i" class="timeline-seg" :style="{ width: segW(seg) }">
              <div class="seg-top">
                <span class="seg-idx">{{ i + 1 }}</span>
                <span class="seg-dur">{{ seg.duration.toFixed(1) }}s</span>
              </div>
              <div class="seg-text" :title="seg.sentence">{{ seg.sentence }}</div>
              <div class="seg-topic" v-if="seg.topic" :title="'素材搜索词: ' + ((seg.search_terms || []).join(' / '))">🎯 {{ seg.topic }}</div>
              <div class="seg-src" :title="seg.source">{{ srcShort(seg.source) }}</div>
              <div class="seg-ops">
                <button class="seg-btn" :disabled="i === 0" @click="move(i, -1)" title="上移">↑</button>
                <button class="seg-btn" :disabled="i === segs.length - 1" @click="move(i, 1)" title="下移">↓</button>
                <button class="seg-btn del" @click="remove(i)" title="删除该段">✕</button>
              </div>
            </div>
          </div>

          <div class="export-row">
            <button class="studio-btn regen-btn" :disabled="busy" @click="regenerate">
              {{ busy ? '⏳ 重新生成中…' : '🔄 按剪辑重新生成（导出）' }}
            </button>
            <a v-if="result.videoPath" class="export-path" :href="result.video" download target="_blank">
              📁 {{ result.videoPath }}
            </a>
          </div>
        </template>
      </section>
    </main>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { API_BASE_URL } from '../config.js'

const topic = ref('')
const voice = ref('zh-TW-HsiaoChenNeural')  // 曉臻默认
const text = ref('')
const busy = ref(false)
const logLines = ref([])
const result = ref(null)
const segs = ref([])
const pexelsKey = ref(localStorage.getItem('pexels_key') || '')
const keySaved = ref(false)
const orientation = ref('landscape')
const compose = ref(false)
const transOpen = ref(false)
const transLang = ref('en')
const transBusy = ref(false)
const transResult = ref('')
const transError = ref('')

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
  const payload = {
    topic: t, text: text.value.trim(), voice: voice.value,
    pexels_key: pexelsKey.value.trim(),
    orientation: orientation.value,
    compose: compose.value,
  }
  await runGen(payload)
}

async function regenerate() {
  if (!result.value || !segs.value.length) return
  // 剪辑 = 保留段的文案按顺序拼回，引擎重新 TTS + 重新配素材（保证音画字对齐）
  const newText = segs.value.map(s => s.sentence).join('|')
  await runGen({
    topic: topic.value.trim() || '曼波',
    text: newText,
    voice: voice.value,
    out: result.value.videoPath.split(/[\\/]/).pop().replace(/\.mp4$/, '')
  }, true)
}

async function runGen(payload, isRegen = false) {
  busy.value = true
  logLines.value = []
  pushLog('🎬 开始生成（约 10-20 分钟）')
  try {
    const resp = await fetch(`${API_BASE_URL}/api/studio/mambo`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ...payload, pexels_key: pexelsKey.value.trim() })
    })
    const reader = resp.body.getReader()
    const decoder = new TextDecoder()
    let buf = ''
    let resultData = null
    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buf += decoder.decode(value, { stream: true })
      const events = buf.split('\n\n')
      buf = events.pop() || ''
      for (const evt of events) {
        const lines = evt.split('\n')
        const eventType = lines.find(l => l.startsWith('event:'))?.slice(6).trim() || ''
        const dataLine = lines.find(l => l.startsWith('data:'))?.slice(5).trim() || ''
        if (eventType === 'result') {
          try { resultData = JSON.parse(dataLine) } catch {}
        } else if (eventType === 'progress') {
          pushLog(dataLine)
        } else if (eventType === 'error') {
          pushLog('✗ ' + dataLine)
        } else if (dataLine) {
          pushLog(dataLine)
        }
      }
    }
    if (resultData?.ok) {
      result.value = {
        video: resultData.video || resultData.videoPath || '',
        videoPath: resultData.videoPath || resultData.video || '',
        duration: resultData.duration || 0,
        segments: resultData.segments || []
      }
      segs.value = (resultData.segments || []).map(s => ({ ...s }))
      pushLog(`✅ 完成：${resultData.duration || 0}s`)
    } else {
      pushLog('✗ 生成失败')
    }
  } catch (e) {
    pushLog('✗ 请求失败：' + e.message)
  } finally {
    busy.value = false
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

onMounted(() => { document.title = '创作工作台 · 文案成片' })
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
</style>
