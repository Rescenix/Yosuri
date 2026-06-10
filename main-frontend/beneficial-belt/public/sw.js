const CACHE_NAME = 'shanxi-__BUILD_TIMESTAMP__';

// 预缓存首页和关键资源（确保 /read/ 和 /reading-hut/ 的 HTML 被缓存）
const PRECACHE_URLS = [
  '/read/',
  '/read/index.html',          // 如果存在
  '/reading-hut/',
  '/reading-hut/index.html',   // 如果存在
  '/manifest.webmanifest',
  // 可以继续添加其他静态资源
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

  // 处理导航请求（页面跳转）
  if (event.request.mode === 'navigate') {
    event.respondWith(
      fetch(event.request).catch(() => {
        // 网络失败，返回缓存的阅读首页（选择 /reading-hut/ 或 /read/，优先匹配当前路径）
        return caches.match(event.request).then(cached => {
          if (cached) return cached;
          // 如果当前路径没有缓存，回退到 /reading-hut/ 或 /read/
          return caches.match('/reading-hut/') || caches.match('/read/');
        });
      })
    );
    return;
  }

  // 其他资源：缓存优先，网络更新
  event.respondWith(
    caches.match(event.request).then((cached) => {
      const network = fetch(event.request).then((response) => {
        if (response.ok && event.request.method === 'GET') {
          const clone = response.clone();
          caches.open(CACHE_NAME).then((cache) => cache.put(event.request, clone));
        }
        return response;
      }).catch(() => cached);
      return cached || network;
    })
  );
});