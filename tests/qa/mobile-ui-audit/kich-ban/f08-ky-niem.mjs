/* F08, memories and media: the group wall (prints, check-in, hearts,
 * comments), sharing a moment, the album shelf and one album with the photo
 * viewer, stories (post, watch, delete), posts (write, comment, react,
 * report, delete a comment), achievements, and the same screens opened with
 * no session, cold, failing, low and wide.
 *
 *   node kich-ban/f08-ky-niem.mjs [--chi du-lieu,tha,chon-huy,nhap,checkin,tuong,album,xem-anh,story,story-dong,bai,thanh-tich,tem-hep,khong-phien,lanh,loi,c8,voi-toi,tablet]
 *
 * Who writes what, all on the local stack. Team Đà Lạt is only read (its wall
 * has 2 photos and 3 check-ins from the seed). Every write goes to the
 * chat-test group (chat-0 is its admin) or to chat-0/chat-1 as people: one
 * outing «Kèo album F08» starting today so the shelf has an album, three
 * synthetic photos and one check-in on the group wall, a comment and a heart
 * from chat-1, stories of chat-0 (the delete case removes one), two posts of
 * chat-0 («Chỉ mình tôi», the composer's default, and «Bạn bè») with comments
 * from chat-1, one of them deleted. The report sheet is opened and closed,
 * never sent; the draft case (`nhap`) sends nothing. Image bytes are
 * generated (`pngThuBytes`), never a real photograph.
 */
import { randomUUID } from "node:crypto";
import { readFileSync } from "node:fs";
import { join } from "node:path";

import { pngThuBytes } from "../../../../apps/mobile/tools/png-thu.mjs";
import { chayAxe } from "../thu-vien/axe.mjs";
import { batDauLayMau, ketThucLayMau, phanTich } from "../thu-vien/chuyen-dong.mjs";
import { cauHinh } from "../thu-vien/cau-hinh.mjs";
import { chup } from "../thu-vien/chup.mjs";
import { cdpCua, cham, keo, nhip } from "../thu-vien/cu-chi.mjs";
import { choOn, duongDan } from "../thu-vien/dieu-huong.mjs";
import { choDialog, demDialog, dong, focusHienTai, inertConLai } from "../thu-vien/lop-phu.mjs";
import { goHet, loiMayChu } from "../thu-vien/mang.mjs";
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
  so.ghi({ feature: "F08", nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, ...rec });
  console.log(`${rec.status.padEnd(10)} ${rec.tc} ${rec.cauHinh ?? ""} ${rec.ghiChu ?? ""}`);
};
const log = (o) => console.log(JSON.stringify(o));
const phienDir = join(mt.out, "phien");

const P = (ten) => personaTheoTen(ten, mt.chatSessions);
const phienCua = (ten) => layPhien(mt.api, P(ten), phienDir);
const api = (method, path, body, phien) => goiApi(mt.api, method, path, body, phien.token);
/** A write with its own Idempotency-Key, answering the status and the body. */
const ghiApi = async (method, path, body, phien) => {
  const r = await fetch(`${mt.api}${path}`, { method, headers: { authorization: `Bearer ${phien.token}`, "content-type": "application/json", "idempotency-key": randomUUID() }, body: body === undefined ? undefined : JSON.stringify(body) });
  const text = await r.text();
  let json = null;
  try { json = JSON.parse(text); } catch { /* not JSON */ }
  return { status: r.status, json };
};
const an = (s) => String(s ?? "").replace(/[0-9a-f]{8}-[0-9a-f-]{27}/g, "[id]");

// Looked up, not written in: the ids differ from one stack to the next (the retest of
// main runs this on a second stack).
const G20 = JSON.parse(readFileSync(mt.chatSessions, "utf8")).groupId;
const { nhomId: G8 } = await moNhom(mt);
const KEO_ALBUM = "Kèo album F08";
const ngayVN = (lech = 0) => new Date(Date.now() + 7 * 3600e3 + lech * 864e5).toISOString().slice(0, 10);
const CAU_DAI = "Chiều cuối tuần cả nhóm ngồi bờ hồ đợi hoàng hôn, trời trong tới mức thấy cả dãy núi phía xa; ai cũng bảo lần sau phải mang thêm áo khoác vì gió lên nhanh, và phải đặt bàn sớm hơn vì quán đông từ năm giờ.";
const ANH = [
  { ten: "doc-9-16", w: 360, h: 640, cau: CAU_DAI },
  { ten: "ngang", w: 640, h: 360, cau: "Hoàng hôn trên hồ" },
  { ten: "doc-3-4", w: 480, h: 640, cau: "" },
];

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
const timNut = (page, chu, { vai = '[role="button"],button,[role="link"],a[href],[role="tab"],[role="radio"],[role="checkbox"]', batDau = false } = {}) =>
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
/** The visible text of the page, whole: a regex is never run on a sliced copy (F07 lesson). */
const chuDu = (page) => page.evaluate(() => (document.body.innerText ?? "").replace(/\s+/g, " "));
const catChu = (s, n = 220) => (s.length > n ? `${s.slice(0, n)}…` : s);
/** Pick a file through the picker a tap opens; `null` cancels it. */
const chonTep = async (t, cdp, chuNut, tep, opts = {}) => {
  const [fc] = await Promise.all([t.page.waitForEvent("filechooser", { timeout: 6000 }).catch(() => null), bam(t, cdp, chuNut, { cho: 200, ...opts })]);
  if (fc) await fc.setFiles(tep === null ? [] : [{ name: `${tep.ten}.png`, mimeType: "image/png", buffer: Buffer.from(pngThuBytes(tep.w, tep.h)) }]);
  await t.page.waitForTimeout(1500);
  return !!fc;
};
const nhapO = async (page, nhan, chu) => {
  const o = page.locator(`[aria-label="${nhan}"]`).filter({ visible: true }).first();
  if (!(await o.count())) return false;
  await o.click();
  await o.fill(chu);
  return true;
};
const hopNut = (page, chu) =>
  page.evaluate((chu) => {
    const ten = (x) => (x.getAttribute("aria-label") || (x.innerText ?? "")).replace(/[-]/g, "").replace(/\s+/g, " ").trim();
    const e = [...document.querySelectorAll('[role="button"],button')].find((x) => ten(x).startsWith(chu) && x.getClientRects().length);
    if (!e) return null;
    const r = e.getBoundingClientRect();
    const cs = getComputedStyle(e);
    const sau = e.parentElement?.nextElementSibling ?? e.nextElementSibling;
    return { w: Math.round(r.width), h: Math.round(r.height), tat: e.getAttribute("aria-disabled") === "true" || e.hasAttribute("disabled"), vien: cs.borderStyle, lyDoSau: (sau?.innerText ?? "").trim().slice(0, 80) };
  }, chu);

