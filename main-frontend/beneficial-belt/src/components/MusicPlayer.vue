<template>
  <div class="mini-player">
    <div class="player-info">
      <span class="player-song">{{ currentTitle || '未播放' }}</span>
    </div>
    <div class="player-controls">
      <button @click="prevTrack" title="上一首">
        <Icon icon="mdi:skip-previous" width="18" color="#475467" />
      </button>
      <button @click="togglePlay" class="play-btn" title="播放/暂停">
        <Icon :icon="isPlaying ? 'mdi:pause' : 'mdi:play'" width="18" color="#fff" />
      </button>
      <button @click="nextTrack" title="下一首">
        <Icon icon="mdi:skip-next" width="18" color="#475467" />
      </button>
    </div>
    <audio ref="audioRef" :src="currentSrc" @play="isPlaying = true" @pause="isPlaying = false" @ended="onSongEnded" @error="onError"></audio>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { onMounted } from 'vue'
import { Icon } from '@iconify/vue'

const playlist = [
  { title: 'CopyMemory', src: '/music/CopyMemory.mp3', cover: '/images/CopyMemory.png', theme: '#60a5fa' },
  { title: 'Bamboo', src: '/music/Bamboo.mp3', cover: '/images/Bamboo.jpg', theme: '#10b981' },
  { title: 'AspiralMoon', src: '/music/AspiralMoon.mp3', cover: '/images/AspiralMoon.jpg', theme: '#c084fc' },
]

const audioRef = ref(null)
const currentIndex = ref(0)
const isPlaying = ref(false)
let canAutoPlay = false

const currentTrack = computed(() => playlist[currentIndex.value])
const currentSrc = computed(() => currentTrack.value.src)
const currentTitle = computed(() => currentTrack.value.title)
const currentCover = computed(() => currentTrack.value.cover)

const waitForCanPlay = () => {
  return new Promise((resolve, reject) => {
    const audio = audioRef.value
    if (!audio) return reject(new Error('音频元素未初始化'))
    const timeout = setTimeout(() => {
      audio.removeEventListener('canplay', handler)
      reject(new Error('音频加载超时'))
    }, 5000)
    const handler = () => {
      clearTimeout(timeout)
      audio.removeEventListener('canplay', handler)
      resolve()
    }
    audio.addEventListener('canplay', handler)
  })
}

const playCurrentTrack = async () => {
  const audio = audioRef.value
  if (!audio) return
  audio.load()
  try {
    await waitForCanPlay()
    await audio.play()
    canAutoPlay = true
    isPlaying.value = true
  } catch (err) {
    console.warn('播放失败:', err.message || err)
  }
}

const togglePlay = () => {
  const audio = audioRef.value
  if (!audio) return
  if (audio.paused) {
    playCurrentTrack()
  } else {
    audio.pause()
  }
}

const onSongEnded = () => {
  if (canAutoPlay) {
    nextTrack()
  }
}

const nextTrack = () => {
  currentIndex.value = (currentIndex.value + 1) % playlist.length
  const nextColor = playlist[currentIndex.value].theme
  window.dispatchEvent(new CustomEvent('theme-switch', { detail: nextColor }))
  playCurrentTrack()
}

const prevTrack = () => {
  currentIndex.value = (currentIndex.value - 1 + playlist.length) % playlist.length
  const prevColor = playlist[currentIndex.value].theme
  window.dispatchEvent(new CustomEvent('theme-switch', { detail: prevColor }))
  playCurrentTrack()
}

const onError = () => {
  console.warn('音频加载失败:', currentSrc.value)
}

onMounted(() => {
    window.addEventListener('shanxi-action', (event) => {
        const { action } = event.detail
        if (action.startsWith('switch_music:')) {
            const songName = action.split(':')[1]
            switchToSong(songName)
        }
    })
})

function switchToSong(songName) {
    const index = playlist.findIndex(s => s.title === songName || s.src.includes(songName))
    if (index !== -1) {
        currentIndex.value = index
        const targetTheme = playlist[index].theme
        window.dispatchEvent(new CustomEvent('theme-switch', { detail: targetTheme }))
        playCurrentTrack()
    } else {
        nextTrack()
    }
}

window.__musicState = {
  get currentIndex() { return currentIndex.value },
  playlist
}
</script>

<style scoped>
.mini-player {
  position: fixed;
  left: 20px;
  top: 50%;
  transform: translateY(-50%);
  z-index: 10;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  background: rgba(255, 255, 255, 0.85);
  backdrop-filter: blur(10px);
  border: 1px solid #e5e5e5;
  border-radius: 20px;
  padding: 10px 14px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.04);
  transition: all 0.2s ease;
}

.mini-player:hover {
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.08);
  background: #fff;
}

.player-info {
  max-width: 120px;
  overflow: hidden;
  text-align: center;
}

.player-song {
  font-size: 11px;
  color: #333;
  font-weight: 500;
  white-space: nowrap;
  text-overflow: ellipsis;
  overflow: hidden;
  display: block;
}

.player-controls {
  display: flex;
  align-items: center;
  gap: 8px;
}

.player-controls button {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border: 1px solid #d0d5dd;
  border-radius: 50%;
  background: #fff;
  cursor: pointer;
  transition: all 0.15s ease;
}

.player-controls button:hover {
  background: #f0f0f0;
  border-color: #a0a8b4;
}

.play-btn {
  background: #2563eb !important;
  border-color: #2563eb !important;
  width: 36px !important;
  height: 36px !important;
}

.play-btn:hover {
  background: #1d4ed8 !important;
  border-color: #1d4ed8 !important;
}

.mini-player {
  position: fixed;
  left: 24px;        /* 从 20px 改为 24px，稍微靠内 */
  top: 50%;
  transform: translateY(-50%);
  /* 其余不变 */
}
</style>