<template>
  <div class="stn-node">
    <div
      class="stn-row"
      :class="{ active: node.id === activeSession, running: node.id === runningSession, 'just-forked': node.justForked }"
      @mouseenter="$emit('hover', node.id)"
      @mouseleave="$emit('hover-leave', node.id)"
      @click="$emit('select', node)"
    >
      <!-- 有子分支才给箭头；没有的也占同样宽度，免得名字参差不齐 -->
      <button
        v-if="node.children.length"
        class="stn-chevron"
        :class="{ open: expanded }"
        :title="expanded ? '折叠分支' : '展开分支'"
        @click.stop="$emit('toggle', node.id)"
      >
        <Icon icon="mdi:chevron-right" width="13" />
      </button>
      <span v-else class="stn-chevron-spacer"></span>

      <span class="stn-dot" :class="{ on: node.id === runningSession }"></span>
      <span class="stn-name">{{ node.name }}</span>
      <span v-if="node.children.length" class="stn-branch-count">{{ node.children.length }}</span>

      <div v-if="hoveredId === node.id || openMenuId === node.id" class="stn-menu-wrap">
        <button class="stn-menu-btn" title="更多" @click.stop="$emit('menu', node, $event)">
          <Icon icon="mdi:dots-horizontal" width="16" />
        </button>
      </div>
    </div>

    <!-- 子分支：整棵子树共用容器上的一条左边框当导引线，
         比逐层算 padding 简单，任意深度自动嵌套 -->
    <div v-if="expanded && node.children.length" class="stn-children" :class="{ flush: depth >= 4 }">
      <SessionTreeNode
        v-for="child in node.children"
        :key="child.id"
        :node="child"
        :depth="depth + 1"
        :active-session="activeSession"
        :running-session="runningSession"
        :hovered-id="hoveredId"
        :open-menu-id="openMenuId"
        :is-expanded="isExpanded"
        @select="$emit('select', $event)"
        @toggle="$emit('toggle', $event)"
        @menu="(n, e) => $emit('menu', n, e)"
        @hover="$emit('hover', $event)"
        @hover-leave="$emit('hover-leave', $event)"
      />
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { Icon } from '@iconify/vue'

// 递归 SFC：靠文件名自引用（同 FileTreeNode.vue 的做法）。所有交互状态都由
// SessionMenuContent 持有，这里只负责渲染 + 把事件逐层冒泡上去。
const props = defineProps({
  node: { type: Object, required: true },
  depth: { type: Number, default: 0 },
  activeSession: { type: String, default: '' },
  runningSession: { type: String, default: '' },
  hoveredId: { type: String, default: null },
  openMenuId: { type: String, default: null },
  // 展开态存在父组件（节点来自 computed，直接写 node.expanded 下次重算就丢了）
  isExpanded: { type: Function, required: true }
})
defineEmits(['select', 'toggle', 'menu', 'hover', 'hover-leave'])

const expanded = computed(() => props.isExpanded(props.node))
</script>

<style scoped>
/* 全部用主题变量：SessionMenuContent 那边的行样式硬编码了浅色值，暗色模式下是坏的，
   新代码不继承那个毛病 */
.stn-row {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 6px 8px;
  border-radius: 8px;
  cursor: pointer;
  font-size: 13px;
  color: var(--app-text, #1a1a1a);
  transition: background 0.15s ease;
}
.stn-row:hover { background: color-mix(in srgb, var(--app-text, #1a1a1a), transparent 94%); }
.stn-row.active { background: color-mix(in srgb, var(--app-text, #1a1a1a), transparent 92%); }
.stn-row.running { background: var(--app-accent-soft, rgba(201, 100, 66, 0.12)); }

.stn-chevron,
.stn-chevron-spacer {
  flex: 0 0 auto;
  width: 16px;
  height: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
}
.stn-chevron {
  border: none;
  background: none;
  padding: 0;
  cursor: pointer;
  color: var(--app-text-soft, #6b6b6b);
  transition: transform 0.18s ease;
}
.stn-chevron.open { transform: rotate(90deg); }

.stn-dot {
  flex: 0 0 auto;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: transparent;
}
/* 运行中的脉冲点在任意深度都渲染，分支跑起来和根会话一样显眼 */
.stn-dot.on {
  background: var(--app-accent, #c96442);
  animation: stn-dot-pulse 1.4s ease-in-out infinite;
}
@keyframes stn-dot-pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.45; transform: scale(0.7); }
}

.stn-name {
  flex: 1 1 auto;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.stn-branch-count {
  flex: 0 0 auto;
  font-size: 11px;
  padding: 0 5px;
  border-radius: 8px;
  color: var(--app-text-faint, #a3a3a3);
  background: color-mix(in srgb, var(--app-text-faint, #a3a3a3), transparent 88%);
}

.stn-menu-wrap { flex: 0 0 auto; display: flex; }
.stn-menu-btn {
  border: none;
  background: none;
  padding: 0 2px;
  cursor: pointer;
  color: var(--app-text-soft, #6b6b6b);
  display: flex;
  align-items: center;
}
.stn-menu-btn:hover { color: var(--app-text, #1a1a1a); }

/* 导引线：一条左边框画出整棵子树的血缘 */
.stn-children {
  margin-left: 18px;
  padding-left: 6px;
  border-left: 1px solid color-mix(in srgb, var(--app-text-faint, #a3a3a3), transparent 50%);
  animation: stn-expand 0.18s ease;
}
/* 太深就不再往右缩了，否则 260px 的侧栏会被吃光、名字没地方显示 */
.stn-children.flush { margin-left: 0; }
@keyframes stn-expand {
  from { opacity: 0; transform: translateY(-2px); }
  to { opacity: 1; transform: translateY(0); }
}

/* 刚分叉出来的那条高亮一下，让用户一眼看到新分支落在哪 */
.stn-row.just-forked { animation: stn-just-forked 1.2s ease-out; }
@keyframes stn-just-forked {
  0% { background: var(--app-accent-soft, rgba(201, 100, 66, 0.12)); }
  100% { background: transparent; }
}
</style>
