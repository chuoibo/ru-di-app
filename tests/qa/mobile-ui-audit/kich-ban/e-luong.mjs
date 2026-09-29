/* E1–E6, journeys across features: each one walks the app the way a person
 * does, by its own buttons, and checks that what one screen wrote is what the
 * next screen shows. Screens already audited one by one (F00–F11) are not
 * measured again here; a finding that repeats one of theirs points at the
 * existing issue.
 *
 *   node kich-ban/e-luong.mjs [--chi e1-vao,e1-nhom,e1-moi,e1-tin,e1-dem,e2-tao,e2-chang,e2-album,e3,e3-sau,e5-ban,e4,e5-chan,e5-chan-so,e5-ghep,e6-thoat,e6-khong-phien,e6-co-phien]
 *
 * Order matters: e1 opens the group every later journey uses, e5-ban makes
 * the two people friends for e4, e5-chan blocks after e4 on purpose. The
 * first two parts of e1 write accounts and a group once: run again, they
 * find them and stop without writing a row.
 *
 * Who writes what, all on the local stack:
 *  - moi-56 (A) and moi-57 (B), two brand-new accounts, sign in through the UI;
 *    A opens «Nhóm E1 …» and invites B's number; B accepts and writes.
 *  - Every later journey writes only inside that group or between A and B.
 * Team Đà Lạt and the chat-test group are not touched.
 */
import { spawnSync } from "node:child_process";
import { existsSync, unlinkSync } from "node:fs";
import { join } from "node:path";

import { cauHinh } from "../thu-vien/cau-hinh.mjs";
import { chup, ghepAnh } from "../thu-vien/chup.mjs";
import { cdpCua, cham, tamCua } from "../thu-vien/cu-chi.mjs";
import { choOn, duongDan } from "../thu-vien/dieu-huong.mjs";
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
  so.ghi({ nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, ...rec });
  console.log(`${rec.status.padEnd(10)} ${rec.tc} ${rec.cauHinh ?? ""} ${rec.ghiChu ?? ""}`);
};
const log = (o) => console.log(JSON.stringify(o));
const phienDir = join(mt.out, "phien");
const P = (ten) => personaTheoTen(ten, mt.chatSessions);
const phienCua = (ten) => layPhien(mt.api, P(ten), phienDir);
const api = (method, path, body, phien) => goiApi(mt.api, method, path, body, phien.token);
const an = (s) => String(s ?? "").replace(/[0-9a-f]{8}-[0-9a-f-]{27}/g, "[id]");

const A = "moi-56";
const B = "moi-57";
const TEN_A = "Hạ Kiểm Thử";
const TEN_B_DAT = "Khôi bạn đi dạo";
const TEN_B = "Khôi Kiểm Thử";
const TEN_NHOM = "Nhóm E1 cuối tuần đi dạo";
const TIN_B = "Chào cả nhóm, mình là Khôi, cuối tuần này mình rảnh";
const TEN_KEO = "Kèo E2 dạo hồ cuối tuần";
const QUAN = "Lưng Chừng Cafe";
const MON_E3 = [["Cà phê muối", "3", "105000"], ["Bánh căn", "1", "45000"]];
const TONG_E3 = "150.000đ";
const PHAN_E3 = "75.000đ";
const TEN_CHI = "Cà phê hồ Tuyền Lâm";
const TIN_DM = "Kết bạn rồi nhé, tối nay mình gửi ảnh hồ cho Hạ";
const TIN_SAU_CHAN = "Hạ ơi mình gửi lại ảnh hồ nhé";
const TIN_NHOM_SAU_CHAN = "Cả nhóm ơi tuần sau đi tiếp không";

/** The group of the journeys, found by name on A's side. */
const nhomE = async () => {
  const pa = await phienCua(A);
  const ds = (await api("GET", "/people/me/contexts", undefined, pa)).contexts ?? [];
  return ds.find((c) => c.kind === "group" && c.display_name === TEN_NHOM) ?? null;
};

/** A control by its whole accessible name or visible text, scrolled into view vertically. */
const timNut = (page, chu, { vai = '[role="button"],button,[role="link"],a[href],[role="tab"],[role="radio"],[role="checkbox"]', batDau = false, trong = null } = {}) =>
  page.evaluate(
    ({ chu, vai, batDau, trong }) => {
      // Icon glyphs render as private-use characters inside the button text.
      const ten = (x) => (x.getAttribute("aria-label") || (x.innerText ?? "")).replace(/[-]/g, "").replace(/\s+/g, " ").trim();
      const goc = trong ? [...document.querySelectorAll(trong)].pop() ?? document : document;
      const e = [...goc.querySelectorAll(vai)].find((x) => (batDau ? ten(x).startsWith(chu) : ten(x) === chu) && x.getBoundingClientRect().width > 0);
      if (!e) return null;
      e.scrollIntoView({ block: "center", behavior: "instant" });
      for (let n = e.parentElement; n; n = n.parentElement) if (n.scrollLeft && !/(auto|scroll)/.test(getComputedStyle(n).overflowX)) n.scrollLeft = 0;
      const r = e.getBoundingClientRect();
      return { x: r.left + r.width / 2, y: r.top + r.height / 2, w: Math.round(r.width), h: Math.round(r.height), ten: ten(e) };
    },
    { chu, vai, batDau, trong },
  );
const bam = async (t, cdp, chu, { cho = 800, ...o } = {}) => {
  const n = await timNut(t.page, chu, o);
  if (!n) return null;
  await cham(cdp, n);
  await t.page.waitForTimeout(cho);
  await choOn(t.page, { mang: t.mang });
  return n;
};
const go = async (page, nhan, chu) => {
  const o = page.locator(`[aria-label="${nhan}"]`).first();
  await o.click();
  await o.fill(chu);
};
/** Only a rendered element counts: stacks keep earlier screens mounted and hidden. */
const coChu = (page, re) =>
  page.evaluate((s) => {
    const R = new RegExp(s);
    const hien = (x) => x.getClientRects().length > 0 && (x.checkVisibility ? x.checkVisibility({ visibilityProperty: true, opacityProperty: true }) : true);
    return [...document.querySelectorAll("body *")].some((x) => [...x.childNodes].some((n) => n.nodeType === 3 && R.test(n.textContent)) && hien(x));
  }, re.source);
/** Wait for a sentence to be on screen; the milliseconds it took, or null. */
const choChu = async (page, re, toiDa = 20_000) => {
  const t0 = Date.now();
  while (Date.now() - t0 < toiDa) {
    if (await coChu(page, re)) return Date.now() - t0;
    await page.waitForTimeout(400);
  }
  return null;
};
const chuTrang = (page) => page.evaluate(() => (document.body.innerText ?? "").replace(/[-]/g, "").replace(/\s+/g, " ").slice(0, 400));
const tab = async (t, cdp, ten) => {
  const n = await timNut(t.page, ten, { vai: '[role="tab"]' });
  if (n) {
    await cham(cdp, n);
    await t.page.waitForTimeout(1200);
    await choOn(t.page, { mang: t.mang });
  }
  return n;
};
/** Welcome → «Rủ Đi thôi!» → number → code, the way a person signs in; where it lands. */
const vaoCua = async (t, cdp, phone) => {
  await t.page.waitForTimeout(1500);
  const cta = await tamCua(t.page, '[data-testid="welcome-cta"]');
  if (cta) await cham(cdp, cta);
  await t.page.waitForURL("**/login", { timeout: 10_000 }).catch(() => undefined);
  await choOn(t.page, { mang: t.mang });
  const login = await duongDan(t.page);
  await t.page.locator('input[aria-label="Ô số điện thoại"]').fill(phone);
  await cham(cdp, await tamCua(t.page, '[data-testid="login-gui-ma"]'));
  await t.page.waitForURL("**/otp", { timeout: 10_000 }).catch(() => undefined);
  await choOn(t.page, { mang: t.mang });
  const otp = await duongDan(t.page);
  await t.page.keyboard.type("000000", { delay: 40 });
  await t.page.waitForURL((u) => !/\/otp$/.test(u.pathname), { timeout: 12_000 }).catch(() => undefined);
  await t.page.waitForTimeout(1500);
  await choOn(t.page, { mang: t.mang });
  return { login, otp, den: await duongDan(t.page) };
};
/** Composer and send arrow of a chat. */
const guiTin = async (t, cdp, chu) => {
  const o = t.page.locator('textarea[aria-label="Ô soạn tin"]');
  await o.click();
  await o.fill(chu);
  await t.page.waitForTimeout(200);
  const n = await timNut(t.page, "Gửi tin nhắn");
  if (n) await cham(cdp, n);
  await t.page.waitForTimeout(1500);
  return !!n;
};
const bongTin = (page, chu) =>
  page.evaluate((chu) => [...document.querySelectorAll('[aria-label^="Tin nhắn: "]')].some((e) => e.getAttribute("aria-label") === `Tin nhắn: ${chu}` && e.getBoundingClientRect().height > 0), chu);
const anhE = (ten) => join(mt.out, "jpg", `${ten}.jpg`);
/** «Tài chính của tôi», reached from the profile tab; the three tiles read from the DOM. */
const taiChinh = async (ten, id) => {
  const t = await trangMoi(mt, cauHinh("C1"), { persona: P(ten), path: "/profile" });
  const cdp = await cdpCua(t.page);
  await t.page.waitForTimeout(2500);
  await bam(t, cdp, "Tài chính của tôi", { batDau: true, cho: 2500 });
  const duong = await duongDan(t.page);
  const o = { phan: await oTien(t.page, "Phần chi của bạn"), conTra: await oTien(t.page, "Còn phải trả"), seNhan: await oTien(t.page, "Sẽ nhận") };
  const chu = (await t.page.evaluate(() => (document.body.innerText ?? "").replace(/[\uE000-\uF8FF]/g, "").replace(/\s+/g, " "))).slice(0, 900);
  if (id) await chup(t.page, { out: mt.out, id, suKien: t.suKien });
  await t.context.close();
  return { duong, o, chu };
};
/** One count on the LOCAL stack's database (read only), or null without a URL. */
function demSql(sql) {
  const url = (process.env.MOBILE_DATABASE_URL ?? "").replace("postgresql+psycopg", "postgresql");
  if (!url) return null;
  const r = spawnSync("psql", [url, "-Atc", sql], { encoding: "utf8" });
  return r.status === 0 ? Number(String(r.stdout).trim()) : null;
}
const sach = (id) => String(id).replace(/[^0-9a-f-]/g, "");
/** The amount on the row a label heads (the nearest ancestor holding a bare sum). */
const oTien = (page, nhan) =>
  page.evaluate((nhan) => {
    const RE = /^-?\d{1,3}(\.\d{3})*đ$/;
    const e = [...document.querySelectorAll("div,span")].find((x) => [...x.childNodes].some((n) => n.nodeType === 3 && n.textContent.trim() === nhan) && x.getClientRects().length);
    for (let n = e; n; n = n.parentElement) {
      const tien = [...n.querySelectorAll("div,span")].map((x) => x.textContent.trim()).filter((t) => RE.test(t));
      if (tien.length) return tien[0];
    }
    return null;
  }, nhan);
/** Type into an input by its accessible name, then leave it (number boxes commit on blur). */
const goSo = async (page, nhan, chu) => {
  const o = page.locator(`[aria-label="${nhan}"]`).first();
  await o.scrollIntoViewIfNeeded();
  await o.click();
  await o.fill("");
  await o.type(chu, { delay: 4 });
  await page.keyboard.press("Tab");
};
const thuTuChang = (page) => page.evaluate(() => [...document.querySelectorAll('[aria-label^="Chặng "][role="button"]')].filter((e) => e.getBoundingClientRect().height > 0).map((e) => e.getAttribute("aria-label").slice(6)));

