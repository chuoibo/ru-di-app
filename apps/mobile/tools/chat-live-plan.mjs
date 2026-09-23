/** Real-provider UI gate: explicit sharing, two-person promotion and peer sync. */
import fs from 'node:fs';
import path from 'node:path';
import puppeteer from 'puppeteer-core';
// The pinned Chrome lives in the puppeteer cache under whoever's HOME this
// is; a path through one person's home directory is a machine, not a default.
// repo-guard: allow=long-number reason=public-chrome-version
const chromeMacDinh = () => process.env.CHROME_PATH ?? `${process.env.HOME ?? ''}/.cache/puppeteer/chrome/linux-148.0.7778.97/chrome-linux64/chrome`;

const fixture = JSON.parse(fs.readFileSync(process.env.CHAT_E2E_SESSIONS, 'utf8'));
const output = process.env.CHAT_E2E_OUTPUT;
if (!output?.startsWith('/tmp/')) throw Error('Synthetic evidence must stay outside the checkout.');
fs.mkdirSync(output, { recursive: true, mode: 0o700 });
const web = process.env.CHAT_E2E_WEB ?? 'http://127.0.0.1:8178';
const result = { mockResponses: false, provider: 'configured real inference', steps: [] };
const save = () => fs.writeFileSync(path.join(output, 'plan.json'), JSON.stringify(result, null, 2));
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
// repo-guard: allow=long-number reason=public-chrome-version
const browser = await puppeteer.launch({ executablePath: chromeMacDinh(), headless: true, args: ['--no-sandbox', '--disable-background-timer-throttling', '--disable-renderer-backgrounding'] });
const fill = async (page, selector, value) => {
  await page.click(selector); await page.keyboard.down('Control'); await page.keyboard.press('KeyA'); await page.keyboard.up('Control'); await page.keyboard.type(value);
};
const clickText = async (page, text, root = 'body') => {
  const element = await page.waitForFunction((value, selector) => [...(document.querySelector(selector)?.querySelectorAll('[role="button"]') ?? [])].find(el => el.textContent.trim() === value), { timeout: 15000 }, text, root);
  await element.asElement().scrollIntoView(); await element.asElement().click();
};
const read = async route => {
  const response = await fetch(fixture.apiUrl + route, { headers: { Authorization: `Bearer ${fixture.users[0].token}` } });
  if (!response.ok) throw Error(`Verification API ${response.status}`);
  return response.json();
};
try {
  const pages = [];
  for (let index = 0; index < 3; index++) {
    const context = await browser.createBrowserContext(), page = await context.newPage();
    await page.setViewport({ width: 430, height: 932 });
    const cdp = await page.createCDPSession(); await cdp.send('Emulation.setFocusEmulationEnabled', { enabled: true });
    await page.goto(web + '/login', { waitUntil: 'domcontentloaded' });
    await page.waitForSelector('[aria-label="Ô số điện thoại"]'); await page.type('[aria-label="Ô số điện thoại"]', fixture.users[index].phone);
    let accepted = false;
    for (let attempt = 0; attempt < 3; attempt++) {
      const pending = page.waitForResponse(r => r.request().method() === 'POST' && r.url().endsWith('/auth/otp/request'));
      await page.click('[data-testid="login-gui-ma"]'); const response = await pending;
      if (response.status() === 429) { await sleep(61000); continue; }
      if (![200, 202].includes(response.status())) throw Error(`OTP request ${response.status()}`);
      accepted = true; break;
    }
    if (!accepted) throw Error('OTP limit remained active');
    await page.waitForSelector('[data-testid="otp-input"]'); await page.type('[data-testid="otp-input"]', '000000');
    await page.waitForSelector('[role="tab"][aria-label="Tin nhắn"]'); await page.click('[role="tab"][aria-label="Tin nhắn"]');
    await page.waitForSelector('[aria-label="Mở nhóm Phòng kiểm thử đồng thời"]'); await page.click('[aria-label="Mở nhóm Phòng kiểm thử đồng thời"]');
    await page.waitForSelector('[aria-label="Thêm vào cuộc trò chuyện"]'); pages.push(page);
  }
  const [author, peer, observer] = pages;
  await author.click('[aria-label="Thêm vào cuộc trò chuyện"]'); await author.click('[aria-label="Tờ hẹn"]');
  await fill(author, '[aria-label="Lời nhờ lập kế hoạch"]', 'Lập tờ hẹn tổng hợp cho nhóm thử nghiệm: 4 người đi cà phê và ăn tối ở Đà Lạt, 18:00 đến 20:00, 200000 đồng mỗi người. Chỉ chọn địa điểm có trong danh mục được cung cấp. Ghi giờ HH:mm cho từng chặng.');
  // The block above the send button names a number of messages. Asserting the
  // SENTENCE would only prove some words are on screen; asserting the NUMBER
  // against what the request actually carried is the only version of this check
  // that could fail when the screen starts lying.
  const xemTruoc = await author.evaluate(() => {
    const el = document.querySelector('[data-testid="chat-boi-canh"]');
    return el ? el.textContent : '';
  });
  const soTrenMan = Number((xemTruoc.match(/\d+/) || [])[0] ?? -1);
  await author.screenshot({ path: path.join(output, '01-boi-canh.png') });
  const pending = author.waitForResponse(r => r.request().method() === 'POST' && r.url().endsWith('/ai-invocations'));
  await clickText(author, 'Gửi lời nhờ cho AI'); const response = await pending, invocation = await response.json();
  let soTrenDay = -1;
  try { soTrenDay = JSON.parse(response.request().postData() || '{}').boi_canh?.luot?.length ?? -1; } catch { soTrenDay = -1; }
  result.steps.push({
    name: 'explicit_invocation',
    pass: response.status() === 202 && soTrenMan >= 0 && soTrenMan === soTrenDay,
    status: response.status(), invocationId: invocation.id,
    xemTruoc, soTrenMan, soTrenDay,
  }); save();
  if (response.status() !== 202) throw Error(`Invocation refused: ${response.status()}`);
  let job;
  for (let attempt = 0; attempt < 80; attempt++) {
    job = await read(`/contexts/${fixture.groupId}/ai-invocations/${invocation.id}`);
    if (['succeeded', 'failed', 'cancelled'].includes(job.status)) break;
    await sleep(1000);
  }
  result.steps.push({ name: 'real_model_completion', pass: job.status === 'succeeded', job }); save();
  if (job.status !== 'succeeded') throw Error(`Model job ${job.status}: ${job.code}`);
  const source = `[data-testid="chat-message-${job.message_id}"]`;
  await observer.waitForSelector(source); await observer.screenshot({ path: path.join(output, '02-peer-plan.png') });
  for (const page of [author, peer]) {
    await clickText(page, 'Sửa tờ hẹn này', source);
    await page.waitForSelector('[aria-label="Ô ngân sách một người"]');
    await fill(page, '[aria-label="Ô số người"]', '4'); await fill(page, '[aria-label="Ô ngân sách một người"]', '200000');
    const times = await page.$$('[aria-label^="Giờ · "]');
    for (let index = 0; index < times.length; index++) {
      await times[index].click(); await page.keyboard.down('Control'); await page.keyboard.press('KeyA'); await page.keyboard.up('Control'); await page.keyboard.type(`${String(18 + index).padStart(2, '0')}:00`);
    }
  }
  await author.screenshot({ path: path.join(output, '03-human-review.png') });
  const promotions = [author, peer].map(page => page.waitForResponse(r => r.request().method() === 'POST' && r.url().endsWith('/plan-promotions')));
  await Promise.all([author, peer].map(page => clickText(page, 'Xác nhận và tạo kèo')));
  const replies = await Promise.all(promotions), bodies = await Promise.all(replies.map(r => r.json()));
  const same = bodies[0].outing_id && bodies[0].outing_id === bodies[1].outing_id && bodies.every(body => body.source_message_id === job.message_id);
  result.steps.push({ name: 'two_people_promote_one_plan', pass: Boolean(same) && replies.map(r => r.status()).sort().join(',') === '200,201', statuses: replies.map(r => r.status()), outings: bodies }); save();
  if (!same) throw Error('Concurrent confirmation did not resolve to one outing');
  await observer.waitForFunction(selector => document.querySelector(selector)?.textContent.includes('Mở kèo của hội'), { timeout: 10000 }, source);
  await observer.screenshot({ path: path.join(output, '04-peer-promoted.png') });
  await clickText(observer, 'Mở kèo của hội', source);
  await observer.waitForFunction(id => location.pathname.endsWith('/outings/' + id), { timeout: 10000 }, bodies[0].outing_id);
  await observer.screenshot({ path: path.join(output, '05-shared-outing.png') });
  result.steps.push({ name: 'observer_opens_same_outing', pass: true }); save();
} catch (error) {
  result.steps.push({ name: 'flow_error', pass: false, error: error.message }); save(); process.exitCode = 1;
} finally {
  if (result.steps.some(step => !step.pass)) process.exitCode = 1;
  console.log(JSON.stringify(result, null, 2)); await browser.close();
}
