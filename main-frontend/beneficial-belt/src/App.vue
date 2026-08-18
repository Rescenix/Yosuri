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
            <!-- DHS 安全插件生态：鲸鱼入口挂在底部工具条右端（2026-08-18 自输入工具栏移入） -->
            <button class="dhs-whale-shortcut" type="button" title="DHS 安全插件生态" aria-label="打开 DHS 安全插件生态" @click="showDHSCommunity = true">
              <Icon icon="simple-icons:deepseek" width="16" />
              <span class="dhs-whale-shield"><Icon icon="mdi:shield-check" width="9" /></span>
            </button>
          </nav>
          <router-view />
          <UpdateModal v-if="showUpdate" :update="updateInfo" @close="showUpdate = false" />
          <DHSCommunityModal v-if="showDHSCommunity" @close="showDHSCommunity = false" />
    <!-- 顶部轻量更新提示：15s 自动消失，点击才弹全窗（2026-08-17 用户定稿：堵塞弹窗破坏体验） -->
    <button v-if="showUpdateBanner" class="update-banner" type="button" @click="openUpdateModal">
      <span class="update-banner-dot" />
      <span>发现新版本 <b>{{ updateInfo && updateInfo.latest_version }}</b>，点击查看</span>
      <span class="update-banner-arrow">›</span>
    </button>
  </template>

<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { Icon } from '@iconify/vue'
import { useAuth } from './composables/useAuth.js'
import { getSkippedVersion, isUpdateNotifyDisabled, isTestUpdatesEnabled, isPrereleaseVersionString } from './composables/updatePrefs.js'
import UpdateModal from './components/shanxi/chat/UpdateModal.vue'
import DHSCommunityModal from './components/shanxi/chat/DHSCommunityModal.vue'

const auth = useAuth()
const showUpdate = ref(false)
const updateInfo = ref(null)
// DHS 安全插件生态：鲸鱼入口从输入工具栏移到底部工具条右端（2026-08-18）
const showDHSCommunity = ref(false)
// 顶部轻量更新横幅：检测到新安装包已就绪时显示 15s，点击才弹全窗（2026-08-17 用户定稿）
const showUpdateBanner = ref(false)
let updateBannerTimer = null

function openUpdateModal() {
  clearTimeout(updateBannerTimer)
  updateBannerTimer = null
  showUpdateBanner.value = false
  showUpdate.value = true
}

onBeforeUnmount(() => {
  if (updateBannerTimer) clearTimeout(updateBannerTimer)
})

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
      // 关闭「热更新测试版本」时忽略预发布（alpha/beta/rc）更新（2026-08-16）
      if (!isTestUpdatesEnabled() && isPrereleaseVersionString(data.update.latest_version)) return
      updateInfo.value = data.update
      // 第一次进应用：静默后台下载安装包（用户无感知、不弹窗）。
      // 下完本次不打扰；下次启动本地已有安装包 → 直接弹「一键安装」。
      try {
        const dl = await fetch('/api/update/download', { method: 'POST' })
        const dlData = await dl.json()
        if (dlData.state === 'done') {
                  // 安装包已就绪（上次启动已下载完）→ 顶部轻量横幅提示 15s，不堵塞界面；
                  // 用户点击横幅才弹「一键安装」全窗（2026-08-17 用户定稿，替代原直接弹窗）
                  showUpdateBanner.value = true
                  updateBannerTimer = setTimeout(() => {
                    showUpdateBanner.value = false
                    updateBannerTimer = null
                  }, 15000)
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
/* 顶部轻量更新横幅：fixed 顶部居中，不堵塞界面；15s 自动消失，点击弹全窗（2026-08-17） */
.update-banner {
  position: fixed;
  top: 14px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 10001;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  max-width: calc(100vw - 32px);
  padding: 7px 14px;
  border: 1px solid var(--app-border);
  border-radius: 999px;
  background: var(--app-surface-2);
  color: var(--app-text-soft);
  font-size: 12.5px;
  line-height: 1;
  cursor: pointer;
  box-shadow: 0 4px 18px rgba(15, 23, 42, 0.12);
  transition: background 0.15s ease, color 0.15s ease, border-color 0.15s ease;
}
.update-banner:hover { background: var(--app-surface-3); color: var(--app-text); border-color: var(--app-accent); }
.update-banner b { font-weight: 600; color: var(--app-accent); }
.update-banner-dot {
  width: 7px; height: 7px;
  border-radius: 50%;
  background: var(--app-accent);
  flex: none;
  animation: update-banner-pulse 1.2s ease-in-out infinite;
}
.update-banner-arrow { color: var(--app-text-faint); font-size: 14px; }
@keyframes update-banner-pulse {
  0%, 100% { opacity: 1; box-shadow: 0 0 0 0 color-mix(in srgb, var(--app-accent) 35%, transparent); }
  50% { opacity: 0.75; box-shadow: 0 0 0 5px color-mix(in srgb, var(--app-accent) 0%, transparent); }
}
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
/* DHS 鲸鱼入口（自 chat-window.css 的 dhs-whale-shortcut，移入底部工具条右端后样式随附） */
.dhs-whale-shortcut {
  position: relative;
  width: 30px;
  height: 30px;
  display: grid;
  place-items: center;
  flex: 0 0 30px;
  padding: 0;
  color: #8750ff;
  background: color-mix(in srgb, #8750ff 9%, var(--app-surface));
  border: 1px solid color-mix(in srgb, #8750ff 28%, var(--app-border));
  border-radius: 10px;
  cursor: pointer;
  box-shadow: 0 4px 12px rgba(126, 73, 255, 0.10);
  transition: transform .16s ease, color .16s ease, background .16s ease, box-shadow .16s ease;
}
.dhs-whale-shortcut:hover {
  color: #fff;
  background: linear-gradient(135deg, #7548ff, #a245ff);
  box-shadow: 0 7px 18px rgba(126, 73, 255, 0.24);
  transform: translateY(-1px);
}
.dhs-whale-shortcut:focus-visible { outline: 2px solid color-mix(in srgb, #8750ff 55%, transparent); outline-offset: 2px; }
.dhs-whale-shield {
  position: absolute;
  right: -4px;
  bottom: -4px;
  width: 14px;
  height: 14px;
  display: grid;
  place-items: center;
  color: #087a57;
  background: #e6fff5;
  border: 2px solid var(--app-surface);
  border-radius: 50%;
}
.dhs-whale-shortcut:hover .dhs-whale-shield { color: #067647; background: #d7ffef; }
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
