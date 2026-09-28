/* F09, profile and settings: the profile tab and its in-screen panels
 * (Tài khoản, Đã lưu, the inline profile form), Settings (avatar, theme,
 * privacy switch and wall-comment chips), sessions, the blocked list, «Về Rủ
 * Đi», and account deletion; plus the same screens cold, failing, without a
 * session, low and wide.
 *
 *   node kich-ban/f09-ho-so.mjs [--chi du-lieu,ho-so,tab-back,da-luu,sua,cai-dat,chan-trang,loi-cai-dat,phien,da-chan,lanh,khong-phien,loi,c8,tablet,xoa]
 *
 * Who writes what, all on the local stack. chat-0 and dalat-0 are only read.
 * moi-54 (a new account) takes every edit: name, bio, city, avatar (synthetic
 * bytes from `pngThuBytes`), theme, the phone-search switch, the wall-comment
 * policy, and one extra session made through the OTP API that the sessions
 * page then signs out. chat-20 lifts the block on chat-21 set in F06. moi-55
 * is made for one purpose and deleted through the two-step page, last; run
 * `--chi xoa` only once per stack. Team Đà Lạt is not touched.
 */
import { randomUUID } from "node:crypto";
import { existsSync, rmSync, writeFileSync } from "node:fs";
import { join } from "node:path";

import { pngThuBytes } from "../../../../apps/mobile/tools/png-thu.mjs";
import { chayAxe } from "../thu-vien/axe.mjs";
import { cauHinh } from "../thu-vien/cau-hinh.mjs";
import { chup } from "../thu-vien/chup.mjs";
import { cdpCua, cham, keo } from "../thu-vien/cu-chi.mjs";
import { choOn, duongDan } from "../thu-vien/dieu-huong.mjs";
import { focusHienTai } from "../thu-vien/lop-phu.mjs";
import { THAN_LOI } from "../thu-vien/mang.mjs";
import { khoiDong, trangMoi } from "../thu-vien/moi-truong.mjs";
import { goiApi, layPhien, personaTheoTen } from "../thu-vien/phien.mjs";
import { soGhi } from "../thu-vien/ghi.mjs";

const chi = (() => {
  const i = process.argv.indexOf("--chi");
  return i === -1 ? null : new Set(process.argv[i + 1].split(","));
})();
const chay = (ten) => !chi || chi.has(ten);
const mt = await khoiDong();
const so = soGhi(mt.out);
const ket = (rec) => {
  so.ghi({ feature: "F09", nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, ...rec });
  console.log(`${rec.status.padEnd(10)} ${rec.tc} ${rec.cauHinh ?? ""} ${rec.ghiChu ?? ""}`);
};
const log = (o) => console.log(JSON.stringify(o));
const phienDir = join(mt.out, "phien");
const ngu = (ms) => new Promise((r) => setTimeout(r, ms));

const P = (ten) => personaTheoTen(ten, mt.chatSessions);
const phienCua = (ten) => layPhien(mt.api, P(ten), phienDir);
const api = (method, path, body, phien) => goiApi(mt.api, method, path, body, phien.token);
/** A call that answers its status instead of throwing (a revoked token is a result here). */
const thu = async (method, path, token, body) => {
  const r = await fetch(`${mt.api}${path}`, { method, headers: { authorization: `Bearer ${token}`, "content-type": "application/json", "idempotency-key": randomUUID() }, body: body === undefined ? undefined : JSON.stringify(body) });
  let json = null;
  try { json = JSON.parse(await r.text()); } catch { /* not JSON */ }
  return { status: r.status, json };
};
const an = (s) => String(s ?? "").replace(/[0-9a-f]{8}-[0-9a-f-]{27}/g, "[id]");

const mo = (cfg, persona, path, them = {}) => trangMoi(mt, cauHinh(cfg), { persona, path, ...them });
const diToi = async (t, path) => {
  await t.page.evaluate((p) => {
    history.pushState({}, "", p);
    dispatchEvent(new PopStateEvent("popstate"));
  }, path);
  await choOn(t.page, { mang: t.mang });
  await t.page.waitForTimeout(900);
};
const NGANG = "for (let n = e.parentElement; n; n = n.parentElement) if (n.scrollLeft && !/(auto|scroll)/.test(getComputedStyle(n).overflowX)) n.scrollLeft = 0;";
const timNut = (page, chu, { vai = '[role="button"],button,[role="link"],a[href],[role="tab"],[role="radio"],[role="checkbox"],[role="switch"]', batDau = false } = {}) =>
  page.evaluate(
    ({ chu, vai, batDau, NGANG }) => {
      // Icon glyphs render as private-use characters inside the button text.
      const ten = (x) => (x.getAttribute("aria-label") || (x.innerText ?? "")).replace(/[-]/g, "").replace(/\s+/g, " ").trim();
      const hien = (x) => x.getClientRects().length > 0 && (x.checkVisibility ? x.checkVisibility({ visibilityProperty: true }) : true);
      const e = [...document.querySelectorAll(vai)].find((x) => (batDau ? ten(x).startsWith(chu) : ten(x) === chu) && hien(x));
      if (!e) return null;
      e.scrollIntoView({ block: "center", behavior: "instant" });
      new Function("e", NGANG)(e);
      const r = e.getBoundingClientRect();
      return { x: r.left + r.width / 2, y: r.top + r.height / 2, w: Math.round(r.width), h: Math.round(r.height), top: Math.round(r.top), ten: ten(e) };
    },
    { chu, vai, batDau, NGANG },
  );
const bam = async (t, cdp, chu, { cho = 900, ...o } = {}) => {
  const n = await timNut(t.page, chu, o);
  if (!n) return null;
  await cham(cdp, n);
  await t.page.waitForTimeout(cho);
  return n;
};
/** Only rendered text counts: the stack keeps earlier screens mounted, hidden. */
const viTriCau = (page, re) =>
  page.evaluate((s) => {
    const R = new RegExp(s);
    const hien = (x) => x.getClientRects().length > 0 && (x.checkVisibility ? x.checkVisibility({ visibilityProperty: true, opacityProperty: true }) : true);
    const e = [...document.querySelectorAll("body *")].find((x) => [...x.childNodes].some((n) => n.nodeType === 3 && R.test(n.textContent)) && hien(x));
    if (!e) return null;
    const r = e.getBoundingClientRect();
    return { top: Math.round(r.top), bottom: Math.round(r.bottom), trongMan: r.bottom > 0 && r.top < innerHeight, text: e.textContent.trim().slice(0, 200) };
  }, re.source);
const chuDu = (page) => page.evaluate(() => (document.body.innerText ?? "").replace(/[-]/g, "").replace(/\s+/g, " "));
const catChu = (s, n = 220) => (s.length > n ? `${s.slice(0, n)}…` : s);
const nhapO = async (page, nhan, chu) => {
  const o = page.locator(`[aria-label="${nhan}"]`).filter({ visible: true }).first();
  if (!(await o.count())) return false;
  await o.click();
  await o.fill(chu);
  return true;
};
const giaTriO = (page, nhan) => page.evaluate((nhan) => [...document.querySelectorAll(`[aria-label="${nhan}"]`)].find((x) => x.getClientRects().length)?.value ?? null, nhan);
const hopNut = (page, chu) =>
  page.evaluate((chu) => {
    const ten = (x) => (x.getAttribute("aria-label") || (x.innerText ?? "")).replace(/[-]/g, "").replace(/\s+/g, " ").trim();
    const e = [...document.querySelectorAll('[role="button"],button')].find((x) => ten(x).startsWith(chu) && x.getClientRects().length);
    if (!e) return null;
    const r = e.getBoundingClientRect();
    const cs = getComputedStyle(e);
    const sau = e.parentElement?.nextElementSibling ?? e.nextElementSibling;
    return { w: Math.round(r.width), h: Math.round(r.height), top: Math.round(r.top), tat: e.getAttribute("aria-disabled") === "true" || e.hasAttribute("disabled"), vien: cs.borderStyle, lyDoSau: (sau?.innerText ?? "").replace(/[-]/g, "").replace(/\s+/g, " ").trim().slice(0, 80) };
  }, chu);
/** Reach the profile tab by the tab bar, so the page has history to go back through. */
const quaTabCaNhan = async (t, cdp) => {
  await t.page.waitForTimeout(1200);
  const n = await bam(t, cdp, "Cá nhân", { cho: 1500, vai: '[role="tab"],[role="link"],a[href],[role="button"]' });
  return n;
};
/** 503 for requests matching `mau` (and `method`, if given); answers a function that lifts it. */
const loi503 = async (page, mau, method) => {
  const h = (r) => (!method || r.request().method() === method ? r.fulfill({ status: 503, contentType: "application/json", body: THAN_LOI }) : r.continue());
  await page.route(mau, h);
  return () => page.unroute(mau, h);
};
const nenTrang = (page) => page.evaluate(() => { const e = document.querySelector('[data-testid$="-screen"]') ?? document.body; let n = e; while (n && getComputedStyle(n).backgroundColor === "rgba(0, 0, 0, 0)") n = n.parentElement; return n ? getComputedStyle(n).backgroundColor : null; });

