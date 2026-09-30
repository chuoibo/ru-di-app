/* Audit of a feature new on main 461eabf (from 16f24d5, #658, task #21): the profile that
 * tells a story. docs/architecture/03-profile-story-social.md is the design read against.
 *   - The journey book «Cuốn sổ có nhiều ngã rẽ» (/achievements): three paths and their
 *     endings, the crossroads, the badges one may display (three at most), MP4 credits and
 *     the templates «sắp dùng được», Nếp's suggestion behind a consent preview.
 *   - The personal wall v2 (/people/[id]): displayed badges as others see them, cursor
 *     pages, the 20 s long poll.
 *   - The wall post (/posts/[id], legacy reader): parent comments and one level of replies,
 *     likes on comments, reposts and their ACL, the photo full screen with its comment tray;
 *     observation Q3 (no way left to delete a comment).
 *
 *   source audit-env-main.sh && node kich-ban/n21-ho-so.mjs [--chi api,hanh-trinh,…]
 *
 * Order matters. Read first, before any write: `api`, `hanh-trinh`, `moi-mo`, `nep`, `loi`.
 * Then the writes, each checking what exists before it writes:
 *   `bai-anh`    chat-0's legacy «Bạn bè» post with a synthetic photo, a story of at least
 *                40 characters: on a Vietnam date after 30/09 it is chat-0's third story day,
 *                so the ending «Chuyện mình kể» becomes eligible;
 *   `ket`        chooses that ending and claims it; `trung-bay` displays three badges, then
 *                tries a fourth;
 *   `xem-nguoi`  reads chat-0's profile as a friend, a groupmate and a stranger;
 *   `binh-luan`, `anh-toan-man`, `dang-lai`  work on the photo post, two personas;
 *   `trang`      writes 15 short «Chỉ mình tôi» posts to chat-0's wall, then pages and polls.
 *
 * Personas (chat-test seed on stack 2, read over the API before any write):
 *   chat-0   owner of the wall and of the journey book
 *   chat-1   friend of chat-0: comments, replies, likes, reposts
 *   chat-16  groupmate of chat-0, not a friend
 *   dalat-0  outside the chat-test group: a stranger
 */
import { cauHinh } from "../thu-vien/cau-hinh.mjs";
import { chup } from "../thu-vien/chup.mjs";
import { cdpCua, cham, nhanGiu } from "../thu-vien/cu-chi.mjs";
import { choOn, duongDan } from "../thu-vien/dieu-huong.mjs";
import { focusHienTai } from "../thu-vien/lop-phu.mjs";
import { chayAxe } from "../thu-vien/axe.mjs";
import { loiMayChu } from "../thu-vien/mang.mjs";
import { khoiDong, trangMoi } from "../thu-vien/moi-truong.mjs";
import { layPhien, personaTheoTen } from "../thu-vien/phien.mjs";
import { soGhi } from "../thu-vien/ghi.mjs";
import { pngThuBytes } from "../../../../apps/mobile/tools/png-thu.mjs";
import { randomUUID } from "node:crypto";
import { readFileSync } from "node:fs";
import { join } from "node:path";

const chi = (() => {
  const i = process.argv.indexOf("--chi");
  return i === -1 ? null : new Set(process.argv[i + 1].split(","));
})();
const chay = (ten) => !chi || chi.has(ten) || [...chi].some((x) => x.startsWith(`${ten}:`));
const chayPhan = (ten, phan) => !chi || chi.has(ten) || chi.has(`${ten}:${phan}`);
const mt = await khoiDong();
const so = soGhi(mt.out);
const ghi = (rec) => {
  // Bare ISO dates side by side read, to the repo guard, as a phone number: quote each one (see n15-phan-xu.mjs, incident 34).
  if (rec.ghiChu) rec = { ...rec, ghiChu: rec.ghiChu.replace(/(?<!«)\b(\d{4}-\d{2}-\d{2})\b(?!»)/g, "«$1»") };
  so.ghi({ feature: "N21", nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, evidence: [], ...rec });
  console.log(`${rec.status.padEnd(10)} ${rec.tc} ${rec.cauHinh ?? ""} ${rec.ghiChu ?? ""}`);
};
const P = (ten) => personaTheoTen(ten, mt.chatSessions);
const phienDir = join(mt.out, "phien");
const phienCua = (ten) => layPhien(mt.api, P(ten), phienDir);
const mo = (cfg, persona, path) => trangMoi(mt, cauHinh(cfg), { persona: persona ? P(persona) : null, path });
const anh = (page, id, t, extra = {}) => chup(page, { out: mt.out, id, suKien: t.suKien, ...extra });
/** Any call, answering its status, the server's code and the body; never throws. */
const goi = async (method, path, body, phien) => {
  const headers = { authorization: `Bearer ${phien.token}` };
  if (method !== "GET") {
    headers["content-type"] = "application/json";
    headers["idempotency-key"] = randomUUID();
  }
  const r = await fetch(`${mt.api}${path}`, { method, headers, body: body === undefined ? undefined : JSON.stringify(body) });
  const text = await r.text();
  let json = null;
  try {
    json = JSON.parse(text);
  } catch {
    /* not JSON */
  }
  return { status: r.status, code: json?.code ?? null, json };
};
const chuTrang = (page) => page.evaluate(() => (document.body.innerText ?? "").replace(/[-]/g, "").replace(/\s+/g, " ").trim());
/** chat-N shows as «Chat Test NN», counted from 01. */
const tenHien = (ten) => `Chat Test ${String(Number(ten.split("-")[1]) + 1).padStart(2, "0")}`;
const idNguoi = (ten) => JSON.parse(readFileSync(mt.chatSessions, "utf8")).users.find((u) => `chat-${u.index}` === ten)?.personId;
const anId = (s) => String(s ?? "").replace(/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/g, "[id]");
/**
 * RudiButton's and StampButton's `lyDo` (ui.tsx, ui/StampButton.tsx): a disabled button with a
 * reason is followed by a sibling holding the info glyph and the sentence. One source for every
 * reader and for the `tu-kiem-ly-do` self-check, evaluated in the page.
 */
const LY_DO_SRC = String.raw`(b) => {
  const s = b.nextElementSibling;
  if (!s || s.matches('[role="button"]') || s.querySelector('[role="button"]') || !/[-]/.test(s.innerText ?? "")) return null;
  return (s.innerText ?? "").replace(/[-]/g, "").replace(/\s+/g, " ").trim() || null;
}`;
/**
 * A control by its accessible name or visible text, scrolled into view, and only
 * if a tap at its centre lands on it: a tab screen left mounted under a stack
 * screen carries controls of the same name (lesson of checkpoint retest 2).
 */
const nut = (page, chu, { batDau = false, trongDialog = false, trong = null } = {}) =>
  page.evaluate(
    ({ chu, batDau, trongDialog, trong }) => {
      const ten = (x) => (x.getAttribute("aria-label") || (x.innerText ?? "")).replace(/[-]/g, "").replace(/\s+/g, " ").trim();
      const goc = trongDialog ? [...document.querySelectorAll('[role="dialog"]')].filter((d) => d.getClientRects().length).pop() : trong ? document.querySelector(trong) : document;
      if (!goc) return null;
      for (const e of goc.querySelectorAll('[role="button"],button,[role="radio"],[role="checkbox"],[role="tab"],[role="link"],a[href]')) {
        if (!(batDau ? ten(e).startsWith(chu) : ten(e) === chu) || !e.getClientRects().length) continue;
        e.scrollIntoView({ block: "center", behavior: "instant" });
        const r = e.getBoundingClientRect();
        const x = r.left + r.width / 2;
        const y = r.top + r.height / 2;
        const tren = document.elementFromPoint(x, y);
        if (tren && (tren === e || e.contains(tren)))
          return { x, y, w: Math.round(r.width), h: Math.round(r.height), ten: ten(e), tat: e.getAttribute("aria-disabled") === "true" || e.hasAttribute("disabled") };
      }
      return null;
    },
    { chu, batDau, trongDialog, trong },
  );
const bam = async (t, cdp, chu, opts) => {
  const p = await nut(t.page, chu, opts);
  if (!p) return null;
  await cham(cdp, p);
  await t.page.waitForTimeout(opts?.cho ?? 900);
  return p;
};
/** A text field by its accessible name: tap it, select what it holds, type. */
const go = async (t, cdp, ten, chu, { xoa = true } = {}) => {
  const o = await t.page.evaluate((ten) => {
    const e = [...document.querySelectorAll(`[aria-label="${ten}"]`)].filter((x) => x.getClientRects().length && /^(INPUT|TEXTAREA)$/.test(x.tagName)).pop();
    if (!e) return null;
    e.scrollIntoView({ block: "center", behavior: "instant" });
    const r = e.getBoundingClientRect();
    return { x: r.left + Math.min(40, r.width / 2), y: r.top + Math.min(20, r.height / 2) };
  }, ten);
  if (!o) return false;
  await cham(cdp, o);
  await t.page.waitForTimeout(250);
  if (xoa) {
    await t.page.keyboard.press("Control+A");
    await t.page.keyboard.press("Backspace");
  }
  if (chu) await t.page.keyboard.type(chu, { delay: 8 });
  await t.page.waitForTimeout(300);
  return true;
};
const giaTri = (page, ten) => page.evaluate((ten) => [...document.querySelectorAll(`[aria-label="${ten}"]`)].filter((x) => x.getClientRects().length && /^(INPUT|TEXTAREA)$/.test(x.tagName)).pop()?.value ?? null, ten);
/** Inside the app, the way a tap would get there: the stack keeps its history. */
const diToi = async (t, path) => {
  await t.page.evaluate((p) => {
    history.pushState({}, "", p);
    dispatchEvent(new PopStateEvent("popstate"));
  }, path);
  await t.page.waitForTimeout(2200);
  await choOn(t.page, { mang: t.mang });
};
/** The top open dialog's label and visible text; null when none. */
const dialogTren = (page) =>
  page.evaluate(() => {
    const d = [...document.querySelectorAll('[role="dialog"]')].filter((x) => x.getClientRects().length && x.checkVisibility({ opacityProperty: true })).pop();
    if (!d) return null;
    const r = d.getBoundingClientRect();
    const nut = [...d.querySelectorAll('[role="button"]')].filter((b) => b.getClientRects().length).map((b) => ({ ten: (b.getAttribute("aria-label") || (b.innerText ?? "")).replace(/[-]/g, "").replace(/\s+/g, " ").trim(), h: Math.round(b.getBoundingClientRect().height), tat: b.getAttribute("aria-disabled") === "true" }));
    return { ten: d.getAttribute("aria-label"), chu: (d.innerText ?? "").replace(/[-]/g, "").replace(/\s+/g, " ").trim(), top: Math.round(r.top), cao: Math.round(r.height), rong: Math.round(r.width), nut };
  });



/**
 * One screen as drawn: the words, every control with the state a screen
 * reader gets, the error box and where it sits, the scroll position, and every
 * photo (loaded or not, and from what kind of address).
 */
