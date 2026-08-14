// src/config.js
// 这个文件用于集中管理所有API请求的地址

// 开发环境同样走相对地址，由 Vite 代理到当前桌面后端。
// 旧的 8081 绝对地址会绕过代理，读到另一个旧进程的 directive.json，
// 造成模型刚选中就被轮询改回“自动切换”。
export const API_BASE_URL = '';

export const CHAT_API = `${API_BASE_URL}/api/chat`;
