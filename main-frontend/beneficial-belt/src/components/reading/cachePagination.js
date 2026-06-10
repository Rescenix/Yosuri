// cachePagination.js
const DB_NAME = 'reading-hut-pages'
const STORE_NAME = 'pages'
let db = null

function openDB() {
  return new Promise((resolve, reject) => {
    if (db) return resolve(db)
    const request = indexedDB.open(DB_NAME, 1)
    request.onupgradeneeded = (e) => {
      const database = e.target.result
      if (!database.objectStoreNames.contains(STORE_NAME)) {
        database.createObjectStore(STORE_NAME, { keyPath: 'id' })
      }
    }
    request.onsuccess = (e) => {
      db = e.target.result
      resolve(db)
    }
    request.onerror = (e) => reject(e.target.error)
  })
}

/**
 * 构建缓存键（与安卓原生缓存一致）
 */
function cacheKey(bookId, fontSize, pageWidth, pageHeight) {
  return `${bookId}_f${fontSize}_w${pageWidth}h${pageHeight}`
}

/**
 * 获取缓存的分页 HTML 数组（优先安卓原生，回退 IndexedDB）
 */
export async function getCachedPages(bookId, fontSize, pageWidth, pageHeight) {
  const key = cacheKey(bookId, fontSize, pageWidth, pageHeight)

  // 1. 优先从安卓原生缓存读取
  if (typeof window.loadPageCache === 'function') {
    try {
      const json = window.loadPageCache(key)
      if (json) {
        const pages = JSON.parse(json)
        if (Array.isArray(pages) && pages.length > 0) {
          return pages
        }
      }
    } catch (e) {
      console.warn('原生缓存读取失败', e)
    }
  }

  // 2. 回退到 IndexedDB
  try {
    const database = await openDB()
    const tx = database.transaction(STORE_NAME, 'readonly')
    const store = tx.objectStore(STORE_NAME)
    return new Promise((resolve, reject) => {
      const request = store.get(key)
      request.onsuccess = () => {
        const result = request.result
        resolve(result?.pages ?? null)
      }
      request.onerror = () => reject(request.error)
    })
  } catch (e) {
    console.warn('IndexedDB 缓存读取失败', e)
    return null
  }
}

/**
 * 存储分页结果（同时写入安卓原生和 IndexedDB）
 */
export async function setCachedPages(bookId, fontSize, pageWidth, pageHeight, pages) {
  const key = cacheKey(bookId, fontSize, pageWidth, pageHeight)

  // 1. 写入安卓原生缓存（立刻执行）
  if (typeof window.savePageCache === 'function') {
    try {
      window.savePageCache(key, JSON.stringify(pages))
    } catch (e) {
      console.warn('原生缓存写入失败', e)
    }
  }

  // 2. 同时写入 IndexedDB（可选，作为双重保险）
  try {
    const database = await openDB()
    const tx = database.transaction(STORE_NAME, 'readwrite')
    const store = tx.objectStore(STORE_NAME)
    store.put({ id: key, pages, timestamp: Date.now() })
    return new Promise((resolve, reject) => {
      tx.oncomplete = resolve
      tx.onerror = reject
    })
  } catch (e) {
    console.warn('IndexedDB 缓存写入失败', e)
  }
}

export { openDB, STORE_NAME }