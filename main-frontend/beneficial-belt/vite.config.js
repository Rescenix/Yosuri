import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
import { fileURLToPath } from 'url';
import { dirname, resolve } from 'path';
import { readFileSync } from 'fs';
import { homedir } from 'os';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

// 默认跟随桌面端正式 API；需要联调其他实例时再显式覆盖目标端口。
// 端口可能不是 8080：后端启动时 8080 被 Steam/Docker 占用会自动顺延，
// 并把实际端口写到 ~/rescene_data/api_port，这里优先读它。
function resolveApiTarget() {
  if (process.env.AURORA_API_TARGET) return process.env.AURORA_API_TARGET;
  try {
    const p = readFileSync(resolve(homedir(), 'rescene_data', 'api_port'), 'utf8').trim();
    if (/^\d+$/.test(p)) return `http://localhost:${p}`;
  } catch { /* 文件不存在 = 后端还没起过，用默认 */ }
  return 'http://localhost:8080';
}
const apiTarget = resolveApiTarget();

export default defineConfig({
  plugins: [vue()],
  define: {
    'process.env': '{}',
    global: 'globalThis',
  },
  build: { charset: 'utf8' },
  esbuild: { charset: 'utf8' },
  server: {
    host: '0.0.0.0',
    port: 4322,
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
      '/images': apiTarget,
      '/api': {
        target: apiTarget,
        ws: true,
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
  optimizeDeps: { include: [] },
});
