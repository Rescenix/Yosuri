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
  background: rgba(var(--app-surface-rgb), var(--wallpaper-panel-alpha, 0.35)) !important;
  backdrop-filter: blur(var(--wallpaper-blur, 4px)) !important;
  -webkit-backdrop-filter: blur(var(--wallpaper-blur, 4px)) !important;
}

[data-dynamic-wallpaper="on"] .chat-window.expanded .chat-body-row {
  background: rgba(var(--app-surface-rgb), calc(var(--wallpaper-panel-alpha, 0.35) - 0.05)) !important;
  backdrop-filter: blur(var(--wallpaper-blur, 4px)) !important;
  -webkit-backdrop-filter: blur(var(--wallpaper-blur, 4px)) !important;
}

[data-dynamic-wallpaper="on"] .chat-window.expanded .chat-body.studio .chat-content {
  background: transparent !important;
  backdrop-filter: none !important;
  -webkit-backdrop-filter: none !important;
}

/* 主页全透明：不要毛玻璃，动态壁纸完全露出来 */
[data-dynamic-wallpaper="on"] .chat-window.expanded:has(.home-container-for-layout) {
  background: transparent !important;
  backdrop-filter: none !important;
  -webkit-backdrop-filter: none !important;
}
[data-dynamic-wallpaper="on"] .chat-window.expanded:has(.home-container-for-layout) .chat-body-row {
  background: transparent !important;
  backdrop-filter: none !important;
  -webkit-backdrop-filter: none !important;
}
[data-dynamic-wallpaper="on"] .home-container-for-layout {
  background: transparent !important;
}
[data-dynamic-wallpaper="on"] .session-home {
  background: transparent !important;
  backdrop-filter: none !important;
  -webkit-backdrop-filter: none !important;
} 
[data-dynamic-wallpaper="on"] .session-home .home-stats-card {
  background: transparent !important;
  backdrop-filter: none !important;
  -webkit-backdrop-filter: none !important;
  box-shadow: none !important;
}

/* 输入框白色背景卡片 → 变透明毛玻璃 */
[data-dynamic-wallpaper="on"] .input-wrapper {
  background: transparent !important;
  border-color: color-mix(in srgb, var(--app-border) 50%, transparent) !important;
  box-shadow: none !important;
}
[data-dynamic-wallpaper="on"] .input-dir-bar {
  background: rgba(var(--app-surface-rgb), var(--wallpaper-panel-alpha, 0.35)) !important;
  backdrop-filter: blur(calc(var(--wallpaper-blur, 4px) * 0.6)) !important;
  -webkit-backdrop-filter: blur(calc(var(--wallpaper-blur, 4px) * 0.6)) !important;
}

[data-dynamic-wallpaper="on"] .chat-window.expanded .gem-sidebar,
[data-dynamic-wallpaper="on"] .chat-window.expanded .tool-panel-header,
[data-dynamic-wallpaper="on"] .chat-window.expanded .dock-pane {
  background: rgba(var(--app-surface-rgb), var(--wallpaper-panel-alpha, 0.35)) !important;
  backdrop-filter: blur(var(--wallpaper-blur, 4px)) !important;
  -webkit-backdrop-filter: blur(var(--wallpaper-blur, 4px)) !important;
}

[data-dynamic-wallpaper="on"] .chat-window.expanded .chat-input-area {
  background: rgba(var(--app-surface-rgb), calc(var(--wallpaper-panel-alpha, 0.35) - 0.1)) !important;
  backdrop-filter: blur(calc(var(--wallpaper-blur, 4px) * 0.6)) !important;
  -webkit-backdrop-filter: blur(calc(var(--wallpaper-blur, 4px) * 0.6)) !important;
}

[data-dynamic-wallpaper="on"] .message-bubble.user {
  background: rgba(var(--app-surface-rgb), var(--wallpaper-panel-alpha, 0.35)) !important;
  border-color: color-mix(in srgb, var(--app-accent) 40%, transparent) !important;
  backdrop-filter: blur(calc(var(--wallpaper-blur, 4px) * 0.75)) !important;
  -webkit-backdrop-filter: blur(calc(var(--wallpaper-blur, 4px) * 0.75)) !important;
}

</style>
