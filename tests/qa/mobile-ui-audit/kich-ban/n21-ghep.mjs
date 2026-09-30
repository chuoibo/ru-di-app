/* N21's captures, side by side: one composite per finding or per state
 * checked, from the frames n21-ho-so.mjs took. Then every ledger row whose
 * frame sits in a composite gets that composite too, so the matrix links a row
 * to a committed image (the single frames stay outside git). Same shape as
 * n15-ghep.mjs. Annotated frames (`-ct`) carry the red box of the capture.
 *
 *   source audit-env-main.sh && node kich-ban/n21-ghep.mjs
 */
import { existsSync } from "node:fs";
import { join } from "node:path";

import { ghepAnh } from "../thu-vien/chup.mjs";
import { soGhi } from "../thu-vien/ghi.mjs";
import { moTrinhDuyet } from "../thu-vien/trinh-duyet.mjs";

const out = process.env.AUDIT_OUT;
if (!out) throw new Error("thiếu AUDIT_OUT");
const so = soGhi(out);
const jpg = (id) => join(out, "jpg", `${id}.jpg`);

const GHEP = [
  ["EV-N21-HT-ghep", "N21 · sổ hành trình của chat-0 (3 huy hiệu mở đầu, chưa kết) ở C1, C2, C3; C6 trải khoảng 720 (UI-093)", [
    ["EV-N21-HT-C1", "C1 390 · đầu màn («MỚI MỞ» của máy mới)"],
    ["EV-N21-HT-DUOI-C1", "C1 · thẻ ngã rẽ, «sắp dùng được»"],
    ["EV-N21-HT-C2", "C2 320"],
    ["EV-N21-HT-C3", "C3 360 tối"],
    ["EV-N21-HT-C6", "C6 768"],
  ]],
];

const browser = await moTrinhDuyet();
try {
  for (const [id, tieuDe, khung] of GHEP) {
    const thieu = khung.filter(([f]) => !existsSync(jpg(f))).map(([f]) => f);
    if (thieu.length) throw new Error(`${id}: thiếu ảnh ${thieu.join(", ")}`);
    const kq = await ghepAnh(browser, khung.map(([f, nhan]) => ({ file: jpg(f), nhan })), jpg(id), { cao: 700, tieuDe });
    console.log(`${id}: ${khung.length} khung, ${kq.bytes} byte`);
  }
} finally {
  await browser.close();
}

// Every row naming a frame of a composite (or its unannotated twin) gets the composite as well;
// a row that already names it is left alone, so a second run adds nothing.
const khungCua = new Map();
for (const [id, , khung] of GHEP)
  for (const [f] of khung) for (const ten of [f, f.replace(/-ct$/, "")]) khungCua.set(ten, [...new Set([...(khungCua.get(ten) ?? []), id])]);
const moiNhat = new Map();
for (const r of so.doc()) {
  if (r.rut) {
    for (const k of [...moiNhat.keys()]) if (k.startsWith(`${r.tc}|`)) moiNhat.delete(k);
    continue;
  }
  moiNhat.set(`${r.tc}|${r.nenTang ?? "web"}|${r.cauHinh ?? ""}`, r);
}
let gan = 0;
for (const r of moiNhat.values()) {
  if (r.feature !== "N21") continue;
  const them = [...new Set((r.evidence ?? []).flatMap((e) => khungCua.get(e) ?? []))].filter((g) => !(r.evidence ?? []).includes(g));
  if (!them.length) continue;
  const { luc: _luc, ...giu } = r;
  so.ghi({ ...giu, evidence: [...(r.evidence ?? []), ...them] });
  gan++;
}
console.log(`n21-ghep: ${GHEP.length} ảnh ghép; gắn vào ${gan} hàng`);
