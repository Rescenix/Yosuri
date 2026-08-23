import { createRouter, createWebHistory } from 'vue-router';

const routes = [
  { path: '/', redirect: '/chat' },
  { path: '/studio', name: 'studio', component: () => import('./views/StudioView.vue') },
  { path: '/chat', name: 'chat', component: () => import('./views/ChatView.vue') },
  { path: '/sites', name: 'sites', component: () => import('./views/SitesView.vue') },
    { path: '/publish', name: 'publish', component: () => import('./views/PublishView.vue') },
        { path: '/comic', name: 'comic', component: () => import('./views/ComicView.vue') },
          { path: '/company', name: 'company', component: () => import('./views/CompanyView.vue') },
        { path: '/sync', redirect: '/company' },
        { path: '/:pathMatch(.*)*', redirect: '/chat' },
];

export default createRouter({
  history: createWebHistory(),
  routes,
});
