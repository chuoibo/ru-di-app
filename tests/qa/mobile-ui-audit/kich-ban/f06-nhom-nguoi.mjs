/* F06, Groups and people: open a group, its roster, inviting by number, the
 * friends list and adding a friend, somebody's profile and its actions sheet.
 *
 *   node kich-ban/f06-nhom-nguoi.mjs [--chi tao-nhom,moi,loi-moi,moi-lai,duoc-moi,thanh-vien,tu-bo-quan-tri,ban-be,them-ban,them-ban-app,ho-so,chan,chan-chat,vung-bam,lanh,loi,c8,tablet]
 *
 * Who writes what, all on the local stack:
 *  - moi-51, a new account, opens «Nhóm kiểm thử F06 …» and invites the
 *    number of moi-52, which then signs in and accepts;
 *  - chat-20 and chat-21 (the two chat-seed people with no friend and no
 *    group) become friends through the screens, open their pair, and chat-20
 *    blocks chat-21 (the blocked list and unblocking are F09.S04);
 *  - chat-0 sends a friend request to chat-5 from a profile.
 * Team Đà Lạt and the chat-test group are only read.
 */
import { join } from "node:path";

import { chayAxe } from "../thu-vien/axe.mjs";
import { cauHinh } from "../thu-vien/cau-hinh.mjs";
import { chup } from "../thu-vien/chup.mjs";
import { cdpCua, cham } from "../thu-vien/cu-chi.mjs";
import { choOn, duongDan } from "../thu-vien/dieu-huong.mjs";
import { demDialog, dong, focusHienTai } from "../thu-vien/lop-phu.mjs";
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
  so.ghi({ feature: "F06", nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, ...rec });
  console.log(`${rec.status.padEnd(10)} ${rec.tc} ${rec.cauHinh ?? ""} ${rec.ghiChu ?? ""}`);
};
const log = (o) => console.log(JSON.stringify(o));
const phienDir = join(mt.out, "phien");

const P = (ten) => personaTheoTen(ten, mt.chatSessions);
const phienCua = (ten) => layPhien(mt.api, P(ten), phienDir);
const { nhomId: G8 } = await moNhom(mt);
const api = (method, path, body, phien) => goiApi(mt.api, method, path, body, phien.token);
const ngCanh = async (phien) => (await api("GET", "/people/me/contexts", undefined, phien)).contexts ?? [];
const an = (s) => String(s ?? "").replace(/[0-9a-f]{8}-[0-9a-f-]{27}/g, "[id]");

const mo = (cfg, persona, path, them = {}) => trangMoi(mt, cauHinh(cfg), { persona, path, ...them });
/** Open `goc`, then walk into `path` inside the app, so Back has an app screen to return to. */
const moQua = async (cfg, persona, path, goc = "/messages") => {
  const t = await mo(cfg, persona, goc);
  await diToi(t, path);
  return t;
};
const diToi = async (t, path) => {
  await t.page.evaluate((p) => {
    history.pushState({}, "", p);
    dispatchEvent(new PopStateEvent("popstate"));
  }, path);
  await choOn(t.page, { mang: t.mang });
  await t.page.waitForTimeout(900);
};

/** Put back sideways scrolls of rows no finger can scroll (see thu-vien/cu-chi.mjs `tamCua`). */
const NGANG = "for (let n = e.parentElement; n; n = n.parentElement) if (n.scrollLeft && !/(auto|scroll)/.test(getComputedStyle(n).overflowX)) n.scrollLeft = 0;";
/** A control by its whole accessible name or visible text, scrolled into view vertically. */
const timNut = (page, chu, { vai = '[role="button"],button,[role="link"],a[href],[role="tab"],[role="radio"],[role="checkbox"]', batDau = false } = {}) =>
  page.evaluate(
    ({ chu, vai, batDau, NGANG }) => {
      // Icon glyphs render as private-use characters inside the button text.
      const ten = (x) => (x.getAttribute("aria-label") || (x.innerText ?? "")).replace(/[\uE000-\uF8FF]/g, "").replace(/\s+/g, " ").trim();
      const e = [...document.querySelectorAll(vai)].find((x) => (batDau ? ten(x).startsWith(chu) : ten(x) === chu) && x.getBoundingClientRect().width > 0);
      if (!e) return null;
      e.scrollIntoView({ block: "center", behavior: "instant" });
      new Function("e", NGANG)(e);
      const r = e.getBoundingClientRect();
      return { x: r.left + r.width / 2, y: r.top + r.height / 2, w: Math.round(r.width), h: Math.round(r.height), top: Math.round(r.top), ten: ten(e) };
    },
    { chu, vai, batDau, NGANG },
  );
const bam = async (t, cdp, chu, { cho = 800, ...o } = {}) => {
  const n = await timNut(t.page, chu, o);
  if (!n) return null;
  await cham(cdp, n);
  await t.page.waitForTimeout(cho);
  return n;
};
const go = async (page, nhan, chu) => {
  const o = page.locator(`[aria-label="${nhan}"]`).first();
  await o.click();
  await o.fill(chu);
};
/**
 * Where a sentence sits, and whether it is on screen. Only a rendered element
 * counts: the stack keeps earlier screens mounted (hidden), and their text
 * matched here once instead of the sentence on screen (TC-F06-MOI-LAI).
 */
const viTriCau = (page, re) =>
  page.evaluate((s) => {
    const R = new RegExp(s);
    const hien = (x) => x.getClientRects().length > 0 && (x.checkVisibility ? x.checkVisibility({ visibilityProperty: true, opacityProperty: true }) : true);
    const e = [...document.querySelectorAll("body *")].find((x) => [...x.childNodes].some((n) => n.nodeType === 3 && R.test(n.textContent)) && hien(x));
    if (!e) return null;
    const r = e.getBoundingClientRect();
    return { top: Math.round(r.top), bottom: Math.round(r.bottom), trongMan: r.bottom > 0 && r.top < innerHeight, text: e.textContent.trim().slice(0, 160) };
  }, re.source);
const coChu = (page, re) => page.evaluate((s) => new RegExp(s).test(document.body.innerText ?? ""), re.source);
const chuTrang = (page) => page.evaluate(() => (document.body.innerText ?? "").replace(/\s+/g, " ").slice(0, 400));

