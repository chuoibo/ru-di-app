/* F00, the shell: first route by session, unknown URLs, the tab bar and its
 * rail, the create tray's whole life cycle, the create route opened cold,
 * Nếp's margin slip, a slow session resume, and the stack/tab transitions.
 *
 *   node kich-ban/f00-vo.mjs [--chi a,b,…]   (run only the named parts)
 *
 * Every part writes its evidence and its measurements; the verdicts in the
 * ledger are written by `ket()` from those measurements, and each one names
 * the evidence it rests on. Judgements that need eyes (does it LOOK right)
 * are left NOT_TESTED here and recorded after the screenshot was opened.
 */
import { join } from "node:path";

import { cauHinh } from "../lib/cau-hinh.mjs";
import { chup, ghepAnh } from "../lib/chup.mjs";
import { batDauLayMau, ketThucLayMau, phanTich, quayKhung, ghepKhung } from "../lib/chuyen-dong.mjs";
import { cdpCua, cham, keo, tamCua } from "../lib/cu-chi.mjs";
import { choOn, duongDan, moLanh } from "../lib/dieu-huong.mjs";
import { tre } from "../lib/mang.mjs";
import { choDialog, demDialog, dong, focusHienTai, inertConLai, luoiChamTrang, soLuoi } from "../lib/lop-phu.mjs";
import { khoiDong, trangMoi } from "../lib/moi-truong.mjs";
import { personaTheoTen } from "../lib/phien.mjs";
import { soGhi } from "../lib/ghi.mjs";

const chi = (() => {
  const i = process.argv.indexOf("--chi");
  return i === -1 ? null : new Set(process.argv[i + 1].split(","));
})();
const chay = (ten) => !chi || chi.has(ten);
const mt = await khoiDong();
const so = soGhi(mt.out);
const ket = (rec) => {
  so.ghi({ feature: "F00", nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, ...rec });
  console.log(`${rec.status.padEnd(10)} ${rec.tc} ${rec.cauHinh ?? ""} ${rec.ghiChu ?? ""}`);
};
const P = (ten) => personaTheoTen(ten, mt.chatSessions);
const log = (o) => console.log(JSON.stringify(o));

async function moTrang(cfg, persona, path) {
  return trangMoi(mt, cauHinh(cfg), { persona: persona ? P(persona) : null, path });
}

