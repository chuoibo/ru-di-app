/* F04, Tiền: the bill in five steps, the settlement, a collection round, the
 * person's finance page.
 *
 *   node kich-ban/f04-tien.mjs [--chi buoc,chan,lui,ban,ban-20,anh,ghi,qt,dot,chia-se,tien-ve,dot-rong,loi,lanh,nep,c8,tablet,demo,lat]
 *
 * Two groups, two rules:
 *  - Team Đà Lạt (dalat-0, 8 people) is read, never written to the ledger. The
 *    bill flow runs there up to the result step, which creates only a draft
 *    bill (no expense). «Tạo đợt thu từ sổ» is pressed there once because the
 *    server refuses it (every expense is already in a round); the batch count
 *    is checked before and after.
 *  - The chat-test group (chat-0, 20 people) takes the writes: one expense
 *    (8-digit sums), one round, publishing it, one arrival. The ledger is
 *    append-only, so these rows stay; each write part first looks at what is
 *    there and does not write twice on a second run.
 */
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { spawnSync } from "node:child_process";

import { pngThuBytes } from "../../../../apps/mobile/tools/png-thu.mjs";
import { cauHinh } from "../thu-vien/cau-hinh.mjs";
import { chup, ghepAnh } from "../thu-vien/chup.mjs";
import { ghepKhung, quayKhung } from "../thu-vien/chuyen-dong.mjs";
import { cdpCua, cham, keo } from "../thu-vien/cu-chi.mjs";
import { choOn, duongDan } from "../thu-vien/dieu-huong.mjs";
import { goHet, loiMayChu, ngatMang } from "../thu-vien/mang.mjs";
import { khoiDong, trangMoi } from "../thu-vien/moi-truong.mjs";
import { goiApi, layPhien, personaTheoTen } from "../thu-vien/phien.mjs";
import { soGhi } from "../thu-vien/ghi.mjs";
import { moNhom } from "../seed-bien-the.mjs";

const chi = (() => {
  const i = process.argv.indexOf("--chi");
  return i === -1 ? null : new Set(process.argv[i + 1].split(","));
})();
const chay = (ten) => !chi || chi.has(ten);
const mt = await khoiDong();
const so = soGhi(mt.out);
const ket = (rec) => {
  so.ghi({ feature: "F04", nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, ...rec });
  console.log(`${rec.status.padEnd(10)} ${rec.tc} ${rec.cauHinh ?? ""} ${rec.ghiChu ?? ""}`);
};
const log = (o) => console.log(JSON.stringify(o));

const P8 = personaTheoTen("dalat-0", mt.chatSessions);
const P20 = personaTheoTen("chat-0", mt.chatSessions);
const { nhomId: G8 } = await moNhom(mt);
const G20 = JSON.parse(readFileSync(mt.chatSessions, "utf8")).groupId;
const phien20 = await layPhien(mt.api, P20, join(mt.out, "phien"));
const phien8 = await layPhien(mt.api, P8, join(mt.out, "phien"));
const dotCua = async (phien, ctx) => (await goiApi(mt.api, "GET", `/contexts/${ctx}/batches`, undefined, phien.token)).batches ?? [];
const B8 = (await dotCua(phien8, G8))[0]?.batch_id ?? (await dotCua(phien8, G8))[0]?.id;

/** One count on the LOCAL stack's database (read only), or null without a URL. */
function demSql(sql) {
  const url = (process.env.MOBILE_DATABASE_URL ?? "").replace("postgresql+psycopg", "postgresql");
  if (!url) return null;
  const r = spawnSync("psql", [url, "-Atc", sql], { encoding: "utf8" });
  return r.status === 0 ? Number(String(r.stdout).trim()) : null;
}
const sach = (id) => String(id).replace(/[^0-9a-f-]/g, "");
const soKhoanChi = (ctx) => demSql(`select count(*) from expenses where context_id = '${sach(ctx)}'`);
const soDot = (ctx) => demSql(`select count(*) from collection_batches where context_id = '${sach(ctx)}'`);

const mo = (cfg, persona, path, them = {}) => trangMoi(mt, cauHinh(cfg), { persona, path, ...them });
const duongBill = (ctx) => `/smart-split/moi/review${ctx ? `?ctx=${ctx}` : ""}`;

/** A control by its whole visible name (icon glyphs stripped), scrolled into view. */
const timNut = (page, chu, { trong = null, vai = '[role="button"],button,[role="checkbox"],[role="radio"],a[href]' } = {}) =>
  page.evaluate(
    ({ chu, trong, vai }) => {
      const ten = (x) => (x.getAttribute("aria-label") || (x.innerText ?? "")).replace(/[-]/g, "").replace(/\s+/g, " ").trim();
      const goc = trong ? document.querySelector(trong) : document;
      if (!goc) return null;
      const e = [...goc.querySelectorAll(vai)].find((x) => ten(x) === chu && x.getBoundingClientRect().width > 0);
      if (!e) return null;
      e.scrollIntoView({ block: "center", behavior: "instant" });
      // Only vertical: scrollIntoView also scrolls overflow-hidden rows sideways, which no finger can.
      for (let n = e.parentElement; n; n = n.parentElement) if (n.scrollLeft && !/(auto|scroll)/.test(getComputedStyle(n).overflowX)) n.scrollLeft = 0;
      const r = e.getBoundingClientRect();
      return { x: r.left + r.width / 2, y: r.top + r.height / 2, w: Math.round(r.width), h: Math.round(r.height), top: Math.round(r.top), bottom: Math.round(r.bottom), disabled: e.getAttribute("aria-disabled") };
    },
    { chu, trong, vai },
  );
const bam = async (t, cdp, chu, { cho = 700, ...o } = {}) => {
  const n = await timNut(t.page, chu, o);
  if (!n) return null;
  await cham(cdp, n);
  await t.page.waitForTimeout(cho);
  return n;
};
const coChu = (page, re) => page.evaluate((s) => new RegExp(s).test(document.body.innerText ?? ""), re.source);
/** Type into an input by its accessible name, then leave it (the number boxes commit on blur). */
const go = async (page, nhan, chu) => {
  const o = page.locator(`[aria-label="${nhan}"]`).first();
  await o.scrollIntoViewIfNeeded();
  await o.evaluate((e) => {
    for (let n = e.parentElement; n; n = n.parentElement) if (n.scrollLeft && !/(auto|scroll)/.test(getComputedStyle(n).overflowX)) n.scrollLeft = 0;
  });
  await o.click();
  await o.fill("");
  await o.type(chu, { delay: 4 });
  await page.keyboard.press("Tab");
};
const buocHienTai = (page) => page.evaluate(() => document.querySelector('[role="progressbar"][aria-label^="Bước"]')?.getAttribute("aria-label") ?? null);
const oGiaTri = (page) =>
  page.evaluate(() => [...document.querySelectorAll('input[aria-label^="Ô "]')].filter((e) => e.getBoundingClientRect().width > 0).map((e) => `${e.getAttribute("aria-label")}=${e.value}`));

const RE_TIEN = /^[-+]?\d{1,3}(\.\d{3})*\s?đ$/;
/**
 * Every amount on screen and whether it is cut: an element whose own text or
 * accessible name is a sum, with its box, `scrollWidth > clientWidth` (the
 * ellipsis of `numberOfLines={1}`), or a clipping ancestor it sticks out of.
 */
const tienTrenMan = (page) =>
  page.evaluate((re) => {
    const RE = new RegExp(re);
    const ra = [];
    for (const e of document.querySelectorAll("body *")) {
      const nhan = e.getAttribute("aria-label");
      const rieng = [...e.childNodes].some((n) => n.nodeType === 3 && RE.test(n.textContent.trim()));
      if (!(nhan && RE.test(nhan)) && !rieng) continue;
      const r = e.getBoundingClientRect();
      if (r.width < 1 || r.height < 1) continue;
      let catBoi = null;
      for (let n = e.parentElement; n && n !== document.body; n = n.parentElement) {
        const s = getComputedStyle(n);
        if (s.overflowX === "visible" && s.overflow === "visible") continue;
        const p = n.getBoundingClientRect();
        if (r.left < p.left - 1 || r.right > p.right + 1) catBoi = n.getAttribute("data-testid") ?? n.tagName;
        break;
      }
      ra.push({ tien: nhan && RE.test(nhan) ? nhan : e.textContent.trim(), hien: (e.innerText ?? "").trim(), w: Math.round(r.width), x: Math.round(r.left), phai: Math.round(r.right), y: Math.round(r.top), elip: e.scrollWidth > e.clientWidth + 1, catBoi });
    }
    return ra;
  }, RE_TIEN.source);
/**
 * Mark the cut sums (a data attribute the overlay can select), bring the first
 * into view and capture it boxed. `fullPage` cannot do this: the page scrolls
 * inside react-native-web's ScrollView, not the document.
 */
const chupTienCat = async (page, suKien, id) => {
  const n = await page.evaluate((re) => {
    const RE = new RegExp(re);
    let dem = 0;
    for (const e of document.querySelectorAll("[data-audit-cat]")) e.removeAttribute("data-audit-cat");
    for (const e of document.querySelectorAll("body *")) {
      const nhan = e.getAttribute("aria-label");
      const rieng = [...e.childNodes].some((n) => n.nodeType === 3 && RE.test(n.textContent.trim()));
      if (!((nhan && RE.test(nhan)) || rieng) || e.getBoundingClientRect().width < 1) continue;
      if (e.scrollWidth > e.clientWidth + 1) {
        e.setAttribute("data-audit-cat", "1");
        if (dem === 0) {
          e.scrollIntoView({ block: "center", behavior: "instant" });
          for (let n = e.parentElement; n; n = n.parentElement) if (n.scrollLeft && !/(auto|scroll)/.test(getComputedStyle(n).overflowX)) n.scrollLeft = 0;
        }
        dem += 1;
      }
    }
    return dem;
  }, RE_TIEN.source);
  if (!n) return null;
  await page.waitForTimeout(300);
  await chup(page, { out: mt.out, id, suKien, chuThich: [{ selector: "[data-audit-cat]", nhan: "số tiền bị cắt" }] });
  return id;
};
/** The closest Nếp (dock edge or an in-page moment) to any visible sum. */
const nepGanTien = (page) =>
  page.evaluate((re) => {
    const RE = new RegExp(re);
    const thay = (e) => {
      const r = e.getBoundingClientRect();
      return r.width > 1 && r.height > 1 && r.bottom > 0 && r.top < innerHeight && r.right > 0 && r.left < innerWidth;
    };
    const nep = [...document.querySelectorAll('[data-testid="nep-mep"],[data-testid="nep-dia"],[role="img"][aria-label^="Nếp"]')].filter(thay);
    const tien = [...document.querySelectorAll("body *")].filter((e) => {
      const nhan = e.getAttribute("aria-label");
      return ((nhan && RE.test(nhan)) || [...e.childNodes].some((n) => n.nodeType === 3 && RE.test(n.textContent.trim()))) && thay(e);
    });
    const ra = [];
    for (const n of nep) {
      const a = n.getBoundingClientRect();
      for (const t of tien) {
        const b = t.getBoundingClientRect();
        const dx = Math.max(0, b.left - a.right, a.left - b.right);
        const dy = Math.max(0, b.top - a.bottom, a.top - b.bottom);
        ra.push({ nep: n.getAttribute("data-testid") ?? n.getAttribute("aria-label"), tien: t.getAttribute("aria-label") ?? t.textContent.trim(), cach: Math.round(Math.hypot(dx, dy)) });
      }
    }
    return { soNep: nep.length, ganNhat: ra.sort((x, y) => x.cach - y.cach)[0] ?? null };
  }, RE_TIEN.source);
