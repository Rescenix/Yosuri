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
 * 获取缓存的分页 HTML 数组
 * @param {string} bookId
 * @param {number} fontSize
 * @param {number} pageWidth
 * @param {number} pageHeight
 * @returns {Promise<string[]|null>}
 */
export async function getCachedPages(bookId, fontSize, pageWidth, pageHeight) {
  try {
    const database = await openDB()
    const tx = database.transaction(STORE_NAME, 'readonly')
    const store = tx.objectStore(STORE_NAME)
    const key = `${bookId}_f${fontSize}_w${pageWidth}h${pageHeight}`
    return new Promise((resolve, reject) => {
      const request = store.get(key)
      request.onsuccess = () => resolve(request.result?.pages ?? null)
      request.onerror = () => reject(request.error)
    })
  } catch (e) {
    console.warn('读取缓存失败', e)
    return null
  }
}

/**
 * 存储分页结果
 * @param {string} bookId
 * @param {number} fontSize
 * @param {number} pageWidth
 * @param {number} pageHeight
 * @param {string[]} pages
 */
export async function setCachedPages(bookId, fontSize, pageWidth, pageHeight, pages) {
  try {
    const database = await openDB()
    const tx = database.transaction(STORE_NAME, 'readwrite')
    const store = tx.objectStore(STORE_NAME)
    const key = `${bookId}_f${fontSize}_w${pageWidth}h${pageHeight}`
    store.put({ id: key, pages, timestamp: Date.now() })
    return new Promise((resolve, reject) => {
      tx.oncomplete = resolve
      tx.onerror = reject
    })
  } catch (e) {
    console.warn('写入缓存失败', e)
  }
}

export { openDB, STORE_NAME }