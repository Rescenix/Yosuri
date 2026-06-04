<template>
  <div class="side-panel">
    <div class="search-section">
      <div class="search-box">
        <Icon icon="ph:magnifying-glass" width="16" />
        <input v-model="keyword" placeholder="问问杉汐..." @keyup.enter="doSearch" />
      </div>
      <div v-if="searchLoading" class="search-status">杉汐正在查...</div>
      <div v-else-if="displayedText" class="result-card">
        {{ displayedText }}
        <span v-if="isTyping" class="cursor">|</span>
      </div>
      <div v-else-if="keyword && searchDone && !displayedText" class="search-status">无结果</div>
    </div>

    <div class="annotations-section">
      <h4>杉汐的痕迹</h4>
     <div v-for="(item, idx) in dynamicAnnotations" :key="idx" class="annotation-wrapper">
  <div
    class="annotation-item"
    :class="{ swiped: swipedIndex === idx }"
    @click="handleAnnotationClick(item, idx)"
  >
    <div class="anno-content">
      <!-- ★ 原文在上 -->
      <div class="quote-text">“{{ item.text }}”</div>
      <!-- ★ 批注在下 -->
      <div class="comment-text">{{ item.comment }}</div>
    </div>
    <div class="delete-btn" @click.stop="deleteAnnotation(idx)">
      <Icon icon="ph:trash" width="18" />
    </div>
  </div>
</div>
      <div v-if="dynamicAnnotations.length === 0" class="outline-empty">暂无批注，选中文本即可生成</div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { Icon } from '@iconify/vue'

const props = defineProps({ threeReaderRef: Object })

const keyword = ref('')
const searchLoading = ref(false)
const searchDone = ref(false)
const displayedText = ref('')
const isTyping = ref(false)
let typingTimer = null

const typewriter = (text) => {
  clearInterval(typingTimer)
  displayedText.value = ''
  isTyping.value = true
  let i = 0
  typingTimer = setInterval(() => {
    if (i < text.length) {
      displayedText.value += text[i]
      i++
    } else {
      clearInterval(typingTimer)
      isTyping.value = false
    }
  }, 30)
}

const doSearch = async () => {
  const kw = keyword.value.trim()
  if (!kw) return
  // 防止重复点击
  if (searchLoading.value) return

  clearInterval(typingTimer)
  isTyping.value = false
  searchLoading.value = true
  searchDone.value = false
  displayedText.value = ''

  try {
    const response = await fetch('/api/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        message: `帮我搜索一下“${kw}”`
      })
    })
    if (!response.ok) {
      if (response.status === 429) {
        throw new Error('请求太频繁，请稍后再试')
      }
      throw new Error(`HTTP ${response.status}`)
    }
    const data = await response.json()
    const reply = data.reply || data.message || data.content || '杉汐暂时没有回复'
    typewriter(reply)
  } catch (e) {
    console.error('搜索失败:', e)
    typewriter(e.message || '搜索失败，请稍后重试')
  } finally {
    searchLoading.value = false
    searchDone.value = true
  }
}

const STORAGE_KEY = 'shanxi_annotations'
const dynamicAnnotations = ref([])
const swipedIndex = ref(-1)

function loadAnnotations() {
  dynamicAnnotations.value = JSON.parse(localStorage.getItem(STORAGE_KEY) || '[]')
}

function jumpToPage(pageIndex) {
  if (props.threeReaderRef?.flipToPhysicalPage) {
    props.threeReaderRef.flipToPhysicalPage(pageIndex)
  }
}

function handleAnnotationClick(item, idx) {
  if (swipedIndex.value === idx) {
    jumpToPage(item.page)
    swipedIndex.value = -1
  } else {
    swipedIndex.value = idx
  }
}

function deleteAnnotation(idx) {
  dynamicAnnotations.value.splice(idx, 1)
  localStorage.setItem(STORAGE_KEY, JSON.stringify(dynamicAnnotations.value))
  swipedIndex.value = -1
}

function handleSearchEvent(e) {
  keyword.value = e.detail.text
  doSearch()
}

onMounted(() => {
  loadAnnotations()
  window.addEventListener('annotations-updated', loadAnnotations)
  window.addEventListener('search-text', handleSearchEvent)
})

onBeforeUnmount(() => {
  window.removeEventListener('annotations-updated', loadAnnotations)
  window.removeEventListener('search-text', handleSearchEvent)
})
</script>

<style scoped>
.side-panel { display: flex; flex-direction: column; gap: 20px; padding: 16px; }
.search-box { display: flex; align-items: center; background: #f1f5f9; border-radius: 8px; padding: 6px 10px; }
.search-box input { border: none; background: transparent; outline: none; flex: 1; margin-left: 6px; font-size: 0.9rem; }
.search-status { font-size: 0.8rem; color: #64748b; margin-top: 8px; }
.result-card { margin-top: 8px; background: #fff; border-radius: 12px; padding: 12px 14px; box-shadow: 0 2px 8px rgba(0,0,0,0.06); font-size: 0.9rem; line-height: 1.6; color: #1e293b; white-space: pre-wrap; word-wrap: break-word; }
.cursor { color: #60a5fa; animation: blink 0.8s infinite; }
@keyframes blink { 0%,100% { opacity: 1; } 50% { opacity: 0; } }

.annotation-wrapper { position: relative; overflow: hidden; margin-bottom: 6px; border-radius: 8px; }

.annotation-item {
  display: flex;
  align-items: center;
  transition: transform 0.25s ease;
  transform: translateX(0);
  padding: 10px 12px;
  background: #fff;
  cursor: pointer;
  position: relative;
  z-index: 2;
  writing-mode: horizontal-tb !important; /* 强制横排 */
}
.annotation-item.swiped { transform: translateX(-36px); }

.anno-content {
  flex: 1;
  display: flex;
  flex-direction: column;  /* 原文在上，批注在下 */
  gap: 4px;
  min-width: 0;
  padding-right: 8px;
}

.quote-text {
  font-size: 0.85rem;
  color: #64748b;
  font-style: italic;
  line-height: 1.5;
  word-break: break-word;      /* 允许长文本换行 */
  white-space: normal;         /* 移除 nowrap */
  overflow: visible;           /* 允许完整显示 */
  writing-mode: horizontal-tb !important;
}

.comment-text {
  font-size: 0.85rem;
  color: #334155;
  line-height: 1.5;
  word-break: break-word;
  writing-mode: horizontal-tb !important;
}

.delete-btn {
  position: absolute;
  right: -36px;
  top: 0;
  bottom: 0;
  width: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #fee2e2;
  color: #ef4444;
  cursor: pointer;
  border-radius: 0 8px 8px 0;
  transition: right 0.25s ease;
  z-index: 1;
}
.annotation-item.swiped ~ .delete-btn { right: 0; }

.outline-empty { padding: 20px; text-align: center; color: var(--text-secondary); }
</style>