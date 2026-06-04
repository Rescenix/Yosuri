<template>
  <div class="gallery-container">
    <!-- 上传区域 -->
    <div v-if="isLoggedIn" class="upload-area">
      <input
        type="file"
        accept="image/*"
        multiple
        style="display: none"
        ref="fileInput"
        @change="handleUpload"
      />
      <button class="upload-btn" @click="$refs.fileInput.click()">
        <Icon icon="mdi:cloud-upload" width="20" />
        <span>上传图片</span>
      </button>
    </div>

    <!-- 标签筛选栏（每个标签右侧加删除按钮） -->
    <div class="filter-bar" v-if="allTags.length">
      <span class="filter-label">筛选标签：</span>
      <div class="filter-tags">
        <div
          v-for="tag in allTags"
          :key="tag"
          class="filter-tag-wrapper"
        >
          <button
            class="filter-tag"
            :class="{ active: selectedFilters.includes(tag) }"
            @click="toggleFilter(tag)"
          >{{ tag }}</button>
          <button
            v-if="isLoggedIn"
            class="delete-tag-btn"
            @click.stop="deleteTag(tag)"
            title="删除此标签（会从所有图片中移除）"
          >
            <Icon icon="mdi:close" width="14" />
          </button>
        </div>
      </div>
      <button v-if="selectedFilters.length" class="clear-filter" @click="clearFilters">清除</button>
    </div>

    <!-- 图片瀑布流 -->
    <div class="masonry" v-if="filteredImages.length">
      <div class="masonry-item" v-for="img in filteredImages" :key="img.url">
        <div class="image-wrapper">
          <img :src="img.url" :alt="img.rel_path" loading="lazy" />
          <div class="image-overlay">
            <button class="overlay-btn" @click="openOriginal(img.url)" title="查看原图">
              <Icon icon="mdi:open-in-new" width="18" />
            </button>
            <button class="overlay-btn" @click="copyUrl(img.url)" title="复制链接">
              <Icon icon="mdi:content-copy" width="18" />
            </button>
            <button v-if="isLoggedIn" class="overlay-btn delete-btn" @click="deleteImage(img)" title="删除">
              <Icon icon="mdi:delete-outline" width="18" />
            </button>
          </div>
        </div>

        <!-- 标签编辑区（显示已有标签+添加新标签） -->
        <div class="tags-edit">
          <div class="tag-list">
            <span v-for="tag in img.tags" :key="tag" class="tag-badge">
              {{ tag }}
              <span class="remove-tag" @click="removeTag(img, tag)">×</span>
            </span>
            <input
              v-model="img.newTagInput"
              @keyup.enter="addTag(img, img.newTagInput)"
              @blur="addTag(img, img.newTagInput)"
              placeholder="添加标签"
              class="tag-input-small"
            />
          </div>
        </div>
      </div>
    </div>

    <!-- 空状态 -->
    <div v-else class="empty-state">
      <Icon icon="mdi:image-outline" width="48" color="#ccc" />
      <p>没有图片</p>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { Icon } from '@iconify/vue'

const isLoggedIn = ref(!!localStorage.getItem('token'))
const images = ref([])
const selectedFilters = ref([])   // 选中的标签数组（AND 逻辑）
const fileInput = ref(null)

// 获取所有标签
const allTags = computed(() => {
  const tags = new Set()
  images.value.forEach(img => {
    if (img.tags) img.tags.forEach(t => tags.add(t))
  })
  return Array.from(tags).sort()
})

// 筛选后的图片（AND：必须包含所有选中标签）
const filteredImages = computed(() => {
  if (selectedFilters.value.length === 0) return images.value
  return images.value.filter(img => {
    if (!img.tags || img.tags.length === 0) return false
    return selectedFilters.value.every(filterTag => img.tags.includes(filterTag))
  })
})

// 切换筛选标签
// 切换筛选标签（单选模式）
function toggleFilter(tag) {
  // 如果点击的是当前已选中的标签，则取消选中；否则替换为当前标签
  if (selectedFilters.value.length === 1 && selectedFilters.value[0] === tag) {
    selectedFilters.value = []
  } else {
    selectedFilters.value = [tag]
  }
}

function clearFilters() {
  selectedFilters.value = []
}

async function handleUpload(event) {
  const files = event.target.files
  if (!files.length) return
  for (const file of files) {
    const formData = new FormData()
    formData.append('file', file)
    try {
      const res = await fetch('/api/upload', {
        method: 'POST',
        headers: { 'Authorization': `Bearer ${localStorage.getItem('token')}` },
        body: formData
      })
      if (res.ok) {
        // 延迟 500ms 再刷新，确保后端缓存已更新
        await new Promise(resolve => setTimeout(resolve, 500))
        await loadImages()
      }
    } catch (e) { console.error('上传失败:', e) }
  }
  fileInput.value.value = ''
}
async function loadImages() {
  try {
    const res = await fetch('/api/images')
    if (res.ok) {
      const raw = await res.json()
      // 为每张图片添加临时编辑字段
      images.value = raw.map(img => ({
        ...img,
        newTagInput: ''
      }))
    }
  } catch (e) { console.error('加载图片列表失败:', e) }
}

// 添加标签
async function addTag(img, inputValue) {
  if (!inputValue || !inputValue.trim()) {
    img.newTagInput = ''
    return
  }
  const newTags = [...(img.tags || []), inputValue.trim()]
  const uniqueTags = [...new Set(newTags)]
  await updateTags(img, uniqueTags)
  img.newTagInput = ''
}

// 删除单张图片上的某个标签
async function removeTag(img, tagToRemove) {
  const newTags = (img.tags || []).filter(t => t !== tagToRemove)
  await updateTags(img, newTags)
}

