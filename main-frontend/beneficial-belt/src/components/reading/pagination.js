let worker = null;

function createWorker() {
  if (worker) {
    worker.terminate();
    worker = null;
  }
  try {
    worker = new Worker(new URL('./pagination.worker.js', import.meta.url), { type: 'module' });
    return worker;
  } catch (e) {
    console.error('Worker 创建失败', e);
    return null;
  }
}

/**
 * 流式分页函数
 * @param {string} text
 * @param {number} pageWidth
 * @param {number} pageHeight
 * @param {number} fontSize
 * @param {function} onPages - 每次收到一批页面时回调 (newPages, totalNow)
 * @param {function} onProgress - 进度回调 (percent)
 * @returns {Promise<number>} 解析时返回总页数
 */
export function paginate(text, pageWidth, pageHeight, fontSize, onPages, onProgress) {
  return new Promise((resolve, reject) => {
    const w = createWorker();
    if (!w) {
      reject(new Error('分页 Worker 不可用'));
      return;
    }

    w.onmessage = (e) => {
      const { type, pages, total, percent, totalPages, message } = e.data;
      switch (type) {
        case 'pages':
          if (onPages) onPages(pages, total);
          break;
        case 'progress':
          if (onProgress) onProgress(percent);
          break;
        case 'complete':
          resolve(totalPages);
          w.terminate();
          worker = null;
          break;
        case 'error':
          reject(new Error(message));
          w.terminate();
          worker = null;
          break;
      }
    };

    w.onerror = (err) => {
      reject(err);
      w.terminate();
      worker = null;
    };

    w.postMessage({ text, pageWidth, pageHeight, fontSize });
  });
}

export function terminateWorker() {
  if (worker) {
    worker.terminate();
    worker = null;
  }
}