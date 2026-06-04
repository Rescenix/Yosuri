<template>
  <div class="gallery-section" v-if="displayImages.length">
    <div class="gallery-header">
      <Icon icon="mdi:image-multiple" width="22" class="header-icon" />
      <h3>偶遇时光</h3>
    </div>

    <div class="carousel-container">
      <button class="nav-btn prev" @click="prevSlide" :disabled="isAnimating">
        <Icon icon="mdi:chevron-left" width="28" />
      </button>

      <div class="carousel-track-wrapper" ref="trackWrapper">
        <div class="carousel-track" ref="track" :style="{ transform: `translateX(${offsetX}px)` }">
          <div
            v-for="img in displayImages"
            :key="img.url"
            class="gallery-card"
            @click="openImage(img.url)"
          >
            <div class="card-image">
              <img :src="img.url" :alt="img.rel_path" loading="lazy" />
            </div>
            <div class="card-footer" v-if="img.tags && img.tags.length">
              <span class="tag">{{ img.tags[0] }}</span>
              <span v-if="img.tags.length > 1" class="tag-count">+{{ img.tags.length - 1 }}</span>
            </div>
          </div>
        </div>
      </div>

      <button class="nav-btn next" @click="nextSlide" :disabled="isAnimating">
        <Icon icon="mdi:chevron-right" width="28" />
      </button>
    </div>

    <div class="dots" v-if="totalSlides > 1">
      <span
        v-for="i in totalSlides"
        :key="i"
        class="dot"
        :class="{ active: i - 1 === currentIndex }"
        @click="goToSlide(i - 1)"
      ></span>
    </div>
  </div>
  <div v-else class="gallery-section empty">
    <p>暂无更多图片</p>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, computed, nextTick } from 'vue'
import { Icon } from '@iconify/vue'

const MAX_ITEMS = 10
const displayImages = ref([])

// 轮播参数
const track = ref(null)
const trackWrapper = ref(null)
let slideWidth = 0
const totalSlides = ref(0)
const currentIndex = ref(0)
const offsetX = ref(0)
const isAnimating = ref(false)
let autoTimer = null
const AUTO_INTERVAL = 5000

function shuffleArray(arr) {
  for (let i = arr.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [arr[i], arr[j]] = [arr[j], arr[i]]
  }
  return arr
}

async function fetchRandomImages() {
  try {
    const res = await fetch('/api/images')
    if (!res.ok) throw new Error('加载失败')
    const all = await res.json()
    if (!all.length) return
    const shuffled = shuffleArray([...all])
    displayImages.value = shuffled.slice(0, MAX_ITEMS)
    await nextTick()
    calcSlideWidth()
    totalSlides.value = displayImages.value.length
    goToSlide(0, false)
  } catch (err) {
    console.error('获取画廊图片失败', err)
  }
}

function calcSlideWidth() {
  if (!track.value || !trackWrapper.value) return
  const firstCard = track.value.querySelector('.gallery-card')
  if (firstCard) {
    slideWidth = firstCard.offsetWidth + 16 // 宽度 + gap
  } else {
    slideWidth = 176 // fallback
  }
}

function goToSlide(index, animate = true) {
  if (isAnimating.value) return
  index = Math.max(0, Math.min(totalSlides.value - 1, index))
  if (index === currentIndex.value) return
  isAnimating.value = true
  currentIndex.value = index
  const newOffset = -index * slideWidth
  if (!animate) {
    offsetX.value = newOffset
    setTimeout(() => { isAnimating.value = false }, 50)
  } else {
    offsetX.value = newOffset
    setTimeout(() => { isAnimating.value = false }, 300)
  }
}

function nextSlide() {
  let newIndex = currentIndex.value + 1
  if (newIndex >= totalSlides.value) newIndex = 0
  goToSlide(newIndex)
  resetAutoTimer()
}

function prevSlide() {
  let newIndex = currentIndex.value - 1
  if (newIndex < 0) newIndex = totalSlides.value - 1
  goToSlide(newIndex)
  resetAutoTimer()
}

function resetAutoTimer() {
  if (autoTimer) clearInterval(autoTimer)
  autoTimer = setInterval(() => {
    nextSlide()
  }, AUTO_INTERVAL)
}

function pauseAutoTimer() {
  if (autoTimer) clearInterval(autoTimer)
  autoTimer = null
}

function openImage(url) {
  window.open(window.location.origin + url, '_blank')
}

onMounted(() => {
  fetchRandomImages()
  window.addEventListener('resize', () => {
    calcSlideWidth()
    goToSlide(currentIndex.value, false)
  })
  autoTimer = setInterval(() => nextSlide(), AUTO_INTERVAL)
})

onUnmounted(() => {
  if (autoTimer) clearInterval(autoTimer)
})
</script>

<style scoped>
.gallery-section {
  margin: 32px auto;
  max-width: 1000px;
  padding: 0 16px;
}

.gallery-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 20px;
  color: #2563eb;
}
.gallery-header h3 {
  margin: 0;
  font-size: 1.2rem;
  font-weight: 500;
}
.header-icon {
  color: #2563eb;
}

.carousel-container {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  position: relative;
}

.carousel-track-wrapper {
  flex: 1;
  overflow: hidden;
  border-radius: 24px;
}
.carousel-track {
  display: flex;
  gap: 16px;
  transition: transform 0.3s ease-out;
  will-change: transform;
}

.gallery-card {
  flex-shrink: 0;
  width: 180px;
  background: white;
  border-radius: 20px;
  box-shadow: 0 4px 12px rgba(0,0,0,0.04);
  border: 1px solid #eef2f6;
  overflow: hidden;
  cursor: pointer;
  transition: transform 0.2s, box-shadow 0.2s;
}
.gallery-card:hover {
  transform: translateY(-6px);
  box-shadow: 0 16px 28px rgba(0,0,0,0.08);
}
.card-image {
  width: 100%;
  height: 180px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f8fafc;
}
.card-image img {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
}
.card-footer {
  padding: 8px 12px;
  border-top: 1px solid #f0f2f5;
  font-size: 12px;
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}
.tag {
  background: #eef2ff;
  color: #2563eb;
  padding: 2px 8px;
  border-radius: 20px;
  font-size: 11px;
}
.tag-count {
  color: #94a3b8;
  font-size: 11px;
}

.nav-btn {
  background: white;
  border: 1px solid #e2e8f0;
  border-radius: 40px;
  width: 40px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  color: #2563eb;
  transition: all 0.2s;
}
.nav-btn:hover:not(:disabled) {
  background: #eef2ff;
  border-color: #2563eb;
}
.nav-btn:disabled {
  opacity: 0.3;
  cursor: default;
}

.dots {
  display: flex;
  justify-content: center;
  gap: 8px;
  margin-top: 20px;
}
.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #cbd5e1;
  cursor: pointer;
  transition: background 0.2s, width 0.2s;
}
.dot.active {
  background: #2563eb;
  width: 20px;
  border-radius: 4px;
}
.empty {
  text-align: center;
  color: #94a3b8;
  padding: 40px;
}
</style>