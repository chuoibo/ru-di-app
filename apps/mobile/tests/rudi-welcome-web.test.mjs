/* UI-016/017/020: exercise the exported onboarding with real input. */
import assert from "node:assert/strict";
import test from "node:test";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { findChrome, launch, serve } from "./chrome-cdp.mjs";
import { lyDoBanDungCu } from "./tuoi-ban-dung.mjs";

const root = fileURLToPath(new URL("..", import.meta.url));
const exported = process.env.MOBILE_WEB_EXPORT ?? join(root, ".expo-build-check");

test("Welcome keeps a fast swipe to one chapter, exposes its controls and preserves the chapter on resize", async () => {
  assert.equal(lyDoBanDungCu(exported, root), null);
  const chrome = findChrome();
  assert.ok(chrome, "Chrome is required for Welcome input regression");
  const server = await serve(exported);
  const page = await launch(chrome);
  const current = async (number) => page.waitFor((n) => {
    const pager = document.querySelector('[data-testid="welcome-pager"]');
    const selected = document.querySelector('[aria-pressed="true"][aria-label^="Trang "]');
    return pager?.getAttribute("aria-label") === `Giới thiệu Rủ Đi, trang ${n} trên 4`
      && selected?.getAttribute("aria-label").startsWith(`Trang ${n}:`)
      && Math.abs(pager.scrollLeft - (n - 1) * pager.clientWidth) < 1;
  }, { label: `chapter ${number} settled` }, number);
  const key = async (name, code) => {
    await page.call("Input.dispatchKeyEvent", { type: "rawKeyDown", key: name, code: name, windowsVirtualKeyCode: code });
    await page.call("Input.dispatchKeyEvent", { type: "keyUp", key: name, code: name, windowsVirtualKeyCode: code });
  };
  try {
    await page.viewport(390, 844);
    await page.call("Emulation.setTouchEmulationEnabled", { enabled: true });
    await page.goto(`${server.url}welcome`, () => !!document.querySelector('[data-testid="welcome-pager"]'));
    await current(1);
    const box = await page.evaluate(() => {
      const r = document.querySelector('[data-testid="welcome-pager"]').getBoundingClientRect();
      return { x: r.x, y: r.y, w: r.width, h: r.height };
    });
    const point = { x: box.x + box.w * 0.85, y: box.y + box.h * 0.55 };
    await page.call("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [point] });
    // The interval is the gesture velocity, not a sleep to make a render pass.
    for (let i = 1; i <= 8; i++) {
      await new Promise(resolve => setTimeout(resolve, 16));
      await page.call("Input.dispatchTouchEvent", { type: "touchMove", touchPoints: [{ ...point, x: point.x - box.w * 0.7 * i / 8 }] });
    }
    await page.call("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
    await current(2);
    const tree = await page.call("Accessibility.getFullAXTree");
    assert.equal(tree.nodes.some(n => !n.ignored && n.name?.value === "Một lời rủ. Nhiều ngày đáng nhớ."), false, "offscreen chapters must not be read together");
    const targets = await page.evaluate(() => [...document.querySelectorAll('[aria-label^="Trang "]')].map(e => {
      const r = e.getBoundingClientRect(); return { w: r.width, h: r.height, role: e.getAttribute("role") };
    }));
    assert.equal(targets.length, 4);
    assert.ok(targets.every(t => t.role === "button" && t.w >= 48 && t.h >= 48));
    await page.clickLabel("Giới thiệu Rủ Đi, trang 2 trên 4");
    await key("End", 35); await current(4);
    await key("Home", 36); await current(1);
    await key("ArrowRight", 39); await current(2);
    await page.viewport(768, 1024); await current(2);
    await page.clickLabel("Trang 4: Đi rồi, còn điều để nhớ"); await current(4);
    await page.clickLabel("Xem lại từ đầu"); await current(1);
    await page.call("Emulation.setEmulatedMedia", { features: [{ name: "prefers-reduced-motion", value: "reduce" }] });
    await page.clickLabel("Trang 3: Vui cùng nhau, rõ phần mỗi người"); await current(3);
    await page.clickLabel("Rủ Đi thôi!");
    await page.waitFor(() => location.pathname === "/login" && document.body.innerText.includes("Chào bạn"), { label: "cover opens Login" });
  } finally { await page.close(); await server.close(); }
});
