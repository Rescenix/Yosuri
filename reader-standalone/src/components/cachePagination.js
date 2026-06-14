// cachePagination.js（适配安卓原生存储）
function cacheKey(bookId, fontSize) {
  return `pagination_${bookId}_f${fontSize}`
}

export async function getCachedPages(bookId, fontSize) {
  try {
    if (typeof window.loadPageCache === 'function') {
      const json = window.loadPageCache(cacheKey(bookId, fontSize))
      if (json) {
        const data = JSON.parse(json)
        if (data && Array.isArray(data.pages)) {
          return data.pages
        }
      }
    }
    return null
  } catch (e) {
    console.warn('读取分页缓存失败', e)
    return null
  }
}

export async function setCachedPages(bookId, fontSize, pages) {
  try {
    if (typeof window.savePageCache === 'function') {
      const data = JSON.stringify({ pages, timestamp: Date.now() })
      window.savePageCache(cacheKey(bookId, fontSize), data)
    }
  } catch (e) {
    console.warn('写入分页缓存失败', e)
  }
}