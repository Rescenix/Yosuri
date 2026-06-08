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

// 搜索物品
router.get('/search', async (req, res) => {
  const { keyword, minPrice, maxPrice, page = 1, limit = 20, sort = 'price_asc' } = req.query
  const offset = (page - 1) * limit

  let query = `SELECT * FROM market_listings WHERE status = 'active'`
  const params = []

  if (keyword) {
    query += ` AND (item_id ILIKE $${params.length + 1} OR item_data->>'name' ILIKE $${params.length + 1})`
    params.push(`%${keyword}%`)
  }
  if (minPrice) {
    query += ` AND price >= $${params.length + 1}`
    params.push(minPrice)
  }
  if (maxPrice) {
    query += ` AND price <= $${params.length + 1}`
    params.push(maxPrice)
  }

  switch (sort) {
    case 'price_asc': query += ' ORDER BY price ASC'; break
    case 'price_desc': query += ' ORDER BY price DESC'; break
    case 'newest': query += ' ORDER BY created_at DESC'; break
    default: query += ' ORDER BY price ASC'
  }

  query += ` LIMIT $${params.length + 1} OFFSET $${params.length + 2}`
  params.push(limit, offset)

  try {
    const result = await pool.query(query, params)
    res.json({ listings: result.rows, page: Number(page), limit: Number(limit) })
  } catch (err) {
    console.error('搜索失败:', err)
    res.status(500).json({ error: '查询失败' })
  }
})

// 下架物品
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

    await client.query(
      `UPDATE market_listings SET status = 'cancelled', updated_at = NOW() WHERE id = $1`,
      [listingId]
    )

    await client.query(
      `INSERT INTO player_inventory (player_id, item_id, item_data, quantity)
       VALUES ($1, $2, $3, $4)
       ON CONFLICT (player_id, item_id) DO UPDATE SET quantity = player_inventory.quantity + $4`,
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