const doMan = (page, testid = "achievements-screen") =>
  page.evaluate((testid) => {
    const hien = (e) => e && e.getClientRects().length > 0 && e.checkVisibility({ opacityProperty: true });
    const chu = (e) => (e?.innerText ?? "").replace(/[-]/g, "").replace(/\s+/g, " ").trim();
    const man = [...document.querySelectorAll(`[data-testid="${testid}"]`)].filter(hien).pop();
    if (!man) return null;
    const nut = [...man.querySelectorAll('[role="button"],[role="tab"],[role="checkbox"],[role="radio"]')].filter(hien).map((b) => {
      const r = b.getBoundingClientRect();
      return { vaiTro: b.getAttribute("role"), ten: (b.getAttribute("aria-label") || chu(b)).replace(/[-]/g, "").trim(), tat: b.getAttribute("aria-disabled") === "true", chon: b.getAttribute("aria-selected") ?? b.getAttribute("aria-checked"), y: Math.round(r.top), day: Math.round(r.bottom), h: Math.round(r.height), w: Math.round(r.width) };
    });
    let loi = null;
    const thu = [...man.querySelectorAll('[role="button"]')].filter(hien).find((b) => chu(b) === "Thử lại");
    if (thu) {
      let hop = thu;
      for (let i = 0; i < 6 && hop.parentElement && chu(hop).length < chu(thu).length + 20; i++) hop = hop.parentElement;
      const r = hop.getBoundingClientRect();
      loi = { chu: chu(hop), top: Math.round(r.top), day: Math.round(r.bottom), trongCuaSo: r.bottom > 0 && r.top < innerHeight };
    }
    const laCuon = (d) => {
      const s = getComputedStyle(d);
      return /(auto|scroll)/.test(s.overflowY) && d.scrollHeight > d.clientHeight + 4;
    };
    let cuon = [man, ...man.querySelectorAll("div")].find(laCuon) ?? null;
    for (let e = man.parentElement; !cuon && e; e = e.parentElement) if (laCuon(e)) cuon = e;
    const anh = [...man.querySelectorAll("img")].filter(hien).map((i) => {
      const r = i.getBoundingClientRect();
      return { w: Math.round(r.width), h: Math.round(r.height), nw: i.naturalWidth, nh: i.naturalHeight, xong: i.complete, kieu: /^blob:/.test(i.currentSrc) ? "blob" : /^data:/.test(i.currentSrc) ? "data" : i.currentSrc ? "url" : "rỗng", alt: i.getAttribute("alt") };
    });
    return { chu: chu(man), nut, loi, cuon: cuon ? { top: Math.round(cuon.scrollTop), cao: cuon.scrollHeight, nhin: cuon.clientHeight } : null, anh, h: innerHeight, w: innerWidth };
  }, testid);
/** Scroll a screen's own scroller (or the window) to y. */
const cuonToi = (page, y, testid = "achievements-screen") =>
  page.evaluate(
    ({ y, testid }) => {
      const man = [...document.querySelectorAll(`[data-testid="${testid}"]`)].filter((e) => e.getClientRects().length).pop();
      const laCuon = (d) => /(auto|scroll)/.test(getComputedStyle(d).overflowY) && d.scrollHeight > d.clientHeight + 4;
      let c = man ? [man, ...man.querySelectorAll("div")].find(laCuon) : null;
      for (let e = man?.parentElement; !c && e; e = e.parentElement) if (laCuon(e)) c = e;
      (c ?? document.scrollingElement).scrollTop = y;
    },
    { y, testid },
  );

const lyDoCua = async (page, ten) =>
  page.evaluate(
    ({ ten, src }) => {
      const doc = (0, eval)(src);
      const b = [...document.querySelectorAll('[role="button"]')].find((x) => x.getClientRects().length && (x.getAttribute("aria-label") || (x.innerText ?? "")).replace(/[-]/g, "").trim() === ten);
      return b ? { tat: b.getAttribute("aria-disabled") === "true", lyDo: doc(b) } : null;
    },
    { ten, src: LY_DO_SRC },
  );


// ------------------------------------------------------------ N21 helpers
const P0 = await phienCua("chat-0");
const ID0 = P0.person_id;
const NGOAI = "dalat-0"; // outside the chat-test group
const phienNgoai = () => layPhien(mt.api, personaTheoTen(NGOAI, mt.chatSessions), phienDir);
const phienAi = (ten) => (ten === NGOAI ? phienNgoai() : phienCua(ten));
/** The Vietnam calendar date: story days and «đã xong» are counted on it. */
const ngayVN = () => new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Ho_Chi_Minh" }).format(new Date());
const soCua = async (ten) => (await goi("GET", "/me/achievement-routes", undefined, await phienAi(ten))).json;
const TEN_KET = ["Dấu chân đầu tiên", "Khung ảnh đầu", "Lời kể đầu tiên", "Có người đi cùng", "Một ngày nhiều ngã", "Bản đồ mở", "Những tấm ảnh còn đây", "Chuyện mình kể", "Hẹn rồi lại hẹn", "Đủ mặt hôm nay", "Bản đồ thành trang", "Kỷ niệm chung", "Hành trình của mình"];
const KET = "Chuyện mình kể"; // ky_niem/storyteller: story days 2/3 before `bai-anh`
/** chat-0's legacy wall post with a photo, found by its first words (never an id written by hand). */
const THAN_BAI_ANH = "Buổi sáng cuối cùng ở hồ, cả nhóm ngồi đợi sương tan rồi mới chịu về. Ảnh tổng hợp cho lượt kiểm thử.";
const baiAnh = async () => ((await goi("GET", `/social/v2/people/${ID0}/posts?limit=50`, undefined, P0)).json?.posts ?? []).find((b) => b.body === THAN_BAI_ANH) ?? null;

/** The journey page as drawn: route tabs, choice cards, badge rows, cover line, fresh-badge block, alerts. */
const docSo = (page) =>
  page.evaluate((src) => {
    const lyDo = (0, eval)(src);
    const hien = (e) => e && e.getClientRects().length > 0 && e.checkVisibility({ opacityProperty: true });
    const chu = (e) => (e?.innerText ?? "").replace(/[-]/g, "").replace(/\s+/g, " ").trim();
    const man = [...document.querySelectorAll('[data-testid="achievements-screen"]')].filter(hien).pop();
    if (!man) return null;
    const hop = (e) => { const r = e.getBoundingClientRect(); return { y: Math.round(r.top), w: Math.round(r.width), h: Math.round(r.height) }; };
    const tabs = [...man.querySelectorAll('[role="tab"]')].filter(hien).map((t) => ({ ten: t.getAttribute("aria-label") || chu(t), chon: t.getAttribute("aria-selected"), ...hop(t) }));
    const nutChon = [...man.querySelectorAll('[role="button"]')].filter((b) => hien(b) && /^(Chọn hướng này|Nhận kết này|Đang theo hướng này|Đã ghi vào sổ)$/.test(chu(b)));
    const the = nutChon.map((b) => {
      let c = b;
      for (let i = 0; i < 8 && c.parentElement && !/Mẫu sáng tạo/.test(chu(c)); i++) c = c.parentElement;
      return { nut: chu(b), tat: b.getAttribute("aria-disabled") === "true", lyDo: lyDo(b), ...hop(b), chu: chu(c).slice(0, 320) };
    });
    const huyHieu = [...man.querySelectorAll('[role="checkbox"]')].filter(hien).map((b) => ({ ten: b.getAttribute("aria-label"), checked: b.getAttribute("aria-checked"), ...hop(b) }));
    const t = chu(man);
    const bia = t.match(/(\d+) kết đã mở · (\d+) lượt dựng MP4 còn dùng được/) ?? [];
    const loi = [...man.querySelectorAll('[role="alert"]')].filter(hien).map((e) => ({ chu: chu(e), ...hop(e) }));
    return { chu: t, tabs, the, huyHieu, moi: /MỚI MỞ/.test(t) ? (t.match(/MỚI MỞ (.{0,40})/) ?? [])[1] ?? "" : null, ketMo: bia[1] ? Number(bia[1]) : null, mp4: bia[2] ? Number(bia[2]) : null, loi, h: innerHeight };
  }, LY_DO_SRC);
const tenThe = (the) => TEN_KET.find((ten) => the.chu.includes(ten)) ?? "?";
/** The action button of the choice card holding `tieuDe`, hit-tested like a tap. */
const nutTrongThe = (page, tieuDe, nhan) =>
  page.evaluate(
    ({ tieuDe, nhan }) => {
      const chu = (e) => (e?.innerText ?? "").replace(/[-]/g, "").replace(/\s+/g, " ").trim();
      for (const b of document.querySelectorAll('[role="button"]')) {
        if (!b.getClientRects().length || chu(b) !== nhan) continue;
        let c = b;
        for (let i = 0; i < 8 && c.parentElement && !chu(c).includes(tieuDe); i++) c = c.parentElement;
        if (!chu(c).includes(tieuDe) || chu(c).length > 600) continue;
        b.scrollIntoView({ block: "center", behavior: "instant" });
        const r = b.getBoundingClientRect();
        const x = r.left + r.width / 2;
        const y = r.top + r.height / 2;
        const tren = document.elementFromPoint(x, y);
        if (tren && (tren === b || b.contains(tren))) return { x, y, w: Math.round(r.width), h: Math.round(r.height), tat: b.getAttribute("aria-disabled") === "true" };
      }
      return null;
    },
    { tieuDe, nhan },
  );
const moSo = async (cfg, ten = "chat-0") => {
  const t = await mo(cfg, ten, "/achievements");
  const cdp = await cdpCua(t.page);
  await t.page.waitForTimeout(2500);
  await choOn(t.page, { mang: t.mang });
  return { t, cdp };
};
const soTom = (s) => (s ? `mở ${s.ketMo ?? "?"} kết, ${s.mp4 ?? "?"} lượt MP4; tab ${s.tabs.map((x) => `«${x.ten}» aria-selected=${x.chon}`).join(", ")}; thẻ ${s.the.map((x) => `«${tenThe(x)}» «${x.nut}»${x.tat ? ` tắt, lý do ${x.lyDo ? `«${x.lyDo}»` : "không"}` : ""}`).join("; ")}; huy hiệu ${s.huyHieu.map((x) => `«${x.ten}» aria-checked=${x.checked}`).join(", ") || "không"}` : "không thấy màn");
/** Wall cards on a person page: one «Mở bài: …» button per post, and the load-more button. */
const docTuong = (page) =>
  page.evaluate(() => {
    const hien = (e) => e && e.getClientRects().length > 0;
    const man = [...document.querySelectorAll('[data-testid="ho-so-nguoi-screen"]')].filter(hien).pop();
    if (!man) return null;
    const chu = (e) => (e?.innerText ?? "").replace(/[-]/g, "").replace(/\s+/g, " ").trim();
    const bai = [...man.querySelectorAll('[role="button"]')].filter((b) => hien(b) && (b.getAttribute("aria-label") ?? "").startsWith("Mở bài: "));
    const anh = [...man.querySelectorAll('[aria-label="Mở ảnh và bình luận"]')].filter(hien).length;
    const them = [...man.querySelectorAll('[role="button"]')].some((b) => hien(b) && chu(b) === "Xem những trang trước");
    const laCuon = (d) => /(auto|scroll)/.test(getComputedStyle(d).overflowY) && d.scrollHeight > d.clientHeight + 4;
    let cuon = [man, ...man.querySelectorAll("div")].find(laCuon) ?? null;
    return { soBai: bai.length, anh, them, cuon: cuon ? Math.round(cuon.scrollTop) : null, chu: chu(man) };
  });