try {
  // ------------------------------------------------------------ open a group
  if (chay("tao-nhom")) {
    const p51 = await phienCua("moi-51");
    const truoc = (await ngCanh(p51)).filter((c) => c.kind === "group");
    const t = await moQua("C1", P("moi-51"), "/groups/new");
    const cdp = await cdpCua(t.page);
    // Empty name.
    await bam(t, cdp, "Mở nhóm", { cho: 700 });
    const trong = await viTriCau(t.page, /Đặt tên cho nhóm\./);
    const focusTrong = await focusHienTai(t.page);
    ket({ tc: "TC-F06-TAO-TRONG", screen: "F06.S01", state: "tên trống", action: "chạm «Mở nhóm»", cauHinh: "C1", expected: "một câu nói thiếu gì, trong tầm nhìn, cạnh ô; không tạo nhóm", status: trong?.trongMan ? "PASS" : "FAIL", evidence: [], ghiChu: `câu «${trong?.text ?? "không có"}» ở y ${trong?.top}; focus ${focusTrong?.ten ?? focusTrong?.role ?? "không rõ"}` });
    // Long name, then a double tap.
    const TEN = `Nhóm kiểm thử F06 hội săn mây cuối tuần uống cà phê sớm ${Date.now() % 1000} 🌄☕`;
    await go(t.page, "Ô tên nhóm", TEN);
    const o = await t.page.evaluate(() => { const e = document.querySelector('[aria-label="Ô tên nhóm"]'); const r = e.getBoundingClientRect(); return { w: Math.round(r.width), h: Math.round(r.height), tran: e.scrollWidth > e.clientWidth + 1 }; });
    const n = await timNut(t.page, "Mở nhóm");
    await cham(cdp, n);
    await t.page.waitForTimeout(60);
    await cham(cdp, n);
    await t.page.waitForTimeout(3500);
    const sau = (await ngCanh(p51)).filter((c) => c.kind === "group");
    const moi = sau.filter((c) => !truoc.some((x) => x.id === c.id));
    const duong = await duongDan(t.page);
    await chup(t.page, { out: mt.out, id: "EV-F06-TAO-SAU-C1", suKien: t.suKien });
    const chu = await chuTrang(t.page);
    log({ o, truoc: truoc.length, sau: sau.length, moi: moi.map((c) => c.display_name), duong, chu: chu.slice(0, 200) });
    ket({ tc: "TC-F06-TAO-CHAM-DUP", screen: "F06.S01", state: `tên ${TEN.length} ký tự có emoji`, action: "chạm «Mở nhóm» hai lần cách 60 ms", cauHinh: "C1", expected: "đúng một nhóm được mở", status: moi.length === 1 ? "PASS" : "FAIL", evidence: [], ghiChu: `nhóm của moi-51: ${truoc.length} → ${sau.length}; ô tên ${o.w}×${o.h}, chữ ${o.tran ? "cuộn ngang trong ô" : "vừa ô"}` });
    ket({ tc: "TC-F06-TAO-SAU", screen: "F06.S01", state: "vừa mở nhóm (người chưa có nhóm nào)", action: "sau «Mở nhóm»", cauHinh: "C1", expected: "tới nơi thấy nhóm vừa mở (chat hoặc mời), hoặc có một câu xác nhận kèm lối mời bạn; câu trên màn hứa «Mời bạn bè sau»", status: /\/groups\//.test(duong) || new RegExp(moi[0]?.display_name?.slice(0, 20) ?? "#không#").test(chu) ? "PASS" : "FAIL", evidence: ["EV-F06-TAO-SAU-C1"], ghiChu: `tới ${an(duong)}; tên nhóm mới ${moi[0] && chu.includes(moi[0].display_name.slice(0, 20)) ? "có" : "không có"} trên màn` });
    await t.context.close();
    // A server failure while opening, then the retry of the same name.
    const t2 = await moQua("C1", P("moi-51"), "/groups/new");
    const cdp2 = await cdpCua(t2.page);
    const TEN2 = `Nhóm kiểm thử F06 lỗi mạng ${Date.now() % 1000}`;
    await go(t2.page, "Ô tên nhóm", TEN2);
    await loiMayChu(t2.page, /\/contexts(\?|$)/);
    const n0 = (await ngCanh(p51)).filter((c) => c.kind === "group").length;
    await bam(t2, cdp2, "Mở nhóm", { cho: 1500 });
    const cau = await viTriCau(t2.page, /sự cố|Kiểm tra mạng|thử lại/);
    await chup(t2.page, { out: mt.out, id: "EV-F06-TAO-503-C1", suKien: t2.suKien });
    await goHet(t2.page);
    await bam(t2, cdp2, "Mở nhóm", { cho: 3500 });
    const n1 = (await ngCanh(p51)).filter((c) => c.kind === "group").length;
    ket({ tc: "TC-F06-TAO-503", screen: "F06.S01", state: "POST /contexts trả 503", action: "«Mở nhóm», rồi máy chủ ổn và chạm lại", cauHinh: "C1", expected: "câu nói đúng lỗi máy chủ, trong tầm nhìn, tên còn nguyên; chạm lại mở đúng một nhóm", status: cau?.trongMan && n1 - n0 === 1 ? "PASS" : "FAIL", evidence: ["EV-F06-TAO-503-C1"], ghiChu: `câu «${cau?.text ?? "không có"}» ở y ${cau?.top}; nhóm ${n0} → ${n1} sau lần chạm lại` });
    await t2.context.close();
  }

  // ------------------------------------------------- invite by phone number
  if (chay("moi")) {
    const p51 = await phienCua("moi-51");
    const nhom = (await ngCanh(p51)).find((c) => c.kind === "group" && /^Nhóm kiểm thử F06 hội săn mây/.test(c.display_name ?? ""));
    if (!nhom) throw new Error("chưa có nhóm F06 của moi-51; chạy --chi tao-nhom trước");
    const t = await moQua("C1", P("moi-51"), `/groups/${nhom.id}/members`);
    const cdp = await cdpCua(t.page);
    await bam(t, cdp, "Mời bằng số điện thoại", { cho: 1000 });
    const vao = await duongDan(t.page);
    // Nothing typed, then a number and no name.
    await bam(t, cdp, "Gửi lời mời", { cho: 600 });
    const c1 = await viTriCau(t.page, /Chưa đúng dạng số/);
    await go(t.page, "Ô số điện thoại người được mời", P("moi-52").phone);
    await bam(t, cdp, "Gửi lời mời", { cho: 600 });
    const c2 = await viTriCau(t.page, /Đặt tên cho người bạn đang mời/);
    ket({ tc: "TC-F06-MOI-THIEU", screen: "F06.S03", state: "form mời trống, rồi chỉ có số", action: "«Gửi lời mời» hai lần", cauHinh: "C1", expected: "mỗi lần một câu nói thiếu gì, trong tầm nhìn", status: c1?.trongMan && c2?.trongMan ? "PASS" : "FAIL", evidence: [], ghiChu: `vào ${an(vao)}; trống: «${c1?.text ?? "không"}» y ${c1?.top}; thiếu tên: «${c2?.text ?? "không"}» y ${c2?.top}` });
    // A real invitation, with a long name.
    const TEN = "Bạn thân từ hồi cấp ba của mình, người hay trễ hẹn nhất hội";
    await go(t.page, "Ô tên người được mời", TEN);
    await bam(t, cdp, "Gửi lời mời", { cho: 3000 });
    const xong = await t.page.evaluate(() => ({ chu: (document.body.innerText ?? "").replace(/\s+/g, " ").slice(0, 300) }));
    await chup(t.page, { out: mt.out, id: "EV-F06-MOI-XONG-C1", suKien: t.suKien });
    const tv = await goiApi(mt.api, "GET", `/contexts/${nhom.id}/members`, undefined, p51.token).catch(() => null);
    const duocMoi = (tv?.members ?? tv ?? []).filter?.((m) => m.state === "invited").length ?? null;
    ket({ tc: "TC-F06-MOI-GUI", screen: "F06.S03", state: "tên 59 ký tự, số của một người chưa dùng Rủ Đi", action: "«Gửi lời mời»", cauHinh: "C1", expected: "phong bì «Đã gửi», nói người kia thấy lời mời ở đâu; một thành viên «invited»", status: /Đã mời/.test(xong.chu) && duocMoi === 1 ? "PASS" : "FAIL", evidence: ["EV-F06-MOI-XONG-C1"], ghiChu: `màn: «${xong.chu.slice(0, 160)}»; thành viên đang được mời: ${duocMoi}` });
    // The same number again.
    await bam(t, cdp, "Mời thêm người", { cho: 800 });
    const sach = await t.page.evaluate(() => [...document.querySelectorAll("input")].map((i) => i.value).join("|"));
    await go(t.page, "Ô tên người được mời", "Mời lại lần hai");
    await go(t.page, "Ô số điện thoại người được mời", P("moi-52").phone);
    await bam(t, cdp, "Gửi lời mời", { cho: 2500 });
    const lai = { cau: await viTriCau(t.page, /đã|Đã|mời|thử lại|sự cố/), chu: await chuTrang(t.page) };
    await chup(t.page, { out: mt.out, id: "EV-F06-MOI-LAI-C1", suKien: t.suKien });
    log({ sach, lai });
    ket({ tc: "TC-F06-MOI-LAI", screen: "F06.S03", state: "số này đã được mời vào nhóm", action: "«Mời thêm người», mời lại đúng số đó", cauHinh: "C1", expected: "form trống lại; mời lại thì một câu nói số này đã được mời (không phải lỗi mạng hay lỗi chung)", status: sach === "|" && /đã (được )?mời|đang chờ|đã ở trong/i.test(lai.chu) && !/Kiểm tra mạng|sự cố/.test(lai.chu) ? "PASS" : "FAIL", evidence: ["EV-F06-MOI-LAI-C1"], ghiChu: `form sau «Mời thêm người»: ${sach === "|" ? "trống" : `còn «${sach}»`}; câu: «${lai.cau?.text ?? "không"}»` });
    // Back to the roster.
    await t.page.goBack();
    await t.page.waitForTimeout(1500);
    const ds = await t.page.evaluate(() => (document.body.innerText ?? "").replace(/\s+/g, " "));
    ket({ tc: "TC-F06-MOI-DANH-SACH", screen: "F06.S02", state: "vừa mời một người", action: "Back về Thành viên", cauHinh: "C1", expected: "người vừa mời hiện «Đã mời, chưa đồng ý»; đếm «1 đang được mời»", status: /Đã mời, chưa đồng ý/.test(ds) && /1 đang được mời/.test(ds) ? "PASS" : "FAIL", evidence: [], ghiChu: `«Đã mời, chưa đồng ý» ${/Đã mời, chưa đồng ý/.test(ds) ? "có" : "không"}; tiêu đề phụ: «${(ds.match(/\d+ đang ở trong nhóm, \d+ đang được mời\./) ?? ["không"])[0]}»` });
    await t.context.close();
  }

  // -------------------------------------------- the invitation, on its side
  if (chay("loi-moi")) {
    const p52 = await phienCua("moi-52");
    const tr = await ngCanh(p52);
    const t = await mo("C1", P("moi-52"), "/messages");
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(1200);
    const man = await chuTrang(t.page);
    await chup(t.page, { out: mt.out, id: "EV-F06-LOI-MOI-DEN-C1", suKien: t.suKien });
    const dy = await timNut(t.page, "Đồng ý", { batDau: true });
    ket({ tc: "TC-F05.S01-LOI-MOI", screen: "F05.S01", state: "người được mời vừa đăng nhập bằng số đó", action: "mở Tin nhắn", cauHinh: "C1", expected: "lời mời hiện, nói nhóm nào và ai mời, có «Đồng ý»", status: dy ? "PASS" : "FAIL", evidence: ["EV-F06-LOI-MOI-DEN-C1"], ghiChu: `trước: ${tr.map((c) => `${c.kind}:${c.membership_state ?? "?"}`).join(", ") || "không ngữ cảnh"}; màn: «${man.slice(0, 200)}»; nút «${dy?.ten ?? "không có"}» ${dy ? `${dy.w}×${dy.h}` : ""}` });
    if (dy) {
      await cham(cdp, dy);
      await t.page.waitForTimeout(3000);
    }
    const sau = await ngCanh(p52);
    const duong = await duongDan(t.page);
    await chup(t.page, { out: mt.out, id: "EV-F06-LOI-MOI-DONG-Y-C1", suKien: t.suKien });
    const ten = await api("GET", "/people/me", undefined, p52).catch(() => null);
    ket({ tc: "TC-F05.S01-DONG-Y", screen: "F05.S01", state: "lời mời đang chờ", action: "«Đồng ý»", cauHinh: "C1", expected: "vào nhóm (thành viên active), thấy nhóm hoặc chat của nó", status: sau.some((c) => c.kind === "group" && (c.membership_state ?? "active") === "active") ? "PASS" : "FAIL", evidence: ["EV-F06-LOI-MOI-DONG-Y-C1"], ghiChu: `sau: ${sau.map((c) => `${c.kind}:${c.membership_state ?? "?"}`).join(", ")}; tới ${an(duong)}; tên của người mới trên máy chủ «${ten?.display_name ?? "?"}» (tên do người mời đặt)` });
    await t.context.close();
  }

  // --------------------------- inviting a number that is already invited
  // Two cases the first run mixed up: the number of moi-53 invited twice
  // (still pending), and the number of moi-52 (already an active member). The
  // HTTP status of the membership write is recorded next to the sentence.
  if (chay("moi-lai")) {
    const p51 = await phienCua("moi-51");
    const nhom = (await ngCanh(p51)).find((c) => c.kind === "group" && /^Nhóm kiểm thử F06 hội săn mây/.test(c.display_name ?? ""));
    if (!nhom) throw new Error("chưa có nhóm F06 của moi-51; chạy --chi tao-nhom trước");
    const t = await moQua("C1", P("moi-51"), `/groups/${nhom.id}/invite`, `/groups/${nhom.id}/members`);
    const cdp = await cdpCua(t.page);
    const maGhi = [];
    t.page.on("response", (r) => { if (r.request().method() === "POST" && /\/contexts\/[^/]+\/members(\?|$)/.test(r.url())) maGhi.push(r.status()); });
    const moiSo = async (ten, so) => {
      await go(t.page, "Ô tên người được mời", ten);
      await go(t.page, "Ô số điện thoại người được mời", so);
      maGhi.length = 0;
      await bam(t, cdp, "Gửi lời mời", { cho: 3000 });
      return { ma: [...maGhi], chu: await chuTrang(t.page), cau: (await viTriCau(t.page, /Lần bấm trước|đã được mời|đã mời|Đã mời|đã ở trong|sự cố|Kiểm tra mạng/)) };
    };
    // Pending: moi-53, twice.
    const lan1 = await moiSo("Tên do người mời đặt cho số 53", P("moi-53").phone);
    await bam(t, cdp, "Mời thêm người", { cho: 800 });
    const lan2 = await moiSo("Tên do người mời đặt cho số 53", P("moi-53").phone);
    await chup(t.page, { out: mt.out, id: "EV-F06-MOI-LAI-CHO-C1", suKien: t.suKien });
    log({ lan1: { ma: lan1.ma, cau: lan1.cau?.text }, lan2: { ma: lan2.ma, cau: lan2.cau } });
    const dungLyDo = (x) => x.cau?.trongMan && /đã được mời|đã mời|đang chờ|đã ở trong/i.test(x.cau.text) && !/Lần bấm trước|sự cố|Kiểm tra mạng/.test(x.cau.text);
    ket({ tc: "TC-F06-MOI-LAI", screen: "F06.S03", state: "số này đã được mời, chưa đồng ý", action: "«Mời thêm người», mời lại đúng số đó", cauHinh: "C1", expected: "một câu nói số này đã được mời và đang chờ (không phải câu về lần bấm trước, lỗi mạng hay lỗi chung)", status: dungLyDo(lan2) ? "PASS" : "FAIL", evidence: ["EV-F06-MOI-LAI-CHO-C1"], ghiChu: `lần 1: HTTP ${lan1.ma.join(",") || "không gọi"}, «${(lan1.chu.match(/Đã mời[^.]*/) ?? ["?"])[0].slice(0, 60)}»; lần 2: HTTP ${lan2.ma.join(",") || "không gọi"}, câu «${lan2.cau?.text ?? "không có"}» ở y ${lan2.cau?.top}` });
    // Already a member: moi-52.
    if (await timNut(t.page, "Mời thêm người")) await bam(t, cdp, "Mời thêm người", { cho: 800 });
    const tv = await moiSo("Mời người đã ở trong nhóm", P("moi-52").phone);
    await chup(t.page, { out: mt.out, id: "EV-F06-MOI-THANH-VIEN-C1", suKien: t.suKien });
    log({ tv: { ma: tv.ma, cau: tv.cau } });
    ket({ tc: "TC-F06-MOI-THANH-VIEN", screen: "F06.S03", state: "số của một người đã ở trong nhóm", action: "mời số đó", cauHinh: "C1", expected: "một câu nói người này đã ở trong nhóm", status: dungLyDo(tv) ? "PASS" : "FAIL", evidence: ["EV-F06-MOI-THANH-VIEN-C1"], ghiChu: `HTTP ${tv.ma.join(",") || "không gọi"}; câu «${tv.cau?.text ?? "không có"}» ở y ${tv.cau?.top}` });
    await t.context.close();
  }

  // ------------------- the invited person's first sign-in, through the door
  // moi-53 was invited by number (moi-lai) under a name the inviter chose,
  // before ever using the app. Measured: where the door lands, whether the
  // person sees (and can change) that name, then «Đồng ý» checked on the roster.
  if (chay("duoc-moi")) {
    const p51 = await phienCua("moi-51");
    const nhom = (await ngCanh(p51)).find((c) => c.kind === "group" && /^Nhóm kiểm thử F06 hội săn mây/.test(c.display_name ?? ""));
    const t = await trangMoi(mt, cauHinh("C1"), { path: "/login" });
    const cdp = await cdpCua(t.page);
    await t.page.locator('input[aria-label="Ô số điện thoại"]').fill(P("moi-53").phone);
    await bam(t, cdp, "Gửi mã", { batDau: true, cho: 200 });
    if (!/\/otp$/.test(await duongDan(t.page))) {
      const n = await t.page.evaluate(() => { const e = document.querySelector('[data-testid="login-gui-ma"]'); if (!e) return null; const r = e.getBoundingClientRect(); return { x: r.left + r.width / 2, y: r.top + r.height / 2 }; });
      if (n) await cham(cdp, n);
    }
    await t.page.waitForURL("**/otp", { timeout: 10_000 }).catch(() => undefined);
    await choOn(t.page, { mang: t.mang });
    await t.page.keyboard.type("000000", { delay: 40 });
    await t.page.waitForTimeout(4000);
    await choOn(t.page, { mang: t.mang });
    const den = await duongDan(t.page);
    const oTen = await t.page.evaluate(() => { const e = document.querySelector('input[aria-label="Ô tên của bạn"]'); return e ? e.value : null; });
    const chu = await chuTrang(t.page);
    await chup(t.page, { out: mt.out, id: "EV-F06-DUOC-MOI-VAO-CUA-C1", suKien: t.suKien });
    log({ den, oTen, chu: chu.slice(0, 240) });
    ket({ tc: "TC-F06-DUOC-MOI-VAO-CUA", screen: "F01.S03", state: "số này được mời vào nhóm dưới tên do người mời đặt, chưa từng đăng nhập", action: "đăng nhập bằng OTP qua UI", cauHinh: "C1", expected: "người mới thấy tên người khác đặt cho mình và đổi được trước khi cả nhóm thấy (ví dụ ô tên ở Sở thích điền sẵn)", status: oTen !== null && /Tên do người mời đặt/.test(oTen) ? "PASS" : "FAIL", evidence: ["EV-F06-DUOC-MOI-VAO-CUA-C1"], ghiChu: `tới ${an(den)}; ô «Ô tên của bạn» ${oTen === null ? "không có" : `«${oTen}»`}; tên do người mời đặt ${/Tên do người mời đặt/.test(chu) ? "có" : "không"} trên màn` });
    // Then the invitation, accepted.
    if (!/\/messages$/.test(den)) await diToi(t, "/messages");
    const dy = await timNut(t.page, "Đồng ý vào nhóm");
    if (dy) {
      await cham(cdp, dy);
      await t.page.waitForTimeout(3000);
    }
    const ds = nhom ? await goiApi(mt.api, "GET", `/contexts/${nhom.id}/members`, undefined, p51.token) : null;
    const p53 = await phienCua("moi-53");
    const tv53 = (ds?.members ?? ds ?? []).find?.((m) => m.person_id === p53.person_id);
    const conMoi = await coChu(t.page, /bạn được mời/);
    await chup(t.page, { out: mt.out, id: "EV-F06-DUOC-MOI-DONG-Y-C1", suKien: t.suKien });
    ket({ tc: "TC-F05.S01-DONG-Y", screen: "F05.S01", state: "lời mời đang chờ", action: "«Đồng ý vào nhóm»", cauHinh: "C1", expected: "thành viên chuyển sang active; hàng «bạn được mời» biến mất", status: tv53?.state === "active" && !conMoi ? "PASS" : "FAIL", evidence: ["EV-F06-DUOC-MOI-DONG-Y-C1"], ghiChu: `nút ${dy ? `${dy.w}×${dy.h}` : "không thấy"}; trạng thái trên máy chủ ${tv53?.state ?? "không thấy"}; tên trong nhóm «${tv53?.display_name ?? "?"}»; «bạn được mời» ${conMoi ? "còn" : "hết"}; ở ${an(await duongDan(t.page))}` });
    await t.context.close();
  }

  // ------------------------------------------------------------- the roster
  if (chay("thanh-vien")) {
    const p0 = await phienCua("chat-0");
    const G20 = (await ngCanh(p0)).find((c) => c.kind === "group")?.id;
    for (const cfg of ["C1", "C6"]) {
      const t = await moQua(cfg, P("chat-0"), `/groups/${G20}/members`);
      await t.page.waitForTimeout(800);
      const r = await t.page.evaluate(() => {
        const nut = [...document.querySelectorAll('[role="button"]')].filter((e) => /quản trị/.test(e.innerText ?? "") && e.getBoundingClientRect().height > 0);
        const ten = nut.map((e) => (e.getAttribute("aria-label") || e.innerText || "").trim());
        const hang = [...document.querySelectorAll("div")].filter((d) => /^(Thành viên|Người lập nhóm|Đã mời, chưa đồng ý)$/.test((d.innerText ?? "").trim()) && d.children.length === 0);
        // A row is openable when an ancestor of its caption is a button or link.
        const moDuoc = hang.filter((h) => h.closest('[role="button"],[role="link"],a[href]')).length;
        return { soNut: nut.length, tenKhacNhau: new Set(ten).size, vd: ten[0], cao: nut[0] ? Math.round(nut[0].getBoundingClientRect().height) : null, soHang: hang.length, moDuoc };
      });
      await chup(t.page, { out: mt.out, id: `EV-F06-THANH-VIEN-20-${cfg}`, suKien: t.suKien });
      log({ cfg, r });
      ket({ tc: "TC-F06-THANH-VIEN-20", screen: "F06.S02", state: "nhóm 20 người, người xem là quản trị", action: "mở Thành viên", cauHinh: cfg, expected: "mỗi nút đổi vai trò có tên riêng (kèm tên người) và ≥ 48dp; chạm một hàng mở hồ sơ người đó", status: r.soNut > 0 && r.tenKhacNhau === r.soNut && r.cao >= 48 && r.moDuoc > 0 ? "PASS" : "FAIL", evidence: [`EV-F06-THANH-VIEN-20-${cfg}`], ghiChu: `${r.soNut} nút đổi vai trò, ${r.tenKhacNhau} tên khác nhau (vd «${r.vd}»), cao ${r.cao}; ${r.soHang} hàng, ${r.moDuoc} hàng chạm mở được` });
      await t.context.close();
    }
    // Promote and demote in the F06 group, where it harms nobody.
    const p51 = await phienCua("moi-51");
    const nhom = (await ngCanh(p51)).find((c) => c.kind === "group" && /^Nhóm kiểm thử F06 hội săn mây/.test(c.display_name ?? ""));
    if (nhom) {
      const t = await moQua("C1", P("moi-51"), `/groups/${nhom.id}/members`);
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(800);
      const nut = await timNut(t.page, "Đặt làm quản trị");
      if (nut) {
        await cham(cdp, nut);
        await t.page.waitForTimeout(2000);
      }
      const sau = await t.page.evaluate(() => ({ hoi: /Chắc|xác nhận|Thôi/.test(document.body.innerText ?? ""), nut: [...document.querySelectorAll('[role="button"]')].map((e) => (e.innerText ?? "").trim()).filter((x) => /quản trị/.test(x)) }));
      await chup(t.page, { out: mt.out, id: "EV-F06-VAI-TRO-C1", suKien: t.suKien });
      // The first «Bỏ quyền quản trị» in the list is one's OWN row (it comes
      // first): a first run tapped it and the creator lost admin (see part
      // tu-bo-quan-tri). Demote the member just promoted, found by its row.
      const bo = await t.page.evaluate(() => {
        const hang = [...document.querySelectorAll('[role="button"]')].filter((e) => (e.innerText ?? "").trim() === "Bỏ quyền quản trị").find((e) => !/\(bạn\)/.test(e.parentElement?.innerText ?? ""));
        if (!hang) return null;
        const r = hang.getBoundingClientRect();
        return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
      });
      if (bo) {
        await cham(cdp, bo);
        await t.page.waitForTimeout(2000);
      }
      log({ nut, sau });
      ket({ tc: "TC-F06-VAI-TRO", screen: "F06.S02", state: "nhóm F06: quản trị + 1 thành viên", action: "«Đặt làm quản trị», rồi bỏ lại", cauHinh: "C1", expected: "đổi vai trò có bước hỏi hoặc lối hoàn tác thấy ngay; nhãn đổi theo", status: nut && (sau.hoi || sau.nut.some((x) => /Bỏ quyền quản trị/.test(x))) ? "PASS" : "FAIL", evidence: ["EV-F06-VAI-TRO-C1"], ghiChu: `nút ${nut ? `«${nut.ten}» ${nut.w}×${nut.h}` : "không có (thành viên chưa đồng ý?)"}; sau khi chạm: bước hỏi ${sau.hoi ? "có" : "không"}, nút: ${sau.nut.join(" · ")}` });
      await t.context.close();
    }
  }

  // ------------------------------------------ an admin demoting themselves
  // Found by accident: a run tapped the first «Bỏ quyền quản trị», which is
  // one's own row, and the creator lost admin in one tap. Reproduced on
  // purpose here with a capture before and after, then put back over the API.
  if (chay("tu-bo-quan-tri")) {
    const p51 = await phienCua("moi-51");
    const p52 = await phienCua("moi-52");
    const nhom = (await ngCanh(p51)).find((c) => c.kind === "group" && /^Nhóm kiểm thử F06 hội săn mây/.test(c.display_name ?? ""));
    const { randomUUID } = await import("node:crypto");
    const vaiTro = async (phien, personId, role) => {
      const r = await fetch(`${mt.api}/contexts/${nhom.id}/members/${personId}/role`, { method: "PUT", headers: { authorization: `Bearer ${phien.token}`, "content-type": "application/json", "idempotency-key": randomUUID() }, body: JSON.stringify({ role }) });
      return r.status;
    };
    const vaiCua = async (personId) => ((await api("GET", `/contexts/${nhom.id}/members`, undefined, p51)).members ?? []).find((m) => m.person_id === personId)?.role;
    // Two admins, so the app offers the creator to step down.
    const vaiTruoc = await vaiCua(p51.person_id);
    const ma1 = vaiTruoc === "admin" ? "sẵn" : await vaiTro(p52, p51.person_id, "admin");
    const ma2 = (await vaiCua(p52.person_id)) === "admin" ? "sẵn" : await vaiTro(p51, p52.person_id, "admin");
    const t = await moQua("C1", P("moi-51"), `/groups/${nhom.id}/members`);
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(1000);
    const cuaToi = await t.page.evaluate(() => {
      const b = [...document.querySelectorAll('[role="button"]')].find((e) => (e.innerText ?? "").trim() === "Bỏ quyền quản trị" && /\(bạn\)/.test(e.parentElement?.innerText ?? ""));
      if (!b) return null;
      b.setAttribute("data-audit-tu-bo", "1");
      const tatCa = [...document.querySelectorAll('[role="button"]')].filter((e) => /quản trị/.test(e.innerText ?? ""));
      const r = b.getBoundingClientRect();
      return { x: r.left + r.width / 2, y: r.top + r.height / 2, thuTu: tatCa.indexOf(b) + 1, tong: tatCa.length, ten: b.getAttribute("aria-label") ?? (b.innerText ?? "").trim() };
    });
    await chup(t.page, { out: mt.out, id: "EV-F06-TU-BO-TRUOC-C1", suKien: t.suKien, chuThich: cuaToi ? [{ selector: "[data-audit-tu-bo]", nhan: "hàng của chính mình" }] : [] });
    if (cuaToi) await cham(cdp, cuaToi);
    await t.page.waitForTimeout(2500);
    const hoi = await coChu(t.page, /Chắc chưa|Bạn chắc|xác nhận|mất quyền|không tự lấy lại/i);
    const vaiSau = await vaiCua(p51.person_id);
    await chup(t.page, { out: mt.out, id: "EV-F06-TU-BO-SAU-C1", suKien: t.suKien });
    // Put back: moi-51 sole admin, moi-52 member.
    const traLai = [await vaiTro(p52, p51.person_id, "admin"), await vaiTro(p51, p52.person_id, "member")];
    log({ vaiTruoc, ma1, ma2, cuaToi, hoi, vaiSau, traLai, cuoi: [await vaiCua(p51.person_id), await vaiCua(p52.person_id)] });
    ket({ tc: "TC-F06-TU-BO-QUAN-TRI", screen: "F06.S02", state: "nhóm có hai quản trị; người xem là người lập nhóm", action: "chạm «Bỏ quyền quản trị» trên hàng của chính mình", cauHinh: "C1", expected: "việc tự bỏ quyền (không tự lấy lại được) có bước hỏi, và nút nói rõ là của chính mình", status: cuaToi && hoi ? "PASS" : "FAIL", evidence: ["EV-F06-TU-BO-TRUOC-C1", "EV-F06-TU-BO-SAU-C1"], ghiChu: cuaToi ? `nút của mình là nút thứ ${cuaToi.thuTu}/${cuaToi.tong} mang tên «${cuaToi.ten}» (không có tên người); sau một chạm: bước hỏi ${hoi ? "có" : "không"}, vai trò trên máy chủ ${vaiTruoc} → ${vaiSau}; đã trả lại qua API (HTTP ${traLai.join(", ")})` : "không thấy nút của chính mình" });
    await t.context.close();
  }

  // ----------------- add a friend, reached and left by the app's own buttons
  // The first run opened /friends/add by pushState, which leaves the router
  // one screen deep, so «Về danh sách bạn» (router.back) had nowhere to go.
  // Here the screen is reached by tapping «Thêm bạn bằng số điện thoại».
  if (chay("them-ban-app")) {
    const t = await mo("C1", P("chat-20"), "/friends");
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(900);
    await bam(t, cdp, "Thêm bạn bằng số điện thoại", { cho: 1200 });
    const vao = await duongDan(t.page);
    await go(t.page, "Ô số điện thoại bạn", P("moi-52").phone);
    await bam(t, cdp, "Tìm", { cho: 2000 });
    await bam(t, cdp, "Gửi lời mời", { cho: 2500 });
    const da = await chuTrang(t.page);
    await chup(t.page, { out: mt.out, id: "EV-F06-THEM-DA-GUI-C1", suKien: t.suKien });
    await bam(t, cdp, "Về danh sách bạn", { cho: 1500 });
    const ve = await duongDan(t.page);
    await bam(t, cdp, "Đã gửi", { cho: 900 });
    const daGui = await coChu(t.page, /Bạn thân từ hồi cấp ba/);
    ket({ tc: "TC-F06-THEM-GUI", screen: "F06.S06", state: "vào Thêm bạn từ nút của màn Bạn bè", action: "tìm số, «Gửi lời mời», «Về danh sách bạn», xem «Đã gửi»", cauHinh: "C1", expected: "«Đã gửi lời mời tới …»; về Bạn bè; lời mời nằm ở «Đã gửi»", status: /\/friends\/add$/.test(vao) && /Đã gửi lời mời tới/.test(da) && /\/friends$/.test(ve) && daGui ? "PASS" : "FAIL", evidence: ["EV-F06-THEM-DA-GUI-C1"], ghiChu: `vào ${vao}; «${(da.match(/Đã gửi lời mời tới[^.]*/) ?? ["không"])[0].slice(0, 90)}»; về ${ve}; «Đã gửi» ${daGui ? "có" : "không có"} người vừa mời` });
    await t.context.close();
  }

  // ------------------------------------------------------------ friends list
  if (chay("ban-be")) {
    for (const cfg of ["C1"]) {
      const t = await moQua(cfg, P("chat-20"), "/friends");
      const cdp = await cdpCua(t.page);
      const tabs = await t.page.evaluate(() => [...document.querySelectorAll('[role="tab"]')].map((e) => ({ ten: (e.innerText ?? "").trim(), chon: e.getAttribute("aria-selected"), h: Math.round(e.getBoundingClientRect().height) })));
      const rong = [];
      for (const muc of ["Đã là bạn", "Đã nhận", "Đã gửi"]) {
        await bam(t, cdp, muc, { cho: 500 });
        rong.push(`${muc}: «${(await viTriCau(t.page, /Chưa có bạn nào|Không có lời mời|Chưa gửi|Chưa có lời mời|lời mời/))?.text ?? "không có câu"}»`);
      }
      await chup(t.page, { out: mt.out, id: `EV-F06-BAN-RONG-${cfg}`, suKien: t.suKien });
      ket({ tc: "TC-F06-BAN-RONG", screen: "F06.S05", state: "chưa có bạn, không lời mời", action: "chạm lần lượt 3 phân đoạn", cauHinh: cfg, expected: "mỗi phân đoạn có trạng thái rỗng riêng; tab đang chọn báo aria-selected; tab ≥ 48dp", status: tabs.length === 3 && tabs.filter((x) => x.chon === "true").length === 1 && tabs.every((x) => x.h >= 48) && rong.every((x) => !/không có câu/.test(x)) ? "PASS" : "FAIL", evidence: [`EV-F06-BAN-RONG-${cfg}`], ghiChu: `tab: ${tabs.map((x) => `${x.ten} ${x.chon} ${x.h}`).join(", ")}; ${rong.join("; ")}` });
      await t.context.close();
    }
    // The long list at 320dp: can the last row come out from under the footer?
    const t = await mo("C2", P("dalat-0"), "/friends");
    await t.page.waitForTimeout(1000);
    const r = await t.page.evaluate(async () => {
      const sc = [...document.querySelectorAll("div")].find((d) => d.scrollHeight > d.clientHeight + 10 && /(auto|scroll)/.test(getComputedStyle(d).overflowY));
      if (sc) { sc.scrollTop = sc.scrollHeight; sc.dispatchEvent(new Event("scroll", { bubbles: true })); }
      await new Promise((ok) => setTimeout(ok, 600));
      const hang = [...document.querySelectorAll('[role="button"]')].filter((e) => /^Nhắn tin cho /.test(e.getAttribute("aria-label") ?? ""));
      const cuoi = hang[hang.length - 1]?.getBoundingClientRect();
      const chan = [...document.querySelectorAll('[role="button"]')].find((e) => /Thêm bạn bằng số điện thoại/.test(e.innerText ?? ""))?.getBoundingClientRect();
      return { soHang: hang.length, cuoiDay: cuoi ? Math.round(cuoi.bottom) : null, chanTren: chan ? Math.round(chan.top) : null, cuon: !!sc };
    });
    await chup(t.page, { out: mt.out, id: "EV-F06-BAN-CUOI-C2", suKien: t.suKien });
    log({ r });
    ket({ tc: "TC-F06-BAN-CUOI", screen: "F06.S05", state: "7 bạn, 320dp", action: "cuộn tới cuối danh sách", cauHinh: "C2", expected: "hàng cuối (và nút «Nhắn tin» của nó) nằm trọn trên nút chân trang", status: r.cuoiDay !== null && r.chanTren !== null && r.cuoiDay <= r.chanTren ? "PASS" : "FAIL", evidence: ["EV-F06-BAN-CUOI-C2"], ghiChu: `${r.soHang} hàng; nút «Nhắn tin» cuối đáy y ${r.cuoiDay}, chân trang bắt đầu y ${r.chanTren}` });
    await t.context.close();
  }

  // ---------------------------------------------- add a friend by the number
  if (chay("them-ban")) {
    const p20 = await phienCua("chat-20");
    const p21 = await phienCua("chat-21");
    const banTruoc = ((await api("GET", `/people/${p20.person_id}/friends`, undefined, p20)).friends ?? []).length;
    const t = await moQua("C1", P("chat-20"), "/friends/add", "/friends");
    const cdp = await cdpCua(t.page);
    const thu = async (so) => {
      await go(t.page, "Ô số điện thoại bạn", so);
      await bam(t, cdp, "Tìm", { cho: 2000 });
      return { cau: (await viTriCau(t.page, /Chưa đúng dạng|chính bạn|Chưa có ai dùng|Không tìm thấy|chưa dùng|không có|sự cố|Kiểm tra mạng|Chưa tìm/))?.text ?? null, thay: await coChu(t.page, /Tìm thấy theo số điện thoại/) };
    };
    const sai = await thu("12345");
    const minh = await thu(P("chat-20").phone);
    const la = await thu(P("moi-99").phone);
    await chup(t.page, { out: mt.out, id: "EV-F06-THEM-LA-C1", suKien: t.suKien });
    ket({ tc: "TC-F06-THEM-SAI", screen: "F06.S06", state: "số sai dạng, số của mình, số chưa ai dùng", action: "«Tìm» ba lần", cauHinh: "C1", expected: "mỗi lần một câu đúng lý do, không lộ gì về số lạ", status: sai.cau && minh.cau && la.cau && !la.thay ? "PASS" : "FAIL", evidence: ["EV-F06-THEM-LA-C1"], ghiChu: `sai dạng: «${sai.cau}»; số của mình: «${minh.cau}»; số chưa ai dùng: «${la.cau}»` });
    if (banTruoc === 0) {
      const thay = await thu(P("chat-21").phone);
      const the = await t.page.evaluate(() => (document.querySelector('[data-testid="danh-thiep"]')?.innerText ?? "").replace(/\s+/g, " "));
      await chup(t.page, { out: mt.out, id: "EV-F06-THEM-THAY-C1", suKien: t.suKien });
      ket({ tc: "TC-F06-THEM-THAY", screen: "F06.S06", state: "số của một người đã dùng Rủ Đi, chưa là bạn, không chung nhóm", action: "«Tìm»", cauHinh: "C1", expected: "thẻ người tìm thấy và «Gửi lời mời»", status: thay.thay ? "PASS" : "FAIL", evidence: ["EV-F06-THEM-THAY-C1"], ghiChu: `thẻ: «${the.slice(0, 120)}». Ghi chú: ở mốc này tra số trả về tên của người không chung nhóm; main đã đổi luật ở dd75752 (retest ở hàng đợi sau pipeline)` });
      await bam(t, cdp, "Gửi lời mời", { cho: 2500 });
      const da = await chuTrang(t.page);
      await bam(t, cdp, "Về danh sách bạn", { cho: 1500 });
      const ve = await duongDan(t.page);
      await bam(t, cdp, "Đã gửi", { cho: 800 });
      const daGui = await coChu(t.page, /Chat Test 22/);
      ket({ tc: "TC-F06-THEM-GUI", screen: "F06.S06", state: "đã tìm thấy", action: "«Gửi lời mời», «Về danh sách bạn», xem «Đã gửi»", cauHinh: "C1", expected: "«Đã gửi lời mời tới …»; về Bạn bè; lời mời nằm ở «Đã gửi»", status: /Đã gửi lời mời tới/.test(da) && /\/friends$/.test(ve) && daGui ? "PASS" : "FAIL", evidence: [], ghiChu: `«${(da.match(/Đã gửi lời mời tới[^.]*/) ?? ["không"])[0]}»; về ${ve}; «Đã gửi» ${daGui ? "có" : "không có"} Chat Test 22` });
    }
    await t.context.close();
    // The other side: accept, with one failure first.
    const t2 = await moQua("C1", P("chat-21"), "/friends?muc=da-nhan", "/profile");
    const cdp2 = await cdpCua(t2.page);
    await t2.page.waitForTimeout(800);
    const coLoiMoi = await timNut(t2.page, "Đồng ý");
    if (coLoiMoi) {
      await loiMayChu(t2.page, /\/friend-requests\/[^/]+(\/|\?|$)|\/friends\/requests\/[^/]+/);
      await bam(t2, cdp2, "Đồng ý", { cho: 1800 });
      const hong = { tieuDe: (await viTriCau(t2.page, /Chưa đọc được danh sách bạn/))?.text ?? null, conHang: !!(await timNut(t2.page, "Đồng ý")), chu: await chuTrang(t2.page) };
      await chup(t2.page, { out: mt.out, id: "EV-F06-DONG-Y-503-C1", suKien: t2.suKien });
      await goHet(t2.page);
      await bam(t2, cdp2, "Thử lại", { cho: 1500 });
      await bam(t2, cdp2, "Đồng ý", { cho: 2500 });
      const ban = ((await api("GET", `/people/${p21.person_id}/friends`, undefined, p21)).friends ?? []).map((b) => b.display_name);
      log({ hong, ban });
      ket({ tc: "TC-F06-DONG-Y-503", screen: "F06.S05", state: "lời mời kết bạn đang chờ; máy chủ trả 503 cho câu trả lời", action: "«Đồng ý»; rồi máy chủ ổn, chạm lại", cauHinh: "C1", expected: "câu lỗi nằm cạnh hàng vừa chạm, danh sách giữ nguyên; chạm lại thì thành bạn", status: !hong.tieuDe && hong.conHang && ban.includes("Chat Test 21") ? "PASS" : "FAIL", evidence: ["EV-F06-DONG-Y-503-C1"], ghiChu: `khi 503: ${hong.tieuDe ? `cả danh sách thành màn lỗi «${hong.tieuDe}»` : "danh sách còn"}, hàng lời mời ${hong.conHang ? "còn" : "mất"}; sau khi chạm lại: bạn của Chat Test 22 = ${ban.join(", ") || "không ai"}` });
      // «Nhắn tin» from the friend's row opens the pair.
      await bam(t2, cdp2, "Đã là bạn", { cho: 800 });
      await bam(t2, cdp2, "Nhắn tin cho Chat Test 21", { cho: 3000 });
      const d = await duongDan(t2.page);
      ket({ tc: "TC-F06-NHAN-TIN", screen: "F06.S05", state: "vừa thành bạn", action: "«Nhắn tin» trên hàng bạn", cauHinh: "C1", expected: "mở chat hai người", status: /\/groups\/[^/]+\/chat$/.test(d) ? "PASS" : "FAIL", evidence: [], ghiChu: `tới ${an(d)}` });
    } else {
      log({ boQua: "Chat Test 22 không có lời mời đang chờ (đã là bạn từ lượt trước?)" });
    }
    await t2.context.close();
  }

  // ------------------------------------------------ somebody else's profile
  if (chay("ho-so")) {
    const p5 = await phienCua("chat-5");
    const t = await moQua("C1", P("chat-0"), `/people/${p5.person_id}`, "/friends");
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(1000);
    const man = await chuTrang(t.page);
    await chup(t.page, { out: mt.out, id: "EV-F06-HO-SO-CHUNG-NHOM-C1", suKien: t.suKien });
    const kb = await timNut(t.page, "Kết bạn");
    ket({ tc: "TC-F06-HO-SO-CHUNG-NHOM", screen: "F06.S07", state: "người chung nhóm, chưa là bạn", action: "mở hồ sơ", cauHinh: "C1", expected: "nói vì sao chưa nhắn riêng được và có «Kết bạn» ngay đó (ADR-0038 §2.2)", status: /Kết bạn để nhắn riêng/.test(man) && kb ? "PASS" : "FAIL", evidence: ["EV-F06-HO-SO-CHUNG-NHOM-C1"], ghiChu: `câu ${/Kết bạn để nhắn riêng/.test(man) ? "có" : "không"}; nút «Kết bạn» ${kb ? `${kb.w}×${kb.h}` : "không có"}` });
    // The actions sheet (L06): open, look, close four ways.
    const kq = {};
    for (const cach of ["esc", "nen", "keo-dai", "back"]) {
      await bam(t, cdp, "Thêm hành động", { cho: 900 });
      const mo0 = await demDialog(t.page);
      const f = await focusHienTai(t.page);
      const nut = await t.page.evaluate(() => [...document.querySelectorAll('[role="dialog"] [role="button"]')].map((e) => (e.getAttribute("aria-label") || e.innerText || "").trim()).filter((x) => x && !/^Đóng/.test(x)));
      if (cach === "esc") await chup(t.page, { out: mt.out, id: "EV-F06-HANH-DONG-C1", suKien: t.suKien });
      await dong(t.page, cdp, cach);
      await t.page.waitForTimeout(900);
      kq[cach] = { mo: mo0, focus: f?.ten ?? f?.role ?? null, nut, con: await demDialog(t.page), duong: await duongDan(t.page) };
      if (!/\/people\//.test(kq[cach].duong)) await diToi(t, `/people/${p5.person_id}`);
    }
    log({ kq });
    const fm = (x) => `${x.mo} → ${x.con} hộp thoại, ở ${an(x.duong)}`;
    ket({ tc: "TC-L06-VONGDOI", screen: "F06.S07", layer: "L06", state: "hồ sơ người chung nhóm", action: "«Thêm hành động»; đóng bằng Esc, nền, kéo xuống, Back", cauHinh: "C1", expected: "sheet có «Chặn …» và «Báo cáo»; focus vào sheet; mọi cách đóng đều ở lại hồ sơ", status: Object.values(kq).every((x) => x.mo === 1 && x.con === 0 && /\/people\//.test(x.duong)) ? "PASS" : "FAIL", evidence: ["EV-F06-HANH-DONG-C1"], ghiChu: `nút: ${kq.esc.nut.join(" · ")}; focus «${kq.esc.focus}»; ${Object.entries(kq).map(([k, v]) => `${k}: ${fm(v)}`).join("; ")}` });
    // «Kết bạn» from the profile.
    const kb2 = await timNut(t.page, "Kết bạn");
    if (kb2) {
      await cham(cdp, kb2);
      await t.page.waitForTimeout(2500);
    }
    const daGui = await viTriCau(t.page, /Đã gửi lời mời kết bạn/);
    ket({ tc: "TC-F06-HO-SO-KET-BAN", screen: "F06.S07", state: "người chung nhóm", action: "«Kết bạn»", cauHinh: "C1", expected: "nút thành câu «Đã gửi lời mời kết bạn…» tại chỗ", status: daGui?.trongMan ? "PASS" : "FAIL", evidence: [], ghiChu: `«${daGui?.text ?? "không có câu"}» y ${daGui?.top}` });
    await t.context.close();
  }

  // ------------------------------------------------------------------ block
  if (chay("chan")) {
    const p20 = await phienCua("chat-20");
    const p21 = await phienCua("chat-21");
    const t = await moQua("C1", P("chat-20"), `/people/${p21.person_id}`, "/friends");
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(1000);
    const quanHe = await chuTrang(t.page);
    await bam(t, cdp, "Thêm hành động", { cho: 900 });
    await bam(t, cdp, "Chặn Chat Test 22", { cho: 800 });
    const hoi = await chuTrang(t.page);
    await chup(t.page, { out: mt.out, id: "EV-F06-CHAN-HOI-C1", suKien: t.suKien });
    await bam(t, cdp, "Chặn", { cho: 2500 });
    const sau = { dialog: await demDialog(t.page), chip: await coChu(t.page, /Đã chặn/), nhanTin: !!(await timNut(t.page, "Nhắn tin")), chu: await chuTrang(t.page) };
    await chup(t.page, { out: mt.out, id: "EV-F06-CHAN-XONG-C1", suKien: t.suKien });
    const ds = (await api("GET", "/people/me/blocked", undefined, p20)).blocked ?? [];
    ket({ tc: "TC-F06-CHAN", screen: "F06.S07", layer: "L06", state: "hồ sơ một người bạn", action: "«Thêm hành động» → «Chặn …» → «Chặn»", cauHinh: "C1", expected: "có bước hỏi nói hậu quả; sau đó sheet đóng, hồ sơ hiện «Đã chặn», không còn mời nhắn tin", status: /Chặn/.test(hoi) && sau.dialog === 0 && sau.chip && !sau.nhanTin && ds.length === 1 ? "PASS" : "FAIL", evidence: ["EV-F06-CHAN-HOI-C1", "EV-F06-CHAN-XONG-C1"], ghiChu: `quan hệ trước: ${/Nhắn tin/.test(quanHe) ? "bạn (có «Nhắn tin»)" : "khác"}; sau: ${sau.dialog} hộp thoại, «Đã chặn» ${sau.chip ? "có" : "không"}, «Nhắn tin» ${sau.nhanTin ? "vẫn còn" : "hết"}; danh sách chặn trên máy chủ: ${ds.length}` });
    // Leave and come back: the flag is local by design (ADR-0023 §2.3).
    await t.page.reload();
    await choOn(t.page, { mang: t.mang });
    await t.page.waitForTimeout(1500);
    const lai = { chip: await coChu(t.page, /Đã chặn/), nhanTin: !!(await timNut(t.page, "Nhắn tin")) };
    await bam(t, cdp, "Thêm hành động", { cho: 900 });
    const trongSheet = await t.page.evaluate(() => [...document.querySelectorAll('[role="dialog"] [role="button"]')].map((e) => (e.innerText ?? "").trim()).filter((x) => /Chặn|Bỏ chặn/.test(x)));
    await chup(t.page, { out: mt.out, id: "EV-F06-CHAN-LAI-C1", suKien: t.suKien });
    await t.page.keyboard.press("Escape");
    ket({ tc: "TC-F06-CHAN-MO-LAI", screen: "F06.S07", layer: "L06", state: "đã chặn người này, mở lại hồ sơ (tải lại)", action: "nhìn hồ sơ, mở «Thêm hành động»", cauHinh: "C1", expected: "vẫn thấy mình đã chặn người này; sheet mời «Bỏ chặn», không mời chặn lần nữa", status: lai.chip && trongSheet.some((x) => /Bỏ chặn/.test(x)) ? "PASS" : "FAIL", evidence: ["EV-F06-CHAN-LAI-C1"], ghiChu: `«Đã chặn» ${lai.chip ? "có" : "không"}; «Nhắn tin» ${lai.nhanTin ? "có" : "không"}; sheet: ${trongSheet.join(" · ")}` });
    // The pair chat after the block, from the blocker's side.
    const cap = (await ngCanh(p20)).find((c) => c.kind === "pair");
    if (cap) {
      await diToi(t, `/groups/${cap.id}/chat`);
      const chat = { dung: await viTriCau(t.page, /không còn nhận tin|đã chặn|bị chặn/), oSoan: await t.page.evaluate(() => !!document.querySelector('textarea[aria-label="Ô soạn tin"]')) };
      await chup(t.page, { out: mt.out, id: "EV-F06-CHAN-CHAT-C1", suKien: t.suKien });
      ket({ tc: "TC-F06-CHAN-CHAT", screen: "F05.S03", state: "chat hai người, mình vừa chặn người kia", action: "mở chat đôi", cauHinh: "C1", expected: "nói rõ đã chặn và cách bỏ chặn; không mời gõ tin", status: chat.dung && !chat.oSoan ? "PASS" : "FAIL", evidence: ["EV-F06-CHAN-CHAT-C1"], ghiChu: `câu «${chat.dung?.text ?? "không có"}»; ô soạn ${chat.oSoan ? "còn" : "không"}` });
    }
    await t.context.close();
  }

  // ------------------------------- the pair chat after a block, both sides
  // Measured at 3 s and at 20 s: the reconnecting note, the empty state, the
  // paper band, the sentence in place of the composer.
  if (chay("chan-chat")) {
    const p20 = await phienCua("chat-20");
    const cap = (await ngCanh(p20)).find((c) => c.kind === "pair");
    const kq = {};
    for (const ten of ["chat-20", "chat-21"]) {
      const t = await mo("C1", P(ten), "/messages");
      await diToi(t, `/groups/${cap.id}/chat`);
      const doc = () => t.page.evaluate(() => { const x = (document.body.innerText ?? "").replace(/\s+/g, " "); return { noiLai: /Đang nối lại/.test(x), moDau: /Một lời mở đầu/.test(x), toGiay: /Đi đâu không\?/.test(x), dung: (x.match(/Cuộc trò chuyện này[^.]*\./) ?? [null])[0], chan: /chặn/i.test(x), oSoan: !!document.querySelector('textarea[aria-label="Ô soạn tin"]') }; });
      const luc3 = await doc();
      await t.page.waitForTimeout(17_000);
      const luc20 = await doc();
      await chup(t.page, { out: mt.out, id: `EV-F06-CHAN-CHAT-${ten === "chat-20" ? "NGUOI-CHAN" : "BI-CHAN"}-C1`, suKien: t.suKien });
      kq[ten] = { luc3, luc20 };
      await t.context.close();
    }
    log({ kq });
    const a = kq["chat-20"].luc20;
    const f = (x) => `«Đang nối lại» ${x.noiLai ? "có" : "không"}, «Một lời mở đầu» ${x.moDau ? "có" : "không"}, dải tờ giấy mời hẹn ${x.toGiay ? "có" : "không"}, câu «${x.dung ?? "không"}», chữ «chặn» ${x.chan ? "có" : "không"}, ô soạn ${x.oSoan ? "có" : "không"}`;
    ket({ tc: "TC-F06-CHAN-CHAT", screen: "F05.S03", state: "chat hai người, người xem vừa chặn người kia", action: "mở chat đôi, nhìn lúc 3 s và 20 s", cauHinh: "C1", expected: "nói rõ mình đã chặn và cách bỏ chặn; không còn lời mời nhắn tin hay hẹn; không báo «Đang nối lại» mãi", status: a.chan && !a.moDau && !a.toGiay && !a.noiLai && !a.oSoan ? "PASS" : "FAIL", evidence: ["EV-F06-CHAN-CHAT-NGUOI-CHAN-C1"], ghiChu: `người chặn, 3 s: ${f(kq["chat-20"].luc3)}; 20 s: ${f(a)}` });
    ket({ tc: "TC-F06-BI-CHAN-CHAT", screen: "F05.S03", state: "chat hai người, người xem vừa bị người kia chặn", action: "mở chat đôi, nhìn lúc 20 s", cauHinh: "C1", expected: "không gõ được tin; không có lời mời nhắn hay hẹn; không báo «Đang nối lại» mãi (không bắt buộc nói là bị chặn)", status: !kq["chat-21"].luc20.oSoan && !kq["chat-21"].luc20.moDau && !kq["chat-21"].luc20.toGiay && !kq["chat-21"].luc20.noiLai ? "PASS" : "FAIL", evidence: ["EV-F06-CHAN-CHAT-BI-CHAN-C1"], ghiChu: `người bị chặn, 20 s: ${f(kq["chat-21"].luc20)}` });
  }

  // ------------------------------------------------------------ tap targets
  if (chay("vung-bam")) {
    const p0 = await phienCua("chat-0");
    const G20 = (await ngCanh(p0)).find((c) => c.kind === "group")?.id;
    const ketQua = [];
    for (const [nhan, persona, path] of [["Bạn bè", "dalat-0", "/friends"], ["Lập nhóm", "chat-0", "/groups/new"], ["Mời", "chat-0", `/groups/${G20}/invite`], ["Thêm bạn", "chat-0", "/friends/add"], ["Hồ sơ", "chat-0", `/people/${(await phienCua("chat-5")).person_id}`]]) {
      const t = await mo("C1", P(persona), path);
      await t.page.waitForTimeout(1000);
      const nho = await t.page.evaluate(() => [...document.querySelectorAll('[role="button"],button,input,[role="link"],a[href],[role="tab"]')].filter((e) => { const r = e.getBoundingClientRect(); return r.width > 0 && r.height > 0 && (r.height < 48 || r.width < 48); }).map((e) => { const r = e.getBoundingClientRect(); return `${(e.getAttribute("aria-label") || e.innerText || e.tagName).replace(/[\uE000-\uF8FF]/g, "").replace(/\s+/g, " ").trim().slice(0, 36)} ${Math.round(r.width)}×${Math.round(r.height)}`; }));
      ketQua.push(`${nhan}: ${nho.length ? [...new Set(nho)].join(", ") : "không có"}`);
      await t.context.close();
    }
    ket({ tc: "TC-F06-VUNG-BAM", screen: "F06.S05", state: "Bạn bè (7 bạn), Lập nhóm, Mời, Thêm bạn, Hồ sơ người chung nhóm", action: "đo mọi phần tử bấm được", cauHinh: "C1", expected: "mọi vùng bấm ≥ 48dp (DESIGN.md), tối thiểu 44 (PRODUCT)", status: ketQua.every((x) => /không có$/.test(x)) ? "PASS" : "FAIL", evidence: [], ghiChu: ketQua.join("; ") });
  }

  // --------------------------------------------------------- cold open, back
  if (chay("lanh")) {
    const p0 = await phienCua("chat-0");
    const p5 = await phienCua("chat-5");
    const G20 = (await ngCanh(p0)).find((c) => c.kind === "group")?.id;
    for (const [man, path] of [["F06.S02", `/groups/${G20}/members`], ["F06.S03", `/groups/${G20}/invite`], ["F06.S05", "/friends"], ["F06.S06", "/friends/add"], ["F06.S07", `/people/${p5.person_id}`]]) {
      const t = await mo("C1", P("chat-0"), path);
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(900);
      const truoc = await duongDan(t.page);
      await bam(t, cdp, "Quay lại", { cho: 1200 });
      const sau = await duongDan(t.page);
      ket({ tc: `TC-${man}-BACK-LANH`, screen: man, state: "mở thẳng bằng link", action: "chạm «Quay lại»", cauHinh: "C1", expected: "tới một màn hợp lý", status: sau !== truoc ? "PASS" : "FAIL", issue: sau !== truoc ? null : "UI-018", evidence: [], ghiChu: `${an(truoc)} → ${an(sau)}` });
      await t.context.close();
    }
  }

  // -------------------------------------------------------------- failures
  if (chay("loi")) {
    const p0 = await phienCua("chat-0");
    const p5 = await phienCua("chat-5");
    const G20 = (await ngCanh(p0)).find((c) => c.kind === "group")?.id;
    for (const [man, path, mau, cau] of [
      ["F06.S02", `/groups/${G20}/members`, /\/contexts\/[^/]+\/members(\?|$)/, /Chưa đọc được danh sách thành viên/],
      ["F06.S05", "/friends", /\/people\/[^/]+\/friends(\?|$)/, /Chưa đọc được danh sách bạn/],
      ["F06.S07", `/people/${p5.person_id}`, new RegExp(`/people/${p5.person_id}(\\?|$)`), /Chưa mở được hồ sơ/],
    ]) {
      const t = await mo("C1", P("chat-0"), "/messages");
      const cdp = await cdpCua(t.page);
      await loiMayChu(t.page, mau);
      await diToi(t, path);
      await t.page.waitForTimeout(1200);
      const c = await viTriCau(t.page, cau);
      const thuLai = await timNut(t.page, "Thử lại");
      await chup(t.page, { out: mt.out, id: `EV-${man}-503-C1`, suKien: t.suKien });
      await goHet(t.page);
      if (thuLai) await bam(t, cdp, "Thử lại", { cho: 2000 });
      const het = !(await viTriCau(t.page, cau));
      ket({ tc: `TC-${man}-503`, screen: man, state: "GET chính của màn trả 503", action: "mở màn; máy chủ ổn; «Thử lại»", cauHinh: "C1", expected: "màn lỗi nói phần nào hỏng, có «Thử lại»; thử lại thì tải được", status: c && thuLai && het ? "PASS" : "FAIL", evidence: [`EV-${man}-503-C1`], ghiChu: `câu «${c?.text ?? "không có"}»; Thử lại ${thuLai ? "có" : "không"}; sau thử lại ${het ? "đã tải" : "vẫn lỗi"}` });
      await t.context.close();
    }
  }

  // ------------------------------------------------------ low window, tablet
  if (chay("c8")) {
    const p0 = await phienCua("chat-0");
    const G20 = (await ngCanh(p0)).find((c) => c.kind === "group")?.id;
    for (const [man, path, nut] of [["F06.S03", `/groups/${G20}/invite`, "Gửi lời mời"], ["F06.S06", "/friends/add", "Tìm"], ["F06.S01", "/groups/new", "Mở nhóm"]]) {
      const t = await mo("C8", P("chat-0"), path);
      await t.page.waitForTimeout(900);
      const n = await timNut(t.page, nut);
      await chup(t.page, { out: mt.out, id: `EV-${man}-C8`, suKien: t.suKien });
      ket({ tc: `TC-${man}-C8`, screen: man, state: "cửa sổ 390×460 (proxy bàn phím)", action: "mở form, cuộn tới nút chính", cauHinh: "C8", expected: "nút chính với tới được bằng cuộn, không bị che", status: n && n.top >= 0 && n.top + n.h <= 460 ? "PASS" : "FAIL", evidence: [`EV-${man}-C8`], ghiChu: n ? `«${nut}» ${n.w}×${n.h} ở y ${n.top} sau khi cuộn` : `không thấy «${nut}»` });
      await t.context.close();
    }
  }
  if (chay("tablet")) {
    const p0 = await phienCua("chat-0");
    const G20 = (await ngCanh(p0)).find((c) => c.kind === "group")?.id;
    for (const cfg of ["C6", "C7"]) {
      const t = await mo(cfg, P("dalat-0"), "/friends");
      await t.page.waitForTimeout(900);
      const r = await t.page.evaluate(() => {
        const hang = [...document.querySelectorAll('[role="button"]')].filter((e) => /^Nhắn tin cho /.test(e.getAttribute("aria-label") ?? ""));
        const a = hang[0]?.getBoundingClientRect();
        // The name's own glyphs, not its block: the block spans the row (a
        // first run measured the block and reported a false 12px).
        const o = [...document.querySelectorAll("div")].find((d) => (d.innerText ?? "").trim() === "Thu Thảo" && d.children.length === 0);
        let ten = null;
        if (o) { const rg = document.createRange(); rg.selectNodeContents(o); ten = rg.getBoundingClientRect(); }
        return { khoang: a && ten ? Math.round(a.left - ten.right) : null, cw: innerWidth, hangRong: o ? Math.round(o.getBoundingClientRect().width) : null };
      });
      await chup(t.page, { out: mt.out, id: `EV-F06-BAN-${cfg}`, suKien: t.suKien });
      ket({ tc: "TC-F06-BAN-TABLET", screen: "F06.S05", state: "7 bạn", action: "mở Bạn bè ở tablet", cauHinh: cfg, expected: "nút «Nhắn tin» còn gần tên người (hàng không bị kéo giãn quá rộng)", status: r.khoang !== null && r.khoang <= 480 ? "PASS" : "FAIL", evidence: [`EV-F06-BAN-${cfg}`], ghiChu: `từ cuối tên tới nút ${r.khoang}px (khối chữ rộng ${r.hangRong}), cửa sổ ${r.cw}` });
      await t.context.close();
    }
    void G20;
  }
} finally {
  await mt.dong();
}
