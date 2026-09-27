/* F02, Khám phá: filters, search, the AI door without a key, saving a place,
 * the end of the list under the tab bar, failure and offline, changing city,
 * the place detail page and its actions, and the stage pop-up.
 *
 *   node kich-ban/f02-kham-pha.mjs [--chi loc,tim,ai,tim-luu,cuoi,loi,diem-den,chi-tiet,bat-san,cat-chu]
 */
import { join } from "node:path";

import { cauHinh } from "../thu-vien/cau-hinh.mjs";
import { chup } from "../thu-vien/chup.mjs";
import { quayKhung, ghepKhung } from "../thu-vien/chuyen-dong.mjs";
import { cdpCua, cham, keo, tamCua } from "../thu-vien/cu-chi.mjs";
import { choOn, duongDan } from "../thu-vien/dieu-huong.mjs";
import { goHet, loiMayChu, ngatMang } from "../thu-vien/mang.mjs";
import { khoiDong, trangMoi } from "../thu-vien/moi-truong.mjs";
import { personaTheoTen } from "../thu-vien/phien.mjs";
import { soGhi } from "../thu-vien/ghi.mjs";

const chi = (() => {
  const i = process.argv.indexOf("--chi");
  return i === -1 ? null : new Set(process.argv[i + 1].split(","));
})();
const chay = (ten) => !chi || chi.has(ten);
const mt = await khoiDong();
const so = soGhi(mt.out);
const ket = (rec) => {
  so.ghi({ feature: "F02", nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, ...rec });
  console.log(`${rec.status.padEnd(10)} ${rec.tc} ${rec.cauHinh ?? ""} ${rec.ghiChu ?? ""}`);
};
const log = (o) => console.log(JSON.stringify(o));
const P = personaTheoTen("dalat-0", mt.chatSessions);
const moKhamPha = (cfg) => trangMoi(mt, cauHinh(cfg), { persona: P, path: "/explore" });
const tieuDeDanhSach = (page) =>
  page.evaluate(() => {
    const e = [...document.querySelectorAll("div[dir],span")].find((x) => /^\d+ (kết quả|nơi ở .+)$/.test((x.innerText ?? "").trim()));
    if (!e) return null;
    const r = e.getBoundingClientRect();
    return { chu: e.innerText.trim(), top: Math.round(r.top), trongMan: r.top >= 0 && r.bottom <= innerHeight };
  });
const cao = (page, sel) => page.evaluate((s) => Math.round(document.querySelector(s)?.getBoundingClientRect().height ?? -1), sel);

