<template>
  <div v-if="attachments.length" class="attach-chip-row">
    <div v-for="att in attachments" :key="att.id" class="attach-chip" :class="[att.kind, att.status]">
      <img
        v-if="att.kind === 'image'"
        :src="att.previewUrl"
        class="attach-chip-thumb"
        @click="openPreview(att.previewUrl)"
        title="点击查看大图"
      />
      <div v-else class="attach-chip-icon">
        <Icon v-if="att.kind === 'folder'" icon="mdi:folder-outline" width="20" color="#94a3b8" />
        <Icon v-else-if="att.kind === 'video'" icon="mdi:video-outline" width="20" color="#94a3b8" />
        <span v-else>{{ att.ext }}</span>
      </div>
      <div class="attach-chip-meta">
        <span class="attach-chip-name" :title="att.name">{{ att.name }}</span>
        <span v-if="att.status === 'analyzing'" class="attach-chip-status">分析中…</span>
        <span v-else-if="att.status === 'error'" class="attach-chip-status error">{{ att.errorMsg }}</span>
        <span v-else-if="att.kind === 'folder'" class="attach-chip-status">{{ att.fileCount }} 个文件</span>
      </div>
      <button v-if="removable" class="attach-chip-remove" type="button" @click="$emit('remove', att.id)" title="移除">
        <Icon icon="mdi:close" width="11" />
      </button>
    </div>

    <!-- 图片悬浮预览：用 Teleport 挂到 body，避免被父级 transform/overflow 裁切 -->
    <Teleport to="body">
      <div
        v-if="currentIndex !== -1"
        class="image-preview-overlay"
        @click.self="closePreview"
        @wheel.prevent="onWheel"
      >
        <div class="image-preview-card" @click.stop>
          <!-- 顶部工具栏：position:fixed（跟底部工具条一样），不占布局、
               不会被 transform 放大的图片盖住——之前它在文档流里，
               图片一放大就从视觉上把它糊住了 -->
          <div class="image-preview-header">
            <span class="image-preview-title">{{ currentImage?.name || '图片预览' }}</span>
            <button class="image-preview-close" @click="closePreview" title="关闭 (Esc)">
              <Icon icon="mdi:close" width="20" />
            </button>
          </div>

          <!-- 图片容器：滚轮缩放挪到最外层 overlay 了，这里只留拖拽平移 -->
          <div
            class="image-preview-stage"
            @mousedown="onDragStart"
            @mousemove="onDragMove"
            @mouseup="onDragEnd"
            @mouseleave="onDragEnd"
          >
            <img
              v-if="currentImage"
              :src="currentImage.previewUrl"
              class="image-preview-img"
              :style="imageStyle"
              draggable="false"
            />
          </div>

          <!-- 底部工具栏 -->
          <div class="image-preview-toolbar">
            <button class="toolbar-btn" :disabled="currentIndex <= 0" @click="prevImage" title="上一张">
              <Icon icon="mdi:chevron-left" width="18" />
            </button>
            <span class="toolbar-page">{{ currentIndex + 1 }} / {{ imageAttachments.length }}</span>
            <button class="toolbar-btn" :disabled="currentIndex >= imageAttachments.length - 1" @click="nextImage" title="下一张">
              <Icon icon="mdi:chevron-right" width="18" />
            </button>
            <div class="toolbar-divider"></div>
            <button class="toolbar-btn" @click="zoomOut" title="缩小">
              <Icon icon="mdi:magnify-minus" width="18" />
            </button>
            <span class="toolbar-zoom">{{ Math.round(scale * 100) }}%</span>
            <button class="toolbar-btn" @click="zoomIn" title="放大">
              <Icon icon="mdi:magnify-plus" width="18" />
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup>
// 附件预览 chip 行——输入框上方（可移除）和用户消息气泡里（只读回放）共用同一份，
// 之前工作流气泡是把附件内容整段拼进纯文本显示，太丑；现在气泡也用这份 chip
// 展示"贴过什么"，正文只留用户自己敲的话
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { Icon } from '@iconify/vue'

const props = defineProps({
  attachments: { type: Array, default: () => [] },
  removable: { type: Boolean, default: false }
})
defineEmits(['remove'])

const imageAttachments = computed(() => props.attachments.filter(a => a.kind === 'image'))
const currentIndex = ref(-1)
const currentImage = computed(() => imageAttachments.value[currentIndex.value] || null)

const scale = ref(1)
const offsetX = ref(0)
const offsetY = ref(0)
const isDragging = ref(false)
const dragStartX = ref(0)
const dragStartY = ref(0)
const dragStartOffsetX = ref(0)
const dragStartOffsetY = ref(0)

const imageStyle = computed(() => ({
  transform: `translate(${offsetX.value}px, ${offsetY.value}px) scale(${scale.value})`,
  cursor: isDragging.value ? 'grabbing' : 'grab'
}))

function openPreview(url) {
  const idx = imageAttachments.value.findIndex(a => a.previewUrl === url)
  currentIndex.value = idx >= 0 ? idx : 0
  resetZoom()
}

function closePreview() {
  currentIndex.value = -1
  resetZoom()
}

function prevImage() {
  if (currentIndex.value > 0) {
    currentIndex.value--
    resetZoom()
  }
}

function nextImage() {
  if (currentIndex.value < imageAttachments.value.length - 1) {
    currentIndex.value++
    resetZoom()
  }
}

function zoomIn() {
  scale.value = Math.min(scale.value + 0.25, 5)
}

function zoomOut() {
  scale.value = Math.max(scale.value - 0.25, 0.25)
}

function resetZoom() {
  scale.value = 1
  offsetX.value = 0
  offsetY.value = 0
}

