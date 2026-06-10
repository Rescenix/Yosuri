// pagination.worker.js
import { exactPaginate } from './ExactPaginator.js'

self.onmessage = async (e) => {
  const { text, fontSize, pageWidth, pageHeight } = e.data
  try {
    // 一次性排版，返回所有页面
    const pages = await exactPaginate(
      text,
      fontSize,
      pageWidth,
      pageHeight,
      (pct) => {
        // 发送进度消息（主线程可接收更新进度条）
        self.postMessage({ type: 'progress', percent: pct })
      }
    )
    // 排版完成，发送最终结果
    self.postMessage({ type: 'result', pages })
  } catch (err) {
    self.postMessage({ type: 'error', message: err.message })
  }
}