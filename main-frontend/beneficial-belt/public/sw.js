const CACHE_NAME = 'shanxi-reader-v7';
const PRECACHE_URLS = [
  '/reading-hut/',
  '/reading-hut/index.html',
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
        // 网络失败时，仅对书架导航请求回退，不干扰阅读页
        if (event.request.mode === 'navigate' && !event.request.url.includes('/read')) {
          return caches.match('/reading-hut/');
        }
        // 其他请求（包括阅读页）直接报错，不跳转
        return new Response('网络错误', { status: 408 });
      });
    })
  );
});