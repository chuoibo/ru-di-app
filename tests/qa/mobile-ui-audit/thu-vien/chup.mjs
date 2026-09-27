/* One capture = one screenshot pair + the measurements taken on the same frame.
 *
 * Written per capture, under the run's output directory (outside git):
 *   raw/<id>.png       lossless, for pixel questions
 *   jpg/<id>.jpg       q80, the candidate for committing next to the report
 *   metrics/<id>.json  do-dac signals, page events since the last capture, settle info
 *
 * Phone numbers are masked on EVERY capture, raw included, before the shutter:
 * the seeded numbers are synthetic, but a 09x number can still belong to a
 * real person, and an image in git cannot be grepped by the repo guard. The
 * mask is a labelled box, so nobody mistakes it for a rendering defect.
 *
 * Annotations (`chuThich`) are drawn as fixed, pointer-events:none outlines
 * AFTER the measurements, so they can never be measured as page content, and
 * are removed right after the shutter. The unannotated JPEG is always kept.
 */
import { createHash } from "node:crypto";
import { existsSync, mkdirSync, readFileSync, unlinkSync, writeFileSync } from "node:fs";
import { join } from "node:path";

import { doDac, tomTat } from "./do-dac.mjs";

const sha = (buf) => createHash("sha256").update(buf).digest("hex");

function datLopChe({ che, chuThich }) {
  const lop = document.createElement("div");
  lop.id = "__audit_lop__";
  // Above everything the app draws (its highest zIndex is 2); six digits keep
  // the value clear of the repository guard's long-number rule.
  lop.style.cssText = "position:fixed;inset:0;pointer-events:none;z-index:999999";
  const hop = (r0, css, nhan) => {
    // Clamp to the window: a box around something tucked past the edge (Nếp's
    // slip) would otherwise draw its outline and label off-screen.
    const left = Math.max(0, r0.left);
    const top = Math.max(0, r0.top);
    const r = { left, top, width: Math.max(4, Math.min(innerWidth, r0.left + r0.width) - left), height: Math.max(4, Math.min(innerHeight, r0.top + r0.height) - top) };
    const d = document.createElement("div");
    d.style.cssText = `position:fixed;left:${r.left}px;top:${r.top}px;width:${r.width}px;height:${r.height}px;${css}`;
    if (nhan) {
      const s = document.createElement("span");
      s.textContent = nhan;
      const benPhai = r.left > innerWidth * 0.55;
      const lon = r.height > innerHeight * 0.6;
      const duoi = r.top < 18;
      // A box as large as the screen keeps its label inside, top-left.
      const viTri = lon ? "top:4px;left:4px" : `${benPhai ? "right:0" : "left:0"};${duoi ? "bottom:0;transform:translateY(100%)" : "top:0;transform:translateY(-100%)"}`;
      s.style.cssText = `position:absolute;${viTri};background:#c00;color:#fff;font:600 11px/14px sans-serif;padding:1px 4px;white-space:nowrap`;
      d.appendChild(s);
    }
    lop.appendChild(d);
  };
  if (che) {
    const SO = /(\+84|\b0)(?:[\s.]?\d){8,10}\b/;
    // Only what is actually painted on screen: a screen further down the stack
    // (the Login field under OTP) keeps its value in the DOM at a zero-size or
    // hidden box, and masking that drew a stray label in a corner.
    const thay = (el, r) => {
      if (r.width < 2 || r.height < 2 || r.bottom < 0 || r.right < 0 || r.top > innerHeight || r.left > innerWidth) return false;
      for (let n = el; n && n !== document.documentElement; n = n.parentElement) {
        const s = getComputedStyle(n);
        if (s.display === "none" || s.visibility === "hidden" || Number(s.opacity) < 0.05) return false;
      }
      return true;
    };
    const cheEl = (el, r) => {
      if (thay(el, r)) hop(r, "background:#5b5f6e;border-radius:4px;display:flex;align-items:center", "đã che số");
    };
    for (const el of document.body.querySelectorAll("*")) {
      if (el.id === "__audit_lop__") continue;
      const direct = [...el.childNodes].some((n) => n.nodeType === 3 && SO.test(n.textContent));
      if (direct) cheEl(el, el.getBoundingClientRect());
    }
    for (const el of document.querySelectorAll("input,textarea")) {
      if (SO.test(el.value ?? "")) cheEl(el, el.getBoundingClientRect());
    }
  }
  for (const c of chuThich ?? []) {
    const els = c.selector ? [...document.querySelectorAll(c.selector)] : [];
    const rects = c.rect ? [c.rect] : els.map((e) => e.getBoundingClientRect());
    for (const r of rects) {
      const rr = { left: r.left ?? r.x, top: r.top ?? r.y, width: r.width ?? r.w, height: r.height ?? r.h };
      hop(rr, "outline:3px solid #e00000;outline-offset:-3px;border-radius:3px", c.nhan ?? "");
    }
  }
  document.body.appendChild(lop);
}

