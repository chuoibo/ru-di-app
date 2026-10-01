/* N22's captures, side by side: one composite per finding or per state checked, from the
 * frames n22-ai-chat.mjs took. Then every ledger row whose frame sits in a composite gets that
 * composite too, so the matrix links a row to a committed image (the single frames stay outside
 * git). Same shape as n21-ghep.mjs. Kept to three composites: the evidence folder is near its
 * self-set budget (report §D).
 *
 *   source audit-env-main.sh && node kich-ban/n22-ghep.mjs
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
  ["EV-N22-CHIP-ghep", "N22 · chat khi máy chủ chưa có khoá AI: chip «chưa sẵn sàng» ở nhóm (C1–C3) và ở chat đôi; lúc gửi vẫn hiện «Đang hỏi Rủ Đi AI…» (UI-164); sau khi gửi: tin thường", [
    ["EV-N22-CHIP-C1", "nhóm, C1 390 · chip «chưa sẵn sàng · Gửi như tin thường»"],
    ["EV-N22-CHIP-C2", "C2 320"],
    ["EV-N22-CHIP-C3", "C3 360 tối"],
    ["EV-N22-GUI-CHO-C1", "UI-164 · đang gửi: «Đang hỏi Rủ Đi AI…»"],
    ["EV-N22-GUI-SAU-C1", "sau khi gửi: tin thường, không lời gọi AI"],
    ["EV-N22-DOI-CHIP-C1", "chat đôi chat-0/chat-1 (trống), C1: cùng chip"],
  ]],
  ["EV-N22-SAN-ghep", "N22 · nhóm như máy chủ có khoá (chỉ chat-capabilities viết lại ở trình duyệt): chip «Kèm 40 tin» (UI-163; nút 28×32, UI-001); tấm «Xem» trượt dưới dải ghim (UI-166); máy chủ thật từ chối; lời nhờ hỏng cũ (UI-091); Nếp khi không có khoá", [
    ["EV-N22-SAN-CHIP-C1", "UI-163 · chip «Kèm 40 tin gần đây» (ADR-0046: mặc định 20)"],
    ["EV-N22-XEM-GHIM-C1-ct", "UI-166 · «Xem», C1: dải ghim (khung đỏ) đè tay cầm, «Đóng bảng», dòng đầu"],
    ["EV-N22-XEM-GHIM-C8-ct", "UI-166 · C8 390×460: dải đè giữa câu đầu"],
    ["EV-N22-SAN-LOI-C1", "503 thật: «Rủ Đi AI chưa nhận lời nhờ» · Thử lại · Bỏ"],
    ["EV-N22-LOI-GOI-C1", "UI-091 · «Thử lại lời nhờ» tắt, không lý do"],
    ["EV-N22-NEP-C1", "bảng Nếp không khoá: câu lỗi, câu hỏi còn trong ô"],
  ]],
  ["EV-N22-LAB-ghep", "N22 · hai trang lab (server dev, fixture): /dev/tra-loi-song: bảng Nếp đang nghĩ / đang viết / xong (đầu trang) và hàng Rủ Đi AI trong luồng nhóm (nửa dưới) ở C1–C3, «Chạy thử», không vùng aria-live nào quanh chữ đang hiện (UI-165); /dev/hai-lop-chat: chip và tấm «Xem» của chat đôi", [
    ["EV-N22-LAB-C1", "C1 390 · đầu trang: bảng Nếp"],
    ["EV-N22-LAB-NHOM-C1", "C1 · luồng nhóm: người hỏi, thành viên khác"],
    ["EV-N22-LAB-C2", "C2 320 · đầu trang"],
    ["EV-N22-LAB-NHOM-C2", "C2 · luồng nhóm"],
    ["EV-N22-LAB-C3", "C3 360 tối · đầu trang"],
    ["EV-N22-LAB-NHOM-C3", "C3 · luồng nhóm"],
    ["EV-N22-LAB-CHAY-C1", "«Chạy thử» xong: lượt trả lời có chip"],
    ["EV-N22-DOI-LAB-C1", "hai-lop-chat · «Xem» chat đôi"],
  ]],
];

const browser = await moTrinhDuyet();
try {
  for (const [id, tieuDe, khung] of GHEP) {
    const thieu = khung.filter(([f]) => !existsSync(jpg(f))).map(([f]) => f);
    if (thieu.length) throw new Error(`${id}: thiếu ảnh ${thieu.join(", ")}`);
    const kq = await ghepAnh(browser, khung.map(([f, nhan]) => ({ file: jpg(f), nhan })), jpg(id), { cao: 660, tieuDe });
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
  if (r.feature !== "N22") continue;
  const them = [...new Set((r.evidence ?? []).flatMap((e) => khungCua.get(e) ?? []))].filter((g) => !(r.evidence ?? []).includes(g));
  if (!them.length) continue;
  const { luc: _luc, ...giu } = r;
  so.ghi({ ...giu, evidence: [...(r.evidence ?? []), ...them] });
  gan++;
}
console.log(`n22-ghep: ${GHEP.length} ảnh ghép; gắn vào ${gan} hàng`);
