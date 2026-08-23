<template>
  <!-- 底部竖排圆胶囊工具条（纯图标，样式对齐聊天界面工具条 icon-pill） -->
  <nav class="app-tool-rail">
    <router-link to="/chat" class="app-tool-btn" title="编码" active-class="active">
      <Icon icon="mdi:code-tags" width="16" />
    </router-link>
    <router-link to="/company" class="app-tool-btn" title="Agent 公司" active-class="active">
      <Icon icon="mdi:domain" width="16" />
    </router-link>
    <router-link to="/sites" class="app-tool-btn" title="站点：发布并分享 Agent 作品" active-class="active">
      <Icon icon="mdi:web" width="16" />
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
          <!-- 左下角：聚合 API 快捷入口（2026-08-18，右移避让） -->
          <nav class="app-tool-rail-left">
            <button class="app-tool-btn agg-api-shortcut" type="button" title="聚合 API" aria-label="打开聚合 API" @click="showAggApi = true">
              <Icon icon="mdi:api" width="16" />聚合 API
            </button>
          </nav>
          <SettingsModal v-if="showAggApi" default-tab="aggapi" @close="showAggApi = false" />
          <router-view />
          <UpdateModal v-if="showUpdate" :update="updateInfo" @close="showUpdate = false" />
          <DHSCommunityModal v-if="showDHSCommunity" @close="showDHSCommunity = false" />
    <!-- 顶部轻量更新提示：15s 自动消失，点击才弹全窗（2026-08-17 用户定稿：堵塞弹窗破坏体验） -->
    <button v-if="showUpdateBanner" class="update-banner" type="button" @click="openUpdateModal">
      <span class="update-banner-dot" />
      <span>发现新版本 <b>{{ updateInfo && updateInfo.latest_version }}</b>，点击查看</span>
      <span class="update-banner-arrow">›</span>
    </button>
    <!-- 升级完成提示：alpha 补丁启动时静默自动应用后，下一次启动显示（2026-08-18 用户定稿） -->
    <div v-if="showUpdatedBanner" class="updated-banner">
      <span class="updated-banner-check">✓</span>
      <span>已更新到 <b>{{ updatedVersion }}</b></span>
      <button class="updated-banner-close" type="button" @click="closeUpdatedBanner" aria-label="关闭">×</button>
    </div>
    <!-- 登录/注册成功提示：欢迎回来横幅，和升级完成横幅同款样式/15s 自动消失（2026-08-20 用户定稿） -->
    <div v-if="showWelcomeBanner" class="updated-banner">
      <span class="updated-banner-check">✓</span>
      <span>欢迎回来，<b>{{ welcomeName }}</b>！</span>
      <button class="updated-banner-close" type="button" @click="closeWelcomeBanner" aria-label="关闭">×</button>
    </div>
  </template>

<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { Icon } from '@iconify/vue'
import { useAuth } from './composables/useAuth.js'
import { getSkippedVersion, isUpdateNotifyDisabled, isTestUpdatesEnabled, isPrereleaseVersionString, shouldShowUpdateBanner, markUpdateBannerShown } from './composables/updatePrefs.js'
import UpdateModal from './components/shanxi/chat/UpdateModal.vue'
import DHSCommunityModal from './components/shanxi/chat/DHSCommunityModal.vue'
import SettingsModal from './components/shanxi/chat/SettingsModal.vue'

const auth = useAuth()
const showUpdate = ref(false)
const updateInfo = ref(null)
// DHS 安全插件生态：鲸鱼入口从输入工具栏移到底部工具条右端（2026-08-18）
const showDHSCommunity = ref(false)
// 左下角聚合 API 快捷入口：点开直接跳到设置弹窗的「聚合 API」tab（2026-08-18）
const showAggApi = ref(false)
// 顶部轻量更新横幅：检测到新安装包已就绪时显示 15s，点击才弹全窗（2026-08-17 用户定稿）
const showUpdateBanner = ref(false)
let updateBannerTimer = null
// 升级完成横幅：alpha 补丁静默自动应用后显示「已更新到 vX」15s（2026-08-18 用户定稿）
const showUpdatedBanner = ref(false)
const updatedVersion = ref('')
let updatedBannerTimer = null
let updateCheckTimer = null
// 登录/注册欢迎横幅：SessionMenuContent 登录/注册成功后广播 auth-welcome，
// 这里监听并显示 15s（2026-08-20 用户定稿：注册成功即直接登录，需要一个提示告诉用户成功了）
const showWelcomeBanner = ref(false)
const welcomeName = ref('')
let welcomeBannerTimer = null

function showBanner() {
  showUpdateBanner.value = true
  updateBannerTimer = setTimeout(() => {
    showUpdateBanner.value = false
    updateBannerTimer = null
  }, 15000)
}

function closeUpdatedBanner() {
  clearTimeout(updatedBannerTimer)
  updatedBannerTimer = null
  showUpdatedBanner.value = false
}

function closeWelcomeBanner() {
  clearTimeout(welcomeBannerTimer)
  welcomeBannerTimer = null
  showWelcomeBanner.value = false
}

