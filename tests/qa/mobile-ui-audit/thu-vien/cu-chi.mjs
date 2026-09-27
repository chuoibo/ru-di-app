/* Touch gestures through the DevTools protocol.
 *
 * react-native-gesture-handler on the web listens to Pointer Events, and
 * Chromium derives touch-type pointer events from `Input.dispatchTouchEvent`.
 * A mouse drag would also move a Pan gesture, but it would not exercise the
 * touch path a phone uses, and long-press / pinch need real touch points.
 *
 * TIMESTAMPS ARE EXPLICIT. Measured on 27/09: in headless Chromium with
 * SwiftShader, two dispatched moves land 30–125 ms apart whatever the script
 * waits, so a "fling" arrived at ~200 px/s. RNGH's velocity tracker reads the
 * events' timeStamp and drops samples more than 40 ms apart, so the app saw a
 * slow drag and the audit nearly reported «fling does not close the sheet»
 * about its own input. Every event now carries the timestamp of the gesture
 * being simulated (a virtual clock), and the real clock is only ever made to
 * wait for it, never allowed to define it. Velocity-dependent checks are then
 * about the app; timer-dependent ones (long-press) still get real time.
 */
const cho = (ms) => new Promise((ok) => setTimeout(ok, ms));

export async function cdpCua(page) {
  return page.context().newCDPSession(page);
}

async function gui(cdp, type, points, ts) {
  await cdp.send("Input.dispatchTouchEvent", {
    type,
    touchPoints: points.map((p, i) => ({ x: p.x, y: p.y, id: p.id ?? i, radiusX: 4, radiusY: 4, force: 1 })),
    timestamp: ts,
  });
}

/** A virtual clock anchored at "now": `luc(ms)` is the timestamp `ms` later. */
function dongHo() {
  const goc = Date.now();
  return {
    luc: (ms) => (goc + ms) / 1000,
    async den(ms) {
      const con = goc + ms - Date.now();
      if (con > 0) await cho(con);
    },
  };
}

export async function cham(cdp, p, giuMs = 40) {
  const d = dongHo();
  await gui(cdp, "touchStart", [p], d.luc(0));
  await d.den(giuMs);
  await gui(cdp, "touchEnd", [], d.luc(giuMs));
}

export async function nhanGiu(cdp, p, giuMs = 650) {
  const d = dongHo();
  await gui(cdp, "touchStart", [p], d.luc(0));
  await d.den(giuMs);
  await gui(cdp, "touchEnd", [], d.luc(giuMs));
}

/**
 * Drag from `a` to `b` over `ms`, in `buoc` moves; `giuTruocMs` holds still
 * after touching down (a long-press to pick up), `giuCuoiMs` before lifting.
 */
export async function keo(cdp, a, b, { ms = 400, buoc = 16, giuTruocMs = 0, giuCuoiMs = 0 } = {}) {
  const d = dongHo();
  await gui(cdp, "touchStart", [a], d.luc(0));
  if (giuTruocMs) await d.den(giuTruocMs);
  for (let i = 1; i <= buoc; i++) {
    const t = giuTruocMs + (ms * i) / buoc;
    await d.den(t);
    const f = i / buoc;
    await gui(cdp, "touchMove", [{ x: a.x + (b.x - a.x) * f, y: a.y + (b.y - a.y) * f }], d.luc(t));
  }
  const ketThuc = giuTruocMs + ms + giuCuoiMs;
  await d.den(ketThuc);
  await gui(cdp, "touchEnd", [], d.luc(ketThuc + (giuCuoiMs ? 0 : ms / buoc)));
}

/** Two fingers moving apart (den > tu) or together around `tam`. */
export async function nhip(cdp, tam, { tu = 40, den = 140, ms = 400, buoc = 12 } = {}) {
  const d = dongHo();
  const diem = (k) => [
    { x: tam.x - k / 2, y: tam.y, id: 0 },
    { x: tam.x + k / 2, y: tam.y, id: 1 },
  ];
  await gui(cdp, "touchStart", diem(tu), d.luc(0));
  for (let i = 1; i <= buoc; i++) {
    const t = (ms * i) / buoc;
    await d.den(t);
    await gui(cdp, "touchMove", diem(tu + ((den - tu) * i) / buoc), d.luc(t));
  }
  await gui(cdp, "touchEnd", [], d.luc(ms + ms / buoc));
}

/**
 * Centre of the first element matching `selector`, or null. The element is
 * scrolled to the middle of its scroll container first, as a person would
 * before tapping it: a row half under the floating create button has its
 * centre ON the button, and a tap there measures the button (27/09, the
 * «Cài đặt» row on Cá nhân). Fixed elements do not move.
 */
export async function tamCua(page, selector, { cuon = true } = {}) {
  return page.evaluate(({ sel, cuon }) => {
    const e = document.querySelector(sel);
    if (!e) return null;
    if (cuon) {
      e.scrollIntoView({ block: "center", inline: "nearest", behavior: "instant" });
      // scrollIntoView also scrolls overflow-hidden rows sideways, which no
      // finger can (F04, 27/09: a heading pushed to x = -97 and a clipped Nếp
      // measured as fitting). Put those back; real horizontal scrollers stay.
      for (let n = e.parentElement; n; n = n.parentElement) {
        if (n.scrollLeft && !/(auto|scroll)/.test(getComputedStyle(n).overflowX)) n.scrollLeft = 0;
      }
    }
    const r = e.getBoundingClientRect();
    return { x: r.left + r.width / 2, y: r.top + r.height / 2, w: r.width, h: r.height, top: r.top, bottom: r.bottom };
  }, { sel: selector, cuon });
}
