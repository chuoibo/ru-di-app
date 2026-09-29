/* What browser history a chain of tab switches leaves, then where Back goes.
 * Read-only: prints one JSON line, writes nothing to the ledger. It is the
 * probe behind UI-123, kept so the comparison can be re-run on either build:
 *
 *   source audit-env-main.sh && node kich-ban/tham-do-lich-su-tab.mjs
 *   source audit-env.sh      && CHUOI="/explore,Lên plan,Tin nhắn,Cá nhân" node kich-ban/tham-do-lich-su-tab.mjs
 *
 * CHUOI is the start path, then the tab names to tap; PERSONA names who is
 * signed in (default dalat-0, «none» for signed out). Each step prints
 * «path#history.length»; Back is pressed once per tab tapped. «/favicon.ico» is
 * the page the harness opens first to set the session, so Back landing there
 * (or on about:blank) means Back left the app.
 */
import { cauHinh } from "../thu-vien/cau-hinh.mjs";
import { cdpCua, cham } from "../thu-vien/cu-chi.mjs";
import { khoiDong, trangMoi } from "../thu-vien/moi-truong.mjs";
import { personaTheoTen } from "../thu-vien/phien.mjs";

const mt = await khoiDong();
const [batDau, ...tabs] = (process.env.CHUOI ?? "/community,Khám phá,Lên plan,Tin nhắn").split(",");
const t = await trangMoi(mt, cauHinh("C1"), { persona: personaTheoTen(process.env.PERSONA ?? "dalat-0", mt.chatSessions), path: batDau });
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
  tien.push(`${ten}→${n ? await doc() : "không thấy tab"}`);
}
const back = [];
for (let i = 0; i < tabs.length; i += 1) {
  await t.page.goBack().catch(() => null);
  await t.page.waitForTimeout(1500);
  back.push(t.page.url().startsWith("about:") ? "about:blank" : new URL(t.page.url()).pathname);
}
console.log(JSON.stringify({ tien, back }));
// The browser is left to the process exit: closing it has hung here once.
process.exit(0);
