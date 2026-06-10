const CACHE_NAME = 'shanxi-reader-v5';
const PRECACHE_URLS = [
  '/reading-hut/',
  '/read/',
  '/favicon.ico',
];

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => {
      return Promise.allSettled(
        PRECACHE_URLS.map(url =>
          cache.add(url).catch(err => console.warn('预缓存失败:', url, err))
        )
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
  // 不拦截 API 请求
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
        // 离线降级：所有导航请求都返回书架页面（避免闪屏）
        if (event.request.mode === 'navigate') {
          // 无论原 URL 是什么，都返回书架页面
          return caches.match('/reading-hut/');
        }
        // 非导航请求返回简单错误提示（可选）
        return new Response('离线无法加载此资源', { status: 503 });
      });
    })
  );
});