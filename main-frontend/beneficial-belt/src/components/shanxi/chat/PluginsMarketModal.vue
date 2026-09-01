<template>
  <Teleport to="body">
    <div class="plugins-market-backdrop" @click.self="$emit('close')">
      <div class="plugins-market-glass">
        <!-- 顶部标题栏 -->
        <div class="plugins-market-head">
          <div class="plugins-market-title-wrap">
            <div class="plugins-market-title">
              <Icon icon="mdi:puzzle-outline" width="22" />
              <span>插件市场</span>
            </div>
            <p class="plugins-market-subtitle">发现并安装插件和技能，扩展 Yosuri 的能力。</p>
          </div>
          <div class="plugins-market-actions">
            <button class="pm-head-btn" @click="onManage">
              <Icon icon="mdi:tune" width="16" />
              <span>管理</span>
            </button>
            <button class="plugins-market-close" @click="$emit('close')">
              <Icon icon="mdi:close" width="18" />
            </button>
          </div>
        </div>

        <!-- 标签页 -->
        <div class="plugins-market-tabs">
          <button
            v-for="tab in tabs"
            :key="tab.key"
            class="pm-tab"
            :class="{ active: activeTab === tab.key }"
            @click="activeTab = tab.key"
          >
            {{ tab.label }}
          </button>
        </div>

        <!-- 工具栏：分类 + 搜索 + 上传 -->
        <div class="plugins-market-toolbar">
          <div class="pm-categories">
            <button
              v-for="cat in categories"
              :key="cat.key"
              class="pm-category-chip"
              :class="{ active: activeCategory === cat.key }"
              @click="activeCategory = cat.key"
            >
              {{ cat.label }}
            </button>
          </div>
          <div class="pm-toolbar-right">
            <div class="pm-search">
              <Icon icon="mdi:magnify" width="16" class="pm-search-icon" />
              <input
                v-model="searchQuery"
                type="text"
                placeholder="搜索技能"
                class="pm-search-input"
              />
            </div>
            <button class="pm-upload-btn" @click="onUpload">
              <Icon icon="mdi:plus" width="16" />
              <span>上传技能</span>
            </button>
          </div>
        </div>

        <!-- 内容区 -->
        <div class="plugins-market-body">
          <div class="pm-section-title">{{ currentCategoryLabel }}</div>
          <div class="pm-grid">
            <div
              v-for="plugin in filteredPlugins"
              :key="plugin.id"
              class="pm-card"
            >
              <div class="pm-card-main">
                <div class="pm-card-icon" :style="{ background: plugin.iconBg }">
                  <Icon :icon="plugin.icon" width="26" :color="plugin.iconColor" />
                </div>
                <div class="pm-card-info">
                  <div class="pm-card-name">{{ plugin.name }}</div>
                  <div class="pm-card-desc">{{ plugin.description }}</div>
                </div>
              </div>
              <button
                class="pm-card-action"
                :class="{ installed: isInstalled(plugin.id) }"
                @click="toggleInstall(plugin.id)"
                :title="isInstalled(plugin.id) ? '已安装' : '安装'"
              >
                <Icon
                  :icon="isInstalled(plugin.id) ? 'mdi:check' : 'mdi:plus'"
                  width="18"
                />
              </button>
            </div>
          </div>

          <div v-if="filteredPlugins.length === 0" class="pm-empty">
            <Icon icon="mdi:store-search-outline" width="48" color="var(--app-text-faint)" />
            <p>暂无相关插件</p>
            <span>试试其他关键词或分类吧～</span>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup>
import { ref, computed } from 'vue'
import { Icon } from '@iconify/vue'

defineEmits(['close'])

const tabs = [
  { key: 'plugins', label: '插件' },
  { key: 'skills', label: '技能' }
]

const categories = [
  { key: 'all', label: '全部' },
  { key: 'devtools', label: '开发工具' },
  { key: 'design', label: '界面设计' },
  { key: 'content', label: '内容创作' },
  { key: 'efficiency', label: '效率提升' },
  { key: 'data', label: '数据分析' }
]

