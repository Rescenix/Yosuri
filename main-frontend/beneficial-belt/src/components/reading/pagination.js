let worker = null;
const WORKER_VERSION = '6';

function createWorker() {
  try {
    worker = new Worker(`/pagination-worker.js?v=${WORKER_VERSION}`);
    return worker;
  } catch (e) {
    console.error('Worker 创建失败', e);
    return null;
  }
}

export function paginate(text, pageWidth, pageHeight, fontSize) {
  return new Promise((resolve, reject) => {
    if (!worker) {
      const w = createWorker();
      if (!w) {
        reject(new Error('分页 Worker 不可用'));
        return;
      }
    }
    worker.onmessage = function (e) {
      console.log('Worker 原始返回:', e.data);
      // 兼容各种可能的返回格式
      let pages = [];
      if (Array.isArray(e.data)) {
        pages = e.data;
      } else if (e.data && Array.isArray(e.data.pages)) {
        pages = e.data.pages;
      } else if (e.data && e.data.type === 'result' && Array.isArray(e.data.pages)) {
        pages = e.data.pages;
      } else if (typeof e.data === 'string') {
        // 如果意外返回字符串，尝试当作单页处理
        pages = [e.data];
      }
      console.log('解析后页数:', pages.length);
      resolve(pages);
    };
    worker.onerror = function (err) {
      reject(err);
    };
    worker.postMessage({ text, pageWidth, pageHeight, fontSize });
  });
}

export function terminateWorker() {
  if (worker) {
    worker.terminate();
    worker = null;
  }
}