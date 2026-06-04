<template>
  <div class="post-list-container">
    <!-- 标签筛选栏 -->
    <div class="tag-filter" v-if="allTags.length">
      <span class="filter-label">筛选标签：</span>
      <div class="filter-tags">
        <button
          v-for="tag in allTags"
          :key="tag"
          :class="['filter-tag', { active: selectedFilters.includes(tag) }]"
          @click="toggleFilter(tag)"
        >{{ tag }}</button>
      </div>
      <button v-if="selectedFilters.length" class="clear-filter" @click="clearFilters">清除</button>
    </div>

    <!-- 加载状态 -->
    <div v-if="loading" class="loading-state">
      <div class="loading-spinner"></div>
      <div>加载文章...</div>
    </div>

    <!-- 错误状态 -->
    <div v-else-if="error" class="error-state">
      <Icon icon="mdi:alert-circle" width="48" class="error-icon" />
      <div class="error-message">{{ error }}</div>
      <button class="retry-btn" @click="fetchPosts">重试</button>
    </div>

    <!-- 空状态 -->
    <div v-else-if="posts.length === 0" class="empty-state">
      <div class="empty-graphic">
        <div class="glow-bg"></div>
        <Icon icon="mdi:pen" width="80" class="pen-icon" />
        <div class="floating-dot dot1"></div>
        <div class="floating-dot dot2"></div>
      </div>
      <div class="empty-title">还没有笔记</div>
      <div class="empty-sub">写下你的技术思考、项目复盘或研习心得</div>
      <div class="empty-hint">第一篇笔记，从记录开始</div>
      <div class="empty-actions">
        <button class="write-btn" @click="openEditor">
          <Icon icon="mdi:pencil" width="20" />
          <span>撰写新文章</span>
        </button>
      </div>
    </div>

    <!-- 文章列表 -->
    <div v-else class="posts-grid">
      <div
        v-for="post in filteredPosts"
        :key="post.id"
        class="post-card"
        @click="goToDetail(post.slug)"
      >
        <div class="card-header">
          <h2>{{ post.title }}</h2>
          <div class="post-date">{{ formatDate(post.created_at) }}</div>
        </div>
        <div class="post-excerpt">{{ getExcerpt(post.content) }}</div>

        <!-- 已有标签 -->
        <div class="post-tags" v-if="post.tags && post.tags.length">
          <span v-for="tag in post.tags" :key="tag" class="tag-badge">
            {{ tag }}
            <button
              v-if="isLoggedIn"
              class="remove-tag"
              @click.stop="removeTagFromPost(post, tag)"
              title="删除此标签"
            >×</button>
          </span>
        </div>

        <!-- 添加标签（仅登录可见） -->
        <div v-if="isLoggedIn" class="add-tag-wrapper" @click.stop>
          <input
            v-model="post.newTagInput"
            @keyup.enter="addTagToPost(post)"
            @blur="addTagToPost(post)"
            placeholder="+ 添加标签"
            class="add-tag-input"
          />
        </div>

        <!-- 删除文章按钮 -->
        <button
          v-if="isLoggedIn"
          class="delete-post-btn"
          @click.stop="deletePost(post.id)"
          title="删除文章"
        >
          <Icon icon="mdi:delete-outline" width="18" />
        </button>
      </div>
    </div>

    <!-- 浮动写文章按钮 -->
    <button v-if="!loading && !error && posts.length > 0" class="floating-write-btn" @click="openEditor">
      <Icon icon="mdi:plus" width="24" />
    </button>

    <!-- 发布模态框 -->
    <PostEditorModal :visible="showEditor" @close="showEditor = false" @published="onPublished" />
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { Icon } from '@iconify/vue'
import PostEditorModal from './PostEditorModal.vue'

const posts = ref([])
const loading = ref(true)
const error = ref('')
const selectedFilters = ref([])
const isLoggedIn = ref(!!localStorage.getItem('token'))
const showEditor = ref(false)

