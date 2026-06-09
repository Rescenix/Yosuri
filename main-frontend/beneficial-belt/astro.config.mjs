import { defineConfig } from 'astro/config';
import vue from '@astrojs/vue';
import { fileURLToPath } from 'url';
import { dirname, resolve } from 'path';

// 在 ES 模块中手动构建 __dirname
const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

export default defineConfig({
  integrations: [vue()],
  vite: {
     define: {
      'process.env': '{}',       // 模拟空环境变量
      global: 'globalThis',      // 部分库可能用到
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
      host: '0.0.0.0',           // ★ 允许局域网访问
      port: 4321,
      proxy: {
        '/aether': {
        target: 'http://localhost:80',   // Aether 前端容器映射的宿主机端口
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/aether/, '')
      },
      '/aether/api': {
        target: 'http://localhost:8082', // Aether 后端容器映射的宿主机端口
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/aether\/api/, '/api')
      },
        '/images': 'http://localhost:8080' , // 新增
        '/api': {
          target: 'http://localhost:8080',
          changeOrigin: true,
        secure: false,
        timeout: 120000,
         proxyTimeout: 120000,
         // 设置请求体大小限制为 50MB
        maxBodyLength: 50 * 1024 * 1024,
        }
      },
      fs: {
        allow: [
          resolve(__dirname, '.'),
          resolve(__dirname, 'node_modules'),
          resolve(__dirname, 'public'),
        ],
      },
    },

    optimizeDeps: {
      include: [],
    },
  },
});