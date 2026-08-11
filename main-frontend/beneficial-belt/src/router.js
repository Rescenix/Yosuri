import { createRouter, createWebHistory } from 'vue-router';

const routes = [
  { path: '/', name: 'home', component: () => import('./views/StudioView.vue') },
  { path: '/studio', name: 'studio', redirect: '/' },
  { path: '/chat', name: 'chat', component: () => import('./views/ChatView.vue') },
    { path: '/publish', name: 'publish', component: () => import('./views/PublishView.vue') },
      { path: '/company', name: 'company', component: () => import('./views/CompanyView.vue') },
        { path: '/sync', redirect: '/company' },
        { path: '/:pathMatch(.*)*', redirect: '/' },
];

export default createRouter({
  history: createWebHistory(),
  routes,
});
