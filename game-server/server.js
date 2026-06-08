import express from 'express'
import next from 'next'
import cors from 'cors'

import adminRoutes from './routes/admin.js'
import marketRoutes from './routes/market.js'   // 顶部
import 'dotenv/config'   // 如果没有安装 dotenv，需要先 npm install dotenv
const dev = process.env.NODE_ENV !== 'production'
const app = next({ dev })
const handle = app.getRequestHandler()

app.prepare().then(() => {
   const server = express()

  // 使用内置 JSON 解析中间件
  server.use(express.json({ limit: '10mb' }))
  server.use(cors())
server.use('/api/market', marketRoutes)         // 在 app.prepare 之后

  server.use('/api/admin', adminRoutes)
  server.get('/ping', (req, res) => res.json({ ok: true }))

  // 所有未匹配的请求交给 Next.js
  server.use((req, res) => handle(req, res))

  const port = process.env.PORT || 3001
  server.listen(port, () => {
    console.log(`> Ready on http://localhost:${port}`)
  })
})