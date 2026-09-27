/* Automatic measurements taken on every capture.
 *
 * These are SIGNALS, not findings. Each one points a human at a place in a
 * screenshot; nothing becomes an issue until the screenshot has been opened
 * and the signal adjudicated (report §B). The self-check (tu-kiem.mjs) proves
 * each detector goes red on a planted defect and stays green on a clean page.
 *
 * The occlusion sampling follows apps/mobile/tools/che-chu.mjs: ask the
 * browser's own hit-testing (`elementsFromPoint`) what sits on top of the
 * middle line of a text, rather than doing box arithmetic that cannot see
 * stacking contexts, transforms or clipping. Unlike che-chu it does NOT scroll
 * anything into view: a scan describes the screen as the person sees it at
 * that moment, and moving the page would make the screenshot and the metrics
 * disagree.
 *
 * Runs inside the page, so `doTrang` may not close over anything here.
 */

function doTrang(opts) {
  const nguong = opts?.nguongBam ?? 48;
  const vw = innerWidth;
  const vh = innerHeight;
  const out = {
    viewport: { w: vw, h: vh },
    duong: location.pathname + location.search,
    tranTrang: {
      html: { sw: document.documentElement.scrollWidth, cw: document.documentElement.clientWidth },
      body: { sw: document.body.scrollWidth, cw: document.body.clientWidth },
    },
    tranNgang: [],
    chuBiCat: [],
    ellipsis: [],
    cheChu: [],
    phuTrongSuot: [],
    vungBamNho: [],
    khongTen: [],
    trungTen: [],
    gachDai: [],
    maLoi: [],
    hopThoai: [],
    inert: [],
    renderer: [],
    nep: null,
    tien: [],
    demChu: 0,
    demTuongTac: 0,
  };

  const csCache = new Map();
  const cs = (el) => {
    let v = csCache.get(el);
    if (!v) {
      v = getComputedStyle(el);
      csCache.set(el, v);
    }
    return v;
  };
  const anCache = new Map();
  const bian = (el) => {
    if (!el || el === document.documentElement) return false;
    if (anCache.has(el)) return anCache.get(el);
    const s = cs(el);
    const v = s.display === "none" || s.visibility === "hidden" || Number(s.opacity) < 0.05 || bian(el.parentElement);
    anCache.set(el, v);
    return v;
  };
  const hien = (el) => {
    if (bian(el)) return false;
    const r = el.getBoundingClientRect();
    return r.width > 0.5 && r.height > 0.5;
  };
  const tid = (el) => el?.closest?.("[data-testid]")?.getAttribute("data-testid") ?? null;
  const ngan = (s, n = 70) => {
    const t = String(s ?? "").replace(/\s+/g, " ").trim();
    return t.length > n ? `${t.slice(0, n - 1)}…` : t;
  };
  const hop = (r) => ({ x: Math.round(r.left), y: Math.round(r.top), w: Math.round(r.width), h: Math.round(r.height) });
  const trongInert = (el) => !!el.closest("[inert]");
  const tenTruyCap = (el) => {
    const aria = el.getAttribute("aria-label");
    if (aria && aria.trim()) return aria.trim();
    const by = el.getAttribute("aria-labelledby");
    if (by) {
      const t = by
        .split(/\s+/)
        .map((id) => document.getElementById(id)?.textContent ?? "")
        .join(" ")
        .trim();
      if (t) return t;
    }
    const txt = (el.innerText ?? el.textContent ?? "").trim();
    if (txt) return txt;
    const alt = el.querySelector("img[alt]")?.getAttribute("alt");
    if (alt && alt.trim()) return alt.trim();
    const title = el.getAttribute("title") || el.querySelector("svg title")?.textContent;
    if (title && title.trim()) return title.trim();
    if (el.tagName === "INPUT" || el.tagName === "TEXTAREA") {
      const ph = el.getAttribute("placeholder");
      if (ph) return `(placeholder) ${ph}`;
    }
    return "";
  };

  // --- text leaves -----------------------------------------------------------
  const laChu = (el) => {
    for (const n of el.childNodes) if (n.nodeType === 3 && n.textContent.trim()) return true;
    return false;
  };
  const chu = [];
  for (const el of document.body.querySelectorAll("*")) {
    if (["SCRIPT", "STYLE", "NOSCRIPT", "TITLE", "OPTION"].includes(el.tagName)) continue;
    if (!laChu(el)) continue;
    if (!hien(el)) continue;
    chu.push(el);
  }
  out.demChu = chu.length;

  const cuon = (s, truc) => {
    const v = truc === "x" ? s.overflowX : s.overflowY;
    return v === "auto" || v === "scroll";
  };
  const cat = (s, truc) => {
    const v = truc === "x" ? s.overflowX : s.overflowY;
    return v === "hidden" || v === "clip";
  };

  for (const el of chu) {
    const s = cs(el);
    const r = el.getBoundingClientRect();
    const text = el.textContent ?? "";
    // Own overflow: ellipsis (intended, listed for judgement) or a hard cut.
    const tranX = el.scrollWidth > el.clientWidth + 1;
    const tranY = el.scrollHeight > el.clientHeight + 1;
    if ((tranX || tranY) && (cat(s, "x") || cat(s, "y"))) {
      const coEllipsis = s.textOverflow === "ellipsis" || (s.webkitLineClamp && s.webkitLineClamp !== "none");
      (coEllipsis ? out.ellipsis : out.chuBiCat).push({
        kieu: coEllipsis ? "ellipsis" : "tu-cat",
        text: ngan(text, 90),
        testid: tid(el),
        hop: hop(r),
        thieu: { x: el.scrollWidth - el.clientWidth, y: el.scrollHeight - el.clientHeight },
      });
    }
    // Clipped by an ancestor that cannot scroll on that axis. Only a PARTIAL
    // cut counts: text wholly outside its clipping box is hidden content (a
    // pager's other slides), not a word with its end missing. The walk stops at
    // the first scroll container, since beyond it every row below the fold is
    // "outside" its ancestors by design.
    for (let a = el.parentElement; a && a !== document.body; a = a.parentElement) {
      const as = cs(a);
      const cx = cat(as, "x");
      const cy = cat(as, "y");
      const cuonDuoc = cuon(as, "x") || cuon(as, "y");
      if (!cx && !cy && !cuonDuoc) continue;
      const ar = a.getBoundingClientRect();
      const giaoNhau = r.left < ar.right && r.right > ar.left && r.top < ar.bottom && r.bottom > ar.top;
      const ngoaiX = cx ? Math.max(0, ar.left - r.left, r.right - ar.right) : 0;
      const ngoaiY = cy ? Math.max(0, ar.top - r.top, r.bottom - ar.bottom) : 0;
      if (giaoNhau && (ngoaiX > 1.5 || ngoaiY > 1.5)) {
        out.chuBiCat.push({
          kieu: "cha-cat",
          text: ngan(text, 90),
          testid: tid(el),
          hop: hop(r),
          cha: tid(a) ?? a.tagName.toLowerCase(),
          thieu: { x: Math.round(ngoaiX), y: Math.round(ngoaiY) },
        });
        break;
      }
      if (cuonDuoc) break;
    }
    // Occlusion, sampled on the text's middle line, only where it is on screen.
    if (r.bottom > 0 && r.top < vh && r.right > 0 && r.left < vw) {
      const y = Math.min(vh - 1, Math.max(0, r.top + r.height / 2));
      let bi = 0;
      let dem = 0;
      let tren = null;
      let trongSuot = 0;
      for (const f of [0.1, 0.3, 0.5, 0.7, 0.9]) {
        const x = r.left + r.width * f;
        if (x < 0 || x >= vw) continue;
        dem++;
        const top = document.elementsFromPoint(x, y)[0];
        if (!top || top === el || el.contains(top) || top.contains(el)) continue;
        // A text field drawn over its own painted placeholder (DESIGN.md: the
        // placeholder is a Text under the input, not a native hint) is the
        // intended stack, not a cover.
        if (top.tagName === "INPUT" || top.tagName === "TEXTAREA") continue;
        // Walk from the covering element up to the common ancestor; any paint
        // on the way makes it an opaque cover.
        let mo = false;
        for (let n = top; n && !n.contains(el); n = n.parentElement) {
          const ns = cs(n);
          const bg = ns.backgroundColor.match(/[\d.]+/g);
          const alpha = bg ? (bg.length === 4 ? Number(bg[3]) : 1) : 0;
          if (alpha > 0.3 || ns.backgroundImage !== "none" || ["IMG", "CANVAS", "SVG", "VIDEO"].includes(n.tagName.toUpperCase())) {
            mo = true;
            break;
          }
        }
        if (mo) {
          bi++;
          tren = tren ?? { testid: tid(top), tag: top.tagName.toLowerCase(), ten: ngan(tenTruyCap(top.closest("button,[role]") ?? top), 40) };
        } else trongSuot++;
      }
      if (bi > 0) out.cheChu.push({ text: ngan(text, 60), testid: tid(el), hop: hop(r), phan: `${bi}/${dem}`, tren });
      else if (trongSuot >= 3) out.phuTrongSuot.push({ text: ngan(text, 60), testid: tid(el), hop: hop(r), phan: `${trongSuot}/${dem}` });
    }
    if (/—/.test(text)) out.gachDai.push({ text: ngan(text, 90), testid: tid(el) });
    if (/\b(undefined|NaN|null)\b|\[object Object\]|Failed to fetch|NetworkError|TypeError|Error:|\b[a-z]+_[a-z]+(_[a-z]+)*\b/.test(text))
      out.maLoi.push({ text: ngan(text, 90), testid: tid(el) });
  }

  // --- interactive elements -------------------------------------------------
  const SEL =
    'button,a[href],input,textarea,select,[role="button"],[role="link"],[role="tab"],[role="checkbox"],[role="switch"],[role="radio"],[role="menuitem"],[role="slider"],[role="adjustable"],[tabindex]:not([tabindex="-1"])';
  const tuongTac = [...document.querySelectorAll(SEL)].filter((el) => hien(el) && !trongInert(el) && el.getAttribute("aria-hidden") !== "true");
  out.demTuongTac = tuongTac.length;
  const demTen = new Map();
  for (const el of tuongTac) {
    const r = el.getBoundingClientRect();
    const ten = tenTruyCap(el);
    if (!ten) out.khongTen.push({ tag: el.tagName.toLowerCase(), role: el.getAttribute("role"), testid: tid(el), hop: hop(r) });
    // Nested interactive: counted once, by the outermost.
    if (el.parentElement?.closest(SEL)) continue;
    const w = r.width;
    const h = r.height;
    if (w < nguong || h < nguong) {
      out.vungBamNho.push({
        ten: ngan(ten, 40),
        testid: tid(el),
        role: el.getAttribute("role") ?? el.tagName.toLowerCase(),
        w: Math.round(w),
        h: Math.round(h),
        duoi44: w < 44 || h < 44,
        duoi24: w < 24 || h < 24,
        hop: hop(r),
      });
    }
    const khoa = ngan(ten, 60);
    if (khoa) demTen.set(khoa, (demTen.get(khoa) ?? 0) + 1);
  }
  out.trungTen = [...demTen.entries()].filter(([, n]) => n > 1).map(([ten, n]) => ({ ten, n }));

  // --- page-level horizontal overflow -----------------------------------------
  for (const el of document.body.querySelectorAll("*")) {
    if (!hien(el)) continue;
    const r = el.getBoundingClientRect();
    if (r.right <= vw + 1 && r.left >= -1) continue;
    // Inside something that scrolls sideways: a carousel, a chip row. Intended.
    let trongCuonNgang = false;
    for (let a = el.parentElement; a && a !== document.body; a = a.parentElement) {
      const as = cs(a);
      if (cuon(as, "x") && a.scrollWidth > a.clientWidth + 1) {
        trongCuonNgang = true;
        break;
      }
      if (cat(as, "x")) {
        const ar = a.getBoundingClientRect();
        if (ar.right <= vw + 1 && ar.left >= -1) {
          trongCuonNgang = true; // clipped before reaching the window edge
          break;
        }
      }
    }
    if (trongCuonNgang) continue;
    const s = cs(el);
    out.tranNgang.push({
      tag: el.tagName.toLowerCase(),
      testid: tid(el),
      text: ngan(el.innerText ?? "", 40),
      hop: hop(r),
      vuot: Math.round(Math.max(r.right - vw, -r.left)),
      position: s.position,
    });
  }
  out.tranNgang.sort((a, b) => b.vuot - a.vuot);
  out.tranNgang = out.tranNgang.slice(0, 25);

  // --- overlays, inert, renderer, Nếp, money --------------------------------
  for (const d of document.querySelectorAll('[role="dialog"],[aria-modal="true"]')) {
    if (!hien(d)) continue;
    out.hopThoai.push({ ten: ngan(d.getAttribute("aria-label") ?? "", 40), testid: tid(d), hop: hop(d.getBoundingClientRect()), modal: d.getAttribute("aria-modal") });
  }
  for (const el of document.querySelectorAll("[inert],[aria-hidden='true']")) {
    if (el.parentElement?.closest("[inert],[aria-hidden='true']")) continue;
    const r = el.getBoundingClientRect();
    if (r.width * r.height < vw * vh * 0.25) continue;
    out.inert.push({ testid: tid(el), inert: el.inert === true, ariaHidden: el.getAttribute("aria-hidden"), hop: hop(r) });
  }
  out.renderer = [...document.querySelectorAll("[data-renderer]")].map((e) => e.getAttribute("data-renderer"));

  const nepEl = document.querySelector('[data-testid="nep-mep"],[data-testid="nep-dia"]');
  const tienEls = chu.filter((el) => /^[-+]?\d{1,3}(\.\d{3})*\s?đ$/.test((el.textContent ?? "").trim()));
  out.tien = tienEls.slice(0, 40).map((el) => ({ text: (el.textContent ?? "").trim(), hop: hop(el.getBoundingClientRect()), testid: tid(el) }));
  if (nepEl && hien(nepEl)) {
    const nr = nepEl.getBoundingClientRect();
    const giao = (r) => r.left < nr.right && r.right > nr.left && r.top < nr.bottom && r.bottom > nr.top;
    const chuBiNepChe = chu.filter((el) => !nepEl.contains(el) && giao(el.getBoundingClientRect()));
    const khoangCachTien = tienEls.map((el) => {
      const r = el.getBoundingClientRect();
      const dx = Math.max(0, r.left - nr.right, nr.left - r.right);
      const dy = Math.max(0, r.top - nr.bottom, nr.top - r.bottom);
      return { text: (el.textContent ?? "").trim(), cach: Math.round(Math.hypot(dx, dy)) };
    });
    out.nep = {
      testid: nepEl.getAttribute("data-testid"),
      hop: hop(nr),
      chuBiChe: chuBiNepChe.slice(0, 10).map((el) => ({ text: ngan(el.textContent, 40), testid: tid(el) })),
      tienGanNhat: khoangCachTien.sort((a, b) => a.cach - b.cach)[0] ?? null,
    };
  }
  return out;
}