function boLopChe() {
  document.getElementById("__audit_lop__")?.remove();
}

export async function chup(page, { out, id, suKien, on, chuThich = [], fullPage = false, doDacOpts = {}, ghiChu = "" }) {
  for (const d of ["raw", "jpg", "metrics"]) mkdirSync(join(out, d), { recursive: true });
  const kq = await doDac(page, doDacOpts);
  const tt = tomTat(kq);
  await page.evaluate(datLopChe, { che: true, chuThich: [] });
  const png = await page.screenshot({ fullPage, type: "png" });
  const jpg = await page.screenshot({ fullPage, type: "jpeg", quality: 80 });
  await page.evaluate(boLopChe);
  let jpgChuThich = null;
  if (chuThich.length) {
    await page.evaluate(datLopChe, { che: true, chuThich });
    jpgChuThich = await page.screenshot({ fullPage, type: "jpeg", quality: 80 });
    await page.evaluate(boLopChe);
  }
  writeFileSync(join(out, "raw", `${id}.png`), png);
  writeFileSync(join(out, "jpg", `${id}.jpg`), jpg);
  // An annotated copy from an earlier run must not outlive this capture: a
  // contact sheet built from it would show boxes this run never drew.
  const duongCt = join(out, "jpg", `${id}-ct.jpg`);
  if (jpgChuThich) writeFileSync(duongCt, jpgChuThich);
  else if (existsSync(duongCt)) unlinkSync(duongCt);
  const meta = {
    id,
    ghiChu,
    duong: kq.duong,
    viewport: kq.viewport,
    on: on ?? null,
    tomTat: tt,
    doDac: kq,
    suKien: suKien ? suKien.xa() : null,
    sha256: { png: sha(png), jpg: sha(jpg), jpgChuThich: jpgChuThich ? sha(jpgChuThich) : null },
    bytes: { png: png.length, jpg: jpg.length, jpgChuThich: jpgChuThich?.length ?? null },
  };
  writeFileSync(join(out, "metrics", `${id}.json`), JSON.stringify(meta, null, 1));
  return meta;
}

/**
 * Side-by-side sheet of several captures (one screen across configurations),
 * each scaled to the same height and labelled. For looking, and for the report.
 */
export async function ghepAnh(browser, anh, duongRa, { cao = 900, tieuDe = "" } = {}) {
  const the = anh
    .map(
      (a) =>
        // Captures are read from the run's output and embedded at run time.
        // repo-guard: allow=data-uri-base64 reason=runtime-encoded-audit-captures
        `<figure style="margin:0;display:flex;flex-direction:column;align-items:center"><figcaption style="font:600 14px sans-serif;padding:4px 0">${a.nhan}</figcaption><img style="height:${cao}px;border:1px solid #888" src="data:image/jpeg;base64,${readFileSync(a.file).toString("base64")}"></figure>`,
    )
    .join("");
  const html = `<!doctype html><meta charset="utf-8"><body style="margin:0;background:#e9e7e1;color:#222"><div style="font:600 15px sans-serif;padding:8px 10px">${tieuDe.replace(/</g, "&lt;")}</div><div style="display:flex;gap:14px;align-items:flex-start;padding:0 10px 10px">${the}</div></body>`;
  const ctx = await browser.newContext({ viewport: { width: 2400, height: cao + 80 }, deviceScaleFactor: 1 });
  const p = await ctx.newPage();
  await p.setContent(html, { waitUntil: "load" });
  // One scale for every capture: the tallest one gets `cao`, the others keep
  // their proportion. Equal heights would blow a short window (C8) up and make
  // its text look twice the size it is.
  await p.evaluate((cao) => {
    const imgs = [...document.querySelectorAll("img")];
    const maxH = Math.max(...imgs.map((i) => i.naturalHeight));
    for (const i of imgs) i.style.height = `${Math.round((cao * i.naturalHeight) / maxH)}px`;
  }, cao);
  const w = await p.evaluate(() => document.querySelector("div:last-child").scrollWidth + 20);
  await p.setViewportSize({ width: Math.min(4000, w), height: cao + 80 });
  const buf = await p.screenshot({ fullPage: true, type: "jpeg", quality: 78 });
  await ctx.close();
  writeFileSync(duongRa, buf);
  return { bytes: buf.length, sha256: sha(buf) };
}

export function docMetrics(out, id) {
  return JSON.parse(readFileSync(join(out, "metrics", `${id}.json`), "utf8"));
}
