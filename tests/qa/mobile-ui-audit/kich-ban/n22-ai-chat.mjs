/* Audit of a feature new on main 461eabf: Rủ Đi AI inside the chat (AI v2, PR #654, and the
 * two-person chat of #659; task #22). Read against ADR-0046 (accepted by the product owner
 * 2026-09-27) and the code: `chat/nhac-ai.ts`, `chat/chip-boi-canh.ts`, `chat/useChatAi.ts`,
 * `screens/chat/TraLoiAiDangViet.tsx`, `nep/NepPhien.tsx`.
 *   - `@Rủ Đi`, `/plan`, `/chia-bill` are ordinary messages. Only once the server has stored
 *     the message does the app ask the AI, naming it. The chip above «Gửi» says what goes
 *     along («Kèm N tin gần đây · Xem · Chỉ gửi lời nhờ»), or that the AI is not ready.
 *   - The answer is written into the thread a few words at a time; other members watch the
 *     same row. A two-person chat has exactly a group's AI (owner decision 2026-09-28).
 * Stack 2 has no AI key, so every room reports `provider_unavailable`:
 *   - the live screens are first read as they are (not ready);
 *   - a «ready» room is staged by rewriting ONE response in the browser, the room's
 *     `chat-capabilities`. Every other call reaches the real server, which refuses the
 *     invocation honestly (503 provider_unavailable). The request the app sends is real;
 *   - failed invocations of the past are drawn by rewriting the invocation list (`loi-goi`);
 *   - states that need a model (words arriving, the hand-over to the card) are read on the
 *     lab board /dev/tra-loi-song of the dev server (fixture on), built by the shipped parts.
 *
 *   source audit-env-main.sh && node kich-ban/n22-ai-chat.mjs [--chi api,chan,nhac,san-sang,doi,xem-ghim,loi-goi,nep]
 *   AUDIT_BASE=http://127.0.0.1:8091 node kich-ban/n22-ai-chat.mjs --chi lab,lab-nhom,doi-lab   # dev server
 *   node kich-ban/n22-ai-chat.mjs --chi lab-prod                                # the store build
 *
 * Writes (report §A): `nhac` and `san-sang` send three messages to the chat-test group, each
 * only if the thread does not hold it yet; `chan` blocks chat-21 from chat-20 and unblocks
 * right after. Rows are TC-N22-*; the placeholder TC-N-22-AI-CHAT is withdrawn by the verdict
 * script.
 */
import { cauHinh } from "../thu-vien/cau-hinh.mjs";
import { chup } from "../thu-vien/chup.mjs";
import { cdpCua, cham, tamCua } from "../thu-vien/cu-chi.mjs";
import { choOn, duongDan } from "../thu-vien/dieu-huong.mjs";
import { choDialog } from "../thu-vien/lop-phu.mjs";
import { khoiDong, trangMoi } from "../thu-vien/moi-truong.mjs";
import { layPhien, personaTheoTen } from "../thu-vien/phien.mjs";
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
  // Bare ISO dates side by side read, to the repo guard, as a phone number (incident 34).
  if (rec.ghiChu) rec = { ...rec, ghiChu: rec.ghiChu.replace(/(?<!«)\b(\d{4}-\d{2}-\d{2})\b(?!»)/g, "«$1»") };
  so.ghi({ feature: "N22", nenTang: "web", method: "RUNTIME-WEB", layer: "-", issue: null, evidence: [], ...rec });
  console.log(`${rec.status.padEnd(10)} ${rec.tc} ${rec.cauHinh ?? ""} ${rec.ghiChu ?? ""}`);
};
const P = (ten) => personaTheoTen(ten, mt.chatSessions);
const phienDir = join(mt.out, "phien");
const phienCua = (ten) => layPhien(mt.api, P(ten), phienDir);
const mo = (cfg, persona, path) => trangMoi(mt, cauHinh(cfg), { persona: persona ? P(persona) : null, path });
const anh = (page, id, t, extra = {}) => chup(page, { out: mt.out, id, suKien: t.suKien, ...extra });
const G = JSON.parse(readFileSync(mt.chatSessions, "utf8")).groupId;
const anId = (s) => String(s ?? "").replace(/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/g, "[id]");
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
/** chat-N shows as «Chat Test NN», counted from 01. */
const tenHien = (ten) => `Chat Test ${String(Number(ten.split("-")[1]) + 1).padStart(2, "0")}`;
/** The two-person chat of A and B, as A reads it from its contexts. */
const capGiua = async (tenA, tenB) => {
  const pa = await phienCua(tenA);
  const cap = ((await goi("GET", "/people/me/contexts", undefined, pa)).json?.contexts ?? []).find((c) => c.kind === "pair" && c.counterpart?.display_name === tenHien(tenB));
  if (!cap) throw new Error(`không thấy chat đôi ${tenA}/${tenB}`);
  return { cap, pa };
};
const capCua = async (id, phien) => (await goi("GET", `/contexts/${id}/chat-capabilities`, undefined, phien)).json;
const tinNhom = async (phien) => (await goi("GET", `/contexts/${G}/messages?limit=100`, undefined, phien)).json?.messages ?? [];

/**
 * A control by its accessible name or visible text, scrolled into view, and only if a tap
 * at its centre lands on it (lesson of checkpoint retest 2).
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
        if (tren && (tren === e || e.contains(tren)))
          return { x, y, w: Math.round(r.width), h: Math.round(r.height), ten: ten(e), tat: e.getAttribute("aria-disabled") === "true" || e.hasAttribute("disabled") };
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
/** Inside the app, the way a tap would get there: the stack keeps its history. */
const diToi = async (t, path) => {
  await t.page.evaluate((p) => {
    history.pushState({}, "", p);
    dispatchEvent(new PopStateEvent("popstate"));
  }, path);
  await t.page.waitForTimeout(2500);
  await choOn(t.page, { mang: t.mang });
};
/** Tap the composer and type, after clearing what it holds. */
const goSoan = async (t, cdp, chu) => {
  const o = await t.page.evaluate(() => {
    const e = [...document.querySelectorAll('[aria-label="Ô soạn tin"]')].find((x) => x.getClientRects().length);
    if (!e) return null;
    const r = e.getBoundingClientRect();
    return { x: r.left + Math.min(40, r.width / 2), y: r.top + r.height / 2 };
  });
  if (!o) return false;
  await cham(cdp, o);
  await t.page.waitForTimeout(250);
  await t.page.keyboard.press("Control+A");
  await t.page.keyboard.press("Backspace");
  if (chu) await t.page.keyboard.type(chu, { delay: 12 });
  await t.page.waitForTimeout(700);
  return true;
};
/** The chip above «Gửi»: its sentence, and its two small buttons with their boxes. */
const docChip = (page) =>
  page.evaluate(() => {
    const c = [...document.querySelectorAll('[data-testid="chat-chip-boi-canh"]')].find((x) => x.getClientRects().length);
    if (!c) return null;
    const hop = (e) => {
      if (!e) return null;
      const r = e.getBoundingClientRect();
      return { w: Math.round(r.width), h: Math.round(r.height), x: Math.round(r.left), y: Math.round(r.top), chu: (e.innerText ?? "").replace(/\s+/g, " ").trim() };
    };
    const cau = c.querySelector('[data-testid="chat-boi-canh"]');
    const o = [...document.querySelectorAll('[aria-label="Ô soạn tin"]')].find((x) => x.getClientRects().length);
    const gui = [...document.querySelectorAll('[aria-label="Gửi tin nhắn"]')].find((x) => x.getClientRects().length);
    return {
      cau: (cau?.innerText ?? "").replace(/\s+/g, " ").trim(),
      live: cau?.getAttribute("aria-live") ?? null,
      xem: hop(c.querySelector('[data-testid="chat-boi-canh-mo"]')),
      doi: hop(c.querySelector('[data-testid="chat-boi-canh-doi"]')),
      chip: hop(c),
      oSoan: hop(o),
      gui: hop(gui),
      cuaSo: { w: innerWidth, h: innerHeight },
    };
  });
/** What sits at the newest end of the thread: placeholders, AI rows, refusals, the notice. */
const docDauLuong = (page) =>
  page.evaluate(() => {
    const chu = (e) => (e?.innerText ?? "").replace(/[-]/g, "").replace(/\s+/g, " ").trim();
    const hien = (e) => e && e.getClientRects().length > 0;
    const t = document.body.innerText ?? "";
    const hop = (e) => {
      const r = e.getBoundingClientRect();
      return { y: Math.round(r.top), day: Math.round(r.bottom), trongCuaSo: r.bottom > 0 && r.top < innerHeight };
    };
    const loiNho = [...document.querySelectorAll('[data-testid="chat-loi-nho-hong"]')].filter(hien).map((e) => ({ chu: chu(e), ...hop(e), nut: [...e.querySelectorAll('[role="button"]')].map((b) => chu(b)) }));
    const song = [...document.querySelectorAll('[data-testid^="chat-tra-loi-song-"],[data-testid^="chat-tra-loi-phong-"]')].filter(hien).map((e) => chu(e));
    return { dangHoi: /Đang hỏi Rủ Đi AI/.test(t), loiNho, song, chuaSanSang: /chưa sẵn sàng/.test(t) };
  });
