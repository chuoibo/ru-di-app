/* Getting a page to a screen, and knowing when it has stopped moving.
 *
 * Two ways in, and they answer different questions:
 *   - `moLanh(url)` loads the URL: a cold deep link, the app boots, resumes the
 *     session from its cookie and routes. This is what a link from outside does.
 *   - `di(path)` pushes a history entry and fires popstate without a reload:
 *     in-app navigation as the router sees it, with the stack underneath kept.
 * A person tapping is still the preferred path; these exist for the steps
 * before the one under test.
 *
 * `choOn` waits for the network to go quiet (map tiles excluded: the sandbox
 * blocks them and maplibre retries forever), for fonts, for skeletons to leave,
 * and for two animation frames. It returns what it waited on so a capture can
 * say whether it was taken on a settled page.
 */

const LOAI_TRU = ["tiles.openfreemap.org"];

export function theoDoiMang(page) {
  const dangBay = new Set();
  const bo = (r) => LOAI_TRU.some((h) => r.url().includes(h));
  page.on("request", (r) => {
    if (!bo(r)) dangBay.add(r);
  });
  const xong = (r) => dangBay.delete(r);
  page.on("requestfinished", xong);
  page.on("requestfailed", xong);
  return dangBay;
}

export async function choOn(page, { mang, toiDa = 12_000, yenLang = 450, choThem = 350 } = {}) {
  const t0 = Date.now();
  let lanCuoiBan = Date.now();
  // Network quiet: no request in flight for `yenLang` ms.
  while (Date.now() - t0 < toiDa) {
    if (mang && mang.size > 0) lanCuoiBan = Date.now();
    if (Date.now() - lanCuoiBan >= yenLang) break;
    await page.waitForTimeout(80);
  }
  const conBay = mang ? mang.size : 0;
  await page.evaluate(() => document.fonts?.ready?.then(() => true)).catch(() => undefined);
  // Skeletons are `role=progressbar` labelled «Đang tải» (ui/Skeleton.tsx).
  let conSkeleton = 0;
  while (Date.now() - t0 < toiDa) {
    conSkeleton = await page.evaluate(
      () => [...document.querySelectorAll('[role="progressbar"]')].filter((e) => e.getClientRects().length > 0).length,
    );
    if (conSkeleton === 0) break;
    await page.waitForTimeout(150);
  }
  await page.evaluate(() => new Promise((ok) => requestAnimationFrame(() => requestAnimationFrame(() => ok(true)))));
  if (choThem) await page.waitForTimeout(choThem);
  return { msCho: Date.now() - t0, conBay, conSkeleton };
}

export async function moLanh(page, base, path = "/", opts = {}) {
  await page.goto(new URL(path, base).toString(), { waitUntil: "domcontentloaded" });
  return choOn(page, opts);
}

export async function di(page, path, opts = {}) {
  await page.evaluate((p) => {
    history.pushState(null, "", p);
    dispatchEvent(new PopStateEvent("popstate", { state: null }));
  }, path);
  return choOn(page, opts);
}

/** Browser Back: the web's version of the system back action. */
export async function quayLai(page, opts = {}) {
  await page.goBack({ waitUntil: "commit" }).catch(() => undefined);
  return choOn(page, opts);
}

export async function duongDan(page) {
  return page.evaluate(() => location.pathname + location.search);
}
