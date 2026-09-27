/* F01, the door: Welcome's pager and cover, Login (cold back, validation,
 * server failure, short window), the real OTP door for a brand-new account
 * (wrong code, right code), Sở thích for that new person, and a wrong invite
 * code on /moi.
 *
 *   node kich-ban/f01-vao-cua.mjs [--chi welcome,login,otp,moi]
 *
 * OTP here goes through the UI on purpose (the rest of the audit signs in by
 * cookie). New accounts use numbers no seed uses (`personaMoi`), typed into the
 * field and masked on every capture.
 */
import { join } from "node:path";

import { cauHinh } from "../thu-vien/cau-hinh.mjs";
import { chup, ghepAnh } from "../thu-vien/chup.mjs";
import { quayKhung, ghepKhung } from "../thu-vien/chuyen-dong.mjs";
import { cdpCua, cham, keo, tamCua } from "../thu-vien/cu-chi.mjs";
import { choOn, duongDan } from "../thu-vien/dieu-huong.mjs";
import { loiMayChu, goHet } from "../thu-vien/mang.mjs";
import { khoiDong, trangMoi } from "../thu-vien/moi-truong.mjs";
import { personaMoi } from "../thu-vien/phien.mjs";
import { soGhi } from "../thu-vien/ghi.mjs";

const chi = (() => {
  const i = process.argv.indexOf("--chi");
  return i === -1 ? null : new Set(process.argv[i + 1].split(","));
})();
const chay = (ten) => !chi || chi.has(ten);
const mt = await khoiDong();
const so = soGhi(mt.out);
const ket = (rec) => {
  so.ghi({ feature: "F01", nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, ...rec });
  console.log(`${rec.status.padEnd(10)} ${rec.tc} ${rec.cauHinh ?? ""} ${rec.ghiChu ?? ""}`);
};
const log = (o) => console.log(JSON.stringify(o));
const chuCua = (page, sel) => page.evaluate((s) => [...document.querySelectorAll(s)].map((e) => (e.innerText ?? "").trim()).filter(Boolean), sel);
const loiDuoiForm = (page) => page.evaluate(() => [...document.querySelectorAll('[aria-live="polite"]')].map((e) => (e.innerText ?? "").trim()).filter(Boolean));

