// public/sw.js
const CACHE_NAME = 'shanxi-reader-v4';   // 更新版本，强制刷新
const PRECACHE_URLS = [
  '/reading-hut/',
  '/read/',                // 预缓存阅读页外壳（如果存在）
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

      // 网络请求
      return fetch(event.request).then((response) => {
        if (response.ok && event.request.method === 'GET') {
          const clone = response.clone();
          caches.open(CACHE_NAME).then((cache) => cache.put(event.request, clone));
        }
        return response;
      }).catch(() => {
        // 网络失败，对于导航请求，回退到对应的外壳
        if (event.request.mode === 'navigate') {
          // 如果访问的是 /read 系列，则回退到 /read/ 的缓存
          if (event.request.url.includes('/read')) {
            return caches.match('/read/') || caches.match('/reading-hut/');
          }
          // 否则回退到 /reading-hut/
          return caches.match('/reading-hut/');
        }
      });
    })
  );
});