try {
  // ------------------------------------------------------------------ people
  // moi-54 and moi-55 are made here (OTP through the API, one at a time).
  if (chay("du-lieu")) {
    const kq = {};
    for (const ten of ["moi-54", "moi-55", "chat-20", "chat-0", "dalat-0"]) {
      const p = await phienCua(ten);
      const me = await thu("GET", "/people/me", p.token);
      kq[ten] = { me: me.status, ten: me.json?.display_name ?? null };
      await ngu(7000);
    }
    const p20 = await phienCua("chat-20");
    const chan = await thu("GET", "/people/me/blocked", p20.token);
    log({ duLieu: kq, chat20Chan: (chan.json?.blocked ?? []).map((x) => x.display_name) });
  }

  // ------------------------------------------------ the profile tab (S01, L36)
  if (chay("ho-so")) {
    const t = await mo("C1", P("chat-0"), "/messages");
    const cdp = await cdpCua(t.page);
    const vao = await quaTabCaNhan(t, cdp);
    const truoc = { duong: an(await duongDan(t.page)), hoChieu: !!(await viTriCau(t.page, /Hộ chiếu Rủ Đi/)), dem: await viTriCau(t.page, /bạn bè · .* nhóm · .* kèo/) };
    const hang = await t.page.evaluate(() => [...document.querySelectorAll('[role="button"]')].filter((e) => e.getClientRects().length && e.closest('[data-testid="profile-screen"]')).map((e) => { const r = e.getBoundingClientRect(); return { ten: (e.getAttribute("aria-label") || e.innerText || "").replace(/[-]/g, "").replace(/\s+/g, " ").trim().slice(0, 40), h: Math.round(r.height) }; }));
    await chup(t.page, { out: mt.out, id: "EV-F09-HO-SO-C1", suKien: t.suKien });
    // «Tài khoản»: an in-screen panel, not a route.
    await bam(t, cdp, "Tài khoản", { batDau: true, cho: 1200 });
    const panel = { duong: an(await duongDan(t.page)), dangXem: await viTriCau(t.page, /Đang xem với tư cách/), dangXuat: !!(await timNut(t.page, "Đăng xuất")), banDung: await viTriCau(t.page, /Bản dựng/), oTen: await t.page.evaluate(() => [...document.querySelectorAll("input,textarea")].filter((x) => x.getClientRects().length).length) };
    await chup(t.page, { out: mt.out, id: "EV-F09-TAI-KHOAN-C1", suKien: t.suKien });
    ket({ tc: "TC-F09-HO-SO", screen: "F09.S01", state: "chat-0: 1 nhóm chat-test, bạn bè, kỷ niệm", action: "Tin nhắn → tab «Cá nhân»", cauHinh: "C1", expected: "thẻ hộ chiếu với tên và số đếm của máy chủ; các hàng lối vào ≥ 48dp", status: truoc.hoChieu && truoc.dem && hang.every((h) => h.h >= 48 || /Đổi ảnh/.test(h.ten)) ? "PASS" : "FAIL", evidence: ["EV-F09-HO-SO-C1"], ghiChu: `tới ${truoc.duong}; hộ chiếu ${truoc.hoChieu ? "có" : "không"}; số đếm «${truoc.dem?.text ?? "không thấy"}»; ${hang.length} nút: ${hang.map((h) => `${h.ten} ${h.h}`).join(", ")}` });
    await t.context.close();
    // Back with the panel open, the usual way in: Khám phá, then the tab bar.
    // A first tab switch from the first tab adds one history entry
    // (TC-F00-TAB-BACK), so Back returns to Khám phá; tabs keep their state.
    const tA = await mo("C1", P("chat-0"), "/explore");
    const cdpA = await cdpCua(tA.page);
    await tA.page.waitForTimeout(1500);
    await bam(tA, cdpA, "Cá nhân", { cho: 1500, vai: '[role="tab"],[role="link"],a[href],[role="button"]' });
    await bam(tA, cdpA, "Tài khoản", { batDau: true, cho: 1200 });
    const moA = !!(await timNut(tA.page, "Đăng xuất"));
    await tA.page.goBack();
    await tA.page.waitForTimeout(1500);
    const sauBackA = { duong: an(await duongDan(tA.page)), panelCon: !!(await timNut(tA.page, "Đăng xuất")) };
    await bam(tA, cdpA, "Cá nhân", { cho: 1500, vai: '[role="tab"],[role="link"],a[href],[role="button"]' });
    const laiA = { duong: an(await duongDan(tA.page)), panelCon: !!(await timNut(tA.page, "Đăng xuất")) };
    await tA.context.close();
    // Back with the panel open. A tab switch adds no history entry on the web
    // (history.length read around it), so a Back there leaves the app for
    // whatever came before it. The profile is reached here by pushState from
    // Tin nhắn instead: Back then has an in-app entry, and the app stays loaded.
    const t2 = await mo("C1", P("chat-0"), "/messages");
    const cdp2 = await cdpCua(t2.page);
    await t2.page.waitForTimeout(1200);
    const VAI_TAB = { cho: 1500, vai: '[role="tab"],[role="link"],a[href],[role="button"]' };
    const soLS = () => t2.page.evaluate(() => history.length);
    const h0 = await soLS();
    await bam(t2, cdp2, "Cá nhân", VAI_TAB);
    const hTab = await soLS();
    await bam(t2, cdp2, "Tin nhắn", VAI_TAB);
    await diToi(t2, "/profile");
    const hPush = await soLS();
    await bam(t2, cdp2, "Tài khoản", { batDau: true, cho: 1200 });
    const moPanel = !!(await timNut(t2.page, "Đăng xuất"));
    await t2.page.goBack();
    await t2.page.waitForTimeout(1500);
    const sauBack = { duong: an(await duongDan(t2.page)), panelCon: !!(await timNut(t2.page, "Đăng xuất")), hoChieu: !!(await viTriCau(t2.page, /Hộ chiếu Rủ Đi/)) };
    await bam(t2, cdp2, "Cá nhân", VAI_TAB);
    const lai = { duong: an(await duongDan(t2.page)), panelCon: !!(await timNut(t2.page, "Đăng xuất")) };
    if (!lai.panelCon) await bam(t2, cdp2, "Tài khoản", { batDau: true, cho: 1200 });
    await bam(t2, cdp2, "Quay lại", { cho: 1000 });
    const quayLai = { duong: an(await duongDan(t2.page)), panelCon: !!(await timNut(t2.page, "Đăng xuất")), hoChieu: !!(await viTriCau(t2.page, /Hộ chiếu Rủ Đi/)) };
    // A tab switch with the panel open, and back.
    await bam(t2, cdp2, "Tài khoản", { batDau: true, cho: 1200 });
    await bam(t2, cdp2, "Tin nhắn", VAI_TAB);
    await bam(t2, cdp2, "Cá nhân", VAI_TAB);
    const doiTab = { duong: an(await duongDan(t2.page)), panelCon: !!(await timNut(t2.page, "Đăng xuất")) };
    log({ hoSo: { vao: vao?.ten, truoc, hang, panel, A: { moA, sauBackA, laiA }, lichSu: { h0, hTab, hPush }, moPanel, sauBack, lai, quayLai, doiTab } });
    ket({ tc: "TC-L36-VONGDOI", screen: "F09.S01", layer: "L36", state: "panel «Tài khoản» mở trên Cá nhân; hai đường vào: thanh tab từ Khám phá (A), link /profile từ Tin nhắn (B)", action: "Back trình duyệt; chạm lại tab Cá nhân; «Quay lại» của panel; đổi tab khi panel mở", cauHinh: "C1", expected: "panel có «Quay lại» như một màn con, nên Back đóng panel và ở lại Cá nhân; «Quay lại» về trang Cá nhân", status: moA && /\/profile/.test(sauBackA.duong) && !sauBackA.panelCon && !quayLai.panelCon && quayLai.hoChieu ? "PASS" : "FAIL", evidence: ["EV-F09-TAI-KHOAN-C1"], ghiChu: `panel: «${panel.dangXem?.text ?? "?"}», nút «Đăng xuất» ${panel.dangXuat ? "có" : "không"}, «${panel.banDung?.text ?? "không có dòng bản dựng"}», ${panel.oTen} ô nhập. A (thanh tab): Back → ${sauBackA.duong}, panel ${sauBackA.panelCon ? "còn" : "không còn"}; chạm lại tab Cá nhân: panel ${laiA.panelCon ? "vẫn mở" : "đóng"}. B (link, lịch sử ${h0} → ${hTab} sau khi chạm tab, ${hPush} sau pushState): Back → ${sauBack.duong}, panel ${sauBack.panelCon ? "còn" : "không còn"}; chạm lại tab: panel ${lai.panelCon ? "vẫn mở" : "đóng"}. «Quay lại» của panel: ${quayLai.duong}, ${quayLai.hoChieu ? "thấy hộ chiếu" : "không thấy hộ chiếu"}; đổi tab khi panel mở rồi về: panel ${doiTab.panelCon ? "vẫn mở" : "đóng"}` });
    await t2.context.close();
  }

  // ------------------------- Back after switching tabs (F00.S03, found in F09)
  // The L36 case showed a tab tap leaves history.length unchanged. Where does a
  // Back go after two tab taps? The harness opens every page from its own
  // start page (`/favicon.ico` on the app's origin): landing there means the
  // app itself was left.
  if (chay("tab-back")) {
    const t = await mo("C1", P("chat-0"), "/explore");
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(1500);
    const VAI_TAB = { cho: 1500, vai: '[role="tab"],[role="link"],a[href],[role="button"]' };
    const doc = async () => ({ duong: an(await duongDan(t.page)), ls: await t.page.evaluate(() => history.length) });
    const d0 = await doc();
    await bam(t, cdp, "Tin nhắn", VAI_TAB);
    const d1 = await doc();
    await bam(t, cdp, "Cá nhân", VAI_TAB);
    const d2 = await doc();
    await t.page.goBack();
    await t.page.waitForTimeout(1500);
    const sau = { duong: await t.page.evaluate(() => location.pathname), app: await t.page.evaluate(() => !!document.querySelector('[data-testid$="-screen"]')) };
    log({ tabBack: { d0, d1, d2, sau } });
    ket({ tc: "TC-F00-TAB-BACK", feature: "F00", screen: "F00.S03", layer: "L05", state: "mở /explore, có phiên", action: "chạm tab «Tin nhắn», rồi «Cá nhân», rồi Back trình duyệt", cauHinh: "C1", expected: "Back đưa về tab trước (như Back phần cứng Android với `backBehavior` mặc định về tab đầu), không rời app", status: sau.app && /\/(explore|messages)/.test(sau.duong) ? "PASS" : "FAIL", evidence: [], ghiChu: `history.length: ${d0.ls} ở ${d0.duong}, ${d1.ls} sau «Tin nhắn» (${d1.duong}), ${d2.ls} sau «Cá nhân» (${d2.duong}); Back → ${sau.duong}, app ${sau.app ? "còn" : "không còn trên trang (đã rời app)"}` });
    await t.context.close();
  }

  // --------------------------------------------------- the saved panel (L36)
  if (chay("da-luu")) {
    // moi-54 saves two places through the API, so the panel has something to show.
    const p54 = await phienCua("moi-54");
    for (const id of ["p-an-cafe-da-lat", "p-tiem-nuong-xom-lao"]) await thu("PUT", `/people/me/saved-places/${id}`, p54.token);
    const soLuu = await thu("GET", "/people/me/saved-places", p54.token);
    const t = await mo("C1", P("moi-54"), "/messages");
    const cdp = await cdpCua(t.page);
    await quaTabCaNhan(t, cdp);
    const hangLuu = await timNut(t.page, "Đã lưu", { batDau: true });
    await bam(t, cdp, "Đã lưu", { batDau: true, cho: 1500 });
    const r = { tieuDe: await viTriCau(t.page, /\d+ địa điểm|Đã lưu/), danhSach: await viTriCau(t.page, /Danh sách lưu/), nut: await timNut(t.page, "Mở Khám phá"), chu: catChu(await chuDu(t.page), 300) };
    await chup(t.page, { out: mt.out, id: "EV-F09-DA-LUU-C1", suKien: t.suKien });
    log({ daLuu: { api: soLuu.status, soApi: Array.isArray(soLuu.json?.saved) ? soLuu.json.saved.length : null, hangLuu, r } });
    ket({ tc: "TC-F09-DA-LUU", screen: "F09.S01", layer: "L36", state: `moi-54 lưu ${Array.isArray(soLuu.json?.saved) ? soLuu.json.saved.length : "?"} địa điểm qua API`, action: "tab Cá nhân → «Đã lưu»", cauHinh: "C1", expected: "panel cho thấy các nơi đã lưu (tên, lối mở), không chỉ con số", status: "BLOCKED", evidence: ["EV-F09-DA-LUU-C1"], ghiChu: `hàng «${hangLuu?.ten ?? "?"}»; panel: tiêu đề «${r.tieuDe?.text ?? "?"}», câu «${r.danhSach?.text ?? "không"}», nút «Mở Khám phá» ${r.nut ? "có" : "không"}; chữ trang «${r.chu}» (phán quyết bằng mắt ở f09-phan-xu.mjs)` });
    await t.context.close();
  }

  // ------------------------------------------- the inline profile form (S01)
  if (chay("sua")) {
    const p54 = await phienCua("moi-54");
    const TEN_DAI = "Người Kiểm Thử Có Tên Rất Dài Để Xem Hộ Chiếu Có Xuống Dòng Không 🌿🌿 Và Còn Thêm Một Đoạn Nữa Cho Đủ";
    const BIO = "Thích đi bộ buổi sáng, ăn phở gánh và ngồi quán cà phê nhìn mưa. Cuối tuần hay rủ cả nhóm đi đâu đó gần thành phố, miễn có chỗ ngồi lâu và không phải chờ bàn.";
    const t = await mo("C1", P("moi-54"), "/messages");
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(1200);
    await diToi(t, "/profile");
    await bam(t, cdp, "Chỉnh hồ sơ", { cho: 900 });
    const coForm = { ten: await giaTriO(t.page, "Ô tên hiển thị"), luu: await hopNut(t.page, "Lưu hồ sơ") };
    // Empty name: the sentence belongs next to the field.
    await nhapO(t.page, "Ô tên hiển thị", "");
    await bam(t, cdp, "Lưu hồ sơ", { cho: 900 });
    const loiTen = await viTriCau(t.page, /Tên hiển thị không được rỗng/);
    const oTen = await t.page.evaluate(() => { const o = [...document.querySelectorAll('[aria-label="Ô tên hiển thị"]')].find((x) => x.getClientRects().length); const r = o?.getBoundingClientRect(); return r ? { top: Math.round(r.top), bottom: Math.round(r.bottom) } : null; });
    await chup(t.page, { out: mt.out, id: "EV-F09-SUA-TEN-RONG-C1", suKien: t.suKien });
    ket({ tc: "TC-F09-SUA-TEN-RONG", screen: "F09.S01", state: "form «Chỉnh hồ sơ», tên xoá trắng", action: "«Lưu hồ sơ»", cauHinh: "C1", expected: "câu lỗi ngay cạnh ô tên, trong khung nhìn; không gửi", status: loiTen?.trongMan && oTen && Math.abs(loiTen.top - oTen.bottom) < 60 ? "PASS" : "FAIL", evidence: ["EV-F09-SUA-TEN-RONG-C1"], ghiChu: `form mở với tên «${coForm.ten}», «Lưu hồ sơ» ${coForm.luu ? `${coForm.luu.w}×${coForm.luu.h}` : "không thấy"}; câu «${loiTen?.text ?? "không có"}» ở y ${loiTen?.top ?? "-"}, đáy ô tên y ${oTen?.bottom ?? "-"}` });
    // A long name with emoji, a long bio, a city: saved and drawn on the passport.
    await nhapO(t.page, "Ô tên hiển thị", TEN_DAI);
    await nhapO(t.page, "Ô giới thiệu", BIO);
    await nhapO(t.page, "Ô thành phố", "Thành phố Hồ Chí Minh");
    await bam(t, cdp, "Lưu hồ sơ", { cho: 2500 });
    const me = await thu("GET", "/people/me", p54.token);
    const sauLuu = { form: !!(await timNut(t.page, "Lưu hồ sơ")), ten: await viTriCau(t.page, /Người Kiểm Thử Có Tên Rất Dài/), loi: await viTriCau(t.page, /không|Không|lỗi/) };
    const mDai = await chup(t.page, { out: mt.out, id: "EV-F09-HO-CHIEU-DAI-C1", suKien: t.suKien });
    const tran = mDai.tomTat.tranNgang + mDai.tomTat.chuBiCat;
    ket({ tc: "TC-F09-SUA-DAI", screen: "F09.S01", state: `tên ${TEN_DAI.length} ký tự có emoji, giới thiệu ${BIO.length} ký tự, thành phố`, action: "gõ rồi «Lưu hồ sơ»", cauHinh: "C1", expected: "máy chủ nhận; form đóng; hộ chiếu xuống dòng, không tràn ngang", status: me.json?.display_name === TEN_DAI && !sauLuu.form && sauLuu.ten && tran === 0 ? "PASS" : "FAIL", evidence: ["EV-F09-HO-CHIEU-DAI-C1"], ghiChu: `máy chủ: tên ${me.json?.display_name === TEN_DAI ? "đúng" : `«${catChu(String(me.json?.display_name), 60)}»`}, thành phố «${me.json?.city ?? "-"}»; form ${sauLuu.form ? "còn mở" : "đóng"}; tên trên hộ chiếu ${sauLuu.ten ? `y ${sauLuu.ten.top}–${sauLuu.ten.bottom}` : "không thấy"}; tràn ngang ${mDai.tomTat.tranNgang}, chữ bị cắt ${mDai.tomTat.chuBiCat} (bộ đo của harness)` });
    // Back in the middle of an edit. B: this page, reached by pushState from Tin nhắn.
    await bam(t, cdp, "Chỉnh hồ sơ", { cho: 900 });
    await nhapO(t.page, "Ô giới thiệu", "Đang gõ dở thì bấm Back");
    await t.page.goBack();
    await t.page.waitForTimeout(1500);
    const sauBack = { duong: an(await duongDan(t.page)) };
    await quaTabCaNhan(t, cdp);
    const lai = { form: !!(await timNut(t.page, "Lưu hồ sơ")), bio: await giaTriO(t.page, "Ô giới thiệu") };
    // A: the usual way in, Khám phá then the tab bar.
    const tA = await mo("C1", P("moi-54"), "/explore");
    const cdpA = await cdpCua(tA.page);
    await tA.page.waitForTimeout(1500);
    await bam(tA, cdpA, "Cá nhân", { cho: 1500, vai: '[role="tab"],[role="link"],a[href],[role="button"]' });
    await bam(tA, cdpA, "Chỉnh hồ sơ", { cho: 900 });
    await nhapO(tA.page, "Ô giới thiệu", "Đang gõ dở thì bấm Back");
    await tA.page.goBack();
    await tA.page.waitForTimeout(1500);
    const sauBackA = { duong: an(await duongDan(tA.page)) };
    await bam(tA, cdpA, "Cá nhân", { cho: 1500, vai: '[role="tab"],[role="link"],a[href],[role="button"]' });
    const laiA = { form: !!(await timNut(tA.page, "Lưu hồ sơ")), bio: await giaTriO(tA.page, "Ô giới thiệu") };
    if (laiA.form) await bam(tA, cdpA, "Huỷ", { cho: 800 });
    await tA.context.close();
    const me2 = await thu("GET", "/people/me", p54.token);
    const conNhap = (x) => x.form && x.bio === "Đang gõ dở thì bấm Back";
    ket({ tc: "TC-F09-SUA-BACK", screen: "F09.S01", layer: "L36", state: "form «Chỉnh hồ sơ» đang gõ dở; hai đường vào: thanh tab từ Khám phá (A), link /profile từ Tin nhắn (B)", action: "Back trình duyệt; chạm lại tab Cá nhân", cauHinh: "C1", expected: "Back không bỏ chữ đang gõ mà không hỏi: hoặc ở lại form, hoặc mở lại tab còn nguyên form và chữ", status: conNhap(laiA) && conNhap(lai) ? "PASS" : "FAIL", evidence: [], ghiChu: `A: Back → ${sauBackA.duong}; chạm lại tab: form ${laiA.form ? `còn, ô giới thiệu «${catChu(String(laiA.bio), 40)}»` : "đóng"}. B: Back → ${sauBack.duong}; chạm lại tab: form ${lai.form ? `còn, ô giới thiệu «${catChu(String(lai.bio), 40)}»` : "đóng, chữ đang gõ mất"}. Máy chủ giữ giới thiệu cũ: ${me2.json?.bio === BIO ? "có" : "không"}` });
    // Leave the form the way the screen offers.
    if (lai.form) await bam(t, cdp, "Huỷ", { cho: 800 });
    await t.context.close();
  }

  // ---------------------------------------------------------- settings (S02)
  if (chay("cai-dat")) {
    const p54 = await phienCua("moi-54");
    const t = await mo("C1", P("moi-54"), "/messages");
    const cdp = await cdpCua(t.page);
    await quaTabCaNhan(t, cdp);
    await bam(t, cdp, "Cài đặt", { batDau: true, cho: 2000 });
    const vao = { duong: an(await duongDan(t.page)), focus: (await focusHienTai(t.page))?.ten ?? "body" };
    const aria = await t.page.evaluate(() => {
      const s = document.querySelector('[data-testid="cai-dat-screen"]');
      const tabs = s ? [...s.querySelectorAll('[role="tab"]')].map((e) => ({ ten: e.getAttribute("aria-label"), chon: e.getAttribute("aria-selected"), cha: e.parentElement?.getAttribute("role") ?? null })) : [];
      const sw = s?.querySelector('[role="switch"],input[type="checkbox"]');
      const nhom = s?.querySelector('[role="radiogroup"]');
      return { tabs, switch: sw ? { role: sw.getAttribute("role") ?? sw.type, checked: sw.getAttribute("aria-checked") ?? String(sw.checked), ten: sw.getAttribute("aria-label") } : null, radiogroup: nhom ? [...nhom.children].map((c) => ({ role: c.getAttribute("role"), pressed: c.getAttribute("aria-pressed"), checked: c.getAttribute("aria-checked"), ten: (c.innerText ?? "").replace(/[-]/g, "").trim() })) : null };
    });
    const axe = await chayAxe(t.page).catch(() => ({ vi: [] }));
    await chup(t.page, { out: mt.out, id: "EV-F09-CAI-DAT-C1", suKien: t.suKien });
    log({ caiDat: { vao, aria, axe: (axe.vi ?? []).map((x) => `${x.rule}×${x.so}:${x.vd.map((v) => v.target).join("|").slice(0, 160)}`) } });
    ket({ tc: "TC-F09-CAI-DAT-ARIA", screen: "F09.S02", state: "Cài đặt của moi-54", action: "đọc cây truy cập và axe", cauHinh: "C1", expected: "ba mục Giao diện là tab trong tablist có aria-selected; công tắc có role switch và trạng thái; nhóm «Ai được bình luận» là radio có aria-checked", status: aria.tabs.length && aria.tabs.every((x) => x.cha === "tablist") && aria.radiogroup?.every((c) => c.role === "radio") ? "PASS" : "FAIL", evidence: ["EV-F09-CAI-DAT-C1"], ghiChu: `tab: ${aria.tabs.map((x) => `${x.ten} (aria-selected ${x.chon}, cha role ${x.cha ?? "không"})`).join(", ")}; công tắc: ${aria.switch ? `role ${aria.switch.role}, checked ${aria.switch.checked}, tên «${aria.switch.ten}»` : "không thấy"}; radiogroup chứa: ${aria.radiogroup ? aria.radiogroup.map((c) => `«${c.ten}» role ${c.role}, aria-pressed ${c.pressed}`).join("; ") : "không có"}; axe: ${(axe.vi ?? []).map((x) => `${x.rule}×${x.so}`).join(", ") || "sạch"}; focus sau khi mở ${vao.focus}` });

    // Avatar: the picker closed empty, then a synthetic picture. The frame is
    // the view named after the person (64dp); it holds an <img> or an initial.
    const khungAnh = () => t.page.evaluate(() => {
      const s = document.querySelector('[data-testid="cai-dat-screen"]');
      const f = s ? [...s.querySelectorAll("[aria-label]")].find((e) => { const r = e.getBoundingClientRect(); return Math.round(r.width) === 64 && Math.round(r.height) === 64; }) : null;
      const i = f?.querySelector("img");
      return f ? { ten: (f.getAttribute("aria-label") ?? "").slice(0, 30), anh: i ? { src: (i.currentSrc || i.getAttribute("src") || "").replace(/^.*\/people\/[^/]+/, "…"), nw: i.naturalWidth, nh: i.naturalHeight } : null, chu: (f.innerText ?? "").trim() } : null;
    });
    const truocAnh = await khungAnh();
    const [fc0] = await Promise.all([t.page.waitForEvent("filechooser", { timeout: 6000 }).catch(() => null), bam(t, cdp, "Đổi ảnh đại diện", { cho: 200 })]);
    if (fc0) await fc0.setFiles([]);
    await t.page.waitForTimeout(1500);
    const huyAnh = { moBoChon: !!fc0, loi: await viTriCau(t.page, /[Ll]ỗi|không được|Không được|Kiểm tra mạng/) };
    const doiLan = [];
    for (const [w, h] of [[400, 400], [300, 200]]) {
      const [fc] = await Promise.all([t.page.waitForEvent("filechooser", { timeout: 6000 }).catch(() => null), bam(t, cdp, "Đổi ảnh đại diện", { cho: 200 })]);
      if (fc) await fc.setFiles([{ name: `anh-dai-dien-${w}.png`, mimeType: "image/png", buffer: Buffer.from(pngThuBytes(w, h)) }]);
      await t.page.waitForTimeout(5000);
      doiLan.push({ w, h, khung: await khungAnh() });
    }
    const avatarApi = await fetch(`${mt.api}/people/${p54.person_id}/avatar`, { headers: { authorization: `Bearer ${p54.token}` } });
    await chup(t.page, { out: mt.out, id: "EV-F09-DOI-ANH-C1", suKien: t.suKien });
    const [l1, l2] = doiLan;
    ket({ tc: "TC-F09-DOI-ANH", screen: "F09.S02", layer: "L31", state: `khung ảnh đại diện ${truocAnh?.anh ? "đang có ảnh" : `đang là chữ «${truocAnh?.chu ?? "?"}»`}`, action: "«Đổi ảnh đại diện»: đóng bộ chọn không chọn; rồi chọn ảnh tổng hợp 400×400, rồi 300×200", cauHinh: "C1", expected: "đóng bộ chọn thì không có câu lỗi; mỗi lần chọn thì tải lên và khung đổi ngay sang ảnh mới", status: huyAnh.moBoChon && !huyAnh.loi?.trongMan && avatarApi.status === 200 && l1?.khung?.anh && l2?.khung?.anh && l1.khung.anh.src !== l2.khung.anh.src ? "PASS" : "FAIL", evidence: ["EV-F09-DOI-ANH-C1"], ghiChu: `lượt huỷ: bộ chọn ${huyAnh.moBoChon ? "mở" : "không mở"}, câu ${huyAnh.loi ? `«${huyAnh.loi.text}»` : "không"}; khung trước: ${truocAnh ? (truocAnh.anh ? `ảnh ${truocAnh.anh.nw}×${truocAnh.anh.nh}` : `chữ «${truocAnh.chu}»`) : "không thấy"}; sau ảnh 400×400: ${l1?.khung?.anh ? `ảnh ${l1.khung.anh.nw}×${l1.khung.anh.nh}, «${l1.khung.anh.src.slice(0, 40)}»` : `không ảnh (${l1?.khung?.chu ?? "?"})`}; sau ảnh 300×200: ${l2?.khung?.anh ? `ảnh ${l2.khung.anh.nw}×${l2.khung.anh.nh}, «${l2.khung.anh.src.slice(0, 40)}»` : `không ảnh (${l2?.khung?.chu ?? "?"})`}; GET /people/[id]/avatar ${avatarApi.status}` });

    // Theme: applied at once, kept after a reload, «Hệ thống» follows the browser (light here).
    const nen0 = await nenTrang(t.page);
    await bam(t, cdp, "Tối", { cho: 800, vai: '[role="tab"]' });
    const nenToi = await nenTrang(t.page);
    await t.page.reload();
    await choOn(t.page, { mang: t.mang });
    await t.page.waitForTimeout(1500);
    const nenTaiLai = await nenTrang(t.page);
    const chonSauTaiLai = await t.page.evaluate(() => [...document.querySelectorAll('[role="tab"]')].filter((e) => e.getAttribute("aria-selected") === "true").map((e) => e.getAttribute("aria-label")));
    await bam(t, cdp, "Theo hệ thống", { cho: 800, vai: '[role="tab"]' });
    const nenHeThong = await nenTrang(t.page);
    await bam(t, cdp, "Sáng", { cho: 800, vai: '[role="tab"]' });
    const nenSang = await nenTrang(t.page);
    ket({ tc: "TC-F09-GIAO-DIEN", screen: "F09.S02", state: "trình duyệt ở chế độ sáng", action: "«Tối» → tải lại → «Hệ thống» → «Sáng»", cauHinh: "C1", expected: "đổi nền ngay; giữ lựa chọn sau khi tải lại; «Hệ thống» theo trình duyệt (sáng)", status: nenToi !== nen0 && nenTaiLai === nenToi && nenHeThong === nen0 && nenSang === nen0 ? "PASS" : "FAIL", evidence: [], ghiChu: `nền đầu ${nen0}; sau «Tối» ${nenToi}; sau tải lại ${nenTaiLai} (tab chọn: ${chonSauTaiLai.join(", ") || "không có"}); «Hệ thống» ${nenHeThong}; «Sáng» ${nenSang}` });

    // Privacy: the phone-search switch and the wall-comment chips, each to the server and back.
    const me0 = await thu("GET", "/people/me", p54.token);
    const sw = await timNut(t.page, "Cho tìm theo số điện thoại", { vai: '[role="switch"],input[type="checkbox"],[aria-label="Cho tìm theo số điện thoại"]' });
    if (sw) await cham(cdp, sw);
    await t.page.waitForTimeout(1500);
    const me1 = await thu("GET", "/people/me", p54.token);
    const cauTat = await viTriCau(t.page, /Không ai tìm được bạn theo số điện thoại|Bạn bè nhập đúng số của bạn/);
    if (sw) await cham(cdp, sw);
    await t.page.waitForTimeout(1500);
    const me2 = await thu("GET", "/people/me", p54.token);
    ket({ tc: "TC-F09-TIM-THEO-SO", screen: "F09.S02", state: `«Tìm theo số điện thoại» ${me0.json?.discoverable_by_phone ? "bật" : "tắt"}`, action: "chạm công tắc hai lần", cauHinh: "C1", expected: "mỗi lần chạm đổi trên máy chủ và câu dưới nhãn đổi theo", status: sw && me1.json?.discoverable_by_phone === !me0.json?.discoverable_by_phone && me2.json?.discoverable_by_phone === me0.json?.discoverable_by_phone ? "PASS" : "FAIL", evidence: [], ghiChu: `công tắc ${sw ? `${sw.w}×${sw.h}` : "không thấy"}; máy chủ ${me0.json?.discoverable_by_phone} → ${me1.json?.discoverable_by_phone} → ${me2.json?.discoverable_by_phone}; câu sau lần đầu «${cauTat?.text ?? "?"}»` });
    const chinh0 = me2.json?.wall_comment_policy;
    const chip = await t.page.evaluate(() => { const g = document.querySelector('[role="radiogroup"]'); const cs = g ? [...g.querySelectorAll('[role="button"]')].filter((e) => e.getClientRects().length) : []; const khac = cs.find((e) => e.getAttribute("aria-pressed") !== "true"); if (!khac) return null; khac.scrollIntoView({ block: "center" }); const r = khac.getBoundingClientRect(); return { x: r.left + r.width / 2, y: r.top + r.height / 2, ten: (khac.innerText ?? "").replace(/[-]/g, "").trim(), h: Math.round(r.height) }; });
    if (chip) await cham(cdp, chip);
    await t.page.waitForTimeout(1500);
    const me3 = await thu("GET", "/people/me", p54.token);
    if (chinh0 && me3.json?.wall_comment_policy !== chinh0) await thu("PATCH", "/people/me", p54.token, { wall_comment_policy: chinh0 });
    ket({ tc: "TC-F09-CHINH-SACH-BL", screen: "F09.S02", state: `«Ai được bình luận tường tôi» đang «${chinh0}»`, action: "chạm một chip khác", cauHinh: "C1", expected: "máy chủ đổi chính sách; chip mới được đánh dấu", status: chip && me3.json?.wall_comment_policy && me3.json.wall_comment_policy !== chinh0 ? "PASS" : "FAIL", evidence: [], ghiChu: `chip «${chip?.ten ?? "?"}» cao ${chip?.h ?? "-"}; máy chủ ${chinh0} → ${me3.json?.wall_comment_policy}` });
    await t.context.close();
  }

  // ------------------------------- the last line of Settings points somewhere
  // «Đăng nhập, đăng xuất và tên hiển thị vẫn nằm ở mục Tài khoản trên màn Cá
  // nhân.» Read it, then count the fields in that «Tài khoản» panel.
  if (chay("chan-trang")) {
    const t = await mo("C1", P("moi-54"), "/explore");
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(1500);
    await bam(t, cdp, "Cá nhân", { cho: 1500, vai: '[role="tab"],[role="link"],a[href],[role="button"]' });
    await bam(t, cdp, "Cài đặt", { batDau: true, cho: 2000 });
    const cau = await t.page.evaluate(() => { const e = [...document.querySelectorAll("div,span")].find((x) => x.children.length === 0 && /vẫn nằm ở mục Tài khoản/.test(x.textContent ?? "") && x.getClientRects().length); if (!e) return null; e.scrollIntoView({ block: "center" }); return e.textContent.trim(); });
    await t.page.waitForTimeout(500);
    await chup(t.page, { out: mt.out, id: "EV-F09-CHAN-TRANG-C1", suKien: t.suKien });
    await t.page.goBack();
    await t.page.waitForTimeout(1500);
    await bam(t, cdp, "Tài khoản", { batDau: true, cho: 1200 });
    const panel = { oNhap: await t.page.evaluate(() => [...document.querySelectorAll('[data-testid="profile-screen"] input,[data-testid="profile-screen"] textarea')].filter((x) => x.getClientRects().length).length), nut: await t.page.evaluate(() => [...document.querySelectorAll('[role="button"]')].filter((e) => e.getClientRects().length && e.closest('[data-testid="profile-screen"]')).map((e) => (e.innerText ?? e.getAttribute("aria-label") ?? "").replace(/[-]/g, "").trim()).filter(Boolean)) };
    await bam(t, cdp, "Quay lại", { cho: 900 });
    const suaTen = await timNut(t.page, "Chỉnh hồ sơ");
    ket({ tc: "TC-F09-CHU-CHAN-TRANG", screen: "F09.S02", state: "Cài đặt của moi-54", action: "đọc câu cuối trang Cài đặt; mở panel «Tài khoản» của Cá nhân", cauHinh: "C1", expected: "câu chỉ đúng chỗ đổi tên hiển thị", status: cau && panel.oNhap === 0 && suaTen ? "FAIL" : "PASS", evidence: ["EV-F09-CHAN-TRANG-C1"], ghiChu: `câu «${cau ?? "không thấy"}»; panel «Tài khoản»: ${panel.oNhap} ô nhập, nút ${panel.nut.map((x) => `«${x}»`).join(", ") || "không"}; tên đổi ở «Chỉnh hồ sơ» trên thẻ hộ chiếu (${suaTen ? "có nút" : "không thấy nút"})` });
    await t.context.close();
  }

  // ----------------------------------- a failing privacy save (S02, like UI-095)
  if (chay("loi-cai-dat")) {
    const t = await mo("C1", P("moi-54"), "/messages");
    const cdp = await cdpCua(t.page);
    await quaTabCaNhan(t, cdp);
    await bam(t, cdp, "Cài đặt", { batDau: true, cho: 2000 });
    const tat = await loi503(t.page, /\/people\/me$/, "PATCH");
    const sw = await timNut(t.page, "Cho tìm theo số điện thoại", { vai: '[role="switch"],input[type="checkbox"],[aria-label="Cho tìm theo số điện thoại"]' });
    if (sw) await cham(cdp, sw);
    await t.page.waitForTimeout(1800);
    const cau = await viTriCau(t.page, /gặp sự cố|thử lại|Thử lại|Kiểm tra mạng/);
    const cao = await t.page.evaluate(() => innerHeight);
    await chup(t.page, { out: mt.out, id: "EV-F09-LOI-CONG-TAC-C1", suKien: t.suKien });
    await tat?.();
    ket({ tc: "TC-F09-LOI-CONG-TAC", screen: "F09.S02", state: "máy chủ trả 503 cho PATCH /people/me", action: "chạm «Tìm theo số điện thoại»", cauHinh: "C1", expected: "câu lỗi ngay gần công tắc, trong khung nhìn; công tắc giữ trạng thái cũ", status: cau?.trongMan && sw && Math.abs(cau.top - sw.y) < 200 ? "PASS" : "FAIL", evidence: ["EV-F09-LOI-CONG-TAC-C1"], ghiChu: `công tắc ở y ${sw ? Math.round(sw.y) : "-"}; câu «${cau?.text ?? "không có"}» ở y ${cau?.top ?? "-"} (cửa sổ cao ${cao})` });
    await t.context.close();
  }

  // --------------------------------------------------------- sessions (S03)
  if (chay("phien")) {
    const p54 = await phienCua("moi-54");
    // One more session for moi-54, straight through the OTP API.
    const P54 = P("moi-54");
    await ngu(7000);
    const ch = await thu("POST", "/auth/otp/request", "", { phone: P54.phone });
    const s2 = ch.status < 300 ? await thu("POST", "/auth/otp/verify", "", { phone: P54.phone, challenge_id: ch.json?.challenge_id, code: "000000" }) : null;
    const token2 = s2?.json?.token ?? null;
    const truocApi = await thu("GET", "/sessions", p54.token);
    const t = await mo("C1", P("moi-54"), "/messages");
    const cdp = await cdpCua(t.page);
    await quaTabCaNhan(t, cdp);
    await bam(t, cdp, "Cài đặt", { batDau: true, cho: 1800 });
    await bam(t, cdp, "Phiên đăng nhập", { batDau: true, cho: 2000 });
    const focusPhien = (await focusHienTai(t.page))?.ten ?? "body";
    const doc = () => t.page.evaluate(() => { const s = document.querySelector('[data-testid="phien-screen"]'); return s ? { hang: [...s.querySelectorAll("div")].filter((e) => /Phiên này, đang dùng| · từ /.test(e.textContent ?? "") && e.children.length === 0).map((e) => e.textContent.trim()), nut: [...s.querySelectorAll('[role="button"]')].filter((e) => /Đăng xuất phiên này/.test(e.innerText ?? "") && e.getClientRects().length).map((e) => { const r = e.getBoundingClientRect(); return `${Math.round(r.width)}×${Math.round(r.height)}`; }), dau: /PHIÊN NÀY/i.test(s.innerText ?? "") } : null; });
    const truoc = await doc();
    await chup(t.page, { out: mt.out, id: "EV-F09-PHIEN-C1", suKien: t.suKien });
    const nut = await timNut(t.page, "Đăng xuất phiên này");
    if (nut) await cham(cdp, nut);
    await t.page.waitForTimeout(2500);
    const sau = await doc();
    const conTok1 = await thu("GET", "/people/me", p54.token);
    const conTok2 = token2 ? await thu("GET", "/people/me", token2) : null;
    // A revoked harness token must not be reused by the next section.
    if (conTok1.status === 401) rmSync(join(phienDir, "moi-54.json"), { force: true });
    log({ phien: { tao2: s2?.status, truocApi: truocApi.json?.sessions?.length, truoc, sau, tok1: conTok1.status, tok2: conTok2?.status } });
    ket({ tc: "TC-F09-PHIEN", screen: "F09.S03", state: `moi-54 có ${truocApi.json?.sessions?.length ?? "?"} phiên (thêm một phiên qua OTP)`, action: "Cá nhân → Cài đặt → «Phiên đăng nhập» → «Đăng xuất phiên này» ở hàng đầu tiên có nút", cauHinh: "C1", expected: "phiên đang dùng có dấu, không có nút thu hồi; thu hồi một phiên khác thì hàng biến mất và token đó hết dùng được", status: truoc?.dau && sau && sau.hang.length === truoc.hang.length - 1 && (conTok1.status === 401 || conTok2?.status === 401) ? "PASS" : "FAIL", evidence: ["EV-F09-PHIEN-C1"], ghiChu: `trước: ${truoc?.hang.length ?? "?"} hàng (${truoc?.hang.join(" | ") ?? ""}), ${truoc?.nut.length ?? 0} nút ${truoc?.nut.join(", ") ?? ""}, dấu «PHIÊN NÀY» ${truoc?.dau ? "có" : "không"}; sau một chạm: ${sau?.hang.length ?? "?"} hàng, không hỏi; token harness → ${conTok1.status}, token thêm → ${conTok2?.status ?? "-"}; focus sau khi mở màn «${focusPhien}»` });
    await t.context.close();
  }

  // --------------------------------------------------- the blocked list (S04)
  if (chay("da-chan")) {
    const p20 = await phienCua("chat-20");
    const p21 = await phienCua("chat-21");
    const truocApi = await thu("GET", "/people/me/blocked", p20.token);
    const t = await mo("C1", P("chat-20"), "/messages");
    const cdp = await cdpCua(t.page);
    await quaTabCaNhan(t, cdp);
    await bam(t, cdp, "Cài đặt", { batDau: true, cho: 1800 });
    await bam(t, cdp, "Người đã chặn", { batDau: true, cho: 2000 });
    const focusChan = (await focusHienTai(t.page))?.ten ?? "body";
    const truoc = { hang: await viTriCau(t.page, /Chặn từ/), nut: await t.page.evaluate(() => [...document.querySelectorAll('[aria-label^="Bỏ chặn "]')].filter((e) => e.getClientRects().length).map((e) => { const r = e.getBoundingClientRect(); return { ten: e.getAttribute("aria-label"), w: Math.round(r.width), h: Math.round(r.height) }; })) };
    await chup(t.page, { out: mt.out, id: "EV-F09-DA-CHAN-C1", suKien: t.suKien });
    const n = truoc.nut[0];
    if (n) await bam(t, cdp, n.ten, { cho: 2500 });
    const sau = { rong: await viTriCau(t.page, /Bạn chưa chặn ai/), hop: await t.page.evaluate(() => [...document.querySelectorAll('[role="dialog"],[role="alertdialog"]')].filter((x) => x.getClientRects().length).length) };
    await chup(t.page, { out: mt.out, id: "EV-F09-DA-CHAN-SAU-C1", suKien: t.suKien });
    const sauApi = await thu("GET", "/people/me/blocked", p20.token);
    const ban = await thu("GET", `/people/${p20.person_id}/friends`, p20.token);
    const laBan = JSON.stringify(ban.json ?? {}).includes(p21.person_id);
    log({ daChan: { truocApi: (truocApi.json?.blocked ?? []).map((x) => x.display_name), truoc, sau, sauApi: (sauApi.json?.blocked ?? []).length, banSauBoChan: { status: ban.status, laBan } } });
    ket({ tc: "TC-F09-BO-CHAN", screen: "F09.S04", state: `chat-20 đang chặn ${truocApi.json?.blocked?.map((x) => x.display_name).join(", ") || "không ai"} (từ F06)`, action: "Cá nhân → Cài đặt → «Người đã chặn» → «Bỏ chặn»", cauHinh: "C1", expected: "hàng có tên và ngày chặn; bỏ chặn xong danh sách về trạng thái rỗng và máy chủ không còn chặn", status: truoc.hang && n && sau.rong && (sauApi.json?.blocked ?? []).length === 0 ? "PASS" : "FAIL", evidence: ["EV-F09-DA-CHAN-C1", "EV-F09-DA-CHAN-SAU-C1"], ghiChu: `hàng «${truoc.hang?.text ?? "không thấy"}»; nút ${truoc.nut.map((x) => `«${x.ten}» ${x.w}×${x.h}`).join(", ") || "không"}; sau một chạm: ${sau.hop} hộp thoại, «${sau.rong?.text ?? "không về rỗng"}»; máy chủ còn ${(sauApi.json?.blocked ?? []).length} người bị chặn; sau khi bỏ chặn hai người ${laBan ? "vẫn là bạn" : "không còn là bạn"} (GET friends ${ban.status}); focus sau khi mở màn «${focusChan}»` });
    await t.context.close();
  }

  // ---------------------------------------- cold opens: «Quay lại» (UI-018)
  if (chay("lanh")) {
    const kq = [];
    for (const [id, path] of [["cai-dat", "/settings"], ["phien", "/settings/phien"], ["da-chan", "/settings/da-chan"], ["ve-rudi", "/settings/ve-rudi"], ["xoa", "/settings/xoa-tai-khoan"]]) {
      const t = await mo("C1", P("moi-54"), path);
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(1500);
      const truoc = an(await duongDan(t.page));
      const nut = await bam(t, cdp, "Quay lại", { cho: 1200 });
      const sau = an(await duongDan(t.page));
      let oLai = null;
      if (id === "xoa") {
        const n2 = await bam(t, cdp, "Ở lại", { cho: 1200 });
        oLai = { co: !!n2, sau: an(await duongDan(t.page)) };
      }
      kq.push({ id, truoc, nut: !!nut, sau, oLai });
      ket({ tc: `TC-F09-BACK-LANH-${id.toUpperCase()}`, screen: { "cai-dat": "F09.S02", phien: "F09.S03", "da-chan": "F09.S04", "ve-rudi": "F09.S05", xoa: "F09.S06" }[id], state: "mở thẳng bằng link, có phiên", action: id === "xoa" ? "«Quay lại», rồi «Ở lại»" : "«Quay lại»", cauHinh: "C1", expected: "về một màn có nghĩa (Cài đặt hoặc Cá nhân), không đứng yên", status: nut && sau !== truoc && (!oLai || oLai.sau !== truoc) ? "PASS" : "FAIL", issue: nut && sau === truoc ? "UI-018" : null, evidence: [], ghiChu: `${truoc} → ${sau}${oLai ? `; «Ở lại» ${oLai.co ? `→ ${oLai.sau}` : "không thấy"}` : ""}` });
      await t.context.close();
    }
    log({ lanh: kq });
  }

  // ------------------------------------------------ no session (UI-082 family)
  if (chay("khong-phien")) {
    const kq = [];
    for (const [id, path] of [["cai-dat", "/settings"], ["phien", "/settings/phien"], ["da-chan", "/settings/da-chan"], ["xoa", "/settings/xoa-tai-khoan"]]) {
      const t = await mo("C1", null, path);
      await t.page.waitForTimeout(8000);
      const den = an(await duongDan(t.page));
      const chu = catChu(await chuDu(t.page), 200);
      const khung = await t.page.evaluate(() => [...document.querySelectorAll('[role="progressbar"][aria-label="Đang tải"]')].filter((e) => e.getClientRects().length).length);
      await chup(t.page, { out: mt.out, id: `EV-F09-KHONG-PHIEN-${id}-C1`, suKien: t.suKien });
      // A list page is done when it shows rows, its empty state or an error.
      // `SkeletonRow` used bare carries no role, so «still loading» is read as
      // none of those after 8 s (the capture shows the grey shapes).
      const xong = /Phiên này|Chưa có phiên|Chưa đọc được|Bạn chưa chặn ai|Chặn từ|Bỏ chặn/.test(chu);
      kq.push({ id, den, chu, khung, xong });
      await t.context.close();
    }
    log({ khongPhien: kq });
    const tot = (x) => /\/welcome|\/login/.test(x.den) || /Dữ liệu demo|Đăng nhập/.test(x.chu);
    ket({ tc: "TC-F09-KHONG-PHIEN", screen: "F09.S02", state: "không có phiên", action: "mở thẳng /settings, /settings/phien, /settings/da-chan, /settings/xoa-tai-khoan", cauHinh: "C1", expected: "tới cửa vào, hoặc nói rõ cần đăng nhập; không màn trống hay khung chờ mãi", status: kq.every(tot) ? "PASS" : "FAIL", evidence: kq.map((x) => `EV-F09-KHONG-PHIEN-${x.id}-C1`), ghiChu: kq.map((x) => `${x.id}: sau 8 s ở ${x.den}${x.id === "phien" || x.id === "da-chan" ? (x.xong ? ", danh sách đã ra" : ", không có hàng, trạng thái rỗng hay câu lỗi (ảnh: khung chờ xám)") : ""}, «${x.chu}»`).join(" | ") });
  }

  // ----------------------------------------------------------- 503 on reads
  if (chay("loi")) {
    const kq = [];
    for (const [id, path, mau, tieuDe] of [["ho-so", "/profile", /\/people\/me$/, /Chưa đọc được hồ sơ/], ["phien", "/settings/phien", /\/sessions$/, /Chưa đọc được danh sách phiên/], ["da-chan", "/settings/da-chan", /\/people\/me\/blocked$/, /Chưa đọc được danh sách/]]) {
      const t = await mo("C1", P("moi-54"), "/messages");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(1000);
      const tat = await loi503(t.page, mau);
      await diToi(t, path);
      await t.page.waitForTimeout(1500);
      const khiLoi = { cau: await viTriCau(t.page, tieuDe), thuLai: !!(await timNut(t.page, "Thử lại")) };
      if (id === "ho-so") await chup(t.page, { out: mt.out, id: "EV-F09-503-ho-so-C1", suKien: t.suKien });
      await tat?.();
      await bam(t, cdp, "Thử lại", { cho: 2000 });
      const sau = { conLoi: !!(await viTriCau(t.page, tieuDe)), chu: catChu(await chuDu(t.page), 120) };
      kq.push({ id, khiLoi, sau });
      ket({ tc: `TC-F09-503-${id.toUpperCase()}`, screen: { "ho-so": "F09.S01", phien: "F09.S03", "da-chan": "F09.S04" }[id], state: "máy chủ trả 503 cho lượt đọc", action: "mở màn; rồi «Thử lại» khi máy chủ ổn", cauHinh: "C1", expected: "câu tiếng Việt và «Thử lại»; thử lại thì nội dung về", status: khiLoi.cau?.trongMan && khiLoi.thuLai && !sau.conLoi ? "PASS" : "FAIL", evidence: id === "ho-so" ? ["EV-F09-503-ho-so-C1"] : [], ghiChu: `khi 503: «${khiLoi.cau?.text ?? "không có câu"}», «Thử lại» ${khiLoi.thuLai ? "có" : "không"}; sau thử lại: «${sau.chu}»` });
      await t.context.close();
    }
  }

  // ---------------------------------------------------------------- C8
  if (chay("c8")) {
    const kq = {};
    {
      const t = await mo("C8", P("moi-54"), "/messages");
      const cdp = await cdpCua(t.page);
      await quaTabCaNhan(t, cdp);
      await bam(t, cdp, "Chỉnh hồ sơ", { cho: 900 });
      const vp = t.page.viewportSize();
      // Small drags, reading the button after each: is it ever whole on screen?
      const buoc = [];
      for (let i = 0; i <= 10; i += 1) {
        const n = await hopNut(t.page, "Lưu hồ sơ");
        buoc.push(n ? n.top : null);
        if (n && n.top >= 0 && n.top + n.h <= vp.height) { kq.sua = { ...n, buoc: i }; break; }
        // Finger down scrolls the page back up (the button is above), finger up scrolls on.
        const len = n && n.top < 0 ? 100 : -100;
        await keo(cdp, { x: 8, y: Math.round(vp.height * 0.5) }, { x: 8, y: Math.round(vp.height * 0.5) + len }, { ms: 250, buoc: 10 });
        await t.page.waitForTimeout(250);
      }
      kq.suaBuoc = buoc;
      kq.cao = vp.height;
      await chup(t.page, { out: mt.out, id: "EV-F09-C8-SUA", suKien: t.suKien });
      if (await timNut(t.page, "Huỷ")) await bam(t, cdp, "Huỷ", { cho: 600 });
      await t.context.close();
    }
    {
      const t = await mo("C8", P("moi-54"), "/messages");
      const cdp = await cdpCua(t.page);
      await diToi(t, "/settings/xoa-tai-khoan");
      await bam(t, cdp, "Tôi hiểu, tiếp tục", { cho: 900 });
      await nhapO(t.page, "Ô xác nhận xoá", "XOA");
      await t.page.waitForTimeout(400);
      kq.xoa = await hopNut(t.page, "Xoá vĩnh viễn");
      await chup(t.page, { out: mt.out, id: "EV-F09-C8-XOA", suKien: t.suKien });
      await t.context.close();
    }
    log({ c8: kq });
    const trong = (n) => n && n.top >= 0 && n.top + n.h <= kq.cao;
    ket({ tc: "TC-F09-C8", screen: "F09.S01", state: "cửa sổ 390×460 (proxy bàn phím)", action: "form «Chỉnh hồ sơ» (kéo trang từng 100px); bước 2 của xoá tài khoản, đã gõ XOA", cauHinh: "C8", expected: "nút chính thấy được hoặc cuộn tới được", status: trong(kq.sua) && trong(kq.xoa) ? "PASS" : "FAIL", evidence: ["EV-F09-C8-SUA", "EV-F09-C8-XOA"], ghiChu: `«Lưu hồ sơ» ${kq.sua ? `trọn trong cửa sổ sau ${kq.sua.buoc} cú kéo 100px, y ${kq.sua.top}, ${kq.sua.w}×${kq.sua.h}` : `không lúc nào trọn trong cửa sổ (đỉnh nút theo từng bước: ${kq.suaBuoc.join(", ")})`}; «Xoá vĩnh viễn» ${kq.xoa ? `y ${kq.xoa.top}, ${kq.xoa.tat ? "tắt" : "bật"}` : "không thấy"} (bước 2 chưa bấm; cửa sổ ${kq.cao})` });
  }

  // ------------------------------------------------------------- tablet
  if (chay("tablet")) {
    for (const cfg of ["C6", "C7"]) {
      const t = await mo(cfg, P("chat-0"), "/profile");
      await t.page.waitForTimeout(1500);
      const r = await t.page.evaluate(() => { const hang = [...document.querySelectorAll('[data-testid="profile-screen"] [role="button"]')].filter((e) => e.getClientRects().length).map((e) => Math.round(e.getBoundingClientRect().width)); const hc = [...document.querySelectorAll("div")].find((e) => (e.textContent ?? "").startsWith("Hộ chiếu Rủ Đi") && e.getClientRects().length); return { hang: [...new Set(hang)].slice(0, 4), hoChieu: hc ? Math.round(hc.getBoundingClientRect().width) : null, cua: innerWidth }; });
      await chup(t.page, { out: mt.out, id: `EV-F09-TABLET-${cfg}`, suKien: t.suKien });
      ket({ tc: "TC-F09-TABLET", screen: "F09.S01", state: "chat-0", action: "tab Cá nhân ở tablet", cauHinh: cfg, expected: "cột nội dung có trần bề ngang (≤ 640) như các form", status: r.hoChieu !== null && r.hoChieu <= 680 ? "PASS" : "FAIL", evidence: [`EV-F09-TABLET-${cfg}`], ghiChu: `cửa sổ ${r.cua}; thẻ hộ chiếu rộng ${r.hoChieu}; hàng rộng ${r.hang.join("/")}` });
      await t.context.close();
    }
  }

  // ------------------------------------------ account deletion (S06, L27), last
  if (chay("xoa")) {
    const p55 = await phienCua("moi-55");
    const truocMe = await thu("GET", "/people/me", p55.token);
    const t = await mo("C1", P("moi-55"), "/messages");
    const cdp = await cdpCua(t.page);
    await quaTabCaNhan(t, cdp);
    await bam(t, cdp, "Cài đặt", { batDau: true, cho: 1800 });
    await bam(t, cdp, "Xoá tài khoản", { batDau: true, cho: 1800 });
    const focusXoa = (await focusHienTai(t.page))?.ten ?? "body";
    const buoc1 = { duong: an(await duongDan(t.page)), tieuDe: await viTriCau(t.page, /Xoá tài khoản là vĩnh viễn/), cau: await t.page.evaluate(() => (document.querySelector('[data-testid="xoa-tai-khoan-screen"]')?.innerText ?? "").replace(/[-]/g, "").replace(/\s+/g, " ").slice(0, 400)) };
    await chup(t.page, { out: mt.out, id: "EV-F09-XOA-B1-C1", suKien: t.suKien });
    await bam(t, cdp, "Tôi hiểu, tiếp tục", { cho: 900 });
    const buoc2 = { o: !!(await t.page.locator('[aria-label="Ô xác nhận xoá"]').filter({ visible: true }).count()), nut: await hopNut(t.page, "Xoá vĩnh viễn") };
    // Back at step 2: step 1 again, or out of the page?
    await t.page.goBack();
    await t.page.waitForTimeout(1500);
    const sauBack = { duong: an(await duongDan(t.page)), buoc1: !!(await viTriCau(t.page, /Xoá tài khoản là vĩnh viễn/)) };
    if (!/xoa-tai-khoan/.test(sauBack.duong)) await bam(t, cdp, "Xoá tài khoản", { batDau: true, cho: 1800 });
    if (!(await t.page.locator('[aria-label="Ô xác nhận xoá"]').filter({ visible: true }).count())) await bam(t, cdp, "Tôi hiểu, tiếp tục", { cho: 900 });
    // The word as the rest of the page spells it, then as the prompt spells it.
    const thuTu = {};
    for (const tu of ["xoá", "XOÁ", "xoa", "XOA"]) {
      await nhapO(t.page, "Ô xác nhận xoá", tu);
      await t.page.waitForTimeout(300);
      const n = await hopNut(t.page, "Xoá vĩnh viễn");
      thuTu[tu] = n ? { tat: n.tat, lyDo: n.lyDoSau } : null;
      if (tu === "XOÁ") await chup(t.page, { out: mt.out, id: "EV-F09-XOA-GO-DAU-C1", suKien: t.suKien });
    }
    const nhacChu = await viTriCau(t.page, /Gõ .* vào ô dưới/);
    const phOld = await t.page.evaluate(() => { const o = [...document.querySelectorAll('[aria-label="Ô xác nhận xoá"]')].find((x) => x.getClientRects().length); return o ? o.getAttribute("placeholder") : null; });
    ket({ tc: "TC-L27-VONGDOI", screen: "F09.S06", layer: "L27", state: "moi-55, tài khoản dùng một lần", action: "Cài đặt → «Xoá tài khoản» → «Tôi hiểu, tiếp tục» → Back trình duyệt; gõ «xoá», «XOÁ», «xoa», «XOA»", cauHinh: "C1", expected: "bước 1 nói cái gì mất; Back ở bước 2 về bước 1 (hoặc rời mà không mất gì); nút xoá chỉ bật khi gõ đúng, và khi chưa bật thì nói vì sao", status: buoc1.tieuDe && buoc2.o && /xoa-tai-khoan/.test(sauBack.duong) && thuTu["XOA"] && !thuTu["XOA"].tat && thuTu["XOÁ"]?.tat && thuTu["XOÁ"]?.lyDo ? "PASS" : "FAIL", evidence: ["EV-F09-XOA-B1-C1", "EV-F09-XOA-GO-DAU-C1"], ghiChu: `bước 1 ở ${buoc1.duong} (focus «${focusXoa}»): «${catChu(buoc1.cau, 260)}»; bước 2: ô ${buoc2.o ? "có" : "không"}, placeholder «${phOld}», câu nhắc «${nhacChu?.text ?? "?"}»; Back ở bước 2 → ${sauBack.duong} (${sauBack.buoc1 ? "thấy bước 1" : "không thấy bước 1"}); nút «Xoá vĩnh viễn» với ${Object.entries(thuTu).map(([k, v]) => `«${k}»: ${v ? `${v.tat ? "tắt" : "bật"}, dòng dưới «${v.lyDo || "không"}»` : "?"}`).join("; ")}` });
    // The deletion itself.
    await nhapO(t.page, "Ô xác nhận xoá", "XOA");
    await t.page.waitForTimeout(300);
    await bam(t, cdp, "Xoá vĩnh viễn", { cho: 4000 });
    const sauXoa = { duong: an(await duongDan(t.page)) };
    const meSau = await thu("GET", "/people/me", p55.token);
    await t.page.reload();
    await choOn(t.page, { mang: t.mang });
    await t.page.waitForTimeout(1500);
    const sauTaiLai = an(await duongDan(t.page));
    await chup(t.page, { out: mt.out, id: "EV-F09-XOA-SAU-C1", suKien: t.suKien });
    if (meSau.status === 401 || meSau.status === 404) rmSync(join(phienDir, "moi-55.json"), { force: true });
    ket({ tc: "TC-F09-XOA-TAI-KHOAN", screen: "F09.S06", layer: "L27", state: `moi-55 (GET /people/me trước: ${truocMe.status})`, action: "gõ XOA → «Xoá vĩnh viễn»; tải lại trang", cauHinh: "C1", expected: "tài khoản bị xoá trên máy chủ, phiên hết; về màn chào; tải lại vẫn ở ngoài", status: /\/welcome/.test(sauXoa.duong) && meSau.status >= 400 && /\/welcome|\/login/.test(sauTaiLai) ? "PASS" : "FAIL", evidence: ["EV-F09-XOA-SAU-C1"], ghiChu: `sau khi xoá ở ${sauXoa.duong}; GET /people/me bằng token cũ → ${meSau.status}; tải lại → ${sauTaiLai}` });
    writeFileSync(join(mt.out, "moi-55-da-xoa.txt"), "moi-55 đã xoá qua UI ở F09\n");
    await t.context.close();
  }
} finally {
  await mt.dong();
}
void existsSync;
