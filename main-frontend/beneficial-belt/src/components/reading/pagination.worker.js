// pagination.worker.js
// 在 Worker 中运行分页，不阻塞主线程

import { exactPaginate } from './ExactPaginator.js'

self.onmessage = async (e) => {
  const { text, fontSize, pageWidth, pageHeight } = e.data
  try {
    const pages = await exactPaginate(
      text,
      fontSize,
      pageWidth,
      pageHeight,
      (pct) => {
        self.postMessage({ type: 'progress', percent: pct })
      }
    )
    self.postMessage({ type: 'result', pages })
  } catch (err) {
    self.postMessage({ type: 'error', message: err.message })
  }
}