/* Motion evidence that does not depend on how smooth this machine is.
 *
 * Headless Chromium paints through SwiftShader, so frame rate here says nothing
 * about a phone and no number from this file is reported as FPS. What it CAN
 * show is the shape of a transition: when an element appears, the path its box
 * takes, where it ends, its final opacity, whether it jumps backwards, whether
 * a second copy shows up, and whether reduced motion really cuts to the end.
 *
 *   batDauLayMau(selector) → trigger in Node → ketThucLayMau() → phanTich(mau)
 *
 * The sampler re-queries the selector every animation frame, so an element
 * mounted by the trigger is picked up the frame it exists. `n` counts matches
 * per frame: two overlays for one tap shows up as n=2.
 *
 * `quayKhung` records compositor frames through Page.startScreencast for a
 * contact sheet: a picture of the sequence, for a human to read, not a
 * measurement.
 */
import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";

export async function batDauLayMau(page, selector) {
  await page.evaluate((sel) => {
    const w = window;
    w.__auditMau = [];
    w.__auditDung = false;
    const t0 = performance.now();
    const buoc = () => {
      if (w.__auditDung) return;
      const els = document.querySelectorAll(sel);
      const e = els[els.length - 1];
      const t = Math.round(performance.now() - t0);
      if (e) {
        const r = e.getBoundingClientRect();
        let op = 1;
        for (let n = e; n && n !== document.documentElement; n = n.parentElement) op *= Number(getComputedStyle(n).opacity);
        w.__auditMau.push({ t, n: els.length, x: Math.round(r.left), y: Math.round(r.top), w: Math.round(r.width), h: Math.round(r.height), op: Math.round(op * 100) / 100 });
      } else w.__auditMau.push({ t, n: 0 });
      requestAnimationFrame(buoc);
    };
    requestAnimationFrame(buoc);
  }, selector);
}

export async function ketThucLayMau(page) {
  return page.evaluate(() => {
    window.__auditDung = true;
    return window.__auditMau ?? [];
  });
}

/** Summarise a sample series; `nguongOn` px of movement or less counts as still. */
export function phanTich(mau, { nguongOn = 0.5 } = {}) {
  const co = mau.filter((m) => m.n > 0);
  if (!co.length) return { xuatHien: false, soMau: mau.length };
  const dau = co[0];
  const cuoi = co[co.length - 1];
  let tOn = null;
  for (let i = co.length - 1; i > 0; i--) {
    const a = co[i];
    const b = co[i - 1];
    if (Math.abs(a.x - b.x) > nguongOn || Math.abs(a.y - b.y) > nguongOn || Math.abs(a.op - b.op) > 0.01 || a.h !== b.h) {
      tOn = a.t;
      break;
    }
  }
  // Direction reversals on y: a sheet that overshoots once is a spring; one
  // that goes down, up, down again is a jump.
  let doiChieu = 0;
  let huong = 0;
  for (let i = 1; i < co.length; i++) {
    const d = co[i].y - co[i - 1].y;
    if (Math.abs(d) < 1) continue;
    const h = Math.sign(d);
    if (huong && h !== huong) doiChieu++;
    huong = h;
  }
  let khoangLonNhat = 0;
  for (let i = 1; i < mau.length; i++) khoangLonNhat = Math.max(khoangLonNhat, mau[i].t - mau[i - 1].t);
  const bienMat = mau.length && mau[mau.length - 1].n === 0 && co.length > 0;
  return {
    xuatHien: true,
    tXuatHien: dau.t,
    tDungYen: tOn ?? dau.t,
    thoiLuongMs: (tOn ?? dau.t) - dau.t,
    dau: { x: dau.x, y: dau.y, op: dau.op, h: dau.h },
    cuoi: { x: cuoi.x, y: cuoi.y, op: cuoi.op, h: cuoi.h },
    doiChieuY: doiChieu,
    nhieuBanSao: Math.max(...co.map((m) => m.n)),
    khoangKhungLonNhatMs: khoangLonNhat,
    bienMatCuoi: bienMat,
    soMau: mau.length,
  };
}

export async function quayKhung(page, cdp, { ms = 1200, chatLuong = 55, kichThuocToiDa = 480 } = {}) {
  const khung = [];
  const nhan = (e) => {
    khung.push({ data: e.data, t: e.metadata?.timestamp ?? 0 });
    cdp.send("Page.screencastFrameAck", { sessionId: e.sessionId }).catch(() => undefined);
  };
  cdp.on("Page.screencastFrame", nhan);
  await cdp.send("Page.startScreencast", { format: "jpeg", quality: chatLuong, maxWidth: kichThuocToiDa, maxHeight: kichThuocToiDa * 2, everyNthFrame: 1 });
  return {
    async dung() {
      await page.waitForTimeout(ms);
      await cdp.send("Page.stopScreencast").catch(() => undefined);
      cdp.off("Page.screencastFrame", nhan);
      const t0 = khung[0]?.t ?? 0;
      return khung.map((k) => ({ ...k, ms: Math.round((k.t - t0) * 1000) }));
    },
  };
}

// Each frame goes into the sheet page as a data URI built at run time from the
// screencast bytes; nothing encoded is stored in this file.
// repo-guard: allow=data-uri-base64 reason=runtime-encoded-audit-frames
const anhKhung = (k) => `<figure style="margin:0"><img style="width:100%;display:block" src="data:image/jpeg;base64,${k.data}"><figcaption style="text-align:center">${k.ms} ms</figcaption></figure>`;

/** Lay frames out on one page and photograph it: a contact sheet JPEG. */
export async function ghepKhung(browser, khung, duongRa, { cot = 6, toiDa = 24, tieuDe = "" } = {}) {
  mkdirSync(join(duongRa, ".."), { recursive: true });
  const buoc = Math.max(1, Math.ceil(khung.length / toiDa));
  const chon = khung.filter((_, i) => i % buoc === 0).slice(0, toiDa);
  const html = `<!doctype html><meta charset="utf-8"><body style="margin:0;background:#222;color:#eee;font:12px sans-serif">
<div style="padding:6px 8px">${tieuDe.replace(/</g, "&lt;")} · ${khung.length} khung, hiện ${chon.length}</div>
<div style="display:grid;grid-template-columns:repeat(${cot},1fr);gap:4px;padding:4px">
${chon.map(anhKhung).join("")}
</div></body>`;
  const ctx = await browser.newContext({ viewport: { width: 1200, height: 800 }, deviceScaleFactor: 1 });
  const p = await ctx.newPage();
  await p.setContent(html, { waitUntil: "load" });
  const buf = await p.screenshot({ fullPage: true, type: "jpeg", quality: 78 });
  await ctx.close();
  writeFileSync(duongRa, buf);
  return { soKhung: khung.length, hien: chon.length, bytes: buf.length };
}
