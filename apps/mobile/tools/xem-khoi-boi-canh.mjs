/**
 * Open the real app against the real stack and LOOK at the context block.
 *
 * Everything else that passed today stops short of this. `tsc`, 959 client
 * tests and the e2e tier all agree the numbers are right; none of them can say
 * whether the block renders at all, or whether the number a person reads is the
 * number that actually left the phone. That second question is the only one
 * worth a browser, so it is the one this asserts.
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

const nguoi = fixture.users[0];
const group = fixture.groupId;

// Seed the conversation over the API: the point here is what the SCREEN does
// with messages that exist, not whether typing works.
const cauNoi = ['Tối thứ Bảy đi đâu tụi bay', 'Tao dị ứng hải sản nhé', 'Dưới 300k mỗi đứa thôi', 'Quận 1 cho tiện, tao ở Bình Thạnh'];
for (const [i, body] of cauNoi.entries()) {
  const nguoiGui = fixture.users[i % 3];
  const r = await fetch(`${fixture.apiUrl}/contexts/${group}/messages`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${nguoiGui.token}`, 'content-type': 'application/json', 'Idempotency-Key': crypto.randomUUID() },
    body: JSON.stringify({ kind: 'text', body }),
  });
  if (!r.ok) throw Error(`seed message ${r.status}`);
}
ket.steps.push({ name: 'seed_hoi_thoai', pass: true, soTin: cauNoi.length });
save();

const browser = await puppeteer.launch({ executablePath: chrome, headless: true, args: ['--no-sandbox'] });
try {
  const context = await browser.createBrowserContext();
  const page = await context.newPage();
  await page.setViewport({ width: 430, height: 932 });
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
  await page.screenshot({ path: path.join(output, '01-vao-app.png') });
  await page.click('[role="tab"][aria-label="Tin nhắn"]');
  await page.waitForSelector('[aria-label="Mở nhóm Phòng kiểm thử đồng thời"]', { timeout: 30000 });
  await page.click('[aria-label="Mở nhóm Phòng kiểm thử đồng thời"]');
  await page.waitForSelector('[aria-label="Thêm vào cuộc trò chuyện"]', { timeout: 30000 });
  await sleep(1500);
  await page.screenshot({ path: path.join(output, '02-trong-nhom.png') });

  await page.click('[aria-label="Thêm vào cuộc trò chuyện"]');
  await page.click('[aria-label="Tờ hẹn"]');
  await page.waitForSelector('[aria-label="Lời nhờ lập kế hoạch"]', { timeout: 20000 });
  await page.type('[aria-label="Lời nhờ lập kế hoạch"]', 'Lên kế hoạch tối thứ Bảy giúp hội');
  await sleep(800);

  const khoi = await page.evaluate(() => {
    const el = document.querySelector('[data-testid="chat-boi-canh"]');
    const nut = [...document.querySelectorAll('[role="button"]')].map(e => e.textContent.trim());
    return { co: !!el, chu: el ? el.textContent : null, nut };
  });
  await page.screenshot({ path: path.join(output, '03-khoi-boi-canh.png') });
  ket.steps.push({ name: 'khoi_hien_ra', pass: khoi.co, chu: khoi.chu });
  save();
  if (!khoi.co) throw Error('Khối «Mình đang thấy» không có trên màn');

  // Open the list, so the evidence shows the turns themselves rather than a count.
  const moRong = await page.$('[data-testid="chat-boi-canh-mo"]');
  if (moRong) { await moRong.click(); await sleep(600); await page.screenshot({ path: path.join(output, '04-mo-danh-sach.png'), fullPage: true }); }
  const muc = await page.$$eval('[data-testid="chat-boi-canh-muc"]', els => els.map(e => e.textContent));
  ket.steps.push({ name: 'danh_sach_mo_ra', pass: muc.length > 0, muc });
  save();

  const soTrenMan = Number((khoi.chu?.match(/\d+/) || [])[0] ?? -1);
  const cho = page.waitForResponse(r => r.request().method() === 'POST' && r.url().endsWith('/ai-invocations'));
  const guiNut = await page.waitForFunction(() => [...document.querySelectorAll('[role="button"]')].find(e => e.textContent.trim() === 'Gửi lời nhờ cho AI'), { timeout: 20000 });
  await guiNut.asElement().click();
  const res = await cho;
  let than = {};
  try { than = JSON.parse(res.request().postData() || '{}'); } catch { than = {}; }
  const soTrenDay = than.boi_canh?.luot?.length ?? -1;
  ket.steps.push({
    name: 'so_tren_man_bang_so_tren_day',
    pass: res.status() === 202 && soTrenMan >= 0 && soTrenMan === soTrenDay,
    status: res.status(), soTrenMan, soTrenDay,
    coKhoaCam: JSON.stringify(than).includes('image_url') || JSON.stringify(than).includes(nguoi.id ?? '\u0000'),
  });
  save();
  await sleep(2500);
  await page.screenshot({ path: path.join(output, '05-sau-khi-gui.png'), fullPage: true });
  ket.pass = ket.steps.every(s => s.pass);
  save();
  console.log(JSON.stringify(ket, null, 2));
} finally {
  await browser.close();
}