// 所有标签（用于筛选栏）
const allTags = computed(() => {
  const tags = new Set()
  posts.value.forEach(post => {
    if (post.tags && post.tags.length) {
      post.tags.forEach(t => tags.add(t))
    }
  })
  return Array.from(tags).sort()
})

// 筛选后的文章
const filteredPosts = computed(() => {
  if (selectedFilters.value.length === 0) return posts.value
  return posts.value.filter(post => {
    if (!post.tags || post.tags.length === 0) return false
    return selectedFilters.value.every(filterTag => post.tags.includes(filterTag))
  })
})

function toggleFilter(tag) {
  const index = selectedFilters.value.indexOf(tag)
  if (index === -1) selectedFilters.value.push(tag)
  else selectedFilters.value.splice(index, 1)
}
function clearFilters() { selectedFilters.value = [] }

// 删除标签
async function removeTagFromPost(post, tagToRemove) {
  const newTags = (post.tags || []).filter(t => t !== tagToRemove)
  if (newTags.length === (post.tags?.length || 0)) return
  try {
    const res = await fetch(`/api/posts/${post.id}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ tags: newTags })
    })
    if (res.ok) {
      post.tags = newTags
    } else {
      console.error('删除标签失败')
    }
  } catch (err) {
    console.error('删除标签异常', err)
  }
}

// 添加标签
async function addTagToPost(post) {
  const newTag = post.newTagInput?.trim()
  if (!newTag) {
    post.newTagInput = ''
    return
  }
  const currentTags = post.tags || []
  if (currentTags.includes(newTag)) {
    post.newTagInput = ''
    return
  }
  const newTags = [...currentTags, newTag]
  try {
    const res = await fetch(`/api/posts/${post.id}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ tags: newTags })
    })
    if (res.ok) {
      post.tags = newTags
    } else {
      console.error('添加标签失败')
    }
  } catch (err) {
    console.error('添加标签异常', err)
  }
  post.newTagInput = ''
}

// 删除文章
async function deletePost(id) {
  if (!confirm('确定删除这篇文章吗？')) return
  try {
    const res = await fetch(`/api/posts/${id}`, { method: 'DELETE' })
    if (res.ok) fetchPosts()
    else alert('删除失败')
  } catch (err) { alert('删除失败') }
}

// 加载文章
async function fetchPosts() {
  loading.value = true
  error.value = ''
  const controller = new AbortController()
  const timeoutId = setTimeout(() => controller.abort(), 10000)
  try {
    const res = await fetch('/api/posts', { signal: controller.signal })
    clearTimeout(timeoutId)
    if (res.ok) {
      const data = await res.json()
      posts.value = Array.isArray(data) ? data : []
      // 为每个文章添加临时变量 newTagInput
      posts.value.forEach(p => { p.newTagInput = '' })
    } else {
      error.value = `请求失败（${res.status}）`
    }
  } catch (err) {
    if (err.name === 'AbortError') error.value = '请求超时'
    else error.value = '网络错误'
    console.error(err)
  } finally {
    loading.value = false
  }
}

function formatDate(dateStr) {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  return `${d.getFullYear()}.${d.getMonth() + 1}.${d.getDate()}`
}

function getExcerpt(content) {
  if (!content) return ''
  const plainText = content.replace(/<[^>]*>/g, '')
  return plainText.slice(0, 120) + '...'
}

function goToDetail(slug) {
  window.location.href = `/blog/post?slug=${slug}`
}

function openEditor() { showEditor.value = true }
function onPublished() { fetchPosts() }

onMounted(() => { fetchPosts() })
</script>

