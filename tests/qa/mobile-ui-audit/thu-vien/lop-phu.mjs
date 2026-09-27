/* The life cycle of one overlay: open → use → close (every way it offers) →
 * gone without residue → reopen.
 *
 * "Gone without residue" is measured, not looked at: the tap grid from
 * do-dac.mjs is taken before opening and after closing, and must land on the
 * same things; no element may stay `inert`/`aria-hidden` over the page; focus
 * must come back to the control that opened it. A dialog that has faded out
 * but still swallows taps shows up as a grid cell that changed.
 *
 * Close methods, in the words of the checklist:
 *   x        the panel's own close button («Đóng bảng» on Sheet)
 *   nen      a tap on the backdrop, above the panel
 *   esc      Escape (the web's accessibility escape; Sheet listens for it)
 *   back     browser Back (the web's system back)
 *   keo-ngan drag the handle 60dp and release slowly: must spring back, stay open
 *   keo-dai  drag the handle 140dp and release slowly: must close
 *   fling    flick the handle 70dp in 60ms: must close (velocity rule)
 */
import { cham, keo } from "./cu-chi.mjs";
import { luoiChamTrang } from "./do-dac.mjs";

const cho = (ms) => new Promise((ok) => setTimeout(ok, ms));

export async function demDialog(page) {
  return page.evaluate(
    () =>
      [...document.querySelectorAll('[role="dialog"]')].filter((d) => {
        const r = d.getBoundingClientRect();
        let op = 1;
        for (let n = d; n && n !== document.documentElement; n = n.parentElement) op *= Number(getComputedStyle(n).opacity);
        return r.width > 0 && r.height > 0 && op > 0.05 && getComputedStyle(d).visibility !== "hidden";
      }).length,
  );
}

export async function choDialog(page, { co = true, toiDa = 4000 } = {}) {
  const t0 = Date.now();
  while (Date.now() - t0 < toiDa) {
    const n = await demDialog(page);
    if (co ? n > 0 : n === 0) return { ok: true, ms: Date.now() - t0, n };
    await cho(40);
  }
  return { ok: false, ms: Date.now() - t0, n: await demDialog(page) };
}

export async function focusHienTai(page) {
  return page.evaluate(() => {
    const a = document.activeElement;
    if (!a || a === document.body) return { ten: null, trongDialog: false, tag: a?.tagName?.toLowerCase() ?? null };
    return {
      ten: (a.getAttribute("aria-label") || (a.innerText ?? "").trim()).slice(0, 50),
      trongDialog: !!a.closest('[role="dialog"]'),
      tag: a.tagName.toLowerCase(),
      testid: a.closest("[data-testid]")?.getAttribute("data-testid") ?? null,
    };
  });
}

export async function inertConLai(page) {
  return page.evaluate(() =>
    [...document.querySelectorAll("[inert],[aria-hidden='true']")]
      .filter((e) => {
        const r = e.getBoundingClientRect();
        return r.width * r.height > innerWidth * innerHeight * 0.25 && !e.closest('[role="dialog"]');
      })
      .map((e) => ({ inert: e.inert === true, ariaHidden: e.getAttribute("aria-hidden"), testid: e.getAttribute("data-testid") })),
  );
}

async function hopDialog(page) {
  return page.evaluate(() => {
    const ds = [...document.querySelectorAll('[role="dialog"]')];
    const d = ds[ds.length - 1];
    if (!d) return null;
    const r = d.getBoundingClientRect();
    const tay = d.querySelector('[aria-label="Tay cầm"]')?.getBoundingClientRect();
    const x = [...d.querySelectorAll('[aria-label="Đóng bảng"],[aria-label="Đóng"]')].pop()?.getBoundingClientRect();
    return {
      top: r.top,
      bottom: r.bottom,
      left: r.left,
      right: r.right,
      tay: tay ? { x: tay.left + tay.width / 2, y: tay.top + tay.height / 2 } : null,
      nutX: x ? { x: x.left + x.width / 2, y: x.top + x.height / 2, top: x.top } : null,
    };
  });
}

export async function dong(page, cdp, cach) {
  const h = await hopDialog(page);
  if (!h) return { lam: false, ly: "không có dialog" };
  switch (cach) {
    case "x":
      if (!h.nutX) return { lam: false, ly: "không thấy nút đóng" };
      await cham(cdp, h.nutX);
      break;
    case "nen": {
      const y = Math.max(8, h.top / 2);
      if (h.top < 24) return { lam: false, ly: `panel chạm mép trên (top=${Math.round(h.top)}), không còn nền để chạm` };
      await cham(cdp, { x: (h.left + h.right) / 2, y });
      break;
    }
    case "esc":
      await page.keyboard.press("Escape");
      break;
    case "back":
      await page.goBack({ waitUntil: "commit" }).catch(() => undefined);
      break;
    case "keo-ngan":
    case "keo-dai":
    case "fling": {
      if (!h.tay) return { lam: false, ly: "không thấy tay cầm" };
      const dy = cach === "keo-ngan" ? 60 : cach === "keo-dai" ? 140 : 70;
      const ms = cach === "fling" ? 60 : 700;
      await keo(cdp, h.tay, { x: h.tay.x, y: h.tay.y + dy }, { ms, buoc: cach === "fling" ? 4 : 18, giuCuoiMs: cach === "fling" ? 0 : 150 });
      break;
    }
    default:
      throw new Error(`cách đóng lạ: ${cach}`);
  }
  return { lam: true, hop: h };
}

/** Compare the tap grid before opening with the grid after closing. */
export function soLuoi(truoc, sau) {
  const khac = [];
  for (let i = 0; i < truoc.length; i++) {
    const a = truoc[i];
    const b = sau[i];
    if (!b) continue;
    if (b.trongDialog || a.testid !== b.testid || a.nut !== b.nut) khac.push({ o: `${a.fx},${a.fy}`, truoc: a, sau: b });
  }
  return khac;
}

export { luoiChamTrang };
