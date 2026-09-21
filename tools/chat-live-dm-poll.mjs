/** Exercise an existing synthetic live browser session without exposing credentials. */
import puppeteer from '../apps/mobile/node_modules/puppeteer-core/lib/esm/puppeteer/puppeteer-core.js';
import fs from 'node:fs/promises';
import path from 'node:path';

const [fixturePath, endpointPath, outputDir, phase = 'all'] = process.argv.slice(2);
if (!fixturePath || !endpointPath || !outputDir) throw new Error('Usage: node tools/chat-live-dm-poll.mjs <external-fixture> <external-browser-endpoint> <external-output-dir>');
const fixture = JSON.parse(await fs.readFile(fixturePath, 'utf8'));
const browser = await puppeteer.connect({ browserWSEndpoint: (await fs.readFile(endpointPath, 'utf8')).trim(), defaultViewport: null });
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
await fs.mkdir(outputDir, { recursive: true });
const previous = phase === 'dm' ? JSON.parse(await fs.readFile(path.join(outputDir, 'results.json'), 'utf8')) : {};
const result = { ...previous, at: new Date().toISOString(), provenance: 'Existing live Expo web browsers and real API; synthetic accounts; no response interception', mapping: [], poll: previous.poll ?? {}, dm: {} };
const selected = new Map();
const pages = await browser.pages();
const user = index => fixture.users.find(value => value.index === index);
const api = async (index, route) => {
  const response = await fetch(fixture.apiUrl + route, { headers: { Authorization: `Bearer ${user(index).token}` } });
  return { status: response.status, data: await response.json().catch(() => null) };
};
try {
  await Promise.all(pages.map(async (page, position) => {
    if (!(await page.evaluate(() => Boolean(window.__chatE2ESeen)).catch(() => false))) return;
    let bearer;
    const listen = request => { const auth = request.headers().authorization; if (auth) bearer = auth; };
    page.on('request', listen);
    await sleep(6500);
    page.off('request', listen);
    if (!bearer) return;
    const response = await fetch(fixture.apiUrl + '/people/me', { headers: { Authorization: bearer } });
    const me = await response.json();
    const index = fixture.users.find(value => value.personId === (me.person_id ?? me.id))?.index;
    result.mapping.push({ position, index });
    if ([2, 3].includes(index)) {
      if (process.env.CHAT_E2E_PROTECT_FIRST_TWO !== '0' && position <= 2) throw new Error('Refusing to use the first two retained application pages');
      selected.set(index, page);
    }
  }));
  if (selected.size !== 2) throw new Error('Could not identify both requested live users from actual API identity');
  const a = selected.get(2), b = selected.get(3);
  if (phase !== 'dm') {
  const question = 'QA design A: Toi nay an gi?';
  const card = async page => (await page.evaluateHandle(text => {
    const heading = [...document.querySelectorAll('div')].find(element => element.textContent === text && element.children.length === 0);
    return heading?.parentElement;
  }, question)).asElement();
  const snapshot = async page => {
    const element = await card(page);
    if (!element) throw new Error('Target poll is not rendered in the live conversation');
    return await element.evaluate(element => element.innerText);
  };
  const select = async (page, option) => {
    const element = await card(page);
    const radio = (await element.$$('[role="radio"]'))[option];
    await radio.scrollIntoView();
    await radio.click();
    await sleep(1000);
  };
  await select(a, 0);
  result.poll.aBeforePeerVote = await snapshot(a);
  const peerCard = await card(b);
  const checked = await peerCard.$$eval('[role="radio"]', elements => elements.findIndex(element => element.getAttribute('aria-checked') === 'true'));
  let voteRoute = null;
  const ballot = response => {
    const route = new URL(response.url()).pathname;
    if (route.endsWith('/ballots') && response.request().method() === 'POST') {
      voteRoute = route.replace(/\/ballots$/, '');
      result.poll.peerPostStatus = response.status();
    }
  };
  b.on('response', ballot);
  await select(b, checked === 1 ? 0 : 1);
  await sleep(8500);
  b.off('response', ballot);
  result.poll.aAfterPeerVoteAndWait = await snapshot(a);
  result.poll.bAfterOwnVote = await snapshot(b);
  result.poll.waitAfterPeerVoteMs = 8500;
  result.poll.observerUnchanged = result.poll.aBeforePeerVote === result.poll.aAfterPeerVoteAndWait;
  if (voteRoute) result.poll.server = await api(2, voteRoute);
  await (await card(a)).screenshot({ path: path.join(outputDir, '01-poll-observer-after-peer.png') });
  await (await card(b)).screenshot({ path: path.join(outputDir, '02-poll-voter-after-peer.png') });
  }
  const contextsA = await api(2, '/people/me/contexts');
  const contextsB = await api(3, '/people/me/contexts');
  const pairA = contextsA.data.contexts.find(item => item.kind === 'pair' && item.counterpart?.id === user(3).personId);
  const pairB = contextsB.data.contexts.find(item => item.id === pairA?.id);
  if (!pairA || !pairB) throw new Error('Both real server memberships must identify the same direct conversation');
  const visible = async (page, selector) => {
    for (let attempt = 0; attempt < 100; attempt++) {
      for (const element of await page.$$(selector)) {
        const box = await element.boundingBox();
        if (box?.width > 0 && box?.height > 0) return element;
      }
      await sleep(100);
    }
    throw new Error(`No visible control: ${selector}`);
  };
  const openConversation = async (page, label) => {
    await (await visible(page, '[aria-label="Quay lại"]')).click();
    const selector = `[aria-label="Mở nhóm ${label}"]`;
    await (await visible(page, selector)).click();
    await visible(page, '[aria-label="Ô soạn tin"]');
  };
  await openConversation(a, pairA.counterpart.display_name);
  await openConversation(b, pairB.counterpart.display_name);
  result.dm.sameServerPair = new URL(a.url()).pathname.includes(pairA.id) && new URL(b.url()).pathname.includes(pairA.id);
  const marker = `DM-LIVE-${Date.now()}`;
  const send = async (page, text) => {
    await (await visible(page, '[aria-label="Ô soạn tin"]')).type(text);
    const post = page.waitForResponse(response => response.request().method() === 'POST' && new URL(response.url()).pathname === `/contexts/${pairA.id}/messages`);
    await (await visible(page, '[aria-label="Gửi tin nhắn"]')).click();
    return (await post).status();
  };
  const forward = `${marker} user-03: Minh den cong, ban toi chua?`;
  const reverse = `${marker} user-04: Minh den roi, di cung nhau nhe.`;
  result.dm.forwardPostStatus = await send(a, forward);
  await b.waitForFunction(text => document.body.innerText.includes(text), { timeout: 15000 }, forward);
  result.dm.bReceivedForward = true;
  result.dm.reversePostStatus = await send(b, reverse);
  await a.waitForFunction(text => document.body.innerText.includes(text), { timeout: 15000 }, reverse);
  result.dm.aReceivedReverse = true;
  await a.screenshot({ path: path.join(outputDir, '03-dm-user03-bidirectional.png') });
  await b.screenshot({ path: path.join(outputDir, '04-dm-user04-bidirectional.png') });
  const history = await api(2, `/contexts/${pairA.id}/messages?limit=100`);
  result.dm.serverStatus = history.status;
  result.dm.serverStoredBoth = [forward, reverse].every(body => history.data.messages.some(item => item.body === body));
  const outsiderContexts = await api(4, '/people/me/contexts');
  result.dm.thirdUserDoesNotListPair = !outsiderContexts.data.contexts.some(item => item.id === pairA.id);
  result.dm.thirdUserReadStatus = (await api(4, `/contexts/${pairA.id}/messages?limit=10`)).status;
  const group = contextsA.data.contexts.find(item => item.id === fixture.groupId);
  await openConversation(a, group.display_name);
  await openConversation(b, group.display_name);
  result.dm.returnedBothToGroup = true;
  await fs.writeFile(path.join(outputDir, 'results.json'), JSON.stringify(result, null, 2));
  console.log(JSON.stringify({ mappedUsers: result.mapping.length, selectedUsers: [2, 3], poll: result.poll, dm: result.dm }, null, 2));
} finally {
  await browser.disconnect();
}
