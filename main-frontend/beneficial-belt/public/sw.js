const CACHE_NAME = 'shanxi-reader-v2';  // 更新版本强制刷新缓存
const PRECACHE_URLS = [
  '/reading-hut/',          // 你的新入口
  '/reading-hut/index.html',
  '/favicon.ico',
  // 可根据需要添加更多静态资源，但运行时也会自动缓存
];

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => {
      return Promise.allSettled(
        PRECACHE_URLS.map(url => cache.add(url).catch(err => {
          console.warn('预缓存失败: ' + url, err);
        }))
      );
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
  // 不缓存 API 请求（保持数据新鲜）
  if (event.request.url.includes('/api/')) return;

  event.respondWith(
    caches.match(event.request).then((cachedResponse) => {
      const networkFetch = fetch(event.request).then((networkResponse) => {
        if (networkResponse.ok && event.request.method === 'GET') {
          const responseClone = networkResponse.clone();
          caches.open(CACHE_NAME).then((cache) => cache.put(event.request, responseClone));
        }
        return networkResponse;
      }).catch(() => cachedResponse);
      return cachedResponse || networkFetch;
    })
  );
});