/**
 * Look at Nếp's slip on the real app, and measure the one thing a picture
 * cannot prove: at rest it touches no text, at any scroll offset.
 *
 * States, light and dark: resting over the first tab and over a conversation
 * (scrolled through, text rects collected at every stop), making room while a
 * tray is open, and pulled out by the person. Built with
 * `EXPO_PUBLIC_QA_NEP_VIEC=bao|hoi`, the same run also shows the second slip,
 * tucked with a money screen (`MODE=bao`) or pulled out (`MODE=hoi`); nothing
 * in the app sends the dock work yet, so those states are reachable only that
 * way.
 *
 * The pictures are for looking at; `pass` is for failing. Pulled out is the
 * person's choice, so it is captured and used as the canary, not held to the
 * no-overlap rule; nothing may ever widen past it.
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

/**
 * The seed makes the room but says nothing in it, and an empty room has no
 * bubble for the slip to cover: "0 covered" there is a measure of nothing,
 * and the canary cannot go red. So fill it once per stack, with the person
 * being measured speaking too (their bubbles sit on the right, against the
 * margin the slip lives in), and with lines long enough to wrap to full width.
 */
async function nhoiTin() {
  const dau = `${process.env.SESSIONS}.da-nhoi`;
  if (fs.existsSync(dau)) return;
  const ai = [nguoi, fixture.users[0], fixture.users[2], fixture.users[3]];
  const cau = [
    'Tối nay ai rảnh không, đi ăn gì đó rồi cà phê nhé',
    'Mình rảnh từ 7h, nhưng đừng xa quá, mình đi xe buýt',
    'Ok',
    'Hay là lẩu nấm ở gần hồ, lần trước ăn thấy ổn mà giá cũng mềm, tầm hai trăm một người là no',
    'Được đó, nhớ đặt bàn trước vì cuối tuần hay kín chỗ lắm',
    'Mình dị ứng hải sản nha, lẩu nấm thì ok',
    '7h30 gặp ở cổng chợ nhé mọi người, ai tới trễ nhắn trước một tiếng',
    'Chốt!',
  ];
  for (let i = 0; i < 24; i++) {
    const u = ai[i % ai.length];
    const r = await fetch(`${fixture.apiUrl}/contexts/${fixture.groupId}/messages`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${u.token}`, 'Idempotency-Key': crypto.randomUUID() },
      body: JSON.stringify({ kind: 'text', body: cau[i % cau.length], image_url: null, card: null }),
    });
    if (r.status !== 201 && r.status !== 200) throw Error(`seed message ${r.status}: ${await r.text()}`);
  }
  fs.writeFileSync(dau, '');
}

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
      // Work arriving while Nếp is already out. Sampled for ten seconds: the
      // second slip must show, and nothing may widen past the pulled-out slip
      // (the old four-second line lay over a card's price and hours).
      const dongThoiGian = [];
      let rongNhat = 0;
      for (let t = 0; t < 10000; t += 250) {
        const o = await page.evaluate(() => {
          const r = document.querySelector('[data-testid="nep-dia"]')?.getBoundingClientRect();
          return {
            url: location.pathname,
            mep: !!document.querySelector('[data-testid="nep-mep"]'),
            dia: !!r,
            rong: r ? Math.round(r.width) : 0,
            sau: !!document.querySelector('[data-testid="nep-to-sau"]'),
          };
        });
        rongNhat = Math.max(rongNhat, o.rong);
        const { t: _t, ...truocKhongT } = dongThoiGian.at(-1) ?? {};
        if (!dongThoiGian.length || JSON.stringify(truocKhongT) !== JSON.stringify(o)) dongThoiGian.push({ t, ...o });
        await sleep(250);
      }
      await chup('5-ra-co-viec');
      const cuoi = dongThoiGian.at(-1);
      ket.steps.push({ scheme, name: 'ra_co_viec_khong_bung_rong', pass: cuoi.dia && cuoi.sau && rongNhat <= 56, rongNhat, dongThoiGian });
      save();
      return;
    }

    await sleep(1500);
    await chup('1-tab-dau');

    const tabDau = await doChe(page);
    ket.steps.push({ scheme, name: 'nghi_tab_dau_khong_che_chu', pass: tabDau.hopDock.length > 0 && tabDau.chuBiChe.length === 0, ...tabDau });
    save();

    if (mode === 'thuong') {
      // Open the panel and close it again: pulled out is only the first of
      // the two taps, so Nếp must come back to the edge, not rest outside.
      await page.click('[data-testid="nep-mep"]');
      await page.waitForSelector('[data-testid="nep-dia"]', { timeout: 10000 });
      await page.click('[data-testid="nep-dia"]');
      await page.waitForSelector('[data-testid="nep-ngu-canh"]', { timeout: 10000 });
      await page.keyboard.press('Escape');
      await sleep(1200);
      const sauBang = await doChe(page);
      const veMep = !!(await page.$('[data-testid="nep-mep"]'));
      ket.steps.push({ scheme, name: 'dong_bang_ve_mep_khong_che_chu', pass: veMep && sauBang.chuBiChe.length === 0, veMep, ...sauBang });
      save();
    }

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
    // Making room means not even the edge is drawn over the sheet.
    ket.steps.push({ scheme, name: 'nhuong_cho_khay_khong_ve_gi', pass: khay.hopDock.length === 0, ...khay });
    save();

    if (mode === 'bao') {
      // The money law (ADR-0033 §2.2-2.3): with work waiting, a money screen
      // shows the bare edge only -- no second slip, no Nếp.
      // Walked there in the app, not loaded by URL: a reload would drop the
      // work and the check would pass on nothing. The second slip must be
      // seen on the way in, or the result says nothing about the law.
      const docMep = () => page.evaluate(() => ({
        url: location.pathname,
        mep: !!document.querySelector('[data-testid="nep-mep"]'),
        dia: !!document.querySelector('[data-testid="nep-dia"]'),
        sau: !!document.querySelector('[data-testid="nep-to-sau"]'),
      }));
      await page.keyboard.press('Escape');
      await sleep(800);
      await page.goBack();
      await page.waitForSelector('[role="tab"][aria-label="Cá nhân"]', { timeout: 20000 });
      await page.click('[role="tab"][aria-label="Cá nhân"]');
      await sleep(1500);
      const truoc = await docMep();
      const [loiVao] = await page.$$('xpath/.//*[text()="Tài chính của tôi"]');
      if (loiVao) await loiVao.click();
      await sleep(2500);
      await chup('4-man-tien');
      const tien = await docMep();
      ket.steps.push({ scheme, name: 'man_tien_chi_mep_tron', pass: truoc.sau && tien.url === '/finance' && tien.mep && !tien.dia && !tien.sau, truoc, ...tien });
      // The edge there is a door, not a face: a tap may not bring Nếp out.
      await page.click('[data-testid="nep-mep"]');
      await sleep(900);
      const sauCham = await docMep();
      ket.steps.push({ scheme, name: 'man_tien_cham_mep_khong_ra_mat', pass: sauCham.mep && !sauCham.dia, ...sauCham });
      save();
    }

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
      await chup('4-keo-ra-hoi-thoai');
      // The canary is measured on the first tab, not here. In a conversation
      // the text of a bubble stops at the bubble's padding, well short of the
      // slip even when it is out (measured 24-09: 0 of 363), so "nothing
      // covered" here cannot tell a blind measure from a clean one. On the
      // first tab a card's metadata line runs to the page margin, and the slip
      // pulled out DOES stand on it.
      await page.goBack();
      await page.waitForSelector('[role="tab"][aria-label="Khám phá"]', { timeout: 20000 });
      await page.click('[role="tab"][aria-label="Khám phá"]');
      await sleep(1500);
      // Leaving the conversation put Nếp back in the edge (ADR-0035), so it is
      // pulled out again here, on the page the canary measures.
      await page.click('[data-testid="nep-mep"]');
      await sleep(900);
      await chup('4-keo-ra');
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

if (mode !== 'hoi') await nhoiTin();
for (const scheme of (process.env.SCHEMES ?? 'light,dark').split(',')) await chay(scheme);
ket.pass = ket.steps.every(s => s.pass);
save();
console.log(JSON.stringify(ket, null, 2));
