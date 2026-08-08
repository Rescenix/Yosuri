<template>
  <main class="ai-write-view">
    <header class="aw-header">
      <h1>✨ AI 写作工坊</h1>
      <p class="sub">输入主题，AI 公司里的作者立即为你写出完整文章 · 一键发布全网</p>
    </header>

    <section class="aw-card">
      <div class="aw-input-row">
        <input
          v-model="topic"
          class="aw-input"
          placeholder="输入文章主题，如：AI 会取代程序员吗？"
          @keyup.enter="generate"
        />
        <button class="aw-btn" :disabled="busy" @click="generate">
          {{ busy ? '✍️ AI 写作中…' : '✨ AI 生成' }}
        </button>
      </div>

      <div v-if="notice" class="aw-notice">{{ notice }}</div>

      <div v-if="article.title" class="aw-article">
        <h2 class="aw-title">{{ article.title }}</h2>
        <div class="aw-content">{{ article.content }}</div>

        <div class="aw-actions">
          <button class="aw-btn" @click="publish">📚 一键发布到网文平台</button>
          <button class="aw-btn ghost" @click="regenerate">🔄 重新生成</button>
        </div>
        <div v-if="pubNotice" class="aw-notice">{{ pubNotice }}</div>
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
const article = ref({ title: '', content: '' })
const busy = ref(false)
const notice = ref('')
const pubNotice = ref('')
const pubResults = ref([])

async function generate() {
  if (!topic.value.trim()) { notice.value = '请输入主题'; return }
  busy.value = true
  notice.value = ''
  pubResults.value = []
  try {
    const r = await fetch('/api/ai/write', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ topic: topic.value }),
    })
    const d = await r.json()
    if (d.error) { notice.value = d.error }
    else { article.value = { title: d.title, content: d.content } }
  } catch (e) { notice.value = '生成失败：' + e.message }
  finally { busy.value = false }
}

function regenerate() { generate() }

async function publish() {
  if (!article.value.content) return
  pubNotice.value = '发布中…'
  pubResults.value = []
  try {
    const r = await fetch('/api/publish', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        title: article.value.title,
        content: article.value.content,
        platforms: ['fanqie', 'jjwxc', 'qimao'],
      }),
    })
    const d = await r.json()
    pubResults.value = d.results || []
    const ok = pubResults.value.filter(x => x.ok).length
    pubNotice.value = `发布完成：${ok}/${pubResults.value.length} 成功`
  } catch (e) { pubNotice.value = '发布失败：' + e.message }
}

onMounted(() => { document.title = '杉汐 | AI 写作工坊' })
</script>

<style scoped>
.ai-write-view { max-width: 860px; margin: 0 auto; padding: 28px 20px 60px; font-family: -apple-system, 'PingFang SC', 'Microsoft YaHei', sans-serif; }
.aw-header h1 { margin: 0 0 4px; font-size: 24px; }
.sub { color: #888; font-size: 13px; }
.aw-card { background: #fff; border: 1px solid #eee; border-radius: 12px; padding: 24px; margin-top: 16px; }
.aw-input-row { display: flex; gap: 10px; }
.aw-input { flex: 1; border: 1px solid #ddd; border-radius: 8px; padding: 12px 14px; font-size: 15px; }
.aw-input:focus { outline: none; border-color: #8b5cf6; }
.aw-btn { background: linear-gradient(135deg, #8b5cf6, #6366f1); color: #fff; border: none; border-radius: 8px; padding: 12px 20px; font-size: 14px; cursor: pointer; white-space: nowrap; }
.aw-btn:disabled { opacity: .5; cursor: not-allowed; }
.aw-btn.ghost { background: #fff; color: #8b5cf6; border: 1px solid #8b5cf6; }
.aw-notice { margin-top: 12px; font-size: 13px; color: #8b5cf6; }
.aw-article { margin-top: 20px; border-top: 1px solid #eee; padding-top: 16px; }
.aw-title { font-size: 22px; margin: 0 0 12px; }
.aw-content { font-size: 15px; line-height: 1.9; color: #333; white-space: pre-wrap; }
.aw-actions { display: flex; gap: 10px; margin-top: 18px; }
.aw-results { margin-top: 12px; }
.aw-result { font-size: 13px; padding: 4px 0; }
.aw-result.ok { color: #2e9e4f; }
.aw-result.fail { color: #d43; }
</style>