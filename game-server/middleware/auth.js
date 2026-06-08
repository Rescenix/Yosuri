export default function auth(req, res, next) {
  const ip = req.ip
  if (ip === '127.0.0.1' || ip === '::1') return next()
  const authHeader = req.headers.authorization
  if (authHeader && authHeader.startsWith('Bearer ')) {
    const token = authHeader.slice(7)
    if (token === 'dev-secret-token') return next()
  }
  return res.status(403).json({ error: 'Forbidden' })
}