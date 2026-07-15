<template>
  <div>
    <NavBar />
    <ImageGallery />
  </div>
</template>

<script setup>
import { onMounted } from 'vue'
import NavBar from '../components/NavBar.vue'
import ImageGallery from '../components/image-bed/ImageGallery.vue'

onMounted(() => {
  document.title = '图床 | 杉汐的星尘核心'
})
</script>
