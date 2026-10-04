/* UI-008/011/012: real keyboard/pointer input, isolated synthetic transport.
 * No AI provider is called. Consent is tested against the exact outbound body.
 */
import assert from "node:assert/strict";
import test from "node:test";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { findChrome, launch, serve } from "./chrome-cdp.mjs";
import { lyDoBanDungCu } from "./tuoi-ban-dung.mjs";

const root = fileURLToPath(new URL("..", import.meta.url));
const exported = process.env.MOBILE_WEB_EXPORT ?? join(root, ".expo-build-check");
const description = "Một cuốn sổ mở bên cửa sổ, ba người bạn cùng lên kế hoạch ghé quán cà phê trên đồi. Giữ nét giấy thủ công, ánh sáng buổi sáng và chỗ trống để viết kỷ niệm.";
async function key(page, key, code) {
  await page.call("Input.dispatchKeyEvent", { type: key === "Enter" ? "keyDown" : "rawKeyDown", key, code: key, windowsVirtualKeyCode: code, ...(key === "Enter" ? { text: "\r", unmodifiedText: "\r" } : {}) });
  await page.call("Input.dispatchKeyEvent", { type: "keyUp", key, code: key, windowsVirtualKeyCode: code });
}
async function setup(signedIn = true) {
  assert.equal(lyDoBanDungCu(exported, root), null);
  assert.ok(findChrome(), "Chrome is required for Nếp regression");
  const server = await serve(exported);
  const page = await launch(findChrome());
  await page.call("Page.addScriptToEvaluateOnNewDocument", { source: `
    const realFetch = globalThis.fetch;
    globalThis.nepRequests = [];
    globalThis.nepReads = [];
    globalThis.nepSkip = null;
    new MutationObserver(() => {
      const skip = [...document.querySelectorAll('[role="button"]')].find(e => e.getAttribute('aria-label')?.includes('Bỏ qua chuyển động của Nếp'));
      if (skip) globalThis.nepSkip = skip.getAttribute('aria-label');
    }).observe(document, {subtree:true,childList:true,attributes:true});
    globalThis.fetch = async (input, init) => {
      const url = String(input);
      globalThis.nepReads.push(url);
      if (url.endsWith('/sessions/web/resume') && !${signedIn}) return new Response('{}',{status:401});
      if (url.endsWith('/sessions/web/resume')) return new Response(JSON.stringify({token:'synthetic-nep-session',person_id:'synthetic-person',expires_at:'2999-01-01T00:00:00Z',issued_via:'otp',profile:{display_name:'Người thử tổng hợp'}}),{status:200});
      if (url.endsWith('/people/me/contexts')) return new Response(JSON.stringify({contexts:[{id:'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa',display_name:'Hội thử tổng hợp',my_state:'active',my_role:'member',membership_id:'bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb',member_count:3,unread_count:0}]}),{status:200});
      if (url.endsWith('/me/nep/media') && init?.method === 'POST') {
        globalThis.nepRequests.push(JSON.parse(init.body));
        return new Response('{"code":"nep_media_chua_cau_hinh"}',{status:503});
      }
      if (url.endsWith('/people/me/saved-places')) return new Response('{"saved":[]}',{status:200});
      if (url.split('?')[0].endsWith('/places')) return new Response(JSON.stringify({places:[],categories:[{id:'cafe',label:'Cafe'}],destination:{id:'synthetic-city',name:'Nơi thử tổng hợp'}}),{status:200});
      if (url.includes('api.build-check.invalid')) return new Response('{}',{status:404});
      return realFetch(input,init);
    };
  ` });
  return { page, url: server.url, close: async () => { await page.close(); await server.close(); } };
}

