/**
 * Look at Nếp's slip on the real app, and measure the one thing a picture
 * cannot prove: at rest it touches no text, at any scroll offset.
 *
 * States, light and dark: resting over the first tab and over a conversation
 * (scrolled through, text rects collected at every stop), making room while a
 * tray is open, and pulled out by the person. Built with
 * `EXPO_PUBLIC_QA_NEP_VIEC=bao|hoi`, the same run also shows the second slip
 * or the line written on the slip (`MODE=bao|hoi`); nothing in the app sends
 * the dock work yet, so those states are reachable only that way.
 *
 * The pictures are for looking at; `pass` is for failing. Pulled out and the
 * peeked line are the person's choice and a four-second announcement, so they
 * are captured but not held to the no-overlap rule.
 */
import fs from 'node:fs';
import path from 'node:path';
import puppeteer from 'puppeteer-core';

const fixture = JSON.parse(fs.readFileSync(process.env.SESSIONS, 'utf8'));
const output = process.env.OUTPUT;
if (!output?.startsWith('/tmp/')) throw Error('Evidence stays outside the checkout.');
fs.mkdirSync(output, { recursive: true, mode: 0o700 });
const web = process.env.WEB ?? 'http://127.0.0.1:8178';
const mode = process.env.MODE ?? 'thuong';
// repo-guard: allow=long-number reason=public-chrome-version
const chrome = process.env.CHROME_PATH ?? `${process.env.HOME}/.cache/puppeteer/chrome/linux-148.0.7778.97/chrome-linux64/chrome`;
const ket = { mode, steps: [] };
const save = () => fs.writeFileSync(path.join(output, `ket-qua-${mode}.json`), JSON.stringify(ket, null, 2));
const sleep = ms => new Promise(r => setTimeout(r, ms));
const nguoi = fixture.users[Number(process.env.USER_INDEX ?? 1)];

/**
 * Every visible text rectangle that the dock's VISIBLE part reaches into, at
 * every scroll stop of the largest scroller on the page. The dock's visible
 * part is its slip (and the second slip) clipped to the viewport.
 */
const doChe = async (page) => page.evaluate(async () => {
  const wait = ms => new Promise(r => setTimeout(r, ms));
  const dock = [...document.querySelectorAll('[data-testid="nep-mep"], [data-testid="nep-dia"], [data-testid="nep-to-sau"]')];
  const cuon = [...document.querySelectorAll('*')]
    .filter(e => e.scrollHeight > e.clientHeight + 40 && getComputedStyle(e).overflowY !== 'visible' && e.getBoundingClientRect().height > 200)
    .sort((a, b) => b.clientHeight - a.clientHeight)[0];
  const buoc = cuon ? Math.min(12, Math.ceil(cuon.scrollHeight / (cuon.clientHeight / 2))) : 1;
  const bi = new Map();
  let soChu = 0;
  let hop = [];
  for (let i = 0; i < buoc; i++) {
    if (cuon) cuon.scrollTop = (i * cuon.clientHeight) / 2;
    await wait(200);
    hop = dock.map(d => d.getBoundingClientRect()).filter(b => b.width > 0)
      .map(b => ({ x: Math.max(b.left, 0), y: b.top, r: Math.min(b.right, innerWidth), b: b.bottom }));
    const w = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
    while (w.nextNode()) {
      const n = w.currentNode;
      const el = n.parentElement;
      if (!n.textContent.trim() || dock.some(d => d.contains(el))) continue;
      if (!el.checkVisibility({ opacityProperty: true, visibilityProperty: true })) continue;
      // A text box keeps its full width when an ancestor clips it («200.00…»
      // ends at the ellipsis, its box runs on off-screen), so measure what is
      // actually painted: the box cut by every clipping ancestor.
      let khung = { x: 0, y: 0, r: innerWidth, b: innerHeight };
      for (let a = el; a && a !== document.body; a = a.parentElement) {
        const st = getComputedStyle(a);
        if (st.overflowX !== 'visible' || st.overflowY !== 'visible') {
          const k = a.getBoundingClientRect();
          khung = { x: Math.max(khung.x, k.left), y: Math.max(khung.y, k.top), r: Math.min(khung.r, k.right), b: Math.min(khung.b, k.bottom) };
        }
      }
      const rg = document.createRange();
      rg.selectNodeContents(n);
      for (const tho of rg.getClientRects()) {
        const c = { left: Math.max(tho.left, khung.x), top: Math.max(tho.top, khung.y), right: Math.min(tho.right, khung.r), bottom: Math.min(tho.bottom, khung.b) };
        if (c.right - c.left <= 0 || c.bottom - c.top <= 0) continue;
        // Text clipped away by an ancestor (a horizontal carousel) is not on screen.
        const giua = document.elementFromPoint(Math.min(c.left + 1, innerWidth - 1), (c.top + c.bottom) / 2);
        if (!giua || !(el.contains(giua) || giua.contains(el) || dock.some(d => d.contains(giua)))) continue;
        soChu++;
        for (const h of hop) {
          if (c.right > h.x && c.left < h.r && c.bottom > h.y && c.top < h.b) {
            bi.set(n.textContent.trim().slice(0, 40), Math.round(c.right - h.x));
          }
        }
      }
    }
  }
  if (cuon) cuon.scrollTop = 0;
  return {
    buoc,
    soChu,
    hopDock: hop.map(h => ({ rong: Math.round(h.r - h.x), cao: Math.round(h.b - h.y) })),
    chuBiChe: [...bi].map(([t, lan]) => `${t} (lấn ${lan}px)`),
  };
});

