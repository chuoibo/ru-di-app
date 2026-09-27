/* Baseline captures: one or more screens, cold-opened at each configuration,
 * measured, and laid side by side for looking.
 *
 *   node kich-ban/baseline.mjs --persona dalat-0 --configs C1,C2,C3 \
 *     --man F02.S01=/explore,F02.S02=/destinations
 *
 * --persona: dalat-<i> (Team Đà Lạt), chat-<i> (chat seed), moi-<n> (new
 * account), or "none" (signed out). Each capture is EV-<screen>-BASE-<config>;
 * the side-by-side sheet is EV-<screen>-BASE-ghep. A line per capture goes to
 * stdout with the signal summary; nothing is judged here.
 */
import { join } from "node:path";

import { danhSach } from "../lib/cau-hinh.mjs";
import { chup, ghepAnh } from "../lib/chup.mjs";
import { choOn, duongDan } from "../lib/dieu-huong.mjs";
import { khoiDong, trangMoi } from "../lib/moi-truong.mjs";
import { personaTheoTen } from "../lib/phien.mjs";

const arg = (ten, macDinh) => {
  const i = process.argv.indexOf(`--${ten}`);
  return i === -1 ? macDinh : process.argv[i + 1];
};

const mt = await khoiDong();
try {
  const nguoi = personaTheoTen(arg("persona", "none"), mt.chatSessions);
  const cfgs = danhSach(arg("configs", "C1,C2,C3"));
  const man = arg("man", "")
    .split(",")
    .filter(Boolean)
    .map((s) => s.split("="));
  const choThem = Number(arg("cho", "0"));
  for (const [id, path] of man) {
    const anh = [];
    for (const ch of cfgs) {
      const t = await trangMoi(mt, ch, { persona: nguoi, path });
      if (choThem) await t.page.waitForTimeout(choThem);
      await choOn(t.page, { mang: t.mang });
      const evId = `EV-${id}-BASE-${ch.id}`;
      const m = await chup(t.page, { out: mt.out, id: evId, suKien: t.suKien, on: t.on });
      console.log(JSON.stringify({ ev: evId, duong: await duongDan(t.page), ...m.tomTat, loiTrang: m.suKien.pageerror.length, http: m.suKien.http.map((h) => `${h.status} ${h.method} ${h.url}`).slice(0, 4) }));
      anh.push({ file: join(mt.out, "jpg", `${evId}.jpg`), nhan: `${ch.id} ${ch.width}×${ch.height} ${ch.colorScheme}` });
      await t.context.close();
    }
    if (anh.length > 1) await ghepAnh(mt.browser, anh, join(mt.out, "jpg", `EV-${id}-BASE-ghep.jpg`), { tieuDe: `${id} ${path}` });
  }
} finally {
  await mt.dong();
}