try {
  // ------------------------------------------------------------- filter chip
  if (chay("loc")) {
    for (const cfg of ["C1", "C2"]) {
      const t = await moKhamPha(cfg);
      const cdp = await cdpCua(t.page);
      const truoc = { san: await cao(t.page, '[data-testid="san-thanh-pho"]'), tieuDe: await tieuDeDanhSach(t.page) };
      const chip = await t.page.evaluate(() => {
        const e = [...document.querySelectorAll('[role="button"],button')].find((x) => (x.innerText ?? "").trim() === "Cafe");
        if (!e) return null;
        const r = e.getBoundingClientRect();
        return { x: r.left + r.width / 2, y: r.top + r.height / 2, sel: e.getAttribute("aria-selected"), pressed: e.getAttribute("aria-pressed") };
      });
      await cham(cdp, chip);
      await choOn(t.page, { mang: t.mang });
      await t.page.waitForTimeout(700);
      const sau = { san: await cao(t.page, '[data-testid="san-thanh-pho"]'), tieuDe: await tieuDeDanhSach(t.page) };
      const chipSau = await t.page.evaluate(() => {
        const e = [...document.querySelectorAll('[role="button"],button')].find((x) => (x.innerText ?? "").trim() === "Cafe");
        return { sel: e?.getAttribute("aria-selected"), pressed: e?.getAttribute("aria-pressed"), checked: e?.getAttribute("aria-checked") };
      });
      const id = `EV-F02-LOC-cafe-${cfg}`;
      await chup(t.page, { out: mt.out, id, suKien: t.suKien });
      log({ cfg, truoc, sau, chipSau });
      ket({ tc: "TC-F02-LOC", screen: "F02.S01", state: "danh sách Đà Lạt", action: "chạm chip «Cafe»", cauHinh: cfg, expected: "danh sách lọc; sân khấu gập; dòng «N kết quả» nằm trong màn; chip báo đã chọn", status: sau.tieuDe && /kết quả/.test(sau.tieuDe.chu) && sau.tieuDe.trongMan ? "PASS" : "FAIL", evidence: [id], ghiChu: `sân khấu ${truoc.san}→${sau.san}dp; «${sau.tieuDe?.chu}» top ${sau.tieuDe?.top} trong màn: ${sau.tieuDe?.trongMan}; chip aria-selected ${chipSau.sel}, aria-pressed ${chipSau.pressed}` });
      await t.context.close();
    }
  }

  // ------------------------------------------------------------------ search
  if (chay("tim")) {
    const t = await moKhamPha("C1");
    const o = t.page.locator('input[aria-label="Ô tìm địa điểm"]');
    await o.fill("lẩu");
    await t.page.keyboard.press("Enter");
    await choOn(t.page, { mang: t.mang });
    await t.page.waitForTimeout(800);
    const coKq = await tieuDeDanhSach(t.page);
    await chup(t.page, { out: mt.out, id: "EV-F02-TIM-lau-C1", suKien: t.suKien });
    await o.fill("zzqqxx khong co quan nao");
    await t.page.keyboard.press("Enter");
    await choOn(t.page, { mang: t.mang });
    await t.page.waitForTimeout(800);
    const khongKq = await tieuDeDanhSach(t.page);
    const chuRong = await t.page.evaluate(() => (document.querySelector('[data-testid="explore-screen"]')?.innerText ?? "").split("\n").filter((l) => /không|chưa|thử/i.test(l)).slice(0, 6));
    const m = await chup(t.page, { out: mt.out, id: "EV-F02-TIM-rong-C1", suKien: t.suKien });
    log({ coKq, khongKq, chuRong });
    ket({ tc: "TC-F02-TIM", screen: "F02.S01", state: "gõ «lẩu» rồi một câu không khớp", action: "Enter", cauHinh: "C1", expected: "có kết quả thì đếm đúng; không kết quả thì trạng thái rỗng nói cách thoát (xoá lọc)", status: khongKq && m.tomTat.maLoi === 0 ? "PASS" : "FAIL", evidence: ["EV-F02-TIM-lau-C1", "EV-F02-TIM-rong-C1"], ghiChu: `«lẩu»: ${coKq?.chu}; không khớp: ${khongKq?.chu}; câu hiện: ${chuRong.join(" | ")}` });
    await t.context.close();
  }

  // ------------------------------------------------------ AI door, no key
  // Two cases, because the ✦ button only drops a sample into the field
  // (ExploreLive `setQuery(CAU_MAU)`): what the screen says right after the
  // tap, and what it says once the question is really sent from the field.
  if (chay("ai")) {
    const t = await moKhamPha("C1");
    const cdp = await cdpCua(t.page);
    await cham(cdp, await tamCua(t.page, '[aria-label="Hỏi Rủ Đi AI"]'));
    await t.page.waitForTimeout(900);
    const sauCham = await t.page.evaluate(() => {
      const o = document.querySelector('input[aria-label="Ô tìm địa điểm"]');
      const a = document.activeElement;
      return {
        o: o?.value ?? null,
        focusVaoO: a === o,
        focus: a ? (a.getAttribute("aria-label") || a.tagName) : null,
        placeholder: o?.getAttribute("placeholder") ?? null,
        placeholderCat: o ? o.scrollWidth > o.clientWidth + 1 : null,
      };
    });
    const tieuDe1 = await tieuDeDanhSach(t.page);
    const chu1 = await t.page.evaluate(() => (document.querySelector('[data-testid="explore-screen"]')?.innerText ?? "").split("\n").filter((l) => /Chưa thấy|Thử từ khóa|Xóa lọc|Rủ Đi AI/.test(l)).slice(0, 5));
    await chup(t.page, { out: mt.out, id: "EV-F02-AI-MAU-C1", suKien: t.suKien });
    log({ sauCham, tieuDe1, chu1 });
    ket({ tc: "TC-F02-AI-MAU", screen: "F02.S01", state: "danh sách Đà Lạt, máy chủ không có khoá AI", action: "chạm ✦ «Hỏi Rủ Đi AI»", cauHinh: "C1", expected: "câu mẫu vào ô và sẵn để gửi (focus vào ô), danh sách chưa báo thất bại khi chưa hỏi", status: sauCham.o && !/^0 /.test(tieuDe1?.chu ?? "") ? "PASS" : "FAIL", evidence: ["EV-F02-AI-MAU-C1"], ghiChu: `ô: «${sauCham.o}»; focus: ${sauCham.focus} (vào ô: ${sauCham.focusVaoO}); danh sách: «${tieuDe1?.chu}»; câu: ${chu1.join(" | ")}` });

    // Now send it the way a person would: tap the field, then Enter.
    const o = t.page.locator('input[aria-label="Ô tìm địa điểm"]');
    const oHop = await o.boundingBox();
    await cham(cdp, { x: oHop.x + oHop.width / 2, y: oHop.y + oHop.height / 2 });
    const phanHoi = t.page.waitForResponse((r) => /\/places\/search/.test(r.url()), { timeout: 20_000 }).catch(() => null);
    await t.page.keyboard.press("Enter");
    const r = await phanHoi;
    const than = r ? await r.json().catch(() => null) : null;
    await choOn(t.page, { mang: t.mang, toiDa: 20_000 });
    await t.page.waitForTimeout(800);
    const tieuDe2 = await tieuDeDanhSach(t.page);
    const chu2 = await t.page.evaluate(() => (document.querySelector('[data-testid="explore-screen"]')?.innerText ?? "").split("\n").filter((l) => /Chưa thấy|Thử|Rủ Đi AI|chưa|không/i.test(l)).slice(0, 6));
    const m = await chup(t.page, { out: mt.out, id: "EV-F02-AI-HOI-C1", suKien: t.suKien });
    log({ http: r ? r.status() : null, source: than?.source ?? null, soNoi: Array.isArray(than?.places) ? than.places.length : null, tieuDe2, chu2 });
    ket({ tc: "TC-F02-AI-HOI", screen: "F02.S01", state: "câu mẫu trong ô, máy chủ không có khoá AI", action: "chạm ô, Enter", cauHinh: "C1", expected: "kết quả, hoặc một câu tiếng Việt nói Rủ Đi AI chưa sẵn sàng; không mã lỗi", status: r && m.tomTat.maLoi === 0 ? "PASS" : "FAIL", evidence: ["EV-F02-AI-HOI-C1"], ghiChu: `POST /places/search → ${r ? r.status() : "không gửi"}, source ${than?.source ?? "?"}, ${Array.isArray(than?.places) ? than.places.length : "?"} nơi; danh sách «${tieuDe2?.chu}»; câu: ${chu2.join(" | ")}` });
    await t.context.close();
  }

  // ------------------------------------------------------------- save heart
  if (chay("tim-luu")) {
    const t = await moKhamPha("C1");
    const cdp = await cdpCua(t.page);
    const tim = await tamCua(t.page, '[aria-label="Lưu Sống Màu Workshop"],[aria-label="Bỏ lưu Sống Màu Workshop"]');
    const nhan0 = await t.page.evaluate(() => document.querySelector('[aria-label$="Sống Màu Workshop"][role="button"][aria-label^="Lưu"],[aria-label^="Bỏ lưu Sống Màu"]')?.getAttribute("aria-label"));
    await cham(cdp, tim);
    await t.page.waitForTimeout(1200);
    const nhan1 = await t.page.evaluate(() => [...document.querySelectorAll('[aria-label*="Sống Màu Workshop"]')].map((e) => e.getAttribute("aria-label")).filter((x) => /lưu/i.test(x)));
    await chup(t.page, { out: mt.out, id: "EV-F02-LUU-C1", suKien: t.suKien });
    await cham(cdp, await tamCua(t.page, '[aria-label="Lưu Sống Màu Workshop"],[aria-label="Bỏ lưu Sống Màu Workshop"]'));
    await t.page.waitForTimeout(1200);
    const nhan2 = await t.page.evaluate(() => [...document.querySelectorAll('[aria-label*="Sống Màu Workshop"]')].map((e) => e.getAttribute("aria-label")).filter((x) => /lưu/i.test(x)));
    log({ nhan0, nhan1, nhan2 });
    ket({ tc: "TC-F02-LUU", screen: "F02.S01", state: "quán chưa lưu", action: "chạm tim, chạm lại", cauHinh: "C1", expected: "trạng thái lưu đổi và nhãn truy cập đổi theo; chạm lại thì về như cũ", status: nhan1.some((x) => x.startsWith("Bỏ lưu")) && nhan2.every((x) => x.startsWith("Lưu")) ? "PASS" : "FAIL", evidence: ["EV-F02-LUU-C1"], ghiChu: `${nhan0} → ${nhan1.join("/")} → ${nhan2.join("/")}` });
    await t.context.close();
  }

  // ------------------------------------------------ end of list vs tab bar
  if (chay("cuoi")) {
    for (const cfg of ["C1", "C2"]) {
      const t = await moKhamPha(cfg);
      const kq = await t.page.evaluate(async () => {
        const sv = [...document.querySelectorAll("div")].filter((e) => { const s = getComputedStyle(e); return (s.overflowY === "auto" || s.overflowY === "scroll") && e.scrollHeight > e.clientHeight + 20; }).sort((a, b) => b.clientHeight - a.clientHeight)[0];
        if (!sv) return null;
        sv.scrollTop = sv.scrollHeight;
        await new Promise((ok) => setTimeout(ok, 500));
        const hang = [...document.querySelectorAll('[aria-label^="Mở "]')].filter((e) => e.getBoundingClientRect().height > 0);
        const cuoi = hang[hang.length - 1];
        const r = cuoi.getBoundingClientRect();
        const bar = document.querySelector('[role="tab"]')?.parentElement?.getBoundingClientRect();
        const fab = document.querySelector('[aria-label="Tạo mới"]')?.getBoundingClientRect();
        return { ten: cuoi.getAttribute("aria-label"), day: Math.round(r.bottom), barTop: bar ? Math.round(bar.top) : null, fabTop: fab ? Math.round(fab.top) : null, khoangTrong: bar ? Math.round(bar.top - r.bottom) : null };
      });
      const id = `EV-F02-CUOI-${cfg}`;
      await chup(t.page, { out: mt.out, id, suKien: t.suKien });
      log({ cfg, kq });
      ket({ tc: "TC-F02-CUOI", screen: "F02.S01", state: "cuộn hết danh sách", action: "cuộn xuống đáy", cauHinh: cfg, expected: "hàng cuối nằm trên thanh tab và nút «+», không bị che", status: kq && kq.day <= Math.min(kq.barTop, kq.fabTop ?? 1e9) ? "PASS" : "FAIL", evidence: [id], ghiChu: kq ? `hàng cuối «${kq.ten}» đáy ${kq.day}; thanh tab ${kq.barTop}; nút + ${kq.fabTop}` : "không thấy vùng cuộn" });
      await t.context.close();
    }
  }

  // ------------------------------------------------ failure, offline, retry
  if (chay("loi")) {
    const t = await trangMoi(mt, cauHinh("C1"), { persona: P, path: "/profile" });
    // A fresh context has no remembered city, so the list read is a bare
    // `/places` (no query string); `/places/<id>` must stay untouched.
    await loiMayChu(t.page, /\/places(\?|$)/, 503);
    const cdp = await cdpCua(t.page);
    await cham(cdp, await tamCua(t.page, '[role="tab"][aria-label="Khám phá"]', { cuon: false }));
    await choOn(t.page, { mang: t.mang });
    await t.page.waitForTimeout(800);
    // The retry is a RudiButton: its name is its text, there is no aria-label.
    const nutThuLai = await t.page.evaluate(() => {
      const e = [...document.querySelectorAll('[role="button"],button')].find((x) => (x.innerText ?? "").trim() === "Thử lại");
      if (!e) return null;
      e.scrollIntoView({ block: "center", behavior: "instant" });
      const r = e.getBoundingClientRect();
      return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
    });
    const chuLoi = await t.page.evaluate(() => (document.querySelector('[data-testid="explore-screen"]')?.innerText ?? "").split("\n").filter((l) => /chưa|thử|mạng/i.test(l)).slice(0, 4));
    const m1 = await chup(t.page, { out: mt.out, id: "EV-F02-LOI-503-C1", suKien: t.suKien });
    await goHet(t.page);
    if (nutThuLai) await cham(cdp, nutThuLai);
    await choOn(t.page, { mang: t.mang });
    await t.page.waitForTimeout(800);
    const sauThuLai = await tieuDeDanhSach(t.page);
    log({ nutThuLai: !!nutThuLai, maLoi: m1.tomTat.maLoi, chuLoi, sauThuLai });
    ket({ tc: "TC-F02-LOI-503", screen: "F02.S01", layer: "L35", state: "danh mục trả 503", action: "mở Khám phá, rồi «Thử lại» khi máy chủ lành", cauHinh: "C1", expected: "ErrorState tiếng Việt có «Thử lại»; thử lại thì tải được", status: nutThuLai && m1.tomTat.maLoi === 0 && sauThuLai ? "PASS" : "FAIL", evidence: ["EV-F02-LOI-503-C1"], ghiChu: `nút thử lại: ${!!nutThuLai}; câu: ${chuLoi.join(" | ")}; sau thử lại: ${sauThuLai?.chu}` });
    await t.context.close();

    const t2 = await moKhamPha("C1");
    await ngatMang(t2.context, true);
    const cdp2 = await cdpCua(t2.page);
    await cham(cdp2, await tamCua(t2.page, '[role="tab"][aria-label="Lên plan"]', { cuon: false }));
    await t2.page.waitForTimeout(1500);
    await cham(cdp2, await tamCua(t2.page, '[role="tab"][aria-label="Khám phá"]', { cuon: false }));
    await t2.page.waitForTimeout(800);
    // Pull-to-refresh is not reachable by mouse; the refresh path is the retry
    // of a failed load, so force one: open the city picker and come back.
    const m2 = await chup(t2.page, { out: mt.out, id: "EV-F02-OFFLINE-C1", suKien: t2.suKien });
    await ngatMang(t2.context, false);
    ket({ tc: "TC-F02-OFFLINE", screen: "F02.S01", layer: "L35", state: "mất mạng sau khi đã tải", action: "đổi tab rồi quay lại", cauHinh: "C1", expected: "giữ nội dung đã tải hoặc báo mất mạng bằng tiếng Việt", status: m2.tomTat.maLoi === 0 ? "PASS" : "FAIL", evidence: ["EV-F02-OFFLINE-C1"], ghiChu: `chữ lỗi tiếng Anh: ${m2.tomTat.maLoi}` });
    await t2.context.close();
  }

  // --------------------------------------------------------- change city
  if (chay("diem-den")) {
    const t = await moKhamPha("C1");
    const cdp = await cdpCua(t.page);
    await cham(cdp, await tamCua(t.page, '[aria-label="Đổi điểm đến"]'));
    await choOn(t.page, { mang: t.mang });
    const o1 = await duongDan(t.page);
    await chup(t.page, { out: mt.out, id: "EV-F02-DIEM-DEN-C1", suKien: t.suKien });
    await cham(cdp, await tamCua(t.page, '[aria-label="Chọn Hội An"]'));
    await choOn(t.page, { mang: t.mang });
    await t.page.waitForTimeout(800);
    const o2 = await duongDan(t.page);
    const tieu = await tieuDeDanhSach(t.page);
    const rong = await t.page.evaluate(() => (document.querySelector('[data-testid="explore-screen"]')?.innerText ?? "").split("\n").filter((l) => /chưa|không|thêm/i.test(l)).slice(0, 5));
    const m = await chup(t.page, { out: mt.out, id: "EV-F02-HOI-AN-C1", suKien: t.suKien });
    log({ o1, o2, tieu, rong });
    ket({ tc: "TC-F02-DIEM-DEN", screen: "F02.S02", state: "Đà Lạt", action: "«Đổi điểm đến» → chọn Hội An", cauHinh: "C1", expected: "về Khám phá của Hội An; thành phố chưa có quán thì trạng thái rỗng nói rõ, có lối ra", status: o2 === "/explore" && m.tomTat.maLoi === 0 ? "PASS" : "FAIL", evidence: ["EV-F02-DIEM-DEN-C1", "EV-F02-HOI-AN-C1"], ghiChu: `${o1} → ${o2}; «${tieu?.chu}»; câu: ${rong.join(" | ")}` });
    // Put Minh Anh back in Đà Lạt for the rest of the audit.
    await cham(cdp, await tamCua(t.page, '[aria-label="Đổi điểm đến"]'));
    await choOn(t.page, { mang: t.mang });
    await cham(cdp, await tamCua(t.page, '[aria-label="Chọn Đà Lạt"]'));
    await choOn(t.page, { mang: t.mang });
    log({ traVe: await tieuDeDanhSach(t.page) });
    await t.context.close();
  }

  // ---------------------------------------------------------- place detail
  if (chay("chi-tiet")) {
    const t = await trangMoi(mt, cauHinh("C1"), { persona: P, path: "/places/p-tiem-nuong-xom-lao" });
    const cdp = await cdpCua(t.page);
    const nut = await t.page.evaluate(() => [...document.querySelectorAll('[role="button"],a[href],button')].map((e) => ({ ten: (e.getAttribute("aria-label") || (e.innerText ?? "").trim()).slice(0, 40), href: e.getAttribute("href"), w: Math.round(e.getBoundingClientRect().width), h: Math.round(e.getBoundingClientRect().height) })).filter((x) => x.ten));
    const toanTrang = await chup(t.page, { out: mt.out, id: "EV-F02-CHI-TIET-full-C1", suKien: t.suKien, fullPage: true });
    log({ nut, tomTat: toanTrang.tomTat });
    // Back, opened cold.
    await cham(cdp, await tamCua(t.page, '[aria-label="Quay lại"]'));
    await t.page.waitForTimeout(900);
    const sauBack = await duongDan(t.page);
    ket({ tc: "TC-F02-CHI-TIET-BACK-LANH", screen: "F02.S03", state: "mở chi tiết quán bằng link", action: "chạm «Quay lại»", cauHinh: "C1", expected: "về Khám phá", status: sauBack !== "/places/p-tiem-nuong-xom-lao" ? "PASS" : "FAIL", evidence: [], issue: sauBack === "/places/p-tiem-nuong-xom-lao" ? "UI-018" : null, ghiChu: `sau khi chạm: ${sauBack}` });
    await t.context.close();

    // From the list, then each action.
    // «Rủ <tên> tới đây» exists only for a person with an active two-person
    // notebook (PlaceDetailLive `doi`); that persona is audited in F07.
    // ASCII ids: an evidence file name with diacritics is a hazard for every
    // tool downstream (allowlist paths, links), so each action has a slug.
    for (const [nhan, mau, ma] of [["Thêm vào kèo", /outings\/chon/, "THEM-VAO-KEO"], ["Rủ ", /to-giay|chon-nguoi/, "RU"], ["Chỉ đường", null, "CHI-DUONG"]]) {
      const u = await moKhamPha("C1");
      const c = await cdpCua(u.page);
      await cham(c, await tamCua(u.page, '[aria-label="Mở Tiệm Nướng Xóm Lào"]'));
      await choOn(u.page, { mang: u.mang });
      // Linking.openURL on web is window.open(url, "_blank", "noopener"): record
      // what the app asked for, since an unhandled scheme opens nothing to see.
      await u.page.evaluate(() => {
        const goc = window.open;
        window.__audit_open = [];
        window.open = function (...a) {
          let tra = null;
          let loi = null;
          try {
            tra = goc.apply(this, a);
          } catch (e) {
            loi = String(e);
          }
          window.__audit_open.push({ url: String(a[0]), target: String(a[1] ?? ""), tra: tra === null ? "null" : typeof tra, loi });
          if (loi) throw new Error(loi);
          return tra;
        };
      });
      const popup = u.context.waitForEvent("page", { timeout: 3000 }).catch(() => null);
      const n = await u.page.evaluate((nh) => {
        // Whole-name match: «Rủ » alone also occurs inside «hỏi Rủ Đi AI».
        // Ionicons draw a private-use glyph inside the button text: strip it.
        const ten = (x) => (x.getAttribute("aria-label") || (x.innerText ?? "")).replace(/[\uE000-\uF8FF]/g, "").trim();
        const khop = nh === "Rủ " ? (x) => /^Rủ .+ tới đây$/.test(ten(x)) : (x) => ten(x) === nh;
        const e = [...document.querySelectorAll('[role="button"],a[href],button')].find((x) => khop(x) && x.getBoundingClientRect().width > 0);
        if (!e) return null;
        e.scrollIntoView({ block: "center", behavior: "instant" });
        const r = e.getBoundingClientRect();
        return { x: r.left + r.width / 2, y: r.top + r.height / 2, href: e.getAttribute("href") };
      }, nhan);
      if (n) await cham(c, n);
      await u.page.waitForTimeout(1500);
      const trangMoiMo = await popup;
      const d = await duongDan(u.page);
      const goiMo = await u.page.evaluate(() => window.__audit_open ?? []);
      // The one sentence the catch branch of chiDuong would set.
      const thongBao = await u.page.evaluate(() => ((document.querySelector('[data-testid="place-detail-screen"]')?.innerText ?? "").includes("chưa có ứng dụng bản đồ") ? ["Máy này chưa có ứng dụng bản đồ để chỉ đường."] : []));
      const ten = ma;
      const id = `EV-F02-CHI-TIET-${ten}-C1`;
      await chup(u.page, { out: mt.out, id, suKien: u.suKien });
      log({ nhan, n, d, goiMo, thongBao, trangMoi: trangMoiMo?.url() ?? null });
      const dat = n && (mau ? mau.test(d) : trangMoiMo !== null && trangMoiMo.url() !== "about:blank");
      const ghiChu = !n
        ? nhan === "Rủ "
          ? "không thấy nút: persona này không có sổ đôi đang mở, kiểm ở F07"
          : "không thấy nút"
        : `tới ${d}${goiMo.length ? `; window.open ${goiMo.map((g) => `${g.url.split("?")[0]} → ${g.tra}${g.loi ? ` (${g.loi})` : ""}`).join(", ")}` : ""}${trangMoiMo ? `; trang mới ${trangMoiMo.url() || "about:blank"}` : "; không có trang mới"}; câu trên màn: ${thongBao.join(" | ") || "không"}`;
      ket({ tc: `TC-F02-CHI-TIET-${ten}`, screen: "F02.S03", layer: mau ? "-" : "L33", state: "chi tiết Tiệm Nướng Xóm Lào", action: `chạm «${nhan.trim()}${mau ? "… tới đây" : ""}»`, cauHinh: "C1", expected: mau ? `đi tới ${mau}` : "mở bản đồ ngoài (tab/cửa sổ mới), hoặc một câu nói vì sao không mở được", status: dat ? "PASS" : n ? "FAIL" : "NOT_TESTED", evidence: n ? [id] : [], ghiChu });
      await u.context.close();
    }
  }

  // ------------------------------------------------ stage pop-up (MO12)
  // Explore hands its stage no `thiSai`: the drag tilt is wired only in the dev
  // lab (ThuSanKhau, F10), so a drag here must simply do nothing. What the
  // product screen has is the pop-up on mount, which Reduce Motion skips, and
  // a mount per filter/search cycle (the stage is unmounted while filtering).
  if (chay("bat-san")) {
    for (const cfg of ["C1", "C9"]) {
      const t = await trangMoi(mt, cauHinh(cfg), { persona: P, path: "/profile" });
      const cdp = await cdpCua(t.page);
      // Per animation frame, where the list heading («N nơi ở …») sits: a
      // heading first painted high and then pushed down is a layout jump.
      await t.page.evaluate(() => {
        window.__mauTieuDe = [];
        const t0 = performance.now();
        const buoc = () => {
          const e = [...document.querySelectorAll("div[dir],span")].find((x) => /^\d+ (kết quả|nơi ở .+)$/.test((x.innerText ?? "").trim()));
          const top = e ? Math.round(e.getBoundingClientRect().top) : null;
          const m = window.__mauTieuDe;
          if (!m.length || m[m.length - 1].top !== top) m.push({ ms: Math.round(performance.now() - t0), top });
          if (performance.now() - t0 < 3000) requestAnimationFrame(buoc);
        };
        requestAnimationFrame(buoc);
      });
      const quay = await quayKhung(t.page, cdp, { ms: 2600 });
      await cham(cdp, await tamCua(t.page, '[role="tab"][aria-label="Khám phá"]', { cuon: false }));
      const khung = await quay.dung();
      await t.page.waitForTimeout(500);
      const mauTieuDe = (await t.page.evaluate(() => window.__mauTieuDe)).filter((m) => m.top !== null);
      const anh = join(mt.out, "jpg", `EV-F02-MO12-bat-${cfg}.jpg`);
      await ghepKhung(mt.browser, khung, anh, { tieuDe: `MO12 sân khấu Khám phá dựng lên khi mở tab, ${cfg}${cfg === "C9" ? " (giảm chuyển động)" : ""}` });
      await choOn(t.page, { mang: t.mang });
      const renderer = await t.page.evaluate(() => document.querySelector("[data-renderer]")?.getAttribute("data-renderer") ?? null);

      // Drag across the stage: no tilt is wired, so nothing may move or scroll.
      const san = await tamCua(t.page, '[data-testid="san-thanh-pho"]');
      const quay2 = await quayKhung(t.page, cdp, { ms: 1200 });
      await keo(cdp, { x: san.x - 90, y: san.y }, { x: san.x + 90, y: san.y }, { ms: 500, buoc: 20, giuCuoiMs: 200 });
      const khung2 = await quay2.dung();
      const cuon = await t.page.evaluate(() => ({ x: Math.round(scrollX), doc: Math.round([...document.querySelectorAll("div")].find((e) => { const s = getComputedStyle(e); return s.overflowY === "auto" && e.scrollHeight > e.clientHeight + 20; })?.scrollTop ?? -1) }));

      // Filter then clear: the stage unmounts and mounts again.
      const chip = async () =>
        t.page.evaluate(() => {
          const e = [...document.querySelectorAll('[role="button"],button')].find((x) => (x.innerText ?? "").trim() === "Cafe");
          const r = e.getBoundingClientRect();
          return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
        });
      await cham(cdp, await chip());
      await choOn(t.page, { mang: t.mang });
      // Which renderer is mounted inside the stage, per frame, on the same wall
      // clock as the screencast: the SVG leaving before the canvas has painted
      // shows up as frames with an empty stage.
      await t.page.evaluate(() => {
        window.__mauVe = [];
        const t0 = Date.now();
        const buoc = () => {
          const san = document.querySelector('[data-testid="san-thanh-pho"]');
          const svg = san ? san.querySelectorAll('[data-renderer="svg"]').length : -1;
          const sk = san?.querySelector('[data-renderer="skia"]');
          const op = sk ? Number(getComputedStyle(sk.parentElement).opacity).toFixed(2) : null;
          const m = window.__mauVe;
          const k = `${svg}|${sk ? 1 : 0}|${op}`;
          if (!m.length || m[m.length - 1].k !== k) m.push({ t: Date.now(), svg, skia: sk ? 1 : 0, op, k });
          if (Date.now() - t0 < 3000) requestAnimationFrame(buoc);
        };
        requestAnimationFrame(buoc);
      });
      const quay3 = await quayKhung(t.page, cdp, { ms: 2600 });
      await cham(cdp, await chip());
      const khung3 = await quay3.dung();
      await t.page.waitForTimeout(500);
      const mauVe = await t.page.evaluate(() => window.__mauVe);
      const t0Ve = khung3[0] ? Math.round(khung3[0].t * 1000) : mauVe[0]?.t ?? 0;
      log({ cfg, dongHoVe: mauVe.map((m) => `${m.t - t0Ve}ms svg${m.svg} skia${m.skia} op${m.op}`), khungBoLoc: khung3.map((k) => k.ms) });
      const anh3 = join(mt.out, "jpg", `EV-F02-MO12-bo-loc-${cfg}.jpg`);
      await ghepKhung(mt.browser, khung3, anh3, { tieuDe: `MO12 bỏ lọc «Cafe»: sân khấu mount lại, ${cfg}` });
      log({ cfg, renderer, khungMo: khung.length, khungKeo: khung2.length, cuon, khungBoLoc: khung3.length, mauTieuDe });
      const dau = mauTieuDe[0];
      const cuoiCung = mauTieuDe[mauTieuDe.length - 1];
      const nhay = dau && cuoiCung ? cuoiCung.top - dau.top : null;
      const oLai = mauTieuDe.length > 1 ? mauTieuDe[1].ms - dau.ms : null;
      ket({ tc: "TC-F02-NHAY", screen: "F02.S01", state: "mở tab Khám phá lần đầu", action: "chạm tab, danh mục về", cauHinh: cfg, expected: "dòng «N nơi ở …» vẽ một lần ở chỗ cuối cùng; không bị đẩy xuống sau khi đã hiện", status: nhay === null ? "NOT_TESTED" : Math.abs(nhay) > 4 ? "FAIL" : "PASS", evidence: [`EV-F02-MO12-bat-${cfg}`], ghiChu: nhay === null ? "không thấy dòng tiêu đề" : `vị trí theo khung: ${mauTieuDe.map((m) => `${m.ms}ms→${m.top}`).join(", ")}; nhảy ${nhay}dp sau ${oLai}ms` });
      ket({ tc: `TC-MO12-KEO`, screen: "F02.S01", state: "sân khấu thành phố", action: "kéo ngang 180dp trên tranh", cauHinh: cfg, expected: "tranh nghiêng theo tay (nếu màn có nối nghiêng)", status: "NOT_APPLICABLE", evidence: [], ghiChu: `Khám phá không truyền thiSai cho SanKhau; kéo nghiêng chỉ nối ở bảng dev ThuSanKhau (F10). Đo: ${khung2.length} khung đổi khi kéo, cuộn ngang ${cuon.x}, cuộn dọc ${cuon.doc}` });
      ket({ tc: `TC-MO12-BAT`, screen: "F02.S01", state: "mở tab Khám phá lần đầu", action: "chạm tab", cauHinh: cfg, expected: cfg === "C9" ? "giảm chuyển động: sân khấu hiện đứng sẵn, không dựng từng lớp" : "các lớp dựng lên lần lượt rồi dừng ở tư thế đứng", status: "NOT_TESTED", evidence: [`EV-F02-MO12-bat-${cfg}`], ghiChu: `renderer ${renderer}; ${khung.length} khung; cần đọc ảnh ghép để kết luận` });
      ket({ tc: `TC-MO12-BO-LOC`, screen: "F02.S01", state: "đang lọc «Cafe»", action: "bỏ lọc", cauHinh: cfg, expected: "sân khấu quay lại (ExploreLive ghi «dựng một lần mỗi thành phố»)", status: "NOT_TESTED", evidence: [`EV-F02-MO12-bo-loc-${cfg}`], ghiChu: `${khung3.length} khung; cần đọc ảnh ghép để kết luận` });
      await t.context.close();
    }
  }
  // ------------------------------------------------------------- cut text
  // A one-line text is cut when it is wider than its box. Measured on the
  // element RNW renders for `numberOfLines={1}` (overflow hidden, ellipsis).
  if (chay("cat-chu")) {
    const doCat = (page, mau) =>
      page.evaluate((mauChu) => {
        const re = new RegExp(mauChu);
        return [...document.querySelectorAll("div[dir]")]
          .filter((e) => re.test(e.textContent ?? "") && e.children.length === 0 && e.getBoundingClientRect().width > 0)
          .map((e) => {
            const r = e.getBoundingClientRect();
            // "In view" stops at the tab bar: a row scrolled under it is
            // measured and counted, but not outlined over the bar.
            const bar = document.querySelector('[role="tab"]')?.parentElement?.getBoundingClientRect();
            const day = bar && bar.top > innerHeight / 2 ? bar.top : innerHeight;
            return { chu: e.textContent, thieu: Math.round(e.scrollWidth - e.clientWidth), rong: Math.round(r.width), x: r.left, y: r.top, w: r.width, h: r.height, trongMan: r.top >= 0 && r.bottom <= day };
          });
      }, mau.source);
    for (const cfg of ["C1", "C2", "C3", "C4", "C5", "C6", "C7"]) {
      const t = await moKhamPha(cfg);
      // Bring the list heading to the top so the first rows sit above the tab
      // bar and can be outlined; every row is measured either way.
      await t.page.evaluate(() => [...document.querySelectorAll("div[dir],span")].find((x) => /^\d+ (kết quả|nơi ở .+)$/.test((x.innerText ?? "").trim()))?.scrollIntoView({ block: "start", behavior: "instant" }));
      await t.page.waitForTimeout(600);
      const dong = await doCat(t.page, /mỗi người/);
      const cat = dong.filter((d) => d.thieu > 1);
      const hienTrongMan = cat.filter((d) => d.trongMan);
      const id = `EV-F02-CAT-META-${cfg}`;
      await chup(t.page, { out: mt.out, id, suKien: t.suKien, chuThich: hienTrongMan.slice(0, 8).map((d) => ({ rect: { x: d.x, y: d.y, w: d.w, h: d.h }, nhan: `thiếu ${d.thieu}px` })) });
      log({ cfg, soDong: dong.length, soCat: cat.length, thieu: cat.map((d) => d.thieu) });
      ket({ tc: "TC-F02-META", screen: "F02.S01", state: "danh sách Đà Lạt (10 nơi)", action: "đọc dòng điểm · km · giá", cauHinh: cfg, expected: "khoảng giá và «mỗi người» đọc được trọn (thiết kế cho dải giá một dòng riêng)", status: dong.length === 0 ? "NOT_TESTED" : cat.length === 0 ? "PASS" : "FAIL", evidence: [id], ghiChu: `${cat.length}/${dong.length} dòng chứa giá bị cắt; thiếu ${cat.length ? `${Math.min(...cat.map((d) => d.thieu))}–${Math.max(...cat.map((d) => d.thieu))}px` : "0px"}` });
      await t.context.close();
    }
    for (const cfg of ["C1", "C2", "C3", "C4", "C5", "C6", "C7"]) {
      const t = await trangMoi(mt, cauHinh(cfg), { persona: P, path: "/places/p-tiem-nuong-xom-lao" });
      const nhan = (await doCat(t.page, /^(Lưu địa điểm|Đã lưu)$/)).filter((d) => d.trongMan);
      const cat = nhan.filter((d) => d.thieu > 1);
      const id = `EV-F02-CAT-LUU-${cfg}`;
      await chup(t.page, { out: mt.out, id, suKien: t.suKien, chuThich: cat.map((d) => ({ rect: { x: d.x, y: d.y, w: d.w, h: d.h }, nhan: `thiếu ${d.thieu}px` })) });
      log({ cfg, nhan: nhan.map((d) => `${d.chu} rộng ${d.rong} thiếu ${d.thieu}`) });
      ket({ tc: "TC-F02-NHAN-LUU", screen: "F02.S03", state: "chi tiết Tiệm Nướng Xóm Lào", action: "đọc nhãn nút ở chân trang", cauHinh: cfg, expected: "nhãn «Lưu địa điểm» đọc được trọn", status: nhan.length === 0 ? "NOT_TESTED" : cat.length === 0 ? "PASS" : "FAIL", evidence: [id], ghiChu: nhan.length ? nhan.map((d) => `«${d.chu}» ô chữ ${d.rong}px, thiếu ${d.thieu}px`).join("; ") : "không thấy nhãn" });
      await t.context.close();
    }
  }
} finally {
  await mt.dong();
}
