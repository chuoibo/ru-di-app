/**
 * Look at Nếp's note on the real app, in the states that decide whether it is
 * any good: resting over a screen, resting over a conversation, and making room
 * while a tray is open. Light and dark.
 *
 * It asserts the one behaviour a screenshot alone cannot prove: while the tray
 * is open, the note's visible strip does not reach any text the tray draws.
 * The pictures are for looking at; this number is for failing.
 */
import fs from 'node:fs';
import path from 'node:path';
import puppeteer from 'puppeteer-core';

const fixture = JSON.parse(fs.readFileSync(process.env.SESSIONS, 'utf8'));
const output = process.env.OUTPUT;
if (!output?.startsWith('/tmp/')) throw Error('Evidence stays outside the checkout.');
fs.mkdirSync(output, { recursive: true, mode: 0o700 });
const web = process.env.WEB ?? 'http://127.0.0.1:8178';
// repo-guard: allow=long-number reason=public-chrome-version
const chrome = process.env.CHROME_PATH ?? `${process.env.HOME}/.cache/puppeteer/chrome/linux-148.0.7778.97/chrome-linux64/chrome`;
const ket = { steps: [] };
const save = () => fs.writeFileSync(path.join(output, 'ket-qua.json'), JSON.stringify(ket, null, 2));
const sleep = ms => new Promise(r => setTimeout(r, ms));
const nguoi = fixture.users[Number(process.env.USER_INDEX ?? 1)];

/** Where the note is, and where every piece of text is, in page pixels. */
const doHinh = () => {
  const to = document.querySelector('[data-testid="nep-dia"], [data-testid="nep-mep"]');
  const hop = to ? to.getBoundingClientRect() : null;
  const chu = [];
  const w = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
  while (w.nextNode()) {
    const n = w.currentNode;
    if (!n.textContent.trim() || (to && to.contains(n.parentElement))) continue;
    const r = document.createRange();
    r.selectNodeContents(n);
    for (const b of r.getClientRects()) if (b.width > 0 && b.height > 0) chu.push({ t: n.textContent.trim().slice(0, 40), x: b.left, y: b.top, r: b.right, b: b.bottom });
  }
  return { to: hop && { x: hop.left, y: hop.top, r: hop.right, b: hop.bottom, w: hop.width }, chu, rong: window.innerWidth };
};

async function chay(scheme) {
  const browser = await puppeteer.launch({ executablePath: chrome, headless: true, args: ['--no-sandbox'] });
  try {
    const page = await (await browser.createBrowserContext()).newPage();
    await page.setViewport({ width: 390, height: 844 });
    await page.emulateMediaFeatures([{ name: 'prefers-color-scheme', value: scheme }, { name: 'prefers-reduced-motion', value: 'reduce' }]);
    const cdp = await page.createCDPSession();
    await cdp.send('Emulation.setFocusEmulationEnabled', { enabled: true });

    await page.goto(web + '/login', { waitUntil: 'domcontentloaded' });
    await page.waitForSelector('[aria-label="Ô số điện thoại"]', { timeout: 30000 });
    await page.type('[aria-label="Ô số điện thoại"]', nguoi.phone);
    for (let lan = 0; lan < 3; lan++) {
      const cho = page.waitForResponse(r => r.request().method() === 'POST' && r.url().endsWith('/auth/otp/request'));
      await page.click('[data-testid="login-gui-ma"]');
      const res = await cho;
      if (res.status() === 429) { await sleep(61000); continue; }
      if (![200, 202].includes(res.status())) throw Error(`OTP request ${res.status()}`);
      break;
    }
    await page.waitForSelector('[data-testid="otp-input"]', { timeout: 30000 });
    await page.type('[data-testid="otp-input"]', '000000');
    await page.waitForSelector('[role="tab"][aria-label="Tin nhắn"]', { timeout: 40000 });
    await sleep(1500);
    await page.screenshot({ path: path.join(output, `${scheme}-1-nghi-man-dau.png`) });

    await page.click('[role="tab"][aria-label="Tin nhắn"]');
    await page.waitForSelector('[aria-label="Mở nhóm Phòng kiểm thử đồng thời"]', { timeout: 30000 });
    await page.click('[aria-label="Mở nhóm Phòng kiểm thử đồng thời"]');
    await page.waitForSelector('[aria-label="Thêm vào cuộc trò chuyện"]', { timeout: 30000 });
    await sleep(1500);
    await page.screenshot({ path: path.join(output, `${scheme}-2-nghi-tren-hoi-thoai.png`) });
    const truocKhay = await page.evaluate(doHinh);

    await page.click('[aria-label="Thêm vào cuộc trò chuyện"]');
    await page.click('[aria-label="Tờ hẹn"]');
    await page.waitForSelector('[aria-label="Lời nhờ lập kế hoạch"]', { timeout: 20000 });
    await sleep(1200);
    const moRong = await page.$('[data-testid="chat-boi-canh-mo"]');
    if (moRong) { await moRong.click(); await sleep(500); }
    await page.screenshot({ path: path.join(output, `${scheme}-3-nhuong-cho-khi-mo-khay.png`) });
    const khiMoKhay = await page.evaluate(doHinh);

    // The note's visible strip is whatever is on screen: from its left edge to
    // the viewport's right. Any text rectangle that reaches into it is covered.
    const lo = khiMoKhay.to ? Math.max(khiMoKhay.to.x, 0) : khiMoKhay.rong;
    const bi = khiMoKhay.to ? khiMoKhay.chu.filter(c => c.r > lo && c.b > khiMoKhay.to.y && c.y < khiMoKhay.to.b) : [];
    ket.steps.push({
      scheme,
      name: 'khong_che_chu_khi_mo_khay',
      pass: !!khiMoKhay.to && bi.length === 0,
      rongLo: khiMoKhay.to ? Math.round(khiMoKhay.rong - lo) : null,
      rongLoTruocKhay: truocKhay.to ? Math.round(truocKhay.rong - Math.max(truocKhay.to.x, 0)) : null,
      chuBiChe: bi.map(c => c.t),
    });
    save();
  } finally {
    await browser.close();
  }
}

for (const scheme of (process.env.SCHEMES ?? 'light,dark').split(',')) await chay(scheme);
ket.pass = ket.steps.every(s => s.pass);
save();
console.log(JSON.stringify(ket, null, 2));