try {
  // ------------------------------------------------------------------ E1
  // A new person signs up, opens a group, invites a second new person by
  // number; the second signs up, accepts, and writes; A reads it.
  if (chay("e1-vao") && existsSync(join(phienDir, `${A}.json`))) console.log(`e1-vao: ${A} đã có tài khoản, bỏ qua`);
  else if (chay("e1-vao")) {
    const ta = await trangMoi(mt, cauHinh("C1"), { path: "/welcome" });
    const cdpA = await cdpCua(ta.page);
    const cua = await vaoCua(ta, cdpA, P(A).phone);
    const moi = cua.den === "/personalization";
    if (moi) {
      await go(ta.page, "Ô tên của bạn", TEN_A);
      for (const g of ["Ăn uống", "Cafe", "Chơi đêm"]) {
        const c = await tamCua(ta.page, `[aria-label="${g}"]`);
        if (c) await cham(cdpA, c);
        await ta.page.waitForTimeout(150);
      }
      const mc = await tamCua(ta.page, '[aria-label*="Rộng tay"]');
      if (mc) await cham(cdpA, mc);
      await ta.page.waitForTimeout(300);
      await bam(ta, cdpA, "Lưu sở thích", { batDau: true, cho: 2500 });
    }
    const sauSoThich = await duongDan(ta.page);
    await chup(ta.page, { out: mt.out, id: "EV-E1-A1-C1", suKien: ta.suKien });
    await tab(ta, cdpA, "Cá nhân");
    const tenHoSo = await coChu(ta.page, new RegExp(TEN_A));
    ket({ feature: "E1", tc: "TC-E1-VAO-CUA", screen: "E1", state: "người mới, chưa có tài khoản", action: "màn chào → «Rủ Đi thôi!» → số → mã → Sở thích (tên, 3 gu, mức chi) → «Lưu sở thích»", cauHinh: "C1", expected: "mỗi bước tới đúng màn kế; sau Sở thích tới màn đầu của người chưa có nhóm; tab Cá nhân mang đúng tên vừa gõ", status: cua.login === "/login" && /\/otp$/.test(cua.otp) && moi && sauSoThich !== "/personalization" && tenHoSo ? "PASS" : "FAIL", evidence: ["EV-E1-A1-C1"], ghiChu: `màn chào → ${cua.login} → ${cua.otp} → ${cua.den} → ${sauSoThich}; tên «${TEN_A}» ở Cá nhân: ${tenHoSo ? "có" : "không"}` });
    await ta.context.close();
  }

  if (chay("e1-nhom") && (await nhomE())) console.log("e1-nhom: nhóm đã có, bỏ qua");
  else if (chay("e1-nhom")) {
    const ta = await trangMoi(mt, cauHinh("C1"), { persona: P(A), path: "/messages" });
    const cdpA = await cdpCua(ta.page);
    await ta.page.waitForTimeout(1500);
    await bam(ta, cdpA, "Tạo nhóm", { cho: 1200 });
    const form = await duongDan(ta.page);
    await go(ta.page, "Ô tên nhóm", TEN_NHOM);
    await bam(ta, cdpA, "Mở nhóm", { cho: 3500 });
    const sauMoNhom = await duongDan(ta.page);
    const tenTrenMan = await coChu(ta.page, new RegExp(TEN_NHOM));
    await chup(ta.page, { out: mt.out, id: "EV-E1-A2-C1", suKien: ta.suKien });
    const nhom = await nhomE();
    await tab(ta, cdpA, "Tin nhắn");
    const trongDanhSach = await coChu(ta.page, new RegExp(TEN_NHOM));
    ket({ feature: "E1", tc: "TC-E1-LAP-NHOM", screen: "E1", state: "A vừa vào cửa, chưa có nhóm", action: "Tin nhắn → «Tạo nhóm» → tên → «Mở nhóm», rồi quay về Tin nhắn", cauHinh: "C1", expected: "nhóm được mở; màn sau «Mở nhóm» cho thấy nhóm vừa lập (hoặc lối mời bạn); Tin nhắn liệt kê nhóm", status: nhom && tenTrenMan && trongDanhSach ? "PASS" : "FAIL", evidence: ["EV-E1-A2-C1"], ghiChu: `form ở ${form}; nhóm trên máy chủ: ${nhom ? "có" : "không"}; sau «Mở nhóm» tới ${sauMoNhom}, tên nhóm trên màn: ${tenTrenMan ? "có" : "không"}; trong Tin nhắn: ${trongDanhSach ? "có" : "không"}` });
    await ta.context.close();
  }

  if (chay("e1-moi")) {
    const kq = { a: {}, b: {} };
    const nhom = await nhomE();
    if (!nhom) throw new Error("chưa có nhóm E1; chạy --chi e1-vao,e1-nhom trước");
    const buoc = [{ file: anhE("EV-E1-A1-C1"), nhan: "A: vào cửa xong (Sở thích đã lưu)" }, { file: anhE("EV-E1-A2-C1"), nhan: "A: «Mở nhóm» xong, về Khám phá" }];
    const pa = await phienCua(A);
    // A invites B from the group's own screens.
    const ta = await trangMoi(mt, cauHinh("C1"), { persona: P(A), path: "/messages" });
    const cdpA = await cdpCua(ta.page);
    await ta.page.waitForTimeout(1500);
    const cham0 = [];
    if (await bam(ta, cdpA, `Mở nhóm ${TEN_NHOM}`, { cho: 1500 })) cham0.push("hàng nhóm");
    kq.a.chat = await duongDan(ta.page);
    if (await bam(ta, cdpA, "Thành viên nhóm", { cho: 1200 })) cham0.push("«Thành viên nhóm»");
    if (await bam(ta, cdpA, "Mời bằng số điện thoại", { cho: 1200 })) cham0.push("«Mời bằng số điện thoại»");
    kq.a.form = await duongDan(ta.page);
    const coForm = /\/invite$/.test(kq.a.form);
    if (coForm) {
      await go(ta.page, "Ô tên người được mời", TEN_B_DAT);
      await go(ta.page, "Ô số điện thoại người được mời", P(B).phone);
      if (await bam(ta, cdpA, "Gửi lời mời", { cho: 3000 })) cham0.push("«Gửi lời mời»");
    }
    kq.a.daMoi = await coChu(ta.page, /^Đã mời/);
    await chup(ta.page, { out: mt.out, id: "EV-E1-A3-C1", suKien: ta.suKien });
    buoc.push({ file: anhE("EV-E1-A3-C1"), nhan: "A: mời B bằng số, từ Thành viên" });
    const tv = (await api("GET", `/contexts/${nhom.id}/members`, undefined, pa)).members ?? [];
    kq.a.dangMoi = tv.filter((m) => m.state === "invited").length;
    ket({ feature: "E1", tc: "TC-E1-MOI", screen: "E1", state: "A ở Tin nhắn, nhóm vừa lập", action: `${cham0.join(" → ")}`, cauHinh: "C1", expected: "tới form mời từ chính nhóm, không cần gõ đường dẫn; «Đã mời …»; máy chủ có một người đang được mời", status: coForm && kq.a.daMoi && kq.a.dangMoi === 1 ? "PASS" : "FAIL", evidence: ["EV-E1-A3-C1"], ghiChu: `hàng nhóm mở ${an(kq.a.chat)}; form ở ${an(kq.a.form)}; ${cham0.length} chạm từ Tin nhắn tới lúc gửi; phong bì «Đã mời»: ${kq.a.daMoi ? "có" : "không"}; đang được mời trên máy chủ: ${kq.a.dangMoi}` });
    // Leave A in the group chat, as someone waiting for friends to join would be.
    await ta.page.goBack();
    await ta.page.waitForTimeout(800);
    await ta.page.goBack();
    await ta.page.waitForTimeout(1500);
    await choOn(ta.page, { mang: ta.mang });
    kq.a.choTrong = await duongDan(ta.page);

    // B signs up through the UI, finds the invitation, accepts, writes.
    const tb = await trangMoi(mt, cauHinh("C1"), { path: "/welcome" });
    const cdpB = await cdpCua(tb.page);
    const cuaB = await vaoCua(tb, cdpB, P(B).phone);
    kq.b.cua = cuaB;
    kq.b.thayLoiMoi = !!(await timNut(tb.page, "Đồng ý vào nhóm"));
    kq.b.tenNhom = await coChu(tb.page, new RegExp(TEN_NHOM));
    await chup(tb.page, { out: mt.out, id: "EV-E1-B1-C1", suKien: tb.suKien });
    buoc.push({ file: anhE("EV-E1-B1-C1"), nhan: `B: vào cửa, tới ${cuaB.den}` });
    ket({ feature: "E1", tc: "TC-E1-B-VAO", screen: "E1", state: "B được mời bằng số, chưa từng đăng nhập", action: "màn chào → «Rủ Đi thôi!» → số → mã", cauHinh: "C1", expected: "B được hỏi tên và sở thích như mọi người mới, rồi thấy lời mời vào đúng nhóm", status: kq.b.thayLoiMoi && kq.b.tenNhom && cuaB.den === "/personalization" ? "PASS" : "FAIL", evidence: ["EV-E1-B1-C1"], ghiChu: `màn chào → ${cuaB.login} → ${cuaB.otp} → ${cuaB.den}; nút «Đồng ý vào nhóm»: ${kq.b.thayLoiMoi ? "có" : "không"}; tên nhóm trên màn: ${kq.b.tenNhom ? "có" : "không"}` });
    if (kq.b.thayLoiMoi) await bam(tb, cdpB, "Đồng ý vào nhóm", { cho: 3000 });
    kq.b.sauDongY = await duongDan(tb.page);
    if (!/\/chat$/.test(kq.b.sauDongY)) await bam(tb, cdpB, `Mở nhóm ${TEN_NHOM}`, { cho: 2000 });
    kq.b.chat = await duongDan(tb.page);
    kq.b.gui = /\/chat$/.test(kq.b.chat) ? await guiTin(tb, cdpB, TIN_B) : false;
    kq.b.bongMinh = await bongTin(tb.page, TIN_B);
    await chup(tb.page, { out: mt.out, id: "EV-E1-B2-C1", suKien: tb.suKien });
    buoc.push({ file: anhE("EV-E1-B2-C1"), nhan: "B: đồng ý, mở nhóm, gửi tin" });
    ket({ feature: "E1", tc: "TC-E1-B-DONG-Y", screen: "E1", state: "B thấy lời mời", action: "«Đồng ý vào nhóm», mở nhóm, gửi một tin", cauHinh: "C1", expected: "đồng ý xong vào được nhóm; tin gửi hiện trong chat của B", status: /\/chat$/.test(kq.b.chat) && kq.b.bongMinh ? "PASS" : "FAIL", evidence: ["EV-E1-B2-C1"], ghiChu: `sau «Đồng ý» ở ${an(kq.b.sauDongY)}; chat ở ${an(kq.b.chat)}; gửi: ${kq.b.gui ? "có" : "không"}; bong bóng của B: ${kq.b.bongMinh ? "có" : "không"}` });

    // A, still in the chat, gets B's message without reloading.
    kq.a.msTin = await choChu(ta.page, new RegExp(TIN_B.slice(0, 30)), 30_000);
    await chup(ta.page, { out: mt.out, id: "EV-E1-A4-C1", suKien: ta.suKien });
    buoc.push({ file: anhE("EV-E1-A4-C1"), nhan: "A: tin đầu của B hiện trong chat đang mở" });
    // The delay itself is measured in e1-tin: here the wait starts only after
    // B has sent and been photographed, so it cannot time anything.
    kq.a.thayTinDau = kq.a.msTin !== null;
    const tv2 = (await api("GET", `/contexts/${nhom.id}/members`, undefined, pa)).members ?? [];
    const pb = await phienCua(B);
    const mB = tv2.find((m) => m.person_id === pb.person_id);
    kq.b.tenTrongNhom = mB?.display_name ?? null;
    ket({ feature: "E1", tc: "TC-E1-TEN-B", screen: "E1", state: "B vừa vào nhóm", action: "đọc danh sách thành viên", cauHinh: "C1", expected: "B ở trạng thái active, dưới tên B tự chọn (hoặc đã được hỏi để xác nhận tên A đặt)", status: mB?.state === "active" && mB?.display_name !== TEN_B_DAT ? "PASS" : "FAIL", evidence: [], ghiChu: `B trên máy chủ: ${mB?.state ?? "không thấy"}, tên trong nhóm «${kq.b.tenTrongNhom ?? "?"}» (A đặt «${TEN_B_DAT}»)` });
    await ghepAnh(mt.browser, buoc, anhE("EV-E1-C1-ghep"), { cao: 760, tieuDe: "E1 người mới → lập nhóm → mời → người được mời vào và nhắn, C1" });
    log({ e1: kq });
    await ta.context.close();
    await tb.context.close();
  }
  // Message delay, both ways, from the moment the sender starts writing: the
  // wait on the reader starts first and runs alongside the sending.
  if (chay("e1-tin")) {
    const nhom = await nhomE();
    if (!nhom) throw new Error("chưa có nhóm E1");
    const ta = await trangMoi(mt, cauHinh("C1"), { persona: P(A), path: `/groups/${nhom.id}/chat` });
    const tb = await trangMoi(mt, cauHinh("C1"), { persona: P(B), path: `/groups/${nhom.id}/chat` });
    const cdpA = await cdpCua(ta.page);
    const cdpB = await cdpCua(tb.page);
    await ta.page.waitForTimeout(2500);
    await tb.page.waitForTimeout(2500);
    const dau = (page) => page.evaluate(() => (document.querySelector('[aria-label="Thành viên nhóm"]')?.innerText ?? "").replace(/\s+/g, " ").trim());
    const tinBA = "Mình mang theo bánh căn nhé";
    const tinAB = "Tuyệt, mình đặt bàn lúc 7 giờ tối";
    const doiA = choChu(ta.page, new RegExp(tinBA), 30_000);
    await guiTin(tb, cdpB, tinBA);
    const msBA = await doiA;
    const doiB = choChu(tb.page, new RegExp(tinAB), 30_000);
    await guiTin(ta, cdpA, tinAB);
    const msAB = await doiB;
    const dauA = await dau(ta.page);
    ket({ feature: "E1", tc: "TC-E1-TIN-TOI", screen: "E1", state: "A và B cùng mở chat nhóm", action: "B gửi một tin, rồi A gửi một tin; bên kia chờ, không tải lại", cauHinh: "C1", expected: "mỗi tin hiện ở máy bên kia trong vài giây (≤ 10 s), không cần tải lại", status: msBA !== null && msAB !== null && msBA <= 10_000 && msAB <= 10_000 ? "PASS" : "FAIL", evidence: [], ghiChu: `B → A: ${msBA === null ? "sau 30 s chưa tới" : `${msBA} ms`}; A → B: ${msAB === null ? "sau 30 s chưa tới" : `${msAB} ms`} (đo từ lúc máy gửi bắt đầu soạn, gồm cả lúc gõ và chạm gửi); đầu chat của A: «${dauA}»` });
    const buoc = [
      { file: anhE("EV-E1-A1-C1"), nhan: "A: vào cửa xong (Sở thích đã lưu)" },
      { file: anhE("EV-E1-A2-C1"), nhan: "A: «Mở nhóm» xong, về Khám phá" },
      { file: anhE("EV-E1-A3-C1"), nhan: "A: mời B bằng số, từ Thành viên" },
      { file: anhE("EV-E1-B1-C1"), nhan: "B: vào cửa, tới /messages" },
      { file: anhE("EV-E1-B2-C1"), nhan: "B: đồng ý, mở nhóm, gửi tin" },
      { file: anhE("EV-E1-A4-C1"), nhan: "A: tin đầu của B hiện trong chat đang mở" },
    ];
    await ghepAnh(mt.browser, buoc, anhE("EV-E1-C1-ghep"), { cao: 760, tieuDe: "E1 người mới → lập nhóm → mời → người được mời vào và nhắn, C1" });
    await ta.context.close();
    await tb.context.close();
  }
  // The header count of a chat left open while an invited person joins and
  // writes. EV-E1-A4-C1 showed «1 thành viên» over B's first message; this
  // part repeats the E1 order with a third person, C, and times the header.
  // The roster is read on focus (invited people included, by name), so the
  // prediction is: invite, back to the chat, C joins and writes, and the
  // header keeps the count it had at focus; reopening the chat corrects it.
  const C = "moi-58";
  const TEN_C_DAT = "Lan bạn cùng lớp";
  const TIN_C = "Chào cả nhóm, mình là Lan, mình vào rồi nè";
  if (chay("e1-dem") && existsSync(join(phienDir, `${C}.json`))) console.log(`e1-dem: ${C} đã có tài khoản, bỏ qua`);
  else if (chay("e1-dem")) {
    const nhom = await nhomE();
    if (!nhom) throw new Error("chưa có nhóm E1");
    const pa = await phienCua(A);
    const dem = async () => ((await api("GET", `/contexts/${nhom.id}/members`, undefined, pa)).members ?? []).filter((m) => m.state === "active").length;
    const dau = (page) => page.evaluate(() => (document.querySelector('[aria-label="Thành viên nhóm"]')?.innerText ?? "").replace(/\s+/g, " ").trim());
    const kq = { a: {}, c: {} };
    // A invites C from the group's screens, then goes back to the chat: E1's order.
    const ta = await trangMoi(mt, cauHinh("C1"), { persona: P(A), path: "/messages" });
    const cdpA = await cdpCua(ta.page);
    await ta.page.waitForTimeout(1500);
    await bam(ta, cdpA, `Mở nhóm ${TEN_NHOM}`, { cho: 1500 });
    await bam(ta, cdpA, "Thành viên nhóm", { cho: 1200 });
    await bam(ta, cdpA, "Mời bằng số điện thoại", { cho: 1200 });
    kq.a.form = await duongDan(ta.page);
    if (!/\/invite$/.test(kq.a.form)) throw new Error(`không tới form mời: ${an(kq.a.form)}`);
    await go(ta.page, "Ô tên người được mời", TEN_C_DAT);
    await go(ta.page, "Ô số điện thoại người được mời", P(C).phone);
    await bam(ta, cdpA, "Gửi lời mời", { cho: 3000 });
    kq.a.daMoi = await coChu(ta.page, /^Đã mời/);
    await ta.page.goBack();
    await ta.page.waitForTimeout(800);
    await ta.page.goBack();
    await ta.page.waitForTimeout(2500);
    await choOn(ta.page, { mang: ta.mang });
    kq.a.chat = await duongDan(ta.page);
    kq.a.dauTruoc = await dau(ta.page);
    kq.demTruoc = await dem();

    // C signs up through the UI, accepts, opens the group, writes.
    const tc = await trangMoi(mt, cauHinh("C1"), { path: "/welcome" });
    const cdpC = await cdpCua(tc.page);
    kq.c.cua = (await vaoCua(tc, cdpC, P(C).phone)).den;
    if (await bam(tc, cdpC, "Đồng ý vào nhóm", { cho: 3000 })) kq.c.dongY = true;
    if (!/\/chat$/.test(await duongDan(tc.page))) await bam(tc, cdpC, `Mở nhóm ${TEN_NHOM}`, { cho: 2000 });
    kq.c.chat = await duongDan(tc.page);
    const doi = choChu(ta.page, new RegExp(TIN_C.slice(0, 30)), 30_000);
    kq.c.gui = /\/chat$/.test(kq.c.chat) ? await guiTin(tc, cdpC, TIN_C) : false;
    kq.a.msTin = await doi;
    kq.demSau = await dem();

    // A, still in the chat: the header right away, at 5 s and at 15 s.
    kq.a.dau0 = await dau(ta.page);
    await ta.page.waitForTimeout(5000);
    kq.a.dau5 = await dau(ta.page);
    await ta.page.waitForTimeout(10_000);
    kq.a.dau15 = await dau(ta.page);
    kq.a.tenC = await coChu(ta.page, new RegExp(TEN_C_DAT));
    await chup(ta.page, { out: mt.out, id: "EV-E1-DEM-C1", suKien: ta.suKien });
    // Leave and reopen the chat: the focus reads the roster again.
    await ta.page.goBack();
    await ta.page.waitForTimeout(1500);
    await choOn(ta.page, { mang: ta.mang });
    await bam(ta, cdpA, `Mở nhóm ${TEN_NHOM}`, { cho: 2500 });
    kq.a.dauMoLai = await dau(ta.page);
    await chup(ta.page, { out: mt.out, id: "EV-E1-DEM-SAU-C1", suKien: ta.suKien });
    const dung = new RegExp(`\\b${kq.demSau} thành viên`);
    ket({ feature: "E1", tc: "TC-E1-DEM-THANH-VIEN", screen: "E1", state: "A mời C từ Thành viên rồi quay về chat nhóm; C vào cửa, đồng ý, nhắn", action: "A để chat mở, không chạm gì; đọc thanh đầu ngay khi tin của C tới, sau 5 s và 15 s; rồi rời chat và mở lại", cauHinh: "C1", expected: "thanh đầu của chat đang mở theo kịp số thành viên khi người được mời vào nhóm (trong vài giây), như nó theo kịp tin nhắn", status: kq.a.msTin !== null && dung.test(kq.a.dau15) ? "PASS" : "FAIL", evidence: ["EV-E1-DEM-C1", "EV-E1-DEM-SAU-C1"], ghiChu: `trên máy chủ ${kq.demTruoc} → ${kq.demSau} người active; tin của C tới A sau ${kq.a.msTin ?? "> 30000"} ms; thanh đầu của A: trước «${kq.a.dauTruoc}», khi tin tới «${kq.a.dau0}», 5 s «${kq.a.dau5}», 15 s «${kq.a.dau15}», mở lại «${kq.a.dauMoLai}»; tên C trên bong bóng: ${kq.a.tenC ? "có" : "không"}; C vào cửa tới ${an(kq.c.cua)}` });
    await ghepAnh(mt.browser, [{ file: anhE("EV-E1-DEM-C1"), nhan: "A: chat vẫn mở, 15 s sau tin của C" }, { file: anhE("EV-E1-DEM-SAU-C1"), nhan: "A: rời chat rồi mở lại" }], anhE("EV-E1-DEM-ghep"), { cao: 760, tieuDe: "E1 người được mời vào nhóm khi chat đang mở: số thành viên ở thanh đầu, C1" });
    log({ e1dem: kq });
    await ta.context.close();
    await tc.context.close();
  }
  // ------------------------------------------------------------------ E2
  // From the group chat, an outing; a place from Explore added to it; the
  // map; A's check-in at the stop; the memory on the group wall; B's view.
  const keoE = async () => {
    const nhom = await nhomE();
    if (!nhom) return null;
    const pa = await phienCua(A);
    return ((await api("GET", `/contexts/${nhom.id}/outings`, undefined, pa)).outings ?? []).find((k) => k.title === TEN_KEO) ?? null;
  };
  if (chay("e2-tao") && (await keoE())) console.log("e2-tao: kèo đã có, bỏ qua");
  else if (chay("e2-tao")) {
    const nhom = await nhomE();
    if (!nhom) throw new Error("chưa có nhóm E1");
    const kq = {};
    const ta = await trangMoi(mt, cauHinh("C1"), { persona: P(A), path: "/messages" });
    const cdpA = await cdpCua(ta.page);
    await ta.page.waitForTimeout(1500);
    await bam(ta, cdpA, `Mở nhóm ${TEN_NHOM}`, { cho: 2000 });
    kq.chat = await duongDan(ta.page);
    let keo = await keoE();
    const cham1 = [];
    if (!keo) {
      // An empty chat offers «Rủ hội một buổi»; once it has messages the way
      // is the composer's «+», then «Tờ hẹn».
      if (await bam(ta, cdpA, "Rủ hội một buổi", { cho: 1200 })) cham1.push("«Rủ hội một buổi»");
      else {
        if (await bam(ta, cdpA, "Thêm vào cuộc trò chuyện", { cho: 1000 })) cham1.push("«+»");
        if (await bam(ta, cdpA, "Tờ hẹn", { cho: 1200 })) cham1.push("«Tờ hẹn»");
      }
      kq.khay = (await ta.page.evaluate(() => (document.body.innerText ?? "").replace(/\s+/g, " "))).match(/AI chưa sẵn sàng[^.]*\.[^.]*\./)?.[0] ?? null;
      if (await bam(ta, cdpA, "Tự tạo kèo", { cho: 1500 })) cham1.push("«Tự tạo kèo»");
      kq.form = await duongDan(ta.page);
      await go(ta.page, "Ô tên kèo", TEN_KEO);
      if (await bam(ta, cdpA, "300 nghìn", { cho: 300 })) cham1.push("chip «300 nghìn»");
      if (await bam(ta, cdpA, "Tạo kèo", { cho: 3500 })) cham1.push("«Tạo kèo»");
      keo = await keoE();
    }
    kq.sauTao = await duongDan(ta.page);
    await chup(ta.page, { out: mt.out, id: "EV-E2-A1-C1", suKien: ta.suKien });
    ket({ feature: "E2", tc: "TC-E2-TAO-TU-CHAT", screen: "E2", state: "A trong chat nhóm E1, nhóm chưa có kèo", action: `${cham1.join(" → ")}`, cauHinh: "C1", expected: "từ chat tới form kèo trong vài chạm, form gắn sẵn nhóm; «Tạo kèo» đưa tới kèo mới, kèo thuộc đúng nhóm", status: keo && keo.context_id === nhom.id && /contextId=/.test(kq.form ?? "") && new RegExp(`^/outings/${keo.id}`).test(kq.sauTao) ? "PASS" : "FAIL", evidence: ["EV-E2-A1-C1"], ghiChu: `chat ${an(kq.chat)}; khay: «${kq.khay ?? "-"}»; form ${an(kq.form)}; sau «Tạo kèo» ${an(kq.sauTao)}; kèo trên máy chủ: ${keo ? `có, ${keo.starts_on} → ${keo.ends_on}, ${keo.days ?? "?"} ngày` : "không"}` });
    if (!keo) throw new Error("không tạo được kèo E2");

    // Back in the chat: does the group see that there is an outing now?
    await ta.page.goBack();
    await ta.page.waitForTimeout(1500);
    await choOn(ta.page, { mang: ta.mang });
    kq.veChat = await duongDan(ta.page);
    kq.chatThayKeo = await coChu(ta.page, new RegExp(TEN_KEO));
    await chup(ta.page, { out: mt.out, id: "EV-E2-A2-C1", suKien: ta.suKien });
    ket({ feature: "E2", tc: "TC-E2-CHAT-BIET-KEO", screen: "E2", state: "vừa tạo kèo từ chat", action: "Back về chat nhóm", cauHinh: "C1", expected: "chat cho cả nhóm thấy có kèo mới (thẻ, tin hệ thống hoặc dải kèo sắp tới) và lối mở nó", status: /\/chat$/.test(kq.veChat) && kq.chatThayKeo ? "PASS" : "FAIL", evidence: ["EV-E2-A2-C1"], ghiChu: `Back tới ${an(kq.veChat)}; tên kèo trong chat: ${kq.chatThayKeo ? "có" : "không"}` });
    log({ e2tao: kq });
    await ta.context.close();
  }

  if (chay("e2-chang")) {
    const nhom = await nhomE();
    let keo = await keoE();
    if (!keo) throw new Error("chưa có kèo E2; chạy --chi e2-tao trước");
    const pa = await phienCua(A);
    const kq = {};
    const buoc = [{ file: anhE("EV-E2-A1-C1"), nhan: "A: kèo vừa tạo từ chat" }, { file: anhE("EV-E2-A2-C1"), nhan: "A: Back về chat: không thấy kèo" }];
    // A place from Explore into the outing.
    const ta = await trangMoi(mt, cauHinh("C1"), { persona: P(A), path: "/explore" });
    const cdpA = await cdpCua(ta.page);
    await ta.page.waitForTimeout(2500);
    await choOn(ta.page, { mang: ta.mang });
    const the = await ta.page.evaluate((ten) => {
      const e = [...document.querySelectorAll('[role="button"],button,[role="link"],a[href]')].find((x) => { const n = (x.getAttribute("aria-label") || x.innerText || "").replace(/\s+/g, " "); return n.includes(ten) && !/^(Lưu|Bỏ lưu)/.test(n) && x.getBoundingClientRect().width > 0; });
      if (!e) return null;
      e.scrollIntoView({ block: "center", behavior: "instant" });
      const r = e.getBoundingClientRect();
      return { x: r.left + r.width / 2, y: r.top + Math.min(r.height / 2, 40), ten: (e.getAttribute("aria-label") || "").slice(0, 60) };
    }, QUAN);
    if (the) { await cham(cdpA, the); await ta.page.waitForTimeout(1500); await choOn(ta.page, { mang: ta.mang }); }
    kq.quan = await duongDan(ta.page);
    await bam(ta, cdpA, "Thêm vào kèo", { cho: 2000 });
    kq.chon = await duongDan(ta.page);
    const them = await ta.page.evaluate((ten) => {
      const nutThem = (e) => [...e.querySelectorAll('[role="button"]')].filter((x) => (x.innerText ?? "").trim() === "Thêm vào");
      const hang = [...document.querySelectorAll("div")].filter((e) => (e.innerText ?? "").includes(ten) && nutThem(e).length === 1).sort((a, b) => a.getBoundingClientRect().width * a.getBoundingClientRect().height - b.getBoundingClientRect().width * b.getBoundingClientRect().height)[0];
      const b = hang ? nutThem(hang)[0] : null;
      if (!b) return null;
      b.scrollIntoView({ block: "center", behavior: "instant" });
      const r = b.getBoundingClientRect();
      return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
    }, TEN_KEO);
    if (them) { await cham(cdpA, them); await ta.page.waitForTimeout(2500); await choOn(ta.page, { mang: ta.mang }); }
    kq.sauThem = await duongDan(ta.page);
    kq.chang = await thuTuChang(ta.page);
    await chup(ta.page, { out: mt.out, id: "EV-E2-A3-C1", suKien: ta.suKien });
    buoc.push({ file: anhE("EV-E2-A3-C1"), nhan: `A: thêm ${QUAN} từ Khám phá` });
    keo = await keoE();
    const changQuan = (keo?.stops ?? []).find((s) => (s.place_name ?? s.label ?? "").includes(QUAN));
    ket({ feature: "E2", tc: "TC-E2-THEM-QUAN", screen: "E2", state: `kèo «${TEN_KEO}» chưa có chặng`, action: `Khám phá → thẻ «${QUAN}» → «Thêm vào kèo» → «Thêm vào» ở kèo này`, cauHinh: "C1", expected: "về đúng kèo, có một chặng mang tên quán; máy chủ giữ chặng đó", status: new RegExp(`^/outings/${keo?.id}`).test(kq.sauThem) && kq.chang.some((c) => c.includes(QUAN)) && changQuan ? "PASS" : "FAIL", evidence: ["EV-E2-A3-C1"], ghiChu: `thẻ ${the ? "có" : "không"} → ${an(kq.quan)} → ${an(kq.chon)} → ${an(kq.sauThem)}; chặng trên màn: ${kq.chang.join(" | ") || "không"}; chặng trên máy chủ: ${changQuan ? `«${changQuan.label}», ngày ${changQuan.day ?? "không có"}` : "không"}` });

    // The map, then back to the itinerary.
    await bam(ta, cdpA, "Bản đồ", { cho: 1500 });
    const ngay = await ta.page.evaluate(() => [...document.querySelectorAll('[role="button"],[role="radio"],[role="tab"]')].map((e) => (e.innerText ?? "").replace(/[-]/g, "").trim()).filter((x) => /^Ngày \d+$/.test(x)));
    const moc = [];
    for (const d of ngay.length ? ngay : ["(một ngày)"]) {
      if (ngay.length) await bam(ta, cdpA, d, { cho: 900 });
      moc.push({ d, n: await ta.page.evaluate(() => document.querySelectorAll('[aria-label^="Mốc "]').length) });
    }
    await chup(ta.page, { out: mt.out, id: "EV-E2-A4-C1", suKien: ta.suKien });
    buoc.push({ file: anhE("EV-E2-A4-C1"), nhan: "A: Bản đồ của kèo" });
    const tongMoc = moc.reduce((a, b) => a + b.n, 0);
    ket({ feature: "E2", tc: "TC-E2-BAN-DO", screen: "E2", state: "kèo có một chặng gắn quán (có toạ độ)", action: "chuyển sang Bản đồ, đi qua từng ngày", cauHinh: "C1", expected: "một mốc cho chặng vừa thêm", status: tongMoc >= 1 ? "PASS" : "FAIL", evidence: ["EV-E2-A4-C1"], ghiChu: `${moc.map((m) => `${m.d}: ${m.n} mốc`).join("; ")}; kèo ${keo?.days ?? "?"} ngày, chặng mang ngày ${changQuan?.day ?? "không có"}` });
    await bam(ta, cdpA, "Lịch trình", { cho: 1200 });

    // A's check-in at the stop.
    const toi = await ta.page.evaluate((ten) => {
      const hop = [...document.querySelectorAll('[aria-label^="Chặng "][role="button"]')].find((e) => e.getAttribute("aria-label").includes(ten))?.getBoundingClientRect();
      const tatCa = [...document.querySelectorAll('[role="button"]')].filter((e) => (e.innerText ?? "").trim() === "Tôi đã tới" && e.getBoundingClientRect().width > 0);
      const gan = tatCa.map((e) => ({ e, r: e.getBoundingClientRect() })).sort((a, b) => Math.abs(a.r.top - (hop?.top ?? 0)) - Math.abs(b.r.top - (hop?.top ?? 0)))[0];
      if (!gan) return null;
      gan.e.scrollIntoView({ block: "center", behavior: "instant" });
      const r = gan.e.getBoundingClientRect();
      return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
    }, QUAN);
    if (toi) { await cham(cdpA, toi); await ta.page.waitForTimeout(2000); await choOn(ta.page, { mang: ta.mang }); }
    kq.daToi = await coChu(ta.page, /^Đã tới$/);
    const ci = (await api("GET", `/outings/${keo.id}/checkins`, undefined, pa)).checkins ?? [];
    ket({ feature: "E2", tc: "TC-E2-CHECK-IN", screen: "E2", state: `chặng «${QUAN}», chưa ai tới`, action: "«Tôi đã tới»", cauHinh: "C1", expected: "chặng mang nhãn «Đã tới»; máy chủ có một check-in của A", status: toi && kq.daToi && ci.length >= 1 ? "PASS" : "FAIL", evidence: [], ghiChu: `nút ${toi ? "có" : "không"}; nhãn «Đã tới»: ${kq.daToi ? "có" : "không"}; check-in trên máy chủ: ${ci.length}` });

    // The memory: the group wall and the album, reached from the group.
    await ta.page.goto(`${mt.base}/messages`);
    await ta.page.waitForTimeout(2500);
    await choOn(ta.page, { mang: ta.mang });
    await bam(ta, cdpA, `Mở nhóm ${TEN_NHOM}`, { cho: 2000 });
    await bam(ta, cdpA, "Thành viên nhóm", { cho: 1200 });
    await bam(ta, cdpA, "Tường kỷ niệm", { batDau: true, cho: 2500 });
    kq.tuong = await duongDan(ta.page);
    kq.tuongCoQuan = await coChu(ta.page, new RegExp(QUAN));
    kq.tuongCoKeo = await coChu(ta.page, new RegExp(TEN_KEO));
    await chup(ta.page, { out: mt.out, id: "EV-E2-A5-C1", suKien: ta.suKien });
    buoc.push({ file: anhE("EV-E2-A5-C1"), nhan: "A: Tường kỷ niệm của nhóm" });
    ket({ feature: "E2", tc: "TC-E2-KY-NIEM", screen: "E2", state: `A vừa check-in ở «${QUAN}» trong kèo «${TEN_KEO}»`, action: "Tin nhắn → nhóm → «Thành viên nhóm» → «Tường kỷ niệm»", cauHinh: "C1", expected: "tường của nhóm giữ lại dấu vết chuyến đi: check-in ở quán, hoặc ít nhất kèo", status: /\/wall$/.test(kq.tuong) && (kq.tuongCoQuan || kq.tuongCoKeo) ? "PASS" : "FAIL", evidence: ["EV-E2-A5-C1"], ghiChu: `tới ${an(kq.tuong)}; tên quán trên tường: ${kq.tuongCoQuan ? "có" : "không"}; tên kèo: ${kq.tuongCoKeo ? "có" : "không"}` });
    await ta.context.close();

    // B's side: Lên plan lists the outing, and it shows the stop and A's check-in.
    const tb = await trangMoi(mt, cauHinh("C1"), { persona: P(B), path: "/plan" });
    const cdpB = await cdpCua(tb.page);
    await tb.page.waitForTimeout(2500);
    kq.bPlan = await coChu(tb.page, new RegExp(TEN_KEO));
    const hangKeo = await tb.page.evaluate((ten) => { const e = [...document.querySelectorAll('[role="button"],[role="link"],a[href]')].find((x) => (x.getAttribute("aria-label") || x.innerText || "").includes(ten) && x.getBoundingClientRect().width > 0); if (!e) return null; e.scrollIntoView({ block: "center", behavior: "instant" }); const r = e.getBoundingClientRect(); return { x: r.left + r.width / 2, y: r.top + Math.min(r.height / 2, 30) }; }, TEN_KEO);
    if (hangKeo) { await cham(cdpB, hangKeo); await tb.page.waitForTimeout(2500); await choOn(tb.page, { mang: tb.mang }); }
    kq.bKeo = await duongDan(tb.page);
    kq.bChang = await thuTuChang(tb.page);
    kq.bDaToi = await coChu(tb.page, /đã tới/);
    await chup(tb.page, { out: mt.out, id: "EV-E2-B1-C1", suKien: tb.suKien });
    buoc.push({ file: anhE("EV-E2-B1-C1"), nhan: "B: mở kèo từ Lên plan" });
    ket({ feature: "E2", tc: "TC-E2-B-THAY", screen: "E2", state: "B là thành viên nhóm E1", action: "Lên plan → kèo", cauHinh: "C1", expected: "B thấy kèo trong Lên plan, mở ra thấy chặng A đã thêm và việc A đã tới", status: kq.bPlan && new RegExp(`^/outings/${keo.id}`).test(kq.bKeo) && kq.bChang.some((c) => c.includes(QUAN)) && kq.bDaToi ? "PASS" : "FAIL", evidence: ["EV-E2-B1-C1"], ghiChu: `kèo trong Lên plan: ${kq.bPlan ? "có" : "không"}; mở tới ${an(kq.bKeo)}; chặng: ${kq.bChang.join(" | ") || "không"}; dòng «đã tới»: ${kq.bDaToi ? "có" : "không"}` });
    await ghepAnh(mt.browser, buoc, anhE("EV-E2-C1-ghep"), { cao: 760, tieuDe: "E2 kèo từ chat → thêm quán từ Khám phá → bản đồ → check-in → tường → phía B, C1" });
    log({ e2: kq });
    await tb.context.close();
  }
  // The other place a trip is remembered: the album shelf of the group.
  if (chay("e2-album")) {
    const nhom = await nhomE();
    const keo = await keoE();
    if (!keo) throw new Error("chưa có kèo E2");
    const ta = await trangMoi(mt, cauHinh("C1"), { persona: P(A), path: "/messages" });
    const cdpA = await cdpCua(ta.page);
    await ta.page.waitForTimeout(1500);
    await bam(ta, cdpA, `Mở nhóm ${TEN_NHOM}`, { cho: 2000 });
    await bam(ta, cdpA, "Thành viên nhóm", { cho: 1200 });
    await bam(ta, cdpA, "Album chuyến đi", { batDau: true, cho: 2500 });
    const duong = await duongDan(ta.page);
    const chu = (await ta.page.evaluate(() => (document.body.innerText ?? "").replace(/[\uE000-\uF8FF]/g, "").replace(/\s+/g, " "))).slice(0, 600);
    const coKeo = chu.includes(TEN_KEO);
    const daToi = (chu.match(/(\d+) chỗ đã tới/) ?? [null, null])[1];
    const dangDi = /đang đi/.test(chu);
    await chup(ta.page, { out: mt.out, id: "EV-E2-A6-C1", suKien: ta.suKien });
    ket({ feature: "E2", tc: "TC-E2-ALBUM", screen: "E2", state: `A đã check-in ở «${QUAN}» trong kèo «${TEN_KEO}»; tường nhóm vẫn «Chưa có kỷ niệm nào»`, action: "Tin nhắn → nhóm → «Thành viên nhóm» → «Album chuyến đi»", cauHinh: "C1", expected: "kệ album có kèo này và ghi chỗ đã tới", status: /\/album$/.test(duong) && coKeo && Number(daToi) >= 1 ? "PASS" : "FAIL", evidence: ["EV-E2-A6-C1"], ghiChu: `tới ${an(duong)}; kèo trên kệ: ${coKeo ? "có" : "không"}; chỗ đã tới: ${daToi ?? "không có dòng"}; ${dangDi ? "ghi «đang đi»" : "không ghi «đang đi»"} cho kèo ${keo.starts_on} → ${keo.ends_on}; chữ: «${chu.slice(0, 220)}»` });
    void nhom;
    await ta.context.close();
    // The journey in six frames, the album last: where the trip is and is not remembered.
    await ghepAnh(mt.browser, [
      { file: anhE("EV-E2-A1-C1"), nhan: "A: kèo vừa tạo từ chat" },
      { file: anhE("EV-E2-A2-C1"), nhan: "A: Back về chat: không thấy kèo" },
      { file: anhE("EV-E2-A3-C1"), nhan: `A: thêm ${QUAN} từ Khám phá` },
      { file: anhE("EV-E2-B1-C1"), nhan: "B: kèo có chặng và «đã tới»" },
      { file: anhE("EV-E2-A5-C1"), nhan: "A: tường «Chưa có kỷ niệm nào»" },
      { file: anhE("EV-E2-A6-C1"), nhan: "A: album «0 chỗ đã tới»" },
    ], anhE("EV-E2-C1-ghep"), { cao: 760, tieuDe: "E2 kèo từ chat → thêm quán từ Khám phá → «Tôi đã tới» → tường và album của nhóm, C1" });
  }
  // ------------------------------------------------------------------ E3
  // The bill of the E2 outing, from the outing: five steps, the settlement,
  // a collection round published, B's finance page, A marking the money in,
  // both finance pages after. Every write first looks at what is there: the
  // ledger is append-only and a second run must not write twice.
  if (chay("e3")) {
    const nhom = await nhomE();
    const keo = await keoE();
    if (!keo) throw new Error("chưa có kèo E2");
    const pa = await phienCua(A);
    const pb = await phienCua(B);
    const soChi = () => demSql(`select count(*) from expenses where context_id = '${sach(nhom.id)}'`);
    const dotCua = async () => (await api("GET", `/contexts/${nhom.id}/batches`, undefined, pa)).batches ?? [];
    const kq = {};
    const buoc = [];
    const tienTren = (page) => page.evaluate(() => [...new Set(((document.body.innerText ?? "").match(/\d{1,3}(\.\d{3})+\s?đ/g) ?? []).map((x) => x.replace(/\s/g, "")))]);

    // 1–5: the bill, from the outing.
    const chiTruoc = soChi();
    if (chiTruoc === null) throw new Error("thiếu MOBILE_DATABASE_URL: source /tmp/rudi-stack/env.sh");
    const ta = await trangMoi(mt, cauHinh("C1"), { persona: P(A), path: "/plan" });
    const cdpA = await cdpCua(ta.page);
    await ta.page.waitForTimeout(2500);
    if (chiTruoc === 0) {
      const hang = await ta.page.evaluate((ten) => { const e = [...document.querySelectorAll('[role="button"],[role="link"],a[href]')].find((x) => (x.getAttribute("aria-label") || x.innerText || "").includes(ten) && x.getBoundingClientRect().width > 0); if (!e) return null; e.scrollIntoView({ block: "center", behavior: "instant" }); const r = e.getBoundingClientRect(); return { x: r.left + r.width / 2, y: r.top + Math.min(r.height / 2, 30) }; }, TEN_KEO);
      if (hang) { await cham(cdpA, hang); await ta.page.waitForTimeout(2500); await choOn(ta.page, { mang: ta.mang }); }
      await bam(ta, cdpA, "Chia bill buổi này", { cho: 2000 });
      kq.bill = await duongDan(ta.page);
      await bam(ta, cdpA, "Nhập tay", { cho: 900 });
      for (let i = 0; i < MON_E3.length; i += 1) {
        if (i > 0) await bam(ta, cdpA, "Thêm món", { cho: 400 });
        if (!(await ta.page.locator(`[aria-label="Ô tên món ${i + 1}"]`).count())) await bam(ta, cdpA, `Sửa Món ${i + 1}`, { cho: 400 });
        await goSo(ta.page, `Ô tên món ${i + 1}`, MON_E3[i][0]);
        await goSo(ta.page, `Ô số lượng món ${i + 1}`, MON_E3[i][1]);
        await goSo(ta.page, `Ô tiền món ${i + 1}`, MON_E3[i][2]);
      }
      await ta.page.waitForTimeout(400);
      await bam(ta, cdpA, "Tiếp: ai dùng món nào?", { cho: 400 });
      await ta.page.waitForFunction(() => /Bước 3/.test(document.querySelector('[role="progressbar"]')?.getAttribute("aria-label") ?? ""), null, { timeout: 12_000 }).catch(() => undefined);
      await ta.page.waitForTimeout(900);
      await bam(ta, cdpA, "Xem kết quả", { cho: 400 });
      await ta.page.waitForFunction(() => /Bước 4/.test(document.querySelector('[role="progressbar"]')?.getAttribute("aria-label") ?? ""), null, { timeout: 15_000 }).catch(() => undefined);
      await ta.page.waitForTimeout(900);
      await goSo(ta.page, "Ô tên khoản chi", TEN_CHI);
      kq.buoc4 = await tienTren(ta.page);
      kq.phanBan = await coChu(ta.page, /^Phần của bạn$/);
      await chup(ta.page, { out: mt.out, id: "EV-E3-A1-C1", suKien: ta.suKien });
      buoc.push({ file: anhE("EV-E3-A1-C1"), nhan: "A: bước 4, phần của mỗi người" });
      await bam(ta, cdpA, "Ghi vào sổ", { cho: 400 });
      await ta.page.waitForFunction(() => /Bước 5/.test(document.querySelector('[role="progressbar"]')?.getAttribute("aria-label") ?? ""), null, { timeout: 15_000 }).catch(() => undefined);
      await ta.page.waitForTimeout(1500);
      kq.chiSau = soChi();
      kq.daGhi = await coChu(ta.page, /Đã ghi sổ/);
      await chup(ta.page, { out: mt.out, id: "EV-E3-A2-C1", suKien: ta.suKien });
      buoc.push({ file: anhE("EV-E3-A2-C1"), nhan: "A: bước 5, đã ghi sổ" });
      ket({ feature: "E3", tc: "TC-E3-BILL-TU-KEO", screen: "E3", state: `kèo «${TEN_KEO}» của nhóm E1, nhóm chưa có khoản chi`, action: `Lên plan → kèo → «Chia bill buổi này» → «Nhập tay» ${MON_E3.length} món (tổng ${TONG_E3}) → «Tiếp: ai dùng món nào?» (giữ mặc định) → «Xem kết quả» → tên khoản → «Ghi vào sổ»`, cauHinh: "C1", expected: `luồng bill mở sẵn nhóm của kèo; bước 4 chia ${TONG_E3} thành hai phần ${PHAN_E3}; ghi xong có đúng một khoản chi`, status: /\/smart-split\/.+\/review\?ctx=/.test(kq.bill) && kq.buoc4.includes(PHAN_E3) && kq.chiSau - chiTruoc === 1 && kq.daGhi ? "PASS" : "FAIL", evidence: ["EV-E3-A1-C1", "EV-E3-A2-C1"], ghiChu: `bill mở ở ${an(kq.bill)}; số tiền ở bước 4: ${kq.buoc4.join(", ")}; «Phần của bạn»: ${kq.phanBan ? "có" : "không"}; khoản chi ${chiTruoc} → ${kq.chiSau}; «Đã ghi sổ»: ${kq.daGhi ? "có" : "không"}` });
      await bam(ta, cdpA, "Xem quyết toán", { cho: 2500 });
    } else {
      console.log(`e3: nhóm đã có ${chiTruoc} khoản chi, bỏ qua phần ghi bill`);
      await ta.page.goto(`${mt.base}/settlements/${nhom.id}`);
      await ta.page.waitForTimeout(2500);
    }
    await choOn(ta.page, { mang: ta.mang });

    // The settlement, then a round of collection, published.
    kq.qt = await duongDan(ta.page);
    kq.qtTien = await tienTren(ta.page);
    kq.qtB = await coChu(ta.page, new RegExp(TEN_B_DAT));
    await chup(ta.page, { out: mt.out, id: "EV-E3-A3-C1", suKien: ta.suKien });
    buoc.push({ file: anhE("EV-E3-A3-C1"), nhan: "A: quyết toán" });
    ket({ feature: "E3", tc: "TC-E3-QUYET-TOAN", screen: "E3", state: `vừa ghi khoản ${TONG_E3}, A đã trả bill`, action: "«Xem quyết toán»", cauHinh: "C1", expected: `quyết toán của nhóm: B chuyển cho A ${PHAN_E3}; tổng ${TONG_E3}`, status: new RegExp(`^/settlements/${nhom.id}`).test(kq.qt) && kq.qtTien.includes(PHAN_E3) && kq.qtTien.includes(TONG_E3) && kq.qtB ? "PASS" : "FAIL", evidence: ["EV-E3-A3-C1"], ghiChu: `ở ${an(kq.qt)}; số tiền trên màn: ${kq.qtTien.join(", ")}; tên B: ${kq.qtB ? "có" : "không"}` });
    let dot = await dotCua();
    if (dot.length === 0) {
      await bam(ta, cdpA, "Tạo đợt thu từ sổ", { cho: 2500 });
      await choOn(ta.page, { mang: ta.mang });
      await ta.page.waitForTimeout(1200);
      if (await coChu(ta.page, /Chưa phát: chưa ai bị nhắn gì/)) {
        await bam(ta, cdpA, "Phát đợt thu", { cho: 700 });
        await bam(ta, cdpA, "Phát, không hoàn lại", { cho: 3500 });
      }
      dot = await dotCua();
      kq.dot = await duongDan(ta.page);
      kq.dotTien = await tienTren(ta.page);
      kq.daPhat = await coChu(ta.page, /Đã phát: mỗi người xem phần/);
      kq.guiB = !!(await ta.page.evaluate((ten) => [...document.querySelectorAll('[role="button"]')].some((e) => (e.innerText ?? "").replace(/[-]/g, "").trim() === `Gửi cho ${ten}`), TEN_B_DAT));
      await chup(ta.page, { out: mt.out, id: "EV-E3-A4-C1", suKien: ta.suKien });
      buoc.push({ file: anhE("EV-E3-A4-C1"), nhan: "A: đợt thu đã phát" });
      kq.bTruoc = true;
      ket({ feature: "E3", tc: "TC-E3-DOT-THU", screen: "E3", state: "quyết toán có một khoản chưa vào đợt", action: "«Tạo đợt thu từ sổ» → «Phát đợt thu» → «Phát, không hoàn lại»", cauHinh: "C1", expected: `một đợt đã phát; phong bì có lối gửi cho B, phần của B ${PHAN_E3}`, status: dot.length === 1 && kq.daPhat && kq.guiB && kq.dotTien.includes(PHAN_E3) ? "PASS" : "FAIL", evidence: ["EV-E3-A4-C1"], ghiChu: `đợt ở ${an(kq.dot)}; đợt trên máy chủ: ${dot.length}; «Đã phát»: ${kq.daPhat ? "có" : "không"}; «Gửi cho ${TEN_B_DAT}»: ${kq.guiB ? "có" : "không"}; số tiền: ${kq.dotTien.join(", ")}` });
    } else console.log(`e3: đã có ${dot.length} đợt, bỏ qua phần tạo và phát`);

    if (kq.bTruoc) {
    const bTruoc = await taiChinh(B, "EV-E3-B1-C1");
    buoc.push({ file: anhE("EV-E3-B1-C1"), nhan: "B: Tài chính, trước khi trả" });
    const canTra = bTruoc.o.conTra;
    ket({ feature: "E3", tc: "TC-E3-B-TAI-CHINH", screen: "E3", state: `đợt đã phát, B nợ A ${PHAN_E3}`, action: "B: Cá nhân → «Tài chính của tôi»", cauHinh: "C1", expected: `trang tài chính của B nói B còn phải trả ${PHAN_E3}`, status: /\/finance/.test(bTruoc.duong) && canTra === PHAN_E3 ? "PASS" : "FAIL", evidence: ["EV-E3-B1-C1"], ghiChu: `ở ${an(bTruoc.duong)}; «Còn phải trả» ${canTra ?? "không đọc được"}, «Phần chi của bạn» ${bTruoc.o.phan ?? "-"}, «Sẽ nhận» ${bTruoc.o.seNhan ?? "-"}` });
    }

    // A marks B's money as arrived, on the round.
    dot = await dotCua();
    const idDot = dot[0]?.batch_id ?? dot[0]?.id;
    if (idDot) {
      if (!/\/batches\//.test(await duongDan(ta.page))) {
        await ta.page.goto(`${mt.base}/batches/${idDot}?ctx=${nhom.id}`);
        await ta.page.waitForTimeout(2500);
        await choOn(ta.page, { mang: ta.mang });
      }
      const demTruoc = await ta.page.evaluate(() => (document.body.innerText ?? "").match(/(\d+)\/(\d+) lượt chuyển đã về/)?.[0] ?? null);
      const nutVe = await ta.page.evaluate(() => { const e = [...document.querySelectorAll('[role="button"]')].find((x) => /^Tiền đã về từ /.test((x.innerText ?? "").replace(/[-]/g, "").trim())); if (!e) return null; e.scrollIntoView({ block: "center" }); const r = e.getBoundingClientRect(); return { x: r.left + r.width / 2, y: r.top + r.height / 2, ten: (e.innerText ?? "").replace(/[-]/g, "").trim() }; });
      if (nutVe) { await cham(cdpA, nutVe); await ta.page.waitForTimeout(2500); }
      const demSau = await ta.page.evaluate(() => (document.body.innerText ?? "").match(/(\d+)\/(\d+) lượt chuyển đã về/)?.[0] ?? null);
      await chup(ta.page, { out: mt.out, id: "EV-E3-A5-C1", suKien: ta.suKien });
      buoc.push({ file: anhE("EV-E3-A5-C1"), nhan: "A: tiền của B đã về" });
      if (nutVe) ket({ feature: "E3", tc: "TC-E3-TIEN-VE", screen: "E3", state: "đợt đã phát, B chưa trả", action: `«${nutVe.ten}»`, cauHinh: "C1", expected: "đếm lượt đã về tăng 1", status: demTruoc && demSau && demTruoc !== demSau && /^1\/1/.test(demSau) ? "PASS" : "FAIL", evidence: ["EV-E3-A5-C1"], ghiChu: `«${demTruoc ?? "-"}» → «${demSau ?? "-"}»` });
      else console.log("e3: không còn nút «Tiền đã về» (lượt trước đã bấm)");
    }
    await ta.context.close();

    log({ e3: kq });
    void pb;
  }

  // Both finance pages once A has marked B's money in; the six frames of E3.
  if (chay("e3-sau")) {
    const bSau = await taiChinh(B, "EV-E3-B2-C1");
    const aSau = await taiChinh(A, null);
    ket({ feature: "E3", tc: "TC-E3-SAU-KHI-VE", screen: "E3", state: "A đã bấm «Tiền đã về» cho phần của B", action: "B và A mở «Tài chính của tôi» từ tab Cá nhân", cauHinh: "C1", expected: "B không còn phải trả; A không còn sẽ nhận", status: bSau.o.conTra === "0đ" && aSau.o.seNhan === "0đ" ? "PASS" : "FAIL", evidence: ["EV-E3-B2-C1"], ghiChu: `B: «Còn phải trả» ${bSau.o.conTra ?? "-"}, «Phần chi» ${bSau.o.phan ?? "-"}; A: «Sẽ nhận» ${aSau.o.seNhan ?? "-"}, «Còn phải trả» ${aSau.o.conTra ?? "-"}; B: «${(bSau.chu.match(/Bạn [^.]*\./) ?? ["-"])[0]}»` });
    await ghepAnh(mt.browser, [
      { file: anhE("EV-E3-A1-C1"), nhan: "A: bước 4, phần của mỗi người" },
      { file: anhE("EV-E3-A3-C1"), nhan: "A: quyết toán" },
      { file: anhE("EV-E3-A4-C1"), nhan: "A: đợt thu đã phát" },
      { file: anhE("EV-E3-B1-C1"), nhan: "B: Tài chính, trước khi trả" },
      { file: anhE("EV-E3-A5-C1"), nhan: "A: tiền của B đã về" },
      { file: anhE("EV-E3-B2-C1"), nhan: "B: Tài chính, sau" },
    ], anhE("EV-E3-C1-ghep"), { cao: 760, tieuDe: `E3 chia bill từ kèo → quyết toán → đợt thu → tài chính của B → tiền đã về, C1 (tổng ${TONG_E3})` });
  }
  // ------------------------------------------------------------ E5 (a)
  // A and B share a group but are not friends. A adds B by number from
  // Tin nhắn; B finds the request and accepts; B writes privately; A reads.
  const tenCua = async (ten) => {
    const p = await phienCua(ten);
    const me = await api("GET", "/people/me", undefined, p);
    return me.display_name ?? me.name ?? null;
  };
  const laBanAB = async () => {
    const pa = await phienCua(A);
    const pb = await phienCua(B);
    const ds = (await api("GET", `/people/${pa.person_id}/friends`, undefined, pa)).friends ?? [];
    return JSON.stringify(ds).includes(pb.person_id);
  };
  const capAB = async () => {
    const pa = await phienCua(A);
    const pb = await phienCua(B);
    const ds = (await api("GET", "/people/me/contexts", undefined, pa)).contexts ?? [];
    for (const c of ds.filter((x) => x.kind === "pair")) {
      const tv = (await api("GET", `/contexts/${c.id}/members`, undefined, pa)).members ?? [];
      if (tv.some((m) => m.person_id === pb.person_id)) return c;
    }
    return null;
  };
  const moHangCo = async (t, cdp, ten) => {
    const n = await t.page.evaluate((ten) => { const e = [...document.querySelectorAll('[role="button"],[role="link"],a[href]')].find((x) => (x.getAttribute("aria-label") || x.innerText || "").includes(ten) && x.getBoundingClientRect().width > 0); if (!e) return null; e.scrollIntoView({ block: "center", behavior: "instant" }); const r = e.getBoundingClientRect(); return { x: r.left + r.width / 2, y: r.top + Math.min(r.height / 2, 30) }; }, ten);
    if (n) { await cham(cdp, n); await t.page.waitForTimeout(2500); await choOn(t.page, { mang: t.mang }); }
    return !!n;
  };
  if (chay("e5-ban") && (await laBanAB())) console.log("e5-ban: A và B đã là bạn, bỏ qua");
  else if (chay("e5-ban")) {
    const kq = {};
    const buoc = [];
    const tenA = await tenCua(A);
    const tenB = await tenCua(B);
    // Once A has a group, Tin nhắn no longer offers adding a friend: Cá nhân → Bạn bè does.
    const ta = await trangMoi(mt, cauHinh("C1"), { persona: P(A), path: "/messages" });
    const cdpA = await cdpCua(ta.page);
    await ta.page.waitForTimeout(2000);
    await tab(ta, cdpA, "Cá nhân");
    await bam(ta, cdpA, "Bạn bè", { batDau: true, cho: 1800 });
    await bam(ta, cdpA, "Thêm bạn bằng số điện thoại", { cho: 1500 });
    kq.them = await duongDan(ta.page);
    await go(ta.page, "Ô số điện thoại bạn", P(B).phone);
    await bam(ta, cdpA, "Tìm", { cho: 2000 });
    kq.the = (await ta.page.evaluate(() => (document.querySelector('[data-testid="danh-thiep"]')?.innerText ?? "").replace(/\s+/g, " ").trim())).slice(0, 160);
    await bam(ta, cdpA, "Gửi lời mời", { cho: 2500 });
    kq.daGui = await coChu(ta.page, /Đã gửi lời mời/);
    await chup(ta.page, { out: mt.out, id: "EV-E5-A1-C1", suKien: ta.suKien });
    buoc.push({ file: anhE("EV-E5-A1-C1"), nhan: "A: gửi lời mời kết bạn" });
    ket({ feature: "E5", tc: "TC-E5-KET-BAN", screen: "E5", state: "A và B chung nhóm E1, chưa là bạn", action: "Cá nhân → «Bạn bè» → «Thêm bạn bằng số điện thoại» → số của B → «Tìm» → «Gửi lời mời»", cauHinh: "C1", expected: "thẻ người tìm thấy mang tên B; gửi xong có câu «Đã gửi lời mời»", status: /\/friends\/add/.test(kq.them) && kq.the.includes(tenB ?? "#") && kq.daGui ? "PASS" : "FAIL", evidence: ["EV-E5-A1-C1"], ghiChu: `tới ${an(kq.them)}; thẻ: «${kq.the}»; tên B trên máy chủ «${tenB}»; «Đã gửi lời mời»: ${kq.daGui ? "có" : "không"}` });

    // B: is there any sign of the request before hunting for it?
    const tb = await trangMoi(mt, cauHinh("C1"), { persona: P(B), path: "/messages" });
    const cdpB = await cdpCua(tb.page);
    await tb.page.waitForTimeout(2500);
    kq.dauTinNhan = await coChu(tb.page, /lời mời kết bạn|muốn kết bạn|kết bạn với bạn/);
    await tab(tb, cdpB, "Cá nhân");
    kq.dauCaNhan = await coChu(tb.page, /lời mời kết bạn|chờ bạn đồng ý|\d+ lời mời/);
    await bam(tb, cdpB, "Bạn bè", { batDau: true, cho: 2000 });
    kq.banBe = await duongDan(tb.page);
    await bam(tb, cdpB, "Đã nhận", { batDau: true, cho: 1000 });
    kq.coNutDongY = !!(await timNut(tb.page, "Đồng ý"));
    await bam(tb, cdpB, "Đồng ý", { cho: 2500 });
    kq.laBan = await laBanAB();
    await chup(tb.page, { out: mt.out, id: "EV-E5-B1-C1", suKien: tb.suKien });
    buoc.push({ file: anhE("EV-E5-B1-C1"), nhan: "B: Bạn bè → Đã nhận → «Đồng ý»" });
    ket({ feature: "E5", tc: "TC-E5-DONG-Y", screen: "E5", state: "A vừa gửi lời mời kết bạn", action: "B mở app; Cá nhân → «Bạn bè» → «Đã nhận» → «Đồng ý»", cauHinh: "C1", expected: "B thấy dấu hiệu có lời mời mà không phải đi tìm (ở Tin nhắn hoặc Cá nhân); đồng ý xong hai người là bạn", status: (kq.dauTinNhan || kq.dauCaNhan) && kq.laBan ? "PASS" : "FAIL", evidence: ["EV-E5-B1-C1"], ghiChu: `dấu ở Tin nhắn: ${kq.dauTinNhan ? "có" : "không"}; dấu ở Cá nhân: ${kq.dauCaNhan ? "có" : "không"}; Bạn bè ở ${an(kq.banBe)}; nút «Đồng ý»: ${kq.coNutDongY ? "có" : "không"}; là bạn trên máy chủ: ${kq.laBan ? "có" : "không"}` });

    // B writes privately; A reads it from Tin nhắn.
    await bam(tb, cdpB, "Đã là bạn", { batDau: true, cho: 800 });
    await bam(tb, cdpB, `Nhắn tin cho ${tenA}`, { cho: 3000 });
    kq.dm = await duongDan(tb.page);
    kq.gui = /\/chat$/.test(kq.dm) ? await guiTin(tb, cdpB, TIN_DM) : false;
    kq.bBong = await bongTin(tb.page, TIN_DM);
    await chup(tb.page, { out: mt.out, id: "EV-E5-B2-C1", suKien: tb.suKien });
    buoc.push({ file: anhE("EV-E5-B2-C1"), nhan: "B: nhắn riêng cho A" });
    await ta.page.goto(`${mt.base}/messages`);
    await ta.page.waitForTimeout(2500);
    await choOn(ta.page, { mang: ta.mang });
    kq.aHang = await coChu(ta.page, new RegExp(TIN_DM.slice(0, 20)));
    kq.aMo = await moHangCo(ta, cdpA, tenB);
    kq.aBong = await bongTin(ta.page, TIN_DM);
    await chup(ta.page, { out: mt.out, id: "EV-E5-A2-C1", suKien: ta.suKien });
    buoc.push({ file: anhE("EV-E5-A2-C1"), nhan: "A: đọc tin riêng" });
    ket({ feature: "E5", tc: "TC-E5-NHAN-RIENG", screen: "E5", state: "vừa thành bạn", action: "B: «Đã là bạn» → «Nhắn tin cho …» → gửi; A: Tin nhắn → hàng chat đôi", cauHinh: "C1", expected: "mở chat hai người; tin của B hiện ở hàng của A trong Tin nhắn và trong chat đôi", status: /\/chat$/.test(kq.dm) && kq.bBong && kq.aHang && kq.aBong ? "PASS" : "FAIL", evidence: ["EV-E5-B2-C1", "EV-E5-A2-C1"], ghiChu: `chat đôi ${an(kq.dm)}; bong bóng của B: ${kq.bBong ? "có" : "không"}; tin trên hàng Tin nhắn của A: ${kq.aHang ? "có" : "không"}; A mở hàng: ${kq.aMo ? "có" : "không"}; bong bóng ở A: ${kq.aBong ? "có" : "không"}` });
    log({ e5ban: kq });
    await ta.context.close();
    await tb.context.close();
  }

  // ------------------------------------------------------------------ E4
  // A proposes the two-person notebook from «Tạo mới»; B learns of it and
  // agrees; A's open screen follows.
  const soDaLap = async () => {
    const cap = await capAB();
    if (!cap) return false;
    const so = await api("GET", `/contexts/${cap.id}/notebook`, undefined, await phienCua(A)).catch(() => null);
    return so?.my_consents?.find?.((c) => c.purpose === "lap_so")?.granted === true;
  };
  if (chay("e4") && (await soDaLap())) console.log("e4: sổ của A và B đã lập, bỏ qua");
  else if (chay("e4")) {
    const kq = {};
    const buoc = [];
    const tenA = await tenCua(A);
    const tenB = await tenCua(B);
    const ta = await trangMoi(mt, cauHinh("C1"), { persona: P(A), path: "/messages" });
    const cdpA = await cdpCua(ta.page);
    await ta.page.waitForTimeout(2000);
    await bam(ta, cdpA, "Tạo mới", { cho: 1500 });
    await bam(ta, cdpA, "Rủ một người đi chơi", { batDau: true, cho: 2000 });
    kq.chon = await duongDan(ta.page);
    kq.coB = await moHangCo(ta, cdpA, tenB);
    kq.giay = await duongDan(ta.page);
    const coDeNghi = !!(await timNut(ta.page, "Đề nghị lập sổ"));
    if (coDeNghi) {
      await bam(ta, cdpA, "Đề nghị lập sổ", { cho: 1200 });
      const trong = await ta.page.evaluate(() => { const d = [...document.querySelectorAll('[role="dialog"]')].pop(); const b = d && [...d.querySelectorAll('[role="button"]')].find((x) => /Đề nghị lập sổ/.test(x.innerText ?? "")); if (!b) return null; const r = b.getBoundingClientRect(); return { x: r.left + r.width / 2, y: r.top + r.height / 2 }; });
      if (trong) await cham(cdpA, trong);
      await ta.page.waitForTimeout(2500);
    }
    // The whole page: chuTrang keeps only the first 400 characters and the sentence sits below the sheet's list.
    kq.aCho = (await ta.page.evaluate(() => (document.body.innerText ?? "").replace(/\s+/g, " "))).match(/Đã đề nghị\.[^.]*\./)?.[0] ?? null;
    await chup(ta.page, { out: mt.out, id: "EV-E4-A1-C1", suKien: ta.suKien });
    buoc.push({ file: anhE("EV-E4-A1-C1"), nhan: "A: đề nghị lập sổ, đang chờ" });
    ket({ feature: "E4", tc: "TC-E4-DE-NGHI", screen: "E4", state: "A và B vừa thành bạn, chưa có sổ hai người", action: "«Tạo mới» → «Rủ một người đi chơi» → B → «Đề nghị lập sổ» → sheet → «Đề nghị lập sổ»", cauHinh: "C1", expected: "tới tờ giấy của cặp; gửi đề nghị xong, màn nói đang chờ B", status: /\/hai-nguoi\/chon-nguoi/.test(kq.chon) && /\/to-giay/.test(kq.giay) && coDeNghi && kq.aCho ? "PASS" : "FAIL", evidence: ["EV-E4-A1-C1"], ghiChu: `chọn người ở ${an(kq.chon)}; hàng B: ${kq.coB ? "có" : "không"}; tờ giấy ${an(kq.giay)}; nút «Đề nghị lập sổ»: ${coDeNghi ? "có" : "không"}; câu chờ: «${kq.aCho ?? "-"}»` });

    // B: a sign of the proposal where B already is (Tin nhắn, the pair chat)?
    const tb = await trangMoi(mt, cauHinh("C1"), { persona: P(B), path: "/messages" });
    const cdpB = await cdpCua(tb.page);
    await tb.page.waitForTimeout(2500);
    const reDeNghi = /đề nghị lập sổ|muốn lập sổ|Xem lời đề nghị|lời đề nghị/;
    kq.bTinNhan = await coChu(tb.page, reDeNghi);
    await moHangCo(tb, cdpB, tenA);
    kq.bChatDoi = await coChu(tb.page, reDeNghi);
    await chup(tb.page, { out: mt.out, id: "EV-E4-B1-C1", suKien: tb.suKien });
    buoc.push({ file: anhE("EV-E4-B1-C1"), nhan: "B: chat đôi, sau khi A đề nghị" });
    await tb.page.goto(`${mt.base}/messages`);
    await tb.page.waitForTimeout(2000);
    await bam(tb, cdpB, "Tạo mới", { cho: 1500 });
    await bam(tb, cdpB, "Rủ một người đi chơi", { batDau: true, cho: 2000 });
    await moHangCo(tb, cdpB, tenA);
    kq.bGiay = await duongDan(tb.page);
    await bam(tb, cdpB, "Xem lời đề nghị", { cho: 1200 });
    const dongY = await tb.page.evaluate(() => { const d = [...document.querySelectorAll('[role="dialog"]')].pop(); const b = d && [...d.querySelectorAll('[role="button"]')].find((x) => (x.innerText ?? "").trim() === "Đồng ý"); if (!b) return null; const r = b.getBoundingClientRect(); return { x: r.left + r.width / 2, y: r.top + r.height / 2 }; });
    if (dongY) await cham(cdpB, dongY);
    const t0 = Date.now();
    let moB = false;
    while (!moB && Date.now() - t0 < 10_000) { moB = await tb.page.evaluate(() => !!document.querySelector('[data-testid="giay-trong"]')); if (!moB) await tb.page.waitForTimeout(300); }
    await tb.page.waitForTimeout(1500);
    await chup(tb.page, { out: mt.out, id: "EV-E4-B2-C1", suKien: tb.suKien });
    buoc.push({ file: anhE("EV-E4-B2-C1"), nhan: "B: đồng ý, sổ mở" });
    ket({ feature: "E4", tc: "TC-E4-DONG-Y", screen: "E4", state: "A vừa đề nghị lập sổ", action: "B mở app, rồi «Tạo mới» → «Rủ một người đi chơi» → A → «Xem lời đề nghị» → «Đồng ý»", cauHinh: "C1", expected: "B thấy có lời đề nghị ở nơi B đang ở (Tin nhắn hoặc chat đôi) mà không phải đi tìm; đồng ý xong sổ mở", status: (kq.bTinNhan || kq.bChatDoi) && dongY && moB ? "PASS" : "FAIL", evidence: ["EV-E4-B1-C1", "EV-E4-B2-C1"], ghiChu: `dấu ở Tin nhắn: ${kq.bTinNhan ? "có" : "không"}; ở chat đôi: ${kq.bChatDoi ? "có" : "không"}; tờ giấy ${an(kq.bGiay)}; «Đồng ý» trong sheet: ${dongY ? "có" : "không"}; sổ mở ở B: ${moB ? "có" : "không"}` });

    // A's screen, still on the paper: does it follow without reload?
    const t1 = Date.now();
    let moA = false;
    while (!moA && Date.now() - t1 < 30_000) { moA = await ta.page.evaluate(() => !!document.querySelector('[data-testid="giay-trong"]')); if (!moA) await ta.page.waitForTimeout(500); }
    kq.msA = Date.now() - t1;
    await chup(ta.page, { out: mt.out, id: "EV-E4-A2-C1", suKien: ta.suKien });
    buoc.push({ file: anhE("EV-E4-A2-C1"), nhan: `A: ${moA ? `sổ mở sau ${Math.round(kq.msA / 100) / 10} s` : "chưa mở sau 30 s"}` });
    ket({ feature: "E4", tc: "TC-E4-A-THEO", screen: "E4", state: "A đang ở tờ giấy, B vừa đồng ý", action: "chờ, không tải lại", cauHinh: "C1", expected: "màn của A tự sang sổ đã mở trong vài giây", status: moA && kq.msA <= 10_000 ? "PASS" : "FAIL", evidence: ["EV-E4-A2-C1"], ghiChu: moA ? `sau ${kq.msA} ms (đo từ lúc sổ đã mở ở B)` : "sau 30 s vẫn chưa" });
    await ghepAnh(mt.browser, buoc, anhE("EV-E4-C1-ghep"), { cao: 760, tieuDe: "E4 sổ hai người: A đề nghị từ «Tạo mới», B đồng ý, C1" });
    log({ e4: kq });
    await ta.context.close();
    await tb.context.close();
  }
  // ------------------------------------------------------------ E5 (b)
  // A blocks B from B's profile, reached from their private chat. What the
  // app promises («không nhắn riêng được nữa. Nhóm chung giữ nguyên.») is
  // checked on the pair chat and the shared group, and the notebook is
  // looked at; then the blocked list and unblocking («Bỏ chặn không tự nối
  // lại tình bạn»).
  const chanAB = async () => {
    const pa = await phienCua(A);
    const pb = await phienCua(B);
    return JSON.stringify((await api("GET", "/people/me/blocked", undefined, pa)).blocked ?? []).includes(pb.person_id);
  };
  if (chay("e5-chan") && !(await chanAB()) && !(await laBanAB())) console.log("e5-chan: đã chặn rồi bỏ chặn ở lượt trước, bỏ qua");
  else if (chay("e5-chan")) {
    const kq = {};
    const tenA = await tenCua(A);
    const tenB = await tenCua(B);
    const cap = await capAB();
    const nhom = await nhomE();
    if (!cap) throw new Error("chưa có chat đôi A–B; chạy e5-ban trước");
    const ta = await trangMoi(mt, cauHinh("C1"), { persona: P(A), path: "/messages" });
    const cdpA = await cdpCua(ta.page);
    await ta.page.waitForTimeout(2000);
    if (!(await chanAB())) {
      await moHangCo(ta, cdpA, tenB);
      kq.dm = await duongDan(ta.page);
      await bam(ta, cdpA, "Xem hồ sơ", { cho: 2000 });
      kq.hoSo = await duongDan(ta.page);
      await bam(ta, cdpA, "Thêm hành động", { cho: 900 });
      await bam(ta, cdpA, `Chặn ${tenB}`, { cho: 800 });
      kq.hoi = (await ta.page.evaluate(() => (document.body.innerText ?? "").replace(/\s+/g, " "))).match(/Chặn [^?]{0,60}\?[^.]*\.[^.]*\./)?.[0] ?? null;
      await bam(ta, cdpA, "Chặn", { cho: 2500 });
      kq.chan = await chanAB();
      await chup(ta.page, { out: mt.out, id: "EV-E5-A3-C1", suKien: ta.suKien });
      ket({ feature: "E5", tc: "TC-E5-CHAN", screen: "E5", state: "A và B là bạn, có chat đôi và sổ hai người, chung nhóm E1", action: "Tin nhắn → chat đôi → «Xem hồ sơ» → «Thêm hành động» → «Chặn …» → «Chặn»", cauHinh: "C1", expected: "từ chat đôi tới hồ sơ và hành động chặn; bước hỏi nói hậu quả; chặn xong máy chủ ghi đã chặn", status: /\/chat$/.test(kq.dm) && /\/people\//.test(kq.hoSo) && kq.hoi && kq.chan ? "PASS" : "FAIL", evidence: ["EV-E5-A3-C1"], ghiChu: `chat đôi ${an(kq.dm)} → hồ sơ ${an(kq.hoSo)}; câu hỏi: «${kq.hoi ?? "-"}»; đã chặn trên máy chủ: ${kq.chan ? "có" : "không"}` });
    }

    // The private chat, both sides: B tries to write.
    const tb = await trangMoi(mt, cauHinh("C1"), { persona: P(B), path: "/messages" });
    const cdpB = await cdpCua(tb.page);
    await tb.page.waitForTimeout(2000);
    await moHangCo(tb, cdpB, tenA);
    kq.bDm = await duongDan(tb.page);
    kq.bOSoan = await tb.page.evaluate(() => !!document.querySelector('textarea[aria-label="Ô soạn tin"]'));
    kq.bGui = kq.bOSoan ? await guiTin(tb, cdpB, TIN_SAU_CHAN) : false;
    await tb.page.waitForTimeout(2000);
    kq.bCau = (await tb.page.evaluate(() => (document.body.innerText ?? "").replace(/\s+/g, " "))).match(/[^.]*(chặn|không gửi được|Chưa gửi|không nhận)[^.]*\./i)?.[0] ?? null;
    await chup(tb.page, { out: mt.out, id: "EV-E5-B3-C1", suKien: tb.suKien });
    await ta.page.goto(`${mt.base}/messages`);
    await ta.page.waitForTimeout(2500);
    await moHangCo(ta, cdpA, tenB);
    await ta.page.waitForTimeout(3000);
    kq.aThayTinSauChan = await bongTin(ta.page, TIN_SAU_CHAN);
    kq.aOSoan = await ta.page.evaluate(() => !!document.querySelector('textarea[aria-label="Ô soạn tin"]'));
    ket({ feature: "E5", tc: "TC-E5-SAU-CHAN-DM", screen: "E5", state: "A vừa chặn B", action: "B mở chat đôi và gửi một tin; A mở chat đôi", cauHinh: "C1", expected: "như app hứa, không nhắn riêng được nữa: B không gửi được (hoặc được báo rõ), A không nhận; A không được mời gõ", status: !kq.aThayTinSauChan && !kq.aOSoan && (!kq.bOSoan || kq.bCau) ? "PASS" : "FAIL", evidence: ["EV-E5-B3-C1"], ghiChu: `B: ô soạn ${kq.bOSoan ? "có" : "không"}, gửi ${kq.bGui ? "chạm được" : "không"}, câu «${kq.bCau ?? "không"}»; A: tin của B ${kq.aThayTinSauChan ? "tới" : "không tới"}, ô soạn ${kq.aOSoan ? "có" : "không"}` });

    // The shared group is kept: B writes there, A reads it.
    const pa = await phienCua(A);
    const pb = await phienCua(B);
    const tv = (await api("GET", `/contexts/${nhom.id}/members`, undefined, pa)).members ?? [];
    kq.bTrongNhom = tv.find((m) => m.person_id === pb.person_id)?.state ?? null;
    await ta.page.goto(`${mt.base}/groups/${nhom.id}/chat`);
    await ta.page.waitForTimeout(2500);
    await tb.page.goto(`${mt.base}/groups/${nhom.id}/chat`);
    await tb.page.waitForTimeout(2500);
    const doiA = choChu(ta.page, new RegExp(TIN_NHOM_SAU_CHAN.slice(0, 25)), 20_000);
    await guiTin(tb, cdpB, TIN_NHOM_SAU_CHAN);
    kq.msNhom = await doiA;
    await chup(ta.page, { out: mt.out, id: "EV-E5-A4-C1", suKien: ta.suKien });
    ket({ feature: "E5", tc: "TC-E5-SAU-CHAN-NHOM", screen: "E5", state: "A vừa chặn B; hai người chung nhóm E1", action: "B gửi một tin trong nhóm; A đang mở chat nhóm", cauHinh: "C1", expected: "như app hứa, nhóm chung giữ nguyên: B vẫn là thành viên, tin của B tới A", status: kq.bTrongNhom === "active" && kq.msNhom !== null ? "PASS" : "FAIL", evidence: ["EV-E5-A4-C1"], ghiChu: `B trong nhóm: ${kq.bTrongNhom ?? "không thấy"}; tin nhóm của B tới A: ${kq.msNhom === null ? "không, sau 20 s" : `sau ${kq.msNhom} ms`}` });

    // The notebook of the pair, as A opens it now.
    await ta.page.evaluate((p) => { history.pushState({}, "", p); dispatchEvent(new PopStateEvent("popstate")); }, `/groups/${cap.id}/to-giay`);
    await ta.page.waitForTimeout(3000);
    await choOn(ta.page, { mang: ta.mang });
    kq.so = (await ta.page.evaluate(() => (document.body.innerText ?? "").replace(/[-]/g, "").replace(/\s+/g, " "))).slice(0, 400);
    kq.soMoiPhac = await ta.page.evaluate(() => [...document.querySelectorAll('[role="button"]')].filter((e) => e.getBoundingClientRect().width > 0).map((e) => (e.innerText ?? "").replace(/[-]/g, "").trim()).filter((x) => /Rủ đi chơi|Gửi cho|Phác|Sửa trước khi gửi/.test(x)));
    await chup(ta.page, { out: mt.out, id: "EV-E5-A5-C1", suKien: ta.suKien });
    ket({ feature: "E5", tc: "TC-E5-SAU-CHAN-SO", screen: "E5", state: "A vừa chặn B; hai người có sổ hai người đã lập", action: "A mở tờ giấy của cặp", cauHinh: "C1", expected: "sổ nói rõ đang chặn và không mời phác hay gửi tờ mới (kênh riêng đã chặn)", status: /chặn/i.test(kq.so) && kq.soMoiPhac.length === 0 ? "PASS" : "FAIL", evidence: ["EV-E5-A5-C1"], ghiChu: `nút mời phác hay gửi: ${kq.soMoiPhac.join(", ") || "không"}; chữ: «${kq.so.slice(0, 200)}»` });

    // The blocked list, from the profile tab; unblock.
    await ta.page.goto(`${mt.base}/profile`);
    await ta.page.waitForTimeout(2500);
    await bam(ta, cdpA, "Cài đặt", { batDau: true, cho: 1800 });
    await bam(ta, cdpA, "Người đã chặn", { batDau: true, cho: 2000 });
    kq.dsChan = await duongDan(ta.page);
    kq.coB = await coChu(ta.page, new RegExp(tenB));
    await chup(ta.page, { out: mt.out, id: "EV-E5-A6-C1", suKien: ta.suKien });
    ket({ feature: "E5", tc: "TC-E5-DS-CHAN", screen: "E5", state: "A đang chặn B", action: "Cá nhân → «Cài đặt» → «Người đã chặn»", cauHinh: "C1", expected: "B có trong danh sách, kèm nút bỏ chặn", status: /\/settings\/da-chan/.test(kq.dsChan) && kq.coB && !!(await timNut(ta.page, `Bỏ chặn ${tenB}`)) ? "PASS" : "FAIL", evidence: ["EV-E5-A6-C1"], ghiChu: `ở ${an(kq.dsChan)}; tên B: ${kq.coB ? "có" : "không"}` });
    await bam(ta, cdpA, `Bỏ chặn ${tenB}`, { cho: 2500 });
    kq.hetChan = !(await chanAB());
    kq.conBan = await laBanAB();
    const tv2 = (await api("GET", `/contexts/${nhom.id}/members`, undefined, pa)).members ?? [];
    kq.nhomSau = tv2.filter((m) => m.state === "active").length;
    ket({ feature: "E5", tc: "TC-E5-BO-CHAN", screen: "E5", state: "B trong danh sách chặn", action: `«Bỏ chặn ${tenB}»`, cauHinh: "C1", expected: "bỏ chặn xong; như app hứa, không tự nối lại tình bạn; nhóm chung vẫn hai người", status: kq.hetChan && !kq.conBan && kq.nhomSau === 2 ? "PASS" : "FAIL", evidence: [], ghiChu: `hết chặn: ${kq.hetChan ? "có" : "không"}; còn là bạn: ${kq.conBan ? "có" : "không"}; thành viên nhóm E1: ${kq.nhomSau}` });
    await ghepAnh(mt.browser, [
      { file: anhE("EV-E5-A1-C1"), nhan: "A: gửi lời mời kết bạn" },
      { file: anhE("EV-E5-B2-C1"), nhan: "B: đồng ý, nhắn riêng" },
      { file: anhE("EV-E5-A3-C1"), nhan: "A: chặn B từ hồ sơ" },
      { file: anhE("EV-E5-B3-C1"), nhan: "B: chat đôi sau khi bị chặn" },
      { file: anhE("EV-E5-A5-C1"), nhan: "A: sổ hai người sau khi chặn" },
      { file: anhE("EV-E5-A6-C1"), nhan: "A: Người đã chặn" },
    ], anhE("EV-E5-C1-ghep"), { cao: 760, tieuDe: "E5 kết bạn → nhắn riêng → chặn → sổ và nhóm sau khi chặn → danh sách chặn, C1" });
    log({ e5chan: kq });
    await ta.context.close();
    await tb.context.close();
  }
  // The notebook under a block, from the blocked side: can B still send A a
  // sheet? No code on the paper routes reads the block (grep «block» in
  // routes/repo/service of pair papers: 0), so it is measured. A blocks B
  // over the API (the UI path was measured above), B walks the sheet
  // through the UI, then A unblocks over the API to leave things as found.
  if (chay("e5-chan-so")) {
    const kq = {};
    const pa = await phienCua(A);
    const pb = await phienCua(B);
    const cap = await capAB();
    if (!cap) throw new Error("chưa có chat đôi A–B");
    const toCua = async (p) => (await api("GET", `/contexts/${cap.id}/papers`, undefined, p)).papers ?? [];
    await api("POST", `/people/${pb.person_id}/block`, {}, pa);
    kq.chan = await chanAB();
    const truoc = await toCua(pa);
    const tb = await trangMoi(mt, cauHinh("C1"), { persona: P(B), path: "/messages" });
    const cdpB = await cdpCua(tb.page);
    await tb.page.waitForTimeout(2000);
    await tb.page.evaluate((p) => { history.pushState({}, "", p); dispatchEvent(new PopStateEvent("popstate")); }, `/groups/${cap.id}/to-giay`);
    await tb.page.waitForTimeout(3000);
    await choOn(tb.page, { mang: tb.mang });
    kq.bTruoc = (await tb.page.evaluate(() => (document.body.innerText ?? "").replace(/\s+/g, " "))).slice(0, 300);
    kq.ru = !!(await bam(tb, cdpB, "Rủ đi chơi", { cho: 3000 }));
    kq.gui = !!(await bam(tb, cdpB, "Gửi cho người ấy", { cho: 3000 }));
    kq.bSau = (await tb.page.evaluate(() => (document.body.innerText ?? "").replace(/\s+/g, " "))).match(/[^.]*(Đã gửi|chặn|không gửi|Chưa gửi|thử lại)[^.]*\./i)?.[0] ?? null;
    await chup(tb.page, { out: mt.out, id: "EV-E5-B4-C1", suKien: tb.suKien });
    const sau = await toCua(pa);
    const moi = sau.filter((x) => !truoc.some((y) => y.id === x.id));
    kq.toMoi = moi.map((x) => `${x.state ?? x.status ?? "?"}`);
    kq.guiToiA = moi.some((x) => /sent|gui|da_gui|pending/i.test(`${x.state ?? x.status ?? ""}`));
    const ta = await trangMoi(mt, cauHinh("C1"), { persona: P(A), path: "/messages" });
    await ta.page.waitForTimeout(2000);
    await ta.page.evaluate((p) => { history.pushState({}, "", p); dispatchEvent(new PopStateEvent("popstate")); }, `/groups/${cap.id}/to-giay`);
    await ta.page.waitForTimeout(3000);
    kq.aThay = (await ta.page.evaluate(() => (document.body.innerText ?? "").replace(/\s+/g, " "))).slice(0, 300);
    await chup(ta.page, { out: mt.out, id: "EV-E5-A7-C1", suKien: ta.suKien });
    ket({ feature: "E5", tc: "TC-E5-CHAN-SO-GUI", screen: "E5", state: "A đang chặn B; hai người có sổ hai người đã lập", action: "B mở tờ giấy của cặp → «Rủ đi chơi» → «Gửi cho người ấy»; A mở tờ giấy", cauHinh: "C1", expected: "như app hứa (không nhắn riêng được nữa): B không gửi được tờ nào tới A, và được báo; A không nhận gì", status: moi.length === 0 || !kq.guiToiA ? "PASS" : "FAIL", evidence: ["EV-E5-B4-C1", "EV-E5-A7-C1"], ghiChu: `đã chặn trên máy chủ: ${kq.chan ? "có" : "không"}; B: «Rủ đi chơi» ${kq.ru ? "chạm được" : "không có"}, «Gửi cho người ấy» ${kq.gui ? "chạm được" : "không có"}, câu sau: «${kq.bSau ?? "-"}»; tờ mới trên máy chủ: ${kq.toMoi.join(", ") || "không"}; A thấy: «${kq.aThay.slice(0, 160)}»` });
    await api("DELETE", `/people/${pb.person_id}/block`, undefined, pa);
    kq.hetChan = !(await chanAB());
    log({ e5chanSo: kq });
    await ta.context.close();
    await tb.context.close();
  }
  // ------------------------------------------------------------------ E6
  // The session: a reload on a deep screen resumes; logging out resets, and
  // Back does not bring A's screens back; deep links opened cold, without a
  // session (then signing in from there) and with one.
  const reA = new RegExp(`${TEN_NHOM}|${TEN_KEO}|${TEN_A}`);
  const linkE6 = async () => {
    const nhom = await nhomE();
    const keo = await keoE();
    return [["CHAT", `/groups/${nhom.id}/chat`], ["KEO", `/outings/${keo.id}`], ["QUYET-TOAN", `/settlements/${nhom.id}`], ["TAI-CHINH", "/finance"]];
  };
  if (chay("e6-thoat")) {
    const kq = {};
    const buoc = [];
    const keo = await keoE();
    const re = reA;
    // 1. Reload on the outing.
    const ta = await trangMoi(mt, cauHinh("C1"), { persona: P(A), path: "/plan" });
    const cdpA = await cdpCua(ta.page);
    await ta.page.waitForTimeout(2500);
    await moHangCo(ta, cdpA, TEN_KEO);
    kq.keo = await duongDan(ta.page);
    await ta.page.reload();
    await ta.page.waitForTimeout(3500);
    await choOn(ta.page, { mang: ta.mang });
    kq.sauTaiLai = await duongDan(ta.page);
    kq.conKeo = await coChu(ta.page, new RegExp(TEN_KEO));
    await chup(ta.page, { out: mt.out, id: "EV-E6-A1-C1", suKien: ta.suKien });
    buoc.push({ file: anhE("EV-E6-A1-C1"), nhan: "A: tải lại ở kèo" });
    ket({ feature: "E6", tc: "TC-E6-TAI-LAI", screen: "E6", state: "A đang ở kèo E2, tới từ Lên plan", action: "tải lại trang", cauHinh: "C1", expected: "ở lại đúng kèo, vẫn đăng nhập, nội dung như trước", status: kq.sauTaiLai === kq.keo.split("?")[0] || (kq.sauTaiLai.startsWith(`/outings/${keo.id}`) && kq.conKeo) ? (kq.conKeo ? "PASS" : "FAIL") : "FAIL", evidence: ["EV-E6-A1-C1"], ghiChu: `${an(kq.keo)} → ${an(kq.sauTaiLai)}; tên kèo: ${kq.conKeo ? "có" : "không"}` });

    // 2. Log out from the profile tab, then Back twice, then Tin nhắn.
    await ta.page.goto(`${mt.base}/profile`);
    await ta.page.waitForTimeout(2500);
    await bam(ta, cdpA, "Tài khoản", { batDau: true, cho: 1500 });
    await bam(ta, cdpA, "Đăng xuất", { cho: 3500 });
    kq.sauDangXuat = await duongDan(ta.page);
    await chup(ta.page, { out: mt.out, id: "EV-E6-A2-C1", suKien: ta.suKien });
    buoc.push({ file: anhE("EV-E6-A2-C1"), nhan: `A: đăng xuất, tới ${kq.sauDangXuat}` });
    kq.back = [];
    for (let i = 0; i < 2; i += 1) {
      await ta.page.goBack().catch(() => null);
      await ta.page.waitForTimeout(2000);
      kq.back.push({ duong: await duongDan(ta.page), lo: await coChu(ta.page, re) });
    }
    await chup(ta.page, { out: mt.out, id: "EV-E6-A3-C1", suKien: ta.suKien });
    buoc.push({ file: anhE("EV-E6-A3-C1"), nhan: "A: Back hai lần sau đăng xuất" });
    await ta.page.goto(`${mt.base}/messages`);
    await ta.page.waitForTimeout(2500);
    kq.tinNhanLo = await coChu(ta.page, re);
    const pa = await phienCua(A);
    const r = await fetch(`${mt.api}/people/me`, { headers: { Authorization: `Bearer ${pa.token}` } });
    kq.tokenCu = r.status;
    // The page was signed in with the harness's cached bearer, so logging out ends that very session.
    if (r.status === 401) unlinkSync(join(phienDir, `${A}.json`));
    ket({ feature: "E6", tc: "TC-E6-DANG-XUAT", screen: "E6", state: "A đăng nhập, vừa mở kèo và Cá nhân", action: "Cá nhân → «Tài khoản» → «Đăng xuất»; Back hai lần; mở Tin nhắn", cauHinh: "C1", expected: "về màn chào; Back không đưa lại màn nào mang dữ liệu của A; Tin nhắn không còn nhóm của A", status: kq.sauDangXuat === "/welcome" && kq.back.every((b) => !b.lo) && !kq.tinNhanLo ? "PASS" : "FAIL", evidence: ["EV-E6-A2-C1", "EV-E6-A3-C1"], ghiChu: `sau «Đăng xuất» ở ${kq.sauDangXuat}; Back: ${kq.back.map((b) => `${an(b.duong)} (${b.lo ? "lộ dữ liệu của A" : "không lộ"})`).join(", ")}; Tin nhắn ${kq.tinNhanLo ? "còn" : "không còn"} tên nhóm của A; token của chính phiên vừa đăng xuất gọi /people/me: ${kq.tokenCu}` });
    await ta.context.close();
    log({ e6thoat: kq });
  }

  if (chay("e6-khong-phien")) {
    const buoc = [];
    const nhom = await nhomE();
    const re = reA;
    const LINK = await linkE6();
    // 3. Deep links opened cold, without a session.
    for (const [ten, path] of LINK) {
      const t = await trangMoi(mt, cauHinh("C1"), { path });
      await t.page.waitForTimeout(3500);
      await choOn(t.page, { mang: t.mang });
      const den = await duongDan(t.page);
      const lo = await coChu(t.page, re);
      const demo = await coChu(t.page, /^(Dữ liệu demo|Demo)$/);
      const cua = /^\/(welcome|login)/.test(den);
      if (ten === "CHAT") { await chup(t.page, { out: mt.out, id: "EV-E6-L1-C1", suKien: t.suKien }); buoc.push({ file: anhE("EV-E6-L1-C1"), nhan: `link chat, không phiên: ${den}` }); }
      if (ten === "QUYET-TOAN") await chup(t.page, { out: mt.out, id: "EV-E6-L4-C1", suKien: t.suKien });
      ket({ feature: "E6", tc: `TC-E6-LANH-KHONG-PHIEN-${ten}`, screen: "E6", state: "không phiên (trình duyệt khác, hoặc đã đăng xuất)", action: `mở thẳng ${an(path)}`, cauHinh: "C1", expected: ten === "TAI-CHINH" ? "tới cửa vào, hoặc bản demo có nhãn (đường dẫn không mang id; như F11)" : "tới cửa vào (màn chào hoặc đăng nhập); không hiện dữ liệu nào, thật hay demo, dưới id của nhóm hay kèo thật (UI-082)", status: !lo && (cua || (ten === "TAI-CHINH" && demo)) ? "PASS" : "FAIL", evidence: ten === "CHAT" ? ["EV-E6-L1-C1"] : [], ghiChu: `tới ${an(den)}; dữ liệu của A: ${lo ? "có" : "không"}; nhãn demo: ${demo ? "có" : "không"}` });
      await t.context.close();
    }

    // 4. From a cold link to the chat, sign in: does the app come back to it?
    {
      const t = await trangMoi(mt, cauHinh("C1"), { path: `/groups/${nhom.id}/chat` });
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(3000);
      const den0 = await duongDan(t.page);
      let cua;
      if (den0 === "/welcome") cua = await vaoCua(t, cdp, P(A).phone);
      else if (den0 === "/login") {
        await t.page.locator('input[aria-label="Ô số điện thoại"]').fill(P(A).phone);
        await cham(cdp, await tamCua(t.page, '[data-testid="login-gui-ma"]'));
        await t.page.waitForURL("**/otp", { timeout: 10_000 }).catch(() => undefined);
        await t.page.keyboard.type("000000", { delay: 40 });
        await t.page.waitForTimeout(3500);
        cua = { den: await duongDan(t.page) };
      }
      await t.page.waitForTimeout(1500);
      const den = await duongDan(t.page);
      await chup(t.page, { out: mt.out, id: "EV-E6-L2-C1", suKien: t.suKien });
      buoc.push({ file: anhE("EV-E6-L2-C1"), nhan: `đăng nhập từ link chat: tới ${an(den)}` });
      ket({ feature: "E6", tc: "TC-E6-VE-LAI-LINK", screen: "E6", state: "không phiên, mở link chat nhóm", action: "từ cửa vào, đăng nhập bằng OTP qua UI", cauHinh: "C1", expected: "đăng nhập xong quay về đúng chat nhóm của link", status: den === `/groups/${nhom.id}/chat` ? "PASS" : "FAIL", evidence: ["EV-E6-L2-C1"], ghiChu: `link mở ở ${an(den0)}; sau mã tới ${an(cua?.den ?? den)}, cuối cùng ${an(den)}` });
      await t.context.close();
    }

    void buoc;
  }

  if (chay("e6-co-phien")) {
    const LINK = await linkE6();
    // 5. The same links with a session (B).
    for (const [ten, path] of LINK) {
      const t = await trangMoi(mt, cauHinh("C1"), { persona: P(B), path });
      await t.page.waitForTimeout(3500);
      await choOn(t.page, { mang: t.mang });
      const den = await duongDan(t.page);
      // The settlement names the outing, not the group; the chat names the group.
      const noiDung = await coChu(t.page, ten === "TAI-CHINH" ? /Phần chi của bạn/ : ten === "CHAT" ? new RegExp(TEN_NHOM) : new RegExp(TEN_KEO));
      if (ten === "KEO") await chup(t.page, { out: mt.out, id: "EV-E6-L3-C1", suKien: t.suKien });
      ket({ feature: "E6", tc: `TC-E6-LANH-CO-PHIEN-${ten}`, screen: "E6", state: "B đăng nhập (phiên đã lưu)", action: `mở thẳng ${an(path)}`, cauHinh: "C1", expected: "mở đúng màn đó với dữ liệu của nhóm/kèo, có lối quay lại", status: den === path && noiDung ? "PASS" : "FAIL", evidence: ten === "KEO" ? ["EV-E6-L3-C1"] : [], ghiChu: `tới ${an(den)}; nội dung đúng: ${noiDung ? "có" : "không"}` });
      await t.context.close();
    }
    await ghepAnh(mt.browser, [
      { file: anhE("EV-E6-A2-C1"), nhan: "A: đăng xuất, tới /welcome" },
      { file: anhE("EV-E6-A3-C1"), nhan: "A: Back hai lần sau đăng xuất" },
      { file: anhE("EV-E6-L1-C1"), nhan: "không phiên: link chat → /login" },
      { file: anhE("EV-E6-L4-C1"), nhan: "không phiên: link quyết toán → demo" },
      { file: anhE("EV-E6-L2-C1"), nhan: "đăng nhập từ link chat → /explore" },
      { file: anhE("EV-E6-L3-C1"), nhan: "B có phiên: link kèo" },
    ], anhE("EV-E6-C1-ghep"), { cao: 760, tieuDe: "E6 phiên: đăng xuất và Back, link lạnh không phiên, đăng nhập từ link, link có phiên, C1" });
  }
  // Only a picture, from captures already made by e5-chan and e5-chan-so:
  // re-running those would send another sheet under a block.
  if (chay("e5-ghep")) {
    await ghepAnh(mt.browser, [
      { file: anhE("EV-E5-B3-C1"), nhan: "B bị chặn: chat đôi «không còn nhận tin», dải tờ giấy vẫn mời" },
      { file: anhE("EV-E5-B4-C1"), nhan: "B bị chặn: gửi được tờ «Ăn tối Thứ Bảy»" },
      { file: anhE("EV-E5-A7-C1"), nhan: "A (người chặn): nhận tờ, được mời «Ừ»" },
    ], anhE("EV-E5-CHAN-SO-ghep"), { cao: 760, tieuDe: "E5 khi A đang chặn B: sổ hai người vẫn cho B gửi tờ tới A, C1" });
  }
} finally {
  await mt.dong();
}
