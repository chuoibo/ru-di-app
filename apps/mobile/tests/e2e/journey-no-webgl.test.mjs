import assert from "node:assert/strict";
import test from "node:test";
import { capture, enabled, openJourney, required } from "./journey-browser.mjs";

test("authenticated itinerary remains usable without WebGL2", { skip: !enabled && !required }, async () => {
  const journey = await openJourney({ webgl2: false });
  const { page } = journey;
  try {
    assert.equal(await page.evaluate(() => !!document.createElement("canvas").getContext("webgl2")), false);
    await page.clickLabel("Bản đồ");
    await page.waitFor(() => !!document.querySelector('[aria-label="Bản đồ không khả dụng"]'), { label: "no WebGL fallback" });
    assert.equal(await page.evaluate(() => document.body.innerText.includes("Các chặng trong ngày")), true);
    await page.clickLabel("Mốc 1, 12:30, Điểm QA thứ nhất");
    await page.waitFor(() => document.querySelector('[data-testid="hanh-trinh-selected-stop"]')?.textContent === "Điểm QA thứ nhất", { label: "stop selectable without map" });
    await capture(page, "journey-no-webgl");
  } catch (error) { await capture(page, "journey-no-webgl-failure"); throw error; }
  finally { await journey.close(); }
});
