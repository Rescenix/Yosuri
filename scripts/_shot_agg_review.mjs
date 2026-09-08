import { chromium } from 'playwright-core';

const URL = process.env.URL || 'http://localhost:4323';
const SHOT = process.env.SHOT || 'C:/Pro2026/re0/scripts/_agg_review.png';

const browser = await chromium.launch({ headless: true });
const page = await browser.newPage({ viewport: { width: 1280, height: 920 } });
const errors = [];
page.on('console', m => { if (m.type() === 'error') errors.push(m.text()); });
page.on('pageerror', e => errors.push('PAGEERROR: ' + e.message));

await page.goto(URL, { waitUntil: 'networkidle' });

await page.waitForSelector('.agg-api-shortcut', { timeout: 15000 });
await page.click('.agg-api-shortcut', { force: true });
await page.waitForTimeout(600);

await page.getByText('聚合 API', { exact: true }).first().click({ force: true });
await page.waitForTimeout(400);

await page.getByText('用户自定义', { exact: true }).first().click({ force: true });
await page.waitForSelector('.agg-cfg-name', { timeout: 15000 });
await page.waitForTimeout(800);

const info = await page.evaluate(() => {
  const btns = [...document.querySelectorAll('.agg-cfg-info')];
  return { n: btns.length, names: btns.map(b => b.closest('.agg-cfg-item-wrap')?.querySelector('.agg-cfg-name')?.textContent || '?') };
});
console.log('INFOBTN=' + JSON.stringify(info));

if (info.n > 0) {
  const firstId = await page.evaluate(() => document.querySelector('.agg-cfg-info')?.getAttribute('class'));
  await page.locator('.agg-cfg-info').first().click({ force: true });
  await page.waitForTimeout(500);
  const cardVisible = await page.locator('.agg-review-card').first().isVisible().catch(() => false);
  console.log('CARD_VISIBLE=' + cardVisible);
  const cardText = await page.locator('.agg-review-text').first().textContent().catch(() => '');
  console.log('FIRST_REVIEW=' + cardText);
}

await page.screenshot({ path: SHOT, fullPage: false });
console.log('SHOT=' + SHOT);
console.log('ERRORS=' + JSON.stringify(errors.slice(0, 10)));
await browser.close();
