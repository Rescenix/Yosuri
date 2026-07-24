<template>
  <button
    class="base-btn"
    :class="[
      `base-btn--${variant}`,
      `base-btn--${size}`,
      {
        'base-btn--loading': loading,
        'base-btn--disabled': disabled,
      },
    ]"
    :disabled="disabled || loading"
    :type="type"
    v-bind="$attrs"
  >
    <!-- Loading 旋转图标 -->
    <span v-if="loading" class="base-btn__spinner">
      <Icon icon="svg-spinners:90-ring-with-bg" width="1em" height="1em" />
    </span>

    <!-- 前缀图标 slot -->
    <span v-else-if="$slots.prefix" class="base-btn__prefix">
      <slot name="prefix" />
    </span>

    <!-- 按钮文本 -->
    <span v-if="$slots.default" class="base-btn__text">
      <slot />
    </span>

    <!-- 后缀图标 slot -->
    <span v-if="$slots.suffix" class="base-btn__suffix">
      <slot name="suffix" />
    </span>
  </button>
</template>

<script setup>
/**
 * BaseButton · 基础按钮组件
 * 支持 primary / secondary / ghost / danger 四种变体，
 * sm / md / lg 三种尺寸，loading 与 disabled 状态，
 * 以及前缀/后缀图标 slot。
 */
import { Icon } from '@iconify/vue'

defineOptions({
  inheritAttrs: true,
})

const props = defineProps({
  /** 按钮变体类型 */
  variant: {
    type: String,
    default: 'primary',
    validator: (v) => ['primary', 'secondary', 'ghost', 'danger'].includes(v),
  },
  /** 按钮尺寸 */
  size: {
    type: String,
    default: 'md',
    validator: (v) => ['sm', 'md', 'lg'].includes(v),
  },
  /** 加载中状态 */
  loading: {
    type: Boolean,
    default: false,
  },
  /** 禁用状态 */
  disabled: {
    type: Boolean,
    default: false,
  },
  /** 原生 button type */
  type: {
    type: String,
    default: 'button',
  },
})
</script>

<style scoped>
/* ========================================
   BaseButton · 星尘按钮样式
   基于项目全局 CSS 变量（--app-*）设计
   ======================================== */

.base-btn {
  /* 重置默认样式 */
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  border: 1px solid transparent;
  border-radius: 8px;
  font-family: var(--app-font, 'Inter', system-ui, sans-serif);
  font-weight: 600;
  line-height: 1;
  white-space: nowrap;
  cursor: pointer;
  outline: none;
  transition: all 0.2s ease;
  position: relative;
  user-select: none;
}

/* 禁用 & loading 状态 */
.base-btn--disabled,
.base-btn--loading {
  opacity: 0.5;
  cursor: not-allowed;
  pointer-events: none;
}

/* ==========================================
   尺寸变体
   ========================================== */
.base-btn--sm {
  font-size: 12px;
  padding: 6px 12px;
  min-height: 28px;
  border-radius: 6px;
}

.base-btn--md {
  font-size: 14px;
  padding: 8px 18px;
  min-height: 36px;
}

.base-btn--lg {
  font-size: 16px;
  padding: 12px 24px;
  min-height: 44px;
  border-radius: 10px;
}

/* ==========================================
   变体：primary
   ========================================== */
.base-btn--primary {
  background: var(--app-accent, #c96442);
  border-color: var(--app-accent, #c96442);
  color: #ffffff;
}

.base-btn--primary:hover:not(:disabled) {
  background: var(--app-accent-hover, #b85737);
  border-color: var(--app-accent-hover, #b85737);
  box-shadow: 0 4px 16px var(--app-accent-soft, rgba(201, 100, 66, 0.3));
  transform: translateY(-1px);
}

.base-btn--primary:active:not(:disabled) {
  transform: translateY(0);
  box-shadow: none;
}

/* ==========================================
   变体：secondary
   ========================================== */
.base-btn--secondary {
  background: var(--app-surface-2, #fafafa);
  border-color: var(--app-border, #e5e5e5);
  color: var(--app-text, #1a1a1a);
}

.base-btn--secondary:hover:not(:disabled) {
  background: var(--app-surface-3, #f4f4f5);
  border-color: var(--app-text-soft, #6b6b6b);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
  transform: translateY(-1px);
}

.base-btn--secondary:active:not(:disabled) {
  transform: translateY(0);
  box-shadow: none;
}

/* ==========================================
   变体：ghost
   ========================================== */
.base-btn--ghost {
  background: transparent;
  border-color: transparent;
  color: var(--app-text-soft, #6b6b6b);
}

.base-btn--ghost:hover:not(:disabled) {
  background: var(--app-accent-soft, rgba(201, 100, 66, 0.08));
  color: var(--app-accent, #c96442);
  transform: translateY(-1px);
}

.base-btn--ghost:active:not(:disabled) {
  transform: translateY(0);
}

/* ==========================================
   变体：danger
   ========================================== */
.base-btn--danger {
  background: #fef2f2;
  border-color: #fecaca;
  color: #dc2626;
}

.base-btn--danger:hover:not(:disabled) {
  background: #fee2e2;
  border-color: #fca5a5;
  box-shadow: 0 4px 16px rgba(220, 38, 38, 0.15);
  transform: translateY(-1px);
}

.base-btn--danger:active:not(:disabled) {
  background: #fecaca;
  transform: translateY(0);
  box-shadow: none;
}

/* 暗色适配：danger 在暗色下 */
:root.dark .base-btn--danger {
  background: rgba(220, 38, 38, 0.1);
  border-color: rgba(220, 38, 38, 0.3);
  color: #fca5a5;
}

:root.dark .base-btn--danger:hover:not(:disabled) {
  background: rgba(220, 38, 38, 0.2);
  border-color: #dc2626;
}

/* ==========================================
   内部元素
   ========================================== */
.base-btn__spinner {
  display: flex;
  align-items: center;
  animation: base-btn-spin 0.8s linear infinite;
}

.base-btn__prefix,
.base-btn__suffix {
  display: flex;
  align-items: center;
  font-size: 1.15em;
}

.base-btn__text {
  display: inline-flex;
  align-items: center;
}

/* Loading 旋转动画 */
@keyframes base-btn-spin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}
</style>
