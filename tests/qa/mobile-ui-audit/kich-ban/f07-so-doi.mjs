/* F07, the two-person notebook: choosing a person, proposing the notebook and
 * agreeing to it on the other side (M6), a week's sheet drafted, edited, sent,
 * answered with a new version and agreed (E4), the notebook's settings sheet
 * and its sub-sheets, and «Rủ <tên> tới đây» from a place.
 *
 *   node kich-ban/f07-so-doi.mjs [--chi chon-nguoi,lap-so,ru,cai-dat,vong-doi,ru-toi-day,m6-c9,m6-khung,c8,lanh,loi,tablet]
 *
 * Who writes what, all on the local stack: A = chat-0 and B = chat-1, friends
 * with a pair already open, get a notebook and one sheet (the week allows
 * three). For M6 under Reduce Motion a second pair is set up over the API:
 * chat-2 and chat-3 become friends and open their pair; chat-2 then proposes
 * on its own screen (the proposer's waiting state is read there) and the
 * agreeing side, chat-3, is watched at C9. Nothing here closes a notebook
 * (the close sheet is opened and left). Team Đà Lạt is not touched.
 */
import { createHash, randomUUID } from "node:crypto";
import { writeFileSync } from "node:fs";
import { join } from "node:path";

import { chayAxe } from "../thu-vien/axe.mjs";
import { cauHinh } from "../thu-vien/cau-hinh.mjs";
import { chup, ghepAnh } from "../thu-vien/chup.mjs";
import { cdpCua, cham } from "../thu-vien/cu-chi.mjs";
import { choOn, duongDan } from "../thu-vien/dieu-huong.mjs";
import { choDialog, demDialog, dong, focusHienTai, inertConLai, luoiChamTrang, soLuoi } from "../thu-vien/lop-phu.mjs";
import { batDauLayMau, ghepKhung, ketThucLayMau, phanTich, quayKhung } from "../thu-vien/chuyen-dong.mjs";
import { goHet, loiMayChu } from "../thu-vien/mang.mjs";
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
  so.ghi({ feature: "F07", nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, ...rec });
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
const ngCanh = async (phien) => (await api("GET", "/people/me/contexts", undefined, phien)).contexts ?? [];
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
    return { top: Math.round(r.top), bottom: Math.round(r.bottom), trongMan: r.bottom > 0 && r.top < innerHeight, text: e.textContent.trim().slice(0, 160) };
  }, re.source);
const chuTrang = (page) => page.evaluate(() => (document.body.innerText ?? "").replace(/\s+/g, " ").slice(0, 500));
const coTestId = (page, id) => page.evaluate((id) => { const e = document.querySelector(`[data-testid="${id}"]`); return !!e && e.getClientRects().length > 0; }, id);

/** Screenshots of one region every 150 ms; first, middle and last kept for looking. */
const layMau = async (page, selector, soMau = 16) => {
  const mau = [];
  const t0 = Date.now();
  for (let i = 0; i < soMau; i += 1) {
    const hop = await page.evaluate((s) => { const e = document.querySelector(s); if (!e) return null; const r = e.getBoundingClientRect(); return { x: Math.max(0, r.left - 4), y: Math.max(0, r.top - 4), width: Math.min(innerWidth, r.width + 8), height: Math.min(innerHeight, r.height + 8) }; }, selector);
    if (hop && hop.width > 0 && hop.height > 0) {
      const buf = await page.screenshot({ clip: hop });
      mau.push({ ms: Date.now() - t0, h: createHash("sha1").update(buf).digest("hex").slice(0, 6), buf });
    }
    await page.waitForTimeout(150);
  }
  return { mau, khac: new Set(mau.map((m) => m.h)).size, doiCuoi: mau.reduce((m, b, i) => (i > 0 && b.h !== mau[i - 1].h ? b.ms : m), 0) };
};
const ghepMau = async (kq, id, tieuDe) => {
  if (!kq.mau.length) return null;
  const chon = [0, Math.floor(kq.mau.length / 3), Math.floor((2 * kq.mau.length) / 3), kq.mau.length - 1].filter((v, i, a) => a.indexOf(v) === i);
  const anh = chon.map((i) => {
    const f = join(mt.out, "raw", `${id}-${i}.png`);
    writeFileSync(f, kq.mau[i].buf);
    return { file: f, nhan: `${kq.mau[i].ms} ms` };
  });
  await ghepAnh(mt.browser, anh, join(mt.out, "jpg", `${id}.jpg`), { cao: 360, tieuDe });
  return id;
};

const capCua = async (phienA, tenB) => (await ngCanh(phienA)).find((c) => c.kind === "pair" && (c.display_name === tenB || c.counterpart?.display_name === tenB));
const soCua = (cap, phien) => api("GET", `/contexts/${cap.id}/notebook`, undefined, phien);
const toCua = async (cap, phien) => (await api("GET", `/contexts/${cap.id}/papers`, undefined, phien)).papers ?? [];
/** chat-N shows as «Chat Test NN», counted from 01. */
const tenHien = (ten) => `Chat Test ${String(Number(ten.split("-")[1]) + 1).padStart(2, "0")}`;
/** Friends and an open pair for two chat-test people, over the API; the pair as A sees it. */
const lapCap = async (tenA, tenB) => {
  const pa = await phienCua(tenA);
  const pb = await phienCua(tenB);
  let cap = await capCua(pa, tenHien(tenB));
  if (!cap) {
    const yc = await ghiApi("POST", "/friends/requests", { addressee_id: pb.person_id }, pa);
    const dsDen = await api("GET", `/people/${pb.person_id}/friend-requests?direction=incoming`, undefined, pb);
    const lm = (dsDen.requests ?? dsDen.friend_requests ?? []).find((x) => x.state === "pending");
    const tl = lm ? await ghiApi("POST", `/friends/requests/${lm.id}/respond`, { decision: "accept" }, pb) : null;
    const dm = await ghiApi("POST", `/people/${pb.person_id}/dm`, undefined, pa);
    log({ lapCap: `${tenA}/${tenB}`, ketBan: yc.status, dongY: tl?.status, dm: dm.status });
    cap = await capCua(pa, tenHien(tenB));
  }
  if (!cap) throw new Error(`không mở được chat đôi ${tenA}/${tenB}`);
  return { cap, pa, pb };
};

