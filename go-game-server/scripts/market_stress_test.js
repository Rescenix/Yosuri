import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '30s', target: 20 },
    { duration: '1m', target: 50 },
    { duration: '30s', target: 0 },
  ],
};

const BASE = 'http://localhost:8080/api/market';
const headers = { 'Content-Type': 'application/json', 'x-user-id': 'test-user' };

export default function () {
  const uniqueId = `test_item_${__VU}_${__ITER}`;

  // 1. 上架独立物品
  const listRes = http.post(`${BASE}/list`, JSON.stringify({
    itemId: uniqueId,
    itemData: { name: '测试之剑', type: 'weapon' },
    price: 1,
    quantity: 10,
  }), { headers });
  check(listRes, { 'list OK': (r) => r.json('success') === true });

  // 2. 搜索自己的物品
  const searchRes = http.get(`${BASE}/search?keyword=${uniqueId}`);
  check(searchRes, { 'search OK': (r) => r.status === 200 });

  // 3. 购买自己的物品（确保存在）
  const listings = searchRes.json('listings');
  if (listings && listings.length > 0) {
    const listingId = listings[0].id;
    const buyRes = http.post(`${BASE}/buy`, JSON.stringify({ listingId, quantity: 1 }), { headers });
    check(buyRes, { 'buy OK': (r) => r.json('success') === true });
  }

  sleep(1);
}