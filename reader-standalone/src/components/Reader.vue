<template>
  <div class="reader-root">
    <!-- 空状态：上传书籍 -->
    <div v-if="!bookLoaded" class="upload-area">
      <div class="upload-card">
        <div class="upload-icon">
          <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="var(--primary)" stroke-width="1.5">
            <path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"/>
            <path d="M6.5 17A2.5 2.5 0 0 1 4 14.5V5a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2v9.5"/>
            <path d="M12 7v6m-3-3 3-3 3 3" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
        </div>
        <h2>阅读小屋</h2>
        <p>上传一本 TXT 文件，开始阅读</p>
        <label class="upload-btn">
          选择文件
          <input type="file" accept=".txt" @change="loadBook" hidden />
        </label>
      </div>
    </div>

    <!-- 阅读状态 -->
    <template v-else>
      <!-- 工具栏 -->
      <div class="toolbar">
        <button class="tb-btn" @click="backToList" title="返回">← 退出</button>
        <span class="tb-title">{{ bookTitle }}</span>
        <div class="tb-right">
          <!-- 字体调节 -->
          <div class="font-controls">
            <button class="tb-btn" @click="changeFontSize(-1)" :disabled="fontSizeIndex <= 0">A-</button>
            <span class="font-label">{{ fontSizes[fontSizeIndex] }}</span>
            <button class="tb-btn" @click="changeFontSize(1)" :disabled="fontSizeIndex >= fontSizes.length - 1">A+</button>
          </div>
        </div>
      </div>

      <!-- 阅读区域 -->
      <div 
        class="reader-area"
        ref="readerAreaRef"
        @click="handleClick"
        @touchstart="handleTouchStart"
        @touchend="handleTouchEnd"
      >
        <div class="page-content" :style="{ fontSize: currentFontSize + 'px' }">
          <p v-if="currentPageContent">{{ currentPageContent }}</p>
          <p v-else class="empty-page">— 本页无内容 —</p>
        </div>
        <div class="page-indicator">
          {{ currentPage + 1 }} / {{ totalPages }}
        </div>
      </div>

      <!-- 翻页提示区（桌面端左右点击） -->
      <div class="nav-zones">
        <div class="nav-zone left" @click="prevPage"></div>
        <div class="nav-zone right" @click="nextPage"></div>
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue';

// ==================== 状态 ====================
const bookLoaded = ref(false);
const bookTitle = ref('');
const fullText = ref('');
const pages = ref([]);
const currentPage = ref(0);
const fontSizeIndex = ref(1); // 默认中号
const fontSizes = [14, 18, 22, 26];
const currentFontSize = computed(() => fontSizes[fontSizeIndex.value]);
const readerAreaRef = ref(null);

// 触摸滑动
let touchStartX = 0;
let touchStartY = 0;

// ==================== 计算属性 ====================
const totalPages = computed(() => pages.value.length);
const currentPageContent = computed(() => pages.value[currentPage.value] || '');

// ==================== 加载书籍 ====================
const loadBook = (e) => {
  const file = e.target.files[0];
  if (!file) return;
  
  bookTitle.value = file.name.replace(/\.txt$/i, '');
  
  const reader = new FileReader();
  reader.onload = (ev) => {
    let text = ev.target.result;
    
    // 尝试 GBK 检测并转换（简单处理，后续可加固）
    if (text.includes('�')) {
      // 可能是 GBK 被当成 UTF-8 读了，先用默认结果
      console.warn('文件可能非 UTF-8 编码，部分字符显示可能异常');
    }
    
    fullText.value = text;
    buildPages();
    restoreProgress(file.name);
    bookLoaded.value = true;
  };
  
  reader.readAsText(file, 'UTF-8');
};

// ==================== 分页逻辑 ====================
const buildPages = () => {
  const text = fullText.value;
  if (!text.trim()) {
    pages.value = [''];
    return;
  }
  
  // 测量可容纳字符数
  const charPerPage = estimateCharsPerPage();
  
  const result = [];
  let start = 0;
  
  while (start < text.length) {
    let end = start + charPerPage;
    
    if (end >= text.length) {
      result.push(text.slice(start));
      break;
    }
    
    // 尽量在换行符处分页
    const segment = text.slice(start, end + 1);
    const lastNewline = segment.lastIndexOf('\n');
    
    if (lastNewline > 0 && lastNewline > charPerPage * 0.6) {
      end = start + lastNewline;
    } else {
      // 在空格处分页（英文）
      const lastSpace = segment.lastIndexOf(' ');
      if (lastSpace > charPerPage * 0.7) {
        end = start + lastSpace;
      }
    }
    
    result.push(text.slice(start, end).trim());
    start = end;
  }
  
  pages.value = result;
  currentPage.value = 0;
};

const estimateCharsPerPage = () => {
  // 基础估算：根据字体大小和容器尺寸
  const area = readerAreaRef.value;
  if (!area) return 1500;
  
  const width = area.clientWidth - 64; // 减去 padding
  const height = area.clientHeight - 80; // 减去页码指示器
  
  const charWidth = currentFontSize.value * 0.6; // 中文字符近似宽度
  const lineHeight = currentFontSize.value * 1.8;
  
  const charsPerLine = Math.floor(width / charWidth);
  const linesPerPage = Math.floor(height / lineHeight);
  
  return Math.max(500, charsPerLine * linesPerPage - 10); // 留一点余量
};

// ==================== 翻页 ====================
const nextPage = () => {
  if (currentPage.value < totalPages.value - 1) {
    currentPage.value++;
    saveProgress();
  }
};

const prevPage = () => {
  if (currentPage.value > 0) {
    currentPage.value--;
    saveProgress();
  }
};

