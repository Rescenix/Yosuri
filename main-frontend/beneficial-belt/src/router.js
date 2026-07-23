import { createRouter, createWebHistory } from 'vue-router';

const routes = [
  { path: '/', name: 'home', component: () => import('./views/HomeView.vue') },
  { path: '/chat', name: 'chat', component: () => import('./views/ChatView.vue') },
  { path: '/image-bed', name: 'image-bed', component: () => import('./views/ImageBedView.vue') },
  { path: '/:pathMatch(.*)*', redirect: '/' },
];

export default createRouter({
  history: createWebHistory(),
  routes,
});
