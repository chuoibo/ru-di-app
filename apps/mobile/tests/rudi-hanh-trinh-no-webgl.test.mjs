/* The trip notebook must remain usable when a browser has no WebGL2. */
import assert from "node:assert/strict";
import test from "node:test";
import { existsSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

import { findChrome, launch, serve } from "./chrome-cdp.mjs";

const ROOT = fileURLToPath(new URL("..", import.meta.url));
const EXPORT_DIR = process.env.MOBILE_WEB_EXPORT ?? join(ROOT, ".expo-build-check");
const required = process.env.MOBILE_REQUIRE_HANH_TRINH_WEB === "1";

test("/plan stays usable without WebGL2", { skip: !existsSync(join(EXPORT_DIR, "index.html")) || (!findChrome() && !required) }, async () => {
  const server = await serve(EXPORT_DIR);
  // Ask for a browser without WebGL2 instead of assuming this machine's Chrome
  // is one: the CI runner's software fallback provides it (see launch).
  const page = await launch(findChrome(), { webgl2: false });
  try {
    await page.viewport(390, 844);
    await page.goto(`${server.url}plan`, () => document.body != null);
    assert.equal(await page.evaluate(() => !!document.createElement("canvas").getContext("webgl2")), false);
    await page.waitFor(() => document.body?.innerText?.includes("Lịch trình"), { timeout: 25000, label: "Lịch trình trên /plan" });
    await page.clickLabel("Bản đồ");
    await page.waitFor(() => document.body?.innerText?.includes("Các chặng trong ngày"), { label: "trang ngày không bị trắng" });
    assert.equal(await page.evaluate(() => !!document.querySelector('[aria-label="Bản đồ không khả dụng"]')), true);
    assert.equal(await page.evaluate(() => document.body?.innerText?.includes("Ăn trưa - Bánh căn Lệ")), true);
    await page.clickLabel("Mốc 1, 12:30, Ăn trưa - Bánh căn Lệ");
    await page.waitFor(() => document.querySelector('[data-testid="hanh-trinh-selected-stop"]')?.textContent === "Ăn trưa - Bánh căn Lệ", { label: "chọn mốc khi không có bản đồ" });
  } finally {
    await page.close();
    await server.close();
  }
});