async function chay(scheme) {
  const browser = await puppeteer.launch({ executablePath: chrome, headless: true, args: ['--no-sandbox'] });
  try {
    const page = await (await browser.createBrowserContext()).newPage();
    await page.setViewport({ width: 390, height: 844, deviceScaleFactor: 2 });
    await page.emulateMediaFeatures([{ name: 'prefers-color-scheme', value: scheme }, { name: 'prefers-reduced-motion', value: 'reduce' }]);
    const cdp = await page.createCDPSession();
    await cdp.send('Emulation.setFocusEmulationEnabled', { enabled: true });
    const chup = (ten) => page.screenshot({ path: path.join(output, `${mode}-${scheme}-${ten}.png`) });

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
    if (mode === 'hoi') {
      // The line written on the slip: an announcement, captured, not measured.
      // It lasts four seconds from the first screen that declares itself, so
      // wait for it rather than for a fixed delay.
      const dong = await page.waitForSelector('[data-testid="nep-bong-bong"]', { timeout: 8000 }).catch(() => null);
      if (dong) await chup('5-dong-he');
      ket.steps.push({ scheme, name: 'dong_he_hien', pass: !!dong });
      save();
      return;
    }

    await sleep(1500);
    await chup('1-tab-dau');

    const tabDau = await doChe(page);
    ket.steps.push({ scheme, name: 'nghi_tab_dau_khong_che_chu', pass: tabDau.hopDock.length > 0 && tabDau.chuBiChe.length === 0, ...tabDau });
    save();

    await page.click('[role="tab"][aria-label="Tin nhắn"]');
    await page.waitForSelector('[aria-label="Mở nhóm Phòng kiểm thử đồng thời"]', { timeout: 30000 });
    await page.click('[aria-label="Mở nhóm Phòng kiểm thử đồng thời"]');
    await page.waitForSelector('[aria-label="Thêm vào cuộc trò chuyện"]', { timeout: 30000 });
    await sleep(1500);
    const hoiThoai = await doChe(page);
    await chup('2-hoi-thoai');
    ket.steps.push({ scheme, name: 'nghi_hoi_thoai_khong_che_chu', pass: hoiThoai.hopDock.length > 0 && hoiThoai.chuBiChe.length === 0, ...hoiThoai });
    save();

    await page.click('[aria-label="Thêm vào cuộc trò chuyện"]');
    await page.click('[aria-label="Tờ hẹn"]');
    await page.waitForSelector('[aria-label="Lời nhờ lập kế hoạch"]', { timeout: 20000 });
    await sleep(1200);
    const moRong = await page.$('[data-testid="chat-boi-canh-mo"]');
    if (moRong) { await moRong.click(); await sleep(500); }
    await chup('3-nhuong-cho-khay');
    const khay = await doChe(page);
    ket.steps.push({ scheme, name: 'nhuong_cho_khay_khong_che_chu', pass: khay.hopDock.length > 0 && khay.chuBiChe.length === 0, ...khay });
    save();

    if (mode === 'thuong') {
      // Close the tray, then pull Nếp out the way a person would.
      await page.keyboard.press('Escape');
      await sleep(800);
      const mep = await page.$('[data-testid="nep-mep"]');
      if (mep) {
        const b = await mep.boundingBox();
        await page.mouse.click(b.x + Math.min(b.width, 8) / 2, b.y + b.height / 2);
        await sleep(900);
      }
      await chup('4-keo-ra');
      // The canary: pulled out, the slip DOES stand on the conversation, so a
      // measure that still reports nothing covered here is blind, not clean.
      const keoRa = await doChe(page);
      ket.steps.push({
        scheme,
        name: 'keo_ra_hien_nep_va_phep_do_thay_duoc_cho_che',
        pass: !!(await page.$('[data-testid="nep-dia"]')) && keoRa.chuBiChe.length > 0,
        ...keoRa,
      });
      save();
    }
  } finally {
    await browser.close();
  }
}

for (const scheme of (process.env.SCHEMES ?? 'light,dark').split(',')) await chay(scheme);
ket.pass = ket.steps.every(s => s.pass);
save();
console.log(JSON.stringify(ket, null, 2));
