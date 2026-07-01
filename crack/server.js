const http = require('http');
const { chromium } = require('playwright');
const TurndownService = require('turndown');

const turndown = new TurndownService({
    headingStyle: 'atx',
    bulletListMarker: '-',
    codeBlockStyle: 'fenced',
    emDelimiter: '*' // 用 * 表示斜体/粗体，避免下划线混淆
});

// 添加表格支持（原本 Turndown 不转表格，需要手动规则）
turndown.addRule('table', {
    filter: ['table'],
    replacement: function (content) {
        // 将 HTML 表格转换为 Markdown 表格
        return '\n\n' + content.replace(/<\/td>/g, ' | ').replace(/<\/th>/g, ' | ').replace(/<\/tr>/g, '\n').replace(/<[^>]+>/g, '') + '\n\n';
    }
});


let browser, page, input;
let lastReplyLength = 0;

async function initBrowser() {
    if (browser) return;
    browser = await chromium.launchPersistentContext('C:\\Pro2026\\re0\\crack\\chrome_temp', {
        headless: true,
        executablePath: 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe',
    });
    page = browser.pages()[0];
    await page.goto('https://chat.deepseek.com/a/chat/s/2345bc85-2e43-4823-a94e-0dc0e935736f', {
        waitUntil: 'networkidle', timeout: 60000
    });
    input = page.locator('[placeholder*="输入"], [placeholder*="问题"], textarea').first();
    console.log('[INIT] 浏览器就绪');
}

async function sendMessage(message) {
    // 记录发送前最后一条消息的长度（用于 /ready 判断）
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
    await input.fill('');                    // 清空
    await input.fill(message);               // ★ 一次性填入整个消息，不逐字打字
    await page.waitForTimeout(500);
    await page.keyboard.press('Enter');
    console.log('[SEND] 已发送');
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

async function readLatestReplyAsMarkdown() {
    try {
        const elements = page.locator('.assistant-message, .bot-message, [class*="assistant"]');
        const count = await elements.count();
        if (count > 0) {
            const html = await elements.nth(count - 1).innerHTML();
            //console.log(`[READ] 获取 HTML 长度: ${html.length}, 前 100 字符: ${html.substring(0, 100)}`);
            const markdown = turndown.turndown(html);
            //console.log(`[READ] 转换 Markdown 长度: ${markdown.length}, 前 100 字符: ${markdown.substring(0, 100)}`);
            return markdown.trim();
        }
    } catch (e) {
        console.log(`[READ] 出错: ${e.message}`);
    }
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
                console.log(`[SEND] 收到前端消息，长度: ${message.length}, 内容: ${message.substring(0, 200)}...`);
                await sendMessage(message);
                res.writeHead(200, { 'Content-Type': 'text/plain' });
                res.end('ok');
            } catch (e) {
                console.log(`[SEND] 错误: ${e.message}`);
                res.writeHead(500);
                res.end('Error');
            }
        });
        return;
    }
// ★ 流式推送 + DONE 信号
if (req.method === 'POST' && req.url === '/stream') {
    let body = '';
    req.on('data', chunk => body += chunk);
    req.on('end', async () => {
        try {
            const { message } = JSON.parse(body);
            await sendMessage(message);

            res.writeHead(200, {
                'Content-Type': 'text/event-stream',
                'Cache-Control': 'no-cache',
                'Connection': 'keep-alive',
                'Access-Control-Allow-Origin': '*'
            });

            // 等待 DS 开始回复
            let ready = false;
            for (let i = 0; i < 30; i++) {
                await new Promise(r => setTimeout(r, 500));
                if (await checkReady()) { ready = true; break; }
            }
            if (!ready) {
                res.write(`data: 杉汐没有回应，请稍后再试\n\n`);
                res.end();
                return;
            }

        // ---------- 按词/按块推送，速度适中 ----------
const queue = [];
let consumerTimer = null;
let finished = false;

// 消费者：每 50ms 推送一个词（或片段）
const consumer = () => {
    if (queue.length === 0) {
        if (finished) {
            clearInterval(consumerTimer);
            res.write(`data: [DONE]\n\n`);
            res.end();
        }
        return;
    }
    const chunk = queue.shift();
    res.write(`data: ${chunk}\n\n`);
};
consumerTimer = setInterval(consumer, 50);

// 生产者：将增量拆成短词推入队列（按空格或每 10 字符切分）
let lastText = '';
let stableCount = 0;
for (let i = 0; i < 150; i++) {
    await new Promise(r => setTimeout(r, 200));
    try {
        const currentText = await readLatestReplyAsMarkdown();
        if (currentText && currentText.length > lastText.length) {
            const newPart = currentText.slice(lastText.length);
            // 将 newPart 拆分为更自然的片段：按空格或每 10 个字符切分
            const tokens = newPart.match(/[\s\S]{1,10}/g) || [newPart];
            tokens.forEach(token => queue.push(token));
            lastText = currentText;
            stableCount = 0;
        } else if (currentText && currentText.length === lastText.length && currentText.length > 10) {
            stableCount++;
            if (stableCount >= 15) break;
        }
    } catch (e) {}
}
finished = true;
        } catch (e) {
            res.write(`data: 流式推送出错: ${e.message}\n\n`);
            res.end();
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

if (req.method === 'GET' && req.url === '/read') {
    let reply = await readLatestReplyAsMarkdown();
    // 不再做任何转义还原，直接返回原始 Markdown
    console.log('[READ] 最终返回：', reply.substring(0, 120));
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