// usePaginationLoader.js
// 独立的分页加载管理器，封装异步分片、缓存、进度、取消等功能

import { exactPaginate } from './ExactPaginator.js'
import { getCachedPages, setCachedPages } from './cachePagination.js'

/**
 * 创建一个分页加载器
 * @param {Ref<string>} textRef - 全文的 ref
 * @param {Ref<number>} fontSizeRef - 字体大小 ref
 * @param {Ref<number>} pageWidthRef - 页面宽度 ref
 * @param {Ref<number>} pageHeightRef - 页面高度 ref
 * @param {Function} onProgress - 进度回调 (percent: number, message: string)
 * @param {Function} onComplete - 完成回调 (pages: string[])
 * @param {Function} onError - 错误回调 (error: Error)
 * @returns {Object} 控制对象 { start, cancel, isLoading }
 */
export function usePaginationLoader(textRef, fontSizeRef, pageWidthRef, pageHeightRef, onProgress, onComplete, onError) {
  let cancelFlag = false
  let isLoading = false

  async function start() {
    if (isLoading) return
    isLoading = true
    cancelFlag = false

    const text = textRef.value
    const fontSize = fontSizeRef.value
    const pageWidth = pageWidthRef.value
    const pageHeight = pageHeightRef.value
    const bookId = 'current-book' // 可以外部传入，这里简化为固定 key

    try {
      // 1. 尝试从缓存加载
      const cached = await getCachedPages(bookId, fontSize, pageWidth, pageHeight)
      if (cached && !cancelFlag) {
        onProgress(100, '加载完成（缓存）')
        onComplete(cached)
        isLoading = false
        return
      }

      // 2. 执行精确分页（内部已异步分片）
      const pages = await exactPaginate(
        text,
        fontSize,
        pageWidth,
        pageHeight,
        (pct) => {
          if (cancelFlag) return
          // 映射进度到 0-90%，留 10% 给缓存写入
          const scaled = Math.floor(pct * 0.9)
          onProgress(scaled, `排版中 ${scaled}%`)
        }
      )

      if (cancelFlag) return

      // 3. 写入缓存（后台进行，不阻塞回调）
      setCachedPages(bookId, fontSize, pageWidth, pageHeight, pages).catch(() => {})

      // 4. 组装封面与底页（由外部处理，这里只返回正文页）
      onProgress(100, '加载完成')
      onComplete(pages)
    } catch (err) {
      console.error('分页加载失败:', err)
      onError(err)
    } finally {
      isLoading = false
    }
  }

  function cancel() {
    cancelFlag = true
    isLoading = false
  }

  return {
    start,
    cancel,
    isLoading: () => isLoading
  }
}