const CACHE_NAME = 'shanxi-reader-v3';   // 更新版本号，强制刷新旧缓存
const PRECACHE_URLS = [
  '/reading-hut/',
  '/favicon.ico',
  // 不要添加不存在的文件，避免安装失败
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
  // 不处理 API 请求
  if (event.request.url.includes('/api/')) return;

  event.respondWith(
    caches.match(event.request).then((cached) => {
      if (cached) return cached;

      // 对于导航请求（页面跳转），如果网络失败，返回缓存的入口页面
      const fetchPromise = fetch(event.request).then((response) => {
        if (response.ok && event.request.method === 'GET') {
          const clone = response.clone();
          caches.open(CACHE_NAME).then((cache) => cache.put(event.request, clone));
        }
        return response;
      }).catch(() => {
        // 网络错误时，如果是导航请求，回退到缓存的 /reading-hut/
        if (event.request.mode === 'navigate') {
          return caches.match('/reading-hut/');
        }
        // 其他资源失败则抛出错误
        throw new Error('Network error');
      });

      // 如果缓存未命中，优先网络，但网络失败时导航请求有兜底
      return fetchPromise;
    })
  );
});