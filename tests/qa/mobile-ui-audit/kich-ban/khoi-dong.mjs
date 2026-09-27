/* Smoke: the build loads, which renderer it drew with, which font the body
 * text got, and whether a cookie sign-in lands on a tab. Run before any
 * feature script; a red smoke means every later capture would measure the
 * setup, not the app.
 *
 *   AUDIT_WEB=… AUDIT_OUT=… node kich-ban/khoi-dong.mjs
 */
import { cauHinh } from "../thu-vien/cau-hinh.mjs";
import { chup } from "../thu-vien/chup.mjs";
import { choOn, duongDan } from "../thu-vien/dieu-huong.mjs";
import { khoiDong, trangMoi } from "../thu-vien/moi-truong.mjs";
import { personaDaLat } from "../thu-vien/phien.mjs";

const mt = await khoiDong();
try {
  const ch = cauHinh("C1");
  const a = await trangMoi(mt, ch, { path: "/" });
  const fontChu = await a.page.evaluate(() => {
    const el = [...document.querySelectorAll("div,span")].find((e) => [...e.childNodes].some((n) => n.nodeType === 3 && n.textContent.trim().length > 3));
    return el ? getComputedStyle(el).fontFamily : null;
  });
  const m1 = await chup(a.page, { out: mt.out, id: "SMOKE-welcome-C1", suKien: a.suKien, on: a.on });
  console.log(JSON.stringify({ buoc: "chua-dang-nhap", duong: await duongDan(a.page), fontChu, font: mt.font, tomTat: m1.tomTat }));
  await a.context.close();

  const b = await trangMoi(mt, ch, { persona: personaDaLat(0), path: "/" });
  await choOn(b.page, { mang: b.mang, toiDa: 15_000 });
  const m2 = await chup(b.page, { out: mt.out, id: "SMOKE-dalat0-C1", suKien: b.suKien, on: b.on });
  console.log(JSON.stringify({ buoc: "dang-nhap-cookie", duong: await duongDan(b.page), tomTat: m2.tomTat, suKien: m2.suKien }));
  await b.context.close();
} finally {
  await mt.dong();
}
