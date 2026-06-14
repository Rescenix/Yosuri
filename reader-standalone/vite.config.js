import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'
import { viteSingleFile } from 'vite-plugin-singlefile'

export default defineConfig({
  plugins: [
    vue(),
    viteSingleFile(),  // ★ 将所有 JS/CSS 内联到 HTML 中
  ],
  base: './',
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
    },
  },
  build: {
    outDir: 'dist',
    // 单文件插件会自动处理，无需额外 rollup 配置
  },
})