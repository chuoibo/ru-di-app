/* F10, the dev boards: /dev/ui-lab and /dev/san-khau. They exist only on a
 * dev server started with EXPO_PUBLIC_RUDI_FIXTURE=1 (`CUA_FIXTURE_DEV` needs
 * `__DEV__` too), and they render the live components under invented states
 * that live data cannot reach: broken pictures, seven album states, a failed
 * send, long names, the stage's tilt and fold. A finding here is about the
 * component the app ships, measured on synthetic data; the boards themselves
 * are not a product screen.
 *
 *   AUDIT_BASE=http://127.0.0.1:8081 node kich-ban/f10-bang-dev.mjs [--chi lab-base,san-base,nghieng,gap,album,kham-pha,so-sanh,long-nut,chang,sticker,hang-cho,o-tim,reorder,xem-anh,renderer,prod]
 *
 * `prod` opens the same paths on the production export (AUDIT_WEB) and needs
 * no dev server. Nothing here writes to the stack: the boards take no session.
 */
import { join } from "node:path";

import { chayAxe } from "../thu-vien/axe.mjs";
import { cauHinh } from "../thu-vien/cau-hinh.mjs";
import { chup, ghepAnh } from "../thu-vien/chup.mjs";
import { ghepKhung, quayKhung } from "../thu-vien/chuyen-dong.mjs";
import { cdpCua, cham, keo } from "../thu-vien/cu-chi.mjs";
import { duongDan } from "../thu-vien/dieu-huong.mjs";
import { choDialog, demDialog, dong, focusHienTai, inertConLai, luoiChamTrang, soLuoi } from "../thu-vien/lop-phu.mjs";
import { khoiDong, trangMoi } from "../thu-vien/moi-truong.mjs";
import { soGhi } from "../thu-vien/ghi.mjs";

const chi = (() => {
  const i = process.argv.indexOf("--chi");
  return i === -1 ? null : new Set(process.argv[i + 1].split(","));
})();
const chay = (ten) => !chi || chi.has(ten);
const log = (o) => console.log(JSON.stringify(o));
const catChu = (s, n = 220) => (s.length > n ? `${s.slice(0, n)}…` : s);

// The production check needs the export, not the dev server: read both before
// `khoiDong` picks one.
const baseDev = process.env.AUDIT_BASE ?? null;
const chiProd = chi && chi.size === 1 && chi.has("prod");
if (!baseDev && !chiProd) throw new Error("đặt AUDIT_BASE (server dev có EXPO_PUBLIC_RUDI_FIXTURE=1)");
if (chiProd) delete process.env.AUDIT_BASE;
const mt = await khoiDong();
const so = soGhi(mt.out);
const ket = (rec) => {
  so.ghi({ feature: "F10", nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, ...rec });
  console.log(`${rec.status.padEnd(10)} ${rec.tc} ${rec.cauHinh ?? ""} ${rec.ghiChu ?? ""}`);
};
const mo = (cfg, path) => trangMoi(mt, cauHinh(cfg), { persona: null, path });

/** The board's own scroll view (RudiScreen): the document itself never scrolls. */
const CUON = `(() => [...document.querySelectorAll("div")].filter((e) => /(auto|scroll)/.test(getComputedStyle(e).overflowY) && e.scrollHeight > e.clientHeight + 40).sort((a, b) => b.clientHeight - a.clientHeight)[0] ?? null)()`;
const NGANG = "for (let n = e.parentElement; n; n = n.parentElement) if (n.scrollLeft && !/(auto|scroll)/.test(getComputedStyle(n).overflowX)) n.scrollLeft = 0;";
/** Bring `sel` to `dinh` px from the top of the window, vertically only. */
const cuonToi = async (page, sel, dinh = 90) => {
  const ok = await page.evaluate(
    ({ sel, dinh, CUON }) => {
      const e = document.querySelector(sel);
      const c = eval(CUON);
      if (!e || !c) return false;
      c.scrollTop += e.getBoundingClientRect().top - dinh;
      return true;
    },
    { sel, dinh, CUON },
  );
  await page.waitForTimeout(500);
  return ok;
};
const cuonDoc = (page) => page.evaluate((CUON) => { const c = eval(CUON); return c ? Math.round(c.scrollTop) : null; }, CUON);
const tenCua = (x) => (x.getAttribute("aria-label") || (x.innerText ?? "")).replace(/[-]/g, "").replace(/\s+/g, " ").trim();
const timNut = (page, chu, { vai = '[role="button"],button,[role="tab"],[role="checkbox"],[role="radio"],[role="switch"]', batDau = false, trong = null } = {}) =>
  page.evaluate(
    ({ chu, vai, batDau, trong, NGANG, TEN }) => {
      const ten = new Function("x", TEN);
      const goc = trong ? document.querySelector(trong) : document;
      if (!goc) return null;
      const e = [...goc.querySelectorAll(vai)].find((x) => (batDau ? ten(x).startsWith(chu) : ten(x) === chu) && x.getClientRects().length);
      if (!e) return null;
      e.scrollIntoView({ block: "center", behavior: "instant" });
      new Function("e", NGANG)(e);
      const r = e.getBoundingClientRect();
      return { x: r.left + r.width / 2, y: r.top + r.height / 2, w: Math.round(r.width), h: Math.round(r.height), ten: ten(e) };
    },
    { chu, vai, batDau, trong, NGANG, TEN: `return (x.getAttribute("aria-label") || (x.innerText ?? "")).replace(/[\\uE000-\\uF8FF]/g, "").replace(/\\s+/g, " ").trim();` },
  );
const bam = async (t, cdp, chu, { cho = 700, ...o } = {}) => {
  const n = await timNut(t.page, chu, o);
  if (!n) return null;
  await cham(cdp, n);
  await t.page.waitForTimeout(cho);
  return n;
};
/** Visible text inside `sel`, normalised. */
const chuTrong = (page, sel) => page.evaluate((sel) => { const e = document.querySelector(sel); return e ? (e.innerText ?? "").replace(/[-]/g, "").replace(/\s+/g, " ").trim() : null; }, sel);
/** Measured signals (from the capture's own detector) that fall inside `sel`. */
const trongVung = async (page, sel, kq) => {
  const hop = await page.evaluate((sel) => { const r = document.querySelector(sel)?.getBoundingClientRect(); return r ? { l: r.left, t: r.top, r: r.right, b: r.bottom } : null; }, sel);
  if (!hop) return null;
  const trong = (h) => h && h.x + h.w / 2 >= hop.l && h.x + h.w / 2 <= hop.r && h.y + h.h / 2 >= hop.t && h.y + h.h / 2 <= hop.b;
  return {
    ellipsis: kq.doDac.ellipsis.filter((e) => trong(e.hop)).map((e) => `«${e.text}» thiếu ${e.thieu.x}/${e.thieu.y}px`),
    cat: kq.doDac.chuBiCat.filter((e) => trong(e.hop)).map((e) => `«${e.text}» (${e.kieu}) thiếu ${e.thieu.x}/${e.thieu.y}px`),
    tran: kq.doDac.tranNgang.filter((e) => trong(e.hop)).length,
    nho: kq.doDac.vungBamNho.filter((e) => trong(e.hop)).map((e) => `${e.ten} ${e.w}×${e.h}`),
  };
};