try {
  const pA = await phienCua("chat-0");
  const pB = await phienCua("chat-1");
  const CAP = await capCua(pA, "Chat Test 02");
  if (!CAP) throw new Error("chat-0 không có chat đôi với Chat Test 02");
  const DUONG = `/groups/${CAP.id}/to-giay`;
  const daLap = async () => (await soCua(CAP, pA)).my_consents?.find?.((c) => c.purpose === "lap_so")?.granted === true;

  // --------------------------------------------------- choosing a person
  if (chay("chon-nguoi")) {
    // No friend at all.
    const t0 = await mo("C1", P("moi-50"), "/hai-nguoi/chon-nguoi");
    await t0.page.waitForTimeout(1200);
    const rong = await chuTrang(t0.page);
    await chup(t0.page, { out: mt.out, id: "EV-F07-CHON-RONG-C1", suKien: t0.suKien });
    ket({ tc: "TC-F07.S01-RONG", screen: "F07.S01", state: "tài khoản không có bạn nào", action: "mở Rủ một người đi chơi", cauHinh: "C1", expected: "trạng thái rỗng nói cần kết bạn trước, có lối «Thêm bạn»", status: /Chưa có bạn nào để rủ/.test(rong) && /Thêm bạn/.test(rong) ? "PASS" : "FAIL", evidence: ["EV-F07-CHON-RONG-C1"], ghiChu: `«${rong.slice(0, 200)}»` });
    await t0.context.close();
    // One friend, no notebook yet: choosing opens the pair's paper, which only offers to propose.
    const t = await mo("C1", P("chat-0"), "/hai-nguoi/chon-nguoi");
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(1200);
    const hang = await timNut(t.page, "Chat Test 02", { batDau: true });
    if (hang) await cham(cdp, hang);
    await t.page.waitForTimeout(2500);
    const den = await duongDan(t.page);
    const chu = await chuTrang(t.page);
    const toTruoc = (await toCua(CAP, pA)).length;
    await chup(t.page, { out: mt.out, id: "EV-F07-CHUA-LAP-SO-C1", suKien: t.suKien });
    ket({ tc: "TC-F07-CHON-NGUOI", screen: "F07.S01", state: "một người bạn, chưa có sổ hai người", action: "chạm «Chat Test 02»", cauHinh: "C1", expected: "mở tờ giấy của cặp; chưa có sổ thì mời «Đề nghị lập sổ» và không phác tờ nào", status: /\/to-giay/.test(den) && /Đề nghị lập sổ/.test(chu) && toTruoc === 0 ? "PASS" : "FAIL", evidence: ["EV-F07-CHUA-LAP-SO-C1"], ghiChu: `hàng ${hang ? `«${hang.ten}» ${hang.w}×${hang.h}` : "không thấy"}; tới ${an(den)}; «Đề nghị lập sổ» ${/Đề nghị lập sổ/.test(chu) ? "có" : "không"}; tờ trên máy chủ ${toTruoc}` });
    await t.context.close();
  }

  // ----------------------------- propose the notebook, agree on the other side
  if (chay("lap-so")) {
    if (await daLap()) {
      log({ boQua: "sổ của chat-0/chat-1 đã lập (lượt trước)" });
    } else {
      // A proposes, and keeps the screen open.
      const tA = await mo("C1", P("chat-0"), "/messages");
      const cdpA = await cdpCua(tA.page);
      await diToi(tA, DUONG);
      // The two names on the closed cover: cut or whole.
      const bia = await tA.page.evaluate(() => {
        const vung = document.querySelector('[data-testid="giay-chua-lap-so"]');
        const la = vung ? [...vung.querySelectorAll("div,span")].filter((e) => e.children.length === 0 && /Chat Test/.test(e.textContent ?? "")) : [];
        return la.map((e) => ({ chu: e.textContent.trim(), cat: e.scrollWidth > e.clientWidth + 1, rong: Math.round(e.clientWidth), can: e.scrollWidth }));
      });
      ket({ tc: "TC-F07-BIA-TEN", screen: "F07.S02", state: "chưa có sổ; hai tên «Chat Test 01», «Chat Test 02» trên bìa", action: "mở tờ giấy", cauHinh: "C1", expected: "đọc được hai tên trên bìa, hoặc ít nhất phân biệt được hai người", status: bia.length === 2 && bia.every((x) => !x.cat) ? "PASS" : "FAIL", evidence: ["EV-F07.S02-BASE-ghep"], ghiChu: bia.map((x) => `«${x.chu}» ${x.cat ? `bị cắt (${x.rong}px cho ${x.can}px)` : "trọn"}`).join("; ") || "không thấy tên trên bìa" });
      await bam(tA, cdpA, "Đề nghị lập sổ", { cho: 1200 });
      const sheet = { mo: await demDialog(tA.page), chu: await chuTrang(tA.page) };
      await chup(tA.page, { out: mt.out, id: "EV-F07-DE-NGHI-LAP-SO-C1", suKien: tA.suKien });
      // The sheet's own «Đề nghị lập sổ» (the stamp inside the dialog).
      const nutTrong = await tA.page.evaluate(() => { const d = [...document.querySelectorAll('[role="dialog"]')].pop(); const b = d && [...d.querySelectorAll('[role="button"]')].find((x) => /Đề nghị lập sổ/.test(x.innerText ?? "")); if (!b) return null; const r = b.getBoundingClientRect(); return { x: r.left + r.width / 2, y: r.top + r.height / 2 }; });
      if (nutTrong) await cham(cdpA, nutTrong);
      await tA.page.waitForTimeout(2500);
      const sauA = { dialog: await demDialog(tA.page), chu: await chuTrang(tA.page) };
      ket({ tc: "TC-F07-DE-NGHI", screen: "F07.S02", layer: "L23", state: "chưa có sổ", action: "«Đề nghị lập sổ» → sheet → «Đề nghị lập sổ»", cauHinh: "C1", expected: "sheet nói lập sổ là gì; sau khi gửi, màn nói đang chờ người kia", status: sheet.mo === 1 && nutTrong && /chờ|Đã đề nghị|đợi/i.test(sauA.chu) ? "PASS" : "FAIL", evidence: ["EV-F07-DE-NGHI-LAP-SO-C1"], ghiChu: `sheet ${sheet.mo} hộp thoại; sau khi gửi: ${sauA.dialog} hộp thoại, màn «${sauA.chu.slice(0, 220)}»` });
      // B agrees on their own screen.
      const tB = await mo("C1", P("chat-1"), "/messages");
      const cdpB = await cdpCua(tB.page);
      await diToi(tB, DUONG);
      const truocB = await chuTrang(tB.page);
      await bam(tB, cdpB, "Xem lời đề nghị", { cho: 1200 });
      await chup(tB.page, { out: mt.out, id: "EV-F07-XEM-DE-NGHI-C1", suKien: tB.suKien });
      const dongY = await tB.page.evaluate(() => { const d = [...document.querySelectorAll('[role="dialog"]')].pop(); const b = d && [...d.querySelectorAll('[role="button"]')].find((x) => (x.innerText ?? "").trim() === "Đồng ý"); if (!b) return null; const r = b.getBoundingClientRect(); return { x: r.left + r.width / 2, y: r.top + r.height / 2 }; });
      if (dongY) await cham(cdpB, dongY);
      // M6 on B's screen: the cover opening, sampled.
      const t0 = Date.now();
      while (!(await coTestId(tB.page, "giay-trong")) && Date.now() - t0 < 8000) await tB.page.waitForTimeout(60);
      const m6 = await layMau(tB.page, '[data-testid="giay-trong"]');
      await ghepMau(m6, "EV-F07-M6-C1", "M6 ở C1: bìa sổ mở trên màn người vừa đồng ý");
      await chup(tB.page, { out: mt.out, id: "EV-F07-SO-MO-B-C1", suKien: tB.suKien });
      const sauB = await chuTrang(tB.page);
      log({ truocB: truocB.slice(0, 160), m6: { soMau: m6.mau.length, khac: m6.khac, doiCuoi: m6.doiCuoi }, sauB: sauB.slice(0, 200) });
      ket({ tc: "TC-F07-DONG-Y-LAP-SO", screen: "F07.S02", layer: "L23", state: "người kia đã đề nghị lập sổ", action: "«Xem lời đề nghị» → «Đồng ý»", cauHinh: "C1", expected: "sheet đóng khi máy chủ trả lời; sổ mở, bìa mở một lần (M6); màn mời «Rủ đi chơi»", status: dongY && (await daLap()) && /Rủ đi chơi/.test(sauB) && m6.khac > 1 ? "PASS" : "FAIL", evidence: ["EV-F07-XEM-DE-NGHI-C1", "EV-F07-M6-C1", "EV-F07-SO-MO-B-C1"], ghiChu: `B trước: «${truocB.slice(0, 90)}»; nút «Đồng ý» ${dongY ? "có" : "không"}; M6: ${m6.mau.length} mẫu, ${m6.khac} ảnh khác nhau, đổi lần cuối ở ${m6.doiCuoi} ms` });
      // A's screen, still open: does it learn without a reload?
      const t1 = Date.now();
      let thay = false;
      while (!thay && Date.now() - t1 < 30_000) {
        thay = await coTestId(tA.page, "giay-trong");
        if (!thay) await tA.page.waitForTimeout(500);
      }
      const choA = Date.now() - t1;
      await chup(tA.page, { out: mt.out, id: "EV-F07-SO-MO-A-C1", suKien: tA.suKien });
      ket({ tc: "TC-F07-SO-MO-BEN-KIA", screen: "F07.S02", state: "mình vừa đề nghị, màn đang mở; người kia đồng ý trên máy họ", action: "đợi, không tải lại", cauHinh: "C1", expected: "màn tự đổi sang sổ đã mở (và M6) trong vài giây", status: thay ? "PASS" : "FAIL", evidence: ["EV-F07-SO-MO-A-C1"], ghiChu: thay ? `đổi sau ${choA} ms` : `30 s sau vẫn chưa đổi; màn: «${(await chuTrang(tA.page)).slice(0, 160)}»` });
      await tA.context.close();
      await tB.context.close();
    }
  }

  // ------------------------------------------- one week's sheet, both sides (E4)
  if (chay("ru")) {
    const toMo = async () => (await toCua(CAP, pA)).find((x) => ["nhap", "da_gui", "da_xem", "de_nghi_sua", "dong_y", "chot"].includes(x.state));
    const tA = await mo("C1", P("chat-0"), "/messages");
    const cdpA = await cdpCua(tA.page);
    await diToi(tA, DUONG);
    if (!(await toMo())) {
      await bam(tA, cdpA, "Rủ đi chơi", { cho: 3000 });
    }
    const nhap = { chu: await chuTrang(tA.page), to: await toMo() };
    await chup(tA.page, { out: mt.out, id: "EV-F07-TO-NHAP-C1", suKien: tA.suKien });
    ket({ tc: "TC-F07-RU", screen: "F07.S02", state: "sổ đã mở, tuần này chưa có tờ", action: "«Rủ đi chơi»", cauHinh: "C1", expected: "Nếp phác sẵn một tờ nháp; có «Gửi cho người ấy», «Sửa trước khi gửi», «Bỏ bản phác này»", status: nhap.to?.state === "nhap" && /Gửi cho người ấy/.test(nhap.chu) && /Sửa trước khi gửi/.test(nhap.chu) ? "PASS" : "FAIL", evidence: ["EV-F07-TO-NHAP-C1"], ghiChu: `tờ ${nhap.to?.state ?? "không có"}; màn «${nhap.chu.slice(0, 220)}»` });
    // Edit the draft: the five-field sheet.
    if (nhap.to?.state === "nhap") {
      await bam(tA, cdpA, "Sửa trước khi gửi", { cho: 1200 });
      const s = await tA.page.evaluate(() => {
        const d = [...document.querySelectorAll('[role="dialog"]')].pop();
        const r = d?.getBoundingClientRect();
        const o = d ? [...d.querySelectorAll("input,textarea")].map((i) => ({ ten: i.getAttribute("aria-label") ?? i.getAttribute("placeholder"), h: Math.round(i.getBoundingClientRect().height) })) : [];
        return { cao: r ? Math.round(r.height) : null, phan: r ? Math.round((r.height / innerHeight) * 100) : null, o };
      });
      await chup(tA.page, { out: mt.out, id: "EV-F07-SUA-NHAP-C1", suKien: tA.suKien });
      const o1 = tA.page.locator('[data-testid="de-nghi-sua-viec"]').first();
      if (await o1.count()) { await o1.click(); await o1.fill("Ăn tối ở quán nướng quen, rồi đi dạo bờ hồ"); }
      await bam(tA, cdpA, "Lưu bản phác", { cho: 2500 });
      const sau = await toMo();
      log({ s, sau: sau?.state });
      ket({ tc: "TC-F07-SUA-NHAP", screen: "F07.S02", layer: "L23", state: "tờ nháp của mình", action: "«Sửa trước khi gửi», đổi chỗ chính, «Lưu bản phác»", cauHinh: "C1", expected: "sheet ≤ 82% cao, mọi ô ≥ 48dp; lưu xong sheet đóng, tờ vẫn là nháp với chữ mới", status: s.phan !== null && s.phan <= 82 && (await demDialog(tA.page)) === 0 && sau?.state === "nhap" ? "PASS" : "FAIL", evidence: ["EV-F07-SUA-NHAP-C1"], ghiChu: `sheet cao ${s.cao} (${s.phan}% màn); ô: ${s.o.map((x) => `${x.ten} ${x.h}`).join(", ")}; sau khi lưu: ${sau?.state}` });
      // Send.
      await bam(tA, cdpA, "Gửi cho người ấy", { cho: 2500 });
    }
    const guiXong = await toMo();
    await chup(tA.page, { out: mt.out, id: "EV-F07-DA-GUI-C1", suKien: tA.suKien });
    ket({ tc: "TC-F07-GUI", screen: "F07.S02", state: "tờ nháp đã sửa", action: "«Gửi cho người ấy»", cauHinh: "C1", expected: "tờ thành «đã gửi», màn nói đang chờ người ấy", status: ["da_gui", "da_xem", "de_nghi_sua", "dong_y", "chot"].includes(guiXong?.state) ? "PASS" : "FAIL", evidence: ["EV-F07-DA-GUI-C1"], ghiChu: `tờ ${guiXong?.state}; màn «${(await chuTrang(tA.page)).slice(0, 200)}»` });
    // B answers with a new version.
    const tB = await mo("C1", P("chat-1"), "/messages");
    const cdpB = await cdpCua(tB.page);
    await diToi(tB, DUONG);
    const bThay = await chuTrang(tB.page);
    await chup(tB.page, { out: mt.out, id: "EV-F07-B-NHAN-C1", suKien: tB.suKien });
    if ((await toMo())?.state !== "chot") {
      await bam(tB, cdpB, "Đề nghị sửa", { cho: 1200 });
      const o2 = tB.page.locator('[data-testid="de-nghi-sua-viec"]').first();
      if (await o2.count()) { await o2.click(); await o2.fill("Ăn tối ở quán nướng quen, rồi đi xem phim"); }
      const lyDo = tB.page.locator('textarea[aria-label^="Vì sao đổi"], [aria-label^="Vì sao đổi"]').first();
      if (await lyDo.count()) await lyDo.fill("Tối đó có phim mới.");
      await chup(tB.page, { out: mt.out, id: "EV-F07-DE-NGHI-SUA-C1", suKien: tB.suKien });
      await bam(tB, cdpB, "Gửi phiên bản", { batDau: true, cho: 2500 });
    }
    const pb2 = await toMo();
    ket({ tc: "TC-F07-DE-NGHI-SUA", screen: "F07.S02", layer: "L23", state: "B nhận tờ của A", action: "«Đề nghị sửa», đổi chỗ chính và lý do, «Gửi phiên bản 2»", cauHinh: "C1", expected: "tờ có phiên bản 2 do B gửi; A thấy khác biệt và lý do", status: (pb2?.versions?.length ?? pb2?.version ?? 0) >= 2 || pb2?.state === "de_nghi_sua" ? "PASS" : "FAIL", evidence: ["EV-F07-B-NHAN-C1", "EV-F07-DE-NGHI-SUA-C1"], ghiChu: `B thấy: «${bThay.slice(0, 160)}»; tờ ${pb2?.state}, phiên bản ${pb2?.version ?? pb2?.versions?.length ?? "?"}` });
    // A agrees.
    await tA.page.reload();
    await choOn(tA.page, { mang: tA.mang });
    await tA.page.waitForTimeout(1500);
    const aThay = await chuTrang(tA.page);
    await chup(tA.page, { out: mt.out, id: "EV-F07-A-THAY-SUA-C1", suKien: tA.suKien });
    await bam(tA, cdpA, "Ừ, hẹn", { batDau: true, cho: 3000 });
    const chot = await toMo();
    await chup(tA.page, { out: mt.out, id: "EV-F07-CHOT-C1", suKien: tA.suKien });
    ket({ tc: "TC-F07-DONG-Y-TO", screen: "F07.S02", state: "A nhận phiên bản 2 của B", action: "«Ừ, hẹn …»", cauHinh: "C1", expected: "A đọc được chỗ đã đổi và lý do trước khi đồng ý; sau đó tờ chốt, có «Xem kèo»", status: chot?.state === "chot" && /Xem kèo/.test(await chuTrang(tA.page)) ? "PASS" : "FAIL", evidence: ["EV-F07-A-THAY-SUA-C1", "EV-F07-CHOT-C1"], ghiChu: `A thấy «${aThay.slice(0, 200)}»; tờ ${chot?.state}` });
    await tA.context.close();
    await tB.context.close();
  }

  // ------------------------------------------------- the notebook's settings
  if (chay("cai-dat")) {
    const t = await mo("C1", P("chat-0"), "/messages");
    const cdp = await cdpCua(t.page);
    await diToi(t, DUONG);
    await bam(t, cdp, "Cài đặt sổ", { cho: 1000 });
    const mo0 = await demDialog(t.page);
    const hang = await t.page.evaluate(() => { const d = [...document.querySelectorAll('[role="dialog"]')].pop(); return d ? [...d.querySelectorAll('[role="button"],[role="link"]')].map((e) => (e.innerText ?? "").replace(/[-]/g, "").replace(/\s+/g, " ").trim()).filter(Boolean) : []; });
    await chup(t.page, { out: mt.out, id: "EV-F07-CAI-DAT-C1", suKien: t.suKien });
    // A row that pushes another route, then Back.
    await bam(t, cdp, "Tin nhắn Về cuộc trò chuyện", { cho: 1500 });
    const qua = await duongDan(t.page);
    // In the chat: is the notebook's sheet in the way of anything?
    const trongChat = await t.page.evaluate(() => {
      const ta = [...document.querySelectorAll("textarea")].find((x) => x.getClientRects().length > 0);
      const r = ta?.getBoundingClientRect();
      const tren = r ? document.elementFromPoint(r.left + r.width / 2, r.top + r.height / 2) : null;
      return { oSoanTrenCung: !!ta && !!tren && (tren === ta || ta.contains(tren)) };
    });
    const hopTrongChat = await demDialog(t.page);
    await t.page.goBack();
    await t.page.waitForTimeout(1500);
    const ve = { duong: await duongDan(t.page), dialog: await demDialog(t.page) };
    await chup(t.page, { out: mt.out, id: "EV-F07-CAI-DAT-VE-C1", suKien: t.suKien });
    // The same trip back by the chat's own «Quay lại».
    if (ve.dialog === 0) await bam(t, cdp, "Cài đặt sổ", { cho: 1000 });
    await bam(t, cdp, "Tin nhắn Về cuộc trò chuyện", { cho: 1500 });
    await bam(t, cdp, "Quay lại", { cho: 1500 });
    const veApp = { duong: await duongDan(t.page), dialog: await demDialog(t.page) };
    log({ mo0, hang, qua, trongChat, hopTrongChat, ve, veApp });
    ket({ tc: "TC-L23-CAI-DAT-DAY", screen: "F07.S02", layer: "L23", state: "sheet Cài đặt sổ mở", action: "chạm «Tin nhắn» (mở màn khác), rồi Back trình duyệt; lại lần nữa rồi «Quay lại» của chat", cauHinh: "C1", expected: "sheet đóng khi rời màn; quay lại thấy tờ giấy, không phải sheet còn treo", status: /\/chat$/.test(qua) && /\/to-giay/.test(ve.duong) && ve.dialog === 0 && veApp.dialog === 0 ? "PASS" : "FAIL", evidence: ["EV-F07-CAI-DAT-C1", "EV-F07-CAI-DAT-VE-C1"], ghiChu: `hàng trong sheet: ${hang.join(" · ")}; tới ${an(qua)}: ${hopTrongChat} hộp thoại hiện, ô soạn ${trongChat.oSoanTrenCung ? "nhận chạm" : "bị che"}; Back trình duyệt về ${an(ve.duong)} với ${ve.dialog} hộp thoại; «Quay lại» của chat về ${an(veApp.duong)} với ${veApp.dialog} hộp thoại` });
    // The sub-sheets: open each, look, close without doing anything.
    const kq = [];
    for (const [nhan, id] of [["Loại sổ", "LOAI-SO"], ["Hai ô ràng buộc", "RANG-BUOC"], ["Đóng sổ", "DONG-SO"]]) {
      if ((await demDialog(t.page)) === 0) await bam(t, cdp, "Cài đặt sổ", { cho: 900 });
      await bam(t, cdp, nhan, { batDau: true, cho: 1500 });
      const r = await t.page.evaluate(() => { const d = [...document.querySelectorAll('[role="dialog"]')].pop(); const b = d?.getBoundingClientRect(); return { ten: d?.getAttribute("aria-label"), phan: b ? Math.round((b.height / innerHeight) * 100) : null, chu: (d?.innerText ?? "").replace(/\s+/g, " ").slice(0, 160) }; });
      const f = await focusHienTai(t.page);
      const axe = await chayAxe(t.page).catch(() => ({ vi: [] }));
      await chup(t.page, { out: mt.out, id: `EV-F07-${id}-C1`, suKien: t.suKien });
      await t.page.keyboard.press("Escape");
      await t.page.waitForTimeout(800);
      kq.push({ nhan, ...r, focus: f?.ten ?? f?.role, axe: (axe.vi ?? []).map((v) => `${v.rule}×${v.so}`).join(",") || "sạch", con: await demDialog(t.page) });
    }
    log({ kq });
    ket({ tc: "TC-L23-SHEET-CON", screen: "F07.S02", layer: "L23", state: "Cài đặt sổ", action: "mở Loại sổ, Hai ô ràng buộc, Đóng sổ; Esc", cauHinh: "C1", expected: "mỗi sheet ≤ 82% cao, focus vào sheet, axe sạch, Esc đóng sạch; «Đóng sổ» có xem trước và bước hỏi", status: kq.every((x) => x.phan !== null && x.phan <= 82 && x.con === 0 && x.axe === "sạch") ? "PASS" : "FAIL", evidence: ["EV-F07-LOAI-SO-C1", "EV-F07-RANG-BUOC-C1", "EV-F07-DONG-SO-C1"], ghiChu: kq.map((x) => `${x.nhan}: «${x.ten}» ${x.phan}% màn, focus «${x.focus}», axe ${x.axe}, sau Esc ${x.con} hộp thoại`).join("; ") });
    await t.context.close();
  }

  // ------------------------------ the settings sheet's life cycle (L23)
  if (chay("vong-doi")) {
    const kq = [];
    for (const cach of ["x", "nen", "esc", "keo-ngan", "keo-dai", "fling", "back"]) {
      const t = await mo("C1", P("chat-0"), "/messages");
      const cdp = await cdpCua(t.page);
      await diToi(t, DUONG);
      const truoc = await luoiChamTrang(t.page);
      await bam(t, cdp, "Cài đặt sổ", { cho: 100 });
      const moRa = await choDialog(t.page, { co: true });
      await t.page.waitForTimeout(700);
      const focusMo = await focusHienTai(t.page);
      const d = await dong(t.page, cdp, cach);
      await t.page.waitForTimeout(cach === "keo-ngan" ? 900 : 1200);
      const conMo = await demDialog(t.page);
      const duong = await duongDan(t.page);
      const oLai = /\/to-giay/.test(duong);
      const khac = oLai && conMo === 0 ? soLuoi(truoc, await luoiChamTrang(t.page)).length : null;
      const inertSau = oLai && conMo === 0 ? (await inertConLai(t.page)).length : null;
      const focusSau = await focusHienTai(t.page);
      kq.push({ cach, mo: moRa.ok, msMo: moRa.ms, focusMo, lam: d.lam, ly: d.ly, conMo, oLai, khac, inertSau, focusSau, duong: an(duong) });
      await t.context.close();
    }
    log({ vongDoi: kq });
    const f = (k) => `${k.cach}: ${k.lam ? "" : `không làm được (${k.ly}); `}còn ${k.conMo} hộp thoại, ở ${k.duong}${k.khac === null ? "" : `, lưới đổi ${k.khac}, inert ${k.inertSau}`}`;
    const dung = kq.filter((k) => k.cach !== "back");
    ket({ tc: "TC-L23-VONGDOI", screen: "F07.S02", layer: "L23", state: "tờ giấy của cặp chat-0/chat-1", action: "mở «Cài đặt sổ»; đóng bằng X, nền, Esc, kéo ngắn, kéo dài, vuốt nhanh", cauHinh: "C1", expected: "mở một hộp thoại, focus vào trong; kéo ngắn bật về; mọi cách khác đóng hẳn, ở lại tờ giấy, không sót lớp chặn hay inert", status: dung.every((k) => k.mo && k.lam && (k.cach === "keo-ngan" ? k.conMo === 1 : k.conMo === 0 && k.oLai && k.khac === 0 && k.inertSau === 0)) && kq[0].focusMo?.trongDialog ? "PASS" : "FAIL", evidence: ["EV-F07-CAI-DAT-C1"], ghiChu: `${dung.map(f).join("; ")}; focus khi mở «${kq[0].focusMo?.ten ?? "body"}» ${kq[0].focusMo?.trongDialog ? "trong" : "ngoài"} sheet` });
    const x = kq[0];
    ket({ tc: "TC-L23-FOCUS", screen: "F07.S02", layer: "L23", state: "tờ giấy", action: "mở «Cài đặt sổ» rồi đóng bằng X", cauHinh: "C1", expected: "focus vào sheet khi mở; trả về nút «Cài đặt sổ» khi đóng", status: x.focusMo?.trongDialog && /Cài đặt sổ/.test(x.focusSau?.ten ?? "") ? "PASS" : "FAIL", evidence: [], ghiChu: `khi mở: «${x.focusMo?.ten ?? "body"}»; sau khi đóng: «${x.focusSau?.ten ?? "body"}» (${x.focusSau?.tag ?? "-"})` });
    const b = kq.find((k) => k.cach === "back");
    ket({ tc: "TC-L23-BACK", screen: "F07.S02", layer: "L23", state: "sheet «Cài đặt sổ» mở, tới tờ giấy từ Tin nhắn", action: "Back trình duyệt", cauHinh: "C1", expected: "Back đóng sheet và ở lại tờ giấy (Android: BackHandler)", status: b.conMo === 0 && b.oLai ? "PASS" : "FAIL", issue: b.conMo === 0 && b.oLai ? null : "UI-038", evidence: [], ghiChu: f(b) });
    // Two quick taps on the gear.
    const t = await mo("C1", P("chat-0"), "/messages");
    const cdp = await cdpCua(t.page);
    await diToi(t, DUONG);
    const n = await timNut(t.page, "Cài đặt sổ");
    await cham(cdp, n, 20);
    await t.page.waitForTimeout(60);
    // What the second tap lands on, 60 ms after the first.
    const duoiNgon = await t.page.evaluate(({ x, y }) => { const e = document.elementFromPoint(x, y); const b = e?.closest('[role="button"],button'); return b ? (b.getAttribute("aria-label") || b.innerText || "").trim().slice(0, 30) : e?.tagName ?? null; }, n);
    await cham(cdp, n, 20).catch(() => undefined);
    await t.page.waitForTimeout(1300);
    const soHop = await demDialog(t.page);
    ket({ tc: "TC-L23-CHAM-DUP", screen: "F07.S02", layer: "L23", state: "tờ giấy", action: "chạm nút «Cài đặt sổ» hai lần cách 60 ms", cauHinh: "C1", expected: "đúng một sheet mở", status: soHop === 1 ? "PASS" : "FAIL", evidence: [], ghiChu: `60 ms sau chạm đầu, chỗ nút «Cài đặt sổ» là «${duoiNgon}»; sau 1,3 s: ${soHop} hộp thoại` });
    await t.context.close();
    // The same sheet under Reduce Motion: open and close should cut.
    const t9 = await mo("C9", P("chat-0"), "/messages");
    const cdp9 = await cdpCua(t9.page);
    await diToi(t9, DUONG);
    const SEL = '[role="dialog"][aria-label="Cài đặt sổ"]';
    await batDauLayMau(t9.page, SEL);
    await bam(t9, cdp9, "Cài đặt sổ", { cho: 1200 });
    const moRa = phanTich(await ketThucLayMau(t9.page));
    await batDauLayMau(t9.page, SEL);
    await dong(t9.page, cdp9, "x");
    await t9.page.waitForTimeout(1200);
    const mauDong = await ketThucLayMau(t9.page);
    const dongLai = phanTich(mauDong);
    const conHien = mauDong.filter((m) => m.n > 0);
    log({ C9: { moRa, dongLai, khungCuoi: conHien.slice(-3) } });
    ket({ tc: "TC-MO04-C9-SO", screen: "F07.S02", layer: "L23", state: "giảm chuyển động", action: "mở «Cài đặt sổ» rồi đóng bằng X", cauHinh: "C9", expected: "sheet hiện ngay ở chỗ cuối và mất ngay khi đóng, không trượt", status: moRa.xuatHien && moRa.thoiLuongMs <= 50 && conHien.length > 0 && new Set(conHien.map((m) => m.y)).size === 1 ? "PASS" : "FAIL", evidence: [], ghiChu: `mở: xuất hiện ở y ${moRa.dau?.y} → đứng ở y ${moRa.cuoi?.y} sau ${moRa.thoiLuongMs} ms; đóng: ${conHien.length} mẫu còn thấy, y ${[...new Set(conHien.map((m) => m.y))].join("→")}, rồi mất` });
    await t9.context.close();
  }

  // ------------------------------ «Rủ <tên> tới đây» from a place (TC-F02-CHI-TIET-RU)
  // Two cases, by what this week already holds when the button is pressed:
  // an agreed sheet and nothing open (the first run: TC-F02-CHI-TIET-RU), or
  // one's own draft (every run after it: TC-F02-CHI-TIET-RU-NHAP). The number
  // of sheets on the server is read before and after either way.
  if (chay("ru-toi-day")) {
    const MO = ["nhap", "da_gui", "da_xem", "de_nghi_sua", "dong_y"];
    const truoc = await toCua(CAP, pA);
    const moTruoc = truoc.find((x) => MO.includes(x.state));
    const t = await mo("C1", P("chat-0"), "/explore");
    const cdp = await cdpCua(t.page);
    await diToi(t, "/places/p-tiem-nuong-xom-lao");
    const nut = await timNut(t.page, "Rủ Chat Test 02 tới đây", { batDau: true });
    await chup(t.page, { out: mt.out, id: "EV-F07-RU-TOI-DAY-C1", suKien: t.suKien });
    if (nut) await cham(cdp, nut);
    await t.page.waitForTimeout(3500);
    const den = await duongDan(t.page);
    const sau = await toCua(CAP, pA);
    const cau = await viTriCau(t.page, /tuần này|Tuần này hai bạn|đã ở trên tờ|đang chờ/);
    const hop = await t.page.evaluate(() => {
      const d = [...document.querySelectorAll('[role="dialog"]')].filter((x) => x.getClientRects().length).pop();
      if (!d) return null;
      const dai = d.querySelector('[data-testid="de-nghi-sua-ngay"]');
      const r = dai?.getBoundingClientRect();
      // Day leaves are role=radio; the chosen one is named in words under the strip («Thứ Bảy 03/10»).
      const ngayChon = ((d.innerText ?? "").match(/Thứ \S+ (\d{2})\/\d{2}/) ?? [])[1];
      const la = dai ? [...dai.querySelectorAll('[role="radio"]')].map((b) => { const chu = (b.innerText ?? "").replace(/\s+/g, " ").trim(); const q = b.getBoundingClientRect(); return { chu: `${chu}${ngayChon && chu.includes(` ${ngayChon} `) ? " (đang chọn)" : ""}`, trai: Math.round(q.left), phai: Math.round(q.right) }; }) : [];
      return { ten: d.getAttribute("aria-label"), cho: /Ở Tiệm Nướng Xóm Lào/.test(d.innerText ?? ""), viec: d.querySelector('[data-testid="de-nghi-sua-viec"]')?.value ?? null, dai: r ? { phai: Math.round(r.right), cuonDuoc: dai.scrollWidth > dai.clientWidth, soLa: la.length, ariaChecked: dai.querySelectorAll('[role="radio"][aria-checked]').length, cat: la.filter((x) => x.trai < r.right && x.phai > r.right + 1).map((x) => `${x.chu}, thấy ${Math.round(r.right) - x.trai}/${x.phai - x.trai}px`) } : null };
    });
    const moiTo = sau.length - truoc.length;
    const toMoi = sau.find((x) => !truoc.some((y) => y.id === x.id));
    await chup(t.page, { out: mt.out, id: moTruoc?.state === "nhap" ? "EV-F07-RU-TOI-DAY-NHAP-C1" : "EV-F07-RU-TOI-DAY-SAU-C1", suKien: t.suKien });
    log({ truoc: truoc.map((x) => x.state), sau: sau.map((x) => x.state), hop, cau: cau?.text });
    if (moTruoc?.state === "nhap") {
      ket({ tc: "TC-F02-CHI-TIET-RU-NHAP", feature: "F02", screen: "F02.S03", state: "người xem có sổ hai người, tuần này có bản phác của chính mình", action: "chạm «Rủ Chat Test 02 tới đây»", cauHinh: "C1", expected: "mở tờ giấy và sheet sửa bản phác với chỗ vừa chọn làm chỗ chính; không tạo thêm tờ", status: nut && /\/to-giay/.test(den) && hop?.cho && moiTo === 0 ? "PASS" : "FAIL", evidence: ["EV-F07-RU-TOI-DAY-C1", "EV-F07-RU-TOI-DAY-NHAP-C1"], ghiChu: `nút ${nut ? `«${nut.ten}» ${nut.w}×${nut.h}` : "không có"}; tới ${an(den)}; sheet «${hop?.ten ?? "không"}», chỗ «Tiệm Nướng Xóm Lào» ${hop?.cho ? "có" : "không"}, ô chỗ chính «${hop?.viec ?? "-"}»; dải ngày ${hop?.dai ? `${hop.dai.soLa} lá role=radio (${hop.dai.ariaChecked} lá có aria-checked), ${hop.dai.cuonDuoc ? "cuộn ngang được" : "không cuộn"}, lá lưng chừng mép phải: ${hop.dai.cat.join("; ") || "không"}` : "không thấy"}; tờ trên máy chủ ${truoc.length} → ${sau.length}` });
      await t.page.keyboard.press("Escape");
      await t.page.waitForTimeout(800);
    } else {
      so.rut("TC-F02-CHI-TIET-RU", "đo ở F07 vì nút chỉ hiện khi người xem có sổ đôi đang mở");
      ket({ tc: "TC-F02-CHI-TIET-RU", feature: "F02", screen: "F02.S03", state: `người xem có sổ hai người; tuần này: ${truoc.map((x) => x.state).join(", ") || "chưa có tờ"}`, action: "chạm «Rủ Chat Test 02 tới đây»", cauHinh: "C1", expected: "mở tờ giấy; chỗ vừa chọn nằm trên một bản phác, hoặc màn nói rõ vì sao không và không tự tạo tờ nào", status: nut && /\/to-giay/.test(den) && (moiTo === 0 ? cau?.trongMan : hop?.cho) ? "PASS" : "FAIL", evidence: ["EV-F07-RU-TOI-DAY-C1", "EV-F07-RU-TOI-DAY-SAU-C1"], ghiChu: `nút ${nut ? `«${nut.ten}» ${nut.w}×${nut.h}` : "không có"}; tới ${an(den)}; tờ trên máy chủ ${truoc.length} → ${sau.length}${toMoi ? ` (tờ mới: ${toMoi.state})` : ""}; sheet ${hop ? `«${hop.ten}»` : "không mở"}; câu «${cau?.text ?? "không có"}»` });
    }
    await t.context.close();
  }

  // ---------------------------------------- M6 under Reduce Motion, a second pair
  if (chay("m6-c9")) {
    const p2 = await phienCua("chat-2");
    const p3 = await phienCua("chat-3");
    let cap = await capCua(p2, "Chat Test 04");
    if (!cap) {
      const yc = await ghiApi("POST", "/friends/requests", { addressee_id: p3.person_id }, p2);
      const dsDen = await api("GET", `/people/${p3.person_id}/friend-requests?direction=incoming`, undefined, p3);
      const lm = (dsDen.requests ?? dsDen.friend_requests ?? []).find((x) => x.state === "pending");
      const tl = lm ? await ghiApi("POST", `/friends/requests/${lm.id}/respond`, { decision: "accept" }, p3) : null;
      const dm = await ghiApi("POST", `/people/${p3.person_id}/dm`, undefined, p2);
      log({ ketBan: yc.status, dongY: tl?.status, dm: dm.status });
      cap = await capCua(p2, "Chat Test 04");
    }
    const nb = await api("GET", `/contexts/${cap.id}/notebook`, undefined, p2);
    if (nb.my_consents?.find?.((c) => c.purpose === "lap_so")?.granted) {
      log({ boQua: "sổ của chat-2/chat-3 đã lập" });
    } else {
      // chat-2 proposes on its own screen (C1): the proposer's waiting state,
      // read from what is rendered (the sheet stays open by design and says so).
      const tA = await mo("C1", P("chat-2"), "/messages");
      const cdpA = await cdpCua(tA.page);
      await diToi(tA, `/groups/${cap.id}/to-giay`);
      await bam(tA, cdpA, "Đề nghị lập sổ", { cho: 1200 });
      const nutTrong = await tA.page.evaluate(() => { const d = [...document.querySelectorAll('[role="dialog"]')].pop(); const b = d && [...d.querySelectorAll('[role="button"]')].find((x) => /Đề nghị lập sổ/.test(x.innerText ?? "")); if (!b) return null; const r = b.getBoundingClientRect(); return { x: r.left + r.width / 2, y: r.top + r.height / 2 }; });
      if (nutTrong) await cham(cdpA, nutTrong);
      await tA.page.waitForTimeout(2500);
      const cho = { dialog: await demDialog(tA.page), cau: await viTriCau(tA.page, /Đã đề nghị\. Chờ/), nutDongY: !!(await timNut(tA.page, "Đồng ý")) };
      const deNghiMayChu = await api("GET", `/contexts/${cap.id}/notebook`, undefined, p2);
      await chup(tA.page, { out: mt.out, id: "EV-F07-DANG-CHO-C1", suKien: tA.suKien });
      log({ cho, khoaSo: Object.keys(deNghiMayChu ?? {}) });
      ket({ tc: "TC-F07-DE-NGHI", screen: "F07.S02", layer: "L23", state: "chưa có sổ (cặp chat-2/chat-3)", action: "«Đề nghị lập sổ» → sheet → «Đề nghị lập sổ»", cauHinh: "C1", expected: "sheet nói lập sổ là gì; sau khi gửi, sheet nói đang chờ người kia và không mời mình tự đồng ý", status: nutTrong && cho.dialog === 1 && cho.cau?.trongMan && !cho.nutDongY ? "PASS" : "FAIL", evidence: ["EV-F07-DANG-CHO-C1"], ghiChu: `nút trong sheet ${nutTrong ? "có" : "không"}; sau khi gửi: ${cho.dialog} hộp thoại, câu «${cho.cau?.text ?? "không thấy"}» ${cho.cau?.trongMan ? "trong màn" : "ngoài màn"}; nút «Đồng ý» ${cho.nutDongY ? "có" : "không"}` });
      // Closed with «Để sau»: what the screen itself says about the wait.
      await bam(tA, cdpA, "Để sau", { cho: 1200 });
      const sauDong = { dialog: await demDialog(tA.page), nut: (await timNut(tA.page, "Xem lời đề nghị"))?.ten ?? null, cauCho: await viTriCau(tA.page, /[Cc]hờ|đã đề nghị|Đã đề nghị/), tieuDe: await viTriCau(tA.page, /Chưa có sổ hai người/) };
      await chup(tA.page, { out: mt.out, id: "EV-F07-DANG-CHO-DONG-C1", suKien: tA.suKien });
      ket({ tc: "TC-F07-CHO-SAU-KHI-DONG", screen: "F07.S02", state: "mình vừa đề nghị lập sổ", action: "«Để sau» đóng sheet", cauHinh: "C1", expected: "màn vẫn nói mình đang chờ người kia, không đọc như chưa có gì hay như người kia đề nghị", status: sauDong.dialog === 0 && sauDong.cauCho?.trongMan ? "PASS" : "FAIL", evidence: ["EV-F07-DANG-CHO-DONG-C1"], ghiChu: `${sauDong.dialog} hộp thoại; tiêu đề «${sauDong.tieuDe?.text ?? "không"}»; nút «${sauDong.nut ?? "không"}»; câu chờ trên màn: ${sauDong.cauCho ? `«${sauDong.cauCho.text}»` : "không có"}` });
      // chat-3 agrees under Reduce Motion (C9): M6 should cut straight to the end pose.
      const t = await mo("C9", P("chat-3"), "/messages");
      const cdp = await cdpCua(t.page);
      await diToi(t, `/groups/${cap.id}/to-giay`);
      await bam(t, cdp, "Xem lời đề nghị", { cho: 1200 });
      const dongY = await t.page.evaluate(() => { const d = [...document.querySelectorAll('[role="dialog"]')].pop(); const b = d && [...d.querySelectorAll('[role="button"]')].find((x) => (x.innerText ?? "").trim() === "Đồng ý"); if (!b) return null; const r = b.getBoundingClientRect(); return { x: r.left + r.width / 2, y: r.top + r.height / 2 }; });
      if (dongY) await cham(cdp, dongY);
      const t0 = Date.now();
      while (!(await coTestId(t.page, "giay-trong")) && Date.now() - t0 < 8000) await t.page.waitForTimeout(60);
      const m6 = await layMau(t.page, '[data-testid="giay-trong"]');
      await ghepMau(m6, "EV-F07-M6-C9", "M6 ở C9 (giảm chuyển động): bìa sổ trên màn người vừa đồng ý");
      log({ m6: { soMau: m6.mau.length, khac: m6.khac, doiCuoi: m6.doiCuoi } });
      ket({ tc: "TC-MO19-C9", screen: "F07.S02", state: "giảm chuyển động; người kia đã đề nghị lập sổ", action: "«Đồng ý»", cauHinh: "C9", expected: "bìa hiện ở tư thế cuối ngay (cắt thẳng), không lật", status: m6.mau.length > 0 && m6.doiCuoi <= 300 ? "PASS" : "FAIL", evidence: ["EV-F07-M6-C9"], ghiChu: `nút «Đồng ý» ${dongY ? "có" : "không"}; ${m6.mau.length} mẫu mỗi 150 ms, ${m6.khac} ảnh khác nhau, đổi lần cuối ở ${m6.doiCuoi} ms` });
      await t.context.close();
      await tA.context.close();
    }
  }

  // ------------- M6 frame by frame: the cover's angle on every animation frame
  // Screenshot polling (above) starts too late to see a 600 ms swing on this
  // machine; this records, per rAF from before the tap: the cover's angle and
  // its inside face's (SoBia.tsx: cover −108·m, inside 180 − 108·m), the seal,
  // which body the screen shows and what the still-open sheet offers. Plus the
  // compositor frames for a picture. One fresh pair per config: the moment
  // plays only when the notebook opens while the screen is up.
  if (chay("m6-khung")) {
    // chat-10/chat-11 ran C9 once before the sheet's position was sampled; chat-12/chat-13 re-ran it.
    for (const [cfg, tenA, tenB] of [["C1", "chat-8", "chat-9"], ["C9", "chat-12", "chat-13"]]) {
      const { cap, pa, pb } = await lapCap(tenA, tenB);
      const nb = await api("GET", `/contexts/${cap.id}/notebook`, undefined, pb);
      if (nb.my_consents?.find?.((c) => c.purpose === "lap_so")?.granted) {
        log({ boQua: `sổ của ${tenA}/${tenB} đã lập` });
        continue;
      }
      if (!(nb.pending_proposals ?? []).length) {
        const dn = await ghiApi("POST", `/contexts/${cap.id}/notebook/proposals`, { purpose: "lap_so" }, pa);
        log({ deNghi: `${tenA} → ${tenB}`, status: dn.status });
      }
      const t = await mo(cfg, P(tenB), "/messages");
      const cdp = await cdpCua(t.page);
      await diToi(t, `/groups/${cap.id}/to-giay`);
      await bam(t, cdp, "Xem lời đề nghị", { cho: 1200 });
      const dongY = await t.page.evaluate(() => { const d = [...document.querySelectorAll('[role="dialog"]')].pop(); const b = d && [...d.querySelectorAll('[role="button"]')].find((x) => (x.innerText ?? "").trim() === "Đồng ý"); if (!b) return null; const r = b.getBoundingClientRect(); return { x: r.left + r.width / 2, y: r.top + r.height / 2 }; });
      await t.page.evaluate(() => {
        const w = window;
        w.__m6 = [];
        w.__m6Dung = false;
        w.__m6T0 = performance.now();
        const hien = (x) => x.getClientRects().length > 0 && x.checkVisibility({ visibilityProperty: true });
        const buoc = () => {
          if (w.__m6Dung) return;
          const vung = document.querySelector('[data-testid="giay-trong"]');
          const goc = vung ? [...vung.querySelectorAll("div")].map((e) => (/rotateY\((-?[\d.]+)deg\)/.exec(e.style.transform) ?? [])[1]).filter((g) => g !== undefined).map(Number) : [];
          const bia = goc.find((g) => g <= 0);
          const trong = goc.find((g) => g > 0);
          const dau = vung?.querySelector('[aria-label="Sổ đã mở"]');
          let op = null;
          if (dau) { op = 1; for (let n = dau; n && n !== document.documentElement; n = n.parentElement) op *= Number(getComputedStyle(n).opacity); }
          const d = [...document.querySelectorAll('[role="dialog"]')].filter(hien).pop();
          const nut = d ? [...d.querySelectorAll('[role="button"]')].map((b) => (b.innerText ?? "").trim()) : [];
          const than = document.querySelector('[data-testid="giay-chua-lap-so"]') ? "chua-lap-so" : vung ? (goc.length ? "m6" : "thuong") : "khac";
          w.__m6.push({ t: Math.round(performance.now() - w.__m6T0), than, bia: bia ?? null, trong: trong ?? null, dauOp: op === null ? null : Math.round(op * 100) / 100, dauTf: dau ? dau.style.transform : null, sheet: d ? (nut.includes("Đồng ý") ? "dong-y" : nut.some((x) => /^Đề nghị lập sổ/.test(x)) ? "de-nghi" : "khac") : null, sheetY: d ? Math.round(d.getBoundingClientRect().top) : null });
          requestAnimationFrame(buoc);
        };
        requestAnimationFrame(buoc);
      });
      const quay = await quayKhung(t.page, cdp, { ms: 4200 });
      const tCham = await t.page.evaluate(() => Math.round(performance.now() - window.__m6T0));
      if (dongY) await cham(cdp, dongY);
      const khung = await quay.dung();
      const mau = await t.page.evaluate(() => { window.__m6Dung = true; return window.__m6; });
      await ghepKhung(mt.browser, khung, join(mt.out, "jpg", `EV-F07-M6-KHUNG-${cfg}.jpg`), { tieuDe: `M6 bìa sổ mở trên màn người đồng ý, ${cfg}${cfg === "C9" ? " (giảm chuyển động)" : ""}`, cot: 6, toiDa: 24 });
      writeFileSync(join(mt.out, `m6-mau-${cfg}.json`), JSON.stringify({ tCham, mau }));
      const sau = mau.filter((m) => m.t >= tCham);
      // The cover: from the first frame of the opening body to the frame its angle stops.
      const co = sau.filter((m) => m.than === "m6" && m.bia !== null);
      const dau = co[0];
      const cuoi = co[co.length - 1];
      const gocKhac = [...new Set(co.map((m) => m.bia))];
      const trungGian = gocKhac.filter((g) => g !== 0 && g !== -108);
      let tDung = dau?.t ?? null;
      for (let i = co.length - 1; i > 0; i -= 1) if (co[i].bia !== co[i - 1].bia) { tDung = co[i].t; break; }
      const dauCo = co.filter((m) => m.dauOp !== null);
      const dauKhac = [...new Set(dauCo.map((m) => `${m.dauOp}|${m.dauTf}`))];
      const dauXong = dauCo.find((m) => m.dauOp === 1 && /scale\(1\)|scale\(0\.99/.test(m.dauTf ?? ""));
      // Frames between the tap and the opening body that show something else.
      const thanTruoc = sau.filter((m) => !dau || m.t < dau.t).map((m) => m.than);
      const thuong = sau.filter((m) => m.than === "thuong").length;
      const sheetDeNghi = sau.filter((m) => m.sheet === "de-nghi");
      // Where the closing sheet's top edge stood on each frame after the tap.
      const sheetY = [...new Set(sau.filter((m) => m.sheetY !== null).map((m) => m.sheetY))];
      log({ cfg, sheetY, soMau: mau.length, tCham, soKhung: khung.length, thanTruoc: [...new Set(thanTruoc)], thuong, sheetDeNghi: sheetDeNghi.length ? { soKhung: sheetDeNghi.length, tu: sheetDeNghi[0].t - tCham, den: sheetDeNghi[sheetDeNghi.length - 1].t - tCham } : 0, bia: dau ? { tXuatHien: dau.t - tCham, gocDau: dau.bia, gocCuoi: cuoi.bia, soGoc: gocKhac.length, trungGian: trungGian.length, keoDaiMs: tDung - dau.t } : null, dau: { soTrangThai: dauKhac.length, dauTien: dauKhac[0], cuoi: dauKhac[dauKhac.length - 1], xongSau: dauXong ? dauXong.t - dau.t : null } });
      const giam = cfg === "C9";
      const ghi = dau ? `${co.length} mẫu rAF có bìa đang mở (từ ${dau.t - tCham} ms sau chạm); góc bìa đầu ${dau.bia}°, cuối ${cuoi.bia}°, ${trungGian.length} góc trung gian, đổi lần cuối ${tDung - dau.t} ms sau mẫu đầu; dấu «Sổ đã mở»: ${dauKhac.length} trạng thái, xong ở ${dauXong ? `${dauXong.t - dau.t} ms` : "chưa"}; trước bìa: ${[...new Set(thanTruoc)].join(" → ")}, ${thuong} khung thân «Tuần này» thường; sheet mời «Đề nghị lập sổ» sau chạm: ${sheetDeNghi.length} khung; ${khung.length} khung compositor (SwiftShader: chỉ đọc trình tự, không đọc thời lượng)` : `không thấy bìa đang mở sau khi đồng ý (nút «Đồng ý» ${dongY ? "có" : "không"})`;
      ket({ tc: `TC-MO16-${cfg}`, screen: "F07.S02", state: `${giam ? "giảm chuyển động; " : ""}người kia đã đề nghị lập sổ (cặp ${tenA}/${tenB})`, action: "«Xem lời đề nghị» → «Đồng ý»", cauHinh: cfg, expected: giam ? "bìa hiện ngay ở góc cuối (−108°), không có góc trung gian; dấu hiện ngay" : "bìa lật từ 0° tới −108° qua nhiều khung, một lần, rồi đứng yên; dấu rơi rồi đậm", status: dau && cuoi.bia === -108 && (giam ? trungGian.length === 0 : trungGian.length >= 3) ? "PASS" : "FAIL", evidence: [`EV-F07-M6-KHUNG-${cfg}`], ghiChu: ghi });
      ket({ tc: `TC-F07-DONG-Y-NHAY-${cfg}`, screen: "F07.S02", layer: "L23", state: `người kia đã đề nghị lập sổ (cặp ${tenA}/${tenB})`, action: "«Đồng ý» trong sheet «Lập sổ hai người»", cauHinh: cfg, expected: "từ lúc chạm tới lúc sheet đóng, sheet không quay về trạng thái mời «Đề nghị lập sổ», thân màn không hiện trạng thái khác trước bìa mở", status: sheetDeNghi.length === 0 && thuong === 0 ? "PASS" : "FAIL", evidence: [`EV-F07-M6-KHUNG-${cfg}`], ghiChu: `sheet mời «Đề nghị lập sổ»: ${sheetDeNghi.length ? `${sheetDeNghi.length} khung rAF, từ ${sheetDeNghi[0].t - tCham} tới ${sheetDeNghi[sheetDeNghi.length - 1].t - tCham} ms sau chạm` : "không"}; thân «Tuần này» thường trước bìa: ${thuong} khung; thứ tự thân: ${[...new Set(sau.map((m) => m.than))].join(" → ")}; đỉnh sheet sau chạm: ${sheetY.join("→") || "-"}` });
      await t.context.close();
    }
  }

  // ------------------------------------------------ low window: the edit sheets
  // chat-2/chat-3's notebook (opened in m6-c9): chat-2 drafts this week's
  // sheet if there is none, then the two sheets with fields are read at C8.
  if (chay("c8")) {
    const { cap: cap2, pa: p2 } = await lapCap("chat-2", "chat-3");
    const t = await mo("C8", P("chat-2"), "/messages");
    const cdp = await cdpCua(t.page);
    await diToi(t, `/groups/${cap2.id}/to-giay`);
    const toMo2 = async () => (await toCua(cap2, p2)).find((x) => ["nhap", "da_gui", "da_xem", "de_nghi_sua"].includes(x.state));
    if (!(await toMo2())) await bam(t, cdp, "Rủ đi chơi", { cho: 3000 });
    const doSheet = (nutChinh) =>
      t.page.evaluate((nutChinh) => {
        const d = [...document.querySelectorAll('[role="dialog"]')].filter((x) => x.getClientRects().length).pop();
        if (!d) return null;
        const b = d.getBoundingClientRect();
        const ten = (e) => (e.innerText ?? "").replace(/[\uE000-\uF8FF]/g, "").replace(/\s+/g, " ").trim();
        const nut = () => [...d.querySelectorAll('[role="button"]')].find((e) => ten(e).startsWith(nutChinh));
        const hop = (e) => { const r = e?.getBoundingClientRect(); return r ? { top: Math.round(r.top), bottom: Math.round(r.bottom), trongMan: r.top >= 0 && r.bottom <= innerHeight } : null; };
        const truoc = hop(nut());
        const cuon = [...d.querySelectorAll("*")].filter((e) => e.scrollHeight > e.clientHeight + 2 && /(auto|scroll)/.test(getComputedStyle(e).overflowY));
        for (const e of cuon) e.scrollTop = e.scrollHeight;
        const o = [...d.querySelectorAll("input,textarea")].map((i) => ({ ten: i.getAttribute("aria-label") ?? i.getAttribute("placeholder"), h: Math.round(i.getBoundingClientRect().height) }));
        return { phan: Math.round((b.height / innerHeight) * 100), top: Math.round(b.top), cuon: cuon.length, truoc, sau: hop(nut()), o, cao: innerHeight };
      }, nutChinh);
    const kq = [];
    // The draft's edit sheet (five fields).
    const moSua = await bam(t, cdp, "Sửa trước khi gửi", { cho: 1200 });
    if (moSua) {
      await chup(t.page, { out: mt.out, id: "EV-F07-C8-SUA-C8", suKien: t.suKien });
      kq.push({ ten: "Sửa bản phác", nutChinh: "Lưu bản phác", ...(await doSheet("Lưu bản phác")) });
      await chup(t.page, { out: mt.out, id: "EV-F07-C8-SUA-CUON-C8", suKien: t.suKien });
      await t.page.keyboard.press("Escape");
      await t.page.waitForTimeout(900);
    }
    // «Hai ô ràng buộc» from the settings sheet.
    await bam(t, cdp, "Cài đặt sổ", { cho: 1000 });
    const moRb = await bam(t, cdp, "Hai ô ràng buộc", { batDau: true, cho: 1500 });
    if (moRb) {
      await chup(t.page, { out: mt.out, id: "EV-F07-C8-RANG-BUOC-C8", suKien: t.suKien });
      kq.push({ ten: "Hai ô ràng buộc", nutChinh: "Lưu hai ô của tôi", ...(await doSheet("Lưu hai ô của tôi")) });
      await t.page.keyboard.press("Escape");
      await t.page.waitForTimeout(900);
    }
    log({ c8: kq });
    const tom = (x) => `${x.ten}: sheet ${x.phan}% cửa sổ ${x.cao}px, đỉnh y ${x.top}; «${x.nutChinh}» ${x.truoc ? `đáy ${x.truoc.bottom} ${x.truoc.trongMan ? "trong" : "ngoài"} màn` : "không thấy"} → sau khi cuộn trong sheet (${x.cuon} vùng cuộn) ${x.sau ? `${x.sau.trongMan ? "trong" : "ngoài"} màn (đáy ${x.sau.bottom})` : "không thấy"}; ô: ${x.o.map((q) => `${q.ten} ${q.h}`).join(", ")}`;
    ket({ tc: "TC-F07-C8", screen: "F07.S02", layer: "L23", state: "cửa sổ 390×460 (proxy bàn phím); tờ nháp của mình", action: "mở «Sửa trước khi gửi» và «Hai ô ràng buộc»", cauHinh: "C8", expected: "sheet ≤ 82% cao; nút chính với tới được (cuộn trong sheet)", status: kq.length === 2 && kq.every((x) => x.phan <= 82 && x.sau?.trongMan) ? "PASS" : "FAIL", evidence: ["EV-F07-C8-SUA-C8", "EV-F07-C8-SUA-CUON-C8", "EV-F07-C8-RANG-BUOC-C8"], ghiChu: kq.map(tom).join("; ") || "không mở được sheet nào" });
    await t.context.close();
  }

  // -------------------------------------------------- cold open, failure, tablet
  if (chay("lanh")) {
    const t = await mo("C1", P("chat-0"), DUONG);
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(1200);
    const truoc = await duongDan(t.page);
    await bam(t, cdp, "Quay lại", { cho: 1200 });
    const sau = await duongDan(t.page);
    ket({ tc: "TC-F07.S02-BACK-LANH", screen: "F07.S02", state: "mở thẳng bằng link", action: "chạm «Quay lại»", cauHinh: "C1", expected: "tới một màn hợp lý", status: sau !== truoc ? "PASS" : "FAIL", issue: sau !== truoc ? null : "UI-018", evidence: [], ghiChu: `${an(truoc)} → ${an(sau)}` });
    await t.context.close();
    // No session at all: the link lands on the way in, not on an empty notebook.
    const t2 = await mo("C1", null, DUONG);
    await t2.page.waitForTimeout(1500);
    const den2 = await duongDan(t2.page);
    const chu2 = await chuTrang(t2.page);
    await chup(t2.page, { out: mt.out, id: "EV-F07-LANH-KHONG-PHIEN-C1", suKien: t2.suKien });
    const nhanDemo = /Dữ liệu demo|[Dd]emo|[Tt]rải nghiệm/.test(await t2.page.evaluate(() => document.body.innerText));
    ket({ tc: "TC-F07.S02-LANH-KHONG-PHIEN", screen: "F07.S02", state: "không có phiên", action: "mở thẳng link tờ giấy của một cặp thật", cauHinh: "C1", expected: "đưa tới cửa vào (Welcome/đăng nhập); nếu cho xem bản trải nghiệm thì nói rõ là dữ liệu demo", status: !/to-giay/.test(den2) || /Đăng nhập|số điện thoại|Bắt đầu/.test(chu2) || nhanDemo ? "PASS" : "FAIL", evidence: ["EV-F07-LANH-KHONG-PHIEN-C1"], ghiChu: `tới ${an(den2)}; nhãn demo trên trang: ${nhanDemo ? "có" : "không"}; màn «${chu2.slice(0, 160)}»` });
    // …and what a press on that page does: every write the page sends to the API is counted.
    const ghiDi = [];
    t2.page.on("request", (r) => { if (r.url().startsWith(mt.api) && r.method() !== "GET") ghiDi.push(`${r.method()} ${an(new URL(r.url()).pathname)}`); });
    const cdp2 = await cdpCua(t2.page);
    const ru2 = await bam(t2, cdp2, "Rủ đi chơi", { cho: 2500 });
    const gui2 = await bam(t2, cdp2, "Gửi cho người ấy", { cho: 2500 });
    const cauGui = await viTriCau(t2.page, /Đã gửi, chờ trả lời/);
    await chup(t2.page, { out: mt.out, id: "EV-F07-LANH-KHONG-PHIEN-GUI-C1", suKien: t2.suKien });
    ket({ tc: "TC-F07-KHONG-PHIEN-GUI", screen: "F07.S02", state: "không có phiên, trang tờ giấy mở từ link", action: "«Rủ đi chơi» → «Gửi cho người ấy»", cauHinh: "C1", expected: "không nói «đã gửi» khi không có gì tới máy chủ; mời đăng nhập", status: !(cauGui?.trongMan && ghiDi.length === 0) ? "PASS" : "FAIL", evidence: ["EV-F07-LANH-KHONG-PHIEN-GUI-C1"], ghiChu: `«Rủ đi chơi» ${ru2 ? "có" : "không"}, «Gửi cho người ấy» ${gui2 ? "có" : "không"}; câu «${cauGui?.text ?? "không"}»; lệnh ghi tới API: ${ghiDi.length ? ghiDi.join(", ") : "0"}` });
    await t2.context.close();
  }
  if (chay("loi")) {
    const t = await mo("C1", P("chat-0"), "/messages");
    const cdp = await cdpCua(t.page);
    await loiMayChu(t.page, new RegExp(`/contexts/${CAP.id}/(notebook|papers)(\\?|$)`));
    await diToi(t, DUONG);
    await t.page.waitForTimeout(2000);
    const chu = await chuTrang(t.page);
    const thuLai = await timNut(t.page, "Thử lại") ?? await timNut(t.page, "Làm mới");
    await chup(t.page, { out: mt.out, id: "EV-F07.S02-503-C1", suKien: t.suKien });
    await goHet(t.page);
    if (thuLai) await bam(t, cdp, thuLai.ten, { cho: 2500 });
    // No retry on screen: does the four-second poll bring the notebook back by itself?
    const tHoi = Date.now();
    while (!thuLai && (await coTestId(t.page, "giay-chua-lap-so")) && Date.now() - tHoi < 12_000) await t.page.waitForTimeout(250);
    const hoiSau = Date.now() - tHoi;
    const sau = await chuTrang(t.page);
    ket({ tc: "TC-F07.S02-503", screen: "F07.S02", state: "GET sổ và tờ trả 503", action: "mở tờ giấy; máy chủ ổn; thử lại", cauHinh: "C1", expected: "nói không đọc được sổ (không vẽ như sổ trống), có cách thử lại", status: /sự cố|chưa đọc|Chưa đọc|thử lại/i.test(chu) && !/Chưa có sổ hai người/.test(chu) && thuLai && !/sự cố/.test(sau) ? "PASS" : "FAIL", evidence: ["EV-F07.S02-503-C1"], ghiChu: `khi 503: «${chu.slice(0, 220)}»; nút thử lại ${thuLai ? `«${thuLai.ten}»` : "không có"}; máy chủ ổn lại: ${(await coTestId(t.page, "giay-chua-lap-so")) ? `12 s sau vẫn «Chưa có sổ»` : `trang tự đổi sau ${hoiSau} ms`}, «${sau.slice(0, 120)}»` });
    await t.context.close();
  }
  if (chay("tablet")) {
    for (const cfg of ["C6", "C7"]) {
      const t = await mo(cfg, P("chat-0"), DUONG);
      await t.page.waitForTimeout(1500);
      const r = await t.page.evaluate(() => { const e = document.querySelector('[data-testid="to-mo"]') ?? document.querySelector('[data-testid="giay-trong"]'); const b = e?.getBoundingClientRect(); return b ? { w: Math.round(b.width), left: Math.round(b.left), cw: innerWidth } : null; });
      await chup(t.page, { out: mt.out, id: `EV-F07-TABLET-${cfg}`, suKien: t.suKien });
      // The settings sheet on the same tablet.
      const cdpT = await cdpCua(t.page);
      await bam(t, cdpT, "Cài đặt sổ", { cho: 1200 });
      const sh = await t.page.evaluate(() => { const d = [...document.querySelectorAll('[role="dialog"]')].filter((x) => x.getClientRects().length).pop(); const b = d?.getBoundingClientRect(); return b ? { w: Math.round(b.width), left: Math.round(b.left), phan: Math.round((b.height / innerHeight) * 100) } : null; });
      await chup(t.page, { out: mt.out, id: `EV-F07-TABLET-CAI-DAT-${cfg}`, suKien: t.suKien });
      ket({ tc: "TC-L23-TABLET", screen: "F07.S02", layer: "L23", state: "tablet", action: "mở «Cài đặt sổ»", cauHinh: cfg, expected: "sheet ≤ 82% cao, bề ngang có trần, hàng đọc được", status: sh && sh.phan <= 82 && sh.w <= 640 ? "PASS" : "FAIL", evidence: [`EV-F07-TABLET-CAI-DAT-${cfg}`], ghiChu: sh ? `sheet rộng ${sh.w}px, lề trái ${sh.left}, cao ${sh.phan}% cửa sổ` : "không mở được sheet" });
      ket({ tc: "TC-F07-TABLET", screen: "F07.S02", state: "tờ tuần này đang hiện (bản phác hoặc đã chốt)", action: "mở tờ giấy ở tablet", cauHinh: cfg, expected: "tờ giấy có trần bề ngang, đọc như một tờ giấy", status: r && r.w <= 640 ? "PASS" : "FAIL", evidence: [`EV-F07-TABLET-${cfg}`], ghiChu: r ? `tờ rộng ${r.w}px, lề trái ${r.left}, cửa sổ ${r.cw}` : "không thấy tờ" });
      await t.context.close();
    }
  }
} finally {
  await mt.dong();
}
