import puppeteer from 'puppeteer-core';
import fs from 'node:fs';

fs.mkdirSync('/opt/cursor/artifacts', { recursive: true });

const browser = await puppeteer.launch({
  executablePath: '/usr/bin/google-chrome-stable',
  headless: true,
  args: ['--no-sandbox', '--disable-setuid-sandbox', '--window-size=1280,900'],
  defaultViewport: { width: 1280, height: 900 },
});
const page = await browser.newPage();
page.on('console', (msg) => console.log('CONSOLE', msg.type(), msg.text()));
page.on('pageerror', (err) => console.log('PAGEERROR', err.message));

await page.goto('http://127.0.0.1:5173/', { waitUntil: 'networkidle0', timeout: 30000 });
await page.waitForSelector('[data-act="login"]', { timeout: 10000 });
await page.screenshot({ path: '/opt/cursor/artifacts/h5_login.png', fullPage: true });
await page.click('#nick', { clickCount: 3 });
await page.type('#nick', '甲');
await page.click('[data-act="login"]');
await page.waitForSelector('[data-act="create"]', { timeout: 15000 });
await new Promise((r) => setTimeout(r, 400));
await page.screenshot({ path: '/opt/cursor/artifacts/h5_lobby.png', fullPage: true });

await page.click('[data-act="create"]');
await page.waitForSelector('[data-act="ready"]', { timeout: 15000 });
await new Promise((r) => setTimeout(r, 400));
await page.screenshot({ path: '/opt/cursor/artifacts/h5_room.png', fullPage: true });
const roomText = await page.evaluate(() => document.body.innerText);
console.log('ROOM_SNIP\n', roomText.slice(0, 500));
const code = (roomText.match(/(\d{6})/) || [])[1];
console.log('ROOM_CODE', code);

await page.click('[data-act="ready"]');
await page.waitForFunction(() => document.body.innerText.includes('已准备'), { timeout: 10000 });
await page.screenshot({ path: '/opt/cursor/artifacts/h5_room_ready.png', fullPage: true });
console.log('READY_OK', true);

await page.click('[data-nav="records"]');
await page.waitForSelector('[data-act="loadRecords"]', { timeout: 5000 });
await page.click('[data-act="loadRecords"]');
await new Promise((r) => setTimeout(r, 800));
await page.screenshot({ path: '/opt/cursor/artifacts/h5_records.png', fullPage: true });
console.log('RECORDS_OK', (await page.evaluate(() => document.body.innerText)).includes('战绩'));

await browser.close();
