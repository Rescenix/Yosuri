<template>
  <main class="publish-view">
    <header class="pv-header">
      <h1>📚 多平台一键发布</h1>
      <p class="sub">发布到网文平台 · 无头 Chrome 自动发布（登录态独立保存，不打扰日常浏览器）</p>
      <button class="pv-btn chrome-login" @click="openLoginChrome">🔑 打开登录 Chrome（第一次用）</button>
      <div v-if="loginMsg" class="pv-notice">{{ loginMsg }}</div>
    </header>

    <section class="pv-card">
      <label class="pv-label">上传文章文件（.md / .txt，自动解析标题+正文）</label>
      <input type="file" accept=".md,.txt,.markdown" class="pv-file" @change="onFileChange" />
      <div v-if="fileName" class="pv-filename">📄 {{ fileName }} · {{ content.length }} 字</div>

      <label class="pv-label">标题</label>
      <input v-model="title" class="pv-input" placeholder="文章标题" />

      <label class="pv-label">正文</label>
      <textarea v-model="content" class="pv-textarea" placeholder="粘贴正文内容…" rows="10"></textarea>

      <label class="pv-label">发布到（{{ picked.length }} 个）</label>
      <div class="pv-platforms">
        <button class="pv-chip ghost" @click="pickAll">全选</button>
        <button class="pv-chip ghost" @click="pickNone">取消</button>
        <button
          v-for="p in platforms"
          :key="p.id"
          class="pv-chip"
          :class="{ on: picked.includes(p.id) }"
          @click="toggle(p.id)"
          :title="p.notes"
        >
          {{ p.name }}
          <span v-if="p.minLen" class="pv-min">≥{{ p.minLen }}字</span>
        </button>
      </div>

      <div class="pv-actions">
        <button class="pv-btn" :disabled="busy || picked.length === 0" @click="publish">
          {{ busy ? '发布中…' : `一键发布（${picked.length} 平台）` }}
        </button>
        <button class="pv-btn ghost" @click="loadPlatforms">刷新平台</button>
      </div>

      <div v-if="results.length" class="pv-results">
        <div v-for="r in results" :key="r.platform" class="pv-result" :class="r.ok ? 'ok' : 'fail'">
          <span class="pv-result-icon">{{ r.ok ? '✅' : '❌' }}</span>
          <span class="pv-result-name">{{ r.name }}</span>
          <span class="pv-result-msg">{{ r.message || (r.ok ? '发布成功' : '失败') }}</span>
        </div>
      </div>

      <div v-if="notice" class="pv-notice">{{ notice }}</div>
    </section>
  </main>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'

const platforms = ref([])
const picked = ref([])
const title = ref('')
const content = ref('')
const results = ref([])
const busy = ref(false)
const notice = ref('')
const fileName = ref('')
const loginMsg = ref('')

// 打开发布专用 Chrome（登录一次，发布永久可用）
async function openLoginChrome() {
  loginMsg.value = ''
  try {
    const r = await fetch('/api/publish/login-chrome', { method: 'POST' })
    const d = await r.json()
    loginMsg.value = d.message || ''
  } catch (e) {
    loginMsg.value = '打开失败：' + e.message
  }
}

// 用户上传文件：自动解析标题（第一个 # 或文件名）+ 正文（发布时后端自动去 markdown）
function onFileChange(e) {
  const f = e.target.files && e.target.files[0]
  if (!f) return
  fileName.value = f.name
  const reader = new FileReader()
  reader.onload = () => {
    const text = String(reader.result || '')
    let t = ''
    for (const l of text.split('\n')) {
      if (l.startsWith('#') && !t) {
        t = l.replace(/^#+\s*/, '').trim()
        break
      }
    }
    title.value = t || f.name.replace(/\.(md|txt|markdown)$/i, '')
    content.value = text
  }
  reader.readAsText(f, 'utf-8')
}

async function loadPlatforms() {
  try {
    const r = await fetch('/api/publish/platforms')
    const d = await r.json()
    platforms.value = d.platforms || []
  } catch (e) {
    notice.value = '加载平台失败：' + e.message
  }
}

function toggle(id) {
  const i = picked.value.indexOf(id)
  if (i >= 0) picked.value.splice(i, 1)
  else picked.value.push(id)
}

function pickAll() {
  picked.value = platforms.value.map(p => p.id)
}

function pickNone() {
  picked.value = []
}

async function publish() {
  if (!title.value.trim() || !content.value.trim()) {
    notice.value = '请填写标题和正文'
    return
  }
  busy.value = true
  notice.value = ''
  results.value = []
  try {
    const r = await fetch('/api/publish', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title: title.value, content: content.value, platforms: picked.value }),
    })
    const d = await r.json()
    results.value = d.results || []
    const okCount = results.value.filter(x => x.ok).length
    notice.value = `发布完成：${okCount}/${results.value.length} 成功`
  } catch (e) {
    notice.value = '发布失败：' + e.message
  } finally {
    busy.value = false
  }
}

onMounted(() => {
  document.title = '杉汐 | 发布'
  loadPlatforms()
})
onUnmounted(() => {})
</script>

<style scoped>
.publish-view {
  max-width: 820px;
  margin: 0 auto;
  padding: 28px 20px 60px;
  font-family: -apple-system, 'PingFang SC', 'Microsoft YaHei', sans-serif;
}
.pv-header h1 { margin: 0 0 4px; font-size: 22px; }
.pv-sub, .sub { color: #888; font-size: 13px; margin: 0 0 20px; }
.pv-card { background: #fff; border: 1px solid #eee; border-radius: 12px; padding: 20px; }
.pv-label { display: block; font-size: 13px; color: #555; margin: 14px 0 6px; }
.pv-input, .pv-textarea, .pv-file {
  width: 100%; box-sizing: border-box; border: 1px solid #ddd; border-radius: 8px;
  padding: 10px 12px; font-size: 14px; font-family: inherit;
}
.pv-file { background: #fafafa; cursor: pointer; }
.pv-filename { margin-top: 6px; font-size: 13px; color: #666; }
.pv-input:focus, .pv-textarea:focus { outline: none; border-color: #4a9eff; }
.pv-textarea { resize: vertical; min-height: 160px; line-height: 1.6; }
.pv-platforms { display: flex; flex-wrap: wrap; gap: 8px; }
.pv-chip {
  border: 1px solid #ddd; background: #fafafa; color: #555;
  border-radius: 999px; padding: 6px 14px; font-size: 13px; cursor: pointer;
}
.pv-chip.on { background: #4a9eff; border-color: #4a9eff; color: #fff; }
.pv-min { font-size: 11px; opacity: 0.75; margin-left: 4px; }
.pv-actions { display: flex; gap: 10px; margin-top: 18px; }
.pv-btn {
  background: #4a9eff; color: #fff; border: none; border-radius: 8px;
  padding: 10px 22px; font-size: 14px; cursor: pointer;
}
.pv-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.pv-btn.ghost { background: #fff; color: #4a9eff; border: 1px solid #4a9eff; }
.pv-results { margin-top: 18px; border-top: 1px solid #eee; padding-top: 12px; }
.pv-result { display: flex; align-items: center; gap: 8px; padding: 6px 0; font-size: 14px; }
.pv-result.ok .pv-result-msg { color: #2e9e4f; }
.pv-result.fail .pv-result-msg { color: #d43; }
.pv-result-name { font-weight: 600; min-width: 90px; }
.pv-result-msg { color: #666; }
.pv-notice { margin-top: 14px; font-size: 14px; color: #4a9eff; }
</style>