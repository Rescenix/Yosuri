import { defineConfig } from 'astro/config';
import vue from '@astrojs/vue';
import { VitePWA } from 'vite-plugin-pwa';
import { fileURLToPath } from 'url';
import { dirname, resolve } from 'path';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

export default defineConfig({
  integrations: [vue()],
  vite: {
    define: {
      'process.env': '{}',
      global: 'globalThis',
    },
    build: {
      charset: 'utf8',
    },
    esbuild: {
      charset: 'utf8',
    },
    ssr: {
      noExternal: ['tsparticles-engine', 'tsparticles-slim'],
    },
    server: {
      host: '0.0.0.0',
      port: 4321,
      headers: {
        'Cache-Control': 'no-cache, no-store, must-revalidate',
        'Pragma': 'no-cache',
        'Expires': '0',
      },
      proxy: {
        '/aether': {
          target: 'http://localhost:80',
          changeOrigin: true,
          rewrite: (path) => path.replace(/^\/aether/, ''),
        },
        '/aether/api': {
          target: 'http://localhost:8082',
          changeOrigin: true,
          rewrite: (path) => path.replace(/^\/aether\/api/, '/api'),
        },
        '/images': 'http://localhost:8080',
        '/api': {
          target: 'http://localhost:8080',
          changeOrigin: true,
          secure: false,
          timeout: 120000,
          proxyTimeout: 120000,
          maxBodyLength: 50 * 1024 * 1024,
        },
      },
      fs: {
        allow: [
          resolve(__dirname, '.'),
          resolve(__dirname, 'node_modules'),
          resolve(__dirname, 'public'),
        ],
      },
    },
    plugins: [
      VitePWA({
        registerType: 'autoUpdate',
        workbox: {
          globPatterns: ['**/*.{js,css,html,ico,png,svg,woff2}'],
          // 关键：所有导航请求（即页面跳转）回退到 /index.html，保证 SPA 离线可用
          navigateFallback: '/index.html',
          runtimeCaching: [
            {
              urlPattern: /\/api\/.*/,
              handler: 'NetworkOnly',            // 不缓存 API，保持数据新鲜
            },
            {
              urlPattern: /\/book\/.*/,
              handler: 'NetworkFirst',
              options: {
                cacheName: 'book-pages',
                expiration: { maxEntries: 100, maxAgeSeconds: 86400 },
              },
            },
            {
              // HTML 文件：网络优先，失败时回退缓存
              urlPattern: ({ request }) => request.mode === 'navigate',
              handler: 'NetworkFirst',
              options: {
                cacheName: 'pages-cache',
                expiration: { maxEntries: 50, maxAgeSeconds: 86400 },
              },
            },
          ],
        },
        manifest: {
          name: '杉汐阅读',
          short_name: '杉汐',
          theme_color: '#fafafa',
          background_color: '#ffffff',
          display: 'standalone',
          icons: [
            { src: '/favicon.ico', sizes: '48x48', type: 'image/x-icon' },
          ],
        },
      }),
    ],
    optimizeDeps: {
      include: [],
    },
  },
});