<template>
  <div style="padding-top: 100px">
    <NavBar />
    <div class="blog-container">
      <PostList />
    </div>
  </div>
</template>

<script setup>
import { onMounted } from 'vue'
import NavBar from '../components/NavBar.vue'
import PostList from '../components/blog/PostList.vue'

onMounted(() => {
  document.title = '博客 | Aurora'
})
</script>

<style scoped>
.blog-container {
  max-width: 900px;
  margin: 0 auto;
  padding: 0 1.5rem 4rem;
}
@media (max-width: 640px) {
  .blog-container { padding: 0 1rem 3rem; }
}
</style>
