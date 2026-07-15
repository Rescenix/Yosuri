<template>
  <div>
    <NavBar />
    <div class="article-page">
      <BlogPost />
    </div>
  </div>
</template>

<script setup>
import { onMounted } from 'vue'
import NavBar from '../components/NavBar.vue'
import BlogPost from '../components/blog/BlogPost.vue'

onMounted(() => {
  document.title = '文章 | 杉汐的日记本'
})
</script>

<style scoped>
.article-page {
  max-width: 800px;
  margin: 100px auto 60px;
  padding: 0 24px;
}

.article-page :deep(article) {
  padding: 0;
  background: transparent;
}

.article-page :deep(h1) {
  font-size: 2.2rem;
  font-weight: 700;
  background: linear-gradient(135deg, #2563eb, #60a5fa);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  margin-bottom: 0.75rem;
  line-height: 1.3;
}

.article-page :deep(time),
.article-page :deep(.post-date) {
  display: block;
  font-size: 0.95rem;
  color: #64748b;
  margin-bottom: 2.5rem;
  border-left: 3px solid #2563eb;
  padding-left: 0.75rem;
}

.article-page :deep(.post-content) {
  font-size: 1.08rem;
  line-height: 1.85;
  color: #334155;
}

.article-page :deep(.post-content p) {
  margin-bottom: 1.3rem;
}

.article-page :deep(.post-content img) {
  max-width: 100%;
  border-radius: 8px;
  margin: 1.5rem 0;
}

.article-page :deep(.post-content pre) {
  background: #f1f5f9;
  padding: 1.2rem;
  border-radius: 8px;
  overflow-x: auto;
  font-size: 0.95rem;
  line-height: 1.6;
}

.article-page :deep(.post-not-found) {
  text-align: center;
  padding: 80px 0;
  color: #94a3b8;
}

.article-page :deep(.post-not-found a) {
  color: #2563eb;
  text-decoration: none;
  font-weight: 500;
}

@media (max-width: 768px) {
  .article-page {
    margin-top: 80px;
    padding: 0 16px;
  }
  .article-page :deep(h1) {
    font-size: 1.8rem;
  }
  .article-page :deep(.post-content) {
    font-size: 1rem;
    line-height: 1.7;
  }
}
</style>
