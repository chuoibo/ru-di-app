/* Browser and context factory for the audit.
 *
 * The browser is found by `timTrinhDuyet()` (tests/qa/tim-trinh-duyet.mjs), so
 * no path to one machine's browser is written here. playwright-core is pinned
 * in package.json to the release whose Chromium is installed; it never
 * downloads one.
 *
 * WebGL: headless Chromium only offers it through SwiftShader, and both the
 * Skia stage (CanvasKit) and maplibre-gl need it. Without these flags the app
 * silently falls back to SVG and the audit would be looking at the fallback
 * while believing it looked at Skia. Every capture records `data-renderer` so
 * the difference cannot pass unnoticed.
 *
 * Fonts: when AUDIT_FONTCONFIG names a fontconfig file, the browser process
 * gets it as FONTCONFIG_FILE. The audit uses that to put Roboto in the font
 * stack react-native-web asks for, so body text is measured with Android's
 * metrics rather than DejaVu Sans, which is wider. Unset, the machine default
 * applies; the report records which one each run used.
 */
import { chromium } from "playwright-core";

import { timTrinhDuyet } from "../../tim-trinh-duyet.mjs";
import { DPR } from "./cau-hinh.mjs";

export const CO_CHROMIUM = Object.freeze([
  "--enable-unsafe-swiftshader",
  "--use-angle=swiftshader",
  "--ignore-gpu-blocklist",
  "--disable-dev-shm-usage",
  "--no-first-run",
  "--no-default-browser-check",
]);

export async function moTrinhDuyet() {
  const env = { ...process.env };
  if (process.env.AUDIT_FONTCONFIG) env.FONTCONFIG_FILE = process.env.AUDIT_FONTCONFIG;
  return chromium.launch({ executablePath: timTrinhDuyet(), headless: true, args: [...CO_CHROMIUM], env });
}

export async function taoContext(browser, ch, them = {}) {
  return browser.newContext({
    viewport: { width: ch.width, height: ch.height },
    screen: { width: ch.width, height: ch.height },
    deviceScaleFactor: DPR,
    isMobile: true,
    hasTouch: true,
    colorScheme: ch.colorScheme,
    reducedMotion: ch.reducedMotion,
    locale: "vi-VN",
    timezoneId: "Asia/Ho_Chi_Minh",
    ...them,
  });
}

/**
 * Collect what the page complains about, per step. `xa()` returns the records
 * gathered since the last call and empties the buffers, so each capture carries
 * only its own errors. URLs are cut short: the log stays outside git, but a
 * media URL is still no business of a report.
 */
export function ghiSuKien(page, { boQuaHost = ["tiles.openfreemap.org"] } = {}) {
  const log = { console: [], pageerror: [], requestfailed: [], http: [] };
  const ngan = (u) => {
    try {
      const url = new URL(u);
      return `${url.host}${url.pathname}`.slice(0, 160);
    } catch {
      return String(u).slice(0, 160);
    }
  };
  const boQua = (u) => boQuaHost.some((h) => String(u).includes(h));
  page.on("console", (m) => {
    if (m.type() === "error" || m.type() === "warning") log.console.push({ type: m.type(), text: m.text().slice(0, 300) });
  });
  page.on("pageerror", (e) => log.pageerror.push(String(e?.message ?? e).slice(0, 300)));
  page.on("requestfailed", (r) => {
    if (!boQua(r.url())) log.requestfailed.push({ url: ngan(r.url()), err: r.failure()?.errorText ?? "" });
  });
  page.on("response", (r) => {
    if (r.status() >= 400 && !boQua(r.url())) log.http.push({ url: ngan(r.url()), status: r.status(), method: r.request().method() });
  });
  return {
    log,
    xa() {
      const out = JSON.parse(JSON.stringify(log));
      for (const k of Object.keys(log)) log[k].length = 0;
      return out;
    },
  };
}
