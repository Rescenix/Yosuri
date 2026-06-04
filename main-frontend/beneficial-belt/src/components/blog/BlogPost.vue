<template>
  <div class="blog-container">
    <div v-if="loading" class="status-message">正在加载文章...</div>
    <div v-else-if="error" class="status-message error-message">
      <p>{{ error }}</p>
      <a href="/blog" class="back-link">← 回到杉汐的日记本</a>
    </div>
    <article v-else class="post-card">
      <header class="post-header">
        <h1 class="post-title">{{ post.title }}</h1>
        <div class="post-meta">
          <time class="post-date">{{ formatDate(post.created_at) }}</time>
          <span v-if="post.tags && post.tags.length" class="post-tags">
            <span v-for="tag in post.tags" :key="tag" class="tag">#{{ tag }}</span>
          </span>
        </div>
      </header>
      <div class="post-content" v-html="renderedContent"></div>
      <footer class="post-footer">
        <a href="/blog" class="back-link">← 所有文章</a>
      </footer>
    </article>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { marked } from 'marked' // 需要安装：npm install marked

const post = ref(null)
const loading = ref(true)
const error = ref('')

function formatDate(dateStr) {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  return `${d.getFullYear()}.${d.getMonth() + 1}.${d.getDate()}`
}

// 使用 marked 渲染 Markdown（若后端存的是纯文本，可保留；若存 HTML 则直接输出）
const renderedContent = computed(() => {
  if (!post.value?.content) return ''
  // 如果内容已经是 HTML，直接返回；否则当作 Markdown 渲染
  // 简单判断：是否包含 HTML 标签（但可能误判），为了安全统一用 marked
  return marked.parse(post.value.content, { breaks: true, gfm: true })
})

onMounted(async () => {
  const params = new URLSearchParams(window.location.search)
  const slug = params.get('slug') || ''
  if (!slug) {
    error.value = '请指定文章'
    loading.value = false
    return
  }

  try {
    const res = await fetch('/api/posts')
    if (res.ok) {
      const posts = await res.json()
      const found = posts.find(p => p.slug === slug)
      if (found) {
        post.value = found
      } else {
        error.value = '这篇文章暂时还未出现'
      }
    } else {
      error.value = '无法加载文章，请稍后再试'
    }
  } catch (e) {
    error.value = '网络开小差了，请稍后刷新'
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
/* 强制允许滚动（覆盖全局锁定） */
:global(html), :global(body) {
  overflow: auto !important;
  height: auto !important;
  -webkit-overflow-scrolling: touch;
}

.blog-container {
  max-width: 860px;
  margin: 0 auto;
  padding: 2rem 1.5rem;
  min-height: 100vh;          /* 确保内容超出时有滚动空间 */
  overflow-y: auto;           /* 允许垂直滚动 */
  -webkit-overflow-scrolling: touch; /* 移动端平滑滚动 */
}

/* 其余原有样式保持不变 */
.status-message {
  text-align: center;
  padding: 4rem 2rem;
  color: #64748b;
  background: white;
  border-radius: 28px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.02);
  font-size: 1rem;
}
.error-message {
  color: #ef4444;
}
.error-message .back-link {
  display: inline-block;
  margin-top: 1rem;
  color: #2563eb;
}

.post-card {
  background: white;
  border-radius: 28px;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.02);
  border: 1px solid #eef2f6;
  overflow: hidden;
  transition: box-shadow 0.2s ease;
}

.post-header {
  padding: 2rem 2rem 1rem 2rem;
  border-bottom: 1px solid #f0f4f8;
}

.post-title {
  margin: 0 0 0.5rem 0;
  font-size: 2rem;
  font-weight: 600;
  letter-spacing: -0.02em;
  color: #0f172a;
  line-height: 1.3;
}

.post-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 1rem;
  margin-top: 0.5rem;
  font-size: 0.85rem;
  color: #5b6e8c;
}

.post-date {
  color: #6c86a3;
}

.post-tags {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}
.tag {
  background: #eef2ff;
  color: #2563eb;
  padding: 0.2rem 0.75rem;
  border-radius: 30px;
  font-size: 0.75rem;
  font-weight: 500;
}

.post-content {
  padding: 1.5rem 2rem 2rem 2rem;
  font-size: 1rem;
  line-height: 1.7;
  color: #1e293b;
}

/* Markdown 内容样式（可选） */
.post-content :deep(p) {
  margin-bottom: 1.2em;
}
.post-content :deep(h1), .post-content :deep(h2), .post-content :deep(h3) {
  margin-top: 1.5em;
  margin-bottom: 0.6em;
  font-weight: 600;
  color: #0f172a;
}
.post-content :deep(h2) {
  font-size: 1.5rem;
  border-left: 4px solid #2563eb;
  padding-left: 1rem;
}
.post-content :deep(code) {
  background: #f1f5f9;
  padding: 0.2rem 0.4rem;
  border-radius: 8px;
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.85em;
  color: #1e293b;
}
.post-content :deep(pre) {
  background: #f8fafc;
  padding: 1rem;
  border-radius: 16px;
  overflow-x: auto;
  margin: 1.5rem 0;
}
.post-content :deep(pre code) {
  background: none;
  padding: 0;
}
.post-content :deep(blockquote) {
  border-left: 3px solid #2563eb;
  margin: 1.2rem 0;
  padding-left: 1.2rem;
  color: #475569;
  font-style: normal;
}
.post-content :deep(a) {
  color: #2563eb;
  text-decoration: none;
  border-bottom: 1px solid transparent;
  transition: border-color 0.2s;
}
.post-content :deep(a:hover) {
  border-bottom-color: #2563eb;
}
.post-content :deep(img) {
  max-width: 100%;
  border-radius: 16px;
  margin: 1.2rem 0;
}

.post-footer {
  padding: 1rem 2rem 2rem 2rem;
  border-top: 1px solid #f0f4f8;
  text-align: right;
}

.back-link {
  color: #2563eb;
  text-decoration: none;
  font-size: 0.9rem;
  transition: color 0.2s;
}
.back-link:hover {
  color: #1d4ed8;
  text-decoration: underline;
}

/* 移动端适配 */
@media (max-width: 640px) {
  .blog-container {
    padding: 1rem;
  }
  .post-header {
    padding: 1.5rem;
  }
  .post-content {
    padding: 1rem 1.5rem 1.5rem 1.5rem;
  }
  .post-title {
    font-size: 1.6rem;
  }
}
</style>