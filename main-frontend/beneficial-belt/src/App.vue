<template>
  <!-- 右侧统一工具条（聊天工具组件风格：方形圆角图标 + 文字标签） -->
  <nav class="app-tool-rail">
    <router-link to="/chat" class="app-tool-btn" title="对话" active-class="active">
      <span class="tool-ico">💬</span>
      <span class="tool-lbl">对话</span>
    </router-link>
    <router-link to="/sync" class="app-tool-btn" title="部门协同工作台" active-class="active">
      <span class="tool-ico">👥</span>
      <span class="tool-lbl">协同</span>
    </router-link>
    <router-link to="/company" class="app-tool-btn" title="公司管理" active-class="active">
      <span class="tool-ico">🏢</span>
      <span class="tool-lbl">公司</span>
    </router-link>
    <router-link to="/ai-write" class="app-tool-btn" title="AI 女儿们写小说" active-class="active">
      <span class="tool-ico">✨</span>
      <span class="tool-lbl">写作</span>
    </router-link>
    <router-link to="/publish" class="app-tool-btn" title="多平台一键发布" active-class="active">
      <span class="tool-ico">📚</span>
      <span class="tool-lbl">发布</span>
    </router-link>
    <router-link to="/studio" class="app-tool-btn" title="创作工作台" active-class="active">
      <span class="tool-ico">🎬</span>
      <span class="tool-lbl">工作台</span>
    </router-link>
  </nav>
  <router-view />
  <UpdateModal v-if="showUpdate" :update="updateInfo" @close="showUpdate = false" />
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useAuth } from './composables/useAuth.js'
import { getSkippedVersion, isUpdateNotifyDisabled } from './composables/updatePrefs.js'
import UpdateModal from './components/shanxi/chat/UpdateModal.vue'

const auth = useAuth()
const showUpdate = ref(false)
const updateInfo = ref(null)

onMounted(() => {
  const params = new URLSearchParams(window.location.search)
  const token = params.get('token')
  if (token) {
    const url = new URL(window.location.href)
    url.searchParams.delete('token')
    window.history.replaceState({}, document.title, url.pathname + url.search)
    localStorage.setItem('token', token)
    window.dispatchEvent(new Event('auth-change'))
  }
})

onMounted(async () => {
  try {
    if (isUpdateNotifyDisabled()) return
    const res = await fetch('/api/update/check')
    const data = await res.json()
    if (data.ok && data.update?.has_update) {
      if (getSkippedVersion() === data.update.latest_version) return
      updateInfo.value = data.update
      showUpdate.value = true
    }
  } catch {}
})
</script>

<style>
/* 右侧统一工具条（聊天工具组件风格） */
.app-tool-rail {
  position: fixed;
  right: 12px;
  top: 50%;
  transform: translateY(-50%);
  z-index: 9999;
  display: flex;
  flex-direction: column;
  gap: 6px;
  background: var(--app-surface-2, rgba(20,20,40,.85));
  border: 1px solid var(--app-border, rgba(255,255,255,.1));
  border-radius: 12px;
  padding: 6px;
  backdrop-filter: blur(12px);
  box-shadow: 0 8px 30px rgba(0,0,0,.3);
}
.app-tool-btn {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  border: none;
  background: transparent;
  color: var(--app-text-faint, #94a3b8);
  border-radius: 8px;
  padding: 6px 8px;
  width: 52px;
  text-decoration: none;
  transition: all .15s;
}
.app-tool-btn:hover { background: var(--app-surface-3, rgba(255,255,255,.1)); color: var(--app-text, #fff); }
.app-tool-btn.active { background: var(--app-accent-soft, rgba(139,92,246,.25)); color: var(--app-accent, #a78bfa); }
.tool-ico { font-size: 18px; }
.tool-lbl { font-size: 10px; }
</style>