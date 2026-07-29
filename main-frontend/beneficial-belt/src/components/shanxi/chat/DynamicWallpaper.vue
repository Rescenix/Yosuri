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
    ></video>
    <div class="dynamic-wallpaper-shade"></div>
  </div>
</template>

<script setup>
import { onMounted, onUnmounted, ref, watch } from 'vue'
import {
  dynamicWallpaperReady,
  dynamicWallpaperSettings,
  dynamicWallpaperUrl,
  initDynamicWallpaper,
} from '../composables/useDynamicWallpaper.js'

const videoElement = ref(null)

function playIfAllowed() {
  if (!dynamicWallpaperSettings.enabled || document.hidden) return
  videoElement.value?.play().catch(() => {})
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
  background:
    linear-gradient(rgba(0, 0, 0, var(--wallpaper-dim, 0.24)), rgba(0, 0, 0, var(--wallpaper-dim, 0.24))),
    radial-gradient(circle at 50% 0%, rgba(var(--app-surface-rgb), 0.02), rgba(var(--app-surface-rgb), 0.12));
}

[data-dynamic-wallpaper="on"] body {
  background: #09090b;
}

[data-dynamic-wallpaper="on"] .chat-window.expanded {
  background: transparent;
  border-color: transparent;
}

[data-dynamic-wallpaper="on"] .chat-window.expanded .chat-body-row {
  background: rgba(var(--app-surface-rgb), var(--wallpaper-workspace-alpha, 0.6));
  backdrop-filter: blur(var(--wallpaper-blur, 10px));
  -webkit-backdrop-filter: blur(var(--wallpaper-blur, 10px));
}

[data-dynamic-wallpaper="on"] .chat-window.expanded .chat-body.studio .chat-content,
[data-dynamic-wallpaper="on"] .chat-window.expanded .gem-sidebar,
[data-dynamic-wallpaper="on"] .chat-window.expanded .tool-panel-header,
[data-dynamic-wallpaper="on"] .chat-window.expanded .dock-pane {
  background: rgba(var(--app-surface-rgb), var(--wallpaper-panel-alpha, 0.84));
  backdrop-filter: blur(var(--wallpaper-blur, 10px));
  -webkit-backdrop-filter: blur(var(--wallpaper-blur, 10px));
}

[data-dynamic-wallpaper="on"] .chat-window.expanded .chat-input-area {
  background: transparent;
}

</style>
