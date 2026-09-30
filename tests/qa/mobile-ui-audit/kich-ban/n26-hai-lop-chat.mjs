/* Audit of a feature new on main 461eabf: the two classes of a two-person chat
 * (PR #660, task #26). A friends' pair («đám bạn», every ordinary two-person
 * chat) gets a group's tools worded for two; a couple («cặp đôi», both said yes
 * to «Một đôi», the server's `cap_doi`) adds the pinned «Tờ giấy» row, a fifth
 * tray tool and four stickers. The taste switch of the notebook now names the
 * chat, and a switch turned on before that wording offers «Bật lại cho chat»
 * (ADR-0048).
 *
 *   source audit-env-main.sh && node kich-ban/n26-hai-lop-chat.mjs [--chi api,ban,…]
 *
 * Pairs on stack 2 (read over the API before any write):
 *   chat-0/chat-1   friends, no notebook
 *   chat-4/chat-5   friends, notebook open (lap_so), not a couple
 *   chat-12/chat-13 friends, chat-12's lap_so proposal waits for chat-13
 *   chat-10/chat-11 friends with a notebook, made a couple here through the UI
 *   chat-8/chat-9   a couple since the retest (lap_so, bat_doi)
 *   chat-6/chat-7   friends with a notebook: the paper the server still drafts (Q2)
 * Rows are TC-N26-*; the placeholder TC-N-26-HAI-LOP-CHAT is withdrawn by the verdict script.
 */
import { cauHinh } from "../thu-vien/cau-hinh.mjs";
import { chup, ghepAnh } from "../thu-vien/chup.mjs";
import { cdpCua, cham } from "../thu-vien/cu-chi.mjs";
import { choOn, duongDan } from "../thu-vien/dieu-huong.mjs";
import { inertConLai } from "../thu-vien/lop-phu.mjs";
import { ghepKhung, quayKhung } from "../thu-vien/chuyen-dong.mjs";
import { chayAxe } from "../thu-vien/axe.mjs";
import { goHet, loiMayChu, tre } from "../thu-vien/mang.mjs";
import { khoiDong, trangMoi } from "../thu-vien/moi-truong.mjs";
import { goiApi, layPhien, personaTheoTen } from "../thu-vien/phien.mjs";
import { soGhi } from "../thu-vien/ghi.mjs";
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
  so.ghi({ feature: "N26", nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, evidence: [], ...rec });
  console.log(`${rec.status.padEnd(10)} ${rec.tc} ${rec.cauHinh ?? ""} ${rec.ghiChu ?? ""}`);
};
const P = (ten) => personaTheoTen(ten, mt.chatSessions);
const phienDir = join(mt.out, "phien");
const phienCua = (ten) => layPhien(mt.api, P(ten), phienDir);
const mo = (cfg, persona, path) => trangMoi(mt, cauHinh(cfg), { persona: persona ? P(persona) : null, path });
const anh = (page, id, t, extra = {}) => chup(page, { out: mt.out, id, suKien: t.suKien, ...extra });
const api = (method, path, body, phien) => goiApi(mt.api, method, path, body, phien.token);
/** A write with its own Idempotency-Key, answering the status and the body; never throws. */
const ghiApi = async (method, path, body, phien) => {
  const r = await fetch(`${mt.api}${path}`, { method, headers: { authorization: `Bearer ${phien.token}`, "content-type": "application/json", "idempotency-key": randomUUID() }, body: body === undefined ? undefined : JSON.stringify(body) });
  const text = await r.text();
  let json = null;
  try {
    json = JSON.parse(text);
  } catch {
    /* not JSON */
  }
  return { status: r.status, json };
};
const docApi = async (path, phien) => {
  try {
    return await api("GET", path, undefined, phien);
  } catch (e) {
    return { loi: String(e.message).slice(0, 160) };
  }
};
/** chat-N shows as «Chat Test NN», counted from 01. */
const tenHien = (ten) => `Chat Test ${String(Number(ten.split("-")[1]) + 1).padStart(2, "0")}`;
/** The pair of two chat-test people as A reads it from its contexts. */
const capGiua = async (tenA, tenB) => {
  const pa = await phienCua(tenA);
  const pb = await phienCua(tenB);
  const cap = ((await docApi("/people/me/contexts", pa)).contexts ?? []).find((c) => c.kind === "pair" && c.counterpart?.display_name === tenHien(tenB));
  if (!cap) throw new Error(`không thấy chat đôi ${tenA}/${tenB}`);
  return { cap, pa, pb };
};
const soCua = (id, phien) => docApi(`/contexts/${id}/notebook`, phien);
const capCua = (id, phien) => docApi(`/contexts/${id}/chat-capabilities`, phien);
const chuTrang = (page) => page.evaluate(() => (document.body.innerText ?? "").replace(/\s+/g, " "));
const G = JSON.parse(readFileSync(mt.chatSessions, "utf8")).groupId;

/**
 * A control by its accessible name or visible text, scrolled into view, and only
 * if a tap at its centre lands on it: a stack screen left mounted underneath
 * carries buttons of the same name (lesson of checkpoint retest 2).
 */
const nut = (page, chu, { batDau = false, trongDialog = false } = {}) =>
  page.evaluate(
    ({ chu, batDau, trongDialog }) => {
      const ten = (x) => (x.getAttribute("aria-label") || (x.innerText ?? "")).replace(/[-]/g, "").replace(/\s+/g, " ").trim();
      const goc = trongDialog ? [...document.querySelectorAll('[role="dialog"]')].filter((d) => d.getClientRects().length).pop() : document;
      if (!goc) return null;
      for (const e of goc.querySelectorAll('[role="button"],button,[role="radio"],[role="tab"],[role="link"],a[href]')) {
        if (!(batDau ? ten(e).startsWith(chu) : ten(e) === chu) || !e.getClientRects().length) continue;
        e.scrollIntoView({ block: "center", behavior: "instant" });
        const r = e.getBoundingClientRect();
        const x = r.left + r.width / 2;
        const y = r.top + r.height / 2;
        const tren = document.elementFromPoint(x, y);
        if (tren && (tren === e || e.contains(tren))) return { x, y, w: Math.round(r.width), h: Math.round(r.height), ten: ten(e) };
      }
      return null;
    },
    { chu, batDau, trongDialog },
  );
const bam = async (t, cdp, chu, opts) => {
  const p = await nut(t.page, chu, opts);
  if (!p) return null;
  await cham(cdp, p);
  await t.page.waitForTimeout(opts?.cho ?? 900);
  return p;
};
/** The centre of the visible composer («Ô soạn tin»). */
const oSoan = (page) =>
  page.evaluate(() => {
    const o = [...document.querySelectorAll('[aria-label="Ô soạn tin"]')].find((x) => x.getClientRects().length);
    if (!o) return null;
    const r = o.getBoundingClientRect();
    return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
  });
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
    return { ten: d.getAttribute("aria-label"), chu: (d.innerText ?? "").replace(/\s+/g, " ").trim(), top: Math.round(r.top), cao: Math.round(r.height), rong: Math.round(r.width) };
  });

/**
 * The chat's tray of tools as drawn: the row (`khay-hang-cong-cu`), every tool
 * with its box, its paper square and its label, whether the label is cut
 * (clamped past two lines, or a word wider than the tool), the title, the
 * caption under it, and the scroll box's height against its content.
 */
const doKhay = (page) =>
  page.evaluate(() => {
    const hang = document.querySelector('[data-testid="khay-hang-cong-cu"]');
    if (!hang || !hang.getClientRects().length) return null;
    const khung = hang.getBoundingClientRect();
    const cong = [...hang.querySelectorAll(':scope > [role="button"]')].map((b) => {
      const r = b.getBoundingClientRect();
      const icon = b.firstElementChild?.getBoundingClientRect();
      const nhan = [...b.querySelectorAll('div[dir="auto"]')].pop();
      const n = nhan?.getBoundingClientRect();
      return {
        ten: b.getAttribute("aria-label"),
        x: Math.round(r.left),
        y: Math.round(r.top),
        w: +r.width.toFixed(1),
        h: Math.round(r.height),
        icon: icon ? { w: Math.round(icon.width), h: Math.round(icon.height) } : null,
        nhan: n ? { w: +n.width.toFixed(1), h: Math.round(n.height), dong: Math.round(n.height / 16), cat: nhan.scrollWidth > nhan.clientWidth + 1 || nhan.scrollHeight > nhan.clientHeight + 1 } : null,
      };
    });
    let cuon = hang.parentElement;
    while (cuon && getComputedStyle(cuon).overflowY !== "auto" && getComputedStyle(cuon).overflowY !== "scroll") cuon = cuon.parentElement;
    const khay = hang.closest('[data-testid]') === hang ? cuon?.parentElement : cuon?.parentElement;
    const chu = (khay?.innerText ?? "").replace(/\s+/g, " ").trim();
    const hangY = [...new Set(cong.map((c) => c.y))];
    return {
      rong: Math.round(khung.width),
      cao: Math.round(khung.height),
      soCong: cong.length,
      soHang: hangY.length,
      cong,
      cuon: cuon ? { cao: Math.round(cuon.clientHeight), noiDung: Math.round(cuon.scrollHeight), top: Math.round(cuon.getBoundingClientRect().top) } : null,
      chu: chu.slice(0, 400),
      cuaSo: { w: innerWidth, h: innerHeight },
    };
  });
/** The sticker sheet: its headings, every sticker's name and whether its label is cut. */
const doSticker = (page) =>
  page.evaluate(() => {
    const d = document.querySelector('[data-testid="khay-sticker"]');
    if (!d || !d.getClientRects().length) return null;
    const tieuDe = [...d.querySelectorAll('[role="heading"]')].map((h) => (h.innerText ?? "").trim());
    const nut = [...d.querySelectorAll('[role="button"]')].filter((b) => b.getClientRects().length && !/^Đóng/.test(b.getAttribute("aria-label") ?? ""));
    const cat = nut.filter((b) => [...b.querySelectorAll('div[dir="auto"]')].some((x) => x.scrollWidth > x.clientWidth + 1 || x.scrollHeight > x.clientHeight + 1)).map((b) => b.getAttribute("aria-label") ?? b.innerText);
    return { tieuDe, so: nut.length, ten: nut.map((b) => (b.getAttribute("aria-label") ?? (b.innerText ?? "").trim()).replace(/\s+/g, " ")), cat, chu: (d.innerText ?? "").replace(/\s+/g, " ").trim().slice(0, 300) };
  });
/** The pinned line under a pair's header: the invitation or the paper row, as drawn. */
const doHangGhim = (page) =>
  page.evaluate(() => {
    const hien = (e) => e && e.getClientRects().length > 0;
    const moi = [...document.querySelectorAll('[data-testid="hang-loi-de-nghi"]')].find(hien);
    const giay = [...document.querySelectorAll('[data-testid="hang-to-giay"]')].find(hien);
    const mot = (e) => {
      if (!e) return null;
      const r = e.getBoundingClientRect();
      // Icon glyphs are text in a private-use range: not words anybody reads.
      const chu = [...e.querySelectorAll('div[dir="auto"]')].map((x) => ({ chu: (x.innerText ?? "").replace(/[\uE000-\uF8FF]/g, "").trim(), cao: Math.round(x.getBoundingClientRect().height), cat: x.scrollWidth > x.clientWidth + 1 || x.scrollHeight > x.clientHeight + 1 })).filter((x) => x.chu);
      return { top: Math.round(r.top), x: Math.round(r.left), w: Math.round(r.width), h: Math.round(r.height), role: e.getAttribute("role"), ten: e.getAttribute("aria-label"), chu };
    };
    const baoMat = [...document.querySelectorAll('div[dir="auto"]')].find((x) => (x.innerText ?? "").trim() === "Chưa mã hoá đầu cuối" && hien(x));
    return { moi: mot(moi), giay: mot(giay), baoMatTop: baoMat ? Math.round(baoMat.getBoundingClientRect().top) : null };
  });

