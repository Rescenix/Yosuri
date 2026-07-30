<template>
  <div v-if="dynamicWallpaperReady" class="dynamic-wallpaper" aria-hidden="true">
    <video
      ref="videoElement"
      class="dynamic-wallpaper-video"
      :src="dynamicWallpaperUrl"
      autoplay
      loop
      muted
      playsinline
      disablepictureinpicture
      @canplay="playIfAllowed"
      @error="handleMediaError"
    ></video>
    <div class="dynamic-wallpaper-shade"></div>
  </div>
</template>

<script setup>
import { onMounted, onUnmounted, ref, watch } from 'vue'
import {
  dynamicWallpaperReady,
  dynamicWallpaperError,
  dynamicWallpaperSettings,
  dynamicWallpaperUrl,
  initDynamicWallpaper,
} from '../composables/useDynamicWallpaper.js'

const videoElement = ref(null)

function playIfAllowed() {
  if (!dynamicWallpaperSettings.enabled || document.hidden) return
  videoElement.value?.play().catch(() => {})
}

function handleMediaError() {
  const mediaError = videoElement.value?.error
  dynamicWallpaperError.value = mediaError?.message || '浏览器无法解码该视频，请改用 H.264 MP4 或 WebM'
}

function syncPlayback() {
  const video = videoElement.value
  if (!video) return
  if (
    !dynamicWallpaperSettings.enabled ||
    (dynamicWallpaperSettings.pauseWhenHidden && document.hidden)
  ) {
    video.pause()
    return
  }
  video.play().catch(() => {})
}

watch(
  () => [dynamicWallpaperSettings.enabled, dynamicWallpaperSettings.pauseWhenHidden, dynamicWallpaperUrl.value],
  syncPlayback,
)

onMounted(() => {
  initDynamicWallpaper()
  document.addEventListener('visibilitychange', syncPlayback)
})

onUnmounted(() => {
  document.removeEventListener('visibilitychange', syncPlayback)
})
</script>

<style>
.dynamic-wallpaper {
  position: fixed;
  inset: 0;
  z-index: 0;
  overflow: hidden;
  pointer-events: none;
  background: #09090b;
}

.dynamic-wallpaper-video {
  width: 100%;
  height: 100%;
  display: block;
  object-fit: cover;
}

.dynamic-wallpaper-shade {
  position: absolute;
  inset: 0;
  background: rgba(0, 0, 0, var(--wallpaper-dim, 0.08));
}

[data-dynamic-wallpaper="on"] body {
  background: #09090b;
}

[data-dynamic-wallpaper="on"] .app-shell,
[data-dynamic-wallpaper="on"] .chat-page {
  background: transparent !important;
}

[data-dynamic-wallpaper="on"] .chat-window.expanded {
  background: transparent !important;
  backdrop-filter: none !important;
  -webkit-backdrop-filter: none !important;
}

[data-dynamic-wallpaper="on"] .chat-window.expanded .chat-body-row {
  background: transparent !important;
  backdrop-filter: none !important;
  -webkit-backdrop-filter: none !important;
}

[data-dynamic-wallpaper="on"] .chat-window.expanded .chat-body.studio .chat-content {
  background: transparent !important;
  backdrop-filter: none !important;
  -webkit-backdrop-filter: none !important;
}

/* 侧边栏 + 工具面板 -> 玻璃透明，去掉实体边框和阴影 */
[data-dynamic-wallpaper="on"] .chat-window.expanded .gem-sidebar,
[data-dynamic-wallpaper="on"] .chat-window.expanded .tool-panel-header,
[data-dynamic-wallpaper="on"] .chat-window.expanded .dock-pane {
  background: rgba(var(--app-surface-rgb), var(--wallpaper-panel-alpha, 0.35)) !important;
  backdrop-filter: blur(var(--wallpaper-blur, 4px)) !important;
  -webkit-backdrop-filter: blur(var(--wallpaper-blur, 4px)) !important;
  border-color: color-mix(in srgb, var(--app-border) 30%, transparent) !important;
  box-shadow: none !important;
}

/* 主页时侧边栏去毛玻璃 */
[data-dynamic-wallpaper="on"] .chat-window.expanded:has(.home-container-for-layout) .gem-sidebar {
  background: transparent !important;
  backdrop-filter: none !important;
  -webkit-backdrop-filter: none !important;
}

/* ========== 全局灰色图标 + 透明玻璃（排除消息气泡和热力图） ========== */

/* 工具栏所有图标 -> 灰色 */
[data-dynamic-wallpaper="on"] .toolbar-pill-btn,
[data-dynamic-wallpaper="on"] .toolbar-icon-pill-btn,
[data-dynamic-wallpaper="on"] .gem-icon-btn,
[data-dynamic-wallpaper="on"] .sch-model,
[data-dynamic-wallpaper="on"] .effort-widget {
  background: transparent !important;
  border-color: transparent !important;
  color: var(--app-text-faint) !important;
}
[data-dynamic-wallpaper="on"] .toolbar-pill-btn .iconify,
[data-dynamic-wallpaper="on"] .toolbar-icon-pill-btn .iconify,
[data-dynamic-wallpaper="on"] .gem-icon-btn .iconify,
[data-dynamic-wallpaper="on"] .mode-pill .iconify,
[data-dynamic-wallpaper="on"] .sch-model .iconify,
[data-dynamic-wallpaper="on"] .input-inner-btn .iconify {
  color: var(--app-text-faint) !important;
}

/* 模式按钮（Yolo/Balanced）-> 透明 */
[data-dynamic-wallpaper="on"] .mode-pill,
[data-dynamic-wallpaper="on"] .toolbar-pill-btn.mode-pill {
  background: transparent !important;
  border: 1px solid color-mix(in srgb, var(--app-border) 40%, transparent) !important;
  color: var(--app-text-soft) !important;
}

/* 模型选择pill -> 透明 */
[data-dynamic-wallpaper="on"] .sch-model {
  background: transparent !important;
  border: 1px solid color-mix(in srgb, var(--app-border) 40%, transparent) !important;
}

/* 输入框底部工具栏 -> 全透明 */
[data-dynamic-wallpaper="on"] .input-bottom-toolbar {
  background: transparent !important;
}

/* 聊天消息区 -> 毛玻璃，文字清晰可见 */
[data-dynamic-wallpaper="on"] .chat-window.expanded .chat-messages {
  background: rgba(var(--app-surface-rgb), var(--wallpaper-panel-alpha, 0.25)) !important;
  backdrop-filter: blur(var(--wallpaper-blur, 4px)) !important;
  -webkit-backdrop-filter: blur(var(--wallpaper-blur, 4px)) !important;
}

/* 消息气泡保持原有毛玻璃（排除在透明化之外） */
[data-dynamic-wallpaper="on"] .message-bubble.user {
  background: rgba(var(--app-surface-rgb), var(--wallpaper-panel-alpha, 0.35)) !important;
  border-color: color-mix(in srgb, var(--app-accent) 40%, transparent) !important;
  backdrop-filter: blur(calc(var(--wallpaper-blur, 4px) * 0.75)) !important;
  -webkit-backdrop-filter: blur(calc(var(--wallpaper-blur, 4px) * 0.75)) !important;
}

/* 热力图保持原样（排除在透明化之外） */
[data-dynamic-wallpaper="on"] .home-heatmap-cell {
  /* 保持原有颜色，不覆盖 */
}

</style>
