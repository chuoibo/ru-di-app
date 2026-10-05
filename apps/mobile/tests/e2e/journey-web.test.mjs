import assert from "node:assert/strict";
import test from "node:test";
import { capture, enabled, openJourney, required } from "./journey-browser.mjs";

test("authenticated itinerary keeps URL, markers, selection and the edited day", { skip: !enabled && !required }, async () => {
  const journey = await openJourney();
  const { page } = journey;
  try {
    const path = await page.evaluate(() => location.pathname + location.hash);
    await page.clickLabel("Bản đồ");
    await page.waitFor(() => !!document.querySelector('[aria-label="Khớp hành trình"]'), { label: "fit control", timeout: 20000, diagnose: () => ({text:document.body.innerText.slice(-1600), reads:window.qaOutingReads}) });
    assert.equal(await page.evaluate(() => location.pathname + location.hash), path);
    await page.waitFor(() => !!document.querySelector(".maplibregl-canvas"), { label: "real map canvas", timeout: 20000 });
    await page.clickLabel("Mốc 1, 12:30, Điểm QA thứ nhất");
    await page.waitFor(() => document.querySelector('[data-testid="hanh-trinh-selected-stop"]')?.textContent === "Điểm QA thứ nhất", { label: "selected stop sheet" });
    await capture(page, "journey-map");
    await page.clickLabel("Lịch trình");
    await page.waitFor(() => document.querySelector('[data-testid="che-do-lich-trinh"]')?.getAttribute("aria-selected") === "true", { label: "timeline selected" });
    await page.waitFor(() => [...document.querySelectorAll('[role="button"]')].some(el => el.getClientRects().length > 0 && (el.checkVisibility?.({ opacityProperty: true, visibilityProperty: true }) ?? true) && (el.getAttribute("aria-label") ?? el.innerText ?? "").includes("Điểm QA thứ nhất") && el.getAttribute("aria-pressed") === "true"), { label: "selection survived view switch" });
    await page.clickLabel("Bản đồ");
    const openEditor = async () => {
      await page.evaluate(async () => {
        const button = [...document.querySelectorAll('[role="button"]')].find(el => el.textContent.trim() === "Sửa trang ngày");
        button.scrollIntoView({ block: "center" });
        await new Promise(resolve => {
          let timer;
          const settled = () => { document.removeEventListener("scroll", onScroll, true); resolve(); };
          const onScroll = () => { clearTimeout(timer); timer = setTimeout(settled, 160); };
          document.addEventListener("scroll", onScroll, true); onScroll();
        });
      });
      await page.clickChu("Sửa trang ngày");
      await page.waitFor(() => !!document.querySelector('input[aria-label="Giờ xuất phát"]'), { label: "day editor" });
    };
    await openEditor();
    await page.clickLabel("Giờ xuất phát");
    await page.call("Input.dispatchKeyEvent", { type: "rawKeyDown", key: "a", code: "KeyA", windowsVirtualKeyCode: 65, modifiers: 2 });
    await page.call("Input.dispatchKeyEvent", { type: "keyUp", key: "a", code: "KeyA", windowsVirtualKeyCode: 65, modifiers: 2 });
    await page.call("Input.insertText", { text: "07:45" });
    await page.clickChu("Xem trên bản đồ");
    await page.waitFor(() => !document.querySelector('input[aria-label="Giờ xuất phát"]'), { label: "day editor closed" });
    await page.clickLabel("Lịch trình"); await page.clickLabel("Bản đồ"); await openEditor();
    assert.equal(await page.evaluate(() => document.querySelector('input[aria-label="Giờ xuất phát"]')?.value), "07:45");
    await capture(page, "journey-day-editor");
  } catch (error) { await capture(page, "journey-web-failure"); throw error; }
  finally { await journey.close(); }
});
