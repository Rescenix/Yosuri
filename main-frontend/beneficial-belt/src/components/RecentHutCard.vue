<template>
  <div class="recent-hut-card">
    <div v-if="recentItem" class="recent-card" @click="goTo(recentItem.link)">
      <div class="recent-icon"><Icon :icon="recentItem.icon" width="28" color="var(--primary)" /></div>
      <div class="recent-content">
        <div class="recent-label">最近使用</div>
        <div class="recent-title">{{ recentItem.title }}</div>
      </div>
      <button class="recent-btn">继续</button>
    </div>
    <div v-else class="recent-placeholder">
      <span>点击任意工具卡片，将在此显示</span>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue';
import { Icon } from '@iconify/vue';

const recentItem = ref(null);
const STORAGE_KEY = 'recent_hut';

// 从 localStorage 加载并更新状态
function loadFromStorage() {
  const saved = localStorage.getItem(STORAGE_KEY);
  if (saved) {
    try {
      const parsed = JSON.parse(saved);
      // 可选：检查时效（7天）
      if (Date.now() - parsed.timestamp < 7 * 24 * 3600 * 1000) {
        recentItem.value = parsed;
        console.log('[RecentHutCard] 从存储加载:', parsed.title);
      } else {
        localStorage.removeItem(STORAGE_KEY);
        recentItem.value = null;
      }
    } catch (e) {}
  } else {
    recentItem.value = null;
  }
}

// 保存到 localStorage（供其他页面写入时使用，但此处不主动写入）
function saveToStorage(item) {
  const toStore = {
    title: item.title,
    link: item.link,
    icon: item.icon,
    timestamp: Date.now(),
  };
  localStorage.setItem(STORAGE_KEY, JSON.stringify(toStore));
  recentItem.value = toStore;
  console.log('[RecentHutCard] 已保存到存储:', toStore.title);
}

// 监听跨页面的 storage 事件（同一浏览器不同标签页/页面）
function handleStorageChange(e) {
  if (e.key === STORAGE_KEY) {
    console.log('[RecentHutCard] 检测到跨页面存储变化');
    loadFromStorage();
  }
}

onMounted(() => {
  loadFromStorage();
  window.addEventListener('storage', handleStorageChange);
  // 也可以监听自定义事件（同页面内直接调用），但非必须，保留以便同页面内实时更新
  window.addEventListener('recent-hut-update', (e) => {
    console.log('[RecentHutCard] 收到同页面事件，更新存储');
    if (e.detail && e.detail.title) {
      saveToStorage(e.detail);
    }
  });
});

onUnmounted(() => {
  window.removeEventListener('storage', handleStorageChange);
});

const goTo = (link) => {
  if (link.startsWith('http')) {
    window.open(link, '_blank');
  } else {
    window.location.href = link;
  }
};
</script>

<style scoped>
/* 样式不变，省略 ... */
.recent-hut-card { width: 100%; margin-bottom: 24px; }
.recent-card { display: flex; align-items: center; gap: 16px; background: #f8fafc; border: 1px solid #e2e8f0; border-radius: 20px; padding: 16px 20px; cursor: pointer; transition: all 0.2s ease; }
.recent-card:hover { border-color: #2563eb; box-shadow: 0 2px 8px rgba(37,99,235,0.08); }
.recent-icon { flex-shrink: 0; }
.recent-content { flex: 1; }
.recent-label { font-size: 0.7rem; color: #64748b; text-transform: uppercase; letter-spacing: 0.5px; }
.recent-title { font-weight: 600; color: #0f172a; }
.recent-btn { background: #2563eb; color: white; border: none; border-radius: 30px; padding: 6px 16px; font-size: 0.75rem; cursor: pointer; }
.recent-placeholder { padding: 20px; text-align: center; background: #f8fafc; border-radius: 20px; color: #64748b; font-size: 0.85rem; border: 1px dashed #cbd5e1; }
</style>