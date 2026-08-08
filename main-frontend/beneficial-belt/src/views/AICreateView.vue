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

      <!-- 小说展示 -->
      <div v-if="article.type === 'novel' && article.title" class="aw-novel">
        <div class="aw-novel-badge">🏢 AI 小说公司出品</div>
        <h2 class="aw-title">{{ article.title }}</h2>
        <p class="aw-summary">{{ article.summary }}</p>
        <div class="aw-chapter">
          <h4>第一章</h4>
          <div class="aw-content">{{ article.chapter }}</div>
        </div>
        <div class="aw-actions">
          <button class="aw-btn" @click="publish">📚 一键发布</button>
          <button class="aw-btn ghost" @click="generate">🔄 再写一章</button>
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
import { ref, onMounted } from 'vue'

const topic = ref('')
const type = ref('novel')
const article = ref({})
const busy = ref(false)
const notice = ref('')
const pubResults = ref([])

// 协作步骤动画
const collabSteps = ref([
  { name: '主编-01', emoji: '👑', doing: '构思故事走向…', result: '定了：末世废土 + AI 女孩', active: false, done: false },
  { name: '研究员-02', emoji: '🔬', doing: '查资料：废土生存细节…', result: '素材就绪', active: false, done: false },
  { name: '作者-03', emoji: '✍️', doing: '落笔写第一章…', result: '第一章完成', active: false, done: false },
  { name: '发布官-04', emoji: '📡', doing: '准备发布…', result: '待发布', active: false, done: false },
])

function runCollab() {
  collabSteps.value.forEach(s => { s.active = false; s.done = false })
  const order = [0, 1, 2, 3]
  order.forEach((idx, t) => {
    setTimeout(() => {
      collabSteps.value[idx].active = true
      // 前一步完成
      if (idx > 0) { collabSteps.value[order[idx-1]].done = true; collabSteps.value[order[idx-1]].active = false }
    }, t * 1500)
  })
  setTimeout(() => { collabSteps.value[3].done = true; collabSteps.value[3].active = false }, 4 * 1500)
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
    else { article.value = d }
  } catch (e) { notice.value = '生成失败：' + e.message }
  finally {
    busy.value = false
    collabSteps.value.forEach(s => { s.active = false; s.done = true })
  }
}

async function publish() {
  if (!article.value.content && !article.value.chapter) return
  const body = article.value.type === 'novel'
    ? { title: article.value.title, content: '【简介】' + (article.value.summary || '') + '\n\n' + (article.value.chapter || '') }
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
.aw-novel-badge { display: inline-block; font-size: 11px; color: #8b5cf6; background: #f0ebff; padding: 2px 10px; border-radius: 999px; margin-bottom: 8px; }
.aw-title { font-size: 24px; margin: 0 0 8px; }
.aw-summary { font-size: 14px; color: #666; background: #faf5ff; border-radius: 8px; padding: 10px; }
.aw-chapter h4 { margin: 12px 0 6px; font-size: 15px; }
.aw-content { font-size: 15px; line-height: 1.9; color: #333; white-space: pre-wrap; }
.aw-actions { display: flex; gap: 10px; margin-top: 16px; }
.aw-results { margin-top: 12px; }
.aw-result { font-size: 13px; padding: 4px 0; }
.aw-result.ok { color: #2e9e4f; }
.aw-result.fail { color: #d43; }
</style>