try {
  const pA = await phienCua("chat-0");
  const pB = await phienCua("chat-1");
  const pD = await phienCua("dalat-0");
  const keoAlbum = async () => ((await api("GET", `/contexts/${G20}/outings`, undefined, pA)).outings ?? []).find((o) => o.title === KEO_ALBUM);
  const kyNiem = async (ctx = G20, p = pA) => (await api("GET", `/contexts/${ctx}/memories?limit=50`, undefined, p)).memories ?? [];

  // -------------------------------------------------------- data for the shelf
  if (chay("du-lieu")) {
    let o = await keoAlbum();
    if (!o) {
      const r = await ghiApi("POST", `/contexts/${G20}/outings`, { title: KEO_ALBUM, starts_on: ngayVN(0), ends_on: ngayVN(1), headcount: 20, budget_per_person_vnd: 300000 }, pA);
      log({ taoKeo: r.status, loi: r.status >= 300 ? an(JSON.stringify(r.json)).slice(0, 200) : undefined });
      o = await keoAlbum();
    }
    log({ keoAlbum: o ? { tu: o.starts_on, den: o.ends_on } : null, kyNiemG20: (await kyNiem()).length, kyNiemG8: (await kyNiem(G8, pD)).length });
  }

  // ---------------------------------------------- share three moments (S04)
  // expo-image renders `<img alt=…>` on the web, and a print on the wall is
  // tilted, so a frame is measured by its layout box (offsetWidth/Height),
  // never by its bounding rectangle.
  if (chay("tha")) {
    const khungCua = (page, alt) =>
      page.evaluate((alt) => {
        const i = [...document.querySelectorAll("img")].find((x) => x.getAttribute("alt") === alt && x.getClientRects().length);
        if (!i) return null;
        i.scrollIntoView({ block: "center" });
        return { w: i.offsetWidth, h: i.offsetHeight, tiLe: Math.round((i.offsetWidth / i.offsetHeight) * 100) / 100, nw: i.naturalWidth, nh: i.naturalHeight };
      }, alt);
    const daCo = await kyNiem();
    for (const [i, a] of ANH.entries()) {
      const tiLeMuon = Math.round(Math.min(1.91, Math.max(0.75, a.w / a.h)) * 100) / 100;
      const t = await mo("C1", P("chat-0"), "/messages");
      const cdp = await cdpCua(t.page);
      await diToi(t, `/groups/${G20}/wall`);
      let guiLuc = null;
      if (!daCo.some((k) => k.kind === "photo" && (k.caption ?? "") === a.cau)) {
        await bam(t, cdp, "Thả khoảnh khắc", { cho: 1500 });
        if (i === 0) {
          const nut = await hopNut(t.page, "Chia sẻ ngay vào nhóm");
          await chup(t.page, { out: mt.out, id: "EV-F08-THA-TRONG-C1", suKien: t.suKien });
          ket({ tc: "TC-F08.S04-NUT-TAT", screen: "F08.S04", state: "chưa chọn ảnh", action: "mở «Thả khoảnh khắc» từ tường nhóm", cauHinh: "C1", expected: "ADR-0038 §2.2: nút gửi chưa dùng được thì ẩn, hoặc nói lý do ngay dưới", status: nut && nut.tat && !nut.lyDoSau ? "FAIL" : "PASS", evidence: ["EV-F08-THA-TRONG-C1"], ghiChu: `«Chia sẻ ngay vào nhóm» ${nut ? `${nut.w}×${nut.h}, ${nut.tat ? "tắt" : "bật"}, viền ${nut.vien}, dòng ngay dưới «${nut.lyDoSau || "không có"}»` : "không thấy"}` });
        }
        await chonTep(t, cdp, "Chưa chọn ảnh, chạm để chọn", a);
        if (a.cau) await nhapO(t.page, "Ô câu chú thích", a.cau);
        await bam(t, cdp, "Chia sẻ ngay vào nhóm", { cho: 4000 });
        guiLuc = an(await duongDan(t.page));
      }
      const khung = await khungCua(t.page, a.cau || "Ảnh của nhóm");
      await chup(t.page, { out: mt.out, id: `EV-F08-THA-SAU-${a.ten}-C1`, suKien: t.suKien });
      // The preview, without sending: pick the same bytes and read the frame.
      await bam(t, cdp, "Thả khoảnh khắc", { cho: 1500 });
      await chonTep(t, cdp, "Chưa chọn ảnh, chạm để chọn", a);
      const xem = await khungCua(t.page, "Ảnh đã chọn");
      if (i === 0) await chup(t.page, { out: mt.out, id: "EV-F08-THA-DA-CHON-C1", suKien: t.suKien });
      log({ anh: a.ten, guiLuc, khung, xem });
      ket({ tc: `TC-F08-THA-${a.ten.toUpperCase()}`, screen: "F08.S04", state: `ảnh tổng hợp ${a.w}×${a.h}${a.cau ? `, chú thích ${a.cau.length} ký tự` : ", không chú thích"}`, action: "chọn ảnh → (chú thích) → «Chia sẻ ngay vào nhóm»", cauHinh: "C1", expected: `xem trước và khung trên tường theo tỉ lệ ảnh, kẹp trong 3:4 … 1,91:1 (ở đây ${tiLeMuon}); gửi xong về tường nhóm`, status: (guiLuc === null || /\/wall/.test(guiLuc)) && khung && Math.abs(khung.tiLe - tiLeMuon) <= 0.02 && xem && Math.abs(xem.tiLe - tiLeMuon) <= 0.02 ? "PASS" : "FAIL", evidence: [`EV-F08-THA-SAU-${a.ten}-C1`], ghiChu: `${guiLuc ? `gửi xong ở ${guiLuc}; ` : "đã đăng từ lượt trước; "}khung trên tường ${khung ? `${khung.w}×${khung.h} (${khung.tiLe})` : "không thấy"}; xem trước ${xem ? `${xem.w}×${xem.h} (${xem.tiLe})` : "không thấy"}` });
      await t.context.close();
    }
  }

  // ---------------------------------------------- the picker closed empty (L31)
  if (chay("chon-huy")) {
    const t = await mo("C1", P("chat-0"), "/messages");
    const cdp = await cdpCua(t.page);
    await diToi(t, `/groups/${G20}/wall`);
    await bam(t, cdp, "Thả khoảnh khắc", { cho: 1500 });
    const moRa = await chonTep(t, cdp, "Chưa chọn ảnh, chạm để chọn", null);
    const r = { trong: !!(await timNut(t.page, "Chưa chọn ảnh, chạm để chọn")), loi: await viTriCau(t.page, /[Ll]ỗi|không|Không/), nut: await hopNut(t.page, "Chia sẻ ngay vào nhóm") };
    ket({ tc: "TC-L31-VONGDOI", screen: "F08.S04", layer: "L31", state: "chưa chọn ảnh", action: "chạm khung trống → bộ chọn tệp → đóng không chọn", cauHinh: "C1", expected: "không có câu lỗi; màn giữ nguyên, khung trống vẫn chạm lại được", status: moRa && r.trong && !r.loi?.trongMan ? "PASS" : "FAIL", evidence: [], ghiChu: `bộ chọn ${moRa ? "mở" : "không mở"}; khung trống ${r.trong ? "còn" : "mất"}; câu ${r.loi ? `«${r.loi.text}»` : "không"}` });
    await t.context.close();
  }

  // ------------------------------------ leaving a draft in a route modal (L34)
  // `moments/new` and `stories/new` keep the picked photo and the caption in
  // state and drop the pick on unmount (`boAnh`). Nothing in the app guards a
  // leave (no beforeRemove, no beforeunload). Does leaving ask first, and is
  // the draft there when the composer is opened again? Nothing is sent here.
  // `/check-ins/new`, the third route of L34, redirects to Kèo on a session.
  if (chay("nhap")) {
    const nhapCon = (page, oCau, nutTrong) =>
      page.evaluate(
        ({ oCau, nutTrong }) => {
          const hien = (x) => x.getClientRects().length > 0 && (x.checkVisibility ? x.checkVisibility({ visibilityProperty: true, opacityProperty: true }) : true);
          const o = [...document.querySelectorAll(`[aria-label="${oCau}"]`)].find(hien);
          return {
            duong: location.pathname,
            anh: [...document.querySelectorAll("img")].some((i) => i.getAttribute("alt") === "Ảnh đã chọn" && hien(i)),
            cau: o ? o.value : null,
            trong: [...document.querySelectorAll(`[aria-label="${nutTrong}"]`)].some(hien),
            hop: [...document.querySelectorAll('[role="dialog"],[role="alertdialog"]')].filter(hien).length,
          };
        },
        { oCau, nutTrong },
      );
    const MAN = [
      { ten: "moments", tu: `/groups/${G20}/wall`, nutMo: "Thả khoảnh khắc", nutTrong: "Chưa chọn ảnh, chạm để chọn", oCau: "Ô câu chú thích", screen: "F08.S04" },
      { ten: "stories", tu: "/messages", nutMo: "Đăng story mới", nutTrong: "Chưa có ảnh nào, chạm để chọn", oCau: "Ô chú thích", screen: "F08.S05" },
    ];
    const kq = {};
    for (const m of MAN) {
      for (const cach of ["back", "nut"]) {
        const t = await mo("C1", P("chat-0"), "/messages");
        const cdp = await cdpCua(t.page);
        const hoiTrinhDuyet = [];
        t.page.on("dialog", (d) => {
          hoiTrinhDuyet.push(d.type());
          void d.dismiss().catch(() => undefined);
        });
        if (m.tu === "/messages") await t.page.waitForTimeout(1500);
        else await diToi(t, m.tu);
        await bam(t, cdp, m.nutMo, { cho: 1500 });
        const focusVao = await focusHienTai(t.page);
        await chonTep(t, cdp, m.nutTrong, ANH[1]);
        await nhapO(t.page, m.oCau, "Nháp thử quay lại");
        await t.page.waitForTimeout(300);
        const truoc = await nhapCon(t.page, m.oCau, m.nutTrong);
        if (m.ten === "moments" && cach === "back") await chup(t.page, { out: mt.out, id: "EV-F08-L34-TRUOC-C1", suKien: t.suKien });
        if (cach === "back") await t.page.goBack();
        else await bam(t, cdp, "Quay lại", { cho: 0 });
        await t.page.waitForTimeout(1500);
        const roi = { ...(await nhapCon(t.page, m.oCau, m.nutTrong)), inert: (await inertConLai(t.page)).length };
        let toi = null;
        if (cach === "back") {
          // Forward, the one browser way back to the page just left.
          await t.page.goForward();
          await t.page.waitForTimeout(1500);
          toi = await nhapCon(t.page, m.oCau, m.nutTrong);
          await t.page.goBack();
          await t.page.waitForTimeout(1200);
        }
        await bam(t, cdp, m.nutMo, { cho: 1500 });
        const lai = await nhapCon(t.page, m.oCau, m.nutTrong);
        if (m.ten === "moments" && cach === "back") await chup(t.page, { out: mt.out, id: "EV-F08-L34-MO-LAI-C1", suKien: t.suKien });
        kq[`${m.ten}-${cach}`] = { focusVao: focusVao?.ten ?? null, truoc, roi: { ...roi, duong: an(roi.duong) }, toi: toi && { ...toi, duong: an(toi.duong) }, lai: { ...lai, duong: an(lai.duong) }, hoiTrinhDuyet };
        log({ l34: `${m.ten}-${cach}`, ...kq[`${m.ten}-${cach}`] });
        await t.context.close();
      }
    }
    const coNhap = (x) => x && (x.anh || x.cau === "Nháp thử quay lại");
    const moTa = (k) => {
      const x = kq[k];
      return `${k}: trước khi rời ${x.truoc.anh ? "có ảnh" : "không ảnh"}, câu «${x.truoc.cau ?? "?"}»; rời về ${x.roi.duong}, ${x.roi.hop} hộp thoại, ${x.hoiTrinhDuyet.length} hộp của trình duyệt, inert ${x.roi.inert}${x.toi ? `; Forward về ${x.toi.duong}, ${coNhap(x.toi) ? "còn nháp" : "trống"}` : ""}; mở lại ${x.lai.duong}, ${coNhap(x.lai) ? "còn nháp" : x.lai.trong ? "khung trống" : "?"}`;
    };
    const hoiHoacGiu = (k) => { const x = kq[k]; return x.hoiTrinhDuyet.length > 0 || x.roi.hop > 0 || /\/(moments|stories)\/new/.test(x.roi.duong) || coNhap(x.lai) || coNhap(x.toi); };
    const soanDuoc = (k) => { const x = kq[k]; return coNhap(x.truoc) && x.truoc.anh; };
    ket({ tc: "TC-L34-VONGDOI", screen: "F08.S04", layer: "L34", state: "«Thả khoảnh khắc» và «Đăng story» có ảnh và câu, chưa gửi", action: "Back trình duyệt (rồi Forward) hoặc «Quay lại» của đầu màn → mở lại từ cùng nút", cauHinh: "C1", expected: "rời màn không bỏ ảnh và câu đã soạn mà không hỏi: hoặc hỏi trước, hoặc mở lại còn nháp; rời xong không sót lớp chặn", status: Object.keys(kq).every(soanDuoc) ? (Object.keys(kq).every(hoiHoacGiu) && Object.values(kq).every((x) => x.roi.inert === 0) ? "PASS" : "FAIL") : "BLOCKED", evidence: ["EV-F08-L34-TRUOC-C1", "EV-F08-L34-MO-LAI-C1"], ghiChu: Object.keys(kq).map(moTa).join(" | ") });
  }

  // -------------------------------------------------- the check-in sheet (L24)
  if (chay("checkin")) {
    const kq = {};
    for (const cach of ["x", "nen", "esc", "back"]) {
      const t = await mo("C1", P("chat-0"), "/messages");
      const cdp = await cdpCua(t.page);
      await diToi(t, `/groups/${G20}/wall`);
      await bam(t, cdp, "Check-in", { cho: 100 });
      const moRa = await choDialog(t.page, { co: true });
      await t.page.waitForTimeout(1200);
      const focus = await focusHienTai(t.page);
      let nut = null;
      if (cach === "x") {
        nut = await hopNut(t.page, "Đăng check-in");
        await chup(t.page, { out: mt.out, id: "EV-F08-CHECKIN-C1", suKien: t.suKien });
      }
      const d = await dong(t.page, cdp, cach);
      await t.page.waitForTimeout(1000);
      kq[cach] = { mo: moRa.ok, focus: focus?.ten, trongDialog: focus?.trongDialog, lam: d.lam, con: await demDialog(t.page), duong: an(await duongDan(t.page)), inert: (await inertConLai(t.page)).length, nut };
      await t.context.close();
    }
    log({ checkinDong: kq });
    const x = kq.x;
    ket({ tc: "TC-L24-VONGDOI", screen: "F08.S01", layer: "L24", state: "tường nhóm chat-test", action: "«Check-in»; đóng bằng X, nền, Esc, Back", cauHinh: "C1", expected: "focus vào sheet; X, nền, Esc đóng mà ở lại tường, không sót inert; Back đóng sheet mà không rời tường", status: ["x", "nen", "esc", "back"].every((c) => kq[c].mo && kq[c].con === 0 && /\/wall$/.test(kq[c].duong) && kq[c].inert === 0) && x.trongDialog ? "PASS" : "FAIL", issue: /\/wall$/.test(kq.back.duong) ? null : "UI-038", evidence: ["EV-F08-CHECKIN-C1"], ghiChu: `${Object.entries(kq).map(([k, v]) => `${k}: ${v.con} hộp thoại, ở ${v.duong}, inert ${v.inert}`).join("; ")}; focus «${x.focus}» ${x.trongDialog ? "trong" : "ngoài"} sheet` });
    ket({ tc: "TC-L24-NUT-TAT", screen: "F08.S01", layer: "L24", state: "sheet Check-in, chưa chọn chỗ", action: "mở sheet", cauHinh: "C1", expected: "ADR-0038 §2.2: «Đăng check-in» chưa dùng được thì ẩn, hoặc nói lý do ngay dưới", status: x.nut && x.nut.tat && !/[Cc]họn/.test(x.nut.lyDoSau) ? "FAIL" : "PASS", evidence: ["EV-F08-CHECKIN-C1"], ghiChu: x.nut ? `${x.nut.w}×${x.nut.h}, ${x.nut.tat ? "tắt" : "bật"}, viền ${x.nut.vien}, ngay dưới «${x.nut.lyDoSau || "không có"}»` : "không thấy nút" });
    // Long catalogue names as chips: do they stay inside the sheet?
    {
      const t = await mo("C1", P("chat-0"), "/messages");
      const cdp = await cdpCua(t.page);
      await diToi(t, `/groups/${G20}/wall`);
      await bam(t, cdp, "Check-in", { cho: 2500 });
      const r = await t.page.evaluate(() => {
        const d = [...document.querySelectorAll('[role="dialog"]')].filter((x) => x.getClientRects().length).pop();
        const b = d?.getBoundingClientRect();
        const chips = d ? [...d.querySelectorAll('[aria-label^="Chọn "]')].filter((e) => e.getClientRects().length).map((e) => { const q = e.getBoundingClientRect(); return { ten: e.getAttribute("aria-label").slice(5), w: Math.round(q.width), phai: Math.round(q.right) }; }) : [];
        return { phaiSheet: b ? Math.round(b.right) : null, chips, tran: chips.filter((c) => b && c.phai > b.right - 1) };
      });
      await chup(t.page, { out: mt.out, id: "EV-F08-CHECKIN-CHIP-C1", suKien: t.suKien });
      ket({ tc: "TC-L24-CHIP-DAI", screen: "F08.S01", layer: "L24", state: "danh mục có tên quán 72 ký tự và tên liền 57 ký tự (dữ liệu biến thể F02)", action: "mở sheet Check-in", cauHinh: "C1", expected: "chip tên dài xuống dòng hoặc có dấu lược, nằm trọn trong sheet", status: r.tran.length === 0 ? "PASS" : "FAIL", evidence: ["EV-F08-CHECKIN-CHIP-C1"], ghiChu: `${r.chips.length} chip; mép phải sheet ${r.phaiSheet}; chip vượt mép: ${r.tran.map((c) => `«${catChu(c.ten, 40)}» rộng ${c.w}, tới ${c.phai}`).join("; ") || "không"}` });
      await t.context.close();
    }
    // Post one check-in, once.
    if (!(await kyNiem()).some((k) => k.kind === "checkin")) {
      const t = await mo("C1", P("chat-0"), "/messages");
      const cdp = await cdpCua(t.page);
      await diToi(t, `/groups/${G20}/wall`);
      await bam(t, cdp, "Check-in", { cho: 2000 });
      await nhapO(t.page, "Ô tìm chỗ check-in", "khong co cho nao nhu the");
      await t.page.waitForTimeout(600);
      const rong = await viTriCau(t.page, /Không có chỗ nào khớp/);
      await nhapO(t.page, "Ô tìm chỗ check-in", "");
      await t.page.waitForTimeout(600);
      const chips = await t.page.evaluate(() => [...document.querySelectorAll('[role="dialog"] [aria-label^="Chọn "]')].filter((e) => e.getClientRects().length).map((e) => { const r = e.getBoundingClientRect(); const c = e.querySelector("div,span"); return { ten: e.getAttribute("aria-label").slice(5), w: Math.round(r.width), h: Math.round(r.height), cat: [...e.querySelectorAll("*")].some((x) => x.scrollWidth > x.clientWidth + 1) }; }));
      const chip = chips[0];
      if (chip) await bam(t, cdp, `Chọn ${chip.ten}`, { cho: 500 });
      await nhapO(t.page, "Ô câu check-in", "Ngồi đây đợi cả nhóm");
      await bam(t, cdp, "Đăng check-in", { cho: 2500 });
      const sau = { con: await demDialog(t.page), cau: await viTriCau(t.page, /Check-in tại/) };
      await chup(t.page, { out: mt.out, id: "EV-F08-CHECKIN-SAU-C1", suKien: t.suKien });
      ket({ tc: "TC-L24-DANG", screen: "F08.S01", layer: "L24", state: "sheet Check-in", action: "tìm không ra → xoá tìm → chọn chip đầu → câu → «Đăng check-in»", cauHinh: "C1", expected: "tìm không ra có câu; chip đọc được và ≥ 44dp; đăng xong sheet đóng, tường có «Check-in tại …» ở đầu", status: rong && chip && chip.h >= 44 && sau.con === 0 && sau.cau?.trongMan ? "PASS" : "FAIL", evidence: ["EV-F08-CHECKIN-SAU-C1"], ghiChu: `câu tìm rỗng ${rong ? `«${rong.text}»` : "không có"}; ${chips.length} chip, chip đầu «${chip?.ten}» ${chip?.w}×${chip?.h}${chips.some((c) => c.cat) ? `, chip bị cắt: ${chips.filter((c) => c.cat).map((c) => c.ten).join(", ")}` : ""}; sau: ${sau.con} hộp thoại, «${sau.cau?.text ?? "không thấy check-in"}»` });
      await t.context.close();
    } else log({ boQua: "tường đã có check-in" });
  }

  // ------------------------------------------ hearts, comments, a failed heart
  if (chay("tuong")) {
    const t = await mo("C1", P("chat-1"), "/messages");
    const cdp = await cdpCua(t.page);
    await diToi(t, `/groups/${G20}/wall`);
    const truoc = await t.page.evaluate(() => [...document.querySelectorAll('[aria-label^="Thả tim"],[aria-label^="Bỏ tim"]')].filter((e) => e.getClientRects().length).map((e) => ({ ten: e.getAttribute("aria-label").slice(0, 60), pressed: e.getAttribute("aria-pressed"), h: Math.round(e.getBoundingClientRect().height), dai: e.getAttribute("aria-label").length })));
    const nutTim = await timNut(t.page, "Thả tim", { batDau: true });
    if (nutTim) await cham(cdp, nutTim);
    await t.page.waitForTimeout(1500);
    const sauTim = await t.page.evaluate(() => { const e = [...document.querySelectorAll('[aria-label^="Bỏ tim"]')].find((x) => x.getClientRects().length); return e ? { pressed: e.getAttribute("aria-pressed"), chu: (e.innerText ?? "").trim() } : null; });
    ket({ tc: "TC-F08-TIM", screen: "F08.S01", state: "tường nhóm có ảnh", action: "«Thả tim» trên kỷ niệm đầu", cauHinh: "C1", expected: "nút đổi thành «Đã tim», `aria-pressed` true; đếm tim tăng", status: sauTim && sauTim.pressed === "true" ? "PASS" : "FAIL", evidence: [], ghiChu: `trước: ${truoc.length} nút tim, nhãn dài nhất ${Math.max(0, ...truoc.map((x) => x.dai))} ký tự, cao ${[...new Set(truoc.map((x) => x.h))].join("/")}; sau: ${sauTim ? `«${sauTim.chu}», aria-pressed ${sauTim.pressed}` : "không thấy «Bỏ tim»"}` });
    // A comment of some length.
    const nutBl = await timNut(t.page, "Bình luận", { batDau: true });
    if (nutBl) await cham(cdp, nutBl);
    await t.page.waitForTimeout(1500);
    const moBl = await t.page.evaluate(() => { const e = [...document.querySelectorAll('[aria-label^="Ẩn bình luận"]')].find((x) => x.getClientRects().length); return e ? e.getAttribute("aria-expanded") : null; });
    const guiTruoc = !!(await timNut(t.page, "Gửi bình luận"));
    const BL = "Ảnh đẹp quá, lần sau nhớ gọi mình sớm hơn nhé, mình muốn ngồi đúng chỗ này lúc năm giờ chiều để chụp thêm vài tấm nữa với cả nhóm. Mà quán nước bên cạnh còn mở không?";
    await nhapO(t.page, "Ô viết bình luận", BL);
    await t.page.waitForTimeout(400);
    const guiSau = await timNut(t.page, "Gửi bình luận");
    if (guiSau) await cham(cdp, guiSau);
    await t.page.waitForTimeout(2000);
    const bl = await viTriCau(t.page, /lần sau nhớ gọi mình sớm hơn/);
    const tran = await t.page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1);
    await chup(t.page, { out: mt.out, id: "EV-F08-BINH-LUAN-C1", suKien: t.suKien });
    ket({ tc: "TC-F08-BINH-LUAN", screen: "F08.S01", state: "kỷ niệm đầu", action: "«Bình luận» → gõ 150 ký tự → «Gửi bình luận»", cauHinh: "C1", expected: "khối bình luận mở (`aria-expanded` true); nút gửi chỉ hiện khi có chữ (ADR-0038); bình luận hiện, xuống dòng, không tràn", status: moBl === "true" && !guiTruoc && guiSau && bl && !tran ? "PASS" : "FAIL", evidence: ["EV-F08-BINH-LUAN-C1"], ghiChu: `aria-expanded ${moBl}; nút gửi khi trống ${guiTruoc ? "có" : "không"}, khi có chữ ${guiSau ? `${guiSau.w}×${guiSau.h}` : "không"}; bình luận ${bl ? `y ${bl.top}–${bl.bottom}` : "không thấy"}; tràn ngang ${tran}` });
    // A heart that fails while reading an older memory: where does the sentence land?
    await loiMayChu(t.page, /\/memories\/[^/]+\/reactions/);
    const tim3 = await t.page.evaluate(() => { const ds = [...document.querySelectorAll('[aria-label^="Thả tim"],[aria-label^="Bỏ tim"]')].filter((e) => e.getClientRects().length); const e = ds[ds.length - 1]; if (!e) return null; e.scrollIntoView({ block: "center" }); const r = e.getBoundingClientRect(); return { x: r.left + r.width / 2, y: r.top + r.height / 2, i: ds.length }; });
    if (tim3) await cham(cdp, tim3);
    await t.page.waitForTimeout(1800);
    const cauLoi = await t.page.evaluate(() => { const e = [...document.querySelectorAll('[aria-live="polite"]')].find((x) => x.getClientRects().length && (x.innerText ?? "").trim()); if (!e) return null; const r = e.getBoundingClientRect(); return { chu: e.innerText.trim().slice(0, 120), top: Math.round(r.top), trongMan: r.bottom > 0 && r.top < innerHeight }; });
    await chup(t.page, { out: mt.out, id: "EV-F08-TIM-503-C1", suKien: t.suKien });
    await goHet(t.page);
    ket({ tc: "TC-F08-TIM-503", screen: "F08.S01", state: "đang xem kỷ niệm cuối tường; máy chủ trả 503 cho tim", action: "«Thả tim»", cauHinh: "C1", expected: "câu lỗi hiện gần kỷ niệm vừa chạm, trong khung nhìn", status: cauLoi?.trongMan ? "PASS" : "FAIL", evidence: ["EV-F08-TIM-503-C1"], ghiChu: `chạm nút tim thứ ${tim3?.i}; câu ${cauLoi ? `«${cauLoi.chu}» ở y ${cauLoi.top} (${cauLoi.trongMan ? "trong" : "ngoài"} khung nhìn)` : "không có"}` });
    await t.context.close();
  }

  // ------------------------------------------------------------ the shelf (S02)
  if (chay("album")) {
    const o = await keoAlbum();
    const t = await mo("C1", P("chat-0"), "/messages");
    const cdp = await cdpCua(t.page);
    await diToi(t, `/groups/${G20}/album`);
    const hang = await t.page.evaluate(() => [...document.querySelectorAll('[role="button"]')].filter((e) => e.getClientRects().length && /người/.test(e.innerText ?? "")).map((e) => (e.innerText ?? "").replace(/[-]/g, "").replace(/\s+/g, " ").trim()));
    await chup(t.page, { out: mt.out, id: "EV-F08-KE-ALBUM-C1", suKien: t.suKien });
    const vao = await bam(t, cdp, KEO_ALBUM, { batDau: true, cho: 2500 });
    const den = await duongDan(t.page);
    const anhs = await t.page.evaluate(() => [...document.querySelectorAll('[aria-label^="Mở ảnh"]')].filter((e) => e.getClientRects().length).map((e) => { const r = e.getBoundingClientRect(); return { ten: (e.getAttribute("aria-label") ?? "").slice(0, 40), w: Math.round(r.width), h: Math.round(r.height) }; }));
    const phim = !!(await timNut(t.page, "Dựng thước phim"));
    await chup(t.page, { out: mt.out, id: "EV-F08-ALBUM-C1", suKien: t.suKien });
    ket({ tc: "TC-F08-KE-ALBUM", screen: "F08.S02", state: `nhóm chat-test có kèo «${KEO_ALBUM}» bắt đầu hôm nay, 3 ảnh`, action: "mở kệ album, chạm album", cauHinh: "C1", expected: "kệ có một album «đang đi» với số ảnh; mở ra thấy ảnh, «Dựng thước phim»", status: o && hang.some((h) => h.includes(KEO_ALBUM)) && vao && /\/trips\/[^/]+\/album/.test(den) && anhs.length >= 3 && phim ? "PASS" : "FAIL", evidence: ["EV-F08-KE-ALBUM-C1", "EV-F08-ALBUM-C1"], ghiChu: `kệ: ${hang.map((h) => `«${catChu(h, 90)}»`).join(" ") || "rỗng"}; tới ${an(den)}; ${anhs.length} ảnh bấm được (${anhs.map((x) => `${x.w}×${x.h}`).join(", ")}); «Dựng thước phim» ${phim ? "có" : "không"}` });
    // The reel with no AI key: a sentence, not an error.
    if (phim) {
      await bam(t, cdp, "Dựng thước phim", { cho: 3500 });
      const cau = await chuDu(t.page);
      const m = cau.match(/Thước phim ([^.]*\.)|(chưa[^.]*\.)/);
      ket({ tc: "TC-F08-THUOC-PHIM", screen: "F08.S03", state: "máy chủ không có khoá AI", action: "«Dựng thước phim»", cauHinh: "C1", expected: "một câu nói vì sao chưa dựng được, không mã lỗi tiếng Anh", status: !/[A-Z_]{6,}|Error|error/.test(cau) ? "PASS" : "FAIL", evidence: [], ghiChu: `câu: «${catChu(m?.[0] ?? cau.slice(cau.indexOf("Thước phim"), cau.indexOf("Thước phim") + 200), 200)}»` });
    }
    await t.context.close();
  }

  // ------------------------------------------------ the photo viewer (L25)
  if (chay("xem-anh")) {
    const o = await keoAlbum();
    const moAlbum = async (cfg) => {
      const t = await mo(cfg, P("chat-0"), "/messages");
      const cdp = await cdpCua(t.page);
      await diToi(t, `/trips/${o.id}/album?ctx=${G20}`);
      return { t, cdp };
    };
    const nutMoCua = (page) => page.evaluate(() => { const e = [...document.querySelectorAll('[aria-label^="Mở ảnh"]')].find((x) => x.getClientRects().length); if (!e) return null; e.scrollIntoView({ block: "center" }); const r = e.getBoundingClientRect(); return { x: r.left + r.width / 2, y: r.top + r.height / 2, ten: e.getAttribute("aria-label") }; });
    const conHop = (page) => page.evaluate(() => !!document.querySelector('[aria-label="Đóng ảnh"]'));
    const moTa = (x) => (x.xuatHien ? `hiện ở mẫu ${x.tXuatHien} ms, độ mờ ${x.dau.op} → ${x.cuoi.op} trong ${x.thoiLuongMs} ms` : "không thấy");
    // Lifecycle and the fade, on a viewer nobody has pinched: on the web a
    // pinch zooms the whole page (measured below), which moves «Đóng» off the
    // screen, and a tap at the button's layout point then lands elsewhere.
    for (const cfg of ["C1", "C9"]) {
      const { t, cdp } = await moAlbum(cfg);
      const nutMo = await nutMoCua(t.page);
      await batDauLayMau(t.page, '[aria-label="Đóng ảnh"]');
      if (nutMo) await cham(cdp, nutMo);
      await t.page.waitForTimeout(1500);
      const moRa = phanTich(await ketThucLayMau(t.page));
      const v = await t.page.evaluate(() => {
        const dong = document.querySelector('[aria-label="Đóng ảnh"]');
        const hop = dong?.closest('[role="dialog"],[aria-modal="true"]');
        const dem = [...document.querySelectorAll("div,span")].find((e) => e.children.length === 0 && /^\d+ \/ \d+$/.test((e.textContent ?? "").trim()) && e.getClientRects().length)?.textContent.trim();
        const a = document.activeElement;
        const r = dong?.getBoundingClientRect();
        return { dialog: !!hop, role: hop?.getAttribute("role") ?? null, modal: hop?.getAttribute("aria-modal") ?? null, dem, focus: a && a !== document.body ? (a.getAttribute("aria-label") || a.tagName) : "body", nutDong: r ? `${Math.round(r.width)}×${Math.round(r.height)}` : null };
      });
      await chup(t.page, { out: mt.out, id: `EV-F08-XEM-ANH-${cfg}`, suKien: t.suKien });
      await t.page.keyboard.press("Escape");
      await t.page.waitForTimeout(1000);
      const sauEsc = await conHop(t.page);
      const focusEsc = await focusHienTai(t.page);
      const lai = await nutMoCua(t.page);
      if (lai) await cham(cdp, lai);
      await t.page.waitForTimeout(1500);
      const moLai = await conHop(t.page);
      const nutDong = await timNut(t.page, "Đóng ảnh");
      await batDauLayMau(t.page, '[aria-label="Đóng ảnh"]');
      if (nutDong) await cham(cdp, nutDong);
      await t.page.waitForTimeout(1200);
      const mauDong = await ketThucLayMau(t.page);
      const conSau = await conHop(t.page);
      const fAfter = await focusHienTai(t.page);
      const opDong = mauDong.filter((m) => m.n > 0).map((m) => m.op);
      const tMat = mauDong.find((m) => m.n === 0)?.t ?? null;
      log({ cfg, v, moRa: { tXuatHien: moRa.tXuatHien, thoiLuongMs: moRa.thoiLuongMs, dau: moRa.dau, cuoi: moRa.cuoi }, sauEsc, focusEsc: focusEsc?.ten, moLai, conSau, focus: fAfter?.ten, opDong: [...new Set(opDong)], tMat, soMau: mauDong.length });
      const lanDong = `«Đóng» ${conSau ? "không đóng" : "đóng"}${opDong.length ? `, còn thấy ${opDong.length} mẫu (độ mờ ${[...new Set(opDong)].join("→")})` : ", không còn mẫu nào thấy trình xem"}${tMat !== null ? `, mẫu đầu không còn trình xem ở ${tMat} ms kể từ lúc bắt đầu lấy mẫu` : ""}`;
      if (cfg === "C1") {
        ket({ tc: "TC-L25-VONGDOI", screen: "F08.S03", layer: "L25", state: "album có 3 ảnh, trình xem chưa ai chụm", action: "chạm ảnh; Esc; mở lại; «Đóng»", cauHinh: "C1", expected: "hộp thoại có role và focus vào trong; bộ đếm «1 / 3»; Esc và «Đóng» đóng hẳn; focus về ảnh đã mở", status: v.dialog && v.dem && !sauEsc && moLai && !conSau && /^Mở ảnh/.test(fAfter?.ten ?? "") ? "PASS" : "FAIL", evidence: ["EV-F08-XEM-ANH-C1"], ghiChu: `role ${v.role ?? "không có"}, aria-modal ${v.modal ?? "không có"}; focus khi mở «${v.focus}»; bộ đếm «${v.dem}»; nút «Đóng» ${v.nutDong}; Esc ${sauEsc ? "không đóng" : "đóng"}, focus sau Esc «${focusEsc?.ten ?? "body"}»; mở lại ${moLai ? "được" : "không được"}; ${lanDong}; focus sau «Đóng» «${fAfter?.ten ?? "body"}»` });
        ket({ tc: "TC-MO24-DONG", screen: "F08.S03", layer: "L25", state: "trình xem ảnh mở", action: "«Đóng»", cauHinh: "C1", expected: "mờ dần khi đóng như khi mở (animationType fade)", status: conSau ? "BLOCKED" : opDong.some((x) => x > 0.05 && x < 0.95) ? "PASS" : "FAIL", evidence: [], ghiChu: `mở: ${moTa(moRa)}; đóng: ${lanDong}` });
        const axe = await chayAxe(t.page).catch(() => ({ vi: [] }));
        log({ axeAlbum: (axe.vi ?? []).map((x) => `${x.rule}×${x.so}`) });
      } else {
        ket({ tc: "TC-MO24-C9", screen: "F08.S03", layer: "L25", state: "giảm chuyển động", action: "mở ảnh; «Đóng»", cauHinh: "C9", expected: "hiện và mất ngay, không mờ dần", status: moRa.xuatHien && moRa.thoiLuongMs <= 50 && !conSau && !opDong.some((x) => x > 0.05 && x < 0.95) ? "PASS" : "FAIL", evidence: ["EV-F08-XEM-ANH-C9"], ghiChu: `mở: ${moTa(moRa)}; đóng: ${lanDong}` });
      }
      await t.context.close();
    }
    // Gestures, C1, on a viewer of their own. Touches go to the middle of the
    // viewer's own scroller (the pager), never to «the last img», which can be
    // the next photo off to the right. The page's own zoom is read from
    // visualViewport: a pinch the photo does not take goes to the browser.
    {
      const { t, cdp } = await moAlbum("C1");
      const nutMo = await nutMoCua(t.page);
      if (nutMo) await cham(cdp, nutMo);
      await t.page.waitForTimeout(1500);
      const tam = await t.page.evaluate(() => {
        const d = document.querySelector('[role="dialog"][aria-modal="true"]');
        const cuon = d ? [...d.querySelectorAll("div")].find((e) => /(auto|scroll)/.test(getComputedStyle(e).overflowX) && e.clientWidth >= 300 && e.getBoundingClientRect().height > 100) : null;
        const r = cuon?.getBoundingClientRect();
        return r ? { x: r.left + r.width / 2, y: r.top + r.height / 2 } : null;
      });
      const anhHien = () => t.page.evaluate(() => { const d = document.querySelector('[role="dialog"][aria-modal="true"]'); const im = d ? [...d.querySelectorAll("img")].find((x) => { const r = x.getBoundingClientRect(); return r.left >= -1 && r.right <= innerWidth + 1; }) : null; const r = im?.getBoundingClientRect(); return im ? { alt: im.getAttribute("alt"), w: Math.round(r.width), h: Math.round(r.height), nw: im.naturalWidth, nh: im.naturalHeight } : null; });
      const tl = () => t.page.evaluate(() => { const d = document.querySelector('[role="dialog"][aria-modal="true"]'); const im = d ? [...d.querySelectorAll("img")].find((x) => { const r = x.getBoundingClientRect(); return r.left >= -1 && r.right <= innerWidth + 1; }) : null; let n = im; for (let i = 0; i < 6 && n; i++, n = n.parentElement) { const tf = n.style?.transform; if (tf && /scale/.test(tf)) return tf; } return null; });
      const cuonX = () => t.page.evaluate(() => { const d = document.querySelector('[role="dialog"][aria-modal="true"]'); const c = d ? [...d.querySelectorAll("div")].find((e) => /(auto|scroll)/.test(getComputedStyle(e).overflowX) && e.clientWidth >= 300) : null; return c ? Math.round(c.scrollLeft) : null; });
      const demDoc = () => t.page.evaluate(() => [...document.querySelectorAll("div,span")].find((e) => e.children.length === 0 && /^\d+ \/ \d+$/.test((e.textContent ?? "").trim()) && e.getClientRects().length)?.textContent.trim());
      const vv = () => t.page.evaluate(() => {
        const s = visualViewport.scale;
        const d = document.querySelector('[aria-label="Đóng ảnh"]')?.getBoundingClientRect();
        const x = d ? (d.left + d.width / 2 - visualViewport.offsetLeft) * s : null;
        const y = d ? (d.top + d.height / 2 - visualViewport.offsetTop) * s : null;
        return { scale: Math.round(s * 100) / 100, dong: d ? { x: Math.round(x), y: Math.round(y), trongMan: x >= 0 && x <= innerWidth * s && y >= 0 && y <= visualViewport.height * s } : null };
      });
      const anh0 = await anhHien();
      const x0 = await cuonX();
      const demTruoc = await demDoc();
      // Swipe first, on a fresh viewer: where the pager goes, and what the header says.
      if (tam) await keo(cdp, { x: tam.x + 120, y: tam.y }, { x: tam.x - 160, y: tam.y }, { ms: 250, buoc: 10 });
      await t.page.waitForTimeout(1500);
      const x1 = await cuonX();
      const demSau = await demDoc();
      const anh1 = await anhHien();
      const tf0 = await tl();
      if (tam) { await cham(cdp, tam, 30); await t.page.waitForTimeout(90); await cham(cdp, tam, 30); }
      await t.page.waitForTimeout(900);
      const tf1 = await tl();
      const vv1 = await vv();
      if (tam) await nhip(cdp, tam, { tu: 60, den: 220, ms: 500 });
      await t.page.waitForTimeout(900);
      const tf2 = await tl();
      const vv2 = await vv();
      const x2 = await cuonX();
      if (tam) await keo(cdp, { x: tam.x - 120, y: tam.y }, { x: tam.x + 160, y: tam.y }, { ms: 250, buoc: 10 });
      await t.page.waitForTimeout(1500);
      const x3 = await cuonX();
      const vv3 = await vv();
      await chup(t.page, { out: mt.out, id: "EV-F08-XEM-ANH-CHUM-C1", suKien: t.suKien });
      log({ tam, anh0, anh1, tf0, tf1, tf2, vv1, vv2, vv3, x0, x1, x2, x3, demTruoc, demSau });
      ket({ tc: "TC-L25-ANH", screen: "F08.S03", layer: "L25", state: "album có 3 ảnh (480×640, 640×360, 360×640)", action: "chạm ảnh để mở trình xem", cauHinh: "C1", expected: "ảnh hiện trọn trong vùng giữa đầu và chú thích (contain)", status: anh0 && anh0.h > 100 ? "PASS" : "FAIL", evidence: ["EV-F08-XEM-ANH-C1"], ghiChu: `ảnh trong khung nhìn: ${anh0 ? `«${anh0.alt}» ${anh0.w}×${anh0.h} (ảnh gốc ${anh0.nw}×${anh0.nh})` : "không có"}` });
      ket({ tc: "TC-L25-CU-CHI", screen: "F08.S03", layer: "L25", state: "trình xem ảnh mở ở ảnh đầu", action: "vuốt trái; chạm đúp; chụm hai ngón; vuốt phải (giữa vùng lật trang)", cauHinh: "C1", expected: "vuốt sang ảnh sau và bộ đếm đổi; chạm đúp phóng to ảnh 2×; chụm phóng to ảnh, trang giữ nguyên cỡ", status: demSau && demSau !== demTruoc && /scale\(2\)/.test(tf1 ?? "") && tf2 && /scale\((1\.[1-9]|[2-4])/.test(tf2) && vv2.scale === 1 ? "PASS" : "FAIL", evidence: ["EV-F08-XEM-ANH-CHUM-C1"], ghiChu: `chạm ở ${tam ? `${Math.round(tam.x)},${Math.round(tam.y)}` : "-"}; vuốt trái lúc mới mở: khung lật trang cuộn ${x0} → ${x1}px (mỗi ảnh ${Math.round(tam ? tam.x * 2 : 0)}px), bộ đếm ${demTruoc} → ${demSau}, ảnh trong khung ${anh1 ? `«${catChu(anh1.alt, 40)}» ${anh1.w}×${anh1.h}` : "không có"}; transform của ảnh: đầu «${tf0}», sau chạm đúp «${tf1}» (trang ×${vv1.scale}), sau chụm «${tf2}»; sau chụm trang phóng ×${vv2.scale}, «Đóng ảnh» trên màn ở ${vv2.dong ? `${vv2.dong.x},${vv2.dong.y} (${vv2.dong.trongMan ? "trong" : "ngoài"} màn)` : "-"}; vuốt phải sau đó: khung lật trang ${x2} → ${x3}px, trang ×${vv3.scale}` });
      await t.context.close();
    }
  }

  // ------------------------------------------------------------ stories (S05, S06)
  if (chay("story")) {
    const cua = async () => ((await api("GET", "/stories", undefined, pA)).authors ?? []).find((x) => x.author?.id === pA.person_id);
    // chat-0 posts two stories through the rail on Tin nhắn.
    for (let i = (await cua())?.stories?.length ?? 0; i < 2; i += 1) {
      const t = await mo("C1", P("chat-0"), "/messages");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(1500);
      await bam(t, cdp, "Đăng story mới", { cho: 1500 });
      const den = await duongDan(t.page);
      const nut0 = await hopNut(t.page, "Đăng story");
      const chon = await chonTep(t, cdp, "Chưa có ảnh nào, chạm để chọn", i === 0 ? { ten: "story-doc", w: 360, h: 640 } : { ten: "story-ngang", w: 640, h: 360 });
      await nhapO(t.page, "Ô chú thích", i === 0 ? "Sáng nay ở chợ" : "");
      if (i === 0) await chup(t.page, { out: mt.out, id: "EV-F08-DANG-STORY-C1", suKien: t.suKien });
      await bam(t, cdp, "Đăng story", { cho: 4000 });
      const sau = await duongDan(t.page);
      log({ story: i, den: an(den), nut0, chon, sau: an(sau) });
      if (i === 0) ket({ tc: "TC-F08-DANG-STORY", screen: "F08.S05", state: "chưa có story", action: "Tin nhắn → «Đăng story mới» → chọn ảnh → chú thích → «Đăng story»", cauHinh: "C1", expected: "nút tắt có lý do dưới khi chưa chọn ảnh; đăng xong về Tin nhắn, dải story có story của mình", status: /\/stories\/new/.test(den) && chon && /\/messages/.test(sau) && nut0?.tat ? "PASS" : "FAIL", evidence: ["EV-F08-DANG-STORY-C1"], ghiChu: `tới ${an(den)}; «Đăng story» lúc chưa chọn: ${nut0 ? `${nut0.tat ? "tắt" : "bật"}, dưới «${nut0.lyDoSau || "không"}»` : "không thấy"}; sau khi đăng ở ${an(sau)}` });
      await t.context.close();
    }
    const nhomA = await cua();
    log({ storiesA: nhomA?.stories?.length ?? 0 });
    // chat-1 watches: tap zones, sizes, the clock at C1 and at C9.
    for (const cfg of ["C1", "C9"]) {
      const t = await mo(cfg, P("chat-1"), "/messages");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(1800);
      const vong = await t.page.evaluate(() => [...document.querySelectorAll('[data-testid="story-rail"] [role="button"],[data-testid="story-rail"] [aria-label]')].filter((e) => e.getClientRects().length).map((e) => ({ ten: (e.getAttribute("aria-label") ?? "").slice(0, 60), role: e.getAttribute("role") })));
      const vongA = vong.find((x) => /Chat Test 01/.test(x.ten));
      if (vongA) await bam(t, cdp, vongA.ten, { cho: 300 });
      const t0 = Date.now();
      const doanDay = () => t.page.evaluate(() => { const bars = [...document.querySelectorAll('[data-testid="xem-story-screen"] div')].filter((e) => e.getClientRects().length && Math.round(e.getBoundingClientRect().height) === 3 && e.parentElement && getComputedStyle(e.parentElement).flexDirection === "row" && e.children.length === 2); return bars.map((b) => { const f = b.children[0]; return Math.round((f.getBoundingClientRect().width / b.getBoundingClientRect().width) * 100); }); });
      await t.page.waitForTimeout(1200);
      const d1 = await doanDay();
      await t.page.waitForTimeout(6000);
      const d2 = await doanDay();
      const ms = Date.now() - t0;
      const vung = await t.page.evaluate(() => [...document.querySelectorAll('[aria-label="Story trước"],[aria-label="Story tiếp theo"],[aria-label="Đóng story"],[aria-label="Xoá story"]')].map((e) => { const r = e.getBoundingClientRect(); return { ten: e.getAttribute("aria-label"), role: e.getAttribute("role"), tab: e.getAttribute("tabindex"), w: Math.round(r.width), h: Math.round(r.height) }; }));
      const axe = cfg === "C1" ? await chayAxe(t.page).catch(() => ({ vi: [] })) : { vi: [] };
      await chup(t.page, { out: mt.out, id: `EV-F08-XEM-STORY-${cfg}`, suKien: t.suKien });
      // Keyboard: focus «Story tiếp theo» (the last story is showing) and press Enter.
      let banPhim = null;
      if (cfg === "C1") {
        const truocPhim = await duongDan(t.page);
        const coFocus = await t.page.evaluate(() => { const e = document.querySelector('[aria-label="Story tiếp theo"]'); e?.focus(); return document.activeElement === e; });
        await t.page.keyboard.press("Enter");
        await t.page.waitForTimeout(1200);
        banPhim = { coFocus, truoc: an(truocPhim), sau: an(await duongDan(t.page)) };
      }
      log({ cfg, vong, d1, d2, ms, vung, banPhim, axe: (axe.vi ?? []).map((x) => `${x.rule}×${x.so}:${x.vd.map((v) => v.target).join("|")}`) });
      if (cfg === "C1") {
        ket({ tc: "TC-F08-STORY-TU-CHUYEN", screen: "F08.S06", layer: "L26", state: "Chat Test 01 có 2 story, người xem là bạn", action: "mở story từ dải, đợi", cauHinh: "C1", expected: "đoạn đầu đầy trong khoảng 5 s rồi sang story thứ hai; story cuối dừng ở vạch đầy, không tự đóng", status: d1.length === 2 && d2.length === 2 && d2[0] === 100 && d2[1] > 0 ? "PASS" : "FAIL", evidence: [`EV-F08-XEM-STORY-C1`], ghiChu: `vạch sau 1,2 s: ${d1.join("/")}%; sau ${Math.round(ms / 100) / 10} s: ${d2.join("/")}%` });
        const khongRole = vung.filter((x) => /Story (trước|tiếp theo)/.test(x.ten) && !x.role);
        const nho = vung.filter((x) => x.w < 48 || x.h < 48);
        ket({ tc: "TC-F08-STORY-VUNG-CHAM", screen: "F08.S06", layer: "L26", state: "đang xem story", action: "đọc cây truy cập và kích thước nút", cauHinh: "C1", expected: "hai vùng chạm trước/sau có vai trò nút; nút «Đóng story», «Xoá story» ≥ 48dp", status: khongRole.length === 0 && nho.length === 0 ? "PASS" : "FAIL", evidence: [`EV-F08-XEM-STORY-C1`], ghiChu: `${vung.map((x) => `«${x.ten}» ${x.w}×${x.h}, role ${x.role ?? "không có"}, tabindex ${x.tab ?? "-"}`).join("; ")}; axe: ${(axe.vi ?? []).map((x) => `${x.rule}×${x.so}`).join(", ") || "sạch"}; bàn phím: focus vào «Story tiếp theo» ${banPhim?.coFocus ? "được" : "không"}, Enter ở story cuối: ${banPhim?.truoc} → ${banPhim?.sau}` });
      } else {
        ket({ tc: "TC-MO06-C9", screen: "F08.S06", layer: "L26", state: "giảm chuyển động; 2 story", action: "mở story, đợi", cauHinh: "C9", expected: "không tự chuyển: vạch đứng yên, người xem chạm để đi tiếp", status: d1.length === 2 && JSON.stringify(d1) === JSON.stringify(d2) && d2[0] < 100 ? "PASS" : "FAIL", evidence: [`EV-F08-XEM-STORY-C9`], ghiChu: `vạch sau 1,2 s: ${d1.join("/")}%; sau ${Math.round(ms / 100) / 10} s: ${d2.join("/")}%` });
      }
      await t.context.close();
    }
    // chat-0 deletes one of their own: «Giữ lại» first, then «Xoá».
    const t = await mo("C1", P("chat-0"), "/messages");
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(1500);
    const cuaToi = await t.page.evaluate(() => [...document.querySelectorAll('[data-testid="story-rail"] [aria-label]')].filter((e) => e.getClientRects().length).map((e) => e.getAttribute("aria-label")).find((x) => /của bạn|Story của bạn/i.test(x)));
    if (cuaToi) await bam(t, cdp, cuaToi, { cho: 1500 });
    const dem0 = (await cua())?.stories?.length ?? 0;
    await bam(t, cdp, "Xoá story", { cho: 800 });
    const hoi = { cau: await viTriCau(t.page, /Xoá story này\?/), dialog: await demDialog(t.page), focus: await focusHienTai(t.page), giu: await timNut(t.page, "Giữ lại"), xoa: await timNut(t.page, "Xoá") };
    await chup(t.page, { out: mt.out, id: "EV-F08-XOA-STORY-C1", suKien: t.suKien });
    await bam(t, cdp, "Giữ lại", { cho: 800 });
    const sauGiu = !!(await viTriCau(t.page, /Xoá story này\?/));
    await bam(t, cdp, "Xoá story", { cho: 800 });
    await bam(t, cdp, "Xoá", { cho: 2500 });
    const dem1 = (await cua())?.stories?.length ?? 0;
    const den = await duongDan(t.page);
    ket({ tc: "TC-L26-XOA", screen: "F08.S06", layer: "L26", state: "story của mình", action: "«Xoá story» → «Giữ lại»; lại «Xoá story» → «Xoá»", cauHinh: "C1", expected: "hỏi trước khi xoá, focus vào câu hỏi; «Giữ lại» đóng câu hỏi; «Xoá» xoá đúng một story và rời trình xem; nút ≥ 48dp", status: hoi.cau && !sauGiu && dem1 === dem0 - 1 && hoi.giu?.h >= 48 && hoi.xoa?.h >= 48 ? "PASS" : "FAIL", evidence: ["EV-F08-XOA-STORY-C1"], ghiChu: `vòng «${cuaToi ?? "không thấy"}»; câu hỏi ${hoi.cau ? `y ${hoi.cau.top}` : "không có"}, ${hoi.dialog} hộp thoại, focus «${hoi.focus?.ten ?? "body"}»; «Giữ lại» ${hoi.giu ? `${hoi.giu.w}×${hoi.giu.h}` : "-"}, «Xoá» ${hoi.xoa ? `${hoi.xoa.w}×${hoi.xoa.h}` : "-"}; sau «Giữ lại» câu hỏi ${sauGiu ? "còn" : "đóng"}; story của chat-0 ${dem0} → ${dem1}; ở ${an(den)}` });
    await t.context.close();
  }

  // --------------------------------- the story viewer opened from the rail (L26)
  // chat-1 opens chat-0's story from Tin nhắn and leaves it each way the
  // viewer offers on the web. It is a full-screen route, not a dialog: Esc is
  // read and reported, and does not decide the verdict.
  if (chay("story-dong")) {
    const kq = {};
    for (const cach of ["nut", "back", "esc"]) {
      const t = await mo("C1", P("chat-1"), "/messages");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(1800);
      const vongA = await t.page.evaluate(() => [...document.querySelectorAll('[data-testid="story-rail"] [aria-label]')].filter((e) => e.getClientRects().length).map((e) => e.getAttribute("aria-label")).find((x) => /Chat Test 01/.test(x)));
      if (vongA) await bam(t, cdp, vongA, { cho: 1500 });
      const vao = an(await duongDan(t.page));
      const focusVao = await focusHienTai(t.page);
      if (cach === "nut") await bam(t, cdp, "Đóng story", { cho: 1200 });
      else if (cach === "back") {
        await t.page.goBack();
        await t.page.waitForTimeout(1200);
      } else {
        await t.page.keyboard.press("Escape");
        await t.page.waitForTimeout(1200);
      }
      const roi = an(await duongDan(t.page));
      const conXem = await t.page.evaluate(() => [...document.querySelectorAll('[data-testid="xem-story-screen"]')].some((e) => e.getClientRects().length));
      const focusRoi = await focusHienTai(t.page);
      kq[cach] = { vongA: !!vongA, vao, focusVao: focusVao?.ten ?? "body", roi, conXem, focusRoi: focusRoi?.ten ?? "body" };
      await t.context.close();
    }
    log({ storyDong: kq });
    const dongDuoc = (k) => kq[k].vongA && /\/stories\//.test(kq[k].vao) && /\/messages$/.test(kq[k].roi) && !kq[k].conXem;
    ket({ tc: "TC-L26-VONGDOI", screen: "F08.S06", layer: "L26", state: "chat-1 xem story của Chat Test 01 (một story), mở từ dải trên Tin nhắn", action: "mở story; đóng bằng «Đóng story», Back trình duyệt; thử Esc", cauHinh: "C1", expected: "«Đóng story» và Back về Tin nhắn, trình xem không còn; Esc chỉ ghi nhận (route toàn màn, không phải hộp thoại)", status: dongDuoc("nut") && dongDuoc("back") ? "PASS" : "FAIL", evidence: ["EV-F08-XEM-STORY-C1"], ghiChu: Object.entries(kq).map(([k, v]) => `${k}: vào ${v.vao} (focus «${v.focusVao}»), sau ở ${v.roi}, trình xem ${v.conXem ? "còn" : "không còn"}, focus «${v.focusRoi}»`).join("; ") });
  }

  // -------------------------------------------------------------- posts (S07, S08)
  // The composer starts at «Chỉ mình tôi». The first run posted with that
  // default, so there is one private post of chat-0; the cases that need a
  // reader post a second one for «Bạn bè» (chat-1 is a friend).
  if (chay("bai")) {
    const baiCua = async (muc) => ((await api("GET", `/people/${pA.person_id}/posts`, undefined, pA)).posts ?? []).find((x) => !muc || x.audience === muc);
    const dangBai = async (muc) => {
      const t = await mo("C1", P("chat-0"), "/messages");
      const cdp = await cdpCua(t.page);
      await diToi(t, `/people/${pA.person_id}`);
      await bam(t, cdp, "Đăng bài mới", { cho: 1500 });
      const nutDang0 = await hopNut(t.page, "Đăng");
      const radio = await t.page.evaluate(() => [...document.querySelectorAll('[aria-label^="Mức người đọc"]')].map((e) => ({ ten: e.getAttribute("aria-label").slice(14).trim(), role: e.getAttribute("role"), checked: e.getAttribute("aria-checked") })));
      if (muc) await bam(t, cdp, `Mức người đọc: ${muc}`, { cho: 500 });
      await nhapO(t.page, "Bạn muốn kể gì?", "Cuối tuần vừa rồi cả nhóm đi hồ, trời trong và gió nhẹ. Ai muốn đi lần sau thì bình luận nhé.");
      await chonTep(t, cdp, "Chọn ảnh", { ten: "bai-ngang", w: 640, h: 360 });
      await chup(t.page, { out: mt.out, id: "EV-F08-DANG-BAI-C1", suKien: t.suKien });
      await bam(t, cdp, "Đăng", { cho: 4000 });
      const sau = await duongDan(t.page);
      await t.context.close();
      return { nutDang0, radio, sau };
    };
    if (!(await baiCua("friends"))) {
      const r = await dangBai("Bạn bè");
      ket({ tc: "TC-F08-DANG-BAI", screen: "F08.S07", state: "hồ sơ của mình", action: "«Đăng bài mới» → chữ + ảnh → «Bạn bè» → «Đăng»", cauHinh: "C1", expected: "mức người đọc là nhóm radio có trạng thái chọn; «Đăng» tắt có lý do khi trống; đăng xong về hồ sơ, bài hiện", status: /\/people\//.test(r.sau) && (await baiCua("friends")) && r.radio.every((x) => x.role === "radio" && x.checked !== null) ? "PASS" : "FAIL", evidence: ["EV-F08-DANG-BAI-C1"], ghiChu: `«Đăng» lúc trống: ${r.nutDang0 ? `${r.nutDang0.tat ? "tắt" : "bật"}, dưới «${r.nutDang0.lyDoSau.trim() || "không"}»` : "không thấy"}; mặc định «Chỉ mình tôi»; mức người đọc: ${r.radio.map((x) => `${x.ten} (${x.role}, aria-checked ${x.checked ?? "không có"})`).join(", ")}; sau ở ${an(r.sau)}` });
    }
    // A reader who may not read it: the private post, as chat-1.
    const rieng = await baiCua("only_me");
    if (rieng) {
      const t = await mo("C1", P("chat-1"), "/messages");
      await diToi(t, `/posts/${rieng.id}`);
      const loi = await t.page.evaluate(() => ({ khoi: [...document.querySelectorAll("div,span")].filter((e) => e.children.length === 0 && /^Chưa (mở|đọc) được/.test((e.textContent ?? "").trim()) && e.getClientRects().length).map((e) => e.textContent.trim()), thuLai: [...document.querySelectorAll('[role="button"]')].filter((e) => e.getClientRects().length && /Thử lại/.test(e.innerText ?? "")).length }));
      await chup(t.page, { out: mt.out, id: "EV-F08-BAI-RIENG-C1", suKien: t.suKien });
      ket({ tc: "TC-F08-BAI-KHONG-DANH-CHO", screen: "F08.S08", state: "bài «Chỉ mình tôi» của người khác", action: "mở /posts/[id]", cauHinh: "C1", expected: "một câu nói bài không dành cho mình; không mời «Thử lại» khi thử lại không đổi được gì", status: loi.khoi.length <= 1 && loi.thuLai === 0 ? "PASS" : "FAIL", evidence: ["EV-F08-BAI-RIENG-C1"], ghiChu: `khối lỗi: ${loi.khoi.map((x) => `«${x}»`).join(", ")}; nút «Thử lại»: ${loi.thuLai}` });
      await t.context.close();
    }
    const bai = await baiCua("friends");
    // chat-1 reads, reacts, comments twice, and opens the report sheet (L07).
    const blCua = async () => ((await api("GET", `/posts/${bai.id}/comments`, undefined, pA)).comments ?? []);
    const t = await mo("C1", P("chat-1"), "/messages");
    const cdp = await cdpCua(t.page);
    await diToi(t, `/posts/${bai.id}`);
    const camXuc = await t.page.evaluate(() => [...document.querySelectorAll('[role="button"],button')].filter((e) => e.getClientRects().length && e.getBoundingClientRect().width < 60 && /^[\u{1F300}-\u{1FAFF}\u2600-\u27BF]/u.test((e.innerText ?? "").trim())).map((e) => { const r = e.getBoundingClientRect(); return { ten: e.getAttribute("aria-label") ?? e.innerText.trim(), w: Math.round(r.width), h: Math.round(r.height), pressed: e.getAttribute("aria-pressed") }; }));
    await chup(t.page, { out: mt.out, id: "EV-F08-BAI-C1", suKien: t.suKien });
    const guiTat = await hopNut(t.page, "Gửi bình luận");
    if ((await blCua()).length < 2) {
      for (const noi of ["Mình đi! Lần sau nhớ gọi mình từ tối hôm trước để còn xin nghỉ buổi chiều, với cả hỏi giúp mình quán nướng hôm trước có còn mở tới khuya không, lần này mình muốn ngồi lâu hơn một chút.", "Đẹp quá"]) {
        await nhapO(t.page, "Ô viết bình luận", noi);
        await t.page.waitForTimeout(300);
        await bam(t, cdp, "Gửi bình luận", { cho: 2000 });
      }
    }
    const tran = await t.page.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1);
    ket({ tc: "TC-F08-CAM-XUC-BAI", screen: "F08.S08", state: "bài «Bạn bè» của Chat Test 01", action: "đọc hàng cảm xúc", cauHinh: "C1", expected: "mỗi nút cảm xúc ≥ 48dp (hoặc 44) và báo trạng thái đã chọn", status: camXuc.length > 0 && camXuc.every((x) => x.w >= 44 && x.h >= 44) ? "PASS" : "FAIL", evidence: ["EV-F08-BAI-C1"], ghiChu: `${camXuc.length} nút: ${camXuc.map((x) => `«${x.ten}» ${x.w}×${x.h}${x.pressed !== null ? `, aria-pressed ${x.pressed}` : ""}`).join("; ")}` });
    const kq = {};
    for (const cach of ["x", "esc", "nen", "back"]) {
      if (!/\/posts\//.test(await duongDan(t.page))) await diToi(t, `/posts/${bai.id}`);
      const nutBC = await bam(t, cdp, "Báo cáo bài này", { cho: 100 });
      const moRa = await choDialog(t.page, { co: true });
      await t.page.waitForTimeout(900);
      const f = await focusHienTai(t.page);
      if (cach === "x") await chup(t.page, { out: mt.out, id: "EV-F08-BAO-CAO-BAI-C1", suKien: t.suKien });
      await dong(t.page, cdp, cach);
      await t.page.waitForTimeout(900);
      kq[cach] = { nut: !!nutBC, mo: moRa.ok, focus: f?.ten, con: await demDialog(t.page), duong: an(await duongDan(t.page)) };
    }
    ket({ tc: "TC-L07-VONGDOI", screen: "F08.S08", layer: "L07", state: "bài của người khác", action: "«Báo cáo bài này»; đóng bằng X, Esc, nền, Back", cauHinh: "C1", expected: "focus vào sheet; mọi cách đóng đều ở lại bài; Back đóng sheet mà không rời bài", status: Object.values(kq).every((x) => x.mo && x.con === 0 && /\/posts\//.test(x.duong)) ? "PASS" : "FAIL", issue: kq.back.mo && !/\/posts\//.test(kq.back.duong) ? "UI-038" : null, evidence: ["EV-F08-BAO-CAO-BAI-C1"], ghiChu: Object.entries(kq).map(([k, v]) => `${k}: ${v.mo ? "mở" : "không mở"}, sau đó ${v.con} hộp thoại, ở ${v.duong}, focus lúc mở «${v.focus}»`).join("; ") });
    ket({ tc: "TC-F08-BINH-LUAN-BAI", screen: "F08.S08", state: "bài «Bạn bè» của Chat Test 01", action: "gõ và gửi bình luận 190 ký tự và 7 ký tự", cauHinh: "C1", expected: "nút gửi chưa dùng được thì ẩn hoặc có lý do (ADR-0038); bình luận xuống dòng, không tràn", status: !tran && (await blCua()).length >= 2 && !(guiTat?.tat && !guiTat.lyDoSau.trim()) ? "PASS" : "FAIL", evidence: ["EV-F08-BAI-C1"], ghiChu: `«Gửi bình luận» lúc trống: ${guiTat ? `${guiTat.w}×${guiTat.h}, ${guiTat.tat ? "tắt" : "bật"}, dưới «${guiTat.lyDoSau.trim() || "không"}»` : "ẩn"}; bình luận trên máy chủ ${(await blCua()).length}; tràn ngang ${tran}` });
    await t.context.close();
    // The author deletes a comment of chat-1: size of the target, and whether it asks.
    const t2 = await mo("C1", P("chat-0"), "/messages");
    const cdp2 = await cdpCua(t2.page);
    await diToi(t2, `/posts/${bai.id}`);
    const thung = await t2.page.evaluate(() => [...document.querySelectorAll('[aria-label^="Xoá bình luận"]')].filter((e) => e.getClientRects().length).map((e) => { e.scrollIntoView({ block: "center" }); const r = e.getBoundingClientRect(); return { ten: e.getAttribute("aria-label").slice(0, 50), w: Math.round(r.width), h: Math.round(r.height) }; }));
    await chup(t2.page, { out: mt.out, id: "EV-F08-XOA-BL-C1", suKien: t2.suKien });
    const blTruoc = (await blCua()).length;
    const ngan = thung.find((x) => /Đẹp quá/.test(x.ten));
    if (ngan) await bam(t2, cdp2, ngan.ten, { cho: 2000 });
    const hoi = { dialog: await demDialog(t2.page), cau: await viTriCau(t2.page, /Xoá bình luận này|chắc|Chắc/) };
    const blSau = (await blCua()).length;
    ket({ tc: "TC-F08-XOA-BINH-LUAN", screen: "F08.S08", state: "tác giả bài, bình luận của người khác", action: "chạm thùng rác của bình luận «Đẹp quá»", cauHinh: "C1", expected: "vùng bấm ≥ 48dp (hoặc 44); hỏi trước khi xoá như xoá tin nhắn ở chat", status: ngan && ngan.w >= 44 && ngan.h >= 44 && (hoi.dialog > 0 || hoi.cau) ? "PASS" : "FAIL", evidence: ["EV-F08-XOA-BL-C1"], ghiChu: `thùng rác: ${thung.map((x) => `${x.w}×${x.h}`).join(", ") || "không thấy"}; sau một chạm: ${hoi.dialog} hộp thoại, câu hỏi ${hoi.cau ? `«${hoi.cau.text}»` : "không"}; bình luận trên máy chủ ${blTruoc} → ${blSau}` });
    await t2.context.close();
  }

  // ---------------------------------------------------------- achievements (S09)
  if (chay("thanh-tich")) {
    for (const [ten, cfg] of [["dalat-0", "C1"], ["chat-0", "C1"]]) {
      const t = await mo(cfg, P(ten), "/profile");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(1200);
      const vao = await bam(t, cdp, "Thành tích", { batDau: true, cho: 2000 });
      const den = await duongDan(t.page);
      const chu = await chuDu(t.page);
      await chup(t.page, { out: mt.out, id: `EV-F08-THANH-TICH-${ten}-C1`, suKien: t.suKien });
      ket({ tc: `TC-F08-THANH-TICH-${ten.toUpperCase()}`, screen: "F08.S09", state: ten === "dalat-0" ? "Team Đà Lạt (có chi, kèo, kỷ niệm)" : "chat-0 (nhóm chat-test, sổ đôi)", action: "Hồ sơ → «Thành tích»", cauHinh: cfg, expected: "mở từ Hồ sơ; con dấu đọc được, chưa có thì nói cách có", status: vao && /\/achievements/.test(den) && !/[A-Z_]{6,}/.test(chu) ? "PASS" : "FAIL", evidence: [`EV-F08-THANH-TICH-${ten}-C1`], ghiChu: `nút ${vao ? `«${vao.ten}» ${vao.w}×${vao.h}` : "không thấy"}; tới ${an(den)}; «${catChu(chu, 240)}»` });
      await t.context.close();
    }
  }

  // ------------------------------- the badge hero on narrow phones (S09, M8)
  // The hero is one row: the stamp (80), the words (flex 1) and, the first
  // time a badge is seen opened, Nếp's M8 box (112). A line break between two
  // letters of one word is read from each character's own line (Range rects).
  if (chay("tem-hep")) {
    const doHang = (page) =>
      page.evaluate(() => {
        const tem = [...document.querySelectorAll('[aria-label^="Huy hiệu "]')].find((x) => x.getClientRects().length);
        if (!tem) return null;
        let hang = null;
        for (let n = tem.parentElement; n && !hang; n = n.parentElement) if (getComputedStyle(n).flexDirection === "row" && n.children.length >= 2) hang = n;
        if (!hang) return null;
        const cot = [...hang.children].find((c) => /mới mở|đã mở/.test((c.innerText ?? "").toLowerCase()));
        const chuCua = (e) => [...e.childNodes].find((n) => n.nodeType === 3 && n.textContent.trim());
        const dong = (cot ? [...cot.querySelectorAll("*")] : []).filter((e) => chuCua(e) && e.getClientRects().length).map((e) => {
          const node = chuCua(e);
          const s = node.textContent;
          const tren = [...s].map((_, i) => {
            const r = document.createRange();
            r.setStart(node, i);
            r.setEnd(node, i + 1);
            const q = r.getClientRects()[0];
            return q ? Math.round(q.top) : null;
          });
          let soDong = 1;
          const beGiua = [];
          for (let i = 1; i < s.length; i += 1) {
            if (tren[i] === null || tren[i - 1] === null || tren[i] <= tren[i - 1] + 2) continue;
            soDong += 1;
            if (!/\s/.test(s[i - 1]) && !/\s/.test(s[i])) beGiua.push(`${s.slice(Math.max(0, i - 3), i)}|${s.slice(i, i + 3)}`);
          }
          return { chu: s.slice(0, 40), soDong, beGiua };
        });
        return { the: Math.round(hang.getBoundingClientRect().width), con: [...hang.children].map((c) => Math.round(c.getBoundingClientRect().width)), cot: cot ? Math.round(cot.getBoundingClientRect().width) : null, dong };
      });
    const moTaHang = (h) => (h ? `thẻ ${h.the}px, ba ô ${h.con.join(" + ")}, cột chữ ${h.cot}px; ${h.dong.map((d) => `«${d.chu}» ${d.soDong} dòng${d.beGiua.length ? `, bẻ giữa chữ: ${d.beGiua.join(" ")}` : ""}`).join("; ")}` : "không thấy thẻ nổi bật");
    for (const cfg of ["C1", "C2", "C3", "C4", "C5", "C9"]) {
      const t = await mo(cfg, P("chat-0"), "/achievements");
      await t.page.waitForTimeout(1500);
      const dau = await doHang(t.page);
      if (cfg === "C2") await chup(t.page, { out: mt.out, id: "EV-F08-TEM-HEP-C2", suKien: t.suKien });
      // A second look on the same phone: the badge is no longer new, so Nếp's
      // box is gone and the words get the room back.
      await t.page.reload();
      await choOn(t.page, { mang: t.mang });
      await t.page.waitForTimeout(1500);
      const sau = await doHang(t.page);
      if (cfg === "C2") await chup(t.page, { out: mt.out, id: "EV-F08-TEM-HEP-LAN2-C2", suKien: t.suKien });
      log({ temHep: cfg, dau, sau });
      const be = (h) => (h?.dong ?? []).some((d) => d.beGiua.length);
      ket({ tc: "TC-F08-TEM-HEP", screen: "F08.S09", state: "chat-0 có huy hiệu vừa mở («Mới mở», Nếp M8)", action: "mở «Thành tích» lần đầu trên máy này, rồi tải lại (lần xem thứ hai)", cauHinh: cfg, expected: "tên huy hiệu và điều kiện xuống dòng giữa hai từ, không bẻ đôi một từ, ở mọi cỡ điện thoại", status: dau && sau ? (be(dau) || be(sau) ? "FAIL" : "PASS") : "BLOCKED", evidence: cfg === "C2" ? ["EV-F08-TEM-HEP-C2", "EV-F08-TEM-HEP-LAN2-C2"] : [], ghiChu: `lần đầu: ${moTaHang(dau)} | lần hai: ${moTaHang(sau)}` });
      await t.context.close();
    }
  }

  // --------------------------------------- no session: the demo behind real links
  if (chay("khong-phien")) {
    const o = await keoAlbum();
    const kq = [];
    for (const [id, path] of [["tuong", `/groups/${G20}/wall`], ["album-nhom", `/groups/${G20}/album`], ["album-keo", `/trips/${o?.id}/album?ctx=${G20}`], ["tha", "/moments/new"], ["story-moi", "/stories/new"], ["bai-moi", "/posts/new"], ["thanh-tich", "/achievements"]]) {
      const t = await mo("C1", null, path);
      await t.page.waitForTimeout(1500);
      const den = await duongDan(t.page);
      const chu = await chuDu(t.page);
      const demo = /Dữ liệu demo|[Dd]emo|[Tt]rải nghiệm/.test(chu);
      await chup(t.page, { out: mt.out, id: `EV-F08-KHONG-PHIEN-${id}-C1`, suKien: t.suKien });
      kq.push({ id, path: an(path), den: an(den), demo, dau: catChu(chu, 90) });
      await t.context.close();
    }
    log({ khongPhien: kq });
    ket({ tc: "TC-F08-KHONG-PHIEN", screen: "F08.S01", state: "không có phiên", action: "mở thẳng 7 link của F08", cauHinh: "C1", expected: "tới cửa vào, hoặc bản trải nghiệm có nhãn «Dữ liệu demo»", status: kq.every((x) => /welcome|login/.test(x.den) || x.demo) ? "PASS" : "FAIL", evidence: kq.map((x) => `EV-F08-KHONG-PHIEN-${x.id}-C1`), ghiChu: kq.map((x) => `${x.path} → ${x.den}${x.demo ? " (có nhãn demo)" : ""}: «${x.dau}»`).join("; ") });
    // Sharing a moment there: what does the screen claim, and what reaches the API?
    const t = await mo("C1", null, "/moments/new");
    const cdp = await cdpCua(t.page);
    const ghiDi = [];
    t.page.on("request", (r) => { if (r.url().startsWith(mt.api) && r.method() !== "GET") ghiDi.push(`${r.method()} ${an(new URL(r.url()).pathname)}`); });
    await t.page.waitForTimeout(1200);
    const nutChon = await t.page.evaluate(() => [...document.querySelectorAll('[role="button"]')].filter((e) => e.getClientRects().length).map((e) => (e.getAttribute("aria-label") || e.innerText || "").replace(/[-]/g, "").replace(/\s+/g, " ").trim()).slice(0, 12));
    const chon = await chonTep(t, cdp, nutChon.find((x) => /chọn|Chọn/.test(x)) ?? "Chọn ảnh", { ten: "demo", w: 480, h: 360 });
    const nutGui = nutChon.find((x) => /Chia sẻ|Đăng|Gửi|Thả/.test(x));
    if (nutGui) await bam(t, cdp, nutGui, { cho: 3000 });
    const sau = { duong: an(await duongDan(t.page)), chu: catChu(await chuDu(t.page), 200) };
    await chup(t.page, { out: mt.out, id: "EV-F08-KHONG-PHIEN-THA-GUI-C1", suKien: t.suKien });
    ket({ tc: "TC-F08-KHONG-PHIEN-THA", screen: "F08.S04", state: "không có phiên", action: "/moments/new → chọn ảnh → nút gửi", cauHinh: "C1", expected: "không báo đã đăng khi không có gì tới máy chủ; mời đăng nhập", status: ghiDi.length > 0 || !/[Đđ]ã (đăng|chia sẻ|thả|gửi)|lên tường/.test(sau.chu) ? "PASS" : "FAIL", evidence: ["EV-F08-KHONG-PHIEN-THA-GUI-C1"], ghiChu: `nút trên màn: ${nutChon.join(" · ")}; bộ chọn ${chon ? "mở" : "không mở"}; bấm «${nutGui ?? "không có"}»; sau: ${sau.duong} «${sau.chu}»; lệnh ghi tới API: ${ghiDi.length ? ghiDi.join(", ") : "0"}` });
    await t.context.close();
  }

  // ---------------------------------------------------------------- cold back
  if (chay("lanh")) {
    const o = await keoAlbum();
    const bai = ((await api("GET", `/people/${pA.person_id}/posts`, undefined, pA)).posts ?? [])[0];
    const kq = [];
    for (const [id, path] of [["tuong", `/groups/${G20}/wall`], ["album-nhom", `/groups/${G20}/album`], ["album-keo", `/trips/${o?.id}/album?ctx=${G20}`], ["tha", `/moments/new?ctx=${G20}`], ["story-moi", "/stories/new"], ["bai-moi", "/posts/new"], ["bai", bai ? `/posts/${bai.id}` : null], ["thanh-tich", "/achievements"]]) {
      if (!path) continue;
      const t = await mo("C1", P("chat-0"), path);
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(1200);
      const truoc = await duongDan(t.page);
      const nut = await bam(t, cdp, "Quay lại", { cho: 1200 });
      const sau = await duongDan(t.page);
      kq.push({ id, truoc: an(truoc), sau: an(sau), nut: !!nut });
      await t.context.close();
    }
    for (const x of kq) ket({ tc: `TC-F08-BACK-LANH-${x.id.toUpperCase()}`, screen: "F08.S01", state: "mở thẳng bằng link", action: "chạm «Quay lại»", cauHinh: "C1", expected: "tới một màn hợp lý", status: x.nut && x.sau !== x.truoc ? "PASS" : "FAIL", issue: x.nut && x.sau !== x.truoc ? null : "UI-018", evidence: [], ghiChu: `${x.truoc} → ${x.sau}${x.nut ? "" : " (không có nút «Quay lại»)"}` });
    // The story viewer, cold: «Đóng story» is router.back() too.
    const t = await mo("C1", P("chat-1"), `/stories/${pA.person_id}`);
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(1500);
    const truoc = await duongDan(t.page);
    await bam(t, cdp, "Đóng story", { cho: 1200 });
    const sau = await duongDan(t.page);
    ket({ tc: "TC-F08-STORY-DONG-LANH", screen: "F08.S06", layer: "L26", state: "mở story bằng link", action: "«Đóng story»", cauHinh: "C1", expected: "rời trình xem story", status: sau !== truoc ? "PASS" : "FAIL", issue: sau !== truoc ? null : "UI-018", evidence: [], ghiChu: `${an(truoc)} → ${an(sau)}` });
    await t.context.close();
  }

  // ------------------------------------------------------------------- 503s
  if (chay("loi")) {
    const o = await keoAlbum();
    const bai = ((await api("GET", `/people/${pA.person_id}/posts`, undefined, pA)).posts ?? [])[0];
    for (const [id, path, mau] of [["tuong", `/groups/${G20}/wall`, /\/contexts\/[^/]+\/memories(\?|$)/], ["album-nhom", `/groups/${G20}/album`, /\/contexts\/[^/]+\/albums(\?|$)/], ["album-keo", `/trips/${o?.id}/album?ctx=${G20}`, /\/contexts\/[^/]+\/albums\/[^/?]+(\?|$)/], ["bai", bai ? `/posts/${bai.id}` : null, /\/posts\/[^/?]+(\?|$)/], ["story", `/stories/${pA.person_id}`, /\/stories(\?|$)/]]) {
      if (!path) continue;
      const t = await mo("C1", P(id === "story" ? "chat-1" : "chat-0"), "/messages");
      const cdp = await cdpCua(t.page);
      await loiMayChu(t.page, mau);
      await diToi(t, path);
      await t.page.waitForTimeout(1500);
      const chu = await chuDu(t.page);
      const thuLai = await timNut(t.page, "Thử lại");
      await chup(t.page, { out: mt.out, id: `EV-F08-503-${id}-C1`, suKien: t.suKien });
      await goHet(t.page);
      if (thuLai) await bam(t, cdp, "Thử lại", { cho: 2500 });
      const sau = await chuDu(t.page);
      const cau = (chu.match(/Chưa [^.]*\.?[^.]*\./) ?? [])[0] ?? catChu(chu, 160);
      ket({ tc: `TC-F08-503-${id.toUpperCase()}`, screen: "F08.S01", state: "máy chủ trả 503 cho lần đọc", action: "mở màn; máy chủ ổn; «Thử lại»", cauHinh: "C1", expected: "câu tiếng Việt nói không đọc được và có «Thử lại»; thử lại thì có nội dung", status: /Chưa (đọc|mở)/.test(chu) && thuLai && !/Chưa (đọc|mở) được/.test(sau) && !/audit_injected|HTTP|Error/.test(chu) ? "PASS" : "FAIL", evidence: [`EV-F08-503-${id}-C1`], ghiChu: `khi 503: «${catChu(cau, 180)}»; nút thử lại ${thuLai ? "có" : "không"}; sau thử lại: «${catChu(sau, 100)}»` });
      await t.context.close();
    }
  }

  // ------------------------------------------------------------- low window
  if (chay("c8")) {
    const kq = [];
    for (const [id, path, nutChinh] of [["tha", `/moments/new?ctx=${G20}`, "Chia sẻ ngay vào nhóm"], ["story-moi", "/stories/new", "Đăng story"], ["bai-moi", "/posts/new", "Đăng"]]) {
      const t = await mo("C8", P("chat-0"), "/messages");
      await diToi(t, path);
      const r = await t.page.evaluate((nutChinh) => {
        const ten = (x) => (x.getAttribute("aria-label") || (x.innerText ?? "")).replace(/[-]/g, "").replace(/\s+/g, " ").trim();
        const n = [...document.querySelectorAll('[role="button"],button')].find((x) => ten(x) === nutChinh && x.getClientRects().length);
        const q = n?.getBoundingClientRect();
        return { nut: q ? { top: Math.round(q.top), bottom: Math.round(q.bottom), trongMan: q.top >= 0 && q.bottom <= innerHeight } : null, cao: innerHeight };
      }, nutChinh);
      await chup(t.page, { out: mt.out, id: `EV-F08-C8-${id}`, suKien: t.suKien });
      kq.push({ id, nutChinh, ...r });
      await t.context.close();
    }
    const t = await mo("C8", P("chat-0"), "/messages");
    const cdp = await cdpCua(t.page);
    await diToi(t, `/groups/${G20}/wall`);
    await bam(t, cdp, "Check-in", { cho: 2000 });
    const sh = await t.page.evaluate(() => { const d = [...document.querySelectorAll('[role="dialog"]')].filter((x) => x.getClientRects().length).pop(); const b = d?.getBoundingClientRect(); return b ? { phan: Math.round((b.height / innerHeight) * 100), top: Math.round(b.top) } : null; });
    await chup(t.page, { out: mt.out, id: "EV-F08-C8-checkin", suKien: t.suKien });
    await t.context.close();
    log({ c8: kq, sheet: sh });
    ket({ tc: "TC-F08-C8", screen: "F08.S04", state: "cửa sổ 390×460 (proxy bàn phím)", action: "mở 3 form đăng và sheet Check-in", cauHinh: "C8", expected: "nút chính của form thấy được (chân trang) hoặc cuộn tới được; sheet ≤ 82%", status: kq.every((x) => x.nut) && sh && sh.phan <= 82 ? "PASS" : "FAIL", evidence: ["EV-F08-C8-tha", "EV-F08-C8-checkin"], ghiChu: `${kq.map((x) => `${x.id}: «${x.nutChinh}» ${x.nut ? `y ${x.nut.top}–${x.nut.bottom} ${x.nut.trongMan ? "trong" : "ngoài"} cửa sổ ${x.cao}` : "không thấy"}`).join("; ")}; sheet Check-in ${sh ? `${sh.phan}% cửa sổ, đỉnh y ${sh.top}` : "không mở"}` });
  }

  // ----------------------- a publish button past the fold, reached by touch
  // Story and post keep their button in the scrolling page, not in a footer.
  // Drag the page up in its left margin (never on a field) until it stops,
  // then read the button's box and what sits on top of its centre.
  if (chay("voi-toi")) {
    const kq = [];
    for (const cfg of ["C8", "C2"]) {
      for (const [id, path, nutChinh] of [["story-moi", "/stories/new", "Đăng story"], ["bai-moi", "/posts/new", "Đăng"]]) {
        const t = await mo(cfg, P("chat-0"), "/messages");
        const cdp = await cdpCua(t.page);
        await diToi(t, path);
        const vp = t.page.viewportSize();
        for (let i = 0; i < 6; i += 1) {
          await keo(cdp, { x: 8, y: Math.round(vp.height * 0.8) }, { x: 8, y: Math.round(vp.height * 0.2) }, { ms: 300, buoc: 12 });
          await t.page.waitForTimeout(250);
        }
        const r = await t.page.evaluate((nutChinh) => {
          const ten = (x) => (x.getAttribute("aria-label") || (x.innerText ?? "")).replace(/[-]/g, "").replace(/\s+/g, " ").trim();
          const n = [...document.querySelectorAll('[role="button"],button')].find((x) => ten(x) === nutChinh && x.getClientRects().length);
          if (!n) return null;
          const q = n.getBoundingClientRect();
          const tren = document.elementFromPoint(q.left + q.width / 2, Math.min(innerHeight - 1, q.top + q.height / 2));
          const sau = n.parentElement?.nextElementSibling ?? n.nextElementSibling;
          const s = sau?.getBoundingClientRect();
          return { top: Math.round(q.top), bottom: Math.round(q.bottom), cao: innerHeight, tron: q.top >= 0 && q.bottom <= innerHeight, trenLaNut: !!tren && (tren === n || n.contains(tren)), lyDo: sau ? { chu: (sau.innerText ?? "").replace(/[\uE000-\uF8FF]/g, "").replace(/\s+/g, " ").trim().slice(0, 80), bottom: Math.round(s.bottom) } : null };
        }, nutChinh);
        if (cfg === "C8") await chup(t.page, { out: mt.out, id: `EV-F08-C8-VOI-TOI-${id}`, suKien: t.suKien });
        kq.push({ cfg, id, nutChinh, r });
        await t.context.close();
      }
    }
    // «Thả khoảnh khắc» keeps its button in a fixed footer instead: the last
    // lines of the page must scroll clear of it.
    const cuonTha = {};
    for (const cfg of ["C2", "C8"]) {
      const t = await mo(cfg, P("chat-0"), "/messages");
      const cdp = await cdpCua(t.page);
      await diToi(t, `/moments/new?ctx=${G20}`);
      const vp = t.page.viewportSize();
      for (let i = 0; i < 6; i += 1) {
        await keo(cdp, { x: 8, y: Math.round(vp.height * 0.7) }, { x: 8, y: Math.round(vp.height * 0.15) }, { ms: 300, buoc: 12 });
        await t.page.waitForTimeout(250);
      }
      cuonTha[cfg] = await t.page.evaluate(() => {
        const hien = (x) => x.getClientRects().length > 0;
        const nut = [...document.querySelectorAll('[role="button"],button')].find((x) => /Chia sẻ ngay vào nhóm/.test(x.innerText ?? "") && hien(x));
        const chu = (re) => [...document.querySelectorAll("div,span")].find((e) => [...e.childNodes].some((n) => n.nodeType === 3 && re.test(n.textContent)) && hien(e));
        const conLai = chu(/ký tự còn lại/);
        const ghiChu = chu(/^Đăng vào /);
        const b = (e) => (e ? Math.round(e.getBoundingClientRect().bottom) : null);
        return { dinhNut: nut ? Math.round(nut.getBoundingClientRect().top) : null, conLai: b(conLai), ghiChu: b(ghiChu), cao: innerHeight };
      });
      if (cfg === "C2") await chup(t.page, { out: mt.out, id: "EV-F08-THA-CUON-C2", suKien: t.suKien });
      await t.context.close();
    }
    log({ voiToi: kq, cuonTha });
    for (const [cfg, r] of Object.entries(cuonTha)) {
      ket({ tc: "TC-F08-THA-CUON", screen: "F08.S04", state: "form trống, nút ở chân trang cố định", action: "kéo trang lên bằng ngón tay ở lề trái tới khi dừng", cauHinh: cfg, expected: "dòng cuối của trang (câu «Đăng vào …») và «… ký tự còn lại» cuộn ra khỏi dưới chân trang", status: r.dinhNut !== null && r.ghiChu !== null && r.ghiChu <= r.dinhNut && r.conLai !== null && r.conLai <= r.dinhNut ? "PASS" : "FAIL", evidence: cfg === "C2" ? ["EV-F08-THA-CUON-C2"] : [], ghiChu: `đỉnh nút chân trang y ${r.dinhNut}; đáy «ký tự còn lại» y ${r.conLai}; đáy câu «Đăng vào …» y ${r.ghiChu}; cửa sổ cao ${r.cao}` });
    }
    for (const cfg of ["C8", "C2"]) {
      const cua = kq.filter((x) => x.cfg === cfg);
      ket({ tc: "TC-F08-VOI-TOI", screen: "F08.S05", state: "form trống (story, bài)", action: "kéo trang lên bằng ngón tay ở lề trái tới khi dừng", cauHinh: cfg, expected: "nút chính và câu lý do dưới nó nằm trọn trong cửa sổ, chạm được (không gì đè lên tâm nút)", status: cua.every((x) => x.r && x.r.tron && x.r.trenLaNut && (!x.r.lyDo || x.r.lyDo.bottom <= x.r.cao)) ? "PASS" : "FAIL", evidence: cfg === "C8" ? ["EV-F08-C8-VOI-TOI-story-moi", "EV-F08-C8-VOI-TOI-bai-moi"] : [], ghiChu: cua.map((x) => `${x.id}: «${x.nutChinh}» ${x.r ? `y ${x.r.top}–${x.r.bottom} trong cửa sổ ${x.r.cao}, ${x.r.tron ? "trọn" : "không trọn"}, tâm ${x.r.trenLaNut ? "là nút" : "bị đè"}; dưới nó «${x.r.lyDo?.chu ?? "không"}» tới y ${x.r.lyDo?.bottom ?? "?"}` : "không thấy"}`).join("; ") });
    }
  }

  // ------------------------------------------------------------------ tablet
  if (chay("tablet")) {
    const o = await keoAlbum();
    for (const cfg of ["C6", "C7"]) {
      const t = await mo(cfg, P("chat-0"), "/messages");
      await diToi(t, `/groups/${G20}/wall`);
      // The prints are found by their alt text (the captions this script posted), laid out size.
      const w = await t.page.evaluate(() => [...document.querySelectorAll("img")].filter((x) => ["Hoàng hôn trên hồ", "Ảnh của nhóm"].includes(x.getAttribute("alt") ?? "") && x.getClientRects().length).map((x) => ({ alt: x.getAttribute("alt"), w: x.offsetWidth, h: x.offsetHeight, left: Math.round(x.getBoundingClientRect().left), cao: innerHeight })));
      await chup(t.page, { out: mt.out, id: `EV-F08-TUONG-${cfg}`, suKien: t.suKien });
      await diToi(t, `/trips/${o.id}/album?ctx=${G20}`);
      const a = await t.page.evaluate(() => [...document.querySelectorAll('[aria-label^="Mở ảnh"]')].filter((e) => e.getClientRects().length).map((e) => { const r = e.getBoundingClientRect(); return `${Math.round(r.width)}×${Math.round(r.height)}`; }));
      await chup(t.page, { out: mt.out, id: `EV-F08-ALBUM-${cfg}`, suKien: t.suKien });
      ket({ tc: "TC-F08-TABLET", screen: "F08.S01", state: "tường và album có ảnh", action: "mở ở tablet", cauHinh: cfg, expected: "ảnh trên tường có trần bề ngang như các cột nội dung khác (≤ 640), một ảnh dọc không cao hơn một màn; lưới album dùng được bề ngang", status: w.length > 0 && w.every((x) => x.w <= 640 && x.h <= x.cao) ? "PASS" : "FAIL", evidence: [`EV-F08-TUONG-${cfg}`, `EV-F08-ALBUM-${cfg}`], ghiChu: `ảnh trên tường: ${w.map((x) => `«${x.alt}» ${x.w}×${x.h}`).join(", ") || "không thấy"} (cửa sổ cao ${w[0]?.cao}); album: ${a.join(", ")}` });
      await t.context.close();
    }
  }
} finally {
  await mt.dong();
}
