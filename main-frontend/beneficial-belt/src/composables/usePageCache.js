// usePageCache.js
const DB_NAME = 'ShanxiReader';
const DB_VERSION = 1;

function openDB() {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(DB_NAME, DB_VERSION);
    request.onupgradeneeded = (event) => {
      const db = event.target.result;
      if (!db.objectStoreNames.contains('pages')) {
        db.createObjectStore('pages', { keyPath: 'id' }); // id = `${bookId}_${page}`
      }
    };
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error);
  });
}

/**
 * 将页面 HTML 存入 IndexedDB
 * @param {string} bookId
 * @param {number} page - 页码 (从 1 开始)
 * @param {string} html
 */
export async function savePage(bookId, page, html) {
  const db = await openDB();
  const tx = db.transaction('pages', 'readwrite');
  tx.objectStore('pages').put({ id: `${bookId}_${page}`, html, bookId, page });
  return tx.complete;
}

/**
 * 从 IndexedDB 读取页面
 * @param {string} bookId
 * @param {number} page
 * @returns {Promise<string|null>}
 */
export async function getPage(bookId, page) {
  const db = await openDB();
  return new Promise((resolve, reject) => {
    const tx = db.transaction('pages', 'readonly');
    const req = tx.objectStore('pages').get(`${bookId}_${page}`);
    req.onsuccess = () => resolve(req.result?.html || null);
    req.onerror = () => reject(req.error);
  });
}

/**
 * 可选：删除整本书的缓存
 */
export async function removeBookCache(bookId) {
  const db = await openDB();
  const tx = db.transaction('pages', 'readwrite');
  const store = tx.objectStore('pages');
  const range = IDBKeyRange.bound(`${bookId}_`, `${bookId}_\uffff`);
  store.delete(range);
  return tx.complete;
}