<template>
  <div class="reader-root">
    <div v-if="reader.loading.value" class="status-msg">加载中...</div>
    <div v-else-if="reader.error.value" class="status-msg error">{{ reader.error.value }}</div>

    <template v-else>
      <div class="toolbar"  >



        <button class="tb-btn" @click="back">← 书架</button>
        
        <div class="tb-actions">



  <!-- 移动端专用：读书笔记和杉汐痕迹按钮 -->
    <button v-if="isMobile" class="tb-btn" @click="openMobilePanel('notes')">
      <Icon icon="ph:notebook" width="18" />
    </button>
    <button v-if="isMobile" class="tb-btn" @click="openMobilePanel('sidebar')">
      <Icon icon="ph:list" width="18" />
    </button>



          <button class="tb-btn" @click="toggleBookmarkAtCurrentPage">
            <Icon :icon="isCurrentPageBookmarked ? 'ph:bookmark-simple-fill' : 'ph:bookmark-simple'" width="18" />
          </button>
          <button class="tb-btn" @click="reader.changeFont()">{{ reader.fontSize.value }}px</button>
          <button class="tb-btn" @click="showOutline = !showOutline">
            <Icon icon="ph:list-bullets" width="18" />
          </button>
        </div>
      </div>

      <div class="reader-body">
        <div class="left-spacer"></div>
        <div class="reader-card">
          <div class="three-reader-wrapper">
            <ThreeReader ref="threeReaderRef" :reader="reader" />
          </div>
        </div>
        <div class="right-panels">
          <NotesPanel />
          <div class="side-panel">
            <SidePanel :threeReaderRef="threeReaderRef" />
          </div>
        </div>
      </div>

      <!-- 目录浮层 -->
      <transition name="outline-fade">
        <div v-if="showOutline" class="outline-overlay" @click.self="showOutline = false">
          <div class="outline-panel">
            <div class="outline-header">
              <div class="outline-tabs">
                <button :class="['tab-btn', { active: outlineTab === 'outline' }]" @click="outlineTab = 'outline'">目录</button>
                <button :class="['tab-btn', { active: outlineTab === 'bookmarks' }]" @click="outlineTab = 'bookmarks'">书签</button>
              </div>
              <button class="tb-btn" @click="showOutline = false">
                <Icon icon="ph:x" width="18" />
              </button>
            </div>

            <!-- 目录列表 -->
            <div v-show="outlineTab === 'outline'" class="outline-list">
              <div v-for="(item, idx) in outline" :key="idx" class="outline-item" @click="jumpToChapter(item)">
                {{ item.title }}
              </div>
              <div v-if="outline.length === 0" class="outline-empty">未识别到章节标题</div>
            </div>

            <!-- 书签列表 -->
            <div v-show="outlineTab === 'bookmarks'" class="outline-list">
              <div v-for="(bm, idx) in reader.bookmarks.value" :key="idx" class="outline-item" @click="jumpToBookmark(bm.page)">
                <span class="bm-text">{{ bm.text }}</span>
                <span class="bm-page">第{{ bm.page + 1 }}页</span>
              </div>
              <div v-if="reader.bookmarks.value.length === 0" class="outline-empty">暂无书签</div>
            </div>
          </div>
        </div>
      </transition>
    </template>

 <!-- 移动端面板浮层（从顶部滑出） -->
<transition name="slide-down">
  <div v-if="isMobile && activeMobilePanel" class="mobile-panel-overlay" @click.self="activeMobilePanel = null">
    <div class="mobile-panel">
      <div class="mobile-panel-header">
        <span>{{ activeMobilePanel === 'notes' ? '读书笔记' : '杉汐的痕迹' }}</span>
       <button class="tb-btn" @click="activeMobilePanel = null">
  <Icon icon="ph:x" width="18" />
</button>
      </div>
      <div class="mobile-panel-content">
        <NotesPanel v-if="activeMobilePanel === 'notes'" />
        <SidePanel v-else :threeReaderRef="threeReaderRef" />
      </div>
    </div>
  </div>
