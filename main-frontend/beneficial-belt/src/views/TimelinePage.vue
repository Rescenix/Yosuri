<template>
  <div>
    <NavBar />
    <div class="blog-container">
      <TimelineView id="timeline-view" />
      <button v-if="canEdit" class="fab-edit-btn" @click="toggleEditor">✎</button>
    </div>

    <TimelineEditor
      v-if="editorOpen"
      :initialData="editorData"
      @close="closeEditor"
      @saved="onSaved"
    />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import NavBar from '../components/NavBar.vue'
import TimelineView from '../components/timeline/Timeline.vue'
import TimelineEditor from '../components/timeline/TimelineEditor.vue'

const canEdit = ref(false)
const editorOpen = ref(false)

const defaultTimelineData = [
  { name: 'v0.1 - 星尘初醒', date: '2026-05-15', type: 'major', changes: ['杉汐诞生：接入DeepSeek V4，拥有基础对话能力', '情绪光晕：悬浮按钮实现心跳脉动和光晕呼吸动画', '群岛架构：Astro + Vue 3，前端框架定型', '音乐播放器：左下角黑胶唱片播放器上线'] },
  { name: 'v0.2 - 记忆觉醒', date: '2026-05-16', type: 'major', changes: ['长期记忆：基于向量语义检索的记忆存储系统落地', '三层记忆架构：Cache（本能）+ Redis（思绪）+ PostgreSQL（回忆）设计完成', '自动记忆清理：杉汐可自主整理记忆库，去重合并', 'JWT身份认证：admin登录系统，区分主人与访客'] },
  { name: 'v0.3 - 神性初现', date: '2026-05-17', type: 'major', changes: ['Function Calling：杉汐可自主切歌、写博客、清理记忆', '博客自动生成：一句话指令自动发布文章到数据库', '情绪系统升级：从固定关键词改为后端驱动的情绪表达', '阿里云Embedding：接入text-embedding-v4，实现语义向量检索'] },
  { name: 'v0.4 - 感官觉醒', date: '2026-05-18', type: 'major', changes: ['语音合成：接入千问3-TTS-Flash，杉汐开口说话', '图片分析：接入qwen-vl-max，杉汐拥有视觉能力', '联网搜索：接入qwen-plus内置搜索，可查询实时信息', '调试面板：实时监控Token消耗、延迟、API余额'] },
  { name: 'v0.5 - 界面重构', date: '2026-05-19', type: 'major', changes: ['白蓝极简主题：全面复刻DeepSeek设计哲学', '播放器重做：从大圆盘改为左侧悬浮控制条', '图片卡片：上传图片直接展示在对话框中', '手机端适配：导航栏、播放器、卡片布局优化', '调试面板：支持深度思考模式切换'] },
  { name: 'v0.6 - 工具箱与阅读', date: '2026-05-20', type: 'minor', changes: ['阅读小屋上线：支持TXT导入、分页阅读、杉汐朗读', '工具箱页面：展示杉汐的所有能力卡片', '生命线页面：记录网站版本迭代历史', '成本优化：记忆检索增加相似度阈值，Token消耗降低47%'] },
]

function getInitialData() {
  const stored = localStorage.getItem('shanxi-timeline-data')
  if (stored) return JSON.parse(stored)
  return defaultTimelineData
}

const editorData = ref(getInitialData())

function toggleEditor() {
  if (editorOpen.value) {
    closeEditor()
    return
  }
  editorData.value = getInitialData()
  editorOpen.value = true
}

function closeEditor() {
  editorOpen.value = false
}

function onSaved(newData) {
  localStorage.setItem('shanxi-timeline-data', JSON.stringify(newData))
  window.dispatchEvent(new CustomEvent('timeline-refresh'))
  closeEditor()
}

onMounted(() => {
  document.title = '更新日志 | Aurora'
  canEdit.value = !!localStorage.getItem('token')
})
</script>

<style scoped>
.blog-container {
  max-width: 100%;
  margin: 0 auto;
  padding: 20px 20px 40px;
}
.fab-edit-btn {
  position: fixed;
  bottom: 30px;
  right: 30px;
  width: 48px;
  height: 48px;
  border-radius: 50%;
  background: var(--primary, #2563eb);
  color: white;
  border: none;
  box-shadow: 0 4px 12px rgba(37, 99, 235, 0.4);
  font-size: 1.2rem;
  cursor: pointer;
  z-index: 15;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: transform 0.2s, box-shadow 0.2s;
}
.fab-edit-btn:hover {
  transform: scale(1.08);
  box-shadow: 0 6px 18px rgba(37, 99, 235, 0.6);
}
</style>
