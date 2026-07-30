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

/* 侧边栏 + 工具面板 -> 完全透明，不毛玻璃 */
[data-dynamic-wallpaper="on"] .chat-window.expanded .gem-sidebar,
[data-dynamic-wallpaper="on"] .chat-window.expanded .tool-panel,
[data-dynamic-wallpaper="on"] .chat-window.expanded .tool-panel-header,
[data-dynamic-wallpaper="on"] .chat-window.expanded .dock-pane {
  background: transparent !important;
  backdrop-filter: none !important;
  -webkit-backdrop-filter: none !important;
  border-color: transparent !important;
  box-shadow: none !important;
}

/* ========== 全局灰色图标 + 透明玻璃（排除消息气泡和热力图） ========== */

/* 工具栏所有图标 -> 灰色 */
[data-dynamic-wallpaper="on"] .toolbar-pill-btn,
[data-dynamic-wallpaper="on"] .toolbar-icon-pill-btn,
[data-dynamic-wallpaper="on"] .gem-icon-btn,
[data-dynamic-wallpaper="on"] .sch-model,
[data-dynamic-wallpaper="on"] .effort-widget,
[data-dynamic-wallpaper="on"] .effort-value,
[data-dynamic-wallpaper="on"] .gem-rail-bar,
[data-dynamic-wallpaper="on"] .gem-rail-bottom,
[data-dynamic-wallpaper="on"] .gem-rail-avatar,
[data-dynamic-wallpaper="on"] .UserMessageRail,
[data-dynamic-wallpaper="on"] .UserMessageRail button,
[data-dynamic-wallpaper="on"] .umr-node,
[data-dynamic-wallpaper="on"] .umr-node.active,
[data-dynamic-wallpaper="on"] .context-bar-widget,
[data-dynamic-wallpaper="on"] .input-row,
[data-dynamic-wallpaper="on"] .input-wrapper,
[data-dynamic-wallpaper="on"] .input-wrapper:focus-within,
[data-dynamic-wallpaper="on"] .floating-tools {
  background: transparent !important;
  border-color: transparent !important;
  color: var(--app-text-faint) !important;
}
[data-dynamic-wallpaper="on"] .toolbar-pill-btn .iconify,
[data-dynamic-wallpaper="on"] .toolbar-icon-pill-btn .iconify,
[data-dynamic-wallpaper="on"] .gem-icon-btn .iconify,
[data-dynamic-wallpaper="on"] .mode-pill .iconify,
[data-dynamic-wallpaper="on"] .sch-model .iconify,
[data-dynamic-wallpaper="on"] .input-inner-btn .iconify,
[data-dynamic-wallpaper="on"] .gem-rail-bar .iconify,
[data-dynamic-wallpaper="on"] .gem-rail-bottom .iconify,
[data-dynamic-wallpaper="on"] .UserMessageRail .iconify,
[data-dynamic-wallpaper="on"] .umr-node .iconify,
[data-dynamic-wallpaper="on"] .floating-tools .iconify {
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

/* 输入框灰线边框 + 背景透明 */
[data-dynamic-wallpaper="on"] .input-wrapper,
[data-dynamic-wallpaper="on"] .input-wrapper:focus-within {
  background: transparent !important;
  box-shadow: none !important;
  border-color: color-mix(in srgb, var(--app-border) 50%, transparent) !important;
}
[data-dynamic-wallpaper="on"] textarea.chat-input,
[data-dynamic-wallpaper="on"] textarea.chat-input:focus {
  background: transparent !important;
  border-color: color-mix(in srgb, var(--app-border) 40%, transparent) !important;
  box-shadow: none !important;
}

/* floating-tools：透明背景，去掉自带的毛玻璃和边框 */
[data-dynamic-wallpaper="on"] .floating-tools {
  background: transparent !important;
  backdrop-filter: none !important;
  -webkit-backdrop-filter: none !important;
  border-color: color-mix(in srgb, var(--app-border) 20%, transparent) !important;
  box-shadow: none !important;
}

/* 消息气泡 active 态：去掉紫色高亮边框 */
[data-dynamic-wallpaper="on"] .message-bubble.user.active {
  box-shadow: none !important;
}

/* 输入框底部工具栏 -> 全透明 */
[data-dynamic-wallpaper="on"] .input-bottom-toolbar {
  background: transparent !important;
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
