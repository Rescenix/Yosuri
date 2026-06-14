<template>
  <div class="reader-standalone-root">
    <ReaderView />
  </div>
</template>

<script setup>
import ReaderView from './components/ReaderView.vue'
// 不需要任何 setBook 定义，ReaderView 已暴露 window.setBook
</script>

<style scoped>
.reader-standalone-root {
  width: 100vw;
  height: 100vh;
  overflow: hidden;
}
</style>