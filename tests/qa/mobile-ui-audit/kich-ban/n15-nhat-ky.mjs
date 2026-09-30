/* Audit of a feature new on main 461eabf: memory books, «Nếp v3» (task #15,
 * ADR-0039, #657). The organiser of an outing closes it («Khép cuộc đi»); then
 * each member keeps a book of their own on their wall: choose the photos and an
 * excerpt, build it with Nếp (AI) or lay the pages out by hand, edit, keep it
 * private or public, send a public one to the community, delete it. Also
 * observation Q4: the album's «đã chia» adds up every expense of the group
 * whose date falls inside the outing's days.
 *
 *   source audit-env-main.sh && node kich-ban/n15-nhat-ky.mjs [--chi api,vao,…]
 *
 * Order matters: `khep-truoc` measures «Kèo album retest» before it is closed;
 * `khep` closes it for good (an outing's ending is not undone). Within `vao`,
 * `vao:keo` is the outing screens (meaningful only before `khep`) and
 * `vao:<persona>:<config>` one wall shelf; `hep` is the editor at C2 and C3,
 * read only.
 *
 * Personas (chat-test seed on stack 2, read over the API before any write):
 *   chat-0   organiser of «Kèo album retest» (29–30/09); keeps a trip book
 *   chat-1   member; keeps a book, then deletes it from a cold link
 *   chat-15  operator (community_moderators since N14): approves the shared book
 *   dalat-0  outside the chat-test group: may read a public book, never a private one
 * Every write checks what exists first, so a rerun writes nothing twice. Rows
 * are TC-N15-*; the placeholder TC-N-15-NHAT-KY is withdrawn by the verdict script.
 */
import { cauHinh } from "../thu-vien/cau-hinh.mjs";
import { chup, ghepAnh } from "../thu-vien/chup.mjs";
import { cdpCua, cham, tamCua } from "../thu-vien/cu-chi.mjs";
import { choOn, duongDan } from "../thu-vien/dieu-huong.mjs";
import { focusHienTai, inertConLai } from "../thu-vien/lop-phu.mjs";
import { chayAxe } from "../thu-vien/axe.mjs";
import { goHet, loiMayChu, ngatMang } from "../thu-vien/mang.mjs";
import { khoiDong, trangMoi } from "../thu-vien/moi-truong.mjs";
import { layPhien, personaTheoTen } from "../thu-vien/phien.mjs";
import { soGhi } from "../thu-vien/ghi.mjs";
import { pngThuBytes } from "../../../../apps/mobile/tools/png-thu.mjs";
import { randomUUID } from "node:crypto";
import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
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
  // Bare ISO dates side by side read, to the repo guard, as a phone number: quote each one (see n15-phan-xu.mjs).
  if (rec.ghiChu) rec = { ...rec, ghiChu: rec.ghiChu.replace(/(?<!«)\b(\d{4}-\d{2}-\d{2})\b(?!»)/g, "«$1»") };
  so.ghi({ feature: "N15", nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, evidence: [], ...rec });
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


// The outings and the photos of the chat-test group, found by name (never an id written by hand).
const KEO = "Kèo album retest"; // 29–30/09, organiser chat-0: closed by `khep`
const KEO_SAU = "Kèo retest 2 ngày chưa có chặng"; // 24–25/10: not started
const KEO_TRUNG = "Kèo trùng ngày kiểm tổng"; // created by `q4` on 29/09, the day of the group's one expense
const NGOAI = "dalat-0"; // outside the chat-test group
const PUA = /[-]/g;
const nhom = async () => {
  const p = await phienCua("chat-0");
  for (const c of (await goi("GET", "/people/me/contexts", undefined, p)).json?.contexts ?? []) {
    if (c.kind !== "group") continue;
    const ks = (await goi("GET", `/contexts/${c.id}/outings`, undefined, p)).json?.outings ?? [];
    if (ks.some((k) => k.title === KEO)) return { id: c.id, keo: ks };
  }
  throw new Error(`không thấy nhóm có «${KEO}» của chat-0`);
};
const G = await nhom();
const keo = (ten) => G.keo.find((k) => k.title === ten) ?? null;
const O1 = keo(KEO);
const O2 = keo(KEO_SAU);
if (!O1 || !O2) throw new Error("thiếu kèo của nhóm chat-test; chạy retest-main r-f08 và r-f03 trước");
const phienNgoai = () => layPhien(mt.api, personaTheoTen(NGOAI, mt.chatSessions), phienDir);
const sachCua = async (ten) => {
  const p = ten === NGOAI ? await phienNgoai() : await phienCua(ten);
  const r = await goi("GET", `/outings/${O1.id}/ending`, undefined, p);
  return r.json?.diary_id ?? null;
};

/**
 * One diary screen as drawn: the words, every control with the state a screen
 * reader gets, the error box and where it sits, the scroll position, and every
 * photo (loaded or not, and from what kind of address).
 */
const doMan = (page, testid = "diary-ending-screen") =>
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
/** Scroll the diary screen's own scroller (or the window) to y. */
const cuonToi = (page, y, testid = "diary-ending-screen") =>
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
/** Transform samples of the book as it turns (the Animated.View of the editor), one per change. */
const batDauMau = (page) =>
  page.evaluate(() => {
    const w = window;
    w.__mau = [];
    w.__dung = false;
    const t0 = performance.now();
    const buoc = () => {
      if (w.__dung) return;
      const man = document.querySelector('[data-testid="diary-ending-screen"]');
      let tf = null;
      if (man) for (const e of man.querySelectorAll("div")) if (/translateY/.test(e.style.transform) && /scale/.test(e.style.transform)) { tf = e.style.transform; break; }
      const cuoi = w.__mau.at(-1);
      if (!cuoi || cuoi.tf !== tf) w.__mau.push({ t: Math.round(performance.now() - t0), tf });
      requestAnimationFrame(buoc);
    };
    requestAnimationFrame(buoc);
  });
const dungMau = (page) =>
  page.evaluate(() => {
    window.__dung = true;
    return window.__mau ?? [];
  });
const tomMau = (m) => m.map((x) => `${x.tf ? x.tf.replace(/translateY\(([-\d.]+)px\)/, "y $1").replace(/scale\(([-\d.]+)\)/, "s $1") : "-"}@${x.t}`).join(" → ");
/** A phase is known by its heading, the only words each phase never shares. */
const pha = (m) =>
  !m ? "không thấy màn" : /Cuộc đi khép lại\. Câu chuyện còn đây\./.test(m.chu) ? "khep" : /Mang theo điều gì vào sổ\?/.test(m.chu) ? "nguon" : /Những mẩu chuyện đang thành trang/.test(m.chu) ? "dung" : /Đã giữ lại một cuộc đi\./.test(m.chu) ? "da-luu" : /Viết lại theo cách mình nhớ/.test(m.chu) ? "sua" : m.nut.some((n) => n.ten === "Sửa theo cách mình nhớ") ? "doc-thu" : m.loi ? "loi" : "khác";
/** Wait until the diary screen reaches a phase, polling its heading. */
const choPha = async (page, muon, toiDa = 15_000, testid) => {
  const t0 = Date.now();
  let m = null;
  while (Date.now() - t0 < toiDa) {
    m = await doMan(page, testid);
    if (muon.includes(pha(m))) return { m, ms: Date.now() - t0 };
    await page.waitForTimeout(250);
  }
  return { m, ms: null };
};
/** Requests to the diary routes the page made, with their statuses. */
const theoDoiSo = (page) => {
  const log = [];
  page.on("response", (r) => {
    const u = new URL(r.url());
    if (/^\/(outings\/[^/]+\/(ending|diary|diary-sources|diary-jobs)|diary-jobs\/|diaries\/|people\/[^/]+\/diaries)/.test(u.pathname)) log.push({ m: r.request().method(), p: anId(u.pathname), s: r.status() });
  });
  return log;
};
const tomSo = (log) => log.map((x) => `${x.m} ${x.p} ${x.s}`).join(", ") || "không";
const lyDoCua = async (page, ten) =>
  page.evaluate(
    ({ ten, src }) => {
      const doc = (0, eval)(src);
      const b = [...document.querySelectorAll('[role="button"]')].find((x) => x.getClientRects().length && (x.getAttribute("aria-label") || (x.innerText ?? "")).replace(/[-]/g, "").trim() === ten);
      return b ? { tat: b.getAttribute("aria-disabled") === "true", lyDo: doc(b) } : null;
    },
    { ten, src: LY_DO_SRC },
  );

