/* F05, Chat: the conversations list, a group thread, a two-person thread, the
 * composer, the trays and sheets of a message.
 *
 *   node kich-ban/f05-chat.mjs [--chi bong-dai,soan,gui,offline,anh,sticker,menu,menu-dong,bao-cao,cong-cu,cai-dat,tin-moi,bo-cuc,phieu,rong,ghim-phu,thong-bao,ghim,lanh,loi,c8,dm,votes]
 *
 * Writes go to the chat-test group only (chat-0 and chat-1), whose legacy
 * thread holds the forty synthetic lines of seed-bien-the.mjs `datLichSuChat`.
 * Team Đà Lạt's thread is read, never written. Nothing here posts outside the
 * local stack.
 */
import { readFileSync } from "node:fs";
import { join } from "node:path";

import { pngThuBytes } from "../../../../apps/mobile/tools/png-thu.mjs";
import { chayAxe } from "../thu-vien/axe.mjs";
import { cauHinh } from "../thu-vien/cau-hinh.mjs";
import { chup } from "../thu-vien/chup.mjs";
import { cdpCua, cham } from "../thu-vien/cu-chi.mjs";
import { choOn, duongDan } from "../thu-vien/dieu-huong.mjs";
import { demDialog, dong, focusHienTai } from "../thu-vien/lop-phu.mjs";
import { goHet, loiMayChu, ngatMang } from "../thu-vien/mang.mjs";
import { khoiDong, trangMoi } from "../thu-vien/moi-truong.mjs";
import { goiApi, layPhien, personaTheoTen } from "../thu-vien/phien.mjs";
import { soGhi } from "../thu-vien/ghi.mjs";
import { datLichSuChat, moNhom } from "../seed-bien-the.mjs";

const chi = (() => {
  const i = process.argv.indexOf("--chi");
  return i === -1 ? null : new Set(process.argv[i + 1].split(","));
})();
const chay = (ten) => !chi || chi.has(ten);
const mt = await khoiDong();
const so = soGhi(mt.out);
const ket = (rec) => {
  so.ghi({ feature: "F05", nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, ...rec });
  console.log(`${rec.status.padEnd(10)} ${rec.tc} ${rec.cauHinh ?? ""} ${rec.ghiChu ?? ""}`);
};
const log = (o) => console.log(JSON.stringify(o));

const P8 = personaTheoTen("dalat-0", mt.chatSessions);
const P0 = personaTheoTen("chat-0", mt.chatSessions);
const P1 = personaTheoTen("chat-1", mt.chatSessions);
const { nhomId: G8 } = await moNhom(mt);
const G20 = JSON.parse(readFileSync(mt.chatSessions, "utf8")).groupId;
await datLichSuChat(mt);
const phien0 = await layPhien(mt.api, P0, join(mt.out, "phien"));
const phien1 = await layPhien(mt.api, P1, join(mt.out, "phien"));
const toiCtx = async (phien, kind) => ((await goiApi(mt.api, "GET", "/people/me/contexts", undefined, phien.token)).contexts ?? []).filter((c) => c.kind === kind);
const DM = (await toiCtx(phien0, "pair")).find((c) => /Chat Test 02/.test(c.display_name ?? c.counterpart?.display_name ?? ""))?.id ?? (await toiCtx(phien0, "pair"))[0]?.id;
const tinNhom = async (ctx, phien) => (await goiApi(mt.api, "GET", `/contexts/${ctx}/messages?limit=50`, undefined, phien.token)).messages ?? [];

const mo = (cfg, persona, path, them = {}) => trangMoi(mt, cauHinh(cfg), { persona, path, ...them });
const chatCua = (ctx) => `/groups/${ctx}/chat`;
/** Open /messages, then walk into `path` inside the app, so Back has an app screen to return to. */
const moQuaTin = async (cfg, persona, path) => {
  const t = await mo(cfg, persona, "/messages");
  await t.page.evaluate((p) => {
    history.pushState({}, "", p);
    dispatchEvent(new PopStateEvent("popstate"));
  }, path);
  await choOn(t.page, { mang: t.mang });
  await t.page.waitForTimeout(900);
  return t;
};

/** Put back sideways scrolls of rows no finger can scroll (see thu-vien/cu-chi.mjs `tamCua`). */
const NGANG = "for (let n = e.parentElement; n; n = n.parentElement) if (n.scrollLeft && !/(auto|scroll)/.test(getComputedStyle(n).overflowX)) n.scrollLeft = 0;";
/** A control by its whole accessible name or visible text, scrolled into view vertically. */
const timNut = (page, chu, { vai = '[role="button"],button,[role="checkbox"],[role="radio"],[role="switch"],a[href]', batDau = false } = {}) =>
  page.evaluate(
    ({ chu, vai, batDau, NGANG }) => {
      const ten = (x) => (x.getAttribute("aria-label") || (x.innerText ?? "")).replace(/[-]/g, "").replace(/\s+/g, " ").trim();
      const e = [...document.querySelectorAll(vai)].find((x) => (batDau ? ten(x).startsWith(chu) : ten(x) === chu) && x.getBoundingClientRect().width > 0);
      if (!e) return null;
      e.scrollIntoView({ block: "center", behavior: "instant" });
      new Function("e", NGANG)(e);
      const r = e.getBoundingClientRect();
      return { x: r.left + r.width / 2, y: r.top + r.height / 2, w: Math.round(r.width), h: Math.round(r.height), top: Math.round(r.top), ten: ten(e) };
    },
    { chu, vai, batDau, NGANG },
  );
const bam = async (t, cdp, chu, { cho = 700, ...o } = {}) => {
  const n = await timNut(t.page, chu, o);
  if (!n) return null;
  await cham(cdp, n);
  await t.page.waitForTimeout(cho);
  return n;
};
const coChu = (page, re) => page.evaluate((s) => new RegExp(s).test(document.body.innerText ?? ""), re.source);
const viTriCau = (page, re) =>
  page.evaluate((s) => {
    const R = new RegExp(s);
    const e = [...document.querySelectorAll("body *")].find((x) => [...x.childNodes].some((n) => n.nodeType === 3 && R.test(n.textContent)));
    if (!e) return null;
    const r = e.getBoundingClientRect();
    return { top: Math.round(r.top), bottom: Math.round(r.bottom), trongMan: r.bottom > 0 && r.top < innerHeight, text: e.textContent.trim().slice(0, 140) };
  }, re.source);
/** The composer: type, then send with the arrow. */
const soan = async (page, chu) => {
  const o = page.locator('textarea[aria-label="Ô soạn tin"]');
  await o.click();
  await o.fill(chu);
};
const guiTin = async (t, cdp, chu) => {
  await soan(t.page, chu);
  await t.page.waitForTimeout(200);
  const n = await timNut(t.page, "Gửi tin nhắn");
  await cham(cdp, n);
  await t.page.waitForTimeout(1500);
};
/** The own bubble with this text (newest first), its box. */
const bongCua = (page, chu) =>
  page.evaluate((chu) => {
    const ds = [...document.querySelectorAll('[role="button"][aria-label^="Tin nhắn: "]')].filter((e) => e.getAttribute("aria-label") === `Tin nhắn: ${chu}` && e.getBoundingClientRect().height > 0);
    const e = ds.sort((a, b) => b.getBoundingClientRect().top - a.getBoundingClientRect().top)[0];
    if (!e) return null;
    const r = e.getBoundingClientRect();
    return { x: r.left + r.width / 2, y: r.top + r.height / 2, left: Math.round(r.left), right: Math.round(r.right), top: Math.round(r.top), w: Math.round(r.width), h: Math.round(r.height) };
  }, chu);
/** Scroll the inverted thread towards older messages by `px` (its scrollTop). */
const cuonLichSu = (page, px) =>
  page.evaluate((px) => {
    const ds = document.querySelector('[data-testid="chat-list"]');
    const sc = [ds, ...(ds ? ds.querySelectorAll("div") : [])].find((d) => d && d.scrollHeight > d.clientHeight + 10 && /(auto|scroll)/.test(getComputedStyle(d).overflowY));
    if (!sc) return null;
    sc.scrollTop = px;
    sc.dispatchEvent(new Event("scroll", { bubbles: true }));
    return Math.round(sc.scrollTop);
  }, px);
const trangThaiCuon = (page) =>
  page.evaluate(() => {
    const ds = document.querySelector('[data-testid="chat-list"]');
    const sc = [ds, ...(ds ? ds.querySelectorAll("div") : [])].find((d) => d && d.scrollHeight > d.clientHeight + 10 && /(auto|scroll)/.test(getComputedStyle(d).overflowY));
    const r = sc?.getBoundingClientRect();
    const thay = [...document.querySelectorAll('[role="button"][aria-label^="Tin nhắn: "]')].filter((e) => { const b = e.getBoundingClientRect(); return r && b.bottom > r.top && b.top < r.bottom; }).map((e) => ({ ten: e.getAttribute("aria-label").slice(10, 50), top: Math.round(e.getBoundingClientRect().top) }));
    return { scrollTop: sc ? Math.round(sc.scrollTop) : null, thay, pill: !!document.querySelector('[aria-label="Về tin nhắn mới nhất"]') };
  });