try {
  // ------------------------------------------------------------------ api
  // The contract the two classes hang on (ADR-0046 §8.4, ADR-0048 §3.2), read as each person sees it.
  if (chay("api")) {
    const bang = [];
    const p0 = await phienCua("chat-0");
    const nhom = await capCua(G, p0);
    bang.push({ phong: "nhóm chat-test", cap_doi: nhom.cap_doi, gu_chat: nhom.gu_chat, ai: nhom.ai?.plan?.reason ?? (nhom.ai?.plan?.available ? "ok" : "?") });
    for (const [a, b, ten] of [["chat-0", "chat-1", "đám bạn, chưa sổ"], ["chat-4", "chat-5", "đám bạn, sổ đã mở"], ["chat-13", "chat-12", "đám bạn, lời đề nghị lập sổ chờ"], ["chat-8", "chat-9", "cặp đôi"], ["chat-9", "chat-8", "cặp đôi, phía kia"]]) {
      const { cap, pa } = await capGiua(a, b);
      const c = await capCua(cap.id, pa);
      const s = await soCua(cap.id, pa);
      bang.push({ phong: `${a}→${b} (${ten})`, cap_doi: c.cap_doi, gu_chat: c.gu_chat, ai: c.ai?.plan?.reason ?? (c.ai?.plan?.available ? "ok" : "?"), so: (s.my_consents ?? []).filter((x) => x.granted).map((x) => x.purpose).join("+") || "-", cho: (s.pending_proposals ?? []).map((d) => `${d.purpose}:${d.proposed_by_id === pa.person_id ? "của tôi" : "của người kia"}`).join(",") || "-" });
    }
    const dung = bang.every((r) => (/cặp đôi/.test(r.phong) ? r.cap_doi === true && r.gu_chat && typeof r.gu_chat.cua_toi === "string" && typeof r.gu_chat.nguoi_kia === "boolean" : r.cap_doi === false && r.gu_chat === null));
    for (const r of bang) console.log(JSON.stringify(r));
    ghi({ tc: "TC-N26-API-CAP", screen: "N26.S00", state: "5 phòng: nhóm, ba cặp đám bạn (chưa sổ, sổ mở, lời đề nghị chờ), một cặp đôi đọc từ hai phía", action: "GET /contexts/{id}/chat-capabilities và /notebook (chỉ đọc)", cauHinh: "-", expected: "cap_doi true chỉ ở cặp đôi; gu_chat null ngoài cặp đôi, trong cặp đôi là {cua_toi, nguoi_kia} (ADR-0046 §8.4, ADR-0048 §3.2)", status: dung ? "PASS" : "FAIL", ghiChu: bang.map((r) => `${r.phong}: cap_doi=${r.cap_doi}, gu_chat=${JSON.stringify(r.gu_chat)}, AI plan=${r.ai}${r.so ? `, đồng ý=${r.so}, chờ=${r.cho}` : ""}`).join("; ") });
  }

  // ------------------------------------------------------------------ ban
  // A friends' pair with no notebook (chat-0 → chat-1): a group's tools, worded for two.
  if (chay("ban")) {
    const { cap } = await capGiua("chat-0", "chat-1");
    const duong = `/groups/${cap.id}/chat`;
    const anhBan = [];
    for (const cfg of ["C1", "C2", "C3", "C4", "C5", "C6", "C7"]) {
      if (!chayPhan("ban", cfg)) continue;
      const t = await mo(cfg, "chat-0", duong);
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(1800);
      await choOn(t.page, { mang: t.mang });
      const ghim = await doHangGhim(t.page);
      const dau = await chuTrang(t.page);
      await anh(t.page, `EV-N26-BAN-${cfg}`, t);
      const moKhay = await bam(t, cdp, "Thêm vào cuộc trò chuyện");
      const khay = moKhay ? await doKhay(t.page) : null;
      await anh(t.page, `EV-N26-BAN-KHAY-${cfg}`, t);
      anhBan.push({ id: `EV-N26-BAN-KHAY-${cfg}`, nhan: `${cfg}` });
      const tenCong = khay?.cong.map((c) => c.ten) ?? [];
      const bonCong = JSON.stringify(tenCong) === JSON.stringify(["Ảnh", "Sticker", "Bình chọn", "Tờ hẹn"]);
      const cat = khay?.cong.filter((c) => c.nhan?.cat).map((c) => c.ten) ?? [];
      const chuNhom = /\b(nhóm|cả hội|bạn đồng hành|người giữ sổ)\b/i.test(khay?.chu ?? "");
      const dat = bonCong && khay.soHang === 1 && cat.length === 0 && !chuNhom && ghim.moi === null && ghim.giay === null;
      ghi({ tc: "TC-N26-BAN-KHAY", screen: "N26.S01", layer: "L16", state: "cặp đám bạn chat-0/chat-1, chưa sổ, chưa có tin", action: "mở chat → «Thêm vào cuộc trò chuyện»", cauHinh: cfg, expected: "khay bốn công cụ Ảnh, Sticker, Bình chọn, Tờ hẹn trên một hàng, không nhãn nào bị cắt, không «Tờ giấy»; chữ khay viết cho hai người; không hàng ghim nào dưới tiêu đề", status: dat ? "PASS" : "FAIL", evidence: [`EV-N26-BAN-${cfg}`, `EV-N26-BAN-KHAY-${cfg}`], ghiChu: `${khay ? `${khay.soCong} công cụ (${tenCong.join(", ")}), ${khay.soHang} hàng; ô ${khay.cong.map((c) => c.w).join("/")}px, ô vuông ${khay.cong[0]?.icon?.w}px; nhãn bị cắt: ${cat.join(", ") || "không"}; khay cao ${khay.cuon?.cao}/${khay.cuon?.noiDung}px; chữ khay «${khay.chu.slice(0, 170)}»` : "không mở được khay"}; hàng ghim: ${ghim.moi ? "hàng mời" : ghim.giay ? "Tờ giấy" : "không"}; đầu màn «${dau.slice(0, 110)}»` });
      if (cfg === "C1") {
        // The empty room: its words and where «Rủ hội một buổi» leads.
        await bam(t, cdp, "Đóng khay công cụ");
        const trong = await t.page.evaluate(() => {
          const h = [...document.querySelectorAll('div[dir="auto"]')].filter((x) => x.getClientRects().length).map((x) => (x.innerText ?? "").trim());
          return { tieuDe: h.find((x) => /^Một lời mở đầu|^Có hội rồi/.test(x)) ?? null, phu: h.find((x) => /^Một tin nhắn nhỏ cho/.test(x)) ?? null, oSoan: document.querySelector('[aria-label="Ô soạn tin"]')?.getAttribute("placeholder") ?? null };
        });
        const ru = await bam(t, cdp, "Rủ hội một buổi");
        const khayRu = await t.page.evaluate(() => (document.querySelector('[data-testid="chat-khay-hoi-ai"]')?.closest('[style]')?.parentElement?.innerText ?? "").replace(/\s+/g, " ").trim());
        const toHen = await t.page.evaluate(() => {
          const h = [...document.querySelectorAll('[role="heading"]')].map((x) => (x.innerText ?? "").trim());
          const loi = document.querySelector('[data-testid="chat-khay-hoi-ai"]');
          const nutTen = [...document.querySelectorAll('[role="button"]')].filter((b) => b.getClientRects().length).map((b) => (b.getAttribute("aria-label") || b.innerText || "").trim());
          return { tieuDe: h.includes("Phác một tờ hẹn"), loi: (loi?.innerText ?? "").replace(/\s+/g, " ").trim(), hoiAi: nutTen.includes("Hỏi Rủ Đi AI"), tuTao: nutTen.includes("Tự tạo kèo"), chuaSan: /AI chưa sẵn sàng/.test(document.body.innerText ?? "") };
        });
        await anh(t.page, "EV-N26-BAN-TO-HEN-C1", t);
        ghi({ tc: "TC-N26-BAN-TRONG", screen: "N26.S01", state: "cặp đám bạn chat-0/chat-1, chưa có tin", action: "đọc trạng thái rỗng → «Rủ hội một buổi»", cauHinh: cfg, expected: "trạng thái rỗng viết cho hai người; nút mở bảng «Phác một tờ hẹn» có «Tự tạo kèo», và khi AI chưa sẵn sàng thì nói thẳng", status: trong.tieuDe && ru && toHen.tieuDe && toHen.tuTao && (toHen.hoiAi || toHen.chuaSan) ? "PASS" : "FAIL", evidence: ["EV-N26-BAN-C1", "EV-N26-BAN-TO-HEN-C1"], ghiChu: `tiêu đề «${trong.tieuDe}», dòng phụ «${trong.phu}», ô soạn «${trong.oSoan}»; nút «Rủ hội một buổi»: ${ru ? "có" : "không"}; bảng: tiêu đề «Phác một tờ hẹn» ${toHen.tieuDe ? "có" : "không"}, lời «${toHen.loi.slice(0, 160)}», «Hỏi Rủ Đi AI» ${toHen.hoiAi ? "có" : "không"}, «AI chưa sẵn sàng» ${toHen.chuaSan ? "có" : "không"}, «Tự tạo kèo» ${toHen.tuTao ? "có" : "không"}${khayRu ? "" : ""}` });
        await bam(t, cdp, "Đóng khay công cụ");
        // The sticker sheet: a friends' pair keeps a group's eight.
        await bam(t, cdp, "Thêm vào cuộc trò chuyện");
        await bam(t, cdp, "Sticker", { cho: 1200 });
        const st = await doSticker(t.page);
        await anh(t.page, "EV-N26-BAN-STICKER-C1", t);
        ghi({ tc: "TC-N26-BAN-STICKER", screen: "N26.S01", layer: "L19", state: "cặp đám bạn chat-0/chat-1", action: "khay → «Sticker»", cauHinh: cfg, expected: "tám sticker của nhóm, không có mục «Cho hai người»", status: st && st.so === 8 && !st.tieuDe.includes("Cho hai người") && st.cat.length === 0 ? "PASS" : "FAIL", evidence: ["EV-N26-BAN-STICKER-C1"], ghiChu: st ? `${st.so} sticker (${st.ten.join(", ")}); tiêu đề: ${st.tieuDe.join(" / ")}; nhãn bị cắt: ${st.cat.join(", ") || "không"}` : "không mở được khay sticker" });
        await t.page.keyboard.press("Escape");
        await t.page.waitForTimeout(700);
        // Commands and the chip: «/» lists the commands in words for two; «@Rủ Đi» shows the chip.
        // The composer is a textbox, not a button: found by its label, tapped at its centre.
        const o = await oSoan(t.page);
        if (o) await cham(cdp, o);
        await t.page.waitForTimeout(300);
        // The list keeps what starts with what was typed: «/» lists the three slash commands, «@» the mention.
        const goiY = () =>
          t.page.evaluate(() => [...document.querySelectorAll('[role="button"]')].filter((b) => b.getClientRects().length && /^(\/plan|\/vote|\/chia-bill|@Rủ Đi)/.test((b.innerText ?? "").trim())).map((b) => (b.innerText ?? "").replace(/\s+/g, " ").trim()));
        await t.page.keyboard.type("/", { delay: 60 });
        await t.page.waitForTimeout(700);
        const gach = await goiY();
        await anh(t.page, "EV-N26-BAN-LENH-C1", t);
        await t.page.keyboard.press("Backspace");
        await t.page.keyboard.type("@", { delay: 60 });
        await t.page.waitForTimeout(700);
        const acong = await goiY();
        const lenh = [...gach, ...acong];
        const chuLenh = lenh.join(" | ");
        ghi({ tc: "TC-N26-BAN-LENH", screen: "N26.S01", state: "cặp đám bạn chat-0/chat-1", action: "gõ «/», rồi thay bằng «@», vào ô soạn", cauHinh: cfg, expected: "«/» gợi ý ba lệnh như nhóm (/plan, /vote, /chia-bill), «@» gợi ý @Rủ Đi; mô tả viết cho hai bạn, không «cả hội», «nhóm»", status: gach.length === 3 && acong.length === 1 && /hai bạn/.test(chuLenh) && !/cả hội|trong nhóm/.test(chuLenh) ? "PASS" : "FAIL", evidence: ["EV-N26-BAN-LENH-C1"], ghiChu: `«/»: ${gach.length} gợi ý (${gach.join(" | ")}); «@»: ${acong.length} gợi ý (${acong.join(" | ")})` });
        await t.page.keyboard.press("Backspace");
        await t.page.keyboard.type("@Rủ Đi tối nay đi đâu", { delay: 30 });
        await t.page.waitForTimeout(900);
        const chip = await t.page.evaluate(() => {
          const c = document.querySelector('[data-testid="chat-chip-boi-canh"]');
          const r = c?.getBoundingClientRect();
          return c ? { chu: (c.innerText ?? "").replace(/\s+/g, " ").trim(), xem: !!document.querySelector('[data-testid="chat-boi-canh-mo"]'), w: Math.round(r.width), h: Math.round(r.height) } : null;
        });
        await anh(t.page, "EV-N26-BAN-CHIP-C1", t);
        ghi({ tc: "TC-N26-BAN-CHIP", screen: "N26.S01", state: "cặp đám bạn, máy chủ báo provider_unavailable cho cặp (engine nhóm brain)", action: "gõ «@Rủ Đi tối nay đi đâu»", cauHinh: cfg, expected: "chip trên nút gửi nói AI chưa sẵn sàng và tin đi như tin thường; không có «Xem», «Chỉ gửi lời nhờ»", status: chip && /chưa sẵn sàng/.test(chip.chu) && !chip.xem ? "PASS" : "FAIL", evidence: ["EV-N26-BAN-CHIP-C1"], ghiChu: chip ? `chip «${chip.chu}» ${chip.w}×${chip.h}px; «Xem»: ${chip.xem ? "có" : "không"}` : "không thấy chip" });
        // Clear the composer: nothing is sent from this pair.
        await t.page.evaluate(() => {
          const o = document.querySelector('[aria-label="Ô soạn tin"]');
          o?.focus();
        });
        await t.page.keyboard.press("Control+A");
        await t.page.keyboard.press("Backspace");
        await t.page.waitForTimeout(400);
        // The pair's settings sheet: its words, and the row that leads to the paper.
        const caiDat = await bam(t, cdp, "Cài đặt nhóm", { cho: 1200 });
        const d = await dialogTren(t.page);
        await anh(t.page, "EV-N26-BAN-CAI-DAT-C1", t);
        const nhanNut = caiDat?.ten ?? null;
        const chuNhomCd = (d?.chu ?? "").match(/[^.]*\b(nhóm|hội)\b[^.]*\./gi) ?? [];
        ghi({ tc: "TC-N26-BAN-CAI-DAT", screen: "N26.S01", layer: "L17", state: "cặp đám bạn chat-0/chat-1", action: "«…» ở tiêu đề chat (nhãn trợ năng «Cài đặt nhóm»)", cauHinh: cfg, expected: "sheet của chat hai người: tiêu đề và mọi dòng viết cho hai người, có dòng «Tờ giấy của hai mình»", status: d && /Tờ giấy của hai mình/.test(d.chu) && chuNhomCd.length === 0 && !/nhóm/i.test(`${d.ten} ${nhanNut}`) ? "PASS" : "FAIL", evidence: ["EV-N26-BAN-CAI-DAT-C1"], ghiChu: d ? `nút mở: nhãn trợ năng «${nhanNut}»; sheet: nhãn trợ năng «${d.ten}», chữ «${d.chu.slice(0, 220)}»; câu có «nhóm/hội»: ${chuNhomCd.map((x) => `«${x.trim()}»`).join(" ") || "không"}` : "không mở được sheet" });
        await t.page.keyboard.press("Escape");
        await t.page.waitForTimeout(600);
        const axe = await chayAxe(t.page);
        ghi({ tc: "TC-N26-BAN-AXE", screen: "N26.S01", state: "cặp đám bạn, khay đã đóng", action: "axe-core trên màn chat", cauHinh: cfg, expected: "không vi phạm serious/critical", status: axe.vi.filter((v) => ["serious", "critical"].includes(v.impact)).length === 0 ? "PASS" : "FAIL", ghiChu: `vi phạm: ${axe.vi.map((v) => `${v.rule}(${v.impact})×${v.so}: ${v.vd.map((x) => x.target).join(" ; ")}`).join(", ") || "không"}; contrast chưa rõ: ${axe.chuaRo.map((v) => v.so).reduce((a, b) => a + b, 0)}` });
      }
      await t.context.close();
    }
    if (chayPhan("ban", "ghep") && anhBan.length > 1) await ghepAnh(mt.browser, anhBan.map((a) => ({ file: join(mt.out, "jpg", `${a.id}.jpg`), nhan: a.nhan })), join(mt.out, "jpg", "EV-N26-BAN-KHAY-ghep.jpg"), { tieuDe: "N26 · khay chat hai người, cặp đám bạn (chat-0/chat-1), C1–C7" });
  }

  // ------------------------------------------------------------------ moi
  // chat-12 proposed opening the notebook; chat-13 has not answered. Read only: nobody answers here.
  if (chay("moi")) {
    const { cap, pa: p12, pb: p13 } = await capGiua("chat-12", "chat-13");
    const cho = ((await soCua(cap.id, p13)).pending_proposals ?? []).filter((d) => d.proposed_by_id === p12.person_id && d.purpose === "lap_so" && !d.my_granted);
    if (!cho.length) console.log("moi: không còn lời đề nghị lap_so của chat-12 chờ chat-13, bỏ qua");
    for (const cfg of cho.length ? ["C1", "C2", "C3", "C6"] : []) {
      if (!chayPhan("moi", cfg)) continue;
      const t = await mo(cfg, "chat-13", `/groups/${cap.id}/chat`);
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(1800);
      await choOn(t.page, { mang: t.mang });
      const g = await doHangGhim(t.page);
      await anh(t.page, `EV-N26-MOI-${cfg}`, t, { chuThich: g.moi ? [{ selector: '[data-testid="hang-loi-de-nghi"]', nhan: `hàng mời ${g.moi.w}×${g.moi.h}` }] : [] });
      const cau = g.moi?.chu.map((x) => x.chu).join(" ") ?? "";
      const dung = /^Chat Test 13 đề nghị lập sổ lời hẹn\. Mở để xem và trả lời\.$/.test(cau);
      const cat = g.moi?.chu.some((x) => x.cat) ?? false;
      ghi({ tc: "TC-N26-MOI-LAP-SO", screen: "N26.S01", layer: "-", state: "cặp đám bạn chat-12/chat-13, chat-12 đề nghị lập sổ, chat-13 chưa trả lời", action: "chat-13 mở chat đôi", cauHinh: cfg, expected: "hàng mời mỏng dưới tiêu đề đọc trọn câu «… đề nghị lập sổ lời hẹn. Mở để xem và trả lời.», là một nút có tên, không có hàng ghim Tờ giấy", status: g.moi && dung && !cat && !g.giay && g.moi.role === "button" ? "PASS" : "FAIL", evidence: [`EV-N26-MOI-${cfg}`], ghiChu: g.moi ? `hàng ${g.moi.w}×${g.moi.h}px ở y ${g.moi.top}, role ${g.moi.role}, tên «${g.moi.ten}»; chữ «${cau}» ${g.moi.chu.map((x) => `${x.cao}px${x.cat ? " bị cắt" : ""}`).join("/")}; hàng Tờ giấy: ${g.giay ? "có" : "không"}` : "không thấy hàng mời" });
      // The row's height against the tap-target floor (DESIGN.md 48dp, PRODUCT 44dp).
      ghi({ tc: "TC-N26-MOI-VUNG-BAM", screen: "N26.S01", state: "hàng mời lập sổ ở chat-13", action: "đo vùng bấm của hàng mời", cauHinh: cfg, expected: "vùng bấm cao ≥ 48dp (ngưỡng DESIGN.md; PRODUCT ghi 44)", status: g.moi && g.moi.h >= 48 ? "PASS" : "FAIL", evidence: [`EV-N26-MOI-${cfg}`], ghiChu: g.moi ? `${g.moi.w}×${g.moi.h}px (${g.moi.h >= 48 ? "đạt 48" : g.moi.h >= 44 ? "đạt 44, dưới 48" : "dưới 44"}); chữ ${g.moi.chu.map((x) => `${x.cao}px`).join("/")}` : "không thấy hàng mời" });
      if (cfg === "C1") {
        // The proposer's own chat: no question for them, no row.
        const tA = await mo(cfg, "chat-12", `/groups/${cap.id}/chat`);
        await tA.page.waitForTimeout(1800);
        await choOn(tA.page, { mang: tA.mang });
        const gA = await doHangGhim(tA.page);
        await anh(tA.page, "EV-N26-MOI-CUA-TOI-C1", tA);
        ghi({ tc: "TC-N26-MOI-CUA-TOI", screen: "N26.S01", state: "chat-12, người đề nghị lập sổ", action: "chat-12 mở chat đôi với chat-13", cauHinh: cfg, expected: "không hàng mời, không hàng ghim (lời đề nghị của chính mình không phải câu hỏi cho mình)", status: !gA.moi && !gA.giay ? "PASS" : "FAIL", evidence: ["EV-N26-MOI-CUA-TOI-C1"], ghiChu: `hàng mời: ${gA.moi ? `có «${gA.moi.chu.map((x) => x.chu).join(" ")}»` : "không"}; hàng Tờ giấy: ${gA.giay ? "có" : "không"}` });
        await tA.context.close();
        // The tap: where the row leads, and what the landing offers to answer with.
        const hang = g.moi ? { x: g.moi.x + g.moi.w / 2, y: g.moi.top + g.moi.h / 2 } : null;
        if (hang) {
          await cham(cdp, hang);
          await t.page.waitForTimeout(2200);
          await choOn(t.page, { mang: t.mang });
        }
        const den = await duongDan(t.page);
        const tra = await nut(t.page, "Xem lời đề nghị");
        const chuDen = await chuTrang(t.page);
        await anh(t.page, "EV-N26-MOI-DEN-C1", t);
        ghi({ tc: "TC-N26-MOI-CHAM", screen: "N26.S03", state: "hàng mời lập sổ ở chat-13", action: "chạm hàng mời", cauHinh: cfg, expected: "tới /groups/{id}/to-giay, và màn tới có ngay lối trả lời lời đề nghị («Xem lời đề nghị»)", status: /\/to-giay$/.test(den) && tra ? "PASS" : "FAIL", evidence: ["EV-N26-MOI-C1", "EV-N26-MOI-DEN-C1"], ghiChu: `tới ${den.replace(/[0-9a-f-]{36}/, "[id]")}; «Xem lời đề nghị»: ${tra ? `có (${tra.w}×${tra.h})` : "không"}; chữ màn «${chuDen.slice(0, 200)}»` });
      }
      await t.context.close();
    }
  }

  // ------------------------------------------------------------------ doi
  // A couple (chat-8 ↔ chat-9): the pinned paper row, five tools, the stickers for two.
  if (chay("doi")) {
    const { cap, pa: p8 } = await capGiua("chat-8", "chat-9");
    const laDoi = (await capCua(cap.id, p8)).cap_doi === true;
    if (!laDoi) console.log("doi: chat-8/chat-9 không còn là cặp đôi trên máy chủ");
    const duong = `/groups/${cap.id}/chat`;
    for (const cfg of laDoi ? ["C1", "C2", "C3", "C4", "C5", "C6", "C7", "C8", "C9"] : []) {
      if (!chayPhan("doi", cfg)) continue;
      const t = await mo(cfg, "chat-8", duong);
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(2000);
      await choOn(t.page, { mang: t.mang });
      const g = await doHangGhim(t.page);
      await anh(t.page, `EV-N26-DOI-${cfg}`, t);
      const moKhay = await bam(t, cdp, "Thêm vào cuộc trò chuyện");
      const khay = moKhay ? await doKhay(t.page) : null;
      const soan = await t.page.evaluate(() => {
        const o = [...document.querySelectorAll('[aria-label="Ô soạn tin"]')].find((x) => x.getClientRects().length);
        const gui = [...document.querySelectorAll('[aria-label="Gửi tin nhắn"]')].find((x) => x.getClientRects().length);
        const ro = o?.getBoundingClientRect();
        const rg = gui?.getBoundingClientRect();
        return { oTop: ro ? Math.round(ro.top) : null, oDay: ro ? Math.round(ro.bottom) : null, guiDay: rg ? Math.round(rg.bottom) : null, cao: innerHeight, cuon: document.scrollingElement.scrollHeight > innerHeight + 1 };
      });
      await anh(t.page, `EV-N26-DOI-KHAY-${cfg}`, t);
      const tenCong = khay?.cong.map((c) => c.ten) ?? [];
      const namCong = JSON.stringify(tenCong) === JSON.stringify(["Ảnh", "Sticker", "Bình chọn", "Tờ hẹn", "Tờ giấy"]);
      const cat = khay?.cong.filter((c) => c.nhan?.cat).map((c) => c.ten) ?? [];
      const dat = namCong && khay.soHang === 1 && cat.length === 0 && g.giay !== null;
      ghi({ tc: "TC-N26-DOI-KHAY", screen: "N26.S02", layer: "L16", state: "cặp đôi chat-8/chat-9 (cap_doi), chưa có tin", action: "mở chat → «Thêm vào cuộc trò chuyện»", cauHinh: cfg, expected: "hàng ghim «Tờ giấy» dưới tiêu đề; khay năm công cụ (thêm «Tờ giấy») trên một hàng, không nhãn nào bị cắt (762d5c5: 51.2 ở 320, 59.2 ở 360, 65.2 ở 390)", status: dat ? "PASS" : "FAIL", evidence: [`EV-N26-DOI-${cfg}`, `EV-N26-DOI-KHAY-${cfg}`], ghiChu: `hàng ghim: ${g.giay ? `Tờ giấy ${g.giay.w}×${g.giay.h} «${g.giay.chu.map((x) => x.chu).join(" · ").slice(0, 120)}»` : g.moi ? "hàng mời" : "không"}; ${khay ? `${khay.soCong} công cụ (${tenCong.join(", ")}), ${khay.soHang} hàng; ô ${khay.cong.map((c) => c.w).join("/")}px, ô vuông ${khay.cong.map((c) => c.icon?.w).join("/")}px; nhãn ${khay.cong.map((c) => `${c.ten} ${c.nhan?.dong} dòng`).join(", ")}; bị cắt: ${cat.join(", ") || "không"}; cuộn khay ${khay.cuon?.cao}/${khay.cuon?.noiDung}px` : "không mở được khay"}; ô soạn ${soan.oTop}–${soan.oDay}, nút gửi đáy ${soan.guiDay}, cửa sổ ${soan.cao}` });
      if (cfg === "C1") {
        // The stickers for two, and the fifth tool's way to the paper and back.
        await bam(t, cdp, "Sticker", { cho: 1200 });
        const st = await doSticker(t.page);
        await anh(t.page, "EV-N26-DOI-STICKER-C1", t);
        ghi({ tc: "TC-N26-DOI-STICKER", screen: "N26.S02", layer: "L19", state: "cặp đôi chat-8/chat-9", action: "khay → «Sticker»", cauHinh: cfg, expected: "tám sticker chung, rồi mục «Cho hai người» có bốn sticker; không nhãn nào bị cắt", status: st && st.so === 12 && /Cho hai người/.test(st.chu) && st.cat.length === 0 ? "PASS" : "FAIL", evidence: ["EV-N26-DOI-STICKER-C1"], ghiChu: st ? `${st.so} sticker (${st.ten.join(", ")}); tiêu đề: ${st.tieuDe.join(" / ") || "không có role=heading"}; «Cho hai người»: ${/Cho hai người/.test(st.chu) ? "có" : "không"}; nhãn bị cắt: ${st.cat.join(", ") || "không"}` : "không mở được khay sticker" });
        await t.page.keyboard.press("Escape");
        await t.page.waitForTimeout(700);
        await bam(t, cdp, "Thêm vào cuộc trò chuyện");
        const toGiay = await bam(t, cdp, "Tờ giấy", { cho: 2200 });
        await choOn(t.page, { mang: t.mang });
        const den = await duongDan(t.page);
        const tieuDe = await t.page.evaluate(() => (document.body.innerText ?? "").includes("Tờ giấy của hai mình"));
        await anh(t.page, "EV-N26-DOI-TO-GIAY-C1", t);
        const ve = await bam(t, cdp, "Quay lại", { cho: 2200 });
        const sau = await duongDan(t.page);
        const khaySau = await doKhay(t.page);
        ghi({ tc: "TC-N26-DOI-TO-GIAY", screen: "N26.S02", layer: "L16", state: "cặp đôi chat-8/chat-9, khay mở", action: "«Tờ giấy» → «Quay lại»", cauHinh: cfg, expected: "tới không gian giấy của cặp; Quay lại về chat, khay đã đóng", status: toGiay && /\/to-giay$/.test(den) && tieuDe && ve && /\/chat$/.test(sau) && !khaySau ? "PASS" : "FAIL", evidence: ["EV-N26-DOI-TO-GIAY-C1"], ghiChu: `chạm «Tờ giấy»: ${toGiay ? "có" : "không thấy"}; tới ${den.replace(/[0-9a-f-]{36}/, "[id]")} (tiêu đề ${tieuDe ? "có" : "không"}); «Quay lại»: ${ve ? "có" : "không"}; về ${sau.replace(/[0-9a-f-]{36}/, "[id]")}; khay sau khi về: ${khaySau ? "còn mở" : "đã đóng"}` });
        const axe = await chayAxe(t.page);
        ghi({ tc: "TC-N26-DOI-AXE", screen: "N26.S02", state: "cặp đôi, hàng ghim Tờ giấy", action: "axe-core trên màn chat", cauHinh: cfg, expected: "không vi phạm serious/critical", status: axe.vi.filter((v) => ["serious", "critical"].includes(v.impact)).length === 0 ? "PASS" : "FAIL", ghiChu: `vi phạm: ${axe.vi.map((v) => `${v.rule}(${v.impact})×${v.so}: ${v.vd.map((x) => x.target).join(" ; ")}`).join(", ") || "không"}; contrast chưa rõ: ${axe.chuaRo.map((v) => v.so).reduce((a, b) => a + b, 0)}` });
      }
      await t.context.close();
    }
  }

  // ------------------------------------------------------------------ soan
  // An empty two-person chat on a short window: the empty state is drawn in the column, not in the
  // list, so the tray and the composer come after it. Nothing is scrolled here (the finder above
  // scrolls controls into view, which moved the page at C8): positions are read as the window shows them.
  if (chay("soan")) {
    const oCuaSo = (page) =>
      page.evaluate(() => {
        const hien = (e) => e && e.getClientRects().length > 0;
        const o = [...document.querySelectorAll('[aria-label="Ô soạn tin"]')].find(hien);
        const gui = [...document.querySelectorAll('[aria-label="Gửi tin nhắn"]')].find(hien);
        const cong = [...document.querySelectorAll('[aria-label="Thêm vào cuộc trò chuyện"],[aria-label="Đóng công cụ chat"]')].find(hien);
        const hang = document.querySelector('[data-testid="khay-hang-cong-cu"]');
        const r = (e) => (e ? e.getBoundingClientRect() : null);
        const nhan = hang ? [...hang.querySelectorAll(':scope > [role="button"]')].map((b) => {
          const n = [...b.querySelectorAll('div[dir="auto"]')].pop()?.getBoundingClientRect();
          return n ? { ten: b.getAttribute("aria-label"), day: Math.round(n.bottom) } : null;
        }).filter(Boolean) : [];
        const se = document.scrollingElement;
        return {
          cao: innerHeight,
          cuonTop: Math.round(se.scrollTop),
          cuonDuoc: se.scrollHeight > innerHeight + 1,
          trangCao: se.scrollHeight,
          o: o ? { top: Math.round(r(o).top), day: Math.round(r(o).bottom) } : null,
          gui: gui ? { top: Math.round(r(gui).top), day: Math.round(r(gui).bottom) } : null,
          cong: cong ? { x: r(cong).left + r(cong).width / 2, y: r(cong).top + r(cong).height / 2, day: Math.round(r(cong).bottom) } : null,
          nhanNgoai: nhan.filter((n) => n.day > innerHeight).map((n) => n.ten),
          soNhan: nhan.length,
        };
      });
    for (const [ai, ten, phong] of [["chat-0", "chat-1", "ban"], ["chat-8", "chat-9", "doi"]]) {
      const { cap } = await capGiua(ai, ten);
      for (const cfg of ["C1", "C4", "C2", "C8"]) {
        if (!chayPhan("soan", `${phong}-${cfg}`) && !chayPhan("soan", phong)) continue;
        const t = await mo(cfg, ai, `/groups/${cap.id}/chat`);
        const cdp = await cdpCua(t.page);
        await t.page.waitForTimeout(2000);
        await choOn(t.page, { mang: t.mang });
        const dong = await oCuaSo(t.page);
        await anh(t.page, `EV-N26-SOAN-${phong}-${cfg}`, t);
        let mo_ = null;
        if (dong.cong && dong.cong.day <= dong.cao) {
          await cham(cdp, dong.cong);
          await t.page.waitForTimeout(1200);
          mo_ = await oCuaSo(t.page);
          await anh(t.page, `EV-N26-SOAN-${phong}-KHAY-${cfg}`, t);
        }
        const thay = (s) => s && s.o && s.gui && s.gui.day <= s.cao && s.o.top < s.cao;
        const dat = thay(dong) && (!mo_ || (thay(mo_) && mo_.nhanNgoai.length === 0));
        const ta = (s) => (s ? `ô soạn ${s.o ? `${s.o.top}–${s.o.day}` : "-"}, nút gửi đáy ${s.gui?.day ?? "-"}, cửa sổ ${s.cao}, trang cao ${s.trangCao}${s.cuonDuoc ? " (trang cuộn được trên web)" : ""}, cuộn ${s.cuonTop}` : "-");
        ghi({ tc: `TC-N26-SOAN-CHAT-RONG-${phong.toUpperCase()}`, screen: phong === "doi" ? "N26.S02" : "N26.S01", layer: "L16", state: `${phong === "doi" ? "cặp đôi chat-8/chat-9 (có hàng ghim Tờ giấy)" : "cặp đám bạn chat-0/chat-1"}, chưa có tin (trạng thái rỗng «Một lời mở đầu.»)`, action: "mở chat; chạm «+» (Thêm vào cuộc trò chuyện) nếu thấy", cauHinh: cfg, expected: "ô soạn và nút gửi nằm trong cửa sổ khi khay đóng và khi khay mở; nhãn công cụ không rơi ra ngoài đáy", status: dat ? "PASS" : "FAIL", evidence: [`EV-N26-SOAN-${phong}-${cfg}`, ...(mo_ ? [`EV-N26-SOAN-${phong}-KHAY-${cfg}`] : [])], ghiChu: `khay đóng: ${ta(dong)}; nút «+» ${dong.cong ? (dong.cong.day <= dong.cao ? "trong cửa sổ" : `ngoài cửa sổ (đáy ${dong.cong.day})`) : "không thấy"}; khay mở: ${mo_ ? `${ta(mo_)}; nhãn công cụ ngoài đáy: ${mo_.nhanNgoai.join(", ") || "không"} (${mo_.soNhan} công cụ)` : "không mở (nút «+» ngoài cửa sổ)"}` });
        await t.context.close();
      }
    }
  }

  // ------------------------------------------------------------------ chuyen
  // Friends to couple on screen, chat-10/chat-11 (notebook open, not a couple): chat-10 proposes
  // «Một đôi» through «Mở sổ cặp đôi»; chat-11 sees the slim line in the chat, follows it and agrees.
  // chat-10's chat stays open meanwhile. Writes: one bat_doi proposal and chat-11's consent.
  if (chay("chuyen")) {
    const { cap, pa: p10, pb: p11 } = await capGiua("chat-10", "chat-11");
    const giay = `/groups/${cap.id}/to-giay`;
    const chat = `/groups/${cap.id}/chat`;
    const truoc = await capCua(cap.id, p10);
    const soTruoc = await soCua(cap.id, p11);
    const daDoi = truoc.cap_doi === true;
    const choSan = (soTruoc.pending_proposals ?? []).find((d) => d.purpose === "bat_doi" && d.proposed_by_id === p10.person_id);
    /** Per animation frame: the top dialog and the notebook body, kept as runs of equal states. */
    const batMau = (page) =>
      page.evaluate(() => {
        const w = window;
        w.__cm = [];
        w.__cmDung = false;
        w.__cmT0 = performance.now();
        const hien = (x) => x.getClientRects().length > 0 && x.checkVisibility({ visibilityProperty: true, opacityProperty: true });
        const buoc = () => {
          if (w.__cmDung) return;
          const d = [...document.querySelectorAll('[role="dialog"]')].filter(hien).pop();
          const sheet = d ? d.getAttribute("aria-label") ?? "?" : "-";
          const trong = [...document.querySelectorAll('[data-testid="giay-trong"]')].find(hien);
          const than = trong ? (trong.querySelector('[aria-label="Sổ đã mở"]') ? "m6" : "giay-trong") : [...document.querySelectorAll('[data-testid="to-mo"]')].some(hien) ? "to-mo" : /Hai người cũng thành một hội/.test(document.body.innerText ?? "") ? "hoi" : "khac";
          const t = Math.round(performance.now() - w.__cmT0);
          const cuoi = w.__cm[w.__cm.length - 1];
          if (cuoi && cuoi.sheet === sheet && cuoi.than === than) {
            cuoi.n += 1;
            cuoi.den = t;
          } else w.__cm.push({ tu: t, den: t, n: 1, sheet, than });
          requestAnimationFrame(buoc);
        };
        requestAnimationFrame(buoc);
      });
    const tatMau = (page) =>
      page.evaluate(() => {
        window.__cmDung = true;
        return window.__cm;
      });
    // The friends' notebook, before anything is written (both sides read it the same way).
    for (const cfg of ["C1", "C2", "C3"]) {
      if (!chayPhan("chuyen", `so-ban-${cfg}`) || daDoi) continue;
      const t = await mo(cfg, "chat-10", giay);
      await t.page.waitForTimeout(2000);
      await choOn(t.page, { mang: t.mang });
      const chu = await chuTrang(t.page);
      const nutRu = await nut(t.page, "Rủ hội mình đi chơi");
      const nutDoi = await nut(t.page, "Mở sổ cặp đôi");
      await t.page.evaluate(() => window.scrollTo(0, 0));
      await anh(t.page, `EV-N26-SO-BAN-${cfg}`, t);
      ghi({ tc: "TC-N26-SO-BAN", screen: "N26.S03", state: "cặp đám bạn chat-10/chat-11, sổ đã mở (lap_so), chưa «Một đôi»", action: "chat-10 mở /groups/{id}/to-giay", cauHinh: cfg, expected: "thân sổ của đám bạn: nói hai người hẹn như một hội, có «Rủ hội mình đi chơi» và lối sang cặp đôi; không «Rủ đi chơi» của tờ giấy", status: /Hai người cũng thành một hội/.test(chu) && nutRu && nutDoi && !/Chưa có tờ nào tuần này/.test(chu) ? "PASS" : "FAIL", evidence: [`EV-N26-SO-BAN-${cfg}`], ghiChu: `chữ màn «${chu.slice(0, 260)}»; «Rủ hội mình đi chơi»: ${nutRu ? `${nutRu.w}×${nutRu.h}` : "không"}; «Mở sổ cặp đôi»: ${nutDoi ? `${nutDoi.w}×${nutDoi.h}` : "không"}` });
      await t.context.close();
    }
    if (chayPhan("chuyen", "doi") && !daDoi) {
      // 1. chat-10 proposes «Một đôi» on screen (unless an earlier run already did).
      const tA = await mo("C1", "chat-10", giay);
      const cdpA = await cdpCua(tA.page);
      await tA.page.waitForTimeout(2000);
      await choOn(tA.page, { mang: tA.mang });
      const buocA = [];
      if (!choSan) {
        buocA.push(`«Mở sổ cặp đôi»: ${(await bam(tA, cdpA, "Mở sổ cặp đôi", { cho: 1200 })) ? "có" : "không"}`);
        const loaiSo = await dialogTren(tA.page);
        await anh(tA.page, "EV-N26-CHUYEN-LOAI-SO-A-C1", tA);
        buocA.push(`sheet «${loaiSo?.ten ?? "-"}»: «${(loaiSo?.chu ?? "").slice(0, 150)}»`);
        const doiSeg = await tA.page.evaluate(() => {
          const e = document.querySelector('[data-testid="loai-so-doi"]');
          if (!e || !e.getClientRects().length) return null;
          const r = e.getBoundingClientRect();
          return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
        });
        if (doiSeg) await cham(cdpA, doiSeg);
        await tA.page.waitForTimeout(1200);
        const batDoi = await dialogTren(tA.page);
        await anh(tA.page, "EV-N26-CHUYEN-BAT-DOI-A-C1", tA);
        buocA.push(`«Cặp đôi»: ${doiSeg ? "có" : "không"} → sheet «${batDoi?.ten ?? "-"}»: «${(batDoi?.chu ?? "").slice(0, 200)}»`);
        const deNghi = await bam(tA, cdpA, "Đề nghị bật «Một đôi»", { cho: 1800, trongDialog: true });
        const sau = await dialogTren(tA.page);
        buocA.push(`«Đề nghị bật «Một đôi»»: ${deNghi ? "có" : "không"} → «${(sau?.chu ?? "").slice(0, 160)}»`);
        await bam(tA, cdpA, "Để sau", { cho: 900, trongDialog: true });
      }
      const choA = ((await soCua(cap.id, p11)).pending_proposals ?? []).find((d) => d.purpose === "bat_doi" && d.proposed_by_id === p10.person_id);
      ghi({ tc: "TC-N26-CHUYEN-DE-NGHI", screen: "N26.S03", layer: "L23", state: "cặp đám bạn chat-10/chat-11, sổ đã mở", action: "chat-10: «Mở sổ cặp đôi» → «Cặp đôi» → «Đề nghị bật «Một đôi»» → «Để sau»", cauHinh: "C1", expected: "lời đề nghị bat_doi của chat-10 ghi trên máy chủ, sheet nói đã đề nghị và chờ người kia", status: choA ? "PASS" : "FAIL", evidence: choSan ? [] : ["EV-N26-CHUYEN-LOAI-SO-A-C1", "EV-N26-CHUYEN-BAT-DOI-A-C1"], ghiChu: `${choSan ? "lời đề nghị đã có từ lượt trước; " : ""}${buocA.join("; ")}; máy chủ: lời đề nghị bat_doi chờ chat-11: ${choA ? "có" : "không"}` });
      // 2. chat-10 back in the chat, left open there while chat-11 answers.
      await diToi(tA, chat);
      const aTruoc = await doHangGhim(tA.page);
      // 3. chat-11: the slim line in the chat, where it leads, and the answer.
      const tB = await mo("C1", "chat-11", chat);
      const cdpB = await cdpCua(tB.page);
      await tB.page.waitForTimeout(2000);
      await choOn(tB.page, { mang: tB.mang });
      const gB = await doHangGhim(tB.page);
      await anh(tB.page, "EV-N26-CHUYEN-MOI-B-C1", tB, { chuThich: gB.moi ? [{ selector: '[data-testid="hang-loi-de-nghi"]', nhan: "hàng mời «Một đôi»" }] : [] });
      const cauB = gB.moi?.chu.map((x) => x.chu).join(" ") ?? "";
      ghi({ tc: "TC-N26-MOI-BAT-DOI", screen: "N26.S01", state: "cặp đám bạn chat-10/chat-11, chat-10 đề nghị «Một đôi»", action: "chat-11 mở chat đôi", cauHinh: "C1", expected: "hàng mời đọc trọn «… đề nghị hai bạn là «Một đôi». Mở để xem và trả lời.», chưa có hàng ghim Tờ giấy", status: gB.moi && /đề nghị hai bạn là «Một đôi»\. Mở để xem và trả lời\.$/.test(cauB) && !gB.moi.chu.some((x) => x.cat) && !gB.giay ? "PASS" : "FAIL", evidence: ["EV-N26-CHUYEN-MOI-B-C1"], ghiChu: gB.moi ? `hàng ${gB.moi.w}×${gB.moi.h}px, chữ «${cauB}»; hàng Tờ giấy: ${gB.giay ? "có" : "không"}` : "không thấy hàng mời" });
      if (gB.moi) {
        await cham(cdpB, { x: gB.moi.x + gB.moi.w / 2, y: gB.moi.top + gB.moi.h / 2 });
        await tB.page.waitForTimeout(2200);
        await choOn(tB.page, { mang: tB.mang });
      }
      const denB = await duongDan(tB.page);
      const chuDen = await chuTrang(tB.page);
      const traLoiNgay = await nut(tB.page, "Đồng ý là một đôi");
      await tB.page.evaluate(() => window.scrollTo(0, 0));
      await anh(tB.page, "EV-N26-CHUYEN-DEN-B-C1", tB);
      const nhacDeNghi = /đề nghị hai bạn là «Một đôi»|đề nghị «Một đôi»/.test(chuDen);
      ghi({ tc: "TC-N26-MOI-BAT-DOI-DEN", screen: "N26.S03", state: "hàng mời «Một đôi» ở chat-11", action: "chạm hàng mời", cauHinh: "C1", expected: "như hàng mời lập sổ: màn tới nói có lời đề nghị và có ngay lối trả lời (hàng mời hứa «Mở để xem và trả lời»)", status: /\/to-giay$/.test(denB) && (nhacDeNghi || traLoiNgay) ? "PASS" : "FAIL", evidence: ["EV-N26-CHUYEN-MOI-B-C1", "EV-N26-CHUYEN-DEN-B-C1"], ghiChu: `tới ${denB.replace(/[0-9a-f-]{36}/, "[id]")}; màn nhắc lời đề nghị: ${nhacDeNghi ? "có" : "không"}; «Đồng ý là một đôi» thấy ngay: ${traLoiNgay ? "có" : "không"}; chữ màn «${chuDen.slice(0, 260)}»` });
      // The answer lives in «Loại sổ», behind «Mở sổ cặp đôi» (Maestro 47 goes the same way).
      await bam(tB, cdpB, "Mở sổ cặp đôi", { cho: 1200 });
      const loaiSoB = await dialogTren(tB.page);
      await anh(tB.page, "EV-N26-CHUYEN-LOAI-SO-B-C1", tB);
      await batMau(tB.page);
      const dongY = await bam(tB, cdpB, "Đồng ý là một đôi", { cho: 3500, trongDialog: true });
      const chuoi = await tatMau(tB.page);
      const sauB = await chuTrang(tB.page);
      await tB.page.evaluate(() => window.scrollTo(0, 0));
      await anh(tB.page, "EV-N26-CHUYEN-SAU-B-C1", tB);
      const laDoi = (await capCua(cap.id, p11)).cap_doi === true;
      const m6 = chuoi.filter((k) => k.than === "m6").reduce((a, k) => a + k.n, 0);
      ghi({ tc: "TC-N26-CHUYEN-DONG-Y", screen: "N26.S03", layer: "L23", state: "chat-11 ở không gian giấy, lời đề nghị «Một đôi» của chat-10", action: "«Mở sổ cặp đôi» → «Đồng ý là một đôi»", cauHinh: "C1", expected: "sheet đóng, thân sổ sang sổ cặp đôi («Một đôi · …»), máy chủ báo cap_doi", status: dongY && laDoi && /Một đôi ·/.test(sauB) && !chuoi.some((k) => k.den > 1500 && k.sheet === "Loại sổ") ? "PASS" : "FAIL", evidence: ["EV-N26-CHUYEN-LOAI-SO-B-C1", "EV-N26-CHUYEN-SAU-B-C1"], ghiChu: `sheet trước khi chạm «${loaiSoB?.ten ?? "-"}»: «${(loaiSoB?.chu ?? "").slice(0, 160)}»; chạm «Đồng ý là một đôi»: ${dongY ? "có" : "không"}; cap_doi sau đó: ${laDoi}; chuỗi khung (sheet/thân): ${chuoi.map((k) => `${k.sheet}/${k.than}×${k.n}@${k.tu}`).join(" → ")}; chữ màn sau «${sauB.slice(0, 200)}»` });
      ghi({ tc: "TC-N26-M6-BAT-DOI", screen: "N26.S03", layer: "MO", state: "chat-11 đồng ý «Một đôi» ngay trên màn (sổ đã mở từ trước, lúc hai người còn là đám bạn)", action: "lấy mẫu mỗi khung rAF 3,5 s từ lúc chạm «Đồng ý là một đôi»", cauHinh: "C1", expected: "khoảnh khắc sổ hai người mở (M6, ADR-0037) diễn đúng một lần ở một bước của thang đồng ý: lúc lập sổ, hoặc lúc thành «Một đôi»", status: m6 > 0 ? "PASS" : "FAIL", evidence: ["EV-N26-CHUYEN-SAU-B-C1"], ghiChu: `khung có bìa «Sổ đã mở» (M6): ${m6}; chuỗi: ${chuoi.map((k) => `${k.than}×${k.n}`).join(" → ")}. Phía lập sổ: TC-R-UI-084-B (C1, C9) không thấy khung M6 nào, thân sổ sang «Hai người cũng thành một hội»` });
      // 4. chat-11 back in the chat: the couple's row and five tools.
      await bam(tB, cdpB, "Quay lại", { cho: 2500 });
      if (!/\/chat$/.test(await duongDan(tB.page))) await diToi(tB, chat);
      await tB.page.waitForTimeout(1500);
      const gB2 = await doHangGhim(tB.page);
      await bam(tB, cdpB, "Thêm vào cuộc trò chuyện");
      const khayB = await doKhay(tB.page);
      await anh(tB.page, "EV-N26-CHUYEN-CHAT-B-C1", tB);
      ghi({ tc: "TC-N26-CHUYEN-CHAT-TOI", screen: "N26.S02", state: "chat-11 vừa đồng ý «Một đôi»", action: "về chat đôi, mở khay", cauHinh: "C1", expected: "hàng mời biến mất, hàng ghim Tờ giấy hiện, khay năm công cụ", status: !gB2.moi && gB2.giay && khayB?.soCong === 5 ? "PASS" : "FAIL", evidence: ["EV-N26-CHUYEN-CHAT-B-C1"], ghiChu: `hàng mời: ${gB2.moi ? "còn" : "không"}; hàng Tờ giấy: ${gB2.giay ? "có" : "không"}; khay: ${khayB ? `${khayB.soCong} công cụ` : "không mở"}` });
      // 5. chat-10's chat, open the whole time: does the room become a couple's without leaving it?
      await tA.page.waitForTimeout(8000);
      const aSau = await doHangGhim(tA.page);
      const moA = await bam(tA, cdpA, "Thêm vào cuộc trò chuyện");
      const khayA = moA ? await doKhay(tA.page) : null;
      await anh(tA.page, "EV-N26-CHUYEN-CHAT-A-C1", tA);
      if (moA) await bam(tA, cdpA, "Đóng khay công cụ");
      // A return to the foreground re-reads capabilities (useChatAi's AppState listener).
      await tA.page.evaluate(() => {
        Object.defineProperty(document, "visibilityState", { value: "hidden", configurable: true });
        document.dispatchEvent(new Event("visibilitychange"));
      });
      await tA.page.waitForTimeout(400);
      await tA.page.evaluate(() => {
        Object.defineProperty(document, "visibilityState", { value: "visible", configurable: true });
        document.dispatchEvent(new Event("visibilitychange"));
      });
      await tA.page.waitForTimeout(2500);
      const aNen = await doHangGhim(tA.page);
      const moA2 = await bam(tA, cdpA, "Thêm vào cuộc trò chuyện");
      const khayA2 = moA2 ? await doKhay(tA.page) : null;
      ghi({ tc: "TC-N26-CHUYEN-BEN-KIA", screen: "N26.S02", state: "chat-10 để chat đôi mở trong lúc chat-11 đồng ý «Một đôi»", action: "đợi 8 s trên chat; rồi giả lập app về nền và trở lại (visibilitychange)", cauHinh: "C1", expected: "phòng của người đề nghị thành phòng cặp đôi mà không phải rời chat: hàng ghim Tờ giấy và công cụ thứ năm hiện", status: aSau.giay && khayA?.soCong === 5 ? "PASS" : "FAIL", evidence: ["EV-N26-CHUYEN-CHAT-A-C1"], ghiChu: `trước khi chat-11 đồng ý: hàng ${aTruoc.giay ? "Tờ giấy" : aTruoc.moi ? "mời" : "không"}; sau 8 s: hàng ${aSau.giay ? "Tờ giấy" : aSau.moi ? "mời" : "không"}, khay ${khayA ? `${khayA.soCong} công cụ` : "không mở"}; sau khi về nền rồi trở lại: hàng ${aNen.giay ? "Tờ giấy" : aNen.moi ? "mời" : "không"}, khay ${khayA2 ? `${khayA2.soCong} công cụ` : "không mở"}` });
      await tA.context.close();
      await tB.context.close();
    }
  }

  // ------------------------------------------------------------------ gu
  // «Gu của hai bạn» in the couple chat-8/chat-9, as chat-9. The switch is chat-9's own: turned on
  // (real consent, after ADR-0048's cut-off, so the chat is covered), then shown as a pre-cut-off
  // consent by rewriting `gu_chat.cua_toi` of the capabilities reply to «can_bat_lai» -- the only
  // way to reach that state now that the cut-off has passed -- and re-consented for real; then the
  // second call of the re-consent is made to fail. The switch is left off, as it was found.
  if (chay("gu")) {
    const { cap, pb: p9 } = await capGiua("chat-8", "chat-9");
    const giay = `/groups/${cap.id}/to-giay`;
    const guChat = async () => (await capCua(cap.id, p9)).gu_chat;
    const dau = await guChat();
    for (const cfg of ["C1", "C3", "C6"]) {
      if (!chayPhan("gu", cfg)) continue;
      const t = await mo(cfg, "chat-9", giay);
      const cdp = await cdpCua(t.page);
      const gia = { bat: false };
      const loiDeNghi = { bat: false };
      const goi = [];
      t.page.on("request", (r) => {
        if (/\/notebook\/(proposals|consents)/.test(r.url()) && r.method() !== "GET") goi.push(`${r.method()} ${new URL(r.url()).pathname.replace(/[0-9a-f-]{36}/, "[id]")}`);
      });
      await t.page.route(/\/chat-capabilities/, async (route) => {
        const r = await route.fetch();
        let j = null;
        try {
          j = await r.json();
        } catch {
          /* not JSON: pass it on */
        }
        if (gia.bat && j) j.gu_chat = { ...(j.gu_chat ?? { nguoi_kia: false }), cua_toi: "can_bat_lai" };
        await route.fulfill({ response: r, json: j ?? undefined });
      });
      await t.page.route(/\/notebook\/proposals$/, async (route) => {
        if (loiDeNghi.bat && route.request().method() === "POST") return route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ detail: "audit_injected_failure" }) });
        return route.continue();
      });
      await t.page.waitForTimeout(2000);
      await choOn(t.page, { mang: t.mang });
      const moGu = async () => {
        await bam(t, cdp, "Cài đặt sổ", { cho: 1100 });
        const hang = await nut(t.page, "Gu của hai bạn", { batDau: true, trongDialog: true });
        if (hang) await cham(cdp, hang);
        await t.page.waitForTimeout(1600);
        return hang;
      };
      const docSheet = () =>
        t.page.evaluate(() => {
          const d = document.querySelector('[data-testid="gu-hai-ban"]');
          if (!d || !d.getClientRects().length) return null;
          const tx = (id) => (d.querySelector(`[data-testid="${id}"]`)?.innerText ?? "").replace(/\s+/g, " ").trim() || null;
          const nut = [...d.querySelectorAll('[role="button"]')].filter((b) => b.getClientRects().length).map((b) => (b.getAttribute("aria-label") || b.innerText || "").replace(/\s+/g, " ").trim());
          const loi = [...document.querySelectorAll('[data-testid="loi-lenh-so"]')].find((x) => x.getClientRects().length);
          const rl = loi?.getBoundingClientRect();
          const trongSheet = loi ? d.contains(loi) : false;
          const r = d.getBoundingClientRect();
          return { chu: (d.innerText ?? "").replace(/\s+/g, " ").trim(), cuaToi: tx("gu-cua-toi"), batLaiCau: tx("gu-bat-lai-cau"), nut, rong: Math.round(r.width), loi: loi ? { chu: (loi.innerText ?? "").trim(), trongSheet, top: Math.round(rl.top), duoiLopPhu: document.elementFromPoint(rl.left + 4, rl.top + 4) !== loi && !loi.contains(document.elementFromPoint(rl.left + 4, rl.top + 4)) } : null };
        });
      const dong = async () => {
        await t.page.keyboard.press("Escape");
        await t.page.waitForTimeout(800);
      };
      const hangGu = await moGu();
      const s0 = await docSheet();
      await anh(t.page, `EV-N26-GU-${cfg}`, t);
      ghi({ tc: "TC-N26-GU-LOI", screen: "N26.S04", layer: "L23", state: `cặp đôi chat-8/chat-9, chat-9 chưa chia gu (gu_chat ${JSON.stringify(dau)})`, action: "không gian giấy → «Cài đặt sổ» → «Gu của hai bạn»", cauHinh: cfg, expected: "sheet nói bật là cho người ấy thấy gu, Nếp dùng khi phác tờ và Rủ Đi AI dùng trong chat của hai bạn (ADR-0048 §3.5); sheet ≤ 640dp ở tablet", status: s0 && /Rủ Đi AI trong chat của hai bạn/.test(s0.chu) && (cfg !== "C6" || s0.rong <= 640) ? "PASS" : "FAIL", evidence: [`EV-N26-GU-${cfg}`], ghiChu: s0 ? `hàng «Gu của hai bạn»: ${hangGu ? "có" : "không"}; sheet rộng ${s0.rong}px; chữ «${s0.chu.slice(0, 330)}»; nút: ${s0.nut.join(" | ")}` : `không mở được sheet (hàng «Gu của hai bạn»: ${hangGu ? "có" : "không"})` });
      if (cfg !== "C1" || !s0) {
        await t.context.close();
        continue;
      }
      // On: the real consent, after the cut-off.
      goi.length = 0;
      await bam(t, cdp, "Cho Chat Test 09 thấy gu của mình", { cho: 2500, trongDialog: true });
      const s1 = await docSheet();
      const g1 = await guChat();
      await anh(t.page, "EV-N26-GU-BAT-C1", t);
      ghi({ tc: "TC-N26-GU-BAT", screen: "N26.S04", layer: "L23", state: "sheet «Gu của hai bạn» của chat-9", action: "«Cho Chat Test 09 thấy gu của mình»", cauHinh: "C1", expected: "một lệnh đề nghị chia_gu; sheet sang «đang chia» nói cả chat; máy chủ báo gu_chat.cua_toi = bat; không có «Bật lại cho chat»", status: s1 && g1?.cua_toi === "bat" && !s1.batLaiCau && /Thôi cho/.test(s1.nut.join(" ")) ? "PASS" : "FAIL", evidence: ["EV-N26-GU-BAT-C1"], ghiChu: `lệnh: ${goi.join(", ") || "không"}; gu_chat sau: ${JSON.stringify(g1)}; dòng của tôi «${s1?.cuaToi}»; nút: ${s1?.nut.join(" | ")}` });
      // A consent from before the cut-off, as the server would word it: «can_bat_lai».
      await dong();
      gia.bat = true;
      await moGu();
      const s2 = await docSheet();
      await anh(t.page, "EV-N26-GU-BAT-LAI-C1", t);
      ghi({ tc: "TC-N26-GU-BAT-LAI-HIEN", screen: "N26.S04", layer: "L23", state: "chat-9 đang chia gu; phản hồi chat-capabilities sửa gu_chat.cua_toi thành can_bat_lai (page.route, vì mốc 29/09 đã qua nên không tạo được đồng ý cũ thật)", action: "mở lại «Gu của hai bạn»", cauHinh: "C1", expected: "một dòng giải thích vì sao và nút «Bật lại cho chat» (ADR-0048 §3.2)", status: s2?.batLaiCau && s2.nut.includes("Bật lại cho chat") ? "PASS" : "FAIL", evidence: ["EV-N26-GU-BAT-LAI-C1"], ghiChu: s2 ? `dòng «${s2.batLaiCau ?? "-"}»; nút: ${s2.nut.join(" | ")}` : "không mở được sheet" });
      // The re-consent for real: off, then on; the next read of the capabilities is the server's own.
      gia.bat = false;
      goi.length = 0;
      await bam(t, cdp, "Bật lại cho chat", { cho: 3500, trongDialog: true });
      const s3 = await docSheet();
      const g3 = await guChat();
      await anh(t.page, "EV-N26-GU-BAT-LAI-XONG-C1", t);
      ghi({ tc: "TC-N26-GU-BAT-LAI", screen: "N26.S04", layer: "L23", state: "sheet có «Bật lại cho chat»", action: "chạm «Bật lại cho chat»", cauHinh: "C1", expected: "hai lệnh theo thứ tự thu hồi rồi đề nghị chia_gu; công tắc vẫn bật; máy chủ báo cua_toi = bat; nút «Bật lại cho chat» biến mất", status: goi.length === 2 && /^DELETE/.test(goi[0]) && /^POST/.test(goi[1]) && g3?.cua_toi === "bat" && s3 && !s3.batLaiCau ? "PASS" : "FAIL", evidence: ["EV-N26-GU-BAT-LAI-XONG-C1"], ghiChu: `lệnh: ${goi.join(" → ") || "không"}; gu_chat sau: ${JSON.stringify(g3)}; dòng «${s3?.batLaiCau ?? "-"}»; nút: ${s3?.nut.join(" | ")}` });
      // The re-consent whose second call fails: the switch is left off, and the person must learn why.
      await dong();
      gia.bat = true;
      await moGu();
      gia.bat = false;
      loiDeNghi.bat = true;
      goi.length = 0;
      await bam(t, cdp, "Bật lại cho chat", { cho: 3500, trongDialog: true });
      const s4 = await docSheet();
      const g4 = await guChat();
      await anh(t.page, "EV-N26-GU-BAT-LAI-LOI-C1", t);
      loiDeNghi.bat = false;
      const loiThay = s4?.loi && !s4.loi.duoiLopPhu;
      ghi({ tc: "TC-N26-GU-BAT-LAI-LOI", screen: "N26.S04", layer: "L23", state: "sheet có «Bật lại cho chat»; lệnh đề nghị chia_gu trả 503 (page.route)", action: "chạm «Bật lại cho chat»", cauHinh: "C1", expected: "công tắc ở trạng thái tắt (đóng an toàn) và người bấm thấy một câu lỗi ngay ở chỗ đang nhìn, để bấm lại (ADR-0048 §3.2)", status: g4?.cua_toi === "tat" && loiThay ? "PASS" : "FAIL", evidence: ["EV-N26-GU-BAT-LAI-LOI-C1"], ghiChu: `lệnh: ${goi.join(" → ") || "không"}; gu_chat sau: ${JSON.stringify(g4)}; sheet: ${s4 ? `còn mở, nút ${s4.nut.join(" | ")}, dòng của tôi «${s4.cuaToi}»` : "đã đóng"}; câu lỗi: ${s4?.loi ? `«${s4.loi.chu}» ở y ${s4.loi.top}, ${s4.loi.trongSheet ? "trong sheet" : "ngoài sheet"}, ${s4.loi.duoiLopPhu ? "nằm dưới lớp phủ của sheet" : "thấy được"}` : "không có"}` });
      await dong();
      const cuoi = await guChat();
      console.log(`gu: công tắc của chat-9 lúc cuối ${JSON.stringify(cuoi)} (lúc đầu ${JSON.stringify(dau)})`);
      await t.context.close();
    }
    // The same sheet's plain switch failing (no re-consent): where does its sentence land?
    if (chi?.has("gu:loi-bat") || !chi || chi.has("gu")) {
      const t = await mo("C1", "chat-9", giay);
      const cdp = await cdpCua(t.page);
      const goi = [];
      t.page.on("request", (r) => {
        if (/\/notebook\/(proposals|consents)/.test(r.url()) && r.method() !== "GET") goi.push(`${r.method()} ${new URL(r.url()).pathname.replace(/[0-9a-f-]{36}/, "[id]")}`);
      });
      await t.page.route(/\/notebook\/proposals$/, (route) => (route.request().method() === "POST" ? route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ detail: "audit_injected_failure" }) }) : route.continue()));
      await t.page.waitForTimeout(2000);
      await choOn(t.page, { mang: t.mang });
      await bam(t, cdp, "Cài đặt sổ", { cho: 1100 });
      const hang = await nut(t.page, "Gu của hai bạn", { batDau: true, trongDialog: true });
      if (hang) await cham(cdp, hang);
      await t.page.waitForTimeout(1600);
      const g0 = await guChat();
      const bat = await bam(t, cdp, "Cho Chat Test 09 thấy gu của mình", { cho: 2500, trongDialog: true });
      const loi = await t.page.evaluate(() => {
        const d = document.querySelector('[data-testid="gu-hai-ban"]');
        const l = [...document.querySelectorAll('[data-testid="loi-lenh-so"]')].find((x) => x.getClientRects().length);
        if (!l) return { sheetMo: !!d?.getClientRects().length };
        const r = l.getBoundingClientRect();
        const tren = document.elementFromPoint(r.left + 4, r.top + 4);
        return { sheetMo: !!d?.getClientRects().length, chu: (l.innerText ?? "").trim(), top: Math.round(r.top), trongSheet: d ? d.contains(l) : false, duoiLopPhu: tren !== l && !l.contains(tren) };
      });
      const g5 = await guChat();
      await anh(t.page, "EV-N26-GU-BAT-LOI-C1", t);
      ghi({ tc: "TC-N26-GU-BAT-LOI", screen: "N26.S04", layer: "L23", state: `sheet «Gu của hai bạn» của chat-9, công tắc ${g0?.cua_toi}; lệnh đề nghị chia_gu trả 503 (page.route)`, action: "chạm «Cho Chat Test 09 thấy gu của mình»", cauHinh: "C1", expected: "công tắc vẫn tắt, câu lỗi hiện ngay trong sheet đang mở", status: bat && g5?.cua_toi === "tat" && loi.chu && !loi.duoiLopPhu ? "PASS" : "FAIL", evidence: ["EV-N26-GU-BAT-LOI-C1"], ghiChu: `chạm: ${bat ? "có" : "không thấy nút"}; lệnh: ${goi.join(" → ") || "không"}; gu_chat sau: ${JSON.stringify(g5)}; sheet ${loi.sheetMo ? "còn mở" : "đã đóng"}; câu lỗi: ${loi.chu ? `«${loi.chu}» ở y ${loi.top}, ${loi.trongSheet ? "trong sheet" : "ngoài sheet"}, ${loi.duoiLopPhu ? "dưới lớp phủ của sheet" : "thấy được"}` : "không có"}` });
      await t.context.close();
    }
  }

  // ------------------------------------------------------------------ ru
  // «Rủ … tới đây» on a place's page is offered for every open pair (PlaceDetailLive: the first three
  // active pairs), but a sheet with the place on it is a couple's thing now. Read only: nothing is
  // drafted, proposed or created (the outing form is left without saving).
  if (chay("ru")) {
    const CHO = "p-tiem-nuong-xom-lao";
    for (const [ai, ten, loai] of [["chat-0", "chat-1", "chua-so"], ["chat-4", "chat-5", "co-so"]]) {
      if (!chayPhan("ru", loai)) continue;
      const { cap } = await capGiua(ai, ten);
      const t = await mo("C1", ai, `/places/${CHO}`);
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(2200);
      await choOn(t.page, { mang: t.mang });
      // The place's own name, from the API: an empty name would match any text.
      const tenCho = (await docApi(`/places/${CHO}`, await phienCua(ai))).name ?? "Tiệm Nướng Xóm Lào";
      if (!tenCho) throw new Error("không đọc được tên quán");
      const nutRu = await t.page.evaluate(() => [...document.querySelectorAll('[role="button"]')].filter((b) => b.getClientRects().length && /^Rủ .* tới đây$/.test((b.getAttribute("aria-label") || b.innerText || "").replace(/[-]/g, "").trim())).map((b) => (b.getAttribute("aria-label") || b.innerText || "").replace(/[-]/g, "").trim()));
      const nhan = `Rủ ${tenHien(ten)} tới đây`;
      const p = await nut(t.page, nhan);
      await anh(t.page, `EV-N26-RU-${loai}-C1`, t, { chuThich: p ? [{ rect: { x: p.x - p.w / 2, y: p.y - p.h / 2, w: p.w, h: p.h }, nhan }] : [] });
      if (p) await cham(cdp, p);
      await t.page.waitForTimeout(2600);
      await choOn(t.page, { mang: t.mang });
      const den = await duongDan(t.page);
      const chuDen = await chuTrang(t.page);
      const nhacCho = chuDen.includes(tenCho) || /chỗ này|chỗ bạn chọn/i.test(chuDen);
      const cau = await t.page.evaluate(() => (document.querySelector('[data-testid="giay-goi-y-cho"]')?.innerText ?? "").trim() || null);
      await t.page.evaluate(() => window.scrollTo(0, 0));
      await anh(t.page, `EV-N26-RU-${loai}-DEN-C1`, t);
      let form = null;
      if (loai === "co-so" && (await nut(t.page, "Rủ hội mình đi chơi"))) {
        await bam(t, cdp, "Rủ hội mình đi chơi", { cho: 2600 });
        await choOn(t.page, { mang: t.mang });
        const chuForm = await chuTrang(t.page);
        form = { den: await duongDan(t.page), coCho: chuForm.includes(tenCho) };
        await anh(t.page, `EV-N26-RU-${loai}-FORM-C1`, t);
      }
      ghi({ tc: `TC-N26-RU-BAN-${loai === "co-so" ? "CO-SO" : "CHUA-SO"}`, screen: "N26.S06", state: `${ai} có chat đôi với ${ten}: đám bạn, ${loai === "co-so" ? "sổ đã mở" : "chưa sổ"}`, action: `chi tiết «${tenCho}» → «${nhan}»${loai === "co-so" ? " → «Rủ hội mình đi chơi»" : ""}`, cauHinh: "C1", expected: "nút «Rủ … tới đây» đưa chỗ vừa chọn vào một lời rủ với người đó, hoặc nói rõ vì sao không và chỗ đó đi đâu; không bỏ chỗ đã chọn trong im lặng", status: nhacCho && (!form || form.coCho) ? "PASS" : "FAIL", evidence: [`EV-N26-RU-${loai}-C1`, `EV-N26-RU-${loai}-DEN-C1`, ...(form ? [`EV-N26-RU-${loai}-FORM-C1`] : [])], ghiChu: `nút «Rủ … tới đây» trên trang: ${nutRu.join(", ") || "không"}; chạm «${nhan}»: ${p ? "có" : "không thấy"}; tới ${den.replace(/[0-9a-f-]{36}/, "[id]")}; câu về chỗ đã chọn: ${cau ? `«${cau}»` : "không"}; màn nhắc tên chỗ: ${chuDen.includes(tenCho) ? "có" : "không"}; chữ màn «${chuDen.slice(0, 220)}»${form ? `; «Rủ hội mình đi chơi» → ${form.den.replace(/[0-9a-f-]{36}/, "[id]")}, form có «${tenCho}»: ${form.coCho ? "có" : "không"}` : ""}` });
      await t.context.close();
      void cap;
    }
  }

  // ------------------------------------------------------------------ q2
  // Q2: the server still drafts a paper for a friends' pair (DraftPaper checks membership and the
  // cycle, not «Một đôi»). chat-6/chat-7: notebook open, not a couple. One draft by chat-6 over the
  // API -- a draft is private until sent, chat-7 never sees it -- then what chat-6's screens make of
  // it, then the draft is thrown away through the screen's own «Bỏ bản phác này».
  if (chay("q2")) {
    const { cap, pa: p6 } = await capGiua("chat-6", "chat-7");
    const c = await capCua(cap.id, p6);
    const toTruoc = (await docApi(`/contexts/${cap.id}/papers`, p6)).papers ?? [];
    const moSan = toTruoc.find((x) => ["nhap", "da_gui", "da_xem", "de_nghi_sua", "dong_y"].includes(x.state));
    const r = moSan ? { status: "đã có tờ mở từ lượt trước", json: moSan } : await ghiApi("POST", `/contexts/${cap.id}/papers/draft`, undefined, p6);
    const toSau = (await docApi(`/contexts/${cap.id}/papers`, p6)).papers ?? [];
    ghi({ tc: "TC-N26-Q2-PHAC-API", screen: "N26.S00", state: `cặp đám bạn chat-6/chat-7, sổ đã mở, cap_doi=${c.cap_doi}`, action: "chat-6: POST /contexts/{id}/papers/draft (API, như bản app cũ hay một client khác sẽ gọi)", cauHinh: "-", expected: "máy chủ từ chối phác tờ ở cặp chưa «Một đôi», vì từ #660 tờ giấy chỉ dành cho cặp đôi", status: typeof r.status === "number" && r.status >= 400 ? "PASS" : "FAIL", ghiChu: `trả ${r.status}${r.json?.state ? `, tờ ở trạng thái ${r.json.state}` : r.json?.code ? `, mã ${r.json.code}` : ""}; số tờ của cặp: ${toTruoc.length} → ${toSau.length} (${toSau.map((x) => x.state).join(", ") || "-"})` });
    const coTo = toSau.some((x) => x.state === "nhap");
    if (coTo && chayPhan("q2", "ui")) {
      const t = await mo("C1", "chat-6", `/groups/${cap.id}/to-giay`);
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(2200);
      await choOn(t.page, { mang: t.mang });
      const chu = await chuTrang(t.page);
      const gui = await nut(t.page, "Gửi cho người ấy");
      await t.page.evaluate(() => window.scrollTo(0, 0));
      await anh(t.page, "EV-N26-Q2-SO-C1", t);
      await diToi(t, `/groups/${cap.id}/chat`);
      const g = await doHangGhim(t.page);
      await anh(t.page, "EV-N26-Q2-CHAT-C1", t);
      ghi({ tc: "TC-N26-Q2-UI", screen: "N26.S03", state: "cặp đám bạn chat-6/chat-7 có một bản phác của chat-6 (phác qua API)", action: "chat-6 mở không gian giấy, rồi chat đôi", cauHinh: "C1", expected: "(ghi nhận) màn nào cho thấy bản phác ở cặp đám bạn, và có lối gửi đi không", status: "PASS", evidence: ["EV-N26-Q2-SO-C1", "EV-N26-Q2-CHAT-C1"], ghiChu: `không gian giấy: ${/Hai người cũng thành một hội/.test(chu) ? "thân đám bạn" : "thân tờ giấy"}, «Gửi cho người ấy» ${gui ? "có" : "không"}; chữ «${chu.slice(0, 200)}»; chat đôi: hàng ${g.giay ? "Tờ giấy" : g.moi ? "mời" : "không"}` });
      // Thrown away through the screen.
      await diToi(t, `/groups/${cap.id}/to-giay`);
      const bo = await bam(t, cdp, "Bỏ bản phác này", { cho: 1200 });
      const xn = bo ? await bam(t, cdp, "Bỏ bản phác", { cho: 2200, trongDialog: true }) : null;
      const toCuoi = (await docApi(`/contexts/${cap.id}/papers`, p6)).papers ?? [];
      console.log(`q2: bỏ bản phác: ${bo ? "có" : "không thấy"} / xác nhận: ${xn ? "có" : "không"}; tờ của cặp lúc cuối: ${toCuoi.map((x) => x.state).join(", ") || "-"}`);
      await t.context.close();
    }
  }

  // ------------------------------------------------------------------ loi
  // The couple's room when its reads fail: capabilities 503 (the room reads as friends, by design
  // «fails closed»), then the notebook read 503 under a working capabilities read.
  if (chay("loi")) {
    const { cap } = await capGiua("chat-8", "chat-9");
    for (const [phan, mau] of [["cap-503", /\/chat-capabilities/], ["so-503", /\/notebook$/]]) {
      if (!chayPhan("loi", phan)) continue;
      const t = await mo("C1", "chat-8", "/messages");
      const cdp = await cdpCua(t.page);
      // Every read of the two, with the status the page got: the fault must be seen to land.
      const doc = [];
      t.page.on("response", (r) => {
        const u = new URL(r.url()).pathname;
        if (/\/chat-capabilities$|\/notebook$/.test(u) && r.request().method() === "GET") doc.push(`${u.split("/").pop()} ${r.status()}`);
      });
      await t.page.waitForTimeout(1500);
      await loiMayChu(t.page, mau, 503);
      await diToi(t, `/groups/${cap.id}/chat`);
      const g = await doHangGhim(t.page);
      const moKhay = await bam(t, cdp, "Thêm vào cuộc trò chuyện");
      const khay = moKhay ? await doKhay(t.page) : null;
      const chu = await chuTrang(t.page);
      await anh(t.page, `EV-N26-LOI-${phan}-C1`, t);
      if (moKhay) await bam(t, cdp, "Đóng khay công cụ");
      const docLucLoi = doc.splice(0);
      await goHet(t.page);
      // Back to the list and in again: the reads run on focus. «Quay lại» first, the list by URL if not.
      const veDs = await bam(t, cdp, "Quay lại", { cho: 1800 });
      const oDs = await duongDan(t.page);
      if (!/\/messages$/.test(oDs)) await diToi(t, "/messages");
      await diToi(t, `/groups/${cap.id}/chat`);
      const g2 = await doHangGhim(t.page);
      ghi({ tc: `TC-N26-LOI-${phan.toUpperCase()}`, screen: "N26.S02", state: `cặp đôi chat-8/chat-9; ${phan === "cap-503" ? "GET chat-capabilities" : "GET notebook"} trả 503`, action: "Tin nhắn → chat đôi; mở khay; rồi hết lỗi, quay ra vào lại", cauHinh: "C1", expected: phan === "cap-503" ? "phòng hiện như đám bạn (đóng an toàn): không «Tờ giấy», không lỗi tiếng Anh; vào lại khi hết lỗi thì là cặp đôi" : "không hàng ghim hỏng, không mã lỗi trên màn; vào lại khi hết lỗi thì hàng ghim hiện", status: !/\b(503|error|undefined|null)\b/i.test(chu) && g2.giay && (phan !== "cap-503" || (khay?.soCong === 4 && !g.giay)) ? "PASS" : "FAIL", evidence: [`EV-N26-LOI-${phan}-C1`], ghiChu: `lúc lỗi: hàng ${g.giay ? `Tờ giấy «${g.giay.chu.map((x) => x.chu).join(" · ").slice(0, 110)}»` : g.moi ? "mời" : "không"}, khay ${khay ? `${khay.soCong} công cụ` : "không mở"}; chữ lạ trên màn: ${(chu.match(/\b(503|error|undefined|null)\b/gi) ?? []).join(", ") || "không"}; «Quay lại»: ${veDs ? `có, tới ${oDs.replace(/[0-9a-f-]{36}/, "[id]")}` : "không thấy"}; vào lại khi hết lỗi: hàng ${g2.giay ? `Tờ giấy «${g2.giay.chu.map((x) => x.chu).join(" · ").slice(0, 110)}»` : g2.moi ? "mời" : "không"}; lần đọc lúc lỗi: ${docLucLoi.join(", ") || "không"}; lúc vào lại: ${doc.join(", ") || "không"}` });
      await t.context.close();
    }
  }

  // ------------------------------------------------------------------ lab
  // The lab board of the two classes (app/dev/hai-lop-chat.tsx). On a store-like build (the web
  // export, no fixture flag) it must not open; on the dev server with EXPO_PUBLIC_RUDI_FIXTURE=1
  // (AUDIT_BASE) it draws the chat's own components with invented data.
  if (chay("lab-e1")) {
    for (const persona of [null, "chat-0"]) {
      const t = await mo("C1", persona, "/dev/hai-lop-chat");
      await t.page.waitForTimeout(2200);
      await choOn(t.page, { mang: t.mang });
      const den = await duongDan(t.page);
      const chu = await chuTrang(t.page);
      await anh(t.page, `EV-N26-LAB-E1-${persona ? "co-phien" : "khong-phien"}-C1`, t);
      ghi({ tc: `TC-N26-LAB-E1-${persona ? "CO-PHIEN" : "KHONG-PHIEN"}`, screen: "N26.S05", state: `bản web export (không cờ fixture), ${persona ? "có phiên chat-0" : "không phiên"}`, action: "mở thẳng /dev/hai-lop-chat", cauHinh: "C1", expected: "trang lab không mở ở bản không có cờ fixture (CUA_FIXTURE_DEV): chuyển đi, không lộ dữ liệu bịa", status: !/\/dev\//.test(den) && !/Hai lớp chat hai người|Dữ liệu tổng hợp/.test(chu) ? "PASS" : "FAIL", evidence: [`EV-N26-LAB-E1-${persona ? "co-phien" : "khong-phien"}-C1`], ghiChu: `tới ${den}; chữ đầu «${chu.slice(0, 120)}»` });
      await t.context.close();
    }
  }
  if (chay("lab")) {
    /** One lab section by its title, scrolled to the top of the window. */
    const toiMuc = (page, tieuDe) =>
      page.evaluate((tieuDe) => {
        const h = [...document.querySelectorAll('[role="heading"],div[dir="auto"]')].find((x) => (x.innerText ?? "").trim() === tieuDe && x.getClientRects().length);
        if (!h) return false;
        h.scrollIntoView({ block: "start", behavior: "instant" });
        return true;
      }, tieuDe);
    for (const cfg of ["C1", "C3", "C9", "C2", "C6"]) {
      if (!chayPhan("lab", cfg)) continue;
      const w = { C1: 390, C3: 360, C9: 390, C2: 320, C6: 768 }[cfg];
      const t = await mo(cfg, null, "/dev/hai-lop-chat");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(3500);
      await choOn(t.page, { mang: t.mang, toiDa: 20_000 });
      const den = await duongDan(t.page);
      await anh(t.page, `EV-N26-LAB-${cfg}`, t);
      // The five-tool couple tray at the window's own width (the lab's fix of 28/09: never 4 + 1).
      await bam(t, cdp, "Khay cặp đôi", { cho: 1200 });
      const khay = await doKhay(t.page);
      await anh(t.page, `EV-N26-LAB-KHAY-DOI-${cfg}`, t);
      const cat = khay?.cong.filter((c) => c.nhan?.cat).map((c) => c.ten) ?? [];
      ghi({ tc: "TC-N26-LAB-KHAY-DOI", screen: "N26.S05", layer: "L16", state: `trang lab, bề rộng ${w}`, action: "chip «Khay cặp đôi»", cauHinh: cfg, expected: "năm công cụ; một hàng khi mỗi ô còn ≥ 50dp, không thì 3 + 2, không bao giờ 4 + 1; không nhãn nào bị cắt (762d5c5)", status: khay && khay.soCong === 5 && cat.length === 0 && (khay.soHang === 1 || JSON.stringify([...new Set(khay.cong.map((c) => c.y))].map((y) => khay.cong.filter((c) => c.y === y).length)) === "[3,2]") ? "PASS" : "FAIL", evidence: [`EV-N26-LAB-KHAY-DOI-${cfg}`], ghiChu: `tới ${den}; ${khay ? `${khay.soCong} công cụ, ${khay.soHang} hàng, ô ${khay.cong.map((c) => c.w).join("/")}px, ô vuông ${khay.cong.map((c) => c.icon?.w).join("/")}px; bị cắt: ${cat.join(", ") || "không"}; cuộn khay ${khay.cuon?.cao}/${khay.cuon?.noiDung}px` : "không thấy khay"}` });
      await bam(t, cdp, "Đóng khay công cụ", { cho: 800 });
      if (cfg !== "C1" && cfg !== "C9") {
        await t.context.close();
        continue;
      }
      // The chip's «Xem» sheet: fixed in 762d5c5 to cover the window, not the chip.
      if (cfg === "C1") {
        await toiMuc(t.page, "Chip trên nút gửi · hai người");
        await t.page.waitForTimeout(500);
        await anh(t.page, "EV-N26-LAB-CHIP-C1", t);
        const xem = await bam(t, cdp, "Xem những tin sẽ gửi kèm", { cho: 1200 });
        const tam = await t.page.evaluate(() => {
          const d = document.querySelector('[data-testid="chat-boi-canh-tam"]');
          if (!d || !d.getClientRects().length) return null;
          const r = d.getBoundingClientRect();
          const giua = document.elementFromPoint(innerWidth / 2, 60);
          const f = document.activeElement;
          return { x: Math.round(r.left), y: Math.round(r.top), w: Math.round(r.width), h: Math.round(r.height), giua: giua?.getAttribute("aria-label") ?? giua?.tagName, focus: f?.getAttribute("aria-label") ?? f?.tagName, muc: [...d.querySelectorAll('[data-testid="chat-boi-canh-muc"]')].map((x) => (x.innerText ?? "").trim()), chu: (d.innerText ?? "").replace(/\s+/g, " ").trim().slice(0, 200) };
        });
        await anh(t.page, "EV-N26-LAB-XEM-C1", t);
        await t.page.keyboard.press("Escape");
        await t.page.waitForTimeout(800);
        const sauEsc = await t.page.evaluate(() => ({ con: !!document.querySelector('[data-testid="chat-boi-canh-tam"]')?.getClientRects().length, focus: document.activeElement?.getAttribute("aria-label") ?? document.activeElement?.tagName }));
        ghi({ tc: "TC-N26-LAB-XEM", screen: "N26.S05", layer: "L30", state: "trang lab, chip «Kèm 3 tin gần đây» của hai người", action: "«Xem» → đọc tấm → Esc", cauHinh: "C1", expected: "tấm phủ cả cửa sổ và trượt từ đáy (không neo vào chip), liệt kê đúng ba tin, nói «cả hai bạn»; Esc đóng và trả tiêu điểm về «Xem»", status: xem && tam && tam.w >= w - 2 && tam.h > 700 && tam.muc.length === 3 && /hai bạn/.test(tam.chu) && !sauEsc.con && /Xem những tin/.test(sauEsc.focus ?? "") ? "PASS" : "FAIL", evidence: ["EV-N26-LAB-CHIP-C1", "EV-N26-LAB-XEM-C1"], ghiChu: tam ? `tấm ${tam.w}×${tam.h} tại (${tam.x},${tam.y}); điểm y60 thuộc «${tam.giua}»; tiêu điểm khi mở «${tam.focus}»; ${tam.muc.length} tin: ${tam.muc.join(" | ")}; chữ «${tam.chu}»; sau Esc: tấm ${sauEsc.con ? "còn" : "đóng"}, tiêu điểm «${sauEsc.focus}»` : `«Xem»: ${xem ? "chạm được" : "không thấy"}; không thấy tấm` });
      }
      // The answer being read and written. The lab's «Reduce Motion» chip is the row's own
      // `giamChuyenDong` (the lab does not read the system setting, so C9 changes nothing here):
      // with it on, the writing row shows whole sentences only (chay-may-that.md §5, capture 13).
      if (cfg === "C1") {
        await toiMuc(t.page, "Câu trả lời đang tới");
        await t.page.waitForTimeout(400);
        const docHang = () => t.page.evaluate(() => [...document.querySelectorAll('[data-testid="lab-tra-loi"] div[dir="auto"]')].map((x) => (x.innerText ?? "").trim()).filter((x) => /^Chiều thứ bảy/.test(x)).pop() ?? null);
        const doc = await t.page.evaluate(() => (document.querySelector('[data-testid="lab-tra-loi"]')?.innerText ?? "").replace(/\s+/g, " ").trim());
        const truoc = await docHang();
        await anh(t.page, "EV-N26-LAB-TRA-LOI-C1", t);
        await bam(t, cdp, "Reduce Motion", { cho: 900 });
        await toiMuc(t.page, "Câu trả lời đang tới");
        const sau = await docHang();
        await anh(t.page, "EV-N26-LAB-TRA-LOI-GIAM-C1", t);
        ghi({ tc: "TC-N26-LAB-TRA-LOI", screen: "N26.S05", layer: "MO", state: "trang lab, lượt hỏi của Linh: hàng đang đọc và hàng đang viết", action: "đọc hai hàng; bật chip «Reduce Motion» rồi đọc lại hàng đang viết", cauHinh: "C1", expected: "hàng đang đọc nói số tin và trích tin của «Bạn»; giảm chuyển động thì hàng đang viết chỉ hiện câu trọn (không có đoạn đang chạy dở)", status: /đang đọc 3 tin/.test(doc) && /Bạn/.test(doc) && sau && /\.$/.test(sau) && !/Rồi ghé quán/.test(sau) ? "PASS" : "FAIL", evidence: ["EV-N26-LAB-TRA-LOI-C1", "EV-N26-LAB-TRA-LOI-GIAM-C1"], ghiChu: `hai hàng «${doc.slice(0, 200)}»; hàng đang viết khi thường «${truoc}», khi giảm chuyển động «${sau}»` });
      }
      // Stickers of each class.
      if (cfg === "C1") {
        for (const [chip, lop] of [["Sticker đám bạn", "ban"], ["Sticker cặp đôi", "doi"]]) {
          await toiMuc(t.page, "Khay sticker");
          await bam(t, cdp, chip, { cho: 1200 });
          const st = await doSticker(t.page);
          await anh(t.page, `EV-N26-LAB-STICKER-${lop}-C1`, t);
          ghi({ tc: `TC-N26-LAB-STICKER-${lop.toUpperCase()}`, screen: "N26.S05", layer: "L19", state: "trang lab", action: `chip «${chip}»`, cauHinh: "C1", expected: lop === "doi" ? "mười hai sticker, mục «Cho hai người» bốn cái, không nhãn nào bị cắt" : "tám sticker, không mục «Cho hai người»", status: st && st.cat.length === 0 && (lop === "doi" ? st.so === 12 && /Cho hai người/.test(st.chu) : st.so === 8 && !/Cho hai người/.test(st.chu)) ? "PASS" : "FAIL", evidence: [`EV-N26-LAB-STICKER-${lop}-C1`], ghiChu: st ? `${st.so} sticker; «Cho hai người»: ${/Cho hai người/.test(st.chu) ? "có" : "không"}; bị cắt: ${st.cat.join(", ") || "không"}` : "không mở được" });
          await t.page.keyboard.press("Escape");
          await t.page.waitForTimeout(800);
        }
      }
      await t.context.close();
    }
  }

  // ------------------------------------------------------------------ nhay
  // Coming back to a couple's chat: capabilities are cleared on focus and read again, and until they
  // land the room is a friends' pair. Every animation frame records whether the paper row is drawn
  // and where the «Chưa mã hoá đầu cuối» line sits.
  if (chay("nhay")) {
    const { cap } = await capGiua("chat-8", "chat-9");
    for (const [phan, treMs] of [["mang-nhanh", 0], ["tre-800", 800]]) {
      if (!chayPhan("nhay", phan)) continue;
      const t = await mo("C1", "chat-8", `/groups/${cap.id}/chat`);
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(2000);
      await choOn(t.page, { mang: t.mang });
      const truoc = await doHangGhim(t.page);
      const ghim = truoc.giay ? { x: truoc.giay.x + truoc.giay.w / 2, y: truoc.giay.top + truoc.giay.h / 2 } : null;
      if (ghim) await cham(cdp, ghim);
      await t.page.waitForTimeout(2200);
      const o = await duongDan(t.page);
      if (treMs) await tre(t.page, /\/chat-capabilities/, treMs);
      await t.page.evaluate(() => {
        const w = window;
        w.__nh = [];
        w.__nhDung = false;
        w.__nhT0 = performance.now();
        const buoc = () => {
          if (w.__nhDung) return;
          const hien = (e) => e && e.getClientRects().length > 0;
          const giay = [...document.querySelectorAll('[data-testid="hang-to-giay"]')].some(hien);
          const bm = [...document.querySelectorAll('div[dir="auto"]')].find((x) => (x.innerText ?? "").trim() === "Chưa mã hoá đầu cuối" && hien(x));
          const y = bm ? Math.round(bm.getBoundingClientRect().top) : null;
          const t = Math.round(performance.now() - w.__nhT0);
          const cuoi = w.__nh[w.__nh.length - 1];
          if (cuoi && cuoi.giay === giay && cuoi.y === y) {
            cuoi.n += 1;
            cuoi.den = t;
          } else w.__nh.push({ tu: t, den: t, n: 1, giay, y });
          requestAnimationFrame(buoc);
        };
        requestAnimationFrame(buoc);
      });
      // With the delay, the frames themselves are kept: the row's absence is a moment, not an end state.
      const veP = await nut(t.page, "Quay lại");
      const q = treMs ? await quayKhung(t.page, cdp, { ms: 1500, kichThuocToiDa: 390 }) : null;
      if (veP) await cham(cdp, veP);
      const khung = q ? await q.dung() : null;
      await t.page.waitForTimeout(q ? 2000 : 3500);
      const ve = veP;
      if (khung) await ghepKhung(mt.browser, khung, join(mt.out, "jpg", `EV-N26-NHAY-${phan}-khung-C1.jpg`), { cot: 6, toiDa: 12, tieuDe: `N26 · về chat cặp đôi, chat-capabilities trễ ${treMs} ms: khung theo thời gian từ lúc chạm «Quay lại»` });
      const chuoi = await t.page.evaluate(() => {
        window.__nhDung = true;
        return window.__nh;
      });
      const sau = await duongDan(t.page);
      await anh(t.page, `EV-N26-NHAY-${phan}-C1`, t);
      // From the moment the chat is back on screen (the line visible), was the row ever missing?
      const tuLucHien = chuoi.filter((k) => k.y !== null);
      const mat = tuLucHien.filter((k) => !k.giay);
      const doi = new Set(tuLucHien.map((k) => k.y)).size;
      const cuoi = tuLucHien[tuLucHien.length - 1];
      ghi({ tc: `TC-N26-NHAY-VE-CHAT${treMs ? "-TRE" : ""}`, screen: "N26.S02", state: `cặp đôi chat-8/chat-9${treMs ? `, chat-capabilities trễ ${treMs} ms` : ""}`, action: "chạm hàng ghim Tờ giấy → không gian giấy → «Quay lại» về chat", cauHinh: "C1", expected: "về chat thì hàng ghim Tờ giấy đứng yên: không biến mất rồi hiện lại, dòng «Chưa mã hoá đầu cuối» không nhảy", status: mat.length === 0 && doi <= 1 ? "PASS" : "FAIL", evidence: [`EV-N26-NHAY-${phan}-C1`, ...(treMs ? [`EV-N26-NHAY-${phan}-khung-C1`] : [])], ghiChu: `chạm hàng ghim: ${ghim ? "có" : "không thấy hàng"}; tới ${o.replace(/[0-9a-f-]{36}/, "[id]")}; «Quay lại»: ${ve ? "có" : "không"}, về ${sau.replace(/[0-9a-f-]{36}/, "[id]")}; chuỗi khung (hàng ghim/y dòng bảo mật): ${chuoi.map((k) => `${k.giay ? "có" : "mất"}/${k.y ?? "-"}×${k.n}@${k.tu}`).join(" → ")}; khung không có hàng sau khi chat hiện: ${mat.reduce((a, k) => a + k.n, 0)} (${mat.length ? `${mat[0].tu}–${mat[mat.length - 1].den} ms` : "-"}); lúc cuối: ${cuoi ? `${cuoi.giay ? "có" : "mất"} hàng, y ${cuoi.y}` : "-"}` });
      await t.context.close();
    }
  }
} finally {
  await mt.dong();
}