try {
  // ------------------------------------------------------------------ api
  // The contract before anything is closed: who may close, what the sources say, whose walls hold books.
  if (chay("api")) {
    const dong = [];
    for (const ten of ["chat-0", "chat-1", NGOAI]) {
      const p = ten === NGOAI ? await phienNgoai() : await phienCua(ten);
      for (const [nhan, o] of [["O1", O1], ["O2", O2]]) {
        const e = await goi("GET", `/outings/${o.id}/ending`, undefined, p);
        const s = await goi("GET", `/outings/${o.id}/diary-sources`, undefined, p);
        dong.push({ ten, nhan, e: e.status, code: e.code, can: e.json?.can_end ?? null, ended: e.json?.ended_at ?? null, kind: e.json?.kind ?? null, s: s.status, sCode: s.code });
      }
      const w = await goi("GET", `/people/${p.person_id}/diaries`, undefined, p);
      dong.push({ ten, nhan: "tường", e: w.status, soSach: w.json?.diaries?.length ?? null });
    }
    const O1c0 = dong.find((d) => d.ten === "chat-0" && d.nhan === "O1");
    const O1c1 = dong.find((d) => d.ten === "chat-1" && d.nhan === "O1");
    const O1n = dong.find((d) => d.ten === NGOAI && d.nhan === "O1");
    const O2c0 = dong.find((d) => d.ten === "chat-0" && d.nhan === "O2");
    const daDong = !!O1c0?.ended;
    ghi({
      tc: "TC-N15-API",
      screen: "N15.S01",
      state: `${daDong ? "sau khi khép" : "trước khi khép"} «${KEO}» (O1, 29–30/09) và «${KEO_SAU}» (O2, 24–25/10); ba người: tổ chức, thành viên, người ngoài nhóm`,
      action: "GET /outings/{id}/ending, /diary-sources, /people/{id}/diaries",
      cauHinh: "-",
      expected: "chỉ người tổ chức được khép (can_end); O2 chưa bắt đầu thì không cho khép; người ngoài nhóm nhận 404; nguồn chỉ mở sau khi khép",
      status: daDong || (O1c0?.can && !O1c1?.can && O1n?.e === 404 && O1c0?.s === 409) ? (O2c0?.can ? "FAIL" : "PASS") : "FAIL",
      ghiChu: dong.map((d) => (d.nhan === "tường" ? `${d.ten} tường ${d.e} (${d.soSach} sổ)` : `${d.ten} ${d.nhan}: ending ${d.e}${d.code ? ` ${d.code}` : ""}${d.can !== null ? ` can_end=${d.can}` : ""}${d.ended ? " đã khép" : ""}${d.kind ? ` kind=${d.kind}` : ""}; nguồn ${d.s}${d.sCode ? ` ${d.sCode}` : ""}`)).join(" | "),
    });
  }

  // ------------------------------------------------------------------ vao
  // Ways in: the outing's own screen, and the wall shelf «Những ngày muốn giữ».
  if (chay("vao")) {
    // The outing screens (before `khep` closed O1): skipped when only a shelf part is asked for.
    if (chayPhan("vao", "keo")) {
      const t = await mo("C1", "chat-0", `/outings/${O1.id}?ctx=${G.id}`);
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(2500);
      await choOn(t.page, { mang: t.mang });
      const giu = await nut(t.page, "Giữ lại cuộc đi");
      await anh(t.page, "EV-N15-VAO-KEO-C1", t, giu ? { chuThich: [{ rect: { x: giu.x - giu.w / 2, y: giu.y - giu.h / 2, w: giu.w, h: giu.h } }] } : {});
      let den = null;
      if (giu) {
        await cham(cdp, giu);
        await t.page.waitForTimeout(2500);
        den = anId(await duongDan(t.page));
      }
      ghi({ tc: "TC-N15-VAO-KEO", screen: "N15.S01", state: `chat-0 (người tổ chức), màn kèo «${KEO}», kèo đã tới ngày`, action: "tìm «Giữ lại cuộc đi» rồi chạm", cauHinh: "C1", expected: "có lối «Giữ lại cuộc đi», tới /outings/[id]/ending", status: giu && /\/outings\/\[id\]\/ending$/.test(den ?? "") ? "PASS" : "FAIL", evidence: ["EV-N15-VAO-KEO-C1"], ghiChu: `nút ${giu ? `«${giu.ten}» ${giu.w}×${giu.h} ở y ${Math.round(giu.y)}` : "không thấy"}; tới ${den ?? "-"}` });
      await t.context.close();

      const t2 = await mo("C1", "chat-0", `/outings/${O2.id}?ctx=${G.id}`);
      await t2.page.waitForTimeout(2500);
      await choOn(t2.page, { mang: t2.mang });
      const giu2 = await nut(t2.page, "Giữ lại cuộc đi");
      const chu2 = (await chuTrang(t2.page)).slice(0, 80);
      ghi({ tc: "TC-N15-VAO-SAP-TOI", screen: "N15.S01", state: `chat-0, màn kèo «${KEO_SAU}» (24–25/10, chưa tới ngày)`, action: "tìm «Giữ lại cuộc đi»", cauHinh: "C1", expected: "kèo chưa bắt đầu thì không có lối khép", status: !giu2 && /Kèo retest 2 ngày/.test(chu2) ? "PASS" : "FAIL", ghiChu: `nút «Giữ lại cuộc đi»: ${giu2 ? "có" : "không"}; đầu màn «${chu2}»` });
      await t2.context.close();
    }

    // The shelf with nothing on it (chat-2 keeps no book) and with a book (chat-0, once `luu` ran).
    for (const [ten, cfg] of [["chat-2", "C1"], ["chat-2", "C2"], ["chat-2", "C3"], ["chat-0", "C1"], ["chat-0", "C2"], ["chat-0", "C3"]]) {
      if (!chayPhan("vao", `${ten}:${cfg}`) && !chayPhan("vao", "tuong")) continue;
      const p = await phienCua(ten);
      const soSach = (await goi("GET", `/people/${p.person_id}/diaries`, undefined, p)).json?.diaries?.length ?? 0;
      if (ten === "chat-0" && !soSach) continue;
      const nhan = soSach ? "CO-SO" : "RONG";
      const t3 = await mo(cfg, ten, "/profile");
      await t3.page.waitForTimeout(2500);
      await choOn(t3.page, { mang: t3.mang });
      const ke = await t3.page.evaluate(() => {
        const w = [...document.querySelectorAll('[data-testid="diary-wall"]')].filter((x) => x.getClientRects().length).pop();
        if (!w) return null;
        w.scrollIntoView({ block: "start", behavior: "instant" });
        const r = w.getBoundingClientRect();
        return { chu: (w.innerText ?? "").replace(/[\uE000-\uF8FF]/g, "").replace(/\s+/g, " ").trim(), nut: [...w.querySelectorAll('[role="button"],a[href]')].filter((b) => b.getClientRects().length).map((b) => (b.getAttribute("aria-label") || (b.innerText ?? "")).trim()), top: Math.round(r.top) };
      });
      await t3.page.waitForTimeout(400);
      await anh(t3.page, `EV-N15-TUONG-${nhan}-${cfg}`, t3);
      ghi({ tc: `TC-N15-TUONG-${nhan}`, screen: "N15.S05", state: `${ten}, tab Cá nhân, kệ «Những ngày muốn giữ» ${soSach ? `có ${soSach} sổ` : "chưa có sổ nào"}`, action: "mở tab Cá nhân, cuộn tới kệ", cauHinh: cfg, expected: soSach ? "kệ có cuốn sổ, chạm mở được" : "trạng thái rỗng theo EmptyState của DESIGN.md: một câu và một hành động dẫn tới chỗ có trang đầu tiên", status: ke ? (soSach ? (ke.nut.some((n) => /^Mở /.test(n)) ? "PASS" : "FAIL") : ke.nut.length ? "PASS" : "FAIL") : "FAIL", evidence: [`EV-N15-TUONG-${nhan}-${cfg}`], ghiChu: ke ? `kệ «${ke.chu.slice(0, 220)}»; nút trong kệ: ${ke.nut.join(", ") || "không"}` : "không thấy kệ" });
      await t3.context.close();
    }
  }

  // ------------------------------------------------------------ khep-truoc
  // The outing before it is closed: a member who cannot close it, the organiser at five sizes,
  // an outing still to come, and one the reader has no access to.
  if (chay("khep-truoc")) {
    const daDong = !!(await goi("GET", `/outings/${O1.id}/ending`, undefined, await phienCua("chat-0"))).json?.ended_at;
    if (daDong) console.log("khep-truoc: O1 đã khép, bỏ qua phần đo trước khi khép");
    else {
      if (chayPhan("khep-truoc", "thanh-vien")) {
        const t = await mo("C1", "chat-1", `/outings/${O1.id}/ending`);
        const cdp = await cdpCua(t.page);
        const { m, ms } = await choPha(t.page, ["khep", "loi"]);
        await anh(t.page, "EV-N15-KHEP-THANH-VIEN-C1", t);
        const tieuDe0 = m?.chu.slice(0, 20);
        const seg = await nut(t.page, "Khoảnh khắc");
        if (seg) await cham(cdp, seg);
        await t.page.waitForTimeout(700);
        const m2 = await doMan(t.page);
        await anh(t.page, "EV-N15-KHEP-THANH-VIEN-DOI-C1", t);
        const coDau = m?.nut.some((n) => n.ten === "Khép cuộc đi");
        const segNut = m?.nut.filter((n) => n.vaiTro === "tab").map((n) => `«${n.ten}» ${n.chon}`) ?? [];
        ghi({ tc: "TC-N15-KHEP-THANH-VIEN", screen: "N15.S02", state: `chat-1 (thành viên, không tổ chức), «${KEO}» chưa khép`, action: "mở /outings/[id]/ending; chạm «Khoảnh khắc»", cauHinh: "C1", expected: "không có dấu «Khép cuộc đi», có câu nói người tổ chức sẽ khép; không có điều khiển nào đổi được mà không có tác dụng", status: !coDau && /Người tổ chức sẽ khép cuộc đi/.test(m?.chu ?? "") && !segNut.length ? "PASS" : "FAIL", evidence: ["EV-N15-KHEP-THANH-VIEN-C1", "EV-N15-KHEP-THANH-VIEN-DOI-C1"], ghiChu: `tới pha ${pha(m)} sau ${ms ?? "?"} ms; «Khép cuộc đi»: ${coDau ? "có" : "không"}; câu «Người tổ chức sẽ khép…»: ${/Người tổ chức sẽ khép cuộc đi/.test(m?.chu ?? "") ? "có" : "không"}; bộ chọn loại: ${segNut.join(", ") || "không"}; chạm «Khoảnh khắc»: đầu màn «${tieuDe0}» → «${m2?.chu.slice(0, 20)}», câu dưới «${(m2?.chu.match(/(Không cần đi xa[^.]*\.|Mang về vài tấm ảnh[^.]*\.)/) ?? [])[0] ?? "-"}»` });
        await t.context.close();
      }
      for (const cfg of ["C1", "C2", "C3", "C8", "C6"]) {
        if (!chayPhan("khep-truoc", cfg)) continue;
        const t = await mo(cfg, "chat-0", `/outings/${O1.id}/ending`);
        const { m, ms } = await choPha(t.page, ["khep", "loi"]);
        const cap = await anh(t.page, `EV-N15-KHEP-${cfg}`, t);
        const dau = m?.nut.find((n) => n.ten === "Khép cuộc đi");
        const tt = cap?.tomTat ?? {};
        ghi({ tc: "TC-N15-KHEP-BASE", screen: "N15.S02", state: `chat-0 (người tổ chức), «${KEO}» chưa khép`, action: "mở /outings/[id]/ending", cauHinh: cfg, expected: "lời mời khép: tên, ngày, chọn «Khoảnh khắc | Sổ chuyến đi», dấu «Khép cuộc đi» với tới được; không tràn, không cắt", status: pha(m) === "khep" && dau && (tt.tranTrangPx ?? 0) === 0 && (tt.chuBiCat ?? 0) === 0 ? "PASS" : "FAIL", evidence: [`EV-N15-KHEP-${cfg}`], ghiChu: `pha ${pha(m)} sau ${ms ?? "?"} ms; «Khép cuộc đi» ${dau ? `${dau.w}×${dau.h} ở y ${dau.y}–${dau.day} (cửa sổ ${m.h})` : "không thấy"}; bộ chọn: ${m?.nut.filter((n) => n.vaiTro === "tab").map((n) => `«${n.ten}» aria-selected=${n.chon}`).join(", ") || "không"}; tràn trang ${tt.tranTrangPx ?? "?"}px, chữ bị cắt ${tt.chuBiCat ?? "?"}, vùng bấm < 48: ${tt.vungBamNho48 ?? "?"}` });
        await t.context.close();
      }
      if (chayPhan("khep-truoc", "sap-toi")) {
        // An outing still to come, opened by its address: the server refuses the close (409).
        const t = await mo("C1", "chat-0", `/outings/${O2.id}/ending`);
        const cdp = await cdpCua(t.page);
        const so = theoDoiSo(t.page);
        const { m } = await choPha(t.page, ["khep", "loi"]);
        await anh(t.page, "EV-N15-KHEP-SAP-TOI-C1", t);
        const dau = m?.nut.find((n) => n.ten === "Khép cuộc đi");
        let m2 = null;
        if (dau) {
          const p = await nut(t.page, "Khép cuộc đi");
          if (p) await cham(cdp, p);
          await t.page.waitForTimeout(1800);
          m2 = await doMan(t.page);
          await anh(t.page, "EV-N15-KHEP-SAP-TOI-LOI-C1", t);
        }
        ghi({ tc: "TC-N15-KHEP-SAP-TOI", screen: "N15.S02", state: `chat-0 (người tổ chức), «${KEO_SAU}» (24–25/10) mở thẳng bằng đường dẫn`, action: "mở /outings/[id]/ending; chạm «Khép cuộc đi»", cauHinh: "C1", expected: "kèo chưa tới ngày thì màn không mời khép (hoặc nói trước là chưa khép được); không bấm rồi mới báo lỗi", status: !dau ? "PASS" : "FAIL", evidence: ["EV-N15-KHEP-SAP-TOI-C1", ...(m2 ? ["EV-N15-KHEP-SAP-TOI-LOI-C1"] : [])], ghiChu: `pha ${pha(m)}; «Khép cuộc đi» ${dau ? "có" : "không"}; sau khi chạm: ${m2?.loi ? `câu lỗi «${m2.loi.chu.slice(0, 160)}» ở y ${m2.loi.top}–${m2.loi.day} (${m2.loi.trongCuaSo ? "trong" : "ngoài"} cửa sổ ${m2.h})` : "không có câu lỗi"}; gọi: ${tomSo(so)}` });
        await t.context.close();
      }
      if (chayPhan("khep-truoc", "khong-co")) {
        // An outing the reader cannot see (outside the group), and one that does not exist.
        const kq = [];
        for (const [ten, id, nhan] of [[NGOAI, O1.id, "NGOAI"], ["chat-0", randomUUID(), "KHONG-CO"]]) {
          const t = await mo("C1", ten, `/outings/${id}/ending`);
          const { m } = await choPha(t.page, ["loi", "khep"], 8000);
          await anh(t.page, `EV-N15-KHEP-${nhan}-C1`, t);
          kq.push({ ten, nhan, pha: pha(m), loi: m?.loi?.chu ?? null });
          await t.context.close();
        }
        const saiNguyenNhan = kq.some((k) => /Cập nhật app/.test(k.loi ?? ""));
        ghi({ tc: "TC-N15-KHEP-404", screen: "N15.S02", state: `${NGOAI} (ngoài nhóm) mở kèo của nhóm chat-test; chat-0 mở một kèo không tồn tại`, action: "mở /outings/[id]/ending", cauHinh: "C1", expected: "một câu nói đúng: kèo không có hoặc không dành cho bạn, và lối về; không bảo cập nhật app", status: kq.every((k) => k.loi) && !saiNguyenNhan ? "PASS" : "FAIL", evidence: kq.map((k) => `EV-N15-KHEP-${k.nhan}-C1`), ghiChu: kq.map((k) => `${k.ten}: pha ${k.pha}, câu «${(k.loi ?? "-").slice(0, 170)}»`).join(" | ") });
      }
    }
  }

  // ------------------------------------------------------------------ khep
  // chat-0 closes «Kèo album retest» (a write that is not undone). Skipped once closed.
  if (chay("khep")) {
    const t = await mo("C1", "chat-0", `/outings/${O1.id}/ending`);
    const cdp = await cdpCua(t.page);
    const so = theoDoiSo(t.page);
    const { m } = await choPha(t.page, ["khep", "nguon", "doc-thu", "sua", "da-luu", "loi"]);
    if (pha(m) !== "khep") console.log(`khep: O1 đã khép (pha ${pha(m)}), không bấm lại`);
    else {
      const p = await nut(t.page, "Khép cuộc đi");
      if (p) await cham(cdp, p);
      const r = await choPha(t.page, ["nguon", "loi"], 15_000);
      await anh(t.page, "EV-N15-KHEP-XONG-C1", t);
      const e1 = await goi("GET", `/outings/${O1.id}/ending`, undefined, await phienCua("chat-1"));
      ghi({ tc: "TC-N15-KHEP", screen: "N15.S02", state: `chat-0 (người tổ chức), «${KEO}» chưa khép, loại gợi ý «Sổ chuyến đi»`, action: "chạm «Khép cuộc đi»", cauHinh: "C1", expected: "cuộc đi khép cho cả hội; màn sang «Mang theo điều gì vào sổ?»; thành viên đọc ending thấy đã khép", status: pha(r.m) === "nguon" && e1.json?.ended_at ? "PASS" : "FAIL", evidence: ["EV-N15-KHEP-XONG-C1"], ghiChu: `sau ${r.ms ?? "-"} ms: pha ${pha(r.m)}; gọi: ${tomSo(so)}; chat-1 đọc ending: ${e1.json?.ended_at ? "đã khép" : "chưa khép"}, can_end=${e1.json?.can_end}, kind=${e1.json?.kind}` });
    }
    await t.context.close();
  }

  /** The editor of chat-0 (or `ten`) at a draft or saved book: builds one by hand when none exists yet. */
  const denSo = async (t, cdp) => {
    let { m } = await choPha(t.page, ["nguon", "doc-thu", "sua", "da-luu", "loi"]);
    if (pha(m) === "nguon") {
      const p = await nut(t.page, "Tự xếp trang, không gửi AI");
      if (p) await cham(cdp, p);
      m = (await choPha(t.page, ["doc-thu", "sua", "loi"], 20_000)).m;
    }
    return m;
  };

  // ----------------------------------------------------------------- nguon
  // What goes into the book: the photos of the outing's days, an excerpt, the bundle Nếp would receive.
  if (chay("nguon")) {
    for (const cfg of ["C1", "C2", "C3", "C8", "C6"]) {
      if (!chayPhan("nguon", cfg)) continue;
      const t = await mo(cfg, "chat-0", `/outings/${O1.id}/ending`);
      const cdp = await cdpCua(t.page);
      const { m } = await choPha(t.page, ["nguon", "doc-thu", "sua", "khep", "loi"]);
      if (pha(m) !== "nguon") {
        console.log(`nguon ${cfg}: màn đang ở pha ${pha(m)} (đã có sổ, hoặc chưa khép); bỏ qua`);
        await t.context.close();
        continue;
      }
      await t.page.waitForTimeout(1500);
      const m1 = await doMan(t.page);
      const cap = await anh(t.page, `EV-N15-NGUON-${cfg}`, t);
      const tt = cap?.tomTat ?? {};
      const hop = m1.nut.filter((n) => n.vaiTro === "checkbox");
      const dem = m1.chu.match(/(\d+) \/ 40 ảnh đã chọn/)?.[1] ?? null;
      const tom = m1.chu.match(/(\d+) ảnh · (\d+) trích đoạn/);
      const anhTai = m1.anh.filter((a) => a.nw > 0).length;
      ghi({ tc: "TC-N15-NGUON-BASE", screen: "N15.S03", state: `chat-0, «${KEO}» đã khép, chưa có sổ`, action: "mở /outings/[id]/ending (pha chất liệu)", cauHinh: cfg, expected: "lưới ảnh của những ngày đi, số ảnh đã chọn, ô trích đoạn, phần Nếp sẽ nhận; không tràn, không cắt", status: dem !== null && hop.length && (tt.tranTrangPx ?? 0) === 0 && (tt.chuBiCat ?? 0) === 0 ? "PASS" : "FAIL", evidence: [`EV-N15-NGUON-${cfg}`], ghiChu: `«${dem ?? "?"} / 40 ảnh đã chọn»; ${hop.length} ô ảnh, ${m1.anh.length} ảnh, ${anhTai} ảnh đã tải (kiểu ${[...new Set(m1.anh.map((a) => a.kieu))].join(", ") || "-"}); phần gửi Nếp «${tom?.[0] ?? "-"}»; tràn trang ${tt.tranTrangPx ?? "?"}px, chữ bị cắt ${tt.chuBiCat ?? "?"}, vùng bấm < 48: ${tt.vungBamNho48 ?? "?"}` });
      if (cfg === "C1") {
        ghi({ tc: "TC-N15-NGUON-ANH", screen: "N15.S03", state: "chat-0, pha chất liệu, ảnh là ảnh tường nhóm của những ngày đi", action: "đọc từng ô ảnh", cauHinh: "C1", expected: "mọi ô ảnh hiện ảnh (ảnh tải xong, có kích thước tự nhiên)", status: m1.anh.length && anhTai === m1.anh.length ? "PASS" : "FAIL", evidence: [`EV-N15-NGUON-${cfg}`], ghiChu: m1.anh.map((a) => `${a.w}×${a.h} tự nhiên ${a.nw}×${a.nh} ${a.xong ? "xong" : "chưa xong"} ${a.kieu}`).join("; ") || "không có ảnh" });
        ghi({ tc: "TC-N15-NGUON-ARIA", screen: "N15.S03", state: "chat-0, pha chất liệu", action: "đọc trạng thái trợ năng của ô ảnh", cauHinh: "C1", expected: "ô ảnh là checkbox có aria-checked đúng trạng thái chọn", status: hop.length && hop.every((h) => h.chon === "true" || h.chon === "false") ? "PASS" : "FAIL", ghiChu: hop.slice(0, 4).map((h) => `«${h.ten}» aria-checked=${h.chon}`).join(", ") });
        // Toggle the first photo off and on again: the count and the label follow.
        const o = await nut(t.page, hop[0]?.ten ?? "-");
        let sau = null;
        let lai = null;
        if (o) {
          await cham(cdp, o);
          await t.page.waitForTimeout(500);
          sau = (await doMan(t.page)).chu.match(/(\d+) \/ 40 ảnh đã chọn/)?.[1] ?? null;
          const o2 = await nut(t.page, hop[0].ten);
          if (o2) await cham(cdp, o2);
          await t.page.waitForTimeout(500);
          lai = (await doMan(t.page)).chu.match(/(\d+) \/ 40 ảnh đã chọn/)?.[1] ?? null;
        }
        ghi({ tc: "TC-N15-NGUON-CHON", screen: "N15.S03", state: "chat-0, pha chất liệu", action: "chạm ô ảnh đầu hai lần", cauHinh: "C1", expected: "số ảnh đã chọn giảm một rồi trở lại; phần Nếp sẽ nhận theo đúng", status: o && Number(sau) === Number(dem) - 1 && Number(lai) === Number(dem) ? "PASS" : "FAIL", ghiChu: `ô «${hop[0]?.ten ?? "-"}»: ${dem} → ${sau} → ${lai}` });
        const goDuoc = await go(t, cdp, "Một đoạn chuyện muốn gửi cùng", "Buổi chiều bên hồ, cả nhóm ngồi đợi hoàng hôn.");
        await t.page.waitForTimeout(500);
        const m2 = await doMan(t.page);
        ghi({ tc: "TC-N15-NGUON-TRICH", screen: "N15.S03", state: "chat-0, pha chất liệu", action: "gõ một trích đoạn", cauHinh: "C1", expected: "phần «Nếp sẽ nhận đúng phần này» có «1 trích đoạn» và chính câu đó; câu nói chat không được tự đọc", status: goDuoc && /1 trích đoạn/.test(m2.chu) && /Buổi chiều bên hồ/.test(m2.chu) && /Chat không được tự đọc/.test(m2.chu) ? "PASS" : "FAIL", ghiChu: `ô: ${goDuoc ? "gõ được" : "không thấy"}; phần Nếp nhận «${m2.chu.match(/\d+ ảnh · \d+ trích đoạn/)?.[0] ?? "-"}»; câu riêng tư: ${/Chat không được tự đọc/.test(m2.chu) ? "có" : "không"}` });
      }
      await t.context.close();
    }
  }

  // --------------------------------------------------------------- dung-ai
  // «Dựng sổ cùng Nếp» with no model key: how long the wait, and what the person is told.
  if (chay("dung-ai")) {
    const t = await mo("C1", "chat-0", `/outings/${O1.id}/ending`);
    const cdp = await cdpCua(t.page);
    const so = theoDoiSo(t.page);
    const { m } = await choPha(t.page, ["nguon", "doc-thu", "sua", "loi"]);
    if (pha(m) === "doc-thu" || pha(m) === "sua") {
      const p = await nut(t.page, "Chọn lại chất liệu");
      if (p) await cham(cdp, p);
      await t.page.waitForTimeout(900);
    }
    const p = await nut(t.page, "Dựng sổ cùng Nếp");
    const t0 = Date.now();
    if (p) await cham(cdp, p);
    const dang = await choPha(t.page, ["dung"], 5000);
    const nutKhiDung = dang.m ? dang.m.nut.map((n) => n.ten) : [];
    if (dang.m) await anh(t.page, "EV-N15-DUNG-AI-CHO-C1", t);
    const xong = await choPha(t.page, ["nguon", "doc-thu", "loi"], 125_000);
    const ms = Date.now() - t0;
    await anh(t.page, "EV-N15-DUNG-AI-C1", t);
    const jobs = so.filter((x) => /diary-jobs/.test(x.p));
    ghi({ tc: "TC-N15-DUNG-AI", screen: "N15.S03", state: "chat-0, pha chất liệu; stack không có khoá AI", action: "chạm «Dựng sổ cùng Nếp»", cauHinh: "C1", expected: "một câu nói Nếp chưa dựng được và vì sao, trong vài giây; trong lúc chờ có lối dừng; chất liệu còn nguyên và có lối tự xếp", status: pha(xong.m) === "nguon" && ms < 15_000 && xong.m.loi && nutKhiDung.length > 0 ? "PASS" : "FAIL", evidence: [...(dang.m ? ["EV-N15-DUNG-AI-CHO-C1"] : []), "EV-N15-DUNG-AI-C1"], ghiChu: `nút: ${p ? "có" : "không"}; pha dựng ${dang.m ? `sau ${dang.ms} ms, nút lúc chờ: ${nutKhiDung.join(", ") || "không có nút nào"}` : "không thấy"}; kết thúc sau ${ms} ms ở pha ${pha(xong.m)}; câu lỗi «${(xong.m?.loi?.chu ?? "-").slice(0, 200)}»${xong.m?.loi ? ` ở y ${xong.m.loi.top}–${xong.m.loi.day} (${xong.m.loi.trongCuaSo ? "trong" : "ngoài"} cửa sổ)` : ""}; «Tự xếp trang»: ${xong.m?.nut.some((n) => n.ten === "Tự xếp trang, không gửi AI") ? "có" : "không"}; gọi diary-jobs: ${jobs.length} (${[...new Set(jobs.map((j) => `${j.m} ${j.s}`))].join(", ")})` });
    await t.context.close();
  }

  // -------------------------------------------------------------- dung-tay
  // «Tự xếp trang, không gửi AI»: the draft book, and its turn (C1) or none (C9, Reduce Motion).
  if (chay("dung-tay")) {
    for (const cfg of ["C1", "C9"]) {
      if (!chayPhan("dung-tay", cfg)) continue;
      const t = await mo(cfg, "chat-0", `/outings/${O1.id}/ending`);
      const cdp = await cdpCua(t.page);
      const { m } = await choPha(t.page, ["nguon", "doc-thu", "sua", "loi"]);
      if (pha(m) !== "nguon") {
        const p = await nut(t.page, "Chọn lại chất liệu");
        if (p) await cham(cdp, p);
        await t.page.waitForTimeout(900);
      }
      const p = await nut(t.page, "Tự xếp trang, không gửi AI");
      await batDauMau(t.page);
      const t0 = Date.now();
      if (p) await cham(cdp, p);
      const r = await choPha(t.page, ["doc-thu", "loi"], 20_000);
      await t.page.waitForTimeout(1200);
      const mau = await dungMau(t.page);
      await t.page.waitForTimeout(800);
      const m2 = await doMan(t.page);
      const cap = await anh(t.page, `EV-N15-DUNG-TAY-${cfg}`, t);
      const giua = mau.filter((x) => x.tf && !/translateY\(0px\) scale\(1\)/.test(x.tf)).length;
      if (cfg === "C1") {
        const tt = cap?.tomTat ?? {};
        const bia = m2?.anh.find((a) => /^Ảnh bìa /.test(a.alt ?? "")) ?? null;
        ghi({ tc: "TC-N15-DUNG-TAY", screen: "N15.S04", state: "chat-0, pha chất liệu, những ảnh gợi ý theo ngày", action: "chạm «Tự xếp trang, không gửi AI»", cauHinh: cfg, expected: "cuốn sổ nháp hiện ngay: bìa có ảnh và tên, các trang; có «Sửa theo cách mình nhớ» và lối lưu; không gửi gì cho AI", status: pha(r.m) === "doc-thu" && bia && bia.nw > 0 && (tt.tranTrangPx ?? 0) === 0 ? "PASS" : "FAIL", evidence: [`EV-N15-DUNG-TAY-${cfg}`], ghiChu: `sau ${r.ms ?? "-"} ms: pha ${pha(r.m)}; bìa ${bia ? `${bia.w}×${bia.h}, tự nhiên ${bia.nw}×${bia.nh}` : "không có ảnh"}; ${m2?.anh.length ?? 0} ảnh, ${m2?.anh.filter((a) => a.nw > 0).length ?? 0} đã tải; số trang «${(m2?.chu.match(/\d+ \/ \d+/g) ?? []).join(", ")}»; tràn trang ${tt.tranTrangPx ?? "?"}px, chữ bị cắt ${tt.chuBiCat ?? "?"}` });
      }
      ghi({ tc: "TC-N15-SO-MO", screen: "N15.S04", layer: "MO-so-lat", state: `chat-0, vừa dựng sổ nháp${cfg === "C9" ? ", prefers-reduced-motion" : ""}`, action: "chạm «Tự xếp trang, không gửi AI»; lấy mẫu transform của cuốn sổ mỗi khung", cauHinh: cfg, expected: cfg === "C9" ? "Reduce Motion: sổ hiện ngay ở tư thế cuối, không khung giữa chừng (ADR-0039: tôn trọng Reduce Motion)" : "sổ lật vào (translateY và scale từ 1 về 0), có khung giữa chừng", status: cfg === "C9" ? (giua <= 1 ? "PASS" : "FAIL") : giua >= 2 ? "PASS" : "FAIL", ghiChu: `${mau.length} mẫu; ${giua} khung khác tư thế cuối; ${tomMau(mau).slice(0, 320)}` });
      await t.context.close();
    }
  }

  // ------------------------------------------------------------------- sua
  // Editing: title, cover sheet, page photos, pages added, removed, moved; a photo from the device.
  if (chay("sua") && chayPhan("sua", "C1")) {
    const t = await mo("C1", "chat-0", `/outings/${O1.id}/ending`);
    const cdp = await cdpCua(t.page);
    const m = await denSo(t, cdp);
    const sua = await bam(t, cdp, "Sửa theo cách mình nhớ");
    const m1 = (await choPha(t.page, ["sua"], 5000)).m;
    await anh(t.page, "EV-N15-SUA-C1", t);
    // The page names the hand layout gives, as the editor shows them.
    const tenTrang = await t.page.evaluate(() => [...document.querySelectorAll('[aria-label="Tên trang"]')].filter((x) => x.getClientRects().length).map((x) => x.value));
    await t.page.evaluate(() => [...document.querySelectorAll('[aria-label="Tên trang"]')].filter((x) => x.getClientRects().length)[0]?.scrollIntoView({ block: "center", behavior: "instant" }));
    await t.page.waitForTimeout(300);
    await anh(t.page, "EV-N15-TRANG-TEN-C1", t, { chuThich: [{ selector: '[aria-label="Tên trang"]' }] });
    ghi({ tc: "TC-N15-TRANG-TEN", screen: "N15.S04", state: "chat-0, sổ dựng tay, vừa vào chế độ sửa", action: "đọc ô «Tên trang» của các trang", cauHinh: "C1", expected: "tên trang là chữ người đọc được (như «Ngày 29/09/2026» chế độ đọc hiện), không phải chuỗi ngày dạng máy", status: tenTrang.length && tenTrang.every((v) => !/^\d{4}-\d{2}-\d{2}$/.test(v)) ? "PASS" : "FAIL", evidence: ["EV-N15-TRANG-TEN-C1"], ghiChu: `ô «Tên trang»: ${tenTrang.map((v) => `«${v}»`).join(", ") || "không có"}; chế độ đọc hiện: ${(m?.chu.match(/Ngày \d{2}\/\d{2}\/\d{4}/g) ?? []).join(", ") || "-"}` });
    // The title, then back to the reader to see it on the cover.
    const TEN = "Hai ngày ở hồ, trời trong";
    const goTen = await go(t, cdp, "Tên cuốn sổ", TEN);
    const ve = await bam(t, cdp, "Xem như người đọc");
    const m2 = await doMan(t.page);
    ghi({ tc: "TC-N15-SUA-TEN", screen: "N15.S04", state: "chat-0, sổ nháp dựng tay", action: "«Sửa theo cách mình nhớ» → sửa «Tên cuốn sổ» → «Xem như người đọc»", cauHinh: "C1", expected: "bìa của cuốn sổ mang tên mới", status: sua && goTen && ve && m2?.chu.includes(TEN) ? "PASS" : "FAIL", evidence: ["EV-N15-SUA-C1"], ghiChu: `pha trước ${pha(m)}; vào sửa: ${sua ? "có" : "không"} (pha ${pha(m1)}); gõ tên: ${goTen ? "có" : "không"}; về chế độ đọc: ${ve ? "có" : "không"}; bìa có tên mới: ${m2?.chu.includes(TEN) ? "có" : "không"}` });
    // The cover sheet: a dialog, focus inside, radios, Esc, a pick closes it.
    await bam(t, cdp, "Sửa theo cách mình nhớ");
    await choPha(t.page, ["sua"], 5000);
    const moBia = await bam(t, cdp, "Thay ảnh bìa", { cho: 1200 });
    const d = await dialogTren(t.page);
    const f = await focusHienTai(t.page);
    const radio = await t.page.evaluate(() => {
      const dl = [...document.querySelectorAll('[role="dialog"]')].filter((x) => x.getClientRects().length).pop();
      return dl ? [...dl.querySelectorAll('[role="radio"]')].filter((x) => x.getClientRects().length).map((x) => ({ ten: x.getAttribute("aria-label"), checked: x.getAttribute("aria-checked"), img: !!x.querySelector("img"), nw: x.querySelector("img")?.naturalWidth ?? 0 })) : [];
    });
    await anh(t.page, "EV-N15-BIA-SHEET-C1", t);
    await t.page.keyboard.press("Escape");
    await t.page.waitForTimeout(800);
    const dEsc = await dialogTren(t.page);
    await bam(t, cdp, "Thay ảnh bìa", { cho: 1200 });
    const chon = radio.find((r) => r.checked !== "true") ?? radio[1] ?? null;
    let dSau = null;
    if (chon) {
      const p = await nut(t.page, chon.ten, { trongDialog: true });
      if (p) await cham(cdp, p);
      await t.page.waitForTimeout(900);
      dSau = await dialogTren(t.page);
    }
    ghi({ tc: "TC-N15-BIA-SHEET", screen: "N15.S04", layer: "L-so-chon-anh", state: "chat-0, đang sửa sổ", action: "«Thay ảnh bìa»; Esc; mở lại, chọn một ảnh khác", cauHinh: "C1", expected: "sheet có role dialog và tên, tiêu điểm vào trong, ảnh hiện trong ô; Esc đóng; chọn ảnh bìa thì sheet đóng", status: moBia && d?.ten && f.trongDialog && radio.length && radio.every((r) => r.nw > 0) && !dEsc && chon && !dSau ? "PASS" : "FAIL", evidence: ["EV-N15-BIA-SHEET-C1"], ghiChu: `sheet «${d?.ten ?? "-"}» ${d ? `${d.rong}×${d.cao} ở y ${d.top}` : ""}; tiêu điểm «${f.ten}» (trong dialog ${f.trongDialog}); ${radio.length} ô, ảnh đã tải ${radio.filter((r) => r.nw > 0).length}; sau Esc: ${dEsc ? `còn «${dEsc.ten}»` : "đóng"}; chọn «${chon?.ten ?? "-"}»: ${dSau ? "sheet còn" : "sheet đóng"}` });
    ghi({ tc: "TC-N15-BIA-RADIO", screen: "N15.S04", layer: "L-so-chon-anh", state: "chat-0, sheet ảnh bìa", action: "đọc trạng thái trợ năng của ô ảnh", cauHinh: "C1", expected: "ô ảnh là radio có aria-checked (ảnh bìa đang dùng là true)", status: radio.length && radio.every((r) => r.checked === "true" || r.checked === "false") ? "PASS" : "FAIL", ghiChu: radio.slice(0, 4).map((r) => `«${r.ten}» aria-checked=${r.checked}`).join(", ") || "không có ô" });
    // Pages: one more, then removed; the second moved up; the only page cannot be removed (reason).
    const trang = async () => (await doMan(t.page)).chu.match(/Trang \d+/g)?.length ?? 0;
    const n0 = await trang();
    const them = await bam(t, cdp, "Thêm một trang viết");
    const n1 = await trang();
    const boCuoi = await t.page.evaluate(() => {
      const bs = [...document.querySelectorAll('[role="button"]')].filter((b) => b.getClientRects().length && (b.innerText ?? "").trim() === "Bỏ trang này");
      const b = bs.at(-1);
      if (!b) return null;
      b.scrollIntoView({ block: "center", behavior: "instant" });
      const r = b.getBoundingClientRect();
      return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
    });
    if (boCuoi) await cham(cdp, boCuoi);
    await t.page.waitForTimeout(700);
    const n2 = await trang();
    const lyDoBo = n2 === 1 ? await lyDoCua(t.page, "Bỏ trang này") : null;
    ghi({ tc: "TC-N15-TRANG-THEM-BO", screen: "N15.S04", state: "chat-0, đang sửa sổ", action: "«Thêm một trang viết», rồi «Bỏ trang này» ở trang cuối", cauHinh: "C1", expected: "thêm một trang rồi bỏ đúng trang đó; còn một trang thì nút bỏ tắt kèm lý do", status: them && n1 === n0 + 1 && n2 === n0 && (n2 > 1 || lyDoBo?.lyDo) ? "PASS" : "FAIL", ghiChu: `trang: ${n0} → ${n1} → ${n2}${lyDoBo ? `; còn một trang: «Bỏ trang này» tắt ${lyDoBo.tat}, lý do «${lyDoBo.lyDo ?? "không"}»` : ""}` });
    if (n2 >= 2) {
      const dau = async () => t.page.evaluate(() => [...document.querySelectorAll('[aria-label="Tên trang"]')].filter((x) => x.getClientRects().length).map((x) => x.value));
      // Both composed pages may carry the same day as their name: name them apart first.
      for (const [i, ten] of [[0, "Trang thử A"], [1, "Trang thử B"]]) {
        const o = await t.page.evaluate((i) => {
          const e = [...document.querySelectorAll('[aria-label="Tên trang"]')].filter((x) => x.getClientRects().length)[i];
          if (!e) return null;
          e.scrollIntoView({ block: "center", behavior: "instant" });
          const r = e.getBoundingClientRect();
          return { x: r.left + Math.min(40, r.width / 2), y: r.top + r.height / 2 };
        }, i);
        if (!o) continue;
        await cham(cdp, o);
        await t.page.waitForTimeout(200);
        await t.page.keyboard.press("Control+A");
        await t.page.keyboard.press("Backspace");
        await t.page.keyboard.type(ten, { delay: 8 });
        await t.page.waitForTimeout(250);
      }
      const truoc = await dau();
      const len = await bam(t, cdp, "Đưa trang 2 lên trước");
      const sau = await dau();
      ghi({ tc: "TC-N15-TRANG-DOI-CHO", screen: "N15.S04", state: "chat-0, đang sửa sổ, ít nhất hai trang", action: "«Đưa trang 2 lên trước»", cauHinh: "C1", expected: "trang 2 thành trang 1", status: len && truoc[1] === sau[0] && truoc[0] === sau[1] ? "PASS" : "FAIL", ghiChu: `tên trang trước «${truoc.slice(0, 3).join(" | ")}», sau «${sau.slice(0, 3).join(" | ")}»` });
      // Put them back so the saved book keeps the composed order.
      if (len) await bam(t, cdp, "Đưa trang 2 lên trước");
    }
    // A photo from the device: the web file chooser, a synthetic image (a write: its own part, run once).
    if (chayPhan("sua", "tai-anh")) {
    const tep = join(mt.out, "tep-n15-anh.png");
    if (!existsSync(tep)) writeFileSync(tep, pngThuBytes(640, 480));
    const cho = t.page.waitForEvent("filechooser", { timeout: 8000 }).catch(() => null);
    const nTai = await bam(t, cdp, "Thêm ảnh từ máy", { cho: 300 });
    const fc = await cho;
    let tai = null;
    if (fc) {
      await fc.setFiles(tep);
      const t0 = Date.now();
      for (let i = 0; i < 40; i++) {
        await t.page.waitForTimeout(500);
        const b = await nut(t.page, "Thêm ảnh từ máy");
        if (b && !b.tat) break;
      }
      tai = Date.now() - t0;
    }
    await bam(t, cdp, "Thay ảnh bìa", { cho: 1200 });
    const radio2 = await t.page.evaluate(() => {
      const dl = [...document.querySelectorAll('[role="dialog"]')].filter((x) => x.getClientRects().length).pop();
      return dl ? [...dl.querySelectorAll('[role="radio"]')].filter((x) => x.getClientRects().length).map((x) => ({ ten: x.getAttribute("aria-label"), nw: x.querySelector("img")?.naturalWidth ?? 0 })) : [];
    });
    await anh(t.page, "EV-N15-TAI-ANH-C1", t);
    await t.page.keyboard.press("Escape");
    await t.page.waitForTimeout(600);
    ghi({ tc: "TC-N15-TAI-ANH", screen: "N15.S04", state: "chat-0, đang sửa sổ", action: "«Thêm ảnh từ máy» → chọn một ảnh tổng hợp 640×480 → mở «Thay ảnh bìa»", cauHinh: "C1", expected: "ảnh tải lên xong và có trong danh sách chọn ảnh, hiện được", status: fc && radio2.length > radio.length && radio2.every((r) => r.nw > 0) ? "PASS" : "FAIL", evidence: ["EV-N15-TAI-ANH-C1"], ghiChu: `nút: ${nTai ? "có" : "không"}; bộ chọn tệp: ${fc ? "mở" : "không mở"}; xong sau ${tai ?? "-"} ms; ô trong sheet: ${radio.length} → ${radio2.length}, ảnh đã tải ${radio2.filter((r) => r.nw > 0).length}` });
    }
    await t.context.close();
  }
  // The cover sheet at a low window, and the editor on a tablet.
  if (chay("sua")) {
    for (const cfg of ["C8", "C6"]) {
      if (!chayPhan("sua", cfg)) continue;
      const t = await mo(cfg, "chat-0", `/outings/${O1.id}/ending`);
      const cdp = await cdpCua(t.page);
      await denSo(t, cdp);
      await bam(t, cdp, "Sửa theo cách mình nhớ");
      await choPha(t.page, ["sua"], 5000);
      const o = await t.page.evaluate(() => {
        const e = [...document.querySelectorAll('[aria-label="Tên cuốn sổ"]')].filter((x) => x.getClientRects().length).pop();
        return e ? Math.round(e.getBoundingClientRect().width) : null;
      });
      await bam(t, cdp, "Thay ảnh bìa", { cho: 1200 });
      const d = await dialogTren(t.page);
      await anh(t.page, `EV-N15-BIA-SHEET-${cfg}`, t);
      const h = t.page.viewportSize().height;
      const w = t.page.viewportSize().width;
      const dat = d ? (cfg === "C8" ? d.cao <= 0.82 * h + 1 && d.top >= 0 : (o ?? 9999) <= 560 && d.rong <= 640) : false;
      ghi({ tc: "TC-N15-SUA-CO", screen: "N15.S04", layer: "L-so-chon-anh", state: "chat-0, đang sửa sổ, sheet ảnh bìa mở", action: "«Sửa theo cách mình nhớ» → «Thay ảnh bìa»", cauHinh: cfg, expected: cfg === "C8" ? "sheet cao tối đa 82% cửa sổ, đầu sheet và «Đóng» trong cửa sổ" : "ô nhập theo cột đọc (tối đa 560), sheet không trải hết bề ngang", status: dat ? "PASS" : "FAIL", evidence: [`EV-N15-BIA-SHEET-${cfg}`], ghiChu: `ô «Tên cuốn sổ» rộng ${o ?? "-"}; sheet ${d ? `${d.rong}×${d.cao} ở y ${d.top} (${Math.round((100 * d.cao) / h)}% cửa sổ ${w}×${h})` : "không mở"}` });
      await t.context.close();
    }
  }

  // ------------------------------------------------------------------ luu
  // Keeping the book: a failed save first (503 on the save only), then a private save, the wall, the reader.
  if (chayPhan("luu", "loi") || chayPhan("luu", "giu")) {
    const t = await mo("C1", "chat-0", `/outings/${O1.id}/ending`);
    const cdp = await cdpCua(t.page);
    const so = theoDoiSo(t.page);
    const m = await denSo(t, cdp);
    if (chayPhan("luu", "loi")) {
      // In the reader view the keep controls sit under the book: tap there with the save failing.
      await loiMayChu(t.page, new RegExp(`/outings/${O1.id}/diary$`));
      const p = await nut(t.page, "Lưu riêng tư");
      const yNut = p ? Math.round(p.y) : null;
      if (p) await cham(cdp, p);
      await t.page.waitForTimeout(1500);
      const ml = await doMan(t.page);
      await anh(t.page, "EV-N15-LUU-LOI-C1", t);
      await goHet(t.page);
      ghi({ tc: "TC-N15-LUU-LOI", screen: "N15.S04", state: "chat-0, sổ nháp ở chế độ đọc, cuộn tới nút lưu; PUT …/diary trả 503", action: "chạm «Lưu riêng tư»", cauHinh: "C1", expected: "câu lỗi hiện gần nút vừa bấm, trong cửa sổ; bản nháp còn nguyên", status: ml?.loi?.trongCuaSo && pha(ml) !== "nguon" ? "PASS" : "FAIL", evidence: ["EV-N15-LUU-LOI-C1"], ghiChu: `nút ở y ${yNut ?? "-"} lúc chạm; sau: ${ml?.loi ? `câu lỗi «${ml.loi.chu.slice(0, 160)}» ở y ${ml.loi.top}–${ml.loi.day} (${ml.loi.trongCuaSo ? "trong" : "ngoài"} cửa sổ ${ml.h}), cuộn ${ml.cuon?.top ?? "-"}` : "không có câu lỗi"}; bản nháp: ${/Sửa theo cách mình nhớ/.test(ml?.chu ?? "") ? "còn" : "không thấy"}` });
    }
    const daCo = await sachCua("chat-0");
    if (daCo || !chayPhan("luu", "giu")) console.log("luu: chat-0 đã có sổ (hoặc không chạy phần giu), không lưu");
    else {
      const p = await nut(t.page, "Lưu riêng tư");
      await batDauMau(t.page);
      if (p) await cham(cdp, p);
      const r = await choPha(t.page, ["da-luu", "loi"], 15_000);
      await t.page.waitForTimeout(1200);
      const mau = await dungMau(t.page);
      await anh(t.page, "EV-N15-LUU-C1", t);
      const id = await sachCua("chat-0");
      const doc = id ? await goi("GET", `/diaries/${id}`, undefined, await phienCua("chat-0")) : null;
      ghi({ tc: "TC-N15-LUU", screen: "N15.S04", state: "chat-0, sổ nháp dựng tay, «Chỉ mình tôi»", action: "chạm «Lưu riêng tư»", cauHinh: "C1", expected: "«Đã giữ lại một cuộc đi.», nói chỉ mình bạn mở được; sổ lưu riêng tư; có lối về tường", status: pha(r.m) === "da-luu" && doc?.json?.audience === "private" && /chỉ mình bạn mở được/.test(r.m.chu) ? "PASS" : "FAIL", evidence: ["EV-N15-LUU-C1"], ghiChu: `sau ${r.ms ?? "-"} ms: pha ${pha(r.m)}; sổ ${id ? `[id], audience ${doc?.json?.audience}, phiên bản ${doc?.json?.revision}, ${doc?.json?.document?.pages?.length} trang` : "không có"}; chuyển động: ${tomMau(mau).slice(0, 200)}; gọi: ${tomSo(so).slice(0, 200)}` });
      if (pha(r.m) === "da-luu") {
        const v = await bam(t, cdp, "Về tường nhà mình", { cho: 2500 });
        const den = anId(await duongDan(t.page));
        const ke = await t.page.evaluate(() => {
          const w = [...document.querySelectorAll('[data-testid="diary-wall"]')].filter((x) => x.getClientRects().length).pop();
          if (!w) return null;
          w.scrollIntoView({ block: "start", behavior: "instant" });
          return { chu: (w.innerText ?? "").replace(/[-]/g, "").replace(/\s+/g, " ").trim(), nut: [...w.querySelectorAll('[role="button"]')].filter((b) => b.getClientRects().length).map((b) => b.getAttribute("aria-label") || (b.innerText ?? "").trim()) };
        });
        await t.page.waitForTimeout(500);
        await anh(t.page, "EV-N15-LUU-TUONG-C1", t);
        ghi({ tc: "TC-N15-LUU-TUONG", screen: "N15.S05", state: "chat-0, vừa lưu sổ riêng tư", action: "«Về tường nhà mình»", cauHinh: "C1", expected: "tới tường của mình; kệ «Những ngày muốn giữ» có cuốn sổ với «Chỉ mình tôi»", status: v && ke && ke.nut.some((n) => /^Mở /.test(n)) && /Chỉ mình tôi/.test(ke.chu) ? "PASS" : "FAIL", evidence: ["EV-N15-LUU-TUONG-C1"], ghiChu: `tới ${den}; kệ ${ke ? `«${ke.chu.slice(0, 200)}», nút ${ke.nut.join(", ")}` : "không thấy"}` });
      }
    }
    await t.context.close();
  }

  // Leaving the editor with a change not kept: the app has no guard on leaving a screen (UI-097).
  if (chayPhan("luu", "roi")) {
    const id = await sachCua("chat-0");
    if (!id) console.log("luu:roi: chat-0 chưa có sổ; chạy --chi luu:giu trước");
    else {
      const t = await mo("C1", "chat-0", `/outings/${O1.id}?ctx=${G.id}`);
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(2000);
      await choOn(t.page, { mang: t.mang });
      await bam(t, cdp, "Giữ lại cuộc đi", { cho: 2500 });
      await choPha(t.page, ["doc-thu", "sua"], 8000);
      await bam(t, cdp, "Sửa theo cách mình nhớ");
      await choPha(t.page, ["sua"], 5000);
      const TEN = "Tên sửa dở chưa lưu";
      const goDuoc = await go(t, cdp, "Tên cuốn sổ", TEN);
      await anh(t.page, "EV-N15-ROI-TRUOC-C1", t);
      const ve = await bam(t, cdp, "Quay lại", { cho: 1500 });
      const hop = await dialogTren(t.page);
      const den = anId(await duongDan(t.page));
      await bam(t, cdp, "Giữ lại cuộc đi", { cho: 2500 });
      const { m } = await choPha(t.page, ["doc-thu", "sua"], 8000);
      await anh(t.page, "EV-N15-ROI-SAU-C1", t);
      const con = (m?.chu ?? "").includes(TEN);
      const tenLuu = (await goi("GET", `/diaries/${id}`, undefined, await phienCua("chat-0"))).json?.document?.title;
      ghi({ tc: "TC-N15-ROI-MAT-SUA", screen: "N15.S04", state: "chat-0, sổ đã lưu, đang sửa: đổi «Tên cuốn sổ», chưa bấm lưu", action: "«Quay lại» ở đầu màn; rồi mở lại «Giữ lại cuộc đi»", cauHinh: "C1", expected: "rời màn khi còn chỗ sửa chưa lưu thì hỏi trước, hoặc bản sửa còn khi mở lại", status: hop || con ? "PASS" : "FAIL", evidence: ["EV-N15-ROI-TRUOC-C1", "EV-N15-ROI-SAU-C1"], ghiChu: `gõ tên: ${goDuoc ? "có" : "không"}; «Quay lại»: ${ve ? "chạm" : "không thấy"}; hộp hỏi: ${hop ? `«${hop.ten}»` : "không"}; về ${den}; mở lại: pha ${pha(m)}, tên đang sửa ${con ? "còn" : "mất"}; tên đã lưu trên máy chủ «${tenLuu ?? "-"}»` });
      await t.context.close();
    }
  }

  // ------------------------------------------------------------------- doc
  // The reader of a kept book: its owner at three sizes; someone outside the group while it is private.
  if (chay("doc")) {
    const id = await sachCua("chat-0");
    if (!id) console.log("doc: chat-0 chưa có sổ; chạy --chi luu trước");
    else {
      const rieng = (await goi("GET", `/diaries/${id}`, undefined, await phienCua("chat-0"))).json?.audience === "private";
      for (const cfg of ["C1", "C2", "C3"]) {
        if (!chayPhan("doc", cfg)) continue;
        const t = await mo(cfg, "chat-0", `/diaries/${id}`);
        await t.page.waitForTimeout(2500);
        await choOn(t.page, { mang: t.mang });
        const m = await doMan(t.page, "diary-reader-screen");
        const cap = await anh(t.page, `EV-N15-DOC-${cfg}`, t);
        const tt = cap?.tomTat ?? {};
        ghi({ tc: "TC-N15-DOC-BASE", screen: "N15.S06", state: `chat-0, sổ của mình (${rieng ? "Chỉ mình tôi" : "Công khai"})`, action: "mở /diaries/[id]", cauHinh: cfg, expected: "bìa, các trang, dòng người đọc, «Sửa cuốn sổ», «Xóa cuốn sổ»; ảnh hiện; không tràn, không cắt", status: m && m.anh.length && m.anh.every((a) => a.nw > 0) && m.nut.some((n) => n.ten === "Sửa cuốn sổ") && (tt.tranTrangPx ?? 0) === 0 && (tt.chuBiCat ?? 0) === 0 ? "PASS" : "FAIL", evidence: [`EV-N15-DOC-${cfg}`], ghiChu: m ? `${m.anh.length} ảnh, ${m.anh.filter((a) => a.nw > 0).length} đã tải; nút ${m.nut.map((n) => n.ten).join(", ")}; dòng người đọc «${(m.chu.match(/(Chỉ mình tôi|Công khai)( · Có Nếp giúp viết)?/) ?? [])[0] ?? "-"}»; tràn trang ${tt.tranTrangPx ?? "?"}px, chữ bị cắt ${tt.chuBiCat ?? "?"}` : "không thấy màn đọc" });
        await t.context.close();
      }
      if (rieng && chayPhan("doc", "ngoai")) {
        const t = await mo("C1", NGOAI, `/diaries/${id}`);
        const cdp = await cdpCua(t.page);
        await t.page.waitForTimeout(2500);
        await choOn(t.page, { mang: t.mang });
        const m = await doMan(t.page, "diary-reader-screen");
        await anh(t.page, "EV-N15-DOC-NGOAI-RIENG-C1", t);
        const lo = /Kèo album retest|Hai ngày ở hồ|Một cuộc đi, những điều muốn giữ/.test(m?.chu ?? "") || (m?.anh.length ?? 0) > 0;
        const thu = await nut(t.page, "Thử lại");
        let sau = null;
        if (thu) {
          await cham(cdp, thu);
          await t.page.waitForTimeout(1500);
          sau = await doMan(t.page, "diary-reader-screen");
        }
        ghi({ tc: "TC-N15-DOC-NGOAI-RIENG", screen: "N15.S06", state: `${NGOAI} (ngoài nhóm), sổ riêng tư của chat-0`, action: "mở /diaries/[id]", cauHinh: "C1", expected: "không lộ chữ hay ảnh nào của sổ; một câu nói sổ đang được giữ riêng", status: !lo && /đang được giữ riêng/.test(m?.chu ?? "") ? "PASS" : "FAIL", evidence: ["EV-N15-DOC-NGOAI-RIENG-C1"], ghiChu: `chữ trang «${(m?.chu ?? "-").slice(0, 200)}»; ảnh ${m?.anh.length ?? 0}; lộ nội dung: ${lo ? "có" : "không"}` });
        ghi({ tc: "TC-N15-DOC-NGOAI-THU-LAI", screen: "N15.S06", state: `${NGOAI}, sổ riêng tư của chat-0 (404 diary_not_found)`, action: "đọc khối lỗi; chạm «Thử lại»", cauHinh: "C1", expected: "không có «Thử lại» cho một sổ không dành cho mình (thử lại không đổi được gì); tiêu đề không nói «chưa mở được» như lỗi tạm", status: !thu ? "PASS" : "FAIL", evidence: ["EV-N15-DOC-NGOAI-RIENG-C1"], ghiChu: `tiêu đề khối lỗi «${(m?.loi?.chu ?? "-").slice(0, 60)}»; «Thử lại»: ${thu ? "có" : "không"}${sau ? `; sau khi chạm: «${(sau.loi?.chu ?? sau.chu).slice(0, 120)}»` : ""}` });
        await t.context.close();
      }
    }
  }

  // ------------------------------------------------------------------- hep
  // The editor at the two narrow sizes the baseline asks for (C2 320, C3 360 dark), reached the way
  // an owner gets back to a kept book: reader → «Sửa cuốn sổ» → «Sửa theo cách mình nhớ». Read only:
  // nothing is saved, the context closes on the editor (leaving drops the edit, TC-N15-ROI-MAT-SUA).
  if (chay("hep")) {
    const id = await sachCua("chat-0");
    if (!id) console.log("hep: chat-0 chưa có sổ; chạy --chi luu trước");
    else
      for (const cfg of ["C2", "C3"]) {
        if (!chayPhan("hep", cfg)) continue;
        const t = await mo(cfg, "chat-0", `/diaries/${id}`);
        const cdp = await cdpCua(t.page);
        await t.page.waitForTimeout(2500);
        await choOn(t.page, { mang: t.mang });
        const s = await bam(t, cdp, "Sửa cuốn sổ", { cho: 2500 });
        const m0 = await denSo(t, cdp);
        const v = await bam(t, cdp, "Sửa theo cách mình nhớ");
        const { m } = await choPha(t.page, ["sua"], 5000);
        await cuonToi(t.page, 0);
        await t.page.waitForTimeout(300);
        const cap = await anh(t.page, `EV-N15-SUA-BASE-${cfg}`, t);
        // The first page's tools (name field, photo slots, buttons): where a narrow width bites first.
        await t.page.evaluate(() => [...document.querySelectorAll('[aria-label="Tên trang"]')].filter((x) => x.getClientRects().length)[0]?.scrollIntoView({ block: "start", behavior: "instant" }));
        await t.page.waitForTimeout(300);
        const cap2 = await anh(t.page, `EV-N15-SUA-BASE-TRANG-${cfg}`, t);
        const tt = [cap?.tomTat ?? {}, cap2?.tomTat ?? {}];
        const so = (x) => `tràn trang ${x.tranTrangPx ?? "?"}px, chữ bị cắt ${x.chuBiCat ?? "?"}, ellipsis ${x.ellipsis ?? "?"}, vùng bấm < 48: ${x.vungBamNho48 ?? "?"} (< 44: ${x.vungBamNho44 ?? "?"})`;
        ghi({ tc: "TC-N15-SUA-BASE", screen: "N15.S04", state: "chat-0, sổ đã lưu (Chỉ mình tôi), chế độ sửa", action: "/diaries/[id] → «Sửa cuốn sổ» → «Sửa theo cách mình nhớ»; chụp đầu màn và trang đầu", cauHinh: cfg, expected: "chế độ sửa hiển thị đủ ở màn hẹp: tên sổ, ảnh bìa, từng trang với tên, ảnh và nút; không tràn, không cắt chữ", status: s && v && pha(m) === "sua" && tt.every((x) => x.tranTrangPx === 0 && x.chuBiCat === 0) ? "PASS" : "FAIL", evidence: [`EV-N15-SUA-BASE-${cfg}`, `EV-N15-SUA-BASE-TRANG-${cfg}`], ghiChu: `«Sửa cuốn sổ»: ${s ? "chạm" : "không thấy"}; pha sau đó ${pha(m0)}; vào sửa: ${v ? "có" : "không"} (pha ${pha(m)}); đầu màn: ${so(tt[0])}; trang đầu: ${so(tt[1])}` });
        await t.context.close();
      }
  }

  // ------------------------------------------------------------- cong-khai
  // Public, then shared to the community (approved by the operator), then back to private:
  // what an outsider can read at each step, the book and the post.
  if (chay("cong-khai")) {
    const id = await sachCua("chat-0");
    if (!id) console.log("cong-khai: chat-0 chưa có sổ; chạy --chi luu trước");
    else {
      const p0 = await phienCua("chat-0");
      const pN = await phienNgoai();
      // The cover is named by its owner: once the book is private the outsider gets no document to read it from.
      const docN = async () => {
        const d = await goi("GET", `/diaries/${id}`, undefined, pN);
        const bia = (await goi("GET", `/diaries/${id}`, undefined, p0)).json?.document?.cover_id;
        const a = bia ? await fetch(`${mt.api}/diaries/${id}/photos/${bia}`, { headers: { authorization: `Bearer ${pN.token}` } }) : null;
        return { doc: d.status, anh: a?.status ?? null, loaiAnh: a?.headers.get("content-type") ?? null };
      };
      let d0 = (await goi("GET", `/diaries/${id}`, undefined, p0)).json;
      if (d0?.audience !== "public" && chayPhan("cong-khai", "dang")) {
        const t = await mo("C1", "chat-0", `/outings/${O1.id}/ending`);
        const cdp = await cdpCua(t.page);
        await denSo(t, cdp);
        const seg = await nut(t.page, "Công khai");
        if (seg) await cham(cdp, seg);
        await t.page.waitForTimeout(600);
        const m = await doMan(t.page);
        const canh = /sẽ ra ngoài hội, kể cả ảnh bạn bè đã đăng/.test(m?.chu ?? "");
        await anh(t.page, "EV-N15-CONG-KHAI-CANH-C1", t);
        const p = await nut(t.page, "Đăng sổ công khai");
        if (p) await cham(cdp, p);
        const r = await choPha(t.page, ["da-luu", "loi"], 15_000);
        await anh(t.page, "EV-N15-CONG-KHAI-C1", t);
        d0 = (await goi("GET", `/diaries/${id}`, undefined, p0)).json;
        const n = await docN();
        ghi({ tc: "TC-N15-CONG-KHAI", screen: "N15.S04", state: "chat-0, sổ riêng tư đã lưu", action: "chọn «Công khai» → «Đăng sổ công khai»", cauHinh: "C1", expected: "trước khi đăng có câu nói ảnh và lời sẽ ra ngoài hội; sau khi đăng: sổ công khai, người ngoài nhóm đọc được sổ và ảnh bìa", status: canh && pha(r.m) === "da-luu" && d0?.audience === "public" && n.doc === 200 && n.anh === 200 ? "PASS" : "FAIL", evidence: ["EV-N15-CONG-KHAI-CANH-C1", "EV-N15-CONG-KHAI-C1"], ghiChu: `câu cảnh báo: ${canh ? "có" : "không"}; sau ${r.ms ?? "-"} ms: pha ${pha(r.m)}, «${(r.m?.chu.match(/Cuốn sổ đã có trên tường bạn[^.]*\./) ?? [])[0] ?? "-"}»; audience ${d0?.audience}, phiên bản ${d0?.revision}; ${NGOAI}: sổ ${n.doc}, ảnh bìa ${n.anh} (${n.loaiAnh ?? "-"})` });
        await t.context.close();
      }
      if (d0?.audience === "public" && chayPhan("cong-khai", "nguoi-ngoai")) {
        const t = await mo("C1", NGOAI, `/diaries/${id}`);
        await t.page.waitForTimeout(2500);
        await choOn(t.page, { mang: t.mang });
        const m = await doMan(t.page, "diary-reader-screen");
        await anh(t.page, "EV-N15-DOC-NGOAI-CONG-KHAI-C1", t);
        ghi({ tc: "TC-N15-DOC-NGOAI-CONG-KHAI", screen: "N15.S06", state: `${NGOAI} (ngoài nhóm), sổ công khai của chat-0`, action: "mở /diaries/[id]", cauHinh: "C1", expected: "đọc được bìa, trang và ảnh; không có nút của chủ sổ", status: m && m.anh.length && m.anh.every((a) => a.nw > 0) && !m.nut.some((n) => /Sửa cuốn sổ|Xóa cuốn sổ|Cất về riêng tư/.test(n.ten)) ? "PASS" : "FAIL", evidence: ["EV-N15-DOC-NGOAI-CONG-KHAI-C1"], ghiChu: m ? `${m.anh.length} ảnh, ${m.anh.filter((a) => a.nw > 0).length} đã tải; nút ${m.nut.map((n) => n.ten).join(", ") || "không"}; dòng «${(m.chu.match(/(Chỉ mình tôi|Công khai)( · Có Nếp giúp viết)?/) ?? [])[0] ?? "-"}»` : "không thấy màn đọc" });
        await t.context.close();
      }
      // Sent to the community for review, approved, then the book goes back to private.
      let bai = null;
      if (d0?.audience === "public" && chayPhan("cong-khai", "gui")) {
        const t = await mo("C1", "chat-0", `/diaries/${id}`);
        const cdp = await cdpCua(t.page);
        await t.page.waitForTimeout(2500);
        await choOn(t.page, { mang: t.mang });
        const g = await bam(t, cdp, "Gửi sổ lên cộng đồng để duyệt", { cho: 3000 });
        const den = await duongDan(t.page);
        const chu = (await chuTrang(t.page)).slice(0, 200);
        await anh(t.page, "EV-N15-GUI-CONG-DONG-C1", t);
        bai = den.match(/\/community\/posts\/([0-9a-f-]{36})/)?.[1] ?? null;
        ghi({ tc: "TC-N15-GUI-CONG-DONG", screen: "N15.S06", state: "chat-0, sổ công khai", action: "«Gửi sổ lên cộng đồng để duyệt»", cauHinh: "C1", expected: "tới bài cộng đồng của sổ, đang chờ duyệt", status: g && bai && /Đang chờ duyệt/.test(chu) ? "PASS" : "FAIL", evidence: ["EV-N15-GUI-CONG-DONG-C1"], ghiChu: `nút: ${g ? "có" : "không"}; tới ${anId(den)}; chữ «${chu}»` });
        await t.context.close();
      }
      if (!bai) bai = (await goi("GET", "/v2/community/feed?mode=mine", undefined, p0)).json?.posts?.find((x) => (x.diary?.id ?? x.diary?.diary_id ?? x.diary_id) === id)?.id ?? null;
      if (bai && chayPhan("cong-khai", "rut")) {
        const p15 = await phienCua("chat-15");
        const b = (await goi("GET", `/v2/community/posts/${bai}`, undefined, p0)).json;
        let duyet = null;
        if (b && b.status !== "approved") duyet = await goi("POST", `/v2/community/posts/${bai}/review`, { revision: b.revision, approve: true, reason: "Sổ chuyến đi tổng hợp, đúng chủ đề, không có thông tin riêng." }, p15);
        const nTruoc = await goi("GET", `/v2/community/posts/${bai}`, undefined, pN);
        const soTruoc = await docN();
        // Back to private from the reader.
        const t = await mo("C1", "chat-0", `/diaries/${id}`);
        const cdp = await cdpCua(t.page);
        await t.page.waitForTimeout(2500);
        await choOn(t.page, { mang: t.mang });
        const c = await bam(t, cdp, "Cất về riêng tư", { cho: 2500 });
        const m = await doMan(t.page, "diary-reader-screen");
        await anh(t.page, "EV-N15-CAT-RIENG-C1", t);
        await t.context.close();
        const soSau = await docN();
        const nSau = await goi("GET", `/v2/community/posts/${bai}`, undefined, pN);
        const anhBai = nSau.json?.media?.[0]?.url ?? nSau.json?.diary?.cover_url ?? null;
        ghi({ tc: "TC-N15-CAT-RIENG", screen: "N15.S06", state: "chat-0, sổ công khai đã gửi lên cộng đồng và được duyệt", action: "«Cất về riêng tư» ở màn đọc", cauHinh: "C1", expected: "người ngoài không còn đọc được sổ, ảnh của sổ, hay bài cộng đồng dựng từ sổ (ADR-0039: thu hồi hiển thị áp dụng cả byte ảnh)", status: c && /Chỉ mình tôi/.test(m?.chu ?? "") && soSau.doc === 404 && soSau.anh === 404 && nSau.status === 404 ? "PASS" : "FAIL", evidence: ["EV-N15-CAT-RIENG-C1"], ghiChu: `duyệt bài: ${duyet ? `${duyet.status}` : "đã duyệt từ trước"}; trước khi cất, ${NGOAI}: bài ${nTruoc.status}, sổ ${soTruoc.doc}, ảnh bìa ${soTruoc.anh}; nút «Cất về riêng tư»: ${c ? "có" : "không"}, màn sau «${(m?.chu.match(/(Chỉ mình tôi|Công khai)/) ?? [])[0] ?? "-"}»; sau khi cất, ${NGOAI}: sổ ${soSau.doc}, ảnh bìa ${soSau.anh}, bài cộng đồng ${nSau.status}${nSau.code ? ` ${nSau.code}` : ""}${anhBai ? `, ảnh bài ${anId(anhBai)}` : ""}` });
      }
    }
  }

  // ------------------------------------------------------- khoanh-khac-xoa
  // chat-1 keeps a book of their own, then deletes it from a cold link (nothing behind the reader).
  if (chay("khoanh-khac-xoa")) {
    let id = await sachCua("chat-1");
    if (!id && chayPhan("khoanh-khac-xoa", "giu")) {
      const t = await mo("C1", "chat-1", `/outings/${O1.id}/ending`);
      const cdp = await cdpCua(t.page);
      const m = await denSo(t, cdp);
      const p = await nut(t.page, "Lưu riêng tư");
      if (p) await cham(cdp, p);
      const r = await choPha(t.page, ["da-luu", "loi"], 15_000);
      id = await sachCua("chat-1");
      await anh(t.page, "EV-N15-THANH-VIEN-GIU-C1", t);
      ghi({ tc: "TC-N15-THANH-VIEN-GIU", screen: "N15.S04", state: `chat-1 (thành viên), «${KEO}» đã khép`, action: "mở /outings/[id]/ending → «Tự xếp trang, không gửi AI» → «Lưu riêng tư»", cauHinh: "C1", expected: "thành viên tự giữ một cuốn sổ riêng trên tường mình (ADR-0039)", status: pha(r.m) === "da-luu" && id ? "PASS" : "FAIL", evidence: ["EV-N15-THANH-VIEN-GIU-C1"], ghiChu: `pha trước ${pha(m)}; sau ${r.ms ?? "-"} ms: pha ${pha(r.m)}; sổ ${id ? "có" : "không"}` });
      await t.context.close();
    }
    if (id && chayPhan("khoanh-khac-xoa", "xoa")) {
      const p1 = await phienCua("chat-1");
      const t = await mo("C1", "chat-1", `/diaries/${id}`);
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(2500);
      await choOn(t.page, { mang: t.mang });
      const lichSu = await t.page.evaluate(() => history.length);
      await bam(t, cdp, "Xóa cuốn sổ", { cho: 1200 });
      const d = await dialogTren(t.page);
      const f = await focusHienTai(t.page);
      await anh(t.page, "EV-N15-XOA-HOI-C1", t);
      await bam(t, cdp, "Giữ sổ lại", { trongDialog: true, cho: 900 });
      const dThoi = await dialogTren(t.page);
      const conSo = (await goi("GET", `/diaries/${id}`, undefined, p1)).status;
      ghi({ tc: "TC-N15-XOA-HOI", screen: "N15.S06", layer: "L-so-xoa", state: "chat-1, sổ riêng tư của mình, mở thẳng bằng đường dẫn", action: "«Xóa cuốn sổ» → «Giữ sổ lại»", cauHinh: "C1", expected: "hỏi trước trong một sheet có tên, tiêu điểm vào trong; «Giữ sổ lại» đóng sheet, sổ còn", status: d?.ten === "Xóa cuốn sổ" && f.trongDialog && !dThoi && conSo === 200 ? "PASS" : "FAIL", evidence: ["EV-N15-XOA-HOI-C1"], ghiChu: `sheet «${d?.ten ?? "-"}» «${(d?.chu ?? "").slice(0, 140)}»; tiêu điểm «${f.ten}» (trong dialog ${f.trongDialog}); sau «Giữ sổ lại»: ${dThoi ? "sheet còn" : "đóng"}; sổ ${conSo}` });
      await bam(t, cdp, "Xóa cuốn sổ", { cho: 1200 });
      const x = await bam(t, cdp, "Xóa cuốn sổ", { trongDialog: true, cho: 3000 });
      const den = anId(await duongDan(t.page));
      const chu = (await chuTrang(t.page)).slice(0, 120);
      await anh(t.page, "EV-N15-XOA-C1", t);
      const sau = (await goi("GET", `/diaries/${id}`, undefined, p1)).status;
      ghi({ tc: "TC-N15-XOA-LANH", screen: "N15.S06", layer: "L-so-xoa", state: `chat-1, sổ mở thẳng bằng đường dẫn (history.length ${lichSu}, không có màn phía sau)`, action: "«Xóa cuốn sổ» → xác nhận «Xóa cuốn sổ»", cauHinh: "C1", expected: "sổ bị xoá; người dùng rời khỏi cuốn sổ đã xoá, về tab Cá nhân (sửa ở bàn giao 28/09)", status: x && sau === 404 && !/\/diaries\//.test(den) ? "PASS" : "FAIL", evidence: ["EV-N15-XOA-C1"], ghiChu: `xác nhận: ${x ? "chạm" : "không thấy"}; tới ${den}; chữ «${chu}»; GET sổ sau: ${sau}` });
      await t.context.close();
    }
  }

  // ----------------------------------------------------------- khong-phien
  if (chay("khong-phien")) {
    const id = await sachCua("chat-0");
    const kq = [];
    for (const [nhan, path] of [["SO", id ? `/diaries/${id}` : null], ["KHEP", `/outings/${O1.id}/ending`]]) {
      if (!path) continue;
      const t = await mo("C1", null, path);
      await t.page.waitForTimeout(2500);
      await choOn(t.page, { mang: t.mang });
      const den = anId(await duongDan(t.page));
      const chu = (await chuTrang(t.page)).slice(0, 140);
      await anh(t.page, `EV-N15-KHONG-PHIEN-${nhan}-C1`, t);
      let sauDangNhap = null;
      if (nhan === "SO" && chayPhan("khong-phien", "dang-nhap")) {
        // Sign in the way a person would, from where the link left them (the number stays in memory).
        const cdp = await cdpCua(t.page);
        const cta = await tamCua(t.page, '[data-testid="welcome-cta"]');
        if (cta) await cham(cdp, cta);
        await t.page.waitForURL("**/login", { timeout: 10_000 }).catch(() => undefined);
        await choOn(t.page, { mang: t.mang });
        await t.page.locator('input[aria-label="Ô số điện thoại"]').fill(personaTheoTen("chat-0", mt.chatSessions).phone);
        const gui = await tamCua(t.page, '[data-testid="login-gui-ma"]');
        if (gui) await cham(cdp, gui);
        await t.page.waitForURL("**/otp", { timeout: 10_000 }).catch(() => undefined);
        await choOn(t.page, { mang: t.mang });
        await t.page.keyboard.type("000000", { delay: 40 });
        await t.page.waitForURL((u) => !/\/otp$/.test(u.pathname), { timeout: 12_000 }).catch(() => undefined);
        await t.page.waitForTimeout(2500);
        await choOn(t.page, { mang: t.mang });
        sauDangNhap = anId(await duongDan(t.page));
        await anh(t.page, "EV-N15-KHONG-PHIEN-SAU-DANG-NHAP-C1", t);
      }
      kq.push({ nhan, path: anId(path), den, chu, sauDangNhap });
      await t.context.close();
    }
    ghi({ tc: "TC-N15-KHONG-PHIEN", screen: "N15.S06", state: "không phiên (chưa đăng nhập hay phiên hết), mở link sổ và link khép kèo", action: "mở thẳng /diaries/[id], /outings/[id]/ending", cauHinh: "C1", expected: "nói cần đăng nhập để mở cuốn sổ, và đăng nhập xong trở lại đúng cuốn sổ", status: kq.every((k) => /\/(diaries|outings)\//.test(k.sauDangNhap ?? k.den)) ? "PASS" : "FAIL", evidence: [...kq.map((k) => `EV-N15-KHONG-PHIEN-${k.nhan}-C1`), ...(kq.some((k) => k.sauDangNhap) ? ["EV-N15-KHONG-PHIEN-SAU-DANG-NHAP-C1"] : [])], ghiChu: kq.map((k) => `${k.path} → ${k.den}: «${k.chu.slice(0, 90)}»${k.sauDangNhap ? `; đăng nhập bằng UI từ đó (chat-0): tới ${k.sauDangNhap}` : ""}`).join(" | ") });
  }

  // ------------------------------------------------------------------- loi
  if (chay("loi")) {
    const id = await sachCua("chat-0");
    for (const [nhan, path, mau, testid] of [["KHEP", `/outings/${O1.id}/ending`, new RegExp(`/outings/${O1.id}/ending$`), "diary-ending-screen"], ["DOC", id ? `/diaries/${id}` : null, new RegExp(`/diaries/${id}$`), "diary-reader-screen"]]) {
      if (!path || !chayPhan("loi", nhan.toLowerCase())) continue;
      const t = await mo("C1", "chat-0", "/profile");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(1500);
      await loiMayChu(t.page, mau);
      await diToi(t, path);
      await t.page.waitForTimeout(1200);
      const m = await doMan(t.page, testid);
      await anh(t.page, `EV-N15-LOI-${nhan}-C1`, t);
      await goHet(t.page);
      const thu = await bam(t, cdp, "Thử lại", { cho: 2500 });
      const m2 = await doMan(t.page, testid);
      ghi({ tc: `TC-N15-LOI-${nhan}`, screen: nhan === "KHEP" ? "N15.S02" : "N15.S06", state: `chat-0; GET ${nhan === "KHEP" ? "…/ending" : "/diaries/[id]"} trả 503 lần đầu`, action: "mở màn; rồi «Thử lại» khi máy chủ đã lành", cauHinh: "C1", expected: "câu lỗi nói Rủ Đi gặp sự cố (không đổ cho mạng hay app), «Thử lại» mở được màn", status: m?.loi && /sự cố/.test(m.loi.chu) && thu && !m2?.loi ? "PASS" : "FAIL", evidence: [`EV-N15-LOI-${nhan}-C1`], ghiChu: `lúc lỗi: «${(m?.loi?.chu ?? m?.chu ?? "-").slice(0, 170)}»; «Thử lại»: ${thu ? "chạm" : "không thấy"}; sau: ${m2?.loi ? `vẫn lỗi «${m2.loi.chu.slice(0, 80)}»` : `màn «${(m2?.chu ?? "-").slice(0, 60)}»`}` });
      await t.context.close();
    }
  }

  // -------------------------------------------------------------------- q4
  // Observation Q4: an expense counts towards every outing whose days hold its date.
  if (chay("q4")) {
    const p0 = await phienCua("chat-0");
    const docAlbum = async () => ((await goi("GET", `/contexts/${G.id}/albums`, undefined, p0)).json?.albums ?? []).filter((a) => [KEO, KEO_TRUNG].includes(a.title ?? a.outing?.title));
    let ks = (await goi("GET", `/contexts/${G.id}/outings`, undefined, p0)).json?.outings ?? [];
    let o3 = ks.find((k) => k.title === KEO_TRUNG) ?? null;
    let tao = null;
    if (!o3 && chayPhan("q4", "tao")) {
      tao = await goi("POST", `/contexts/${G.id}/outings`, { title: KEO_TRUNG, starts_on: "2026-09-29", ends_on: "2026-09-29", headcount: 20, budget_per_person_vnd: 300000 }, p0);
      ks = (await goi("GET", `/contexts/${G.id}/outings`, undefined, p0)).json?.outings ?? [];
      o3 = ks.find((k) => k.title === KEO_TRUNG) ?? null;
    }
    if (chayPhan("q4", "api")) {
      const al = await docAlbum();
      const raw = await goi("GET", `/contexts/${G.id}/albums`, undefined, p0);
      const tong = raw.json?.split_total_vnd ?? null;
      const moiKeo = al.map((a) => `«${a.title ?? a.outing?.title}» ${a.starts_on ?? a.outing?.starts_on}→${a.ends_on ?? a.outing?.ends_on}: split_total_vnd ${a.split_total_vnd}, expense_count ${a.expense_count}`);
      const rc = (await goi("GET", `/contexts/${G.id}/recap`, undefined, p0)).json;
      const rcTom = rc ? `recap: đã xong ${(rc.outings ?? []).map((o) => `«${o.title}» ${o.split_total_vnd}`).join(", ") || "không"}; đang đi ${(rc.in_progress ?? []).map((o) => `«${o.title}» ${o.split_total_vnd}`).join(", ") || "không"}; tổng các kèo đã xong ${rc.split_total_vnd}` : "recap: không đọc được";
      const trung = al.length === 2 && al.every((a) => a.split_total_vnd > 0 && a.split_total_vnd === al[0].split_total_vnd);
      ghi({ tc: "TC-N15-Q4-API", screen: "N15.S07", state: `nhóm chat-test: một khoản chi ngày 29/09 (tổng phân bổ 13.705.678đ); «${KEO}» 29–30/09 và «${KEO_TRUNG}» 29/09`, action: "GET /contexts/{id}/albums", cauHinh: "-", expected: "một khoản chi chỉ tính cho một cuộc đi (hoặc album nói rõ đây là chi tiêu của nhóm trong những ngày đó); tổng các album không cộng một khoản hai lần", status: o3 && trung ? "FAIL" : o3 ? "PASS" : "BLOCKED", method: o3 ? "RUNTIME-WEB" : "STATIC", ghiChu: `${tao ? `tạo «${KEO_TRUNG}»: ${tao.status}${tao.code ? ` ${tao.code}` : ""}; ` : ""}${moiKeo.join(" | ") || "không thấy album"}; tổng của danh sách: ${tong ?? "không có trường tổng"}; ${rcTom}` });
    }
    if (o3 && chayPhan("q4", "ke")) {
      const t = await mo("C1", "chat-0", "/messages");
      await t.page.waitForTimeout(1500);
      await diToi(t, `/groups/${G.id}/album`);
      await t.page.waitForTimeout(1000);
      const hang = await t.page.evaluate((ten) => {
        const cac = [...document.querySelectorAll('div[dir="auto"]')].filter((x) => x.getClientRects().length && ten.some((n) => (x.innerText ?? "").includes(n)));
        return cac.map((x) => {
          let h = x;
          for (let i = 0; i < 5 && h.parentElement && !/đã chia/.test(h.innerText ?? ""); i++) h = h.parentElement;
          const r = h.getBoundingClientRect();
          return { chu: (h.innerText ?? "").replace(/[-]/g, "").replace(/\s+/g, " ").trim().slice(0, 140), x: r.left, y: r.top, w: r.width, h: r.height };
        });
      }, [KEO, KEO_TRUNG]);
      const coChia = hang.filter((h) => /đã chia/.test(h.chu));
      await anh(t.page, "EV-N15-Q4-KE-C1", t, { chuThich: coChia.map((h) => ({ rect: { x: h.x, y: h.y, w: h.w, h: h.h } })) });
      ghi({ tc: "TC-N15-Q4-KE", screen: "N15.S07", state: `nhóm chat-test, kệ album; «${KEO}» và «${KEO_TRUNG}» cùng chứa ngày 29/09`, action: "mở /groups/[id]/album", cauHinh: "C1", expected: "mỗi khoản chi của nhóm chỉ hiện «đã chia» ở một cuộc đi", status: coChia.length >= 2 && coChia.every((h) => (h.chu.match(/đã chia\s?([\d.]+đ)/) ?? [])[1] === (coChia[0].chu.match(/đã chia\s?([\d.]+đ)/) ?? [])[1]) ? "FAIL" : "PASS", evidence: ["EV-N15-Q4-KE-C1"], ghiChu: hang.map((h) => `«${h.chu}»`).join(" | ") || "không thấy hàng của hai kèo" });
      await t.context.close();
    }
    // Once «Kèo album retest» has ended (Vietnam date after 30/09), both outings are finished: the recap
    // sums them and the settlement hero prints that sum. Measured only then; before it the part says so.
    const ngayVN = new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Ho_Chi_Minh" }).format(new Date());
    if (chayPhan("q4", "hero") && ngayVN <= "2026-09-30") console.log(`q4:hero: hôm nay ${ngayVN} giờ Việt Nam, «${KEO}» chưa xong; đo sau 0 giờ 01/10`);
    else if (chayPhan("q4", "hero")) {
      const rc = (await goi("GET", `/contexts/${G.id}/recap`, undefined, p0)).json;
      const xong = (rc?.outings ?? []).map((o) => `«${o.title}» ${o.split_total_vnd}`);
      const dangDi = (rc?.in_progress ?? []).map((o) => `«${o.title}» ${o.split_total_vnd}`);
      const t = await mo("C1", "chat-0", `/settlements/${G.id}`);
      await t.page.waitForTimeout(3000);
      await choOn(t.page, { mang: t.mang });
      const hero = await t.page.evaluate(() => {
        const man = [...document.querySelectorAll('[data-testid="settlement-screen"]')].filter((e) => e.getClientRects().length).pop();
        if (!man) return null;
        const nhan = [...man.querySelectorAll('div[dir="auto"]')].find((e) => /chuyến đã kết thúc|đang đi \(|Chi tiêu theo chuyến/.test(e.innerText ?? "") && (e.innerText ?? "").length < 120);
        if (!nhan) return { chu: (man.innerText ?? "").replace(/\s+/g, " ").slice(0, 200) };
        let khoi = nhan.parentElement;
        for (let i = 0; i < 3 && khoi && !/đ/.test((khoi.innerText ?? "").replace(nhan.innerText, "")); i++) khoi = khoi.parentElement;
        const r = (khoi ?? nhan).getBoundingClientRect();
        return { nhan: (nhan.innerText ?? "").trim(), chu: ((khoi ?? nhan).innerText ?? "").replace(/\s+/g, " ").trim().slice(0, 220), rect: { x: r.left, y: r.top, w: r.width, h: r.height } };
      });
      await anh(t.page, "EV-N15-Q4-HERO-C1", t, hero?.rect ? { chuThich: [{ rect: hero.rect }] } : {});
      const so = hero?.chu?.match(/(\d{1,3}(?:\.\d{3})+)đ/)?.[1] ?? null;
      const tongNhom = 13705678;
      const soNguyen = so ? Number(so.replace(/\./g, "")) : null;
      ghi({ tc: "TC-N15-Q4-HERO", screen: "N15.S07", state: `nhóm chat-test sau 30/09: «${KEO}» (29–30/09) và «${KEO_TRUNG}» (29/09) đều đã xong; cả nhóm có một khoản chi 13.705.678đ`, action: "GET /contexts/{id}/recap; mở /settlements/[id] (quyết toán của nhóm)", cauHinh: "C1", expected: "tổng «đã kết thúc» không vượt tổng chi của nhóm: một khoản chi không bị cộng hai lần", status: hero && soNguyen !== null ? (soNguyen <= tongNhom ? "PASS" : "FAIL") : "FAIL", evidence: ["EV-N15-Q4-HERO-C1"], ghiChu: `ngày đo ${ngayVN} (giờ Việt Nam); recap: đã xong ${xong.join(", ") || "không"}; đang đi ${dangDi.join(", ") || "không"}; tổng các kèo đã xong ${rc?.split_total_vnd ?? "-"}; hero «${hero?.chu ?? "không thấy"}»` });
      await t.context.close();
    }
  }
} finally {
  await mt.dong();
}
