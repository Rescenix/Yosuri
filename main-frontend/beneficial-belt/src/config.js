// src/config.js
// 这个文件用于集中管理所有API请求的地址

const isLocal = import.meta.env.DEV;

export const API_BASE_URL = isLocal 
  ? 'http://localhost:8080' // 本地开发时的地址
  : ''; // 部署到服务器后，使用相对路径，由Nginx代理

export const CHAT_API = `${API_BASE_URL}/api/chat`;
export const POSTS_API = `${API_BASE_URL}/api/posts`;