function onAuthWelcome(e) {
  const name = e.detail && e.detail.name
  if (!name) return
  welcomeName.value = name
  showWelcomeBanner.value = true
  clearTimeout(welcomeBannerTimer)
  welcomeBannerTimer = setTimeout(closeWelcomeBanner, 15000)
}

function openUpdateModal() {
  clearTimeout(updateBannerTimer)
  updateBannerTimer = null
  showUpdateBanner.value = false
  showUpdate.value = true
}

// 更新检查 + 触发后台下载。
// silent=true（30 分钟周期）：下完静默不打扰，等下次进应用提示；
// silent=false（启动）：安装包已就绪 → 弹轻量横幅（同版本 3 天节流）。
async function checkAndDownload(silent) {
  if (isUpdateNotifyDisabled()) return
  let res
  try {
    res = await fetch('/api/update/check')
  } catch { return }
  let data
  try { data = await res.json() } catch { return }
  if (!(data.ok && data.update && data.update.has_update)) return
  if (getSkippedVersion() === data.update.latest_version) return
  if (!isTestUpdatesEnabled() && isPrereleaseVersionString(data.update.latest_version)) return
  updateInfo.value = data.update
  try {
    const dl = await fetch('/api/update/download', { method: 'POST' })
    const dlData = await dl.json()
    if (dlData.state === 'done') {
      // 安装包已就绪（本次启动前已下好）→ 弹轻量横幅；
      // 同一版本 3 天内只提醒一次，3 天后没装再提醒（用户 2026-08-18 定稿）
      if (silent) return
      if (!shouldShowUpdateBanner(data.update.latest_version)) return
      markUpdateBannerShown(data.update.latest_version)
      showBanner()
      return
    }
    // 下载中：轮询等待完成，完成后静默（下次进应用再提示）
    const timer = setInterval(async () => {
      try {
        const r = await fetch('/api/update/download/status')
        const d = await r.json()
        if (d.state === 'done' || d.state === 'error') {
          clearInterval(timer)
          // 静默：安装包已就绪，下次启动磁盘判断 done → 弹横幅
        }
      } catch { /* 忽略轮询错误 */ }
    }, 2000)
  } catch {
    // 下载接口不可达：本次不弹窗，下次启动再试
  }
}

onBeforeUnmount(() => {
  if (updateBannerTimer) clearTimeout(updateBannerTimer)
  if (updatedBannerTimer) clearTimeout(updatedBannerTimer)
  if (welcomeBannerTimer) clearTimeout(welcomeBannerTimer)
  if (updateCheckTimer) clearInterval(updateCheckTimer)
  window.removeEventListener('auth-welcome', onAuthWelcome)
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
  window.addEventListener('auth-welcome', onAuthWelcome)
})

onMounted(async () => {
  // 1) 升级完成提示：alpha 补丁静默自动应用后，后端留了一次性标记（读完即删）
  try {
    const r = await fetch('/api/update/last-applied')
    const d = await r.json()
    if (d && d.ok && d.version) {
      updatedVersion.value = d.version
      showUpdatedBanner.value = true
      updatedBannerTimer = setTimeout(closeUpdatedBanner, 15000)
    }
  } catch { /* 标记接口不可达：静默 */ }
  // 2) 更新检查 + 后台下载：启动时一次 + 每 30 分钟周期检查
  //    （运行中发布新版也会自动下载，下完等下次进应用提示；2026-08-18）
  await checkAndDownload(false)
  updateCheckTimer = setInterval(() => checkAndDownload(true), 30 * 60 * 1000)
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
/* 升级完成横幅：alpha 静默自动应用后显示「已更新到 vX」，成功绿色（2026-08-18） */
.updated-banner {
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
  border: 1px solid color-mix(in srgb, #22c55e 45%, var(--app-border));
  border-radius: 999px;
  background: var(--app-surface-2);
  color: var(--app-text-soft);
  font-size: 12.5px;
  line-height: 1;
  box-shadow: 0 4px 18px rgba(15, 23, 42, 0.12);
}
.updated-banner b { font-weight: 600; color: #22c55e; }
.updated-banner-check {
  width: 16px; height: 16px;
  border-radius: 50%;
  background: #22c55e;
  color: #fff;
  font-size: 11px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: none;
}
.updated-banner-close {
  border: none;
  background: none;
  color: var(--app-text-faint);
  font-size: 14px;
  line-height: 1;
  cursor: pointer;
  padding: 0 0 0 4px;
}
.updated-banner-close:hover { color: var(--app-text); }
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
/* 左下角聚合 API 快捷入口：与右下角工具条同款胶囊，镜像到左侧（2026-08-18 右移避让） */
.app-tool-rail-left {
  position: fixed;
  left: 150px;
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
.agg-api-shortcut {
  color: var(--app-accent);
  font-weight: 600;
}
.agg-api-shortcut:hover { background: var(--app-surface-3); color: var(--app-accent); }
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
  .app-tool-rail-left { left: 130px; bottom: 10px; padding: 3px; }
}
</style>
