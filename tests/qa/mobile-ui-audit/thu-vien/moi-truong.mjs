/* Where the audit runs: which build, which API, where evidence goes.
 *
 *   AUDIT_WEB   directory of an `expo export --platform web` build (served here
 *               by apps/mobile/tests/chrome-cdp.mjs `serve()`, which falls back
 *               to index.html and knows the wasm MIME type), OR
 *   AUDIT_BASE  URL of an already running server (the dev server, for /dev/*)
 *   AUDIT_API   API base the build was exported against (the Go front door)
 *   AUDIT_OUT   output directory, outside the repository
 *   AUDIT_CHAT_SESSIONS  sessions file written by scripts/chat_e2e_seed.mjs
 *
 * Page and API both live on 127.0.0.1: the web session cookie is only sent
 * same-site, and a mismatch would make every "resume after reload" check
 * measure the setup instead of the app.
 */
import { serve } from "../../../../apps/mobile/tests/chrome-cdp.mjs";

import { taoContext, moTrinhDuyet, ghiSuKien } from "./trinh-duyet.mjs";
import { ganPhien, layPhien } from "./phien.mjs";
import { moLanh, theoDoiMang } from "./dieu-huong.mjs";

export function docMoiTruong() {
  const out = process.env.AUDIT_OUT;
  if (!out) throw new Error("đặt AUDIT_OUT (thư mục ngoài repo cho ảnh và số đo)");
  if (!process.env.AUDIT_WEB && !process.env.AUDIT_BASE) throw new Error("đặt AUDIT_WEB (thư mục export web) hoặc AUDIT_BASE (URL)");
  return {
    web: process.env.AUDIT_WEB ?? null,
    baseCoSan: process.env.AUDIT_BASE ?? null,
    api: process.env.AUDIT_API ?? "http://127.0.0.1:58099",
    out,
    chatSessions: process.env.AUDIT_CHAT_SESSIONS ?? null,
    font: process.env.AUDIT_FONTCONFIG ? "roboto" : "mac-dinh",
  };
}

export async function khoiDong() {
  const env = docMoiTruong();
  let sv = null;
  let base = env.baseCoSan;
  if (!base) {
    sv = await serve(env.web);
    base = sv.url;
  }
  const browser = await moTrinhDuyet();
  return {
    ...env,
    base,
    browser,
    async dong() {
      await browser.close().catch(() => undefined);
      if (sv) await sv.close();
    },
  };
}

/**
 * A fresh context at configuration `ch`, signed in as `persona` (or signed out
 * when persona is null), opened cold at `path`.
 */
export async function trangMoi(mt, ch, { persona = null, path = "/", context: themContext = {} } = {}) {
  const context = await taoContext(mt.browser, ch, themContext);
  const page = await context.newPage();
  const mang = theoDoiMang(page);
  const suKien = ghiSuKien(page);
  if (persona) {
    const phien = await layPhien(mt.api, persona, `${mt.out}/phien`);
    await page.goto(new URL("/favicon.ico", mt.base).toString(), { waitUntil: "commit" }).catch(() => undefined);
    await ganPhien(page, mt.api, phien);
  }
  const on = await moLanh(page, mt.base, path, { mang });
  return { context, page, mang, suKien, on };
}
