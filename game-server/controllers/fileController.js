import path from 'path'
import fs from 'fs'
import { fileURLToPath } from 'url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const DATA_ROOT = path.join(__dirname, '../../game-client/public/data')

function validatePath(filename) {
  // 实现路径穿越防护
  const safe = path.normalize(filename).replace(/^(\.\.[\/\\])+/, '')
  const full = path.join(DATA_ROOT, safe)
  if (!full.startsWith(DATA_ROOT)) throw new Error('Path traversal')
  return full
}

export function saveJson(req, res) {
  // TODO
  res.json({ ok: true })
}

export function loadJson(req, res) {
  // TODO
  res.json({ ok: true })
}

export function listFiles(req, res) {
  // TODO
  res.json([])
}