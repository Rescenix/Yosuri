<template>
  <main class="ai-write-view">
    <header class="aw-header">
      <h1>🏢 AI 女儿们协作写小说</h1>
      <p class="sub">输入主题，公司里的 AI 女儿们同时开工：构思 · 查资料 · 落笔 · 发布</p>
    </header>

    <!-- 生成动画：多女儿协作 -->
    <section v-if="busy" class="aw-collab">
      <div class="aw-collab-title">她们正在协作…</div>
      <div class="aw-collab-grid">
        <div v-for="step in collabSteps" :key="step.name" class="aw-step" :class="{ active: step.active, done: step.done }">
          <span class="aw-step-emoji">{{ step.emoji }}</span>
          <span class="aw-step-name">{{ step.name }}</span>
          <span class="aw-step-text">{{ step.done ? step.result : (step.active ? step.doing : '待命') }}</span>
        </div>
      </div>
    </section>

    <section class="aw-card">
      <div class="aw-input-row">
        <input
          v-model="topic"
          class="aw-input"
          placeholder="输入小说主题，如：末世废土里的 AI 女孩"
          @keyup.enter="generate"
        />
        <button class="aw-btn" :disabled="busy" @click="generate">
          {{ busy ? '✍️ 她们在写…' : '✨ 让她们写' }}
        </button>
      </div>
      <div class="aw-type">
        <button class="aw-chip" :class="{ on: type === 'novel' }" @click="type = 'novel'">📖 小说</button>
        <button class="aw-chip" :class="{ on: type === 'article' }" @click="type = 'article'">📄 文章</button>
      </div>

      <div v-if="notice" class="aw-notice">{{ notice }}</div>

      <!-- 小说展示：封面卡 + 章节 -->
      <div v-if="article.type === 'novel' && article.title" class="aw-novel">
        <!-- AI 封面卡（SVG 动态生成，可直接截图发小红书） -->
        <div class="aw-cover-wrap">
          <svg class="aw-cover" :viewBox="'0 0 400 560'">
            <defs>
              <linearGradient :id="gradId" x1="0%" y1="0%" x2="100%" y2="100%">
                <stop offset="0%" :stop-color="grad[0]" />
                <stop offset="100%" :stop-color="grad[1]" />
              </linearGradient>
            </defs>
            <rect width="400" height="560" :fill="'url(#' + gradId + ')'" rx="12" />
            <!-- 装饰星星 -->
            <circle v-for="(s, i) in stars" :key="'s'+i" :cx="s.x" :cy="s.y" :r="s.r" fill="rgba(255,255,255,0.6)" />
            <text x="200" y="90" text-anchor="middle" font-size="14" fill="rgba(255,255,255,0.8)">AI 小说公司出品</text>
            <text x="200" y="150" text-anchor="middle" font-size="30" font-weight="bold" fill="#fff">{{ coverTitle }}</text>
            <text x="200" y="180" text-anchor="middle" font-size="14" fill="rgba(255,255,255,0.7)">第 {{ article.chapterNo }} 章</text>
            <line x1="80" y1="210" x2="320" y2="210" stroke="rgba(255,255,255,0.4)" />
            <text x="200" y="480" text-anchor="middle" font-size="12" fill="rgba(255,255,255,0.6)">Rescene AI 女儿们协作创作</text>
          </svg>
        </div>

        <h2 class="aw-title">{{ article.title }}</h2>
        <p class="aw-summary">{{ article.summary }}</p>

        <!-- 章节列表 -->
        <div class="aw-chapters">
          <div v-for="ch in chapters" :key="ch.no" class="aw-chapter" :class="{ active: ch.no === article.chapterNo }">
            <h4>第 {{ ch.no }} 章</h4>
            <div class="aw-content">{{ ch.text }}</div>
          </div>
        </div>

        <div class="aw-actions">
          <button class="aw-btn" @click="publish">📚 一键发布</button>
          <button class="aw-btn ghost" @click="nextChapter" :disabled="busy">{{ busy ? '✍️ 写中…' : '📖 写下一章' }}</button>
        </div>
        <div v-if="pubResults.length" class="aw-results">
          <div v-for="r in pubResults" :key="r.platform" class="aw-result" :class="r.ok ? 'ok' : 'fail'">
            {{ r.ok ? '✅' : '❌' }} {{ r.name }} — {{ r.message || (r.ok ? '发布成功' : '失败') }}
          </div>
        </div>
      </div>

      <!-- 文章展示 -->
      <div v-else-if="article.type === 'article' && article.title" class="aw-article">
        <h2 class="aw-title">{{ article.title }}</h2>
        <div class="aw-content">{{ article.content }}</div>
        <div class="aw-actions">
          <button class="aw-btn" @click="publish">📚 一键发布</button>
        </div>
        <div v-if="pubResults.length" class="aw-results">
          <div v-for="r in pubResults" :key="r.platform" class="aw-result" :class="r.ok ? 'ok' : 'fail'">
            {{ r.ok ? '✅' : '❌' }} {{ r.name }} — {{ r.message || (r.ok ? '发布成功' : '失败') }}
          </div>
        </div>
      </div>
    </section>
  </main>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'

