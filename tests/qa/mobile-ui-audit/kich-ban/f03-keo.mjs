/* F03, Plan / Kèo / Hành trình: the plan list, an outing in its two modes
 * (timeline and journey map), the new-stop and attach-place sheets, reordering,
 * check-in, creating an outing, adding a place to an outing, the demo routes,
 * failure and the cold back.
 *
 *   node kich-ban/f03-keo.mjs [--chi nep,che-do,ban-do,ban-do-chang,sheet-them,gan-quan,sap-xep,check-in,tao,chon,demo,loi,dai,lop,tablet,meta]
 *
 * Writes only to the two variant outings of seed-bien-the.mjs and puts their
 * stops back afterwards; the seeded outing «Đà Lạt cuối tuần» is read, and its
 * order is changed only as an unsaved draft that is then discarded.
 */
import { join } from "node:path";

import { CHANG_DAI, KEO_RONG, datKeoBienThe, datLaiChang, donKeoThu } from "../seed-bien-the.mjs";
import { cauHinh } from "../thu-vien/cau-hinh.mjs";
import { chup } from "../thu-vien/chup.mjs";
import { ghepKhung, quayKhung } from "../thu-vien/chuyen-dong.mjs";
import { cdpCua, cham, keo, tamCua } from "../thu-vien/cu-chi.mjs";
import { choOn, duongDan } from "../thu-vien/dieu-huong.mjs";
import { choDialog, demDialog, dong, focusHienTai, inertConLai, luoiChamTrang, soLuoi } from "../thu-vien/lop-phu.mjs";
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
  so.ghi({ feature: "F03", nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, ...rec });
  console.log(`${rec.status.padEnd(10)} ${rec.tc} ${rec.cauHinh ?? ""} ${rec.ghiChu ?? ""}`);
};
const log = (o) => console.log(JSON.stringify(o));
const P = personaTheoTen("dalat-0", mt.chatSessions);
const { dai: DAI, rong: RONG, goc: GOC } = await datKeoBienThe(mt);
const mo = (cfg, path) => trangMoi(mt, cauHinh(cfg), { persona: P, path });
/** Open /plan, then walk into `path` inside the app, so Back has a screen of
 *  the app to return to (a cold open's history starts outside the app). */
const moQuaPlan = async (cfg, path) => {
  const t = await mo(cfg, "/plan");
  await t.page.evaluate((p) => {
    history.pushState({}, "", p);
    dispatchEvent(new PopStateEvent("popstate"));
  }, path);
  await choOn(t.page, { mang: t.mang });
  await t.page.waitForTimeout(600);
  return t;
};

/** A control by its whole visible name (icon glyphs stripped), scrolled into view. */
const timNut = (page, chu, { trong = null } = {}) =>
  page.evaluate(
    ({ chu, trong }) => {
      const ten = (x) => (x.getAttribute("aria-label") || (x.innerText ?? "")).replace(/[-]/g, "").trim();
      const goc = trong ? document.querySelector(trong) : document;
      if (!goc) return null;
      const e = [...goc.querySelectorAll('[role="button"],button,[role="tab"],[role="radio"],a[href]')].find((x) => ten(x) === chu && x.getBoundingClientRect().width > 0);
      if (!e) return null;
      e.scrollIntoView({ block: "center", behavior: "instant" });
      const r = e.getBoundingClientRect();
      return { x: r.left + r.width / 2, y: r.top + r.height / 2, w: Math.round(r.width), h: Math.round(r.height), top: Math.round(r.top), bottom: Math.round(r.bottom), disabled: e.getAttribute("aria-disabled") };
    },
    { chu, trong },
  );
const coChu = (page, re) => page.evaluate((s) => new RegExp(s).test(document.body.innerText ?? ""), re.source);
/** Is the Nếp edge or disc on screen (not only in the DOM)? */
const nepHien = (page) =>
  page.evaluate(() =>
    [...document.querySelectorAll('[data-testid="nep-mep"],[data-testid="nep-dia"]')].some((e) => {
      const r = e.getBoundingClientRect();
      if (r.width < 2 || r.height < 2 || r.right <= 0 || r.left >= innerWidth || r.bottom <= 0 || r.top >= innerHeight) return false;
      for (let n = e; n; n = n.parentElement) {
        const s = getComputedStyle(n);
        if (s.display === "none" || s.visibility === "hidden" || Number(s.opacity) < 0.05) return false;
      }
      return true;
    }),
  );
/** Stop titles in the order the timeline draws them. */
const thuTuChang = (page) => page.evaluate(() => [...document.querySelectorAll('[aria-label^="Chặng "][role="button"]')].filter((e) => e.getBoundingClientRect().height > 0).map((e) => e.getAttribute("aria-label").slice(6)));
const moTheoChe = async (page, cdp, ten) => {
  const n = await timNut(page, ten);
  if (n) await cham(cdp, n);
  await page.waitForTimeout(900);
  return !!n;
};

