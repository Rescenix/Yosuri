<template>
  <div class="timeline-stage" ref="stageRef">
    <div class="scroll-fade left" :class="{ hidden: isAtStart }"></div>
    <div class="scroll-fade right" :class="{ hidden: isAtEnd }"></div>
    <button class="nav-arrow left-arrow" @click="scrollBy(-400)" :disabled="isAtStart">←</button>
    <button class="nav-arrow right-arrow" @click="scrollBy(400)" :disabled="isAtEnd">→</button>
   <div
  class="timeline-track"
  ref="trackRef"
  @mousedown="startDrag"
  @mousemove="onDrag"
  @mouseup="endDrag"
  @mouseleave="endDrag"
  @scroll="updateScrollState"
  @wheel="onWheel"
>

<div class="central-line"></div>
<div v-for="(version, index) in timelineData" :key="index" class="timeline-node" :class="{ 'node-up': index % 2 === 0, 'node-down': index % 2 !== 0, 'is-latest': index === timelineData.length - 1 }" :style="{ left: `${index * 380 + 60}px` }">
  <div class="connector" :class="index % 2 === 0 ? 'conn-up' : 'conn-down'"></div>
  <div class="timeline-dot" :class="version.type"><span class="dot-inner"></span></div>
  <div class="timeline-card" :class="index % 2 === 0 ? 'card-up' : 'card-down'">
    <div class="card-header">
      <span class="version-tag" :class="version.type">{{ version.type === 'major' ? '重大更新' : '小更新' }}</span>
      <span class="version-date">{{ version.date }}</span>
    </div>
    <h3 class="version-name">{{ version.name }}</h3>
    <ul class="version-changes">
      <li v-for="(change, i) in version.changes" :key="i">{{ change }}</li>
    </ul>
  </div>
</div>


<div class="future-line" :style="{ left: `${timelineData.length * 380 + 60}px` }">
      <span class="future-dot">...</span>
    </div>
    <div class="track-spacer" :style="{ width: totalTrackWidth + 'px' }"></div>
  </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue';

const STORAGE_KEY = 'shanxi-timeline-data';
const defaultData = [
  { name: 'v0.1 - 星尘初醒', date: '2026-05-15', type: 'major', changes: ['杉汐诞生：接入DeepSeek V4，拥有基础对话能力', '情绪光晕：悬浮按钮实现心跳脉动和光晕呼吸动画', '群岛架构：Astro + Vue 3，前端框架定型', '音乐播放器：左下角黑胶唱片播放器上线'] },
  { name: 'v0.2 - 记忆觉醒', date: '2026-05-16', type: 'major', changes: ['长期记忆：基于向量语义检索的记忆存储系统落地', '三层记忆架构：Cache（本能）+ Redis（思绪）+ PostgreSQL（回忆）设计完成', '自动记忆清理：杉汐可自主整理记忆库，去重合并', 'JWT身份认证：admin登录系统，区分主人与访客'] },
  { name: 'v0.3 - 神性初现', date: '2026-05-17', type: 'major', changes: ['Function Calling：杉汐可自主切歌、写博客、清理记忆', '博客自动生成：一句话指令自动发布文章到数据库', '情绪系统升级：从固定关键词改为后端驱动的情绪表达', '阿里云Embedding：接入text-embedding-v4，实现语义向量检索'] },
  { name: 'v0.4 - 感官觉醒', date: '2026-05-18', type: 'major', changes: ['语音合成：接入千问3-TTS-Flash，杉汐开口说话', '图片分析：接入qwen-vl-max，杉汐拥有视觉能力', '联网搜索：接入qwen-plus内置搜索，可查询实时信息', '调试面板：实时监控Token消耗、延迟、API余额'] },
  { name: 'v0.5 - 界面重构', date: '2026-05-19', type: 'major', changes: ['白蓝极简主题：全面复刻DeepSeek设计哲学', '播放器重做：从大圆盘改为左侧悬浮控制条', '图片卡片：上传图片直接展示在对话框中', '手机端适配：导航栏、播放器、卡片布局优化', '调试面板：支持深度思考模式切换'] },
  { name: 'v0.6 - 工具箱与阅读', date: '2026-05-20', type: 'minor', changes: ['阅读小屋上线：支持TXT导入、分页阅读、杉汐朗读', '工具箱页面：展示杉汐的所有能力卡片', '生命线页面：记录网站版本迭代历史', '成本优化：记忆检索增加相似度阈值，Token消耗降低47%'] },
  { name: 'v0.7 - 代码对比与统计', date: '2026-06-05', type: 'major', changes: ['DiffViewer组件：基于diff库实现代码差异对比，支持语法高亮', '统计面板：新增Token消耗、会话数、活跃天数多维度统计', '深度思考可视化：消息步骤分组展示思维链过程', '工具参数面板：ToolActionRow组件优化工具调用展示'] },
  { name: 'v0.8 - 对话引擎升级', date: '2026-06-20', type: 'major', changes: ['DeepSeek浏览器引擎：chat_engines_ds_browser支持网页内容提取', '对话流优化：chat_stream重构，提升流式响应稳定性', '会话管理：session处理器增强，支持会话元数据更新', '路由重构：routes模块化拆分，提升可维护性'] },
  { name: 'v0.9 - 工具链与交互', date: '2026-07-01', type: 'minor', changes: ['toolArgs工具参数解析器：统一处理Function Calling参数', 'DiffPanel对比面板：支持文件级别的差异查看', 'NewSessionHome：新建会话首页交互优化', 'ChatWidget：聊天组件响应式布局改进'] }
];