// 通用标签更新函数
async function updateTags(img, newTags) {
  try {
    const res = await fetch('/api/images/tag', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        rel_path: img.rel_path,
        tags: newTags
      })
    })
    if (res.ok) {
      img.tags = newTags
    } else {
      console.error('标签更新失败')
    }
  } catch (err) {
    console.error('标签更新异常', err)
  }
}

// ========== 新增：彻底删除某个标签（从所有图片中移除） ==========
async function deleteTag(tag) {
  if (!confirm(`确定要删除标签「${tag}」吗？\n此操作会从所有图片中移除该标签，不可撤销。`)) {
    return
  }
  try {
    // 需要后端提供接口：DELETE /api/tags 或 POST /api/tags/delete
    // 请求体：{ tag: "标签名" }
    const res = await fetch('/api/tags', {
      method: 'DELETE',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('token')}`
      },
      body: JSON.stringify({ tag })
    })
    if (res.ok) {
      // 删除成功后重新加载图片列表（标签自然消失）
      await loadImages()
      // 同时清除筛选列表中可能包含的该标签
      if (selectedFilters.value.includes(tag)) {
        selectedFilters.value = selectedFilters.value.filter(t => t !== tag)
      }
    } else {
      const err = await res.json()
      alert('删除标签失败：' + (err.error || '未知错误'))
    }
  } catch (e) {
    console.error('删除标签请求异常', e)
    alert('删除标签失败，请检查网络')
  }
}

async function deleteImage(img) {
  if (!confirm('确定删除这张图片吗？')) return
  try {
    const res = await fetch('/api/images/remove', {
      method: 'DELETE',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('token')}`
      },
      body: JSON.stringify({ rel_paths: [img.rel_path] })
    })
    if (res.ok) {
      await loadImages()
    } else {
      const err = await res.json()
      alert('删除失败：' + (err.error || '未知错误'))
    }
  } catch (e) {
    console.error('删除请求异常', e)
    alert('删除失败')
  }
}

function openOriginal(url) {
  window.open(window.location.origin + url, '_blank')
}

function copyUrl(url) {
  navigator.clipboard.writeText(window.location.origin + url).then(() => alert('链接已复制'))
}

onMounted(() => {
  loadImages()
})
</script>

<style scoped>
.gallery-container {
  max-width: 1400px;
  margin: 0 auto;
  padding: 80px 20px 40px 20px;   /* 顶部留出导航栏空间（70+10缓冲） */
  overflow-y: auto;
  height: 100%;
  box-sizing: border-box;
}

/* 确保上传区域不被遮挡 */
.upload-area {
  margin-bottom: 30px;
  text-align: center;
  position: relative;
  z-index: 1;
}

.upload-btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 12px 28px;
  background: #2563eb;
  color: white;
  border: none;
  border-radius: 8px;
  font-size: 15px;
  cursor: pointer;
}

.filter-bar {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 24px;
  flex-wrap: wrap;
}

.filter-label {
  font-size: 14px;
  color: #666;
}

.filter-tags {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  align-items: center;
}

.filter-tag-wrapper {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  background: white;
  border-radius: 24px;
  border: 1px solid #ccc;
  padding: 2px 4px 2px 12px;
}

.filter-tag {
  background: transparent;
  border: none;
  padding: 4px 0;
  font-size: 13px;
  cursor: pointer;
  color: #475467;
}

.filter-tag.active {
  color: #2563eb;
  font-weight: 500;
}

.delete-tag-btn {
  background: transparent;
  border: none;
  padding: 2px 4px;
  cursor: pointer;
  border-radius: 12px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #999;
  transition: all 0.2s;
}

.delete-tag-btn:hover {
  background: #fee2e2;
  color: #dc2626;
}

.clear-filter {
  background: none;
  border: none;
  color: #dc2626;
  cursor: pointer;
  font-size: 13px;
}

.masonry {
  columns: 3 300px;
  column-gap: 16px;
}

.masonry-item {
  break-inside: avoid;
  margin-bottom: 24px;           /* 增大底部边距，避免标签被截断 */
  border-radius: 12px;
  background: #fff;
  box-shadow: 0 2px 12px rgba(0,0,0,0.04);
  overflow: visible;             /* 确保标签区域不被裁剪 */
}

.image-wrapper {
  position: relative;
  overflow: hidden;              /* 仅图片区域溢出隐藏，不影响下面标签 */
  border-radius: 12px 12px 0 0;
}

.image-wrapper img {
  width: 100%;
  display: block;
}

.image-overlay {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0,0,0,0.3);
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  opacity: 0;
  transition: opacity 0.2s;
  border-radius: 12px 12px 0 0;
}

.image-wrapper:hover .image-overlay {
  opacity: 1;
}

.overlay-btn {
  background: white;
  border: none;
  width: 36px;
  height: 36px;
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  color: #333;
}

.delete-btn {
  color: #dc2626;
}

.tags-edit {
  padding: 12px;
  border-top: 1px solid #eee;
  background: white;            /* 确保底色一致 */
}

.tag-list {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.tag-badge {
  background: #eef2ff;
  color: #1d4ed8;
  padding: 4px 10px;
  border-radius: 16px;
  font-size: 12px;
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.remove-tag {
  cursor: pointer;
  font-weight: bold;
  font-size: 14px;
  color: #666;
}

.remove-tag:hover {
  color: #dc2626;
}

.tag-input-small {
  border: 1px solid #ddd;
  border-radius: 16px;
  padding: 4px 12px;
  font-size: 12px;
  width: 100px;
  outline: none;
}

.tag-input-small:focus {
  border-color: #2563eb;
}

.empty-state {
  text-align: center;
  padding: 80px;
  color: #999;
}
</style>