<style scoped>
/* 保留原有全部样式，添加 .add-tag-wrapper 和 .add-tag-input 样式 */
.floating-write-btn {
  position: fixed;
  bottom: 90px;
  right: 24px;
  width: 56px;
  height: 56px;
  border-radius: 28px;
  background: #2563eb;
  color: white;
  border: none;
  box-shadow: 0 4px 12px rgba(37, 99, 235, 0.3);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: transform 0.2s;
  z-index: 100;
}
.floating-write-btn:hover {
  transform: scale(1.05);
}
@media (max-width: 768px) {
  .floating-write-btn {
    bottom: 70px;
    right: 16px;
    width: 48px;
    height: 48px;
  }
}
.post-list-container {
  max-width: 900px;
  margin: 0 auto;
  padding: 2rem 1.5rem;
  min-height: 60vh;
  position: relative;
}

/* 标签筛选栏 */
.tag-filter {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 24px;
  flex-wrap: wrap;
}
.filter-label {
  font-size: 0.85rem;
  color: #64748b;
}
.filter-tags {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.filter-tag {
  background: #f1f5f9;
  border: none;
  padding: 6px 16px;
  border-radius: 30px;
  cursor: pointer;
  font-size: 0.85rem;
  transition: all 0.2s;
}
.filter-tag.active {
  background: #2563eb;
  color: white;
}
.clear-filter {
  background: none;
  border: none;
  color: #ef4444;
  cursor: pointer;
  font-size: 0.8rem;
}

/* 加载状态 */
.loading-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 1rem;
  padding: 5rem 0;
  color: #64748b;
}
.loading-spinner {
  width: 40px;
  height: 40px;
  border: 3px solid #e2e8f0;
  border-top-color: #2563eb;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}
@keyframes spin {
  to { transform: rotate(360deg); }
}
.error-state {
  text-align: center;
  padding: 4rem 2rem;
  background: #fef2f2;
  border-radius: 32px;
  margin-top: 2rem;
}
.error-icon {
  color: #b91c1c;
  margin-bottom: 1rem;
}
.error-message {
  color: #b91c1c;
  margin-bottom: 1.5rem;
}
.retry-btn {
  background: #2563eb;
  color: white;
  border: none;
  padding: 0.5rem 1.5rem;
  border-radius: 40px;
  cursor: pointer;
  transition: 0.2s;
}
.empty-state {
  text-align: center;
  padding: 3rem 1rem 4rem;
  position: relative;
  overflow: hidden;
  border-radius: 48px;
  background: linear-gradient(145deg, #f8fafc 0%, #ffffff 100%);
  box-shadow: 0 20px 35px -12px rgba(0, 0, 0, 0.05);
  margin: 1rem 0;
}
.empty-graphic {
  position: relative;
  width: 160px;
  height: 160px;
  margin: 0 auto 2rem;
  display: flex;
  align-items: center;
  justify-content: center;
}
.glow-bg {
  position: absolute;
  width: 140px;
  height: 140px;
  background: radial-gradient(circle, rgba(37, 99, 235, 0.08) 0%, rgba(37, 99, 235, 0) 70%);
  border-radius: 50%;
  animation: pulse 3s infinite;
}
.pen-icon {
  z-index: 1;
  filter: drop-shadow(0 8px 12px rgba(0,0,0,0.1));
  animation: float 3s ease-in-out infinite;
  color: #2563eb;
}
.floating-dot {
  position: absolute;
  width: 8px;
  height: 8px;
  background: #2563eb;
  border-radius: 50%;
  opacity: 0.5;
}
.dot1 {
  top: 20px;
  left: 20px;
  animation: floatDot 4s infinite;
}
.dot2 {
  bottom: 30px;
  right: 30px;
  width: 12px;
  height: 12px;
  background: #60a5fa;
  animation: floatDot 5s infinite reverse;
}
@keyframes float {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(-10px); }
}
@keyframes floatDot {
  0%, 100% { transform: translate(0, 0); }
  50% { transform: translate(10px, -10px); }
}
@keyframes pulse {
  0% { transform: scale(0.9); opacity: 0.5; }
  50% { transform: scale(1.1); opacity: 0.8; }
  100% { transform: scale(0.9); opacity: 0.5; }
}
.empty-title {
  font-size: 2rem;
  font-weight: 600;
  color: #0f172a;
  margin-bottom: 0.5rem;
}
.empty-sub {
  font-size: 1rem;
  color: #475569;
  max-width: 400px;
  margin: 0 auto 0.5rem;
}
.empty-hint {
  font-size: 0.85rem;
  color: #94a3b8;
  margin-bottom: 2rem;
}
.empty-actions {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 1rem;
  flex-wrap: wrap;
}
.write-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  background: #2563eb;
  color: white;
  padding: 0.7rem 1.8rem;
  border-radius: 60px;
  text-decoration: none;
  font-weight: 500;
  transition: all 0.2s;
  box-shadow: 0 2px 6px rgba(37, 99, 235, 0.2);
}
.write-btn:hover {
  background: #1d4ed8;
  transform: translateY(-2px);
}

