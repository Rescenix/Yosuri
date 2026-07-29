import { computed, reactive, ref, watch } from 'vue'

const SETTINGS_KEY = 'aurora_dynamic_wallpaper'
const DB_NAME = 'aurora_appearance'
const DB_VERSION = 1
const STORE_NAME = 'wallpapers'
const ACTIVE_VIDEO_KEY = 'active-video'

const DEFAULT_SETTINGS = {
  enabled: false,
  dim: 24,
  panelOpacity: 84,
  blur: 10,
  pauseWhenHidden: true,
}

function readSettings() {
  try {
    return { ...DEFAULT_SETTINGS, ...JSON.parse(localStorage.getItem(SETTINGS_KEY) || '{}') }
  } catch {
    return { ...DEFAULT_SETTINGS }
  }
}

export const dynamicWallpaperSettings = reactive(readSettings())
export const dynamicWallpaperUrl = ref('')
export const dynamicWallpaperFileName = ref('')
export const dynamicWallpaperLoading = ref(true)
export const dynamicWallpaperError = ref('')
export const dynamicWallpaperReady = computed(() => Boolean(dynamicWallpaperUrl.value))

let currentObjectUrl = ''
let loadPromise = null

function openDatabase() {
  return new Promise((resolve, reject) => {
    if (!('indexedDB' in window)) {
      reject(new Error('当前浏览器不支持本地壁纸持久化'))
      return
    }
    const request = indexedDB.open(DB_NAME, DB_VERSION)
    request.onupgradeneeded = () => {
      const db = request.result
      if (!db.objectStoreNames.contains(STORE_NAME)) db.createObjectStore(STORE_NAME)
    }
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error || new Error('无法打开壁纸存储'))
  })
}

async function readStoredVideo() {
  const db = await openDatabase()
  try {
    return await new Promise((resolve, reject) => {
      const request = db.transaction(STORE_NAME, 'readonly').objectStore(STORE_NAME).get(ACTIVE_VIDEO_KEY)
      request.onsuccess = () => resolve(request.result || null)
      request.onerror = () => reject(request.error || new Error('无法读取壁纸'))
    })
  } finally {
    db.close()
  }
}

async function writeStoredVideo(value) {
  const db = await openDatabase()
  try {
    await new Promise((resolve, reject) => {
      const request = db.transaction(STORE_NAME, 'readwrite').objectStore(STORE_NAME).put(value, ACTIVE_VIDEO_KEY)
      request.onsuccess = () => resolve()
      request.onerror = () => reject(request.error || new Error('无法保存壁纸'))
    })
  } finally {
    db.close()
  }
}

async function deleteStoredVideo() {
  const db = await openDatabase()
  try {
    await new Promise((resolve, reject) => {
      const request = db.transaction(STORE_NAME, 'readwrite').objectStore(STORE_NAME).delete(ACTIVE_VIDEO_KEY)
      request.onsuccess = () => resolve()
      request.onerror = () => reject(request.error || new Error('无法移除壁纸'))
    })
  } finally {
    db.close()
  }
}

function replaceObjectUrl(blob, fileName = '') {
  if (currentObjectUrl) URL.revokeObjectURL(currentObjectUrl)
  currentObjectUrl = blob ? URL.createObjectURL(blob) : ''
  dynamicWallpaperUrl.value = currentObjectUrl
  dynamicWallpaperFileName.value = fileName
  applyDynamicWallpaper()
}

export function applyDynamicWallpaper() {
  const root = document.documentElement
  const visible = dynamicWallpaperSettings.enabled && dynamicWallpaperReady.value
  root.dataset.dynamicWallpaper = visible ? 'on' : 'off'
  root.style.setProperty('--wallpaper-dim', String(dynamicWallpaperSettings.dim / 100))
  root.style.setProperty('--wallpaper-panel-alpha', String(dynamicWallpaperSettings.panelOpacity / 100))
  root.style.setProperty('--wallpaper-workspace-alpha', String((dynamicWallpaperSettings.panelOpacity / 100) * 0.72))
  root.style.setProperty('--wallpaper-blur', `${dynamicWallpaperSettings.blur}px`)
}

export async function initDynamicWallpaper() {
  if (loadPromise) return loadPromise
  loadPromise = (async () => {
    dynamicWallpaperLoading.value = true
    dynamicWallpaperError.value = ''
    try {
      const stored = await readStoredVideo()
      if (stored?.blob) replaceObjectUrl(stored.blob, stored.name || '本地动态壁纸')
      else applyDynamicWallpaper()
    } catch (error) {
      dynamicWallpaperError.value = error?.message || '加载动态壁纸失败'
      applyDynamicWallpaper()
    } finally {
      dynamicWallpaperLoading.value = false
    }
  })()
  return loadPromise
}

export async function setDynamicWallpaperFile(file) {
  if (!(file instanceof Blob)) throw new Error('请选择有效的视频文件')
  const isVideo = file.type.startsWith('video/') || /\.(mp4|webm|ogg|mov|m4v)$/i.test(file.name || '')
  if (!isVideo) throw new Error('仅支持 MP4、WebM、Ogg、MOV 等视频文件')

  dynamicWallpaperLoading.value = true
  dynamicWallpaperError.value = ''
  try {
    await writeStoredVideo({ blob: file, name: file.name || '本地动态壁纸', type: file.type })
    replaceObjectUrl(file, file.name || '本地动态壁纸')
    dynamicWallpaperSettings.enabled = true
  } catch (error) {
    const message = error?.name === 'QuotaExceededError'
      ? '视频太大，浏览器存储空间不足。可换用体积更小的 MP4/WebM。'
      : (error?.message || '保存动态壁纸失败')
    dynamicWallpaperError.value = message
    throw new Error(message)
  } finally {
    dynamicWallpaperLoading.value = false
  }
}

export async function clearDynamicWallpaper() {
  dynamicWallpaperLoading.value = true
  dynamicWallpaperError.value = ''
  try {
    await deleteStoredVideo()
    replaceObjectUrl(null)
    dynamicWallpaperSettings.enabled = false
  } catch (error) {
    dynamicWallpaperError.value = error?.message || '移除动态壁纸失败'
  } finally {
    dynamicWallpaperLoading.value = false
  }
}

export function resetDynamicWallpaperAppearance() {
  Object.assign(dynamicWallpaperSettings, DEFAULT_SETTINGS, {
    enabled: dynamicWallpaperReady.value,
  })
}

watch(
  dynamicWallpaperSettings,
  (value) => {
    localStorage.setItem(SETTINGS_KEY, JSON.stringify(value))
    applyDynamicWallpaper()
  },
  { deep: true },
)
