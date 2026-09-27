/* F00, Nếp's panel (L03): opened from the margin slip, its question field at
 * the bottom, and the «Vẽ» button whose confirmation is `Alert.alert`
 * (react-native-web ships Alert as an empty function).
 */
import { cauHinh } from "../thu-vien/cau-hinh.mjs";
import { chup } from "../thu-vien/chup.mjs";
import { cdpCua, cham, tamCua } from "../thu-vien/cu-chi.mjs";
import { choOn } from "../thu-vien/dieu-huong.mjs";
import { choDialog, demDialog, dong, inertConLai } from "../thu-vien/lop-phu.mjs";
import { khoiDong, trangMoi } from "../thu-vien/moi-truong.mjs";
import { personaTheoTen } from "../thu-vien/phien.mjs";
import { soGhi } from "../thu-vien/ghi.mjs";

const mt = await khoiDong();
const so = soGhi(mt.out);
const ket = (rec) => {
  so.ghi({ feature: "F00", nenTang: "web", method: "RUNTIME-WEB", screen: "F00.S05", layer: "L03", issue: null, ...rec });
  console.log(`${rec.status.padEnd(10)} ${rec.tc} ${rec.cauHinh} ${rec.ghiChu ?? ""}`);
};
async function moBang(t, cdp) {
  const mep = await tamCua(t.page, '[data-testid="nep-mep"]', { cuon: false });
  if (!mep) return { ok: false, ly: "không có mép Nếp" };
  // `tamCua` gives the centre of the whole slip, most of which sits past the
  // window's right edge; the person taps the 10dp that shows.
  const vw = await t.page.evaluate(() => innerWidth);
  await cham(cdp, { x: vw - 5, y: mep.y });
  await t.page.waitForTimeout(600);
  const dia = await tamCua(t.page, '[data-testid="nep-dia"]', { cuon: false });
  if (!dia) return { ok: false, ly: "chạm mép không rút Nếp ra" };
  await cham(cdp, dia);
  return choDialog(t.page, { co: true });
}
try {
  for (const cfg of ["C1", "C8", "C3"]) {
    const t = await trangMoi(mt, cauHinh(cfg), { persona: personaTheoTen("dalat-0", mt.chatSessions), path: "/explore" });
    const cdp = await cdpCua(t.page);
    const mo = await moBang(t, cdp);
    await t.page.waitForTimeout(800);
    const hinh = await t.page.evaluate(() => {
      const d = [...document.querySelectorAll('[role="dialog"]')].pop();
      const o = d?.querySelector('[data-testid="nep-o-nhap"]')?.getBoundingClientRect();
      const r = d?.getBoundingClientRect();
      return { dTop: r ? Math.round(r.top) : null, dH: r ? Math.round(r.height) : null, oTop: o ? Math.round(o.top) : null, oBottom: o ? Math.round(o.bottom) : null, vh: innerHeight };
    });
    const id = `EV-F00-NEP-BANG-${cfg}`;
    const m = await chup(t.page, { out: mt.out, id, suKien: t.suKien });
    console.log(JSON.stringify({ cfg, mo, hinh, tomTat: m.tomTat }));
    ket({ tc: `TC-L03-MO-${cfg}`, state: "Khám phá", action: "chạm mép Nếp, chạm Nếp", cauHinh: cfg, expected: "bảng Nếp mở; ô hỏi nằm trong màn", status: mo.ok && hinh.oBottom !== null && hinh.oBottom <= hinh.vh ? "PASS" : "FAIL", evidence: [id], ghiChu: `mở ${mo.ok ? `${mo.ms} ms` : mo.ly}; panel top ${hinh.dTop}, cao ${hinh.dH}/${hinh.vh}; ô hỏi ${hinh.oTop}–${hinh.oBottom}` });
    if (cfg === "C1" && mo.ok) {
      // «Vẽ»: type a description, press, and watch for any dialog or request.
      await t.page.locator('[data-testid="nep-o-nhap"]').fill("một con mèo đội nón lá");
      await t.page.waitForTimeout(300);
      const yeuCau = [];
      t.page.on("request", (r) => { if (!r.url().includes("openfreemap")) yeuCau.push(`${r.method()} ${new URL(r.url()).pathname}`); });
      const dialogs = [];
      t.page.on("dialog", (d) => { dialogs.push(d.message()); d.dismiss().catch(() => undefined); });
      const nutVe = await t.page.evaluate(() => {
        const d = [...document.querySelectorAll('[role="dialog"]')].pop();
        const b = [...d.querySelectorAll('[role="button"],button')].find((e) => (e.innerText ?? "").trim() === "Vẽ" || e.getAttribute("aria-label") === "Vẽ");
        if (!b) return null;
        const r = b.getBoundingClientRect();
        return { x: r.left + r.width / 2, y: r.top + r.height / 2, disabled: b.getAttribute("aria-disabled") ?? b.disabled ?? null };
      });
      const soDialogTruoc = await demDialog(t.page);
      if (nutVe) await cham(cdp, nutVe);
      await t.page.waitForTimeout(1500);
      const soDialogSau = await demDialog(t.page);
      const chu = await t.page.evaluate(() => [...document.querySelectorAll('[data-testid="nep-dang-ve"],[data-testid="nep-loi-ve"]')].map((e) => e.innerText));
      await chup(t.page, { out: mt.out, id: "EV-F00-NEP-VE-C1", suKien: t.suKien });
      console.log(JSON.stringify({ nutVe, soDialogTruoc, soDialogSau, dialogs, yeuCau, chu }));
      ket({ tc: "TC-L03-VE", layer: "L03", state: "bảng Nếp, đã gõ mô tả", action: "bấm «Vẽ»", cauHinh: "C1", expected: "hỏi xác nhận «Nhờ Nếp vẽ?» rồi vẽ, hoặc báo vì sao không vẽ được", status: nutVe && soDialogSau === soDialogTruoc && dialogs.length === 0 && yeuCau.length === 0 && chu.length === 0 ? "FAIL" : "PASS", evidence: ["EV-F00-NEP-VE-C1"], issue: null, ghiChu: `nút ${nutVe ? "có" : "không thấy"}; dialog mới: ${soDialogSau - soDialogTruoc}; alert trình duyệt: ${dialogs.length}; request: ${yeuCau.length}; chữ trạng thái vẽ: ${chu.length}` });
      // Residue after closing the panel with Escape.
      await dong(t.page, cdp, "esc");
      await t.page.waitForTimeout(1200);
      const du = await inertConLai(t.page);
      ket({ tc: "TC-L03-DONG-esc", state: "bảng Nếp mở", action: "Esc", cauHinh: "C1", expected: "đóng, không sót inert/aria-hidden", status: (await demDialog(t.page)) === 0 && du.length === 0 ? "PASS" : "FAIL", evidence: [], ghiChu: `sót ${du.length}` });
    }
    await t.context.close();
  }
} finally {
  await mt.dong();
}