try {
  // ------------------------------------------------------------ baselines
  // The board is one long scroll view: capture it window by window, and let
  // each window's detector speak for that part.
  if (chay("lab-base")) {
    for (const cfg of ["C1", "C2", "C3"]) {
      const t = await mo(cfg, "/dev/ui-lab");
      await t.page.waitForTimeout(4000);
      const duong = await duongDan(t.page);
      const cao = await t.page.evaluate((CUON) => { const c = eval(CUON); return c ? { h: c.scrollHeight, v: c.clientHeight } : null; }, CUON);
      const doan = [];
      for (let y = 0, i = 0; cao && y < cao.h && i < 40; y += Math.round(cao.v * 0.85), i++) {
        await t.page.evaluate(({ y, CUON }) => { eval(CUON).scrollTop = y; }, { y, CUON });
        await t.page.waitForTimeout(700);
        const id = `EV-F10.S01-BASE-${cfg}-${String(i).padStart(2, "0")}`;
        const m = await chup(t.page, { out: mt.out, id, suKien: t.suKien });
        doan.push({ id, y, tt: m.tomTat, tran: m.doDac.tranNgang.slice(0, 3).map((e) => `${e.testid ?? e.tag} +${e.phai ?? ""}`), cat: m.doDac.chuBiCat.slice(0, 4).map((e) => `«${e.text}» ${e.kieu}`), elip: m.doDac.ellipsis.map((e) => `«${e.text}»`), nho: m.doDac.vungBamNho.filter((v) => v.duoi44).map((v) => `${v.ten} ${v.w}×${v.h}`) });
      }
      const axe = await chayAxe(t.page).catch((e) => ({ loi: String(e) }));
      for (let k = 0; k < doan.length; k += 6) {
        await ghepAnh(mt.browser, doan.slice(k, k + 6).map((d) => ({ file: join(mt.out, "jpg", `${d.id}.jpg`), nhan: `${d.id.slice(-2)} · y ${d.y}` })), join(mt.out, "xem", `F10-lab-${cfg}-${k / 6}.jpg`), { tieuDe: `/dev/ui-lab ${cfg}, cửa sổ ${k + 1}–${Math.min(k + 6, doan.length)} / ${doan.length}` });
      }
      log({ cfg, duong, cao, doan: doan.map((d) => ({ id: d.id.slice(-2), tran: d.tt.tranNgang, cat: d.tt.chuBiCat, elip: d.tt.ellipsis, che: d.tt.cheChu, nho44: d.tt.vungBamNho44, khongTen: d.tt.khongTen, renderer: d.tt.renderer, chiTiet: { tran: d.tran, cat: d.cat, nho: d.nho } })), axe: axe.loi ?? { vi: axe.vi.map((v) => `${v.rule}×${v.so} (${v.impact})`), chuaRo: axe.chuaRo.reduce((a, v) => a + v.so, 0) } });
      await t.context.close();
    }
  }

  if (chay("san-base")) {
    for (const [cfg, path, ten] of [["C1", "/dev/san-khau", "pho"], ["C2", "/dev/san-khau", "pho"], ["C3", "/dev/san-khau", "pho"], ["C1", "/dev/san-khau?canh=1", "canh"]]) {
      const t = await mo(cfg, path);
      await t.page.waitForTimeout(4000);
      const m = await chup(t.page, { out: mt.out, id: `EV-F10.S02-BASE-${ten}-${cfg}`, suKien: t.suKien });
      const axe = await chayAxe(t.page).catch((e) => ({ loi: String(e) }));
      log({ cfg, path, duong: await duongDan(t.page), tt: m.tomTat, axe: axe.loi ?? { vi: axe.vi.map((v) => `${v.rule}×${v.so} (${v.impact})`), chuaRo: axe.chuaRo.reduce((a, v) => a + v.so, 0) } });
      await t.context.close();
    }
    await ghepAnh(mt.browser, [["pho-C1", "C1"], ["pho-C2", "C2"], ["pho-C3", "C3"], ["canh-C1", "C1 ?canh=1"]].map(([k, nhan]) => ({ file: join(mt.out, "jpg", `EV-F10.S02-BASE-${k}.jpg`), nhan })), join(mt.out, "jpg", "EV-F10.S02-BASE-ghep.jpg"), { tieuDe: "F10.S02 /dev/san-khau" });
  }

  // ------------------------------------------------- the stage: tilt (MO12)
  // A sideways drag leans the stage (layers slide by depth) and a release
  // springs it back; a vertical drag must stay a scroll. Reduce Motion turns
  // the gesture off. Measured on pixels: the stage is a canvas or an SVG.
  if (chay("nghieng")) {
    for (const cfg of ["C1", "C9"]) {
      const t = await mo(cfg, "/dev/san-khau");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(4500);
      const san = await t.page.evaluate(() => { const r = document.querySelector('[data-testid="thu-canh-gap-san"]')?.getBoundingClientRect(); return r ? { x: r.left + r.width / 2, y: r.top + r.height / 2, l: Math.round(r.left), t: Math.round(r.top), w: Math.round(r.width), h: Math.round(r.height) } : null; });
      const renderer = await t.page.evaluate(() => [...new Set([...document.querySelectorAll("[data-renderer]")].map((e) => e.getAttribute("data-renderer")))].join("|"));
      const clip = san ? { x: san.l, y: san.t, width: san.w, height: san.h } : undefined;
      const anh = async () => (clip ? t.page.screenshot({ clip, type: "png" }) : null);
      const truoc = await anh();
      const cuon0 = await cuonDoc(t.page);
      // Drag right 120dp and hold; take the stage while the finger is still down.
      const dang = keo(cdp, { x: san.x - 60, y: san.y }, { x: san.x + 60, y: san.y }, { ms: 450, buoc: 18, giuCuoiMs: 1600 });
      await t.page.waitForTimeout(900);
      const giua = await anh();
      await dang;
      await t.page.waitForTimeout(1200);
      const sau = await anh();
      const cuon1 = await cuonDoc(t.page);
      const ngangTrang = await t.page.evaluate(() => Math.round(scrollX));
      // Compare the three stage pictures in a page, pixel by pixel.
      const khac = await t.page.evaluate(
        async ([a, b, c]) => {
          // Screenshots of the synthetic board, encoded at run time to decode them in the page.
          // repo-guard: allow=data-uri-base64 reason=runtime-encoded-audit-captures
          const doc = async (b64) => { const im = new Image(); im.src = `data:image/png;base64,${b64}`; await im.decode(); const cv = document.createElement("canvas"); cv.width = im.width; cv.height = im.height; const x = cv.getContext("2d"); x.drawImage(im, 0, 0); return x.getImageData(0, 0, im.width, im.height).data; };
          const [pa, pb, pc] = await Promise.all([a, b, c].map(doc));
          const ti = (p, q) => { let n = 0; for (let i = 0; i < p.length; i += 4) if (Math.abs(p[i] - q[i]) + Math.abs(p[i + 1] - q[i + 1]) + Math.abs(p[i + 2] - q[i + 2]) > 48) n++; return Math.round((1000 * n) / (p.length / 4)) / 10; };
          return { giuaSoTruoc: ti(pa, pb), sauSoTruoc: ti(pa, pc) };
        },
        [truoc, giua, sau].map((b) => b.toString("base64")),
      );
      // A vertical drag on the stage must scroll the list, not lean the stage.
      const d0 = await cuonDoc(t.page);
      await keo(cdp, { x: san.x, y: san.y + 40 }, { x: san.x + 6, y: san.y - 120 }, { ms: 400, buoc: 16 });
      await t.page.waitForTimeout(900);
      const d1 = await cuonDoc(t.page);
      const nghieng = khac.giuaSoTruoc >= 1;
      const veCho = khac.sauSoTruoc < 0.5;
      const chanCuon = await t.page.evaluate(() => { const san = document.querySelector('[data-testid="thu-canh-gap-san"]'); for (let n = san; n && n !== document.body; n = n.parentElement) if (getComputedStyle(n).touchAction === "none") return n.getAttribute("data-testid") ?? n.tagName.toLowerCase(); return null; });
      ket({
        tc: "TC-F10-MO12-KEO",
        screen: "F10.S02",
        state: `sân khấu «Phố đêm» đứng yên, renderer ${renderer || "?"}`,
        action: "kéo ngang 120dp trên tranh, giữ, thả",
        cauHinh: cfg,
        expected: cfg === "C9" ? "giảm chuyển động: kéo ngang không làm tranh nghiêng" : "tranh nghiêng theo tay khi giữ, thả ra thì về chỗ; trang không cuộn ngang, không cuộn dọc",
        status: (cfg === "C9" ? !nghieng : nghieng && veCho) && ngangTrang === 0 && cuon1 === cuon0 ? "PASS" : "FAIL",
        evidence: [],
        ghiChu: `điểm ảnh sân khấu khác lúc đầu: đang giữ ${khac.giuaSoTruoc}%, sau khi thả 1,2 s ${khac.sauSoTruoc}%; cuộn dọc khi kéo ngang ${cuon0} → ${cuon1}; trang cuộn ngang ${ngangTrang}`,
      });
      ket({
        tc: "TC-F10-KEO-DOC-TREN-TRANH",
        screen: "F10.S02",
        state: `sân khấu nhận kéo nghiêng (\`CanhGap keo\`), renderer ${renderer || "?"}`,
        action: "kéo dọc 160dp, bắt đầu trên tranh",
        cauHinh: cfg,
        expected: "danh sách cuộn theo tay, như khi bắt đầu kéo ở chỗ khác (chú thích mã: cử chỉ chỉ nhận cú kéo rõ ràng sang ngang)",
        status: d1 > d0 + 40 ? "PASS" : "FAIL",
        evidence: [],
        ghiChu: `danh sách cuộn ${d0} → ${d1}; lớp mang touch-action: none phía trên tranh: ${chanCuon ?? "không có"}`,
      });
      await t.context.close();
    }
  }

  // ---------------------------------------------- the stage: fold on scroll
  // Scrolling folds the stage into the page (C9: it only fades), and the bar's
  // small title may show only once the big title has gone under the bar.
  if (chay("gap")) {
    for (const cfg of ["C1", "C9"]) {
      const t = await mo(cfg, "/dev/san-khau");
      await t.page.waitForTimeout(4500);
      const mau = [];
      const doc = () => t.page.evaluate((CUON) => {
        const c = eval(CUON);
        const san = document.querySelector('[data-testid="thu-canh-gap-san"]');
        const nho = document.querySelector('[data-testid="thanh-canh-tieu-de"]');
        const lon = [...document.querySelectorAll('[role="heading"]')].find((e) => (e.innerText ?? "").trim() === "Phố đêm" && e !== nho);
        const thanh = nho?.parentElement?.parentElement?.getBoundingClientRect();
        let op = 1;
        for (let n = san; n && n !== document.body; n = n.parentElement) op *= Number(getComputedStyle(n).opacity);
        const rs = san?.getBoundingClientRect();
        return { y: Math.round(c?.scrollTop ?? -1), opSan: Math.round(op * 100) / 100, sanTop: rs ? Math.round(rs.top) : null, opNho: nho ? Math.round(Number(getComputedStyle(nho).opacity) * 100) / 100 : null, dayLon: lon ? Math.round(lon.getBoundingClientRect().bottom) : null, dayThanh: thanh ? Math.round(thanh.bottom) : null };
      }, CUON);
      mau.push(await doc());
      const buoc = [];
      await t.page.screenshot({ path: join(mt.out, "jpg", `EV-F10-GAP-${cfg}-0.jpg`), type: "jpeg", quality: 80 });
      buoc.push({ file: join(mt.out, "jpg", `EV-F10-GAP-${cfg}-0.jpg`), nhan: "cuộn 0" });
      for (let i = 0; i < 14; i++) {
        await t.page.evaluate((CUON) => { eval(CUON).scrollTop += 40; }, CUON);
        await t.page.waitForTimeout(260);
        mau.push(await doc());
        if (i < 4) {
          const f = join(mt.out, "jpg", `EV-F10-GAP-${cfg}-${(i + 1) * 40}.jpg`);
          await t.page.screenshot({ path: f, type: "jpeg", quality: 80 });
          buoc.push({ file: f, nhan: `cuộn ${(i + 1) * 40}` });
        }
      }
      const id = `EV-F10-GAP-${cfg}-ghep`;
      await ghepAnh(mt.browser, buoc, join(mt.out, "jpg", `${id}.jpg`), { tieuDe: `/dev/san-khau cuộn từng nấc 40dp, ${cfg}${cfg === "C9" ? " (giảm chuyển động)" : ""}` });
      // When the small title first shows, where is the big one?
      const hien = mau.find((m) => m.opNho !== null && m.opNho >= 0.5);
      const dungLuc = hien && hien.dayLon !== null && hien.dayThanh !== null ? hien.dayLon <= hien.dayThanh + 2 : null;
      const gapDan = cfg === "C9" ? mau.some((m) => m.opSan > 0.05 && m.opSan < 0.95) : null;
      log({ cfg, mau });
      ket({
        tc: "TC-F10-MO12-GAP",
        screen: "F10.S02",
        state: "sân khấu đứng yên ở đầu màn",
        action: "cuộn danh sách xuống 560dp, từng nấc 40dp",
        cauHinh: cfg,
        expected: cfg === "C9" ? "giảm chuyển động: sân khấu không gập 3D mà mờ dần theo cuộn" : "sân khấu gập vào trang theo cuộn (đọc ảnh), không nhảy",
        status: cfg === "C9" ? (gapDan ? "PASS" : "FAIL") : "NOT_TESTED",
        evidence: [id],
        ghiChu: `theo cuộn (y → độ mờ sân khấu, đỉnh sân khấu): ${mau.map((m) => `${m.y}→${m.opSan}/${m.sanTop}`).join(", ")}`,
      });
      ket({
        tc: "TC-F10-THANH-CANH",
        screen: "F10.S02",
        state: "tiêu đề lớn «Phố đêm» dưới sân khấu",
        action: "cuộn tới khi tiêu đề nhỏ trên thanh hiện",
        cauHinh: cfg,
        expected: "tiêu đề nhỏ chỉ hiện khi tiêu đề lớn đã khuất dưới thanh (không hiện hai tiêu đề cùng lúc, không hở khoảng không có tiêu đề nào)",
        status: dungLuc === null ? "FAIL" : dungLuc ? "PASS" : "FAIL",
        evidence: [id],
        ghiChu: hien ? `tiêu đề nhỏ đạt độ mờ ${hien.opNho} ở cuộn ${hien.y}: đáy tiêu đề lớn y ${hien.dayLon}, đáy thanh y ${hien.dayThanh}; theo cuộn (y → độ mờ tiêu đề nhỏ, đáy tiêu đề lớn): ${mau.map((m) => `${m.y}→${m.opNho}/${m.dayLon}`).join(", ")}` : `tiêu đề nhỏ không hiện sau ${mau.at(-1)?.y}dp cuộn`,
      });
      await t.context.close();
    }
  }

  // ------------------------------------------------ album: seven states
  if (chay("album")) {
    const CA = [["0", "0 ảnh"], ["1", "1 ảnh"], ["2-ngay", "2 ngày, lead lẻ"], ["le", "4 ảnh, lẻ"], ["ngay-la", "ngày không đọc được"], ["caption-dai", "caption dài"], ["anh-hong", "ảnh hỏng"]];
    for (const cfg of ["C1", "C2"]) {
      const t = await mo(cfg, "/dev/ui-lab");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(4000);
      const ketQua = [];
      for (const [ma, nhan] of CA) {
        const n = await bam(t, cdp, nhan, { vai: '[role="button"],[role="checkbox"],[role="radio"]', cho: 900 });
        // The album is whatever lies between the state chips and the next
        // section's title («Cảnh rỗng · A/B …»), measured by position.
        const vung = await t.page.evaluate(() => {
          const la = (re) => [...document.querySelectorAll("div,span")].find((e) => [...e.childNodes].some((n) => n.nodeType === 3 && re.test(n.textContent.trim())));
          const chip = la(/^ảnh hỏng$/);
          const sau = la(/^Cảnh rỗng · A\/B/);
          if (!chip || !sau) return null;
          const tren = chip.getBoundingClientRect().bottom + 4;
          const duoi = sau.getBoundingClientRect().top - 4;
          const trong = (e) => { const r = e.getBoundingClientRect(); return r.height > 0 && r.top >= tren && r.bottom <= duoi + 1; };
          const leaf = [...document.querySelectorAll("div,span")].filter((e) => trong(e) && [...e.childNodes].some((n) => n.nodeType === 3 && n.textContent.trim())).map((e) => e.textContent.trim());
          const imgs = [...document.querySelectorAll("img")].filter(trong).map((i) => ({ alt: i.getAttribute("alt"), w: i.offsetWidth, h: i.offsetHeight }));
          const hong = [...document.querySelectorAll('[aria-label^="Chưa tải được ảnh"]')].filter(trong).map((e) => e.getAttribute("aria-label"));
          const nut = [...document.querySelectorAll('[aria-label^="Mở ảnh: "]')].filter(trong).length;
          return { top: Math.round(tren), h: Math.round(duoi - tren), chu: [...new Set(leaf)].join(" · ").slice(0, 400), imgs, hong, nut };
        });
        const id = `EV-F10-ALBUM-${ma}-${cfg}`;
        if (n) await t.page.evaluate((top) => { const c = [...document.querySelectorAll("div")].filter((e) => /(auto|scroll)/.test(getComputedStyle(e).overflowY) && e.scrollHeight > e.clientHeight + 40).sort((a, b) => b.clientHeight - a.clientHeight)[0]; if (c) c.scrollTop += top - 140; }, vung?.top ?? 0);
        await t.page.waitForTimeout(500);
        const m = await chup(t.page, { out: mt.out, id, suKien: t.suKien });
        ketQua.push({ ma, nhan, chip: !!n, vung, tt: m.tomTat, cat: m.doDac.chuBiCat.slice(0, 3).map((e) => `«${e.text}» ${e.kieu}`), elip: m.doDac.ellipsis.slice(0, 4).map((e) => `«${e.text}»`) });
      }
      log({ cfg, ketQua });
      for (const k of ketQua) {
        const v = k.vung;
        const anhDoc = (v?.imgs ?? []).filter((i) => i.h > 0).length;
        let status = "PASS";
        let ly = "";
        if (!k.chip || !v) { status = "FAIL"; ly = "không mở được trạng thái"; }
        else if (k.ma === "0" && !/Chưa có khoảnh khắc/.test(v.chu)) { status = "FAIL"; ly = "không có trạng thái rỗng"; }
        else if (k.ma === "anh-hong" && v.hong.length < 2) { status = "FAIL"; ly = "ảnh hỏng không có chỗ thay"; }
        else if (k.tt.tranNgang || k.tt.chuBiCat) { status = "FAIL"; ly = "tràn hoặc cắt chữ"; }
        ket({
          tc: `TC-F10-ALBUM-${k.ma}`,
          screen: "F10.S01",
          layer: "-",
          state: `album tổng hợp «${k.nhan}»`,
          action: "chạm chip trạng thái",
          cauHinh: cfg,
          expected: "renderer album của app hiện đúng trạng thái: nhãn ngày, ảnh dẫn và ảnh phụ, chú thích; ảnh hỏng có chỗ thay; không tràn, không cắt",
          status,
          evidence: [`EV-F10-ALBUM-${k.ma}-${cfg}`],
          ghiChu: `${ly ? `${ly}; ` : ""}chữ: «${catChu(v?.chu ?? "-", 260)}»; ảnh vẽ ${anhDoc}/${v?.imgs?.length ?? 0} (${(v?.imgs ?? []).map((i) => `${i.w}×${i.h}`).join(", ")}); chỗ thay ảnh hỏng ${v?.hong?.length ?? 0}; nút mở ảnh ${v?.nut ?? 0}; tràn ${k.tt.tranNgang}, cắt ${k.tt.chuBiCat}${k.cat.length ? ` (${k.cat.join("; ")})` : ""}, ellipsis ${k.tt.ellipsis}${k.elip.length ? ` (${k.elip.join("; ")})` : ""}`,
        });
      }
      // The album's viewer: the component behind UI-094, on synthetic photos.
      if (cfg === "C1") {
        await bam(t, cdp, "4 ảnh, lẻ", { vai: '[role="button"],[role="checkbox"],[role="radio"]', cho: 900 });
        const nut = await timNut(t.page, "Mở ảnh: Ảnh tổng hợp 1");
        if (nut) await cham(cdp, nut);
        const moRa = await choDialog(t.page);
        await t.page.waitForTimeout(900);
        const anhXem = await t.page.evaluate(() => { const d = document.querySelector('[role="dialog"]'); return d ? [...d.querySelectorAll("img")].map((i) => { const r = i.getBoundingClientRect(); return { alt: i.getAttribute("alt"), w: Math.round(r.width), h: Math.round(r.height), trongMan: r.left >= -1 && r.right <= innerWidth + 1 }; }) : null; });
        const dem = await t.page.evaluate(() => [...document.querySelectorAll("div,span")].find((e) => e.children.length === 0 && /^\d+ \/ \d+$/.test((e.textContent ?? "").trim()) && e.getClientRects().length)?.textContent.trim() ?? null);
        await chup(t.page, { out: mt.out, id: "EV-F10-ALBUM-XEM-C1", suKien: t.suKien });
        const trongMan = (anhXem ?? []).find((a) => a.trongMan);
        ket({ tc: "TC-F10-ALBUM-XEM", screen: "F10.S01", layer: "L25", state: "album tổng hợp «4 ảnh, lẻ»", action: "chạm ảnh dẫn", cauHinh: "C1", expected: "trình xem mở, ảnh đang xem hiện với chiều cao thật", status: moRa.ok && trongMan && trongMan.h > 100 ? "PASS" : "FAIL", issue: null, evidence: ["EV-F10-ALBUM-XEM-C1"], ghiChu: `dialog ${moRa.ok ? `mở sau ${moRa.ms} ms` : "không mở"}; bộ đếm «${dem ?? "-"}»; ảnh: ${(anhXem ?? []).map((a) => `«${a.alt}» ${a.w}×${a.h}${a.trongMan ? " (trong khung)" : ""}`).join(", ") || "không có"}` });
        await t.page.keyboard.press("Escape");
        await t.page.waitForTimeout(600);
      }
      await t.context.close();
    }
  }

  // ------------------------------------------ Khám phá renderers, synthetic
  if (chay("kham-pha")) {
    for (const cfg of ["C1", "C2"]) {
      const t = await mo(cfg, "/dev/ui-lab");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(4000);
      for (const [ca, dai] of [["Không ảnh", false], ["Có ảnh", false], ["Ảnh hỏng", false], ["Cặp lệch", false], ["Không ảnh", true], ["Có ảnh", true]]) {
        await bam(t, cdp, ca, { vai: '[role="button"],[role="checkbox"],[role="radio"]', cho: 600 });
        const dangDai = await t.page.evaluate(() => { const e = [...document.querySelectorAll('[role="button"],[role="checkbox"]')].find((x) => (x.innerText ?? "").trim() === "Tên dài"); return e ? e.getAttribute("aria-checked") === "true" || e.getAttribute("aria-selected") === "true" || e.getAttribute("aria-pressed") === "true" : null; });
        if (dangDai !== null && dangDai !== dai) await bam(t, cdp, "Tên dài", { vai: '[role="button"],[role="checkbox"],[role="radio"]', cho: 600 });
        await cuonToi(t.page, '[data-testid="lab-lead"]', 70);
        const id = `EV-F10-KHAM-PHA-${ca === "Không ảnh" ? "khong" : ca === "Có ảnh" ? "co" : ca === "Ảnh hỏng" ? "hong" : "lech"}${dai ? "-dai" : ""}-${cfg}`;
        const m = await chup(t.page, { out: mt.out, id, suKien: t.suKien });
        const vung = {};
        for (const sel of ["lab-lead", "lab-compare", "lab-row"]) vung[sel] = await trongVung(t.page, `[data-testid="${sel}"]`, m);
        const chu = {};
        for (const sel of ["lab-lead", "lab-compare", "lab-row"]) chu[sel] = catChu((await chuTrong(t.page, `[data-testid="${sel}"]`)) ?? "-", 200);
        const luu = await t.page.evaluate(() => [...document.querySelectorAll('[aria-label^="Lưu "],[aria-label^="Bỏ lưu "]')].filter((e) => e.getClientRects().length).map((e) => { const r = e.getBoundingClientRect(); return `${Math.round(r.width)}×${Math.round(r.height)}`; }));
        const hong = await t.page.evaluate(() => [...document.querySelectorAll('[data-testid="lab-lead"] img,[data-testid="lab-compare"] img,[data-testid="lab-row"] img')].map((i) => `${i.offsetWidth}×${i.offsetHeight}${i.complete && i.naturalWidth === 0 ? " hỏng" : ""}`));
        const catHet = Object.values(vung).flatMap((v) => v?.cat ?? []);
        const tranHet = Object.values(vung).reduce((s, v) => s + (v?.tran ?? 0), 0);
        const elipHet = Object.entries(vung).flatMap(([k, v]) => (v?.ellipsis ?? []).map((e) => `${k.slice(4)}: ${e}`));
        ket({
          tc: `TC-F10-KHAM-PHA-${id.split("-").slice(4, -1).join("-").toUpperCase()}`,
          screen: "F10.S01",
          state: `ba renderer của Khám phá (dẫn, so sánh, hàng) với «${ca}»${dai ? " + «Tên dài»" : ""}`,
          action: "chạm chip trạng thái",
          cauHinh: cfg,
          expected: "tên, dòng mô tả và dòng giá đọc được; ảnh hỏng có chỗ thay; nút lưu ≥ 44dp; không tràn, không cắt",
          status: catHet.length || tranHet ? "FAIL" : "PASS",
          evidence: [id],
          ghiChu: `ellipsis: ${elipHet.join("; ") || "0"}; cắt: ${catHet.join("; ") || "0"}; tràn ${tranHet}; nút lưu ${luu.join(", ")}; ảnh ${hong.join(", ") || "0"}; chữ dẫn «${chu["lab-lead"]}»; hàng «${chu["lab-row"]}»`,
        });
      }
      await t.context.close();
    }
  }

  // ------------------------------- the compare pair's hearts, narrow phones
  // Without pictures each tile's head is one row: object, seal, spacer, heart.
  // With a seal («Hợp gu», live: an AI match without a reason line) the row
  // may not fit half of a narrow phone.
  if (chay("so-sanh")) {
    for (const cfg of ["C1", "C4", "C2"]) {
      const t = await mo(cfg, "/dev/ui-lab");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(4000);
      await bam(t, cdp, "Không ảnh", { vai: '[role="button"],[role="checkbox"],[role="radio"]', cho: 700 });
      await cuonToi(t.page, '[data-testid="lab-compare"]', 200);
      const tim = await t.page.evaluate(() => [...document.querySelectorAll('[data-testid="lab-compare"] [aria-label^="Lưu "],[data-testid="lab-compare"] [aria-label^="Bỏ lưu "]')].map((e) => {
        const r = e.getBoundingClientRect();
        // What stays visible after every clipping ancestor and the window.
        let l = r.left, rr = r.right;
        for (let n = e.parentElement; n && n !== document.body; n = n.parentElement) { const cs = getComputedStyle(n); if (/(hidden|clip)/.test(cs.overflowX)) { const b = n.getBoundingClientRect(); l = Math.max(l, b.left); rr = Math.min(rr, b.right); } }
        rr = Math.min(rr, innerWidth);
        const hien = Math.max(0, rr - l);
        const tamX = Math.min(innerWidth - 1, (l + rr) / 2);
        const trung = document.elementFromPoint(tamX, r.top + r.height / 2);
        return { ten: e.getAttribute("aria-label"), l: Math.round(r.left), r: Math.round(r.right), w: Math.round(r.width), hien: Math.round(hien), trung: !!trung && (trung === e || e.contains(trung)) };
      }));
      const id = `EV-F10-SO-SANH-${cfg}`;
      await chup(t.page, { out: mt.out, id, suKien: t.suKien });
      const hong = tim.filter((x) => x.hien < x.w - 1);
      ket({ tc: "TC-F10-SO-SANH-TIM", screen: "F10.S01", state: "cặp so sánh của Khám phá không ảnh; «Still Cafe» mang dấu «Hợp gu», «Lẩu gà lá é» không", action: "chạm «Không ảnh», đo nút tim của hai ô", cauHinh: cfg, expected: "cả hai nút «Lưu …» nằm trọn trong ô và trong màn, chạm được", status: hong.length ? "FAIL" : "PASS", evidence: [id], ghiChu: `cửa sổ ${cfg === "C2" ? 320 : cfg === "C4" ? 375 : 390}; ${tim.map((x) => `«${x.ten}» ${x.w}px ở ${x.l}–${x.r}, còn thấy ${x.hien}px, chạm giữa phần thấy ${x.trung ? "trúng nút" : "không trúng"}`).join(" | ")}` });
      await t.context.close();
    }
  }

  // ---------------------------------------- nested controls (seen by axe)
  // PlaceCompare draws the heart on the picture's corner, inside the «Mở …»
  // pressable, as soon as one of the pair has a picture; BanXoay puts its
  // typed-time field inside the dial. On the web both become a control inside
  // a control.
  if (chay("long-nut")) {
    const t = await mo("C1", "/dev/ui-lab");
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(4000);
    const longNhau = (goc) => t.page.evaluate((goc) => [...document.querySelectorAll(goc)].flatMap((g) => [...g.querySelectorAll('button,[role="button"],[role="adjustable"],[role="slider"]')].map((b) => ({ b, con: [...b.querySelectorAll('button,[role="button"],input,textarea,a[href]')] })).filter((x) => x.con.length).map(({ b, con }) => `${b.tagName.toLowerCase()}[${b.getAttribute("role") ?? ""}] «${b.getAttribute("aria-label") ?? ""}» ⊃ ${con.map((c) => `${c.tagName.toLowerCase()} «${c.getAttribute("aria-label") ?? c.getAttribute("placeholder") ?? ""}»`).join(", ")}`)), goc);
    const ketQua = {};
    for (const ca of ["Không ảnh", "Có ảnh"]) {
      await bam(t, cdp, ca, { vai: '[role="button"],[role="checkbox"],[role="radio"]', cho: 800 });
      ketQua[ca] = await longNhau('[data-testid="lab-compare"]');
    }
    // With a picture: does a tap on the heart save, and does Enter on it save?
    const tim = async () => t.page.evaluate(() => { const e = [...document.querySelectorAll('[data-testid="lab-compare"] [aria-label$="Lẩu gà lá é"]')].find((x) => /^(Lưu|Bỏ lưu) /.test(x.getAttribute("aria-label"))); if (!e) return null; e.scrollIntoView({ block: "center" }); const r = e.getBoundingClientRect(); return { x: r.left + r.width / 2, y: r.top + r.height / 2, ten: e.getAttribute("aria-label"), w: Math.round(r.width), h: Math.round(r.height) }; });
    const t0 = await tim();
    if (t0) await cham(cdp, t0);
    await t.page.waitForTimeout(600);
    const t1 = await tim();
    await t.page.evaluate(() => { const e = [...document.querySelectorAll('[data-testid="lab-compare"] [aria-label$="Lẩu gà lá é"]')].find((x) => /^(Lưu|Bỏ lưu) /.test(x.getAttribute("aria-label"))); e?.focus(); });
    await t.page.keyboard.press("Enter");
    await t.page.waitForTimeout(600);
    const t2 = await tim();
    await cuonToi(t.page, '[data-testid="lab-compare"]', 120);
    const m = await chup(t.page, { out: mt.out, id: "EV-F10-TIM-LONG-C1", suKien: t.suKien });
    const axe = await chayAxe(t.page);
    const long = axe.vi.find((v) => v.rule === "nested-interactive");
    ket({ tc: "TC-F10-TIM-LONG", screen: "F10.S01", state: "cặp so sánh của Khám phá, một quán có ảnh («Có ảnh»)", action: "đọc cây DOM; chạm tim; focus tim rồi Enter", cauHinh: "C1", expected: "nút «Lưu …» không nằm trong nút «Mở …» (không lồng control); chạm và Enter đều lưu", status: ketQua["Có ảnh"].length ? "FAIL" : "PASS", evidence: ["EV-F10-TIM-LONG-C1"], ghiChu: `«Không ảnh»: ${ketQua["Không ảnh"].length} chỗ lồng; «Có ảnh»: ${ketQua["Có ảnh"].join(" | ") || "0"}; axe nested-interactive ${long ? `×${long.so} (${long.impact}): ${long.vd.map((x) => x.target).join(", ")}` : "0"}; tim ${t0 ? `${t0.w}×${t0.h}` : "-"}: «${t0?.ten ?? "-"}» → chạm «${t1?.ten ?? "-"}» → Enter «${t2?.ten ?? "-"}»` });
    // The dial of a stop's time, on the board's primitives.
    await cuonToi(t.page, '[data-testid="lab-ban-xoay"]', 200);
    const quay = await longNhau('[data-testid="lab-ban-xoay"]');
    const vai = await t.page.evaluate(() => { const e = document.querySelector('[aria-label="Giờ chặng"]'); return e ? { role: e.getAttribute("role"), tabindex: e.getAttribute("tabindex"), now: e.getAttribute("aria-valuenow"), con: [...e.querySelectorAll("input,button,[role=\"button\"]")].map((c) => `${c.tagName.toLowerCase()} «${c.getAttribute("aria-label") ?? ""}»`) } : null; });
    ket({ tc: "TC-F10-BAN-XOAY-LONG", screen: "F10.S01", layer: "L15", state: "mặt quay giờ (BanXoay) trên bảng dev, cùng component của sheet chặng", action: "đọc cây DOM và axe", cauHinh: "C1", expected: "ô gõ giờ không nằm trong phần tử mặt quay có vai trò điều khiển", status: vai && vai.con.length ? "FAIL" : "PASS", evidence: [], ghiChu: `mặt quay: ${vai ? `role ${vai.role ?? "không"}, tabindex ${vai.tabindex ?? "-"}, aria-valuenow ${vai.now ?? "không"}, chứa ${vai.con.join(", ") || "không control nào"}` : "không thấy"}; lồng trong khối: ${quay.join(" | ") || "0"}` });
    await t.context.close();
  }

  // ------------------------------------------------- stops of an itinerary
  if (chay("chang")) {
    const t = await mo("C1", "/dev/ui-lab");
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(4000);
    for (const [ma, nhan] of [["ghi-cong", "Chặng có ghi công"], ["hong", "Chặng ảnh hỏng"], ["khong", "Chặng không ảnh"]]) {
      await bam(t, cdp, nhan, { vai: '[role="button"],[role="checkbox"],[role="radio"]', cho: 900 });
      await cuonToi(t.page, '[data-testid="lab-chang"]', 160);
      const id = `EV-F10-CHANG-${ma}-C1`;
      const m = await chup(t.page, { out: mt.out, id, suKien: t.suKien });
      const v = await trongVung(t.page, '[data-testid="lab-chang"]', m);
      const chu = await chuTrong(t.page, '[data-testid="lab-chang"]');
      const anh = await t.page.evaluate(() => [...document.querySelectorAll('[data-testid="lab-chang"] img')].map((i) => `${i.getAttribute("alt")} ${i.offsetWidth}×${i.offsetHeight}${i.complete && i.naturalWidth === 0 ? " hỏng" : ""}`));
      const coGhiCong = /Tác giả tổng hợp/.test(chu ?? "");
      ket({ tc: `TC-F10-CHANG-${ma.toUpperCase()}`, screen: "F10.S01", state: `hai chặng tổng hợp, «${nhan}»`, action: "chạm chip trạng thái", cauHinh: "C1", expected: ma === "ghi-cong" ? "ảnh chặng kèm dòng ghi công của chính chặng" : ma === "hong" ? "ảnh hỏng nhường chỗ cho đồ vật của loại chặng, không còn khung trống" : "chặng không ảnh vẫn đủ giờ, tên, dòng phụ", status: (ma === "ghi-cong" ? coGhiCong : true) && !v?.cat?.length && !v?.tran ? "PASS" : "FAIL", evidence: [id], ghiChu: `chữ «${catChu(chu ?? "-", 240)}»; ảnh ${anh.join(", ") || "0"}; cắt ${v?.cat?.join("; ") || "0"}; ellipsis ${v?.ellipsis?.join("; ") || "0"}` });
    }
    await t.context.close();
  }

  // ------------------------------------------------------- sticker tray
  if (chay("sticker")) {
    const t = await mo("C1", "/dev/ui-lab");
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(4000);
    const nutMo = await timNut(t.page, "Mở khay sticker");
    await t.page.waitForTimeout(400);
    const luoi0 = await luoiChamTrang(t.page);
    const vongDoi = [];
    for (const cach of ["x", "nen", "esc"]) {
      const n = await timNut(t.page, "Mở khay sticker");
      if (!n) { vongDoi.push({ cach, mo: false }); continue; }
      await cham(cdp, n);
      const moRa = await choDialog(t.page);
      await t.page.waitForTimeout(500);
      const focus = await focusHienTai(t.page);
      const d = await dong(t.page, cdp, cach);
      const tat = await choDialog(t.page, { co: false, toiDa: 2500 });
      await t.page.waitForTimeout(400);
      vongDoi.push({ cach, mo: moRa.ok, focusTrong: focus.trongDialog, dong: d.lam ? tat.ok : `không làm được: ${d.ly}`, url: await duongDan(t.page), focusSau: (await focusHienTai(t.page)).ten, inert: (await inertConLai(t.page)).length });
    }
    await timNut(t.page, "Mở khay sticker");
    await t.page.waitForTimeout(400);
    const luoi1 = await luoiChamTrang(t.page);
    // Pick one: the tray closes and both bubbles take the picked sticker.
    const n = await timNut(t.page, "Mở khay sticker");
    if (n) await cham(cdp, n);
    await choDialog(t.page);
    await t.page.waitForTimeout(500);
    await chup(t.page, { out: mt.out, id: "EV-F10-KHAY-STICKER-C1", suKien: t.suKien });
    const o = await t.page.evaluate(() => { const d = [...document.querySelectorAll('[role="dialog"]')].pop(); const b = d ? [...d.querySelectorAll('[role="button"]')].filter((x) => !/Đóng|Tay cầm/.test(x.getAttribute("aria-label") ?? "")) : []; const e = b[2]; if (!e) return null; const r = e.getBoundingClientRect(); return { x: r.left + r.width / 2, y: r.top + r.height / 2, ten: e.getAttribute("aria-label"), w: Math.round(r.width), h: Math.round(r.height), so: b.length }; });
    if (o) await cham(cdp, o);
    const tat = await choDialog(t.page, { co: false, toiDa: 2500 });
    const bubble = await t.page.evaluate(() => [...document.querySelectorAll('[data-testid="lab-sticker-bubble"] [aria-label]')].map((e) => e.getAttribute("aria-label")).slice(0, 4));
    ket({ tc: "TC-F10-KHAY-STICKER", screen: "F10.S01", layer: "L18", state: "khay sticker của chat, đặt trên bảng dev", action: "mở rồi đóng bằng nút, nền, Esc; mở, chọn một hình", cauHinh: "C1", expected: "mỗi cách đóng đều đóng, focus vào khay khi mở và về nút mở khi đóng, không sót lớp chặn, lưới chạm như trước; chọn hình thì khay đóng và bong bóng đổi hình", status: vongDoi.every((v) => v.mo && v.dong === true && v.inert === 0) && tat.ok && soLuoi(luoi0, luoi1).length === 0 ? "PASS" : "FAIL", evidence: ["EV-F10-KHAY-STICKER-C1"], ghiChu: `${vongDoi.map((v) => `${v.cach}: mở ${v.mo ? "có" : "không"}, focus ${v.focusTrong ? "trong khay" : "ngoài"}, đóng ${v.dong}, ở ${v.url}, focus sau «${v.focusSau ?? "body"}», lớp sót ${v.inert}`).join(" | ")}; lưới chạm khác ${soLuoi(luoi0, luoi1).length} ô; ô trong khay ${o ? `${o.so} ô, «${o.ten}» ${o.w}×${o.h}` : "không thấy"}; sau khi chọn: khay ${tat.ok ? "đóng" : "còn"}, bong bóng ${bubble.join(", ") || "-"}; nút mở ${nutMo ? `${nutMo.w}×${nutMo.h}` : "-"}` });
    await t.context.close();

    // The eight stickers with their names, at the narrowest phone: the
    // board's legend, and the names inside the real tray.
    const t2 = await mo("C2", "/dev/ui-lab");
    const cdp2 = await cdpCua(t2.page);
    await t2.page.waitForTimeout(4000);
    await cuonToi(t2.page, '[data-testid="lab-tam-sticker"]', 60);
    const m2 = await chup(t2.page, { out: mt.out, id: "EV-F10-TAM-STICKER-C2", suKien: t2.suKien });
    const v2 = await trongVung(t2.page, '[data-testid="lab-tam-sticker"]', m2);
    const nhan = await t2.page.evaluate(() => [...document.querySelectorAll('[data-testid="lab-tam-sticker"] > div')].map((h) => (h.innerText ?? "").trim()));
    ket({ tc: "TC-F10-TAM-STICKER", screen: "F10.S01", state: "bảng chú thích của trang dev: tám sticker ở cỡ 120 và 64 kèm tên", action: "cuộn tới bảng", cauHinh: "C2", expected: "tên mỗi sticker hiện trọn trên một dòng", status: v2 && !v2.ellipsis.length && !v2.cat.length ? "PASS" : "FAIL", evidence: ["EV-F10-TAM-STICKER-C2"], ghiChu: `tên: ${nhan.join(", ")}; ellipsis ${v2?.ellipsis?.join("; ") || "0"}; cắt ${v2?.cat?.join("; ") || "0"}` });
    const n2 = await timNut(t2.page, "Mở khay sticker");
    if (n2) await cham(cdp2, n2);
    await choDialog(t2.page);
    await t2.page.waitForTimeout(700);
    const m3 = await chup(t2.page, { out: mt.out, id: "EV-F10-KHAY-STICKER-C2", suKien: t2.suKien });
    const o2 = await t2.page.evaluate(() => { const d = [...document.querySelectorAll('[role="dialog"]')].pop(); return d ? [...d.querySelectorAll('[role="button"]')].filter((x) => /^Sticker|^Gửi|\S/.test(x.getAttribute("aria-label") ?? "") && !/Đóng|Tay cầm/.test(x.getAttribute("aria-label") ?? "")).map((x) => { const r = x.getBoundingClientRect(); return { ten: x.getAttribute("aria-label"), chu: (x.innerText ?? "").trim(), w: Math.round(r.width), h: Math.round(r.height) }; }) : []; });
    // Only signals whose box lies inside the tray: the board's own legend with
    // the same names sits under the backdrop.
    const hopKhay = await t2.page.evaluate(() => { const r = [...document.querySelectorAll('[role="dialog"]')].pop()?.getBoundingClientRect(); return r ? { l: r.left, t: r.top, r: r.right, b: r.bottom } : null; });
    const trongKhay = (h) => hopKhay && h && h.x + h.w / 2 >= hopKhay.l && h.x + h.w / 2 <= hopKhay.r && h.y + h.h / 2 >= hopKhay.t && h.y + h.h / 2 <= hopKhay.b;
    const catKhay = m3.doDac.ellipsis.concat(m3.doDac.chuBiCat).filter((e) => trongKhay(e.hop)).map((e) => `«${e.text}» thiếu ${e.thieu.x}/${e.thieu.y}px`);
    const dongNhan = await t2.page.evaluate(() => { const d = [...document.querySelectorAll('[role="dialog"]')].pop(); if (!d) return []; const out = []; const w = document.createTreeWalker(d, NodeFilter.SHOW_TEXT); for (let n = w.nextNode(); n; n = w.nextNode()) { if (!n.textContent.trim()) continue; const r = document.createRange(); r.selectNodeContents(n); const dinh = new Set([...r.getClientRects()].filter((x) => x.width > 0).map((x) => Math.round(x.top))); if (dinh.size > 1) out.push(`«${n.textContent.trim()}» ${dinh.size} dòng`); } return out; });
    ket({ tc: "TC-F10-KHAY-STICKER-TEN", screen: "F10.S01", layer: "L18", state: "khay sticker của chat (component thật)", action: "mở khay", cauHinh: "C2", expected: "mỗi ô ≥ 48dp; tên dưới hình đọc trọn hoặc có «…»", status: o2.length && o2.every((o) => o.w >= 48 && o.h >= 48) ? "PASS" : "FAIL", evidence: ["EV-F10-KHAY-STICKER-C2"], ghiChu: `${o2.length} ô: ${o2.map((o) => `«${o.chu || o.ten}» ${o.w}×${o.h}`).join(", ")}; chữ bị cắt hoặc «…» trong khay: ${catKhay.join("; ") || "0"}; nhãn xuống dòng: ${dongNhan.join(", ") || "0"}` });
    await t2.context.close();
  }

  // ----------------------------------------------------- send queue rows
  if (chay("hang-cho")) {
    for (const cfg of ["C1", "C2"]) {
      const t = await mo(cfg, "/dev/ui-lab");
      await t.page.waitForTimeout(4000);
      await cuonToi(t.page, '[data-testid="lab-hang-cho"]', 60);
      const id = `EV-F10-HANG-CHO-${cfg}`;
      const m = await chup(t.page, { out: mt.out, id, suKien: t.suKien });
      const v = await trongVung(t.page, '[data-testid="lab-hang-cho"]', m);
      const hang = await t.page.evaluate(() => [...document.querySelectorAll('[data-testid="lab-hang-cho"] > div')].map((h) => ({ chu: (h.innerText ?? "").replace(/[-]/g, "").replace(/\s+/g, " ").trim(), nut: [...h.querySelectorAll('[role="button"]')].map((b) => { const r = b.getBoundingClientRect(); return `${(b.getAttribute("aria-label") || b.innerText || "").replace(/\s+/g, " ").trim()} ${Math.round(r.width)}×${Math.round(r.height)}`; }) })));
      const [dangGui, hong, vinhVien] = hang;
      const dung = hang.length === 3 && !dangGui.nut.some((n) => /Thử lại/.test(n)) && hong.nut.some((n) => /Thử lại/.test(n)) && /Không nối được máy chủ/.test(hong.chu) && !vinhVien.nut.some((n) => /Thử lại/.test(n)) && /bản app chưa có/.test(vinhVien.chu);
      ket({ tc: "TC-F10-HANG-CHO", screen: "F10.S01", state: "ba trạng thái của một lần gửi sticker (hàng thật của chat)", action: "cuộn tới bảng", cauHinh: cfg, expected: "đang gửi: không nút; hỏng: câu lỗi và «Thử lại»; hỏng vĩnh viễn: câu lỗi, không mời thử lại; nút ≥ 44dp; không cắt", status: dung && !v?.cat?.length ? "PASS" : "FAIL", evidence: [id], ghiChu: `${hang.map((h) => `«${catChu(h.chu, 120)}» nút [${h.nut.join(", ")}]`).join(" | ")}; cắt ${v?.cat?.join("; ") || "0"}; vùng bấm < 48: ${v?.nho?.join(", ") || "0"}` });
      await t.context.close();
    }
  }

  // --------------------------------------------------- long placeholders
  if (chay("o-tim")) {
    for (const cfg of ["C1", "C2"]) {
      const t = await mo(cfg, "/dev/ui-lab");
      await t.page.waitForTimeout(4000);
      await cuonToi(t.page, '[data-testid="lab-o-tim"]', 120);
      const id = `EV-F10-O-TIM-${cfg}`;
      // The placeholder is a Text painted over the field (F44), so read the
      // capture's own ellipsis/cut signals inside the three fields.
      const m = await chup(t.page, { out: mt.out, id, suKien: t.suKien });
      const v = await trongVung(t.page, '[data-testid="lab-o-tim"]', m);
      const o = await t.page.evaluate(() => [...document.querySelectorAll('[data-testid="lab-o-tim"] input')].map((i) => {
        const khung = i.parentElement;
        const ve = [...(khung?.querySelectorAll("div,span") ?? [])].filter((e) => e !== i && [...e.childNodes].some((n) => n.nodeType === 3 && n.textContent.trim()) && e.getClientRects().length).map((e) => ({ chu: e.textContent.trim(), dong: Math.round(e.getBoundingClientRect().height), sw: e.scrollWidth, cw: e.clientWidth }));
        return { value: i.value, ve, h: i.offsetHeight };
      }));
      ket({ tc: "TC-F10-O-TIM", screen: "F10.S01", state: "ba ô tìm: placeholder dài của Khám phá, «Tìm thành phố hoặc tỉnh», ô đã gõ", action: "cuộn tới bảng", cauHinh: cfg, expected: "placeholder vẽ trên một dòng; dài hơn ô thì kết bằng «…», không xuống dòng rồi bị cắt ở đáy ô (F44)", status: !(v?.cat?.length) ? "PASS" : "FAIL", evidence: [id], ghiChu: `${o.map((x) => `ô cao ${x.h}: ${x.value ? `đã gõ «${x.value}»` : x.ve.map((e) => `«${e.chu}» cao ${e.dong}, rộng cần ${e.sw}/${e.cw}px`).join(", ") || "không thấy chữ vẽ"}`).join(" | ")}; ellipsis ${v?.ellipsis?.join("; ") || "0"}; cắt ${v?.cat?.join("; ") || "0"}` });
      await t.context.close();
    }
  }

  // --------------------------------------------------------- reorder list
  if (chay("reorder")) {
    const t = await mo("C1", "/dev/ui-lab");
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(4000);
    const ten = "Chặng C · tên dài để kiểm tra dòng chữ khi tăng kích cỡ hệ thống";
    const sel = `[aria-label="Thứ tự ${ten}"]`;
    await cuonToi(t.page, sel, 520);
    const thuTu = () => t.page.evaluate(() => [...document.querySelectorAll("div,span")].find((e) => e.children.length === 0 && /^Thứ tự: /.test((e.textContent ?? "").trim()) && e.getClientRects().length)?.textContent.trim() ?? null);
    const truoc = await thuTu();
    const tay = await t.page.evaluate((sel) => { const r = document.querySelector(sel)?.getBoundingClientRect(); return r ? { x: r.left + r.width / 2, y: r.top + r.height / 2, w: Math.round(r.width), h: Math.round(r.height) } : null; }, sel);
    const cuon0 = await cuonDoc(t.page);
    if (tay) await keo(cdp, tay, { x: tay.x, y: tay.y - 260 }, { ms: 900, buoc: 24, giuTruocMs: 450, giuCuoiMs: 150 });
    await t.page.waitForTimeout(900);
    const sau = await thuTu();
    const cuon1 = await cuonDoc(t.page);
    await chup(t.page, { out: mt.out, id: "EV-F10-REORDER-C1", suKien: t.suKien });
    const vaiTro = await t.page.evaluate((sel) => { const e = document.querySelector(sel); return e ? { role: e.getAttribute("role"), now: e.getAttribute("aria-valuenow"), text: e.getAttribute("aria-valuetext") } : null; }, sel);
    ket({ tc: "TC-F10-REORDER", screen: "F10.S01", state: `«${truoc}»`, action: "giữ tay nắm của chặng C 450 ms rồi kéo lên 260dp", cauHinh: "C1", expected: "chặng C lên đầu; danh sách không cuộn theo tay trong lúc kéo", status: sau && /^Thứ tự: c/.test(sau) && cuon1 === cuon0 ? "PASS" : "FAIL", evidence: ["EV-F10-REORDER-C1"], ghiChu: `trước «${truoc}», sau «${sau}»; cuộn ${cuon0} → ${cuon1}; tay nắm ${tay ? `${tay.w}×${tay.h}` : "không thấy"}; vai trò ${vaiTro ? `${vaiTro.role}, aria-valuenow ${vaiTro.now ?? "không có"}` : "-"}` });
    await t.context.close();
  }

  // ------------------------------------------------ photo viewer, 2 photos
  if (chay("xem-anh")) {
    for (const cfg of ["C1", "C9"]) {
      const t = await mo(cfg, "/dev/ui-lab");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(4000);
      const n = await bam(t, cdp, "Mở bộ ảnh tổng hợp", { cho: 100 });
      const moRa = await choDialog(t.page);
      await t.page.waitForTimeout(900);
      const anh = await t.page.evaluate(() => { const d = document.querySelector('[role="dialog"]'); return d ? [...d.querySelectorAll("img")].map((i) => { const r = i.getBoundingClientRect(); return { alt: i.getAttribute("alt"), w: Math.round(r.width), h: Math.round(r.height), trong: r.left >= -1 && r.right <= innerWidth + 1 }; }) : null; });
      const focus = await focusHienTai(t.page);
      const id = `EV-F10-XEM-ANH-${cfg}`;
      await chup(t.page, { out: mt.out, id, suKien: t.suKien });
      await t.page.keyboard.press("Escape");
      const tat = await choDialog(t.page, { co: false, toiDa: 2500 });
      const focusSau = await focusHienTai(t.page);
      const dangXem = (anh ?? []).find((a) => a.trong);
      ket({ tc: "TC-F10-XEM-ANH", screen: "F10.S01", layer: "L25", state: "hai ảnh minh hoạ tổng hợp (asset đóng gói, không qua mạng)", action: "«Mở bộ ảnh tổng hợp», rồi Esc", cauHinh: cfg, expected: "ảnh đang xem hiện với chiều cao thật; Esc đóng, focus về nút mở", status: moRa.ok && dangXem && dangXem.h > 100 && tat.ok ? "PASS" : "FAIL", evidence: [id], ghiChu: `nút ${n ? `${n.w}×${n.h}` : "không thấy"}; dialog ${moRa.ok ? `sau ${moRa.ms} ms` : "không mở"}; ảnh ${(anh ?? []).map((a) => `«${catChu(a.alt ?? "", 30)}» ${a.w}×${a.h}${a.trong ? " (trong khung)" : ""}`).join(", ") || "0"}; focus khi mở «${focus.ten ?? "body"}» (${focus.trongDialog ? "trong" : "ngoài"}); Esc ${tat.ok ? "đóng" : "không đóng"}, focus sau «${focusSau.ten ?? "body"}»` });
      await t.context.close();
    }
  }

  // ------------------------------------------------- renderer and replay
  if (chay("renderer")) {
    for (const cfg of ["C1", "C9"]) {
      const t = await mo(cfg, "/dev/ui-lab");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(4500);
      const ve = await t.page.evaluate(() => [...document.querySelectorAll("[data-renderer]")].map((e) => e.getAttribute("data-renderer")));
      const quay = await quayKhung(t.page, cdp, { ms: 2200 });
      await bam(t, cdp, "Chạy lại", { cho: 50 });
      const khung = await quay.dung();
      const anh = join(mt.out, "jpg", `EV-F10-CHAY-LAI-${cfg}.jpg`);
      await ghepKhung(mt.browser, khung, anh, { tieuDe: `/dev/ui-lab «Chạy lại» renderer, ${cfg}${cfg === "C9" ? " (giảm chuyển động)" : ""}` });
      ket({ tc: `TC-F10-CHAY-LAI`, screen: "F10.S01", state: `renderer trên trang: ${[...new Set(ve)].join(", ") || "không có"} (${ve.length} khung)`, action: "chạm «Chạy lại»", cauHinh: cfg, expected: cfg === "C9" ? "giảm chuyển động: Nếp và đường mực hiện ở tư thế cuối, không diễn" : "Nếp dựng lên và đường mực tự vẽ lại (đọc ảnh ghép)", status: "NOT_TESTED", evidence: [`EV-F10-CHAY-LAI-${cfg}`], ghiChu: `${khung.length} khung trong 2,2 s; cần đọc ảnh ghép để kết luận` });
      await t.context.close();
    }
  }

  // --------------------------------------- the boards in a production build
  if (chay("prod") && chiProd) {
    for (const path of ["/dev/ui-lab", "/dev/san-khau"]) {
      const t = await mo("C1", path);
      await t.page.waitForTimeout(3000);
      const toi = await duongDan(t.page);
      const coBang = await t.page.evaluate(() => !!document.querySelector('[data-testid="thu-renderer"],[data-testid="thu-man-san-khau"]'));
      ket({ tc: `TC-F10-PROD-${path.split("/").pop().toUpperCase()}`, screen: path === "/dev/ui-lab" ? "F10.S01" : "F10.S02", state: "bản export production (E1), chưa đăng nhập", action: `mở thẳng ${path}`, cauHinh: "C1", expected: "bảng dev không mở ở bản production (chuyển về /welcome)", status: toi === "/welcome" && !coBang ? "PASS" : "FAIL", evidence: [], ghiChu: `tới ${toi}; phần tử của bảng ${coBang ? "có" : "không"}` });
      await t.context.close();
    }
  }
} finally {
  await mt.dong();
}
