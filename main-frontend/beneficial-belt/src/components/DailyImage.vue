<template>
  <div class="daily-card">
    <div class="daily-header">
      <div class="title-wrapper">
        <Icon icon="mdi:star-four-points" width="22" class="title-icon" />
        <h3>今日一图</h3>
      </div>
      <span class="date">{{ today }}</span>
    </div>
    <div v-if="loading" class="loading">加载中...</div>
    <div v-else-if="error" class="error">{{ error }}</div>
    <div v-else>
      <div class="image-wrapper" @click="openOriginal">
        <img :src="imageUrl" :alt="'今日图片'" />
      </div>
      <div class="comment">
        <Icon icon="mdi:message-text" width="20" class="quote-icon" />
        <p>{{ comment }}</p>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { Icon } from '@iconify/vue'

const today = computed(() => {
  const d = new Date()
  return `${d.getFullYear()}.${d.getMonth()+1}.${d.getDate()}`
})

const loading = ref(true)
const error = ref('')
const imageUrl = ref('')
const comment = ref('')
const cacheKey = computed(() => `daily_image_${today.value}`)

async function fetchDaily() {
  const cached = localStorage.getItem(cacheKey.value)
  if (cached) {
    try {
      const data = JSON.parse(cached)
      imageUrl.value = data.imageUrl
      comment.value = data.comment
      loading.value = false
      return
    } catch (e) {}
  }

  try {
    const res = await fetch('/api/images/random')
    if (!res.ok) throw new Error('获取失败')
    const data = await res.json()
    imageUrl.value = data.imageUrl
    comment.value = data.comment || '暂无评价'
    localStorage.setItem(cacheKey.value, JSON.stringify({
      imageUrl: data.imageUrl,
      comment: data.comment
    }))
  } catch (err) {
    console.error(err)
    error.value = '加载失败，请刷新重试'
  } finally {
    loading.value = false
  }
}

function openOriginal() {
  if (imageUrl.value) window.open(window.location.origin + imageUrl.value, '_blank')
}

onMounted(() => {
  fetchDaily()
})
</script>

<style scoped>
.daily-card {
  background: white;
  border-radius: 24px;
  box-shadow: 0 8px 20px rgba(0,0,0,0.03);
  padding: 20px;
  margin: 20px auto;
  max-width: 500px;
  border: 1px solid #eef2f6;
}
.daily-header {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  margin-bottom: 16px;
  padding-bottom: 8px;
  border-bottom: 1px solid #eef2f6;
}
.title-wrapper {
  display: flex;
  align-items: center;
  gap: 6px;
}
.title-icon {
  color: #2563eb;
}
.daily-header h3 {
  margin: 0;
  font-size: 1.2rem;
  font-weight: 500;
  color: #2563eb;
}
.date {
  font-size: 0.8rem;
  color: #94a3b8;
}
.image-wrapper {
  display: flex;
  justify-content: center;
  margin-bottom: 16px;
  cursor: pointer;
}
.image-wrapper img {
  max-width: 100%;
  max-height: 280px;
  border-radius: 16px;
  object-fit: contain;
  background: #f8fafc;
  transition: transform 0.2s;
}
.image-wrapper img:hover {
  transform: scale(1.02);
}
.comment {
  display: flex;
  gap: 12px;
  background: #f8fafc;
  padding: 16px;
  border-radius: 20px;
  color: #1e293b;
  font-size: 0.9rem;
  line-height: 1.5;
}
.quote-icon {
  flex-shrink: 0;
  color: #2563eb;
}
.comment p {
  margin: 0;
}
.loading, .error {
  text-align: center;
  padding: 40px 20px;
  color: #94a3b8;
}
.error {
  color: #ef4444;
}
</style>