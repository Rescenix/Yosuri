import { createApp } from 'vue';
import App from './App.vue';
import router from './router.js';
import './styles/global.css';
import { initTheme } from './components/shanxi/composables/useTheme.js';

initTheme(); // 启动即根据持久化/系统偏好把 data-theme 打到 <html>

createApp(App).use(router).mount('#app');