test("drawing consent cancels without a request, keeps the draft and sends only the confirmed description", async () => {
  const { page, url, close } = await setup();
  try {
    await page.viewport(320, 700);
    await page.goto(`${url}explore`, () => !!document.querySelector('[data-testid="nep-mep"]') && !document.querySelector('[data-testid="opening-app"]') && !document.querySelector('[inert]'));
    await page.evaluate(() => [...document.querySelectorAll('[role="tab"]')].find(e => e.textContent.includes("Cá nhân")).focus());
    await key(page, "Enter", 13);
    await page.waitFor(() => location.pathname === "/profile");
    await page.evaluate(() => [...document.querySelectorAll('[role="tab"]')].find(e => e.textContent.includes("Khám phá")).focus());
    await key(page, "Enter", 13);
    await page.waitFor(() => location.pathname === "/explore" && !document.querySelector('[inert]'));
    await page.waitFor(() => [...document.querySelectorAll('[role="button"]')].some(e => e.textContent.trim() === "Cafe"), { label: "synthetic catalogue categories ready", diagnose: () => ({ text: document.body.innerText.slice(0,1800), reads: globalThis.nepReads, labels: [...document.querySelectorAll('[role="button"]')].map(e => ({name:e.getAttribute("aria-label"),text:e.textContent.slice(0,80)})) }) });
    const gutter = await page.evaluate(() => {
      const chip = [...document.querySelectorAll('[role="button"]')].find(e => e.textContent.trim() === "Cafe");
      let rail = chip?.parentElement;
      while (rail && !["auto", "scroll"].includes(getComputedStyle(rail).overflowX)) rail = rail.parentElement;
      return { right: rail?.getBoundingClientRect().right, bookmarkLeft: document.querySelector('[data-testid="nep-mep"]').getBoundingClientRect().left };
    });
    assert.ok(gutter.right <= gutter.bookmarkLeft, `the category viewport ends before the tucked bookmark: ${JSON.stringify(gutter)}`);
    // The bookmark is mostly tucked outside the viewport; activate its actual
    // keyboard control instead of sending a pointer to its offscreen centre.
    await page.evaluate(() => document.querySelector('[data-testid="nep-mep"]').focus());
    await key(page, "Enter", 13);
    await page.waitFor(() => !!document.querySelector('[aria-label="Mở Nếp"]'), { label: "bookmark untucks", diagnose: () => ({ active: document.activeElement?.outerHTML.slice(0,250), labels: [...document.querySelectorAll('[role="button"]')].map(e=>e.getAttribute('aria-label')), inert: document.querySelectorAll('[inert]').length }) });
    await page.evaluate(() => document.querySelector('[aria-label="Mở Nếp"]').focus());
    await key(page, "Enter", 13);
    await page.waitFor(() => !!document.querySelector('textarea[aria-label="Hỏi Nếp"]'));
    assert.ok(await page.evaluate(() => document.querySelector('[data-testid="nep-bang"]').textContent.includes("Trợ lý riêng của bạn")), "the test uses a live synthetic session, not the demonstration catalogue");
    const chips = await page.evaluate(() => [...document.querySelectorAll('[data-testid="nep-bang"] [aria-label]')].filter(e => ["Quanh đây có gì hay?", "Chỗ này hợp đi mấy người?"].includes(e.getAttribute("aria-label"))).map(e => e.getBoundingClientRect().height));
    assert.equal(chips.length, 2);
    assert.ok(chips.every(height => height >= 48));
    await page.typeInto("Hỏi Nếp", description);
    await page.clickChu("Vẽ");
    await page.waitFor(() => document.activeElement?.textContent === "Nhờ Nếp vẽ?", { label: "consent heading focus" });
    assert.ok(await page.evaluate((text) => document.querySelector('[data-testid="nep-xac-nhan-ve"]').textContent.includes(text), description));
    assert.equal(await page.evaluate(() => globalThis.nepRequests.length), 0);
    await key(page, "Escape", 27);
    await page.waitFor(() => document.activeElement?.getAttribute("aria-label") === "Hỏi Nếp", { label: "cancel restores editor focus" });
    assert.equal(await page.evaluate(() => document.querySelector('textarea[aria-label="Hỏi Nếp"]').value), description);
    assert.equal(await page.evaluate(() => document.body.getBoundingClientRect().top), 0, "returning focus keeps the app viewport anchored");
    assert.equal(await page.evaluate(() => globalThis.nepRequests.length), 0);
    await page.clickChu("Vẽ");
    await page.waitFor(() => !!document.querySelector('[data-testid="nep-xac-nhan-ve"]'));
    assert.ok(await page.evaluate((text) => document.querySelector('[data-testid="nep-xac-nhan-ve"]').textContent.includes(text), description));
    const history = await page.call("Page.getNavigationHistory");
    await page.call("Page.navigateToHistoryEntry", { entryId: history.entries[history.currentIndex - 1].id });
    await page.waitFor(() => document.activeElement?.getAttribute("aria-label") === "Hỏi Nếp", { label: "browser Back cancels only the confirmation" });
    assert.equal(await page.evaluate(() => location.pathname), "/explore");
    assert.equal(await page.evaluate(() => globalThis.nepRequests.length), 0);
    await page.clickChu("Vẽ");
    await page.waitFor(() => !!document.querySelector('[data-testid="nep-xac-nhan-ve"]'));
    await page.clickChu("Vẽ đi");
    await page.waitFor(() => document.body.innerText.includes("Rủ Đi chưa bật phần vẽ ảnh của Nếp."), { label: "visible synthetic failure", diagnose: () => ({text:document.body.innerText.slice(-1100),requests:globalThis.nepRequests}) });
    const failure = await page.evaluate(() => {
      const bounds = document.querySelector('[data-testid="nep-loi-ve"]').getBoundingClientRect();
      return { top: bounds.top, bottom: bounds.bottom, viewport: innerHeight };
    });
    assert.ok(failure.top >= 0 && failure.bottom <= failure.viewport, `drawing failure stays beside the editor: ${JSON.stringify(failure)}`);
    const requests = await page.evaluate(() => globalThis.nepRequests);
    assert.equal(requests.length, 1);
    assert.deepEqual(requests[0], { loai: "anh", mo_ta: description, man: "explore" });
    assert.equal(await page.evaluate(() => document.querySelector('textarea[aria-label="Hỏi Nếp"]').value), description);
    await page.clickChu("Vẽ");
    await page.clickChu("Sửa mô tả");
    assert.equal(await page.evaluate(() => globalThis.nepRequests.length), 1, "another draw needs another consent, cancellation never draws");
    await page.waitFor(() => document.activeElement?.getAttribute("aria-label") === "Hỏi Nếp");
    // Closing the parent cancels the draft consent rather than consuming a
    // sheet dismiss as an inline Back. The default Sheet contract stays intact.
    await page.clickChu("Vẽ");
    await page.clickLabel("Đóng bảng");
    await page.waitFor(() => !document.querySelector('[data-testid="nep-bang"]'), { label: "parent sheet dismissed" });
    assert.equal(await page.evaluate(() => globalThis.nepRequests.length), 1);
  } finally { await close(); }
});

