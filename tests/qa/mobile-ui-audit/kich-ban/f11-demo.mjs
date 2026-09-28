/* F11, the demo mode: what a signed-out person sees when a link takes them
 * past the welcome screen. `/` sends them to /welcome, but the four tab
 * routes and eight demo routes render the «Team Đà Lạt» fixture instead of a
 * sign-in gate. Measured here: the demo label on every such screen, the way
 * back to signing in, the eight routes with a session (they must leave the
 * demo, regression B5), the demo trip's options sheet (L28), and «Đăng xuất
 * bản trải nghiệm».
 *
 *   node kich-ban/f11-demo.mjs [--chi tab,route,co-phien,l28,l28-thoat,cat-chu,thoat]
 *
 * Nothing is written to the stack: the demo keeps its state in the page.
 */
import { join } from "node:path";

import { chayAxe } from "../thu-vien/axe.mjs";
import { cauHinh } from "../thu-vien/cau-hinh.mjs";
import { chup, ghepAnh } from "../thu-vien/chup.mjs";
import { cdpCua, cham } from "../thu-vien/cu-chi.mjs";
import { duongDan } from "../thu-vien/dieu-huong.mjs";
import { choDialog, demDialog, dong, focusHienTai, inertConLai, luoiChamTrang, soLuoi } from "../thu-vien/lop-phu.mjs";
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
  so.ghi({ feature: "F11", nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, ...rec });
  console.log(`${rec.status.padEnd(10)} ${rec.tc} ${rec.cauHinh ?? ""} ${rec.ghiChu ?? ""}`);
};
const log = (o) => console.log(JSON.stringify(o));
const P = (ten) => personaTheoTen(ten, mt.chatSessions);
const mo = (cfg, persona, path) => trangMoi(mt, cauHinh(cfg), { persona, path });
const catChu = (s, n = 160) => (s.length > n ? `${s.slice(0, n)}…` : s);

/** What a signed-out visitor can read and press on the current screen. */
const doMan = (page) =>
  page.evaluate(() => {
    const hien = (x) => x.getClientRects().length > 0 && (x.checkVisibility ? x.checkVisibility({ visibilityProperty: true, opacityProperty: true }) : true);
    const ten = (x) => (x.getAttribute("aria-label") || (x.innerText ?? "")).replace(/[-]/g, "").replace(/\s+/g, " ").trim();
    const la = [...document.querySelectorAll("div,span")].filter((e) => hien(e) && [...e.childNodes].some((n) => n.nodeType === 3 && n.textContent.trim()));
    const demo = la.filter((e) => /^(Dữ liệu demo|Demo)$/.test(e.textContent.trim())).map((e) => { const r = e.getBoundingClientRect(); return `«${e.textContent.trim()}» y ${Math.round(r.top)}`; });
    const nut = [...document.querySelectorAll('[role="button"],button,[role="link"],a[href]')].filter(hien).map(ten).filter(Boolean);
    const vao = nut.filter((t) => /Đăng nhập|Rủ Đi thôi|lời mời|Đăng xuất bản trải nghiệm|Bắt đầu|Vào app/i.test(t));
    const tieuDe = [...document.querySelectorAll('[role="heading"]')].filter(hien).map((e) => e.textContent.trim()).slice(0, 3);
    const loi = la.filter((e) => /Chưa đọc được|Không mở được|lỗi|sự cố/i.test(e.textContent)).map((e) => catChu(e.textContent.trim(), 80));
    function catChu(s, n) { return s.length > n ? `${s.slice(0, n)}…` : s; }
    return { demo, vao: [...new Set(vao)], soNut: nut.length, tieuDe, loi: loi.slice(0, 2) };
  });

const TAB = [["explore", "Khám phá"], ["plan", "Lên plan"], ["messages", "Tin nhắn"], ["profile", "Cá nhân"]];
const ROUTE = [
  ["finance", "/finance"],
  ["settlements", "/settlements/team-da-lat"],
  ["timeline", "/trips/team-da-lat/timeline"],
  ["itinerary", "/trips/team-da-lat/itinerary"],
  ["votes", "/votes/diem-den"],
  ["checkin", "/check-ins/new"],
  ["ai-match", "/ai-match"],
  ["assignment", "/smart-split/team-da-lat/assignment"],
];