try {
  // ------------------------------------------------------------- Welcome
  if (chay("welcome")) {
    const t = await trangMoi(mt, cauHinh("C1"), { path: "/welcome" });
    const cdp = await cdpCua(t.page);
    const trangThai = async () =>
      t.page.evaluate(() => {
        const dots = [...document.querySelectorAll("[aria-label]")].find((e) => /^Trang \d+ trên \d+$/.test(e.getAttribute("aria-label")));
        const sv = [...document.querySelectorAll("div")].find((e) => {
          const s = getComputedStyle(e);
          return (s.overflowX === "auto" || s.overflowX === "scroll") && e.scrollWidth > e.clientWidth + 10;
        });
        return { dots: dots?.getAttribute("aria-label") ?? null, scrollLeft: sv ? Math.round(sv.scrollLeft) : null, rong: sv ? sv.clientWidth : null };
      });
    const buoc = [await trangThai()];
    const tieuDe = [];
    for (let i = 1; i <= 3; i++) {
      await keo(cdp, { x: 340, y: 560 }, { x: 60, y: 565 }, { ms: 260, buoc: 10 });
      await t.page.waitForTimeout(900);
      buoc.push(await trangThai());
      await chup(t.page, { out: mt.out, id: `EV-F01-WEL-trang${i + 1}-C1`, suKien: t.suKien });
      tieuDe.push((await chuCua(t.page, '[role="heading"],h1,h2')).slice(0, 2).join(" / "));
    }
    log({ pager: buoc, tieuDe });
    const khop = buoc.every((b, i) => b.scrollLeft !== null && Math.abs(b.scrollLeft - i * b.rong) <= 2 && b.dots === `Trang ${i + 1} trên 4`);
    ket({ tc: "TC-L30-VUOT", screen: "F01.S01", layer: "L30", state: "Welcome trang 1", action: "vuốt trái 3 lần", cauHinh: "C1", expected: "mỗi lần sang đúng một trang, dừng khít, chấm trang theo đúng", status: khop ? "PASS" : "FAIL", evidence: ["EV-F01-WEL-trang2-C1", "EV-F01-WEL-trang4-C1"], ghiChu: buoc.map((b) => `${b.scrollLeft}/${b.dots}`).join(" → ") });
    // «Tìm hiểu thêm» from the last page wraps to the first.
    await cham(cdp, await tamCua(t.page, '[role="button"][aria-label="Tìm hiểu thêm"],[aria-label="Tìm hiểu thêm"]'));
    await t.page.waitForTimeout(900);
    const sauLink = await trangThai();
    ket({ tc: "TC-L30-TIM-HIEU", screen: "F01.S01", layer: "L30", state: "Welcome trang 4", action: "chạm «Tìm hiểu thêm»", cauHinh: "C1", expected: "sang trang kế (từ trang cuối về trang đầu)", status: sauLink.scrollLeft === 0 && sauLink.dots === "Trang 1 trên 4" ? "PASS" : "FAIL", evidence: [], ghiChu: `${sauLink.scrollLeft}/${sauLink.dots}` });
    await t.context.close();

    // The cover opening (MO09), normal and reduced motion, then /login.
    for (const cfg of ["C1", "C9"]) {
      const w = await trangMoi(mt, cauHinh(cfg), { path: "/welcome" });
      const c2 = await cdpCua(w.page);
      const quay = await quayKhung(w.page, c2, { ms: 1600 });
      await cham(c2, await tamCua(w.page, '[data-testid="welcome-cta"]'));
      const khung = await quay.dung();
      await ghepKhung(mt.browser, khung, join(mt.out, "jpg", `EV-F01-MO09-bia-${cfg}.jpg`), { tieuDe: `MO09 lật bìa Welcome → Đăng nhập, ${cfg}` });
      await choOn(w.page, { mang: w.mang });
      const d = await duongDan(w.page);
      log({ cfg, soKhung: khung.length, moc: khung.map((k) => k.ms).slice(0, 20), sau: d });
      ket({ tc: `TC-MO09-${cfg}`, screen: "F01.S01", state: "Welcome", action: "chạm «Rủ Đi thôi!»", cauHinh: cfg, expected: cfg === "C9" ? "không lật bìa, sang Đăng nhập ngay" : "lật bìa rồi sang Đăng nhập", status: d === "/login" ? "PASS" : "FAIL", evidence: [`EV-F01-MO09-bia-${cfg}`], ghiChu: `tới ${d}; ${khung.length} khung trong 1,6 s (xem ảnh ghép để đọc trình tự)` });
      await w.context.close();
    }
  }

  // ---------------------------------------------------------------- Login
  if (chay("login")) {
    // Cold: the back chevron with no history under it.
    const t = await trangMoi(mt, cauHinh("C1"), { path: "/login" });
    const cdp = await cdpCua(t.page);
    await cham(cdp, await tamCua(t.page, '[aria-label="Quay lại"]'));
    await t.page.waitForTimeout(900);
    const sauBack = await duongDan(t.page);
    await chup(t.page, { out: mt.out, id: "EV-F01-LOGIN-back-lanh-C1", suKien: t.suKien });
    ket({ tc: "TC-F01-LOGIN-BACK-LANH", screen: "F01.S02", state: "mở /login bằng link", action: "chạm «Quay lại»", cauHinh: "C1", expected: "về màn trước hợp lý (Welcome), không đứng im", status: sauBack !== "/login" ? "PASS" : "FAIL", evidence: ["EV-F01-LOGIN-back-lanh-C1"], ghiChu: `sau khi chạm: ${sauBack}` });
    await t.context.close();

    // Validation: a number that is not a Vietnamese mobile.
    const v = await trangMoi(mt, cauHinh("C1"), { path: "/login" });
    const c2 = await cdpCua(v.page);
    await v.page.locator('input[aria-label="Ô số điện thoại"]').fill("12345");
    await cham(c2, await tamCua(v.page, '[data-testid="login-gui-ma"]'));
    await v.page.waitForTimeout(600);
    const loi1 = await loiDuoiForm(v.page);
    const m1 = await chup(v.page, { out: mt.out, id: "EV-F01-LOGIN-so-sai-C1", suKien: v.suKien });
    ket({ tc: "TC-F01-LOGIN-SO-SAI", screen: "F01.S02", state: "gõ 12345", action: "«Gửi mã»", cauHinh: "C1", expected: "một câu lỗi tiếng Việt dưới ô, không gửi", status: loi1.length === 1 && m1.tomTat.maLoi === 0 && (await duongDan(v.page)) === "/login" ? "PASS" : "FAIL", evidence: ["EV-F01-LOGIN-so-sai-C1"], ghiChu: loi1.join(" | ") });

    // Server failure while sending the code.
    await loiMayChu(v.page, "**/auth/otp/request", 503);
    await v.page.locator('input[aria-label="Ô số điện thoại"]').fill(personaMoi(90).phone);
    await cham(c2, await tamCua(v.page, '[data-testid="login-gui-ma"]'));
    await v.page.waitForTimeout(1500);
    const loi2 = await loiDuoiForm(v.page);
    const m2 = await chup(v.page, { out: mt.out, id: "EV-F01-LOGIN-503-C1", suKien: v.suKien });
    ket({ tc: "TC-F01-LOGIN-503", screen: "F01.S02", state: "máy chủ trả 503 khi gửi mã", action: "«Gửi mã»", cauHinh: "C1", expected: "một câu lỗi tiếng Việt, không lộ mã lỗi; nút dùng lại được", status: loi2.length >= 1 && m2.tomTat.maLoi === 0 && !loi2.some((x) => /audit_injected|503|error/i.test(x)) ? "PASS" : "FAIL", evidence: ["EV-F01-LOGIN-503-C1"], ghiChu: loi2.join(" | ") });
    await goHet(v.page);
    await v.context.close();

    // Short window: is «Gửi mã» on screen without scrolling?
    const s = await trangMoi(mt, cauHinh("C8"), { path: "/login" });
    const nut = await tamCua(s.page, '[data-testid="login-gui-ma"]', { cuon: false });
    await chup(s.page, { out: mt.out, id: "EV-F01-LOGIN-C8", suKien: s.suKien });
    ket({ tc: "TC-F01-LOGIN-C8", screen: "F01.S02", state: "cửa sổ 390×460", action: "mở /login", cauHinh: "C8", expected: "ô số và «Gửi mã» nằm trong màn không phải cuộn", status: nut && nut.bottom <= 460 ? "PASS" : "FAIL", evidence: ["EV-F01-LOGIN-C8"], ghiChu: nut ? `nút ${Math.round(nut.top)}–${Math.round(nut.bottom)} / 460` : "không thấy nút" });
    await s.context.close();
  }

  // ------------------------------------------------ OTP, brand-new account
  if (chay("otp")) {
    const nguoi = personaMoi(Number(process.env.AUDIT_MOI ?? "11"));
    const t = await trangMoi(mt, cauHinh("C1"), { path: "/login" });
    const cdp = await cdpCua(t.page);
    await t.page.locator('input[aria-label="Ô số điện thoại"]').fill(nguoi.phone);
    await cham(cdp, await tamCua(t.page, '[data-testid="login-gui-ma"]'));
    await t.page.waitForURL("**/otp", { timeout: 10_000 }).catch(() => undefined);
    await choOn(t.page, { mang: t.mang });
    const mOtp = await chup(t.page, { out: mt.out, id: "EV-F01-OTP-C1", suKien: t.suKien });
    log({ otp: await duongDan(t.page), tomTat: mOtp.tomTat, chu: (await chuCua(t.page, '[data-testid="otp-screen"] div[dir]')).slice(0, 12) });
    // Wrong code.
    await t.page.keyboard.type("111111", { delay: 40 });
    await t.page.waitForTimeout(1500);
    const loiSai = await loiDuoiForm(t.page);
    // Only the OTP boxes: the Login screen stays mounted under OTP in the stack,
    // and its phone field still holds the number.
    const oTrong = await t.page.evaluate(() => [...document.querySelectorAll('[data-testid="otp-screen"] input')].map((i) => i.value).join(""));
    await chup(t.page, { out: mt.out, id: "EV-F01-OTP-sai-C1", suKien: t.suKien });
    ket({ tc: "TC-F01-OTP-SAI", screen: "F01.S03", state: "mã sai", action: "gõ 111111", cauHinh: "C1", expected: "một câu lỗi (còn bao nhiêu lượt), các ô xoá để gõ lại", status: loiSai.length >= 1 && oTrong === "" ? "PASS" : "FAIL", evidence: ["EV-F01-OTP-sai-C1"], ghiChu: `${loiSai.join(" | ")}; ô sau lỗi: «${oTrong}»` });
    // Right code: a new account goes to Sở thích.
    await t.page.keyboard.type("000000", { delay: 40 });
    await t.page.waitForURL("**/personalization", { timeout: 10_000 }).catch(() => undefined);
    await choOn(t.page, { mang: t.mang });
    const den = await duongDan(t.page);
    await chup(t.page, { out: mt.out, id: "EV-F01-SO-THICH-moi-C1", suKien: t.suKien });
    ket({ tc: "TC-F01-OTP-DUNG", screen: "F01.S03", state: "mã đúng, tài khoản mới", action: "gõ 000000", cauHinh: "C1", expected: "vào Sở thích (người mới)", status: den === "/personalization" ? "PASS" : "FAIL", evidence: ["EV-F01-SO-THICH-moi-C1"], ghiChu: `tới ${den}` });
    // Sở thích for the new person: name, 3 tastes, a budget, save.
    const trangSoThich = await t.page.evaluate(() => ({
      o: [...document.querySelectorAll("input")].map((i) => i.getAttribute("aria-label")),
      chip: [...document.querySelectorAll('[role="checkbox"],[role="button"]')].map((e) => ({ ten: e.getAttribute("aria-label") ?? (e.innerText ?? "").trim().slice(0, 20), checked: e.getAttribute("aria-checked"), selected: e.getAttribute("aria-selected"), role: e.getAttribute("role") })).slice(0, 16),
    }));
    log({ soThich: trangSoThich });
    const nutLuu = async () => t.page.evaluate(() => {
      const b = [...document.querySelectorAll('[role="button"],button')].find((e) => /Lưu sở thích|Xong|Tiếp tục/.test((e.innerText ?? "") + (e.getAttribute("aria-label") ?? "")));
      if (!b) return null;
      b.scrollIntoView({ block: "center", behavior: "instant" });
      const r = b.getBoundingClientRect();
      return { x: Math.round(r.left + r.width / 2), y: Math.round(r.top + r.height / 2), ten: (b.innerText ?? "").trim(), disabled: b.getAttribute("aria-disabled") };
    });
    const luu0 = await nutLuu();
    if (luu0) await cham(cdp, luu0);
    await t.page.waitForTimeout(600);
    const loiChuaChon = await loiDuoiForm(t.page);
    const denChuaChon = await duongDan(t.page);
    ket({ tc: "TC-F01-SO-THICH-CHUA-CHON", screen: "F01.S05", state: "chưa chọn gu", action: "bấm «Lưu sở thích»", cauHinh: "C1", expected: "không lưu; lý do nhìn thấy ngay cạnh nút", status: denChuaChon === "/personalization" ? "PASS" : "FAIL", evidence: ["EV-F01-SO-THICH-moi-C1"], ghiChu: `nút ${JSON.stringify(luu0)}; câu live: ${loiChuaChon.join(" | ") || "không"}` });
    const ten = t.page.locator('input[aria-label="Ô tên của bạn"]');
    if (await ten.count()) await ten.fill("Nguyễn Thị Phương Thảo Hoàng Anh 🌸");
    for (const g of ["Ăn uống", "Cafe", "Chơi đêm"]) {
      const c = await tamCua(t.page, `[aria-label="${g}"]`);
      if (c) await cham(cdp, c);
      await t.page.waitForTimeout(150);
    }
    const chipSau = await t.page.evaluate(() => ["Ăn uống", "Cafe", "Chơi đêm"].map((g) => { const e = document.querySelector(`[aria-label="${g}"]`); return { g, checked: e?.getAttribute("aria-checked"), selected: e?.getAttribute("aria-selected"), role: e?.getAttribute("role") }; }));
    const mc = await tamCua(t.page, '[aria-label*="Rộng tay"]');
    if (mc) await cham(cdp, mc);
    await t.page.waitForTimeout(300);
    await chup(t.page, { out: mt.out, id: "EV-F01-SO-THICH-da-chon-C1", suKien: t.suKien });
    ket({ tc: "TC-F01-SO-THICH-A11Y", screen: "F01.S05", state: "đã chọn 3 gu", action: "đọc thuộc tính ARIA của chip", cauHinh: "C1", expected: "chip đã chọn có aria-checked=true (hoặc aria-selected)", status: chipSau.every((c) => c.checked === "true" || c.selected === "true") ? "PASS" : "FAIL", evidence: ["EV-F01-SO-THICH-da-chon-C1"], issue: chipSau.every((c) => c.checked === "true" || c.selected === "true") ? null : "UI-003", ghiChu: JSON.stringify(chipSau) });
    const luu = await nutLuu();
    if (luu) await cham(cdp, luu);
    await t.page.waitForTimeout(2500);
    await choOn(t.page, { mang: t.mang });
    const denSau = await duongDan(t.page);
    await chup(t.page, { out: mt.out, id: "EV-F01-SAU-SO-THICH-C1", suKien: t.suKien });
    ket({ tc: "TC-F01-SO-THICH-LUU", screen: "F01.S05", state: "đủ 3 gu + mức chi", action: "«Lưu sở thích»", cauHinh: "C1", expected: "lưu và sang màn đầu của người chưa có nhóm", status: denSau !== "/personalization" ? "PASS" : "FAIL", evidence: ["EV-F01-SAU-SO-THICH-C1"], ghiChu: `tới ${denSau}` });
    await t.context.close();
  }

  // ------------------------------------------------------------ wrong invite
  if (chay("moi")) {
    const t = await trangMoi(mt, cauHinh("C1"), { path: "/moi" });
    const cdp = await cdpCua(t.page);
    await t.page.locator('input[aria-label="Mã lời mời"]').fill("khong-phai-ma-that");
    const nut = await t.page.evaluate(() => {
      const b = [...document.querySelectorAll('[role="button"],button')].find((e) => (e.innerText ?? "").includes("Nhận lời mời"));
      const r = b?.getBoundingClientRect();
      return r ? { x: r.left + r.width / 2, y: r.top + r.height / 2 } : null;
    });
    if (nut) await cham(cdp, nut);
    await t.page.waitForTimeout(1800);
    const loi = await loiDuoiForm(t.page);
    const m = await chup(t.page, { out: mt.out, id: "EV-F01-MOI-sai-C1", suKien: t.suKien });
    ket({ tc: "TC-F01-MOI-SAI", screen: "F01.S04", state: "mã lời mời sai", action: "«Nhận lời mời»", cauHinh: "C1", expected: "một câu lỗi tiếng Việt, không lộ mã lỗi", status: loi.length >= 1 && m.tomTat.maLoi === 0 ? "PASS" : "FAIL", evidence: ["EV-F01-MOI-sai-C1"], ghiChu: `${loi.join(" | ")}; http: ${m.suKien.http.map((h) => h.status).join(",")}` });
    await t.context.close();
  }
} finally {
  await mt.dong();
}