const handleClick = (e) => {
  const area = readerAreaRef.value;
  if (!area) return;
  const rect = area.getBoundingClientRect();
  const x = e.clientX - rect.left;
  const mid = rect.width / 2;
  
  if (x < mid) {
    prevPage();
  } else {
    nextPage();
  }
};

const handleTouchStart = (e) => {
  touchStartX = e.touches[0].clientX;
  touchStartY = e.touches[0].clientY;
};

const handleTouchEnd = (e) => {
  const dx = e.changedTouches[0].clientX - touchStartX;
  const dy = e.changedTouches[0].clientY - touchStartY;
  
  // 水平滑动超过 50px 且大于垂直滑动
  if (Math.abs(dx) > 50 && Math.abs(dx) > Math.abs(dy)) {
    if (dx < 0) {
      nextPage();
    } else {
      prevPage();
    }
  }
};

// ==================== 字体调节 ====================
const changeFontSize = (delta) => {
  const newIndex = fontSizeIndex.value + delta;
  if (newIndex >= 0 && newIndex < fontSizes.length) {
    fontSizeIndex.value = newIndex;
    nextTick(() => {
      buildPages();
      if (currentPage.value >= totalPages.value) {
        currentPage.value = totalPages.value - 1;
      }
      saveProgress();
    });
  }
};

// ==================== 进度保存 ====================
const STORAGE_KEY = 'reading-hut-progress';

const saveProgress = () => {
  const data = {
    bookId: bookTitle.value,
    page: currentPage.value,
    fontSizeIndex: fontSizeIndex.value,
    timestamp: Date.now()
  };
  localStorage.setItem(STORAGE_KEY, JSON.stringify(data));
};

const restoreProgress = (fileName) => {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return;
    const data = JSON.parse(raw);
    if (data.bookId === fileName) {
      currentPage.value = Math.min(data.page, totalPages.value - 1);
      fontSizeIndex.value = data.fontSizeIndex ?? 1;
    }
  } catch (e) {
    // ignore
  }
};

const backToList = () => {
  bookLoaded.value = false;
  pages.value = [];
  fullText.value = '';
  currentPage.value = 0;
};

// ==================== 键盘翻页 ====================
const onKeyDown = (e) => {
  if (!bookLoaded.value) return;
  if (e.key === 'ArrowLeft') prevPage();
  if (e.key === 'ArrowRight') nextPage();
};

onMounted(() => window.addEventListener('keydown', onKeyDown));
onUnmounted(() => window.removeEventListener('keydown', onKeyDown));
</script>

<style scoped>
.reader-root {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 80px);
  max-height: 800px;
  min-height: 500px;
}

/* ==================== 上传区 ==================== */
.upload-area {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}
.upload-card {
  text-align: center;
  padding: 48px;
  border: 2px dashed var(--border, #e2e8f0);
  border-radius: 16px;
  transition: border-color 0.2s;
}
.upload-card:hover {
  border-color: var(--primary, #2563eb);
}
.upload-icon {
  margin-bottom: 16px;
}
.upload-card h2 {
  font-size: 1.4rem;
  font-weight: 700;
  color: var(--text-primary, #0f172a);
  margin: 0 0 8px;
}
.upload-card p {
  font-size: 0.9rem;
  color: var(--text-secondary, #64748b);
  margin: 0 0 24px;
}
.upload-btn {
  display: inline-block;
  padding: 10px 28px;
  background: var(--primary, #2563eb);
  color: #fff;
  border-radius: 8px;
  font-size: 0.9rem;
  cursor: pointer;
  transition: background 0.2s;
}
.upload-btn:hover {
  background: var(--primary-hover, #1d4ed8);
}

/* ==================== 工具栏 ==================== */
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 0;
  border-bottom: 1px solid var(--border, #e2e8f0);
  flex-shrink: 0;
}
.tb-title {
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--text-primary, #0f172a);
  text-align: center;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 40%;
}
.tb-right {
  display: flex;
  align-items: center;
  gap: 8px;
}
.font-controls {
  display: flex;
  align-items: center;
  gap: 6px;
}
.font-label {
  font-size: 0.75rem;
  color: var(--text-secondary, #64748b);
  min-width: 24px;
  text-align: center;
}
.tb-btn {
  padding: 6px 12px;
  background: var(--bg-card, #f8fafc);
  border: 1px solid var(--border, #e2e8f0);
  border-radius: 6px;
  font-size: 0.8rem;
  color: var(--text-primary, #0f172a);
  cursor: pointer;
  transition: all 0.15s;
  white-space: nowrap;
}
.tb-btn:hover:not(:disabled) {
  border-color: var(--primary, #2563eb);
  color: var(--primary, #2563eb);
}
.tb-btn:disabled {
  opacity: 0.4;
  cursor: default;
}

/* ==================== 阅读区域 ==================== */
.reader-area {
  flex: 1;
  position: relative;
  display: flex;
  flex-direction: column;
  justify-content: center;
  padding: 32px;
  user-select: none;
  cursor: default;
  min-height: 0;
}
.page-content {
  flex: 1;
  overflow: hidden;
  line-height: 1.8;
  color: var(--text-primary, #0f172a);
}
.empty-page {
  color: var(--text-secondary, #64748b);
  font-style: italic;
}
.page-indicator {
  text-align: center;
  font-size: 0.75rem;
  color: var(--text-secondary, #64748b);
  padding-top: 12px;
  flex-shrink: 0;
}

/* ==================== 翻页区域（桌面） ==================== */
.nav-zones {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  display: flex;
  pointer-events: none;
}
.nav-zone {
  flex: 1;
  pointer-events: auto;
}
</style>