test("Nếp performance is a named keyboard skip while playing, then a static image outside the Tab order", async () => {
  const { page, url, close } = await setup();
  try {
    await page.viewport(390, 844);
    await page.call("Emulation.setEmulatedMedia", { features: [{ name: "prefers-reduced-motion", value: "no-preference" }] });
    await page.goto(`${url}explore`, () => !!document.querySelector('[data-testid="nep-mep"]') && !document.querySelector('[data-testid="opening-app"]') && !document.querySelector('[inert]'));
    await page.clickLabel("Tạo mới");
    await page.waitFor(() => !!globalThis.nepSkip, { label: "named performance appeared", diagnose: () => ({ text: document.body.innerText.slice(0,900), labels: [...document.querySelectorAll('[role="button"]')].map(e=>e.getAttribute('aria-label')), observed: globalThis.nepSkip, reduced: matchMedia('(prefers-reduced-motion: reduce)').matches }) });
    const label = await page.evaluate(() => globalThis.nepSkip);
    await page.waitFor(() => !![...document.querySelectorAll('[role="button"]')].find(e => e.getAttribute("aria-label") === globalThis.nepSkip), {label:"playing control is present"});
    assert.ok(label.includes("Bỏ qua chuyển động của Nếp"));
    await page.evaluate((name) => [...document.querySelectorAll('[role="button"]')].find(e => e.getAttribute("aria-label") === name)?.focus(), label);
    await key(page, "Enter", 13);
    await page.waitFor(() => ![...document.querySelectorAll('[role="button"]')].some(e => e.getAttribute("aria-label")?.includes("Bỏ qua chuyển động của Nếp")), { label: "skip completes the moment" });
    const image = await page.evaluate((name) => {
      const e = [...document.querySelectorAll('[role="img"]')].find(e => name.startsWith(e.getAttribute("aria-label")));
      return e && { label: e.getAttribute("aria-label"), interactive: !!e.closest('button,[role="button"],[tabindex="0"]') };
    }, label);
    assert.ok(image?.label);
    assert.equal(image.interactive, false);
    const tree = await page.call("Accessibility.getFullAXTree");
    assert.equal(tree.nodes.some(n => !n.ignored && n.role?.value === "button" && !n.name?.value?.trim()), false);
    await page.clickLabel("Đóng bảng");
    await page.waitFor(() => !document.querySelector('[data-testid="create-sheet"]'));
    assert.equal(await page.evaluate(() => globalThis.nepRequests.length), 0);
  } finally { await close(); }
});


// Production (4f74b011) has no demonstration catalogue: signed out is the
// sign-in door. The gutter (ExploreLive `cuonLoai`) is held on the live one.
test("the live catalogue keeps the same gutter before the tucked Nếp bookmark", async () => {
  const { page, url, close } = await setup(true);
  try {
    await page.viewport(320, 700);
    await page.goto(`${url}explore`, () => !!document.querySelector('[data-testid="nep-mep"]') && !document.querySelector('[data-testid="opening-app"]') && !document.querySelector('[inert]'));
    // The global bookmark mounts before the asynchronous catalogue.
    // Measure the category rail only once its actual category is rendered.
    await page.waitFor(() => [...document.querySelectorAll('[role="button"]')].some(e => e.textContent.trim() === "Cafe"), { label: "catalogue categories ready" });
    const gutter = await page.evaluate(() => {
      const chip = [...document.querySelectorAll('[role="button"]')].find(e => e.textContent.trim() === "Cafe");
      let rail = chip?.parentElement;
      while (rail && !["auto", "scroll"].includes(getComputedStyle(rail).overflowX)) rail = rail.parentElement;
      return { right: rail?.getBoundingClientRect().right, bookmarkLeft: document.querySelector('[data-testid="nep-mep"]').getBoundingClientRect().left };
    });
    assert.ok(gutter.right <= gutter.bookmarkLeft, `catalogue categories stay clear: ${JSON.stringify(gutter)}`);
  } finally { await close(); }
});