try {
  // ------------------------------------------------------------------ api
  // Read only: each journey book, what other viewers receive, the relations that decide it.
  if (chay("api")) {
    const dong = [];
    for (const ten of ["chat-0", "chat-1", "chat-2"]) {
      const s = await soCua(ten);
      dong.push(`${ten}: đã đạt [${(s?.earned_badges ?? []).map((b) => b.id + (b.displayed ? "*" : "")).join(", ")}], lượt MP4 ${s?.mp4_credits?.available ?? "?"}, xem trước cho Nếp ${JSON.stringify(s?.suggestion_preview ?? null)}`);
    }
    const nguoiXem = [];
    let lo = false;
    for (const ten of ["chat-1", "chat-16", NGOAI]) {
      const p = await phienAi(ten);
      const a = await goi("GET", `/people/${ID0}/achievements`, undefined, p);
      const h = await goi("GET", `/people/${ID0}`, undefined, p);
      const w = await goi("GET", `/social/v2/people/${ID0}/posts?limit=20`, undefined, p);
      const khoa = a.json ? Object.keys(a.json).sort().join(",") : a.code;
      const khoaHuyHieu = [...new Set((a.json?.badges ?? []).flatMap((b) => Object.keys(b)))].sort().join(",");
      if (a.status === 200 && a.json && (Object.keys(a.json).some((k) => !["badges", "person_id"].includes(k)) || /progress|credit|candidate|requirement/.test(khoaHuyHieu))) lo = true;
      nguoiXem.push(`${ten}: hồ sơ ${h.status} (${h.json?.relation ?? h.code}), huy hiệu ${a.status} khoá [${khoa}] ${a.json?.badges?.length ?? "-"} chiếc${khoaHuyHieu ? ` (khoá mỗi chiếc ${khoaHuyHieu})` : ""}, tường ${w.status} ${w.json?.posts?.length ?? w.code} bài`);
    }
    const p16 = await phienCua("chat-16");
    const q = await goi("GET", `/people/${(await phienCua("chat-1")).person_id}`, undefined, p16);
    ghi({ tc: "TC-N21-API", screen: "N21.S02", state: "sổ hành trình của chat-0, chat-1, chat-2; hồ sơ chat-0 qua mắt người khác", action: "GET /me/achievement-routes; GET /people/[id]/achievements, /people/[id], /social/v2/people/[id]/posts", cauHinh: "-", expected: "người khác chỉ nhận huy hiệu chủ hồ sơ chọn trưng bày; không có tiến độ, nhánh hay lượt MP4 (03-profile-story-social.md)", status: lo ? "FAIL" : "PASS", ghiChu: `ngày ${ngayVN()} (giờ Việt Nam); ${dong.join(" | ")} | ${nguoiXem.join(" | ")} | chat-16 xem chat-1: ${q.json?.relation ?? q.code}` });
  }

  // ----------------------------------------------------------- hanh-trinh
  // The journey book of chat-0 before any write: baseline sizes, the route tabs, the choice cards.
  if (chay("hanh-trinh")) {
    for (const cfg of ["C1", "C2", "C3", "C6"]) {
      if (!chayPhan("hanh-trinh", cfg)) continue;
      const { t, cdp } = await moSo(cfg);
      const s0 = await docSo(t.page);
      const cap = await anh(t.page, `EV-N21-HT-${cfg}`, t);
      await cuonToi(t.page, 900, "achievements-screen");
      await t.page.waitForTimeout(300);
      const cap2 = await anh(t.page, `EV-N21-HT-DUOI-${cfg}`, t);
      const tt = [cap?.tomTat ?? {}, cap2?.tomTat ?? {}];
      const soX = (x) => `tràn trang ${x.tranTrangPx ?? "?"}px, chữ bị cắt ${x.chuBiCat ?? "?"}, ellipsis ${x.ellipsis ?? "?"}, vùng bấm < 48: ${x.vungBamNho48 ?? "?"} (< 44: ${x.vungBamNho44 ?? "?"})`;
      ghi({ tc: "TC-N21-HT-BASE", screen: "N21.S02", state: "chat-0, sổ hành trình: 3 huy hiệu mở đầu, chưa có kết, chưa chọn lối", action: "mở /achievements; chụp đầu màn và phần dưới", cauHinh: cfg, expected: "bìa, bản đồ bốn tuyến, lối đang chọn và các thẻ ngã rẽ hiển thị đủ; không tràn, không cắt chữ", status: s0 && tt.every((x) => x.tranTrangPx === 0 && x.chuBiCat === 0) ? "PASS" : "FAIL", evidence: [`EV-N21-HT-${cfg}`, `EV-N21-HT-DUOI-${cfg}`], ghiChu: `${soTom(s0)}; đầu: ${soX(tt[0])}; dưới: ${soX(tt[1])}` });
      if (cfg === "C1") {
        ghi({ tc: "TC-N21-HT-TAB-ARIA", screen: "N21.S02", state: "chat-0, bản đồ bốn tuyến", action: "đọc trạng thái trợ năng của bốn nút tuyến (role tab)", cauHinh: "C1", expected: "tab đang chọn có aria-selected=true, các tab khác false", status: s0 && s0.tabs.length >= 4 && s0.tabs.every((x) => x.chon === "true" || x.chon === "false") && s0.tabs.filter((x) => x.chon === "true").length === 1 ? "PASS" : "FAIL", ghiChu: s0 ? s0.tabs.map((x) => `«${x.ten}» aria-selected=${x.chon} ${x.w}×${x.h}`).join(", ") : "không thấy màn" });
        const mau = (s0?.the ?? []).map((x) => ({ ten: tenThe(x), mau: (x.chu.match(/Mẫu sáng tạo[^:]*:[^·]*?(?=Chọn hướng|Nhận kết|Đang theo|Đã ghi|$)/) ?? [""])[0].trim() }));
        ghi({ tc: "TC-N21-HT-MAU-MP4", screen: "N21.S02", state: "chat-0, lối «Dấu chân» (mặc định)", action: "đọc dòng phần thưởng của từng thẻ ngã rẽ", cauHinh: "C1", expected: "mẫu sáng tạo chưa nối vào bộ dựng ghi «sắp dùng được», không quảng cáo như quyền đã dùng được (03-profile-story-social.md)", status: mau.length && mau.every((m) => /sắp dùng được/.test(m.mau)) ? "PASS" : "FAIL", evidence: ["EV-N21-HT-C1"], ghiChu: mau.map((m) => `«${m.ten}»: «${m.mau}»`).join("; ") || "không có thẻ" });
        // Each route in turn (local state only): the crossroads is empty until an ending exists.
        const theoTuyen = [];
        for (const tuyen of ["Kỷ niệm", "Đồng hành", "Ngã rẽ", "Dấu chân"]) {
          const p = await nut(t.page, tuyen, { batDau: true });
          if (p) await cham(cdp, p);
          await t.page.waitForTimeout(500);
          const s = await docSo(t.page);
          theoTuyen.push(`${tuyen}: ${s?.the.map((x) => `«${tenThe(x)}» «${x.nut}»`).join(", ") || (/Ngã rẽ đang chờ/.test(s?.chu ?? "") ? "«Ngã rẽ đang chờ»" : "không thẻ")}`);
          if (tuyen === "Ngã rẽ") await anh(t.page, "EV-N21-HT-NGA-RE-C1", t);
        }
        ghi({ tc: "TC-N21-HT-TUYEN", screen: "N21.S02", state: "chat-0, chưa có kết nào", action: "chạm lần lượt bốn tuyến trên bản đồ", cauHinh: "C1", expected: "mỗi tuyến hiện hai ngã rẽ của nó với tiến độ; Ngã rẽ nói cần một kết trước (không có thẻ chọn được)", status: theoTuyen.length === 4 && /Ngã rẽ: «Ngã rẽ đang chờ»/.test(theoTuyen.join(" | ")) ? "PASS" : "FAIL", evidence: ["EV-N21-HT-NGA-RE-C1"], ghiChu: theoTuyen.join(" | ") });
        const ax = await chayAxe(t.page).catch(() => null);
        ghi({ tc: "TC-N21-HT-AXE", screen: "N21.S02", state: "chat-0, sổ hành trình", action: "chạy axe", cauHinh: "C1", expected: "không lỗi critical/serious", status: ax && !ax.vi.some((v) => ["critical", "serious"].includes(v.impact)) ? "PASS" : "FAIL", ghiChu: ax ? ax.vi.map((v) => `${v.rule} (${v.impact}) ×${v.so}: ${v.vd.map((x) => x.target).join(" | ")}`).join("; ") || "sạch" : "axe không chạy" });
      }
      await t.context.close();
    }
  }

  // --------------------------------------------------------------- ca-nhan
  // The two ways in from the tab Cá nhân: the journey teaser and the menu row «Thành tích».
  if (chay("ca-nhan")) {
    for (const cfg of ["C1", "C2"]) {
      if (!chayPhan("ca-nhan", cfg)) continue;
      const t = await mo(cfg, "chat-0", "/profile");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(3000);
      await choOn(t.page, { mang: t.mang });
      const vao = await t.page.evaluate(() => {
        const nut = [...document.querySelectorAll('[role="button"]')].filter((b) => b.getClientRects().length);
        const teaser = nut.find((b) => /Mở sổ hành trình$/.test(b.getAttribute("aria-label") ?? ""));
        const menu = nut.find((b) => /^Thành tích/.test((b.innerText ?? "").replace(/[\uE000-\uF8FF]/g, "").trim()));
        const chu = (e) => (e?.innerText ?? "").replace(/[-]/g, "").replace(/\s+/g, " ").trim();
        if (teaser) teaser.scrollIntoView({ block: "center", behavior: "instant" });
        const r = teaser?.getBoundingClientRect();
        return { teaser: teaser ? { ten: teaser.getAttribute("aria-label"), chu: chu(teaser), rect: { x: r.left, y: r.top, w: r.width, h: r.height } } : null, menu: menu ? chu(menu) : null };
      });
      await anh(t.page, `EV-N21-CA-NHAN-${cfg}`, t, vao.teaser ? { chuThich: [{ rect: vao.teaser.rect }] } : {});
      let den = null;
      let tieuDe = null;
      if (cfg === "C1" && vao.teaser) {
        await bam(t, cdp, vao.teaser.ten, { cho: 2500 });
        den = await duongDan(t.page);
        tieuDe = await t.page.evaluate(() => ([...document.querySelectorAll('[data-testid="achievements-screen"]')].filter((e) => e.getClientRects().length).pop()?.innerText ?? "").replace(/[\uE000-\uF8FF]/g, "").trim().split("\n")[0] ?? null);
      }
      ghi({ tc: "TC-N21-CA-NHAN", screen: "N21.S01", state: "chat-0, tab Cá nhân; sổ hành trình có 3 huy hiệu mở đầu", action: "đọc thẻ Hành trình và mục «Thành tích»; chạm thẻ", cauHinh: cfg, expected: "thẻ nói đúng tiến độ (huy hiệu, lượt MP4) và dẫn tới sổ hành trình; tên gọi của lối vào khớp tên màn tới", status: vao.teaser && (cfg !== "C1" || /\/achievements$/.test(den ?? "")) ? "PASS" : "FAIL", evidence: [`EV-N21-CA-NHAN-${cfg}`], ghiChu: `thẻ «${vao.teaser?.chu.slice(0, 200) ?? "không thấy"}»; mục menu «${vao.menu ?? "không thấy"}»${den ? `; chạm thẻ: tới ${den}, tiêu đề màn «${tieuDe ?? "-"}»` : ""}` });
      await t.context.close();
    }
  }

  // --------------------------------------------------------------- moi-mo
  // M8, «MỚI MỞ»: which badge a fresh device (a new browser context, empty storage) presents as new.
  if (chay("moi-mo")) {
    const { t } = await moSo("C1");
    const s1 = await docSo(t.page);
    const dat = (await soCua("chat-0"))?.earned_badges ?? [];
    await anh(t.page, "EV-N21-MOI-MO-C1", t);
    await t.page.reload();
    await t.page.waitForTimeout(2500);
    await choOn(t.page, { mang: t.mang });
    const s2 = await docSo(t.page);
    const cuNhat = dat.slice().sort((a, b) => a.earned_at.localeCompare(b.earned_at))[0];
    const moiNhat = dat.slice().sort((a, b) => b.earned_at.localeCompare(a.earned_at))[0];
    ghi({ tc: "TC-N21-MOI-MO", screen: "N21.S02", state: `chat-0 trên một máy chưa từng mở sổ (context mới); đã đạt ${dat.map((b) => b.id).join(", ")}, huy hiệu mới nhất đạt ${moiNhat?.earned_at?.slice(0, 10) ?? "-"}`, action: "mở /achievements; tải lại", cauHinh: "C1", expected: "khoảnh khắc «MỚI MỞ» chỉ dành cho huy hiệu vừa đạt kể từ lần xem trước; máy mới không trình bày một huy hiệu cũ như vừa mở", status: s1?.moi ? "FAIL" : "PASS", evidence: ["EV-N21-MOI-MO-C1"], ghiChu: `lần đầu: ${s1?.moi !== null && s1?.moi !== undefined ? `«MỚI MỞ ${s1.moi}»` : "không có khối MỚI MỞ"}; huy hiệu đạt sớm nhất ${cuNhat?.id ?? "-"} (${cuNhat?.earned_at?.slice(0, 10) ?? "-"}); sau khi tải lại: ${s2?.moi !== null && s2?.moi !== undefined ? `«MỚI MỞ ${s2.moi}»` : "không có khối MỚI MỞ"}` });
    await t.context.close();
  }

  // ------------------------------------------------------------------ nep
  // Nếp's suggestion: the preview says what would be sent; consent is asked for each call.
  if (chay("nep")) {
    const { t, cdp } = await moSo("C1");
    const gui = [];
    t.page.on("request", (r) => { if (/\/me\/achievement-suggestions$/.test(new URL(r.url()).pathname)) gui.push(r.postData()); });
    const xem = await bam(t, cdp, "Xem Nếp sẽ nhận gì", { cho: 700 });
    const trangThai = await t.page.evaluate(() => {
      const b = [...document.querySelectorAll('[role="button"]')].find((x) => x.getClientRects().length && /Đóng bản xem trước|Xem Nếp sẽ nhận gì/.test(x.innerText ?? ""));
      return b ? { expanded: b.getAttribute("aria-expanded"), chu: (b.innerText ?? "").trim() } : null;
    });
    const s = await docSo(t.page);
    const xemTruoc = (s?.chu.match(/Chỉ gửi số đếm và mã ngã rẽ.{0,420}/) ?? [""])[0];
    await anh(t.page, "EV-N21-NEP-XEM-C1", t);
    const dy = await bam(t, cdp, "Đồng ý hỏi Nếp lần này", { cho: 3500 });
    const s2 = await docSo(t.page);
    await anh(t.page, "EV-N21-NEP-C1", t);
    ghi({ tc: "TC-N21-NEP-XEM-TRUOC", screen: "N21.S02", layer: "L-nep-goi-y", state: "chat-0, sổ hành trình", action: "«Xem Nếp sẽ nhận gì»", cauHinh: "C1", expected: "bản xem trước nói đúng thứ sẽ gửi: chỉ số đếm và mã ngã rẽ; không ảnh, bài, bạn đồng hành, chat, vị trí; mỗi lần hỏi cần đồng ý lại", status: xem && /Chỉ gửi số đếm/.test(xemTruoc) && /Không gửi ảnh/.test(xemTruoc) && /Mỗi lần hỏi cần bạn đồng ý lại/.test(xemTruoc) ? "PASS" : "FAIL", evidence: ["EV-N21-NEP-XEM-C1"], ghiChu: `«${xemTruoc.slice(0, 380)}»` });
    ghi({ tc: "TC-N21-NEP-ARIA", screen: "N21.S02", layer: "L-nep-goi-y", state: "chat-0, bản xem trước đang mở", action: "đọc trạng thái trợ năng của nút mở bản xem trước", cauHinh: "C1", expected: "nút mở/đóng có aria-expanded đúng trạng thái", status: trangThai?.expanded === "true" ? "PASS" : "FAIL", ghiChu: `nút «${trangThai?.chu ?? "-"}» aria-expanded=${trangThai?.expanded ?? "-"}` });
    ghi({ tc: "TC-N21-NEP-DONG-Y", screen: "N21.S02", layer: "L-nep-goi-y", state: "chat-0; máy chủ không có khoá AI", action: "«Đồng ý hỏi Nếp lần này»", cauHinh: "C1", expected: "yêu cầu chỉ mang lời đồng ý; không có AI thì sổ tự gợi ý và nói rõ Nếp đang vắng; gợi ý dẫn tới ngã rẽ", status: dy && gui.length === 1 && /^\{"consent":true\}$/.test(gui[0] ?? "") && /Nếp đang vắng/.test(s2?.chu ?? "") ? "PASS" : "FAIL", evidence: ["EV-N21-NEP-C1"], ghiChu: `thân yêu cầu ${gui.map((x) => x ?? "rỗng").join(" | ") || "không gửi"}; sau: «${((s2?.chu ?? "").match(/Nếp nhìn đường đi.{0,300}/) ?? [""])[0].slice(0, 300)}»` });
    await t.context.close();
  }

  // ------------------------------------------------------------------ loi
  // Server errors: reading the book; choosing a route (the error sentence and where it lands).
  if (chay("loi")) {
    {
      const t = await mo("C1", "chat-0", "/messages");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(1500);
      let lan = 0;
      await t.page.route(/\/me\/achievement-routes$/, (route) => (lan++ === 0 ? route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ code: "achievement_unavailable" }) }) : route.continue()));
      await diToi(t, "/achievements");
      const truoc = await t.page.evaluate(() => (document.querySelector('[data-testid="achievements-screen"]')?.innerText ?? "").replace(/\s+/g, " ").trim().slice(0, 200));
      await anh(t.page, "EV-N21-LOI-DOC-C1", t);
      const thu = await bam(t, cdp, "Thử lại", { cho: 2500 });
      const sau = await docSo(t.page);
      ghi({ tc: "TC-N21-LOI-DOC", screen: "N21.S02", state: "chat-0; GET /me/achievement-routes trả 503 lần đầu", action: "mở sổ hành trình; «Thử lại» khi máy chủ đã lành", cauHinh: "C1", expected: "câu lỗi nói sổ chưa mở được, không đổ cho mạng; «Thử lại» mở được sổ", status: /Chưa mở được sổ hành trình/.test(truoc) && thu && sau?.tabs.length ? "PASS" : "FAIL", evidence: ["EV-N21-LOI-DOC-C1"], ghiChu: `lúc lỗi «${truoc}»; «Thử lại»: ${thu ? "chạm" : "không thấy"}; sau: ${sau?.tabs.length ? "sổ mở" : "chưa mở"}` });
      await t.context.close();
    }
    {
      const { t, cdp } = await moSo("C1");
      await loiMayChu(t.page, /\/me\/achievement-runs$/);
      const p = await nut(t.page, "Kỷ niệm", { batDau: true });
      if (p) await cham(cdp, p);
      await t.page.waitForTimeout(500);
      const n = await nutTrongThe(t.page, "Những tấm ảnh còn đây", "Chọn hướng này");
      if (n) await cham(cdp, n);
      await t.page.waitForTimeout(1500);
      const s = await docSo(t.page);
      await anh(t.page, "EV-N21-LOI-CHON-C1", t);
      const l = s?.loi[0] ?? null;
      const trong = l && l.y >= 0 && l.y + l.h <= s.h;
      ghi({ tc: "TC-N21-LOI-CHON", screen: "N21.S02", state: "chat-0, lối Kỷ niệm; POST /me/achievement-runs trả 503", action: "«Chọn hướng này» ở «Những tấm ảnh còn đây»", cauHinh: "C1", expected: "câu lỗi hiện gần nút vừa chạm, trong khung nhìn; không có gì được ghi", status: n && l && trong ? "PASS" : "FAIL", evidence: ["EV-N21-LOI-CHON-C1"], ghiChu: `nút ${n ? `ở y ${Math.round(n.y)} lúc chạm` : "không thấy"}; câu lỗi ${l ? `«${l.chu}» ở y ${l.y}–${l.y + l.h} (cửa sổ ${s.h})` : "không thấy"}; chưa chọn lối nào: ${(await soCua("chat-0"))?.active_run ? "không, đã có lối" : "đúng"}` });
      await t.context.close();
    }
  }

  // -------------------------------------------------------------- bai-anh
  // chat-0's legacy «Bạn bè» wall post with a synthetic photo (the reader of /posts/[id] opens
  // legacy posts; the composer of this stack writes community posts). A story of at least 40
  // characters: on a Vietnam date after 30/09 it is chat-0's third story day.
  if (chay("bai-anh")) {
    const ngay = ngayVN();
    let b = await baiAnh();
    const truoc = await soCua("chat-0");
    let taiLen = null;
    let dang = null;
    if (!b && ngay <= "2026-09-30") console.log(`bai-anh: hôm nay ${ngay} giờ Việt Nam; chờ sau 0 giờ 01/10 để bài là ngày kể thứ ba`);
    else if (!b) {
      const fd = new FormData();
      fd.append("file", new Blob([pngThuBytes(640, 480)], { type: "image/png" }), "anh.png");
      const r = await fetch(`${mt.api}/people/me/photos`, { method: "POST", headers: { authorization: `Bearer ${P0.token}`, "idempotency-key": randomUUID() }, body: fd });
      taiLen = { status: r.status, json: await r.json().catch(() => null) };
      const url = taiLen.json?.url ?? taiLen.json?.image_url ?? null;
      if (url) dang = await goi("POST", "/posts", { body: THAN_BAI_ANH, audience: "friends", image_url: url }, P0);
      b = await baiAnh();
    }
    if (b || taiLen) {
      const sau = await soCua("chat-0");
      ghi({ tc: "TC-N21-BAI-ANH", screen: "N21.S04", state: `chat-0, ngày ${ngay} (giờ Việt Nam)`, action: "tải một ảnh tổng hợp 640×480 (POST /people/me/photos), đăng bài «Bạn bè» cũ có ảnh (POST /posts)", cauHinh: "-", expected: "bài có ảnh trên tường chat-0; là bài kể đủ dài ở một ngày mới, nên số ngày có bài kể tăng một", status: b?.image_url && (sau?.suggestion_preview?.story_days ?? 0) === (truoc?.suggestion_preview?.story_days ?? 0) + (taiLen ? 1 : 0) ? "PASS" : "FAIL", ghiChu: `tải ảnh ${taiLen ? taiLen.status : "đã có"}; đăng ${dang ? dang.status : "đã có"}; bài ${b ? `[id], ảnh ${b.image_url ? "có" : "không"}, ${b.audience}` : "không thấy"}; ngày có bài kể ${truoc?.suggestion_preview?.story_days ?? "?"} → ${sau?.suggestion_preview?.story_days ?? "?"}` });
    }
  }

  // ------------------------------------------------------------------ ket
  // Choose «Chuyện mình kể» on the Kỷ niệm route, then claim it; the MP4 credit; the M8 moment.
  if (chay("ket")) {
    const { t, cdp } = await moSo("C1");
    await docSo(t.page); // this context's first look: every badge earned so far is now «seen»
    const tab = async () => {
      const p = await nut(t.page, "Kỷ niệm", { batDau: true });
      if (p) await cham(cdp, p);
      await t.page.waitForTimeout(600);
      return docSo(t.page);
    };
    let s = await tab();
    const the = () => s?.the.find((x) => x.chu.includes(KET)) ?? null;
    const truocChon = the()?.nut ?? null;
    if (truocChon === "Chọn hướng này") {
      const n = await nutTrongThe(t.page, KET, "Chọn hướng này");
      if (n) await cham(cdp, n);
      await t.page.waitForTimeout(2500);
      s = await tab();
    }
    await anh(t.page, "EV-N21-KET-CHON-C1", t);
    const sauChon = the();
    ghi({ tc: "TC-N21-CHON", screen: "N21.S02", state: `chat-0, lối Kỷ niệm, «${KET}» (ngày có bài kể ${(await soCua("chat-0"))?.suggestion_preview?.story_days ?? "?"}/3)`, action: "«Chọn hướng này»", cauHinh: "C1", expected: "thẻ thành «Nhận kết này» khi đủ dấu mốc, hoặc «Đang theo hướng này» tắt kèm lý do (dấu mốc còn thiếu)", status: sauChon && (sauChon.nut === "Nhận kết này" || (sauChon.nut === "Đang theo hướng này" && sauChon.tat && sauChon.lyDo)) ? "PASS" : "FAIL", evidence: ["EV-N21-KET-CHON-C1"], ghiChu: `trước «${truocChon ?? "-"}»; sau «${sauChon?.nut ?? "-"}»${sauChon?.tat ? `, tắt, lý do «${sauChon.lyDo ?? "-"}»` : ""}; ${soTom(s)}` });
    let nhan = null;
    if (sauChon?.nut === "Nhận kết này") {
      nhan = await nutTrongThe(t.page, KET, "Nhận kết này");
      if (nhan) await cham(cdp, nhan);
      await t.page.waitForTimeout(3000);
      s = await tab();
    }
    await cuonToi(t.page, 0, "achievements-screen");
    await t.page.waitForTimeout(300);
    await anh(t.page, "EV-N21-KET-NHAN-C1", t);
    const so = await soCua("chat-0");
    const daDat = (so?.earned_badges ?? []).some((b) => b.id === "storyteller");
    const theSau = the();
    ghi({ tc: "TC-N21-NHAN-KET", screen: "N21.S02", state: `chat-0, «${KET}» đủ dấu mốc`, action: "«Nhận kết này»", cauHinh: "C1", expected: "kết ghi vào sổ; thẻ «Đã ghi vào sổ» tắt kèm lý do; bìa đếm 1 kết và số lượt MP4 bằng số máy chủ cấp; mẫu vẫn «sắp dùng được»", status: nhan && daDat && theSau?.nut === "Đã ghi vào sổ" && theSau.tat && theSau.lyDo && s?.ketMo === 1 && s?.mp4 === (so?.mp4_credits?.available ?? -1) && /sắp dùng được/.test(theSau.chu) ? "PASS" : "FAIL", evidence: ["EV-N21-KET-NHAN-C1"], ghiChu: `nút ${nhan ? "chạm" : "không có"}; máy chủ: đạt storyteller ${daDat ? "có" : "không"}, lượt MP4 ${JSON.stringify(so?.mp4_credits ?? null)}; màn: bìa ${s?.ketMo ?? "?"} kết, ${s?.mp4 ?? "?"} lượt; thẻ «${theSau?.nut ?? "-"}»${theSau?.tat ? `, lý do «${theSau.lyDo ?? "-"}»` : ""}` });
    await t.page.reload();
    await t.page.waitForTimeout(2500);
    await choOn(t.page, { mang: t.mang });
    const s2 = await docSo(t.page);
    await anh(t.page, "EV-N21-KET-M8-C1", t);
    ghi({ tc: "TC-N21-M8", screen: "N21.S02", state: "chat-0, cùng máy vừa nhận kết", action: "mở lại sổ hành trình", cauHinh: "C1", expected: "huy hiệu vừa đạt có khoảnh khắc «MỚI MỞ» một lần", status: /Chuyện mình kể/.test(s2?.moi ?? "") ? "PASS" : "FAIL", evidence: ["EV-N21-KET-M8-C1"], ghiChu: `khối: ${s2?.moi !== null && s2?.moi !== undefined ? `«MỚI MỞ ${s2.moi}»` : "không có"}` });
    await t.context.close();
  }

  // ------------------------------------------------------------ trung-bay
  // Display three badges on the profile, then tap a fourth.
  if (chay("trung-bay")) {
    const { t, cdp } = await moSo("C1");
    const patch = [];
    t.page.on("request", (r) => { if (/\/me\/achievement-display$/.test(new URL(r.url()).pathname)) patch.push(r.postData()); });
    let s = await docSo(t.page);
    for (let i = 0; i < 4 && s && s.huyHieu.filter((h) => /đang trưng bày/.test(h.ten ?? "")).length < 3; i++) {
      const h = s.huyHieu.find((x) => /chưa trưng bày/.test(x.ten ?? ""));
      if (!h) break;
      await bam(t, cdp, h.ten, { cho: 1800 });
      s = await docSo(t.page);
    }
    const ba = s?.huyHieu.filter((h) => /đang trưng bày/.test(h.ten ?? "")) ?? [];
    const may = ((await soCua("chat-0"))?.earned_badges ?? []).filter((b) => b.displayed).map((b) => b.id);
    ghi({ tc: "TC-N21-TRUNG-BAY", screen: "N21.S02", state: "chat-0, 4 huy hiệu đã đạt, chưa trưng bày chiếc nào", action: "chạm ba hàng trong «Dấu ấn đã giữ»", cauHinh: "C1", expected: "ba huy hiệu thành «Trên hồ sơ»; máy chủ lưu đúng ba", status: ba.length === 3 && may.length === 3 ? "PASS" : "FAIL", ghiChu: `màn: ${s?.huyHieu.map((h) => `«${h.ten}» aria-checked=${h.checked}`).join(", ") ?? "-"}; máy chủ trưng bày ${may.join(", ") || "không"}` });
    const thu4 = s?.huyHieu.find((x) => /chưa trưng bày/.test(x.ten ?? "")) ?? null;
    const truoc = patch.length;
    let sau4 = null;
    if (thu4) {
      await bam(t, cdp, thu4.ten, { cho: 2000 });
      sau4 = await docSo(t.page);
    }
    await anh(t.page, "EV-N21-TRUNG-BAY-4-C1", t);
    const cau = (sau4?.chu.match(/[^.]*(tối đa|ba huy hiệu|bỏ bớt|đủ ba)[^.]*\./i) ?? [null])[0];
    ghi({ tc: "TC-N21-TRUNG-BAY-4", screen: "N21.S02", state: "chat-0, đã trưng bày ba huy hiệu", action: `chạm huy hiệu thứ tư «${thu4?.ten ?? "-"}»`, cauHinh: "C1", expected: "chạm thứ tư nói vì sao không được (đã đủ ba; bỏ một chiếc trước), hoặc hàng đó tắt kèm lý do (ADR-0038)", status: thu4 && sau4 && sau4.loi.length ? "PASS" : "FAIL", evidence: ["EV-N21-TRUNG-BAY-4-C1"], ghiChu: `yêu cầu PATCH gửi thêm ${patch.length - truoc} lần (thân ${patch.slice(truoc).join(" | ") || "-"}); hàng thứ tư sau khi chạm «${sau4?.huyHieu.find((h) => h.ten?.startsWith((thu4?.ten ?? "").split(",")[0]))?.ten ?? "-"}»; câu báo: ${sau4?.loi.map((l) => `«${l.chu}»`).join(", ") || "không"}; câu dưới tiêu đề: «${cau ?? "-"}»` });
    await t.context.close();
  }

  // ------------------------------------------------------------ xem-nguoi
  // chat-0's profile read by a friend (three sizes), a groupmate, a stranger.
  if (chay("xem-nguoi")) {
    const may = ((await soCua("chat-0"))?.earned_badges ?? []).filter((b) => b.displayed).map((b) => b.id);
    for (const [ten, cfg] of [["chat-1", "C1"], ["chat-1", "C2"], ["chat-1", "C3"], ["chat-16", "C1"], [NGOAI, "C1"]]) {
      if (!chayPhan("xem-nguoi", `${ten}:${cfg}`) && !chayPhan("xem-nguoi", "tat-ca")) continue;
      const t = await mo(cfg, ten, `/people/${ID0}`);
      await t.page.waitForTimeout(3000);
      await choOn(t.page, { mang: t.mang });
      const k = await t.page.evaluate((ten) => {
        const man = [...document.querySelectorAll('[data-testid="ho-so-nguoi-screen"]')].filter((e) => e.getClientRects().length).pop();
        const chu = (man?.innerText ?? "").replace(/[-]/g, "").replace(/\s+/g, " ").trim();
        const khoi = (chu.match(/Dấu ấn chọn giữ trên bìa sổ(.{0,260})/) ?? [null, null])[1];
        return { chu, khoi, rieng: ["lượt dựng MP4", "kết đã mở", "Chọn lối đi", "Ngày có bài kể"].filter((w) => chu.includes(w)) };
      }, ten);
      const cap = await anh(t.page, `EV-N21-XEM-${ten.toUpperCase()}-${cfg}`, t);
      const thay = TEN_KET.filter((x) => (k.khoi ?? "").includes(x));
      const tt = cap?.tomTat ?? {};
      const la = ten === NGOAI;
      if (ten === "chat-1") ghi({ tc: "TC-N21.S03-BASE", screen: "N21.S03", state: "chat-1 (bạn) mở hồ sơ chat-0; chat-0 trưng bày ba huy hiệu", action: "mở /people/[id]", cauHinh: cfg, expected: "hồ sơ, dấu ấn trưng bày, kệ sổ và trang viết hiển thị đủ; không tràn, không cắt chữ", status: k.chu && tt.tranTrangPx === 0 && tt.chuBiCat === 0 ? "PASS" : "FAIL", evidence: [`EV-N21-XEM-${ten.toUpperCase()}-${cfg}`], ghiChu: `dấu ấn: ${thay.join(", ") || "không"}; tràn trang ${tt.tranTrangPx ?? "?"}px, chữ bị cắt ${tt.chuBiCat ?? "?"}, vùng bấm < 48: ${tt.vungBamNho48 ?? "?"}` });
      if (ten === "chat-1" && cfg === "C1") {
        // A community post with a photo (B2 of N14) shown as a card on the personal wall.
        const pv = await phienCua("chat-1");
        const tuongB = (await goi("GET", `/social/v2/people/${ID0}/posts?limit=50`, undefined, pv)).json?.posts ?? [];
        const b2 = tuongB.find((x) => /^Chiều hôm đó cả nhóm đi bộ/.test(x.body)) ?? null;
        const cd = b2 ? (await goi("GET", `/v2/community/posts/${b2.id}`, undefined, pv)).json : null;
        const soAnhCd = (cd?.media ?? cd?.photos ?? []).length;
        const the = b2 ? await t.page.evaluate((dau) => {
          const e = [...document.querySelectorAll('[role="button"]')].find((x) => x.getClientRects().length && (x.getAttribute("aria-label") ?? "").startsWith(`Mở bài: ${dau}`));
          let c = e;
          for (let i = 0; i < 3 && c?.parentElement; i++) c = c.parentElement;
          c?.scrollIntoView({ block: "center", behavior: "instant" });
          return c ? { anh: c.querySelectorAll('[aria-label="Mở ảnh và bình luận"], img').length } : null;
        }, b2.body.slice(0, 20)) : null;
        await t.page.waitForTimeout(400);
        await anh(t.page, "EV-N21-TUONG-B2-C1", t);
        ghi({ tc: "TC-N21-TUONG-ANH-CD", screen: "N21.S03", state: "chat-1 mở tường chat-0; bài Cộng đồng B2 (công khai, một ảnh 640×480) nằm trên tường", action: "đọc thẻ của B2 trên tường", cauHinh: "C1", expected: "thẻ trên tường hiện ảnh của bài, như chi tiết bài Cộng đồng", status: b2 && soAnhCd > 0 ? (the && the.anh > 0 ? "PASS" : "FAIL") : "BLOCKED", evidence: ["EV-N21-TUONG-B2-C1"], ghiChu: `tường: B2 ${b2 ? `có, image_url ${b2.image_url ? "có" : "null"}` : "không thấy"}; chi tiết Cộng đồng: ${soAnhCd} ảnh; thẻ trên màn: ${the ? `${the.anh} ảnh` : "không thấy"}` });
      }
      if (cfg === "C1") ghi({ tc: `TC-N21-XEM-HUY-HIEU-${la ? "LA" : ten === "chat-16" ? "NHOM" : "BAN"}`, screen: "N21.S03", state: `${ten} (${la ? "người lạ" : ten === "chat-16" ? "cùng nhóm, không là bạn" : "bạn"}) mở hồ sơ chat-0; máy chủ trưng bày ${may.length} chiếc`, action: "đọc khối «Dấu ấn chọn giữ trên bìa sổ»", cauHinh: "C1", expected: la ? "người lạ không thấy huy hiệu; không thấy tiến độ" : `thấy đúng ${may.length} huy hiệu chủ hồ sơ chọn; không thấy tiến độ, nhánh, lượt MP4`, status: la ? (!thay.length && !k.rieng.length ? "PASS" : "FAIL") : (thay.length === may.length && !k.rieng.length ? "PASS" : "FAIL"), evidence: [`EV-N21-XEM-${ten.toUpperCase()}-C1`], ghiChu: `thấy: ${thay.join(", ") || "không"}; chữ riêng tư lộ ra: ${k.rieng.join(", ") || "không"}; đầu hồ sơ «${k.chu.slice(0, 120)}»` });
      await t.context.close();
    }
  }

  // ------------------------------------------------------------ binh-luan
  // On chat-0's photo post: chat-1 comments, chat-0 replies, chat-1 answers the reply (one level
  // only: it must hang under the parent), chat-1 likes chat-0's reply; Q3, a way to delete.
  if (chay("binh-luan")) {
    const b = await baiAnh();
    if (!b) console.log("binh-luan: chưa có bài ảnh; chạy --chi bai-anh trước");
    else {
      const P1 = await phienCua("chat-1");
      const CAU1 = "Ảnh này đẹp quá, lần sau rủ mình đi sớm hơn nhé.";
      const CAU2 = "Hẹn lần sau đi sớm, mình gọi cả nhóm dậy.";
      const CAU3 = "Nhớ mang áo khoác, sáng sớm ở hồ lạnh lắm.";
      const binhLuan = async () => (await goi("GET", `/social/v2/posts/${b.id}/comments?limit=50`, undefined, P0)).json?.comments ?? [];
      const tim = (ds, cau) => { for (const c of ds) { if (c.body === cau) return c; const r = (c.replies ?? []).find((x) => x.body === cau); if (r) return r; } return null; };
      const gui = async (t, cdp, cau) => {
        const ok = await go(t, cdp, "Viết bình luận", cau);
        const n = ok ? await bam(t, cdp, "Gửi bình luận", { cho: 2000 }) : null;
        return !!(ok && n);
      };
      // chat-1: a parent comment.
      const t1 = await mo("C1", "chat-1", `/posts/${b.id}`);
      const cdp1 = await cdpCua(t1.page);
      await t1.page.waitForTimeout(3000);
      await choOn(t1.page, { mang: t1.mang });
      const docCu = await t1.page.evaluate(() => !!document.querySelector('[data-testid="bai-chi-tiet-screen"]'));
      let ds = await binhLuan();
      if (!tim(ds, CAU1)) await gui(t1, cdp1, CAU1);
      ds = await binhLuan();
      const c1 = tim(ds, CAU1);
      await anh(t1.page, "EV-N21-BL-GUI-C1", t1);
      ghi({ tc: "TC-N21-BL-GUI", screen: "N21.S04", state: "chat-1 (bạn) mở bài «Bạn bè» có ảnh của chat-0 (trình đọc bài cũ)", action: "gõ bình luận, «Gửi bình luận»", cauHinh: "C1", expected: "bình luận hiện dưới bài, là bình luận gốc (không cha)", status: docCu && c1 && c1.parent_id === null ? "PASS" : "FAIL", evidence: ["EV-N21-BL-GUI-C1"], ghiChu: `màn bài cũ: ${docCu ? "có" : "không"}; bình luận ${c1 ? `có, cha ${c1.parent_id ?? "không"}` : "không thấy"}` });
      // chat-0: reply to chat-1's comment.
      const t0 = await mo("C1", "chat-0", `/posts/${b.id}`);
      const cdp0 = await cdpCua(t0.page);
      await t0.page.waitForTimeout(3000);
      await choOn(t0.page, { mang: t0.mang });
      if (!tim(ds, CAU2)) {
        await bam(t0, cdp0, `Trả lời ${tenHien("chat-1")}`, { cho: 700 });
        await gui(t0, cdp0, CAU2);
      }
      ds = await binhLuan();
      const c2 = tim(ds, CAU2);
      await anh(t0.page, "EV-N21-BL-TRA-LOI-C1", t0);
      ghi({ tc: "TC-N21-BL-TRA-LOI", screen: "N21.S04", state: "chat-0 mở bài của mình, có bình luận của chat-1", action: `«Trả lời ${tenHien("chat-1")}», gõ, gửi`, cauHinh: "C1", expected: "lời đáp nằm dưới bình luận của chat-1 (một tầng)", status: c1 && c2 && c2.parent_id === c1.id ? "PASS" : "FAIL", evidence: ["EV-N21-BL-TRA-LOI-C1"], ghiChu: `lời đáp ${c2 ? `cha ${c2.parent_id === c1?.id ? "= bình luận của chat-1" : c2.parent_id ?? "không"}` : "không thấy"}` });
      await t0.context.close();
      // chat-1: answer chat-0's reply; the answer hangs under the same parent.
      await t1.page.reload();
      await t1.page.waitForTimeout(3000);
      await choOn(t1.page, { mang: t1.mang });
      if (!tim(ds, CAU3)) {
        await bam(t1, cdp1, `Trả lời ${tenHien("chat-0")}`, { cho: 700 });
        const dangTraLoi = await t1.page.evaluate(() => [...document.querySelectorAll("input,textarea")].find((x) => x.getAttribute("aria-label") === "Viết bình luận")?.getAttribute("placeholder") ?? null);
        await gui(t1, cdp1, CAU3);
        console.log(`binh-luan: ô gõ khi trả lời lời đáp: placeholder «${dangTraLoi}»`);
      }
      ds = await binhLuan();
      const c3 = tim(ds, CAU3);
      await anh(t1.page, "EV-N21-BL-MOT-TANG-C1", t1);
      ghi({ tc: "TC-N21-BL-MOT-TANG", screen: "N21.S04", state: "chat-1, bài có bình luận của chat-1 và lời đáp của chat-0", action: `«Trả lời ${tenHien("chat-0")}» trên lời đáp, gõ, gửi`, cauHinh: "C1", expected: "chỉ một tầng trả lời: câu mới nằm dưới bình luận gốc của chat-1, không lồng dưới lời đáp", status: c1 && c3 && c3.parent_id === c1.id ? "PASS" : "FAIL", evidence: ["EV-N21-BL-MOT-TANG-C1"], ghiChu: `câu mới ${c3 ? `cha ${c3.parent_id === c1?.id ? "= bình luận gốc của chat-1" : c3.parent_id === c2?.id ? "= lời đáp của chat-0" : c3.parent_id ?? "không"}` : "không thấy"}; số lời đáp dưới bình luận gốc ${(ds.find((c) => c.id === c1?.id)?.replies ?? []).length}` });
      // chat-1: like chat-0's reply.
      if (c2 && !c2.liked) {
        const p = await t1.page.evaluate((cau) => {
          for (const b of document.querySelectorAll('[role="button"]')) {
            if (!b.getClientRects().length || !/^(Thích|Bỏ thích) bình luận$/.test(b.getAttribute("aria-label") ?? "")) continue;
            let c = b;
            for (let i = 0; i < 5 && c.parentElement && !(c.innerText ?? "").includes(cau); i++) c = c.parentElement;
            if (!(c.innerText ?? "").includes(cau) || (c.innerText ?? "").length > 300) continue;
            b.scrollIntoView({ block: "center", behavior: "instant" });
            const r = b.getBoundingClientRect();
            return { x: r.left + r.width / 2, y: r.top + r.height / 2, w: Math.round(r.width), h: Math.round(r.height), ten: b.getAttribute("aria-label"), pressed: b.getAttribute("aria-pressed") };
          }
          return null;
        }, CAU2);
        if (p) await cham(cdp1, p);
        await t1.page.waitForTimeout(1500);
        const sau = (await goi("GET", `/social/v2/posts/${b.id}/comments?limit=50`, undefined, P1)).json?.comments ?? [];
        const c2s = tim(sau, CAU2);
        const nut2 = await t1.page.evaluate(() => [...document.querySelectorAll('[role="button"]')].filter((b) => b.getClientRects().length && /bình luận$/.test(b.getAttribute("aria-label") ?? "") && /thích/i.test(b.getAttribute("aria-label") ?? "")).map((b) => `${b.getAttribute("aria-label")} aria-pressed=${b.getAttribute("aria-pressed")} ${Math.round(b.getBoundingClientRect().width)}×${Math.round(b.getBoundingClientRect().height)}`));
        ghi({ tc: "TC-N21-BL-THICH", screen: "N21.S04", state: "chat-1, lời đáp của chat-0", action: "chạm «Thích bình luận» trên lời đáp", cauHinh: "C1", expected: "lượt thích lên 1; trạng thái đã thích nói bằng lời hoặc aria; nút đủ lớn để chạm", status: p && c2s?.liked && c2s.like_count === 1 ? "PASS" : "FAIL", ghiChu: `nút ${p ? `«${p.ten}» ${p.w}×${p.h}` : "không thấy"}; máy chủ: liked ${c2s?.liked}, ${c2s?.like_count ?? "?"} lượt; các nút thích sau khi chạm: ${nut2.join(", ")}` });
      }
      // Q3: can the writer take their own comment back?
      const q3 = await t1.page.evaluate((cau) => {
        for (const e of document.querySelectorAll('div[dir="auto"]')) {
          if ((e.innerText ?? "").trim() !== cau) continue;
          let c = e;
          for (let i = 0; i < 4 && c.parentElement; i++) c = c.parentElement;
          const nut = [...c.querySelectorAll('[role="button"]')].filter((b) => b.getClientRects().length).map((b) => b.getAttribute("aria-label") || (b.innerText ?? "").trim());
          const r = e.getBoundingClientRect();
          return { nut, x: r.left + r.width / 2, y: r.top + r.height / 2 };
        }
        return null;
      }, CAU1);
      let sauGiu = null;
      if (q3) {
        await nhanGiu(cdp1, q3, 700);
        await t1.page.waitForTimeout(800);
        sauGiu = await dialogTren(t1.page);
      }
      ghi({ tc: "TC-N21-Q3-XOA-BL", screen: "N21.S04", state: "chat-1, bình luận của chính mình dưới bài của chat-0", action: "tìm lối xoá bình luận: nút trên hàng, nhấn giữ bình luận", cauHinh: "C1", expected: "người viết xoá được bình luận của mình (có bước hỏi); API DELETE /social/v2/comments/{id} vẫn còn", status: q3 && (q3.nut.some((x) => /xo[aá]/i.test(x ?? "")) || /xo[aá]/i.test(sauGiu?.chu ?? "")) ? "PASS" : "FAIL", ghiChu: `nút trên hàng bình luận: ${q3?.nut.join(", ") || "-"}; nhấn giữ: ${sauGiu ? `mở «${sauGiu.ten}»` : "không mở gì"}` });
      await t1.context.close();
    }
  }

  // -------------------------------------------------------- anh-toan-man
  // The photo full screen and its comment tray, opened from the wall card (?photo=1).
  if (chay("anh-toan-man")) {
    const b = await baiAnh();
    if (!b) console.log("anh-toan-man: chưa có bài ảnh; chạy --chi bai-anh trước");
    else
      for (const cfg of ["C1", "C8"]) {
        if (!chayPhan("anh-toan-man", cfg)) continue;
        const t = await mo(cfg, "chat-1", `/people/${ID0}`);
        const cdp = await cdpCua(t.page);
        await t.page.waitForTimeout(3000);
        await choOn(t.page, { mang: t.mang });
        const moAnh = await bam(t, cdp, "Mở ảnh và bình luận", { cho: 3500 });
        await choOn(t.page, { mang: t.mang });
        const v = await t.page.evaluate(() => {
          const d = [...document.querySelectorAll('[role="dialog"],[aria-modal="true"]')].filter((x) => x.getClientRects().length);
          const img = [...document.querySelectorAll("img")].filter((i) => i.getClientRects().length).map((i) => { const r = i.getBoundingClientRect(); return { alt: i.getAttribute("alt"), w: Math.round(r.width), h: Math.round(r.height), nw: i.naturalWidth }; }).filter((i) => i.nw > 0).sort((a, b) => b.w * b.h - a.w * a.h)[0] ?? null;
          const dong = [...document.querySelectorAll('[aria-label="Đóng ảnh"]')].filter((x) => x.getClientRects().length).map((x) => { const r = x.getBoundingClientRect(); return `${Math.round(r.width)}×${Math.round(r.height)}`; });
          return { dialog: d.map((x) => `${x.getAttribute("role") ?? "-"}/${x.getAttribute("aria-label") ?? "-"}`), img, dong, url: location.pathname + location.search };
        });
        const khay = await dialogTren(t.page);
        const f = await focusHienTai(t.page);
        await anh(t.page, `EV-N21-ANH-${cfg}`, t);
        ghi({ tc: "TC-N21-ANH-MO", screen: "N21.S04", layer: "L-anh-toan-man", state: "chat-1 ở tường của chat-0, bài «Bạn bè» có ảnh", action: "chạm ảnh trên thẻ bài («Mở ảnh và bình luận»)", cauHinh: cfg, expected: "ảnh toàn màn hiện ảnh thật (cao > 0, đã tải) cùng khay bình luận; lớp phủ là dialog có tên, tiêu điểm vào trong; nút «Đóng ảnh» ≥ 44", status: moAnh && v.img && v.img.h > 100 && khay?.ten && f.trongDialog ? "PASS" : "FAIL", evidence: [`EV-N21-ANH-${cfg}`], ghiChu: `tới ${anId(v.url)}; lớp: ${v.dialog.join(", ") || "không có dialog"}; ảnh ${v.img ? `${v.img.w}×${v.img.h} (tự nhiên rộng ${v.img.nw})` : "không có ảnh đã tải"}; «Đóng ảnh» ${v.dong.join(", ") || "không thấy"}; khay trên cùng «${khay?.ten ?? "-"}» ${khay ? `${khay.rong}×${khay.cao} ở y ${khay.top}` : ""}; tiêu điểm «${f.ten}» (trong dialog ${f.trongDialog})` });
        if (cfg === "C1") {
          const truoc = ((await goi("GET", `/social/v2/posts/${b.id}/comments?limit=50`, undefined, P0)).json?.comments ?? []).length;
          const CAU4 = "Gửi từ khay của ảnh: tấm này nên in ra treo tường.";
          const daCo = ((await goi("GET", `/social/v2/posts/${b.id}/comments?limit=50`, undefined, P0)).json?.comments ?? []).some((c) => c.body === CAU4);
          let gui = null;
          if (!daCo) {
            const ok = await go(t, cdp, "Viết bình luận", CAU4);
            gui = ok ? await bam(t, cdp, "Gửi bình luận", { cho: 2000 }) : null;
          }
          const sau = (await goi("GET", `/social/v2/posts/${b.id}/comments?limit=50`, undefined, P0)).json?.comments ?? [];
          const c4 = sau.find((c) => c.body === CAU4) ?? null;
          ghi({ tc: "TC-N21-ANH-GUI", screen: "N21.S04", layer: "L-anh-toan-man", state: "chat-1, ảnh toàn màn, khay bình luận mở", action: "gõ trong khay, gửi", cauHinh: "C1", expected: "lời gửi từ khay vào đúng chuỗi bình luận của bài", status: c4 ? "PASS" : "FAIL", ghiChu: `trước ${truoc} bình luận gốc; gửi: ${daCo ? "đã có từ lượt trước" : gui ? "chạm" : "không gửi được"}; sau: ${c4 ? `có, cha ${c4.parent_id ?? "không (gốc)"}` : "không thấy"}` });
          await t.page.keyboard.press("Escape");
          await t.page.waitForTimeout(900);
          const sauEsc = await dialogTren(t.page);
          const lan2 = sauEsc ? (await t.page.keyboard.press("Escape"), await t.page.waitForTimeout(900), await dialogTren(t.page)) : null;
          const dongBang = sauEsc && lan2 ? await bam(t, cdp, "Đóng ảnh", { cho: 900 }) : null;
          const cuoi = await dialogTren(t.page);
          ghi({ tc: "TC-N21-ANH-DONG", screen: "N21.S04", layer: "L-anh-toan-man", state: "chat-1, ảnh toàn màn với khay bình luận", action: "Esc; Esc lần nữa; «Đóng ảnh» nếu còn", cauHinh: "C1", expected: "Esc đóng khay rồi đóng ảnh; về trang bài", status: !lan2 && !cuoi ? "PASS" : "FAIL", ghiChu: `sau Esc 1: ${sauEsc ? `còn «${sauEsc.ten}»` : "đóng hết"}; sau Esc 2: ${lan2 ? `còn «${lan2.ten}»` : sauEsc ? "đóng" : "-"}; «Đóng ảnh»: ${dongBang ? "chạm" : "không cần"}; cuối: ${cuoi ? `còn «${cuoi.ten}»` : "không còn lớp phủ"}; đường dẫn ${anId(await duongDan(t.page))}` });
        }
        await t.context.close();
      }
  }

  // ------------------------------------------------------------- dang-lai
  // chat-1 reposts chat-0's «Bạn bè» photo post; chat-17 (a friend of chat-1 made for this, a
  // groupmate of chat-0 but not its friend) sees the repost without the original's content.
  if (chay("dang-lai")) {
    const b = await baiAnh();
    if (!b) console.log("dang-lai: chưa có bài ảnh; chạy --chi bai-anh trước");
    else {
      const P1 = await phienCua("chat-1");
      const P17 = await phienCua("chat-17");
      const quanHe = async () => (await goi("GET", `/people/${P1.person_id}`, undefined, P17)).json?.relation ?? null;
      let qh = await quanHe();
      if (qh !== "friend" && chayPhan("dang-lai", "ket-ban")) {
        await goi("POST", "/friends/requests", { addressee_id: P17.person_id }, P1);
        const den = (await goi("GET", `/people/${P17.person_id}/friend-requests?direction=incoming`, undefined, P17)).json;
        const lm = (den?.requests ?? den?.friend_requests ?? []).find((x) => x.state === "pending");
        if (lm) await goi("POST", `/friends/requests/${lm.id}/respond`, { decision: "accept" }, P17);
        qh = await quanHe();
      }
      const tuong1 = async (p) => (await goi("GET", `/social/v2/people/${P1.person_id}/posts?limit=50`, undefined, p)).json?.posts ?? [];
      const t = await mo("C1", "chat-1", `/posts/${b.id}`);
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(3000);
      await choOn(t.page, { mang: t.mang });
      const daDang = (await tuong1(P1)).find((x) => x.is_repost && x.origin?.id === b.id) ?? null;
      const mo1 = await bam(t, cdp, "Chia sẻ bài", { cho: 1200 });
      const sheet = await dialogTren(t.page);
      await anh(t.page, "EV-N21-CHIA-SE-C1", t);
      ghi({ tc: "TC-N21-CHIA-SE", screen: "N21.S04", layer: "L-chia-se-bai", state: "chat-1, bài «Bạn bè» của chat-0", action: "«Chia sẻ bài»", cauHinh: "C1", expected: "khay nói ai thấy bài đăng lại và bài gốc chỉ hiện cho người có quyền; «Gửi vào chat» tắt, kèm câu nói chờ chat mã hoá đầu cuối (03-profile-story-social.md)", status: mo1 && sheet && sheet.nut.some((n) => n.ten === "Gửi vào chat" && n.tat) && /mã hoá đầu cuối/.test(sheet.chu) && /chỉ hiện cho người đã có quyền/.test(sheet.chu) ? "PASS" : "FAIL", evidence: ["EV-N21-CHIA-SE-C1"], ghiChu: sheet ? `«${sheet.ten}» ${sheet.rong}×${sheet.cao} ở y ${sheet.top}; «${sheet.chu.slice(0, 260)}»; nút ${sheet.nut.map((n) => `${n.ten}${n.tat ? " (tắt)" : ""}`).join(", ")}` : "không mở" });
      let thongBao = null;
      if (!daDang) {
        await bam(t, cdp, "Bạn bè của tôi", { cho: 2500 });
        thongBao = await t.page.evaluate(() => {
          const e = [...document.querySelectorAll('div[dir="auto"]')].find((x) => x.getClientRects().length && (x.innerText ?? "").trim() === "Đã chia sẻ lên tường của bạn.");
          if (!e) return null;
          const r = e.getBoundingClientRect();
          return { top: Math.round(r.top), day: Math.round(r.bottom), h: innerHeight };
        });
      }
      await anh(t.page, "EV-N21-DANG-LAI-C1", t);
      await t.context.close();
      const dang = (await tuong1(P1)).find((x) => x.is_repost && x.origin?.id === b.id) ?? null;
      ghi({ tc: "TC-N21-DANG-LAI", screen: "N21.S04", layer: "L-chia-se-bai", state: "chat-1, khay «Chia sẻ bài» mở", action: "«Bạn bè của tôi»", cauHinh: "C1", expected: "bài đăng lại lên tường chat-1 (bạn bè), mang trích bài gốc; câu xác nhận nằm trong khung nhìn", status: dang && (daDang || (thongBao && thongBao.top >= 0 && thongBao.day <= thongBao.h)) ? "PASS" : "FAIL", evidence: ["EV-N21-DANG-LAI-C1"], ghiChu: `${daDang ? "đã đăng từ lượt trước" : `câu xác nhận ${thongBao ? `ở y ${thongBao.top}–${thongBao.day} (cửa sổ ${thongBao.h})` : "không thấy"}`}; tường chat-1: bài đăng lại ${dang ? `có, ${dang.audience}, trích gốc «${(dang.origin?.body ?? "-").slice(0, 40)}»` : "không có"}` });
      if (dang) {
        const cua17 = (await tuong1(P17)).find((x) => x.id === dang.id) ?? null;
        const t17 = await mo("C1", "chat-17", `/people/${P1.person_id}`);
        await t17.page.waitForTimeout(3000);
        await choOn(t17.page, { mang: t17.mang });
        const w = await docTuong(t17.page);
        await anh(t17.page, "EV-N21-DANG-LAI-ACL-C1", t17);
        await t17.context.close();
        const lo = (w?.chu ?? "").includes(THAN_BAI_ANH.slice(0, 30)) || !!cua17?.origin?.body;
        ghi({ tc: "TC-N21-DANG-LAI-ACL", screen: "N21.S03", state: `chat-17: ${qh === "friend" ? "bạn của chat-1" : `quan hệ với chat-1: ${qh}`}, cùng nhóm với chat-0 nhưng không là bạn`, action: "mở tường chat-1, đọc thẻ bài đăng lại", cauHinh: "C1", expected: "thấy bài đăng lại nhưng không thấy chữ hay ảnh của bài gốc «Bạn bè» (ACL kiểm trên trích đoạn bài gốc)", status: qh === "friend" && cua17 && !lo ? "PASS" : qh === "friend" ? "FAIL" : "BLOCKED", evidence: ["EV-N21-DANG-LAI-ACL-C1"], ghiChu: `API: bài đăng lại ${cua17 ? `có, trích gốc ${cua17.origin ? `«${(cua17.origin.body ?? "").slice(0, 40)}»` : "null"}` : "không thấy"}; màn: ${w ? `${w.soBai} thẻ; chữ bài gốc ${lo ? "LỘ" : "không lộ"}; «${((w.chu.match(/CHIA SẺ.{0,160}/) ?? [""])[0])}»` : "không thấy màn"}` });
      }
    }
  }

  // ---------------------------------------------------------------- trang
  // More than one page on chat-0's wall: 15 short «Chỉ mình tôi» posts (under 40 characters, so no
  // story day). chat-0 loads page two; chat-1 likes a post; does the long poll keep page two, how
  // fast does the like show, and does the page keep its content while the poll fails?
  if (chay("trang")) {
    const THU = (i) => `Ghi chú trang thử ${String(i).padStart(2, "0")}`;
    const docTatCa = async () => (await goi("GET", `/social/v2/people/${ID0}/posts?limit=50`, undefined, P0)).json?.posts ?? [];
    let viet = 0;
    if (chayPhan("trang", "ghi")) {
      const hien = await docTatCa();
      for (let i = 1; i <= 15; i++) {
        if (hien.some((x) => x.body === THU(i))) continue;
        await goi("POST", "/posts", { body: THU(i), audience: "only_me" }, P0);
        viet++;
      }
    }
    const tong = (await docTatCa()).length;
    const b = await baiAnh();
    const P1 = await phienCua("chat-1");
    const t = await mo("C1", "chat-0", `/people/${ID0}`);
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(3500);
    await choOn(t.page, { mang: t.mang });
    const w0 = await docTuong(t.page);
    const them = await bam(t, cdp, "Xem những trang trước", { cho: 3000 });
    const w1 = await docTuong(t.page);
    await anh(t.page, "EV-N21-TRANG-2-C1", t);
    ghi({ tc: "TC-N21-TRANG", screen: "N21.S03", state: `chat-0, tường của mình có ${tong} bài (15 bài ngắn «Chỉ mình tôi» ghi cho phép đo, ${viet} bài ghi ở lượt này)`, action: "mở /people/[id] của mình; «Xem những trang trước»", cauHinh: "C1", expected: "trang đầu 20 bài; «Xem những trang trước» nối trang sau, đủ mọi bài", status: w0?.soBai === 20 && them && w1?.soBai === tong ? "PASS" : "FAIL", evidence: ["EV-N21-TRANG-2-C1"], ghiChu: `trang đầu ${w0?.soBai ?? "?"} thẻ, nút tải thêm ${w0?.them ? "có" : "không"}; sau khi chạm ${w1?.soBai ?? "?"} thẻ, nút ${w1?.them ? "còn" : "hết"}` });
    // The event: chat-1 likes (or unlikes) chat-0's photo post.
    const likeCua = () => t.page.evaluate((cau) => {
      const man = [...document.querySelectorAll('[data-testid="ho-so-nguoi-screen"]')].filter((e) => e.getClientRects().length).pop();
      const e = [...(man?.querySelectorAll('[role="button"]') ?? [])].find((x) => (x.getAttribute("aria-label") ?? "").startsWith("Mở bài: ") && (x.getAttribute("aria-label") ?? "").includes(cau));
      let c = e;
      for (let i = 0; i < 4 && c?.parentElement && !/\d+ thích · \d+ bình luận/.test(c.innerText ?? ""); i++) c = c.parentElement;
      return (c?.innerText ?? "").match(/(\d+) thích · (\d+) bình luận/)?.[1] ?? null;
    }, THAN_BAI_ANH.slice(0, 30));
    const likeTruoc = await likeCua();
    const daThich = b ? (await goi("GET", `/social/v2/posts/${b.id}`, undefined, P1)).json?.liked : null;
    const t0 = Date.now();
    const su = b ? await goi(daThich ? "DELETE" : "PUT", `/social/v2/posts/${b.id}/like`, undefined, P1) : null;
    const mau = [];
    for (let i = 0; i < 52; i++) {
      await t.page.waitForTimeout(500);
      const w = await docTuong(t.page);
      mau.push({ ms: Date.now() - t0, n: w?.soBai ?? null, like: await likeCua() });
    }
    await anh(t.page, "EV-N21-TRANG-SAU-SU-KIEN-C1", t);
    const doiLike = mau.find((m) => m.like !== likeTruoc) ?? null;
    const giam = mau.find((m) => m.n !== null && w1?.soBai && m.n < w1.soBai) ?? null;
    ghi({ tc: "TC-N21-DOI", screen: "N21.S03", state: "chat-0 đang xem tường của mình; chat-1 thích/bỏ thích bài ảnh", action: `${daThich ? "DELETE" : "PUT"} /social/v2/posts/[id]/like từ chat-1; theo dõi thẻ 26 s`, cauHinh: "C1", expected: "lượt thích mới hiện trên thẻ trong vài giây (long poll, LISTEN đánh thức sớm)", status: su && su.status < 300 && doiLike && doiLike.ms <= 5000 ? "PASS" : "FAIL", ghiChu: `sự kiện ${su?.status ?? "-"}; thích trên thẻ ${likeTruoc ?? "?"} → ${doiLike ? `${doiLike.like} sau ${doiLike.ms} ms` : "không đổi trong 26 s"}` });
    ghi({ tc: "TC-N21-TRANG-GIU", screen: "N21.S03", state: `chat-0 đã mở trang sau (${w1?.soBai ?? "?"} thẻ); một sự kiện tới qua long poll`, action: "đứng yên 26 s sau sự kiện", cauHinh: "C1", expected: "làm mới nền giữ những trang đã mở và vị trí đọc (03-profile-story-social.md: làm mới nền khi có sự kiện)", status: w1?.soBai > 20 && !giam ? "PASS" : "FAIL", evidence: ["EV-N21-TRANG-2-C1", "EV-N21-TRANG-SAU-SU-KIEN-C1"], ghiChu: `số thẻ theo thời gian: ${mau.filter((m, i) => i === 0 || m.n !== mau[i - 1].n).map((m) => `${m.n}@${m.ms}ms`).join(" → ")}${giam ? `; về ${giam.n} thẻ sau ${giam.ms} ms` : ""}; vị trí cuộn cuối ${(await docTuong(t.page))?.cuon ?? "-"}` });
    // The poll fails for a while: the page keeps what it shows.
    await loiMayChu(t.page, /\/social\/v2\/people\/[^/]+\/(changes|posts)/);
    const wTruoc = await docTuong(t.page);
    await t.page.waitForTimeout(25_000);
    const wLoi = await docTuong(t.page);
    await anh(t.page, "EV-N21-DOI-LOI-C1", t);
    ghi({ tc: "TC-N21-DOI-LOI", screen: "N21.S03", state: "chat-0 đang xem tường của mình; /changes và /posts trả 503", action: "đứng yên 25 s", cauHinh: "C1", expected: "giữ nội dung đang hiện khi mạng tạm lỗi (03-profile-story-social.md)", status: wLoi?.soBai && wLoi.soBai >= (wTruoc?.soBai ?? 0) && !/Chưa đọc được tường/.test(wLoi.chu) ? "PASS" : "FAIL", evidence: ["EV-N21-DOI-LOI-C1"], ghiChu: `trước ${wTruoc?.soBai ?? "?"} thẻ; sau 25 s lỗi ${wLoi?.soBai ?? "?"} thẻ; khối lỗi ${/Chưa đọc được tường/.test(wLoi?.chu ?? "") ? "có" : "không"}` });
    await t.context.close();
  }

  // ---------------------------------------------------------- khong-phien
  if (chay("khong-phien")) {
    const t = await mo("C1", null, "/achievements");
    await t.page.waitForTimeout(3000);
    await choOn(t.page, { mang: t.mang });
    const chu = (await chuTrang(t.page)).slice(0, 220);
    const den = await duongDan(t.page);
    await anh(t.page, "EV-N21-KHONG-PHIEN-C1", t);
    ghi({ tc: "TC-N21-KHONG-PHIEN", screen: "N21.S02", state: "không phiên", action: "mở /achievements bằng link", cauHinh: "C1", expected: "không lộ dữ liệu của ai; hoặc có lối đăng nhập, hoặc bản demo mang nhãn «Dữ liệu demo»", status: /Dữ liệu demo|Đăng nhập/.test(chu) ? "PASS" : "FAIL", evidence: ["EV-N21-KHONG-PHIEN-C1"], ghiChu: `tới ${den}; «${chu}»` });
    await t.context.close();
  }
} finally {
  await mt.dong();
}
