// pagination.worker.js
import { exactPaginate } from './ExactPaginator.js'

self.onmessage = async (e) => {
  const { text, fontSize, pageWidth, pageHeight } = e.data
  try {
    const paragraphs = text.split('\n')
    const totalParagraphs = paragraphs.length
    let allPages = []
    const CHUNK_SIZE = 200 // 每次处理 200 个段落，快速产出一批页面

    for (let i = 0; i < totalParagraphs; i += CHUNK_SIZE) {
      const end = Math.min(i + CHUNK_SIZE, totalParagraphs)
      const chunkText = paragraphs.slice(i, end).join('\n')

      // 分批调用精确排版，不等待全部完成
      const chunkPages = await exactPaginate(chunkText, fontSize, pageWidth, pageHeight, () => {})
      allPages = allPages.concat(chunkPages)

      // 立即将这批发给主线程
      self.postMessage({
        type: 'pages',
        pages: chunkPages,               // 新增页面
        total: allPages.length           // 当前总页数
      })

      // 更新整体进度
      const percent = Math.floor((end / totalParagraphs) * 100)
      self.postMessage({ type: 'progress', percent })
    }

    // 全部完成
    self.postMessage({ type: 'complete', totalPages: allPages.length })
  } catch (err) {
    self.postMessage({ type: 'error', message: err.message })
  }
}