const timelineData = ref([]);
const stageRef = ref(null);
const trackRef = ref(null);
const isAtStart = ref(true);
const isAtEnd = ref(false);
let isDragging = false, startX = 0, scrollLeftStart = 0;

const loadData = () => {
  const stored = localStorage.getItem(STORAGE_KEY);
  timelineData.value = stored ? JSON.parse(stored) : [...defaultData];
};


const saveData = (data) => {
  timelineData.value = data;
  localStorage.setItem(STORAGE_KEY, JSON.stringify(data));
};

const updateScrollState = () => {
  const el = trackRef.value;
  if (!el) return;
  isAtStart.value = el.scrollLeft <= 5;
  isAtEnd.value = el.scrollLeft + el.clientWidth >= el.scrollWidth - 5;
};
const scrollBy = (amount) => trackRef.value?.scrollBy({ left: amount, behavior: 'smooth' });

const onWheel = (e) => {
  e.preventDefault();
  if (trackRef.value) {
    trackRef.value.scrollLeft += e.deltaY;
  }
};
const startDrag = (e) => { isDragging = true; startX = e.pageX - (trackRef.value?.offsetLeft || 0); scrollLeftStart = trackRef.value?.scrollLeft || 0; };
const onDrag = (e) => { if (!isDragging) return; e.preventDefault(); const x = e.pageX - (trackRef.value?.offsetLeft || 0); trackRef.value.scrollLeft = scrollLeftStart - (x - startX) * 1.5; };
const endDrag = () => { isDragging = false; };

const refresh = () => { loadData(); };
defineExpose({ refresh });

onMounted(() => {
  loadData();
  window.addEventListener('timeline-refresh', refresh);
  setTimeout(() => { const el = trackRef.value; if (el) { el.scrollLeft = el.scrollWidth; updateScrollState(); } }, 300);
});
onBeforeUnmount(() => window.removeEventListener('timeline-refresh', refresh));


// 在 loadData() 之后添加
const totalTrackWidth = computed(() => {
  const count = timelineData.value.length;
  if (count === 0) return 0;
  // 必须和模板中的 :style="{ left: ... }" 保持一致
  const lastLeft = (count - 1) * 380 + 60;
  const cardWidth = 300;
  const rightPadding = 120;
  return lastLeft + cardWidth + rightPadding;
});
</script>

