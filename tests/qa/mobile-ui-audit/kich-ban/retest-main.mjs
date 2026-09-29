/* Retest of the audit's issues on a newer build of main (task #16).
 *
 * Every issue of the 7ea1a7c audit gets one row, `TC-R-UI-xxx`, whose
 * expected line is the issue's own «Tiêu chí gỡ»: PASS means the criterion now
 * holds (the defect is gone), FAIL means it does not. The note starts with
 * «còn:», «hết:» or «đổi:» and says what was measured, including where the way
 * to reproduce had to change because main's UI moved (for example the create
 * tray, which main no longer offers from the tab bar once there are five tabs).
 *
 *   source audit-env-main.sh && node kich-ban/retest-main.mjs [--chi r-f00,…]
 *
 * Runs against the stack and web build the environment names (AUDIT_API,
 * AUDIT_WEB); the ledger is AUDIT_OUT's, so the base audit's is never touched.
 */
import { cauHinh } from "../thu-vien/cau-hinh.mjs";
import { chup } from "../thu-vien/chup.mjs";
import { cdpCua, cham, keo, nhanGiu, nhip, tamCua } from "../thu-vien/cu-chi.mjs";
import { choOn, duongDan } from "../thu-vien/dieu-huong.mjs";
import { choDialog, demDialog, dong, inertConLai } from "../thu-vien/lop-phu.mjs";
import { goHet, loiMayChu } from "../thu-vien/mang.mjs";
import { khoiDong, trangMoi } from "../thu-vien/moi-truong.mjs";
import { goiApi, layPhien, personaTheoTen } from "../thu-vien/phien.mjs";
import { soGhi } from "../thu-vien/ghi.mjs";
import { pngThuBytes } from "../../../../apps/mobile/tools/png-thu.mjs";
import { randomUUID } from "node:crypto";
import { existsSync } from "node:fs";
import { join } from "node:path";

const chi = (() => {
  const i = process.argv.indexOf("--chi");
  return i === -1 ? null : new Set(process.argv[i + 1].split(","));
})();
const chay = (ten) => !chi || chi.has(ten);
// A section's parts run alone with `--chi r-f08:094`; the section's own setup runs for any of them.
const coPhan = (ten) => !chi || chi.has(ten) || [...chi].some((x) => x.startsWith(`${ten}:`));
const chayPhan = (ten, phan) => !chi || chi.has(ten) || chi.has(`${ten}:${phan}`);
const mt = await khoiDong();
const so = soGhi(mt.out);
// An issue whose criterion has parts measured apart gets one row per part: TC-R-UI-084-A, -B.
// A finding new on main (not an issue of the base audit) is a TC-M row.
const ket = ({ phan, moi, ...rec }) => {
  const tc = `TC-${moi ? "M" : "R"}-${rec.issue}${phan ? `-${phan}` : ""}`;
  so.ghi({ nenTang: "web", method: "RUNTIME-WEB", layer: "-", ...rec, tc });
  console.log(`${rec.status.padEnd(10)} ${tc} ${rec.cauHinh ?? ""} ${rec.ghiChu ?? ""}`);
};
const P = (ten) => personaTheoTen(ten, mt.chatSessions);
const phienDir = join(mt.out, "phien");
const mo = (cfg, persona, path) => trangMoi(mt, cauHinh(cfg), { persona: persona ? P(persona) : null, path });
const anh = (page, id, t) => chup(page, { out: mt.out, id, suKien: t.suKien });

/** A control by its accessible name or visible text; the centre, scrolled into view vertically. */
const timNut = (page, chu, { batDau = false } = {}) =>
  page.evaluate(
    ({ chu, batDau }) => {
      const ten = (x) => (x.getAttribute("aria-label") || (x.innerText ?? "")).replace(/[-]/g, "").replace(/\s+/g, " ").trim();
      const e = [...document.querySelectorAll('[role="button"],button,[role="link"],a[href],[role="tab"]')].find((x) => (batDau ? ten(x).startsWith(chu) : ten(x) === chu) && x.getBoundingClientRect().width > 0);
      if (!e) return null;
      e.scrollIntoView({ block: "center", behavior: "instant" });
      for (let n = e.parentElement; n; n = n.parentElement) if (n.scrollLeft && !/(auto|scroll)/.test(getComputedStyle(n).overflowX)) n.scrollLeft = 0;
      const r = e.getBoundingClientRect();
      return { x: r.left + r.width / 2, y: r.top + r.height / 2, w: Math.round(r.width), h: Math.round(r.height), ten: ten(e) };
    },
    { chu, batDau },
  );
/** Inert or aria-hidden regions outside dialogs covering at least a quarter of the window (UI-005's measure). */
const vungKhoaLon = (page) => inertConLai(page);
const api = (method, path, body, phien) => goiApi(mt.api, method, path, body, phien.token);
/** A write with its own Idempotency-Key, answering the status and the body. */
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
const anId = (s) => String(s ?? "").replace(/[0-9a-f]{8}-[0-9a-f-]{27}/g, "[id]");
const phienCua = (ten) => layPhien(mt.api, P(ten), phienDir);
/** Inside the app, the way a tap would get there: the stack keeps its history. */
const diToi = async (t, path) => {
  await t.page.evaluate((p) => {
    history.pushState({}, "", p);
    dispatchEvent(new PopStateEvent("popstate"));
  }, path);
  await t.page.waitForTimeout(2500);
  await choOn(t.page, { mang: t.mang });
};
const chuTrang = (page) => page.evaluate(() => (document.body.innerText ?? "").replace(/\s+/g, " "));