/** The Nếp panel, opened from the margin slip the way a person does (as f00-nep-bang.mjs). */
async function moBangNep(t, cdp) {
  const mep = await tamCua(t.page, '[data-testid="nep-mep"]', { cuon: false });
  if (!mep) return { ok: false, ly: "không có mép Nếp" };
  const vw = await t.page.evaluate(() => innerWidth);
  await cham(cdp, { x: vw - 5, y: mep.y });
  await t.page.waitForTimeout(600);
  const dia = await tamCua(t.page, '[data-testid="nep-dia"]', { cuon: false });
  if (!dia) return { ok: false, ly: "chạm mép không rút Nếp ra" };
  await cham(cdp, dia);
  return choDialog(t.page, { co: true });
}
/** The room as a server with a key would describe it: only the three AI commands change. */
const sanSang = (raw) => {
  const c = JSON.parse(raw);
  for (const k of ["plan", "chia_bill", "hoi"]) c.ai[k] = { available: true, reason: null };
  return JSON.stringify(c);
};
const dungSanSang = (page, id) =>
  page.route(`**/contexts/${id}/chat-capabilities`, async (r) => {
    const resp = await r.fetch();
    await r.fulfill({ response: resp, body: sanSang(await resp.text()) });
  });

const CAU_NHAC = "@Rủ Đi tối nay nhóm mình đi đâu ngắm đèn?";
const CAU_KEM = "@Rủ Đi gợi ý quán ăn tối gần hồ cho cả nhóm";
const CAU_CHI = "@Rủ Đi cuối tuần này đi đâu cho mát?";