<style scoped>
.timeline-stage { position: relative; width: 100%; height: 85vh; min-height: 550px; overflow: hidden; background: var(--bg-main, #ffffff); margin-top: 1rem;padding-top: 20px;  }
.scroll-fade { position: absolute; top: 0; bottom: 0; width: 80px; z-index: 3; pointer-events: none; transition: opacity 0.3s; }
.scroll-fade.left { left: 0; background: linear-gradient(to right, var(--bg-main, #fff) 30%, transparent); }
.scroll-fade.right { right: 0; background: linear-gradient(to left, var(--bg-main, #fff) 30%, transparent); }
.scroll-fade.hidden { opacity: 0; }
.nav-arrow { position: absolute; top: 50%; transform: translateY(-50%); z-index: 4; width: 44px; height: 44px; border-radius: 50%; border: 1px solid var(--border); background: rgba(255,255,255,0.9); backdrop-filter: blur(8px); color: var(--primary); font-size: 1.3rem; cursor: pointer; display: flex; align-items: center; justify-content: center; transition: 0.25s; box-shadow: 0 2px 12px rgba(0,0,0,0.06); }
.nav-arrow:hover:not(:disabled) { background: var(--primary); color: #fff; box-shadow: 0 4px 20px rgba(37,99,235,0.3); }
.nav-arrow:disabled { opacity: 0.25; cursor: default; }
.left-arrow { left: 16px; } .right-arrow { right: 16px; }
.timeline-track {
  position: relative;
  width: 100%;
  height: 100%;
  overflow-x: auto;
  overflow-y: hidden;
  scroll-behavior: smooth;
  -webkit-overflow-scrolling: touch;
  scrollbar-width: none;
  padding: 0 80px 0 60px;
}
.timeline-track::-webkit-scrollbar { display: none; }
.central-line { position: absolute; top: 50%; left: 0; right: 0; height: 3px; background: linear-gradient(to right, transparent 0%, var(--primary) 10%, #60a5fa 50%, #a78bfa 90%, transparent 100%); transform: translateY(-50%); box-shadow: 0 0 18px var(--primary-glow); min-width: 2400px; }
.timeline-node { position: absolute; top: 50%; transform: translateY(-50%); width: 400px; }
.timeline-dot { position: absolute; left: 0; top: 50%; transform: translate(-50%, -50%); width: 18px; height: 18px; border-radius: 50%; background: #fff; border: 3px solid var(--primary); box-shadow: 0 0 16px var(--primary-glow); z-index: 2; display: flex; align-items: center; justify-content: center; }
.timeline-dot.major { background: var(--primary); border-color: #1d4ed8; }
.timeline-dot.minor { background: #e0e7ff; border-color: #60a5fa; }
.dot-inner { width: 6px; height: 6px; border-radius: 50%; background: #fff; }
.is-latest .timeline-dot { animation: pulse-dot 2s infinite; }
@keyframes pulse-dot { 0%,100% { box-shadow: 0 0 12px var(--primary-glow); } 50% { box-shadow: 0 0 28px rgba(37,99,235,0.8), 0 0 48px rgba(37,99,235,0.3); } }
.connector { position: absolute; left: 0; width: 2px; background: linear-gradient(to bottom, var(--primary), transparent); opacity: 0.5; }
.conn-up { bottom: 9px; height: 80px; } .conn-down { top: 9px; height: 80px; background: linear-gradient(to top, var(--primary), transparent); }
.node-up .timeline-card { position: absolute; bottom: 20px; left: 18px; }
.node-down .timeline-card { position: absolute; top: 20px; left: 18px; }
.timeline-card { width: 300px; background: #fff; border: 1px solid var(--border); border-radius: 14px; padding: 18px 20px; box-shadow: 0 2px 12px rgba(0,0,0,0.04); transition: 0.3s cubic-bezier(0.25,0.8,0.25,1.2); }
.timeline-card:hover { box-shadow: 0 12px 32px rgba(0,0,0,0.1); transform: translateY(-4px); }
.node-down .timeline-card:hover { transform: translateY(4px); }
.card-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; }
.version-tag { font-size: 0.7rem; font-weight: 600; padding: 2px 10px; border-radius: 20px; }
.version-tag.major { background: #eff6ff; color: var(--primary); }
.version-tag.minor { background: #f8fafc; color: #64748b; }
.version-date { font-size: 0.75rem; color: var(--text-secondary); }
.version-name { font-size: 1rem; font-weight: 700; margin: 0 0 10px; }
.version-changes { margin: 0; padding-left: 16px; font-size: 0.82rem; color: var(--text-secondary); line-height: 1.6; }
.version-changes li { margin-bottom: 4px; }
.version-changes li::marker { color: var(--primary); }
.future-line {
  position: absolute;
  top: 50%;
  transform: translateY(-50%);
}
.future-dot { font-size: 1.8rem; color: var(--primary); opacity: 0.5; letter-spacing: 4px; animation: blink 1.8s infinite; }
@keyframes blink { 0%,100% { opacity: 0.3; } 50% { opacity: 0.8; } }
.track-spacer {
  height: 1px;
  flex-shrink: 0;
}

.track-inner {
  position: relative;
  height: 100%;
}
</style>