try {
  // ---------------------------------------- a long unbroken string in a bubble
  if (chay("bong-dai")) {
    const URL_DAI = (await tinNhom(G20, phien0)).find((m) => /^https:\/\/example\.com/.test(m.body ?? ""))?.body;
    for (const cfg of ["C1", "C2", "C3", "C5"]) {
      const t = await mo(cfg, P0, chatCua(G20));
      await t.page.waitForTimeout(900);
      const b = await t.page.evaluate(({ url, NGANG }) => {
        const e = [...document.querySelectorAll('[role="button"][aria-label^="Tin nhắn: "]')].find((x) => x.getAttribute("aria-label") === `Tin nhắn: ${url}`);
        if (!e) return null;
        e.scrollIntoView({ block: "center", behavior: "instant" });
        new Function("e", NGANG)(e);
        e.setAttribute("data-audit-bong", "1");
        const r = e.getBoundingClientRect();
        const hang = e.closest('[data-testid^="chat-message-"]')?.getBoundingClientRect();
        return { left: Math.round(r.left), right: Math.round(r.right), w: Math.round(r.width), hang: hang ? Math.round(hang.width) : null, cw: innerWidth };
      }, { url: URL_DAI, NGANG });
      await t.page.waitForTimeout(300);
      await chup(t.page, { out: mt.out, id: `EV-F05-URL-${cfg}`, suKien: t.suKien, chuThich: [{ selector: "[data-audit-bong]", nhan: "tin có URL dài" }] });
      log({ cfg, b });
      const toiDa = b?.hang ? Math.round(b.hang * 0.82) : null;
      ket({ tc: "TC-F05-URL-DAI", screen: "F05.S02", state: "tin của mình là một URL 158 ký tự (đoạn liền dài nhất 36 ký tự)", action: "mở chat nhóm 20 người", cauHinh: cfg, expected: "bong bóng ≤ 82% hàng, nằm trọn trong màn, chữ xuống dòng", status: b && b.left >= 0 && b.right <= b.cw && (!toiDa || b.w <= toiDa + 1) ? "PASS" : "FAIL", evidence: [`EV-F05-URL-${cfg}`], ghiChu: b ? `bong bóng x ${b.left}–${b.right} (rộng ${b.w}, trần 82% = ${toiDa}), cửa sổ ${b.cw}` : "không thấy tin URL" });
      await t.context.close();
    }
  }

  // ------------------------------------------------------------- the composer
  if (chay("soan")) {
    const t = await mo("C1", P0, chatCua(G20));
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(800);
    const doSoan = () =>
      t.page.evaluate(() => {
        const o = document.querySelector('textarea[aria-label="Ô soạn tin"]');
        const cong = document.querySelector('[aria-label="Thêm vào cuộc trò chuyện"]')?.getBoundingClientRect();
        const gui = document.querySelector('[aria-label="Gửi tin nhắn"]')?.getBoundingClientRect();
        const r = o.getBoundingClientRect();
        const s = getComputedStyle(o);
        const lh = parseFloat(s.lineHeight) || 22;
        return { oTop: Math.round(r.top), oBottom: Math.round(r.bottom), cao: Math.round(r.height), dongDauGiua: Math.round(r.top + parseFloat(s.paddingTop) + lh / 2), nutGiua: cong ? Math.round(cong.top + cong.height / 2) : null, guiGiua: gui ? Math.round(gui.top + gui.height / 2) : null, cuon: o.scrollHeight > o.clientHeight + 1, rows: o.getAttribute("rows") };
      });
    const rong = await doSoan();
    await chup(t.page, { out: mt.out, id: "EV-F05-SOAN-rong-C1", suKien: t.suKien, chuThich: [{ selector: 'textarea[aria-label="Ô soạn tin"]', nhan: "ô soạn" }] });
    const lech = rong.nutGiua !== null ? Math.abs(rong.dongDauGiua - rong.nutGiua) : null;
    ket({ tc: "TC-F05-SOAN-CAN", screen: "F05.S02", state: "ô soạn trống", action: "mở chat", cauHinh: "C1", expected: "một dòng chữ nằm giữa viên thuốc, thẳng hàng với «+» và mũi tên gửi (chú thích trong mã: «Centred on purpose»)", status: lech !== null && lech <= 6 ? "PASS" : "FAIL", evidence: ["EV-F05-SOAN-rong-C1"], ghiChu: `ô cao ${rong.cao} (minHeight 48, rows=${rong.rows}); giữa dòng đầu y ${rong.dongDauGiua}, giữa nút «+» y ${rong.nutGiua}, lệch ${lech}px` });
    // Seven lines: Enter makes a new line on the web, nothing is sent.
    const truoc = (await tinNhom(G20, phien0)).length;
    const o = t.page.locator('textarea[aria-label="Ô soạn tin"]');
    await o.click();
    for (let i = 1; i <= 7; i += 1) {
      await t.page.keyboard.type(`Dòng thử ${i}`);
      if (i < 7) await t.page.keyboard.press("Enter");
    }
    await t.page.waitForTimeout(500);
    const bay = await doSoan();
    const sau = (await tinNhom(G20, phien0)).length;
    const guiThay = await t.page.evaluate(() => { const r = document.querySelector('[aria-label="Gửi tin nhắn"]')?.getBoundingClientRect(); return r ? r.bottom <= innerHeight && r.top >= 0 : false; });
    await chup(t.page, { out: mt.out, id: "EV-F05-SOAN-7-DONG-C1", suKien: t.suKien });
    log({ rong, bay, truoc, sau });
    ket({ tc: "TC-F05-SOAN-NHIEU-DONG", screen: "F05.S02", state: "gõ 7 dòng, Enter giữa các dòng", action: "gõ", cauHinh: "C1", expected: "Enter xuống dòng, không gửi; ô cao dần tới trần 120 rồi cuộn; nút gửi vẫn thấy", status: sau === truoc && bay.cao > rong.cao && bay.cao <= 122 && guiThay ? "PASS" : "FAIL", evidence: ["EV-F05-SOAN-7-DONG-C1"], ghiChu: `tin trong nhóm ${truoc} → ${sau}; ô cao ${rong.cao} → ${bay.cao}, cuộn trong ô ${bay.cuon ? "có" : "không"}; nút gửi ${guiThay ? "thấy" : "khuất"}` });
    await o.fill("");
    await t.context.close();
  }

  // ----------------------------------------------------------- sending a line
  if (chay("gui")) {
    const t = await mo("C1", P0, chatCua(G20));
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(800);
    const chu = `Tin kiểm thử gửi ${Date.now() % 100000}`;
    const truoc = (await tinNhom(G20, phien0)).length;
    await guiTin(t, cdp, chu);
    const b = await bongCua(t.page, chu);
    const oTrong = await t.page.evaluate(() => document.querySelector('textarea[aria-label="Ô soạn tin"]').value === "");
    const sau = (await tinNhom(G20, phien0)).length;
    const cuon = await trangThaiCuon(t.page);
    await chup(t.page, { out: mt.out, id: "EV-F05-GUI-C1", suKien: t.suKien });
    ket({ tc: "TC-F05-GUI", screen: "F05.S02", state: "chat 40 tin, đang ở cuối", action: "gõ một dòng, chạm «Gửi tin nhắn»", cauHinh: "C1", expected: "bong bóng của mình hiện ở cuối, thấy được; ô soạn trống; một tin được ghi", status: b && b.top > 0 && b.top < 844 && oTrong && sau - truoc === 1 ? "PASS" : "FAIL", evidence: ["EV-F05-GUI-C1"], ghiChu: `bong bóng y ${b?.top}; ô soạn ${oTrong ? "trống" : "còn chữ"}; tin ${truoc} → ${sau}; cuộn ${cuon.scrollTop}` });
    await t.context.close();
  }

  // --------------------------------------------- offline, failed row, retry
  if (chay("offline")) {
    const t = await mo("C1", P0, chatCua(G20));
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(800);
    const chu = `Tin kiểm thử mất mạng ${Date.now() % 100000}`;
    await ngatMang(t.context, true);
    await soan(t.page, chu);
    await cham(cdp, await timNut(t.page, "Gửi tin nhắn"));
    await t.page.waitForTimeout(3500);
    const hong = await t.page.evaluate((chu) => {
      const txt = document.body.innerText;
      return { coChu: txt.includes(chu), dangGui: /Đang gửi…/.test(txt), cau: (txt.match(/Không kết nối[^\n]*|Chưa gửi được[^\n]*|Kiểm tra mạng[^\n]*/) ?? [null])[0], thuLai: !![...document.querySelectorAll('[role="button"]')].find((e) => (e.innerText ?? "").trim() === "Thử lại"), bo: !![...document.querySelectorAll('[role="button"]')].find((e) => (e.innerText ?? "").trim() === "Bỏ") };
    }, chu);
    await chup(t.page, { out: mt.out, id: "EV-F05-OFFLINE-C1", suKien: t.suKien });
    await ngatMang(t.context, false);
    await t.page.waitForTimeout(800);
    await bam(t, cdp, "Thử lại", { cho: 3000 });
    const daGui = (await tinNhom(G20, phien0)).some((m) => m.body === chu);
    const conHang = await coChu(t.page, /Thử lại/);
    log({ hong, daGui, conHang });
    ket({ tc: "TC-F05-OFFLINE", screen: "F05.S02", state: "mất mạng", action: "gửi một dòng, rồi có mạng lại và «Thử lại»", cauHinh: "C1", expected: "hàng của tin ở lại với câu vì sao và «Thử lại», «Bỏ»; thử lại thì gửi đúng một lần", status: hong.coChu && hong.thuLai && daGui && !conHang ? "PASS" : "FAIL", evidence: ["EV-F05-OFFLINE-C1"], ghiChu: `khi mất mạng: chữ ${hong.coChu ? "còn" : "mất"}, «${hong.cau ?? "không có câu"}», Thử lại ${hong.thuLai ? "có" : "không"}, Bỏ ${hong.bo ? "có" : "không"}; sau «Thử lại»: tin ${daGui ? "đã ghi" : "chưa ghi"}, hàng lỗi ${conHang ? "còn" : "hết"}` });
    await t.context.close();
  }

  // --------------------------------------------------------------- a photo
  if (chay("anh")) {
    const t = await mo("C1", P0, chatCua(G20));
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(800);
    const truoc = (await tinNhom(G20, phien0)).filter((m) => m.kind === "image").length;
    // Cancel first.
    await bam(t, cdp, "Thêm vào cuộc trò chuyện", { cho: 600 });
    const [fc1] = await Promise.all([t.page.waitForEvent("filechooser", { timeout: 6000 }).catch(() => null), bam(t, cdp, "Ảnh", { cho: 200 })]);
    if (fc1) await fc1.setFiles([]);
    await t.page.waitForTimeout(1200);
    const sauHuy = { anh: (await tinNhom(G20, phien0)).filter((m) => m.kind === "image").length, loi: await viTriCau(t.page, /Không mở được|Kiểm tra mạng/), nutCong: (await timNut(t.page, "Thêm vào cuộc trò chuyện")) !== null };
    ket({ tc: "TC-F05-ANH-HUY", screen: "F05.S02", layer: "L31", state: "khay công cụ", action: "«Ảnh», huỷ bộ chọn tệp", cauHinh: "C1", expected: "không gửi gì, không báo lỗi, «+» dùng lại được", status: fc1 && sauHuy.anh === truoc && !sauHuy.loi && sauHuy.nutCong ? "PASS" : "FAIL", evidence: [], ghiChu: `bộ chọn ${fc1 ? "mở" : "không mở"}; ảnh ${truoc} → ${sauHuy.anh}; câu lỗi ${sauHuy.loi ? `«${sauHuy.loi.text}»` : "không"}` });
    // Then a synthetic photo.
    await bam(t, cdp, "Thêm vào cuộc trò chuyện", { cho: 600 });
    const [fc2] = await Promise.all([t.page.waitForEvent("filechooser", { timeout: 6000 }).catch(() => null), bam(t, cdp, "Ảnh", { cho: 200 })]);
    if (fc2) await fc2.setFiles([{ name: "anh-tong-hop.png", mimeType: "image/png", buffer: Buffer.from(pngThuBytes(480, 360)) }]);
    await t.page.waitForTimeout(5000);
    const sau = (await tinNhom(G20, phien0)).filter((m) => m.kind === "image").length;
    const hien = await t.page.evaluate(() => {
      const e = [...document.querySelectorAll('[aria-label="Ảnh trong nhóm"], img[alt="Ảnh trong nhóm"]')].pop();
      if (!e) return null;
      const r = e.getBoundingClientRect();
      return { w: Math.round(r.width), h: Math.round(r.height), top: Math.round(r.top) };
    });
    await chup(t.page, { out: mt.out, id: "EV-F05-ANH-C1", suKien: t.suKien });
    ket({ tc: "TC-F05-ANH", screen: "F05.S02", layer: "L31", state: "khay công cụ", action: "«Ảnh», chọn ảnh tổng hợp", cauHinh: "C1", expected: "một tin ảnh được ghi và hiện ở cuối", status: sau - truoc === 1 && hien ? "PASS" : "FAIL", evidence: ["EV-F05-ANH-C1"], ghiChu: `tin ảnh ${truoc} → ${sau}; ảnh trên màn ${hien ? `${hien.w}×${hien.h} ở y ${hien.top}` : "không thấy"}` });
    await t.context.close();
  }

  // ------------------------------------------------------------ sticker tray
  if (chay("sticker")) {
    const t = await moQuaTin("C1", P0, chatCua(G20));
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(800);
    const moKhay = async () => {
      await bam(t, cdp, "Thêm vào cuộc trò chuyện", { cho: 600 });
      await bam(t, cdp, "Sticker", { cho: 900 });
      return demDialog(t.page);
    };
    const kq = {};
    for (const cach of ["esc", "nen", "back"]) {
      const d0 = await moKhay();
      const f = await focusHienTai(t.page);
      await dong(t.page, cdp, cach);
      await t.page.waitForTimeout(900);
      kq[cach] = { mo: d0, focus: f?.ten ?? f?.role ?? null, con: await demDialog(t.page), duong: await duongDan(t.page) };
      if (!/\/chat$/.test(kq[cach].duong)) {
        await t.page.evaluate((p) => { history.pushState({}, "", p); dispatchEvent(new PopStateEvent("popstate")); }, chatCua(G20));
        await choOn(t.page, { mang: t.mang });
        await t.page.waitForTimeout(900);
      }
    }
    const truoc = (await tinNhom(G20, phien0)).filter((m) => m.kind === "sticker").length;
    await moKhay();
    await chup(t.page, { out: mt.out, id: "EV-F05-STICKER-KHAY-C1", suKien: t.suKien });
    const s1 = await t.page.evaluate(() => {
      // Not the full-screen backdrop («Đóng») nor the close button («Đóng bảng»).
      const e = [...document.querySelectorAll('[data-testid="khay-sticker"] [role="button"]')].find((x) => !/^Đóng/.test(x.getAttribute("aria-label") ?? "") && x.getBoundingClientRect().width > 40 && x.getBoundingClientRect().width < 300);
      if (!e) return null;
      const r = e.getBoundingClientRect();
      return { x: r.left + r.width / 2, y: r.top + r.height / 2, ten: e.getAttribute("aria-label"), w: Math.round(r.width), h: Math.round(r.height) };
    });
    if (s1) await cham(cdp, s1);
    await t.page.waitForTimeout(2000);
    const sau = (await tinNhom(G20, phien0)).filter((m) => m.kind === "sticker").length;
    log({ kq, s1, truoc, sau });
    const f = (x) => `mở ${x.mo}, focus «${x.focus}», sau đóng ${x.con} hộp thoại, ở ${x.duong.replace(/[0-9a-f-]{36}/, "[id]")}`;
    ket({ tc: "TC-L19-VONGDOI", screen: "F05.S02", layer: "L19", state: "khay sticker", action: "mở từ «+» → «Sticker»; đóng bằng Esc, nền, Back; mở lại và chọn một sticker", cauHinh: "C1", expected: "Esc và nền đóng khay; Back đóng khay mà không rời chat; chọn thì gửi một sticker", status: kq.esc.con === 0 && kq.nen.con === 0 && kq.back.con === 0 && /\/chat$/.test(kq.back.duong) && sau - truoc === 1 ? "PASS" : "FAIL", evidence: ["EV-F05-STICKER-KHAY-C1"], ghiChu: `Esc: ${f(kq.esc)}; nền: ${f(kq.nen)}; Back: ${f(kq.back)}; sticker «${s1?.ten}» ${s1?.w}×${s1?.h}, tin sticker ${truoc} → ${sau}` });
    await t.context.close();
  }

  // ------------------------------------------------- the menu of one message
  if (chay("menu")) {
    const t = await mo("C1", P0, chatCua(G20));
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(800);
    const chu = `Tin kiểm thử menu ${Date.now() % 100000}`;
    await guiTin(t, cdp, chu);
    const moMenu = async () => {
      const b = await bongCua(t.page, chu);
      await cham(cdp, b);
      await t.page.waitForTimeout(900);
    };
    await moMenu();
    const phanUng = await t.page.evaluate(() => [...document.querySelectorAll('[data-testid="menu-tin"] [role="button"]')].filter((e) => ["Thích", "Haha", "Đồng ý", "Wow", "Buồn", "Cháy"].includes(e.getAttribute("aria-label"))).map((e) => { const r = e.getBoundingClientRect(); return { ten: e.getAttribute("aria-label"), w: Math.round(r.width), h: Math.round(r.height) }; }));
    await chup(t.page, { out: mt.out, id: "EV-F05-MENU-C1", suKien: t.suKien });
    ket({ tc: "TC-F05-MENU-PHAN-UNG", screen: "F05.S02", layer: "L18", state: "menu của một tin", action: "đo sáu nút cảm xúc", cauHinh: "C1", expected: "mỗi nút ≥ 48dp (DESIGN.md), tối thiểu 44 (PRODUCT)", status: phanUng.length === 6 && phanUng.every((p) => p.w >= 48 && p.h >= 48) ? "PASS" : "FAIL", evidence: ["EV-F05-MENU-C1"], ghiChu: phanUng.map((p) => `${p.ten} ${p.w}×${p.h}`).join(", ") });
    await bam(t, cdp, "Thích", { cho: 1500 });
    const chip = await t.page.evaluate(() => { const e = [...document.querySelectorAll('[aria-label^="1 "]')].find((x) => /❤/.test(x.getAttribute("aria-label"))); if (!e) return null; const r = e.getBoundingClientRect(); return { ten: e.getAttribute("aria-label"), w: Math.round(r.width), h: Math.round(r.height) }; });
    ket({ tc: "TC-F05-PHAN-UNG", screen: "F05.S02", state: "menu mở", action: "chạm «Thích»", cauHinh: "C1", expected: "menu đóng, chip ❤️ 1 hiện dưới bong bóng, chip ≥ 48dp", status: chip && chip.h >= 48 && (await demDialog(t.page)) === 0 ? "PASS" : "FAIL", evidence: [], ghiChu: chip ? `chip «${chip.ten}» ${chip.w}×${chip.h}` : "không thấy chip" });
    // Reply.
    await moMenu();
    await bam(t, cdp, "Trả lời", { cho: 800 });
    const dai = await t.page.evaluate(() => { const e = document.querySelector('[aria-label^="Đang trả lời"]'); return e ? e.getAttribute("aria-label") : null; });
    const traLoi = `Trả lời kiểm thử ${Date.now() % 100000}`;
    await guiTin(t, cdp, traLoi);
    // The quote of THIS reply: the row holding the reply bubble (the list is
    // inverted, so document order is not screen order).
    const trich = await t.page.evaluate((traLoi) => {
      const bong = [...document.querySelectorAll('[role="button"]')].find((e) => e.getAttribute("aria-label") === `Tin nhắn: ${traLoi}`);
      return bong?.closest('[data-testid^="chat-message-"]')?.querySelector('[aria-label^="Trích: "]')?.getAttribute("aria-label") ?? null;
    }, traLoi);
    ket({ tc: "TC-F05-TRA-LOI", screen: "F05.S02", state: "menu mở", action: "«Trả lời», gõ, gửi", cauHinh: "C1", expected: "dải «Đang trả lời …» trên ô soạn; tin gửi đi mang trích dẫn", status: dai && trich && trich.includes(chu) ? "PASS" : "FAIL", evidence: [], ghiChu: `dải: «${dai}»; trích dưới tin mới: «${trich}»` });
    // Copy.
    await t.context.grantPermissions(["clipboard-read", "clipboard-write"]).catch(() => undefined);
    await moMenu();
    await bam(t, cdp, "Sao chép", { cho: 800 });
    const clip = await t.page.evaluate(async () => { try { return await navigator.clipboard.readText(); } catch (e) { return `lỗi: ${e.name}`; } });
    const menuDong = (await demDialog(t.page)) === 0;
    ket({ tc: "TC-F05-SAO-CHEP", screen: "F05.S02", state: "menu mở", action: "«Sao chép»", cauHinh: "C1", expected: "chữ của tin nằm trong clipboard; menu đóng", status: clip === chu && menuDong ? "PASS" : "FAIL", evidence: [], ghiChu: `clipboard «${String(clip).slice(0, 60)}»; menu ${menuDong ? "đóng" : "còn"}; không có câu xác nhận nào (đúng luật không toast)` });
    // Delete with the confirmation step.
    await moMenu();
    await bam(t, cdp, "Xoá", { cho: 800 });
    const hoi = await coChu(t.page, /Xoá tin này\?/);
    await bam(t, cdp, "Xoá tin", { cho: 2000 });
    const daXoa = await t.page.evaluate(() => !!document.querySelector('[aria-label="Tin nhắn đã bị xoá"]'));
    const conChu = (await bongCua(t.page, chu)) !== null;
    await chup(t.page, { out: mt.out, id: "EV-F05-XOA-C1", suKien: t.suKien });
    ket({ tc: "TC-F05-XOA", screen: "F05.S02", layer: "L18", state: "tin của mình", action: "«Xoá» → «Xoá tin»", cauHinh: "C1", expected: "hỏi trong sheet; sau đó tin thành «Tin nhắn đã bị xoá»", status: hoi && daXoa && !conChu ? "PASS" : "FAIL", evidence: ["EV-F05-XOA-C1"], ghiChu: `câu hỏi ${hoi ? "có" : "không"}; «Tin nhắn đã bị xoá» ${daXoa ? "có" : "không"}; bong bóng cũ ${conChu ? "còn" : "hết"}` });
    await t.context.close();
  }

  // ---------------------------------------------------- closing the menu sheet
  if (chay("menu-dong")) {
    const t = await moQuaTin("C1", P0, chatCua(G20));
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(800);
    const kq = {};
    for (const cach of ["esc", "nen", "keo-dai", "back"]) {
      const b = await t.page.evaluate(() => { const ds = [...document.querySelectorAll('[role="button"][aria-label^="Tin nhắn: "]')].filter((e) => { const r = e.getBoundingClientRect(); return r.top > 200 && r.bottom < 700; }); const e = ds.pop(); if (!e) return null; const r = e.getBoundingClientRect(); return { x: r.left + r.width / 2, y: r.top + r.height / 2 }; });
      await cham(cdp, b);
      await t.page.waitForTimeout(900);
      const mo0 = await demDialog(t.page);
      await dong(t.page, cdp, cach);
      await t.page.waitForTimeout(900);
      kq[cach] = { mo: mo0, con: await demDialog(t.page), duong: await duongDan(t.page) };
      if (!/\/chat$/.test(kq[cach].duong)) {
        await t.page.evaluate((p) => { history.pushState({}, "", p); dispatchEvent(new PopStateEvent("popstate")); }, chatCua(G20));
        await choOn(t.page, { mang: t.mang });
        await t.page.waitForTimeout(900);
      }
    }
    log({ kq });
    const f = (x) => `${x.mo} → ${x.con} hộp thoại, ở ${x.duong.replace(/[0-9a-f-]{36}/, "[id]")}`;
    ket({ tc: "TC-L18-VONGDOI", screen: "F05.S02", layer: "L18", state: "menu của một tin", action: "mở bằng chạm; đóng bằng Esc, nền, kéo xuống, Back", cauHinh: "C1", expected: "mọi cách đóng đều đóng menu và ở lại chat", status: Object.values(kq).every((x) => x.mo === 1 && x.con === 0 && /\/chat$/.test(x.duong)) ? "PASS" : "FAIL", evidence: [], ghiChu: Object.entries(kq).map(([k, v]) => `${k}: ${f(v)}`).join("; ") });
    await t.context.close();
  }

  // ------------------------------------------------------ reporting a message
  if (chay("bao-cao")) {
    // A fresh line by Chat Test 02, so it sits at the newest end: scrolling the
    // inverted list to an old one races the list's own jump back to the end.
    const cuaNguoiKhac = `Tin để báo cáo ${Date.now() % 100000}`;
    await goiApi(mt.api, "POST", `/contexts/${G20}/messages`, { kind: "text", body: cuaNguoiKhac, image_url: null, card: null }, phien1.token);
    const t = await mo("C1", P0, chatCua(G20));
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(1200);
    const b = await bongCua(t.page, cuaNguoiKhac);
    await cham(cdp, b);
    await t.page.waitForTimeout(900);
    await bam(t, cdp, "Báo cáo", { cho: 1200 });
    const tieuDe = await coChu(t.page, /Vì sao bạn báo cáo\?/);
    const axe = await chayAxe(t.page).catch((e) => ({ loi: String(e).slice(0, 100) }));
    const nhom = await t.page.evaluate(() => { const g = document.querySelector('[role="radiogroup"]'); if (!g) return null; return [...g.querySelectorAll('[role="button"],[role="radio"]')].map((e) => ({ vai: e.getAttribute("role"), ten: (e.innerText ?? "").trim(), chon: e.getAttribute("aria-checked") ?? e.getAttribute("aria-pressed") })); });
    await bam(t, cdp, "Làm phiền, quấy rối", { cho: 400 });
    await chup(t.page, { out: mt.out, id: "EV-F05-BAO-CAO-C1", suKien: t.suKien });
    await bam(t, cdp, "Gửi báo cáo", { cho: 2000 });
    const daGui = await coChu(t.page, /Đã gửi báo cáo/);
    await bam(t, cdp, "Xong", { cho: 800 });
    const conHop = await demDialog(t.page);
    log({ tieuDe, nhom, axe: axe.vi?.map((v) => `${v.rule}×${v.so}`), daGui, conHop });
    const axeTom = axe.vi ? axe.vi.map((v) => `${v.rule}×${v.so}`).join(", ") || "sạch" : axe.loi;
    ket({ tc: "TC-L16-VONGDOI", screen: "F05.S02", layer: "L16", state: "tin của người khác", action: "menu → «Báo cáo» → chọn lý do → «Gửi báo cáo» → «Xong»", cauHinh: "C1", expected: "sheet hỏi lý do; lựa chọn đang chọn đọc được bằng công nghệ hỗ trợ; gửi xong có xác nhận; «Xong» đóng sạch", status: tieuDe && daGui && conHop === 0 && nhom && nhom.every((x) => x.chon !== null) ? "PASS" : "FAIL", evidence: ["EV-F05-BAO-CAO-C1"], ghiChu: `radiogroup có ${nhom?.length ?? 0} lựa chọn, vai ${[...new Set((nhom ?? []).map((x) => x.vai))].join("/")}, trạng thái chọn trong DOM: ${(nhom ?? []).some((x) => x.chon !== null) ? "có" : "không"}; axe: ${axeTom}; gửi ${daGui ? "xong" : "không"}; sau «Xong» ${conHop} hộp thoại` });
    await t.context.close();
  }

  // ------------------------------------ tools tray, a poll, the shared sheet
  if (chay("cong-cu")) {
    const t = await moQuaTin("C1", P0, chatCua(G20));
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(800);
    const khayMo = () => coChu(t.page, /Thêm vào cuộc trò chuyện\n|Bình chọn|Tờ hẹn/).then(async () => !!(await timNut(t.page, "Đóng khay công cụ")));
    const kq = {};
    for (const cach of ["x", "esc", "back"]) {
      await bam(t, cdp, "Thêm vào cuộc trò chuyện", { cho: 700 });
      const mo0 = await khayMo();
      if (cach === "x") await bam(t, cdp, "Đóng khay công cụ", { cho: 600 });
      else if (cach === "esc") { await t.page.keyboard.press("Escape"); await t.page.waitForTimeout(600); }
      else { await t.page.goBack(); await t.page.waitForTimeout(1200); }
      kq[cach] = { mo: mo0, con: await khayMo(), duong: await duongDan(t.page) };
      if (!/\/chat$/.test(kq[cach].duong)) {
        await t.page.evaluate((p) => { history.pushState({}, "", p); dispatchEvent(new PopStateEvent("popstate")); }, chatCua(G20));
        await choOn(t.page, { mang: t.mang });
        await t.page.waitForTimeout(900);
      }
    }
    log({ kq });
    const f = (x) => `${x.mo ? "mở" : "không mở"} → ${x.con ? "còn" : "đóng"}, ở ${x.duong.replace(/[0-9a-f-]{36}/, "[id]")}`;
    ket({ tc: "TC-L20-VONGDOI", screen: "F05.S02", layer: "L20", state: "khay công cụ", action: "mở bằng «+»; đóng bằng X, Esc, Back trình duyệt", cauHinh: "C1", expected: "X và Esc đóng khay; Back đóng khay mà không rời chat (Android: BackHandler đã làm vậy)", status: Object.values(kq).every((x) => x.mo && !x.con && /\/chat$/.test(x.duong)) ? "PASS" : "FAIL", evidence: [], ghiChu: Object.entries(kq).map(([k, v]) => `${k}: ${f(v)}`).join("; ") });

    // A poll: create, vote, close, open the shared sheet.
    const cauHoi = `Kiểm thử bình chọn ${Date.now() % 100000}`;
    await bam(t, cdp, "Thêm vào cuộc trò chuyện", { cho: 700 });
    await bam(t, cdp, "Bình chọn", { cho: 900 });
    await t.page.locator('[aria-label="Câu hỏi bình chọn"]').fill(cauHoi);
    const oLuaChon = t.page.locator('input[aria-label^="Lựa chọn"]');
    const soO = await oLuaChon.count();
    if (soO >= 2) {
      await oLuaChon.nth(0).fill("Lẩu");
      await oLuaChon.nth(1).fill("Nướng");
    }
    await bam(t, cdp, "Gửi bình chọn", { cho: 2500 });
    const phieu = await timNut(t.page, "Bỏ phiếu Lẩu");
    ket({ tc: "TC-F05-BINH-CHON-TAO", screen: "F05.S02", layer: "L20", state: "khay «Bình chọn»", action: "gõ câu hỏi và 2 lựa chọn, «Gửi bình chọn»", cauHinh: "C1", expected: "thẻ bình chọn hiện trong chat, khay đóng", status: phieu ? "PASS" : "FAIL", evidence: [], ghiChu: `ô lựa chọn tìm thấy ${soO}; thẻ ${phieu ? "có" : "không"} («Bỏ phiếu Lẩu»)` });
    if (phieu) {
      await cham(cdp, phieu);
      await t.page.waitForTimeout(1500);
      const r = await t.page.evaluate(() => { const e = document.querySelector('[aria-label="Bỏ phiếu Lẩu"]'); return { vai: e?.getAttribute("role"), chon: e?.getAttribute("aria-checked") }; });
      ket({ tc: "TC-F05-BINH-CHON-PHIEU", screen: "F05.S02", state: "thẻ bình chọn", action: "chạm «Lẩu»", cauHinh: "C1", expected: "phiếu được ghi; lựa chọn của mình báo aria-checked", status: r.chon === "true" ? "PASS" : "FAIL", issue: r.chon === "true" ? null : "UI-003", evidence: [], ghiChu: `role ${r.vai}, aria-checked ${r.chon ?? "không có"}` });
      await bam(t, cdp, "Chốt bình chọn", { cho: 700 });
      await bam(t, cdp, "Đóng bình chọn", { cho: 2000 });
      const nut = await timNut(t.page, "Mở tờ hẹn chung cho lựa chọn này");
      if (nut) {
        await cham(cdp, nut);
        await t.page.waitForTimeout(2500);
      }
      const coKhay = async () => !!(await timNut(t.page, "Đóng tờ hẹn chung"));
      const kq2 = { mo: await coKhay() };
      await chup(t.page, { out: mt.out, id: "EV-F05-TO-HEN-C1", suKien: t.suKien });
      await t.page.keyboard.press("Escape");
      await t.page.waitForTimeout(700);
      kq2.esc = await coKhay();
      await t.page.goBack();
      await t.page.waitForTimeout(1200);
      kq2.backDuong = await duongDan(t.page);
      log({ kq2 });
      ket({ tc: "TC-L21-VONGDOI", screen: "F05.S02", layer: "L21", state: "tờ hẹn chung vừa mở từ bình chọn đã chốt", action: "Esc, rồi Back trình duyệt", cauHinh: "C1", expected: "Esc đóng khay như khay công cụ; Back đóng khay mà không rời chat", status: kq2.mo && !kq2.esc && /\/chat$/.test(kq2.backDuong) ? "PASS" : "FAIL", evidence: ["EV-F05-TO-HEN-C1"], ghiChu: `«Mở tờ hẹn chung…» ${nut ? "có" : "không có"}; khay ${kq2.mo ? "mở" : "không mở"}; sau Esc khay ${kq2.esc ? "còn" : "đóng"}; sau Back ở ${kq2.backDuong.replace(/[0-9a-f-]{36}/, "[id]")}` });
    }
    await t.context.close();
  }

  // ------------------------------------------------------------ group settings
  if (chay("cai-dat")) {
    const t = await mo("C1", P0, chatCua(G20));
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(800);
    await bam(t, cdp, "Cài đặt nhóm", { cho: 1000 });
    const mo0 = await demDialog(t.page);
    const sw = await t.page.evaluate(() => { const e = document.querySelector('[aria-label="Rủ Đi AI tự gợi ý"]'); const i = e?.matches("input") ? e : e?.querySelector("input"); return e ? { vai: e.getAttribute("role") ?? i?.getAttribute("role") ?? i?.type, chon: e.getAttribute("aria-checked") ?? i?.getAttribute("aria-checked") ?? String(i?.checked) } : null; });
    const theme = await t.page.evaluate(() => [...document.querySelectorAll('[aria-label^="Theme "]')].map((e) => ({ ten: e.getAttribute("aria-label"), vai: e.getAttribute("role"), chon: e.getAttribute("aria-checked") ?? e.getAttribute("aria-selected") ?? e.getAttribute("aria-pressed"), w: Math.round(e.getBoundingClientRect().width), h: Math.round(e.getBoundingClientRect().height) })));
    await chup(t.page, { out: mt.out, id: "EV-F05-CAI-DAT-C1", suKien: t.suKien });
    await bam(t, cdp, "Rời nhóm", { cho: 700 });
    const hoiRoi = await coChu(t.page, /Ở lại/);
    await bam(t, cdp, "Ở lại", { cho: 600 });
    await t.page.keyboard.press("Escape");
    await t.page.waitForTimeout(800);
    const sauEsc = await demDialog(t.page);
    log({ mo0, sw, theme, hoiRoi, sauEsc });
    ket({ tc: "TC-L17-VONGDOI", screen: "F05.S02", layer: "L17", state: "sheet Cài đặt nhóm", action: "mở bằng «⋯»; đọc 5 màu (và công tắc nếu có); «Rời nhóm» → «Ở lại»; Esc", cauHinh: "C1", expected: "màu đang chọn báo trạng thái (radio có aria-checked); rời nhóm có bước hỏi; Esc đóng sạch; ô màu ≥ 48dp", status: mo0 === 1 && hoiRoi && sauEsc === 0 && theme.length === 5 && theme.every((x) => x.chon !== null && x.w >= 48 && x.h >= 48) ? "PASS" : "FAIL", evidence: ["EV-F05-CAI-DAT-C1"], ghiChu: `công tắc «Rủ Đi AI tự gợi ý» ${sw ? `có, vai ${sw.vai}, trạng thái ${sw.chon}` : "không có ở chat thật (GroupChatLive không truyền aiTuGoiY; khớp luật AI chỉ nhận nội dung được gọi)"}; màu: ${theme.map((x) => `${x.ten?.slice(6)} ${x.vai} ${x.chon ?? "không trạng thái"} ${x.w}×${x.h}`).join(", ")}; hỏi trước khi rời ${hoiRoi ? "có" : "không"}; sau Esc ${sauEsc} hộp thoại` });
    await t.context.close();
  }

  // -------------------- a friend writes while I read old messages up the thread
  if (chay("tin-moi")) {
    const t = await mo("C1", P0, chatCua(G20));
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(1000);
    await cuonLichSu(t.page, 1400);
    await t.page.waitForTimeout(1200);
    const truoc = await trangThaiCuon(t.page);
    const chu = `Tin mới từ Chat Test 02 ${Date.now() % 100000}`;
    await goiApi(mt.api, "POST", `/contexts/${G20}/messages`, { kind: "text", body: chu, image_url: null, card: null }, phien1.token);
    await t.page.waitForFunction((chu) => [...document.querySelectorAll('[role="button"][aria-label^="Tin nhắn: "]')].some((e) => e.getAttribute("aria-label") === `Tin nhắn: ${chu}`), chu, { timeout: 15_000 }).catch(() => undefined);
    await t.page.waitForTimeout(800);
    const sau = await trangThaiCuon(t.page);
    await chup(t.page, { out: mt.out, id: "EV-F05-TIN-MOI-C1", suKien: t.suKien });
    const giuCho = truoc.thay[0] && sau.thay.some((x) => x.ten === truoc.thay[0].ten && Math.abs(x.top - truoc.thay[0].top) <= 4);
    if (sau.pill) await bam(t, cdp, "Về tin nhắn mới nhất", { cho: 900 });
    const cuoi = await bongCua(t.page, chu);
    log({ truoc: { ...truoc, thay: truoc.thay.slice(0, 2) }, sau: { ...sau, thay: sau.thay.slice(0, 2) }, cuoi });
    ket({ tc: "TC-F05-TIN-MOI", screen: "F05.S02", state: "đang đọc tin cũ (cuộn lên 1400px)", action: "Chat Test 02 gửi một tin qua API", cauHinh: "C1", expected: "chỗ đang đọc đứng yên; có nút «Tin mới nhất»; chạm thì tới tin mới", status: giuCho && sau.pill && cuoi && cuoi.top > 0 && cuoi.top < 844 ? "PASS" : "FAIL", evidence: ["EV-F05-TIN-MOI-C1"], ghiChu: `tin đầu đang thấy «${truoc.thay[0]?.ten}» y ${truoc.thay[0]?.top} → ${sau.thay.find((x) => x.ten === truoc.thay[0]?.ten)?.top ?? "mất khỏi màn"}; cuộn ${truoc.scrollTop} → ${sau.scrollTop}; nút «Tin mới nhất» ${sau.pill ? "có" : "không"}; sau khi chạm, tin mới ${cuoi ? `ở y ${cuoi.top}` : "không thấy"}` });
    await t.context.close();
  }

  // ---------------------- layout against the common messenger conventions
  // The requester (27/09) judged the thread and the poll card against
  // Messenger: where a bubble sits, how times show, the composer's alignment
  // and spare room, long messages dropping out of view, the poll's look. Each
  // of those is measured here as a fact; the judgement stays in issues.md.
  if (chay("bo-cuc")) {
    for (const [ten, persona, ctx] of [["G8", P8, G8], ["G20", P0, G20]]) {
      const t = await mo("C1", persona, chatCua(ctx));
      await t.page.waitForTimeout(1200);
      const r = await t.page.evaluate(() => {
        const hang = [...document.querySelectorAll('[data-testid^="chat-message-"]')].filter((h) => { const b = h.getBoundingClientRect(); return b.bottom > 0 && b.top < innerHeight; });
        const lech = [];
        for (const h of hang) {
          const av = [...h.children].find((c) => c.getBoundingClientRect().width <= 34 && c.getBoundingClientRect().width >= 26 && /^\S{1,2}$/.test((c.innerText ?? "").trim()));
          const bong = h.querySelector('[role="button"][aria-label^="Tin nhắn: "]');
          if (av && bong) lech.push(Math.round(av.getBoundingClientRect().bottom - bong.getBoundingClientRect().bottom));
        }
        const gio = [...document.querySelectorAll("div,span")].filter((e) => /^\d{2}:\d{2}$/.test((e.innerText ?? "").trim()) && e.children.length === 0 && e.getBoundingClientRect().height > 0 && e.getBoundingClientRect().bottom > 0 && e.getBoundingClientRect().top < innerHeight);
        const soBong = document.querySelectorAll('[role="button"][aria-label^="Tin nhắn: "]').length;
        // The bubble is the only thing that opens a message's menu: its height is a tap target.
        const caoBong = [...document.querySelectorAll('[role="button"][aria-label^="Tin nhắn: "]')].filter((e) => { const b = e.getBoundingClientRect(); return b.height > 0 && b.bottom > 0 && b.top < innerHeight; }).map((e) => Math.round(e.getBoundingClientRect().height));
        const pill = document.querySelector('textarea[aria-label="Ô soạn tin"]')?.closest('div[style*="border-radius"]') ?? document.querySelector('textarea[aria-label="Ô soạn tin"]')?.parentElement;
        const o = document.querySelector('textarea[aria-label="Ô soạn tin"]');
        const s = getComputedStyle(o);
        const or = o.getBoundingClientRect();
        const lh = parseFloat(s.lineHeight) || 24;
        const cong = document.querySelector('[aria-label="Thêm vào cuộc trò chuyện"]').getBoundingClientRect();
        const gui = document.querySelector('[aria-label="Gửi tin nhắn"]').getBoundingClientRect();
        const pr = pill?.getBoundingClientRect();
        return {
          avatarDuoiBong: lech,
          gio: { hien: gio.length, cao: gio[0] ? Math.round(gio[0].getBoundingClientRect().height) : null, giaTri: [...new Set(gio.map((g) => g.innerText.trim()))] },
          soBong,
          bongThapNhat: caoBong.length ? Math.min(...caoBong) : null,
          soan: { vien: pr ? Math.round(pr.height) : null, o: Math.round(or.height), chuGiua: Math.round(or.top + parseFloat(s.paddingTop) + lh / 2), congGiua: Math.round(cong.top + cong.height / 2), guiGiua: Math.round(gui.top + gui.height / 2), duoiChu: Math.round(or.bottom - (or.top + parseFloat(s.paddingTop) + lh)) },
        };
      });
      // The poll card, when this thread has one.
      const binhChon = await t.page.evaluate(() => {
        const lc = [...document.querySelectorAll('[role="radio"][aria-label^="Bỏ phiếu "]')];
        if (!lc.length) return null;
        const the = lc[0].closest('[data-testid^="chat-message-"]');
        const rt = the?.getBoundingClientRect();
        const txt = the?.innerText ?? "";
        return { soLuaChon: lc.length, caoLuaChon: lc.map((e) => Math.round(e.getBoundingClientRect().height)), caoThe: rt ? Math.round(rt.height) : null, phanManHinh: rt ? Math.round((rt.height / innerHeight) * 100) : null, lapPhieu: (txt.match(/\d+ phiếu/g) ?? []).length, coThanhTiLe: !!the?.querySelector('[role="progressbar"],[aria-valuenow]'), ghim: !!document.querySelector('[data-testid="day-ghim"]') };
      });
      if (binhChon) {
        await t.page.evaluate(() => { const e = document.querySelector('[role="radio"][aria-label^="Bỏ phiếu "]'); e?.closest('[data-testid^="chat-message-"]')?.setAttribute("data-audit-binh-chon", "1"); });
      }
      await chup(t.page, { out: mt.out, id: `EV-F05-BO-CUC-${ten}-C1`, suKien: t.suKien, chuThich: [{ selector: 'textarea[aria-label="Ô soạn tin"]', nhan: "ô soạn" }, ...(binhChon ? [{ selector: "[data-audit-binh-chon]", nhan: "thẻ bình chọn" }] : [])] });
      log({ ten, r, binhChon });
      ket({ tc: "TC-F05-BO-CUC-TIN", screen: "F05.S02", state: `chat ${ten === "G8" ? "Team Đà Lạt (13 tin)" : "nhóm 20 người (lịch sử dài)"}`, action: "đo vị trí avatar, giờ, ô soạn", cauHinh: `C1 ${ten}`, expected: "avatar thẳng đáy bong bóng cuối của cụm; giờ không lặp dưới mọi cụm (quy ước Messenger: dải giờ khi cách quãng); ô soạn một dòng: chữ, «+» và nút gửi cùng một đường giữa, không dư chỗ dưới chữ", status: r.avatarDuoiBong.every((d) => Math.abs(d) <= 4) && Math.abs(r.soan.chuGiua - r.soan.congGiua) <= 4 && r.soan.duoiChu <= 12 ? "PASS" : "FAIL", evidence: [`EV-F05-BO-CUC-${ten}-C1`], ghiChu: `avatar thấp hơn đáy bong bóng ${[...new Set(r.avatarDuoiBong)].join("/")}px; ${r.gio.hien} nhãn giờ trên màn cho ${r.soBong} tin (giá trị ${r.gio.giaTri.slice(0, 3).join(", ")}); ô soạn: viền ${r.soan.vien}, ô ${r.soan.o}, giữa dòng chữ y ${r.soan.chuGiua} so với giữa «+» y ${r.soan.congGiua} và nút gửi y ${r.soan.guiGiua}, dư ${r.soan.duoiChu}px dưới dòng chữ; bong bóng thấp nhất trên màn ${r.bongThapNhat}px (chạm vào bong bóng mới mở menu tin)` });
      if (binhChon) ket({ tc: "TC-F05-BINH-CHON-THE", screen: "F05.S02", state: `thẻ bình chọn (${ten})`, action: "đo thẻ", cauHinh: `C1 ${ten}`, expected: "thẻ gọn so với nội dung; số phiếu không lặp; có tỉ lệ nhìn được (quy ước phổ biến: thanh tỉ lệ, người đã bầu)", status: binhChon.phanManHinh <= 25 && binhChon.coThanhTiLe ? "PASS" : "FAIL", evidence: [`EV-F05-BO-CUC-${ten}-C1`], ghiChu: `${binhChon.soLuaChon} lựa chọn cao ${binhChon.caoLuaChon.join("/")}px; thẻ cao ${binhChon.caoThe}px = ${binhChon.phanManHinh}% chiều cao màn; chữ «N phiếu» xuất hiện ${binhChon.lapPhieu} lần; thanh tỉ lệ ${binhChon.coThanhTiLe ? "có" : "không"}; dải ghim trùng thẻ ${binhChon.ghim ? "có" : "không"}` });
      await t.context.close();
    }
  }

  // ------------------------------- a poll with ballots (the thumbprint state)
  // The seeded poll has no ballot, so its card shows «0 phiếu» four times and
  // no print. Here a three-choice poll in the chat-test group gets twelve
  // ballots over the API (7/4/1, one of the four is the viewer's), then is read
  // open at C1 and C3, the way a member sees it. Ballots are idempotent per
  // voter, so a second run changes nothing.
  if (chay("phieu")) {
    // One question mark, at the end: the form refuses any other («Đặt dấu hỏi ở cuối câu hỏi.»).
    const CAU = "Cuối tuần này đi đâu, nhóm kiểm thử phiếu?";
    const LUA = ["Đồi chè Cầu Đất", "Hồ Tuyền Lâm", "Ở nhà ngủ"];
    const tim = async () => ((await goiApi(mt.api, "GET", `/contexts/${G20}/votes`, undefined, phien0.token)).votes ?? []).find((v) => v.question === CAU);
    let vote = await tim();
    if (!vote) {
      const t = await moQuaTin("C1", P0, chatCua(G20));
      const cdp = await cdpCua(t.page);
      await bam(t, cdp, "Thêm vào cuộc trò chuyện", { cho: 700 });
      await bam(t, cdp, "Bình chọn", { cho: 900 });
      await t.page.locator('[aria-label="Câu hỏi bình chọn"]').fill(CAU);
      await bam(t, cdp, "Thêm lựa chọn", { cho: 400 });
      const o = t.page.locator('input[aria-label^="Lựa chọn"]');
      for (const [i, s] of LUA.entries()) await o.nth(i).fill(s);
      await bam(t, cdp, "Gửi bình chọn", { cho: 2500 });
      await t.context.close();
      vote = await tim();
    }
    if (!vote) throw new Error("bình chọn kiểm thử chưa được tạo");
    const idCua = (nhan) => vote.options.find((x) => x.label === nhan)?.id;
    const chia = [[0, 1], [1, 0], [2, 0], [3, 0], [4, 0], [5, 0], [6, 0], [7, 0], [8, 1], [9, 1], [10, 1], [11, 2]];
    const bo = [];
    for (const [i, lc] of chia) {
      try {
        const p = await layPhien(mt.api, personaTheoTen(`chat-${i}`, mt.chatSessions), join(mt.out, "phien"));
        await goiApi(mt.api, "POST", `/votes/${vote.id}/ballots`, { option_id: idCua(LUA[lc]) }, p.token);
        bo.push(i);
      } catch (e) {
        log({ phieuTuChoi: i, loi: String(e).slice(0, 80) });
      }
    }
    const ket0 = await goiApi(mt.api, "GET", `/votes/${vote.id}`, undefined, phien0.token);
    for (const cfg of ["C1", "C3"]) {
      const t = await mo(cfg, P0, chatCua(G20));
      await t.page.waitForTimeout(1500);
      const r = await t.page.evaluate(({ nhan, NGANG }) => {
        const dau = document.querySelector(`[role="radio"][aria-label="Bỏ phiếu ${nhan}"]`);
        const the = dau?.closest('[data-testid^="chat-message-"]');
        if (!the) return null;
        the.scrollIntoView({ block: "center", behavior: "instant" });
        new Function("e", NGANG)(the);
        the.setAttribute("data-audit-phieu", "1");
        const rt = the.getBoundingClientRect();
        const lc = [...the.querySelectorAll('[role="radio"]')].map((e) => {
          const b = e.getBoundingClientRect();
          // Prints are drawn layers inside the choice: count the drawn boxes of print size.
          const in_ = [...e.querySelectorAll("svg,canvas")].filter((s) => { const q = s.getBoundingClientRect(); return q.width >= 10 && q.width <= 40; });
          const q0 = in_[0]?.getBoundingClientRect();
          return { ten: e.getAttribute("aria-label").slice(9), cao: Math.round(b.height), dau: in_.length, dauCo: q0 ? `${Math.round(q0.width)}×${Math.round(q0.height)}` : null, chu: (e.innerText ?? "").replace(/\s+/g, " ").trim() };
        });
        const txt = the.innerText ?? "";
        return { caoThe: Math.round(rt.height), phan: Math.round((rt.height / innerHeight) * 100), lc, lapPhieu: (txt.match(/\d+ phiếu/g) ?? []).length, top: Math.round(rt.top), bottom: Math.round(rt.bottom) };
      }, { nhan: LUA[0], NGANG });
      await t.page.waitForTimeout(400);
      await chup(t.page, { out: mt.out, id: `EV-F05-PHIEU-${cfg}`, suKien: t.suKien, chuThich: r ? [{ selector: "[data-audit-phieu]", nhan: "bình chọn 12 phiếu" }] : [] });
      log({ cfg, r, tong: ket0.total_ballots });
      ket({ tc: "TC-F05-BINH-CHON-CO-PHIEU", screen: "F05.S02", state: `bình chọn mở, 3 lựa chọn, ${ket0.total_ballots} phiếu (7/4/1, một phiếu của người xem)`, action: "mở chat, cuộn tới thẻ", cauHinh: cfg, expected: "đọc được bên nào hơn mà không phải đếm; phiếu của mình nổi rõ; thẻ nằm trọn trong màn", status: r && r.top >= 0 && r.bottom <= (cfg === "C3" ? 800 : 844) && r.lc.every((x) => x.dau === Math.min(12, Number(/(\d+) phiếu/.exec(x.chu)?.[1] ?? -1))) ? "PASS" : "FAIL", evidence: [`EV-F05-PHIEU-${cfg}`], ghiChu: r ? `thẻ cao ${r.caoThe}px = ${r.phan}% màn; ${r.lc.map((x) => `«${x.ten}» cao ${x.cao}, ${x.dau} dấu vân tay ${x.dauCo ?? ""}, chữ «${x.chu.slice(x.ten.length).trim()}»`).join("; ")}; «N phiếu» ${r.lapPhieu} lần; phiếu đã bỏ ${bo.length}/12` : "không thấy thẻ" });
      await t.context.close();
    }
  }

  // ----------------------------------- the conversations list with nothing in it
  // A brand-new account (a number no seed uses, offset 50, kept apart from the
  // ones F01 signs up): no group, no pair, no invitation.
  if (chay("rong")) {
    const PM = personaTheoTen("moi-50", mt.chatSessions);
    await layPhien(mt.api, PM, join(mt.out, "phien"));
    for (const cfg of ["C1", "C3"]) {
      const t = await mo(cfg, PM, "/messages");
      await t.page.waitForTimeout(1500);
      const r = await t.page.evaluate(() => ({
        duong: location.pathname,
        chu: (document.body.innerText ?? "").replace(/\s+/g, " ").slice(0, 260),
        nut: [...document.querySelectorAll('[role="button"],a[href]')].map((e) => (e.getAttribute("aria-label") || e.innerText || "").trim()).filter((x) => x && x.length < 40).slice(0, 14),
      }));
      await chup(t.page, { out: mt.out, id: `EV-F05-RONG-${cfg}`, suKien: t.suKien });
      log({ cfg, r });
      const coLoi = /lời mời|mã mời/i.test(r.chu) || r.nut.some((x) => /lời mời/i.test(x));
      const coTao = r.nut.some((x) => /Tạo nhóm|Lập nhóm/.test(x));
      ket({ tc: "TC-F05.S01-RONG", screen: "F05.S01", state: "tài khoản mới: 0 nhóm, 0 chat đôi, 0 lời mời", action: "mở Tin nhắn", cauHinh: cfg, expected: "trạng thái rỗng nói đây là chỗ nào, có lối lập nhóm và lối nhập lời mời", status: r.duong === "/messages" && coTao && coLoi ? "PASS" : "FAIL", evidence: [`EV-F05-RONG-${cfg}`], ghiChu: `ở ${r.duong}; lập nhóm ${coTao ? "có" : "không"}; lối lời mời ${coLoi ? "có" : "không"}; nút: ${r.nut.join(" · ")}` });
      await t.context.close();
    }
  }

  // ------------------------- the pinned band over an open sheet's backdrop
  // Seen in the captures of L16 and L17: the band stays bright above the
  // scrim (`dayGhim` has zIndex 1, the sheet sits in the screen's own tree).
  // Measured: what a finger at the band's centre lands on, and what it does.
  if (chay("ghim-phu")) {
    const kq = {};
    // A fresh line by Chat Test 02 to long-press: it sits at the newest end.
    const moc = `Tin để mở menu ${Date.now() % 100000}`;
    await goiApi(mt.api, "POST", `/contexts/${G20}/messages`, { kind: "text", body: moc, image_url: null, card: null }, phien1.token);
    for (const [ten, mo1] of [["L17", async (t, cdp) => bam(t, cdp, "Cài đặt nhóm", { cho: 1000 })], ["L18", async (t, cdp) => {
      await cham(cdp, await bongCua(t.page, moc));
      await t.page.waitForTimeout(900);
    }]]) {
      const t = await moQuaTin("C1", P0, chatCua(G20));
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(800);
      await mo1(t, cdp);
      const truoc = await demDialog(t.page);
      const diem = await t.page.evaluate(() => {
        const band = document.querySelector('[data-testid="day-ghim"]');
        if (!band) return null;
        const r = band.getBoundingClientRect();
        const x = r.left + r.width / 2;
        const y = r.top + r.height / 2;
        const e = document.elementFromPoint(x, y);
        return { x, y, trungDai: band.contains(e), ten: e?.closest('[role="button"]')?.getAttribute("aria-label") ?? (e?.innerText ?? "").slice(0, 40) };
      });
      if (ten === "L17") await chup(t.page, { out: mt.out, id: "EV-F05-GHIM-TREN-NEN-C1", suKien: t.suKien, chuThich: [{ selector: '[data-testid="day-ghim"]', nhan: "dải ghim trên nền mờ" }] });
      if (diem) await cham(cdp, diem);
      await t.page.waitForTimeout(1800);
      kq[ten] = { truoc, diem, sau: await demDialog(t.page), khayToHen: !!(await timNut(t.page, "Đóng tờ hẹn chung")), duong: await duongDan(t.page) };
      await t.context.close();
    }
    log({ kq });
    const f = (x) => `trước ${x.truoc} hộp thoại; điểm giữa dải trúng ${x.diem?.trungDai ? `dải («${x.diem.ten}»)` : `«${x.diem?.ten}»`}; sau khi chạm ${x.sau} hộp thoại, khay tờ hẹn ${x.khayToHen ? "mở" : "không"}`;
    ket({ tc: "TC-F05-GHIM-TREN-NEN", screen: "F05.S02", state: "nhóm có tờ hẹn chung (dải ghim); một sheet đang mở", action: "chạm vào dải ghim phía trên sheet (L17 Cài đặt nhóm, L18 menu tin)", cauHinh: "C1", expected: "nền mờ phủ cả dải; chạm ngoài sheet thì đóng sheet, không mở thứ khác bên dưới", status: Object.values(kq).every((x) => x.diem && !x.diem.trungDai) ? "PASS" : "FAIL", evidence: ["EV-F05-GHIM-TREN-NEN-C1"], ghiChu: Object.entries(kq).map(([k, v]) => `${k}: ${f(v)}`).join("; ") });
  }

  // ------------------------------------ the notice card «Đã hiểu» (L22)
  // A failed reaction is one of the paths that fills this card (with the
  // failed image send, a refused delete and the send's intent sentence).
  if (chay("thong-bao")) {
    const chu = `Tin để thả cảm xúc ${Date.now() % 100000}`;
    await goiApi(mt.api, "POST", `/contexts/${G20}/messages`, { kind: "text", body: chu, image_url: null, card: null }, phien1.token);
    const t = await moQuaTin("C1", P0, chatCua(G20));
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(1200);
    await loiMayChu(t.page, /\/reactions(\/|\?|$)/);
    const doThe = () => t.page.evaluate(() => {
      const nut = [...document.querySelectorAll('[role="button"]')].find((e) => (e.innerText ?? "").trim() === "Đã hiểu" && e.getBoundingClientRect().height > 0);
      if (!nut) return null;
      const the = nut.parentElement;
      const r = the.getBoundingClientRect();
      const n = nut.getBoundingClientRect();
      const list = document.querySelector('[data-testid="chat-list"]')?.getBoundingClientRect();
      return { cau: (the.innerText ?? "").replace("Đã hiểu", "").replace(/\s+/g, " ").trim().slice(0, 140), top: Math.round(r.top), bottom: Math.round(r.bottom), trongKhung: !!list && r.bottom > list.top && r.top < list.bottom, nut: `${Math.round(n.width)}×${Math.round(n.height)}` };
    });
    // At the newest end.
    await cham(cdp, await bongCua(t.page, chu));
    await t.page.waitForTimeout(900);
    await bam(t, cdp, "Thích", { cho: 1800 });
    const cuoi = await doThe();
    await chup(t.page, { out: mt.out, id: "EV-F05-THONG-BAO-C1", suKien: t.suKien, chuThich: cuoi ? [{ rect: { x: 16, y: cuoi.top, w: 358, h: cuoi.bottom - cuoi.top }, nhan: "thẻ thông báo" }] : [] });
    if (cuoi) await bam(t, cdp, "Đã hiểu", { cho: 700 });
    const heT = (await doThe()) === null;
    log({ cuoi, heT });
    ket({ tc: "TC-L22-VONGDOI", screen: "F05.S02", layer: "L22", state: "đang ở cuối chat; máy chủ trả 503 cho cảm xúc", action: "menu của tin → «Thích»; rồi «Đã hiểu»", cauHinh: "C1", expected: "thẻ nói vì sao, nằm trong vùng đang xem; «Đã hiểu» ≥ 48dp và gỡ thẻ", status: cuoi && cuoi.trongKhung && heT ? "PASS" : "FAIL", evidence: ["EV-F05-THONG-BAO-C1"], ghiChu: cuoi ? `thẻ y ${cuoi.top}–${cuoi.bottom}, ${cuoi.trongKhung ? "trong" : "ngoài"} vùng tin; «${cuoi.cau}»; nút «Đã hiểu» ${cuoi.nut}; sau khi chạm thẻ ${heT ? "hết" : "còn"}` : "không có thẻ thông báo" });
    // While reading older messages.
    await cuonLichSu(t.page, 1400);
    await t.page.waitForTimeout(1200);
    const cu = await t.page.evaluate(() => {
      const ds = [...document.querySelectorAll('[role="button"][aria-label^="Tin nhắn: "]')].filter((e) => { const r = e.getBoundingClientRect(); return r.top > 220 && r.bottom < 640; });
      const e = ds[0];
      if (!e) return null;
      const r = e.getBoundingClientRect();
      return { x: r.left + r.width / 2, y: r.top + r.height / 2, ten: e.getAttribute("aria-label").slice(10, 50) };
    });
    if (cu) {
      await cham(cdp, cu);
      await t.page.waitForTimeout(900);
      await bam(t, cdp, "Thích", { cho: 1800 });
    }
    const tren = await doThe();
    const pill = await t.page.evaluate(() => !!document.querySelector('[aria-label="Về tin nhắn mới nhất"]'));
    await chup(t.page, { out: mt.out, id: "EV-F05-THONG-BAO-CU-C1", suKien: t.suKien });
    await goHet(t.page);
    log({ cu, tren, pill });
    ket({ tc: "TC-L22-KHI-DOC-CU", screen: "F05.S02", layer: "L22", state: "đang đọc tin cũ (cuộn lên 1400px); máy chủ trả 503 cho cảm xúc", action: "menu của một tin cũ → «Thích»", cauHinh: "C1", expected: "người vừa chạm biết là không được: câu hiện trong vùng đang xem, hoặc có lối tới nó", status: cu && tren && tren.trongKhung ? "PASS" : "FAIL", evidence: ["EV-F05-THONG-BAO-CU-C1"], ghiChu: cu ? `chạm tin «${cu.ten}»; thẻ ${tren ? `ở y ${tren.top}–${tren.bottom}, ${tren.trongKhung ? "trong" : "ngoài"} vùng tin` : "không có trong DOM"}; nút «Tin mới nhất» ${pill ? "có" : "không"}` : "không thấy tin cũ để chạm" });
    await t.context.close();
  }

  // ------------------------------------------ the pinned band over the thread
  if (chay("ghim")) {
    for (const cfg of ["C1", "C2"]) {
      const t = await mo(cfg, P8, chatCua(G8));
      await t.page.waitForTimeout(900);
      const r = await t.page.evaluate(() => {
        const band = document.querySelector('[data-testid="day-ghim"]')?.getBoundingClientRect();
        const list = document.querySelector('[data-testid="chat-list"]')?.getBoundingClientRect();
        return band && list ? { bandDuoi: Math.round(band.bottom), listTren: Math.round(list.top) } : null;
      });
      ket({ tc: "TC-F05-GHIM", screen: "F05.S02", state: "chat có bình chọn đang mở (dải ghim)", action: "mở chat Team Đà Lạt", cauHinh: cfg, expected: "danh sách tin bắt đầu dưới dải ghim; dải không đè lên tin (tin cuộn khuất dưới mép danh sách là bình thường)", status: r && r.listTren >= r.bandDuoi - 1 ? "PASS" : "FAIL", evidence: [`EV-F05.S02-BASE-${cfg}`], ghiChu: r ? `dải ghim tới y ${r.bandDuoi}, danh sách bắt đầu y ${r.listTren}` : "không có dải ghim" });
      await t.context.close();
    }
  }

  // --------------------------------------------------- cold open, in-app back
  if (chay("lanh")) {
    const t = await mo("C1", P8, chatCua(G8));
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(800);
    const truoc = await duongDan(t.page);
    await bam(t, cdp, "Quay lại", { cho: 1200 });
    const sau = await duongDan(t.page);
    ket({ tc: "TC-F05.S02-BACK-LANH", screen: "F05.S02", state: "mở thẳng bằng link", action: "chạm «Quay lại» của đầu chat", cauHinh: "C1", expected: "về Tin nhắn", status: sau !== truoc ? "PASS" : "FAIL", issue: sau !== truoc ? null : "UI-018", evidence: [], ghiChu: `${truoc.replace(/[0-9a-f-]{36}/, "[id]")} → ${sau.replace(/[0-9a-f-]{36}/, "[id]")}` });
    await t.context.close();
  }

  // --------------------------------------------------------- failure, offline
  if (chay("loi")) {
    const t = await mo("C1", P8, "/messages");
    await loiMayChu(t.page, /\/contexts\/[^/]+\/messages(\?|$)/);
    await t.page.evaluate((p) => { history.pushState({}, "", p); dispatchEvent(new PopStateEvent("popstate")); }, chatCua(G8));
    await t.page.waitForTimeout(2500);
    const CAU = /Rủ Đi đang gặp sự cố[^\n]*|Kiểm tra mạng[^\n]*|Chưa tải[^\n]*/;
    const luc1 = { cau: await viTriCau(t.page, CAU), soTin: await t.page.evaluate(() => document.querySelectorAll('[role="button"][aria-label^="Tin nhắn: "]').length), thuLai: !!(await timNut(t.page, "Thử lại")) };
    await chup(t.page, { out: mt.out, id: "EV-F05-503-C1", suKien: t.suKien, chuThich: luc1.cau ? [{ rect: { x: 16, y: luc1.cau.top, w: 358, h: luc1.cau.bottom - luc1.cau.top }, nhan: "câu lỗi" }] : [] });
    await goHet(t.page);
    await t.page.waitForTimeout(15_000);
    const luc2 = { cau: await viTriCau(t.page, CAU) };
    log({ luc1, luc2 });
    ket({ tc: "TC-F05.S02-503", screen: "F05.S02", state: "GET danh sách tin trả 503 (luồng thay đổi vẫn chạy)", action: "mở chat từ Tin nhắn; 15 s sau máy chủ ổn lại", cauHinh: "C1", expected: "nếu tin vẫn hiện từ luồng thay đổi thì không báo lỗi chung; nếu báo thì nói phần nào hỏng, có cách thử lại, và tự tắt khi máy chủ ổn", status: !luc1.cau || (luc1.thuLai && !luc2.cau) ? "PASS" : "FAIL", evidence: ["EV-F05-503-C1"], ghiChu: `lúc hỏng: ${luc1.soTin} tin vẫn hiện; câu «${luc1.cau?.text ?? "không"}» ở y ${luc1.cau?.top}; Thử lại ${luc1.thuLai ? "có" : "không"}. 15 s sau khi máy chủ ổn: câu ${luc2.cau ? "vẫn còn" : "đã tắt"}` });
    await t.context.close();
  }

  // ------------------------------------------------------------- low window C8
  if (chay("c8")) {
    const t = await mo("C8", P8, chatCua(G8));
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(900);
    const vung = async () => t.page.evaluate(() => {
      const list = document.querySelector('[data-testid="chat-list"]')?.getBoundingClientRect();
      const o = document.querySelector('textarea[aria-label="Ô soạn tin"]')?.getBoundingClientRect();
      return { listCao: list ? Math.round(list.height) : null, oDuoi: o ? Math.round(o.bottom) : null, cao: innerHeight };
    });
    const v0 = await vung();
    await chup(t.page, { out: mt.out, id: "EV-F05-C8-C8", suKien: t.suKien });
    await bam(t, cdp, "Thêm vào cuộc trò chuyện", { cho: 800 });
    const v1 = await vung();
    await chup(t.page, { out: mt.out, id: "EV-F05-C8-khay-C8", suKien: t.suKien });
    log({ v0, v1 });
    ket({ tc: "TC-F05-C8", screen: "F05.S02", state: "cửa sổ 390×460 (proxy bàn phím)", action: "mở chat; mở khay công cụ", cauHinh: "C8", expected: "ô soạn thấy; vùng tin còn đọc được (≥ 1/3 cửa sổ); khay không đẩy ô soạn ra ngoài", status: v0.oDuoi <= v0.cao && v0.listCao >= v0.cao / 3 && v1.oDuoi <= v1.cao ? "PASS" : "FAIL", evidence: ["EV-F05-C8-C8", "EV-F05-C8-khay-C8"], ghiChu: `vùng tin cao ${v0.listCao}/${v0.cao}; ô soạn đáy y ${v0.oDuoi}; khi mở khay: vùng tin ${v1.listCao}, ô soạn đáy y ${v1.oDuoi}` });
    await t.context.close();
  }

  // -------------------------------------------------------- two-person thread
  if (chay("dm") && DM) {
    const t = await mo("C1", P0, chatCua(DM));
    await t.page.waitForTimeout(900);
    const r = await t.page.evaluate(() => ({
      hoSo: !!document.querySelector('[aria-label="Xem hồ sơ"]'),
      e2ee: /Chưa mã hoá đầu cuối/.test(document.body.innerText),
      goiY: document.querySelector('textarea[aria-label="Ô soạn tin"]')?.getAttribute("placeholder"),
      rong: /Một lời mở đầu/.test(document.body.innerText),
    }));
    await chup(t.page, { out: mt.out, id: "EV-F05-DM-C1", suKien: t.suKien });
    ket({ tc: "TC-F05-DM", screen: "F05.S03", state: "chat hai người chưa có tin", action: "mở", cauHinh: "C1", expected: "đầu chat mở hồ sơ người kia; nhãn «Chưa mã hoá đầu cuối»; ô soạn gọi tên người kia; trạng thái trống có lời mở đầu", status: r.hoSo && r.e2ee && /Chat Test 02/.test(r.goiY ?? "") ? "PASS" : "FAIL", evidence: ["EV-F05-DM-C1"], ghiChu: `«Xem hồ sơ» ${r.hoSo ? "có" : "không"}; nhãn mã hoá ${r.e2ee ? "có" : "không"}; placeholder «${r.goiY}»; trống ${r.rong ? "«Một lời mở đầu.»" : "không"}` });
    await t.context.close();
  }

  // ------------------------------------------------ /votes/[id] with a session
  if (chay("votes")) {
    const t = await mo("C1", P8, "/votes/binh-chon-mau");
    await t.page.waitForTimeout(1200);
    const d = await duongDan(t.page);
    ket({ tc: "TC-F05.S04-CO-PHIEN", screen: "F05.S04", state: "đã đăng nhập", action: "mở /votes/[id]", cauHinh: "C1", expected: "chuyển về /messages (route demo)", status: /^\/messages/.test(d) ? "PASS" : "FAIL", evidence: [], ghiChu: `đường cuối ${d}` });
    await t.context.close();
  }
} finally {
  await mt.dong();
}