try {
  // ------------------------------------------------ the four tabs, signed out
  if (chay("tab")) {
    for (const cfg of ["C1", "C2", "C3"]) {
      const anh = [];
      for (const [tab, nhan] of TAB) {
        const t = await mo(cfg, null, `/${tab}`);
        await t.page.waitForTimeout(3500);
        const toi = await duongDan(t.page);
        const m = await doMan(t.page);
        const id = `EV-F11-TAB-${tab}-${cfg}`;
        const c = await chup(t.page, { out: mt.out, id, suKien: t.suKien });
        anh.push({ file: join(mt.out, "jpg", `${id}.jpg`), nhan: `${nhan} (${toi})` });
        const axe = cfg === "C1" ? await chayAxe(t.page).catch(() => ({ vi: [] })) : null;
        ket({ tc: `TC-F11-TAB-${tab.toUpperCase()}`, screen: "F11.S01", state: "chưa đăng nhập, mở thẳng đường dẫn của tab", action: `mở /${tab}`, cauHinh: cfg, expected: "màn demo có nhãn «Dữ liệu demo» (hoặc «Demo» trên thanh đầu); không tràn, không cắt", status: m.demo.length && !c.tomTat.tranNgang && !c.tomTat.chuBiCat ? "PASS" : "FAIL", evidence: [id], ghiChu: `tới ${toi}; nhãn demo ${m.demo.join(", ") || "không có"}; lối về cửa vào: ${m.vao.join(", ") || "không có"}; ${m.soNut} nút; tiêu đề ${m.tieuDe.join(" / ") || "-"}; tràn ${c.tomTat.tranNgang}, cắt ${c.tomTat.chuBiCat}${axe ? `; axe ${axe.vi.map((v) => `${v.rule}×${v.so}`).join(", ") || "sạch"}` : ""}` });
        await t.context.close();
      }
      await ghepAnh(mt.browser, anh, join(mt.out, "jpg", `EV-F11.S01-BASE-${cfg}-ghep.jpg`), { tieuDe: `F11.S01 bốn tab khi chưa đăng nhập, ${cfg}` });
    }
  }

  // ------------------------------------------- the eight routes, signed out
  if (chay("route")) {
    for (const cfg of ["C1", "C2", "C3"]) {
      const anh = [];
      for (const [ten, path] of ROUTE) {
        const t = await mo(cfg, null, path);
        await t.page.waitForTimeout(3500);
        const toi = await duongDan(t.page);
        const m = await doMan(t.page);
        const id = `EV-F11-ROUTE-${ten}-${cfg}`;
        const c = await chup(t.page, { out: mt.out, id, suKien: t.suKien });
        anh.push({ file: join(mt.out, "jpg", `${id}.jpg`), nhan: ten });
        const loiTrang = (c.suKien?.log?.pageerror ?? []).length;
        ket({ tc: `TC-F11-ROUTE-${ten.toUpperCase()}`, screen: "F11.S02", state: "chưa đăng nhập, mở thẳng route demo", action: `mở ${path}`, cauHinh: cfg, expected: "màn demo có nhãn «Dữ liệu demo» (hoặc «Demo»); không lỗi trang; không tràn, không cắt", status: m.demo.length && !loiTrang && !c.tomTat.tranNgang && !c.tomTat.chuBiCat ? "PASS" : "FAIL", evidence: [id], ghiChu: `tới ${toi}; nhãn demo ${m.demo.join(", ") || "không có"}; tiêu đề ${m.tieuDe.join(" / ") || "-"}; lối về cửa vào: ${m.vao.join(", ") || "không có"}; câu lỗi ${m.loi.join(" | ") || "không"}; lỗi trang ${loiTrang}; tràn ${c.tomTat.tranNgang}, cắt ${c.tomTat.chuBiCat}` });
        await t.context.close();
      }
      for (let k = 0; k < anh.length; k += 4) await ghepAnh(mt.browser, anh.slice(k, k + 4), join(mt.out, "jpg", `EV-F11.S02-BASE-${cfg}-${k / 4}-ghep.jpg`), { tieuDe: `F11.S02 route demo khi chưa đăng nhập, ${cfg}, phần ${k / 4 + 1}` });
    }
  }

  // ----------------------------------- the same routes with a session (B5)
  if (chay("co-phien")) {
    const DICH = { finance: /^\/finance$/, settlements: /^\/settlements\//, timeline: /^\/outings\//, itinerary: /^\/outings\//, votes: /^\/messages$/, checkin: /^\/plan$/, "ai-match": /^\/explore$/, assignment: /^\/smart-split\/moi\/review$/ };
    for (const [ten, path] of ROUTE) {
      const t = await mo("C1", P("chat-0"), path);
      await t.page.waitForTimeout(3500);
      const toi = await duongDan(t.page);
      const m = await doMan(t.page);
      const id = `EV-F11-CO-PHIEN-${ten}-C1`;
      await chup(t.page, { out: mt.out, id, suKien: t.suKien });
      const dung = DICH[ten].test(toi) && !m.demo.length;
      ket({ tc: `TC-F11-CO-PHIEN-${ten.toUpperCase()}`, screen: "F11.S02", state: "đã đăng nhập (chat-0), mở thẳng route demo", action: `mở ${path}`, cauHinh: "C1", expected: "rời bản demo: chuyển về màn sống tương ứng, không còn nhãn demo (sửa B5)", status: dung ? "PASS" : "FAIL", evidence: [id], ghiChu: `tới ${toi}; nhãn demo ${m.demo.join(", ") || "không có"}; tiêu đề ${m.tieuDe.join(" / ") || "-"}; câu lỗi ${m.loi.join(" | ") || "không"}` });
      await t.context.close();
    }
  }

  // --------------------------------- the demo trip's options sheet (L28)
  // Reached through the tab bar from Khám phá, so Back has somewhere to go.
  const moSheet = async (cfg) => {
    const t = await mo(cfg, null, "/explore");
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(3000);
    const tab = await t.page.evaluate(() => { const e = [...document.querySelectorAll('[role="tab"],[role="link"],a[href]')].find((x) => /Lên plan/.test(x.getAttribute("aria-label") ?? x.innerText ?? "")); if (!e) return null; const r = e.getBoundingClientRect(); return { x: r.left + r.width / 2, y: r.top + r.height / 2 }; });
    if (tab) await cham(cdp, tab);
    await t.page.waitForTimeout(2500);
    const truoc = await luoiChamTrang(t.page);
    const inertTruoc = (await inertConLai(t.page)).length;
    const nut = await t.page.evaluate(() => { const e = [...document.querySelectorAll('[aria-label="Tùy chọn"]')].find((x) => x.getClientRects().length); if (!e) return null; const r = e.getBoundingClientRect(); return { x: r.left + r.width / 2, y: r.top + r.height / 2, w: Math.round(r.width), h: Math.round(r.height) }; });
    if (nut) await cham(cdp, nut);
    const moRa = await choDialog(t.page);
    await t.page.waitForTimeout(700);
    return { t, cdp, truoc, inertTruoc, nut, moRa };
  };
  if (chay("l28")) {
    const kq = [];
    for (const cach of ["x", "nen", "esc", "keo-ngan", "keo-dai", "fling", "back"]) {
      const { t, cdp, truoc, inertTruoc, nut, moRa } = await moSheet("C1");
      const focusMo = await focusHienTai(t.page);
      const hop = await t.page.evaluate(() => { const d = [...document.querySelectorAll('[role="dialog"]')].pop(); const r = d?.getBoundingClientRect(); return r ? { ten: d.getAttribute("aria-label"), phan: Math.round((r.height / innerHeight) * 100), nut: [...d.querySelectorAll('[role="button"],button')].map((b) => (b.getAttribute("aria-label") || b.innerText || "").replace(/\s+/g, " ").trim()).filter(Boolean) } : null; });
      if (cach === "x") await chup(t.page, { out: mt.out, id: "EV-F11-L28-C1", suKien: t.suKien });
      if (cach === "back") await chup(t.page, { out: mt.out, id: "EV-F11-L28-BACK-C1-a", suKien: t.suKien });
      const d = await dong(t.page, cdp, cach);
      await t.page.waitForTimeout(cach === "keo-ngan" ? 900 : 1200);
      const conMo = await demDialog(t.page);
      const duong = await duongDan(t.page);
      const oLai = duong === "/plan";
      const khac = oLai && conMo === 0 ? soLuoi(truoc, await luoiChamTrang(t.page)).length : null;
      const inertSau = oLai && conMo === 0 ? (await inertConLai(t.page)).length : null;
      const focusSau = await focusHienTai(t.page);
      let dungDuoc = null;
      if (cach === "back") {
        const khoa = await t.page.evaluate(() => { const man = document.querySelector('[data-testid="explore-screen"]'); for (let n = man; n; n = n.parentElement) if (n.inert) return true; return man ? false : null; });
        const tim = await t.page.evaluate(() => { const e = [...document.querySelectorAll('[data-testid="explore-screen"] [aria-label^="Lưu "]')].find((x) => { const r = x.getBoundingClientRect(); return r.top > 0 && r.bottom < innerHeight - 90; }); if (!e) return null; const r = e.getBoundingClientRect(); return { x: r.left + r.width / 2, y: r.top + r.height / 2, w: r.width, h: r.height, ten: e.getAttribute("aria-label") }; });
        const inertLuc = (await inertConLai(t.page)).length;
        // Hit testing skips inert nodes, so this names what lies under the
        // inert Khám phá at the heart: where the tap will really land.
        const duoiTim = tim ? await t.page.evaluate(({ x, y }) => { const e = document.elementFromPoint(x, y); const c = e?.closest("[aria-label]"); return c ? c.getAttribute("aria-label") : e ? e.tagName.toLowerCase() : null; }, tim) : null;
        if (tim) await cham(cdp, tim);
        await t.page.waitForTimeout(1200);
        const doi = tim ? await t.page.evaluate((ten) => !!document.querySelector(`[aria-label="${ten.replace(/^Lưu /, "Bỏ lưu ")}"]`), tim.ten) : null;
        const khoaSau = await t.page.evaluate(() => { const man = document.querySelector('[data-testid="explore-screen"]'); for (let n = man; n; n = n.parentElement) if (n.inert) return true; return man ? false : null; });
        dungDuoc = { khoa, inertLuc, tim: tim?.ten ?? null, duoiTim, doi, conMoSau: await demDialog(t.page), khoaSau };
        // Before and after, side by side: the screen Back lands on looks
        // normal, so the frame after the tap carries a box on the heart.
        const nhanTim = doi === null ? "không thấy tim" : doi ? "đã chạm: đổi" : "đã chạm: không đổi";
        await chup(t.page, { out: mt.out, id: "EV-F11-L28-BACK-C1-b", suKien: t.suKien, chuThich: tim ? [{ rect: { x: tim.x - tim.w / 2, y: tim.y - tim.h / 2, w: tim.w, h: tim.h }, nhan: nhanTim }] : [] });
        await ghepAnh(mt.browser, [{ file: join(mt.out, "jpg", "EV-F11-L28-BACK-C1-a.jpg"), nhan: "1 · sheet mở ở /plan" }, { file: join(mt.out, "jpg", tim ? "EV-F11-L28-BACK-C1-b-ct.jpg" : "EV-F11-L28-BACK-C1-b.jpg"), nhan: `2 · Back → ${duong}, ${nhanTim}` }], join(mt.out, "jpg", "EV-F11-L28-BACK-C1.jpg"), { tieuDe: "L28 Back trình duyệt khi sheet «Tùy chọn chuyến đi» mở, C1" });
      }
      kq.push({ cach, nut, mo: moRa.ok, focusMo, hop, lam: d.lam, ly: d.ly, conMo, duong, khac, inertTruoc, inertSau, focusSau, dungDuoc });
      await t.context.close();
    }
    log({ l28: kq });
    const f = (k) => `${k.cach}: ${k.lam ? "" : `không làm được (${k.ly}); `}còn ${k.conMo} hộp thoại, ở ${k.duong}${k.khac === null ? "" : `, lưới đổi ${k.khac}, vùng inert/aria-hidden ${k.inertTruoc} trước → ${k.inertSau} sau`}`;
    const dung = kq.filter((k) => k.cach !== "back");
    const x = kq[0];
    ket({ tc: "TC-L28-VONGDOI", screen: "F11.S01", layer: "L28", state: "tab «Lên plan» demo (chuyến «Đà Lạt cuối tuần»), tới từ Khám phá bằng thanh tab", action: "chạm «Tùy chọn»; đóng bằng X, nền, Esc, kéo ngắn, kéo dài, vuốt nhanh", cauHinh: "C1", expected: "mở một hộp thoại, focus vào trong, cao ≤ 82%; kéo ngắn bật về; mọi cách khác đóng hẳn, ở lại tab, không sót lớp chặn hay inert; focus về «Tùy chọn»", status: dung.every((k) => k.mo && k.lam && (k.cach === "keo-ngan" ? k.conMo === 1 : k.conMo === 0 && k.duong === "/plan" && k.khac === 0 && k.inertSau === k.inertTruoc)) && x.focusMo?.trongDialog && x.hop && x.hop.phan <= 82 ? "PASS" : "FAIL", evidence: ["EV-F11-L28-C1"], ghiChu: `nút «Tùy chọn» ${x.nut ? `${x.nut.w}×${x.nut.h}` : "không thấy"}; sheet «${x.hop?.ten ?? "-"}» ${x.hop?.phan ?? "-"}% màn, nút: ${x.hop?.nut?.join(", ") || "-"}; ${dung.map(f).join("; ")}; focus khi mở «${x.focusMo?.ten ?? "body"}» (${x.focusMo?.trongDialog ? "trong" : "ngoài"}), sau khi đóng bằng X «${x.focusSau?.ten ?? "body"}»` });
    const b = kq.find((k) => k.cach === "back");
    const u = b.dungDuoc ?? {};
    ket({ tc: "TC-L28-BACK", screen: "F11.S01", layer: "L28", state: "sheet «Tùy chọn chuyến đi» mở, tới tab từ Khám phá", action: "Back trình duyệt, rồi chạm tim của quán đầu trên màn hiện ra", cauHinh: "C1", expected: "Back đóng sheet và ở lại tab; hoặc, nếu rời tab, màn hiện ra dùng được bình thường (không inert, chạm có tác dụng)", status: (b.duong === "/plan" && b.conMo === 0) || (u.khoa === false && u.doi === true) ? "PASS" : "FAIL", evidence: ["EV-F11-L28-BACK-C1"], ghiChu: `tới ${b.duong}; sheet còn mở trong màn «Lên plan» nằm dưới: ${b.conMo ? "có" : "không"}; màn Khám phá nằm dưới tổ tiên inert: ${u.khoa === null ? "-" : u.khoa ? "có" : "không"}; vùng inert/aria-hidden ${b.inertTruoc} trước khi mở → ${u.inertLuc ?? "-"} sau Back; chạm «${u.tim ?? "-"}»: ${u.doi === null ? "-" : u.doi ? "đổi thành Bỏ lưu" : "không đổi"}, dưới điểm chạm là «${u.duoiTim ?? "-"}» của sheet; sau cú chạm còn ${u.conMoSau ?? "-"} hộp thoại, Khám phá ${u.khoaSau === null ? "-" : u.khoaSau ? "vẫn inert" : "hết inert"}` });
    // Low window and tablet widths.
    for (const cfg of ["C8", "C6", "C7"]) {
      const { t, moRa } = await moSheet(cfg);
      const hop = await t.page.evaluate(() => { const d = [...document.querySelectorAll('[role="dialog"]')].pop(); const r = d?.getBoundingClientRect(); if (!r) return null; const nut = [...d.querySelectorAll('[role="button"],button')].filter((b) => b.getClientRects().length).map((b) => { const q = b.getBoundingClientRect(); return { ten: (b.getAttribute("aria-label") || b.innerText || "").replace(/\s+/g, " ").trim(), top: Math.round(q.top), bottom: Math.round(q.bottom) }; }); return { phan: Math.round((r.height / innerHeight) * 100), rong: Math.round(r.width), cua: innerWidth, nut }; });
      const id = `EV-F11-L28-${cfg}`;
      await chup(t.page, { out: mt.out, id, suKien: t.suKien });
      const ngoai = (hop?.nut ?? []).filter((n) => n.bottom > (cfg === "C8" ? 460 : 99999) || n.top < 0);
      ket({ tc: "TC-L28-KICH-THUOC", screen: "F11.S01", layer: "L28", state: "sheet «Tùy chọn chuyến đi» mở", action: "đo sheet", cauHinh: cfg, expected: cfg === "C8" ? "cao ≤ 82%; mọi nút trong sheet với tới được (cuộn trong sheet nếu cần)" : "sheet có trần bề ngang hợp lý ở tablet", status: moRa.ok && hop && hop.phan <= 82 && !ngoai.length ? "PASS" : "FAIL", evidence: [id], ghiChu: hop ? `${hop.phan}% cao, rộng ${hop.rong}/${hop.cua}; nút: ${hop.nut.map((n) => `${n.ten} (${n.top}–${n.bottom})`).join(", ")}` : "sheet không mở" });
      await t.context.close();
    }
  }

  // ------------- after Back with the sheet open: what a tap does from there
  // TC-L28-BACK leaves the sheet open in the «Lên plan» screen, which stays
  // laid out under Khám phá, while Khám phá and the tab bar are inert. Hit
  // testing skips inert nodes, so a tap on Khám phá lands on the sheet
  // underneath; `duoi` names what it lands on. Tried, each from a fresh
  // Back: the «Lên plan» tab, «Tạo mới», the search box (upper part: the
  // sheet's scrim lies under it), a point on the «Bánh căn Lệ» card that
  // sits over the sheet's «Tường nhóm» button, browser Forward, and Esc
  // (desktop only) for comparison.
  if (chay("l28-thoat")) {
    const nutCo = (page, sel) => page.evaluate((sel) => { const e = [...document.querySelectorAll(sel)].find((x) => x.getClientRects().length); if (!e) return null; let khoa = false; for (let n = e; n; n = n.parentElement) if (n.inert) khoa = true; const r = e.getBoundingClientRect(); return { x: r.left + r.width / 2, y: r.top + r.height / 2, khoa }; }, sel);
    const khoaKP = (page) => page.evaluate(() => { const man = document.querySelector('[data-testid="explore-screen"]'); for (let n = man; n; n = n.parentElement) if (n.inert) return true; return man ? false : null; });
    const duoiDiem = (page, p) => page.evaluate(({ x, y }) => { const e = document.elementFromPoint(x, y); const c = e?.closest("[aria-label]"); return c ? c.getAttribute("aria-label") : e ? e.tagName.toLowerCase() : null; }, p);
    // What the person sees at a point: the smallest labelled or texted box of
    // Khám phá containing it, found by geometry because hit testing skips it.
    const thayTai = (page, p) => page.evaluate(({ x, y }) => { const man = document.querySelector('[data-testid="explore-screen"]'); if (!man) return null; let tot = null; for (const e of man.querySelectorAll("*")) { const r = e.getBoundingClientRect(); if (x < r.left || x > r.right || y < r.top || y > r.bottom) continue; const chu = (e.getAttribute("aria-label") || e.innerText || "").replace(/\s+/g, " ").trim(); if (!chu) continue; const s = r.width * r.height; if (!tot || s < tot.s) tot = { s, chu: chu.slice(0, 60) }; } return tot?.chu ?? null; }, p);
    const oTim = (page) => page.evaluate(() => { const e = [...document.querySelectorAll('[data-testid="explore-screen"] input')].find((x) => x.getClientRects().length); if (!e) return null; const r = e.getBoundingClientRect(); return { x: r.left + r.width / 2, y: r.top + r.height / 2, focus: document.activeElement === e }; });
    const trangThai = async (page) => ({ duong: await duongDan(page), conMo: await demDialog(page), khoa: await khoaKP(page) });
    const kq = [];
    for (const cachRa of ["tab", "tao", "tren", "duoi", "tien", "esc"]) {
      const { t, cdp } = await moSheet("C1");
      await dong(t.page, cdp, "back");
      await t.page.waitForTimeout(1200);
      const sauBack = await trangThai(t.page);
      const k = { cachRa, sauBack };
      if (cachRa === "tab" || cachRa === "tao") {
        const nut = await nutCo(t.page, cachRa === "tab" ? '[role="tab"][aria-label="Lên plan"]' : '[aria-label="Tạo mới"]');
        k.khoaNut = nut?.khoa ?? null;
        k.duoi = nut ? await duoiDiem(t.page, nut) : null;
        if (nut) await cham(cdp, nut);
      } else if (cachRa === "tren") {
        const o = await oTim(t.page);
        k.duoi = o ? await duoiDiem(t.page, o) : null;
        if (o) await cham(cdp, o);
        await t.page.waitForTimeout(1200);
        k.lan1 = (await oTim(t.page))?.focus ?? null;
        k.giua = await trangThai(t.page);
        if (o) await cham(cdp, o);
        await t.page.waitForTimeout(700);
        k.lan2 = (await oTim(t.page))?.focus ?? null;
      } else if (cachRa === "duoi") {
        const nut = await t.page.evaluate(() => { const e = [...document.querySelectorAll('[role="dialog"] [role="button"],[role="dialog"] button')].find((b) => /Tường nhóm/.test(b.getAttribute("aria-label") || b.innerText || "")); if (!e) return null; const r = e.getBoundingClientRect(); return { x: r.left + r.width * 0.25, y: r.top + r.height / 2 }; });
        if (nut) {
          k.thay = await thayTai(t.page, nut);
          k.duoi = await duoiDiem(t.page, nut);
          await chup(t.page, { out: mt.out, id: "EV-F11-L28-THOAT-C1-a", suKien: t.suKien, chuThich: [{ rect: { x: nut.x - 22, y: nut.y - 22, w: 44, h: 44 }, nhan: `chạm «${k.thay ?? "?"}»` }] });
          await cham(cdp, nut);
        }
      } else if (cachRa === "tien") {
        await t.page.goForward().catch(() => null);
      } else {
        await t.page.keyboard.press("Escape");
      }
      await t.page.waitForTimeout(1800);
      k.sau = await trangThai(t.page);
      if (cachRa === "duoi") {
        await chup(t.page, { out: mt.out, id: "EV-F11-L28-THOAT-C1-b", suKien: t.suKien });
        await ghepAnh(mt.browser, [{ file: join(mt.out, "jpg", "EV-F11-L28-THOAT-C1-a-ct.jpg"), nhan: "1 · sau Back: chạm thẻ quán trên Khám phá" }, { file: join(mt.out, "jpg", "EV-F11-L28-THOAT-C1-b.jpg"), nhan: `2 · kết quả: ${k.sau.duong}` }], join(mt.out, "jpg", "EV-F11-L28-THOAT-C1.jpg"), { tieuDe: "L28 sau Back: chạm lên Khám phá rơi xuống sheet vô hình bên dưới, C1" });
      }
      if (cachRa === "esc") {
        const o = await oTim(t.page);
        if (o) await cham(cdp, o);
        await t.page.waitForTimeout(700);
        k.lan1 = (await oTim(t.page))?.focus ?? null;
      }
      kq.push(k);
      await t.context.close();
    }
    log({ l28Thoat: kq });
    const ts = (s) => `ở ${s.duong}, ${s.conMo} hộp thoại, Khám phá ${s.khoa ? "inert" : "không inert"}`;
    const by = Object.fromEntries(kq.map((k) => [k.cachRa, k]));
    const tabNhan = (k) => k.sau.duong !== k.sauBack.duong || k.sau.conMo !== k.sauBack.conMo;
    ket({ tc: "TC-L28-BACK-LOI-RA", screen: "F11.S01", layer: "L28", state: "sau TC-L28-BACK: ở /explore, sheet còn mở trong màn «Lên plan» nằm dưới", action: "chạm tab «Lên plan»; chạm «Tạo mới»; Forward trình duyệt; Esc (so sánh)", cauHinh: "C1", expected: "thanh tab và «Tạo mới» phản hồi như thường, không cần bàn phím hay tải lại", status: tabNhan(by.tab) && tabNhan(by.tao) ? "PASS" : "FAIL", evidence: [], ghiChu: `sau Back: ${ts(by.tab.sauBack)}; tab «Lên plan» ${by.tab.khoaNut ? "nằm dưới inert" : "không inert"}, dưới điểm chạm «${by.tab.duoi ?? "-"}», sau khi chạm ${ts(by.tab.sau)}; «Tạo mới» ${by.tao.khoaNut ? "nằm dưới inert" : "không inert"}, dưới điểm chạm «${by.tao.duoi ?? "-"}», sau khi chạm ${ts(by.tao.sau)}; Forward: ${ts(by.tien.sau)}; Esc: ${ts(by.esc.sau)}, rồi chạm ô tìm: ô ${by.esc.lan1 ? "nhận" : "không nhận"} focus` });
    const t1 = by.tren;
    const t2 = by.duoi;
    ket({ tc: "TC-L28-BACK-CHAM-XUYEN", screen: "F11.S01", layer: "L28", state: "sau TC-L28-BACK: ở /explore, sheet còn mở trong màn «Lên plan» nằm dưới", action: "chạm ô tìm (hai lần); chạm thẻ quán «Bánh căn Lệ» ở phần dưới màn", cauHinh: "C1", expected: "mỗi cú chạm làm đúng việc của phần tử người dùng thấy: ô tìm nhận focus ngay lần đầu; chạm thẻ quán không đưa tới màn khác màn quán", status: t1.lan1 === true && !(t2.sau.duong !== t2.sauBack.duong && !/places/.test(t2.sau.duong)) ? "PASS" : "FAIL", evidence: ["EV-F11-L28-THOAT-C1"], ghiChu: `ô tìm: dưới điểm chạm là «${t1.duoi ?? "-"}» của sheet; lần 1 ô ${t1.lan1 ? "nhận" : "không nhận"} focus, sau đó ${ts(t1.giua)}; lần 2 ô ${t1.lan2 ? "nhận" : "không nhận"} focus. Thẻ quán: người dùng thấy «${t2.thay ?? "-"}», dưới điểm chạm là «${t2.duoi ?? "-"}» của sheet; sau khi chạm ${ts(t2.sau)}` });
  }

  // --------------- texts cut with «…» on the demo routes (money, button labels)
  // doDac lists ellipses apart from clipping, so the route rows above passed
  // with these cut; one sits below the fold. Each is brought into view, boxed
  // and measured (scrollWidth against clientWidth) so the verdict rests on a
  // capture, not on the list alone.
  if (chay("cat-chu")) {
    const CAT = [
      ["settlements", "/settlements/team-da-lat", ["C2"], "^(1\\.106\\.250đ|Quyết toán chuyến đi)$"],
      ["itinerary", "/trips/team-da-lat/itinerary", ["C2", "C3"], "^(Chỉnh lịch trình|Dùng plan này)$"],
      ["checkin", "/check-ins/new", ["C2", "C3"], "^(Nhắc thành viên|Tôi đã tới)$"],
    ];
    const tim = (page, mau, cuon) =>
      page.evaluate(({ mau, cuon }) => {
        const re = new RegExp(mau);
        const ds = [...document.querySelectorAll("div,span")].filter((e) => re.test((e.textContent ?? "").trim()) && [...e.childNodes].some((n) => n.nodeType === 3 && n.textContent.trim()) && e.getClientRects().length);
        if (cuon && ds.length) ds[ds.length - 1].scrollIntoView({ block: "center" });
        return ds.map((e) => { const r = e.getBoundingClientRect(); return { chu: e.textContent.trim(), rong: e.clientWidth, can: e.scrollWidth, cat: e.scrollWidth > e.clientWidth + 1, x: r.left, y: r.top, w: r.width, h: r.height }; });
      }, { mau, cuon });
    for (const [ten, path, cfgs, mau] of CAT) {
      for (const cfg of cfgs) {
        const t = await mo(cfg, null, path);
        await t.page.waitForTimeout(3000);
        await tim(t.page, mau, true);
        await t.page.waitForTimeout(600);
        const ds = await tim(t.page, mau, false);
        const id = `EV-F11-CAT-${ten}-${cfg}`;
        await chup(t.page, { out: mt.out, id, suKien: t.suKien, chuThich: ds.filter((d) => d.cat).map((d) => ({ rect: { x: d.x - 3, y: d.y - 3, w: d.w + 6, h: d.h + 6 }, nhan: `thiếu ${d.can - d.rong}px` })) });
        ket({ tc: `TC-F11-CAT-${ten.toUpperCase()}`, screen: "F11.S02", state: "chưa đăng nhập, route demo", action: `mở ${path}, cuộn tới chữ cần đo`, cauHinh: cfg, expected: "số tiền, tiêu đề và nhãn nút đọc trọn, không bị dấu …", status: ds.some((d) => d.cat) ? "FAIL" : "PASS", evidence: [id], ghiChu: ds.map((d) => `«${d.chu}» ${d.cat ? `bị cắt: cần ${d.can}px, có ${d.rong}px` : `đọc trọn (${d.rong}px)`}`).join("; ") || "không thấy chữ cần đo" });
        await t.context.close();
      }
    }
    // The three routes side by side at 320dp, boxes drawn, for the report.
    await ghepAnh(mt.browser, CAT.map(([ten]) => ({ file: join(mt.out, "jpg", `EV-F11-CAT-${ten}-C2-ct.jpg`), nhan: `${ten}, C2` })), join(mt.out, "jpg", "EV-F11-CAT-C2-ghep.jpg"), { tieuDe: "F11 route demo ở 320dp: số tiền và nhãn nút bị cắt bằng dấu …" });
  }

  // ------------------------------ leaving the demo from the profile tab
  if (chay("thoat")) {
    const t = await mo("C1", null, "/profile");
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(3000);
    const m = await doMan(t.page);
    // The way out sits in the «Tài khoản» panel of the profile tab.
    const hang = await t.page.evaluate(() => { const e = [...document.querySelectorAll('[role="button"],button')].find((x) => /^Tài khoản/.test((x.getAttribute("aria-label") || x.innerText || "").replace(/[\uE000-\uF8FF]/g, "").trim())); if (!e) return null; e.scrollIntoView({ block: "center" }); const r = e.getBoundingClientRect(); return { x: r.left + r.width / 2, y: r.top + r.height / 2 }; });
    if (hang) await cham(cdp, hang);
    await t.page.waitForTimeout(1500);
    const nut = await t.page.evaluate(() => { const e = [...document.querySelectorAll('[role="button"],button')].find((x) => /Đăng xuất bản trải nghiệm/.test(x.getAttribute("aria-label") || x.innerText || "") && x.getClientRects().length); if (!e) return null; e.scrollIntoView({ block: "center" }); const r = e.getBoundingClientRect(); return { x: r.left + r.width / 2, y: r.top + r.height / 2, w: Math.round(r.width), h: Math.round(r.height) }; });
    const cau = await t.page.evaluate(() => [...document.querySelectorAll("div,span")].find((e) => /Bản trải nghiệm không có phiên đăng nhập/.test(e.textContent ?? "") && e.children.length === 0)?.textContent.trim() ?? null);
    if (nut) await cham(cdp, nut);
    await t.page.waitForTimeout(2500);
    const toi = await duongDan(t.page);
    await chup(t.page, { out: mt.out, id: "EV-F11-THOAT-C1", suKien: t.suKien });
    ket({ tc: "TC-F11-THOAT", screen: "F11.S01", state: "tab Cá nhân demo, chưa đăng nhập", action: "«Tài khoản» → «Đăng xuất bản trải nghiệm»", cauHinh: "C1", expected: "về màn chào, nơi có lối đăng nhập", status: nut && toi === "/welcome" ? "PASS" : "FAIL", evidence: ["EV-F11-THOAT-C1"], ghiChu: `nút ${nut ? `${nut.w}×${nut.h}` : "không thấy"}; câu giải thích «${cau ?? "-"}»; nhãn demo trên tab ${m.demo.join(", ") || "không có"}; sau khi chạm: ${toi}` });
    await t.context.close();
  }
} finally {
  await mt.dong();
}
