const CACHE_NAME = 'shanxi-reader-v5';
const PRECACHE_URLS = [
  '/reading-hut/',
  '/favicon.ico',
];

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => {
      return Promise.allSettled(
        PRECACHE_URLS.map(url => cache.add(url).catch(err => console.warn('预缓存失败:', url, err)))
      );
    })
  );
  self.skipWaiting();
});

self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) => {
      return Promise.all(
        keys.filter(key => key !== CACHE_NAME).map(key => caches.delete(key))
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
      if (cached) return cached;

      return fetch(event.request).then((response) => {
        if (response.ok && event.request.method === 'GET') {
          const clone = response.clone();
          caches.open(CACHE_NAME).then((cache) => cache.put(event.request, clone));
        }
        return response;
      }).catch(() => {
        // 网络失败时，不进行导航回退，让浏览器显示默认错误页（但前端已处理，不会出现）
        // 这里返回空响应，避免显示“网络错误”文本
        return new Response('', { status: 408, statusText: 'Offline' });
      });
    })
  );
});