const allPlugins = [
  {
    id: 'github-mcp',
    name: 'GitHub MCP',
    description: '通过 MCP 连接 GitHub，支持仓库搜索、Issue / PR 管理、代码审查等能力。',
    icon: 'mdi:github',
    iconColor: '#ffffff',
    iconBg: '#24292f',
    category: 'devtools',
    tab: 'plugins'
  },
  {
    id: 'figma',
    name: 'Figma',
    description: '通过 MCP 获取 Figma 设计上下文、截图、变量和资源，并将 Figma 节点转译为生产代码。',
    icon: 'simple-icons:figma',
    iconColor: '#ffffff',
    iconBg: 'linear-gradient(135deg, #ff7262, #a259ff)',
    category: 'design',
    tab: 'plugins'
  },
  {
    id: 'canvas',
    name: 'Canvas',
    description: '可视化画布工具，支持流程图、架构图绘制与 AI 辅助设计，让创意一键落地。',
    icon: 'mdi:artboard',
    iconColor: '#ffffff',
    iconBg: 'linear-gradient(135deg, #0ea5e9, #6366f1)',
    category: 'content',
    tab: 'plugins'
  }
]

const activeTab = ref('plugins')
const activeCategory = ref('all')
const searchQuery = ref('')
const installed = ref(new Set())

const currentCategoryLabel = computed(() => {
  const cat = categories.find(c => c.key === activeCategory.value)
  return cat ? cat.label : '全部'
})

const filteredPlugins = computed(() => {
  return allPlugins.filter(p => {
    if (p.tab !== activeTab.value) return false
    if (activeCategory.value !== 'all' && p.category !== activeCategory.value) return false
    const q = searchQuery.value.trim().toLowerCase()
    if (!q) return true
    return p.name.toLowerCase().includes(q) || p.description.toLowerCase().includes(q)
  })
})

function isInstalled(id) {
  return installed.value.has(id)
}

function toggleInstall(id) {
  if (installed.value.has(id)) {
    installed.value.delete(id)
  } else {
    installed.value.add(id)
  }
}

function onManage() {
  // TODO: 跳转插件管理页面
  console.log('[PluginsMarket] open manage')
}

function onUpload() {
  // TODO: 打开上传技能弹窗
  console.log('[PluginsMarket] open upload')
}
</script>

<style scoped>
.plugins-market-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.55);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 99999;
  animation: plugins-fade-in 0.18s ease;
}
@keyframes plugins-fade-in {
  from { opacity: 0; }
  to { opacity: 1; }
}
.plugins-market-glass {
  width: min(960px, 92vw);
  max-height: min(720px, 88vh);
  background: rgba(var(--app-surface-rgb), 0.96);
  backdrop-filter: blur(18px) saturate(180%);
  -webkit-backdrop-filter: blur(18px) saturate(180%);
  border: 1px solid rgba(var(--app-surface-rgb), 0.8);
  border-radius: 18px;
  box-shadow:
    0 0 0 1px rgba(255, 255, 255, 0.35) inset,
    0 24px 64px rgba(0, 0, 0, 0.28);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  animation: plugins-scale-in 0.22s cubic-bezier(0.22, 1, 0.36, 1);
}
@keyframes plugins-scale-in {
  from { opacity: 0; transform: scale(0.96) translateY(8px); }
  to { opacity: 1; transform: scale(1) translateY(0); }
}

/* 头部 */
.plugins-market-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  padding: 22px 26px 18px;
  border-bottom: 1px solid var(--app-border);
  flex-shrink: 0;
  gap: 16px;
}
.plugins-market-title-wrap {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.plugins-market-title {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 20px;
  font-weight: 800;
  color: var(--app-text);
}
.plugins-market-title .iconify { color: var(--app-accent); }
.plugins-market-subtitle {
  margin: 0;
  font-size: 13px;
  color: var(--app-text-soft);
  line-height: 1.5;
}
.plugins-market-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}
.pm-head-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 7px 14px;
  border: 1px solid var(--app-border);
  border-radius: 10px;
  background: var(--app-surface-2);
  color: var(--app-text);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s ease;
}
.pm-head-btn:hover {
  background: var(--app-surface-3);
  border-color: var(--app-border-soft);
}
.plugins-market-close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border: none;
  border-radius: 10px;
  background: transparent;
  color: var(--app-text-faint);
  cursor: pointer;
  transition: all 0.15s ease;
}
.plugins-market-close:hover {
  background: var(--app-surface-2);
  color: var(--app-text);
}

