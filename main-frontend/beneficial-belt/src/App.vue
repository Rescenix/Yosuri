<template>
  <router-link to="/studio" class="studio-float-btn" title="创作工作台 · 文案成片">
      <span>🎬</span>
    </router-link>
    <router-link to="/sync" class="sync-float-btn" title="多女儿同步工作台">
      <span>👯</span>
    </router-link>
    <router-link to="/company" class="company-float-btn" title="公司 · 多 Agent 协作">
          <span>🏢</span>
        </router-link>
        <router-link to="/ai-write" class="aiwrite-float-btn" title="AI 写作工坊">
          <span>✨</span>
        </router-link>
        <router-link to="/publish" class="publish-float-btn" title="多平台一键发布">
      <span>📚</span>
    </router-link>
    <router-link to="/chat" class="chat-float-btn" title="回到对话">
      <span>💬</span>
    </router-link>
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
.studio-float-btn {
  position: fixed;
  right: 20px;
  bottom: 20px;
  z-index: 9999;
  width: 52px;
  height: 52px;
  border-radius: 50%;
  background: var(--app-accent, #2dd4bf);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 22px;
  text-decoration: none;
  box-shadow: 0 4px 16px rgba(0,0,0,.25);
  transition: transform .15s, box-shadow .15s;
}
.studio-float-btn:hover {
  transform: scale(1.1);
  box-shadow: 0 6px 24px rgba(0,0,0,.35);
}
.publish-float-btn {
  position: fixed;
  right: 20px;
  bottom: 84px;
  z-index: 9999;
  width: 52px;
  height: 52px;
  border-radius: 50%;
  background: var(--app-accent, #4a9eff);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 22px;
  text-decoration: none;
  box-shadow: 0 4px 16px rgba(0,0,0,.25);
  transition: transform .15s, box-shadow .15s;
}
.publish-float-btn:hover {
  transform: scale(1.1);
  box-shadow: 0 6px 24px rgba(0,0,0,.35);
}
.chat-float-btn {
  position: fixed;
  right: 20px;
  top: 20px;
  z-index: 9999;
  width: 52px;
  height: 52px;
  border-radius: 50%;
  background: var(--app-accent, #2dd4bf);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  text-decoration: none;
  box-shadow: 0 4px 16px rgba(0,0,0,.25);
  transition: transform .15s, box-shadow .15s;
}
.chat-float-btn:hover {
  transform: scale(1.1);
  box-shadow: 0 6px 24px rgba(0,0,0,.35);
}
.company-float-btn {
  position: fixed;
  right: 20px;
  bottom: 148px;
  z-index: 9999;
  width: 52px;
  height: 52px;
  border-radius: 50%;
  background: var(--app-accent, #f59e0b);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  text-decoration: none;
  box-shadow: 0 4px 16px rgba(0,0,0,.25);
  transition: transform .15s, box-shadow .15s;
}
.company-float-btn:hover {
  transform: scale(1.1);
  box-shadow: 0 6px 24px rgba(0,0,0,.35);
}
.aiwrite-float-btn {
  position: fixed;
  right: 20px;
  bottom: 212px;
  z-index: 9999;
  width: 52px;
  height: 52px;
  border-radius: 50%;
  background: var(--app-accent, #8b5cf6);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  text-decoration: none;
  box-shadow: 0 4px 16px rgba(0,0,0,.25);
  transition: transform .15s, box-shadow .15s;
}
.aiwrite-float-btn:hover {
  transform: scale(1.1);
  box-shadow: 0 6px 24px rgba(0,0,0,.35);
}
.sync-float-btn {
  position: fixed;
  right: 20px;
  bottom: 276px;
  z-index: 9999;
  width: 52px;
  height: 52px;
  border-radius: 50%;
  background: var(--app-accent, #22c55e);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  text-decoration: none;
  box-shadow: 0 4px 16px rgba(0,0,0,.25);
  transition: transform .15s, box-shadow .15s;
}
.sync-float-btn:hover {
  transform: scale(1.1);
  box-shadow: 0 6px 24px rgba(0,0,0,.35);
}
</style>