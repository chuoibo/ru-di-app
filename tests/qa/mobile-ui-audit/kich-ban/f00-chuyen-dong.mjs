/* F00 motion: the tab indicator slide (MO02), the create tray under reduced
 * motion (MO03/MO04 at C9), and M1 playing once per session (L04).
 * Shapes of transitions only; no frame-rate claims (see thu-vien/chuyen-dong.mjs).
 */
import { join } from "node:path";
import { cauHinh } from "../thu-vien/cau-hinh.mjs";
import { batDauLayMau, ketThucLayMau, phanTich, quayKhung, ghepKhung } from "../thu-vien/chuyen-dong.mjs";
import { cdpCua, cham, tamCua } from "../thu-vien/cu-chi.mjs";
import { choDialog, dong } from "../thu-vien/lop-phu.mjs";
import { khoiDong, trangMoi } from "../thu-vien/moi-truong.mjs";
import { personaTheoTen } from "../thu-vien/phien.mjs";

const mt = await khoiDong();
const log = (o) => console.log(JSON.stringify(o));
try {
  for (const cfg of ["C1", "C9"]) {
    const t = await trangMoi(mt, cauHinh(cfg), { persona: personaTheoTen("dalat-0", mt.chatSessions), path: "/explore" });
    const cdp = await cdpCua(t.page);
    // MO02: the washi strip under the active tab (absolute child of the bar, 6px tall).
    await t.page.evaluate(() => {
      const bar = document.querySelector('[role="tab"]').parentElement;
      const ind = [...bar.children].find((e) => getComputedStyle(e).position === "absolute");
      ind.setAttribute("data-audit", "chi-bao");
    });
    await batDauLayMau(t.page, '[data-audit="chi-bao"]');
    await cham(cdp, await tamCua(t.page, '[role="tab"][aria-label="Cá nhân"]', { cuon: false }));
    await t.page.waitForTimeout(900);
    const mo02 = phanTich(await ketThucLayMau(t.page));
    log({ cfg, MO02: mo02 });
    // Back to Explore, then the tray: panel box over time.
    await cham(cdp, await tamCua(t.page, '[role="tab"][aria-label="Khám phá"]', { cuon: false }));
    await t.page.waitForTimeout(900);
    const quay = await quayKhung(t.page, cdp, { ms: 1300 });
    await batDauLayMau(t.page, '[data-testid="create-sheet"] [role="dialog"]');
    await cham(cdp, await tamCua(t.page, '[aria-label="Tạo mới"][role="button"]', { cuon: false }));
    const khung = await quay.dung();
    const mo04 = phanTich(await ketThucLayMau(t.page));
    await ghepKhung(mt.browser, khung, join(mt.out, "jpg", `EV-F00-MO04-mo-khay-${cfg}.jpg`), { tieuDe: `MO03/MO04 mở khay tạo, ${cfg}` });
    log({ cfg, MO04mo: mo04 });
    // Close, sample the panel's exit: where it is when it disappears.
    await batDauLayMau(t.page, '[data-testid="create-sheet"] [role="dialog"]');
    await dong(t.page, cdp, "x");
    await t.page.waitForTimeout(1200);
    const mauDong = await ketThucLayMau(t.page);
    const conHien = mauDong.filter((m) => m.n > 0);
    log({ cfg, MO04dong: phanTich(mauDong), khungCuoiTruocKhiMat: conHien.slice(-3) });
    // M1 once per session: the Nếp puppet in the tray, second opening.
    await cham(cdp, await tamCua(t.page, '[aria-label="Tạo mới"][role="button"]', { cuon: false }));
    await choDialog(t.page);
    await t.page.evaluate(() => {
      const d = document.querySelector('[data-testid="create-sheet"] [role="dialog"]');
      const nep = [...d.querySelectorAll("[tabindex]")].find((e) => !e.getAttribute("aria-label") && e.getBoundingClientRect().width > 90);
      nep?.firstElementChild?.setAttribute("data-audit", "nep-m1");
    });
    await batDauLayMau(t.page, '[data-audit="nep-m1"]');
    await t.page.waitForTimeout(1600);
    log({ cfg, M1lan2: phanTich(await ketThucLayMau(t.page)) });
    await t.context.close();
  }
} finally {
  await mt.dong();
}
