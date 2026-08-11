// src/config.js
// 这个文件用于集中管理所有API请求的地址

const isLocal = import.meta.env.DEV;

export const API_BASE_URL = isLocal 
  ? 'http://localhost:8081' // 本地开发时的地址
  : '';

export const CHAT_API = `${API_BASE_URL}/api/chat`;