try {
  // ---------------------------------------------------------------- routing
  if (chay("dinh-tuyen")) {
    const ca = [
      ["dalat-0", "/", "/explore", "người có nhóm đang hoạt động"],
      ["chat-20", "/", "/messages", "người chưa có nhóm"],
      ["dalat-0", "/khong-co-trang-nay", null, "URL lạ khi đã đăng nhập"],
    ];
    for (const [persona, path, mongDoi, moTa] of ca) {
      const t = await moTrang("C1", persona, path);
      await choOn(t.page, { mang: t.mang, toiDa: 15_000 });
      const den = await duongDan(t.page);
      const id = `EV-F00-DT-${persona}-${path === "/" ? "goc" : "la"}`;
      await chup(t.page, { out: mt.out, id, suKien: t.suKien, on: t.on });
      log({ ca: moTa, path, den });
      if (mongDoi) {
        ket({ tc: `TC-F00-DT-${persona}`, screen: "F00.S01", state: moTa, action: `mở ${path}`, cauHinh: "C1", expected: `đi tới ${mongDoi}`, status: den.startsWith(mongDoi) ? "PASS" : "FAIL", evidence: [id], ghiChu: `tới ${den}` });
      } else {
        ket({ tc: "TC-F00-LA-DANG-NHAP", screen: "F00.S02", state: moTa, action: `mở ${path}`, cauHinh: "C1", expected: "không lạc vào màn chào/đăng nhập khi đã có phiên", status: den.startsWith("/welcome") || den.startsWith("/login") ? "FAIL" : "PASS", evidence: [id], ghiChu: `tới ${den}` });
      }
      await t.context.close();
    }
  }

  // ------------------------------------------------------- tab bar and rail
  if (chay("tab")) {
    const anh = [];
    for (const cfg of ["C1", "C2", "C3", "B599", "B600", "C6", "B839", "B840", "C7"]) {
      const t = await moTrang(cfg, "dalat-0", "/explore");
      const tabs = await t.page.evaluate(() =>
        [...document.querySelectorAll('[role="tab"]')].map((e) => {
          const r = e.getBoundingClientRect();
          return { ten: e.getAttribute("aria-label"), chon: e.getAttribute("aria-selected"), x: Math.round(r.left), y: Math.round(r.top), w: Math.round(r.width), h: Math.round(r.height) };
        }),
      );
      const coTablist = await t.page.evaluate(() => !!document.querySelector('[role="tablist"]'));
      const id = `EV-F00-TAB-${cfg}`;
      const m = await chup(t.page, { out: mt.out, id, suKien: t.suKien, on: t.on });
      anh.push({ file: join(mt.out, "jpg", `${id}.jpg`), nhan: `${cfg} ${cauHinh(cfg).width}×${cauHinh(cfg).height}` });
      const rail = tabs.length && tabs.every((x) => x.x < 110) && new Set(tabs.map((x) => x.y)).size === tabs.length;
      log({ cfg, rail, coTablist, tabs, tomTat: m.tomTat });
      await t.context.close();
    }
    await ghepAnh(mt.browser, anh.slice(0, 5), join(mt.out, "jpg", "EV-F00-TAB-ghep-a.jpg"), { tieuDe: "Thanh tab: C1, C2, C3, biên 599, biên 600", cao: 760 });
    await ghepAnh(mt.browser, anh.slice(5), join(mt.out, "jpg", "EV-F00-TAB-ghep-b.jpg"), { tieuDe: "Rail: C6, biên 839, biên 840, C7", cao: 760 });

    // Switching tabs at C1: selection, URL and the indicator follow the tap.
    const t = await moTrang("C1", "dalat-0", "/explore");
    const cdp = await cdpCua(t.page);
    const buoc = [];
    for (const ten of ["Lên plan", "Tin nhắn", "Cá nhân", "Khám phá"]) {
      const c = await tamCua(t.page, `[role="tab"][aria-label="${ten}"]`);
      await cham(cdp, c);
      await choOn(t.page, { mang: t.mang });
      const s = await t.page.evaluate(() => {
        const chon = [...document.querySelectorAll('[role="tab"]')].find((e) => e.getAttribute("aria-selected") === "true");
        return { chon: chon?.getAttribute("aria-label") ?? null, duong: location.pathname };
      });
      buoc.push({ bam: ten, ...s });
    }
    log({ doiTab: buoc });
    const dung = buoc.every((b) => b.chon === b.bam);
    ket({ tc: "TC-F00-TAB-DOI", screen: "F00.S03", layer: "L05", state: "4 tab", action: "chạm lần lượt 4 tab", cauHinh: "C1", expected: "tab được chọn, URL và nội dung đổi theo chạm", status: dung ? "PASS" : "FAIL", evidence: ["EV-F00-TAB-C1"], ghiChu: buoc.map((b) => `${b.bam}→${b.duong}`).join(", ") });
    await t.context.close();
  }

  // ------------------------------------------------ create tray: life cycle
  if (chay("khay-tao")) {
    const cachDong = ["x", "nen", "esc", "back", "keo-ngan", "keo-dai", "fling"];
    const ketQua = [];
    for (const cach of cachDong) {
      const t = await moTrang("C1", "dalat-0", "/explore");
      const cdp = await cdpCua(t.page);
      const truoc = await luoiChamTrang(t.page);
      const fab = await tamCua(t.page, '[aria-label="Tạo mới"][role="button"]');
      await cham(cdp, fab);
      const mo = await choDialog(t.page, { co: true });
      await t.page.waitForTimeout(700);
      const focusMo = await focusHienTai(t.page);
      const inertMo = await inertConLai(t.page);
      if (cach === "x") await chup(t.page, { out: mt.out, id: "EV-F00-KT-mo-C1", suKien: t.suKien });
      const d = await dong(t.page, cdp, cach);
      await t.page.waitForTimeout(cach === "keo-ngan" ? 900 : 1200);
      const conMo = await demDialog(t.page);
      await choOn(t.page, { mang: t.mang, choThem: 200 });
      const sau = await luoiChamTrang(t.page);
      const khac = soLuoi(truoc, sau);
      const inertSau = await inertConLai(t.page);
      const focusSau = await focusHienTai(t.page);
      const duong = await duongDan(t.page);
      ketQua.push({ cach, mo: mo.ok, msMo: mo.ms, focusMo, inertMo: inertMo.length, lam: d.lam, ly: d.ly, conMo, khac: khac.length, inertSau: inertSau.length, focusSau, duong });
      if (cach === "keo-ngan" || (conMo > 0 && cach !== "keo-ngan") || khac.length) {
        await chup(t.page, { out: mt.out, id: `EV-F00-KT-sau-${cach}-C1`, suKien: t.suKien });
      }
      await t.context.close();
    }
    log({ khayTao: ketQua });
    for (const k of ketQua) {
      const muonMo = k.cach === "keo-ngan";
      const dat = k.mo && k.lam && (muonMo ? k.conMo === 1 : k.conMo === 0 && k.khac === 0 && k.inertSau === 0);
      ket({ tc: `TC-L01-DONG-${k.cach}`, screen: "F00.S04", layer: "L01", state: "khay đang mở", action: `đóng bằng ${k.cach}`, cauHinh: "C1", expected: muonMo ? "bật về, vẫn mở" : "đóng hẳn, không sót lớp chặn, không còn inert", status: dat ? "PASS" : "FAIL", evidence: ["EV-F00-KT-mo-C1"], ghiChu: `mở ${k.msMo} ms; còn ${k.conMo} dialog; lưới đổi ${k.khac}; inert ${k.inertSau}; focus sau: ${k.focusSau?.ten ?? "body"}; ${k.duong}${k.ly ? `; ${k.ly}` : ""}` });
    }
    const f = ketQua[0];
    ket({ tc: "TC-L01-FOCUS", screen: "F00.S04", layer: "L01", state: "mở khay", action: "mở rồi đóng bằng X", cauHinh: "C1", expected: "focus vào trong khay khi mở; trả về nút «Tạo mới» khi đóng", status: f.focusMo?.trongDialog && /Tạo mới/.test(f.focusSau?.ten ?? "") ? "PASS" : "FAIL", evidence: ["EV-F00-KT-mo-C1"], ghiChu: `khi mở: ${JSON.stringify(f.focusMo)}; sau khi đóng: ${JSON.stringify(f.focusSau)}` });

    // Double tap on the FAB, and close-while-opening.
    {
      const t = await moTrang("C1", "dalat-0", "/explore");
      const cdp = await cdpCua(t.page);
      const fab = await tamCua(t.page, '[aria-label="Tạo mới"][role="button"]');
      await cham(cdp, fab, 20);
      await t.page.waitForTimeout(60);
      await cham(cdp, fab, 20).catch(() => undefined);
      await t.page.waitForTimeout(1200);
      const n = await demDialog(t.page);
      const soSheet = await t.page.evaluate(() => document.querySelectorAll('[data-testid="create-sheet"]').length);
      await chup(t.page, { out: mt.out, id: "EV-F00-KT-cham-dup-C1", suKien: t.suKien });
      ket({ tc: "TC-L01-CHAM-DUP", screen: "F00.S04", layer: "L01", state: "khay đóng", action: "chạm «+» hai lần cách 60 ms", cauHinh: "C1", expected: "đúng một khay", status: n === 1 && soSheet === 1 ? "PASS" : "FAIL", evidence: ["EV-F00-KT-cham-dup-C1"], ghiChu: `dialog ${n}, create-sheet ${soSheet}` });
      await t.page.keyboard.press("Escape");
      await t.page.waitForTimeout(1200);
      const truoc = await luoiChamTrang(t.page);
      await cham(cdp, fab, 20);
      await t.page.waitForTimeout(50);
      await t.page.keyboard.press("Escape");
      await t.page.waitForTimeout(1400);
      const n2 = await demDialog(t.page);
      const khac2 = soLuoi(truoc, await luoiChamTrang(t.page));
      await cham(cdp, fab, 20);
      const lai = await choDialog(t.page, { co: true });
      ket({ tc: "TC-L01-NGAT", screen: "F00.S04", layer: "L01", state: "khay đang mở dở", action: "Esc 50 ms sau khi chạm «+», rồi mở lại", cauHinh: "C1", expected: "đóng sạch; mở lại được", status: n2 === 0 && khac2.length === 0 && lai.ok ? "PASS" : "FAIL", evidence: [], ghiChu: `còn ${n2} dialog; lưới đổi ${khac2.length}; mở lại ${lai.ok ? `sau ${lai.ms} ms` : "KHÔNG"}` });
      await t.context.close();
    }

    // Sizes: small, short, tablet. The close button must stay on screen.
    for (const cfg of ["C2", "C8", "C6", "C7", "C3"]) {
      const t = await moTrang(cfg, "dalat-0", "/explore");
      const cdp = await cdpCua(t.page);
      await cham(cdp, await tamCua(t.page, '[aria-label="Tạo mới"][role="button"]'));
      await choDialog(t.page, { co: true });
      await t.page.waitForTimeout(900);
      const hinh = await t.page.evaluate(() => {
        const d = document.querySelector('[role="dialog"]');
        const x = d?.querySelector('[aria-label="Đóng bảng"]')?.getBoundingClientRect();
        const r = d?.getBoundingClientRect();
        const cuon = [...(d?.querySelectorAll("div") ?? [])].find((e) => {
          const s = getComputedStyle(e);
          return (s.overflowY === "auto" || s.overflowY === "scroll") && e.scrollHeight > e.clientHeight + 1;
        });
        return { top: r ? Math.round(r.top) : null, h: r ? Math.round(r.height) : null, w: r ? Math.round(r.width) : null, xTop: x ? Math.round(x.top) : null, cuonDuoc: !!cuon, vh: innerHeight, vw: innerWidth };
      });
      const id = `EV-F00-KT-mo-${cfg}`;
      const m = await chup(t.page, { out: mt.out, id, suKien: t.suKien });
      log({ cfg, hinh, tomTat: m.tomTat });
      ket({ tc: `TC-L01-KICH-${cfg}`, screen: "F00.S04", layer: "L01", state: "khay mở", action: "mở khay", cauHinh: cfg, expected: "panel ≤82% cao, nút đóng trong màn, nội dung dài cuộn được, không tràn", status: hinh.xTop !== null && hinh.xTop >= 0 && m.tomTat.tranTrangPx === 0 ? "PASS" : "FAIL", evidence: [id], ghiChu: `panel top ${hinh.top}, cao ${hinh.h}/${hinh.vh}, rộng ${hinh.w}/${hinh.vw}, nút đóng top ${hinh.xTop}, cuộn ${hinh.cuonDuoc}` });
      await t.context.close();
    }
  }

  // ------------------------------------------------------ /create opened cold
  if (chay("create-lanh")) {
    const t = await moTrang("C1", "dalat-0", "/create");
    await t.page.waitForTimeout(1500);
    await choOn(t.page, { mang: t.mang });
    const duong = await duongDan(t.page);
    const n = await demDialog(t.page);
    await chup(t.page, { out: mt.out, id: "EV-F00-CREATE-LANH-C1", suKien: t.suKien, on: t.on });
    const cdp = await cdpCua(t.page);
    await dong(t.page, cdp, "x");
    await t.page.waitForTimeout(1200);
    const duongSau = await duongDan(t.page);
    ket({ tc: "TC-F00-CREATE-LANH", screen: "F00.S04", layer: "L01", state: "mở lạnh bằng link", action: "mở /create rồi đóng", cauHinh: "C1", expected: "khay mở trên tab Khám phá; đóng thì còn Khám phá", status: n === 1 && duongSau.startsWith("/explore") ? "PASS" : "FAIL", evidence: ["EV-F00-CREATE-LANH-C1"], ghiChu: `lúc mở ${duong}, ${n} dialog; sau khi đóng ${duongSau}` });
    await t.context.close();
  }

  // --------------------------------------------------- slow session resume
  if (chay("resume-cham")) {
    const ch = cauHinh("C1");
    const { taoContext } = await import("../lib/trinh-duyet.mjs");
    const { layPhien, ganPhien } = await import("../lib/phien.mjs");
    const context = await taoContext(mt.browser, ch);
    const page = await context.newPage();
    const phien = await layPhien(mt.api, P("dalat-0"), `${mt.out}/phien`);
    await page.goto(new URL("/favicon.ico", mt.base).toString(), { waitUntil: "commit" }).catch(() => undefined);
    await ganPhien(page, mt.api, phien);
    await tre(page, "**/sessions/web/resume", 5000);
    await page.goto(new URL("/plan", mt.base).toString(), { waitUntil: "domcontentloaded" });
    await page.waitForTimeout(1800);
    const chu = await page.evaluate(() => (document.body.innerText ?? "").trim().length);
    const coTai = await page.evaluate(() => document.querySelectorAll('[role="progressbar"]').length);
    await chup(page, { out: mt.out, id: "EV-F00-RESUME-CHAM-1800ms-C1" });
    await page.waitForTimeout(5000);
    const chuSau = await page.evaluate(() => (document.body.innerText ?? "").trim().length);
    await chup(page, { out: mt.out, id: "EV-F00-RESUME-CHAM-6800ms-C1" });
    log({ resumeCham: { chu1800: chu, coTai, chu6800: chuSau } });
    ket({ tc: "TC-F00-RESUME-CHAM", screen: "F00.S01", layer: "L35", state: "khôi phục phiên chậm (trễ 5 s giả lập)", action: "mở /plan lạnh", cauHinh: "C1", expected: "có chỉ báo đang tải trong lúc chờ, không trang trắng", status: chu === 0 && coTai === 0 ? "FAIL" : "PASS", evidence: ["EV-F00-RESUME-CHAM-1800ms-C1", "EV-F00-RESUME-CHAM-6800ms-C1"], ghiChu: `1,8 s: ${chu} ký tự, ${coTai} progressbar; 6,8 s: ${chuSau} ký tự` });
    await context.close();
  }

  // ---------------------------------------------------- Nếp's margin slip
  if (chay("nep")) {
    const t = await moTrang("C1", "dalat-0", "/explore");
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(800);
    const mep = await t.page.evaluate(() => {
      const e = document.querySelector('[data-testid="nep-mep"]');
      if (!e) return null;
      const r = e.getBoundingClientRect();
      return { x: Math.round(r.left), y: Math.round(r.top), w: Math.round(r.width), h: Math.round(r.height), vw: innerWidth, hienRa: Math.round(innerWidth - r.left), ten: e.getAttribute("aria-label") };
    });
    const m0 = await chup(t.page, { out: mt.out, id: "EV-F00-NEP-mep-C1", suKien: t.suKien, chuThich: [{ selector: '[data-testid="nep-mep"]', nhan: "mép Nếp" }] });
    log({ mep, nep: m0.doDac.nep });
    const scrollTruoc = await t.page.evaluate(() => ({ x: scrollX, sw: document.documentElement.scrollWidth, root: document.getElementById("root")?.scrollLeft ?? 0 }));
    if (mep) await cham(cdp, { x: mep.x + Math.min(5, mep.w / 2), y: mep.y + mep.h / 2 });
    await t.page.waitForTimeout(700);
    const scrollSau = await t.page.evaluate(() => ({ x: scrollX, sw: document.documentElement.scrollWidth, root: document.getElementById("root")?.scrollLeft ?? 0 }));
    const dia = await t.page.evaluate(() => {
      const e = document.querySelector('[data-testid="nep-dia"]');
      if (!e) return null;
      const r = e.getBoundingClientRect();
      return { x: Math.round(r.left), y: Math.round(r.top), w: Math.round(r.width), h: Math.round(r.height) };
    });
    const m1 = await chup(t.page, { out: mt.out, id: "EV-F00-NEP-rut-C1", suKien: t.suKien });
    await t.page.waitForTimeout(6600);
    const conRa = await t.page.evaluate(() => !!document.querySelector('[data-testid="nep-dia"]'));
    log({ scrollTruoc, scrollSau, dia, conRaSau66s: conRa, nepRut: m1.doDac.nep });
    ket({ tc: "TC-L02-MEP", screen: "F00.S05", layer: "L02", state: "Khám phá, Nếp thu", action: "nhìn mép", cauHinh: "C1", expected: "chỉ lộ mép hẹp trong lề phải, không che chữ", status: mep && mep.hienRa <= 16 && (m0.doDac.nep?.chuBiChe?.length ?? 0) === 0 ? "PASS" : "FAIL", evidence: ["EV-F00-NEP-mep-C1"], ghiChu: `lộ ${mep?.hienRa}dp, cao ${mep?.h}; chữ bị che: ${m0.doDac.nep?.chuBiChe?.length ?? "?"}` });
    ket({ tc: "TC-L02-CHAM-KHONG-CUON", screen: "F00.S05", layer: "L02", state: "Nếp thu", action: "chạm mép", cauHinh: "C1", expected: "Nếp rút ra; trang không cuộn ngang (ADR-0035)", status: dia && scrollSau.x === scrollTruoc.x && scrollSau.root === scrollTruoc.root ? "PASS" : "FAIL", evidence: ["EV-F00-NEP-rut-C1"], ghiChu: `rút ra ${dia ? `${dia.w}×${dia.h}` : "KHÔNG"}; scrollX ${scrollTruoc.x}→${scrollSau.x}` });
    ket({ tc: "TC-L02-TU-THU", screen: "F00.S05", layer: "L02", state: "Nếp rút ra", action: "chờ 6,6 s", cauHinh: "C1", expected: "tự thu vào mép sau 6 s", status: conRa ? "FAIL" : "PASS", evidence: [], ghiChu: conRa ? "vẫn còn rút ra" : "đã thu" });
    await t.context.close();

    // Hidden while a sheet is open; on a money screen only a plain edge that does nothing.
    const t2 = await moTrang("C1", "dalat-0", "/explore");
    const cdp2 = await cdpCua(t2.page);
    await cham(cdp2, await tamCua(t2.page, '[aria-label="Tạo mới"][role="button"]'));
    await choDialog(t2.page, { co: true });
    await t2.page.waitForTimeout(800);
    const nepKhiSheet = await t2.page.evaluate(() => {
      const e = document.querySelector('[data-testid="nep-mep"],[data-testid="nep-dia"]');
      if (!e) return "không có";
      const r = e.getBoundingClientRect();
      let op = 1;
      for (let n = e; n && n !== document.documentElement; n = n.parentElement) op *= Number(getComputedStyle(n).opacity);
      return r.width > 0 && op > 0.05 && r.left < innerWidth ? `hiện (${Math.round(r.left)}, op ${op.toFixed(2)})` : "ẩn";
    });
    ket({ tc: "TC-L02-AN-KHI-SHEET", screen: "F00.S05", layer: "L02", state: "khay tạo mở", action: "nhìn lề phải", cauHinh: "C1", expected: "Nếp không vẽ trên sheet đang mở", status: /hiện/.test(nepKhiSheet) ? "FAIL" : "PASS", evidence: ["EV-F00-KT-mo-C1"], ghiChu: nepKhiSheet });
    await t2.context.close();
  }

  // ------------------------------------------- stack push/pop, reduced motion
  if (chay("chuyen-canh")) {
    for (const cfg of ["C1", "C9"]) {
      const t = await moTrang(cfg, "dalat-0", "/profile");
      const cdp = await cdpCua(t.page);
      const nut = await t.page.evaluate(() => {
        const e = [...document.querySelectorAll('[role="button"],a[href]')].find((x) => /Cài đặt/.test(x.getAttribute("aria-label") ?? x.innerText ?? ""));
        if (!e) return null;
        e.scrollIntoView({ block: "center", behavior: "instant" });
        const r = e.getBoundingClientRect();
        return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
      });
      await t.page.waitForTimeout(300);
      if (!nut) {
        log({ cfg, loi: "không thấy nút Cài đặt" });
        await t.context.close();
        continue;
      }
      const quay = await quayKhung(t.page, cdp, { ms: 1100 });
      await batDauLayMau(t.page, '[data-testid="cai-dat-screen"]');
      await cham(cdp, nut);
      const khung = await quay.dung();
      const mau = await ketThucLayMau(t.page);
      await ghepKhung(mt.browser, khung, join(mt.out, "jpg", `EV-F00-MO01-push-${cfg}.jpg`), { tieuDe: `MO01 push Cá nhân → Cài đặt, ${cfg}` });
      log({ cfg, soKhung: khung.length, mocMs: khung.slice(0, 12).map((k) => k.ms), phanTich: phanTich(mau) });
      await t.context.close();
    }
  }
} finally {
  await mt.dong();
}
