<template>
  <div v-if="papers.length" class="arxiv-cards">
    <div class="arxiv-cards-head">
      <Icon icon="mdi:book-open-page-variant" width="16" class="arxiv-cards-icon" />
      <span>arXiv 论文检索</span>
      <span class="arxiv-cards-count">{{ papers.length }} 篇</span>
    </div>
    <div class="arxiv-cards-list">
      <a
        v-for="p in papers"
        :key="p.id"
        :href="p.abs_url"
        target="_blank"
        rel="noopener"
        class="arxiv-card"
      >
        <div class="arxiv-card-thumb">
          <img
            :src="thumbnailURL(p.id)"
            :alt="p.title"
            loading="lazy"
            @error="onThumbError($event)"
          />
        </div>
        <div class="arxiv-card-body">
          <div class="arxiv-card-title">{{ p.title }}</div>
          <div class="arxiv-card-meta">
            <span class="arxiv-card-authors">{{ formatAuthors(p.authors) }}</span>
            <span class="arxiv-card-date">{{ p.published }}</span>
            <span v-if="p.primary_category" class="arxiv-card-cat">{{ p.primary_category }}</span>
          </div>
          <div v-if="p.summary" class="arxiv-card-summary">{{ p.summary }}</div>
          <div class="arxiv-card-links">
            <span class="arxiv-card-link" title="PDF">
              <Icon icon="mdi:file-pdf-box" width="14" /> PDF
            </span>
            <span class="arxiv-card-link" title="HTML 预览">
              <Icon icon="mdi:web" width="14" /> HTML
            </span>
            <span class="arxiv-card-link" :href="p.abs_url" target="_blank" rel="noopener" title="arXiv 页面">
              <Icon icon="mdi:arrow-top-right" width="14" /> arXiv
            </span>
          </div>
        </div>
      </a>
    </div>
  </div>
  <div v-else class="arxiv-empty">
    <Icon icon="mdi:book-search" width="20" />
    <span>未找到相关论文</span>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { Icon } from '@iconify/vue'

const props = defineProps({
  output: { type: String, default: '' },
})

const papers = computed(() => {
  try {
    const data = JSON.parse(props.output)
    return Array.isArray(data.papers) ? data.papers : []
  } catch {
    return []
  }
})

function thumbnailURL(id) {
  // alphaXiv 缩略图服务：260x195 裁剪
  return `https://thumbnails.assets.alphaxiv.org/${encodeURIComponent(id)}v1.png`
}

function onThumbError(e) {
  e.target.style.display = 'none'
}

function formatAuthors(authors) {
  if (!authors || !authors.length) return ''
  if (authors.length <= 3) return authors.join(', ')
  return `${authors[0]} 等 ${authors.length} 人`
}
</script>

<style scoped>
.arxiv-cards {
  margin: 6px 0;
  border: 1px solid var(--app-border);
  border-radius: 12px;
  background: var(--app-surface);
  overflow: hidden;
}
.arxiv-cards-head {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--app-border);
  background: var(--app-surface-2);
  font-size: 13px;
  font-weight: 700;
  color: var(--app-text);
}
.arxiv-cards-icon {
  color: var(--app-accent);
}
.arxiv-cards-count {
  font-size: 11px;
  font-weight: 600;
  color: var(--app-text-soft);
  background: var(--app-surface-3);
  padding: 1px 8px;
  border-radius: 999px;
}
.arxiv-cards-list {
  display: flex;
  flex-direction: column;
}
.arxiv-card {
  display: flex;
  gap: 12px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--app-border-soft);
  text-decoration: none;
  color: var(--app-text);
  transition: background 0.15s ease;
}
.arxiv-card:last-child {
  border-bottom: none;
}
.arxiv-card:hover {
  background: var(--app-surface-2);
}
.arxiv-card-thumb {
  flex-shrink: 0;
  width: 80px;
  height: 60px;
  border-radius: 6px;
  overflow: hidden;
  background: var(--app-surface-3);
}
.arxiv-card-thumb img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
.arxiv-card-body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.arxiv-card-title {
  font-size: 13px;
  font-weight: 700;
  line-height: 1.4;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  color: var(--app-text);
}
.arxiv-card-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  color: var(--app-text-soft);
  flex-wrap: wrap;
}
.arxiv-card-authors {
  font-style: italic;
}
.arxiv-card-date {
  white-space: nowrap;
}
.arxiv-card-cat {
  background: var(--app-accent-soft);
  color: var(--app-accent);
  padding: 0 6px;
  border-radius: 4px;
  font-weight: 600;
}
.arxiv-card-summary {
  font-size: 12px;
  color: var(--app-text-soft);
  line-height: 1.5;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.arxiv-card-links {
  display: flex;
  gap: 12px;
  font-size: 11px;
}
.arxiv-card-link {
  color: var(--app-accent);
  text-decoration: none;
  display: inline-flex;
  align-items: center;
  gap: 3px;
  font-weight: 600;
  transition: opacity 0.15s;
}
.arxiv-card-link:hover {
  opacity: 0.7;
}
.arxiv-empty {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 14px;
  font-size: 13px;
  color: var(--app-text-soft);
}
</style>