const topic = ref('')
const type = ref('novel')
const article = ref({})
const chapters = ref([])
const busy = ref(false)
const notice = ref('')
const pubResults = ref([])
const gradId = 'coverGrad' + Date.now()

const collabSteps = ref([
  { name: '主编-01', emoji: '👑', doing: '构思故事走向…', result: '定了：世界观+主线', active: false, done: false },
  { name: '研究员-02', emoji: '🔬', doing: '查资料：细节设定…', result: '素材就绪', active: false, done: false },
  { name: '作者-03', emoji: '✍️', doing: '落笔写章节…', result: '章节完成', active: false, done: false },
  { name: '发布官-04', emoji: '📡', doing: '准备发布…', result: '待发布', active: false, done: false },
])

// 封面装饰：随机星星 + 渐变
const stars = Array.from({ length: 24 }, () => ({ x: 20 + Math.random() * 360, y: 30 + Math.random() * 500, r: 1 + Math.random() * 2.5 }))
const grad = ['#4a1f7a', '#8b5cf6'].concat([])
if (Math.random() > 0.5) grad[1] = '#6366f1'

const coverTitle = computed(() => {
  const t = article.value.title || '未命名'
  return t.length > 10 ? t.slice(0, 10) + '…' : t
})

function runCollab() {
  collabSteps.value.forEach(s => { s.active = false; s.done = false })
  const order = [0, 1, 2, 3]
  order.forEach((idx, t) => {
    setTimeout(() => {
      collabSteps.value[idx].active = true
      if (idx > 0) { collabSteps.value[order[idx - 1]].done = true; collabSteps.value[order[idx - 1]].active = false }
    }, t * 1200)
  })
  setTimeout(() => { collabSteps.value[3].done = true; collabSteps.value[3].active = false }, 4 * 1200)
}

async function generate() {
  if (!topic.value.trim()) { notice.value = '请输入主题'; return }
  busy.value = true
  notice.value = ''
  pubResults.value = []
  runCollab()
  try {
    const r = await fetch('/api/ai/write', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ topic: topic.value, type: type.value }),
    })
    const d = await r.json()
    if (d.error) { notice.value = d.error }
    else {
      article.value = d
      chapters.value = [{ no: 1, text: d.chapter || d.content }]
    }
  } catch (e) { notice.value = '生成失败：' + e.message }
  finally {
    busy.value = false
    collabSteps.value.forEach(s => { s.active = false; s.done = true })
  }
}

// 续写下一章
async function nextChapter() {
  if (!article.value.title) return
  busy.value = true
  runCollab()
  const cur = article.value.chapterNo || 1
  try {
    const r = await fetch('/api/ai/write', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ topic: topic.value, type: 'novel', chapter: cur, title: article.value.title }),
    })
    const d = await r.json()
    if (d.error) { notice.value = d.error }
    else {
      article.value = { ...article.value, chapterNo: d.chapterNo, chapter: d.chapter }
      chapters.value.push({ no: d.chapterNo, text: d.chapter })
    }
  } catch (e) { notice.value = '续写失败：' + e.message }
  finally {
    busy.value = false
    collabSteps.value.forEach(s => { s.active = false; s.done = true })
  }
}

