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

    async function openEditor() {
      // The notebook scrolls independently under a fixed primary action.
      // Let its scroll settle before a real pointer press; a press during a
      // scroll is cancelled by the scroll responder, just like on a phone.
      await page.evaluate(async () => {
        const button = [...document.querySelectorAll('[role="button"]')].find((el) => el.textContent.trim() === "Sửa trang ngày");
        button.scrollIntoView({ block: "center" });
        await new Promise((resolve) => {
          let timer;
          const settled = () => { document.removeEventListener("scroll", onScroll, true); resolve(); };
          const onScroll = () => { clearTimeout(timer); timer = setTimeout(settled, 160); };
          document.addEventListener("scroll", onScroll, true);
          onScroll();
        });
      });
      await page.clickChu("Sửa trang ngày");
      await page.waitFor(() => !!document.querySelector('input[aria-label="Giờ xuất phát"]'), { timeout: 5000, label: "editor mounted" });
    }

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
      // The rail is the accessible control now: pins are MapLibre markers and
      // carry no name. Its label is «Mốc N, giờ, tên chặng».
      await page.clickLabel("Mốc 1, 12:30, Ăn trưa - Bánh căn Lệ");
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
        // Both views remain mounted to preserve the draft and undo; assert the
        // visible timeline row, never a hidden map marker or rail control.
        const nut = [...document.querySelectorAll('[role="button"]')].find((el) => el.getClientRects().length > 0 && (el.getAttribute("aria-label") ?? el.innerText ?? "").includes("Ăn trưa - Bánh căn Lệ"));
        return nut ? nut.getAttribute("aria-selected") : null;
      });
      assert.equal(chon, "true", `chặng đã chọn phải còn highlight khi về Lịch trình, nhận ${chon}`);
      await page.clickLabel("Hành trình");
      await openEditor();
      await page.evaluate(() => {
        const input = document.querySelector('input[aria-label="Giờ xuất phát"]');
        if (!input) throw new Error("Missing day editor");
        Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value").set.call(input, "07:45");
        input.dispatchEvent(new Event("input", { bubbles: true }));
      });
      await page.clickChu("Xem trên bản đồ");
      await page.waitFor(() => !document.querySelector('input[aria-label="Giờ xuất phát"]'), { timeout: 5000, label: "editor closed" });
      await page.clickLabel("Lịch trình");
      await page.clickLabel("Hành trình");
      await openEditor();
      assert.equal(await page.evaluate(() => document.querySelector('input[aria-label="Giờ xuất phát"]')?.value), "07:45", "bản nháp phải còn sau khi đổi chế độ");
      await page.clickChu("Xem trên bản đồ");
      await page.waitFor(() => !document.querySelector('input[aria-label="Giờ xuất phát"]'));
      const center = await page.evaluate(() => {
        const canvas = document.querySelector(".maplibregl-canvas");
        canvas.scrollIntoView({ block: "center" });
        const box = canvas.getBoundingClientRect();
        return { x: box.x + box.width / 2, y: box.y + box.height / 2 };
      });
      await page.call("Input.dispatchMouseEvent", { type: "mouseWheel", ...center, deltaX: 0, deltaY: 800 });
      await page.waitFor(() => [...document.querySelectorAll('[aria-label]')].some((el) => el.getAttribute("aria-label").includes("điểm gần nhau:")), { timeout: 10000, label: "cụm điểm sau khi thu nhỏ bản đồ" });
      const cluster = await page.evaluate(() => [...document.querySelectorAll('[aria-label]')].find((el) => el.getAttribute("aria-label").includes("điểm gần nhau:")).getAttribute("aria-label"));
      await page.clickLabel(cluster);
      await page.waitFor(() => !!document.querySelector('[aria-label="Chọn điểm hẹn gần nhau"]'));
      await page.clickChu("3 · 20:00 · Chợ đêm Đà Lạt");
      await page.waitFor(() => document.querySelector('[data-testid="hanh-trinh-selected-stop"]')?.textContent === "Chợ đêm Đà Lạt" && !document.querySelector('[aria-label="Chọn điểm hẹn gần nhau"]'), { label: "chi tiết đúng điểm chọn từ cụm" });
      await page.clickLabel("Lịch trình");
      await page.waitFor(() => [...document.querySelectorAll('[role="button"]')].some((el) => el.getClientRects().length > 0 && (el.getAttribute("aria-label") ?? el.innerText ?? "").includes("Chợ đêm Đà Lạt") && el.getAttribute("aria-selected") === "true"), { label: "điểm chọn từ cụm được giữ ở timeline" });
    });
  });
}
