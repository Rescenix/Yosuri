const { chromium } = require('playwright');

(async () => {
  const browser = await chromium.launchPersistentContext(
    'C:\\Pro2026\\re0\\crack\\chrome_temp',
    {
      headless: true,  // 这一行改成 true，浏览器在后台运行，不弹窗口
      executablePath: 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe',
    }
  );
  const page = browser.pages()[0];

  const targetUrl =
    'https://chat.deepseek.com/a/chat/s/65cf206b-0a62-4182-b2e5-504aa0980040';
  await page.goto(targetUrl, { waitUntil: 'networkidle', timeout: 60000 });
  console.log('已进入会话，准备发送消息...');

  const input = page.locator('[placeholder*="输入"], [placeholder*="问题"], textarea').first();
  await input.click();
  await page.waitForTimeout(300);
  await input.type('你好，请用一句话介绍你自己', { delay: 50 });
  await page.waitForTimeout(500);
  await page.keyboard.press('Enter');
  console.log('消息已发送，开始轮询DOM增量：\n');

  let lastText = '';

  for (let i = 0; i < 30; i++) {
    await page.waitForTimeout(1000);
    try {
      const currentText = await page.locator('.assistant-message, .bot-message, [class*="assistant"]').last().textContent();
      if (currentText && currentText !== lastText) {
        const newPart = currentText.slice(lastText.length);
        process.stdout.write(newPart);
        lastText = currentText;
      }
    } catch (e) {}
  }

  console.log('\n--- 回复结束 ---');
  await browser.close();
})();