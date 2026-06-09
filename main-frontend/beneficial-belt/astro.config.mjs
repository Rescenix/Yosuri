import { defineConfig } from 'astro/config';
import vue from '@astrojs/vue';
import { fileURLToPath } from 'url';
import { dirname, resolve } from 'path';
import fs from 'fs';
import path from 'path';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

// 自定义 Vite 插件：构建结束后将 sw.js 中的 __BUILD_TIMESTAMP__ 替换为当前时间戳
function replaceSwTimestamp() {
  let swDestPath;
  return {
    name: 'replace-sw-timestamp',
    configResolved(config) {
      // 获取构建输出目录（默认为 dist）
      swDestPath = path.resolve(config.build.outDir, 'sw.js');
    },
    closeBundle() {
      if (fs.existsSync(swDestPath)) {
        let content = fs.readFileSync(swDestPath, 'utf-8');
        const timestamp = Date.now().toString();
        content = content.replace('__BUILD_TIMESTAMP__', timestamp);
        fs.writeFileSync(swDestPath, content);
        console.log(`[SW] 缓存版本已更新为: ${timestamp}`);
      } else {
        console.warn('[SW] 未找到 sw.js，请确认 public/sw.js 存在');
      }
    },
  };
}

export default defineConfig({
  integrations: [vue()],
  vite: {
    define: {
      'process.env': '{}',
      global: 'globalThis',
    },
    build: { charset: 'utf8' },
    esbuild: { charset: 'utf8' },
    ssr: { noExternal: ['tsparticles-engine', 'tsparticles-slim'] },
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
      replaceSwTimestamp(),   // 自动替换时间戳，保证每次构建缓存版本更新
      // 注意：已删除 VitePWA 插件
    ],
    optimizeDeps: { include: [] },
  },
});