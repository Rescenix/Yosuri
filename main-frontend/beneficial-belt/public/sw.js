const CACHE_NAME = 'shanxi-reader-v6';  // 更新版本号
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
  if (event.request.url.includes('/api/')) return;

  event.respondWith(
    new Promise((resolve) => {
      // 设置总超时 3 秒，确保不白屏
      const timeoutId = setTimeout(() => {
        // 超时后，如果是导航请求，返回入口缓存
        if (event.request.mode === 'navigate') {
          resolve(caches.match('/reading-hut/'));
        } else {
          resolve(new Response('', { status: 408, statusText: 'Timeout' }));
        }
      }, 3000);

      caches.match(event.request).then((cached) => {
        if (cached) {
          clearTimeout(timeoutId);
          resolve(cached);
          return;
        }

        fetch(event.request).then((response) => {
          clearTimeout(timeoutId);
          if (response.ok && event.request.method === 'GET') {
            const clone = response.clone();
            caches.open(CACHE_NAME).then((cache) => cache.put(event.request, clone));
          }
          resolve(response);
        }).catch(() => {
          clearTimeout(timeoutId);
          if (event.request.mode === 'navigate') {
            resolve(caches.match('/reading-hut/'));
          } else {
            resolve(new Response('Network error', { status: 408 }));
          }
        });
      });
    })
  );
});