import { Router } from 'express'
import pool from '../lib/db.js'

const router = Router()

// 临时鉴权
router.use((req, res, next) => {
  req.userId = req.headers['x-user-id'] || 'test-user'
  next()
})

// 上架物品
router.post('/list', async (req, res) => {
  const { itemId, itemData, price, quantity } = req.body
  const sellerId = req.userId

  if (!itemId || !price || !quantity) {
    return res.status(400).json({ error: '缺少必要参数' })
  }

  const client = await pool.connect()
  try {
    const listingResult = await client.query(
      `INSERT INTO market_listings (seller_id, item_id, item_data, price, quantity)
       VALUES ($1, $2, $3, $4, $5) RETURNING *`,
      [sellerId, itemId, itemData || {}, price, quantity]
    )
    res.json({ success: true, listing: listingResult.rows[0] })
  } catch (err) {
    console.error('上架失败:', err)
    res.status(500).json({ error: '上架失败' })
  } finally {
    client.release()
  }
})

// 购买物品
router.post('/buy', async (req, res) => {
  const { listingId, quantity = 1 } = req.body
  const buyerId = req.userId

  const client = await pool.connect()
  try {
    await client.query('BEGIN')

    const listingRes = await client.query(
      `SELECT * FROM market_listings WHERE id = $1 FOR UPDATE`,
      [listingId]
    )
    if (listingRes.rows.length === 0) {
      await client.query('ROLLBACK')
      return res.status(404).json({ error: '物品不存在' })
    }
    const listing = listingRes.rows[0]
    if (listing.status !== 'active' || listing.quantity < quantity) {
      await client.query('ROLLBACK')
      return res.status(400).json({ error: '库存不足或已下架' })
    }
    if (listing.seller_id === buyerId) {
      await client.query('ROLLBACK')
      return res.status(400).json({ error: '不能购买自己的物品' })
    }

    const totalCost = listing.price * quantity

    const userRes = await client.query(`SELECT gold FROM users WHERE id = $1 FOR UPDATE`, [buyerId])
    if (userRes.rows.length === 0 || userRes.rows[0].gold < totalCost) {
      await client.query('ROLLBACK')
      return res.status(400).json({ error: '金币不足' })
    }

    await client.query(`UPDATE users SET gold = gold - $1 WHERE id = $2`, [totalCost, buyerId])
    await client.query(`UPDATE users SET gold = gold + $1 WHERE id = $2`, [totalCost, listing.seller_id])

    const newQuantity = listing.quantity - quantity
    const newStatus = newQuantity === 0 ? 'sold' : 'active'
    await client.query(
      `UPDATE market_listings SET quantity = $1, status = $2, updated_at = NOW() WHERE id = $3`,
      [newQuantity, newStatus, listingId]
    )

    await client.query(
      `INSERT INTO player_inventory (player_id, item_id, item_data, quantity)
       VALUES ($1, $2, $3, $4)`,
      [buyerId, listing.item_id, listing.item_data, quantity]
    )

    await client.query(
      `INSERT INTO market_transactions (listing_id, buyer_id, seller_id, price, quantity)
       VALUES ($1, $2, $3, $4, $5)`,
      [listingId, buyerId, listing.seller_id, listing.price, quantity]
    )

    await client.query('COMMIT')
    res.json({ success: true, paid: totalCost, receivedItem: listing.item_id })
  } catch (err) {
    await client.query('ROLLBACK')
    console.error('购买失败:', err)
    res.status(500).json({ error: '购买失败' })
  } finally {
    client.release()
  }
})

// 搜索物品（带分页、排序、关键词、价格、卖家过滤）
router.get('/search', async (req, res) => {
  const { keyword, minPrice, maxPrice, page = 1, limit = 20, sort = 'price_asc', sellerId } = req.query
  const offset = (page - 1) * limit

  // 构建 WHERE 条件
  const conditions = [`status = 'active'`]
  const params = []

  if (keyword) {
    params.push(`%${keyword}%`)
    conditions.push(`(item_id ILIKE $${params.length} OR item_data->>'name' ILIKE $${params.length})`)
  }
  if (minPrice) {
    params.push(minPrice)
    conditions.push(`price >= $${params.length}`)
  }
  if (maxPrice) {
    params.push(maxPrice)
    conditions.push(`price <= $${params.length}`)
  }
  if (sellerId) {
    params.push(sellerId)
    conditions.push(`seller_id = $${params.length}`)
  }

  const where = conditions.length > 0 ? `WHERE ${conditions.join(' AND ')}` : ''

  // 排序
  let order = ''
  switch (sort) {
    case 'price_asc': order = 'ORDER BY price ASC'; break
    case 'price_desc': order = 'ORDER BY price DESC'; break
    case 'newest': order = 'ORDER BY created_at DESC'; break
    default: order = 'ORDER BY price ASC'
  }

  try {
    // 先查总数
    const countSql = `SELECT COUNT(*) FROM market_listings ${where}`
    const countResult = await pool.query(countSql, params)
    const total = parseInt(countResult.rows[0].count)

    // 查数据
    const dataSql = `SELECT * FROM market_listings ${where} ${order} LIMIT $${params.length + 1} OFFSET $${params.length + 2}`
    params.push(limit, offset)
    const dataResult = await pool.query(dataSql, params)

    res.json({
      listings: dataResult.rows,
      page: Number(page),
      limit: Number(limit),
      total
    })
  } catch (err) {
    console.error('搜索失败:', err)
    res.status(500).json({ error: '查询失败' })
  }
})

// 下架物品（取消出售）
router.post('/cancel', async (req, res) => {
  const { listingId } = req.body
  const userId = req.userId

  const client = await pool.connect()
  try {
    await client.query('BEGIN')

    const listingRes = await client.query(
      `SELECT * FROM market_listings WHERE id = $1 FOR UPDATE`,
      [listingId]
    )
    if (listingRes.rows.length === 0 || listingRes.rows[0].seller_id !== userId || listingRes.rows[0].status !== 'active') {
      await client.query('ROLLBACK')
      return res.status(403).json({ error: '无权操作或物品已售出' })
    }

    const listing = listingRes.rows[0]

    // 标记为取消
    await client.query(
      `UPDATE market_listings SET status = 'cancelled', updated_at = NOW() WHERE id = $1`,
      [listingId]
    )

    // 归还物品到库存（不合并，直接插入新行）
    await client.query(
      `INSERT INTO player_inventory (player_id, item_id, item_data, quantity)
       VALUES ($1, $2, $3, $4)`,
      [userId, listing.item_id, listing.item_data, listing.quantity]
    )

    await client.query('COMMIT')
    res.json({ success: true })
  } catch (err) {
    await client.query('ROLLBACK')
    console.error('下架失败:', err)
    res.status(500).json({ error: '下架失败' })
  } finally {
    client.release()
  }
})

export default router