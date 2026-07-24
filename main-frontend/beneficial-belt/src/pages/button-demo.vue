<template>
  <div class="demo-page">
    <h1 class="demo-title">✨ BaseButton · 按钮组件展示</h1>
    <p class="demo-subtitle">杉汐主题 · 四种变体 × 三种尺寸 × 三种状态</p>

    <!-- ====== Variants 变体 ====== -->
    <section class="demo-section">
      <h2 class="section-title">🎨 Variant · 变体</h2>
      <div class="demo-row">
        <BaseButton variant="primary">Primary</BaseButton>
        <BaseButton variant="secondary">Secondary</BaseButton>
        <BaseButton variant="ghost">Ghost</BaseButton>
        <BaseButton variant="danger">Danger</BaseButton>
      </div>
    </section>

    <!-- ====== Sizes 尺寸 ====== -->
    <section class="demo-section">
      <h2 class="section-title">📐 Size · 尺寸（primary）</h2>
      <div class="demo-row">
        <BaseButton variant="primary" size="sm">Small</BaseButton>
        <BaseButton variant="primary" size="md">Medium</BaseButton>
        <BaseButton variant="primary" size="lg">Large</BaseButton>
      </div>
    </section>

    <!-- ====== Loading 加载状态 ====== -->
    <section class="demo-section">
      <h2 class="section-title">⏳ Loading · 加载中</h2>
      <div class="demo-row">
        <BaseButton variant="primary" loading>Primary</BaseButton>
        <BaseButton variant="secondary" loading>Secondary</BaseButton>
        <BaseButton variant="ghost" loading>Ghost</BaseButton>
        <BaseButton variant="danger" loading>Danger</BaseButton>
      </div>
    </section>

    <!-- ====== Disabled 禁用状态 ====== -->
    <section class="demo-section">
      <h2 class="section-title">🚫 Disabled · 禁用</h2>
      <div class="demo-row">
        <BaseButton variant="primary" disabled>Primary</BaseButton>
        <BaseButton variant="secondary" disabled>Secondary</BaseButton>
        <BaseButton variant="ghost" disabled>Ghost</BaseButton>
        <BaseButton variant="danger" disabled>Danger</BaseButton>
      </div>
    </section>

    <!-- ====== 前缀/后缀图标 ====== -->
    <section class="demo-section">
      <h2 class="section-title">🎭 Icons · 前缀 &amp; 后缀图标</h2>
      <div class="demo-row">
        <BaseButton variant="primary">
          <template #prefix><Icon icon="ion:star" /></template>
          收藏
        </BaseButton>
        <BaseButton variant="secondary">
          <template #prefix><Icon icon="ion:heart" /></template>
          点赞
        </BaseButton>
        <BaseButton variant="ghost">
          设置
          <template #suffix><Icon icon="ion:settings-outline" /></template>
        </BaseButton>
        <BaseButton variant="danger">
          <template #prefix><Icon icon="ion:trash-outline" /></template>
          删除
        </BaseButton>
      </div>
    </section>

    <!-- ====== 综合展示 ====== -->
    <section class="demo-section">
      <h2 class="section-title">🌈 Combo · 组合展示</h2>
      <div class="demo-row demo-row--wrap">
        <BaseButton variant="primary" size="lg" loading>
          <template #prefix><Icon icon="ion:cloud-upload-outline" /></template>
          上传中...
        </BaseButton>
        <BaseButton variant="danger" size="lg">
          <template #prefix><Icon icon="ion:alert-circle" /></template>
          确认删除
        </BaseButton>
        <BaseButton variant="secondary" size="sm" disabled>
          <template #prefix><Icon icon="ion:lock-closed" /></template>
          已锁定
        </BaseButton>
        <BaseButton variant="ghost" size="lg">
          <template #prefix><Icon icon="ion:logo-github" /></template>
          GitHub
          <template #suffix><Icon icon="ion:open-outline" /></template>
        </BaseButton>
      </div>
    </section>
  </div>
</template>

<script setup>
import { Icon } from '@iconify/vue'
import BaseButton from '../components/base/BaseButton.vue'
</script>

<style scoped>
.demo-page {
  max-width: 960px;
  margin: 0 auto;
  padding: 48px 24px;
  font-family: var(--app-font, 'Inter', system-ui, 'PingFang SC', sans-serif);
}

.demo-title {
  font-size: 32px;
  font-weight: 700;
  margin-bottom: 6px;
  color: var(--app-text, #1a1a1a);
}

.demo-subtitle {
  font-size: 15px;
  color: var(--app-text-soft, #6b6b6b);
  margin-top: 0;
  margin-bottom: 40px;
}

.demo-section {
  margin-bottom: 36px;
}

.section-title {
  font-size: 16px;
  font-weight: 600;
  margin-bottom: 14px;
  color: var(--app-text, #1a1a1a);
}

.demo-row {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.demo-row--wrap {
  gap: 16px;
}
</style>
