/* Network faults for the loading / error / offline / retry states.
 *
 * Injected at the browser, never at the server: the stack stays honest for the
 * next step, and the fault is scoped to one page. The injected error body uses
 * a snake_case code on purpose, so a screen that prints the server's code
 * instead of a Vietnamese sentence is caught by the `maLoi` detector.
 */
const cho = (ms) => new Promise((ok) => setTimeout(ok, ms));

export const THAN_LOI = JSON.stringify({ detail: "audit_injected_failure" });

export async function loiMayChu(page, mau, status = 503) {
  await page.route(mau, (r) => r.fulfill({ status, contentType: "application/json", body: THAN_LOI }));
}

export async function tre(page, mau, ms) {
  await page.route(mau, async (r) => {
    await cho(ms);
    await r.continue().catch(() => undefined);
  });
}

export async function hetGio(page, mau) {
  await page.route(mau, (r) => r.abort("timedout"));
}

export async function anhHong(page, mau = /\.(png|jpe?g|webp)(\?|$)|\/media\//) {
  await page.route(mau, (r) => (r.request().resourceType() === "image" ? r.fulfill({ status: 404, body: "" }) : r.continue()));
}

export async function ngatMang(context, tat) {
  await context.setOffline(tat);
}

export async function goHet(page) {
  await page.unrouteAll({ behavior: "ignoreErrors" });
}
