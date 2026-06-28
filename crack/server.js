const http = require('http');
const { chromium } = require('playwright');
const TurndownService = require('turndown');  // 引入转换库

const turndown = new TurndownService();       // 创建实例

let browser, page, input;
let lastReplyLength = 0; // 记录发送前最后一条消息的长度

async function initBrowser() {
    if (browser) return;
    browser = await chromium.launchPersistentContext('C:\\Pro2026\\re0\\crack\\chrome_temp', {
        headless: true,
        executablePath: 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe',
    });
    page = browser.pages()[0];
    await page.goto('https://chat.deepseek.com/a/chat/s/65cf206b-0a62-4182-b2e5-504aa0980040', {
        waitUntil: 'networkidle', timeout: 60000
    });
    input = page.locator('[placeholder*="输入"], [placeholder*="问题"], textarea').first();
    console.log('[INIT] 浏览器就绪');
}

async function sendMessage(message) {
    // 记录发送前最后一条消息的长度（用于后续 /ready 判断）
    try {
        const texts = await page.locator('.assistant-message, .bot-message, [class*="assistant"]').allTextContents();
        if (texts.length > 0) {
            lastReplyLength = texts[texts.length - 1].length;
        } else {
            lastReplyLength = 0;
        }
    } catch (e) {
        lastReplyLength = 0;
    }
    console.log(`[SEND] 发送前最后一条消息长度: ${lastReplyLength}`);

    // 输入并发送
    await input.click();
    await page.waitForTimeout(300);
    await input.fill('');
    await input.type(message, { delay: 50 });
    await page.waitForTimeout(500);
    await page.keyboard.press('Enter');
    console.log('[SEND] 已发送:', message);
}

async function checkReady() {
    try {
        const texts = await page.locator('.assistant-message, .bot-message, [class*="assistant"]').allTextContents();
        if (texts.length > 0) {
            const currentLength = texts[texts.length - 1].length;
            return currentLength !== lastReplyLength;
        }
    } catch (e) {}
    return false;
}

// ★ 核心改造：读取 HTML 并转换为 Markdown
async function readLatestReplyAsMarkdown() {
    try {
        const elements = page.locator('.assistant-message, .bot-message, [class*="assistant"]');
        const count = await elements.count();
        if (count > 0) {
            const html = await elements.nth(count - 1).innerHTML();
            const markdown = turndown.turndown(html);
            return markdown.trim();
        }
    } catch (e) {}
    return '';
}

const server = http.createServer(async (req, res) => {
    res.setHeader('Access-Control-Allow-Origin', '*');
    res.setHeader('Access-Control-Allow-Methods', 'POST, GET, OPTIONS');
    res.setHeader('Access-Control-Allow-Headers', 'Content-Type');

    if (req.method === 'OPTIONS') {
        res.writeHead(200);
        res.end();
        return;
    }

    if (req.method === 'POST' && req.url === '/send') {
        let body = '';
        req.on('data', chunk => body += chunk);
        req.on('end', async () => {
            try {
                const { message } = JSON.parse(body);
                await sendMessage(message);
                res.writeHead(200, { 'Content-Type': 'text/plain' });
                res.end('ok');
            } catch (e) {
                res.writeHead(500);
                res.end('Error');
            }
        });
        return;
    }

    if (req.method === 'GET' && req.url === '/ready') {
        const ready = await checkReady();
        res.writeHead(200, { 'Content-Type': 'text/plain' });
        res.end(ready ? 'yes' : 'no');
        return;
    }

    // ★ 将 /read 改为返回 Markdown
    if (req.method === 'GET' && req.url === '/read') {
        const reply = await readLatestReplyAsMarkdown();
        res.writeHead(200, { 'Content-Type': 'text/plain; charset=utf-8' });
        res.end(reply);
        return;
    }

    res.writeHead(404);
    res.end();
});

server.listen(3000, () => {
    console.log('[START] DS 代理: http://localhost:3000');
    initBrowser();
});