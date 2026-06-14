import { createApp } from 'vue'
import App from './App.vue'
import './assets/global.css'

const app = createApp(App)
app.mount('#app')
// 不再需要手动移除 initial-loader