/** Scroll the page top to bottom in steps and keep the smallest Nếp-to-sum distance seen. */
const quetNepTien = async (page) => {
  const cao = await page.evaluate(() => document.scrollingElement.scrollHeight - innerHeight);
  let nhoNhat = null;
  for (let y = 0; y <= cao + 1; y += Math.max(60, Math.round(cao / 24) || 60)) {
    await page.evaluate((y) => scrollTo(0, y), y);
    await page.waitForTimeout(90);
    const d = await nepGanTien(page);
    if (d.ganNhat && (!nhoNhat || d.ganNhat.cach < nhoNhat.cach)) nhoNhat = { ...d.ganNhat, cuon: y };
  }
  await page.evaluate(() => scrollTo(0, 0));
  return nhoNhat;
};
/** Where a sentence is now, relative to the window. */
const viTriCau = (page, re) =>
  page.evaluate((s) => {
    const R = new RegExp(s);
    const e = [...document.querySelectorAll("body *")].find((x) => [...x.childNodes].some((n) => n.nodeType === 3 && R.test(n.textContent)));
    if (!e) return null;
    const r = e.getBoundingClientRect();
    return { top: Math.round(r.top), bottom: Math.round(r.bottom), trongMan: r.bottom > 0 && r.top < innerHeight, cao: innerHeight, text: e.textContent.trim().slice(0, 140) };
  }, re.source);

// The bill used on both groups: a long dish, an unbroken word, eight-digit sums.
const MON = [
  ["Lẩu gà lá é nồi lớn cho cả nhóm hai mươi người, thêm nấm và rau rừng", "2", "12345678"],
  ["Bia", "24", "960000"],
  ["NướcSuốiChaiLớnKhôngCóKhoảngTrắngNàoĐểNgắt", "20", "400000"],
];
const TONG = "13.705.678đ";
/** From step 1: «Nhập tay», then fill `mon` lines. */
const nhapBill = async (t, cdp, mon = MON) => {
  await bam(t, cdp, "Nhập tay", { cho: 900 });
  for (let i = 0; i < mon.length; i += 1) {
    if (i > 0) await bam(t, cdp, "Thêm món", { cho: 400 });
    // A line that is not open (a long bill opens only the new one) opens on tap.
    const moRa = await t.page.locator(`[aria-label="Ô tên món ${i + 1}"]`).count();
    if (!moRa) await bam(t, cdp, `Sửa Món ${i + 1}`, { cho: 400 });
    if (mon[i][0] !== null) await go(t.page, `Ô tên món ${i + 1}`, mon[i][0]);
    await go(t.page, `Ô số lượng món ${i + 1}`, mon[i][1]);
    await go(t.page, `Ô tiền món ${i + 1}`, mon[i][2]);
  }
  await t.page.waitForTimeout(400);
};
/** From step 2 to step 3 (creates a draft bill on the server, no expense). */
const sangGanMon = async (t, cdp) => {
  await bam(t, cdp, "Tiếp: ai dùng món nào?", { cho: 400 });
  await t.page.waitForFunction(() => /Bước 3/.test(document.querySelector('[role="progressbar"]')?.getAttribute("aria-label") ?? ""), null, { timeout: 12_000 }).catch(() => undefined);
  await t.page.waitForTimeout(900);
};
const sangKetQua = async (t, cdp) => {
  await bam(t, cdp, "Xem kết quả", { cho: 400 });
  await t.page.waitForFunction(() => /Bước 4/.test(document.querySelector('[role="progressbar"]')?.getAttribute("aria-label") ?? ""), null, { timeout: 15_000 }).catch(() => undefined);
  await t.page.waitForTimeout(900);
};
/** Open /plan, then walk into `path` inside the app, so Back has an app screen to return to. */
const moQuaPlan = async (cfg, persona, path) => {
  const t = await mo(cfg, persona, "/plan");
  await t.page.evaluate((p) => {
    history.pushState({}, "", p);
    dispatchEvent(new PopStateEvent("popstate"));
  }, path);
  await choOn(t.page, { mang: t.mang });
  await t.page.waitForTimeout(700);
  return t;
};
/** The seats of the bill table: name, checked, and where a finger aiming at the figure lands. */
const gheBan = (page) =>
  page.evaluate(() => {
    const ghe = [...document.querySelectorAll('[data-testid="ban-gan-mon"] [role="checkbox"]')];
    return ghe.map((g) => {
      const r = g.getBoundingClientRect();
      // The standee is the seat's largest drawn child; aim at its centre.
      const hinh = [...g.querySelectorAll("svg,div")].filter((c) => c.getBoundingClientRect().width >= 20 && c.getBoundingClientRect().height >= 20 && !c.innerText?.trim()).sort((a, b) => b.getBoundingClientRect().width - a.getBoundingClientRect().width)[0];
      const h = (hinh ?? g).getBoundingClientRect();
      const x = h.left + h.width / 2;
      const y = h.top + h.height / 2;
      const tren = document.elementFromPoint(x, y);
      const trung = tren?.closest('[role="checkbox"]');
      return { ten: g.getAttribute("aria-label"), chon: g.getAttribute("aria-checked"), x: Math.round(x), y: Math.round(y), hop: [Math.round(r.left), Math.round(r.top), Math.round(r.width), Math.round(r.height)], trungGhe: trung === g, trungVao: trung && trung !== g ? trung.getAttribute("aria-label") : null };
    });
  });