function onWheel(e) {
  const delta = e.deltaY > 0 ? -0.15 : 0.15
  scale.value = Math.min(Math.max(scale.value + delta, 0.25), 5)
}

function onDragStart(e) {
  isDragging.value = true
  dragStartX.value = e.clientX
  dragStartY.value = e.clientY
  dragStartOffsetX.value = offsetX.value
  dragStartOffsetY.value = offsetY.value
}

function onDragMove(e) {
  if (!isDragging.value) return
  offsetX.value = dragStartOffsetX.value + (e.clientX - dragStartX.value)
  offsetY.value = dragStartOffsetY.value + (e.clientY - dragStartY.value)
}

function onDragEnd() {
  isDragging.value = false
}

function onKeydown(e) {
  if (currentIndex.value === -1) return
  if (e.key === 'Escape') closePreview()
  else if (e.key === 'ArrowLeft') prevImage()
  else if (e.key === 'ArrowRight') nextImage()
  else if (e.key === '+' || e.key === '=') zoomIn()
  else if (e.key === '-') zoomOut()
  else if (e.key === '0') resetZoom()
}

onMounted(() => window.addEventListener('keydown', onKeydown))
onUnmounted(() => window.removeEventListener('keydown', onKeydown))
</script>

<style scoped>
.attach-chip-row {
  display: flex;
  gap: 8px;
  overflow-x: auto;
}
.attach-chip {
  position: relative;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  width: 168px;
  padding: 6px 22px 6px 6px;
  border: 1px solid var(--app-border);
  border-radius: 10px;
  background: var(--app-surface-2);
  box-sizing: border-box;
}
.attach-chip.error { border-color: #f3c9c2; background: var(--app-surface-2); }
.attach-chip.analyzing { opacity: 0.7; }
.attach-chip-thumb { width: 36px; height: 36px; border-radius: 7px; object-fit: cover; flex-shrink: 0; }
.attach-chip-icon {
  width: 36px; height: 36px; border-radius: 7px; flex-shrink: 0;
  display: flex; align-items: center; justify-content: center;
  background: var(--app-surface-3); color: var(--app-text-soft); font-size: 10px; font-weight: 700;
  font-family: "JetBrains Mono", ui-monospace, Menlo, monospace;
}
.attach-chip-meta { min-width: 0; display: flex; flex-direction: column; gap: 1px; }
.attach-chip-name { font-size: 12px; color: var(--app-text); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.attach-chip-status { font-size: 10.5px; color: var(--app-text-faint); }
.attach-chip-status.error { color: #d94834; }
.attach-chip-remove {
  position: absolute; top: -5px; right: -5px;
  width: 16px; height: 16px; border-radius: 50%;
  display: flex; align-items: center; justify-content: center;
  background: #404040; color: #fff; border: 1.5px solid #fff;
  cursor: pointer; padding: 0;
}
.attach-chip-remove:hover { background: #1a1a1a; }

/* 图片悬浮预览 */
.attach-chip-thumb { cursor: pointer; }
.image-preview-overlay {
  position: fixed;
  inset: 0;
  z-index: 10001;
  overflow: hidden; /* 全屏灯箱，缩放/平移用 transform 控制，不需要外层滚动 */
  background: rgba(0, 0, 0, 0.55);
}
.image-preview-card {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  /* 之前这里 max-width:420px 把整个预览锁死在一个窄列里，图不管缩不缩放
     两边都是大片空白——现在让它铺满视口，图片自身用 vw/vh 上限控制大小 */
}
.image-preview-header {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  z-index: 2;
  height: 52px;
  padding: 0 16px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  /* 图片全屏铺开后，头部可能直接压在图片像素上，纯 text-shadow 不够稳；
     加一层顶部渐变遮罩兜底可读性 */
  background: linear-gradient(to bottom, rgba(0, 0, 0, 0.55), transparent);
}
.image-preview-title {
  font-size: 13px;
  font-weight: 500;
  color: #fff;
  max-width: 70%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-shadow: 0 1px 3px rgba(0, 0, 0, 0.35);
}
.image-preview-close {
  width: 30px;
  height: 30px;
  border-radius: 8px;
  border: none;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.9);
  color: #333;
  cursor: pointer;
  transition: background 0.2s;
}
.image-preview-close:hover { background: #fff; }
.image-preview-stage {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  /* 给固定头/尾工具条留呼吸区，图片在剩下的可视区域里居中，而不是贴着头尾 */
  padding: 70px 24px 96px;
  box-sizing: border-box;
  overflow: visible; /* 高倍缩放时图片会超出这个盒子，得让它露出来才能拖动看到 */
  user-select: none;
}
.image-preview-img {
  max-width: 92vw;
  max-height: 78vh;
  border-radius: 6px;
  object-fit: contain;
  transition: transform 0.05s linear;
  user-select: none;
  -webkit-user-drag: none;
}
.image-preview-toolbar {
  position: fixed;
  left: 50%;
  bottom: 24px;
  transform: translateX(-50%);
  height: 38px;
  padding: 0 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  background: rgba(255, 255, 255, 0.92);
  border-radius: 999px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.15);
}
.toolbar-btn {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  border: none;
  display: flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  color: #333;
  cursor: pointer;
  transition: background 0.15s;
}
.toolbar-btn:hover:not(:disabled) { background: rgba(0, 0, 0, 0.08); }
.toolbar-btn:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}
.toolbar-page {
  min-width: 40px;
  text-align: center;
  font-size: 12px;
  color: #444;
  font-weight: 500;
}
.toolbar-zoom {
  min-width: 40px;
  text-align: center;
  font-size: 12px;
  color: #444;
  font-weight: 500;
}
.toolbar-divider {
  width: 1px;
  height: 16px;
  background: rgba(0, 0, 0, 0.1);
  margin: 0 4px;
}
</style>