try {
  // ------------------------------------------------ Nếp on the outing screen
  if (chay("nep")) {
    const t = await mo("C1", "/plan");
    const cdp = await cdpCua(t.page);
    const buoc = [];
    buoc.push({ o: "plan", nep: await nepHien(t.page) });
    const the = await t.page.evaluate(() => {
      const e = [...document.querySelectorAll('[role="button"]')].find((x) => (x.innerText ?? "").includes("Đà Lạt cuối tuần") && x.getBoundingClientRect().height > 60);
      if (!e) return null;
      const r = e.getBoundingClientRect();
      return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
    });
    await cham(cdp, the);
    await choOn(t.page, { mang: t.mang });
    await t.page.waitForTimeout(900);
    buoc.push({ o: `kèo, Lịch trình (${await duongDan(t.page)})`, nep: await nepHien(t.page) });
    await chup(t.page, { out: mt.out, id: "EV-F03-NEP-lich-trinh-C1", suKien: t.suKien });
    await moTheoChe(t.page, cdp, "Bản đồ");
    buoc.push({ o: "kèo, Bản đồ", nep: await nepHien(t.page) });
    await moTheoChe(t.page, cdp, "Lịch trình");
    buoc.push({ o: "kèo, lại Lịch trình", nep: await nepHien(t.page) });
    await t.page.goBack();
    await choOn(t.page, { mang: t.mang });
    await t.page.waitForTimeout(900);
    buoc.push({ o: `quay lại (${await duongDan(t.page)})`, nep: await nepHien(t.page) });
    log({ nep: buoc });
    const lt = buoc[1].nep;
    ket({ tc: "TC-F03-NEP-LICH-TRINH", screen: "F03.S03", state: "kèo ở chế độ Lịch trình (không có bản đồ trên màn)", action: "mở kèo từ Lên plan", cauHinh: "C1", expected: "mép Nếp vẫn ở đó như ở Lên plan; chỉ nhường chỗ khi bản đồ hiện (ManHinhHanhTrinh: bản đồ chạy mép tới mép)", status: lt ? "PASS" : "FAIL", evidence: ["EV-F03-NEP-lich-trinh-C1"], ghiChu: buoc.map((b) => `${b.o}: ${b.nep ? "có Nếp" : "không"}`).join("; ") });
    await t.context.close();
  }

  // ------------------------------------------- Timeline | Map switch, state
  if (chay("che-do")) {
    const t = await mo("C1", `/outings/${GOC}`);
    const cdp = await cdpCua(t.page);
    const trangThai = () =>
      t.page.evaluate(() => {
        const tab = (n) => document.querySelector(`[role="tab"][aria-label="${n}"]`);
        const banDo = [...document.querySelectorAll('[aria-label="Khớp hành trình"],[aria-label="Về Lịch trình"]')].some((e) => e.getBoundingClientRect().height > 0);
        return { lichTrinh: tab("Lịch trình")?.getAttribute("aria-selected"), banDo: tab("Bản đồ")?.getAttribute("aria-selected"), tablist: !!document.querySelector('[role="tablist"]'), coBanDo: banDo };
      });
    const truoc = await trangThai();
    await moTheoChe(t.page, cdp, "Bản đồ");
    const sau = await trangThai();
    await chup(t.page, { out: mt.out, id: "EV-F03-BAN-DO-goc-C1", suKien: t.suKien });
    // Push a screen from map mode and come back: the mode lives in the
    // outing screen, which stays mounted under a push.
    await cham(cdp, await tamCua(t.page, '[aria-label="Thành viên nhóm"]', { cuon: false }));
    await choOn(t.page, { mang: t.mang });
    const quaDuong = await duongDan(t.page);
    await t.page.goBack();
    await choOn(t.page, { mang: t.mang });
    await t.page.waitForTimeout(700);
    const quayVe = await trangThai();
    log({ truoc, sau, quaDuong, quayVe });
    ket({ tc: "TC-F03-CHE-DO", screen: "F03.S03", state: "kèo 3 chặng", action: "chạm «Bản đồ»", cauHinh: "C1", expected: "bản đồ hiện; công tắc báo chế độ đang chọn cho công nghệ hỗ trợ", status: sau.coBanDo && sau.banDo === "true" ? "PASS" : "FAIL", issue: sau.coBanDo && sau.banDo !== "true" ? "UI-003" : null, evidence: ["EV-F03-BAN-DO-goc-C1"], ghiChu: `trước: aria-selected Lịch trình ${truoc.lichTrinh}, Bản đồ ${truoc.banDo}; sau: ${sau.lichTrinh}/${sau.banDo}; có tablist: ${sau.tablist}; bản đồ hiện: ${sau.coBanDo}` });
    ket({ tc: "TC-F03-CHE-DO-GIU", screen: "F03.S03", state: "đang ở Bản đồ", action: `mở «Thành viên nhóm» (${quaDuong}) rồi Back`, cauHinh: "C1", expected: "quay về vẫn ở Bản đồ", status: quayVe.coBanDo ? "PASS" : "FAIL", evidence: [], ghiChu: `sau khi quay về: bản đồ hiện ${quayVe.coBanDo}` });
    await t.context.close();
  }

  // --------------------------------------------- map mode, the empty day
  if (chay("ban-do")) {
    for (const cfg of ["C1", "C2", "C3", "C4", "C8", "C6", "C7"]) {
      const t = await mo(cfg, `/outings/${RONG}`);
      const cdp = await cdpCua(t.page);
      await moTheoChe(t.page, cdp, "Bản đồ");
      await choOn(t.page, { mang: t.mang });
      const ve = await t.page.evaluate(() => {
        const e = [...document.querySelectorAll('[aria-label="Về Lịch trình"]')].find((x) => x.getBoundingClientRect().width > 0);
        if (!e) return null;
        const r = e.getBoundingClientRect();
        let cuon = null;
        for (let n = e.parentElement; n; n = n.parentElement) {
          const s = getComputedStyle(n);
          if ((s.overflowY === "auto" || s.overflowY === "scroll") && n.scrollHeight > n.clientHeight + 1) {
            cuon = n;
            break;
          }
        }
        const c = cuon?.getBoundingClientRect();
        const khung = c ? { top: Math.max(0, c.top), bottom: Math.min(innerHeight, c.bottom) } : { top: 0, bottom: innerHeight };
        const thay = Math.max(0, Math.min(r.bottom, khung.bottom) - Math.max(r.top, khung.top));
        const goi = [...document.querySelectorAll('[role="button"]')].find((x) => (x.innerText ?? "").trim() === "Xem cách đi gọn hơn");
        return { top: Math.round(r.top), bottom: Math.round(r.bottom), khungTop: Math.round(khung.top), khungBottom: Math.round(khung.bottom), phanThay: Math.round((100 * thay) / r.height), phaiCuon: !!cuon, goiDisabled: goi?.getAttribute("aria-disabled") ?? null, goiVien: goi ? getComputedStyle(goi).borderStyle : null };
      });
      const id = `EV-F03-BAN-DO-rong-${cfg}`;
      await chup(t.page, { out: mt.out, id, suKien: t.suKien, chuThich: ve ? [{ rect: { x: 0, y: ve.khungTop, w: cauHinh(cfg).width, h: ve.khungBottom - ve.khungTop }, nhan: `vùng cuộn của trang ngày; «Về Lịch trình» thấy ${ve.phanThay}%` }] : [] });
      log({ cfg, ve });
      ket({ tc: "TC-F03-VE-LICH-TRINH", screen: "F03.S04", layer: "L13", state: "Bản đồ, ngày không có điểm nào", action: "mở Bản đồ", cauHinh: cfg, expected: "«Về Lịch trình» thấy trọn mà không phải cuộn trong trang ngày", status: !ve ? "FAIL" : ve.phanThay >= 99 ? "PASS" : "FAIL", evidence: [id], ghiChu: ve ? `nút ${ve.top}–${ve.bottom}, vùng thấy ${ve.khungTop}–${ve.khungBottom}, thấy ${ve.phanThay}%, trang ngày ${ve.phaiCuon ? "phải cuộn" : "không cuộn"}; «Xem cách đi gọn hơn» aria-disabled ${ve.goiDisabled}, viền ${ve.goiVien}` : "không thấy nút «Về Lịch trình»" });
      await t.context.close();
    }
  }

  // ------------------------------------- stops with a place, on the map
  // Every stop with a catalogue place has coordinates; the map should show one
  // numbered pin per such stop on its day. Stops that carry no day (all stops a
  // multi-day outing gets from the timeline editor or «Thêm vào kèo») are
  // counted separately so a missing pin is traced to that, not to the map.
  if (chay("ban-do-chang")) {
    const n = await (await import("../seed-bien-the.mjs")).moNhom(mt);
    for (const [ten, id] of [["kèo 3 ngày (seed)", GOC], ["kèo 1 ngày, 12 chặng", DAI]]) {
      await datLaiChang(mt, DAI, CHANG_DAI);
      const keoDl = n.keo.find((k) => k.id === id) ?? (await (await import("../seed-bien-the.mjs")).moNhom(mt)).keo.find((k) => k.id === id);
      const coQuan = keoDl.stops.filter((s) => s.place_id).length;
      const khongNgay = keoDl.stops.filter((s) => s.day === null).length;
      const t = await mo("C1", `/outings/${id}`);
      const cdp = await cdpCua(t.page);
      await moTheoChe(t.page, cdp, "Bản đồ");
      await choOn(t.page, { mang: t.mang });
      const ngay = await t.page.evaluate(() => [...document.querySelectorAll('[role="button"],[role="radio"],[role="tab"]')].map((e) => (e.innerText ?? "").replace(/[\uE000-\uF8FF]/g, "").trim()).filter((x) => /^Ngày \d+$/.test(x)));
      const theoNgay = [];
      for (const d of ngay.length ? ngay : ["(một ngày)"]) {
        if (ngay.length) await moTheoChe(t.page, cdp, d);
        theoNgay.push({ d, moc: await t.page.evaluate(() => document.querySelectorAll('[aria-label^="Mốc "]').length), rong: await coChu(t.page, /Ngày này chưa có điểm nào trên bản đồ/) });
      }
      const tong = theoNgay.reduce((a, b) => a + b.moc, 0);
      const idEv = `EV-F03-BAN-DO-chang-${id === GOC ? "goc" : "dai"}-C1`;
      await chup(t.page, { out: mt.out, id: idEv, suKien: t.suKien });
      log({ ten, coQuan, khongNgay, theoNgay });
      ket({ tc: `TC-F03-BAN-DO-CHANG-${id === GOC ? "NHIEU-NGAY" : "MOT-NGAY"}`, screen: "F03.S04", state: `${ten}: ${keoDl.stops.length} chặng, ${coQuan} chặng gắn quán có toạ độ, ${khongNgay} chặng không có ngày`, action: "mở Bản đồ, đi qua từng ngày", cauHinh: "C1", expected: "mỗi chặng gắn quán hiện một mốc ở ngày của nó; chặng chưa xếp ngày được nói ra, không bị lặng lẽ bỏ", status: tong >= coQuan ? "PASS" : "FAIL", evidence: [idEv], ghiChu: `${theoNgay.map((x) => `${x.d}: ${x.moc} mốc${x.rong ? ", «chưa có điểm nào»" : ""}`).join("; ")}; tổng ${tong}/${coQuan}` });
      await t.context.close();
    }
  }

  // ------------------------------------------------ «Chặng mới» sheet (L08)
  if (chay("sheet-them")) {
    const cachDong = ["x", "nen", "esc", "back", "keo-ngan", "keo-dai", "fling"];
    const kq = [];
    for (const cach of cachDong) {
      const t = await moQuaPlan("C1", `/outings/${GOC}`);
      const cdp = await cdpCua(t.page);
      const truoc = await luoiChamTrang(t.page);
      await cham(cdp, await timNut(t.page, "Thêm chặng"));
      const moRa = await choDialog(t.page, { co: true });
      await t.page.waitForTimeout(700);
      const focusMo = await focusHienTai(t.page);
      const inertMo = await inertConLai(t.page);
      if (cach === "x") await chup(t.page, { out: mt.out, id: "EV-F03-THEM-mo-C1", suKien: t.suKien });
      const d = await dong(t.page, cdp, cach);
      await t.page.waitForTimeout(cach === "keo-ngan" ? 900 : 1200);
      const conMo = await demDialog(t.page);
      await choOn(t.page, { mang: t.mang, choThem: 200 });
      const khac = soLuoi(truoc, await luoiChamTrang(t.page));
      const inertSau = await inertConLai(t.page);
      const focusSau = await focusHienTai(t.page);
      const nhanDau = await t.page.evaluate(() => [...document.querySelectorAll('[role="button"]')].some((e) => (e.innerText ?? "").trim() === "Thêm chặng"));
      kq.push({ cach, mo: moRa.ok, msMo: moRa.ms, focusMo, inertMo: inertMo.length, lam: d.lam, ly: d.ly, conMo, khac: khac.length, inertSau: inertSau.length, focusSau, duong: await duongDan(t.page), nhanDau });
      await t.context.close();
    }
    log({ themChang: kq });
    for (const k of kq) {
      const muonMo = k.cach === "keo-ngan";
      const dat = k.mo && k.lam && (muonMo ? k.conMo === 1 : k.conMo === 0 && k.khac === 0 && k.inertSau === 0);
      ket({ tc: `TC-L08-DONG-${k.cach}`, screen: "F03.S03", layer: "L08", state: "sheet «Chặng mới» đang mở", action: `đóng bằng ${k.cach}`, cauHinh: "C1", expected: muonMo ? "bật về, vẫn mở" : "đóng hẳn, không sót lớp chặn, không còn inert; nhãn mục trở lại «Thêm chặng»", status: dat && (muonMo || k.nhanDau) ? "PASS" : "FAIL", evidence: ["EV-F03-THEM-mo-C1"], ghiChu: `mở ${k.msMo} ms; còn ${k.conMo} dialog; lưới đổi ${k.khac}; inert ${k.inertSau}; focus sau: ${k.focusSau?.ten ?? "body"}; nhãn mục «Thêm chặng»: ${k.nhanDau}; ${k.duong}${k.ly ? `; ${k.ly}` : ""}` });
    }
    const f = kq[0];
    ket({ tc: "TC-L08-FOCUS", screen: "F03.S03", layer: "L08", state: "mở sheet «Chặng mới»", action: "mở rồi đóng bằng X", cauHinh: "C1", expected: "focus vào trong sheet khi mở; trả về «Thêm chặng» khi đóng", status: f.focusMo?.trongDialog && /Thêm chặng/.test(f.focusSau?.ten ?? "") ? "PASS" : "FAIL", evidence: ["EV-F03-THEM-mo-C1"], ghiChu: `khi mở: ${JSON.stringify(f.focusMo)}; sau khi đóng: ${JSON.stringify(f.focusSau)}` });

    // Double tap on the section action (it toggles the sheet).
    {
      const t = await mo("C1", `/outings/${GOC}`);
      const cdp = await cdpCua(t.page);
      const n = await timNut(t.page, "Thêm chặng");
      await cham(cdp, n, 20);
      await t.page.waitForTimeout(60);
      await cham(cdp, n, 20).catch(() => undefined);
      await t.page.waitForTimeout(1300);
      const soDialog = await demDialog(t.page);
      await chup(t.page, { out: mt.out, id: "EV-F03-THEM-cham-dup-C1", suKien: t.suKien });
      ket({ tc: "TC-L08-CHAM-DUP", screen: "F03.S03", layer: "L08", state: "sheet đóng", action: "chạm «Thêm chặng» hai lần cách 60 ms", cauHinh: "C1", expected: "đúng một sheet mở", status: soDialog === 1 ? "PASS" : "FAIL", evidence: ["EV-F03-THEM-cham-dup-C1"], ghiChu: `dialog sau 1,3 s: ${soDialog}` });
      await t.context.close();
    }

    // Empty label, then a real stop on the empty outing (put back afterwards).
    {
      const t = await mo("C1", `/outings/${RONG}`);
      const cdp = await cdpCua(t.page);
      await cham(cdp, await timNut(t.page, "Thêm chặng"));
      await choDialog(t.page, { co: true });
      await t.page.waitForTimeout(600);
      await cham(cdp, await timNut(t.page, "Thêm chặng", { trong: '[role="dialog"]' }));
      await t.page.waitForTimeout(900);
      const loi = await t.page.evaluate(() => {
        const d = document.querySelector('[role="dialog"]');
        const e = [...(d?.querySelectorAll("div[dir]") ?? [])].find((x) => /Đặt tên cho chặng|trống|tối đa|Nhãn chặng/.test(x.textContent ?? ""));
        if (!e) return null;
        const r = e.getBoundingClientRect();
        return { chu: e.textContent, trongMan: r.top >= 0 && r.bottom <= innerHeight, live: e.getAttribute("aria-live") };
      });
      await chup(t.page, { out: mt.out, id: "EV-F03-THEM-rong-C1", suKien: t.suKien });
      ket({ tc: "TC-L08-NHAN-RONG", screen: "F03.S03", layer: "L08", state: "sheet «Chặng mới», chưa gõ tên", action: "chạm «Thêm chặng» trong sheet", cauHinh: "C1", expected: "một câu tiếng Việt trong sheet, thấy được, đọc được bởi trình đọc màn hình", status: loi && loi.trongMan ? "PASS" : "FAIL", evidence: ["EV-F03-THEM-rong-C1"], ghiChu: loi ? `«${loi.chu}», trong màn ${loi.trongMan}, aria-live ${loi.live}` : "không có câu nào" });
      await t.page.locator('[role="dialog"] input[aria-label="Ô tên chặng"]').fill("Chặng thử từ harness");
      await cham(cdp, await timNut(t.page, "Thêm chặng", { trong: '[role="dialog"]' }));
      await t.page.waitForTimeout(1600);
      await choOn(t.page, { mang: t.mang });
      const conDialog = await demDialog(t.page);
      const chang = await thuTuChang(t.page);
      await chup(t.page, { out: mt.out, id: "EV-F03-THEM-xong-C1", suKien: t.suKien });
      ket({ tc: "TC-L08-GUI", screen: "F03.S03", layer: "L08", state: "kèo 0 chặng", action: "gõ tên, chạm «Thêm chặng»", cauHinh: "C1", expected: "sheet đóng, chặng mới hiện trong lịch trình", status: conDialog === 0 && chang.includes("Chặng thử từ harness") ? "PASS" : "FAIL", evidence: ["EV-F03-THEM-xong-C1"], ghiChu: `còn ${conDialog} dialog; chặng: ${chang.join(" | ")}` });
      await t.context.close();
      await datLaiChang(mt, RONG, []);
    }

    // Short window and tablet: is the sheet's submit reachable?
    for (const cfg of ["C8", "C2", "C6"]) {
      const t = await mo(cfg, `/outings/${GOC}`);
      const cdp = await cdpCua(t.page);
      await cham(cdp, await timNut(t.page, "Thêm chặng"));
      await choDialog(t.page, { co: true });
      await t.page.waitForTimeout(900);
      const hinh = await t.page.evaluate(() => {
        const d = document.querySelector('[role="dialog"]');
        const r = d?.getBoundingClientRect();
        const nut = [...(d?.querySelectorAll('[role="button"]') ?? [])].find((x) => (x.innerText ?? "").replace(/[-]/g, "").trim() === "Thêm chặng");
        const n = nut?.getBoundingClientRect();
        let cuon = null;
        for (let p = nut?.parentElement; p && p !== d?.parentElement; p = p.parentElement) {
          const s = getComputedStyle(p);
          if ((s.overflowY === "auto" || s.overflowY === "scroll") && p.scrollHeight > p.clientHeight + 1) {
            cuon = p;
            break;
          }
        }
        return { dialog: r ? { top: Math.round(r.top), h: Math.round(r.height), tiLe: Math.round((100 * r.height) / innerHeight) } : null, nut: n ? { top: Math.round(n.top), bottom: Math.round(n.bottom) } : null, trongMan: n ? n.bottom <= innerHeight && n.top >= 0 : false, cuonDuoc: !!cuon };
      });
      const id = `EV-F03-THEM-${cfg}`;
      await chup(t.page, { out: mt.out, id, suKien: t.suKien });
      log({ cfg, hinh });
      ket({ tc: "TC-L08-KICH-THUOC", screen: "F03.S03", layer: "L08", state: "sheet «Chặng mới» mở", action: "đo sheet và nút gửi", cauHinh: cfg, expected: "sheet ≤ 82% chiều cao; nút «Thêm chặng» thấy được hoặc cuộn tới được trong sheet", status: hinh.dialog && hinh.dialog.tiLe <= 82 && (hinh.trongMan || hinh.cuonDuoc) ? "PASS" : "FAIL", evidence: [id], ghiChu: `sheet cao ${hinh.dialog?.h}px (${hinh.dialog?.tiLe}%); nút ${hinh.nut?.top}–${hinh.nut?.bottom}, trong màn ${hinh.trongMan}, cuộn được ${hinh.cuonDuoc}` });
      await t.context.close();
    }
  }

  // ----------------------------------------------------- attach a place (L09)
  if (chay("gan-quan")) {
    await datLaiChang(mt, DAI, CHANG_DAI);
    const t = await mo("C1", `/outings/${DAI}`);
    const cdp = await cdpCua(t.page);
    await cham(cdp, await tamCua(t.page, '[aria-label="Chặng Nghỉ trưa"]'));
    const moRa = await choDialog(t.page, { co: true });
    await t.page.waitForTimeout(700);
    await chup(t.page, { out: mt.out, id: "EV-F03-GAN-mo-C1", suKien: t.suKien });
    const chip = await timNut(t.page, "Lưng Chừng Cafe", { trong: '[role="dialog"]' });
    if (chip) await cham(cdp, chip);
    await t.page.waitForTimeout(1500);
    await choOn(t.page, { mang: t.mang });
    const conDialog = await demDialog(t.page);
    const hang = await t.page.evaluate(() => {
      const e = document.querySelector('[aria-label="Chặng Nghỉ trưa"]');
      return e ? (e.innerText ?? "").replace(/\s+/g, " ").trim() : null;
    });
    await chup(t.page, { out: mt.out, id: "EV-F03-GAN-xong-C1", suKien: t.suKien });
    ket({ tc: "TC-L09-GAN", screen: "F03.S03", layer: "L09", state: "chặng «Nghỉ trưa» chưa có địa điểm", action: "chạm chặng, chọn «Lưng Chừng Cafe»", cauHinh: "C1", expected: "sheet «Gắn địa điểm» mở; chọn xong thì đóng và chặng hiện tên quán", status: moRa.ok && conDialog === 0 && /Lưng Chừng Cafe/.test(hang ?? "") ? "PASS" : "FAIL", evidence: ["EV-F03-GAN-mo-C1", "EV-F03-GAN-xong-C1"], ghiChu: `mở ${moRa.ok ? `${moRa.ms} ms` : "không"}; chip ${chip ? "có" : "không"}; còn ${conDialog} dialog; hàng: «${hang}»` });
    await t.context.close();
    await datLaiChang(mt, DAI, CHANG_DAI);
  }

  // ---------------------------------------------------------- reorder (drag)
  if (chay("sap-xep")) {
    for (const cfg of ["C1", "C9"]) {
      const t = await mo(cfg, `/outings/${GOC}`);
      const cdp = await cdpCua(t.page);
      const truoc = await thuTuChang(t.page);
      const tay = await tamCua(t.page, '[aria-label="Thứ tự Cà phê sáng"]');
      const cuonTruoc = await t.page.evaluate(() => Math.round([...document.querySelectorAll("div")].find((e) => getComputedStyle(e).overflowY === "auto" && e.scrollHeight > e.clientHeight + 20)?.scrollTop ?? -1));
      const quay = await quayKhung(t.page, cdp, { ms: 2600 });
      await keo(cdp, tay, { x: tay.x, y: tay.y + 160 }, { ms: 900, buoc: 24, giuTruocMs: 450, giuCuoiMs: 150 });
      const khung = await quay.dung();
      await t.page.waitForTimeout(600);
      const sau = await thuTuChang(t.page);
      const nhap = await coChu(t.page, /Thứ tự nháp · chưa lưu lên nhóm/);
      const cuonSau = await t.page.evaluate(() => Math.round([...document.querySelectorAll("div")].find((e) => getComputedStyle(e).overflowY === "auto" && e.scrollHeight > e.clientHeight + 20)?.scrollTop ?? -1));
      await ghepKhung(mt.browser, khung, join(mt.out, "jpg", `EV-F03-SAP-XEP-keo-${cfg}.jpg`), { tieuDe: `F03 kéo tay nắm «Cà phê sáng» xuống 160dp, ${cfg}` });
      await chup(t.page, { out: mt.out, id: `EV-F03-SAP-XEP-nhap-${cfg}`, suKien: t.suKien });
      ket({ tc: "TC-F03-SAP-XEP-KEO", screen: "F03.S03", state: "kèo 3 chặng", action: "giữ tay nắm rồi kéo xuống 160dp", cauHinh: cfg, expected: "thứ tự đổi thành bản nháp chưa lưu; trang không cuộn theo tay", status: sau.join("|") !== truoc.join("|") && nhap && cuonSau === cuonTruoc ? "PASS" : "FAIL", evidence: [`EV-F03-SAP-XEP-keo-${cfg}`, `EV-F03-SAP-XEP-nhap-${cfg}`], ghiChu: `trước: ${truoc.join(" → ")}; sau: ${sau.join(" → ")}; bản nháp: ${nhap}; cuộn ${cuonTruoc}→${cuonSau}; ${khung.length} khung` });
      const bo = await timNut(t.page, "Bỏ thứ tự nháp");
      if (bo) await cham(cdp, bo);
      await t.page.waitForTimeout(700);
      const boXong = await thuTuChang(t.page);
      if (cfg === "C1") {
        ket({ tc: "TC-F03-SAP-XEP-BO", screen: "F03.S03", state: "có thứ tự nháp", action: "chạm «Bỏ thứ tự nháp»", cauHinh: cfg, expected: "về thứ tự đã lưu", status: boXong.join("|") === truoc.join("|") ? "PASS" : "FAIL", evidence: [], ghiChu: `sau khi bỏ: ${boXong.join(" → ")}` });
        // The keyboard and screen-reader path: the handle is an adjustable
        // with increment/decrement actions on native.
        const phim = await t.page.evaluate(() => {
          const e = document.querySelector('[aria-label="Thứ tự Cà phê sáng"]');
          e?.focus();
          return { role: e?.getAttribute("role"), tabindex: e?.getAttribute("tabindex"), valuenow: e?.getAttribute("aria-valuenow"), valuemax: e?.getAttribute("aria-valuemax"), focus: document.activeElement === e };
        });
        await t.page.keyboard.press("ArrowDown");
        await t.page.waitForTimeout(500);
        const sauPhim = await thuTuChang(t.page);
        ket({ tc: "TC-F03-SAP-XEP-PHIM", screen: "F03.S03", state: "kèo 3 chặng", action: "focus tay nắm «Thứ tự Cà phê sáng», nhấn mũi tên xuống", cauHinh: cfg, expected: "có lối đổi thứ tự không cần kéo (bàn phím, trình đọc màn hình), như thao tác tăng giảm trên native", status: sauPhim.join("|") !== truoc.join("|") ? "PASS" : "FAIL", evidence: [], ghiChu: `role ${phim.role}, tabindex ${phim.tabindex}, aria-valuenow ${phim.valuenow}, aria-valuemax ${phim.valuemax}, nhận focus ${phim.focus}; sau phím: ${sauPhim.join(" → ")}` });
      }
      await t.context.close();
    }
  }

  // ----------------------------------------------------------------- check-in
  if (chay("check-in")) {
    await datLaiChang(mt, DAI, CHANG_DAI);
    const t = await mo("C1", `/outings/${DAI}`);
    const cdp = await cdpCua(t.page);
    const nut = await t.page.evaluate(() => {
      const hang = document.querySelector('[aria-label="Chặng Làm gốm"]')?.closest('[style*="flex-direction: row"]')?.parentElement ?? null;
      const tatCa = [...document.querySelectorAll('[role="button"]')].filter((e) => (e.innerText ?? "").trim() === "Tôi đã tới");
      const hop = document.querySelector('[aria-label="Chặng Làm gốm"]')?.getBoundingClientRect();
      const gan = tatCa.map((e) => ({ e, r: e.getBoundingClientRect() })).sort((a, b) => Math.abs(a.r.top - (hop?.top ?? 0)) - Math.abs(b.r.top - (hop?.top ?? 0)))[0];
      if (!gan) return null;
      gan.e.scrollIntoView({ block: "center", behavior: "instant" });
      const r = gan.e.getBoundingClientRect();
      return { x: r.left + r.width / 2, y: r.top + r.height / 2, hang: !!hang };
    });
    if (nut) await cham(cdp, nut);
    await t.page.waitForTimeout(1500);
    await choOn(t.page, { mang: t.mang });
    const sau = await t.page.evaluate(() => {
      const e = document.querySelector('[aria-label="Chặng Làm gốm"]');
      const r = e?.getBoundingClientRect();
      const daToi = [...document.querySelectorAll("div[dir]")].filter((x) => (x.textContent ?? "").trim() === "Đã tới").map((x) => x.getBoundingClientRect()).some((q) => r && Math.abs(q.top - r.top) < 60);
      return { chu: (e?.innerText ?? "").replace(/\s+/g, " ").trim(), daToi };
    });
    await chup(t.page, { out: mt.out, id: "EV-F03-CHECK-IN-C1", suKien: t.suKien });
    ket({ tc: "TC-F03-CHECK-IN", screen: "F03.S03", state: "chặng «Làm gốm», chưa ai tới", action: "chạm «Tôi đã tới»", cauHinh: "C1", expected: "nút thành nhãn «Đã tới»; dòng phụ ghi có bạn", status: nut && sau.daToi && /bạn/.test(sau.chu) ? "PASS" : "FAIL", evidence: ["EV-F03-CHECK-IN-C1"], ghiChu: nut ? `hàng: «${sau.chu}»; nhãn Đã tới cạnh hàng: ${sau.daToi}` : "không thấy nút «Tôi đã tới»" });
    await t.context.close();
    await datLaiChang(mt, DAI, CHANG_DAI);
  }

  // ------------------------------------------------------ create an outing
  if (chay("tao")) {
    // Empty title.
    {
      const t = await mo("C1", "/outings/new");
      const cdp = await cdpCua(t.page);
      await cham(cdp, await timNut(t.page, "Tạo kèo"));
      await t.page.waitForTimeout(900);
      const loi = await t.page.evaluate(() => {
        const e = [...document.querySelectorAll("div[dir]")].find((x) => /Đặt tên cho chuyến đi/.test(x.textContent ?? "") && x.children.length === 0);
        if (!e) return null;
        const r = e.getBoundingClientRect();
        const o = document.querySelector('input[aria-label="Ô tên kèo"]')?.getBoundingClientRect();
        return { top: Math.round(r.top), trongMan: r.top >= 0 && r.bottom <= innerHeight, cachO: o ? Math.round(r.top - o.bottom) : null, live: e.getAttribute("aria-live") ?? e.parentElement?.getAttribute("aria-live") };
      });
      await chup(t.page, { out: mt.out, id: "EV-F03-TAO-rong-C1", suKien: t.suKien });
      ket({ tc: "TC-F03-TAO-RONG", screen: "F03.S02", state: "chưa gõ tên kèo", action: "chạm «Tạo kèo»", cauHinh: "C1", expected: "một câu dưới form, thấy được ngay, được đọc to", status: loi && loi.trongMan ? "PASS" : "FAIL", evidence: ["EV-F03-TAO-rong-C1"], ghiChu: loi ? `câu ở y=${loi.top}, trong màn ${loi.trongMan}, cách ô tên ${loi.cachO}px, aria-live ${loi.live}` : "không có câu «Đặt tên cho chuyến đi.»" });

      // Budget chips: role and checked state.
      const chip = await timNut(t.page, "300 nghìn");
      if (chip) await cham(cdp, chip);
      await t.page.waitForTimeout(500);
      const ngan = await t.page.evaluate(() => {
        const e = document.querySelector('[aria-label="300 nghìn"]');
        return { role: e?.getAttribute("role"), checked: e?.getAttribute("aria-checked"), selected: e?.getAttribute("aria-selected"), o: document.querySelector('input[aria-label="Ô ngân sách một người"]')?.value ?? null };
      });
      ket({ tc: "TC-F03-TAO-NGAN-SACH", screen: "F03.S02", state: "form kèo mới", action: "chạm «300 nghìn»", cauHinh: "C1", expected: "ô số đồng nhận 300000; chip báo đã chọn cho công nghệ hỗ trợ", status: ngan.o === "300000" && ngan.checked === "true" ? "PASS" : "FAIL", issue: ngan.checked !== "true" ? "UI-003" : null, evidence: [], ghiChu: `role ${ngan.role}, aria-checked ${ngan.checked}, aria-selected ${ngan.selected}; ô ngân sách «${ngan.o}»` });

      // The month leaf.
      await cham(cdp, await tamCua(t.page, '[data-testid="ngay-di-la"]'));
      await t.page.waitForTimeout(700);
      const lich = await t.page.evaluate(() => {
        const o = [...document.querySelectorAll('[role="button"]')].filter((e) => /tháng \d+ năm \d+/.test(e.getAttribute("aria-label") ?? "") && e.getBoundingClientRect().width > 0);
        const r = o.map((e) => e.getBoundingClientRect());
        return { so: o.length, w: r.length ? Math.round(Math.min(...r.map((x) => x.width))) : null, h: r.length ? Math.round(Math.min(...r.map((x) => x.height))) : null };
      });
      await chup(t.page, { out: mt.out, id: "EV-F03-TAO-lich-C1", suKien: t.suKien });
      ket({ tc: "TC-F03-TAO-LICH", screen: "F03.S02", layer: "L14", state: "form kèo mới", action: "chạm lá «Ngày đi»", cauHinh: "C1", expected: "lịch tháng mở; ô ngày ≥ 44dp (PRODUCT), ≥ 48dp (DESIGN)", status: lich.so >= 28 && lich.w >= 44 && lich.h >= 44 ? "PASS" : "FAIL", evidence: ["EV-F03-TAO-lich-C1"], ghiChu: `${lich.so} ô ngày, nhỏ nhất ${lich.w}×${lich.h}px${lich.w < 48 || lich.h < 48 ? " (dưới 48 của DESIGN)" : ""}` });
      await t.context.close();
    }

    // Title typed, budget left as it looks: the field shows «250000», which is
    // a placeholder, not a value. What does «Tạo kèo» say, and where?
    {
      const t = await mo("C1", "/outings/new");
      const cdp = await cdpCua(t.page);
      await t.page.locator('input[aria-label="Ô tên kèo"]').fill("Kèo thử để trống ngân sách");
      const o = await t.page.evaluate(() => {
        const e = document.querySelector('input[aria-label="Ô ngân sách một người"]');
        return { value: e?.value ?? null, placeholder: e?.getAttribute("placeholder") ?? null };
      });
      await cham(cdp, await timNut(t.page, "Tạo kèo"));
      await t.page.waitForTimeout(1200);
      const d = await duongDan(t.page);
      const loi = await t.page.evaluate(() => {
        const e = [...document.querySelectorAll("div[dir]")].find((x) => /Ngân sách mỗi người|số tiền|Số tiền/.test(x.textContent ?? "") && x.children.length === 0);
        if (!e) return null;
        const r = e.getBoundingClientRect();
        const cta = [...document.querySelectorAll('[role="button"]')].find((x) => (x.innerText ?? "").trim() === "Tạo kèo")?.getBoundingClientRect();
        return { chu: e.textContent, top: Math.round(r.top), bottom: Math.round(r.bottom), trongMan: r.top >= 0 && r.bottom <= (cta ? cta.top : innerHeight), live: e.getAttribute("aria-live") };
      });
      await chup(t.page, { out: mt.out, id: "EV-F03-TAO-ngan-sach-trong-C1", suKien: t.suKien });
      ket({ tc: "TC-F03-TAO-NGAN-SACH-TRONG", screen: "F03.S02", state: `tên đã gõ; ô ngân sách hiện «${o.placeholder}» (placeholder, giá trị «${o.value}»)`, action: "chạm «Tạo kèo»", cauHinh: "C1", expected: "hoặc tạo kèo, hoặc một câu thấy được ngay nói thiếu ngân sách; ô không trông như đã có số", status: d === "/outings/new" && !(loi && loi.trongMan) ? "FAIL" : "PASS", evidence: ["EV-F03-TAO-ngan-sach-trong-C1"], ghiChu: `ở lại ${d}; câu: ${loi ? `«${loi.chu}» ở y=${loi.top}–${loi.bottom}, thấy mà không cuộn: ${loi.trongMan}, aria-live ${loi.live}` : "không có"}` });
      await t.context.close();
    }

    // A real creation, then the M5 moment on the new outing.
    for (const cfg of ["C1", "C9"]) {
      const t = await mo(cfg, "/outings/new");
      const cdp = await cdpCua(t.page);
      await t.page.locator('input[aria-label="Ô tên kèo"]').fill(`Kèo thử tạo từ giao diện ${cfg}`);
      await cham(cdp, await timNut(t.page, "300 nghìn"));
      await t.page.waitForTimeout(300);
      const quay = await quayKhung(t.page, cdp, { ms: 3600 });
      await cham(cdp, await timNut(t.page, "Tạo kèo"));
      const khung = await quay.dung();
      await choOn(t.page, { mang: t.mang });
      const d = await duongDan(t.page);
      await ghepKhung(mt.browser, khung, join(mt.out, "jpg", `EV-F03-TAO-M5-${cfg}.jpg`), { tieuDe: `F03 tạo kèo rồi tới kèo mới (M5), ${cfg}` });
      await chup(t.page, { out: mt.out, id: `EV-F03-TAO-xong-${cfg}`, suKien: t.suKien });
      ket({ tc: "TC-F03-TAO-GUI", screen: "F03.S02", state: "form hợp lệ", action: "gõ tên, chạm «Tạo kèo»", cauHinh: cfg, expected: "tới /outings/<id>?vua=tao, kèo mới hiện tiêu đề", status: /^\/outings\/[0-9a-f-]{36}\?vua=tao/.test(d) && (await coChu(t.page, new RegExp(`Kèo thử tạo từ giao diện ${cfg}`))) ? "PASS" : "FAIL", evidence: [`EV-F03-TAO-xong-${cfg}`, `EV-F03-TAO-M5-${cfg}`], ghiChu: `${d.replace(/[0-9a-f-]{36}/, "<id>")}; ${khung.length} khung` });
      await t.context.close();
    }

    // M5 in pixels, from the moment the new outing's URL appears: the puppet's
    // own box every 150 ms, no screencast running beside it. Reduce Motion must
    // hold one still frame from the start (ADR-0037 D3).
    for (const cfg of ["C1", "C9"]) {
      const t = await mo(cfg, "/outings/new");
      const cdp = await cdpCua(t.page);
      await t.page.locator('input[aria-label="Ô tên kèo"]').fill(`Kèo thử M5 ${cfg}`);
      await cham(cdp, await timNut(t.page, "300 nghìn"));
      await t.page.waitForTimeout(300);
      await cham(cdp, await timNut(t.page, "Tạo kèo"));
      await t.page.waitForURL(/\/outings\/[0-9a-f-]{36}/, { timeout: 10_000 }).catch(() => undefined);
      const t0 = Date.now();
      let hop = null;
      while (!hop && Date.now() - t0 < 3000) {
        hop = await t.page.evaluate(() => {
          const e = [...document.querySelectorAll('[role="img"]')].find((x) => { const r = x.getBoundingClientRect(); return r.top < innerHeight / 2 && r.width >= 40 && r.width <= 200; });
          if (!e) return null;
          const r = e.getBoundingClientRect();
          return { x: Math.max(0, r.left - 8), y: Math.max(0, r.top - 8), width: r.width + 16, height: r.height + 16, ten: e.getAttribute("aria-label") };
        });
        if (!hop) await t.page.waitForTimeout(40);
      }
      const bam = [];
      const { createHash } = await import("node:crypto");
      if (hop) {
        for (let i = 0; i < 14; i++) {
          const buf = await t.page.screenshot({ clip: { x: hop.x, y: hop.y, width: hop.width, height: hop.height } });
          const ve = await t.page.evaluate((h) => {
            const e = [...document.querySelectorAll('[role="img"]')].find((x) => { const r = x.getBoundingClientRect(); return Math.abs(r.top - (h.y + 8)) < 4; });
            return [...(e?.querySelectorAll("[data-renderer]") ?? [])].map((x) => x.getAttribute("data-renderer")).join("+") || "-";
          }, hop);
          bam.push({ ms: Date.now() - t0, h: createHash("sha1").update(buf).digest("hex").slice(0, 8), ve });
          await t.page.waitForTimeout(150);
        }
      }
      const khac = new Set(bam.map((b) => b.h)).size;
      const doiCuoi = bam.reduce((m, b, i) => (i > 0 && b.h !== bam[i - 1].h ? b.ms : m), 0);
      ket({ tc: "TC-MO-M5", screen: "F03.S03", state: "kèo vừa tạo (?vua=tao)", action: "tới kèo sau «Tạo kèo»", cauHinh: cfg, expected: cfg === "C9" ? "Nếp hiện cạnh tiêu đề ở khung cuối, đứng yên ngay từ đầu" : "Nếp diễn một lần cạnh tiêu đề rồi đứng yên, trong trần 1400 ms", status: !hop ? "FAIL" : cfg === "C9" ? (khac === 1 ? "PASS" : "FAIL") : khac > 1 && doiCuoi <= 1400 + 400 ? "PASS" : "FAIL", evidence: [`EV-F03-TAO-M5-${cfg}`], ghiChu: hop ? `«${hop.ten}»; 14 ảnh vùng Nếp mỗi 150 ms từ lúc URL đổi: ${khac} ảnh khác nhau; đổi lần cuối ở ${doiCuoi} ms (${bam.map((b) => `${b.ms}:${b.h.slice(0, 4)}/${b.ve}`).join(" ")})` : "không thấy Nếp cạnh tiêu đề trong 3 s" });
      await t.context.close();
    }

    // The outings made above would otherwise pile up in «Lên plan».
    await donKeoThu(mt);

    // Short window: the sticky «Tạo kèo» must stay reachable.
    {
      const t = await mo("C8", "/outings/new");
      const n = await t.page.evaluate(() => {
        const e = [...document.querySelectorAll('[role="button"]')].find((x) => (x.innerText ?? "").trim() === "Tạo kèo");
        const r = e?.getBoundingClientRect();
        return r ? { top: Math.round(r.top), bottom: Math.round(r.bottom), trongMan: r.bottom <= innerHeight && r.top >= 0 } : null;
      });
      await chup(t.page, { out: mt.out, id: "EV-F03-TAO-C8", suKien: t.suKien });
      ket({ tc: "TC-F03-TAO-C8", screen: "F03.S02", state: "cửa sổ thấp 390×460", action: "mở form", cauHinh: "C8", expected: "«Tạo kèo» nằm trong màn mà không phải cuộn", status: n?.trongMan ? "PASS" : "FAIL", evidence: ["EV-F03-TAO-C8"], ghiChu: n ? `nút ${n.top}–${n.bottom}` : "không thấy nút" });
      await t.context.close();
    }
  }

  // ------------------------------------------- add a place to an outing (S05)
  if (chay("chon")) {
    {
      const t = await mo("C1", "/outings/chon?place=p-lung-chung-cafe");
      const cdp = await cdpCua(t.page);
      await chup(t.page, { out: mt.out, id: "EV-F03-CHON-C1", suKien: t.suKien });
      const nut = await t.page.evaluate((ten) => {
        // The smallest box holding this outing's title and exactly one «Thêm vào».
        const nutThem = (e) => [...e.querySelectorAll('[role="button"]')].filter((x) => (x.innerText ?? "").trim() === "Thêm vào");
        const hang = [...document.querySelectorAll("div")]
          .filter((e) => (e.innerText ?? "").includes(ten) && nutThem(e).length === 1)
          .sort((a, b) => a.getBoundingClientRect().width * a.getBoundingClientRect().height - b.getBoundingClientRect().width * b.getBoundingClientRect().height)[0];
        const b = hang ? nutThem(hang)[0] : null;
        if (!b) return null;
        b.scrollIntoView({ block: "center", behavior: "instant" });
        const r = b.getBoundingClientRect();
        return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
      }, KEO_RONG);
      if (nut) await cham(cdp, nut);
      await t.page.waitForTimeout(1500);
      await choOn(t.page, { mang: t.mang });
      const d = await duongDan(t.page);
      const chang = await thuTuChang(t.page);
      await chup(t.page, { out: mt.out, id: "EV-F03-CHON-xong-C1", suKien: t.suKien });
      ket({ tc: "TC-F03-CHON-THEM", screen: "F03.S05", state: "đang thêm Lưng Chừng Cafe", action: "chạm «Thêm vào» ở kèo rỗng", cauHinh: "C1", expected: "về kèo đó, có một chặng mang tên quán", status: d.startsWith(`/outings/${RONG}`) && chang.some((c) => /Lưng Chừng Cafe/.test(c)) ? "PASS" : "FAIL", evidence: ["EV-F03-CHON-C1", "EV-F03-CHON-xong-C1"], ghiChu: `${d.replace(/[0-9a-f-]{36}/, "<id>")}; chặng: ${chang.join(" | ") || "không"}` });
      await t.context.close();
      await datLaiChang(mt, RONG, []);
    }
    {
      const t = await mo("C1", "/outings/chon");
      await t.page.waitForTimeout(6000);
      const tt = await t.page.evaluate(() => ({ khung: document.querySelectorAll('[role="progressbar"]').length, chu: (document.querySelector('[data-testid="pick-outing-screen"]')?.innerText ?? "").replace(/\s+/g, " ").trim().slice(0, 120) }));
      await chup(t.page, { out: mt.out, id: "EV-F03-CHON-thieu-C1", suKien: t.suKien });
      ket({ tc: "TC-F03-CHON-THIEU", screen: "F03.S05", state: "đường dẫn thiếu ?place", action: "mở /outings/chon", cauHinh: "C1", expected: "một câu nói thiếu địa điểm và lối ra; không kẹt ở trạng thái tải", status: tt.khung === 0 ? "PASS" : "FAIL", evidence: ["EV-F03-CHON-thieu-C1"], ghiChu: `sau 6 s: ${tt.khung} khung skeleton; chữ trên màn: «${tt.chu}»` });
      await t.context.close();
    }
  }

  // -------------------------------------------------------- demo routes
  if (chay("demo")) {
    for (const [duong, mau, tc] of [[`/trips/${GOC}/timeline`, null, "TC-F03-DEMO-TIMELINE"], [`/trips/${GOC}/itinerary`, `/outings/${GOC}`, "TC-F03-DEMO-ITINERARY"], ["/check-ins/new", "/plan", "TC-F03-DEMO-CHECKIN"]]) {
      const t = await mo("C1", duong);
      const d = await duongDan(t.page);
      const tt = await t.page.evaluate(() => ({ demo: /Khởi hành từ TP\.HCM|Check-in homestay|Bánh căn Lệ/.test(document.body.innerText ?? ""), nhan: /Dữ liệu demo|dữ liệu mẫu|demo/i.test(document.body.innerText ?? ""), back: !!document.querySelector('[aria-label="Quay lại"]') }));
      const id = `EV-${tc.slice(3)}-C1`;
      await chup(t.page, { out: mt.out, id, suKien: t.suKien });
      const dat = mau ? d === mau : !tt.demo;
      ket({ tc, screen: tc === "TC-F03-DEMO-TIMELINE" ? "F03.S08" : tc === "TC-F03-DEMO-ITINERARY" ? "F03.S07" : "F03.S06", state: "đã đăng nhập (phiên thật)", action: `mở ${duong.replace(/[0-9a-f-]{36}/, "<id>")}`, cauHinh: "C1", expected: mau ? `chuyển về ${mau.replace(/[0-9a-f-]{36}/, "<id>")} (B5)` : "không hiện dữ liệu demo dưới id kèo thật; chuyển về màn sống như hai route anh em (B5)", status: dat ? "PASS" : "FAIL", evidence: [id], ghiChu: `đường cuối ${d.replace(/[0-9a-f-]{36}/, "<id>")}; nội dung demo: ${tt.demo}; nhãn demo: ${tt.nhan}; nút Quay lại: ${tt.back}` });
      await t.context.close();
    }
  }

  // ------------------------------------------------- failure, offline, back
  if (chay("loi")) {
    {
      const t = await mo("C1", "/plan");
      await loiMayChu(t.page, /\/contexts\/[^/]+\/outings(\?|$)/, 503);
      const cdp = await cdpCua(t.page);
      await t.page.evaluate((id) => {
        history.pushState({}, "", `/outings/${id}`);
        dispatchEvent(new PopStateEvent("popstate"));
      }, GOC);
      await choOn(t.page, { mang: t.mang });
      await t.page.waitForTimeout(900);
      const chu = await t.page.evaluate(() => (document.body.innerText ?? "").split("\n").filter((l) => /Chưa mở|thử lại|Thử lại|Về Lên plan|sự cố|mạng/i.test(l)).slice(0, 6));
      const m = await chup(t.page, { out: mt.out, id: "EV-F03-LOI-503-C1", suKien: t.suKien });
      await goHet(t.page);
      const thuLai = await timNut(t.page, "Thử lại");
      if (thuLai) await cham(cdp, thuLai);
      await choOn(t.page, { mang: t.mang });
      await t.page.waitForTimeout(700);
      const sau = await thuTuChang(t.page);
      ket({ tc: "TC-F03-LOI-503", screen: "F03.S03", layer: "L35", state: "danh sách kèo trả 503", action: "mở kèo, rồi «Thử lại» khi máy chủ lành", cauHinh: "C1", expected: "câu tiếng Việt đúng nguyên nhân, có «Thử lại»; thử lại thì mở được", status: thuLai && m.tomTat.maLoi === 0 && sau.length === 3 ? "PASS" : "FAIL", evidence: ["EV-F03-LOI-503-C1"], ghiChu: `câu: ${chu.join(" | ")}; sau thử lại: ${sau.join(" → ") || "chưa mở"}` });
      await t.context.close();
    }
    {
      const t = await mo("C1", `/outings/${GOC}`);
      const cdp = await cdpCua(t.page);
      await ngatMang(t.context, true);
      const n = await t.page.evaluate(() => {
        const e = [...document.querySelectorAll('[role="button"]')].find((x) => (x.innerText ?? "").trim() === "Tôi đã tới");
        if (!e) return null;
        e.scrollIntoView({ block: "center", behavior: "instant" });
        const r = e.getBoundingClientRect();
        return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
      });
      if (n) await cham(cdp, n);
      await t.page.waitForTimeout(2500);
      const chu = await t.page.evaluate(() => [...document.querySelectorAll('[aria-live]')].map((e) => (e.innerText ?? "").trim()).filter(Boolean).slice(0, 3));
      const m = await chup(t.page, { out: mt.out, id: "EV-F03-OFFLINE-C1", suKien: t.suKien });
      await ngatMang(t.context, false);
      ket({ tc: "TC-F03-OFFLINE", screen: "F03.S03", layer: "L35", state: "kèo đã mở, mất mạng", action: "chạm «Tôi đã tới»", cauHinh: "C1", expected: "một câu tiếng Việt báo chưa ghi được; lịch trình còn nguyên", status: chu.length > 0 && m.tomTat.maLoi === 0 && (await thuTuChang(t.page)).length === 3 ? "PASS" : "FAIL", evidence: ["EV-F03-OFFLINE-C1"], ghiChu: `câu: ${chu.join(" | ") || "không có"}; mã lỗi trên màn ${m.tomTat.maLoi}` });
      await t.context.close();
    }
    {
      const t = await mo("C1", `/outings/${GOC}`);
      const cdp = await cdpCua(t.page);
      await cham(cdp, await tamCua(t.page, '[aria-label="Quay lại"]', { cuon: false }));
      await t.page.waitForTimeout(900);
      const d = await duongDan(t.page);
      ket({ tc: "TC-F03-BACK-LANH", screen: "F03.S03", state: "mở kèo thẳng bằng link", action: "chạm «Quay lại»", cauHinh: "C1", expected: "về Lên plan", status: d !== `/outings/${GOC}` ? "PASS" : "FAIL", issue: d === `/outings/${GOC}` ? "UI-018" : null, evidence: [], ghiChu: `sau khi chạm: ${d.replace(/[0-9a-f-]{36}/, "<id>")}` });
      await t.context.close();
    }
  }

  // ---------------------------------------------- layers L09–L15, map controls
  if (chay("lop")) {
    // L09: the attach-place sheet, closed three ways.
    await datLaiChang(mt, DAI, CHANG_DAI);
    for (const cach of ["x", "esc", "de-sau"]) {
      const t = await moQuaPlan("C1", `/outings/${DAI}`);
      const cdp = await cdpCua(t.page);
      // Scroll the stop into view first, THEN take the grid: the grid is for
      // what the sheet leaves behind, not for the scroll that led to it.
      const oChang = await tamCua(t.page, '[aria-label="Chặng Nghỉ trưa"]');
      await t.page.waitForTimeout(300);
      const truoc = await luoiChamTrang(t.page);
      await cham(cdp, oChang);
      const moRa = await choDialog(t.page, { co: true });
      await t.page.waitForTimeout(600);
      if (cach === "de-sau") await cham(cdp, await timNut(t.page, "Để sau", { trong: '[role="dialog"]' }));
      else await dong(t.page, cdp, cach);
      await t.page.waitForTimeout(1200);
      const con = await demDialog(t.page);
      const khac = soLuoi(truoc, await luoiChamTrang(t.page));
      const focusSau = await focusHienTai(t.page);
      ket({ tc: `TC-L09-DONG-${cach}`, screen: "F03.S03", layer: "L09", state: "sheet «Gắn địa điểm» mở", action: `đóng bằng ${cach === "de-sau" ? "«Để sau»" : cach}`, cauHinh: "C1", expected: "đóng hẳn, không sót lớp chặn; focus về chặng vừa chạm", status: moRa.ok && con === 0 && khac.length <= 1 ? "PASS" : "FAIL", evidence: [], ghiChu: `mở ${moRa.ok}; còn ${con} dialog; lưới đổi ${khac.length}; focus sau: ${focusSau?.ten ?? "body"}` });
      await t.context.close();
    }

    // Map mode on the one-day outing: L10, L11, L12, L13, and the controls.
    {
      const t = await moQuaPlan("C1", `/outings/${DAI}`);
      const cdp = await cdpCua(t.page);
      await moTheoChe(t.page, cdp, "Bản đồ");
      await choOn(t.page, { mang: t.mang });
      await t.page.waitForTimeout(800);

      // Controls' names, read by a screen reader.
      const nutBanDo = await t.page.evaluate(() => [...document.querySelectorAll(".maplibregl-ctrl button, .maplibregl-popup-close-button")].map((e) => e.getAttribute("aria-label") || e.getAttribute("title") || "(không tên)"));
      ket({ tc: "TC-F03-BAN-DO-NHAN", screen: "F03.S04", state: "Bản đồ", action: "đọc tên các nút điều khiển bản đồ", cauHinh: "C1", expected: "tên tiếng Việt như phần còn lại của app", status: nutBanDo.length && nutBanDo.every((x) => /[ăâđêôơư]|Bản đồ|Phóng|Thu nhỏ|Hướng/i.test(x)) ? "PASS" : "FAIL", evidence: [], ghiChu: `nút: ${nutBanDo.join(" | ") || "không có"}` });

      // L13: the day page folds and unfolds.
      const gap = async () => {
        const n = await t.page.evaluate(() => {
          const e = [...document.querySelectorAll('[role="button"]')].find((x) => /Các chặng trong ngày/.test(x.innerText ?? ""));
          if (!e) return null;
          const r = e.getBoundingClientRect();
          return { x: r.left + r.width / 2, y: r.top + r.height / 2, expanded: e.getAttribute("aria-expanded"), chu: (e.innerText ?? "").replace(/\s+/g, " ").trim() };
        });
        if (n) await cham(cdp, n);
        await t.page.waitForTimeout(700);
        return n;
      };
      const g1 = await gap();
      await chup(t.page, { out: mt.out, id: "EV-F03-TRANG-NGAY-gap-C1", suKien: t.suKien });
      const g2 = await gap();
      ket({ tc: "TC-L13-GAP", screen: "F03.S04", layer: "L13", state: "trang ngày đang mở", action: "chạm «Thu gọn» rồi «Mở trang»", cauHinh: "C1", expected: "gập và mở lại; trạng thái mở/gập báo cho công nghệ hỗ trợ (aria-expanded)", status: g1 && g2 && /Thu gọn/.test(g1.chu) && /Mở trang/.test(g2.chu) && g1.expanded !== null ? "PASS" : "FAIL", issue: g1 && g1.expanded === null ? "UI-003" : null, evidence: ["EV-F03-TRANG-NGAY-gap-C1"], ghiChu: `trước khi gập: «${g1?.chu}», aria-expanded ${g1?.expanded}; trước khi mở lại: «${g2?.chu}», aria-expanded ${g2?.expanded}` });

      // L12: clusters. Zoom out over the map until pins group.
      const khung = await t.page.evaluate(() => { const r = document.querySelector(".maplibregl-map")?.getBoundingClientRect(); return r ? { x: r.left + r.width / 2, y: r.top + r.height / 2, top: r.top, bottom: r.bottom } : null; });
      let cum = null;
      for (let i = 0; i < 6 && khung && !cum; i++) {
        await t.page.mouse.move(khung.x, khung.y);
        await t.page.mouse.wheel(0, 400);
        await t.page.waitForTimeout(700);
        cum = await t.page.evaluate(() => {
          const e = [...document.querySelectorAll('[aria-label*="điểm gần nhau"]')].find((x) => x.getBoundingClientRect().width > 0);
          if (!e) return null;
          const r = e.getBoundingClientRect();
          return { x: r.left + r.width / 2, y: r.top + r.height / 2, ten: e.getAttribute("aria-label") };
        });
      }
      if (cum) {
        await t.page.mouse.click(cum.x, cum.y);
        await t.page.waitForTimeout(700);
        const pop = await t.page.evaluate(() => {
          const p = document.querySelector(".maplibregl-popup");
          const m = document.querySelector(".maplibregl-map")?.getBoundingClientRect();
          if (!p || !m) return null;
          const r = p.getBoundingClientRect();
          const nut = [...p.querySelectorAll('[role="group"] button')].map((b) => Math.round(b.getBoundingClientRect().height));
          return { trongBanDo: r.top >= m.top - 1 && r.bottom <= m.bottom + 1 && r.left >= m.left - 1 && r.right <= m.right + 1, nut, dong: p.querySelector(".maplibregl-popup-close-button")?.getAttribute("aria-label"), focusTrong: p.contains(document.activeElement) };
        });
        await chup(t.page, { out: mt.out, id: "EV-F03-CUM-C1", suKien: t.suKien });
        await t.page.keyboard.press("Escape");
        await t.page.waitForTimeout(500);
        const conSauEsc = await t.page.evaluate(() => !!document.querySelector(".maplibregl-popup"));
        if (conSauEsc) await t.page.click(".maplibregl-popup-close-button").catch(() => undefined);
        await t.page.waitForTimeout(400);
        const conSauX = await t.page.evaluate(() => !!document.querySelector(".maplibregl-popup"));
        ket({ tc: "TC-L12-CUM", screen: "F03.S04", layer: "L12", state: `bản đồ thu nhỏ, cụm «${cum.ten}»`, action: "chạm cụm, rồi Esc, rồi nút đóng", cauHinh: "C1", expected: "danh sách chọn nằm trọn trong bản đồ, mục ≥ 48dp, focus vào danh sách; Esc hoặc nút đóng tắt được", status: pop && pop.trongBanDo && pop.nut.every((h) => h >= 48) && !conSauX ? "PASS" : "FAIL", evidence: ["EV-F03-CUM-C1"], ghiChu: pop ? `trong bản đồ ${pop.trongBanDo}; mục cao ${pop.nut.join("/")}px; focus trong popup ${pop.focusTrong}; nút đóng tên «${pop.dong}»; Esc đóng: ${!conSauEsc}; nút đóng đóng: ${!conSauX}` : "không có popup" });
      } else {
        ket({ tc: "TC-L12-CUM", screen: "F03.S04", layer: "L12", state: "bản đồ kèo 1 ngày", action: "thu nhỏ bản đồ 6 lần bằng con lăn", cauHinh: "C1", expected: "mốc gần nhau gom thành cụm", status: "NOT_TESTED", evidence: [], ghiChu: "không tạo được cụm (nền bản đồ bị chặn; mốc vẫn xa nhau ở mọi mức thu phóng đã thử)" });
      }

      // L10: «Sửa trang ngày» sheet; is the header under its backdrop?
      await cham(cdp, await timNut(t.page, "Sửa trang ngày"));
      const moSua = await choDialog(t.page, { co: true });
      await t.page.waitForTimeout(700);
      const phu = await t.page.evaluate(() => {
        const tab = document.querySelector('[role="tab"][aria-label="Lịch trình"]')?.getBoundingClientRect();
        const back = document.querySelector('[aria-label="Quay lại"]')?.getBoundingClientRect();
        const tren = (r) => { if (!r) return null; const e = document.elementFromPoint(r.left + r.width / 2, r.top + r.height / 2); return e ? { tab: !!e.closest('[role="tab"]'), back: !!e.closest('[aria-label="Quay lại"]'), dialog: !!e.closest('[role="dialog"]') } : null; };
        const sw = [...document.querySelectorAll('[role="dialog"] [role="switch"], [role="dialog"] input[type="checkbox"]')].map((e) => ({ ten: e.getAttribute("aria-label"), checked: e.getAttribute("aria-checked") ?? String(e.checked) }));
        return { tab: tren(tab), back: tren(back), sw };
      });
      await chup(t.page, { out: mt.out, id: "EV-F03-SUA-NGAY-C1", suKien: t.suKien });
      // Tap the «Lịch trình» tab through the backdrop, if it is reachable.
      const tabO = await t.page.evaluate(() => { const r = document.querySelector('[role="tab"][aria-label="Lịch trình"]')?.getBoundingClientRect(); return r ? { x: r.left + r.width / 2, y: r.top + r.height / 2 } : null; });
      if (tabO) await cham(cdp, tabO);
      await t.page.waitForTimeout(900);
      const sauCham = await t.page.evaluate(() => ({ dialog: document.querySelectorAll('[role="dialog"]').length, lichTrinh: document.querySelector('[role="tab"][aria-label="Lịch trình"]')?.getAttribute("aria-selected") }));
      ket({ tc: "TC-L10-NEN", screen: "F03.S04", layer: "L10", state: "sheet «Sửa trang ngày» mở", action: "chạm công tắc «Lịch trình» phía trên sheet", cauHinh: "C1", expected: "nền của sheet phủ cả đầu màn: chạm ra ngoài chỉ đóng sheet, không đổi chế độ dưới nó", status: moSua.ok && phu.tab && !phu.tab.tab && sauCham.lichTrinh !== "true" ? "PASS" : "FAIL", evidence: ["EV-F03-SUA-NGAY-C1"], ghiChu: `điểm giữa công tắc thuộc: ${JSON.stringify(phu.tab)}; nút Quay lại: ${JSON.stringify(phu.back)}; sau khi chạm: ${sauCham.dialog} dialog, «Lịch trình» aria-selected ${sauCham.lichTrinh}; công tắc trong sheet: ${phu.sw.map((x) => `${x.ten}=${x.checked}`).join(", ")}` });
      await t.context.close();
    }

    // L11: a meeting point from the map (right click; long press on touch).
    {
      const t = await moQuaPlan("C1", `/outings/${DAI}`);
      const cdp = await cdpCua(t.page);
      await moTheoChe(t.page, cdp, "Bản đồ");
      await choOn(t.page, { mang: t.mang });
      const m = await t.page.evaluate(() => { const r = document.querySelector(".maplibregl-canvas")?.getBoundingClientRect(); return r ? { x: r.left + r.width * 0.3, y: r.top + r.height * 0.4 } : null; });
      await cdp.send("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [{ x: m.x, y: m.y }] });
      await t.page.waitForTimeout(900);
      await cdp.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
      await t.page.waitForTimeout(800);
      const quaCham = await demDialog(t.page);
      if (quaCham) await t.page.keyboard.press("Escape");
      await t.page.waitForTimeout(600);
      await t.page.mouse.click(m.x, m.y, { button: "right" });
      const moPin = await choDialog(t.page, { co: true, toiDa: 3000 });
      await t.page.waitForTimeout(600);
      const pin = await t.page.evaluate(() => {
        const d = document.querySelector('[role="dialog"]');
        const nut = [...(d?.querySelectorAll('[role="button"]') ?? [])].find((x) => (x.innerText ?? "").trim() === "Thêm điểm hẹn");
        return { ten: d?.getAttribute("aria-label"), nutDisabled: nut?.getAttribute("aria-disabled") ?? null, vien: nut ? getComputedStyle(nut).borderStyle : null };
      });
      await chup(t.page, { out: mt.out, id: "EV-F03-DIEM-HEN-C1", suKien: t.suKien });
      ket({ tc: "TC-L11-MO", screen: "F03.S04", layer: "L11", state: "Bản đồ kèo 1 ngày", action: "chuột phải trên bản đồ", cauHinh: "C1", expected: "sheet «Điểm hẹn» mở; «Thêm điểm hẹn» chờ tên", status: moPin.ok ? "PASS" : "FAIL", evidence: ["EV-F03-DIEM-HEN-C1"], ghiChu: `${moPin.ok ? `mở «${pin.ten}»` : "không mở"}; «Thêm điểm hẹn» khi chưa có tên: aria-disabled ${pin.nutDisabled}, viền ${pin.vien}` });
      // A touch long press: MapLibre listens for «contextmenu», which a real
      // mobile Chrome synthesises from a long press; CDP touch emulation does not.
      ket({ tc: "TC-L11-GIU", screen: "F03.S04", layer: "L11", state: "Bản đồ kèo 1 ngày", action: "giữ ngón tay 900 ms trên bản đồ", cauHinh: "C1", expected: "sheet «Điểm hẹn» mở (lối duy nhất trên điện thoại)", status: quaCham > 0 ? "PASS" : "BLOCKED", evidence: [], ghiChu: quaCham > 0 ? "mở bằng giữ ngón tay" : "giả lập cảm ứng CDP không sinh sự kiện contextmenu từ cú giữ; cần Chrome Android thật. Native dùng onLongPress (BanDoMapLibre.native.tsx:114)" });
      if (moPin.ok) await dong(t.page, cdp, "x");
      await t.page.waitForTimeout(800);
      ket({ tc: "TC-L11-DONG", screen: "F03.S04", layer: "L11", state: "sheet «Điểm hẹn» mở", action: "đóng bằng X", cauHinh: "C1", expected: "đóng hẳn, không thêm điểm hẹn", status: moPin.ok && (await demDialog(t.page)) === 0 ? "PASS" : moPin.ok ? "FAIL" : "NOT_TESTED", evidence: [], ghiChu: moPin.ok ? "đã đóng bằng X" : "sheet không mở" });
      await t.context.close();
    }

    // L14: the month leaf, choosing a day.
    {
      const t = await mo("C1", "/outings/new");
      const cdp = await cdpCua(t.page);
      await cham(cdp, await tamCua(t.page, '[data-testid="ngay-di-la"]'));
      await t.page.waitForTimeout(600);
      await cham(cdp, await tamCua(t.page, '[aria-label="Tháng sau"]'));
      await t.page.waitForTimeout(500);
      const ngay = await t.page.evaluate(() => {
        const e = [...document.querySelectorAll('[role="button"]')].find((x) => /, 10 tháng \d+ năm/.test(x.getAttribute("aria-label") ?? ""));
        if (!e) return null;
        e.scrollIntoView({ block: "center", behavior: "instant" });
        const r = e.getBoundingClientRect();
        return { x: r.left + r.width / 2, y: r.top + r.height / 2, ten: e.getAttribute("aria-label") };
      });
      if (ngay) await cham(cdp, ngay);
      await t.page.waitForTimeout(700);
      // Day cells only: the two date leaves carry «… tháng … năm …» too.
      const sau = await t.page.evaluate(() => ({ la: (document.querySelector('[data-testid="ngay-di-la"]')?.innerText ?? "").replace(/\s+/g, " ").trim(), luoiCon: [...document.querySelectorAll('[role="button"]')].filter((x) => /tháng \d+ năm \d+/.test(x.getAttribute("aria-label") ?? "") && !(x.getAttribute("data-testid") ?? "").endsWith("-la") && x.getBoundingClientRect().width > 0).length, ve: (document.querySelector('[data-testid="ngay-ve-la"]')?.innerText ?? "").replace(/\s+/g, " ").trim() }));
      await chup(t.page, { out: mt.out, id: "EV-F03-LICH-chon-C1", suKien: t.suKien });
      ket({ tc: "TC-L14-CHON", screen: "F03.S02", layer: "L14", state: "lịch tháng mở", action: `«Tháng sau», chạm «${ngay?.ten ?? "?"}»`, cauHinh: "C1", expected: "lá «Ngày đi» hiện ngày 10; lịch tháng khép lại", status: ngay && /10/.test(sau.la) && sau.luoiCon === 0 ? "PASS" : "FAIL", evidence: ["EV-F03-LICH-chon-C1"], ghiChu: `lá «Ngày đi»: «${sau.la}»; ô ngày còn trên màn: ${sau.luoiCon}; lá «Ngày về» vẫn: «${sau.ve}»` });
      await t.context.close();
    }

    // L15: the time dial, dragged and typed.
    {
      const t = await moQuaPlan("C1", `/outings/${GOC}`);
      const cdp = await cdpCua(t.page);
      await cham(cdp, await timNut(t.page, "Thêm chặng"));
      await choDialog(t.page, { co: true });
      await t.page.waitForTimeout(700);
      const docGio = () => t.page.evaluate(() => [...document.querySelectorAll('[role="dialog"] div[dir]')].map((e) => (e.textContent ?? "").trim()).find((x) => /^\d{1,2}:\d{2}$/.test(x)) ?? null);
      const dial = await t.page.evaluate(() => { const e = document.querySelector('[role="dialog"] [aria-label="Giờ chặng"]'); const r = e?.getBoundingClientRect(); return r ? { cx: r.left + r.width / 2, cy: r.top + r.height / 2, r: r.width / 2 - 14, role: e.getAttribute("role"), now: e.getAttribute("aria-valuenow"), text: e.getAttribute("aria-valuetext") } : null; });
      const g0 = await docGio();
      if (dial) await keo(cdp, { x: dial.cx, y: dial.cy - dial.r }, { x: dial.cx + dial.r, y: dial.cy }, { ms: 700, buoc: 20, giuCuoiMs: 120 });
      await t.page.waitForTimeout(600);
      const g1 = await docGio();
      await cham(cdp, await tamCua(t.page, '[role="dialog"] [aria-label="Gõ giờ: Giờ chặng"]', { cuon: false }));
      await t.page.waitForTimeout(500);
      const o = t.page.locator('[role="dialog"] input[aria-label="Ô giờ chặng"]');
      const coO = (await o.count()) > 0;
      if (coO) {
        await o.fill("07:30");
        await t.page.keyboard.press("Enter");
      }
      await t.page.waitForTimeout(600);
      const g2 = await docGio();
      await chup(t.page, { out: mt.out, id: "EV-F03-BAN-XOAY-C1", suKien: t.suKien });
      ket({ tc: "TC-L15-KEO", screen: "F03.S03", layer: "L15", state: "sheet «Chặng mới», mặt quay giờ", action: "kéo kim từ 12 giờ sang 3 giờ", cauHinh: "C1", expected: "giờ ở giữa đổi theo tay (bậc 15 phút)", status: g0 && g1 && g0 !== g1 ? "PASS" : "FAIL", evidence: ["EV-F03-BAN-XOAY-C1"], ghiChu: `trước ${g0}, sau khi kéo ${g1}; role ${dial?.role}, aria-valuenow ${dial?.now}, aria-valuetext ${dial?.text}` });
      ket({ tc: "TC-L15-GO", screen: "F03.S03", layer: "L15", state: "sheet «Chặng mới», mặt quay giờ", action: "chạm giữa mặt, gõ 07:30, Enter", cauHinh: "C1", expected: "ô gõ giờ hiện; mặt quay nhận 07:30", status: coO && g2 === "07:30" ? "PASS" : "FAIL", evidence: ["EV-F03-BAN-XOAY-C1"], ghiChu: `ô gõ: ${coO}; giờ sau khi gõ: ${g2}` });
      await t.context.close();
    }
  }

  // ---------------------------------------- 12 stops, the unbroken label
  if (chay("dai")) {
    for (const cfg of ["C2", "C1"]) {
      const t = await mo(cfg, `/outings/${DAI}`);
      const o = await t.page.evaluate(() => {
        const e = document.querySelector('[aria-label^="Chặng ChụpẢnh"]');
        if (!e) return null;
        e.scrollIntoView({ block: "center", behavior: "instant" });
        const r = e.getBoundingClientRect();
        const chu = [...e.querySelectorAll("div[dir]")].find((x) => (x.textContent ?? "").startsWith("ChụpẢnh"));
        const c = chu?.getBoundingClientRect();
        return { phai: Math.round(r.right), chuPhai: c ? Math.round(c.right) : null, cot: c ? Math.round(c.width) : null, dong: c ? Math.round(c.height / parseFloat(getComputedStyle(chu).lineHeight || "20")) : null, tranTrang: document.documentElement.scrollWidth > innerWidth };
      });
      await t.page.waitForTimeout(400);
      const id = `EV-F03-DAI-lien-${cfg}`;
      const m = await chup(t.page, { out: mt.out, id, suKien: t.suKien });
      // Covered-text signals here are rows scrolled under the fixed header
      // (the Timeline | Map switch): normal scrolling, not an overlap.
      ket({ tc: "TC-F03.S03-TEN-LIEN", screen: "F03.S03", state: "kèo 12 chặng, một nhãn 57 ký tự không có dấu cách", action: "cuộn tới chặng đó", cauHinh: cfg, expected: "nhãn được bẻ dòng, không tràn ngang, không đè nút bên phải", status: o && !o.tranTrang && m.tomTat.tranNgang === 0 && o.chuPhai <= o.phai ? "PASS" : "FAIL", evidence: [id], ghiChu: o ? `chữ tới x=${o.chuPhai}, hàng tới x=${o.phai}; tràn trang ${o.tranTrang}; tín hiệu tràn ${m.tomTat.tranNgang}` : "không thấy chặng" });
      ket({ tc: "TC-F03-COT-CHANG", screen: "F03.S03", state: "kèo 12 chặng", action: "đọc tên chặng dài", cauHinh: cfg, expected: "cột tên chặng đủ rộng để một nhãn dài đọc thành vài dòng, không gãy từng mẩu", status: o && o.cot >= 120 ? "PASS" : "FAIL", evidence: [id], ghiChu: o ? `cột tên rộng ${o.cot}px trên màn ${cfg === "C2" ? 320 : 390}px; nhãn 57 ký tự thành khoảng ${o.dong} dòng` : "không thấy chặng" });
      await t.context.close();
    }
  }

  // ------------------------------------------- the outing header on tablets
  if (chay("tablet")) {
    for (const cfg of ["C1", "C6", "C7"]) {
      const t = await mo(cfg, `/outings/${GOC}`);
      const hinh = await t.page.evaluate(() => {
        const r = (sel) => document.querySelector(sel)?.getBoundingClientRect();
        const back = r('[aria-label="Quay lại"]');
        const tab = r('[role="tab"][aria-label="Lịch trình"]');
        const tab2 = r('[role="tab"][aria-label="Bản đồ"]');
        const h1 = [...document.querySelectorAll("div[dir]")].find((e) => (e.textContent ?? "").trim() === "Đà Lạt cuối tuần" && parseFloat(getComputedStyle(e).fontSize) >= 24)?.getBoundingClientRect();
        return { back: back ? Math.round(back.left) : null, congTac: tab && tab2 ? { trai: Math.round(tab.left), phai: Math.round(tab2.right), rong: Math.round(tab2.right - tab.left) } : null, noiDungTrai: h1 ? Math.round(h1.left) : null, w: innerWidth };
      });
      const id = `EV-F03-TABLET-${cfg}`;
      await chup(t.page, { out: mt.out, id, suKien: t.suKien });
      const lech = hinh.back !== null && hinh.noiDungTrai !== null ? hinh.back - hinh.noiDungTrai : null;
      ket({ tc: "TC-F03-DAU-MAN", screen: "F03.S03", state: "kèo 3 chặng", action: "mở kèo", cauHinh: cfg, expected: "nút Quay lại và công tắc thẳng hàng với cột nội dung; công tắc rộng theo cột", status: lech !== null && lech <= 24 && hinh.congTac && hinh.congTac.rong >= 0.6 * (hinh.w - 2 * hinh.noiDungTrai) ? "PASS" : "FAIL", evidence: [id], ghiChu: `rộng ${hinh.w}: nút Quay lại x=${hinh.back}, mép trái nội dung x=${hinh.noiDungTrai} (lệch ${lech}px); công tắc ${hinh.congTac?.trai}–${hinh.congTac?.phai} (rộng ${hinh.congTac?.rong}px)` });
      await t.context.close();
    }
  }

  // ------------------------------------------------ plan list, cut text
  if (chay("meta")) {
    for (const cfg of ["C1", "C2", "C4", "C5"]) {
      const t = await mo(cfg, "/plan");
      const dong = await t.page.evaluate(() =>
        [...document.querySelectorAll("div[dir]")]
          .filter((e) => /\d+ người/.test(e.textContent ?? "") && e.children.length === 0 && e.getBoundingClientRect().width > 0)
          .map((e) => {
            const r = e.getBoundingClientRect();
            return { chu: e.textContent, thieu: Math.round(e.scrollWidth - e.clientWidth), x: r.left, y: r.top, w: r.width, h: r.height };
          }),
      );
      const cat = dong.filter((d) => d.thieu > 1);
      const id = `EV-F03-META-${cfg}`;
      await chup(t.page, { out: mt.out, id, suKien: t.suKien, chuThich: cat.map((d) => ({ rect: { x: d.x, y: d.y, w: d.w, h: d.h }, nhan: `thiếu ${d.thieu}px` })) });
      log({ cfg, cat: cat.map((d) => `${d.chu} (thiếu ${d.thieu})`) });
      ket({ tc: "TC-F03-META", screen: "F03.S01", state: "3 kèo (1 sắp tới, 2 «Sau đó»)", action: "đọc dòng ngày · người · chặng · còn N ngày", cauHinh: cfg, expected: "dòng thông tin của mỗi kèo đọc được trọn, hoặc phần bị cắt là phần phụ", status: cat.length === 0 ? "PASS" : "FAIL", evidence: [id], ghiChu: `${cat.length}/${dong.length} dòng bị cắt: ${cat.map((d) => `«${d.chu}» thiếu ${d.thieu}px`).join("; ")}` });
      await t.context.close();
    }
  }
} finally {
  await mt.dong();
}