async function publish() {
  const body = article.value.type === 'novel'
    ? { title: article.value.title, content: chapters.value.map(c => `第${c.no}章\n${c.text}`).join('\n\n') }
    : { title: article.value.title, content: article.value.content }
  try {
    const r = await fetch('/api/publish', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ...body, platforms: ['fanqie', 'jjwxc', 'qimao'] }),
    })
    const d = await r.json()
    pubResults.value = d.results || []
  } catch (e) { notice.value = '发布失败：' + e.message }
}

onMounted(() => { document.title = '杉汐 | AI 女儿写小说' })
</script>

<style scoped>
.ai-write-view { max-width: 900px; margin: 0 auto; padding: 24px 20px 60px; font-family: -apple-system, 'PingFang SC', 'Microsoft YaHei', sans-serif; background: linear-gradient(180deg, #faf5ff, #fff); min-height: 100vh; }
.aw-header h1 { margin: 0; font-size: 26px; background: linear-gradient(135deg, #8b5cf6, #6366f1); -webkit-background-clip: text; -webkit-text-fill-color: transparent; }
.sub { color: #888; font-size: 13px; }
.aw-collab { background: #fff; border: 1px solid #e8e0f5; border-radius: 16px; padding: 18px; margin: 14px 0; box-shadow: 0 4px 20px rgba(139,92,246,.1); }
.aw-collab-title { font-size: 15px; font-weight: 600; color: #8b5cf6; margin-bottom: 12px; }
.aw-collab-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
.aw-step { display: flex; align-items: center; gap: 8px; padding: 8px 10px; border-radius: 8px; background: #f7f5fb; }
.aw-step.active { background: #f0ebff; border: 1px solid #8b5cf6; animation: stepPulse 1s infinite; }
.aw-step.done { background: #ecfdf3; }
.aw-step-emoji { font-size: 18px; }
.aw-step-name { font-weight: 600; font-size: 13px; }
.aw-step-text { font-size: 11px; color: #777; flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
@keyframes stepPulse { 0%,100% { opacity: 1 } 50% { opacity: .6 } }
.aw-card { background: #fff; border: 1px solid #eee; border-radius: 16px; padding: 22px; }
.aw-input-row { display: flex; gap: 10px; }
.aw-input { flex: 1; border: 1px solid #ddd; border-radius: 10px; padding: 13px 15px; font-size: 15px; }
.aw-input:focus { outline: none; border-color: #8b5cf6; }
.aw-btn { background: linear-gradient(135deg, #8b5cf6, #6366f1); color: #fff; border: none; border-radius: 10px; padding: 13px 22px; font-size: 14px; cursor: pointer; white-space: nowrap; }
.aw-btn:disabled { opacity: .5; cursor: not-allowed; }
.aw-btn.ghost { background: #fff; color: #8b5cf6; border: 1px solid #8b5cf6; }
.aw-type { display: flex; gap: 8px; margin-top: 10px; }
.aw-chip { border: 1px solid #ddd; background: #fafafa; border-radius: 999px; padding: 4px 14px; font-size: 13px; cursor: pointer; }
.aw-chip.on { background: #8b5cf6; color: #fff; border-color: #8b5cf6; }
.aw-notice { margin-top: 12px; font-size: 13px; color: #8b5cf6; }
.aw-novel, .aw-article { margin-top: 18px; border-top: 2px solid #f0ebff; padding-top: 16px; }
.aw-cover-wrap { display: flex; justify-content: center; margin-bottom: 14px; }
.aw-cover { width: 220px; height: 308px; box-shadow: 0 8px 30px rgba(74,31,122,.3); border-radius: 12px; }
.aw-title { font-size: 24px; margin: 0 0 8px; text-align: center; }
.aw-summary { font-size: 14px; color: #666; background: #faf5ff; border-radius: 8px; padding: 10px; }
.aw-chapters { margin-top: 14px; }
.aw-chapter { margin-bottom: 14px; padding: 12px; border-radius: 10px; background: #fafbfc; }
.aw-chapter.active { border: 1px solid #8b5cf6; }
.aw-chapter h4 { margin: 0 0 6px; font-size: 15px; color: #8b5cf6; }
.aw-content { font-size: 15px; line-height: 1.9; color: #333; white-space: pre-wrap; }
.aw-actions { display: flex; gap: 10px; margin-top: 16px; }
.aw-results { margin-top: 12px; }
.aw-result { font-size: 13px; padding: 4px 0; }
.aw-result.ok { color: #2e9e4f; }
.aw-result.fail { color: #d43; }
</style>