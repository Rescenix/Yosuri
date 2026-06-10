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

// 缓存键只包含 bookId 和 fontSize，不再包含宽高
function cacheKey(bookId, fontSize) {
  return `${bookId}_f${fontSize}`
}

export async function getCachedPages(bookId, fontSize) {
  try {
    const database = await openDB()
    const tx = database.transaction(STORE_NAME, 'readonly')
    const store = tx.objectStore(STORE_NAME)
    const key = cacheKey(bookId, fontSize)
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

export async function setCachedPages(bookId, fontSize, pages) {
  try {
    const database = await openDB()
    const tx = database.transaction(STORE_NAME, 'readwrite')
    const store = tx.objectStore(STORE_NAME)
    const key = cacheKey(bookId, fontSize)
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