export async function doDac(page, opts = {}) {
  return page.evaluate(doTrang, opts);
}

/** Hit-test a 3×4 grid: what a tap at each point would land on. */
function luoiCham() {
  const vw = innerWidth;
  const vh = innerHeight;
  const ket = [];
  for (const fy of [0.15, 0.4, 0.65, 0.9]) {
    for (const fx of [0.2, 0.5, 0.8]) {
      const top = document.elementsFromPoint(vw * fx, vh * fy)[0];
      const nut = top?.closest('button,[role="button"],a[href],[role="link"],[role="tab"],input,textarea');
      ket.push({
        fx,
        fy,
        tag: top?.tagName.toLowerCase() ?? null,
        testid: top?.closest("[data-testid]")?.getAttribute("data-testid") ?? null,
        nut: nut ? (nut.getAttribute("aria-label") || (nut.innerText ?? "").trim()).slice(0, 40) : null,
        trongDialog: !!top?.closest('[role="dialog"]'),
      });
    }
  }
  return ket;
}

export async function luoiChamTrang(page) {
  return page.evaluate(luoiCham);
}

/** One line per signal family, for the console and results.jsonl. */
export function tomTat(kq) {
  const tran = Math.max(0, kq.tranTrang.html.sw - kq.tranTrang.html.cw, kq.tranTrang.body.sw - kq.tranTrang.body.cw);
  return {
    tranTrangPx: tran,
    tranNgang: kq.tranNgang.length,
    chuBiCat: kq.chuBiCat.length,
    ellipsis: kq.ellipsis.length,
    cheChu: kq.cheChu.length,
    phuTrongSuot: kq.phuTrongSuot.length,
    vungBamNho48: kq.vungBamNho.length,
    vungBamNho44: kq.vungBamNho.filter((v) => v.duoi44).length,
    khongTen: kq.khongTen.length,
    trungTen: kq.trungTen.length,
    gachDai: kq.gachDai.length,
    maLoi: kq.maLoi.length,
    hopThoai: kq.hopThoai.length,
    inert: kq.inert.length,
    renderer: [...new Set(kq.renderer)].join("|") || "-",
    nepCheChu: kq.nep?.chuBiChe?.length ?? 0,
    nepCachTien: kq.nep?.tienGanNhat?.cach ?? null,
  };
}
