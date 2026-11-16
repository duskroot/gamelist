import puppeteer from 'puppeteer-core';

const browser = await puppeteer.launch({
  executablePath: '/usr/bin/google-chrome-stable',
  headless: true,
  args: ['--no-sandbox', '--disable-setuid-sandbox'],
  defaultViewport: { width: 1280, height: 900 },
});
const page = await browser.newPage();
page.on('console', (msg) => console.log('CONSOLE', msg.type(), msg.text()));
page.on('pageerror', (err) => console.log('PAGEERROR', err.message, err.stack));
page.on('response', async (res) => {
  if (res.status() >= 400) console.log('BAD', res.status(), res.url());
});
page.on('requestfailed', (req) => console.log('REQFAIL', req.url(), req.failure()?.errorText));

await page.goto('http://127.0.0.1:5173/', { waitUntil: 'networkidle0', timeout: 30000 });
await page.waitForSelector('#nick', { timeout: 10000 });
console.log('SCREEN HTML snippet', await page.evaluate(() => document.querySelector('#app')?.innerHTML.slice(0, 400)));

// intercept login request
page.on('response', async (res) => {
  if (res.url().includes('/table/')) {
    console.log('API', res.status(), res.url(), await res.text().catch(()=>''));
  }
});

await page.evaluate(() => {
  const btn = document.querySelector('[data-act="login"]');
  console.log('btn', !!btn, btn?.outerHTML);
});

await page.click('[data-act="login"]');
await new Promise((r) => setTimeout(r, 3000));
console.log('AFTER CLICK', await page.evaluate(() => document.body.innerText.slice(0, 800)));
console.log('TOAST', await page.evaluate(() => document.querySelector('.toast')?.textContent || 'none'));
console.log('localStorage', await page.evaluate(() => JSON.stringify(localStorage)));
await browser.close();