/* 标签页 */
.plugins-market-tabs {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 14px 26px 0;
  flex-shrink: 0;
  border-bottom: 1px solid var(--app-border);
}
.pm-tab {
  position: relative;
  padding: 10px 6px;
  border: none;
  background: transparent;
  color: var(--app-text-soft);
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: color 0.15s ease;
}
.pm-tab:hover { color: var(--app-text); }
.pm-tab.active {
  color: var(--app-text);
  font-weight: 700;
}
.pm-tab.active::after {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  bottom: -1px;
  height: 2px;
  background: var(--app-text);
  border-radius: 2px 2px 0 0;
}

/* 工具栏 */
.plugins-market-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 26px;
  flex-shrink: 0;
  flex-wrap: wrap;
}
.pm-categories {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.pm-category-chip {
  padding: 6px 14px;
  border: 1px solid transparent;
  border-radius: 999px;
  background: transparent;
  color: var(--app-text-soft);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s ease;
}
.pm-category-chip:hover {
  background: var(--app-surface-2);
  color: var(--app-text);
}
.pm-category-chip.active {
  background: var(--app-surface-3);
  color: var(--app-text);
  border-color: var(--app-border);
}
.pm-toolbar-right {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}
.pm-search {
  position: relative;
  display: flex;
  align-items: center;
}
.pm-search-icon {
  position: absolute;
  left: 12px;
  color: var(--app-text-faint);
  pointer-events: none;
}
.pm-search-input {
  width: 200px;
  padding: 8px 12px 8px 34px;
  border: 1px solid var(--app-border);
  border-radius: 10px;
  background: var(--app-surface);
  color: var(--app-text);
  font-size: 13px;
  outline: none;
  transition: all 0.15s ease;
}
.pm-search-input::placeholder { color: var(--app-text-faint); }
.pm-search-input:focus {
  border-color: var(--app-accent);
  box-shadow: 0 0 0 3px var(--app-accent-soft);
}
.pm-upload-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 14px;
  border: none;
  border-radius: 10px;
  background: var(--app-text);
  color: var(--app-surface);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s ease;
}
.pm-upload-btn:hover {
  background: var(--app-text-soft);
  transform: translateY(-1px);
}

/* 内容区 */
.plugins-market-body {
  padding: 6px 26px 26px;
  flex: 1;
  overflow-y: auto;
}
.pm-section-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--app-text-soft);
  margin-bottom: 14px;
}
.pm-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 14px;
}
@media (max-width: 720px) {
  .pm-grid { grid-template-columns: 1fr; }
}
.pm-card {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
  padding: 16px;
  border: 1px solid var(--app-border);
  border-radius: 14px;
  background: var(--app-surface);
  transition: all 0.15s ease;
}
.pm-card:hover {
  border-color: var(--app-border-soft);
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.06);
  transform: translateY(-1px);
}
.pm-card-main {
  display: flex;
  align-items: flex-start;
  gap: 14px;
  min-width: 0;
}
.pm-card-icon {
  flex-shrink: 0;
  width: 48px;
  height: 48px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
}
.pm-card-info {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}
.pm-card-name {
  font-size: 15px;
  font-weight: 700;
  color: var(--app-text);
}
.pm-card-desc {
  font-size: 12.5px;
  color: var(--app-text-soft);
  line-height: 1.6;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.pm-card-action {
  flex-shrink: 0;
  width: 32px;
  height: 32px;
  border: 1px solid var(--app-border);
  border-radius: 10px;
  background: var(--app-surface-2);
  color: var(--app-text);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 0.15s ease;
}
.pm-card-action:hover {
  background: var(--app-surface-3);
  border-color: var(--app-border-soft);
}
.pm-card-action.installed {
  background: var(--app-accent-soft);
  border-color: var(--app-accent-soft);
  color: var(--app-accent);
}

.pm-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 48px 20px;
  text-align: center;
  color: var(--app-text);
}
.pm-empty p {
  margin: 0;
  font-size: 15px;
  font-weight: 700;
}
.pm-empty span {
  font-size: 13px;
  color: var(--app-text-faint);
}
</style>