try {
  // ----------------------------------------------- five steps, no ledger write
  if (chay("buoc")) {
    for (const cfg of ["C1", "C2", "C3", "C4", "C5"]) {
      const t = await mo(cfg, P8, duongBill(null));
      const cdp = await cdpCua(t.page);
      await nhapBill(t, cdp);
      await t.page.evaluate(() => scrollTo(0, 0));
      await chup(t.page, { out: mt.out, id: `EV-F04-BUOC2-${cfg}`, suKien: t.suKien });
      const tien2 = await tienTrenMan(t.page);
      const cat2 = await chupTienCat(t.page, t.suKien, `EV-F04-TIEN-CAT-B2-${cfg}`);
      const tran2 = await t.page.evaluate(() => document.scrollingElement.scrollWidth - innerWidth);
      await sangGanMon(t, cdp);
      await chup(t.page, { out: mt.out, id: `EV-F04-BUOC3-${cfg}`, suKien: t.suKien });
      const ghe = await gheBan(t.page);
      const tien3 = await tienTrenMan(t.page);
      const cat3 = await chupTienCat(t.page, t.suKien, `EV-F04-TIEN-CAT-B3-${cfg}`);
      await sangKetQua(t, cdp);
      await chup(t.page, { out: mt.out, id: `EV-F04-BUOC4-${cfg}`, suKien: t.suKien });
      const tien4 = await tienTrenMan(t.page);
      const cat4 = await chupTienCat(t.page, t.suKien, `EV-F04-TIEN-CAT-B4-${cfg}`);
      const buoc4 = await buocHienTai(t.page);
      const moTa = (ds, buoc) => ds.filter((x) => x.elip || x.catBoi).map((x) => `B${buoc} ${x.tien} còn ${x.w}px`);
      const catTien = [...moTa(tien2, 2), ...moTa(tien3, 3), ...moTa(tien4, 4)];
      log({ cfg, tran2, ghe: ghe.length, catTien, buoc4, tong: tien2.some((x) => x.tien === TONG) });
      ket({ tc: "TC-F04-MON-DAI", screen: "F04.S01", state: "3 món: tên 70 ký tự, một từ 41 ký tự liền, 12.345.678đ", action: "nhập tay ở bước 2, sang bước 3 và 4", cauHinh: cfg, expected: "không tràn ngang; không số tiền nào bị cắt hay ellipsis; tổng đúng", status: tran2 <= 0 && catTien.length === 0 && /Bước 4/.test(buoc4 ?? "") ? "PASS" : "FAIL", evidence: [`EV-F04-BUOC2-${cfg}`, `EV-F04-BUOC3-${cfg}`, `EV-F04-BUOC4-${cfg}`, cat2, cat3, cat4].filter(Boolean), ghiChu: `tràn ${tran2}px; số tiền bị cắt ${catTien.length}${catTien.length ? ": " + catTien.join("; ") : ""}; tới ${buoc4}; ${ghe.length} ghế` });
      await t.context.close();
    }
  }

  // ------------------------------------ why «Tiếp» / «Xem kết quả» cannot go on
  if (chay("chan")) {
    const t = await mo("C1", P8, duongBill(null));
    const cdp = await cdpCua(t.page);
    // Six lines, the fourth without a name: long enough that «Tiếp» sits far below the top.
    const sau = [...MON, ["Cơm chiên", "1", "85000"], [null, "1", "50000"], ["Trà đá", "8", "40000"]];
    await nhapBill(t, cdp, sau);
    const khoaTruoc = await viTriCau(t.page, /Một món chưa có tên/);
    const nut = await timNut(t.page, "Tiếp: ai dùng món nào?");
    await cham(cdp, nut);
    await t.page.waitForTimeout(900);
    const sauBam = await t.page.evaluate(() => [...document.querySelectorAll("body *")].filter((e) => [...e.childNodes].some((n) => n.nodeType === 3 && /Một món chưa có tên/.test(n.textContent))).map((e) => { const r = e.getBoundingClientRect(); return { top: Math.round(r.top), bottom: Math.round(r.bottom), trongMan: r.bottom > 0 && r.top < innerHeight }; }));
    const buoc = await buocHienTai(t.page);
    await chup(t.page, { out: mt.out, id: "EV-F04-CHAN-TEN-C1", suKien: t.suKien, chuThich: [{ selector: '[aria-label="Tiếp: ai dùng món nào?"]', nhan: "vừa chạm" }] });
    log({ khoaTruoc, nut: { top: nut?.top, disabled: nut?.disabled }, sauBam, buoc });
    const thay = sauBam.some((x) => x.trongMan);
    ket({ tc: "TC-F04-CHAN-TEN", screen: "F04.S01", state: "bước 2, 6 món, món thứ 5 chưa có tên, đang ở cuối trang", action: "chạm «Tiếp: ai dùng món nào?»", cauHinh: "C1", expected: "ở lại bước 2 và lý do hiện ở chỗ người dùng đang nhìn (DESIGN: lỗi là một câu cạnh chỗ bấm)", status: /Bước 2/.test(buoc ?? "") && thay ? "PASS" : "FAIL", evidence: ["EV-F04-CHAN-TEN-C1"], ghiChu: `ở ${buoc}; nút không mờ (aria-disabled ${nut?.disabled ?? "không có"}); câu lý do có ${sauBam.length} chỗ, ${sauBam.map((x) => `y ${x.top}–${x.bottom}${x.trongMan ? " thấy" : " ngoài màn"}`).join(", ")}; cửa sổ cao 844` });
    // Step 3: one dish nobody had.
    const t2 = await mo("C1", P8, duongBill(null));
    const cdp2 = await cdpCua(t2.page);
    await nhapBill(t2, cdp2);
    await sangGanMon(t2, cdp2);
    await bam(t2, cdp2, `Sửa người dùng ${MON[2][0]}`, { cho: 500 });
    await bam(t2, cdp2, "Bỏ hết", { cho: 500, vai: '[role="button"],button' });
    const nut3 = await timNut(t2.page, "Xem kết quả");
    await cham(cdp2, nut3);
    await t2.page.waitForTimeout(900);
    const buoc3 = await buocHienTai(t2.page);
    const cau3 = await t2.page.evaluate(() => [...document.querySelectorAll("body *")].filter((e) => [...e.childNodes].some((n) => n.nodeType === 3 && /chưa (ai|có người)|Chưa chọn/.test(n.textContent))).map((e) => { const r = e.getBoundingClientRect(); return { text: e.textContent.trim().slice(0, 120), top: Math.round(r.top), trongMan: r.bottom > 0 && r.top < innerHeight }; }));
    await chup(t2.page, { out: mt.out, id: "EV-F04-CHAN-NGUOI-C1", suKien: t2.suKien });
    log({ buoc3, cau3 });
    const lyDo = cau3.filter((x) => !/^Chưa chọn người$/.test(x.text));
    ket({ tc: "TC-F04-CHAN-NGUOI", screen: "F04.S01", state: "bước 3, một món bỏ hết người", action: "chạm «Xem kết quả»", cauHinh: "C1", expected: "ở lại bước 3, câu lý do thấy được cạnh chỗ bấm", status: /Bước 3/.test(buoc3 ?? "") && lyDo.some((x) => x.trongMan) ? "PASS" : "FAIL", evidence: ["EV-F04-CHAN-NGUOI-C1"], ghiChu: `ở ${buoc3}; câu: ${lyDo.map((x) => `«${x.text.slice(0, 70)}» y ${x.top}${x.trongMan ? " thấy" : " ngoài màn"}`).join("; ") || "không có"}` });
    await t.context.close();
    await t2.context.close();
  }

  // ------------------------------------------------- going back inside and out
  if (chay("lui")) {
    // The chevron steps back inside the flow and keeps what was typed.
    const t = await mo("C1", P8, duongBill(null));
    const cdp = await cdpCua(t.page);
    await nhapBill(t, cdp);
    await sangGanMon(t, cdp);
    await sangKetQua(t, cdp);
    const buoc = [await buocHienTai(t.page)];
    for (let i = 0; i < 2; i += 1) {
      await bam(t, cdp, "Quay lại", { cho: 900 });
      buoc.push(await buocHienTai(t.page));
    }
    const giu = await oGiaTri(t.page);
    const tongCon = await coChu(t.page, new RegExp(TONG.replace(/\./g, "\\.")));
    // One more: step 2 → step 1. Nothing on step 1 leads back to the typed bill.
    await bam(t, cdp, "Quay lại", { cho: 900 });
    buoc.push(await buocHienTai(t.page));
    const nutBuoc1 = await t.page.evaluate(() => [...document.querySelectorAll('[role="button"]')].filter((e) => e.getBoundingClientRect().width > 0).map((e) => (e.getAttribute("aria-label") || e.innerText).replace(/[-]/g, "").trim()));
    await bam(t, cdp, "Nhập tay", { cho: 900 });
    const sauNhapLai = await oGiaTri(t.page);
    await chup(t.page, { out: mt.out, id: "EV-F04-LUI-MAT-C1", suKien: t.suKien });
    log({ buoc, giu, tongCon, nutBuoc1, sauNhapLai });
    ket({ tc: "TC-F04-LUI-TRONG", screen: "F04.S01", state: "bước 4, bill 3 món đã gõ", action: "chạm «Quay lại» của đầu màn hai lần", cauHinh: "C1", expected: "lùi 4 → 3 → 2, giữ nguyên các món và số tiền", status: /Bước 3/.test(buoc[1] ?? "") && /Bước 2/.test(buoc[2] ?? "") && tongCon ? "PASS" : "FAIL", evidence: [], ghiChu: `${buoc.slice(0, 3).map((b) => b?.slice(0, 7)).join(" → ")}; tổng ${TONG} ${tongCon ? "còn" : "mất"}` });
    const mat = !sauNhapLai.some((v) => v.includes("Lẩu gà"));
    ket({ tc: "TC-F04-LUI-VE-BUOC1", screen: "F04.S01", state: "bước 2, bill 3 món đã gõ", action: "«Quay lại» về bước 1, rồi chạm «Nhập tay» (bước 1 không có lối nào khác về bill)", cauHinh: "C1", expected: "bill đã gõ còn nguyên, hoặc có câu hỏi trước khi bỏ", status: mat ? "FAIL" : "PASS", evidence: ["EV-F04-LUI-MAT-C1"], ghiChu: `bước 1 có: ${nutBuoc1.join(", ")}; sau «Nhập tay»: ${sauNhapLai.join(", ").slice(0, 160)}` });
    await t.context.close();

    // Browser Back and reload at step 3.
    const t2 = await moQuaPlan("C1", P8, duongBill(null));
    const cdp2 = await cdpCua(t2.page);
    await nhapBill(t2, cdp2);
    await sangGanMon(t2, cdp2);
    const truoc = await duongDan(t2.page);
    await t2.page.goBack();
    await choOn(t2.page, { mang: t2.mang });
    await t2.page.waitForTimeout(900);
    const sauBack = await duongDan(t2.page);
    await t2.page.goForward();
    await choOn(t2.page, { mang: t2.mang });
    await t2.page.waitForTimeout(1200);
    const sauToi = { duong: await duongDan(t2.page), buoc: await buocHienTai(t2.page) };
    await chup(t2.page, { out: mt.out, id: "EV-F04-BACK-C1", suKien: t2.suKien });
    log({ truoc, sauBack, sauToi });
    ket({ tc: "TC-F04-BACK-TRINH-DUYET", screen: "F04.S01", state: "bước 3, bill đã gõ", action: "Back của trình duyệt, rồi Forward", cauHinh: "C1", expected: "Back lùi một bước trong luồng như nút của màn (hoặc hỏi trước khi bỏ bill); Forward không mất bill", status: sauBack === truoc || /Bước 2|Bước 3/.test(sauToi.buoc ?? "") ? "PASS" : "FAIL", evidence: ["EV-F04-BACK-C1"], ghiChu: `trước ${truoc.split("?")[0]}; sau Back ${sauBack}; sau Forward ${sauToi.duong.split("?")[0]}, ${sauToi.buoc}` });
    await t2.context.close();

    const t3 = await mo("C1", P8, duongBill(null));
    const cdp3 = await cdpCua(t3.page);
    await nhapBill(t3, cdp3);
    await sangGanMon(t3, cdp3);
    await t3.page.reload();
    await choOn(t3.page, { mang: t3.mang });
    await t3.page.waitForTimeout(1200);
    const sauTai = await buocHienTai(t3.page);
    log({ sauTai });
    ket({ tc: "TC-F04-TAI-LAI", screen: "F04.S01", state: "bước 3 (máy chủ đã giữ bản nháp bill)", action: "tải lại trang", cauHinh: "C1", expected: "trở lại được bill đang làm, hoặc được báo là bill đã bỏ", status: /Bước 1/.test(sauTai ?? "") ? "FAIL" : "PASS", evidence: [], ghiChu: `sau khi tải lại: ${sauTai}, không có câu nào nhắc bill vừa làm` });
    await t3.context.close();
  }

  // ---------------------------------------------------- the bill table, 8 seats
  if (chay("ban")) {
    const t = await mo("C1", P8, duongBill(null));
    const cdp = await cdpCua(t.page);
    await nhapBill(t, cdp);
    await sangGanMon(t, cdp);
    const ghe0 = await gheBan(t.page);
    const dich = ghe0.find((g) => g.ten.endsWith("Tuấn Kiệt")) ?? ghe0[1];
    await cham(cdp, { x: dich.x, y: dich.y });
    await t.page.waitForTimeout(500);
    const ghe1 = await gheBan(t.page);
    const dem1 = await t.page.evaluate(() => document.querySelector('[data-testid="ban-gan-mon"]')?.parentElement?.innerText.match(/(\d+) người/)?.[1] ?? null);
    const sau1 = ghe1.find((g) => g.ten === dich.ten);
    await cham(cdp, { x: dich.x, y: dich.y });
    await t.page.waitForTimeout(500);
    const sau2 = (await gheBan(t.page)).find((g) => g.ten === dich.ten);
    ket({ tc: "TC-F04-BAN-CHAM", screen: "F04.S01", layer: "-", state: "bước 3, 8 ghế, món 1 cả nhóm", action: `chạm hình nhân ${dich.ten} hai lần`, cauHinh: "C1", expected: "ghế đó ra rồi vào lại (aria-checked đổi), không ghế khác đổi", status: dich.chon === "true" && sau1?.chon === "false" && sau2?.chon === "true" && ghe1.filter((g) => g.ten !== dich.ten).every((g) => g.chon === "true") ? "PASS" : "FAIL", evidence: [], ghiChu: `${dich.chon} → ${sau1?.chon} → ${sau2?.chon}; ghế khác đổi: ${ghe1.filter((g) => g.ten !== dich.ten && g.chon !== "true").length}; 8 ghế, ${ghe0.filter((g) => !g.trungGhe).length} ghế chạm trúng ghế khác` });
    // Drag the dish card onto a seat.
    const the = await t.page.evaluate(() => {
      const e = document.querySelector('[aria-label^="Món trên bàn:"]');
      if (!e) return null;
      const r = e.getBoundingClientRect();
      return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
    });
    const dich2 = ghe0.find((g) => g.ten.endsWith("Hà Vy")) ?? ghe0[3];
    const cuonTruoc = await t.page.evaluate(() => scrollY);
    await keo(cdp, the, { x: dich2.x, y: dich2.y }, { ms: 500, buoc: 20 });
    await t.page.waitForTimeout(700);
    const sauKeo = (await gheBan(t.page)).find((g) => g.ten === dich2.ten);
    const cuonSau = await t.page.evaluate(() => scrollY);
    ket({ tc: "TC-F04-BAN-KEO", screen: "F04.S01", state: "bước 3, món 1 cả nhóm", action: `kéo thẻ món tới ${dich2.ten}`, cauHinh: "C1", expected: "ghế đó đổi (ra khỏi món); trang không cuộn theo", status: sauKeo?.chon === "false" && cuonSau === cuonTruoc ? "PASS" : "FAIL", evidence: [], ghiChu: `ghế ${dich2.chon} → ${sauKeo?.chon}; cuộn ${cuonTruoc} → ${cuonSau}` });
    // Keyboard: Tab to a seat, Space.
    await t.page.keyboard.press("Escape");
    const phim = await t.page.evaluate(async () => {
      const g = document.querySelector('[data-testid="ban-gan-mon"] [role="checkbox"]');
      g.focus();
      return { focus: document.activeElement === g, tab: g.getAttribute("tabindex"), ten: g.getAttribute("aria-label"), chon: g.getAttribute("aria-checked") };
    });
    await t.page.keyboard.press("Space");
    await t.page.waitForTimeout(400);
    const sauSpace = await t.page.evaluate((ten) => document.querySelector(`[aria-label="${ten}"]`)?.getAttribute("aria-checked"), phim.ten);
    await t.page.keyboard.press("Enter");
    await t.page.waitForTimeout(400);
    const sauEnter = await t.page.evaluate((ten) => document.querySelector(`[aria-label="${ten}"]`)?.getAttribute("aria-checked"), phim.ten);
    // The same on a checkbox of the per-dish roster (RosterPicker), outside the table.
    const phim2 = await t.page.evaluate(() => {
      const g = [...document.querySelectorAll('[role="checkbox"]')].find((e) => !e.closest('[data-testid="ban-gan-mon"]') && e.getBoundingClientRect().width > 0);
      if (!g) return null;
      g.scrollIntoView({ block: "center" });
      for (let n = g.parentElement; n; n = n.parentElement) if (n.scrollLeft && !/(auto|scroll)/.test(getComputedStyle(n).overflowX)) n.scrollLeft = 0;
      g.focus();
      return { focus: document.activeElement === g, ten: g.getAttribute("aria-label"), chon: g.getAttribute("aria-checked") };
    });
    let sauSpace2 = null;
    if (phim2) {
      await t.page.keyboard.press("Space");
      await t.page.waitForTimeout(400);
      sauSpace2 = await t.page.evaluate((ten) => document.querySelector(`[aria-label="${ten}"]`)?.getAttribute("aria-checked"), phim2.ten);
    }
    ket({ tc: "TC-F04-BAN-PHIM", screen: "F04.S01", state: "bước 3", action: "focus ghế đầu, phím Space rồi Enter; rồi Space trên một ô «tên · món» của danh sách dưới bàn", cauHinh: "C1", expected: "nhận focus; Space đổi trạng thái (checkbox, WAI-ARIA)", status: phim.focus && sauSpace !== phim.chon && (!phim2 || sauSpace2 !== phim2.chon) ? "PASS" : "FAIL", evidence: [], ghiChu: `ghế: focus ${phim.focus} (tabindex ${phim.tab}); ${phim.chon} → Space ${sauSpace} → Enter ${sauEnter}; ô danh sách «${phim2?.ten?.slice(0, 40)}»: ${phim2?.chon} → Space ${sauSpace2}` });
    await t.context.close();
  }

  // ------------------------------------------------ the bill table, 20 seats
  if (chay("ban-20")) {
    for (const cfg of ["C1", "C2", "C6"]) {
      const t = await mo(cfg, P20, duongBill(G20));
      const cdp = await cdpCua(t.page);
      await nhapBill(t, cdp);
      await sangGanMon(t, cdp);
      const ghe = await gheBan(t.page);
      const sai = ghe.filter((g) => !g.trungGhe);
      await t.page.evaluate(() => {
        const b = document.querySelector('[data-testid="ban-gan-mon"]');
        if (!b) return;
        b.scrollIntoView({ block: "start" });
        for (let n = b.parentElement; n; n = n.parentElement) if (n.scrollLeft && !/(auto|scroll)/.test(getComputedStyle(n).overflowX)) n.scrollLeft = 0;
      });
      await t.page.waitForTimeout(300);
      await chup(t.page, { out: mt.out, id: `EV-F04-BAN-20-${cfg}`, suKien: t.suKien, chuThich: [{ selector: '[data-testid="ban-gan-mon"]', nhan: "20 ghế" }] });
      // Tap the figure of one seat that another seat covers, and see which one changed.
      let thu = null;
      if (sai.length) {
        const g = sai[0];
        const truoc = await gheBan(t.page);
        const a = truoc.find((x) => x.ten === g.ten);
        await cham(cdp, { x: a.x, y: a.y });
        await t.page.waitForTimeout(500);
        const sau = await gheBan(t.page);
        thu = { nham: a.ten, doi: sau.filter((x) => truoc.find((y) => y.ten === x.ten)?.chon !== x.chon).map((x) => x.ten) };
      }
      log({ cfg, tong: ghe.length, sai: sai.length, vd: sai.slice(0, 4).map((g) => `${g.ten}→${g.trungVao}`), thu });
      ket({ tc: "TC-F04-BAN-20", screen: "F04.S01", state: "bước 3, nhóm 20 người", action: "chạm vào giữa hình nhân của từng ghế (elementFromPoint), rồi chạm thật một ghế bị che", cauHinh: cfg, expected: "chạm vào hình nhân nào thì đúng ghế đó; tên đọc được", status: sai.length === 0 ? "PASS" : "FAIL", evidence: [`EV-F04-BAN-20-${cfg}`], ghiChu: `${sai.length}/${ghe.length} ghế: chạm vào hình nhân trúng ghế khác${thu ? `; chạm thật «${thu.nham}» đổi ${thu.doi.join(", ") || "không ghế nào"}` : ""}` });
      await t.context.close();
    }
  }

  // ------------------------------------------------------------ bill from a photo
  if (chay("anh")) {
    const t = await mo("C1", P8, duongBill(null));
    const cdp = await cdpCua(t.page);
    // Cancel the chooser.
    const [chon1] = await Promise.all([t.page.waitForEvent("filechooser", { timeout: 6000 }).catch(() => null), bam(t, cdp, "Chọn ảnh bill", { cho: 200 })]);
    if (chon1) await chon1.setFiles([]);
    await t.page.waitForTimeout(900);
    const sauHuy = { cau: await coChu(t.page, /Không chọn ảnh/), nhapTay: (await timNut(t.page, "Nhập tay"))?.disabled ?? "không có" };
    ket({ tc: "TC-L31-HUY", screen: "F04.S01", layer: "L31", state: "bước 1", action: "«Chọn ảnh bill», huỷ bộ chọn tệp", cauHinh: "C1", expected: "ở lại bước 1, nút còn dùng được, không kẹt trạng thái bận", status: chon1 && (await timNut(t.page, "Nhập tay")) ? "PASS" : "FAIL", evidence: [], ghiChu: `bộ chọn ${chon1 ? "mở" : "không mở"}; câu «Không chọn ảnh.» ${sauHuy.cau ? "có" : "không"}; «Nhập tay» aria-disabled ${sauHuy.nhapTay}` });
    // Pick a synthetic photo.
    const [chon2] = await Promise.all([t.page.waitForEvent("filechooser", { timeout: 6000 }).catch(() => null), bam(t, cdp, "Chọn ảnh bill", { cho: 200 })]);
    if (chon2) await chon2.setFiles([{ name: "bill-tong-hop.png", mimeType: "image/png", buffer: Buffer.from(pngThuBytes(480, 640)) }]);
    await t.page.waitForTimeout(1500);
    const xem = { tieuDe: await coChu(t.page, /Ảnh này đúng bill chứ/), nep: await t.page.evaluate(() => !!document.querySelector('[role="img"][aria-label="Nếp giơ máy ảnh chụp hoá đơn"]')), buoc: await buocHienTai(t.page) };
    await chup(t.page, { out: mt.out, id: "EV-F04-ANH-XEM-C1", suKien: t.suKien });
    ket({ tc: "TC-F04-ANH-XEM", screen: "F04.S01", state: "bước 1, đã chọn ảnh tổng hợp", action: "chọn ảnh", cauHinh: "C1", expected: "xem trước ảnh, chưa gửi gì; «Dùng ảnh này» và «Chọn ảnh khác»; Nếp M2", status: xem.tieuDe ? "PASS" : "FAIL", evidence: ["EV-F04-ANH-XEM-C1"], ghiChu: `${xem.buoc}; Nếp M2 ${xem.nep ? "có" : "không"}` });
    // Send it: no AI key on this stack, so the server cannot read it.
    const guiLuc = Date.now();
    let quet = null;
    t.page.on("response", (r) => {
      if (/\/receipts\/scan/.test(r.url())) quet = { status: r.status(), ms: Date.now() - guiLuc };
    });
    await bam(t, cdp, "Dùng ảnh này", { cho: 200 });
    await t.page.waitForFunction(() => /nhập món bằng tay|Không đọc được/.test(document.body.innerText), null, { timeout: 20_000 }).catch(() => undefined);
    const hienSau = Date.now() - guiLuc;
    log({ quet, hienSau });
    const cau = await t.page.evaluate(() => [...document.querySelectorAll("body *")].filter((e) => [...e.childNodes].some((n) => n.nodeType === 3 && /nhập món bằng tay/.test(n.textContent))).map((e) => { const r = e.getBoundingClientRect(); return { text: e.textContent.trim(), top: Math.round(r.top), trongMan: r.bottom > 0 && r.top < innerHeight }; }));
    const nutCon = await t.page.evaluate(() => [...document.querySelectorAll('[role="button"]')].filter((e) => e.getBoundingClientRect().width > 0).map((e) => (e.getAttribute("aria-label") || e.innerText).replace(/[-]/g, "").trim()));
    await chup(t.page, { out: mt.out, id: "EV-F04-ANH-DOC-C1", suKien: t.suKien });
    log({ cau, nutCon });
    const coNhapTay = nutCon.some((n) => n === "Nhập tay");
    ket({ tc: "TC-F04-ANH-DOC", screen: "F04.S01", state: "xem ảnh, máy chủ không đọc được (không có khoá AI)", action: "«Dùng ảnh này»", cauHinh: "C1", expected: "một câu nói vì sao, và lối đi tiếp mà câu đó chỉ tới («nhập món bằng tay») có ngay trên màn", status: cau.some((x) => x.trongMan) && coNhapTay ? "PASS" : "FAIL", evidence: ["EV-F04-ANH-DOC-C1"], ghiChu: `máy chủ ${quet ? `${quet.status} sau ${quet.ms} ms` : "chưa trả lời"}; câu hiện sau ${hienSau} ms: ${cau.map((x) => `«${x.text.slice(0, 130)}»${x.trongMan ? "" : " (ngoài màn)"}`).join("; ") || "không"}; nút trên màn: ${nutCon.join(", ")}` });
    await t.context.close();
  }

  // ------------------------------ fold/unfold rows: is the state in the DOM?
  if (chay("aria")) {
    const t = await mo("C1", P8, duongBill(null));
    const cdp = await cdpCua(t.page);
    const doc = () => t.page.evaluate(() => [...document.querySelectorAll('[aria-label^="Gấp "],[aria-label^="Sửa "]')].map((e) => ({ ten: e.getAttribute("aria-label"), exp: e.getAttribute("aria-expanded") })));
    await nhapBill(t, cdp, [["Bún bò", "1", "50000"]]);
    const b2 = await doc();
    await sangGanMon(t, cdp);
    const b3 = await doc();
    const tatCa = [...b2, ...b3];
    ket({ tc: "TC-F04-ARIA-GAP", screen: "F04.S01", state: "bước 2 và 3, dòng món gập/mở", action: "đọc thuộc tính của nút gập/mở", cauHinh: "C1", expected: "nút gập/mở báo trạng thái bằng aria-expanded", status: tatCa.length && tatCa.every((x) => x.exp !== null) ? "PASS" : "FAIL", issue: "UI-003", evidence: [], ghiChu: tatCa.map((x) => `«${x.ten}» aria-expanded ${x.exp ?? "không có"}`).join("; ") });
    await t.context.close();
  }

  // ----------------------------------- the M2 moment beside the photo heading
  if (chay("m2")) {
    for (const cfg of ["C1", "C2", "C3", "C6"]) {
      const t = await mo(cfg, P8, duongBill(null));
      const cdp = await cdpCua(t.page);
      const [fc] = await Promise.all([t.page.waitForEvent("filechooser", { timeout: 6000 }).catch(() => null), bam(t, cdp, "Chọn ảnh bill", { cho: 200 })]);
      if (fc) await fc.setFiles([{ name: "bill-tong-hop.png", mimeType: "image/png", buffer: Buffer.from(pngThuBytes(480, 640)) }]);
      await t.page.waitForTimeout(2500);
      const m = await t.page.evaluate(() => {
        const e = document.querySelector('[role="img"][aria-label="Nếp giơ máy ảnh chụp hoá đơn"]');
        const h = [...document.querySelectorAll("*")].find((x) => [...x.childNodes].some((n) => n.nodeType === 3 && /Ảnh này đúng bill chứ/.test(n.textContent)));
        const r = e?.getBoundingClientRect();
        const hr = h?.getBoundingClientRect();
        return { w: innerWidth, nep: r ? [Math.round(r.left), Math.round(r.right)] : null, tieuDe: hr ? [Math.round(hr.left), Math.round(hr.right)] : null };
      });
      await chup(t.page, { out: mt.out, id: `EV-F04-NEP-M2-${cfg}`, suKien: t.suKien, chuThich: [{ selector: '[role="img"][aria-label="Nếp giơ máy ảnh chụp hoá đơn"]', nhan: "Nếp M2" }] });
      const ngoai = m.nep ? Math.max(0, m.nep[1] - m.w) : null;
      ket({ tc: "TC-F04-NEP-M2-KHUNG", screen: "F04.S01", state: "xem trước ảnh bill (Nếp M2 cạnh tiêu đề)", action: "chọn ảnh", cauHinh: cfg, expected: "Nếp chiếm chỗ riêng trong bố cục, nằm trọn trong màn; tiêu đề giữ lề phải 16dp", status: m.nep && ngoai === 0 && m.tieuDe && m.tieuDe[1] <= m.w - 16 ? "PASS" : "FAIL", evidence: [`EV-F04-NEP-M2-${cfg}`], ghiChu: `cửa sổ ${m.w}; Nếp x ${m.nep?.join("–")} (ra ngoài ${ngoai}px); tiêu đề x ${m.tieuDe?.join("–")}` });
      await t.context.close();
    }
  }

  // ------------------------------------------------ recording (chat-test group)
  if (chay("ghi")) {
    const truoc = soKhoanChi(G20);
    if (truoc === null) {
      ket({ tc: "TC-F04-GHI-CHAM-DUP", screen: "F04.S01", state: "bước 4", action: "chạm đúp «Ghi vào sổ»", cauHinh: "C1", expected: "một khoản chi", status: "BLOCKED", evidence: [], ghiChu: "không có MOBILE_DATABASE_URL để đếm khoản chi" });
    } else if (truoc > 0) {
      console.log(`bỏ qua ghi: nhóm chat-test đã có ${truoc} khoản chi (lượt trước đã ghi)`);
    } else {
      const t = await mo("C1", P20, duongBill(G20));
      const cdp = await cdpCua(t.page);
      await nhapBill(t, cdp);
      await sangGanMon(t, cdp);
      await sangKetQua(t, cdp);
      await go(t.page, "Ô tên khoản chi", "Tất niên nhóm kiểm thử");
      await chup(t.page, { out: mt.out, id: "EV-F04-KET-QUA-20-C1", suKien: t.suKien, fullPage: true });
      const n = await timNut(t.page, "Ghi vào sổ");
      await cham(cdp, n);
      await t.page.waitForTimeout(70);
      await cham(cdp, n);
      await t.page.waitForFunction(() => /Bước 5/.test(document.querySelector('[role="progressbar"]')?.getAttribute("aria-label") ?? ""), null, { timeout: 15_000 }).catch(() => undefined);
      // The Nếp moment and the seal: frames of the Nếp slot every 150 ms.
      const khung = [];
      for (let i = 0; i < 12; i += 1) {
        const h = await t.page.evaluate(() => {
          const e = document.querySelector('[role="img"][aria-label="Nếp đóng dấu đã ghi sổ"]');
          const d = document.querySelector('[aria-label="Đã ghi sổ"]');
          return { nep: !!e, dau: d ? getComputedStyle(d).transform : null, dauOp: d ? getComputedStyle(d).opacity : null };
        });
        khung.push(h);
        await t.page.waitForTimeout(150);
      }
      await t.page.waitForTimeout(600);
      const sau = soKhoanChi(G20);
      await chup(t.page, { out: mt.out, id: "EV-F04-DA-GHI-C1", suKien: t.suKien, fullPage: true });
      const nep = await nepGanTien(t.page);
      log({ truoc, sau, nep, khung: khung.map((k) => `${k.nep ? "N" : "-"} ${k.dauOp}`) });
      ket({ tc: "TC-F04-GHI-CHAM-DUP", screen: "F04.S01", state: "bước 4, 20 cuống, tổng 13.705.678đ", action: "chạm «Ghi vào sổ» hai lần cách 70 ms", cauHinh: "C1", expected: "đúng một khoản chi được ghi; sang bước 5", status: sau - truoc === 1 ? "PASS" : "FAIL", evidence: ["EV-F04-KET-QUA-20-C1", "EV-F04-DA-GHI-C1"], ghiChu: `khoản chi của nhóm: ${truoc} → ${sau}` });
      ket({ tc: "TC-F04-NEP-M3", screen: "F04.S01", state: "bước 5 «Đã ghi sổ»", action: "sau khi ghi", cauHinh: "C1", expected: "Nếp M3 và dấu «Đã ghi sổ» cùng nhịp; Nếp cách số tiền ≥16dp", status: nep.ganNhat && nep.ganNhat.cach >= 16 ? "PASS" : "FAIL", evidence: ["EV-F04-DA-GHI-C1"], ghiChu: `Nếp gần tiền nhất: ${nep.ganNhat ? `${nep.ganNhat.cach}dp tới ${nep.ganNhat.tien}` : "không có Nếp"}; khung: ${khung.map((k) => (k.nep ? "N" : "-") + (k.dauOp ?? "x")).join(" ")}` });
      await t.context.close();
    }
  }

  // ------------------------------- M3 and M4 under Reduce Motion (C9), in pixels
  if (chay("c9")) {
    const { createHash } = await import("node:crypto");
    /** The Nếp slot every 150 ms: pixel hash, renderer, and the seal's opacity when there is one. */
    const layMau = async (page, tenNep, nhanDau) => {
      const t0 = Date.now();
      let hop = null;
      while (!hop && Date.now() - t0 < 8000) {
        hop = await page.evaluate((ten) => {
          const e = document.querySelector(`[role="img"][aria-label="${ten}"]`);
          if (!e) return null;
          const r = e.getBoundingClientRect();
          return { x: Math.max(0, r.left - 4), y: Math.max(0, r.top - 4), width: r.width + 8, height: r.height + 8 };
        }, tenNep);
        if (!hop) await page.waitForTimeout(40);
      }
      const mau = [];
      if (hop) {
        for (let i = 0; i < 14; i += 1) {
          const buf = await page.screenshot({ clip: hop });
          const phu = await page.evaluate(({ ten, dau }) => {
            const e = document.querySelector(`[role="img"][aria-label="${ten}"]`);
            const d = dau ? document.querySelector(`[aria-label="${dau}"]`) : null;
            return { ve: [...(e?.querySelectorAll("[data-renderer]") ?? [])].map((x) => x.getAttribute("data-renderer")).join("+") || "-", dau: d ? `${getComputedStyle(d).opacity}` : null };
          }, { ten: tenNep, dau: nhanDau });
          mau.push({ ms: Date.now() - t0, h: createHash("sha1").update(buf).digest("hex").slice(0, 6), buf, ...phu });
          await page.waitForTimeout(150);
        }
      }
      return { hop, mau, khac: new Set(mau.map((m) => m.h)).size, doiCuoi: mau.reduce((m, b, i) => (i > 0 && b.h !== mau[i - 1].h ? b.ms : m), 0) };
    };
    /** First and last image of the Nếp slot side by side, for looking at the pose. */
    const ghepDauCuoi = async (kq, id, tieuDe) => {
      if (!kq.mau.length) return null;
      const { writeFileSync } = await import("node:fs");
      const dau = join(mt.out, "raw", `${id}-dau.png`);
      const cuoi = join(mt.out, "raw", `${id}-cuoi.png`);
      writeFileSync(dau, kq.mau[0].buf);
      writeFileSync(cuoi, kq.mau[kq.mau.length - 1].buf);
      await ghepAnh(mt.browser, [{ file: dau, nhan: `${kq.mau[0].ms} ms · ${kq.mau[0].ve}` }, { file: cuoi, nhan: `${kq.mau[kq.mau.length - 1].ms} ms · ${kq.mau[kq.mau.length - 1].ve}` }], join(mt.out, "jpg", `${id}.jpg`), { cao: 300, tieuDe });
      return id;
    };
    const truoc = soKhoanChi(G20);
    // Up to two small expenses: the first run sampled hashes only, the second
    // keeps the first and last image of the slot so the pose can be looked at.
    if (truoc === null || truoc >= 3) {
      console.log(`bỏ qua M3 ở C9: ${truoc === null ? "không có MOBILE_DATABASE_URL" : "đã có khoản thứ hai (lượt trước)"}`);
    } else {
      const t = await mo("C9", P20, duongBill(G20));
      const cdp = await cdpCua(t.page);
      await nhapBill(t, cdp, [["Trà đá", "20", "20000"]]);
      await sangGanMon(t, cdp);
      await sangKetQua(t, cdp);
      await go(t.page, "Ô tên khoản chi", "Trà đá kiểm thử giảm chuyển động");
      await cham(cdp, await timNut(t.page, "Ghi vào sổ"));
      const kq = await layMau(t.page, "Nếp đóng dấu đã ghi sổ", "Đã ghi sổ");
      await ghepDauCuoi(kq, "EV-F04-M3-C9-dau-cuoi", "M3 ở C9: ảnh vùng Nếp đầu và cuối");
      await chup(t.page, { out: mt.out, id: "EV-F04-M3-C9", suKien: t.suKien });
      log({ m3: kq.mau.map((m) => `${m.ms}:${m.h}/${m.ve}/${m.dau}`).join(" ") });
      const dauDau = kq.mau[0]?.dau;
      ket({ tc: "TC-MO13-M3", screen: "F04.S01", state: "bước 5 «Đã ghi sổ», giảm chuyển động", action: "«Ghi vào sổ» (khoản nhỏ thứ hai của nhóm chat-test)", cauHinh: "C9", expected: "Nếp và dấu hiện ngay ở khung cuối, đứng yên: một ảnh vùng Nếp, dấu đủ đậm từ mẫu đầu (ADR-0037 D3)", status: kq.hop && kq.khac === 1 && dauDau === "1" ? "PASS" : "FAIL", evidence: ["EV-F04-M3-C9", "EV-F04-M3-C9-dau-cuoi"], ghiChu: kq.hop ? `14 ảnh vùng Nếp mỗi 150 ms: ${kq.khac} ảnh khác nhau, đổi lần cuối ở ${kq.doiCuoi} ms; độ đậm dấu: ${kq.mau.map((m) => m.dau).join(" ")}; vẽ: ${[...new Set(kq.mau.map((m) => m.ve))].join(", ")}` : "không thấy Nếp M3 trong 8 s" });
      await t.context.close();
    }
    // M4: one more arrival in the published round (skip with --khong-m4).
    const ds = process.argv.includes("--khong-m4") ? [] : await dotCua(phien20, G20);
    const dot = ds.find((d) => (d.status ?? "") !== "frozen") ?? ds[0];
    if (dot) {
      const t = await mo("C9", P20, `/batches/${dot.batch_id ?? dot.id}?ctx=${G20}`);
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(800);
      const n = await t.page.evaluate(() => {
        const e = [...document.querySelectorAll('[role="button"]')].find((x) => /^Tiền đã về từ /.test((x.innerText ?? "").replace(/[-]/g, "").trim()));
        if (!e) return null;
        e.scrollIntoView({ block: "center" });
        for (let p = e.parentElement; p; p = p.parentElement) if (p.scrollLeft && !/(auto|scroll)/.test(getComputedStyle(p).overflowX)) p.scrollLeft = 0;
        const r = e.getBoundingClientRect();
        return { x: r.left + r.width / 2, y: r.top + r.height / 2, ten: (e.innerText ?? "").replace(/[-]/g, "").trim() };
      });
      if (n) {
        await cham(cdp, n);
        await t.page.waitForFunction(() => !!document.querySelector('[role="img"][aria-label="Nếp cúi đầu cảm ơn"]'), null, { timeout: 8000 }).catch(() => undefined);
        await t.page.evaluate(() => scrollTo(0, 0));
        const kq = await layMau(t.page, "Nếp cúi đầu cảm ơn", null);
        await ghepDauCuoi(kq, "EV-F04-M4-C9-dau-cuoi", "M4 ở C9: ảnh vùng Nếp đầu và cuối");
        await chup(t.page, { out: mt.out, id: "EV-F04-M4-C9", suKien: t.suKien });
        log({ m4: kq.mau.map((m) => `${m.ms}:${m.h}/${m.ve}`).join(" ") });
        ket({ tc: "TC-MO13-M4", screen: "F04.S04", state: "đợt đã phát, giảm chuyển động", action: `«${n.ten}»`, cauHinh: "C9", expected: "Nếp cúi chào hiện ngay ở khung cuối và đứng yên (một ảnh)", status: kq.hop && kq.khac === 1 ? "PASS" : "FAIL", evidence: ["EV-F04-M4-C9", "EV-F04-M4-C9-dau-cuoi"], ghiChu: kq.hop ? `14 ảnh vùng Nếp mỗi 150 ms: ${kq.khac} ảnh khác nhau, đổi lần cuối ở ${kq.doiCuoi} ms; vẽ: ${[...new Set(kq.mau.map((m) => m.ve))].join(", ")}` : "không thấy Nếp M4 trong 8 s" });
      }
      await t.context.close();
    }
  }

  // ----------------------------------------------- the settlement of 20 people
  if (chay("qt")) {
    for (const cfg of ["C1", "C2"]) {
      const t = await mo(cfg, P20, `/settlements/${G20}`);
      await t.page.waitForTimeout(800);
      await chup(t.page, { out: mt.out, id: `EV-F04-QT-20-${cfg}`, suKien: t.suKien, fullPage: true });
      const tien = await tienTrenMan(t.page);
      const cat = tien.filter((x) => x.elip || x.catBoi);
      const hinh = await t.page.evaluate(() => {
        const s = document.querySelector('[data-testid="so-do-chuyen"]');
        if (!s) return null;
        const r = s.getBoundingClientRect();
        const ten = [...s.querySelectorAll("*")].filter((e) => [...e.childNodes].some((n) => n.nodeType === 3 && n.textContent.trim())).map((e) => e.getBoundingClientRect());
        let de = 0;
        for (let i = 0; i < ten.length; i += 1) for (let j = i + 1; j < ten.length; j += 1) if (ten[i].left < ten[j].right && ten[j].left < ten[i].right && ten[i].top < ten[j].bottom && ten[j].top < ten[i].bottom) de += 1;
        return { cao: Math.round(r.height), soTen: ten.length, tenDeNhau: de };
      });
      log({ cfg, soTien: tien.length, cat: cat.slice(0, 4), hinh });
      ket({ tc: "TC-F04-QT-20", screen: "F04.S03", state: "nhóm 20 người, một khoản 13.705.678đ", action: "mở /settlements/[id]", cauHinh: cfg, expected: "sơ đồ đọc được (tên không đè nhau), số tiền không bị cắt", status: cat.length === 0 && hinh && hinh.tenDeNhau === 0 ? "PASS" : "FAIL", evidence: [`EV-F04-QT-20-${cfg}`], ghiChu: `${tien.length} số tiền, cắt ${cat.length}; sơ đồ cao ${hinh?.cao}px, ${hinh?.soTen} nhãn, ${hinh?.tenDeNhau} cặp nhãn đè nhau` });
      await t.context.close();
    }
  }

  // ------------------------------------- a round with nothing new to collect (G8)
  if (chay("dot-rong")) {
    const truoc = soDot(G8);
    const t = await mo("C1", P8, `/settlements/${G8}`);
    const cdp = await cdpCua(t.page);
    const nut = await bam(t, cdp, "Tạo đợt thu từ sổ", { cho: 2500 });
    const cau = await viTriCau(t.page, /Sổ chưa có khoản nào để thu|Không mở được đợt thu|Kiểm tra mạng/);
    const duong = await duongDan(t.page);
    await chup(t.page, { out: mt.out, id: "EV-F04-DOT-RONG-C1", suKien: t.suKien });
    const sau = soDot(G8);
    log({ truoc, sau, cau, duong, nut: nut?.top });
    ket({ tc: "TC-F04-DOT-RONG", screen: "F04.S03", state: "Team Đà Lạt: mọi khoản đã vào đợt đã phát, 7 khoản chuyển chưa về", action: "chạm «Tạo đợt thu từ sổ»", cauHinh: "C1", expected: "không mời một việc chắc chắn bị từ chối; nếu vẫn chạm được thì câu nói đúng lý do, cạnh nút", status: "FAIL", evidence: ["EV-F04-DOT-RONG-C1"], ghiChu: `nút hiện vì danh sách chuyển còn 7 dòng; máy chủ từ chối, số đợt ${truoc} → ${sau}; câu: «${cau?.text ?? "không có"}» ${cau?.trongMan ? "thấy" : "ngoài màn"}; ở ${duong.split("?")[0]}` });
    await t.context.close();
  }

  // --------------------------------------- a round in the chat-test group, publish
  if (chay("dot")) {
    let dsDot = await dotCua(phien20, G20);
    const t = await mo("C1", P20, `/settlements/${G20}`);
    const cdp = await cdpCua(t.page);
    if (dsDot.length === 0) {
      await bam(t, cdp, "Tạo đợt thu từ sổ", { cho: 2500 });
      await choOn(t.page, { mang: t.mang });
    } else {
      await t.page.goto(new URL(`/batches/${dsDot[0].batch_id ?? dsDot[0].id}?ctx=${G20}`, mt.base).toString());
      await choOn(t.page, { mang: t.mang });
    }
    await t.page.waitForTimeout(1200);
    const duong = await duongDan(t.page);
    const chuaPhat = await coChu(t.page, /Chưa phát: chưa ai bị nhắn gì/);
    await chup(t.page, { out: mt.out, id: "EV-F04-DOT-MOI-C1", suKien: t.suKien, fullPage: true });
    if (chuaPhat) {
      await bam(t, cdp, "Phát đợt thu", { cho: 700 });
      const hop = await coChu(t.page, /Phát đợt thu này\?/);
      await chup(t.page, { out: mt.out, id: "EV-F04-PHAT-HOI-C1", suKien: t.suKien });
      await bam(t, cdp, "Thôi, chưa phát", { cho: 600 });
      const lai = { hop: await coChu(t.page, /Phát đợt thu này\?/), nut: !!(await timNut(t.page, "Phát đợt thu")) };
      ket({ tc: "TC-F04-PHAT-HAI-BUOC", screen: "F04.S04", state: "đợt mới, chưa phát", action: "«Phát đợt thu», rồi «Thôi, chưa phát»", cauHinh: "C1", expected: "lần chạm đầu chỉ mở câu hỏi nói rõ không hoàn lại; «Thôi» đóng lại, chưa phát", status: hop && !lai.hop && lai.nut ? "PASS" : "FAIL", evidence: ["EV-F04-PHAT-HOI-C1"], ghiChu: `hộp hỏi ${hop ? "hiện" : "không"}; sau «Thôi»: hộp ${lai.hop ? "còn" : "đóng"}, nút «Phát đợt thu» ${lai.nut ? "còn" : "mất"}` });
      await bam(t, cdp, "Phát đợt thu", { cho: 700 });
      await bam(t, cdp, "Phát, không hoàn lại", { cho: 3500 });
    }
    dsDot = await dotCua(phien20, G20);
    const daPhat = await coChu(t.page, /Đã phát: mỗi người xem phần/);
    const soThu = await t.page.evaluate(() => [...document.querySelectorAll('[role="button"]')].filter((e) => /^Gửi (lại )?cho /.test((e.innerText ?? "").replace(/[-]/g, "").trim())).length);
    await chup(t.page, { out: mt.out, id: "EV-F04-DA-PHAT-C1", suKien: t.suKien, fullPage: true });
    log({ duong, chuaPhat, daPhat, soThu, dot: dsDot.map((d) => d.status ?? d.trang_thai) });
    ket({ tc: "TC-F04-PHAT", screen: "F04.S04", state: "đợt của nhóm 20 người", action: "«Phát đợt thu» → «Phát, không hoàn lại»", cauHinh: "C1", expected: "đã phát; phong bì có một link cho mỗi người nợ", status: daPhat && soThu > 0 ? "PASS" : "FAIL", evidence: ["EV-F04-DOT-MOI-C1", "EV-F04-DA-PHAT-C1"], ghiChu: `đường ${duong.split("?")[0]}; ${soThu} nút «Gửi cho …»` });

    if (chay("chia-se") || !chi) {
      // Share on the web, three browsers' worth: none, one that shares, one the person dismisses.
      const nutGui = async () => t.page.evaluate(() => {
        const e = [...document.querySelectorAll('[role="button"]')].find((x) => /^Gửi (lại )?cho /.test((x.innerText ?? "").replace(/[-]/g, "").trim()));
        if (!e) return null;
        e.scrollIntoView({ block: "center" });
        for (let n = e.parentElement; n; n = n.parentElement) if (n.scrollLeft && !/(auto|scroll)/.test(getComputedStyle(n).overflowX)) n.scrollLeft = 0;
        const r = e.getBoundingClientRect();
        return { x: r.left + r.width / 2, y: r.top + r.height / 2, ten: (e.innerText ?? "").replace(/[-]/g, "").trim() };
      });
      const doc = async () => ({ cau: await viTriCau(t.page, /Không kết nối được Rủ Đi|Kiểm tra mạng|chưa rõ đã gửi|Share is not supported/), daMo: await coChu(t.page, /Đã mở khay chia sẻ/) });
      const kq = {};
      const n0 = await nutGui();
      await t.page.evaluate(() => { try { delete navigator.share; } catch {} });
      if (n0) await cham(cdp, n0);
      await t.page.waitForTimeout(800);
      kq.khongCo = { ...(await doc()), coShare: await t.page.evaluate(() => typeof navigator.share) };
      await chup(t.page, { out: mt.out, id: "EV-F04-CHIA-SE-KHONG-CO-C1", suKien: t.suKien });
      // A browser that has navigator.share and shares (Chrome on Android resolves with undefined).
      await t.page.evaluate(() => Object.defineProperty(navigator, "share", { configurable: true, value: async () => undefined }));
      const n1 = await nutGui();
      if (n1) await cham(cdp, n1);
      await t.page.waitForTimeout(800);
      kq.chiaDuoc = await doc();
      await chup(t.page, { out: mt.out, id: "EV-F04-CHIA-SE-DUOC-C1", suKien: t.suKien });
      // The person closes the share sheet (AbortError).
      await t.page.evaluate(() => Object.defineProperty(navigator, "share", { configurable: true, value: async () => { throw new DOMException("Share canceled", "AbortError"); } }));
      const n2 = await nutGui();
      if (n2) await cham(cdp, n2);
      await t.page.waitForTimeout(800);
      kq.dong = await doc();
      log({ chiaSe: kq, nut: n0?.ten });
      const f = (x) => `${x.cau ? `«${x.cau.text.slice(0, 80)}» y ${x.cau.top}${x.cau.trongMan ? "" : " (ngoài màn)"}` : "không câu"}${x.daMo ? ", hàng ghi «Đã mở khay chia sẻ»" : ""}`;
      ket({ tc: "TC-L32-VONGDOI", screen: "F04.S04", layer: "L32", state: "đợt đã phát, phong bì link trên máy này", action: `«${n0?.ten ?? "Gửi cho …"}» trên web: (a) trình duyệt không có navigator.share, (b) có và chia sẻ xong, (c) người dùng đóng khay`, cauHinh: "C1", expected: "(a) có lối khác (chép link) hoặc câu nói đúng là trình duyệt không chia sẻ được; (b) hàng ghi «Đã mở khay chia sẻ»; (c) không báo lỗi", status: kq.chiaDuoc.daMo && !kq.chiaDuoc.cau && !kq.dong.cau && kq.khongCo.cau && !/mạng/.test(kq.khongCo.cau.text) ? "PASS" : "FAIL", evidence: ["EV-F04-CHIA-SE-KHONG-CO-C1", "EV-F04-CHIA-SE-DUOC-C1"], ghiChu: `(a) ${f(kq.khongCo)}; (b) ${f(kq.chiaDuoc)}; (c) ${f(kq.dong)}` });
    }

    if (chay("tien-ve") || !chi) {
      await t.page.evaluate(() => scrollTo(0, 0));
      const n = await t.page.evaluate(() => {
        const e = [...document.querySelectorAll('[role="button"]')].find((x) => /^Tiền đã về từ /.test((x.innerText ?? "").replace(/[-]/g, "").trim()));
        if (!e) return null;
        e.scrollIntoView({ block: "center" });
        for (let n = e.parentElement; n; n = n.parentElement) if (n.scrollLeft && !/(auto|scroll)/.test(getComputedStyle(n).overflowX)) n.scrollLeft = 0;
        const r = e.getBoundingClientRect();
        return { x: r.left + r.width / 2, y: r.top + r.height / 2, ten: (e.innerText ?? "").replace(/[-]/g, "").trim() };
      });
      const demTruoc = await t.page.evaluate(() => document.body.innerText.match(/(\d+)\/(\d+) lượt chuyển đã về/)?.[0] ?? null);
      if (n) await cham(cdp, n);
      await t.page.waitForTimeout(2500);
      const demSau = await t.page.evaluate(() => document.body.innerText.match(/(\d+)\/(\d+) lượt chuyển đã về/)?.[0] ?? null);
      await t.page.evaluate(() => scrollTo(0, 0));
      await t.page.waitForTimeout(300);
      const nep = await nepGanTien(t.page);
      const coM4 = await t.page.evaluate(() => !!document.querySelector('[role="img"][aria-label="Nếp cúi đầu cảm ơn"]'));
      await chup(t.page, { out: mt.out, id: "EV-F04-TIEN-VE-C1", suKien: t.suKien });
      log({ n: n?.ten, demTruoc, demSau, nep, coM4 });
      ket({ tc: "TC-F04-TIEN-VE", screen: "F04.S04", state: "đợt đã phát, người nhận là người xem", action: `«${n?.ten ?? "Tiền đã về từ …"}»`, cauHinh: "C1", expected: "đếm tăng 1 theo máy chủ; Nếp M4 ở đầu trang, cách số tiền ≥16dp", status: demTruoc !== demSau && coM4 && (!nep.ganNhat || nep.ganNhat.cach >= 16) ? "PASS" : "FAIL", evidence: ["EV-F04-TIEN-VE-C1"], ghiChu: `${demTruoc} → ${demSau}; Nếp M4 ${coM4 ? "có" : "không"}; gần tiền nhất ${nep.ganNhat ? `${nep.ganNhat.cach}dp (${nep.ganNhat.nep})` : "-"}` });
    }
    await t.context.close();
  }

  // ------------------------------------------------------ failures and retries
  if (chay("loi")) {
    const ca = [
      { tc: "TC-F04.S03-503", screen: "F04.S03", path: `/settlements/${G8}`, mau: /\/contexts\/[^/]+\/balances/, cau: /Chưa đọc được sổ/, persona: P8 },
      { tc: "TC-F04.S04-503", screen: "F04.S04", path: `/batches/${B8}?ctx=${G8}`, mau: /\/batches\/[^/]+\/obligations/, cau: /Chưa đọc được bảng thu/, persona: P8 },
      { tc: "TC-F04.S05-503", screen: "F04.S05", path: "/finance", mau: /\/people\/[^/]+\/finance/, cau: /Chưa đọc được sổ/, persona: P8 },
    ];
    for (const c of ca) {
      const t = await mo("C1", c.persona, "/plan");
      await loiMayChu(t.page, c.mau);
      await t.page.goto(new URL(c.path, mt.base).toString());
      await choOn(t.page, { mang: t.mang });
      await t.page.waitForTimeout(900);
      const man = await t.page.evaluate(() => document.body.innerText.replace(/\s+/g, " ").slice(0, 260));
      const id = `EV-${c.tc}-C1`;
      await chup(t.page, { out: mt.out, id, suKien: t.suKien });
      await goHet(t.page);
      const cdp = await cdpCua(t.page);
      await bam(t, cdp, "Thử lại", { cho: 1800 });
      const hoi = !(await coChu(t.page, c.cau));
      log({ tc: c.tc, man, hoi });
      ket({ tc: c.tc, screen: c.screen, state: "máy chủ trả 503", action: `mở ${c.path.split("?")[0].replace(/[0-9a-f-]{36}/, "[id]")}, rồi «Thử lại» khi máy chủ đã ổn`, cauHinh: "C1", expected: "câu tiếng Việt nói đúng là máy chủ lỗi, không nói mạng; «Thử lại» tải lại được", status: hoi && !/Kiểm tra mạng|audit_injected/.test(man) ? "PASS" : "FAIL", evidence: [id], ghiChu: `màn: «${man.slice(0, 200)}»; thử lại ${hoi ? "được" : "không được"}` });
      await t.context.close();
    }
    // Offline on the finance page.
    const t = await mo("C1", P8, "/plan");
    await ngatMang(t.context, true);
    await t.page.evaluate(() => { history.pushState({}, "", "/finance"); dispatchEvent(new PopStateEvent("popstate")); });
    await t.page.waitForTimeout(2500);
    const man = await t.page.evaluate(() => document.body.innerText.replace(/\s+/g, " ").slice(0, 240));
    await chup(t.page, { out: mt.out, id: "EV-F04.S05-OFFLINE-C1", suKien: t.suKien });
    await ngatMang(t.context, false);
    ket({ tc: "TC-F04.S05-OFFLINE", screen: "F04.S05", state: "mất mạng", action: "mở Tài chính từ trong app", cauHinh: "C1", expected: "câu nói không kết nối được, có «Thử lại»", status: /Không kết nối được/.test(man) && /Thử lại/.test(man) ? "PASS" : "FAIL", evidence: ["EV-F04.S05-OFFLINE-C1"], ghiChu: `màn: «${man.slice(0, 180)}»` });
    await t.context.close();
    // The bill: POST /bills refused at step 2 → 3.
    const t2 = await mo("C1", P8, duongBill(null));
    const cdp2 = await cdpCua(t2.page);
    await nhapBill(t2, cdp2);
    await loiMayChu(t2.page, /\/bills(\?|$)/);
    await t2.page.evaluate(() => scrollTo(0, document.scrollingElement.scrollHeight));
    await bam(t2, cdp2, "Tiếp: ai dùng món nào?", { cho: 2500 });
    const buoc = await buocHienTai(t2.page);
    const cau = await t2.page.evaluate(() => [...document.querySelectorAll('[aria-live="polite"]')].map((e) => { const r = e.getBoundingClientRect(); return { text: e.textContent.trim(), top: Math.round(r.top), trongMan: r.bottom > 0 && r.top < innerHeight }; }).filter((x) => x.text));
    const giu = await oGiaTri(t2.page);
    await chup(t2.page, { out: mt.out, id: "EV-F04-HOA-DON-503-C1", suKien: t2.suKien });
    await goHet(t2.page);
    await bam(t2, cdp2, "Tiếp: ai dùng món nào?", { cho: 400 });
    await t2.page.waitForFunction(() => /Bước 3/.test(document.querySelector('[role="progressbar"]')?.getAttribute("aria-label") ?? ""), null, { timeout: 12_000 }).catch(() => undefined);
    const sauThu = await buocHienTai(t2.page);
    log({ buoc, cau, giu: giu.length, sauThu });
    ket({ tc: "TC-F04-BILL-503", screen: "F04.S01", state: "bước 2, máy chủ trả 503 khi tạo bill", action: "chạm «Tiếp» ở cuối trang, rồi chạm lại khi máy chủ ổn", cauHinh: "C1", expected: "ở lại bước 2, bill còn nguyên; câu lỗi thấy được và không đổ cho mạng; lần sau đi tiếp được", status: /Bước 2/.test(buoc ?? "") && cau.some((x) => x.trongMan && !/mạng/.test(x.text)) && /Bước 3/.test(sauThu ?? "") ? "PASS" : "FAIL", evidence: ["EV-F04-HOA-DON-503-C1"], ghiChu: `${buoc}; câu: ${cau.map((x) => `«${x.text.slice(0, 90)}» y ${x.top}${x.trongMan ? "" : " (ngoài màn)"}`).join("; ") || "không có"}; lần sau: ${sauThu}` });
    await t2.context.close();
  }

  // --------------------------------------------- cold open, the in-screen back
  if (chay("lanh")) {
    for (const [tc, screen, path] of [["TC-F04.S03-BACK-LANH", "F04.S03", `/settlements/${G8}`], ["TC-F04.S04-BACK-LANH", "F04.S04", `/batches/${B8}?ctx=${G8}`], ["TC-F04.S05-BACK-LANH", "F04.S05", "/finance"], ["TC-F04.S01-BACK-LANH", "F04.S01", duongBill(null)]]) {
      const t = await mo("C1", P8, path);
      const cdp = await cdpCua(t.page);
      const truoc = await duongDan(t.page);
      await bam(t, cdp, "Quay lại", { cho: 1200 });
      const sau = await duongDan(t.page);
      ket({ tc, screen, state: "mở thẳng bằng link (không có lịch sử trong app)", action: "chạm «Quay lại» của đầu màn", cauHinh: "C1", expected: "về một màn của app (tab gốc)", status: sau !== truoc ? "PASS" : "FAIL", issue: sau !== truoc ? null : "UI-018", evidence: [], ghiChu: `${truoc.split("?")[0].replace(/[0-9a-f-]{36}/, "[id]")} → ${sau.split("?")[0].replace(/[0-9a-f-]{36}/, "[id]")}` });
      await t.context.close();
    }
  }

  // ---------------------------------------------- the Nếp edge on money screens
  if (chay("nep")) {
    const t = await mo("C1", P8, `/settlements/${G8}`);
    const cdp = await cdpCua(t.page);
    const mep = await t.page.evaluate(() => {
      const e = document.querySelector('[data-testid="nep-mep"]');
      if (!e) return null;
      const r = e.getBoundingClientRect();
      return { x: r.left + r.width / 2, y: r.top + r.height / 2, w: Math.round(r.width), h: Math.round(r.height), nhan: e.getAttribute("aria-label"), vai: e.getAttribute("role"), tab: e.getAttribute("tabindex") };
    });
    if (mep) await cham(cdp, mep);
    await t.page.waitForTimeout(900);
    const sau = await t.page.evaluate(() => ({ mep: !!document.querySelector('[data-testid="nep-mep"]'), dia: !!document.querySelector('[data-testid="nep-dia"]'), bang: document.querySelectorAll('[role="dialog"]').length }));
    // Keyboard too.
    await t.page.evaluate(() => document.querySelector('[data-testid="nep-mep"]')?.focus());
    await t.page.keyboard.press("Enter");
    await t.page.waitForTimeout(700);
    const sauPhim = await t.page.evaluate(() => ({ dia: !!document.querySelector('[data-testid="nep-dia"]'), bang: document.querySelectorAll('[role="dialog"]').length }));
    await chup(t.page, { out: mt.out, id: "EV-F04-NEP-MEP-C1", suKien: t.suKien, chuThich: [{ selector: '[data-testid="nep-mep"]', nhan: "đã chạm" }] });
    log({ mep, sau, sauPhim });
    ket({ tc: "TC-F04-NEP-MEP", screen: "F04.S03", state: "màn tiền: Nếp phải lùi (MAN_NEP_LUI)", action: "chạm mép Nếp, rồi Enter khi mép có focus", cauHinh: "C1", expected: "Nếp không ra cạnh số tiền (luật); nhưng mép không hứa một việc nó không làm: nhãn và vai trò khớp với việc chạm", status: mep && !sau.dia && sau.bang === 0 && !/chạm để kéo ra/.test(mep.nhan ?? "") ? "PASS" : "FAIL", evidence: ["EV-F04-NEP-MEP-C1"], ghiChu: `mép ${mep?.w}×${mep?.h}, role ${mep?.vai}, tabindex ${mep?.tab}, nhãn «${mep?.nhan}»; sau chạm: đĩa ${sau.dia ? "ra" : "không ra"}, bảng ${sau.bang}; sau Enter: đĩa ${sauPhim.dia ? "ra" : "không"}, bảng ${sauPhim.bang}` });
    await t.context.close();
    for (const [cfg, path, screen] of [["C1", `/settlements/${G8}`, "F04.S03"], ["C2", `/settlements/${G8}`, "F04.S03"], ["C6", `/settlements/${G8}`, "F04.S03"], ["C1", `/batches/${B8}?ctx=${G8}`, "F04.S04"], ["C2", `/batches/${B8}?ctx=${G8}`, "F04.S04"], ["C2", "/finance", "F04.S05"]]) {
      const t2 = await mo(cfg, P8, path);
      const d = await quetNepTien(t2.page);
      ket({ tc: `TC-${screen}-NEP-TIEN`, screen, state: "cuộn từ đầu tới cuối trang", action: "đo khoảng cách mép Nếp tới số tiền gần nhất ở mỗi nấc cuộn", cauHinh: cfg, expected: "ghi nhận (luật ≥16dp của DESIGN nói về Nếp trong trang; mép dock nằm trong lề 16dp)", status: "PASS", evidence: [], ghiChu: `nhỏ nhất ${d ? `${d.cach}dp tới ${d.tien} ở cuộn ${d.cuon}` : "không có cặp nào cùng lúc trên màn"}` });
      await t2.context.close();
    }
  }

  // ----------------------------------------------------------- low window C8
  if (chay("c8")) {
    const t = await mo("C8", P8, duongBill(null));
    const cdp = await cdpCua(t.page);
    await bam(t, cdp, "Nhập tay", { cho: 900 });
    const o = t.page.locator('[aria-label="Ô tiền món 1"]');
    await o.click();
    await t.page.waitForTimeout(400);
    const vt = await t.page.evaluate(() => {
      const a = document.activeElement.getBoundingClientRect();
      const n = [...document.querySelectorAll('[role="button"]')].find((x) => /Tiếp: ai dùng món nào/.test(x.innerText ?? ""))?.getBoundingClientRect();
      return { o: [Math.round(a.top), Math.round(a.bottom)], nut: n ? [Math.round(n.top), Math.round(n.bottom)] : null, cao: innerHeight, cheO: n ? a.bottom > n.top && a.top < n.bottom : null };
    });
    await chup(t.page, { out: mt.out, id: "EV-F04-C8-BUOC2", suKien: t.suKien });
    log({ vt });
    ket({ tc: "TC-F04-C8-BUOC2", screen: "F04.S01", state: "bước 2, cửa sổ 390×460", action: "chạm ô «Thành tiền»", cauHinh: "C8", expected: "ô đang gõ và nút «Tiếp» cùng thấy, nút không đè ô", status: vt.nut && !vt.cheO && vt.o[1] <= vt.cao && vt.o[0] >= 0 ? "PASS" : "FAIL", evidence: ["EV-F04-C8-BUOC2"], ghiChu: `ô y ${vt.o.join("–")}, nút y ${vt.nut?.join("–")}, cửa sổ ${vt.cao}` });
    await t.context.close();
  }

  // ----------------------------------------------------------- tablet, 2 panes
  if (chay("tablet")) {
    for (const cfg of ["C6", "C7"]) {
      const t = await mo(cfg, P8, duongBill(null));
      const cdp = await cdpCua(t.page);
      await nhapBill(t, cdp);
      await sangGanMon(t, cdp);
      await chup(t.page, { out: mt.out, id: `EV-F04-TABLET-GAN-${cfg}`, suKien: t.suKien });
      const aiCoGi = await t.page.evaluate(() => /Ai có gì/i.test(document.body.innerText));
      await sangKetQua(t, cdp);
      await chup(t.page, { out: mt.out, id: `EV-F04-TABLET-KQ-${cfg}`, suKien: t.suKien });
      const tien = await tienTrenMan(t.page);
      const cat = tien.filter((x) => x.elip || x.catBoi);
      ket({ tc: "TC-F04-TABLET", screen: "F04.S01", state: "bước 3 và 4 ở cửa sổ rộng", action: "đi hết bước 3, 4", cauHinh: cfg, expected: "hai trang: «Ai có gì» ở bước 3, người trả và tên khoản ở bước 4; không cắt số", status: cat.length === 0 ? "PASS" : "FAIL", evidence: [`EV-F04-TABLET-GAN-${cfg}`, `EV-F04-TABLET-KQ-${cfg}`], ghiChu: `«Ai có gì» ${aiCoGi ? "có" : "không"}; số tiền cắt ${cat.length}` });
      await t.context.close();
    }
  }

  // ----------------------------------------------- demo assignment (signed out)
  if (chay("demo")) {
    const t = await mo("C1", P8, "/smart-split/xom-leo/assignment");
    await t.page.waitForTimeout(900);
    const duong = await duongDan(t.page);
    ket({ tc: "TC-F04.S02-CO-PHIEN", screen: "F04.S02", state: "đã đăng nhập", action: "mở /smart-split/[id]/assignment", cauHinh: "C1", expected: "chuyển về luồng chia bill thật (review)", status: /\/smart-split\/moi\/review/.test(duong) ? "PASS" : "FAIL", evidence: [], ghiChu: `đường cuối ${duong}` });
    await t.context.close();
  }

  // ------------------------------ MO15 the settlement's ink arrows drawing in
  if (chay("mo15")) {
    for (const cfg of ["C1", "C9"]) {
      const t = await mo(cfg, P8, "/plan");
      // Every frame: the first arrow's dash offset and whether the diagram exists yet.
      await t.page.evaluate(() => {
        window.__muc = [];
        const t0 = performance.now();
        const buoc = () => {
          const p = document.querySelector('[data-testid="so-do-chuyen"] svg path');
          window.__muc.push({ t: Math.round(performance.now() - t0), co: !!p, off: p ? p.getAttribute("stroke-dashoffset") ?? getComputedStyle(p).strokeDashoffset : null });
          if (performance.now() - t0 < 2500) requestAnimationFrame(buoc);
        };
        requestAnimationFrame(buoc);
      });
      await t.page.evaluate((p) => {
        history.pushState({}, "", p);
        dispatchEvent(new PopStateEvent("popstate"));
      }, `/settlements/${G8}`);
      await t.page.waitForTimeout(2700);
      const mau = (await t.page.evaluate(() => window.__muc)).filter((m) => m.co);
      const cacGiaTri = [...new Set(mau.map((m) => m.off))];
      const dauTien = mau[0];
      const doiCuoi = mau.reduce((m, b, i) => (i > 0 && b.off !== mau[i - 1].off ? b.t : m), dauTien?.t ?? 0);
      log({ cfg, soMau: mau.length, dau: dauTien, cuoi: mau[mau.length - 1], giaTri: cacGiaTri.slice(0, 6) });
      const giam = cfg === "C9";
      ket({ tc: `TC-MO15-${cfg}`, screen: "F04.S03", state: "mở Quyết toán (sơ đồ 7 mũi tên)", action: "vào /settlements/[id] từ trong app", cauHinh: cfg, expected: giam ? "mũi tên hiện đủ ngay, không tự vẽ (giảm chuyển động)" : "mũi tên tự vẽ một lần rồi đứng yên (trần bật dựng ≤ 420 ms theo DESIGN)", status: !mau.length ? "FAIL" : giam ? (cacGiaTri.length === 1 ? "PASS" : "FAIL") : cacGiaTri.length > 1 ? "PASS" : "FAIL", evidence: [], ghiChu: mau.length ? `${mau.length} mẫu rAF từ lúc sơ đồ có mặt; dash offset: ${cacGiaTri.length} giá trị, đổi lần cuối ở ${doiCuoi - dauTien.t} ms sau khung đầu (SwiftShader: chỉ đọc trình tự, không đọc thời lượng)` : "không thấy sơ đồ" });
      await t.context.close();
    }
  }

  // ------------------------------------------ MO10 page turn between the steps
  if (chay("lat")) {
    for (const cfg of ["C1", "C9"]) {
      const t = await mo(cfg, P8, duongBill(null));
      const cdp = await cdpCua(t.page);
      // Count the page wrappers under the flip while going from step 1 to 2.
      await t.page.evaluate(() => {
        window.__lat = [];
        const goc = document.querySelector('[data-testid="receipt-review-screen"]') ?? document.body;
        const t0 = performance.now();
        const buoc = () => {
          const lat = [...goc.querySelectorAll("div")].filter((d) => getComputedStyle(d).backfaceVisibility === "hidden" && d.getBoundingClientRect().height > 50);
          window.__lat.push({ t: Math.round(performance.now() - t0), n: lat.length, tf: lat[0] ? getComputedStyle(lat[0]).transform.slice(0, 40) : null });
          if (performance.now() - t0 < 1200) requestAnimationFrame(buoc);
        };
        window.__batDau = () => requestAnimationFrame(buoc);
      });
      const n = await timNut(t.page, "Nhập tay");
      await t.page.evaluate(() => window.__batDau());
      const quay = await quayKhung(t.page, cdp, { ms: 900 });
      await cham(cdp, n);
      const khung = await quay.dung();
      await t.page.waitForTimeout(700);
      const mau = await t.page.evaluate(() => window.__lat);
      const coLat = mau.filter((m) => m.n > 0);
      await ghepKhung(mt.browser, khung, join(mt.out, "jpg", `EV-MO10-${cfg}.jpg`), { tieuDe: `MO10 lật trang bước 1 → 2, ${cfg}` });
      log({ cfg, soMau: mau.length, coLat: coLat.length, dau: coLat[0], cuoi: coLat[coLat.length - 1], khung: khung.length });
      const giam = cfg === "C9";
      ket({ tc: `TC-MO10-${cfg}`, screen: "F04.S01", state: "bước 1 → bước 2", action: "chạm «Nhập tay»", cauHinh: cfg, expected: giam ? "cắt thẳng, không có trang lật (giảm chuyển động)" : "trang cũ lật đi trong khoảng 300 ms, trang mới nằm yên bên dưới", status: giam ? (coLat.length === 0 ? "PASS" : "FAIL") : coLat.length > 0 && coLat[coLat.length - 1].t <= 600 ? "PASS" : "FAIL", evidence: [`EV-MO10-${cfg}`], ghiChu: `${mau.length} mẫu rAF; trang lật có mặt ở ${coLat.length} mẫu${coLat.length ? `, từ ${coLat[0].t} tới ${coLat[coLat.length - 1].t} ms` : ""}; ${khung.length} khung screencast` });
      await t.context.close();
    }
  }
} finally {
  await mt.dong();
}
