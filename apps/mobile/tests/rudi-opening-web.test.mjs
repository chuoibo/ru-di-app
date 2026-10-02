/* QA UI-002/009: the exported shell, with only its session transport held.
 * The frame budget starts when restoration requests a session, not at Chrome
 * startup or Metro bundling. This measures browser paint opportunity, not
 * Android launch time, TalkBack, or an independent QA acceptance.
 */
import assert from "node:assert/strict";
import test from "node:test";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { findChrome, launch, serve } from "./chrome-cdp.mjs";
import { lyDoBanDungCu } from "./tuoi-ban-dung.mjs";

const root = fileURLToPath(new URL("..", import.meta.url));
const exported = process.env.MOBILE_WEB_EXPORT ?? join(root, ".expo-build-check");

test("slow restore paints a protected cover within 300ms, then an unknown link preserves the signed-in door", async () => {
  assert.equal(lyDoBanDungCu(exported, root), null);
  const chrome = findChrome();
  assert.ok(chrome, "Chrome is required for the opening-frame regression");
  const server = await serve(exported);
  const page = await launch(chrome);
  try {
    await page.viewport(390, 844);
    await page.call("Page.addScriptToEvaluateOnNewDocument", { source: `
      const realFetch = globalThis.fetch;
      globalThis.openingProbe = { frame: null };
      globalThis.fetch = async (input, init) => {
        const url = String(input);
        if (url.endsWith('/sessions/web/resume')) {
          const start = performance.now();
          const checkFrame = () => {
            const cover = [...document.querySelectorAll('[data-testid="opening-app"]')].find(node => !node.closest('[inert]'));
            const bounds = cover?.getBoundingClientRect();
            if (bounds?.width > 0 && bounds?.height > 0 && !cover.closest('[inert]')) {
              const scene = document.querySelector('[inert]');
              openingProbe.frame = { elapsed: performance.now() - start, width: bounds.width, height: bounds.height,
                protected: !!scene && scene.getAttribute('aria-hidden') === 'true' };
            } else requestAnimationFrame(checkFrame);
          };
          requestAnimationFrame(checkFrame);
          await new Promise(resolve => setTimeout(resolve, 5000));
          return new Response(JSON.stringify({ token: 'synthetic-test-session', person_id: 'synthetic-person',
            expires_at: '2999-01-01T00:00:00Z', issued_via: 'otp', profile: { display_name: 'Người thử tổng hợp' } }),
            {status:200,headers:{'Content-Type':'application/json'}});
        }
        if (url.endsWith('/people/me/contexts')) return new Response(JSON.stringify({contexts:[]}),{status:200});
        if (url.includes('api.build-check.invalid')) return new Response('{}',{status:404});
        return realFetch(input,init);
      };
    ` });
    await page.goto(`${server.url}duong-dan-khong-co`, () => !!globalThis.openingProbe?.frame);
    const frame = await page.evaluate(() => globalThis.openingProbe.frame);
    assert.ok(frame.elapsed <= 300, `opening frame took ${frame.elapsed.toFixed(1)}ms`);
    assert.equal(frame.width, 390);
    assert.equal(frame.height, 844);
    assert.equal(frame.protected, true, "the mounted scene must be hidden and inert during restoration");
    await page.waitFor(() => document.body.innerText.includes("Trang này chưa có trong Rủ Đi"), { timeout: 15000, label: "recovery screen after restore" });
    await page.waitFor(() => document.activeElement?.textContent === "Không tìm thấy trang", { label: "recovery title owns focus" });
    assert.equal(await page.evaluate(() => document.querySelectorAll('[inert]').length), 0);
    await page.clickChu("Về Rủ Đi");
    await page.waitFor(() => location.pathname === "/messages", { label: "signed-in home without a group, preserving the session" });
    assert.equal(await page.evaluate(() => !!document.querySelector('[data-testid="welcome-screen"]')), false);
    console.log(`Opening frame ${frame.elapsed.toFixed(1)}ms; mounted scene protected; signed-in recovery /messages`);
  } finally {
    await page.close();
    await server.close();
  }
});
