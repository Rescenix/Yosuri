// src/composables/bookCache.js
import { openDB } from '../components/reading/cachePagination.js'

let db = null

async function getDB() {
  if (db) return db
  return new Promise((resolve, reject) => {
    const request = indexedDB.open('shanxi_book_cache', 2)
    request.onupgradeneeded = (e) => {
      const db = e.target.result
      if (!db.objectStoreNames.contains('books')) {
        db.createObjectStore('books', { keyPath: 'id' })
      }
      if (!db.objectStoreNames.contains('texts')) {
        db.createObjectStore('texts', { keyPath: 'id' })
      }
    }
    request.onsuccess = () => {
      db = request.result
      resolve(db)
    }
    request.onerror = () => reject(request.error)
  })
}

export async function cacheBookList(list) {
  const db = await getDB()
  const tx = db.transaction('books', 'readwrite')
  const store = tx.objectStore('books')
  for (const book of list) {
    store.put(book)
  }
  return new Promise((resolve, reject) => {
    tx.oncomplete = resolve
    tx.onerror = reject
  })
}

export async function getBookListFromCache() {
  const db = await getDB()
  const tx = db.transaction('books', 'readonly')
  const store = tx.objectStore('books')
  const list = await new Promise((resolve, reject) => {
    const req = store.getAll()
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error)
  })
  return list
}

export async function cacheBookText(bookId, text) {
  const db = await getDB()
  const tx = db.transaction('texts', 'readwrite')
  const store = tx.objectStore('texts')
  store.put({ id: bookId, text })
  return new Promise((resolve, reject) => {
    tx.oncomplete = resolve
    tx.onerror = reject
  })
}

export async function getBookTextFromCache(bookId) {
  const db = await getDB()
  const tx = db.transaction('texts', 'readonly')
  const store = tx.objectStore('texts')
  const result = await new Promise((resolve, reject) => {
    const req = store.get(bookId)
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error)
  })
  return result?.text || null
}