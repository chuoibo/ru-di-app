/* Audit of a feature new on main 461eabf: the community (task #14, ADR-0040).
 * The first tab of the app on main: a moderated public feed in three modes
 * («Dành cho bạn», «Đang theo dõi», «Thịnh hành»), the post composer, post
 * detail with its management, Nếp and report sheets, comments, search, topic,
 * notifications, «Điều mình muốn giữ», and the operator's review queue.
 *
 *   source audit-env-main.sh && AUDIT_CORE_LOG=<core log> node kich-ban/n14-cong-dong.mjs [--chi api,rong,…]
 *
 * Order matters: `api` and `rong` measure the community with no approved public
 * post, which can be measured only before `duyet` approves the first one.
 *
 * Personas (chat-test seed on stack 2, read over the API before any write):
 *   chat-0   author (Chat Test 01); friend of chat-1 only
 *   chat-1   friend and reader (Chat Test 02), writes one public post
 *   chat-2   answers the personalisation invitation with «Cá nhân hóa»
 *   chat-3   answers it with «Để sau»
 *   chat-15  operator: granted `community_moderators` by SQL before `duyet`
 *            (docs/testing/cong-dong.md: an operator is granted the role explicitly)
 *   chat-16  stranger: not a friend of chat-0, no role
 * Posts are synthetic; each write checks what exists first so a rerun writes nothing twice.
 * The review queue is read and decided through the UI; nothing is ever reported.
 *
 * The local stack sets no MOBILE_CORS_ALLOW_ORIGINS, so the stream refuses the
 * page's origin (403). `ws` sections relay the page's socket through Node,
 * which connects without an Origin header: the frames are the server's own.
 * Rows are TC-N14-*; the placeholder TC-N-14-CONG-DONG is withdrawn by the verdict script.
 */
import { cauHinh } from "../thu-vien/cau-hinh.mjs";
import { chup, ghepAnh } from "../thu-vien/chup.mjs";
import { cdpCua, cham } from "../thu-vien/cu-chi.mjs";
import { choOn, duongDan } from "../thu-vien/dieu-huong.mjs";
import { focusHienTai, inertConLai } from "../thu-vien/lop-phu.mjs";
import { chayAxe } from "../thu-vien/axe.mjs";
import { goHet, loiMayChu } from "../thu-vien/mang.mjs";
import { khoiDong, trangMoi } from "../thu-vien/moi-truong.mjs";
import { ganPhien, layPhien, personaTheoTen } from "../thu-vien/phien.mjs";
import { ghiSuKien, taoContext } from "../thu-vien/trinh-duyet.mjs";
import { moLanh, theoDoiMang } from "../thu-vien/dieu-huong.mjs";
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
  so.ghi({ feature: "N14", nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, evidence: [], ...rec });
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
/** Lines of the core log that record the NOT NULL violation (read only; optional). */
const dem23502 = () => {
  const f = process.env.AUDIT_CORE_LOG;
  if (!f || !existsSync(f)) return null;
  return readFileSync(f, "utf8").split("\n").filter((l) => l.includes("sqlstate=23502")).length;
};

// The synthetic posts, found again by their opening words.
const BAI = {
  B1: { ten: "chat-0", body: "Sáng thứ Bảy ghé quán cà phê nhỏ ở dốc Nhà Làng, gọi một ly sữa đậu nành nóng rồi ngồi nhìn sương tan dần trên mái nhà.", topics: "cà phê, đà lạt" },
  B2: {
    ten: "chat-0",
    body: "Chiều hôm đó cả nhóm đi bộ dọc bờ hồ từ lúc nắng còn gắt, dừng ở mấy quán nước ven đường, chụp vài tấm ảnh rồi lại đi tiếp. Tới khi mặt trời xuống thấp thì cả mặt hồ chuyển sang màu cam, gió bắt đầu lạnh và ai cũng im lặng một lúc lâu. Lần sau nhất định sẽ quay lại đúng giờ này, mang thêm áo khoác và một bình trà nóng.",
    topics: "đi bộ, hoàng hôn",
  },
  B3: { ten: "chat-0", body: "Tối nay ăn bánh căn ở chợ, hẹn cả nhà tuần sau đi tiếp một vòng.", topics: "" },
  B4: { ten: "chat-1", body: "Cung đường đi bộ quanh đồi thông buổi sớm, sương dày tới mức chỉ thấy vài bước chân phía trước.", topics: "đi bộ" },
};
const dauBai = (k) => BAI[k].body.slice(0, 40);
/** Readers defined with one section and used by another (filled as the sections' blocks run). */
const hoTro = {};
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
/** The author's own posts (mode mine), or [] when the feed cannot be read. */
const baiCua = async (ten) => {
  const r = await goi("GET", "/v2/community/feed?mode=mine", undefined, await phienCua(ten));
  return r.status === 200 ? r.json.posts ?? [] : [];
};
const timBai = async (k) => (await baiCua(BAI[k].ten)).find((p) => p.body.startsWith(dauBai(k))) ?? null;

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
 * The community screen as drawn: title and caption, the three tabs with the
 * state a screen reader gets, the invitation box, the error box, the empty
 * state, and every post card on screen.
 */
const doBangTin = (page) =>
  page.evaluate(() => {
    const hien = (e) => e && e.getClientRects().length > 0 && e.checkVisibility({ opacityProperty: true });
    const chu = (e) => (e?.innerText ?? "").replace(/[-]/g, "").replace(/\s+/g, " ").trim();
    const man = [...document.querySelectorAll('[data-testid="community-screen"]')].filter(hien).pop();
    if (!man) return null;
    const cac = [...man.querySelectorAll('div[dir="auto"]')].filter(hien).map((x) => chu(x));
    const tabs = [...man.querySelectorAll('[role="tab"]')].filter(hien).map((t) => {
      const r = t.getBoundingClientRect();
      return { ten: chu(t), ariaSelected: t.getAttribute("aria-selected"), ariaPressed: t.getAttribute("aria-pressed"), mau: getComputedStyle(t.querySelector('div[dir="auto"]') ?? t).color, x: Math.round(r.left), y: Math.round(r.top), w: Math.round(r.width), h: Math.round(r.height) };
    });
    const alert = [...man.querySelectorAll('[role="alert"]')].filter(hien).map((a) => ({ chu: chu(a), y: Math.round(a.getBoundingClientRect().top) }));
    const the = [...man.querySelectorAll('[data-testid^="community-post-"]')].filter((e) => hien(e) && /^community-post-[0-9a-f-]{36}$/.test(e.dataset.testid)).map((e) => {
      const r = e.getBoundingClientRect();
      return { id: e.dataset.testid.slice(15), top: Math.round(r.top), cao: Math.round(r.height), dau: chu(e).slice(0, 80) };
    });
    const trong = cac.find((x) => /^(Một ngày đáng kể|Câu chuyện bắt đầu từ một người)$/.test(x)) ?? null;
    const moi = cac.includes("Một góc hợp với bạn");
    const phu = cac.find((x) => /^(Những câu chuyện đang tiếp nối|Kết nối những cuộc đi)$/.test(x)) ?? null;
    const tieuDe = cac[0] ?? null;
    const thuLai = [...man.querySelectorAll('[role="button"]')].some((b) => hien(b) && chu(b) === "Thử lại");
    const keTruoc = [...man.querySelectorAll('[role="button"]')].some((b) => hien(b) && chu(b) === "Kể khoảnh khắc đầu tiên");
    const dai = [...man.querySelectorAll('[role="button"]')].filter((b) => hien(b) && chu(b) === "Bảng tin có cập nhật").map((b) => {
      const r = b.getBoundingClientRect();
      return { x: Math.round(r.left), y: Math.round(r.top), w: Math.round(r.width), h: Math.round(r.height) };
    })[0] ?? null;
    return { tieuDe, phu, tabs, alert, trong, keTruoc, moi, thuLai, the, dai };
  });
/** Every WebSocket the page opens: its handshake fate and how many frames arrived. */
const theoDoiWs = (page) => {
  const log = [];
  page.on("websocket", (ws) => {
    const m = { url: new URL(ws.url()).pathname, mo: Date.now(), dong: null, loi: null, khung: 0 };
    log.push(m);
    ws.on("close", () => (m.dong = Date.now()));
    ws.on("socketerror", (e) => (m.loi = String(e).slice(0, 90)));
    ws.on("framereceived", () => (m.khung += 1));
  });
  return log;
};
const tomWs = (log) => (log.length ? `${log.length} lần mở ${[...new Set(log.map((m) => m.url))].join(", ")}; ${log.filter((m) => m.khung > 0).length} lần nhận khung; lỗi: ${[...new Set(log.map((m) => m.loi).filter(Boolean))].join(" | ") || "không"}` : "không mở WebSocket nào");
/** Feed reads the page made, with their statuses. */
const theoDoiDoc = (page, mau = /\/v2\/community\/feed\?/) => {
  const log = [];
  page.on("response", (r) => {
    if (mau.test(r.url())) log.push({ mode: new URL(r.url()).searchParams.get("mode"), status: r.status() });
  });
  return log;
};

