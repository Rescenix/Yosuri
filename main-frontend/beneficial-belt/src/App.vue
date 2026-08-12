<template>
  <!-- 右侧统一工具条（iconify 图标 + 亮色） -->
  <nav class="app-tool-rail">
    <router-link to="/chat" class="app-tool-btn" title="对话" active-class="active">
      <Icon icon="mdi:chat" width="20" />
      <span class="tool-lbl">对话</span>
    </router-link>
    <router-link to="/company" class="app-tool-btn" title="公司目标与协作" active-class="active">
      <Icon icon="mdi:target-arrow" width="20" />
      <span class="tool-lbl">目标</span>
    </router-link>
    <router-link to="/publish" class="app-tool-btn" title="多平台一键发布" active-class="active">
      <Icon icon="mdi:rocket-launch" width="20" />
      <span class="tool-lbl">发布</span>
    </router-link>
    <router-link to="/studio" class="app-tool-btn" title="创作工作台" active-class="active">
      <Icon icon="mdi:movie-open" width="20" />
      <span class="tool-lbl">工作台</span>
    </router-link>
  </nav>
  <router-view />
  <UpdateModal v-if="showUpdate" :update="updateInfo" @close="showUpdate = false" />
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { Icon } from '@iconify/vue'
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
      // 第一次进应用：静默后台下载安装包（用户无感知、不弹窗）。
      // 下完本次不打扰；下次启动本地已有安装包 → 直接弹「一键安装」。
      try {
        const dl = await fetch('/api/update/download', { method: 'POST' })
        const dlData = await dl.json()
        if (dlData.state === 'done') {
          // 安装包已就绪（上次启动已下载完）→ 本次弹一键安装
          showUpdate.value = true
          return
        }
        // 本次开始下载：轮询等待完成，完成后静默，不弹窗
        const timer = setInterval(async () => {
          try {
            const r = await fetch('/api/update/download/status')
            const d = await r.json()
            if (d.state === 'done' || d.state === 'error') {
              clearInterval(timer)
              // 故意不弹窗：安装包留到下次启动再提示一键安装
            }
          } catch { /* 忽略轮询错误 */ }
        }, 2000)
      } catch {
        // 下载接口不可达：本次不弹窗，下次启动再试
      }
    }
  } catch {}
})
</script>

<style>
/* 右侧统一工具条（亮色 + iconify） */
.app-tool-rail {
  position: fixed;
  right: 12px;
  top: 50%;
  transform: translateY(-50%);
  z-index: 9999;
  display: flex;
  flex-direction: column;
  gap: 4px;
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  padding: 6px;
  box-shadow: 0 4px 20px rgba(0,0,0,.08);
}
.app-tool-btn {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  border: none;
  background: transparent;
  color: #9ca3af;
  border-radius: 8px;
  padding: 6px 8px;
  width: 52px;
  text-decoration: none;
  transition: all .15s;
}
.app-tool-btn:hover { background: #f3f4f6; color: #1a1a2e; }
.app-tool-btn.active { background: #eff6ff; color: #2563eb; }
.tool-lbl { font-size: 10px; }
</style>