try {
  // ------------------------------------------------------------------ api
  // Read the contract, and ask once where nothing can be stored: the server refuses
  // before it writes (503 when no provider, 400 for an empty ask).
  if (chay("api")) {
    const p0 = await phienCua("chat-0");
    const phong = [["nhóm chat-test", G, p0]];
    for (const [a, b, ten] of [["chat-0", "chat-1", "đám bạn"], ["chat-8", "chat-9", "cặp đôi"]]) {
      const { cap, pa } = await capGiua(a, b);
      phong.push([`${ten} ${a}→${b}`, cap.id, pa]);
    }
    const bang = [];
    for (const [ten, id, p] of phong) {
      const c = await capCua(id, p);
      bang.push({ ten, plan: c?.ai?.plan, chia: c?.ai?.chia_bill, hoi: c?.ai?.hoi, mention: c?.ai?.mention, scope: c?.ai?.share_scope, stream: c?.ai?.stream ?? null });
    }
    const dongAnToan = bang.every((r) => [r.plan, r.chia, r.hoi].every((x) => x?.available === false && x?.reason === "provider_unavailable") && r.mention === true && r.scope === "caller_attached");
    ghi({ tc: "TC-N22-API-CAP", screen: "N22.S00", state: "stack không có khoá AI; ba phòng: nhóm chat-test, đám bạn chat-0/chat-1, cặp đôi chat-8/chat-9", action: "GET /contexts/{id}/chat-capabilities (chỉ đọc)", cauHinh: "-", expected: "đóng an toàn: plan, chia_bill, hoi đều available=false với lý do provider_unavailable; mention=true, share_scope caller_attached; cặp hai người có đúng AI của nhóm", status: dongAnToan ? "PASS" : "FAIL", ghiChu: bang.map((r) => `${r.ten}: plan ${r.plan?.available}/${r.plan?.reason}, chia_bill ${r.chia?.available}/${r.chia?.reason}, hoi ${r.hoi?.available}/${r.hoi?.reason}, mention ${r.mention}, share_scope ${r.scope}, stream ${r.stream}`).join(" | ") });
    const truoc = (await goi("GET", `/contexts/${G}/ai-invocations?limit=20`, undefined, p0)).json?.invocations?.length ?? null;
    const r503 = await goi("POST", `/contexts/${G}/ai-invocations`, { logical_id: randomUUID(), command: "plan", prompt: "Tối nay nhóm đi đâu ngắm đèn?" }, p0);
    const r400 = await goi("POST", `/contexts/${G}/ai-invocations`, { logical_id: randomUUID(), command: "plan", prompt: "   " }, p0);
    const sau = (await goi("GET", `/contexts/${G}/ai-invocations?limit=20`, undefined, p0)).json?.invocations?.length ?? null;
    ghi({ tc: "TC-N22-API-TU-CHOI", screen: "N22.S00", state: "chat-0 trong nhóm chat-test; máy chủ không có khoá AI", action: "POST /contexts/{id}/ai-invocations: một lời nhờ hợp lệ, một lời nhờ trống", cauHinh: "-", expected: "lời nhờ hợp lệ: 503 provider_unavailable, không lưu lời gọi nào; lời nhờ trống: từ chối 400 invalid_invocation (lỗi đầu vào của chatassist), cũng không lưu gì", status: r503.status === 503 && r503.code === "provider_unavailable" && r400.status === 400 && r400.code === "invalid_invocation" && truoc === sau ? "PASS" : "FAIL", ghiChu: `hợp lệ: ${r503.status} ${r503.code}; trống: ${r400.status} ${r400.code}; số lời gọi của chat-0 trước ${truoc}, sau ${sau}` });
  }

  // ----------------------------------------------------------------- chan
  // A pair closed by a block: the server's room gate (capConMo) refuses before it looks for a
  // provider, so even without a key a blocked pair answers 403 and an open pair 503.
  if (chay("chan")) {
    const p20 = await phienCua("chat-20");
    const p21 = await phienCua("chat-21");
    const { cap } = await capGiua("chat-20", "chat-21");
    const { cap: cap01 } = await capGiua("chat-0", "chat-1");
    const p0 = await phienCua("chat-0");
    const than = () => ({ logical_id: randomUUID(), command: "plan", prompt: "Cuối tuần đi đâu?" });
    const doiChung = await goi("POST", `/contexts/${cap01.id}/ai-invocations`, than(), p0);
    const daChanTruoc = JSON.stringify((await goi("GET", "/people/me/blocked", undefined, p20)).json ?? {}).includes(p21.person_id);
    const chan = daChanTruoc ? { status: "đã chặn từ trước" } : await goi("POST", `/people/${p21.person_id}/block`, {}, p20);
    const tu21 = await goi("POST", `/contexts/${cap.id}/ai-invocations`, than(), p21);
    const tu20 = await goi("POST", `/contexts/${cap.id}/ai-invocations`, than(), p20);
    const capKhiChan = await goi("GET", `/contexts/${cap.id}/chat-capabilities`, undefined, p21);
    const boChan = daChanTruoc ? { status: "giữ nguyên" } : await goi("DELETE", `/people/${p21.person_id}/block`, undefined, p20);
    const conChan = JSON.stringify((await goi("GET", "/people/me/blocked", undefined, p20)).json ?? {}).includes(p21.person_id);
    ghi({ tc: "TC-N22-CHAN-API", screen: "N22.S00", state: "cặp chat-20/chat-21; chat-20 chặn chat-21 trong lúc đo, rồi bỏ chặn", action: "POST ai-invocations trong chat đôi từ cả hai phía khi đang chặn; đối chứng ở chat-0/chat-1 không chặn", cauHinh: "-", expected: "cặp đang chặn: 403 membership_required từ cả hai phía (cổng phòng chạy trước cổng nhà cung cấp); cặp không chặn: 503 provider_unavailable", status: tu21.status === 403 && tu21.code === "membership_required" && tu20.status === 403 && tu20.code === "membership_required" && doiChung.status === 503 ? "PASS" : "FAIL", ghiChu: `đối chứng chat-0/chat-1: ${doiChung.status} ${doiChung.code}; chặn: ${chan.status}; chat-21 gọi: ${tu21.status} ${tu21.code}; chat-20 gọi: ${tu20.status} ${tu20.code}; capabilities của chat-21 khi bị chặn: ${capKhiChan.status}${capKhiChan.json?.ai ? ` plan ${capKhiChan.json.ai.plan?.reason}` : ` ${capKhiChan.code}`}; bỏ chặn: ${boChan.status}; còn chặn sau cùng: ${conChan ? "có" : "không"}` });
  }

  // ----------------------------------------------------------------- nhac
  // The live group, not ready: the chip while «@Rủ Đi …» is typed (C1–C3), then one send at C1
  // with the message held 2.5 s at the browser, to read what the thread says meanwhile.
  if (chay("nhac")) {
    const p0 = await phienCua("chat-0");
    for (const cfg of ["C1", "C2", "C3"]) {
      if (!chayPhan("nhac", cfg)) continue;
      const t = await mo(cfg, "chat-0", `/groups/${G}/chat`);
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(3000);
      await choOn(t.page, { mang: t.mang });
      await goSoan(t, cdp, CAU_NHAC);
      const chip = await docChip(t.page);
      await anh(t.page, `EV-N22-CHIP-${cfg}`, t, chip?.chip ? { chuThich: [{ rect: { x: chip.chip.x, y: chip.chip.y, w: chip.chip.w, h: chip.chip.h } }] } : {});
      ghi({ tc: "TC-N22-CHIP-CHUA-SAN-SANG", screen: "N22.S01", state: "chat-0 trong nhóm chat-test; máy chủ báo provider_unavailable", action: `gõ «${CAU_NHAC}» vào ô soạn`, cauHinh: cfg, expected: "chip trên nút gửi nói Rủ Đi AI chưa sẵn sàng và tin đi như tin thường; không «Xem», không «Chỉ gửi lời nhờ»; chip, ô soạn và nút gửi trong cửa sổ", status: chip && /chưa sẵn sàng/.test(chip.cau) && /tin thường/.test(chip.cau) && !chip.xem && !chip.doi && chip.chip.y >= 0 && chip.gui && chip.gui.y + chip.gui.h <= chip.cuaSo.h ? "PASS" : "FAIL", evidence: [`EV-N22-CHIP-${cfg}`], ghiChu: chip ? `chip «${chip.cau}» ${chip.chip.w}×${chip.chip.h} ở y ${chip.chip.y}, aria-live ${chip.live}; «Xem» ${chip.xem ? "có" : "không"}, nút đổi ${chip.doi ? "có" : "không"}; ô soạn ${chip.oSoan ? `${chip.oSoan.w}×${chip.oSoan.h} ở y ${chip.oSoan.y}` : "không thấy"}, nút gửi ${chip.gui ? `ở y ${chip.gui.y}` : "không thấy"} (cửa sổ ${chip.cuaSo.h})` : "không thấy chip" });
      if (cfg === "C1" && chayPhan("nhac", "gui")) {
        const daCo = (await tinNhom(p0)).some((m) => m.body === CAU_NHAC);
        if (daCo) {
          console.log("nhac:gui: tin đã có trong nhóm từ lượt trước; không gửi lại");
        } else {
          const goiAi = [];
          t.page.on("request", (r) => { if (r.method() === "POST" && /\/ai-invocations(\/|$)/.test(new URL(r.url()).pathname)) goiAi.push(r.url()); });
          // Hold the message 2.5 s at the browser, so the pending state can be read.
          await t.page.route(`**/contexts/${G}/messages`, async (r) => {
            if (r.request().method() !== "POST") return r.continue();
            await new Promise((ok) => setTimeout(ok, 2500));
            await r.continue().catch(() => undefined);
          });
          const g = await bam(t, cdp, "Gửi tin nhắn", { cho: 900 });
          const luc = await docDauLuong(t.page);
          await anh(t.page, "EV-N22-GUI-CHO-C1", t);
          await t.page.waitForTimeout(4000);
          const sau = await docDauLuong(t.page);
          await anh(t.page, "EV-N22-GUI-SAU-C1", t);
          const tin = (await tinNhom(p0)).find((m) => m.body === CAU_NHAC) ?? null;
          const loiGoi = (await goi("GET", `/contexts/${G}/ai-invocations?limit=20`, undefined, p0)).json?.invocations?.length ?? null;
          ghi({ tc: "TC-N22-GUI-CHUA-SAN-SANG", screen: "N22.S01", state: "chat-0, nhóm chat-test, máy chủ không có khoá; tin giữ 2,5 s ở trình duyệt", action: `«Gửi tin nhắn» với «${CAU_NHAC}»`, cauHinh: "C1", expected: "đúng như chip nói: tin đi như tin thường. Trong lúc gửi không nói đang hỏi AI; sau khi gửi không gọi AI, không có hàng lời nhờ", status: g && tin && !luc.dangHoi && goiAi.length === 0 && !sau.loiNho.length && !sau.song.length ? "PASS" : "FAIL", evidence: ["EV-N22-GUI-CHO-C1", "EV-N22-GUI-SAU-C1"], ghiChu: `trong lúc gửi: «Đang hỏi Rủ Đi AI…» ${luc.dangHoi ? "có" : "không"}; sau 4 s: tin lưu ${tin ? `có (${tin.kind})` : "không"}, app gọi AI ${goiAi.length} lần, lời gọi trên máy chủ ${loiGoi}, hàng lời nhờ ${sau.loiNho.length}, hàng trả lời ${sau.song.length}, «Đang hỏi» còn ${sau.dangHoi ? "có" : "không"}` });
          await t.page.unroute(`**/contexts/${G}/messages`);
        }
      }
      await t.context.close();
    }
  }

  // ------------------------------------------------------------- san-sang
  // The same group as a server with a key would describe it (capabilities rewritten in the
  // browser; nothing else). The chip, «Xem», «Chỉ gửi lời nhờ», the request the app sends, and
  // the real server's 503, answered by «Rủ Đi AI chưa nhận lời nhờ», «Thử lại», «Bỏ».
  if (chay("san-sang")) {
    const p0 = await phienCua("chat-0");
    for (const cfg of ["C1", "C2", "C8"]) {
      if (!chayPhan("san-sang", cfg)) continue;
      const t = await mo(cfg, "chat-0", "/profile");
      const cdp = await cdpCua(t.page);
      await dungSanSang(t.page, G);
      const yeuCau = [];
      t.page.on("request", (r) => {
        if (r.method() === "POST" && /\/ai-invocations$/.test(new URL(r.url()).pathname)) yeuCau.push({ than: r.postData() ?? "", khoa: r.headers()["idempotency-key"] ?? null });
      });
      const tinDaLuu = [];
      t.page.on("response", async (r) => {
        if (r.request().method() === "POST" && new URL(r.url()).pathname.endsWith(`/contexts/${G}/messages`)) tinDaLuu.push(await r.json().catch(() => null));
      });
      await diToi(t, `/groups/${G}/chat`);
      await goSoan(t, cdp, CAU_KEM);
      const chip = await docChip(t.page);
      await anh(t.page, `EV-N22-SAN-CHIP-${cfg}`, t, chip?.chip ? { chuThich: [{ rect: { x: chip.chip.x, y: chip.chip.y, w: chip.chip.w, h: chip.chip.h } }] } : {});
      const n = Number((chip?.cau.match(/Kèm (\d+) tin/) ?? [])[1] ?? NaN);
      ghi({ tc: "TC-N22-SAN-CHIP", screen: "N22.S01", state: "nhóm chat-test như máy chủ có khoá (chỉ chat-capabilities được viết lại ở trình duyệt)", action: `gõ «${CAU_KEM}»`, cauHinh: cfg, expected: "chip «Kèm N tin gần đây · Xem · Chỉ gửi lời nhờ», chữ không tràn; «Xem» và «Chỉ gửi lời nhờ» là nút chạm được (≥ 44, DESIGN.md ≥ 48); chip, ô soạn và nút gửi trong cửa sổ", status: chip && Number.isFinite(n) && chip.xem && chip.doi && Math.min(chip.xem.h, chip.doi.h) >= 44 && chip.chip.x + chip.chip.w <= chip.cuaSo.w && chip.gui && chip.gui.y + chip.gui.h <= chip.cuaSo.h ? "PASS" : "FAIL", evidence: [`EV-N22-SAN-CHIP-${cfg}`], ghiChu: chip ? `chip «${chip.cau}» ${chip.chip.w}×${chip.chip.h} ở y ${chip.chip.y}; «${chip.xem?.chu ?? "-"}» ${chip.xem ? `${chip.xem.w}×${chip.xem.h}` : "không"}; «${chip.doi?.chu ?? "-"}» ${chip.doi ? `${chip.doi.w}×${chip.doi.h}` : "không"}; ô soạn ${chip.oSoan ? `y ${chip.oSoan.y}` : "-"}, nút gửi ${chip.gui ? `${chip.gui.y}–${chip.gui.y + chip.gui.h}` : "-"} (cửa sổ ${chip.cuaSo.w}×${chip.cuaSo.h})` : "không thấy chip" });
      if (cfg === "C1") {
        // ADR-0046 (accepted 2026-09-27), decision 2: the bundle is the ask, plus N recent messages
        // (default 20) plus at most 6 turns of a follow-up thread, under the 40-turn ceiling.
        const soTinNhom = (await tinNhom(p0)).length;
        ghi({ tc: "TC-N22-SAN-SO-TIN", screen: "N22.S01", state: `nhóm sẵn sàng (dựng); luồng có ${soTinNhom} tin`, action: "đọc số tin chip nói sẽ kèm (đếm từ chính gói)", cauHinh: "C1", expected: "mặc định kèm 20 tin gần nhất (ADR-0046, quyết định 2: «N tin gần (mặc định 20)… trong trần 40 lượt»)", status: Number.isFinite(n) && soTinNhom > 20 ? (n <= 20 ? "PASS" : "FAIL") : "BLOCKED", evidence: ["EV-N22-SAN-CHIP-C1"], ghiChu: `chip «${chip?.cau ?? "-"}»: ${Number.isFinite(n) ? n : "?"} tin; luồng ${soTinNhom} tin; gói dựng bằng gomBoiCanhChat không truyền soLuot nên lấy trần GIOI_HAN_BOI_CANH.soLuot` });
      }
      if (cfg !== "C1") {
        await t.context.close();
        continue;
      }
      // «Xem»: exactly the turns that will go.
      let xem = null;
      if (chip?.xem) {
        await cham(cdp, { x: chip.xem.x + chip.xem.w / 2, y: chip.xem.y + chip.xem.h / 2 });
        await t.page.waitForTimeout(1200);
        xem = await t.page.evaluate(() => {
          const tam = [...document.querySelectorAll('[data-testid="chat-boi-canh-tam"]')].find((x) => x.getClientRects().length);
          const d = [...document.querySelectorAll('[role="dialog"]')].filter((x) => x.getClientRects().length).pop();
          const muc = [...document.querySelectorAll('[data-testid="chat-boi-canh-muc"]')].map((e) => (e.innerText ?? "").trim());
          return { co: !!tam, ten: d?.getAttribute("aria-label") ?? null, dau: ((tam ?? d)?.innerText ?? "").replace(/\s+/g, " ").trim().slice(0, 260), muc };
        });
        await anh(t.page, "EV-N22-SAN-XEM-C1", t);
        await t.page.keyboard.press("Escape");
        await t.page.waitForTimeout(800);
      }
      ghi({ tc: "TC-N22-SAN-XEM", screen: "N22.S01", layer: "L-xem-boi-canh", state: "nhóm sẵn sàng (dựng), chip «Kèm N tin gần đây»", action: "chạm «Xem»", cauHinh: "C1", expected: "tấm «Những tin sẽ gửi kèm lời nhờ» liệt kê đúng N tin mà chip đếm; nói ai sẽ đọc lời nhờ và câu trả lời; không ảnh nào đi bằng ảnh", status: xem?.co && xem.muc.length === n ? "PASS" : "FAIL", evidence: ["EV-N22-SAN-XEM-C1"], ghiChu: xem ? `tấm ${xem.co ? "có" : "không"} («${xem.ten}»); chip đếm ${n}, tấm liệt kê ${xem.muc.length}; đầu tấm «${xem.dau}»; mục đầu «${anId(xem.muc[0] ?? "-").slice(0, 80)}»` : "không mở được «Xem»" });
      // Send with the bundle: the request as the app sends it, then the real refusal.
      const guiMot = async (cau, chiLoiNho) => {
        const daCo = (await tinNhom(p0)).some((m) => m.body === cau);
        if (daCo) return { boQua: true };
        await goSoan(t, cdp, cau);
        if (chiLoiNho) {
          const c = await docChip(t.page);
          if (c?.doi) await cham(cdp, { x: c.doi.x + c.doi.w / 2, y: c.doi.y + c.doi.h / 2 });
          await t.page.waitForTimeout(600);
        }
        const chipLuc = await docChip(t.page);
        const soYeuCau = yeuCau.length;
        const soTin = tinDaLuu.length;
        await bam(t, cdp, "Gửi tin nhắn", { cho: 4500 });
        const tin = tinDaLuu.slice(soTin).find(Boolean) ?? null;
        const yc = yeuCau.slice(soYeuCau);
        let than = null;
        try {
          than = yc[0] ? JSON.parse(yc[0].than) : null;
        } catch {
          than = null;
        }
        return { boQua: false, chipLuc, tin, yc, than };
      };
      if (chayPhan("san-sang", "gui")) {
        const kem = await guiMot(CAU_KEM, false);
        const dau = await docDauLuong(t.page);
        await anh(t.page, "EV-N22-SAN-LOI-C1", t);
        if (kem.boQua) console.log("san-sang:gui: tin kèm đã có từ lượt trước; không gửi lại");
        else {
          const luot = kem.than?.boi_canh?.luot ?? [];
          const coAnh = JSON.stringify(kem.than ?? {}).includes("image_url");
          ghi({ tc: "TC-N22-SAN-GUI-KEM", screen: "N22.S01", state: "nhóm sẵn sàng (dựng); chip «Kèm N tin»; máy chủ thật không có khoá", action: `gửi «${CAU_KEM}»`, cauHinh: "C1", expected: "tin lưu trước; rồi đúng một lời gọi AI: lệnh plan, lời nhờ đã bỏ «@Rủ Đi», trigger_message_id là tin vừa lưu, kèm đúng N lượt chip đếm, không image_url (03-…, ADR-0036 §2.5)", status: kem.tin && kem.yc.length === 1 && kem.than?.command === "plan" && kem.than?.trigger_message_id === kem.tin.id && luot.length === n && !coAnh && !/@Rủ Đi/.test(kem.than?.prompt ?? "") ? "PASS" : "FAIL", evidence: ["EV-N22-SAN-LOI-C1"], ghiChu: `tin lưu ${kem.tin ? "có" : "không"}; lời gọi ${kem.yc.length}; command ${kem.than?.command}; prompt «${kem.than?.prompt}»; trigger ${kem.than?.trigger_message_id === kem.tin?.id ? "đúng tin vừa lưu" : anId(kem.than?.trigger_message_id)}; boi_canh ${luot.length} lượt (chip đếm ${n}); trường của một lượt: ${Object.keys(luot[0] ?? {}).join(", ") || "-"}; image_url ${coAnh ? "CÓ" : "không"}; logical_id ${kem.than?.logical_id ? "có" : "không"}` });
          const hang = dau.loiNho[0] ?? null;
          ghi({ tc: "TC-N22-SAN-TU-CHOI", screen: "N22.S01", state: "lời gọi AI bị máy chủ thật từ chối 503 provider_unavailable", action: "đọc hàng ở cuối luồng sau khi gửi", cauHinh: "C1", expected: "hàng «Rủ Đi AI chưa nhận lời nhờ» nói lý do bằng lời (LOI_GOI_AI), có «Thử lại» và «Bỏ», nằm trong khung nhìn; tin vẫn ở luồng", status: hang && /chưa nhận lời nhờ/.test(hang.chu) && /chưa sẵn sàng/.test(hang.chu) && hang.nut.includes("Thử lại") && hang.nut.includes("Bỏ") && hang.trongCuaSo ? "PASS" : "FAIL", evidence: ["EV-N22-SAN-LOI-C1"], ghiChu: hang ? `«${hang.chu}» ở y ${hang.y}–${hang.day} (${hang.trongCuaSo ? "trong" : "ngoài"} cửa sổ); nút ${hang.nut.join(", ")}` : "không có hàng lời nhờ hỏng" });
          // «Thử lại»: the same key, the same bytes.
          const soYc = yeuCau.length;
          const thu = await bam(t, cdp, "Thử lại", { cho: 3000 });
          const lai = yeuCau.slice(soYc)[0] ?? null;
          ghi({ tc: "TC-N22-SAN-THU-LAI", screen: "N22.S01", state: "hàng «Rủ Đi AI chưa nhận lời nhờ»", action: "«Thử lại»", cauHinh: "C1", expected: "gửi lại đúng lời gọi cũ: cùng logical_id, cùng thân từng byte (gói đã đóng băng lúc bấm gửi), để máy chủ trả bản ghi cũ thay vì xung đột", status: thu && lai && lai.than === kem.yc[0].than ? "PASS" : "FAIL", ghiChu: `«Thử lại» ${thu ? "chạm" : "không thấy"}; lời gọi mới ${lai ? "có" : "không"}; thân ${lai ? (lai.than === kem.yc[0].than ? "trùng từng byte" : "KHÁC") : "-"}; idempotency-key ${lai ? (lai.khoa === kem.yc[0].khoa ? "trùng" : "khác") : "-"}` });
          // «Bỏ»: the row goes, the message stays.
          const bo = await bam(t, cdp, "Bỏ", { cho: 1200 });
          const sauBo = await docDauLuong(t.page);
          const conTin = await t.page.evaluate((cau) => (document.body.innerText ?? "").includes(cau.replace("@Rủ Đi ", "")), CAU_KEM);
          ghi({ tc: "TC-N22-SAN-BO", screen: "N22.S01", state: "hàng «Rủ Đi AI chưa nhận lời nhờ»", action: "«Bỏ»", cauHinh: "C1", expected: "hàng lời nhờ hỏng biến mất; tin @Rủ Đi vẫn ở luồng", status: bo && sauBo.loiNho.length === 0 && conTin ? "PASS" : "FAIL", ghiChu: `«Bỏ» ${bo ? "chạm" : "không thấy"}; hàng còn ${sauBo.loiNho.length}; tin còn ở luồng ${conTin ? "có" : "không"}` });
        }
        // «Chỉ gửi lời nhờ»: nothing goes along.
        const chiLoi = await guiMot(CAU_CHI, true);
        if (chiLoi.boQua) console.log("san-sang:gui: tin chỉ lời nhờ đã có từ lượt trước; không gửi lại");
        else {
          ghi({ tc: "TC-N22-SAN-CHI-LOI-NHO", screen: "N22.S01", state: "nhóm sẵn sàng (dựng)", action: `gõ «${CAU_CHI}», chạm «Chỉ gửi lời nhờ», gửi`, cauHinh: "C1", expected: "chip đổi thành «Chỉ gửi lời nhờ, không kèm tin nào · Kèm lại N tin»; lời gọi AI không có boi_canh", status: /không kèm tin nào/.test(chiLoi.chipLuc?.cau ?? "") && chiLoi.yc.length === 1 && chiLoi.than && !("boi_canh" in chiLoi.than) ? "PASS" : "FAIL", ghiChu: `chip lúc gửi «${chiLoi.chipLuc?.cau ?? "-"}»${chiLoi.chipLuc?.doi ? ` · «${chiLoi.chipLuc.doi.chu}»` : ""}; lời gọi ${chiLoi.yc.length}; boi_canh ${chiLoi.than && "boi_canh" in chiLoi.than ? "CÓ" : "không"}` });
          await bam(t, cdp, "Bỏ", { cho: 900 });
        }
      }
      await t.context.close();
    }
  }

  // -------------------------------------------------------------- loi-goi
  // Past invocations that failed, as the room's list would return them (the list rewritten in
  // the browser; the room itself is read as it is: not ready). What each row says and offers.
  if (chay("loi-goi")) {
    const p0 = await phienCua("chat-0");
    const tin = (await tinNhom(p0)).find((m) => m.body === CAU_NHAC) ?? (await tinNhom(p0))[0] ?? null;
    const luc = new Date(Date.now() - 600_000).toISOString();
    const hong = (id, command, code, trigger = null) => ({ id, status: "failed", code, command, message_id: null, trigger_message_id: trigger, so_tin_doc: null, created_at: luc, updated_at: luc });
    const DS = [
      hong("0b8f1c9e-aaaa-4bbb-8ccc-0000000c0001", "plan", "provider_unavailable"),
      hong("0b8f1c9e-aaaa-4bbb-8ccc-0000000c0002", "chia_bill", "chia_bill_no_expenses"),
      hong("0b8f1c9e-aaaa-4bbb-8ccc-0000000c0003", "hoi", "ai_tu_choi", tin?.id ?? null),
    ];
    const t = await mo("C1", "chat-0", "/profile");
    const cdp = await cdpCua(t.page);
    await t.page.route(`**/contexts/${G}/ai-invocations?*`, (r) => (r.request().method() === "GET" ? r.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ invocations: DS }) }) : r.continue()));
    await diToi(t, `/groups/${G}/chat`);
    await t.page.waitForTimeout(2500);
    const hang = await t.page.evaluate((LY_DO_SRC) => {
      const lyDo = (0, eval)(LY_DO_SRC);
      const chu = (e) => (e?.innerText ?? "").replace(/[-]/g, "").replace(/\s+/g, " ").trim();
      const out = [];
      for (const b of document.querySelectorAll('[role="button"]')) {
        if (!b.getClientRects().length || !/^(Thử lại lời nhờ|Tự tạo kèo)$/.test(chu(b))) continue;
        let k = b;
        for (let i = 0; i < 6 && k.parentElement && !/Lời nhờ|Rủ Đi AI|chưa|gom|phác/i.test(chu(k).replace(chu(b), "")); i++) k = k.parentElement;
        out.push({ nut: chu(b), tat: b.getAttribute("aria-disabled") === "true", lyDo: lyDo(b), vien: getComputedStyle(b).borderStyle, khoi: chu(k).slice(0, 220) });
      }
      const khoi = [...document.querySelectorAll("div")].filter((d) => d.getClientRects().length && /Rủ Đi AI chưa thấy khoản chi|Rủ Đi AI vừa viết ra một câu|AI chưa sẵn sàng|chưa phác được|không còn nữa/.test(d.innerText ?? "") && d.children.length <= 4).map((d) => chu(d).slice(0, 200));
      return { out, khoi: [...new Set(khoi)].slice(0, 6) };
    }, String.raw`(b) => {
      const s = b.nextElementSibling;
      if (!s || s.matches('[role="button"]') || s.querySelector('[role="button"]') || !/[-]/.test(s.innerText ?? "")) return null;
      return (s.innerText ?? "").replace(/[-]/g, "").replace(/\s+/g, " ").trim() || null;
    }`);
    await t.page.evaluate(() => document.querySelector('[data-testid="chat-loi-nho-hong"], [role="button"]')?.scrollIntoView({ block: "center" }));
    await anh(t.page, "EV-N22-LOI-GOI-C1", t, { fullPage: false });
    const thuLai = hang.out.filter((x) => x.nut === "Thử lại lời nhờ");
    ghi({ tc: "TC-N22-LOI-GOI-HANG", screen: "N22.S01", state: "nhóm chưa sẵn sàng; danh sách lời gọi viết lại ở trình duyệt: plan hỏng (provider_unavailable), chia_bill hỏng (chia_bill_no_expenses), hoi hỏng (ai_tu_choi)", action: "mở chat, đọc các hàng lời nhờ hỏng", cauHinh: "C1", expected: "mỗi hàng nói chuyện gì xảy ra và nên làm gì; «Thử lại lời nhờ» chỉ ở hàng thử lại có ích (không ở chia_bill_no_expenses); «Tự tạo kèo» ở hàng plan", status: thuLai.length === 2 && hang.out.some((x) => x.nut === "Tự tạo kèo") ? "PASS" : "FAIL", evidence: ["EV-N22-LOI-GOI-C1"], ghiChu: `nút: ${hang.out.map((x) => `«${x.nut}»${x.tat ? " tắt" : ""}`).join(", ") || "không"}; khối: ${hang.khoi.map((k) => `«${k}»`).join(" | ") || "-"}` });
    ghi({ tc: "TC-N22-LOI-GOI-THU-LAI-TAT", screen: "N22.S01", state: "như trên; phòng chưa sẵn sàng nên «Thử lại lời nhờ» tắt", action: "đọc trạng thái và lý do của «Thử lại lời nhờ»", cauHinh: "C1", expected: "nút tắt thì ẩn, hoặc viền đứt kèm lý do (ADR-0038): «AI chưa sẵn sàng…»", status: thuLai.length && thuLai.every((x) => !x.tat || x.lyDo) ? "PASS" : "FAIL", evidence: ["EV-N22-LOI-GOI-C1"], ghiChu: thuLai.map((x) => `«${x.nut}» ${x.tat ? "tắt" : "bật"}, viền ${x.vien}, lý do ${x.lyDo ? `«${x.lyDo}»` : "không"}`).join("; ") || "không có nút «Thử lại lời nhờ»" });
    await t.context.close();
  }

  // ------------------------------------------------------------------ nep
  // Nếp's panel with no key: the question, the «thinking» line, then the answer that never comes.
  if (chay("nep")) {
    const t = await mo("C1", "chat-0", "/explore");
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(2500);
    await choOn(t.page, { mang: t.mang });
    const moBang = await moBangNep(t, cdp);
    await t.page.waitForTimeout(800);
    const o = await t.page.evaluate(() => {
      const e = [...document.querySelectorAll('[aria-label="Hỏi Nếp"]')].find((x) => x.getClientRects().length && /^(INPUT|TEXTAREA)$/.test(x.tagName));
      if (!e) return null;
      e.scrollIntoView({ block: "center" });
      const r = e.getBoundingClientRect();
      return { x: r.left + 30, y: r.top + r.height / 2 };
    });
    const mau = [];
    if (o) {
      await cham(cdp, o);
      await t.page.keyboard.type("Tối nay đi đâu ngắm đèn?", { delay: 12 });
      await t.page.keyboard.press("Enter");
      const bd = Date.now();
      for (let i = 0; i < 40; i++) {
        mau.push({
          ms: Date.now() - bd,
          ...(await t.page.evaluate(() => ({
            nghi: document.querySelector('[data-testid="nep-dang-nghi"]')?.innerText?.trim() ?? null,
            viet: document.querySelector('[data-testid="nep-dang-viet"]')?.innerText?.trim() ?? null,
            loi: document.querySelector('[data-testid="nep-loi-hoi"]')?.innerText?.trim() ?? null,
            hoi: document.querySelector('[data-testid="nep-cau-dang-hoi"]')?.innerText?.trim() ?? null,
            liveNghi: document.querySelector('[data-testid="nep-dang-nghi"]')?.closest("[aria-live]")?.getAttribute("aria-live") ?? null,
            liveLoi: document.querySelector('[data-testid="nep-loi-hoi"]')?.closest("[aria-live]")?.getAttribute("aria-live") ?? null,
          }))),
        });
        if (mau.at(-1).loi) break;
        await t.page.waitForTimeout(250);
      }
    }
    await anh(t.page, "EV-N22-NEP-C1", t);
    const nghi = mau.find((m) => m.nghi);
    const loi = mau.find((m) => m.loi);
    ghi({ tc: "TC-N22-NEP-KHONG-KHOA", screen: "N22.S03", layer: "L03", state: "chat-0 ở Khám phá, bảng Nếp; máy chủ không có khoá AI", action: "hỏi «Tối nay đi đâu ngắm đèn?», Enter", cauHinh: "C1", expected: "câu hỏi hiện như lượt của mình, dòng «Nếp đang nghĩ…», rồi một câu nói Nếp chưa trả lời được (không đổ cho mạng); câu đọc được bằng trình đọc màn hình", status: moBang?.ok !== false && loi && !/mạng/.test(loi.loi) ? "PASS" : "FAIL", evidence: ["EV-N22-NEP-C1"], ghiChu: `bảng ${moBang?.ok === false ? `không mở (${moBang.ly})` : "mở"}; câu hỏi ${o ? "gõ" : "không thấy ô"}; «đang nghĩ» ${nghi ? `«${nghi.nghi}» lúc ${nghi.ms} ms, aria-live ${nghi.liveNghi}` : "không thấy"}; lỗi ${loi ? `«${loi.loi}» lúc ${loi.ms} ms, aria-live ${loi.liveLoi}` : "không thấy sau 10 s"}; câu hỏi còn ${mau.at(-1)?.hoi ? "có" : "không"}` });
    await t.context.close();
  }

  // ------------------------------------------------------------------ doi
  // The two-person chat of chat-0 and chat-1 (friends): the same AI as a group (owner decision
  // 2026-09-28, ADR-0046 «như nhóm»), in words for two. The commands offered on «/» and «@», the
  // chip as the room is (not ready), then as a server with a key would describe it (only
  // `chat-capabilities` rewritten) with its «Xem» sheet. Typed, never sent.
  if (chay("doi")) {
    const { cap } = await capGiua("chat-0", "chat-1");
    const CAU_DOI = "@Rủ Đi cuối tuần hai đứa đi đâu?";
    const docLenh = (page) =>
      page.evaluate(() =>
        [...document.querySelectorAll('[role="button"]')]
          .filter((b) => b.getClientRects().length && /^(\/plan|\/vote|\/chia-bill|@Rủ Đi)\n/.test(b.innerText ?? ""))
          .map((b) => (b.innerText ?? "").replace(/\s+/g, " ").trim()),
      );
    for (const cfg of ["C1", "C2", "C3"]) {
      if (!chayPhan("doi", cfg)) continue;
      const t = await mo(cfg, "chat-0", `/groups/${cap.id}/chat`);
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(3000);
      await choOn(t.page, { mang: t.mang });
      if (cfg === "C1") {
        await goSoan(t, cdp, "/");
        const gach = await docLenh(t.page);
        await goSoan(t, cdp, "@");
        const a = await docLenh(t.page);
        const lenh = [...gach, ...a];
        const tenNhom = lenh.filter((l) => /nhóm|cả hội/.test(l));
        ghi({ tc: "TC-N22-DOI-LENH", screen: "N22.S02", state: "chat đôi chat-0/chat-1 (đám bạn), máy chủ không có khoá", action: "gõ «/», rồi «@», đọc các lệnh gợi ý", cauHinh: "C1", expected: "đúng các lệnh của nhóm (/plan, /vote, /chia-bill, @Rủ Đi): cặp có AI như nhóm (quyết định 2026-09-28); lời mô tả nói về hai bạn, không «nhóm», không «cả hội»", status: ["/plan", "/vote", "/chia-bill", "@Rủ Đi"].every((x) => lenh.some((l) => l.startsWith(`${x} `))) && !tenNhom.length ? "PASS" : "FAIL", ghiChu: `«/»: ${gach.map((l) => `«${l}»`).join(", ") || "không lệnh"}; «@»: ${a.map((l) => `«${l}»`).join(", ") || "không lệnh"}${tenNhom.length ? `; nói «nhóm»: ${tenNhom.join(" | ")}` : ""}` });
      }
      await goSoan(t, cdp, CAU_DOI);
      const chip = await docChip(t.page);
      await anh(t.page, `EV-N22-DOI-CHIP-${cfg}`, t, chip?.chip ? { chuThich: [{ rect: { x: chip.chip.x, y: chip.chip.y, w: chip.chip.w, h: chip.chip.h } }] } : {});
      ghi({ tc: "TC-N22-DOI-CHIP-CHUA-SAN-SANG", screen: "N22.S02", state: "chat đôi chat-0/chat-1; máy chủ báo provider_unavailable", action: `gõ «${CAU_DOI}» vào ô soạn (không gửi)`, cauHinh: cfg, expected: "chip trên nút gửi nói Rủ Đi AI chưa sẵn sàng và tin đi như tin thường, như ở nhóm; chip, ô soạn và nút gửi trong cửa sổ", status: chip && /chưa sẵn sàng/.test(chip.cau) && /tin thường/.test(chip.cau) && !chip.xem && chip.gui && chip.gui.y + chip.gui.h <= chip.cuaSo.h ? "PASS" : "FAIL", evidence: [`EV-N22-DOI-CHIP-${cfg}`], ghiChu: chip ? `chip «${chip.cau}» ${chip.chip.w}×${chip.chip.h} ở y ${chip.chip.y}, aria-live ${chip.live}; ô soạn ${chip.oSoan ? `y ${chip.oSoan.y}` : "-"}, nút gửi ${chip.gui ? `${chip.gui.y}–${chip.gui.y + chip.gui.h}` : "-"} (cửa sổ ${chip.cuaSo.w}×${chip.cuaSo.h})` : "không thấy chip" });
      await t.context.close();
    }
    if (chayPhan("doi", "san-sang")) {
      // The pair's thread as the server holds it: the seed left every pair empty, and an empty
      // thread has its own chip (nothing to attach), so the ready chip with «Xem» is read on
      // the lab board (`doi-lab`) rather than by writing messages here.
      const { pa } = await capGiua("chat-0", "chat-1");
      const soTin = ((await goi("GET", `/contexts/${cap.id}/messages?limit=100`, undefined, pa)).json?.messages ?? []).length;
      const t = await mo("C1", "chat-0", "/profile");
      const cdp = await cdpCua(t.page);
      await dungSanSang(t.page, cap.id);
      await diToi(t, `/groups/${cap.id}/chat`);
      await goSoan(t, cdp, CAU_DOI);
      const chip = await docChip(t.page);
      await anh(t.page, "EV-N22-DOI-SAN-C1", t);
      ghi({ tc: "TC-N22-DOI-SAN-TRONG", screen: "N22.S02", state: `chat đôi chat-0/chat-1 như máy chủ có khoá (chỉ chat-capabilities viết lại ở trình duyệt); luồng có ${soTin} tin`, action: `gõ «${CAU_DOI}» (không gửi)`, cauHinh: "C1", expected: "luồng trống: chip nói hai bạn chưa có tin nào nên chỉ gửi lời nhờ (lời của hai người, không «nhóm»); không «Xem», không nút đổi", status: soTin === 0 ? (chip && /^Hai bạn chưa có tin nào/.test(chip.cau) && !chip.xem && !chip.doi ? "PASS" : "FAIL") : "BLOCKED", evidence: ["EV-N22-DOI-SAN-C1"], ghiChu: `luồng ${soTin} tin; chip «${chip?.cau ?? "-"}»; «Xem» ${chip?.xem ? "có" : "không"}; nút đổi ${chip?.doi ? `«${chip.doi.chu}»` : "không"}` });
      await t.context.close();
    }
  }

  // ------------------------------------------------------------- doi-lab
  // The pair's chip with something to attach, and its «Xem» sheet in words for two, on the
  // N26 lab board /dev/hai-lop-chat (dev server, fixture on): the shipped ChipBoiCanh and
  // TamXemBoiCanh with an invented three-message bundle.
  if (chay("doi-lab")) {
    if (!process.env.AUDIT_BASE) throw new Error("doi-lab cần AUDIT_BASE trỏ server dev có fixture");
    const t = await mo("C1", null, "/dev/hai-lop-chat");
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(4000);
    const chips = await t.page.evaluate(() =>
      [...document.querySelectorAll('[data-testid="lab-chip"] [data-testid="chat-chip-boi-canh"]')].map((c) => (c.innerText ?? "").replace(/[\uE000-\uF8FF]/g, "").replace(/\s+/g, " ").trim()),
    );
    const mo1 = await t.page.evaluate(() => {
      const e = document.querySelector('[data-testid="lab-chip"] [data-testid="chat-boi-canh-mo"]');
      if (!e) return null;
      e.scrollIntoView({ block: "center", behavior: "instant" });
      const r = e.getBoundingClientRect();
      return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
    });
    let xem = null;
    if (mo1) {
      await cham(cdp, mo1);
      await t.page.waitForTimeout(1200);
      xem = await t.page.evaluate(() => {
        const tam = [...document.querySelectorAll('[data-testid="chat-boi-canh-tam"]')].find((x) => x.getClientRects().length);
        const luot = tam?.querySelector('[data-testid="chat-boi-canh-luot"]');
        const chu = (luot?.innerText ?? "").split("\n").map((s) => s.trim()).filter(Boolean);
        return { co: !!tam, dau: chu.slice(0, 2), muc: [...(tam?.querySelectorAll('[data-testid="chat-boi-canh-muc"]') ?? [])].length };
      });
    }
    await anh(t.page, "EV-N22-DOI-LAB-C1", t);
    const n = Number((chips[0]?.match(/Kèm (\d+) tin/) ?? [])[1] ?? NaN);
    const loiNhom = (xem?.dau ?? []).filter((s) => /nhóm|thành viên|cả hội/.test(s));
    ghi({ tc: "TC-N22-DOI-LAB-XEM", screen: "N22.S02", layer: "L-xem-boi-canh", state: "trang lab /dev/hai-lop-chat (server dev, fixture bật), mục «Chip trên nút gửi · hai người», gói bịa 3 tin", action: "đọc ba chip; chạm «Xem» của chip sẵn sàng", cauHinh: "C1", expected: "chip sẵn sàng «Kèm N tin gần đây · Xem · Chỉ gửi lời nhờ»; tấm «Xem» liệt kê đúng N tin, nói bằng lời của hai người («…hiện cho cả hai bạn», «Tên hiển thị của hai bạn»), không «nhóm», không «các thành viên» (thiết kế 2026-09-28)", status: Number.isFinite(n) && xem?.co && xem.muc === n && xem.dau.some((s) => /cả hai bạn/.test(s)) && !loiNhom.length ? "PASS" : "FAIL", evidence: ["EV-N22-DOI-LAB-C1"], ghiChu: `ba chip: ${chips.map((c) => `«${c}»`).join(", ") || "không thấy"}; tấm ${xem?.co ? "mở" : "không mở"}, ${xem?.muc ?? 0} tin (chip đếm ${Number.isFinite(n) ? n : "?"}); hai câu đầu: ${(xem?.dau ?? []).map((s) => `«${s}»`).join(" ")}${loiNhom.length ? `; nói «nhóm»: ${loiNhom.join(" | ")}` : ""}` });
    await t.context.close();
  }

  // ------------------------------------------------------------- xem-ghim
  // The «Xem» sheet against the chat's pinned band. The band carries `zIndex: 1` and the Sheet
  // none (UI-070), and the Sheet's 82% cap leaves its handle row out (UI-040), so a sheet this
  // tall rises under the band. Read the boxes, what paints on top (the capture), and what a tap
  // where the hidden «Đóng bảng» sits lands on: hit-testing skips inert layers exactly as a tap
  // does (lesson of F11), so it is read, not tapped. Staged ready as in `san-sang`; nothing is sent.
  if (chay("xem-ghim")) {
    for (const cfg of ["C1", "C2", "C8"]) {
      if (!chayPhan("xem-ghim", cfg)) continue;
      const t = await mo(cfg, "chat-0", "/profile");
      const cdp = await cdpCua(t.page);
      await dungSanSang(t.page, G);
      await diToi(t, `/groups/${G}/chat`);
      await goSoan(t, cdp, CAU_KEM);
      const chip = await docChip(t.page);
      let d = null;
      if (chip?.xem) {
        await cham(cdp, { x: chip.xem.x + chip.xem.w / 2, y: chip.xem.y + chip.xem.h / 2 });
        await t.page.waitForTimeout(1200);
        d = await t.page.evaluate(() => {
          const hien = (e) => e && e.getClientRects().length > 0;
          const hop = (e) => {
            if (!e) return null;
            const r = e.getBoundingClientRect();
            return { x: Math.round(r.left), y: Math.round(r.top), w: Math.round(r.width), h: Math.round(r.height), day: Math.round(r.bottom) };
          };
          const ghim = [...document.querySelectorAll('[data-testid="day-ghim"]')].find(hien) ?? null;
          const tam = [...document.querySelectorAll('[data-testid="chat-boi-canh-tam"]')].find(hien) ?? null;
          const panel = tam ? [...tam.querySelectorAll('[role="dialog"]')].find(hien) ?? null : null;
          const dong = panel?.querySelector('[aria-label="Đóng bảng"]') ?? null;
          const tay = panel?.querySelector('[aria-label="Tay cầm"]') ?? null;
          let cau = null;
          if (panel) {
            const w = document.createTreeWalker(panel, NodeFilter.SHOW_TEXT);
            while (w.nextNode()) {
              if (/Rủ Đi AI sẽ đọc đúng/.test(w.currentNode.nodeValue)) {
                cau = w.currentNode;
                break;
              }
            }
          }
          // The first line of the sheet's first sentence, by a Range over its first words.
          let dongDau = null;
          if (cau) {
            const r = document.createRange();
            r.setStart(cau, 0);
            r.setEnd(cau, Math.min(20, cau.length));
            const b = r.getClientRects()[0];
            if (b) dongDau = { x: Math.round(b.left), y: Math.round(b.top), w: Math.round(b.width), h: Math.round(b.height) };
          }
          const trung = (p) => {
            if (!p) return null;
            const e = document.elementFromPoint(p.x, p.y);
            if (!e) return "không gì";
            if (ghim?.contains(e)) return "dải ghim";
            if (dong?.contains(e)) return "«Đóng bảng»";
            if (panel?.contains(e)) return "tấm Xem";
            return e.getAttribute("aria-label") || e.tagName.toLowerCase();
          };
          const bDong = hop(dong);
          return {
            ghim: hop(ghim),
            panel: hop(panel),
            dong: bDong,
            tay: hop(tay),
            cau: dongDau,
            chamDong: bDong ? trung({ x: bDong.x + bDong.w / 2, y: bDong.y + bDong.h / 2 }) : null,
            chamCau: dongDau ? trung({ x: dongDau.x + 6, y: dongDau.y + dongDau.h / 2 }) : null,
            ghimInert: !!ghim?.closest("[inert]"),
            zGhim: ghim ? getComputedStyle(ghim).zIndex : null,
            zTam: tam ? getComputedStyle(tam).zIndex : null,
            cuaSo: { w: innerWidth, h: innerHeight },
          };
        });
      }
      await anh(t.page, `EV-N22-XEM-GHIM-${cfg}`, t, d?.ghim ? { chuThich: [{ rect: { x: d.ghim.x, y: d.ghim.y, w: d.ghim.w, h: d.ghim.h } }] } : {});
      const chong = d?.ghim && d?.panel ? Math.max(0, Math.min(d.ghim.day, d.panel.day) - Math.max(d.ghim.y, d.panel.y)) : null;
      const biChe = (b) => (b && d?.ghim ? b.y < d.ghim.day && b.y + b.h > d.ghim.y : false);
      ghi({ tc: "TC-N22-XEM-GHIM", screen: "N22.S01", layer: "L-xem-boi-canh", state: "nhóm chat-test có «Tờ hẹn chung» ghim trên đầu luồng; nhóm sẵn sàng (dựng), chip «Kèm N tin gần đây»", action: "chạm «Xem»; đọc khung của tấm, của dải ghim, của «Đóng bảng» và câu đầu", cauHinh: cfg, expected: "tấm «Những tin sẽ gửi kèm lời nhờ» phủ lên mọi thứ của màn, kể cả dải ghim: tay cầm, «Đóng bảng» và câu đầu «Rủ Đi AI sẽ đọc đúng N tin…» thấy được; tấm ≤ 82% chiều cao (DESIGN.md)", status: d?.ghim && d?.panel ? (chong > 0 && d.zGhim === "1" && d.zTam !== "1" ? "FAIL" : "PASS") : "BLOCKED", evidence: [`EV-N22-XEM-GHIM-${cfg}`], ghiChu: d ? `tấm y ${d.panel?.y}–${d.panel?.day} (${d.panel ? Math.round((100 * d.panel.h) / d.cuaSo.h) : "?"}% cửa sổ ${d.cuaSo.w}×${d.cuaSo.h}); dải ghim y ${d.ghim?.y}–${d.ghim?.day}, z-index ${d.zGhim} (lớp tấm ${d.zTam}), inert ${d.ghimInert ? "có" : "không"}; chồng ${chong}px; «Đóng bảng» y ${d.dong?.y}–${d.dong?.day} ${biChe(d.dong) ? "DƯỚI dải" : "ngoài dải"}; tay cầm y ${d.tay?.y}–${d.tay?.day} ${biChe(d.tay) ? "DƯỚI dải" : "ngoài dải"}; dòng đầu «Rủ Đi AI sẽ đọc đúng…» y ${d.cau?.y ?? "?"} ${biChe(d.cau) ? "DƯỚI dải" : "ngoài dải"}; chạm vào chỗ «Đóng bảng» trúng ${d.chamDong}; chạm vào dòng đầu trúng ${d.chamCau}` : `không mở được «Xem» (chip ${chip ? `«${chip.cau}»` : "không thấy"})` });
      await t.context.close();
    }
  }

  // ------------------------------------------------------------------ lab
  // /dev/tra-loi-song on the dev server (fixture on): every state of a streamed answer, drawn by
  // the shipped components from the screens' own reducer. Invented words, no server.
  if (chay("lab")) {
    if (!process.env.AUDIT_BASE) throw new Error("lab cần AUDIT_BASE trỏ server dev có fixture");
    const docLab = (page) =>
      page.evaluate(() => {
        const chu = (e) => (e?.innerText ?? "").replace(/[-]/g, "").replace(/\s+/g, " ").trim();
        const khoi = (id) => document.querySelector(`[data-testid="${id}"]`);
        const live = (sel) => [...document.querySelectorAll(sel)].map((e) => e.closest("[aria-live]")?.getAttribute("aria-live") ?? null);
        return {
          dangNghi: chu(khoi("lab-nep-dang-nghi")),
          dangViet: chu(khoi("lab-nep-dang-viet")),
          xong: chu(khoi("lab-nep-xong")),
          nhom: chu(khoi("lab-nhom")),
          phong: chu(khoi("lab-phong")),
          live: { nepNghi: live('[data-testid="nep-dang-nghi"]'), nepViet: live('[data-testid="nep-dang-viet"]'), nhomDoc: live('[data-testid="chat-tra-loi-dang-doc"]'), nhomViet: live('[data-testid="chat-tra-loi-dang-viet"]') },
          tieuDe: chu(document.querySelector('[role="heading"]')) || null,
        };
      });
    for (const cfg of ["C1", "C2", "C3"]) {
      if (!chayPhan("lab", cfg)) continue;
      const t = await mo(cfg, null, "/dev/tra-loi-song");
      await t.page.waitForTimeout(4000);
      const den = await duongDan(t.page);
      const l = await docLab(t.page);
      const cap = await anh(t.page, `EV-N22-LAB-${cfg}`, t, { fullPage: true });
      const tt = cap?.tomTat ?? {};
      ghi({ tc: "TC-N22-LAB-BASE", screen: "N22.S04", state: "trang lab /dev/tra-loi-song (server dev, fixture bật), dữ liệu bịa", action: "mở trang, đọc mọi trạng thái", cauHinh: cfg, expected: "bảng Nếp: đang nghĩ, đang viết (chữ lớn dần cùng kiểu chữ câu trả lời), xong (chip dưới câu trả lời); luồng nhóm: người hỏi đang đọc, đang viết; thành viên khác cùng hàng đó. Không tràn, không cắt chữ", status: /dev\/tra-loi-song/.test(den ?? "") && l.dangNghi && l.dangViet && l.xong && l.nhom && l.phong && tt.tranTrangPx === 0 && tt.chuBiCat === 0 ? "PASS" : "FAIL", evidence: [`EV-N22-LAB-${cfg}`], ghiChu: `tới ${den}; đang nghĩ «${l.dangNghi.slice(0, 80)}»; đang viết «${l.dangViet.slice(0, 90)}»; xong «${l.xong.slice(0, 60)}…»; nhóm «${l.nhom.slice(0, 160)}»; phòng «${l.phong.slice(0, 120)}»; tràn ${tt.tranTrangPx ?? "?"}px, cắt chữ ${tt.chuBiCat ?? "?"}, vùng bấm < 48: ${tt.vungBamNho48 ?? "?"} (< 44: ${tt.vungBamNho44 ?? "?"})` });
      if (cfg === "C1") {
        ghi({ tc: "TC-N22-LAB-LIVE", screen: "N22.S04", state: "trang lab, các hàng đang nghĩ, đang đọc, đang viết", action: "đọc vùng thông báo (aria-live) quanh chữ đang hiện dần", cauHinh: "C1", expected: "chữ đang hiện dần, và câu «đang nghĩ / đang đọc», nằm trong một vùng aria-live (polite) để trình đọc màn hình báo câu trả lời tới; như chip và câu lỗi của chat đã làm", status: [...l.live.nepNghi, ...l.live.nepViet, ...l.live.nhomDoc, ...l.live.nhomViet].every((x) => x === "polite" || x === "assertive") && l.live.nepViet.length ? "PASS" : "FAIL", evidence: ["EV-N22-LAB-C1"], ghiChu: `aria-live: Nếp đang nghĩ ${JSON.stringify(l.live.nepNghi)}, Nếp đang viết ${JSON.stringify(l.live.nepViet)}, nhóm đang đọc ${JSON.stringify(l.live.nhomDoc)}, nhóm đang viết ${JSON.stringify(l.live.nhomViet)}` });
        // «Chạy thử»: the answer replayed at the server's pace; then once more under «Reduce Motion».
        const cdp = await cdpCua(t.page);
        const chayThu = async (giam) => {
          if (giam) await bam(t, cdp, "Reduce Motion", { cho: 400 });
          await t.page.evaluate(() => scrollTo(0, 0));
          const p = await nut(t.page, "Chạy thử");
          if (!p) return null;
          await cham(cdp, p);
          const bd = Date.now();
          const mau = [];
          for (let i = 0; i < 160; i++) {
            const s = await t.page.evaluate(() => {
              const k = document.querySelector('[data-testid="lab-nep-chay"]');
              if (!k) return { co: false };
              return { co: true, viet: k.querySelector('[data-testid="nep-dang-viet"]')?.innerText ?? null, nghi: !!k.querySelector('[data-testid="nep-dang-nghi"]'), xong: !!k.querySelector('[data-testid="nep-tra-loi"]') };
            });
            mau.push({ ms: Date.now() - bd, ...s });
            if (s.xong) break;
            await t.page.waitForTimeout(30);
          }
          return mau;
        };
        const thuong = await chayThu(false);
        await anh(t.page, "EV-N22-LAB-CHAY-C1", t);
        await t.page.reload();
        await t.page.waitForTimeout(3500);
        const giam = await chayThu(true);
        const tom = (m) => {
          if (!m) return null;
          const dai = m.map((x) => (x.viet ?? "").length);
          const doi = [];
          for (let i = 0; i < m.length; i++) if (i === 0 || dai[i] !== dai[i - 1]) doi.push({ ms: m[i].ms, n: dai[i] });
          const dauTien = m.find((x) => (x.viet ?? "").length > 0);
          const xong = m.find((x) => x.xong);
          return { buoc: doi.filter((d) => d.n > 0), dauTien: dauTien?.ms ?? null, xong: xong?.ms ?? null, nghi: m.some((x) => x.nghi) };
        };
        const a = tom(thuong);
        const b = tom(giam);
        const cauLaTron = (b?.buoc ?? []).every((d) => {
          const s = (giam.find((x) => x.ms === d.ms)?.viet ?? "").trim();
          return /[.!?…]$/.test(s);
        });
        ghi({ tc: "TC-N22-LAB-CHAY", screen: "N22.S04", state: "trang lab, «Chạy thử» (16 ký tự mỗi 25 ms như máy chủ)", action: "«Chạy thử»; rồi tải lại, bật «Reduce Motion», «Chạy thử»", cauHinh: "C1", expected: "thường: «đang nghĩ», rồi chữ lớn dần từng nhịp tới hết, xong thành lượt trả lời có chip; Reduce Motion: chữ tới từng câu trọn, không từng nhịp (NepPhien.tsx)", status: a && b && a.nghi && a.buoc.length >= 4 && a.xong !== null && b.xong !== null && b.buoc.length >= 1 && b.buoc.length <= 2 && cauLaTron ? "PASS" : "FAIL", evidence: ["EV-N22-LAB-CHAY-C1"], ghiChu: `thường: đang nghĩ ${a?.nghi ? "có" : "không"}, chữ đầu lúc ${a?.dauTien ?? "-"} ms, ${a?.buoc.length ?? 0} lần lớn lên (${(a?.buoc ?? []).slice(0, 6).map((d) => `${d.n}@${d.ms}`).join(", ")}…), xong lúc ${a?.xong ?? "-"} ms; Reduce Motion: ${b?.buoc.length ?? 0} lần lớn lên (${(b?.buoc ?? []).map((d) => `${d.n}@${d.ms}`).join(", ")}), mỗi lần ${cauLaTron ? "là câu trọn" : "KHÔNG phải câu trọn"}, xong lúc ${b?.xong ?? "-"} ms` });
      }
      await t.context.close();
    }
  }

  // ------------------------------------------------------------- lab-nhom
  // The lower half of the lab board: the group thread's row while the AI reads, then writes, as
  // the asker and as another member see it. react-native-web scrolls an inner view, so the
  // full-page capture of `lab` stops at the fold; this scrolls that view to the group sections.
  if (chay("lab-nhom")) {
    if (!process.env.AUDIT_BASE) throw new Error("lab-nhom cần AUDIT_BASE trỏ server dev có fixture");
    for (const cfg of ["C1", "C2", "C3"]) {
      if (!chayPhan("lab-nhom", cfg)) continue;
      const t = await mo(cfg, null, "/dev/tra-loi-song");
      await t.page.waitForTimeout(4000);
      const vt = await t.page.evaluate(() => {
        const chu = (e) => (e?.innerText ?? "").replace(/[\uE000-\uF8FF]/g, "").replace(/\s+/g, " ").trim();
        const w = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
        let dau = null;
        while (w.nextNode()) if (w.currentNode.nodeValue.trim() === "Luồng nhóm · người hỏi") dau = w.currentNode.parentElement;
        const nhom = document.querySelector('[data-testid="lab-nhom"]');
        const phong = document.querySelector('[data-testid="lab-phong"]');
        if (!dau || !nhom || !phong) return null;
        let k = dau.parentElement;
        while (k && !(k.scrollHeight > k.clientHeight + 4 && /auto|scroll/.test(getComputedStyle(k).overflowY))) k = k.parentElement;
        if (!k) return null;
        k.scrollTop += dau.getBoundingClientRect().top - 120;
        const r = (e) => {
          const b = e.getBoundingClientRect();
          return { y: Math.round(b.top), day: Math.round(b.bottom), w: Math.round(b.width) };
        };
        return { nhom: chu(nhom), phong: chu(phong), rDau: r(dau), rNhom: r(nhom), rPhong: r(phong), cao: innerHeight };
      });
      await t.page.waitForTimeout(400);
      const cap = await anh(t.page, `EV-N22-LAB-NHOM-${cfg}`, t);
      const tt = cap?.tomTat ?? {};
      ghi({ tc: "TC-N22-LAB-NHOM-BASE", screen: "N22.S04", state: "trang lab /dev/tra-loi-song, nửa dưới: luồng nhóm (server dev, fixture bật), dữ liệu bịa", action: "cuộn tới «Luồng nhóm · người hỏi», đọc và chụp", cauHinh: cfg, expected: "hàng của Rủ Đi AI trong luồng nhóm: người hỏi thấy «đang đọc» rồi chữ đang viết; thành viên khác thấy cùng hàng đó; đọc được (sáng, tối, 320), không tràn, không cắt chữ", status: vt && vt.nhom && vt.phong && tt.tranTrangPx === 0 && tt.chuBiCat === 0 ? "PASS" : "FAIL", evidence: [`EV-N22-LAB-NHOM-${cfg}`], ghiChu: vt ? `người hỏi «${vt.nhom.slice(0, 160)}»; thành viên khác «${vt.phong.slice(0, 120)}»; tiêu đề ở y ${vt.rDau.y}, khối người hỏi y ${vt.rNhom.y}–${vt.rNhom.day}, khối thành viên khác y ${vt.rPhong.y}–${vt.rPhong.day} (cửa sổ ${vt.cao}), rộng ${vt.rNhom.w}; tràn ${tt.tranTrangPx ?? "?"}px, cắt chữ ${tt.chuBiCat ?? "?"}` : "không thấy hai khối luồng nhóm" });
      await t.context.close();
    }
  }

  // ------------------------------------------------------------- lab-prod
  if (chay("lab-prod")) {
    if (process.env.AUDIT_BASE) throw new Error("lab-prod chạy trên bản export production: bỏ AUDIT_BASE");
    const t = await mo("C1", null, "/dev/tra-loi-song");
    await t.page.waitForTimeout(2500);
    const den = await duongDan(t.page);
    ghi({ tc: "TC-N22-LAB-PROD", screen: "N22.S04", state: "bản export production (E1)", action: "mở /dev/tra-loi-song", cauHinh: "C1", expected: "bản production không có trang lab: chuyển về /welcome", status: /^\/welcome/.test(den ?? "") ? "PASS" : "FAIL", ghiChu: `tới ${den}` });
    await t.context.close();
  }
} finally {
  await mt.dong();
}