try {
  // ------------------------------------------------------------------ api
  // The feed contract with no approved public post: what each mode answers.
  if (chay("api")) {
    const p0 = await phienCua("chat-0");
    const truoc = dem23502();
    const kq = {};
    for (const mode of ["for_you", "following", "trending", "mine", "saved"]) kq[mode] = await goi("GET", `/v2/community/feed?mode=${mode}`, undefined, p0);
    const pref = await goi("GET", "/v2/community/preferences", undefined, p0);
    const sau = dem23502();
    const ba = ["for_you", "following", "trending"];
    const cong = await baiCua("chat-0");
    console.log(JSON.stringify({ kq: Object.fromEntries(Object.entries(kq).map(([k, v]) => [k, `${v.status} ${v.code ?? ""} ${v.json?.posts?.length ?? ""}`])), pref: pref.json }));
    ghi({ tc: "TC-N14-API-RONG", screen: "N14.S01", state: `cộng đồng chưa có bài công khai nào được duyệt (bài của chat-0: ${cong.length}); chat-0 chưa trả lời lời mời cá nhân hoá (asked=${pref.json?.asked})`, action: "GET /v2/community/feed với năm mode, GET /v2/community/preferences (chỉ đọc)", cauHinh: "-", expected: "ba mode của bảng tin (for_you, following, trending) trả 200 với danh sách rỗng; không lỗi máy chủ", status: ba.every((m) => kq[m].status === 200) ? "PASS" : "FAIL", ghiChu: `${Object.entries(kq).map(([k, v]) => `${k} ${v.status}${v.code ? ` ${v.code}` : ""}${v.status === 200 ? ` (${v.json?.posts?.length ?? 0} bài)` : ""}`).join("; ")}; preferences ${pref.status} asked=${pref.json?.asked} personalized=${pref.json?.personalized}; dòng «sqlstate=23502» trong log core: ${truoc ?? "không đọc"} → ${sau ?? "không đọc"}` });
  }

  // ------------------------------------------------------------------ rong
  // The first tab of the app with an empty community, as chat-0 meets it.
  if (chay("rong")) {
    const anhRong = [];
    for (const cfg of ["C1", "C2", "C3"]) {
      if (!chayPhan("rong", cfg)) continue;
      const t = await mo(cfg, "chat-0", "/community");
      const ws = theoDoiWs(t.page);
      const doc = theoDoiDoc(t.page);
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(2500);
      await choOn(t.page, { mang: t.mang });
      const truoc23502 = dem23502();
      const m = await doBangTin(t.page);
      const den = await duongDan(t.page);
      // The first read happens inside `mo()`, before `doc` listens: its failure is in the page's HTTP log.
      const loiDau = t.suKien.log.http.filter((h) => /\/v2\/community\/feed$/.test(h.url)).map((h) => `${h.method} feed ${h.status}`);
      await anh(t.page, `EV-N14-RONG-${cfg}`, t, { chuThich: m?.alert.length ? [{ selector: '[data-testid="community-screen"] [role="alert"]' }] : [] });
      anhRong.push({ id: `EV-N14-RONG-${cfg}`, nhan: cfg });
      const dat = m && m.trong && m.keTruoc && !m.alert.length;
      ghi({ tc: "TC-N14-RONG-BANG-TIN", screen: "N14.S01", state: "chat-0, cộng đồng chưa có bài công khai được duyệt, chưa trả lời lời mời cá nhân hoá", action: "mở tab Cộng đồng (tab đầu), «Dành cho bạn»", cauHinh: cfg, expected: "trạng thái rỗng «Một ngày đáng kể» với «Kể khoảnh khắc đầu tiên»; không câu lỗi, không «Thử lại»", status: dat ? "PASS" : "FAIL", evidence: [`EV-N14-RONG-${cfg}`], ghiChu: m ? `tới ${den}; tiêu đề «${m.tieuDe}», dòng phụ «${m.phu}»; tab ${m.tabs.map((x) => x.ten).join(" / ")}; hộp mời cá nhân hoá: ${m.moi ? "có" : "không"}; câu lỗi: ${m.alert.map((a) => `«${a.chu}» ở y ${a.y}`).join(", ") || "không"}; trạng thái rỗng: ${m.trong ?? "không"}; «Kể khoảnh khắc đầu tiên»: ${m.keTruoc ? "có" : "không"}; lần đọc lúc mở trả lỗi: ${loiDau.join(", ") || "không"}; WebSocket: ${tomWs(ws)}` : `không thấy màn cộng đồng, tới ${den}` });
      if (cfg === "C1") {
        // «Thử lại»: another read, the same answer?
        const soDoc = doc.length;
        const thu = await bam(t, cdp, "Thử lại", { cho: 2500 });
        const m2 = await doBangTin(t.page);
        const moiDoc = doc.slice(soDoc);
        ghi({ tc: "TC-N14-RONG-THU-LAI", screen: "N14.S01", state: "chat-0, bảng tin «Dành cho bạn» đang báo lỗi", action: "chạm «Thử lại»", cauHinh: cfg, expected: "đọc lại bảng tin và ra trạng thái rỗng (không lỗi nào tự hết mà người dùng chờ được)", status: thu && m2?.trong && !m2.alert.length ? "PASS" : "FAIL", evidence: [`EV-N14-RONG-${cfg}`], ghiChu: `«Thử lại»: ${thu ? `chạm (${thu.w}×${thu.h})` : "không thấy"}; lần đọc sau khi chạm: ${moiDoc.map((d) => `${d.mode} ${d.status}`).join(", ") || "không"}; sau đó câu lỗi: ${m2?.alert.map((a) => `«${a.chu}»`).join(", ") || "không"}; trạng thái rỗng: ${m2?.trong ?? "không"}; dòng 23502 trong log core: ${truoc23502 ?? "?"} → ${dem23502() ?? "?"}` });
        // The two other tabs.
        for (const [tab, id, tc, kyVong] of [
          ["Đang theo dõi", "THEO-DOI", "TC-N14-RONG-THEO-DOI", "Câu chuyện bắt đầu từ một người"],
          ["Thịnh hành", "THINH-HANH", "TC-N14-RONG-THINH-HANH", "Một ngày đáng kể"],
        ]) {
          const n0 = doc.length;
          const cham1 = await bam(t, cdp, tab, { cho: 2500 });
          await choOn(t.page, { mang: t.mang });
          const mt2 = await doBangTin(t.page);
          await anh(t.page, `EV-N14-RONG-${id}-C1`, t);
          ghi({ tc, screen: "N14.S01", state: "chat-0, cộng đồng chưa có bài công khai được duyệt", action: `chạm tab «${tab}»`, cauHinh: cfg, expected: `trạng thái rỗng «${kyVong}», không câu lỗi`, status: cham1 && mt2?.trong === kyVong && !mt2.alert.length ? "PASS" : "FAIL", evidence: [`EV-N14-RONG-${id}-C1`], ghiChu: `tab: ${cham1 ? `${cham1.w}×${cham1.h}` : "không thấy"}; lần đọc: ${doc.slice(n0).map((d) => `${d.mode} ${d.status}`).join(", ") || "không"}; câu lỗi: ${mt2?.alert.map((a) => `«${a.chu}»`).join(", ") || "không"}; trạng thái rỗng: ${mt2?.trong ?? "không"}` });
        }
        // What a screen reader is told about the selected tab (DESIGN.md; UI-003 on the tab bar).
        const mt3 = await doBangTin(t.page);
        const chon = mt3?.tabs.find((x) => x.ten === "Thịnh hành");
        ghi({ tc: "TC-N14-TAB-ARIA", screen: "N14.S01", state: "tab «Thịnh hành» đang chọn", action: "đọc thuộc tính trợ năng của ba tab bảng tin", cauHinh: cfg, expected: "tab đang chọn có aria-selected=\"true\", hai tab kia \"false\" (role tab)", status: chon?.ariaSelected === "true" && mt3.tabs.filter((x) => x !== chon).every((x) => x.ariaSelected === "false") ? "PASS" : "FAIL", evidence: [`EV-N14-RONG-THINH-HANH-C1`], ghiChu: mt3 ? mt3.tabs.map((x) => `«${x.ten}» aria-selected=${x.ariaSelected} aria-pressed=${x.ariaPressed}, màu chữ ${x.mau}, ${x.w}×${x.h}`).join("; ") : "không thấy tab" });
        // The invitation box's two answers, measured (nobody answers here).
        const vung = await t.page.evaluate(() => {
          const hien = (e) => e && e.getClientRects().length > 0;
          return [...document.querySelectorAll('[role="button"]')].filter((b) => hien(b) && /^(Cá nhân hóa|Để sau)$/.test((b.getAttribute("aria-label") || b.innerText || "").trim())).map((b) => {
            const r = b.getBoundingClientRect();
            return { ten: (b.getAttribute("aria-label") || b.innerText).trim(), w: Math.round(r.width), h: Math.round(r.height) };
          });
        });
        ghi({ tc: "TC-N14-RONG-MOI", screen: "N14.S01", state: "chat-0 chưa trả lời lời mời cá nhân hoá", action: "đọc hộp «Một góc hợp với bạn»", cauHinh: cfg, expected: "hộp mời nói rõ dùng gì, có «Cá nhân hóa» và «Để sau», mỗi nút vùng bấm ≥ 48dp", status: m?.moi && vung.length === 2 && vung.every((v) => v.h >= 48) ? "PASS" : "FAIL", evidence: [`EV-N14-RONG-C1`], ghiChu: `hộp mời: ${m?.moi ? "có" : "không"}; nút: ${vung.map((v) => `«${v.ten}» ${v.w}×${v.h}`).join(", ") || "không thấy"}` });
        const axe = await chayAxe(t.page);
        ghi({ tc: "TC-N14-RONG-AXE", screen: "N14.S01", state: "chat-0, bảng tin rỗng đang báo lỗi", action: "axe-core trên tab Cộng đồng", cauHinh: cfg, expected: "không vi phạm serious/critical", status: axe.vi.filter((v) => ["serious", "critical"].includes(v.impact)).length === 0 ? "PASS" : "FAIL", ghiChu: `vi phạm: ${axe.vi.map((v) => `${v.rule}(${v.impact})×${v.so}: ${v.vd.map((x) => x.target).join(" ; ")}`).join(", ") || "không"}; contrast chưa rõ: ${axe.chuaRo.map((v) => v.so).reduce((a, b) => a + b, 0)}` });
      }
      await t.context.close();
    }
    if (anhRong.length === 3) await ghepAnh(mt.browser, anhRong.map((a) => ({ file: join(mt.out, "jpg", `${a.id}.jpg`), nhan: a.nhan })), join(mt.out, "jpg", "EV-N14-RONG-ghep.jpg"), { tieuDe: "N14 · tab Cộng đồng khi chưa có bài công khai nào được duyệt, chat-0, C1–C3" });

    // The two answers to the invitation, by two people who have not answered yet.
    for (const [ten, tra, id, tc] of [
      ["chat-2", "Cá nhân hóa", "CA-NHAN-HOA", "TC-N14-RONG-CA-NHAN-HOA"],
      ["chat-3", "Để sau", "DE-SAU", "TC-N14-RONG-DE-SAU"],
    ]) {
      if (!chayPhan("rong", id.toLowerCase())) continue;
      const pr = await goi("GET", "/v2/community/preferences", undefined, await phienCua(ten));
      if (pr.json?.asked) {
        console.log(`rong: ${ten} đã trả lời lời mời (asked=true), bỏ qua ${tc}`);
        continue;
      }
      const t = await mo("C1", ten, "/community");
      const doc = theoDoiDoc(t.page);
      const put = [];
      t.page.on("response", (r) => {
        if (/\/v2\/community\/preferences/.test(r.url()) && r.request().method() === "PUT") put.push(r.status());
      });
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(2500);
      await choOn(t.page, { mang: t.mang });
      const truoc = await doBangTin(t.page);
      const n0 = doc.length;
      const c = await bam(t, cdp, tra, { cho: 2800 });
      await choOn(t.page, { mang: t.mang });
      const sau = await doBangTin(t.page);
      await anh(t.page, `EV-N14-RONG-${id}-C1`, t);
      const docSau = doc.slice(n0);
      const n1 = doc.length;
      const th = await bam(t, cdp, "Thịnh hành", { cho: 2500 });
      const sauTh = await doBangTin(t.page);
      ghi({ tc, screen: "N14.S01", state: `${ten} (${tenHien(ten)}), cộng đồng chưa có bài công khai được duyệt, chưa trả lời lời mời`, action: `chạm «${tra}» trong hộp «Một góc hợp với bạn»; rồi tab «Thịnh hành»`, cauHinh: "C1", expected: "hộp mời biến mất; «Dành cho bạn» ra trạng thái rỗng «Một ngày đáng kể», không câu lỗi", status: c && !sau?.moi && sau?.trong && !sau.alert.length ? "PASS" : "FAIL", evidence: [`EV-N14-RONG-${id}-C1`], ghiChu: `trước: câu lỗi ${truoc?.alert.map((a) => `«${a.chu}»`).join(", ") || "không"}, hộp mời ${truoc?.moi ? "có" : "không"}; PUT preferences: ${put.join(", ") || "không"}; đọc lại: ${docSau.map((d) => `${d.mode} ${d.status}`).join(", ") || "không"}; sau: hộp mời ${sau?.moi ? "còn" : "hết"}, câu lỗi ${sau?.alert.map((a) => `«${a.chu}»`).join(", ") || "không"}, trạng thái rỗng ${sau?.trong ?? "không"}; «Thịnh hành»: ${th ? `đọc ${doc.slice(n1).map((d) => `${d.mode} ${d.status}`).join(", ")}, câu lỗi ${sauTh?.alert.map((a) => `«${a.chu}»`).join(", ") || "không"}, rỗng ${sauTh?.trong ?? "không"}` : "không thấy tab"}` });
      await t.context.close();
    }
  }

  // ------------------------------------------------------------------ khong-phien
  // Signed out (Q5): the tab, and the community's inner screens opened from a link.
  if (chay("khong-phien")) {
    const t = await mo("C1", null, "/community");
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(2200);
    await choOn(t.page, { mang: t.mang });
    const den = await duongDan(t.page);
    const chu = await chuTrang(t.page);
    const tabBar = await t.page.evaluate(() => [...document.querySelectorAll('[role="tablist"] [role="tab"], [role="tablist"] a, [role="tablist"] [role="button"]')].filter((x) => x.getClientRects().length).map((x) => (x.getAttribute("aria-label") || x.innerText || "").replace(/[-]/g, "").trim()));
    await anh(t.page, "EV-N14-KHONG-PHIEN-C1", t);
    const dn = await bam(t, cdp, "Đăng nhập", { cho: 2000 });
    const toi = await duongDan(t.page);
    ghi({ tc: "TC-N14-KHONG-PHIEN", screen: "N14.S01", state: "không phiên", action: "mở thẳng /community; chạm «Đăng nhập»", cauHinh: "C1", expected: "màn mời đăng nhập nói cộng đồng là gì, «Đăng nhập» tới /login", status: /Đăng nhập để gặp cộng đồng/.test(chu) && dn && /\/login$/.test(toi) ? "PASS" : "FAIL", evidence: ["EV-N14-KHONG-PHIEN-C1"], ghiChu: `tới ${den}; chữ «${chu.slice(0, 160)}»; thanh tab: ${tabBar.join(" / ") || "không thấy"}; «Đăng nhập»: ${dn ? `${dn.w}×${dn.h}` : "không thấy"} → ${toi}` });
    await t.context.close();
    // Inner screens from a cold link, signed out: each must say what is needed and offer a way in.
    const ra = [];
    for (const [path, id] of [
      [`/community/posts/${randomUUID()}`, "BAI"],
      ["/community/new", "VIET"],
      ["/community/notifications", "THONG-BAO"],
      ["/community/review", "DUYET"],
    ]) {
      const u = await mo("C1", null, path);
      await u.page.waitForTimeout(3000);
      await choOn(u.page, { mang: u.mang });
      const d = await duongDan(u.page);
      const c = await chuTrang(u.page);
      const lo = await nut(u.page, "Đăng nhập");
      await anh(u.page, `EV-N14-KHONG-PHIEN-${id}-C1`, u);
      ra.push({ path: anId(path), id, den: anId(d), chu: c.slice(0, 120), lo: !!lo });
      await u.context.close();
    }
    ghi({ tc: "TC-N14-KHONG-PHIEN-SAU", screen: "N14.S02", state: "không phiên", action: "mở thẳng bốn màn trong: chi tiết bài, viết bài, thông báo, hàng duyệt", cauHinh: "C1", expected: "mỗi màn nói cần đăng nhập và có lối «Đăng nhập» (hoặc chuyển về màn đăng nhập); không màn nào chờ mãi", status: ra.every((r) => r.lo || /\/(login|welcome)$/.test(r.den)) ? "PASS" : "FAIL", evidence: ra.map((r) => `EV-N14-KHONG-PHIEN-${r.id}-C1`), ghiChu: ra.map((r) => `${r.path} → ${r.den}: «${r.chu}»; «Đăng nhập» ${r.lo ? "có" : "không"}`).join(" | ") });
  }

  // ------------------------------------------------------------------ tu-kiem-ly-do
  // Self-check of the `lyDo` reader on screens whose answer is known from the code: the story form
  // without a photo carries «Chọn một tấm ảnh trước đã.» (DangStoryScreen.tsx:148, canary: must be
  // read); the community composer's send button carries none (identity: must read nothing).
  // Nothing is written to the ledger: a failure stops the run.
  if (chay("tu-kiem-ly-do")) {
    const doc1 = async (path, ten) => {
      const t = await mo("C1", "chat-0", path);
      await t.page.waitForTimeout(2500);
      await choOn(t.page, { mang: t.mang });
      const kq = await t.page.evaluate(
        ({ ten, src }) => {
          const lyDoCua = (0, eval)(src);
          const b = [...document.querySelectorAll('[role="button"]')].find((x) => x.getClientRects().length && (x.getAttribute("aria-label") || (x.innerText ?? "")).trim() === ten);
          return b ? { tat: b.getAttribute("aria-disabled") === "true", lyDo: lyDoCua(b) } : null;
        },
        { ten, src: LY_DO_SRC },
      );
      await t.context.close();
      return kq;
    };
    const canary = await doc1("/stories/new", "Đăng story");
    const identity = await doc1("/community/new", "Gửi lên cộng đồng");
    console.log(`tu-kiem-ly-do: canary «Đăng story» ${JSON.stringify(canary)}; identity «Gửi lên cộng đồng» ${JSON.stringify(identity)}`);
    if (canary?.lyDo !== "Chọn một tấm ảnh trước đã." || !identity || identity.lyDo !== null) throw new Error("tu-kiem-ly-do: bộ đọc lý do sai");
  }

  // ------------------------------------------------------------------ dang
  // The composer «Kể một khoảnh khắc», and the four synthetic posts. Its readers are shared with
  // `chi-tiet` (editing a post opens the same form).
  {
    /** The composer as drawn: fields, the audience radios, the send button and what it says when it cannot send. */
    const doForm = (page) =>
      page.evaluate((lyDoSrc) => {
        const hien = (e) => e && e.getClientRects().length > 0 && e.checkVisibility({ opacityProperty: true });
        const chu = (e) => (e?.innerText ?? "").replace(/[-]/g, "").replace(/\s+/g, " ").trim();
        const lyDoCua = (0, eval)(lyDoSrc);
        const man = [...document.querySelectorAll('[data-testid="community-composer"]')].filter(hien).pop();
        if (!man) return null;
        const body = man.querySelector('[data-testid="community-body"]');
        const radios = [...man.querySelectorAll('[role="radio"]')].filter(hien).map((r) => ({ ten: chu(r).replace(/ · Đã chọn.*$/, "").split(/(?<=Bạn bè|Cộng đồng|Chỉ mình tôi|Một nhóm)/)[0], chon: / · Đã chọn/.test(chu(r)), ariaChecked: r.getAttribute("aria-checked"), ariaSelected: r.getAttribute("aria-selected"), tat: r.getAttribute("aria-disabled") }));
        const gui = [...document.querySelectorAll('[role="button"]')].filter((b) => hien(b) && /^(Gửi lên cộng đồng|Đăng lên tường|Gửi bản sửa|Đang gửi…)$/.test(chu(b))).pop();
        const rg = gui?.getBoundingClientRect();
        const loi = [...man.querySelectorAll('[role="alert"]')].filter(hien).map(chu);
        // Where the error sentence is, and whether a person looking at the screen can see it.
        const loiO = [...man.querySelectorAll('[role="alert"]')].filter(hien).map((a) => {
          const r = a.getBoundingClientRect();
          const tren = document.elementFromPoint(r.left + Math.min(20, r.width / 2), r.top + r.height / 2);
          return { top: Math.round(r.top), day: Math.round(r.bottom), thay: r.top >= 0 && r.bottom <= innerHeight && !!tren && (tren === a || a.contains(tren)) };
        });
        const chuDe = man.querySelector('[aria-label="Chủ đề"]');
        const nhom = [...man.querySelectorAll('[role="button"]')].filter(hien).map(chu).filter((x) => !/^(Thêm ảnh hoặc video|Tag bạn bè|Lựa chọn khác|Thu gọn lựa chọn|Quay lại|Bỏ tệp|Đang chuẩn bị tệp…)$/.test(x) && x.length);
        const cac = [...man.querySelectorAll('div[dir="auto"]')].filter(hien).map(chu);
        return {
          tieuDe: cac.find((x) => /^(Kể một khoảnh khắc|Viết tiếp câu chuyện)$/.test(x)) ?? null,
          body: body ? { gt: body.value, cao: Math.round(body.getBoundingClientRect().height) } : null,
          chuDe: chuDe ? chuDe.value : null,
          goiY: cac.find((x) => /^Tối đa 5 chủ đề/.test(x)) ?? null,
          radios,
          gui: gui ? { ten: chu(gui), tat: gui.getAttribute("aria-disabled") === "true", top: Math.round(rg.top), day: Math.round(rg.bottom), cao: Math.round(rg.height), lyDo: lyDoCua(gui), vien: getComputedStyle(gui).borderTopStyle } : null,
          loi,
          loiO,
          anh: [...man.querySelectorAll("img")].filter(hien).map((i) => ({ w: Math.round(i.getBoundingClientRect().width), h: Math.round(i.getBoundingClientRect().height) })),
          boTep: [...man.querySelectorAll('[role="button"]')].filter((b) => hien(b) && chu(b) === "Bỏ tệp").length,
          dangChuanBi: cac.includes("Đang chuẩn bị tệp…"),
          nutKhac: nhom.slice(0, 12),
          cuaSo: { w: innerWidth, h: innerHeight },
        };
      }, LY_DO_SRC);
    /** The post detail as drawn: the card, its status band and audience, images, the comment box. */
    const doChiTietNhanh = (page) =>
      page.evaluate(() => {
        const hien = (e) => e && e.getClientRects().length > 0 && e.checkVisibility({ opacityProperty: true });
        const chu = (e) => (e?.innerText ?? "").replace(/[-]/g, "").replace(/\s+/g, " ").trim();
        const man = [...document.querySelectorAll('[data-testid="community-post-detail"]')].filter(hien).pop();
        if (!man) return null;
        const the = [...man.querySelectorAll('[data-testid^="community-post-"]')].find((e) => /^community-post-[0-9a-f-]{36}$/.test(e.dataset.testid));
        const cac = [...(the ?? man).querySelectorAll('div[dir="auto"]')].filter(hien).map(chu);
        return {
          id: the?.dataset.testid.slice(15) ?? null,
          meta: cac.find((x) => /· (Cộng đồng|Bạn bè|Trong nhóm|Chỉ mình tôi)$/.test(x)) ?? null,
          dai: cac.find((x) => /^(Đang chờ duyệt|Chưa phù hợp cộng đồng)/.test(x)) ?? null,
          than: cac.find((x) => x.length > 30) ?? null,
          anh: [...(the ?? man).querySelectorAll("img")].filter(hien).map((i) => ({ w: Math.round(i.getBoundingClientRect().width), h: Math.round(i.getBoundingClientRect().height) })),
          loi: [...man.querySelectorAll('[role="alert"]')].filter(hien).map(chu),
          chuDe: [...(the ?? man).querySelectorAll('[role="button"]')].filter(hien).map(chu).filter((x) => /^[a-zà-ỹđ ]{2,40}$/.test(x) && !/^(Chia sẻ|Thích|Bình luận)$/.test(x)),
        };
      });
    const cho = async (page, dk, ms = 12_000) => {
      const t0 = Date.now();
      while (Date.now() - t0 < ms) {
        if (await dk()) return Date.now() - t0;
        await page.waitForTimeout(200);
      }
      return null;
    };
    const guiBai = async (t, cdp) => {
      const g = await nut(t.page, "Gửi lên cộng đồng") ?? (await nut(t.page, "Đăng lên tường"));
      if (!g || g.tat) return { g, den: await duongDan(t.page) };
      await cham(cdp, g);
      const ms = await cho(t.page, async () => /\/community\/posts\/[0-9a-f-]{36}$/.test(await duongDan(t.page)));
      await t.page.waitForTimeout(1500);
      await choOn(t.page, { mang: t.mang });
      return { g, ms, den: await duongDan(t.page) };
    };
    const moForm = async (t, cdp) => {
      if (!/\/community$/.test(await duongDan(t.page))) await diToi(t, "/community");
      const mo1 = await bam(t, cdp, "Đăng khoảnh khắc", { cho: 2200 });
      await choOn(t.page, { mang: t.mang });
      return mo1;
    };

    hoTro.doForm = doForm;
    hoTro.doChiTietNhanh = doChiTietNhanh;
    hoTro.cho = cho;
    if (chay("dang") && chayPhan("dang", "C1")) {
      const t = await mo("C1", "chat-0", "/community");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(2200);
      await choOn(t.page, { mang: t.mang });
      const vao = await moForm(t, cdp);
      const den = await duongDan(t.page);
      const f = await doForm(t.page);
      await anh(t.page, "EV-N14-DANG-C1", t);
      ghi({ tc: "TC-N14-DANG-FORM", screen: "N14.S03", state: "chat-0, form trống", action: "tab Cộng đồng → «Đăng khoảnh khắc»", cauHinh: "C1", expected: "form «Kể một khoảnh khắc» đủ: ô nội dung, thêm ảnh, chủ đề kèm gợi ý, tag bạn, hai lựa chọn người đọc; nút gửi thấy trọn trong cửa sổ", status: vao && f?.tieuDe && f.body && f.chuDe !== null && f.radios.length === 2 && f.gui && f.gui.day <= f.cuaSo.h ? "PASS" : "FAIL", evidence: ["EV-N14-DANG-C1"], ghiChu: f ? `tới ${den}; tiêu đề «${f.tieuDe}»; ô nội dung cao ${f.body?.cao}px; gợi ý «${f.goiY}»; radio ${f.radios.map((r) => `«${r.ten}»${r.chon ? " (đã chọn)" : ""}`).join(", ")}; nút gửi «${f.gui?.ten}» ${f.gui?.top}–${f.gui?.day} trong cửa sổ ${f.cuaSo.h}` : `không thấy form, tới ${den}` });
      ghi({ tc: "TC-N14-DANG-NUT-TAT", screen: "N14.S03", state: "chat-0, form trống", action: "đọc nút gửi khi chưa có nội dung", cauHinh: "C1", expected: "nút tắt theo ADR-0038 §2.2: viền đứt kèm một câu lý do (ví dụ «Viết vài dòng trước khi gửi»)", status: f?.gui?.tat && f.gui.lyDo ? "PASS" : "FAIL", evidence: ["EV-N14-DANG-C1"], ghiChu: f?.gui ? `«${f.gui.ten}»: aria-disabled ${f.gui.tat}, viền ${f.gui.vien}; lý do cạnh nút: ${f.gui.lyDo ? `«${f.gui.lyDo}»` : "không"}` : "không thấy nút gửi" });
      const chon = f?.radios.find((r) => r.chon);
      ghi({ tc: "TC-N14-DANG-RADIO", screen: "N14.S03", state: "chat-0, form trống, người đọc mặc định", action: "đọc thuộc tính trợ năng của nhóm radio người đọc", cauHinh: "C1", expected: "radio đang chọn có aria-checked=\"true\", radio kia \"false\"", status: chon && chon.ariaChecked === "true" && f.radios.filter((r) => r !== chon).every((r) => r.ariaChecked === "false") ? "PASS" : "FAIL", evidence: ["EV-N14-DANG-C1"], ghiChu: f ? f.radios.map((r) => `«${r.ten}» chữ «Đã chọn» ${r.chon ? "có" : "không"}, aria-checked=${r.ariaChecked}, aria-selected=${r.ariaSelected}`).join("; ") : "-" });
      // «Một nhóm»: the send button waits for a group.
      await bam(t, cdp, "Lựa chọn khác");
      const nh = await nut(t.page, "Một nhóm", { batDau: true });
      if (nh) {
        await cham(cdp, nh);
        await t.page.waitForTimeout(900);
      }
      await go(t, cdp, "Nội dung bài đăng", "Thử chọn người đọc là một nhóm.");
      const fn = await doForm(t.page);
      // The group list sits under the radios: bring it into view for the capture.
      await t.page.evaluate(() => {
        const r = [...document.querySelectorAll('[role="radio"]')].find((x) => /^Một nhóm/.test((x.innerText ?? "").trim()));
        r?.scrollIntoView({ block: "start", behavior: "instant" });
      });
      await t.page.waitForTimeout(400);
      await anh(t.page, "EV-N14-DANG-NHOM-C1", t);
      ghi({ tc: "TC-N14-DANG-NHOM", screen: "N14.S03", state: "chat-0 (quản trị nhóm chat-test), đã gõ nội dung", action: "«Lựa chọn khác» → «Một nhóm»", cauHinh: "C1", expected: "hiện các nhóm để chọn; nút gửi tắt kèm lý do «chọn một nhóm» tới khi chọn", status: fn?.gui?.tat && fn.gui.lyDo && fn.nutKhac.length ? "PASS" : "FAIL", evidence: ["EV-N14-DANG-NHOM-C1"], ghiChu: fn ? `radio: ${fn.radios.map((r) => `«${r.ten}»${r.chon ? " (đã chọn)" : ""}`).join(", ")}; nút nhóm: ${fn.nutKhac.filter((x) => !/^(Gửi|Đăng)/.test(x)).join(", ") || "không có"}; nút gửi «${fn.gui?.ten}» aria-disabled ${fn.gui?.tat}; lý do: ${fn.gui?.lyDo ? `«${fn.gui.lyDo}»` : "không"}` : "-" });
      const cc = await nut(t.page, "Công khai · Cộng đồng", { batDau: true });
      if (cc) await cham(cdp, cc);
      await bam(t, cdp, "Thu gọn lựa chọn");
      // Topics the server refuses: six, then one that is too short. Nothing is written (422).
      for (const [chuDe, id, tc, kyVong] of [
        ["cà phê, đi bộ, đà lạt, ăn sáng, hoàng hôn, chợ đêm", "6-CHU-DE", "TC-N14-DANG-6-CHU-DE", "câu lỗi nói bớt còn 5 chủ đề; bản viết còn nguyên"],
        ["a", "CHU-DE-NGAN", "TC-N14-DANG-CHU-DE-NGAN", "câu lỗi nói mỗi chủ đề cần từ 2 ký tự; bản viết còn nguyên"],
      ]) {
        const noiDung = "Buổi sáng ở chợ đêm, thử xem máy chủ nhận bao nhiêu chủ đề.";
        await go(t, cdp, "Nội dung bài đăng", noiDung);
        await go(t, cdp, "Chủ đề", chuDe);
        const post = [];
        const nghe = (r) => {
          if (/\/v2\/community\/posts$/.test(r.url()) && r.request().method() === "POST") post.push(r.status());
        };
        t.page.on("response", nghe);
        const g = await nut(t.page, "Gửi lên cộng đồng");
        if (g && !g.tat) await cham(cdp, g);
        await t.page.waitForTimeout(2500);
        t.page.off("response", nghe);
        const fl = await doForm(t.page);
        const loi = fl?.loi.join(" ") ?? "";
        // What the screen shows right after the press, then the sentence itself scrolled into view.
        await anh(t.page, `EV-N14-DANG-${id}-C1`, t);
        await t.page.evaluate(() => [...document.querySelectorAll('[data-testid="community-composer"] [role="alert"]')].pop()?.scrollIntoView({ block: "center", behavior: "instant" }));
        await t.page.waitForTimeout(400);
        await anh(t.page, `EV-N14-DANG-${id}-LOI-C1`, t, { chuThich: fl?.loi.length ? [{ selector: '[data-testid="community-composer"] [role="alert"]' }] : [] });
        const noiRo = id === "6-CHU-DE" ? /5 chủ đề|năm chủ đề/.test(loi) : /2 ký tự|hai ký tự|quá ngắn/.test(loi);
        const thay = fl?.loiO.some((x) => x.thay);
        ghi({ tc, screen: "N14.S03", state: "chat-0, đã gõ nội dung", action: `ô «Chủ đề»: «${chuDe}» → «Gửi lên cộng đồng»`, cauHinh: "C1", expected: kyVong, status: g && noiRo && thay && fl.body?.gt === noiDung ? "PASS" : "FAIL", evidence: [`EV-N14-DANG-${id}-C1`, `EV-N14-DANG-${id}-LOI-C1`], ghiChu: `POST /v2/community/posts: ${post.join(", ") || "không gửi"}; câu hiện ra: ${loi ? `«${loi}»` : "không"}; vị trí câu lỗi lúc bấm: ${fl?.loiO.map((x) => `y ${x.top}–${x.day} trong cửa sổ ${fl.cuaSo.h}, ${x.thay ? "thấy được" : "ngoài tầm nhìn"}`).join(", ") || "-"}; nút gửi ${fl?.gui?.top}–${fl?.gui?.day}; bản viết còn: ${fl?.body?.gt === noiDung ? "có" : "không"}; chủ đề còn «${fl?.chuDe}»; gợi ý dưới ô «${fl?.goiY}»` });
      }
      // B1: public, short, two topics.
      if (!(await timBai("B1"))) {
        await go(t, cdp, "Nội dung bài đăng", BAI.B1.body);
        await go(t, cdp, "Chủ đề", BAI.B1.topics);
        const kq = await guiBai(t, cdp);
        const d = await doChiTietNhanh(t.page);
        await anh(t.page, "EV-N14-DANG-CONG-KHAI-C1", t);
        ghi({ tc: "TC-N14-DANG-CONG-KHAI", screen: "N14.S03", state: "chat-0, bài công khai ngắn, hai chủ đề", action: "gõ nội dung và chủ đề → «Gửi lên cộng đồng»", cauHinh: "C1", expected: "tới chi tiết bài; thẻ nói «Đang chờ duyệt · Bản mới chưa xuất hiện công khai»; chủ đề đã chuẩn hoá", status: /\/community\/posts\//.test(kq.den) && /^Đang chờ duyệt/.test(d?.dai ?? "") ? "PASS" : "FAIL", evidence: ["EV-N14-DANG-CONG-KHAI-C1"], ghiChu: `tới ${anId(kq.den)} sau ${kq.ms ?? "?"} ms; dải «${d?.dai}»; meta «${d?.meta}»; chủ đề ${d?.chuDe.join(", ") || "không thấy"}; câu lỗi: ${d?.loi.join(" ") || "không"}` });
        // Back from the detail: the composer was replaced, so Back is the feed.
        const ve = await bam(t, cdp, "Quay lại", { cho: 2200 });
        const sauVe = await duongDan(t.page);
        ghi({ tc: "TC-N14-DANG-VE", screen: "N14.S03", state: "chat-0 vừa gửi bài, đang ở chi tiết bài", action: "«Quay lại»", cauHinh: "C1", expected: "về bảng tin, không về lại form đã gửi", status: ve && /\/community$/.test(sauVe) ? "PASS" : "FAIL", evidence: ["EV-N14-DANG-CONG-KHAI-C1"], ghiChu: `«Quay lại»: ${ve ? "có" : "không thấy"}; về ${anId(sauVe)}` });
      } else console.log("dang: B1 đã có, không gửi lại");
      // B2: public, long, one synthetic photo through the file chooser.
      if (!(await timBai("B2"))) {
        await moForm(t, cdp);
        const png = join(mt.out, "tam", "n14-anh-640x480.png");
        mkdirSync(join(mt.out, "tam"), { recursive: true });
        writeFileSync(png, pngThuBytes(640, 480));
        const tai = [];
        const nghe = (r) => {
          if (/\/v2\/community\/media$/.test(r.url()) && r.request().method() === "POST") tai.push(r.status());
        };
        t.page.on("response", nghe);
        const [fc] = await Promise.all([t.page.waitForEvent("filechooser", { timeout: 10_000 }).catch(() => null), bam(t, cdp, "Thêm ảnh hoặc video", { cho: 300 })]);
        if (fc) await fc.setFiles(png);
        const msTai = await cho(t.page, async () => (await doForm(t.page))?.boTep > 0, 20_000);
        t.page.off("response", nghe);
        await go(t, cdp, "Nội dung bài đăng", BAI.B2.body);
        await go(t, cdp, "Chủ đề", BAI.B2.topics);
        const fa = await doForm(t.page);
        await anh(t.page, "EV-N14-DANG-ANH-C1", t);
        const kq = await guiBai(t, cdp);
        const d = await doChiTietNhanh(t.page);
        await anh(t.page, "EV-N14-DANG-ANH-GUI-C1", t);
        ghi({ tc: "TC-N14-DANG-ANH", screen: "N14.S03", state: "chat-0, bài công khai dài (hơn 240 ký tự) với một ảnh tổng hợp 640×480", action: "«Thêm ảnh hoặc video» → chọn tệp → gõ → «Gửi lên cộng đồng»", cauHinh: "C1", expected: "ảnh tải lên xong, ô xem trước và «Bỏ tệp»; gửi xong chi tiết bài có ảnh và dải chờ duyệt", status: fc && msTai !== null && fa?.anh.length && kq.den && /\/community\/posts\//.test(kq.den) && d?.anh.length && /^Đang chờ duyệt/.test(d.dai ?? "") ? "PASS" : "FAIL", evidence: ["EV-N14-DANG-ANH-C1", "EV-N14-DANG-ANH-GUI-C1"], ghiChu: `bộ chọn tệp: ${fc ? "mở" : "không mở"}; POST media: ${tai.join(", ") || "không"}; «Bỏ tệp» hiện sau ${msTai ?? "—"} ms; xem trước ${fa?.anh.map((a) => `${a.w}×${a.h}`).join(", ") || "không"}; tới ${anId(kq.den)}; ảnh trên chi tiết ${d?.anh.map((a) => `${a.w}×${a.h}`).join(", ") || "không"}; dải «${d?.dai}»` });
        await bam(t, cdp, "Quay lại", { cho: 2000 });
      } else console.log("dang: B2 đã có, không gửi lại");
      // B3: friends only.
      if (!(await timBai("B3"))) {
        await moForm(t, cdp);
        await go(t, cdp, "Nội dung bài đăng", BAI.B3.body);
        const bb = await nut(t.page, "Riêng tư · Bạn bè", { batDau: true });
        if (bb) await cham(cdp, bb);
        await t.page.waitForTimeout(600);
        const fb = await doForm(t.page);
        const kq = await guiBai(t, cdp);
        const d = await doChiTietNhanh(t.page);
        await anh(t.page, "EV-N14-DANG-BAN-BE-C1", t);
        ghi({ tc: "TC-N14-DANG-BAN-BE", screen: "N14.S03", state: "chat-0, bài «Bạn bè»", action: "gõ nội dung → radio «Riêng tư · Bạn bè» → gửi", cauHinh: "C1", expected: "nút đổi thành «Đăng lên tường»; bài đăng ngay, người đọc «Bạn bè», không dải chờ duyệt", status: fb?.gui?.ten === "Đăng lên tường" && /\/community\/posts\//.test(kq.den) && /Bạn bè$/.test(d?.meta ?? "") && !d.dai ? "PASS" : "FAIL", evidence: ["EV-N14-DANG-BAN-BE-C1"], ghiChu: `nút gửi «${fb?.gui?.ten}»; tới ${anId(kq.den)}; meta «${d?.meta}»; dải ${d?.dai ? `«${d.dai}»` : "không"}` });
      } else console.log("dang: B3 đã có, không gửi lại");
      await t.context.close();
    }
    // Baselines of the empty form at the other sampled sizes; nothing is sent.
    for (const cfg of ["C2", "C8", "C6", "C7"]) {
      if (!chay("dang") || !chayPhan("dang", cfg)) continue;
      const t = await mo(cfg, "chat-0", "/community");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(2200);
      await choOn(t.page, { mang: t.mang });
      await moForm(t, cdp);
      if (cfg === "C8") await go(t, cdp, "Nội dung bài đăng", "Một buổi chiều ở hồ, gió lạnh và trà nóng.");
      const f = await doForm(t.page);
      const m = await anh(t.page, `EV-N14-DANG-${cfg}`, t);
      ghi({ tc: "TC-N14-DANG-FORM", screen: "N14.S03", state: `chat-0, form ${cfg === "C8" ? "đã gõ một dòng" : "trống"}`, action: "tab Cộng đồng → «Đăng khoảnh khắc»", cauHinh: cfg, expected: "form đủ, không tràn ngang; nút gửi thấy trọn trong cửa sổ", status: f?.tieuDe && f.gui && f.gui.day <= f.cuaSo.h && m.tomTat.tranTrangPx === 0 && m.tomTat.chuBiCat === 0 ? "PASS" : "FAIL", evidence: [`EV-N14-DANG-${cfg}`], ghiChu: f ? `nút gửi «${f.gui?.ten}» ${f.gui?.top}–${f.gui?.day} trong cửa sổ ${f.cuaSo.w}×${f.cuaSo.h}; ô nội dung cao ${f.body?.cao}px; tràn trang ${m.tomTat.tranTrangPx}px, chữ bị cắt ${m.tomTat.chuBiCat}, vùng bấm < 48: ${m.tomTat.vungBamNho48}` : "không thấy form" });
      if (cfg === "C6" || cfg === "C7") {
        // Tablet: the feed keeps a 560dp reading column; does the form?
        const rong = await t.page.evaluate(() => {
          const hop = (e) => {
            if (!e) return null;
            const r = e.getBoundingClientRect();
            return { x: Math.round(r.left), phai: Math.round(r.right), w: Math.round(r.width) };
          };
          const noiDung = [...document.querySelectorAll('[data-testid="community-body"]')].filter((x) => x.getClientRects().length).pop();
          const gui = [...document.querySelectorAll('[role="button"]')].filter((b) => b.getClientRects().length && /^(Gửi lên cộng đồng|Đăng lên tường)$/.test((b.innerText ?? "").trim())).pop();
          return { noiDung: hop(noiDung), gui: hop(gui) };
        });
        ghi({ tc: "TC-N14-DANG-TABLET", screen: "N14.S03", state: "chat-0, form trống, tablet", action: "tab Cộng đồng → «Đăng khoảnh khắc»", cauHinh: cfg, expected: "nội dung form gom trong cột đọc như bảng tin cùng cấu hình (cột 560, DESIGN.md), không quá 640px", status: rong.noiDung && rong.noiDung.w <= 640 && (!rong.gui || rong.gui.w <= 640) ? "PASS" : "FAIL", evidence: [`EV-N14-DANG-${cfg}`], ghiChu: `ô nội dung x ${rong.noiDung?.x}–${rong.noiDung?.phai} (rộng ${rong.noiDung?.w}), nút gửi rộng ${rong.gui?.w}; cửa sổ ${f?.cuaSo.w}` });
      }
      await t.context.close();
    }
    // B4: chat-1's public post, written in the dark theme (C3). It stays pending until `ws`.
    if (chay("dang") && chayPhan("dang", "C3")) {
      const t = await mo("C3", "chat-1", "/community");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(2200);
      await choOn(t.page, { mang: t.mang });
      await moForm(t, cdp);
      const f = await doForm(t.page);
      const m = await anh(t.page, "EV-N14-DANG-C3", t);
      ghi({ tc: "TC-N14-DANG-FORM", screen: "N14.S03", state: "chat-1, form trống, giao diện tối", action: "tab Cộng đồng → «Đăng khoảnh khắc»", cauHinh: "C3", expected: "form đủ, không tràn ngang; nút gửi thấy trọn trong cửa sổ", status: f?.tieuDe && f.gui && f.gui.day <= f.cuaSo.h && m.tomTat.tranTrangPx === 0 && m.tomTat.chuBiCat === 0 ? "PASS" : "FAIL", evidence: ["EV-N14-DANG-C3"], ghiChu: f ? `nút gửi «${f.gui?.ten}» ${f.gui?.top}–${f.gui?.day} trong cửa sổ ${f.cuaSo.w}×${f.cuaSo.h}; tràn trang ${m.tomTat.tranTrangPx}px, chữ bị cắt ${m.tomTat.chuBiCat}` : "không thấy form" });
      if (!(await timBai("B4"))) {
        await go(t, cdp, "Nội dung bài đăng", BAI.B4.body);
        await go(t, cdp, "Chủ đề", BAI.B4.topics);
        const kq = await guiBai(t, cdp);
        const d = await doChiTietNhanh(t.page);
        await anh(t.page, "EV-N14-DANG-CONG-KHAI-C3", t);
        ghi({ tc: "TC-N14-DANG-CONG-KHAI", screen: "N14.S03", state: "chat-1, bài công khai ngắn, một chủ đề, giao diện tối", action: "gõ nội dung và chủ đề → «Gửi lên cộng đồng»", cauHinh: "C3", expected: "tới chi tiết bài; thẻ nói «Đang chờ duyệt · Bản mới chưa xuất hiện công khai»", status: /\/community\/posts\//.test(kq.den) && /^Đang chờ duyệt/.test(d?.dai ?? "") ? "PASS" : "FAIL", evidence: ["EV-N14-DANG-CONG-KHAI-C3"], ghiChu: `tới ${anId(kq.den)} sau ${kq.ms ?? "?"} ms; dải «${d?.dai}»; meta «${d?.meta}»` });
      } else console.log("dang: B4 đã có, không gửi lại");
      await t.context.close();
    }
  }

  // ------------------------------------------------------------------ duyet
  // The operator's queue at /community/review (no entry in the app: operators open the link).
  // Its readers are shared with `binh-luan` (comments are decided in the same queue).
  {
    /** Every item of the queue: caption, body, images, the reason field and the two decisions. */
    const doHang = (page) =>
      page.evaluate((lyDoSrc) => {
        const hien = (e) => e && e.getClientRects().length > 0 && e.checkVisibility({ opacityProperty: true });
        const chu = (e) => (e?.innerText ?? "").replace(/[-]/g, "").replace(/\s+/g, " ").trim();
        return [...document.querySelectorAll('[aria-label="Lý do quyết định"]')].filter(hien).map((o) => {
          let k = o.parentElement;
          while (k && ![...k.querySelectorAll('[role="button"]')].some((b) => chu(b) === "Duyệt công khai")) k = k.parentElement;
          const cac = [...k.querySelectorAll('div[dir="auto"]')].filter(hien).map(chu);
          const lyDoCua = (0, eval)(lyDoSrc);
          const nutCua = (ten) => {
            const b = [...k.querySelectorAll('[role="button"]')].find((x) => chu(x) === ten);
            return b ? { tat: b.getAttribute("aria-disabled") === "true", lyDo: lyDoCua(b) } : null;
          };
          return { chuThich: cac[0], than: cac[1] ?? "", anh: [...k.querySelectorAll("img")].filter(hien).map((i) => Math.round(i.getBoundingClientRect().height)), lyDo: o.value, duyet: nutCua("Duyệt công khai"), tuChoi: nutCua("Từ chối") };
        });
      }, LY_DO_SRC);
    /** Type a reason into one item's field and press one of its decisions. */
    const quyet = async (t, cdp, dau, lyDo, nutTen) => {
      const toaDo = async (loai) =>
        t.page.evaluate(
          ({ dau, loai, nutTen }) => {
            const chu = (e) => (e?.innerText ?? "").replace(/\s+/g, " ").trim();
            for (const o of document.querySelectorAll('[aria-label="Lý do quyết định"]')) {
              let k = o.parentElement;
              while (k && ![...k.querySelectorAll('[role="button"]')].some((b) => chu(b) === "Duyệt công khai")) k = k.parentElement;
              if (!k || !chu(k).includes(dau)) continue;
              const e = loai === "o" ? o : [...k.querySelectorAll('[role="button"]')].find((b) => chu(b) === nutTen);
              e.scrollIntoView({ block: "center", behavior: "instant" });
              const r = e.getBoundingClientRect();
              return { x: r.left + Math.min(40, r.width / 2), y: r.top + r.height / 2, tat: e.getAttribute("aria-disabled") === "true" };
            }
            return null;
          },
          { dau, loai, nutTen },
        );
      const o = await toaDo("o");
      if (!o) return null;
      await cham(cdp, o);
      await t.page.keyboard.type(lyDo, { delay: 8 });
      await t.page.waitForTimeout(400);
      const b = await toaDo("nut");
      if (!b || b.tat) return { bam: false, tat: b?.tat };
      await cham(cdp, b);
      await t.page.waitForTimeout(2500);
      await choOn(t.page, { mang: t.mang });
      return { bam: true };
    };
    hoTro.doHang = doHang;
    hoTro.quyet = quyet;
    if (chay("duyet") && chayPhan("duyet", "khong-quyen")) {
      const t = await mo("C1", "chat-16", "/community/review");
      await t.page.waitForTimeout(2500);
      await choOn(t.page, { mang: t.mang });
      const c = await chuTrang(t.page);
      const h = await doHang(t.page);
      await anh(t.page, "EV-N14-DUYET-KHONG-QUYEN-C1", t);
      ghi({ tc: "TC-N14-DUYET-KHONG-QUYEN", screen: "N14.S09", state: "chat-16, không có vai trò kiểm duyệt", action: "mở thẳng /community/review", cauHinh: "C1", expected: "câu «Tài khoản này không có quyền kiểm duyệt.»; không thấy nội dung chờ duyệt", status: /không có quyền kiểm duyệt/.test(c) && h.length === 0 ? "PASS" : "FAIL", evidence: ["EV-N14-DUYET-KHONG-QUYEN-C1"], ghiChu: `chữ «${c.slice(0, 200)}»; mục trong hàng: ${h.length}` });
      await t.context.close();
    }
    const q = chay("duyet") ? await goi("GET", "/v2/community/review", undefined, await phienCua("chat-15")) : { status: 0 };
    if (chay("duyet") && q.status !== 200) console.log(`duyet: chat-15 đọc hàng duyệt ra ${q.status} ${q.code}; cấp vai trò trước (INSERT INTO community_moderators)`);
    // Re-measure only the two decision buttons on whatever waits in the queue (no decision is made).
    if (q.status === 200 && chi?.has("duyet:nut-tat")) {
      const t = await mo("C1", "chat-15", "/community/review");
      await t.page.waitForTimeout(2500);
      await choOn(t.page, { mang: t.mang });
      const h = await doHang(t.page);
      await anh(t.page, "EV-N14-DUYET-NUT-TAT-C1", t);
      ghi({ tc: "TC-N14-DUYET-NUT-TAT", screen: "N14.S09", state: `chat-15, ô «Lý do quyết định» còn trống; ${h.length} mục chờ`, action: "đọc hai nút quyết định", cauHinh: "C1", expected: "hai nút tắt kèm lý do (cần lý do ≥ 3 ký tự) theo ADR-0038 §2.2", status: h.length && h.every((x) => x.duyet?.tat && x.duyet.lyDo && x.tuChoi?.tat && x.tuChoi.lyDo) ? "PASS" : "FAIL", evidence: ["EV-N14-DUYET-NUT-TAT-C1"], ghiChu: h.map((x) => `«${x.chuThich}» «${x.than.slice(0, 30)}…»: «Duyệt công khai» tắt ${x.duyet?.tat}, lý do ${x.duyet?.lyDo ? `«${x.duyet.lyDo}»` : "không"}; «Từ chối» tắt ${x.tuChoi?.tat}, lý do ${x.tuChoi?.lyDo ? `«${x.tuChoi.lyDo}»` : "không"}`).join(" | ") || "hàng đợi trống" });
      await t.context.close();
    }
    for (const cfg of q.status === 200 ? ["C2", "C3", "C1"] : []) {
      if (!chayPhan("duyet", cfg)) continue;
      const t = await mo(cfg, "chat-15", "/community/review");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(2500);
      await choOn(t.page, { mang: t.mang });
      const h = await doHang(t.page);
      const m = await anh(t.page, `EV-N14-DUYET-${cfg}`, t);
      if (cfg !== "C1") {
        ghi({ tc: "TC-N14-DUYET-BASE", screen: "N14.S09", state: `chat-15 (người vận hành), ${h.length} mục chờ`, action: "mở /community/review", cauHinh: cfg, expected: "hàng đợi đọc được, không tràn ngang, không chữ bị cắt", status: h.length && m.tomTat.tranTrangPx === 0 && m.tomTat.chuBiCat === 0 ? "PASS" : "FAIL", evidence: [`EV-N14-DUYET-${cfg}`], ghiChu: `${h.length} mục: ${h.map((x) => `«${x.chuThich}»`).join(", ")}; tràn trang ${m.tomTat.tranTrangPx}px, chữ bị cắt ${m.tomTat.chuBiCat}` });
        await t.context.close();
        continue;
      }
      const baiCho = h.filter((x) => /^Bài đăng/.test(x.chuThich));
      const coB3 = h.some((x) => x.than.startsWith(dauBai("B3")));
      ghi({ tc: "TC-N14-DUYET-HANG", screen: "N14.S09", state: "chat-15 (người vận hành); B1, B2 của chat-0 và B4 của chat-1 chờ duyệt, B3 là bài «Bạn bè»", action: "mở /community/review", cauHinh: cfg, expected: "hàng đợi có các bài công khai chờ duyệt (không có bài «Bạn bè»); mỗi mục nói trạng thái bằng lời; ảnh của bài hiện", status: baiCho.length >= 3 && !coB3 && baiCho.every((x) => !/·\s*(pending|review)\b/.test(x.chuThich)) && h.some((x) => x.anh.some((a) => a > 0)) ? "PASS" : "FAIL", evidence: [`EV-N14-DUYET-${cfg}`], ghiChu: `${h.length} mục: ${h.map((x) => `«${x.chuThich}» «${x.than.slice(0, 40)}…» ảnh ${x.anh.join("/") || "không"}`).join(" | ")}; có bài «Bạn bè»: ${coB3 ? "có" : "không"}` });
      ghi({ tc: "TC-N14-DUYET-NUT-TAT", screen: "N14.S09", state: "chat-15, ô «Lý do quyết định» còn trống", action: "đọc hai nút quyết định", cauHinh: cfg, expected: "hai nút tắt kèm lý do (cần lý do ≥ 3 ký tự) theo ADR-0038 §2.2", status: h.length && h.every((x) => x.duyet?.tat && x.duyet.lyDo && x.tuChoi?.tat && x.tuChoi.lyDo) ? "PASS" : "FAIL", evidence: [`EV-N14-DUYET-${cfg}`], ghiChu: h.map((x) => `«${x.chuThich}»: «Duyệt công khai» tắt ${x.duyet?.tat}, lý do ${x.duyet?.lyDo ? `«${x.duyet.lyDo}»` : "không"}; «Từ chối» tắt ${x.tuChoi?.tat}, lý do ${x.tuChoi?.lyDo ? `«${x.tuChoi.lyDo}»` : "không"}`).join(" | ") });
      // B2's photo, scrolled into view.
      await t.page.evaluate((dau) => {
        const e = [...document.querySelectorAll('div[dir="auto"]')].find((x) => (x.innerText ?? "").startsWith(dau));
        e?.scrollIntoView({ block: "start", behavior: "instant" });
      }, dauBai("B2"));
      await t.page.waitForTimeout(500);
      await anh(t.page, "EV-N14-DUYET-ANH-C1", t);
      // Approve B1 and B2 with a reason; B4 stays pending for the realtime step.
      const kq = [];
      for (const k of ["B1", "B2"]) {
        if (!h.some((x) => x.than.startsWith(dauBai(k)))) {
          kq.push(`${k}: không có trong hàng`);
          continue;
        }
        const r = await quyet(t, cdp, dauBai(k), "Đúng chủ đề đi chơi, không có thông tin riêng.", "Duyệt công khai");
        kq.push(`${k}: ${r?.bam ? "đã chạm «Duyệt công khai»" : `không chạm được (${JSON.stringify(r)})`}`);
      }
      const h2 = await doHang(t.page);
      const loi = await t.page.evaluate(() => [...document.querySelectorAll('[role="alert"]')].filter((a) => a.getClientRects().length).map((a) => (a.innerText ?? "").trim()));
      await anh(t.page, "EV-N14-DUYET-SAU-C1", t);
      ghi({ tc: "TC-N14-DUYET-DUYET", screen: "N14.S09", state: "chat-15, B1 và B2 chờ duyệt", action: "gõ lý do cho từng bài → «Duyệt công khai»", cauHinh: cfg, expected: "bài đã duyệt rời hàng đợi; không câu lỗi", status: ["B1", "B2"].every((k) => !h2.some((x) => x.than.startsWith(dauBai(k)))) && !loi.length ? "PASS" : "FAIL", evidence: ["EV-N14-DUYET-C1", "EV-N14-DUYET-SAU-C1"], ghiChu: `${kq.join("; ")}; còn lại ${h2.length} mục: ${h2.map((x) => `«${x.chuThich}» «${x.than.slice(0, 30)}…»`).join(", ") || "không"}; câu lỗi: ${loi.join(" ") || "không"}` });
      await t.context.close();
    }
  }

  // ------------------------------------------------------------------ sau-duyet
  // With one approved public post, the same reads again: the error was the empty ranking.
  if (chay("sau-duyet")) {
    const truoc = dem23502();
    const bang = [];
    for (const ten of ["chat-0", "chat-3"]) {
      const p = await phienCua(ten);
      for (const mode of ["for_you", "following", "trending"]) {
        const r = await goi("GET", `/v2/community/feed?mode=${mode}`, undefined, p);
        bang.push({ ten, mode, status: r.status, code: r.code, so: r.json?.posts?.length ?? null });
      }
    }
    ghi({ tc: "TC-N14-API-SAU-DUYET", screen: "N14.S01", state: "đã duyệt B1, B2 (hai bài công khai)", action: "GET /v2/community/feed ba mode, chat-0 và chat-3 (chỉ đọc)", cauHinh: "-", expected: "ba mode trả 200", status: bang.every((b) => b.status === 200) ? "PASS" : "FAIL", ghiChu: `${bang.map((b) => `${b.ten} ${b.mode} ${b.status}${b.code ? ` ${b.code}` : ""}${b.so !== null ? ` (${b.so} bài)` : ""}`).join("; ")}; dòng 23502 trong log core: ${truoc ?? "?"} → ${dem23502() ?? "?"}` });
    const t = await mo("C1", "chat-0", "/community");
    await t.page.waitForTimeout(2500);
    await choOn(t.page, { mang: t.mang });
    const m = await doBangTin(t.page);
    await anh(t.page, "EV-N14-SAU-DUYET-C1", t);
    ghi({ tc: "TC-N14-RONG-SAU-DUYET", screen: "N14.S01", state: "chat-0, đã có hai bài công khai được duyệt", action: "mở tab Cộng đồng", cauHinh: "C1", expected: "bảng tin hiện hai bài, không câu lỗi (cùng tài khoản, cùng lời mời chưa trả lời như lúc rỗng)", status: m && m.the.length >= 1 && !m.alert.length ? "PASS" : "FAIL", evidence: ["EV-N14-SAU-DUYET-C1"], ghiChu: m ? `${m.the.length} thẻ: ${m.the.map((x) => `«${x.dau.slice(0, 50)}…»`).join(", ")}; câu lỗi: ${m.alert.map((a) => a.chu).join(" ") || "không"}; hộp mời: ${m.moi ? "có" : "không"}` : "không thấy màn" });
    await t.context.close();
  }

  // ------------------------------------------------------------------ helpers of the feed and the post
  /** Every post card on screen: its box, photo frames against the album's clip, «Đọc tiếp», actions. */
  const doThe = (page, goc = '[data-testid="community-screen"]') =>
    page.evaluate((goc) => {
      const hien = (e) => e && e.getClientRects().length > 0 && e.checkVisibility({ opacityProperty: true });
      const chu = (e) => (e?.innerText ?? "").replace(/[-]/g, "").replace(/\s+/g, " ").trim();
      const man = [...document.querySelectorAll(goc)].filter(hien).pop();
      if (!man) return [];
      return [...man.querySelectorAll('[data-testid^="community-post-"]')].filter((e) => hien(e) && /^community-post-[0-9a-f-]{36}$/.test(e.dataset.testid)).map((e) => {
        const r = e.getBoundingClientRect();
        const album = [...e.querySelectorAll("div")].find((d) => ["auto", "scroll"].includes(getComputedStyle(d).overflowX) && d.scrollWidth > 0 && d.querySelector("img"));
        const ra = album?.getBoundingClientRect();
        const khung = [...e.querySelectorAll("img")].filter(hien).map((i) => {
          const q = i.getBoundingClientRect();
          return { x: Math.round(q.left), phai: Math.round(q.right), w: Math.round(q.width), h: Math.round(q.height) };
        });
        const than = [...e.querySelectorAll('[role="button"]')].find((b) => b.getAttribute("aria-label") === "Đọc toàn bộ câu chuyện");
        const rt = than?.getBoundingClientRect();
        const hanhDong = [...e.querySelectorAll('[role="button"]')].filter(hien).map((b) => {
          const q = b.getBoundingClientRect();
          return { ten: b.getAttribute("aria-label") || chu(b), chu: chu(b), w: Math.round(q.width), h: Math.round(q.height), phai: Math.round(q.right), ariaSelected: b.getAttribute("aria-selected"), ariaPressed: b.getAttribute("aria-pressed") };
        });
        return {
          id: e.dataset.testid.slice(15),
          dau: chu(e).slice(0, 90),
          x: Math.round(r.left),
          phai: Math.round(r.right),
          top: Math.round(r.top),
          cao: Math.round(r.height),
          than: rt ? { x: rt.left + rt.width / 2, y: rt.top + Math.min(rt.height / 2, 40), cao: Math.round(rt.height), top: Math.round(rt.top) } : null,
          docTiep: [...e.querySelectorAll('div[dir="auto"]')].some((x) => hien(x) && chu(x) === "Đọc tiếp"),
          dai: [...e.querySelectorAll('div[dir="auto"]')].map(chu).find((x) => /^(Đang chờ duyệt|Chưa phù hợp cộng đồng)/.test(x)) ?? null,
          meta: [...e.querySelectorAll('div[dir="auto"]')].map(chu).find((x) => /· (Cộng đồng|Bạn bè|Trong nhóm|Chỉ mình tôi)$/.test(x)) ?? null,
          album: album ? { x: Math.round(ra.left), phai: Math.round(ra.right), cw: album.clientWidth, sw: album.scrollWidth } : null,
          khung,
          hanhDong,
          cuaSo: innerWidth,
        };
      });
    }, goc);
  const theCua = async (page, k, goc) => (await doThe(page, goc)).find((x) => x.dau.includes(dauBai(k).slice(0, 30))) ?? null;
  /** Scroll the card of post k into view (the list is virtualised: scroll until it is mounted). */
  const denThe = async (t, k, block = "start") => {
    for (let lan = 0; lan < 12; lan++) {
      const ok = await t.page.evaluate(
        ({ dau, block }) => {
          const e = [...document.querySelectorAll('[data-testid^="community-post-"]')].find((x) => /^community-post-[0-9a-f-]{36}$/.test(x.dataset.testid) && (x.innerText ?? "").includes(dau) && x.getClientRects().length);
          if (e) {
            e.scrollIntoView({ block, behavior: "instant" });
            return true;
          }
          const s = [...document.querySelectorAll('[data-testid="community-screen"] div')].filter((d) => ["auto", "scroll"].includes(getComputedStyle(d).overflowY) && d.scrollHeight > d.clientHeight + 4).sort((a, b) => b.scrollHeight - a.scrollHeight)[0];
          if (s) s.scrollTop += s.clientHeight * 0.8;
          return false;
        },
        { dau: dauBai(k).slice(0, 30), block },
      );
      await t.page.waitForTimeout(ok ? 500 : 350);
      if (ok) return true;
    }
    return false;
  };
  /** The list's own scroller: its offset, and which card sits at the top of the window. */
  const viTriCuon = (page) =>
    page.evaluate(() => {
      const s = [...document.querySelectorAll('[data-testid="community-screen"] div')].filter((d) => d.getClientRects().length && ["auto", "scroll"].includes(getComputedStyle(d).overflowY) && d.scrollHeight > d.clientHeight + 4).sort((a, b) => b.scrollHeight - a.scrollHeight)[0];
      const the = [...document.querySelectorAll('[data-testid^="community-post-"]')].filter((x) => /^community-post-[0-9a-f-]{36}$/.test(x.dataset.testid) && x.getClientRects().length);
      const dau = the.map((x) => ({ chu: (x.innerText ?? "").replace(/\s+/g, " ").trim().slice(0, 40), top: Math.round(x.getBoundingClientRect().top) })).filter((x) => x.top > -400).sort((a, b) => Math.abs(a.top) - Math.abs(b.top))[0];
      return { top: s ? Math.round(s.scrollTop) : null, cao: s ? s.scrollHeight : null, the: the.length, gan: dau ?? null };
    });
  /** The detail screen: the card, the management/Nếp/report sheets are read with dialogTren. */
  const doChiTiet = (page) =>
    page.evaluate(() => {
      const hien = (e) => e && e.getClientRects().length > 0 && e.checkVisibility({ opacityProperty: true });
      const chu = (e) => (e?.innerText ?? "").replace(/[-]/g, "").replace(/\s+/g, " ").trim();
      const man = [...document.querySelectorAll('[data-testid="community-post-detail"]')].filter(hien).pop();
      if (!man) return null;
      const the = [...man.querySelectorAll('[data-testid^="community-post-"]')].find((e) => /^community-post-[0-9a-f-]{36}$/.test(e.dataset.testid));
      const cac = [...man.querySelectorAll('div[dir="auto"]')].filter(hien).map(chu);
      const oBl = [...man.querySelectorAll('[aria-label="Bình luận"]')].filter((x) => hien(x) && /^(INPUT|TEXTAREA)$/.test(x.tagName)).pop();
      const guiBl = [...man.querySelectorAll('[role="button"]')].filter((b) => hien(b) && /^(Gửi bình luận|Đang gửi…)$/.test(chu(b))).pop();
      return {
        id: the?.dataset.testid.slice(15) ?? null,
        tieuDe: cac.find((x) => x === "Một câu chuyện") ?? null,
        quayLai: [...man.querySelectorAll('[role="button"]')].some((b) => hien(b) && b.getAttribute("aria-label") === "Quay lại"),
        dangMo: cac.includes("Đang mở câu chuyện…"),
        meta: cac.find((x) => /· (Cộng đồng|Bạn bè|Trong nhóm|Chỉ mình tôi)$/.test(x)) ?? null,
        dai: cac.find((x) => /^(Đang chờ duyệt|Chưa phù hợp cộng đồng)/.test(x)) ?? null,
        // The body: the longest text of the card that is not the status band or the meta line.
        than: the ? [...the.querySelectorAll('div[dir="auto"]')].map(chu).filter((x) => x.length > 40 && !/^(Đang chờ duyệt|Chưa phù hợp cộng đồng)/.test(x) && !/· (Cộng đồng|Bạn bè|Trong nhóm|Chỉ mình tôi)$/.test(x)).sort((a, b) => b.length - a.length)[0] ?? null : null,
        loi: [...man.querySelectorAll('[role="alert"]')].filter(hien).map(chu),
        binhLuan: cac.filter((x) => /^(Đang chờ duyệt · Chỉ bạn thấy|Bình luận chưa phù hợp)$/.test(x)).length,
        oBl: oBl ? { gt: oBl.value, top: Math.round(oBl.getBoundingClientRect().top) } : null,
        guiBl: guiBl ? { tat: guiBl.getAttribute("aria-disabled") === "true", lyDo: chu(guiBl.parentElement).replace(chu(guiBl), "").trim() || null } : null,
        nhan: cac.find((x) => /^(Trả lời .+|Thêm một lời)$/.test(x)) ?? null,
      };
    });
  /** Open post k's detail from the feed the way a reader does: the body twice (once expands). */
  const moChiTiet = async (t, cdp, k) => {
    if (!(await denThe(t, k, "center"))) return null;
    for (let lan = 0; lan < 2; lan++) {
      // Centre the body itself (a tall card centred leaves its body under the header at 320dp),
      // then tap the middle of its first line.
      const p = await t.page.evaluate((dau) => {
        const the = [...document.querySelectorAll('[data-testid^="community-post-"]')].find((x) => /^community-post-[0-9a-f-]{36}$/.test(x.dataset.testid) && (x.innerText ?? "").includes(dau) && x.getClientRects().length);
        const b = the && [...the.querySelectorAll('[role="button"]')].find((x) => x.getAttribute("aria-label") === "Đọc toàn bộ câu chuyện");
        if (!b) return null;
        b.scrollIntoView({ block: "center", behavior: "instant" });
        const r = b.getBoundingClientRect();
        return { x: r.left + r.width / 2, y: r.top + Math.min(r.height / 2, 14) };
      }, dauBai(k).slice(0, 30));
      if (!p) return null;
      await t.page.waitForTimeout(300);
      await cham(cdp, p);
      await t.page.waitForTimeout(1500);
      if (/\/community\/posts\//.test(await duongDan(t.page))) break;
    }
    await choOn(t.page, { mang: t.mang });
    return /\/community\/posts\//.test(await duongDan(t.page));
  };

  // ------------------------------------------------------------------ bang
  // The feed with posts, as chat-1 (a friend, not the author) reads it.
  if (chay("bang")) {
    const anhBang = [];
    for (const cfg of ["C1", "C2", "C3", "C4", "C5", "C6", "C7", "C8"]) {
      if (!chayPhan("bang", cfg)) continue;
      const t = await mo(cfg, "chat-1", "/community");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(2500);
      await choOn(t.page, { mang: t.mang });
      const m = await doBangTin(t.page);
      const cards = await doThe(t.page);
      const cap = await anh(t.page, `EV-N14-BANG-${cfg}`, t);
      anhBang.push({ id: `EV-N14-BANG-${cfg}`, nhan: cfg });
      // The photo card: its frame against the album's clip and the window.
      await denThe(t, "B2", "start");
      await t.page.evaluate(() => {
        const s = [...document.querySelectorAll('[data-testid="community-screen"] div')].filter((d) => ["auto", "scroll"].includes(getComputedStyle(d).overflowY) && d.scrollHeight > d.clientHeight + 4).sort((a, b) => b.scrollHeight - a.scrollHeight)[0];
        if (s) s.scrollTop += 120;
      });
      await t.page.waitForTimeout(500);
      const b2 = await theCua(t.page, "B2");
      const k = b2?.khung[0];
      const catKhung = k && b2.album ? Math.max(0, k.phai - b2.album.phai) : null;
      const capAnh = await anh(t.page, `EV-N14-BANG-ANH-${cfg}`, t, { chuThich: k ? [{ selector: `[data-testid="community-post-${b2.id}"] img` }] : [] });
      const nho = [...new Set([...cards, ...(b2 ? [b2] : [])].flatMap((c) => c.hanhDong.filter((h) => h.h < 48 || h.w < 48).map((h) => `${h.ten} ${h.w}×${h.h}`)))];
      const dat = m && !m.alert.length && cards.length >= 1 && catKhung === 0 && (b2?.album ? b2.album.sw <= b2.album.cw + 1 : true) && cap.tomTat.tranTrangPx === 0 && capAnh.tomTat.chuBiCat === 0;
      ghi({ tc: "TC-N14-BANG-THE", screen: "N14.S01", layer: "-", state: "chat-1, «Dành cho bạn» có B1 (ngắn) và B2 (dài, một ảnh) của chat-0", action: "mở tab Cộng đồng; cuộn tới thẻ có ảnh", cauHinh: cfg, expected: "thẻ bài nằm trong cột đọc: khung ảnh không bị cắt, không cuộn ngang thừa; chữ không bị cắt; nút hành động ≥ 48dp", status: dat ? "PASS" : "FAIL", evidence: [`EV-N14-BANG-${cfg}`, `EV-N14-BANG-ANH-${cfg}`], ghiChu: `${cards.length} thẻ lúc mở; thẻ B2 ${b2 ? `x ${b2.x}–${b2.phai}` : "không thấy"}; khung ảnh ${k ? `${k.w}×${k.h} tại x ${k.x}–${k.phai}` : "không"}; vùng cắt của album ${b2?.album ? `x ${b2.album.x}–${b2.album.phai}, rộng ${b2.album.cw}, nội dung ${b2.album.sw}` : "-"}; ảnh bị cắt ${catKhung ?? "-"}px; tràn trang ${cap.tomTat.tranTrangPx}px; chữ bị cắt ${capAnh.tomTat.chuBiCat}; nút < 48: ${nho.join(", ") || "không"}; câu lỗi: ${m?.alert.map((a) => a.chu).join(" ") || "không"}` });
      if (cfg === "C1") {
        // Back to the top for the interactions.
        await t.page.evaluate(() => {
          const s = [...document.querySelectorAll('[data-testid="community-screen"] div')].filter((d) => ["auto", "scroll"].includes(getComputedStyle(d).overflowY) && d.scrollHeight > d.clientHeight + 4).sort((a, b) => b.scrollHeight - a.scrollHeight)[0];
          if (s) s.scrollTop = 0;
        });
        await t.page.waitForTimeout(600);
        // A short body: what the first tap does.
        await denThe(t, "B1", "center");
        const b1 = await theCua(t.page, "B1");
        const u0 = await duongDan(t.page);
        if (b1?.than) await cham(cdp, b1.than);
        await t.page.waitForTimeout(1500);
        const u1 = await duongDan(t.page);
        const b1b = await theCua(t.page, "B1");
        const lan1 = { url: u1, cao: b1b?.than?.cao };
        if (b1b?.than && u1 === u0) await cham(cdp, b1b.than);
        await t.page.waitForTimeout(2000);
        const u2 = await duongDan(t.page);
        ghi({ tc: "TC-N14-BANG-CHAM-THAN", screen: "N14.S01", state: "chat-1, thẻ B1 (thân 121 ký tự, không «Đọc tiếp»)", action: "chạm thân bài một lần, rồi lần hai", cauHinh: "C1", expected: "chạm lần đầu mở chi tiết bài, hoặc thẻ đổi thấy rõ (thân bài ngắn không có gì để mở rộng)", status: u1 !== u0 || (lan1.cao ?? 0) > (b1?.than?.cao ?? 0) + 2 ? "PASS" : "FAIL", ghiChu: `trước: ${anId(u0)}, thân cao ${b1?.than?.cao ?? "-"}px, nhãn trợ năng «Đọc toàn bộ câu chuyện»; lần 1: ${anId(u1)}, thân cao ${lan1.cao ?? "-"}px; lần 2: ${anId(u2)}` });
        if (/\/community\/posts\//.test(u2)) {
          await bam(t, cdp, "Quay lại", { cho: 2500 });
          await choOn(t.page, { mang: t.mang });
        }
        // A long body: «Đọc tiếp».
        await denThe(t, "B2", "start");
        const b2a = await theCua(t.page, "B2");
        const dt = await nut(t.page, "Đọc toàn bộ câu chuyện", { trong: `[data-testid="community-post-${b2a?.id}"]` });
        const uA = await duongDan(t.page);
        if (dt) await cham(cdp, { x: dt.x, y: dt.y });
        await t.page.waitForTimeout(1200);
        const b2b = await theCua(t.page, "B2");
        ghi({ tc: "TC-N14-BANG-DOC-TIEP", screen: "N14.S01", state: "chat-1, thẻ B2 (thân 337 ký tự, cắt 6 dòng)", action: "chạm thân bài có «Đọc tiếp»", cauHinh: "C1", expected: "thân bài mở hết tại chỗ, «Đọc tiếp» biến mất, không rời bảng tin", status: b2a?.docTiep && !b2b?.docTiep && (b2b?.than?.cao ?? 0) > (b2a?.than?.cao ?? 0) && (await duongDan(t.page)) === uA ? "PASS" : "FAIL", ghiChu: `trước: «Đọc tiếp» ${b2a?.docTiep ? "có" : "không"}, thân cao ${b2a?.than?.cao ?? "-"}px; sau: «Đọc tiếp» ${b2b?.docTiep ? "có" : "không"}, thân cao ${b2b?.than?.cao ?? "-"}px; đường ${anId(await duongDan(t.page))}` });
        // Where the reader is, then into the post and back: B2's body 150dp under the window's top.
        await t.page.evaluate((dau) => {
          const s = [...document.querySelectorAll('[data-testid="community-screen"] div')].filter((d) => ["auto", "scroll"].includes(getComputedStyle(d).overflowY) && d.scrollHeight > d.clientHeight + 4).sort((a, b) => b.scrollHeight - a.scrollHeight)[0];
          const the = [...document.querySelectorAll('[data-testid^="community-post-"]')].find((x) => /^community-post-[0-9a-f-]{36}$/.test(x.dataset.testid) && (x.innerText ?? "").includes(dau));
          const e = the && [...the.querySelectorAll('[role="button"]')].find((b) => b.getAttribute("aria-label") === "Đọc toàn bộ câu chuyện");
          if (s && e) s.scrollTop += e.getBoundingClientRect().top - 150;
        }, dauBai("B2").slice(0, 30));
        await t.page.waitForTimeout(700);
        const truocCuon = await viTriCuon(t.page);
        await anh(t.page, "EV-N14-BANG-CUON-TRUOC-C1", t);
        const b2c = await theCua(t.page, "B2");
        if (b2c?.than) await cham(cdp, b2c.than);
        await t.page.waitForTimeout(2200);
        const vao = await duongDan(t.page);
        await bam(t, cdp, "Quay lại", { cho: 2800 });
        await choOn(t.page, { mang: t.mang });
        const sauCuon = await viTriCuon(t.page);
        await anh(t.page, "EV-N14-BANG-CUON-SAU-C1", t);
        const b2d = await theCua(t.page, "B2");
        ghi({ tc: "TC-N14-BANG-CUON", screen: "N14.S01", state: "chat-1, đã cuộn tới ảnh của B2 và mở hết thân B2", action: "chạm thân B2 (mở chi tiết) → «Quay lại»", cauHinh: "C1", expected: "về bảng tin đúng chỗ đang đọc, thẻ vừa mở hết vẫn mở (ADR-0040: không tự đẩy vị trí cuộn)", status: /\/community\/posts\//.test(vao) && truocCuon.top !== null && sauCuon.top !== null && Math.abs(sauCuon.top - truocCuon.top) <= 24 ? "PASS" : "FAIL", evidence: ["EV-N14-BANG-CUON-TRUOC-C1", "EV-N14-BANG-CUON-SAU-C1"], ghiChu: `trước: cuộn ${truocCuon.top}/${truocCuon.cao}px, thẻ gần đầu «${truocCuon.gan?.chu}» ở y ${truocCuon.gan?.top}; vào ${anId(vao)}; sau khi về: cuộn ${sauCuon.top}/${sauCuon.cao}px, thẻ gần đầu «${sauCuon.gan?.chu}» ở y ${sauCuon.gan?.top}; thân B2 ${b2d?.docTiep ? "đã gập lại («Đọc tiếp» trở lại)" : "vẫn mở"}` });
        // The photo viewer.
        await denThe(t, "B2", "start");
        const moAnh = await nut(t.page, "Mở ảnh khoảnh khắc");
        if (moAnh) await cham(cdp, moAnh);
        await t.page.waitForTimeout(1500);
        const xem = await t.page.evaluate(() => {
          const d = [...document.querySelectorAll('[role="dialog"],[aria-modal="true"]')].filter((x) => x.getClientRects().length).pop();
          if (!d) return null;
          const imgs = [...d.querySelectorAll("img")].map((i) => {
            const r = i.getBoundingClientRect();
            let cha = i.parentElement;
            let caoCha = null;
            for (let n = 0; n < 4 && cha; n++, cha = cha.parentElement) caoCha = caoCha ?? (cha.getBoundingClientRect().height === 0 ? 0 : null);
            return { w: Math.round(r.width), h: Math.round(r.height), caoCha };
          });
          const r = d.getBoundingClientRect();
          return { ten: d.getAttribute("aria-label"), w: Math.round(r.width), h: Math.round(r.height), imgs, chu: (d.innerText ?? "").replace(/\s+/g, " ").trim().slice(0, 120) };
        });
        const fXem = await focusHienTai(t.page);
        await anh(t.page, "EV-N14-BANG-XEM-ANH-C1", t);
        await t.page.keyboard.press("Escape");
        await t.page.waitForTimeout(900);
        const conXem = await t.page.evaluate(() => [...document.querySelectorAll('[role="dialog"],[aria-modal="true"]')].filter((x) => x.getClientRects().length).length);
        ghi({ tc: "TC-N14-BANG-XEM-ANH", screen: "N14.S01", layer: "L25", state: "chat-1, thẻ B2 có một ảnh 640×480", action: "chạm «Mở ảnh khoảnh khắc»; Esc", cauHinh: "C1", expected: "trình xem ảnh mở, ảnh hiện (cao > 0), tiêu điểm vào trong; Esc đóng", status: moAnh && xem?.imgs.some((i) => i.h > 0) && conXem === 0 ? "PASS" : "FAIL", evidence: ["EV-N14-BANG-XEM-ANH-C1"], ghiChu: `nút mở ${moAnh ? `${moAnh.w}×${moAnh.h}` : "không thấy"}; trình xem: ${xem ? `«${xem.ten}» ${xem.w}×${xem.h}, ảnh ${xem.imgs.map((i) => `${i.w}×${i.h}`).join(", ") || "không có img"}, chữ «${xem.chu}»` : "không mở"}; tiêu điểm «${fXem.ten}» (trong dialog: ${fXem.trongDialog}); sau Esc còn ${conXem} dialog` });
        // «Chia sẻ» on the web, as it is; then with a Web Share stub to read what would be sent.
        await denThe(t, "B1", "center");
        const b1s = await theCua(t.page, "B1");
        const chiaSe = await nut(t.page, "Chia sẻ", { trong: `[data-testid="community-post-${b1s?.id}"]` });
        const truocCs = { url: await duongDan(t.page), chu: (await chuTrang(t.page)).length, dialog: (await dialogTren(t.page))?.ten ?? null };
        const coShare = await t.page.evaluate(() => ({ share: typeof navigator.share, clipboard: typeof navigator.clipboard?.writeText }));
        t.suKien.xa();
        if (chiaSe) await cham(cdp, chiaSe);
        await t.page.waitForTimeout(1800);
        const sauCs = { url: await duongDan(t.page), chu: (await chuTrang(t.page)).length, dialog: (await dialogTren(t.page))?.ten ?? null };
        const loiCs = t.suKien.xa();
        await anh(t.page, "EV-N14-BANG-CHIA-SE-C1", t);
        ghi({ tc: "TC-N14-BANG-CHIA-SE", screen: "N14.S01", state: `chat-1, thẻ B1; trình duyệt: navigator.share ${coShare.share}, clipboard ${coShare.clipboard}`, action: "chạm «Chia sẻ»", cauHinh: "C1", expected: "mở khay chia sẻ, hoặc chép link và nói đã chép, hoặc nói vì sao chưa chia sẻ được; không im lặng", status: chiaSe && (sauCs.dialog || sauCs.chu !== truocCs.chu || sauCs.url !== truocCs.url) ? "PASS" : "FAIL", evidence: ["EV-N14-BANG-CHIA-SE-C1"], ghiChu: `nút ${chiaSe ? `${chiaSe.w}×${chiaSe.h}` : "không thấy"}; trước/sau: đường ${anId(truocCs.url)}/${anId(sauCs.url)}, dialog ${truocCs.dialog ?? "-"}/${sauCs.dialog ?? "-"}, độ dài chữ trang ${truocCs.chu}/${sauCs.chu}; lỗi trang: ${loiCs.pageerror.join(" | ") || "không"}; console: ${loiCs.console.map((c) => c.text.slice(0, 100)).join(" | ") || "không"}` });
        await t.page.evaluate(() => {
          window.__chiaSe = [];
          Object.defineProperty(navigator, "share", { configurable: true, value: (d) => (window.__chiaSe.push(d), Promise.resolve()) });
        });
        if (chiaSe) await cham(cdp, chiaSe);
        await t.page.waitForTimeout(1200);
        const guiDi = await t.page.evaluate(() => window.__chiaSe);
        ghi({ tc: "TC-N14-BANG-CHIA-SE-LINK", screen: "N14.S01", state: "chat-1, thẻ B1; trình duyệt có Web Share (thay bằng hàm ghi lại, không gửi đi đâu)", action: "chạm «Chia sẻ»", cauHinh: "C1", expected: "nội dung chia sẻ là một link người nhận mở được (https), kèm tên bài", status: guiDi?.length && guiDi.some((d) => /^https:\/\//.test(d.url ?? d.text ?? "")) ? "PASS" : "FAIL", ghiChu: `navigator.share nhận: ${guiDi?.length ? guiDi.map((d) => JSON.stringify({ title: d.title ?? null, text: anId(d.text ?? null), url: anId(d.url ?? null) })).join(", ") : "không lần nào"}` });
        // Like, and what a screen reader is told.
        const thich = await t.page.evaluate((id) => {
          const b = [...document.querySelectorAll(`[data-testid="community-post-${id}"] [role="button"]`)].find((x) => /^(Thích bài|Bỏ thích bài)/.test(x.getAttribute("aria-label") ?? ""));
          if (!b) return null;
          b.scrollIntoView({ block: "center", behavior: "instant" });
          const r = b.getBoundingClientRect();
          return { x: r.left + r.width / 2, y: r.top + r.height / 2, ten: b.getAttribute("aria-label") };
        }, b1s?.id);
        if (thich && /^Thích bài/.test(thich.ten)) {
          await cham(cdp, thich);
          await t.page.waitForTimeout(1500);
        }
        const sauThich = await t.page.evaluate((id) => {
          const b = [...document.querySelectorAll(`[data-testid="community-post-${id}"] [role="button"]`)].find((x) => /^(Thích bài|Bỏ thích bài)/.test(x.getAttribute("aria-label") ?? ""));
          return b ? { ten: b.getAttribute("aria-label"), chu: (b.innerText ?? "").trim(), sel: b.getAttribute("aria-selected"), pressed: b.getAttribute("aria-pressed") } : null;
        }, b1s?.id);
        await anh(t.page, "EV-N14-BANG-THICH-C1", t);
        ghi({ tc: "TC-N14-BANG-THICH", screen: "N14.S01", state: "chat-1, thẻ B1 chưa thích", action: "chạm «Thích»", cauHinh: "C1", expected: "đếm lên 1; trạng thái đã thích đọc được (aria-pressed hoặc aria-selected \"true\") chứ không chỉ bằng màu", status: sauThich && /^Bỏ thích bài, 1/.test(sauThich.ten) && (sauThich.pressed === "true" || sauThich.sel === "true") ? "PASS" : "FAIL", evidence: ["EV-N14-BANG-THICH-C1"], ghiChu: `trước «${thich?.ten}»; sau: nhãn «${sauThich?.ten}», chữ «${sauThich?.chu}», aria-pressed ${sauThich?.pressed}, aria-selected ${sauThich?.sel}` });
        // Following is measured in its own part (`bang:theo-doi`), whatever the state it finds.
        // The post's options sheet: its life cycle.
        await denThe(t, "B1", "center");
        const b1o = await theCua(t.page, "B1");
        const them = await nut(t.page, "Thêm lựa chọn cho bài", { trong: `[data-testid="community-post-${b1o?.id}"]` });
        if (them) await cham(cdp, them);
        await t.page.waitForTimeout(1200);
        const sh = await dialogTren(t.page);
        const fSh = await focusHienTai(t.page);
        await anh(t.page, "EV-N14-BANG-SHEET-BAI-C1", t);
        await t.page.keyboard.press("Escape");
        await t.page.waitForTimeout(900);
        const shSau = await dialogTren(t.page);
        const fSau = await focusHienTai(t.page);
        const ine = await inertConLai(t.page);
        ghi({ tc: "TC-N14-BANG-SHEET-BAI", screen: "N14.S01", layer: "L-cong-dong-lua-chon", state: "chat-1, thẻ B1 của người khác", action: "«Thêm lựa chọn cho bài» → đọc sheet → Esc", cauHinh: "C1", expected: "sheet «Lựa chọn cho bài đăng» có role dialog, tiêu điểm vào trong; có «Lưu để đọc lại», «Không quan tâm», «Báo cáo bài viết»; Esc đóng, tiêu điểm về nút mở, không lớp inert sót lại", status: sh?.ten === "Lựa chọn cho bài đăng" && fSh.trongDialog && !shSau && /Thêm lựa chọn cho bài/.test(fSau.ten ?? "") && !ine.length ? "PASS" : "FAIL", evidence: ["EV-N14-BANG-SHEET-BAI-C1"], ghiChu: sh ? `sheet «${sh.ten}» ${sh.rong}×${sh.cao} ở y ${sh.top}; chữ «${sh.chu.slice(0, 160)}»; nút ${sh.nut.map((n) => `${n.ten} ${n.h}`).join(", ")}; tiêu điểm khi mở «${fSh.ten}» (trong dialog ${fSh.trongDialog}); sau Esc: ${shSau ? "còn mở" : "đóng"}, tiêu điểm «${fSau.ten}» (${fSau.tag}); inert còn ${ine.length}` : "không mở" });
        // Save for later, then find it under «Bài đã lưu».
        if (them) await cham(cdp, them);
        await t.page.waitForTimeout(1000);
        const luu = (await nut(t.page, "Lưu để đọc lại", { trongDialog: true })) ?? null;
        if (luu) {
          await cham(cdp, luu);
          await t.page.waitForTimeout(1500);
        } else await t.page.keyboard.press("Escape");
        await bam(t, cdp, "Cài đặt bảng tin", { cho: 1200 });
        const cd = await dialogTren(t.page);
        await anh(t.page, "EV-N14-BANG-CAI-DAT-C1", t);
        ghi({ tc: "TC-N14-BANG-CAI-DAT", screen: "N14.S01", layer: "L-cong-dong-cai-dat", state: "chat-1", action: "«Cài đặt bảng tin» → đọc sheet", cauHinh: "C1", expected: "sheet «Bảng tin của bạn» đủ lối: cá nhân hoá, xoá lịch sử đề xuất, bài đã lưu, bài của tôi, điều muốn giữ, thông báo; nút vùng bấm ≥ 44dp", status: cd?.ten === "Bảng tin của bạn" && cd.nut.filter((n) => !/^Đóng/.test(n.ten)).length >= 6 && cd.nut.every((n) => n.h >= 44) ? "PASS" : "FAIL", evidence: ["EV-N14-BANG-CAI-DAT-C1"], ghiChu: cd ? `sheet «${cd.ten}» ${cd.rong}×${cd.cao} ở y ${cd.top}; nút ${cd.nut.map((n) => `«${n.ten}» ${n.h}`).join(", ")}; «Xóa lịch sử đề xuất» không bấm (xem hàng STATIC TC-N14-XOA-LICH-SU)` : "không mở" });
        so.ghi({ feature: "N14", nenTang: "web", method: "STATIC", layer: "L-cong-dong-cai-dat", issue: null, evidence: ["EV-N14-BANG-CAI-DAT-C1"], tc: "TC-N14-XOA-LICH-SU", screen: "N14.S01", state: "sheet «Bảng tin của bạn»", action: "đọc mã của «Xóa lịch sử đề xuất» (không bấm)", cauHinh: "-", expected: "xoá lịch sử đề xuất (không lấy lại được) có bước hỏi hoặc lối hoàn tác", status: "FAIL", ghiChu: "CommunityScreen.tsx:122–126: onPress gọi DELETE /v2/community/history ngay, rồi tải lại và đóng sheet; không bước hỏi, không hoàn tác, không câu báo đã xoá. Không bấm trên stack (chỉ đọc mã)." });
        await bam(t, cdp, "Bài đã lưu", { trongDialog: true, cho: 2500 });
        await choOn(t.page, { mang: t.mang });
        const mLuu = await doBangTin(t.page);
        const tieuDeLuu = (await chuTrang(t.page)).includes("Để dành cho một ngày");
        await anh(t.page, "EV-N14-BANG-DA-LUU-C1", t);
        ghi({ tc: "TC-N14-BANG-DA-LUU", screen: "N14.S01", state: "chat-1 vừa lưu B1", action: "«Lưu để đọc lại» trên sheet → «Cài đặt bảng tin» → «Bài đã lưu»", cauHinh: "C1", expected: "danh sách «Để dành cho một ngày» có B1; một tab (hoặc tiêu đề) cho biết đang xem bài đã lưu", status: tieuDeLuu && mLuu?.the.some((x) => x.dau.includes(dauBai("B1").slice(0, 30))) ? "PASS" : "FAIL", evidence: ["EV-N14-BANG-DA-LUU-C1"], ghiChu: `«Lưu để đọc lại»: ${luu ? "chạm" : "không thấy (có thể đã lưu)"}; tiêu đề «Để dành cho một ngày»: ${tieuDeLuu ? "có" : "không"}; ${mLuu?.the.length ?? 0} thẻ; tab: ${mLuu?.tabs.map((x) => `«${x.ten}» màu ${x.mau}`).join(", ")}` });
        await bam(t, cdp, "Dành cho bạn", { cho: 2500 });
        await choOn(t.page, { mang: t.mang });
        // «Báo cáo bài viết»: open the report sheet on the post, and leave with «Thôi».
        await denThe(t, "B1", "center");
        const b1r = await theCua(t.page, "B1");
        const them2 = await nut(t.page, "Thêm lựa chọn cho bài", { trong: `[data-testid="community-post-${b1r?.id}"]` });
        if (them2) await cham(cdp, them2);
        await t.page.waitForTimeout(1000);
        const bc = await bam(t, cdp, "Báo cáo bài viết", { trongDialog: true, cho: 2600 });
        await choOn(t.page, { mang: t.mang });
        const dBc = await dialogTren(t.page);
        const uBc = await duongDan(t.page);
        await anh(t.page, "EV-N14-BANG-BAO-CAO-C1", t);
        const thoi = await bam(t, cdp, "Thôi", { trongDialog: true, cho: 1200 });
        const dSau = await dialogTren(t.page);
        ghi({ tc: "TC-N14-BANG-BAO-CAO", screen: "N14.S04", layer: "L-cong-dong-bao-cao", state: "chat-1, bài B1 của chat-0", action: "sheet lựa chọn → «Báo cáo bài viết» → «Thôi» (không gửi)", cauHinh: "C1", expected: "mở chi tiết bài với sheet «Báo cáo bài»; «Thôi» đóng sheet, ở lại bài", status: bc && dBc?.ten === "Báo cáo bài" && thoi && !dSau && /\/community\/posts\//.test(await duongDan(t.page)) ? "PASS" : "FAIL", evidence: ["EV-N14-BANG-BAO-CAO-C1"], ghiChu: `tới ${anId(uBc)}; sheet ${dBc ? `«${dBc.ten}» ${dBc.rong}×${dBc.cao}, chữ «${dBc.chu.slice(0, 160)}»` : "không"}; «Thôi»: ${thoi ? "chạm" : "không thấy"}; sau đó sheet ${dSau ? `«${dSau.ten}» còn` : "đóng"}` });
        await bam(t, cdp, "Quay lại", { cho: 2200 });
        const axe = await chayAxe(t.page);
        ghi({ tc: "TC-N14-BANG-AXE", screen: "N14.S01", state: "chat-1, bảng tin có bài", action: "axe-core trên bảng tin", cauHinh: "C1", expected: "không vi phạm serious/critical", status: axe.vi.filter((v) => ["serious", "critical"].includes(v.impact)).length === 0 ? "PASS" : "FAIL", ghiChu: `vi phạm: ${axe.vi.map((v) => `${v.rule}(${v.impact})×${v.so}: ${v.vd.map((x) => x.target).join(" ; ")}`).join(", ") || "không"}; contrast chưa rõ: ${axe.chuaRo.map((v) => v.so).reduce((a, b) => a + b, 0)}` });
      }
      if (cfg === "C8") {
        // The feed settings sheet in a low window.
        await bam(t, cdp, "Cài đặt bảng tin", { cho: 1200 });
        const cd = await dialogTren(t.page);
        await anh(t.page, "EV-N14-BANG-CAI-DAT-C8", t);
        const cuoi = await nut(t.page, "Thông báo", { trongDialog: true });
        ghi({ tc: "TC-N14-BANG-CAI-DAT", screen: "N14.S01", layer: "L-cong-dong-cai-dat", state: "chat-1, cửa sổ 390×460", action: "«Cài đặt bảng tin»", cauHinh: "C8", expected: "sheet không cao quá 82% cửa sổ; lựa chọn cuối («Thông báo») cuộn tới được và chạm được", status: cd && cd.cao <= 460 * 0.82 + 1 && cuoi ? "PASS" : "FAIL", evidence: ["EV-N14-BANG-CAI-DAT-C8"], ghiChu: cd ? `sheet ${cd.rong}×${cd.cao} ở y ${cd.top} (${Math.round((cd.cao / 460) * 100)}% cửa sổ); «Thông báo» ${cuoi ? `chạm được ${cuoi.w}×${cuoi.h}` : "không chạm được"}` : "không mở" });
        await t.page.keyboard.press("Escape");
      }
      await t.context.close();
    }
    if (anhBang.length >= 3) await ghepAnh(mt.browser, anhBang.filter((a) => ["C1", "C2", "C3"].includes(a.nhan)).map((a) => ({ file: join(mt.out, "jpg", `${a.id}.jpg`), nhan: a.nhan })), join(mt.out, "jpg", "EV-N14-BANG-ghep.jpg"), { tieuDe: "N14 · bảng tin có bài, chat-1, C1–C3" });
    // Following the author from one card, read on the other card by the same author. Two taps on B1
    // (whatever state they find), B2 read after each: the run ends in the state it began with.
    if (chayPhan("bang", "theo-doi")) {
      const t = await mo("C1", "chat-1", "/community");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(2500);
      await choOn(t.page, { mang: t.mang });
      const NUT_THEO = /^(Theo dõi tác giả|Bỏ theo dõi tác giả)$/;
      const nhanTheo = async (k) => (await theCua(t.page, k))?.hanhDong.find((h) => NUT_THEO.test(h.ten))?.ten ?? null;
      const buoc = [];
      let daChup = false;
      for (let i = 0; i < 2; i++) {
        await denThe(t, "B1", "center");
        const b1 = await theCua(t.page, "B1");
        const truocB1 = await nhanTheo("B1");
        const p = truocB1 ? await nut(t.page, truocB1, { trong: `[data-testid="community-post-${b1.id}"]` }) : null;
        if (p) await cham(cdp, p);
        await t.page.waitForTimeout(1500);
        const sauB1 = await nhanTheo("B1");
        await denThe(t, "B2", "start");
        const b2 = await nhanTheo("B2");
        buoc.push({ truocB1, sauB1, b2 });
        if (!daChup && sauB1 && b2 && sauB1 !== b2) {
          await anh(t.page, "EV-N14-BANG-THEO-DOI-C1", t, { chuThich: [{ selector: '[data-testid="community-screen"] [aria-label$="heo dõi tác giả"]' }] });
          daChup = true;
        }
      }
      if (!daChup) await anh(t.page, "EV-N14-BANG-THEO-DOI-C1", t);
      const f = await goi("GET", "/v2/community/follows", undefined, await phienCua("chat-1"));
      const dangTheo = (f.json?.follows ?? []).some((x) => x.kind === "person" && x.target === idNguoi("chat-0"));
      await bam(t, cdp, "Đang theo dõi", { cho: 2500 });
      await choOn(t.page, { mang: t.mang });
      const mTd = await doBangTin(t.page);
      const khongKhop = buoc.filter((b) => b.sauB1 && b.b2 && b.sauB1 !== b.b2);
      ghi({ tc: "TC-N14-BANG-THEO-DOI", screen: "N14.S01", state: "chat-1; B1 và B2 cùng của chat-0 trên «Dành cho bạn»", action: "chạm nút theo dõi trên thẻ B1 hai lần; sau mỗi lần đọc nút trên thẻ B2; rồi tab «Đang theo dõi»", cauHinh: "C1", expected: "mỗi chạm đổi nút trên mọi thẻ của cùng tác giả (B1 và B2 luôn nói giống nhau); «Đang theo dõi» có bài của chat-0 khi đang theo dõi", status: buoc.every((b) => b.truocB1 && b.sauB1 && b.truocB1 !== b.sauB1) && khongKhop.length === 0 && (!dangTheo || mTd?.the.length >= 1) ? "PASS" : "FAIL", evidence: ["EV-N14-BANG-THEO-DOI-C1"], ghiChu: `${buoc.map((b, i) => `lần ${i + 1}: B1 «${b.truocB1}» → «${b.sauB1}», B2 «${b.b2}»`).join("; ")}; máy chủ: chat-1 ${dangTheo ? "đang" : "không"} theo dõi chat-0; «Đang theo dõi»: ${mTd?.the.length ?? 0} thẻ` });
      await t.context.close();
    }
    // «Không quan tâm», by chat-2: the post leaves the feed; is there a way back?
    if (chayPhan("bang", "an")) {
      const t = await mo("C1", "chat-2", "/community");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(2500);
      await choOn(t.page, { mang: t.mang });
      const truoc = await doBangTin(t.page);
      const coB2 = truoc?.the.some((x) => x.dau.includes(dauBai("B2").slice(0, 30)));
      let ketQua = "B2 không có trên bảng tin của chat-2 (có thể đã ẩn ở lượt trước)";
      if (coB2) {
        await denThe(t, "B2", "center");
        const b2 = await theCua(t.page, "B2");
        const them = await nut(t.page, "Thêm lựa chọn cho bài", { trong: `[data-testid="community-post-${b2?.id}"]` });
        if (them) await cham(cdp, them);
        await t.page.waitForTimeout(1000);
        const kq = await bam(t, cdp, "Không quan tâm", { trongDialog: true, cho: 1800 });
        const sau = await doBangTin(t.page);
        const chu = await chuTrang(t.page);
        await anh(t.page, "EV-N14-BANG-AN-C1", t);
        ketQua = `«Không quan tâm»: ${kq ? "chạm" : "không thấy"}; sau: ${sau?.the.length ?? 0} thẻ, B2 ${sau?.the.some((x) => x.dau.includes(dauBai("B2").slice(0, 30))) ? "còn" : "biến mất"}; câu báo/hoàn tác: ${/hoàn tác|Đã ẩn|Hoàn tác|khôi phục/i.test(chu) ? "có" : "không"}`;
      }
      await t.page.reload({ waitUntil: "domcontentloaded" });
      await t.page.waitForTimeout(2500);
      await choOn(t.page, { mang: t.mang });
      const sauTai = await doBangTin(t.page);
      await bam(t, cdp, "Cài đặt bảng tin", { cho: 1200 });
      const cd = await dialogTren(t.page);
      ghi({ tc: "TC-N14-BANG-AN", screen: "N14.S01", layer: "L-cong-dong-lua-chon", state: "chat-2 (đã bật cá nhân hoá), bài B2 của chat-0", action: "sheet lựa chọn → «Không quan tâm»; tải lại; mở «Bảng tin của bạn»", cauHinh: "C1", expected: "bài rời bảng tin kèm một câu và lối hoàn tác (hoặc có chỗ xem lại bài đã ẩn)", status: /hoàn tác|khôi phục|đã ẩn/i.test(`${ketQua} ${cd?.chu ?? ""}`) && !/câu báo\/hoàn tác: không/.test(ketQua) ? "PASS" : "FAIL", evidence: coB2 ? ["EV-N14-BANG-AN-C1"] : [], ghiChu: `${ketQua}; sau khi tải lại B2 ${sauTai?.the.some((x) => x.dau.includes(dauBai("B2").slice(0, 30))) ? "hiện lại" : "vẫn ẩn"}; sheet «Bảng tin của bạn»: ${cd?.nut.map((n) => n.ten).join(", ") ?? "-"} (không có mục bài đã ẩn)` });
      await t.context.close();
    }
  }

  // ------------------------------------------------------------------ ws
  // Realtime through a relay (see the header): the caption, the update band, and what a
  // reconnect does to the post detail.
  if (chay("ws")) {
    const relays = [];
    /** Route the page's community sockets through Node; `nhat` collects the server's frames. */
    const noiRelay = (noi, nhat) =>
      noi.routeWebSocket(/\/v2\/community\/stream/, (ws) => {
        const s = new WebSocket(mt.api.replace(/^http/, "ws") + "/v2/community/stream");
        const hang = [];
        const muc = { mo: Date.now(), khung: [] };
        nhat.push(muc);
        relays.push(s);
        s.onopen = () => {
          for (const m of hang.splice(0)) s.send(m);
        };
        ws.onMessage((m) => (s.readyState === 1 ? s.send(m) : hang.push(m)));
        s.onmessage = (e) => {
          const d = String(e.data);
          // Mask the post id and the server's epoch-millisecond clock: neither is a measurement,
          // and the 13-digit clock trips the repo guard's long-number rule in the matrix.
          muc.khung.push(d.trim().replace(/"post_id":"[^"]+"/, '"post_id":"[id]"').replace(/"occurred_at_ms":\d+/, '"occurred_at_ms":"[ms]"').slice(0, 90));
          ws.send(d);
        };
        s.onclose = () => ws.close().catch(() => undefined);
        s.onerror = () => undefined;
        ws.onClose(() => {
          try {
            s.close();
          } catch {
            /* already closed */
          }
        });
      });
    const dongRelay = () => {
      for (const s of relays.splice(0)) s.close();
    };
    /** `trangMoi` with the relay installed on the context before the app's first socket. */
    const moCoRelay = async (cfg, persona, path, nhat) => {
      const context = await taoContext(mt.browser, cauHinh(cfg));
      await noiRelay(context, nhat);
      const page = await context.newPage();
      const mang = theoDoiMang(page);
      const suKien = ghiSuKien(page);
      const phien = await layPhien(mt.api, P(persona), phienDir);
      await page.goto(new URL("/favicon.ico", mt.base).toString(), { waitUntil: "commit" }).catch(() => undefined);
      await ganPhien(page, mt.api, phien);
      const on = await moLanh(page, mt.base, path, { mang });
      return { context, page, mang, suKien, on };
    };
    /** A change to a public post by somebody else (chat-2's like on B1, toggled): every reader gets feed.changed. */
    const doiCongKhai = async () => {
      const b1 = await timBai("B1");
      const p2 = await phienCua("chat-2");
      const hien = await goi("GET", `/v2/community/posts/${b1.id}`, undefined, p2);
      return goi(hien.json?.liked ? "DELETE" : "PUT", `/v2/community/posts/${b1.id}/like`, undefined, p2);
    };
    for (const cfg of ["C1", "C2", "C9"]) {
      if (!chayPhan("ws", cfg)) continue;
      const nhat = [];
      const t = await moCoRelay(cfg, "chat-0", "/community", nhat);
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(2500);
      await choOn(t.page, { mang: t.mang });
      const m0 = await doBangTin(t.page);
      // Frames of the band's opacity from the moment the change is sent.
      await t.page.evaluate(() => {
        const w = window;
        w.__dai = [];
        w.__daiDung = false;
        const t0 = performance.now();
        const buoc = () => {
          if (w.__daiDung) return;
          // The button carries an arrow glyph (private-use range) before its words.
          const b = [...document.querySelectorAll('[role="button"]')].find((x) => (x.innerText ?? "").replace(/[-]/g, "").trim() === "Bảng tin có cập nhật" && x.getClientRects().length);
          let op = null;
          if (b) {
            op = 1;
            for (let n = b; n && n !== document.body; n = n.parentElement) op *= Number(getComputedStyle(n).opacity);
          }
          const cuoi = w.__dai[w.__dai.length - 1];
          const o = op === null ? null : Math.round(op * 100) / 100;
          if (!cuoi || cuoi.o !== o) w.__dai.push({ t: Math.round(performance.now() - t0), o });
          requestAnimationFrame(buoc);
        };
        requestAnimationFrame(buoc);
      });
      const doi = cfg === "C1" && (await timBai("B4"))?.status !== "approved" ? "duyệt B4 qua API (chat-15)" : "chat-2 đổi lượt thích B1";
      let kq;
      if (doi.startsWith("duyệt")) {
        const b4 = await timBai("B4");
        kq = await goi("POST", `/v2/community/posts/${b4.id}/review`, { revision: b4.revision, approve: true, reason: "Đúng chủ đề đi chơi, không có thông tin riêng." }, await phienCua("chat-15"));
      } else kq = await doiCongKhai();
      let dai = null;
      for (let i = 0; i < 40 && !dai; i++) {
        await t.page.waitForTimeout(250);
        dai = (await doBangTin(t.page))?.dai ?? null;
      }
      await t.page.waitForTimeout(600);
      const m1 = await doBangTin(t.page);
      const khungDai = await t.page.evaluate(() => {
        window.__daiDung = true;
        return window.__dai;
      });
      const tab = m1?.tabs[0];
      const chong = dai && tab ? Math.max(0, Math.min(dai.y + dai.h, tab.y + tab.h) - Math.max(dai.y, tab.y)) : null;
      await anh(t.page, `EV-N14-WS-DAI-${cfg}`, t, { chuThich: dai ? [{ rect: { x: dai.x, y: dai.y, w: dai.w, h: dai.h } }] : [] });
      const moDau = khungDai.find((k) => k.o !== null);
      const trongC9 = cfg === "C9" ? moDau && moDau.o >= 0.99 : true;
      ghi({ tc: "TC-N14-WS-DAI", screen: "N14.S01", layer: "MO-cong-dong-dai", state: `chat-0 trên «Dành cho bạn», stream nối (qua relay); ${doi}`, action: "chờ sự kiện của stream", cauHinh: cfg, expected: `dải «Bảng tin có cập nhật» hiện, không đè lên tab hay tiêu đề${cfg === "C9" ? "; giảm chuyển động thì hiện ngay (không mờ dần)" : ""}`, status: dai && chong === 0 && trongC9 ? "PASS" : "FAIL", evidence: [`EV-N14-WS-DAI-${cfg}`], ghiChu: `dòng phụ lúc nối «${m0?.phu}»; ${doi}: ${kq.status}${kq.code ? ` ${kq.code}` : ""}; khung stream: ${nhat.flatMap((n) => n.khung).slice(-4).join(" ")}; dải: ${dai ? `${dai.w}×${dai.h} tại (${dai.x},${dai.y})` : "không hiện trong 10 s"}; tab «${tab?.ten}» y ${tab?.y}–${tab ? tab.y + tab.h : "-"}; chồng ${chong ?? "-"}px; độ mờ theo khung: ${khungDai.slice(0, 8).map((k) => `${k.o ?? "-"}@${k.t}`).join(" → ")}` });
      if (cfg === "C1") {
        ghi({ tc: "TC-N14-WS-KET-NOI", screen: "N14.S01", state: "chat-0, stream nối (qua relay)", action: "mở tab Cộng đồng", cauHinh: "C1", expected: "khi stream nối, dòng phụ đổi thành «Những câu chuyện đang tiếp nối»", status: m0?.phu === "Những câu chuyện đang tiếp nối" ? "PASS" : "FAIL", evidence: ["EV-N14-WS-DAI-C1"], ghiChu: `dòng phụ «${m0?.phu}»; ${nhat.length} kết nối, khung đầu ${nhat[0]?.khung[0] ?? "-"}` });
        // Tapping the band is measured only when the event was a new post (B4's approval).
        if (dai && doi.startsWith("duyệt")) {
          const nTruoc = m1?.the.length ?? 0;
          await cham(cdp, { x: dai.x + dai.w / 2, y: dai.y + dai.h / 2 });
          await t.page.waitForTimeout(2500);
          const m2 = await doBangTin(t.page);
          const cuon = await viTriCuon(t.page);
          ghi({ tc: "TC-N14-WS-DAI-CHAM", screen: "N14.S01", state: "chat-0, dải «Bảng tin có cập nhật» đang hiện", action: "chạm dải", cauHinh: "C1", expected: "bảng tin tải lại, về đầu, bài mới (B4 vừa duyệt) hiện; dải biến mất", status: !m2?.dai && m2?.the.some((x) => x.dau.includes(dauBai("B4").slice(0, 30))) && (cuon.top ?? 0) <= 4 ? "PASS" : "FAIL", ghiChu: `thẻ trước ${nTruoc}, sau ${m2?.the.length}; B4 ${m2?.the.some((x) => x.dau.includes(dauBai("B4").slice(0, 30))) ? "có" : "không"}; dải ${m2?.dai ? "còn" : "hết"}; cuộn ${cuon.top}` });
        }
      }
      await t.context.close();
    }
    // The detail: a reconnect clears the post and remounts the comment box.
    if (chayPhan("ws", "chi-tiet")) {
      const nhat = [];
      const t = await moCoRelay("C1", "chat-1", "/community", nhat);
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(2000);
      await choOn(t.page, { mang: t.mang });
      await t.page.evaluate(() => {
        const w = window;
        w.__ct = [];
        w.__ctDung = false;
        const t0 = performance.now();
        const buoc = () => {
          if (w.__ctDung) return;
          const man = [...document.querySelectorAll('[data-testid="community-post-detail"]')].filter((x) => x.getClientRects().length).pop();
          const coThe = !!man && [...man.querySelectorAll('[data-testid^="community-post-"]')].some((e) => /^community-post-[0-9a-f-]{36}$/.test(e.dataset.testid));
          const dangMo = !!man && (man.innerText ?? "").includes("Đang mở câu chuyện…");
          const trangThai = !man ? "chưa" : coThe ? "bài" : dangMo ? "đang mở" : "trống";
          const cuoi = w.__ct[w.__ct.length - 1];
          const t = Math.round(performance.now() - t0);
          if (cuoi && cuoi.s === trangThai) {
            cuoi.n += 1;
            cuoi.den = t;
          } else w.__ct.push({ s: trangThai, tu: t, den: t, n: 1 });
          requestAnimationFrame(buoc);
        };
        requestAnimationFrame(buoc);
      });
      const vao = await moChiTiet(t, cdp, "B1");
      await t.page.waitForTimeout(3500);
      const chuoi = await t.page.evaluate(() => window.__ct);
      const bai = chuoi.filter((k) => k.s === "bài");
      const matSauKhiHien = chuoi.findIndex((k, i) => i > 0 && k.s !== "bài" && chuoi.slice(0, i).some((x) => x.s === "bài"));
      ghi({ tc: "TC-N14-WS-NHAY", screen: "N14.S04", state: "chat-1, stream nối (qua relay)", action: "từ bảng tin mở chi tiết B1", cauHinh: "C1", expected: "thẻ bài hiện một lần và đứng yên; không biến mất rồi hiện lại khi stream gửi «sync»", status: vao && bai.length >= 1 && matSauKhiHien === -1 ? "PASS" : "FAIL", ghiChu: `vào chi tiết: ${vao ? "có" : "không"}; chuỗi khung: ${chuoi.map((k) => `${k.s}×${k.n}@${k.tu}–${k.den}`).join(" → ")}; khung stream: ${nhat.flatMap((n) => n.khung).join(" ")}` });
      // A draft in the comment box, then the server drops the socket (a deploy, a network blip).
      const nhap = "Một lời nháp chưa gửi, gõ dở giữa chừng";
      await go(t, cdp, "Bình luận", nhap);
      const truoc = await giaTri(t.page, "Bình luận");
      await anh(t.page, "EV-N14-WS-MAT-CHU-TRUOC-C1", t);
      const soNoi = nhat.length;
      dongRelay();
      await t.page.waitForTimeout(5000);
      const sau = await giaTri(t.page, "Bình luận");
      await anh(t.page, "EV-N14-WS-MAT-CHU-SAU-C1", t);
      ghi({ tc: "TC-N14-WS-MAT-CHU", screen: "N14.S04", state: "chat-1 đang gõ bình luận ở chi tiết B1; stream nối (qua relay)", action: "máy chủ đóng socket (relay đóng phía máy chủ), app tự nối lại", cauHinh: "C1", expected: "chữ đang gõ trong ô bình luận còn nguyên sau khi stream nối lại", status: truoc === nhap && sau === nhap ? "PASS" : "FAIL", evidence: ["EV-N14-WS-MAT-CHU-TRUOC-C1", "EV-N14-WS-MAT-CHU-SAU-C1"], ghiChu: `trước «${truoc}»; kết nối trước ${soNoi}, sau ${nhat.length}; khung mới: ${nhat.slice(soNoi).flatMap((n) => n.khung).join(" ") || "không"}; ô bình luận sau 5 s «${sau ?? "(không thấy ô)"}»` });
      // The same draft, then the page is hidden and shown again (switching apps or tabs).
      await go(t, cdp, "Bình luận", nhap);
      const truoc2 = await giaTri(t.page, "Bình luận");
      const soNoi2 = nhat.length;
      await t.page.evaluate(() => {
        Object.defineProperty(document, "visibilityState", { configurable: true, get: () => "hidden" });
        document.dispatchEvent(new Event("visibilitychange"));
      });
      await t.page.waitForTimeout(800);
      await t.page.evaluate(() => {
        Object.defineProperty(document, "visibilityState", { configurable: true, get: () => "visible" });
        document.dispatchEvent(new Event("visibilitychange"));
      });
      await t.page.waitForTimeout(4000);
      const sau2 = await giaTri(t.page, "Bình luận");
      ghi({ tc: "TC-N14-WS-AN-HIEN", screen: "N14.S04", state: "chat-1 đang gõ bình luận ở chi tiết B1; stream nối (qua relay)", action: "trang ẩn rồi hiện lại (visibilitychange, xấp xỉ chuyển app)", cauHinh: "C1", expected: "chữ đang gõ còn nguyên", status: truoc2 === nhap && sau2 === nhap ? "PASS" : "FAIL", ghiChu: `trước «${truoc2}»; kết nối trước ${soNoi2}, sau ${nhat.length}; ô bình luận sau «${sau2 ?? "(không thấy ô)"}»` });
      await t.context.close();
    }
  }

  // ------------------------------------------------------------------ chi-tiet
  // Post detail: baselines of B2 (photo), then chat-0's management, delete question, edit,
  // audience change (on B3), Nếp without AI; and a stranger at the door of a friends' post.
  if (chay("chi-tiet")) {
    const b1 = await timBai("B1");
    const b3 = await timBai("B3");
    /** The reason under one visible button, by the shared reader. */
    const lyDoNut = (page, ten) =>
      page.evaluate(
        ({ ten, src }) => {
          const lyDoCua = (0, eval)(src);
          const b = [...document.querySelectorAll('[role="button"]')].filter((x) => x.getClientRects().length).find((x) => (x.getAttribute("aria-label") || (x.innerText ?? "")).replace(/[-]/g, "").trim() === ten);
          return b ? { tat: b.getAttribute("aria-disabled") === "true", lyDo: lyDoCua(b) } : null;
        },
        { ten, src: LY_DO_SRC },
      );
    for (const cfg of ["C1", "C2", "C3", "C8"]) {
      if (!chayPhan("chi-tiet", cfg)) continue;
      const t = await mo(cfg, "chat-0", "/community");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(2500);
      await choOn(t.page, { mang: t.mang });
      const vao = await moChiTiet(t, cdp, "B2");
      const d = await doChiTiet(t.page);
      const m = await anh(t.page, `EV-N14-CHI-TIET-${cfg}`, t);
      let bl = "";
      if (cfg === "C8") {
        const o = await t.page.evaluate(() => {
          const e = [...document.querySelectorAll('[aria-label="Bình luận"]')].filter((x) => x.getClientRects().length && /^(INPUT|TEXTAREA)$/.test(x.tagName)).pop();
          e?.scrollIntoView({ block: "center", behavior: "instant" });
          const g = [...document.querySelectorAll('[role="button"]')].filter((b) => b.getClientRects().length && (b.innerText ?? "").trim() === "Gửi bình luận").pop();
          const r = e?.getBoundingClientRect();
          const rg = g?.getBoundingClientRect();
          return { o: r ? Math.round(r.top) : null, g: rg ? Math.round(rg.bottom) : null, h: innerHeight };
        });
        await anh(t.page, "EV-N14-CHI-TIET-BL-C8", t);
        bl = `; ô bình luận cuộn tới y ${o.o}, đáy nút gửi ${o.g} trong cửa sổ ${o.h}`;
      }
      ghi({ tc: "TC-N14-CHI-TIET-BASE", screen: "N14.S04", state: "chat-0, chi tiết B2 của chính mình (một ảnh, đã duyệt)", action: "bảng tin → chạm thân B2 (hai lần: lần đầu mở hết)", cauHinh: cfg, expected: "«Một câu chuyện» có «Quay lại»; thẻ bài, «@Nếp · Giúp giữ khoảnh khắc», khu bình luận; không tràn ngang, không chữ bị cắt", status: vao && d?.id && d.quayLai && m.tomTat.tranTrangPx === 0 && m.tomTat.chuBiCat === 0 ? "PASS" : "FAIL", evidence: [`EV-N14-CHI-TIET-${cfg}`, ...(cfg === "C8" ? ["EV-N14-CHI-TIET-BL-C8"] : [])], ghiChu: d ? `meta «${d.meta}»; dải ${d.dai ? `«${d.dai}»` : "không"}; «Quay lại» ${d.quayLai ? "có" : "không"}; tràn trang ${m.tomTat.tranTrangPx}px, chữ bị cắt ${m.tomTat.chuBiCat}, vùng bấm < 48: ${m.tomTat.vungBamNho48}${bl}` : `không vào được chi tiết (${anId(await duongDan(t.page))})` });
      if (cfg === "C1") {
        // The author's management sheet, on B2 (still on the author's feed; B1 leaves it while an edit
        // waits for review, see TC-N14-SUA-BANG-TIN). We are on B2's detail already.
        const moSheet = async () => {
          const them = await nut(t.page, "Thêm lựa chọn cho bài", { trong: '[data-testid="community-post-detail"]' });
          if (them) await cham(cdp, them);
          await t.page.waitForTimeout(1200);
          return them;
        };
        await moSheet();
        const ql = await dialogTren(t.page);
        const fQl = await focusHienTai(t.page);
        await anh(t.page, "EV-N14-QUAN-LY-C1", t);
        ghi({ tc: "TC-N14-QUAN-LY", screen: "N14.S04", layer: "L-cong-dong-quan-ly", state: "chat-0, chi tiết B2 của chính mình (đã duyệt)", action: "«Thêm lựa chọn cho bài»", cauHinh: "C1", expected: "sheet «Quản lý bài» có role dialog, tiêu điểm vào trong; đủ lựa chọn của tác giả: lưu, sửa, cất cho bạn bè, chỉ mình tôi, xoá", status: ql?.ten === "Quản lý bài" && fQl.trongDialog && ["Sửa bài", "Cất lại cho bạn bè", "Chỉ mình tôi", "Xóa bài"].every((x) => ql.nut.some((n) => n.ten === x)) ? "PASS" : "FAIL", evidence: ["EV-N14-QUAN-LY-C1"], ghiChu: ql ? `«${ql.ten}» ${ql.rong}×${ql.cao} ở y ${ql.top}; nút ${ql.nut.filter((n) => !/^Đóng/.test(n.ten)).map((n) => `«${n.ten}» ${n.h}`).join(", ")}; tiêu điểm «${fQl.ten}» (trong dialog ${fQl.trongDialog})` : "không mở" });
        // «Xóa bài»: the question before the delete (never confirmed here).
        const xoa = await bam(t, cdp, "Xóa bài", { trongDialog: true, cho: 800 });
        const ql2 = await dialogTren(t.page);
        const fX = await focusHienTai(t.page);
        await anh(t.page, "EV-N14-XOA-HOI-C1", t);
        const xn = ql2?.nut.find((n) => /^Xác nhận xóa/.test(n.ten));
        await t.page.keyboard.press("Escape");
        await t.page.waitForTimeout(900);
        await moSheet();
        const ql3 = await dialogTren(t.page);
        await t.page.keyboard.press("Escape");
        await t.page.waitForTimeout(800);
        ghi({ tc: "TC-N14-XOA-HOI", screen: "N14.S04", layer: "L-cong-dong-quan-ly", state: "chat-0, sheet «Quản lý bài» của B2", action: "«Xóa bài» (không bấm xác nhận); Esc; mở lại sheet", cauHinh: "C1", expected: "có bước hỏi trước khi xoá, nói rõ mất gì (bài và bình luận); đóng sheet là thôi; mở lại thì về «Xóa bài»", status: xoa && xn && /bình luận/.test(xn.ten) && ql3?.nut.some((n) => n.ten === "Xóa bài") ? "PASS" : "FAIL", evidence: ["EV-N14-XOA-HOI-C1"], ghiChu: `sau chạm: ${xn ? `nút đổi nhãn thành «${xn.ten}»` : "không thấy bước hỏi"}; chữ sheet «${(ql2?.chu ?? "").slice(0, 200)}»; tiêu điểm «${fX.ten}» (trong dialog ${fX.trongDialog}); không có «Thôi» riêng, không vùng thông báo; mở lại: ${ql3?.nut.some((n) => n.ten === "Xóa bài") ? "«Xóa bài»" : "không về nhãn cũ"}` });
        // Editing B1: a new revision goes to review; others keep reading the approved one.
        if (b1 && b1.revision === 1) {
          await bam(t, cdp, "Quay lại", { cho: 2200 });
          await choOn(t.page, { mang: t.mang });
          await moChiTiet(t, cdp, "B1");
          await moSheet();
          await bam(t, cdp, "Sửa bài", { trongDialog: true, cho: 2500 });
          await choOn(t.page, { mang: t.mang });
          const f = await hoTro.doForm(t.page);
          const khoa = f?.radios.map((r) => `«${r.ten}» aria-disabled ${r.tat}`).join(", ");
          await anh(t.page, "EV-N14-SUA-C1", t);
          ghi({ tc: "TC-N14-SUA-KHOA", screen: "N14.S03", state: "chat-0 sửa B1 (form «Viết tiếp câu chuyện»)", action: "đọc hai lựa chọn người đọc", cauHinh: "C1", expected: "người đọc bị khoá khi sửa thì nói vì sao (và cách đổi người đọc: trong sheet quản lý)", status: /khoá|khóa|không đổi|đổi người đọc/i.test(await chuTrang(t.page)) ? "PASS" : "FAIL", evidence: ["EV-N14-SUA-C1"], ghiChu: `tiêu đề «${f?.tieuDe}»; ${khoa}; không câu nào giải thích (chữ trang không có «khoá»/«đổi người đọc»)` });
          // Append a sentence at the end of the body.
          const o = await t.page.evaluate(() => {
            const e = document.querySelector('[data-testid="community-body"]');
            e?.scrollIntoView({ block: "center", behavior: "instant" });
            const r = e?.getBoundingClientRect();
            return r ? { x: r.left + 40, y: r.top + 20 } : null;
          });
          if (o) {
            await cham(cdp, o);
            await t.page.keyboard.press("Control+End");
            await t.page.keyboard.type(" Lần sau sẽ thử thêm bánh mì xíu mại.", { delay: 8 });
          }
          const g = await nut(t.page, "Gửi bản sửa");
          if (g && !g.tat) await cham(cdp, g);
          await hoTro.cho(t.page, async () => /\/community\/posts\/[0-9a-f-]{36}$/.test(await duongDan(t.page)) && !!(await doChiTiet(t.page))?.id, 12_000);
          await t.page.waitForTimeout(1500);
          const sau = await doChiTiet(t.page);
          await anh(t.page, "EV-N14-SUA-GUI-C1", t);
          const cua1 = await goi("GET", `/v2/community/posts/${b1.id}`, undefined, await phienCua("chat-1"));
          ghi({ tc: "TC-N14-SUA", screen: "N14.S03", state: "chat-0, B1 đã duyệt (phiên bản 1)", action: "sheet quản lý → «Sửa bài» → thêm một câu → «Gửi bản sửa»", cauHinh: "C1", expected: "form điền sẵn; gửi xong tác giả thấy bản mới với dải «Đang chờ duyệt · Bản mới chưa xuất hiện công khai»; người khác vẫn đọc bản đã duyệt", status: f?.tieuDe === "Viết tiếp câu chuyện" && f.body?.gt === BAI.B1.body && /^Đang chờ duyệt/.test(sau?.dai ?? "") && /bánh mì xíu mại/.test(sau?.than ?? "") && cua1.json?.body === BAI.B1.body ? "PASS" : "FAIL", evidence: ["EV-N14-SUA-C1", "EV-N14-SUA-GUI-C1"], ghiChu: `form «${f?.tieuDe}», nội dung ${f?.body?.gt === BAI.B1.body ? "điền sẵn đúng" : `«${String(f?.body?.gt).slice(0, 40)}»`}; nút «Gửi bản sửa» ${g ? (g.tat ? "tắt" : "chạm") : "không thấy"}; sau: dải «${sau?.dai}», thân «${(sau?.than ?? "").slice(-50)}»; chat-1 đọc (API) thân ${cua1.json?.body === BAI.B1.body ? "bản đã duyệt" : `«${String(cua1.json?.body).slice(-40)}»`}` });
        } else console.log(`chi-tiet: B1 đã có phiên bản ${b1?.revision}, không sửa lại`);
        // The audience of B3 (friends): only me, then back to friends.
        if (b3) {
          await diToi(t, `/community/posts/${b3.id}`);
          const meta0 = (await doChiTiet(t.page))?.meta;
          await moSheet();
          await bam(t, cdp, "Chỉ mình tôi", { trongDialog: true, cho: 2200 });
          const meta1 = (await doChiTiet(t.page))?.meta;
          await anh(t.page, "EV-N14-NGUOI-DOC-C1", t);
          await moSheet();
          await bam(t, cdp, "Cất lại cho bạn bè", { trongDialog: true, cho: 2200 });
          const meta2 = (await doChiTiet(t.page))?.meta;
          ghi({ tc: "TC-N14-NGUOI-DOC", screen: "N14.S04", layer: "L-cong-dong-quan-ly", state: "chat-0, B3 «Bạn bè»", action: "sheet quản lý → «Chỉ mình tôi»; rồi → «Cất lại cho bạn bè»", cauHinh: "C1", expected: "đổi người đọc có hiệu lực ngay trên thẻ: «Chỉ mình tôi», rồi về «Bạn bè»", status: /Bạn bè$/.test(meta0 ?? "") && /Chỉ mình tôi$/.test(meta1 ?? "") && /Bạn bè$/.test(meta2 ?? "") ? "PASS" : "FAIL", evidence: ["EV-N14-NGUOI-DOC-C1"], ghiChu: `meta: «${meta0}» → «${meta1}» → «${meta2}»` });
        }
        // Nếp on B1, with no AI configured.
        if (b1) {
          await diToi(t, `/community/posts/${b1.id}`);
          await bam(t, cdp, "@Nếp · Giúp giữ khoảnh khắc", { cho: 1200 });
          const dn = await dialogTren(t.page);
          const nutNep = await lyDoNut(t.page, "Đồng ý chia sẻ và gọi Nếp");
          await anh(t.page, "EV-N14-NEP-C1", t);
          ghi({ tc: "TC-N14-NEP-NUT-TAT", screen: "N14.S04", layer: "L-cong-dong-nep", state: "chat-0, sheet «Gọi Nếp», ô yêu cầu trống", action: "đọc nút «Đồng ý chia sẻ và gọi Nếp»", cauHinh: "C1", expected: "nút tắt kèm lý do (viết yêu cầu trước)", status: nutNep?.tat && nutNep.lyDo ? "PASS" : "FAIL", evidence: ["EV-N14-NEP-C1"], ghiChu: `sheet «${dn?.ten}» ${dn?.rong}×${dn?.cao}; nút tắt ${nutNep?.tat}, lý do ${nutNep?.lyDo ? `«${nutNep.lyDo}»` : "không"}` });
          const yeuCau = "Giữ lại cảm giác buổi sáng sương mù.";
          await go(t, cdp, "Bạn muốn Nếp giúp gì?", yeuCau);
          const goiNep = [];
          const nghe = (r) => {
            if (/\/nep$/.test(new URL(r.url()).pathname)) goiNep.push(r.status());
          };
          t.page.on("response", nghe);
          await bam(t, cdp, "Đồng ý chia sẻ và gọi Nếp", { trongDialog: true, cho: 5000 });
          t.page.off("response", nghe);
          const loiNep = await t.page.evaluate(() =>
            [...document.querySelectorAll('[role="alert"]')].filter((a) => a.getClientRects().length).map((a) => {
              const r = a.getBoundingClientRect();
              const tren = document.elementFromPoint(r.left + Math.min(20, r.width / 2), r.top + r.height / 2);
              return { chu: (a.innerText ?? "").trim(), trongDialog: !!a.closest('[role="dialog"]'), thay: !!tren && (tren === a || a.contains(tren)), y: Math.round(r.top) };
            }),
          );
          const conYc = await giaTri(t.page, "Bạn muốn Nếp giúp gì?");
          await anh(t.page, "EV-N14-NEP-LOI-C1", t);
          ghi({ tc: "TC-N14-NEP", screen: "N14.S04", layer: "L-cong-dong-nep", state: "chat-0, sheet «Gọi Nếp»; stack không có khoá AI (engine brain)", action: "gõ yêu cầu → «Đồng ý chia sẻ và gọi Nếp»", cauHinh: "C1", expected: "câu lỗi hiện trong sheet Nếp (thấy được, không nằm dưới lớp phủ); yêu cầu vừa gõ còn nguyên", status: loiNep.some((l) => l.trongDialog && l.thay) && conYc === yeuCau ? "PASS" : "FAIL", evidence: ["EV-N14-NEP-C1", "EV-N14-NEP-LOI-C1"], ghiChu: `POST …/nep: ${goiNep.join(", ") || "không gọi"}; câu lỗi: ${loiNep.map((l) => `«${l.chu}» y ${l.y}, ${l.trongDialog ? "trong sheet" : "ngoài sheet"}, ${l.thay ? "thấy được" : "bị che"}`).join(" | ") || "không có"}; yêu cầu còn «${conYc}»` });
          await t.page.keyboard.press("Escape");
        }
      }
      await t.context.close();
    }
    // The outcome of the edit, read again (the edit itself ran once; it is not repeated): the author's
    // detail shows the new revision under the pending band, others read the approved one.
    if (chi?.has("chi-tiet:sua-doc") && b1) {
      const t = await mo("C1", "chat-0", "/community");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(2500);
      await choOn(t.page, { mang: t.mang });
      // The author finds a post waiting for review under «Bài của tôi · Trạng thái duyệt».
      await bam(t, cdp, "Cài đặt bảng tin", { cho: 1200 });
      await bam(t, cdp, "Bài của tôi · Trạng thái duyệt", { trongDialog: true, cho: 2500 });
      await choOn(t.page, { mang: t.mang });
      const vao = await moChiTiet(t, cdp, "B1");
      const d = await doChiTiet(t.page);
      await anh(t.page, "EV-N14-SUA-DOC-C1", t);
      const cua1 = await goi("GET", `/v2/community/posts/${b1.id}`, undefined, await phienCua("chat-1"));
      ghi({ tc: "TC-N14-SUA", screen: "N14.S03", state: `chat-0, B1 đã duyệt ở phiên bản 1, bản sửa phiên bản ${b1.revision} chờ duyệt (sửa qua «Sửa bài» → «Gửi bản sửa» ở lượt đo trước, EV-N14-SUA-C1, EV-N14-SUA-GUI-C1)`, action: "«Bài của tôi · Trạng thái duyệt» → mở lại chi tiết B1; đọc bản chat-1 nhận (API)", cauHinh: "C1", expected: "tác giả thấy bản mới với dải «Đang chờ duyệt · Bản mới chưa xuất hiện công khai»; người khác vẫn đọc bản đã duyệt", status: vao && /^Đang chờ duyệt/.test(d?.dai ?? "") && /bánh mì xíu mại/.test(d?.than ?? "") && cua1.json?.body === BAI.B1.body ? "PASS" : "FAIL", evidence: ["EV-N14-SUA-C1", "EV-N14-SUA-GUI-C1", "EV-N14-SUA-DOC-C1"], ghiChu: `phiên bản ${b1.revision}; dải «${d?.dai}»; thân tác giả thấy «…${(d?.than ?? "").slice(-45)}»; chat-1 đọc (API) ${cua1.json?.body === BAI.B1.body ? "bản đã duyệt, không có câu mới" : `«${String(cua1.json?.body).slice(-40)}»`}` });
      await t.context.close();
    }
    // After editing an approved post: where the post is on the author's feed and on a reader's.
    if (chi?.has("chi-tiet:sua-bang") && b1) {
      const bang = [];
      for (const ten of ["chat-0", "chat-1"]) {
        const r = await goi("GET", "/v2/community/feed?mode=for_you", undefined, await phienCua(ten));
        bang.push({ ten, co: (r.json?.posts ?? []).some((p) => p.body.startsWith(dauBai("B1"))), so: r.json?.posts?.length ?? 0 });
      }
      const t = await mo("C1", "chat-0", "/community");
      await t.page.waitForTimeout(2500);
      await choOn(t.page, { mang: t.mang });
      const m = await doBangTin(t.page);
      await anh(t.page, "EV-N14-SUA-BANG-TIN-C1", t);
      await t.context.close();
      const cua0 = bang.find((b) => b.ten === "chat-0");
      ghi({ tc: "TC-N14-SUA-BANG-TIN", screen: "N14.S01", state: `chat-0 vừa sửa B1 đã duyệt (phiên bản ${b1.revision} chờ duyệt)`, action: "chat-0 mở tab Cộng đồng; đọc «Dành cho bạn» của chat-0 và chat-1 (API)", cauHinh: "C1", expected: "bài đã duyệt vẫn ở bảng tin của tác giả (kèm dải chờ duyệt cho bản mới), như người khác vẫn thấy bản cũ", status: cua0?.co ? "PASS" : "FAIL", evidence: ["EV-N14-SUA-BANG-TIN-C1"], ghiChu: `${bang.map((b) => `${b.ten}: ${b.so} bài, B1 ${b.co ? "có" : "không"}`).join("; ")}; thẻ trên màn chat-0: ${m?.the.map((x) => `«${x.dau.slice(18, 50)}…»`).join(", ") || "không"}; B1 chỉ còn ở «Bài của tôi · Trạng thái duyệt» (mode mine, pending)` });
    }
    // A stranger (chat-16) at the door of chat-0's friends post, and at an approved public one.
    if (chayPhan("chi-tiet", "nguoi-la") && b3 && b1) {
      const t = await mo("C1", "chat-16", `/community/posts/${b3.id}`);
      await t.page.waitForTimeout(2500);
      await choOn(t.page, { mang: t.mang });
      const d = await doChiTiet(t.page);
      const chu = await chuTrang(t.page);
      await anh(t.page, "EV-N14-NGUOI-LA-C1", t);
      await diToi(t, `/community/posts/${b1.id}`);
      const d2 = await doChiTiet(t.page);
      ghi({ tc: "TC-N14-NGUOI-LA", screen: "N14.S04", state: "chat-16, không là bạn của chat-0", action: "mở thẳng B3 («Bạn bè»), rồi B1 (công khai, đã duyệt)", cauHinh: "C1", expected: "B3: câu «Bài đã được cất riêng hoặc không còn ở đây.», không lộ nội dung; B1 đọc được, bản đã duyệt", status: !chu.includes(dauBai("B3")) && /cất riêng/.test(d?.loi.join(" ") ?? "") && (d2?.than ?? "").startsWith(dauBai("B1")) && !/bánh mì xíu mại/.test(d2?.than ?? "") ? "PASS" : "FAIL", evidence: ["EV-N14-NGUOI-LA-C1"], ghiChu: `B3: câu lỗi ${d?.loi.map((x) => `«${x}»`).join(" ") || "không"}, nội dung lộ ${chu.includes(dauBai("B3")) ? "có" : "không"}; B1: thân «${(d2?.than ?? "").slice(0, 50)}…», bản mới (xíu mại) ${/bánh mì xíu mại/.test(d2?.than ?? "") ? "lộ" : "không lộ"}` });
      await t.context.close();
    }
  }

  // ------------------------------------------------------------------ binh-luan
  // Comments on B1: chat-1 writes (pending), deletes a throwaway; the operator approves; chat-0 replies
  // with a tag; the comment sheet from the feed in a low window.
  if (chay("binh-luan")) {
    const b1 = await timBai("B1");
    const BL1 = "Quán này mở cửa từ mấy giờ vậy bạn?";
    const BL_XOA = "Bình luận thử, sẽ xoá ngay.";
    const TL = "Mở từ sáu giờ sáng nhé, nhớ đi sớm.";
    const docBl = async (ten) => {
      const r = await goi("GET", `/v2/community/posts/${b1.id}/comments?after=`, undefined, await phienCua(ten));
      return [...(r.json?.comments ?? []), ...(r.json?.pending ?? [])];
    };
    /** One comment block by its text: status line, «Trả lời», «Xóa» (with its accessible name). */
    const khoiBl = (page, chuBl) =>
      page.evaluate((chuBl) => {
        const chu = (e) => (e?.innerText ?? "").replace(/[-]/g, "").replace(/\s+/g, " ").trim();
        const t = [...document.querySelectorAll('div[dir="auto"]')].filter((x) => x.getClientRects().length).find((x) => chu(x) === chuBl);
        if (!t) return null;
        let k = t.parentElement;
        while (k && !k.querySelector('[role="img"],img,div[dir="auto"]')) k = k.parentElement;
        k = t.parentElement;
        const nut = [...k.querySelectorAll('[role="button"]')].filter((b) => b.getClientRects().length).map((b) => {
          const r = b.getBoundingClientRect();
          return { ten: b.getAttribute("aria-label") || chu(b), chu: chu(b), x: r.left + r.width / 2, y: r.top + r.height / 2, w: Math.round(r.width), h: Math.round(r.height) };
        });
        return { chu: chu(k), nut, y: Math.round(t.getBoundingClientRect().top) };
      }, chuBl);
    if (chayPhan("binh-luan", "gui") && b1) {
      const t = await mo("C1", "chat-1", "/community");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(2500);
      await choOn(t.page, { mang: t.mang });
      await moChiTiet(t, cdp, "B1");
      const tat = await t.page.evaluate((src) => {
        const lyDoCua = (0, eval)(src);
        const g = [...document.querySelectorAll('[role="button"]')].filter((b) => b.getClientRects().length && (b.innerText ?? "").trim() === "Gửi bình luận").pop();
        return g ? { tat: g.getAttribute("aria-disabled") === "true", lyDo: lyDoCua(g) } : null;
      }, LY_DO_SRC);
      ghi({ tc: "TC-N14-BL-NUT-TAT", screen: "N14.S05", state: "chat-1, chi tiết B1, ô bình luận trống", action: "đọc nút «Gửi bình luận»", cauHinh: "C1", expected: "nút tắt kèm lý do (ADR-0038 §2.2)", status: tat?.tat && tat.lyDo ? "PASS" : "FAIL", ghiChu: `tắt ${tat?.tat}, lý do ${tat?.lyDo ? `«${tat.lyDo}»` : "không"}` });
      if (!(await docBl("chat-1")).some((c) => c.body === BL1)) {
        await go(t, cdp, "Bình luận", BL1);
        await bam(t, cdp, "Gửi bình luận", { cho: 2500 });
        const k = await khoiBl(t.page, BL1);
        const o = await giaTri(t.page, "Bình luận");
        await t.page.evaluate((chuBl) => [...document.querySelectorAll('div[dir="auto"]')].find((x) => (x.innerText ?? "").trim() === chuBl)?.scrollIntoView({ block: "center", behavior: "instant" }), BL1);
        await anh(t.page, "EV-N14-BL-GUI-C1", t);
        ghi({ tc: "TC-N14-BL-GUI", screen: "N14.S05", state: "chat-1, bài công khai B1 của chat-0", action: "gõ bình luận → «Gửi bình luận»", cauHinh: "C1", expected: "bình luận hiện dưới bài với «Đang chờ duyệt · Chỉ bạn thấy»; ô gõ trống lại", status: k && /Đang chờ duyệt · Chỉ bạn thấy/.test(k.chu) && o === "" ? "PASS" : "FAIL", evidence: ["EV-N14-BL-GUI-C1"], ghiChu: k ? `khối «${k.chu.slice(0, 120)}»; nút ${k.nut.map((n) => `«${n.ten}» ${n.w}×${n.h}`).join(", ") || "không"}; ô gõ sau «${o}»` : "không thấy bình luận" });
      } else console.log("binh-luan: bình luận BL1 của chat-1 đã có, không gửi lại");
      // A throwaway comment, deleted with one tap.
      await go(t, cdp, "Bình luận", BL_XOA);
      await bam(t, cdp, "Gửi bình luận", { cho: 2500 });
      const kx = await khoiBl(t.page, BL_XOA);
      const nutXoa = kx?.nut.find((n) => n.chu === "Xóa");
      await t.page.evaluate((chuBl) => [...document.querySelectorAll('div[dir="auto"]')].find((x) => (x.innerText ?? "").trim() === chuBl)?.scrollIntoView({ block: "center", behavior: "instant" }), BL_XOA);
      await t.page.waitForTimeout(400);
      const kx2 = await khoiBl(t.page, BL_XOA);
      const nx = kx2?.nut.find((n) => n.chu === "Xóa");
      await anh(t.page, "EV-N14-BL-XOA-TRUOC-C1", t);
      if (nx) await cham(cdp, nx);
      await t.page.waitForTimeout(1800);
      const conSau = await khoiBl(t.page, BL_XOA);
      const hoi = await dialogTren(t.page);
      const chuSau = await chuTrang(t.page);
      await anh(t.page, "EV-N14-BL-XOA-C1", t);
      ghi({ tc: "TC-N14-BL-XOA", screen: "N14.S05", state: "chat-1, bình luận thử của chính mình (chờ duyệt)", action: "chạm «Xóa» dưới bình luận", cauHinh: "C1", expected: "hỏi lại (hoặc cho hoàn tác) trước khi bình luận mất; tên trợ năng của nút nói xoá bình luận nào", status: nx && (hoi || /Hoàn tác|hoàn tác|Xác nhận/.test(chuSau)) && nx.ten !== "Xóa" ? "PASS" : "FAIL", evidence: ["EV-N14-BL-XOA-TRUOC-C1", "EV-N14-BL-XOA-C1"], ghiChu: `nút «${nx?.ten ?? nutXoa?.ten ?? "không thấy"}» ${nx ? `${nx.w}×${nx.h}` : ""}; sau một chạm: bình luận ${conSau ? "còn" : "mất ngay"}, hộp hỏi ${hoi ? `«${hoi.ten}»` : "không"}, lối hoàn tác ${/Hoàn tác|hoàn tác/.test(chuSau) ? "có" : "không"}` });
      await t.context.close();
    }
    // The operator approves chat-1's comment; comments share the queue.
    if (chayPhan("binh-luan", "duyet") && b1) {
      const cho1 = (await docBl("chat-1")).find((c) => c.body === BL1);
      const t = await mo("C1", "chat-15", "/community/review");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(2500);
      await choOn(t.page, { mang: t.mang });
      const h = await hoTro.doHang(t.page);
      const muc = h.find((x) => x.than === BL1);
      await t.page.evaluate((chuBl) => [...document.querySelectorAll('div[dir="auto"]')].find((x) => (x.innerText ?? "").trim() === chuBl)?.scrollIntoView({ block: "center", behavior: "instant" }), BL1);
      await anh(t.page, "EV-N14-DUYET-BL-C1", t);
      let kq = null;
      if (muc) kq = await hoTro.quyet(t, cdp, BL1, "Câu hỏi về giờ mở cửa, không có thông tin riêng.", "Duyệt công khai");
      const sau = (await docBl("chat-0")).find((c) => c.body === BL1);
      ghi({ tc: "TC-N14-DUYET-BL", screen: "N14.S09", state: "chat-15; bình luận BL1 của chat-1 chờ duyệt", action: "tìm bình luận trong hàng → lý do → «Duyệt công khai»", cauHinh: "C1", expected: "bình luận có trong hàng (chú thích «Bình luận», trạng thái bằng lời); duyệt xong người khác đọc được", status: muc && !/·\s*(pending|review)\b/.test(muc.chuThich) && kq?.bam && sau ? "PASS" : "FAIL", evidence: ["EV-N14-DUYET-BL-C1"], ghiChu: `trong hàng: ${muc ? `«${muc.chuThich}»` : cho1?.status === "approved" ? "không (đã duyệt ở lượt trước)" : "không"}; quyết: ${kq?.bam ? "đã chạm «Duyệt công khai»" : JSON.stringify(kq)}; chat-0 đọc được (API): ${sau ? `có, trạng thái ${sau.status}` : "không"}` });
      await t.context.close();
    }
    // chat-0 replies to the approved comment and tags chat-1.
    if (chayPhan("binh-luan", "tra-loi") && b1) {
      const t = await mo("C1", "chat-0", `/community/posts/${b1.id}`);
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(2500);
      await choOn(t.page, { mang: t.mang });
      const k = await khoiBl(t.page, BL1);
      const tl = k?.nut.find((n) => n.chu === "Trả lời");
      await t.page.evaluate((chuBl) => [...document.querySelectorAll('div[dir="auto"]')].find((x) => (x.innerText ?? "").trim() === chuBl)?.scrollIntoView({ block: "start", behavior: "instant" }), BL1);
      await t.page.waitForTimeout(400);
      const k2 = await khoiBl(t.page, BL1);
      const tl2 = k2?.nut.find((n) => n.chu === "Trả lời");
      if (tl2) await cham(cdp, tl2);
      await t.page.waitForTimeout(900);
      const sauTl = await t.page.evaluate(() => {
        const nhan = [...document.querySelectorAll('div[dir="auto"]')].find((x) => /^Trả lời .+/.test((x.innerText ?? "").trim()) && x.getClientRects().length);
        const o = [...document.querySelectorAll('[aria-label="Bình luận"]')].filter((x) => x.getClientRects().length && /^(INPUT|TEXTAREA)$/.test(x.tagName)).pop();
        const r = nhan?.getBoundingClientRect();
        const ro = o?.getBoundingClientRect();
        return { nhan: nhan ? (nhan.innerText ?? "").trim() : null, nhanY: r ? Math.round(r.top) : null, oY: ro ? Math.round(ro.top) : null, h: innerHeight, focus: document.activeElement === o };
      });
      await anh(t.page, "EV-N14-BL-TRA-LOI-C1", t);
      ghi({ tc: "TC-N14-BL-TRA-LOI", screen: "N14.S05", state: "chat-0, bình luận đã duyệt của chat-1 dưới B1", action: "chạm «Trả lời»", cauHinh: "C1", expected: "ô soạn vào tầm nhìn (hoặc nhận tiêu điểm) và nói đang trả lời ai", status: tl2 && sauTl.nhan && ((sauTl.oY ?? -1) >= 0 && (sauTl.oY ?? 1e9) < sauTl.h || sauTl.focus) ? "PASS" : "FAIL", evidence: ["EV-N14-BL-TRA-LOI-C1"], ghiChu: `«Trả lời» ${tl2 ? `${tl2.w}×${tl2.h}` : tl ? "có nhưng không chạm được" : "không thấy"}; sau chạm: nhãn «${sauTl.nhan}» ở y ${sauTl.nhanY}, ô soạn ở y ${sauTl.oY} trong cửa sổ ${sauTl.h}, tiêu điểm vào ô ${sauTl.focus}` });
      if (!(await docBl("chat-0")).some((c) => c.body === TL) && tl2) {
        await bam(t, cdp, "Tag bạn bè", { cho: 1500 });
        const hop = await t.page.evaluate(() => [...document.querySelectorAll('[role="checkbox"]')].filter((x) => x.getClientRects().length).map((x) => ({ chu: (x.innerText ?? "").trim(), checked: x.getAttribute("aria-checked") })));
        const cb = await nut(t.page, "Chat Test 02", { batDau: true });
        if (cb) await cham(cdp, cb);
        await t.page.waitForTimeout(600);
        const hop2 = await t.page.evaluate(() => [...document.querySelectorAll('[role="checkbox"]')].filter((x) => x.getClientRects().length).map((x) => ({ chu: (x.innerText ?? "").trim(), checked: x.getAttribute("aria-checked") })));
        await anh(t.page, "EV-N14-BL-TAG-C1", t);
        await go(t, cdp, "Bình luận", TL);
        await bam(t, cdp, "Gửi bình luận", { cho: 2500 });
        const gui = (await docBl("chat-0")).find((c) => c.body === TL);
        ghi({ tc: "TC-N14-BL-TAG", screen: "N14.S05", state: "chat-0 đang trả lời chat-1", action: "«Tag bạn bè» → chọn «Chat Test 02» → gõ → «Gửi bình luận»", cauHinh: "C1", expected: "danh sách tag là checkbox đọc được trạng thái (aria-checked); trả lời gửi đi kèm tag, chờ duyệt", status: hop2.some((x) => /Chat Test 02/.test(x.chu) && x.checked === "true") && gui?.parent_id && gui.mentions?.length === 1 ? "PASS" : "FAIL", evidence: ["EV-N14-BL-TAG-C1"], ghiChu: `trước chọn: ${hop.map((x) => `«${x.chu}» aria-checked ${x.checked}`).join(", ") || "không có checkbox"}; sau: ${hop2.map((x) => `«${x.chu}» aria-checked ${x.checked}`).join(", ")}; đã gửi: ${gui ? `có, trả lời ${gui.parent_id ? "đúng bình luận" : "không có cha"}, tag ${gui.mentions?.length ?? 0}, trạng thái ${gui.status}` : "không"}` });
      } else console.log("binh-luan: trả lời TL đã có hoặc không chạm được «Trả lời»");
      await t.context.close();
      // The operator approves the reply, so the tag reaches chat-1 as a notification.
      const cho0 = (await docBl("chat-0")).find((c) => c.body === TL);
      if (cho0 && cho0.status !== "approved") {
        const u = await mo("C1", "chat-15", "/community/review");
        const cdpU = await cdpCua(u.page);
        await u.page.waitForTimeout(2500);
        await choOn(u.page, { mang: u.mang });
        const kq = await hoTro.quyet(u, cdpU, TL, "Trả lời giờ mở cửa, không có thông tin riêng.", "Duyệt công khai");
        console.log(`binh-luan: duyệt trả lời của chat-0: ${JSON.stringify(kq)}`);
        await u.context.close();
      }
    }
    // The comment sheet from the feed, in a low window; «Mở rộng» and back.
    if (chayPhan("binh-luan", "sheet") && b1) {
      const t = await mo("C8", "chat-1", "/community");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(2500);
      await choOn(t.page, { mang: t.mang });
      await denThe(t, "B1", "center");
      const the = await theCua(t.page, "B1");
      const moBl = the?.hanhDong.find((h) => /^Mở bình luận/.test(h.ten));
      const p = moBl ? await nut(t.page, moBl.ten, { trong: `[data-testid="community-post-${the.id}"]` }) : null;
      if (p) await cham(cdp, p);
      await t.page.waitForTimeout(1500);
      const d = await dialogTren(t.page);
      const oBl = await t.page.evaluate(() => {
        const dd = [...document.querySelectorAll('[role="dialog"]')].filter((x) => x.getClientRects().length).pop();
        const o = dd && [...dd.querySelectorAll('[aria-label="Bình luận"]')].find((x) => /^(INPUT|TEXTAREA)$/.test(x.tagName));
        const g = dd && [...dd.querySelectorAll('[role="button"]')].find((b) => (b.innerText ?? "").trim() === "Gửi bình luận");
        g?.scrollIntoView({ block: "end", behavior: "instant" });
        const ro = o?.getBoundingClientRect();
        const rg = g?.getBoundingClientRect();
        const rd = dd?.getBoundingClientRect();
        return { o: ro ? Math.round(ro.top) : null, g: rg ? Math.round(rg.bottom) : null, dTop: rd ? Math.round(rd.top) : null, dDay: rd ? Math.round(rd.bottom) : null, h: innerHeight };
      });
      await anh(t.page, "EV-N14-BL-SHEET-C8", t);
      ghi({ tc: "TC-N14-BL-SHEET", screen: "N14.S05", layer: "L-cong-dong-binh-luan", state: "chat-1, cửa sổ 390×460", action: "thẻ B1 → «Mở bình luận»", cauHinh: "C8", expected: "sheet «Bình luận» cao ≤ 82% cửa sổ; cuộn tới được ô gõ và nút gửi", status: d?.ten === "Bình luận" && d.cao <= 460 * 0.82 + 1 && oBl.g !== null && oBl.g <= oBl.h ? "PASS" : "FAIL", evidence: ["EV-N14-BL-SHEET-C8"], ghiChu: d ? `sheet «${d.ten}» ${d.rong}×${d.cao} ở y ${d.top} (${Math.round((d.cao / 460) * 100)}%); ô gõ y ${oBl.o}, đáy nút gửi ${oBl.g} (sau khi cuộn nút vào tầm nhìn)` : "không mở" });
      const mr = await bam(t, cdp, "Mở rộng", { trongDialog: true, cho: 2500 });
      const u1 = await duongDan(t.page);
      await bam(t, cdp, "Quay lại", { cho: 2500 });
      await choOn(t.page, { mang: t.mang });
      const d2 = await dialogTren(t.page);
      const u2 = await duongDan(t.page);
      await anh(t.page, "EV-N14-BL-MO-RONG-C8", t);
      ghi({ tc: "TC-N14-BL-MO-RONG", screen: "N14.S05", layer: "L-cong-dong-binh-luan", state: "chat-1, sheet «Bình luận» mở trên bảng tin", action: "«Mở rộng» → chi tiết bài → «Quay lại»", cauHinh: "C8", expected: "«Mở rộng» tới chi tiết bài; quay lại bảng tin không còn sheet che (hoặc sheet giữ đúng chỗ, dùng được)", status: mr && /\/community\/posts\//.test(u1) && !d2 ? "PASS" : "FAIL", evidence: ["EV-N14-BL-MO-RONG-C8"], ghiChu: `«Mở rộng» ${mr ? "chạm" : "không thấy"} → ${anId(u1)}; về ${anId(u2)}; sheet sau khi về: ${d2 ? `«${d2.ten}» còn mở` : "không"}` });
      await t.context.close();
    }
  }

  // ------------------------------------------------------------------ phu
  // Search, a topic, notifications, «Điều mình muốn giữ», «Bài của tôi».
  if (chay("phu")) {
    if (chayPhan("phu", "tim")) {
      for (const cfg of ["C1", "C2"]) {
        const t = await mo(cfg, "chat-1", "/community");
        const cdp = await cdpCua(t.page);
        await t.page.waitForTimeout(2500);
        await choOn(t.page, { mang: t.mang });
        await go(t, cdp, "Tìm chủ đề", "cà");
        await t.page.keyboard.press("Enter");
        await t.page.waitForTimeout(2500);
        await choOn(t.page, { mang: t.mang });
        const u = await duongDan(t.page);
        const doTim = () =>
          t.page.evaluate(() => {
            const hien = (e) => e && e.getClientRects().length > 0;
            const cac = [...document.querySelectorAll('div[dir="auto"]')].filter(hien).map((x) => (x.innerText ?? "").replace(/\s+/g, " ").trim());
            const i = cac.indexOf("Chủ đề");
            const j = cac.indexOf("Người chia sẻ");
            return { chuDe: i >= 0 ? cac.slice(i + 1, j > i ? j : undefined).filter((x) => x !== "Theo dõi" && x !== "Đang theo dõi") : [], nguoi: j >= 0 ? cac.slice(j + 1).filter((x) => x !== "Theo dõi" && x !== "Đang theo dõi") : [], coTieuDe: i >= 0 && j >= 0 };
          });
        const kq = await doTim();
        const m = await anh(t.page, `EV-N14-TIM-${cfg}`, t);
        ghi({ tc: "TC-N14-TIM", screen: "N14.S06", state: "chat-1; chủ đề đã duyệt: cà phê, đà lạt, đi bộ, hoàng hôn", action: "ô «Tìm chủ đề» trên bảng tin: «cà» → Enter", cauHinh: cfg, expected: "tới trang tìm, chủ đề «cà phê» kèm số câu chuyện và nút «Theo dõi»; không tràn, không chữ bị cắt", status: /\/community\/search/.test(u) && kq.chuDe.some((x) => /^cà phê/.test(x)) && m.tomTat.tranTrangPx === 0 && m.tomTat.chuBiCat === 0 ? "PASS" : "FAIL", evidence: [`EV-N14-TIM-${cfg}`], ghiChu: `tới ${decodeURIComponent(u)}; chủ đề: ${kq.chuDe.join(" | ") || "không"}; người: ${kq.nguoi.join(" | ") || "không"}; tràn ${m.tomTat.tranTrangPx}px, chữ bị cắt ${m.tomTat.chuBiCat}` });
        if (cfg === "C1") {
          await go(t, cdp, "Chủ đề hoặc người chia sẻ", "Chat Test 01");
          await t.page.waitForTimeout(1800);
          const kq2 = await doTim();
          await anh(t.page, "EV-N14-TIM-NGUOI-C1", t);
          ghi({ tc: "TC-N14-TIM-NGUOI", screen: "N14.S06", state: "chat-1 đang theo dõi chat-0", action: "gõ «Chat Test 01»", cauHinh: "C1", expected: "người «Chat Test 01» hiện với trạng thái «Đang theo dõi»", status: kq2.nguoi.some((x) => /Chat Test 01/.test(x)) && /Đang theo dõi/.test(await chuTrang(t.page)) ? "PASS" : "FAIL", evidence: ["EV-N14-TIM-NGUOI-C1"], ghiChu: `người: ${kq2.nguoi.join(" | ") || "không"}; chủ đề: ${kq2.chuDe.join(" | ") || "không"}` });
          await go(t, cdp, "Chủ đề hoặc người chia sẻ", "zzzq");
          await t.page.waitForTimeout(1800);
          const kq3 = await doTim();
          const chu3 = await chuTrang(t.page);
          await anh(t.page, "EV-N14-TIM-RONG-C1", t);
          ghi({ tc: "TC-N14-TIM-RONG", screen: "N14.S06", state: "chat-1", action: "gõ «zzzq» (không khớp gì)", cauHinh: "C1", expected: "nói rõ không tìm thấy (và gợi ý thử từ khác), không để hai tiêu đề trống", status: /Không tìm thấy|không thấy|Chưa có/.test(chu3) ? "PASS" : "FAIL", evidence: ["EV-N14-TIM-RONG-C1"], ghiChu: `chữ trang «${chu3.slice(0, 160)}»; chủ đề ${kq3.chuDe.length}, người ${kq3.nguoi.length}` });
          // A topic from the search page.
          await go(t, cdp, "Chủ đề hoặc người chia sẻ", "cà");
          await t.page.waitForTimeout(1800);
          const td = await nut(t.page, "cà phê", { batDau: true });
          if (td) await cham(cdp, td);
          await t.page.waitForTimeout(2500);
          await choOn(t.page, { mang: t.mang });
          const u2 = await duongDan(t.page);
          const mCd = await doBangTin(t.page);
          const coQuay = !!(await nut(t.page, "Quay lại"));
          await anh(t.page, "EV-N14-CHU-DE-C1", t);
          ghi({ tc: "TC-N14-CHU-DE", screen: "N14.S07", state: "chat-1, từ trang tìm", action: "chạm chủ đề «cà phê»", cauHinh: "C1", expected: "trang chủ đề có tiêu đề «cà phê», chỉ bài của chủ đề, và lối quay lại trên màn", status: /\/community\/topic/.test(u2) && mCd?.tieuDe === "cà phê" && mCd.the.length >= 1 && coQuay ? "PASS" : "FAIL", evidence: ["EV-N14-CHU-DE-C1"], ghiChu: `tới ${decodeURIComponent(u2)}; tiêu đề «${mCd?.tieuDe}»; ${mCd?.the.length ?? 0} thẻ; tab ${mCd?.tabs.map((x) => x.ten).join(" / ")}; nút «Quay lại» trên màn: ${coQuay ? "có" : "không"}; «Theo dõi» chủ đề trên trang: ${(await nut(t.page, "Theo dõi")) ? "có" : "không"}` });
        }
        await t.context.close();
      }
    }
    if (chayPhan("phu", "thong-bao")) {
      const t = await mo("C1", "chat-1", "/community");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(2500);
      await choOn(t.page, { mang: t.mang });
      await bam(t, cdp, "Cài đặt bảng tin", { cho: 1200 });
      await bam(t, cdp, "Thông báo", { trongDialog: true, cho: 2500 });
      await choOn(t.page, { mang: t.mang });
      const u = await duongDan(t.page);
      const ds = await t.page.evaluate(() => [...document.querySelectorAll('[role="button"]')].filter((b) => b.getClientRects().length && /nhắc/.test(b.innerText ?? "")).map((b) => (b.innerText ?? "").replace(/\s+/g, " ").trim()));
      const api = await goi("GET", "/v2/community/notifications", undefined, await phienCua("chat-1"));
      await anh(t.page, "EV-N14-THONG-BAO-C1", t);
      let den = null;
      if (ds.length) {
        await bam(t, cdp, ds[0]);
        await t.page.waitForTimeout(2200);
        den = await doChiTiet(t.page);
      }
      ghi({ tc: "TC-N14-THONG-BAO", screen: "N14.S08", state: "chat-1 được chat-0 tag trong trả lời đã duyệt", action: "«Cài đặt bảng tin» → «Thông báo» → chạm thông báo", cauHinh: "C1", expected: "thông báo nói ai nhắc mình và ở bài nào; chạm mở đúng bài", status: ds.length && /Chat Test 01/.test(ds[0]) && den?.id ? "PASS" : "FAIL", evidence: ["EV-N14-THONG-BAO-C1"], ghiChu: `tới ${u}; ${ds.length} dòng: ${ds.map((x) => `«${x}»`).join(", ") || "không"}; API: ${api.json?.notifications?.map((n) => n.kind).join(", ") ?? api.status}; chạm dòng đầu → ${den?.id ? `chi tiết bài «${(den.than ?? "").slice(0, 40)}…»` : "không tới bài"}; lối vào: chỉ trong sheet «Bảng tin của bạn», không có dấu hiệu ở tab` });
      await t.context.close();
    }
    if (chayPhan("phu", "cua-toi")) {
      const t = await mo("C1", "chat-0", "/community");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(2500);
      await choOn(t.page, { mang: t.mang });
      await bam(t, cdp, "Cài đặt bảng tin", { cho: 1200 });
      await bam(t, cdp, "Bài của tôi · Trạng thái duyệt", { trongDialog: true, cho: 2500 });
      await choOn(t.page, { mang: t.mang });
      const m = await doBangTin(t.page);
      const the = await doThe(t.page);
      const chu = await chuTrang(t.page);
      await anh(t.page, "EV-N14-CUA-TOI-C1", t);
      ghi({ tc: "TC-N14-CUA-TOI", screen: "N14.S01", state: "chat-0: B1 (bản sửa chờ duyệt), B2 (đã duyệt), B3 (bạn bè)", action: "«Cài đặt bảng tin» → «Bài của tôi · Trạng thái duyệt»", cauHinh: "C1", expected: "«Những điều bạn đã kể» liệt kê bài của tôi với trạng thái từng bài; biết đang ở danh sách nào (tab/tiêu đề)", status: /Những điều bạn đã kể/.test(chu) && the.length >= 3 && the.some((x) => /^Đang chờ duyệt/.test(x.dai ?? "")) ? "PASS" : "FAIL", evidence: ["EV-N14-CUA-TOI-C1"], ghiChu: `${the.length} thẻ: ${the.map((x) => `«${x.dau.slice(18, 50)}…» ${x.meta ?? ""}${x.dai ? ` [${x.dai}]` : ""}`).join(" | ")}; tab: ${m?.tabs.map((x) => `«${x.ten}» màu ${x.mau}`).join(", ")}` });
      await bam(t, cdp, "Cài đặt bảng tin", { cho: 1200 });
      await bam(t, cdp, "Điều mình muốn giữ", { trongDialog: true, cho: 2500 });
      await choOn(t.page, { mang: t.mang });
      const chuGiu = await chuTrang(t.page);
      await anh(t.page, "EV-N14-GIU-C1", t);
      ghi({ tc: "TC-N14-GIU", screen: "N14.S08", state: "chat-0 chưa có ghi chép riêng (Nếp không có AI)", action: "«Cài đặt bảng tin» → «Điều mình muốn giữ»", cauHinh: "C1", expected: "trang rỗng nói chưa có ghi chép và cách tạo (qua «@Nếp» trên một bài)", status: /Chưa có|chưa có/.test(chuGiu) && /Nếp/.test(chuGiu) ? "PASS" : "FAIL", evidence: ["EV-N14-GIU-C1"], ghiChu: `chữ trang «${chuGiu.slice(0, 200)}»` });
      await t.context.close();
    }
  }

  // ------------------------------------------------------------------ loi
  // Faults at the browser: the feed, the comments, a like, offline, a missing post.
  if (chay("loi")) {
    const b1 = await timBai("B1");
    const t = await mo("C1", "chat-1", "/community");
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(2500);
    await choOn(t.page, { mang: t.mang });
    if (chayPhan("loi", "bang")) {
      const doc = [];
      const nghe = (r) => {
        if (/\/v2\/community\/feed\?/.test(r.url())) doc.push(r.status());
      };
      t.page.on("response", nghe);
      await loiMayChu(t.page, /\/v2\/community\/feed\?/);
      await bam(t, cdp, "Thịnh hành", { cho: 2500 });
      const m1 = await doBangTin(t.page);
      await anh(t.page, "EV-N14-LOI-BANG-C1", t);
      await goHet(t.page);
      const thu = await bam(t, cdp, "Thử lại", { cho: 2500 });
      const m2 = await doBangTin(t.page);
      t.page.off("response", nghe);
      ghi({ tc: "TC-N14-LOI-BANG", screen: "N14.S01", state: "chat-1; GET feed trả 503 (chặn ở trình duyệt)", action: "tab «Thịnh hành»; bỏ lỗi; «Thử lại»", cauHinh: "C1", expected: "một câu tiếng Việt và «Thử lại»; thử lại đọc được bài", status: m1?.alert.length && !/\b(503|error|undefined)\b/i.test(m1.alert.join(" ")) && thu && m2?.the.length >= 1 && !m2.alert.length ? "PASS" : "FAIL", evidence: ["EV-N14-LOI-BANG-C1"], ghiChu: `lúc lỗi: ${m1?.alert.map((a) => `«${a.chu}»`).join(" ") || "không câu"}; lần đọc ${doc.join(", ")}; sau «Thử lại»: ${m2?.the.length ?? 0} thẻ, câu lỗi ${m2?.alert.length ? "còn" : "hết"}` });
      await bam(t, cdp, "Dành cho bạn", { cho: 2500 });
    }
    if (chayPhan("loi", "thich")) {
      // A like that fails while the reader is at the second card.
      await denThe(t, "B2", "center");
      const b2 = await theCua(t.page, "B2");
      await loiMayChu(t.page, /\/like$/);
      const th = b2?.hanhDong.find((h) => /^(Thích bài|Bỏ thích bài)/.test(h.ten));
      const p = th ? await nut(t.page, th.ten, { trong: `[data-testid="community-post-${b2.id}"]` }) : null;
      if (p) await cham(cdp, p);
      await t.page.waitForTimeout(1800);
      const loi = await t.page.evaluate(() =>
        [...document.querySelectorAll('[data-testid="community-screen"] [role="alert"]')].filter((a) => a.getClientRects().length).map((a) => {
          const r = a.getBoundingClientRect();
          return { chu: (a.innerText ?? "").trim(), top: Math.round(r.top), day: Math.round(r.bottom), h: innerHeight };
        }),
      );
      const sau = (await theCua(t.page, "B2"))?.hanhDong.find((h) => /^(Thích bài|Bỏ thích bài)/.test(h.ten));
      await anh(t.page, "EV-N14-LOI-THICH-C1", t);
      await goHet(t.page);
      ghi({ tc: "TC-N14-LOI-THICH", screen: "N14.S01", state: "chat-1 ở thẻ B2 (thẻ thứ hai); PUT like trả 503", action: "chạm «Thích»", cauHinh: "C1", expected: "lượt thích trả về như cũ và câu lỗi hiện trong tầm nhìn (gần thẻ vừa chạm)", status: loi.some((l) => l.top >= 0 && l.day <= l.h) && sau?.ten === th?.ten ? "PASS" : "FAIL", evidence: ["EV-N14-LOI-THICH-C1"], ghiChu: `trước «${th?.ten}», sau «${sau?.ten}»; câu lỗi: ${loi.map((l) => `«${l.chu.slice(0, 80)}» y ${l.top}–${l.day} trong cửa sổ ${l.h}`).join(" | ") || "không có"}` });
    }
    if (chayPhan("loi", "binh-luan") && b1) {
      await loiMayChu(t.page, /\/comments\?/);
      await diToi(t, `/community/posts/${b1.id}`);
      const d = await doChiTiet(t.page);
      const coThu = await t.page.evaluate(() => {
        const man = [...document.querySelectorAll('[data-testid="community-post-detail"]')].filter((x) => x.getClientRects().length).pop();
        return !!man && [...man.querySelectorAll('[role="button"]')].some((b) => /^Thử lại/.test((b.innerText ?? "").trim()));
      });
      await anh(t.page, "EV-N14-LOI-BL-C1", t);
      await goHet(t.page);
      ghi({ tc: "TC-N14-LOI-BL", screen: "N14.S05", state: "chat-1, chi tiết B1; GET comments trả 503", action: "mở chi tiết bài", cauHinh: "C1", expected: "khu bình luận nói không đọc được, có lối thử lại; bài vẫn đọc được", status: d?.id && d.loi.length && coThu ? "PASS" : "FAIL", evidence: ["EV-N14-LOI-BL-C1"], ghiChu: `bài ${d?.id ? "đọc được" : "không"}; câu lỗi ${d?.loi.map((x) => `«${x}»`).join(" ") || "không"}; «Thử lại» trong chi tiết: ${coThu ? "có" : "không"}` });
    }
    if (chayPhan("loi", "404")) {
      await diToi(t, `/community/posts/${randomUUID()}`);
      const d = await doChiTiet(t.page);
      await anh(t.page, "EV-N14-LOI-404-C1", t);
      ghi({ tc: "TC-N14-LOI-404", screen: "N14.S04", state: "chat-1", action: "mở /community/posts/<id không tồn tại>", cauHinh: "C1", expected: "câu «Bài đã được cất riêng hoặc không còn ở đây.» và «Quay lại»", status: /cất riêng|không còn/.test(d?.loi.join(" ") ?? "") && d.quayLai ? "PASS" : "FAIL", evidence: ["EV-N14-LOI-404-C1"], ghiChu: `câu lỗi ${d?.loi.map((x) => `«${x}»`).join(" ") || "không"}; «Quay lại» ${d?.quayLai ? "có" : "không"}` });
    }
    if (chayPhan("loi", "offline")) {
      await diToi(t, "/community");
      await t.context.setOffline(true);
      await bam(t, cdp, "Đang theo dõi", { cho: 2500 });
      const m1 = await doBangTin(t.page);
      await anh(t.page, "EV-N14-LOI-OFFLINE-C1", t);
      await t.context.setOffline(false);
      const thu = await bam(t, cdp, "Thử lại", { cho: 2500 });
      const m2 = await doBangTin(t.page);
      ghi({ tc: "TC-N14-LOI-OFFLINE", screen: "N14.S01", state: "chat-1, mất mạng", action: "tab «Đang theo dõi»; có mạng lại; «Thử lại»", cauHinh: "C1", expected: "câu nói mất kết nối và «Thử lại»; có mạng lại thì đọc được", status: m1?.alert.length && thu && !m2?.alert.length ? "PASS" : "FAIL", evidence: ["EV-N14-LOI-OFFLINE-C1"], ghiChu: `lúc mất mạng: ${m1?.alert.map((a) => `«${a.chu}»`).join(" ") || "không câu"}; sau «Thử lại»: ${m2?.the.length ?? 0} thẻ, câu lỗi ${m2?.alert.length ? "còn" : "hết"}` });
    }
    await t.context.close();
  }

  // ------------------------------------------------------------------ tuong
  // Where the community meets the personal wall v2 (#658): chat-0's posts seen from chat-0's profile.
  if (chay("tuong")) {
    const id0 = idNguoi("chat-0");
    for (const [ten, tc, kyVong] of [
      ["chat-1", "TC-N14-TUONG", "tường của chat-0 (người xem là bạn) có bài «Bạn bè» B3 và bài công khai đã duyệt; lượt thích từ Cộng đồng (1 ở B1) thấy ở tường"],
      ["chat-16", "TC-N14-TUONG-NGUOI-LA", "người lạ thấy bài công khai đã duyệt, không thấy bài «Bạn bè» B3"],
    ]) {
      const t = await mo("C1", ten, `/people/${id0}`);
      await t.page.waitForTimeout(3000);
      await choOn(t.page, { mang: t.mang });
      const chu = await chuTrang(t.page);
      // The profile may list posts further down: scroll to the end once.
      await t.page.evaluate(() => {
        const s = [...document.querySelectorAll("div")].filter((d) => ["auto", "scroll"].includes(getComputedStyle(d).overflowY) && d.scrollHeight > d.clientHeight + 4).sort((a, b) => b.scrollHeight - a.scrollHeight)[0];
        if (s) s.scrollTop = s.scrollHeight;
      });
      await t.page.waitForTimeout(1500);
      const chu2 = await chuTrang(t.page);
      const tat = `${chu} ${chu2}`;
      await anh(t.page, `EV-N14-${tc.replace("TC-N14-", "")}-C1`, t);
      const coB3 = tat.includes(dauBai("B3").slice(0, 30));
      const coB1 = tat.includes(dauBai("B1").slice(0, 30));
      const coB2 = tat.includes(dauBai("B2").slice(0, 30));
      const dat = ten === "chat-1" ? coB3 && (coB1 || coB2) : !coB3 && (coB1 || coB2);
      ghi({ tc, screen: "N14.S10", state: `${ten} mở hồ sơ chat-0 (/people/[id])`, action: "đọc tường cá nhân", cauHinh: "C1", expected: kyVong, status: dat ? "PASS" : "FAIL", evidence: [`EV-N14-${tc.replace("TC-N14-", "")}-C1`], ghiChu: `B1 ${coB1 ? "có" : "không"}, B2 ${coB2 ? "có" : "không"}, B3 ${coB3 ? "có" : "không"}; chữ đầu «${chu.slice(0, 160)}»` });
      await t.context.close();
    }
  }
} finally {
  await mt.dong();
}
