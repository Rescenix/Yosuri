import { createApp } from 'vue';
import App from './App.vue';
import router from './router.js';
import './styles/global.css';
import { initTheme } from './components/shanxi/composables/useTheme.js';
import { installDesktopTransport } from './desktopTransport.js';

async function bootstrap() {
  // Wails 的 AssetServer 不承载 SSE/WebSocket；桌面壳先注入随机 loopback API 地址，
  // 普通 Vite/Web 部署没有 Wails 绑定时保持原有相对路径行为。
  await installDesktopTransport();
  initTheme(); // 启动即根据持久化/系统偏好把 data-theme 打到 <html>
  createApp(App).use(router).mount('#app');
}

bootstrap();