/* 文章列表 - 整张卡片可点击 */
.posts-grid {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}
.post-card {
  background: white;
  border: 1px solid #eef2f6;
  border-radius: 24px;
  padding: 1.8rem;
  transition: all 0.2s ease;
  text-decoration: none;
  color: inherit;
  display: block;
  position: relative;
  cursor: pointer;
}
.post-card:hover {
  border-color: #cbd5e1;
  box-shadow: 0 12px 24px -12px rgba(0, 0, 0, 0.08);
  transform: translateY(-2px);
}
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  flex-wrap: wrap;
  margin-bottom: 0.75rem;
}
.post-card h2 {
  font-size: 1.4rem;
  font-weight: 600;
  margin: 0;
  color: #0f172a;
}
.post-date {
  font-size: 0.8rem;
  color: #94a3b8;
}
.post-excerpt {
  color: #475569;
  line-height: 1.5;
  margin-bottom: 1rem;
}
/* 已有标签区域 */
.post-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin: 12px 0;
}
.tag-badge {
  background: #eef2ff;
  color: #2563eb;
  padding: 4px 10px;
  border-radius: 30px;
  font-size: 0.75rem;
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.remove-tag {
  background: none;
  border: none;
  font-size: 0.9rem;
  cursor: pointer;
  color: #64748b;
  padding: 0 4px;
  border-radius: 50%;
  line-height: 1;
}
.remove-tag:hover {
  color: #ef4444;
  background: #fee2e2;
}
/* 添加标签输入框 */
.add-tag-wrapper {
  margin-top: 8px;
}
.add-tag-input {
  border: 1px solid #e2e8f0;
  border-radius: 40px;
  padding: 4px 12px;
  font-size: 0.75rem;
  width: 140px;
  outline: none;
  transition: border-color 0.2s;
}
.add-tag-input:focus {
  border-color: #2563eb;
}
/* 删除文章按钮 */
.delete-post-btn {
  position: absolute;
  bottom: 20px;
  right: 20px;
  background: rgba(255,255,255,0.8);
  border: 1px solid #e2e8f0;
  border-radius: 30px;
  padding: 6px 10px;
  cursor: pointer;
  opacity: 0;
  transition: opacity 0.2s;
  color: #64748b;
}
.post-card:hover .delete-post-btn {
  opacity: 1;
}
.delete-post-btn:hover {
  background: #fee2e2;
  border-color: #ef4444;
  color: #ef4444;
}
@media (max-width: 640px) {
  .post-list-container {
    padding: 1rem;
  }
  .empty-title {
    font-size: 1.5rem;
  }
  .empty-graphic {
    width: 120px;
    height: 120px;
  }
  .pen-icon {
    width: 60px;
  }
  .post-card h2 {
    font-size: 1.2rem;
  }
  .delete-post-btn {
    opacity: 0.6;
  }
  .add-tag-input {
    width: 100%;
  }
}
</style>