try {
  // ------------------------------------------------------------------ F00
  if (chay("r-f00")) {
    // UI-002: an unknown URL while signed in.
    {
      const t = await mo("C1", "dalat-0", "/khong-co-trang-nay");
      await t.page.waitForTimeout(1500);
      await choOn(t.page, { mang: t.mang });
      const den = await duongDan(t.page);
      await anh(t.page, "EV-R-UI-002-C1", t);
      const veTab = /^\/(community|explore|plan|messages|profile)/.test(den);
      const loi = await t.page.evaluate(() => (document.body.innerText ?? "").replace(/\s+/g, " ").slice(0, 160));
      ket({ feature: "F00", issue: "UI-002", screen: "F00.S02", state: "đã đăng nhập (dalat-0)", action: "mở /khong-co-trang-nay", cauHinh: "C1", expected: "URL lạ khi có phiên thì dẫn về tab (hoặc trang lỗi có lối về)", status: veTab ? "PASS" : "FAIL", evidence: ["EV-R-UI-002-C1"], ghiChu: `${veTab ? "hết" : "còn"}: tới ${den}; chữ đầu trang «${loi.slice(0, 80)}»` });
      await t.context.close();
    }
    // UI-003: the selected tab's ARIA state.
    {
      const t = await mo("C1", "dalat-0", "/explore");
      await t.page.waitForTimeout(1200);
      const tabs = await t.page.evaluate(() => [...document.querySelectorAll('[role="tab"]')].filter((e) => e.getClientRects().length).map((e) => ({ ten: e.getAttribute("aria-label"), chon: e.getAttribute("aria-selected") })));
      const dung = tabs.filter((x) => x.chon === "true").map((x) => x.ten);
      ket({ feature: "F00", issue: "UI-003", screen: "F00.S03", layer: "L05", state: "đã đăng nhập, ở Khám phá", action: "đọc ARIA của các phần tử role=tab", cauHinh: "C1", expected: "tab đang chọn có aria-selected=true", status: dung.length === 1 && dung[0] === "Khám phá" ? "PASS" : "FAIL", evidence: [], ghiChu: `${dung.length === 1 && dung[0] === "Khám phá" ? "hết" : "còn"}: ${tabs.length} tab (${tabs.map((x) => `${x.ten}=${x.chon ?? "null"}`).join(", ")})` });
      await t.context.close();
    }
    // UI-004: the rail indicator against the selected tab, every tab, two widths.
    {
      const hang = [];
      for (const cfg of ["C6", "C7"]) {
        for (const path of ["/community", "/explore", "/plan", "/messages", "/profile"]) {
          const t = await mo(cfg, "dalat-0", path);
          await t.page.waitForTimeout(1400);
          await choOn(t.page, { mang: t.mang });
          const d = await t.page.evaluate((path) => {
            const tabs = [...document.querySelectorAll('[role="tab"]')].filter((e) => e.getClientRects().length);
            const ten = { "/community": "Cộng đồng", "/explore": "Khám phá", "/plan": "Lên plan", "/messages": "Tin nhắn", "/profile": "Cá nhân" }[path];
            const tab = tabs.find((e) => e.getAttribute("aria-label") === ten);
            const cha = tab?.parentElement;
            const vach = cha && [...cha.children].find((c) => getComputedStyle(c).position === "absolute" && Math.round(c.getBoundingClientRect().width) === 4);
            if (!tab || !vach) return { ten, tab: !!tab, vach: !!vach };
            const a = tab.getBoundingClientRect();
            const b = vach.getBoundingClientRect();
            const giua = b.top + b.height / 2;
            return { ten, tabY: [Math.round(a.top), Math.round(a.bottom)], vachY: [Math.round(b.top), Math.round(b.bottom)], trung: giua >= a.top && giua <= a.bottom };
          }, path);
          if (cfg === "C6" && path === "/profile") await anh(t.page, "EV-R-UI-004-C6", t);
          hang.push({ cfg, ...d });
          await t.context.close();
        }
      }
      const lech = hang.filter((h) => !h.trung);
      ket({ feature: "F00", issue: "UI-004", screen: "F00.S03", layer: "L05", state: "rail ở cửa sổ rộng", action: "mở từng tab ở C6 và C7, đo vạch chỉ báo", cauHinh: "C6,C7", expected: "tâm vạch nằm trong khoảng dọc của tab đang chọn, ở mọi tab", status: lech.length === 0 ? "PASS" : "FAIL", evidence: ["EV-R-UI-004-C6"], ghiChu: `${lech.length === 0 ? "hết" : "còn"}: ${hang.length - lech.length}/${hang.length} đúng; lệch: ${lech.map((h) => `${h.cfg} ${h.ten} tab ${h.tabY?.join("–") ?? "?"} vạch ${h.vachY?.join("–") ?? (h.vach ? "?" : "không thấy")}`).join("; ") || "-"}` });
    }
    // UI-005 and UI-006: the create tray, which main opens from «Tạo mới» in Lên plan.
    {
      const t = await mo("C1", "dalat-0", "/plan");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(1500);
      await choOn(t.page, { mang: t.mang });
      const truocMo = await vungKhoaLon(t.page);
      const nut = await timNut(t.page, "Tạo mới");
      if (nut) await cham(cdp, nut);
      const moRa = await choDialog(t.page, { co: true });
      await t.page.waitForTimeout(700);
      const d = await dong(t.page, cdp, "back");
      await t.page.waitForTimeout(1200);
      await choOn(t.page, { mang: t.mang, choThem: 200 });
      const sau = await vungKhoaLon(t.page);
      const den = await duongDan(t.page);
      await anh(t.page, "EV-R-UI-005-C1", t);
      const coNut = !!nut;
      ket({ feature: "F00", issue: "UI-005", screen: "F00.S04", layer: "L01", state: "khay tạo mở từ «Tạo mới» của Lên plan", action: "Back trình duyệt khi khay mở, rồi đọc cây truy cập", cauHinh: "C1", expected: "0 phần tử inert/aria-hidden phủ ≥25% màn sau Back", status: coNut && moRa.ok && sau.length <= truocMo.length ? "PASS" : "FAIL", evidence: ["EV-R-UI-005-C1"], ghiChu: `${!coNut ? "đổi: không thấy nút «Tạo mới» ở Lên plan" : sau.length <= truocMo.length ? "hết" : "còn"}: thanh tab không còn nút «Tạo mới» (5 tab); khay mở từ Lên plan: ${moRa.ok ? "có" : "không"}; Back ${d.lam ? "làm" : "không làm"}, tới ${den}; vùng khoá ≥25% màn: trước ${truocMo.length}, sau ${sau.length}` });
      await t.context.close();

      const t2 = await mo("C1", "dalat-0", "/plan");
      const cdp2 = await cdpCua(t2.page);
      await t2.page.waitForTimeout(1500);
      await choOn(t2.page, { mang: t2.mang });
      const nut2 = await timNut(t2.page, "Tạo mới");
      let duoiNgon = null;
      if (nut2) {
        await cham(cdp2, nut2, 20);
        await t2.page.waitForTimeout(60);
        // What the second tap will land on, 60 ms after the first (hit tests skip inert nodes, as taps do).
        duoiNgon = await t2.page.evaluate(({ x, y }) => {
          const e = document.elementFromPoint(x, y);
          const nhan = e?.closest("[aria-label]");
          return nhan ? nhan.getAttribute("aria-label") : e?.tagName ?? null;
        }, nut2);
        await cham(cdp2, nut2, 20).catch(() => undefined);
      }
      await t2.page.waitForTimeout(1200);
      const n = await demDialog(t2.page);
      const den2 = await duongDan(t2.page);
      await anh(t2.page, "EV-R-UI-006-C1", t2);
      const dat = nut2 && n === 1 && den2 === "/create";
      // UI-006 was the second tap activating a row of the open tray; a tray that opens and closes again is a different failure.
      const doi = nut2 && n === 0 && /Đóng/.test(duoiNgon ?? "");
      ket({ feature: "F00", issue: "UI-006", screen: "F00.S04", layer: "L01", state: "Lên plan, khay đóng", action: "chạm «Tạo mới» hai lần cách 60 ms", cauHinh: "C1", expected: "đúng 1 dialog, URL vẫn /create", status: dat ? "PASS" : "FAIL", evidence: ["EV-R-UI-006-C1"], ghiChu: `${!nut2 ? "đổi: không thấy nút" : dat ? "hết" : doi ? "đổi" : "còn"}: nút «+» của thanh tab không còn, chạm nút «Tạo mới» của Lên plan; 60 ms sau chạm đầu, dưới ngón là «${duoiNgon ?? "-"}»; sau hai chạm ${n} dialog, URL ${den2}` });
      await t2.context.close();
    }
    // UI-011: «Vẽ» in Nếp's panel.
    {
      const t = await mo("C1", "dalat-0", "/explore");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(1200);
      const mep = await tamCua(t.page, '[data-testid="nep-mep"]', { cuon: false });
      let moBang = { ok: false };
      if (mep) {
        const vw = await t.page.evaluate(() => innerWidth);
        await cham(cdp, { x: vw - 5, y: mep.y });
        await t.page.waitForTimeout(600);
        const dia = await tamCua(t.page, '[data-testid="nep-dia"]', { cuon: false });
        if (dia) {
          await cham(cdp, dia);
          moBang = await choDialog(t.page, { co: true });
        }
      }
      let ghi = "không mở được bảng Nếp";
      let conLoi = false;
      if (moBang.ok) {
        await t.page.locator('[data-testid="nep-o-nhap"]').fill("một con mèo đội nón lá").catch(() => undefined);
        await t.page.waitForTimeout(300);
        const yeuCau = [];
        t.page.on("request", (r) => { if (!r.url().includes("openfreemap")) yeuCau.push(`${r.method()} ${new URL(r.url()).pathname}`); });
        const dialogs = [];
        t.page.on("dialog", (dl) => { dialogs.push(dl.message()); dl.dismiss().catch(() => undefined); });
        const nutVe = await t.page.evaluate(() => {
          const d = [...document.querySelectorAll('[role="dialog"]')].pop();
          const b = d && [...d.querySelectorAll('[role="button"],button')].find((e) => (e.innerText ?? "").trim() === "Vẽ" || e.getAttribute("aria-label") === "Vẽ");
          if (!b) return null;
          const r = b.getBoundingClientRect();
          return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
        });
        const truoc = await demDialog(t.page);
        const chuTruoc = await t.page.evaluate(() => [...document.querySelectorAll('[role="dialog"]')].pop()?.innerText ?? "");
        if (nutVe) await cham(cdp, nutVe);
        await t.page.waitForTimeout(2000);
        const sauN = await demDialog(t.page);
        const chuSau = await t.page.evaluate(() => [...document.querySelectorAll('[role="dialog"]')].pop()?.innerText ?? "");
        conLoi = !!nutVe && sauN === truoc && dialogs.length === 0 && yeuCau.length === 0 && chuSau === chuTruoc;
        ghi = `nút «Vẽ» ${nutVe ? "có" : "không thấy"}; dialog mới ${sauN - truoc}; alert ${dialogs.length}; request ${yeuCau.length}; chữ bảng đổi: ${chuSau !== chuTruoc ? "có" : "không"}`;
      }
      await anh(t.page, "EV-R-UI-011-C1", t);
      ket({ feature: "F00", issue: "UI-011", screen: "F00.S05", layer: "L03", state: "bảng Nếp, đã gõ mô tả", action: "bấm «Vẽ»", cauHinh: "C1", expected: "web: bấm «Vẽ» luôn cho một phản hồi nhìn thấy được", status: moBang.ok && !conLoi ? "PASS" : "FAIL", evidence: ["EV-R-UI-011-C1"], ghiChu: `${!moBang.ok ? "đổi" : conLoi ? "còn" : "hết"}: ${ghi}` });
      await t.context.close();
    }
  }

  // ------------------------------------------------------------------ F01
  if (chay("r-f01")) {
    // UI-016: the Welcome pager's dots after each swipe.
    {
      const t = await mo("C1", null, "/welcome");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(1500);
      const trangThai = () =>
        t.page.evaluate(() => {
          const dots = [...document.querySelectorAll("[aria-label]")].find((e) => /^Trang \d+ trên \d+$/.test(e.getAttribute("aria-label")));
          const sv = [...document.querySelectorAll("div")].find((e) => {
            const s = getComputedStyle(e);
            return (s.overflowX === "auto" || s.overflowX === "scroll") && e.scrollWidth > e.clientWidth + 10;
          });
          return { dots: dots?.getAttribute("aria-label") ?? null, scrollLeft: sv ? Math.round(sv.scrollLeft) : null, rong: sv ? sv.clientWidth : null };
        });
      const buoc = [await trangThai()];
      const tong = Number(buoc[0].dots?.match(/trên (\d+)/)?.[1] ?? 0);
      for (let i = 1; i < Math.max(2, Math.min(tong, 4)); i++) {
        await keo(cdp, { x: 340, y: 560 }, { x: 60, y: 565 }, { ms: 260, buoc: 10 });
        await t.page.waitForTimeout(900);
        buoc.push(await trangThai());
      }
      await anh(t.page, "EV-R-UI-016-C1", t);
      const coPager = buoc[0].scrollLeft !== null && buoc[0].dots !== null;
      const khop = coPager && buoc.every((b, i) => b.scrollLeft !== null && Math.abs(b.scrollLeft - i * b.rong) <= 2 && b.dots === `Trang ${i + 1} trên ${tong}`);
      ket({ feature: "F01", issue: "UI-016", screen: "F01.S01", layer: "L30", state: "Welcome trang 1", action: "vuốt trái trên đoạn chữ, đọc chấm trang sau mỗi lần", cauHinh: "C1", expected: "sau mỗi lần vuốt, chấm, mốc và nhãn khớp scrollLeft / pageWidth", status: khop ? "PASS" : "FAIL", evidence: ["EV-R-UI-016-C1"], ghiChu: `${!coPager ? "đổi" : khop ? "hết" : "còn"}: ${buoc.map((b) => `${b.dots ?? "không chấm"} @${b.scrollLeft ?? "-"}/${b.rong ?? "-"}`).join(" → ")}` });
      await t.context.close();
    }
    // UI-018: «Quay lại» on screens opened cold by a link.
    {
      const pa = await layPhien(mt.api, P("dalat-0"), phienDir);
      const nhom = ((await goiApi(mt.api, "GET", "/people/me/contexts", undefined, pa.token)).contexts ?? []).find((c) => c.kind === "group");
      const keo1 = nhom ? ((await goiApi(mt.api, "GET", `/contexts/${nhom.id}/outings`, undefined, pa.token)).outings ?? [])[0] : null;
      const ca = [["/login", null], ["/places/p-lung-chung-cafe", "dalat-0"], ...(keo1 ? [[`/outings/${keo1.id}`, "dalat-0"]] : [])];
      const kq = [];
      for (const [path, persona] of ca) {
        const t = await mo("C1", persona, path);
        const cdp = await cdpCua(t.page);
        await t.page.waitForTimeout(1500);
        await choOn(t.page, { mang: t.mang });
        const truoc = await duongDan(t.page);
        const nut = await tamCua(t.page, '[aria-label="Quay lại"]');
        if (nut) await cham(cdp, nut);
        await t.page.waitForTimeout(1200);
        const sau = await duongDan(t.page);
        if (path === "/login") await anh(t.page, "EV-R-UI-018-C1", t);
        kq.push({ path: path.replace(/[0-9a-f]{8}-[0-9a-f-]{27}/g, "[id]"), nut: !!nut, truoc: truoc.replace(/[0-9a-f]{8}-[0-9a-f-]{27}/g, "[id]"), sau: sau.replace(/[0-9a-f]{8}-[0-9a-f-]{27}/g, "[id]") });
        await t.context.close();
      }
      const dung = kq.filter((k) => k.nut && k.sau !== k.truoc);
      ket({ feature: "F01", issue: "UI-018", screen: "F01.S02", state: "màn mở thẳng bằng link, không có lịch sử", action: "chạm «Quay lại»", cauHinh: "C1", expected: "mở lạnh rồi chạm «Quay lại» luôn đi tới một màn", status: dung.length === kq.length ? "PASS" : "FAIL", evidence: ["EV-R-UI-018-C1"], ghiChu: `${dung.length === kq.length ? "hết" : dung.length ? "đổi" : "còn"}: ${kq.map((k) => `${k.path}: ${k.nut ? `${k.truoc} → ${k.sau}` : "không thấy nút «Quay lại»"}`).join("; ")}` });
    }
    // UI-019: a wrong invitation code.
    {
      const t = await mo("C1", null, "/moi");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(1200);
      const o = t.page.locator('input[aria-label="Mã lời mời"]');
      const coO = (await o.count()) > 0;
      if (coO) await o.fill("khong-phai-ma-that");
      const nut = await t.page.evaluate(() => {
        const b = [...document.querySelectorAll('[role="button"],button')].find((e) => (e.innerText ?? "").includes("Nhận lời mời"));
        const r = b?.getBoundingClientRect();
        return r ? { x: r.left + r.width / 2, y: r.top + r.height / 2 } : null;
      });
      if (nut) await cham(cdp, nut);
      await t.page.waitForTimeout(1800);
      const chu = await t.page.evaluate(() => (document.body.innerText ?? "").replace(/\s+/g, " "));
      const cau = chu.match(/[^.«»]*(mã|cập nhật|Cập nhật|lời mời)[^.]*\./g)?.slice(-3) ?? [];
      const nhacCapNhat = /cập nhật app|Cập nhật app|cập nhật ứng dụng/i.test(chu);
      await anh(t.page, "EV-R-UI-019-C1", t);
      ket({ feature: "F01", issue: "UI-019", screen: "F01.S04", state: "mã lời mời sai", action: "dán «khong-phai-ma-that», «Nhận lời mời»", cauHinh: "C1", expected: "mã sai hiện câu về mã, không nhắc cập nhật app", status: coO && nut && !nhacCapNhat ? "PASS" : "FAIL", evidence: ["EV-R-UI-019-C1"], ghiChu: `${!coO || !nut ? "đổi: không thấy ô mã hoặc nút" : nhacCapNhat ? "còn" : "hết"}: ${cau.join(" | ").slice(0, 240)}` });
      await t.context.close();
    }
  }

  // ------------------------------------------------------------------ F02
  if (chay("r-f02")) {
    // Leaf text nodes whose content matches, with how many pixels they miss.
    const doCat = (page, mau) =>
      page.evaluate((mauChu) => {
        const re = new RegExp(mauChu);
        return [...document.querySelectorAll("div[dir]")]
          .filter((e) => re.test(e.textContent ?? "") && e.children.length === 0 && e.getBoundingClientRect().width > 0)
          .map((e) => ({ chu: e.textContent, thieu: Math.round(e.scrollWidth - e.clientWidth), rong: Math.round(e.getBoundingClientRect().width) }));
      }, mau.source);
    // UI-021: the price line of every Explore row.
    {
      const kq = [];
      for (const cfg of ["C1", "C2", "C3", "C4", "C5", "C6"]) {
        const t = await mo(cfg, "dalat-0", "/explore");
        await t.page.waitForTimeout(1800);
        await choOn(t.page, { mang: t.mang });
        const dong = await doCat(t.page, /mỗi người/);
        kq.push({ cfg, tong: dong.length, cat: dong.filter((d) => d.thieu > 1).length });
        if (cfg === "C1") await anh(t.page, "EV-R-UI-021-C1", t);
        await t.context.close();
      }
      const tongCat = kq.reduce((a, k) => a + k.cat, 0);
      ket({ feature: "F02", issue: "UI-021", screen: "F02.S01", state: "Khám phá Đà Lạt, có phiên", action: "đọc dòng «điểm · km · giá» của mỗi hàng", cauHinh: "C1–C6", expected: "0 dòng chứa giá bị cắt ở C1–C6", status: kq.every((k) => k.tong > 0) && tongCat === 0 ? "PASS" : "FAIL", evidence: ["EV-R-UI-021-C1"], ghiChu: `${kq.some((k) => k.tong === 0) ? "đổi: có cấu hình không thấy dòng «mỗi người»" : tongCat === 0 ? "hết" : "còn"}: ${kq.map((k) => `${k.cfg} ${k.cat}/${k.tong} bị cắt`).join(", ")}` });
    }
    // UI-022 and UI-023: the detail of Tiệm Nướng Xóm Lào, reached from the list.
    {
      const t = await mo("C1", "dalat-0", "/explore");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(1500);
      const the = await tamCua(t.page, '[aria-label="Mở Tiệm Nướng Xóm Lào"]');
      if (the) await cham(cdp, the);
      await choOn(t.page, { mang: t.mang });
      const chiTiet = await duongDan(t.page);
      await t.page.evaluate(() => {
        const goc = window.open;
        window.__audit_open = [];
        window.open = function (...a) {
          let tra = null;
          let loi = null;
          try {
            tra = goc.apply(this, a);
          } catch (e) {
            loi = String(e);
          }
          window.__audit_open.push({ url: String(a[0]), tra: tra === null ? "null" : typeof tra, loi });
          if (loi) throw new Error(loi);
          return tra;
        };
      });
      const popup = t.context.waitForEvent("page", { timeout: 4000 }).catch(() => null);
      const nut = await timNut(t.page, "Chỉ đường");
      if (nut) await cham(cdp, nut);
      await t.page.waitForTimeout(1500);
      const trangMoiMo = await popup;
      const goi = await t.page.evaluate(() => window.__audit_open ?? []);
      const cau = await t.page.evaluate(() => (document.body.innerText ?? "").match(/[^.\n]*(bản đồ|chỉ đường|Chỉ đường)[^.\n]*\./g)?.slice(0, 2) ?? []);
      await anh(t.page, "EV-R-UI-022-C1", t);
      const urlMoi = trangMoiMo ? trangMoiMo.url() : null;
      const dat = !!nut && ((urlMoi && urlMoi !== "about:blank") || goi.some((g) => /^https?:/.test(g.url) && g.tra !== "null") || cau.length > 0);
      const anDanh = (u) => u.replace(/([?&](query|q|destination)=)[^&]*/g, "$1…");
      ket({ feature: "F02", issue: "UI-022", screen: "F02.S03", layer: "L33", state: "chi tiết Tiệm Nướng Xóm Lào", action: "chạm «Chỉ đường»", cauHinh: "C1", expected: "web: chạm «Chỉ đường» mở trang mới hoặc hiện câu", status: dat ? "PASS" : "FAIL", evidence: ["EV-R-UI-022-C1"], ghiChu: `${!nut ? "đổi: không thấy nút" : dat ? "hết" : "còn"}: chi tiết ở ${chiTiet}; window.open ${goi.map((g) => `${anDanh(g.url).slice(0, 90)} → ${g.tra}${g.loi ? ` (${g.loi})` : ""}`).join(", ") || "không gọi"}; trang mới ${urlMoi ? anDanh(urlMoi).slice(0, 90) : "không"}; câu: ${cau.join(" | ") || "không"}` });
      await t.context.close();
      if (trangMoiMo) await trangMoiMo.close().catch(() => undefined);

      const kq = [];
      for (const cfg of ["C1", "C2"]) {
        const u = await mo(cfg, "dalat-0", chiTiet);
        await u.page.waitForTimeout(1500);
        await choOn(u.page, { mang: u.mang });
        const nhan = await doCat(u.page, /^(Lưu địa điểm|Đã lưu|Lưu)$/);
        kq.push({ cfg, nhan: nhan.map((d) => `«${d.chu}» rộng ${d.rong} thiếu ${d.thieu}`) });
        if (cfg === "C2") await anh(u.page, "EV-R-UI-023-C2", u);
        await u.context.close();
      }
      const cat = kq.flatMap((k) => k.nhan).filter((s) => !/thiếu (0|1|-\d+)$/.test(s));
      ket({ feature: "F02", issue: "UI-023", screen: "F02.S03", state: "chi tiết Tiệm Nướng Xóm Lào", action: "đọc nhãn nút ở chân trang", cauHinh: "C1,C2", expected: "nhãn «Lưu địa điểm» đọc trọn ở C2", status: kq.every((k) => k.nhan.length) && cat.length === 0 ? "PASS" : "FAIL", evidence: ["EV-R-UI-023-C2"], ghiChu: `${kq.some((k) => !k.nhan.length) ? "đổi: không thấy nhãn lưu" : cat.length ? "còn" : "hết"}: ${kq.map((k) => `${k.cfg} ${k.nhan.join(", ") || "-"}`).join("; ")}` });
    }
    // UI-024: the ✦ button before any question is sent.
    {
      const t = await mo("C1", "dalat-0", "/explore");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(1500);
      const nut = await tamCua(t.page, '[aria-label="Hỏi Rủ Đi AI"]');
      if (nut) await cham(cdp, nut);
      await t.page.waitForTimeout(1200);
      const s = await t.page.evaluate(() => {
        const o = document.querySelector('input[aria-label="Ô tìm địa điểm"]');
        const chu = (document.body.innerText ?? "").replace(/\s+/g, " ");
        return { o: o?.value ?? null, focus: document.activeElement === o, khong: /\b0 kết quả|Chưa thấy nơi phù hợp/.test(chu), dau: chu.match(/\d+ (kết quả|nơi ở [^.]+?)(?= )/)?.[0] ?? null };
      });
      await anh(t.page, "EV-R-UI-024-C1", t);
      ket({ feature: "F02", issue: "UI-024", screen: "F02.S01", state: "Khám phá, máy chủ không có khoá AI", action: "chạm ✦ «Hỏi Rủ Đi AI»", cauHinh: "C1", expected: "chạm ✦ không hiện «0 kết quả» trước khi có câu trả lời", status: nut && !s.khong ? "PASS" : "FAIL", evidence: ["EV-R-UI-024-C1"], ghiChu: `${!nut ? "đổi: không thấy nút ✦" : s.khong ? "còn" : "hết"}: ô «${(s.o ?? "").slice(0, 60)}», focus vào ô: ${s.focus ? "có" : "không"}; tiêu đề danh sách «${s.dau ?? "-"}»; «0 kết quả/Chưa thấy nơi phù hợp»: ${s.khong ? "có" : "không"}` });
      await t.context.close();
    }
  }

  // ------------------------------------------------------------------ F03
  if (chay("r-f03")) {
    const pa = await layPhien(mt.api, P("dalat-0"), phienDir);
    const nhomDl = ((await goiApi(mt.api, "GET", "/people/me/contexts", undefined, pa.token)).contexts ?? []).find((c) => c.kind === "group");
    const keoGoc = ((await goiApi(mt.api, "GET", `/contexts/${nhomDl.id}/outings`, undefined, pa.token)).outings ?? []).find((k) => k.title === "Đà Lạt cuối tuần");
    const chonChe = async (page, cdp, ten) => {
      const n = await timNut(page, ten);
      if (n) await cham(cdp, n);
      await page.waitForTimeout(900);
      return !!n;
    };
    // UI-032: a three-day outing whose three stops carry a place and no day.
    {
      const t = await mo("C1", "dalat-0", `/outings/${keoGoc.id}`);
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(1500);
      await chonChe(t.page, cdp, "Bản đồ");
      await choOn(t.page, { mang: t.mang });
      const ngay = await t.page.evaluate(() => [...document.querySelectorAll('[role="button"],[role="radio"],[role="tab"]')].map((e) => (e.innerText ?? "").replace(/[-]/g, "").trim()).filter((x) => /^Ngày \d+$/.test(x)));
      const theoNgay = [];
      for (const d of ngay.length ? [...new Set(ngay)] : ["(một ngày)"]) {
        if (ngay.length) await chonChe(t.page, cdp, d);
        theoNgay.push({ d, moc: await t.page.evaluate(() => document.querySelectorAll('[aria-label^="Mốc "]').length), rong: await t.page.evaluate(() => /chưa có điểm nào/.test(document.body.innerText ?? "")) });
      }
      const loiGiai = await t.page.evaluate(() => /chưa xếp ngày|chưa có ngày|chưa chọn ngày/i.test(document.body.innerText ?? ""));
      await anh(t.page, "EV-R-UI-032-C1", t);
      const tong = theoNgay.reduce((a, b) => a + b.moc, 0);
      const coQuan = keoGoc.stops.filter((s) => s.place_id).length;
      const dat = tong === coQuan || loiGiai;
      ket({ feature: "F03", issue: "UI-032", screen: "F03.S04", state: `kèo 3 ngày, ${keoGoc.stops.length} chặng gắn quán, chặng không có ngày`, action: "mở kèo, chọn «Bản đồ», lần lượt từng ngày", cauHinh: "C1", expected: "tổng mốc qua các ngày = số chặng gắn quán, hoặc có dòng nói rõ các chặng chưa xếp ngày", status: dat ? "PASS" : "FAIL", evidence: ["EV-R-UI-032-C1"], ghiChu: `${dat ? "hết" : "còn"}: ${theoNgay.map((x) => `${x.d}: ${x.moc} mốc${x.rong ? ", «chưa có điểm nào»" : ""}`).join("; ")}; câu về chặng chưa xếp ngày: ${loiGiai ? "có" : "không"}` });
      await t.context.close();
    }
    // UI-033: a two-day outing with no stop, on the map, at the heights the issue named.
    {
      const pc = await layPhien(mt.api, P("chat-0"), phienDir);
      const nhomChat = ((await goiApi(mt.api, "GET", "/people/me/contexts", undefined, pc.token)).contexts ?? []).find((c) => c.kind === "group");
      const TEN_RONG = "Kèo retest 2 ngày chưa có chặng";
      let rong = ((await goiApi(mt.api, "GET", `/contexts/${nhomChat.id}/outings`, undefined, pc.token)).outings ?? []).find((k) => k.title === TEN_RONG);
      if (!rong) rong = await goiApi(mt.api, "POST", `/contexts/${nhomChat.id}/outings`, { title: TEN_RONG, starts_on: "2026-10-24", ends_on: "2026-10-25", headcount: 8, budget_per_person_vnd: 450000 }, pc.token);
      const kq = [];
      for (const cfg of ["C1", "C2", "C3", "C4", "C8"]) {
        const t = await mo(cfg, "chat-0", `/outings/${rong.id}`);
        const cdp = await cdpCua(t.page);
        await t.page.waitForTimeout(1500);
        await chonChe(t.page, cdp, "Bản đồ");
        await choOn(t.page, { mang: t.mang });
        const ve = await t.page.evaluate(() => {
          const e = [...document.querySelectorAll('[aria-label="Về Lịch trình"]')].find((x) => x.getBoundingClientRect().width > 0);
          if (!e) return null;
          const r = e.getBoundingClientRect();
          let cuon = null;
          for (let n = e.parentElement; n; n = n.parentElement) {
            const s = getComputedStyle(n);
            if ((s.overflowY === "auto" || s.overflowY === "scroll") && n.scrollHeight > n.clientHeight + 1) {
              cuon = n;
              break;
            }
          }
          const c = cuon?.getBoundingClientRect();
          const khung = c ? { top: Math.max(0, c.top), bottom: Math.min(innerHeight, c.bottom) } : { top: 0, bottom: innerHeight };
          const thay = Math.max(0, Math.min(r.bottom, khung.bottom) - Math.max(r.top, khung.top));
          return { phanThay: Math.round((100 * thay) / r.height) };
        });
        if (cfg === "C1") await anh(t.page, "EV-R-UI-033-C1", t);
        kq.push({ cfg, phanThay: ve ? ve.phanThay : null });
        await t.context.close();
      }
      const dat = kq.every((k) => k.phanThay === 100);
      ket({ feature: "F03", issue: "UI-033", screen: "F03.S04", layer: "L13", state: "kèo 2 ngày chưa có chặng (dựng qua API trong nhóm chat-test)", action: "chọn «Bản đồ»", cauHinh: "C1,C2,C3,C4,C8", expected: "«Về Lịch trình» thấy trọn không cần cuộn ở C1–C4 và C8", status: dat ? "PASS" : "FAIL", evidence: ["EV-R-UI-033-C1"], ghiChu: `${kq.some((k) => k.phanThay === null) ? "đổi: có cấu hình không thấy «Về Lịch trình»" : dat ? "hết" : "còn"}: ${kq.map((k) => `${k.cfg} ${k.phanThay ?? "-"}% thấy`).join(", ")}` });
    }
    // UI-034: creating an outing with an empty budget.
    {
      const pc = await layPhien(mt.api, P("chat-0"), phienDir);
      const nhomChat = ((await goiApi(mt.api, "GET", "/people/me/contexts", undefined, pc.token)).contexts ?? []).find((c) => c.kind === "group");
      const dem = async () => ((await goiApi(mt.api, "GET", `/contexts/${nhomChat.id}/outings`, undefined, pc.token)).outings ?? []).length;
      const truoc = await dem();
      const t = await mo("C1", "chat-0", `/outings/new?contextId=${nhomChat.id}`);
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(1500);
      const o = t.page.locator('[aria-label="Ô tên kèo"]').first();
      const coO = (await o.count()) > 0;
      if (coO) {
        await o.click();
        await o.fill("Kèo retest ngân sách trống");
      }
      const ns = await t.page.evaluate(() => {
        const e = [...document.querySelectorAll("input")].find((x) => /ngân sách|mỗi người|Ngân sách/i.test(x.getAttribute("aria-label") ?? ""));
        return e ? { nhan: e.getAttribute("aria-label"), placeholder: e.getAttribute("placeholder"), value: e.value } : null;
      });
      const nut = await timNut(t.page, "Tạo kèo");
      if (nut) await cham(cdp, nut);
      await t.page.waitForTimeout(2500);
      const den = await duongDan(t.page);
      const cauTrongMan = await t.page.evaluate(() =>
        [...document.querySelectorAll("div[dir],span")]
          .filter((e) => e.children.length === 0 && /ngân sách|mỗi người|Chọn một mức|mức chi/i.test(e.textContent ?? ""))
          .map((e) => ({ chu: e.textContent.trim().slice(0, 80), y: Math.round(e.getBoundingClientRect().top), trong: e.getBoundingClientRect().top >= 0 && e.getBoundingClientRect().bottom <= innerHeight })),
      );
      const sau = await dem();
      await anh(t.page, "EV-R-UI-034-C1", t);
      const taoDuoc = sau > truoc;
      const cauLoiThay = cauTrongMan.some((c) => c.trong && /Chọn|chưa|cần/i.test(c.chu));
      ket({ feature: "F03", issue: "UI-034", screen: "F03.S02", state: "form tạo kèo, tên đã gõ, ngân sách bỏ trống", action: "chạm «Tạo kèo»", cauHinh: "C1", expected: "bỏ trống ngân sách rồi «Tạo kèo»: câu lỗi thấy được ngay (hoặc kèo được tạo)", status: taoDuoc || cauLoiThay ? "PASS" : "FAIL", evidence: ["EV-R-UI-034-C1"], ghiChu: `${!coO || !nut ? "đổi: không thấy ô tên hoặc nút" : taoDuoc ? "hết (kèo tạo được khi bỏ trống ngân sách)" : cauLoiThay ? "hết" : "còn"}: ô ngân sách ${ns ? `«${ns.nhan}», placeholder «${ns.placeholder ?? ""}», giá trị «${ns.value}»` : "không thấy"}; sau chạm ở ${den.replace(/[0-9a-f]{8}-[0-9a-f-]{27}/g, "[id]")}; kèo trên máy chủ ${truoc} → ${sau}; câu liên quan: ${cauTrongMan.map((c) => `«${c.chu}» y ${c.y}${c.trong ? "" : " (ngoài màn)"}`).join(", ") || "không"}` });
      await t.context.close();
    }
    // UI-035: the demo timeline route with a session.
    {
      const t = await mo("C1", "dalat-0", `/trips/${keoGoc.id}/timeline`);
      await t.page.waitForTimeout(2000);
      await choOn(t.page, { mang: t.mang });
      const den = await duongDan(t.page);
      const demo = await t.page.evaluate(() => /Dữ liệu demo|Demo|Khởi hành từ TP\.HCM/.test(document.body.innerText ?? ""));
      await anh(t.page, "EV-R-UI-035-C1", t);
      const dat = den === `/outings/${keoGoc.id}`;
      ket({ feature: "F03", issue: "UI-035", screen: "F03", state: "có phiên (dalat-0)", action: "mở /trips/<id kèo>/timeline", cauHinh: "C1", expected: "có phiên: /trips/<id>/timeline về /outings/<id>", status: dat ? "PASS" : "FAIL", evidence: ["EV-R-UI-035-C1"], ghiChu: `${dat ? "hết" : "còn"}: tới ${den.replace(keoGoc.id, "[id]")}; dữ liệu demo trên màn: ${demo ? "có" : "không"}` });
      await t.context.close();
    }
    // UI-036: reordering a stop with the keyboard only (a draft; nothing is saved).
    {
      const t = await mo("C1", "dalat-0", `/outings/${keoGoc.id}`);
      await t.page.waitForTimeout(1500);
      await choOn(t.page, { mang: t.mang });
      const thuTu = () => t.page.evaluate(() => [...document.querySelectorAll('[aria-label^="Chặng "][role="button"]')].filter((e) => e.getBoundingClientRect().height > 0).map((e) => e.getAttribute("aria-label").slice(6)));
      const truoc = await thuTu();
      const tay = await t.page.evaluate(() => {
        const e = document.querySelector('[aria-label="Thứ tự Cà phê sáng"]');
        e?.focus();
        return { co: !!e, focus: document.activeElement === e, role: e?.getAttribute("role"), tabindex: e?.getAttribute("tabindex") };
      });
      await t.page.keyboard.press("ArrowDown");
      await t.page.waitForTimeout(700);
      const sau = await thuTu();
      const doi = JSON.stringify(truoc) !== JSON.stringify(sau);
      await anh(t.page, "EV-R-UI-036-C1", t);
      if (doi) {
        const bo = await timNut(t.page, "Bỏ thứ tự nháp");
        if (bo) await cham(await cdpCua(t.page), bo);
      }
      ket({ feature: "F03", issue: "UI-036", screen: "F03.S03", state: `kèo 3 chặng, thứ tự ${truoc.join(" → ")}`, action: "focus tay nắm «Thứ tự Cà phê sáng», nhấn mũi tên xuống", cauHinh: "C1", expected: "chỉ bằng bàn phím đổi được thứ tự một chặng", status: doi ? "PASS" : "FAIL", evidence: ["EV-R-UI-036-C1"], ghiChu: `${!tay.co ? "đổi: không thấy tay nắm" : doi ? "hết" : "còn"}: tay nắm role ${tay.role ?? "-"}, tabindex ${tay.tabindex ?? "-"}, nhận focus: ${tay.focus ? "có" : "không"}; sau mũi tên xuống: ${sau.join(" → ")}` });
      await t.context.close();
    }
  }

  // ------------------------------------------------------------------ F06
  if (chay("r-f06")) {
    const coChu = (page, chu) => page.evaluate((c) => (document.body.innerText ?? "").includes(c), chu);
    const go = async (page, nhan, chu) => {
      const o = page.locator(`[aria-label="${nhan}"]`).first();
      if (!(await o.count())) return false;
      await o.click();
      await o.fill(chu);
      return true;
    };
    const bam = async (t, cdp, chu, cho = 900) => {
      const n = await timNut(t.page, chu);
      if (n) await cham(cdp, n);
      await t.page.waitForTimeout(cho);
      await choOn(t.page, { mang: t.mang });
      return !!n;
    };
    const p51 = await layPhien(mt.api, P("moi-51"), phienDir);
    const nhom = ((await goiApi(mt.api, "GET", "/people/me/contexts", undefined, p51.token)).contexts ?? []).find((c) => c.kind === "group" && /^Nhóm kiểm thử F06 hội săn mây/.test(c.display_name ?? ""));
    if (!nhom) throw new Error("chưa có nhóm F06 của moi-51 trên stack này; chạy f06-nhom-nguoi.mjs --chi tao-nhom trước");
    const R = "moi-61";
    const TEN_DAT = "Bạn cũ tên do người mời đặt R61";
    const TEN_TU = "Tên tự chọn R61";
    const thanhVien = async () => (await goiApi(mt.api, "GET", `/contexts/${nhom.id}/members`, undefined, p51.token)).members ?? [];

    // UI-073: an invited number signs in for the first time.
    if (existsSync(join(phienDir, `${R}.json`))) console.log(`r-f06: ${R} đã vào cửa ở lượt trước, bỏ qua UI-073`);
    else {
      const ta = await mo("C1", "moi-51", `/groups/${nhom.id}/invite`);
      const cdpA = await cdpCua(ta.page);
      await ta.page.waitForTimeout(1500);
      await go(ta.page, "Ô tên người được mời", TEN_DAT);
      await go(ta.page, "Ô số điện thoại người được mời", P(R).phone);
      await bam(ta, cdpA, "Gửi lời mời", 2500);
      const daMoi = await coChu(ta.page, "Đã mời");
      await ta.context.close();

      const tb = await mo("C1", null, "/welcome");
      const cdpB = await cdpCua(tb.page);
      await tb.page.waitForTimeout(1500);
      const cta = await tamCua(tb.page, '[data-testid="welcome-cta"]');
      if (cta) await cham(cdpB, cta);
      await tb.page.waitForURL("**/login", { timeout: 10_000 }).catch(() => undefined);
      await tb.page.locator('input[aria-label="Ô số điện thoại"]').fill(P(R).phone);
      await cham(cdpB, await tamCua(tb.page, '[data-testid="login-gui-ma"]'));
      await tb.page.waitForURL("**/otp", { timeout: 10_000 }).catch(() => undefined);
      await tb.page.keyboard.type("000000", { delay: 40 });
      await tb.page.waitForURL((u) => !/\/otp$/.test(u.pathname), { timeout: 12_000 }).catch(() => undefined);
      await tb.page.waitForTimeout(1500);
      await choOn(tb.page, { mang: tb.mang });
      const den = await duongDan(tb.page);
      const oTen = await tb.page.locator('[aria-label="Ô tên của bạn"]').first().inputValue().catch(() => null);
      const thayTenDat = await coChu(tb.page, TEN_DAT);
      await anh(tb.page, "EV-R-UI-073-A-C1", tb);
      let tenTrongNhom = null;
      let trangThai = null;
      if (den === "/personalization") {
        await go(tb.page, "Ô tên của bạn", TEN_TU);
        for (const g of ["Ăn uống", "Cafe", "Chơi đêm"]) {
          const c = await tamCua(tb.page, `[aria-label="${g}"]`);
          if (c) await cham(cdpB, c);
          await tb.page.waitForTimeout(150);
        }
        const mc = await tamCua(tb.page, '[aria-label*="Rộng tay"]');
        if (mc) await cham(cdpB, mc);
        await tb.page.waitForTimeout(300);
        const luu = await timNut(tb.page, "Lưu sở thích", { batDau: true });
        if (luu) await cham(cdpB, luu);
        await tb.page.waitForTimeout(2500);
        await choOn(tb.page, { mang: tb.mang });
      }
      const sauSoThich = await duongDan(tb.page);
      if (!/\/messages/.test(sauSoThich)) {
        const tabTin = await tamCua(tb.page, '[role="tab"][aria-label="Tin nhắn"]');
        if (tabTin) await cham(cdpB, tabTin);
        await tb.page.waitForTimeout(1500);
      }
      const dongY = await bam(tb, cdpB, "Đồng ý vào nhóm", 3000);
      const m = (await thanhVien()).find((x) => x.display_name === TEN_TU || x.display_name === TEN_DAT);
      tenTrongNhom = m?.display_name ?? null;
      trangThai = m?.state ?? null;
      await anh(tb.page, "EV-R-UI-073-B-C1", tb);
      await tb.context.close();

      // A stranger with no group and no friendship looks the number up.
      const tc = await mo("C1", "chat-20", "/friends/add");
      const cdpC = await cdpCua(tc.page);
      await tc.page.waitForTimeout(1500);
      await go(tc.page, "Ô số điện thoại bạn", P(R).phone);
      await bam(tc, cdpC, "Tìm", 2000);
      const nguoiLaThay = (await coChu(tc.page, TEN_DAT)) ? "tên người mời đặt" : (await coChu(tc.page, TEN_TU)) ? "tên tự chọn" : "không tên nào";
      const cauTra = await tc.page.evaluate(() => (document.body.innerText ?? "").replace(/\s+/g, " ").match(/(Chưa có ai dùng|Không tìm thấy|không cho tìm|Người này|Gửi lời mời|Kết bạn)[^.]*\.?/)?.[0] ?? null);
      await anh(tc.page, "EV-R-UI-073-C-C1", tc);
      await tc.context.close();

      const vaoSoThich = den === "/personalization";
      const dienSan = oTen === TEN_DAT;
      const dat = vaoSoThich && tenTrongNhom === TEN_TU && nguoiLaThay !== "tên người mời đặt";
      ket({ feature: "F06", issue: "UI-073", screen: "F06.S03", state: `số chưa từng đăng nhập, được moi-51 mời dưới tên «${TEN_DAT}»`, action: "đăng nhập lần đầu bằng OTP qua UI; qua Sở thích với tên tự chọn; «Đồng ý vào nhóm»; một người lạ (chat-20) tra số", cauHinh: "C1", expected: "số được mời đăng nhập lần đầu: vào Sở thích, ô tên điền sẵn và sửa được; tra số không lộ tên người khác đặt khi chính chủ chưa xác nhận", status: dat ? "PASS" : "FAIL", evidence: ["EV-R-UI-073-A-C1", "EV-R-UI-073-B-C1", "EV-R-UI-073-C-C1"], ghiChu: `${dat ? (dienSan ? "hết" : "đổi") : vaoSoThich ? "đổi" : "còn"}: lời mời gửi: ${daMoi ? "có" : "không"}; vào cửa tới ${den}; ô tên «${oTen ?? "-"}» (điền sẵn tên người mời đặt: ${dienSan ? "có" : "không"}), tên người mời đặt trên màn: ${thayTenDat ? "có" : "không"}; sau Sở thích ở ${sauSoThich}; «Đồng ý vào nhóm»: ${dongY ? "có" : "không thấy"}; trên máy chủ ${trangThai ?? "không thấy"}, tên trong nhóm «${tenTrongNhom ?? "?"}»; người lạ tra số thấy ${nguoiLaThay} (${(cauTra ?? "").slice(0, 60)})` });
    }

    // UI-074: the creator steps down with a second admin in the group.
    {
      const pr = await layPhien(mt.api, P(R), phienDir);
      const { randomUUID } = await import("node:crypto");
      const datVai = async (phien, personId, role) =>
        (await fetch(`${mt.api}/contexts/${nhom.id}/members/${personId}/role`, { method: "PUT", headers: { authorization: `Bearer ${phien.token}`, "content-type": "application/json", "idempotency-key": randomUUID() }, body: JSON.stringify({ role }) })).status;
      const vaiCua = async (personId) => (await thanhVien()).find((x) => x.person_id === personId);
      const r0 = await vaiCua(pr.person_id);
      if (r0?.state === "active" && r0.role !== "admin") await datVai(p51, pr.person_id, "admin");
      const hai = (await thanhVien()).filter((x) => x.state === "active" && x.role === "admin").length;
      const t = await mo("C1", "moi-51", `/groups/${nhom.id}/members`);
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(1800);
      await choOn(t.page, { mang: t.mang });
      const nut = await t.page.evaluate(() => {
        const hang = [...document.querySelectorAll("div")].filter((d) => /\(bạn\)/.test(d.innerText ?? "") && d.querySelector('[role="button"]')).sort((a, b) => a.innerText.length - b.innerText.length)[0];
        const b = hang && [...hang.querySelectorAll('[role="button"]')].find((x) => /Bỏ quyền quản trị/.test(x.innerText ?? x.getAttribute("aria-label") ?? ""));
        if (!b) return null;
        b.scrollIntoView({ block: "center" });
        const r = b.getBoundingClientRect();
        return { x: r.left + r.width / 2, y: r.top + r.height / 2, ten: (b.getAttribute("aria-label") || b.innerText).trim() };
      });
      const truoc = (await vaiCua(p51.person_id))?.role;
      if (nut) await cham(cdp, nut);
      await t.page.waitForTimeout(1500);
      const hoi = (await demDialog(t.page)) > 0 || (await t.page.evaluate(() => /Bỏ quyền của chính bạn|chắc|không tự lấy lại/i.test(document.body.innerText ?? "")));
      const sau = (await vaiCua(p51.person_id))?.role;
      await anh(t.page, "EV-R-UI-074-C1", t);
      await t.context.close();
      if (truoc === "admin" && sau !== "admin") await datVai(pr, p51.person_id, "admin");
      const dat = !!nut && sau === "admin" && hoi;
      ket({ feature: "F06", issue: "UI-074", screen: "F06.S02", state: `nhóm F06 có ${hai} quản trị; người xem là người lập nhóm (moi-51)`, action: "Thành viên → chạm «Bỏ quyền quản trị» trên hàng «… (bạn)»", cauHinh: "C1", expected: "có bước hỏi; vai trò chỉ đổi sau khi xác nhận", status: dat ? "PASS" : "FAIL", evidence: ["EV-R-UI-074-C1"], ghiChu: `${!nut ? "đổi: không thấy nút trên hàng của mình" : dat ? "hết" : "còn"}: nút «${nut?.ten ?? "-"}»; vai trò ${truoc} → ${sau} sau một chạm; bước hỏi: ${hoi ? "có" : "không"}${truoc === "admin" && sau !== "admin" ? "; đã đặt lại vai trò qua API" : ""}` });
    }
  }

  // ------------------------------------------------------------------ F07 and E5
  // Four fresh pairs of chat-test people, each set up over the API with only
  // the step it needs, and each guarded against writing twice:
  //   chat-4/chat-5   A proposes on screen while B agrees elsewhere (UI-084 a),
  //                   then the notebook read failing (UI-083) and the cold link (UI-082);
  //   chat-6/chat-7   B agrees on screen, C1 (UI-084 b);
  //   chat-10/chat-11 the same at C9;
  //   chat-8/chat-9   made a couple («Một đôi» from both): on main only a couple
  //                   drafts and sends sheets, so the block (UI-120) and «Rủ … tới
  //                   đây» on a week whose sheet is agreed (UI-085) need one.
  if (chay("r-f07")) {
    /** chat-N shows as «Chat Test NN», counted from 01. */
    const tenHien = (ten) => `Chat Test ${String(Number(ten.split("-")[1]) + 1).padStart(2, "0")}`;
    const capCua = async (phien, tenB) => ((await api("GET", "/people/me/contexts", undefined, phien)).contexts ?? []).find((c) => c.kind === "pair" && (c.display_name === tenB || c.counterpart?.display_name === tenB));
    /** Friends and an open pair for two chat-test people, over the API; the pair as A sees it. */
    const lapCap = async (tenA, tenB) => {
      const pa = await phienCua(tenA);
      const pb = await phienCua(tenB);
      let cap = await capCua(pa, tenHien(tenB));
      if (!cap) {
        await ghiApi("POST", "/friends/requests", { addressee_id: pb.person_id }, pa);
        const den = await api("GET", `/people/${pb.person_id}/friend-requests?direction=incoming`, undefined, pb);
        const lm = (den.requests ?? den.friend_requests ?? []).find((x) => x.state === "pending");
        if (lm) await ghiApi("POST", `/friends/requests/${lm.id}/respond`, { decision: "accept" }, pb);
        await ghiApi("POST", `/people/${pb.person_id}/dm`, undefined, pa);
        cap = await capCua(pa, tenHien(tenB));
      }
      if (!cap) throw new Error(`không mở được chat đôi ${tenA}/${tenB}`);
      return { cap, pa, pb };
    };
    const soCua = (cap, phien) => api("GET", `/contexts/${cap.id}/notebook`, undefined, phien);
    const daDongY = async (cap, phien, purpose) => (await soCua(cap, phien)).my_consents?.find?.((c) => c.purpose === purpose)?.granted === true;
    /** A proposes and B agrees, over the API; nothing when both already have. */
    const haiDongY = async ({ cap, pa, pb }, purpose) => {
      if ((await daDongY(cap, pa, purpose)) && (await daDongY(cap, pb, purpose))) return "đã có";
      let dn = ((await soCua(cap, pb)).pending_proposals ?? []).find((d) => d.purpose === purpose);
      if (!dn) {
        await ghiApi("POST", `/contexts/${cap.id}/notebook/proposals`, { purpose }, pa);
        dn = ((await soCua(cap, pb)).pending_proposals ?? []).find((d) => d.purpose === purpose);
      }
      return dn ? (await ghiApi("POST", `/contexts/${cap.id}/notebook/proposals/${dn.id}/grant`, undefined, pb)).status : "không có lời đề nghị";
    };
    const toCua = async (cap, phien) => (await api("GET", `/contexts/${cap.id}/papers`, undefined, phien)).papers ?? [];
    const hienTestId = (page, id) => page.evaluate((id) => [...document.querySelectorAll(`[data-testid="${id}"]`)].some((e) => e.getClientRects().length > 0), id);
    /** A button inside the top open dialog, by its visible text. */
    const trongDialog = (page, re) =>
      page.evaluate((s) => {
        const d = [...document.querySelectorAll('[role="dialog"]')].filter((x) => x.getClientRects().length).pop();
        const b = d && [...d.querySelectorAll('[role="button"]')].find((x) => new RegExp(s).test((x.innerText ?? "").trim()));
        if (!b) return null;
        const r = b.getBoundingClientRect();
        return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
      }, re.source);
    /** The «Lập sổ» sheet's content and the notebook body on every animation frame, kept as runs of equal states. */
    const batMau = (page) =>
      page.evaluate(() => {
        const w = window;
        w.__rt = [];
        w.__rtDung = false;
        w.__rtT0 = performance.now();
        const hien = (x) => x.getClientRects().length > 0 && x.checkVisibility({ visibilityProperty: true, opacityProperty: true });
        const buoc = () => {
          if (w.__rtDung) return;
          const d = [...document.querySelectorAll('[role="dialog"]')].filter(hien).pop();
          const nut = d ? [...d.querySelectorAll('[role="button"]')].map((b) => (b.innerText ?? "").trim()) : [];
          const sheet = !d ? "-" : d.querySelector('[data-testid="lap-so-dang-cho"]') ? "cho" : nut.includes("Đồng ý") ? "dong-y" : nut.some((x) => /^Đề nghị lập sổ/.test(x)) ? "de-nghi" : "khac";
          const trong = [...document.querySelectorAll('[data-testid="giay-trong"]')].find(hien);
          const than = [...document.querySelectorAll('[data-testid="giay-chua-lap-so"]')].some(hien) ? "chua-lap-so" : trong ? (trong.querySelector('[aria-label="Sổ đã mở"]') ? "m6" : "thuong") : /Hai người cũng thành một hội/.test(document.body.innerText ?? "") ? "hoi" : "khac";
          const t = Math.round(performance.now() - w.__rtT0);
          const cuoi = w.__rt[w.__rt.length - 1];
          if (cuoi && cuoi.sheet === sheet && cuoi.than === than) {
            cuoi.n += 1;
            cuoi.den = t;
          } else w.__rt.push({ tu: t, den: t, n: 1, sheet, than });
          requestAnimationFrame(buoc);
        };
        requestAnimationFrame(buoc);
      });
    const tatMau = (page) =>
      page.evaluate(() => {
        window.__rtDung = true;
        return window.__rt;
      });
    const bayGio = (page) => page.evaluate(() => Math.round(performance.now() - window.__rtT0));
    const chuoi = (runs, tu) => runs.filter((r) => r.den >= tu).map((r) => `${r.than}/${r.sheet}×${r.n}@${Math.max(0, r.tu - tu)}`).join(" → ");

    // UI-084 (a): the proposer keeps the waiting sheet open; the other side agrees on their own phone.
    const c45 = await lapCap("chat-4", "chat-5");
    if ((await daDongY(c45.cap, c45.pb, "lap_so")) || ((await soCua(c45.cap, c45.pa)).pending_proposals ?? []).length) console.log("r-f07: sổ chat-4/chat-5 đã có lời đề nghị hoặc đã mở ở lượt trước, bỏ qua UI-084 (a)");
    else {
      const t = await mo("C1", "chat-4", "/messages");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(1500);
      await diToi(t, `/groups/${c45.cap.id}/to-giay`);
      const ngoai = await timNut(t.page, "Đề nghị lập sổ");
      if (ngoai) await cham(cdp, ngoai);
      await t.page.waitForTimeout(1200);
      const trong = await trongDialog(t.page, /^Đề nghị lập sổ/);
      if (trong) await cham(cdp, trong);
      await t.page.waitForTimeout(2500);
      const dangCho = await hienTestId(t.page, "lap-so-dang-cho");
      await batMau(t.page);
      await t.page.waitForTimeout(300);
      const tDongY = await bayGio(t.page);
      const g = await haiDongY(c45, "lap_so");
      // The screen learns on its own read (focus or poll): wait for the body to leave the shut cover, then 2 s more.
      const t0 = Date.now();
      while ((await hienTestId(t.page, "giay-chua-lap-so")) && Date.now() - t0 < 30_000) await t.page.waitForTimeout(250);
      await t.page.waitForTimeout(2000);
      const runs = await tatMau(t.page);
      await anh(t.page, "EV-R-UI-084-A-C1", t);
      const moSo = runs.find((r) => r.den >= tDongY && !["chua-lap-so", "khac"].includes(r.than));
      const cuoi = runs[runs.length - 1];
      const dat = dangCho && !!moSo && cuoi.sheet === "-";
      ket({ feature: "F07", issue: "UI-084", phan: "A", screen: "F07.S02", layer: "L23", state: "chat-4 vừa đề nghị lập sổ, sheet chờ để mở", action: "chat-5 đồng ý trên máy của họ (API); chat-4 chờ, không tải lại", cauHinh: "C1", expected: "(a) sheet chờ tự đóng khi sổ mở", status: dat ? "PASS" : "FAIL", evidence: ["EV-R-UI-084-A-C1"], ghiChu: `${!dangCho ? "đổi: không thấy sheet chờ" : dat ? "hết" : "còn"}: đồng ý qua API ${g}; thân màn đổi ${moSo ? `sau ${moSo.tu - tDongY} ms, sang «${moSo.than}»` : "chưa đổi sau 30 s"}; sheet lúc cuối: ${cuoi.sheet === "-" ? "đã đóng" : cuoi.sheet}; chuỗi khung (thân/sheet) sau đồng ý: ${chuoi(runs, tDongY).slice(0, 260)}` });
      await t.context.close();
    }

    // UI-084 (b): the invited side agrees on screen, at C1 and under Reduce Motion.
    for (const [cfg, tenA, tenB] of [["C1", "chat-6", "chat-7"], ["C9", "chat-10", "chat-11"]]) {
      const c = await lapCap(tenA, tenB);
      if (await daDongY(c.cap, c.pb, "lap_so")) {
        console.log(`r-f07: sổ ${tenA}/${tenB} đã mở ở lượt trước, bỏ qua UI-084 (b) ${cfg}`);
        continue;
      }
      if (!((await soCua(c.cap, c.pb)).pending_proposals ?? []).some((d) => d.purpose === "lap_so")) await ghiApi("POST", `/contexts/${c.cap.id}/notebook/proposals`, { purpose: "lap_so" }, c.pa);
      const t = await mo(cfg, tenB, "/messages");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(1500);
      await diToi(t, `/groups/${c.cap.id}/to-giay`);
      const xem = await timNut(t.page, "Xem lời đề nghị");
      if (xem) await cham(cdp, xem);
      await t.page.waitForTimeout(1200);
      const dongY = await trongDialog(t.page, /^Đồng ý$/);
      await batMau(t.page);
      await t.page.waitForTimeout(300);
      const tCham = await bayGio(t.page);
      if (dongY) await cham(cdp, dongY);
      await t.page.waitForTimeout(4000);
      const runs = await tatMau(t.page);
      await anh(t.page, `EV-R-UI-084-B-${cfg}`, t);
      const sau = runs.filter((r) => r.den >= tCham);
      const deNghi = sau.filter((r) => r.sheet === "de-nghi").reduce((a, r) => a + r.n, 0);
      const m6 = sau.find((r) => r.than === "m6");
      const thuong = sau.filter((r) => r.than === "thuong" && (!m6 || r.tu < m6.tu)).reduce((a, r) => a + r.n, 0);
      const moSo = await daDongY(c.cap, c.pb, "lap_so");
      const dat = !!dongY && moSo && deNghi === 0 && thuong === 0;
      ket({ feature: "F07", issue: "UI-084", phan: "B", screen: "F07.S02", layer: "L23", state: `${cfg === "C9" ? "giảm chuyển động; " : ""}${tenA} đã đề nghị lập sổ (API)`, action: `${tenB}: «Xem lời đề nghị» → «Đồng ý»`, cauHinh: cfg, expected: "(b) 0 khung trạng thái mời sau «Đồng ý», 0 khung thân thường trước bìa", status: dat ? "PASS" : "FAIL", evidence: [`EV-R-UI-084-B-${cfg}`], ghiChu: `${!dongY ? "đổi: không thấy «Đồng ý»" : dat ? "hết" : "còn"}: sau chạm: ${deNghi} khung sheet mời «Đề nghị lập sổ», ${thuong} khung thân «Tuần này» thường trước bìa; bìa M6: ${m6 ? "có" : "không"}; sổ mở trên máy chủ: ${moSo ? "có" : "không"}; chuỗi: ${chuoi(runs, tCham).slice(0, 260)}` });
      await t.context.close();
    }

    // UI-083: the notebook of chat-4/chat-5 (open) read while the server answers 503.
    {
      const soMo = (await daDongY(c45.cap, c45.pa, "lap_so")) && (await daDongY(c45.cap, c45.pb, "lap_so"));
      const t = await mo("C1", "chat-4", "/messages");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(1500);
      await loiMayChu(t.page, new RegExp(`/contexts/${c45.cap.id}/(notebook|papers)(\\?|$)`));
      await diToi(t, `/groups/${c45.cap.id}/to-giay`);
      await t.page.waitForTimeout(1000);
      const chu = await chuTrang(t.page);
      const bia = await hienTestId(t.page, "giay-chua-lap-so");
      const thuLai = (await timNut(t.page, "Thử lại")) ?? (await timNut(t.page, "Làm mới"));
      await anh(t.page, "EV-R-UI-083-C1", t);
      await goHet(t.page);
      if (thuLai) await cham(cdp, thuLai);
      await t.page.waitForTimeout(3500);
      const sau = await chuTrang(t.page);
      const dat = soMo && !bia && /chưa đọc|Chưa đọc|sự cố/.test(chu) && !!thuLai;
      ket({ feature: "F07", issue: "UI-083", screen: "F07.S02", state: `sổ chat-4/chat-5 ${soMo ? "đã mở" : "chưa mở (ca không hợp lệ)"}; GET sổ và tờ trả 503`, action: "mở tờ giấy; máy chủ ổn lại; «Thử lại» nếu có", cauHinh: "C1", expected: "503 khi đọc sổ: màn nói chưa đọc được, có «Thử lại»; không hiện bìa «chưa có sổ»", status: dat ? "PASS" : "FAIL", evidence: ["EV-R-UI-083-C1"], ghiChu: `${dat ? "hết" : "còn"}: khi 503: bìa sổ chưa lập ${bia ? "có" : "không"}, «${chu.slice(0, 170)}»; nút ${thuLai ? `«${thuLai.ten}»` : "thử lại: không có"}; sau khi máy chủ ổn: «${sau.slice(0, 110)}»` });
      await t.context.close();
    }

    // UI-082: the same notebook's link opened without a session.
    {
      const duong = `/groups/${c45.cap.id}/to-giay`;
      const t = await mo("C1", null, duong);
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(2500);
      await choOn(t.page, { mang: t.mang });
      const den = await duongDan(t.page);
      const chu = await chuTrang(t.page);
      const nhanDemo = /Dữ liệu demo|[Bb]ản trải nghiệm/.test(chu);
      await anh(t.page, "EV-R-UI-082-A-C1", t);
      const ghiDi = [];
      t.page.on("request", (r) => {
        if (r.url().startsWith(mt.api) && r.method() !== "GET") ghiDi.push(`${r.method()} ${anId(new URL(r.url()).pathname)}`);
      });
      let ru = null;
      let gui = null;
      let daGui = false;
      if (/to-giay/.test(den)) {
        ru = await timNut(t.page, "Rủ đi chơi");
        if (ru) await cham(cdp, ru);
        await t.page.waitForTimeout(2500);
        gui = await timNut(t.page, "Gửi cho người ấy");
        if (gui) await cham(cdp, gui);
        await t.page.waitForTimeout(2500);
        daGui = /Đã gửi/.test(await chuTrang(t.page));
        await anh(t.page, "EV-R-UI-082-B-C1", t);
      }
      const cua = /^\/(welcome|login)/.test(den);
      ket({ feature: "F07", issue: "UI-082", phan: "SO", screen: "F07.S02", state: "không phiên; link tờ giấy của cặp chat-4/chat-5 (sổ đã mở)", action: "mở thẳng link; nếu ở lại trang: «Rủ đi chơi» → «Gửi cho người ấy»", cauHinh: "C1", expected: "không phiên, mở link: tới cửa vào; không màn nào hiện «Đã gửi» khi chưa có lệnh ghi thành công", status: cua ? "PASS" : "FAIL", evidence: /to-giay/.test(den) ? ["EV-R-UI-082-A-C1", "EV-R-UI-082-B-C1"] : ["EV-R-UI-082-A-C1"], ghiChu: `${cua ? "hết" : "còn"}: tới ${anId(den)}; nhãn demo/trải nghiệm: ${nhanDemo ? "có" : "không"}; «Rủ đi chơi» ${ru ? "có" : "không"}, «Gửi cho người ấy» ${gui ? "có" : "không"}, «Đã gửi» ${daGui ? "hiện" : "không hiện"}; lệnh ghi tới API: ${ghiDi.join(", ") || "0"}; màn đầu «${chu.slice(0, 120)}»` });
      await t.context.close();
    }

    // UI-120: a couple; A (chat-8) blocks B (chat-9), and B tries to send A a sheet.
    const c89 = await lapCap("chat-8", "chat-9");
    const g1 = await haiDongY(c89, "lap_so");
    const g2 = await haiDongY(c89, "bat_doi");
    const laDoi = (await daDongY(c89.cap, c89.pa, "bat_doi")) && (await daDongY(c89.cap, c89.pb, "bat_doi"));
    const chanAB = async () => JSON.stringify((await api("GET", "/people/me/blocked", undefined, c89.pa)).blocked ?? []).includes(c89.pb.person_id);
    if ((await toCua(c89.cap, c89.pb)).length) console.log("r-f07: cặp chat-8/chat-9 đã có tờ ở lượt trước, bỏ qua UI-120");
    else {
      const kq = { lapSo: g1, batDoi: g2, laDoi };
      kq.chanStatus = (await ghiApi("POST", `/people/${c89.pb.person_id}/block`, {}, c89.pa)).status;
      kq.chan = await chanAB();
      const tb = await mo("C1", "chat-9", "/messages");
      const cdpB = await cdpCua(tb.page);
      await tb.page.waitForTimeout(2500);
      const hang = await tb.page.evaluate((ten) => {
        const e = [...document.querySelectorAll('[role="button"],[role="link"],a[href]')].find((x) => (x.getAttribute("aria-label") || x.innerText || "").includes(ten) && x.getBoundingClientRect().width > 0);
        if (!e) return null;
        e.scrollIntoView({ block: "center", behavior: "instant" });
        const r = e.getBoundingClientRect();
        return { x: r.left + r.width / 2, y: r.top + Math.min(r.height / 2, 30) };
      }, tenHien("chat-8"));
      if (hang) await cham(cdpB, hang);
      await tb.page.waitForTimeout(3000);
      await choOn(tb.page, { mang: tb.mang });
      kq.chat = anId(await duongDan(tb.page));
      const dai = await tamCua(tb.page, '[data-testid="hang-to-giay"]');
      kq.loi = dai ? "dải «Tờ giấy» của chat đôi" : "link tờ giấy (chat không có dải)";
      if (dai) {
        await cham(cdpB, dai);
        await tb.page.waitForTimeout(2500);
        await choOn(tb.page, { mang: tb.mang });
      } else await diToi(tb, `/groups/${c89.cap.id}/to-giay`);
      kq.bGiay = anId(await duongDan(tb.page));
      const ru = await timNut(tb.page, "Rủ đi chơi");
      if (ru) await cham(cdpB, ru);
      await tb.page.waitForTimeout(3000);
      const gui = await timNut(tb.page, "Gửi cho người ấy");
      if (gui) await cham(cdpB, gui);
      await tb.page.waitForTimeout(3000);
      kq.ru = !!ru;
      kq.gui = !!gui;
      kq.bCau = (await chuTrang(tb.page)).match(/[^.]*(Đã gửi|chặn|không gửi|Chưa gửi|thử lại)[^.]*\./i)?.[0]?.trim() ?? null;
      await anh(tb.page, "EV-R-UI-120-B-C1", tb);
      await tb.context.close();
      // What reached the server, from both sides; a draft B could not send is tried once more over the API.
      const toB = await toCua(c89.cap, c89.pb);
      kq.toB = toB.map((x) => x.state);
      const nhap = toB.find((x) => x.state === "nhap");
      if (nhap) kq.guiApi = (await ghiApi("POST", `/papers/${nhap.id}/send`, { version: nhap.version }, c89.pb)).status;
      const toA = await toCua(c89.cap, c89.pa);
      kq.toA = toA.map((x) => x.state);
      const ta = await mo("C1", "chat-8", "/messages");
      await ta.page.waitForTimeout(1500);
      await diToi(ta, `/groups/${c89.cap.id}/to-giay`);
      const aChu = await chuTrang(ta.page);
      kq.aRu = /Rủ đi chơi/.test(aChu);
      kq.aChan = /chặn/i.test(aChu);
      await anh(ta.page, "EV-R-UI-120-A-C1", ta);
      await ta.context.close();
      kq.boChan = (await ghiApi("DELETE", `/people/${c89.pb.person_id}/block`, undefined, c89.pa)).status;
      kq.hetChan = !(await chanAB());
      const toiA = toA.some((x) => ["da_gui", "da_xem", "de_nghi_sua", "dong_y", "chot"].includes(x.state));
      const dat = kq.chan && laDoi && !toiA && (kq.guiApi === undefined || kq.guiApi === 409) && /chặn|không gửi/i.test(kq.bCau ?? "") && !kq.aRu;
      ket({ feature: "E5", issue: "UI-120", screen: "E5", state: `cặp đôi chat-8/chat-9 (lập sổ ${g1}, «Một đôi» ${g2}, là cặp đôi: ${laDoi ? "có" : "không"}); chat-8 chặn chat-9 (${kq.chanStatus})`, action: `chat-9: Tin nhắn → chat đôi → ${kq.loi} → «Rủ đi chơi» → «Gửi cho người ấy»; chat-8 mở tờ giấy`, cauHinh: "C1", expected: "A chặn B: POST /papers/{id}/send của B bị từ chối (409); màn B nói không gửi được; sổ của A không nhận tờ mới và không mời «Rủ đi chơi»", status: dat ? "PASS" : "FAIL", evidence: ["EV-R-UI-120-B-C1", "EV-R-UI-120-A-C1"], ghiChu: `${dat ? "hết" : "còn"}: đã chặn trên máy chủ: ${kq.chan ? "có" : "không"}; chat mở ở ${kq.chat}, tờ giấy ${kq.bGiay}; «Rủ đi chơi» ${kq.ru ? "chạm được" : "không có"}, «Gửi cho người ấy» ${kq.gui ? "chạm được" : "không có"}; câu ở B: «${kq.bCau ?? "-"}»; tờ phía B: ${kq.toB.join(", ") || "không"}${kq.guiApi !== undefined ? `; gửi lại qua API: ${kq.guiApi}` : ""}; tờ phía A: ${kq.toA.join(", ") || "không"}; A thấy «Rủ đi chơi»: ${kq.aRu ? "có" : "không"}, chữ «chặn»: ${kq.aChan ? "có" : "không"}; bỏ chặn ${kq.boChan}, hết chặn: ${kq.hetChan ? "có" : "không"}` });
    }

    // UI-085: the same couple, a week whose sheet is agreed; «Rủ Chat Test 10 tới đây» from a place.
    {
      const tuanNay = async () => (await toCua(c89.cap, c89.pa)).find((x) => x.state === "chot");
      let chot = await tuanNay();
      const dung = [];
      if (!chot) {
        const mo_ = (await toCua(c89.cap, c89.pa)).find((x) => ["da_gui", "da_xem"].includes(x.state));
        if (mo_) dung.push(`A đồng ý tờ của B: ${(await ghiApi("POST", `/papers/${mo_.id}/versions/${mo_.version}/responses`, { kind: "dong_y" }, c89.pa)).status}`);
        else {
          const nhapA = await ghiApi("POST", `/contexts/${c89.cap.id}/papers/draft`, undefined, c89.pa);
          const gui = nhapA.json?.id ? await ghiApi("POST", `/papers/${nhapA.json.id}/send`, { version: nhapA.json.version }, c89.pa) : { status: "-" };
          const dy = nhapA.json?.id ? await ghiApi("POST", `/papers/${nhapA.json.id}/versions/${gui.json?.version ?? nhapA.json.version}/responses`, { kind: "dong_y" }, c89.pb) : { status: "-" };
          dung.push(`A phác ${nhapA.status}, gửi ${gui.status}, B đồng ý ${dy.status}`);
        }
        chot = await tuanNay();
      }
      const truoc = await toCua(c89.cap, c89.pa);
      const t = await mo("C1", "chat-8", "/places/p-tiem-nuong-xom-lao");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(2000);
      await choOn(t.page, { mang: t.mang });
      const nut = await timNut(t.page, `Rủ ${tenHien("chat-9")} tới đây`);
      if (nut) await cham(cdp, nut);
      await t.page.waitForTimeout(3500);
      await choOn(t.page, { mang: t.mang });
      const den = anId(await duongDan(t.page));
      const dau = (await chuTrang(t.page)).slice(0, 260);
      await anh(t.page, "EV-R-UI-085-C1", t);
      await t.context.close();
      const sau = await toCua(c89.cap, c89.pa);
      const moi = sau.filter((x) => !truoc.some((y) => y.id === x.id));
      let mangCho = null;
      if (moi.length) mangCho = /p-tiem-nuong-xom-lao|Tiệm Nướng Xóm Lào/.test(JSON.stringify(await api("GET", `/papers/${moi[0].id}`, undefined, c89.pa)));
      const dat = !!chot && !!nut && (moi.length === 0 || mangCho === true);
      ket({ feature: "F07", issue: "UI-085", screen: "F07.S02", state: `cặp đôi chat-8/chat-9, tuần này có tờ chốt: ${chot ? "có" : "không"}${dung.length ? ` (${dung.join("; ")})` : ""}`, action: "chat-8: chi tiết Tiệm Nướng Xóm Lào → «Rủ Chat Test 10 tới đây»", cauHinh: "C1", expected: "tuần đã có tờ chốt: số tờ không đổi và câu giải thích khớp với màn; hoặc bản phác mới mang chỗ vừa chọn", status: dat ? "PASS" : "FAIL", evidence: ["EV-R-UI-085-C1"], ghiChu: `${!nut ? "đổi: không thấy nút «Rủ … tới đây»" : dat ? "hết (câu trên đầu phân xử bằng mắt)" : "còn"}: tới ${den}; tờ ${truoc.length} → ${sau.length}${moi.length ? ` (mới: ${moi.map((x) => x.state).join(", ")}, mang chỗ vừa chọn: ${mangCho ? "có" : "không"})` : ""}; đầu màn «${dau}»` });
    }
  }

  // ------------------------------------------------------------------ F08
  // The chat-test group gets, once: an outing that starts today (its album is
  // the photos of its days) and three synthetic photos shared through «Thả
  // khoảnh khắc» by chat-0, the long caption first so it ends at the bottom of
  // the wall; and chat-0 a «Bạn bè» post with two comments by chat-1 over the
  // API. Nothing of Team Đà Lạt is written.
  if (coPhan("r-f08")) {
    const pa = await phienCua("chat-0");
    const pb = await phienCua("chat-1");
    const G = ((await api("GET", "/people/me/contexts", undefined, pa)).contexts ?? []).find((c) => c.kind === "group");
    const KEO = "Kèo album retest";
    const ngayVN = (lech = 0) => new Date(Date.now() + 7 * 3600e3 + lech * 864e5).toISOString().slice(0, 10);
    const keoCua = async () => ((await api("GET", `/contexts/${G.id}/outings`, undefined, pa)).outings ?? []).find((o) => o.title === KEO);
    let keoAlbum = await keoCua();
    if (!keoAlbum) {
      await ghiApi("POST", `/contexts/${G.id}/outings`, { title: KEO, starts_on: ngayVN(0), ends_on: ngayVN(1), headcount: 20, budget_per_person_vnd: 300000 }, pa);
      keoAlbum = await keoCua();
    }
    const kyNiem = async () => (await api("GET", `/contexts/${G.id}/memories?limit=50`, undefined, pa)).memories ?? [];
    const CAU_DAI = "Chiều cuối tuần cả nhóm ngồi bờ hồ đợi hoàng hôn, trời trong tới mức thấy cả dãy núi phía xa; ai cũng bảo lần sau phải mang thêm áo khoác vì gió lên nhanh.";
    const ANH = [
      { ten: "doc-9-16", w: 360, h: 640, cau: CAU_DAI },
      { ten: "ngang", w: 640, h: 360, cau: "Hoàng hôn trên hồ, lượt retest" },
      { ten: "doc-3-4", w: 480, h: 640, cau: "" },
    ];
    /** Pick a synthetic file through the picker a tap opens. */
    const chonTep = async (t, cdp, chuNut, tep) => {
      const nut = await timNut(t.page, chuNut);
      const [fc] = await Promise.all([t.page.waitForEvent("filechooser", { timeout: 6000 }).catch(() => null), nut ? cham(cdp, nut) : Promise.resolve()]);
      if (fc) await fc.setFiles([{ name: `${tep.ten}.png`, mimeType: "image/png", buffer: Buffer.from(pngThuBytes(tep.w, tep.h)) }]);
      await t.page.waitForTimeout(1500);
      return !!fc;
    };
    const oChu = async (page, nhan, chu) => {
      const o = page.locator(`[aria-label="${nhan}"]`).filter({ visible: true }).first();
      if (!(await o.count())) return false;
      await o.click();
      await o.fill(chu);
      return true;
    };
    const daCo = await kyNiem();
    const dang = [];
    for (const a of ANH) {
      if (daCo.some((k) => k.kind === "photo" && (k.caption ?? "") === a.cau)) continue;
      const t = await mo("C1", "chat-0", "/messages");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(1500);
      await diToi(t, `/groups/${G.id}/wall`);
      const tha = await timNut(t.page, "Thả khoảnh khắc");
      if (tha) await cham(cdp, tha);
      await t.page.waitForTimeout(1500);
      await chonTep(t, cdp, "Chưa chọn ảnh, chạm để chọn", a);
      if (a.cau) await oChu(t.page, "Ô câu chú thích", a.cau);
      const gui = await timNut(t.page, "Chia sẻ ngay vào nhóm");
      if (gui) await cham(cdp, gui);
      await t.page.waitForTimeout(4000);
      dang.push(`${a.ten}: ${gui ? "gửi" : "không thấy nút gửi"}`);
      await t.context.close();
    }
    const anhTuong = (await kyNiem()).filter((k) => k.kind === "photo").length;
    console.log(`r-f08: dữ liệu: kèo «${KEO}» ${keoAlbum ? "có" : "không"}; ảnh trên tường ${anhTuong}; lượt này đăng: ${dang.join(", ") || "không"}`);

    // UI-094: the photo viewer of the outing's album.
    if (chayPhan("r-f08", "094")) {
      const t = await mo("C1", "chat-0", "/messages");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(1500);
      await diToi(t, `/trips/${keoAlbum.id}/album?ctx=${G.id}`);
      const soAnh = await t.page.evaluate(() => [...document.querySelectorAll('[aria-label^="Mở ảnh"]')].filter((e) => e.getClientRects().length).length);
      const nutMo = await t.page.evaluate(() => {
        const e = [...document.querySelectorAll('[aria-label^="Mở ảnh"]')].find((x) => x.getClientRects().length);
        if (!e) return null;
        e.scrollIntoView({ block: "center" });
        const r = e.getBoundingClientRect();
        return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
      });
      if (nutMo) await cham(cdp, nutMo);
      await t.page.waitForTimeout(1500);
      // The viewer's own pager: the middle of it is where a finger goes.
      const tam = await t.page.evaluate(() => {
        const d = document.querySelector('[role="dialog"][aria-modal="true"]');
        const cuon = d ? [...d.querySelectorAll("div")].find((e) => /(auto|scroll)/.test(getComputedStyle(e).overflowX) && e.clientWidth >= 300 && e.getBoundingClientRect().height > 100) : null;
        const r = cuon?.getBoundingClientRect();
        return r ? { x: r.left + r.width / 2, y: r.top + r.height / 2 } : null;
      });
      const anhHien = () =>
        t.page.evaluate(() => {
          const d = document.querySelector('[role="dialog"][aria-modal="true"]');
          const im = d ? [...d.querySelectorAll("img")].find((x) => { const r = x.getBoundingClientRect(); return r.left >= -1 && r.right <= innerWidth + 1; }) : null;
          const r = im?.getBoundingClientRect();
          return im ? { w: Math.round(r.width), h: Math.round(r.height), top: Math.round(r.top), bottom: Math.round(r.bottom), trong: r.top >= 0 && r.bottom <= innerHeight } : null;
        });
      const cuonX = () =>
        t.page.evaluate(() => {
          const d = document.querySelector('[role="dialog"][aria-modal="true"]');
          const c = d ? [...d.querySelectorAll("div")].find((e) => /(auto|scroll)/.test(getComputedStyle(e).overflowX) && e.clientWidth >= 300) : null;
          return c ? { x: Math.round(c.scrollLeft), rong: c.clientWidth } : null;
        });
      const dem = () => t.page.evaluate(() => [...document.querySelectorAll("div,span")].find((e) => e.children.length === 0 && /^\d+ \/ \d+$/.test((e.textContent ?? "").trim()) && e.getClientRects().length)?.textContent.trim() ?? null);
      const tfAnh = () =>
        t.page.evaluate(() => {
          const d = document.querySelector('[role="dialog"][aria-modal="true"]');
          const im = d ? [...d.querySelectorAll("img")].find((x) => { const r = x.getBoundingClientRect(); return r.left >= -1 && r.right <= innerWidth + 1; }) : null;
          for (let n = im, i = 0; i < 6 && n; i += 1, n = n.parentElement) if (n.style?.transform && /scale/.test(n.style.transform)) return n.style.transform;
          return null;
        });
      const anh0 = await anhHien();
      const x0 = await cuonX();
      const dem0 = await dem();
      // The viewer as it opens, before any gesture: the pinch below zooms the whole page.
      await anh(t.page, "EV-R-UI-094-MO-C1", t);
      if (tam) await keo(cdp, { x: tam.x + 120, y: tam.y }, { x: tam.x - 160, y: tam.y }, { ms: 250, buoc: 10 });
      await t.page.waitForTimeout(1500);
      const x1 = await cuonX();
      const dem1 = await dem();
      if (tam) {
        await cham(cdp, tam, 30);
        await t.page.waitForTimeout(90);
        await cham(cdp, tam, 30);
      }
      await t.page.waitForTimeout(900);
      const tf1 = await tfAnh();
      if (tam) await nhip(cdp, tam, { tu: 60, den: 220, ms: 500 });
      await t.page.waitForTimeout(900);
      const scale = await t.page.evaluate(() => Math.round(visualViewport.scale * 100) / 100);
      await anh(t.page, "EV-R-UI-094-C1", t);
      const vuotDung = !!x1 && !!x0 && Math.abs(x1.x - x0.rong) <= 2 && dem1 === `2 / ${soAnh >= 3 ? 3 : soAnh}`;
      const dat = !!anh0 && anh0.h > 0 && anh0.trong && vuotDung && /scale\(2/.test(tf1 ?? "") && scale === 1;
      ket({ feature: "F08", issue: "UI-094", screen: "F08.S03", layer: "L25", state: `album «${KEO}», ${soAnh} ô ảnh (3 ảnh tổng hợp)`, action: "chạm ảnh dẫn; vuốt trái giữa khung; chạm đúp; chụm hai ngón", cauHinh: "C1", expected: "web C1: ảnh cao > 0, trọn trong vùng; một cú vuốt sang đúng ảnh sau và bộ đếm «2 / 3»; chạm đúp ra scale(2); chụm không đổi visualViewport.scale", status: dat ? "PASS" : "FAIL", evidence: ["EV-R-UI-094-MO-C1", "EV-R-UI-094-C1"], ghiChu: `${!nutMo ? "đổi: không thấy ảnh để mở" : dat ? "hết" : "còn"}: ảnh ${anh0 ? `${anh0.w}×${anh0.h}, ${anh0.trong ? "trọn" : "không trọn"} trong khung` : "không thấy"}; vuốt: cuộn ${x0?.x ?? "-"} → ${x1?.x ?? "-"} (một trang = ${x0?.rong ?? "-"}), bộ đếm «${dem0 ?? "-"}» → «${dem1 ?? "-"}»; chạm đúp: ${tf1 ?? "không có transform scale"}; sau chụm visualViewport.scale ${scale}` });
      await t.context.close();
    }

    // UI-095: a heart failing on the last memory of the wall.
    if (chayPhan("r-f08", "095")) {
      const t = await mo("C1", "chat-1", "/messages");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(1500);
      await diToi(t, `/groups/${G.id}/wall`);
      await loiMayChu(t.page, /\/memories\/[^/]+\/reactions/);
      const tim = await t.page.evaluate(() => {
        const ds = [...document.querySelectorAll('[aria-label^="Thả tim"],[aria-label^="Bỏ tim"]')].filter((e) => e.getClientRects().length);
        const e = ds[ds.length - 1];
        if (!e) return null;
        e.scrollIntoView({ block: "center" });
        const r = e.getBoundingClientRect();
        return { x: r.left + r.width / 2, y: r.top + r.height / 2, i: ds.length };
      });
      if (tim) await cham(cdp, tim);
      await t.page.waitForTimeout(1800);
      const cau = await t.page.evaluate(() => {
        const e = [...document.querySelectorAll('[aria-live="polite"]')].find((x) => x.getClientRects().length && (x.innerText ?? "").trim());
        if (!e) return null;
        const r = e.getBoundingClientRect();
        return { chu: e.innerText.trim().slice(0, 100), top: Math.round(r.top), trongMan: r.bottom > 0 && r.top < innerHeight };
      });
      await anh(t.page, "EV-R-UI-095-C1", t);
      await goHet(t.page);
      ket({ feature: "F08", issue: "UI-095", screen: "F08.S01", state: `tường nhóm chat-test, ${anhTuong} ảnh; máy chủ trả 503 cho tim`, action: "chat-1 cuộn tới kỷ niệm cuối, chạm «Thả tim»", cauHinh: "C1", expected: "tim lỗi ở bất kỳ kỷ niệm nào: câu lỗi nằm trong khung nhìn ngay sau khi chạm", status: cau?.trongMan ? "PASS" : "FAIL", evidence: ["EV-R-UI-095-C1"], ghiChu: `${!tim ? "đổi: không thấy nút tim" : cau?.trongMan ? "hết" : "còn"}: chạm nút tim thứ ${tim?.i ?? "-"}; câu ${cau ? `«${cau.chu}» ở y ${cau.top}${cau.trongMan ? "" : " (ngoài màn)"}` : "không thấy"}` });
      await t.context.close();
    }

    // UI-096: deleting a comment of a post, for its writer and for the post's author.
    if (chayPhan("r-f08", "096")) {
      const baiCua = async () => ((await api("GET", `/people/${pa.person_id}/posts`, undefined, pa)).posts ?? []).find((x) => x.audience === "friends");
      let bai = await baiCua();
      if (!bai) {
        await ghiApi("POST", "/posts", { body: "Cuối tuần vừa rồi cả nhóm đi hồ, trời trong và gió nhẹ. Ai muốn đi lần sau thì bình luận nhé.", audience: "friends" }, pa);
        bai = await baiCua();
      }
      const blCua = async () => (await api("GET", `/posts/${bai.id}/comments`, undefined, pa)).comments ?? [];
      if ((await blCua()).length < 2) for (const noi of ["Mình đi! Lần sau nhớ gọi mình từ tối hôm trước.", "Đẹp quá"]) await ghiApi("POST", `/posts/${bai.id}/comments`, { body: noi }, pb);
      const kq = [];
      for (const [ten, vai] of [["chat-1", "người viết bình luận"], ["chat-0", "tác giả bài"]]) {
        const t = await mo("C1", ten, "/messages");
        const cdp = await cdpCua(t.page);
        await t.page.waitForTimeout(1500);
        await diToi(t, `/posts/${bai.id}`);
        await t.page.waitForTimeout(800);
        const d = await t.page.evaluate(() => {
          const hien = (x) => x.getClientRects().length > 0;
          const nut = [...document.querySelectorAll('[role="button"],button')].filter(hien).map((e) => (e.getAttribute("aria-label") || e.innerText || "").replace(/\s+/g, " ").trim());
          const la = [...document.querySelectorAll("div,span")].find((e) => hien(e) && e.children.length === 0 && (e.textContent ?? "").trim() === "Đẹp quá");
          const r = la?.getBoundingClientRect();
          return { xoa: nut.filter((x) => /Xoá|Xóa|thùng rác/i.test(x)), hanhDong: [...new Set(nut.filter((x) => /bình luận|Trả lời/.test(x)))], bl: r ? { x: r.left + r.width / 2, y: r.top + r.height / 2 } : null };
        });
        // A long press on the comment: is deleting hidden behind it?
        let menu = 0;
        if (d.bl) {
          await nhanGiu(cdp, d.bl);
          await t.page.waitForTimeout(900);
          menu = await demDialog(t.page);
        }
        if (ten === "chat-0") await anh(t.page, "EV-R-UI-096-C1", t);
        kq.push({ ten, vai, ...d, menu });
        await t.context.close();
      }
      const soBl = (await blCua()).length;
      const khongXoa = kq.every((k) => k.xoa.length === 0 && k.menu === 0);
      ket({ feature: "F08", issue: "UI-096", screen: "F08.S08", state: `bài «Bạn bè» của Chat Test 01, ${soBl} bình luận của Chat Test 02`, action: "mở bài dưới hai vai; tìm nút xoá bình luận; nhấn giữ bình luận «Đẹp quá»", cauHinh: "C1", expected: "một chạm vào thùng rác không xoá; vùng chạm ≥ 48dp", status: khongXoa ? "PASS" : "FAIL", evidence: ["EV-R-UI-096-C1"], ghiChu: `${khongXoa ? "đổi: màn bài không còn nút xoá bình luận nào, nên không còn xoá một chạm; cũng không còn lối xoá bình luận trên màn này (API DELETE /posts/{id}/comments/{id} vẫn có)" : "còn"}: ${kq.map((k) => `${k.vai}: nút xoá ${k.xoa.length ? k.xoa.join(", ") : "không"}, hành động trên bình luận: ${k.hanhDong.join(", ") || "-"}, nhấn giữ mở ${k.menu} hộp`).join("; ")}` });
    }

    // UI-097: leaving «Thả khoảnh khắc» and «Đăng story» with a photo and a caption.
    if (chayPhan("r-f08", "097")) {
      const nhapCon = (page, oCau, nutTrong) =>
        page.evaluate(
          ({ oCau, nutTrong }) => {
            const hien = (x) => x.getClientRects().length > 0 && (x.checkVisibility ? x.checkVisibility({ visibilityProperty: true, opacityProperty: true }) : true);
            const o = [...document.querySelectorAll(`[aria-label="${oCau}"]`)].find(hien);
            return { duong: location.pathname, anh: [...document.querySelectorAll("img")].some((i) => i.getAttribute("alt") === "Ảnh đã chọn" && hien(i)), cau: o ? o.value : null, trong: [...document.querySelectorAll(`[aria-label="${nutTrong}"]`)].some(hien), hop: [...document.querySelectorAll('[role="dialog"],[role="alertdialog"]')].filter(hien).length };
          },
          { oCau, nutTrong },
        );
      const MAN = [
        { ten: "moments", tu: `/groups/${G.id}/wall`, nutMo: "Thả khoảnh khắc", nutTrong: "Chưa chọn ảnh, chạm để chọn", oCau: "Ô câu chú thích" },
        { ten: "stories", tu: "/messages", nutMo: "Đăng story mới", nutTrong: "Chưa có ảnh nào, chạm để chọn", oCau: "Ô chú thích" },
      ];
      const kq = {};
      for (const m of MAN) {
        for (const cach of ["back", "nut"]) {
          const t = await mo("C1", "chat-0", "/messages");
          const cdp = await cdpCua(t.page);
          const hoiTrinhDuyet = [];
          t.page.on("dialog", (dl) => {
            hoiTrinhDuyet.push(dl.type());
            void dl.dismiss().catch(() => undefined);
          });
          await t.page.waitForTimeout(1500);
          if (m.tu !== "/messages") await diToi(t, m.tu);
          const moNut = await timNut(t.page, m.nutMo);
          if (moNut) await cham(cdp, moNut);
          await t.page.waitForTimeout(1500);
          await chonTep(t, cdp, m.nutTrong, ANH[1]);
          await oChu(t.page, m.oCau, "Nháp thử quay lại");
          await t.page.waitForTimeout(300);
          const truoc = await nhapCon(t.page, m.oCau, m.nutTrong);
          if (cach === "back") await t.page.goBack();
          else {
            const ql = await timNut(t.page, "Quay lại");
            if (ql) await cham(cdp, ql);
          }
          await t.page.waitForTimeout(1500);
          const roi = await nhapCon(t.page, m.oCau, m.nutTrong);
          let toi = null;
          if (cach === "back") {
            await t.page.goForward();
            await t.page.waitForTimeout(1500);
            toi = await nhapCon(t.page, m.oCau, m.nutTrong);
            await t.page.goBack();
            await t.page.waitForTimeout(1200);
          }
          const lai = await timNut(t.page, m.nutMo);
          if (lai) await cham(cdp, lai);
          await t.page.waitForTimeout(1500);
          const moLai = await nhapCon(t.page, m.oCau, m.nutTrong);
          if (m.ten === "moments" && cach === "back") await anh(t.page, "EV-R-UI-097-C1", t);
          kq[`${m.ten}-${cach}`] = { truoc, roi, toi, lai: moLai, hoiTrinhDuyet };
          await t.context.close();
        }
      }
      const coNhap = (x) => !!x && (x.anh || x.cau === "Nháp thử quay lại");
      const giu = (k) => { const x = kq[k]; return x.hoiTrinhDuyet.length > 0 || x.roi.hop > 0 || /\/(moments|stories)\/new/.test(x.roi.duong) || coNhap(x.lai) || coNhap(x.toi); };
      const soanDuoc = Object.values(kq).every((x) => x.truoc.anh && x.truoc.cau === "Nháp thử quay lại");
      const dat = soanDuoc && Object.keys(kq).every(giu);
      const moTa = (k) => { const x = kq[k]; return `${k}: rời về ${anId(x.roi.duong)}, ${x.roi.hop + x.hoiTrinhDuyet.length} câu hỏi${x.toi ? `, Forward ${coNhap(x.toi) ? "còn nháp" : "trống"}` : ""}, mở lại ${coNhap(x.lai) ? "còn nháp" : x.lai.trong ? "khung trống" : anId(x.lai.duong)}`; };
      ket({ feature: "F08", issue: "UI-097", screen: "F08.S04", layer: "L34", state: "«Thả khoảnh khắc» và «Đăng story» có ảnh và câu, chưa gửi", action: "Back trình duyệt (rồi Forward) hoặc «Quay lại» → mở lại từ cùng nút", cauHinh: "C1", expected: "bốn đường: có câu hỏi, hoặc mở lại còn ảnh và câu", status: dat ? "PASS" : "FAIL", evidence: ["EV-R-UI-097-C1"], ghiChu: `${!soanDuoc ? "đổi: không soạn được đủ ảnh và câu ở một màn" : dat ? "hết" : "còn"}: ${Object.keys(kq).map(moTa).join("; ")}` });
    }
  }

  // ------------------------------------------------------------------ F09
  if (chay("r-f09")) {
    // UI-107: the phone-lookup switch saved while PATCH /people/me answers 503 (nothing is saved).
    const t = await mo("C1", "chat-2", "/messages");
    const cdp = await cdpCua(t.page);
    await t.page.waitForTimeout(1500);
    await diToi(t, "/settings");
    const h = (r) => (r.request().method() === "PATCH" ? r.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ detail: "audit_injected_failure" }) }) : r.continue());
    await t.page.route(/\/people\/me$/, h);
    const sw = await t.page.evaluate(() => {
      const e = [...document.querySelectorAll('[role="switch"],input[type="checkbox"]')].find((x) => /số điện thoại/.test(x.getAttribute("aria-label") ?? "") && x.getClientRects().length);
      if (!e) return null;
      e.scrollIntoView({ block: "center" });
      const r = e.getBoundingClientRect();
      return { x: r.left + r.width / 2, y: r.top + r.height / 2, ten: e.getAttribute("aria-label") };
    });
    if (sw) await cham(cdp, sw);
    await t.page.waitForTimeout(1800);
    const cau = await t.page.evaluate(() => {
      const hien = (x) => x.getClientRects().length > 0;
      const e = [...document.querySelectorAll("div,span")].find((x) => x.children.length === 0 && hien(x) && /gặp sự cố|thử lại|Kiểm tra mạng/.test(x.textContent ?? ""));
      if (!e) return null;
      const r = e.getBoundingClientRect();
      return { chu: e.textContent.trim().slice(0, 100), top: Math.round(r.top), trongMan: r.bottom > 0 && r.top < innerHeight };
    });
    await anh(t.page, "EV-R-UI-107-C1", t);
    await t.page.unroute(/\/people\/me$/, h);
    ket({ feature: "F09", issue: "UI-107", screen: "F09.S02", state: "Cài đặt của chat-2; máy chủ trả 503 cho PATCH /people/me", action: `chạm công tắc «${sw?.ten ?? "tìm theo số điện thoại"}»`, cauHinh: "C1", expected: "lỗi khi lưu công tắc, chip hay ảnh: câu nằm trong khung nhìn, sát mục vừa chạm", status: sw && cau?.trongMan && Math.abs(cau.top - sw.y) <= 200 ? "PASS" : "FAIL", evidence: ["EV-R-UI-107-C1"], ghiChu: `${!sw ? "đổi: không thấy công tắc" : cau?.trongMan && Math.abs(cau.top - sw.y) <= 200 ? "hết" : "còn"}: công tắc ở y ${sw ? Math.round(sw.y) : "-"}; câu ${cau ? `«${cau.chu}» ở y ${cau.top}${cau.trongMan ? "" : " (ngoài màn)"}` : "không thấy"}` });
    await t.context.close();
  }

  // ------------------------------------------------------------------ F11
  // Signed out: the demo chat, the demo «Lên plan» and its options sheet, and
  // what each demo screen says about itself.
  if (coPhan("r-f11")) {
    // UI-116: the long bubble of the demo chat, C1–C3.
    if (chayPhan("r-f11", "116")) {
      const kq = [];
      for (const cfg of ["C1", "C2", "C3"]) {
        const t = await mo(cfg, null, "/messages");
        await t.page.waitForTimeout(3500);
        const den = await duongDan(t.page);
        const d = await t.page.evaluate(() => {
          const hien = (x) => x.getClientRects().length > 0;
          const cuonNgang = (e) => { for (let n = e.parentElement; n; n = n.parentElement) if (/(auto|scroll)/.test(getComputedStyle(n).overflowX) && n.scrollWidth > n.clientWidth + 1) return true; return false; };
          const la = [...document.querySelectorAll("div,span")].filter((e) => hien(e) && e.children.length === 0 && (e.textContent ?? "").trim().length > 8);
          const cau = la.find((e) => /Plan xịn đó/.test(e.textContent));
          let dong = null;
          if (cau) {
            const r = document.createRange();
            r.selectNodeContents(cau);
            dong = new Set([...r.getClientRects()].map((q) => Math.round(q.top))).size;
          }
          const tran = la.filter((e) => e.getBoundingClientRect().right > innerWidth + 1 && !cuonNgang(e)).map((e) => `«${e.textContent.trim().slice(0, 28)}…» thừa ${Math.round(e.getBoundingClientRect().right - innerWidth)}px`);
          return { cau: cau ? { dong, thua: Math.max(0, Math.round(cau.getBoundingClientRect().right - innerWidth)) } : null, tran };
        });
        if (cfg === "C2") await anh(t.page, "EV-R-UI-116-C2", t);
        kq.push({ cfg, den, ...d });
        await t.context.close();
      }
      const dat = kq.every((k) => k.cau && k.cau.thua === 0 && k.tran.length === 0);
      ket({ feature: "F11", issue: "UI-116", screen: "F11.S01", state: "chưa đăng nhập, tab Tin nhắn demo", action: "mở /messages, đọc bong bóng «Plan xịn đó, mình bình chọn chỗ BBQ trước đi.»", cauHinh: "C1–C3", expected: "/messages không phiên ở C1–C3: 0 chữ bị cắt trong bong bóng, câu dài xuống dòng", status: dat ? "PASS" : "FAIL", evidence: ["EV-R-UI-116-C2"], ghiChu: `${kq.some((k) => !k.cau) ? "đổi: có cấu hình không thấy câu" : dat ? "hết" : "còn"}: ${kq.map((k) => `${k.cfg} ở ${k.den}: câu ${k.cau ? `${k.cau.dong} dòng, thừa ${k.cau.thua}px` : "không thấy"}; chữ tràn mép phải ${k.tran.length}${k.tran.length ? ` (${k.tran.slice(0, 2).join(", ")})` : ""}`).join("; ")}` });
    }

    // UI-117: Back with the «Lên plan» demo's options sheet open, then three taps, each from a fresh Back.
    if (chayPhan("r-f11", "117")) {
      const moSheet = async () => {
        const t = await mo("C1", null, "/explore");
        const cdp = await cdpCua(t.page);
        await t.page.waitForTimeout(3000);
        const tab = await t.page.evaluate(() => {
          const e = [...document.querySelectorAll('[role="tab"],[role="link"],a[href]')].find((x) => /Lên plan/.test(x.getAttribute("aria-label") ?? x.innerText ?? ""));
          if (!e) return null;
          const r = e.getBoundingClientRect();
          return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
        });
        if (tab) await cham(cdp, tab);
        await t.page.waitForTimeout(2500);
        const nut = await tamCua(t.page, '[aria-label="Tùy chọn"]');
        if (nut) await cham(cdp, nut);
        const moRa = await choDialog(t.page);
        await t.page.waitForTimeout(700);
        await dong(t.page, cdp, "back");
        await t.page.waitForTimeout(1200);
        const duong = t.page.url().startsWith("about:") ? "about:blank" : await duongDan(t.page);
        return { t, cdp, moRa: moRa.ok, duong, inert: (await inertConLai(t.page)).length };
      };
      // 1. The heart of the first place on Khám phá.
      const a = await moSheet();
      const tim = await a.t.page.evaluate(() => {
        const e = [...document.querySelectorAll('[data-testid="explore-screen"] [aria-label^="Lưu "]')].find((x) => { const r = x.getBoundingClientRect(); return r.top > 0 && r.bottom < innerHeight - 90; });
        if (!e) return null;
        const r = e.getBoundingClientRect();
        return { x: r.left + r.width / 2, y: r.top + r.height / 2, ten: e.getAttribute("aria-label") };
      });
      if (tim) await cham(a.cdp, tim);
      await a.t.page.waitForTimeout(1200);
      const doi = tim ? await a.t.page.evaluate((ten) => !!document.querySelector(`[aria-label="${ten.replace(/^Lưu /, "Bỏ lưu ")}"]`), tim.ten) : null;
      await anh(a.t.page, "EV-R-UI-117-C1", a.t);
      await a.t.context.close();
      // 2. The «Bánh căn Lệ» card, where the sheet's «Tường nhóm» lay under it (or its centre if no sheet is left).
      const b = await moSheet();
      const diem = await b.t.page.evaluate(() => {
        const s = [...document.querySelectorAll('[role="dialog"] [role="button"],[role="dialog"] button')].find((x) => /Tường nhóm/.test(x.getAttribute("aria-label") || x.innerText || ""));
        if (s) { const r = s.getBoundingClientRect(); return { x: r.left + r.width * 0.25, y: r.top + r.height / 2, tu: "chỗ «Tường nhóm» của sheet" }; }
        const e = [...document.querySelectorAll('[data-testid="explore-screen"] [role="button"],[data-testid="explore-screen"] a[href]')].find((x) => /Bánh căn Lệ/.test(x.getAttribute("aria-label") || x.innerText || "") && !/^(Lưu|Bỏ lưu)/.test(x.getAttribute("aria-label") || ""));
        if (!e) return null;
        e.scrollIntoView({ block: "center" });
        const r = e.getBoundingClientRect();
        return { x: r.left + r.width / 2, y: r.top + Math.min(r.height / 2, 40), tu: "giữa thẻ" };
      });
      const thay = diem
        ? await b.t.page.evaluate(({ x, y }) => {
            const man = document.querySelector('[data-testid="explore-screen"]');
            if (!man) return null;
            let tot = null;
            for (const e of man.querySelectorAll("*")) {
              const r = e.getBoundingClientRect();
              if (x < r.left || x > r.right || y < r.top || y > r.bottom) continue;
              const chu = (e.getAttribute("aria-label") || e.innerText || "").replace(/\s+/g, " ").trim();
              if (!chu) continue;
              const s = r.width * r.height;
              if (!tot || s < tot.s) tot = { s, chu: chu.slice(0, 50) };
            }
            return tot?.chu ?? null;
          }, diem)
        : null;
      if (diem) await cham(b.cdp, diem);
      await b.t.page.waitForTimeout(1800);
      const denThe = await duongDan(b.t.page);
      await b.t.context.close();
      // 3. The «Lên plan» tab.
      const c = await moSheet();
      const tabPlan = await c.t.page.evaluate(() => {
        const e = [...document.querySelectorAll('[role="tab"]')].find((x) => x.getAttribute("aria-label") === "Lên plan" && x.getClientRects().length);
        if (!e) return null;
        const r = e.getBoundingClientRect();
        return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
      });
      const truocTab = { duong: await duongDan(c.t.page), hop: await demDialog(c.t.page) };
      if (tabPlan) await cham(c.cdp, tabPlan);
      await c.t.page.waitForTimeout(1800);
      const sauTab = { duong: await duongDan(c.t.page), hop: await demDialog(c.t.page) };
      await c.t.context.close();
      const theDung = /^\/places\//.test(denThe) && (!thay || /Bánh căn Lệ/.test(thay));
      const tabDung = sauTab.duong !== truocTab.duong || sauTab.hop !== truocTab.hop;
      const dat = a.moRa && a.inert === 0 && doi === true && theDung && tabDung;
      // Back from a tab reached through the tab bar can leave the app on main (UI-123): then there is no Khám phá to tap.
      const roiApp = !a.duong || !a.duong.startsWith("/");
      ket({ feature: "F11", issue: "UI-117", phan: "A", screen: "F11.S01", layer: "L28", state: "chưa đăng nhập; từ Khám phá chạm tab «Lên plan», mở «Tùy chọn»", action: "Back trình duyệt; rồi, mỗi lần từ một lượt mới: chạm tim quán đầu, chạm thẻ «Bánh căn Lệ», chạm tab «Lên plan»", cauHinh: "C1", expected: "sau Back không còn vùng inert nào ngoài hộp thoại; chạm tim lần đầu đổi thành «Bỏ lưu»; chạm thẻ quán mở đúng quán; thanh tab phản hồi", status: roiApp ? "BLOCKED" : dat ? "PASS" : "FAIL", evidence: ["EV-R-UI-117-C1"], ghiChu: `${!a.moRa ? "đổi: không mở được sheet «Tùy chọn»" : roiApp ? "đổi: Back rời app (UI-123), không tới được Khám phá nên tiêu chí không đo được trên đường này; xem TC-R-UI-117-B" : dat ? "hết" : "còn"}: sau Back ở ${a.duong}, vùng inert ≥25% màn ${a.inert}; tim «${tim?.ten ?? "-"}» ${doi === null ? "không thấy" : doi ? "đổi thành «Bỏ lưu»" : "không đổi"}; chạm ${diem?.tu ?? "-"} (người dùng thấy «${thay ?? "-"}») → ${anId(denThe)}; tab «Lên plan»: ${truocTab.duong} ${truocTab.hop} hộp → ${sauTab.duong} ${sauTab.hop} hộp` });
    }

    // UI-117 by the one way Back still lands inside the app on main: Cộng đồng (the first tab) → Khám phá → Lên plan.
    if (chayPhan("r-f11", "117b")) {
      const t = await mo("C1", null, "/community");
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(3000);
      const tab = (ten) =>
        t.page.evaluate((ten) => {
          const e = [...document.querySelectorAll('[role="tab"]')].find((x) => x.getAttribute("aria-label") === ten && x.getClientRects().length);
          if (!e) return null;
          let khoa = false;
          for (let n = e; n; n = n.parentElement) if (n.inert) khoa = true;
          const r = e.getBoundingClientRect();
          return { x: r.left + r.width / 2, y: r.top + r.height / 2, khoa };
        }, ten);
      const chuoi = [];
      for (const ten of ["Khám phá", "Lên plan"]) {
        const n = await tab(ten);
        if (n) await cham(cdp, n);
        await t.page.waitForTimeout(2200);
        chuoi.push(`${ten} → ${await duongDan(t.page)}`);
      }
      const nut = await tamCua(t.page, '[aria-label="Tùy chọn"]');
      if (nut) await cham(cdp, nut);
      const moRa = (await choDialog(t.page)).ok;
      await t.page.waitForTimeout(700);
      await dong(t.page, cdp, "back");
      await t.page.waitForTimeout(1500);
      const sauBack = t.page.url().startsWith("about:") ? "about:blank" : await duongDan(t.page);
      const inert = (await inertConLai(t.page)).length;
      const hop = await demDialog(t.page);
      await anh(t.page, "EV-R-UI-117-B-C1", t);
      // The tab bar after that Back: does «Khám phá» answer?
      const kp = await tab("Khám phá");
      if (kp) await cham(cdp, kp);
      await t.page.waitForTimeout(1800);
      const sauTab = t.page.url().startsWith("about:") ? "about:blank" : await duongDan(t.page);
      await t.context.close();
      const dat = moRa && sauBack.startsWith("/") && inert === 0 && sauTab === "/explore";
      ket({ feature: "F11", issue: "UI-117", phan: "B", screen: "F11.S01", layer: "L28", state: "chưa đăng nhập; mở /community, chạm tab «Khám phá» rồi «Lên plan», mở «Tùy chọn»", action: "Back trình duyệt; rồi chạm tab «Khám phá»", cauHinh: "C1", expected: "sau Back không còn vùng inert nào ngoài hộp thoại; thanh tab phản hồi", status: !moRa ? "BLOCKED" : dat ? "PASS" : "FAIL", evidence: ["EV-R-UI-117-B-C1"], ghiChu: `${!moRa ? "đổi: không mở được sheet «Tùy chọn»" : dat ? "hết" : "còn"}: ${chuoi.join(", ")}; sheet mở: ${moRa ? "có" : "không"}; sau Back ở ${sauBack}, ${hop} hộp thoại thấy được, vùng inert ≥25% màn ${inert}, tab «Khám phá» ${kp ? (kp.khoa ? "nằm trong vùng inert" : "không inert") : "không thấy"}; chạm tab «Khám phá» → ${sauTab}` });
    }

    // UI-082, the demo half: a label and a way in on every demo screen.
    if (chayPhan("r-f11", "082")) {
      const MAN = [["/community", "Cộng đồng"], ["/explore", "Khám phá"], ["/plan", "Lên plan"], ["/messages", "Tin nhắn"], ["/profile", "Cá nhân"], ["/trips/team-da-lat/itinerary", "Lịch trình AI"], ["/smart-split/team-da-lat/assignment", "Ai dùng món nào?"]];
      const kq = [];
      for (const [path, ten] of MAN) {
        const t = await mo("C1", null, path);
        await t.page.waitForTimeout(3500);
        const den = await duongDan(t.page);
        const m = await t.page.evaluate(() => {
          const hien = (x) => x.getClientRects().length > 0 && (x.checkVisibility ? x.checkVisibility({ visibilityProperty: true, opacityProperty: true }) : true);
          const ten = (x) => (x.getAttribute("aria-label") || (x.innerText ?? "")).replace(/[-]/g, "").replace(/\s+/g, " ").trim();
          const la = [...document.querySelectorAll("div,span")].filter((e) => hien(e) && [...e.childNodes].some((n) => n.nodeType === 3 && n.textContent.trim()));
          const nhan = [...new Set(la.map((e) => e.textContent.trim()).filter((x) => /^(Dữ liệu demo|Demo|Nháp)$/.test(x)))];
          const nut = [...document.querySelectorAll('[role="button"],button,[role="link"],a[href]')].filter(hien).map(ten).filter(Boolean);
          const vao = [...new Set(nut.filter((x) => /Đăng nhập|Rủ Đi thôi|Bắt đầu|Vào app/i.test(x)))];
          return { nhan, vao };
        });
        if (path === "/messages") await anh(t.page, "EV-R-UI-082-F11-C1", t);
        kq.push({ ten, path, den, ...m, cua: /^\/(welcome|login)/.test(den) });
        await t.context.close();
      }
      const demo = kq.filter((k) => !k.cua);
      const thieu = demo.filter((k) => !k.nhan.some((x) => x !== "Nháp") || !k.vao.length);
      ket({ feature: "F11", issue: "UI-082", phan: "F11", screen: "F11.S01", state: "chưa đăng nhập", action: "mở thẳng năm tab và hai route demo «Lịch trình AI», «Ai dùng món nào?»", cauHinh: "C1", expected: "F11: mọi màn demo có nhãn «Dữ liệu demo» và một lối «Đăng nhập» nhìn thấy được", status: thieu.length === 0 ? "PASS" : "FAIL", evidence: ["EV-R-UI-082-F11-C1"], ghiChu: `${thieu.length === 0 ? "hết" : "còn"}: ${kq.map((k) => `${k.ten} → ${k.den}${k.cua ? " (cửa vào)" : `, nhãn ${k.nhan.join("/") || "không"}, lối vào ${k.vao.join("/") || "không"}`}`).join("; ")}` });
    }
  }

  // ------------------------------------------------------------------ E
  if (coPhan("r-e")) {
    const pa = await phienCua("chat-0");
    const G = ((await api("GET", "/people/me/contexts", undefined, pa)).contexts ?? []).find((c) => c.kind === "group");

    // UI-119: «Tôi đã tới» at a stop of «Kèo album retest», then the group's album and wall.
    if (chayPhan("r-e", "119")) {
      const KEO = "Kèo album retest";
      const QUAN = "Lưng Chừng Cafe";
      const keoCua = async () => ((await api("GET", `/contexts/${G.id}/outings`, undefined, pa)).outings ?? []).find((o) => o.title === KEO);
      let keoE = await keoCua();
      if (!keoE) throw new Error("chưa có «Kèo album retest»; chạy --chi r-f08 trước");
      const coChang = (k) => (k?.stops ?? []).some((s) => (s.place_name ?? s.label ?? "").includes(QUAN));
      const buoc = [];
      if (!coChang(keoE)) {
        const t = await mo("C1", "chat-0", "/explore");
        const cdp = await cdpCua(t.page);
        await t.page.waitForTimeout(2500);
        await choOn(t.page, { mang: t.mang });
        const the = await t.page.evaluate((ten) => {
          const e = [...document.querySelectorAll('[role="button"],button,[role="link"],a[href]')].find((x) => { const n = (x.getAttribute("aria-label") || x.innerText || "").replace(/\s+/g, " "); return n.includes(ten) && !/^(Lưu|Bỏ lưu)/.test(n) && x.getBoundingClientRect().width > 0; });
          if (!e) return null;
          e.scrollIntoView({ block: "center", behavior: "instant" });
          const r = e.getBoundingClientRect();
          return { x: r.left + r.width / 2, y: r.top + Math.min(r.height / 2, 40) };
        }, QUAN);
        if (the) await cham(cdp, the);
        await t.page.waitForTimeout(2000);
        const them = await timNut(t.page, "Thêm vào kèo");
        if (them) await cham(cdp, them);
        await t.page.waitForTimeout(2500);
        const vao = await t.page.evaluate((ten) => {
          const nutThem = (e) => [...e.querySelectorAll('[role="button"]')].filter((x) => (x.innerText ?? "").trim() === "Thêm vào");
          const hang = [...document.querySelectorAll("div")].filter((e) => (e.innerText ?? "").includes(ten) && nutThem(e).length === 1).sort((a, b) => a.getBoundingClientRect().width * a.getBoundingClientRect().height - b.getBoundingClientRect().width * b.getBoundingClientRect().height)[0];
          const b = hang ? nutThem(hang)[0] : null;
          if (!b) return null;
          b.scrollIntoView({ block: "center", behavior: "instant" });
          const r = b.getBoundingClientRect();
          return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
        }, KEO);
        if (vao) await cham(cdp, vao);
        await t.page.waitForTimeout(3000);
        buoc.push(`thêm «${QUAN}» từ Khám phá: thẻ ${the ? "có" : "không"}, «Thêm vào kèo» ${them ? "có" : "không"}, «Thêm vào» ${vao ? "có" : "không"}`);
        await t.context.close();
        keoE = await keoCua();
      }
      const checkin = async () => (await api("GET", `/outings/${keoE.id}/checkins`, undefined, pa)).checkins ?? [];
      if (coChang(keoE) && !(await checkin()).length) {
        const t = await mo("C1", "chat-0", "/messages");
        const cdp = await cdpCua(t.page);
        await t.page.waitForTimeout(1500);
        await diToi(t, `/outings/${keoE.id}`);
        const toi = await t.page.evaluate((ten) => {
          const hop = [...document.querySelectorAll('[aria-label^="Chặng "][role="button"]')].find((e) => e.getAttribute("aria-label").includes(ten))?.getBoundingClientRect();
          const tatCa = [...document.querySelectorAll('[role="button"]')].filter((e) => (e.innerText ?? "").trim() === "Tôi đã tới" && e.getBoundingClientRect().width > 0);
          const gan = tatCa.map((e) => ({ e, r: e.getBoundingClientRect() })).sort((a, b) => Math.abs(a.r.top - (hop?.top ?? 0)) - Math.abs(b.r.top - (hop?.top ?? 0)))[0];
          if (!gan) return null;
          gan.e.scrollIntoView({ block: "center", behavior: "instant" });
          const r = gan.e.getBoundingClientRect();
          return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
        }, QUAN);
        if (toi) await cham(cdp, toi);
        await t.page.waitForTimeout(2500);
        buoc.push(`«Tôi đã tới»: ${toi ? "chạm" : "không thấy nút"}`);
        await t.context.close();
      }
      const soCheckin = (await checkin()).length;
      const t = await mo("C1", "chat-0", "/messages");
      await t.page.waitForTimeout(1500);
      await diToi(t, `/groups/${G.id}/album`);
      const chu = await chuTrang(t.page);
      const daToi = (chu.match(/(\d+) chỗ đã tới/) ?? [null, null])[1];
      await anh(t.page, "EV-R-UI-119-C1", t);
      await diToi(t, `/groups/${G.id}/wall`);
      const tuongCo = (await chuTrang(t.page)).includes(QUAN);
      await t.context.close();
      const dat = soCheckin >= 1 && (Number(daToi) >= 1 || tuongCo);
      ket({ feature: "E2", issue: "UI-119", screen: "E2", state: `nhóm chat-test, kèo «${KEO}» có chặng «${QUAN}»: ${coChang(keoE) ? "có" : "không"}; check-in trên máy chủ: ${soCheckin}`, action: "«Tôi đã tới» ở chặng; mở album và tường của nhóm", cauHinh: "C1", expected: "sau «Tôi đã tới» ở một chặng: album của kèo ghi ít nhất «1 chỗ đã tới», hoặc tường có dấu của lần tới đó", status: dat ? "PASS" : "FAIL", evidence: ["EV-R-UI-119-C1"], ghiChu: `${soCheckin < 1 ? "đổi: chưa có check-in để đo" : dat ? "hết" : "còn"}: ${buoc.join("; ") || "chặng và check-in có từ lượt trước"}; album ghi «${daToi ?? "?"} chỗ đã tới»; tường có tên quán: ${tuongCo ? "có" : "không"}` });
    }

    // UI-121: a cold link to the group chat, then signing in from there (chat-3, a member).
    if (chayPhan("r-e", "121")) {
      const duong = `/groups/${G.id}/chat`;
      const t = await mo("C1", null, duong);
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(3000);
      const den0 = await duongDan(t.page);
      if (den0 === "/welcome") {
        const cta = await tamCua(t.page, '[data-testid="welcome-cta"]');
        if (cta) await cham(cdp, cta);
        await t.page.waitForURL("**/login", { timeout: 10_000 }).catch(() => undefined);
      }
      const denLogin = await duongDan(t.page);
      await t.page.locator('input[aria-label="Ô số điện thoại"]').fill(P("chat-3").phone);
      await cham(cdp, await tamCua(t.page, '[data-testid="login-gui-ma"]'));
      await t.page.waitForURL("**/otp", { timeout: 10_000 }).catch(() => undefined);
      await t.page.keyboard.type("000000", { delay: 40 });
      await t.page.waitForTimeout(4500);
      await choOn(t.page, { mang: t.mang });
      const den = await duongDan(t.page);
      await anh(t.page, "EV-R-UI-121-C1", t);
      await t.context.close();
      ket({ feature: "E6", issue: "UI-121", screen: "E6", state: "không phiên, mở link chat nhóm chat-test", action: "từ cửa vào, đăng nhập chat-3 bằng OTP qua UI", cauHinh: "C1", expected: "không phiên, mở link chat nhóm, đăng nhập: tới đúng /groups/<id>/chat", status: den === duong ? "PASS" : "FAIL", evidence: ["EV-R-UI-121-C1"], ghiChu: `${den === duong ? "hết" : "còn"}: link mở ở ${anId(den0)}, qua ${anId(denLogin)}; sau mã tới ${anId(den)}` });
    }
  }

  // ------------------------------------------------------------------ new on main
  // UI-123: which history entry a tab switch leaves. The bar's first tab is now
  // Cộng đồng while a session still lands on Khám phá; the router pushes only
  // when the tab navigator's own history grows, and with the default
  // backBehavior «firstRoute» it holds [first tab, current tab].
  if (coPhan("r-moi") && chayPhan("r-moi", "123")) {
    const chuoi = async (persona, batDau, tabs) => {
      const t = await mo("C1", persona, batDau);
      const cdp = await cdpCua(t.page);
      await t.page.waitForTimeout(3000);
      const doc = () => t.page.evaluate(() => `${location.pathname}#${history.length}`);
      const tien = [await doc()];
      for (const ten of tabs) {
        const n = await t.page.evaluate((ten) => {
          const e = [...document.querySelectorAll('[role="tab"]')].find((x) => x.getAttribute("aria-label") === ten && x.getClientRects().length);
          if (!e) return null;
          const r = e.getBoundingClientRect();
          return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
        }, ten);
        if (n) await cham(cdp, n);
        await t.page.waitForTimeout(1800);
        tien.push(`${ten} → ${await doc()}`);
      }
      await t.page.goBack().catch(() => null);
      await t.page.waitForTimeout(1500);
      const url = t.page.url();
      const back = url.startsWith("about:") ? "about:blank" : new URL(url).origin === new URL(mt.base).origin ? new URL(url).pathname : url;
      await t.context.close();
      return { tien, back };
    };
    const a = await chuoi("dalat-0", "/explore", ["Lên plan"]);
    const b = await chuoi(null, "/explore", ["Lên plan"]);
    const c = await chuoi("dalat-0", "/community", ["Khám phá", "Lên plan"]);
    // Inside the app means a route of the app, not the harness's first page (/favicon.ico) or about:blank.
    const trongApp = (x) => /^\/(community|explore|plan|messages|profile)/.test(x.back);
    ket({ moi: true, feature: "F00", issue: "UI-123", screen: "F00.S03", layer: "L05", state: "có phiên (dalat-0) và không phiên; app mở ở Khám phá", action: "chạm tab «Lên plan», rồi Back trình duyệt; đối chứng: bắt đầu ở Cộng đồng, qua Khám phá rồi Lên plan", cauHinh: "C1", expected: "Back sau khi chuyển tab về tab trước (hoặc ít nhất về một màn của app), không rời app", status: trongApp(a) && trongApp(b) ? "PASS" : "FAIL", evidence: [], ghiChu: `có phiên: ${a.tien.join(", ")}; Back → ${a.back}. Không phiên: ${b.tien.join(", ")}; Back → ${b.back}. Bắt đầu ở tab đầu: ${c.tien.join(", ")}; Back → ${c.back}. Số sau «#» là history.length` });
  }

  // ------------------------------------------------------------------ F10 (the dev board)
  // Needs the dev server of main with fixtures: AUDIT_BASE=<its URL> instead of AUDIT_WEB.
  // The production export sends /dev/* to /welcome: without a dev server there is nothing to measure, and no row is written.
  if (coPhan("r-f10") && chayPhan("r-f10", "113") && !process.env.AUDIT_BASE) console.log("r-f10: cần AUDIT_BASE (server dev có fixture), bỏ qua");
  else if (coPhan("r-f10") && chayPhan("r-f10", "113")) {
    const kq = [];
    for (const cfg of ["C1", "C4", "C2"]) {
      const t = await mo(cfg, null, "/dev/ui-lab");
      const cdp = await cdpCua(t.page);
      // The first request bundles the app: wait for the chip rather than a fixed time.
      // Icon glyphs are private-use characters inside a chip's text: drop them before comparing names.
      await t.page.waitForFunction(() => [...document.querySelectorAll('[role="button"],[role="checkbox"],[role="radio"]')].some((e) => (e.getAttribute("aria-label") || e.innerText || "").replace(/[\uE000-\uF8FF]/g, "").trim() === "Không ảnh"), null, { timeout: 240_000 }).catch(() => undefined);
      const chip = await t.page.evaluate(() => {
        const e = [...document.querySelectorAll('[role="button"],[role="checkbox"],[role="radio"]')].find((x) => (x.getAttribute("aria-label") || x.innerText || "").replace(/[\uE000-\uF8FF]/g, "").trim() === "Không ảnh" && x.getClientRects().length);
        if (!e) return null;
        e.scrollIntoView({ block: "center" });
        const r = e.getBoundingClientRect();
        return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
      });
      if (chip) await cham(cdp, chip);
      await t.page.waitForTimeout(700);
      await t.page.evaluate(() => document.querySelector('[data-testid="lab-compare"]')?.scrollIntoView({ block: "center" }));
      await t.page.waitForTimeout(600);
      const tim = await t.page.evaluate(() =>
        [...document.querySelectorAll('[data-testid="lab-compare"] [aria-label^="Lưu "],[data-testid="lab-compare"] [aria-label^="Bỏ lưu "]')].map((e) => {
          const r = e.getBoundingClientRect();
          // What stays visible after every clipping ancestor and the window.
          let l = r.left;
          let rr = r.right;
          for (let n = e.parentElement; n && n !== document.body; n = n.parentElement) {
            if (/(hidden|clip)/.test(getComputedStyle(n).overflowX)) {
              const b = n.getBoundingClientRect();
              l = Math.max(l, b.left);
              rr = Math.min(rr, b.right);
            }
          }
          rr = Math.min(rr, innerWidth);
          return { ten: e.getAttribute("aria-label"), l: Math.round(r.left), r: Math.round(r.right), w: Math.round(r.width), hien: Math.round(Math.max(0, rr - l)) };
        }),
      );
      if (cfg === "C2") await anh(t.page, "EV-R-UI-113-C2", t);
      kq.push({ cfg, chip: !!chip, tim });
      await t.context.close();
    }
    const c2 = kq.find((k) => k.cfg === "C2");
    const dat = !!c2 && c2.chip && c2.tim.length === 2 && c2.tim.every((x) => x.hien >= x.w - 1);
    ket({ feature: "F02", issue: "UI-113", screen: "F10.S01", state: "bảng dev /dev/ui-lab, mục «Khám phá · renderer live», chip «Không ảnh»; «Still Cafe» mang dấu", action: "đo hai nút «Lưu …» của cặp so sánh", cauHinh: "C1,C4,C2", expected: "ở 320dp, cả hai nút «Lưu …» của cặp so sánh không ảnh nằm trọn trong ô và trong màn", status: dat ? "PASS" : "FAIL", evidence: ["EV-R-UI-113-C2"], ghiChu: `${!c2?.chip ? "đổi: không thấy chip «Không ảnh»" : dat ? "hết" : "còn"}: ${kq.map((k) => `${k.cfg} ${k.tim.map((x) => `«${x.ten}» ${x.w}px ở ${x.l}–${x.r}, thấy ${x.hien}px`).join(", ") || "không thấy nút"}`).join("; ")}` });
  }
} finally {
  await mt.browser.close().catch(() => undefined);
}
process.exit(0);
