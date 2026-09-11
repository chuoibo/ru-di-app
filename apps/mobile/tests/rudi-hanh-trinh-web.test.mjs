/* Journey map on the web export: a second view of /plan, not a new itinerary.
 *
 * Run from apps/mobile after `npm run build:check`:
 *     node --test tests/rudi-hanh-trinh-web.test.mjs
 *
 * What must stay true: switching Lịch trình | Hành trình does not change the
 * URL; the map canvas and numbered markers appear; Fit Journey is on screen;
 * tapping a marker opens a sheet with the stop name; selection survives the
 * switch back. Chrome is required when MOBILE_REQUIRE_HANH_TRINH_WEB=1.
 */
import assert from "node:assert/strict";
import test, { after, before, describe } from "node:test";
import { existsSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

import { findChrome, launch, serve } from "./chrome-cdp.mjs";
import { lyDoBanDungCu } from "./tuoi-ban-dung.mjs";

const ROOT = fileURLToPath(new URL("..", import.meta.url));
const EXPORT_DIR = process.env.MOBILE_WEB_EXPORT ?? join(ROOT, ".expo-build-check");
const chromeBin = findChrome();
const REQUIRED = process.env.MOBILE_REQUIRE_HANH_TRINH_WEB === "1";
const INDEX = join(EXPORT_DIR, "index.html");

function boQua(ly) {
  test(`hành trình trên web — BỎ QUA: ${ly}`, { skip: ly }, () => {});
}

if (!existsSync(INDEX)) {
  boQua("chưa có bản web — npm run build:check");
} else if (!chromeBin && !REQUIRED) {
  boQua("không tìm thấy Chrome");
} else {
  describe("hành trình là chế độ xem thứ hai của lịch trình", () => {
    let page;
    let server;

    before(async () => {
      assert.ok(chromeBin, "MOBILE_REQUIRE_HANH_TRINH_WEB=1 nhưng không tìm thấy Chrome");
      const cu = lyDoBanDungCu(EXPORT_DIR, ROOT);
      assert.equal(cu, null, cu);
      server = await serve(EXPORT_DIR);
      page = await launch(chromeBin);
      await page.viewport(390, 844);
    });

    after(async () => {
      if (page) await page.close();
      if (server) await server.close();
    });

    test("Lịch trình → Hành trình không đổi URL; marker, sheet, khớp, selected còn", async () => {
      await page.goto(`${server.url}plan`, () => document.body != null);
      await page.waitFor(
        () => document.body?.innerText?.includes("Lịch trình"),
        { timeout: 25000, label: "Lịch trình trên /plan" },
      );
      const urlTruoc = await page.evaluate(() => location.pathname + location.hash);
      assert.match(urlTruoc, /plan/, `mong /plan, nhận ${urlTruoc}`);

      await page.clickLabel("Hành trình");
      await page.waitFor(
        () => document.body?.innerText?.includes("Khớp hành trình"),
        { timeout: 20000, label: "nút Khớp hành trình" },
      );
      const urlSau = await page.evaluate(() => location.pathname + location.hash);
      assert.equal(urlSau, urlTruoc, `đổi chế độ không được đổi route (${urlTruoc} → ${urlSau})`);

      const coMap = await page.waitFor(
        () => !!(document.querySelector("canvas") || document.querySelector(".maplibregl-map") || document.getElementById("ban-do-hanh-trinh")),
        { timeout: 20000, label: "canvas bản đồ" },
      ).then(() => true);
      assert.equal(coMap, true);

      await page.waitFor(
        () =>
          [...document.querySelectorAll("[aria-label]")].some((el) =>
            (el.getAttribute("aria-label") ?? "").includes("Ăn trưa - Bánh căn Lệ"),
          ),
        { timeout: 15000, label: "marker Bánh căn Lệ" },
      );
      await page.clickLabel("Ăn trưa - Bánh căn Lệ");
      await page.waitFor(
        () => document.body?.innerText?.includes("Xem chi tiết") || document.body?.innerText?.includes("Ăn trưa - Bánh căn Lệ"),
        { timeout: 10000, label: "sheet chặng" },
      );

      const khop = await page.evaluate(() => document.body?.innerText?.includes("Khớp hành trình"));
      assert.equal(khop, true);

      await page.clickLabel("Lịch trình");
      await page.waitFor(
        () => document.body?.innerText?.includes("Ăn trưa - Bánh căn Lệ") && document.body?.innerText?.includes("Lịch trình"),
        { timeout: 10000, label: "về lịch trình" },
      );
      const chon = await page.evaluate(() => {
        const nut = [...document.querySelectorAll('[role="button"]')].find((el) => (el.getAttribute("aria-label") ?? el.innerText ?? "").includes("Ăn trưa - Bánh căn Lệ"));
        return nut ? nut.getAttribute("aria-selected") : null;
      });
      assert.equal(chon, "true", `chặng đã chọn phải còn highlight khi về Lịch trình, nhận ${chon}`);
    });
  });
}