</transition>
  </div>
</template>
<script setup>
import { ref, onMounted, nextTick, computed, onBeforeUnmount } from 'vue'
import { Icon } from '@iconify/vue'
import { useReader } from './useReader.js'
import ThreeReader from './ThreeReader.vue'
import SidePanel from './SidePanel.vue'
import NotesPanel from './NotesPanel.vue'



const activeMobilePanel = ref(null) // 'notes' | 'sidebar' | null

const reader = useReader()
const threeReaderRef = ref(null)
const showOutline = ref(false)
const outline = ref([])
const outlineTab = ref('outline')
let panelTimer = null
function openMobilePanel(type) {
  if (panelTimer) return
  panelTimer = setTimeout(() => {
    activeMobilePanel.value = activeMobilePanel.value === type ? null : type
    panelTimer = null
  }, 100)
}

// ============ 移动端检测（顶层执行） ============
const isMobile = ref(window.innerWidth <= 768)
let mediaQuery = null

// 在组件挂载时添加监听
onMounted(() => {
  mediaQuery = window.matchMedia('(max-width: 768px)')
  isMobile.value = mediaQuery.matches
  const handler = (e) => { isMobile.value = e.matches }
  mediaQuery.addEventListener('change', handler)
  
  // 存储 handler 引用，以便移除
  mediaQuery._handler = handler
})

// 在组件卸载时移除监听
onBeforeUnmount(() => {
  if (mediaQuery && mediaQuery._handler) {
    mediaQuery.removeEventListener('change', mediaQuery._handler)
  }
})


function parseOutline(fullText) {
  const lines = fullText.split('\n')
  const chapterPattern = /^(第[一二三四五六七八九十百千\d]+[卷章节回部篇]|序章|尾声|楔子|前言|后记|[Pp]art\s+\d+|Chapter\s+\d+|第[0-9]+[章節回]|[零壹贰叁肆伍陆柒捌玖拾佰仟\d]+[、．.]\s*)([ 　\t].*)?$/u
  const result = []
  lines.forEach((line, idx) => {
    const trimmed = line.trim()
    if (chapterPattern.test(trimmed)) {
      result.push({ title: trimmed, lineIndex: idx })
    }
  })
  return result
}

const currentPage = computed(() => threeReaderRef.value?.currentPage ?? 0)

const isCurrentPageBookmarked = computed(() => {
  return reader.isBookmarked(currentPage.value)
})

const toggleBookmarkAtCurrentPage = () => {
  const page = currentPage.value
  const text = threeReaderRef.value?.getCurrentPageText?.() || `第${page + 1}页`
  reader.toggleBookmark(page, text)
}

const jumpToBookmark = (page) => {
  if (threeReaderRef.value?.flipToPage) {
    threeReaderRef.value.flipToPage(page)
  }
  showOutline.value = false
}

const jumpToChapter = (item) => {
  if (threeReaderRef.value && threeReaderRef.value.jumpToChapter) {
    threeReaderRef.value.jumpToChapter(item.title)
  }
  showOutline.value = false
}

onMounted(async () => {
  try {
    const params = new URLSearchParams(window.location.search)
    const file = params.get('book')
    if (!file) throw new Error('未指定书籍')
    reader.title.value = file.replace(/\.txt$/i, '')

    const res = await fetch(`/api/book/content?bookId=${encodeURIComponent(file)}`)
    if (!res.ok) throw new Error('书籍加载失败')
    const text = await res.text()
    reader.fullText.value = text
    await nextTick()
    outline.value = parseOutline(text)
    reader.restoreProgress()
  } catch (e) {
    reader.error.value = e.message
  } finally {
    reader.loading.value = false
  }
})

const back = async () => {
  if (threeReaderRef.value?.flipToCoverAnimated) {
    await threeReaderRef.value.flipToCoverAnimated()
  }
  window.location.href = '/reading-hut'
}
</script>

<style>
@import './ReaderView.css';
</style>