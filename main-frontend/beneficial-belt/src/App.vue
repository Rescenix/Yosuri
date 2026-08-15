<template>
  <!-- 底部竖排圆胶囊工具条（纯图标，样式对齐聊天界面工具条 icon-pill） -->
  <nav class="app-tool-rail">
    <router-link to="/chat" class="app-tool-btn" title="编码" active-class="active">
      <Icon icon="mdi:code-tags" width="16" />
    </router-link>
    <router-link to="/company" class="app-tool-btn" title="Agent 公司" active-class="active">
      <Icon icon="mdi:domain" width="16" />
    </router-link>
    <router-link to="/publish" class="app-tool-btn" title="网文创作与发布" active-class="active">
          <Icon icon="mdi:book-open-page-variant-outline" width="16" />
        </router-link>
        <router-link to="/comic" class="app-tool-btn" title="漫画创作" active-class="active">
                  <Icon icon="mdi:brush" width="16" />
                </router-link>
        <router-link to="/studio" class="app-tool-btn" title="视频剪辑" active-class="active">
      <Icon icon="mdi:movie-edit-outline" width="16" />
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
/* 底部横排圆胶囊工具条（纯图标，2026-08-13 用户定稿：
   照搬聊天界面终端预览工具条 .terminal-tabs-bar 样式：容器 surface-2 底 + 边框，
   按钮无边框透明，hover/active 背景变化；横排，右下角） */
.app-tool-rail {
  position: fixed;
  right: 30px;
  bottom: 18px;
  z-index: 9999;
  display: flex;
  flex-direction: row;
  align-items: center;
  gap: 2px;
  padding: 4px;
  background: var(--app-surface-2);
  border: 1px solid var(--app-border);
  border-radius: 999px;
  box-shadow: 0 4px 16px rgba(15,23,42,.08);
}
.app-tool-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  height: 26px;
  padding: 0 10px;
  border: none;
  background: transparent;
  color: var(--app-text-faint);
  font-size: 11.5px;
  text-decoration: none;
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
}
.app-tool-btn:hover { background: var(--app-surface-3); color: var(--app-text-soft); }
.app-tool-btn.active { background: var(--app-surface); color: var(--app-text); font-weight: 600; }
html:has(.company-view),html:has(.publish-view) { scrollbar-width: thin; scrollbar-color: #aa8fa0 #f4f1f3; }
html:has(.company-view)::-webkit-scrollbar,html:has(.publish-view)::-webkit-scrollbar { width: 9px; }
html:has(.company-view)::-webkit-scrollbar-track,html:has(.publish-view)::-webkit-scrollbar-track { background: #f4f1f3; }
html:has(.company-view)::-webkit-scrollbar-thumb { border: 2px solid #f4f1f3; border-radius: 999px; background: linear-gradient(#73b895,#39775d); }
html:has(.publish-view)::-webkit-scrollbar-thumb { border: 2px solid #f4f1f3; border-radius: 999px; background: linear-gradient(#dba8bc,#aaa4d4); }
html:has(.company-view)::-webkit-scrollbar-thumb:hover { background: linear-gradient(#55a77d,#245f47); }
html:has(.publish-view)::-webkit-scrollbar-thumb:hover { background: linear-gradient(#c77f9d,#8883bf); }
@media (max-width: 620px) {
  .app-tool-rail { right: 22px; bottom: 10px; padding: 3px; gap: 1px; }
  .app-tool-btn { height: 24px; padding: 0 8px; }
}
</style>
