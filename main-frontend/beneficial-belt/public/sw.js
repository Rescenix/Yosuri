const CACHE_NAME = 'shanxi-__BUILD_TIMESTAMP__';

// 预缓存你的实际入口页面 + 核心资源
const PRECACHE_URLS = [
  '/shanxi-hut/',               // 应用主页面
  '/shanxi-hut/index.html',     // 入口 HTML
  '/manifest.webmanifest',      // 如果有
  '/favicon.ico',
  // 可以添加更多关键文件，如 '/_astro/xxx.css'
];

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => {
      return cache.addAll(PRECACHE_URLS);
    })
  );
  self.skipWaiting();
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) => {
      return Promise.all(
        keys.filter((key) => key !== CACHE_NAME).map((key) => caches.delete(key))
      );
    })
  );
  self.clients.claim();
});

self.addEventListener('fetch', (event) => {
  // 不缓存 API 请求
  if (event.request.url.includes('/api/')) return;

  event.respondWith(
    caches.match(event.request).then((cached) => {
      const network = fetch(event.request).then((response) => {
        if (response.ok) {
          const clone = response.clone();
          caches.open(CACHE_NAME).then((cache) => cache.put(event.request, clone));
        }
        return response;
      }).catch(() => cached);
      return cached || network;
    })
  );
});